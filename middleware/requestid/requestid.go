package requestid

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// The contextKey type is unexported to prevent collisions with context keys defined in
// other packages.
type contextKey int

// The keys for the values in context
const (
	requestIDKey contextKey = iota
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}
		// Get id from request, else we generate one
		rid := c.Get(cfg.Header)
		if rid == "" {
			rid = cfg.Generator()
		}

		// Set new id to response header
		c.Set(cfg.Header, rid)

		// Add the request ID to locals
		c.Locals(requestIDKey, rid)

		// Add the request ID to UserContext
		ctx := context.WithValue(c.Context(), requestIDKey, rid)
		c.SetContext(ctx)

		// Continue stack
		return c.Next()
	}
}

// FromContext returns the request ID from context.
// If there is no request ID, an empty string is returned.
// Supported context types:
// - fiber.Ctx: Retrieves request ID from Locals
// - context.Context: Retrieves request ID from context values
func FromContext(c any) string {
	switch ctx := c.(type) {
	case fiber.Ctx:
		if rid, ok := ctx.Locals(requestIDKey).(string); ok {
			return rid
		}
	case context.Context:
		if rid, ok := ctx.Value(requestIDKey).(string); ok {
			return rid
		}
	default:
		log.Errorf("Unsupported context type: %T. Expected fiber.Ctx or context.Context", c)
	}
	return ""
}
