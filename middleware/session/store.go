package session

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/maphash"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/ctxlocal"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
)

// ErrEmptySessionID is an error that occurs when the session ID is empty.
var (
	ErrEmptySessionID                   = errors.New("session ID cannot be empty")
	ErrSessionAlreadyLoadedByMiddleware = errors.New("session already loaded by middleware")
	ErrSessionIDNotFoundInStore         = errors.New("session ID not found in session store")
	// ErrKeyNotFound is returned by GetInto when the session holds nothing under the key.
	ErrKeyNotFound = errors.New("session key not found")
)

// keyNamespace keeps sessions apart from other middleware sharing the Storage:
// the ID comes from the request, so a bare one could name any of their entries.
// Config.StorageKeyPrefix goes in front of it.
const keyNamespace = "session:"

// idLockShards is a power of two so the shard index is a mask. Shared by every
// Store, since two stores on one Storage address the same entries.
const idLockShards = 64

// idLock keeps a request that loaded a session from writing it back after
// another request deleted or changed it: fiber.Storage has no conditional write.
type idLock struct {
	mu  sync.RWMutex  // write-backs share it, a delete or a change holds it alone
	gen atomic.Uint64 // bumped after every delete or change in the shard; a load reads it before the storage
}

var (
	idLocks [idLockShards]idLock
	idSeed  = maphash.MakeSeed()
)

func lockFor(key string) *idLock {
	return &idLocks[maphash.String(idSeed, key)&(idLockShards-1)]
}

// storageKey is the key a session is stored under, which is not its ID: it has the
// StorageKeyPrefix and the namespace of the store, or is the digest of the ID. It is a
// type of its own so that an ID cannot be passed where a key is wanted, to the lock of
// a shard or to a delete, and be read, written or deleted under another name than the
// rest of the session.
type storageKey string

// lock returns the lock of the shard the key belongs to.
func (k storageKey) lock() *idLock {
	return lockFor(string(k))
}

// Store manages session data using the configured storage backend. It holds atomic
// fields, so it is made by NewStore and used through the pointer it returns: copying
// a Store is flagged by go vet.
type Store struct {
	// named is the Codec with its name, asked once. It is nil for a store without a
	// Codec, and for one built without NewStore, which sessionFormat makes up. Changing
	// the Codec of a Store after NewStore has no effect.
	named *namedCodec
	// locals is the PassLocalsToContext setting of the app that served the store last,
	// see passesLocalsToContext.
	locals atomic.Pointer[localsSetting]
	// profile is what NewStore worked out from the Extractor. A Store built without
	// NewStore has none, and a Store whose Extractor was replaced since has a stale
	// one: both walk the extractor when they need an answer, see extractorProfile.
	profile extractorProfile
	Config
	// unreadableWarned is set once the store has said that it met a session it
	// cannot read.
	unreadableWarned atomic.Bool
}

// localsSetting is the PassLocalsToContext setting of an app.
type localsSetting struct {
	app    *fiber.App
	passes bool
}

// passesLocalsToContext reports whether the app of c copies the request locals into the
// request context. An app does not change the setting once it is made, and Config returns
// the whole Config by value, which every request would pay for it, so the store reads it
// once for each app it serves.
func (s *Store) passesLocalsToContext(c fiber.Ctx) bool {
	app := c.App()
	if known := s.locals.Load(); known != nil && known.app == app {
		return known.passes
	}
	fresh := &localsSetting{app: app, passes: app.Config().PassLocalsToContext}
	s.locals.Store(fresh)
	return fresh.passes
}

// hashedKeyLen is the length of a base64url-encoded SHA-256 digest without padding.
const hashedKeyLen = 43

// storageKey returns the key the session with the given ID is stored under: the
// StorageKeyPrefix of the store, the namespace of the middleware ("session:"), and
// the ID itself, or its digest with HashStorageKeys. Every read, write and delete
// of a session goes through it.
//
// Parameters:
//   - id: The session ID the client holds.
//
// Returns:
//   - storageKey: The key to use with the storage.
func (s *Store) storageKey(id string) storageKey {
	// The default is the common case, and kept small enough to inline.
	if s.StorageKeyPrefix == "" && !s.HashStorageKeys {
		return storageKey(keyNamespace + id)
	}
	return s.derivedStorageKey(id)
}

// derivedStorageKey is storageKey for a store with a prefix or hashing.
func (s *Store) derivedStorageKey(id string) storageKey {
	if !s.HashStorageKeys {
		return storageKey(s.StorageKeyPrefix + keyNamespace + id)
	}
	sum := sha256.Sum256(utils.UnsafeBytes(id))
	// One buffer of the exact size, handed over as the key: nothing writes to it
	// afterwards, and it spares the second allocation a string conversion costs.
	key := make([]byte, 0, len(s.StorageKeyPrefix)+len(keyNamespace)+hashedKeyLen)
	key = append(key, s.StorageKeyPrefix...)
	key = append(key, keyNamespace...)
	key = base64.RawURLEncoding.AppendEncode(key, sum[:])
	return storageKey(utils.UnsafeString(key))
}

// ownedID returns id as a string that does not point into the request: the end of
// the key when the key holds the ID, a copy when it holds its digest. A session
// outlives the request that named it, and the buffer of the request is reused.
func (s *Store) ownedID(key storageKey, id string) string {
	if s.HashStorageKeys {
		return utils.CopyString(id)
	}
	return string(key[len(key)-len(id):])
}

// NewStore creates a new session store with the provided configuration.
//
// Parameters:
//   - config: Variadic parameter to override default config.
//
// Returns:
//   - *Store: The session store.
//
// Usage:
//
//	store := session.NewStore()
func NewStore(config ...Config) *Store {
	// Set default config
	cfg := configDefault(config...)
	if isTypedNil(cfg.Storage) {
		panic("[session] Storage must not be a typed nil")
	}
	if isTypedNil(cfg.Codec) {
		panic("[session] Codec must not be a typed nil")
	}
	var named *namedCodec
	if cfg.Codec != nil {
		name := cfg.Codec.Name()
		checkCodecName(name)
		named = &namedCodec{Codec: cfg.Codec, name: name}
	}
	warnCookiePrefixes(&cfg)

	if cfg.Storage == nil {
		cfg.Storage = memory.New()
	}

	store := &Store{
		Config:  cfg,
		profile: profileOf(cfg.Extractor),
		named:   named,
	}

	if cfg.AbsoluteTimeout > 0 {
		store.RegisterType(absExpirationKey)
		store.RegisterType(time.Time{})
	}

	return store
}

// RegisterType registers a custom type for encoding/decoding into any storage provider.
//
// Parameters:
//   - i: The custom type to register.
//
// Usage:
//
//	store.RegisterType(MyCustomType{})
func (*Store) RegisterType(i any) {
	gob.Register(i)
}

// Get will get/create a session.
//
// This function will return an ErrSessionAlreadyLoadedByMiddleware if
// the session is already loaded by the middleware.
//
// For a visitor who has no cookie yet, the ID that Get issues is remembered for the
// request by the store and by the stores that share a cookie or header with it, and by
// no others: asked again after a Save, a store returns the session it saved, whatever
// other stores were asked in between, and under the new ID if Regenerate or Reset gave
// the session one. An ID that was not saved is not a session, and asking again before
// a Save issues another.
//
// Parameters:
//   - c: The Fiber context.
//
// Returns:
//   - *Session: The session object.
//   - error: An error if the session retrieval fails or if the session is already loaded by the middleware.
//
// Usage:
//
//	sess, err := store.Get(c)
//	if err != nil {
//	    // handle error
//	}
func (s *Store) Get(c fiber.Ctx) (*Session, error) {
	// If session is already loaded in the context,
	// it should not be loaded again
	if currentMiddleware(c) != nil {
		return nil, ErrSessionAlreadyLoadedByMiddleware
	}

	sess, err := s.loadSession(c, nil)
	if err == nil && sess.isFresh {
		// Nothing the request carries names the ID that was just issued: say so, for
		// the next store that is asked, which could not find it there.
		cacheID(c, s, sess.identityLocked())
	}
	return sess, err
}

// FromContext returns the Middleware that this store's session middleware
// installed for the request, or nil when the store has none there. It accepts
// the same contexts as the package-level FromContext and ends the same way: a
// context kept past the request returns nil.
//
// Register one session middleware per store, each over its own cookie or
// header, and a handler reaches every session of the request:
//
//	loginHandler, loginStore := session.NewWithStore(session.Config{
//	    Extractor: extractors.FromCookie("login_sid"),
//	})
//	appHandler, appStore := session.NewWithStore(session.Config{
//	    Extractor: extractors.FromCookie("app_sid"),
//	})
//	app.Use(loginHandler, appHandler)
//
//	login := loginStore.FromContext(c)
//	appSession := appStore.FromContext(c)
//
// Like the methods of Middleware it takes the read lock of the middleware it
// visits, so call it from handlers, not from a Storage while that middleware is
// destroying, resetting or regenerating its session, which hold the lock.
//
// Parameters:
//   - ctx: The Fiber context, a *fasthttp.RequestCtx, or a context.Context.
//
// Returns:
//   - *Middleware: The store's middleware if it is running for the request;
//     otherwise, nil.
func (s *Store) FromContext(ctx any) *Middleware {
	return FromContext(ctx).find(func(m *Middleware) bool {
		return m.store == s
	})
}

// loadSession retrieves a session based on the context. The session is the one saved
// under an ID that a store sharing a source with s cached on the request, the newest
// first, or else the one the request names; a new one if there is neither.
//
// Parameters:
//   - c: The Fiber context.
//   - outer: The session middleware the request currently holds, nil if it holds none.
//
// Returns:
//   - *Session: The session object.
//   - error: An error if the session retrieval fails.
//
// Usage:
//
//	sess, err := store.loadSession(c, nil)
//	if err != nil {
//	    // handle error
//	}
func (s *Store) loadSession(c fiber.Ctx, outer *Middleware) (*Session, error) {
	var found storedSession
	cached := requestIDs(c)
	if outer != nil || cached != nil {
		if err := s.cachedSession(c, outer, cached, &found); err != nil {
			return nil, err
		}
	}
	if found.raw == nil {
		// Nothing was cached for s that is saved: the request names the session, if any.
		res, err := s.getSessionID(c)
		if err != nil {
			return nil, err
		}
		found.res = res
		if s.mayBeStored(cached, res.Value) {
			if readErr := s.readSession(c, res.Value, &found); readErr != nil {
				return nil, readErr
			}
		}
	}

	// A new session if none is stored: nothing that was cached or named is ours.
	isFresh := found.raw == nil
	id := found.id
	if isFresh {
		id = s.KeyGenerator()
	}

	// Create session object
	sess := acquireSession()

	sess.mu.Lock()

	sess.ctx = c
	sess.bind(s)
	sess.id = id
	sess.key = found.key
	sess.isFresh = isFresh
	sess.extractor = found.res

	// Decode session data if found
	if found.raw != nil {
		if err := sess.decodeSessionData(found.raw); err != nil {
			sess.mu.Unlock()
			sess.Release()
			return nil, fmt.Errorf("failed to decode session data: %w", err)
		}
		sess.loaded = true
		sess.loadGen = found.gen
	}

	sess.mu.Unlock()

	if isFresh {
		if s.AbsoluteTimeout > 0 {
			sess.setAbsExpiration(time.Now().Add(s.AbsoluteTimeout))
		}
	} else if sess.isAbsExpired() {
		// Whatever AbsoluteTimeout the store has now: the session was stored with a
		// deadline, and it has passed. Reset arms the session that takes its place
		// when the store has an AbsoluteTimeout; a store without one gives it none,
		// where a deadline of now would make the next load start it over again.
		if err := sess.Reset(); err != nil {
			return nil, fmt.Errorf("failed to reset session: %w", err)
		}
	}

	return sess, nil
}

// mayBeStored reports whether a session may be stored under id, the ID the request names:
// not when it names none, and not when s looked for a session under it earlier in the
// request and found none. The entry s cached then holds the ID the request named when s
// issued its own, or the one a Regenerate or Reset deleted, and neither names a session.
func (s *Store) mayBeStored(cached *cachedID, id string) bool {
	if id == "" {
		return false
	}
	entry := cached.of(s)
	return entry == nil || entry.res.Value != id
}

// cachedSession reads the session that was saved under the newest ID which s, or a
// store sharing a source with it, cached on the request, into found: first that of
// the nearest session middleware s runs inside that shares one, then those of
// Store.Get, cached, newest first. An ID that names no stored session, because it was
// never saved, is passed over, and the request is asked instead. Stores that share no
// source with s keep sessions of their own: their IDs are not taken, and they do not
// replace the ones of s.
func (s *Store) cachedSession(c fiber.Ctx, outer *Middleware, cached *cachedID, found *storedSession) error {
	if outer != nil {
		if ident, ok := outer.sharedID(s); ok {
			if err := s.readSession(c, ident.id, found); err != nil || found.raw != nil {
				found.res = s.places().adopt(ident.res)
				return err
			}
		}
	}
	for ; cached != nil; cached = cached.next {
		if !s.sharesSource(cached.store) {
			continue
		}
		if err := s.readSession(c, cached.id, found); err != nil || found.raw != nil {
			found.res = s.places().adopt(cached.res)
			return err
		}
	}
	return nil
}

// places returns where s reads its session ID from and writes it back to: what NewStore
// worked out, or, when the Extractor was replaced since or the Store was built without
// NewStore, what a walk of the extractor it has now finds. Do not modify the slices:
// every session of the store shares them.
func (s *Store) places() *idPlaces {
	if s.profile.current(s.Extractor) {
		return &s.profile.places
	}
	fresh := placesOf(s.Extractor)
	return &fresh
}

// sharesSource reports whether s and other read the session ID from a common
// source, by kind and name, or cannot be told apart because one of them names
// none.
//
// A request can hold the sessions of several stores. Stores over the same
// cookie, such as a session middleware and the store the CSRF middleware was
// given, resolve the same session and so share the ID cached on the request.
// Stores over different sources keep independent sessions: one adopting the
// other's cached ID would look up a foreign session, or write its own ID to a
// cookie it does not own. A source that cannot be written back to, such as the
// query string, counts like any other: the ID a link carries reaches the stores
// that read the query string, and no cookie store next to them.
func (s *Store) sharesSource(other *Store) bool {
	return s == other || s.places().shares(other.places())
}

// storedSession is the outcome of a lookup of a session: the record, if a session is
// stored under the ID that was looked up, and where that ID came from.
type storedSession struct {
	raw []byte     // the record, nil if no session is stored under the ID
	key storageKey // storage key of the session
	// sessionIdentity is the ID of the session, which does not point into the request, with
	// the provenance of the ID the request named. The provenance is set when no session is
	// stored under the ID the request named, too: the new session is written back to the
	// same place.
	sessionIdentity
	gen uint64 // idLock generation read before the storage
}

// readSession looks for the session with the given ID in the storage and, if one is
// stored, fills found with it. A record the store cannot read counts as none.
func (s *Store) readSession(ctx context.Context, id string, found *storedSession) error {
	key := s.storageKey(id)
	gen := key.lock().gen.Load()
	raw, err := s.Storage.GetWithContext(ctx, string(key))
	if err != nil || raw == nil || s.refusesRecord(raw) {
		return err
	}
	// Not the ID as given: it may point into the request buffer the next request reuses.
	found.raw, found.key, found.id, found.gen = raw, key, s.ownedID(key, id), gen
	return nil
}

// recordCodec returns the Codec of the store with its name, nil for a store without
// one. NewStore asks the codec for its name once; a Store built without NewStore asks
// every time.
func (s *Store) recordCodec() *namedCodec {
	switch {
	case s.named != nil:
		return s.named
	case s.Codec == nil:
		return nil
	default:
		return &namedCodec{Codec: s.Codec, name: s.Codec.Name()}
	}
}

// sessionFormat returns the format the sessions of the store hold.
func (s *Store) sessionFormat() format {
	if codec := s.recordCodec(); codec != nil {
		return codec
	}
	return gobStream
}

// refusesRecord reports whether raw, a stored session, is a record this store
// cannot read, and says so, once per store. A store reads the records of the codec
// that wrote them, and no other. (A stored session that is not a record is gob,
// which every store reads, a store with a codec as a session that stays gob.) A
// record that is damaged counts as one the store can read, and the decode says
// what is wrong with it.
func (s *Store) refusesRecord(raw []byte) bool {
	return isRecord(raw) && s.cannotRead(raw)
}

// cannotRead is refusesRecord for a stored session that is a record.
func (s *Store) cannotRead(raw []byte) bool {
	codec := s.recordCodec()
	if codec != nil && codec.reads(raw) {
		return false
	}
	s.warnUnreadable(raw, codec)
	return true
}

// warnUnreadable says, once per store, that a stored session is a record this store,
// whose codec is codec, nil if it has none, cannot read, and is treated as a session
// that does not exist.
func (s *Store) warnUnreadable(raw []byte, codec *namedCodec) {
	if !s.unreadableWarned.CompareAndSwap(false, true) {
		return
	}
	have := "has no codec"
	if codec != nil {
		have = fmt.Sprintf("uses the codec %q", codec.name)
	}
	version, name, known := recordCodecOf(raw)
	switch {
	case len(name) > 0:
		log.Warnf("[session] a stored session is a record of the codec %q (format version %d), which this store, which %s, cannot read; "+
			"it is treated as a new session, and the record is left alone. This is expected once after Config.Codec was changed or turned off; "+
			"if it goes on, applications with different codecs share a storage and a session cookie",
			name, version, have)
	case known:
		log.Warnf("[session] a stored session is a record of format version %d, which this store, which %s, does not know; "+
			"it was probably written by a newer release of the application. It is treated as a new session, and the record is left alone",
			version, have)
	default:
		log.Warn("[session] a stored session starts like a record, but its header is damaged; " +
			"it is treated as a new session, and the record is left alone")
	}
}

// getSessionID returns the session ID using the configured extractor, together
// with the provenance of the extractor that supplied it.
//
// Parameters:
//   - c: The Fiber context.
//
// Returns:
//   - extractors.Result: the ID in Value with the provenance that supplied it,
//     zero when nothing was extracted.
//   - error: why extraction failed, nil when nothing was supplied at all.
//
// Usage:
//
//	from, err := store.getSessionID(c)
func (s *Store) getSessionID(c fiber.Ctx) (extractors.Result, error) {
	// Resolved through the extractors package rather than walked here: walking
	// Extractor.Chain directly would skip a chain-level Extract, and Resolve
	// also reports which extractor won, which setSession needs to write the ID
	// back to the sink it came from.
	//
	// That Extract must be value-preserving — validate or refuse, do not
	// rewrite. The ID is written back untransformed, so a decorator that
	// rewrote it on read would never match its own stored session.
	res, err := extractors.Resolve(s.Extractor, c)
	switch {
	case err == nil:
		return res, nil
	case errors.Is(err, extractors.ErrNotFound):
		// The ordinary first-visit case, not a failure: an empty ID generates a
		// fresh session.
		return extractors.Result{}, nil
	default:
		// A validator refusing a forged ID, a cycle, or a custom extractor's own
		// error. Reported rather than collapsed into "no ID present", which
		// would be indistinguishable from a first-time visitor.
		return extractors.Result{}, err
	}
}

// Reset deletes all sessions from the storage.
//
// Returns:
//   - error: An error if the reset operation fails.
//
// Usage:
//
//	err := store.Reset()
//	if err != nil {
//	    // handle error
//	}
func (s *Store) Reset(ctx context.Context) error {
	for i := range idLocks {
		idLocks[i].mu.Lock()
	}
	defer func() {
		for i := range idLocks {
			idLocks[i].gen.Add(1)
			idLocks[i].mu.Unlock()
		}
	}()
	return s.Storage.ResetWithContext(ctx)
}

// Delete deletes a session by its ID.
//
// Parameters:
//   - id: The unique identifier of the session.
//
// Returns:
//   - error: An error if the deletion fails or if the session ID is empty.
//
// Usage:
//
//	err := store.Delete(id)
//	if err != nil {
//	    // handle error
//	}
func (s *Store) Delete(ctx context.Context, id string) error {
	if id == "" {
		return ErrEmptySessionID
	}
	return s.deleteKey(ctx, s.storageKey(id))
}

func (s *Store) deleteKey(ctx context.Context, key storageKey) error {
	l := key.lock()
	l.mu.Lock()
	defer l.mu.Unlock()
	err := s.Storage.DeleteWithContext(ctx, string(key))
	l.gen.Add(1) // after the delete, so a load that sees the new value cannot read the entry
	return err
}

// deleteLoadedKey is deleteKey for a session read at loadGen. It returns what the key held
// when another request had written the shard since, nil when none had, and reports whether
// one had deleted the key.
//
//nolint:gocritic // returning explicit values keeps the signature concise while avoiding unnecessary named results
func (s *Store) deleteLoadedKey(ctx context.Context, key storageKey, loadGen uint64) ([]byte, bool, error) {
	l := key.lock()
	l.mu.Lock()
	defer l.mu.Unlock()
	stored, moved, err := s.storedSince(ctx, key, loadGen)
	if err != nil {
		return nil, false, err
	}
	err = s.Storage.DeleteWithContext(ctx, string(key))
	l.gen.Add(1)
	return stored, moved && stored == nil, err
}

// storedSince returns what key holds when another request wrote its shard since loadGen and
// reports that one did; nil and false when none did. The caller holds the shard's lock.
//
//nolint:gocritic // returning explicit values keeps the signature concise while avoiding unnecessary named results
func (s *Store) storedSince(ctx context.Context, key storageKey, loadGen uint64) ([]byte, bool, error) {
	if key.lock().gen.Load() == loadGen {
		return nil, false, nil
	}
	raw, err := s.Storage.GetWithContext(ctx, string(key))
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

// GetByID retrieves a session by its ID from the storage.
// If the session is not found, it returns nil and an error.
//
// Unlike session middleware methods, this function does not automatically:
//
//   - Load the session into the request context.
//
//   - Save the session data to the storage or update the client cookie.
//
// Important Notes:
//
//   - The session object returned by GetByID does not have a context associated with it.
//
//   - When using this method alongside session middleware, there is a potential for collisions,
//     so be mindful of interactions between manually retrieved sessions and middleware-managed sessions.
//
//   - If you modify a session returned by GetByID, you must call session.Save() to persist the changes.
//
//   - When you are done with the session, you should call session.Release() to release the session back to the pool.
//
// Parameters:
//   - id: The unique identifier of the session.
//
// Returns:
//   - *Session: The session object if found; otherwise, nil.
//   - error: An error if the session retrieval fails or if the session ID is empty.
//
// Usage:
//
//	sess, err := store.GetByID(id)
//	if err != nil {
//	    // handle error
//	}
func (s *Store) GetByID(ctx context.Context, id string) (*Session, error) {
	if id == "" {
		return nil, ErrEmptySessionID
	}

	var found storedSession
	if err := s.readSession(ctx, id, &found); err != nil {
		return nil, err
	}
	if found.raw == nil {
		return nil, ErrSessionIDNotFoundInStore
	}

	sess := acquireSession()

	sess.mu.Lock()

	sess.bind(s)
	sess.id = found.id // the caller may pass a view of the request, such as c.Cookies
	sess.key = found.key
	sess.isFresh = false
	sess.loaded = true
	sess.loadGen = found.gen

	decodeErr := sess.decodeSessionData(found.raw)
	sess.mu.Unlock()
	if decodeErr != nil {
		sess.Release()
		return nil, fmt.Errorf("failed to decode session data: %w", decodeErr)
	}

	// Whatever AbsoluteTimeout the store has now, as on the request path: the
	// session was stored with a deadline, and it has passed.
	if sess.isAbsExpired() {
		if err := sess.Destroy(); err != nil { //nolint:contextcheck // it is not right
			sess.Release()
			log.Errorf("failed to destroy session: %v", err)
		}
		return nil, ErrSessionIDNotFoundInStore
	}

	return sess, nil
}

// sessionIDKey is the local key type used to store and retrieve the session ID in context.
type sessionIDKey int

const (
	// sessionIDContextKey is the key used to store the session ID in the context locals:
	// the ID of the store or middleware that cached one last, which the logger shows.
	// Nothing reads it to find a session.
	sessionIDContextKey sessionIDKey = iota
	// cachedIDsContextKey stores the IDs that Store.Get cached on the request, see cachedID.
	cachedIDsContextKey
)

// sessionIdentity is the ID of a session with the provenance of the ID the request
// named when the session was loaded, if any, which says where the ID is written back
// to. The two go together wherever an ID is taken from a request or handed to a store.
type sessionIdentity struct {
	id  string
	res extractors.Result
}

// cachedID is the identity of a session that a store cached on the request, because it
// issued the ID for a visitor the request names no session of. A store that is asked
// again takes its ID, and finds the session it saved under it.
//
// The IDs of a request are a list, newest first, with one for each store: a store that
// caches another replaces its first. The request holds the head behind a pointer,
// since storing a Result by value would box it onto the heap, and fasthttp hands the
// list back through Close on request reset, as it does the extractors package's chain
// state.
type cachedID struct {
	next  *cachedID
	store *Store
	sessionIdentity
}

var cachedIDPool = sync.Pool{New: func() any { return new(cachedID) }}

// Close returns the list that starts at head to the pool. fasthttp calls it on every
// request-local io.Closer when it resets the request; callers should not.
func (head *cachedID) Close() error {
	for entry := head; entry != nil; {
		next := entry.next
		*entry = cachedID{}
		cachedIDPool.Put(entry)
		entry = next
	}
	return nil
}

// of returns the entry of s in the list that starts at head, nil if s has none.
func (head *cachedID) of(s *Store) *cachedID {
	for entry := head; entry != nil; entry = entry.next {
		if entry.store == s {
			return entry
		}
	}
	return nil
}

// requestIDs returns the head of the IDs that Store.Get cached on the request, newest
// first, nil if it cached none.
func requestIDs(c fiber.Ctx) *cachedID {
	head, ok := c.Locals(cachedIDsContextKey).(*cachedID)
	if !ok {
		return nil
	}
	return head
}

// cacheID records that s issued ident for the request, as the newest of its IDs, and
// as the one the logger shows. The value of the provenance in ident points into the
// buffer of the request, which the application can rewrite, so the entry keeps a copy.
// The entry s had is reused as the new head, so that the list, which fasthttp reclaims
// from its head, stays in one piece: overwriting the local with an entry that does not
// lead to the old ones would strand them outside the pool.
func cacheID(c fiber.Ctx, s *Store, ident sessionIdentity) {
	head := requestIDs(c)
	ident.res.Value = utils.CopyString(ident.res.Value)
	var entry *cachedID
	for link := &head; *link != nil; link = &(*link).next {
		if (*link).store == s {
			entry, *link = *link, (*link).next
			break
		}
	}
	if entry == nil {
		var ok bool
		if entry, ok = cachedIDPool.Get().(*cachedID); !ok || entry == nil {
			entry = new(cachedID)
		}
	}
	entry.next, entry.store, entry.sessionIdentity = head, s, ident
	ctxlocal.Set(c, cachedIDsContextKey, entry)
	ctxlocal.Set(c, sessionIDContextKey, ident.id)
}

// recacheID records that the session of s that had the ID old has the identity ident
// now, which Regenerate and Reset give it, unless s has cached the ID of another session
// since: a session of the store that was never saved must not hide one that was.
func recacheID(c fiber.Ctx, s *Store, old string, ident sessionIdentity) {
	if entry := requestIDs(c).of(s); entry != nil && entry.id != old {
		return
	}
	cacheID(c, s, ident)
}

// extractorRoot identifies an extractor well enough to notice that the Extractor of
// a store was replaced after NewStore, which Config being embedded allows: it holds
// what the cookies and headers of an extractor depend on and tells one extractor
// from another without calling it, the key, source and authentication scheme of the
// root (the fields sourceOf reads) and the identity of its chain (a chain reports the
// key and source of its first child, so these alone cannot tell a leaf from a chain
// that starts with it). Changing a nested extractor in place is not noticed.
type extractorRoot struct {
	key    string
	scheme string
	chain  []extractors.Extractor
	source extractors.Source
}

func rootOf(ext extractors.Extractor) extractorRoot {
	return extractorRoot{key: ext.Key, scheme: ext.AuthScheme, chain: ext.Chain, source: ext.Source}
}

// matches reports whether ext is the extractor r was taken from.
func (r extractorRoot) matches(ext extractors.Extractor) bool {
	if r.key != ext.Key || r.scheme != ext.AuthScheme || r.source != ext.Source || len(r.chain) != len(ext.Chain) {
		return false
	}
	return len(r.chain) == 0 || &r.chain[0] == &ext.Chain[0]
}

// writesBack reports whether a session ID can be written back to a source of this
// kind: only a cookie or a header names a place the response can carry it to.
func writesBack(kind extractors.Source) bool {
	return kind == extractors.SourceCookie || kind == extractors.SourceHeader
}

// leaves calls visit for every extractor of ext that reads a source itself, in order.
// The tree is walked rather than ranged over Chain: a nested chain reports the metadata
// of its first child, so the direct children alone hide every leaf inside it, and a
// cookie one level down would never be found.
func leaves(ext extractors.Extractor, visit func(leaf extractors.Extractor)) {
	ext.Walk(func(candidate extractors.Extractor) bool {
		if len(candidate.Chain) == 0 {
			visit(candidate)
		}
		return true
	})
}

// collectSinks lists the cookies and headers an extractor reads a session ID from,
// which are the ones a session ID can be written back to: the leaves of the
// extractor that name a cookie or a header.
func collectSinks(ext extractors.Extractor) []extractors.Extractor {
	var sinks []extractors.Extractor
	leaves(ext, func(leaf extractors.Extractor) {
		// A keyless leaf would name a cookie or header with no name at all.
		if leaf.Key != "" && writesBack(leaf.Source) {
			sinks = append(sinks, leaf)
		}
	})
	return sinks
}

// sourceID names a source a session ID is read from: the kind of source and the
// name in it, such as the cookie "session_id", the header "X-Session" or the query
// parameter "sid". For the Authorization header the name is the scheme, since the
// header is always the same. A source without a name is named by its kind alone, so
// two custom extractors without a key count as one source.
type sourceID struct {
	name string
	kind extractors.Source
}

// sourceOf names a leaf extractor.
func sourceOf(leaf extractors.Extractor) sourceID {
	if leaf.Source == extractors.SourceAuthHeader {
		return sourceID{kind: leaf.Source, name: leaf.AuthScheme}
	}
	return sourceID{kind: leaf.Source, name: leaf.Key}
}

// same reports whether a and b are one source: the same kind and the same name,
// which for a header or an authentication scheme is compared without regard to case.
func (a sourceID) same(b sourceID) bool {
	if a.kind != b.kind {
		return false
	}
	switch a.kind {
	case extractors.SourceHeader, extractors.SourceAuthHeader:
		return utils.EqualFold(a.name, b.name)
	default:
		return a.name == b.name
	}
}

// collectSources names every source an extractor reads a session ID from, as
// sharesSource compares them: the leaves of the extractor.
func collectSources(ext extractors.Extractor) []sourceID {
	var sources []sourceID
	leaves(ext, func(leaf extractors.Extractor) {
		sources = append(sources, sourceOf(leaf))
	})
	return sources
}

// idPlaces are the places a store reads its session ID from and writes it back to.
type idPlaces struct {
	// sinks are the cookies and headers a session ID can be written back to, in the
	// order of the extractor. Read-only: every session of the store shares the slice.
	sinks []extractors.Extractor
	// sources names every source the store reads the session ID from, as shares
	// compares them.
	sources []sourceID
}

// placesOf works out the places of ext.
func placesOf(ext extractors.Extractor) idPlaces {
	return idPlaces{sinks: collectSinks(ext), sources: collectSources(ext)}
}

// shares reports whether p and other have a source in common, by kind and name, or
// cannot be told apart because one of them names none.
func (p *idPlaces) shares(other *idPlaces) bool {
	if len(p.sources) == 0 || len(other.sources) == 0 {
		return true
	}
	return slices.ContainsFunc(p.sources, func(a sourceID) bool {
		return slices.ContainsFunc(other.sources, a.same)
	})
}

// owns reports whether res names a cookie or header that is one of the sinks.
func (p *idPlaces) owns(res extractors.Result) bool {
	source := sourceID{kind: res.Source, name: res.Key}
	return slices.ContainsFunc(p.sinks, func(sink extractors.Extractor) bool {
		return sourceOf(sink).same(source)
	})
}

// adopt returns res, the provenance of a session ID that another store cached on the
// request, as it holds for a store with these places. A cookie or header that is not
// one of the sinks is not its to write the ID back to: the ID is then treated as one
// from a source that cannot be written back to, which is what Resolved false means to
// getExtractorInfo. A read-only source, and a sink of its own, stay as they are.
func (p *idPlaces) adopt(res extractors.Result) extractors.Result {
	if res.Resolved && writesBack(res.Source) && !p.owns(res) {
		res.Resolved = false
	}
	return res
}

// extractorProfile is what a store works out from its Extractor once, so that no request
// has to walk it. Config is embedded in the Store, so the Extractor can be replaced after
// NewStore; root tells that, and the store then walks the extractor it has now.
type extractorProfile struct {
	places idPlaces
	// root identifies the extractor that places were worked out from.
	root extractorRoot
	// ready is set by profileOf: the zero value of a Store built without NewStore is
	// not a profile of an extractor that has no sinks.
	ready bool
}

// profileOf works out the profile of ext.
func profileOf(ext extractors.Extractor) extractorProfile {
	return extractorProfile{root: rootOf(ext), places: placesOf(ext), ready: true}
}

// current reports whether the profile is that of ext: it was worked out, and ext is
// the extractor it was worked out from.
func (p *extractorProfile) current(ext extractors.Extractor) bool {
	return p.ready && p.root.matches(ext)
}
