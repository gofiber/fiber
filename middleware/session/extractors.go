package session

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

var (
	ErrMissingSessionID         = errors.New("missing session id")
	ErrMissingSessionIDInHeader = errors.New("missing session id in header")
	ErrMissingSessionIDInQuery  = errors.New("missing session id in query")
	ErrMissingSessionIDInParam  = errors.New("missing session id in param")
	ErrMissingSessionIDInForm   = errors.New("missing session id in form")
	ErrMissingSessionIDInCookie = errors.New("missing session id in cookie")
)

// FromCookie returns a function that extracts session ID from the request cookie.
func FromCookie(key string) func(c fiber.Ctx) (string, error) {
	return func(c fiber.Ctx) (string, error) {
		sessionID := c.Cookies(key)
		if sessionID == "" {
			return "", ErrMissingSessionIDInCookie
		}
		return sessionID, nil
	}
}

// FromParam returns a function that extracts session ID from the url param string.
func FromParam(param string) func(c fiber.Ctx) (string, error) {
	return func(c fiber.Ctx) (string, error) {
		sessionID := c.Params(param)
		if sessionID == "" {
			return "", ErrMissingSessionIDInParam
		}
		return sessionID, nil
	}
}

// FromForm returns a function that extracts session ID from a multipart-form.
func FromForm(param string) func(c fiber.Ctx) (string, error) {
	return func(c fiber.Ctx) (string, error) {
		sessionID := c.FormValue(param)
		if sessionID == "" {
			return "", ErrMissingSessionIDInForm
		}
		return sessionID, nil
	}
}

// FromHeader returns a function that extracts session ID from the request header.
func FromHeader(param string) func(c fiber.Ctx) (string, error) {
	return func(c fiber.Ctx) (string, error) {
		sessionID := c.Get(param)
		if sessionID == "" {
			return "", ErrMissingSessionIDInHeader
		}
		return sessionID, nil
	}
}

// FromQuery returns a function that extracts session ID from the query string.
func FromQuery(param string) func(c fiber.Ctx) (string, error) {
	return func(c fiber.Ctx) (string, error) {
		sessionID := fiber.Query[string](c, param)
		if sessionID == "" {
			return "", ErrMissingSessionIDInQuery
		}
		return sessionID, nil
	}
}

// Chain tries multiple extractors in order until one succeeds.
// Returns the first successful extraction or the last error encountered.
func Chain(extractors ...func(fiber.Ctx) (string, error)) func(fiber.Ctx) (string, error) {
	if len(extractors) == 0 {
		return func(fiber.Ctx) (string, error) {
			return "", ErrMissingSessionID // Default fallback error
		}
	}

	return func(c fiber.Ctx) (string, error) {
		var lastErr error

		for _, extractor := range extractors {
			sessionID, err := extractor(c)

			if err == nil && sessionID != "" {
				return sessionID, nil
			}

			// Only update lastErr if we got an actual error
			if err != nil {
				lastErr = err
			}
		}
		if lastErr != nil {
			return "", lastErr
		}
		return "", ErrMissingSessionID
	}
}
}
