package keyauth

import (
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

// FromAuthHeader extracts an API key from the specified header and authentication scheme.
// It's commonly used for the "Authorization" header with a "Bearer" scheme.
func FromAuthHeader(header, authScheme string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			authHeader := strings.Trim(c.Get(header), " \t")
			if authHeader == "" {
				return "", ErrMissingOrMalformedAPIKey
			}

			// Check if the header starts with the specified auth scheme
			if authScheme != "" {
				schemeLen := len(authScheme)
				if len(authHeader) <= schemeLen || !strings.EqualFold(authHeader[:schemeLen], authScheme) {
					return "", ErrMissingOrMalformedAPIKey
				}
				rest := authHeader[schemeLen:]
				if len(rest) == 0 || rest[0] != ' ' {
					return "", ErrMissingOrMalformedAPIKey
				}
				i := 1
				for i < len(rest) && rest[i] == ' ' {
					i++
				}
				if i < len(rest) && rest[i] == '\t' {
					return "", ErrMissingOrMalformedAPIKey
				}
				token := rest[i:]
				if token == "" || strings.IndexAny(token, " \t") >= 0 {
					return "", ErrMissingOrMalformedAPIKey
				}
				seenEq := false
				for j := 0; j < len(token); j++ {
					c := token[j]
					if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.' || c == '_' || c == '~' || c == '+' || c == '/' || c == '=') {
						return "", ErrMissingOrMalformedAPIKey
					}
					if c == '=' {
						if j == 0 {
							return "", ErrMissingOrMalformedAPIKey
						}
						seenEq = true
					} else if seenEq {
						return "", ErrMissingOrMalformedAPIKey
					}
				}
				return token, nil
			}

			return authHeader, nil
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
//	is not present or does not contain an API key, it returns ErrMissingOrMalformedAPIKey.
func FromCookie(key string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			apiKey := c.Cookies(key)
			if apiKey == "" {
				return "", ErrMissingOrMalformedAPIKey
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
//	parameter is not present or does not contain an API key, it returns ErrMissingOrMalformedAPIKey.
func FromParam(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			apiKey := c.Params(param)
			if apiKey == "" {
				return "", ErrMissingOrMalformedAPIKey
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
//	field is not present or does not contain an API key, it returns ErrMissingOrMalformedAPIKey.
func FromForm(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			apiKey := c.FormValue(param)
			if apiKey == "" {
				return "", ErrMissingOrMalformedAPIKey
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
//	header is not present or does not contain an API key, it returns ErrMissingOrMalformedAPIKey.
func FromHeader(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			apiKey := c.Get(param)
			if apiKey == "" {
				return "", ErrMissingOrMalformedAPIKey
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
//	parameter is not present or does not contain an API key, it returns ErrMissingOrMalformedAPIKey.
func FromQuery(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			apiKey := fiber.Query[string](c, param)
			if apiKey == "" {
				return "", ErrMissingOrMalformedAPIKey
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
//	extraction. If all extractors fail, it returns the last error encountered, or ErrMissingOrMalformedAPIKey
//	if no errors were returned. If no extractors are provided, it always fails with ErrMissingOrMalformedAPIKey.
func Chain(extractors ...Extractor) Extractor {
	if len(extractors) == 0 {
		return Extractor{
			Extract: func(fiber.Ctx) (string, error) {
				return "", ErrMissingOrMalformedAPIKey
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
			return "", ErrMissingOrMalformedAPIKey
		},
		Source: primarySource,
		Key:    primaryKey,
		Chain:  extractors,
	}
}
