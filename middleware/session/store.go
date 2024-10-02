package session

import (
	"encoding/gob"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
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

type Store struct {
	Config
}

// New creates a new session store with the provided configuration.
//
// Parameters:
//   - config: Variadic parameter to override default config.
//
// Returns:
//   - *Store: The session store.
//
// Usage:
//
//	store := session.New()
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
	if !ok {
		id = s.getSessionID(c)
	}

	fresh := ok // Assume the session is fresh if the ID is found in locals

	// Attempt to fetch session data if an ID is provided
	if id != "" {
		rawData, err = s.Storage.Get(id)
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

// getSessionID returns the session ID from cookies, headers, or query string.
//
// Parameters:
//   - c: The Fiber context.
//
// Returns:
//   - string: The session ID.
//
// Usage:
//
//	id := store.getSessionID(c)
func (s *Store) getSessionID(c fiber.Ctx) string {
	id := c.Cookies(s.sessionName)
	if len(id) > 0 {
		return utils.CopyString(id)
	}

	if s.source == SourceHeader {
		id = string(c.Request().Header.Peek(s.sessionName))
		if len(id) > 0 {
			return id
		}
	}

	if s.source == SourceURLQuery {
		id = fiber.Query[string](c, s.sessionName)
		if len(id) > 0 {
			return utils.CopyString(id)
		}
	}

	return ""
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
func (s *Store) Reset() error {
	return s.Storage.Reset()
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
func (s *Store) Delete(id string) error {
	if id == "" {
		return ErrEmptySessionID
	}
	return s.Storage.Delete(id)
}

// GetByID retrieves a session by its ID from the storage.
// If the session is not found, it returns nil and an error.
//
// Note:
// - Unlike session Middleware methods, Session methods do not automatically:
//   - Load the session into the context
//   - Save the session data to the storage and update the client cookie
//
// - Be aware of possible collisions if you are also using the session in a middleware.
//
// Usage:
//   - If you modify a session returned by GetByID, you must call session.Save() to persist the changes.
//   - When you are done with the session, you should call session.Release() to release the session back to the pool.
//
// Parameters:
//   - id: The unique identifier of the session.
//
// Returns:
//   - *Session: The session object if found, otherwise nil.
//   - error: An error if the session retrieval fails or if the session ID is empty.
//
// Usage:
//
//	sess, err := store.GetByID(id)
//	if err != nil {
//	    // handle error
//	}
func (s *Store) GetByID(id string) (*Session, error) {
	if id == "" {
		return nil, ErrEmptySessionID
	}

	rawData, err := s.Storage.Get(id)
	if err != nil {
		return nil, err
	}
	if rawData == nil {
		return nil, ErrSessionIDNotFoundInStore
	}

	sess := acquireSession()

	sess.mu.Lock()

	sess.id = id
	sess.config = s

	sess.data.Lock()
	decodeErr := sess.decodeSessionData(rawData)
	sess.data.Unlock()
	if decodeErr != nil {
		sess.mu.Unlock()
		return nil, fmt.Errorf("failed to decode session data: %w", err)
	}
	sess.mu.Unlock()

	if s.AbsoluteTimeout > 0 {
		if sess.isAbsExpired() {
			err := sess.config.Storage.Delete(sess.ID())
			sess.Release()
			if err != nil {
				log.Errorf("failed to delete expired session: %v", err)
			}
			return nil, ErrSessionIDNotFoundInStore
		}
	}

	return sess, nil
}
