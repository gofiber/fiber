package timeout

import (
	"context"
	"errors"
	"runtime/debug"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/ctxlocal"
	"github.com/gofiber/fiber/v3/log"
)

type contextKey int

// timeoutContextKey holds the context a timeout.New handed to its handler goroutine.
const timeoutContextKey contextKey = 0

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
		// An enclosing timeout.New whose deadline fires first already guards h; a goroutine
		// of our own would time out and abandon the same ctx a second time.
		if enclosingFiresFirst(parent, ctx, timeout) {
			return handleResult(runInline(h, ctx), ctx, cfg)
		}
		tCtx, cancel := context.WithTimeout(parent, timeout)
		ctx.SetContext(tCtx)
		_ = ctxlocal.Set(ctx, timeoutContextKey, tCtx)

		// Channels for handler result and panics
		done := make(chan error, 1)
		panicChan := make(chan any, 1)
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
					case panicChan <- p:
					default:
						// Middleware already returned, panic value discarded
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
			// Handler finished normally - cleanup and return
			cancel()
			ctx.SetContext(parent)
			return handleResult(err, ctx, cfg)

		case <-panicChan:
			// Handler panicked - cleanup and return error
			cancel()
			ctx.SetContext(parent)
			return fiber.ErrInternalServerError

		case <-tCtx.Done():
			// Timeout occurred - abandon context and return immediately.
			// Reclamation is scheduled so the abandoned fiber.Ctx is returned to
			// the pool once the handler goroutine finishes, instead of leaking.
			return handleTimeout(parent, ctx, cancel, handlerDone, cfg)
		}
	}
}

// enclosingFiresFirst reports whether parent's deadline is that of an enclosing timeout.New
// and comes no later than timeout. A deadline set elsewhere does not count: nothing answers when it fires.
func enclosingFiresFirst(parent context.Context, ctx fiber.Ctx, timeout time.Duration) bool {
	enclosing, ok := ctx.Locals(timeoutContextKey).(context.Context)
	if !ok {
		return false
	}
	deadline, ok := parent.Deadline()
	if !ok || deadline.After(time.Now().Add(timeout)) {
		return false
	}
	enclosingDeadline, ok := enclosing.Deadline()
	return ok && enclosingDeadline.Equal(deadline)
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
	parent context.Context,
	ctx fiber.Ctx,
	cancel context.CancelFunc,
	handlerDone <-chan struct{},
	cfg Config,
) error {
	var timeoutErr error
	switch {
	case ctx.IsAbandoned() && ctx.RequestCtx().LastTimeoutErrorResponse() != nil:
		// An inner timeout.New answered first and fasthttp holds its response, so only
		// wait for our goroutine too. IsAbandoned orders the read after that write.
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
		// Prepare the timeout response before marking the RequestCtx as timed out so
		// custom OnTimeout handlers can shape the response body.
		timeoutErr = invokeOnTimeout(ctx, cfg)

		// If the response is still the default 200/empty, ensure a sensible timeout
		// response is captured for fasthttp to send. ResetBody closes a body
		// stream rather than draining it, so a reader the timed-out handler never
		// finishes writing cannot block this path.
		resp := ctx.Response()
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
		ctx.RequestCtx().TimeoutErrorWithResponse(&ctx.RequestCtx().Response)
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

	// Custom context implementations fall back to the previous behavior: once the
	// handler finishes, cancel the timeout context and restore the parent. The
	// context stays out of the pool.
	//
	// Mark the context as abandoned so ReleaseCtx will skip pooling, allowing the
	// handler goroutine to continue safely using the context.
	ctx.Abandon()
	go func() {
		<-handlerDone
		cancel()
		ctx.SetContext(parent)
	}()

	return timeoutErr
}

// timeoutResponseUnwritten reports whether the response still carries nothing a
// client could use. A stream counts as unwritten, since Response.CopyTo does not
// carry one over, and is checked first because Body would drain it.
func timeoutResponseUnwritten(resp *fasthttp.Response) bool {
	return resp.IsBodyStream() || len(resp.Body()) == 0
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
