package session

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/maphash"
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

// cachedID is a session ID that a store cached on the request, because it issued it
// for a visitor the request names no session of, with the provenance of the ID the
// request did name, if any, which says where the new one is written back to. A store
// that is asked again takes its ID, and finds the session it saved under it.
//
// The IDs of a request are a list, newest first, with one for each store: a store that
// caches another replaces its first. The request holds the head behind a pointer,
// since storing a Result by value would box it onto the heap, and fasthttp hands the
// list back through Close on request reset, as it does the extractors package's chain
// state.
type cachedID struct {
	next  *cachedID
	store *Store
	id    string
	res   extractors.Result
}

// A pointer so a test can swap in a pool of its own; nothing else reassigns it.
var cachedIDPool = &sync.Pool{New: func() any { return new(cachedID) }}

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

// cacheID records that s issued id for the request, as the newest of its IDs, and
// as the one the logger shows. head is the list of the request, res the provenance of
// the ID the request named, if any. The entry s had is reused as the new head, so that
// the list, which fasthttp reclaims from its head, stays in one piece: overwriting the
// local with an entry that does not lead to the old ones would strand them outside the
// pool.
func cacheID(c fiber.Ctx, head *cachedID, s *Store, id string, res extractors.Result) {
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
	entry.next, entry.store, entry.id, entry.res = head, s, id, res
	ctxlocal.Set(c, cachedIDsContextKey, entry)
	ctxlocal.Set(c, sessionIDContextKey, id)
}

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

// Store manages session data using the configured storage backend.
type Store struct {
	// named is the Codec with its name, asked once. It is nil for a store without a
	// Codec, and for one built without NewStore, which sessionFormat makes up. Changing
	// the Codec of a Store after NewStore has no effect.
	named *namedCodec
	// locals is the PassLocalsToContext setting of the app that served the store last,
	// see passesLocalsToContext.
	locals atomic.Pointer[localsSetting]
	// sinks are the cookies and headers a session ID can be written back to, in the
	// order of the extractor. NewStore works them out once; a Store built without it
	// has none (nil), and a Store whose Extractor was replaced since has stale ones:
	// both walk the extractor when they need them. Read-only: every session of the
	// store shares the slice.
	sinks []extractors.Extractor
	// sources names every source the store reads the session ID from, as
	// sharesSource compares them.
	sources []sourceID
	// root identifies the extractor that sinks and sources were worked out from.
	root extractorRoot
	Config
	// unreadableWarned is set once the store has said that it met a session it
	// cannot read.
	unreadableWarned atomic.Bool
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
//   - string: The key to use with the storage.
func (s *Store) storageKey(id string) string {
	// The default is the common case, and kept small enough to inline.
	if s.StorageKeyPrefix == "" && !s.HashStorageKeys {
		return keyNamespace + id
	}
	return s.derivedStorageKey(id)
}

// derivedStorageKey is storageKey for a store with a prefix or hashing.
func (s *Store) derivedStorageKey(id string) string {
	if !s.HashStorageKeys {
		return s.StorageKeyPrefix + keyNamespace + id
	}
	sum := sha256.Sum256(utils.UnsafeBytes(id))
	// One buffer of the exact size, handed over as the key: nothing writes to it
	// afterwards, and it spares the second allocation a string conversion costs.
	key := make([]byte, 0, len(s.StorageKeyPrefix)+len(keyNamespace)+hashedKeyLen)
	key = append(key, s.StorageKeyPrefix...)
	key = append(key, keyNamespace...)
	key = base64.RawURLEncoding.AppendEncode(key, sum[:])
	return utils.UnsafeString(key)
}

// ownedID returns id as a string that does not point into the request: the end of
// the key when the key holds the ID, a copy when it holds its digest. A session
// outlives the request that named it, and the buffer of the request is reused.
func (s *Store) ownedID(key, id string) string {
	if s.HashStorageKeys {
		return utils.CopyString(id)
	}
	return key[len(key)-len(id):]
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

	sinks := collectSinks(cfg.Extractor)
	if sinks == nil {
		// Worked out, and none: not the nil that marks a Store built without NewStore.
		sinks = []extractors.Extractor{}
	}
	store := &Store{
		Config:  cfg,
		root:    rootOf(cfg.Extractor),
		sinks:   sinks,
		sources: collectSources(cfg.Extractor),
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
	_, ok := c.Locals(middlewareContextKey).(*Middleware)
	if ok {
		return nil, ErrSessionAlreadyLoadedByMiddleware
	}

	sess, cached, err := s.loadSession(c, nil)
	if err == nil && sess.isFresh {
		// Nothing the request carries names the ID that was just issued: say so, for
		// the next store that is asked, which could not find it there.
		cacheID(c, cached, s, sess.id, sess.extractor)
	}
	return sess, err
}

// loadSession retrieves a session based on the context. The session is the one saved
// under an ID that a store sharing a source with s cached on the request, the newest
// first, or else the one the request names; a new one if there is neither. It returns
// the IDs that Store.Get cached on the request, so that Store.Get can add to them.
//
// Parameters:
//   - c: The Fiber context.
//   - outer: The session middleware the request currently holds, nil if it holds none.
//
// Returns:
//   - *Session: The session object.
//   - *cachedID: The IDs cached on the request, newest first, nil if there are none.
//   - error: An error if the session retrieval fails.
//
// Usage:
//
//	sess, _, err := store.loadSession(c, nil)
//	if err != nil {
//	    // handle error
//	}
func (s *Store) loadSession(c fiber.Ctx, outer *Middleware) (*Session, *cachedID, error) {
	var found storedSession
	cached := requestIDs(c)
	if outer != nil || cached != nil {
		if err := s.cachedSession(c, outer, cached, &found); err != nil {
			return nil, nil, err
		}
	}
	if found.raw == nil {
		// Nothing was cached for s that is saved: the request names the session, if any.
		res, err := s.getSessionID(c)
		if err != nil {
			return nil, nil, err
		}
		found.res = res
		if res.Value != "" && !s.foundNoSession(cached, res.Value) {
			if readErr := s.readSession(c, res.Value, &found); readErr != nil {
				return nil, nil, readErr
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
		sess.data.Lock()
		err := sess.decodeSessionData(found.raw)
		sess.data.Unlock()
		if err != nil {
			sess.mu.Unlock()
			sess.Release()
			return nil, nil, fmt.Errorf("failed to decode session data: %w", err)
		}
		sess.loaded = true
		sess.loadGen = found.gen
	}

	sess.mu.Unlock()

	if isFresh && s.AbsoluteTimeout > 0 {
		sess.setAbsExpiration(time.Now().Add(s.AbsoluteTimeout))
	} else if sess.isAbsExpired() {
		// Whatever AbsoluteTimeout the store has now: the session was stored with a
		// deadline, and it has passed. Reset arms the session that takes its place
		// when the store has an AbsoluteTimeout; a store without one gives it none,
		// where a deadline of now would make the next load start it over again.
		if err := sess.Reset(); err != nil {
			return nil, nil, fmt.Errorf("failed to reset session: %w", err)
		}
	}

	return sess, cached, nil
}

// foundNoSession reports whether s looked for a session under id earlier in the request
// and found none: the entry s cached holds the ID the request named when s issued its own,
// or the one a Regenerate or Reset deleted, and neither names a session.
func (s *Store) foundNoSession(cached *cachedID, id string) bool {
	for entry := cached; entry != nil; entry = entry.next {
		if entry.store == s {
			return entry.res.Value == id
		}
	}
	return false
}

// storedSession is the outcome of a lookup of a session: the record, if a session is
// stored under the ID that was looked up, and where that ID came from.
type storedSession struct {
	raw []byte // the record, nil if no session is stored under the ID
	key string // storage key of the session
	id  string // ID of the session, which does not point into the request
	// res is the provenance of the ID, which says where it is written back to. It is
	// set when no session is stored under the ID the request named, too: the new
	// session is written back to the same place.
	res extractors.Result
	gen uint64 // idLock generation read before the storage
}

// readSession looks for the session with the given ID in the storage and, if one is
// stored, fills found with it. A record the store cannot read counts as none.
func (s *Store) readSession(c fiber.Ctx, id string, found *storedSession) error {
	key := s.storageKey(id)
	gen := lockFor(key).gen.Load()
	raw, err := s.Storage.GetWithContext(c, key)
	if err != nil || raw == nil || s.refusesRecord(raw) {
		return err
	}
	// Not the ID as given: it may point into the request buffer the next request reuses.
	found.raw, found.key, found.id, found.gen = raw, key, s.ownedID(key, id), gen
	return nil
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

func (s *Store) deleteKey(ctx context.Context, key string) error {
	l := lockFor(key)
	l.mu.Lock()
	defer l.mu.Unlock()
	err := s.Storage.DeleteWithContext(ctx, key)
	l.gen.Add(1) // after the delete, so a load that sees the new value cannot read the entry
	return err
}

// deleteLoadedKey is deleteKey for a session read at loadGen; it reports whether
// another request had deleted the key since.
func (s *Store) deleteLoadedKey(ctx context.Context, key string, loadGen uint64) (bool, error) {
	l := lockFor(key)
	l.mu.Lock()
	defer l.mu.Unlock()
	ended := false
	if l.gen.Load() != loadGen {
		raw, err := s.Storage.GetWithContext(ctx, key)
		if err != nil {
			return false, err
		}
		ended = raw == nil
	}
	err := s.Storage.DeleteWithContext(ctx, key)
	l.gen.Add(1)
	return ended, err
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

	key := s.storageKey(id)
	loadGen := lockFor(key).gen.Load()
	rawData, err := s.Storage.GetWithContext(ctx, key)
	if err != nil {
		return nil, err
	}
	if rawData == nil {
		return nil, ErrSessionIDNotFoundInStore
	}
	if s.refusesRecord(rawData) {
		return nil, ErrSessionIDNotFoundInStore
	}

	sess := acquireSession()

	sess.mu.Lock()

	sess.bind(s)
	sess.id = s.ownedID(key, id) // the caller may pass a view of the request, such as c.Cookies
	sess.key = key
	sess.isFresh = false
	sess.loaded = true
	sess.loadGen = loadGen

	sess.data.Lock()
	decodeErr := sess.decodeSessionData(rawData)
	sess.data.Unlock()
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
