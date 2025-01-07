package timeout

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
)

// New sets a request timeout, runs the handler in a separate Goroutine, and
// returns fiber.ErrRequestTimeout when the timeout or any of the specified errors occur.
func New(h fiber.Handler, timeout time.Duration, tErrs ...error) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Create a context with a timeout
		ctx, cancel := context.WithTimeout(c.Context(), timeout)
		defer cancel()

		// Attach the new context to the Fiber context
		c.SetContext(ctx)

		// Channel to capture the handler's result (error)
		done := make(chan error, 1)

		// Execute the handler in a separate Goroutine
		go func() {
			done <- h(c)
		}()

		// Wait for either the timeout or the handler to finish
		select {
		case <-ctx.Done():
			// Triggered if the timeout occurs or the context is canceled
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fiber.ErrRequestTimeout
			}
			// For other context cancellations, we can still treat them the same
			return fiber.ErrRequestTimeout

		case err := <-done:
			// If the handler returned an error
			if err != nil {
				// Check if it's a deadline exceeded error
				if errors.Is(err, context.DeadlineExceeded) {
					return fiber.ErrRequestTimeout
				}
				// Check against any custom errors in the list
				for _, timeoutErr := range tErrs {
					if errors.Is(err, timeoutErr) {
						return fiber.ErrRequestTimeout
					}
				}
			}
			// Otherwise, return the handler's error or nil
			return err
		}
	}
}
