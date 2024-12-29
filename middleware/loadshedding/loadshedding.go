package loadshedding

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
)

// New creates a middleware handler enforces a timeout on request processing to manage server load.
// If a request exceeds the specified timeout, a custom load-shedding handler is executed.
func New(timeout time.Duration, loadSheddingHandler fiber.Handler, exclude func(fiber.Ctx) bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Skip load-shedding for excluded requests
		if exclude != nil && exclude(c) {
			return c.Next()
		}

		// Create a context with the specified timeout
		ctx, cancel := context.WithTimeout(c.Context(), timeout)
		defer cancel()

		// Channel to signal request completion
		done := make(chan error, 1)

		// Capture the current handler execution
		handler := func() error {
			return c.Next()
		}

		// Process the handler in a separate goroutine
		go func() {
			done <- handler()
		}()

		select {
		case <-ctx.Done():
			// Timeout occurred; invoke the load-shedding handler
			return loadSheddingHandler(c)
		case err := <-done:
			// Request completed successfully; return any handler error
			return err
		}
	}
}
