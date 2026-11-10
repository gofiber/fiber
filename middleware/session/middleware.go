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
	"github.com/valyala/fasthttp"
)

// Middleware holds session data and configuration.
// Middleware serializes access to its internal state with a mutex, but it is
// request-scoped and must not be used after the request lifecycle ends.
type Middleware struct {
	ctx     fiber.Ctx
	Session *Session
	// outer is the session middleware this one runs inside when several are
	// registered, so a store can find its own among them. It is only ever set by
	// link, which keeps outer from being recycled while this middleware runs,
	// and releaseMiddleware clears it. A panic that unwinds this middleware leaves
	// it, for whatever recovers the panic. The chain is only walked from the
	// middleware the request currently holds (find), never from one a goroutine
	// kept past its request.
	outer *Middleware
	// linked is the middleware this one holds a link on, counted in its links, while
	// it runs inside it. It is cleared when that link is given back, once, by the end
	// of its release or by the unwinding of a panic, whichever comes first. Unlike
	// outer it is read and written by the goroutine that runs the middleware only,
	// without the lock: a panic can unwind it from inside the locked part of the
	// release, and it must still be able to give the link back.
	linked *Middleware
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
	// links counts the middleware that run inside this one and still refer to it as
	// their outer. Ordinarily each finishes before this one does. A middleware that
	// abandons the request around the inner one breaks that: this one can finish
	// while the handlers it abandoned go on running.
	links atomic.Int32
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
	} else if len(config) > 0 {
		// A Store comes with a Config of the application: the defaults are not options, so
		// without a Config there is nothing the application set to warn about.
		warnStoreOptions(&config[0])
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
// costs the request a few instructions.
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
	session, err := cfg.Store.loadSession(c, outer)
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

	// Last, so that nothing after the link is made can panic: the call that gives it
	// back is only installed once initialize has returned.
	if outer != nil && outer.link(c) {
		m.outer = outer
		m.linked = outer
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
// outer, and one whose request context was abandoned: both happen when a
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
		outer, c, session = m.outer, m.ctx, m.Session
		// A request whose context was abandoned, as the sse middleware does, can have
		// handlers still running that hold its middleware and its session: neither goes
		// back to the pool, or such a handler could find the next request's in its place.
		abandoned = c != nil && c.IsAbandoned()
		drop = m.is(lifeEscaped) || abandoned || m.links.Load() > 0
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
		m.outer = nil
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
			session.leaveCtx(nil)
		} else {
			releaseSession(session)
		}
	}
	if outer != nil {
		if c != nil {
			restoreMiddlewareContext(c, outer)
		}
		// Last: until here the outer middleware must not be recycled, and if anything
		// above panics, the unwinding gives the link back.
		m.unlinkFromOuter()
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
// in charge of the session. From the unwinding on its session writes to its own
// request only, while the handlers of that request still run, and to nothing once
// the request has ended or was abandoned; it learns of the unwinding and of the
// end without waiting for a call that holds the session across the storage, so a
// goroutine of the handler that still holds the session writes to no other request;
// a Go context kept past the request does not find the middleware. A goroutine of
// your own that outlives a handler which returned has no such guarantee: give it
// the session ID, not the Middleware.
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
func (m *Middleware) Get(key any) (value any) { //nolint:nonamedreturns // the deferred call decodes the value once the lock is let go
	m.mu.RLock()
	defer func() {
		m.mu.RUnlock()
		// The lock is held for the lookup only: a codec decodes without it.
		value = readValue(value)
	}()

	value, _ = m.Session.fetch(key)
	return value
}

// fetch is Session.fetch under the read lock of the middleware, as Get takes it.
func (m *Middleware) fetch(key any) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.Session.fetch(key)
}

// GetInto stores the value under key into target, a non-nil pointer. See
// Session.GetInto.
//
// Parameters:
//   - key: The key to retrieve.
//   - target: A pointer to the variable to fill.
//
// Returns:
//   - error: ErrKeyNotFound, or why the value could not be decoded or assigned.
//
// Usage:
//
//	var user User
//	err := m.GetInto("user", &user)
func (m *Middleware) GetInto(key, target any) error {
	return getInto(m.fetch, key, target)
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

// restoreMiddlewareContext makes outer the current middleware of the request
// again, once the middleware that ran inside it has finished and cleared its
// own. Code that runs between the two after c.Next() returns, and an explicit
// Save of the outer session, then still find the middleware that manages it.
//
// outer may have finished in the meantime, as it does when a middleware that
// abandons the request, such as sse, lets the handlers inside it outlive it. It is
// then left out of the request, whose locals were cleared when it finished. The
// check and the publication happen under outer's lock, which its own release takes
// before it clears them, so one cannot overtake the other.
func restoreMiddlewareContext(c fiber.Ctx, outer *Middleware) {
	outer.mu.RLock()
	defer outer.mu.RUnlock()

	if outer.Session == nil {
		return
	}
	fiber.StoreInContext(c, sessionIDContextKey, outer.Session.ID())
	fiber.StoreInContext(c, middlewareContextKey, outer)
}

// link records that a middleware is about to run inside m, and reports whether it
// may. It succeeds only while m still serves the request of c: m can have
// finished, and even been handed to another request, while a handler that
// outlives its request goes on running. The middleware that was linked must end
// the link when it finishes, with unlinkFromOuter.
func (m *Middleware) link(c fiber.Ctx) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// The request is identified by its fasthttp context, which the locals live on
	// and which, unlike the fiber.Ctx, is comparable whatever Ctx the app uses.
	if m.Session == nil || m.ctx == nil || m.ctx.RequestCtx() != c.RequestCtx() {
		return false
	}
	m.links.Add(1)
	return true
}

// unlink ends a link that link made on m, which the caller must not use afterwards:
// with its last link gone, m can be recycled.
func (m *Middleware) unlink() {
	m.links.Add(-1)
}

// unlinkFromOuter gives back the link this middleware holds on the one it runs
// inside, once. It takes no lock: it runs while a panic unwinds this goroutine,
// which may hold the lock of this middleware.
func (m *Middleware) unlinkFromOuter() {
	if outer := m.linked; outer != nil {
		m.linked = nil
		outer.unlink()
	}
}

// currentMiddleware returns the session middleware the request currently holds, or
// nil if it holds none.
func currentMiddleware(c fiber.Ctx) *Middleware {
	m, ok := c.Locals(middlewareContextKey).(*Middleware)
	if !ok {
		return nil
	}
	return m
}

// isRequest reports whether ctx is a request itself, a fiber.Ctx or the
// *fasthttp.RequestCtx under it, as opposed to a Go context taken from one.
func isRequest(ctx any) bool {
	switch ctx.(type) {
	case fiber.Ctx, *fasthttp.RequestCtx:
		return true
	default:
		return false
	}
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
		if m.is(lifeReleased) {
			m.mu.RUnlock()
			return nil
		}
		if match(m) {
			m.mu.RUnlock()
			return m
		}
		outer := m.outer
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

// sharedID returns the identity of the session of the nearest session middleware, m or
// one it runs inside, whose store shares a source with s. A middleware does not cache
// its ID on the request: it holds the session, and what it holds is gone with it, which
// is why a Store.Get after it reads the request again.
func (m *Middleware) sharedID(s *Store) (sessionIdentity, bool) {
	var ident sessionIdentity
	found := m.find(func(cur *Middleware) bool {
		if cur.Session == nil || !s.sharesSource(cur.store) {
			return false
		}
		ident = cur.Session.identity()
		return true
	})
	return ident, found != nil
}

// endIfUnwound is the deferred call of serve: it ends the middleware when
// a panic unwinds it. Once serve finished, the middleware was released or detached,
// and may already belong to another request, so nothing is done.
func (m *Middleware) endIfUnwound(finished *bool, c fiber.Ctx) {
	if !*finished {
		m.endUnwound(c)
	}
}

// endUnwound ends a middleware that a panic unwound, whose release never runs. The
// outer middleware gets its link back, so that it is recycled when it finishes
// instead of counting this one as running inside for good. The middleware and its
// session let go of the fiber.Ctx, which goes back to the pool as soon as the
// handlers return, where the next request takes it; a goroutine of the handler
// that still held the session would otherwise write its cookie into that request,
// or run its storage calls under it. The session keeps the request itself, which
// lives on until fasthttp resets it: whatever recovers the panic can still reach
// the middleware, its session and the one it runs inside, change the storage
// through them and set the cookies (a Destroy, Reset or Regenerate as before the
// panic, a Save now that the middleware is not going to save); the middleware is kept in
// the request's values, so that fasthttp closes it when it resets the request, and
// the session lets go of the request then, before fasthttp reuses it (Close). An
// outer middleware that finishes first closes it itself. Neither is saved or
// recycled, and the middleware stays the current one of the request, as one
// without an outer middleware does; it is marked unwound, so that a Go context
// kept past the request does not find it. Nothing here waits for the lock of the
// middleware or of the session, which a call of another goroutine can hold across the
// storage, and the Ctx is asked only for the request, since it may be what panicked.
// The session is told all the same, at once (see detachSoon), and before the
// middleware is added to the values of the request: a Regenerate or a Reset that
// another goroutine has under way writes to those values too, and is waited for.
// A write that is not the session's own is not waited for, of another session of the
// request or of the application, and of two writes to those values at once one can
// be lost. A session that loses this one is never told that its request ended; it
// writes to no later request all the same, since it tells its request from the ones
// that follow on the same context (Session.handlersRun).
func (m *Middleware) endUnwound(c fiber.Ctx) {
	m.unlinkFromOuter()
	m.mark(lifeUnwound)
	rctx := c.RequestCtx()
	// The session keeps the request while its handlers run; a Ctx that has none lets go of it too.
	m.detachSoon(rctx)
	if rctx != nil {
		closer := unwoundCloser{m}
		rctx.SetUserValue(closer, closer)
	}
}

// unwoundCloser is the request value that makes fasthttp end the request of a
// middleware a panic unwound, and its own key: the Fiber locals of a custom Ctx need
// not be the request's values, and one panic can unwind several middleware that are
// nested in the request, each of which has to be closed. A type of its own, so that
// the middleware itself does not have to be an io.Closer, which would put a method
// that is not for applications into the exported API.
type unwoundCloser struct{ m *Middleware }

// Close ends the request of the middleware: fasthttp calls it when it resets the
// request, before the request is reused, and the session lets go of it then. It is
// harmless for a middleware that already let go.
func (u unwoundCloser) Close() error {
	u.m.detachIfUnwound()
	return nil
}

// detach makes m forget the fiber.Ctx of the request, under which its storage calls
// would otherwise still run. It waits for the lock.
func (m *Middleware) detach() {
	m.mu.Lock()
	m.ctx = nil
	m.mu.Unlock()
}

// detachSoon cuts m and its session off from the fiber.Ctx of the request, and has the
// session write to rctx, the request itself, instead, or to nothing when rctx is nil (see
// Session.leaveCtx). It is for the request that the timeout middleware answered, or one
// that a panic unwound: m and its session stay as they are for the handlers that outlive
// the request, and the response is fixed, so nothing those handlers write to it counts.
// What they must not do is hold the session past their end and write into whichever
// request gets the context next, or run storage calls under it.
//
// The session is told first, and without the lock of m or its own: a handler can be
// inside a call that holds either across the storage, such as Destroy or Save, and what
// that call still writes must not go to a request that has moved on by the time the
// storage answers. m.Session is read without the lock for that, as serve reads it: only
// the goroutine that runs the middleware sets it, and never again once that goroutine
// was unwound or found the request timed out, which is when this is called.
//
// Forgetting the fiber.Ctx does not hold up the answer either, which is what the timeout
// middleware is there to send on time: when the lock is held, it follows from a goroutine
// of its own as soon as the call is done.
func (m *Middleware) detachSoon(rctx *fasthttp.RequestCtx) {
	if session := m.Session; session != nil {
		session.leaveCtx(rctx)
	}
	if !m.mu.TryLock() {
		go m.detach()
		return
	}
	m.ctx = nil
	m.mu.Unlock()
}

// detachIfUnwound ends the request of a middleware a panic unwound, and is also what the
// release of an outer middleware that finishes first does to the inner one.
func (m *Middleware) detachIfUnwound() {
	if m.is(lifeUnwound) {
		m.detachSoon(nil)
	}
}
