package middleware

import "github.com/gofiber/fiber"

// LoggerConfig ...
type LoggerConfig struct {
}

// Logger ...
func Logger(config ...LoggerConfig) func(*fiber.Ctx) {
	// Init config
	var cfg LoggerConfig
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
