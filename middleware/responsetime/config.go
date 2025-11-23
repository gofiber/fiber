package responsetime

import (
	"github.com/gofiber/fiber/v3"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// Header is the header key used to set the response time.
	//
	// Optional. Default: "X-Response-Time"
	Header string
}

// ConfigDefault is the default config.
var ConfigDefault = Config{
	Next:   nil,
	Header: fiber.HeaderXResponseTime,
}

// Helper function to set default values.
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.Header == "" {
		cfg.Header = ConfigDefault.Header
	}

	return cfg
}
