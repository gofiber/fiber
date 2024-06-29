package session

import (
	"encoding/gob"
	"errors"
	"fmt"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/storage/memory"
	"github.com/gofiber/fiber/v2/utils"
)

// ErrEmptySessionID is an error that occurs when the session ID is empty.
var ErrEmptySessionID = errors.New("session id cannot be empty")

// mux is a global mutex for session operations.
var mux sync.Mutex

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
func New(config ...Config) *Store {
	// Set default config
	cfg := configDefault(config...)

	if cfg.Storage == nil {
		cfg.Storage = memory.New()
	}

	return &Store{
		cfg,
	}
}

// RegisterType registers a custom type for encoding/decoding into any storage provider.
func (*Store) RegisterType(i interface{}) {
	gob.Register(i)
}

// Get retrieves or creates a session for the given context.
func (s *Store) Get(c *fiber.Ctx) (*Session, error) {
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
	sess.ctx = c
	sess.config = s
	sess.id = id
	sess.fresh = fresh

	// Decode session data if found
	if rawData != nil {
		if err := sess.decodeSessionData(rawData); err != nil {
			return nil, fmt.Errorf("failed to decode session data: %w", err)
		}
	}

	return sess, nil
}

// getSessionID returns the session ID from cookies, headers, or query string.
func (s *Store) getSessionID(c *fiber.Ctx) string {
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
		id = c.Query(s.sessionName)
		if len(id) > 0 {
			return utils.CopyString(id)
		}
	}

	return ""
}

// Reset deletes all sessions from the storage.
func (s *Store) Reset() error {
	return s.Storage.Reset()
}

// Delete deletes a session by its ID.
func (s *Store) Delete(id string) error {
	if id == "" {
		return ErrEmptySessionID
	}
	return s.Storage.Delete(id)
}
