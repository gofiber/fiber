package session

import (
	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// requestCtx returns the context of the request the session belongs to, or nil
// if it has none, or none any more.
func (s *Session) requestCtx() fiber.Ctx {
	s.mu.RLock()
	c := s.ctx
	s.mu.RUnlock()
	return c
}

// requestResponse returns the request and the response the session writes its ID
// to: those of its fiber.Ctx, or of the request itself once the middleware was
// unwound, or nil when the session has let go of the request. The caller holds s.mu.
func (s *Session) requestResponse() (*fasthttp.Request, *fasthttp.Response) {
	if s.ctx != nil {
		return s.ctx.Request(), s.ctx.Response()
	}
	if s.rctx != nil {
		return &s.rctx.Request, &s.rctx.Response
	}
	return nil, nil
}

// detach points the session at rctx, the request itself, instead of at the fiber.Ctx
// of its request, or at nothing when rctx is nil. The session of an abandoned or
// unwound request is not recycled, since a handler that outlives the request may
// still be using it, but the fiber.Ctx is: it is reused for another request once
// those handlers are done, and whoever still holds the session must not write to
// that request's response, or run its storage calls under its context. The request
// itself lives on until fasthttp resets it, and a session a panic unwound the
// middleware of still writes its cookies there, for whatever recovers the panic.
func (s *Session) detach(rctx *fasthttp.RequestCtx) {
	s.mu.Lock()
	s.ctx, s.rctx = nil, rctx
	s.mu.Unlock()
}

// detachSoon is detach for a caller that must not wait: when the session is locked, by a
// save that is writing to the storage for instance, the detaching follows from a
// goroutine of its own.
func (s *Session) detachSoon(rctx *fasthttp.RequestCtx) {
	if s.mu.TryLock() {
		s.ctx, s.rctx = nil, rctx
		s.mu.Unlock()
		return
	}
	go s.detach(rctx)
}
