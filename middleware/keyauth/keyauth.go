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

type extractorFunc func(c fiber.Ctx) (string, error)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Init config
	cfg := configDefault(config...)

	// Initialize

	var extractor extractorFunc
	extractor, err := parseSingleExtractor(cfg.KeyLookup, cfg.AuthScheme)
	if err != nil {
		panic(fmt.Errorf("error creating middleware: invalid keyauth Config.KeyLookup: %w", err))
	}
	if len(cfg.FallbackKeyLookups) > 0 {
		subExtractors := map[string]extractorFunc{cfg.KeyLookup: extractor}
		for _, keyLookup := range cfg.FallbackKeyLookups {
			subExtractors[keyLookup], err = parseSingleExtractor(keyLookup, cfg.AuthScheme)
			if err != nil {
				panic(fmt.Errorf("error creating middleware: invalid keyauth Config.FallbackKeyLookups[%s]: %w", keyLookup, err))
			}
		}
		extractor = func(c fiber.Ctx) (string, error) {
			for keyLookup, subExtractor := range subExtractors {
				res, err := subExtractor(c)
				if err == nil && res != "" {
					return res, nil
				}
				if !errors.Is(err, ErrMissingOrMalformedAPIKey) {
					return "", fmt.Errorf("[%s] %w", keyLookup, err)
				}
			}
			return "", ErrMissingOrMalformedAPIKey
		}
	}

	// Return middleware handler
	return func(c fiber.Ctx) error {
		// Filter request to skip middleware
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Extract and verify key
		key, err := extractor(c)
		if err != nil {
			return cfg.ErrorHandler(c, err)
		}

		valid, err := cfg.Validator(c, key)

		if err == nil && valid {
			c.Locals(tokenKey, key)
			return cfg.SuccessHandler(c)
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

func parseSingleExtractor(keyLookup string, authScheme string) (extractorFunc, error) {
	parts := strings.Split(keyLookup, ":")
	if len(parts) <= 1 {
		return nil, fmt.Errorf("invalid keyLookup: %s", keyLookup)
	}
	extractor := keyFromHeader(parts[1], authScheme) // in the event of an invalid prefix, it is interpreted as header:
	switch parts[0] {
	case query:
		extractor = keyFromQuery(parts[1])
	case form:
		extractor = keyFromForm(parts[1])
	case param:
		extractor = keyFromParam(parts[1])
	case cookie:
		extractor = keyFromCookie(parts[1])
	}
	return extractor, nil
}

// keyFromHeader returns a function that extracts api key from the request header.
func keyFromHeader(header, authScheme string) func(c fiber.Ctx) (string, error) {
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
func keyFromQuery(param string) func(c fiber.Ctx) (string, error) {
	return func(c fiber.Ctx) (string, error) {
		key := fiber.Query[string](c, param)
		if key == "" {
			return "", ErrMissingOrMalformedAPIKey
		}
		return key, nil
	}
}

// keyFromForm returns a function that extracts api key from the form.
func keyFromForm(param string) func(c fiber.Ctx) (string, error) {
	return func(c fiber.Ctx) (string, error) {
		key := c.FormValue(param)
		if key == "" {
			return "", ErrMissingOrMalformedAPIKey
		}
		return key, nil
	}
}

// keyFromParam returns a function that extracts api key from the url param string.
func keyFromParam(param string) func(c fiber.Ctx) (string, error) {
	return func(c fiber.Ctx) (string, error) {
		key, err := url.PathUnescape(c.Params(param))
		if err != nil {
			return "", ErrMissingOrMalformedAPIKey
		}
		return key, nil
	}
}

// keyFromCookie returns a function that extracts api key from the named cookie.
func keyFromCookie(name string) func(c fiber.Ctx) (string, error) {
	return func(c fiber.Ctx) (string, error) {
		key := c.Cookies(name)
		if key == "" {
			return "", ErrMissingOrMalformedAPIKey
		}
		return key, nil
	}
}
