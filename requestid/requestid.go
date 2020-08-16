package requestid

import (
	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
)

// Config defines the config for requestid middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	Next func(c *fiber.Ctx) bool

	// Header is the header key where to get/set the unique request ID
	// Optional. Default: X-Request-ID
	Header string

	// Generator defines a function to generate the unique identifier.
	// Optional. Default: func() string {
	//   return utils.UUID()
	// }
	Generator func() string
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:   nil,
	Header: fiber.HeaderXRequestID,
	Generator: func() string {
		return utils.UUID()
	},
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		if config[0].Next == nil {
			config[0].Next = ConfigDefault.Next
		}
		if config[0].Header == "" {
			config[0].Header = ConfigDefault.Header
		}
		if config[0].Generator == nil {
			config[0].Generator = ConfigDefault.Generator
		}
		cfg = config[0]
	}

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Don't execute the middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}
		// Get id from request
		rid := c.Get(cfg.Header)

		// Create new UUID if empty
		if len(rid) <= 0 {
			rid = cfg.Generator()
		}

		// Set new id to response header
		c.Set(cfg.Header, rid)

		// Continue stack
		return c.Next()
	}
}
