//nolint:contextcheck,revive // Much easier to just ignore memory leaks in tests
package adaptor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_HTTPHandler(t *testing.T) {
	expectedMethod := fiber.MethodPost
	expectedProto := "HTTP/1.1"
	expectedProtoMajor := 1
	expectedProtoMinor := 1
	expectedRequestURI := "/foo/bar?baz=123"
	expectedBody := "body 123 foo bar baz"
	expectedContentLength := len(expectedBody)
	expectedHost := "foobar.com"
	expectedRemoteAddr := "1.2.3.4:6789"
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

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
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

type contextKey string

func (c contextKey) String() string {
	return "test-" + string(c)
}

var (
	TestContextKey       = contextKey("TestContextKey")
	TestContextSecondKey = contextKey("TestContextSecondKey")
)

func Test_HTTPMiddleware(t *testing.T) {
	const expectedHost = "foobar.com"
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
		req, err := http.NewRequestWithContext(context.Background(), tt.method, tt.url, nil)
		req.Host = expectedHost
		require.NoError(t, err)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, tt.statusCode, resp.StatusCode, "StatusCode")
	}

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodPost, "/", nil)
	req.Host = expectedHost
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, "okay", resp.Header.Get("context_okay"))
	require.Equal(t, "okay", resp.Header.Get("context_second_okay"))
}

func Test_HTTPMiddlewareWithCookies(t *testing.T) {
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
		var cookies []string = strings.Split(c.Get(cookieHeader), "; ")
		for _, cookie := range cookies {
			c.Set(setCookieHeader, cookie)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	// Test case for POST request with cookies
	t.Run("POST request with cookies", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodPost, "/", nil)
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
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/", nil)
		require.NoError(t, err)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})

	// New test case for request without cookies
	t.Run("POST request without cookies", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodPost, "/", nil)
		require.NoError(t, err)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Empty(t, resp.Cookies())
	})
}

func Test_FiberHandler(t *testing.T) {
	testFiberToHandlerFunc(t, false)
}

func Test_FiberApp(t *testing.T) {
	testFiberToHandlerFunc(t, false, fiber.New())
}

func Test_FiberHandlerDefaultPort(t *testing.T) {
	testFiberToHandlerFunc(t, true)
}

func Test_FiberAppDefaultPort(t *testing.T) {
	testFiberToHandlerFunc(t, true, fiber.New())
}

func testFiberToHandlerFunc(t *testing.T, checkDefaultPort bool, app ...*fiber.App) {
	t.Helper()

	expectedMethod := fiber.MethodPost
	expectedRequestURI := "/foo/bar?baz=123"
	expectedBody := "body 123 foo bar baz"
	expectedContentLength := len(expectedBody)
	expectedHost := "foobar.com"
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
		_, err := c.Write([]byte(fmt.Sprintf("request body is %q", body)))
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
	expectedMethod := fiber.MethodGet
	expectedRequestURI := "/foo/bar"
	expectedContentLength := 0

	callsCount := 0
	fiberH := func(c fiber.Ctx) error {
		callsCount++
		require.Equal(t, expectedMethod, c.Method(), "Method")
		require.Equal(t, expectedRequestURI, string(c.RequestCtx().RequestURI()), "RequestURI")
		require.Equal(t, expectedContentLength, c.RequestCtx().Request.Header.ContentLength(), "ContentLength")

		_, err := c.Write([]byte("request body is nil"))
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

func Test_ConvertRequest(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/test", func(c fiber.Ctx) error {
		httpReq, err := ConvertRequest(c, false)
		if err != nil {
			return err
		}

		return c.SendString("Request URL: " + httpReq.URL.String())
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test?hello=world&another=test", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, http.StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Request URL: /test?hello=world&another=test", string(body))
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
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
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
	b.ResetTimer()

	fiberHandler := HTTPHandler(handler)

	for i := 0; i < b.N; i++ {
		ctx.Request().Reset()
		ctx.Response().Reset()
		ctx.Request().SetRequestURI("/test")
		ctx.Request().Header.SetMethod("GET")

		err = fiberHandler(ctx)
	}

	require.NoError(b, err)
}
