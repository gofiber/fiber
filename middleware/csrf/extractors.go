package csrf

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

var (
	ErrMissingHeader = errors.New("missing csrf token in header")
	ErrMissingQuery  = errors.New("missing csrf token in query")
	ErrMissingParam  = errors.New("missing csrf token in param")
	ErrMissingForm   = errors.New("missing csrf token in form")
	ErrMissingCookie = errors.New("missing csrf token in cookie")
)

// Note: FromCookie is intentionally omitted as it would defeat CSRF protection.
// See documentation for security implications of cookie-based extraction.

// FromParam returns a function that extracts token from the url param string.
func FromParam(param string) func(c fiber.Ctx) (string, error) {
	return func(c fiber.Ctx) (string, error) {
		token := c.Params(param)
		if token == "" {
			return "", ErrMissingParam
		}
		return token, nil
	}
}

// FromForm returns a function that extracts a token from a multipart-form.
func FromForm(param string) func(c fiber.Ctx) (string, error) {
	return func(c fiber.Ctx) (string, error) {
		token := c.FormValue(param)
		if token == "" {
			return "", ErrMissingForm
		}
		return token, nil
	}
}

// FromHeader returns a function that extracts token from the request header.
func FromHeader(param string) func(c fiber.Ctx) (string, error) {
	return func(c fiber.Ctx) (string, error) {
		token := c.Get(param)
		if token == "" {
			return "", ErrMissingHeader
		}
		return token, nil
	}
}

// FromQuery returns a function that extracts token from the query string.
func FromQuery(param string) func(c fiber.Ctx) (string, error) {
	return func(c fiber.Ctx) (string, error) {
		token := fiber.Query[string](c, param)
		if token == "" {
			return "", ErrMissingQuery
		}
		return token, nil
	}
}

// Chain tries multiple extractors in order until one succeeds.
// Returns the first successful extraction or the last error encountered.
func Chain(extractors ...func(fiber.Ctx) (string, error)) func(fiber.Ctx) (string, error) {
	if len(extractors) == 0 {
		return func(fiber.Ctx) (string, error) {
			return "", ErrTokenNotFound
		}
	}

	return func(c fiber.Ctx) (string, error) {
		var lastErr error
		var hasAttempted bool

		for _, extractor := range extractors {
			hasAttempted = true
			token, err := extractor(c)

			if err == nil && token != "" {
				return token, nil
			}

			// Only update lastErr if we got an actual error
			if err != nil {
				lastErr = err
			}
		}

		if hasAttempted && lastErr != nil {
			return "", lastErr
		}
		return "", ErrTokenNotFound
	}
}
