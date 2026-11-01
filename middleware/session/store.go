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
	// sessionIDContextKey is the key used to store the session ID in the context locals.
	sessionIDContextKey sessionIDKey = iota
	// sessionExtractorContextKey stores the extractor that provided the session ID.
	sessionExtractorContextKey
	// sessionOwnerContextKey stores the *Store whose Store.Get published the session
	// ID, so that a store reading other cookies or headers does not adopt it, when
	// the request has no provenance box to say so in. The session middleware
	// publishes itself instead, and it names the store.
	sessionOwnerContextKey
)

// provenanceBox carries the resolved provenance through request locals behind a
// pointer: storing the Result by value would box it onto the heap on every
// request carrying a session ID. fasthttp hands it back through Close on
// request reset, as it does the extractors package's chain state.
type provenanceBox struct {
	// owner is the store whose extractor produced res.
	owner *Store
	// publisher is the store whose Store.Get cached the session ID on the request,
	// when the request had a box to say so in; otherwise it is in a local of its own.
	publisher *Store
	res       extractors.Result
}

// A pointer so a test can swap in a pool of its own; nothing else reassigns it.
var provenancePool = &sync.Pool{New: func() any { return new(provenanceBox) }}

// storeProvenance records res, resolved by s, in the request, reusing the box
// already there, and returns the box. Overwriting the local with a fresh box would
// strand the old one outside the pool: fasthttp only reclaims the box the local
// currently holds.
func storeProvenance(c fiber.Ctx, s *Store, res extractors.Result) *provenanceBox {
	if b, ok := c.Locals(sessionExtractorContextKey).(*provenanceBox); ok && b != nil {
		b.res, b.owner = res, s
		return b
	}
	b, ok := provenancePool.Get().(*provenanceBox)
	if !ok || b == nil {
		b = new(provenanceBox)
	}
	b.res, b.owner = res, s
	ctxlocal.Set(c, sessionExtractorContextKey, b)
	return b
}

// Close returns the box to the pool. fasthttp calls it on every request-local
// io.Closer when it resets the request; callers should not.
func (b *provenanceBox) Close() error {
	b.res = extractors.Result{}
	b.owner = nil
	b.publisher = nil
	provenancePool.Put(b)
	return nil
}

// storageKeyPrefix keeps sessions apart from other middleware sharing the Storage:
// the ID comes from the request, so a bare one could name any of their entries.
const storageKeyPrefix = "session:"

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
	// Codec, and for one built without NewStore, which sessionCodec makes up. Changing
	// the Codec of a Store after NewStore has no effect.
	named *namedCodec
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
		return storageKeyPrefix + id
	}
	return s.derivedStorageKey(id)
}

// derivedStorageKey is storageKey for a store with a prefix or hashing.
func (s *Store) derivedStorageKey(id string) string {
	if !s.HashStorageKeys {
		return s.StorageKeyPrefix + storageKeyPrefix + id
	}
	sum := sha256.Sum256(utils.UnsafeBytes(id))
	// One buffer of the exact size, handed over as the key: nothing writes to it
	// afterwards, and it spares the second allocation a string conversion costs.
	key := make([]byte, 0, len(s.StorageKeyPrefix)+len(storageKeyPrefix)+hashedKeyLen)
	key = append(key, s.StorageKeyPrefix...)
	key = append(key, storageKeyPrefix...)
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

	sess, box, err := s.loadSession(c)
	if err == nil && sess.isFresh {
		// loadSession has just cached the new ID on the request; say whose it is.
		if box != nil {
			box.publisher = s
		} else {
			ctxlocal.Set(c, sessionOwnerContextKey, s)
		}
	}
	return sess, err
}

// loadSession retrieves a session based on the context, and returns the provenance
// box of the request when it has one, so that Store.Get can say in it whose session
// ID is cached.
//
// Parameters:
//   - c: The Fiber context.
//
// Returns:
//   - *Session: The session object.
//   - *provenanceBox: The provenance box of the request, nil if it has none.
//   - error: An error if the session retrieval fails.
//
// Usage:
//
//	sess, _, err := store.loadSession(c)
//	if err != nil {
//	    // handle error
//	}
func (s *Store) loadSession(c fiber.Ctx) (*Session, *provenanceBox, error) {
	var rawData []byte
	var err error

	var selectedExtractor extractors.Result
	id, ok := c.Locals(sessionIDContextKey).(string)
	var cached *provenanceBox
	// Whether the store that left the provenance box shares a cookie or header with
	// s. A store always shares with itself, and a request with a single store is by
	// far the common one, so the question is only asked of another store's box.
	ownerShares := true
	if ok {
		cached = requestProvenance(c)
		if cached != nil && cached.owner != nil && cached.owner != s {
			ownerShares = s.sharesSource(cached.owner)
		}
		// The owner of the box has a say when the cached ID is the one its extractor
		// supplied. When that one named no session, the ID cached since is a fresh one
		// of its publisher, and the box still describes the earlier one.
		if (!ownerShares && cached.res.Value == id) || !s.publisherShares(c, cached) {
			// Cached by a store over other cookies or headers: its ID is not ours.
			ok = false
		}
	}
	if !ok {
		var resolveErr error
		selectedExtractor, resolveErr = s.getSessionID(c)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}
		id = selectedExtractor.Value
		if id != "" {
			// Kept for a second loadSession on the same request, which takes the
			// cached-ID path above and would otherwise lose the provenance.
			cached = storeProvenance(c, s, selectedExtractor)
		}
	} else if cached != nil && ownerShares {
		// The provenance of the cached ID, which says where it is written back to. A
		// box that a store over other cookies or headers left describes an ID of its
		// own, and says nothing about the one cached since.
		selectedExtractor = s.adoptProvenance(cached.res)
	}

	isFresh := false // Session is not fresh initially; only set to true if we generate a new ID
	var loadGen uint64
	var key string

	// Attempt to fetch session data if an ID is provided
	if id != "" {
		key = s.storageKey(id)
		loadGen = lockFor(key).gen.Load()
		rawData, err = s.Storage.GetWithContext(c, key)
		if err != nil {
			return nil, nil, err
		}
		if s.refusesRecord(rawData) {
			rawData = nil
		}
		if rawData == nil {
			// Data not found, prepare to generate a new session
			id, key = "", ""
		} else {
			// Not the extracted ID: it points into the request buffer the next request reuses.
			id = s.ownedID(key, id)
		}
	}

	// Generate a new ID if needed
	if id == "" {
		isFresh = true // The session is fresh if a new ID is generated
		id = s.KeyGenerator()
		ctxlocal.Set(c, sessionIDContextKey, id)
	}

	// Create session object
	sess := acquireSession()

	sess.mu.Lock()

	sess.ctx = c
	sess.bind(s)
	sess.id = id
	sess.key = key
	sess.isFresh = isFresh
	sess.extractor = selectedExtractor

	// Decode session data if found
	if rawData != nil {
		sess.data.Lock()
		err := sess.decodeSessionData(rawData)
		sess.data.Unlock()
		if err != nil {
			sess.mu.Unlock()
			sess.Release()
			return nil, nil, fmt.Errorf("failed to decode session data: %w", err)
		}
		sess.loaded = true
		sess.loadGen = loadGen
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
