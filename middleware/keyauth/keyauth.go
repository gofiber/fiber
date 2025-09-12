package keyauth

import (
	"errors"
	"fmt"
	"strings"

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
var ErrMissingOrMalformedAPIKey = errors.New("missing or invalid API Key")

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Init config
	cfg := configDefault(config...)

	// Determine the auth schemes from the extractor chain.
	authSchemes := getAuthSchemes(cfg.Extractor)

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

		// Execute the error handler first
		handlerErr := cfg.ErrorHandler(c, err)

		status := c.Response().StatusCode()
		if status == fiber.StatusUnauthorized || status == fiber.StatusProxyAuthRequired {
			header := fiber.HeaderWWWAuthenticate
			if status == fiber.StatusProxyAuthRequired {
				header = fiber.HeaderProxyAuthenticate
			}
			if len(authSchemes) > 0 {
				challenges := make([]string, 0, len(authSchemes))
				for _, scheme := range authSchemes {
					challenge := fmt.Sprintf("%s realm=%q", scheme, cfg.Realm)
					if strings.EqualFold(scheme, "Bearer") {
						if cfg.Error != "" {
							challenge += fmt.Sprintf(", error=%q", cfg.Error)
							if cfg.ErrorDescription != "" {
								challenge += fmt.Sprintf(", error_description=%q", cfg.ErrorDescription)
							}
							if cfg.ErrorURI != "" {
								challenge += fmt.Sprintf(", error_uri=%q", cfg.ErrorURI)
							}
							if cfg.Error == "insufficient_scope" {
								challenge += fmt.Sprintf(", scope=%q", cfg.Scope)
							}
						}
					}
					challenges = append(challenges, challenge)
				}
				c.Set(header, strings.Join(challenges, ", "))
			} else if cfg.Challenge != "" {
				c.Set(header, cfg.Challenge)
			}
		}

		return handlerErr
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

// getAuthSchemes inspects an extractor and its chain to find all auth schemes
// used by FromAuthHeader. It returns a slice of schemes, or an empty slice if
// none are found.
func getAuthSchemes(e Extractor) []string {
	var schemes []string
	if e.Source == SourceAuthHeader && e.AuthScheme != "" {
		schemes = append(schemes, e.AuthScheme)
	}
	for _, ex := range e.Chain {
		schemes = append(schemes, getAuthSchemes(ex)...)
	}
	return schemes
}
