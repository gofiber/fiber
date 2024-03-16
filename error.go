package fiber

import (
	"errors"
	"reflect"
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

// NilValidatorError is the validate error when context.EnableValidate is called but no validator is set in config.
type NilValidatorError struct {
}

// Format errors
var (
	// ErrNoHandlers is returned when c.Format is called with no arguments.
	ErrNoHandlers = errors.New("format: at least one handler is required, but none were set")
)

func (n NilValidatorError) Error() string {
	return "fiber: ctx.EnableValidate(v any) is called without validator"
}

// InvalidBinderError is the error when try to bind invalid value.
type InvalidBinderError struct {
	Type reflect.Type
}

func (e *InvalidBinderError) Error() string {
	if e.Type == nil {
		return "fiber: Bind(nil)"
	}

	if e.Type.Kind() != reflect.Pointer {
		return "fiber: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "fiber: Bind(nil " + e.Type.String() + ")"
}

// UnsupportedBinderError is the error when try to bind unsupported type.
type UnsupportedBinderError struct {
	Type reflect.Type
}

func (e *UnsupportedBinderError) Error() string {
	return "unsupported binder: ctx.Bind().Req(" + e.Type.String() + "), only binding struct is supported new"
}
