package probechecker

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

// ProbeChecker defines a function to check liveness or readiness of the application
type ProbeChecker func(*fiber.Ctx) bool

// ProbeCheckerHandler defines a function that returns a ProbeChecker
type ProbeCheckerHandler func(ProbeChecker) fiber.Handler

// Config is the config struct for the probechecker middleware
type Config struct {
	// Config for liveness probe of the container engine being used
	//
	// Optional. Default: func(c *Ctx) bool { return true }
	IsLive ProbeChecker

	// HTTP endpoint of the liveness probe
	//
	// Optional. Default: /livez
	IsLiveEndpoint string

	// Config for readiness probe of the container engine being used
	//
	// Optional. Default: nil
	IsReady ProbeChecker

	// HTTP endpoint of the readiness probe
	//
	// Optional. Default: /readyz
	IsReadyEndpoint string
}

var DefaultLiveFunc = func(c *fiber.Ctx) bool { return true }

const (
	DefaultLivenessEndpoint  = "/livez"
	DefaultReadinessEndpoint = "/readyz"
)

func probeCheckerHandler(checker ProbeChecker) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if checker(c) {
			return c.SendStatus(fiber.StatusOK)
		}

		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
}

func checkRoute(path string, config *Config) string {
	switch path {
	case DefaultLivenessEndpoint, config.IsLiveEndpoint:
		return "liveness"
	case DefaultReadinessEndpoint, config.IsReadyEndpoint:
		return "readiness"
	default:
		return ""
	}
}

func New(config *Config) fiber.Handler {
	if config.IsLiveEndpoint == "" {
		config.IsLiveEndpoint = DefaultLivenessEndpoint
	}
	if config.IsReadyEndpoint == "" {
		config.IsReadyEndpoint = DefaultReadinessEndpoint
	}
	if config.IsLive == nil {
		config.IsLive = DefaultLiveFunc
	}

	var checkers = map[string]fiber.Handler{
		"liveness":  probeCheckerHandler(config.IsLive),
		"readiness": probeCheckerHandler(config.IsReady),
	}

	return func(c *fiber.Ctx) error {
		route := c.Route()
		routeType := checkRoute(route.Path, config)

		if routeType != "" || route.Method != fiber.MethodGet {
			handler, ok := checkers[routeType]

			if !ok {
				return fmt.Errorf("routeType of %s not found in checkers", routeType)
			}

			return handler(c)
		}

		return c.Next()
	}
}
