package session

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
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
)

// sessionIDInfo bundles the resolved session ID with the extractor source that
// produced it. Both pieces are cached together in the request locals so that a
// second Store.Get within the same request returns a consistent answer — in
// particular, chained extractors keep their original source decision instead of
// being re-derived from the wrapper Extractor.Source.
type sessionIDInfo struct {
	id     string
	source extractors.Source
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

	// Resolve the session ID and the source that produced it. The pair is cached
	// in the request locals so a second call within the same request returns the
	// same answer — including for chained extractors where the source is decided
	// at extraction time and would otherwise be lost.
	info, alreadyResolved := c.Locals(sessionIDContextKey).(sessionIDInfo)
	if !alreadyResolved {
		info = s.resolveSessionID(c)
		c.Locals(sessionIDContextKey, info)
	}
	id := info.id

	fresh := false // Session is not fresh initially; only set to true if we generate a new ID

	// Attempt to fetch session data if an ID is provided
	if id != "" {
		rawData, err = s.Storage.GetWithContext(c, id)
		if err != nil {
			return nil, err
		}
		if rawData == nil {
			switch {
			case alreadyResolved:
				// A prior call within this request already committed to this ID.
				// Keep it so multiple Store.Get calls in the same request observe
				// the same session.
				fresh = true
			case s.acceptClientID(info):
				// Read-only source with an opt-in trusted client ID — preserve so
				// that subsequent requests carrying the same ID load the same
				// session.
				fresh = true
			default:
				// Writable source (cookie/header) with an unknown ID, or
				// untrusted read-only ID — discard and generate a fresh one to
				// prevent session fixation and storage poisoning.
				id = ""
			}
		}
	}

	// Generate a new ID if needed
	if id == "" {
		fresh = true // The session is fresh if a new ID is generated
		id = s.KeyGenerator()
		// Mark the cached source as cookie so the regenerated ID is treated as
		// server-issued (writable) on any subsequent call within this request.
		c.Locals(sessionIDContextKey, sessionIDInfo{id: id, source: extractors.SourceCookie})
	}

	// Create session object
	sess := acquireSession()

	sess.mu.Lock()

	sess.ctx = c
	sess.config = s
	sess.id = id
	sess.fresh = fresh

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

	if fresh && s.AbsoluteTimeout > 0 {
		sess.setAbsExpiration(time.Now().Add(s.AbsoluteTimeout))
	} else if sess.isAbsExpired() {
		if err := sess.Reset(); err != nil {
			return nil, fmt.Errorf("failed to reset session: %w", err)
		}
		sess.setAbsExpiration(time.Now().Add(s.AbsoluteTimeout))
	}

	return sess, nil
}

// resolveSessionID extracts the session ID from the request and reports the
// source that produced it. For chained extractors the sub-extractors are tried
// in order so the source of the first one that yields a value wins; for a
// single extractor the source on the wrapper is used. When extraction fails the
// returned ID is empty and the source falls back to the wrapper's source.
//
// Parameters:
//   - c: The Fiber context.
//
// Returns:
//   - sessionIDInfo: The resolved ID together with its originating source.
func (s *Store) resolveSessionID(c fiber.Ctx) sessionIDInfo {
	ext := s.Extractor

	if len(ext.Chain) > 0 {
		for _, chainExt := range ext.Chain {
			if chainExt.Extract == nil {
				continue
			}
			v, err := chainExt.Extract(c)
			if err == nil && v != "" {
				return sessionIDInfo{id: v, source: chainExt.Source}
			}
		}
		return sessionIDInfo{source: ext.Source}
	}

	v, err := ext.Extract(c)
	if err != nil {
		return sessionIDInfo{source: ext.Source}
	}
	return sessionIDInfo{id: v, source: ext.Source}
}

// acceptClientID reports whether a client-supplied session ID from a read-only
// source should be persisted as-is. Writable sources (cookie/header) are never
// accepted here — they are subject to fixation protection. For read-only
// sources the application must explicitly opt in via TrustClientSessionID and
// supply a ClientSessionIDValidator that accepts the ID; otherwise the ID is
// rejected and a server-generated one is used.
func (s *Store) acceptClientID(info sessionIDInfo) bool {
	if info.id == "" || info.source.IsWritable() {
		return false
	}
	if !s.TrustClientSessionID || s.ClientSessionIDValidator == nil {
		return false
	}
	return s.ClientSessionIDValidator(info.id)
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
	sess.fresh = false

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
