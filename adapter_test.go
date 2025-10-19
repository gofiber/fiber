package fiber

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

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

func TestToFiberHandler_FiberHandlerNoErrorReturn(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	handler := func(c Ctx) {
		require.Equal(t, app, c.App())
		c.Set("X-Handler", "ok")
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)
	require.NotNil(t, converted)

	require.NoError(t, converted(ctx))
	require.Equal(t, "ok", string(ctx.Response().Header.Peek("X-Handler")))
}

func newTestCtx(t *testing.T) (*App, *DefaultCtx) {
	t.Helper()

	app := New()
	fasthttpCtx := &fasthttp.RequestCtx{}
	customCtx := app.AcquireCtx(fasthttpCtx)
	ctx, ok := customCtx.(*DefaultCtx)
	require.True(t, ok)

	t.Cleanup(func() {
		app.ReleaseCtx(customCtx)
	})

	return app, ctx
}

func TestToFiberHandler_ExpressTwoParamsWithError(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	handler := func(req Req, res Res) error {
		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
		return res.SendString("express")
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	require.NoError(t, converted(ctx))
	require.Equal(t, "express", string(ctx.Response().Body()))
}

func TestToFiberHandler_ExpressTwoParamsWithoutError(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	handler := func(req Req, res Res) {
		assert.Equal(t, app, req.App())
		require.NoError(t, res.SendStatus(http.StatusCreated))
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	require.NoError(t, converted(ctx))
	require.Equal(t, http.StatusCreated, ctx.Response().StatusCode())
}

func TestToFiberHandler_ExpressThreeParamsWithError(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	handler := func(req Req, res Res, next func() error) error {
		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
		return next()
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	nextErr := errors.New("next")
	nextCalled := false
	nextHandler := func(_ Ctx) error {
		nextCalled = true
		return nextErr
	}

	route := &Route{Handlers: []Handler{converted, nextHandler}}
	ctx.route = route
	ctx.indexHandler = 0
	t.Cleanup(func() {
		ctx.route = nil
		ctx.indexHandler = 0
	})

	err := converted(ctx)
	require.ErrorIs(t, err, nextErr)
	require.True(t, nextCalled)
}

func TestToFiberHandler_ExpressThreeParamsWithoutError(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	handler := func(req Req, _ Res, next func() error) {
		assert.Equal(t, app, req.App())
		err := next()
		require.Error(t, err)
		assert.EqualError(t, err, "next without error")
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	nextHandler := func(_ Ctx) error {
		return errors.New("next without error")
	}

	route := &Route{Handlers: []Handler{converted, nextHandler}}
	ctx.route = route
	ctx.indexHandler = 0
	t.Cleanup(func() {
		ctx.route = nil
		ctx.indexHandler = 0
	})

	err := converted(ctx)
	require.EqualError(t, err, "next without error")
}

func TestToFiberHandler_ExpressNextNoArgWithErrorReturn(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	handler := func(req Req, res Res, next func()) error {
		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
		next()
		return nil
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	nextErr := errors.New("next without return value")
	nextCalled := false
	nextHandler := func(_ Ctx) error {
		nextCalled = true
		return nextErr
	}

	route := &Route{Handlers: []Handler{converted, nextHandler}}
	ctx.route = route
	ctx.indexHandler = 0
	t.Cleanup(func() {
		ctx.route = nil
		ctx.indexHandler = 0
	})

	err := converted(ctx)
	require.ErrorIs(t, err, nextErr)
	require.True(t, nextCalled)
}

func TestToFiberHandler_ExpressNextNoArgPropagatesError(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	handler := func(req Req, res Res, next func()) {
		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
		next()
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	nextErr := errors.New("next without return value")
	nextCalled := false
	nextHandler := func(_ Ctx) error {
		nextCalled = true
		return nextErr
	}

	route := &Route{Handlers: []Handler{converted, nextHandler}}
	ctx.route = route
	ctx.indexHandler = 0
	t.Cleanup(func() {
		ctx.route = nil
		ctx.indexHandler = 0
	})

	err := converted(ctx)
	require.ErrorIs(t, err, nextErr)
	require.True(t, nextCalled)
}

func TestToFiberHandler_ExpressNextNoArgStopsChain(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	handler := func(req Req, res Res, _ func()) {
		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
		// Intentionally do not call next().
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	nextCalled := false
	nextHandler := func(_ Ctx) error {
		nextCalled = true
		return errors.New("should not be called")
	}

	route := &Route{Handlers: []Handler{converted, nextHandler}}
	ctx.route = route
	ctx.indexHandler = 0
	t.Cleanup(func() {
		ctx.route = nil
		ctx.indexHandler = 0
	})

	err := converted(ctx)
	require.NoError(t, err)
	require.False(t, nextCalled)
}

func TestToFiberHandler_ExpressNextNoArgMiddleware(t *testing.T) {
	t.Parallel()

	app := New()
	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
	})

	callOrder := make([]string, 0, 2)

	app.Use(func(req Req, res Res, next func()) {
		callOrder = append(callOrder, "middleware")
		next()
		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
	})

	app.Get("/", func(c Ctx) error {
		callOrder = append(callOrder, "handler")
		return c.SendStatus(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	require.Equal(t, []string{"middleware", "handler"}, callOrder)
}

func TestToFiberHandler_ExpressErrorHandlerSkipsWhenNoError(t *testing.T) {
	t.Parallel()

	_, ctx := newTestCtx(t)

	handler := func(_ error, _ Req, _ Res) error {
		t.Fatalf("error handler should not run when downstream succeeds")
		return nil
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	nextHandler := func(_ Ctx) error {
		return nil
	}

	ctx.route = &Route{Handlers: []Handler{converted, nextHandler}}
	ctx.indexHandler = 0
	t.Cleanup(func() {
		ctx.route = nil
		ctx.indexHandler = 0
	})

	require.NoError(t, converted(ctx))
}

func TestToFiberHandler_ExpressErrorHandlerHandlesError(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	boom := errors.New("boom")
	handler := func(err error, req Req, res Res) error {
		assert.Equal(t, boom, err)
		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
		return res.Status(http.StatusTeapot).SendString("handled")
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	nextHandler := func(_ Ctx) error {
		return boom
	}

	ctx.route = &Route{Handlers: []Handler{converted, nextHandler}}
	ctx.indexHandler = 0
	t.Cleanup(func() {
		ctx.route = nil
		ctx.indexHandler = 0
	})

	err := converted(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusTeapot, ctx.Response().StatusCode())
	require.Equal(t, "handled", string(ctx.Response().Body()))
}

func TestToFiberHandler_ExpressErrorHandlerNoReturnHandlesError(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	boom := errors.New("boom")
	handler := func(err error, req Req, res Res) {
		assert.Equal(t, boom, err)
		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
		require.NoError(t, res.Status(http.StatusGatewayTimeout).SendString("handled"))
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	nextHandler := func(_ Ctx) error {
		return boom
	}

	ctx.route = &Route{Handlers: []Handler{converted, nextHandler}}
	ctx.indexHandler = 0
	t.Cleanup(func() {
		ctx.route = nil
		ctx.indexHandler = 0
	})

	err := converted(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusGatewayTimeout, ctx.Response().StatusCode())
	require.Equal(t, "handled", string(ctx.Response().Body()))
}

func TestToFiberHandler_ExpressErrorHandlerNextPropagates(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	boom := errors.New("boom")
	handler := func(err error, req Req, res Res, next func() error) error {
		assert.Equal(t, boom, err)
		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
		return next()
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	nextCalled := false
	nextHandler := func(_ Ctx) error {
		nextCalled = true
		return boom
	}

	ctx.route = &Route{Handlers: []Handler{converted, nextHandler}}
	ctx.indexHandler = 0
	t.Cleanup(func() {
		ctx.route = nil
		ctx.indexHandler = 0
	})

	err := converted(ctx)
	require.ErrorIs(t, err, boom)
	require.True(t, nextCalled)
}

func TestToFiberHandler_ExpressErrorHandlerNextCallbackPropagates(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	boom := errors.New("boom")
	handler := func(err error, req Req, res Res, next func() error) {
		assert.Equal(t, boom, err)
		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
		require.NoError(t, res.SendStatus(http.StatusAccepted))
		nextErr := next()
		require.ErrorIs(t, nextErr, boom)
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	nextHandler := func(_ Ctx) error {
		return boom
	}

	ctx.route = &Route{Handlers: []Handler{converted, nextHandler}}
	ctx.indexHandler = 0
	t.Cleanup(func() {
		ctx.route = nil
		ctx.indexHandler = 0
	})

	err := converted(ctx)
	require.ErrorIs(t, err, boom)
}

func TestToFiberHandler_ExpressErrorHandlerNoReturnControlsPropagation(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	boom := errors.New("boom")
	handler := func(err error, req Req, res Res, _ func()) {
		assert.Equal(t, boom, err)
		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
		// Do not call next() so the error is swallowed.
		require.NoError(t, res.SendStatus(http.StatusAccepted))
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	nextHandler := func(_ Ctx) error {
		return boom
	}

	ctx.route = &Route{Handlers: []Handler{converted, nextHandler}}
	ctx.indexHandler = 0
	t.Cleanup(func() {
		ctx.route = nil
		ctx.indexHandler = 0
	})

	err := converted(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, ctx.Response().StatusCode())
}

func TestToFiberHandler_ExpressErrorHandlerNoReturnNextPropagates(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	boom := errors.New("boom")
	handler := func(err error, req Req, res Res, next func()) {
		assert.Equal(t, boom, err)
		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
		next()
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	nextHandler := func(_ Ctx) error {
		return boom
	}

	ctx.route = &Route{Handlers: []Handler{converted, nextHandler}}
	ctx.indexHandler = 0
	t.Cleanup(func() {
		ctx.route = nil
		ctx.indexHandler = 0
	})

	err := converted(ctx)
	require.ErrorIs(t, err, boom)
}

func TestToFiberHandler_ExpressErrorHandlerNoArgNextWithErrorReturn(t *testing.T) {
	t.Parallel()

	app, ctx := newTestCtx(t)

	boom := errors.New("boom")
	handler := func(err error, req Req, res Res, next func()) error {
		assert.Equal(t, boom, err)
		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
		next()
		return nil
	}

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)

	nextHandler := func(_ Ctx) error {
		return boom
	}

	ctx.route = &Route{Handlers: []Handler{converted, nextHandler}}
	ctx.indexHandler = 0
	t.Cleanup(func() {
		ctx.route = nil
		ctx.indexHandler = 0
	})

	err := converted(ctx)
	require.ErrorIs(t, err, boom)
}

func TestToFiberHandler_ExpressErrorHandlerMiddlewareIntegration(t *testing.T) {
	t.Parallel()

	app := New()
	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
	})

	app.Use(func(err error, req Req, res Res, _ func() error) error {
		if err == nil {
			return nil
		}

		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
		return res.Status(http.StatusBadGateway).SendString(err.Error())
	})

	boom := errors.New("boom")
	app.Get("/", func(_ Ctx) error {
		return boom
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadGateway, resp.StatusCode)
	body, readErr := io.ReadAll(resp.Body)
	require.NoError(t, readErr)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, "boom", string(body))
}

func TestToFiberHandler_ExpressErrorHandlerRouteIntegration(t *testing.T) {
	t.Parallel()

	app := New()
	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
	})

	app.Get("/", func(err error, req Req, res Res) error {
		if err == nil {
			return nil
		}

		assert.Equal(t, app, req.App())
		assert.Equal(t, app, res.App())
		return res.Status(http.StatusServiceUnavailable).SendString("handled:" + err.Error())
	}, func(_ Ctx) error {
		return errors.New("route failure")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	body, readErr := io.ReadAll(resp.Body)
	require.NoError(t, readErr)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, "handled:route failure", string(body))
}

func TestCollectHandlers_HTTPHandler(t *testing.T) {
	t.Parallel()

	var writeErr error
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-HTTP", "ok")
		w.WriteHeader(http.StatusTeapot)
		_, writeErr = w.Write([]byte("http"))
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
	require.NoError(t, writeErr)
}

func TestToFiberHandler_HTTPHandler(t *testing.T) {
	t.Parallel()

	var writeErr error
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-HTTP", "handler")
		_, writeErr = w.Write([]byte("through"))
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
	require.NoError(t, writeErr)
}

func TestToFiberHandler_FasthttpHandlerWithError(t *testing.T) {
	t.Parallel()

	_, ctx := newTestCtx(t)

	fasthttpHandler := func(fctx *fasthttp.RequestCtx) error {
		fctx.Response.Header.Set("X-FASTHTTP", "error")
		return errors.New("fasthttp error")
	}

	converted, ok := toFiberHandler(fasthttpHandler)
	require.True(t, ok)
	require.NotNil(t, converted)

	err := converted(ctx)
	require.EqualError(t, err, "fasthttp error")
	require.Equal(t, "error", string(ctx.Response().Header.Peek("X-FASTHTTP")))
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

func TestCollectHandlers_TypedNilFasthttpHandlers(t *testing.T) {
	t.Parallel()

	var requestHandler fasthttp.RequestHandler
	var requestHandlerWithError func(*fasthttp.RequestCtx) error

	tests := []struct {
		handler any
		name    string
	}{
		{
			name:    "RequestHandler",
			handler: requestHandler,
		},
		{
			name:    "RequestHandlerWithError",
			handler: requestHandlerWithError,
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

func TestCollectHandlers_FiberHandlerNoErrorReturn(t *testing.T) {
	t.Parallel()

	noError := func(c Ctx) {
		c.Set("X-Handler", "fiber")
	}

	handlers := collectHandlers("ctx", noError)
	require.Len(t, handlers, 1)

	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() {
		app.ReleaseCtx(ctx)
	})

	require.NoError(t, handlers[0](ctx))
	require.Equal(t, "fiber", string(ctx.Response().Header.Peek("X-Handler")))
}

func TestCollectHandlers_MixedHandlers(t *testing.T) {
	t.Parallel()

	before := func(c Ctx) error {
		c.Set("X-Before", "fiber")
		return nil
	}
	var writeErr error
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, writeErr = w.Write([]byte("done"))
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
	require.NoError(t, writeErr)
}

func TestCollectHandlers_Nil(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "nil: invalid handler #0 (<nil>)\n", func() {
		collectHandlers("nil", nil)
	})
}
