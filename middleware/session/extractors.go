package session

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

type Source int

const (
	SourceCookie Source = iota
	SourceHeader
	SourceOther // For query, form, param, or custom extractors
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

// FromCookie returns an extractor that extracts session ID from the request cookie.
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

// FromParam returns an extractor that extracts session ID from the url param string.
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

// FromForm returns an extractor that extracts session ID from a multipart-form.
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

// FromHeader returns an extractor that extracts session ID from the request header.
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

// FromQuery returns an extractor that extracts session ID from the query string.
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

// Chain tries multiple extractors in order until one succeeds.
// Returns the first successful extraction or the last error encountered.
// If no extractors are provided, the returned extractor always fails with ErrMissingSessionID.
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
