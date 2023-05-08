//nolint:bodyclose, contextcheck, revive // Much easier to just ignore memory leaks in tests
package adaptor

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/gofiber/fiber/v2"
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
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedContextKey := "contextKey"
	expectedContextValue := "contextValue"

	callsCount := 0
	nethttpH := func(w http.ResponseWriter, r *http.Request) {
		callsCount++
		if r.Method != expectedMethod {
			t.Fatalf("unexpected method %q. Expecting %q", r.Method, expectedMethod)
		}
		if r.Proto != expectedProto {
			t.Fatalf("unexpected proto %q. Expecting %q", r.Proto, expectedProto)
		}
		if r.ProtoMajor != expectedProtoMajor {
			t.Fatalf("unexpected protoMajor %d. Expecting %d", r.ProtoMajor, expectedProtoMajor)
		}
		if r.ProtoMinor != expectedProtoMinor {
			t.Fatalf("unexpected protoMinor %d. Expecting %d", r.ProtoMinor, expectedProtoMinor)
		}
		if r.RequestURI != expectedRequestURI {
			t.Fatalf("unexpected requestURI %q. Expecting %q", r.RequestURI, expectedRequestURI)
		}
		if r.ContentLength != int64(expectedContentLength) {
			t.Fatalf("unexpected contentLength %d. Expecting %d", r.ContentLength, expectedContentLength)
		}
		if len(r.TransferEncoding) != 0 {
			t.Fatalf("unexpected transferEncoding %q. Expecting []", r.TransferEncoding)
		}
		if r.Host != expectedHost {
			t.Fatalf("unexpected host %q. Expecting %q", r.Host, expectedHost)
		}
		if r.RemoteAddr != expectedRemoteAddr {
			t.Fatalf("unexpected remoteAddr %q. Expecting %q", r.RemoteAddr, expectedRemoteAddr)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("unexpected error when reading request body: %s", err)
		}
		if string(body) != expectedBody {
			t.Fatalf("unexpected body %q. Expecting %q", body, expectedBody)
		}
		if !reflect.DeepEqual(r.URL, expectedURL) {
			t.Fatalf("unexpected URL: %#v. Expecting %#v", r.URL, expectedURL)
		}
		if r.Context().Value(expectedContextKey) != expectedContextValue {
			t.Fatalf("unexpected context value for key %q. Expecting %q", expectedContextKey, expectedContextValue)
		}

		for k, expectedV := range expectedHeader {
			v := r.Header.Get(k)
			if v != expectedV {
				t.Fatalf("unexpected header value %q for key %q. Expecting %q", v, k, expectedV)
			}
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
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	fctx.Init(&req, remoteAddr, nil)
	app := fiber.New()
	ctx := app.AcquireCtx(&fctx)
	defer app.ReleaseCtx(ctx)

	err = fiberH(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if callsCount != 1 {
		t.Fatalf("unexpected callsCount: %d. Expecting 1", callsCount)
	}

	resp := &fctx.Response
	if resp.StatusCode() != fiber.StatusBadRequest {
		t.Fatalf("unexpected statusCode: %d. Expecting %d", resp.StatusCode(), fiber.StatusBadRequest)
	}
	if string(resp.Header.Peek("Header1")) != "value1" {
		t.Fatalf("unexpected header value: %q. Expecting %q", resp.Header.Peek("Header1"), "value1")
	}
	if string(resp.Header.Peek("Header2")) != "value2" {
		t.Fatalf("unexpected header value: %q. Expecting %q", resp.Header.Peek("Header2"), "value2")
	}
	expectedResponseBody := fmt.Sprintf("request body is %q", expectedBody)
	if string(resp.Body()) != expectedResponseBody {
		t.Fatalf("unexpected response body %q. Expecting %q", resp.Body(), expectedResponseBody)
	}
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
		if err != nil {
			t.Fatalf(`%s: %s`, t.Name(), err)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf(`%s: %s`, t.Name(), err)
		}
		if resp.StatusCode != tt.statusCode {
			t.Fatalf(`%s: StatusCode: got %v - expected %v`, t.Name(), resp.StatusCode, tt.statusCode)
		}
	}

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodPost, "/", nil)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.Header.Get("context_okay") != "okay" {
		t.Fatalf(`%s: Header context_okay: got %v - expected %v`, t.Name(), resp.Header.Get("context_okay"), "okay")
	}
	if resp.Header.Get("context_second_okay") != "okay" {
		t.Fatalf(`%s: Header context_second_okay: got %v - expected %v`, t.Name(), resp.Header.Get("context_second_okay"), "okay")
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	callsCount := 0
	fiberH := func(c *fiber.Ctx) error {
		callsCount++
		if c.Method() != expectedMethod {
			t.Fatalf("unexpected method %q. Expecting %q", c.Method(), expectedMethod)
		}
		if string(c.Context().RequestURI()) != expectedRequestURI {
			t.Fatalf("unexpected requestURI %q. Expecting %q", string(c.Context().RequestURI()), expectedRequestURI)
		}
		contentLength := c.Context().Request.Header.ContentLength()
		if contentLength != expectedContentLength {
			t.Fatalf("unexpected contentLength %d. Expecting %d", contentLength, expectedContentLength)
		}
		if c.Hostname() != expectedHost {
			t.Fatalf("unexpected host %q. Expecting %q", c.Hostname(), expectedHost)
		}
		remoteAddr := c.Context().RemoteAddr().String()
		if remoteAddr != expectedRemoteAddr {
			t.Fatalf("unexpected remoteAddr %q. Expecting %q", remoteAddr, expectedRemoteAddr)
		}
		body := string(c.Body())
		if body != expectedBody {
			t.Fatalf("unexpected body %q. Expecting %q", body, expectedBody)
		}
		if c.OriginalURL() != expectedURL.String() {
			t.Fatalf("unexpected URL: %#v. Expecting %#v", c.OriginalURL(), expectedURL)
		}

		for k, expectedV := range expectedHeader {
			v := c.Get(k)
			if v != expectedV {
				t.Fatalf("unexpected header value %q for key %q. Expecting %q", v, k, expectedV)
			}
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

	if w.StatusCode() != http.StatusBadRequest {
		t.Fatalf("unexpected statusCode: %d. Expecting %d", w.StatusCode(), http.StatusBadRequest)
	}
	if w.Header().Get("Header1") != "value1" {
		t.Fatalf("unexpected header value: %q. Expecting %q", w.Header().Get("Header1"), "value1")
	}
	if w.Header().Get("Header2") != "value2" {
		t.Fatalf("unexpected header value: %q. Expecting %q", w.Header().Get("Header2"), "value2")
	}
	expectedResponseBody := fmt.Sprintf("request body is %q", expectedBody)
	if string(w.body) != expectedResponseBody {
		t.Fatalf("unexpected response body %q. Expecting %q", string(w.body), expectedResponseBody)
	}
}

func setFiberContextValueMiddleware(next fiber.Handler, key string, value interface{}) fiber.Handler {
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
		if c.Method() != expectedMethod {
			t.Fatalf("unexpected method %q. Expecting %q", c.Method(), expectedMethod)
		}
		if string(c.Request().RequestURI()) != expectedRequestURI {
			t.Fatalf("unexpected requestURI %q. Expecting %q", string(c.Request().RequestURI()), expectedRequestURI)
		}
		contentLength := c.Request().Header.ContentLength()
		if contentLength != expectedContentLength {
			t.Fatalf("unexpected contentLength %d. Expecting %d", contentLength, expectedContentLength)
		}

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
	if string(w.body) != expectedResponseBody {
		t.Fatalf("unexpected response body %q. Expecting %q", string(w.body), expectedResponseBody)
	}
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
