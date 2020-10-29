package requestid

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Header is the header key where to get/set the unique request ID
	//
	// Optional. Default: "X-Request-ID"
	Header string

	// Generator defines a function to generate the unique identifier.
	//
	// Optional. Default: utils.UUID
	Generator func() string

	// ContextKey defines the key used when storing the request ID in
	// the locals for a specific request.
	//
	// Optional. Default: requestid
	ContextKey string
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:       nil,
	Header:     fiber.HeaderXRequestID,
	Generator:  utils.UUID,
	ContextKey: "requestid",
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]

		// Set default values
		if cfg.Header == "" {
			cfg.Header = ConfigDefault.Header
		}
		if cfg.Generator == nil {
			cfg.Generator = ConfigDefault.Generator
		}
		if cfg.ContextKey == "" {
			cfg.ContextKey = ConfigDefault.ContextKey
		}
	}

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}
		// Get id from request, else we generate one
		rid := c.Get(cfg.Header, cfg.Generator())

		// Set new id to response header
		c.Set(cfg.Header, rid)

		// Add the request ID to locals
		c.Locals(cfg.ContextKey, rid)

		// Continue stack
		return c.Next()
	}
}
