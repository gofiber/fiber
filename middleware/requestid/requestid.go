package requestid

import (
	"context"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
)

// The contextKey type is unexported to prevent collisions with context keys defined in
// other packages.
type contextKey int

// The keys for the values in context
const (
	requestIDKey contextKey = iota
)

// registerExtractor ensures the log context extractor for request IDs is
// registered exactly once, regardless of how many times New() is called.
var registerExtractor sync.Once

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Register a log context extractor so that log.WithContext(c) automatically
	// includes the request ID when the requestid middleware is in use.
	// An empty request ID (no middleware or middleware skipped) is omitted.
	registerExtractor.Do(func() {
		log.RegisterContextExtractor(func(ctx context.Context) (string, any, bool) {
			rid := FromContext(ctx)
			return "request-id", rid, rid != ""
		})
	})

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
		fiber.StoreInContext(c, requestIDKey, rid)

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
// It accepts fiber.CustomCtx, fiber.Ctx, *fasthttp.RequestCtx, and context.Context.
// If there is no request ID, an empty string is returned.
func FromContext(ctx any) string {
	if rid, ok := fiber.ValueFromContext[string](ctx, requestIDKey); ok {
		return rid
	}

	return ""
}
