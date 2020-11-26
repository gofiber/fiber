package session

import (
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/gotiny"
	"github.com/gofiber/fiber/v2/internal/storage/memory"
)

type Store struct {
	Config
}

var mux sync.Mutex

// Storage ErrNotExist
var errNotExist = "key does not exist"

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

func (s *Store) Get(c *fiber.Ctx) (*Session, error) {
	var fresh bool

	// Get key from cookie
	id := c.Cookies(s.CookieName)

	// If no key exist, create new one
	if len(id) == 0 {
		id = s.KeyGenerator()
		fresh = true
	}

	// Create session object
	sess := acquireSession()
	sess.ctx = c
	sess.config = s
	sess.id = id
	sess.fresh = fresh

	// Fetch existing data
	if !fresh {
		raw, err := s.Storage.Get(id)
		// Unmashal if we found data
		if err == nil {
			sess.Lock()
			gotiny.Unmarshal(raw, &sess.data)
			sess.Unlock()
			sess.fresh = false
		} else if raw != nil && err.Error() != "key does not exist" {
			return nil, err
		} else {
			sess.fresh = true
		}
	}

	return sess, nil
}

// Reset will delete all session from the storage
func (s *Store) Reset() error {
	return s.Storage.Reset()
}
