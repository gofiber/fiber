package session

import (
	"sync"

	"github.com/gofiber/fiber/v2"
)

type Store struct {
	Config
	mux      *sync.RWMutex
	sessions map[string]*data
}

// Storage ErrNotExist
var errNotExist = "key does not exist"

func New(config ...Config) *Store {
	// Set default config
	cfg := configDefault(config...)

	// Create Store object
	store := &Store{
		Config: cfg,
	}

	// Default store logic (if no Storage is provided)
	if cfg.Storage == nil {
		store.mux = &sync.RWMutex{}
		store.sessions = make(map[string]*data)
	}

	return store
}

func (s *Store) Get(c *fiber.Ctx) (*Session, error) {
	var fresh bool

	// Get session id from cookie
	id := c.Cookies(s.CookieName)

	// Create key if not exist
	if len(id) == 0 {
		id = s.KeyGenerator()
		fresh = true
	}

	// Get session object from pool
	sess := acquireSession()
	sess.id = id
	sess.fresh = fresh
	sess.ctx = c
	sess.config = s

	// Get session data if not fresh
	if !sess.fresh {
		// Use external Storage if exist
		if s.Storage != nil {
			raw, err := s.Storage.Get(id)
			// Unmashal if we found data
			if err == nil {
				sess.data = acquireData()
				if _, err = sess.data.UnmarshalMsg(raw); err != nil {
					return nil, err
				}
			} else if err.Error() != errNotExist {
				// Only return error if it's not ErrNotExist
				return nil, err
			} else {
				// No data was found, this is now a fresh session
				sess.fresh = true
			}
		} else {
			// Find data in local memory map
			s.mux.RLock()
			data, ok := s.sessions[id]
			s.mux.RUnlock()
			if ok && data != nil {
				sess.data = data
			} else {
				// No data was found, this is now a fresh session
				sess.fresh = true
			}
		}
	}

	// Get new kv store if nil
	if sess.data == nil {
		sess.data = acquireData()
	}

	return sess, nil
}

// Reset will delete all session from the storage
func (s *Store) Reset() error {
	if s.Storage != nil {
		return s.Storage.Reset()
	}
	s.mux.Lock()
	s.sessions = make(map[string]*data)
	s.mux.Unlock()
	return nil
}
