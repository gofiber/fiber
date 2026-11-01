package session

import (
	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

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

// adopt records that a middleware is about to run inside m, and reports whether
// it may. It succeeds only while m still serves the request of c: m can have
// finished, and even been handed to another request, while a handler that
// outlives its request goes on running. The middleware that was adopted must call
// unlink when it finishes.
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

// unpin gives back the count this middleware holds on the one it runs inside, once.
// It takes no lock: it runs while a panic unwinds this goroutine, which may hold
// the lock of this middleware.
func (m *Middleware) unpin() {
	if outer := m.pinned; outer != nil {
		m.pinned = nil
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
		return m.store == s
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
