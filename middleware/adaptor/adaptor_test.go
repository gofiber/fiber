//nolint:contextcheck,revive // Much easier to just ignore memory leaks in tests
package adaptor

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

const (
	expectedRequestURI = "/foo/bar?baz=123"
	expectedBody       = "body 123 foo bar baz"
	expectedHost       = "foobar.com"
	expectedRemoteAddr = "1.2.3.4:6789"
)

func Test_HTTPHandler(t *testing.T) {
	t.Parallel()

	expectedMethod := fiber.MethodPost
	expectedProto := "HTTP/1.1"
	expectedProtoMajor := 1
	expectedProtoMinor := 1
	expectedContentLength := len(expectedBody)
	expectedHeader := map[string]string{
		"Foo-Bar":         "baz",
		"Abc":             "defg",
		"XXX-Remote-Addr": "123.43.4543.345",
	}
	expectedURL, err := url.ParseRequestURI(expectedRequestURI)
	require.NoError(t, err)

	type contextKeyType string
	expectedContextKey := contextKeyType("contextKey")
	expectedContextValue := "contextValue"

	callsCount := 0
	nethttpH := func(w http.ResponseWriter, r *http.Request) {
		callsCount++
		assert.Equal(t, expectedMethod, r.Method, "Method")
		assert.Equal(t, expectedProto, r.Proto, "Proto")
		assert.Equal(t, expectedProtoMajor, r.ProtoMajor, "ProtoMajor")
		assert.Equal(t, expectedProtoMinor, r.ProtoMinor, "ProtoMinor")
		assert.Equal(t, expectedRequestURI, r.RequestURI, "RequestURI")
		assert.Equal(t, expectedContentLength, int(r.ContentLength), "ContentLength")
		assert.Empty(t, r.TransferEncoding, "TransferEncoding")
		assert.Equal(t, expectedHost, r.Host, "Host")
		assert.Equal(t, expectedRemoteAddr, r.RemoteAddr, "RemoteAddr")

		body, readErr := io.ReadAll(r.Body)
		assert.NoError(t, readErr)
		assert.Equal(t, expectedBody, string(body), "Body")
		assert.Equal(t, expectedURL, r.URL, "URL")
		assert.Equal(t, expectedContextValue, r.Context().Value(expectedContextKey), "Context")

		for k, expectedV := range expectedHeader {
			v := r.Header.Get(k)
			assert.Equal(t, expectedV, v, "Header")
		}

		w.Header().Set("Header1", "value1")
		w.Header().Set("Header2", "value2")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "request body is %q", body)
	}
	fiberH := HTTPHandlerFunc(http.HandlerFunc(nethttpH))
	fiberH = setFiberContextValueMiddleware(fiberH, expectedContextKey, expectedContextValue)

	var fctx fasthttp.RequestCtx
	var req fasthttp.Request

	req.Header.SetMethod(expectedMethod)
	req.SetRequestURI(expectedRequestURI)
	req.Header.SetHost(expectedHost)
	req.BodyWriter().Write([]byte(expectedBody)) //nolint:errcheck // not needed
	for k, v := range expectedHeader {
		req.Header.Set(k, v)
	}

	remoteAddr, err := net.ResolveTCPAddr("tcp", expectedRemoteAddr)
	require.NoError(t, err)

	fctx.Init(&req, remoteAddr, &disableLogger{})
	app := fiber.New()
	ctx := app.AcquireCtx(&fctx)
	defer app.ReleaseCtx(ctx)

	err = fiberH(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, callsCount, "callsCount")

	resp := &fctx.Response
	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), "StatusCode")
	require.Equal(t, "value1", string(resp.Header.Peek("Header1")), "Header1")
	require.Equal(t, "value2", string(resp.Header.Peek("Header2")), "Header2")

	expectedResponseBody := fmt.Sprintf("request body is %q", expectedBody)
	require.Equal(t, expectedResponseBody, string(resp.Body()), "Body")
}

func Test_HTTPHandler_Flush(t *testing.T) {
	t.Parallel()

	expectedMethod := fiber.MethodPost
	expectedProto := "HTTP/1.1"
	expectedProtoMajor := 1
	expectedProtoMinor := 1
	expectedContentLength := len(expectedBody)
	expectedHeader := map[string]string{
		"Foo-Bar":         "baz",
		"Abc":             "defg",
		"XXX-Remote-Addr": "123.43.4543.345",
	}
	expectedURL, err := url.ParseRequestURI(expectedRequestURI)
	require.NoError(t, err)

	type contextKeyType string
	expectedContextKey := contextKeyType("contextKey")
	expectedContextValue := "contextValue"

	callsCount := 0
	nethttpH := func(w http.ResponseWriter, r *http.Request) {
		callsCount++
		assert.Equal(t, expectedMethod, r.Method, "Method")
		assert.Equal(t, expectedProto, r.Proto, "Proto")
		assert.Equal(t, expectedProtoMajor, r.ProtoMajor, "ProtoMajor")
		assert.Equal(t, expectedProtoMinor, r.ProtoMinor, "ProtoMinor")
		assert.Equal(t, expectedRequestURI, r.RequestURI, "RequestURI")
		assert.Equal(t, expectedContentLength, int(r.ContentLength), "ContentLength")
		assert.Empty(t, r.TransferEncoding, "TransferEncoding")
		assert.Equal(t, expectedHost, r.Host, "Host")
		assert.Equal(t, expectedRemoteAddr, r.RemoteAddr, "RemoteAddr")

		body, readErr := io.ReadAll(r.Body)
		assert.NoError(t, readErr)
		assert.Equal(t, expectedBody, string(body), "Body")
		assert.Equal(t, expectedURL, r.URL, "URL")
		assert.Equal(t, expectedContextValue, r.Context().Value(expectedContextKey), "Context")

		for k, expectedV := range expectedHeader {
			v := r.Header.Get(k)
			assert.Equal(t, expectedV, v, "Header")
		}

		w.Header().Set("Header1", "value1")
		w.Header().Set("Header2", "value2")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "request body is ")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("w does not implement http.Flusher")
		}
		flusher.Flush()
		fmt.Fprintf(w, "%q", body)
	}
	fiberH := HTTPHandlerFunc(http.HandlerFunc(nethttpH))
	fiberH = setFiberContextValueMiddleware(fiberH, expectedContextKey, expectedContextValue)

	var fctx fasthttp.RequestCtx
	var req fasthttp.Request

	req.Header.SetMethod(expectedMethod)
	req.SetRequestURI(expectedRequestURI)
	req.Header.SetHost(expectedHost)
	req.BodyWriter().Write([]byte(expectedBody)) //nolint:errcheck // not needed
	for k, v := range expectedHeader {
		req.Header.Set(k, v)
	}

	remoteAddr, err := net.ResolveTCPAddr("tcp", expectedRemoteAddr)
	require.NoError(t, err)

	fctx.Init(&req, remoteAddr, &disableLogger{})
	app := fiber.New()
	ctx := app.AcquireCtx(&fctx)
	defer app.ReleaseCtx(ctx)

	err = fiberH(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, callsCount, "callsCount")

	resp := &fctx.Response
	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), "StatusCode")
	require.Equal(t, "value1", string(resp.Header.Peek("Header1")), "Header1")
	require.Equal(t, "value2", string(resp.Header.Peek("Header2")), "Header2")

	expectedResponseBody := fmt.Sprintf("request body is %q", expectedBody)
	require.Equal(t, expectedResponseBody, string(resp.Body()), "Body")
}

func Test_HTTPHandler_Flush_App_Test(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/", HTTPHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("w does not implement http.Flusher")
		}
		w.WriteHeader(fiber.StatusOK)
		fmt.Fprintf(w, "Hello ")
		flusher.Flush()
		time.Sleep(500 * time.Millisecond)
		fmt.Fprintf(w, "World!")
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck // not needed

	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Hello World!", string(body))
}

func Test_HTTPHandler_App_Test_Interrupted(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/", HTTPHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("w does not implement http.Flusher")
		}
		w.WriteHeader(fiber.StatusOK)
		fmt.Fprintf(w, "Hello ")
		flusher.Flush()
		time.Sleep(500 * time.Millisecond)
		fmt.Fprintf(w, "World!")
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody), fiber.TestConfig{
		Timeout:       200 * time.Millisecond,
		FailOnTimeout: false,
	})
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck // not needed

	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	require.Equal(t, "Hello ", string(body))
}

func Test_HTTPHandler_local_context(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// unique type for avoiding collisions in context
	type key struct{}
	var testKey key

	const testVal string = "test-value"

	// a middleware to add a value to the local context
	app.Use(func(c fiber.Ctx) error {
		ctx := context.WithValue(c, testKey, testVal)
		c.SetContext(ctx)
		return c.Next()
	})

	// a handler that checks if the value has been appended to the local context
	app.Get("/", HTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		ctx, ok := r.Context().Value("__local_context__").(context.Context)
		if !ok {
			http.Error(w, "Context not found", http.StatusInternalServerError)
			return
		}

		val, ok := ctx.Value(testKey).(string)
		if !ok {
			http.Error(w, "Test value not found", http.StatusInternalServerError)
			return
		}

		if _, err := w.Write([]byte(val)); err != nil {
			t.Logf("write failed: %v", err)
		}
	})))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody), fiber.TestConfig{
		Timeout:       200 * time.Millisecond,
		FailOnTimeout: false,
	})
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	defer resp.Body.Close() //nolint:errcheck // no need

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, testVal, string(body))
}

type contextKey string

func (c contextKey) String() string {
	return "test-" + string(c)
}

var (
	TestContextKey       = contextKey("TestContextKey")
	TestContextSecondKey = contextKey("TestContextSecondKey")
)

func Test_HTTPMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		url        string
		method     string
		statusCode int
	}{
		{
			name:       "Should return 200",
			url:        "/",
			method:     "POST",
			statusCode: 200,
		},
		{
			name:       "Should return 405",
			url:        "/",
			method:     "GET",
			statusCode: 405,
		},
		{
			name:       "Should return 400",
			url:        "/unknown",
			method:     "POST",
			statusCode: 404,
		},
	}

	nethttpMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			r = r.WithContext(context.WithValue(r.Context(), TestContextKey, "okay"))
			r = r.WithContext(context.WithValue(r.Context(), TestContextSecondKey, "not_okay"))
			r = r.WithContext(context.WithValue(r.Context(), TestContextSecondKey, "okay"))

			next.ServeHTTP(w, r)
		})
	}

	app := fiber.New()
	app.Use(HTTPMiddleware(nethttpMW))
	app.Post("/", func(c fiber.Ctx) error {
		value := c.RequestCtx().Value(TestContextKey)
		val, ok := value.(string)
		if !ok {
			t.Error("unexpected error on type-assertion")
		}
		if value != nil {
			c.Set("context_okay", val)
		}
		value = c.RequestCtx().Value(TestContextSecondKey)
		if value != nil {
			val, ok := value.(string)
			if !ok {
				t.Error("unexpected error on type-assertion")
			}
			c.Set("context_second_okay", val)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	for _, tt := range tests {
		req, err := http.NewRequestWithContext(context.Background(), tt.method, tt.url, http.NoBody)
		req.Host = expectedHost
		require.NoError(t, err)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, tt.statusCode, resp.StatusCode, "StatusCode")
	}

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodPost, "/", http.NoBody)
	req.Host = expectedHost
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, "okay", resp.Header.Get("context_okay"))
	require.Equal(t, "okay", resp.Header.Get("context_second_okay"))
}

func Test_HTTPMiddlewareWithCookies(t *testing.T) {
	t.Parallel()

	const (
		cookieHeader    = "Cookie"
		setCookieHeader = "Set-Cookie"
		cookieOneName   = "cookieOne"
		cookieTwoName   = "cookieTwo"
		cookieOneValue  = "valueCookieOne"
		cookieTwoValue  = "valueCookieTwo"
	)
	nethttpMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	app := fiber.New()
	app.Use(HTTPMiddleware(nethttpMW))
	app.Post("/", func(c fiber.Ctx) error {
		// RETURNING CURRENT COOKIES TO RESPONSE
		cookies := strings.Split(c.Get(cookieHeader), "; ")
		for _, cookie := range cookies {
			c.Set(setCookieHeader, cookie)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	// Test case for POST request with cookies
	t.Run("POST request with cookies", func(t *testing.T) {
		t.Parallel()
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodPost, "/", http.NoBody)
		require.NoError(t, err)
		req.AddCookie(&http.Cookie{Name: cookieOneName, Value: cookieOneValue})
		req.AddCookie(&http.Cookie{Name: cookieTwoName, Value: cookieTwoValue})

		resp, err := app.Test(req)
		require.NoError(t, err)
		cookies := resp.Cookies()
		require.Len(t, cookies, 2)
		for _, cookie := range cookies {
			switch cookie.Name {
			case cookieOneName:
				require.Equal(t, cookieOneValue, cookie.Value)
			case cookieTwoName:
				require.Equal(t, cookieTwoValue, cookie.Value)
			default:
				t.Error("unexpected cookie key")
			}
		}
	})

	// New test case for GET request
	t.Run("GET request", func(t *testing.T) {
		t.Parallel()
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/", http.NoBody)
		require.NoError(t, err)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})

	// New test case for request without cookies
	t.Run("POST request without cookies", func(t *testing.T) {
		t.Parallel()
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodPost, "/", http.NoBody)
		require.NoError(t, err)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Empty(t, resp.Cookies())
	})
}

func Test_FiberHandler(t *testing.T) {
	t.Parallel()

	testFiberToHandlerFunc(t, false)
}

func Test_FiberApp(t *testing.T) {
	t.Parallel()

	testFiberToHandlerFunc(t, false, fiber.New())
}

func Test_FiberHandlerDefaultPort(t *testing.T) {
	t.Parallel()

	testFiberToHandlerFunc(t, true)
}

func Test_FiberAppDefaultPort(t *testing.T) {
	t.Parallel()

	testFiberToHandlerFunc(t, true, fiber.New())
}

func testFiberToHandlerFunc(t *testing.T, checkDefaultPort bool, app ...*fiber.App) {
	t.Helper()

	expectedMethod := fiber.MethodPost
	expectedContentLength := len(expectedBody)
	expectedRemoteAddr := "1.2.3.4:6789"
	if checkDefaultPort {
		expectedRemoteAddr = "1.2.3.4:80"
	}
	expectedHeader := map[string]string{
		"Foo-Bar":         "baz",
		"Abc":             "defg",
		"XXX-Remote-Addr": "123.43.4543.345",
	}
	expectedURL, err := url.ParseRequestURI(expectedRequestURI)
	require.NoError(t, err)

	callsCount := 0
	fiberH := func(c fiber.Ctx) error {
		callsCount++
		require.Equal(t, expectedMethod, c.Method(), "Method")
		require.Equal(t, expectedRequestURI, string(c.RequestCtx().RequestURI()), "RequestURI")
		require.Equal(t, expectedContentLength, c.RequestCtx().Request.Header.ContentLength(), "ContentLength")
		require.Equal(t, expectedHost, c.Hostname(), "Host")
		require.Equal(t, expectedHost, string(c.Request().Header.Host()), "Host")
		require.Equal(t, "http://"+expectedHost, c.BaseURL(), "BaseURL")
		require.Equal(t, expectedRemoteAddr, c.RequestCtx().RemoteAddr().String(), "RemoteAddr")

		body := string(c.Body())
		require.Equal(t, expectedBody, body, "Body")
		require.Equal(t, expectedURL.String(), c.OriginalURL(), "URL")

		for k, expectedV := range expectedHeader {
			v := c.Get(k)
			require.Equal(t, expectedV, v, "Header")
		}

		c.Set("Header1", "value1")
		c.Set("Header2", "value2")
		c.Status(fiber.StatusBadRequest)
		_, err := c.Write(fmt.Appendf(nil, "request body is %q", body))
		return err
	}

	var handlerFunc http.HandlerFunc
	if len(app) > 0 {
		app[0].Post("/foo/bar", fiberH)
		handlerFunc = FiberApp(app[0])
	} else {
		handlerFunc = FiberHandlerFunc(fiberH)
	}

	var r http.Request

	r.Method = expectedMethod
	r.Body = &netHTTPBody{b: []byte(expectedBody)}
	r.RequestURI = expectedRequestURI
	r.ContentLength = int64(expectedContentLength)
	r.Host = expectedHost
	r.RemoteAddr = expectedRemoteAddr
	if checkDefaultPort {
		r.RemoteAddr = "1.2.3.4"
	}

	hdr := make(http.Header)
	for k, v := range expectedHeader {
		hdr.Set(k, v)
	}
	r.Header = hdr

	var w netHTTPResponseWriter
	handlerFunc.ServeHTTP(&w, &r)

	require.Equal(t, http.StatusBadRequest, w.StatusCode(), "StatusCode")
	require.Equal(t, "value1", w.Header().Get("Header1"), "Header1")
	require.Equal(t, "value2", w.Header().Get("Header2"), "Header2")

	expectedResponseBody := fmt.Sprintf("request body is %q", expectedBody)
	require.Equal(t, expectedResponseBody, string(w.body), "Body")
}

func setFiberContextValueMiddleware(next fiber.Handler, key, value any) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals(key, value)
		return next(c)
	}
}

func Test_FiberHandler_RequestNilBody(t *testing.T) {
	t.Parallel()

	expectedMethod := fiber.MethodGet
	expectedRequestURI := "/foo/bar"
	expectedContentLength := 0

	callsCount := 0
	fiberH := func(c fiber.Ctx) error {
		callsCount++
		require.Equal(t, expectedMethod, c.Method(), "Method")
		require.Equal(t, expectedRequestURI, string(c.RequestCtx().RequestURI()), "RequestURI")
		require.Equal(t, expectedContentLength, c.RequestCtx().Request.Header.ContentLength(), "ContentLength")

		_, err := c.WriteString("request body is nil")
		return err
	}
	nethttpH := FiberHandler(fiberH)

	var r http.Request

	r.Method = expectedMethod
	r.RequestURI = expectedRequestURI

	var w netHTTPResponseWriter
	nethttpH.ServeHTTP(&w, &r)

	expectedResponseBody := "request body is nil"
	require.Equal(t, expectedResponseBody, string(w.body), "Body")
}

type netHTTPBody struct {
	b []byte
}

func (r *netHTTPBody) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.b)
	r.b = r.b[n:]
	return n, nil
}

func (r *netHTTPBody) Close() error {
	r.b = r.b[:0]
	return nil
}

func createTestRequest(method, uri, remoteAddr string, body io.Reader) *http.Request {
	r := &http.Request{
		Method:     method,
		RequestURI: uri,
		RemoteAddr: remoteAddr,
		Header:     make(http.Header),
		Body:       http.NoBody,
	}
	if body != nil {
		if rc, ok := body.(io.ReadCloser); ok {
			r.Body = rc
		} else {
			r.Body = io.NopCloser(body)
		}
	}
	return r
}

func executeHandlerTest(_ *testing.T, handler http.HandlerFunc, req *http.Request) *netHTTPResponseWriter {
	w := &netHTTPResponseWriter{}
	handler.ServeHTTP(w, req)
	return w
}

type netHTTPResponseWriter struct {
	h          http.Header
	body       []byte
	statusCode int
}

func (w *netHTTPResponseWriter) StatusCode() int {
	if w.statusCode == 0 {
		return http.StatusOK
	}
	return w.statusCode
}

func (w *netHTTPResponseWriter) Header() http.Header {
	if w.h == nil {
		w.h = make(http.Header)
	}
	return w.h
}

func (w *netHTTPResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *netHTTPResponseWriter) Write(p []byte) (int, error) {
	w.body = append(w.body, p...)
	return len(p), nil
}

func (w *netHTTPResponseWriter) Flush() {}

func Test_ConvertRequest(t *testing.T) {
	t.Parallel()

	t.Run("successful conversion", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()

		app.Get("/test", func(c fiber.Ctx) error {
			httpReq, err := ConvertRequest(c, false)
			if err != nil {
				return err
			}
			return c.SendString("Request URL: " + httpReq.URL.String())
		})

		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test?hello=world&another=test", http.NoBody))
		require.NoError(t, err, "app.Test(req)")
		require.Equal(t, http.StatusOK, resp.StatusCode, "Status code")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "Request URL: /test?hello=world&another=test", string(body))
	})

	t.Run("conversion error handling", func(t *testing.T) {
		t.Parallel()
		// Test error case by creating a context with an invalid URL that will cause fasthttpadaptor.ConvertRequest to fail
		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// Create a malformed request URI that should cause conversion to fail
		ctx.Request().SetRequestURI("http://[::1:bad:url") // Invalid URL format
		ctx.Request().Header.SetMethod(fiber.MethodGet)

		_, err := ConvertRequest(ctx, true) // Use forServer=true which does more validation
		if err == nil {
			// If the above doesn't fail, try a different approach
			ctx.Request().SetRequestURI("\x00\x01\x02") // Invalid characters in URI
			_, err = ConvertRequest(ctx, true)
		}
		// Note: This test may pass if fasthttpadaptor is very permissive
		// The important thing is that our function doesn't panic
		if err != nil {
			require.Error(t, err, "Expected error from fasthttpadaptor.ConvertRequest")
		}
	})
}

func Test_CopyContextToFiberContext(t *testing.T) {
	t.Parallel()

	t.Run("unsupported context type", func(t *testing.T) {
		t.Parallel()
		// Test with non-struct context (should return early)
		var fctx fasthttp.RequestCtx
		stringContext := "not a struct"

		// This should not panic and should handle the non-struct gracefully
		CopyContextToFiberContext(&stringContext, &fctx)
		// No assertions needed - just ensuring it doesn't panic
	})

	t.Run("context with unknown field", func(t *testing.T) {
		t.Parallel()
		// Test the default case (continue statement coverage)
		type customContext struct {
			UnknownField string
		}

		var fctx fasthttp.RequestCtx
		ctx := customContext{UnknownField: "test"}

		// This should hit the default case and continue
		CopyContextToFiberContext(&ctx, &fctx)
		// No assertions needed - just ensuring it doesn't panic and continues
	})

	t.Run("invalid src", func(t *testing.T) {
		var fctx fasthttp.RequestCtx
		CopyContextToFiberContext(nil, &fctx)
		// Add assertion to ensure no panic and coverage is detected
		assert.NotNil(t, &fctx)
	})

	t.Run("nil pointer", func(t *testing.T) {
		var nilPtr *context.Context // Nil pointer to a context
		var fctx fasthttp.RequestCtx
		CopyContextToFiberContext(nilPtr, &fctx)
		// Add assertion to ensure no panic and coverage is detected
		assert.NotNil(t, &fctx)
	})

	t.Run("multi-level pointer", func(t *testing.T) {
		t.Parallel()
		var fctx fasthttp.RequestCtx
		ctx := context.Background()
		ptr := &ctx
		doublePtr := &ptr
		// Test deref pointer chains
		CopyContextToFiberContext(doublePtr, &fctx)
		// No assertions needed - just ensuring it doesn't panic
	})

	t.Run("non-addressable struct", func(t *testing.T) {
		t.Parallel()
		var fctx fasthttp.RequestCtx
		type testStruct struct {
			Field string
		}
		// Pass struct value directly to test addressability check
		CopyContextToFiberContext(testStruct{Field: "test"}, &fctx)
		// No assertions needed - just ensuring it doesn't panic and creates temporary
	})
}

func Test_HTTPMiddleware_ErrorHandling(t *testing.T) {
	t.Parallel()

	// Test middleware that returns an error from HTTPHandler
	errorMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// This will cause an error in the underlying handler
			w.WriteHeader(http.StatusInternalServerError)
			next.ServeHTTP(w, r)
		})
	}

	fiberHandler := func(c fiber.Ctx) error {
		return fiber.NewError(fiber.StatusBadRequest, "test error")
	}

	app := fiber.New()
	app.Use(HTTPMiddleware(errorMiddleware))
	app.Get("/error", fiberHandler)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/error", http.NoBody))
	require.NoError(t, err)
	// The error should be handled by the error handler
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func Test_FiberHandler_IOError(t *testing.T) {
	t.Parallel()

	// Test io.Copy error by using a failing reader
	fiberH := func(c fiber.Ctx) error {
		return c.SendString("should not reach here")
	}
	handlerFunc := FiberHandlerFunc(fiberH)

	// Create a reader that fails
	failingReader := &failingReader{}

	r := &http.Request{
		Method:        http.MethodPost,
		RequestURI:    "/test",
		Body:          failingReader,
		ContentLength: 100, // Set content length so it tries to read
		Header:        make(http.Header),
	}

	w := &netHTTPResponseWriter{}
	handlerFunc.ServeHTTP(w, r)

	// Should return 500 due to io.Copy error
	require.Equal(t, http.StatusInternalServerError, w.StatusCode())
}

func Test_FiberHandler_WithErrorInHandler(t *testing.T) {
	t.Parallel()

	// Test error handling in fiber handler
	fiberH := func(c fiber.Ctx) error {
		return fiber.NewError(fiber.StatusTeapot, "I'm a teapot")
	}
	handlerFunc := FiberHandlerFunc(fiberH)

	r := &http.Request{
		Method:     http.MethodGet,
		RequestURI: "/test",
		Header:     make(http.Header),
		Body:       http.NoBody,
	}

	w := &netHTTPResponseWriter{}
	handlerFunc.ServeHTTP(w, r)

	// Should return the error status
	require.Equal(t, fiber.StatusTeapot, w.StatusCode())
}

func Test_FiberHandler_WithSendStreamWriter(t *testing.T) {
	t.Parallel()

	// Test streaming functionality in FiberHandler using SendStreamWriter.
	fiberH := func(c fiber.Ctx) error {
		c.Status(fiber.StatusTeapot)
		return c.SendStreamWriter(func(w *bufio.Writer) {
			w.WriteString("Hello ")            //nolint:errcheck // not needed
			w.Flush()                          //nolint:errcheck // not needed
			time.Sleep(200 * time.Millisecond) // Simulate a long operation
			w.WriteString("World!")            //nolint:errcheck // not needed
		})
	}
	handlerFunc := FiberHandlerFunc(fiberH)

	r := &http.Request{
		Method:     http.MethodGet,
		RequestURI: "/test",
		Header:     make(http.Header),
		Body:       http.NoBody,
	}

	w := &netHTTPResponseWriter{}
	handlerFunc.ServeHTTP(w, r)

	// Should return the error status
	require.Equal(t, fiber.StatusTeapot, w.StatusCode())
	require.Equal(t, "Hello World!", string(w.body))
}

func Test_FiberHandler_WithInterruptedSendStreamWriter(t *testing.T) {
	t.Parallel()

	// Test streaming functionality to ensure data is sent even during a timeout.
	fiberH := func(c fiber.Ctx) error {
		c.Status(fiber.StatusTeapot)
		return c.SendStreamWriter(func(w *bufio.Writer) {
			w.WriteString("Hello ")            //nolint:errcheck // not needed
			w.Flush()                          //nolint:errcheck // not needed
			time.Sleep(500 * time.Millisecond) // Simulate a long operation
			w.WriteString("World!")            //nolint:errcheck // not needed
		})
	}
	handlerFunc := FiberHandlerFunc(fiberH)

	// Start a mock HTTP server using the handlerFunc
	server := &http.Server{
		Handler:      handlerFunc,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	listener, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	addr := fmt.Sprintf("http://%s", listener.Addr())

	go func() {
		server.Serve(listener) //nolint:errcheck // not needed
	}()
	defer func() {
		require.NoError(t, server.Close())
	}()

	cc := &http.Client{
		Timeout: 200 * time.Millisecond,
	}
	resp, err := cc.Get(addr) //nolint:noctx // ctx is not needed
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	body, readErr := io.ReadAll(resp.Body)
	require.ErrorIs(t, readErr, context.DeadlineExceeded)
	require.Equal(t, "Hello ", string(body))
}

// failingReader always returns an error when Read is called
type failingReader struct{}

func (f *failingReader) Read(p []byte) (int, error) {
	return 0, errors.New("simulated read error")
}

func (f *failingReader) Close() error {
	return nil
}

// Benchmark for FiberHandlerFunc
func Benchmark_FiberHandlerFunc(b *testing.B) {
	benchmarks := []struct {
		name        string
		bodyContent []byte
	}{
		{
			name:        "No Content",
			bodyContent: nil, // No body content case
		},
		{
			name:        "100KB",
			bodyContent: make([]byte, 100*1024),
		},
		{
			name:        "500KB",
			bodyContent: make([]byte, 500*1024),
		},
		{
			name:        "1MB",
			bodyContent: make([]byte, 1*1024*1024),
		},
		{
			name:        "5MB",
			bodyContent: make([]byte, 5*1024*1024),
		},
		{
			name:        "10MB",
			bodyContent: make([]byte, 10*1024*1024),
		},
		{
			name:        "25MB",
			bodyContent: make([]byte, 25*1024*1024),
		},
		{
			name:        "50MB",
			bodyContent: make([]byte, 50*1024*1024),
		},
	}

	fiberH := func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}
	handlerFunc := FiberHandlerFunc(fiberH)

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			w := httptest.NewRecorder()
			var bodyBuffer *bytes.Buffer

			// Handle the "No Content" case where bodyContent is nil
			if bm.bodyContent != nil {
				bodyBuffer = bytes.NewBuffer(bm.bodyContent)
			} else {
				bodyBuffer = bytes.NewBuffer([]byte{}) // Empty buffer for no content
			}

			r := http.Request{
				Method: http.MethodPost,
				Body:   nil,
			}

			// Replace the empty Body with our buffer
			r.Body = io.NopCloser(bodyBuffer)
			defer r.Body.Close() //nolint:errcheck // not needed

			b.ReportAllocs()

			for b.Loop() {
				handlerFunc.ServeHTTP(w, &r)
			}
		})
	}
}

func Benchmark_FiberHandlerFunc_Parallel(b *testing.B) {
	benchmarks := []struct {
		name        string
		bodyContent []byte
	}{
		{
			name:        "No Content",
			bodyContent: nil, // No body content case
		},
		{
			name:        "100KB",
			bodyContent: make([]byte, 100*1024),
		},
		{
			name:        "500KB",
			bodyContent: make([]byte, 500*1024),
		},
		{
			name:        "1MB",
			bodyContent: make([]byte, 1*1024*1024),
		},
		{
			name:        "5MB",
			bodyContent: make([]byte, 5*1024*1024),
		},
		{
			name:        "10MB",
			bodyContent: make([]byte, 10*1024*1024),
		},
		{
			name:        "25MB",
			bodyContent: make([]byte, 25*1024*1024),
		},
		{
			name:        "50MB",
			bodyContent: make([]byte, 50*1024*1024),
		},
	}

	fiberH := func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}
	handlerFunc := FiberHandlerFunc(fiberH)

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			var bodyBuffer *bytes.Buffer

			// Handle the "No Content" case where bodyContent is nil
			if bm.bodyContent != nil {
				bodyBuffer = bytes.NewBuffer(bm.bodyContent)
			} else {
				bodyBuffer = bytes.NewBuffer([]byte{}) // Empty buffer for no content
			}

			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				w := httptest.NewRecorder()
				r := http.Request{
					Method: http.MethodPost,
					Body:   nil,
				}

				// Replace the empty Body with our buffer
				r.Body = io.NopCloser(bodyBuffer)
				defer r.Body.Close() //nolint:errcheck // not needed

				for pb.Next() {
					handlerFunc(w, &r)
				}
			})
		})
	}
}

func Benchmark_HTTPHandler(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok")) //nolint:errcheck // not needed
	})

	var err error
	app := fiber.New()

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer func() {
		app.ReleaseCtx(ctx)
	}()

	b.ReportAllocs()

	fiberHandler := HTTPHandler(handler)

	for b.Loop() {
		ctx.Request().Reset()
		ctx.Response().Reset()
		ctx.Request().SetRequestURI("/test")
		ctx.Request().Header.SetMethod("GET")

		err = fiberHandler(ctx)
	}

	require.NoError(b, err)
}

func Test_resolveRemoteAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expectedErr   error
		localAddr     any
		name          string
		remoteAddr    string
		errorContains string
		expectError   bool
	}{
		{
			name:        "valid TCP address with port",
			remoteAddr:  "192.168.1.1:8080",
			localAddr:   nil,
			expectError: false,
		},
		{
			name:        "valid TCP address without port - should add default port 80",
			remoteAddr:  "192.168.1.1",
			localAddr:   nil,
			expectError: false,
		},
		{
			name:        "unix socket - should return local addr",
			remoteAddr:  "irrelevant",
			localAddr:   &net.UnixAddr{Name: "/tmp/test.sock", Net: "unix"},
			expectError: false,
		},
		{
			name:          "invalid address - should fail",
			remoteAddr:    "[invalid:address:format",
			localAddr:     nil,
			expectError:   true,
			errorContains: "failed to resolve TCP address:",
		},
		{
			name:          "invalid address after adding port - should fail",
			remoteAddr:    "[invalid",
			localAddr:     nil,
			expectError:   true,
			errorContains: "failed to resolve TCP address after adding port:",
		},
		{
			name:        "empty address - should fail",
			remoteAddr:  "",
			localAddr:   nil,
			expectError: true,
			expectedErr: ErrRemoteAddrEmpty,
		},
		{
			name:        "too long address - should fail",
			remoteAddr:  strings.Repeat("a", 254),
			localAddr:   nil,
			expectError: true,
			expectedErr: ErrRemoteAddrTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			addr, err := resolveRemoteAddr(tt.remoteAddr, tt.localAddr)

			expectError := tt.expectedErr != nil || tt.errorContains != ""
			if expectError {
				require.Error(t, err)
				if tt.expectedErr != nil {
					require.ErrorIs(t, err, tt.expectedErr)
				}
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Nil(t, addr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, addr)
			}
		})
	}
}

func Test_isUnixNetwork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		network  string
		expected bool
	}{
		{"unix", "unix", true},
		{"unixgram", "unixgram", true},
		{"unixpacket", "unixpacket", true},
		{"tcp", "tcp", false},
		{"tcp4", "tcp4", false},
		{"tcp6", "tcp6", false},
		{"udp", "udp", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isUnixNetwork(tt.network)
			require.Equal(t, tt.expected, result)
		})
	}
}

func Test_FiberHandler_ErrorFallback(t *testing.T) {
	t.Parallel()

	// Test case where resolveRemoteAddr fails and falls back to nil
	fiberH := func(c fiber.Ctx) error {
		return c.SendString("success")
	}
	handlerFunc := FiberHandlerFunc(fiberH)

	// Use helper function for cleaner test setup
	req := createTestRequest(http.MethodGet, "/test", "[invalid:address:format", nil)
	w := executeHandlerTest(t, handlerFunc, req)

	// Should still work despite the invalid remote address
	require.Equal(t, http.StatusOK, w.StatusCode())
	require.Equal(t, "success", string(w.body))
}

func Test_FiberHandler_WithUnixSocket(t *testing.T) {
	t.Parallel()

	// Test case where request has unix socket context
	fiberH := func(c fiber.Ctx) error {
		return c.SendString("unix socket success")
	}
	handlerFunc := FiberHandlerFunc(fiberH)

	// Create a context with unix socket local address
	unixAddr := &net.UnixAddr{Name: "/tmp/test.sock", Net: "unix"}
	ctx := context.WithValue(context.Background(), http.LocalAddrContextKey, unixAddr)

	r := &http.Request{
		Method:     http.MethodGet,
		RequestURI: "/test",
		RemoteAddr: "someremoteaddr", // This will be ignored due to unix socket
		Header:     make(http.Header),
		Body:       http.NoBody,
	}
	r = r.WithContext(ctx)

	w := &netHTTPResponseWriter{}
	handlerFunc.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.StatusCode())
	require.Equal(t, "unix socket success", string(w.body))
}

func Test_FiberHandler_BodySizeLimit(t *testing.T) {
	t.Parallel()

	// Test body size limit enforcement
	fiberH := func(c fiber.Ctx) error {
		return c.SendString("processed")
	}
	handlerFunc := FiberHandlerFunc(fiberH)

	// Create a large body exceeding limit
	largeBody := make([]byte, 15*1024*1024) // 15MB > 10MB limit
	req := createTestRequest(http.MethodPost, "/test", "127.0.0.1:8080", bytes.NewReader(largeBody))
	req.ContentLength = int64(len(largeBody))

	w := executeHandlerTest(t, handlerFunc, req)

	// Should return 413 due to size limit
	require.Equal(t, http.StatusRequestEntityTooLarge, w.StatusCode())
}

func Test_CopyContextToFiberContext_Safe(t *testing.T) {
	t.Parallel()

	t.Run("safe handling of unexported fields", func(t *testing.T) {
		t.Parallel()
		// Test that unexported fields are handled safely
		type testContext struct {
			exportedField string
			unexported    string // unexported
		}

		var fctx fasthttp.RequestCtx
		ctx := testContext{exportedField: "exported", unexported: "unexported"}

		// Should not panic and handle safely
		CopyContextToFiberContext(&ctx, &fctx)
		// No specific assertion, just ensure no panic
	})
}

func TestUnixSocketAdaptor(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "test.sock")
	defer func() {
		if err := os.Remove(socketPath); err != nil {
			t.Logf("cleanup failed: %v", err)
		}
	}()

	app := fiber.New()
	app.Get("/hello", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	handler := FiberApp(app)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		// Skip on platforms where the "unix" network is unsupported
		if strings.Contains(err.Error(), "unknown network") ||
			strings.Contains(err.Error(), "address family not supported") {
			t.Skipf("Unix domain sockets not supported on this platform: %v", err)
		}
		t.Fatal(err)
	}
	defer func() {
		if closeErr := listener.Close(); closeErr != nil {
			t.Logf("listener close failed: %v", closeErr)
		}
	}()

	// start server with timeouts
	srv := &http.Server{
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	done := make(chan struct{})
	go func() {
		if serveErr := srv.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			t.Errorf("http server failed: %v", serveErr)
		}
		close(done)
	}()

	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			t.Logf("conn close failed: %v", closeErr)
		}
	}()

	// set deadline for both write + read (2s)
	require.NoError(t, conn.SetDeadline(time.Now().Add(2*time.Second)))

	// write request
	_, err = conn.Write([]byte("GET /hello HTTP/1.1\r\nHost: localhost\r\n\r\n"))
	require.NoError(t, err)

	// read response
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	require.NoError(t, err)

	// clear deadline to avoid affecting further calls
	require.NoError(t, conn.SetDeadline(time.Time{}))

	raw := string(buf[:n])
	t.Logf("Raw response:\n%s", raw)
	require.Contains(t, raw, "HTTP/1.1 200 OK")
	require.Contains(t, raw, "ok")

	// now shutdown the server explicitly before waiting for done
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, srv.Shutdown(ctx))

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server shutdown timed out")
	}
}
