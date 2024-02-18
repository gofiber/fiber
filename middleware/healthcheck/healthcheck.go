package healthcheck

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// HealthChecker defines a function to check liveness or readiness of the application
type HealthChecker func(*fiber.Ctx) bool

// ProbeCheckerHandler defines a function that returns a ProbeChecker
type HealthCheckerHandler func(HealthChecker) fiber.Handler

func healthCheckerHandler(checker HealthChecker) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if checker == nil {
			return c.Next()
		}

		if checker(c) {
			return c.SendStatus(fiber.StatusOK)
		}

		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
}

func New(config ...Config) fiber.Handler {
	cfg := defaultConfig(config...)

	isLiveHandler := healthCheckerHandler(cfg.LivenessProbe)
	isReadyHandler := healthCheckerHandler(cfg.ReadinessProbe)

	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if c.Method() != fiber.MethodGet {
			return c.Next()
		}

		// Find the start of the last segment
		lastSlashIndex := strings.LastIndex(c.Path(), "/")
		lastSegment := c.Path()[lastSlashIndex:] // +1 to skip the slash itself

		// Direct string comparison for the last segment of the path
		if lastSegment == cfg.ReadinessEndpoint {
			return isReadyHandler(c)
		} else if lastSegment == cfg.LivenessEndpoint {
			return isLiveHandler(c)
		}

		return c.Next()
	}
}
