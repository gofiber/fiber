// Package session provides session management middleware for Fiber.
// This middleware handles user sessions, including storing session data in the store.
package session

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/ctxlocal"
	"github.com/gofiber/fiber/v3/internal/redact"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

// Middleware holds session data and configuration.
// Middleware serializes access to its internal state with a mutex, but it is
// request-scoped and must not be used after the request lifecycle ends.
type Middleware struct {
	Session *Session
	ctx     fiber.Ctx
	// parent is the session middleware this one runs inside when several are
	// registered, so a store can find its own among them. It is only ever set by
	// adopt, which keeps parent from being recycled while this middleware runs,
	// and releaseMiddleware clears it.
	parent *Middleware
	config Config
	mu     sync.RWMutex
	// released is set when the request is over and the middleware was not recycled,
	// because the request context holds it or another middleware still refers to it,
	// so FromContext stops returning it.
	released atomic.Bool
	// children counts the middleware that run inside this one and still refer to it
	// as their parent. Ordinarily each finishes before this one does. A timeout
	// middleware around the inner one breaks that: this one can finish while the
	// handlers it abandoned go on running.
	children    atomic.Int32
	isDestroyed bool
	// escaped marks a middleware the request context holds, which releaseMiddleware
	// must not recycle.
	escaped bool
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
	registerLogContextTagsOnce.Do(registerLogContextTags)

	cfg := configDefault(config...)

	if cfg.Store == nil {
		cfg.Store = NewStore(cfg)
	} else {
		warnStoreOptions(&cfg)
	}

	handler := func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Acquire session middleware
		m := acquireMiddleware()
		if err := m.initialize(c, &cfg); err != nil {
			releaseMiddleware(m)
			handleSessionError(c, cfg.ErrorHandler, err)
			return nil
		}

		stackErr := c.Next()

		// A handler timeout.New gave up on may still use m: neither save nor pool it or its session.
		// IsAbandoned goes first: its atomic load orders the read after the timeout middleware's write.
		if c.IsAbandoned() && c.RequestCtx().LastTimeoutErrorResponse() != nil {
			return stackErr
		}

		m.mu.RLock()
		isDestroyed := m.isDestroyed
		m.mu.RUnlock()

		// A destroyed session is not saved, and neither is a new one that holds
		// nothing when SkipEmptySessions is on.
		skipSave := isDestroyed || (cfg.SkipEmptySessions && m.Session.isEmptyFresh())
		if !skipSave {
			m.saveSession()
		}

		releaseMiddleware(m)
		return stackErr
	}

	return handler, cfg.Store
}

var registerLogContextTagsOnce sync.Once

func handleSessionError(c fiber.Ctx, handler func(fiber.Ctx, error), err error) {
	if handler != nil {
		handler(c, err)
		return
	}
	DefaultErrorHandler(c, err)
}

func registerLogContextTags() {
	logger.RegisterContextTag("session-id", func(ctx any) string {
		id, ok := fiber.ValueFromContext[string](ctx, sessionIDContextKey)
		if !ok || id == "" {
			return ""
		}

		return redact.Prefix(id)
	})
}

// storeMiddlewareContext publishes the session ID and the middleware to the
// Fiber locals and, with PassLocalsToContext, to the request context. It
// reports whether the request context now holds m. Unlike the locals, which are
// cleared when the request ends, that context can be kept by a goroutine, so a
// middleware it holds must never go back to the pool.
func storeMiddlewareContext(c fiber.Ctx, session *Session, m *Middleware) bool {
	fiber.StoreInContext(c, sessionIDContextKey, session.ID())
	fiber.StoreInContext(c, middlewareContextKey, m)

	held, ok := c.Context().Value(middlewareContextKey).(*Middleware)
	return ok && held == m
}

// clearMiddlewareContext clears both Fiber locals and the request context
// because session middleware stores these values in both layers.
func clearMiddlewareContext(c fiber.Ctx) {
	ctxlocal.Set(c, sessionIDContextKey, "")
	ctxlocal.Set(c, middlewareContextKey, nil)
	ctx := context.WithValue(c.Context(), sessionIDContextKey, "")
	c.SetContext(context.WithValue(ctx, middlewareContextKey, (*Middleware)(nil)))
}

// restoreMiddlewareContext makes outer the current middleware of the request
// again, once the middleware that ran inside it has finished and cleared its
// own. Code that runs between the two after c.Next() returns, and an explicit
// Save of the outer session, then still find the middleware that manages it.
//
// outer may have finished in the meantime, as it does when a timeout middleware
// abandons the handlers inside it. It is then left out of the request, whose
// locals were cleared when it finished. The check and the publication happen
// under outer's lock, which its own release takes before it clears them, so one
// cannot overtake the other.
func restoreMiddlewareContext(c fiber.Ctx, outer *Middleware) {
	outer.mu.RLock()
	defer outer.mu.RUnlock()

	if outer.Session == nil {
		return
	}
	fiber.StoreInContext(c, sessionIDContextKey, outer.Session.ID())
	fiber.StoreInContext(c, middlewareContextKey, outer)
}

// adopt records that a middleware is about to run inside m, and reports whether
// it may. It succeeds only while m still serves the request of c: m can have
// finished, and even been handed to another request, while a handler that a
// timeout middleware abandoned goes on running. The middleware that was adopted
// must call unlink when it finishes.
func (m *Middleware) adopt(c fiber.Ctx) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// The request is identified by its fasthttp context, which the locals live on
	// and which, unlike the fiber.Ctx, is comparable whatever Ctx the app uses.
	if m.Session == nil || m.ctx == nil || m.ctx.RequestCtx() != c.RequestCtx() {
		return false
	}
	m.children.Add(1)
	return true
}

// unlink ends the link adopt made; m must not be used afterwards.
func (m *Middleware) unlink() {
	m.children.Add(-1)
}

// initialize sets up middleware for the request. It returns an error when the
// session cannot be loaded from the store (e.g. the backing storage is down)
// so the request can fail gracefully instead of crashing the server.
func (m *Middleware) initialize(c fiber.Ctx, cfg *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, err := cfg.Store.getSession(c)
	if err != nil {
		return err
	}

	m.config = *cfg
	m.Session = session
	m.ctx = c

	// The middleware still current in this request, if any, is the one this runs
	// inside; storeMiddlewareContext is about to replace it.
	if outer, ok := c.Locals(middlewareContextKey).(*Middleware); ok && outer != nil && outer.adopt(c) {
		m.parent = outer
	}
	m.escaped = storeMiddlewareContext(c, session, m)
	return nil
}

// saveSession handles session saving and error management after the response.
func (m *Middleware) saveSession() {
	if err := m.Session.saveSessionWithContext(m.resolveContext()); err != nil {
		if m.config.ErrorHandler != nil {
			m.config.ErrorHandler(m.ctx, err)
		} else {
			DefaultErrorHandler(m.ctx, err)
		}
	}
}

// resolveContext returns the middleware's stored fiber context if available,
// otherwise returns context.Background().
// fiber.Ctx implements context.Context directly, so no allocation is needed.
func (m *Middleware) resolveContext() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}

// acquireMiddleware retrieves a middleware instance from the pool.
func acquireMiddleware() *Middleware {
	m, ok := middlewarePool.Get().(*Middleware)
	if !ok {
		panic(ErrTypeAssertionFailed.Error())
	}
	return m
}

// releaseMiddleware resets the middleware, returns its session to the pool and
// returns the middleware to the pool too, unless something still refers to it.
//
// A middleware the request context holds is reset and marked released but not
// pooled. A goroutine can keep that context, and with it the pointer, past the
// end of the request; if the object were reused, the next request's session
// would be reachable through the old context. Dropping it leaves the old
// pointer pointing at a finished, empty middleware that nothing else uses.
//
// So is one that other middleware still run inside, since they hold it as their
// parent, and one whose request context was abandoned: both happen when a
// timeout middleware lets handlers outlive the middleware around them, and such a
// handler finds a finished middleware instead of another request's. The session
// of an abandoned request is not recycled either, for the same reason.
//
// The middleware this one ran inside becomes the current one of the request
// again, and learns that one fewer runs inside it.
//
// Parameters:
//   - m: The middleware object to release.
//
// Usage:
//
//	releaseMiddleware(m)
func releaseMiddleware(m *Middleware) {
	m.mu.Lock()
	outer, c, session := m.parent, m.ctx, m.Session
	// A request whose context was abandoned, as the timeout middleware does, can have
	// handlers still running that hold its middleware and its session: neither goes
	// back to the pool, or such a handler could find the next request's in its place.
	abandoned := c != nil && c.IsAbandoned()
	drop := m.escaped || abandoned || m.children.Load() > 0
	if drop {
		// First, so a lookup that races the release finds a middleware that is
		// visibly gone instead of one that is half reset.
		m.released.Store(true)
	}
	if c != nil {
		clearMiddlewareContext(c)
	}
	m.config = Config{}
	m.Session = nil
	m.ctx = nil
	m.parent = nil
	m.isDestroyed = false
	m.mu.Unlock()

	// After m let go of it: until then a goroutine that holds m could still reach
	// the session through m.Session once it was handed to another request.
	if session != nil && !abandoned {
		releaseSession(session)
	}
	if outer != nil {
		if c != nil {
			restoreMiddlewareContext(c, outer)
		}
		outer.unlink()
	}
	if !drop {
		middlewarePool.Put(m)
	}
}

// FromContext returns the Middleware from the Fiber context.
// It accepts fiber.CustomCtx, fiber.Ctx, *fasthttp.RequestCtx, and context.Context.
//
// A context kept from c.Context() (with PassLocalsToContext enabled) stops
// resolving when the request ends: FromContext returns nil for it afterwards,
// never another request's session.
//
// When several session middleware are registered, FromContext returns the
// innermost one, the last to run. Use Store.FromContext to reach the middleware
// of a particular store.
//
// The Middleware belongs to its request, as the type documents. A handler that a
// timeout middleware abandoned gets nil from here once the request ended, and a
// Middleware it got before stays finished: it is never recycled for another
// request, but its session is gone, so calling its methods panics. A goroutine of
// your own that outlives the handler has no such guarantee: give it the session
// ID, not the Middleware.
//
// Parameters:
//   - c: The Fiber context.
//
// Returns:
//   - *Middleware: The middleware object if found and its request is still being
//     handled; otherwise, nil.
//
// Usage:
//
//	m := session.FromContext(c)
func FromContext(ctx any) *Middleware {
	m, ok := fiber.ValueFromContext[*Middleware](ctx, middlewareContextKey)
	if !ok || m == nil || m.released.Load() {
		return nil
	}

	return m
}

// FromContext returns the Middleware that this store's session middleware
// installed for the request, or nil when the store has none there. It accepts
// the same contexts as the package-level FromContext and ends the same way: a
// context kept past the request returns nil.
//
// Register one session middleware per store, each over its own cookie or
// header, and a handler reaches every session of the request:
//
//	loginHandler, loginStore := session.NewWithStore(session.Config{
//	    Extractor: extractors.FromCookie("login_sid"),
//	})
//	appHandler, appStore := session.NewWithStore(session.Config{
//	    Extractor: extractors.FromCookie("app_sid"),
//	})
//	app.Use(loginHandler, appHandler)
//
//	login := loginStore.FromContext(c)
//	appSession := appStore.FromContext(c)
//
// Like the methods of Middleware it takes the read lock of the middleware it
// visits, so call it from handlers, not from a Storage while that middleware is
// destroying, resetting or regenerating its session, which hold the lock.
//
// Parameters:
//   - ctx: The Fiber context, a *fasthttp.RequestCtx, or a context.Context.
//
// Returns:
//   - *Middleware: The store's middleware if it is running for the request;
//     otherwise, nil.
func (s *Store) FromContext(ctx any) *Middleware {
	return FromContext(ctx).find(func(m *Middleware) bool {
		return m.config.Store == s
	})
}

// find returns the first middleware for which match reports true, looking at m
// and then at the ones it runs inside, from the inside out. It returns nil when
// none matches, or when it reaches one that has finished. match runs with the
// read lock of the middleware it is given, so it must not call out.
func (m *Middleware) find(match func(*Middleware) bool) *Middleware {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	for {
		if m.released.Load() {
			m.mu.RUnlock()
			return nil
		}
		if match(m) {
			m.mu.RUnlock()
			return m
		}
		outer := m.parent
		if outer == nil {
			m.mu.RUnlock()
			return nil
		}
		// outer is locked before m is let go. While m is locked it cannot finish,
		// and while it has not finished outer cannot be recycled.
		outer.mu.RLock()
		m.mu.RUnlock()
		m = outer
	}
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

// Keys returns all keys in the current session.
//
// Returns:
//   - []any: A slice of all keys in the session.
//
// Usage:
//
//	keys := m.Keys()
func (m *Middleware) Keys() []any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.Session.Keys()
}

// IdleTimeout returns the idle timeout of the session. See Session.IdleTimeout.
//
// Like the other methods of the middleware it takes a lock of the session, which
// a save holds while it calls the storage: do not call it from a Storage.
//
// Returns:
//   - time.Duration: The idle timeout of the session.
//
// Usage:
//
//	timeout := m.IdleTimeout()
func (m *Middleware) IdleTimeout() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.Session.IdleTimeout()
}

// AbsoluteExpiry returns the time at which the session ends however active it
// is, and whether it has one. See Session.AbsoluteExpiry.
//
// Returns:
//   - time.Time: The absolute expiration time, or the zero time.
//   - bool: True if the session has an absolute expiration time; otherwise, false.
//
// Usage:
//
//	expiry, ok := m.AbsoluteExpiry()
func (m *Middleware) AbsoluteExpiry() (time.Time, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.Session.AbsoluteExpiry()
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
	return m.DestroyWithContext(m.resolveContext())
}

// DestroyWithContext destroys the session using the provided context for cancellation and timeout control.
//
// Parameters:
//   - ctx: The context to use for the storage operation.
//
// Returns:
//   - error: An error if the destruction fails.
//
// Usage:
//
//	err := m.DestroyWithContext(ctx)
func (m *Middleware) DestroyWithContext(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := m.Session.DestroyWithContext(ctx)
	m.isDestroyed = true
	return err
}

// Fresh checks if the session is fresh.
//
// Returns:
//   - bool: True if the session is fresh; otherwise, false.
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
	return m.ResetWithContext(m.resolveContext())
}

// ResetWithContext resets the session using the provided context for cancellation and timeout control.
//
// Parameters:
//   - ctx: The context to use for the storage operation.
//
// Returns:
//   - error: An error if the reset fails.
//
// Usage:
//
//	err := m.ResetWithContext(ctx)
func (m *Middleware) ResetWithContext(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Session.ResetWithContext(ctx)
}

// Regenerate generates a new session ID while preserving session data.
//
// This method is commonly used after authentication to prevent session fixation attacks.
// Unlike Reset(), this method preserves all existing session data.
//
// Returns:
//   - error: An error if the regeneration fails.
//
// Usage:
//
//	err := m.Regenerate()
func (m *Middleware) Regenerate() error {
	return m.RegenerateWithContext(m.resolveContext())
}

// RegenerateWithContext generates a new session ID while preserving session data,
// using the provided context for cancellation and timeout control.
//
// This method is commonly used after authentication to prevent session fixation attacks.
// Unlike ResetWithContext(), this method preserves all existing session data.
//
// Parameters:
//   - ctx: The context to use for the storage operation.
//
// Returns:
//   - error: An error if the regeneration fails.
//
// Usage:
//
//	err := m.RegenerateWithContext(ctx)
func (m *Middleware) RegenerateWithContext(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Session.RegenerateWithContext(ctx)
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
