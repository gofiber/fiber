package session

import (
	"errors"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// Session defines the session middleware configuration

type Middleware struct {
	Session    *Session
	ctx        *fiber.Ctx
	config     Config
	mu         sync.RWMutex
	hasChanged bool // TODO: use this to optimize interaction with the session store
	destroyed  bool
}

// key for looking up session middleware in request context
const key = 0

var (
	// ErrTypeAssertionFailed is returned when the type assertion failed
	ErrTypeAssertionFailed = errors.New("failed to type-assert to *Middleware")

	middlewarePool = &sync.Pool{
		New: func() any {
			return &Middleware{}
		},
	}
)

// Session is a middleware to manage session state
//
// Session middleware manages common session state between requests.
// This middleware is dependent on the session store, which is responsible for
// storing the session data.
func New(config ...Config) fiber.Handler {
	var handler fiber.Handler
	if len(config) > 0 {
		handler, _ = NewWithStore(config[0])
	} else {
		handler, _ = NewWithStore()
	}

	return handler
}

// NewWithStore returns a new session middleware with the given store
func NewWithStore(config ...Config) (fiber.Handler, *Store) {
	cfg := configDefault(config...)

	if cfg.Store == nil {
		cfg.Store = newStore(cfg)
	}

	handler := func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Get the session
		session, err := cfg.Store.getSession(c)
		if err != nil {
			return err
		}

		// get a middleware from the pool
		m := acquireMiddleware()
		m.mu.Lock()
		m.config = cfg
		m.Session = session
		m.ctx = &c

		// Store the middleware in the context
		c.Locals(key, m)
		m.mu.Unlock()

		// Continue stack
		stackErr := c.Next()

		m.mu.RLock()
		destroyed := m.destroyed
		m.mu.RUnlock()

		if !destroyed {
			// Save the session
			// This is done after the response is sent to the client
			// It allows us to modify the session data during the request
			// without having to worry about calling Save() on the session.
			//
			// It will also extend the session idle timeout automatically.
			if err := session.saveSession(); err != nil {
				if cfg.ErrorHandler != nil {
					cfg.ErrorHandler(&c, err)
				} else {
					DefaultErrorHandler(&c, err)
				}
			}
		}

		// release the middleware back to the pool
		releaseMiddleware(m)

		return stackErr
	}

	return handler, cfg.Store
}

// acquireMiddleware returns a new Middleware from the pool
func acquireMiddleware() *Middleware {
	middleware, ok := middlewarePool.Get().(*Middleware)
	if !ok {
		panic(ErrTypeAssertionFailed.Error())
	}
	return middleware
}

// releaseMiddleware returns a Middleware to the pool
func releaseMiddleware(m *Middleware) {
	m.mu.Lock()
	m.config = Config{}
	m.Session = nil
	m.ctx = nil
	m.destroyed = false
	m.hasChanged = false
	m.mu.Unlock()
	middlewarePool.Put(m)
}

// FromContext returns the Middleware from the fiber context
func FromContext(c fiber.Ctx) *Middleware {
	m, ok := c.Locals(key).(*Middleware)
	if !ok {
		log.Warn("session: Session middleware not registered. See https://docs.gofiber.io/middleware/session")
		return nil
	}
	return m
}

func (m *Middleware) Set(key string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Session.Set(key, value)
	m.hasChanged = true
}

func (m *Middleware) Get(key string) any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.Session.Get(key)
}

func (m *Middleware) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Session.Delete(key)
	m.hasChanged = true
}

func (m *Middleware) Destroy() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := m.Session.Destroy()
	m.destroyed = true
	return err
}

func (m *Middleware) Fresh() bool {
	return m.Session.Fresh()
}

func (m *Middleware) ID() string {
	return m.Session.ID()
}

func (m *Middleware) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := m.Session.Reset()
	m.hasChanged = true
	return err
}

// Store returns the session store
func (m *Middleware) Store() *Store {
	return m.config.Store
}
