package keyauth

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// Source represents the type of source from which an API key is extracted.
// This is informational metadata that helps developers understand the extractor behavior.
type Source int

const (
	// SourceHeader indicates the key is extracted from an HTTP header.
	SourceHeader Source = iota

	// SourceAuthHeader indicates the key is extracted from the Authorization header.
	// This is a common method for API key extraction, often with a 'Bearer' or other scheme.
	SourceAuthHeader

	// SourceForm indicates the key is extracted from form data.
	SourceForm

	// SourceQuery indicates the key is extracted from URL query parameters.
	// This can be less secure as URLs may be logged.
	SourceQuery

	// SourceParam indicates the key is extracted from URL path parameters.
	// This can be less secure as URLs may be logged.
	SourceParam

	// SourceCookie indicates the key is extracted from cookies.
	SourceCookie

	// SourceCustom indicates the key is extracted using a custom extractor function.
	// Security depends on the implementation of the custom extractor.
	SourceCustom
)

// Extractor defines an API key extraction method with metadata.
type Extractor struct {
	Extract    func(fiber.Ctx) (string, error)
	Key        string      // The parameter/header name used for extraction
	AuthScheme string      // The auth scheme, e.g., "Bearer" for AuthHeader
	Chain      []Extractor // For chaining multiple extractors
	Source     Source      // The type of source being extracted from
}

var (
	ErrMissingAPIKey         = errors.New("missing api key")
	ErrMissingAPIKeyInHeader = errors.New("missing api key in header")
	ErrMissingAPIKeyInQuery  = errors.New("missing api key in query")
	ErrMissingAPIKeyInParam  = errors.New("missing api key in param")
	ErrMissingAPIKeyInForm   = errors.New("missing api key in form")
	ErrMissingAPIKeyInCookie = errors.New("missing api key in cookie")
)

// FromAuthHeader extracts an API key from the specified header and authentication scheme.
// It's commonly used for the "Authorization" header with a "Bearer" scheme.
func FromAuthHeader(header, authScheme string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			authHeader := c.Get(header)
			if authHeader == "" {
				return "", ErrMissingAPIKeyInHeader
			}

			// Check if the header starts with the specified auth scheme
			if authScheme != "" {
				schemeLen := len(authScheme)
				if len(authHeader) > schemeLen+1 && strings.EqualFold(authHeader[:schemeLen], authScheme) && authHeader[schemeLen] == ' ' {
					return strings.TrimSpace(authHeader[schemeLen+1:]), nil
				}
				return "", ErrMissingAPIKeyInHeader
			}

			return strings.TrimSpace(authHeader), nil
		},
		Key:        header,
		Source:     SourceAuthHeader,
		AuthScheme: authScheme,
	}
}

// FromCookie creates an Extractor that retrieves an API key from a specified cookie in the request.
//
// Parameters:
//   - key: The name of the cookie from which to extract the API key.
//
// Returns:
//
//	An Extractor that attempts to retrieve the API key from the specified cookie. If the cookie
//	is not present or does not contain an API key, it returns an error (ErrMissingAPIKeyInCookie).
func FromCookie(key string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			apiKey := c.Cookies(key)
			if apiKey == "" {
				return "", ErrMissingAPIKeyInCookie
			}
			return apiKey, nil
		},
		Key:    key,
		Source: SourceCookie,
	}
}

// FromParam creates an Extractor that retrieves an API key from a specified URL parameter in the request.
//
// Parameters:
//   - param: The name of the URL parameter from which to extract the API key.
//
// Returns:
//
//	An Extractor that attempts to retrieve the API key from the specified URL parameter. If the
//	parameter is not present or does not contain an API key, it returns an error (ErrMissingAPIKeyInParam).
func FromParam(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			apiKey := c.Params(param)
			if apiKey == "" {
				return "", ErrMissingAPIKeyInParam
			}
			return apiKey, nil
		},
		Key:    param,
		Source: SourceParam,
	}
}

// FromForm creates an Extractor that retrieves an API key from a specified form field in the request.
//
// Parameters:
//   - param: The name of the form field from which to extract the API key.
//
// Returns:
//
//	An Extractor that attempts to retrieve the API key from the specified form field. If the
//	field is not present or does not contain an API key, it returns an error (ErrMissingAPIKeyInForm).
func FromForm(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			apiKey := c.FormValue(param)
			if apiKey == "" {
				return "", ErrMissingAPIKeyInForm
			}
			return apiKey, nil
		},
		Key:    param,
		Source: SourceForm,
	}
}

// FromHeader creates an Extractor that retrieves an API key from a specified HTTP header in the request.
//
// Parameters:
//   - param: The name of the HTTP header from which to extract the API key.
//
// Returns:
//
//	An Extractor that attempts to retrieve the API key from the specified HTTP header. If the
//	header is not present or does not contain an API key, it returns an error (ErrMissingAPIKeyInHeader).
func FromHeader(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			apiKey := c.Get(param)
			if apiKey == "" {
				return "", ErrMissingAPIKeyInHeader
			}
			return apiKey, nil
		},
		Key:    param,
		Source: SourceHeader,
	}
}

// FromQuery creates an Extractor that retrieves an API key from a specified query parameter in the request.
//
// Parameters:
//   - param: The name of the query parameter from which to extract the API key.
//
// Returns:
//
//	An Extractor that attempts to retrieve the API key from the specified query parameter. If the
//	parameter is not present or does not contain an API key, it returns an error (ErrMissingAPIKeyInQuery).
func FromQuery(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			apiKey := fiber.Query[string](c, param)
			if apiKey == "" {
				return "", ErrMissingAPIKeyInQuery
			}
			return apiKey, nil
		},
		Key:    param,
		Source: SourceQuery,
	}
}

// Chain creates an Extractor that tries multiple extractors in order until one succeeds.
//
// Parameters:
//   - extractors: A variadic list of Extractor instances to try in sequence.
//
// Returns:
//
//	An Extractor that attempts each provided extractor in order and returns the first successful
//	extraction. If all extractors fail, it returns the last error encountered, or ErrMissingAPIKey
//	if no errors were returned. If no extractors are provided, it always fails with ErrMissingAPIKey.
func Chain(extractors ...Extractor) Extractor {
	if len(extractors) == 0 {
		return Extractor{
			Extract: func(fiber.Ctx) (string, error) {
				return "", ErrMissingAPIKey
			},
			Source: SourceCustom,
			Key:    "",
			Chain:  []Extractor{},
		}
	}

	// Use the source and key from the first extractor as the primary
	primarySource := extractors[0].Source
	primaryKey := extractors[0].Key

	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			var lastErr error

			for _, extractor := range extractors {
				token, err := extractor.Extract(c)

				if err == nil && token != "" {
					return token, nil
				}

				// Only update lastErr if we got an actual error
				if err != nil {
					lastErr = err
				}
			}
			if lastErr != nil {
				return "", lastErr
			}
			return "", ErrMissingAPIKey
		},
		Source: primarySource,
		Key:    primaryKey,
		Chain:  extractors,
	}
}
