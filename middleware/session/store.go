package session

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

type Store struct {
	Config
}

func New(config ...Config) *Store {
	// Set default config
	cfg := configDefault(config...)

	return &Store{
		cfg,
	}
}

func (s *Store) Get(c *fiber.Ctx) *Session {
	var fresh bool

	// Get ID from cookie
	id := c.Cookies(s.Cookie.Name)

	// If no ID exist, create new one
	if len(id) == 0 {
		id = s.KeyGenerator()
		fresh = true
	}

	// Create session object
	sess := &Session{
		ctx:    c,
		config: s,
		fresh:  fresh,
		db:     acquireDB(),
		id:     id,
	}

	// Fetch existing data
	if !fresh {
		raw, err := s.Storage.Get(id)

		// Set data
		if err == nil {
			_, err := sess.db.UnmarshalMsg(raw)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
	// Return session object
	return sess
}

func (s *Store) Reset() error {
	return s.Storage.Reset()
}
