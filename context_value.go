package fiber

import (
	"context"

	"github.com/valyala/fasthttp"
)

type localsValueGetter interface {
	Locals(key any, value ...any) any
}

// ValueFromContext retrieves a value stored under key from supported context types.
//
// Supported context types:
//   - CustomCtx
//   - Ctx
//   - *fasthttp.RequestCtx
//   - context.Context
func ValueFromContext[T any](ctx, key any) (T, bool) {
	switch typed := ctx.(type) {
	case localsValueGetter:
		val, ok := typed.Locals(key).(T)
		return val, ok
	case *fasthttp.RequestCtx:
		val, ok := typed.UserValue(key).(T)
		return val, ok
	case context.Context:
		val, ok := typed.Value(key).(T)
		return val, ok
	default:
		var zero T
		return zero, false
	}
}
