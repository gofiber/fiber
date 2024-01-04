//nolint:bodyclose, contextcheck, revive // Much easier to just ignore memory leaks in tests
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
	"testing"

	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
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
	utils.AssertEqual(t, nil, err)

	type contextKeyType string
	expectedContextKey := contextKeyType("contextKey")
	expectedContextValue := "contextValue"

	callsCount := 0
	nethttpH := func(w http.ResponseWriter, r *http.Request) {
		callsCount++
		utils.AssertEqual(t, expectedMethod, r.Method, "Method")
		utils.AssertEqual(t, expectedProto, r.Proto, "Proto")
		utils.AssertEqual(t, expectedProtoMajor, r.ProtoMajor, "ProtoMajor")
		utils.AssertEqual(t, expectedProtoMinor, r.ProtoMinor, "ProtoMinor")
		utils.AssertEqual(t, expectedRequestURI, r.RequestURI, "RequestURI")
		utils.AssertEqual(t, expectedContentLength, int(r.ContentLength), "ContentLength")
		utils.AssertEqual(t, 0, len(r.TransferEncoding), "TransferEncoding")
		utils.AssertEqual(t, expectedHost, r.Host, "Host")
		utils.AssertEqual(t, expectedRemoteAddr, r.RemoteAddr, "RemoteAddr")

		body, err := io.ReadAll(r.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, expectedBody, string(body), "Body")
		utils.AssertEqual(t, expectedURL, r.URL, "URL")
		utils.AssertEqual(t, expectedContextValue, r.Context().Value(expectedContextKey), "Context")

		for k, expectedV := range expectedHeader {
			v := r.Header.Get(k)
			utils.AssertEqual(t, expectedV, v, "Header")
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
	req.BodyWriter().Write([]byte(expectedBody)) //nolint:errcheck, gosec // not needed
	for k, v := range expectedHeader {
		req.Header.Set(k, v)
	}

	remoteAddr, err := net.ResolveTCPAddr("tcp", expectedRemoteAddr)
	utils.AssertEqual(t, nil, err)

	fctx.Init(&req, remoteAddr, nil)
	app := fiber.New()
	ctx := app.AcquireCtx(&fctx)
	defer app.ReleaseCtx(ctx)

	err = fiberH(ctx)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 1, callsCount, "callsCount")

	resp := &fctx.Response
	utils.AssertEqual(t, http.StatusBadRequest, resp.StatusCode(), "StatusCode")
	utils.AssertEqual(t, "value1", string(resp.Header.Peek("Header1")), "Header1")
	utils.AssertEqual(t, "value2", string(resp.Header.Peek("Header2")), "Header2")

	expectedResponseBody := fmt.Sprintf("request body is %q", expectedBody)
	utils.AssertEqual(t, expectedResponseBody, string(resp.Body()), "Body")
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
	app.Post("/", func(c *fiber.Ctx) error {
		value := c.Context().Value(TestContextKey)
		val, ok := value.(string)
		if !ok {
			t.Error("unexpected error on type-assertion")
		}
		if value != nil {
			c.Set("context_okay", val)
		}
		value = c.Context().Value(TestContextSecondKey)
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
		utils.AssertEqual(t, nil, err)

		resp, err := app.Test(req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, tt.statusCode, resp.StatusCode, "StatusCode")
	}

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodPost, "/", nil)
	req.Host = expectedHost
	utils.AssertEqual(t, nil, err)

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, resp.Header.Get("context_okay"), "okay")
	utils.AssertEqual(t, resp.Header.Get("context_second_okay"), "okay")
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
	utils.AssertEqual(t, nil, err)

	callsCount := 0
	fiberH := func(c *fiber.Ctx) error {
		callsCount++
		utils.AssertEqual(t, expectedMethod, c.Method(), "Method")
		utils.AssertEqual(t, expectedRequestURI, string(c.Context().RequestURI()), "RequestURI")
		utils.AssertEqual(t, expectedContentLength, c.Context().Request.Header.ContentLength(), "ContentLength")
		utils.AssertEqual(t, expectedHost, c.Hostname(), "Host")
		utils.AssertEqual(t, expectedHost, string(c.Request().Header.Host()), "Host")
		utils.AssertEqual(t, "http://"+expectedHost, c.BaseURL(), "BaseURL")
		utils.AssertEqual(t, expectedRemoteAddr, c.Context().RemoteAddr().String(), "RemoteAddr")

		body := string(c.Body())
		utils.AssertEqual(t, expectedBody, body, "Body")
		utils.AssertEqual(t, expectedURL.String(), c.OriginalURL(), "URL")

		for k, expectedV := range expectedHeader {
			v := c.Get(k)
			utils.AssertEqual(t, expectedV, v, "Header")
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
	r.Body = &netHTTPBody{[]byte(expectedBody)}
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

	utils.AssertEqual(t, http.StatusBadRequest, w.StatusCode(), "StatusCode")
	utils.AssertEqual(t, "value1", w.Header().Get("Header1"), "Header1")
	utils.AssertEqual(t, "value2", w.Header().Get("Header2"), "Header2")

	expectedResponseBody := fmt.Sprintf("request body is %q", expectedBody)
	utils.AssertEqual(t, expectedResponseBody, string(w.body), "Body")
}

func setFiberContextValueMiddleware(next fiber.Handler, key, value interface{}) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals(key, value)
		return next(c)
	}
}

func Test_FiberHandler_RequestNilBody(t *testing.T) {
	expectedMethod := fiber.MethodGet
	expectedRequestURI := "/foo/bar"
	expectedContentLength := 0

	callsCount := 0
	fiberH := func(c *fiber.Ctx) error {
		callsCount++
		utils.AssertEqual(t, expectedMethod, c.Method(), "Method")
		utils.AssertEqual(t, expectedRequestURI, string(c.Context().RequestURI()), "RequestURI")
		utils.AssertEqual(t, expectedContentLength, c.Context().Request.Header.ContentLength(), "ContentLength")

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
	utils.AssertEqual(t, expectedResponseBody, string(w.body), "Body")
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
	statusCode int
	h          http.Header
	body       []byte
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

	app.Get("/test", func(c *fiber.Ctx) error {
		httpReq, err := ConvertRequest(c, false)
		if err != nil {
			return err
		}

		return c.SendString("Request URL: " + httpReq.URL.String())
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test?hello=world&another=test", http.NoBody))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, http.StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "Request URL: /test?hello=world&another=test", string(body))
}

// Benchmark for FiberHandlerFunc
func Benchmark_FiberHandlerFunc_1MB(b *testing.B) {
	fiberH := func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}
	handlerFunc := FiberHandlerFunc(fiberH)

	// Create body content
	bodyContent := make([]byte, 1*1024*1024)
	bodyBuffer := bytes.NewBuffer(bodyContent)

	r := http.Request{
		Method: http.MethodPost,
		Body:   http.NoBody,
	}

	// Replace the empty Body with our buffer
	r.Body = io.NopCloser(bodyBuffer)
	defer r.Body.Close() //nolint:errcheck // not needed

	// Create recorder
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handlerFunc.ServeHTTP(w, &r)
	}
}

func Benchmark_FiberHandlerFunc_10MB(b *testing.B) {
	fiberH := func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}
	handlerFunc := FiberHandlerFunc(fiberH)

	// Create body content
	bodyContent := make([]byte, 10*1024*1024)
	bodyBuffer := bytes.NewBuffer(bodyContent)

	r := http.Request{
		Method: http.MethodPost,
		Body:   http.NoBody,
	}

	// Replace the empty Body with our buffer
	r.Body = io.NopCloser(bodyBuffer)
	defer r.Body.Close() //nolint:errcheck // not needed

	// Create recorder
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handlerFunc.ServeHTTP(w, &r)
	}
}

func Benchmark_FiberHandlerFunc_50MB(b *testing.B) {
	fiberH := func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}
	handlerFunc := FiberHandlerFunc(fiberH)

	// Create body content
	bodyContent := make([]byte, 50*1024*1024)
	bodyBuffer := bytes.NewBuffer(bodyContent)

	r := http.Request{
		Method: http.MethodPost,
		Body:   http.NoBody,
	}

	// Replace the empty Body with our buffer
	r.Body = io.NopCloser(bodyBuffer)
	defer r.Body.Close() //nolint:errcheck // not needed

	// Create recorder
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handlerFunc.ServeHTTP(w, &r)
	}
}
