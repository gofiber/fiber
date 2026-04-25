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

	id, ok := c.Locals(sessionIDContextKey).(string)

	// writableSource tracks whether the session ID came from a writable source
	// (cookie or header). For writable sources, an unknown ID is discarded and a new
	// one is generated to prevent session fixation. For read-only sources (query,
	// form, param, custom), the client-provided ID is preserved so that subsequent
	// requests using the same query parameter are associated with the same session.
	var writableSource bool
	if !ok {
		id, writableSource = s.getSessionID(c)
	} else {
		// ID was cached from a prior call within this request; derive writability
		// from the primary (first or only) extractor source.
		src := s.Extractor.Source
		writableSource = src == extractors.SourceCookie || src == extractors.SourceHeader
	}

	fresh := false // Session is not fresh initially; only set to true if we generate a new ID

	// Attempt to fetch session data if an ID is provided
	if id != "" {
		rawData, err = s.Storage.GetWithContext(c, id)
		if err != nil {
			return nil, err
		}
		if rawData == nil {
			if writableSource {
				// For writable sources (cookie, header), discard the client-provided
				// ID and generate a new one to prevent session fixation attacks.
				id = ""
			} else {
				// For read-only sources (query, form, param, custom), preserve the
				// client-provided ID and create a fresh session under it so that
				// subsequent requests carrying the same ID are served the same session.
				fresh = true
				c.Locals(sessionIDContextKey, id)
			}
		}
	}

	// Generate a new ID if needed
	if id == "" {
		fresh = true // The session is fresh if a new ID is generated
		id = s.KeyGenerator()
		c.Locals(sessionIDContextKey, id)
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

// getSessionID returns the session ID using the configured extractor, and whether
// the extraction source is writable (cookie or header). A writable source means
// the middleware sets the session ID back in the response (e.g. Set-Cookie), so
// an unrecognized client-supplied ID should be discarded to prevent session
// fixation. A non-writable source (query, form, param, custom) is read-only; the
// client controls the ID on every request, so an unrecognized ID is preserved and
// a fresh session is stored under it.
//
// For chained extractors the function iterates the sub-extractors in order and
// returns the source of the first one that provides a value.
//
// Parameters:
//   - c: The Fiber context.
//
// Returns:
//   - string: The session ID.
//   - bool: true when the source is writable (cookie or header).
//
// Usage:
//
//	id, writable := store.getSessionID(c)
func (s *Store) getSessionID(c fiber.Ctx) (string, bool) {
	isWritable := func(src extractors.Source) bool {
		return src == extractors.SourceCookie || src == extractors.SourceHeader
	}

	ext := s.Extractor

	// For chained extractors, try each sub-extractor in order so we can identify
	// which source actually provided the value.
	if len(ext.Chain) > 0 {
		for _, chainExt := range ext.Chain {
			if chainExt.Extract == nil {
				continue
			}
			v, err := chainExt.Extract(c)
			if err == nil && v != "" {
				return v, isWritable(chainExt.Source)
			}
		}
		return "", false
	}

	// Single extractor.
	sessionID, err := ext.Extract(c)
	if err != nil {
		// If extraction fails, return empty string to generate a new session
		return "", false
	}
	return sessionID, isWritable(ext.Source)
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
