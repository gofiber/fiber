package timeout

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"
)

// New enforces a timeout for each incoming request. It replaces the request's
// context with one that has the configured deadline, which is exposed through
// c.Context(). If the timeout expires, the middleware returns immediately with
// fiber.ErrRequestTimeout, even if the handler is still running. The handler
// can detect the timeout via c.Context().Done().
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

		// Run handler in goroutine
		go func() {
			defer func() {
				if p := recover(); p != nil {
					panicChan <- p
				}
			}()
			done <- h(ctx)
		}()

		// Wait for handler completion or timeout
		select {
		case err := <-done:
			// Handler finished - cleanup and handle errors
			cancel()
			ctx.SetContext(parent)

			if err != nil && isTimeoutError(err, cfg.Errors) {
				if cfg.OnTimeout != nil {
					if toErr := cfg.OnTimeout(ctx); toErr != nil {
						return toErr
					}
				}
				return fiber.ErrRequestTimeout
			}
			return err

		case <-panicChan:
			// Handler panicked - treat as internal server error
			// We don't re-panic because we're in a different goroutine context
			cancel()
			ctx.SetContext(parent)
			return fiber.ErrInternalServerError

		case <-tCtx.Done():
			// Timeout reached - return immediately
			// Note: handler goroutine may still be running but we return immediately
			cancel()
			ctx.SetContext(parent)

			if cfg.OnTimeout != nil {
				return cfg.OnTimeout(ctx)
			}
			return fiber.ErrRequestTimeout
		}
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
