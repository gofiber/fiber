package extractor

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// Extractor defines a value extraction function with metadata.
type Extractor struct {
	Extract func(fiber.Ctx) (string, error)
	Key     string
	Chain   []Extractor // For chaining multiple extractors
}

var (
	ErrValueNotFound     = errors.New("value not found")
	ErrMissingHeader     = errors.New("missing value in header")
	ErrMissingQuery      = errors.New("missing value in query")
	ErrMissingParam      = errors.New("missing value in param")
	ErrMissingForm       = errors.New("missing value in form")
	ErrMissingCookie     = errors.New("missing value in cookie")
	ErrInvalidAuthHeader = errors.New("invalid authentication header format")
)

// FromAuthHeader creates an Extractor that retrieves a value from the specified HTTP header
// and authentication scheme. This is commonly used for the "Authorization" header with a "Bearer" scheme.
func FromAuthHeader(header string, scheme string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			v := c.Get(header)
			if v == "" {
				return "", ErrMissingHeader
			}
			if !strings.HasPrefix(v, scheme+" ") {
				return "", ErrInvalidAuthHeader
			}
			return strings.TrimPrefix(v, scheme+" "), nil
		},
		Key: header,
	}
}

// FromCookie returns an Extractor that gets a value from the given cookie.
func FromCookie(key string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			v := c.Cookies(key)
			if v == "" {
				return "", ErrMissingCookie
			}
			return v, nil
		},
		Key: key,
	}
}

// FromParam returns an Extractor that gets a value from the given URL parameter.
func FromParam(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			v := c.Params(param)
			if v == "" {
				return "", ErrMissingParam
			}
			return v, nil
		},
		Key: param,
	}
}

// FromForm returns an Extractor that gets a value from the given form field.
func FromForm(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			v := c.FormValue(param)
			if v == "" {
				return "", ErrMissingForm
			}
			return v, nil
		},
		Key: param,
	}
}

// FromHeader returns an Extractor that gets a value from the given header.
func FromHeader(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			v := c.Get(param)
			if v == "" {
				return "", ErrMissingHeader
			}
			return v, nil
		},
		Key: param,
	}
}

// FromQuery returns an Extractor that gets a value from the given query parameter.
func FromQuery(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			v := fiber.Query[string](c, param)
			if v == "" {
				return "", ErrMissingQuery
			}
			return v, nil
		},
		Key: param,
	}
}

// Chain tries the provided extractors in order until one succeeds.
func Chain(extractors ...Extractor) Extractor {
	if len(extractors) == 0 {
		return Extractor{
			Extract: func(fiber.Ctx) (string, error) {
				return "", ErrValueNotFound
			},
			Chain: []Extractor{},
		}
	}

	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			var lastErr error
			for _, ex := range extractors {
				v, err := ex.Extract(c)
				if err == nil && v != "" {
					return v, nil
				}
				if err != nil {
					lastErr = err
				}
			}
			if lastErr != nil {
				return "", lastErr
			}
			return "", ErrValueNotFound
		},
		Key:   extractors[0].Key,
		Chain: extractors,
	}
}
