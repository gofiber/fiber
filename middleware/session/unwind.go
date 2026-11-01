package session

import (
	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// unwoundKey is the request value under which a middleware a panic unwound is
// kept, so that fasthttp closes it when it resets the request. A key of its own:
// the Fiber locals of a custom Ctx need not be the request's values. A key for each
// middleware: one panic can unwind several that are nested in the request, and each
// has to be closed.
type unwoundKey struct{ m *Middleware }

// endIfUnwound is the deferred call of serve: it ends the middleware when
// a panic unwinds it. Once serve finished, the middleware was released or detached,
// and may already belong to another request, so nothing is done.
func (m *Middleware) endIfUnwound(finished *bool, c fiber.Ctx) {
	if !*finished {
		m.endUnwound(c)
	}
}

// endUnwound ends a middleware that a panic unwound, whose release never runs. The
// outer middleware gets its count back, so that it is recycled when it finishes
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
// kept past the request does not find it. Nothing here waits for a lock, and the
// Ctx is asked only for the request: a panic in the locked part of the release
// leaves the lock held, and the Ctx may be what panicked.
func (m *Middleware) endUnwound(c fiber.Ctx) {
	m.unpin()
	m.mark(lifeUnwound)
	rctx := c.RequestCtx()
	if rctx != nil {
		rctx.SetUserValue(unwoundKey{m}, unwoundCloser{m})
	}
	// The session keeps the request until it ends; a Ctx that has none lets go of it too.
	m.detachSoon(rctx)
}

// unwoundCloser is the request value that makes fasthttp end the request of a
// middleware a panic unwound. A type of its own, so that the middleware itself does
// not have to be an io.Closer, which would put a method that is not for
// applications into the exported API.
type unwoundCloser struct{ m *Middleware }

// Close ends the request of the middleware: fasthttp calls it when it resets the
// request, before the request is reused, and the session lets go of it then. It is
// harmless for a middleware that already let go.
func (u unwoundCloser) Close() error {
	u.m.detachIfUnwound()
	return nil
}

// detach cuts m and its session off from the fiber.Ctx of the request, and has the
// session keep rctx, the request itself, instead, when it is not nil (see
// Session.detach). It waits for the locks.
func (m *Middleware) detach(rctx *fasthttp.RequestCtx) {
	m.mu.Lock()
	session := m.Session
	m.ctx = nil
	m.mu.Unlock()
	if session != nil {
		session.detach(rctx)
	}
}

// detachSoon is detach for the request that the timeout middleware answered, or one that a
// panic unwound: m and its session stay as they are for the handlers that outlive
// the request, and the response is fixed, so nothing those handlers write to it
// counts. What they must not do is hold the session past their end and write into
// whichever request gets the context next, or run storage calls under it.
//
// The answer does not wait for the detaching. A handler the timeout middleware gave up on
// can be inside a call that holds the lock of m or of the session across the
// storage, such as Destroy or Save, and the answer is what that middleware is there
// to send on time. The detaching then follows from a goroutine of its own, as soon as the
// call is done, and always before the handler is: a handler holding the lock is
// still running, so its context has not been given back yet.
func (m *Middleware) detachSoon(rctx *fasthttp.RequestCtx) {
	if !m.mu.TryLock() {
		go m.detach(rctx)
		return
	}
	session := m.Session
	m.ctx = nil
	m.mu.Unlock()
	if session != nil {
		session.detachSoon(rctx)
	}
}

// detachIfUnwound ends the request of a middleware a panic unwound, and is also what the
// release of an outer middleware that finishes first does to the inner one.
func (m *Middleware) detachIfUnwound() {
	if m.is(lifeUnwound) {
		m.detachSoon(nil)
	}
}
