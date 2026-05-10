package contextvalue

import (
	"context"

	"github.com/valyala/fasthttp"
)

type fiberLocalContext interface {
	Locals(key any, value ...any) any
}

type userValueContext interface {
	UserValue(key any) any
}

type valueContext interface {
	Value(key any) any
}

// Value retrieves a value stored under key from supported Fiber context types.
func Value[T any](ctx, key any) (T, bool) {
	switch typed := ctx.(type) {
	case *fasthttp.RequestCtx:
		val, ok := typed.UserValue(key).(T)
		return val, ok
	case context.Context:
		val, ok := typed.Value(key).(T)
		return val, ok
	case valueContext:
		val, ok := typed.Value(key).(T)
		return val, ok
	case fiberLocalContext:
		val, ok := typed.Locals(key).(T)
		return val, ok
	case userValueContext:
		val, ok := typed.UserValue(key).(T)
		return val, ok
	default:
		var zero T
		return zero, false
	}
}
