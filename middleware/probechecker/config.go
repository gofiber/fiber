package probechecker

import "github.com/gofiber/fiber/v2"

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

func defaultLiveFunc(*fiber.Ctx) bool { return true }

// ConfigDefault is the default config
var ConfigDefault = Config{
	IsLive:          defaultLiveFunc,
	IsLiveEndpoint:  DefaultLivenessEndpoint,
	IsReadyEndpoint: DefaultReadinessEndpoint,
}

// defaultConfig returns a default config for the probechecker middleware.
func defaultConfig(config ...Config) Config {
	if len(config) < 1 {
		return ConfigDefault
	}

	cfg := config[0]

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
