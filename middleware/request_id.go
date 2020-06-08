package middleware

import (
	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
)

// Middleware types
type (
	// RequestIDConfig defines the config for Logger middleware.
	RequestIDConfig struct {
		// Next defines a function to skip this middleware.
		Next func(ctx *fiber.Ctx) bool

		// Header is the header key where to get/set the unique ID
		// Optional. Default: X-Request-ID
		Header string

		// Generator defines a function to generate the unique identifier.
		// Optional. Default: func() string {
		//   return utils.UUID()
		// }
		Generator func() string
	}
)

// RequestIDConfigDefault is the default config
var RequestIDConfigDefault = RequestIDConfig{
	Next:   nil,
	Header: fiber.HeaderXRequestID,
	Generator: func() string {
		return utils.UUID()
	},
}

// RequestID adds an UUID indentifier to the request
func RequestID(header ...string) fiber.Handler {
	// Create default config
	var config = RequestIDConfigDefault
	// Set lookup if provided
	if len(header) > 0 {
		config.Header = header[0]
	}
	// Return LoggerWithConfig
	return RequestIDWithConfig(config)
}

// RequestIDWithConfig allows you to pass a custom config
func RequestIDWithConfig(config RequestIDConfig) fiber.Handler {
	// Set default values
	if config.Header == "" {
		config.Header = RequestIDConfigDefault.Header
	}
	if config.Generator == nil {
		config.Generator = RequestIDConfigDefault.Generator
	}

	// Return handler
	return func(ctx *fiber.Ctx) {
		// Don't execute the middleware if Next returns true
		if config.Next != nil && config.Next(ctx) {
			ctx.Next()
			return
		}
		// Get id from request
		rid := ctx.Get(config.Header)
		// Create new UUID if empty
		if len(rid) <= 0 {
			rid = utils.UUID()
		}
		// Set new id to response header
		ctx.Set(fiber.HeaderXRequestID, rid)
		// Continue stack
		ctx.Next()
	}
}
