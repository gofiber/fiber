package timeout

import (
	"context"
	"errors"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/ctxlocal"
	"github.com/gofiber/fiber/v3/log"
)

type contextKey int

const (
	// timeoutContextKey holds the context a timeout.New handed to its handler goroutine.
	timeoutContextKey contextKey = iota
	// requestKey holds the *request the timeout.New of one request share.
	requestKey
)

// request is shared by the nested timeout.New of one request, whose timers fire on different goroutines.
type request struct {
	mu       sync.Mutex  // held while one of them answers, so only one writes the timeout response
	answered atomic.Bool // set once one has started to answer
}

// snapshot is what OnTimeout sees after the deadline, taken before the handler starts:
// from then on the handler owns ctx and may still write any of it.
type snapshot struct {
	locals []local
	uri    fasthttp.URI
	req    fasthttp.RequestHeader
	resp   fasthttp.ResponseHeader
}

type local struct{ key, value any }

var snapshotPool = sync.Pool{New: func() any { return new(snapshot) }}

// New enforces a timeout for each incoming request. It replaces the request's
// context with one that has the configured deadline, which is exposed through
// c.Context(). Handlers can detect the timeout by listening on c.Context().Done()
// and return early.
//
// When a timeout occurs, the middleware returns immediately with fiber.ErrServiceUnavailable
// (or the error OnTimeout returns, if it returns one). The status is 503, not 408: a slow handler
// is a server-side delay, and 408 (RFC 9110 §15.5.9) invites the client to resend a
// request whose handler may still be running. The handler goroutine can continue
// safely, and resources are recycled when it finishes via the Abandon/ForceRelease
// mechanism.
func New(h fiber.Handler, config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	return func(ctx fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(ctx) {
			return h(ctx)
		}

		timeout := cfg.Timeout
		if timeout <= 0 {
			return h(ctx)
		}

		// Create timeout context - handler can check c.Context().Done()
		parent := ctx.Context()
		deadline := time.Now().Add(timeout)
		req, nested := ctx.Locals(requestKey).(*request)
		// An enclosing timeout.New whose deadline fires first, or one that already answered,
		// leaves nothing to guard: a goroutine of our own would time the ctx out a second time.
		if nested && (req.answered.Load() || enclosingFiresFirst(parent, ctx, deadline)) {
			err := runInline(h, ctx)
			if parent.Err() != nil || req.answered.Load() {
				// The enclosing timeout.New answers this request.
				return err
			}
			return handleResult(err, ctx, cfg)
		}
		if !nested {
			req = new(request)
			_ = ctxlocal.Set(ctx, requestKey, req)
		}
		// The deadline enclosingFiresFirst judged, not a later time.Now().
		tCtx, cancel := context.WithDeadline(parent, deadline)
		ctx.SetContext(tCtx)
		_ = ctxlocal.Set(ctx, timeoutContextKey, tCtx)
		var snap *snapshot
		if cfg.OnTimeout != nil {
			snap = takeSnapshot(ctx.RequestCtx())
			defer snap.release()
		}

		// Channel for handler result, fiber.ErrInternalServerError after a panic
		done := make(chan error, 1)
		// handlerDone is closed once the handler goroutine has fully exited
		// (covering both the normal and panic paths). It tells the timeout path
		// when the goroutine has stopped touching the context, which is what makes
		// reclaiming the abandoned context race-free.
		handlerDone := make(chan struct{})

		// Run handler in goroutine so we can race against the timeout
		go func() {
			defer close(handlerDone)
			defer func() {
				if p := recover(); p != nil {
					log.Errorw("panic recovered in timeout handler", "panic", p, "stack", string(debug.Stack()))
					select {
					case done <- fiber.ErrInternalServerError:
					default:
						// Middleware already returned, panic discarded
					}
				}
			}()
			err := h(ctx)
			select {
			case done <- err:
			default:
				// Middleware already returned, error discarded
			}
		}()

		// Wait for handler completion, panic, or timeout
		select {
		case err := <-done:
			// Handler finished or panicked - cleanup and return
			cancel()
			if req.answered.Load() {
				// An inner timeout.New answered, and the handler it gave up on may still read the context.
				return err
			}
			ctx.SetContext(parent)
			return handleResult(err, ctx, cfg)

		case <-tCtx.Done():
			// Timeout occurred - abandon context and return immediately.
			// Reclamation is scheduled so the abandoned fiber.Ctx is returned to
			// the pool once the handler goroutine finishes, instead of leaking.
			return handleTimeout(ctx, cancel, handlerDone, req, snap, cfg)
		}
	}
}

// enclosingFiresFirst reports whether parent's deadline is that of an enclosing timeout.New
// and comes no later than deadline. A deadline set elsewhere does not count: nothing answers when it fires.
func enclosingFiresFirst(parent context.Context, ctx fiber.Ctx, deadline time.Time) bool {
	enclosing, ok := ctx.Locals(timeoutContextKey).(context.Context)
	if !ok {
		return false
	}
	parentDeadline, ok := parent.Deadline()
	if !ok || parentDeadline.After(deadline) {
		return false
	}
	enclosingDeadline, ok := enclosing.Deadline()
	return ok && enclosingDeadline.Equal(parentDeadline)
}

// runInline runs h on the calling goroutine with the panic handling of the goroutine path.
func runInline(h fiber.Handler, ctx fiber.Ctx) (err error) { //nolint:nonamedreturns // Uses recover() to overwrite the error
	defer func() {
		if p := recover(); p != nil {
			log.Errorw("panic recovered in timeout handler", "panic", p, "stack", string(debug.Stack()))
			err = fiber.ErrInternalServerError
		}
	}()
	return h(ctx)
}

// handleResult processes the handler's return value
func handleResult(err error, ctx fiber.Ctx, cfg Config) error {
	if err != nil && isTimeoutError(err, cfg.Errors) {
		return invokeOnTimeout(ctx, cfg)
	}
	return err
}

// handleTimeout handles the timeout case using the Abandon mechanism
func handleTimeout(
	ctx fiber.Ctx,
	cancel context.CancelFunc,
	handlerDone <-chan struct{},
	req *request,
	snap *snapshot,
	cfg Config,
) error {
	req.mu.Lock()
	defer req.mu.Unlock()

	var timeoutErr error
	switch {
	case req.answered.Swap(true):
		// Another timeout.New of this request answered and fasthttp holds its response,
		// so only wait for our goroutine too.
		timeoutErr = fiber.ErrServiceUnavailable
	case cfg.OnTimeout == nil:
		// Build the default timeout response separately from the RequestCtx response.
		// The timed-out handler can keep running and may have already buffered
		// application data there, so reusing it could disclose partial output.
		// TimeoutErrorWithCode constructs a fresh fasthttp.Response internally, so
		// the active RequestCtx response is never read.
		ctx.RequestCtx().TimeoutErrorWithCode(fiber.ErrServiceUnavailable.Message, fiber.StatusServiceUnavailable)
		timeoutErr = fiber.ErrServiceUnavailable
	default:
		var resp *fasthttp.Response
		resp, timeoutErr = snap.runOnTimeout(ctx, cfg)

		// If the response is still the default 200/empty, ensure a sensible timeout
		// response is captured for fasthttp to send. ResetBody closes a body
		// stream rather than draining it, so a reader OnTimeout never finishes
		// writing cannot block this path.
		if resp.StatusCode() == fiber.StatusOK && timeoutResponseUnwritten(resp) {
			resp.ResetBody()
			// An error OnTimeout returned is the response the client will see.
			status, message := fiber.StatusServiceUnavailable, fiber.ErrServiceUnavailable.Message
			var fiberErr *fiber.Error
			if errors.As(timeoutErr, &fiberErr) && fiberErr != nil {
				status, message = fiberErr.Code, fiberErr.Message
			}
			resp.SetStatusCode(status)
			resp.SetBodyString(message)
		}

		// Tell fasthttp to not recycle the RequestCtx - it will acquire a new one
		// for the response and send the captured payload from OnTimeout. All ctx
		// mutations after this call are ignored by fasthttp.
		ctx.RequestCtx().TimeoutErrorWithResponse(resp)
		if timeoutErr == nil {
			// An error keeps the middleware before us off the response the handler still writes.
			timeoutErr = fiber.ErrServiceUnavailable
		}
	}

	// Schedule race-free reclamation of the abandoned context. The context is
	// returned to the pool only after BOTH the handler goroutine finishes AND
	// Fiber's requestHandler releases the context (after any ErrorHandler runs),
	// so we never race with goroutines still using it. This fixes the unbounded
	// fiber.Ctx leak that previously affected every timed-out request (#4359).
	//
	// Use a concrete type assertion to *fiber.DefaultCtx rather than an interface
	// assertion, because only *fiber.DefaultCtx has the signalReleased wiring in
	// ReleaseCtx/releaseDefaultCtx. An interface assertion could accidentally match
	// custom Ctx implementations that provide ScheduleReclaim but lack the release
	// signal, causing the reclamation goroutine to block forever.
	if dc, ok := ctx.(*fiber.DefaultCtx); ok {
		dc.ScheduleReclaim(handlerDone, cancel)
		return timeoutErr
	}

	// Custom context implementations stay out of the pool, so only cancel the timeout context
	// once the handler finishes: restoring the parent would race an inner timeout's handler.
	//
	// Mark the context as abandoned so ReleaseCtx will skip pooling, allowing the
	// handler goroutine to continue safely using the context.
	ctx.Abandon()
	go func() {
		<-handlerDone
		cancel()
	}()

	return timeoutErr
}

// timeoutResponseUnwritten reports whether the response still carries nothing a
// client could use. A stream counts as unwritten, since Response.CopyTo does not
// carry one over, and is checked first because Body would drain it.
func timeoutResponseUnwritten(resp *fasthttp.Response) bool {
	return resp.IsBodyStream() || len(resp.Body()) == 0
}

func takeSnapshot(fctx *fasthttp.RequestCtx) *snapshot {
	snap, ok := snapshotPool.Get().(*snapshot)
	if !ok {
		snap = new(snapshot)
	}
	// The body is left out: copying it would cost every request, and aliasing it is unsafe.
	fctx.Request.URI().CopyTo(&snap.uri)
	fctx.Request.Header.CopyTo(&snap.req)
	fctx.Response.Header.CopyTo(&snap.resp)
	fctx.VisitUserValuesAll(func(key, value any) {
		snap.locals = append(snap.locals, local{key: key, value: value})
	})
	return snap
}

// runOnTimeout runs OnTimeout on a ctx of its own and returns the response it wrote.
func (s *snapshot) runOnTimeout(ctx fiber.Ctx, cfg Config) (*fasthttp.Response, error) {
	orig := ctx.RequestCtx()
	fctx := new(fasthttp.RequestCtx)
	// The real conn keeps IsTLS, LocalAddr and the TLS state; fctx is never served on it.
	fctx.Init2(orig.Conn(), nil, true)
	fctx.SetRemoteAddr(orig.RemoteAddr())
	s.req.CopyTo(&fctx.Request.Header)
	s.uri.CopyTo(fctx.Request.URI())
	s.resp.CopyTo(&fctx.Response.Header)
	for _, l := range s.locals {
		fctx.SetUserValue(l.key, l.value)
	}

	app := ctx.App()
	c := app.AcquireCtx(fctx)
	err := cfg.OnTimeout(c)
	app.ReleaseCtx(c)
	return &fctx.Response, err
}

func (s *snapshot) release() {
	s.uri.Reset()
	s.req.Reset()
	s.resp.Reset()
	clear(s.locals)
	s.locals = s.locals[:0]
	snapshotPool.Put(s)
}

// invokeOnTimeout calls the OnTimeout handler if configured
func invokeOnTimeout(ctx fiber.Ctx, cfg Config) error {
	if cfg.OnTimeout != nil {
		return cfg.OnTimeout(ctx)
	}
	return fiber.ErrServiceUnavailable
}

// isTimeoutError checks if err is a timeout-like error (context.DeadlineExceeded
// or any of the custom errors).
func isTimeoutError(err error, customErrors []error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if len(customErrors) > 0 {
		for _, e := range customErrors {
			if errors.Is(err, e) {
				return true
			}
		}
	}
	return false
}
