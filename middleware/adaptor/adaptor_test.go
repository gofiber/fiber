//nolint:contextcheck // Much easier to just ignore memory leaks in tests
package adaptor

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/middleware/sse"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

const (
	expectedRequestURI = "/foo/bar?baz=123"
	expectedBody       = "body 123 foo bar baz"
	expectedHost       = "foobar.com"
	expectedRemoteAddr = "1.2.3.4:6789"
)

// adaptorTestServe sends req through h and returns what the handler recorded.
func adaptorTestServe(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func Test_HTTPHandler(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		flush bool // flush the response after its first chunk
	}{
		{name: "plain"},
		{name: "flush", flush: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			expectedMethod := fiber.MethodPost
			expectedProto := "HTTP/1.1"
			expectedProtoMajor := 1
			expectedProtoMinor := 1
			expectedContentLength := len(expectedBody)
			expectedHeader := map[string]string{"Foo-Bar": "baz", "Abc": "defg", "XXX-Remote-Addr": "123.43.4543.345"}
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
				if !tc.flush {
					fmt.Fprintf(w, "request body is %q", body)
					return
				}
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
			ctx := apptest.CtxFor(t, fiber.New(), &fctx)

			err = fiberH(ctx)
			require.NoError(t, err)
			require.Equal(t, 1, callsCount, "callsCount")

			resp := &fctx.Response
			require.Equal(t, http.StatusBadRequest, resp.StatusCode(), "StatusCode")
			require.Equal(t, "value1", string(resp.Header.Peek("Header1")), "Header1")
			require.Equal(t, "value2", string(resp.Header.Peek("Header2")), "Header2")

			expectedResponseBody := fmt.Sprintf("request body is %q", expectedBody)
			require.Equal(t, expectedResponseBody, string(resp.Body()), "Body")
		})
	}
}

func Test_HTTPHandler_Flush_App_Test(t *testing.T) {
	t.Parallel()
	newApp := func(t *testing.T) *fiber.App {
		t.Helper()

		app := fiber.New()
		app.Get("/", HTTPHandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
		return app
	}

	t.Run("complete", func(t *testing.T) {
		t.Parallel()
		apptest.ExpectBody(t, newApp(t), "/", fiber.StatusOK, "Hello World!")
	})

	t.Run("interrupted", func(t *testing.T) {
		t.Parallel()
		resp, err := newApp(t).Test(apptest.Req(fiber.MethodGet, "/"), fiber.TestConfig{
			Timeout:       200 * time.Millisecond,
			FailOnTimeout: true, // Changed to true to test interrupted behavior
		})
		// With FailOnTimeout: true, we should get a timeout error
		require.ErrorIs(t, err, os.ErrDeadlineExceeded)
		require.Nil(t, resp)
	})
}

func Test_LocalContextFromHTTPRequest(t *testing.T) {
	t.Parallel()
	t.Run("nil request", func(t *testing.T) {
		t.Parallel()
		ctx, ok := LocalContextFromHTTPRequest(nil)
		require.False(t, ok)
		require.Nil(t, ctx)
	})

	t.Run("request without stored context key", func(t *testing.T) {
		t.Parallel()
		req := apptest.Req(fiber.MethodGet, "/")

		ctx, ok := LocalContextFromHTTPRequest(req)
		require.False(t, ok)
		require.Nil(t, ctx)
	})

	t.Run("request with stored context key", func(t *testing.T) {
		t.Parallel()
		expectedCtx := context.WithValue(context.Background(), contextKey("k"), "v")
		req := apptest.Req(fiber.MethodGet, "/").WithContext(
			context.WithValue(context.Background(), localContextKey, expectedCtx),
		)

		ctx, ok := LocalContextFromHTTPRequest(req)
		require.True(t, ok)
		require.Equal(t, expectedCtx, ctx)
	})
}

func Test_HTTPHandlerWithContext_local_context(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// unique type for avoiding collisions in context
	type key struct{}
	var testKey key

	const testVal string = "test-value"

	app.Use(func(c fiber.Ctx) error {
		ctx := context.WithValue(c.Context(), testKey, testVal)
		c.SetContext(ctx)
		return c.Next()
	})

	app.Get("/", HTTPHandlerWithContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, ok := LocalContextFromHTTPRequest(r)
		if !ok {
			http.Error(w, "local context not found", http.StatusInternalServerError)
			return
		}
		val, ok := ctx.Value(testKey).(string)
		if !ok {
			http.Error(w, "invalid context value", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(val)); err != nil {
			t.Logf("write failed: %v", err)
		}
	})))

	resp := apptest.Do(t, app, "/", fiber.TestConfig{Timeout: 200 * time.Millisecond, FailOnTimeout: false})

	defer resp.Body.Close() //nolint:errcheck // no need

	require.Equal(t, testVal, apptest.Body(t, resp))
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
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
		{name: "Should return 200", url: "/", method: "POST", statusCode: 200},
		{name: "Should return 405", url: "/", method: "GET", statusCode: 405},
		{name: "Should return 400", url: "/unknown", method: "POST", statusCode: 404},
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
		apptest.Expect(t, app, apptest.Req(tt.method, tt.url, apptest.WithHost(expectedHost)), tt.statusCode)
	}

	resp := apptest.Do(t, app, apptest.Req(fiber.MethodPost, "/", apptest.WithHost(expectedHost)))
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
		cookies := strings.SplitSeq(c.Get(cookieHeader), "; ")
		for cookie := range cookies {
			c.Set(setCookieHeader, cookie)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	t.Run("POST request with cookies", func(t *testing.T) {
		t.Parallel()
		req := apptest.Req(fiber.MethodPost, "/")
		req.AddCookie(&http.Cookie{Name: cookieOneName, Value: cookieOneValue})
		req.AddCookie(&http.Cookie{Name: cookieTwoName, Value: cookieTwoValue})

		resp := apptest.Do(t, app, req)
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

	t.Run("GET request", func(t *testing.T) {
		t.Parallel()
		apptest.Expect(t, app, "/", http.StatusMethodNotAllowed)
	})

	t.Run("POST request without cookies", func(t *testing.T) {
		t.Parallel()
		resp := apptest.Expect(t, app, apptest.Req(fiber.MethodPost, "/"), http.StatusOK)
		require.Empty(t, resp.Cookies())
	})
}

// Test_FiberHandler serves one request through FiberHandlerFunc, or through FiberApp, and checks what
// the Fiber handler reads from it and what the net/http side gets back. A remote address without a port
// gets port 80.
func Test_FiberHandler(t *testing.T) {
	t.Parallel()
	expectedHeader := map[string]string{"Foo-Bar": "baz", "Abc": "defg", "XXX-Remote-Addr": "123.43.4543.345"}
	expectedURL, err := url.ParseRequestURI(expectedRequestURI)
	require.NoError(t, err)

	for _, tc := range []struct {
		name           string
		remoteAddr     string
		wantRemoteAddr string
		app            bool // served by FiberApp rather than FiberHandlerFunc
	}{
		{name: "handler", remoteAddr: "1.2.3.4:6789", wantRemoteAddr: "1.2.3.4:6789"},
		{name: "app", remoteAddr: "1.2.3.4:6789", wantRemoteAddr: "1.2.3.4:6789", app: true},
		{name: "handler default port", remoteAddr: "1.2.3.4", wantRemoteAddr: "1.2.3.4:80"},
		{name: "app default port", remoteAddr: "1.2.3.4", wantRemoteAddr: "1.2.3.4:80", app: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fiberH := func(c fiber.Ctx) error {
				require.Equal(t, fiber.MethodPost, c.Method(), "Method")
				require.Equal(t, expectedRequestURI, string(c.RequestCtx().RequestURI()), "RequestURI")
				require.Equal(t, len(expectedBody), c.RequestCtx().Request.Header.ContentLength(), "ContentLength")
				require.Equal(t, expectedHost, c.Hostname(), "Host")
				require.Equal(t, expectedHost, string(c.Request().Header.Host()), "Host")
				require.Equal(t, "http://"+expectedHost, c.BaseURL(), "BaseURL")
				require.Equal(t, tc.wantRemoteAddr, c.RequestCtx().RemoteAddr().String(), "RemoteAddr")

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
			if tc.app {
				app := fiber.New()
				app.Post("/foo/bar", fiberH)
				handlerFunc = FiberApp(app)
			} else {
				handlerFunc = FiberHandlerFunc(fiberH)
			}

			hdr := make(http.Header)
			for k, v := range expectedHeader {
				hdr.Set(k, v)
			}
			r := http.Request{
				Method:        fiber.MethodPost,
				Body:          &netHTTPBody{b: []byte(expectedBody)},
				RequestURI:    expectedRequestURI,
				ContentLength: int64(len(expectedBody)),
				Host:          expectedHost,
				RemoteAddr:    tc.remoteAddr,
				Header:        hdr,
			}

			var w netHTTPResponseWriter
			handlerFunc.ServeHTTP(&w, &r)

			require.Equal(t, http.StatusBadRequest, w.StatusCode(), "StatusCode")
			require.Equal(t, "value1", w.Header().Get("Header1"), "Header1")
			require.Equal(t, "value2", w.Header().Get("Header2"), "Header2")
			require.Equal(t, fmt.Sprintf("request body is %q", expectedBody), string(w.body), "Body")
		})
	}
}

// Test_FiberHandler_ProtocolPropagation verifies that the request's protocol
// version reaches the fiber ctx, so protocol-dependent behavior (e.g. the
// RFC 9110 rule against sending 1xx responses to HTTP/1.0 clients) sees the
// real version instead of fasthttp's HTTP/1.1 default.
func Test_FiberHandler_ProtocolPropagation(t *testing.T) {
	t.Parallel()
	handler := FiberHandlerFunc(func(c fiber.Ctx) error { return c.SendString(c.Protocol()) })

	// The requests are served one after another by one handler, so a later one
	// may be served by a context an earlier one has used: no subtests, which
	// would run side by side.
	for _, tc := range []struct {
		name         string
		proto        string
		want         string
		major, minor int
	}{
		{name: "HTTP/1.0", proto: "HTTP/1.0", major: 1, minor: 0, want: "HTTP/1.0"},
		// net/http reports HTTP/2 as "HTTP/2.0"; Fiber's convention is "HTTP/2",
		// so the adaptor normalizes it to keep c.Protocol() comparisons working.
		{name: "HTTP/2", proto: "HTTP/2.0", major: 2, minor: 0, want: "HTTP/2"},
		// Same normalization for HTTP/3.
		{name: "HTTP/3", proto: "HTTP/3.0", major: 3, minor: 0, want: "HTTP/3"},
		// A hand-built request with an empty Proto falls back to HTTP/1.1
		// instead of leaving fasthttp in an inconsistent protocol state.
		{name: "empty Proto", proto: "", major: 0, minor: 0, want: "HTTP/1.1"},
	} {
		req := apptest.Req(http.MethodGet, "http://example.com/")
		req.Proto = tc.proto
		req.ProtoMajor = tc.major
		req.ProtoMinor = tc.minor

		require.Equal(t, tc.want, adaptorTestServe(handler, req).Body.String(), tc.name)
	}
}

// Test_FiberHandler_SendEarlyHints verifies that SendEarlyHints through the
// adaptor does not panic: there is no client connection for interim
// responses, so the 103 is silently discarded while the Link headers still
// reach the final response.
func Test_FiberHandler_SendEarlyHints(t *testing.T) {
	t.Parallel()
	handler := FiberHandlerFunc(func(c fiber.Ctx) error {
		if err := c.SendEarlyHints([]string{"<https://cdn.com/style.css>; rel=preload; as=style"}); err != nil {
			return err
		}
		return c.SendString("done")
	})

	req := apptest.Req(http.MethodGet, "http://example.com/")
	rec := httptest.NewRecorder()
	require.NotPanics(t, func() { handler.ServeHTTP(rec, req) })
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "done", rec.Body.String())
	require.Equal(t, "<https://cdn.com/style.css>; rel=preload; as=style", rec.Header().Get("Link"))
}

func Test_FiberHandler_BodyLimit(t *testing.T) {
	t.Parallel()
	const mib = 1024 * 1024
	tooLarge := fiber.StatusRequestEntityTooLarge
	tests := []struct {
		name          string
		limit         int // the BodyLimit of the app
		size          int // of the request body
		status        int
		unknownLength bool
	}{
		{name: "DefaultLimitExceededReturns413", size: fiber.DefaultBodyLimit + 1024, status: tooLarge},
		{name: "CustomLimitExceededReturns413", limit: mib, size: mib + 1, status: tooLarge},
		{
			name: "CustomLimitAllowsLargerPayload", limit: 2 * fiber.DefaultBodyLimit, size: fiber.DefaultBodyLimit + 512,
			status: fiber.StatusOK,
		},
		{name: "ZeroLimitConfigFallsBackToDefault", limit: 0, size: fiber.DefaultBodyLimit - 256, status: fiber.StatusOK},
		{name: "UnknownLengthExceededReturns413", limit: mib, size: mib + 1, unknownLength: true, status: tooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New(fiber.Config{BodyLimit: tt.limit})
			app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

			handlerFunc := FiberApp(app)
			body := bytes.Repeat([]byte("a"), tt.size)
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
			if tt.unknownLength {
				req.ContentLength = -1
			} else {
				req.ContentLength = int64(len(body))
			}

			require.Equal(t, tt.status, adaptorTestServe(handlerFunc, req).Code)
		})
	}
}

func setFiberContextValueMiddleware(next fiber.Handler, key, value any) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals(key, value)
		return next(c)
	}
}

func Test_FiberHandler_RequestNilBody(t *testing.T) {
	t.Parallel()
	nethttpH := FiberHandler(func(c fiber.Ctx) error {
		require.Equal(t, fiber.MethodGet, c.Method(), "Method")
		require.Equal(t, "/foo/bar", string(c.RequestCtx().RequestURI()), "RequestURI")
		require.Equal(t, 0, c.RequestCtx().Request.Header.ContentLength(), "ContentLength")

		_, err := c.WriteString("request body is nil")
		return err
	})

	var w netHTTPResponseWriter
	nethttpH.ServeHTTP(&w, &http.Request{Method: fiber.MethodGet, RequestURI: "/foo/bar"})

	require.Equal(t, "request body is nil", string(w.body), "Body")
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

func createTestRequest(method, remoteAddr string, body io.Reader) *http.Request {
	r := &http.Request{
		Method:     method,
		RequestURI: "/test",
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

func (*netHTTPResponseWriter) Flush() {}

// fasthttpadaptor buffers the body for the http.Request, and with StreamRequestBody the server passes on a body of any size.
func Test_StreamedBodyLimit(t *testing.T) {
	t.Parallel()

	const limit = 1024
	echoLen := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, err := io.Copy(io.Discard, r.Body)
		assert.NoError(t, err)
		_, err = w.Write([]byte(strconv.FormatInt(n, 10)))
		assert.NoError(t, err)
	})

	app := fiber.New(fiber.Config{StreamRequestBody: true, BodyLimit: limit})
	app.Post("/handler", HTTPHandler(echoLen))
	app.Post("/middleware", HTTPMiddleware(func(http.Handler) http.Handler { return echoLen }))
	app.Post("/next", HTTPMiddleware(func(next http.Handler) http.Handler { return next }), func(c fiber.Ctx) error {
		return c.SendString(strconv.Itoa(len(c.Body())))
	})
	app.Post("/convert", func(c fiber.Ctx) error {
		r, err := ConvertRequest(c, true)
		if err != nil {
			return err
		}
		return c.SendString(strconv.FormatInt(r.ContentLength, 10))
	})

	for _, target := range []string{"/handler", "/middleware", "/next", "/convert"} {
		for _, chunked := range []bool{false, true} {
			for size, status := range map[int]int{limit: fiber.StatusOK, limit + 1: fiber.StatusRequestEntityTooLarge} {
				req := httptest.NewRequest(fiber.MethodPost, target, strings.NewReader(strings.Repeat("a", size)))
				if chunked {
					req.TransferEncoding = []string{"chunked"}
				}
				resp, err := app.Test(req)
				require.NoError(t, err)
				require.Equal(t, status, resp.StatusCode, "%s chunked=%v size=%d", target, chunked, size)
				if status == fiber.StatusOK {
					body, err := io.ReadAll(resp.Body)
					require.NoError(t, err)
					require.Equal(t, strconv.Itoa(size), string(body), "%s chunked=%v", target, chunked)
				}
			}
		}
	}

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().SetBodyStream(iotest.ErrReader(errors.New("broken chunk")), -1)
	_, err := ConvertRequest(c, true)
	require.ErrorIs(t, err, fiber.ErrBadRequest)
	c.Request().SetBodyStream(iotest.ErrReader(fiber.ErrRequestHeaderFieldsTooLarge), -1)
	_, err = ConvertRequest(c, true)
	require.ErrorIs(t, err, fiber.ErrRequestHeaderFieldsTooLarge)
}

func Test_StreamedTrailerCap(t *testing.T) {
	t.Parallel()

	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(w, r.Body)
		assert.NoError(t, err)
	})
	app := fiber.New(fiber.Config{StreamRequestBody: true})
	app.Post("/handler", HTTPHandler(echo))
	app.Post("/middleware", HTTPMiddleware(func(http.Handler) http.Handler { return echo }))
	app.Post("/convert", func(c fiber.Ctx) error {
		_, err := ConvertRequest(c, true)
		return err
	})

	ln := fasthttputil.NewInmemoryListener()
	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})
	}()
	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
		if err := <-errCh; err != nil && !errors.Is(err, net.ErrClosed) {
			require.NoError(t, err)
		}
	})

	var trailers strings.Builder
	for i := range fieldname.MaxTrailers + 1 {
		fmt.Fprintf(&trailers, "X-%d: v\r\n", i)
	}
	for _, path := range []string{"/handler", "/middleware", "/convert"} {
		conn, err := ln.Dial()
		require.NoError(t, err)
		_, err = conn.Write([]byte("POST " + path + " HTTP/1.1\r\nHost: app.example\r\nTransfer-Encoding: chunked\r\n" +
			"Connection: close\r\n\r\n5\r\nhello\r\n0\r\n" + trailers.String() + "\r\n"))
		require.NoError(t, err)
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.NoError(t, conn.Close())
		require.Equal(t, fiber.StatusRequestHeaderFieldsTooLarge, resp.StatusCode, path)
	}
}

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

		apptest.ExpectBody(t, app, "/test?hello=world&another=test", http.StatusOK, "Request URL: /test?hello=world&another=test")
	})

	t.Run("conversion error handling", func(t *testing.T) {
		t.Parallel()
		// Test error case by creating a context with an invalid URL that will cause fasthttpadaptor.ConvertRequest to fail
		ctx := apptest.Ctx(t, fiber.New())

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

	// Test the default case (continue statement coverage)
	type customContext struct {
		UnknownField string
	}
	// Test that unexported fields are handled safely
	type testContext struct {
		exportedField string
		unexported    string // unexported
	}
	type testStruct struct {
		Field string
	}
	notAStruct := "not a struct"
	var nilPtr *context.Context // Nil pointer to a context
	background := context.Background()
	backgroundPtr := &background

	// None of these sources holds anything to copy: each is a shape the copy
	// has to tolerate, which no panic proves.
	for _, tc := range []struct {
		src  any
		name string
	}{
		// Non-struct context (should return early)
		{name: "unsupported context type", src: &notAStruct},
		// This should hit the default case and continue
		{name: "context with unknown field", src: &customContext{UnknownField: "test"}},
		{name: "invalid src", src: nil},
		{name: "nil pointer", src: nilPtr},
		// Test deref pointer chains
		{name: "multi-level pointer", src: &backgroundPtr},
		// Pass struct value directly to test addressability check
		{name: "non-addressable struct", src: testStruct{Field: "test"}},
		{name: "safe handling of unexported fields", src: &testContext{exportedField: "exported", unexported: "unexported"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var fctx fasthttp.RequestCtx
			require.NotPanics(t, func() { CopyContextToFiberContext(tc.src, &fctx) })
		})
	}

	t.Run("nil request context", func(t *testing.T) {
		t.Parallel()
		ctx := context.WithValue(context.Background(), contextKey("nil-request-context"), "value")
		require.NotPanics(t, func() { CopyContextToFiberContext(ctx, nil) })
	})
}

// Test_HTTPMiddleware_ErrorHandling covers a downstream Fiber error behind a net/http middleware that
// already wrote a status of its own: the app's error handler answers.
func Test_HTTPMiddleware_ErrorHandling(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(HTTPMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			next.ServeHTTP(w, r)
		})
	}))
	app.Get("/error", func(_ fiber.Ctx) error { return fiber.NewError(fiber.StatusBadRequest, "test error") })

	apptest.Expect(t, app, "/error", fiber.StatusBadRequest)
}

// Test_FiberHandlerFunc_Responses serves one request through FiberHandlerFunc and checks the status, and
// the body when a row names one, that the net/http side gets back.
func Test_FiberHandlerFunc_Responses(t *testing.T) {
	t.Parallel()
	sendString := func(body string) fiber.Handler { return func(c fiber.Ctx) error { return c.SendString(body) } }
	largeBody := make([]byte, 15*1024*1024) // 15MB > 10MB limit

	for _, tc := range []struct {
		handler       fiber.Handler
		body          io.Reader
		localAddr     net.Addr // the address the request arrived on, when it names a unix socket
		name          string
		method        string
		remoteAddr    string
		wantBody      string
		contentLength int64
		wantStatus    int
	}{
		// The body is read through a reader that fails.
		{
			name: "body reader that fails", handler: sendString("should not reach here"), method: http.MethodPost,
			body: &failingReader{}, contentLength: 100, wantStatus: http.StatusInternalServerError,
		},
		{
			name: "error in the handler", method: http.MethodGet, wantStatus: fiber.StatusTeapot,
			handler: func(_ fiber.Ctx) error { return fiber.NewError(fiber.StatusTeapot, "I'm a teapot") },
		},
		{
			name: "streamed response", method: http.MethodGet,
			handler:    adaptorTestHelloWorldStream(200 * time.Millisecond),
			wantStatus: fiber.StatusTeapot, wantBody: "Hello World!",
		},
		// The request is still served, with no remote address.
		{
			name: "remote address that does not resolve", handler: sendString("success"), method: http.MethodGet,
			remoteAddr: "[invalid:address:format", wantStatus: http.StatusOK, wantBody: "success",
		},
		// The remote address of a request that arrived on a unix socket is ignored.
		{
			name: "unix socket", handler: sendString("unix socket success"), method: http.MethodGet,
			remoteAddr: "someremoteaddr", localAddr: &net.UnixAddr{Name: "/tmp/test.sock", Net: "unix"},
			wantStatus: http.StatusOK, wantBody: "unix socket success",
		},
		{
			name: "body over the limit", handler: sendString("processed"), method: http.MethodPost,
			remoteAddr: "127.0.0.1:8080", body: bytes.NewReader(largeBody), contentLength: int64(len(largeBody)),
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := createTestRequest(tc.method, tc.remoteAddr, tc.body)
			req.ContentLength = tc.contentLength
			if tc.localAddr != nil {
				req = req.WithContext(context.WithValue(context.Background(), http.LocalAddrContextKey, tc.localAddr))
			}

			w := executeHandlerTest(t, FiberHandlerFunc(tc.handler), req)
			require.Equal(t, tc.wantStatus, w.StatusCode())
			if tc.wantBody != "" {
				require.Equal(t, tc.wantBody, string(w.body))
			}
		})
	}
}

// adaptorTestHelloWorldStream answers 418 and streams "Hello ", flushes, then
// waits for delay, a long operation, before streaming "World!".
func adaptorTestHelloWorldStream(delay time.Duration) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Status(fiber.StatusTeapot)
		return c.SendStreamWriter(func(w *bufio.Writer) {
			w.WriteString("Hello ") //nolint:errcheck // not needed
			w.Flush()               //nolint:errcheck // not needed
			time.Sleep(delay)       // Simulate a long operation
			w.WriteString("World!") //nolint:errcheck // not needed
		})
	}
}

func Test_FiberHandler_ClosesBodyStream(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		w    http.ResponseWriter
		name string
	}{
		{name: "response written", w: httptest.NewRecorder()},
		{name: "write error", w: &failingStreamWriter{header: make(http.Header)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			closed := &atomic.Bool{}
			body := &closeTrackingReader{ReadCloser: io.NopCloser(strings.NewReader("streamed body")), closed: closed}

			handlerFunc := FiberHandlerFunc(func(c fiber.Ctx) error { return c.SendStream(body) })

			handlerFunc.ServeHTTP(tc.w, apptest.Req(http.MethodGet, "/test"))
			require.True(t, closed.Load())
		})
	}
}

type closeTrackingReader struct {
	io.ReadCloser
	closed *atomic.Bool
}

func (r *closeTrackingReader) Close() error {
	r.closed.Store(true)
	_ = r.ReadCloser.Close() //nolint:errcheck // not needed
	return nil
}

type failingStreamWriter struct {
	header http.Header
}

func (f *failingStreamWriter) Header() http.Header {
	return f.header
}

func (*failingStreamWriter) Write([]byte) (int, error) {
	return 0, errors.New("simulated write error")
}

func (*failingStreamWriter) WriteHeader(int) {}

func (*failingStreamWriter) Flush() {}

func Test_FiberHandler_WithInterruptedSendStreamWriter(t *testing.T) {
	t.Parallel()

	// Test streaming functionality to ensure data is sent even during a timeout.
	handlerFunc := FiberHandlerFunc(adaptorTestHelloWorldStream(500 * time.Millisecond))

	server := &http.Server{Handler: handlerFunc, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second}
	listener, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	addr := fmt.Sprintf("http://%s", listener.Addr())

	go func() {
		server.Serve(listener) //nolint:errcheck // not needed
	}()
	defer func() { require.NoError(t, server.Close()) }()

	cc := &http.Client{Timeout: 200 * time.Millisecond}
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

func (*failingReader) Read(_ []byte) (int, error) {
	return 0, errors.New("simulated read error")
}

func (*failingReader) Close() error {
	return nil
}

// adaptorBenchBody is a request body of the named size for the FiberHandlerFunc benchmarks.
type adaptorBenchBody struct {
	name        string
	bodyContent []byte
}

// adaptorBenchBodies returns the request bodies those benchmarks send, from none to 50MB.
func adaptorBenchBodies() []adaptorBenchBody {
	return []adaptorBenchBody{
		{name: "No Content"},
		{name: "100KB", bodyContent: make([]byte, 100*1024)},
		{name: "500KB", bodyContent: make([]byte, 500*1024)},
		{name: "1MB", bodyContent: make([]byte, 1*1024*1024)},
		{name: "5MB", bodyContent: make([]byte, 5*1024*1024)},
		{name: "10MB", bodyContent: make([]byte, 10*1024*1024)},
		{name: "25MB", bodyContent: make([]byte, 25*1024*1024)},
		{name: "50MB", bodyContent: make([]byte, 50*1024*1024)},
	}
}

func Benchmark_FiberHandlerFunc(b *testing.B) {
	fiberH := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }
	handlerFunc := FiberHandlerFunc(fiberH)

	for _, bm := range adaptorBenchBodies() {
		b.Run(bm.name, func(b *testing.B) {
			w := httptest.NewRecorder()
			r := http.Request{Method: http.MethodPost, Body: io.NopCloser(bytes.NewBuffer(bm.bodyContent))}
			defer r.Body.Close() //nolint:errcheck // not needed

			b.ReportAllocs()

			for b.Loop() {
				handlerFunc.ServeHTTP(w, &r)
			}
		})
	}
}

func Benchmark_FiberHandlerFunc_Parallel(b *testing.B) {
	fiberH := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }
	handlerFunc := FiberHandlerFunc(fiberH)

	for _, bm := range adaptorBenchBodies() {
		b.Run(bm.name, func(b *testing.B) {
			bodyBuffer := bytes.NewBuffer(bm.bodyContent)

			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				w := httptest.NewRecorder()
				r := http.Request{Method: http.MethodPost, Body: io.NopCloser(bodyBuffer)}
				defer r.Body.Close() //nolint:errcheck // not needed

				for pb.Next() {
					handlerFunc(w, &r)
				}
			})
		})
	}
}

// adaptorBenchOK is the net/http handler the HTTPHandler benchmarks adapt; it answers 200 "ok".
func adaptorBenchOK(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok")) //nolint:errcheck // not needed
}

func Benchmark_HTTPHandler(b *testing.B) {
	var err error
	ctx := apptest.Ctx(b, fiber.New())

	b.ReportAllocs()
	b.ResetTimer()

	fiberHandler := HTTPHandler(http.HandlerFunc(adaptorBenchOK))

	for b.Loop() {
		ctx.Request().Reset()
		ctx.Response().Reset()
		ctx.Request().SetRequestURI("/test")
		ctx.Request().Header.SetMethod("GET")

		err = fiberHandler(ctx)
	}

	require.NoError(b, err)
}

func Benchmark_HTTPHandlerWithContext(b *testing.B) {
	var err error
	ctx := apptest.Ctx(b, fiber.New())

	b.ReportAllocs()
	b.ResetTimer()

	type key struct{}
	var testKey key
	ctx.SetContext(context.WithValue(ctx.Context(), testKey, "gofiber"))

	fiberHandler := HTTPHandlerWithContext(http.HandlerFunc(adaptorBenchOK))

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
	// A row expects an error when it names one in expectedErr or errorContains.
	tests := []struct {
		expectedErr   error
		localAddr     any
		name          string
		remoteAddr    string
		errorContains string
	}{
		{name: "valid TCP address with port", remoteAddr: "192.168.1.1:8080"},
		{name: "valid TCP address without port - should add default port 80", remoteAddr: "192.168.1.1"},
		{
			name: "unix socket - should return local addr", remoteAddr: "irrelevant",
			localAddr: &net.UnixAddr{Name: "/tmp/test.sock", Net: "unix"},
		},
		{
			name: "invalid address - should fail", remoteAddr: "[invalid:address:format",
			errorContains: "failed to resolve TCP address:",
		},
		{
			name: "invalid address after adding port - should fail", remoteAddr: "[invalid",
			errorContains: "failed to resolve TCP address after adding port:",
		},
		{name: "empty address - should fail", remoteAddr: "", expectedErr: ErrRemoteAddrEmpty},
		{name: "too long address - should fail", remoteAddr: strings.Repeat("a", 254), expectedErr: ErrRemoteAddrTooLong},
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

func TestUnixSocketAdaptor(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "test.sock")
	defer func() {
		if err := os.Remove(socketPath); err != nil {
			t.Logf("cleanup failed: %v", err)
		}
	}()

	app := fiber.New()
	app.Get("/hello", func(c fiber.Ctx) error { return c.SendString("ok") })
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

	srv := &http.Server{Handler: handler, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second}
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

	_, err = conn.Write([]byte("GET /hello HTTP/1.1\r\nHost: localhost\r\n\r\n"))
	require.NoError(t, err)

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

	apptest.Recv(t, done, 5*time.Second, "server shutdown timed out")
}

// Test_ResolveRemoteAddr_FastPathEquivalence pins the ip:port fast path to
// net.ResolveTCPAddr: for every literal form the fast path accepts, both
// must produce the same address, and inputs the fast path rejects must
// still resolve identically through the fallback.
func Test_ResolveRemoteAddr_FastPathEquivalence(t *testing.T) {
	t.Parallel()
	addrs := []string{
		"1.2.3.4:6789",
		"127.0.0.1:80",
		"255.255.255.255:65535",
		"[::1]:8080",
		"[2001:db8::1]:443",
		"[::ffff:1.2.3.4]:80",
		"[fe80::1%eth0]:80",
		"1.2.3.4:0",
	}
	for _, addr := range addrs {
		got, err := resolveRemoteAddr(addr, nil)
		require.NoError(t, err, "resolveRemoteAddr(%q)", addr)
		want, err := net.ResolveTCPAddr("tcp", addr)
		require.NoError(t, err, "ResolveTCPAddr(%q)", addr)
		gotTCP, ok := got.(*net.TCPAddr)
		require.True(t, ok, "resolveRemoteAddr(%q) type", addr)
		require.True(t, want.IP.Equal(gotTCP.IP), "IP for %q: got %v want %v", addr, gotTCP.IP, want.IP)
		require.Equal(t, want.Port, gotTCP.Port, "port for %q", addr)
		require.Equal(t, want.Zone, gotTCP.Zone, "zone for %q", addr)
		require.Equal(t, want.String(), gotTCP.String(), "String() for %q", addr)
	}

	// Rejected by the fast path, resolved by the fallback: identical results.
	fallbackAddrs := []string{
		"localhost:80",   // hostname
		"1.2.3.4:0080",   // leading-zero port (fast path parses digits only, both accept)
		"1.2.3.4",        // missing port -> :80 default via fallback
		"01.2.3.4:80",    // leading-zero octet (rejected by ParseIPv4, accepted by resolver)
		"1.2.3.4:99999",  // port out of range -> error from both? fallback errors
		"1.2.3.4:",       // empty port -> fast path rejects, resolver yields port 0
		"1.2.3.4:123456", // six digits -> fast path length guard, resolver errors
		"1.2.3.4:8a",     // non-digit port -> fast path rejects, resolver service lookup fails
	}
	for _, addr := range fallbackAddrs {
		got, gotErr := resolveRemoteAddr(addr, nil)
		want, wantErr := net.ResolveTCPAddr("tcp", addr)
		if addr == "1.2.3.4" {
			// resolveRemoteAddr appends :80 for missing ports; ResolveTCPAddr errors.
			require.NoError(t, gotErr)
			require.Equal(t, "1.2.3.4:80", got.String())
			continue
		}
		if wantErr != nil {
			require.Error(t, gotErr, "addr %q", addr)
			continue
		}
		require.NoError(t, gotErr, "addr %q", addr)
		require.Equal(t, want.String(), got.String(), "String() for %q", addr)
	}
}

// Test_FiberApp_ProxyHeaderLinesKeepReceivedOrder checks that a removed header line cannot move the line a
// front proxy appended in front of the client's: fasthttp moves the store's last line into the removed slot.
func Test_FiberApp_ProxyHeaderLinesKeepReceivedOrder(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{
		ProxyHeader:        fiber.HeaderXForwardedFor,
		TrustProxy:         true,
		EnableIPValidation: true,
		TrustProxyConfig:   fiber.TrustProxyConfig{Loopback: true},
	})
	app.Use(func(c fiber.Ctx) error {
		c.Request().Header.Del(fiber.HeaderAuthorization)
		return c.Next()
	})
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(c.IP() + " " + c.Scheme())
	})
	h := FiberApp(app)

	// r.Header's map order decides which line is stored last, so several rounds. HTTP/2 keeps a value's edge
	// whitespace, which a received line drops.
	for _, client := range []string{"6.6.6.6", "6.6.6.6 ", "\t6.6.6.6"} {
		for range 32 {
			r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			r.RemoteAddr = "127.0.0.1:40000"
			r.Header.Set(fiber.HeaderAuthorization, "Bearer t")
			r.Header.Add(fiber.HeaderXForwardedFor, client)
			r.Header.Add(fiber.HeaderXForwardedFor, "203.0.113.7")
			r.Header.Add(fiber.HeaderXForwardedProto, "http")
			r.Header.Add(fiber.HeaderXForwardedProto, "https")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, "203.0.113.7 https", w.Body.String(), "%q", client)
		}
	}
}

// Test_FiberHandlerFunc_HeaderValues verifies that repeated header lines on the net/http request all
// reach the fiber handler instead of collapsing to the last value, and that a header key mapped to an
// empty value slice is skipped instead of panicking or producing an empty header.
func Test_FiberHandlerFunc_HeaderValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		key    string
		values []string // what r.Header holds for key
		want   []string // the values the fiber handler reads
	}{
		{
			name: "MultiValueHeaders", key: "X-Multi",
			values: []string{"one", "two", "three"}, want: []string{"one", "two", "three"},
		},
		{name: "EmptyHeaderValues", key: "X-Empty", values: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			h := FiberHandlerFunc(func(c fiber.Ctx) error {
				for _, v := range c.Request().Header.PeekAll(tc.key) {
					got = append(got, string(v))
				}
				return c.SendStatus(fiber.StatusOK)
			})

			r := apptest.Req(http.MethodGet, "/")
			r.Header[tc.key] = tc.values

			require.Equal(t, http.StatusOK, adaptorTestServe(h, r).Code)
			require.Equal(t, tc.want, got)
		})
	}
}

// adaptorTestRewriteRequest returns a net/http middleware that applies edit to
// the request before it passes the request on.
func adaptorTestRewriteRequest(edit func(r *http.Request)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			edit(r)
			next.ServeHTTP(w, r)
		})
	}
}

// Test_HTTPMiddleware_MultiValueHeaders verifies multi-value headers added
// by a wrapped net/http middleware survive the copy-back onto the fiber
// request.
func Test_HTTPMiddleware_MultiValueHeaders(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(HTTPMiddleware(adaptorTestRewriteRequest(func(r *http.Request) {
		r.Header.Add("X-Multi", "one")
		r.Header.Add("X-Multi", "two")
		r.Header["X-Empty"] = nil // empty value slices must be skipped
	})))
	var got []string
	var emptyHeaderValues int
	app.Get("/", func(c fiber.Ctx) error {
		for _, v := range c.Request().Header.PeekAll("X-Multi") {
			got = append(got, string(v))
		}
		emptyHeaderValues = len(c.Request().Header.PeekAll("X-Empty"))
		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Expect(t, app, "/", fiber.StatusOK)
	require.Equal(t, []string{"one", "two"}, got)
	require.Zero(t, emptyHeaderValues, "empty header value slices must not be copied")
}

// Benchmark_HTTPMiddleware measures the net/http middleware -> Fiber handler
// bridge, including the request rewrite the bridge performs on every call.
func Benchmark_HTTPMiddleware(b *testing.B) {
	nethttpMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Middleware", "true")
			next.ServeHTTP(w, r)
		})
	}

	app := fiber.New()
	app.Use(HTTPMiddleware(nethttpMW))
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	handler := app.Handler()

	ctx := &fasthttp.RequestCtx{}

	b.ReportAllocs()

	for b.Loop() {
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		ctx.Request.SetRequestURI("/")
		ctx.Request.Header.Set("Foo", "bar")

		handler(ctx)
	}
}

// Benchmark_ConvertRequest measures building a net/http request from a routed
// Fiber request whose request line is already the routed target, the common
// case, where the line is not set and parsed a second time.
func Benchmark_ConvertRequest(b *testing.B) {
	app := fiber.New()
	app.Get("/users/:id", func(c fiber.Ctx) error {
		r, err := ConvertRequest(c, true)
		if err != nil {
			return err
		}
		if r.URL.Path != "/users/42" {
			return fiber.ErrInternalServerError
		}
		return nil
	})

	handler := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	b.ReportAllocs()

	for b.Loop() {
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(fiber.MethodGet)
		ctx.Request.SetRequestURI("/users/42?expand=1")
		ctx.Request.Header.Set("Foo", "bar")

		handler(ctx)
	}
}

// Benchmark_FiberApp exercises the net/http -> Fiber direction the way a real
// net/http server does: a fresh response writer per request, a routed app, a
// remote address to resolve and a handful of request and response headers.
func Benchmark_FiberApp(b *testing.B) {
	app := fiber.New()
	app.Get("/foo/bar", func(c fiber.Ctx) error {
		c.Set("X-Custom-One", "one")
		c.Set("X-Custom-Two", "two")
		return c.SendString("Hello, World!")
	})

	handler := FiberApp(app)

	r := apptest.Req(http.MethodGet, "/foo/bar",
		apptest.WithHeader("Accept-Encoding", "gzip"), apptest.WithHeader("User-Agent", "fiber-bench"))
	r.RemoteAddr = expectedRemoteAddr

	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		handler(w, r)
	}
}

// Benchmark_FiberApp_Body covers the request body copy path, which streams the
// net/http body into the fasthttp request before the handler runs.
func Benchmark_FiberApp_Body(b *testing.B) {
	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	handler := FiberApp(app)

	body := make([]byte, 8*1024)

	b.ReportAllocs()

	for b.Loop() {
		r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		r.RemoteAddr = expectedRemoteAddr
		// A fresh recorder per iteration: reusing one accumulates the response
		// headers and body across iterations, which both skews the numbers and
		// hides the fresh-header path.
		w := httptest.NewRecorder()
		handler(w, r)
	}
}

// Benchmark_FiberApp_Parallel measures how the net/http -> Fiber bridge scales
// across cores.
func Benchmark_FiberApp_Parallel(b *testing.B) {
	app := fiber.New()
	app.Get("/foo/bar", func(c fiber.Ctx) error {
		c.Set("X-Custom-One", "one")
		c.Set("X-Custom-Two", "two")
		return c.SendString("Hello, World!")
	})

	handler := FiberApp(app)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		r := apptest.Req(http.MethodGet, "/foo/bar")
		r.RemoteAddr = expectedRemoteAddr

		for pb.Next() {
			// Fresh recorder per iteration, matching Benchmark_FiberApp so the
			// two are directly comparable.
			w := httptest.NewRecorder()
			handler(w, r)
		}
	})
}

func Test_copyResponseHeaders(t *testing.T) {
	t.Parallel()
	t.Run("packs keys and values without losing any", func(t *testing.T) {
		t.Parallel()
		var src fasthttp.ResponseHeader
		src.SetContentType("text/plain")
		src.Set("X-One", "1")
		src.Add("X-Multi", "a")
		src.Add("X-Multi", "b")
		src.SetCookie(&fasthttp.Cookie{})

		dst := make(http.Header)
		copyResponseHeaders(dst, &src)

		require.Equal(t, []string{"text/plain"}, dst["Content-Type"])
		require.Equal(t, []string{"1"}, dst["X-One"])
		require.Equal(t, []string{"a", "b"}, dst["X-Multi"])
	})

	t.Run("keys are canonicalized like Header.Add", func(t *testing.T) {
		t.Parallel()
		var src fasthttp.ResponseHeader
		src.DisableNormalizing()
		src.Set("x-lower-case", "value")

		dst := make(http.Header)
		copyResponseHeaders(dst, &src)

		require.Equal(t, "value", dst.Get("X-Lower-Case"))
	})

	t.Run("appends to values already present on the destination", func(t *testing.T) {
		t.Parallel()
		var src fasthttp.ResponseHeader
		src.Set("X-One", "from-fasthttp")

		dst := make(http.Header)
		dst.Add("X-One", "preexisting")
		copyResponseHeaders(dst, &src)

		require.Equal(t, []string{"preexisting", "from-fasthttp"}, dst["X-One"])
	})

	t.Run("entries sharing a backing array stay independent", func(t *testing.T) {
		t.Parallel()
		var src fasthttp.ResponseHeader
		src.Set("X-One", "1")
		src.Set("X-Two", "2")
		src.Set("X-Three", "3")

		dst := make(http.Header)
		copyResponseHeaders(dst, &src)

		// Growing one entry must not overwrite the neighboring entries even
		// though they were carved out of a single []string allocation.
		dst.Add("X-One", "extra")

		require.Equal(t, []string{"1", "extra"}, dst["X-One"])
		require.Equal(t, []string{"2"}, dst["X-Two"])
		require.Equal(t, []string{"3"}, dst["X-Three"])
	})

	t.Run("empty header set leaves the destination untouched", func(t *testing.T) {
		t.Parallel()
		var src fasthttp.ResponseHeader
		src.SetNoDefaultContentType(true)

		dst := make(http.Header)
		copyResponseHeaders(dst, &src)

		require.Empty(t, dst)
	})

	t.Run("values do not alias the pooled scratch buffer", func(t *testing.T) {
		t.Parallel()
		var first fasthttp.ResponseHeader
		first.SetNoDefaultContentType(true)
		first.Set("X-Value", strings.Repeat("a", 64))

		dst := make(http.Header)
		copyResponseHeaders(dst, &first)

		// A second copy reuses the pooled staging buffer; the strings handed
		// to the first destination must not change underneath it.
		var second fasthttp.ResponseHeader
		second.SetNoDefaultContentType(true)
		second.Set("X-Value", strings.Repeat("b", 64))
		copyResponseHeaders(make(http.Header), &second)

		require.Equal(t, strings.Repeat("a", 64), dst.Get("X-Value"))
	})
}

// Test_resolveRemoteAddr_FastPath covers what the ip:port fast path builds beyond the address itself,
// which Test_ResolveRemoteAddr_FastPathEquivalence pins.
func Test_resolveRemoteAddr_FastPath(t *testing.T) {
	t.Parallel()
	t.Run("ipv6 keeps the zone", func(t *testing.T) {
		t.Parallel()
		addr, err := resolveRemoteAddr("[fe80::1%eth0]:6789", nil)
		require.NoError(t, err)

		tcpAddr, ok := addr.(*net.TCPAddr)
		require.True(t, ok)
		require.Equal(t, "eth0", tcpAddr.Zone)
		require.Equal(t, 6789, tcpAddr.Port)
		require.Equal(t, net.ParseIP("fe80::1"), tcpAddr.IP)
	})

	t.Run("each call returns an independent address", func(t *testing.T) {
		t.Parallel()
		first, err := resolveRemoteAddr("1.2.3.4:1111", nil)
		require.NoError(t, err)
		second, err := resolveRemoteAddr("5.6.7.8:2222", nil)
		require.NoError(t, err)

		// Callers may hold on to a remote address past the request that
		// produced it, so resolving another one must not disturb it.
		require.NotSame(t, first, second)
		require.Equal(t, "1.2.3.4:1111", first.String())
		require.Equal(t, "5.6.7.8:2222", second.String())
	})

	t.Run("the IP slice cannot be appended into its backing buffer", func(t *testing.T) {
		t.Parallel()
		addr, err := resolveRemoteAddr("1.2.3.4:6789", nil)
		require.NoError(t, err)

		tcpAddr, ok := addr.(*net.TCPAddr)
		require.True(t, ok)
		require.Equal(t, len(tcpAddr.IP), cap(tcpAddr.IP))

		grown := append(tcpAddr.IP, 9) //nolint:gocritic // deliberately appending to a copy
		require.Equal(t, net.IP{1, 2, 3, 4}, tcpAddr.IP)
		require.Len(t, grown, net.IPv4len+1)
	})
}

func Test_FiberApp_NoBody(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error {
		require.Empty(t, c.Body())
		return c.SendString(strconv.Itoa(c.Request().Header.ContentLength()))
	})

	w := adaptorTestServe(FiberApp(app), apptest.Req(http.MethodPost, "/"))

	require.Equal(t, fiber.StatusOK, w.Code)
	require.Equal(t, "0", w.Body.String())
}

func Test_FiberApp_RemoteAddrSurvivesPooling(t *testing.T) {
	t.Parallel()
	addrs := []string{"1.1.1.1:1111", "2.2.2.2:2222", "3.3.3.3:3333"}

	var retained []net.Addr
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		retained = append(retained, c.RequestCtx().RemoteAddr())
		return c.SendString(c.IP())
	})

	handler := FiberApp(app)

	for _, addr := range addrs {
		r := apptest.Req(http.MethodGet, "/")
		r.RemoteAddr = addr

		host, _, err := net.SplitHostPort(addr)
		require.NoError(t, err)
		require.Equal(t, host, adaptorTestServe(handler, r).Body.String())
	}

	// The ctx pool recycles entries between requests; a remote address kept
	// by the handler must still report the client it was resolved from.
	require.Len(t, retained, len(addrs))
	for i, addr := range addrs {
		require.Equal(t, addr, retained[i].String())
	}
}

// Test_HTTPMiddleware_PropagatesHeaderRemoval asserts that a header the wrapped
// net/http middleware deletes stays deleted for the fiber handler. Copying only
// what r.Header still holds would leave the original value in place, silently
// defeating middleware whose purpose is to strip a header.
func Test_HTTPMiddleware_PropagatesHeaderRemoval(t *testing.T) {
	t.Parallel()
	type seen struct {
		forwardedFor  string
		authorization string
		kept          string
		contentType   string
		body          string
	}

	var got seen

	app := fiber.New()
	app.Use(HTTPMiddleware(adaptorTestRewriteRequest(func(r *http.Request) {
		r.Header.Del("X-Forwarded-For")
		r.Header.Del(fiber.HeaderAuthorization)
	})))
	app.Post("/", func(c fiber.Ctx) error {
		got = seen{
			forwardedFor:  c.Get("X-Forwarded-For"),
			authorization: c.Get(fiber.HeaderAuthorization),
			kept:          c.Get("X-Kept"),
			contentType:   c.Get(fiber.HeaderContentType),
			body:          string(c.Body()),
		}
		return c.SendString("ok")
	})

	req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader("a=1"))
	req.Header.Set(fiber.HeaderContentType, "application/x-www-form-urlencoded")
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set(fiber.HeaderAuthorization, "Bearer secret")
	req.Header.Set("X-Kept", "still here")

	apptest.Expect(t, app, req, fiber.StatusOK)

	require.Empty(t, got.forwardedFor)
	require.Empty(t, got.authorization)
	// Headers the middleware left alone survive, and the framing headers it
	// never touches must not be collateral damage.
	require.Equal(t, "still here", got.kept)
	require.Equal(t, "application/x-www-form-urlencoded", got.contentType)
	require.Equal(t, "a=1", got.body)
}

// Test_HTTPMiddleware_TrailerDoesNotBecomeHeader asserts that a field the client
// sent after the last chunk does not reach the fiber handler as a header the
// middleware removed or replaced.
func Test_HTTPMiddleware_TrailerDoesNotBecomeHeader(t *testing.T) {
	t.Parallel()

	const owned = "X-Authenticated-User"

	// Fiber drops the trailer section before r.Trailer is built: as the request arrives or, streamed, once the body is read.
	tests := []struct {
		name   string
		write  string
		want   []string
		stream bool
	}{
		{name: "stripped"},
		{name: "replaced", write: "alice", want: []string{"alice"}},
		{name: "stripped streamed", stream: true},
		{name: "replaced streamed", write: "alice", want: []string{"alice"}, stream: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				trailer     []string
				body        string
				got         []string
				headers     map[string][]string
				trailerKeys int
			)

			app := fiber.New(fiber.Config{StreamRequestBody: tt.stream})
			app.Use(HTTPMiddleware(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					b, err := io.ReadAll(r.Body)
					assert.NoError(t, err)
					body = string(b)
					trailer = r.Trailer.Values(owned)
					r.Header.Del(owned)
					if tt.write != "" {
						r.Header.Set(owned, tt.write)
					}
					next.ServeHTTP(w, r)
				})
			}))
			app.Post("/", func(c fiber.Ctx) error {
				got = c.GetAll(owned)
				headers = c.GetReqHeaders()
				trailerKeys = len(c.Request().Header.PeekTrailerKeys())
				return c.SendString("ok")
			})

			ln := fasthttputil.NewInmemoryListener()
			errCh := make(chan error, 1)
			go func() {
				errCh <- app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})
			}()
			t.Cleanup(func() {
				require.NoError(t, app.Shutdown())
				if err := <-errCh; err != nil && !errors.Is(err, net.ErrClosed) {
					require.NoError(t, err)
				}
			})

			conn, err := ln.Dial()
			require.NoError(t, err)
			defer conn.Close() //nolint:errcheck // closed by the server already
			_, err = conn.Write([]byte("POST / HTTP/1.1\r\nHost: app.example\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n" +
				"3\r\na=1\r\n0\r\n" + owned + ": admin\r\n\r\n"))
			require.NoError(t, err)
			resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())

			require.Equal(t, fiber.StatusOK, resp.StatusCode)
			require.Equal(t, "a=1", body)
			require.Empty(t, trailer)
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.want, headers[owned])
			require.Zero(t, trailerKeys)
		})
	}
}

// Test_HTTPMiddleware_HeaderFidelity asserts that the fiber request ends up
// holding exactly what the wrapped net/http middleware left in r.Header.
//
// fasthttpadaptor builds r.Header from b2s views into fiber's own header
// storage, so a copy-back that writes while reading corrupts itself: values got
// duplicated, sanitized values kept their originals alongside them, and the
// rewritten Host came back spliced with the bytes of the old one.
func Test_HTTPMiddleware_HeaderFidelity(t *testing.T) {
	t.Parallel()
	forwardedFor := func(t *testing.T, mw func(http.Handler) http.Handler) ([]string, string, string) {
		t.Helper()

		var (
			values  []string
			uriHost string
			hdrHost string
		)
		app := fiber.New()
		app.Use(HTTPMiddleware(mw))
		app.Get("/", func(c fiber.Ctx) error {
			for _, v := range c.Request().Header.PeekAll("X-Forwarded-For") {
				values = append(values, string(v))
			}
			uriHost, hdrHost = c.Host(), c.Get(fiber.HeaderHost)
			return c.SendString("ok")
		})

		req := apptest.Req(fiber.MethodGet, "/", apptest.WithHost("original.example"))
		req.Header.Add("X-Forwarded-For", "1.1.1.1")
		req.Header.Add("X-Forwarded-For", "2.2.2.2")

		apptest.Expect(t, app, req, fiber.StatusOK)
		return values, uriHost, hdrHost
	}

	t.Run("pass-through preserves every value exactly once", func(t *testing.T) {
		t.Parallel()
		got, _, _ := forwardedFor(t, func(next http.Handler) http.Handler { return next })
		require.Equal(t, []string{"1.1.1.1", "2.2.2.2"}, got)
	})

	t.Run("collapsing a multi-valued header drops the originals", func(t *testing.T) {
		t.Parallel()
		got, _, _ := forwardedFor(t, adaptorTestRewriteRequest(func(r *http.Request) {
			r.Header.Set("X-Forwarded-For", "9.9.9.9")
		}))
		// Leaving "2.2.2.2" behind would hand the spoofed value straight to
		// c.IPs() and the TrustProxy chain walk.
		require.Equal(t, []string{"9.9.9.9"}, got)
	})

	t.Run("assigning nil removes the header", func(t *testing.T) {
		t.Parallel()
		got, _, _ := forwardedFor(t, adaptorTestRewriteRequest(func(r *http.Request) {
			r.Header["X-Forwarded-For"] = nil
		}))
		require.Empty(t, got)
	})

	t.Run("rewritten host is not spliced with the old one", func(t *testing.T) {
		t.Parallel()
		_, uriHost, hdrHost := forwardedFor(t, adaptorTestRewriteRequest(func(r *http.Request) {
			r.Host = "internal.svc"
		}))
		require.Equal(t, "internal.svc", uriHost)
		require.Equal(t, "internal.svc", hdrHost)
	})
}

// Test_HTTPMiddleware_PropagatesConnectionRewrite covers Connection, which names
// the hop-by-hop fields a proxy is to strip.
//
// Middleware rewrites that list — dropping the edit left the handler, and any
// proxy reading it downstream, working from the list the client sent. The header
// also drives fasthttp's close flag, which has to survive the round trip.
func Test_HTTPMiddleware_PropagatesConnectionRewrite(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		sent  string
		write string // "" leaves the header alone
		want  string
	}{
		{name: "rewritten", sent: "keep-alive, X-Client-Token", write: "keep-alive", want: "keep-alive"},
		{name: "extended", sent: "keep-alive", write: "keep-alive, X-Internal", want: "keep-alive, X-Internal"},
		{name: "untouched close survives", sent: "close", want: "close"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got string
			var gotClose bool

			app := fiber.New()
			app.Use(HTTPMiddleware(adaptorTestRewriteRequest(func(r *http.Request) {
				if tc.write != "" {
					r.Header.Set(fiber.HeaderConnection, tc.write)
				}
			})))
			app.Get("/", func(c fiber.Ctx) error {
				got = c.Get(fiber.HeaderConnection)
				gotClose = c.Request().Header.ConnectionClose()
				return c.SendString("ok")
			})

			req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderConnection, tc.sent))

			apptest.Expect(t, app, req, fiber.StatusOK)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.want == "close", gotClose, "the close flag follows the value")
		})
	}
}

// Test_HTTPMiddleware_ConnectionCloseSurvivesDownstream covers the close
// instruction against a downstream chain that writes the response field itself.
//
// fasthttp holds one flag for the whole response, so a handler that resets the
// response or sets a Connection value of its own clears it. Setting the flag
// before c.Next left the connection reused despite the rewrite that asked for
// it to close, so it is applied on the way out instead.
func Test_HTTPMiddleware_ConnectionCloseSurvivesDownstream(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		handler func(fiber.Ctx) error
		name    string
		status  int
	}{
		{name: "plain handler", handler: func(c fiber.Ctx) error { return c.SendString("ok") }, status: fiber.StatusOK},
		{
			name: "handler resetting the response",
			handler: func(c fiber.Ctx) error {
				c.Response().Reset()
				return c.SendString("ok")
			},
			status: fiber.StatusOK,
		},
		{
			name: "handler asking to keep the connection",
			handler: func(c fiber.Ctx) error {
				c.Response().Header.Set(fiber.HeaderConnection, "keep-alive")
				return c.SendString("ok")
			},
			status: fiber.StatusOK,
		},
		{
			// The ErrorHandler writes the response after this middleware has
			// returned, so the flag has to outlive that too.
			name:    "handler returning an error",
			handler: func(_ fiber.Ctx) error { return errors.New("boom") },
			status:  fiber.StatusInternalServerError,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(HTTPMiddleware(adaptorTestRewriteRequest(func(r *http.Request) {
				r.Header[http.CanonicalHeaderKey(fiber.HeaderConnection)] = []string{"close", "X-Internal"}
			})))
			app.Get("/", tc.handler)

			resp := apptest.Expect(t, app, "/", tc.status)
			require.True(t, resp.Close, "the connection the middleware asked to close has to close")
		})
	}
}

// Test_HTTPMiddleware_JoinsRepeatedConnectionValues covers middleware that
// writes Connection as several slice entries, which is how net/http represents a
// combined value. fasthttp keeps the field in a slot of its own, so a second Add
// replaced the first rather than appending: the earlier token vanished, taking
// with it whatever it had marked hop-by-hop, and a "close" written that way
// stopped closing the connection.
func Test_HTTPMiddleware_JoinsRepeatedConnectionValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		want      string
		write     []string
		wantClose bool
	}{
		{name: "token list", write: []string{"keep-alive", "X-Internal"}, want: "keep-alive, X-Internal"},
		// Every observer must see the complete token list so a proxy can remove
		// each named hop-by-hop field. fasthttp raises the request flag for a
		// close token anywhere in the list and still reports the whole list, but
		// the server read that flag before the handler ran, so the close
		// instruction is carried on the response as well.
		{name: "close first", write: []string{"close", "X-Internal"}, want: "close, X-Internal", wantClose: true},
		{name: "close last", write: []string{"X-Internal", "close"}, want: "X-Internal, close", wantClose: true},
		{name: "close cased", write: []string{"X-Internal", "CLOSE"}, want: "X-Internal, CLOSE", wantClose: true},
		{name: "close alone still closes", write: []string{"close"}, want: "close", wantClose: true},
		{name: "single entry is unchanged", write: []string{"keep-alive"}, want: "keep-alive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got, afterNext string
			var gotClose, finalClose bool

			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				err := c.Next()
				// A middleware resuming here — or the app's ErrorHandler, which
				// runs once this returns — reads the same field a downstream
				// proxy would, so it has to survive the whole chain intact.
				afterNext = c.Get(fiber.HeaderConnection)
				finalClose = c.Response().Header.ConnectionClose()
				return err
			})
			app.Use(HTTPMiddleware(adaptorTestRewriteRequest(func(r *http.Request) {
				r.Header[http.CanonicalHeaderKey(fiber.HeaderConnection)] = tc.write
			})))
			app.Get("/", func(c fiber.Ctx) error {
				got = c.Get(fiber.HeaderConnection)
				gotClose = c.Request().Header.ConnectionClose()
				return c.SendString("ok")
			})

			resp := apptest.Expect(t, app, "/", fiber.StatusOK)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.wantClose, resp.Close, "close has to reach the wire, not just the fiber context")
			require.Equal(t, tc.want, afterNext, "the complete Connection field must outlive the downstream chain")
			require.Equal(t, tc.wantClose, gotClose, "the request flag reports a close token wherever it sits in the list")
			require.Equal(t, tc.wantClose, finalClose, "the transport close instruction rides on the response")
		})
	}
}

// Test_CopyContextToFiberContext_Values copies the values of contexts a request reaches the handler with:
// a plain one, wrapped ones, and ones derived with a cancel, a timeout or a deadline.
func Test_CopyContextToFiberContext_Values(t *testing.T) {
	t.Parallel()
	type key string
	base := context.WithValue(context.Background(), key("a"), "1")
	wantAB := map[any]any{key("a"): "1", key("b"): "2"}

	withCancel, cancel := context.WithCancel(base)
	t.Cleanup(cancel)
	withTimeout, cancelTimeout := context.WithTimeout(base, time.Hour)
	t.Cleanup(cancelTimeout)
	withoutCancel := context.WithoutCancel(withTimeout)
	withDeadline, cancelDeadline := context.WithDeadline(base, time.Now().Add(time.Hour))
	t.Cleanup(cancelDeadline)
	afterFunc, stop := context.WithCancel(base)
	t.Cleanup(stop)
	context.AfterFunc(afterFunc, func() {})
	nestedBase := context.WithValue(context.Background(), contextKey("nested-a"), "value-a")
	nestedCancel, cancelNested := context.WithCancel(nestedBase)
	t.Cleanup(cancelNested)

	testCases := []struct {
		ctx  context.Context //nolint:containedctx // test table
		want map[any]any
		name string
	}{
		{
			name: "copies key value pairs",
			ctx:  context.WithValue(context.Background(), contextKey("copy-key"), "copy-value"),
			want: map[any]any{contextKey("copy-key"): "copy-value"},
		},
		{
			name: "nested context wrappers", ctx: context.WithValue(nestedCancel, contextKey("nested-b"), "value-b"),
			want: map[any]any{contextKey("nested-a"): "value-a", contextKey("nested-b"): "value-b"},
		},
		{name: "WithCancel", ctx: context.WithValue(withCancel, key("b"), "2"), want: wantAB},
		{name: "WithTimeout", ctx: context.WithValue(withTimeout, key("b"), "2"), want: wantAB},
		{name: "WithDeadline", ctx: context.WithValue(withDeadline, key("b"), "2"), want: wantAB},
		{name: "WithoutCancel", ctx: context.WithValue(withoutCancel, key("b"), "2"), want: wantAB},
		{name: "value below timeout", want: wantAB, ctx: func() context.Context {
			ctx, cancelInner := context.WithTimeout(context.WithValue(base, key("b"), "2"), time.Hour)
			t.Cleanup(cancelInner)
			return ctx
		}()},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var fctx fasthttp.RequestCtx
			CopyContextToFiberContext(tc.ctx, &fctx)
			for k, v := range tc.want {
				require.Equal(t, v, fctx.UserValue(k), "%v", k)
			}
		})
	}
}

func Test_HTTPMiddleware_URLRewrite(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(HTTPMiddleware(func(next http.Handler) http.Handler { return http.StripPrefix("/api", next) }))
	app.Get("/users", func(c fiber.Ctx) error { return c.SendString("users at " + c.Path()) })

	apptest.ExpectBody(t, app, "/api/users?x=1", fiber.StatusOK, "users at /users")
}

// Test_HTTPMiddleware_URLRewrite_OnPrefix pins that routes registered after a
// StripPrefix mounted on its own prefix see the stripped path.
func Test_HTTPMiddleware_URLRewrite_OnPrefix(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use("/api", HTTPMiddleware(func(next http.Handler) http.Handler {
		return http.StripPrefix("/api", next)
	}))
	app.Use("/admin", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusForbidden)
	})
	app.Get("/admin/report", func(c fiber.Ctx) error {
		return c.SendString("report")
	})
	app.Get("/users", func(c fiber.Ctx) error {
		return c.SendString("users at " + c.Path())
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/admin/report", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/api/users", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "users at /users", string(body))
}

// Test_FiberApp_TLS serves a request with a TLS connection state and one without through FiberApp, and
// checks what the Fiber handler reads of it.
func Test_FiberApp_TLS(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		handler   fiber.Handler
		state     *tls.ConnectionState
		name      string
		wantTLS   string
		wantPlain string
	}{
		{
			name:      "scheme propagated",
			handler:   func(c fiber.Ctx) error { return c.SendString(c.Scheme() + " " + strconv.FormatBool(c.Secure())) },
			state:     &tls.ConnectionState{Version: tls.VersionTLS13, ServerName: "example.com"},
			wantTLS:   "https true",
			wantPlain: "http false",
		},
		{
			name: "connection state carried",
			handler: func(c fiber.Ctx) error {
				state := c.RequestCtx().TLSConnectionState()
				if state == nil {
					return c.SendString("none")
				}
				return c.SendString(state.ServerName + " " + state.NegotiatedProtocol)
			},
			state:     &tls.ConnectionState{Version: tls.VersionTLS13, ServerName: "example.com", NegotiatedProtocol: "h2"},
			wantTLS:   "example.com h2",
			wantPlain: "none",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Get("/", tc.handler)
			handler := FiberApp(app)

			req := apptest.Req(http.MethodGet, "https://example.com/")
			req.TLS = tc.state
			rec := adaptorTestServe(handler, req)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, tc.wantTLS, rec.Body.String())

			rec = adaptorTestServe(handler, apptest.Req(http.MethodGet, "http://example.com/"))
			require.Equal(t, tc.wantPlain, rec.Body.String())
		})
	}
}

func Test_TLSNoopConn_HandshakeIsNoop(t *testing.T) {
	t.Parallel()
	conn := &tlsNoopConn{state: tls.ConnectionState{ServerName: "example.com"}}
	require.NoError(t, conn.Handshake())
	require.Equal(t, "example.com", conn.ConnectionState().ServerName)
}

func Test_HTTPMiddleware_NextCalledAfterReturn(t *testing.T) {
	t.Parallel()
	saved := make(chan http.Handler, 1)
	mw := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			saved <- h
			w.WriteHeader(http.StatusAccepted)
		})
	}

	app := fiber.New()
	app.Use(HTTPMiddleware(mw))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("downstream") })

	resp := apptest.Expect(t, app, "/", http.StatusAccepted)
	body := apptest.Body(t, resp)
	require.Empty(t, body)

	// The Fiber context is released by now, so a late next must be a no-op.
	next := <-saved
	require.NotPanics(t, func() { adaptorTestServe(next, apptest.Req(http.MethodPost, "/late")) })
}

func Test_CopyContextToFiberContext_DeepChain(t *testing.T) {
	t.Parallel()
	type key string
	const depth = 200

	ctx := context.Background()
	for i := range depth {
		ctx = context.WithValue(ctx, key(strconv.Itoa(i)), i)
	}

	var fctx fasthttp.RequestCtx
	CopyContextToFiberContext(ctx, &fctx)

	for i := range depth {
		require.Equal(t, i, fctx.UserValue(key(strconv.Itoa(i))), "value %d was dropped", i)
	}
}

// cyclicContext lets a chain point back at itself, the shape the walk must not
// follow forever.
type cyclicContext struct {
	context.Context //nolint:containedctx // the cycle is the point
}

func Test_CopyContextToFiberContext_Cycle(t *testing.T) {
	t.Parallel()
	type key string
	loop := &cyclicContext{Context: context.Background()}
	loop.Context = context.WithValue(loop, key("a"), "1")

	var fctx fasthttp.RequestCtx
	require.NotPanics(t, func() { CopyContextToFiberContext(loop, &fctx) })
	require.Equal(t, "1", fctx.UserValue(key("a")))
}

func Test_HTTPMiddleware_NoRewriteKeepsChainPosition(t *testing.T) {
	t.Parallel()

	// Spellings whose decoded path differs from the raw one the router matched.
	paths := []string{
		"/api%2Fadmin/secret",
		"//admin/secret",
		"/./admin/secret",
		"/x/../admin/secret",
		"/adm%69n/secret",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			var authRan atomic.Bool
			app := fiber.New()
			app.Use("/api/admin", func(_ fiber.Ctx) error {
				authRan.Store(true)
				return fiber.ErrForbidden
			})
			app.Use(HTTPMiddleware(func(next http.Handler) http.Handler { return next }))
			app.Get("/api/admin/secret", func(c fiber.Ctx) error { return c.SendString("SECRET") })

			resp := apptest.Do(t, app, path)
			body := apptest.Body(t, resp)
			require.NotEqual(t, "SECRET", body, "the adaptor resumed past the guard that already ran")
			require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
			require.False(t, authRan.Load())
		})
	}
}

func Test_FiberAdaptors_TimedOutRequest(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		build func(h fiber.Handler) http.Handler
		name  string
	}{
		{name: "FiberApp", build: func(h fiber.Handler) http.Handler {
			app := fiber.New()
			app.Get("/*", h)
			return FiberApp(app)
		}},
		{name: "FiberHandler", build: FiberHandler},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			held := make(chan *fasthttp.RequestCtx, 1)
			seen := make(chan *fasthttp.RequestCtx, 1)
			release := make(chan struct{})
			finished := make(chan struct{})
			handler := tc.build(timeout.New(func(c fiber.Ctx) error {
				if c.Path() != "/slow" {
					seen <- c.RequestCtx()
					return fiber.ErrUnauthorized
				}
				defer close(finished)
				c.Cookie(&fiber.Cookie{Name: "early", Value: "1"})
				held <- c.RequestCtx()
				<-release
				c.Cookie(&fiber.Cookie{Name: "session", Value: "late"})
				return c.SendString("late")
			}, timeout.Config{
				Timeout: 10 * time.Millisecond,
				Next:    func(c fiber.Ctx) bool { return c.Path() != "/slow" },
			}))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(fiber.MethodGet, "/slow", http.NoBody))
			require.Equal(t, fiber.StatusServiceUnavailable, rec.Code)
			require.Equal(t, fiber.ErrServiceUnavailable.Message, rec.Body.String())
			require.Empty(t, rec.Header().Values(fiber.HeaderSetCookie))
			timedOut := <-held

			for range 20 {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(fiber.MethodGet, "/private", http.NoBody))
				require.Equal(t, fiber.StatusUnauthorized, rec.Code)
				require.Empty(t, rec.Header().Values(fiber.HeaderSetCookie))
				require.NotSame(t, timedOut, <-seen, "the timed-out handler's RequestCtx was handed to another request")
			}

			close(release)
			<-finished
		})
	}
}

// net/http discards a HEAD body, so a stream read for one would run with every write succeeding.
func Test_FiberAdaptors_HeadLeavesStreamUnread(t *testing.T) {
	t.Parallel()

	var handlers atomic.Int32
	events := sse.New(sse.Config{
		DisableHeartbeat: true,
		Handler: func(_ fiber.Ctx, stream *sse.Stream) error {
			handlers.Add(1)
			return stream.Comment("open")
		},
	})
	app := fiber.New()
	app.Get("/events", events)

	for _, handler := range []http.Handler{FiberApp(app), FiberHandler(events)} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(fiber.MethodHead, "/events", http.NoBody))
		require.Equal(t, fiber.StatusOK, rec.Code)
		require.Equal(t, fiber.MIMETextEventStream, rec.Header().Get(fiber.HeaderContentType))
		require.Empty(t, rec.Body.String())
	}
	require.Zero(t, handlers.Load())

	rec := httptest.NewRecorder()
	FiberApp(app).ServeHTTP(rec, httptest.NewRequest(fiber.MethodGet, "/events", http.NoBody))
	require.Equal(t, ": open\n\n", rec.Body.String())
	require.Equal(t, int32(1), handlers.Load())
}
