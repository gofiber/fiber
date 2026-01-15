package timeout

import (
	"context"
	"errors"

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

		// Channels for handler result and panics
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

		// Wait for handler completion or panic
		// We must wait for handler to finish to avoid race conditions with ctx
		var err error
		var panicked bool

		select {
		case err = <-done:
			// Handler finished
		case <-panicChan:
			// Handler panicked
			panicked = true
		}

		// Check if timeout occurred BEFORE canceling (cancel() would set Err())
		contextTimedOut := errors.Is(tCtx.Err(), context.DeadlineExceeded)

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
