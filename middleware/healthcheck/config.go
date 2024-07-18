package healthcheck

import (
	"github.com/gofiber/fiber/v3"
)

// Config defines the configuration options for the healthcheck middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true. If this function returns true
	// and no other handlers are defined for the route, Fiber will return a status 404 Not Found, since
	// no other handlers were defined to return a different status.
	//
	// Optional. Default: nil
	Next func(fiber.Ctx) bool

	// Function used for checking the liveness of the application. Returns true if the application
	// is running and false if it is not. The liveness probe is typically used to indicate if
	// the application is in a state where it can handle requests (e.g., the server is up and running).
	//
	// Optional. Default: func(c fiber.Ctx) bool { return true }
	Probe HealthChecker
}

const (
	DefaultLivenessEndpoint  = "/livez"
	DefaultReadinessEndpoint = "/readyz"
	DefaultStartupEndpoint   = "/startupz"
)

func defaultProbe(fiber.Ctx) bool { return true }

func defaultConfigV3(config ...Config) Config {
	if len(config) < 1 {
		return Config{
			Probe: defaultProbe,
		}
	}

	cfg := config[0]

	if cfg.Probe == nil {
		cfg.Probe = defaultProbe
	}

	return cfg
}
