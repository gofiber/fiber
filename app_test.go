// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/utils/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

type typedNilHandlerError struct {
	message string
}

func (e *typedNilHandlerError) Error() string {
	return e.message
}

type fileView struct {
	path    string
	content string
	loads   int
}

func (v *fileView) Load() error {
	contents, err := os.ReadFile(v.path)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	v.content = string(contents)
	v.loads++
	return nil
}

func (*fileView) Render(io.Writer, string, any, ...string) error { return nil }

func testEmptyHandler(_ Ctx) error {
	return nil
}

func testStatus200(t *testing.T, app *App, url, method string) {
	t.Helper()

	apptest.Expect(t, app, apptest.Req(method, url), 200)
}

func Test_App_Test_Goroutine_Leak_Compare(t *testing.T) {
	// Not parallel: this test relies on runtime.NumGoroutine() deltas,
	// which are unreliable when hundreds of other tests spawn goroutines
	// concurrently.

	testCases := []struct {
		handler    Handler
		name       string
		timeout    time.Duration
		sleepTime  time.Duration
		expectLeak bool
	}{
		{
			name: "With timeout (potential leak)",
			handler: func(c Ctx) error {
				time.Sleep(300 * time.Millisecond) // Simulate time-consuming operation
				return c.SendString("ok")
			},
			timeout:    50 * time.Millisecond,  // Short timeout to ensure triggering
			sleepTime:  500 * time.Millisecond, // Wait time longer than handler execution time
			expectLeak: true,
		},
		{
			name:       "Without timeout (no leak)",
			handler:    func(c Ctx) error { return c.SendString("ok") }, // Return immediately
			timeout:    0,                                               // Disable timeout
			sleepTime:  100 * time.Millisecond,
			expectLeak: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := New()

			initialGoroutines := runtime.NumGoroutine()
			t.Logf("[%s] Initial goroutines: %d", tc.name, initialGoroutines)

			app.Get("/", tc.handler)

			numRequests := 10
			for range numRequests {
				req := httptest.NewRequest(MethodGet, "/", http.NoBody)

				if tc.timeout > 0 {
					_, err := app.Test(req, TestConfig{Timeout: tc.timeout, FailOnTimeout: true})
					require.Error(t, err)
					require.ErrorIs(t, err, os.ErrDeadlineExceeded)
				} else if resp, err := app.Test(req); err != nil {
					t.Errorf("unexpected error: %v", err)
				} else {
					require.Equal(t, 200, resp.StatusCode)
				}
			}

			// Wait for normal goroutines to complete
			time.Sleep(tc.sleepTime)

			finalGoroutines := runtime.NumGoroutine()
			leakedGoroutines := max(finalGoroutines-initialGoroutines, 0)
			t.Logf("[%s] Final goroutines: %d (leaked: %d)", tc.name, finalGoroutines, leakedGoroutines)

			if tc.expectLeak {
				// We allow up to 3x the request count to account for noise; zero is tolerated.
				maxLeak := numRequests * 3
				if leakedGoroutines > maxLeak {
					t.Errorf("[%s] Expected at most %d leaked goroutines, but got %d",
						tc.name, maxLeak, leakedGoroutines)
				}
				return
			}

			// No-leak scenario: allow a small buffer for runtime noise.
			// Increase slack to reduce flakes from background goroutines.
			if leakedGoroutines > numRequests {
				t.Errorf("[%s] Expected at most %d leaked goroutines, but got %d",
					tc.name, numRequests, leakedGoroutines)
			}
		})
	}
}

func Test_App_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error { return c.Next() })
	app.Post("/", testEmptyHandler)
	app.Options("/", testEmptyHandler)
	resp := apptest.Expect(t, app, apptest.Req(MethodPost, "/"), 200)
	require.Empty(t, resp.Header.Get(HeaderAllow))

	// requireAllow requires a 405 listing allow for each of methods.
	requireAllow := func(allow string, methods ...string) {
		for _, method := range methods {
			resp := apptest.Expect(t, app, apptest.Req(method, "/"), 405)
			require.Equal(t, allow, resp.Header.Get(HeaderAllow), method)
		}
	}
	requireAllow("POST, OPTIONS", MethodGet, MethodPatch, MethodPut)
	app.Get("/", testEmptyHandler)
	requireAllow("GET, HEAD, POST, OPTIONS", MethodTrace, MethodPatch)
	app.Head("/", testEmptyHandler)
	requireAllow("GET, HEAD, POST, OPTIONS", MethodPut)
}

func Test_App_QueryMethod_AllowHeader(t *testing.T) {
	t.Parallel()
	app := New()
	app.Query("/", testEmptyHandler)

	// OPTIONS auto-response advertises QUERY in the Allow header.
	resp := apptest.Do(t, app, apptest.Req(MethodOptions, "/"))
	require.Contains(t, resp.Header.Get(HeaderAllow), MethodQuery)

	// A non-registered method yields 405 with QUERY listed in Allow.
	resp = apptest.Expect(t, app, "/", StatusMethodNotAllowed)
	require.Contains(t, resp.Header.Get(HeaderAllow), MethodQuery)
}

func Test_App_RegisterNetHTTPHandler(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		register   func(app *App, path string, handler any, handlers ...any) Router
		method     string
		expectBody bool
	}{
		{name: "Get", register: (*App).Get, method: http.MethodGet, expectBody: true},
		{name: "Head", register: (*App).Head, method: http.MethodHead},
		{name: "Post", register: (*App).Post, method: http.MethodPost, expectBody: true},
		{name: "Put", register: (*App).Put, method: http.MethodPut, expectBody: true},
		{name: "Delete", register: (*App).Delete, method: http.MethodDelete, expectBody: true},
		{name: "Connect", register: (*App).Connect, method: http.MethodConnect, expectBody: true},
		{name: "Options", register: (*App).Options, method: http.MethodOptions, expectBody: true},
		{name: "Trace", register: (*App).Trace, method: http.MethodTrace, expectBody: true},
		{name: "Patch", register: (*App).Patch, method: http.MethodPatch, expectBody: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			handler := func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Test", r.Method)
				w.WriteHeader(http.StatusAccepted)
				if r.Method == http.MethodHead {
					return
				}

				_, err := w.Write([]byte("hello from net/http " + r.Method))
				assert.NoError(t, err)
			}

			tt.register(app, "/foo", http.HandlerFunc(handler))

			req := httptest.NewRequest(tt.method, "/foo", http.NoBody)
			if tt.method == http.MethodConnect {
				req.URL.Scheme = "http"
				req.URL.Host = "example.com"
			}

			resp := apptest.Expect(t, app, req, http.StatusAccepted)
			require.Equal(t, tt.method, resp.Header.Get("X-Test"))

			body := apptest.BodyBytes(t, resp)

			if tt.expectBody {
				require.Equal(t, "hello from net/http "+tt.method, string(body))
			} else {
				require.Empty(t, body)
			}
		})
	}
}

func Test_App_Custom_Middleware_404_Should_Not_SetMethodNotAllowed(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use(func(c Ctx) error { return c.SendStatus(404) })

	app.Post("/", testEmptyHandler)

	resp := apptest.ExpectBody(t, app, "/", 404, "Not Found")
	require.Equal(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))
	require.Equal(t, strconv.Itoa(len("Not Found")), resp.Header.Get(HeaderContentLength))

	g := app.Group("/with-next", func(c Ctx) error { return c.Status(404).Next() })

	g.Post("/", testEmptyHandler)

	resp = apptest.ExpectBody(t, app, "/with-next", 404, "Not Found")
	require.Equal(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))
	require.Equal(t, strconv.Itoa(len("Not Found")), resp.Header.Get(HeaderContentLength))
}

func Test_App_ServerErrorHandler_SmallReadBuffer(t *testing.T) {
	t.Parallel()
	app := New(Config{ReadBufferSize: 4096})

	app.Get("/", func(_ Ctx) error { panic(errors.New("should never called")) })

	request := httptest.NewRequest(MethodGet, "/", http.NoBody)
	logHeaderSlice := make([]string, 5000)
	request.Header.Set("Very-Long-Header", strings.Join(logHeaderSlice, "-"))
	_, err := app.Test(request)
	// assert on the error type, not on fasthttp's message wording
	require.ErrorAs(t, err, new(*fasthttp.ErrSmallBuffer))
}

func Test_App_Errors(t *testing.T) {
	t.Parallel()
	app := New(Config{BodyLimit: 4})

	app.Get("/", func(_ Ctx) error { return errors.New("hi, i'm an error") })
	apptest.ExpectBody(t, app, "/", 500, "hi, i'm an error")
	_, err := app.Test(httptest.NewRequest(MethodGet, "/", strings.NewReader("big body")))
	if err != nil {
		require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge, "app.Test(req)")
	}
}

func Test_App_BodyLimit(t *testing.T) {
	t.Parallel()
	const largerLimit = DefaultBodyLimit*2 + 1024 // slightly above double the default

	tests := []struct {
		name     string
		limit    int
		accepted int // size of a body that must get through
		rejected int // size of a body that must be refused
	}{
		// A limit that is not positive falls back to the default.
		{name: "negative", limit: -1, accepted: DefaultBodyLimit - 1, rejected: DefaultBodyLimit + 1},
		{name: "more negative", limit: -512, accepted: DefaultBodyLimit - 1, rejected: DefaultBodyLimit + 1},
		{name: "zero", limit: 0, accepted: DefaultBodyLimit - 1, rejected: DefaultBodyLimit + 1},
		// A body larger than the default but within the custom limit gets through,
		// and one above the custom limit is refused.
		{name: "larger than default", limit: largerLimit, accepted: DefaultBodyLimit + 512, rejected: largerLimit + 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New(Config{BodyLimit: tc.limit})

			app.Post("/", func(c Ctx) error { return c.SendStatus(StatusOK) })

			largeBody := bytes.Repeat([]byte{'a'}, tc.rejected)
			req := httptest.NewRequest(MethodPost, "/", bytes.NewReader(largeBody))
			_, err := app.Test(req)
			require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)

			smallBody := bytes.Repeat([]byte{'a'}, tc.accepted)
			req = httptest.NewRequest(MethodPost, "/", bytes.NewReader(smallBody))
			apptest.Expect(t, app, req, StatusOK)
		})
	}
}

type customConstraint struct{}

func (*customConstraint) Name() string {
	return "test"
}

func (*customConstraint) Execute(param string, args ...string) bool {
	if param == "test" && len(args) == 1 && args[0] == "test" {
		return true
	}

	if len(args) == 0 && param == "c" {
		return true
	}

	return false
}

func Test_App_CustomConstraint(t *testing.T) {
	t.Parallel()
	app := New()
	app.RegisterCustomConstraint(&customConstraint{})

	app.Get("/test/:param<test(test)>", func(c Ctx) error { return c.SendString("test") })
	app.Get("/test2/:param<test>", func(c Ctx) error { return c.SendString("test") })
	app.Get("/test3/:param<test()>", func(c Ctx) error { return c.SendString("test") })
	for path, status := range map[string]int{
		"/test/test": 200, "/test/test2": 404, "/test2/c": 200, "/test2/cc": 404, "/test3/cc": 404,
	} {
		apptest.Expect(t, app, path, status)
	}
}

func Test_App_ErrorHandler_Custom(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ErrorHandler: func(c Ctx, _ error) error { return c.Status(200).SendString("hi, i'm a custom error") },
	})

	app.Get("/", func(_ Ctx) error { return errors.New("hi, i'm an error") })

	apptest.ExpectBody(t, app, "/", 200, "hi, i'm a custom error")
}

func Test_App_ErrorHandler_HandlerStack(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ErrorHandler: func(c Ctx, err error) error {
			require.Equal(t, "1: USE error", err.Error())
			return DefaultErrorHandler(c, err)
		},
	})
	app.Use("/", func(c Ctx) error {
		err := c.Next() // call next USE
		require.Equal(t, "2: USE error", err.Error())
		return errors.New("1: USE error")
	}, func(c Ctx) error {
		err := c.Next() // call [0] GET
		require.Equal(t, "0: GET error", err.Error())
		return errors.New("2: USE error")
	})
	app.Get("/", func(_ Ctx) error { return errors.New("0: GET error") })

	apptest.ExpectBody(t, app, "/", 500, "1: USE error")
}

func Test_App_ErrorHandler_RouteStack(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ErrorHandler: func(c Ctx, err error) error {
			require.Equal(t, "1: USE error", err.Error())
			return DefaultErrorHandler(c, err)
		},
	})
	app.Use("/", func(c Ctx) error {
		err := c.Next()
		require.Equal(t, "0: GET error", err.Error())
		return errors.New("1: USE error") // [2] call ErrorHandler
	})
	app.Get("/test", func(_ Ctx) error {
		return errors.New("0: GET error") // [1] return to USE
	})

	apptest.ExpectBody(t, app, "/test", 500, "1: USE error")
}

// Test_DefaultErrorHandler pins the response DefaultErrorHandler writes: a
// typed nil error is an internal error, and a wrapped fiber.Error keeps its
// code but answers with the wrapping message.
func Test_DefaultErrorHandler(t *testing.T) {
	t.Parallel()
	internal := utils.StatusMessage(StatusInternalServerError)
	wrapped := fmt.Errorf("auth failed: %w", ErrUnauthorized)
	for _, tc := range []struct {
		err        error
		name       string
		wantBody   string
		wantStatus int
	}{
		{name: "typed nil fiber error", err: (*Error)(nil), wantStatus: StatusInternalServerError, wantBody: internal},
		{
			name: "typed nil custom error", err: (*typedNilHandlerError)(nil),
			wantStatus: StatusInternalServerError, wantBody: internal,
		},
		{name: "wrapped fiber error uses wrapped message", err: wrapped, wantStatus: StatusUnauthorized, wantBody: wrapped.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.CtxAs[*DefaultCtx](t, New())
			require.NotPanics(t, func() { require.NoError(t, DefaultErrorHandler(c, tc.err)) })
			require.Equal(t, tc.wantStatus, c.fasthttp.Response.StatusCode())
			require.Equal(t, tc.wantBody, string(c.fasthttp.Response.Body()))
		})
	}
}

func Test_App_serverErrorHandler(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err        error
		name       string
		wantBody   string
		wantStatus int
	}{
		{name: "internal error", err: errors.New("test err"), wantBody: "test err", wantStatus: StatusBadRequest},
		{
			name: "typed nil op error", err: (*net.OpError)(nil), wantBody: utils.StatusMessage(StatusBadGateway),
			wantStatus: StatusBadGateway,
		},
		{
			name: "network error", err: &net.DNSError{Err: "test error", Name: "test host", IsTimeout: false},
			wantBody: utils.StatusMessage(StatusBadGateway), wantStatus: StatusBadGateway,
		},
		{
			name: "unsupported method error", err: errors.New("unsupported http request method 'FOO'"),
			wantBody: utils.StatusMessage(StatusNotImplemented), wantStatus: StatusNotImplemented,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			c := apptest.CtxAs[*DefaultCtx](t, app)

			require.NotPanics(t, func() { app.serverErrorHandler(c.fasthttp, tc.err) })
			require.Equal(t, tc.wantBody, string(c.fasthttp.Response.Body()))
			require.Equal(t, tc.wantStatus, c.fasthttp.Response.StatusCode())
		})
	}
}

func Test_App_Nested_Params(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/test", func(c Ctx) error { return c.Status(400).Send([]byte("Should move on")) })
	app.Get("/test/:param", func(c Ctx) error { return c.Status(400).Send([]byte("Should move on")) })
	app.Get("/test/:param/test", func(c Ctx) error { return c.Status(400).Send([]byte("Should move on")) })
	app.Get("/test/:param/test/:param2", func(c Ctx) error { return c.Status(200).Send([]byte("Good job")) })

	apptest.Expect(t, app, "/test/john/test/doe", 200)
}

func Test_App_Use_Params(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use("/prefix/:param", func(c Ctx) error {
		require.Equal(t, "john", c.Params("param"))
		return nil
	})

	app.Use("/foo/:bar?", func(c Ctx) error {
		require.Equal(t, "foobar", c.Params("bar", "foobar"))
		return nil
	})

	app.Use("/:param/*", func(c Ctx) error {
		require.Equal(t, "john", c.Params("param"))
		require.Equal(t, "doe", c.Params("*"))
		return nil
	})

	apptest.Expect(t, app, "/prefix/john", 200)

	apptest.Expect(t, app, "/john/doe", 200)

	apptest.Expect(t, app, "/foo", 200)

	require.PanicsWithValue(t, "use: invalid handler func()\n", func() {
		app.Use("/:param/*", func() {
			// this should panic
		})
	})
}

func Test_App_Use_PanicsOnDroppedArguments(t *testing.T) {
	t.Parallel()
	deny := func(c Ctx) error { return c.SendStatus(StatusUnauthorized) }
	guardApp := New()
	guardApp.Use(deny)

	for kind, use := range map[string]func(app *App, args ...any){
		"app":    func(app *App, args ...any) { app.Use(args...) },
		"group":  func(app *App, args ...any) { app.Group("/g").Use(args...) },
		"domain": func(app *App, args ...any) { app.Domain("example.com").Use(args...) },
	} {
		for _, tc := range []struct {
			name string
			args []any
		}{
			{name: "prefix, handler, sub-app", args: []any{"/admin", deny, New()}},
			{name: "prefix, sub-app, handler", args: []any{"/admin", New(), deny}},
			{name: "handler, sub-app", args: []any{deny, New()}},
			{name: "prefixes, handler, sub-app", args: []any{[]string{"/a", "/b"}, deny, New()}},
			{name: "prefix, two sub-apps", args: []any{"/admin", guardApp, New()}},
			{name: "two prefixes, handler", args: []any{"/admin", "/internal", deny}},
			{name: "prefix, prefixes, handler", args: []any{"/admin", []string{"/internal"}, deny}},
			{name: "two lists of prefixes, handler", args: []any{[]string{"/a"}, []string{"/b"}, deny}},
		} {
			require.Panics(t, func() { use(New(), tc.args...) }, "%s: %s", kind, tc.name)
		}
	}

	// separate calls keep working
	sub := New()
	sub.Get("/panel", func(c Ctx) error { return c.SendString("panel") })
	app := New()
	app.Use("/admin", deny)
	app.Use("/admin", sub)
	apptest.Expect(t, app, "/admin/panel", StatusUnauthorized)
}

func Test_App_Use_UnescapedPath(t *testing.T) {
	t.Parallel()
	app := New(Config{UnescapePath: true, CaseSensitive: true})

	app.Use("/cRéeR/:param", func(c Ctx) error {
		require.Equal(t, "/cRéeR/اختبار", c.Path())
		return c.SendString(c.Params("param"))
	})

	app.Use("/abc", func(c Ctx) error {
		require.Equal(t, "/AbC", c.Path())
		return nil
	})

	apptest.ExpectBody(t, app, "/cR%C3%A9eR/%D8%A7%D8%AE%D8%AA%D8%A8%D8%A7%D8%B1", StatusOK, "اختبار")
	// with lowercase letters
	apptest.Expect(t, app, "/cr%C3%A9er/%D8%A7%D8%AE%D8%AA%D8%A8%D8%A7%D8%B1", StatusNotFound)
}

func Test_App_Use_CaseSensitive(t *testing.T) {
	t.Parallel()
	app := New(Config{CaseSensitive: true})

	app.Use("/abc", func(c Ctx) error { return c.SendString(c.Path()) })

	// wrong letters in the requested route -> 404
	apptest.Expect(t, app, "/AbC", StatusNotFound)

	// right letters in the requested route -> 200
	apptest.Expect(t, app, "/abc", StatusOK)

	// with case-insensitive matching the handler still sees the path as requested
	app.config.CaseSensitive = false
	apptest.ExpectBody(t, app, "/AbC", StatusOK, "/AbC")
}

func Test_App_Use_MultiplePrefix(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use([]string{"/john", "/doe"}, func(c Ctx) error { return c.SendString(c.Path()) })

	g := app.Group("/test")
	g.Use([]string{"/john", "/doe"}, func(c Ctx) error { return c.SendString(c.Path()) })

	for _, path := range []string{"/john", "/doe", "/test/john", "/test/doe"} {
		apptest.ExpectBody(t, app, path, StatusOK, path)
	}
}

func Test_Group_Use_NoBoundary(t *testing.T) {
	t.Parallel()
	app := New()
	grp := app.Group("/api")

	grp.Use("/foo", func(c Ctx) error { return c.SendStatus(StatusOK) })

	apptest.Expect(t, app, "/api/foo/bar", StatusOK)

	apptest.Expect(t, app, "/api/foobar", StatusNotFound)
}

// Test_App_Use_StrictRouting pins how a trailing slash is matched, on the app
// and on a group: a Use prefix without StrictRouting ignores it, a route with
// StrictRouting does not.
func Test_App_Use_StrictRouting(t *testing.T) {
	t.Parallel()
	handler := func(c Ctx) error { return c.SendString(c.Path()) }
	for _, tc := range []struct {
		want   map[string]int // path -> status
		name   string
		strict bool // StrictRouting with Get routes, instead of Use prefixes without it
	}{
		{name: "Use without StrictRouting", want: map[string]int{"/abc/": StatusOK, "/abc": StatusOK, "/foo": StatusOK, "/foo/": StatusOK}},
		{
			name: "Get with StrictRouting", strict: true,
			want: map[string]int{"/abc/": StatusNotFound, "/abc": StatusOK, "/foo": StatusNotFound, "/foo/": StatusOK},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New(Config{StrictRouting: tc.strict})
			if tc.strict {
				app.Get("/abc", handler)
				app.Group("/foo").Get("/", handler)
			} else {
				app.Use("/abc", handler)
				app.Group("/foo").Use("/", handler)
			}
			for path, status := range tc.want {
				apptest.Expect(t, app, path, status)
			}
		})
	}
}

func Test_App_Add_Method_Test(t *testing.T) {
	t.Parallel()
	methods := append(DefaultMethods, "JOHN") //nolint:gocritic // We want a new slice here
	app := New(Config{RequestMethods: methods})

	app.Add([]string{"JOHN"}, "/john", testEmptyHandler)

	apptest.Expect(t, app, apptest.Req("JOHN", "/john"), StatusOK)

	apptest.Expect(t, app, "/john", StatusMethodNotAllowed)

	apptest.Expect(t, app, apptest.Req("UNKNOWN", "/john"), StatusNotImplemented)

	// Add a new method
	require.Panics(t, func() { app.Add([]string{"JANE"}, "/jane", testEmptyHandler) })
}

func Test_App_All_Method_Test(t *testing.T) {
	t.Parallel()
	methods := append(DefaultMethods, "JOHN") //nolint:gocritic // We want a new slice here
	app := New(Config{RequestMethods: methods})

	app.All("/doe", testEmptyHandler)

	apptest.Expect(t, app, apptest.Req("JOHN", "/doe"), StatusOK)

	// Add a new method
	require.Panics(t, func() { app.Add([]string{"JANE"}, "/jane", testEmptyHandler) })
}

func Test_App_GETOnly(t *testing.T) {
	t.Parallel()
	app := New(Config{GETOnly: true})

	app.Post("/", func(c Ctx) error { return c.SendString("Hello 👋!") })

	apptest.Expect(t, app, apptest.Req(MethodPost, "/"), StatusMethodNotAllowed)
}

func Test_App_RefusedRequest_Middleware_Status(t *testing.T) {
	t.Parallel()
	app := New(Config{BodyLimit: 8})

	observed := make(chan int, 1)
	app.Use(func(c Ctx) error {
		err := c.Next()
		observed <- c.Response().StatusCode()
		return err
	})

	app.Post("/", func(c Ctx) error { return c.SendString("Hello 👋!") })

	req := httptest.NewRequest(MethodPost, "/", strings.NewReader(strings.Repeat("x", 64)))
	_, err := app.Test(req)
	require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
	require.Equal(t, StatusRequestEntityTooLarge, <-observed, "Status code seen by middleware")
}

// The replay would run a Use handler that ends the chain with the method GETOnly refused.
func Test_App_GETOnly_RefusedMethodIsNotReplayed(t *testing.T) {
	t.Parallel()
	app := New(Config{GETOnly: true})

	var ran []string
	app.Use("/api", func(c Ctx) error {
		ran = append(ran, c.Method())
		return c.SendString("done")
	})

	resp := apptest.Expect(t, app, apptest.Req(MethodPost, "/api/jobs/7/cancel"), StatusMethodNotAllowed)
	require.Equal(t, "GET, HEAD", resp.Header.Get(HeaderAllow))
	require.Empty(t, ran)

	apptest.Expect(t, app, "/api/jobs/7", StatusOK)
	require.Equal(t, []string{MethodGet}, ran)
}

// The replay skips a guard registered as a route, and SendStatus keeps a body already written.
func Test_App_RefusedRequest_ChainBodyIsNotSent(t *testing.T) {
	t.Parallel()
	app := New(Config{
		BodyLimit: 16,
		ErrorHandler: func(c Ctx, err error) error {
			var fiberErr *Error
			if !errors.As(err, &fiberErr) {
				return err
			}
			return c.SendStatus(fiberErr.Code)
		},
	})

	app.All("/admin/*", func(c Ctx) error { return c.SendStatus(StatusUnauthorized) })
	app.Use(func(c Ctx) error { return c.SendString("TOP-SECRET") })
	ln := startRawServer(t, app)

	const get = "GET /admin/secret.txt HTTP/1.1\r\nHost: example.com\r\n"
	rawExpectStatus(t, ln, get+"\r\n", StatusUnauthorized)

	resp := rawExpectStatus(t, ln, get+"Content-Length: 99999\r\n\r\n", StatusRequestEntityTooLarge)
	require.Equal(t, "Request Entity Too Large", resp.body)
}

// An ErrorHandler that writes only a body used to answer a transport-refused
// request with 200. Seeding the status changes that, so pin it.
func Test_App_GETOnly_ErrorHandler_WithoutStatus(t *testing.T) {
	t.Parallel()
	app := New(Config{
		GETOnly:      true,
		ErrorHandler: func(c Ctx, err error) error { return c.SendString(err.Error()) },
	})

	app.Post("/", func(c Ctx) error { return c.SendString("Hello 👋!") })

	resp := apptest.Expect(t, app, apptest.Req(MethodPost, "/"), StatusMethodNotAllowed)

	body := apptest.Body(t, resp)
	require.Equal(t, "Method Not Allowed", body, "Body")
}

// RFC 9110 §15.5.6: a 405 lists the allowed methods, even when a custom
// ErrorHandler rewrites the response.
func Test_App_GETOnly_Allow(t *testing.T) {
	t.Parallel()
	for name, config := range map[string]Config{
		"default error handler": {GETOnly: true},
		"custom error handler": {
			GETOnly:      true,
			ErrorHandler: func(c Ctx, err error) error { return c.SendString(err.Error()) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := New(config)
			app.All("/", func(c Ctx) error { return c.SendString("ok") })

			for _, method := range []string{MethodPost, MethodPut, MethodPatch, MethodDelete, MethodOptions} {
				resp := apptest.Do(t, app, apptest.Req(method, "/"))
				require.Equal(t, StatusMethodNotAllowed, resp.StatusCode, method)
				require.Equal(t, "GET, HEAD", resp.Header.Get(HeaderAllow), method)
			}

			for _, method := range []string{MethodGet, MethodHead} {
				resp := apptest.Do(t, app, apptest.Req(method, "/"))
				require.Equal(t, StatusOK, resp.StatusCode, method)
				require.Empty(t, resp.Header.Get(HeaderAllow), method)
			}
		})
	}
}

func Test_App_Use_Params_Group(t *testing.T) {
	t.Parallel()
	app := New()

	group := app.Group("/prefix/:param/*")
	group.Use("/", func(c Ctx) error { return c.Next() })
	group.Get("/test", func(c Ctx) error {
		require.Equal(t, "john", c.Params("param"))
		require.Equal(t, "doe", c.Params("*"))
		return nil
	})

	apptest.Expect(t, app, "/prefix/john/doe/test", 200)
}

func Test_App_Chaining(t *testing.T) {
	t.Parallel()
	n := func(c Ctx) error { return c.Next() }
	app := New()
	app.Use("/john", n, n, n, n, func(c Ctx) error { return c.SendStatus(202) })
	// check handler count for registered HEAD route
	require.Len(t, app.stack[app.methodInt(MethodHead)][0].Handlers, 5, "app.Test(req)")

	apptest.Expect(t, app, apptest.Req(MethodPost, "/john"), 202)

	app.Get("/test", n, n, n, n, func(c Ctx) error { return c.SendStatus(203) })

	apptest.Expect(t, app, "/test", 203)
}

func Test_App_Order(t *testing.T) {
	t.Parallel()
	app := New()
	// write returns a handler that writes s and hands over to the next one.
	write := func(s string) Handler {
		return func(c Ctx) error {
			_, err := c.WriteString(s)
			require.NoError(t, err)
			return c.Next()
		}
	}
	app.Get("/test", write("1"))
	app.All("/test", write("2"))
	app.Use(func(c Ctx) error {
		_, err := c.WriteString("3")
		require.NoError(t, err)
		return c.SendStatus(StatusOK)
	})
	apptest.ExpectBody(t, app, "/test", 200, "123")
}

func Test_App_AutoHead_Compliance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		path     string
		xTest    string
		body     string
		sendFile bool // the handler serves the body from a file instead of a string
	}{
		{name: "string", path: "/hello", xTest: "string", body: "hello"},
		{name: "SendFile", path: "/file", xTest: "file", body: "file-body", sendFile: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var filePath string
			if tc.sendFile {
				if runtime.GOOS == "windows" {
					t.Skip("SendFile auto-HEAD test is skipped on Windows due to file locking semantics")
				}

				filePath = filepath.Join(t.TempDir(), "hello.txt")
				require.NoError(t, os.WriteFile(filePath, []byte(tc.body), 0o644))
			}

			app := New()
			app.Get(tc.path, func(c Ctx) error {
				c.Set("X-Test", tc.xTest)
				if tc.sendFile {
					return c.SendFile(filePath)
				}
				return c.SendString(tc.body)
			})
			app.startupProcess()

			getResp := apptest.Do(t, app, tc.path)
			defer func() { require.NoError(t, getResp.Body.Close()) }()

			body := apptest.BodyBytes(t, getResp)
			require.Equal(t, tc.body, string(body))
			require.Equal(t, tc.xTest, getResp.Header.Get("X-Test"))

			headResp := apptest.Do(t, app, apptest.Req(MethodHead, tc.path))
			defer func() { require.NoError(t, headResp.Body.Close()) }()

			require.Equal(t, getResp.StatusCode, headResp.StatusCode)
			require.Equal(t, strconv.Itoa(len(body)), headResp.Header.Get(HeaderContentLength))
			require.Equal(t, getResp.Header.Get(HeaderContentType), headResp.Header.Get(HeaderContentType))
			require.Equal(t, getResp.Header.Get("X-Test"), headResp.Header.Get("X-Test"))

			headBody := apptest.BodyBytes(t, headResp)
			require.Empty(t, headBody)
		})
	}
}

func Test_App_Methods(t *testing.T) {
	t.Parallel()
	app := New()
	for _, tc := range []struct {
		register func(path string, handler any, handlers ...any) Router
		method   string
	}{
		{register: app.Connect, method: MethodConnect},
		{register: app.Put, method: MethodPut},
		{register: app.Post, method: MethodPost},
		{register: app.Delete, method: MethodDelete},
		{register: app.Head, method: MethodHead},
		{register: app.Patch, method: MethodPatch},
		{register: app.Options, method: MethodOptions},
		{register: app.Trace, method: MethodTrace},
		{register: app.Query, method: MethodQuery},
		{register: app.Get, method: MethodGet},
		{register: app.All, method: MethodPost},
	} {
		tc.register("/:john?/:doe?", testEmptyHandler)
		testStatus200(t, app, "/john/doe", tc.method)
	}
	app.Use("/:john?/:doe?", testEmptyHandler)
	testStatus200(t, app, "/john/doe", MethodGet)
}

func Test_App_Route_Naming(t *testing.T) {
	t.Parallel()
	app := New()
	handler := func(c Ctx) error { return c.SendStatus(StatusOK) }
	app.Get("/john", handler).Name("john")
	app.Delete("/doe", handler)
	app.Name("doe")

	jane := app.Group("/jane").Name("jane.")
	group := app.Group("/group")
	subGroup := jane.Group("/sub-group").Name("sub.")

	jane.Get("/test", handler).Name("test")
	jane.Trace("/trace", handler).Name("trace")

	group.Get("/test", handler).Name("test")

	app.Post("/post", handler).Name("post")

	subGroup.Get("/done", handler).Name("done")

	require.Equal(t, "post", app.GetRoute("post").Name)
	require.Equal(t, "john", app.GetRoute("john").Name)
	require.Equal(t, "jane.test", app.GetRoute("jane.test").Name)
	require.Equal(t, "jane.trace", app.GetRoute("jane.trace").Name)
	require.Equal(t, "jane.sub.done", app.GetRoute("jane.sub.done").Name)
	require.Equal(t, "test", app.GetRoute("test").Name)
}

func Test_App_New(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", testEmptyHandler)

	appConfig := New(Config{Immutable: true})
	appConfig.Get("/", testEmptyHandler)
}

func Test_App_Config(t *testing.T) {
	t.Parallel()
	app := New(Config{StrictRouting: true})
	require.True(t, app.Config().StrictRouting)
}

func Test_App_Config_DefaultBufferSizes(t *testing.T) {
	t.Parallel()
	app := New()
	require.Equal(t, 8192, app.Config().ReadBufferSize)
	require.Equal(t, 8192, app.Config().WriteBufferSize)
	require.Equal(t, DefaultReadBufferSize, app.Config().ReadBufferSize)
	require.Equal(t, DefaultWriteBufferSize, app.Config().WriteBufferSize)
}

func Test_App_GetString(t *testing.T) {
	t.Parallel()
	heap := string([]byte("fiber"))
	appMutable := New()
	same := appMutable.GetString(heap)
	require.Same(t, unsafe.StringData(heap), unsafe.StringData(same), "expected original string when immutable is disabled")

	appImmutable := New(Config{Immutable: true})
	copied := appImmutable.GetString(heap)
	require.NotSame(t, unsafe.StringData(heap), unsafe.StringData(copied), "expected a copy for heap-backed string when immutable is enabled")

	literal := "fiber"
	sameLit := appImmutable.GetString(literal)
	require.Same(t, unsafe.StringData(literal), unsafe.StringData(sameLit), "expected original literal when immutable is enabled")
}

func Test_App_GetBytes(t *testing.T) {
	t.Parallel()
	b := []byte("fiber")
	appMutable := New()
	same := appMutable.GetBytes(b)
	require.Same(t, unsafe.SliceData(b), unsafe.SliceData(same), "expected original slice when immutable is disabled")

	alias := make([]byte, 10)
	copy(alias, b)
	sub := alias[:5]
	appImmutable := New(Config{Immutable: true})
	copied := appImmutable.GetBytes(sub)
	require.NotSame(t, unsafe.SliceData(sub), unsafe.SliceData(copied), "expected a copy for aliased slice when immutable is enabled")

	full := make([]byte, 5)
	copy(full, b)
	detached := appImmutable.GetBytes(full)
	require.NotSame(t, unsafe.SliceData(full), unsafe.SliceData(detached), "expected a copy even when cap==len")
}

// appListen serves app on a new in-memory listener from a goroutine that is
// running by the time it returns; an error from the server fails the test.
func appListen(t *testing.T, app *App, cfg ...ListenConfig) *fasthttputil.InmemoryListener {
	t.Helper()
	ln := fasthttputil.NewInmemoryListener()
	ready := make(chan struct{})
	go func() {
		close(ready)
		assert.NoError(t, app.Listener(ln, cfg...))
	}()
	<-ready
	return ln
}

func Test_App_Shutdown(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		app := New()
		require.NoError(t, app.Shutdown())
	})

	t.Run("no server", func(t *testing.T) {
		t.Parallel()
		app := &App{}
		if err := app.Shutdown(); err != nil {
			require.ErrorContains(t, err, "shutdown: server is not running")
		}
	})
}

func Test_App_ShutdownWithTimeout(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		time.Sleep(5 * time.Second)
		return c.SendString("body")
	})
	ln := appListen(t, app)

	connReady := make(chan struct{})
	go func() {
		conn, err := ln.Dial()
		assert.NoError(t, err)
		_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: google.com\r\n\r\n"))
		assert.NoError(t, err)
		connReady <- struct{}{} // Signal that the request has been sent
	}()
	<-connReady

	shutdownErr := make(chan error)
	go func() { shutdownErr <- app.ShutdownWithTimeout(1 * time.Second) }()

	err := apptest.Recv(t, shutdownErr, 5*time.Second, "idle connections not closed on shutdown")
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected err %v. Expecting %v", err, context.DeadlineExceeded)
	}
}

// A streamed body is written after the handler has already returned, so the
// shutdown tests above, which block inside the handler, say nothing about it.
// Reported as #3370 against v3.0.0-beta.4: a download was cut off instead of
// being allowed to finish within the timeout.
func Test_App_ShutdownWithTimeout_WaitsForStreamedBody(t *testing.T) {
	t.Parallel()
	const chunks = 5

	streaming := make(chan struct{})
	finished := make(chan struct{})

	app := New()
	app.Get("/stream", func(c Ctx) error {
		c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
			defer close(finished)
			for i := range chunks {
				_, err := w.WriteString("chunk")
				assert.NoError(t, err)
				assert.NoError(t, w.Flush())
				if i == 0 {
					close(streaming)
				}
				time.Sleep(50 * time.Millisecond)
			}
		})
		return nil
	})
	ln := appListen(t, app)

	conn, err := ln.Dial()
	require.NoError(t, err)
	_, err = conn.Write([]byte("GET /stream HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	require.NoError(t, err)

	// Shut down mid-stream, which is the point: the handler is long gone by now.
	<-streaming
	// Sampled where shutdown returns, not after the read below: reading the whole
	// body first closes finished, and the check would then hold either way.
	streamDone := make(chan bool, 1)
	go func() {
		assert.NoError(t, app.ShutdownWithTimeout(10*time.Second))
		select {
		case <-finished:
			streamDone <- true
		default:
			streamDone <- false
		}
	}()

	var resp fasthttp.Response
	require.NoError(t, resp.Read(bufio.NewReader(conn)))
	require.Equal(t, strings.Repeat("chunk", chunks), string(resp.Body()))

	require.True(t, <-streamDone, "shutdown returned while the body was still being written")
}

// Test_App_Shutdown_ContextNotCanceled pins #3431: c.Context() stays usable for
// in-flight work during graceful shutdown (unlike RequestCtx.Done()).
func Test_App_Shutdown_ContextNotCanceled(t *testing.T) {
	t.Parallel()
	app := New()
	handlerStarted := make(chan struct{})
	// Results observed inside the handler after shutdown has begun.
	contextErr := make(chan error, 1)
	requestCtxCanceled := make(chan bool, 1)

	app.Get("/slow", func(c Ctx) error {
		close(handlerStarted)

		// Waiting on RequestCtx first is what gives the second check meaning: it
		// proves shutdown has actually started, so a c.Context() that is still
		// alive afterwards cannot just be one that shutdown has yet to reach.
		select {
		case <-c.RequestCtx().Done():
			requestCtxCanceled <- true
		case <-time.After(5 * time.Second):
			requestCtxCanceled <- false
		}

		select {
		case <-c.Context().Done():
			contextErr <- c.Context().Err()
		default:
			contextErr <- nil
		}

		return c.SendString("ok")
	})
	ln := appListen(t, app)

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		conn, err := ln.Dial()
		if !assert.NoError(t, err) {
			return
		}
		_, err = conn.Write([]byte("GET /slow HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		assert.NoError(t, err)
		// Drain response so the connection can finish cleanly.
		_, copyErr := io.Copy(io.Discard, conn)
		assert.NoError(t, copyErr)
		assert.NoError(t, conn.Close())
	}()

	apptest.Recv(t, handlerStarted, 5*time.Second, "the handler never ran; the client goroutine could not reach it")
	require.NoError(t, app.ShutdownWithTimeout(5*time.Second))
	<-clientDone

	require.True(t, <-requestCtxCanceled, "RequestCtx.Done() is closed when the server shuts down")
	require.NoError(t, <-contextErr, "c.Context() must not be canceled by graceful shutdown")
}

func Test_App_ShutdownWithContext(t *testing.T) {
	t.Parallel()
	t.Run("successful shutdown", func(t *testing.T) {
		t.Parallel()
		app := New()
		// Fast request that should complete
		app.Get("/", func(c Ctx) error { return c.SendString("OK") })
		ln := appListen(t, app)

		conn, err := ln.Dial()
		require.NoError(t, err)
		_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, app.ShutdownWithContext(ctx), "Expected successful shutdown")
	})

	t.Run("shutdown with hooks", func(t *testing.T) {
		t.Parallel()
		app := New()
		hookOrder := make([]string, 0)
		var hookMutex sync.Mutex
		record := func(hook string) error {
			hookMutex.Lock()
			defer hookMutex.Unlock()
			hookOrder = append(hookOrder, hook)
			return nil
		}
		app.Hooks().OnPreShutdown(func() error { return record("pre") })
		app.Hooks().OnPostShutdown(func(_ error) error { return record("post") })
		appListen(t, app)
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, app.ShutdownWithContext(context.Background()))
		require.Equal(t, []string{"pre", "post"}, hookOrder, "Hooks should execute in order")
	})

	t.Run("timeout with long running request", func(t *testing.T) {
		t.Parallel()
		app := New()

		requestStarted := make(chan struct{})
		requestProcessing := make(chan struct{})

		app.Get("/", func(c Ctx) error {
			close(requestStarted)
			// Wait for signal to continue processing the request
			<-requestProcessing
			time.Sleep(2 * time.Second)
			return c.SendString("OK")
		})
		ln := appListen(t, app)
		// Ensure server is fully started
		time.Sleep(100 * time.Millisecond)

		go func() {
			conn, err := ln.Dial()
			if err != nil {
				t.Errorf("Failed to dial: %v", err)
				return
			}
			if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n")); err != nil {
				t.Errorf("Failed to write: %v", err)
			}
		}()

		apptest.Recv(t, requestStarted, 2*time.Second, "Request did not start in time")
		// Request has started, signal to continue processing
		close(requestProcessing)

		// Attempt shutdown, should timeout
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		require.ErrorIs(t, app.ShutdownWithContext(ctx), context.DeadlineExceeded)
	})
}

func Test_App_OptionsAsterisk(t *testing.T) {
	t.Parallel()
	app := New()
	app.Options("/resource", func(c Ctx) error {
		c.Set(HeaderAllow, "GET")
		c.Status(StatusNoContent)

		return nil
	})
	app.Options("*", func(c Ctx) error {
		c.Set(HeaderAllow, "GET, POST")
		c.Status(StatusOK)

		return nil
	})

	ln := startRawServer(t, app)

	for _, tc := range []struct {
		target, allow string
		status        int
	}{
		{target: "*", allow: "GET, POST", status: StatusOK},
		{target: "/resource", allow: "GET", status: StatusNoContent}, // the control request
	} {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()
			responses, _ := rawExchange(t, ln, "OPTIONS "+tc.target+" HTTP/1.1\r\nHost: example.com\r\n\r\n", 1)
			resp := responses[0]
			require.Equal(t, tc.status, resp.status)
			require.Equal(t, tc.allow, resp.header.Get(HeaderAllow))
			require.Zero(t, resp.contentLength)
			require.Empty(t, resp.body)
		})
	}
}

func Test_App_Mixed_Routes_WithSameLen(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use(func(c Ctx) error {
		c.Set("TestHeader", "TestValue")
		return c.Next()
	})
	// routes with the same length
	app.Get("/tesbar", func(c Ctx) error {
		c.Type("html")
		return c.Send([]byte("TEST_BAR"))
	})
	app.Get("/foobar", func(c Ctx) error {
		c.Type("html")
		return c.Send([]byte("FOO_BAR"))
	})

	for _, tc := range []struct{ path, body string }{
		{path: "/foobar", body: "FOO_BAR"},  // match get route
		{path: "/tesbar", body: "TEST_BAR"}, // match static route
	} {
		resp := apptest.ExpectBody(t, app, tc.path, 200, tc.body)
		require.NotEmpty(t, resp.Header.Get(HeaderContentLength), tc.path)
		require.Equal(t, "TestValue", resp.Header.Get("TestHeader"), tc.path)
		require.Equal(t, "text/html; charset=utf-8", resp.Header.Get(HeaderContentType), tc.path)
	}
}

func Test_App_Group_Invalid(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "use: invalid handler int\n", func() { New().Group("/").Use(1) })
}

func Test_App_Group(t *testing.T) {
	t.Parallel()
	app := New()
	grp := app.Group("/test")
	grp.Get("/", testEmptyHandler)
	testStatus200(t, app, "/test", MethodGet)
	grp.Get("/:demo?", testEmptyHandler)
	testStatus200(t, app, "/test/john", MethodGet)
	for _, tc := range []struct {
		register     func(path string, handler any, handlers ...any) Router
		path, method string
	}{
		{grp.Connect, "/CONNECT", MethodConnect},
		{grp.Put, "/PUT", MethodPut},
		{grp.Post, "/POST", MethodPost},
		{grp.Delete, "/DELETE", MethodDelete},
		{grp.Head, "/HEAD", MethodHead},
		{grp.Patch, "/PATCH", MethodPatch},
		{grp.Options, "/OPTIONS", MethodOptions},
		{grp.Trace, "/TRACE", MethodTrace},
		{grp.Query, "/QUERY", MethodQuery},
		{grp.All, "/ALL", MethodPost},
	} {
		tc.register(tc.path, testEmptyHandler)
		testStatus200(t, app, "/test"+tc.path, tc.method)
	}
	grp.Use(testEmptyHandler)
	testStatus200(t, app, "/test/oke", MethodGet)
	grp.Use("/USE", testEmptyHandler)
	testStatus200(t, app, "/test/USE/oke", MethodGet)

	api := grp.Group("/v1")
	api.Post("/", testEmptyHandler)
	apptest.Expect(t, app, apptest.Req(MethodPost, "/test/v1/"), 200)
	api.Get("/users", testEmptyHandler)
	apptest.Expect(t, app, "/test/v1/UsErS", 200)
}

func Test_App_RouteChain(t *testing.T) {
	t.Parallel()
	dummyHandler := testEmptyHandler

	app := New()

	register := app.RouteChain("/test").Get(dummyHandler).Head(dummyHandler).Post(dummyHandler).Put(dummyHandler).
		Delete(dummyHandler).
		Connect(dummyHandler).
		Options(dummyHandler).
		Trace(dummyHandler).
		Patch(dummyHandler).
		Query(dummyHandler)

	for _, method := range []string{
		MethodGet, MethodHead, MethodPost, MethodPut, MethodDelete, MethodConnect, MethodOptions, MethodTrace, MethodPatch, MethodQuery,
	} {
		testStatus200(t, app, "/test", method)
	}
	register.RouteChain("/v1").Get(dummyHandler).Post(dummyHandler)
	apptest.Expect(t, app, apptest.Req(MethodPost, "/test/v1"), 200)
	apptest.Expect(t, app, "/test/v1", 200)
	register.RouteChain("/v1").RouteChain("/v2").RouteChain("/v3").Get(dummyHandler).Trace(dummyHandler)
	apptest.Expect(t, app, apptest.Req(MethodTrace, "/test/v1/v2/v3"), 200)
	apptest.Expect(t, app, "/test/v1/v2/v3", 200)
}

func Test_App_Route(t *testing.T) {
	t.Parallel()
	app := New()

	app.Route("/test", func(api Router) {
		api.Get("/foo", testEmptyHandler).Name("foo")

		api.Route("/bar", func(bar Router) { bar.Get("/", testEmptyHandler).Name("index") }, "bar.")
	}, "test.")

	testStatus200(t, app, "/test/foo", MethodGet)

	apptest.Expect(t, app, "/test/bar/", http.StatusOK)

	require.Equal(t, "/test/foo", app.GetRoute("test.foo").Path)
	require.Equal(t, "/test/bar/", app.GetRoute("test.bar.index").Path)
}

func Test_App_Route_nilFuncPanics(t *testing.T) {
	t.Parallel()
	app := New()

	require.PanicsWithValue(t, "route handler 'fn' cannot be nil", func() { app.Route("/panic", nil) })
}

func Test_Group_Route_nilFuncPanics(t *testing.T) {
	t.Parallel()
	app := New()
	grp := app.Group("/api")

	require.PanicsWithValue(t, "route handler 'fn' cannot be nil", func() { grp.Route("/panic", nil) })
}

func Test_Group_RouteChain_All(t *testing.T) {
	t.Parallel()
	app := New()
	var calls []string
	grp := app.Group("/api", func(c Ctx) error {
		calls = append(calls, "group")
		return c.Next()
	})

	grp.RouteChain("/users").All(func(c Ctx) error {
		calls = append(calls, "routechain")
		return c.SendStatus(http.StatusOK)
	})

	apptest.Expect(t, app, "/api/users", http.StatusOK)
	require.Equal(t, []string{"group", "routechain"}, calls)
}

func Test_App_Deep_Group(t *testing.T) {
	t.Parallel()
	runThroughCount := 0
	dummyHandler := func(c Ctx) error {
		runThroughCount++
		return c.Next()
	}

	app := New()
	gAPI := app.Group("/api", dummyHandler)
	gV1 := gAPI.Group("/v1", dummyHandler)
	gUser := gV1.Group("/user", dummyHandler)
	gUser.Get("/authenticate", func(c Ctx) error {
		runThroughCount++
		return c.SendStatus(200)
	})
	testStatus200(t, app, "/api/v1/user/authenticate", MethodGet)
	require.Equal(t, 4, runThroughCount, "Loop count")
}

func Test_App_Next_Method(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use(func(c Ctx) error {
		require.Equal(t, MethodGet, c.Method())
		err := c.Next()
		require.Equal(t, MethodGet, c.Method())
		return err
	})

	apptest.Expect(t, app, "/", 404)
}

func Benchmark_NewError(b *testing.B) {
	for b.Loop() {
		NewError(200, "test") //nolint:errcheck // not needed
	}
}

func Benchmark_NewError_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			NewError(200, "test") //nolint:errcheck // not needed
		}
	})
}

func Test_NewError(t *testing.T) {
	t.Parallel()
	e := NewError(StatusForbidden, "permission denied")
	require.Equal(t, StatusForbidden, e.Code)
	require.Equal(t, "permission denied", e.Message)
}

func Test_NewErrorf_Format(t *testing.T) {
	t.Parallel()
	type args []any

	tests := []struct {
		name string
		want string
		in   args
		code int
	}{
		{name: "no-args → default text", code: StatusNotFound, in: nil, want: utils.StatusMessage(StatusNotFound)},
		{
			name: "single-string arg overrides", code: StatusBadRequest, in: args{"custom bad request"},
			want: "custom bad request",
		},
		{
			name: "single non-string arg stringified", code: StatusInternalServerError, in: args{errors.New("db down")},
			want: "db down",
		},
		{name: "single nil interface", code: StatusInternalServerError, in: args{any(nil)}, want: "<nil>"},
		{name: "format string + args", code: StatusBadRequest, in: args{"invalid id %d", 10}, want: "invalid id 10"},
		{
			name: "format string + excess args", code: StatusBadRequest, in: args{"odd %d", 1, 2, 3},
			want: "odd 1%!(EXTRA int=2, int=3)",
		},
		{
			name: "≥2 args but first not string keeps all args", code: StatusBadRequest,
			in: args{errors.New("boom"), 42}, want: "boom 42",
		},
		{
			name: "≥2 args, first non-string followed by string", code: StatusInternalServerError,
			in: args{42, "extra details"}, want: "42extra details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := NewErrorf(tt.code, tt.in...)
			require.Equal(t, tt.code, e.Code)
			require.Equal(t, tt.want, e.Message)
		})
	}
}

func Test_Test_Timeout(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/", testEmptyHandler)
	resp := apptest.Do(t, app, "/", TestConfig{Timeout: 0})
	require.Equal(t, 200, resp.StatusCode, "Status code")

	app.Get("timeout", func(_ Ctx) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})
	_, err := app.Test(httptest.NewRequest(MethodGet, "/timeout", http.NoBody), TestConfig{Timeout: 20 * time.Millisecond, FailOnTimeout: true})
	require.Error(t, err, "app.Test(req)")
}

type errorReader int

var errErrorReader = errors.New("errorReader")

func (errorReader) Read([]byte) (int, error) {
	return 0, errErrorReader
}

func Test_Test_DumpError(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", errorReader(0)))
	require.Nil(t, resp)
	require.ErrorIs(t, err, errErrorReader)
}

func Test_App_Handler(t *testing.T) {
	t.Parallel()
	h := New().Handler()
	require.Equal(t, "fasthttp.RequestHandler", reflect.TypeOf(h).String())
}

type invalidView struct{}

func (invalidView) Load() error { return errors.New("invalid view") }

func (invalidView) Render(io.Writer, string, any, ...string) error { panic("implement me") }

type countingView struct {
	loadErr error
	loads   int
}

func (v *countingView) Load() error {
	v.loads++
	return v.loadErr
}

func (*countingView) Render(io.Writer, string, any, ...string) error { return nil }

type writerView struct {
	renderErr error
}

func (*writerView) Load() error { return nil }

func (v *writerView) Render(out io.Writer, name string, binding any, layouts ...string) error {
	if v.renderErr != nil {
		return v.renderErr
	}
	bind, ok := binding.(Map)
	if !ok {
		return errors.New("unexpected binding type")
	}
	if _, err := fmt.Fprintf(out, "%s:%s:%s", name, bind["title"], strings.Join(layouts, ",")); err != nil {
		return fmt.Errorf("write rendered view: %w", err)
	}
	return nil
}

type mapViews map[string]struct{}

func (mapViews) Load() error { return nil }

func (mapViews) Render(io.Writer, string, any, ...string) error { return nil }

type sliceViews []struct{}

func (sliceViews) Load() error { return nil }

func (sliceViews) Render(io.Writer, string, any, ...string) error { return nil }

type chanViews chan struct{}

func (chanViews) Load() error { return nil }

func (chanViews) Render(io.Writer, string, any, ...string) error { return nil }

type funcViews func()

func (funcViews) Load() error { return nil }

func (funcViews) Render(io.Writer, string, any, ...string) error { return nil }

type sharedBlockingView struct {
	loadStarted   chan struct{}
	loadRelease   chan struct{}
	renderEntered chan struct{}
	blockOnLoad   int
	loads         int
}

func (v *sharedBlockingView) Load() error {
	v.loads++
	if v.loads == v.blockOnLoad {
		close(v.loadStarted)
		<-v.loadRelease
	}
	return nil
}

func (v *sharedBlockingView) Render(io.Writer, string, any, ...string) error {
	close(v.renderEntered)
	return nil
}

// appBlockingView returns a view whose Load number blockOnLoad blocks until loadRelease is closed.
func appBlockingView(blockOnLoad int) *sharedBlockingView {
	return &sharedBlockingView{
		loadStarted: make(chan struct{}), loadRelease: make(chan struct{}), renderEntered: make(chan struct{}),
		blockOnLoad: blockOnLoad,
	}
}

// appRequireRenderWaitsForReload runs reload and, once it holds view in Load, a
// GET of path through handler: the render must not start before that Load is
// released, and both must then finish, the request with a 200.
func appRequireRenderWaitsForReload(
	t *testing.T, view *sharedBlockingView, reload func() error, handler fasthttp.RequestHandler, path, msg string,
) {
	t.Helper()
	reloadDone := make(chan error, 1)
	go func() { reloadDone <- reload() }()
	<-view.loadStarted

	renderDone := make(chan int, 1)
	go func() { renderDone <- runGetHandlerRequest(handler, path) }()
	rootMiscBRequireNothing(t, view.renderEntered, msg)
	close(view.loadRelease)

	require.NoError(t, apptest.Recv(t, reloadDone, time.Second, "reload did not finish"))
	require.Equal(t, StatusOK, apptest.Recv(t, renderDone, time.Second, "render request did not finish"))
}

func runGetHandlerRequest(handler fasthttp.RequestHandler, path string) int {
	request := &fasthttp.RequestCtx{}
	request.Request.Header.SetMethod(MethodGet)
	request.Request.SetRequestURI(path)
	handler(request)
	return request.Response.StatusCode()
}

// rootMiscBRequireNothing fails the test with msg if ch delivers within 20ms: the way to show that the
// goroutine feeding it is still held back.
func rootMiscBRequireNothing(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()

	select {
	case <-ch:
		t.Fatal(msg)
	case <-time.After(20 * time.Millisecond):
	}
}

type panicLoadView struct {
	loads int
}

func (v *panicLoadView) Load() error {
	v.loads++
	if v.loads > 1 {
		panic("panic load")
	}
	return nil
}

func (*panicLoadView) Render(io.Writer, string, any, ...string) error { return nil }

type panicRenderView struct{}

func (*panicRenderView) Load() error { return nil }

func (*panicRenderView) Render(io.Writer, string, any, ...string) error { panic("panic render") }

func Test_App_ReloadViews_Success(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		subs   int  // sub apps with views of their own, mounted under the app
		parent bool // the app has views of its own
	}{
		{name: "app without mounted apps", parent: true},
		{name: "mounted apps", subs: 2},
		{name: "app with a mounted app", parent: true, subs: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var views []*countingView
			newView := func() *countingView {
				view := &countingView{}
				views = append(views, view)
				return view
			}
			var cfg Config
			if tc.parent {
				cfg.Views = newView()
			}
			app := New(cfg)
			for i := range tc.subs {
				app.Use("/"+strconv.Itoa(i), New(Config{Views: newView()}))
			}
			initialLoads := make([]int, len(views))
			for i, view := range views {
				initialLoads[i] = view.loads
			}

			for round := 1; round <= 2; round++ {
				require.NoError(t, app.ReloadViews())
				for i, view := range views {
					require.Equal(t, initialLoads[i]+round, view.loads, "view %d after reload %d", i, round)
				}
			}
		})
	}
}

func Test_App_ReloadViews_Error(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		mounted bool // the failing views belong to a mounted app
	}{
		{name: "own views"},
		{name: "mounted app's views", mounted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wantedErr := errors.New("boom")
			view := &countingView{loadErr: wantedErr}
			app := New()
			if tc.mounted {
				app.Use("/sub", New(Config{Views: view}))
			} else {
				app = New(Config{Views: view})
			}

			err := app.ReloadViews()
			require.Error(t, err)
			require.ErrorIs(t, err, wantedErr)
		})
	}
}

func Test_App_ReloadViews_BlocksRenderUntilLoadCompletes(t *testing.T) {
	t.Parallel()
	view := appBlockingView(2)
	app := New(Config{Views: view})
	app.Get("/", func(c Ctx) error { return c.Render("home", nil) })
	appRequireRenderWaitsForReload(t, view, app.ReloadViews, app.Handler(), "/", "render should wait until ReloadViews Load finishes")
}

// rootrestRecoverFrom runs fn on a new goroutine and returns what it panicked with, nil when it did not. It
// fails the test with msg if fn has not ended within a second.
func rootrestRecoverFrom(t *testing.T, fn func(), msg string) any {
	t.Helper()

	recovered := make(chan any, 1)
	go func() {
		defer func() { recovered <- recover() }()
		fn()
	}()

	return apptest.Recv(t, recovered, time.Second, msg)
}

func Test_App_ReloadViews_PanicUnlocksRender(t *testing.T) {
	t.Parallel()
	view := &panicLoadView{}
	app := New(Config{Views: view})
	app.Get("/", func(c Ctx) error { return c.Render("home", nil) })
	handler := app.Handler()

	var reloadErr error
	recovered := rootrestRecoverFrom(t, func() { reloadErr = app.ReloadViews() }, "reload panic was not recovered")
	require.NoError(t, reloadErr)
	require.Equal(t, "panic load", recovered)

	renderDone := make(chan int, 1)
	go func() { renderDone <- runGetHandlerRequest(handler, "/") }()

	require.Equal(t, StatusOK, apptest.Recv(t, renderDone, time.Second, "render request did not finish after reload panic"))
}

func Test_App_InitPanicUnlocksRouteRegistration(t *testing.T) {
	t.Parallel()
	view := &panicLoadView{}
	app := New(Config{Views: view})

	require.Equal(t, "panic load", rootrestRecoverFrom(t, func() { app.init() }, "init panic was not recovered"))

	registerDone := make(chan struct{}, 1)
	go func() {
		app.Get("/after-panic", func(Ctx) error { return nil })
		registerDone <- struct{}{}
	}()

	apptest.Recv(t, registerDone, time.Second, "route registration deadlocked after init panic")
}

func Test_App_RenderPanicUnlocksReloadViews(t *testing.T) {
	t.Parallel()
	app := New(Config{Views: &panicRenderView{}})
	app.Get("/", func(c Ctx) error { return c.Render("home", nil) })

	recovered := rootrestRecoverFrom(t, func() {
		app.startupProcess()

		request := &fasthttp.RequestCtx{}
		request.Request.Header.SetMethod(MethodGet)
		request.URI().SetPath("/")

		app.defaultRequestHandler(request)
	}, "render panic was not recovered")
	require.Equal(t, "panic render", recovered)

	reloadDone := make(chan error, 1)
	go func() { reloadDone <- app.ReloadViews() }()

	require.NoError(t, apptest.Recv(t, reloadDone, time.Second, "reload did not finish after render panic"))
}

func Test_App_ReloadViews_InterfaceNilPointer(t *testing.T) {
	t.Parallel()
	var view *countingView
	app := &App{config: Config{Views: view}}

	require.ErrorIs(t, app.ReloadViews(), ErrNoViewEngineConfigured)
}

func Test_App_Render_WritesConfiguredView(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		want    string
		layouts []string
	}{
		{name: "default layout", want: "partial:Fiber:base"},
		{name: "explicit layout", layouts: []string{"override"}, want: "partial:Fiber:override"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			app := New(Config{Views: &writerView{}, ViewsLayout: "base"})
			var out bytes.Buffer

			require.NoError(t, app.Render(&out, "partial", Map{"title": "Fiber"}, testCase.layouts...))
			require.Equal(t, testCase.want, out.String())
		})
	}
}

func Test_App_Render_ReturnsViewErrors(t *testing.T) {
	t.Parallel()
	t.Run("typed nil view engine", func(t *testing.T) {
		t.Parallel()
		var view *writerView
		app := &App{config: Config{Views: view}}

		require.ErrorIs(t, app.Render(io.Discard, "partial", nil), ErrNoViewEngineConfigured)
	})

	t.Run("render failure", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("render failed")
		app := New(Config{Views: &writerView{renderErr: wantErr}})

		require.ErrorIs(t, app.Render(io.Discard, "partial", Map{}), wantErr)
	})
}

func Test_App_RenderAndReloadViews_ReturnNoViewEngineForNilViews(t *testing.T) {
	t.Parallel()
	var (
		pointerView  *writerView
		mapView      mapViews
		sliceView    sliceViews
		channelView  chanViews
		functionView funcViews
	)

	testCases := []struct {
		view Views
		name string
	}{
		{name: "nil interface"},
		{name: "pointer", view: pointerView},
		{name: "map", view: mapView},
		{name: "slice", view: sliceView},
		{name: "channel", view: channelView},
		{name: "function", view: functionView},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			app := New(Config{Views: testCase.view})

			renderErr := app.Render(io.Discard, "partial", nil)
			reloadErr := app.ReloadViews()

			require.ErrorIs(t, renderErr, ErrNoViewEngineConfigured)
			require.ErrorIs(t, reloadErr, ErrNoViewEngineConfigured)
		})
	}
}

func Test_isNilViews_ReturnsTrueForNilUnsafePointer(t *testing.T) {
	t.Parallel()
	var view unsafe.Pointer

	require.True(t, isNilViews(view))
}

func Test_App_Render_WaitsForReloadViews(t *testing.T) {
	t.Parallel()
	view := appBlockingView(2)
	app := New(Config{Views: view})
	view.loads = 1

	reloadDone := make(chan error, 1)
	go func() { reloadDone <- app.ReloadViews() }()

	<-view.loadStarted

	renderDone := make(chan error, 1)
	go func() { renderDone <- app.Render(io.Discard, "partial", nil) }()

	rootMiscBRequireNothing(t, view.renderEntered, "Render should wait until ReloadViews Load finishes")

	close(view.loadRelease)

	require.NoError(t, apptest.Recv(t, reloadDone, time.Second, "reload did not finish"))
	require.NoError(t, apptest.Recv(t, renderDone, time.Second, "Render did not finish"))
}

func Test_App_ReloadViews_MountedViews(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	templatePath := filepath.Join(tempDir, "template.html")

	require.NoError(t, os.WriteFile(templatePath, []byte("before"), 0o600))

	view := &fileView{path: templatePath}
	subApp := New(Config{Views: view})
	app := New()
	app.Use("/sub", subApp)

	require.NoError(t, view.Load())
	initialLoads := view.loads
	require.Equal(t, "before", view.content)

	require.NoError(t, os.WriteFile(templatePath, []byte("after"), 0o600))
	require.NoError(t, app.ReloadViews())

	require.Equal(t, "after", view.content)
	require.Greater(t, view.loads, initialLoads)
}

func Test_App_ReloadViews_MountedViews_ConcurrentMount(t *testing.T) {
	t.Parallel()
	view := &countingView{}
	subApp := New()
	app := New(Config{Views: view})
	app.Use("/sub", subApp)

	// The walk down the mount tree reads every app's mount metadata, so it has
	// to take each app's own lock rather than only the one it started from.
	initialLoads := view.loads

	var (
		wg        sync.WaitGroup
		reloadErr error
	)

	wg.Go(func() {
		for range 200 {
			if err := app.ReloadViews(); err != nil {
				reloadErr = err
				return
			}
		}
	})

	wg.Go(func() {
		for range 200 {
			subApp.Use("/nested", New())
		}
	})

	wg.Wait()
	require.NoError(t, reloadErr)
	require.Equal(t, initialLoads+200, view.loads)
}

func Test_App_ReloadViews_MountedViews_SharedEngineBlocksSiblingRender(t *testing.T) {
	t.Parallel()
	view := appBlockingView(3)
	subAppA := New(Config{Views: view})
	subAppB := New(Config{Views: view})
	subAppB.Get("/render", func(c Ctx) error { return c.Render("home", nil) })
	app := New()
	app.Use("/a", subAppA)
	app.Use("/b", subAppB)
	appRequireRenderWaitsForReload(t, view, subAppA.ReloadViews, app.Handler(), "/b/render",
		"render should wait until shared view engine reload finishes")
}

func Test_App_ReloadViews_SharedEngineBlocksRenderAfterReusableMount(t *testing.T) {
	t.Parallel()
	view := appBlockingView(3)
	reusableSubApp := New(Config{Views: view})
	reusableSubApp.Get("/render", func(c Ctx) error { return c.Render("home", nil) })
	siblingApp := New(Config{Views: view})
	parentAppA := New()
	parentAppA.Use("/shared", reusableSubApp)
	parentAppA.Use("/sibling", siblingApp)
	parentAppB := New()
	parentAppB.Use("/shared", reusableSubApp)
	appRequireRenderWaitsForReload(t, view, siblingApp.ReloadViews, parentAppA.Handler(), "/shared/render",
		"render should wait until shared view engine reload finishes")
}

func Test_App_Init_Error_View(t *testing.T) {
	t.Parallel()
	app := New(Config{Views: invalidView{}})

	require.PanicsWithValue(t, "implement me", func() {
		//nolint:errcheck // not needed
		_ = app.config.Views.Render(nil, "", nil)
	})
}

func Test_App_Stack(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use("/path0", testEmptyHandler)
	app.Get("/path1", testEmptyHandler)
	app.Get("/path2", testEmptyHandler)
	app.Post("/path3", testEmptyHandler)
	app.Query("/path4", testEmptyHandler)

	app.startupProcess()

	stack := app.Stack()
	methodList := app.config.RequestMethods
	require.Len(t, methodList, len(stack))
	require.Len(t, stack[app.methodInt(MethodGet)], 3)
	require.Len(t, stack[app.methodInt(MethodHead)], 3)
	require.Len(t, stack[app.methodInt(MethodPost)], 2)
	require.Len(t, stack[app.methodInt(MethodPut)], 1)
	require.Len(t, stack[app.methodInt(MethodPatch)], 1)
	require.Len(t, stack[app.methodInt(MethodDelete)], 1)
	require.Len(t, stack[app.methodInt(MethodConnect)], 1)
	require.Len(t, stack[app.methodInt(MethodOptions)], 1)
	require.Len(t, stack[app.methodInt(MethodTrace)], 1)
	require.Len(t, stack[app.methodInt(MethodQuery)], 2)
}

func Test_App_HandlersCount(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use("/path0", testEmptyHandler)
	app.Get("/path2", testEmptyHandler)
	app.Post("/path3", testEmptyHandler)

	app.startupProcess()

	count := app.HandlersCount()
	require.Equal(t, uint32(4), count)
}

func Test_App_RefusedRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		config Config
		name   string
		route  string
		raw    string // what the client writes to the server
		want   string // what the answer has to contain
	}{
		{
			name:   "read timeout",
			config: Config{ReadTimeout: time.Nanosecond, IdleTimeout: time.Minute, DisableKeepalive: true},
			route:  "/read-timeout", raw: "HEAD /read-timeout HTTP/1.1\r\n", want: "408 Request Timeout",
		},
		{name: "bad request", route: "/bad-request", raw: "BadRequest\r\n", want: "400 Bad Request"},
		{
			name: "small read buffer", config: Config{ReadBufferSize: 1}, route: "/small-read-buffer",
			raw: "GET /small-read-buffer HTTP/1.1\r\nHost: example.com\r\n\r\n", want: "431 Request Header Fields Too Large",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New(tc.config)

			app.Get(tc.route, func(c Ctx) error { return c.SendString("I should not be sent") })

			ln := newLocalListener(t)
			addr := ln.Addr().String()

			go func() {
				time.Sleep(500 * time.Millisecond)

				conn, err := net.Dial(NetworkTCP4, addr)
				assert.NoError(t, err)
				defer func(conn net.Conn) {
					closeErr := conn.Close()
					assert.NoError(t, closeErr)
				}(conn)

				_, err = conn.Write([]byte(tc.raw))
				assert.NoError(t, err)

				buf := make([]byte, 1024)
				var n int
				n, err = conn.Read(buf)

				assert.NoError(t, err)
				assert.True(t, bytes.Contains(buf[:n], []byte(tc.want)))

				assert.NoError(t, app.Shutdown())
			}()

			require.NoError(t, app.Listener(ln, ListenConfig{DisableStartupMessage: true}))
		})
	}
}

func Test_App_Server(t *testing.T) {
	t.Parallel()
	app := New()

	require.NotNil(t, app.Server())
}

func Test_App_Error_In_Fasthttp_Server(t *testing.T) {
	app := New()
	app.config.ErrorHandler = func(_ Ctx, _ error) error { return errors.New("fake error") }
	app.server.GetOnly = true

	apptest.Expect(t, app, apptest.Req(MethodPost, "/"), 500)
}

func Test_App_New_Test_Parallel(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"Test_App_New_Test_Parallel_1", "Test_App_New_Test_Parallel_2"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			apptest.Do(t, New(Config{Immutable: true}), "/")
		})
	}
}

func Test_App_ReadBodyStream(t *testing.T) {
	t.Parallel()
	app := New(Config{StreamRequestBody: true})
	app.Post("/", func(c Ctx) error {
		// Calling c.Body() automatically reads the entire stream.
		return c.SendString(fmt.Sprintf("%v %s", c.Request().IsBodyStream(), c.Body()))
	})
	testString := "this is a test"
	resp := apptest.Do(t, app, httptest.NewRequest(MethodPost, "/", bytes.NewBufferString(testString)))
	require.Equal(t, "true "+testString, apptest.Body(t, resp))
}

func Test_App_ReadBodyStream_BodyLimit(t *testing.T) {
	t.Parallel()
	app := New(Config{StreamRequestBody: true, BodyLimit: 1024})
	app.Post("/", func(c Ctx) error { return c.SendString(strconv.Itoa(len(c.Body()))) })
	// fasthttpadaptor buffers the body for the http.Request.
	app.Post("/http", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, err := io.Copy(io.Discard, r.Body)
		assert.NoError(t, err)
		_, err = w.Write([]byte(strconv.FormatInt(n, 10)))
		assert.NoError(t, err)
	}))
	// Like logger's ${body}, a body read after the handler has drained the stream must not change its answer.
	app.Use("/stream", func(c Ctx) error {
		err := c.Next()
		c.Body()
		return err
	})
	app.Post("/stream", func(c Ctx) error {
		n, err := io.Copy(io.Discard, c.BodyStream())
		if err != nil {
			return fmt.Errorf("failed to read: %w", err)
		}
		return c.Status(StatusCreated).SendString(strconv.FormatInt(n, 10))
	})

	const refused = StatusRequestEntityTooLarge
	for _, tc := range []struct {
		name    string
		target  string
		size    int
		status  int
		chunked bool
	}{
		{name: "declared", target: "/", size: 1024, status: StatusOK},
		{name: "chunked", target: "/", size: 1024, chunked: true, status: StatusOK},
		{name: "declared over the limit", target: "/", size: 1025, status: refused},
		{name: "chunked over the limit", target: "/", size: 1025, chunked: true, status: refused},
		{name: "BodyStream is not limited", target: "/stream", size: 4096, status: StatusCreated},
		{name: "chunked BodyStream is not limited", target: "/stream", size: 4096, chunked: true, status: StatusCreated},
		{name: "http.Handler, declared", target: "/http", size: 1024, status: StatusOK},
		{name: "http.Handler, chunked", target: "/http", size: 1024, chunked: true, status: StatusOK},
		{name: "http.Handler, declared over the limit", target: "/http", size: 1025, status: refused},
		{name: "http.Handler, chunked over the limit", target: "/http", size: 1025, chunked: true, status: refused},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(MethodPost, tc.target, strings.NewReader(strings.Repeat("a", tc.size)))
			if tc.chunked {
				req.TransferEncoding = []string{"chunked"}
			}
			if tc.status == refused {
				apptest.Expect(t, app, req, tc.status)
				return
			}
			apptest.ExpectBody(t, app, req, tc.status, strconv.Itoa(tc.size))
		})
	}
}

func Test_App_DisablePreParseMultipartForm(t *testing.T) {
	t.Parallel()
	// Must be used with both otherwise there is no point.
	testString := "this is a test"

	app := New(Config{DisablePreParseMultipartForm: true, StreamRequestBody: true})
	app.Post("/", func(c Ctx) error {
		req := c.Request()
		mpf, err := req.MultipartForm()
		if err != nil {
			return err
		}
		if !req.IsBodyStream() {
			return errors.New("not a body stream")
		}
		file, err := mpf.File["test"][0].Open()
		if err != nil {
			return fmt.Errorf("failed to open: %w", err)
		}
		buffer := make([]byte, len(testString))
		n, err := file.Read(buffer)
		if err != nil {
			return fmt.Errorf("failed to read: %w", err)
		}
		if n != len(testString) {
			return errors.New("bad read length")
		}
		return c.Send(buffer)
	})
	body, contentType := ctxTestMultipartFile(t, "test", "test", []byte(testString))
	req := httptest.NewRequest(MethodPost, "/", bytes.NewReader(body))
	req.Header.Set(HeaderContentType, contentType)
	resp := apptest.Do(t, app, req)
	require.Equal(t, testString, apptest.Body(t, resp))
}

// With StreamRequestBody the server must not pre-parse a multipart body of any length before the
// handlers run: FormFile parses it on demand within BodyLimit.
func Test_App_StreamRequestBody_MultipartBodyLimit(t *testing.T) {
	t.Parallel()
	const limit = 64 * 1024
	app := New(Config{StreamRequestBody: true, BodyLimit: limit})
	app.Post("/", func(c Ctx) error {
		file, err := c.FormFile("test")
		if errors.Is(err, fasthttp.ErrBodyTooLarge) {
			return ErrRequestEntityTooLarge
		}
		if err != nil {
			return err
		}
		return c.SendString(strconv.FormatInt(file.Size, 10))
	})

	for _, tc := range []struct {
		name   string
		size   int
		status int
	}{
		{name: "within limit", size: limit / 2, status: StatusOK},
		{name: "over limit", size: 10 * limit, status: StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body, contentType := ctxTestMultipartFile(t, "test", "test", make([]byte, tc.size))
			req := httptest.NewRequest(MethodPost, "/", bytes.NewReader(body))
			req.Header.Set(HeaderContentType, contentType)
			apptest.Expect(t, app, req, tc.status)
		})
	}
}

func Test_App_Test_no_timeout_infinitely(t *testing.T) {
	t.Parallel()
	var err error
	c := make(chan int)

	go func() {
		defer func() { c <- 0 }()
		app := New()
		app.Get("/", func(_ Ctx) error {
			runtime.Goexit()
			return nil
		})

		req := httptest.NewRequest(MethodGet, "/", http.NoBody)
		_, err = app.Test(req, TestConfig{Timeout: 0})
	}()

	tk := time.NewTimer(5 * time.Second)
	defer tk.Stop()

	select {
	case <-tk.C:
		t.Error("hanging test")
		t.FailNow()
	case <-c:
	}

	if err == nil {
		t.Error("unexpected success request")
		t.FailNow()
	}
}

func Test_App_Test_timeout(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		method  string
		delay   time.Duration // how long the handler takes
		timeout time.Duration
	}{
		{name: "GET", method: MethodGet, delay: time.Second, timeout: 100 * time.Millisecond},
		{name: "small timeout", method: MethodPost, delay: 100 * time.Millisecond, timeout: 10 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Add([]string{tc.method}, "/", func(c Ctx) error {
				time.Sleep(tc.delay)
				return c.SendString("late")
			})

			// With FailOnTimeout the timeout is an error, however short it is.
			_, err := app.Test(httptest.NewRequest(tc.method, "/", http.NoBody), TestConfig{Timeout: tc.timeout, FailOnTimeout: true})
			require.ErrorIs(t, err, os.ErrDeadlineExceeded)
		})
	}
}

func Test_App_Test_timeout_empty_response(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(_ Ctx) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	// When FailOnTimeout is false, the test should return whatever response is available
	// at timeout without failing
	resp := apptest.Do(t, app, "/", TestConfig{Timeout: 10 * time.Millisecond, FailOnTimeout: false})
	require.Equal(t, StatusOK, resp.StatusCode)
}

func Test_App_Test_drop_empty_response(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error { return c.Drop() })
	_, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody), TestConfig{Timeout: 0, FailOnTimeout: false})
	require.ErrorIs(t, err, ErrTestGotEmptyResponse)
}

// rootMiscBStubReadResponse replaces httpReadResponse until the test ends. Every app.Test reads that
// package-level variable, so a test that calls this must not be parallel.
func rootMiscBStubReadResponse(t *testing.T, stub func(*bufio.Reader, *http.Request) (*http.Response, error)) {
	t.Helper()

	previous := httpReadResponse
	t.Cleanup(func() { httpReadResponse = previous })
	httpReadResponse = stub
}

func Test_App_Test_response_error(t *testing.T) {
	// Note: Test cannot run in parallel due to
	// overriding the httpReadResponse global variable.
	// t.Parallel()

	rootMiscBStubReadResponse(t, func(_ *bufio.Reader, _ *http.Request) (*http.Response, error) {
		return nil, errErrorReader
	})

	app := New()
	app.Get("/", func(c Ctx) error { return c.SendStatus(StatusOK) })
	_, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody), TestConfig{Timeout: 0, FailOnTimeout: false})
	require.ErrorIs(t, err, errErrorReader)
}

type errorReadCloser int

var errInvalidReadOnBody = errors.New("test: invalid Read on body")

func (errorReadCloser) Read(_ []byte) (int, error) {
	return 0, errInvalidReadOnBody
}

func (errorReadCloser) Close() error {
	return nil
}

func Test_App_Test_ReadFail(t *testing.T) {
	// Note: Test cannot run in parallel due to
	// overriding the httpReadResponse global variable.
	// t.Parallel()

	rootMiscBStubReadResponse(t, func(r *bufio.Reader, req *http.Request) (*http.Response, error) {
		resp, err := http.ReadResponse(r, req)
		require.NoError(t, resp.Body.Close())
		resp.Body = errorReadCloser(0)
		return resp, err //nolint:wrapcheck // unnecessary to wrap it
	})

	app := New()
	hints := []string{"<https://cdn.com>; rel=preload; as=script"}
	app.Get("/early", func(c Ctx) error {
		require.NoError(t, c.SendEarlyHints(hints))
		return c.SendStatus(StatusOK)
	})

	req := httptest.NewRequest(MethodGet, "/early", http.NoBody)
	_, err := app.Test(req)

	require.ErrorIs(t, err, errInvalidReadOnBody)
}

var errDoubleClose = errors.New("test: double close")

type doubleCloseBody struct {
	isClosed bool
}

func (b *doubleCloseBody) Read(_ []byte) (int, error) {
	if b.isClosed {
		return 0, errInvalidReadOnBody
	}

	// Close after reading EOF
	_ = b.Close() //nolint:errcheck // It is fine to ignore the error here
	return 0, io.EOF
}

func (b *doubleCloseBody) Close() error {
	if b.isClosed {
		return errDoubleClose
	}

	b.isClosed = true
	return nil
}

func Test_App_Test_CloseFail(t *testing.T) {
	// Note: Test cannot run in parallel due to
	// overriding the httpReadResponse global variable.
	// t.Parallel()

	rootMiscBStubReadResponse(t, func(r *bufio.Reader, req *http.Request) (*http.Response, error) {
		resp, err := http.ReadResponse(r, req)
		_ = resp.Body.Close() //nolint:errcheck // It is fine to ignore the error here
		resp.Body = &doubleCloseBody{}
		return resp, err //nolint:wrapcheck // unnecessary to wrap it
	})

	app := New()
	hints := []string{"<https://cdn.com>; rel=preload; as=script"}
	app.Get("/early", func(c Ctx) error {
		require.NoError(t, c.SendEarlyHints(hints))
		return c.Status(StatusOK).SendString("done")
	})

	req := httptest.NewRequest(MethodGet, "/early", http.NoBody)
	_, err := app.Test(req)

	require.ErrorIs(t, err, errDoubleClose)
}

func Test_App_SetTLSHandler(t *testing.T) {
	t.Parallel()
	tlsHandler := &TLSHandler{}
	_, err := tlsHandler.GetClientInfo(&tls.ClientHelloInfo{ServerName: "example.golang"})
	require.NoError(t, err)

	app := New()
	app.SetTLSHandler(tlsHandler)

	c := apptest.Ctx(t, app)

	require.Equal(t, "example.golang", c.ClientHelloInfo().ServerName)
}

func Test_App_AddCustomRequestMethod(t *testing.T) {
	t.Parallel()
	methods := append(DefaultMethods, "TEST") //nolint:gocritic // We want a new slice here
	app := New(Config{RequestMethods: methods})
	appMethods := app.config.RequestMethods

	// method name is always uppercase - https://datatracker.ietf.org/doc/html/rfc7231#section-4.1
	require.Len(t, app.stack, len(appMethods))
	require.Equal(t, "TEST", appMethods[len(appMethods)-1])
}

func Test_App_GetRoutes(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error { return c.Next() })
	handler := func(c Ctx) error { return c.SendStatus(StatusOK) }
	app.Delete("/delete", handler).Name("delete")
	app.Post("/post", handler).Name("post")
	routes := app.GetRoutes(false)
	require.Len(t, routes, 2+len(app.config.RequestMethods))
	methodMap := map[string]string{"/delete": "delete", "/post": "post"}
	for _, route := range routes {
		name, ok := methodMap[route.Path]
		if ok {
			require.Equal(t, name, route.Name)
		}
	}

	routes = app.GetRoutes(true)
	require.Len(t, routes, 2)
	for _, route := range routes {
		name, ok := methodMap[route.Path]
		require.True(t, ok)
		require.Equal(t, name, route.Name)
	}
}

func Test_Middleware_Route_Naming_With_Use(t *testing.T) {
	t.Parallel()
	named := "named"
	app := New()
	app.Get("/unnamed", func(c Ctx) error { return c.Next() })
	app.Post("/named", func(c Ctx) error { return c.Next() }).Name(named)
	app.Use(func(c Ctx) error { return c.Next() }) // no name - logging MW
	app.Use(func(c Ctx) error { return c.Next() }).Name("corsMW")
	app.Use(func(c Ctx) error { return c.Next() }).Name("compressMW")
	app.Use(func(c Ctx) error { return c.Next() }) // no name - cache MW
	grp := app.Group("/pages").Name("pages.")
	grp.Use(func(c Ctx) error { return c.Next() }).Name("csrfMW")
	grp.Get("/home", func(c Ctx) error { return c.Next() }).Name("home")
	grp.Get("/unnamed", func(c Ctx) error { return c.Next() })

	for _, route := range app.GetRoutes() {
		switch route.Path {
		case "/":
			require.Equal(t, "compressMW", route.Name)
		case "/unnamed", "/pages/unnamed":
			require.Empty(t, route.Name)
		case "/named":
			require.Equal(t, named, route.Name)
		case "/pages":
			require.Equal(t, "pages.csrfMW", route.Name)
		case "/pages/home":
			require.Equal(t, "pages.home", route.Name)
		default:
			t.Errorf("unknown route: %s", route.Path)
		}
	}
}

func Test_Route_Naming_Issue_2671_2685(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", emptyHandler).Name("index")
	require.Equal(t, "/", app.GetRoute("index").Path)
	app.Get("/a/:a_id", emptyHandler).Name("a")
	require.Equal(t, "/a/:a_id", app.GetRoute("a").Path)
	app.Post("/b/:bId", emptyHandler).Name("b")
	require.Equal(t, "/b/:bId", app.GetRoute("b").Path)

	c := app.Group("/c")
	c.Get("", emptyHandler).Name("c.get")
	require.Equal(t, "/c", app.GetRoute("c.get").Path)
	c.Post("", emptyHandler).Name("c.post")
	require.Equal(t, "/c", app.GetRoute("c.post").Path)
	c.Get("/d", emptyHandler).Name("c.get.d")
	require.Equal(t, "/c/d", app.GetRoute("c.get.d").Path)

	d := app.Group("/d/:d_id")
	d.Get("", emptyHandler).Name("d.get")
	require.Equal(t, "/d/:d_id", app.GetRoute("d.get").Path)
	d.Post("", emptyHandler).Name("d.post")
	require.Equal(t, "/d/:d_id", app.GetRoute("d.post").Path)

	e := app.Group("/e/:eId")
	e.Get("", emptyHandler).Name("e.get")
	require.Equal(t, "/e/:eId", app.GetRoute("e.get").Path)
	e.Post("", emptyHandler).Name("e.post")
	require.Equal(t, "/e/:eId", app.GetRoute("e.post").Path)
	e.Get("f", emptyHandler).Name("e.get.f")
	require.Equal(t, "/e/:eId/f", app.GetRoute("e.get.f").Path)

	postGroup := app.Group("/post/:postId")
	postGroup.Get("", emptyHandler).Name("post.get")
	require.Equal(t, "/post/:postId", app.GetRoute("post.get").Path)
	postGroup.Post("", emptyHandler).Name("post.update")
	require.Equal(t, "/post/:postId", app.GetRoute("post.update").Path)

	// routes with the same path on different methods
	app.Get("/users", emptyHandler).Name("get-users")
	app.Post("/users", emptyHandler).Name("add-user")
	require.Equal(t, "/users", app.GetRoute("get-users").Path)
	require.Equal(t, "/users", app.GetRoute("add-user").Path)

	// the same in a group
	newGrp := app.Group("/name-test")
	newGrp.Get("/users", emptyHandler).Name("grp-get-users")
	newGrp.Post("/users", emptyHandler).Name("grp-add-user")
	require.Equal(t, "/name-test/users", app.GetRoute("grp-get-users").Path)
	require.Equal(t, "/name-test/users", app.GetRoute("grp-add-user").Path)

	// HEAD route naming
	app.Get("/simple-route", emptyHandler).Name("simple-route")
	app.Head("/simple-route", emptyHandler).Name("simple-route2")
	require.Equal(t, "/simple-route", app.GetRoute("simple-route").Path)
	require.Equal(t, "/simple-route", app.GetRoute("simple-route2").Path)
}

func Test_App_State(t *testing.T) {
	t.Parallel()
	app := New()

	app.State().Set("key", "value")
	str, ok := app.State().GetString("key")
	require.True(t, ok)
	require.Equal(t, "value", str)
}

func Benchmark_Communication_Flow(b *testing.B) {
	app := New()
	app.Get("/", func(c Ctx) error { return c.SendString("Hello, World!") })
	h := app.Handler()
	fctx := routerTestRequestCtx(MethodGet, "/")
	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	require.Equal(b, 200, fctx.Response.Header.StatusCode())
	require.Equal(b, "Hello, World!", string(fctx.Response.Body()))
}

func Benchmark_Communication_Flow_Parallel(b *testing.B) {
	app := New()
	app.Get("/", func(c Ctx) error { return c.SendString("Hello, World!") })
	h := app.Handler()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		fctx := routerTestRequestCtx(MethodGet, "/")
		for pb.Next() {
			h(fctx)
		}
	})

	verifyCtx := routerTestRequestCtx(MethodGet, "/")
	h(verifyCtx)
	require.Equal(b, 200, verifyCtx.Response.Header.StatusCode())
	require.Equal(b, "Hello, World!", string(verifyCtx.Response.Body()))
}

func Benchmark_Ctx_AcquireReleaseFlow(b *testing.B) {
	app := New()

	fctx := &fasthttp.RequestCtx{}

	b.Run("withoutRequestCtx", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			c, _ := app.AcquireCtx(fctx).(*DefaultCtx) //nolint:errcheck // not needed
			app.ReleaseCtx(c)
		}
	})

	b.Run("withRequestCtx", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			c, _ := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck // not needed
			app.ReleaseCtx(c)
		}
	})
}

func acquireDefaultCtxForAppBenchmark(b *testing.B, app *App, fctx *fasthttp.RequestCtx) *DefaultCtx {
	b.Helper()

	ctx := app.AcquireCtx(fctx)
	defaultCtx, ok := ctx.(*DefaultCtx)
	if !ok {
		b.Fatal("AcquireCtx did not return *DefaultCtx")
	}
	return defaultCtx
}

func Benchmark_Ctx_AcquireReleaseFlow_Parallel(b *testing.B) {
	app := New()

	b.Run("withoutRequestCtx", func(b *testing.B) {
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			fctx := &fasthttp.RequestCtx{}
			for pb.Next() {
				c := acquireDefaultCtxForAppBenchmark(b, app, fctx)
				app.ReleaseCtx(c)
			}
		})
	})

	b.Run("withRequestCtx", func(b *testing.B) {
		b.ReportAllocs()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				c := acquireDefaultCtxForAppBenchmark(b, app, &fasthttp.RequestCtx{})
				app.ReleaseCtx(c)
			}
		})
	})
}

func TestErrorHandler_PicksRightOne(t *testing.T) {
	t.Parallel()
	// common handler to be used by all routes,
	// it will always fail by returning an error since
	// we need to test that the right ErrorHandler is invoked
	handler := func(_ Ctx) error { return errors.New("random error") }

	// subapp /api/v1/users [no custom error handler]
	appAPIV1Users := New()
	appAPIV1Users.Get("/", handler)

	// subapp /api/v1/use [with custom error handler]
	appAPIV1UseEH := func(c Ctx, _ error) error { return c.SendString("/api/v1/use error handler") }
	appAPIV1Use := New(Config{ErrorHandler: appAPIV1UseEH})
	appAPIV1Use.Get("/", handler)

	// subapp: /api/v1 [with custom error handler]
	appV1EH := func(c Ctx, _ error) error { return c.SendString("/api/v1 error handler") }
	appV1 := New(Config{ErrorHandler: appV1EH})
	appV1.Get("/", handler)
	appV1.Use("/users", appAPIV1Users)
	appV1.Use("/use", appAPIV1Use)

	// root app [no custom error handler]
	app := New()
	app.Get("/", handler)
	app.Use("/api/v1", appV1)

	testCases := []struct {
		path     string // the endpoint url to test
		expected string // the expected error response
	}{
		// /api/v1/users mount doesn't have custom ErrorHandler
		// so it should use the upper-nearest one (/api/v1)
		{"/api/v1/users", "/api/v1 error handler"},

		// /api/v1/use mount has a custom ErrorHandler
		{"/api/v1/use", "/api/v1/use error handler"},

		// /api/v1 mount has a custom ErrorHandler
		{"/api/v1", "/api/v1 error handler"},

		// / mount doesn't have custom ErrorHandler, since is
		// the root path i will use Fiber's default Error Handler
		{"/", "random error"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.path, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, testCase.expected, apptest.Body(t, apptest.Do(t, app, testCase.path)))
		})
	}
}

type groupIDResponse struct {
	GroupID string `json:"group_id"`
}

// Test for the reported bug where Test method returns "test: got empty response" error
// when using a very small timeout value (e.g., 5 microseconds instead of 5 seconds).
// With FailOnTimeout: false, the test should return whatever response is available without error.
func Test_App_Test_SmallTimeout_WithFailOnTimeoutFalse(t *testing.T) {
	t.Parallel()
	app := New()
	app.Post("/admin/api/groups", func(c Ctx) error {
		// Add a small delay to ensure timeout is triggered
		time.Sleep(10 * time.Millisecond)
		groupID := "g.test123"
		return c.JSON(groupIDResponse{GroupID: groupID})
	})

	req := httptest.NewRequest(MethodPost, "/admin/api/groups", http.NoBody)

	// Using 5 microseconds which is too short for the handler to complete
	// But with FailOnTimeout: false, it should return whatever is available without error
	resp, err := app.Test(req, TestConfig{Timeout: 5 * time.Microsecond, FailOnTimeout: false})

	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var response groupIDResponse
	require.NoError(t, json.Unmarshal(apptest.BodyBytes(t, resp), &response))
	require.NotEmpty(t, response.GroupID)
	require.Equal(t, "g.test123", response.GroupID)
}

func Test_App_ShutdownWithTimeout_HandlerTakesAppMutex(t *testing.T) {
	t.Parallel()
	app := New()
	inHandler := make(chan struct{})
	shutdownStarted := make(chan struct{})
	app.Get("/", func(c Ctx) error {
		close(inHandler)
		<-shutdownStarted
		app.RebuildTree()
		return c.SendString("ok")
	})
	app.Hooks().OnPreShutdown(func() error {
		close(shutdownStarted)
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	errs := make(chan error, 1)
	go func() { errs <- app.Listener(ln, ListenConfig{DisableStartupMessage: true}) }()

	rootMiscBSendOnNewConn(t, ln, "GET / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n")

	apptest.Recv(t, inHandler, 2*time.Second, "request never reached the handler")

	const timeout = 3 * time.Second
	start := time.Now()
	require.NoError(t, app.ShutdownWithTimeout(timeout))
	require.Less(t, time.Since(start), timeout, "shutdown waited for the whole timeout: the handler deadlocked on the app mutex")
	require.NoError(t, <-errs)
}

func Test_App_IsProxyTrusted_CanonicalIPForms(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		proxy  string
		remote net.IP
	}{
		{name: "uppercase IPv6", proxy: "2001:DB8::1", remote: net.ParseIP("2001:db8::1")},
		{name: "zero-padded IPv6", proxy: "2001:0db8::1", remote: net.ParseIP("2001:db8::1")},
		{name: "uncompressed IPv6", proxy: "0:0:0:0:0:0:0:1", remote: net.ParseIP("::1")},
		{name: "IPv4-mapped IPv6 with 4-byte peer", proxy: "::ffff:10.0.0.1", remote: net.ParseIP("10.0.0.1").To4()},
		{name: "IPv4-mapped IPv6 with 16-byte peer", proxy: "::ffff:10.0.0.1", remote: net.ParseIP("10.0.0.1")},
		{name: "IPv4 with IPv4-mapped peer", proxy: "10.0.0.1", remote: net.ParseIP("::ffff:10.0.0.1")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New(Config{
				TrustProxy:       true,
				TrustProxyConfig: TrustProxyConfig{Proxies: []string{tc.proxy}},
			}))
			c.RequestCtx().SetRemoteAddr(&net.TCPAddr{IP: tc.remote, Port: 8080})

			require.True(t, c.IsProxyTrusted())
		})
	}
}

func Test_App_IP_StripTrustedProxies_CanonicalIPForms(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		proxy    string
		remote   string
		header   string
		expected string
	}{
		{name: "IPv4-mapped proxy spelling", proxy: "::ffff:10.0.0.1", remote: "10.0.0.1", header: "203.0.113.50, 10.0.0.1", expected: "203.0.113.50"},
		{name: "uppercase IPv6 proxy", proxy: "2001:DB8::1", remote: "2001:db8::1", header: "203.0.113.50, 2001:db8::1", expected: "203.0.113.50"},
		{name: "uncompressed IPv6 proxy", proxy: "0:0:0:0:0:0:0:1", remote: "::1", header: "203.0.113.50, ::1", expected: "203.0.113.50"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New(forwardedForProxies(tc.proxy))
			fastCtx := &fasthttp.RequestCtx{}
			fastCtx.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP(tc.remote), Port: 8080})
			c := apptest.CtxFor(t, app, fastCtx)
			c.Request().Header.Set(HeaderXForwardedFor, tc.header)

			require.Equal(t, tc.expected, c.IP())
		})
	}
}

func Test_App_ErrorHandler_SkipsCommittedTimeoutResponse(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	app := New(Config{
		ErrorHandler: func(c Ctx, err error) error {
			calls.Add(1)
			return c.Status(StatusTeapot).SendString(err.Error())
		},
	})

	rctx := &fasthttp.RequestCtx{}
	rctx.TimeoutErrorWithCode(ErrRequestTimeout.Message, StatusRequestTimeout)
	c := apptest.CtxFor(t, app, rctx)

	require.NoError(t, app.ErrorHandler(c, ErrRequestTimeout))
	require.Equal(t, int32(0), calls.Load())
	require.Equal(t, StatusOK, c.Response().StatusCode())
	require.Empty(t, c.Response().Body())

	// Without a committed response the configured handler runs as usual.
	plain := apptest.Ctx(t, app)
	require.NoError(t, app.ErrorHandler(plain, ErrRequestTimeout))
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, StatusTeapot, plain.Response().StatusCode())
}

// Test_App_Shutdown_HookRegistrationNoRace registers shutdown hooks while a
// shutdown is running; the drain no longer holds the app mutex, so the hook
// slices must be snapshotted rather than iterated in place.
func Test_App_Shutdown_HookRegistrationNoRace(t *testing.T) {
	t.Parallel()
	app := New()
	app.Hooks().OnPreShutdown(func() error { return nil })
	app.Hooks().OnPostShutdown(func(error) error { return nil })
	appListen(t, app, ListenConfig{DisableStartupMessage: true})

	var wg sync.WaitGroup
	wg.Go(func() {
		for range 50 {
			app.Hooks().OnPreShutdown(func() error { return nil })
			app.Hooks().OnPostShutdown(func(error) error { return nil })
		}
	})
	wg.Go(func() { assert.NoError(t, app.Shutdown()) })
	wg.Wait()
}

// rawResponse is a response read off a test connection.
type rawResponse struct {
	header        http.Header
	body          string
	contentLength int64 // -1 when not declared
	status        int
	closes        bool // announced Connection: close
	chunked       bool
}

// What a connection did once the expected responses were read.
const (
	connClosed = "closed" // the server closed it
	connOpen   = "open"   // it stayed open and silent
	connData   = "data"   // it sent more than expected
)

// startRawServer serves app on an in-memory listener until the test ends.
func startRawServer(t *testing.T, app *App) *fasthttputil.InmemoryListener {
	t.Helper()

	return startRawServerWith(t, app, nil)
}

// startRawServerWith is startRawServer with a ListenConfig.BeforeServeFunc.
func startRawServerWith(t *testing.T, app *App, beforeServe func(*App) error) *fasthttputil.InmemoryListener {
	t.Helper()

	ln := fasthttputil.NewInmemoryListener()
	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Listener(ln, ListenConfig{DisableStartupMessage: true, BeforeServeFunc: beforeServe})
	}()

	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
		if err := <-errCh; err != nil && !errors.Is(err, net.ErrClosed) {
			require.NoError(t, err)
		}
	})

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err != nil {
			return false
		}
		return conn.Close() == nil
	}, time.Second, 5*time.Millisecond)

	return ln
}

// dialer opens connections to the server under test.
type dialer interface {
	Dial() (net.Conn, error)
}

// tcpDialer dials a TCP address.
type tcpDialer string

func (d tcpDialer) Dial() (net.Conn, error) {
	conn, err := net.Dial(NetworkTCP4, string(d))
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", string(d), err)
	}
	return conn, nil
}

// loopbackDialer dials an in-memory listener from a loopback TCP address, which the proxy trust checks accept.
type loopbackDialer struct {
	ln *fasthttputil.InmemoryListener
}

func (d loopbackDialer) Dial() (net.Conn, error) {
	conn, err := d.ln.DialWithLocalAddr(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 40000})
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	return conn, nil
}

// rawExchange writes raw to a new connection, reads want responses and reports
// what the connection did next.
func rawExchange(t *testing.T, server dialer, raw string, want int) (responses []rawResponse, after string) { //nolint:nonamedreturns // gocritic's unnamedResult asks for names
	t.Helper()

	conn, err := server.Dial()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() }) //nolint:errcheck // usually closed by the server already

	require.NoError(t, conn.SetDeadline(time.Now().Add(10*time.Second)))
	_, err = conn.Write([]byte(raw))
	require.NoError(t, err)

	// A response to HEAD has no body, so the parser needs the method.
	method, _, _ := strings.Cut(raw, " ")
	request := &http.Request{Method: method}

	reader := bufio.NewReader(conn)
	for range want {
		resp, readErr := http.ReadResponse(reader, request)
		require.NoError(t, readErr, "reading response %d of %d", len(responses)+1, want)
		body := apptest.Body(t, resp)
		responses = append(responses, rawResponse{
			status:        resp.StatusCode,
			header:        resp.Header,
			body:          body,
			closes:        resp.Close,
			contentLength: resp.ContentLength,
			chunked:       len(resp.TransferEncoding) > 0 && resp.TransferEncoding[0] == "chunked",
		})
	}

	// A closed connection answers at once; an open one stays silent until the deadline.
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(250*time.Millisecond)))
	if _, peekErr := reader.Peek(1); peekErr != nil {
		switch {
		case errors.Is(peekErr, io.EOF):
			after = connClosed
		case errors.Is(peekErr, fasthttputil.ErrTimeout), errors.Is(peekErr, os.ErrDeadlineExceeded):
			after = connOpen
		default:
			require.NoError(t, peekErr)
		}
	} else {
		after = connData
	}

	return responses, after
}

// rawExpectStatus is rawExchange for one response of the given status, which it returns for further checks. A
// failure names raw, the request that was sent.
func rawExpectStatus(t *testing.T, server dialer, raw string, status int) rawResponse {
	t.Helper()

	responses, _ := rawExchange(t, server, raw, 1)
	require.Equal(t, status, responses[0].status, raw)
	return responses[0]
}

// rawFieldLines returns n header or trailer lines named prefix0, prefix1, ... that all hold value.
func rawFieldLines(prefix, value string, n int) string {
	var lines strings.Builder
	for i := range n {
		fmt.Fprintf(&lines, "%s%d: %s\r\n", prefix, i, value)
	}
	return lines.String()
}

// An Accept-Encoding with no value reaches AcceptsEncodings as present but empty,
// which wants no content coding, and is not read as an absent one, which accepts any.
func Test_App_AcceptsEncodings_EmptyField_Request(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error { return c.SendString(c.AcceptsEncodings("br", "gzip", "identity")) })
	ln := startRawServer(t, app)

	for name, tc := range map[string]struct{ field, answer string }{
		"absent": {field: "", answer: "br"},
		"empty":  {field: "Accept-Encoding:\r\n", answer: "identity"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw := "GET / HTTP/1.1\r\nHost: example.com\r\n" + tc.field + "Connection: close\r\n\r\n"
			require.Equal(t, tc.answer, rawExpectStatus(t, ln, raw, StatusOK).body)
		})
	}
}

// RFC 9112 §6.1: a transfer coding the server does not understand gets a 501.
func Test_App_TransferEncoding_Request(t *testing.T) {
	t.Parallel()
	app := New()
	app.Post("/", func(c Ctx) error { return c.SendString("posted") })
	ln := startRawServer(t, app)

	tests := []struct {
		name   string
		raw    string
		status int
	}{
		{
			name:   "unknown coding",
			raw:    "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: foo\r\n\r\n5\r\nhello\r\n0\r\n\r\n",
			status: StatusNotImplemented,
		},
		{
			name:   "unknown coding before chunked",
			raw:    "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: gzip, chunked\r\n\r\n5\r\nhello\r\n0\r\n\r\n",
			status: StatusNotImplemented,
		},
		{
			name:   "unknown coding after chunked",
			raw:    "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked, gzip\r\n\r\n0\r\n\r\n",
			status: StatusNotImplemented,
		},
		{
			// HTTP/1.0 has no transfer codings.
			name:   "HTTP/1.0 message with the field",
			raw:    "POST / HTTP/1.0\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n",
			status: StatusBadRequest,
		},
		{
			name:   "repeated field",
			raw:    "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n",
			status: StatusBadRequest,
		},
		{
			name:   "chunked is served",
			raw:    "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n5\r\nhello\r\n0\r\n\r\n",
			status: StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			responses, after := rawExchange(t, ln, tc.raw, 1)
			require.Equal(t, tc.status, responses[0].status)
			if tc.status == StatusNotImplemented {
				// The body names the status, not the request bytes fasthttp quotes.
				require.Equal(t, "Not Implemented", responses[0].body)
			}
			if tc.status != StatusOK {
				// An unusable message leaves the connection in an unknown state.
				require.True(t, responses[0].closes, "Connection: close expected")
				require.Equal(t, connClosed, after)
			}
		})
	}
}

// The status a request the parser refuses is answered with, from the raw request of each row.
func Test_App_serverErrorHandler_RawRequestStatus(t *testing.T) {
	t.Parallel()
	const (
		chunked = "GET / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n"
		post    = "POST / HTTP/1.1\r\nHost: example.com\r\n"
	)
	timeout := Config{ReadTimeout: 200 * time.Millisecond}
	tests := []struct {
		config Config
		name   string
		raw    string
		body   string // the body of the answer, when it is checked
		status int
		secure bool
	}{
		{
			name: "unsupported method", raw: "FOO /bar HTTP/1.1\r\nHost: example.com\r\n\r\n",
			status: StatusNotImplemented, body: utils.StatusMessage(StatusNotImplemented),
		},

		// The status follows what the parser reported, never request bytes its error quotes.
		{name: "rejected method token", raw: "GE(T / HTTP/1.1\r\nHost: example.com\r\n\r\n", status: StatusNotImplemented},
		{
			name: "rejected method token, secure message", raw: "GE(T / HTTP/1.1\r\nHost: example.com\r\n\r\n",
			status: StatusNotImplemented, secure: true,
		},
		{
			name: "trailer quoting the method wording", raw: chunked + "Content-Length: unsupported http request method\r\n\r\n",
			status: StatusBadRequest,
		},
		{name: "trailer quoting a timeout", raw: chunked + "Content-Length: timeout\r\n\r\n", status: StatusBadRequest},

		// With SecureErrorLogMessage fasthttp reports every transfer-encoding fault with the same error, which hides
		// the coding: those requests stay 400.
		{
			name: "unknown transfer coding, secure message", raw: post + "Transfer-Encoding: foo\r\n\r\n0\r\n\r\n",
			status: StatusBadRequest, secure: true,
		},
		{
			name: "chunked, secure message", raw: post + "Transfer-Encoding: chunked\r\nConnection: close\r\n\r\n0\r\n\r\n",
			status: StatusOK, secure: true,
		},

		// A read timeout inside a chunked body surfaces as ErrBrokenChunk, which hides the net error.
		{
			name: "timeout in chunk data", config: timeout, raw: post + "Transfer-Encoding: chunked\r\n\r\n5\r",
			status: StatusRequestTimeout,
		},
		{
			name: "timeout before the trailer ends", config: timeout, raw: post + "Transfer-Encoding: chunked\r\n\r\n0\r\n",
			status: StatusRequestTimeout,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New(tc.config)
			app.Server().SecureErrorLogMessage = tc.secure
			app.Get("/", func(c Ctx) error { return c.SendString("page") })
			app.Get("/bar", func(c Ctx) error { return c.SendString("bar") })
			app.Post("/", func(c Ctx) error { return c.SendString("posted") })
			ln := startRawServer(t, app)

			resp := rawExpectStatus(t, ln, tc.raw, tc.status)
			if tc.body != "" {
				require.Equal(t, tc.body, resp.body)
			}
		})
	}
}

// RFC 9112 §6.1: a request with both Content-Length and Transfer-Encoding is
// framed by Transfer-Encoding, and the connection must close after the response.
func Test_App_ContentLengthWithTransferEncoding_ClosesConnection(t *testing.T) {
	t.Parallel()
	app := New()
	app.Post("/", func(c Ctx) error { return c.SendString("first") })
	app.Get("/second", func(c Ctx) error { return c.SendString("second") })
	ln := startRawServer(t, app)

	const (
		second  = "GET /second HTTP/1.1\r\nHost: example.com\r\n\r\n"
		chunked = "\r\n0\r\n\r\n"
	)

	for group, requests := range map[string]map[string]string{ // headers and body of the first request
		"served by Transfer-Encoding then closed": {
			"Content-Length first":    "Content-Length: 4\r\nTransfer-Encoding: chunked\r\n" + chunked,
			"Transfer-Encoding first": "Transfer-Encoding: chunked\r\nContent-Length: 4\r\n" + chunked,
			"lower-case names":        "content-length: 4\r\ntransfer-encoding: chunked\r\n" + chunked,
		},
		// fasthttp ignores "identity" and frames the request by its Content-Length, so
		// the request is served as one without the field, but it still has both.
		"identity is ignored and still closes": {
			"Content-Length first":    "Content-Length: 4\r\nTransfer-Encoding: identity\r\n\r\nabcd",
			"Transfer-Encoding first": "Transfer-Encoding: identity\r\nContent-Length: 4\r\n\r\nabcd",
			"lower-case names":        "content-length: 4\r\ntransfer-encoding: identity\r\n\r\nabcd",
			"empty body":              "Content-Length: 0\r\nTransfer-Encoding: identity\r\n\r\n",
		},
	} {
		t.Run(group, func(t *testing.T) {
			t.Parallel()
			for name, headers := range requests {
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					responses, after := rawExchange(t, ln, "POST / HTTP/1.1\r\nHost: example.com\r\n"+headers+second, 1)
					require.Equal(t, StatusOK, responses[0].status)
					require.Equal(t, "first", responses[0].body)
					require.True(t, responses[0].closes, "Connection: close expected")
					require.Equal(t, connClosed, after, "the pipelined request must not be answered")
				})
			}
		})
	}

	// Each header alone is ordinary traffic and keeps the connection alive.
	t.Run("one header alone keeps the connection", func(t *testing.T) {
		t.Parallel()
		for name, raw := range map[string]string{
			"Transfer-Encoding only": "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\n" + chunked + second,
			"Content-Length only":    "POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 0\r\n\r\n" + second,
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				responses, after := rawExchange(t, ln, raw, 2)

				require.Equal(t, "first", responses[0].body)
				require.Equal(t, "second", responses[1].body)
				require.False(t, responses[1].closes, "Connection: close not expected")
				require.Equal(t, connOpen, after)
			})
		}
	})
}

// The close hook must keep the application's HeaderReceived callback and its limits,
// whether it is set before the app starts or in ListenConfig.BeforeServeFunc, and
// whether or not an earlier Test has already installed the hook.
func Test_App_ContentLengthWithTransferEncoding_KeepsUserHeaderReceived(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		inBeforeServe bool
		testFirst     bool
	}{
		{name: "set before start"},
		{name: "set in BeforeServeFunc", inBeforeServe: true},
		{name: "set in BeforeServeFunc after Test", inBeforeServe: true, testFirst: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Post("/", func(c Ctx) error { return c.SendString(strconv.Itoa(len(c.Body()))) })

			var calls atomic.Int32
			var chunkedSeen atomic.Bool
			callback := func(header *fasthttp.RequestHeader) fasthttp.RequestConfig {
				calls.Add(1)
				if header.ContentLength() == -1 {
					chunkedSeen.Store(true)
				}
				return fasthttp.RequestConfig{MaxRequestBodySize: 8}
			}

			if tc.testFirst {
				resp := apptest.Do(t, app, apptest.Req(MethodPost, "/"))
				require.NoError(t, resp.Body.Close())
			}

			var beforeServe func(*App) error
			if tc.inBeforeServe {
				beforeServe = func(app *App) error {
					app.Server().HeaderReceived = callback
					return nil
				}
			} else {
				app.Server().HeaderReceived = callback
			}
			ln := startRawServerWith(t, app, beforeServe)

			// Within the limit: served, then closed.
			responses, after := rawExchange(t, ln,
				"POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 4\r\nTransfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n\r\n", 1)
			require.Equal(t, StatusOK, responses[0].status)
			require.Equal(t, "5", responses[0].body)
			require.True(t, responses[0].closes)
			require.Equal(t, connClosed, after)

			// Over the callback's limit: refused.
			responses, _ = rawExchange(t, ln,
				"POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 20\r\nConnection: close\r\n\r\n01234567890123456789", 1)
			require.Equal(t, StatusRequestEntityTooLarge, responses[0].status)

			require.Equal(t, int32(2), calls.Load(), "the application's callback runs for every request")
			require.True(t, chunkedSeen.Load(), "the callback still sees the chunked framing")
		})
	}
}

// The hook is installed once and a replaced callback is wrapped again.
func Test_App_HeaderReceivedHook_Reinstalled(t *testing.T) {
	t.Parallel()
	app := New()
	app.hookHeaderReceived()
	installed := app.Server().HeaderReceived
	require.NotNil(t, installed)

	app.hookHeaderReceived()
	require.True(t, sameFunc(app.Server().HeaderReceived, installed), "the hook is not wrapped again")

	replaced := func(*fasthttp.RequestHeader) fasthttp.RequestConfig { return fasthttp.RequestConfig{} }
	app.Server().HeaderReceived = replaced
	app.hookHeaderReceived()
	require.False(t, sameFunc(app.Server().HeaderReceived, replaced), "a replaced callback is wrapped")
	require.False(t, sameFunc(app.Server().HeaderReceived, installed))
}

// Test and a listener run on one server at the same time, and the connections they
// serve read the callback, so installing the hook must not write it again. Meant
// for -race.
func Test_App_HeaderReceivedHook_RaceFreeWithTest(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error { return c.SendString("ok") })

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				app.hookHeaderReceived()
				runtime.Gosched()
			}
		}
	})

	for range 50 {
		resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
		if assert.NoError(t, err) {
			assert.NoError(t, resp.Body.Close())
		}
	}
	close(stop)
	wg.Wait()
}

// Test serves the connection itself, so it installs the close hook too.
func Test_App_Test_ClosesAfterContentLengthWithTransferEncoding(t *testing.T) {
	t.Parallel()
	app := New()
	app.Post("/", func(c Ctx) error { return c.SendString(strconv.Itoa(len(c.Body()))) })

	// The dump of a chunked request keeps the Content-Length header it is given.
	req := httptest.NewRequest(MethodPost, "/", strings.NewReader("hello"))
	req.TransferEncoding = []string{"chunked"}
	req.Header.Set(HeaderContentLength, "4")

	resp := apptest.Do(t, app, req)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, StatusOK, resp.StatusCode)
	require.True(t, resp.Close, "Connection: close expected")
}

// A client older than HTTP/1.1 that leaves must free the writer. The response is
// written as it is produced, so the writer's Flush fails once the client is gone;
// buffering it instead would hold the handler, and its connection slot, until the
// writer ended on its own.
func Test_App_SendStreamWriter_HTTP10_ClientLeaves(t *testing.T) {
	t.Parallel()
	stop := make(chan struct{})
	ended := make(chan struct{})
	app := New()
	app.Get("/", func(c Ctx) error {
		return c.SendStreamWriter(func(w *bufio.Writer) {
			defer close(ended)
			for {
				select {
				case <-stop:
					return
				default:
				}
				if _, err := w.WriteString(":\n\n"); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
				time.Sleep(time.Millisecond)
			}
		})
	})
	ln := startRawServer(t, app)
	t.Cleanup(func() { close(stop) })

	conn, err := ln.Dial()
	require.NoError(t, err)
	_, err = conn.Write([]byte("GET / HTTP/1.0\r\n\r\n"))
	require.NoError(t, err)

	// The first bytes arrive while the writer is still running.
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err = bufio.NewReader(conn).ReadString('\n')
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	apptest.Recv(t, ended, 5*time.Second, "the writer kept running after the client left")
}
