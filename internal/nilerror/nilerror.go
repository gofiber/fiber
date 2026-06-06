package nilerror

import (
	"reflect"
)

// IsNil reports whether err is nil or contains a typed-nil value.
func IsNil(err error) bool {
	if err == nil {
		return true
	}

	v := reflect.ValueOf(err)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
