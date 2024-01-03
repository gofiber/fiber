package healthcheck

import (
	"github.com/gofiber/fiber/v2"
)

// Config defines the configuration options for the healthcheck middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Function used for checking the liveness of the application. Returns true if the application
	// is running and false if it is not. The liveness probe is typically used to indicate if
	// the application is in a state where it can handle requests (e.g., the server is up and running).
	//
	// Optional. Default: func(c *fiber.Ctx) bool { return true }
	LivenessProbe HealthChecker

	// HTTP endpoint at which the liveness probe will be available.
	//
	// Optional. Default: "/livez"
	LivenessEndpoint string

	// Function used for checking the readiness of the application. Returns true if the application
	// is ready to process requests and false otherwise. The readiness probe typically checks if all necessary
	// services, databases, and other dependencies are available for the application to function correctly.
	//
	// Optional. Default: func(c *fiber.Ctx) bool { return true }
	ReadinessProbe HealthChecker

	// HTTP endpoint at which the readiness probe will be available.
	// Optional. Default: "/readyz"
	ReadinessEndpoint string
}

const (
	DefaultLivenessEndpoint  = "/livez"
	DefaultReadinessEndpoint = "/readyz"
)

func defaultLivenessProbe(*fiber.Ctx) bool { return true }

func defaultReadinessProbe(*fiber.Ctx) bool { return true }

// ConfigDefault is the default config
var ConfigDefault = Config{
	LivenessProbe:     defaultLivenessProbe,
	ReadinessProbe:    defaultReadinessProbe,
	LivenessEndpoint:  DefaultLivenessEndpoint,
	ReadinessEndpoint: DefaultReadinessEndpoint,
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

	if cfg.LivenessProbe == nil {
		cfg.LivenessProbe = defaultLivenessProbe
	}

	if cfg.ReadinessProbe == nil {
		cfg.ReadinessProbe = defaultReadinessProbe
	}

	if cfg.LivenessEndpoint == "" {
		cfg.LivenessEndpoint = DefaultLivenessEndpoint
	}

	if cfg.ReadinessEndpoint == "" {
		cfg.ReadinessEndpoint = DefaultReadinessEndpoint
	}

	return cfg
}
