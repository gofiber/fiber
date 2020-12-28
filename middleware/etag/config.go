package etag

import (
	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Weak indicates that a weak validator is used. Weak etags are easy
	// to generate, but are far less useful for comparisons. Strong
	// validators are ideal for comparisons but can be very difficult
	// to generate efficiently. Weak ETag values of two representations
	// of the same resources might be semantically equivalent, but not
	// byte-for-byte identical. This means weak etags prevent caching
	// when byte range requests are used, but strong etags mean range
	// requests can still be cached.
	Weak bool

	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Weak: false,
	Next: nil,
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

	return cfg
}
