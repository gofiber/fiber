package session

import (
	"errors"
	"fmt"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// key for looking up session middleware in request context
const key = 0

// Session defines the session middleware configuration
type MiddlewareConfig struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// Store defines the session store
	//
	// Required.
	Store *Store

	// ErrorHandler defines a function which is executed for errors
	//
	// Optional. Default: nil
	ErrorHandler func(*fiber.Ctx, error)
}

type Middleware struct {
	config     MiddlewareConfig
	Session    *Session
	ctx        *fiber.Ctx
	hasChanged bool
	mu         sync.RWMutex
}

// Middleware pool
var middlewarePool = &sync.Pool{
	New: func() any {
		return &Middleware{}
	},
}

// Session is a middleware to manage session state
//
// Session middleware manages common session state between requests.
// This middleware is dependent on the session store, which is responsible for
// storing the session data.
func NewMiddleware(config MiddlewareConfig) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if config.Next != nil && config.Next(c) {
			return c.Next()
		}

		// Get the session
		session, err := config.Store.Get(c)
		if err != nil {
			return err
		}

		// get a middleware from the pool
		m := acquireMiddleware()
		m.config = config
		m.Session = session
		m.ctx = &c

		// Store the middleware in the context
		c.Locals(key, m)

		// Continue stack
		stackErr := c.Next()

		// Save the session
		// This is done after the response is sent to the client
		// It allows us to modify the session data during the request
		// Without having to worry about calling Save()
		//
		// It will also extend the session idle timeout automatically.
		if err := session.Save(); err != nil {
			if config.ErrorHandler != nil {
				config.ErrorHandler(&c, err)
			} else {
				log.Errorf("session: %v", err)
			}
		}

		// release the middleware back to the pool
		releaseMiddleware(m)

		return stackErr
	}
}

var ErrTypeAssertionFailed = errors.New("failed to type-assert to *Middleware")

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
	m.config = MiddlewareConfig{}
	m.Session = nil
	m.ctx = nil
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
	m.reaquireSession()
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

func (m *Middleware) reaquireSession() {
	if m.ctx == nil {
		return
	}

	session, err := m.config.Store.Get(*m.ctx)
	if err != nil {
		m.config.ErrorHandler(m.ctx, err)
	}
	m.Session = session
	m.hasChanged = false
}
