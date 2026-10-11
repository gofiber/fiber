package session

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/ctxlocal"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/log"
)

// ErrEmptySessionID is an error that occurs when the session ID is empty.
var (
	ErrEmptySessionID                   = errors.New("session ID cannot be empty")
	ErrSessionAlreadyLoadedByMiddleware = errors.New("session already loaded by middleware")
	ErrSessionIDNotFoundInStore         = errors.New("session ID not found in session store")
)

// sessionIDKey is the local key type used to store and retrieve the session ID in context.
type sessionIDKey int

const (
	// sessionIDContextKey is the key used to store the session ID in the context locals.
	sessionIDContextKey sessionIDKey = iota
	// sessionExtractorContextKey stores the extractor that provided the session ID.
	sessionExtractorContextKey
)

// provenanceBox carries the resolved provenance through request locals behind a
// pointer: storing the Result by value would box it onto the heap on every
// request carrying a session ID. fasthttp hands it back through Close on
// request reset, as it does the extractors package's chain state.
type provenanceBox struct {
	res extractors.Result
}

// A pointer so a test can swap in a pool of its own; nothing else reassigns it.
var provenancePool = &sync.Pool{New: func() any { return new(provenanceBox) }}

// storeProvenance records res in the request, reusing the box already there.
// Overwriting the local with a fresh box would strand the old one outside the
// pool: fasthttp only reclaims the box the local currently holds.
func storeProvenance(c fiber.Ctx, res extractors.Result) {
	if b, ok := c.Locals(sessionExtractorContextKey).(*provenanceBox); ok && b != nil {
		b.res = res
		return
	}
	b, ok := provenancePool.Get().(*provenanceBox)
	if !ok || b == nil {
		b = new(provenanceBox)
	}
	b.res = res
	ctxlocal.Set(c, sessionExtractorContextKey, b)
}

// Close returns the box to the pool. fasthttp calls it on every request-local
// io.Closer when it resets the request; callers should not.
func (b *provenanceBox) Close() error {
	b.res = extractors.Result{}
	provenancePool.Put(b)
	return nil
}

// Store manages session data using the configured storage backend.
type Store struct {
	Config
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

	if cfg.Storage == nil {
		cfg.Storage = memory.New()
	}

	store := &Store{
		Config: cfg,
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

	return s.getSession(c)
}

// getSession retrieves a session based on the context.
//
// Parameters:
//   - c: The Fiber context.
//
// Returns:
//   - *Session: The session object.
//   - error: An error if the session retrieval fails.
//
// Usage:
//
//	sess, err := store.getSession(c)
//	if err != nil {
//	    // handle error
//	}
func (s *Store) getSession(c fiber.Ctx) (*Session, error) {
	var rawData []byte
	var err error

	var selectedExtractor extractors.Result
	id, ok := c.Locals(sessionIDContextKey).(string)
	if !ok {
		var resolveErr error
		selectedExtractor, resolveErr = s.getSessionID(c)
		if resolveErr != nil {
			return nil, resolveErr
		}
		id = selectedExtractor.Value
		if id != "" {
			// Kept for a second getSession on the same request, which takes the
			// cached-ID path above and would otherwise lose the provenance.
			storeProvenance(c, selectedExtractor)
		}
	} else if stored, found := c.Locals(sessionExtractorContextKey).(*provenanceBox); found && stored != nil {
		selectedExtractor = stored.res
	}

	isFresh := false // Session is not fresh initially; only set to true if we generate a new ID

	// Attempt to fetch session data if an ID is provided
	if id != "" {
		rawData, err = s.Storage.GetWithContext(c, id)
		if err != nil {
			return nil, err
		}
		if rawData == nil {
			// Data not found, prepare to generate a new session
			id = ""
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
	sess.config = s
	sess.id = id
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
			return nil, fmt.Errorf("failed to decode session data: %w", err)
		}
	}

	sess.mu.Unlock()

	if isFresh && s.AbsoluteTimeout > 0 {
		sess.setAbsExpiration(time.Now().Add(s.AbsoluteTimeout))
	} else if sess.isAbsExpired() {
		if err := sess.Reset(); err != nil {
			return nil, fmt.Errorf("failed to reset session: %w", err)
		}
		sess.setAbsExpiration(time.Now().Add(s.AbsoluteTimeout))
	}

	return sess, nil
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
	return s.Storage.DeleteWithContext(ctx, id)
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

	rawData, err := s.Storage.GetWithContext(ctx, id)
	if err != nil {
		return nil, err
	}
	if rawData == nil {
		return nil, ErrSessionIDNotFoundInStore
	}

	sess := acquireSession()

	sess.mu.Lock()

	sess.config = s
	sess.id = id
	sess.isFresh = false

	sess.data.Lock()
	decodeErr := sess.decodeSessionData(rawData)
	sess.data.Unlock()
	sess.mu.Unlock()
	if decodeErr != nil {
		sess.Release()
		return nil, fmt.Errorf("failed to decode session data: %w", decodeErr)
	}

	if s.AbsoluteTimeout > 0 {
		if sess.isAbsExpired() {
			if err := sess.Destroy(); err != nil { //nolint:contextcheck // it is not right
				sess.Release()
				log.Errorf("failed to destroy session: %v", err)
			}
			return nil, ErrSessionIDNotFoundInStore
		}
	}

	return sess, nil
}
