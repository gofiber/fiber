package fiber

import (
	"errors"
	"reflect"
)

// Graceful shutdown errors
var (
	ErrGracefulTimeout = errors.New("shutdown: graceful timeout has been reached, exiting")
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
