package fiber

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestToFiberHandler_Nil(t *testing.T) {
	t.Parallel()

	var handler Handler
	converted, ok := toFiberHandler(handler)
	require.False(t, ok)
	require.Nil(t, converted)
}

func TestToFiberHandler_FiberHandler(t *testing.T) {
	t.Parallel()

	fiberHandler := func(c Ctx) error { return c.SendStatus(http.StatusAccepted) }

	converted, ok := toFiberHandler(fiberHandler)
	require.True(t, ok)
	require.NotNil(t, converted)
	require.Equal(t, reflect.ValueOf(fiberHandler).Pointer(), reflect.ValueOf(converted).Pointer())
}

func TestCollectHandlers_HTTPHandler(t *testing.T) {
	t.Parallel()

	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-HTTP", "ok")
		w.WriteHeader(http.StatusTeapot)
		_, err := w.Write([]byte("http"))
		assert.NoError(t, err)
	})

	handlers := collectHandlers("test", httpHandler)
	require.Len(t, handlers, 1)
	converted := handlers[0]
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

func TestToFiberHandler_HTTPHandler(t *testing.T) {
	t.Parallel()

	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-HTTP", "handler")
		_, err := w.Write([]byte("through"))
		assert.NoError(t, err)
	})

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)
	require.NotNil(t, converted)

	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() {
		app.ReleaseCtx(ctx)
	})

	err := converted(ctx)
	require.NoError(t, err)
	require.Equal(t, "handler", string(ctx.Response().Header.Peek("X-HTTP")))
	require.Equal(t, "through", string(ctx.Response().Body()))
}

func TestToFiberHandler_HTTPHandler_Flush(t *testing.T) {
	t.Parallel()

	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-HTTP", "handler")
		_, err := w.Write([]byte("through"))
		flusher, ok := w.(http.Flusher)
		assert.True(t, ok, "w does not implement http.Flusher")
		flusher.Flush()
		assert.NoError(t, err)
	})

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)
	require.NotNil(t, converted)

	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() {
		app.ReleaseCtx(ctx)
	})

	err := converted(ctx)
	require.NoError(t, err)
	require.Equal(t, "handler", string(ctx.Response().Header.Peek("X-HTTP")))
	require.Equal(t, "through", string(ctx.Response().Body()))
}

func TestWrapHTTPHandler_Flush_App_Test(t *testing.T) {
	app := New()

	app.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("w does not implement http.Flusher")
		}
		w.WriteHeader(StatusOK)
		fmt.Fprintf(w, "Hello ")
		flusher.Flush()
		time.Sleep(500 * time.Millisecond)
		fmt.Fprintf(w, "World!")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck // not needed

	require.Equal(t, StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Hello World!", string(body))
}

func Test_HTTPHandler_App_Test_Interrupted(t *testing.T) {
	app := New()

	app.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("w does not implement http.Flusher")
		}
		w.WriteHeader(StatusOK)
		fmt.Fprintf(w, "Hello ")
		flusher.Flush()
		time.Sleep(500 * time.Millisecond)
		fmt.Fprintf(w, "World!")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil), TestConfig{
		Timeout:       200 * time.Millisecond,
		FailOnTimeout: false,
	})
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck // not needed

	require.Equal(t, StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	require.Equal(t, "Hello ", string(body))
}

func TestToFiberHandler_HTTPHandlerFunc(t *testing.T) {
	t.Parallel()

	httpFunc := func(w http.ResponseWriter, _ *http.Request) {
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

	require.PanicsWithValue(t, "context: invalid handler #0 (int)\n", func() {
		collectHandlers("context", 42)
	})
}

func TestCollectHandlers_TypedNilHTTPHandlers(t *testing.T) {
	t.Parallel()

	var handlerFunc http.HandlerFunc
	var handler http.Handler
	var raw func(http.ResponseWriter, *http.Request)

	tests := []struct {
		handler any
		name    string
	}{
		{
			name:    "HandlerFunc",
			handler: handlerFunc,
		},
		{
			name:    "Handler",
			handler: handler,
		},
		{
			name:    "Function",
			handler: raw,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expected := fmt.Sprintf("context: invalid handler #0 (%T)\n", tt.handler)

			require.PanicsWithValue(t, expected, func() {
				collectHandlers("context", tt.handler)
			})
		})
	}
}

type dummyHandler struct{}

func (dummyHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func TestCollectHandlers_TypedNilPointerHTTPHandler(t *testing.T) {
	t.Parallel()

	var handler http.Handler = (*dummyHandler)(nil)

	require.PanicsWithValue(t, "context: invalid handler #0 (*fiber.dummyHandler)\n", func() {
		collectHandlers("context", handler)
	})
}

func TestCollectHandlers_FasthttpHandler(t *testing.T) {
	t.Parallel()

	before := func(c Ctx) error {
		c.Set("X-Before", "fiber")
		return nil
	}

	fasthttpHandler := fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.Set("X-FASTHTTP", "ok")
		ctx.SetBody([]byte("done"))
	})

	handlers := collectHandlers("fasthttp", before, fasthttpHandler)
	require.Len(t, handlers, 2)

	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() {
		app.ReleaseCtx(ctx)
	})

	for _, handler := range handlers {
		require.NoError(t, handler(ctx))
	}

	require.Equal(t, "fiber", string(ctx.Response().Header.Peek("X-Before")))
	require.Equal(t, "ok", string(ctx.Response().Header.Peek("X-FASTHTTP")))
	require.Equal(t, "done", string(ctx.Response().Body()))
}

func TestCollectHandlers_MixedHandlers(t *testing.T) {
	t.Parallel()

	before := func(c Ctx) error {
		c.Set("X-Before", "fiber")
		return nil
	}
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("done"))
		assert.NoError(t, err)
	})

	handlers := collectHandlers("test", before, httpHandler)
	require.Len(t, handlers, 2)
	require.Equal(t, reflect.ValueOf(before).Pointer(), reflect.ValueOf(handlers[0]).Pointer())
	require.NotNil(t, handlers[1])

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

func TestCollectHandlers_Nil(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "nil: invalid handler #0 (<nil>)\n", func() {
		collectHandlers("nil", nil)
	})
}
