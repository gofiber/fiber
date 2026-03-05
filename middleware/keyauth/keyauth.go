package keyauth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
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

// registerExtractor ensures the log context extractor for API keys is
// registered exactly once, regardless of how many times New() is called.
var registerExtractor sync.Once

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Init config
	cfg := configDefault(config...)

	// Register a log context extractor so that log.WithContext(c) automatically
	// includes a redacted API key when the keyauth middleware is in use.
	// An empty token (no middleware or middleware skipped) is omitted.
	registerExtractor.Do(func() {
		log.RegisterContextExtractor(func(ctx context.Context) (string, any, bool) {
			token := TokenFromContext(ctx)
			if token == "" {
				return "", nil, false
			}
			return "api-key", redactValue(token), true
		})
	})

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
		if errors.Is(err, extractors.ErrNotFound) {
			// Replace shared extractor not found error with a keyauth specific error
			err = ErrMissingOrMalformedAPIKey
		}
		// If there was no error extracting the key, validate it
		if err == nil {
			var valid bool
			valid, err = cfg.Validator(c, key)
			if err == nil && valid {
				fiber.StoreInContext(c, tokenKey, key)
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
					var b strings.Builder
					fmt.Fprintf(&b, "%s realm=%q", scheme, cfg.Realm)
					if utils.EqualFold(scheme, "Bearer") {
						if cfg.Error != "" {
							fmt.Fprintf(&b, ", error=%q", cfg.Error)
							if cfg.ErrorDescription != "" {
								fmt.Fprintf(&b, ", error_description=%q", cfg.ErrorDescription)
							}
							if cfg.ErrorURI != "" {
								fmt.Fprintf(&b, ", error_uri=%q", cfg.ErrorURI)
							}
							if cfg.Error == ErrorInsufficientScope {
								fmt.Fprintf(&b, ", scope=%q", cfg.Scope)
							}
						}
					}
					challenges = append(challenges, b.String())
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
// It accepts fiber.CustomCtx, fiber.Ctx, *fasthttp.RequestCtx, and context.Context.
// It returns an empty string if the token does not exist.
func TokenFromContext(ctx any) string {
	if token, ok := fiber.ValueFromContext[string](ctx, tokenKey); ok {
		return token
	}

	return ""
}

// getAuthSchemes inspects an extractor and its chain to find all auth schemes
// used by FromAuthHeader. It returns a slice of schemes, or an empty slice if
// none are found.
func getAuthSchemes(e extractors.Extractor) []string {
	var schemes []string
	if e.Source == extractors.SourceAuthHeader && e.AuthScheme != "" {
		schemes = append(schemes, e.AuthScheme)
	}
	for _, ex := range e.Chain {
		schemes = append(schemes, getAuthSchemes(ex)...)
	}
	return schemes
}

// redactValue returns a masked version of a sensitive value for safe logging.
// It shows the first 4 characters followed by "****" for values longer than
// 8 characters, or "****" for shorter values.
func redactValue(s string) string {
	if len(s) > 8 {
		return s[:4] + "****"
	}
	return "****"
}
