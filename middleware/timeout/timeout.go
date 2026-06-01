package timeout

import (
	"context"
	"errors"
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// New enforces a timeout for each incoming request. It replaces the request's
// context with one that has the configured deadline, which is exposed through
// c.Context(). Handlers can detect the timeout by listening on c.Context().Done()
// and return early.
//
// When a timeout occurs, the middleware returns immediately with fiber.ErrRequestTimeout
// (or the result of OnTimeout if configured). The handler goroutine can continue
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
		tCtx, cancel := context.WithTimeout(parent, timeout)
		ctx.SetContext(tCtx)

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
	// Mark fiber context as abandoned - ReleaseCtx will skip pooling so the
	// handler goroutine can keep using the context safely after we return.
	//
	// This is the same approach fasthttp uses - timed-out RequestCtx objects
	// are never returned to the pool (see fasthttp's releaseCtx which panics
	// if timeoutResponse is set).
	ctx.Abandon()

	// Prepare the timeout response before marking the RequestCtx as timed out so
	// custom OnTimeout handlers can shape the response body.
	timeoutErr := invokeOnTimeout(ctx, cfg)

	// If no OnTimeout handler is configured or the response is still the default
	// 200/empty, ensure a sensible timeout response is captured for fasthttp to send.
	if cfg.OnTimeout == nil || (ctx.Response().StatusCode() == fiber.StatusOK && len(ctx.Response().Body()) == 0) {
		ctx.Response().SetStatusCode(fiber.StatusRequestTimeout)
		if len(ctx.Response().Body()) == 0 {
			ctx.Response().SetBodyString(fiber.ErrRequestTimeout.Message)
		}
	}

	// Tell fasthttp to not recycle the RequestCtx - it will acquire a new one
	// for the response and send the captured payload (either default or from
	// OnTimeout). All ctx mutations after this call are ignored by fasthttp.
	ctx.RequestCtx().TimeoutErrorWithResponse(&ctx.RequestCtx().Response)

	// Schedule race-free reclamation of the abandoned context. The context is
	// returned to the pool only after BOTH the handler goroutine finishes AND
	// Fiber's requestHandler releases the context (after any ErrorHandler runs),
	// so we never race with goroutines still using it. This fixes the unbounded
	// fiber.Ctx leak that previously affected every timed-out request (#4359).
	if r, ok := ctx.(interface {
		ScheduleReclaim(<-chan struct{}, context.CancelFunc)
	}); ok {
		r.ScheduleReclaim(handlerDone, cancel)
		return timeoutErr
	}

	// Custom context implementations that do not support ScheduleReclaim fall
	// back to the previous behavior: once the handler finishes, cancel the
	// timeout context and restore the parent. The context stays out of the pool.
	go func() {
		<-handlerDone
		cancel()
		ctx.SetContext(parent)
	}()

	return timeoutErr
}

// invokeOnTimeout calls the OnTimeout handler if configured
func invokeOnTimeout(ctx fiber.Ctx, cfg Config) error {
	if cfg.OnTimeout != nil {
		return cfg.OnTimeout(ctx)
	}
	return fiber.ErrRequestTimeout
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
