package fiber

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/fieldname"
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

func TestNewTestCtx_ReturnsDefaultCtx(t *testing.T) {
	t.Parallel()
	app, ctx := newTestCtx(t)
	require.NotNil(t, app)
	require.NotNil(t, ctx)
	require.Equal(t, app, ctx.App())
}

// newTestCtx returns a new app and a *DefaultCtx borrowed from it until the test ends.
func newTestCtx(t *testing.T) (*App, *DefaultCtx) {
	t.Helper()

	app := New()
	return app, apptest.CtxAs[*DefaultCtx](t, app)
}

// TestToFiberHandler_Express runs every Express-style handler shape in front
// of a downstream handler that returns nextErr, and checks the error the
// converted handler returns, whether the downstream handler ran and, for the
// shapes without a next callback, the response.
func TestToFiberHandler_Express(t *testing.T) {
	t.Parallel()
	errNext := errors.New("next error")
	tests := []struct {
		build    func(t require.TestingT, bound func(Req, Res)) any // the handler under test, which calls bound
		check    func(t require.TestingT, ctx *DefaultCtx)
		nextErr  error // what the downstream handler returns
		name     string
		wantErr  string // the error the converted handler returns, none when empty
		isNext   bool   // the error is nextErr itself
		wantNext bool   // the downstream handler ran
	}{
		{
			name: "TwoParamsWithError",
			build: func(_ require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res) error {
					bound(req, res)
					return res.SendString("express")
				}
			},
			check: func(t require.TestingT, ctx *DefaultCtx) {
				require.Equal(t, "express", string(ctx.Response().Body()))
			},
		},
		{
			name: "TwoParamsWithoutError",
			build: func(t require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res) {
					bound(req, res)
					require.NoError(t, res.SendStatus(http.StatusCreated))
				}
			},
			check: func(t require.TestingT, ctx *DefaultCtx) {
				require.Equal(t, http.StatusCreated, ctx.Response().StatusCode())
			},
		},
		{
			name: "ThreeParamsWithError",
			build: func(_ require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, next func() error) error {
					bound(req, res)
					return next()
				}
			},
			nextErr: errors.New("next"), wantErr: "next", isNext: true, wantNext: true,
		},
		{
			name: "ThreeParamsWithError_PrefersHandlerErrorOverNextError",
			build: func(t require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, next func() error) error {
					bound(req, res)
					require.EqualError(t, next(), "next error")
					return errors.New("handler error")
				}
			},
			nextErr: errNext, wantErr: "handler error", wantNext: true,
		},
		{
			name: "ThreeParamsWithError_PropagatesNextErrorWhenNoReturnError",
			build: func(t require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, next func() error) error {
					bound(req, res)
					require.ErrorIs(t, next(), errNext)
					return nil
				}
			},
			nextErr: errNext, wantErr: "next error", isNext: true, wantNext: true,
		},
		{
			name: "ThreeParamsWithoutError",
			build: func(t require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, next func() error) {
					bound(req, res)
					err := next()
					require.Error(t, err)
					assert.EqualError(t, err, "next without error")
				}
			},
			nextErr: errors.New("next without error"), wantErr: "next without error", wantNext: true,
		},
		{
			name: "NextNoArgWithErrorReturn",
			build: func(_ require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, next func()) error {
					bound(req, res)
					next()
					return nil
				}
			},
			nextErr: errors.New("next without return value"), wantErr: "next without return value",
			isNext: true, wantNext: true,
		},
		{
			name: "NextNoArgPropagatesError",
			build: func(_ require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, next func()) {
					bound(req, res)
					next()
				}
			},
			nextErr: errors.New("next without return value"), wantErr: "next without return value",
			isNext: true, wantNext: true,
		},
		{
			name: "NextNoArgStopsChain",
			build: func(_ require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, _ func()) {
					bound(req, res)
					// Intentionally do not call next().
				}
			},
			nextErr: errors.New("should not be called"),
		},
		{
			name: "NextWithErrorContinuesOnNil",
			build: func(_ require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, next func(error)) {
					bound(req, res)
					next(nil)
				}
			},
			wantNext: true,
		},
		{
			name: "NextWithErrorShortCircuitsOnError",
			build: func(_ require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, next func(error)) {
					bound(req, res)
					next(errors.New("next error"))
				}
			},
			wantErr: "next error",
		},
		{
			name: "NextWithErrorReturn_ShortCircuitsOnNextError",
			build: func(_ require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, next func(error)) error {
					bound(req, res)
					next(errors.New("next error"))
					return nil
				}
			},
			wantErr: "next error",
		},
		{
			name: "NextWithErrorReturnCallback_PropagatesNextError",
			build: func(t require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, next func(error) error) {
					bound(req, res)
					require.EqualError(t, next(nil), "next error")
				}
			},
			nextErr: errNext, wantErr: "next error", isNext: true, wantNext: true,
		},
		{
			name: "NextWithErrorReturnCallback_ShortCircuitsOnNextError",
			build: func(t require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, next func(error) error) {
					bound(req, res)
					require.EqualError(t, next(errors.New("next error")), "next error")
				}
			},
			wantErr: "next error",
		},
		{
			name: "NextWithErrorReturnCallback_StopsChainWithoutNextCall",
			build: func(_ require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, _ func(error) error) {
					bound(req, res)
					// Intentionally do not call next.
				}
			},
			nextErr: errors.New("should not be called"),
		},
		{
			name: "NextWithErrorReturn_PrefersHandlerErrorOverNextError",
			build: func(t require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, next func(error) error) error {
					bound(req, res)
					require.EqualError(t, next(errors.New("next error")), "next error")
					return errors.New("handler error")
				}
			},
			wantErr: "handler error",
		},
		{
			name: "NextWithErrorReturn_PropagatesNextErrorWhenNoReturnError",
			build: func(_ require.TestingT, bound func(Req, Res)) any {
				return func(req Req, res Res, next func(error) error) error {
					bound(req, res)
					return next(errors.New("next error"))
				}
			},
			wantErr: "next error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, ctx := newTestCtx(t)
			bound := func(req Req, res Res) { // the Req and Res the handler receives belong to app
				assert.Equal(t, app, req.App())
				assert.Equal(t, app, res.App())
			}
			converted, ok := toFiberHandler(tc.build(t, bound))
			require.True(t, ok)

			nextCalled := false
			ctx.route = &Route{Handlers: []Handler{converted, func(Ctx) error {
				nextCalled = true
				return tc.nextErr
			}}}

			err := converted(ctx)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantErr)
			}
			if tc.isNext {
				require.ErrorIs(t, err, tc.nextErr)
			}
			require.Equal(t, tc.wantNext, nextCalled)
			if tc.check != nil {
				tc.check(t, ctx)
			}
		})
	}
}

func TestAdapter_MixedHandlerIntegration(t *testing.T) {
	app := New()

	app.Use(func(c Ctx) error {
		c.Set("X-Middleware", "fiber")
		return c.Next()
	})

	app.Use(func(_ Req, res Res, next func() error) error {
		res.Set("X-Express", "middleware")
		return next()
	})

	app.Get("/fiber", func(c Ctx) error {
		c.Set("X-Route", "fiber")
		return c.SendString("fiber handler")
	})

	app.Post("/express", func(_ Req, res Res) error {
		res.Set("X-Route", "express")
		return res.SendString("express handler")
	})

	var httpHandlerWriteErr error
	app.Put("/http", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Route", "http")
		w.WriteHeader(http.StatusAccepted)
		_, httpHandlerWriteErr = w.Write([]byte("http handler"))
	})

	app.Delete("/fasthttp", func(ctx *fasthttp.RequestCtx) error {
		ctx.Response.Header.Set("X-Route", "fasthttp")
		ctx.SetStatusCode(http.StatusCreated)
		ctx.SetBodyString("fasthttp handler")
		return nil
	})

	tests := []struct {
		name       string
		method     string
		path       string
		wantBody   string
		wantRoute  string
		wantStatus int
	}{
		{name: "fiber", method: http.MethodGet, path: "/fiber", wantStatus: http.StatusOK, wantBody: "fiber handler", wantRoute: "fiber"},
		{name: "express", method: http.MethodPost, path: "/express", wantStatus: http.StatusOK, wantBody: "express handler", wantRoute: "express"},
		{name: "net/http", method: http.MethodPut, path: "/http", wantStatus: http.StatusAccepted, wantBody: "http handler", wantRoute: "http"},
		{name: "fasthttp", method: http.MethodDelete, path: "/fasthttp", wantStatus: http.StatusCreated, wantBody: "fasthttp handler", wantRoute: "fasthttp"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := apptest.Do(t, app, apptest.Req(tc.method, tc.path))
			t.Cleanup(func() { require.NoError(t, resp.Body.Close()) })

			body := apptest.Body(t, resp)

			require.Equal(t, tc.wantStatus, resp.StatusCode)
			require.Equal(t, tc.wantBody, body)
			require.Equal(t, "fiber", resp.Header.Get("X-Middleware"))
			require.Equal(t, "middleware", resp.Header.Get("X-Express"))
			require.Equal(t, tc.wantRoute, resp.Header.Get("X-Route"))
		})
	}

	require.NoError(t, httpHandlerWriteErr)
}

func TestToFiberHandler_ExpressNextNoArgMiddleware(t *testing.T) {
	t.Parallel()
	app := New()
	t.Cleanup(func() { require.NoError(t, app.Shutdown()) })

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

	resp := apptest.Expect(t, app, "/", http.StatusOK)
	require.NoError(t, resp.Body.Close())

	require.Equal(t, []string{"middleware", "handler"}, callOrder)
}

// adapterConvertAndRun converts handler with toFiberHandler and runs it on a new context, which it returns.
func adapterConvertAndRun(t *testing.T, handler any) Ctx {
	t.Helper()

	converted, ok := toFiberHandler(handler)
	require.True(t, ok)
	require.NotNil(t, converted)

	ctx := apptest.Ctx(t, New())
	require.NoError(t, converted(ctx))
	return ctx
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

	ctx := apptest.Ctx(t, New())

	require.NoError(t, converted(ctx))
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

	ctx := adapterConvertAndRun(t, handler)

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

	require.EqualError(t, converted(ctx), "fasthttp error")
	require.Equal(t, "error", string(ctx.Response().Header.Peek("X-FASTHTTP")))
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

	ctx := adapterConvertAndRun(t, handler)

	require.Equal(t, "handler", string(ctx.Response().Header.Peek("X-HTTP")))
	require.Equal(t, "through", string(ctx.Response().Body()))
}

func TestWrapHTTPHandler_Flush_App_Test(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("w does not implement http.Flusher")
		}
		w.WriteHeader(StatusOK)
		fmt.Fprintf(w, "Hello ")
		flusher.Flush()
		fmt.Fprintf(w, "World!")
	})

	resp := apptest.Do(t, app, "/")
	defer resp.Body.Close() //nolint:errcheck // not needed

	require.Equal(t, StatusOK, resp.StatusCode)

	require.Equal(t, "Hello World!", apptest.Body(t, resp))
}

func Test_HTTPHandler_App_Test_Interrupted(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("w does not implement http.Flusher")
		}
		w.WriteHeader(StatusOK)
		fmt.Fprintf(w, "Hello ")
		flusher.Flush()
		time.Sleep(500 * time.Millisecond)
		fmt.Fprintf(w, "World!")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody), TestConfig{
		Timeout:       200 * time.Millisecond,
		FailOnTimeout: true, // Changed to true to test interrupted behavior
	})
	// With FailOnTimeout: true, we should get a timeout error
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)
	require.Nil(t, resp)
}

func TestToFiberHandler_HTTPHandlerFunc(t *testing.T) {
	t.Parallel()
	httpFunc := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }

	ctx := adapterConvertAndRun(t, httpFunc)

	require.Equal(t, http.StatusNoContent, ctx.Response().StatusCode())
}

func TestWrapHTTPHandler_Nil(t *testing.T) {
	t.Parallel()
	require.Nil(t, wrapHTTPHandler(nil))
}

// fasthttpadaptor buffers the body for the http.Request, and with StreamRequestBody the server passes on a body of any size.
func TestWrapHTTPHandler_StreamedBodyLimit(t *testing.T) {
	t.Parallel()

	const limit = 1024
	app := New(Config{StreamRequestBody: true, BodyLimit: limit})
	app.Post("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, err := io.Copy(io.Discard, r.Body)
		assert.NoError(t, err)
		_, err = w.Write([]byte(strconv.FormatInt(n, 10)))
		assert.NoError(t, err)
	}))

	for _, chunked := range []bool{false, true} {
		for size, status := range map[int]int{limit: StatusOK, limit + 1: StatusRequestEntityTooLarge} {
			req := httptest.NewRequest(MethodPost, "/", strings.NewReader(strings.Repeat("a", size)))
			if chunked {
				req.TransferEncoding = []string{"chunked"}
			}
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, status, resp.StatusCode, "chunked=%v size=%d", chunked, size)
			if status == StatusOK {
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Equal(t, strconv.Itoa(size), string(body), "chunked=%v", chunked)
			}
		}
	}

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().SetBodyStream(iotest.ErrReader(errors.New("broken chunk")), -1)
	require.ErrorIs(t, wrapHTTPHandler(http.NotFoundHandler())(c), ErrBadRequest)
	c.Request().SetBodyStream(iotest.ErrReader(ErrRequestHeaderFieldsTooLarge), -1)
	require.ErrorIs(t, wrapHTTPHandler(http.NotFoundHandler())(c), ErrRequestHeaderFieldsTooLarge)
}

func TestWrapHTTPHandler_StreamedTrailerCap(t *testing.T) {
	t.Parallel()

	app := New(Config{StreamRequestBody: true})
	app.Post("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(w, r.Body)
		assert.NoError(t, err)
	}))
	ln := startRawServer(t, app)

	var trailers strings.Builder
	for i := range fieldname.MaxTrailers + 1 {
		fmt.Fprintf(&trailers, "X-%d: v\r\n", i)
	}
	raw := "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n5\r\nhello\r\n0\r\n" +
		trailers.String() + "\r\n"
	responses, _ := rawExchange(t, ln, raw, 1)
	require.Equal(t, StatusRequestHeaderFieldsTooLarge, responses[0].status)
}

func TestCollectHandlers_InvalidHandler(t *testing.T) {
	t.Parallel()
	var (
		handlerFunc           http.HandlerFunc
		handler               http.Handler
		raw                   func(http.ResponseWriter, *http.Request)
		requestHandler        fasthttp.RequestHandler
		requestHandlerWithErr func(*fasthttp.RequestCtx) error
	)

	tests := []struct {
		handler any
		name    string
	}{
		{name: "HandlerFunc", handler: handlerFunc},
		{name: "Handler", handler: handler},
		{name: "Function", handler: raw},
		{name: "FuncHTTPHandler", handler: dummyFuncHandler(nil)},
		{name: "RequestHandler", handler: requestHandler},
		{name: "RequestHandlerWithError", handler: requestHandlerWithErr},
		{name: "unsupported type", handler: 42},
		{name: "typed nil pointer http.Handler", handler: http.Handler((*dummyHandler)(nil))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expected := fmt.Sprintf("context: invalid handler #0 (%T)\n", tt.handler)

			require.PanicsWithValue(t, expected, func() { collectHandlers("context", tt.handler) })
		})
	}
}

type dummyHandler struct{}

func (dummyHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

type dummyFuncHandler func(http.ResponseWriter, *http.Request)

func (handler dummyFuncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler == nil {
		return
	}
	handler(w, r)
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

	ctx := apptest.Ctx(t, New())

	for _, handler := range handlers {
		require.NoError(t, handler(ctx))
	}

	require.Equal(t, "fiber", string(ctx.Response().Header.Peek("X-Before")))
	require.Equal(t, "ok", string(ctx.Response().Header.Peek("X-FASTHTTP")))
	require.Equal(t, "done", string(ctx.Response().Body()))
}

func TestCollectHandlers_FiberHandlerNoErrorReturn(t *testing.T) {
	t.Parallel()
	noError := func(c Ctx) { c.Set("X-Handler", "fiber") }

	handlers := collectHandlers("ctx", noError)
	require.Len(t, handlers, 1)

	ctx := apptest.Ctx(t, New())

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

	ctx := apptest.Ctx(t, New())

	require.NoError(t, handlers[0](ctx))
	require.NoError(t, handlers[1](ctx))
	require.Equal(t, "done", string(ctx.Response().Body()))
	require.Equal(t, "fiber", string(ctx.Response().Header.Peek("X-Before")))
	require.NoError(t, writeErr)
}

func TestCollectHandlers_Nil(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "nil: invalid handler #0 (<nil>)\n", func() { collectHandlers("nil", nil) })
}

// serveRawTarget runs app on a request whose request line carries target as is.
func serveRawTarget(t *testing.T, app *App, target string) *fasthttp.Response {
	t.Helper()

	var req fasthttp.Request
	raw := MethodGet + " " + target + " HTTP/1.1\r\nHost: example.com\r\n\r\n"
	require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))
	var fctx fasthttp.RequestCtx
	fctx.Init(&req, nil, nil)
	app.Handler()(&fctx)
	return &fctx.Response
}

func Test_HTTPHandler_Direct_ServesRoutedPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "admin"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pub"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admin", "secret.txt"), []byte("secret"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pub", "ok.txt"), []byte("ok"), 0o600))

	fileServer := http.FileServer(http.Dir(dir))
	shapes := map[string]any{
		"http.Handler":     fileServer,
		"http.HandlerFunc": http.HandlerFunc(fileServer.ServeHTTP),
		"http func":        fileServer.ServeHTTP,
	}
	mounts := map[string]func(app *App, h any){
		"Use":     func(app *App, h any) { app.Use(h) },
		"Get /*":  func(app *App, h any) { app.Get("/*", h) },
		"Group /": func(app *App, h any) { app.Group("/", h) },
	}
	// target -> status without and with UnescapePath; under UnescapePath the router decodes %2F itself
	respelled := map[string][2]int{
		"/admin%2Fsecret.txt":              {StatusNotFound, StatusUnauthorized},
		"/admin%2F":                        {StatusNotFound, StatusUnauthorized},
		"/pub/..%2Fadmin/secret.txt":       {StatusNotFound, StatusUnauthorized},
		"/pub/..%2fadmin%2fsecret.txt":     {StatusNotFound, StatusUnauthorized},
		"//admin/secret.txt":               {StatusNotFound, StatusNotFound},
		"/%2Fadmin/secret.txt":             {StatusNotFound, StatusNotFound},
		"/pub//../admin/secret.txt":        {StatusNotFound, StatusNotFound},
		`/pub/..\admin/secret.txt`:         {StatusNotFound, StatusNotFound},
		"/pub/%%32e%%32e/admin/secret.txt": {StatusNotFound, StatusNotFound},
	}

	for shape, handler := range shapes {
		for mount, register := range mounts {
			for i, unescape := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/unescape=%v", shape, mount, unescape), func(t *testing.T) {
					t.Parallel()

					app := New(Config{UnescapePath: unescape})
					app.Use("/admin", func(c Ctx) error { return c.SendStatus(StatusUnauthorized) })
					register(app, handler)

					for _, target := range []string{
						"/pub/ok.txt", "/pub/ok.txt?a=%2F", "/pub/./ok.txt", "/%70ub/ok.txt", "/x/../pub/ok.txt",
						"http://example.org/pub/ok.txt",
					} {
						resp := serveRawTarget(t, app, target)
						require.Equal(t, StatusOK, resp.StatusCode(), target)
						require.Equal(t, "ok", string(resp.Body()), target)
					}
					for _, target := range []string{"/admin/secret.txt", "/pub/../admin/secret.txt", "/%61dmin/secret.txt"} {
						require.Equal(t, StatusUnauthorized, serveRawTarget(t, app, target).StatusCode(), target)
					}
					for target, want := range respelled {
						resp := serveRawTarget(t, app, target)
						require.Equal(t, want[i], resp.StatusCode(), target)
						require.NotContains(t, string(resp.Body()), "secret", target)
					}
				})
			}
		}
	}
}

func Test_HTTPHandler_Direct_RestoresRequestLine(t *testing.T) {
	t.Parallel()

	const target = "/a/../%70ub/x%20y?q=%2F"

	for name, handler := range map[string]any{
		"returns": func(w http.ResponseWriter, r *http.Request) {
			_, err := io.WriteString(w, r.RequestURI+" "+r.URL.Path)
			assert.NoError(t, err)
		},
		"panics": func(http.ResponseWriter, *http.Request) {
			panic("direct handler")
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var after string
			app := New()
			app.Use(func(c Ctx) error {
				defer func() {
					after = c.OriginalURL()
					if recover() != nil {
						c.Status(StatusTeapot)
					}
				}()
				return c.Next()
			})
			app.Get("/pub/*", handler)

			resp := serveRawTarget(t, app, target)
			require.Equal(t, target, after)
			if name == "panics" {
				require.Equal(t, StatusTeapot, resp.StatusCode())
				return
			}
			require.Equal(t, StatusOK, resp.StatusCode())
			require.Equal(t, "/pub/x%20y?q=%2F /pub/x y", string(resp.Body()))
		})
	}
}
