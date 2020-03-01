package middleware

import (
	"fmt"

	"github.com/gofiber/fiber"
	"github.com/google/uuid"
)

// RequestIDConfig defines the config for RequestID middleware
type RequestIDConfig struct {
	// Skip defines a function to skip middleware.
	// Optional. Default: nil
	Skip func(*fiber.Ctx) bool
	// Generator defines a function to generate an ID.
	// Optional. Default: func() string {
	//   return uuid.New().String()
	// }
	Generator func() string
}

// RequestIDConfigDefault is the default RequestID middleware config.
var RequestIDConfigDefault = RequestIDConfig{
	Skip: nil,
	Generator: func() string {
		return uuid.New().String()
	},
}

// RequestID adds an indentifier to the request using the `X-Request-ID` header
func RequestID(config ...RequestIDConfig) func(*fiber.Ctx) {
	// Init config
	var cfg RequestIDConfig
	if len(config) > 0 {
		cfg = config[0]
	}
	// Set config default values
	if cfg.Generator == nil {
		cfg.Skip = RequestIDConfigDefault.Skip
	}
	if cfg.Generator == nil {
		cfg.Generator = RequestIDConfigDefault.Generator
	}
	// Return middleware handler
	return func(c *fiber.Ctx) {
		// Skip middleware if Skip returns true
		if cfg.Skip != nil && cfg.Skip(c) {
			c.Next()
			return
		}
		// Get value from RequestID
		rid := c.Get(fiber.HeaderXRequestID)
		fmt.Println(rid)
		// Create new ID
		if rid == "" {
			rid = cfg.Generator()
		}
		// Set X-Request-ID
		c.Set(fiber.HeaderXRequestID, rid)
		// Bye!
		c.Next()
	}
}
