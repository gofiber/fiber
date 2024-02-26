package healthcheck

import (
	"github.com/gofiber/fiber/v3"
)

// HealthChecker defines a function to check liveness or readiness of the application
type HealthChecker func(fiber.Ctx) bool

func NewHealthChecker(config ...Config) fiber.Handler {
	cfg := defaultConfigV3(config...)

	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if c.Method() != fiber.MethodGet {
			return c.Next()
		}

		if cfg.Probe(c) {
			return c.SendStatus(fiber.StatusOK)
		}

		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
}
