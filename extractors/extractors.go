package extractors

// Package extractors provides shared value extraction utilities for Fiber middleware.
// This package helps reduce code duplication across middleware packages
// while ensuring consistent behavior, security practices, and RFC compliance.
// It can extract string values from various HTTP request sources including
// headers, cookies, query parameters, form data, and URL parameters.
//
// Example usage:
//
//	import "github.com/gofiber/fiber/v3/extractors"
//
//	// Extract from Authorization header
//	authExtractor := extractors.FromAuthHeader("Bearer")
//
//	// Chain multiple sources with fallback
//	tokenExtractor := extractors.Chain(
//	    extractors.FromHeader("X-API-Key"),
//	    extractors.FromCookie("api_key"),
//	    extractors.FromQuery("token"),
//	)
//
// Security considerations:
//   - Query parameters and form data can leak sensitive information
//   - Use HTTPS to protect extracted values in transit
//   - Consider source-specific security policies for your use case

import (
	"errors"
	"net/url"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// Source represents the type of source from which an API key is extracted.
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
	AuthScheme string      // The auth scheme used, e.g., "Bearer"
	Chain      []Extractor // For chained extractors, stores all extractors in the chain
	Source     Source      // The type of source being extracted from
}

// FromAuthHeader extracts a value from the Authorization header with an optional prefix.
// This function implements RFC 9110 compliant Authorization header parsing with strict token68 validation.
//
// RFC Compliance:
//   - Follows RFC 9110 Section 11.6.2 for Authorization header format
//   - Enforces 1*SP (one or more spaces) between auth-scheme and credentials
//   - Implements RFC 7235 token68 character validation for extracted tokens
//   - Case-insensitive auth scheme matching per HTTP standards
//
// Token68 Validation:
//   - Only allows characters: A-Z, a-z, 0-9, -, ., _, ~, +, /, =
//   - Rejects tokens containing spaces, tabs, or other whitespace
//   - Validates proper padding: = only at end, no characters after padding starts
//   - Prevents tokens starting with = (invalid padding)
//
// Security Features:
//   - Strict validation prevents header injection attacks
//   - Rejects malformed tokens that could bypass authentication
//   - Consistent error handling for missing or invalid credentials
//
// Parameters:
//   - authScheme: The auth scheme to strip from the header value (e.g., "Bearer", "Basic").
//     If empty, the entire header value is returned without validation.
//
// Returns:
//
//	An Extractor that attempts to retrieve and parse the Authorization header.
//	Returns ErrNotFound if the header is missing, malformed, or doesn't match the expected scheme.
//
// Examples:
//
//	// Extract Bearer token with validation
//	extractor := FromAuthHeader("Bearer")
//	// Input: "Bearer abc123" -> Output: "abc123"
//	// Input: "Bearer abc def" -> Output: ErrNotFound (space in token)
//	// Input: "Basic dXNlcjpwYXNz" -> Output: ErrNotFound (wrong scheme)
//
//	// Extract raw header value (no validation)
//	extractor := FromAuthHeader("")
//	// Input: "CustomAuth token123" -> Output: "CustomAuth token123"
func FromAuthHeader(authScheme string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			authHeader := c.Get(fiber.HeaderAuthorization)
			if authHeader == "" {
				return "", ErrNotFound
			}

			// Check if the header starts with the specified auth scheme
			if authScheme != "" {
				schemeLen := len(authScheme)
				if len(authHeader) <= schemeLen || !utils.EqualFold(authHeader[:schemeLen], authScheme) {
					return "", ErrNotFound
				}
				rest := authHeader[schemeLen:]
				if len(rest) == 0 || rest[0] != ' ' {
					return "", ErrNotFound
				}

				// Extract token after the required space
				token := rest[1:]
				if token == "" {
					return "", ErrNotFound
				}

				if !isValidToken68(token) {
					return "", ErrNotFound
				}

				return token, nil
			}

			return authHeader, nil
		},
		Key:        fiber.HeaderAuthorization,
		Source:     SourceAuthHeader,
		AuthScheme: authScheme,
	}
}

// FromCookie creates an Extractor that retrieves a value from a specified cookie in the request.
//
// The function:
//   - Retrieves the cookie value using the specified name
//   - Returns ErrNotFound if the cookie is missing
//
// Parameters:
//   - key: The name of the cookie from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified cookie.
//	Returns ErrNotFound if the cookie is not present.
//
// Security Note:
//
//	Cookies are generally more secure than query parameters for sensitive data
//	as they are not logged in access logs or visible in browser history.
//	However, ensure cookies are properly secured with appropriate flags.
//
// Example:
//
//	extractor := FromCookie("session_id")
//	// Cookie: "session_id=abc123" -> Output: "abc123"
//	// Missing cookie -> Output: ErrNotFound
func FromCookie(key string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			value := c.Cookies(key)
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
// URL parameters are extracted from the route path (e.g., /users/:id).
//
// SECURITY WARNING: Extracting values from URL parameters can leak sensitive information through:
//   - Server access logs and error logs
//   - Browser referrer headers when following links
//   - Proxy and intermediary server logs
//   - Browser history and bookmarks
//   - Network monitoring tools
//
// For sensitive data, prefer FromAuthHeader, FromCookie, or FromHeader instead.
//
// Parameters:
//   - param: The name of the URL parameter from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified URL parameter.
//	Returns ErrNotFound if the parameter is not present.
//
// Example:
//
//	// Route: GET /users/:userId/posts/:postId
//	userExtractor := FromParam("userId")
//	postExtractor := FromParam("postId")
//	// URL: /users/123/posts/456 -> userId: "123", postId: "456"
func FromParam(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			value := c.Params(param)
			if value == "" {
				return "", ErrNotFound
			}
			unescapedValue, err := url.PathUnescape(value)
			if err != nil {
				return "", ErrNotFound
			}
			return unescapedValue, nil
		},
		Key:    param,
		Source: SourceParam,
	}
}

// FromForm creates an Extractor that retrieves a value from a specified form field in the request.
// Form data is typically submitted via POST requests with content-type application/x-www-form-urlencoded.
//
// SECURITY WARNING: Extracting values from form data can leak sensitive information through:
//   - Server access logs and error logs
//   - Browser referrer headers (especially if form is submitted via GET)
//   - Proxy and intermediary server logs
//   - Browser history (if form uses GET method)
//
// For sensitive data, prefer FromAuthHeader or FromCookie instead.
// If using form data, ensure the form uses POST method and HTTPS.
//
// Parameters:
//   - param: The name of the form field from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified form field.
//	Returns ErrNotFound if the field is not present.
//
// Example:
//
//	extractor := FromForm("username")
//	// Form data: "username=john_doe&password=secret" -> Output: "john_doe"
//	// Missing field -> Output: ErrNotFound
func FromForm(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			value := c.FormValue(param)
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
// HTTP headers are commonly used for API keys, tokens, and other metadata.
//
// The function:
//   - Retrieves the header value using the specified name
//   - Returns ErrNotFound if the header is missing
//
// Parameters:
//   - header: The name of the HTTP header from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified HTTP header.
//	Returns ErrNotFound if the header is not present.
//
// Security Note:
//
//	Headers are generally secure for sensitive data as they are not logged
//	in access logs by default. However, be aware that some proxies may log headers.
//
// Example:
//
//	extractor := FromHeader("X-API-Key")
//	// Header: "X-API-Key: abc123" -> Output: "abc123"
//	// Missing header -> Output: ErrNotFound
func FromHeader(header string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			value := c.Get(header)
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
// Query parameters are extracted from the URL query string (e.g., ?key=value&foo=bar).
//
// SECURITY WARNING: Extracting values from URL query parameters can leak sensitive information through:
//   - Server access logs and error logs
//   - Browser referrer headers when following links
//   - Proxy and intermediary server logs
//   - Browser history and bookmarks
//   - Network monitoring tools and packet sniffers
//   - Web browser developer tools
//
// For sensitive data, prefer FromAuthHeader, FromCookie, or FromHeader instead.
// If query parameters must be used, ensure HTTPS is enforced.
//
// Parameters:
//   - param: The name of the query parameter from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified query parameter.
//	Returns ErrNotFound if the parameter is not present.
//
// Example:
//
//	extractor := FromQuery("token")
//	// URL: /api/data?token=abc123&format=json -> Output: "abc123"
//	// URL: /api/data?format=json -> Output: ErrNotFound
func FromQuery(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			value := c.Query(param)
			if value == "" {
				return "", ErrNotFound
			}
			return value, nil
		},
		Key:    param,
		Source: SourceQuery,
	}
}

// FromCustom creates an Extractor using a provided function.
// This allows for custom extraction logic beyond the built-in extractors.
//
// The function:
//   - Accepts a custom extraction function with signature func(fiber.Ctx) (string, error)
//   - Handles nil functions gracefully by returning ErrNotFound
//   - Preserves the custom function for execution
//
// Parameters:
//   - key: A descriptive identifier for the custom extractor.
//     Used for debugging, logging, and Chain metadata. Should be meaningful for introspection.
//     Examples: "X-Custom-Header", "Database-Lookup", "Cache-Key"
//   - fn: The custom function to extract the value from the fiber.Ctx.
//     If nil, the extractor will return ErrNotFound when executed.
//     The function should return (value, nil) on success or ("", error) on failure.
//
// Returns:
//
//	An Extractor that uses the provided function for extraction.
//	If fn is nil, the returned extractor will always return ErrNotFound.
//
// Examples:
//
//	// Custom header with transformation
//	extractor := FromCustom("X-API-Key", func(c fiber.Ctx) (string, error) {
//	    value := c.Get("X-API-Key")
//	    if value == "" {
//	        return "", ErrNotFound
//	    }
//	    return strings.ToUpper(value), nil
//	})
//
//	// Database lookup (pseudo-code)
//	userExtractor := FromCustom("user-from-db", func(c fiber.Ctx) (string, error) {
//	    userID := c.Params("userId")
//	    user, err := db.GetUser(userID)
//	    if err != nil {
//	        return "", err
//	    }
//	    return user.Name, nil
//	})
//
//	// Conditional extraction
//	smartExtractor := FromCustom("smart-auth", func(c fiber.Ctx) (string, error) {
//	    if c.Get("X-Service-Auth") != "" {
//	        return c.Get("X-Service-Auth"), nil
//	    }
//	    return c.Cookies("session"), nil
//	})
func FromCustom(key string, fn func(fiber.Ctx) (string, error)) Extractor {
	if fn == nil {
		fn = func(fiber.Ctx) (string, error) { return "", ErrNotFound }
	}
	return Extractor{
		Extract: fn,
		Key:     key,
		Source:  SourceCustom,
	}
}

// Chain creates an Extractor that tries multiple extractors in order until one succeeds.
// This implements a fallback pattern where multiple extraction sources are attempted in sequence.
//
// The function:
//   - Tries each extractor in the order provided
//   - Returns the first successful extraction (non-empty value with no error)
//   - Skips extractors with nil Extract functions
//   - Returns the last error encountered if all extractors fail
//   - Returns ErrNotFound if no extractors are provided or all return empty values
//
// Parameters:
//   - extractors: A variadic list of Extractor instances to try in sequence.
//     The order matters - more secure/preferred sources should be listed first.
//
// Returns:
//
//	An Extractor that attempts each provided extractor in order.
//	The returned extractor uses the Source and Key from the first extractor for metadata.
//
// Behavior:
//   - Success: Returns the first non-empty value with no error
//   - Partial failure: Continues to next extractor if current returns error or empty value
//   - Total failure: Returns last error encountered, or ErrNotFound if no errors
//   - Empty chain: Always returns ErrNotFound
//
// Examples:
//
//	// Try header first, then cookie, then query param
//	extractor := Chain(
//	    FromHeader("Authorization"),
//	    FromCookie("auth_token"),
//	    FromQuery("token"),
//	)
//
//	// API key from multiple possible sources
//	apiKeyExtractor := Chain(
//	    FromHeader("X-API-Key"),
//	    FromQuery("api_key"),
//	    FromForm("apiKey"),
//	)
//
// Security Note:
//
//	Order extractors by security preference. Most secure sources (headers, cookies)
//	should be attempted before less secure ones (query params, form data).
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
			var lastErr error // last error encountered (including ErrNotFound)

			for _, extractor := range extractors {
				if extractor.Extract == nil {
					continue
				}
				v, err := extractor.Extract(c)
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
			return "", ErrNotFound
		},
		Source: primarySource,
		Key:    primaryKey,
		Chain:  append([]Extractor(nil), extractors...), // Defensive copy for introspection
	}
}

// isValidToken68 checks if a string is a valid token68 per RFC 7235/9110.
func isValidToken68(token string) bool {
	if token == "" {
		return false
	}
	paddingStarted := false
	for i, c := range []byte(token) {
		switch {
		case (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' || c == '+' || c == '/':
			if paddingStarted {
				return false // No characters allowed after padding starts
			}
		case c == '=':
			if i == 0 {
				return false // Cannot start with padding
			}
			paddingStarted = true
		default:
			return false // Invalid character
		}
	}
	return true
}
