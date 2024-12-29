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
		// Skip load-shedding logic for requests matching the exclusion criteria
		if exclude != nil && exclude(c) {
			return c.Next()
		}

		// Create a context with a timeout for the current request
		ctx, cancel := context.WithTimeout(c.Context(), timeout)
		defer cancel()

		// Set the new context with a timeout
		c.SetContext(ctx)

		// Process the request and capture any error
		err := c.Next()

		// Create a channel to signal when request processing completes
		done := make(chan error, 1)

		// Send the result of the request processing to the channel
		go func() {
			done <- err
		}()

		// Handle either request completion or timeout
		select {
		case <-ctx.Done(): // Triggered if the timeout expires
			return loadSheddingHandler(c)
		case err := <-done: // Triggered if request processing completes
			return err
		}
	}
}
