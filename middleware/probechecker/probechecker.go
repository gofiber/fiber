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

const (
	DefaultLivenessEndpoint  = "/livez"
	DefaultReadinessEndpoint = "/readyz"
)

func defaultLiveFunc(c *fiber.Ctx) bool { return true }

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

func New(config ...Config) fiber.Handler {
	cfg := defaultConfig(config...)

	checkers := map[string]fiber.Handler{
		"liveness":  probeCheckerHandler(cfg.IsLive),
		"readiness": probeCheckerHandler(cfg.IsReady),
	}

	return func(c *fiber.Ctx) error {
		route := c.Route()
		routeType := checkRoute(route.Path, &cfg)

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

func defaultConfig(config ...Config) Config {
	if len(config) < 1 {
		return Config{
			IsLive:          defaultLiveFunc,
			IsLiveEndpoint:  DefaultLivenessEndpoint,
			IsReadyEndpoint: DefaultReadinessEndpoint,
		}
	}

	cfg := config[0]

	if cfg.IsLiveEndpoint == "" {
		cfg.IsLiveEndpoint = DefaultLivenessEndpoint
	}
	if cfg.IsReadyEndpoint == "" {
		cfg.IsReadyEndpoint = DefaultReadinessEndpoint
	}
	if cfg.IsLive == nil {
		cfg.IsLive = defaultLiveFunc
	}

	return cfg
}
