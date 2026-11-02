package apptest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// CtxPool is implemented by *fiber.App. C is the app's context type
// (fiber.CustomCtx) and is inferred from the app, never written out.
type CtxPool[C any] interface {
	AcquireCtx(fctx *fasthttp.RequestCtx) C
	ReleaseCtx(c C)
}

// Ctx acquires a context for a blank request from app and returns it to the
// pool when the test ends, after any parallel subtests have finished. That
// ordering is what a deferred ReleaseCtx cannot give a test whose subtests call
// t.Parallel, because the deferred call runs before they start.
//
// Call Ctx once per context: every call registers its own cleanup.
func Ctx[C any](tb testing.TB, app CtxPool[C]) C {
	tb.Helper()

	return CtxFor(tb, app, &fasthttp.RequestCtx{})
}

// CtxFor is [Ctx] for a context built around fctx, for tests that prepare the
// request on the fasthttp context first.
func CtxFor[C any](tb testing.TB, app CtxPool[C], fctx *fasthttp.RequestCtx) C {
	tb.Helper()

	c := app.AcquireCtx(fctx)
	tb.Cleanup(func() { app.ReleaseCtx(c) })
	return c
}

// CtxAs is [Ctx] for tests that need the concrete context type, such as
// *fiber.DefaultCtx, instead of the interface. The test fails if the app hands
// out a context of any other type. T is the only type argument to spell out:
//
//	c := apptest.CtxAs[*fiber.DefaultCtx](t, app)
func CtxAs[T, C any](tb testing.TB, app CtxPool[C]) T {
	tb.Helper()

	c := Ctx(tb, app)
	typed, ok := any(c).(T)
	require.True(tb, ok, "context is %T, want %T", c, typed)
	return typed
}
