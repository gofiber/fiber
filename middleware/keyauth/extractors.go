package keyauth

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	intextractor "github.com/gofiber/fiber/v3/extractor"
	"github.com/gofiber/utils/v2"
)

// withKeyauthError wraps an existing extractor to return ErrMissingOrMalformedAPIKey on failure.
func withKeyauthError(e intextractor.Extractor) intextractor.Extractor {
	return intextractor.Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			val, err := e.Extract(c)
			if err != nil {
				return "", ErrMissingOrMalformedAPIKey
			}
			return val, nil
		},
		Key: e.Key,
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
	return withKeyauthError(intextractor.FromQuery(param))
}

// keyFromForm returns a function that extracts api key from the form.
func FromForm(param string) intextractor.Extractor {
	return withKeyauthError(intextractor.FromForm(param))
}

// keyFromParam returns a function that extracts api key from the url param string.
func FromParam(param string) intextractor.Extractor {
	return withKeyauthError(intextractor.FromParam(param))
}

// keyFromCookie returns a function that extracts api key from the named cookie.
func FromCookie(name string) intextractor.Extractor {
	return withKeyauthError(intextractor.FromCookie(name))
}

// Chain creates an Extractor that tries the provided extractors in order until one succeeds.
func Chain(extractors ...intextractor.Extractor) intextractor.Extractor {
	base := intextractor.Chain(extractors...)
	return intextractor.Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			val, err := base.Extract(c)
			if err != nil {
				// Preserve the specific error, but default to ErrMissingOrMalformedAPIKey
				// if the underlying chain returns a generic error.
				if errors.Is(err, intextractor.ErrValueNotFound) {
					return "", ErrMissingOrMalformedAPIKey
				}
				return "", err
			}
			if val == "" {
				return "", ErrMissingOrMalformedAPIKey
			}
			return val, nil
		},
		Key:   base.Key,
		Chain: base.Chain,
	}
}
