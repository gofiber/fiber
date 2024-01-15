package fiber

import (
	errors "encoding/json"
	stdErrors "errors"

	"github.com/gofiber/fiber/v3/internal/schema"
)

// Wrap and return this for unreachable code if panicking is undesirable (i.e., in a handler).
// Unexported because users will hopefully never need to see it.
var errUnreachable = stdErrors.New("fiber: unreachable code, please create an issue at github.com/gofiber/fiber")

// Graceful shutdown errors
var (
	ErrGracefulTimeout = stdErrors.New("shutdown: graceful timeout has been reached, exiting")
)

// Fiber redirection errors
var (
	ErrRedirectBackNoFallback = NewError(StatusInternalServerError, "Referer not found, you have to enter fallback URL for redirection.")
)

// Range errors
var (
	ErrRangeMalformed     = stdErrors.New("range: malformed range header string")
	ErrRangeUnsatisfiable = stdErrors.New("range: unsatisfiable range")
)

// Binder errors
var ErrCustomBinderNotFound = stdErrors.New("binder: custom binder not found, please be sure to enter the right name")

// Format errors
var (
	// ErrNoHandlers is returned when c.Format is called with no arguments.
	ErrNoHandlers = stdErrors.New("format: at least one handler is required, but none were set")
)

// gorilla/schema errors
type (
	// ConversionError Conversion error exposes the internal schema.ConversionError for public use.
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
