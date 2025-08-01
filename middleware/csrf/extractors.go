package csrf

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

// Source represents the type of source from which a CSRF token is extracted.
// This is informational metadata that helps developers understand the extractor behavior.
type Source int

const (
	// SourceHeader indicates the token is extracted from an HTTP header.
	// This is the most secure extraction method for CSRF protection.
	SourceHeader Source = iota

	// SourceForm indicates the token is extracted from form data.
	// This is secure for traditional form submissions.
	SourceForm

	// SourceQuery indicates the token is extracted from URL query parameters.
	// This is less secure as URLs may be logged, but acceptable for some use cases.
	SourceQuery

	// SourceParam indicates the token is extracted from URL path parameters.
	// This is less secure as URLs may be logged, but acceptable for some use cases.
	SourceParam

	// SourceCookie indicates the token is extracted from cookies.
	// This is not recommended for CSRF protection as it defeats the purpose of CSRF tokens.
	//
	// If you have an advanced use case that requires reading from cookies, and you understand
	// the security implications, set the Extractor source to SourceCookie. This will trigger
	// a check in the middleware to ensure the extractor does not read from cookies
	// with the same CookieName as the CSRF cookie.
	SourceCookie

	// SourceCustom indicates the token is extracted using a custom extractor function.
	// Security depends on the implementation of the custom extractor.
	SourceCustom
)

// Extractor defines a CSRF token extraction method with metadata
type Extractor struct {
	Extract func(fiber.Ctx) (string, error)
	Key     string      // The parameter/header name used for extraction
	Chain   []Extractor // For chaining multiple extractors
	Source  Source      // The type of source being extracted from
}

var (
	ErrMissingHeader = errors.New("csrf: token missing from header")
	ErrMissingQuery  = errors.New("csrf: token missing from query")
	ErrMissingParam  = errors.New("csrf: token missing from param")
	ErrMissingForm   = errors.New("csrf: token missing from form")
	ErrMissingCookie = errors.New("csrf: token missing from cookie")
)

// Note: FromCookie is intentionally omitted as it would defeat CSRF protection.
// See documentation for security implications of cookie-based extraction.

// FromParam creates an Extractor that retrieves a CSRF token from a specified URL parameter.
//
// Parameters:
//   - param: The name of the URL parameter from which to extract the token.
//
// Returns:
//
//	An Extractor that attempts to retrieve the CSRF token from the specified URL parameter.
//	If the parameter is not present or does not contain a token, it returns an error (ErrMissingParam).
//	This extractor has SourceParam type.
//
// Security: URLs may be logged by servers, proxies, and browsers, so this method should be used
// carefully in production environments.
func FromParam(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			token := c.Params(param)
			if token == "" {
				return "", ErrMissingParam
			}
			return token, nil
		},
		Key:    param,
		Source: SourceParam,
	}
}

// FromForm creates an Extractor that retrieves a CSRF token from a specified form field.
//
// Parameters:
//   - param: The name of the form field from which to extract the token.
//
// Returns:
//
//	An Extractor that attempts to retrieve the CSRF token from the specified form field.
//	If the field is not present or does not contain a token, it returns an error (ErrMissingForm).
//	This extractor has SourceForm type.
//
// Security: This is a secure method for CSRF protection as form data is not typically logged
// and cannot be manipulated via URL manipulation.
func FromForm(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			token := c.FormValue(param)
			if token == "" {
				return "", ErrMissingForm
			}
			return token, nil
		},
		Key:    param,
		Source: SourceForm,
	}
}

// FromHeader creates an Extractor that retrieves a CSRF token from a specified HTTP header.
//
// Parameters:
//   - param: The name of the HTTP header from which to extract the token.
//
// Returns:
//
//	An Extractor that attempts to retrieve the CSRF token from the specified HTTP header.
//	If the header is not present or does not contain a token, it returns an error (ErrMissingHeader).
//	This extractor has SourceHeader type.
//
// Security: This is the most secure method for CSRF protection, especially for APIs, as headers
// are not logged in standard web server logs and cannot be manipulated via simple URL manipulation.
func FromHeader(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			token := c.Get(param)
			if token == "" {
				return "", ErrMissingHeader
			}
			return token, nil
		},
		Key:    param,
		Source: SourceHeader,
	}
}

// FromQuery creates an Extractor that retrieves a CSRF token from a specified query parameter.
//
// Parameters:
//   - param: The name of the query parameter from which to extract the token.
//
// Returns:
//
//	An Extractor that attempts to retrieve the CSRF token from the specified query parameter.
//	If the parameter is not present or does not contain a token, it returns an error (ErrMissingQuery).
//	This extractor has SourceQuery type.
//
// Security: URLs may be logged by servers, proxies, and browsers, so this method should be used
// carefully in production environments.
func FromQuery(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			token := fiber.Query[string](c, param)
			if token == "" {
				return "", ErrMissingQuery
			}
			return token, nil
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
//	extraction. If all extractors fail, it returns the last error encountered, or ErrTokenNotFound
//	if no errors were returned. If no extractors are provided, it always fails with ErrTokenNotFound.
//	The returned extractor uses the Source and Key from the first extractor in the chain, and stores
//	all extractors in the Chain field.
//
// Security: Chaining multiple extractors can increase the attack surface and complexity. Most
// applications should use a single, appropriate extractor for their use case. Use chaining only
// when absolutely necessary for specific requirements.
func Chain(extractors ...Extractor) Extractor {
	if len(extractors) == 0 {
		return Extractor{
			Extract: func(fiber.Ctx) (string, error) {
				return "", ErrTokenNotFound
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
			return "", ErrTokenNotFound
		},
		Source: primarySource,
		Key:    primaryKey,
		Chain:  extractors,
	}
}
