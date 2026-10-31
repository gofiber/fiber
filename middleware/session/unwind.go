package session

import (
	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// unwoundKey is the request value under which a middleware a panic unwound is
// kept, so that fasthttp closes it when it resets the request. A key of its own:
// the Fiber locals of a custom Ctx need not be the request's values.
type unwoundKey struct{}

// unwoundUnlessFinished is the deferred call of serve: it ends the middleware when
// a panic unwinds it. Once serve finished, the middleware was released or cut off,
// and may already belong to another request, so nothing is done.
func (m *Middleware) unwoundUnlessFinished(finished *bool, c fiber.Ctx) {
	if !*finished {
		m.unwound(c)
	}
}

// unwound ends a middleware that a panic unwound, whose release never runs. The
// outer middleware gets its count back, so that it is recycled when it finishes
// instead of counting this one as running inside for good. The middleware and its
// session let go of the fiber.Ctx, which goes back to the pool as soon as the
// handlers return, where the next request takes it; a goroutine of the handler
// that still held the session would otherwise write its cookie into that request,
// or run its storage calls under it. The session keeps the request itself, which
// lives on until fasthttp resets it: whatever recovers the panic can still reach
// the middleware, its session and the one it runs inside, change the storage
// through them and set the cookies, as before the panic; the middleware is kept in
// the request's values, so that fasthttp closes it when it resets the request, and
// the session lets go of the request then, before fasthttp reuses it (Close). An
// outer middleware that finishes first closes it itself. Neither is saved or
// recycled, and the middleware stays the current one of the request, as one
// without an outer middleware does; it is marked unwound, so that a Go context
// kept past the request does not find it. Nothing here waits for a lock, and the
// Ctx is asked only for the request: a panic in the locked part of the release
// leaves the lock held, and the Ctx may be what panicked.
func (m *Middleware) unwound(c fiber.Ctx) {
	m.unpin()
	m.isUnwound.Store(true)
	rctx := c.RequestCtx()
	if rctx != nil {
		rctx.SetUserValue(unwoundKey{}, unwoundCloser{m})
	}
	// The session keeps the request until it ends; a Ctx that has none lets go of it too.
	m.cutSoon(rctx)
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
	u.m.endRequest()
	return nil
}

// cut cuts m and its session off from the fiber.Ctx of the request, and has the
// session keep rctx, the request itself, instead, when it is not nil (see
// Session.cut). It waits for the locks.
func (m *Middleware) cut(rctx *fasthttp.RequestCtx) {
	m.mu.Lock()
	session := m.Session
	m.ctx = nil
	m.mu.Unlock()
	if session != nil {
		session.cut(rctx)
	}
}

// cutSoon is cut for the request that the timeout middleware answered, or one that a
// panic unwound: m and its session stay as they are for the handlers that outlive
// the request, and the response is fixed, so nothing those handlers write to it
// counts. What they must not do is hold the session past their end and write into
// whichever request gets the context next, or run storage calls under it.
//
// The answer does not wait for the cut. A handler the timeout middleware gave up on
// can be inside a call that holds the lock of m or of the session across the
// storage, such as Destroy or Save, and the answer is what that middleware is there
// to send on time. The cut then follows from a goroutine of its own, as soon as the
// call is done, and always before the handler is: a handler holding the lock is
// still running, so its context has not been given back yet.
func (m *Middleware) cutSoon(rctx *fasthttp.RequestCtx) {
	if !m.mu.TryLock() {
		go m.cut(rctx)
		return
	}
	session := m.Session
	m.ctx = nil
	m.mu.Unlock()
	if session != nil {
		session.cutSoon(rctx)
	}
}

// endRequest ends the request of a middleware a panic unwound, and is also what the
// release of an outer middleware that finishes first does to the inner one.
func (m *Middleware) endRequest() {
	if m.isUnwound.Load() {
		m.cutSoon(nil)
	}
}
