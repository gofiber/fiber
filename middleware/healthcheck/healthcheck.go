package healthcheck

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

// HealthChecker defines a function to check liveness or readiness of the application
type HealthChecker func(fiber.Ctx) bool

// HealthCheckerHandler defines a function that returns a HealthChecker
type HealthCheckerHandler func(HealthChecker) fiber.Handler

func healthCheckerHandler(checker HealthChecker) fiber.Handler {
	return func(c fiber.Ctx) error {
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

	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if c.Method() != fiber.MethodGet {
			return c.Next()
		}

		prefixCount := len(strings.TrimRight(c.Route().Path, "/"))
		if len(c.Path()) >= prefixCount {
			checkPath := c.Path()[prefixCount:]
			checkPathTrimmed := checkPath
			if !c.App().Config().StrictRouting {
				checkPathTrimmed = strings.TrimRight(checkPath, "/")
			}
			switch {
			case checkPath == cfg.ReadinessEndpoint || checkPathTrimmed == cfg.ReadinessEndpoint:
				return isReadyHandler(c)
			case checkPath == cfg.LivenessEndpoint || checkPathTrimmed == cfg.LivenessEndpoint:
				return isLiveHandler(c)
			}
		}

		return c.Next()
	}
}
