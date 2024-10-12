package fiber

import (
	"encoding/json"
	"errors"

	"github.com/gofiber/schema"
)

// Wrap and return this for unreachable code if panicking is undesirable (i.e., in a handler).
// Unexported because users will hopefully never need to see it.
var errUnreachable = errors.New("fiber: unreachable code, please create an issue at github.com/gofiber/fiber")

// General errors
var (
	ErrGracefulTimeout = errors.New("shutdown: graceful timeout has been reached, exiting")
	// ErrNotRunning indicates that a Shutdown method was called when the server was not running.
	ErrNotRunning = errors.New("shutdown: server is not running")
	// ErrHandlerExited is returned by App.Test if a handler panics or calls runtime.Goexit().
	ErrHandlerExited = errors.New("runtime.Goexit() called in handler or server panic")
)

// Fiber redirection errors
var (
	ErrRedirectBackNoFallback = NewError(StatusInternalServerError, "Referer not found, you have to enter fallback URL for redirection.")
)

// Range errors
var (
	ErrRangeMalformed     = errors.New("range: malformed range header string")
	ErrRangeUnsatisfiable = errors.New("range: unsatisfiable range")
)

// Binder errors
var ErrCustomBinderNotFound = errors.New("binder: custom binder not found, please be sure to enter the right name")

// Format errors
var (
	// ErrNoHandlers is returned when c.Format is called with no arguments.
	ErrNoHandlers = errors.New("format: at least one handler is required, but none were set")
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
	InvalidUnmarshalError = json.InvalidUnmarshalError

	// A MarshalerError represents an error from calling a MarshalJSON or MarshalText method.
	MarshalerError = json.MarshalerError

	// A SyntaxError is a description of a JSON syntax error.
	SyntaxError = json.SyntaxError

	// An UnmarshalTypeError describes a JSON value that was
	// not appropriate for a value of a specific Go type.
	UnmarshalTypeError = json.UnmarshalTypeError

	// An UnsupportedTypeError is returned by Marshal when attempting
	// to encode an unsupported value type.
	UnsupportedTypeError = json.UnsupportedTypeError

	UnsupportedValueError = json.UnsupportedValueError
)
