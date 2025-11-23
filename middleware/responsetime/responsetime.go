package responsetime

import (
	"time"

	"github.com/gofiber/fiber/v3"
)

// New creates a new middleware handler.
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		start := time.Now()

		err := c.Next()

		c.Set(cfg.Header, time.Since(start).String())

		return err
	}
}
