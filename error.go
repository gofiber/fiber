package fiber

import "github.com/gofiber/fiber/v2/internal/schema"

type (
	// Conversion error exposes the internal schema.ConversionError for public use.
	ConversionError = schema.ConversionError
	// UnknownKeyError error exposes the internal schema.UnknownKeyError for public use.
	UnknownKeyError = schema.UnknownKeyError
	// EmptyFieldError error exposes the internal schema.EmptyFieldError for public use.
	EmptyFieldError = schema.EmptyFieldError
	// MultiError error exposes the internal schema.MultiError for public use.
	MultiError = schema.MultiError
)
