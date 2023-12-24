package healthcheck

import (
	"github.com/gofiber/fiber/v2"
)

// Config is the config struct for the healthcheck middleware
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Config for liveness probe of the container engine being used
	//
	// Optional. Default: func(c *Ctx) bool { return true }
	IsLive HealthChecker

	// HTTP endpoint of the liveness probe
	//
	// Optional. Default: /livez
	IsLiveEndpoint string

	// Config for readiness probe of the container engine being used
	//
	// Optional. Default: nil
	IsReady HealthChecker

	// HTTP endpoint of the readiness probe
	//
	// Optional. Default: /readyz
	IsReadyEndpoint string
}

const (
	DefaultLivenessEndpoint  = "/livez"
	DefaultReadinessEndpoint = "/readyz"
)

func defaultLiveFunc(*fiber.Ctx) bool { return true }

// ConfigDefault is the default config
var ConfigDefault = Config{
	IsLive:          defaultLiveFunc,
	IsLiveEndpoint:  DefaultLivenessEndpoint,
	IsReadyEndpoint: DefaultReadinessEndpoint,
}

// defaultConfig returns a default config for the healthcheck middleware.
func defaultConfig(config ...Config) Config {
	if len(config) < 1 {
		return ConfigDefault
	}

	cfg := config[0]

	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}

	if cfg.IsLive == nil {
		cfg.IsLive = defaultLiveFunc
	}

	if cfg.IsLiveEndpoint == "" {
		cfg.IsLiveEndpoint = DefaultLivenessEndpoint
	}

	if cfg.IsReadyEndpoint == "" {
		cfg.IsReadyEndpoint = DefaultReadinessEndpoint
	}

	return cfg
}
