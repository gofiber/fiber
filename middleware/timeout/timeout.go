package timeout

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
)

// New enforces a timeout for each incoming request. If the timeout expires or
// any of the specified errors occur, fiber.ErrRequestTimeout is returned.
func New(h fiber.Handler, timeout time.Duration, tErrs ...error) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		// Create a context with the specified timeout; any operation exceeding
		// this deadline will be canceled automatically.
		timeoutContext, cancel := context.WithTimeout(ctx.Context(), timeout)
		defer cancel()

		// Attach the timeout-bound context to the current Fiber context.
		ctx.SetContext(timeoutContext)

		// Execute the wrapped handler synchronously.
		err := h(ctx)

		// If the context has timed out, return a request timeout error.
		if timeoutContext.Err() != nil && errors.Is(timeoutContext.Err(), context.DeadlineExceeded) {
			return fiber.ErrRequestTimeout
		}

		// If the handler returned an error, check whether it's a deadline exceeded
		// error or any other listed timeout-triggering error.
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || isCustomError(err, tErrs) {
				return fiber.ErrRequestTimeout
			}
		}
		return err
	}
}

// isCustomError checks whether err matches any error in errList using errors.Is.
func isCustomError(err error, errList []error) bool {
	for _, e := range errList {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
