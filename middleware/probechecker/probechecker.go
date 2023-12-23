package probechecker

import (
	"github.com/gofiber/fiber/v2"
)

// ProbeChecker defines a function to check liveness or readiness of the application
type ProbeChecker func(*fiber.Ctx) bool

// ProbeCheckerHandler defines a function that returns a ProbeChecker
type ProbeCheckerHandler func(ProbeChecker) fiber.Handler

func probeCheckerHandler(checker ProbeChecker) fiber.Handler {
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

	isLiveHandler := probeCheckerHandler(cfg.IsLive)
	isReadyHandler := probeCheckerHandler(cfg.IsReady)

	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if c.Method() != fiber.MethodGet {
			return c.Next()
		}

		switch c.Path() {
		case cfg.IsReadyEndpoint:
			return isReadyHandler(c)
		case cfg.IsLiveEndpoint:
			return isLiveHandler(c)
		}

		return c.Next()
	}
}
