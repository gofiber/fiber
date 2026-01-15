package timeout

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
)

// New enforces a timeout for each incoming request. It replaces the request's
// context with one that has the configured deadline, which is exposed through
// c.Context(). Handlers can detect the timeout by listening on c.Context().Done()
// and return early. If the handler returns a timeout-related error or the context
// deadline is exceeded, fiber.ErrRequestTimeout is returned.
//
// Note: The middleware waits for the handler to complete to avoid race conditions
// with Fiber's context pooling. Handlers should check c.Context().Done() to
// return early when a timeout occurs.
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

		// Channels for handler result and panics.
		// Both channels are buffered (size 1) so the handler goroutine can report
		// even if the timeout fires. We still wait for the goroutine to finish
		// (see select below) to avoid leaking it or accessing a pooled context
		// after the middleware returns.
		done := make(chan error, 1)
		panicChan := make(chan any, 1)

		// Run handler in goroutine so it can be interrupted by context cancellation
		go func() {
			defer func() {
				if p := recover(); p != nil {
					panicChan <- p
				}
			}()
			done <- h(ctx)
		}()

		// Wait for handler completion, panic, or timeout.
		// We still try to wait for the handler to finish to avoid races with Fiber's
		// context pooling, but we bound that wait so a hung handler cannot block
		// this middleware forever.
		var err error
		var panicked bool
		var timedOut bool

		select {
		case err = <-done:
			// Handler finished
		case <-panicChan:
			// Handler panicked
			panicked = true
		case <-tCtx.Done():
			// Timeout fired before handler returned
			timedOut = true
		}

		if timedOut && !panicked && err == nil {
			// Give the handler a bounded grace period to exit after cancellation.
			// This avoids blocking forever on a misbehaving handler, while still
			// reducing the chance of racing with context reuse.
			grace := cfg.Timeout
			if grace <= 0 {
				grace = 50 * time.Millisecond
			}
			select {
			case err = <-done:
				// Handler finished after timeout
			case <-panicChan:
				panicked = true
			case <-time.After(grace):
				// Handler still stuck; proceed with timeout response.
				err = context.DeadlineExceeded
			}
		}

		// Check if timeout occurred BEFORE canceling (cancel() would set Err())
		contextTimedOut := timedOut || errors.Is(tCtx.Err(), context.DeadlineExceeded)

		// Restore parent context and cancel timeout context
		cancel()
		ctx.SetContext(parent)

		// Handle panic
		if panicked {
			return fiber.ErrInternalServerError
		}

		// Check if timeout occurred (handler returned because context was canceled)
		// or if handler returned a timeout-like error
		if contextTimedOut || (err != nil && isTimeoutError(err, cfg.Errors)) {
			if cfg.OnTimeout != nil {
				if toErr := cfg.OnTimeout(ctx); toErr != nil {
					return toErr
				}
			}
			return fiber.ErrRequestTimeout
		}

		return err
	}
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
