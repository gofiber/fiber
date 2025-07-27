package session

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

// Source represents the type of source from which a session ID is extracted.
// The source type determines how the session middleware handles response values:
//   - SourceCookie: Sets cookies in the response when saving sessions
//   - SourceHeader: Sets headers in the response when saving sessions
//   - SourceOther: Read-only extraction; does not set any response values
type Source int

const (
	// SourceCookie indicates the session ID is extracted from a cookie.
	// When using this source type, the session middleware will automatically
	// set the session ID as a cookie in the response when saving sessions.
	SourceCookie Source = iota

	// SourceHeader indicates the session ID is extracted from an HTTP header.
	// When using this source type, the session middleware will automatically
	// set the session ID as a header in the response when saving sessions.
	SourceHeader

	// SourceOther indicates the session ID is extracted from other sources
	// such as query parameters, form fields, URL parameters, or custom extractors.
	// When using this source type, the session middleware operates in read-only mode
	// and will NOT set any response values (cookies or headers) when saving sessions.
	// This is useful for extracting session IDs from sources that should not be
	// automatically written back to the response.
	SourceOther
)

type Extractor struct {
	Extract func(fiber.Ctx) (string, error)
	Key     string
	Chain   []Extractor // For chaining multiple extractors
	Source  Source
}

var (
	ErrMissingSessionID         = errors.New("missing session id")
	ErrMissingSessionIDInHeader = errors.New("missing session id in header")
	ErrMissingSessionIDInQuery  = errors.New("missing session id in query")
	ErrMissingSessionIDInParam  = errors.New("missing session id in param")
	ErrMissingSessionIDInForm   = errors.New("missing session id in form")
	ErrMissingSessionIDInCookie = errors.New("missing session id in cookie")
)

// FromCookie creates an Extractor that retrieves a session ID from a specified cookie in the request.
//
// Parameters:
//   - key: The name of the cookie from which to extract the session ID.
//
// Returns:
//
//	An Extractor that attempts to retrieve the session ID from the specified cookie. If the cookie
//	is not present or does not contain a session ID, it returns an error (ErrMissingSessionIDInCookie).
func FromCookie(key string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			sessionID := c.Cookies(key)
			if sessionID == "" {
				return "", ErrMissingSessionIDInCookie
			}
			return sessionID, nil
		},
		Source: SourceCookie,
		Key:    key,
	}
}

// FromParam creates an Extractor that retrieves a session ID from a specified URL parameter in the request.
//
// Parameters:
//   - param: The name of the URL parameter from which to extract the session ID.
//
// Returns:
//
//	An Extractor that attempts to retrieve the session ID from the specified URL parameter. If the
//	parameter is not present or does not contain a session ID, it returns an error (ErrMissingSessionIDInParam).
//	This extractor has SourceOther type, meaning it will not set response values.
func FromParam(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			sessionID := c.Params(param)
			if sessionID == "" {
				return "", ErrMissingSessionIDInParam
			}
			return sessionID, nil
		},
		Source: SourceOther,
		Key:    param,
	}
}

// FromForm creates an Extractor that retrieves a session ID from a specified form field in the request.
//
// Parameters:
//   - param: The name of the form field from which to extract the session ID.
//
// Returns:
//
//	An Extractor that attempts to retrieve the session ID from the specified form field. If the
//	field is not present or does not contain a session ID, it returns an error (ErrMissingSessionIDInForm).
//	This extractor has SourceOther type, meaning it will not set response values.
func FromForm(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			sessionID := c.FormValue(param)
			if sessionID == "" {
				return "", ErrMissingSessionIDInForm
			}
			return sessionID, nil
		},
		Source: SourceOther,
		Key:    param,
	}
}

// FromHeader creates an Extractor that retrieves a session ID from a specified HTTP header in the request.
//
// Parameters:
//   - param: The name of the HTTP header from which to extract the session ID.
//
// Returns:
//
//	An Extractor that attempts to retrieve the session ID from the specified HTTP header. If the
//	header is not present or does not contain a session ID, it returns an error (ErrMissingSessionIDInHeader).
//	This extractor has SourceHeader type, meaning it will set response headers when saving sessions.
func FromHeader(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			sessionID := c.Get(param)
			if sessionID == "" {
				return "", ErrMissingSessionIDInHeader
			}
			return sessionID, nil
		},
		Source: SourceHeader,
		Key:    param,
	}
}

// FromQuery creates an Extractor that retrieves a session ID from a specified query parameter in the request.
//
// Parameters:
//   - param: The name of the query parameter from which to extract the session ID.
//
// Returns:
//
//	An Extractor that attempts to retrieve the session ID from the specified query parameter. If the
//	parameter is not present or does not contain a session ID, it returns an error (ErrMissingSessionIDInQuery).
//	This extractor has SourceOther type, meaning it will not set response values.
func FromQuery(param string) Extractor {
	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			sessionID := fiber.Query[string](c, param)
			if sessionID == "" {
				return "", ErrMissingSessionIDInQuery
			}
			return sessionID, nil
		},
		Source: SourceOther,
		Key:    param,
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
//	extraction. If all extractors fail, it returns the last error encountered, or ErrMissingSessionID
//	if no errors were returned. If no extractors are provided, it always fails with ErrMissingSessionID.
//	The returned extractor uses the Source and Key from the first extractor in the chain, and stores
//	all extractors in the Chain field for response handling logic.
func Chain(extractors ...Extractor) Extractor {
	if len(extractors) == 0 {
		return Extractor{
			Extract: func(fiber.Ctx) (string, error) {
				return "", ErrMissingSessionID
			},
			Source: SourceOther,
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
				sessionID, err := extractor.Extract(c)

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
		},
		Source: primarySource,
		Key:    primaryKey,
		Chain:  extractors,
	}
}
