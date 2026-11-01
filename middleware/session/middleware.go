// Package session provides session management middleware for Fiber.
// This middleware handles user sessions, including storing session data in the store.
package session

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/ctxlocal"
	"github.com/gofiber/fiber/v3/internal/redact"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

// Middleware holds session data and configuration.
// Middleware serializes access to its internal state with a mutex, but it is
// request-scoped and must not be used after the request lifecycle ends.
type Middleware struct {
	ctx     fiber.Ctx
	Session *Session
	// parent is the session middleware this one runs inside when several are
	// registered, so a store can find its own among them. It is only ever set by
	// adopt, which keeps parent from being recycled while this middleware runs,
	// and releaseMiddleware clears it. A panic that unwinds this middleware leaves
	// it, for whatever recovers the panic. The chain is only walked from the
	// middleware the request currently holds (find), never from one a goroutine
	// kept past its request.
	parent *Middleware
	// pinned is the middleware this one holds a count on, in its children, while it
	// runs inside it. It is cleared when that count is given back, once, by the end
	// of its release or by the unwinding of a panic, whichever comes first. Unlike
	// parent it is read and written by the goroutine that runs the middleware only,
	// without the lock: a panic can unwind it from inside the locked part of the
	// release, and it must still be able to give the count back.
	pinned *Middleware
	// store and errorHandler are the parts of the configuration a running middleware
	// reads. Keeping them, rather than a copy of the whole Config, spares every
	// request a copy and a reset of about 200 bytes, and the request context that
	// holds the middleware that much memory. releaseMiddleware clears both.
	store        *Store
	errorHandler func(fiber.Ctx, error)
	mu           sync.RWMutex
	// life says what has happened to the middleware, see lifecycle. One word, read
	// with and without the lock alike.
	life atomic.Uint32
	// children counts the middleware that run inside this one and still refer to it
	// as their parent. Ordinarily each finishes before this one does. A middleware
	// that abandons the request around the inner one breaks that: this one can
	// finish while the handlers it abandoned go on running.
	children atomic.Int32
}

// lifecycle is what has happened to a Middleware, as flags of one word. A
// middleware goes back to the pool with none of them set (one that has the released
// or the escaped flag does not go back), and the readers that take no lock
// (FromContext, the walk from an inner middleware) see the same value as the ones
// that do.
type lifecycle uint32

const (
	// lifeReleased is set when the request is over and the middleware was not
	// recycled, because the request context holds it or another middleware still
	// refers to it, so FromContext stops returning it.
	lifeReleased lifecycle = 1 << iota
	// lifeUnwound is set when a panic unwound the middleware. It then belongs to its
	// request until the end of it, where nothing of this package runs any more, so
	// FromContext returns it for the request itself, for whatever recovers the
	// panic, and not for a Go context kept past the request.
	lifeUnwound
	// lifeDestroyed is set when the session was destroyed: it is not saved.
	lifeDestroyed
	// lifeEscaped marks a middleware the request context holds, which
	// releaseMiddleware must not recycle.
	lifeEscaped
)

// is reports whether the middleware has every flag of flag.
func (m *Middleware) is(flag lifecycle) bool {
	return lifecycle(m.life.Load())&flag == flag
}

// mark sets the flags of flag.
func (m *Middleware) mark(flag lifecycle) {
	m.life.Or(uint32(flag))
}

// unmark clears the flags of flag.
func (m *Middleware) unmark(flag lifecycle) {
	m.life.And(^uint32(flag))
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

		return m.serve(c, &cfg)
	}

	return handler, cfg.Store
}

// serve runs the rest of the stack, then saves the session and releases the
// middleware. A panic that unwinds it, in the rest of the stack, in the save or in
// the release, skips all of that: the deferred call ends the middleware instead,
// see endUnwound. The panic itself goes on as before. The defer is open-coded, and
// costs the request a few instructions; the closure does not escape.
func (m *Middleware) serve(c fiber.Ctx, cfg *Config) error {
	finished := false
	defer m.endIfUnwound(&finished, c)

	stackErr := c.Next()

	// A handler timeout.New gave up on may still use m: neither save nor pool it or its session,
	// but detach both from the request, whose context is reused once such handlers are done.
	// IsAbandoned goes first: its atomic load orders the read after the timeout middleware's write.
	if c.IsAbandoned() && c.RequestCtx().LastTimeoutErrorResponse() != nil {
		m.detachSoon(nil)
		finished = true
		return stackErr
	}

	m.mu.RLock()
	isDestroyed := m.is(lifeDestroyed)
	m.mu.RUnlock()

	// A destroyed session is not saved, and neither is a new one that holds
	// nothing when SkipEmptySessions is on.
	skipSave := isDestroyed || (cfg.SkipEmptySessions && m.Session.isEmptyFresh())
	if !skipSave {
		m.saveSession()
	}

	releaseMiddleware(m)
	finished = true
	return stackErr
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

	// Without PassLocalsToContext the request context never holds the middleware,
	// and asking it for one would only make Fiber create a context for the request.
	if !m.store.passesLocalsToContext(c) {
		return false
	}
	held, ok := c.Context().Value(middlewareContextKey).(*Middleware)
	return ok && held == m
}

// clearMiddlewareLocals clears the session ID and the middleware from the Fiber
// locals. A Store.Get after the middleware ended finds neither: it looks at the IDs
// that Store.Get cached, and then reads the session ID from the request, as for a
// request the middleware did not serve.
func clearMiddlewareLocals(c fiber.Ctx) {
	ctxlocal.Set(c, sessionIDContextKey, nil)
	ctxlocal.Set(c, middlewareContextKey, nil)
}

// initialize sets up middleware for the request. It returns an error when the
// session cannot be loaded from the store (e.g. the backing storage is down)
// so the request can fail gracefully instead of crashing the server.
func (m *Middleware) initialize(c fiber.Ctx, cfg *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// The middleware still current in this request, if any, is the one this runs
	// inside; storeMiddlewareContext is about to replace it.
	outer := currentMiddleware(c)
	session, _, err := cfg.Store.loadSession(c, outer)
	if err != nil {
		return err
	}

	m.store = cfg.Store
	m.errorHandler = cfg.ErrorHandler
	m.Session = session
	m.ctx = c

	if storeMiddlewareContext(c, session, m) {
		m.mark(lifeEscaped)
	}

	// Last, so that nothing after the count is taken can panic: the call that gives
	// it back is only installed once initialize has returned.
	if outer != nil && outer.adopt(c) {
		m.parent = outer
		m.pinned = outer
	}
	return nil
}

// saveSession handles session saving and error management after the response.
func (m *Middleware) saveSession() {
	if err := m.Session.saveSessionWithContext(m.resolveContext()); err != nil {
		handleSessionError(m.ctx, m.errorHandler, err)
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

// requestContext is resolveContext for the callers that may run while the request
// is ending, as a handler that outlives its request does: release clears the
// context under the lock, and the read must not tear against that.
func (m *Middleware) requestContext() context.Context {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.resolveContext()
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
// middleware that abandons the request, such as sse, lets handlers outlive the
// middleware around them, and such a handler finds a finished middleware instead
// of another request's. The session of an abandoned request is not recycled
// either, for the same reason. A request that the timeout middleware answered
// itself never gets here: serve leaves it alone, see there.
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
	var (
		outer, inner    *Middleware
		c               fiber.Ctx
		session         *Session
		abandoned, drop bool
	)
	// The lock is given back by a deferred call: the Ctx is called under it, and a
	// custom one that panics there would otherwise leave the lock held for good, and
	// whatever recovers the panic and asks the middleware for the session waiting.
	func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		outer, c, session = m.parent, m.ctx, m.Session
		// A request whose context was abandoned, as the sse middleware does, can have
		// handlers still running that hold its middleware and its session: neither goes
		// back to the pool, or such a handler could find the next request's in its place.
		abandoned = c != nil && c.IsAbandoned()
		drop = m.is(lifeEscaped) || abandoned || m.children.Load() > 0
		if drop {
			// First, so a lookup that races the release finds a middleware that is
			// visibly gone instead of one that is half reset.
			m.mark(lifeReleased)
		}
		if c != nil {
			// The current middleware of the request is m, unless a panic unwound one
			// that ran inside it: that one ends with this request, now.
			if current := currentMiddleware(c); current != nil && current != m {
				inner = current
			}
			clearMiddlewareLocals(c)
			if m.is(lifeEscaped) {
				shadowMiddlewareContext(c)
			}
		}
		m.store = nil
		m.errorHandler = nil
		m.Session = nil
		m.ctx = nil
		m.parent = nil
		m.unmark(lifeDestroyed | lifeUnwound)
	}()

	// Outside the lock of m: the walk from an inner middleware takes the locks the
	// other way round.
	if inner != nil {
		inner.detachIfUnwound()
	}

	// After m let go of it: until then a goroutine that holds m could still reach
	// the session through m.Session once it was handed to another request.
	if session != nil {
		if abandoned {
			session.detach(nil)
		} else {
			releaseSession(session)
		}
	}
	if outer != nil {
		if c != nil {
			restoreMiddlewareContext(c, outer)
		}
		// Last: until here the outer middleware must not be recycled, and if anything
		// above panics, the unwinding gives the count back.
		m.unpin()
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
// The Middleware belongs to its request, as the type documents. A handler that the
// timeout middleware gave up on keeps its own: the middleware neither saves the
// session nor recycles it, and the handler goes on using it, never another
// request's; both let go of the request, whose context is reused once the handler
// is done. A request
// abandoned any other way, as the sse middleware does, is saved as usual when its
// handler returns, and FromContext returns nil for it from then on; a Middleware
// that a handler still running got before stays finished: it is never recycled
// for another request, but its session is gone, so the methods that use it panic.
// A middleware that a panic unwound is not saved or recycled either, and stays the
// current one for whatever recovers the panic, with its session and the one it
// runs inside, until the request ends: a Destroy, Reset or Regenerate from there
// changes the storage and sets the cookies as before the panic, and a Save saves
// the session and sets its cookie, which it does not do while the middleware is
// in charge of the session. It has let go of the fiber.Ctx, and lets go of the
// request when fasthttp resets it, so a
// goroutine of the handler that still holds the session reaches no other request;
// a Go context kept past the request does not find the middleware. A goroutine of
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
	if !ok || m == nil || m.is(lifeReleased) {
		return nil
	}
	if m.is(lifeUnwound) && !isRequest(ctx) {
		// A panic unwound it: it belongs to its request until the end of it, and a
		// Go context kept past the request must not find it.
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

// Destroy destroys the session.
//
// Returns:
//   - error: An error if the destruction fails.
//
// Usage:
//
//	err := m.Destroy()
func (m *Middleware) Destroy() error {
	return m.DestroyWithContext(m.requestContext())
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
	m.mark(lifeDestroyed)
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
	return m.ResetWithContext(m.requestContext())
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
	return m.RegenerateWithContext(m.requestContext())
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
	return m.store
}

// shadowedContext is a request context in which the session ID and the middleware
// that a middleware stored in it are hidden behind empty values, as newer values
// would hide them: a context cannot lose a value. It takes one allocation where
// hiding the two values behind context.WithValue takes two.
type shadowedContext struct {
	context.Context //nolint:containedctx // it wraps a context to answer for two keys, as context.WithValue does
}

// Value answers for the two keys of the session middleware, and asks the context
// it wraps for any other.
func (s shadowedContext) Value(key any) any {
	switch key {
	case sessionIDContextKey:
		return ""
	case middlewareContextKey:
		return (*Middleware)(nil)
	default:
		return s.Context.Value(key)
	}
}

// shadowMiddlewareContext hides the session ID and the middleware that the request
// context holds, which only a middleware that stored them there needs. Without
// PassLocalsToContext nothing was stored there, and wrapping the context would
// only allocate.
func shadowMiddlewareContext(c fiber.Ctx) {
	c.SetContext(shadowedContext{Context: c.Context()})
}
