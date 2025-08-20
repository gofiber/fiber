package keyauth

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3"
)

// The contextKey type is unexported to prevent collisions with context keys defined in
// other packages.
type contextKey int

// The keys for the values in context
const (
	tokenKey contextKey = iota
)

// ErrMissingOrMalformedAPIKey is returned when the API key is missing or invalid.
var ErrMissingOrMalformedAPIKey = errors.New("Missing or invalid API Key")

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Init config
	cfg := configDefault(config...)

	// Determine the auth scheme from the extractor.
	authScheme := getAuthScheme(cfg.Extractor)

	// Return middleware handler
	return func(c fiber.Ctx) error {
		// Filter request to skip middleware
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Extract and verify key
		key, err := cfg.Extractor.Extract(c)
		if err == nil {
			var valid bool
			valid, err = cfg.Validator(c, key)
			if err == nil && valid {
				c.Locals(tokenKey, key)
				return cfg.SuccessHandler(c)
			}
		}

		// If we have an error, set the WWW-Authenticate header if appropriate
		if authScheme != "" {
			c.Set(fiber.HeaderWWWAuthenticate, fmt.Sprintf("%s realm=%q", authScheme, cfg.Realm))
		}

		return cfg.ErrorHandler(c, err)
	}
}

// TokenFromContext returns the bearer token from the request context.
// returns an empty string if the token does not exist
func TokenFromContext(c fiber.Ctx) string {
	token, ok := c.Locals(tokenKey).(string)
	if !ok {
		return ""
	}
	return token
}

// getAuthScheme inspects an extractor and its chain to find the auth scheme
// used by FromAuthHeader. It returns the scheme, or an empty string if not found.
func getAuthScheme(e Extractor) string {
	if e.Source == SourceAuthHeader {
		return e.AuthScheme
	}
	for _, ex := range e.Chain {
		if ex.Source == SourceAuthHeader {
			return ex.AuthScheme
		}
	}
	return ""
}
