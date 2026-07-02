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
	// ErrNoViewEngineConfigured indicates that a helper requiring a view engine was invoked without one configured.
	ErrNoViewEngineConfigured = errors.New("fiber: no view engine configured")
	// ErrAutoCertWithCertFile indicates AutoCertManager cannot be used with CertFile/CertKeyFile.
	ErrAutoCertWithCertFile = errors.New("tls: AutoCertManager cannot be combined with CertFile/CertKeyFile")
)

// Fiber redirection errors
var (
	ErrRedirectBackNoFallback = NewError(StatusInternalServerError, "Referer not found, you have to enter fallback URL for redirection.")
)

// Range errors
var (
	// ErrRangeMalformed is returned for a syntactically invalid Range header,
	// which RFC 9110 Section 14.2 allows a server to reject; it carries a
	// 400 Bad Request status so propagating it does not surface as a 500.
	ErrRangeMalformed = NewError(StatusBadRequest, "range: malformed range header string")
	// ErrRangeUnsupported is returned for a Range header whose range unit is
	// not "bytes". RFC 9110 Section 14.2 requires an origin server to IGNORE
	// a Range header field with a range unit it does not understand, so
	// callers receiving this error should serve the full representation
	// instead of returning an error response.
	ErrRangeUnsupported   = errors.New("range: unsupported range unit")
	ErrRangeTooLarge      = NewError(StatusRequestedRangeNotSatisfiable, "range: too many ranges")
	ErrRangeUnsatisfiable = errors.New("range: unsatisfiable range")
	// errRangeBound: absent range bound (the empty side of a suffix or
	// open-ended range); control-flow only, never surfaced.
	errRangeBound = errors.New("range: bound not parsable")
)

// Binder errors
var ErrCustomBinderNotFound = errors.New("binder: custom binder not found, please be sure to enter the right name")

// Format errors
var (
	// ErrNoHandlers is returned when c.Format is called with no arguments.
	ErrNoHandlers = errors.New("format: at least one handler is required, but none were set")
)

// gofiber/schema errors
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
	// InvalidUnmarshalError describes an invalid argument passed to Unmarshal.
	// (The argument to Unmarshal must be a non-nil pointer.)
	InvalidUnmarshalError = json.InvalidUnmarshalError

	// MarshalerError represents an error from calling a MarshalJSON or MarshalText method.
	MarshalerError = json.MarshalerError

	// SyntaxError is a description of a JSON syntax error.
	SyntaxError = json.SyntaxError

	// UnmarshalTypeError describes a JSON value that was
	// not appropriate for a value of a specific Go type.
	UnmarshalTypeError = json.UnmarshalTypeError

	// UnsupportedTypeError is returned by Marshal when attempting
	// to encode an unsupported value type.
	UnsupportedTypeError = json.UnsupportedTypeError

	// UnsupportedValueError exposes json.UnsupportedValueError to describe unsupported values encountered during encoding.
	UnsupportedValueError = json.UnsupportedValueError
)

// File errors
var (
	ErrFileHeaderNil = errors.New("file: file header is nil")
	ErrFileOpen      = errors.New("file: failed to open file")
	ErrFileRead      = errors.New("file: failed to read file")
	ErrFileStore     = errors.New("file: failed to store file")
)
