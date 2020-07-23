package middleware

import (
	fiber "github.com/gofiber/fiber"
	utils "github.com/gofiber/utils"
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
/*
RequestID adds an UUID indentifier to the request, the following config arguments in any order:
	- RequestID()
	- RequestID(next func(*fiber.Ctx) bool)
	- RequestID(header string)
	- RequestID(generator func() string)
	- RequestID(config RequestIDConfig)
*/
func RequestID(options ...interface{}) fiber.Handler {
	// Create default config
	var config = RequestIDConfigDefault
	// Assert options if provided to adjust the config
	if len(options) > 0 {
		for i := range options {
			switch opt := options[i].(type) {
			case func(*fiber.Ctx) bool:
				config.Next = opt
			case string:
				config.Header = opt
			case func() string:
				config.Generator = opt
			case RequestIDConfig:
				config = opt
			default:
				panic("RequestID: the following option types are allowed: `string`, `func() string`, `func(*fiber.Ctx) bool`, `RequestIDConfig`")
			}
		}
	}
	// Return requestID
	return requestID(config)
}

func requestID(config RequestIDConfig) fiber.Handler {
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
			rid = config.Generator()
		}
		// Set new id to response header
		ctx.Set(config.Header, rid)
		// Continue stack
		ctx.Next()
	}
}
