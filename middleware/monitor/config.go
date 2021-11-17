package monitor

import "github.com/gofiber/fiber/v2"

// Config defines the config for middleware.
type Config struct {
	// Whether the service should expose only the monitoring API.
	//
	// Optional. Default: false
	APIOnly bool

	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool
}

var ConfigDefault = Config{
	APIOnly: false,
	Next:    nil,
}

func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}

	if !cfg.APIOnly {
		cfg.APIOnly = ConfigDefault.APIOnly
	}

	return cfg
}
