package session

import (
	"encoding/gob"
	"errors"
	"fmt"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// ErrEmptySessionID is an error that occurs when the session ID is empty.
var (
	ErrEmptySessionID                   = errors.New("session id cannot be empty")
	ErrSessionAlreadyLoadedByMiddleware = errors.New("session already loaded by middleware")
)

type Store struct {
	Config
}

var mux sync.Mutex

func newStore(config ...Config) *Store {
	// Set default config
	cfg := configDefault(config...)

	if cfg.Storage == nil {
		cfg.Storage = memory.New()
	}

	return &Store{
		cfg,
	}
}

// RegisterType will allow you to encode/decode custom types
// into any Storage provider
func (*Store) RegisterType(i any) {
	gob.Register(i)
}

// Get will get/create a session
//
// This function will return an ErrSessionAlreadyLoadedByMiddleware if
// the session is already loaded by the middleware
func (s *Store) Get(c fiber.Ctx) (*Session, error) {
	// If session is already loaded in the context,
	// it should not be loaded again
	_, ok := c.Locals(key).(*Middleware)
	if ok {
		return nil, ErrSessionAlreadyLoadedByMiddleware
	}

	return s.getSession(c)
}

func (s *Store) getSession(c fiber.Ctx) (*Session, error) {
	// Get session based on context
	var fresh bool
	loadData := true

	id := s.getSessionID(c)

	if len(id) == 0 {
		fresh = true
		var err error
		if id, err = s.responseCookies(c); err != nil {
			return nil, err
		}
	}

	// If no key exist, create new one
	if len(id) == 0 {
		loadData = false
		id = s.KeyGenerator()
	}

	// Create session object
	sess := acquireSession()
	sess.ctx = c
	sess.config = s
	sess.id = id
	sess.fresh = fresh

	// Fetch existing data
	if loadData {
		raw, err := s.Storage.Get(id)
		// Unmarshal if we found data
		switch {
		case err != nil:
			return nil, err

		case raw != nil:
			mux.Lock()
			defer mux.Unlock()
			sess.byteBuffer.Write(raw)
			encCache := gob.NewDecoder(sess.byteBuffer)
			err := encCache.Decode(&sess.data.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to decode session data: %w", err)
			}
		default:
			// both raw and err is nil, which means id is not in the storage
			sess.fresh = true
		}
	}

	return sess, nil
}

// getSessionID will return the session id from:
// 1. cookie
// 2. http headers
// 3. query string
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

func (s *Store) responseCookies(c fiber.Ctx) (string, error) {
	// Get key from response cookie
	cookieValue := c.Response().Header.PeekCookie(s.sessionName)
	if len(cookieValue) == 0 {
		return "", nil
	}

	cookie := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(cookie)
	err := cookie.ParseBytes(cookieValue)
	if err != nil {
		return "", err
	}

	value := make([]byte, len(cookie.Value()))
	copy(value, cookie.Value())
	id := string(value)
	return id, nil
}

// Reset will delete all session from the storage
func (s *Store) Reset() error {
	return s.Storage.Reset()
}

// Delete deletes a session by its id.
func (s *Store) Delete(id string) error {
	if id == "" {
		return ErrEmptySessionID
	}
	return s.Storage.Delete(id)
}
