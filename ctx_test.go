// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/tls"
	"embed"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofiber/utils/v2"
	"github.com/shamaton/msgpack/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3/internal/storage/memory"
)

const epsilon = 0.001

type testContextKey struct{}

type testNetAddr struct {
	network string
	address string
}

func (t testNetAddr) Network() string {
	return t.network
}

func (t testNetAddr) String() string {
	return t.address
}

// go test -run Test_Ctx_Accepts
func Test_Ctx_Accepts(t *testing.T) {
	t.Parallel()
	app := New(Config{
		CBOREncoder: cbor.Marshal,
		CBORDecoder: cbor.Unmarshal,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAccept, "text/html,application/xhtml+xml,application/xml;q=0.9")
	require.Empty(t, c.Accepts(""))
	require.Empty(t, c.Req().Accepts())
	require.Equal(t, ".xml", c.Accepts(".xml"))
	require.Empty(t, c.Accepts(".john"))
	require.Equal(t, "application/xhtml+xml", c.Accepts("application/xml", "application/xml+rss", "application/yaml", "application/xhtml+xml"), "must use client-preferred mime type")

	c.Request().Header.Set(HeaderAccept, "application/json, text/plain, */*;q=0")
	require.Empty(t, c.Accepts("html"), "must treat */*;q=0 as not acceptable")

	c.Request().Header.Set(HeaderAccept, "text/*, application/json")
	require.Equal(t, "html", c.Accepts("html"))
	require.Equal(t, "text/html", c.Accepts("text/html"))
	require.Equal(t, "json", c.Req().Accepts("json", "text"))
	require.Equal(t, "application/json", c.Accepts("application/json"))
	require.Empty(t, c.Accepts("image/png"))
	require.Empty(t, c.Accepts("png"))

	c.Request().Header.Set(HeaderAccept, "text/html, application/json")
	require.Equal(t, "text/*", c.Req().Accepts("text/*"))

	c.Request().Header.Set(HeaderAccept, "*/*")
	require.Equal(t, "html", c.Accepts("html"))

	c.Request().Header.Del(HeaderAccept)
	require.Equal(t, "json", c.Accepts("json", "html"))
	require.Equal(t, "application/json", c.Accepts("application/json", "text/html"))
}

// go test -run Test_Ctx_AcceptsHelpers
func Test_Ctx_AcceptsHelpers(t *testing.T) {
	t.Parallel()

	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAccept, "text/html,application/json;q=0.9")
	require.True(t, c.AcceptsHTML())
	require.True(t, c.AcceptsJSON())
	require.False(t, c.AcceptsXML())
	require.False(t, c.AcceptsEventStream())

	c.Request().Header.Set(HeaderAccept, "application/xml")
	require.True(t, c.AcceptsXML())
	require.False(t, c.AcceptsJSON())

	c.Request().Header.Set(HeaderAccept, "text/event-stream")
	require.True(t, c.AcceptsEventStream())
}

// go test -run Test_Ctx_ContentTypeHelpers
func Test_Ctx_ContentTypeHelpers(t *testing.T) {
	t.Parallel()

	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.Empty(t, c.MediaType())
	require.Empty(t, c.Charset())
	require.False(t, c.IsJSON())

	c.Request().Header.Set(HeaderContentType, "application/json; charset=utf-8")
	//nolint:testifylint // This is a MIME type string, not JSON payload.
	require.Equal(t, MIMEApplicationJSON, c.MediaType())
	require.Equal(t, "utf-8", c.Charset())
	require.True(t, c.IsJSON())
	require.False(t, c.IsForm())
	require.False(t, c.IsMultipart())

	c.Request().Header.Set(HeaderContentType, "text/html ; charset=\"UTF-8\"")
	require.Equal(t, MIMETextHTML, c.MediaType())
	require.Equal(t, "UTF-8", c.Charset())

	c.Request().Header.Set(HeaderContentType, MIMEApplicationForm)
	require.Equal(t, MIMEApplicationForm, c.MediaType())
	require.Empty(t, c.Charset())
	require.True(t, c.IsForm())
	require.False(t, c.IsMultipart())

	c.Request().Header.Set(HeaderContentType, MIMEMultipartForm+"; boundary=abc123")
	require.Equal(t, MIMEMultipartForm, c.MediaType())
	require.Empty(t, c.Charset())
	require.True(t, c.IsMultipart())
	require.False(t, c.IsForm())
}

// go test -run Test_Ctx_Charset
func Test_Ctx_Charset(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		contentType string
		expected    string
	}{
		{
			name:        "no_parameters",
			contentType: "text/plain",
			expected:    "",
		},
		{
			name:        "trailing_semicolon",
			contentType: "text/plain;",
			expected:    "",
		},
		{
			name:        "empty_param_before_charset",
			contentType: "text/plain; ; charset=utf-8",
			expected:    "utf-8",
		},
		{
			name:        "charset_with_spaces",
			contentType: "text/plain; charset = utf-8",
			expected:    "utf-8",
		},
		{
			name:        "charset_in_middle",
			contentType: "text/plain; foo=bar; charset=iso-8859-1; baz=qux",
			expected:    "iso-8859-1",
		},
		{
			name:        "charset_quoted",
			contentType: "text/plain; charset=\"utf-8\"; foo=bar",
			expected:    "utf-8",
		},
		{
			name:        "non_charset_only",
			contentType: "text/plain; foo=bar",
			expected:    "",
		},
		{
			name:        "missing_equals",
			contentType: "text/plain; charset",
			expected:    "",
		},
		{
			name:        "empty_charset_value",
			contentType: "text/plain; charset=",
			expected:    "",
		},
		{
			name:        "case_insensitive_charset",
			contentType: "text/plain; chArSet=Shift_JIS",
			expected:    "Shift_JIS",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			app := New()
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().Header.Set(HeaderContentType, testCase.contentType)
			require.Equal(t, testCase.expected, c.Charset())
		})
	}
}

// go test -run Test_Ctx_HeaderHelpers
func Test_Ctx_HeaderHelpers(t *testing.T) {
	t.Parallel()

	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.SetHost("example.com")
	c.Request().SetRequestURI("/search?q=fiber")
	require.Equal(t, "http://example.com/search?q=fiber", c.FullURL())

	c.Request().Header.Set(HeaderUserAgent, "fiber-agent")
	c.Request().Header.Set(HeaderReferer, "https://example.com")
	c.Request().Header.Set(HeaderAcceptLanguage, "en-US,en;q=0.9")
	c.Request().Header.Set(HeaderAcceptEncoding, "gzip, br")
	c.Request().Header.Set("X-Trace-Id", "trace")
	require.True(t, c.HasHeader("X-Trace-Id"))
	require.True(t, c.HasHeader("x-trace-id"))
	require.Equal(t, "fiber-agent", c.UserAgent())
	require.Equal(t, "https://example.com", c.Referer())
	require.Equal(t, "en-US,en;q=0.9", c.AcceptLanguage())
	require.Equal(t, "gzip, br", c.AcceptEncoding())

	c.Request().Header.Set(HeaderXRequestID, "request-id")
	c.Response().Header.Set(HeaderXRequestID, "response-id")
	require.Equal(t, "response-id", c.RequestID())
	c.Response().Header.Del(HeaderXRequestID)
	require.Equal(t, "request-id", c.RequestID())

	c.Request().Header.Del("X-Trace-Id")
	require.False(t, c.HasHeader("X-Trace-Id"))
}

// go test -run Test_Ctx_TypedParsingDefaults
func Test_Ctx_TypedParsingDefaults(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/:id", func(c Ctx) error {
		require.Equal(t, 5, Query[int](c, "count", 1))
		require.Equal(t, 9, Query[int](c, "missing", 9))
		require.Equal(t, 3, Query[int](c, "bad", 3))

		require.Equal(t, 42, Params[int](c, "id", 7))
		require.Equal(t, 7, Params[int](c, "missing", 7))

		require.Equal(t, 11, GetReqHeader[int](c, "X-Limit", 4))
		require.Equal(t, 4, GetReqHeader[int](c, "X-Bad", 4))

		return c.SendStatus(StatusOK)
	})

	req := httptest.NewRequest(MethodGet, "/42?count=5&bad=oops", http.NoBody)
	req.Header.Set("X-Limit", "11")
	req.Header.Set("X-Bad", "oops")
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	defer func() { require.NoError(t, resp.Body.Close()) }()
	require.Equal(t, StatusOK, resp.StatusCode)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Accepts -benchmem -count=4
func Benchmark_Ctx_Accepts(b *testing.B) {
	app := New(Config{
		CBOREncoder: cbor.Marshal,
		CBORDecoder: cbor.Unmarshal,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	acceptHeader := "text/html,application/xhtml+xml,application/xml;q=0.9"
	c.Request().Header.Set("Accept", acceptHeader)
	acceptValues := [][]string{
		{".xml"},
		{"json", "xml"},
		{"application/json", "application/xml"},
	}
	expectedResults := []string{".xml", "xml", "application/xml"}

	for i := range acceptValues {
		b.Run(fmt.Sprintf("run-%#v", acceptValues[i]), func(bb *testing.B) {
			var res string
			bb.ReportAllocs()

			for bb.Loop() {
				res = c.Accepts(acceptValues[i]...)
			}
			require.Equal(bb, expectedResults[i], res)
		})
	}
}

type customCtx struct {
	DefaultCtx
}

func (c *customCtx) Params(key string, defaultValue ...string) string { //revive:disable-line:unused-parameter // We need defaultValue for some cases
	return "prefix_" + c.DefaultCtx.Params(key)
}

// go test -run Test_Ctx_CustomCtx
func Test_Ctx_CustomCtx(t *testing.T) {
	t.Parallel()

	app := NewWithCustomCtx(func(app *App) CustomCtx {
		return &customCtx{
			DefaultCtx: *NewDefaultCtx(app),
		}
	})

	app.Get("/:id", func(c Ctx) error {
		return c.SendString(c.Params("id"))
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v3", &bytes.Buffer{}))
	require.NoError(t, err, "app.Test(req)")
	defer func() { require.NoError(t, resp.Body.Close()) }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "io.ReadAll(resp.Body)")
	require.Len(t, body, len("prefix_v3"))
	require.Equal(t, "prefix_v3", string(body))
	require.Equal(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))
	require.Equal(t, int64(len(body)), resp.ContentLength)
}

func Test_Ctx_CustomCtx_WithMiddleware(t *testing.T) {
	t.Parallel()

	app := NewWithCustomCtx(func(app *App) CustomCtx {
		return &customCtx{
			DefaultCtx: *NewDefaultCtx(app),
		}
	})

	app.Use(func(c Ctx) error {
		_, ok := c.(*customCtx)
		require.True(t, ok)
		return c.Next()
	})

	app.Get("/", func(c Ctx) error {
		custom, ok := c.(*customCtx)
		require.True(t, ok)
		return c.SendString(custom.Params(""))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	defer func() { require.NoError(t, resp.Body.Close()) }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "io.ReadAll(resp.Body)")
	require.Equal(t, "prefix_", string(body))
}

// go test -run Test_Ctx_CustomCtx
func Test_Ctx_CustomCtx_and_Method(t *testing.T) {
	t.Parallel()

	// Create app with custom request methods
	methods := append(DefaultMethods, "JOHN") //nolint:gocritic // We want a new slice here
	app := NewWithCustomCtx(func(app *App) CustomCtx {
		return &customCtx{
			DefaultCtx: *NewDefaultCtx(app),
		}
	}, Config{
		RequestMethods: methods,
	})

	// Add route with custom method
	app.Add([]string{"JOHN"}, "/doe", testEmptyHandler)
	resp, err := app.Test(httptest.NewRequest("JOHN", "/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	defer func() { require.NoError(t, resp.Body.Close()) }()
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "io.ReadAll(resp.Body)")
	require.Empty(t, body)
	require.Empty(t, resp.Header.Get(HeaderContentType))
	require.Equal(t, int64(0), resp.ContentLength)

	// Add a new method
	require.Panics(t, func() {
		app.Add([]string{"JANE"}, "/jane", testEmptyHandler)
	})
}

// go test -run Test_Ctx_Accepts_EmptyAccept
func Test_Ctx_Accepts_EmptyAccept(t *testing.T) {
	t.Parallel()
	app := New(Config{
		CBOREncoder: cbor.Marshal,
		CBORDecoder: cbor.Unmarshal,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.Equal(t, ".forwarded", c.Accepts(".forwarded"))
}

// go test -run Test_Ctx_Accepts_Wildcard
func Test_Ctx_Accepts_Wildcard(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAccept, "*/*;q=0.9")
	require.Equal(t, "html", c.Accepts("html"))
	require.Equal(t, "foo", c.Accepts("foo"))
	require.Equal(t, ".bar", c.Accepts(".bar"))
	c.Request().Header.Set(HeaderAccept, "text/html,application/*;q=0.9")
	require.Equal(t, "xml", c.Accepts("xml"))
}

// go test -run Test_Ctx_Accepts_MultiHeader
func Test_Ctx_Accepts_MultiHeader(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Add(HeaderAccept, "text/plain;q=0.5")
	c.Request().Header.Add(HeaderAccept, "application/json")
	require.Equal(t, "application/json", c.Accepts("text/plain", "application/json"))
}

// go test -run Test_Ctx_AcceptsCharsets
func Test_Ctx_AcceptsCharsets(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAcceptCharset, "utf-8, iso-8859-1;q=0.5")
	require.Equal(t, "utf-8", c.AcceptsCharsets("utf-8"))
}

// go test -run Test_Ctx_AcceptsCharsets_MultiHeader
func Test_Ctx_AcceptsCharsets_MultiHeader(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Add(HeaderAcceptCharset, "utf-8;q=0.1")
	c.Request().Header.Add(HeaderAcceptCharset, "iso-8859-1")
	require.Equal(t, "iso-8859-1", c.AcceptsCharsets("utf-8", "iso-8859-1"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsCharsets -benchmem -count=4
func Benchmark_Ctx_AcceptsCharsets(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.Request().Header.Set("Accept-Charset", "utf-8, iso-8859-1;q=0.5")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.AcceptsCharsets("utf-8")
	}
	require.Equal(b, "utf-8", res)
}

// go test -run Test_Ctx_AcceptsEncodings
func Test_Ctx_AcceptsEncodings(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAcceptEncoding, "deflate, gzip;q=1.0, *;q=0.5")
	require.Equal(t, "gzip", c.AcceptsEncodings("gzip"))
	require.Equal(t, "abc", c.AcceptsEncodings("abc"))
}

// go test -run Test_Ctx_AcceptsEncodings_MultiHeader
func Test_Ctx_AcceptsEncodings_MultiHeader(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Add(HeaderAcceptEncoding, "deflate;q=0.3")
	c.Request().Header.Add(HeaderAcceptEncoding, "gzip")
	require.Equal(t, "gzip", c.AcceptsEncodings("deflate", "gzip"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsEncodings -benchmem -count=4
func Benchmark_Ctx_AcceptsEncodings(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.Request().Header.Set(HeaderAcceptEncoding, "deflate, gzip;q=1.0, *;q=0.5")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.AcceptsEncodings("gzip")
	}
	require.Equal(b, "gzip", res)
}

// go test -run Test_Ctx_AcceptsLanguages
func Test_Ctx_AcceptsLanguages(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAcceptLanguage, "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")
	require.Equal(t, "fr", c.AcceptsLanguages("fr"))
}

// go test -run Test_Ctx_AcceptsLanguages_MultiHeader
func Test_Ctx_AcceptsLanguages_MultiHeader(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Add(HeaderAcceptLanguage, "de;q=0.4")
	c.Request().Header.Add(HeaderAcceptLanguage, "en")
	require.Equal(t, "en", c.AcceptsLanguages("de", "en"))
}

// go test -run Test_Ctx_AcceptsLanguages_BasicFiltering
func Test_Ctx_AcceptsLanguages_BasicFiltering(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAcceptLanguage, "en-US")
	require.Equal(t, "en-US", c.AcceptsLanguages("en", "en-US"))
	require.Empty(t, c.AcceptsLanguages("en"))

	c.Request().Header.Set(HeaderAcceptLanguage, "en-US, fr")
	require.Equal(t, "en-US", c.AcceptsLanguages("de", "en-US", "fr"))

	c.Request().Header.Set(HeaderAcceptLanguage, "en")
	require.Equal(t, "en-US", c.AcceptsLanguages("en-US"))

	c.Request().Header.Set(HeaderAcceptLanguage, "*")
	require.Equal(t, "en", c.AcceptsLanguages("en", "fr"))

	c.Request().Header.Set(HeaderAcceptLanguage, "en_US")
	require.Empty(t, c.AcceptsLanguages("en-US"))

	c.Request().Header.Set(HeaderAcceptLanguage, "en-*")
	require.Empty(t, c.AcceptsLanguages("en-US"))
}

// go test -run Test_Ctx_AcceptsLanguages_CaseInsensitive
func Test_Ctx_AcceptsLanguages_CaseInsensitive(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAcceptLanguage, "EN-us")
	require.Equal(t, "en-US", c.AcceptsLanguages("en-US"))
}

// go test -run Test_Ctx_AcceptsLanguagesExtended
func Test_Ctx_AcceptsLanguagesExtended(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAcceptLanguage, "en-*")
	require.Equal(t, "en-US", c.AcceptsLanguagesExtended("en-US"))

	c.Request().Header.Set(HeaderAcceptLanguage, "*-US")
	require.Equal(t, "en-US", c.AcceptsLanguagesExtended("en-US", "fr-CA"))

	c.Request().Header.Set(HeaderAcceptLanguage, "en-US-*")
	require.Equal(t, "en-US", c.AcceptsLanguagesExtended("en-US"))

	c.Request().Header.Set(HeaderAcceptLanguage, "en")
	require.Equal(t, "en-US", c.AcceptsLanguagesExtended("en-US"))

	c.Request().Header.Set(HeaderAcceptLanguage, "*")
	require.Equal(t, "en-US", c.AcceptsLanguagesExtended("en-US", "fr-CA"))

	c.Request().Header.Set(HeaderAcceptLanguage, "en-US")
	require.Equal(t, "en-US-CA", c.AcceptsLanguagesExtended("en-US-CA"))

	c.Request().Header.Set(HeaderAcceptLanguage, "en-*")
	require.Equal(t, "en-US-CA", c.AcceptsLanguagesExtended("en-US-CA"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsLanguages -benchmem -count=4
func Benchmark_Ctx_AcceptsLanguages(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.Request().Header.Set(HeaderAcceptLanguage, "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.AcceptsLanguages("fr")
	}
	require.Equal(b, "fr", res)
}

// go test -run Test_Ctx_App
func Test_Ctx_App(t *testing.T) {
	t.Parallel()
	app := New()
	app.config.BodyLimit = 1000
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.Equal(t, 1000, c.App().config.BodyLimit)
}

// go test -run Test_Ctx_Append
func Test_Ctx_Append(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Append("X-Test", "Hello")
	c.Append("X-Test", "World")
	c.Append("X-Test", "Hello", "World")
	// similar value in the middle
	c.Append("X2-Test", "World")
	c.Append("X2-Test", "XHello")
	c.Append("X2-Test", "Hello", "World")
	// similar value at the start
	c.Append("X3-Test", "XHello")
	c.Append("X3-Test", "World")
	c.Append("X3-Test", "Hello", "World")
	// try it with multiple similar values
	c.Append("X4-Test", "XHello")
	c.Append("X4-Test", "Hello")
	c.Append("X4-Test", "HelloZ")
	c.Append("X4-Test", "YHello")
	c.Append("X4-Test", "Hello")
	c.Append("X4-Test", "YHello")
	c.Append("X4-Test", "HelloZ")
	c.Append("X4-Test", "XHello")
	// without append value
	c.Append("X-Custom-Header")

	require.Equal(t, "Hello, World", string(c.Response().Header.Peek("X-Test")))
	require.Equal(t, "World, XHello, Hello", string(c.Response().Header.Peek("X2-Test")))
	require.Equal(t, "XHello, World, Hello", string(c.Response().Header.Peek("X3-Test")))
	require.Equal(t, "XHello, Hello, HelloZ, YHello", string(c.Response().Header.Peek("X4-Test")))
	require.Empty(t, string(c.Response().Header.Peek("x-custom-header")))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Append -benchmem -count=4
func Benchmark_Ctx_Append(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	b.ReportAllocs()
	for b.Loop() {
		c.Append("X-Custom-Header", "Hello")
		c.Append("X-Custom-Header", "World")
		c.Append("X-Custom-Header", "Hello")
	}
	require.Equal(b, "Hello, World", app.toString(c.Response().Header.Peek("X-Custom-Header")))
}

// go test -run Test_Ctx_Attachment
func Test_Ctx_Attachment(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// empty
	c.Attachment()
	require.Equal(t, `attachment`, string(c.Response().Header.Peek(HeaderContentDisposition)))
	// real filename
	c.Attachment("./static/img/logo.png")
	require.Equal(t, `attachment; filename="logo.png"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
	require.Equal(t, "image/png", string(c.Response().Header.Peek(HeaderContentType)))
	// filename with spaces
	c.Attachment("report 2024.txt")
	require.Equal(t, `attachment; filename="report+2024.txt"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
	// filename with nested path
	c.Attachment("../docs/archive.tar.gz")
	require.Equal(t, `attachment; filename="archive.tar.gz"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
	// check quoting
	c.Attachment("another document.pdf\"\r\nBla: \"fasel")
	require.Equal(t, `attachment; filename="another+document.pdf%22Bla%3A+%22fasel"`, string(c.Response().Header.Peek(HeaderContentDisposition)))

	c.Attachment("файл.txt")
	header := string(c.Response().Header.Peek(HeaderContentDisposition))
	require.Contains(t, header, `filename="файл.txt"`)
	require.Contains(t, header, `filename*=UTF-8''%D1%84%D0%B0%D0%B9%D0%BB.txt`)
}

// go test -run Test_Ctx_Attachment_SanitizesFilenameControls
func Test_Ctx_Attachment_SanitizesFilenameControls(t *testing.T) {
	t.Parallel()
	app := New()
	testCases := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "base name only",
			filename: "../docs/archive.tar.gz",
			expected: `attachment; filename="archive.tar.gz"`,
		},
		{
			name:     "controls stripped",
			filename: "down\r\nload\t\x00.txt",
			expected: `attachment; filename="download.txt"`,
		},
		{
			name:     "controls stripped without extension",
			filename: "report\r\n\t\x00",
			expected: `attachment; filename="report"`,
		},
		{
			name:     "empty after sanitize",
			filename: "\r\n\t\x00",
			expected: `attachment; filename="download"`,
		},
		{
			name:     "controls stripped in middle",
			filename: "file\rname\n\t\x00.bin",
			expected: `attachment; filename="filename.bin"`,
		},
		{
			name:     "dot fallback",
			filename: ".",
			expected: `attachment; filename="download"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Attachment(tc.filename)
			header := string(c.Response().Header.Peek(HeaderContentDisposition))
			require.Equal(t, tc.expected, header)
			require.NotContains(t, header, "\r")
			require.NotContains(t, header, "\n")
			require.NotContains(t, header, "\t")
			require.NotContains(t, header, "\x00")
		})
	}
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Attachment -benchmem -count=4
func Benchmark_Ctx_Attachment(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	b.ReportAllocs()
	for b.Loop() {
		// example with quote params
		c.Attachment("another document.pdf\"\r\nBla: \"fasel")
	}
	require.Equal(b, `attachment; filename="another+document.pdf%22Bla%3A+%22fasel"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
}

// go test -run Test_Ctx_BaseURL
func Test_Ctx_BaseURL(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().SetRequestURI("http://google.com/test")
	require.Equal(t, "http://google.com", c.BaseURL())
	// Check cache
	require.Equal(t, "http://google.com", c.BaseURL())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_BaseURL -benchmem
func Benchmark_Ctx_BaseURL(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.Request().SetHost("google.com:1337")
	c.Request().URI().SetPath("/haha/oke/lol")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.BaseURL()
	}
	require.Equal(b, "http://google.com:1337", res)
}

// go test -run Test_Ctx_Body
func Test_Ctx_Body(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.Request().SetBody([]byte("john=doe"))
	require.Equal(t, []byte("john=doe"), c.Body())
}

// go test -run Test_Ctx_BodyRaw
func Test_Ctx_BodyRaw(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.Request().SetBodyRaw([]byte("john=doe"))
	require.Equal(t, []byte("john=doe"), c.BodyRaw())
}

// go test -run Test_Ctx_BodyRaw_Immutable
func Test_Ctx_BodyRaw_Immutable(t *testing.T) {
	t.Parallel()
	app := New(Config{Immutable: true})
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.Request().SetBodyRaw([]byte("john=doe"))
	require.Equal(t, []byte("john=doe"), c.BodyRaw())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Body -benchmem -count=4
func Benchmark_Ctx_Body(b *testing.B) {
	const input = "john=doe"

	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.Request().SetBody([]byte(input))
	b.ReportAllocs()
	for b.Loop() {
		_ = c.Body()
	}

	require.Equal(b, []byte(input), c.Body())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_BodyRaw -benchmem -count=4
func Benchmark_Ctx_BodyRaw(b *testing.B) {
	const input = "john=doe"

	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.Request().SetBodyRaw([]byte(input))
	b.ReportAllocs()
	for b.Loop() {
		_ = c.BodyRaw()
	}

	require.Equal(b, []byte(input), c.BodyRaw())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_BodyRaw_Immutable -benchmem -count=4
func Benchmark_Ctx_BodyRaw_Immutable(b *testing.B) {
	const input = "john=doe"

	app := New(Config{Immutable: true})
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.Request().SetBodyRaw([]byte(input))
	b.ReportAllocs()
	for b.Loop() {
		_ = c.BodyRaw()
	}

	require.Equal(b, []byte(input), c.BodyRaw())
}

// go test -run Test_Ctx_Body_Immutable
func Test_Ctx_Body_Immutable(t *testing.T) {
	t.Parallel()
	app := New()
	app.config.Immutable = true
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.Request().SetBody([]byte("john=doe"))
	require.Equal(t, []byte("john=doe"), c.Body())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Body_Immutable -benchmem -count=4
func Benchmark_Ctx_Body_Immutable(b *testing.B) {
	const input = "john=doe"

	app := New()
	app.config.Immutable = true
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.Request().SetBody([]byte(input))
	b.ReportAllocs()

	for b.Loop() {
		_ = c.Body()
	}

	require.Equal(b, []byte(input), c.Body())
}

// go test -run Test_Ctx_Body_With_Compression
func Test_Ctx_Body_With_Compression(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		contentEncoding string
		body            []byte
		expectedBody    []byte
	}{
		{
			name:            "gzip",
			contentEncoding: "gzip",
			body:            []byte("john=doe"),
			expectedBody:    []byte("john=doe"),
		},
		{
			name:            "gzip twice",
			contentEncoding: "gzip, gzip",
			body:            []byte("double"),
			expectedBody:    []byte("double"),
		},
		{
			name:            "unsupported_encoding",
			contentEncoding: "undefined",
			body:            []byte("keeps_ORIGINAL"),
			expectedBody:    []byte("Unsupported Media Type"),
		},
		{
			name:            "compress_not_implemented",
			contentEncoding: "compress",
			body:            []byte("foo"),
			expectedBody:    []byte("Not Implemented"),
		},
		{
			name:            "gzip then unsupported",
			contentEncoding: "gzip, undefined",
			body:            []byte("Go, be gzipped"),
			expectedBody:    []byte("Unsupported Media Type"),
		},
		{
			name:            "invalid_deflate",
			contentEncoding: "gzip,deflate",
			body:            []byte("I'm not correctly compressed"),
			expectedBody:    []byte(zlib.ErrHeader.Error()),
		},
		{
			name:            "identity",
			contentEncoding: "identity",
			body:            []byte("bar"),
			expectedBody:    []byte("bar"),
		},
	}

	for _, testObject := range tests {
		tCase := testObject // Duplicate object to ensure it will be unique across all runs
		t.Run(tCase.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed
			c.Request().Header.Set("Content-Encoding", tCase.contentEncoding)

			encs := strings.SplitSeq(tCase.contentEncoding, ",")
			for enc := range encs {
				enc = utils.TrimSpace(enc)
				if strings.Contains(tCase.name, "invalid_deflate") && enc == StrDeflate {
					continue
				}
				switch enc {
				case "gzip":
					var b bytes.Buffer
					gz := gzip.NewWriter(&b)
					_, err := gz.Write(tCase.body)
					require.NoError(t, err)
					require.NoError(t, gz.Flush())
					require.NoError(t, gz.Close())
					tCase.body = b.Bytes()
				case StrDeflate:
					var b bytes.Buffer
					fl := zlib.NewWriter(&b)
					_, err := fl.Write(tCase.body)
					require.NoError(t, err)
					require.NoError(t, fl.Flush())
					require.NoError(t, fl.Close())
					tCase.body = b.Bytes()
				default:
					// we do nothing and expect the original body to be returned
				}
			}

			c.Request().SetBody(tCase.body)
			body := c.Body()
			require.Equal(t, tCase.expectedBody, body)

			switch {
			case strings.Contains(tCase.name, "unsupported"):
				require.Equal(t, StatusUnsupportedMediaType, c.Response().StatusCode())
			case strings.Contains(tCase.name, "compress_not_implemented"):
				require.Equal(t, StatusNotImplemented, c.Response().StatusCode())
			default:
				require.Equal(t, StatusOK, c.Response().StatusCode())
			}

			// Check if body raw is the same as previous before decompression
			require.Equal(
				t, tCase.body, c.Request().Body(),
				"Body raw must be the same as set before",
			)
		})
	}
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Body_With_Compression -benchmem -count=4
func Benchmark_Ctx_Body_With_Compression(b *testing.B) {
	encodingErr := errors.New("failed to encoding data")

	var (
		compressGzip = func(data []byte) ([]byte, error) {
			var buf bytes.Buffer
			writer := gzip.NewWriter(&buf)
			if _, err := writer.Write(data); err != nil {
				return nil, encodingErr
			}
			if err := writer.Flush(); err != nil {
				return nil, encodingErr
			}
			if err := writer.Close(); err != nil {
				return nil, encodingErr
			}
			return buf.Bytes(), nil
		}
		compressDeflate = func(data []byte) ([]byte, error) {
			var buf bytes.Buffer
			writer := zlib.NewWriter(&buf)
			if _, err := writer.Write(data); err != nil {
				return nil, encodingErr
			}
			if err := writer.Flush(); err != nil {
				return nil, encodingErr
			}
			if err := writer.Close(); err != nil {
				return nil, encodingErr
			}
			return buf.Bytes(), nil
		}
	)
	const input = "john=doe"
	compressionTests := []struct {
		compressWriter  func([]byte) ([]byte, error)
		contentEncoding string
		expectedBody    []byte
	}{
		{
			contentEncoding: "gzip",
			compressWriter:  compressGzip,
			expectedBody:    []byte(input),
		},
		{
			contentEncoding: "gzip,invalid",
			compressWriter:  compressGzip,
			expectedBody:    []byte(ErrUnsupportedMediaType.Error()),
		},
		{
			contentEncoding: StrDeflate,
			compressWriter:  compressDeflate,
			expectedBody:    []byte(input),
		},
		{
			contentEncoding: "gzip,deflate",
			compressWriter: func(data []byte) ([]byte, error) {
				var (
					buf    bytes.Buffer
					writer interface {
						io.WriteCloser
						Flush() error
					}
					err error
				)

				// deflate
				writer = zlib.NewWriter(&buf)
				if _, err = writer.Write(data); err != nil {
					return nil, encodingErr
				}
				if err = writer.Flush(); err != nil {
					return nil, encodingErr
				}
				if err = writer.Close(); err != nil {
					return nil, encodingErr
				}

				data = make([]byte, buf.Len())
				copy(data, buf.Bytes())
				buf.Reset()

				// gzip
				writer = gzip.NewWriter(&buf)
				if _, err = writer.Write(data); err != nil {
					return nil, encodingErr
				}
				if err = writer.Flush(); err != nil {
					return nil, encodingErr
				}
				if err = writer.Close(); err != nil {
					return nil, encodingErr
				}

				return buf.Bytes(), nil
			},
			expectedBody: []byte(zlib.ErrHeader.Error()),
		},
	}

	b.ReportAllocs()
	for _, ct := range compressionTests {
		b.Run(ct.contentEncoding, func(b *testing.B) {
			app := New()
			const input = "john=doe"
			c := app.AcquireCtx(&fasthttp.RequestCtx{})

			c.Request().Header.Set("Content-Encoding", ct.contentEncoding)
			compressedBody, err := ct.compressWriter([]byte(input))
			require.NoError(b, err)

			c.Request().SetBody(compressedBody)
			for b.Loop() {
				_ = c.Body()
			}

			require.Equal(b, ct.expectedBody, c.Body())
		})
	}
}

// go test -run Test_Ctx_Body_With_Compression_Immutable
func Test_Ctx_Body_With_Compression_Immutable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		contentEncoding string
		body            []byte
		expectedBody    []byte
	}{
		{
			name:            "gzip",
			contentEncoding: "gzip",
			body:            []byte("john=doe"),
			expectedBody:    []byte("john=doe"),
		},
		{
			name:            "gzip twice",
			contentEncoding: "gzip, gzip",
			body:            []byte("double"),
			expectedBody:    []byte("double"),
		},
		{
			name:            "unsupported_encoding",
			contentEncoding: "undefined",
			body:            []byte("keeps_ORIGINAL"),
			expectedBody:    []byte("Unsupported Media Type"),
		},
		{
			name:            "compress_not_implemented",
			contentEncoding: "compress",
			body:            []byte("foo"),
			expectedBody:    []byte("Not Implemented"),
		},
		{
			name:            "gzip then unsupported",
			contentEncoding: "gzip, undefined",
			body:            []byte("Go, be gzipped"),
			expectedBody:    []byte("Unsupported Media Type"),
		},
		{
			name:            "invalid_deflate",
			contentEncoding: "gzip,deflate",
			body:            []byte("I'm not correctly compressed"),
			expectedBody:    []byte(zlib.ErrHeader.Error()),
		},
		{
			name:            "identity",
			contentEncoding: "identity",
			body:            []byte("bar"),
			expectedBody:    []byte("bar"),
		},
	}

	for _, testObject := range tests {
		tCase := testObject // Duplicate object to ensure it will be unique across all runs
		t.Run(tCase.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.config.Immutable = true
			c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed
			c.Request().Header.Set("Content-Encoding", tCase.contentEncoding)

			encs := strings.SplitSeq(tCase.contentEncoding, ",")
			for enc := range encs {
				enc = utils.TrimSpace(enc)
				if strings.Contains(tCase.name, "invalid_deflate") && enc == StrDeflate {
					continue
				}
				switch enc {
				case "gzip":
					var b bytes.Buffer
					gz := gzip.NewWriter(&b)
					_, err := gz.Write(tCase.body)
					require.NoError(t, err)
					require.NoError(t, gz.Flush())
					require.NoError(t, gz.Close())
					tCase.body = b.Bytes()
				case StrDeflate:
					var b bytes.Buffer
					fl := zlib.NewWriter(&b)
					_, err := fl.Write(tCase.body)
					require.NoError(t, err)
					require.NoError(t, fl.Flush())
					require.NoError(t, fl.Close())
					tCase.body = b.Bytes()
				default:
					// we do nothing and expect the original body to be returned
				}
			}

			c.Request().SetBody(tCase.body)
			body := c.Body()
			require.Equal(t, tCase.expectedBody, body)

			switch {
			case strings.Contains(tCase.name, "unsupported"):
				require.Equal(t, StatusUnsupportedMediaType, c.Response().StatusCode())
			case strings.Contains(tCase.name, "compress_not_implemented"):
				require.Equal(t, StatusNotImplemented, c.Response().StatusCode())
			default:
				require.Equal(t, StatusOK, c.Response().StatusCode())
			}

			// Check if body raw is the same as previous before decompression
			require.Equal(
				t, tCase.body, c.Request().Body(),
				"Body raw must be the same as set before",
			)
		})
	}
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Body_With_Compression_Immutable -benchmem -count=4
func Benchmark_Ctx_Body_With_Compression_Immutable(b *testing.B) {
	encodingErr := errors.New("failed to encoding data")

	var (
		compressGzip = func(data []byte) ([]byte, error) {
			var buf bytes.Buffer
			writer := gzip.NewWriter(&buf)
			if _, err := writer.Write(data); err != nil {
				return nil, encodingErr
			}
			if err := writer.Flush(); err != nil {
				return nil, encodingErr
			}
			if err := writer.Close(); err != nil {
				return nil, encodingErr
			}
			return buf.Bytes(), nil
		}
		compressDeflate = func(data []byte) ([]byte, error) {
			var buf bytes.Buffer
			writer := zlib.NewWriter(&buf)
			if _, err := writer.Write(data); err != nil {
				return nil, encodingErr
			}
			if err := writer.Flush(); err != nil {
				return nil, encodingErr
			}
			if err := writer.Close(); err != nil {
				return nil, encodingErr
			}
			return buf.Bytes(), nil
		}
	)
	const input = "john=doe"
	compressionTests := []struct {
		compressWriter  func([]byte) ([]byte, error)
		contentEncoding string
		expectedBody    []byte
	}{
		{
			contentEncoding: "gzip",
			compressWriter:  compressGzip,
			expectedBody:    []byte(input),
		},
		{
			contentEncoding: "gzip,invalid",
			compressWriter:  compressGzip,
			expectedBody:    []byte(ErrUnsupportedMediaType.Error()),
		},
		{
			contentEncoding: StrDeflate,
			compressWriter:  compressDeflate,
			expectedBody:    []byte(input),
		},
		{
			contentEncoding: "gzip,deflate",
			compressWriter: func(data []byte) ([]byte, error) {
				var (
					buf    bytes.Buffer
					writer interface {
						io.WriteCloser
						Flush() error
					}
					err error
				)

				// deflate
				writer = zlib.NewWriter(&buf)
				if _, err = writer.Write(data); err != nil {
					return nil, encodingErr
				}
				if err = writer.Flush(); err != nil {
					return nil, encodingErr
				}
				if err = writer.Close(); err != nil {
					return nil, encodingErr
				}

				data = make([]byte, buf.Len())
				copy(data, buf.Bytes())
				buf.Reset()

				// gzip
				writer = gzip.NewWriter(&buf)
				if _, err = writer.Write(data); err != nil {
					return nil, encodingErr
				}
				if err = writer.Flush(); err != nil {
					return nil, encodingErr
				}
				if err = writer.Close(); err != nil {
					return nil, encodingErr
				}

				return buf.Bytes(), nil
			},
			expectedBody: []byte(zlib.ErrHeader.Error()),
		},
	}

	b.ReportAllocs()
	for _, ct := range compressionTests {
		b.Run(ct.contentEncoding, func(b *testing.B) {
			app := New()
			app.config.Immutable = true
			const input = "john=doe"
			c := app.AcquireCtx(&fasthttp.RequestCtx{})

			c.Request().Header.Set("Content-Encoding", ct.contentEncoding)
			compressedBody, err := ct.compressWriter([]byte(input))
			require.NoError(b, err)

			c.Request().SetBody(compressedBody)
			for b.Loop() {
				_ = c.Body()
			}

			require.Equal(b, ct.expectedBody, c.Body())
		})
	}
}

// go test -run Test_Ctx_RequestCtx
func Test_Ctx_RequestCtx(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.Equal(t, "*fasthttp.RequestCtx", fmt.Sprintf("%T", c.RequestCtx()))
}

// go test -run Test_Ctx_Cookie
func Test_Ctx_Cookie(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	expire := time.Now().Add(24 * time.Hour)
	var dst []byte
	dst = expire.In(time.UTC).AppendFormat(dst, time.RFC1123)
	httpdate := strings.ReplaceAll(string(dst), "UTC", "GMT")
	cookie := &Cookie{
		Name:    "username",
		Value:   "john",
		Expires: expire,
		// SameSite: CookieSameSiteStrictMode, // default is "lax"
	}
	c.Res().Cookie(cookie)
	expect := "username=john; expires=" + httpdate + "; path=/; SameSite=Lax"
	require.Equal(t, expect, c.Res().Get(HeaderSetCookie))

	expect = "username=john; expires=" + httpdate + "; path=/"
	cookie.SameSite = CookieSameSiteDisabled
	c.Res().Cookie(cookie)
	require.Equal(t, expect, c.Res().Get(HeaderSetCookie))

	expect = "username=john; expires=" + httpdate + "; path=/; SameSite=Strict"
	cookie.SameSite = CookieSameSiteStrictMode
	c.Res().Cookie(cookie)
	require.Equal(t, expect, c.Res().Get(HeaderSetCookie))

	expect = "username=john; expires=" + httpdate + "; path=/; secure; SameSite=None"
	cookie.Secure = true
	cookie.SameSite = CookieSameSiteNoneMode
	c.Res().Cookie(cookie)
	require.Equal(t, expect, c.Res().Get(HeaderSetCookie))

	expect = "username=john; path=/; secure; SameSite=None"
	// should remove expires and max-age headers
	cookie.SessionOnly = true
	cookie.Expires = expire
	cookie.MaxAge = 10000
	c.Res().Cookie(cookie)
	require.Equal(t, expect, c.Res().Get(HeaderSetCookie))

	expect = "username=john; path=/; secure; SameSite=None"
	// should remove expires and max-age headers when no expire and no MaxAge (default time)
	cookie.SessionOnly = false
	cookie.Expires = time.Time{}
	cookie.MaxAge = 0
	c.Res().Cookie(cookie)
	require.Equal(t, expect, c.Res().Get(HeaderSetCookie))

	expect = "username=john; path=/; secure; SameSite=None; Partitioned"
	cookie.Partitioned = true
	c.Res().Cookie(cookie)
	require.Equal(t, expect, c.Res().Get(HeaderSetCookie))
}

// go test -run Test_Ctx_Cookie_PartitionedSecure
func Test_Ctx_Cookie_PartitionedSecure(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	ck := &Cookie{
		Name:        "ps",
		Value:       "v",
		Secure:      true,
		SameSite:    CookieSameSiteNoneMode,
		Partitioned: true,
	}
	c.Res().Cookie(ck)
	require.Equal(t, "ps=v; path=/; secure; SameSite=None; Partitioned", c.Res().Get(HeaderSetCookie))
}

// go test -run Test_Ctx_Cookie_Invalid
func Test_Ctx_Cookie_Invalid(t *testing.T) {
	t.Parallel()
	app := New()

	cases := []*Cookie{
		{Name: "", Value: "a"},                                                        // empty name
		{Name: "foo bar", Value: "a"},                                                 // invalid char in name
		{Name: "n", Value: "bad\nval"},                                                // invalid value byte
		{Name: "d", Value: "b", Domain: "in valid"},                                   // invalid domain spaces
		{Name: "d", Value: "b", Domain: "example..com"},                               // invalid domain dots
		{Name: "i", Value: "b", Domain: "2001:db8::1"},                                // ipv6 not allowed
		{Name: "p", Value: "b", Path: "\x00"},                                         // invalid path byte
		{Name: "e", Value: "b", Expires: time.Date(1500, 1, 1, 0, 0, 0, 0, time.UTC)}, // invalid expires
		// Note: Partitioned without Secure is auto-fixed (Secure=true set automatically per CHIPS spec)
	}

	for _, invalid := range cases {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Res().Cookie(invalid)
		require.Empty(t, c.Res().Get(HeaderSetCookie))
		c.Response().Header.Reset()
		app.ReleaseCtx(c)
	}
}

// go test -run Test_Ctx_Cookie_DefaultPath
func Test_Ctx_Cookie_DefaultPath(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	ck := &Cookie{
		Name:  "p",
		Value: "v",
		// Path intentionally empty to verify defaulting
	}

	c.Res().Cookie(ck)
	require.Equal(t,
		"p=v; path=/; SameSite=Lax",
		c.Res().Get(HeaderSetCookie),
	)
}

// go test -run Test_Ctx_Cookie_MaxAgeOnly
func Test_Ctx_Cookie_MaxAgeOnly(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	ck := &Cookie{
		Name:   "ttl",
		Value:  "v",
		MaxAge: 3600,
	}
	c.Res().Cookie(ck)

	require.Equal(t,
		"ttl=v; max-age=3600; path=/; SameSite=Lax",
		c.Res().Get(HeaderSetCookie),
	)
}

// go test -run Test_Ctx_Cookie_StrictPartitioned
func Test_Ctx_Cookie_StrictPartitioned(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	ck := &Cookie{
		Name:        "sp",
		Value:       "v",
		Secure:      true,
		SameSite:    CookieSameSiteStrictMode,
		Partitioned: true,
	}
	c.Res().Cookie(ck)

	require.Equal(t,
		"sp=v; path=/; secure; SameSite=Strict; Partitioned",
		c.Res().Get(HeaderSetCookie),
	)
}

// go test -run Test_Ctx_Cookie_SameSite_CaseInsensitive
func Test_Ctx_Cookie_SameSite_CaseInsensitive(t *testing.T) {
	t.Parallel()
	app := New()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Test case-insensitive Strict
		{"Strict lowercase", "strict", "SameSite=Strict"},
		{"Strict uppercase", "STRICT", "SameSite=Strict"},
		{"Strict mixed case", "StRiCt", "SameSite=Strict"},
		{"Strict proper case", "Strict", "SameSite=Strict"},

		// Test case-insensitive Lax
		{"Lax lowercase", "lax", "SameSite=Lax"},
		{"Lax uppercase", "LAX", "SameSite=Lax"},
		{"Lax mixed case", "LaX", "SameSite=Lax"},
		{"Lax proper case", "Lax", "SameSite=Lax"},

		// Test case-insensitive None
		{"None lowercase", "none", "SameSite=None"},
		{"None uppercase", "NONE", "SameSite=None"},
		{"None mixed case", "NoNe", "SameSite=None"},
		{"None proper case", "None", "SameSite=None"},

		// Test case-insensitive disabled
		{"Disabled lowercase", "disabled", ""},
		{"Disabled uppercase", "DISABLED", ""},
		{"Disabled mixed case", "DiSaBlEd", ""},
		{"Disabled proper case", "disabled", ""},

		// Test invalid values default to Lax
		{"Invalid value", "invalid", "SameSite=Lax"},
		{"Empty value", "", "SameSite=Lax"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(c)

			// Reset response
			c.Response().Reset()

			cookie := &Cookie{
				Name:     "test",
				Value:    "value",
				SameSite: tc.input,
			}
			c.Res().Cookie(cookie)

			setCookieHeader := c.Res().Get(HeaderSetCookie)
			if tc.expected == "" {
				// For disabled, SameSite should not appear in the header
				require.NotContains(t, setCookieHeader, "SameSite")
			} else {
				// For all other cases, the expected SameSite should appear
				require.Contains(t, setCookieHeader, tc.expected)
			}
		})
	}
}

// go test -run Test_Ctx_Cookie_SameSite_None_Secure
func Test_Ctx_Cookie_SameSite_None_Secure(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		cookie           *Cookie
		expectedInHeader string
		shouldBeSecure   bool
	}{
		{
			name: "Empty value",
			cookie: &Cookie{
				Name:     "test",
				Value:    "value",
				SameSite: "",
			},
			expectedInHeader: "SameSite=Lax",
			shouldBeSecure:   false,
		},
		{
			name: "None uppercase",
			cookie: &Cookie{
				Name:     "test",
				Value:    "value",
				SameSite: "None",
			},
			expectedInHeader: "SameSite=None",
			shouldBeSecure:   true,
		},
		{
			name: "None lowercase",
			cookie: &Cookie{
				Name:     "test",
				Value:    "value",
				SameSite: "none",
			},
			expectedInHeader: "SameSite=None",
			shouldBeSecure:   true,
		},
		{
			name: "Lax proper case",
			cookie: &Cookie{
				Name:     "test",
				Value:    "value",
				SameSite: "Lax",
			},
			expectedInHeader: "SameSite=Lax",
			shouldBeSecure:   false,
		},
		{
			name: "Strict uppercase",
			cookie: &Cookie{
				Name:     "test",
				Value:    "value",
				SameSite: "STRICT",
			},
			expectedInHeader: "SameSite=Strict",
			shouldBeSecure:   false,
		},
		{
			name: "Disabled Secure",
			cookie: &Cookie{
				Name:     "test",
				Value:    "value",
				SameSite: "none",
				Secure:   false,
			},
			expectedInHeader: "SameSite=None",
			shouldBeSecure:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(ctx)

			ctx.Cookie(tc.cookie)

			cookie := string(ctx.Response().Header.PeekCookie(tc.cookie.Name))
			require.Contains(t, cookie, tc.expectedInHeader)

			if tc.shouldBeSecure {
				require.Contains(t, cookie, "secure")
			} else {
				require.NotContains(t, cookie, "secure")
			}
		})
	}
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Cookie -benchmem -count=4
func Benchmark_Ctx_Cookie(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	b.ReportAllocs()
	for b.Loop() {
		c.Cookie(&Cookie{
			Name:  "John",
			Value: "Doe",
		})
	}
	require.Equal(b, "John=Doe; path=/; SameSite=Lax", app.toString(c.Response().Header.Peek("Set-Cookie")))
}

// go test -run Test_Ctx_Cookies
func Test_Ctx_Cookies(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Cookie", "john=doe")
	require.Equal(t, "doe", c.Req().Cookies("john"))
	require.Equal(t, "default", c.Req().Cookies("unknown", "default"))
}

// go test -run Test_Ctx_Format
func Test_Ctx_Format(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// set `accepted` to whatever media type was chosen by Format
	var accepted string
	formatHandlers := func(types ...string) []ResFmt {
		fmts := []ResFmt{}
		for _, t := range types {
			typ := utils.CopyString(t)
			fmts = append(fmts, ResFmt{MediaType: typ, Handler: func(_ Ctx) error {
				accepted = typ
				return nil
			}})
		}
		return fmts
	}

	c.Request().Header.Set(HeaderAccept, `text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7`)
	err := c.Res().Format(formatHandlers("application/xhtml+xml", "application/xml", "foo/bar")...)
	require.Equal(t, "application/xhtml+xml", accepted)
	require.Equal(t, "application/xhtml+xml", c.GetRespHeader(HeaderContentType))
	require.Equal(t, "application/xhtml+xml", c.Res().Get(HeaderContentType))
	require.NoError(t, err)
	require.NotEqual(t, StatusNotAcceptable, c.Response().StatusCode())

	err = c.Res().Format(formatHandlers("foo/bar;a=b")...)
	require.Equal(t, "foo/bar;a=b", accepted)
	require.Equal(t, "foo/bar;a=b", c.GetRespHeader(HeaderContentType))
	require.Equal(t, "foo/bar;a=b", c.Res().Get(HeaderContentType))
	require.NoError(t, err)
	require.NotEqual(t, StatusNotAcceptable, c.Response().StatusCode())

	myError := errors.New("this is an error")
	err = c.Format(ResFmt{MediaType: "text/html", Handler: func(_ Ctx) error { return myError }})
	require.ErrorIs(t, err, myError)

	c.Request().Header.Set(HeaderAccept, "application/json")
	err = c.Format(ResFmt{MediaType: "text/html", Handler: func(c Ctx) error { return c.SendStatus(StatusOK) }})
	require.Equal(t, StatusNotAcceptable, c.Response().StatusCode())
	require.NoError(t, err)

	c.Request().Header.Set(HeaderAccept, MIMEApplicationMsgPack)
	err = c.Format(ResFmt{MediaType: "text/html", Handler: func(c Ctx) error { return c.SendStatus(StatusOK) }})
	require.Equal(t, StatusNotAcceptable, c.Response().StatusCode())
	require.NoError(t, err)

	err = c.Format(formatHandlers("text/html", "default")...)
	require.Equal(t, "default", accepted)
	require.Equal(t, "text/html", c.GetRespHeader(HeaderContentType))
	require.Equal(t, "text/html", c.Res().Get(HeaderContentType))
	require.NoError(t, err)

	err = c.Format()
	require.ErrorIs(t, err, ErrNoHandlers)
}

func Benchmark_Ctx_Format(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderAccept, "application/json,text/plain; format=flowed; q=0.9")

	fail := func(_ Ctx) error {
		require.FailNow(b, "Wrong type chosen")
		return errors.New("Wrong type chosen")
	}
	ok := func(_ Ctx) error {
		return nil
	}

	var err error
	b.Run("with arg allocation", func(b *testing.B) {
		for b.Loop() {
			err = c.Format(
				ResFmt{MediaType: "application/xml", Handler: fail},
				ResFmt{MediaType: "text/html", Handler: fail},
				ResFmt{MediaType: "text/plain;format=fixed", Handler: fail},
				ResFmt{MediaType: "text/plain;format=flowed", Handler: ok},
			)
		}
		require.NoError(b, err)
	})

	b.Run("pre-allocated args", func(b *testing.B) {
		offers := []ResFmt{
			{MediaType: "application/xml", Handler: fail},
			{MediaType: "text/html", Handler: fail},
			{MediaType: "text/plain;format=fixed", Handler: fail},
			{MediaType: "text/plain;format=flowed", Handler: ok},
		}
		for b.Loop() {
			err = c.Format(offers...)
		}
		require.NoError(b, err)
	})

	c.Request().Header.Set("Accept", "text/plain")
	b.Run("text/plain", func(b *testing.B) {
		offers := []ResFmt{
			{MediaType: "application/xml", Handler: fail},
			{MediaType: "text/plain", Handler: ok},
		}
		for b.Loop() {
			err = c.Format(offers...)
		}
		require.NoError(b, err)
	})

	c.Request().Header.Set("Accept", "json")
	b.Run("json", func(b *testing.B) {
		offers := []ResFmt{
			{MediaType: "xml", Handler: fail},
			{MediaType: "html", Handler: fail},
			{MediaType: "json", Handler: ok},
		}
		for b.Loop() {
			err = c.Format(offers...)
		}
		require.NoError(b, err)
	})

	c.Request().Header.Set("Accept", MIMEApplicationMsgPack)
	b.Run("msgpack", func(b *testing.B) {
		offers := []ResFmt{
			{MediaType: "xml", Handler: fail},
			{MediaType: "html", Handler: fail},
			{MediaType: MIMEApplicationMsgPack, Handler: ok},
		}
		for b.Loop() {
			err = c.Format(offers...)
		}
		require.NoError(b, err)
	})
}

// go test -run Test_Ctx_AutoFormat
func Test_Ctx_AutoFormat(t *testing.T) {
	t.Parallel()
	app := New(Config{
		MsgPackEncoder: msgpack.Marshal,
		MsgPackDecoder: msgpack.Unmarshal,
		CBOREncoder:    cbor.Marshal,
		CBORDecoder:    cbor.Unmarshal,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAccept, MIMETextPlain)
	err := c.AutoFormat([]byte("Hello, World!"))
	require.NoError(t, err)
	require.Equal(t, MIMETextPlainCharsetUTF8, c.GetRespHeader(HeaderContentType))
	require.Equal(t, "Hello, World!", string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMETextHTML)
	err = c.Res().AutoFormat("Hello, World!")
	require.NoError(t, err)
	require.Equal(t, MIMETextHTMLCharsetUTF8, c.GetRespHeader(HeaderContentType))
	require.Equal(t, "<p>Hello, World!</p>", string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMEApplicationJSON)
	err = c.AutoFormat("Hello, World!")
	require.NoError(t, err)
	require.Equal(t, MIMEApplicationJSONCharsetUTF8, c.GetRespHeader(HeaderContentType)) //nolint:testifylint // this is comparing content-type headers, not JSON content
	require.Equal(t, `"Hello, World!"`, string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMEApplicationMsgPack)
	err = c.AutoFormat("Hello, World!")
	require.NoError(t, err)
	require.Equal(t, MIMEApplicationMsgPack, c.GetRespHeader(HeaderContentType))
	require.Equal(t, []byte{
		0xad, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c,
		0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21,
	}, c.Response().Body())

	c.Request().Header.Set(HeaderAccept, MIMEApplicationCBOR)
	err = c.AutoFormat("Hello, World!")
	require.NoError(t, err)
	require.Equal(t, MIMEApplicationCBOR, c.GetRespHeader(HeaderContentType))
	require.Equal(t, `6d48656c6c6f2c20576f726c6421`, hex.EncodeToString(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMETextPlain)
	err = c.Res().AutoFormat(complex(1, 1))
	require.NoError(t, err)
	require.Equal(t, MIMETextPlainCharsetUTF8, c.GetRespHeader(HeaderContentType))
	require.Equal(t, "(1+1i)", string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMEApplicationXML)
	err = c.AutoFormat("Hello, World!")
	require.NoError(t, err)
	require.Equal(t, MIMEApplicationXMLCharsetUTF8, c.GetRespHeader(HeaderContentType))
	require.Equal(t, `<string>Hello, World!</string>`, string(c.Response().Body()))

	err = c.AutoFormat(complex(1, 1))
	require.Error(t, err)

	c.Request().Header.Set(HeaderAccept, MIMETextPlain)
	err = c.AutoFormat(Map{})
	require.NoError(t, err)
	require.Equal(t, MIMETextPlainCharsetUTF8, c.GetRespHeader(HeaderContentType))
	require.Equal(t, "map[]", string(c.Response().Body()))

	type broken string
	c.Request().Header.Set(HeaderAccept, "broken/accept")
	require.NoError(t, err)
	err = c.AutoFormat(broken("Hello, World!"))
	require.NoError(t, err)
	require.Equal(t, MIMETextPlainCharsetUTF8, c.GetRespHeader(HeaderContentType))
	require.Equal(t, `Hello, World!`, string(c.Response().Body()))
}

func Test_Ctx_AutoFormat_Struct(t *testing.T) {
	t.Parallel()
	app := New(Config{
		MsgPackEncoder: msgpack.Marshal,
		MsgPackDecoder: msgpack.Unmarshal,
		CBOREncoder:    cbor.Marshal,
		CBORDecoder:    cbor.Unmarshal,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	type Message struct {
		Sender     string `xml:"sender,attr"`
		Recipients []string
		Urgency    int `xml:"urgency,attr"`
	}
	data := Message{
		Recipients: []string{"Alice", "Bob"},
		Sender:     "Carol",
		Urgency:    3,
	}

	c.Request().Header.Set(HeaderAccept, MIMEApplicationJSON)
	err := c.AutoFormat(data)
	require.NoError(t, err)
	require.Equal(t, MIMEApplicationJSONCharsetUTF8, c.GetRespHeader(HeaderContentType)) //nolint:testifylint // this is comparing content-type headers, not JSON content
	require.JSONEq(t,
		`{"Sender":"Carol","Recipients":["Alice","Bob"],"Urgency":3}`,
		string(c.Response().Body()),
	)

	c.Request().Header.Set(HeaderAccept, MIMEApplicationMsgPack)
	err = c.AutoFormat(data)
	require.NoError(t, err)

	require.Equal(t, MIMEApplicationMsgPack, c.GetRespHeader(HeaderContentType))
	require.Equal(t, []byte{
		// {"Sender":"Carol","Recipients":["Alice","Bob"],"Urgency":3}
		0x83, 0xa6, 0x53, 0x65, 0x6e, 0x64, 0x65, 0x72, 0xa5, 0x43, 0x61,
		0x72, 0x6f, 0x6c, 0xaa, 0x52, 0x65, 0x63, 0x69, 0x70, 0x69, 0x65,
		0x6e, 0x74, 0x73, 0x92, 0xa5, 0x41, 0x6c, 0x69, 0x63, 0x65, 0xa3,
		0x42, 0x6f, 0x62, 0xa7, 0x55, 0x72, 0x67, 0x65, 0x6e, 0x63, 0x79,
		0x3,
	},
		c.Response().Body())

	c.Request().Header.Set(HeaderAccept, MIMEApplicationCBOR)
	err = c.AutoFormat(data)
	require.NoError(t, err)
	require.Equal(t, MIMEApplicationCBOR, c.GetRespHeader(HeaderContentType))
	require.Equal(t, "a36653656e646572654361726f6c6a526563697069656e74738265416c69636563426f6267557267656e637903",
		hex.EncodeToString(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMEApplicationXML)
	err = c.AutoFormat(data)
	require.NoError(t, err)
	require.Equal(t, MIMEApplicationXMLCharsetUTF8, c.GetRespHeader(HeaderContentType))
	require.Equal(t,
		`<Message sender="Carol" urgency="3"><Recipients>Alice</Recipients><Recipients>Bob</Recipients></Message>`,
		string(c.Response().Body()),
	)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AutoFormat -benchmem -count=4
func Benchmark_Ctx_AutoFormat(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Accept", "text/plain")
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.AutoFormat("Hello, World!")
	}
	require.NoError(b, err)
	require.Equal(b, `Hello, World!`, string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AutoFormat_HTML -benchmem -count=4
func Benchmark_Ctx_AutoFormat_HTML(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Accept", "text/html")
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.AutoFormat("Hello, World!")
	}
	require.NoError(b, err)
	require.Equal(b, "<p>Hello, World!</p>", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AutoFormat_JSON -benchmem -count=4
func Benchmark_Ctx_AutoFormat_JSON(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Accept", "application/json")
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.AutoFormat("Hello, World!")
	}
	require.NoError(b, err)
	require.Equal(b, `"Hello, World!"`, string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AutoFormat_MsgPack -benchmem -count=4
func Benchmark_Ctx_AutoFormat_MsgPack(b *testing.B) {
	app := New(
		Config{
			MsgPackEncoder: msgpack.Marshal,
			MsgPackDecoder: msgpack.Unmarshal,
		},
	)
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Accept", MIMEApplicationMsgPack)
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.AutoFormat("Hello, World!")
	}
	require.NoError(b, err)
	require.Equal(b, "\xadHello, World!", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AutoFormat_XML -benchmem -count=4
func Benchmark_Ctx_AutoFormat_XML(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Accept", "application/xml")
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.AutoFormat("Hello, World!")
	}
	require.NoError(b, err)
	require.Equal(b, `<string>Hello, World!</string>`, string(c.Response().Body()))
}

// go test -run Test_Ctx_FormFile
func Test_Ctx_FormFile(t *testing.T) {
	// TODO: We should clean this up
	t.Parallel()
	app := New()

	app.Post("/test", func(c Ctx) error {
		fh, err := c.FormFile("file")
		require.NoError(t, err)
		require.Equal(t, "test", fh.Filename)

		f, err := fh.Open()
		require.NoError(t, err)
		defer func() {
			require.NoError(t, f.Close())
		}()

		b := new(bytes.Buffer)
		_, err = io.Copy(b, f)
		require.NoError(t, err)
		require.Equal(t, "hello world", b.String())
		return nil
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	ioWriter, err := writer.CreateFormFile("file", "test")
	require.NoError(t, err)

	_, err = ioWriter.Write([]byte("hello world"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set(HeaderContentType, writer.FormDataContentType())
	req.Header.Set(HeaderContentLength, strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	defer func() { require.NoError(t, resp.Body.Close()) }()
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "io.ReadAll(resp.Body)")
	require.Empty(t, respBody)
	require.Empty(t, resp.Header.Get(HeaderContentType))
	require.Equal(t, int64(0), resp.ContentLength)
}

// go test -run Test_Ctx_FormValue
func Test_Ctx_FormValue(t *testing.T) {
	t.Parallel()
	app := New()

	app.Post("/test", func(c Ctx) error {
		require.Equal(t, "john", c.FormValue("name"))
		return nil
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("name", "john"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+writer.Boundary())
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	defer func() { require.NoError(t, resp.Body.Close()) }()
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "io.ReadAll(resp.Body)")
	require.Empty(t, respBody)
	require.Empty(t, resp.Header.Get(HeaderContentType))
	require.Equal(t, int64(0), resp.ContentLength)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Fresh_StaleEtag -benchmem -count=4
func Benchmark_Ctx_Fresh_StaleEtag(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	for b.Loop() {
		c.Request().Header.Set(HeaderIfNoneMatch, `"a", "b", "c", "d"`)
		c.Request().Header.Set(HeaderCacheControl, "c")
		c.Fresh()

		c.Request().Header.Set(HeaderIfNoneMatch, `"a", "b", "c", "d"`)
		c.Request().Header.Set(HeaderCacheControl, "e")
		c.Fresh()
	}
}

// go test -run Test_Ctx_Fresh
func Test_Ctx_Fresh(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.False(t, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, "*")
	c.Request().Header.Set(HeaderCacheControl, "no-cache")
	require.False(t, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, "*")
	c.Request().Header.Set(HeaderCacheControl, ",no-cache,")
	require.False(t, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, "*")
	c.Request().Header.Set(HeaderCacheControl, "aa,no-cache,")
	require.False(t, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, "*")
	c.Request().Header.Set(HeaderCacheControl, ",no-cache,bb")
	require.False(t, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, `"675af34563dc-tr34"`)
	c.Request().Header.Set(HeaderCacheControl, "public")
	require.False(t, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, `"a", "b"`)
	c.Response().Header.Set(HeaderETag, `"c"`)
	require.False(t, c.Fresh())

	c.Response().Header.Set(HeaderETag, `"a"`)
	require.True(t, c.Fresh())

	c.Request().Header.Set(HeaderIfModifiedSince, "xxWed, 21 Oct 2015 07:28:00 GMT")
	c.Response().Header.Set(HeaderLastModified, "xxWed, 21 Oct 2015 07:28:00 GMT")
	require.False(t, c.Fresh())

	c.Response().Header.Set(HeaderLastModified, "Wed, 21 Oct 2015 07:28:00 GMT")
	require.False(t, c.Fresh())

	c.Request().Header.Set(HeaderIfModifiedSince, "Wed, 21 Oct 2015 07:28:00 GMT")
	require.True(t, c.Fresh())

	c.Request().Header.Set(HeaderIfModifiedSince, "Wed, 21 Oct 2015 07:27:59 GMT")
	c.Response().Header.Set(HeaderLastModified, "Wed, 21 Oct 2015 07:28:00 GMT")
	require.False(t, c.Fresh())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Fresh_WithNoCache -benchmem -count=4
func Benchmark_Ctx_Fresh_WithNoCache(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderIfNoneMatch, "*")
	c.Request().Header.Set(HeaderCacheControl, "no-cache")
	for b.Loop() {
		c.Fresh()
	}
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Fresh_LastModified -benchmem -count=4
func Benchmark_Ctx_Fresh_LastModified(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Response().Header.Set(HeaderLastModified, "Wed, 21 Oct 2015 07:28:00 GMT")
	c.Request().Header.Set(HeaderIfModifiedSince, "Wed, 21 Oct 2015 07:28:00 GMT")
	for b.Loop() {
		c.Fresh()
	}
}

// go test -run Test_Ctx_Binders -v
func Test_Ctx_Binders(t *testing.T) {
	t.Parallel()
	// setup
	app := New(Config{
		EnableSplittingOnParsers: true,
	})

	type TestEmbeddedStruct struct {
		Names []string `query:"names"`
	}

	type TestStruct struct {
		Name            string
		NameWithDefault string `json:"name2" xml:"Name2" form:"name2" cookie:"name2" query:"name2" uri:"name2" header:"Name2"`
		TestEmbeddedStruct
		Class            int
		ClassWithDefault int `json:"class2" xml:"Class2" form:"class2" cookie:"class2" query:"class2" uri:"class2" header:"Class2"`
	}

	withValues := func(t *testing.T, actionFn func(c Ctx, testStruct *TestStruct) error) {
		t.Helper()

		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		testStruct := new(TestStruct)

		require.NoError(t, actionFn(c, testStruct))
		require.Equal(t, "foo", testStruct.Name)
		require.Equal(t, 111, testStruct.Class)
		require.Equal(t, "bar", testStruct.NameWithDefault)
		require.Equal(t, 222, testStruct.ClassWithDefault)
		require.Equal(t, []string{"foo", "bar", "test"}, testStruct.Names)
	}

	t.Run("Body:xml", func(t *testing.T) {
		t.Parallel()
		withValues(t, func(c Ctx, testStruct *TestStruct) error {
			c.Request().Header.SetContentType(MIMEApplicationXML)
			c.Request().SetBody([]byte(`<TestStruct><Name>foo</Name><Class>111</Class><Name2>bar</Name2><Class2>222</Class2><Names>foo</Names><Names>bar</Names><Names>test</Names></TestStruct>`))
			return c.Bind().Body(testStruct)
		})
	})
	t.Run("Body:form", func(t *testing.T) {
		t.Parallel()
		withValues(t, func(c Ctx, testStruct *TestStruct) error {
			c.Request().Header.SetContentType(MIMEApplicationForm)
			c.Request().SetBody([]byte(`name=foo&class=111&name2=bar&class2=222&names=foo,bar,test`))
			return c.Bind().Body(testStruct)
		})
	})
	t.Run("BodyParser:json", func(t *testing.T) {
		t.Parallel()
		withValues(t, func(c Ctx, testStruct *TestStruct) error {
			c.Request().Header.SetContentType(MIMEApplicationJSON)
			c.Request().SetBody([]byte(`{"name":"foo","class":111,"name2":"bar","class2":222,"names":["foo","bar","test"]}`))
			return c.Bind().Body(testStruct)
		})
	})
	t.Run("Body:multiform", func(t *testing.T) {
		t.Parallel()
		withValues(t, func(c Ctx, testStruct *TestStruct) error {
			body := []byte("--b\r\nContent-Disposition: form-data; name=\"name\"\r\n\r\nfoo\r\n--b\r\nContent-Disposition: form-data; name=\"class\"\r\n\r\n111\r\n--b\r\nContent-Disposition: form-data; name=\"name2\"\r\n\r\nbar\r\n--b\r\nContent-Disposition: form-data; name=\"class2\"\r\n\r\n222\r\n--b\r\nContent-Disposition: form-data; name=\"names\"\r\n\r\nfoo\r\n--b\r\nContent-Disposition: form-data; name=\"names\"\r\n\r\nbar\r\n--b\r\nContent-Disposition: form-data; name=\"names\"\r\n\r\ntest\r\n--b--")
			c.Request().SetBody(body)
			c.Request().Header.SetContentType(MIMEMultipartForm + `;boundary="b"`)
			c.Request().Header.SetContentLength(len(body))
			return c.Bind().Body(testStruct)
		})
	})
	t.Run("Cookie", func(t *testing.T) {
		t.Parallel()
		withValues(t, func(c Ctx, testStruct *TestStruct) error {
			c.Request().Header.Set("Cookie", "name=foo;name2=bar;class=111;class2=222;names=foo,bar,test")
			return c.Bind().Cookie(testStruct)
		})
	})
	t.Run("Query", func(t *testing.T) {
		t.Parallel()
		withValues(t, func(c Ctx, testStruct *TestStruct) error {
			c.Request().URI().SetQueryString("name=foo&name2=bar&class=111&class2=222&names=foo,bar,test")
			return c.Bind().Query(testStruct)
		})
	})

	t.Run("URI", func(t *testing.T) {
		t.Parallel()

		c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed
		defer app.ReleaseCtx(c)

		c.route = &Route{Params: []string{"name", "name2", "class", "class2"}}
		c.values = [maxParams]string{"foo", "bar", "111", "222"}

		testStruct := new(TestStruct)

		require.NoError(t, c.Bind().URI(testStruct))
		require.Equal(t, "foo", testStruct.Name)
		require.Equal(t, 111, testStruct.Class)
		require.Equal(t, "bar", testStruct.NameWithDefault)
		require.Equal(t, 222, testStruct.ClassWithDefault)
		require.Nil(t, testStruct.Names)
	})

	t.Run("ReqHeader", func(t *testing.T) {
		t.Parallel()
		withValues(t, func(c Ctx, testStruct *TestStruct) error {
			c.Request().Header.Add("name", "foo")
			c.Request().Header.Add("name2", "bar")
			c.Request().Header.Add("class", "111")
			c.Request().Header.Add("class2", "222")
			c.Request().Header.Add("names", "foo,bar,test")
			return c.Bind().Header(testStruct)
		})
	})
}

// go test -run Test_Ctx_Get
func Test_Ctx_Get(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAcceptCharset, "utf-8, iso-8859-1;q=0.5")
	c.Request().Header.Set(HeaderReferer, "Monster")
	require.Equal(t, "utf-8, iso-8859-1;q=0.5", c.Get(HeaderAcceptCharset))
	require.Equal(t, "Monster", c.Get(HeaderReferer))
	require.Equal(t, "default", c.Get("unknown", "default"))
}

// go test -run Test_Ctx_GetReqHeader
func Test_Ctx_GetReqHeader(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("foo", "bar")
	c.Request().Header.Set("id", "123")
	require.Equal(t, 123, GetReqHeader[int](c, "id"))
	require.Equal(t, "bar", GetReqHeader[string](c, "foo"))
}

// go test -run Test_Ctx_Host
func Test_Ctx_Host(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().SetRequestURI("http://google.com/test")
	require.Equal(t, "google.com", c.Host())
}

// go test -run Test_Ctx_Host_UntrustedProxy
func Test_Ctx_Host_UntrustedProxy(t *testing.T) {
	t.Parallel()
	// Don't trust any proxy
	{
		app := New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{}}})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		require.Equal(t, "google.com", c.Host())
		app.ReleaseCtx(c)
	}
	// Trust to specific proxy list
	{
		app := New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.8.0.0", "0.8.0.1"}}})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		require.Equal(t, "google.com", c.Host())
		app.ReleaseCtx(c)
	}
}

// go test -run Test_Ctx_Host_TrustedProxy
func Test_Ctx_Host_TrustedProxy(t *testing.T) {
	t.Parallel()
	{
		app := New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.0.0.0", "0.8.0.1"}}})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		require.Equal(t, "google1.com", c.Host())
		app.ReleaseCtx(c)
	}
	t.Run("TrimWhitespaceFromForwardedHost", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name          string
			forwardedHost string
			expectedHost  string
		}{
			{
				name:          "leading whitespace with comma",
				forwardedHost: " example.com, proxy1",
				expectedHost:  "example.com",
			},
			{
				name:          "trailing whitespace with comma",
				forwardedHost: "example.com , proxy1",
				expectedHost:  "example.com",
			},
			{
				name:          "leading and trailing whitespace with comma",
				forwardedHost: "  example.com  , proxy1",
				expectedHost:  "example.com",
			},
			{
				name:          "no whitespace with comma",
				forwardedHost: "example.com, proxy1",
				expectedHost:  "example.com",
			},
			{
				name:          "single value with whitespace",
				forwardedHost: "  example.com  ",
				expectedHost:  "example.com",
			},
			{
				name:          "leading comma",
				forwardedHost: ",example.com",
				expectedHost:  "",
			},
		}

		app := New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.0.0.0", "0.8.0.1"}}})

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				c := app.AcquireCtx(&fasthttp.RequestCtx{})
				defer app.ReleaseCtx(c)
				c.Request().SetRequestURI("http://google.com/test")
				c.Request().Header.Set(HeaderXForwardedHost, tc.forwardedHost)
				require.Equal(t, tc.expectedHost, c.Host())
			})
		}
	})
}

// go test -run Test_Ctx_Host_TrustedProxyRange
func Test_Ctx_Host_TrustedProxyRange(t *testing.T) {
	t.Parallel()

	app := New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.0.0.0/30"}}})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com/test")
	c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
	require.Equal(t, "google1.com", c.Host())
	app.ReleaseCtx(c)
}

// go test -run Test_Ctx_Host_UntrustedProxyRange
func Test_Ctx_Host_UntrustedProxyRange(t *testing.T) {
	t.Parallel()

	app := New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{"1.0.0.0/30"}}})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com/test")
	c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
	require.Equal(t, "google.com", c.Host())
	app.ReleaseCtx(c)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Host -benchmem -count=4
func Benchmark_Ctx_Host(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com/test")
	var host string
	b.ReportAllocs()
	for b.Loop() {
		host = c.Host()
	}
	require.Equal(b, "google.com", host)
}

// go test -run Test_Ctx_IsProxyTrusted
func Test_Ctx_IsProxyTrusted(t *testing.T) {
	t.Parallel()

	{
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		require.False(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			TrustProxy: false,
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		require.False(t, c.IsProxyTrusted())
	}

	{
		app := New(Config{
			TrustProxy: true,
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		require.False(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{},
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		require.False(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"127.0.0.1"},
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		require.False(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"127.0.0.1/8"},
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		require.False(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"0.0.0.0"},
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		require.True(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"0.0.0.1/31"},
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		require.True(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"0.0.0.1/31junk"},
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		require.False(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Private: true,
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		require.False(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Loopback: true,
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		require.False(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				LinkLocal: true,
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		require.False(t, c.IsProxyTrusted())
	}
}

// go test -run Test_Ctx_Hostname
func Test_Ctx_Hostname(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().SetRequestURI("http://google.com/test")
	require.Equal(t, "google.com", c.Hostname())

	c.Request().SetRequestURI("http://google.com:8080/test")
	require.Equal(t, "google.com", c.Hostname())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Hostname -benchmem -count=4
func Benchmark_Ctx_Hostname(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com:8080/test")
	var hostname string
	b.ReportAllocs()
	for b.Loop() {
		hostname = c.Hostname()
	}
	// Trust to specific proxy list
	{
		app := New(Config{
			TrustProxy:       true,
			TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.8.0.0", "0.8.0.1"}},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		require.Equal(b, "google.com", hostname)
		app.ReleaseCtx(c)
	}
}

// go test -run Test_Ctx_Hostname_Trusted
func Test_Ctx_Hostname_TrustedProxy(t *testing.T) {
	t.Parallel()
	{
		app := New(Config{
			TrustProxy:       true,
			TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.0.0.0", "0.8.0.1"}},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		require.Equal(t, "google1.com", c.Hostname())
		app.ReleaseCtx(c)
	}
}

// go test -run Test_Ctx_Hostname_Trusted_Multiple
func Test_Ctx_Hostname_TrustedProxy_Multiple(t *testing.T) {
	t.Parallel()
	{
		app := New(Config{
			TrustProxy:       true,
			TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.0.0.0", "0.8.0.1"}},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com, google2.com")
		require.Equal(t, "google1.com", c.Hostname())
		app.ReleaseCtx(c)
	}
}

// go test -run Test_Ctx_Hostname_UntrustedProxyRange
func Test_Ctx_Hostname_TrustedProxyRange(t *testing.T) {
	t.Parallel()

	app := New(Config{
		TrustProxy:       true,
		TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.0.0.0/30"}},
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com/test")
	c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
	require.Equal(t, "google1.com", c.Hostname())
	app.ReleaseCtx(c)
}

// go test -run Test_Ctx_Hostname_UntrustedProxyRange
func Test_Ctx_Hostname_UntrustedProxyRange(t *testing.T) {
	t.Parallel()

	app := New(Config{
		TrustProxy:       true,
		TrustProxyConfig: TrustProxyConfig{Proxies: []string{"1.0.0.0/30"}},
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com/test")
	c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
	require.Equal(t, "google.com", c.Hostname())
	app.ReleaseCtx(c)
}

// go test -run Test_Ctx_Port
func Test_Ctx_Port(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.Equal(t, "0", c.Port())
}

// go test -run Test_Ctx_Port_RemoteAddrVariants
func Test_Ctx_Port_RemoteAddrVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		remote net.Addr
		want   string
	}{
		{
			name: "tcp",
			remote: &net.TCPAddr{
				IP:   net.IPv4(127, 0, 0, 1),
				Port: 8080,
			},
			want: "8080",
		},
		{
			name:   "unix",
			remote: &net.UnixAddr{Name: "/tmp/fiber.sock", Net: "unix"},
			want:   "",
		},
		{
			name:   "default-remote",
			remote: nil,
			want:   "0",
		},
		{
			name:   "string-host-port",
			remote: testNetAddr{network: "tcp", address: "192.0.2.1:443"},
			want:   "443",
		},
		{
			name:   "string-missing-port",
			remote: testNetAddr{network: "tcp", address: "192.0.2.1"},
			want:   "",
		},
		{
			name:   "string-ipv6-port",
			remote: testNetAddr{network: "tcp", address: "[2001:db8::1]:8443"},
			want:   "8443",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
			defaultCtx, ok := ctx.(*DefaultCtx)
			require.True(t, ok)
			defaultCtx.fasthttp.SetRemoteAddr(test.remote)
			require.Equal(t, test.want, ctx.Port())
			app.ReleaseCtx(ctx)
		})
	}
}

// go test -run Test_Ctx_PortInHandler
func Test_Ctx_PortInHandler(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/port", func(c Ctx) error {
		return c.SendString(c.Port())
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/port", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "0", string(body))
}

// go test -run Test_Ctx_IP
func Test_Ctx_IP(t *testing.T) {
	t.Parallel()

	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// default behavior will return the remote IP from the stack
	require.Equal(t, "0.0.0.0", c.IP())

	// X-Forwarded-For is set, but it is ignored because proxyHeader is not set
	c.Request().Header.Set(HeaderXForwardedFor, "0.0.0.1")
	require.Equal(t, "0.0.0.0", c.IP())
}

// go test -run Test_Ctx_IP_ProxyHeader
func Test_Ctx_IP_ProxyHeader(t *testing.T) {
	t.Parallel()

	// make sure that the same behavior exists for different proxy header names
	proxyHeaderNames := []string{"Real-Ip", HeaderXForwardedFor}

	for _, proxyHeaderName := range proxyHeaderNames {
		app := New(Config{
			ProxyHeader: proxyHeaderName,
			TrustProxy:  true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"0.0.0.0"},
			},
		})
		fastCtx := &fasthttp.RequestCtx{}
		fastCtx.SetRemoteAddr(net.Addr(&net.TCPAddr{IP: net.ParseIP("0.0.0.0")}))
		c := app.AcquireCtx(fastCtx)

		c.Request().Header.Set(proxyHeaderName, "0.0.0.1")
		require.Equal(t, "0.0.0.1", c.IP())

		// without IP validation we return the full string
		c.Request().Header.Set(proxyHeaderName, "0.0.0.1, 0.0.0.2")
		require.Equal(t, "0.0.0.1, 0.0.0.2", c.IP())

		// without IP validation we return invalid IPs
		c.Request().Header.Set(proxyHeaderName, "invalid, 0.0.0.2, 0.0.0.3")
		require.Equal(t, "invalid, 0.0.0.2, 0.0.0.3", c.IP())

		// when proxy header is enabled but the value is empty, without IP validation we return an empty string
		c.Request().Header.Set(proxyHeaderName, "")
		require.Empty(t, c.IP())

		// without IP validation we return an invalid IP
		c.Request().Header.Set(proxyHeaderName, "not-valid-ip")
		require.Equal(t, "not-valid-ip", c.IP())
	}
}

// go test -run Test_Ctx_IP_ProxyHeader
func Test_Ctx_IP_ProxyHeader_With_IP_Validation(t *testing.T) {
	t.Parallel()

	// make sure that the same behavior exists for different proxy header names
	proxyHeaderNames := []string{"Real-Ip", HeaderXForwardedFor}

	for _, proxyHeaderName := range proxyHeaderNames {
		app := New(Config{
			EnableIPValidation: true,
			ProxyHeader:        proxyHeaderName,
			TrustProxy:         true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"0.0.0.0"},
			},
		})
		fastCtx := &fasthttp.RequestCtx{}
		fastCtx.SetRemoteAddr(net.Addr(&net.TCPAddr{IP: net.ParseIP("0.0.0.0")}))
		c := app.AcquireCtx(fastCtx)

		// when proxy header & validation is enabled and the value is a valid IP, we return it
		c.Request().Header.Set(proxyHeaderName, "0.0.0.1")
		require.Equal(t, "0.0.0.1", c.IP())

		// when proxy header & validation is enabled and the value is a list of IPs, we return the first valid IP
		c.Request().Header.Set(proxyHeaderName, "0.0.0.1, 0.0.0.2")
		require.Equal(t, "0.0.0.1", c.IP())

		c.Request().Header.Set(proxyHeaderName, "invalid, 0.0.0.2, 0.0.0.3")
		require.Equal(t, "0.0.0.2", c.IP())

		// when proxy header & validation is enabled but the value is empty, we will ignore the header
		c.Request().Header.Set(proxyHeaderName, "")
		require.Equal(t, "0.0.0.0", c.IP())

		// when proxy header & validation is enabled but the value is not an IP, we will ignore the header
		// and return the IP of the caller
		c.Request().Header.Set(proxyHeaderName, "not-valid-ip")
		require.Equal(t, "0.0.0.0", c.IP())
	}
}

// go test -run Test_Ctx_IP_UntrustedProxy
func Test_Ctx_IP_UntrustedProxy(t *testing.T) {
	t.Parallel()
	app := New(Config{
		TrustProxy:       true,
		TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.8.0.1"}},
		ProxyHeader:      HeaderXForwardedFor,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderXForwardedFor, "0.0.0.1")
	require.Equal(t, "0.0.0.0", c.IP())
}

// go test -run Test_Ctx_IP_TrustedProxy
func Test_Ctx_IP_TrustedProxy(t *testing.T) {
	t.Parallel()
	app := New(Config{
		TrustProxy:       true,
		TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.0.0.0"}},
		ProxyHeader:      HeaderXForwardedFor,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderXForwardedFor, "0.0.0.1")
	require.Equal(t, "0.0.0.1", c.IP())
}

func Test_Ctx_ProxyTrust_UnixRemoteAddr(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not supported on windows in this test")
	}

	t.Run("unix_socket_enabled", func(t *testing.T) {
		t.Parallel()
		parts := strings.SplitN(runCtxProxyTrustUnixRemoteAddrCase(t, true), "|", 2)
		require.Len(t, parts, 2)
		require.Equal(t, "true", parts[0])
		require.Equal(t, "1.1.1.1", parts[1])
	})

	t.Run("unix_socket_disabled", func(t *testing.T) {
		t.Parallel()
		parts := strings.SplitN(runCtxProxyTrustUnixRemoteAddrCase(t, false), "|", 2)
		require.Len(t, parts, 2)
		require.Equal(t, "false", parts[0])
		require.Equal(t, "0.0.0.0", parts[1])
	})
}

func runCtxProxyTrustUnixRemoteAddrCase(t *testing.T, unixSocket bool) string {
	t.Helper()

	app := New(Config{
		TrustProxy: true,
		TrustProxyConfig: TrustProxyConfig{
			UnixSocket: unixSocket,
		},
		ProxyHeader: HeaderXForwardedFor,
	})
	app.Get("/ip", func(c Ctx) error {
		return c.SendString(fmt.Sprintf("%t|%s", c.IsProxyTrusted(), c.IP()))
	})

	tmp, err := os.MkdirTemp(os.TempDir(), "fiber-ctx-unix")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(tmp)) })
	sock := filepath.Join(tmp, "fiber.sock")

	result := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		time.Sleep(1000 * time.Millisecond)

		client := &fasthttp.HostClient{
			Addr: sock,
			Dial: func(addr string) (net.Conn, error) {
				return net.Dial(NetworkUnix, addr)
			},
		}

		req := &fasthttp.Request{}
		resp := &fasthttp.Response{}
		req.SetRequestURI("http://fiber/ip")
		req.Header.Set(HeaderXForwardedFor, "1.1.1.1")

		if err = client.Do(req, resp); err != nil {
			result <- "" // Ensure result channel always receives a value
			errCh <- errors.Join(err, app.Shutdown())
			return
		}

		result <- string(resp.Body())
		errCh <- app.Shutdown()
	}()

	require.NoError(t, app.Listen(sock, ListenConfig{
		DisableStartupMessage: true,
		ListenerNetwork:       NetworkUnix,
		UnixSocketFileMode:    0o660,
	}))
	require.NoError(t, <-errCh)

	return <-result
}

// go test -run Test_Ctx_IPs  -parallel
func Test_Ctx_IPs(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// normal happy path test case
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, 127.0.0.2, 127.0.0.3")
	require.Equal(t, []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}, c.IPs())

	// inconsistent space formatting
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1,127.0.0.2  ,127.0.0.3")
	require.Equal(t, []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}, c.IPs())

	// invalid IPs are allowed to be returned
	c.Request().Header.Set(HeaderXForwardedFor, "invalid, 127.0.0.1, 127.0.0.2")
	require.Equal(t, []string{"invalid", "127.0.0.1", "127.0.0.2"}, c.IPs())
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, invalid, 127.0.0.2")
	require.Equal(t, []string{"127.0.0.1", "invalid", "127.0.0.2"}, c.IPs())

	// ensure that the ordering of IPs in the header is maintained
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.3, 127.0.0.1, 127.0.0.2")
	require.Equal(t, []string{"127.0.0.3", "127.0.0.1", "127.0.0.2"}, c.IPs())

	// ensure for IPv6
	c.Request().Header.Set(HeaderXForwardedFor, "9396:9549:b4f7:8ed0:4791:1330:8c06:e62d, invalid, 2345:0425:2CA1::0567:5673:23b5")
	require.Equal(t, []string{"9396:9549:b4f7:8ed0:4791:1330:8c06:e62d", "invalid", "2345:0425:2CA1::0567:5673:23b5"}, c.IPs())

	// empty header
	c.Request().Header.Set(HeaderXForwardedFor, "")
	require.Empty(t, c.IPs())

	// missing header
	c.Request()
	require.Empty(t, c.IPs())
}

func Test_Ctx_IPs_With_IP_Validation(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableIPValidation: true})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// normal happy path test case
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, 127.0.0.2, 127.0.0.3")
	require.Equal(t, []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}, c.IPs())

	// inconsistent space formatting
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1,127.0.0.2  ,127.0.0.3")
	require.Equal(t, []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}, c.IPs())

	// invalid IPs are in the header
	c.Request().Header.Set(HeaderXForwardedFor, "invalid, 127.0.0.1, 127.0.0.2")
	require.Equal(t, []string{"127.0.0.1", "127.0.0.2"}, c.IPs())
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, invalid, 127.0.0.2")
	require.Equal(t, []string{"127.0.0.1", "127.0.0.2"}, c.IPs())

	// ensure that the ordering of IPs in the header is maintained
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.3, 127.0.0.1, 127.0.0.2")
	require.Equal(t, []string{"127.0.0.3", "127.0.0.1", "127.0.0.2"}, c.IPs())

	// ensure for IPv6
	c.Request().Header.Set(HeaderXForwardedFor, "f037:825e:eadb:1b7b:1667:6f0a:5356:f604, invalid, 9396:9549:b4f7:8ed0:4791:1330:8c06:e62d")
	require.Equal(t, []string{"f037:825e:eadb:1b7b:1667:6f0a:5356:f604", "9396:9549:b4f7:8ed0:4791:1330:8c06:e62d"}, c.IPs())

	// empty header
	c.Request().Header.Set(HeaderXForwardedFor, "")
	require.Empty(t, c.IPs())

	// missing header
	c.Request()
	require.Empty(t, c.IPs())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_IPs -benchmem -count=4
func Benchmark_Ctx_IPs(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, invalid, 127.0.0.1")
	var res []string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IPs()
	}
	require.Equal(b, []string{"127.0.0.1", "invalid", "127.0.0.1"}, res)
}

func Benchmark_Ctx_IPs_v6(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderXForwardedFor, "f037:825e:eadb:1b7b:1667:6f0a:5356:f604, invalid, 2345:0425:2CA1::0567:5673:23b5")
	var res []string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IPs()
	}
	require.Equal(b, []string{"f037:825e:eadb:1b7b:1667:6f0a:5356:f604", "invalid", "2345:0425:2CA1::0567:5673:23b5"}, res)
}

func Benchmark_Ctx_IPs_With_IP_Validation(b *testing.B) {
	app := New(Config{EnableIPValidation: true})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, invalid, 127.0.0.1")
	var res []string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IPs()
	}
	require.Equal(b, []string{"127.0.0.1", "127.0.0.1"}, res)
}

func Benchmark_Ctx_IPs_v6_With_IP_Validation(b *testing.B) {
	app := New(Config{EnableIPValidation: true})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderXForwardedFor, "2345:0425:2CA1:0000:0000:0567:5673:23b5, invalid, 2345:0425:2CA1::0567:5673:23b5")
	var res []string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IPs()
	}
	require.Equal(b, []string{"2345:0425:2CA1:0000:0000:0567:5673:23b5", "2345:0425:2CA1::0567:5673:23b5"}, res)
}

func Benchmark_Ctx_IP_With_ProxyHeader(b *testing.B) {
	app := New(Config{
		ProxyHeader: HeaderXForwardedFor,
		TrustProxy:  true,
		TrustProxyConfig: TrustProxyConfig{
			Loopback: true,
		},
	})
	fastCtx := &fasthttp.RequestCtx{}
	fastCtx.SetRemoteAddr(net.Addr(&net.TCPAddr{IP: net.ParseIP("127.0.0.1")}))
	c := app.AcquireCtx(fastCtx)
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IP()
	}
	require.Equal(b, "127.0.0.1", res)
}

func Benchmark_Ctx_IP_With_ProxyHeader_and_IP_Validation(b *testing.B) {
	app := New(Config{
		ProxyHeader: HeaderXForwardedFor,
		TrustProxy:  true,
		TrustProxyConfig: TrustProxyConfig{
			Loopback: true,
		},
		EnableIPValidation: true,
	})
	fastCtx := &fasthttp.RequestCtx{}
	fastCtx.SetRemoteAddr(net.Addr(&net.TCPAddr{IP: net.ParseIP("127.0.0.1")}))
	c := app.AcquireCtx(fastCtx)
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IP()
	}
	require.Equal(b, "127.0.0.1", res)
}

func Benchmark_Ctx_IP(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request()
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IP()
	}
	require.Equal(b, "0.0.0.0", res)
}

// go test -run Test_Ctx_Is
func Test_Ctx_Is(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderContentType, MIMETextHTML+"; boundary=something")
	require.True(t, c.Is(".html"))
	require.True(t, c.Is("html"))
	require.False(t, c.Is("json"))
	require.False(t, c.Is(".json"))
	require.False(t, c.Is(""))
	require.False(t, c.Is(".foooo"))

	c.Request().Header.Set(HeaderContentType, MIMEApplicationJSONCharsetUTF8)
	require.False(t, c.Is("html"))
	require.True(t, c.Is("json"))
	require.True(t, c.Is(".json"))

	c.Request().Header.Set(HeaderContentType, " application/json;charset=UTF-8")
	require.False(t, c.Is("html"))
	require.True(t, c.Is("json"))
	require.True(t, c.Is(".json"))

	c.Request().Header.Set(HeaderContentType, MIMEApplicationXMLCharsetUTF8)
	require.False(t, c.Is("html"))
	require.True(t, c.Is("xml"))
	require.True(t, c.Is(".xml"))

	c.Request().Header.Set(HeaderContentType, MIMETextPlain)
	require.False(t, c.Is("html"))
	require.True(t, c.Is("txt"))
	require.True(t, c.Is(".txt"))

	// case-insensitive and trimmed
	c.Request().Header.Set(HeaderContentType, "APPLICATION/JSON; charset=utf-8")
	require.True(t, c.Is("json"))
	require.True(t, c.Is(".json"))

	// mismatched subtype should not match
	c.Request().Header.Set(HeaderContentType, "application/json+xml")
	require.False(t, c.Is("json"))
	require.False(t, c.Is(".json"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Is -benchmem -count=4
func Benchmark_Ctx_Is(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderContentType, MIMEApplicationJSON)
	var res bool
	b.ReportAllocs()
	for b.Loop() {
		_ = c.Is(".json")
		res = c.Is("json")
	}
	require.True(b, res)
}

// go test -run Test_Ctx_Locals
func Test_Ctx_Locals(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error {
		c.Locals("john", "doe")
		return c.Next()
	})
	app.Get("/test", func(c Ctx) error {
		require.Equal(t, "doe", c.Locals("john"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Deadline
func Test_Ctx_Deadline(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error {
		return c.Next()
	})
	app.Get("/test", func(c Ctx) error {
		deadline, ok := c.Deadline()
		require.Equal(t, time.Time{}, deadline)
		require.False(t, ok)
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Done
func Test_Ctx_Done(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error {
		return c.Next()
	})
	app.Get("/test", func(c Ctx) error {
		var nilChan <-chan struct{}
		require.Equal(t, nilChan, c.Done())
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Err
func Test_Ctx_Err(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error {
		return c.Next()
	})
	app.Get("/test", func(c Ctx) error {
		require.NoError(t, c.Err())
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Value
func Test_Ctx_Value(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error {
		c.Locals("john", "doe")
		return c.Next()
	})
	app.Get("/test", func(c Ctx) error {
		require.Equal(t, "doe", c.Value("john"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Value_AfterRelease
func Test_Ctx_Value_AfterRelease(t *testing.T) {
	t.Parallel()
	app := New()
	var ctx Ctx
	app.Get("/test", func(c Ctx) error {
		ctx = c
		c.Locals("test", "value")
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// After the handler completes, the context is released and fasthttp is nil
	// Value should return nil instead of panicking
	require.NotPanics(t, func() {
		val := ctx.Value("test")
		require.Nil(t, val)
	})
}

// go test -run Test_Ctx_Value_InGoroutine
func Test_Ctx_Value_InGoroutine(t *testing.T) {
	t.Parallel()
	app := New()
	done := make(chan bool, 1)   // Buffered to prevent goroutine leak
	errCh := make(chan error, 1) // Channel to communicate errors from goroutine

	// Use a synchronization point to avoid race detector complaints
	// while still testing the defensive nil behavior
	start := make(chan struct{})

	app.Get("/test", func(c Ctx) error {
		c.Locals("test", "value")

		// Simulate a goroutine that uses the context (like minio.GetObject)
		go func() {
			// Wait for handler to complete and context to be released
			<-start

			defer func() {
				if r := recover(); r != nil {
					errCh <- fmt.Errorf("panic in goroutine: %v", r)
					return
				}
				done <- true
			}()

			// This simulates what happens when minio or other libraries
			// use the fiber.Ctx as a context.Context in a goroutine
			// The Value method should not panic even if fasthttp is nil
			val := c.Value("test")
			// The value might be nil if the context was released
			_ = val
		}()

		return nil
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// Signal goroutine to proceed - context has been released after app.Test returns
	// since the handler (and its deferred ReleaseCtx) has completed
	close(start)

	// Wait for goroutine to complete with timeout
	select {
	case <-done:
		// Success - goroutine completed without panic
	case err := <-errCh:
		t.Fatalf("error from goroutine: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("test timed out waiting for goroutine")
	}
}

// go test -run Test_Ctx_Context
func Test_Ctx_Context(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	t.Run("Nil_Context", func(t *testing.T) {
		t.Parallel()
		ctx := c.Context()
		require.Equal(t, ctx, context.Background())
	})

	t.Run("ValueContext", func(t *testing.T) {
		t.Parallel()
		var testKey testContextKey
		testValue := "Test Value"
		ctx := context.WithValue(context.Background(), testKey, testValue)
		require.Equal(t, testValue, ctx.Value(testKey))
	})
}

func Test_Ctx_AccessAfterHandlerPanics(t *testing.T) {
	t.Parallel()
	app := New()
	var ctx Ctx
	app.Get("/test", func(c Ctx) error {
		ctx = c
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	require.Panics(t, func() {
		ctx.Locals("foo")
	})
}

func Test_Ctx_Context_AfterHandlerPanics(t *testing.T) {
	t.Parallel()
	app := New()
	var ctx Ctx
	app.Get("/test", func(c Ctx) error {
		ctx = c
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	// After the fix, Context() returns context.Background() instead of panicking
	require.NotPanics(t, func() {
		c := ctx.Context()
		require.NotNil(t, c)
		require.Equal(t, context.Background(), c)
	})
}

// go test -run Test_Ctx_Request_Response_AfterRelease
func Test_Ctx_Request_Response_AfterRelease(t *testing.T) {
	t.Parallel()
	app := New()
	var ctx Ctx
	app.Get("/test", func(c Ctx) error {
		ctx = c
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// After the handler completes and context is released,
	// Request() and Response() should return nil instead of panicking
	require.NotPanics(t, func() {
		req := ctx.Request()
		require.Nil(t, req)

		res := ctx.Response()
		require.Nil(t, res)
	})
}

// go test -run Test_Ctx_SetContext
func Test_Ctx_SetContext(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	var testKey testContextKey
	testValue := "Test Value"
	ctx := context.WithValue(context.Background(), testKey, testValue)
	c.SetContext(ctx)
	require.Equal(t, testValue, c.Context().Value(testKey))
}

type contextHelperTestKey struct{}

func Test_Ctx_StoreInContext_Config(t *testing.T) {
	t.Parallel()

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()

		app := New()
		raw := &fasthttp.RequestCtx{}
		c := app.AcquireCtx(raw)
		defer app.ReleaseCtx(c)

		StoreInContext(c, contextHelperTestKey{}, "locals-only")

		value, ok := c.Locals(contextHelperTestKey{}).(string)
		require.True(t, ok)
		require.Equal(t, "locals-only", value)
		require.Nil(t, c.Context().Value(contextHelperTestKey{}))
	})

	t.Run("enabled", func(t *testing.T) {
		t.Parallel()

		app := New(Config{PassLocalsToContext: true})
		raw := &fasthttp.RequestCtx{}
		c := app.AcquireCtx(raw)
		defer app.ReleaseCtx(c)

		StoreInContext(c, contextHelperTestKey{}, "both")

		value, ok := c.Locals(contextHelperTestKey{}).(string)
		require.True(t, ok)
		require.Equal(t, "both", value)

		contextValue, ok := c.Context().Value(contextHelperTestKey{}).(string)
		require.True(t, ok)
		require.Equal(t, "both", contextValue)
	})
}

func Test_Ctx_ValueFromContext_Config(t *testing.T) {
	t.Parallel()

	t.Run("fiber ctx disabled reads locals", func(t *testing.T) {
		t.Parallel()

		app := New()
		raw := &fasthttp.RequestCtx{}
		c := app.AcquireCtx(raw)
		defer app.ReleaseCtx(c)

		c.Locals(contextHelperTestKey{}, "locals")
		c.SetContext(context.WithValue(context.Background(), contextHelperTestKey{}, "context"))

		value, ok := ValueFromContext[string](c, contextHelperTestKey{})
		require.True(t, ok)
		require.Equal(t, "locals", value)
	})

	t.Run("fiber ctx enabled prefers context", func(t *testing.T) {
		t.Parallel()

		app := New(Config{PassLocalsToContext: true})
		raw := &fasthttp.RequestCtx{}
		c := app.AcquireCtx(raw)
		defer app.ReleaseCtx(c)

		c.Locals(contextHelperTestKey{}, "locals")
		c.SetContext(context.WithValue(context.Background(), contextHelperTestKey{}, "context"))

		value, ok := ValueFromContext[string](c, contextHelperTestKey{})
		require.True(t, ok)
		require.Equal(t, "context", value)
	})

	t.Run("fiber custom ctx enabled prefers context", func(t *testing.T) {
		t.Parallel()

		app := NewWithCustomCtx(func(app *App) CustomCtx {
			return &customCtx{DefaultCtx: *NewDefaultCtx(app)}
		}, Config{PassLocalsToContext: true})
		raw := &fasthttp.RequestCtx{}
		c := app.AcquireCtx(raw)
		defer app.ReleaseCtx(c)

		c.Locals(contextHelperTestKey{}, "locals")
		c.SetContext(context.WithValue(context.Background(), contextHelperTestKey{}, "context"))

		value, ok := ValueFromContext[string](c, contextHelperTestKey{})
		require.True(t, ok)
		require.Equal(t, "context", value)
	})

	t.Run("fasthttp request ctx", func(t *testing.T) {
		t.Parallel()

		raw := &fasthttp.RequestCtx{}
		raw.SetUserValue(contextHelperTestKey{}, "value")

		value, ok := ValueFromContext[string](raw, contextHelperTestKey{})
		require.True(t, ok)
		require.Equal(t, "value", value)
	})

	t.Run("context.Context", func(t *testing.T) {
		t.Parallel()

		ctx := context.WithValue(context.Background(), contextHelperTestKey{}, "value")

		value, ok := ValueFromContext[string](ctx, contextHelperTestKey{})
		require.True(t, ok)
		require.Equal(t, "value", value)
	})
}

// go test -run Test_Ctx_Context_Multiple_Requests
func Test_Ctx_Context_Multiple_Requests(t *testing.T) {
	t.Parallel()
	var testKey testContextKey
	testValue := "foobar-value"

	app := New()
	app.Get("/", func(c Ctx) error {
		ctx := c.Context()

		if ctx.Value(testKey) != nil {
			return c.SendStatus(StatusInternalServerError)
		}

		input := utils.CopyString(Query(c, "input", "NO_VALUE"))
		ctx = context.WithValue(ctx, testKey, fmt.Sprintf("%s_%s", testValue, input))
		c.SetContext(ctx)

		return c.Status(StatusOK).SendString(fmt.Sprintf("resp_%s_returned", input))
	})

	// Consecutive Requests
	for i := 1; i <= 10; i++ {
		t.Run(fmt.Sprintf("request_%d", i), func(t *testing.T) {
			t.Parallel()
			resp, err := app.Test(httptest.NewRequest(MethodGet, fmt.Sprintf("/?input=%d", i), http.NoBody))

			require.NoError(t, err, "Unexpected error from response")
			require.Equal(t, StatusOK, resp.StatusCode, "context.Context returned from c.Context() is reused")

			b, err := io.ReadAll(resp.Body)
			require.NoError(t, err, "Unexpected error from reading response body")
			require.Equal(t, fmt.Sprintf("resp_%d_returned", i), string(b), "response text incorrect")
		})
	}
}

// go test -run Test_Ctx_Locals_Generic
func Test_Ctx_Locals_Generic(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error {
		Locals(c, "john", "doe")
		Locals(c, "age", 18)
		Locals(c, "isHuman", true)
		return c.Next()
	})
	app.Get("/test", func(c Ctx) error {
		require.Equal(t, "doe", Locals[string](c, "john"))
		require.Equal(t, 18, Locals[int](c, "age"))
		require.True(t, Locals[bool](c, "isHuman"))
		require.Equal(t, 0, Locals[int](c, "isHuman"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Locals_GenericCustomStruct
func Test_Ctx_Locals_GenericCustomStruct(t *testing.T) {
	t.Parallel()

	type User struct {
		name string
		age  int
	}

	app := New()
	app.Use(func(c Ctx) error {
		Locals(c, "user", User{name: "john", age: 18})
		return c.Next()
	})
	app.Use("/test", func(c Ctx) error {
		require.Equal(t, User{name: "john", age: 18}, Locals[User](c, "user"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Method
func Test_Ctx_Method(t *testing.T) {
	t.Parallel()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	app := New()
	c := app.AcquireCtx(fctx)

	require.Equal(t, MethodGet, c.Method())
	c.Method(MethodPost)
	require.Equal(t, MethodPost, c.Method())

	c.Method("MethodInvalid")
	require.Equal(t, MethodPost, c.Method())
}

// go test -run Test_Ctx_ClientHelloInfo
func Test_Ctx_ClientHelloInfo(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/ServerName", func(c Ctx) error {
		result := c.ClientHelloInfo()
		if result != nil {
			return c.SendString(result.ServerName)
		}

		return c.SendString("ClientHelloInfo is nil")
	})
	app.Get("/SignatureSchemes", func(c Ctx) error {
		result := c.ClientHelloInfo()
		if result != nil {
			return c.JSON(result.SignatureSchemes)
		}

		return c.SendString("ClientHelloInfo is nil")
	})
	app.Get("/SupportedVersions", func(c Ctx) error {
		result := c.ClientHelloInfo()
		if result != nil {
			return c.JSON(result.SupportedVersions)
		}

		return c.SendString("ClientHelloInfo is nil")
	})

	// Test without TLS handler
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/ServerName", http.NoBody))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("ClientHelloInfo is nil"), body)

	// Test with TLS Handler
	const (
		pssWithSHA256 = 0x0804
		versionTLS13  = 0x0304
	)
	app.tlsHandler = &TLSHandler{clientHelloInfo: &tls.ClientHelloInfo{
		ServerName:        "example.golang",
		SignatureSchemes:  []tls.SignatureScheme{pssWithSHA256},
		SupportedVersions: []uint16{versionTLS13},
	}}

	// Test ServerName
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/ServerName", http.NoBody))
	require.NoError(t, err)

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("example.golang"), body)

	// Test SignatureSchemes
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/SignatureSchemes", http.NoBody))
	require.NoError(t, err)

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "["+strconv.Itoa(pssWithSHA256)+"]", string(body))

	// Test SupportedVersions
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/SupportedVersions", http.NoBody))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "["+strconv.Itoa(versionTLS13)+"]", string(body))
}

// go test -run Test_Ctx_InvalidMethod
func Test_Ctx_InvalidMethod(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(_ Ctx) error {
		return nil
	})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("InvalidMethod")
	fctx.Request.SetRequestURI("/")

	app.Handler()(fctx)

	require.Equal(t, 501, fctx.Response.StatusCode())
	require.Equal(t, []byte("Not Implemented"), fctx.Response.Body())
}

// go test -run Test_Ctx_MultipartForm
func Test_Ctx_MultipartForm(t *testing.T) {
	t.Parallel()
	app := New()

	app.Post("/test", func(c Ctx) error {
		result, err := c.MultipartForm()
		require.NoError(t, err)
		require.Equal(t, "john", result.Value["name"][0])
		return nil
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	require.NoError(t, writer.WriteField("name", "john"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set(HeaderContentType, "multipart/form-data; boundary="+writer.Boundary())
	req.Header.Set(HeaderContentLength, strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -v -run=^$ -bench=Benchmark_Ctx_MultipartForm -benchmem -count=4
func Benchmark_Ctx_MultipartForm(b *testing.B) {
	app := New()

	app.Post("/", func(c Ctx) error {
		_, err := c.MultipartForm()
		return err
	})

	c := &fasthttp.RequestCtx{}

	body := []byte("--b\r\nContent-Disposition: form-data; name=\"name\"\r\n\r\njohn\r\n--b--")
	c.Request.SetBody(body)
	c.Request.Header.SetContentType(MIMEMultipartForm + `;boundary="b"`)
	c.Request.Header.SetContentLength(len(body))

	h := app.Handler()

	b.ReportAllocs()

	for b.Loop() {
		h(c)
	}
}

// go test -run Test_Ctx_OriginalURL
func Test_Ctx_OriginalURL(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.SetRequestURI("http://google.com/test?search=demo")
	require.Equal(t, "http://google.com/test?search=demo", c.OriginalURL())
}

// go test -race -run Test_Ctx_Params
func Test_Ctx_Params(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test/:user", func(c Ctx) error {
		require.Equal(t, "john", c.Params("user"))
		return nil
	})
	app.Get("/test2/*", func(c Ctx) error {
		require.Equal(t, "im/a/cookie", c.Params("*"))
		return nil
	})
	app.Get("/test3/*/blafasel/*", func(c Ctx) error {
		require.Equal(t, "1111", c.Params("*1"))
		require.Equal(t, 1111, Params(c, "*1", 0))
		require.Equal(t, "2222", c.Params("*2"))
		require.Equal(t, 2222, Params(c, "*2", 0))
		require.Equal(t, "1111", c.Params("*"))
		require.Equal(t, 1111, Params(c, "*", 0))
		return nil
	})
	app.Get("/test4/:optional?", func(c Ctx) error {
		require.Empty(t, c.Params("optional"))
		require.Equal(t, "default", Params(c, "optional", "default"))
		return nil
	})
	app.Get("/test5/:id/:Id", func(c Ctx) error {
		require.Equal(t, "first", c.Params("id"))
		require.Equal(t, "first", c.Params("Id"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test/john", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test2/im/a/cookie", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test3/1111/blafasel/2222", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test4", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test5/first/second", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

func Test_Ctx_Params_ErrorHandler_Panic_Issue_2832(t *testing.T) {
	t.Parallel()

	app := New(Config{
		ErrorHandler: func(c Ctx, _ error) error {
			return c.SendString(c.Params("user"))
		},
		BodyLimit: 1 * 1024,
	})

	app.Get("/test/:user", func(_ Ctx) error {
		return NewError(StatusInternalServerError, "error")
	})

	largeBody := make([]byte, 2*1024)
	_, err := app.Test(httptest.NewRequest(MethodGet, "/test/john", bytes.NewReader(largeBody)))
	require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge, "app.Test(req)")
}

func Test_Ctx_Params_Case_Sensitive(t *testing.T) {
	t.Parallel()
	app := New(Config{CaseSensitive: true})
	app.Get("/test/:User", func(c Ctx) error {
		require.Equal(t, "john", c.Params("User"))
		require.Empty(t, c.Params("user"))
		return nil
	})
	app.Get("/test2/:id/:Id", func(c Ctx) error {
		require.Equal(t, "first", c.Params("id"))
		require.Equal(t, "second", c.Params("Id"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test/john", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test2/first/second", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

func Test_Ctx_Params_Immutable(t *testing.T) {
	t.Parallel()
	app := New(Config{Immutable: true})
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.route = &Route{Params: []string{"user"}}
	c.path = []byte("/test/john")
	c.values[0] = c.app.toString(c.path[6:])

	param := c.Params("user")
	c.path[6] = 'p'
	c.path[7] = 'a'
	c.path[8] = 'u'
	c.path[9] = 'l'

	require.Equal(t, "john", param)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Params -benchmem -count=4
func Benchmark_Ctx_Params(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.route = &Route{
		Params: []string{
			"param1", "param2", "param3", "param4",
		},
	}
	c.values = [maxParams]string{
		"john", "doe", "is", "awesome",
	}
	var res string
	b.ReportAllocs()
	for b.Loop() {
		_ = c.Params("param1")
		_ = c.Params("param2")
		_ = c.Params("param3")
		res = c.Params("param4")
	}
	require.Equal(b, "awesome", res)
}

// go test -run Test_Ctx_Path
func Test_Ctx_Path(t *testing.T) {
	t.Parallel()
	app := New(Config{UnescapePath: true})
	app.Get("/test/:user", func(c Ctx) error {
		require.Equal(t, "/Test/John", c.Path())
		require.Equal(t, "/Test/John", string(c.Request().URI().Path()))
		// not strict && case-insensitive
		require.Equal(t, "/ABC/", c.Path("/ABC/"))
		require.Equal(t, "/ABC/", string(c.Request().URI().Path()))
		require.Equal(t, "/test/john/", c.Path("/test/john/"))
		require.Equal(t, "/test/john/", string(c.Request().URI().Path()))
		return nil
	})

	// test with special chars
	app.Get("/specialChars/:name", func(c Ctx) error {
		require.Equal(t, "/specialChars/créer", c.Path())
		// unescape is also working if you set the path afterwards
		require.Equal(t, "/اختبار/", c.Path("/%D8%A7%D8%AE%D8%AA%D8%A8%D8%A7%D8%B1/"))
		require.Equal(t, "/اختبار/", string(c.Request().URI().Path()))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/specialChars/cr%C3%A9er", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Protocol
func Test_Ctx_Protocol(t *testing.T) {
	t.Parallel()
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.Equal(t, "HTTP/1.1", c.Protocol())

	c.Request().Header.SetProtocol("HTTP/2")
	require.Equal(t, "HTTP/2", c.Protocol())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Protocol -benchmem -count=4
func Benchmark_Ctx_Protocol(b *testing.B) {
	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.Protocol()
	}

	require.Equal(b, "HTTP/1.1", res)
}

// go test -run Test_Ctx_Scheme
func Test_Ctx_Scheme(t *testing.T) {
	t.Parallel()

	app := New(Config{
		TrustProxy: true,
		TrustProxyConfig: TrustProxyConfig{
			Proxies: []string{"0.0.0.0"},
		},
	})

	freq := &fasthttp.RequestCtx{}
	freq.SetRemoteAddr(net.Addr(&net.TCPAddr{IP: net.ParseIP("0.0.0.0")}))
	freq.Request.Header.Set("X-Forwarded", "invalid")

	c := app.AcquireCtx(freq)

	c.Request().Header.Set(HeaderXForwardedProto, schemeHTTPS)
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, schemeHTTPS)
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProto, "https, http")
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, "https, http")
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, schemeHTTPS)
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	require.Equal(t, schemeHTTP, c.Scheme())
}

// go test -run Test_Ctx_Scheme_HeaderNormalization
func Test_Ctx_Scheme_HeaderNormalization(t *testing.T) {
	t.Parallel()

	app := New(Config{
		TrustProxy: true,
		TrustProxyConfig: TrustProxyConfig{
			Proxies: []string{"0.0.0.0"},
		},
	})

	freq := &fasthttp.RequestCtx{}
	freq.SetRemoteAddr(net.Addr(&net.TCPAddr{IP: net.ParseIP("0.0.0.0")}))

	c := app.AcquireCtx(freq)

	c.Request().Header.Set("x-forwarded-proto", " HTTPS , http")
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set("X-FORWARDED-PROTOCOL", " HTTPS")
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set("x-url-scheme", " HTTPS ")
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set("x-Forwarded-ProToCol", " HTTPS ")
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Scheme -benchmem -count=4
func Benchmark_Ctx_Scheme(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.Scheme()
	}
	require.Equal(b, "http", res)
}

// go test -run Test_Ctx_Scheme_TrustedProxy
func Test_Ctx_Scheme_TrustedProxy(t *testing.T) {
	t.Parallel()
	app := New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.0.0.0"}}})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXForwardedProto, schemeHTTPS)
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, schemeHTTPS)
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, schemeHTTPS)
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	require.Equal(t, schemeHTTP, c.Scheme())
}

// go test -run Test_Ctx_Scheme_TrustedProxyRange
func Test_Ctx_Scheme_TrustedProxyRange(t *testing.T) {
	t.Parallel()
	app := New(Config{
		TrustProxy:       true,
		TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.0.0.0/30"}},
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXForwardedProto, schemeHTTPS)
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, schemeHTTPS)
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, schemeHTTPS)
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	require.Equal(t, schemeHTTP, c.Scheme())
}

// go test -run Test_Ctx_Scheme_UntrustedProxyRange
func Test_Ctx_Scheme_UntrustedProxyRange(t *testing.T) {
	t.Parallel()
	app := New(Config{
		TrustProxy:       true,
		TrustProxyConfig: TrustProxyConfig{Proxies: []string{"1.1.1.1/30"}},
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXForwardedProto, schemeHTTPS)
	require.Equal(t, schemeHTTP, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, schemeHTTPS)
	require.Equal(t, schemeHTTP, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	require.Equal(t, schemeHTTP, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, schemeHTTPS)
	require.Equal(t, schemeHTTP, c.Scheme())
	c.Request().Header.Reset()

	require.Equal(t, schemeHTTP, c.Scheme())
}

// go test -run Test_Ctx_Scheme_UnTrustedProxy
func Test_Ctx_Scheme_UnTrustedProxy(t *testing.T) {
	t.Parallel()
	app := New(Config{
		TrustProxy:       true,
		TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.8.0.1"}},
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXForwardedProto, schemeHTTPS)
	require.Equal(t, schemeHTTP, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, schemeHTTPS)
	require.Equal(t, schemeHTTP, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	require.Equal(t, schemeHTTP, c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, schemeHTTPS)
	require.Equal(t, schemeHTTP, c.Scheme())
	c.Request().Header.Reset()

	require.Equal(t, schemeHTTP, c.Scheme())
}

// go test -run Test_Ctx_Query
func Test_Ctx_Query(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().URI().SetQueryString("search=john&age=20")
	require.Equal(t, "john", c.Query("search"))
	require.Equal(t, "20", c.Query("age"))
	require.Equal(t, "default", c.Query("unknown", "default"))

	// test with generic
	require.Equal(t, "john", Query[string](c, "search"))
	require.Equal(t, "20", Query[string](c, "age"))
	require.Equal(t, "default", Query(c, "unknown", "default"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Query -benchmem -count=4
func Benchmark_Ctx_Query(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().URI().SetQueryString("search=john&age=8")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = Query[string](c, "search")
	}
	require.Equal(b, "john", res)
}

// go test -run Test_Ctx_Range
func Test_Ctx_Range(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	testRange := func(header string, ranges ...RangeSet) {
		c.Request().Header.Set(HeaderRange, header)
		result, err := c.Range(1000)
		if len(ranges) == 0 {
			require.Error(t, err)
		} else {
			require.Equal(t, "bytes", result.Type)
			require.NoError(t, err)
		}
		require.Len(t, ranges, len(result.Ranges))
		for i := range ranges {
			require.Equal(t, ranges[i], result.Ranges[i])
		}
	}

	testRange("bytes=500")
	testRange("bytes=")
	testRange("bytes=500=")
	testRange("bytes=500-300")
	testRange("bytes=a-700", RangeSet{Start: 300, End: 999})
	testRange("bytes=500-b", RangeSet{Start: 500, End: 999})
	testRange("bytes=500-1000", RangeSet{Start: 500, End: 999})
	testRange("bytes=500-700", RangeSet{Start: 500, End: 700})
	testRange("bytes=0-0,2-1000", RangeSet{Start: 0, End: 0}, RangeSet{Start: 2, End: 999})
	testRange("bytes=0-99,450-549,-100", RangeSet{Start: 0, End: 99}, RangeSet{Start: 450, End: 549}, RangeSet{Start: 900, End: 999})
	testRange("bytes=500-700,601-999", RangeSet{Start: 500, End: 700}, RangeSet{Start: 601, End: 999})
	testRange("bytes= 0-1", RangeSet{Start: 0, End: 1})
	testRange("seconds=0-1")
}

func Test_Ctx_Range_LargeFile(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	size := int64(math.MaxInt32) + 1024
	start := int64(math.MaxInt32) + 10
	end := start + 50

	c.Request().Header.Set(HeaderRange, fmt.Sprintf("bytes=%d-%d", start, end))
	result, err := c.Range(size)
	require.NoError(t, err)
	require.Equal(t, "bytes", result.Type)
	require.Len(t, result.Ranges, 1)
	require.Equal(t, start, result.Ranges[0].Start)
	require.Equal(t, end, result.Ranges[0].End)

	c.Request().Header.Set(HeaderRange, "bytes=-200")
	result, err = c.Range(size)
	require.NoError(t, err)
	require.Equal(t, size-200, result.Ranges[0].Start)
	require.Equal(t, size-1, result.Ranges[0].End)
}

func Test_Ctx_Range_Overflow(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	tooBig := uint64((math.MaxUint64 >> 1) + 1)

	c.Request().Header.Set(HeaderRange, fmt.Sprintf("bytes=%d-100", tooBig))
	_, err := c.Range(math.MaxInt64)
	require.ErrorIs(t, err, ErrRangeMalformed)

	c.Request().Header.Set(HeaderRange, fmt.Sprintf("bytes=0-%d", tooBig))
	_, err = c.Range(math.MaxInt64)
	require.ErrorIs(t, err, ErrRangeMalformed)
}

func Test_Ctx_Range_Unsatisfiable(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		_, err := c.Range(10)
		if err != nil {
			return err
		}
		return c.SendString("ok")
	})

	req := httptest.NewRequest(MethodGet, "http://example.com/", http.NoBody)
	req.Header.Set(HeaderRange, "bytes=20-30")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusRequestedRangeNotSatisfiable, resp.StatusCode)
	require.Equal(t, "bytes */10", resp.Header.Get(HeaderContentRange))
}

func Test_Ctx_Range_TooManyRanges(t *testing.T) {
	t.Parallel()
	app := New(Config{MaxRanges: 2})
	app.Get("/", func(c Ctx) error {
		_, err := c.Range(10)
		if err != nil {
			return err
		}
		return c.SendString("ok")
	})

	req := httptest.NewRequest(MethodGet, "http://example.com/", http.NoBody)
	req.Header.Set(HeaderRange, "bytes=0-1,2-3,4-5")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusRequestedRangeNotSatisfiable, resp.StatusCode)
	require.Equal(t, "bytes */10", resp.Header.Get(HeaderContentRange))
}

func Test_Ctx_Range_SuffixNormalization(t *testing.T) {
	t.Parallel()

	body := bytes.Repeat([]byte("x"), 123)

	newApp := func() *App {
		app := New()

		app.Get("/", func(c Ctx) error {
			rangesHeader := c.Get(HeaderRange)
			if rangesHeader == "" {
				return c.Send(body)
			}

			rangeData, err := c.Range(int64(len(body)))
			if err != nil {
				return err
			}

			if len(rangeData.Ranges) != 1 {
				c.Status(StatusRequestedRangeNotSatisfiable)
				c.Set(HeaderContentRange, fmt.Sprintf("bytes */%d", len(body)))
				return ErrRequestedRangeNotSatisfiable
			}

			currentRange := rangeData.Ranges[0]
			contentRange := fmt.Sprintf("bytes %d-%d/%d", currentRange.Start, currentRange.End, len(body))
			c.Set(HeaderContentRange, contentRange)

			statusCode := StatusPartialContent
			if currentRange.Start == 0 && currentRange.End == int64(len(body))-1 {
				statusCode = StatusOK
			}

			c.Status(statusCode)
			return c.Send(body[currentRange.Start : currentRange.End+1])
		})

		return app
	}

	testCases := []struct {
		name             string
		rangeHeader      string
		contentRange     string
		statusCode       int
		expectedBodySize int
	}{
		{
			name:             "suffix less than size",
			rangeHeader:      "bytes=-20",
			contentRange:     "bytes 103-122/123",
			statusCode:       StatusPartialContent,
			expectedBodySize: 20,
		},
		{
			name:             "suffix equal to size",
			rangeHeader:      "bytes=-123",
			contentRange:     "bytes 0-122/123",
			statusCode:       StatusOK,
			expectedBodySize: 123,
		},
		{
			name:             "suffix larger than size",
			rangeHeader:      "bytes=-9999",
			contentRange:     "bytes 0-122/123",
			statusCode:       StatusOK,
			expectedBodySize: 123,
		},
		{
			name:         "unsatisfiable mixed ranges",
			rangeHeader:  "bytes=200-400,700-1200",
			contentRange: "bytes */123",
			statusCode:   StatusRequestedRangeNotSatisfiable,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := newApp()
			req := httptest.NewRequest(MethodGet, "http://example.com/", http.NoBody)
			if tc.rangeHeader != "" {
				req.Header.Set(HeaderRange, tc.rangeHeader)
			}

			resp, err := app.Test(req)
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, resp.Body.Close())
			})

			require.Equal(t, tc.statusCode, resp.StatusCode)
			require.Equal(t, tc.contentRange, resp.Header.Get(HeaderContentRange))

			if tc.expectedBodySize > 0 {
				bodyBytes, bodyErr := io.ReadAll(resp.Body)
				require.NoError(t, bodyErr)
				require.Len(t, bodyBytes, tc.expectedBodySize)
			}
		})
	}
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Range -benchmem -count=4
func Benchmark_Ctx_Range(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	testCases := []struct {
		str   string
		start int64
		end   int64
	}{
		{str: "bytes=-700", start: 300, end: 999},
		{str: "bytes=500-", start: 500, end: 999},
		{str: "bytes=500-1000", start: 500, end: 999},
		{str: "bytes=0-700,800-1000", start: 0, end: 700},
	}

	for _, tc := range testCases {
		b.Run(tc.str, func(b *testing.B) {
			c.Request().Header.Set(HeaderRange, tc.str)
			var (
				result Range
				err    error
			)
			for b.Loop() {
				result, err = c.Range(1000)
			}
			require.NoError(b, err)
			require.Equal(b, "bytes", result.Type)
			require.Equal(b, tc.start, result.Ranges[0].Start)
			require.Equal(b, tc.end, result.Ranges[0].End)
		})
	}
}

// go test -run Test_Ctx_Route
func Test_Ctx_Route(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c Ctx) error {
		require.Equal(t, "/test", c.Route().Path)
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.Equal(t, "/", c.Route().Path)
	require.Equal(t, MethodGet, c.Route().Method)
	require.Empty(t, c.Route().Handlers)
}

// go test -run Test_Ctx_FullPath
func Test_Ctx_FullPath(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/test", func(c Ctx) error {
		require.Equal(t, "/test", c.FullPath())
		return c.SendStatus(StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	defer func() { require.NoError(t, resp.Body.Close()) }()

	require.Equal(t, StatusOK, resp.StatusCode)
}

// go test -run Test_Ctx_FullPath_Group
func Test_Ctx_FullPath_Group(t *testing.T) {
	t.Parallel()

	app := New()
	group := app.Group("/v1")
	group.Get("/test", func(c Ctx) error {
		require.Equal(t, "/v1/test", c.FullPath())
		return c.SendStatus(StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	defer func() { require.NoError(t, resp.Body.Close()) }()

	require.Equal(t, StatusOK, resp.StatusCode)
}

// go test -run Test_Ctx_FullPath_Middleware
func Test_Ctx_FullPath_Middleware(t *testing.T) {
	t.Parallel()

	app := New()

	var recorded []string

	app.Use(func(c Ctx) error {
		recorded = append(recorded, c.FullPath())

		if err := c.Next(); err != nil {
			return err
		}

		recorded = append(recorded, c.FullPath())
		return nil
	})

	app.Get("/test", func(c Ctx) error {
		require.Equal(t, "/test", c.FullPath())
		return c.SendStatus(StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	defer func() { require.NoError(t, resp.Body.Close()) }()

	require.Equal(t, StatusOK, resp.StatusCode)
	require.Equal(t, []string{"/", "/test"}, recorded)
}

// go test -run Test_Ctx_RouteNormalized
func Test_Ctx_RouteNormalized(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c Ctx) error {
		require.Equal(t, "/test", c.Route().Path)
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "//test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_SaveFile
func Test_Ctx_SaveFile(t *testing.T) {
	// TODO We should clean this up
	t.Parallel()
	app := New()

	app.Post("/test", func(c Ctx) error {
		fh, err := c.Req().FormFile("file")
		require.NoError(t, err)

		tempFile, err := os.CreateTemp(os.TempDir(), "test-")
		require.NoError(t, err)

		defer func(file *os.File) {
			closeErr := file.Close()
			require.NoError(t, closeErr)
			closeErr = os.Remove(file.Name())
			require.NoError(t, closeErr)
		}(tempFile)
		err = c.SaveFile(fh, tempFile.Name())
		require.NoError(t, err)

		bs, err := os.ReadFile(tempFile.Name())
		require.NoError(t, err)
		require.Equal(t, "hello world", string(bs))
		return nil
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	ioWriter, err := writer.CreateFormFile("file", "test")
	require.NoError(t, err)

	_, err = ioWriter.Write([]byte("hello world"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

func createMultipartFileHeader(t *testing.T, filename string, data []byte) *multipart.FileHeader {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	ioWriter, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)

	_, err = ioWriter.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	multipartReader := multipart.NewReader(bytes.NewReader(body.Bytes()), writer.Boundary())
	form, err := multipartReader.ReadForm(int64(len(body.Bytes())))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, form.RemoveAll())
	})

	files := form.File["file"]
	require.Len(t, files, 1)

	return files[0]
}

// go test -run Test_Ctx_SaveFileToStorage
func Test_Ctx_SaveFileToStorage(t *testing.T) {
	t.Parallel()
	app := New()
	storage := memory.New()

	app.Post("/test", func(c Ctx) error {
		fh, err := c.FormFile("file")
		require.NoError(t, err)

		err = c.SaveFileToStorage(fh, "test", storage)
		require.NoError(t, err)

		file, err := storage.Get("test")
		require.Equal(t, []byte("hello world"), file)
		require.NoError(t, err)

		err = storage.Delete("test")
		require.NoError(t, err)

		return nil
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	ioWriter, err := writer.CreateFormFile("file", "test")
	require.NoError(t, err)

	_, err = ioWriter.Write([]byte("hello world"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

func Test_Ctx_SaveFileToStorage_LargeUpload(t *testing.T) {
	t.Parallel()
	const (
		bodyLimit = 8 * 1024 * 1024
		fileSize  = 5 * 1024 * 1024
	)

	app := New(Config{BodyLimit: bodyLimit})
	storage := memory.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	t.Cleanup(func() {
		app.ReleaseCtx(ctx)
	})

	fileHeader := createMultipartFileHeader(t, "large.bin", bytes.Repeat([]byte{'a'}, fileSize))

	err := ctx.SaveFileToStorage(fileHeader, "test", storage)
	require.NoError(t, err)

	stored, err := storage.Get("test")
	require.NoError(t, err)
	require.Len(t, stored, fileSize)
}

func Test_Ctx_SaveFileToStorage_LimitExceeded(t *testing.T) {
	t.Parallel()
	const (
		allowedSize = 1024
		fileSize    = allowedSize + 512
	)

	app := New(Config{BodyLimit: allowedSize})

	storage := memory.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	t.Cleanup(func() {
		app.ReleaseCtx(ctx)
	})

	fileHeader := createMultipartFileHeader(t, "too-large.bin", bytes.Repeat([]byte{'a'}, fileSize))

	err := ctx.SaveFileToStorage(fileHeader, "test", storage)
	require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
}

func Test_Ctx_SaveFileToStorage_LimitExceededUnknownSize(t *testing.T) {
	t.Parallel()
	const (
		allowedSize = 1024
		fileSize    = allowedSize + 256
	)

	app := New(Config{BodyLimit: allowedSize})

	storage := memory.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	t.Cleanup(func() {
		app.ReleaseCtx(ctx)
	})

	fileHeader := createMultipartFileHeader(t, "unknown-size.bin", bytes.Repeat([]byte{'a'}, fileSize))
	fileHeader.Size = -1

	err := ctx.SaveFileToStorage(fileHeader, "test", storage)
	require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
}

type captureStorage struct {
	t    *testing.T
	data map[string][]byte
}

func (s *captureStorage) helperFailure(msg string, args ...any) {
	s.t.Helper()
	s.t.Fatalf(msg, args...)
}

func (s *captureStorage) ensureStore(key string, val []byte) {
	s.t.Helper()
	if key == "" || len(val) == 0 {
		return
	}

	if s.data == nil {
		s.data = make(map[string][]byte)
	}

	s.data[key] = val
}

func (s *captureStorage) GetWithContext(context.Context, string) ([]byte, error) {
	s.helperFailure("unexpected call to GetWithContext")
	return nil, nil
}

func (s *captureStorage) Get(string) ([]byte, error) {
	s.helperFailure("unexpected call to Get")
	return nil, nil
}

func (s *captureStorage) SetWithContext(_ context.Context, key string, val []byte, _ time.Duration) error {
	s.ensureStore(key, val)
	return nil
}

func (s *captureStorage) Set(key string, _ []byte, _ time.Duration) error {
	s.helperFailure("unexpected call to Set for key %q", key)
	return nil
}

func (s *captureStorage) DeleteWithContext(context.Context, string) error {
	s.helperFailure("unexpected call to DeleteWithContext")
	return nil
}

func (s *captureStorage) Delete(string) error {
	s.helperFailure("unexpected call to Delete")
	return nil
}

func (s *captureStorage) ResetWithContext(context.Context) error {
	s.data = nil
	return nil
}

func (s *captureStorage) Reset() error {
	s.data = nil
	return nil
}

func (s *captureStorage) Close() error {
	if s == nil {
		return nil
	}

	s.data = nil
	return nil
}

func Test_Ctx_SaveFileToStorage_BufferNotReused(t *testing.T) {
	t.Parallel()

	app := New()
	storage := &captureStorage{t: t}
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	t.Cleanup(func() {
		app.ReleaseCtx(ctx)
	})

	const payloadSize = 1024
	firstPayload := bytes.Repeat([]byte{'a'}, payloadSize)
	secondPayload := bytes.Repeat([]byte{'b'}, payloadSize)

	firstHeader := createMultipartFileHeader(t, "first.bin", firstPayload)
	require.NoError(t, ctx.SaveFileToStorage(firstHeader, "first", storage))

	firstStored := storage.data["first"]
	require.Equal(t, firstPayload, firstStored)

	secondHeader := createMultipartFileHeader(t, "second.bin", secondPayload)
	require.NoError(t, ctx.SaveFileToStorage(secondHeader, "second", storage))
	require.Equal(t, secondPayload, storage.data["second"])

	require.Equal(t, firstPayload, firstStored, "stored data must not rely on pooled buffers")
}

type mockContextAwareStorage struct {
	t              *testing.T
	key            any
	expectedValue  any
	validateCtx    func(context.Context)
	cancel         context.CancelFunc
	ctxMatched     atomic.Bool
	cancelObserved atomic.Bool
}

func (s *mockContextAwareStorage) helperFailure(msg string, args ...any) {
	s.t.Helper()
	s.t.Fatalf(msg, args...)
}

func (s *mockContextAwareStorage) GetWithContext(context.Context, string) ([]byte, error) {
	s.helperFailure("unexpected call to GetWithContext")
	return nil, nil
}

func (s *mockContextAwareStorage) Get(string) ([]byte, error) {
	s.helperFailure("unexpected call to Get")
	return nil, nil
}

func (s *mockContextAwareStorage) SetWithContext(ctx context.Context, _ string, _ []byte, _ time.Duration) error {
	s.t.Helper()
	if s.validateCtx == nil {
		s.helperFailure("validateCtx must be configured before SetWithContext")
	}
	s.validateCtx(ctx)
	if val := ctx.Value(s.key); val != s.expectedValue {
		s.helperFailure("storage observed unexpected context value: %v", val)
	}
	s.ctxMatched.Store(true)
	if s.cancel != nil {
		s.cancel()
	}
	select {
	case <-ctx.Done():
		s.cancelObserved.Store(true)
	case <-time.After(100 * time.Millisecond):
		s.helperFailure("storage did not observe context cancellation")
	}
	return nil
}

func (s *mockContextAwareStorage) Set(string, []byte, time.Duration) error {
	s.helperFailure("unexpected call to Set")
	return nil
}

func (s *mockContextAwareStorage) DeleteWithContext(context.Context, string) error {
	s.helperFailure("unexpected call to DeleteWithContext")
	return nil
}

func (s *mockContextAwareStorage) Delete(string) error {
	s.helperFailure("unexpected call to Delete")
	return nil
}

func (s *mockContextAwareStorage) ResetWithContext(context.Context) error {
	s.helperFailure("unexpected call to ResetWithContext")
	return nil
}

func (s *mockContextAwareStorage) Reset() error {
	s.helperFailure("unexpected call to Reset")
	return nil
}

func (s *mockContextAwareStorage) Close() error {
	if s == nil {
		return nil
	}
	return nil
}

// go test -run Test_Ctx_SaveFileToStorage_ContextPropagation
func Test_Ctx_SaveFileToStorage_ContextPropagation(t *testing.T) {
	t.Parallel()

	type ctxKeyType string

	const ctxKey ctxKeyType = "storage-context-key"

	storage := &mockContextAwareStorage{t: t, key: ctxKey, expectedValue: "expected-context-value"}
	app := New()

	app.Post("/test", func(c Ctx) error {
		fh, err := c.FormFile("file")
		require.NoError(t, err)

		ctxWithValue := context.WithValue(context.Background(), ctxKey, storage.expectedValue)
		ctx, cancel := context.WithCancel(ctxWithValue)
		storage.validateCtx = func(received context.Context) {
			if received != ctx {
				storage.helperFailure("storage received unexpected context instance")
			}
		}
		storage.cancel = cancel

		c.SetContext(ctx)

		err = c.SaveFileToStorage(fh, "test", storage)
		require.NoError(t, err)

		require.True(t, storage.ctxMatched.Load(), "storage should receive the context installed on Ctx")
		require.True(t, storage.cancelObserved.Load(), "storage should observe context cancellation")

		return nil
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	ioWriter, err := writer.CreateFormFile("file", "test")
	require.NoError(t, err)

	_, err = ioWriter.Write([]byte("hello world"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Secure
func Test_Ctx_Secure(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	// TODO Add TLS conn
	require.False(t, c.Secure())
}

// go test -run Test_Ctx_Stale
func Test_Ctx_Stale(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.True(t, c.Stale())
}

// go test -run Test_Ctx_Subdomains
func Test_Ctx_Subdomains(t *testing.T) {
	app := New()

	type tc struct {
		name   string
		host   string
		offset []int // nil ⇒ call without argument
		want   []string
	}

	cases := []tc{
		{
			name:   "default offset (2) drops registrable domain + TLD",
			host:   "john.doe.is.awesome.google.com",
			offset: nil, // Subdomains()
			want:   []string{"john", "doe", "is", "awesome"},
		},
		{
			name:   "custom offset trims N right-hand labels",
			host:   "john.doe.is.awesome.google.com",
			offset: []int{4},
			want:   []string{"john", "doe"},
		},
		{
			name:   "offset too high returns empty",
			host:   "john.doe.is.awesome.google.com",
			offset: []int{10},
			want:   []string{},
		},
		{
			name:   "zero offset returns all labels",
			host:   "john.doe.google.com",
			offset: []int{0},
			want:   []string{"john", "doe", "google", "com"},
		},
		{
			name:   "offset 1 keeps registrable domain",
			host:   "john.doe.google.com",
			offset: []int{1},
			want:   []string{"john", "doe", "google"},
		},
		{
			name:   "negative offset returns empty",
			host:   "john.doe.google.com",
			offset: []int{-1},
			want:   []string{},
		},
		{
			name:   "offset equal len returns empty",
			host:   "john.doe.com",
			offset: []int{3},
			want:   []string{},
		},
		{
			name:   "offset equal len returns empty",
			host:   "john.doe.com",
			offset: []int{3},
			want:   []string{},
		},
		{
			name:   "zero offset returns all labels with port present",
			host:   "localhost:3000",
			offset: []int{0},
			want:   []string{"localhost"},
		},
		{
			name:   "host with port — custom offset trims 2 labels",
			host:   "foo.bar.example.com:8080",
			offset: []int{2},
			want:   []string{"foo", "bar"},
		},
		{
			name:   "fully qualified domain trims trailing dot",
			host:   "john.doe.example.com.",
			offset: nil,
			want:   []string{"john", "doe"},
		},
		{
			name:   "punycode domain is decoded",
			host:   "xn--bcher-kva.example.com",
			offset: nil,
			want:   []string{"bücher"},
		},
		{
			name:   "punycode fqdn is decoded",
			host:   "xn--bcher-kva.example.com.",
			offset: nil,
			want:   []string{"bücher"},
		},
		{
			name:   "punycode decode failure uses fallback",
			host:   "xn--bcher--.example.com",
			offset: nil,
			want:   []string{"xn--bcher--"},
		},
		{
			name:   "invalid host keeps original lowercased",
			host:   "Foo Bar",
			offset: []int{0},
			want:   []string{"foo bar"},
		},
		{
			name:   "IPv4 host returns empty",
			host:   "192.168.0.1",
			offset: nil,
			want:   []string{},
		},
		{
			name:   "IPv6 host returns empty",
			host:   "[2001:db8::1]",
			offset: nil,
			want:   []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(c)

			c.Request().URI().SetHost(tc.host)
			got := c.Subdomains(tc.offset...)
			require.Equal(t, tc.want, got)
		})
	}
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Subdomains -benchmem -count=4
func Benchmark_Ctx_Subdomains(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().SetRequestURI("http://john.doe.google.com")
	var res []string
	b.ReportAllocs()
	for b.Loop() {
		res = c.Subdomains()
	}
	require.Equal(b, []string{"john", "doe"}, res)
}

// go test -run Test_Ctx_ClearCookie
func Test_Ctx_ClearCookie(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderCookie, "john=doe")
	c.Res().ClearCookie("john")
	require.True(t, strings.HasPrefix(string(c.Response().Header.Peek(HeaderSetCookie)), "john=; expires="))

	c.Request().Header.Set(HeaderCookie, "test1=dummy")
	c.Request().Header.Set(HeaderCookie, "test2=dummy")
	c.ClearCookie()
	require.Contains(t, string(c.Response().Header.Peek(HeaderSetCookie)), "test1=; expires=")
	require.Contains(t, string(c.Response().Header.Peek(HeaderSetCookie)), "test2=; expires=")
}

// go test -race -run Test_Ctx_Download
func Test_Ctx_Download(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.NoError(t, c.Download("ctx.go", "Awesome File!"))

	f, err := os.Open("./ctx.go")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, f.Close())
	}()

	expect, err := io.ReadAll(f)
	require.NoError(t, err)
	require.Equal(t, expect, c.Response().Body())
	require.Equal(t, `attachment; filename="Awesome+File%21"`, string(c.Response().Header.Peek(HeaderContentDisposition)))

	require.NoError(t, c.Res().Download("ctx.go"))
	require.Equal(t, `attachment; filename="ctx.go"`, string(c.Response().Header.Peek(HeaderContentDisposition)))

	require.NoError(t, c.Download("ctx.go", "файл.txt"))
	header := string(c.Response().Header.Peek(HeaderContentDisposition))
	require.Contains(t, header, `filename="файл.txt"`)
	require.Contains(t, header, `filename*=UTF-8''%D1%84%D0%B0%D0%B9%D0%BB.txt`)
}

// go test -race -run Test_Ctx_Download_SanitizesFilenameControls
func Test_Ctx_Download_SanitizesFilenameControls(t *testing.T) {
	t.Parallel()
	app := New()
	testCases := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "base name only",
			filename: "../docs/archive.tar.gz",
			expected: `attachment; filename="archive.tar.gz"`,
		},
		{
			name:     "controls stripped",
			filename: "down\r\nload\t\x00.txt",
			expected: `attachment; filename="download.txt"`,
		},
		{
			name:     "empty after sanitize",
			filename: "\r\n\t\x00",
			expected: `attachment; filename="download"`,
		},
		{
			name:     "dot fallback",
			filename: ".",
			expected: `attachment; filename="download"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			require.NoError(t, c.Download("ctx.go", tc.filename))
			header := string(c.Response().Header.Peek(HeaderContentDisposition))
			require.Equal(t, tc.expected, header)
			require.NotContains(t, header, "\r")
			require.NotContains(t, header, "\n")
			require.NotContains(t, header, "\t")
			require.NotContains(t, header, "\x00")
		})
	}
}

// go test -race -run Test_Ctx_SendEarlyHints
func Test_Ctx_SendEarlyHints(t *testing.T) {
	t.Parallel()
	app := New()

	hints := []string{"<https://cdn.com>; rel=preload; as=script"}
	app.Get("/earlyhints", func(c Ctx) error {
		err := c.SendEarlyHints(hints)
		require.NoError(t, err, "SendEarlyHints")
		c.Status(StatusBadRequest)
		return c.SendString("fail")
	})

	req := httptest.NewRequest(MethodGet, "/earlyhints", http.NoBody)
	resp, err := app.Test(req)

	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusBadRequest, resp.StatusCode, "Status code")
	require.Equal(t, hints, resp.Header["Link"], "Link header")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "fail", string(body))
}

// go test -race -run Test_Ctx_SendFile
func Test_Ctx_SendFile(t *testing.T) {
	t.Parallel()
	app := New()

	// fetch file content
	f, err := os.Open("./ctx.go")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, f.Close())
	}()
	expectFileContent, err := io.ReadAll(f)
	require.NoError(t, err)
	// fetch file info for the not modified test case
	fI, err := os.Stat("./ctx.go")
	require.NoError(t, err)

	// simple test case
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	err = c.SendFile("ctx.go")
	// check expectation
	require.NoError(t, err)
	require.Equal(t, expectFileContent, c.Response().Body())
	require.Equal(t, StatusOK, c.Response().StatusCode())
	app.ReleaseCtx(c)

	// test with custom error code
	c = app.AcquireCtx(&fasthttp.RequestCtx{})
	err = c.Res().Status(StatusInternalServerError).SendFile("ctx.go")
	// check expectation
	require.NoError(t, err)
	require.Equal(t, expectFileContent, c.Response().Body())
	require.Equal(t, StatusInternalServerError, c.Response().StatusCode())
	app.ReleaseCtx(c)

	// test not modified
	c = app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderIfModifiedSince, fI.ModTime().Format(time.RFC1123))
	err = c.SendFile("ctx.go")
	// check expectation
	require.NoError(t, err)
	require.Equal(t, StatusNotModified, c.Response().StatusCode())
	require.Equal(t, []byte(nil), c.Response().Body())
	app.ReleaseCtx(c)
}

// go test -race -run Test_Ctx_SendFile_ContentType
func Test_Ctx_SendFile_ContentType(t *testing.T) {
	t.Parallel()
	app := New()

	// 1) simple case
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	err := c.Res().SendFile("./.github/testdata/fs/img/fiber.png")
	// check expectation
	require.NoError(t, err)
	require.Equal(t, StatusOK, c.Response().StatusCode())
	require.Equal(t, "image/png", string(c.Response().Header.Peek(HeaderContentType)))
	app.ReleaseCtx(c)

	// 2) set by valid file extension, not file header
	// see: https://github.com/valyala/fasthttp/blob/d795f13985f16622a949ea9fc3459cf54dc78b3e/fs.go#L1638
	c = app.AcquireCtx(&fasthttp.RequestCtx{})
	err = c.SendFile("./.github/testdata/fs/img/fiberpng.jpeg")
	// check expectation
	require.NoError(t, err)
	require.Equal(t, StatusOK, c.Response().StatusCode())
	require.Equal(t, "image/jpeg", string(c.Response().Header.Peek(HeaderContentType)))
	app.ReleaseCtx(c)

	// 3) set by file header if extension is invalid
	c = app.AcquireCtx(&fasthttp.RequestCtx{})
	err = c.SendFile("./.github/testdata/fs/img/fiberpng.notvalidext")
	// check expectation
	require.NoError(t, err)
	require.Equal(t, StatusOK, c.Response().StatusCode())
	require.Equal(t, "image/png", string(c.Response().Header.Peek(HeaderContentType)))
	app.ReleaseCtx(c)

	// 4) set by file header if extension is missing
	c = app.AcquireCtx(&fasthttp.RequestCtx{})
	err = c.SendFile("./.github/testdata/fs/img/fiberpng")
	// check expectation
	require.NoError(t, err)
	require.Equal(t, StatusOK, c.Response().StatusCode())
	require.Equal(t, "image/png", string(c.Response().Header.Peek(HeaderContentType)))
	app.ReleaseCtx(c)
}

func Test_Ctx_SendFile_Download(t *testing.T) {
	t.Parallel()
	app := New()

	// fetch file content
	f, err := os.Open("./ctx.go")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, f.Close())
	}()
	expectFileContent, err := io.ReadAll(f)
	require.NoError(t, err)
	// fetch file info for the not modified test case
	_, err = os.Stat("./ctx.go")
	require.NoError(t, err)

	// simple test case
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	err = c.SendFile("ctx.go", SendFile{
		Download: true,
	})
	// check expectation
	require.NoError(t, err)
	require.Equal(t, expectFileContent, c.Response().Body())
	require.Equal(t, "attachment", string(c.Response().Header.Peek(HeaderContentDisposition)))
	require.Equal(t, StatusOK, c.Response().StatusCode())
	app.ReleaseCtx(c)
}

func Test_Ctx_SendFile_MaxAge(t *testing.T) {
	t.Parallel()
	app := New()

	// fetch file content
	f, err := os.Open("./ctx.go")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, f.Close())
	}()
	expectFileContent, err := io.ReadAll(f)
	require.NoError(t, err)

	// fetch file info for the not modified test case
	_, err = os.Stat("./ctx.go")
	require.NoError(t, err)

	// simple test case
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	err = c.SendFile("ctx.go", SendFile{
		MaxAge: 100,
	})

	// check expectation
	require.NoError(t, err)
	require.Equal(t, expectFileContent, c.Response().Body())
	require.Equal(t, "public, max-age=100", string(c.RequestCtx().Response.Header.Peek(HeaderCacheControl)), "CacheControl Control")
	require.Equal(t, StatusOK, c.Response().StatusCode())
	app.ReleaseCtx(c)
}

func Test_Static_Compress(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/file", func(c Ctx) error {
		return c.SendFile("ctx.go", SendFile{
			Compress: true,
		})
	})

	// Note: deflate is not supported by fasthttp.FS
	algorithms := []string{"zstd", "gzip", "br"}
	for _, algo := range algorithms {
		t.Run(algo+"_compression", func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(MethodGet, "/file", http.NoBody)
			req.Header.Set("Accept-Encoding", algo)
			resp, err := app.Test(req, TestConfig{
				Timeout:       10 * time.Second,
				FailOnTimeout: true,
			})

			require.NoError(t, err, "app.Test(req)")
			require.Equal(t, 200, resp.StatusCode, "Status code")
			require.NotEqual(t, "58726", resp.Header.Get(HeaderContentLength))
		})
	}
}

func Test_Ctx_SendFile_Compress_CheckCompressed(t *testing.T) {
	t.Parallel()
	app := New()

	// fetch file content
	f, err := os.Open("./ctx.go")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, f.Close())
	})

	expectedFileContent, err := io.ReadAll(f)
	require.NoError(t, err)

	sendFileBodyReader := func(compression string) ([]byte, error) {
		t.Helper()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.Request().Header.Add(HeaderAcceptEncoding, compression)

		err := c.SendFile("./ctx.go", SendFile{
			Compress: true,
		})

		return c.Response().Body(), err
	}

	t.Run("gzip", func(t *testing.T) {
		t.Parallel()

		b, err := sendFileBodyReader("gzip")
		require.NoError(t, err)
		body, err := fasthttp.AppendGunzipBytes(nil, b)
		require.NoError(t, err)

		require.Equal(t, expectedFileContent, body)
	})

	t.Run("zstd", func(t *testing.T) {
		t.Parallel()

		b, err := sendFileBodyReader("zstd")
		require.NoError(t, err)
		body, err := fasthttp.AppendUnzstdBytes(nil, b)
		require.NoError(t, err)

		require.Equal(t, expectedFileContent, body)
	})

	t.Run("br", func(t *testing.T) {
		t.Parallel()

		b, err := sendFileBodyReader("br")
		require.NoError(t, err)
		body, err := fasthttp.AppendUnbrotliBytes(nil, b)
		require.NoError(t, err)

		require.Equal(t, expectedFileContent, body)
	})
}

//go:embed ctx.go
var embedFile embed.FS

func Test_Ctx_SendFile_EmbedFS(t *testing.T) {
	t.Parallel()
	app := New()

	f, err := os.Open("./ctx.go")
	require.NoError(t, err)

	defer func() {
		require.NoError(t, f.Close())
	}()

	expectFileContent, err := io.ReadAll(f)
	require.NoError(t, err)

	app.Get("/test", func(c Ctx) error {
		return c.SendFile("ctx.go", SendFile{
			FS: embedFile,
		})
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, expectFileContent, body)
}

// go test -race -run Test_Ctx_SendFile_404
func Test_Ctx_SendFile_404(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		return c.SendFile("ctx12.go")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "sendfile: file ctx12.go not found", string(body))
}

func Test_Ctx_SendFile_Multiple(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/test", func(c Ctx) error {
		switch c.Query("file") {
		case "1":
			return c.SendFile("ctx.go")
		case "2":
			return c.SendFile("app.go")
		case "3":
			return c.SendFile("ctx.go", SendFile{
				Download: true,
			})
		case "4":
			return c.SendFile("app_test.go", SendFile{
				FS: os.DirFS("."),
			})
		default:
			return c.SendStatus(StatusNotFound)
		}
	})

	app.Get("/test2", func(c Ctx) error {
		return c.SendFile("ctx.go", SendFile{
			Download: true,
		})
	})

	testCases := []struct {
		url                string
		body               string
		contentDisposition string
	}{
		{url: "/test?file=1", body: "type DefaultCtx struct", contentDisposition: ""},
		{url: "/test?file=2", body: "type App struct", contentDisposition: ""},
		{url: "/test?file=3", body: "type DefaultCtx struct", contentDisposition: "attachment"},
		{url: "/test?file=4", body: "Test_App_MethodNotAllowed", contentDisposition: ""},
		{url: "/test2", body: "type DefaultCtx struct", contentDisposition: "attachment"},
		{url: "/test2", body: "type DefaultCtx struct", contentDisposition: "attachment"},
	}

	for _, tc := range testCases {
		resp, err := app.Test(httptest.NewRequest(MethodGet, tc.url, http.NoBody))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.Equal(t, tc.contentDisposition, resp.Header.Get(HeaderContentDisposition))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), tc.body)
	}

	app.sendfilesMutex.RLock()
	defer app.sendfilesMutex.RUnlock()
	require.Len(t, app.sendfiles, 3)
}

// go test -race -run Test_Ctx_SendFile_Immutable
func Test_Ctx_SendFile_Immutable(t *testing.T) {
	t.Parallel()
	app := New()
	var endpointsForTest []string
	addEndpoint := func(file, endpoint string) {
		endpointsForTest = append(endpointsForTest, endpoint)
		app.Get(endpoint, func(c Ctx) error {
			if err := c.SendFile(file); err != nil {
				require.NoError(t, err)
				return err
			}
			return c.SendStatus(200)
		})
	}

	// relative paths
	addEndpoint("./.github/index.html", "/relativeWithDot")
	addEndpoint(filepath.FromSlash("./.github/index.html"), "/relativeOSWithDot")
	addEndpoint(".github/index.html", "/relative")
	addEndpoint(filepath.FromSlash(".github/index.html"), "/relativeOS")

	// absolute paths
	if path, err := filepath.Abs(".github/index.html"); err != nil {
		require.NoError(t, err)
	} else {
		addEndpoint(path, "/absolute")
		addEndpoint(filepath.FromSlash(path), "/absoluteOS") // os related
	}

	for _, endpoint := range endpointsForTest {
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()
			// 1st try
			resp, err := app.Test(httptest.NewRequest(MethodGet, endpoint, http.NoBody))
			require.NoError(t, err)
			require.Equal(t, StatusOK, resp.StatusCode)
			// 2nd try
			resp, err = app.Test(httptest.NewRequest(MethodGet, endpoint, http.NoBody))
			require.NoError(t, err)
			require.Equal(t, StatusOK, resp.StatusCode)
		})
	}
}

// go test -race -run Test_Ctx_SendFile_RestoreOriginalURL
func Test_Ctx_SendFile_RestoreOriginalURL(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		originalURL := utils.CopyString(c.OriginalURL())
		err := c.SendFile("ctx.go")
		require.Equal(t, originalURL, c.OriginalURL())
		return err
	})

	_, err1 := app.Test(httptest.NewRequest(MethodGet, "/?test=true", http.NoBody))
	// second request required to confirm with zero allocation
	_, err2 := app.Test(httptest.NewRequest(MethodGet, "/?test=true", http.NoBody))

	require.NoError(t, err1)
	require.NoError(t, err2)
}

func Test_SendFile_withRoutes(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/file", func(c Ctx) error {
		return c.SendFile("ctx.go")
	})

	app.Get("/file/download", func(c Ctx) error {
		return c.SendFile("ctx.go", SendFile{
			Download: true,
		})
	})

	app.Get("/file/fs", func(c Ctx) error {
		return c.SendFile("ctx.go", SendFile{
			FS: os.DirFS("."),
		})
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/file", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/file/download", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	require.Equal(t, "attachment", resp.Header.Get(HeaderContentDisposition))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/file/fs", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
}

func Test_SendFile_ByteRange(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("SendFile byte-range tests are flaky on Windows")
	}

	content := []byte("0123456789")
	tmpDir := t.TempDir()
	fixture := filepath.Join(tmpDir, "fixture.txt")
	require.NoError(t, os.WriteFile(fixture, content, 0o600))

	app := New()

	app.Get("/range", func(c Ctx) error {
		return c.SendFile(fixture, SendFile{ByteRange: true})
	})

	app.Get("/norange", func(c Ctx) error {
		return c.SendFile(fixture)
	})

	t.Run("satisfiable single range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/range", http.NoBody)
		req.Header.Set(HeaderRange, "bytes=0-4")

		resp, err := app.Test(req)
		require.NoError(t, err, "app.Test(req)")
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()
		require.Equal(t, StatusPartialContent, resp.StatusCode)
		require.Equal(t, "bytes", resp.Header.Get(HeaderAcceptRanges))
		require.Equal(t, "bytes 0-4/10", resp.Header.Get(HeaderContentRange))
		require.EqualValues(t, len(content[:5]), resp.ContentLength)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, content[:5], body)
	})

	t.Run("single byte range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/range", http.NoBody)
		req.Header.Set(HeaderRange, "bytes=4-4")

		resp, err := app.Test(req)
		require.NoError(t, err, "app.Test(req)")
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()
		require.Equal(t, StatusPartialContent, resp.StatusCode)
		require.Equal(t, "bytes", resp.Header.Get(HeaderAcceptRanges))
		require.Equal(t, "bytes 4-4/10", resp.Header.Get(HeaderContentRange))
		require.EqualValues(t, 1, resp.ContentLength)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, content[4:5], body)
	})

	t.Run("open ended range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/range", http.NoBody)
		req.Header.Set(HeaderRange, "bytes=4-")

		resp, err := app.Test(req)
		require.NoError(t, err, "app.Test(req)")
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()
		require.Equal(t, StatusPartialContent, resp.StatusCode)
		require.Equal(t, "bytes", resp.Header.Get(HeaderAcceptRanges))
		require.Equal(t, "bytes 4-9/10", resp.Header.Get(HeaderContentRange))
		require.EqualValues(t, len(content[4:]), resp.ContentLength)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, content[4:], body)
	})

	t.Run("range exceeding end", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/range", http.NoBody)
		req.Header.Set(HeaderRange, "bytes=5-20")

		resp, err := app.Test(req)
		require.NoError(t, err, "app.Test(req)")
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()
		require.Equal(t, StatusPartialContent, resp.StatusCode)
		require.Equal(t, "bytes", resp.Header.Get(HeaderAcceptRanges))
		require.Equal(t, "bytes 5-9/10", resp.Header.Get(HeaderContentRange))
		require.EqualValues(t, len(content[5:]), resp.ContentLength)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, content[5:], body)
	})

	t.Run("suffix range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/range", http.NoBody)
		req.Header.Set(HeaderRange, "bytes=-3")

		resp, err := app.Test(req)
		require.NoError(t, err, "app.Test(req)")
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()
		require.Equal(t, StatusPartialContent, resp.StatusCode)
		require.Equal(t, "bytes", resp.Header.Get(HeaderAcceptRanges))
		require.Equal(t, "bytes 7-9/10", resp.Header.Get(HeaderContentRange))
		require.EqualValues(t, len(content[len(content)-3:]), resp.ContentLength)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, content[len(content)-3:], body)
	})

	t.Run("suffix range exceeding size", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/range", http.NoBody)
		req.Header.Set(HeaderRange, "bytes=-20")

		resp, err := app.Test(req)
		require.NoError(t, err, "app.Test(req)")
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()
		require.Equal(t, StatusPartialContent, resp.StatusCode)
		require.Equal(t, "bytes", resp.Header.Get(HeaderAcceptRanges))
		require.Equal(t, "bytes 0-9/10", resp.Header.Get(HeaderContentRange))
		require.EqualValues(t, len(content), resp.ContentLength)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, content, body)
	})

	t.Run("unsatisfiable range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/range", http.NoBody)
		req.Header.Set(HeaderRange, "bytes=1000-2000")

		resp, err := app.Test(req)
		require.NoError(t, err, "app.Test(req)")
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()
		require.Equal(t, StatusRequestedRangeNotSatisfiable, resp.StatusCode)
		require.Equal(t, "bytes */10", resp.Header.Get(HeaderContentRange))
	})

	t.Run("unsatisfiable reversed range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/range", http.NoBody)
		req.Header.Set(HeaderRange, "bytes=6-5")

		resp, err := app.Test(req)
		require.NoError(t, err, "app.Test(req)")
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()
		require.Equal(t, StatusRequestedRangeNotSatisfiable, resp.StatusCode)
		require.Equal(t, "bytes */10", resp.Header.Get(HeaderContentRange))
	})

	t.Run("unsatisfiable start past end", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/range", http.NoBody)
		req.Header.Set(HeaderRange, "bytes=10-")

		resp, err := app.Test(req)
		require.NoError(t, err, "app.Test(req)")
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()
		require.Equal(t, StatusRequestedRangeNotSatisfiable, resp.StatusCode)
		require.Equal(t, "bytes */10", resp.Header.Get(HeaderContentRange))
	})

	t.Run("range ignored when byte range disabled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/norange", http.NoBody)
		req.Header.Set(HeaderRange, "bytes=0-4")

		resp, err := app.Test(req)
		require.NoError(t, err, "app.Test(req)")
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()
		require.Equal(t, StatusOK, resp.StatusCode)
		require.Empty(t, resp.Header.Get(HeaderAcceptRanges))
		require.Empty(t, resp.Header.Get(HeaderContentRange))
		require.EqualValues(t, len(content), resp.ContentLength)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, content, body)
	})
}

func Benchmark_Ctx_SendFile(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.SendFile("ctx.go")
	}

	require.NoError(b, err)
	require.Contains(b, string(c.Response().Body()), "type DefaultCtx struct")
}

// go test -run Test_Ctx_JSON
func Test_Ctx_JSON(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.Error(t, c.JSON(complex(1, 1)))

	// Test without ctype
	err := c.JSON(Map{ // map has no order
		"Name": "Grame",
		"Age":  20,
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"Age":20,"Name":"Grame"}`, string(c.Response().Body()))
	require.Equal(t, "application/json; charset=utf-8", string(c.Response().Header.Peek("content-type")))

	// Test with ctype
	err = c.JSON(Map{ // map has no order
		"Name": "Grame",
		"Age":  20,
	}, "application/problem+json")
	require.NoError(t, err)
	require.JSONEq(t, `{"Age":20,"Name":"Grame"}`, string(c.Response().Body()))
	require.Equal(t, "application/problem+json", string(c.Response().Header.Peek("content-type")))

	testEmpty := func(v any, r string) {
		err := c.JSON(v)
		require.NoError(t, err)
		require.Equal(t, r, string(c.Response().Body()))
	}

	testEmpty(nil, "null")
	testEmpty("", `""`)
	testEmpty(0, "0")
	testEmpty([]int{}, "[]")

	t.Run("custom json encoder", func(t *testing.T) {
		t.Parallel()

		app := New(Config{
			JSONEncoder: func(_ any) ([]byte, error) {
				return []byte(`["custom","json"]`), nil
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})

		err := c.JSON(Map{ // map has no order
			"Name": "Grame",
			"Age":  20,
		})
		require.NoError(t, err)
		require.Equal(t, `["custom","json"]`, string(c.Response().Body()))
		require.Equal(t, "application/json; charset=utf-8", string(c.Response().Header.Peek("content-type")))
	})
}

// go test -run=^$ -bench=Benchmark_Ctx_JSON -benchmem -count=4
func Benchmark_Ctx_JSON(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	type SomeStruct struct {
		Name string
		Age  uint8
	}
	data := SomeStruct{
		Name: "Grame",
		Age:  20,
	}
	var err error
	b.ReportAllocs()
	for b.Loop() {
		err = c.JSON(data)
	}
	require.NoError(b, err)
	require.JSONEq(b, `{"Name":"Grame","Age":20}`, string(c.Response().Body()))
}

// go test -run Test_Ctx_MsgPack
func Test_Ctx_MsgPack(t *testing.T) {
	t.Parallel()

	app := New(Config{
		MsgPackEncoder: msgpack.Marshal,
		MsgPackDecoder: msgpack.Unmarshal,
	})

	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.MsgPack(complex(1, 1))

	require.NoError(t, err)
	require.Equal(t, "\u0600?\xf0\x00\x00\x00\x00\x00\x00?\xf0\x00\x00\x00\x00\x00\x00", string(c.Response().Body()))

	// Test without ctype
	err = c.MsgPack(Map{ // map has no order
		"Name": "Grame",
	})
	require.NoError(t, err)
	require.Equal(t, "\x81\xa4Name\xa5Grame", string(c.Response().Body()))
	require.Equal(t, MIMEApplicationMsgPack, string(c.Response().Header.Peek("content-type")))

	// Test with ctype
	err = c.MsgPack(Map{ // map has no order
		"Name": "Grame",
	}, "application/problem+msgpack")
	require.NoError(t, err)
	require.Equal(t, "\x81\xa4Name\xa5Grame", string(c.Response().Body()))
	require.Equal(t, "application/problem+msgpack", string(c.Response().Header.Peek("content-type")))

	testEmpty := func(v any, r string) {
		err := c.MsgPack(v)
		require.NoError(t, err)
		require.Equal(t, r, string(c.Response().Body()))
	}

	testEmpty(nil, "\xc0")
	testEmpty("", "\xa0")
	testEmpty(0, "\x00")
	testEmpty([]int{}, "\x90")

	t.Run("custom msgpack encoder", func(t *testing.T) {
		t.Parallel()

		app := New(Config{
			MsgPackEncoder: func(_ any) ([]byte, error) {
				return []byte(`["custom","msgpack"]`), nil
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})

		err := c.MsgPack(Map{ // map has no order
			"Name": "Grame",
			"Age":  20,
		})
		require.NoError(t, err)
		require.Equal(t, `["custom","msgpack"]`, string(c.Response().Body()))
		require.Equal(t, MIMEApplicationMsgPack, string(c.Response().Header.Peek("content-type")))
	})

	t.Run("error msgpack", func(t *testing.T) {
		t.Parallel()

		app := New(Config{
			MsgPackEncoder: func(_ any) ([]byte, error) {
				return []byte("error"), errors.New("msgpack error")
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})

		err := c.MsgPack(Map{ // map has no order
			"Name": "Grame",
			"Age":  20,
		})
		require.Error(t, err)
	})
}

// go test -run=^$ -bench=Benchmark_Ctx_MsgPack -benchmem -count=4
func Benchmark_Ctx_MsgPack(b *testing.B) {
	app := New(
		Config{
			MsgPackEncoder: msgpack.Marshal,
			MsgPackDecoder: msgpack.Unmarshal,
		},
	)
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	type SomeStruct struct {
		Name string
		Age  uint8
	}
	data := SomeStruct{
		Name: "Grame",
		Age:  20,
	}
	var err error
	b.ReportAllocs()
	for b.Loop() {
		err = c.MsgPack(data)
	}
	require.NoError(b, err)
	require.Equal(b, "\x82\xa4Name\xa5Grame\xa3Age\x14", string(c.Response().Body()))
}

// go test -run Test_Ctx_CBOR
func Test_Ctx_CBOR(t *testing.T) {
	t.Parallel()
	app := New(Config{
		CBOREncoder: cbor.Marshal,
		CBORDecoder: cbor.Unmarshal,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.Error(t, c.CBOR(complex(1, 1)))

	type dummyStruct struct {
		Name string
		Age  int
	}

	// Test without ctype
	err := c.CBOR(dummyStruct{ // map has no order
		Name: "Grame",
		Age:  20,
	})
	require.NoError(t, err)
	require.Equal(t, `a2644e616d65654772616d656341676514`, hex.EncodeToString(c.Response().Body()))
	require.Equal(t, "application/cbor", string(c.Response().Header.Peek("content-type")))

	// Test with ctype
	err = c.CBOR(dummyStruct{ // map has no order
		Name: "Grame",
		Age:  20,
	}, "application/problem+cbor")
	require.NoError(t, err)
	require.Equal(t, `a2644e616d65654772616d656341676514`, hex.EncodeToString(c.Response().Body()))
	require.Equal(t, "application/problem+cbor", string(c.Response().Header.Peek("content-type")))

	testEmpty := func(v any, r string) {
		cbErr := c.CBOR(v)
		require.NoError(t, cbErr)
		require.Equal(t, r, hex.EncodeToString(c.Response().Body()))
	}

	testEmpty(nil, "f6")
	testEmpty("", `60`)
	testEmpty(0, "00")
	testEmpty([]int{}, "80")

	// Test invalid types
	err = c.CBOR(make(chan int))
	require.Error(t, err)

	err = c.CBOR(func() {})
	require.Error(t, err)

	t.Run("custom cbor encoder", func(t *testing.T) {
		t.Parallel()

		app := New(Config{
			CBOREncoder: func(_ any) ([]byte, error) {
				return []byte(hex.EncodeToString([]byte("random"))), nil
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})

		err := c.CBOR(Map{ // map has no order
			"Name": "Grame",
			"Age":  20,
		})
		require.NoError(t, err)
		require.Equal(t, `72616e646f6d`, string(c.Response().Body()))
		require.Equal(t, "application/cbor", string(c.Response().Header.Peek("content-type")))
	})
}

// go test -run=^$ -bench=Benchmark_Ctx_CBOR -benchmem -count=4
func Benchmark_Ctx_CBOR(b *testing.B) {
	app := New(Config{
		CBOREncoder: cbor.Marshal,
		CBORDecoder: cbor.Unmarshal,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	type SomeStruct struct {
		Name string
		Age  uint8
	}
	data := SomeStruct{
		Name: "Grame",
		Age:  20,
	}
	var err error
	b.ReportAllocs()
	for b.Loop() {
		err = c.CBOR(data)
	}
	require.NoError(b, err)
	require.Equal(b, `a2644e616d65654772616d656341676514`, hex.EncodeToString(c.Response().Body()))
}

// go test -run=^$ -bench=Benchmark_Ctx_JSON_Ctype -benchmem -count=4
func Benchmark_Ctx_JSON_Ctype(b *testing.B) {
	app := New()
	// TODO: Check extra allocs because of the interface stuff
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed
	type SomeStruct struct {
		Name string
		Age  uint8
	}
	data := SomeStruct{
		Name: "Grame",
		Age:  20,
	}
	var err error
	b.ReportAllocs()
	for b.Loop() {
		err = c.JSON(data, "application/problem+json")
	}
	require.NoError(b, err)
	require.JSONEq(b, `{"Name":"Grame","Age":20}`, string(c.Response().Body()))
	require.Equal(b, "application/problem+json", string(c.Response().Header.Peek("content-type")))
}

// go test -run Test_Ctx_JSONP
func Test_Ctx_JSONP(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.Error(t, c.JSONP(complex(1, 1)))

	err := c.JSONP(Map{
		"Name": "Grame",
		"Age":  20,
	})
	require.NoError(t, err)
	require.Equal(t, `callback({"Age":20,"Name":"Grame"});`, string(c.Response().Body()))
	require.Equal(t, "text/javascript; charset=utf-8", string(c.Response().Header.Peek("content-type")))

	err = c.Res().JSONP(Map{
		"Name": "Grame",
		"Age":  20,
	}, "john")
	require.NoError(t, err)
	require.Equal(t, `john({"Age":20,"Name":"Grame"});`, string(c.Response().Body()))
	require.Equal(t, "text/javascript; charset=utf-8", string(c.Response().Header.Peek("content-type")))

	t.Run("custom json encoder", func(t *testing.T) {
		t.Parallel()

		app := New(Config{
			JSONEncoder: func(_ any) ([]byte, error) {
				return []byte(`["custom","json"]`), nil
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})

		err := c.JSONP(Map{ // map has no order
			"Name": "Grame",
			"Age":  20,
		})
		require.NoError(t, err)
		require.Equal(t, `callback(["custom","json"]);`, string(c.Response().Body()))
		require.Equal(t, "text/javascript; charset=utf-8", string(c.Response().Header.Peek("content-type")))
	})
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_JSONP -benchmem -count=4
func Benchmark_Ctx_JSONP(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	type SomeStruct struct {
		Name string
		Age  uint8
	}
	data := SomeStruct{
		Name: "Grame",
		Age:  20,
	}
	callback := "emit"
	var err error
	b.ReportAllocs()
	for b.Loop() {
		err = c.JSONP(data, callback)
	}
	require.NoError(b, err)
	require.Equal(b, `emit({"Name":"Grame","Age":20});`, string(c.Response().Body()))
}

// go test -run Test_Ctx_XML
func Test_Ctx_XML(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	require.Error(t, c.JSON(complex(1, 1)))

	type xmlResult struct {
		XMLName xml.Name `xml:"Users"`
		Names   []string `xml:"Names"`
		Ages    []int    `xml:"Ages"`
	}

	err := c.XML(xmlResult{
		Names: []string{"Grame", "John"},
		Ages:  []int{1, 12, 20},
	})
	require.NoError(t, err)
	require.Equal(t, `<Users><Names>Grame</Names><Names>John</Names><Ages>1</Ages><Ages>12</Ages><Ages>20</Ages></Users>`, string(c.Response().Body()))
	require.Equal(t, "application/xml; charset=utf-8", string(c.Response().Header.Peek("content-type")))

	testEmpty := func(v any, r string) {
		err := c.XML(v)
		require.NoError(t, err)
		require.Equal(t, r, string(c.Response().Body()))
	}

	testEmpty(nil, "")
	testEmpty("", `<string></string>`)
	testEmpty(0, "<int>0</int>")
	testEmpty([]int{}, "")

	t.Run("custom xml encoder", func(t *testing.T) {
		t.Parallel()

		app := New(Config{
			XMLEncoder: func(_ any) ([]byte, error) {
				return []byte(`<custom>xml</custom>`), nil
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})

		type xmlResult struct {
			XMLName xml.Name `xml:"Users"`
			Names   []string `xml:"Names"`
			Ages    []int    `xml:"Ages"`
		}

		err := c.XML(xmlResult{
			Names: []string{"Grame", "John"},
			Ages:  []int{1, 12, 20},
		})

		require.NoError(t, err)
		require.Equal(t, `<custom>xml</custom>`, string(c.Response().Body()))
		require.Equal(t, "application/xml; charset=utf-8", string(c.Response().Header.Peek("content-type")))
	})
}

// go test -run=^$ -bench=Benchmark_Ctx_XML -benchmem -count=4
func Benchmark_Ctx_XML(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed
	type SomeStruct struct {
		Name string `xml:"Name"`
		Age  uint8  `xml:"Age"`
	}
	data := SomeStruct{
		Name: "Grame",
		Age:  20,
	}
	var err error
	b.ReportAllocs()
	for b.Loop() {
		err = c.XML(data)
	}

	require.NoError(b, err)
	require.Equal(b, `<SomeStruct><Name>Grame</Name><Age>20</Age></SomeStruct>`, string(c.Response().Body()))
}

// go test -run Test_Ctx_Links
func Test_Ctx_Links(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Links()
	require.Empty(t, string(c.Response().Header.Peek(HeaderLink)))

	c.Links(
		"http://api.example.com/users?page=2", "next",
		"http://api.example.com/users?page=5", "last",
	)
	require.Equal(t, `<http://api.example.com/users?page=2>; rel="next",<http://api.example.com/users?page=5>; rel="last"`, string(c.Response().Header.Peek(HeaderLink)))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Links -benchmem -count=4
func Benchmark_Ctx_Links(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	b.ReportAllocs()
	for b.Loop() {
		c.Links(
			"http://api.example.com/users?page=2", "next",
			"http://api.example.com/users?page=5", "last",
		)
	}
}

// go test -run Test_Ctx_Location
func Test_Ctx_Location(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Location("http://example.com")
	require.Equal(t, "http://example.com", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_Next
func Test_Ctx_Next(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use("/", func(c Ctx) error {
		return c.Next()
	})
	app.Get("/test", func(c Ctx) error {
		c.Set("X-Next-Result", "Works")
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "http://example.com/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	require.Equal(t, "Works", resp.Header.Get("X-Next-Result"))
}

// go test -run Test_Ctx_Next_Error
func Test_Ctx_Next_Error(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use("/", func(c Ctx) error {
		c.Set("X-Next-Result", "Works")
		return ErrNotFound
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "http://example.com/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")
	require.Equal(t, "Works", resp.Header.Get("X-Next-Result"))
}

// go test -run Test_Ctx_Render
func Test_Ctx_Render(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Render("./.github/testdata/index.tmpl", Map{
		"Title": "Hello, World!",
	})
	require.NoError(t, err)

	require.Equal(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))

	err = c.Render("./.github/testdata/template-non-exists.html", nil)
	require.Error(t, err)

	err = c.Res().Render("./.github/testdata/template-invalid.html", nil)
	require.Error(t, err)
}

func Test_Ctx_RenderWithoutLocals(t *testing.T) {
	t.Parallel()
	app := New(Config{
		PassLocalsToViews: false,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Locals("Title", "Hello, World!")

	err := c.Render("./.github/testdata/index.tmpl", Map{})
	require.NoError(t, err)
	require.Equal(t, "<h1></h1>", string(c.Response().Body()))
}

func Test_Ctx_RenderWithLocals(t *testing.T) {
	t.Parallel()
	app := New(Config{
		PassLocalsToViews: true,
	})

	t.Run("EmptyBind", func(t *testing.T) {
		t.Parallel()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})

		c.Locals("Title", "Hello, World!")
		err := c.Render("./.github/testdata/index.tmpl", Map{})

		require.NoError(t, err)
		require.Equal(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
	})

	t.Run("NilBind", func(t *testing.T) {
		t.Parallel()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})

		c.Locals("Title", "Hello, World!")
		err := c.Render("./.github/testdata/index.tmpl", nil)

		require.NoError(t, err)
		require.Equal(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
	})
}

func Test_Ctx_Matched_AfterNext(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use(func(c Ctx) error {
		require.False(t, c.Matched())
		err := c.Next()
		if c.Path() == "/one" {
			require.True(t, c.Matched())
		} else {
			require.False(t, c.Matched())
		}
		return err
	})

	app.Get("/one", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/one", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/missing", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

func Test_Ctx_Matched_RouteError(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ErrorHandler: func(c Ctx, err error) error {
			require.True(t, c.Matched())
			return c.Status(StatusNotFound).SendString(err.Error())
		},
	})

	app.Get("/", func(_ Ctx) error {
		return ErrNotFound
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

func Test_Ctx_IsMiddleware(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use(func(c Ctx) error {
		require.True(t, c.IsMiddleware())
		return c.Next()
	})

	app.Get("/", func(c Ctx) error {
		require.False(t, c.IsMiddleware())
		return c.SendStatus(StatusOK)
	})

	app.Get("/route", func(c Ctx) error {
		require.True(t, c.IsMiddleware())
		return c.Next()
	}, func(c Ctx) error {
		require.False(t, c.IsMiddleware())
		return c.SendStatus(StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/route", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
}

func Test_Ctx_HasBody(t *testing.T) {
	t.Parallel()
	app := New()

	acquire := func(t *testing.T) CustomCtx {
		t.Helper()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		require.NotNil(t, ctx)
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		return ctx
	}

	setTransferEncoding := func(t *testing.T, ctx Ctx, value string) {
		t.Helper()
		hdr := &ctx.Request().Header
		hdr.DisableSpecialHeader()
		hdr.Set(HeaderTransferEncoding, value)
		hdr.Set(HeaderContentLength, "0")
		hdr.EnableSpecialHeader()
		require.Zero(t, hdr.ContentLength())
		require.Empty(t, ctx.Request().Body())
	}

	t.Run("body bytes", func(t *testing.T) {
		t.Parallel()
		ctx := acquire(t)
		ctx.Request().SetBody([]byte("test"))
		require.True(t, ctx.HasBody())
	})

	t.Run("content length header", func(t *testing.T) {
		t.Parallel()
		ctx := acquire(t)
		ctx.Request().Header.SetContentLength(4)
		require.True(t, ctx.HasBody())
	})

	t.Run("chunked sentinel", func(t *testing.T) {
		t.Parallel()
		ctx := acquire(t)
		ctx.Request().Header.SetContentLength(-1)
		require.Equal(t, -1, ctx.Request().Header.ContentLength())
		require.Empty(t, ctx.Request().Body())
		require.True(t, ctx.HasBody())
	})

	t.Run("transfer encoding chunked", func(t *testing.T) {
		t.Parallel()
		ctx := acquire(t)
		setTransferEncoding(t, ctx, "chunked")
		require.True(t, ctx.HasBody())
	})

	t.Run("transfer encoding whitespace", func(t *testing.T) {
		t.Parallel()
		ctx := acquire(t)
		setTransferEncoding(t, ctx, "  ChUnKeD  ")
		require.True(t, ctx.HasBody())
	})

	t.Run("transfer encoding parameters", func(t *testing.T) {
		t.Parallel()
		ctx := acquire(t)
		setTransferEncoding(t, ctx, "chunked; q=1")
		require.True(t, ctx.HasBody())
	})

	t.Run("transfer encoding multiple values", func(t *testing.T) {
		t.Parallel()
		ctx := acquire(t)
		setTransferEncoding(t, ctx, "gzip, chunked")
		require.True(t, ctx.HasBody())
	})

	t.Run("transfer encoding identity", func(t *testing.T) {
		t.Parallel()
		ctx := acquire(t)
		setTransferEncoding(t, ctx, "identity")
		require.False(t, ctx.HasBody())
	})

	t.Run("transfer encoding identity then chunked", func(t *testing.T) {
		t.Parallel()
		ctx := acquire(t)
		setTransferEncoding(t, ctx, "identity, chunked")
		require.True(t, ctx.HasBody())
	})

	t.Run("no body", func(t *testing.T) {
		t.Parallel()
		ctx := acquire(t)
		require.False(t, ctx.HasBody())
	})
}

func Test_Ctx_IsWebSocket(t *testing.T) {
	t.Parallel()
	app := New()

	ws := app.AcquireCtx(&fasthttp.RequestCtx{})
	require.NotNil(t, ws)
	t.Cleanup(func() { app.ReleaseCtx(ws) })
	ws.Request().Header.Set(HeaderConnection, "keep-alive, Upgrade")
	ws.Request().Header.Set(HeaderUpgrade, "websocket")
	require.True(t, ws.IsWebSocket())

	non := app.AcquireCtx(&fasthttp.RequestCtx{})
	require.NotNil(t, non)
	t.Cleanup(func() { app.ReleaseCtx(non) })
	non.Request().Header.Set(HeaderConnection, "not-an-upgrade")
	non.Request().Header.Set(HeaderUpgrade, "websocket")
	require.False(t, non.IsWebSocket())
}

func Test_Ctx_IsPreflight(t *testing.T) {
	t.Parallel()
	app := New()

	preCtx := &fasthttp.RequestCtx{}
	preCtx.Request.Header.SetMethod(MethodOptions)
	preCtx.Request.Header.Set(HeaderAccessControlRequestMethod, MethodGet)
	preCtx.Request.Header.Set(HeaderOrigin, "https://example.com")
	pre := app.AcquireCtx(preCtx)
	require.NotNil(t, pre)
	t.Cleanup(func() { app.ReleaseCtx(pre) })
	require.True(t, pre.IsPreflight())

	noOriginCtx := &fasthttp.RequestCtx{}
	noOriginCtx.Request.Header.SetMethod(MethodOptions)
	noOriginCtx.Request.Header.Set(HeaderAccessControlRequestMethod, MethodGet)
	noOrigin := app.AcquireCtx(noOriginCtx)
	require.NotNil(t, noOrigin)
	t.Cleanup(func() { app.ReleaseCtx(noOrigin) })
	require.False(t, noOrigin.IsPreflight())

	optCtx := &fasthttp.RequestCtx{}
	optCtx.Request.Header.SetMethod(MethodOptions)
	optCtx.Request.Header.Set(HeaderOrigin, "https://example.com")
	opt := app.AcquireCtx(optCtx)
	require.NotNil(t, opt)
	t.Cleanup(func() { app.ReleaseCtx(opt) })
	require.False(t, opt.IsPreflight())
}

func Test_Ctx_RenderWithViewBind(t *testing.T) {
	t.Parallel()

	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.ViewBind(Map{
		"Title": "Hello, World!",
	})
	require.NoError(t, err)

	err = c.Render("./.github/testdata/index.tmpl", Map{})
	require.NoError(t, err)
	buf := bytebufferpool.Get()
	buf.WriteString("overwrite")
	defer bytebufferpool.Put(buf)

	require.NoError(t, err)
	require.Equal(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

func Test_Ctx_RenderWithOverwrittenViewBind(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.ViewBind(Map{
		"Title": "Hello, World!",
	})
	require.NoError(t, err)

	err = c.Render("./.github/testdata/index.tmpl", Map{
		"Title": "Hello from Fiber!",
	})
	require.NoError(t, err)

	buf := bytebufferpool.Get()
	buf.WriteString("overwrite")
	defer bytebufferpool.Put(buf)

	require.Equal(t, "<h1>Hello from Fiber!</h1>", string(c.Response().Body()))
}

func Test_Ctx_RenderWithViewBindLocals(t *testing.T) {
	t.Parallel()
	app := New(Config{
		PassLocalsToViews: true,
	})

	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.ViewBind(Map{
		"Title": "Hello, World!",
	})
	require.NoError(t, err)

	c.Locals("Summary", "Test")

	err = c.Render("./.github/testdata/template.tmpl", Map{})
	require.NoError(t, err)
	require.Equal(t, "<h1>Hello, World! Test</h1>", string(c.Response().Body()))

	require.Equal(t, "<h1>Hello, World! Test</h1>", string(c.Response().Body()))
}

func Test_Ctx_RenderWithLocalsAndBinding(t *testing.T) {
	t.Parallel()
	engine := &testTemplateEngine{}
	err := engine.Load()
	require.NoError(t, err)

	app := New(Config{
		PassLocalsToViews: true,
		Views:             engine,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Locals("Title", "This is a test.")

	err = c.Render("index.tmpl", Map{
		"Title": "Hello, World!",
	})

	require.NoError(t, err)
	require.Equal(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

func Benchmark_Ctx_RenderWithLocalsAndViewBind(b *testing.B) {
	engine := &testTemplateEngine{}
	err := engine.Load()
	require.NoError(b, err)
	app := New(Config{
		PassLocalsToViews: true,
		Views:             engine,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err = c.ViewBind(Map{
		"Title": "Hello, World!",
	})
	require.NoError(b, err)
	c.Locals("Summary", "Test")

	b.ReportAllocs()

	for b.Loop() {
		err = c.Render("template.tmpl", Map{})
	}

	require.NoError(b, err)
	require.Equal(b, "<h1>Hello, World! Test</h1>", string(c.Response().Body()))
}

func Benchmark_Ctx_RenderLocals(b *testing.B) {
	engine := &testTemplateEngine{}
	err := engine.Load()
	require.NoError(b, err)
	app := New(Config{
		PassLocalsToViews: true,
	})
	app.config.Views = engine
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Locals("Title", "Hello, World!")

	b.ReportAllocs()

	for b.Loop() {
		err = c.Render("index.tmpl", Map{})
	}

	require.NoError(b, err)
	require.Equal(b, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

func Benchmark_Ctx_RenderViewBind(b *testing.B) {
	engine := &testTemplateEngine{}
	err := engine.Load()
	require.NoError(b, err)
	app := New()
	app.config.Views = engine
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err = c.ViewBind(Map{
		"Title": "Hello, World!",
	})
	require.NoError(b, err)

	b.ReportAllocs()

	for b.Loop() {
		err = c.Render("index.tmpl", Map{})
	}

	require.NoError(b, err)
	require.Equal(b, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

// go test -run Test_Ctx_RestartRouting
func Test_Ctx_RestartRouting(t *testing.T) {
	t.Parallel()
	app := New()
	calls := 0
	app.Get("/", func(c Ctx) error {
		calls++
		if calls < 3 {
			return c.RestartRouting()
		}
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "http://example.com/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	require.Equal(t, 3, calls, "Number of calls")
}

// go test -run Test_Ctx_RestartRoutingWithChangedPath
func Test_Ctx_RestartRoutingWithChangedPath(t *testing.T) {
	t.Parallel()
	app := New()
	var executedOldHandler, executedNewHandler bool

	app.Get("/old", func(c Ctx) error {
		c.Path("/new")
		return c.RestartRouting()
	})
	app.Get("/old", func(_ Ctx) error {
		executedOldHandler = true
		return nil
	})
	app.Get("/new", func(_ Ctx) error {
		executedNewHandler = true
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "http://example.com/old", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	require.False(t, executedOldHandler, "Executed old handler")
	require.True(t, executedNewHandler, "Executed new handler")
}

// go test -run Test_Ctx_RestartRoutingWithChangedPathAnd404
func Test_Ctx_RestartRoutingWithChangedPathAndCatchAll(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/new", func(_ Ctx) error {
		return nil
	})
	app.Use(func(c Ctx) error {
		c.Path("/new")
		// c.Next() would fail this test as a 404 is returned from the next handler
		return c.RestartRouting()
	})
	app.Use(func(_ Ctx) error {
		return ErrNotFound
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "http://example.com/old", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

type testTemplateEngine struct {
	templates *template.Template
	path      string
}

func (t *testTemplateEngine) Render(w io.Writer, name string, bind any, layout ...string) error {
	if len(layout) == 0 {
		if err := t.templates.ExecuteTemplate(w, name, bind); err != nil {
			return fmt.Errorf("failed to execute template without layout: %w", err)
		}
		return nil
	}
	if err := t.templates.ExecuteTemplate(w, name, bind); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}
	if err := t.templates.ExecuteTemplate(w, layout[0], bind); err != nil {
		return fmt.Errorf("failed to execute template with layout: %w", err)
	}
	return nil
}

func (t *testTemplateEngine) Load() error {
	if t.path == "" {
		t.path = "testdata"
	}
	t.templates = template.Must(template.ParseGlob("./.github/" + t.path + "/*.tmpl"))
	return nil
}

// go test -run Test_Ctx_Render_Engine
func Test_Ctx_Render_Engine(t *testing.T) {
	t.Parallel()
	engine := &testTemplateEngine{}
	require.NoError(t, engine.Load())
	app := New()
	app.config.Views = engine
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Render("index.tmpl", Map{
		"Title": "Hello, World!",
	})
	require.NoError(t, err)
	require.Equal(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

// go test -run Test_Ctx_Render_Engine_With_View_Layout
func Test_Ctx_Render_Engine_With_View_Layout(t *testing.T) {
	t.Parallel()
	engine := &testTemplateEngine{}
	require.NoError(t, engine.Load())
	app := New(Config{ViewsLayout: "main.tmpl"})
	app.config.Views = engine
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Render("index.tmpl", Map{
		"Title": "Hello, World!",
	})
	require.NoError(t, err)
	require.Equal(t, "<h1>Hello, World!</h1><h1>I'm main</h1>", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Render_Engine -benchmem -count=4
func Benchmark_Ctx_Render_Engine(b *testing.B) {
	engine := &testTemplateEngine{}
	err := engine.Load()
	require.NoError(b, err)
	app := New()
	app.config.Views = engine
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	b.ReportAllocs()
	for b.Loop() {
		err = c.Render("index.tmpl", Map{
			"Title": "Hello, World!",
		})
	}
	require.NoError(b, err)
	require.Equal(b, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Get_Location_From_Route -benchmem -count=4
func Benchmark_Ctx_Get_Location_From_Route(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	app.Get("/user/:name", func(c Ctx) error {
		return c.SendString(c.Params("name"))
	}).Name("User")

	var err error
	var location string
	for b.Loop() {
		route := app.GetRoute("User")
		location, err = c.getLocationFromRoute(&route, Map{"name": "fiber"})
	}

	require.Equal(b, "/user/fiber", location)
	require.NoError(b, err)
}

// go test -run Test_Ctx_Get_Location_From_Route_name
func Test_Ctx_Get_Location_From_Route_name(t *testing.T) {
	t.Parallel()

	t.Run("case-insensitive", func(t *testing.T) {
		t.Parallel()
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		app.Get("/user/:name", func(c Ctx) error {
			return c.SendString(c.Params("name"))
		}).Name("User")

		location, err := c.GetRouteURL("User", Map{"name": "fiber"})
		require.NoError(t, err)
		require.Equal(t, "/user/fiber", location)

		location, err = c.GetRouteURL("User", Map{"Name": "fiber"})
		require.NoError(t, err)
		require.Equal(t, "/user/fiber", location)
	})

	t.Run("case-sensitive", func(t *testing.T) {
		t.Parallel()
		app := New(Config{CaseSensitive: true})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		app.Get("/user/:name", func(c Ctx) error {
			return c.SendString(c.Params("name"))
		}).Name("User")

		location, err := c.GetRouteURL("User", Map{"name": "fiber"})
		require.NoError(t, err)
		require.Equal(t, "/user/fiber", location)

		location, err = c.GetRouteURL("User", Map{"Name": "fiber"})
		require.NoError(t, err)
		require.Equal(t, "/user/", location)
	})
}

// go test -run Test_Ctx_Get_Location_From_Route_name_Optional_greedy
func Test_Ctx_Get_Location_From_Route_name_Optional_greedy(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	app.Get("/:phone/*/send/*", func(c Ctx) error {
		return c.SendString("Phone: " + c.Params("phone") + "\nFirst Param: " + c.Params("*1") + "\nSecond Param: " + c.Params("*2"))
	}).Name("SendSms")

	location, err := c.GetRouteURL("SendSms", Map{
		"phone": "23456789",
		"*1":    "sms",
		"*2":    "test-msg",
	})
	require.NoError(t, err)
	require.Equal(t, "/23456789/sms/send/test-msg", location)
}

// go test -run Test_Ctx_Get_Location_From_Route_name_Optional_greedy_one_param
func Test_Ctx_Get_Location_From_Route_name_Optional_greedy_one_param(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	app.Get("/:phone/*/send", func(c Ctx) error {
		return c.SendString("Phone: " + c.Params("phone") + "\nFirst Param: " + c.Params("*1"))
	}).Name("SendSms")

	location, err := c.GetRouteURL("SendSms", Map{
		"phone": "23456789",
		"*":     "sms",
	})
	require.NoError(t, err)
	require.Equal(t, "/23456789/sms/send", location)
}

type errorTemplateEngine struct{}

func (errorTemplateEngine) Render(_ io.Writer, _ string, _ any, _ ...string) error {
	return errors.New("errorTemplateEngine")
}

func (errorTemplateEngine) Load() error { return nil }

// go test -run Test_Ctx_Render_Engine_Error
func Test_Ctx_Render_Engine_Error(t *testing.T) {
	t.Parallel()
	app := New()
	app.config.Views = errorTemplateEngine{}
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Render("index.tmpl", nil)
	require.Error(t, err)
}

// go test -run Test_Ctx_Render_Go_Template
func Test_Ctx_Render_Go_Template(t *testing.T) {
	t.Parallel()
	file, err := os.CreateTemp(os.TempDir(), "fiber")
	require.NoError(t, err)
	defer func() {
		removeErr := os.Remove(file.Name())
		require.NoError(t, removeErr)
	}()

	_, err = file.WriteString("template")
	require.NoError(t, err)

	err = file.Close()
	require.NoError(t, err)

	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err = c.Render(file.Name(), nil)
	require.NoError(t, err)
	require.Equal(t, "template", string(c.Response().Body()))
}

// go test -run Test_Ctx_Send
func Test_Ctx_Send(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.NoError(t, c.Send([]byte("Hello, World")))
	require.NoError(t, c.Send([]byte("Don't crash please")))
	require.NoError(t, c.Send([]byte("1337")))
	require.Equal(t, "1337", string(c.Response().Body()))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Send -benchmem -count=4
func Benchmark_Ctx_Send(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	byt := []byte("Hello, World!")
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.Send(byt)
	}
	require.NoError(b, err)
	require.Equal(b, "Hello, World!", string(c.Response().Body()))
}

// go test -run Test_Ctx_SendStatus
func Test_Ctx_SendStatus(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.SendStatus(415)
	require.NoError(t, err)
	require.Equal(t, 415, c.Response().StatusCode())
	require.Equal(t, "Unsupported Media Type", string(c.Response().Body()))
}

func Test_Ctx_SendStatusNoBodyResponses(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		status int
	}{
		{
			name:   "Informational",
			status: StatusContinue,
		},
		{
			name:   "Processing",
			status: StatusProcessing,
		},
		{
			name:   "SwitchingProtocols",
			status: StatusSwitchingProtocols,
		},
		{
			name:   "EarlyHints",
			status: StatusEarlyHints,
		},
		{
			name:   "NoContent",
			status: StatusNoContent,
		},
		{
			name:   "ResetContent",
			status: StatusResetContent,
		},
		{
			name:   "NotModified",
			status: StatusNotModified,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			app := New()
			c := app.AcquireCtx(&fasthttp.RequestCtx{})

			c.Response().SetBodyString("preset body")

			err := c.SendStatus(testCase.status)
			require.NoError(t, err)
			require.Empty(t, c.Response().Body())
			require.Equal(t, 0, c.Response().Header.ContentLength())
		})
	}
}

// go test -run Test_Ctx_SendString
func Test_Ctx_SendString(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.SendString("Don't crash please")
	require.NoError(t, err)
	require.Equal(t, "Don't crash please", string(c.Response().Body()))
}

// go test -run Test_Ctx_SendStream
func Test_Ctx_SendStream(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.SendStream(bytes.NewReader([]byte("Don't crash please")))
	require.NoError(t, err)
	require.Equal(t, "Don't crash please", string(c.Response().Body()))

	err = c.SendStream(bytes.NewReader([]byte("Don't crash please")), len([]byte("Don't crash please")))
	require.NoError(t, err)
	require.Equal(t, "Don't crash please", string(c.Response().Body()))

	err = c.SendStream(bufio.NewReader(bytes.NewReader([]byte("Hello bufio"))))
	require.NoError(t, err)
	require.Equal(t, "Hello bufio", string(c.Response().Body()))
}

// go test -run Test_Ctx_SendStreamWriter
func Test_Ctx_SendStreamWriter(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.SendStreamWriter(func(w *bufio.Writer) {
		w.WriteString("Don't crash please") //nolint:errcheck // It is fine to ignore the error
	})
	require.NoError(t, err)
	require.Equal(t, "Don't crash please", string(c.Response().Body()))

	err = c.SendStreamWriter(func(w *bufio.Writer) {
		for lineNum := 1; lineNum <= 5; lineNum++ {
			fmt.Fprintf(w, "Line %d\n", lineNum)
			if flushErr := w.Flush(); flushErr != nil {
				t.Errorf("unexpected error: %s", flushErr)
				return
			}
		}
	})
	require.NoError(t, err)
	require.Equal(t, "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n", string(c.Response().Body()))

	err = c.SendStreamWriter(func(_ *bufio.Writer) {})
	require.NoError(t, err)
	require.Empty(t, c.Response().Body())
}

// go test -run Test_Ctx_SendStreamWriter_Interrupted
func Test_Ctx_SendStreamWriter_Interrupted(t *testing.T) {
	t.Parallel()
	app := New()
	var flushed atomic.Int32
	var flushErrLine atomic.Int32
	app.Get("/", func(c Ctx) error {
		return c.SendStreamWriter(func(w *bufio.Writer) {
			for lineNum := 1; lineNum <= 5; lineNum++ {
				fmt.Fprintf(w, "Line %d\n", lineNum)

				if err := w.Flush(); err != nil {
					flushErrLine.Store(int32(lineNum)) //nolint:gosec // G115 - lineNum is 1-5, fits int32
					return
				}

				if lineNum <= 3 {
					flushed.Add(1)
				}

				if lineNum == 3 {
					time.Sleep(500 * time.Millisecond)
				}
			}
		})
	})

	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	testConfig := TestConfig{
		// allow enough time for three lines to flush before
		// the test connection is closed but stop before the
		// fourth line is sent
		Timeout:       200 * time.Millisecond,
		FailOnTimeout: true, // Changed to true to test interrupted behavior
	}
	resp, err := app.Test(req, testConfig)
	// With FailOnTimeout: true, we should get a timeout error
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)
	require.Nil(t, resp)
}

// go test -run Test_Ctx_Set
func Test_Ctx_Set(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Set("X-1", "1")
	c.Set("X-2", "2")
	c.Set("X-3", "3")
	c.Set("X-3", "1337")
	require.Equal(t, "1", string(c.Response().Header.Peek("x-1")))
	require.Equal(t, "2", string(c.Response().Header.Peek("x-2")))
	require.Equal(t, "1337", string(c.Response().Header.Peek("x-3")))
}

// go test -run Test_Ctx_Set_Splitter
func Test_Ctx_Set_Splitter(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Set("Location", "foo\r\nSet-Cookie:%20SESSIONID=MaliciousValue\r\n")
	h := string(c.Response().Header.Peek("Location"))
	require.NotContains(t, h, "\r\n")

	c.Set("Location", "foo\nSet-Cookie:%20SESSIONID=MaliciousValue\n")
	h = string(c.Response().Header.Peek("Location"))
	require.NotContains(t, h, "\n")
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Set -benchmem -count=4
func Benchmark_Ctx_Set(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	val := "1431-15132-3423"
	b.ReportAllocs()
	for b.Loop() {
		c.Set(HeaderXRequestID, val)
	}
}

// go test -run Test_Ctx_Status
func Test_Ctx_Status(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Status(400)
	require.Equal(t, 400, c.Response().StatusCode())
	err := c.Status(415).Send([]byte("Hello, World"))
	require.NoError(t, err)
	require.Equal(t, 415, c.Response().StatusCode())
	require.Equal(t, "Hello, World", string(c.Response().Body()))
}

// go test -run Test_Ctx_Type
func Test_Ctx_Type(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Type(".json")
	require.Equal(t, "application/json; charset=utf-8", string(c.Response().Header.Peek("Content-Type")))

	c.Type("json", "utf-8")
	require.Equal(t, "application/json; charset=utf-8", string(c.Response().Header.Peek("Content-Type")))

	c.Type(".html")
	require.Equal(t, "text/html; charset=utf-8", string(c.Response().Header.Peek("Content-Type")))

	c.Type("html", "utf-8")
	require.Equal(t, "text/html; charset=utf-8", string(c.Response().Header.Peek("Content-Type")))

	// Test other text types get UTF-8 by default
	c.Type("txt")
	require.Equal(t, "text/plain; charset=utf-8", string(c.Response().Header.Peek("Content-Type")))

	c.Type("css")
	require.Equal(t, "text/css; charset=utf-8", string(c.Response().Header.Peek("Content-Type")))

	c.Type("js")
	require.Equal(t, "text/javascript; charset=utf-8", string(c.Response().Header.Peek("Content-Type")))

	c.Type("xml")
	require.Equal(t, "application/xml; charset=utf-8", string(c.Response().Header.Peek("Content-Type")))

	// Test binary types don't get charset
	c.Type("png")
	require.Equal(t, "image/png", string(c.Response().Header.Peek("Content-Type")))

	c.Type("pdf")
	require.Equal(t, "application/pdf", string(c.Response().Header.Peek("Content-Type")))

	// Test custom charset override
	c.Type("html", "iso-8859-1")
	require.Equal(t, "text/html; charset=iso-8859-1", string(c.Response().Header.Peek("Content-Type")))
}

// go test -run Test_shouldIncludeCharset
func Test_shouldIncludeCharset(t *testing.T) {
	t.Parallel()

	// Test text/* types - should include charset
	require.True(t, shouldIncludeCharset("text/html"))
	require.True(t, shouldIncludeCharset("text/plain"))
	require.True(t, shouldIncludeCharset("text/css"))
	require.True(t, shouldIncludeCharset("text/javascript"))
	require.True(t, shouldIncludeCharset("text/xml"))

	// Test explicit application types - should include charset
	require.True(t, shouldIncludeCharset("application/json"))
	require.True(t, shouldIncludeCharset("application/javascript"))
	require.True(t, shouldIncludeCharset("application/xml"))

	// Test +json suffixes - should include charset
	require.True(t, shouldIncludeCharset("application/problem+json"))
	require.True(t, shouldIncludeCharset("application/vnd.api+json"))
	require.True(t, shouldIncludeCharset("application/hal+json"))
	require.True(t, shouldIncludeCharset("application/merge-patch+json"))

	// Test +xml suffixes - should include charset
	require.True(t, shouldIncludeCharset("application/soap+xml"))
	require.True(t, shouldIncludeCharset("application/xhtml+xml"))
	require.True(t, shouldIncludeCharset("application/atom+xml"))
	require.True(t, shouldIncludeCharset("application/rss+xml"))

	// Test binary types - should NOT include charset
	require.False(t, shouldIncludeCharset("image/png"))
	require.False(t, shouldIncludeCharset("image/jpeg"))
	require.False(t, shouldIncludeCharset("application/pdf"))
	require.False(t, shouldIncludeCharset("application/octet-stream"))
	require.False(t, shouldIncludeCharset("video/mp4"))
	require.False(t, shouldIncludeCharset("audio/mpeg"))

	// Test other application types - should NOT include charset
	require.False(t, shouldIncludeCharset("application/cbor"))
	require.False(t, shouldIncludeCharset("application/x-www-form-urlencoded"))
	require.False(t, shouldIncludeCharset("application/vnd.msgpack"))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Type -benchmem -count=4
func Benchmark_Ctx_Type(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	b.ReportAllocs()
	for b.Loop() {
		c.Type(".json")
		c.Type("json")
	}
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Type_Charset -benchmem -count=4
func Benchmark_Ctx_Type_Charset(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	b.ReportAllocs()
	for b.Loop() {
		c.Type(".json", "utf-8")
		c.Type("json", "utf-8")
	}
}

// go test -run Test_Ctx_Vary
func Test_Ctx_Vary(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Vary("Origin")
	c.Vary("User-Agent")
	c.Vary("Accept-Encoding", "Accept")
	require.Equal(t, "Origin, User-Agent, Accept-Encoding, Accept", string(c.Response().Header.Peek("Vary")))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Vary -benchmem -count=4
func Benchmark_Ctx_Vary(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	b.ReportAllocs()
	for b.Loop() {
		c.Vary("Origin", "User-Agent")
	}
}

// go test -run Test_Ctx_Write
func Test_Ctx_Write(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	_, err := c.WriteString("Hello, ")
	require.NoError(t, err)
	_, err = c.WriteString("World!")
	require.NoError(t, err)
	require.Equal(t, "Hello, World!", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Write -benchmem -count=4
func Benchmark_Ctx_Write(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	byt := []byte("Hello, World!")
	b.ReportAllocs()

	var err error
	for b.Loop() {
		_, err = c.Write(byt)
	}
	require.NoError(b, err)
}

// go test -run Test_Ctx_Writef
func Test_Ctx_Writef(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	world := "World!"
	_, err := c.Writef("Hello, %s", world)
	require.NoError(t, err)
	require.Equal(t, "Hello, World!", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Writef -benchmem -count=4
func Benchmark_Ctx_Writef(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	world := "World!"
	b.ReportAllocs()

	var err error
	for b.Loop() {
		_, err = c.Writef("Hello, %s", world)
	}
	require.NoError(b, err)
}

// go test -run Test_Ctx_WriteString
func Test_Ctx_WriteString(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	_, err := c.WriteString("Hello, ")
	require.NoError(t, err)
	_, err = c.WriteString("World!")
	require.NoError(t, err)
	require.Equal(t, "Hello, World!", string(c.Response().Body()))
}

// go test -run Test_Ctx_XHR
func Test_Ctx_XHR(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXRequestedWith, "XMLHttpRequest")
	require.True(t, c.XHR())
}

// go test -run=^$ -bench=Benchmark_Ctx_XHR -benchmem -count=4
func Benchmark_Ctx_XHR(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXRequestedWith, "XMLHttpRequest")
	var equal bool
	b.ReportAllocs()
	for b.Loop() {
		equal = c.XHR()
	}
	require.True(b, equal)
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_SendString_B -benchmem -count=4
func Benchmark_Ctx_SendString_B(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	body := "Hello, world!"
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.SendString(body)
	}
	require.NoError(b, err)
	require.Equal(b, []byte("Hello, world!"), c.Response().Body())
}

// go test -run Test_Ctx_Queries -v
func Test_Ctx_Queries(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball,football&favouriteDrinks=milo,coke,pepsi&alloc=&no=1&field1=value1&field1=value2&field2=value3&list_a=1&list_a=2&list_a=3&list_b[]=1&list_b[]=2&list_b[]=3&list_c=1,2,3")

	queries := c.Queries()
	require.Equal(t, "1", queries["id"])
	require.Equal(t, "tom", queries["name"])
	require.Equal(t, "basketball,football", queries["hobby"])
	require.Equal(t, "milo,coke,pepsi", queries["favouriteDrinks"])
	require.Empty(t, queries["alloc"])
	require.Equal(t, "1", queries["no"])
	require.Equal(t, "value2", queries["field1"])
	require.Equal(t, "value3", queries["field2"])
	require.Equal(t, "3", queries["list_a"])
	require.Equal(t, "3", queries["list_b[]"])
	require.Equal(t, "1,2,3", queries["list_c"])

	c.Request().URI().SetQueryString("filters.author.name=John&filters.category.name=Technology&filters[customer][name]=Alice&filters[status]=pending")

	queries = c.Queries()
	require.Equal(t, "John", queries["filters.author.name"])
	require.Equal(t, "Technology", queries["filters.category.name"])
	require.Equal(t, "Alice", queries["filters[customer][name]"])
	require.Equal(t, "pending", queries["filters[status]"])

	c.Request().URI().SetQueryString("tags=apple,orange,banana&filters[tags]=apple,orange,banana&filters[category][name]=fruits&filters.tags=apple,orange,banana&filters.category.name=fruits")

	queries = c.Req().Queries()
	require.Equal(t, "apple,orange,banana", queries["tags"])
	require.Equal(t, "apple,orange,banana", queries["filters[tags]"])
	require.Equal(t, "fruits", queries["filters[category][name]"])
	require.Equal(t, "apple,orange,banana", queries["filters.tags"])
	require.Equal(t, "fruits", queries["filters.category.name"])

	c.Request().URI().SetQueryString("filters[tags][0]=apple&filters[tags][1]=orange&filters[tags][2]=banana&filters[category][name]=fruits")

	queries = c.Queries()
	require.Equal(t, "apple", queries["filters[tags][0]"])
	require.Equal(t, "orange", queries["filters[tags][1]"])
	require.Equal(t, "banana", queries["filters[tags][2]"])
	require.Equal(t, "fruits", queries["filters[category][name]"])
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Queries -benchmem -count=4
func Benchmark_Ctx_Queries(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	b.ReportAllocs()
	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball,football&favouriteDrinks=milo,coke,pepsi&alloc=&no=1")

	var queries map[string]string
	for b.Loop() {
		queries = c.Queries()
	}

	require.Equal(b, "1", queries["id"])
	require.Equal(b, "tom", queries["name"])
	require.Equal(b, "basketball,football", queries["hobby"])
	require.Equal(b, "milo,coke,pepsi", queries["favouriteDrinks"])
	require.Empty(b, queries["alloc"])
	require.Equal(b, "1", queries["no"])
}

// go test -run Test_Ctx_BodyStreamWriter
func Test_Ctx_BodyStreamWriter(t *testing.T) {
	t.Parallel()
	ctx := &fasthttp.RequestCtx{}

	ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		fmt.Fprintf(w, "body writer line 1\n")
		if err := w.Flush(); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		fmt.Fprintf(w, "body writer line 2\n")
	})

	require.True(t, ctx.IsBodyStream())

	s := ctx.Response.String()
	br := bufio.NewReader(bytes.NewBufferString(s))
	var resp fasthttp.Response
	require.NoError(t, resp.Read(br))

	body := string(resp.Body())
	expectedBody := "body writer line 1\nbody writer line 2\n"
	require.Equal(t, expectedBody, body)
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_BodyStreamWriter -benchmem -count=4
func Benchmark_Ctx_BodyStreamWriter(b *testing.B) {
	ctx := &fasthttp.RequestCtx{}
	user := []byte(`{"name":"john"}`)
	b.ReportAllocs()

	var err error
	for b.Loop() {
		ctx.ResetBody()
		ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
			for range 10 {
				_, err = w.Write(user)
				if flushErr := w.Flush(); flushErr != nil {
					return
				}
			}
		})
	}
	require.NoError(b, err)
}

func Test_Ctx_String(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	require.Equal(t, "#0000000000000000 - 0.0.0.0:0 <-> 0.0.0.0:0 - GET http:///", c.String())
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_String -benchmem -count=4
func Benchmark_Ctx_String(b *testing.B) {
	var str string
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	b.ReportAllocs()

	for b.Loop() {
		str = ctx.String()
	}
	require.Equal(b, "#0000000000000000 - 0.0.0.0:0 <-> 0.0.0.0:0 - GET http:///", str)
}

// go test -run Test_Ctx_IsFromLocal_X_Forwarded
func Test_Ctx_IsFromLocal_X_Forwarded(t *testing.T) {
	t.Parallel()
	// Test unset X-Forwarded-For header.
	{
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		// fasthttp returns "0.0.0.0" as IP as there is no remote address.
		require.Equal(t, "0.0.0.0", c.IP())
		require.False(t, c.IsFromLocal())
	}
	// Test when setting X-Forwarded-For header to localhost "127.0.0.1"
	{
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1")
		defer app.ReleaseCtx(c)
		require.False(t, c.IsFromLocal())
	}
	// Test when setting X-Forwarded-For header to localhost "::1"
	{
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "::1")
		defer app.ReleaseCtx(c)
		require.False(t, c.IsFromLocal())
	}
	// Test when setting X-Forwarded-For to full localhost IPv6 address "0:0:0:0:0:0:0:1"
	{
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "0:0:0:0:0:0:0:1")
		defer app.ReleaseCtx(c)
		require.False(t, c.IsFromLocal())
	}
	// Test for a random IP address.
	{
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "93.46.8.90")

		require.False(t, c.Req().IsFromLocal())
	}
}

// go test -run Test_Ctx_IsFromLocal_RemoteAddr
func Test_Ctx_IsFromLocal_RemoteAddr(t *testing.T) {
	t.Parallel()

	localIPv4 := net.Addr(&net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	localIPv6 := net.Addr(&net.TCPAddr{IP: net.ParseIP("::1")})
	localIPv6long := net.Addr(&net.TCPAddr{IP: net.ParseIP("0:0:0:0:0:0:0:1")})

	zeroIPv4 := net.Addr(&net.TCPAddr{IP: net.IPv4zero})

	someIPv4 := net.Addr(&net.TCPAddr{IP: net.ParseIP("93.46.8.90")})
	someIPv6 := net.Addr(&net.TCPAddr{IP: net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334")})

	// Test for the case fasthttp remoteAddr is set to "127.0.0.1".
	{
		app := New()
		fastCtx := &fasthttp.RequestCtx{}
		fastCtx.SetRemoteAddr(localIPv4)
		c := app.AcquireCtx(fastCtx)
		defer app.ReleaseCtx(c)

		require.Equal(t, "127.0.0.1", c.IP())
		require.True(t, c.IsFromLocal())
	}
	// Test for the case fasthttp remoteAddr is set to "::1".
	{
		app := New()
		fastCtx := &fasthttp.RequestCtx{}
		fastCtx.SetRemoteAddr(localIPv6)
		c := app.AcquireCtx(fastCtx)
		defer app.ReleaseCtx(c)
		require.Equal(t, "::1", c.Req().IP())
		require.True(t, c.Req().IsFromLocal())
	}
	// Test for the case fasthttp remoteAddr is set to "0:0:0:0:0:0:0:1".
	{
		app := New()
		fastCtx := &fasthttp.RequestCtx{}
		fastCtx.SetRemoteAddr(localIPv6long)
		c := app.AcquireCtx(fastCtx)
		defer app.ReleaseCtx(c)
		// fasthttp should return "::1" for "0:0:0:0:0:0:0:1".
		// otherwise IsFromLocal() will break.
		require.Equal(t, "::1", c.IP())
		require.True(t, c.IsFromLocal())
	}
	// Test for the case fasthttp remoteAddr is set to "0.0.0.0".
	{
		app := New()
		fastCtx := &fasthttp.RequestCtx{}
		fastCtx.SetRemoteAddr(zeroIPv4)
		c := app.AcquireCtx(fastCtx)
		defer app.ReleaseCtx(c)
		require.Equal(t, "0.0.0.0", c.IP())
		require.False(t, c.IsFromLocal())
	}
	// Test for the case fasthttp remoteAddr is set to "93.46.8.90".
	{
		app := New()
		fastCtx := &fasthttp.RequestCtx{}
		fastCtx.SetRemoteAddr(someIPv4)
		c := app.AcquireCtx(fastCtx)
		defer app.ReleaseCtx(c)
		require.Equal(t, "93.46.8.90", c.IP())
		require.False(t, c.IsFromLocal())
	}
	// Test for the case fasthttp remoteAddr is set to "2001:0db8:85a3:0000:0000:8a2e:0370:7334".
	{
		app := New()
		fastCtx := &fasthttp.RequestCtx{}
		fastCtx.SetRemoteAddr(someIPv6)
		c := app.AcquireCtx(fastCtx)
		defer app.ReleaseCtx(c)
		require.Equal(t, "2001:db8:85a3::8a2e:370:7334", c.IP())
		require.False(t, c.IsFromLocal())
	}
	// Test for the case fasthttp remoteAddr is set to a Unix socket.
	// Unix sockets are inherently local - only processes on the same host can connect.
	{
		app := New()
		fastCtx := &fasthttp.RequestCtx{}
		unixAddr := &net.UnixAddr{Name: "/tmp/fiber.sock", Net: "unix"}
		fastCtx.SetRemoteAddr(unixAddr)
		c := app.AcquireCtx(fastCtx)
		defer app.ReleaseCtx(c)
		require.True(t, c.IsFromLocal())
	}
}

// go test -run Test_Ctx_extractIPsFromHeader -v
func Test_Ctx_extractIPsFromHeader(t *testing.T) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set("x-forwarded-for", "1.1.1.1,8.8.8.8 , /n, \n,1.1, a.c, 6.,6., , a,,42.118.81.169,10.0.137.108")
	ips := c.IPs()
	res := ips[len(ips)-2]
	require.Equal(t, "42.118.81.169", res)
}

// go test -run Test_Ctx_extractIPsFromHeader -v
func Test_Ctx_extractIPsFromHeader_EnableValidateIp(t *testing.T) {
	app := New()
	app.config.EnableIPValidation = true
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set("x-forwarded-for", "1.1.1.1,8.8.8.8 , /n, \n,1.1, a.c, 6.,6., , a,,42.118.81.169,10.0.137.108")
	ips := c.IPs()
	res := ips[len(ips)-2]
	require.Equal(t, "42.118.81.169", res)
}

// go test -run Test_Ctx_GetRespHeaders
func Test_Ctx_GetRespHeaders(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Set("test", "Hello, World 👋!")
	c.Set("foo", "bar")
	c.Response().Header.Set("multi", "one")
	c.Response().Header.Add("multi", "two")
	c.Response().Header.Set(HeaderContentType, "application/json")

	require.Equal(t, map[string][]string{
		"Content-Type": {"application/json"},
		"Foo":          {"bar"},
		"Multi":        {"one", "two"},
		"Test":         {"Hello, World 👋!"},
	}, c.GetRespHeaders())
	require.Equal(t, map[string][]string{
		"Content-Type": {"application/json"},
		"Foo":          {"bar"},
		"Multi":        {"one", "two"},
		"Test":         {"Hello, World 👋!"},
	}, c.Res().GetHeaders())
}

func Benchmark_Ctx_GetRespHeaders(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Response().Header.Set("test", "Hello, World 👋!")
	c.Response().Header.Set("foo", "bar")
	c.Response().Header.Set(HeaderContentType, "application/json")

	b.ReportAllocs()

	var headers map[string][]string
	for b.Loop() {
		headers = c.GetRespHeaders()
	}

	require.Equal(b, map[string][]string{
		"Content-Type": {"application/json"},
		"Foo":          {"bar"},
		"Test":         {"Hello, World 👋!"},
	}, headers)
}

// go test -run Test_Ctx_GetReqHeaders
func Test_Ctx_GetReqHeaders(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("test", "Hello, World 👋!")
	c.Request().Header.Set("foo", "bar")
	c.Request().Header.Set("multi", "one")
	c.Request().Header.Add("multi", "two")
	c.Request().Header.Set(HeaderContentType, "application/json")

	require.Equal(t, map[string][]string{
		"Content-Type": {"application/json"},
		"Foo":          {"bar"},
		"Test":         {"Hello, World 👋!"},
		"Multi":        {"one", "two"},
	}, c.GetReqHeaders())
	require.Equal(t, map[string][]string{
		"Content-Type": {"application/json"},
		"Foo":          {"bar"},
		"Test":         {"Hello, World 👋!"},
		"Multi":        {"one", "two"},
	}, c.GetHeaders())
}

func Test_Ctx_Set_SanitizeHeaderValue(t *testing.T) {
	t.Parallel()

	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Set("X-Test", "foo\r\nbar: bad")

	headerVal := string(c.Response().Header.Peek("X-Test"))
	require.Equal(t, "foo  bar: bad", headerVal)
}

func Benchmark_Ctx_GetReqHeaders(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("test", "Hello, World 👋!")
	c.Request().Header.Set("foo", "bar")
	c.Request().Header.Set(HeaderContentType, "application/json")

	b.ReportAllocs()

	var headers map[string][]string
	for b.Loop() {
		headers = c.GetReqHeaders()
	}

	require.Equal(b, map[string][]string{
		"Content-Type": {"application/json"},
		"Foo":          {"bar"},
		"Test":         {"Hello, World 👋!"},
	}, headers)
}

// go test -run Test_Ctx_Drop -v
func Test_Ctx_Drop(t *testing.T) {
	t.Parallel()

	app := New()

	// Handler that calls Drop
	app.Get("/block-me", func(c Ctx) error {
		return c.Drop()
	})

	// Additional handler that just calls return
	app.Get("/no-response", func(_ Ctx) error {
		return nil
	})

	// Test the Drop method
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/block-me", http.NoBody))
	require.ErrorIs(t, err, ErrTestGotEmptyResponse)
	require.Nil(t, resp)

	// Test the no-response handler
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/no-response", http.NoBody))
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, StatusOK, resp.StatusCode)
	require.Equal(t, "0", resp.Header.Get("Content-Length"))
}

// go test -run Test_Ctx_DropWithMiddleware -v
func Test_Ctx_DropWithMiddleware(t *testing.T) {
	t.Parallel()

	app := New()

	// Middleware that calls Drop
	app.Use(func(c Ctx) error {
		err := c.Next()
		c.Set("X-Test", "test")
		return err
	})

	// Handler that calls Drop
	app.Get("/block-me", func(c Ctx) error {
		return c.Drop()
	})

	// Test the Drop method
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/block-me", http.NoBody))
	require.ErrorIs(t, err, ErrTestGotEmptyResponse)
	require.Nil(t, resp)
}

// go test -run Test_Ctx_End
func Test_Ctx_End(t *testing.T) {
	app := New()

	app.Get("/", func(c Ctx) error {
		c.SendString("Hello, World!") //nolint:errcheck // unnecessary to check error
		return c.End()
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "io.ReadAll(resp.Body)")
	require.Equal(t, "Hello, World!", string(body))
}

// go test -run Test_Ctx_End_after_timeout
func Test_Ctx_End_after_timeout(t *testing.T) {
	app := New()

	// Early flushing handler
	app.Get("/", func(c Ctx) error {
		time.Sleep(2 * time.Second)
		return c.End()
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)
	require.Nil(t, resp)
}

// go test -run Test_Ctx_End_with_drop_middleware
func Test_Ctx_End_with_drop_middleware(t *testing.T) {
	app := New()

	// Middleware that will drop connections
	// that persist after c.Next()
	app.Use(func(c Ctx) error {
		c.Next() //nolint:errcheck // unnecessary to check error
		return c.Drop()
	})

	// Early flushing handler
	app.Get("/", func(c Ctx) error {
		c.SendStatus(StatusOK) //nolint:errcheck // unnecessary to check error
		return c.End()
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, StatusOK, resp.StatusCode)
}

// go test -run Test_Ctx_End_after_drop
func Test_Ctx_End_after_drop(t *testing.T) {
	app := New()

	// Middleware that ends the request
	// after c.Next()
	app.Use(func(c Ctx) error {
		c.Next() //nolint:errcheck // unnecessary to check error
		return c.End()
	})

	// Early flushing handler
	app.Get("/", func(c Ctx) error {
		return c.Drop()
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", http.NoBody))
	require.ErrorIs(t, err, ErrTestGotEmptyResponse)
	require.Nil(t, resp)
}

// go test -run Test_Ctx_OverrideParam
func Test_Ctx_OverrideParam(t *testing.T) {
	t.Parallel()
	t.Run("route_params", func(t *testing.T) {
		// a basic request to check if OverrideParam functions correctly on different scenarios
		// - Does it change an existing param (it should)
		// - Does it ignore a non-existing param (it should)
		t.Parallel()
		app := New()
		app.Get("/user/:name/:id", func(c Ctx) error {
			c.OverrideParam("name", "overridden")
			c.OverrideParam("nonexistent", "ignored")
			require.Equal(t, "overridden", c.Params("name"))
			require.Equal(t, "123", c.Params("id"))
			require.Empty(t, c.Params("nonexistent"))
			require.Equal(t, []string{"name", "id"}, c.Route().Params)

			return c.JSON(map[string]any{
				"name": c.Params("name"),
				"id":   c.Params("id"),
				"all":  c.Route().Params,
			})
		})

		req, err := http.NewRequest(http.MethodGet, "/user/original/123", http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer func() { require.NoError(t, resp.Body.Close()) }()
		require.Equal(t, StatusOK, resp.StatusCode)
	})

	t.Run("plus_wildcard_params", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/files+/+",
			func(c Ctx) error {
				c.OverrideParam("+", "changed")
				c.OverrideParam("+2", "changed2")

				require.Equal(t, "changed", c.Params("+"))
				require.Equal(t, "changed2", c.Params("+2"))
				return nil
			},
		)

		req, err := http.NewRequest(http.MethodGet, "/filesoriginal/original2", http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer func() { require.NoError(t, resp.Body.Close()) }()
		require.Equal(t, StatusOK, resp.StatusCode)
	})

	t.Run("wildcard_params", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/files/*", func(c Ctx) error {
			c.OverrideParam("*", "changed")
			require.Equal(t, "changed", c.Params("*"))
			return nil
		})
		req, err := http.NewRequest(http.MethodGet, "/files/testing", http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer func() { require.NoError(t, resp.Body.Close()) }()
		require.Equal(t, StatusOK, resp.StatusCode)
	})

	t.Run("multi_wildcard_params", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/files/*/*", func(c Ctx) error {
			c.OverrideParam("*", "changed")
			c.OverrideParam("*2", "changed2")
			require.Equal(t, "changed", c.Params("*"))
			require.Equal(t, "changed2", c.Params("*2"))
			return nil
		})
		req, err := http.NewRequest(http.MethodGet, "/files/testing/testing", http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer func() { require.NoError(t, resp.Body.Close()) }()
		require.Equal(t, StatusOK, resp.StatusCode)
	})

	t.Run("case_sensitive", func(t *testing.T) {
		t.Parallel()

		// Ensure OverrideParam respects the CaseSensitive configuration
		app := New(Config{
			CaseSensitive: true,
		})

		app.Get("/user/:name", func(c Ctx) error {
			c.OverrideParam("name", "overridden")

			require.Equal(t, "overridden", c.Params("name"))
			require.Empty(t, c.Params("NAME"))

			return c.SendStatus(StatusOK)
		})

		req, err := http.NewRequest(http.MethodGet, "/user/original", http.NoBody)
		require.NoError(t, err)

		resp, err := app.Test(req)
		require.NoError(t, err)

		defer func() { require.NoError(t, resp.Body.Close()) }()

		require.Equal(t, StatusOK, resp.StatusCode)
	})

	t.Run("case_insensitive", func(t *testing.T) {
		t.Parallel()

		// CaseInsensitive mode (default)
		app := New(Config{
			CaseSensitive: false,
		})

		app.Get("/user/:name", func(c Ctx) error {
			c.OverrideParam("NAME", "overridden")

			require.Equal(t, "overridden", c.Params("name"))
			require.Equal(t, "overridden", c.Params("NAME"))

			return c.SendStatus(StatusOK)
		})

		req, err := http.NewRequest(http.MethodGet, "/user/original", http.NoBody)
		require.NoError(t, err)

		resp, err := app.Test(req)
		require.NoError(t, err)
		defer func() { require.NoError(t, resp.Body.Close()) }()

		require.Equal(t, StatusOK, resp.StatusCode)
	})

	t.Run("nil_router", func(t *testing.T) {
		t.Parallel()
		// Ensure OverrideParam handles nil route context gracefully
		app := New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		c, ok := ctx.(*DefaultCtx)
		require.True(t, ok)
		defer app.ReleaseCtx(c)
		c.route = nil

		c.OverrideParam("test", "value") // Should not change
		require.Empty(t, c.Params("test"))
	})
}

func Test_Ctx_AbandonSkipsReleaseCtx(t *testing.T) {
	t.Parallel()

	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // controlled test setup
	ctx.route = &Route{}

	t.Cleanup(func() {
		ctx.ForceRelease()
	})

	require.False(t, ctx.IsAbandoned())

	ctx.Abandon()
	require.True(t, ctx.IsAbandoned())

	app.ReleaseCtx(ctx)

	require.True(t, ctx.IsAbandoned(), "ReleaseCtx must not pool abandoned contexts")
	require.NotNil(t, ctx.fasthttp, "ReleaseCtx should not reset fasthttp on abandoned ctx")
	require.NotNil(t, ctx.route, "ReleaseCtx should not reset route on abandoned ctx")
}

func Test_Ctx_ForceReleaseClearsAbandon(t *testing.T) {
	t.Parallel()

	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // controlled test setup
	ctx.route = &Route{}

	ctx.Abandon()
	ctx.ForceRelease()

	require.False(t, ctx.IsAbandoned(), "ForceRelease should clear abandon flag")
	require.Nil(t, ctx.fasthttp, "ForceRelease should release fasthttp reference")
	require.Nil(t, ctx.route, "ForceRelease should reset route before pooling")
}

// go test -v -run=^$ -bench=Benchmark_Ctx_IsProxyTrusted -benchmem -count=4
func Benchmark_Ctx_IsProxyTrusted(b *testing.B) {
	// Scenario without trusted proxy check
	b.Run("NoProxyCheck", func(b *testing.B) {
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com:8080/test")
		b.ReportAllocs()
		for b.Loop() {
			c.IsProxyTrusted()
		}
		app.ReleaseCtx(c)
	})

	// Scenario without trusted proxy check in parallel
	b.Run("NoProxyCheckParallel", func(b *testing.B) {
		app := New()
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().SetRequestURI("http://google.com:8080/test")
			for pb.Next() {
				c.IsProxyTrusted()
			}
			app.ReleaseCtx(c)
		})
	})

	// Scenario with trusted proxy check simple
	b.Run("WithProxyCheckSimple", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		b.ReportAllocs()
		for b.Loop() {
			c.IsProxyTrusted()
		}
		app.ReleaseCtx(c)
	})

	// Scenario with trusted proxy check simple in parallel
	b.Run("WithProxyCheckSimpleParallel", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
		})
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().SetRequestURI("http://google.com/")
			c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
			for pb.Next() {
				c.IsProxyTrusted()
			}
			app.ReleaseCtx(c)
		})
	})

	// Scenario with trusted proxy check
	b.Run("WithProxyCheck", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"0.0.0.0"},
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		b.ReportAllocs()
		for b.Loop() {
			c.IsProxyTrusted()
		}
		app.ReleaseCtx(c)
	})

	// Scenario with trusted proxy check in parallel
	b.Run("WithProxyCheckParallel", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"0.0.0.0"},
			},
		})
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().SetRequestURI("http://google.com/")
			c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
			for pb.Next() {
				c.IsProxyTrusted()
			}
			app.ReleaseCtx(c)
		})
	})

	// Scenario with trusted proxy check allow private
	b.Run("WithProxyCheckAllowPrivate", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Private: true,
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		b.ReportAllocs()
		for b.Loop() {
			c.IsProxyTrusted()
		}
		app.ReleaseCtx(c)
	})

	// Scenario with trusted proxy check allow private in parallel
	b.Run("WithProxyCheckAllowPrivateParallel", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Private: true,
			},
		})
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().SetRequestURI("http://google.com/")
			c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
			for pb.Next() {
				c.IsProxyTrusted()
			}
			app.ReleaseCtx(c)
		})
	})

	// Scenario with trusted proxy check allow private as subnets
	b.Run("WithProxyCheckAllowPrivateAsSubnets", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"},
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		b.ReportAllocs()
		for b.Loop() {
			c.IsProxyTrusted()
		}
		app.ReleaseCtx(c)
	})

	// Scenario with trusted proxy check allow private as subnets in parallel
	b.Run("WithProxyCheckAllowPrivateAsSubnetsParallel", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"},
			},
		})
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().SetRequestURI("http://google.com/")
			c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
			for pb.Next() {
				c.IsProxyTrusted()
			}
			app.ReleaseCtx(c)
		})
	})

	// Scenario with trusted proxy check allow private, loopback, and link-local
	b.Run("WithProxyCheckAllowAll", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Private:   true,
				Loopback:  true,
				LinkLocal: true,
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		b.ReportAllocs()
		for b.Loop() {
			c.IsProxyTrusted()
		}
		app.ReleaseCtx(c)
	})

	// Scenario with trusted proxy check allow private, loopback, and link-local in parallel
	b.Run("WithProxyCheckAllowAllParallel", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Private:   true,
				Loopback:  true,
				LinkLocal: true,
			},
		})
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().SetRequestURI("http://google.com/")
			c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
			for pb.Next() {
				c.IsProxyTrusted()
			}
			app.ReleaseCtx(c)
		})
	})

	// Scenario with trusted proxy check allow private, loopback, and link-local as subnets
	b.Run("WithProxyCheckAllowAllowAllAsSubnets", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{
					// Link-local
					"169.254.0.0/16",
					"fe80::/10",
					// Loopback
					"127.0.0.0/8",
					"::1/128",
					// Private
					"10.0.0.0/8",
					"172.16.0.0/12",
					"192.168.0.0/16",
					"fc00::/7",
				},
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			c.IsProxyTrusted()
		}
		app.ReleaseCtx(c)
	})

	// Scenario with trusted proxy check allow private, loopback, and link-local as subnets in parallel
	b.Run("WithProxyCheckAllowAllowAllAsSubnetsParallel", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{
					// Link-local
					"169.254.0.0/16",
					"fe80::/10",
					// Loopback
					"127.0.0.0/8",
					"::1/128",
					// Private
					"10.0.0.0/8",
					"172.16.0.0/12",
					"192.168.0.0/16",
					"fc00::/7",
				},
			},
		})
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().SetRequestURI("http://google.com/")
			c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
			for pb.Next() {
				c.IsProxyTrusted()
			}
			app.ReleaseCtx(c)
		})
	})

	// Scenario with trusted proxy check with subnet
	b.Run("WithProxyCheckSubnet", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"0.0.0.0/8"},
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		b.ReportAllocs()
		for b.Loop() {
			c.IsProxyTrusted()
		}
		app.ReleaseCtx(c)
	})

	// Scenario with trusted proxy check with subnet in parallel
	b.Run("WithProxyCheckParallelSubnet", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"0.0.0.0/8"},
			},
		})
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().SetRequestURI("http://google.com/")
			c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
			for pb.Next() {
				c.IsProxyTrusted()
			}
			app.ReleaseCtx(c)
		})
	})

	// Scenario with trusted proxy check with multiple subnet
	b.Run("WithProxyCheckMultipleSubnet", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"192.168.0.0/24", "10.0.0.0/16", "0.0.0.0/8"},
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		b.ReportAllocs()
		for b.Loop() {
			c.IsProxyTrusted()
		}
		app.ReleaseCtx(c)
	})

	// Scenario with trusted proxy check with multiple subnet in parallel
	b.Run("WithProxyCheckParallelMultipleSubnet", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{"192.168.0.0/24", "10.0.0.0/16", "0.0.0.0/8"},
			},
		})
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().SetRequestURI("http://google.com/")
			c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
			for pb.Next() {
				c.IsProxyTrusted()
			}
			app.ReleaseCtx(c)
		})
	})

	// Scenario with trusted proxy check with all subnets
	b.Run("WithProxyCheckAllSubnets", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{
					"127.0.0.0/8",     // Loopback addresses
					"169.254.0.0/16",  // Link-Local addresses
					"fe80::/10",       // Link-Local addresses
					"192.168.0.0/16",  // Private Network addresses
					"172.16.0.0/12",   // Private Network addresses
					"10.0.0.0/8",      // Private Network addresses
					"fc00::/7",        // Unique Local addresses
					"173.245.48.0/20", // My custom range
					"0.0.0.0/8",       // All IPv4 addresses
				},
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		b.ReportAllocs()
		for b.Loop() {
			c.IsProxyTrusted()
		}
		app.ReleaseCtx(c)
	})

	// Scenario with trusted proxy check with all subnets in parallel
	b.Run("WithProxyCheckParallelAllSubnets", func(b *testing.B) {
		app := New(Config{
			TrustProxy: true,
			TrustProxyConfig: TrustProxyConfig{
				Proxies: []string{
					"127.0.0.0/8",     // Loopback addresses
					"169.254.0.0/16",  // Link-Local addresses
					"fe80::/10",       // Link-Local addresses
					"192.168.0.0/16",  // Private Network addresses
					"172.16.0.0/12",   // Private Network addresses
					"10.0.0.0/8",      // Private Network addresses
					"fc00::/7",        // Unique Local addresses
					"173.245.48.0/20", // My custom range
					"0.0.0.0/8",       // All IPv4 addresses
				},
			},
		})
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().SetRequestURI("http://google.com/")
			c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
			for pb.Next() {
				c.IsProxyTrusted()
			}
			app.ReleaseCtx(c)
		})
	})
}

func Benchmark_Ctx_IsFromLocalhost(b *testing.B) {
	// Scenario without localhost check
	b.Run("Non_Localhost", func(b *testing.B) {
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com:8080/test")
		b.ReportAllocs()
		for b.Loop() {
			c.IsFromLocal()
		}
		app.ReleaseCtx(c)
	})

	// Scenario with localhost check
	b.Run("Localhost", func(b *testing.B) {
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://localhost:8080/test")
		b.ReportAllocs()
		for b.Loop() {
			c.IsFromLocal()
		}
		app.ReleaseCtx(c)
	})
}

// go test -v -run=^$ -bench=Benchmark_Ctx_OverrideParam -benchmem -count=4
func Benchmark_Ctx_OverrideParam(b *testing.B) {
	app := New()

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	c, ok := ctx.(*DefaultCtx)
	if !ok {
		b.Fatal("AcquireCtx did not return *DefaultCtx")
	}

	defer app.ReleaseCtx(c)

	c.values = [maxParams]string{"original", "12345"}
	c.route = &Route{Params: []string{"name", "id"}}
	c.setMatched(true)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.OverrideParam("name", "changed")
	}
}
