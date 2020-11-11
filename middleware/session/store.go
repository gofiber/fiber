package session

import (
	"github.com/gofiber/fiber/v2"
)

type Store struct {
	Config
}

// Storage ErrNotExist
var errNotExist = "key does not exist"

func New(config ...Config) *Store {
	// Set default config
	cfg := configDefault(config...)

	return &Store{
		cfg,
	}
}

func (s *Store) Get(c *fiber.Ctx) (*Session, error) {
	var fresh bool

	// Get key from cookie
	id := c.Cookies(s.Cookie.Name)

	// If no key exist, create new one
	if len(id) == 0 {
		id = s.KeyGenerator()
		fresh = true
	}

	// Create session object
	sess := acquireSession()
	sess.ctx = c
	sess.config = s
	sess.fresh = fresh
	sess.id = id

	// Fetch existing data
	if !fresh {
		raw, err := s.Storage.Get(id)
		// Unmashal if we found data
		if err == nil {
			if _, err = sess.db.UnmarshalMsg(raw); err != nil {
				return nil, err
			}
		} else if err.Error() != errNotExist {
			// Only return error if it's not ErrNotExist
			return nil, err
		}
	}

	return sess, nil
}

func (s *Store) Reset() error {
	return s.Storage.Reset()
}
