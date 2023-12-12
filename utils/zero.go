package utils

import (
	"reflect"
)

// IsZeroValue reports whether x is the zero value for its type.
//
// For basic types, error and slices of basic types, it uses a fast path
// that does not use reflection.
//
// For all other types, it uses reflection.
func IsZeroValue(x interface{}) bool {
	// Fast path for basic types
	switch v := x.(type) {
	case nil:
		return true
	case bool:
		return !v
	case int:
		return v == 0
	case int8:
		return v == 0
	case int16:
		return v == 0
	case int32:
		return v == 0
	case int64:
		return v == 0
	case uint:
		return v == 0
	case uint8:
		return v == 0
	case uint16:
		return v == 0
	case uint32:
		return v == 0
	case uint64:
		return v == 0
	case uintptr:
		return v == 0
	case float32:
		return v == 0.0
	case float64:
		return v == 0.0
	case complex64:
		return v == 0+0i
	case complex128:
		return v == 0+0i
	case string:
		return v == ""
	case error:
		return v == nil
	default:
		// Slow path using reflection
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Ptr:
			if rv.IsNil() {
				return true
			}
			return reflectDeepEqual(rv.Elem().Interface())
		default:
			return reflectDeepEqual(v)
		}
	}
}

func reflectDeepEqual(x interface{}) bool {
	return reflect.DeepEqual(x, reflect.Zero(reflect.TypeOf(x)).Interface())
}
