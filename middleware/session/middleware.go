// Package session provides session management middleware for Fiber.
// This middleware handles user sessions, including storing session data in the store.
package session

import (
	"errors"
	"sync"

	"github.com/gofiber/fiber/v3"
)

// Middleware holds session data and configuration.
type Middleware struct {
	Session   *Session
	ctx       fiber.Ctx
	config    Config
	mu        sync.RWMutex
	destroyed bool
}

// Context key for session middleware lookup.
type middlewareKey int

const (
	// middlewareContextKey is the key used to store the *Middleware in the context locals.
	middlewareContextKey middlewareKey = iota
)

var (
	// ErrTypeAssertionFailed occurs when a type assertion fails.
	ErrTypeAssertionFailed = errors.New("failed to type-assert to *Middleware")

	// Pool for reusing middleware instances.
	middlewarePool = &sync.Pool{
		New: func() any {
			return &Middleware{}
		},
	}
)

// New initializes session middleware with optional configuration.
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
//
// Usage:
//
//	app.Use(session.New())
func New(config ...Config) fiber.Handler {
	if len(config) > 0 {
		handler, _ := NewWithStore(config[0])
		return handler
	}
	handler, _ := NewWithStore()
	return handler
}

// NewWithStore creates session middleware with an optional custom store.
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
		cfg.Store = NewStore(cfg)
	}

	handler := func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Acquire session middleware
		m := acquireMiddleware()
		m.initialize(c, cfg)

		stackErr := c.Next()

		m.mu.RLock()
		destroyed := m.destroyed
		m.mu.RUnlock()

		if !destroyed {
			m.saveSession()
		}

		releaseMiddleware(m)
		return stackErr
	}

	return handler, cfg.Store
}

// initialize sets up middleware for the request.
func (m *Middleware) initialize(c fiber.Ctx, cfg Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, err := cfg.Store.getSession(c)
	if err != nil {
		panic(err) // handle or log this error appropriately in production
	}

	m.config = cfg
	m.Session = session
	m.ctx = c

	c.Locals(middlewareContextKey, m)
}

// saveSession handles session saving and error management after the response.
func (m *Middleware) saveSession() {
	if err := m.Session.saveSession(); err != nil {
		if m.config.ErrorHandler != nil {
			m.config.ErrorHandler(m.ctx, err)
		} else {
			DefaultErrorHandler(m.ctx, err)
		}
	}

	releaseSession(m.Session)
}

// acquireMiddleware retrieves a middleware instance from the pool.
func acquireMiddleware() *Middleware {
	m, ok := middlewarePool.Get().(*Middleware)
	if !ok {
		panic(ErrTypeAssertionFailed.Error())
	}
	return m
}

// releaseMiddleware resets and returns middleware to the pool.
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
	m, ok := c.Locals(middlewareContextKey).(*Middleware)
	if !ok {
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
func (m *Middleware) Set(key, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Session.Set(key, value)
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
func (m *Middleware) Get(key any) any {
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
func (m *Middleware) Delete(key any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Session.Delete(key)
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

	return m.Session.Reset()
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
