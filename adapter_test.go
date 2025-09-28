package fiber

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestToFiberHandler_Nil(t *testing.T) {
	t.Parallel()

	handler, ok := toFiberHandler(nil)
	require.True(t, ok)
	require.Nil(t, handler)
}

func TestToFiberHandler_FiberHandler(t *testing.T) {
	t.Parallel()

	fiberHandler := func(c Ctx) error { return c.SendStatus(http.StatusAccepted) }

	converted, ok := toFiberHandler(fiberHandler)
	require.True(t, ok)
	require.NotNil(t, converted)
	require.Equal(t, reflect.ValueOf(fiberHandler).Pointer(), reflect.ValueOf(converted).Pointer())
}

func TestToFiberHandler_HTTPHandler(t *testing.T) {
	t.Parallel()

	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-HTTP", "ok")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("http"))
	})

	converted, ok := toFiberHandler(httpHandler)
	require.True(t, ok)
	require.NotNil(t, converted)

	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() {
		app.ReleaseCtx(ctx)
	})

	err := converted(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusTeapot, ctx.Response().StatusCode())
	require.Equal(t, "ok", string(ctx.Response().Header.Peek("X-HTTP")))
	require.Equal(t, "http", string(ctx.Response().Body()))
}

func TestToFiberHandler_HTTPHandlerFunc(t *testing.T) {
	t.Parallel()

	httpFunc := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}

	converted, ok := toFiberHandler(httpFunc)
	require.True(t, ok)
	require.NotNil(t, converted)

	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() {
		app.ReleaseCtx(ctx)
	})

	err := converted(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, ctx.Response().StatusCode())
}

func TestWrapHTTPHandler_Nil(t *testing.T) {
	t.Parallel()

	require.Nil(t, wrapHTTPHandler(nil))
}

func TestCollectHandlers_InvalidType(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "context: invalid handler int\n", func() {
		collectHandlers("context", 42)
	})
}

func TestCollectHandlers_MixedHandlers(t *testing.T) {
	t.Parallel()

	before := func(c Ctx) error {
		c.Set("X-Before", "fiber")
		return nil
	}
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("done"))
	})

	handlers := collectHandlers("test", before, httpHandler, nil)
	require.Len(t, handlers, 3)
	require.Equal(t, reflect.ValueOf(before).Pointer(), reflect.ValueOf(handlers[0]).Pointer())
	require.NotNil(t, handlers[1])
	require.Nil(t, handlers[2])

	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() {
		app.ReleaseCtx(ctx)
	})

	err := handlers[0](ctx)
	require.NoError(t, err)

	err = handlers[1](ctx)
	require.NoError(t, err)
	require.Equal(t, "done", string(ctx.Response().Body()))
	require.Equal(t, "fiber", string(ctx.Response().Header.Peek("X-Before")))
}
