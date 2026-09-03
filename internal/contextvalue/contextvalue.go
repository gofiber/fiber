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

// Value retrieves a value stored under key from supported context types
// (fiber.Ctx, fiber.CustomCtx, context.Context, and *fasthttp.RequestCtx).
func Value[T any](ctx, key any) (T, bool) {
	// Prefer Value-style lookups before Locals/UserValue when a context exposes
	// multiple accessors so Fiber contexts follow context.Value semantics.
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

// userContextKey is the type of UserContextKey. It is unexported so that no
// other package can produce a key that compares equal to it.
type userContextKey struct{}

// UserContextKey is the *fasthttp.RequestCtx user-value key under which Fiber
// keeps a request's user context: the context.Context that fiber.Ctx.Context
// returns and fiber.Ctx.SetContext replaces.
//
// It lives here rather than in the fiber package so that code holding only the
// RequestCtx, or a context.Context descending from it as adapted net/http
// handlers receive from fasthttpadaptor, can read the very value fiber wrote
// without a fiber.Ctx and without keeping a second copy of it per request.
var UserContextKey = userContextKey{}
