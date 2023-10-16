package csrf

import (
	"errors"

	"github.com/gofiber/fiber/v2"
)

var (
	ErrMissingHeader = errors.New("missing csrf token in header")
	ErrMissingQuery  = errors.New("missing csrf token in query")
	ErrMissingParam  = errors.New("missing csrf token in param")
	ErrMissingForm   = errors.New("missing csrf token in form")
	ErrMissingCookie = errors.New("missing csrf token in cookie")
)

// csrfFromParam returns a function that extracts token from the url param string.
func CsrfFromParam(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.Params(param)
		if token == "" {
			return "", ErrMissingParam
		}
		return token, nil
	}
}

// csrfFromForm returns a function that extracts a token from a multipart-form.
func CsrfFromForm(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.FormValue(param)
		if token == "" {
			return "", ErrMissingForm
		}
		return token, nil
	}
}

// csrfFromCookie returns a function that extracts token from the cookie header.
func CsrfFromCookie(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.Cookies(param)
		if token == "" {
			return "", ErrMissingCookie
		}
		return token, nil
	}
}

// csrfFromHeader returns a function that extracts token from the request header.
func CsrfFromHeader(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.Get(param)
		if token == "" {
			return "", ErrMissingHeader
		}
		return token, nil
	}
}

// csrfFromQuery returns a function that extracts token from the query string.
func CsrfFromQuery(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.Query(param)
		if token == "" {
			return "", ErrMissingQuery
		}
		return token, nil
	}
}
