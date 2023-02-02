package pprof

import (
	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Prefix defines a URL prefix added before "/debug/pprof".
	// Note that it should start with (but not end with) a slash.
	// Example: "/federated-fiber"
	//
	// Optional. Default: ""
	Prefix string
}

var ConfigDefault = Config{
	Next: nil,
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

	return cfg
}
