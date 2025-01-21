// Special thanks to Echo: https://github.com/labstack/echo/blob/master/middleware/key_auth.go
package keyauth

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// The contextKey type is unexported to prevent collisions with context keys defined in
// other packages.
type contextKey int

// The keys for the values in context
const (
	tokenKey contextKey = 0
)

// When there is no request of the key thrown ErrMissingOrMalformedAPIKey
var ErrMissingOrMalformedAPIKey = errors.New("missing or malformed API Key")

const (
	query  = "query"
	form   = "form"
	param  = "param"
	cookie = "cookie"
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Init config
	cfg := configDefault(config...)

	// Initialize
	if cfg.CustomKeyLookup == nil {
		var err error
		cfg.CustomKeyLookup, err = DefaultKeyLookup(cfg.KeyLookup, cfg.AuthScheme)
		if err != nil {
			panic(fmt.Errorf("unable to create lookup function: %w", err))
		}
	}

	// Return middleware handler
	return func(c fiber.Ctx) error {
		// Filter request to skip middleware
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Extract and verify key
		key, err := cfg.CustomKeyLookup(c)
		if err != nil {
			return cfg.ErrorHandler(c, err)
		}

		valid, err := cfg.Validator(c, key)

		if err == nil && valid {
			// Store in both Locals and Context
			c.Locals(tokenKey, key)
			ctx := context.WithValue(c.Context(), tokenKey, key)
			c.SetContext(ctx)
			return cfg.SuccessHandler(c)
		}
		return cfg.ErrorHandler(c, err)
	}
}

// TokenFromContext returns the bearer token from the request context.
// returns an empty string if the token does not exist
func TokenFromContext(c any) string {
	switch ctx := c.(type) {
	case context.Context:
		if token, ok := ctx.Value(tokenKey).(string); ok {
			return token
		}
	case fiber.Ctx:
		if token, ok := ctx.Locals(tokenKey).(string); ok {
			return token
		}
	default:
		log.Errorf("Unsupported context type: %T. Expected fiber.Ctx or context.Context", c)
	}
	return ""
}

// MultipleKeySourceLookup creates a CustomKeyLookup function that checks multiple sources until one is found
// Each element should be specified according to the format used in KeyLookup
func MultipleKeySourceLookup(keyLookups []string, authScheme string) (KeyLookupFunc, error) {
	subExtractors := map[string]KeyLookupFunc{}
	var err error
	for _, keyLookup := range keyLookups {
		subExtractors[keyLookup], err = DefaultKeyLookup(keyLookup, authScheme)
		if err != nil {
			return nil, err
		}
	}
	return func(c fiber.Ctx) (string, error) {
		for keyLookup, subExtractor := range subExtractors {
			res, err := subExtractor(c)
			if err == nil && res != "" {
				return res, nil
			}
			if !errors.Is(err, ErrMissingOrMalformedAPIKey) {
				// Defensive Code - not currently possible to hit
				return "", fmt.Errorf("[%s] %w", keyLookup, err)
			}
		}
		return "", ErrMissingOrMalformedAPIKey
	}, nil
}

func DefaultKeyLookup(keyLookup, authScheme string) (KeyLookupFunc, error) {
	parts := strings.Split(keyLookup, ":")
	if len(parts) <= 1 {
		return nil, fmt.Errorf("invalid keyLookup: %q, expected format 'source:name'", keyLookup)
	}
	extractor := KeyFromHeader(parts[1], authScheme) // in the event of an invalid prefix, it is interpreted as header:
	switch parts[0] {
	case query:
		extractor = KeyFromQuery(parts[1])
	case form:
		extractor = KeyFromForm(parts[1])
	case param:
		extractor = KeyFromParam(parts[1])
	case cookie:
		extractor = KeyFromCookie(parts[1])
	}
	return extractor, nil
}

// keyFromHeader returns a function that extracts api key from the request header.
func KeyFromHeader(header, authScheme string) KeyLookupFunc {
	return func(c fiber.Ctx) (string, error) {
		auth := c.Get(header)
		l := len(authScheme)
		if len(auth) > 0 && l == 0 {
			return auth, nil
		}
		if len(auth) > l+1 && auth[:l] == authScheme {
			return auth[l+1:], nil
		}
		return "", ErrMissingOrMalformedAPIKey
	}
}

// keyFromQuery returns a function that extracts api key from the query string.
func KeyFromQuery(param string) KeyLookupFunc {
	return func(c fiber.Ctx) (string, error) {
		key := fiber.Query[string](c, param)
		if key == "" {
			return "", ErrMissingOrMalformedAPIKey
		}
		return key, nil
	}
}

// keyFromForm returns a function that extracts api key from the form.
func KeyFromForm(param string) KeyLookupFunc {
	return func(c fiber.Ctx) (string, error) {
		key := c.FormValue(param)
		if key == "" {
			return "", ErrMissingOrMalformedAPIKey
		}
		return key, nil
	}
}

// keyFromParam returns a function that extracts api key from the url param string.
func KeyFromParam(param string) KeyLookupFunc {
	return func(c fiber.Ctx) (string, error) {
		key, err := url.PathUnescape(c.Params(param))
		if err != nil {
			return "", ErrMissingOrMalformedAPIKey
		}
		return key, nil
	}
}

// keyFromCookie returns a function that extracts api key from the named cookie.
func KeyFromCookie(name string) KeyLookupFunc {
	return func(c fiber.Ctx) (string, error) {
		key := c.Cookies(name)
		if key == "" {
			return "", ErrMissingOrMalformedAPIKey
		}
		return key, nil
	}
}
