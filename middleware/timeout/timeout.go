package timeout

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"
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

		// Run handler in goroutine so we can race against the timeout
		go func() {
			defer func() {
				if p := recover(); p != nil {
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

		case p := <-panicChan:
			// Handler panicked - cleanup and return error
			cancel()
			ctx.SetContext(parent)
			_ = p // TODO: consider logging
			return fiber.ErrInternalServerError

		case <-tCtx.Done():
			// Timeout occurred - abandon context and return immediately
			// The cleanup goroutine will release ctx when handler finishes
			return handleTimeout(parent, ctx, cancel, done, panicChan, cfg)
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
	done <-chan error,
	panicChan <-chan any,
	cfg Config,
) error {
	// Mark fiber context as abandoned - ReleaseCtx will skip pooling.
	// The context will NOT be returned to the pool. This is an intentional
	// trade-off: we accept the small memory cost of not recycling timed-out
	// contexts in exchange for complete race-freedom.
	//
	// This is the same approach fasthttp uses - timed-out RequestCtx objects
	// are never returned to the pool (see fasthttp's releaseCtx which panics
	// if timeoutResponse is set).
	ctx.Abandon()

	// Tell fasthttp to not recycle the RequestCtx - it will acquire a new one
	// for the response. This prevents race conditions where the handler goroutine
	// still accesses the RequestCtx while fasthttp tries to reset it.
	ctx.RequestCtx().TimeoutErrorWithCode("Request Timeout", fiber.StatusRequestTimeout)

	// Spawn cleanup goroutine that waits for handler to finish.
	// This only does context cleanup (cancel + restore parent), NOT ctx release.
	// The fiber.Ctx is intentionally NOT released to avoid races with requestHandler
	// which may still access ctx (e.g., ErrorHandler) after this function returns.
	go func() {
		select {
		case <-done:
		case <-panicChan:
		}
		// Handler finished - cancel timeout context and restore parent
		cancel()
		ctx.SetContext(parent)

		// TODO: Currently the ctx is not returned to the pool (memory leak for timed-out requests).
		// Future improvement: Implement a concurrent "garbage collector" list where abandoned
		// contexts are queued after both the handler AND requestHandler are done. A background
		// goroutine would periodically process this list and call ForceRelease() to recycle
		// the contexts safely. This would require tracking when requestHandler finishes
		// (e.g., via a channel signaled in ReleaseCtx) without adding per-request overhead
		// for non-timeout cases.
	}()

	return invokeOnTimeout(ctx, cfg)
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
