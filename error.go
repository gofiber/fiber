package fiber

import (
	errors "encoding/json"
	goErrors "errors"

	"github.com/gofiber/fiber/v3/internal/schema"
)

// Range errors
var (
	ErrRangeMalformed     = goErrors.New("range: malformed range header string")
	ErrRangeUnsatisfiable = goErrors.New("range: unsatisfiable range")
)

// Binder errors
var ErrCustomBinderNotFound = goErrors.New("binder: custom binder not found, please be sure to enter the right name")

// gorilla/schema errors
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

// encoding/json errors
type (
	// An InvalidUnmarshalError describes an invalid argument passed to Unmarshal.
	// (The argument to Unmarshal must be a non-nil pointer.)
	InvalidUnmarshalError = errors.InvalidUnmarshalError

	// A MarshalerError represents an error from calling a MarshalJSON or MarshalText method.
	MarshalerError = errors.MarshalerError

	// A SyntaxError is a description of a JSON syntax error.
	SyntaxError = errors.SyntaxError

	// An UnmarshalTypeError describes a JSON value that was
	// not appropriate for a value of a specific Go type.
	UnmarshalTypeError = errors.UnmarshalTypeError

	// An UnsupportedTypeError is returned by Marshal when attempting
	// to encode an unsupported value type.
	UnsupportedTypeError = errors.UnsupportedTypeError

	UnsupportedValueError = errors.UnsupportedValueError
)
