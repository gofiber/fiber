package limiter

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

const (
	// X-RateLimit-* headers
	xRateLimitLimit     = "X-RateLimit-Limit"
	xRateLimitRemaining = "X-RateLimit-Remaining"
	xRateLimitReset     = "X-RateLimit-Reset"
)

// Handler defines a rate-limiting strategy that can produce a middleware
// handler using the provided configuration.
type Handler interface {
	New(config *Config) fiber.Handler
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Return the specified middleware handler.
	return cfg.LimiterMiddleware.New(&cfg)
}

// getEffectiveStatusCode returns the actual status code, considering both the error and response status
func getEffectiveStatusCode(c fiber.Ctx, err error) int {
	// If there's an error and it's a *fiber.Error, use its status code
	if err != nil {
		var fiberErr *fiber.Error
		if errors.As(err, &fiberErr) {
			return fiberErr.Code
		}
	}

	// Otherwise, use the response status code
	return c.Response().StatusCode()
}
