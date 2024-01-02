package requestid

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// Header is the header key where to get/set the unique request ID
	//
	// Optional. Default: "X-Request-ID"
	Header string

	// Generator defines a function to generate the unique identifier.
	//
	// Optional. Default: utils.UUID
	Generator func() string
}

// ConfigDefault is the default config
// It uses a fast UUID generator which will expose the number of
// requests made to the server. To conceal this value for better
// privacy, use the "utils.UUIDv4" generator.
var ConfigDefault = Config{
	Next:      nil,
	Header:    fiber.HeaderXRequestID,
	Generator: utils.UUID,
}

// Helper function to set default values
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
	if cfg.Generator == nil {
		cfg.Generator = ConfigDefault.Generator
	}
	return cfg
}
