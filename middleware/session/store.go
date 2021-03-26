package session

import (
	"encoding/gob"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/storage/memory"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

type Store struct {
	Config
}

var mux sync.Mutex

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

// RegisterType will allow you to encode/decode custom types
// into any Storage provider
func (s *Store) RegisterType(i interface{}) {
	gob.Register(i)
}

// Get will get/create a session
func (s *Store) Get(c *fiber.Ctx) (*Session, error) {
	var fresh bool
	var loadDada = true

	// Get key from cookie
	id := c.Cookies(s.CookieName)

	if len(id) == 0 {
		fresh = true
		var err error
		if id, err = s.responseCookies(c); err != nil {
			return nil, err
		}
	}

	// If no key exist, create new one
	if len(id) == 0 {
		loadDada = false
		id = s.KeyGenerator()
	}

	// Create session object
	sess := acquireSession()
	sess.ctx = c
	sess.config = s
	sess.id = id
	sess.fresh = fresh

	// Fetch existing data
	if loadDada {
		raw, err := s.Storage.Get(id)
		// Unmashal if we found data
		if raw != nil && err == nil {
			mux.Lock()
			_, _ = sess.byteBuffer.Write(raw)
			encCache := gob.NewDecoder(sess.byteBuffer)
			err := encCache.Decode(&sess.data.Data)
			if err != nil {
				return nil, err
			}
			mux.Unlock()
		} else if err != nil {
			return nil, err
		} else {
			sess.fresh = true
		}
	}

	return sess, nil
}

func (s *Store) responseCookies(c *fiber.Ctx) (string, error) {
	// Get key from response cookie
	cookieValue := c.Response().Header.PeekCookie(s.CookieName)
	if len(cookieValue) == 0 {
		return "", nil
	}

	cookie := fasthttp.AcquireCookie()
	err := cookie.ParseBytes(cookieValue)
	if err != nil {
		return "", err
	}

	value := make([]byte, len(cookie.Value()))
	copy(value, cookie.Value())
	id := utils.UnsafeString(value)
	fasthttp.ReleaseCookie(cookie)
	return id, nil
}

// Reset will delete all session from the storage
func (s *Store) Reset() error {
	return s.Storage.Reset()
}
