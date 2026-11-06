package keyauth

import (
	"errors"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/quotedstring"
	"github.com/gofiber/fiber/v3/internal/redact"
	"github.com/gofiber/fiber/v3/middleware/logger"
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

var registerLogContextTagsOnce sync.Once

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	registerLogContextTagsOnce.Do(registerLogContextTags)

	// Init config
	cfg := configDefault(config...)

	// Determine the auth schemes from the extractor chain.
	authSchemes := getAuthSchemes(cfg.Extractor)

	// The challenge values only depend on config, so build them once instead
	// of re-formatting them on every 401/407 response. RFC 6750 section 3
	// says a challenge for a request that carried no credentials should not
	// include the error parameters, so that case gets a challenge without them.
	challengeValue := buildChallenge(&cfg, authSchemes, bearerErrorParams(&cfg))
	missingCredentialsChallenge := buildChallenge(&cfg, authSchemes, "")

	// Return middleware handler
	return func(c fiber.Ctx) error {
		// Filter request to skip middleware
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Extract and verify key
		key, err := cfg.Extractor.Extract(c)
		missingCredentials := errors.Is(err, extractors.ErrNotFound)
		if missingCredentials {
			// Replace shared extractor not found error with a keyauth specific error
			err = ErrMissingOrMalformedAPIKey
			// The extractor reports a malformed credential the same way. A Bearer one is
			// still an attempt, and only the Bearer challenge carries the error parameters.
			missingCredentials = !usesBearerScheme(c.Get(fiber.HeaderAuthorization))
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

		status := c.Res().StatusCode()
		// The ErrorHandler may return the error for the app to write; honor its status code too.
		var fiberErr *fiber.Error
		if errors.As(handlerErr, &fiberErr) && fiberErr != nil {
			status = fiberErr.Code
		}
		if status == fiber.StatusUnauthorized || status == fiber.StatusProxyAuthRequired {
			header := fiber.HeaderWWWAuthenticate
			if status == fiber.StatusProxyAuthRequired {
				header = fiber.HeaderProxyAuthenticate
			}
			challenge := challengeValue
			if missingCredentials {
				challenge = missingCredentialsChallenge
			}
			if challenge != "" {
				c.Set(header, challenge)
			}
		}

		return handlerErr
	}
}

// usesBearerScheme reports whether an Authorization header value names the
// Bearer scheme, whether or not the credential after it is well formed.
func usesBearerScheme(header string) bool {
	const scheme = "Bearer"
	return len(header) >= len(scheme) && utils.EqualFold(header[:len(scheme)], scheme) &&
		(len(header) == len(scheme) || header[len(scheme)] == ' ')
}

// bearerErrorParams formats the RFC 6750 error parameters configured for
// Bearer challenges, starting with a separator, or "" when none is set.
func bearerErrorParams(cfg *Config) string {
	if cfg.Error == "" {
		return ""
	}
	b := make([]byte, 0, 64)
	b = append(b, `, error="`...)
	b = append(b, quotedstring.Escape(cfg.Error)...)
	b = append(b, '"')
	if cfg.ErrorDescription != "" {
		b = append(b, `, error_description="`...)
		b = append(b, quotedstring.Escape(cfg.ErrorDescription)...)
		b = append(b, '"')
	}
	if cfg.ErrorURI != "" {
		b = append(b, `, error_uri="`...)
		b = append(b, quotedstring.Escape(cfg.ErrorURI)...)
		b = append(b, '"')
	}
	if cfg.Error == ErrorInsufficientScope {
		b = append(b, `, scope="`...)
		b = append(b, quotedstring.Escape(cfg.Scope)...)
		b = append(b, '"')
	}
	return string(b)
}

// buildChallenge formats the WWW-Authenticate value for the configured auth
// schemes, appending errorParams to the Bearer challenge.
func buildChallenge(cfg *Config, authSchemes []string, errorParams string) string {
	if len(authSchemes) == 0 {
		return cfg.Challenge
	}
	challenges := make([]string, 0, len(authSchemes))
	for _, scheme := range authSchemes {
		b := make([]byte, 0, len(scheme)+len(cfg.Realm)+len(errorParams)+16)
		b = append(b, scheme...)
		b = append(b, ` realm="`...)
		b = append(b, quotedstring.Escape(cfg.Realm)...)
		b = append(b, '"')
		if utils.EqualFold(scheme, "Bearer") {
			b = append(b, errorParams...)
		}
		challenges = append(challenges, string(b))
	}
	return strings.Join(challenges, ", ")
}

func registerLogContextTags() {
	logger.RegisterContextTag("api-key", func(ctx any) string {
		return redact.Prefix(TokenFromContext(ctx))
	})
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
	// Walked through the extractors package so the traversal, and its cycle
	// guard, live in one place.
	var schemes []string
	e.Walk(func(candidate extractors.Extractor) bool {
		if candidate.Source == extractors.SourceAuthHeader && candidate.AuthScheme != "" {
			schemes = append(schemes, candidate.AuthScheme)
		}
		return true
	})
	return schemes
}
