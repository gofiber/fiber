// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/gofiber/utils/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

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

	req := httptest.NewRequest(method, url, http.NoBody)

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
}

func testErrorResponse(t *testing.T, err error, resp *http.Response, expectedBodyError string) {
	t.Helper()

	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 500, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, expectedBodyError, string(body), "Response body")
}

func Test_App_Test_Goroutine_Leak_Compare(t *testing.T) {
	t.Parallel()

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
			timeout:    50 * time.Millisecond,  // // Short timeout to ensure triggering
			sleepTime:  500 * time.Millisecond, // Wait time longer than handler execution time
			expectLeak: true,
		},
		{
			name: "Without timeout (no leak)",
			handler: func(c Ctx) error {
				return c.SendString("ok") // Return immediately
			},
			timeout:    0, // Disable timeout
			sleepTime:  100 * time.Millisecond,
			expectLeak: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()

			// Record initial goroutine count
			initialGoroutines := runtime.NumGoroutine()
			t.Logf("[%s] Initial goroutines: %d", tc.name, initialGoroutines)

			app.Get("/", tc.handler)

			// Send 10 requests
			numRequests := 10
			for range numRequests {
				req := httptest.NewRequest(MethodGet, "/", http.NoBody)

				if tc.timeout > 0 {
					_, err := app.Test(req, TestConfig{
						Timeout:       tc.timeout,
						FailOnTimeout: true,
					})
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

			// Check final goroutine count
			finalGoroutines := runtime.NumGoroutine()
			leakedGoroutines := finalGoroutines - initialGoroutines
			if leakedGoroutines < 0 {
				leakedGoroutines = 0
			}
			t.Logf("[%s] Final goroutines: %d (leaked: %d)",
				tc.name, finalGoroutines, leakedGoroutines)

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

	app.Use(func(c Ctx) error {
		return c.Next()
	})

	app.Post("/", testEmptyHandler)

	app.Options("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodPost, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Empty(t, resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 405, resp.StatusCode)
	require.Equal(t, "POST, OPTIONS", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest(MethodPatch, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 405, resp.StatusCode)
	require.Equal(t, "POST, OPTIONS", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest(MethodPut, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 405, resp.StatusCode)
	require.Equal(t, "POST, OPTIONS", resp.Header.Get(HeaderAllow))

	app.Get("/", testEmptyHandler)

	resp, err = app.Test(httptest.NewRequest(MethodTrace, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 405, resp.StatusCode)
	require.Equal(t, "GET, HEAD, POST, OPTIONS", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest(MethodPatch, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 405, resp.StatusCode)
	require.Equal(t, "GET, HEAD, POST, OPTIONS", resp.Header.Get(HeaderAllow))

	app.Head("/", testEmptyHandler)

	resp, err = app.Test(httptest.NewRequest(MethodPut, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 405, resp.StatusCode)
	require.Equal(t, "GET, HEAD, POST, OPTIONS", resp.Header.Get(HeaderAllow))
}

func Test_App_RegisterNetHTTPHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		register   func(app *App, path string, handler any)
		method     string
		expectBody bool
	}{
		{
			name: "Get",
			register: func(app *App, path string, handler any) {
				app.Get(path, handler)
			},
			method:     http.MethodGet,
			expectBody: true,
		},
		{
			name: "Head",
			register: func(app *App, path string, handler any) {
				app.Head(path, handler)
			},
			method: http.MethodHead,
		},
		{
			name: "Post",
			register: func(app *App, path string, handler any) {
				app.Post(path, handler)
			},
			method:     http.MethodPost,
			expectBody: true,
		},
		{
			name: "Put",
			register: func(app *App, path string, handler any) {
				app.Put(path, handler)
			},
			method:     http.MethodPut,
			expectBody: true,
		},
		{
			name: "Delete",
			register: func(app *App, path string, handler any) {
				app.Delete(path, handler)
			},
			method:     http.MethodDelete,
			expectBody: true,
		},
		{
			name: "Connect",
			register: func(app *App, path string, handler any) {
				app.Connect(path, handler)
			},
			method:     http.MethodConnect,
			expectBody: true,
		},
		{
			name: "Options",
			register: func(app *App, path string, handler any) {
				app.Options(path, handler)
			},
			method:     http.MethodOptions,
			expectBody: true,
		},
		{
			name: "Trace",
			register: func(app *App, path string, handler any) {
				app.Trace(path, handler)
			},
			method:     http.MethodTrace,
			expectBody: true,
		},
		{
			name: "Patch",
			register: func(app *App, path string, handler any) {
				app.Patch(path, handler)
			},
			method:     http.MethodPatch,
			expectBody: true,
		},
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

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusAccepted, resp.StatusCode)
			require.Equal(t, tt.method, resp.Header.Get("X-Test"))

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

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

	app.Use(func(c Ctx) error {
		return c.SendStatus(404)
	})

	app.Post("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
	require.Equal(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Not Found", string(body))
	require.Equal(t, strconv.Itoa(len("Not Found")), resp.Header.Get(HeaderContentLength))

	g := app.Group("/with-next", func(c Ctx) error {
		return c.Status(404).Next()
	})

	g.Post("/", testEmptyHandler)

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/with-next", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
	require.Equal(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Not Found", string(body))
	require.Equal(t, strconv.Itoa(len("Not Found")), resp.Header.Get(HeaderContentLength))
}

func Test_App_ServerErrorHandler_SmallReadBuffer(t *testing.T) {
	t.Parallel()
	expectedError := regexp.MustCompile(
		`error when reading request headers: small read buffer\. Increase ReadBufferSize\. Buffer size=4096, contents: "GET / HTTP/1.1\\r\\nHost: example\.com\\r\\nVery-Long-Header: -+`,
	)
	app := New()

	app.Get("/", func(_ Ctx) error {
		panic(errors.New("should never called"))
	})

	request := httptest.NewRequest(MethodGet, "/", http.NoBody)
	logHeaderSlice := make([]string, 5000)
	request.Header.Set("Very-Long-Header", strings.Join(logHeaderSlice, "-"))
	_, err := app.Test(request)
	if err == nil {
		t.Error("Expect an error at app.Test(request)")
	}

	require.Regexp(t, expectedError, err.Error())
}

func Test_App_Errors(t *testing.T) {
	t.Parallel()
	app := New(Config{
		BodyLimit: 4,
	})

	app.Get("/", func(_ Ctx) error {
		return errors.New("hi, i'm an error")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 500, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "hi, i'm an error", string(body))

	_, err = app.Test(httptest.NewRequest(MethodGet, "/", strings.NewReader("big body")))
	if err != nil {
		require.Equal(t, "body size exceeds the given limit", err.Error(), "app.Test(req)")
	}
}

func Test_App_BodyLimit_Negative(t *testing.T) {
	t.Parallel()

	limits := []int{-1, -512}
	for _, limit := range limits {
		app := New(Config{BodyLimit: limit})

		app.Post("/", func(c Ctx) error {
			return c.SendStatus(StatusOK)
		})

		largeBody := bytes.Repeat([]byte{'a'}, DefaultBodyLimit+1)
		req := httptest.NewRequest(MethodPost, "/", bytes.NewReader(largeBody))
		_, err := app.Test(req)
		require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)

		smallBody := bytes.Repeat([]byte{'a'}, DefaultBodyLimit-1)
		req = httptest.NewRequest(MethodPost, "/", bytes.NewReader(smallBody))
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
	}
}

func Test_App_BodyLimit_Zero(t *testing.T) {
	t.Parallel()

	app := New(Config{BodyLimit: 0})

	app.Post("/", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	largeBody := bytes.Repeat([]byte{'a'}, DefaultBodyLimit+1)
	req := httptest.NewRequest(MethodPost, "/", bytes.NewReader(largeBody))
	_, err := app.Test(req)
	require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)

	smallBody := bytes.Repeat([]byte{'a'}, DefaultBodyLimit-1)
	req = httptest.NewRequest(MethodPost, "/", bytes.NewReader(smallBody))
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
}

func Test_App_BodyLimit_LargerThanDefault(t *testing.T) {
	t.Parallel()

	limit := DefaultBodyLimit*2 + 1024 // slightly above double the default
	app := New(Config{BodyLimit: limit})

	app.Post("/", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	// Body larger than the default but within our custom limit should succeed
	midBody := bytes.Repeat([]byte{'a'}, DefaultBodyLimit+512)
	req := httptest.NewRequest(MethodPost, "/", bytes.NewReader(midBody))
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	// Body above the custom limit should fail
	largeBody := bytes.Repeat([]byte{'a'}, limit+1)
	req = httptest.NewRequest(MethodPost, "/", bytes.NewReader(largeBody))
	_, err = app.Test(req)
	require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
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

	app.Get("/test/:param<test(test)>", func(c Ctx) error {
		return c.SendString("test")
	})

	app.Get("/test2/:param<test>", func(c Ctx) error {
		return c.SendString("test")
	})

	app.Get("/test3/:param<test()>", func(c Ctx) error {
		return c.SendString("test")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/test2", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test2/c", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test2/cc", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test3/cc", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")
}

func Test_App_ErrorHandler_Custom(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(200).SendString("hi, i'm a custom error")
		},
	})

	app.Get("/", func(_ Ctx) error {
		return errors.New("hi, i'm an error")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "hi, i'm a custom error", string(body))
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
	app.Get("/", func(_ Ctx) error {
		return errors.New("0: GET error")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 500, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1: USE error", string(body))
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 500, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1: USE error", string(body))
}

func Test_App_serverErrorHandler_Internal_Error(t *testing.T) {
	t.Parallel()
	app := New()
	msg := "test err"
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	app.serverErrorHandler(c.fasthttp, errors.New(msg))
	require.Equal(t, string(c.fasthttp.Response.Body()), msg)
	require.Equal(t, StatusBadRequest, c.fasthttp.Response.StatusCode())
}

func Test_App_serverErrorHandler_Network_Error(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	app.serverErrorHandler(c.fasthttp, &net.DNSError{
		Err:       "test error",
		Name:      "test host",
		IsTimeout: false,
	})
	require.Equal(t, string(c.fasthttp.Response.Body()), utils.StatusMessage(StatusBadGateway))
	require.Equal(t, StatusBadGateway, c.fasthttp.Response.StatusCode())
}

func Test_App_serverErrorHandler_Unsupported_Method_Error(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	app.serverErrorHandler(c.fasthttp, errors.New("unsupported http request method 'FOO'"))
	require.Equal(t, utils.StatusMessage(StatusNotImplemented), string(c.fasthttp.Response.Body()))
	require.Equal(t, StatusNotImplemented, c.fasthttp.Response.StatusCode())
}

func Test_App_serverErrorHandler_Unsupported_Method_Request(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/bar", func(c Ctx) error {
		return c.SendString("bar")
	})

	ln := fasthttputil.NewInmemoryListener()

	serverStarted := make(chan struct{}, 1)
	serverErr := make(chan error, 1)

	go func() {
		serverStarted <- struct{}{}
		if err := app.Listener(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	<-serverStarted

	conn, err := ln.Dial()
	require.NoError(t, err)
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))

	_, err = conn.Write([]byte("FOO /bar HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	require.Equal(t, StatusNotImplemented, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, utils.StatusMessage(StatusNotImplemented), string(body))
	require.NoError(t, resp.Body.Close())
	require.NoError(t, conn.Close())

	require.NoError(t, app.Shutdown())
	require.NoError(t, <-serverErr)
}

func Test_App_Nested_Params(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/test", func(c Ctx) error {
		return c.Status(400).Send([]byte("Should move on"))
	})
	app.Get("/test/:param", func(c Ctx) error {
		return c.Status(400).Send([]byte("Should move on"))
	})
	app.Get("/test/:param/test", func(c Ctx) error {
		return c.Status(400).Send([]byte("Should move on"))
	})
	app.Get("/test/:param/test/:param2", func(c Ctx) error {
		return c.Status(200).Send([]byte("Good job"))
	})

	req := httptest.NewRequest(MethodGet, "/test/john/test/doe", http.NoBody)
	resp, err := app.Test(req)

	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/prefix/john", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/john/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	require.PanicsWithValue(t, "use: invalid handler func()\n", func() {
		app.Use("/:param/*", func() {
			// this should panic
		})
	})
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/cR%C3%A9eR/%D8%A7%D8%AE%D8%AA%D8%A8%D8%A7%D8%B1", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	// check the param result
	require.Equal(t, "اختبار", app.toString(body))

	// with lowercase letters
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/cr%C3%A9er/%D8%A7%D8%AE%D8%AA%D8%A8%D8%A7%D8%B1", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")
}

func Test_App_Use_CaseSensitive(t *testing.T) {
	t.Parallel()
	app := New(Config{CaseSensitive: true})

	app.Use("/abc", func(c Ctx) error {
		return c.SendString(c.Path())
	})

	// wrong letters in the requested route -> 404
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/AbC", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")

	// right letters in the requested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/abc", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// check the detected path when the case-insensitive recognition is activated
	app.config.CaseSensitive = false
	// check the case-sensitive feature
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/AbC", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	// check the detected path result
	require.Equal(t, "/AbC", app.toString(body))
}

func Test_App_Not_Use_StrictRouting(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use("/abc", func(c Ctx) error {
		return c.SendString(c.Path())
	})

	g := app.Group("/foo")
	g.Use("/", func(c Ctx) error {
		return c.SendString(c.Path())
	})

	// wrong path in the requested route -> 404
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/abc/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// right path in the requested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/abc", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// wrong path with group in the requested route -> 404
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// right path with group in the requested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

func Test_App_Use_MultiplePrefix(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use([]string{"/john", "/doe"}, func(c Ctx) error {
		return c.SendString(c.Path())
	})

	g := app.Group("/test")
	g.Use([]string{"/john", "/doe"}, func(c Ctx) error {
		return c.SendString(c.Path())
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "/john", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "/doe", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/john", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "/test/john", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "/test/doe", string(body))
}

func Test_Group_Use_NoBoundary(t *testing.T) {
	t.Parallel()

	app := New()
	grp := app.Group("/api")

	grp.Use("/foo", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/foo/bar", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/api/foobar", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")
}

func Test_App_Use_StrictRouting(t *testing.T) {
	t.Parallel()
	app := New(Config{StrictRouting: true})

	app.Get("/abc", func(c Ctx) error {
		return c.SendString(c.Path())
	})

	g := app.Group("/foo")
	g.Get("/", func(c Ctx) error {
		return c.SendString(c.Path())
	})

	// wrong path in the requested route -> 404
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/abc/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")

	// right path in the requested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/abc", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// wrong path with group in the requested route -> 404
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")

	// right path with group in the requested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

func Test_App_Add_Method_Test(t *testing.T) {
	t.Parallel()

	methods := append(DefaultMethods, "JOHN") //nolint:gocritic // We want a new slice here
	app := New(Config{
		RequestMethods: methods,
	})

	app.Add([]string{"JOHN"}, "/john", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest("JOHN", "/john", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/john", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusMethodNotAllowed, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("UNKNOWN", "/john", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotImplemented, resp.StatusCode, "Status code")

	// Add a new method
	require.Panics(t, func() {
		app.Add([]string{"JANE"}, "/jane", testEmptyHandler)
	})
}

func Test_App_All_Method_Test(t *testing.T) {
	t.Parallel()

	methods := append(DefaultMethods, "JOHN") //nolint:gocritic // We want a new slice here
	app := New(Config{
		RequestMethods: methods,
	})

	// Add a new method with All
	app.All("/doe", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest("JOHN", "/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// Add a new method
	require.Panics(t, func() {
		app.Add([]string{"JANE"}, "/jane", testEmptyHandler)
	})
}

// go test -run Test_App_GETOnly
func Test_App_GETOnly(t *testing.T) {
	t.Parallel()
	app := New(Config{
		GETOnly: true,
	})

	app.Post("/", func(c Ctx) error {
		return c.SendString("Hello 👋!")
	})

	req := httptest.NewRequest(MethodPost, "/", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusMethodNotAllowed, resp.StatusCode, "Status code")
}

func Test_App_Use_Params_Group(t *testing.T) {
	t.Parallel()
	app := New()

	group := app.Group("/prefix/:param/*")
	group.Use("/", func(c Ctx) error {
		return c.Next()
	})
	group.Get("/test", func(c Ctx) error {
		require.Equal(t, "john", c.Params("param"))
		require.Equal(t, "doe", c.Params("*"))
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/prefix/john/doe/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
}

func Test_App_Chaining(t *testing.T) {
	t.Parallel()
	n := func(c Ctx) error {
		return c.Next()
	}
	app := New()
	app.Use("/john", n, n, n, n, func(c Ctx) error {
		return c.SendStatus(202)
	})
	// check handler count for registered HEAD route
	require.Len(t, app.stack[app.methodInt(MethodHead)][0].Handlers, 5, "app.Test(req)")

	req := httptest.NewRequest(MethodPost, "/john", http.NoBody)

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 202, resp.StatusCode, "Status code")

	app.Get("/test", n, n, n, n, func(c Ctx) error {
		return c.SendStatus(203)
	})

	req = httptest.NewRequest(MethodGet, "/test", http.NoBody)

	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 203, resp.StatusCode, "Status code")
}

func Test_App_Order(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/test", func(c Ctx) error {
		_, err := c.WriteString("1")
		require.NoError(t, err)
		return c.Next()
	})

	app.All("/test", func(c Ctx) error {
		_, err := c.WriteString("2")
		require.NoError(t, err)

		return c.Next()
	})

	app.Use(func(c Ctx) error {
		_, err := c.WriteString("3")
		require.NoError(t, err)

		return c.SendStatus(StatusOK)
	})

	req := httptest.NewRequest(MethodGet, "/test", http.NoBody)

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(body))
}

func Test_App_AutoHead_Compliance(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/hello", func(c Ctx) error {
		c.Set("X-Test", "string")
		return c.SendString("hello")
	})
	app.startupProcess()

	getReq := httptest.NewRequest(MethodGet, "/hello", http.NoBody)
	getResp, err := app.Test(getReq)
	require.NoError(t, err, "app.Test(get)")
	defer func() {
		require.NoError(t, getResp.Body.Close())
	}()

	body, err := io.ReadAll(getResp.Body)
	require.NoError(t, err)
	require.Equal(t, "hello", string(body))
	require.Equal(t, "string", getResp.Header.Get("X-Test"))

	headReq := httptest.NewRequest(MethodHead, "/hello", http.NoBody)
	headResp, err := app.Test(headReq)
	require.NoError(t, err, "app.Test(head)")
	defer func() {
		require.NoError(t, headResp.Body.Close())
	}()

	require.Equal(t, getResp.StatusCode, headResp.StatusCode)
	require.Equal(t, strconv.Itoa(len(body)), headResp.Header.Get(HeaderContentLength))
	require.Equal(t, getResp.Header.Get(HeaderContentType), headResp.Header.Get(HeaderContentType))
	require.Equal(t, getResp.Header.Get("X-Test"), headResp.Header.Get("X-Test"))

	headBody, err := io.ReadAll(headResp.Body)
	require.NoError(t, err)
	require.Empty(t, headBody)
}

func Test_App_AutoHead_Compliance_SendFile(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("SendFile auto-HEAD test is skipped on Windows due to file locking semantics")
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "hello.txt")
	fileContent := []byte("file-body")
	require.NoError(t, os.WriteFile(filePath, fileContent, 0o644)) //nolint:gosec // permissions match test fixtures

	app := New()
	app.Get("/file", func(c Ctx) error {
		c.Set("X-Test", "file")
		return c.SendFile(filePath)
	})
	app.startupProcess()

	getReq := httptest.NewRequest(MethodGet, "/file", http.NoBody)
	getResp, err := app.Test(getReq)
	require.NoError(t, err, "app.Test(get)")
	defer func() {
		require.NoError(t, getResp.Body.Close())
	}()

	body, err := io.ReadAll(getResp.Body)
	require.NoError(t, err)
	require.Equal(t, fileContent, body)
	require.Equal(t, "file", getResp.Header.Get("X-Test"))

	headReq := httptest.NewRequest(MethodHead, "/file", http.NoBody)
	headResp, err := app.Test(headReq)
	require.NoError(t, err, "app.Test(head)")
	defer func() {
		require.NoError(t, headResp.Body.Close())
	}()

	require.Equal(t, getResp.StatusCode, headResp.StatusCode)
	require.Equal(t, strconv.Itoa(len(fileContent)), headResp.Header.Get(HeaderContentLength))
	require.Equal(t, getResp.Header.Get(HeaderContentType), headResp.Header.Get(HeaderContentType))
	require.Equal(t, getResp.Header.Get("X-Test"), headResp.Header.Get("X-Test"))

	headBody, err := io.ReadAll(headResp.Body)
	require.NoError(t, err)
	require.Empty(t, headBody)
}

func Test_App_Methods(t *testing.T) {
	t.Parallel()
	dummyHandler := testEmptyHandler

	app := New()

	app.Connect("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "CONNECT")

	app.Put("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodPut)

	app.Post("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodPost)

	app.Delete("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodDelete)

	app.Head("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodHead)

	app.Patch("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodPatch)

	app.Options("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodOptions)

	app.Trace("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodTrace)

	app.Get("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodGet)

	app.All("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodPost)

	app.Use("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodGet)
}

func Test_App_Route_Naming(t *testing.T) {
	t.Parallel()
	app := New()
	handler := func(c Ctx) error {
		return c.SendStatus(StatusOK)
	}
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

	appConfig := New(Config{
		Immutable: true,
	})
	appConfig.Get("/", testEmptyHandler)
}

func Test_App_Config(t *testing.T) {
	t.Parallel()
	app := New(Config{
		StrictRouting: true,
	})
	require.True(t, app.Config().StrictRouting)
}

func Test_App_GetString(t *testing.T) {
	t.Parallel()

	heap := string([]byte("fiber"))
	appMutable := New()
	same := appMutable.GetString(heap)
	if unsafe.StringData(same) != unsafe.StringData(heap) { //nolint:gosec // compare pointer addresses
		t.Error("expected original string when immutable is disabled")
	}

	appImmutable := New(Config{Immutable: true})
	copied := appImmutable.GetString(heap)
	if unsafe.StringData(copied) == unsafe.StringData(heap) { //nolint:gosec // compare pointer addresses
		t.Error("expected a copy for heap-backed string when immutable is enabled")
	}

	literal := "fiber"
	sameLit := appImmutable.GetString(literal)
	if unsafe.StringData(sameLit) != unsafe.StringData(literal) { //nolint:gosec // compare pointer addresses
		t.Error("expected original literal when immutable is enabled")
	}
}

func Test_App_GetBytes(t *testing.T) {
	t.Parallel()

	b := []byte("fiber")
	appMutable := New()
	same := appMutable.GetBytes(b)
	if unsafe.SliceData(same) != unsafe.SliceData(b) { //nolint:gosec // compare pointer addresses
		t.Error("expected original slice when immutable is disabled")
	}

	alias := make([]byte, 10)
	copy(alias, b)
	sub := alias[:5]
	appImmutable := New(Config{Immutable: true})
	copied := appImmutable.GetBytes(sub)
	if unsafe.SliceData(copied) == unsafe.SliceData(sub) { //nolint:gosec // compare pointer addresses
		t.Error("expected a copy for aliased slice when immutable is enabled")
	}

	full := make([]byte, 5)
	copy(full, b)
	detached := appImmutable.GetBytes(full)
	if unsafe.SliceData(detached) == unsafe.SliceData(full) { //nolint:gosec // compare pointer addresses
		t.Error("expected a copy even when cap==len")
	}
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

	ln := fasthttputil.NewInmemoryListener()
	serverReady := make(chan struct{}) // Signal that the server is ready to start

	go func() {
		serverReady <- struct{}{}
		err := app.Listener(ln)
		assert.NoError(t, err)
	}()

	<-serverReady // Waiting for the server to be ready

	// Create a connection and send a request
	connReady := make(chan struct{})
	go func() {
		conn, err := ln.Dial()
		assert.NoError(t, err)

		_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: google.com\r\n\r\n"))
		assert.NoError(t, err)

		connReady <- struct{}{} // Signal that the request has been sent
	}()

	<-connReady // Waiting for the request to be sent

	shutdownErr := make(chan error)
	go func() {
		shutdownErr <- app.ShutdownWithTimeout(1 * time.Second)
	}()

	timer := time.NewTimer(time.Second * 5)
	select {
	case <-timer.C:
		t.Fatal("idle connections not closed on shutdown")
	case err := <-shutdownErr:
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("unexpected err %v. Expecting %v", err, context.DeadlineExceeded)
		}
	}
}

func Test_App_ShutdownWithContext(t *testing.T) {
	t.Parallel()

	t.Run("successful shutdown", func(t *testing.T) {
		t.Parallel()
		app := New()

		// Fast request that should complete
		app.Get("/", func(c Ctx) error {
			return c.SendString("OK")
		})

		ln := fasthttputil.NewInmemoryListener()
		serverStarted := make(chan bool, 1)

		go func() {
			serverStarted <- true
			if err := app.Listener(ln); err != nil {
				t.Errorf("Failed to start listener: %v", err)
			}
		}()

		<-serverStarted

		// Execute normal request
		conn, err := ln.Dial()
		require.NoError(t, err)
		_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		require.NoError(t, err)

		// Shutdown with sufficient timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = app.ShutdownWithContext(ctx)
		require.NoError(t, err, "Expected successful shutdown")
	})

	t.Run("shutdown with hooks", func(t *testing.T) {
		t.Parallel()
		app := New()

		hookOrder := make([]string, 0)
		var hookMutex sync.Mutex

		app.Hooks().OnPreShutdown(func() error {
			hookMutex.Lock()
			hookOrder = append(hookOrder, "pre")
			hookMutex.Unlock()
			return nil
		})

		app.Hooks().OnPostShutdown(func(_ error) error {
			hookMutex.Lock()
			hookOrder = append(hookOrder, "post")
			hookMutex.Unlock()
			return nil
		})

		ln := fasthttputil.NewInmemoryListener()
		go func() {
			if err := app.Listener(ln); err != nil {
				t.Errorf("Failed to start listener: %v", err)
			}
		}()

		time.Sleep(100 * time.Millisecond)

		err := app.ShutdownWithContext(context.Background())
		require.NoError(t, err)

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

		ln := fasthttputil.NewInmemoryListener()
		go func() {
			if err := app.Listener(ln); err != nil {
				t.Errorf("Failed to start listener: %v", err)
			}
		}()

		// Ensure server is fully started
		time.Sleep(100 * time.Millisecond)

		// Start a long-running request
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

		// Wait for request to start
		select {
		case <-requestStarted:
			// Request has started, signal to continue processing
			close(requestProcessing)
		case <-time.After(2 * time.Second):
			t.Fatal("Request did not start in time")
		}

		// Attempt shutdown, should timeout
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err := app.ShutdownWithContext(ctx)
		require.ErrorIs(t, err, context.DeadlineExceeded)
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

	ln := fasthttputil.NewInmemoryListener()
	errCh := make(chan error, 1)
	serverReady := make(chan struct{})

	go func() {
		serverReady <- struct{}{}
		errCh <- app.Listener(ln)
	}()

	<-serverReady

	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
		require.NoError(t, <-errCh)
	})

	writeRequest := func(conn net.Conn, raw string) {
		t.Helper()
		_, err := conn.Write([]byte(raw))
		require.NoError(t, err)
	}

	conn, err := ln.Dial()
	require.NoError(t, err)

	writeRequest(conn, "OPTIONS * HTTP/1.1\r\nHost: example.com\r\n\r\n")

	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodOptions})
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	require.Equal(t, "GET, POST", resp.Header.Get(HeaderAllow))
	require.Zero(t, resp.ContentLength)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Empty(t, body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, conn.Close())

	controlConn, err := ln.Dial()
	require.NoError(t, err)

	writeRequest(controlConn, "OPTIONS /resource HTTP/1.1\r\nHost: example.com\r\n\r\n")

	controlResp, err := http.ReadResponse(bufio.NewReader(controlConn), &http.Request{Method: http.MethodOptions})
	require.NoError(t, err)
	require.Equal(t, StatusNoContent, controlResp.StatusCode)
	require.Equal(t, "GET", controlResp.Header.Get(HeaderAllow))
	require.Zero(t, controlResp.ContentLength)
	controlBody, err := io.ReadAll(controlResp.Body)
	require.NoError(t, err)
	require.Empty(t, controlBody)
	require.NoError(t, controlResp.Body.Close())
	require.NoError(t, controlConn.Close())
}

// go test -run Test_App_Mixed_Routes_WithSameLen
func Test_App_Mixed_Routes_WithSameLen(t *testing.T) {
	t.Parallel()
	app := New()

	// middleware
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

	// match get route
	req := httptest.NewRequest(MethodGet, "/foobar", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(HeaderContentLength))
	require.Equal(t, "TestValue", resp.Header.Get("TestHeader"))
	require.Equal(t, "text/html; charset=utf-8", resp.Header.Get(HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "FOO_BAR", string(body))

	// match static route
	req = httptest.NewRequest(MethodGet, "/tesbar", http.NoBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(HeaderContentLength))
	require.Equal(t, "TestValue", resp.Header.Get("TestHeader"))
	require.Equal(t, "text/html; charset=utf-8", resp.Header.Get(HeaderContentType))

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "TEST_BAR")
}

func Test_App_Group_Invalid(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "use: invalid handler int\n", func() {
		New().Group("/").Use(1)
	})
}

func Test_App_Group(t *testing.T) {
	t.Parallel()
	dummyHandler := testEmptyHandler

	app := New()

	grp := app.Group("/test")
	grp.Get("/", dummyHandler)
	testStatus200(t, app, "/test", MethodGet)

	grp.Get("/:demo?", dummyHandler)
	testStatus200(t, app, "/test/john", MethodGet)

	grp.Connect("/CONNECT", dummyHandler)
	testStatus200(t, app, "/test/CONNECT", MethodConnect)

	grp.Put("/PUT", dummyHandler)
	testStatus200(t, app, "/test/PUT", MethodPut)

	grp.Post("/POST", dummyHandler)
	testStatus200(t, app, "/test/POST", MethodPost)

	grp.Delete("/DELETE", dummyHandler)
	testStatus200(t, app, "/test/DELETE", MethodDelete)

	grp.Head("/HEAD", dummyHandler)
	testStatus200(t, app, "/test/HEAD", MethodHead)

	grp.Patch("/PATCH", dummyHandler)
	testStatus200(t, app, "/test/PATCH", MethodPatch)

	grp.Options("/OPTIONS", dummyHandler)
	testStatus200(t, app, "/test/OPTIONS", MethodOptions)

	grp.Trace("/TRACE", dummyHandler)
	testStatus200(t, app, "/test/TRACE", MethodTrace)

	grp.All("/ALL", dummyHandler)
	testStatus200(t, app, "/test/ALL", MethodPost)

	grp.Use(dummyHandler)
	testStatus200(t, app, "/test/oke", MethodGet)

	grp.Use("/USE", dummyHandler)
	testStatus200(t, app, "/test/USE/oke", MethodGet)

	api := grp.Group("/v1")
	api.Post("/", dummyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodPost, "/test/v1/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	api.Get("/users", dummyHandler)
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/v1/UsErS", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
}

func Test_App_RouteChain(t *testing.T) {
	t.Parallel()
	dummyHandler := testEmptyHandler

	app := New()

	register := app.RouteChain("/test").
		Get(dummyHandler).
		Head(dummyHandler).
		Post(dummyHandler).
		Put(dummyHandler).
		Delete(dummyHandler).
		Connect(dummyHandler).
		Options(dummyHandler).
		Trace(dummyHandler).
		Patch(dummyHandler)

	testStatus200(t, app, "/test", MethodGet)
	testStatus200(t, app, "/test", MethodHead)
	testStatus200(t, app, "/test", MethodPost)
	testStatus200(t, app, "/test", MethodPut)
	testStatus200(t, app, "/test", MethodDelete)
	testStatus200(t, app, "/test", MethodConnect)
	testStatus200(t, app, "/test", MethodOptions)
	testStatus200(t, app, "/test", MethodTrace)
	testStatus200(t, app, "/test", MethodPatch)

	register.RouteChain("/v1").Get(dummyHandler).Post(dummyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodPost, "/test/v1", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/v1", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	register.RouteChain("/v1").RouteChain("/v2").RouteChain("/v3").Get(dummyHandler).Trace(dummyHandler)

	resp, err = app.Test(httptest.NewRequest(MethodTrace, "/test/v1/v2/v3", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/v1/v2/v3", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
}

func Test_App_Route(t *testing.T) {
	t.Parallel()

	app := New()

	app.Route("/test", func(api Router) {
		api.Get("/foo", testEmptyHandler).Name("foo")

		api.Route("/bar", func(bar Router) {
			bar.Get("/", testEmptyHandler).Name("index")
		}, "bar.")
	}, "test.")

	testStatus200(t, app, "/test/foo", MethodGet)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test/bar/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, http.StatusOK, resp.StatusCode, "Status code")

	require.Equal(t, "/test/foo", app.GetRoute("test.foo").Path)
	require.Equal(t, "/test/bar/", app.GetRoute("test.bar.index").Path)
}

func Test_App_Route_nilFuncPanics(t *testing.T) {
	t.Parallel()

	app := New()

	require.PanicsWithValue(t, "route handler 'fn' cannot be nil", func() {
		app.Route("/panic", nil)
	})
}

func Test_Group_Route_nilFuncPanics(t *testing.T) {
	t.Parallel()

	app := New()
	grp := app.Group("/api")

	require.PanicsWithValue(t, "route handler 'fn' cannot be nil", func() {
		grp.Route("/panic", nil)
	})
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/users", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, http.StatusOK, resp.StatusCode, "Status code")
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

// go test -run Test_App_Next_Method
func Test_App_Next_Method(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use(func(c Ctx) error {
		require.Equal(t, MethodGet, c.Method())
		err := c.Next()
		require.Equal(t, MethodGet, c.Method())
		return err
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")
}

// go test -v -run=^$ -bench=Benchmark_NewError -benchmem -count=4
func Benchmark_NewError(b *testing.B) {
	for b.Loop() {
		NewError(200, "test") //nolint:errcheck // not needed
	}
}

// go test -run Test_NewError
func Test_NewError(t *testing.T) {
	t.Parallel()
	e := NewError(StatusForbidden, "permission denied")
	require.Equal(t, StatusForbidden, e.Code)
	require.Equal(t, "permission denied", e.Message)
}

// go test -run Test_NewError_Format
func Test_NewErrorf_Format(t *testing.T) {
	t.Parallel()

	type args []any

	tests := []struct {
		name string
		want string
		in   args
		code int
	}{
		{
			name: "no-args → default text",
			code: StatusNotFound,
			in:   nil,
			want: utils.StatusMessage(StatusNotFound),
		},
		{
			name: "single-string arg overrides",
			code: StatusBadRequest,
			in:   args{"custom bad request"},
			want: "custom bad request",
		},
		{
			name: "single non-string arg stringified",
			code: StatusInternalServerError,
			in:   args{errors.New("db down")},
			want: "db down",
		},
		{
			name: "single nil interface",
			code: StatusInternalServerError,
			in:   args{any(nil)},
			want: "<nil>",
		},
		{
			name: "format string + args",
			code: StatusBadRequest,
			in:   args{"invalid id %d", 10},
			want: "invalid id 10",
		},
		{
			name: "format string + excess args",
			code: StatusBadRequest,
			in:   args{"odd %d", 1, 2, 3},
			want: "odd 1%!(EXTRA int=2, int=3)",
		},
		{
			name: "≥2 args but first not string",
			code: StatusBadRequest,
			in:   args{errors.New("boom"), 42},
			want: "boom",
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

// go test -run Test_Test_Timeout
func Test_Test_Timeout(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody), TestConfig{
		Timeout: 0,
	})
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	app.Get("timeout", func(_ Ctx) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	_, err = app.Test(httptest.NewRequest(MethodGet, "/timeout", http.NoBody), TestConfig{
		Timeout:       20 * time.Millisecond,
		FailOnTimeout: true,
	})
	require.Error(t, err, "app.Test(req)")
}

type errorReader int

var errErrorReader = errors.New("errorReader")

func (errorReader) Read([]byte) (int, error) {
	return 0, errErrorReader
}

// go test -run Test_Test_DumpError
func Test_Test_DumpError(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", errorReader(0)))
	require.Nil(t, resp)
	require.ErrorIs(t, err, errErrorReader)
}

// go test -run Test_App_Handler
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

func Test_App_ReloadViews_Success(t *testing.T) {
	t.Parallel()
	view := &countingView{}
	app := New(Config{Views: view})
	initialLoads := view.loads

	require.NoError(t, app.ReloadViews())
	require.Equal(t, initialLoads+1, view.loads)

	require.NoError(t, app.ReloadViews())
	require.Equal(t, initialLoads+2, view.loads)
}

func Test_App_ReloadViews_Error(t *testing.T) {
	t.Parallel()
	wantedErr := errors.New("boom")
	view := &countingView{loadErr: wantedErr}
	app := New(Config{Views: view})

	err := app.ReloadViews()
	require.Error(t, err)
	require.ErrorIs(t, err, wantedErr)
}

func Test_App_ReloadViews_NoEngine(t *testing.T) {
	t.Parallel()
	app := New()

	err := app.ReloadViews()
	require.ErrorIs(t, err, ErrNoViewEngineConfigured)
}

func Test_App_ReloadViews_InterfaceNilPointer(t *testing.T) {
	t.Parallel()
	var view *countingView
	app := &App{config: Config{Views: view}}

	err := app.ReloadViews()
	require.ErrorIs(t, err, ErrNoViewEngineConfigured)
}

func Test_App_ReloadViews_MountedViews(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	templatePath := filepath.Join(tempDir, "template.html")

	require.NoError(t, os.WriteFile(templatePath, []byte("before"), 0o600))

	view := &fileView{path: templatePath}
	subApp := New(Config{Views: view})
	app := New()
	app.mount("/sub", subApp)

	require.NoError(t, view.Load())
	initialLoads := view.loads
	require.Equal(t, "before", view.content)

	require.NoError(t, os.WriteFile(templatePath, []byte("after"), 0o600))
	require.NoError(t, app.ReloadViews())

	require.Equal(t, "after", view.content)
	require.Greater(t, view.loads, initialLoads)
}

// go test -run Test_App_Init_Error_View
func Test_App_Init_Error_View(t *testing.T) {
	t.Parallel()
	app := New(Config{Views: invalidView{}})

	require.PanicsWithValue(t, "implement me", func() {
		//nolint:errcheck // not needed
		_ = app.config.Views.Render(nil, "", nil)
	})
}

// go test -run Test_App_Stack
func Test_App_Stack(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use("/path0", testEmptyHandler)
	app.Get("/path1", testEmptyHandler)
	app.Get("/path2", testEmptyHandler)
	app.Post("/path3", testEmptyHandler)

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
}

// go test -run Test_App_HandlersCount
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

// go test -run Test_App_ReadTimeout
func Test_App_ReadTimeout(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ReadTimeout:      time.Nanosecond,
		IdleTimeout:      time.Minute,
		DisableKeepalive: true,
	})

	ln, err := net.Listen(NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()

	app.Get("/read-timeout", func(c Ctx) error {
		return c.SendString("I should not be sent")
	})

	go func() {
		time.Sleep(500 * time.Millisecond)

		conn, err := net.Dial(NetworkTCP4, addr)
		assert.NoError(t, err)
		defer func(conn net.Conn) {
			closeErr := conn.Close()
			assert.NoError(t, closeErr)
		}(conn)

		_, err = conn.Write([]byte("HEAD /read-timeout HTTP/1.1\r\n"))
		assert.NoError(t, err)

		buf := make([]byte, 1024)
		var n int
		n, err = conn.Read(buf)

		assert.NoError(t, err)
		assert.True(t, bytes.Contains(buf[:n], []byte("408 Request Timeout")))

		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listener(ln, ListenConfig{DisableStartupMessage: true}))
}

// go test -run Test_App_BadRequest
func Test_App_BadRequest(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/bad-request", func(c Ctx) error {
		return c.SendString("I should not be sent")
	})

	ln, err := net.Listen(NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()

	go func() {
		time.Sleep(500 * time.Millisecond)
		conn, err := net.Dial(NetworkTCP4, addr)
		assert.NoError(t, err)
		defer func(conn net.Conn) {
			closeErr := conn.Close()
			assert.NoError(t, closeErr)
		}(conn)

		_, err = conn.Write([]byte("BadRequest\r\n"))
		assert.NoError(t, err)

		buf := make([]byte, 1024)
		var n int
		n, err = conn.Read(buf)

		assert.NoError(t, err)
		assert.True(t, bytes.Contains(buf[:n], []byte("400 Bad Request")))
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listener(ln, ListenConfig{DisableStartupMessage: true}))
}

// go test -run Test_App_SmallReadBuffer
func Test_App_SmallReadBuffer(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ReadBufferSize: 1,
	})

	app.Get("/small-read-buffer", func(c Ctx) error {
		return c.SendString("I should not be sent")
	})

	ln, err := net.Listen(NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()

	go func() {
		time.Sleep(500 * time.Millisecond)
		req, err := http.NewRequestWithContext(context.Background(), MethodGet, fmt.Sprintf("http://%s/small-read-buffer", addr), http.NoBody)
		assert.NoError(t, err)
		var client http.Client
		resp, err := client.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, 431, resp.StatusCode)
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listener(ln, ListenConfig{DisableStartupMessage: true}))
}

func Test_App_Server(t *testing.T) {
	t.Parallel()
	app := New()

	require.NotNil(t, app.Server())
}

func Test_App_Error_In_Fasthttp_Server(t *testing.T) {
	app := New()
	app.config.ErrorHandler = func(_ Ctx, _ error) error {
		return errors.New("fake error")
	}
	app.server.GetOnly = true

	resp, err := app.Test(httptest.NewRequest(MethodPost, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 500, resp.StatusCode)
}

// go test -race -run Test_App_New_Test_Parallel
func Test_App_New_Test_Parallel(t *testing.T) {
	t.Parallel()
	t.Run("Test_App_New_Test_Parallel_1", func(t *testing.T) {
		t.Parallel()
		app := New(Config{Immutable: true})
		_, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
		require.NoError(t, err)
	})
	t.Run("Test_App_New_Test_Parallel_2", func(t *testing.T) {
		t.Parallel()
		app := New(Config{Immutable: true})
		_, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
		require.NoError(t, err)
	})
}

func Test_App_ReadBodyStream(t *testing.T) {
	t.Parallel()
	app := New(Config{StreamRequestBody: true})
	app.Post("/", func(c Ctx) error {
		// Calling c.Body() automatically reads the entire stream.
		return c.SendString(fmt.Sprintf("%v %s", c.Request().IsBodyStream(), c.Body()))
	})
	testString := "this is a test"
	resp, err := app.Test(httptest.NewRequest(MethodPost, "/", bytes.NewBufferString(testString)))
	require.NoError(t, err, "app.Test(req)")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "io.ReadAll(resp.Body)")
	require.Equal(t, "true "+testString, string(body))
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
	b := &bytes.Buffer{}
	w := multipart.NewWriter(b)
	writer, err := w.CreateFormFile("test", "test")
	require.NoError(t, err, "w.CreateFormFile")
	n, err := writer.Write([]byte(testString))
	require.NoError(t, err, "writer.Write")
	require.Len(t, testString, n, "writer n")
	require.NoError(t, w.Close(), "w.Close()")

	req := httptest.NewRequest(MethodPost, "/", b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "io.ReadAll(resp.Body)")

	require.Equal(t, testString, string(body))
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
		_, err = app.Test(req, TestConfig{
			Timeout: 0,
		})
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

	app := New()
	app.Get("/", func(_ Ctx) error {
		time.Sleep(1 * time.Second)
		return nil
	})

	_, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody), TestConfig{
		Timeout:       100 * time.Millisecond,
		FailOnTimeout: true,
	})
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)
}

func Test_App_Test_timeout_empty_response(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(_ Ctx) error {
		time.Sleep(1 * time.Second)
		return nil
	})

	_, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody), TestConfig{
		Timeout:       100 * time.Millisecond,
		FailOnTimeout: false,
	})
	require.ErrorIs(t, err, ErrTestGotEmptyResponse)
}

func Test_App_Test_drop_empty_response(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(c Ctx) error {
		return c.Drop()
	})

	_, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody), TestConfig{
		Timeout:       0,
		FailOnTimeout: false,
	})
	require.ErrorIs(t, err, ErrTestGotEmptyResponse)
}

func Test_App_Test_response_error(t *testing.T) {
	// Note: Test cannot run in parallel due to
	// overriding the httpReadResponse global variable.
	// t.Parallel()

	// Override httpReadResponse temporarily
	oldHTTPReadResponse := httpReadResponse
	defer func() {
		httpReadResponse = oldHTTPReadResponse
	}()
	httpReadResponse = func(_ *bufio.Reader, _ *http.Request) (*http.Response, error) {
		return nil, errErrorReader
	}

	app := New()
	app.Get("/", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	_, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody), TestConfig{
		Timeout:       0,
		FailOnTimeout: false,
	})
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

	// Override httpReadResponse temporarily
	oldHTTPReadResponse := httpReadResponse
	defer func() {
		httpReadResponse = oldHTTPReadResponse
	}()

	httpReadResponse = func(r *bufio.Reader, req *http.Request) (*http.Response, error) {
		resp, err := http.ReadResponse(r, req)
		require.NoError(t, resp.Body.Close())
		resp.Body = errorReadCloser(0)
		return resp, err //nolint:wrapcheck // unnecessary to wrap it
	}

	app := New()
	hints := []string{"<https://cdn.com>; rel=preload; as=script"}
	app.Get("/early", func(c Ctx) error {
		err := c.SendEarlyHints(hints)
		require.NoError(t, err)
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

	// Override httpReadResponse temporarily
	oldHTTPReadResponse := httpReadResponse
	defer func() {
		httpReadResponse = oldHTTPReadResponse
	}()

	httpReadResponse = func(r *bufio.Reader, req *http.Request) (*http.Response, error) {
		resp, err := http.ReadResponse(r, req)
		_ = resp.Body.Close() //nolint:errcheck // It is fine to ignore the error here
		resp.Body = &doubleCloseBody{}
		return resp, err //nolint:wrapcheck // unnecessary to wrap it
	}

	app := New()
	hints := []string{"<https://cdn.com>; rel=preload; as=script"}
	app.Get("/early", func(c Ctx) error {
		err := c.SendEarlyHints(hints)
		require.NoError(t, err)
		return c.Status(StatusOK).SendString("done")
	})

	req := httptest.NewRequest(MethodGet, "/early", http.NoBody)
	_, err := app.Test(req)

	require.ErrorIs(t, err, errDoubleClose)
}

func Test_App_SetTLSHandler(t *testing.T) {
	t.Parallel()
	tlsHandler := &TLSHandler{clientHelloInfo: &tls.ClientHelloInfo{
		ServerName: "example.golang",
	}}

	app := New()
	app.SetTLSHandler(tlsHandler)

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	require.Equal(t, "example.golang", c.ClientHelloInfo().ServerName)
}

func Test_App_AddCustomRequestMethod(t *testing.T) {
	t.Parallel()
	methods := append(DefaultMethods, "TEST") //nolint:gocritic // We want a new slice here
	app := New(Config{
		RequestMethods: methods,
	})
	appMethods := app.config.RequestMethods

	// method name is always uppercase - https://datatracker.ietf.org/doc/html/rfc7231#section-4.1
	require.Len(t, app.stack, len(appMethods))
	require.Equal(t, "TEST", appMethods[len(appMethods)-1])
}

func Test_App_GetRoutes(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error {
		return c.Next()
	})
	handler := func(c Ctx) error {
		return c.SendStatus(StatusOK)
	}
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

	app.Get("/unnamed", func(c Ctx) error {
		return c.Next()
	})

	app.Post("/named", func(c Ctx) error {
		return c.Next()
	}).Name(named)

	app.Use(func(c Ctx) error {
		return c.Next()
	}) // no name - logging MW

	app.Use(func(c Ctx) error {
		return c.Next()
	}).Name("corsMW")

	app.Use(func(c Ctx) error {
		return c.Next()
	}).Name("compressMW")

	app.Use(func(c Ctx) error {
		return c.Next()
	}) // no name - cache MW

	grp := app.Group("/pages").Name("pages.")
	grp.Use(func(c Ctx) error {
		return c.Next()
	}).Name("csrfMW")

	grp.Get("/home", func(c Ctx) error {
		return c.Next()
	}).Name("home")

	grp.Get("/unnamed", func(c Ctx) error {
		return c.Next()
	})

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

	// Add testcase for routes use the same PATH on different methods
	app.Get("/users", emptyHandler).Name("get-users")
	app.Post("/users", emptyHandler).Name("add-user")
	getUsers := app.GetRoute("get-users")
	require.Equal(t, "/users", getUsers.Path)

	addUser := app.GetRoute("add-user")
	require.Equal(t, "/users", addUser.Path)

	// Add testcase for routes use the same PATH on different methods (for groups)
	newGrp := app.Group("/name-test")
	newGrp.Get("/users", emptyHandler).Name("grp-get-users")
	newGrp.Post("/users", emptyHandler).Name("grp-add-user")
	getUsers = app.GetRoute("grp-get-users")
	require.Equal(t, "/name-test/users", getUsers.Path)

	addUser = app.GetRoute("grp-add-user")
	require.Equal(t, "/name-test/users", addUser.Path)

	// Add testcase for HEAD route naming
	app.Get("/simple-route", emptyHandler).Name("simple-route")
	app.Head("/simple-route", emptyHandler).Name("simple-route2")

	sRoute := app.GetRoute("simple-route")
	require.Equal(t, "/simple-route", sRoute.Path)

	sRoute2 := app.GetRoute("simple-route2")
	require.Equal(t, "/simple-route", sRoute2.Path)
}

func Test_App_State(t *testing.T) {
	t.Parallel()
	app := New()

	app.State().Set("key", "value")
	str, ok := app.State().GetString("key")
	require.True(t, ok)
	require.Equal(t, "value", str)
}

// go test -v -run=^$ -bench=Benchmark_Communication_Flow -benchmem -count=4
func Benchmark_Communication_Flow(b *testing.B) {
	app := New()

	app.Get("/", func(c Ctx) error {
		return c.SendString("Hello, World!")
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	fctx.Request.SetRequestURI("/")

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	require.Equal(b, 200, fctx.Response.Header.StatusCode())
	require.Equal(b, "Hello, World!", string(fctx.Response.Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcquireReleaseFlow -benchmem -count=4
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

func TestErrorHandler_PicksRightOne(t *testing.T) {
	t.Parallel()
	// common handler to be used by all routes,
	// it will always fail by returning an error since
	// we need to test that the right ErrorHandler is invoked
	handler := func(_ Ctx) error {
		return errors.New("random error")
	}

	// subapp /api/v1/users [no custom error handler]
	appAPIV1Users := New()
	appAPIV1Users.Get("/", handler)

	// subapp /api/v1/use [with custom error handler]
	appAPIV1UseEH := func(c Ctx, _ error) error {
		return c.SendString("/api/v1/use error handler")
	}
	appAPIV1Use := New(Config{ErrorHandler: appAPIV1UseEH})
	appAPIV1Use.Get("/", handler)

	// subapp: /api/v1 [with custom error handler]
	appV1EH := func(c Ctx, _ error) error {
		return c.SendString("/api/v1 error handler")
	}
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
			resp, err := app.Test(httptest.NewRequest(MethodGet, testCase.path, http.NoBody))
			if err != nil {
				t.Fatal(err)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}

			require.Equal(t, testCase.expected, string(body))
		})
	}
}
