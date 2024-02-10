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

// FromCookie returns a function that extracts token from the cookie header.
func FromCookie(param string) func(c fiber.Ctx) (string, error) {
	return func(c fiber.Ctx) (string, error) {
		token := c.Cookies(param)
		if token == "" {
			return "", ErrMissingCookie
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
