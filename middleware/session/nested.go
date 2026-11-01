package session

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
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

// sharedID returns the ID and the provenance of the session of the nearest session
// middleware, m or one it runs inside, whose store shares a source with s. A
// middleware does not cache its ID on the request: it holds the session, and what it
// holds is gone with it, which is why a Store.Get after it reads the request again.
func (m *Middleware) sharedID(s *Store) (string, extractors.Result, bool) {
	var (
		id  string
		res extractors.Result
	)
	found := m.find(func(cur *Middleware) bool {
		if cur.Session == nil || !s.sharesSource(cur.store) {
			return false
		}
		id, res = cur.Session.identity()
		return true
	})
	return id, res, found != nil
}
