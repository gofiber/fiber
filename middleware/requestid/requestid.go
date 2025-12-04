package requestid

import (
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
		if rid == "" {
			rid = utils.UUID()
		}

		// Set new id to response header
		c.Set(cfg.Header, rid)

		// Add the request ID to locals
		c.Locals(requestIDKey, rid)

		// Continue stack
		return c.Next()
	}
}

// sanitizeRequestID returns the provided request ID when it is valid, otherwise
// it tries up to three values from the configured generator, then three UUIDs,
// falling back to an empty string when no visible ASCII ID is produced.
func sanitizeRequestID(rid string, generator func() string) string {
	if isValidRequestID(rid) {
		return rid
	}

	generatorFn := generator
	if generatorFn == nil {
		generatorFn = utils.UUID
	}

	for range 3 {
		rid = generatorFn()
		if isValidRequestID(rid) {
			return rid
		}
	}

	if generator != nil {
		for range 3 {
			rid = utils.UUID()
			if isValidRequestID(rid) {
				return rid
			}
		}
	}

	return ""
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
