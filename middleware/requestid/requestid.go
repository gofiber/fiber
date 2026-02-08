package requestid

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
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
		rid := sanitizeRequestID(c.Get(cfg.Header), cfg.Generator)

		// Set new id to response header
		c.Set(cfg.Header, rid)

		// Add the request ID to locals
		c.Locals(requestIDKey, rid)

		// Also store the request ID in the user context so it can be
		// retrieved from any context.Context (e.g. c.Context() or
		// c.RequestCtx()) further down the call stack.
		ctx := context.WithValue(c.Context(), requestIDKey, rid)
		c.SetContext(ctx)

		// Continue stack
		return c.Next()
	}
}

// sanitizeRequestID returns the provided request ID when it is valid, otherwise
// it tries up to three values from the configured generator, then falls back to SecureToken.
func sanitizeRequestID(rid string, generator func() string) string {
	if isValidRequestID(rid) {
		return rid
	}

	for range 3 {
		rid = generator()
		if isValidRequestID(rid) {
			return rid
		}
	}

	// Final fallback: SecureToken always produces a valid ID
	return utils.SecureToken()
}

// isValidRequestID reports whether the request ID contains only visible ASCII
// characters (0x20–0x7E) and is non-empty.
func isValidRequestID(rid string) bool {
	if rid == "" {
		return false
	}

	for i := 0; i < len(rid); i++ {
		c := rid[i]
		if c < 0x20 || c > 0x7e {
			return false
		}
	}

	return true
}

// FromContext returns the request ID from context.
// If there is no request ID, an empty string is returned.
func FromContext(c fiber.Ctx) string {
	if rid, ok := c.Locals(requestIDKey).(string); ok {
		return rid
	}
	return ""
}

// FromStdContext returns the request ID from a standard context.Context.
// This is useful when the request ID needs to be retrieved outside of a
// Fiber handler, for example in service layers that only receive a
// context.Context (obtained via c.Context() or c.RequestCtx()).
// If there is no request ID, an empty string is returned.
func FromStdContext(ctx context.Context) string {
	if rid, ok := ctx.Value(requestIDKey).(string); ok {
		return rid
	}
	return ""
}
