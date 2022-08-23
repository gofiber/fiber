package fiber

import (
	"errors"
	"reflect"
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

// InvalidBinderError is the error when try to bind unsupported type.
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
