// Special thanks to Echo: https://github.com/labstack/echo/blob/master/middleware/key_auth.go
package keyauth

import (
	"errors"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	intextractor "github.com/gofiber/fiber/v3/extractor"
	"github.com/gofiber/utils/v2"
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

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Init config
	cfg := configDefault(config...)

	if cfg.Extractor.Extract == nil {
		cfg.Extractor = FromHeader(fiber.HeaderAuthorization, cfg.AuthScheme)
	}

	// Return middleware handler
	return func(c fiber.Ctx) error {
		// Filter request to skip middleware
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Extract and verify key
		key, err := cfg.Extractor.Extract(c)
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

// Chain creates an Extractor that tries the provided extractors in order until one succeeds.
func Chain(extractors ...intextractor.Extractor) intextractor.Extractor {
	if len(extractors) == 0 {
		base := intextractor.Chain()
		return intextractor.Extractor{
			Extract: func(c fiber.Ctx) (string, error) {
				_, _ = base.Extract(c)
				return "", ErrMissingOrMalformedAPIKey
			},
			Chain: []intextractor.Extractor{},
		}
	}

	base := intextractor.Chain(extractors...)
	return intextractor.Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			val, err := base.Extract(c)
			if err != nil {
				return "", err
			}
			if val == "" {
				return "", ErrMissingOrMalformedAPIKey
			}
			return val, nil
		},
		Key:   extractors[0].Key,
		Chain: extractors,
	}
}

// FromHeader extracts the API key from the specified header and optional scheme.
func FromHeader(header, authScheme string) intextractor.Extractor {
	return intextractor.Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			auth := utils.Trim(c.Get(header), ' ')
			if auth == "" {
				return "", ErrMissingOrMalformedAPIKey
			}

			if authScheme == "" {
				return auth, nil
			}

			l := len(authScheme)
			if len(auth) <= l || !utils.EqualFold(auth[:l], authScheme) {
				return "", ErrMissingOrMalformedAPIKey
			}

			rest := auth[l:]
			if len(rest) == 0 || (rest[0] != ' ' && rest[0] != '\t') {
				return "", ErrMissingOrMalformedAPIKey
			}

			token := strings.TrimLeft(rest, " \t")
			if token == "" {
				return "", ErrMissingOrMalformedAPIKey
			}

			return token, nil
		},
		Key: header,
	}
}

// keyFromQuery returns a function that extracts api key from the query string.
func FromQuery(param string) intextractor.Extractor {
	base := intextractor.FromQuery(param)
	base.Extract = func(c fiber.Ctx) (string, error) {
		val, err := base.Extract(c)
		if err != nil {
			return "", ErrMissingOrMalformedAPIKey
		}
		return val, nil
	}
	return base
}

// keyFromForm returns a function that extracts api key from the form.
func FromForm(param string) intextractor.Extractor {
	base := intextractor.FromForm(param)
	base.Extract = func(c fiber.Ctx) (string, error) {
		val, err := base.Extract(c)
		if err != nil {
			return "", ErrMissingOrMalformedAPIKey
		}
		return val, nil
	}
	return base
}

// keyFromParam returns a function that extracts api key from the url param string.
func FromParam(param string) intextractor.Extractor {
	return intextractor.Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			key, err := url.PathUnescape(c.Params(param))
			if err != nil || key == "" {
				return "", ErrMissingOrMalformedAPIKey
			}
			return key, nil
		},
		Key: param,
	}
}

// keyFromCookie returns a function that extracts api key from the named cookie.
func FromCookie(name string) intextractor.Extractor {
	base := intextractor.FromCookie(name)
	base.Extract = func(c fiber.Ctx) (string, error) {
		val, err := base.Extract(c)
		if err != nil {
			return "", ErrMissingOrMalformedAPIKey
		}
		return val, nil
	}
	return base
}
