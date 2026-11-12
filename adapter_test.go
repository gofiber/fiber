package fiber

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
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

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
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

// A body stream that fails halfway is a bad request, unless its error names a status of its own.
func TestWrapHTTPHandler_StreamedBodyReadError(t *testing.T) {
	t.Parallel()
	app := New(Config{StreamRequestBody: true, BodyLimit: 1024})

	for _, tc := range []struct {
		err  error
		want error
		name string
	}{
		{name: "broken chunk", err: errors.New("broken chunk"), want: ErrBadRequest},
		{name: "error with a status", err: ErrRequestHeaderFieldsTooLarge, want: ErrRequestHeaderFieldsTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)
			c.Request().SetBodyStream(iotest.ErrReader(tc.err), -1)
			require.ErrorIs(t, wrapHTTPHandler(http.NotFoundHandler())(c), tc.want)
		})
	}
}

func TestWrapHTTPHandler_StreamedTrailerCap(t *testing.T) {
	t.Parallel()
	app := New(Config{StreamRequestBody: true})
	app.Post("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(w, r.Body)
		assert.NoError(t, err)
	}))
	ln := startRawServer(t, app)

	raw := "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n5\r\nhello\r\n0\r\n" +
		apptest.FieldLines("X-", "v", fieldname.MaxTrailers+1) + "\r\n"
	rawExpectStatus(t, ln, raw, StatusRequestHeaderFieldsTooLarge)
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

	fctx := wireRequestCtx(t, MethodGet+" "+target+" HTTP/1.1\r\nHost: example.com\r\n\r\n", nil)
	app.Handler()(fctx)
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
	files := fasthttp.FSHandler(dir, 0)
	shapes := map[string]any{
		"http.Handler":                   fileServer,
		"http.HandlerFunc":               http.HandlerFunc(fileServer.ServeHTTP),
		"http func":                      fileServer.ServeHTTP,
		"fasthttp.RequestHandler":        files,
		"func(*fasthttp.RequestCtx) err": func(ctx *fasthttp.RequestCtx) error { files(ctx); return nil },
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
		"/pub%2F..%2Fadmin%2Fsecret.txt":   {StatusNotFound, StatusUnauthorized},
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

// Test_Adapter_FastHTTP_RestoresRequestLine pins that the request line comes back once a fasthttp route handler returned,
// unless the response is a body stream or hijacked: a net/http handler adapted by fasthttpadaptor goes on reading the line
// then (see Test_Adapter_FastHTTP_AdaptedHandlerKeepsItsRequest), so it is left as the handler read it, as for the
// net/http types.
func Test_Adapter_FastHTTP_RestoresRequestLine(t *testing.T) {
	t.Parallel()
	const target = "/a/../%70ub/x%20y?q=%2F"
	const routed = "/pub/x%20y?q=%2F"
	for name, tc := range map[string]struct {
		respond func(ctx *fasthttp.RequestCtx)
		want    string
	}{
		"buffered":    {respond: func(ctx *fasthttp.RequestCtx) { ctx.SetBodyString("ok") }, want: target},
		"body stream": {respond: func(ctx *fasthttp.RequestCtx) { ctx.SetBodyStream(strings.NewReader("ok"), 2) }, want: routed},
		"hijacked":    {respond: func(ctx *fasthttp.RequestCtx) { ctx.Hijack(func(net.Conn) {}) }, want: routed},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var read, after string
			app := New()
			app.Use(func(c Ctx) error {
				err := c.Next()
				after = c.OriginalURL()
				return err
			})
			app.Use(func(ctx *fasthttp.RequestCtx) error {
				read = string(ctx.Path()) + "?" + string(ctx.QueryArgs().QueryString())
				tc.respond(ctx)
				return nil
			})

			serveRawTarget(t, app, target)
			require.Equal(t, "/pub/x y?q=%2F", read, "the handler reads the routed path")
			require.Equal(t, tc.want, after)
		})
	}
}

// Test_Adapter_FastHTTP_AdaptedHandlerKeepsItsRequest pins that a net/http handler adapted by fasthttpadaptor and registered
// as a route handler, which flushes and goes on, reads the request line it was given until it returns: its URL and
// RequestURI alias the line, so writing the original line over it once the route handler returned changed what it reads.
func Test_Adapter_FastHTTP_AdaptedHandlerKeepsItsRequest(t *testing.T) {
	t.Parallel()
	proceed := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(proceed) }) }
	t.Cleanup(release)
	type reading struct{ before, after string }
	got := make(chan reading, 1)
	app := New()
	app.Get("/events", fasthttpadaptor.NewFastHTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		before := r.URL.Path + "|" + r.RequestURI
		_, err := io.WriteString(w, "first")
		assert.NoError(t, err)
		assert.NoError(t, http.NewResponseController(w).Flush())
		<-proceed
		got <- reading{before: before, after: r.URL.Path + "|" + r.RequestURI}
	})))

	serveRawTarget(t, app, "/a/../events?x=1")
	release()
	require.Equal(t, reading{before: "/events|/events?x=1", after: "/events|/events?x=1"}, apptest.Recv(t, got, 5*time.Second))
}

// Test_Adapter_FastHTTP_KeepsTargetsThatAreNoPath pins that the asterisk form of OPTIONS and the authority form of
// CONNECT, which have no path to guard, reach a fasthttp handler as sent, while the ones that hide a path do not.
func Test_Adapter_FastHTTP_KeepsTargetsThatAreNoPath(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) { ctx.SetBodyString(string(ctx.RequestURI())) }))

	for _, tc := range []struct {
		name, raw, body string
		status          int
	}{
		{"asterisk-form", "OPTIONS * HTTP/1.1\r\nHost: example.com\r\n\r\n", "*", StatusOK},
		{"authority-form", "CONNECT example.com:443 HTTP/1.1\r\nHost: example.com:443\r\n\r\n", "example.com:443", StatusOK},
		{"authority-form with an empty segment", "CONNECT //admin/data.txt HTTP/1.1\r\nHost: example.com\r\n\r\n", "", StatusNotFound},
		{"authority-form with an escaped slash", "CONNECT admin%2Fdata.txt HTTP/1.1\r\nHost: example.com\r\n\r\n", "", StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			status, body := serveRaw(t, app, tc.raw)
			require.Equal(t, tc.status, status)
			if tc.status == StatusOK {
				require.Equal(t, tc.body, body)
			}
		})
	}
}

// Test_Adapter_FastHTTP_TimedOutHandlerKeepsTheRequest pins that a route handler fasthttp.TimeoutHandler gave up on, which
// goes on reading the request, is not written to once it was answered for: under -race, the line put back is a race with it.
func Test_Adapter_FastHTTP_TimedOutHandlerKeepsTheRequest(t *testing.T) {
	t.Parallel()
	read := make(chan string, 1)
	app := New()
	app.Use(fasthttp.TimeoutHandler(func(ctx *fasthttp.RequestCtx) {
		// No channel to tell it when the request was answered: that would order its read after the write it must not see.
		time.Sleep(150 * time.Millisecond)
		read <- string(ctx.Path())
	}, 20*time.Millisecond, "timeout"))
	ln := apptest.Serve(t, app, ListenConfig{DisableStartupMessage: true})

	conn := apptest.Dial(t, ln)
	_, err := io.WriteString(conn, "GET /%70ub/x?q=1 HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n")
	require.NoError(t, err)
	answer, err := io.ReadAll(conn)
	require.NoError(t, err)
	require.Contains(t, string(answer), "408 Request Timeout")
	require.Equal(t, "/pub/x", apptest.Recv(t, read, 5*time.Second))
}

func Test_HTTPHandler_Direct_RestoresRequestLine(t *testing.T) {
	t.Parallel()
	const target = "/a/../%70ub/x%20y?q=%2F"

	for _, tc := range []struct {
		handler any
		name    string
		panics  bool
	}{
		{name: "returns", handler: func(w http.ResponseWriter, r *http.Request) {
			_, err := io.WriteString(w, r.RequestURI+" "+r.URL.Path)
			assert.NoError(t, err)
		}},
		{name: "panics", panics: true, handler: func(http.ResponseWriter, *http.Request) { panic("direct handler") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			app.Get("/pub/*", tc.handler)

			resp := serveRawTarget(t, app, target)
			require.Equal(t, target, after)
			if tc.panics {
				require.Equal(t, StatusTeapot, resp.StatusCode())
				return
			}
			require.Equal(t, StatusOK, resp.StatusCode())
			require.Equal(t, "/pub/x%20y?q=%2F /pub/x y", string(resp.Body()))
		})
	}
}
