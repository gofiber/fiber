package session

import (
	"errors"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// Middleware defines the session middleware configuration
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

// New creates a new session middleware with the given configuration.
//
// Parameters:
//   - config: Variadic parameter to override default config.
//
// Returns:
//   - fiber.Handler: The Fiber handler for the session middleware.
//
// Usage:
//
//	app.Use(session.New())
func New(config ...Config) fiber.Handler {
	var handler fiber.Handler
	if len(config) > 0 {
		handler, _ = NewWithStore(config[0])
	} else {
		handler, _ = NewWithStore()
	}

	return handler
}

// NewWithStore returns a new session middleware with the given store.
//
// Parameters:
//   - config: Variadic parameter to override default config.
//
// Returns:
//   - fiber.Handler: The Fiber handler for the session middleware.
//   - *Store: The session store.
//
// Usage:
//
//	handler, store := session.NewWithStore()
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

			// Release the session back to the pool
			releaseSession(session)
		}

		// release the middleware back to the pool
		releaseMiddleware(m)

		return stackErr
	}

	return handler, cfg.Store
}

// acquireMiddleware returns a new Middleware from the pool.
//
// Returns:
//   - *Middleware: The middleware object.
//
// Usage:
//
//	m := acquireMiddleware()
func acquireMiddleware() *Middleware {
	middleware, ok := middlewarePool.Get().(*Middleware)
	if !ok {
		panic(ErrTypeAssertionFailed.Error())
	}
	return middleware
}

// releaseMiddleware returns a Middleware to the pool.
//
// Parameters:
//   - m: The middleware object to release.
//
// Usage:
//
//	releaseMiddleware(m)
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

// FromContext returns the Middleware from the Fiber context.
//
// Parameters:
//   - c: The Fiber context.
//
// Returns:
//   - *Middleware: The middleware object if found, otherwise nil.
//
// Usage:
//
//	m := session.FromContext(c)
func FromContext(c fiber.Ctx) *Middleware {
	m, ok := c.Locals(key).(*Middleware)
	if !ok {
		// TODO: since this may be called we may not want to log this except in debug mode?
		log.Warn("session: Session middleware not registered. See https://docs.gofiber.io/middleware/session")
		return nil
	}
	return m
}

// Set sets a key-value pair in the session.
//
// Parameters:
//   - key: The key to set.
//   - value: The value to set.
//
// Usage:
//
//	m.Set("key", "value")
func (m *Middleware) Set(key string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Session.Set(key, value)
	m.hasChanged = true
}

// Get retrieves a value from the session by key.
//
// Parameters:
//   - key: The key to retrieve.
//
// Returns:
//   - any: The value associated with the key.
//
// Usage:
//
//	value := m.Get("key")
func (m *Middleware) Get(key string) any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.Session.Get(key)
}

// Delete removes a key-value pair from the session.
//
// Parameters:
//   - key: The key to delete.
//
// Usage:
//
//	m.Delete("key")
func (m *Middleware) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Session.Delete(key)
	m.hasChanged = true
}

// Destroy destroys the session.
//
// Returns:
//   - error: An error if the destruction fails.
//
// Usage:
//
//	err := m.Destroy()
func (m *Middleware) Destroy() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := m.Session.Destroy()
	m.destroyed = true
	return err
}

// Fresh checks if the session is fresh.
//
// Returns:
//   - bool: True if the session is fresh, otherwise false.
//
// Usage:
//
//	isFresh := m.Fresh()
func (m *Middleware) Fresh() bool {
	return m.Session.Fresh()
}

// ID returns the session ID.
//
// Returns:
//   - string: The session ID.
//
// Usage:
//
//	id := m.ID()
func (m *Middleware) ID() string {
	return m.Session.ID()
}

// Reset resets the session.
//
// Returns:
//   - error: An error if the reset fails.
//
// Usage:
//
//	err := m.Reset()
func (m *Middleware) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := m.Session.Reset()
	m.hasChanged = true
	return err
}

// Store returns the session store.
//
// Returns:
//   - *Store: The session store.
//
// Usage:
//
//	store := m.Store()
func (m *Middleware) Store() *Store {
	return m.config.Store
}
