package extractors

// Package extractors provides shared value extraction utilities for Fiber middleware.
// This internal package helps reduce code duplication across middleware packages
// while allowing selective inclusion of extractors based on middleware needs.
// It can extract any string value from various HTTP request sources.

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// Source represents the type of source from which a value is extracted.
// This is informational metadata that helps developers understand the extractor behavior.
type Source int

const (
	// SourceHeader indicates the value is extracted from an HTTP header.
	SourceHeader Source = iota

	// SourceAuthHeader indicates the value is extracted from the Authorization header.
	SourceAuthHeader

	// SourceForm indicates the value is extracted from form data.
	SourceForm

	// SourceQuery indicates the value is extracted from URL query parameters.
	SourceQuery

	// SourceParam indicates the value is extracted from URL path parameters.
	SourceParam

	// SourceCookie indicates the value is extracted from cookies.
	SourceCookie

	// SourceCustom indicates the value is extracted using a custom extractor function.
	SourceCustom
)

// ErrNotFound is returned when the requested value is missing or empty.
var ErrNotFound = errors.New("value not found")

// Extractor defines a value extraction method with metadata.
type Extractor struct {
	Extract    func(fiber.Ctx) (string, error)
	Key        string      // The parameter/header name used for extraction
	Source     Source      // The type of source being extracted from
	AuthScheme string      // The auth scheme used, e.g., "Bearer"
	Chain      []Extractor // For chained extractors, stores all extractors in the chain
}

// FromAuthHeader extracts a value from the Authorization header with an optional prefix.
// This is a convenience function for the common case of extracting from the Authorization header.
//
// Parameters:
//   - authScheme: The auth scheme to strip from the header value (e.g., "Bearer"). If empty, no prefix is stripped.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the Authorization header.
func FromAuthHeader(authScheme string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			authHeader := c.Get(fiber.HeaderAuthorization)
			if authHeader == "" {
				return "", ErrNotFound
			}

			if authScheme != "" {
				parts := strings.Fields(authHeader)
				if len(parts) >= 2 && strings.EqualFold(parts[0], authScheme) {
					return parts[1], nil
				}
				return "", ErrNotFound
			}

			return strings.TrimSpace(authHeader), nil
		},
		Key:        fiber.HeaderAuthorization,
		Source:     SourceAuthHeader,
		AuthScheme: authScheme,
	}
}

// FromCookie creates an Extractor that retrieves a value from a specified cookie in the request.
//
// Parameters:
//   - key: The name of the cookie from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified cookie. If the cookie
//	is not present or does not contain a value, it returns ErrNotFound.
func FromCookie(key string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			value := strings.TrimSpace(c.Cookies(key))
			if value == "" {
				return "", ErrNotFound
			}
			return value, nil
		},
		Key:    key,
		Source: SourceCookie,
	}
}

// FromParam creates an Extractor that retrieves a value from a specified URL parameter in the request.
//
// SECURITY WARNING: Extracting values from URL parameters can leak sensitive information through:
// - Server logs and access logs
// - Browser referrer headers
// - Proxy and intermediary logs
// - Browser history
// Consider using FromAuthHeader or FromCookie for better security.
//
// Parameters:
//   - param: The name of the URL parameter from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified URL parameter. If the
//	parameter is not present or does not contain a value, it returns ErrNotFound.
func FromParam(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			value := strings.TrimSpace(c.Params(param))
			if value == "" {
				return "", ErrNotFound
			}
			return value, nil
		},
		Key:    param,
		Source: SourceParam,
	}
}

// FromForm creates an Extractor that retrieves a value from a specified form field in the request.
//
// SECURITY WARNING: Extracting values from form data can leak sensitive information through:
// - Server logs and access logs
// - Browser referrer headers (if form is submitted via GET)
// - Proxy and intermediary logs
// Consider using FromAuthHeader or FromCookie for better security.
//
// Parameters:
//   - param: The name of the form field from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified form field. If the
//	field is not present or does not contain a value, it returns ErrNotFound.
func FromForm(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			value := strings.TrimSpace(c.FormValue(param))
			if value == "" {
				return "", ErrNotFound
			}
			return value, nil
		},
		Key:    param,
		Source: SourceForm,
	}
}

// FromHeader creates an Extractor that retrieves a value from a specified HTTP header in the request.
//
// Parameters:
//   - header: The name of the HTTP header from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified HTTP header. If the
//	header is not present or does not contain a value, it returns ErrNotFound.
func FromHeader(header string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			value := strings.TrimSpace(c.Get(header))
			if value == "" {
				return "", ErrNotFound
			}
			return value, nil
		},
		Key:    header,
		Source: SourceHeader,
	}
}

// FromQuery creates an Extractor that retrieves a value from a specified query parameter in the request.
//
// SECURITY WARNING: Extracting values from URL query parameters can leak sensitive information through:
// - Server logs and access logs
// - Browser referrer headers
// - Proxy and intermediary logs
// - Browser history and bookmarks
// - Network monitoring tools
// Consider using FromAuthHeader or FromCookie for better security.
//
// Parameters:
//   - param: The name of the query parameter from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified query parameter. If the
//	parameter is not present or does not contain a value, it returns ErrNotFound.
func FromQuery(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			value := strings.TrimSpace(c.Query(param))
			if value == "" {
				return "", ErrNotFound
			}
			return value, nil
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
//	extraction. If all extractors fail, it returns the last error encountered, or ErrNotFound
//	if no errors were returned. If no extractors are provided, it always fails with ErrNotFound.
func Chain(extractors ...Extractor) Extractor {
	if len(extractors) == 0 {
		return Extractor{
			Extract: func(fiber.Ctx) (string, error) {
				return "", ErrNotFound
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
				value, err := extractor.Extract(c)

				if err == nil && value != "" {
					return value, nil
				}

				// Only update lastErr if we got an actual error
				if err != nil {
					lastErr = err
				}
			}
			if lastErr != nil {
				return "", lastErr
			}
			return "", ErrNotFound
		},
		Source: primarySource,
		Key:    primaryKey,
		Chain:  append([]Extractor(nil), extractors...), // Defensive copy for introspection
	}
}
