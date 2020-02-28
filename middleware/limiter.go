package middleware

import "github.com/gofiber/fiber"

// LimiterConfig ...
type LimiterConfig struct {
}

// Limiter ...
func Limiter(config ...LimiterConfig) func(*fiber.Ctx) {
	// Init config
	var cfg LimiterConfig
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
