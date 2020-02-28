package middleware

import "github.com/gofiber/fiber"

// BasicAuthConfig ...
type BasicAuthConfig struct {
}

// BasicAuth ...
func BasicAuth(config ...BasicAuthConfig) func(*fiber.Ctx) {
	// Init config
	var cfg BasicAuthConfig
	// Set config if provided
	if len(config) > 0 {
		cfg = config[0]
	}
	// Don't forget to remove this
	_ = cfg
	// Set config default values
	return func(c *fiber.Ctx) {
		c.Next()
	}
}
