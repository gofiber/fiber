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

	// Probe is executed to determine the current health state. It can be used for liveness,
	// readiness or startup checks. Returning true indicates the application is healthy.
	//
	// Optional. Default: func(c fiber.Ctx) bool { return true }
	Probe func(fiber.Ctx) bool
}

const (
	// LivenessEndpoint is the conventional path for a liveness check.
	// Register the middleware on this path to expose it.
	LivenessEndpoint = "/livez"

	// ReadinessEndpoint is the conventional path for a readiness check.
	// Register the middleware on this path to expose it.
	ReadinessEndpoint = "/readyz"

	// StartupEndpoint is the conventional path for a startup check.
	// Register the middleware on this path to expose it.
	StartupEndpoint = "/startupz"
)

func defaultProbe(_ fiber.Ctx) bool { return true }

// ConfigDefault is the default configuration.
var ConfigDefault = Config{
	Next:  nil,
	Probe: defaultProbe,
}

func configDefault(config ...Config) Config {
	if len(config) < 1 {
		return ConfigDefault
	}

	cfg := config[0]

	if cfg.Probe == nil {
		cfg.Probe = ConfigDefault.Probe
	}

	return cfg
}
