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
	"io/fs"
	"math"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"text/template"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/shamaton/msgpack/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/log"
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

func Test_Ctx_Accepts(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New(Config{CBOREncoder: cbor.Marshal, CBORDecoder: cbor.Unmarshal}))

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

func Test_Ctx_AcceptsHelpers(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

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

func Test_Ctx_ContentTypeHelpers(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

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

func Test_Ctx_Charset(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		contentType string
		expected    string
	}{
		{name: "no_parameters", contentType: "text/plain", expected: ""},
		{name: "trailing_semicolon", contentType: "text/plain;", expected: ""},
		{name: "empty_param_before_charset", contentType: "text/plain; ; charset=utf-8", expected: "utf-8"},
		{name: "charset_with_spaces", contentType: "text/plain; charset = utf-8", expected: "utf-8"},
		{name: "charset_in_middle", contentType: "text/plain; foo=bar; charset=iso-8859-1; baz=qux", expected: "iso-8859-1"},
		{name: "charset_quoted", contentType: "text/plain; charset=\"utf-8\"; foo=bar", expected: "utf-8"},
		{name: "non_charset_only", contentType: "text/plain; foo=bar", expected: ""},
		{name: "missing_equals", contentType: "text/plain; charset", expected: ""},
		{name: "empty_charset_value", contentType: "text/plain; charset=", expected: ""},
		{name: "case_insensitive_charset", contentType: "text/plain; chArSet=Shift_JIS", expected: "Shift_JIS"},
		// RFC 9110 §5.6.6: a ";" inside a quoted parameter value is not
		// a parameter delimiter.
		{name: "quoted_value_with_semicolon_before_charset", contentType: `text/plain; title="x;charset=bad"; charset=utf-8`, expected: "utf-8"},
		// RFC 9110 §5.6.4: quoted-pairs must be replaced with the
		// escaped octet.
		{name: "quoted_pair_in_charset_value", contentType: `text/plain; charset="utf\-8"`, expected: "utf-8"},
		{name: "quoted_value_with_escaped_quote_before_charset", contentType: `text/plain; title="a\";charset=bad"; charset=utf-8`, expected: "utf-8"},
		// A stray quote inside an unquoted value must not swallow the
		// rest of the header: quoted-strings only start at value position.
		{name: "stray_quote_in_earlier_value", contentType: `multipart/form-data; boundary=ab"cd; charset=utf-8`, expected: "utf-8"},
		// A quoted value with trailing junk is not a valid quoted-string.
		{name: "quoted_value_with_trailing_junk", contentType: `text/plain; charset="utf-8"x`, expected: ""},
		{name: "later_equals_does_not_open_quotes", contentType: `multipart/form-data; boundary=ab="cd; charset=utf-8`, expected: "utf-8"},
		{name: "later_equals_does_not_hide_first_charset", contentType: `text/plain; a=b="; charset=bad"; charset=utf-8`, expected: ""},
		{name: "escaped_quote_at_start_of_earlier_value", contentType: `text/plain; a="\";charset=bad"; charset=utf-8`, expected: "utf-8"},
		{name: "unterminated_quoted_value", contentType: `text/plain; charset="utf-8`, expected: ""},
		{name: "quoted_value_ending_in_backslash", contentType: `text/plain; charset="utf-8\"`, expected: ""},
		// The first charset parameter decides; a malformed one is not skipped.
		{name: "bare_value_with_quote_then_valid_charset", contentType: `text/plain; a=b"; charset=bad"; charset=utf-8`, expected: ""},
		{name: "bare_value_with_trailing_quote", contentType: `text/plain; charset=utf-8"`, expected: ""},
		{name: "bare_value_with_comma", contentType: "text/plain; charset=utf-8,latin1", expected: ""},
		{name: "bare_value_with_backslash", contentType: `text/plain; charset=utf\-8`, expected: ""},
		{name: "invalid_bare_value_then_valid_charset", contentType: "text/plain; charset=utf-8,latin1; charset=iso-8859-1", expected: ""},
		{name: "empty_charset_value_then_valid_charset", contentType: "text/plain; charset=; charset=utf-8", expected: ""},
		{name: "bare_value_with_non_ascii", contentType: "text/plain; charset=utf-8\xfc", expected: ""},
		{name: "padded_charset_value", contentType: "text/plain; charset= utf-8 ", expected: "utf-8"},
		{name: "quoted_value_with_comma", contentType: `text/plain; charset="utf-8,latin1"`, expected: ""},
		{name: "quoted_value_with_space", contentType: `text/plain; charset="utf 8"`, expected: ""},
		{name: "quoted_empty_value_then_valid_charset", contentType: `text/plain; charset=""; charset=utf-8`, expected: ""},
		// A malformed parameter before charset does not hide it.
		{name: "parameter_without_value_before_charset", contentType: "text/plain; foo; charset=utf-8", expected: "utf-8"},
		{name: "invalid_parameter_value_before_charset", contentType: "text/plain; foo=[x]; charset=utf-8", expected: "utf-8"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())
			c.Request().Header.Set(HeaderContentType, testCase.contentType)
			require.Equal(t, testCase.expected, c.Charset())
		})
	}
}

func Benchmark_Ctx_Charset(b *testing.B) {
	for _, bench := range []struct {
		name        string
		contentType string
	}{
		{name: "no_parameters", contentType: MIMEApplicationJSON},
		{name: "charset", contentType: MIMEApplicationJSONCharsetUTF8},
		{name: "charset_last", contentType: `multipart/form-data; boundary="a;b"; charset=utf-8`},
		{name: "quoted_pair", contentType: `text/plain; charset="utf\-8"`},
		{name: "empty_parameters", contentType: "text/plain;" + strings.Repeat(";", 4000)},
		{name: "empty_parameters_before_charset", contentType: "text/plain;;;charset=utf-8"},
	} {
		b.Run(bench.name, func(b *testing.B) {
			c := apptest.Ctx(b, New())
			c.Request().Header.Set(HeaderContentType, bench.contentType)
			var charset string
			b.ReportAllocs()
			for b.Loop() {
				charset = c.Charset()
			}
			_ = charset
		})
	}
}

func Benchmark_Ctx_MediaType(b *testing.B) {
	c := apptest.Ctx(b, New())
	c.Request().Header.Set(HeaderContentType, "text/html ; charset=utf-8")
	var mediaType string
	b.ReportAllocs()
	for b.Loop() {
		mediaType = c.MediaType()
	}
	require.Equal(b, MIMETextHTML, mediaType)
}

func Test_Ctx_HeaderHelpers(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().Header.SetHost("example.com")
	c.Request().SetRequestURI("/search?q=fiber")
	require.Equal(t, "http://example.com/search?q=fiber", c.FullURL())

	c.Request().Header.Set(HeaderUserAgent, "fiber-agent")
	c.Request().Header.Set(HeaderReferer, "https://example.com")
	c.Request().Header.Set(HeaderAcceptLanguage, "en-US,en;q=0.9")
	c.Request().Header.Set(HeaderAcceptEncoding, "gzip, br")
	// Repeated field lines are combined per RFC 9110 §5.2, matching the
	// AcceptsEncodings/AcceptsLanguages view of the same headers.
	c.Request().Header.Add(HeaderAcceptEncoding, "zstd")
	c.Request().Header.Add(HeaderAcceptLanguage, "fr;q=0.5")
	c.Request().Header.Set("X-Trace-Id", "trace")
	require.True(t, c.HasHeader("X-Trace-Id"))
	require.True(t, c.HasHeader("x-trace-id"))
	require.Equal(t, "fiber-agent", c.UserAgent())
	require.Equal(t, "https://example.com", c.Referer())
	require.Equal(t, "en-US,en;q=0.9,fr;q=0.5", c.AcceptLanguage())
	require.Equal(t, "gzip, br,zstd", c.AcceptEncoding())

	c.Request().Header.Set(HeaderXRequestID, "request-id")
	c.Response().Header.Set(HeaderXRequestID, "response-id")
	require.Equal(t, "response-id", c.RequestID())
	c.Response().Header.Del(HeaderXRequestID)
	require.Equal(t, "request-id", c.RequestID())

	c.Request().Header.Del("X-Trace-Id")
	require.False(t, c.HasHeader("X-Trace-Id"))
}

func Test_Ctx_HasHeaderValue(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().Header.Set(HeaderCacheControl, "public, max-age=60")
	require.True(t, c.HasHeaderValue(HeaderCacheControl, "public"))
	require.True(t, c.HasHeaderValue("cache-control", "PUBLIC"))
	require.True(t, c.HasHeaderValue(HeaderCacheControl, "max-age=60"))
	require.False(t, c.HasHeaderValue(HeaderCacheControl, "no-transform"))
	require.False(t, c.HasHeaderValue(HeaderCacheControl, "max-age"))
	require.False(t, c.HasHeaderValue(HeaderCacheControl, ""))
	require.False(t, c.HasHeaderValue(HeaderConnection, "close"))

	// A member on a later field line counts: repeated lines are one list
	// (RFC 9110 Section 5.3).
	c.Request().Header.Add(HeaderCacheControl, "no-transform")
	require.True(t, c.HasHeaderValue(HeaderCacheControl, "no-transform"))
	require.True(t, c.Req().HasHeaderValue(HeaderCacheControl, "public"))

	c.Request().Header.Del(HeaderCacheControl)
	require.False(t, c.HasHeaderValue(HeaderCacheControl, "public"))

	// A comma inside a quoted directive argument is part of that member, not
	// a separator (RFC 9111 allows quoted arguments), so no-transform is not
	// listed here even though the bytes appear in the value.
	c.Request().Header.Set(HeaderCacheControl, "public")
	c.Request().Header.Add(HeaderCacheControl, `ext="x,no-transform,y"`)
	require.False(t, c.HasHeaderValue(HeaderCacheControl, "no-transform"))
	require.True(t, c.HasHeaderValue(HeaderCacheControl, `ext="x,no-transform,y"`))
	require.True(t, c.HasHeaderValue(HeaderCacheControl, "public"))
	c.Request().Header.Add(HeaderCacheControl, "no-transform")
	require.True(t, c.HasHeaderValue(HeaderCacheControl, "no-transform"))

	// An escaped quote does not close the argument, so the directive after it is its own member.
	c.Request().Header.Del(HeaderCacheControl)
	c.Request().Header.Set(HeaderCacheControl, `ext="a\"", no-transform`)
	require.True(t, c.HasHeaderValue(HeaderCacheControl, "no-transform"))

	// An empty first line does not hide the lines after it.
	c.Request().Header.Del(HeaderCacheControl)
	c.Request().Header.Add(HeaderCacheControl, "")
	c.Request().Header.Add(HeaderCacheControl, "no-transform")
	require.True(t, c.HasHeaderValue(HeaderCacheControl, "no-transform"))
	require.False(t, c.HasHeaderValue(HeaderCacheControl, "public"))
}

func Test_Ctx_FullURL_DoesNotAliasPooledBuffer(t *testing.T) {
	t.Parallel()
	const bufferPoolReuseAttempts = 128

	c := apptest.Ctx(t, New())

	c.Request().Header.SetHost("example.com")
	c.Request().SetRequestURI("/search?q=fiber")

	fullURL := c.FullURL()
	require.Equal(t, "http://example.com/search?q=fiber", fullURL)

	for range bufferPoolReuseAttempts {
		buf := bytebufferpool.Get()
		buf.Reset()
		buf.WriteString("https://mutated.example/rewritten")
		bytebufferpool.Put(buf)
	}

	require.Equal(t, "http://example.com/search?q=fiber", fullURL)
}

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

	req := apptest.Req(MethodGet, "/42?count=5&bad=oops",
		apptest.WithHeader("X-Limit", "11"), apptest.WithHeader("X-Bad", "oops"))
	apptest.Expect(t, app, req, StatusOK)
}

func Benchmark_Ctx_Accepts(b *testing.B) {
	c := apptest.Ctx(b, New(Config{CBOREncoder: cbor.Marshal, CBORDecoder: cbor.Unmarshal}))

	acceptHeader := "text/html,application/xhtml+xml,application/xml;q=0.9"
	c.Request().Header.Set("Accept", acceptHeader)
	acceptValues := [][]string{{".xml"}, {"json", "xml"}, {"application/json", "application/xml"}}
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

// jsonEnvelopeCustomCtx overrides JSON to wrap payloads in a standard API envelope.
// Used to regression-test #3319 (CustomCtx methods must survive middleware / Next).
type jsonEnvelopeCustomCtx struct {
	DefaultCtx
}

type jsonEnvelope struct {
	Data    any    `json:"data"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func (c *jsonEnvelopeCustomCtx) JSON(data any, ctype ...string) error {
	return c.DefaultCtx.JSON(jsonEnvelope{Code: StatusOK, Data: data, Message: "OK"}, ctype...)
}

// ctxTestACustomApp returns an app whose contexts are *customCtx.
func ctxTestACustomApp(config ...Config) *App {
	return NewWithCustomCtx(func(app *App) CustomCtx { return &customCtx{DefaultCtx: *NewDefaultCtx(app)} }, config...)
}

func Test_Ctx_CustomCtx(t *testing.T) {
	t.Parallel()
	app := ctxTestACustomApp()

	app.Get("/:id", func(c Ctx) error { return c.SendString(c.Params("id")) })
	resp := apptest.Do(t, app, httptest.NewRequest(MethodGet, "/v3", &bytes.Buffer{}))
	body := apptest.BodyBytes(t, resp)
	require.Len(t, body, len("prefix_v3"))
	require.Equal(t, "prefix_v3", string(body))
	require.Equal(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))
	require.Equal(t, int64(len(body)), resp.ContentLength)
}

func Test_App_AcquireDefaultCtx_CustomCtxFallback(t *testing.T) {
	t.Parallel()
	app := New()

	ctx, ok := app.prepareDefaultCtx(&customCtx{DefaultCtx: *NewDefaultCtx(app)}, &fasthttp.RequestCtx{})
	require.False(t, ok)
	require.Nil(t, ctx)
}

func Test_App_SetHandlerCtxIfNeeded_CustomCtxFallback(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := &customCtx{DefaultCtx: *NewDefaultCtx(app)}

	app.setHandlerCtxIfNeeded(ctx)
	require.Same(t, ctx, ctx.handlerCtx)
}

func Test_Ctx_Release_UserContextSetWithoutFastHTTP(t *testing.T) {
	t.Parallel()
	ctx := NewDefaultCtx(New())
	ctx.isUserContextSet = true

	require.NotPanics(t, ctx.release)
	require.False(t, ctx.isUserContextSet)
}

func Test_Ctx_CustomCtx_WithMiddleware(t *testing.T) {
	t.Parallel()
	app := ctxTestACustomApp()

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

	require.Equal(t, "prefix_", apptest.Body(t, apptest.Do(t, app, "/")))
}

func Test_Ctx_CustomCtx_JSON_WithMiddleware(t *testing.T) {
	t.Parallel()
	newCtx := func(app *App) CustomCtx { return &jsonEnvelopeCustomCtx{DefaultCtx: *NewDefaultCtx(app)} }

	for _, tc := range []struct {
		name       string
		middleware bool
	}{
		{name: "without middleware", middleware: false},
		// Middleware must not drop the custom JSON wrapper (#3319).
		{name: "with middleware", middleware: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := NewWithCustomCtx(newCtx)
			if tc.middleware {
				app.Use(func(c Ctx) error { return c.Next() })
			}
			app.Get("/vvv", func(c Ctx) error { return c.JSON(Map{"a": "b"}) })

			resp := apptest.Do(t, app, "/vvv")
			require.JSONEq(t, `{"code":200,"data":{"a":"b"},"message":"OK"}`, apptest.Body(t, resp))
		})
	}
}

// ctxTestAExpectEmptyOK sends req through app and expects a 200 response
// without a body, a Content-Type and a content length.
func ctxTestAExpectEmptyOK(t *testing.T, app *App, req *http.Request) {
	t.Helper()

	resp := apptest.ExpectBody(t, app, req, StatusOK, "")
	require.Empty(t, resp.Header.Get(HeaderContentType))
	require.Equal(t, int64(0), resp.ContentLength)
}

func Test_Ctx_CustomCtx_and_Method(t *testing.T) {
	t.Parallel()
	methods := append(DefaultMethods, "JOHN") //nolint:gocritic // We want a new slice here
	app := ctxTestACustomApp(Config{RequestMethods: methods})

	app.Add([]string{"JOHN"}, "/doe", testEmptyHandler)
	ctxTestAExpectEmptyOK(t, app, apptest.Req("JOHN", "/doe"))

	// a method missing from RequestMethods cannot be routed
	require.Panics(t, func() { app.Add([]string{"JANE"}, "/jane", testEmptyHandler) })
}

// ctxTestAAcceptsCase is one scenario of an Accept-family method: the offers are
// negotiated against a request whose header carries lines, one field line each
// (none leaves the header out), and want is the offer that wins, or empty when
// none is acceptable.
type ctxTestAAcceptsCase struct {
	name   string
	want   string
	lines  []string
	offers []string
}

// ctxTestAAccepts runs every case on a fresh context against method, the
// Accept-family method of Ctx that negotiates the request header named header.
func ctxTestAAccepts(t *testing.T, method func(Ctx, ...string) string, header string, cases []ctxTestAAcceptsCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())
			for _, line := range tc.lines {
				c.Request().Header.Add(header, line)
			}
			require.Equal(t, tc.want, method(c, tc.offers...))
		})
	}
}

func Test_Ctx_Accepts_HeaderForms(t *testing.T) {
	t.Parallel()
	ctxTestAAccepts(t, Ctx.Accepts, HeaderAccept, []ctxTestAAcceptsCase{
		{name: "wildcard accepts an extension", lines: []string{"*/*;q=0.9"}, offers: []string{"html"}, want: "html"},
		{name: "wildcard accepts an unknown extension", lines: []string{"*/*;q=0.9"}, offers: []string{"foo"}, want: "foo"},
		{name: "wildcard accepts a dotted extension", lines: []string{"*/*;q=0.9"}, offers: []string{".bar"}, want: ".bar"},
		{name: "subtype wildcard", lines: []string{"text/html,application/*;q=0.9"}, offers: []string{"xml"}, want: "xml"},

		// RFC 9110 §12.4.2: the weight parameter name "q" is matched case-insensitively.
		// "Q" must be recognized as the weight, not treated as a required media-type parameter.
		{name: "uppercase Q weight", lines: []string{"text/html;Q=0.2"}, offers: []string{"html"}, want: "html"},
		// An uppercase Q=0 means "not acceptable", same as q=0.
		{name: "uppercase Q=0", lines: []string{"text/html;Q=0"}, offers: []string{"html"}},
		// Weight ordering must apply regardless of case.
		{name: "uppercase Q orders by weight", lines: []string{"text/html;Q=0.2, application/json;q=0.8"}, offers: []string{"html", "json"}, want: "json"},

		{name: "repeated field lines", lines: []string{"text/plain;q=0.5", "application/json"}, offers: []string{"text/plain", "application/json"}, want: "application/json"},
		{name: "no Accept header", offers: []string{".forwarded"}, want: ".forwarded"},
	})
}

func Test_Ctx_AcceptsCharsets(t *testing.T) {
	t.Parallel()
	ctxTestAAccepts(t, Ctx.AcceptsCharsets, HeaderAcceptCharset, []ctxTestAAcceptsCase{
		{name: "weighted list", lines: []string{"utf-8, iso-8859-1;q=0.5"}, offers: []string{"utf-8"}, want: "utf-8"},
		{name: "repeated field lines", lines: []string{"utf-8;q=0.1", "iso-8859-1"}, offers: []string{"utf-8", "iso-8859-1"}, want: "iso-8859-1"},
	})
}

func Benchmark_Ctx_AcceptsCharsets(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())

	c.Request().Header.Set("Accept-Charset", "utf-8, iso-8859-1;q=0.5")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.AcceptsCharsets("utf-8")
	}
	require.Equal(b, "utf-8", res)
}

func Test_Ctx_AcceptsEncodings(t *testing.T) {
	t.Parallel()
	ctxTestAAccepts(t, Ctx.AcceptsEncodings, HeaderAcceptEncoding, []ctxTestAAcceptsCase{
		{name: "weighted list", lines: []string{"deflate, gzip;q=1.0, *;q=0.5"}, offers: []string{"gzip"}, want: "gzip"},
		{name: "wildcard accepts an unlisted coding", lines: []string{"deflate, gzip;q=1.0, *;q=0.5"}, offers: []string{"abc"}, want: "abc"},
		{name: "repeated field lines", lines: []string{"deflate;q=0.3", "gzip"}, offers: []string{"deflate", "gzip"}, want: "gzip"},
	})
}

// RFC 9110 §12.5.3: "identity" is acceptable unless the field excludes it.
func Test_Ctx_AcceptsEncodings_Identity(t *testing.T) {
	t.Parallel()
	ctxTestAAccepts(t, Ctx.AcceptsEncodings, HeaderAcceptEncoding, []ctxTestAAcceptsCase{
		// Listed codings keep winning on their weights.
		{name: "listed coding", lines: []string{"gzip"}, offers: []string{"gzip", "br"}, want: "gzip"},
		{name: "listed beats unlisted", lines: []string{"br;q=0.1"}, offers: []string{"identity", "br"}, want: "br"},
		{name: "unlisted coding", lines: []string{"gzip"}, offers: []string{"br"}, want: ""},
		{name: "refused coding", lines: []string{"gzip;q=0"}, offers: []string{"gzip"}, want: ""},

		// Unmentioned identity is acceptable.
		{name: "identity unlisted", lines: []string{"gzip"}, offers: []string{"identity"}, want: "identity"},
		{name: "identity unlisted among offers", lines: []string{"gzip"}, offers: []string{"br", "identity"}, want: "identity"},
		{name: "identity after a refused coding", lines: []string{"gzip;q=0"}, offers: []string{"gzip", "identity"}, want: "identity"},
		{name: "identity ranks below a listed coding", lines: []string{"gzip;q=0.1"}, offers: []string{"identity", "gzip"}, want: "gzip"},
		{name: "identity keeps the caller's spelling", lines: []string{"gzip"}, offers: []string{"Identity"}, want: "Identity"},

		// ... unless the field excludes it.
		{name: "identity refused", lines: []string{"identity;q=0"}, offers: []string{"identity"}, want: ""},
		{name: "identity refused, other coding unlisted", lines: []string{"identity;q=0"}, offers: []string{"gzip", "identity"}, want: ""},
		{name: "identity refused, other coding listed", lines: []string{"identity;q=0, gzip"}, offers: []string{"identity", "gzip"}, want: "gzip"},
		{name: "identity refused in any case", lines: []string{"IDENTITY;q=0"}, offers: []string{"identity"}, want: ""},
		{name: "everything refused", lines: []string{"*;q=0"}, offers: []string{"gzip", "identity"}, want: ""},
		{name: "everything but identity refused", lines: []string{"*;q=0, identity"}, offers: []string{"gzip", "identity"}, want: "identity"},
		{name: "everything but gzip refused", lines: []string{"*;q=0, gzip"}, offers: []string{"identity", "gzip"}, want: "gzip"},

		// An entry for identity is more specific than the wildcard.
		{name: "wildcard accepts identity", lines: []string{"*"}, offers: []string{"identity"}, want: "identity"},
		{name: "identity refused despite the wildcard", lines: []string{"*;q=0.5, identity;q=0"}, offers: []string{"identity"}, want: ""},
		{name: "wildcard still serves another coding", lines: []string{"*;q=0.5, identity;q=0"}, offers: []string{"identity", "gzip"}, want: "gzip"},
		{name: "identity weighed on its own entry", lines: []string{"gzip;q=0.5, identity"}, offers: []string{"gzip", "identity"}, want: "identity"},
		{name: "coding outweighs identity", lines: []string{"gzip, identity;q=0.5"}, offers: []string{"identity", "gzip"}, want: "gzip"},
	})
}

// An absent field accepts any coding; an empty one only identity.
func Test_Ctx_AcceptsEncodings_AbsentAndEmpty(t *testing.T) {
	t.Parallel()
	for _, config := range []Config{{}, {DisableHeaderNormalizing: true}} {
		t.Run(fmt.Sprintf("DisableHeaderNormalizing=%t", config.DisableHeaderNormalizing), func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New(config))

			// Absent: the first offer, whatever it is.
			require.Equal(t, "gzip", c.AcceptsEncodings("gzip", "identity"))
			require.Equal(t, "br", c.AcceptsEncodings("br"))
			require.Empty(t, c.AcceptsEncodings())

			// Present and empty: only identity.
			c.Request().Header.Set(HeaderAcceptEncoding, "")
			require.Equal(t, "identity", c.AcceptsEncodings("gzip", "identity"))
			require.Equal(t, "Identity", c.AcceptsEncodings("gzip", "Identity"))
			require.Empty(t, c.AcceptsEncodings("gzip", "br"))
			require.Empty(t, c.AcceptsEncodings())

			// An empty line beside a coding is an empty list element (§5.6.1.2).
			c.Request().Header.Add(HeaderAcceptEncoding, "gzip")
			require.Equal(t, "gzip", c.AcceptsEncodings("identity", "gzip"))
		})
	}
}

func Benchmark_Ctx_AcceptsEncodings(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())

	c.Request().Header.Set(HeaderAcceptEncoding, "deflate, gzip;q=1.0, *;q=0.5")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.AcceptsEncodings("gzip")
	}
	require.Equal(b, "gzip", res)
}

func Test_Ctx_AcceptsLanguages(t *testing.T) {
	t.Parallel()
	ctxTestAAccepts(t, Ctx.AcceptsLanguages, HeaderAcceptLanguage, []ctxTestAAcceptsCase{
		{name: "weighted list", lines: []string{"fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5"}, offers: []string{"fr"}, want: "fr"},
		{name: "repeated field lines", lines: []string{"de;q=0.4", "en"}, offers: []string{"de", "en"}, want: "en"},
		{name: "case insensitive", lines: []string{"EN-us"}, offers: []string{"en-US"}, want: "en-US"},

		// RFC 4647 Basic Filtering
		{name: "exact tag wins among offers", lines: []string{"en-US"}, offers: []string{"en", "en-US"}, want: "en-US"},
		{name: "range does not cover a shorter tag", lines: []string{"en-US"}, offers: []string{"en"}},
		{name: "skips the unacceptable offer", lines: []string{"en-US, fr"}, offers: []string{"de", "en-US", "fr"}, want: "en-US"},
		{name: "range covers a longer tag", lines: []string{"en"}, offers: []string{"en-US"}, want: "en-US"},
		{name: "wildcard accepts the first offer", lines: []string{"*"}, offers: []string{"en", "fr"}, want: "en"},
		{name: "underscore is not a subtag separator", lines: []string{"en_US"}, offers: []string{"en-US"}},
		{name: "wildcard subtag needs extended filtering", lines: []string{"en-*"}, offers: []string{"en-US"}},
	})
}

func Test_Ctx_AcceptsLanguagesExtended(t *testing.T) {
	t.Parallel()
	// RFC 4647 Extended Filtering
	ctxTestAAccepts(t, Ctx.AcceptsLanguagesExtended, HeaderAcceptLanguage, []ctxTestAAcceptsCase{
		{name: "wildcard subtag", lines: []string{"en-*"}, offers: []string{"en-US"}, want: "en-US"},
		{name: "wildcard primary subtag", lines: []string{"*-US"}, offers: []string{"en-US", "fr-CA"}, want: "en-US"},
		{name: "trailing wildcard subtag", lines: []string{"en-US-*"}, offers: []string{"en-US"}, want: "en-US"},
		{name: "range covers a longer tag", lines: []string{"en"}, offers: []string{"en-US"}, want: "en-US"},
		{name: "wildcard accepts the first offer", lines: []string{"*"}, offers: []string{"en-US", "fr-CA"}, want: "en-US"},
		{name: "range covers a tag with more subtags", lines: []string{"en-US"}, offers: []string{"en-US-CA"}, want: "en-US-CA"},
		{name: "wildcard subtag covers more subtags", lines: []string{"en-*"}, offers: []string{"en-US-CA"}, want: "en-US-CA"},
	})
}

func Benchmark_Ctx_AcceptsLanguages(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())

	c.Request().Header.Set(HeaderAcceptLanguage, "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.AcceptsLanguages("fr")
	}
	require.Equal(b, "fr", res)
}

func Test_Ctx_App(t *testing.T) {
	t.Parallel()
	app := New()
	app.config.BodyLimit = 1000
	c := apptest.Ctx(t, app)

	require.Equal(t, 1000, c.App().config.BodyLimit)
}

func Test_Ctx_Append(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

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
	// empty values are skipped: a sender must not generate empty list
	// elements (RFC 9110 §5.6.1.2)
	c.Append("X5-Test", "a", "", "b")
	c.Append("X5-Test", "")

	require.Equal(t, "Hello, World", string(c.Response().Header.Peek("X-Test")))
	require.Equal(t, "World, XHello, Hello", string(c.Response().Header.Peek("X2-Test")))
	require.Equal(t, "XHello, World, Hello", string(c.Response().Header.Peek("X3-Test")))
	require.Equal(t, "XHello, Hello, HelloZ, YHello", string(c.Response().Header.Peek("X4-Test")))
	require.Equal(t, "a, b", string(c.Response().Header.Peek("X5-Test")))
	require.Empty(t, string(c.Response().Header.Peek("x-custom-header")))
}

func Benchmark_Ctx_Append(b *testing.B) {
	app := New()
	c := apptest.CtxAs[*DefaultCtx](b, app)

	b.ReportAllocs()
	for b.Loop() {
		c.Append("X-Custom-Header", "Hello")
		c.Append("X-Custom-Header", "World")
		c.Append("X-Custom-Header", "Hello")
	}
	require.Equal(b, "Hello, World", app.toString(c.Response().Header.Peek("X-Custom-Header")))
}

func Test_Ctx_Attachment(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Attachment()
	require.Equal(t, `attachment`, string(c.Response().Header.Peek(HeaderContentDisposition)))
	c.Attachment("./static/img/logo.png")
	require.Equal(t, `attachment; filename="logo.png"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
	require.Equal(t, "image/png", string(c.Response().Header.Peek(HeaderContentType)))
	// filename with spaces stays literal inside the quoted-string
	// (RFC 9110 §5.6.4 / RFC 6266 — no URL encoding in the filename param)
	c.Attachment("report 2024.txt")
	require.Equal(t, `attachment; filename="report 2024.txt"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
	c.Attachment("../docs/archive.tar.gz")
	require.Equal(t, `attachment; filename="archive.tar.gz"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
	// check quoting: controls are stripped, quotes are backslash-escaped
	c.Attachment("another document.pdf\"\r\nBla: \"fasel")
	require.Equal(t, `attachment; filename="another document.pdf\"Bla: \"fasel"`, string(c.Response().Header.Peek(HeaderContentDisposition)))

	c.Attachment("файл.txt")
	header := string(c.Response().Header.Peek(HeaderContentDisposition))
	require.Contains(t, header, `filename="файл.txt"`)
	require.Contains(t, header, `filename*=UTF-8''%D1%84%D0%B0%D0%B9%D0%BB.txt`)

	// RFC 8187 §3.2: '=', '@' (and ':') are not attr-chars and must be
	// pct-encoded in the filename* ext-value.
	c.Attachment("ü=final@2x.png")
	header = string(c.Response().Header.Peek(HeaderContentDisposition))
	require.Contains(t, header, `filename*=UTF-8''%C3%BC%3Dfinal%402x.png`)
}

func Test_Ctx_Attachment_SanitizesFilenameControls(t *testing.T) {
	t.Parallel()
	app := New()
	testCases := []struct {
		name     string
		filename string
		expected string
	}{
		{"base name only", "../docs/archive.tar.gz", `attachment; filename="archive.tar.gz"`},
		{"controls stripped", "down\r\nload\t\x00.txt", `attachment; filename="download.txt"`},
		{"controls stripped without extension", "report\r\n\t\x00", `attachment; filename="report"`},
		{"empty after sanitize", "\r\n\t\x00", `attachment; filename="download"`},
		{"controls stripped in middle", "file\rname\n\t\x00.bin", `attachment; filename="filename.bin"`},
		{"dot fallback", ".", `attachment; filename="download"`},
	}

	// Download writes the header through the same sanitizer.
	for _, accessor := range []struct {
		set  func(c Ctx, filename string) error
		name string
	}{
		{name: "Attachment", set: func(c Ctx, filename string) error {
			c.Attachment(filename)
			return nil
		}},
		{name: "Download", set: func(c Ctx, filename string) error { return c.Download("ctx.go", filename) }},
	} {
		t.Run(accessor.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					c := apptest.Ctx(t, app)
					require.NoError(t, accessor.set(c, tc.filename))
					header := string(c.Response().Header.Peek(HeaderContentDisposition))
					require.Equal(t, tc.expected, header)
					require.NotContains(t, header, "\r")
					require.NotContains(t, header, "\n")
					require.NotContains(t, header, "\t")
					require.NotContains(t, header, "\x00")
				})
			}
		})
	}
}

func Benchmark_Ctx_Attachment(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())

	b.ReportAllocs()
	for b.Loop() {
		// example with quote params
		c.Attachment("another document.pdf\"\r\nBla: \"fasel")
	}
	require.Equal(b, `attachment; filename="another document.pdf\"Bla: \"fasel"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
}

func Test_Ctx_BaseURL(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().SetRequestURI("http://google.com/test")
	require.Equal(t, "http://google.com", c.BaseURL())
	// Check cache
	require.Equal(t, "http://google.com", c.BaseURL())
}

func Benchmark_Ctx_BaseURL(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())

	c.Request().SetHost("google.com:1337")
	c.Request().URI().SetPath("/haha/oke/lol")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.BaseURL()
	}
	require.Equal(b, "http://google.com:1337", res)
}

func Benchmark_Ctx_BaseURL_Uncached(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())

	c.Request().SetHost("google.com:1337")
	c.Request().URI().SetPath("/haha/oke/lol")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		c.baseURI = ""
		res = c.BaseURL()
	}
	require.Equal(b, "http://google.com:1337", res)
}

func Benchmark_Ctx_FullURL(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())

	c.Request().SetRequestURI("/haha/oke/lol?name=fiber")
	c.Request().URI().SetHost("google.com:1337")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.FullURL()
	}
	require.Equal(b, "http://google.com:1337/haha/oke/lol?name=fiber", res)
}

func Test_Ctx_Body(t *testing.T) {
	t.Parallel()
	immutable := New()
	immutable.config.Immutable = true
	for _, tc := range []struct {
		app  *App
		set  func(*fasthttp.Request, []byte)
		get  func(*DefaultCtx) []byte
		name string
	}{
		{name: "Body", app: New(), set: (*fasthttp.Request).SetBody, get: (*DefaultCtx).Body},
		{name: "BodyRaw", app: New(), set: (*fasthttp.Request).SetBodyRaw, get: (*DefaultCtx).BodyRaw},
		{
			name: "BodyRaw immutable", app: New(Config{Immutable: true}),
			set: (*fasthttp.Request).SetBodyRaw, get: (*DefaultCtx).BodyRaw,
		},
		{name: "Body immutable", app: immutable, set: (*fasthttp.Request).SetBody, get: (*DefaultCtx).Body},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.CtxAs[*DefaultCtx](t, tc.app)
			tc.set(c.Request(), []byte("john=doe"))
			require.Equal(t, []byte("john=doe"), tc.get(c))
		})
	}
}

// ctxBenchBodyCtx returns a context of app whose request body, stored with set, is "john=doe".
func ctxBenchBodyCtx(b *testing.B, app *App, set func(*fasthttp.Request, []byte)) *DefaultCtx {
	b.Helper()
	c := apptest.CtxAs[*DefaultCtx](b, app)
	set(c.Request(), []byte("john=doe"))
	b.ReportAllocs()
	return c
}

func Benchmark_Ctx_Body(b *testing.B) {
	c := ctxBenchBodyCtx(b, New(), (*fasthttp.Request).SetBody)
	for b.Loop() {
		_ = c.Body()
	}
	require.Equal(b, []byte("john=doe"), c.Body())
}

func Benchmark_Ctx_BodyRaw(b *testing.B) {
	c := ctxBenchBodyCtx(b, New(), (*fasthttp.Request).SetBodyRaw)
	for b.Loop() {
		_ = c.BodyRaw()
	}
	require.Equal(b, []byte("john=doe"), c.BodyRaw())
}

func Benchmark_Ctx_BodyRaw_Immutable(b *testing.B) {
	c := ctxBenchBodyCtx(b, New(Config{Immutable: true}), (*fasthttp.Request).SetBodyRaw)
	for b.Loop() {
		_ = c.BodyRaw()
	}
	require.Equal(b, []byte("john=doe"), c.BodyRaw())
}

func Benchmark_Ctx_Body_Immutable(b *testing.B) {
	app := New()
	app.config.Immutable = true
	c := ctxBenchBodyCtx(b, app, (*fasthttp.Request).SetBody)
	for b.Loop() {
		_ = c.Body()
	}
	require.Equal(b, []byte("john=doe"), c.Body())
}

// ctxTestACompress returns data compressed with the given content coding, gzip
// or deflate.
func ctxTestACompress(tb testing.TB, coding string, data []byte) []byte {
	tb.Helper()

	var (
		buf    bytes.Buffer
		writer interface {
			io.WriteCloser
			Flush() error
		}
	)
	switch coding {
	case StrGzip:
		writer = gzip.NewWriter(&buf)
	case StrDeflate:
		writer = zlib.NewWriter(&buf)
	default:
		require.FailNow(tb, "unsupported content coding", coding)
	}

	_, err := writer.Write(data)
	require.NoError(tb, err)
	require.NoError(tb, writer.Flush())
	require.NoError(tb, writer.Close())
	return buf.Bytes()
}

func Test_Ctx_Body_With_Compression(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		contentEncoding string
		body            []byte
		expectedBody    []byte
	}{
		{name: "gzip", contentEncoding: "gzip", body: []byte("john=doe"), expectedBody: []byte("john=doe")},
		{name: "gzip twice", contentEncoding: "gzip, gzip", body: []byte("double"), expectedBody: []byte("double")},
		// RFC 9110 §5.6.1.2: empty list elements must be parsed and
		// ignored, so a trailing comma must not yield 415.
		{name: "gzip trailing comma", contentEncoding: "gzip,", body: []byte("john=doe"), expectedBody: []byte("john=doe")},
		{name: "gzip twice with empty element between", contentEncoding: "gzip, , gzip", body: []byte("double"), expectedBody: []byte("double")},
		{name: "unsupported_encoding", contentEncoding: "undefined", body: []byte("keeps_ORIGINAL"), expectedBody: []byte("Unsupported Media Type")},
		{name: "compress_not_implemented", contentEncoding: "compress", body: []byte("foo"), expectedBody: []byte("Not Implemented")},
		{name: "gzip then unsupported", contentEncoding: "gzip, undefined", body: []byte("Go, be gzipped"), expectedBody: []byte("Unsupported Media Type")},
		{name: "invalid_deflate", contentEncoding: "gzip,deflate", body: []byte("I'm not correctly compressed"), expectedBody: []byte(zlib.ErrHeader.Error())},
		{name: "identity", contentEncoding: "identity", body: []byte("bar"), expectedBody: []byte("bar")},
	}

	// Config.Immutable must not change what Body decodes to.
	for _, immutable := range []bool{false, true} {
		t.Run(fmt.Sprintf("Immutable=%t", immutable), func(t *testing.T) {
			t.Parallel()
			for _, testObject := range tests {
				tCase := testObject // Duplicate object to ensure it will be unique across all runs
				t.Run(tCase.name, func(t *testing.T) {
					t.Parallel()
					app := New()
					app.config.Immutable = immutable
					c := apptest.CtxAs[*DefaultCtx](t, app)
					c.Request().Header.Set("Content-Encoding", tCase.contentEncoding)

					encs := strings.SplitSeq(tCase.contentEncoding, ",")
					for enc := range encs {
						enc = utils.TrimSpace(enc)
						if strings.Contains(tCase.name, "invalid_deflate") && enc == StrDeflate {
							continue
						}
						switch enc {
						case StrGzip, StrDeflate:
							tCase.body = ctxTestACompress(t, enc, tCase.body)
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

					require.Equal(t, tCase.body, c.Request().Body(), "Body raw must be the same as set before")
				})
			}
		})
	}
}

// ctxWithStreamedBody returns a context of app whose request body is stream, size bytes long (-1: not announced),
// sent as contentType.
func ctxWithStreamedBody(tb testing.TB, app *App, contentType string, stream io.Reader, size int) Ctx {
	tb.Helper()

	c := apptest.Ctx(tb, app)
	c.Request().Header.SetContentType(contentType)
	c.Request().SetBodyStream(stream, size)
	return c
}

// With StreamRequestBody the server passes on a body of any size, so the accessors that buffer it apply BodyLimit.
func Test_Ctx_Body_StreamedBodyLimit(t *testing.T) {
	t.Parallel()
	const limit = 1024
	app := New(Config{StreamRequestBody: true, BodyLimit: limit})
	tooLarge := fasthttp.ErrBodyTooLarge.Error()
	readBody := func(c Ctx) string { return string(c.Body()) }
	readForm := func(c Ctx) string { return "a=" + c.FormValue("a", "none") }

	for _, tc := range []struct {
		read        func(c Ctx) string
		name        string
		contentType string
		coding      string
		refused     string
	}{
		{name: "Body", read: readBody, refused: tooLarge},
		{name: "BodyRaw", read: func(c Ctx) string { return string(c.BodyRaw()) }, refused: tooLarge},
		{name: "Body with a coding", coding: StrIdentity, read: readBody, refused: tooLarge},
		{name: "FormValue", contentType: MIMEApplicationForm, read: readForm, refused: "a=none"},
		// fasthttp's PostArgs reads any media type with this prefix.
		{name: "FormValue by prefix", contentType: MIMEApplicationForm + "-x", read: readForm, refused: "a=none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			streamed := func(t *testing.T, size, declared int) (Ctx, *strings.Reader) {
				t.Helper()

				stream := strings.NewReader("a=" + strings.Repeat("a", size-2))
				c := ctxWithStreamedBody(t, app, tc.contentType, stream, declared)
				if tc.coding != "" {
					c.Request().Header.Set(HeaderContentEncoding, tc.coding)
				}
				return c, stream
			}

			t.Run("within the limit", func(t *testing.T) {
				t.Parallel()
				c, _ := streamed(t, limit, -1)
				require.Equal(t, "a="+strings.Repeat("a", limit-2), tc.read(c))
				require.Equal(t, StatusOK, c.Response().StatusCode())
			})

			for name, declared := range map[string]int{"chunked": -1, "declared length": 100 * limit} {
				t.Run("over the limit, "+name, func(t *testing.T) {
					t.Parallel()
					c, stream := streamed(t, 100*limit, declared)
					for range 2 {
						require.Equal(t, tc.refused, tc.read(c))
						require.Equal(t, StatusRequestEntityTooLarge, c.Response().StatusCode())
						require.Equal(t, 99*limit-1, stream.Len(), "the rest of the stream is not read")
					}
				})
			}
		})
	}
}

// Body returns its failure as the body, and leaves a response the handler has written alone.
func Test_Ctx_Body_FailureKeepsWrittenResponse(t *testing.T) {
	t.Parallel()
	limited := Config{StreamRequestBody: true, BodyLimit: 1024}
	tooLarge := fasthttp.ErrBodyTooLarge.Error()

	for _, tc := range []struct {
		config   Config
		name     string
		encoding string
		body     string // "stream" or "gzip": 4096 bytes, streamed or gzipped; none when empty
		want     string
	}{
		{name: "streamed body", config: limited, body: "stream", want: tooLarge},
		{name: "streamed body, identity", config: limited, encoding: StrIdentity, body: "stream", want: tooLarge},
		{name: "streamed body, gzip", config: limited, encoding: StrGzip, body: "stream", want: tooLarge},
		{name: "gzipped body", config: limited, encoding: StrGzip, body: "gzip", want: tooLarge},
		{name: "unsupported coding", encoding: "x", want: ErrUnsupportedMediaType.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New(tc.config))
			if tc.encoding != "" {
				c.Request().Header.Set(HeaderContentEncoding, tc.encoding)
			}
			switch tc.body {
			case "stream":
				c.Request().SetBodyStream(strings.NewReader(strings.Repeat("a", 4096)), -1)
			case "gzip":
				c.Request().SetBody(fasthttp.AppendGzipBytes(nil, []byte(strings.Repeat("a", 4096))))
			}
			require.NoError(t, c.SendString("ok"))
			require.Equal(t, tc.want, string(c.Body()))
			require.Equal(t, StatusOK, c.Response().StatusCode())
			require.Equal(t, "ok", string(c.Response().Body()))
		})
	}
}

// FormValue leaves a streamed body to BodyStream when the body is not a form or the key is in the query.
func Test_Ctx_FormValue_KeepsBodyStream(t *testing.T) {
	t.Parallel()
	app := New(Config{StreamRequestBody: true, BodyLimit: 8})
	for _, tc := range []struct {
		name        string
		contentType string
		notForm     bool // a key outside the query is not looked up in the body either
	}{
		{name: "not a form", contentType: MIMEOctetStream, notForm: true},
		{name: "form", contentType: MIMEApplicationForm},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := ctxWithStreamedBody(t, app, tc.contentType, strings.NewReader("payload-over-limit"), -1)
			c.Request().URI().SetQueryString("name=upload")

			require.Equal(t, "upload", c.FormValue("name"))
			if tc.notForm {
				require.Empty(t, c.FormValue("other"))
			}
			require.Equal(t, StatusOK, c.Response().StatusCode())

			body, err := io.ReadAll(c.BodyStream())
			require.NoError(t, err)
			require.Equal(t, "payload-over-limit", string(body))
		})
	}
}

func Test_Ctx_Body_With_Compression_BodyLimitExceeded(t *testing.T) {
	t.Parallel()
	c := apptest.CtxAs[*DefaultCtx](t, New(Config{BodyLimit: 8}))
	c.Request().Header.Set(HeaderContentEncoding, StrGzip)

	compressedBody := ctxTestACompress(t, StrGzip, []byte("payload-over-limit"))
	c.Request().SetBody(compressedBody)

	body := c.Body()
	require.Equal(t, []byte(fasthttp.ErrBodyTooLarge.Error()), body)
	require.Equal(t, compressedBody, c.Request().Body())
	require.Equal(t, StatusRequestEntityTooLarge, c.Response().StatusCode())
}

// ctxBenchCompression is one Body decoding sub-benchmark: a body sent with contentEncoding that must decode to want.
type ctxBenchCompression struct {
	contentEncoding string
	body            []byte
	want            []byte
}

// ctxBenchCompressions returns the Body decoding sub-benchmarks: a gzip body, an unsupported coding, a deflate body
// and a gzip,deflate chain whose inner layer is not deflate.
func ctxBenchCompressions(b *testing.B) []ctxBenchCompression {
	b.Helper()
	input := []byte("john=doe")
	gzipped, deflated := ctxTestACompress(b, StrGzip, input), ctxTestACompress(b, StrDeflate, input)
	// deflated, then gzipped: the reverse of the order "gzip,deflate" names
	gzippedDeflated := ctxTestACompress(b, StrGzip, deflated)
	return []ctxBenchCompression{
		{contentEncoding: "gzip", body: gzipped, want: input},
		{contentEncoding: "gzip,invalid", body: gzipped, want: []byte(ErrUnsupportedMediaType.Error())},
		{contentEncoding: StrDeflate, body: deflated, want: input},
		{contentEncoding: "gzip,deflate", body: gzippedDeflated, want: []byte(zlib.ErrHeader.Error())},
	}
}

// ctxBenchCompressedCtx returns a context of an app with the given Config.Immutable that holds bc's body.
//
//nolint:revive // flag-parameter: this selects which of the two benchmarks runs
func ctxBenchCompressedCtx(b *testing.B, immutable bool, bc ctxBenchCompression) Ctx {
	b.Helper()
	app := New()
	app.config.Immutable = immutable
	c := apptest.Ctx(b, app)
	c.Request().Header.Set("Content-Encoding", bc.contentEncoding)
	c.Request().SetBody(bc.body)
	return c
}

func Benchmark_Ctx_Body_With_Compression(b *testing.B) {
	b.ReportAllocs()
	for _, bc := range ctxBenchCompressions(b) {
		b.Run(bc.contentEncoding, func(b *testing.B) {
			c := ctxBenchCompressedCtx(b, false, bc)
			for b.Loop() {
				_ = c.Body()
			}

			require.Equal(b, bc.want, c.Body())
		})
	}
}

func Benchmark_Ctx_Body_With_Compression_Immutable(b *testing.B) {
	b.ReportAllocs()
	for _, bc := range ctxBenchCompressions(b) {
		b.Run(bc.contentEncoding, func(b *testing.B) {
			c := ctxBenchCompressedCtx(b, true, bc)
			for b.Loop() {
				_ = c.Body()
			}

			require.Equal(b, bc.want, c.Body())
		})
	}
}

func Test_Ctx_RequestCtx(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.Equal(t, "*fasthttp.RequestCtx", fmt.Sprintf("%T", c.RequestCtx()))
}

func Test_Ctx_Cookie(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

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
	require.Equal(t, "username=john; expires="+httpdate+"; path=/; SameSite=Lax", c.Res().Get(HeaderSetCookie))

	cookie.SameSite = CookieSameSiteDisabled
	c.Res().Cookie(cookie)
	require.Equal(t, "username=john; expires="+httpdate+"; path=/", c.Res().Get(HeaderSetCookie))

	cookie.SameSite = CookieSameSiteStrictMode
	c.Res().Cookie(cookie)
	require.Equal(t, "username=john; expires="+httpdate+"; path=/; SameSite=Strict", c.Res().Get(HeaderSetCookie))

	cookie.Secure = true
	cookie.SameSite = CookieSameSiteNoneMode
	c.Res().Cookie(cookie)
	require.Equal(t, "username=john; expires="+httpdate+"; path=/; secure; SameSite=None", c.Res().Get(HeaderSetCookie))

	// should remove expires and max-age headers
	cookie.SessionOnly = true
	cookie.Expires = expire
	cookie.MaxAge = 10000
	c.Res().Cookie(cookie)
	require.Equal(t, "username=john; path=/; secure; SameSite=None", c.Res().Get(HeaderSetCookie))

	// should remove expires and max-age headers when no expire and no MaxAge (default time)
	cookie.SessionOnly = false
	cookie.Expires = time.Time{}
	cookie.MaxAge = 0
	c.Res().Cookie(cookie)
	require.Equal(t, "username=john; path=/; secure; SameSite=None", c.Res().Get(HeaderSetCookie))

	cookie.Partitioned = true
	c.Res().Cookie(cookie)
	require.Equal(t, "username=john; path=/; secure; SameSite=None; Partitioned", c.Res().Get(HeaderSetCookie))
}

func Test_Ctx_Cookie_Attributes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		cookie *Cookie
		name   string
		want   string // the Set-Cookie line
	}{
		{
			name:   "partitioned secure",
			cookie: &Cookie{Name: "ps", Value: "v", Secure: true, SameSite: CookieSameSiteNoneMode, Partitioned: true},
			want:   "ps=v; path=/; secure; SameSite=None; Partitioned",
		},
		{
			// Path intentionally empty to verify defaulting
			name:   "default path",
			cookie: &Cookie{Name: "p", Value: "v"},
			want:   "p=v; path=/; SameSite=Lax",
		},
		{
			name:   "max-age only",
			cookie: &Cookie{Name: "ttl", Value: "v", MaxAge: 3600},
			want:   "ttl=v; max-age=3600; path=/; SameSite=Lax",
		},
		{
			name:   "strict partitioned",
			cookie: &Cookie{Name: "sp", Value: "v", Secure: true, SameSite: CookieSameSiteStrictMode, Partitioned: true},
			want:   "sp=v; path=/; secure; SameSite=Strict; Partitioned",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())

			c.Res().Cookie(tc.cookie)
			require.Equal(t, tc.want, c.Res().Get(HeaderSetCookie))
		})
	}
}

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

// A value with a space or comma is written in quotes, as net/http does (RFC 6265
// §4.1.1 allows neither even in quotes), and the quotes are not part of the value read back.
func Test_Ctx_Cookie_ValueQuoting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value string
		line  string // the Set-Cookie line
		read  string // the value read back from it
	}{
		{name: "plain", value: "abc123", line: "n=abc123; path=/; SameSite=Lax", read: "abc123"},
		{name: "empty", value: "", line: "n=; path=/; SameSite=Lax", read: ""},
		{name: "space", value: "hello world", line: `n="hello world"; path=/; SameSite=Lax`, read: "hello world"},
		{name: "comma", value: "x,y", line: `n="x,y"; path=/; SameSite=Lax`, read: "x,y"},
		{name: "space and comma", value: "a b, c", line: `n="a b, c"; path=/; SameSite=Lax`, read: "a b, c"},
		{name: "edge spaces", value: " a ", line: `n=" a "; path=/; SameSite=Lax`, read: " a "},
		{name: "already quoted", value: `"abc"`, line: `n="abc"; path=/; SameSite=Lax`, read: "abc"},
		{name: "already quoted with a space", value: `"a b"`, line: `n="a b"; path=/; SameSite=Lax`, read: "a b"},
		{name: "empty and quoted", value: `""`, line: `n=""; path=/; SameSite=Lax`, read: ""},
		{name: "visible ASCII", value: "!#$%&'()*+-./:<=>?@[]^_`{|}~", line: "n=!#$%&'()*+-./:<=>?@[]^_`{|}~; path=/; SameSite=Lax", read: "!#$%&'()*+-./:<=>?@[]^_`{|}~"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())

			c.Res().Cookie(&Cookie{Name: "n", Value: tc.value})
			require.Equal(t, tc.line, c.Res().Get(HeaderSetCookie))

			got, ok := c.Res().GetCookie("n")
			require.True(t, ok)
			require.Equal(t, tc.read, got.Value)

			// The client sends the pair back as it received it.
			pair, _, _ := strings.Cut(c.Res().Get(HeaderSetCookie), ";")
			c.Request().Header.Set(HeaderCookie, pair)
			require.Equal(t, tc.read, c.Cookies("n"))
		})
	}
}

// Values the grammar cannot carry, even in quotes, are refused, not altered.
func Test_Ctx_Cookie_ValueRejected(t *testing.T) {
	t.Parallel()
	for name, value := range map[string]string{
		"semicolon":                 "a;b",
		"semicolon in quotes":       `"a;b"`,
		"double quote":              `a"b`,
		"double quote inside quote": `"a"b"`,
		"lone double quote":         `"`,
		"opening quote only":        `"abc`,
		"closing quote only":        `abc"`,
		"backslash":                 `a\b`,
		"backslash in quotes":       `"a\b"`,
		"tab":                       "a\tb",
		"line feed":                 "a\nb",
		"non-ASCII":                 "café",
		"DEL":                       "a\x7fb",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())

			c.Res().Cookie(&Cookie{Name: "n", Value: value})
			require.Empty(t, c.Res().Get(HeaderSetCookie))
		})
	}
}

// net/http reads back the values that were set.
func Test_Ctx_Cookie_ValueQuoting_NetHTTP(t *testing.T) {
	t.Parallel()
	values := []string{"plain", "hello world", "x,y", "a b, c", `"abc"`, `"a b"`}

	app := New()
	app.Get("/", func(c Ctx) error {
		for i, value := range values {
			c.Cookie(&Cookie{Name: "c" + strconv.Itoa(i), Value: value})
		}
		return nil
	})

	cookies := apptest.Do(t, app, "/").Cookies()
	require.Len(t, cookies, len(values))
	for i, cookie := range cookies {
		require.Equal(t, "c"+strconv.Itoa(i), cookie.Name)
		require.Equal(t, strings.Trim(values[i], `"`), cookie.Value)
	}
}

func Test_Ctx_Cookie_SameSite_CaseInsensitive(t *testing.T) {
	t.Parallel()
	app := New()

	tests := []struct {
		name     string
		input    string
		expected string // the SameSite attribute; none when empty
		secure   bool   // no cookie sets Secure: only SameSite=None makes it secure
	}{
		{"Strict lowercase", "strict", "SameSite=Strict", false},
		{"Strict uppercase", "STRICT", "SameSite=Strict", false},
		{"Strict mixed case", "StRiCt", "SameSite=Strict", false},
		{"Strict proper case", "Strict", "SameSite=Strict", false},
		{"Lax lowercase", "lax", "SameSite=Lax", false},
		{"Lax uppercase", "LAX", "SameSite=Lax", false},
		{"Lax mixed case", "LaX", "SameSite=Lax", false},
		{"Lax proper case", "Lax", "SameSite=Lax", false},
		{"None lowercase", "none", "SameSite=None", true},
		{"None uppercase", "NONE", "SameSite=None", true},
		{"None mixed case", "NoNe", "SameSite=None", true},
		{"None proper case", "None", "SameSite=None", true},
		{"Disabled lowercase", "disabled", "", false},
		{"Disabled uppercase", "DISABLED", "", false},
		{"Disabled mixed case", "DiSaBlEd", "", false},
		// invalid values default to Lax
		{"Invalid value", "invalid", "SameSite=Lax", false},
		{"Empty value", "", "SameSite=Lax", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)
			c.Response().Reset()

			cookie := &Cookie{Name: "test", Value: "value", SameSite: tc.input}
			c.Res().Cookie(cookie)

			setCookieHeader := c.Res().Get(HeaderSetCookie)
			if tc.expected == "" {
				require.NotContains(t, setCookieHeader, "SameSite")
			} else {
				require.Contains(t, setCookieHeader, tc.expected)
			}
			if tc.secure {
				require.Contains(t, setCookieHeader, "secure")
			} else {
				require.NotContains(t, setCookieHeader, "secure")
			}
		})
	}
}

func Benchmark_Ctx_Cookie(b *testing.B) {
	app := New()
	c := apptest.CtxAs[*DefaultCtx](b, app)

	b.ReportAllocs()
	for b.Loop() {
		c.Cookie(&Cookie{Name: "John", Value: "Doe"})
	}
	require.Equal(b, "John=Doe; path=/; SameSite=Lax", app.toString(c.Response().Header.Peek("Set-Cookie")))
}

func Test_Ctx_Cookies(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().Header.Set("Cookie", "john=doe")
	require.Equal(t, "doe", c.Req().Cookies("john"))
	require.Equal(t, "default", c.Req().Cookies("unknown", "default"))
}

// ctxTestAFormats returns a ResFmt for each media type whose handler records
// the media type it was chosen for in chosen.
func ctxTestAFormats(chosen *string, mediaTypes ...string) []ResFmt {
	formats := make([]ResFmt, 0, len(mediaTypes))
	for _, mediaType := range mediaTypes {
		formats = append(formats, ResFmt{MediaType: mediaType, Handler: func(_ Ctx) error {
			*chosen = mediaType
			return nil
		}})
	}
	return formats
}

func Test_Ctx_Format(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	// set `accepted` to whatever media type was chosen by Format
	var accepted string

	c.Request().Header.Set(HeaderAccept, `text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7`)
	err := c.Res().Format(ctxTestAFormats(&accepted, "application/xhtml+xml", "application/xml", "foo/bar")...)
	require.Equal(t, "application/xhtml+xml", accepted)
	require.Equal(t, "application/xhtml+xml", c.GetRespHeader(HeaderContentType))
	require.Equal(t, "application/xhtml+xml", c.Res().Get(HeaderContentType))
	require.NoError(t, err)
	require.NotEqual(t, StatusNotAcceptable, c.Response().StatusCode())

	err = c.Res().Format(ctxTestAFormats(&accepted, "foo/bar;a=b")...)
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

	err = c.Format(ctxTestAFormats(&accepted, "text/html", "default")...)
	require.Equal(t, "default", accepted)
	require.Equal(t, "text/html", c.GetRespHeader(HeaderContentType))
	require.Equal(t, "text/html", c.Res().Get(HeaderContentType))
	require.NoError(t, err)

	err = c.Format()
	require.ErrorIs(t, err, ErrNoHandlers)
}

func Test_Ctx_Format_NilHandler(t *testing.T) {
	t.Parallel()
	ok := func(_ Ctx) error { return nil }

	for _, tc := range []struct {
		name          string
		accept        string // the Accept header, none when empty
		wantMediaType string // the media type the error names
		wantIndex     string // the entry the error points at
		formats       []ResFmt
	}{
		{
			name: "nil handler in first entry", wantMediaType: `media type "text/html"`, wantIndex: "index 0",
			formats: []ResFmt{{MediaType: "text/html", Handler: nil}, {MediaType: "application/json", Handler: ok}},
		},
		{
			name: "nil handler in matched media type", accept: "application/json",
			wantMediaType: `media type "application/json"`, wantIndex: "index 1",
			formats: []ResFmt{{MediaType: "text/html", Handler: ok}, {MediaType: "application/json", Handler: nil}},
		},
		{
			name: "nil default handler", accept: "application/json",
			wantMediaType: `media type "default"`, wantIndex: "index 1",
			formats: []ResFmt{{MediaType: "text/html", Handler: ok}, {MediaType: "default", Handler: nil}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())
			if tc.accept != "" {
				c.Request().Header.Set(HeaderAccept, tc.accept)
			}

			var err error
			require.NotPanics(t, func() { err = c.Format(tc.formats...) })
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantMediaType)
			require.Contains(t, err.Error(), tc.wantIndex)
		})
	}
}

// Test_Ctx_Format_NoAcceptDefaultFirst verifies that a request without an
// Accept header never emits the literal "default" as a Content-Type value:
// the first real media type wins regardless of the position of the "default"
// entry, and a lone "default" handler leaves Content-Type untouched.
func Test_Ctx_Format_NoAcceptDefaultFirst(t *testing.T) {
	t.Parallel()
	app := New()

	var accepted string

	// "default" listed first must be skipped in favor of a real media type.
	c := apptest.Ctx(t, app)
	err := c.Format(ctxTestAFormats(&accepted, "default", "application/json")...)
	require.NoError(t, err)
	require.Equal(t, "application/json", accepted)
	require.Equal(t, "application/json", c.GetRespHeader(HeaderContentType))

	// Only a "default" handler: it runs, but no Content-Type is fabricated.
	c = app.AcquireCtx(&fasthttp.RequestCtx{})
	err = c.Format(ctxTestAFormats(&accepted, "default")...)
	require.NoError(t, err)
	require.Equal(t, "default", accepted)
	require.NotEqual(t, "default", c.GetRespHeader(HeaderContentType))
}

func Test_Ctx_Format_AcceptEmptyAndMultiLine(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		want   string
		accept []string // the Accept field lines
	}{
		// A single empty Accept line is an empty combined view: treated as absent.
		{name: "single empty line", accept: []string{""}, want: "application/json"},
		// Multiple Accept lines negotiate on the combined view (RFC 9110 Section 5.2).
		{name: "multiple lines", accept: []string{"text/plain;q=0.5", "text/html"}, want: "text/html"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())
			for _, line := range tc.accept {
				c.Request().Header.Add(HeaderAccept, line)
			}

			var accepted string
			require.NoError(t, c.Format(ctxTestAFormats(&accepted, "application/json", "text/html")...))
			require.Equal(t, tc.want, accepted)
		})
	}
}

func Benchmark_Ctx_Format(b *testing.B) {
	c := apptest.Ctx(b, New())
	c.Request().Header.Set(HeaderAccept, "application/json,text/plain; format=flowed; q=0.9")

	fail := func(_ Ctx) error {
		require.FailNow(b, "Wrong type chosen")
		return errors.New("Wrong type chosen")
	}
	ok := func(_ Ctx) error { return nil }

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

	// preAllocated measures Format with offers built before the loop.
	preAllocated := func(name string, offers ...ResFmt) {
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				err = c.Format(offers...)
			}
			require.NoError(b, err)
		})
	}
	preAllocated("pre-allocated args",
		ResFmt{MediaType: "application/xml", Handler: fail}, ResFmt{MediaType: "text/html", Handler: fail},
		ResFmt{MediaType: "text/plain;format=fixed", Handler: fail},
		ResFmt{MediaType: "text/plain;format=flowed", Handler: ok})

	c.Request().Header.Set("Accept", "text/plain")
	preAllocated("text/plain", ResFmt{MediaType: "application/xml", Handler: fail},
		ResFmt{MediaType: "text/plain", Handler: ok})

	c.Request().Header.Set("Accept", "json")
	preAllocated("json",
		ResFmt{MediaType: "xml", Handler: fail}, ResFmt{MediaType: "html", Handler: fail},
		ResFmt{MediaType: "json", Handler: ok})

	c.Request().Header.Set("Accept", MIMEApplicationMsgPack)
	preAllocated("msgpack",
		ResFmt{MediaType: "xml", Handler: fail}, ResFmt{MediaType: "html", Handler: fail},
		ResFmt{MediaType: MIMEApplicationMsgPack, Handler: ok})
}

func Test_Ctx_AutoFormat(t *testing.T) {
	t.Parallel()
	type Message struct {
		Sender     string `xml:"sender,attr"`
		Recipients []string
		Urgency    int `xml:"urgency,attr"`
	}
	message := Message{Recipients: []string{"Alice", "Bob"}, Sender: "Carol", Urgency: 3}
	type broken string

	tests := []struct {
		body        any
		name        string
		accept      string
		contentType string
		want        string // the response body
		wantErr     bool
	}{
		{name: "bytes as text", accept: MIMETextPlain, body: []byte("Hello, World!"), contentType: MIMETextPlainCharsetUTF8, want: "Hello, World!"},
		{name: "string as HTML", accept: MIMETextHTML, body: "Hello, World!", contentType: MIMETextHTMLCharsetUTF8, want: "<p>Hello, World!</p>"},
		{name: "string as JSON", accept: MIMEApplicationJSON, body: "Hello, World!", contentType: MIMEApplicationJSONCharsetUTF8, want: `"Hello, World!"`},
		{name: "string as MsgPack", accept: MIMEApplicationMsgPack, body: "Hello, World!", contentType: MIMEApplicationMsgPack, want: "\xadHello, World!"},
		{name: "string as CBOR", accept: MIMEApplicationCBOR, body: "Hello, World!", contentType: MIMEApplicationCBOR, want: "\x6dHello, World!"},
		{name: "complex number as text", accept: MIMETextPlain, body: complex(1, 1), contentType: MIMETextPlainCharsetUTF8, want: "(1+1i)"},
		{name: "string as XML", accept: MIMEApplicationXML, body: "Hello, World!", contentType: MIMEApplicationXMLCharsetUTF8, want: `<string>Hello, World!</string>`},
		{name: "complex number as XML", accept: MIMEApplicationXML, body: complex(1, 1), wantErr: true},
		{name: "map as text", accept: MIMETextPlain, body: Map{}, contentType: MIMETextPlainCharsetUTF8, want: "map[]"},
		{name: "unknown media type falls back to text", accept: "broken/accept", body: broken("Hello, World!"), contentType: MIMETextPlainCharsetUTF8, want: "Hello, World!"},

		{name: "struct as JSON", accept: MIMEApplicationJSON, body: message, contentType: MIMEApplicationJSONCharsetUTF8, want: `{"Sender":"Carol","Recipients":["Alice","Bob"],"Urgency":3}`},
		{
			name: "struct as MsgPack", accept: MIMEApplicationMsgPack, body: message, contentType: MIMEApplicationMsgPack,
			// {"Sender":"Carol","Recipients":["Alice","Bob"],"Urgency":3}
			want: "\x83\xa6Sender\xa5Carol\xaaRecipients\x92\xa5Alice\xa3Bob\xa7Urgency\x03",
		},
		{
			name: "struct as CBOR", accept: MIMEApplicationCBOR, body: message, contentType: MIMEApplicationCBOR,
			// a36653656e646572654361726f6c6a526563697069656e74738265416c69636563426f6267557267656e637903
			want: "\xa3fSendereCaroljRecipients\x82eAlicecBobgUrgency\x03",
		},
		{
			name: "struct as XML", accept: MIMEApplicationXML, body: message, contentType: MIMEApplicationXMLCharsetUTF8,
			want: `<Message sender="Carol" urgency="3"><Recipients>Alice</Recipients><Recipients>Bob</Recipients></Message>`,
		},
	}

	// Every case runs through c.AutoFormat and through c.Res().AutoFormat.
	for _, accessor := range []struct {
		autoFormat func(Ctx, any) error
		name       string
	}{
		{name: "Ctx", autoFormat: Ctx.AutoFormat},
		{name: "Res", autoFormat: func(c Ctx, body any) error { return c.Res().AutoFormat(body) }},
	} {
		t.Run(accessor.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					c := apptest.Ctx(t, New(Config{
						MsgPackEncoder: msgpack.Marshal,
						MsgPackDecoder: msgpack.Unmarshal,
						CBOREncoder:    cbor.Marshal,
						CBORDecoder:    cbor.Unmarshal,
					}))

					c.Request().Header.Set(HeaderAccept, tc.accept)
					err := accessor.autoFormat(c, tc.body)
					if tc.wantErr {
						require.Error(t, err)
						return
					}
					require.NoError(t, err)
					require.Equal(t, tc.contentType, c.GetRespHeader(HeaderContentType))
					require.Equal(t, tc.want, string(c.Response().Body()))
					// the representation is selected by proactive negotiation (RFC 9110 §12.5.5)
					require.Equal(t, HeaderAccept, c.GetRespHeader(HeaderVary))
				})
			}
		})
	}
}

func Test_Ctx_AutoFormat_XSS_Prevention(t *testing.T) {
	t.Parallel()
	// autoFormatHTML renders body for a client that accepts HTML.
	autoFormatHTML := func(t *testing.T, body any) string {
		t.Helper()

		c := apptest.Ctx(t, New())
		c.Request().Header.Set(HeaderAccept, MIMETextHTML)
		require.NoError(t, c.AutoFormat(body))
		require.Equal(t, MIMETextHTMLCharsetUTF8, c.GetRespHeader(HeaderContentType))
		return string(c.Response().Body())
	}

	for _, tc := range []struct {
		body      any
		name      string
		want      string
		forbidden string // must not appear in the response body
		msg       string // why forbidden must not appear
	}{
		{
			name: "script tag", body: "<script>alert('XSS')</script>",
			want: "<p>&lt;script&gt;alert(&#39;XSS&#39;)&lt;/script&gt;</p>", forbidden: "<script>", msg: "Script tags should be escaped",
		},
		{
			name: "img onerror", body: "<img src=x onerror=alert('XSS')>",
			want: "<p>&lt;img src=x onerror=alert(&#39;XSS&#39;)&gt;</p>", forbidden: "<img", msg: "Img tags should be escaped",
		},
		{
			name: "iframe", body: "<iframe src='javascript:alert(\"XSS\")'></iframe>",
			want: "<p>&lt;iframe src=&#39;javascript:alert(&#34;XSS&#34;)&#39;&gt;&lt;/iframe&gt;</p>", forbidden: "<iframe", msg: "Iframe tags should be escaped",
		},
		{
			name: "event handler attributes", body: "<div onload=alert('XSS')>test</div>",
			want: "<p>&lt;div onload=alert(&#39;XSS&#39;)&gt;test&lt;/div&gt;</p>", forbidden: "<div", msg: "Div tags should be escaped",
		},
		{
			name: "link javascript protocol", body: "<a href='javascript:alert(\"XSS\")'>Click me</a>",
			want: "<p>&lt;a href=&#39;javascript:alert(&#34;XSS&#34;)&#39;&gt;Click me&lt;/a&gt;</p>", forbidden: "<a href", msg: "Anchor tags should be escaped",
		},
		{
			name: "mixed quotes", body: `"><script>alert('XSS')</script><"`,
			want: `<p>&#34;&gt;&lt;script&gt;alert(&#39;XSS&#39;)&lt;/script&gt;&lt;&#34;</p>`, forbidden: "<script>", msg: "Script tags with quotes should be escaped",
		},
		// legitimate HTML special characters are escaped
		{name: "special characters", body: "Price: 5 < 10 & 20 > 15", want: "<p>Price: 5 &lt; 10 &amp; 20 &gt; 15</p>"},
		// pre-rendered HTML is treated as plain text
		{name: "pre-rendered HTML", body: "<b>Hello, World!</b>", want: "<p>&lt;b&gt;Hello, World!&lt;/b&gt;</p>"},
		// normal text without special chars works fine
		{name: "plain text", body: "Hello, World!", want: "<p>Hello, World!</p>"},
		{name: "byte slice", body: []byte("<script>alert('XSS')</script>"), want: "<p>&lt;script&gt;alert(&#39;XSS&#39;)&lt;/script&gt;</p>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := autoFormatHTML(t, tc.body)
			require.Equal(t, tc.want, body)
			if tc.forbidden != "" {
				require.NotContains(t, body, tc.forbidden, tc.msg)
			}
		})
	}

	t.Run("struct", func(t *testing.T) {
		t.Parallel()
		body := autoFormatHTML(t, struct{ Value string }{Value: "<script>alert('XSS')</script>"})
		// When formatted as a string via fmt.Sprintf with %v, the struct becomes something like {<script>...}
		require.NotContains(t, body, "<script>", "Script tags in struct should be escaped")
		require.Contains(t, body, "&lt;script&gt;", "Escaped script tags should be present")
	})
}

// ctxBenchAccepting returns a context of an app with config whose request accepts accept.
func ctxBenchAccepting(b *testing.B, accept string, config ...Config) Ctx {
	b.Helper()
	c := apptest.Ctx(b, New(config...))
	c.Request().Header.Set("Accept", accept)
	return c
}

func Benchmark_Ctx_AutoFormat(b *testing.B) {
	c := ctxBenchAccepting(b, "text/plain")
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.AutoFormat("Hello, World!")
	}
	require.NoError(b, err)
	require.Equal(b, `Hello, World!`, string(c.Response().Body()))
}

func Benchmark_Ctx_AutoFormat_HTML(b *testing.B) {
	c := ctxBenchAccepting(b, "text/html")
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.AutoFormat("Hello, World!")
	}
	require.NoError(b, err)
	require.Equal(b, "<p>Hello, World!</p>", string(c.Response().Body()))
}

func Benchmark_Ctx_AutoFormat_JSON(b *testing.B) {
	c := ctxBenchAccepting(b, "application/json")
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.AutoFormat("Hello, World!")
	}
	require.NoError(b, err)
	require.Equal(b, `"Hello, World!"`, string(c.Response().Body()))
}

func Benchmark_Ctx_AutoFormat_MsgPack(b *testing.B) {
	c := ctxBenchAccepting(b, MIMEApplicationMsgPack, Config{MsgPackEncoder: msgpack.Marshal, MsgPackDecoder: msgpack.Unmarshal})
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.AutoFormat("Hello, World!")
	}
	require.NoError(b, err)
	require.Equal(b, "\xadHello, World!", string(c.Response().Body()))
}

func Benchmark_Ctx_AutoFormat_XML(b *testing.B) {
	c := ctxBenchAccepting(b, "application/xml")
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.AutoFormat("Hello, World!")
	}
	require.NoError(b, err)
	require.Equal(b, `<string>Hello, World!</string>`, string(c.Response().Body()))
}

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
		defer func() { require.NoError(t, f.Close()) }()

		b := new(bytes.Buffer)
		_, err = io.Copy(b, f)
		require.NoError(t, err)
		require.Equal(t, "hello world", b.String())
		return nil
	})

	ctxTestAExpectEmptyOK(t, app, ctxTestBHelloWorldUpload(t))
}

// ctxTestMultipartField returns a multipart body that holds the single form
// field name=value, and the Content-Type that announces it.
func ctxTestMultipartField(t *testing.T, name, value string) (body []byte, contentType string) { //nolint:nonamedreturns // gocritic's unnamedResult asks for names
	t.Helper()
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	require.NoError(t, writer.WriteField(name, value))
	require.NoError(t, writer.Close())
	return buf.Bytes(), writer.FormDataContentType()
}

// ctxTestMultipartFile returns a multipart body that uploads data as the form file field, named filename, and the
// Content-Type that announces it.
func ctxTestMultipartFile(t *testing.T, field, filename string, data []byte) (body []byte, contentType string) { //nolint:nonamedreturns // gocritic's unnamedResult asks for names
	t.Helper()
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	part, err := writer.CreateFormFile(field, filename)
	require.NoError(t, err)
	n, err := part.Write(data)
	require.NoError(t, err)
	require.Len(t, data, n)
	require.NoError(t, writer.Close())
	return buf.Bytes(), writer.FormDataContentType()
}

func Test_Ctx_FormValue(t *testing.T) {
	t.Parallel()
	app := New()

	app.Post("/test", func(c Ctx) error {
		require.Equal(t, "john", c.FormValue("name"))
		return nil
	})

	body, contentType := ctxTestMultipartField(t, "name", "john")
	req := httptest.NewRequest(MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
	ctxTestAExpectEmptyOK(t, app, req)
}

func Test_Ctx_FormFile_BodyLimitExceeded(t *testing.T) {
	t.Parallel()
	c := apptest.CtxAs[*DefaultCtx](t, New(Config{BodyLimit: 64}))

	body, contentType := ctxTestMultipartFile(t, "file", "test.txt", []byte(strings.Repeat("a", 256)))
	c.Request().Header.Set(HeaderContentType, contentType)
	c.Request().SetBody(body)

	_, err := c.FormFile("file")
	require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
}

// FormValue looks a key up in the query args and the post args before it parses
// the multipart form of a multipart request; any other body is read as a form.
func Test_Ctx_FormValue_Lookup(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		field     string // the multipart form holds field=value; none: the body is name=carol, URL-encoded
		value     string
		query     string // the name query arg, if any
		post      string // the name post arg, if any
		key       string
		want      string // what FormValue(key) returns
		bodyLimit int
		fallback  bool // FormValue("missing", "fallback") then returns the default
	}{
		// the default, an empty string, when the body limit is exceeded
		{name: "BodyLimitExceeded", bodyLimit: 64, field: "name", value: strings.Repeat("a", 256), key: "name"},
		{name: "QueryArgs", field: "other", value: "value", query: "alice", key: "name", want: "alice"},
		{name: "PostArgs", field: "other", value: "value", post: "bob", key: "name", want: "bob"},
		// a key absent from the multipart form: "" without a default, the default with one
		{name: "MultipartKeyNotFound", field: "other", value: "value", key: "missing", fallback: true},
		{name: "NonMultipart", key: "name", want: "carol", fallback: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.CtxAs[*DefaultCtx](t, New(Config{BodyLimit: tc.bodyLimit}))
			if tc.field != "" {
				body, contentType := ctxTestMultipartField(t, tc.field, tc.value)
				c.Request().Header.Set(HeaderContentType, contentType)
				c.Request().SetBody(body)
			} else {
				c.Request().Header.Set(HeaderContentType, MIMEApplicationForm)
				c.Request().SetBodyString("name=carol")
			}
			if tc.query != "" {
				c.Request().URI().QueryArgs().Set("name", tc.query)
			}
			if tc.post != "" {
				c.Request().PostArgs().Set("name", tc.post)
			}

			require.Equal(t, tc.want, c.FormValue(tc.key))
			if tc.fallback {
				require.Equal(t, "fallback", c.FormValue("missing", "fallback"))
			}
		})
	}
}

// Test_Ctx_Form_MixedCaseContentType asserts that Ctx's own form accessors
// accept the media type and parameter names in any case (RFC 9110 Sections
// 8.3.1 and 5.6.6). fasthttp matches both case-sensitively, so without the
// normalization these return nothing at all — and a handler should not have to
// call Bind first to get its own form back.
func Test_Ctx_Form_MixedCaseContentType(t *testing.T) {
	t.Parallel()
	for _, accessor := range []string{"FormValue", "MultipartForm", "Bind"} {
		t.Run("quoted_decoy_"+accessor, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())
			c.Request().Header.SetMethod(MethodPost)
			c.Request().Header.Set(HeaderContentType, `Multipart/Form-Data; X= "; BOUNDARY=bogus"; BOUNDARY=Real`)
			c.Request().SetBodyString("--Real\r\nContent-Disposition: form-data; name=\"name\"\r\n\r\ncarol\r\n--Real--\r\n")

			switch accessor {
			case "FormValue":
				require.Equal(t, "carol", c.FormValue("name"))
			case "MultipartForm":
				form, err := c.MultipartForm()
				require.NoError(t, err)
				require.Equal(t, []string{"carol"}, form.Value["name"])
			case "Bind":
				var out struct {
					Name string `form:"name"`
				}
				require.NoError(t, c.Bind().Form(&out))
				require.Equal(t, "carol", out.Name)
			}
		})
	}

	t.Run("urlencoded", func(t *testing.T) {
		t.Parallel()
		c := apptest.CtxAs[*DefaultCtx](t, New())

		c.Request().Header.SetMethod(MethodPost)
		c.Request().Header.Set(HeaderContentType, "Application/X-WWW-Form-Urlencoded")
		c.Request().SetBodyString("name=carol")

		require.Equal(t, "carol", c.FormValue("name"))
	})

	t.Run("multipart", func(t *testing.T) {
		t.Parallel()
		c := apptest.CtxAs[*DefaultCtx](t, New())

		// The boundary parameter *name* is folded, its value is not.
		c.Request().Header.SetMethod(MethodPost)
		c.Request().Header.Set(HeaderContentType, `Multipart/Form-Data; BOUNDARY=AbC`)
		c.Request().SetBodyString("--AbC\r\nContent-Disposition: form-data; name=\"name\"\r\n\r\ncarol\r\n--AbC--\r\n")

		form, err := c.MultipartForm()
		require.NoError(t, err)
		require.Equal(t, []string{"carol"}, form.Value["name"])
		require.Equal(t, "carol", c.FormValue("name"))
	})
}

func Benchmark_Ctx_Fresh_StaleEtag(b *testing.B) {
	c := apptest.Ctx(b, New())

	for b.Loop() {
		c.Request().Header.Set(HeaderIfNoneMatch, `"a", "b", "c", "d"`)
		c.Request().Header.Set(HeaderCacheControl, "c")
		c.Fresh()

		c.Request().Header.Set(HeaderIfNoneMatch, `"a", "b", "c", "d"`)
		c.Request().Header.Set(HeaderCacheControl, "e")
		c.Fresh()
	}
}

func Test_Ctx_Fresh(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.False(t, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, `"675af34563dc-tr34"`)
	c.Request().Header.Set(HeaderCacheControl, "public")
	require.False(t, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, `"a", "b"`)
	c.Response().Header.Set(HeaderETag, `"c"`)
	require.False(t, c.Fresh())

	c.Response().Header.Set(HeaderETag, `"a"`)
	require.True(t, c.Fresh())

	// If-None-Match takes precedence over If-Modified-Since (RFC 9110): once the
	// ETag matches, the response is fresh regardless of the date headers, even
	// when Last-Modified is newer than If-Modified-Since (stale under dates alone).
	c.Request().Header.Set(HeaderIfModifiedSince, "Wed, 21 Oct 2015 07:27:59 GMT")
	c.Response().Header.Set(HeaderLastModified, "Wed, 21 Oct 2015 07:28:00 GMT")
	require.True(t, c.Fresh())

	// A wildcard If-None-Match is fresh too, ignoring the date headers.
	c.Request().Header.Set(HeaderIfNoneMatch, "*")
	require.True(t, c.Fresh())
}

func Test_Ctx_Fresh_ResponseStatus(t *testing.T) {
	t.Parallel()
	app := New()
	conditionals := []struct{ name, header, value string }{
		{name: "If-None-Match", header: HeaderIfNoneMatch, value: `"version"`},
		{name: "wildcard If-None-Match", header: HeaderIfNoneMatch, value: "*"},
		{name: "If-Modified-Since", header: HeaderIfModifiedSince, value: "Wed, 21 Oct 2015 07:28:00 GMT"},
	}

	for _, tc := range []struct {
		status int
		fresh  bool
	}{
		{StatusOK, true},
		{StatusNoContent, true},
		{StatusPartialContent, true},
		{299, true},
		{StatusMovedPermanently, false},
		{StatusFound, false},
		{StatusNotModified, true},
		{StatusTemporaryRedirect, false},
		{StatusBadRequest, false},
		{StatusNotFound, false},
		{StatusPreconditionFailed, false},
		{StatusInternalServerError, false},
	} {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			t.Parallel()
			for _, conditional := range conditionals {
				t.Run(conditional.name, func(t *testing.T) {
					t.Parallel()
					c := apptest.Ctx(t, app)
					c.Method(MethodGet)
					c.Request().Header.Set(conditional.header, conditional.value)
					c.Response().Header.Set(HeaderETag, `"version"`)
					c.Response().Header.Set(HeaderLastModified, "Wed, 21 Oct 2015 07:28:00 GMT")
					c.Status(tc.status)

					require.Equal(t, tc.fresh, c.Fresh())
					require.Equal(t, !tc.fresh, c.Stale())
				})
			}
		})
	}
}

func Test_Ctx_Fresh_PreservesNotFoundResponse(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		c.Status(StatusNotFound)
		if c.Fresh() {
			return c.SendStatus(StatusNotModified)
		}
		return c.SendString("resource no longer exists")
	})

	req := apptest.Req(MethodGet, "/", apptest.WithHeader(HeaderIfNoneMatch, "*"))
	apptest.ExpectBody(t, app, req, StatusNotFound, "resource no longer exists")
}

func Test_Ctx_Fresh_NoCache(t *testing.T) {
	t.Parallel()
	// A no-cache directive makes a wildcard If-None-Match stale, wherever it sits in the list.
	for _, tc := range []struct {
		name         string
		cacheControl string
	}{
		{name: "alone", cacheControl: "no-cache"},
		{name: "between empty elements", cacheControl: ",no-cache,"},
		{name: "after another directive", cacheControl: "aa,no-cache,"},
		{name: "before another directive", cacheControl: ",no-cache,bb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())

			c.Request().Header.Set(HeaderIfNoneMatch, "*")
			c.Request().Header.Set(HeaderCacheControl, tc.cacheControl)
			require.False(t, c.Fresh())
		})
	}
}

// Test_Ctx_Fresh_ModifiedSince verifies that If-Modified-Since is evaluated
// even when If-None-Match is absent from the request. Previously the date check
// was nested inside the if-none-match branch, so an If-Modified-Since-only
// request always reported the response as fresh regardless of Last-Modified.
// It also verifies that If-Modified-Since values in the obsolete RFC 850 and
// ANSI C asctime() formats are accepted, as required by RFC 9110 §5.6.7 for
// any HTTP-date recipient.
func Test_Ctx_Fresh_ModifiedSince(t *testing.T) {
	t.Parallel()
	const (
		date      = "Wed, 21 Oct 2015 07:28:00 GMT"
		laterDate = "Wed, 21 Oct 2015 07:29:00 GMT"
	)

	for _, tc := range []struct {
		name            string
		ifModifiedSince string
		lastModified    string // the response has no Last-Modified when empty
		fresh           bool
	}{
		// Last-Modified newer than the client's date: the resource changed, so it
		// is NOT fresh and the full response should be sent.
		{name: "Last-Modified newer", ifModifiedSince: date, lastModified: laterDate, fresh: false},
		// Last-Modified equal to the client's date: still fresh.
		{name: "Last-Modified equal", ifModifiedSince: date, lastModified: date, fresh: true},
		// Last-Modified older than the client's date: fresh.
		{name: "Last-Modified older", ifModifiedSince: laterDate, lastModified: date, fresh: true},
		// If-Modified-Since present but no Last-Modified response header: stale.
		{name: "no Last-Modified", ifModifiedSince: date, fresh: false},
		// Malformed Last-Modified with a valid If-Modified-Since: stale.
		{name: "malformed Last-Modified", ifModifiedSince: date, lastModified: "not-a-date", fresh: false},
		// Malformed If-Modified-Since with a valid Last-Modified: stale.
		{name: "malformed If-Modified-Since", ifModifiedSince: "not-a-date", lastModified: date, fresh: false},

		// Not modified since the client's date: fresh in the obsolete formats too
		// (the IMF-fixdate case is "Last-Modified equal").
		{name: "obsolete RFC 850 format", ifModifiedSince: "Wednesday, 21-Oct-15 07:28:00 GMT", lastModified: date, fresh: true},
		{name: "ANSI C asctime() format", ifModifiedSince: "Wed Oct 21 07:28:00 2015", lastModified: date, fresh: true},
		// surrounding OWS tolerated (RFC 9110 §5.5)
		{name: "surrounding whitespace", ifModifiedSince: " " + date + " ", lastModified: date, fresh: true},
		// Modified after the client's obsolete-format date: stale.
		{name: "modified after the RFC 850 date", ifModifiedSince: "Wednesday, 21-Oct-15 07:28:00 GMT", lastModified: laterDate, fresh: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())
			c.Request().Header.Set(HeaderIfModifiedSince, tc.ifModifiedSince)
			if tc.lastModified != "" {
				c.Response().Header.Set(HeaderLastModified, tc.lastModified)
			}
			require.Equal(t, tc.fresh, c.Fresh())
		})
	}
}

// Test_Ctx_Fresh_MethodGuard verifies that freshness only applies to GET and
// HEAD requests. RFC 9110 §13.1.3 requires If-Modified-Since to be ignored for
// other methods, and a 304 Not Modified response is only defined for GET/HEAD.
func Test_Ctx_Fresh_MethodGuard(t *testing.T) {
	t.Parallel()
	app := New()

	for _, method := range []string{MethodGet, MethodHead} {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Method(method)
		c.Request().Header.Set(HeaderIfNoneMatch, `"a"`)
		c.Response().Header.Set(HeaderETag, `"a"`)
		require.True(t, c.Fresh(), "matching ETag must be fresh for %s", method)
	}

	for _, method := range []string{MethodPost, MethodPut, MethodDelete, MethodPatch, MethodOptions} {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Method(method)

		// Matching If-None-Match must not report fresh: 304 is undefined for
		// these methods (a 412 would apply instead, which Fresh cannot express).
		c.Request().Header.Set(HeaderIfNoneMatch, `"a"`)
		c.Response().Header.Set(HeaderETag, `"a"`)
		require.False(t, c.Fresh(), "%s must never be fresh", method)

		// If-Modified-Since must be ignored for methods other than GET/HEAD.
		c.Request().Header.Del(HeaderIfNoneMatch)
		c.Request().Header.Set(HeaderIfModifiedSince, "Wed, 21 Oct 2015 07:28:00 GMT")
		c.Response().Header.Set(HeaderLastModified, "Wed, 21 Oct 2015 07:28:00 GMT")
		require.False(t, c.Fresh(), "%s must ignore If-Modified-Since", method)
	}

	// A method that is not registered at all (methodInt == -1) must be
	// handled without panicking and reported as not fresh.
	rctx := &fasthttp.RequestCtx{}
	rctx.Request.Header.SetMethod("PURGE")
	c := apptest.CtxFor(t, app, rctx)
	c.Request().Header.Set(HeaderIfNoneMatch, `"a"`)
	c.Response().Header.Set(HeaderETag, `"a"`)
	require.False(t, c.Fresh())
}

// Test_Ctx_Fresh_MultiFieldLines verifies that repeated If-None-Match and
// Cache-Control field lines are combined into one list per RFC 9110 §5.2.
func Test_Ctx_Fresh_MultiFieldLines(t *testing.T) {
	t.Parallel()
	app := New()

	c := apptest.Ctx(t, app)
	c.Request().Header.Add(HeaderIfNoneMatch, `"a"`)
	c.Request().Header.Add(HeaderIfNoneMatch, `"b"`)
	c.Response().Header.Set(HeaderETag, `"b"`)
	require.True(t, c.Fresh(), "an ETag on a second If-None-Match field line must match")

	c = app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderIfNoneMatch, `"a"`)
	c.Response().Header.Set(HeaderETag, `"a"`)
	c.Request().Header.Add(HeaderCacheControl, "public")
	c.Request().Header.Add(HeaderCacheControl, "no-cache")
	require.False(t, c.Fresh(), "no-cache on a second Cache-Control field line must be honored")
}

// Test_Ctx_Body_With_Compression_MultiFieldLines verifies that repeated
// Content-Encoding field lines are combined into one list per RFC 9110 §5.2.
func Test_Ctx_Body_With_Compression_MultiFieldLines(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		payload string
		lines   []string // the Content-Encoding field lines; the payload is encoded with each non-empty one in turn
	}{
		{name: "repeated lines", lines: []string{StrGzip, StrGzip}, payload: "double"},
		// An empty first field line does not hide the later encodings.
		{name: "empty first line", lines: []string{"", StrGzip}, payload: "john=doe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())
			body := []byte(tc.payload)
			for _, line := range tc.lines {
				c.Request().Header.Add(HeaderContentEncoding, line)
				if line != "" {
					body = ctxTestACompress(t, line, body)
				}
			}

			c.Request().SetBody(body)
			require.Equal(t, []byte(tc.payload), c.Body())
		})
	}
}

func Benchmark_Ctx_Fresh_WithNoCache(b *testing.B) {
	c := apptest.Ctx(b, New())

	c.Request().Header.Set(HeaderIfNoneMatch, "*")
	c.Request().Header.Set(HeaderCacheControl, "no-cache")
	for b.Loop() {
		c.Fresh()
	}
}

func Benchmark_Ctx_Fresh_LastModified(b *testing.B) {
	c := apptest.Ctx(b, New())

	c.Response().Header.Set(HeaderLastModified, "Wed, 21 Oct 2015 07:28:00 GMT")
	c.Request().Header.Set(HeaderIfModifiedSince, "Wed, 21 Oct 2015 07:28:00 GMT")
	for b.Loop() {
		c.Fresh()
	}
}

func Test_Ctx_Binders(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableSplittingOnParsers: true})

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

	// withValues runs a subtest in which actionFn fills the request and binds it
	// into a TestStruct, which must then hold the values of the request.
	withValues := func(t *testing.T, name string, actionFn func(c Ctx, testStruct *TestStruct) error) {
		t.Helper()

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)
			testStruct := new(TestStruct)

			require.NoError(t, actionFn(c, testStruct))
			require.Equal(t, "foo", testStruct.Name)
			require.Equal(t, 111, testStruct.Class)
			require.Equal(t, "bar", testStruct.NameWithDefault)
			require.Equal(t, 222, testStruct.ClassWithDefault)
			require.Equal(t, []string{"foo", "bar", "test"}, testStruct.Names)
		})
	}

	for _, tc := range []struct {
		name        string
		contentType string
		body        string
	}{
		{"Body:xml", MIMEApplicationXML, `<TestStruct><Name>foo</Name><Class>111</Class><Name2>bar</Name2><Class2>222</Class2><Names>foo</Names><Names>bar</Names><Names>test</Names></TestStruct>`},
		{"Body:form", MIMEApplicationForm, `name=foo&class=111&name2=bar&class2=222&names=foo,bar,test`},
		{"BodyParser:json", MIMEApplicationJSON, `{"name":"foo","class":111,"name2":"bar","class2":222,"names":["foo","bar","test"]}`},
	} {
		withValues(t, tc.name, func(c Ctx, testStruct *TestStruct) error {
			c.Request().Header.SetContentType(tc.contentType)
			c.Request().SetBody([]byte(tc.body))
			return c.Bind().Body(testStruct)
		})
	}
	withValues(t, "Body:multiform", func(c Ctx, testStruct *TestStruct) error {
		body := []byte("--b\r\nContent-Disposition: form-data; name=\"name\"\r\n\r\nfoo\r\n--b\r\nContent-Disposition: form-data; name=\"class\"\r\n\r\n111\r\n--b\r\nContent-Disposition: form-data; name=\"name2\"\r\n\r\nbar\r\n--b\r\nContent-Disposition: form-data; name=\"class2\"\r\n\r\n222\r\n--b\r\nContent-Disposition: form-data; name=\"names\"\r\n\r\nfoo\r\n--b\r\nContent-Disposition: form-data; name=\"names\"\r\n\r\nbar\r\n--b\r\nContent-Disposition: form-data; name=\"names\"\r\n\r\ntest\r\n--b--")
		c.Request().SetBody(body)
		c.Request().Header.SetContentType(MIMEMultipartForm + `;boundary="b"`)
		c.Request().Header.SetContentLength(len(body))
		return c.Bind().Body(testStruct)
	})
	withValues(t, "Cookie", func(c Ctx, testStruct *TestStruct) error {
		c.Request().Header.Set("Cookie", "name=foo;name2=bar;class=111;class2=222;names=foo,bar,test")
		return c.Bind().Cookie(testStruct)
	})
	withValues(t, "Query", func(c Ctx, testStruct *TestStruct) error {
		c.Request().URI().SetQueryString("name=foo&name2=bar&class=111&class2=222&names=foo,bar,test")
		return c.Bind().Query(testStruct)
	})

	t.Run("URI", func(t *testing.T) {
		t.Parallel()
		c := apptest.CtxAs[*DefaultCtx](t, app)

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

	withValues(t, "ReqHeader", func(c Ctx, testStruct *TestStruct) error {
		c.Request().Header.Add("name", "foo")
		c.Request().Header.Add("name2", "bar")
		c.Request().Header.Add("class", "111")
		c.Request().Header.Add("class2", "222")
		c.Request().Header.Add("names", "foo,bar,test")
		return c.Bind().Header(testStruct)
	})
}

func Test_Ctx_Get(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().Header.Set(HeaderAcceptCharset, "utf-8, iso-8859-1;q=0.5")
	c.Request().Header.Set(HeaderReferer, "Monster")
	require.Equal(t, "utf-8, iso-8859-1;q=0.5", c.Get(HeaderAcceptCharset))
	require.Equal(t, "Monster", c.Get(HeaderReferer))
	require.Equal(t, "default", c.Get("unknown", "default"))
}

func Test_Ctx_Get_EmptyFirstLine(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	// Get answers with the first field line that holds a value, so an empty
	// leading line does not hide the one after it. net/http's Header.Get and
	// fasthttp's Peek answer with the first line even when it is empty, which
	// is why the docs for Get and the adaptor say the two readers can disagree.
	c.Request().Header.Add("X-Token", "")
	c.Request().Header.Add("X-Token", "secret")
	require.Equal(t, "secret", c.Get("X-Token"))
	require.Empty(t, c.Request().Header.Peek("X-Token"))
	require.Len(t, c.Request().Header.PeekAll("X-Token"), 2)
}

func Test_Ctx_HeaderAccessors_SkipTrailerSection(t *testing.T) {
	t.Parallel()
	for _, normalize := range []bool{true, false} {
		t.Run(fmt.Sprintf("normalize=%v", normalize), func(t *testing.T) {
			t.Parallel()
			app := New(Config{DisableHeaderNormalizing: !normalize})
			app.Post("/", func(c Ctx) error {
				return c.JSON(Map{
					"get":       c.Get("X-User", "anonymous"),
					"generic":   GetReqHeader[string](c, "X-User"),
					"all":       c.GetAll("X-User"),
					"has":       c.HasHeader("X-User"),
					"hasValue":  c.HasHeaderValue("X-User", "admin"),
					"userAgent": c.UserAgent(),
					"referer":   c.Referer(),
					"origin":    c.Origin(),
					"xhr":       c.XHR(),
					"kept":      c.Get("X-Kept"),
				})
			})
			ln := startRawServer(t, app)

			trailers := "X-User: admin\r\nUser-Agent: late\r\nReferer: http://evil.example/\r\n" +
				"Origin: http://evil.example\r\nX-Requested-With: XMLHttpRequest\r\nX-Kept: late\r\n"
			raw := "POST / HTTP/1.1\r\nHost: example.com\r\nX-Kept: early\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n" +
				"5\r\nhello\r\n0\r\n" + trailers + "\r\n"
			resp := rawExpectStatus(t, ln, raw, StatusOK)
			require.JSONEq(t, `{"get":"anonymous","generic":"","all":null,"has":false,"hasValue":false,`+
				`"userAgent":"","referer":"","origin":"","xhr":false,"kept":"early"}`, resp.body)
		})
	}
}

func Test_Ctx_HeaderWrittenUnderTrailerName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		newApp func(...Config) *App
		cfg    Config
		name   string
	}{
		{name: "default", newApp: New},
		{name: "raw names", newApp: New, cfg: Config{DisableHeaderNormalizing: true}},
		{name: "custom ctx", newApp: ctxTestACustomApp},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := tc.newApp(tc.cfg)
			app.Use(func(c Ctx) error {
				h := &c.Request().Header
				h.Set("X-Flag", "1")
				h.Set("X-Tenant", "safe")
				h.Del("X-Role")
				h.Set("X-Role", "user")
				return c.Next()
			})
			app.All("/", func(c Ctx) error {
				return c.SendString(fmt.Sprintf("flag=%s has=%v tenant=%q role=%q trailers=%d", c.Get("X-Flag"), c.HasHeader("X-Flag"),
					c.GetAll("X-Tenant"), c.GetAll("X-Role"), len(c.Request().Header.PeekTrailerKeys())))
			})
			ln := startRawServer(t, app)

			const (
				fields   = "Host: example.com\r\nConnection: close\r\nX-Tenant: evil\r\nX-Role: admin\r\n"
				post     = "POST / HTTP/1.1\r\n" + fields
				chunked  = "Transfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n"
				announce = "Trailer: X-Flag, X-Tenant, X-Role\r\n"
				want     = `flag=1 has=true tenant=["safe"] role=["user"] trailers=0`
			)
			for name, raw := range map[string]string{
				"names in the trailer section":    post + chunked + "X-Flag: 0\r\nX-Tenant: x\r\nX-Role: x\r\n\r\n",
				"names announced, chunked body":   post + announce + chunked + "\r\n",
				"names announced, content length": post + announce + "Content-Length: 5\r\n\r\nhello",
				"names announced, no body":        "GET / HTTP/1.1\r\n" + fields + announce + "\r\n",
			} {
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					require.Equal(t, want, rawExpectStatus(t, ln, raw, StatusOK).body, raw)
				})
			}

			t.Run("too many names announced", func(t *testing.T) {
				t.Parallel()
				// Each name costs fasthttp a pass over the header to drop.
				raw := post + "Trailer: X-1, X-2, X-3, X-4, X-5, X-6, X-7, X-8, X-9, " +
					"X-10, X-11, X-12, X-13, X-14, X-15, X-16, X-17\r\n" + chunked + "\r\n"
				rawExpectStatus(t, ln, raw, StatusRequestHeaderFieldsTooLarge)
			})
		})
	}
}

func Test_Ctx_HeaderWrittenUnderTrailerName_StreamedBody(t *testing.T) {
	t.Parallel()
	for name, newApp := range map[string]func(...Config) *App{"default": New, "custom ctx": ctxTestACustomApp} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := newApp(Config{
				StreamRequestBody:  true,
				TrustProxy:         true,
				TrustProxyConfig:   TrustProxyConfig{Loopback: true},
				ProxyHeader:        HeaderXForwardedFor,
				EnableIPValidation: true,
			})
			respond := func(c Ctx, body []byte) error {
				h := &c.Request().Header
				h.Set("X-User", "alice")
				h.Del("X-Role")
				h.Set("X-Role", "user")
				return c.SendString(fmt.Sprintf("body=%s user=%q role=%q trailers=%d", body, c.GetAll("X-User"),
					c.GetAll("X-Role"), len(h.PeekTrailerKeys())))
			}
			app.Post("/body", func(c Ctx) error { return respond(c, c.Body()) })
			app.Post("/stream", func(c Ctx) error {
				body, err := io.ReadAll(c.BodyStream())
				if err != nil {
					return err //nolint:wrapcheck // the status rides on the stream's own error
				}
				return respond(c, body)
			})
			app.Post("/fields", func(c Ctx) error {
				role := c.Get("X-Role")
				read := string(c.Body()) == "hello"
				return c.SendString(fmt.Sprintf("read=%v role=%s blocked=%q client=%q ip=%s scheme=%s host=%q", read, role,
					c.Get("X-Blocked"), c.GetAll("X-Client"), c.IP(), c.Scheme(), c.Host()))
			})
			ln := startRawServer(t, app)

			const chunked = "Transfer-Encoding: chunked\r\nConnection: close\r\n\r\n5\r\nhello\r\n0\r\n"
			many := rawFieldLines("X-", "v", fieldname.MaxTrailers-2)
			heads := []string{"", "X-User: admin\r\nX-Role: admin\r\n", "X-User: admin\r\nTrailer: X-User, X-Role\r\n"}
			post := func(path, head, trailers string) string {
				return "POST " + path + " HTTP/1.1\r\nHost: example.com\r\n" + head + chunked + trailers +
					"X-User: x\r\nX-Role: x\r\n\r\n"
			}

			for _, path := range []string{"/body", "/stream"} {
				for _, head := range heads {
					for _, trailers := range []string{"", many} {
						raw := post(path, head, trailers)
						resp := rawExpectStatus(t, ln, raw, StatusOK)
						require.Equal(t, `body=hello user=["alice"] role=["user"] trailers=0`, resp.body, raw)
					}
				}
			}

			// One name past the cap: the read fails, and the section is dropped all the same.
			tooMany := many + "X-14: v\r\n"
			for _, head := range heads {
				rawExpectStatus(t, ln, post("/stream", head, tooMany), StatusRequestHeaderFieldsTooLarge)

				raw := post("/body", head, tooMany)
				responses, _ := rawExchange(t, ln, raw, 1)
				require.Equal(t, `body=Request Header Fields Too Large user=["alice"] role=["user"] trailers=0`, responses[0].body, raw)
			}

			// The header section stays as received either way, the proxy's lines included, and so does a value read before.
			for _, trailers := range []string{"", tooMany} {
				raw := "POST /fields HTTP/1.1\r\nHost: example.com\r\nX-Client: c\r\nX-Role: user\r\nX-Evil: root\r\n" +
					"X-Blocked: 1\r\nX-Forwarded-For: 203.0.113.7\r\nX-Forwarded-Proto: https\r\n" +
					chunked + trailers + "X-Role: x\r\nX-Evil: x\r\nX-Blocked: x\r\n\r\n"
				responses, _ := rawExchange(t, loopbackDialer{ln}, raw, 1)
				require.Equal(t, fmt.Sprintf(`read=%v role=user blocked="1" client=["c"] ip=203.0.113.7 scheme=https host="example.com"`,
					trailers == ""), responses[0].body)
			}
		})
	}
}

// Test_Ctx_BodyOfClosableStream checks that a stream handed to App.Handler is read, not closed: only the
// server's own stream reads a trailer section, and wrapping another closed it before the handler read it.
func Test_Ctx_BodyOfClosableStream(t *testing.T) {
	t.Parallel()
	app := New()
	app.Post("/", func(c Ctx) error { return c.Send(c.Body()) })

	for name, set := range map[string]func(*fasthttp.Request){
		"stream writer": func(req *fasthttp.Request) {
			req.SetBodyStreamWriter(func(w *bufio.Writer) {
				_, err := w.WriteString("hello")
				require.NoError(t, err)
			})
		},
		"pipe": func(req *fasthttp.Request) {
			r, w := io.Pipe()
			go func() {
				_, err := w.Write([]byte("hello"))
				w.CloseWithError(err) //nolint:errcheck // CloseWithError always returns nil
			}()
			req.SetBodyStream(r, -1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(MethodPost)
			fctx.Request.SetRequestURI("/")
			set(&fctx.Request)
			app.Handler()(fctx)

			require.Equal(t, StatusOK, fctx.Response.StatusCode())
			require.Equal(t, "hello", string(fctx.Response.Body()))
		})
	}
}

func Test_Ctx_GetReqHeader(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().Header.Set("foo", "bar")
	c.Request().Header.Set("id", "123")
	require.Equal(t, 123, GetReqHeader[int](c, "id"))
	require.Equal(t, "bar", GetReqHeader[string](c, "foo"))
}

func Test_Ctx_Host(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().SetRequestURI("http://google.com/test")
	require.Equal(t, "google.com", c.Host())
}

// Host and Hostname answer with the X-Forwarded-Host header only when the
// request comes from a trusted proxy.
func Test_Ctx_Host_ForwardedHost(t *testing.T) {
	t.Parallel()
	// The requests come from 0.0.0.0 and are for http://google.com/test.
	trusted := []string{"0.0.0.0", "0.8.0.1"}
	tests := []struct {
		name          string
		forwardedHost string
		want          string
		proxies       []string
	}{
		// Don't trust any proxy
		{name: "no trusted proxy", proxies: []string{}, forwardedHost: "google1.com", want: "google.com"},
		// Trust to specific proxy list
		{name: "other proxies trusted", proxies: []string{"0.8.0.0", "0.8.0.1"}, forwardedHost: "google1.com", want: "google.com"},
		{name: "proxy trusted", proxies: trusted, forwardedHost: "google1.com", want: "google1.com"},
		{name: "proxy trusted, several hosts", proxies: trusted, forwardedHost: "google1.com, google2.com", want: "google1.com"},
		// Trust to a proxy range
		{name: "proxy in trusted range", proxies: []string{"0.0.0.0/30"}, forwardedHost: "google1.com", want: "google1.com"},
		{name: "proxy outside trusted range", proxies: []string{"1.0.0.0/30"}, forwardedHost: "google1.com", want: "google.com"},

		// Whitespace around the forwarded host is trimmed
		{name: "leading whitespace with comma", proxies: trusted, forwardedHost: " example.com, proxy1", want: "example.com"},
		{name: "trailing whitespace with comma", proxies: trusted, forwardedHost: "example.com , proxy1", want: "example.com"},
		{name: "leading and trailing whitespace with comma", proxies: trusted, forwardedHost: "  example.com  , proxy1", want: "example.com"},
		{name: "no whitespace with comma", proxies: trusted, forwardedHost: "example.com, proxy1", want: "example.com"},
		{name: "single value with whitespace", proxies: trusted, forwardedHost: "  example.com  ", want: "example.com"},
		{name: "leading comma", proxies: trusted, forwardedHost: ",example.com", want: ""},
	}

	// No host carries a port, so every case answers alike through c.Host and c.Hostname.
	for _, accessor := range []struct {
		get  func(Ctx) string
		name string
	}{
		{name: "Host", get: Ctx.Host},
		{name: "Hostname", get: Ctx.Hostname},
	} {
		t.Run(accessor.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					c := apptest.Ctx(t, New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: tc.proxies}}))
					c.Request().SetRequestURI("http://google.com/test")
					c.Request().Header.Set(HeaderXForwardedHost, tc.forwardedHost)
					require.Equal(t, tc.want, accessor.get(c))
				})
			}
		})
	}
}

func Benchmark_Ctx_Host(b *testing.B) {
	c := apptest.Ctx(b, New())
	c.Request().SetRequestURI("http://google.com/test")
	var host string
	b.ReportAllocs()
	for b.Loop() {
		host = c.Host()
	}
	require.Equal(b, "google.com", host)
}

func Test_Ctx_IsProxyTrusted(t *testing.T) {
	t.Parallel()
	// The requests come from 0.0.0.0.
	trust := func(config TrustProxyConfig) Config { return Config{TrustProxy: true, TrustProxyConfig: config} }

	for _, tc := range []struct {
		config Config
		name   string
		want   bool
	}{
		{name: "default config", config: Config{}, want: false},
		{name: "TrustProxy without proxies", config: trust(TrustProxyConfig{}), want: false},
		{name: "empty proxy list", config: trust(TrustProxyConfig{Proxies: []string{}}), want: false},
		{name: "other proxy IP", config: trust(TrustProxyConfig{Proxies: []string{"127.0.0.1"}}), want: false},
		{name: "other proxy range", config: trust(TrustProxyConfig{Proxies: []string{"127.0.0.1/8"}}), want: false},
		{name: "proxy IP", config: trust(TrustProxyConfig{Proxies: []string{"0.0.0.0"}}), want: true},
		{name: "proxy range", config: trust(TrustProxyConfig{Proxies: []string{"0.0.0.1/31"}}), want: true},
		{name: "malformed proxy range", config: trust(TrustProxyConfig{Proxies: []string{"0.0.0.1/31junk"}}), want: false},
		{name: "private networks", config: trust(TrustProxyConfig{Private: true}), want: false},
		{name: "loopback", config: trust(TrustProxyConfig{Loopback: true}), want: false},
		{name: "link-local", config: trust(TrustProxyConfig{LinkLocal: true}), want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New(tc.config))
			require.Equal(t, tc.want, c.IsProxyTrusted())
		})
	}
}

func Test_Ctx_Hostname(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().SetRequestURI("http://google.com/test")
	require.Equal(t, "google.com", c.Hostname())

	c.Request().SetRequestURI("http://google.com:8080/test")
	require.Equal(t, "google.com", c.Hostname())
}

func Benchmark_Ctx_Hostname(b *testing.B) {
	c := apptest.Ctx(b, New())
	c.Request().SetRequestURI("http://google.com:8080/test")
	var hostname string
	b.ReportAllocs()
	for b.Loop() {
		hostname = c.Hostname()
	}
	require.Equal(b, "google.com", hostname)
}

func Test_Ctx_Port_RemoteAddrVariants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		remote net.Addr
		want   string
	}{
		{name: "tcp", remote: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}, want: "8080"},
		{name: "unix", remote: &net.UnixAddr{Name: "/tmp/fiber.sock", Net: "unix"}, want: ""},
		{name: "default-remote", remote: nil, want: "0"},
		{name: "string-host-port", remote: testNetAddr{network: "tcp", address: "192.0.2.1:443"}, want: "443"},
		{name: "string-missing-port", remote: testNetAddr{network: "tcp", address: "192.0.2.1"}, want: ""},
		{name: "string-ipv6-port", remote: testNetAddr{network: "tcp", address: "[2001:db8::1]:8443"}, want: "8443"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.CtxAs[*DefaultCtx](t, New())
			c.fasthttp.SetRemoteAddr(test.remote)
			require.Equal(t, test.want, c.Port())
		})
	}
}

func Test_Ctx_PortInHandler(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/port", func(c Ctx) error { return c.SendString(c.Port()) })

	apptest.ExpectBody(t, app, "/port", StatusOK, "0")
}

func Test_Ctx_IP(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	// default behavior will return the remote IP from the stack
	require.Equal(t, "0.0.0.0", c.IP())

	// X-Forwarded-For is set, but it is ignored because proxyHeader is not set
	c.Request().Header.Set(HeaderXForwardedFor, "0.0.0.1")
	require.Equal(t, "0.0.0.0", c.IP())
}

// ctxFromIP returns a context of app for a request whose TCP peer is ip.
func ctxFromIP(tb testing.TB, app *App, ip string) Ctx {
	tb.Helper()
	fctx := &fasthttp.RequestCtx{}
	fctx.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP(ip)})
	return apptest.CtxFor(tb, app, fctx)
}

// forwardedForConfig returns the Config of an app that reads the client from X-Forwarded-For behind the proxies
// trust names, and validates the IPs in the chain.
func forwardedForConfig(trust TrustProxyConfig) Config {
	return Config{ProxyHeader: HeaderXForwardedFor, TrustProxy: true, EnableIPValidation: true, TrustProxyConfig: trust}
}

// forwardedForProxies is forwardedForConfig for the proxies at the given IPs or ranges.
func forwardedForProxies(proxies ...string) Config {
	return forwardedForConfig(TrustProxyConfig{Proxies: proxies})
}

func Test_Ctx_IP_ProxyHeader(t *testing.T) {
	t.Parallel()
	// Without IP validation the header comes back as it is, whatever it holds, even
	// the full list or an invalid IP. With it, the rightmost non-trusted IP of the
	// list is returned, and a header that is empty or holds no IP is ignored: the IP
	// of the caller is returned instead.
	tests := []struct {
		name      string
		value     string
		plain     string // IP without IP validation
		validated string // IP with IP validation
	}{
		{name: "valid IP", value: "0.0.0.1", plain: "0.0.0.1", validated: "0.0.0.1"},
		{name: "list of IPs", value: "0.0.0.1, 0.0.0.2", plain: "0.0.0.1, 0.0.0.2", validated: "0.0.0.2"},
		{name: "list with an invalid IP", value: "invalid, 0.0.0.2, 0.0.0.3", plain: "invalid, 0.0.0.2, 0.0.0.3", validated: "0.0.0.3"},
		{name: "empty", value: "", plain: "", validated: "0.0.0.0"},
		{name: "not an IP", value: "not-valid-ip", plain: "not-valid-ip", validated: "0.0.0.0"},
	}

	// make sure that the same behavior exists for different proxy header names
	for _, proxyHeaderName := range []string{"Real-Ip", HeaderXForwardedFor} {
		for _, enableIPValidation := range []bool{false, true} {
			app := New(Config{
				EnableIPValidation: enableIPValidation,
				ProxyHeader:        proxyHeaderName,
				TrustProxy:         true,
				TrustProxyConfig:   TrustProxyConfig{Proxies: []string{"0.0.0.0"}},
			})

			for _, tc := range tests {
				want := tc.plain
				if enableIPValidation {
					want = tc.validated
				}

				t.Run(fmt.Sprintf("%s, EnableIPValidation=%t, %s", proxyHeaderName, enableIPValidation, tc.name), func(t *testing.T) {
					t.Parallel()
					c := ctxFromIP(t, app, "0.0.0.0")
					c.Request().Header.Set(proxyHeaderName, tc.value)
					require.Equal(t, want, c.IP())
				})
			}
		}
	}
}

func Test_Ctx_IP_TrustedProxy(t *testing.T) {
	t.Parallel()
	// The requests come from 0.0.0.0.
	for _, tc := range []struct {
		name  string
		proxy string
		want  string
	}{
		{name: "untrusted proxy", proxy: "0.8.0.1", want: "0.0.0.0"},
		{name: "trusted proxy", proxy: "0.0.0.0", want: "0.0.0.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New(Config{
				TrustProxy:       true,
				TrustProxyConfig: TrustProxyConfig{Proxies: []string{tc.proxy}},
				ProxyHeader:      HeaderXForwardedFor,
			}))
			c.Request().Header.Set(HeaderXForwardedFor, "0.0.0.1")
			require.Equal(t, tc.want, c.IP())
		})
	}
}

func Test_Ctx_IP_StripTrustedProxies(t *testing.T) {
	t.Parallel()
	// Every case validates the X-Forwarded-For chain against the proxies it trusts.
	tests := []struct {
		config   Config
		name     string
		remoteIP string
		header   string // set as the X-Forwarded-For header, when not empty
		expected string
		lines    []string // added to it as field lines of their own
	}{
		{
			name: "strip multiple trusted proxies from right", config: forwardedForProxies("10.0.0.1", "10.0.0.2"),
			remoteIP: "10.0.0.2", header: "203.0.113.50, 10.0.0.1, 10.0.0.2", expected: "203.0.113.50",
		},
		{
			name: "CIDR range stripping", config: forwardedForProxies("10.0.0.0/24"),
			remoteIP: "10.0.0.5", header: "203.0.113.50, 10.0.0.1, 10.0.0.2", expected: "203.0.113.50",
		},
		{
			name: "exact IP match stripping", config: forwardedForProxies("10.0.0.1"),
			remoteIP: "10.0.0.1", header: "203.0.113.50, 10.0.0.1", expected: "203.0.113.50",
		},
		{
			name: "loopback stripping", config: forwardedForConfig(TrustProxyConfig{Loopback: true}),
			remoteIP: "127.0.0.1", header: "203.0.113.50, 127.0.0.1", expected: "203.0.113.50",
		},
		{
			name: "private network stripping", config: forwardedForConfig(TrustProxyConfig{Private: true}),
			remoteIP: "192.168.1.1", header: "203.0.113.50, 192.168.1.1, 10.0.0.1", expected: "203.0.113.50",
		},
		{
			name: "link-local stripping", config: forwardedForConfig(TrustProxyConfig{LinkLocal: true}),
			remoteIP: "169.254.0.1", header: "203.0.113.50, 169.254.0.1", expected: "203.0.113.50",
		},
		{
			name: "all IPs trusted returns leftmost", config: forwardedForProxies("10.0.0.1", "10.0.0.2", "10.0.0.3"),
			remoteIP: "10.0.0.3", header: "10.0.0.1, 10.0.0.2, 10.0.0.3", expected: "10.0.0.1",
		},
		{
			name: "IPv6 chain stripping", config: forwardedForProxies("::1", "fd00::1"),
			remoteIP: "fd00::1", header: "2001:db8::1, fd00::1, ::1", expected: "2001:db8::1",
		},
		{
			name: "single non-trusted IP returns as-is", config: forwardedForProxies("10.0.0.1"),
			remoteIP: "10.0.0.1", header: "203.0.113.50", expected: "203.0.113.50",
		},
		{
			name: "single trusted IP returns it as fallback", config: forwardedForProxies("10.0.0.1"),
			remoteIP: "10.0.0.1", header: "10.0.0.1", expected: "10.0.0.1",
		},
		{
			name: "attacker injects invalid leftmost token", config: forwardedForProxies("10.0.0.1"),
			remoteIP: "10.0.0.1", header: "evil.attacker, 203.0.113.50, 10.0.0.1", expected: "203.0.113.50",
		},
		{
			name: "attacker injects spoofed leftmost IP", config: forwardedForProxies("10.0.0.1"),
			remoteIP: "10.0.0.1", header: "1.2.3.4, 203.0.113.50, 10.0.0.1", expected: "203.0.113.50",
		},
		{
			name: "whitespace around chain entries", config: forwardedForProxies("10.0.0.1"),
			remoteIP: "10.0.0.1", header: "203.0.113.50 , 10.0.0.1", expected: "203.0.113.50",
		},
		{
			// RFC 9110 Section 5.6.3: OWS is SP or HTAB
			name: "tab whitespace around chain entries", config: forwardedForProxies("10.0.0.1"),
			remoteIP: "10.0.0.1", header: "\t203.0.113.50\t,\t10.0.0.1\t", expected: "203.0.113.50",
		},
		{
			name: "tab inside an address is not whitespace", config: forwardedForProxies("10.0.0.1"),
			remoteIP: "10.0.0.1", header: "203.0.\t113.50, 10.0.0.1", expected: "10.0.0.1",
		},
		{
			name: "trailing empty chain element", config: forwardedForProxies("10.0.0.1"),
			remoteIP: "10.0.0.1", header: "203.0.113.50, 10.0.0.1,", expected: "203.0.113.50",
		},
		{
			name: "repeated field lines", config: forwardedForProxies("127.0.0.1"),
			remoteIP: "127.0.0.1", lines: []string{"9.9.9.9", "198.51.100.77, 127.0.0.1"}, expected: "198.51.100.77",
		},
		{
			name: "empty chain element between entries", config: forwardedForProxies("10.0.0.1"),
			remoteIP: "10.0.0.1", header: "203.0.113.50, , 10.0.0.1", expected: "203.0.113.50",
		},
		{
			name: "entry with a port ends the walk", config: forwardedForProxies("10.0.0.1"),
			remoteIP: "10.0.0.1", header: "1.2.3.4, 203.0.113.50:51234", expected: "10.0.0.1",
		},
		{
			name:     "entry with a port behind a trusted proxy ends the walk",
			config:   forwardedForProxies("10.0.0.1", "10.0.0.2"),
			remoteIP: "10.0.0.1", header: "1.2.3.4, 203.0.113.50:51234, 10.0.0.2", expected: "10.0.0.2",
		},
		{
			name: "bracketed IPv6 entry with a port ends the walk", config: forwardedForProxies("10.0.0.1"),
			remoteIP: "10.0.0.1", header: "1.2.3.4, [2001:db8::1]:51234", expected: "10.0.0.1",
		},
		{
			name: "zoned IPv6 entry ends the walk", config: forwardedForProxies("10.0.0.1"),
			remoteIP: "10.0.0.1", header: "1.2.3.4, fe80::1%eth0", expected: "10.0.0.1",
		},
		{
			name: "obfuscated entry ends the walk", config: forwardedForProxies("10.0.0.1"),
			remoteIP: "10.0.0.1", header: "1.2.3.4, unknown", expected: "10.0.0.1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := ctxFromIP(t, New(tc.config), tc.remoteIP)
			if tc.header != "" {
				c.Request().Header.Set(HeaderXForwardedFor, tc.header)
			}
			for _, line := range tc.lines {
				c.Request().Header.Add(HeaderXForwardedFor, line)
			}
			require.Equal(t, tc.expected, c.IP())
		})
	}
}

func Test_Ctx_IP_ProxyHeader_NoTrustedProxies(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New(Config{ProxyHeader: HeaderXForwardedFor, EnableIPValidation: true}))

	c.Request().Header.Set(HeaderXForwardedFor, "invalid, 203.0.113.50, 10.0.0.1")
	require.Equal(t, "203.0.113.50", c.extractIPFromHeader(HeaderXForwardedFor))

	// The left-to-right walk trims HTAB as OWS too (RFC 9110 Section 5.6.3)
	c.Request().Header.Set(HeaderXForwardedFor, "invalid,\t203.0.113.50\t,\t10.0.0.1")
	require.Equal(t, "203.0.113.50", c.extractIPFromHeader(HeaderXForwardedFor))
}

// wireRequestCtx returns a fasthttp context for the request raw, read off the wire as a server does, so that its raw
// header block is kept, and sent from remote (nil: the zero address).
func wireRequestCtx(tb testing.TB, raw string, remote net.Addr) *fasthttp.RequestCtx {
	tb.Helper()

	req := &fasthttp.Request{}
	require.NoError(tb, req.Read(bufio.NewReader(strings.NewReader(raw))))
	fctx := &fasthttp.RequestCtx{}
	fctx.Init(req, remote, nil)
	return fctx
}

// serveRawFromLoopback runs handler on the request raw, read off the wire, from 127.0.0.1.
func serveRawFromLoopback(t *testing.T, handler fasthttp.RequestHandler, raw string) string {
	t.Helper()

	fctx := wireRequestCtx(t, raw, &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	handler(fctx)
	return string(fctx.Response.Body())
}

// fasthttp fills the slot of a removed header line with the last line, and removes
// Transfer-Encoding itself after an empty chunked body.
func Test_Ctx_IP_ProxyHeader_RepeatedFieldLines_ReceivedOrder(t *testing.T) {
	t.Parallel()
	app := New(forwardedForProxies("127.0.0.1"))
	result := func(c Ctx) error {
		// Twice: the first read must not change what the second sees.
		return c.SendString(c.IP() + "|" + c.IP() + "|" + strings.Join(c.IPs(), ","))
	}
	app.Post("/", result)
	app.Post("/del", func(c Ctx) error {
		c.Request().Header.Del("X-Request-Id")
		return result(c)
	})
	app.Post("/add", func(c Ctx) error {
		c.Request().Header.Add(HeaderXForwardedFor, "203.0.113.9")
		return result(c)
	})
	handler := app.Handler()

	testCases := []struct {
		name     string
		raw      string
		expected string
	}{
		{
			name: "empty chunked body",
			raw: "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\n" +
				"X-Forwarded-For: 6.6.6.6\r\nX-Forwarded-For: 198.51.100.77\r\n\r\n0\r\n\r\n",
			expected: "198.51.100.77|198.51.100.77|6.6.6.6,198.51.100.77",
		},
		{
			name: "empty chunked body with a folded line",
			raw: "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\n" +
				"X-Forwarded-For: 6.6.6.6,\r\n 8.8.8.8\r\nX-Forwarded-For: 198.51.100.77\r\n\r\n0\r\n\r\n",
			expected: "198.51.100.77|198.51.100.77|6.6.6.6,8.8.8.8,198.51.100.77",
		},
		{
			name: "line removed by a handler",
			raw: "POST /del HTTP/1.1\r\nHost: example.com\r\nX-Request-Id: 1\r\n" +
				"X-Forwarded-For: 6.6.6.6\r\nX-Forwarded-For: 198.51.100.77\r\nContent-Length: 0\r\n\r\n",
			expected: "198.51.100.77|198.51.100.77|6.6.6.6,198.51.100.77",
		},
		{
			name: "line added by a handler",
			raw: "POST /add HTTP/1.1\r\nHost: example.com\r\n" +
				"X-Forwarded-For: 6.6.6.6\r\nX-Forwarded-For: 198.51.100.77\r\nContent-Length: 0\r\n\r\n",
			expected: "203.0.113.9|203.0.113.9|6.6.6.6,198.51.100.77,203.0.113.9",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, serveRawFromLoopback(t, handler, tc.raw))
		})
	}
}

// Repeated field lines must also be combined when the request is parsed off
// the wire, not just when the headers are set programmatically. fasthttp trims
// the OWS at either end of a field value but keeps the OWS around the commas
// inside it, so a tab there reaches the chain parsers.
func Test_Ctx_IP_ProxyHeader_Wire(t *testing.T) {
	t.Parallel()
	app := New(forwardedForProxies("0.0.0.0"))
	app.Get("/", func(c Ctx) error { return c.SendString(c.IP() + "|" + strings.Join(c.IPs(), ",")) })

	for _, tc := range []struct {
		name   string
		want   string   // c.IP() and c.IPs() joined by "|" and ","
		fields []string // the X-Forwarded-For field lines of the request
	}{
		{name: "repeated field lines", fields: []string{"9.9.9.9", "198.51.100.77, 0.0.0.0"}, want: "198.51.100.77|9.9.9.9,198.51.100.77,0.0.0.0"},
		{name: "tab OWS around the comma", fields: []string{"203.0.113.50\t,\t0.0.0.0"}, want: "203.0.113.50|203.0.113.50,0.0.0.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := apptest.Req(MethodGet, "/")
			for _, field := range tc.fields {
				req.Header.Add(HeaderXForwardedFor, field)
			}
			apptest.ExpectBody(t, app, req, StatusOK, tc.want)
		})
	}
}

func Test_Ctx_IP_ProxyHeader_InvalidIPs(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableIPValidation: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{"10.0.0.1"}}})
	fastCtx := &fasthttp.RequestCtx{}
	fastCtx.SetRemoteAddr(testNetAddr{network: "tcp", address: "invalid"})
	c := apptest.CtxFor(t, app, fastCtx)

	c.Request().Header.Set(HeaderXForwardedFor, "invalid")
	require.Equal(t, "0.0.0.0", c.extractIPFromHeader(HeaderXForwardedFor))
	require.False(t, c.isTrustedProxyIP("invalid"))
}

func Test_Ctx_IP_TrustedProxyIPv6Range(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New(Config{TrustProxyConfig: TrustProxyConfig{Proxies: []string{"2001:db8::/32"}}}))

	require.True(t, c.isTrustedProxyIP("2001:db8::1"))
	require.False(t, c.isTrustedProxyIP("2001:db9::1"))
}

func Test_Ctx_ProxyTrust_UnixRemoteAddr(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not supported on windows in this test")
	}

	for _, tc := range []struct {
		name       string
		trusted    string
		ip         string
		unixSocket bool
	}{
		{name: "unix_socket_enabled", unixSocket: true, trusted: "true", ip: "1.1.1.1"},
		{name: "unix_socket_disabled", unixSocket: false, trusted: "false", ip: "0.0.0.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parts := strings.SplitN(runCtxProxyTrustUnixRemoteAddrCase(t, tc.unixSocket), "|", 2)
			require.Len(t, parts, 2)
			require.Equal(t, tc.trusted, parts[0])
			require.Equal(t, tc.ip, parts[1])
		})
	}
}

func runCtxProxyTrustUnixRemoteAddrCase(t *testing.T, unixSocket bool) string {
	t.Helper()

	app := New(Config{
		TrustProxy:       true,
		TrustProxyConfig: TrustProxyConfig{UnixSocket: unixSocket},
		ProxyHeader:      HeaderXForwardedFor,
	})
	app.Get("/ip", func(c Ctx) error { return c.SendString(fmt.Sprintf("%t|%s", c.IsProxyTrusted(), c.IP())) })

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
			Dial: func(addr string) (net.Conn, error) { return net.Dial(NetworkUnix, addr) },
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

func Test_Ctx_IPs(t *testing.T) {
	t.Parallel()
	local := []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}
	validLocal := []string{"127.0.0.1", "127.0.0.2"}
	reordered := []string{"127.0.0.3", "127.0.0.1", "127.0.0.2"}
	withIPv6 := []string{"127.0.0.1", "127.0.0.2", "2001:db8::1"}

	tests := []struct {
		name          string
		header        string
		want          []string // every element of the header, trimmed
		wantValidated []string // with Config.EnableIPValidation: only the elements that are valid IPs
		validatedOnly bool
	}{
		{name: "normal happy path", header: "127.0.0.1, 127.0.0.2, 127.0.0.3", want: local, wantValidated: local},
		{name: "inconsistent space formatting", header: "127.0.0.1,127.0.0.2  ,127.0.0.3", want: local, wantValidated: local},
		// tabs are optional whitespace too (RFC 9110 Section 5.6.3)
		{name: "tabs around IPv4 addresses", header: "\t127.0.0.1\t,\t127.0.0.2 \t, \t127.0.0.3", want: local, wantValidated: local},
		{name: "tabs around an IPv6 address", header: "\t127.0.0.1\t,\t127.0.0.2 \t, \t2001:db8::1\t", want: withIPv6, wantValidated: withIPv6},
		// but a tab inside an address is not, nor is whitespace other than SP and HTAB
		{name: "tab inside an address and a vertical tab", header: "127.0.\t0.1, \v127.0.0.2, 127.0.0.3", wantValidated: []string{"127.0.0.3"}, validatedOnly: true},
		// invalid IPs are allowed to be returned, unless the validation drops them
		{name: "invalid IP first", header: "invalid, 127.0.0.1, 127.0.0.2", want: []string{"invalid", "127.0.0.1", "127.0.0.2"}, wantValidated: validLocal},
		{name: "invalid IP in the middle", header: "127.0.0.1, invalid, 127.0.0.2", want: []string{"127.0.0.1", "invalid", "127.0.0.2"}, wantValidated: validLocal},
		{name: "ordering of IPs in the header is maintained", header: "127.0.0.3, 127.0.0.1, 127.0.0.2", want: reordered, wantValidated: reordered},
		{
			name:          "IPv6",
			header:        "9396:9549:b4f7:8ed0:4791:1330:8c06:e62d, invalid, 2345:0425:2CA1::0567:5673:23b5",
			want:          []string{"9396:9549:b4f7:8ed0:4791:1330:8c06:e62d", "invalid", "2345:0425:2CA1::0567:5673:23b5"},
			wantValidated: []string{"9396:9549:b4f7:8ed0:4791:1330:8c06:e62d", "2345:0425:2CA1::0567:5673:23b5"},
		},
		{
			name:          "other IPv6",
			header:        "f037:825e:eadb:1b7b:1667:6f0a:5356:f604, invalid, 9396:9549:b4f7:8ed0:4791:1330:8c06:e62d",
			want:          []string{"f037:825e:eadb:1b7b:1667:6f0a:5356:f604", "invalid", "9396:9549:b4f7:8ed0:4791:1330:8c06:e62d"},
			wantValidated: []string{"f037:825e:eadb:1b7b:1667:6f0a:5356:f604", "9396:9549:b4f7:8ed0:4791:1330:8c06:e62d"},
		},
		{name: "empty header", header: ""},
	}

	for _, mode := range []struct {
		app       *App
		name      string
		validated bool
	}{
		{name: "default", app: New()},
		{name: "with IP validation", app: New(Config{EnableIPValidation: true}), validated: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range tests {
				if tc.validatedOnly && !mode.validated {
					continue
				}
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					want := tc.want
					if mode.validated {
						want = tc.wantValidated
					}

					c := apptest.Ctx(t, mode.app)
					c.Request().Header.Set(HeaderXForwardedFor, tc.header)
					if len(want) == 0 {
						require.Empty(t, c.IPs())
						return
					}
					require.Equal(t, want, c.IPs())
				})
			}

			t.Run("missing header", func(t *testing.T) {
				t.Parallel()
				require.Empty(t, apptest.Ctx(t, mode.app).IPs())
			})
		})
	}
}

func Test_Ctx_ForwardedIPScanning(t *testing.T) {
	t.Parallel()
	const remoteIP = "10.0.0.2"
	for _, tc := range []struct {
		name             string
		header           string
		firstValidIP     string
		firstUntrustedIP string
		ips              []string
		validatedIPs     []string
	}{
		{
			name: "empty header", header: "", ips: []string{}, validatedIPs: []string{},
			firstValidIP: remoteIP, firstUntrustedIP: remoteIP,
		},
		{
			name: "only commas", header: ",,,", ips: []string{}, validatedIPs: []string{},
			firstValidIP: remoteIP, firstUntrustedIP: remoteIP,
		},
		{
			name: "only optional whitespace", header: " \t,\t , ", ips: []string{}, validatedIPs: []string{},
			firstValidIP: remoteIP, firstUntrustedIP: remoteIP,
		},
		{
			name: "unvalidated tokens with two commas", header: "a,,b",
			ips: []string{"a", "b"}, validatedIPs: []string{},
			firstValidIP: remoteIP, firstUntrustedIP: remoteIP,
		},
		{
			name: "unvalidated tokens with three commas", header: "a,,,b",
			ips: []string{"a", "b"}, validatedIPs: []string{},
			firstValidIP: remoteIP, firstUntrustedIP: remoteIP,
		},
		{
			name: "leading and trailing commas", header: ",203.0.113.50,",
			ips: []string{"203.0.113.50"}, validatedIPs: []string{"203.0.113.50"},
			firstValidIP: "203.0.113.50", firstUntrustedIP: "203.0.113.50",
		},
		{
			name: "two adjacent commas", header: "203.0.113.50,,10.0.0.1",
			ips: []string{"203.0.113.50", "10.0.0.1"}, validatedIPs: []string{"203.0.113.50", "10.0.0.1"},
			firstValidIP: "203.0.113.50", firstUntrustedIP: "203.0.113.50",
		},
		{
			name: "three adjacent commas", header: "203.0.113.50,,,10.0.0.1",
			ips: []string{"203.0.113.50", "10.0.0.1"}, validatedIPs: []string{"203.0.113.50", "10.0.0.1"},
			firstValidIP: "203.0.113.50", firstUntrustedIP: "203.0.113.50",
		},
		{
			name: "whitespace-only elements", header: " \t,\t203.0.113.50\t, \t",
			ips: []string{"203.0.113.50"}, validatedIPs: []string{"203.0.113.50"},
			firstValidIP: "203.0.113.50", firstUntrustedIP: "203.0.113.50",
		},
		{
			name: "non-OWS stays invalid", header: "\v203.0.113.50, ::ffff:192.0.2.1",
			ips: []string{"\v203.0.113.50", "::ffff:192.0.2.1"}, validatedIPs: []string{"::ffff:192.0.2.1"},
			firstValidIP: "::ffff:192.0.2.1", firstUntrustedIP: "::ffff:192.0.2.1",
		},
		{
			name: "selection direction", header: "198.51.100.9,,203.0.113.50,10.0.0.1,10.0.0.2,",
			ips:          []string{"198.51.100.9", "203.0.113.50", "10.0.0.1", "10.0.0.2"},
			validatedIPs: []string{"198.51.100.9", "203.0.113.50", "10.0.0.1", "10.0.0.2"},
			firstValidIP: "198.51.100.9", firstUntrustedIP: "203.0.113.50",
		},
		{
			name: "only trusted valid IPs", header: "invalid,,10.0.0.1,10.0.0.2",
			ips: []string{"invalid", "10.0.0.1", "10.0.0.2"}, validatedIPs: []string{"10.0.0.1", "10.0.0.2"},
			firstValidIP: "10.0.0.1", firstUntrustedIP: "10.0.0.1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, mode := range []string{"raw", "validated", "trusted"} {
				t.Run(mode, func(t *testing.T) {
					t.Parallel()
					config := Config{ProxyHeader: HeaderXForwardedFor, EnableIPValidation: mode != "raw"}
					if mode == "trusted" {
						config = forwardedForProxies("10.0.0.1", remoteIP)
					}
					c := ctxFromIP(t, New(config), remoteIP)
					c.Request().Header.Set(HeaderXForwardedFor, tc.header)

					wantIPs, wantIP := tc.validatedIPs, tc.firstValidIP
					switch mode {
					case "raw":
						wantIPs, wantIP = tc.ips, tc.header
					case "trusted":
						wantIP = tc.firstUntrustedIP
					}
					require.Equal(t, wantIPs, c.IPs())
					require.Equal(t, wantIP, c.extractIPFromHeader(HeaderXForwardedFor))
				})
			}
		})
	}
}

// ctxBenchForwardedFor returns a context of an app with config whose request carries the X-Forwarded-For header forwardedFor.
func ctxBenchForwardedFor(b *testing.B, forwardedFor string, config ...Config) Ctx {
	b.Helper()
	c := apptest.Ctx(b, New(config...))
	c.Request().Header.Set(HeaderXForwardedFor, forwardedFor)
	return c
}

func Benchmark_Ctx_IPs(b *testing.B) {
	c := ctxBenchForwardedFor(b, "127.0.0.1, invalid, 127.0.0.1")
	var res []string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IPs()
	}
	require.Equal(b, []string{"127.0.0.1", "invalid", "127.0.0.1"}, res)
}

func Benchmark_Ctx_IPs_v6(b *testing.B) {
	c := ctxBenchForwardedFor(b, "f037:825e:eadb:1b7b:1667:6f0a:5356:f604, invalid, 2345:0425:2CA1::0567:5673:23b5")
	var res []string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IPs()
	}
	require.Equal(b, []string{"f037:825e:eadb:1b7b:1667:6f0a:5356:f604", "invalid", "2345:0425:2CA1::0567:5673:23b5"}, res)
}

func Benchmark_Ctx_IPs_With_IP_Validation(b *testing.B) {
	c := ctxBenchForwardedFor(b, "127.0.0.1, invalid, 127.0.0.1", Config{EnableIPValidation: true})
	var res []string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IPs()
	}
	require.Equal(b, []string{"127.0.0.1", "127.0.0.1"}, res)
}

func Benchmark_Ctx_IPs_v6_With_IP_Validation(b *testing.B) {
	c := ctxBenchForwardedFor(b, "2345:0425:2CA1:0000:0000:0567:5673:23b5, invalid, 2345:0425:2CA1::0567:5673:23b5", Config{EnableIPValidation: true})
	var res []string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IPs()
	}
	require.Equal(b, []string{"2345:0425:2CA1:0000:0000:0567:5673:23b5", "2345:0425:2CA1::0567:5673:23b5"}, res)
}

// ctxBenchProxied returns a context of an app with config for a request from remoteIP that carries the X-Forwarded-For
// header forwardedFor.
func ctxBenchProxied(b *testing.B, config *Config, remoteIP, forwardedFor string) Ctx {
	b.Helper()
	c := ctxFromIP(b, New(*config), remoteIP)
	c.Request().Header.Set(HeaderXForwardedFor, forwardedFor)
	return c
}

func Benchmark_Ctx_IP_With_ProxyHeader(b *testing.B) {
	c := ctxBenchProxied(b, &Config{
		ProxyHeader: HeaderXForwardedFor, TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Loopback: true},
	}, "127.0.0.1", "127.0.0.1")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IP()
	}
	require.Equal(b, "127.0.0.1", res)
}

func Benchmark_Ctx_IP_With_ProxyHeader_and_IP_Validation(b *testing.B) {
	c := ctxBenchProxied(b, &Config{
		ProxyHeader: HeaderXForwardedFor, TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Loopback: true},
		EnableIPValidation: true,
	}, "127.0.0.1", "127.0.0.1")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IP()
	}
	require.Equal(b, "127.0.0.1", res)
}

func Benchmark_Ctx_IP(b *testing.B) {
	c := apptest.Ctx(b, New())
	c.Request()
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IP()
	}
	require.Equal(b, "0.0.0.0", res)
}

func Benchmark_Ctx_IP_With_ProxyHeader_Chain(b *testing.B) {
	c := ctxBenchProxied(b, &Config{
		ProxyHeader: HeaderXForwardedFor, TrustProxy: true, EnableIPValidation: true,
		TrustProxyConfig: TrustProxyConfig{Proxies: []string{"10.0.0.1", "10.0.0.2"}},
	}, "10.0.0.2", "203.0.113.50, 10.0.0.1, 10.0.0.2")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IP()
	}
	require.Equal(b, "203.0.113.50", res)
}

// Benchmark_Ctx_IP_With_ProxyHeader_Lines reads a chain a proxy extended with a line of its own, off the wire.
func Benchmark_Ctx_IP_With_ProxyHeader_Lines(b *testing.B) {
	raw := "GET / HTTP/1.1\r\nHost: example.com\r\n" +
		"X-Forwarded-For: 203.0.113.50\r\nAccept: */*\r\nX-Forwarded-For: 10.0.0.1\r\n\r\n"
	fctx := wireRequestCtx(b, raw, &net.TCPAddr{IP: net.ParseIP("10.0.0.2")})
	c := apptest.CtxFor(b, New(forwardedForProxies("10.0.0.1", "10.0.0.2")), fctx)
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IP()
	}
	require.Equal(b, "203.0.113.50", res)
}

// Benchmark_Ctx_IP_With_ProxyHeader_LinesMoved reads the chain after a Del moved the proxy's line in front of the client's.
func Benchmark_Ctx_IP_With_ProxyHeader_LinesMoved(b *testing.B) {
	raw := "GET / HTTP/1.1\r\nHost: example.com\r\nX-Junk: 1\r\n" +
		"X-Forwarded-For: 203.0.113.50\r\nAccept: */*\r\nX-Forwarded-For: 10.0.0.1\r\n\r\n"
	fctx := wireRequestCtx(b, raw, &net.TCPAddr{IP: net.ParseIP("10.0.0.2")})
	c := apptest.CtxFor(b, New(forwardedForProxies("10.0.0.1", "10.0.0.2")), fctx)
	c.Request().Header.Del("X-Junk")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.IP()
	}
	require.Equal(b, "203.0.113.50", res)
}

func Test_Ctx_Is(t *testing.T) {
	t.Parallel()
	app := New()

	tests := []struct {
		name        string
		contentType string
		is          []string // extensions that must match the Content-Type
		isNot       []string // extensions that must not match it
	}{
		{name: "html with a parameter", contentType: MIMETextHTML + "; boundary=something", is: []string{".html", "html"}, isNot: []string{"json", ".json", "", ".foooo"}},
		{name: "json with charset", contentType: MIMEApplicationJSONCharsetUTF8, is: []string{"json", ".json"}, isNot: []string{"html"}},
		{name: "json with leading space", contentType: " application/json;charset=UTF-8", is: []string{"json", ".json"}, isNot: []string{"html"}},
		{name: "xml with charset", contentType: MIMEApplicationXMLCharsetUTF8, is: []string{"xml", ".xml"}, isNot: []string{"html"}},
		{name: "plain text", contentType: MIMETextPlain, is: []string{"txt", ".txt"}, isNot: []string{"html"}},
		{name: "case-insensitive and trimmed", contentType: "APPLICATION/JSON; charset=utf-8", is: []string{"json", ".json"}},
		{name: "mismatched subtype should not match", contentType: "application/json+xml", isNot: []string{"json", ".json"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)
			c.Request().Header.Set(HeaderContentType, tc.contentType)
			for _, extension := range tc.is {
				require.True(t, c.Is(extension), "Is(%q)", extension)
			}
			for _, extension := range tc.isNot {
				require.False(t, c.Is(extension), "Is(%q)", extension)
			}
		})
	}
}

func Benchmark_Ctx_Is(b *testing.B) {
	c := apptest.Ctx(b, New())

	c.Request().Header.Set(HeaderContentType, MIMEApplicationJSON)
	var res bool
	b.ReportAllocs()
	for b.Loop() {
		_ = c.Is(".json")
		res = c.Is("json")
	}
	require.True(b, res)
}

func Test_Ctx_Locals(t *testing.T) {
	t.Parallel()
	// Value reads the very store Locals writes, through the context.Context interface.
	for _, tc := range []struct {
		get  func(c Ctx) any
		name string
	}{
		{name: "Locals", get: func(c Ctx) any { return c.Locals("john") }},
		{name: "Value", get: func(c Ctx) any { return c.Value("john") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Use(func(c Ctx) error {
				c.Locals("john", "doe")
				return c.Next()
			})
			app.Get("/test", func(c Ctx) error {
				require.Equal(t, "doe", tc.get(c))
				return nil
			})
			apptest.Expect(t, app, "/test", StatusOK)
		})
	}
}

func Test_Ctx_Deadline(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c Ctx) error {
		deadline, ok := c.Deadline()
		require.Equal(t, time.Time{}, deadline)
		require.False(t, ok)

		// A user context with a deadline does not change this: Ctx is pooled and
		// can never be canceled, so it reports no deadline of its own.
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Hour))
		defer cancel()
		c.SetContext(ctx)
		deadline, ok = c.Deadline()
		require.Equal(t, time.Time{}, deadline)
		require.False(t, ok)
		return nil
	})
	apptest.Expect(t, app, "/test", StatusOK)
}

func Test_Ctx_Done(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c Ctx) error {
		require.Nil(t, c.Done())

		// Ctx can never be canceled. A cancellable user context does not change
		// that: a non-nil Done would make the stdlib retain the pooled Ctx in
		// watcher goroutines that outlive the handler.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c.SetContext(ctx)
		require.Nil(t, c.Done())
		cancel()
		require.Nil(t, c.Done())
		return nil
	})
	apptest.Expect(t, app, "/test", StatusOK)
}

func Test_Ctx_Err(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c Ctx) error {
		require.NoError(t, c.Err())

		// Err stays nil even after the user context is canceled, mirroring the
		// nil Done. Read the cancellation from Context() instead.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c.SetContext(ctx)
		cancel()
		require.NoError(t, c.Err())
		require.ErrorIs(t, c.Context().Err(), context.Canceled)
		return nil
	})
	apptest.Expect(t, app, "/test", StatusOK)
}

// ctxTestBReleasedCtx runs a GET /test request through a fresh app whose handler
// calls onRequest (when not nil) and returns the Ctx that handler received. The
// request is over by then, so the context has been released, which is the state
// the tests of the *_AfterRelease family probe.
func ctxTestBReleasedCtx(t *testing.T, onRequest func(c Ctx)) Ctx {
	t.Helper()

	app := New()
	var released Ctx
	app.Get("/test", func(c Ctx) error {
		released = c
		if onRequest != nil {
			onRequest(c)
		}
		return nil
	})
	apptest.Expect(t, app, "/test", StatusOK)
	return released
}

func Test_Ctx_Value_AfterRelease(t *testing.T) {
	t.Parallel()
	ctx := ctxTestBReleasedCtx(t, func(c Ctx) {
		c.Locals("test", "value")
		require.Equal(t, "value", c.Locals("test"))
	})

	// After the handler completes, the context is released and fasthttp is nil
	// Value should return nil instead of panicking
	require.NotPanics(t, func() { require.Nil(t, ctx.Value("test")) })
}

func Test_Ctx_Locals_AfterRelease(t *testing.T) {
	t.Parallel()
	ctx := ctxTestBReleasedCtx(t, func(c Ctx) { c.Locals("test", "value") })

	require.NotPanics(t, func() { require.Nil(t, ctx.Locals("test")) })
	require.NotPanics(t, func() { require.Equal(t, "fallback", ctx.Locals("test", "fallback")) })
}

func Test_Ctx_SetLocal(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error {
		c.SetLocal("john", "doe")
		c.SetLocal("age", 18)
		c.SetLocal("age", 19) // an existing key is overwritten, as with Locals
		return c.Next()
	})
	app.Get("/test", func(c Ctx) error {
		require.Equal(t, "doe", c.Locals("john"))
		require.Equal(t, "doe", c.Value("john"))
		require.Equal(t, 19, c.Locals("age"))
		require.Equal(t, 19, Locals[int](c, "age"))
		return nil
	})
	apptest.Expect(t, app, "/test", StatusOK)
}

func Test_Ctx_SetLocal_AfterRelease(t *testing.T) {
	t.Parallel()
	ctx := ctxTestBReleasedCtx(t, nil)

	require.NotPanics(t, func() {
		ctx.SetLocal("test", "value")
		require.Nil(t, ctx.Locals("test"), "a released context stores nothing")
	})
}

// Test_Ctx_SetLocal_NoAllocations pins what SetLocal exists for: storing a
// local through the Ctx interface without the heap-allocated one-element
// slice that the variadic Locals(key, value) costs on the same call.
//
// Not parallel: AllocsPerRun counts allocations process-wide, and Go pauses
// parallel tests while a sequential one runs.
func Test_Ctx_SetLocal_NoAllocations(t *testing.T) {
	app := New()
	var c Ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

	// boxing a constant costs nothing, so the slice is all a Locals call allocates
	c.SetLocal("user", "alice")
	require.Zero(t, testing.AllocsPerRun(100, func() { c.SetLocal("user", "alice") }))
	require.Equal(t, "alice", c.Locals("user"))
}

// Test_Ctx_SetLocal_UsesCustomCtxLocals is the counterpart to
// Test_setLocal_UsesCustomCtxLocals: the direct store exists only to keep the
// variadic slice off the heap, so it must never take precedence over a custom
// context's own Locals, which a type embedding DefaultCtx is entitled to
// override.
func Test_Ctx_SetLocal_UsesCustomCtxLocals(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, NewWithCustomCtx(func(app *App) CustomCtx {
		return &localsRecordingCtx{DefaultCtx: *NewDefaultCtx(app)}
	}))

	custom, ok := c.(*localsRecordingCtx)
	require.True(t, ok, "the app must hand out the custom context")

	c.SetLocal("k", "v")
	require.Equal(t, 1, custom.calls, "the overridden Locals must be the one that ran")
	require.Equal(t, "v", c.Locals("k"), "and the value must actually be stored")
}

func Benchmark_Ctx_SetLocal(b *testing.B) {
	app := New()
	var c Ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	b.Cleanup(func() { app.ReleaseCtx(c) })

	b.Run("Locals", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c.Locals("user", "alice")
		}
	})
	b.Run("SetLocal", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c.SetLocal("user", "alice")
		}
	})
	require.Equal(b, "alice", c.Locals("user"))
}

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

	apptest.Expect(t, app, "/test", StatusOK)

	// Signal goroutine to proceed - context has been released after app.Test returns
	// since the handler (and its deferred ReleaseCtx) has completed
	close(start)

	select {
	case <-done:
		// Success - goroutine completed without panic
	case err := <-errCh:
		t.Fatalf("error from goroutine: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("test timed out waiting for goroutine")
	}
}

func Test_Ctx_Context(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.Equal(t, context.Background(), c.Context())
}

func Test_Ctx_Context_AfterHandlerPanics(t *testing.T) {
	t.Parallel()
	ctx := ctxTestBReleasedCtx(t, nil)

	// After the fix, Context() returns context.Background() instead of panicking
	require.NotPanics(t, func() {
		c := ctx.Context()
		require.NotNil(t, c)
		require.Equal(t, context.Background(), c)
	})
}

func Test_Ctx_Request_Response_AfterRelease(t *testing.T) {
	t.Parallel()
	ctx := ctxTestBReleasedCtx(t, nil)

	// After the handler completes and context is released,
	// Request() and Response() should return nil instead of panicking
	require.NotPanics(t, func() {
		require.Nil(t, ctx.Request())
		require.Nil(t, ctx.Response())
	})
}

func Test_Ctx_SetContext(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	var testKey testContextKey
	testValue := "Test Value"
	ctx := context.WithValue(context.Background(), testKey, testValue)
	c.SetContext(ctx)
	require.Equal(t, testValue, c.Context().Value(testKey))
}

type contextHelperTestKey struct{}

func Test_Ctx_StoreInContext_Config(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		config    Config
		inContext any // what the context holds under the key: nil when the config keeps the value out of it
		name      string
		stored    string
	}{
		{name: "disabled", stored: "locals-only"},
		{name: "enabled", config: Config{PassLocalsToContext: true}, stored: "both", inContext: "both"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New(tc.config))

			StoreInContext(c, contextHelperTestKey{}, tc.stored)

			value, ok := c.Locals(contextHelperTestKey{}).(string)
			require.True(t, ok)
			require.Equal(t, tc.stored, value)
			require.Equal(t, tc.inContext, c.Context().Value(contextHelperTestKey{}))
		})
	}
}

func Test_Ctx_ValueFromContext_Config(t *testing.T) {
	t.Parallel()
	custom := ctxTestACustomApp(Config{PassLocalsToContext: true})
	for _, tc := range []struct {
		app  *App
		name string
	}{
		{name: "fiber ctx disabled reads locals", app: New()},
		{name: "fiber ctx enabled still reads locals", app: New(Config{PassLocalsToContext: true})},
		{name: "fiber custom ctx enabled still reads locals", app: custom},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, tc.app)

			c.Locals(contextHelperTestKey{}, "locals")
			c.SetContext(context.WithValue(context.Background(), contextHelperTestKey{}, "context"))

			value, ok := ValueFromContext[string](c, contextHelperTestKey{})
			require.True(t, ok)
			require.Equal(t, "locals", value)
		})
	}

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

	for i := 1; i <= 10; i++ {
		t.Run(fmt.Sprintf("request_%d", i), func(t *testing.T) {
			t.Parallel()
			// a 500 means the context.Context returned from c.Context() was reused
			apptest.ExpectBody(t, app, fmt.Sprintf("/?input=%d", i), StatusOK, fmt.Sprintf("resp_%d_returned", i))
		})
	}
}

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
	apptest.Expect(t, app, "/test", StatusOK)
}

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
	apptest.Expect(t, app, "/test", StatusOK)
}

func Test_Ctx_Method(t *testing.T) {
	t.Parallel()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	c := apptest.CtxFor(t, New(), fctx)

	require.Equal(t, MethodGet, c.Method())
	c.Method(MethodPost)
	require.Equal(t, MethodPost, c.Method())

	// Lowercase input still maps to the registered uppercase method.
	c.Method("get")
	require.Equal(t, MethodGet, c.Method())

	c.Method("MethodInvalid")
	require.Equal(t, MethodGet, c.Method())
}

// Test_Ctx_Method_Unregistered verifies that a ctx carrying a method that is
// not registered in RequestMethods (methodInt == -1) does not panic: Method()
// reports the raw request-header method and Route()'s fallback stays safe.
func Test_Ctx_Method_Unregistered(t *testing.T) {
	t.Parallel()
	app := New()
	rctx := &fasthttp.RequestCtx{}
	rctx.Request.Header.SetMethod("PURGE")
	c := apptest.CtxFor(t, app, rctx)

	require.Equal(t, "PURGE", c.Method())
	// An invalid override keeps reporting the raw method instead of panicking.
	require.Equal(t, "PURGE", c.Method("StillInvalid"))
	// Route()'s fallback resolves the method the same way Method() does.
	require.Equal(t, "PURGE", c.Route().Method)
}

// Test_Ctx_Method_CustomCaseSensitive verifies that a mixed-case custom
// method registered via Config.RequestMethods can be set as an override:
// method tokens are case-sensitive (RFC 9110 §9.1), so the override must not
// be unconditionally uppercased.
func Test_Ctx_Method_CustomCaseSensitive(t *testing.T) {
	t.Parallel()
	app := New(Config{RequestMethods: append(append([]string{}, DefaultMethods...), "Purge")})
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	c := apptest.CtxFor(t, app, fctx)

	require.Equal(t, "Purge", c.Method("Purge"))
	require.Equal(t, "Purge", c.Method())

	// The uppercase convenience fallback still applies for standard methods.
	require.Equal(t, MethodPost, c.Method("post"))
}

func Test_Ctx_ClientHelloInfo(t *testing.T) {
	t.Parallel()
	app := New()
	// every route answers with one field of the ClientHelloInfo, or says it is nil
	for _, route := range []struct {
		send func(c Ctx, info *tls.ClientHelloInfo) error
		path string
	}{
		{path: "/ServerName", send: func(c Ctx, info *tls.ClientHelloInfo) error { return c.SendString(info.ServerName) }},
		{path: "/SignatureSchemes", send: func(c Ctx, info *tls.ClientHelloInfo) error { return c.JSON(info.SignatureSchemes) }},
		{path: "/SupportedVersions", send: func(c Ctx, info *tls.ClientHelloInfo) error { return c.JSON(info.SupportedVersions) }},
	} {
		app.Get(route.path, func(c Ctx) error {
			result := c.ClientHelloInfo()
			if result != nil {
				return route.send(c, result)
			}

			return c.SendString("ClientHelloInfo is nil")
		})
	}

	// without a TLS handler
	apptest.ExpectBody(t, app, "/ServerName", StatusOK, "ClientHelloInfo is nil")

	const (
		pssWithSHA256 = 0x0804
		versionTLS13  = 0x0304
	)
	app.tlsHandler = &TLSHandler{}
	_, err := app.tlsHandler.GetClientInfo(&tls.ClientHelloInfo{
		ServerName:        "example.golang",
		SignatureSchemes:  []tls.SignatureScheme{pssWithSHA256},
		SupportedVersions: []uint16{versionTLS13},
	})
	require.NoError(t, err)
	apptest.ExpectBody(t, app, "/ServerName", StatusOK, "example.golang")
	apptest.ExpectBody(t, app, "/SignatureSchemes", StatusOK, "["+strconv.Itoa(pssWithSHA256)+"]")
	apptest.ExpectBody(t, app, "/SupportedVersions", StatusOK, "["+strconv.Itoa(versionTLS13)+"]")
}

func Test_Ctx_InvalidMethod(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(_ Ctx) error { return nil })

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("InvalidMethod")
	fctx.Request.SetRequestURI("/")

	app.Handler()(fctx)

	require.Equal(t, 501, fctx.Response.StatusCode())
	require.Equal(t, []byte("Not Implemented"), fctx.Response.Body())
}

func Test_Ctx_MultipartForm(t *testing.T) {
	t.Parallel()
	app := New()

	app.Post("/test", func(c Ctx) error {
		result, err := c.MultipartForm()
		require.NoError(t, err)
		require.Equal(t, "john", result.Value["name"][0])
		return nil
	})

	body, contentType := ctxTestMultipartField(t, "name", "john")
	req := httptest.NewRequest(MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set(HeaderContentType, contentType)
	req.Header.Set(HeaderContentLength, strconv.Itoa(len(body)))
	apptest.Expect(t, app, req, StatusOK)
}

func Test_Ctx_MultipartForm_BodyLimitExceeded(t *testing.T) {
	t.Parallel()
	c := apptest.CtxAs[*DefaultCtx](t, New(Config{BodyLimit: 64}))

	body, contentType := ctxTestMultipartField(t, "name", strings.Repeat("a", 1024))
	c.Request().Header.Set(HeaderContentType, contentType)
	c.Request().Header.Set(HeaderContentEncoding, StrGzip)
	c.Request().SetBody(ctxTestACompress(t, StrGzip, body))

	_, err := c.MultipartForm()
	require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
}

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

func Test_Ctx_OriginalURL(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().Header.SetRequestURI("http://google.com/test?search=demo")
	require.Equal(t, "http://google.com/test?search=demo", c.OriginalURL())
}

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
	for _, target := range []string{
		"/test/john", "/test2/im/a/cookie", "/test3/1111/blafasel/2222", "/test4", "/test5/first/second",
	} {
		apptest.Expect(t, app, target, StatusOK)
	}
}

func Test_Ctx_Params_ErrorHandler_Panic_Issue_2832(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ErrorHandler: func(c Ctx, _ error) error { return c.SendString(c.Params("user")) },
		BodyLimit:    1 * 1024,
	})

	app.Get("/test/:user", func(_ Ctx) error { return NewError(StatusInternalServerError, "error") })

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
	apptest.Expect(t, app, "/test/john", StatusOK)
	apptest.Expect(t, app, "/test2/first/second", StatusOK)
}

func Test_Ctx_Params_Immutable(t *testing.T) {
	t.Parallel()
	c := apptest.CtxAs[*DefaultCtx](t, New(Config{Immutable: true}))

	c.route = &Route{Params: []string{"user"}}
	c.path = []byte("/test/john")
	c.values[0] = c.app.toString(c.path[6:])

	param := c.Params("user")
	copy(c.path[6:], "paul")
	require.Equal(t, "john", param)
}

func Benchmark_Ctx_Params(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())

	c.route = &Route{Params: []string{"param1", "param2", "param3", "param4"}}
	c.values = [maxParams]string{"john", "doe", "is", "awesome"}
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

	app.Get("/specialChars/:name", func(c Ctx) error {
		require.Equal(t, "/specialChars/créer", c.Path())
		// unescape is also working if you set the path afterwards
		require.Equal(t, "/اختبار/", c.Path("/%D8%A7%D8%AE%D8%AA%D8%A8%D8%A7%D8%B1/"))
		require.Equal(t, "/اختبار/", string(c.Request().URI().Path()))
		return nil
	})
	apptest.Expect(t, app, "/specialChars/cr%C3%A9er", StatusOK)
}

// Test_Ctx_Path_OverrideNormalizes pins that an overridden path is normalized
// from what the override says, not from what the request arrived with. Reset
// records whether the request path needs normalizing, so an override has to
// record it again in both directions: a plain request overridden with a dot
// segment or an escape, and an escaped request overridden with a plain path.
func Test_Ctx_Path_OverrideNormalizes(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/plain", func(c Ctx) error {
		require.Equal(t, "/a/b", c.Path("/a/./b"))
		require.Equal(t, "/x/y", c.Path("/%78/y"))
		require.Equal(t, "/a/b", c.Path("/a/c/../b"))
		return c.SendString("ok")
	})
	app.Get("/x", func(c Ctx) error {
		require.Equal(t, "/x", c.Path())
		require.Equal(t, "/plain", c.Path("/plain"))
		require.Equal(t, "/plain", string(c.Request().URI().Path()))
		return c.SendString("ok")
	})

	for _, target := range []string{"/plain", "/%78"} {
		apptest.Expect(t, app, target, StatusOK)
	}
}

func Test_Ctx_Protocol(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.Equal(t, "HTTP/1.1", c.Protocol())

	c.Request().Header.SetProtocol("HTTP/2")
	require.Equal(t, "HTTP/2", c.Protocol())
}

func Benchmark_Ctx_Protocol(b *testing.B) {
	c := apptest.Ctx(b, New())

	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.Protocol()
	}

	require.Equal(b, "HTTP/1.1", res)
}

func Test_Ctx_Scheme(t *testing.T) {
	t.Parallel()
	// Every way a proxy can announce HTTPS. Header names are case-insensitive
	// and the values are trimmed, which the padded and mixed-case rows pin.
	announcements := []struct {
		name   string
		header string
		value  string
	}{
		{name: "X-Forwarded-Proto", header: HeaderXForwardedProto, value: schemeHTTPS},
		{name: "X-Forwarded-Protocol", header: HeaderXForwardedProtocol, value: schemeHTTPS},
		{name: "X-Forwarded-Proto list", header: HeaderXForwardedProto, value: "https, http"},
		{name: "X-Forwarded-Protocol list", header: HeaderXForwardedProtocol, value: "https, http"},
		{name: "X-Forwarded-Ssl", header: HeaderXForwardedSsl, value: "on"},
		{name: "X-Url-Scheme", header: HeaderXUrlScheme, value: schemeHTTPS},
		{name: "lowercase X-Forwarded-Proto, padded list", header: "x-forwarded-proto", value: " HTTPS , http"},
		{name: "uppercase X-Forwarded-Protocol, padded", header: "X-FORWARDED-PROTOCOL", value: " HTTPS"},
		{name: "lowercase X-Url-Scheme, padded", header: "x-url-scheme", value: " HTTPS "},
		{name: "mixed case X-Forwarded-Protocol, padded", header: "x-Forwarded-ProToCol", value: " HTTPS "},
	}

	// The peer is always 0.0.0.0; the rows differ in which proxies the app trusts.
	for _, tc := range []struct {
		name  string
		proxy string
		want  string // the scheme a header announcing HTTPS leads to
	}{
		{name: "trusted proxy", proxy: "0.0.0.0", want: schemeHTTPS},
		{name: "trusted proxy range", proxy: "0.0.0.0/30", want: schemeHTTPS},
		{name: "untrusted proxy range", proxy: "1.1.1.1/30", want: schemeHTTP},
		{name: "untrusted proxy", proxy: "0.8.0.1", want: schemeHTTP},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{tc.proxy}}})
			for _, announcement := range announcements {
				t.Run(announcement.name, func(t *testing.T) {
					t.Parallel()
					c := ctxFromIP(t, app, "0.0.0.0")
					// too short to be an X-Forwarded-* header, Scheme must skip it
					c.Request().Header.Set("X-Forwarded", "invalid")
					c.Request().Header.Set(announcement.header, announcement.value)
					require.Equal(t, tc.want, c.Scheme())
				})
			}

			t.Run("no announcement", func(t *testing.T) {
				t.Parallel()
				require.Equal(t, schemeHTTP, ctxFromIP(t, app, "0.0.0.0").Scheme())
			})
		})
	}
}

func Test_Ctx_Scheme_RejectsForeignSchemes(t *testing.T) {
	t.Parallel()
	app := New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.0.0.0"}}})

	c := ctxFromIP(t, app, "0.0.0.0")

	// A scheme Fiber does not serve must never reach BaseURL or the
	// same-origin comparisons that consume Scheme.
	for _, header := range []string{HeaderXForwardedProto, HeaderXForwardedProtocol, HeaderXUrlScheme} {
		for _, value := range []string{"javascript", "ftp", "data", "HTTPS evil", ""} {
			c.Request().Header.Set(header, value)
			require.Equal(t, schemeHTTP, c.Scheme(), "%s: %q", header, value)
			c.Request().Header.Reset()
		}
	}

	// A rejected value must not clobber a valid one supplied by another header.
	c.Request().Header.Set(HeaderXForwardedProto, schemeHTTPS)
	c.Request().Header.Set(HeaderXUrlScheme, "javascript")
	require.Equal(t, schemeHTTPS, c.Scheme())
	c.Request().Header.Reset()

	// BaseURL is the reason this matters: a foreign scheme spliced in here
	// yields a URL an application may hand to a browser.
	c.Request().URI().SetHost("example.com")
	c.Request().Header.Set(HeaderXForwardedProto, "javascript")
	require.Equal(t, "http://example.com", c.BaseURL())
}

func Test_Ctx_Scheme_HeaderPrecedence(t *testing.T) {
	t.Parallel()
	app := New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.0.0.0"}}})

	// A proxy overwrites X-Forwarded-Proto and relays the others, so its value wins in either order.
	for _, tc := range []struct {
		proto  string
		header string
		value  string
	}{
		{proto: schemeHTTPS, header: HeaderXForwardedProtocol, value: schemeHTTP},
		{proto: schemeHTTPS, header: HeaderXUrlScheme, value: schemeHTTP},
		{proto: schemeHTTP, header: HeaderXForwardedProtocol, value: schemeHTTPS},
		{proto: schemeHTTP, header: HeaderXForwardedSsl, value: "on"},
		{proto: schemeHTTP, header: HeaderXUrlScheme, value: schemeHTTPS},
	} {
		t.Run(tc.header+"_"+tc.value, func(t *testing.T) {
			t.Parallel()
			c := ctxFromIP(t, app, "0.0.0.0")

			c.Request().Header.Set(HeaderXForwardedProto, tc.proto)
			c.Request().Header.Set(tc.header, tc.value)
			require.Equal(t, tc.proto, c.Scheme())
			c.Request().Header.Reset()

			c.Request().Header.Set(tc.header, tc.value)
			c.Request().Header.Set(HeaderXForwardedProto, tc.proto)
			require.Equal(t, tc.proto, c.Scheme())
		})
	}

	for _, tc := range []struct {
		name    string
		want    string
		headers []string // names and values of the request headers
	}{
		{
			name: "X-Forwarded-Protocol over X-Forwarded-Ssl and X-Url-Scheme",
			headers: []string{
				HeaderXUrlScheme, schemeHTTPS, HeaderXForwardedSsl, "on", HeaderXForwardedProtocol, schemeHTTP,
			},
			want: schemeHTTP,
		},
		{
			name:    "X-Forwarded-Ssl over X-Url-Scheme",
			headers: []string{HeaderXUrlScheme, schemeHTTP, HeaderXForwardedSsl, "on"},
			want:    schemeHTTPS,
		},
		{
			name: "an invalid value outranks nothing",
			headers: []string{
				HeaderXForwardedProto, "javascript", HeaderXForwardedSsl, "off", HeaderXUrlScheme, schemeHTTPS,
			},
			want: schemeHTTPS,
		},
	} {
		t.Run("Ranking, "+tc.name, func(t *testing.T) {
			t.Parallel()
			c := ctxFromIP(t, app, "0.0.0.0")
			for name, value := range apptest.Pairs(tc.headers) {
				c.Request().Header.Set(name, value)
			}
			require.Equal(t, tc.want, c.Scheme())
		})
	}
}

// Repeats of one header keep the last line, which an empty chunked body used to move.
func Test_Ctx_Scheme_RepeatedFieldLines_ReceivedOrder(t *testing.T) {
	t.Parallel()
	app := New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{"127.0.0.1"}}})
	app.Post("/", func(c Ctx) error { return c.SendString(c.Scheme()) })

	raw := "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\n" +
		"X-Forwarded-Proto: http\r\nX-Forwarded-Proto: https\r\n\r\n0\r\n\r\n"
	require.Equal(t, schemeHTTPS, serveRawFromLoopback(t, app.Handler(), raw))
}

func Benchmark_Ctx_Scheme(b *testing.B) {
	c := apptest.Ctx(b, New())

	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = c.Scheme()
	}
	require.Equal(b, "http", res)
}

func Test_Ctx_Query(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().URI().SetQueryString("search=john&age=20")
	require.Equal(t, "john", c.Query("search"))
	require.Equal(t, "20", c.Query("age"))
	require.Equal(t, "default", c.Query("unknown", "default"))
	require.Equal(t, "john", Query[string](c, "search"))
	require.Equal(t, "20", Query[string](c, "age"))
	require.Equal(t, "default", Query(c, "unknown", "default"))
}

func Benchmark_Ctx_Query(b *testing.B) {
	c := apptest.Ctx(b, New())
	c.Request().URI().SetQueryString("search=john&age=8")
	var res string
	b.ReportAllocs()
	for b.Loop() {
		res = Query[string](c, "search")
	}
	require.Equal(b, "john", res)
}

// Benchmark_Ctx_Query_Generic_Default exercises the typed Query[V] failure path,
// where genericParseType returns an error for a missing or unparsable value and
// the default is returned - the path the sentinel-error change makes alloc-free.
func Benchmark_Ctx_Query_Generic_Default(b *testing.B) {
	c := apptest.Ctx(b, New())
	c.Request().URI().SetQueryString("age=notanumber")
	var n, m int
	b.ReportAllocs()
	for b.Loop() {
		n = Query[int](c, "missing", 7) // absent -> parse "" fails -> default
		m = Query[int](c, "age", 3)     // unparsable -> fails -> default
	}
	require.Equal(b, 7, n)
	require.Equal(b, 3, m)
}

func Test_Ctx_Range(t *testing.T) {
	t.Parallel()
	app := New()

	tests := []struct {
		name   string
		header string
		ranges []RangeSet // none: the header is invalid and Range must fail without returning partial ranges
	}{
		{name: "bound without a dash", header: "bytes=500"},
		{name: "empty range set", header: "bytes="},
		{name: "stray equals sign", header: "bytes=500="},
		{name: "start after end", header: "bytes=500-300"},
		// A bound that is present but not a valid integer invalidates the whole
		// ranges-specifier (RFC 9110 §14.1.1); it must not be reinterpreted as a
		// suffix or open-ended range.
		{name: "start is not an integer", header: "bytes=a-700"},
		{name: "end is not an integer", header: "bytes=500-b"},
		{name: "lone dash", header: "bytes=-"},
		{name: "end clamped to the size", header: "bytes=500-1000", ranges: []RangeSet{{Start: 500, End: 999}}},
		{name: "start and end", header: "bytes=500-700", ranges: []RangeSet{{Start: 500, End: 700}}},
		// Range units are case-insensitive (RFC 9110 §14.1); the reported Type is
		// still the canonical lowercase form.
		{name: "unit in mixed case", header: "Bytes=500-700", ranges: []RangeSet{{Start: 500, End: 700}}},
		{name: "unit in upper case", header: "BYTES=500-700", ranges: []RangeSet{{Start: 500, End: 700}}},
		{name: "two ranges, the last end clamped", header: "bytes=0-0,2-1000", ranges: []RangeSet{{Start: 0, End: 0}, {Start: 2, End: 999}}},
		{name: "three ranges, the last a suffix", header: "bytes=0-99,450-549,-100", ranges: []RangeSet{{Start: 0, End: 99}, {Start: 450, End: 549}, {Start: 900, End: 999}}},
		{name: "overlapping ranges", header: "bytes=500-700,601-999", ranges: []RangeSet{{Start: 500, End: 700}, {Start: 601, End: 999}}},
		{name: "space after the unit", header: "bytes= 0-1", ranges: []RangeSet{{Start: 0, End: 1}}},
		// Empty list elements must be parsed and ignored (RFC 9110 §5.6.1.2).
		{name: "leading empty element", header: "bytes=,0-5", ranges: []RangeSet{{Start: 0, End: 5}}},
		{name: "empty element between ranges", header: "bytes=0-5,,10-20", ranges: []RangeSet{{Start: 0, End: 5}, {Start: 10, End: 20}}},
		{name: "only empty elements", header: "bytes=,,,"},
		// A malformed later element invalidates the whole header: no partial
		// ranges may leak out alongside the error.
		{name: "malformed later element", header: "bytes=0-5,zzz-10"},
		{name: "unknown unit", header: "seconds=0-1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)

			c.Request().Header.Set(HeaderRange, tc.header)
			result, err := c.Range(1000)
			if len(tc.ranges) == 0 {
				require.Error(t, err)
			} else {
				require.Equal(t, "bytes", result.Type)
				require.NoError(t, err)
			}
			require.Len(t, result.Ranges, len(tc.ranges))
			for i := range tc.ranges {
				require.Equal(t, tc.ranges[i], result.Ranges[i])
			}
		})
	}
}

func Test_Ctx_Range_Errors(t *testing.T) {
	t.Parallel()
	tooBig := uint64((math.MaxUint64 >> 1) + 1)

	tests := []struct {
		wantErr   error
		name      string
		header    string
		size      int64
		maxRanges int
	}{
		// A grammatically valid but unknown range unit is not malformed: it must
		// be signaled distinctly so the caller can ignore the header, as
		// RFC 9110 §14.2 requires.
		{name: "unknown unit", header: "pages=1-3", size: 1000, wantErr: ErrRangeUnsupported},
		// The other-range grammar permits "=", so an unknown unit whose spec
		// contains "=" is still unsupported, not malformed.
		{name: "unknown unit with equals sign in the spec", header: "pages=1-3=note", size: 1000, wantErr: ErrRangeUnsupported},
		// Empty list elements still count toward Config.MaxRanges, so the cap
		// bounds parsing work per header.
		{name: "empty elements count toward MaxRanges", header: "bytes=,,,,0-1", size: 10, maxRanges: 2, wantErr: ErrRangeTooLarge},
		{name: "start overflows int64", header: fmt.Sprintf("bytes=%d-100", tooBig), size: math.MaxInt64, wantErr: ErrRangeMalformed},
		{name: "end overflows int64", header: fmt.Sprintf("bytes=0-%d", tooBig), size: math.MaxInt64, wantErr: ErrRangeMalformed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New(Config{MaxRanges: tc.maxRanges}))

			c.Request().Header.Set(HeaderRange, tc.header)
			_, err := c.Range(tc.size)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// Test_Ctx_Range_ErrorStatus verifies how a Range error returned by a handler
// reaches the client: an invalid ranges-specifier maps to 400 Bad Request (RFC
// 9110 permits rejecting it), not 500, and an unsatisfiable or oversized one to
// 416 with the size of the representation.
func Test_Ctx_Range_ErrorStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		header       string
		contentRange string
		maxRanges    int
		status       int
	}{
		{name: "malformed", header: "bytes=a-700", status: StatusBadRequest},
		{name: "unsatisfiable", header: "bytes=20-30", status: StatusRequestedRangeNotSatisfiable, contentRange: "bytes */10"},
		{name: "too many ranges", header: "bytes=0-1,2-3,4-5", maxRanges: 2, status: StatusRequestedRangeNotSatisfiable, contentRange: "bytes */10"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New(Config{MaxRanges: tc.maxRanges})
			app.Get("/", func(c Ctx) error {
				_, err := c.Range(10)
				if err != nil {
					return err
				}
				return c.SendString("ok")
			})

			req := apptest.Req(MethodGet, "http://example.com/", apptest.WithHeader(HeaderRange, tc.header))
			resp := apptest.Expect(t, app, req, tc.status)
			if tc.contentRange != "" {
				require.Equal(t, tc.contentRange, resp.Header.Get(HeaderContentRange))
			}
		})
	}
}

func Test_Ctx_Range_LargeFile(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

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

func Test_Ctx_Range_SuffixNormalization(t *testing.T) {
	t.Parallel()
	body := bytes.Repeat([]byte("x"), 123)

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

	for _, tc := range []struct {
		name             string
		rangeHeader      string
		contentRange     string
		statusCode       int
		expectedBodySize int
	}{
		{
			name: "suffix less than size", rangeHeader: "bytes=-20",
			contentRange: "bytes 103-122/123", statusCode: StatusPartialContent, expectedBodySize: 20,
		},
		{
			name: "suffix equal to size", rangeHeader: "bytes=-123",
			contentRange: "bytes 0-122/123", statusCode: StatusOK, expectedBodySize: 123,
		},
		{
			name: "suffix larger than size", rangeHeader: "bytes=-9999",
			contentRange: "bytes 0-122/123", statusCode: StatusOK, expectedBodySize: 123,
		},
		{
			name: "unsatisfiable mixed ranges", rangeHeader: "bytes=200-400,700-1200",
			contentRange: "bytes */123", statusCode: StatusRequestedRangeNotSatisfiable,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := apptest.Req(MethodGet, "http://example.com/", apptest.WithHeader(HeaderRange, tc.rangeHeader))
			resp := apptest.Expect(t, app, req, tc.statusCode)
			require.Equal(t, tc.contentRange, resp.Header.Get(HeaderContentRange))
			if tc.expectedBodySize > 0 {
				require.Len(t, apptest.BodyBytes(t, resp), tc.expectedBodySize)
			}
		})
	}
}

func Benchmark_Ctx_Range(b *testing.B) {
	c := apptest.Ctx(b, New())

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

func Test_Ctx_Route(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c Ctx) error {
		require.Equal(t, "/test", c.Route().Path)
		return nil
	})
	apptest.Expect(t, app, "/test", StatusOK)

	c := apptest.Ctx(t, app)
	require.Equal(t, "/", c.Route().Path)
	require.Equal(t, MethodGet, c.Route().Method)
	require.Empty(t, c.Route().Handlers)
}

func Test_Ctx_FullPath(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c Ctx) error {
		require.Equal(t, "/test", c.FullPath())
		return c.SendStatus(StatusOK)
	})

	apptest.Expect(t, app, "/test", StatusOK)
}

func Test_Ctx_FullPath_Group(t *testing.T) {
	t.Parallel()
	app := New()
	group := app.Group("/v1")
	group.Get("/test", func(c Ctx) error {
		require.Equal(t, "/v1/test", c.FullPath())
		return c.SendStatus(StatusOK)
	})

	apptest.Expect(t, app, "/v1/test", StatusOK)
}

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

	apptest.Expect(t, app, "/test", StatusOK)
	require.Equal(t, []string{"/", "/test"}, recorded)
}

func Test_Ctx_Endpoint_Middleware(t *testing.T) {
	t.Parallel()
	app := New()

	var recorded []string

	app.Use(func(c Ctx) error {
		if route := c.Endpoint(); route != nil {
			recorded = append(recorded, route.Path, route.Name)
		}
		return c.Next()
	})

	app.Get("/users/:id", func(c Ctx) error {
		require.Equal(t, "/users/:id", c.Endpoint().Path)
		require.Equal(t, "user.show", c.Endpoint().Name)
		return c.SendStatus(StatusOK)
	}).Name("user.show")

	apptest.Expect(t, app, "/users/42", StatusOK)
	require.Equal(t, []string{"/users/:id", "user.show"}, recorded)
}

func Test_Ctx_Endpoint_NotFound(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use(func(c Ctx) error {
		require.Nil(t, c.Endpoint())
		return c.Next()
	})

	apptest.Expect(t, app, "/not-found", StatusNotFound)
}

func Test_Ctx_Endpoint_RepeatedAndSkipped(t *testing.T) {
	t.Parallel()
	app := New()

	var calls int
	app.Use(func(c Ctx) error {
		// Repeated calls resolve to the same *Route out of the tree, without a
		// cached copy in between.
		first := c.Endpoint()
		second := c.Endpoint()
		require.Same(t, first, second)
		calls++
		return c.Next()
	})
	// A route with a non-matching prefix exercises prefixRejects skip.
	app.Get("/other/:id", func(c Ctx) error { return c.SendStatus(StatusOK) })
	// A middleware route registered ahead of the target exercises the use/mount skip.
	app.Use("/users", func(c Ctx) error { return c.Next() })
	app.Get("/users/:id", func(c Ctx) error { return c.SendStatus(StatusOK) }).Name("user.show")

	apptest.Expect(t, app, "/users/42", StatusOK)
	require.Equal(t, 1, calls)
}

func Benchmark_Ctx_Endpoint(b *testing.B) {
	app := New()
	app.Use(func(c Ctx) error { return c.Next() })
	registerDummyRoutes(app)
	app.startupProcess()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodDelete)
	fctx.Request.SetRequestURI("/user/keys/1337")
	c := app.AcquireCtx(fctx).(*DefaultCtx) //nolint:errcheck,forcetypeassert // matches the other ctx benchmarks
	defer app.ReleaseCtx(c)
	c.Reset(fctx)

	var route *Route
	b.ReportAllocs()
	for b.Loop() {
		route = c.Endpoint()
	}
	require.NotNil(b, route)
}

// serverErrorHandler replays the chain for protocol-level errors with
// shouldSkipNonUseRoutes set, and next() then runs no endpoint at all.
func Test_Ctx_Endpoint_NilWhenNoEndpointCanRun(t *testing.T) {
	t.Parallel()
	app := New(Config{BodyLimit: 8})

	var reported []*Route
	endpointRan := false
	app.Use(func(c Ctx) error {
		reported = append(reported, c.Endpoint())
		return c.Next()
	})
	app.Post("/big", func(c Ctx) error {
		endpointRan = true
		return c.SendString("endpoint")
	}).Name("big.post")

	req := httptest.NewRequest(MethodPost, "/big", bytes.NewReader(bytes.Repeat([]byte("x"), 4096)))
	_, err := app.Test(req)
	require.Error(t, err, "the body limit should reject this request")

	require.False(t, endpointRan)
	require.NotEmpty(t, reported, "the middleware chain should still have run")
	for _, r := range reported {
		require.Nil(t, r, "no endpoint can run in this pass, so none may be named")
	}
}

// Sweeps the whole route fixture: whatever the look-ahead names in middleware
// must be the endpoint next() then executes, or nil when none does. prefixRejects
// warns that hand-written scan copies drift unnoticed; this is that guard.
func Test_Ctx_Endpoint_MatchesTheRouteThatRuns(t *testing.T) {
	t.Parallel()
	app := New()

	var predicted, actual *Route
	app.Use(func(c Ctx) error {
		predicted = c.Endpoint()
		return c.Next()
	})
	// A middleware between the observer and the endpoints: the scan has to skip it,
	// and the sweep has to notice if it ever stops doing so.
	app.Use("/repos", func(c Ctx) error { return c.Next() })
	registerDummyRoutes(app)
	// Record what really ran, from inside the endpoint's own chain position.
	for _, r := range app.stack {
		for _, route := range r {
			if route.use || route.mount || len(route.Handlers) == 0 {
				continue
			}
			handler := route.Handlers[len(route.Handlers)-1]
			route.Handlers[len(route.Handlers)-1] = func(c Ctx) error {
				actual = c.Route()
				return handler(c)
			}
		}
	}

	for i := range routesFixture.TestRoutes {
		tr := routesFixture.TestRoutes[i]
		predicted, actual = nil, nil

		req := httptest.NewRequest(tr.Method, tr.Path, http.NoBody)
		resp, err := app.Test(req, sweepTestConfig)
		require.NoError(t, err, "%s %s", tr.Method, tr.Path)
		require.NoError(t, resp.Body.Close())

		require.Same(t, actual, predicted,
			"%s %s: look-ahead named %v but %v ran", tr.Method, tr.Path,
			routeName(predicted), routeName(actual))
	}
}

func routeName(r *Route) any {
	if r == nil {
		return nil
	}
	return r.Method + " " + r.Path
}

func Test_Ctx_Endpoint_FollowsMethodOverride(t *testing.T) {
	t.Parallel()
	app := New()

	var before, after string
	app.Use(func(c Ctx) error {
		if r := c.Endpoint(); r != nil {
			before = r.Name
		}
		c.Request().Header.SetMethod(MethodPost)
		c.Req().Method(MethodPost)
		if r := c.Endpoint(); r != nil {
			after = r.Name
		}
		return c.Next()
	})
	app.Get("/x", func(c Ctx) error { return c.SendStatus(StatusOK) }).Name("x.get")
	app.Post("/x", func(c Ctx) error { return c.SendStatus(StatusOK) }).Name("x.post")

	apptest.Do(t, app, "/x")
	require.Equal(t, "x.get", before)
	require.Equal(t, "x.post", after, "Endpoint must follow a method override, not report the route the old method would have hit")
}

// A middleware placed behind an endpoint that already ran must be told the next
// endpoint, not the one the chain has passed. Both endpoints are parametric so
// that SkipUnmatchedRoutes resolves a firstMatchIndex at all: a static hit
// short-circuits resolveSkip before it records one.
func Test_Ctx_Endpoint_LooksPastTheCurrentRoute(t *testing.T) {
	t.Parallel()
	for _, skipUnmatched := range []bool{false, true} {
		t.Run(fmt.Sprintf("SkipUnmatchedRoutes=%v", skipUnmatched), func(t *testing.T) {
			t.Parallel()
			app := New(Config{SkipUnmatchedRoutes: skipUnmatched})

			var seen *Route
			app.Get("/:first", func(c Ctx) error { return c.Next() })
			app.Use(func(c Ctx) error {
				seen = c.Endpoint()
				return c.Next()
			})
			app.Get("/:second", func(c Ctx) error { return c.SendStatus(StatusOK) })

			apptest.Expect(t, app, "/x", StatusOK)
			require.NotNil(t, seen, "an endpoint is still ahead")
			require.Equal(t, "/:second", seen.Path, "the /:first endpoint already ran")
		})
	}
}

// A Domain route declines a request for another hostname and a later route takes
// it, so the look-ahead must not name a route the host filter turns down.
func Test_Ctx_Endpoint_DomainRoutes(t *testing.T) {
	t.Parallel()
	for _, skipUnmatched := range []bool{false, true} {
		t.Run(fmt.Sprintf("SkipUnmatchedRoutes=%v", skipUnmatched), func(t *testing.T) {
			t.Parallel()
			app := New(Config{SkipUnmatchedRoutes: skipUnmatched})

			app.Use(func(c Ctx) error {
				if route := c.Endpoint(); route != nil {
					c.Set("X-Predicted", route.Name)
				}
				err := c.Next()
				if route := c.Endpoint(); route != nil {
					c.Set("X-Reported", route.Name)
				}
				return err
			})
			served := func(c Ctx) error { return c.SendString(c.Route().Name) }

			www := app.Domain("www.example.com")
			admin := app.Domain("admin.example.com")

			www.Get("/dashboard", served).Name("www.dashboard")
			www.Get("/:page", served).Name("www.page")
			admin.Get("/dashboard", served).Name("admin.dashboard")
			admin.Get("/users", served).Name("admin.users")

			// Registered back to back, which is where addRoute merges one path
			admin.Get("/reports", served).Name("admin.reports")
			app.Domain("ops.example.com").Get("/reports", served).Name("ops.reports")
			app.Get("/reports", served).Name("reports")

			mounted := New()
			mounted.Get("/dashboard", served).Name("ops.dashboard")
			mounted.Domain("other.example.com").Get("/users", served).Name("ops.unreachable")
			app.Domain("ops.example.com").Use("/", mounted)

			app.Get("/users", served).Name("users")

			for _, tc := range []struct{ host, path, want string }{
				{host: "www.example.com", path: "/dashboard", want: "www.dashboard"},
				{host: "www.example.com", path: "/users", want: "www.page"},
				{host: "admin.example.com", path: "/dashboard", want: "admin.dashboard"},
				{host: "admin.example.com", path: "/users", want: "admin.users"},
				{host: "admin.example.com", path: "/reports", want: "admin.reports"},
				{host: "ops.example.com", path: "/reports", want: "ops.reports"},
				{host: "other.example.com", path: "/reports", want: "reports"},
				{host: "ops.example.com", path: "/dashboard", want: "ops.dashboard"},
				{host: "ops.example.com", path: "/users", want: "users"},
				{host: "other.example.com", path: "/users", want: "users"},
				{host: "other.example.com", path: "/dashboard"},
			} {
				for _, method := range []string{MethodGet, MethodHead} {
					t.Run(method+" "+tc.host+tc.path, func(t *testing.T) {
						t.Parallel()
						resp := apptest.Do(t, app, apptest.Req(method, tc.path, apptest.WithHost(tc.host)))
						body := apptest.Body(t, resp)

						if tc.want == "" {
							require.Equal(t, StatusNotFound, resp.StatusCode)
						} else {
							require.Equal(t, StatusOK, resp.StatusCode)
						}
						if tc.want != "" && method == MethodGet {
							require.Equal(t, tc.want, body)
						}
						require.Equal(t, tc.want, resp.Header.Get("X-Predicted"),
							"the look-ahead named another route than the one that ran")
						require.Equal(t, tc.want, resp.Header.Get("X-Reported"),
							"a route that declined was reported once the chain returned")
					})
				}
			}
		})
	}
}

func Test_Ctx_Endpoint_SkipUnmatchedRoutes(t *testing.T) {
	t.Parallel()
	app := New(Config{SkipUnmatchedRoutes: true})

	var path string
	app.Use(func(c Ctx) error {
		if route := c.Endpoint(); route != nil {
			path = route.Path
		}
		return c.Next()
	})
	app.Get("/items/:id", func(c Ctx) error { return c.SendStatus(StatusOK) })

	apptest.Expect(t, app, "/items/7", StatusOK)
	require.Equal(t, "/items/:id", path)
}

// Test_Ctx_Reset_DisablePathNormalizing covers the branch Reset takes when the
// request URI has DisablePathNormalizing set: the caller asked for the path to
// be treated as sent, so Reset scans the original instead of trusting the
// parsed copy. fasthttp parses with the flag cleared and only honors it when
// it writes a request line back out, so the two agree today; the scan is what
// keeps that an implementation detail of fasthttp rather than a dependency.
func Test_Ctx_Reset_DisablePathNormalizing(t *testing.T) {
	t.Parallel()
	app := New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	fctx.Request.SetRequestURI("/a/./b")

	// Parsing clears the flag, so set it afterwards, as a handler would.
	fctx.Request.URI().DisablePathNormalizing = true

	c := apptest.CtxFor(t, app, fctx)

	require.Equal(t, "/a/./b", c.OriginalURL(), "the target is untouched")
	require.Equal(t, "/a/b", c.Path(), "the router still sees the normalized path")
}

// Test_Ctx_Path_DisablePathNormalizing pins that a Path override is normalized
// before routing even with fasthttp's normalization switched off on the URI.
func Test_Ctx_Path_DisablePathNormalizing(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/a/b", func(c Ctx) error { return c.SendString(c.Path()) })
	for _, tc := range []struct{ mount, override string }{
		{mount: "/dotted", override: "/a/./b"},
		{mount: "/plain", override: "/a/b"},
	} {
		app.Use(tc.mount, func(c Ctx) error {
			c.Request().URI().DisablePathNormalizing = true
			c.Path(tc.override)
			return c.RestartRouting()
		})
	}

	for _, target := range []string{"/dotted", "/plain"} {
		apptest.ExpectBody(t, app, target, StatusOK, "/a/b")
	}
}

func Test_Ctx_RouteNormalized(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c Ctx) error {
		require.Equal(t, "/test", c.Route().Path)
		return nil
	})
	apptest.Expect(t, app, "//test", StatusNotFound)
}

// ctxTestBHelloWorldUpload returns a POST /test request whose multipart body
// uploads "hello world" as the form file "file", named "test".
func ctxTestBHelloWorldUpload(t *testing.T) *http.Request {
	t.Helper()

	body, contentType := ctxTestMultipartFile(t, "file", "test", []byte("hello world"))
	req := httptest.NewRequest(MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return req
}

func Test_Ctx_SaveFile(t *testing.T) {
	// TODO We should clean this up
	t.Parallel()
	app := New()

	app.Post("/test", func(c Ctx) error {
		fh, err := c.Req().FormFile("file")
		require.NoError(t, err)

		tempFile, err := os.CreateTemp(os.TempDir(), "test-")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, tempFile.Close())
			require.NoError(t, os.Remove(tempFile.Name()))
		}()
		require.NoError(t, c.SaveFile(fh, tempFile.Name()))

		bs, err := os.ReadFile(tempFile.Name())
		require.NoError(t, err)
		require.Equal(t, "hello world", string(bs))
		return nil
	})

	apptest.Expect(t, app, ctxTestBHelloWorldUpload(t), StatusOK)
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

	t.Cleanup(func() { require.NoError(t, form.RemoveAll()) })

	files := form.File["file"]
	require.Len(t, files, 1)

	return files[0]
}

func Test_Ctx_SaveFileToStorage(t *testing.T) {
	t.Parallel()
	app := New()
	storage := memory.New()

	app.Post("/test", func(c Ctx) error {
		fh, err := c.FormFile("file")
		require.NoError(t, err)
		require.NoError(t, c.SaveFileToStorage(fh, "test", storage))

		file, err := storage.Get("test")
		require.Equal(t, []byte("hello world"), file)
		require.NoError(t, err)
		require.NoError(t, storage.Delete("test"))
		return nil
	})

	apptest.Expect(t, app, ctxTestBHelloWorldUpload(t), StatusOK)
}

func Test_Ctx_SaveFileToStorage_Stores(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		filename  string
		payload   []byte
		bodyLimit int
		zeroLimit bool // bypass the app default coercion of BodyLimit to exercise the fallback branch
	}{
		{name: "large upload", filename: "large.bin", payload: bytes.Repeat([]byte{'a'}, 5*1024*1024), bodyLimit: 8 * 1024 * 1024},
		{name: "default body limit fallback", filename: "small.txt", payload: []byte("hello"), zeroLimit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New(Config{BodyLimit: tc.bodyLimit})
			if tc.zeroLimit {
				app.config.BodyLimit = 0
			}
			storage := memory.New()
			ctx := apptest.Ctx(t, app)

			require.NoError(t, ctx.SaveFileToStorage(createMultipartFileHeader(t, tc.filename, tc.payload), "key", storage))

			stored, err := storage.Get("key")
			require.NoError(t, err)
			require.Equal(t, tc.payload, stored)
		})
	}
}

func Test_Ctx_SaveFileToStorage_LimitExceeded(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		filename    string
		bodyLimit   int
		fileSize    int
		unknownSize bool // the header does not announce the size, so the limit is enforced while reading
	}{
		{name: "known size", filename: "too-large.bin", bodyLimit: 1024, fileSize: 1024 + 512},
		{name: "unknown size", filename: "unknown-size.bin", bodyLimit: 1024, fileSize: 1024 + 256, unknownSize: true},
		// a small limit to force the error, which must name the file
		{name: "error names the file", filename: "test-file.png", bodyLimit: 10, fileSize: 100},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := apptest.Ctx(t, New(Config{BodyLimit: tc.bodyLimit}))
			storage := memory.New()

			fileHeader := createMultipartFileHeader(t, tc.filename, bytes.Repeat([]byte{'a'}, tc.fileSize))
			if tc.unknownSize {
				fileHeader.Size = -1
			}

			err := ctx.SaveFileToStorage(fileHeader, "test-path", storage)

			require.Error(t, err)
			require.ErrorIs(t, err, ErrFileRead)
			require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
			require.Contains(t, err.Error(), tc.filename)
		})
	}
}

// ctxStrictStorage is a Storage that fails the test on every call a
// SaveFileToStorage test does not expect; the fakes below embed it.
// conventions:ignore storage-fake: it fails the test on every call, which storagetest.Memory cannot do
type ctxStrictStorage struct {
	t *testing.T
}

func (s *ctxStrictStorage) helperFailure(msg string, args ...any) {
	s.t.Helper()
	s.t.Fatalf(msg, args...)
}

func (s *ctxStrictStorage) GetWithContext(context.Context, string) ([]byte, error) {
	s.helperFailure("unexpected call to GetWithContext")
	return nil, nil
}

func (s *ctxStrictStorage) Get(string) ([]byte, error) {
	s.helperFailure("unexpected call to Get")
	return nil, nil
}

func (s *ctxStrictStorage) Set(key string, _ []byte, _ time.Duration) error {
	s.helperFailure("unexpected call to Set for key %q", key)
	return nil
}

func (s *ctxStrictStorage) DeleteWithContext(context.Context, string) error {
	s.helperFailure("unexpected call to DeleteWithContext")
	return nil
}

func (s *ctxStrictStorage) Delete(string) error {
	s.helperFailure("unexpected call to Delete")
	return nil
}

func (s *ctxStrictStorage) ResetWithContext(context.Context) error {
	s.helperFailure("unexpected call to ResetWithContext")
	return nil
}

func (s *ctxStrictStorage) Reset() error {
	s.helperFailure("unexpected call to Reset")
	return nil
}

func (*ctxStrictStorage) Close() error { return nil }

type captureStorage struct {
	data map[string][]byte
	ctxStrictStorage
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

func (s *captureStorage) SetWithContext(_ context.Context, key string, val []byte, _ time.Duration) error {
	s.ensureStore(key, val)
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
	storage := &captureStorage{ctxStrictStorage: ctxStrictStorage{t: t}}
	ctx := apptest.Ctx(t, app)

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
	key           any
	expectedValue any
	validateCtx   func(context.Context)
	cancel        context.CancelFunc
	ctxStrictStorage
	ctxMatched     atomic.Bool
	cancelObserved atomic.Bool
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

func Test_Ctx_Save_NilFileHeader(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		save func(c Ctx) error
		name string
	}{
		{name: "SaveFileToStorage", save: func(c Ctx) error { return c.SaveFileToStorage(nil, "test", memory.New()) }},
		{name: "SaveFile", save: func(c Ctx) error { return c.SaveFile(nil, "test") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.save(apptest.Ctx(t, New()))
			require.Error(t, err)
			require.ErrorIs(t, err, ErrFileHeaderNil)
		})
	}
}

func Test_Ctx_SaveFileToStorage_StoreErrorIncludesPath(t *testing.T) {
	t.Parallel()
	app := New()
	wantErr := errors.New("backend down")
	storage := &failingStorage{ctxStrictStorage: ctxStrictStorage{t: t}, err: wantErr}
	ctx := apptest.Ctx(t, app)
	fileHeader := createMultipartFileHeader(t, "report.pdf", []byte("payload"))

	err := ctx.SaveFileToStorage(fileHeader, "uploads/report.pdf", storage)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrFileStore)
	require.ErrorIs(t, err, wantErr)
	require.Contains(t, err.Error(), "report.pdf")
	require.Contains(t, err.Error(), "uploads/report.pdf")
}

// failingStorage fails every store with err.
type failingStorage struct {
	err error
	ctxStrictStorage
}

func (s *failingStorage) Set(string, []byte, time.Duration) error { return s.err }

func (s *failingStorage) SetWithContext(context.Context, string, []byte, time.Duration) error {
	return s.err
}

func Test_Ctx_SaveFileToStorage_ContextPropagation(t *testing.T) {
	t.Parallel()
	type ctxKeyType string

	const ctxKey ctxKeyType = "storage-context-key"

	storage := &mockContextAwareStorage{
		ctxStrictStorage: ctxStrictStorage{t: t}, key: ctxKey, expectedValue: "expected-context-value",
	}
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
		require.NoError(t, c.SaveFileToStorage(fh, "test", storage))
		require.True(t, storage.ctxMatched.Load(), "storage should receive the context installed on Ctx")
		require.True(t, storage.cancelObserved.Load(), "storage should observe context cancellation")
		return nil
	})

	apptest.Expect(t, app, ctxTestBHelloWorldUpload(t), StatusOK)
}

func Test_Ctx_Secure(t *testing.T) {
	t.Parallel()
	// the peer 0.0.0.0 announces HTTPS through X-Forwarded-Proto
	announcesHTTPS := func(freq *fasthttp.RequestCtx) {
		freq.SetRemoteAddr(net.Addr(&net.TCPAddr{IP: net.ParseIP("0.0.0.0")}))
		freq.Request.Header.Set(HeaderXForwardedProto, schemeHTTPS)
	}

	tests := []struct {
		app       *App
		configure func(*fasthttp.RequestCtx)
		name      string
		expected  bool
	}{
		{name: "plain HTTP", app: New(), expected: false},
		{
			name: "direct TLS", app: New(), expected: true,
			configure: func(freq *fasthttp.RequestCtx) { freq.Init2(&tls.Conn{}, nil, true) },
		},
		{
			name: "trusted proxy HTTPS scheme", configure: announcesHTTPS, expected: true,
			app: New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{"0.0.0.0"}}}),
		},
		{
			name: "untrusted proxy HTTPS scheme", configure: announcesHTTPS, expected: false,
			app: New(Config{TrustProxy: true, TrustProxyConfig: TrustProxyConfig{Proxies: []string{"1.1.1.1"}}}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			freq := &fasthttp.RequestCtx{}
			if tt.configure != nil {
				tt.configure(freq)
			}
			c := apptest.CtxFor(t, tt.app, freq)

			require.Equal(t, tt.expected, c.Secure())
		})
	}
}

func Test_Ctx_Stale(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.True(t, c.Stale())
}

func Test_Ctx_Subdomains(t *testing.T) {
	t.Parallel()
	app := New()

	for _, tc := range []struct {
		name   string
		host   string
		offset []int // nil ⇒ call without argument
		want   []string
	}{
		{
			name: "default offset (2) drops registrable domain + TLD", host: "john.doe.is.awesome.google.com",
			want: []string{"john", "doe", "is", "awesome"},
		},
		{
			name: "custom offset trims N right-hand labels", host: "john.doe.is.awesome.google.com",
			offset: []int{4}, want: []string{"john", "doe"},
		},
		{
			name: "offset too high returns empty", host: "john.doe.is.awesome.google.com",
			offset: []int{10}, want: []string{},
		},
		{
			name: "zero offset returns all labels", host: "john.doe.google.com",
			offset: []int{0}, want: []string{"john", "doe", "google", "com"},
		},
		{
			name: "offset 1 keeps registrable domain", host: "john.doe.google.com",
			offset: []int{1}, want: []string{"john", "doe", "google"},
		},
		{name: "negative offset returns empty", host: "john.doe.google.com", offset: []int{-1}, want: []string{}},
		{name: "offset equal len returns empty", host: "john.doe.com", offset: []int{3}, want: []string{}},
		{
			name: "zero offset returns all labels with port present", host: "localhost:3000",
			offset: []int{0}, want: []string{"localhost"},
		},
		{
			name: "host with port — custom offset trims 2 labels", host: "foo.bar.example.com:8080",
			offset: []int{2}, want: []string{"foo", "bar"},
		},
		{
			name: "fully qualified domain trims trailing dot", host: "john.doe.example.com.",
			want: []string{"john", "doe"},
		},
		{name: "punycode domain is decoded", host: "xn--bcher-kva.example.com", want: []string{"bücher"}},
		{name: "punycode fqdn is decoded", host: "xn--bcher-kva.example.com.", want: []string{"bücher"}},
		{name: "punycode decode failure uses fallback", host: "xn--bcher--.example.com", want: []string{"xn--bcher--"}},
		{name: "invalid host keeps original lowercased", host: "Foo Bar", offset: []int{0}, want: []string{"foo bar"}},
		{name: "IPv4 host returns empty", host: "192.168.0.1", want: []string{}},
		{name: "IPv6 host returns empty", host: "[2001:db8::1]", want: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)

			c.Request().URI().SetHost(tc.host)
			require.Equal(t, tc.want, c.Subdomains(tc.offset...))
		})
	}
}

func Benchmark_Ctx_Subdomains(b *testing.B) {
	c := apptest.Ctx(b, New())

	c.Request().SetRequestURI("http://john.doe.google.com")
	var res []string
	b.ReportAllocs()
	for b.Loop() {
		res = c.Subdomains()
	}
	require.Equal(b, []string{"john", "doe"}, res)
}

func Test_Ctx_ClearCookie(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().Header.Set(HeaderCookie, "john=doe")
	c.Res().ClearCookie("john")
	require.True(t, strings.HasPrefix(string(c.Response().Header.Peek(HeaderSetCookie)), "john=; expires="))

	c.Request().Header.Set(HeaderCookie, "test1=dummy")
	c.Request().Header.Set(HeaderCookie, "test2=dummy")
	c.ClearCookie()
	require.Contains(t, string(c.Response().Header.Peek(HeaderSetCookie)), "test1=; expires=")
	require.Contains(t, string(c.Response().Header.Peek(HeaderSetCookie)), "test2=; expires=")
}

// ctxTestBCtxGoSource returns the content of ctx.go, the file the SendFile and
// Download tests serve and compare against.
func ctxTestBCtxGoSource(t *testing.T) []byte {
	t.Helper()

	f, err := os.Open("./ctx.go")
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	content, err := io.ReadAll(f)
	require.NoError(t, err)
	return content
}

func Test_Ctx_Download(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.NoError(t, c.Download("ctx.go", "Awesome File!"))

	expect := ctxTestBCtxGoSource(t)
	require.Equal(t, expect, c.Response().Body())
	require.Equal(t, `attachment; filename="Awesome File!"`, string(c.Response().Header.Peek(HeaderContentDisposition)))

	require.NoError(t, c.Res().Download("ctx.go"))
	require.Equal(t, `attachment; filename="ctx.go"`, string(c.Response().Header.Peek(HeaderContentDisposition)))

	require.NoError(t, c.Download("ctx.go", "файл.txt"))
	header := string(c.Response().Header.Peek(HeaderContentDisposition))
	require.Contains(t, header, `filename="файл.txt"`)
	require.Contains(t, header, `filename*=UTF-8''%D1%84%D0%B0%D0%B9%D0%BB.txt`)
}

func Test_Ctx_SendEarlyHints(t *testing.T) {
	t.Parallel()
	app := New()

	hints := []string{"<https://cdn.com>; rel=preload; as=script"}
	app.Get("/earlyhints", func(c Ctx) error {
		require.NoError(t, c.SendEarlyHints(hints), "SendEarlyHints")
		c.Status(StatusBadRequest)
		return c.SendString("fail")
	})

	resp := apptest.Expect(t, app, "/earlyhints", StatusBadRequest)
	require.Equal(t, hints, resp.Header["Link"], "Link header")
	require.Equal(t, "fail", apptest.Body(t, resp))
}

// Test_Ctx_SendEarlyHints_HTTP10 verifies that no interim 103 response is
// sent to an HTTP/1.0 client (RFC 9110 §15.2 MUST NOT), while the Link
// headers still go out on the final response.
func Test_Ctx_SendEarlyHints_HTTP10(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())
	c.Request().Header.SetProtocol("HTTP/1.0")

	// Without the protocol guard this would reach fasthttp's EarlyHints,
	// which writes the interim response directly to the connection (and
	// panics here, where no connection exists).
	require.NoError(t, c.SendEarlyHints([]string{"<https://cdn.com/style.css>; rel=preload; as=style"}))
	require.Equal(t, "<https://cdn.com/style.css>; rel=preload; as=style", c.GetRespHeader(HeaderLink))
}

func Test_Ctx_SendFile(t *testing.T) {
	t.Parallel()
	app := New()

	expectFileContent := ctxTestBCtxGoSource(t)
	// fetch file info for the not modified test case
	fI, err := os.Stat("./ctx.go")
	require.NoError(t, err)

	for _, tc := range []struct {
		name   string
		header string // response header the config must set, if any
		want   string
		config SendFile
	}{
		{name: "simple test case"},
		{name: "download", config: SendFile{Download: true}, header: HeaderContentDisposition, want: "attachment"},
		{name: "max age", config: SendFile{MaxAge: 100}, header: HeaderCacheControl, want: "public, max-age=100"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)

			require.NoError(t, c.SendFile("ctx.go", tc.config))
			require.Equal(t, expectFileContent, c.Response().Body())
			if tc.header != "" {
				require.Equal(t, tc.want, string(c.Response().Header.Peek(tc.header)), tc.header)
			}
			require.Equal(t, StatusOK, c.Response().StatusCode())
		})
	}

	t.Run("custom error code", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, app)

		require.NoError(t, c.Res().Status(StatusInternalServerError).SendFile("ctx.go"))
		require.Equal(t, expectFileContent, c.Response().Body())
		require.Equal(t, StatusInternalServerError, c.Response().StatusCode())
	})

	t.Run("not modified", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, app)
		c.Request().Header.Set(HeaderIfModifiedSince, fI.ModTime().UTC().Format(http.TimeFormat))

		require.NoError(t, c.SendFile("ctx.go"))
		require.Equal(t, StatusNotModified, c.Response().StatusCode())
		require.Equal(t, []byte(nil), c.Response().Body())
	})
}

func Test_Ctx_SendFile_ContentType(t *testing.T) {
	t.Parallel()
	app := New()

	for _, tc := range []struct {
		name        string
		file        string
		contentType string
		viaRes      bool // send through c.Res() instead of c
	}{
		{name: "simple case", file: "./.github/testdata/fs/img/fiber.png", contentType: "image/png", viaRes: true},
		// see: https://github.com/valyala/fasthttp/blob/d795f13985f16622a949ea9fc3459cf54dc78b3e/fs.go#L1638
		{name: "set by valid file extension, not file header", file: "./.github/testdata/fs/img/fiberpng.jpeg", contentType: "image/jpeg"},
		{name: "set by file header if extension is invalid", file: "./.github/testdata/fs/img/fiberpng.notvalidext", contentType: "image/png"},
		{name: "set by file header if extension is missing", file: "./.github/testdata/fs/img/fiberpng", contentType: "image/png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)

			var err error
			if tc.viaRes {
				err = c.Res().SendFile(tc.file)
			} else {
				err = c.SendFile(tc.file)
			}
			require.NoError(t, err)
			require.Equal(t, StatusOK, c.Response().StatusCode())
			require.Equal(t, tc.contentType, string(c.Response().Header.Peek(HeaderContentType)))
		})
	}
}

func Test_Static_Compress(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/file", func(c Ctx) error { return c.SendFile("ctx.go", SendFile{Compress: true}) })

	// Note: deflate is not supported by fasthttp.FS
	algorithms := []string{"zstd", "gzip", "br"}
	for _, algo := range algorithms {
		t.Run(algo+"_compression", func(t *testing.T) {
			t.Parallel()
			req := apptest.Req(MethodGet, "/file", apptest.WithHeader("Accept-Encoding", algo))
			resp := apptest.Do(t, app, req, TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
			require.Equal(t, 200, resp.StatusCode, "Status code")
			require.NotEqual(t, "58726", resp.Header.Get(HeaderContentLength))
		})
	}
}

func Test_Ctx_SendFile_Compress_CheckCompressed(t *testing.T) {
	t.Parallel()
	app := New()

	expectedFileContent := ctxTestBCtxGoSource(t)

	sendFileBodyReader := func(compression string) ([]byte, error) {
		t.Helper()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.Request().Header.Add(HeaderAcceptEncoding, compression)
		err := c.SendFile("./ctx.go", SendFile{Compress: true})
		return c.Response().Body(), err
	}

	for _, tc := range []struct {
		decode   func(dst, src []byte) ([]byte, error)
		encoding string
	}{
		{encoding: "gzip", decode: fasthttp.AppendGunzipBytes},
		{encoding: "zstd", decode: fasthttp.AppendUnzstdBytes},
		{encoding: "br", decode: fasthttp.AppendUnbrotliBytes},
	} {
		t.Run(tc.encoding, func(t *testing.T) {
			t.Parallel()
			b, err := sendFileBodyReader(tc.encoding)
			require.NoError(t, err)
			body, err := tc.decode(nil, b)
			require.NoError(t, err)
			require.Equal(t, expectedFileContent, body)
		})
	}
}

//go:embed ctx.go
var embedFile embed.FS

func Test_Ctx_SendFile_EmbedFS(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c Ctx) error { return c.SendFile("ctx.go", SendFile{FS: embedFile}) })
	apptest.ExpectBody(t, app, "/test", StatusOK, string(ctxTestBCtxGoSource(t)))
}

func Test_Ctx_SendFile_404(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error { return c.SendFile("ctx12.go") })

	apptest.ExpectBody(t, app, "/", StatusNotFound, "sendfile: file ctx12.go not found")
}

// Test_Ctx_SendFile_NameIsLiteral pins that SendFile opens exactly the name it
// is given, the way os.Open would. The file server reads the request URI,
// which fasthttp decodes and splits at "?" and "#", so the name is escaped on
// its way in: a "%", "?" or "#" is part of the name, and "100%25.txt" is not
// "100%.txt".
func Test_Ctx_SendFile_NameIsLiteral(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	names := []string{"100%25.txt", "100%.txt", "hash_sign_#.txt", "sp ace.txt", "café.txt", "plus+sign.txt"}
	if runtime.GOOS != windowsOS {
		names = append(names, "question?.txt")
	}
	for _, name := range names {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("content of "+name), 0o600))
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Get("/", func(c Ctx) error { return c.SendFile(filepath.Join(dir, name)) })
			apptest.ExpectBody(t, app, "/", StatusOK, "content of "+name)
		})
	}
}

// Test_Ctx_SendFile_EncodedSeparatorStaysInDirectory pins that a name built
// from request input cannot leave the directory the handler joined it to. The
// router keeps "%2F" encoded, so c.Params("*") of "/files/..%2Fsecret.txt" is
// one segment that filepath.Join cannot clean; SendFile used to decode it
// afterwards into "../secret.txt" and serve the file outside the directory.
// An encoded "?" or "#" likewise stays part of the name.
func Test_Ctx_SendFile_EncodedSeparatorStaysInDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pub := filepath.Join(root, "public")
	require.NoError(t, os.Mkdir(pub, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(pub, "ok.txt"), []byte("OK"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "secret.txt"), []byte("SECRET"), 0o600))

	for _, unescape := range []bool{false, true} {
		app := New(Config{UnescapePath: unescape})
		app.Get("/files/*", func(c Ctx) error { return c.SendFile(filepath.Join(pub, c.Params("*"))) })

		resp := apptest.Do(t, app, "/files/ok.txt")
		require.Equal(t, StatusOK, resp.StatusCode, "unescape=%v", unescape)

		for _, target := range []string{
			"/files/..%2Fsecret.txt",
			"/files/%2e%2e%2Fsecret.txt",
			"/files/..%252Fsecret.txt",
			"/files/ok.txt%3Fx",
			"/files/ok.txt%23x",
		} {
			resp := apptest.Do(t, app, target)
			require.Equal(t, StatusNotFound, resp.StatusCode, "unescape=%v target=%s", unescape, target)
			require.NotContains(t, apptest.Body(t, resp), "SECRET", "unescape=%v target=%s", unescape, target)
		}
	}
}

// Test_Ctx_SendFile_DirectoryIsNotFound pins that a directory named without a
// trailing slash is not found rather than redirected: fasthttp's redirect
// would carry the directory's filesystem path in Location.
func Test_Ctx_SendFile_DirectoryIsNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// fasthttp escapes the name in the redirect it answers with, so a name it
	// has to escape (a space, a percent, UTF-8) must be caught all the same.
	for _, name := range []string{"sub", "sub dir", "100%", "café"} {
		require.NoError(t, os.Mkdir(filepath.Join(dir, name), 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name, "index.html"), []byte("INDEX"), 0o600))

		app := New()
		app.Get("/", func(c Ctx) error { return c.SendFile(filepath.Join(dir, name)) })
		resp := apptest.Do(t, app, "/")
		require.Equal(t, StatusNotFound, resp.StatusCode, name)
		require.Empty(t, resp.Header.Get(HeaderLocation), name)
		body := apptest.Body(t, resp)
		require.NotContains(t, body, "INDEX", name)
		// the error the default handler writes names neither the directory
		// nor where it lives
		require.NotContains(t, body, filepath.ToSlash(dir), name)
		require.NotContains(t, body, name, name)
	}
}

// Test_Ctx_SendFile_DecodedParamWithRoot pins the recipe the SendFile docs give
// for a name taken from a route parameter under the default configuration,
// where c.Params keeps the escapes the router keeps: decode the parameter and
// serve it through the FS option from an os.Root, which refuses a name that
// leaves the directory once decoded.
func Test_Ctx_SendFile_DecodedParamWithRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := filepath.Join(dir, "files")
	require.NoError(t, os.Mkdir(files, 0o750))
	for _, name := range []string{"my file.txt", "café.txt", "100%.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(files, name), []byte("FILE "+name), 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("SECRET"), 0o600))
	root, err := os.OpenRoot(files)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	app := New()
	app.Get("/files/:name", func(c Ctx) error {
		name, err := url.PathUnescape(c.Params("name"))
		if err != nil {
			return ErrBadRequest
		}
		return c.SendFile(name, SendFile{FS: root.FS()})
	})

	for target, want := range map[string]string{
		"/files/my%20file.txt":   "FILE my file.txt",
		"/files/caf%C3%A9.txt":   "FILE café.txt",
		"/files/100%25.txt":      "FILE 100%.txt",
		"/files/..%2Fsecret.txt": "",
	} {
		resp := apptest.Do(t, app, target)
		body := apptest.Body(t, resp)
		if want == "" {
			require.Equal(t, StatusNotFound, resp.StatusCode, target)
			require.NotContains(t, body, "SECRET", target)
			continue
		}
		require.Equal(t, StatusOK, resp.StatusCode, target)
		require.Equal(t, want, body, target)
	}
}

// Test_Ctx_SendFile_ControlByteIsNotFound pins that a name holding a control
// byte is not found. fasthttp's URI parser used to reject it silently, which
// left the root path, so the root's index.html was served instead.
func Test_Ctx_SendFile_ControlByteIsNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("INDEX"), 0o600))

	for _, cfg := range []SendFile{{}, {FS: os.DirFS(dir)}} {
		app := New()
		app.Get("/", func(c Ctx) error {
			name := "nope\x00.txt"
			if cfg.FS == nil {
				name = filepath.Join(dir, name)
			}
			return c.SendFile(name, cfg)
		})
		resp := apptest.Do(t, app, "/")
		require.Equal(t, StatusNotFound, resp.StatusCode, "fs=%v", cfg.FS != nil)
		require.NotContains(t, apptest.Body(t, resp), "INDEX", "fs=%v", cfg.FS != nil)
	}
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
			return c.SendFile("ctx.go", SendFile{Download: true})
		case "4":
			return c.SendFile("app_test.go", SendFile{FS: os.DirFS(".")})
		default:
			return c.SendStatus(StatusNotFound)
		}
	})
	app.Get("/test2", func(c Ctx) error { return c.SendFile("ctx.go", SendFile{Download: true}) })

	for _, tc := range []struct {
		url                string
		body               string
		contentDisposition string
	}{
		{url: "/test?file=1", body: "type DefaultCtx struct"},
		{url: "/test?file=2", body: "type App struct"},
		{url: "/test?file=3", body: "type DefaultCtx struct", contentDisposition: "attachment"},
		{url: "/test?file=4", body: "Test_App_MethodNotAllowed"},
		// twice: the repeated request reuses the file handler the first one created
		{url: "/test2", body: "type DefaultCtx struct", contentDisposition: "attachment"},
		{url: "/test2", body: "type DefaultCtx struct", contentDisposition: "attachment"},
	} {
		resp := apptest.Expect(t, app, tc.url, StatusOK)
		require.Equal(t, tc.contentDisposition, resp.Header.Get(HeaderContentDisposition))
		require.Contains(t, apptest.Body(t, resp), tc.body)
	}

	require.Len(t, sendFileCache(app), 3)
}

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

	addEndpoint("./.github/index.html", "/relativeWithDot")
	addEndpoint(filepath.FromSlash("./.github/index.html"), "/relativeOSWithDot")
	addEndpoint(".github/index.html", "/relative")
	addEndpoint(filepath.FromSlash(".github/index.html"), "/relativeOS")
	if path, err := filepath.Abs(".github/index.html"); err != nil {
		require.NoError(t, err)
	} else {
		addEndpoint(path, "/absolute")
		addEndpoint(filepath.FromSlash(path), "/absoluteOS") // os related
	}

	for _, endpoint := range endpointsForTest {
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()
			apptest.Expect(t, app, endpoint, StatusOK)
			apptest.Expect(t, app, endpoint, StatusOK)
		})
	}
}

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
	app.Get("/file", func(c Ctx) error { return c.SendFile("ctx.go") })
	app.Get("/file/download", func(c Ctx) error { return c.SendFile("ctx.go", SendFile{Download: true}) })
	app.Get("/file/fs", func(c Ctx) error { return c.SendFile("ctx.go", SendFile{FS: os.DirFS(".")}) })

	apptest.Expect(t, app, "/file", StatusOK)
	resp := apptest.Expect(t, app, "/file/download", StatusOK)
	require.Equal(t, "attachment", resp.Header.Get(HeaderContentDisposition))
	apptest.Expect(t, app, "/file/fs", StatusOK)
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
	app.Get("/range", func(c Ctx) error { return c.SendFile(fixture, SendFile{ByteRange: true}) })
	app.Get("/norange", func(c Ctx) error { return c.SendFile(fixture) })

	// get requests target with the given Range header and requires the given status.
	get := func(t *testing.T, target, rangeHeader string, status int) *http.Response {
		t.Helper()
		req := apptest.Req(MethodGet, target, apptest.WithHeader(HeaderRange, rangeHeader))
		return apptest.Expect(t, app, req, status)
	}

	for _, tc := range []struct {
		name         string
		rangeHeader  string
		contentRange string
		body         []byte
	}{
		{name: "satisfiable single range", rangeHeader: "bytes=0-4", contentRange: "bytes 0-4/10", body: content[:5]},
		{name: "single byte range", rangeHeader: "bytes=4-4", contentRange: "bytes 4-4/10", body: content[4:5]},
		{name: "open ended range", rangeHeader: "bytes=4-", contentRange: "bytes 4-9/10", body: content[4:]},
		{name: "range exceeding end", rangeHeader: "bytes=5-20", contentRange: "bytes 5-9/10", body: content[5:]},
		{name: "suffix range", rangeHeader: "bytes=-3", contentRange: "bytes 7-9/10", body: content[len(content)-3:]},
		{name: "suffix range exceeding size", rangeHeader: "bytes=-20", contentRange: "bytes 0-9/10", body: content},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := get(t, "/range", tc.rangeHeader, StatusPartialContent)
			require.Equal(t, "bytes", resp.Header.Get(HeaderAcceptRanges))
			require.Equal(t, tc.contentRange, resp.Header.Get(HeaderContentRange))
			require.EqualValues(t, len(tc.body), resp.ContentLength)
			require.Equal(t, tc.body, apptest.BodyBytes(t, resp))
		})
	}

	for _, tc := range []struct {
		name        string
		rangeHeader string
	}{
		{name: "unsatisfiable range", rangeHeader: "bytes=1000-2000"},
		{name: "unsatisfiable reversed range", rangeHeader: "bytes=6-5"},
		{name: "unsatisfiable start past end", rangeHeader: "bytes=10-"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := get(t, "/range", tc.rangeHeader, StatusRequestedRangeNotSatisfiable)
			require.Equal(t, "bytes */10", resp.Header.Get(HeaderContentRange))
		})
	}

	t.Run("range ignored when byte range disabled", func(t *testing.T) {
		resp := get(t, "/norange", "bytes=0-4", StatusOK)
		require.Empty(t, resp.Header.Get(HeaderAcceptRanges))
		require.Empty(t, resp.Header.Get(HeaderContentRange))
		require.EqualValues(t, len(content), resp.ContentLength)
		require.Equal(t, content, apptest.BodyBytes(t, resp))
	})
}

const (
	byteRangeContent      = "0123456789abcdefghij"
	byteRangeLastModified = "Thu, 02 Jan 2020 03:04:05 GMT"
	byteRangeStale        = "Wed, 01 Jan 2020 03:04:05 GMT"
)

// byteRangeFixture writes the 20-byte file of the byte-range tests, dated to
// byteRangeLastModified, and returns its path.
func byteRangeFixture(t *testing.T) string {
	t.Helper()

	fixture := filepath.Join(t.TempDir(), "fixture.txt")
	require.NoError(t, os.WriteFile(fixture, []byte(byteRangeContent), 0o600))
	require.NoError(t, os.Chtimes(fixture, time.Time{}, time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)))
	return fixture
}

// SendFile wires the Range and If-Range rules (RFC 9110 §13.1.5, §14.2) into the
// file server; internal/byterange tests them in full.
func Test_SendFile_ByteRange_RFC9110(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == windowsOS {
		t.Skip("SendFile byte-range tests are flaky on Windows")
	}

	fixture := byteRangeFixture(t)
	app := New()
	app.Get("/file", func(c Ctx) error {
		c.Set("X-Request", "kept")
		return c.SendFile(fixture, SendFile{ByteRange: true})
	})

	tests := []struct {
		fields     map[string]string
		name       string
		wantRange  string
		wantBody   string
		wantStatus int
		wantWhole  bool // the whole file, with its validators
	}{
		{name: "a range", fields: map[string]string{HeaderRange: "bytes=0-4"}, wantStatus: StatusPartialContent, wantBody: "01234", wantRange: "bytes 0-4/20"},
		{name: "several ranges", fields: map[string]string{HeaderRange: "bytes=0-1,3-4"}, wantStatus: StatusOK, wantBody: byteRangeContent, wantWhole: true},
		{name: "If-Range matches", fields: map[string]string{HeaderRange: "bytes=0-4", HeaderIfRange: byteRangeLastModified}, wantStatus: StatusPartialContent, wantBody: "01234", wantRange: "bytes 0-4/20"},
		{name: "If-Range is stale", fields: map[string]string{HeaderRange: "bytes=0-4", HeaderIfRange: byteRangeStale}, wantStatus: StatusOK, wantBody: byteRangeContent, wantWhole: true},
		{name: "unsatisfiable", fields: map[string]string{HeaderRange: "bytes=100-200"}, wantStatus: StatusRequestedRangeNotSatisfiable, wantRange: "bytes */20", wantBody: "Range Not Satisfiable"},
		{name: "unsatisfiable, If-Range is stale", fields: map[string]string{HeaderRange: "bytes=100-200", HeaderIfRange: byteRangeStale}, wantStatus: StatusOK, wantBody: byteRangeContent, wantWhole: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(MethodGet, "/file", http.NoBody)
			for k, v := range tc.fields {
				req.Header.Set(k, v)
			}
			resp := apptest.Expect(t, app, req, tc.wantStatus)
			require.Equal(t, tc.wantRange, resp.Header.Get(HeaderContentRange))
			require.Equal(t, tc.wantBody, apptest.Body(t, resp))

			if tc.wantWhole {
				require.Equal(t, "20", resp.Header.Get(HeaderContentLength))
				require.Equal(t, byteRangeLastModified, resp.Header.Get(HeaderLastModified))
			}
			// Headers set before SendFile survive unless the response is discarded.
			if tc.wantStatus != StatusRequestedRangeNotSatisfiable {
				require.Equal(t, "kept", resp.Header.Get("X-Request"))
			}
		})
	}
}

// A 416 carries the size of the file the file server opened, also when the
// name holds dot segments that fasthttp's path normalization resolved.
func Test_SendFile_ByteRange_DotSegments(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == windowsOS {
		t.Skip("SendFile byte-range tests are flaky on Windows")
	}

	fixture := byteRangeFixture(t)
	dir := filepath.Dir(fixture)
	app := New()
	app.Get("/os", func(c Ctx) error { return c.SendFile(dir+"/missing/../fixture.txt", SendFile{ByteRange: true}) })
	app.Get("/fs", func(c Ctx) error {
		return c.SendFile("a/../../fixture.txt", SendFile{ByteRange: true, FS: os.DirFS(dir)})
	})

	for _, target := range []string{"/os", "/fs"} {
		resp := apptest.Expect(t, app, apptest.Req(MethodGet, target, apptest.WithHeader(HeaderRange, "bytes=100-200")), StatusRequestedRangeNotSatisfiable)
		require.Equal(t, "bytes */20", resp.Header.Get(HeaderContentRange), target)

		resp = apptest.Expect(t, app, apptest.Req(MethodGet, target, apptest.WithHeader(HeaderRange, "bytes=0-4")), StatusPartialContent)
		require.Equal(t, "01234", apptest.Body(t, resp), target)
	}
}

// A directory is served by its index.html, so a 416 carries that file's size
// and not the directory's.
func Test_SendFile_ByteRange_DirectoryIndex(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == windowsOS {
		t.Skip("SendFile byte-range tests are flaky on Windows")
	}

	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "dir"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "dir", "index.html"), []byte(byteRangeContent), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte("0123"), 0o600))

	app := New()
	app.Get("/os", func(c Ctx) error { return c.SendFile(root+"/missing/../dir/", SendFile{ByteRange: true}) })
	app.Get("/fs", func(c Ctx) error {
		return c.SendFile("a/../../dir/", SendFile{ByteRange: true, FS: os.DirFS(root)})
	})
	app.Get("/fs-root", func(c Ctx) error {
		return c.SendFile("dir/..", SendFile{ByteRange: true, FS: os.DirFS(root)})
	})
	app.Get("/os-missing", func(c Ctx) error { return c.SendFile(root+"/missing.txt", SendFile{ByteRange: true}) })
	app.Get("/fs-missing", func(c Ctx) error {
		return c.SendFile("missing.txt", SendFile{ByteRange: true, FS: os.DirFS(root)})
	})

	for _, tc := range []struct {
		name       string
		wantRange  string
		wantStatus int
	}{
		{name: "os", wantStatus: StatusRequestedRangeNotSatisfiable, wantRange: "bytes */20"},
		{name: "fs", wantStatus: StatusRequestedRangeNotSatisfiable, wantRange: "bytes */20"},
		{name: "fs-root", wantStatus: StatusRequestedRangeNotSatisfiable, wantRange: "bytes */4"},
		// nothing to stat, so no size to state
		{name: "os-missing", wantStatus: StatusNotFound},
		{name: "fs-missing", wantStatus: StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := apptest.Expect(t, app, apptest.Req(MethodGet, "/"+tc.name, apptest.WithHeader(HeaderRange, "bytes=100-200")), tc.wantStatus)
			require.Equal(t, tc.wantRange, resp.Header.Get(HeaderContentRange))
		})
	}
}

// An entity-tag in If-Range matches an ETag the handler set before SendFile,
// which sends none of its own.
func Test_SendFile_ByteRange_IfRangeETag(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == windowsOS {
		t.Skip("SendFile byte-range tests are flaky on Windows")
	}

	fixture := byteRangeFixture(t)
	app := New()
	app.Get("/file", func(c Ctx) error {
		c.Set(HeaderETag, `"v1"`)
		return c.SendFile(fixture, SendFile{ByteRange: true})
	})

	for ifRange, wantStatus := range map[string]int{
		`"v1"`:   StatusPartialContent,
		`"v2"`:   StatusOK,
		`W/"v1"`: StatusOK, // a weak tag never matches
	} {
		t.Run(ifRange, func(t *testing.T) {
			t.Parallel()
			apptest.Expect(t, app, apptest.Req(MethodGet, "/file",
				apptest.WithHeader(HeaderRange, "bytes=0-4"), apptest.WithHeader(HeaderIfRange, ifRange)), wantStatus)
		})
	}
}

// A status set before SendFile replaces only the file server's 200, and an error
// page goes out whole; Download and MaxAge follow the status that is sent.
func Test_SendFile_KeepsFileServerAnswer(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == windowsOS {
		t.Skip("SendFile byte-range tests are flaky on Windows")
	}

	fixture := byteRangeFixture(t)
	serve := func(status int, cfg SendFile) Handler {
		return func(c Ctx) error {
			err := c.Status(status).SendFile(fixture, cfg)
			c.Set("X-Range", c.Get(HeaderRange))
			return err
		}
	}
	app := New()
	app.Get("/status", serve(StatusCreated, SendFile{ByteRange: true}))
	app.Get("/headers", serve(StatusOK, SendFile{ByteRange: true, Download: true, MaxAge: 60}))
	app.Get("/error", serve(StatusInternalServerError, SendFile{ByteRange: true, Download: true, MaxAge: 60}))

	notModified := map[string]string{HeaderIfModifiedSince: byteRangeLastModified}
	inRange := map[string]string{HeaderRange: "bytes=0-4"}
	unsatisfiable := map[string]string{HeaderRange: "bytes=100-200"}

	tests := []struct {
		fields          map[string]string
		name            string
		path            string
		wantStatus      int
		wantLength      int
		wantAttachment  bool
		wantCacheHeader bool
	}{
		{name: "whole file", path: "/status", wantStatus: StatusCreated, wantLength: 20},
		{name: "a range", path: "/status", fields: inRange, wantStatus: StatusPartialContent, wantLength: 5},
		{name: "unsatisfiable", path: "/status", fields: unsatisfiable, wantStatus: StatusRequestedRangeNotSatisfiable},
		{name: "not modified", path: "/status", fields: notModified, wantStatus: StatusNotModified},
		{name: "a range, downloaded", path: "/headers", fields: inRange, wantStatus: StatusPartialContent, wantLength: 5, wantAttachment: true, wantCacheHeader: true},
		{name: "not modified, downloaded", path: "/headers", fields: notModified, wantStatus: StatusNotModified, wantCacheHeader: true},
		{name: "unsatisfiable, downloaded", path: "/headers", fields: unsatisfiable, wantStatus: StatusRequestedRangeNotSatisfiable},
		{name: "error page, a range", path: "/error", fields: inRange, wantStatus: StatusInternalServerError, wantLength: 20},
		{name: "error page, not modified", path: "/error", fields: notModified, wantStatus: StatusInternalServerError, wantLength: 20},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := apptest.Req(MethodGet, tc.path)
			for k, v := range tc.fields {
				req.Header.Set(k, v)
			}
			resp := apptest.Expect(t, app, req, tc.wantStatus)
			if tc.wantLength > 0 {
				require.Equal(t, strconv.Itoa(tc.wantLength), resp.Header.Get(HeaderContentLength))
			}
			require.Equal(t, tc.wantAttachment, resp.Header.Get(HeaderContentDisposition) == "attachment")
			require.Equal(t, tc.wantCacheHeader, resp.Header.Get(HeaderCacheControl) == "public, max-age=60")
			require.Equal(t, tc.fields[HeaderRange], resp.Header.Get("X-Range"))
		})
	}
}

func Benchmark_Ctx_SendFile(b *testing.B) {
	c := apptest.Ctx(b, New())

	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.SendFile("ctx.go")
	}

	require.NoError(b, err)
	require.Contains(b, string(c.Response().Body()), "type DefaultCtx struct")
}

// ctxTestCEmptyValues encodes, one after the other on c, the four values every
// serializer meets sooner or later: nil, an empty string, zero and an empty
// slice. want holds the body each must leave, in that order, so a body left
// over from the previous value shows.
func ctxTestCEmptyValues(t *testing.T, c Ctx, encode func(v any) error, want [4]string) {
	t.Helper()

	for i, v := range []any{nil, "", 0, []int{}} {
		require.NoError(t, encode(v), "%#v", v)
		require.Equal(t, want[i], string(c.Response().Body()), "%#v", v)
	}
}

func Test_Ctx_JSON(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.Error(t, c.JSON(complex(1, 1)))

	// a map has no order, hence JSONEq
	require.NoError(t, c.JSON(Map{"Name": "Grame", "Age": 20}))
	require.JSONEq(t, `{"Age":20,"Name":"Grame"}`, string(c.Response().Body()))
	require.Equal(t, "application/json; charset=utf-8", string(c.Response().Header.Peek("content-type")))

	require.NoError(t, c.JSON(Map{"Name": "Grame", "Age": 20}, "application/problem+json"))
	require.JSONEq(t, `{"Age":20,"Name":"Grame"}`, string(c.Response().Body()))
	require.Equal(t, "application/problem+json", string(c.Response().Header.Peek("content-type")))

	ctxTestCEmptyValues(t, c, func(v any) error { return c.JSON(v) }, [4]string{"null", `""`, "0", "[]"})
}

// ctxBenchPayload is the value the serializer benchmarks encode.
type ctxBenchPayload struct {
	Name string
	Age  uint8
}

func Benchmark_Ctx_JSON(b *testing.B) {
	c := apptest.Ctx(b, New())

	data := ctxBenchPayload{Name: "Grame", Age: 20}
	var err error
	b.ReportAllocs()
	for b.Loop() {
		err = c.JSON(data)
	}
	require.NoError(b, err)
	require.JSONEq(b, `{"Name":"Grame","Age":20}`, string(c.Response().Body()))
}

func Test_Ctx_MsgPack(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New(Config{MsgPackEncoder: msgpack.Marshal, MsgPackDecoder: msgpack.Unmarshal}))

	require.NoError(t, c.MsgPack(complex(1, 1)))
	require.Equal(t, "\u0600?\xf0\x00\x00\x00\x00\x00\x00?\xf0\x00\x00\x00\x00\x00\x00", string(c.Response().Body()))

	// one key, as a map has no order
	require.NoError(t, c.MsgPack(Map{"Name": "Grame"}))
	require.Equal(t, "\x81\xa4Name\xa5Grame", string(c.Response().Body()))
	require.Equal(t, MIMEApplicationMsgPack, string(c.Response().Header.Peek("content-type")))

	require.NoError(t, c.MsgPack(Map{"Name": "Grame"}, "application/problem+msgpack"))
	require.Equal(t, "\x81\xa4Name\xa5Grame", string(c.Response().Body()))
	require.Equal(t, "application/problem+msgpack", string(c.Response().Header.Peek("content-type")))

	ctxTestCEmptyValues(t, c, func(v any) error { return c.MsgPack(v) }, [4]string{"\xc0", "\xa0", "\x00", "\x90"})

	t.Run("error msgpack", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, New(Config{
			MsgPackEncoder: func(_ any) ([]byte, error) { return []byte("error"), errors.New("msgpack error") },
		}))
		require.Error(t, c.MsgPack(Map{"Name": "Grame", "Age": 20}))
	})
}

func Benchmark_Ctx_MsgPack(b *testing.B) {
	c := apptest.Ctx(b, New(Config{MsgPackEncoder: msgpack.Marshal, MsgPackDecoder: msgpack.Unmarshal}))

	data := ctxBenchPayload{Name: "Grame", Age: 20}
	var err error
	b.ReportAllocs()
	for b.Loop() {
		err = c.MsgPack(data)
	}
	require.NoError(b, err)
	require.Equal(b, "\x82\xa4Name\xa5Grame\xa3Age\x14", string(c.Response().Body()))
}

func Test_Ctx_CBOR(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New(Config{CBOREncoder: cbor.Marshal, CBORDecoder: cbor.Unmarshal}))

	require.Error(t, c.CBOR(complex(1, 1)))

	type dummyStruct struct {
		Name string
		Age  int
	}

	require.NoError(t, c.CBOR(dummyStruct{Name: "Grame", Age: 20}))
	require.Equal(t, `a2644e616d65654772616d656341676514`, hex.EncodeToString(c.Response().Body()))
	require.Equal(t, "application/cbor", string(c.Response().Header.Peek("content-type")))

	require.NoError(t, c.CBOR(dummyStruct{Name: "Grame", Age: 20}, "application/problem+cbor"))
	require.Equal(t, `a2644e616d65654772616d656341676514`, hex.EncodeToString(c.Response().Body()))
	require.Equal(t, "application/problem+cbor", string(c.Response().Header.Peek("content-type")))

	// f6, 60, 00 and 80 in hex
	ctxTestCEmptyValues(t, c, func(v any) error { return c.CBOR(v) }, [4]string{"\xf6", "\x60", "\x00", "\x80"})

	// invalid types
	require.Error(t, c.CBOR(make(chan int)))
	require.Error(t, c.CBOR(func() {}))
}

func Benchmark_Ctx_CBOR(b *testing.B) {
	c := apptest.Ctx(b, New(Config{CBOREncoder: cbor.Marshal, CBORDecoder: cbor.Unmarshal}))

	data := ctxBenchPayload{Name: "Grame", Age: 20}
	var err error
	b.ReportAllocs()
	for b.Loop() {
		err = c.CBOR(data)
	}
	require.NoError(b, err)
	require.Equal(b, `a2644e616d65654772616d656341676514`, hex.EncodeToString(c.Response().Body()))
}

func Benchmark_Ctx_JSON_Ctype(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())
	data := ctxBenchPayload{Name: "Grame", Age: 20}
	var err error
	b.ReportAllocs()
	for b.Loop() {
		err = c.JSON(data, "application/problem+json")
	}
	require.NoError(b, err)
	require.JSONEq(b, `{"Name":"Grame","Age":20}`, string(c.Response().Body()))
	require.Equal(b, "application/problem+json", string(c.Response().Header.Peek("content-type")))
}

func Test_Ctx_JSONP(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.Error(t, c.JSONP(complex(1, 1)))

	require.NoError(t, c.JSONP(Map{"Name": "Grame", "Age": 20}))
	require.Equal(t, `callback({"Age":20,"Name":"Grame"});`, string(c.Response().Body()))
	require.Equal(t, "text/javascript; charset=utf-8", string(c.Response().Header.Peek("content-type")))

	require.NoError(t, c.Res().JSONP(Map{"Name": "Grame", "Age": 20}, "john"))
	require.Equal(t, `john({"Age":20,"Name":"Grame"});`, string(c.Response().Body()))
	require.Equal(t, "text/javascript; charset=utf-8", string(c.Response().Header.Peek("content-type")))
}

func Test_Ctx_JSONP_SanitizesCallback(t *testing.T) {
	t.Parallel()
	// The callback name is emitted verbatim into a same-origin
	// text/javascript body, and JSONP callers take it from the query string by
	// design, so anything that could open a statement, a string, or a comment
	// has to be stripped before it gets there.
	tests := []struct {
		name     string
		callback string
		expected string
	}{
		{name: "statement injection", callback: "alert(1);//", expected: "alert1"},
		{name: "markup injection", callback: "</script><img src=x onerror=alert(1)>", expected: "scriptimgsrcxonerroralert1"},
		{name: "eval payload", callback: "eval(atob('YWxlcnQoMSk='));x", expected: "evalatobYWxlcnQoMSkx"},
		{name: "newline", callback: "cb\nalert(1)", expected: "cbalert1"},
		{name: "quote", callback: `cb"+alert(1)+"`, expected: "cbalert1"},

		// Legitimate member expressions must survive untouched.
		{name: "plain identifier", callback: "cb", expected: "cb"},
		{name: "property access", callback: "window.cb", expected: "window.cb"},
		{name: "bracket index", callback: "ns.cb[0]", expected: "ns.cb[0]"},
		{name: "dollar and underscore", callback: "$.jsonp_1", expected: "$.jsonp_1"},

		// Nothing callable left: fall back rather than emit a body that throws.
		{name: "fully stripped", callback: "()+-;", expected: "callback"},
		{name: "empty", callback: "", expected: "callback"},
		{name: "bare number", callback: "123", expected: "callback"},
		{name: "leading digit", callback: "0.0", expected: "callback"},
		{name: "dots only", callback: "...", expected: "callback"},
		{name: "unbalanced bracket", callback: "a[", expected: "callback"},
		{name: "empty index", callback: "a[]", expected: "callback"},
		{name: "empty label", callback: "a..b", expected: "callback"},

		// An identifier may not resume straight after a closing bracket; only
		// '.', '[' or another ']' may follow one.
		{name: "identifier after index", callback: "cb[0]x", expected: "callback"},
		{name: "digit after index", callback: "cb[0]1", expected: "callback"},
		{name: "index after index", callback: "cb[0][1]", expected: "cb[0][1]"},
		{name: "property after index", callback: "cb[0].x", expected: "cb[0].x"},
		{name: "nested index", callback: "a[b[0]]", expected: "a[b[0]]"},

		// An index that opens with a digit is a number, so it is digits to the
		// closing bracket. Anything else there is made of allowed bytes and still
		// emits a body that throws.
		{name: "multi digit index", callback: "cb[10]", expected: "cb[10]"},
		{name: "identifier index", callback: "cb[i]", expected: "cb[i]"},
		{name: "index with trailing letters", callback: "cb[0foo]", expected: "callback"},
		{name: "hex-looking index", callback: "ns[0x]", expected: "callback"},
		{name: "dotted number index", callback: "ns[1.2.3]", expected: "callback"},
		{name: "index into a number", callback: "cb[0[1]]", expected: "callback"},

		// Only a token opened by '[' may be a number. A property named after a dot
		// may not, however deep in brackets the dot sits.
		{name: "numeric property in index", callback: "cb[a.0]", expected: "callback"},
		{name: "numeric property nested", callback: "cb[x[a.1]]", expected: "callback"},
		{name: "numeric property trailing", callback: "cb[a.b.0]", expected: "callback"},
		{name: "identifier property in index", callback: "cb[a.b]", expected: "cb[a.b]"},

		// A reserved word is refused only where the word is read as a name: the
		// head of the expression and the head inside each index. After a dot it is
		// a property name, which any word may be.
		{name: "reserved word alone", callback: "for", expected: "callback"},
		{name: "reserved word in index", callback: "cb[new]", expected: "callback"},

		// A keyword that is itself a complete expression parses where one is
		// read, so these are left alone.
		{name: "this heads the expression", callback: "this.cb", expected: "this.cb"},
		{name: "literal heads the expression", callback: "null.x", expected: "null.x"},
		{name: "literal in an index", callback: "cb[true]", expected: "cb[true]"},
		{name: "this in an index", callback: "cb[this]", expected: "cb[this]"},
		{name: "reserved word heads an index", callback: "cb[new.a]", expected: "callback"},
		{name: "reserved word as property", callback: "cb.new", expected: "cb.new"},
		{name: "reserved word as nested property", callback: "cb[a.class]", expected: "cb[a.class]"},
		{name: "reserved word as prefix", callback: "news", expected: "news"},

		// "let" is a keyword only where a "[" follows it at the head of the
		// statement the body is: "let[a](…);" is read as a destructuring
		// declaration and throws, while every other position is a call.
		{name: "let indexed at the head", callback: "let[a]", expected: "callback"},
		{name: "let indexed by a number", callback: "let[0]", expected: "callback"},
		{name: "let alone", callback: "let", expected: "let"},
		{name: "let with a property", callback: "let.a", expected: "let.a"},
		{name: "let indexed deeper", callback: "cb[let[a]]", expected: "cb[let[a]]"},

		// A JSONP body is parsed as a classic script in sloppy mode, where
		// "await" is an identifier unless it stands in a module or an async
		// function, and "yield" unless in strict mode or a generator. Both are
		// callable names here; V8 parses every one of these.
		{name: "await heads the expression", callback: "await.cb", expected: "await.cb"},
		{name: "await in an index", callback: "cb[await]", expected: "cb[await]"},
		{name: "yield heads the expression", callback: "yield.cb", expected: "yield.cb"},
		{name: "yield in an index", callback: "cb[yield]", expected: "cb[yield]"},
		{name: "let as a property", callback: "cb.let[0]", expected: "cb.let[0]"},
	}

	app := New()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)

			require.NoError(t, c.JSONP(Map{"a": 1}, tc.callback))
			require.Equal(t, tc.expected+`({"a":1});`, string(c.Response().Body()))
			require.Equal(t, "text/javascript; charset=utf-8", string(c.Response().Header.Peek(HeaderContentType)))
			require.Equal(t, "nosniff", string(c.Response().Header.Peek(HeaderXContentTypeOptions)))
		})
	}
}

func Benchmark_Ctx_JSONP(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())

	data := ctxBenchPayload{Name: "Grame", Age: 20}
	callback := "emit"
	var err error
	b.ReportAllocs()
	for b.Loop() {
		err = c.JSONP(data, callback)
	}
	require.NoError(b, err)
	require.Equal(b, `emit({"Name":"Grame","Age":20});`, string(c.Response().Body()))
}

func Test_Ctx_XML(t *testing.T) {
	t.Parallel()
	c := apptest.CtxAs[*DefaultCtx](t, New())

	require.Error(t, c.JSON(complex(1, 1)))

	type xmlResult struct {
		XMLName xml.Name `xml:"Users"`
		Names   []string `xml:"Names"`
		Ages    []int    `xml:"Ages"`
	}

	require.NoError(t, c.XML(xmlResult{Names: []string{"Grame", "John"}, Ages: []int{1, 12, 20}}))
	require.Equal(t, `<Users><Names>Grame</Names><Names>John</Names><Ages>1</Ages><Ages>12</Ages><Ages>20</Ages></Users>`, string(c.Response().Body()))
	require.Equal(t, "application/xml; charset=utf-8", string(c.Response().Header.Peek("content-type")))

	ctxTestCEmptyValues(t, c, func(v any) error { return c.XML(v) }, [4]string{"", `<string></string>`, "<int>0</int>", ""})
}

// ctxTestCEncoder returns an encoder that answers every value with body.
func ctxTestCEncoder(body string) func(any) ([]byte, error) {
	return func(any) ([]byte, error) { return []byte(body), nil }
}

// Every serializer encodes through the encoder its Config names, and sets the
// Content-Type of its own format.
func Test_Ctx_CustomEncoder(t *testing.T) {
	t.Parallel()
	data := Map{"Name": "Grame", "Age": 20}
	for _, tc := range []struct {
		send        func(c Ctx) error
		config      Config
		name        string
		wantBody    string
		wantContent string
	}{
		{
			name: "json", config: Config{JSONEncoder: ctxTestCEncoder(`["custom","json"]`)},
			send:     func(c Ctx) error { return c.JSON(data) },
			wantBody: `["custom","json"]`, wantContent: "application/json; charset=utf-8",
		},
		{
			name: "jsonp", config: Config{JSONEncoder: ctxTestCEncoder(`["custom","json"]`)},
			send:     func(c Ctx) error { return c.JSONP(data) },
			wantBody: `callback(["custom","json"]);`, wantContent: "text/javascript; charset=utf-8",
		},
		{
			name: "msgpack", config: Config{MsgPackEncoder: ctxTestCEncoder(`["custom","msgpack"]`)},
			send:     func(c Ctx) error { return c.MsgPack(data) },
			wantBody: `["custom","msgpack"]`, wantContent: MIMEApplicationMsgPack,
		},
		{
			name: "cbor", config: Config{CBOREncoder: ctxTestCEncoder(hex.EncodeToString([]byte("random")))},
			send:     func(c Ctx) error { return c.CBOR(data) },
			wantBody: `72616e646f6d`, wantContent: "application/cbor",
		},
		{
			name: "xml", config: Config{XMLEncoder: ctxTestCEncoder(`<custom>xml</custom>`)},
			send:     func(c Ctx) error { return c.XML(data) },
			wantBody: `<custom>xml</custom>`, wantContent: "application/xml; charset=utf-8",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New(tc.config))

			require.NoError(t, tc.send(c))
			require.Equal(t, tc.wantBody, string(c.Response().Body()))
			require.Equal(t, tc.wantContent, string(c.Response().Header.Peek("content-type")))
		})
	}
}

func Benchmark_Ctx_XML(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())
	type SomeStruct struct {
		Name string `xml:"Name"`
		Age  uint8  `xml:"Age"`
	}
	data := SomeStruct{Name: "Grame", Age: 20}
	var err error
	b.ReportAllocs()
	for b.Loop() {
		err = c.XML(data)
	}

	require.NoError(b, err)
	require.Equal(b, `<SomeStruct><Name>Grame</Name><Age>20</Age></SomeStruct>`, string(c.Response().Body()))
}

func Test_Ctx_Links(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Links()
	require.Empty(t, string(c.Response().Header.Peek(HeaderLink)))

	c.Links("http://api.example.com/users?page=2", "next", "http://api.example.com/users?page=5", "last")
	require.Equal(t, `<http://api.example.com/users?page=2>; rel="next",<http://api.example.com/users?page=5>; rel="last"`, string(c.Response().Header.Peek(HeaderLink)))

	// A rel value containing quotes/backslashes must be escaped so the
	// quoted-string stays grammar-valid (RFC 9110 §5.6.4).
	c.Links("http://example.com", `next" x`)
	require.Equal(t, `<http://example.com>; rel="next\" x"`, string(c.Response().Header.Peek(HeaderLink)))

	// Unlike Attachment filenames, rel values are not sanitized first, so the
	// shared serializer must also encode controls here.
	c.Links("http://example.com", "next\t\x01\n\r\x7f")
	require.Equal(t, `<http://example.com>; rel="next%09%01\n\r%7F"`, string(c.Response().Header.Peek(HeaderLink)))
}

func Benchmark_Ctx_Links(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())

	b.ReportAllocs()
	for b.Loop() {
		c.Links("http://api.example.com/users?page=2", "next", "http://api.example.com/users?page=5", "last")
	}
}

func Test_Ctx_Location(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Location("http://example.com")
	require.Equal(t, "http://example.com", string(c.Response().Header.Peek(HeaderLocation)))
}

func Test_Ctx_Next(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use("/", func(c Ctx) error { return c.Next() })
	app.Get("/test", func(c Ctx) error {
		c.Set("X-Next-Result", "Works")
		return nil
	})
	resp := apptest.Expect(t, app, "http://example.com/test", StatusOK)
	require.Equal(t, "Works", resp.Header.Get("X-Next-Result"))
}

func Test_Ctx_Next_Error(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use("/", func(c Ctx) error {
		c.Set("X-Next-Result", "Works")
		return ErrNotFound
	})

	resp := apptest.Expect(t, app, "http://example.com/test", StatusNotFound)
	require.Equal(t, "Works", resp.Header.Get("X-Next-Result"))
}

func Test_Ctx_Render(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.NoError(t, c.Render("./.github/testdata/index.tmpl", Map{"Title": "Hello, World!"}))
	require.Equal(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))

	require.Error(t, c.Render("./.github/testdata/template-non-exists.html", nil))
	require.Error(t, c.Res().Render("./.github/testdata/template-invalid.html", nil))
}

// What Render makes of the bind it is given, the view bind, the locals and the
// engine, whether a file renders as a template or an engine renders it by name.
func Test_Ctx_Render_Variants(t *testing.T) {
	t.Parallel()
	const index = "./.github/testdata/index.tmpl"

	// New loads the templates of the engine in a Config, so every row has its own.
	for _, tc := range []struct {
		config   Config
		bind     any
		viewBind Map
		locals   Map
		name     string
		template string
		want     string
	}{
		{
			name: "locals are not passed to views", config: Config{PassLocalsToViews: false},
			locals: Map{"Title": "Hello, World!"}, template: index, bind: Map{}, want: "<h1></h1>",
		},
		{
			name: "locals with an empty bind", config: Config{PassLocalsToViews: true},
			locals: Map{"Title": "Hello, World!"}, template: index, bind: Map{}, want: "<h1>Hello, World!</h1>",
		},
		{
			name: "locals with a nil bind", config: Config{PassLocalsToViews: true},
			locals: Map{"Title": "Hello, World!"}, template: index, bind: nil, want: "<h1>Hello, World!</h1>",
		},
		{
			name: "view bind", viewBind: Map{"Title": "Hello, World!"},
			template: index, bind: Map{}, want: "<h1>Hello, World!</h1>",
		},
		{
			name: "bind overwrites the view bind", viewBind: Map{"Title": "Hello, World!"},
			template: index, bind: Map{"Title": "Hello from Fiber!"}, want: "<h1>Hello from Fiber!</h1>",
		},
		{
			name: "view bind and a nil Map bind", viewBind: Map{"Title": "Hello, World!"},
			template: index, bind: Map(nil), want: "<h1>Hello, World!</h1>",
		},
		{
			name: "view bind and locals", config: Config{PassLocalsToViews: true},
			viewBind: Map{"Title": "Hello, World!"}, locals: Map{"Summary": "Test"},
			template: "./.github/testdata/template.tmpl", bind: Map{}, want: "<h1>Hello, World! Test</h1>",
		},
		{
			name: "engine", config: Config{Views: &testTemplateEngine{}},
			template: "index.tmpl", bind: Map{"Title": "Hello, World!"}, want: "<h1>Hello, World!</h1>",
		},
		{
			name: "engine, the bind wins over the locals", config: Config{PassLocalsToViews: true, Views: &testTemplateEngine{}},
			locals: Map{"Title": "This is a test."}, template: "index.tmpl", bind: Map{"Title": "Hello, World!"}, want: "<h1>Hello, World!</h1>",
		},
		{
			name: "engine with a views layout", config: Config{ViewsLayout: "main.tmpl", Views: &testTemplateEngine{}},
			template: "index.tmpl", bind: Map{"Title": "Hello, World!"}, want: "<h1>Hello, World!</h1><h1>I'm main</h1>",
		},
		{
			name: "typed nil engine", config: Config{Views: (*testTemplateEngine)(nil)},
			template: index, bind: Map{"Title": "Hello, World!"}, want: "<h1>Hello, World!</h1>",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New(tc.config))
			if tc.viewBind != nil {
				require.NoError(t, c.ViewBind(tc.viewBind))
			}
			for key, value := range tc.locals {
				c.Locals(key, value)
			}

			require.NoError(t, c.Render(tc.template, tc.bind))

			// The body must not alias the pooled buffer the template was rendered into.
			buf := bytebufferpool.Get()
			buf.WriteString("overwrite")
			defer bytebufferpool.Put(buf)

			require.Equal(t, tc.want, string(c.Response().Body()))
		})
	}
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

	app.Get("/one", func(c Ctx) error { return c.SendStatus(StatusOK) })

	apptest.Expect(t, app, "/one", StatusOK)
	apptest.Expect(t, app, "/missing", StatusNotFound)
}

func Test_Ctx_Matched_RouteError(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ErrorHandler: func(c Ctx, err error) error {
			require.True(t, c.Matched())
			return c.Status(StatusNotFound).SendString(err.Error())
		},
	})

	app.Get("/", func(_ Ctx) error { return ErrNotFound })

	apptest.Expect(t, app, "/", StatusNotFound)
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

	apptest.Expect(t, app, "/", StatusOK)
	apptest.Expect(t, app, "/route", StatusOK)
}

func Test_Ctx_HasBody(t *testing.T) {
	t.Parallel()
	app := New()

	acquire := func(t *testing.T) CustomCtx {
		t.Helper()
		ctx := apptest.Ctx(t, app)
		require.NotNil(t, ctx)
		return ctx
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

	for _, tc := range []struct {
		name  string
		lines []string // the Transfer-Encoding field lines
		want  bool
	}{
		{name: "transfer encoding chunked", lines: []string{"chunked"}, want: true},
		{name: "transfer encoding whitespace", lines: []string{"  ChUnKeD  "}, want: true},
		{name: "transfer encoding parameters", lines: []string{"chunked; q=1"}, want: true},
		{name: "transfer encoding multiple values", lines: []string{"gzip, chunked"}, want: true},
		{name: "transfer encoding identity", lines: []string{"identity"}, want: false},
		{name: "transfer encoding identity then chunked", lines: []string{"identity, chunked"}, want: true},
		// Repeated field lines form one combined list (RFC 9110 §5.2):
		// "identity" on the first line must not mask "gzip" on the second.
		{name: "transfer encoding multiple field lines", lines: []string{"identity", "gzip"}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := acquire(t)
			hdr := &ctx.Request().Header
			hdr.DisableSpecialHeader()
			for _, line := range tc.lines {
				hdr.Add(HeaderTransferEncoding, line)
			}
			hdr.Set(HeaderContentLength, "0")
			hdr.EnableSpecialHeader()
			require.Zero(t, hdr.ContentLength())
			require.Empty(t, ctx.Request().Body())
			require.Equal(t, tc.want, ctx.HasBody())
		})
	}

	t.Run("no body", func(t *testing.T) {
		t.Parallel()
		ctx := acquire(t)
		require.False(t, ctx.HasBody())
	})
}

func Test_Ctx_IsWebSocket(t *testing.T) {
	t.Parallel()
	app := New()

	for _, tc := range []struct {
		name    string
		headers [][2]string // request field lines, added in order
		want    bool
	}{
		{name: "upgrade", headers: [][2]string{{HeaderConnection, "keep-alive, Upgrade"}, {HeaderUpgrade, "websocket"}}, want: true},
		{name: "connection is not an upgrade", headers: [][2]string{{HeaderConnection, "not-an-upgrade"}, {HeaderUpgrade, "websocket"}}},
		// Upgrade is a comma-separated protocol list (RFC 9110 §7.8): websocket
		// must be found among other protocols and with a "/version" suffix.
		{name: "websocket among other protocols", headers: [][2]string{{HeaderConnection, "Upgrade"}, {HeaderUpgrade, "h2c, websocket/13"}}, want: true},
		// Repeated Connection field lines are equivalent to one combined list
		// (RFC 9110 §5.2).
		{
			name:    "repeated connection field lines",
			headers: [][2]string{{HeaderConnection, "keep-alive"}, {HeaderConnection, "Upgrade"}, {HeaderUpgrade, "websocket"}},
			want:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)
			require.NotNil(t, c)
			for _, header := range tc.headers {
				c.Request().Header.Add(header[0], header[1])
			}
			require.Equal(t, tc.want, c.IsWebSocket())
		})
	}
}

func Test_Ctx_IsPreflight(t *testing.T) {
	t.Parallel()
	app := New()

	for _, tc := range []struct {
		headers map[string]string // request headers besides the OPTIONS method
		name    string
		want    bool
	}{
		{name: "preflight", headers: map[string]string{HeaderAccessControlRequestMethod: MethodGet, HeaderOrigin: "https://example.com"}, want: true},
		{name: "no origin", headers: map[string]string{HeaderAccessControlRequestMethod: MethodGet}},
		{name: "no request method", headers: map[string]string{HeaderOrigin: "https://example.com"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(MethodOptions)
			for key, value := range tc.headers {
				fctx.Request.Header.Set(key, value)
			}
			c := apptest.CtxFor(t, app, fctx)
			require.NotNil(t, c)
			require.Equal(t, tc.want, c.IsPreflight())
		})
	}
}

func Test_Ctx_Render_LeavesBindMapAlone(t *testing.T) {
	t.Parallel()
	app := New(Config{PassLocalsToViews: true})
	page := Map{"Unrelated": "same for everyone"}

	for _, title := range []string{"first", "second"} {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		require.NoError(t, c.ViewBind(Map{"Title": title}))
		c.Locals("Local", title)
		require.NoError(t, c.Render("./.github/testdata/index.tmpl", page))
		require.Equal(t, "<h1>"+title+"</h1>", string(c.Response().Body()))
		app.ReleaseCtx(c)
	}

	require.Equal(t, Map{"Unrelated": "same for everyone"}, page)
}

func Benchmark_Ctx_RenderWithLocalsAndViewBind(b *testing.B) {
	engine := &testTemplateEngine{}
	require.NoError(b, engine.Load())
	c := apptest.Ctx(b, New(Config{PassLocalsToViews: true, Views: engine}))
	require.NoError(b, c.ViewBind(Map{"Title": "Hello, World!"}))
	c.Locals("Summary", "Test")
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.Render("template.tmpl", Map{})
	}
	require.NoError(b, err)
	require.Equal(b, "<h1>Hello, World! Test</h1>", string(c.Response().Body()))
}

// ctxBenchViewsCtx returns a context of an app built from config whose Views,
// set after New, is a loaded testTemplateEngine.
func ctxBenchViewsCtx(b *testing.B, config ...Config) CustomCtx {
	b.Helper()
	engine := &testTemplateEngine{}
	require.NoError(b, engine.Load())
	app := New(config...)
	app.config.Views = engine
	return apptest.Ctx(b, app)
}

func Benchmark_Ctx_RenderLocals(b *testing.B) {
	c := ctxBenchViewsCtx(b, Config{PassLocalsToViews: true})
	c.Locals("Title", "Hello, World!")
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.Render("index.tmpl", Map{})
	}
	require.NoError(b, err)
	require.Equal(b, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

func Benchmark_Ctx_RenderViewBind(b *testing.B) {
	c := ctxBenchViewsCtx(b)
	require.NoError(b, c.ViewBind(Map{"Title": "Hello, World!"}))
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.Render("index.tmpl", Map{})
	}
	require.NoError(b, err)
	require.Equal(b, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

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
	apptest.Expect(t, app, "http://example.com/", StatusOK)
	require.Equal(t, 3, calls, "Number of calls")
}

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

	apptest.Expect(t, app, "http://example.com/old", StatusOK)
	require.False(t, executedOldHandler, "Executed old handler")
	require.True(t, executedNewHandler, "Executed new handler")
}

func Test_Ctx_RestartRoutingWithChangedPathAndCatchAll(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/new", func(_ Ctx) error { return nil })
	app.Use(func(c Ctx) error {
		c.Path("/new")
		// c.Next() would fail this test as a 404 is returned from the next handler
		return c.RestartRouting()
	})
	app.Use(func(_ Ctx) error { return ErrNotFound })

	apptest.Expect(t, app, "http://example.com/old", StatusOK)
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

func Benchmark_Ctx_Render_Engine(b *testing.B) {
	c := ctxBenchViewsCtx(b)

	var err error
	b.ReportAllocs()
	for b.Loop() {
		err = c.Render("index.tmpl", Map{"Title": "Hello, World!"})
	}
	require.NoError(b, err)
	require.Equal(b, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

func Benchmark_Ctx_Get_Location_From_Route(b *testing.B) {
	app := New()
	c := apptest.CtxAs[*DefaultCtx](b, app)

	app.Get("/user/:name", func(c Ctx) error { return c.SendString(c.Params("name")) }).Name("User")

	var err error
	var location string
	for b.Loop() {
		route := app.GetRoute("User")
		location, err = c.getLocationFromRoute(&route, Map{"name": "fiber"})
	}

	require.Equal(b, "/user/fiber", location)
	require.NoError(b, err)
}

func Test_Ctx_Get_Location_From_Route_name(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		config Config
		params Map
		name   string
		route  string
		want   string
	}{
		{name: "case-insensitive", route: "/user/:name", params: Map{"name": "fiber"}, want: "/user/fiber"},
		{name: "case-insensitive, other case", route: "/user/:name", params: Map{"Name": "fiber"}, want: "/user/fiber"},
		{name: "case-sensitive", config: Config{CaseSensitive: true}, route: "/user/:name", params: Map{"name": "fiber"}, want: "/user/fiber"},
		{name: "case-sensitive, other case", config: Config{CaseSensitive: true}, route: "/user/:name", params: Map{"Name": "fiber"}, want: "/user/"},
		{
			name: "optional greedy", route: "/:phone/*/send/*",
			params: Map{"phone": "23456789", "*1": "sms", "*2": "test-msg"}, want: "/23456789/sms/send/test-msg",
		},
		{
			name: "optional greedy, one param", route: "/:phone/*/send",
			params: Map{"phone": "23456789", "*": "sms"}, want: "/23456789/sms/send",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New(tc.config)
			c := apptest.Ctx(t, app)
			app.Get(tc.route, func(_ Ctx) error { return nil }).Name("Named")

			location, err := c.GetRouteURL("Named", tc.params)
			require.NoError(t, err)
			require.Equal(t, tc.want, location)
		})
	}
}

type errorTemplateEngine struct{}

func (errorTemplateEngine) Render(_ io.Writer, _ string, _ any, _ ...string) error {
	return errors.New("errorTemplateEngine")
}

func (errorTemplateEngine) Load() error { return nil }

func Test_Ctx_Render_Engine_Error(t *testing.T) {
	t.Parallel()
	app := New()
	app.config.Views = errorTemplateEngine{}
	c := apptest.Ctx(t, app)

	require.Error(t, c.Render("index.tmpl", nil))
}

// A file is rendered as a Go template, and the output must not alias the template source: the binds put their
// values at the offsets where the source spells a field name and a string constant.
func Test_Ctx_Render_Go_Template(t *testing.T) {
	t.Parallel()
	// Puts "Pass" at the offset where the source spells "Body".
	title := strings.Repeat("A", 21) + "Pass"
	// Puts "guest" at the offset where the source spells "admin".
	q := "xxxxxxxxxxxxxxxeqxxRolexxguest"

	for _, tc := range []struct {
		bind   any
		name   string
		source string
		want   string
	}{
		{name: "plain text", source: "template", want: "template"},
		{
			name: "field name", source: "<h1>{{.Title}}</h1><p>{{.Body}}</p>",
			bind: Map{"Title": title, "Body": "public", "Pass": "secret"}, want: "<h1>" + title + "</h1><p>public</p>",
		},
		{
			name: "string constant", source: `<p>{{.Q}}</p>{{if eq .Role "admin"}}ADMIN PANEL{{end}}`,
			bind: Map{"Q": q, "Role": "guest"}, want: "<p>" + q + "</p>",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			name := filepath.Join(t.TempDir(), "page.html")
			require.NoError(t, os.WriteFile(name, []byte(tc.source), 0o600))
			c := apptest.Ctx(t, New())

			require.NoError(t, c.Render(name, tc.bind))
			require.Equal(t, tc.want, string(c.Response().Body()))
		})
	}
}

func Test_Ctx_Send(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.NoError(t, c.Send([]byte("Hello, World")))
	require.NoError(t, c.Send([]byte("Don't crash please")))
	require.NoError(t, c.Send([]byte("1337")))
	require.Equal(t, "1337", string(c.Response().Body()))
}

func Benchmark_Ctx_Send(b *testing.B) {
	c := apptest.Ctx(b, New())

	byt := []byte("Hello, World!")
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.Send(byt)
	}
	require.NoError(b, err)
	require.Equal(b, "Hello, World!", string(c.Response().Body()))
}

func Test_Ctx_SendStatus(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.NoError(t, c.SendStatus(415))
	require.Equal(t, 415, c.Response().StatusCode())
	require.Equal(t, "Unsupported Media Type", string(c.Response().Body()))
}

func Test_Ctx_SendStatus_DoesNotDrainStream(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.NoError(t, c.SendStream(strings.NewReader("streamed")))
	require.NoError(t, c.SendStatus(StatusUnsupportedMediaType))
	require.Equal(t, StatusUnsupportedMediaType, c.Response().StatusCode())
	require.True(t, c.Response().IsBodyStream())
}

func Test_Ctx_SendStatusNoBodyResponses(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		status int
	}{
		{name: "Informational", status: StatusContinue},
		{name: "Processing", status: StatusProcessing},
		{name: "SwitchingProtocols", status: StatusSwitchingProtocols},
		{name: "EarlyHints", status: StatusEarlyHints},
		{name: "NoContent", status: StatusNoContent},
		{name: "ResetContent", status: StatusResetContent},
		{name: "NotModified", status: StatusNotModified},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())

			c.Response().SetBodyString("preset body")

			require.NoError(t, c.SendStatus(testCase.status))
			require.Empty(t, c.Response().Body())
			require.Equal(t, 0, c.Response().Header.ContentLength())
		})
	}
}

func Test_Ctx_SendString(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.NoError(t, c.SendString("Don't crash please"))
	require.Equal(t, "Don't crash please", string(c.Response().Body()))
}

func Test_Ctx_SendStream(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.NoError(t, c.SendStream(bytes.NewReader([]byte("Don't crash please"))))
	require.Equal(t, "Don't crash please", string(c.Response().Body()))

	require.NoError(t, c.SendStream(bytes.NewReader([]byte("Don't crash please")), len([]byte("Don't crash please"))))
	require.Equal(t, "Don't crash please", string(c.Response().Body()))

	require.NoError(t, c.SendStream(bufio.NewReader(bytes.NewReader([]byte("Hello bufio")))))
	require.Equal(t, "Hello bufio", string(c.Response().Body()))
}

func Test_Ctx_SendStreamWriter(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	require.NoError(t, c.SendStreamWriter(func(w *bufio.Writer) {
		w.WriteString("Don't crash please") //nolint:errcheck // It is fine to ignore the error
	}))
	require.Equal(t, "Don't crash please", string(c.Response().Body()))

	require.NoError(t, c.SendStreamWriter(func(w *bufio.Writer) {
		for lineNum := 1; lineNum <= 5; lineNum++ {
			fmt.Fprintf(w, "Line %d\n", lineNum)
			if flushErr := w.Flush(); flushErr != nil {
				t.Errorf("unexpected error: %s", flushErr)
				return
			}
		}
	}))
	require.Equal(t, "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n", string(c.Response().Body()))

	require.NoError(t, c.SendStreamWriter(func(_ *bufio.Writer) {}))
	require.Empty(t, c.Response().Body())
}

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
					flushErrLine.Store(int32(lineNum))
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

func Test_Ctx_Set(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Set("X-1", "1")
	c.Set("X-2", "2")
	c.Set("X-3", "3")
	c.Set("X-3", "1337")
	require.Equal(t, "1", string(c.Response().Header.Peek("x-1")))
	require.Equal(t, "2", string(c.Response().Header.Peek("x-2")))
	require.Equal(t, "1337", string(c.Response().Header.Peek("x-3")))
}

func Test_Ctx_Set_Splitter(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Set("Location", "foo\r\nSet-Cookie:%20SESSIONID=MaliciousValue\r\n")
	require.NotContains(t, string(c.Response().Header.Peek("Location")), "\r\n")

	c.Set("Location", "foo\nSet-Cookie:%20SESSIONID=MaliciousValue\n")
	require.NotContains(t, string(c.Response().Header.Peek("Location")), "\n")
}

func Benchmark_Ctx_Set(b *testing.B) {
	c := apptest.Ctx(b, New())

	val := "1431-15132-3423"
	b.ReportAllocs()
	for b.Loop() {
		c.Set(HeaderXRequestID, val)
	}
}

func Benchmark_Ctx_Set_Canonical(b *testing.B) {
	c := apptest.Ctx(b, New())

	// A key already in canonical form, which is what the constants are; the
	// one above is not, fasthttp stores it as X-Request-Id.
	val := "https://example.com"
	b.ReportAllocs()
	for b.Loop() {
		c.Set(HeaderAccessControlAllowOrigin, val)
	}
}

func Test_Ctx_Status(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Status(400)
	require.Equal(t, 400, c.Response().StatusCode())
	require.NoError(t, c.Status(415).Send([]byte("Hello, World")))
	require.Equal(t, 415, c.Response().StatusCode())
	require.Equal(t, "Hello, World", string(c.Response().Body()))
}

// Three-digit codes go out as given, 600..999 included (RFC 9112 §4).
func Test_Ctx_Status_ThreeDigits(t *testing.T) {
	t.Parallel()
	app := New()
	for _, code := range []int{100, 101, 200, 204, 304, 404, 499, 500, 599, 600, 601, 799, 999} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)

			c.Status(code)
			require.Equal(t, code, c.Response().StatusCode())
			c.Res().Status(code)
			require.Equal(t, code, c.Response().StatusCode())
		})
	}
}

// Codes that do not fit a status line are logged and sent as 500.
func Test_Ctx_Status_Invalid(t *testing.T) {
	// Not parallel: redirects the process-wide logger output.
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	invalid := []int{0, -1, -5, 1, 99, 1000, 12345, math.MaxInt, math.MinInt}

	app := New()
	for _, code := range invalid {
		app.Get("/status/"+strconv.Itoa(code), func(c Ctx) error { return c.Status(code).SendString("body") })
		app.Get("/send/"+strconv.Itoa(code), func(c Ctx) error { return c.SendStatus(code) })
		app.Get("/error/"+strconv.Itoa(code), func(_ Ctx) error { return NewError(code, "failed") })
		app.Get("/redirect/"+strconv.Itoa(code), func(c Ctx) error {
			return c.Redirect().Status(code).To("/elsewhere")
		})
	}

	for _, code := range invalid {
		for _, kind := range []string{"status", "send", "error", "redirect"} {
			t.Run(kind+"/"+strconv.Itoa(code), func(t *testing.T) {
				logged.Reset()

				// app.Test reads the response with net/http, which refuses malformed status lines.
				apptest.Expect(t, app, "/"+kind+"/"+strconv.Itoa(code), StatusInternalServerError)

				require.Contains(t, logged.String(), fmt.Sprintf("status code %d is not a three-digit HTTP status, sending 500 instead", code))
			})
		}
	}

	t.Run("SendStatus answers with the status that is sent", func(t *testing.T) {
		c := apptest.Ctx(t, app)

		require.NoError(t, c.SendStatus(0))
		require.Equal(t, StatusInternalServerError, c.Response().StatusCode())
		require.Equal(t, "Internal Server Error", string(c.Response().Body()))
	})

	t.Run("a valid status is not logged", func(t *testing.T) {
		logged.Reset()

		c := apptest.Ctx(t, app)
		c.Status(StatusTeapot)
		require.NoError(t, c.SendStatus(StatusNoContent))
		require.Empty(t, logged.String())
	})
}

func Benchmark_Ctx_Status(b *testing.B) {
	c := apptest.Ctx(b, New())

	b.ReportAllocs()
	for b.Loop() {
		c.Status(StatusTeapot)
	}
	require.Equal(b, StatusTeapot, c.Response().StatusCode())
}

func Test_Ctx_Type(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	// The steps run in order on one context: each must replace the Content-Type
	// the one before it set.
	for _, tc := range []struct {
		extension string
		want      string
		charset   []string
	}{
		{extension: ".json", want: "application/json; charset=utf-8"},
		{extension: "json", charset: []string{"utf-8"}, want: "application/json; charset=utf-8"},
		{extension: ".html", want: "text/html; charset=utf-8"},
		{extension: "html", charset: []string{"utf-8"}, want: "text/html; charset=utf-8"},
		// Test other text types get UTF-8 by default
		{extension: "txt", want: "text/plain; charset=utf-8"},
		{extension: "css", want: "text/css; charset=utf-8"},
		{extension: "js", want: "text/javascript; charset=utf-8"},
		{extension: "xml", want: "application/xml; charset=utf-8"},
		// Test binary types don't get charset
		{extension: "png", want: "image/png"},
		{extension: "pdf", want: "application/pdf"},
		// Test custom charset override
		{extension: "html", charset: []string{"iso-8859-1"}, want: "text/html; charset=iso-8859-1"},
	} {
		c.Type(tc.extension, tc.charset...)
		require.Equal(t, tc.want, string(c.Response().Header.Peek("Content-Type")), "Type(%q, %v)", tc.extension, tc.charset)
	}
}

// registerTestMIMETypes adds extensions the standard library answers with
// parameters of its own, which is what utils.GetMIME falls back to for an
// extension outside its table (csv and mjs among them). They are registered here
// so the tests do not depend on the MIME database of the machine that runs them.
var registerTestMIMETypes = sync.OnceFunc(func() {
	for ext, mimeType := range map[string]string{
		".fibertestcsv":     "text/x-fibercsv; charset=utf-8",
		".fibertestlatin":   "text/x-fiberlatin; charset=iso-8859-1",
		".fibertestparam":   "application/x-fiberparam; version=2",
		".fibertestboth":    "application/x-fiberboth; version=2; charset=utf-8",
		".fibertestcaps":    "text/x-fibercaps; Charset=UTF-8",
		".fibertestspacing": "text/x-fiberspacing ;  charset=utf-8 ; version=3",
	} {
		if err := mime.AddExtensionType(ext, mimeType); err != nil {
			panic(err)
		}
	}
})

// Whatever the machine's MIME database holds, no extension may produce two
// charsets, with or without an explicit one. utils.GetMIME hands back the bare
// type for an extension outside its table (utils v2.6.2 and later); with the
// parameters of the standard library's answer left on, "text/csv; charset=utf-8"
// plus the charset Type appends sent two.
func Test_Ctx_Type_NeverTwoCharsets(t *testing.T) {
	t.Parallel()
	registerTestMIMETypes()

	extensions := []string{
		"csv", "mjs", "md", "markdown", "ics", "vcf", "tsv", "go", "log", "vtt", "py", "c", "h", "java", "yaml", "toml",
		"html", "htm", "css", "js", "json", "xml", "txt", "svg", "wasm", "pdf", "png", "unknownext",
		"fibertestcsv", "fibertestlatin", "fibertestparam", "fibertestboth", "fibertestcaps", "fibertestspacing",
	}

	c := apptest.Ctx(t, New())

	for _, ext := range extensions {
		for _, charset := range [][]string{nil, {"utf-8"}, {"iso-8859-1"}} {
			c.Type(ext, charset...)
			contentType := string(c.Response().Header.Peek(HeaderContentType))
			require.LessOrEqual(t, strings.Count(strings.ToLower(contentType), "charset="), 1,
				"Type(%q, %v) sent %q", ext, charset, contentType)
			if len(charset) > 0 {
				require.Contains(t, contentType, "charset="+charset[0], "Type(%q, %v) sent %q", ext, charset, contentType)
			}
		}
	}
}

func Test_Ctx_Attachment_Charset(t *testing.T) {
	t.Parallel()
	registerTestMIMETypes()

	c := apptest.Ctx(t, New())

	c.Attachment("report.fibertestcsv")
	require.Equal(t, "text/x-fibercsv; charset=utf-8", string(c.Response().Header.Peek(HeaderContentType)))
	require.Equal(t, `attachment; filename="report.fibertestcsv"`, string(c.Response().Header.Peek(HeaderContentDisposition)))

	c.Attachment("notes.txt")
	require.Equal(t, "text/plain; charset=utf-8", string(c.Response().Header.Peek(HeaderContentType)))

	c.Attachment("image.png")
	require.Equal(t, "image/png", string(c.Response().Header.Peek(HeaderContentType)))
}

// Is and Accepts compare the type itself, so they match an extension only while
// utils.GetMIME answers it without parameters: "text/x-fibercsv; charset=utf-8"
// is not the "text/x-fibercsv" a request names.
func Test_Ctx_Is_And_Accepts_ExtensionWithCharset(t *testing.T) {
	t.Parallel()
	registerTestMIMETypes()

	c := apptest.Ctx(t, New())

	c.Request().Header.SetContentType("text/x-fibercsv; charset=utf-8")
	require.True(t, c.Is("fibertestcsv"))
	require.False(t, c.Is("fibertestlatin"))

	c.Request().Header.Set(HeaderAccept, "text/x-fibercsv")
	require.Equal(t, "fibertestcsv", c.Accepts("fibertestcsv"))
	require.Empty(t, c.Accepts("fibertestlatin"))

	c.Request().Header.Set(HeaderAccept, "text/*")
	require.Equal(t, "fibertestcsv", c.Accepts("fibertestcsv"))
}

func Test_shouldIncludeCharset(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		types []string
		want  bool // whether the types should include a charset
	}{
		{name: "text/* types", want: true, types: []string{"text/html", "text/plain", "text/css", "text/javascript", "text/xml"}},
		{name: "explicit application types", want: true, types: []string{"application/json", "application/javascript", "application/xml"}},
		{name: "+json suffixes", want: true, types: []string{
			"application/problem+json", "application/vnd.api+json", "application/hal+json", "application/merge-patch+json",
		}},
		{name: "+xml suffixes", want: true, types: []string{
			"application/soap+xml", "application/xhtml+xml", "application/atom+xml", "application/rss+xml",
		}},
		{name: "binary types", want: false, types: []string{
			"image/png", "image/jpeg", "application/pdf", "application/octet-stream", "video/mp4", "audio/mpeg",
		}},
		{name: "other application types", want: false, types: []string{
			"application/cbor", "application/x-www-form-urlencoded", "application/vnd.msgpack",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, mimeType := range tc.types {
				require.Equal(t, tc.want, shouldIncludeCharset(mimeType), mimeType)
			}
		})
	}
}

func Benchmark_Ctx_Type(b *testing.B) {
	c := apptest.Ctx(b, New())

	b.ReportAllocs()
	for b.Loop() {
		c.Type(".json")
		c.Type("json")
	}
}

func Benchmark_Ctx_Type_Charset(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())

	b.ReportAllocs()
	for b.Loop() {
		c.Type(".json", "utf-8")
		c.Type("json", "utf-8")
	}
}

// ctxTestCVaryCase is one Vary scenario: a response that already holds the
// field lines in lines, then one Vary call per entry of calls. The response
// must end up with exactly the field lines in want.
type ctxTestCVaryCase struct {
	name  string
	lines []string
	calls [][]string
	want  []string
}

// ctxTestCRunVary runs every case as a subtest, each on a context of its own.
func ctxTestCRunVary(t *testing.T, cases []ctxTestCVaryCase) {
	t.Helper()

	app := New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)
			for _, line := range tc.lines {
				c.Response().Header.Add(HeaderVary, line)
			}
			for _, fields := range tc.calls {
				c.Vary(fields...)
			}

			var got []string
			for _, line := range c.Response().Header.PeekAll(HeaderVary) {
				got = append(got, string(line))
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func Test_Ctx_Vary(t *testing.T) {
	t.Parallel()
	ctxTestCRunVary(t, []ctxTestCVaryCase{
		{
			name:  "fields are listed in order",
			calls: [][]string{{"Origin"}, {"User-Agent"}, {"Accept-Encoding", "Accept"}},
			want:  []string{"Origin, User-Agent, Accept-Encoding, Accept"},
		},
	})
}

// Test_Ctx_Vary_Wildcard verifies the RFC 9110 §12.5.5 rule that "*" is only
// meaningful as the sole member of the Vary field value.
func Test_Ctx_Vary_Wildcard(t *testing.T) {
	t.Parallel()
	ctxTestCRunVary(t, []ctxTestCVaryCase{
		// Adding "*" collapses any previously listed fields to a single "*".
		{name: "wildcard collapses the fields", calls: [][]string{{"Origin"}, {"*"}}, want: []string{"*"}},
		// Once "*" is present, further fields are not appended alongside it.
		{name: "no field joins the wildcard", calls: [][]string{{"Origin"}, {"*"}, {"Accept"}}, want: []string{"*"}},
		// "*" mixed into a multi-field call also collapses the header.
		{name: "wildcard in a multi-field call", calls: [][]string{{"Accept-Encoding", "*", "Accept"}}, want: []string{"*"}},
		// A pre-existing mixed value containing "*" (set outside Vary) collapses
		// on the next Vary call, matching the documented invariant.
		{name: "wildcard set outside Vary", lines: []string{"User-Agent, *"}, calls: [][]string{{"Origin"}}, want: []string{"*"}},
		// Multiple Vary field lines (via Header.Add) are treated as one combined
		// list (RFC 9110 §5.2): a wildcard on a later line still collapses the
		// whole header, leaving a single field line.
		{name: "wildcard on a later field line", lines: []string{"Accept", "*"}, calls: [][]string{{"Origin"}}, want: []string{"*"}},
		// Dedup also sees members on later field lines: adding an
		// already-listed member is a no-op (already present on the second line:
		// no rewrite), ...
		{
			name: "member listed on a later field line", lines: []string{"Accept", "Origin"},
			calls: [][]string{{"Origin"}}, want: []string{"Accept", "Origin"},
		},
		// ... and a real addition folds all the lines into a single combined
		// field line.
		{
			name: "addition folds the field lines", lines: []string{"Accept", "Origin"},
			calls: [][]string{{"Origin"}, {"Accept-Encoding"}}, want: []string{"Accept,Origin, Accept-Encoding"},
		},
	})
}

// Test_Ctx_Vary_CaseInsensitive verifies RFC 9110 §5.1: Vary members are field
// names, so they are compared case-insensitively and the first spelling is kept.
func Test_Ctx_Vary_CaseInsensitive(t *testing.T) {
	t.Parallel()
	ctxTestCRunVary(t, []ctxTestCVaryCase{
		{name: "lowercase first", calls: [][]string{{"accept"}, {"Accept"}}, want: []string{"accept"}},
		{name: "canonical case first", calls: [][]string{{"Accept-Encoding"}, {"accept-encoding"}}, want: []string{"Accept-Encoding"}},
		{name: "both cases in one call", calls: [][]string{{"accept", "Accept"}}, want: []string{"accept"}},
		// A case-insensitive match on a later field line is still a no-op.
		{name: "match on a later field line", lines: []string{"Accept", "Origin"}, calls: [][]string{{"origin"}}, want: []string{"Accept", "Origin"}},
	})

	// AutoFormat adds Vary("Accept"); a lowercase listing must not be duplicated.
	t.Run("AutoFormat", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, New())
		c.Request().Header.Set(HeaderAccept, MIMETextPlain)
		c.Vary("accept")
		require.NoError(t, c.AutoFormat("hello"))
		require.Equal(t, "accept", string(c.Response().Header.Peek("Vary")))
	})
}

func Benchmark_Ctx_Vary(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())

	b.ReportAllocs()
	for b.Loop() {
		c.Vary("Origin", "User-Agent")
	}
}

func Test_Ctx_Write(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	hello, world := []byte("Hello, "), []byte("World!")
	_, err := c.Write(hello)
	require.NoError(t, err)
	_, err = c.Write(world)
	require.NoError(t, err)
	require.Equal(t, "Hello, World!", string(c.Response().Body()))
}

func Benchmark_Ctx_Write(b *testing.B) {
	c := apptest.Ctx(b, New())

	byt := []byte("Hello, World!")
	b.ReportAllocs()

	var err error
	for b.Loop() {
		_, err = c.Write(byt)
	}
	require.NoError(b, err)
}

func Test_Ctx_Writef(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	world := "World!"
	_, err := c.Writef("Hello, %s", world)
	require.NoError(t, err)
	require.Equal(t, "Hello, World!", string(c.Response().Body()))
}

func Benchmark_Ctx_Writef(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())

	world := "World!"
	b.ReportAllocs()

	var err error
	for b.Loop() {
		_, err = c.Writef("Hello, %s", world)
	}
	require.NoError(b, err)
}

func Test_Ctx_WriteString(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	_, err := c.WriteString("Hello, ")
	require.NoError(t, err)
	_, err = c.WriteString("World!")
	require.NoError(t, err)
	require.Equal(t, "Hello, World!", string(c.Response().Body()))
}

func Test_Ctx_XHR(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().Header.Set(HeaderXRequestedWith, "XMLHttpRequest")
	require.True(t, c.XHR())
}

func Benchmark_Ctx_XHR(b *testing.B) {
	c := apptest.Ctx(b, New())

	c.Request().Header.Set(HeaderXRequestedWith, "XMLHttpRequest")
	var equal bool
	b.ReportAllocs()
	for b.Loop() {
		equal = c.XHR()
	}
	require.True(b, equal)
}

// headerReadBenchCtx builds the request a browser sends, eight field lines, for
// the header reads below. Names are stored the way the peer spelled them, so the
// non-normalizing case exercises the case-insensitive walk rather than a store
// fasthttp has already canonicalized.
//
//nolint:revive // flag-parameter: this selects which header store shape is measured
func headerReadBenchCtx(b *testing.B, disableNormalizing bool) Ctx {
	b.Helper()

	c := apptest.Ctx(b, New(Config{DisableHeaderNormalizing: disableNormalizing}))

	fields := [][2]string{
		{HeaderUserAgent, "Mozilla/5.0"},
		{HeaderAccept, "text/html,application/xhtml+xml"},
		{HeaderAcceptLanguage, "en-US,en;q=0.9"},
		{HeaderAcceptEncoding, "gzip, deflate, br"},
		{HeaderConnection, "keep-alive"},
		{HeaderCacheControl, "max-age=0"},
		{"Sec-Fetch-Site", "same-origin"},
		{HeaderXRequestID, "3f0c1a"},
	}
	if disableNormalizing {
		c.Request().Header.DisableNormalizing()
		for _, f := range fields {
			c.Request().Header.Set(utilsstrings.ToLower(f[0]), f[1])
		}
		return c
	}

	for _, f := range fields {
		c.Request().Header.Set(f[0], f[1])
	}
	return c
}

func benchHeaderReadModes(b *testing.B, f func(b *testing.B, c Ctx)) {
	b.Helper()

	b.Run("Normalizing", func(b *testing.B) {
		c := headerReadBenchCtx(b, false)
		b.ReportAllocs()
		f(b, c)
	})
	b.Run("WithoutNormalizing", func(b *testing.B) {
		c := headerReadBenchCtx(b, true)
		b.ReportAllocs()
		f(b, c)
	})
}

func Benchmark_Ctx_Get_Header(b *testing.B) {
	benchHeaderReadModes(b, func(b *testing.B, c Ctx) {
		b.Helper()
		var v string
		for b.Loop() {
			v = c.Get(HeaderXRequestID)
		}
		require.Equal(b, "3f0c1a", v)
	})
}

func Benchmark_Ctx_Get_Header_Canonical(b *testing.B) {
	// A key already in canonical form, unlike X-Request-ID above.
	benchHeaderReadModes(b, func(b *testing.B, c Ctx) {
		b.Helper()
		var v string
		for b.Loop() {
			v = c.Get(HeaderCacheControl)
		}
		require.Equal(b, "max-age=0", v)
	})
}

func Benchmark_Ctx_Get_HeaderAbsent(b *testing.B) {
	benchHeaderReadModes(b, func(b *testing.B, c Ctx) {
		b.Helper()
		var v string
		for b.Loop() {
			v = c.Get("X-Not-Sent")
		}
		require.Empty(b, v)
	})
}

func Benchmark_Ctx_HasHeaderValue(b *testing.B) {
	benchHeaderReadModes(b, func(b *testing.B, c Ctx) {
		b.Helper()
		var ok bool
		for b.Loop() {
			ok = c.HasHeaderValue(HeaderConnection, "keep-alive")
		}
		require.True(b, ok)
	})
}

func Benchmark_Ctx_HasHeader(b *testing.B) {
	benchHeaderReadModes(b, func(b *testing.B, c Ctx) {
		b.Helper()
		var ok bool
		for b.Loop() {
			ok = c.HasHeader(HeaderXRequestID)
		}
		require.True(b, ok)
	})
}

func Benchmark_Ctx_HasHeaderAbsent(b *testing.B) {
	benchHeaderReadModes(b, func(b *testing.B, c Ctx) {
		b.Helper()
		var ok bool
		for b.Loop() {
			ok = c.HasHeader("X-Not-Sent")
		}
		require.False(b, ok)
	})
}

func Benchmark_Ctx_GetReqHeader(b *testing.B) {
	benchHeaderReadModes(b, func(b *testing.B, c Ctx) {
		b.Helper()
		var v string
		for b.Loop() {
			v = GetReqHeader[string](c, HeaderXRequestID)
		}
		require.Equal(b, "3f0c1a", v)
	})
}

// Covered under both settings because the two paths differ in kind, not degree:
// the canonical one peeks, while the case-insensitive one walks the store and
// collects the field lines, which allocates.
func Benchmark_Ctx_IsWebSocket(b *testing.B) {
	for _, tc := range []struct {
		name               string
		disableNormalizing bool
		handshake          bool
	}{
		{"Normalizing/handshake", false, true},
		{"Normalizing/no-upgrade", false, false},
		{"WithoutNormalizing/handshake", true, true},
		{"WithoutNormalizing/no-upgrade", true, false},
	} {
		b.Run(tc.name, func(b *testing.B) {
			c := apptest.Ctx(b, New(Config{DisableHeaderNormalizing: tc.disableNormalizing}))

			connection, upgrade := HeaderConnection, HeaderUpgrade
			if tc.disableNormalizing {
				c.Request().Header.DisableNormalizing()
				connection, upgrade = utilsstrings.ToLower(connection), utilsstrings.ToLower(upgrade)
			}
			c.Request().Header.Set(connection, "upgrade")
			if tc.handshake {
				c.Request().Header.Set(upgrade, "websocket")
			}

			b.ReportAllocs()
			var got bool
			for b.Loop() {
				got = c.IsWebSocket()
			}
			require.Equal(b, tc.handshake, got)
		})
	}
}

func Benchmark_Ctx_SendString_B(b *testing.B) {
	c := apptest.Ctx(b, New())

	body := "Hello, world!"
	b.ReportAllocs()

	var err error
	for b.Loop() {
		err = c.SendString(body)
	}
	require.NoError(b, err)
	require.Equal(b, []byte("Hello, world!"), c.Response().Body())
}

func Test_Ctx_Queries(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")

	// The query string is replaced between the steps, on one context: each step
	// must answer for its own query string and for nothing left from the step
	// before.
	for _, tc := range []struct {
		want  map[string]string
		query string
	}{
		{
			query: "id=1&name=tom&hobby=basketball,football&favouriteDrinks=milo,coke,pepsi&alloc=&no=1&field1=value1&field1=value2&field2=value3&list_a=1&list_a=2&list_a=3&list_b[]=1&list_b[]=2&list_b[]=3&list_c=1,2,3",
			want: map[string]string{
				"id": "1", "name": "tom", "hobby": "basketball,football", "favouriteDrinks": "milo,coke,pepsi", "alloc": "", "no": "1",
				"field1": "value2", "field2": "value3", "list_a": "3", "list_b[]": "3", "list_c": "1,2,3",
			},
		},
		{
			query: "filters.author.name=John&filters.category.name=Technology&filters[customer][name]=Alice&filters[status]=pending",
			want: map[string]string{
				"filters.author.name": "John", "filters.category.name": "Technology",
				"filters[customer][name]": "Alice", "filters[status]": "pending",
			},
		},
		{
			query: "tags=apple,orange,banana&filters[tags]=apple,orange,banana&filters[category][name]=fruits&filters.tags=apple,orange,banana&filters.category.name=fruits",
			want: map[string]string{
				"tags": "apple,orange,banana", "filters[tags]": "apple,orange,banana", "filters[category][name]": "fruits",
				"filters.tags": "apple,orange,banana", "filters.category.name": "fruits",
			},
		},
		{
			query: "filters[tags][0]=apple&filters[tags][1]=orange&filters[tags][2]=banana&filters[category][name]=fruits",
			want: map[string]string{
				"filters[tags][0]": "apple", "filters[tags][1]": "orange", "filters[tags][2]": "banana", "filters[category][name]": "fruits",
			},
		},
	} {
		c.Request().URI().SetQueryString(tc.query)

		require.Equal(t, tc.want, c.Queries(), tc.query)
		require.Equal(t, tc.want, c.Req().Queries(), tc.query)
	}
}

func Benchmark_Ctx_Queries(b *testing.B) {
	c := apptest.Ctx(b, New())

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

	require.Equal(t, "body writer line 1\nbody writer line 2\n", string(resp.Body()))
}

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
	c := apptest.Ctx(t, New())

	require.Equal(t, "#0000000000000000 - 0.0.0.0:0 <-> 0.0.0.0:0 - GET http:///", c.String())
}

func Benchmark_Ctx_String(b *testing.B) {
	var str string
	ctx := apptest.Ctx(b, New())
	b.ReportAllocs()

	for b.Loop() {
		str = ctx.String()
	}
	require.Equal(b, "#0000000000000000 - 0.0.0.0:0 <-> 0.0.0.0:0 - GET http:///", str)
}

// X-Forwarded-For is not trusted unless Config.TrustProxy says so, so it never
// makes a request local.
func Test_Ctx_IsFromLocal_X_Forwarded(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		forwardedFor string // left out when empty
	}{
		{name: "unset"},
		{name: "localhost", forwardedFor: "127.0.0.1"},
		{name: "IPv6 localhost", forwardedFor: "::1"},
		{name: "full IPv6 localhost", forwardedFor: "0:0:0:0:0:0:0:1"},
		{name: "random IP address", forwardedFor: "93.46.8.90"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New())
			if tc.forwardedFor != "" {
				c.Request().Header.Set(HeaderXForwardedFor, tc.forwardedFor)
			}

			// fasthttp returns "0.0.0.0" as IP as there is no remote address.
			require.Equal(t, "0.0.0.0", c.IP())
			require.False(t, c.IsFromLocal())
			require.False(t, c.Req().IsFromLocal())
		})
	}
}

func Test_Ctx_RemoteAddr(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		addr       net.Addr
		name       string
		wantIP     string
		wantLocal  bool
		wantSocket bool
	}{
		{name: "127.0.0.1", addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1")}, wantIP: "127.0.0.1", wantLocal: true},
		{name: "::1", addr: &net.TCPAddr{IP: net.ParseIP("::1")}, wantIP: "::1", wantLocal: true},
		// fasthttp should return "::1" for "0:0:0:0:0:0:0:1".
		// otherwise IsFromLocal() will break.
		{name: "0:0:0:0:0:0:0:1", addr: &net.TCPAddr{IP: net.ParseIP("0:0:0:0:0:0:0:1")}, wantIP: "::1", wantLocal: true},
		{name: "0.0.0.0", addr: &net.TCPAddr{IP: net.IPv4zero}, wantIP: "0.0.0.0"},
		{name: "93.46.8.90", addr: &net.TCPAddr{IP: net.ParseIP("93.46.8.90")}, wantIP: "93.46.8.90"},
		{
			name: "2001:0db8:85a3:0000:0000:8a2e:0370:7334", addr: &net.TCPAddr{IP: net.ParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334")},
			wantIP: "2001:db8:85a3::8a2e:370:7334",
		},
		// IsFromLocal only treats loopback IPs as local.
		{name: "Unix socket", addr: &net.UnixAddr{Name: "/tmp/fiber.sock", Net: "unix"}, wantIP: "0.0.0.0", wantSocket: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fctx := &fasthttp.RequestCtx{}
			fctx.SetRemoteAddr(tc.addr)
			c := apptest.CtxFor(t, New(), fctx)

			require.Equal(t, tc.wantIP, c.IP())
			require.Equal(t, tc.wantIP, c.Req().IP())
			require.Equal(t, tc.wantLocal, c.IsFromLocal())
			require.Equal(t, tc.wantLocal, c.Req().IsFromLocal())
			require.Equal(t, tc.wantSocket, c.IsFromUnixSocket())
			require.Equal(t, tc.wantSocket, c.Req().IsFromUnixSocket())
		})
	}
}

func Test_Ctx_extractIPsFromHeader(t *testing.T) {
	t.Parallel()
	for _, validate := range []bool{false, true} {
		t.Run(fmt.Sprintf("EnableIPValidation=%t", validate), func(t *testing.T) {
			t.Parallel()
			app := New()
			app.config.EnableIPValidation = validate
			c := apptest.Ctx(t, app)
			c.Request().Header.Set("x-forwarded-for", "1.1.1.1,8.8.8.8 , /n, \n,1.1, a.c, 6.,6., , a,,42.118.81.169,10.0.137.108")
			ips := c.IPs()
			require.Equal(t, "42.118.81.169", ips[len(ips)-2])
		})
	}
}

func Test_Ctx_GetRespHeaders(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Set("test", "Hello, World 👋!")
	c.Set("foo", "bar")
	c.Response().Header.Set("multi", "one")
	c.Response().Header.Add("multi", "two")
	c.Response().Header.Set(HeaderContentType, "application/json")

	want := map[string][]string{
		"Content-Type": {"application/json"},
		"Foo":          {"bar"},
		"Multi":        {"one", "two"},
		"Test":         {"Hello, World 👋!"},
	}
	require.Equal(t, want, c.GetRespHeaders())
	require.Equal(t, want, c.Res().GetHeaders())
}

func Benchmark_Ctx_GetRespHeaders(b *testing.B) {
	c := apptest.Ctx(b, New())

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

func Test_Ctx_GetReqHeaders(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().Header.Set("test", "Hello, World 👋!")
	c.Request().Header.Set("foo", "bar")
	c.Request().Header.Set("multi", "one")
	c.Request().Header.Add("multi", "two")
	c.Request().Header.Set(HeaderContentType, "application/json")

	want := map[string][]string{
		"Content-Type": {"application/json"},
		"Foo":          {"bar"},
		"Test":         {"Hello, World 👋!"},
		"Multi":        {"one", "two"},
	}
	require.Equal(t, want, c.GetReqHeaders())
	require.Equal(t, want, c.GetHeaders())
}

func Test_Ctx_GetReqHeaders_TrailerSection(t *testing.T) {
	t.Parallel()
	for _, normalize := range []bool{true, false} {
		t.Run(fmt.Sprintf("normalize=%v", normalize), func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, New(Config{DisableHeaderNormalizing: !normalize}))
			if !normalize {
				c.Request().Header.DisableNormalizing()
			}
			// A request read outside the router keeps the section, as a streamed body does past the cap.
			raw := "POST / HTTP/1.1\r\nHost: example.com\r\nX-Kept: 1\r\nx-user: admin\r\nTransfer-Encoding: chunked\r\n\r\n" +
				"5\r\nhello\r\n0\r\nX-User: x\r\nX-Role: x\r\n\r\n"
			require.NoError(t, c.Request().Read(bufio.NewReader(strings.NewReader(raw))))
			require.Len(t, c.Request().Header.PeekTrailerKeys(), 2)

			headers := c.GetReqHeaders()
			require.Equal(t, []string{"1"}, headers["X-Kept"])
			for name := range headers {
				require.NotContains(t, []string{"x-user", "x-role"}, strings.ToLower(name))
			}
		})
	}
}

func Benchmark_Ctx_GetReqHeaders_TrailerSection(b *testing.B) {
	for _, n := range []int{256, 4096} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			c := apptest.Ctx(b, New())

			raw := "POST / HTTP/1.1\r\nHost: example.com\r\n" + rawFieldLines("H", "1", n) +
				"Transfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n" + rawFieldLines("T", "1", n) + "\r\n"
			require.NoError(b, c.Request().Read(bufio.NewReaderSize(strings.NewReader(raw), len(raw))))
			require.Len(b, c.Request().Header.PeekTrailerKeys(), n)

			b.ReportAllocs()
			var headers map[string][]string
			for b.Loop() {
				headers = c.GetReqHeaders()
			}
			require.Contains(b, headers, "H0")
		})
	}
}

func Test_Ctx_Set_SanitizeHeaderValue(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Set("X-Test", "foo\r\nbar: bad")
	require.Equal(t, "foo  bar: bad", string(c.Response().Header.Peek("X-Test")))
}

func Benchmark_Ctx_GetReqHeaders(b *testing.B) {
	c := apptest.Ctx(b, New())

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

// ctxTestCNoResponse sends a GET for target through app and requires that it
// fails with wantErr and without a response, as a dropped or timed out
// connection does.
func ctxTestCNoResponse(t *testing.T, app *App, target string, wantErr error) {
	t.Helper()

	resp, err := app.Test(httptest.NewRequest(MethodGet, target, http.NoBody))
	require.ErrorIs(t, err, wantErr)
	require.Nil(t, resp)
}

func Test_Ctx_Drop(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/block-me", func(c Ctx) error { return c.Drop() })
	// a handler that returns without writing still gets its response
	app.Get("/no-response", func(_ Ctx) error { return nil })

	ctxTestCNoResponse(t, app, "/block-me", ErrTestGotEmptyResponse)

	resp := apptest.Do(t, app, "/no-response")
	require.NotNil(t, resp)
	require.Equal(t, StatusOK, resp.StatusCode)
	require.Equal(t, "0", resp.Header.Get("Content-Length"))
}

func Test_Ctx_DropWithMiddleware(t *testing.T) {
	t.Parallel()
	app := New()
	// a middleware that writes a header after the handler dropped the connection
	app.Use(func(c Ctx) error {
		err := c.Next()
		c.Set("X-Test", "test")
		return err
	})
	app.Get("/block-me", func(c Ctx) error { return c.Drop() })

	ctxTestCNoResponse(t, app, "/block-me", ErrTestGotEmptyResponse)
}

func Test_Ctx_End(t *testing.T) {
	app := New()

	app.Get("/", func(c Ctx) error {
		c.SendString("Hello, World!") //nolint:errcheck // unnecessary to check error
		return c.End()
	})

	resp := apptest.Do(t, app, "/")
	require.NotNil(t, resp)
	require.Equal(t, StatusOK, resp.StatusCode)
	require.Equal(t, "Hello, World!", apptest.Body(t, resp))
}

func Test_Ctx_End_after_timeout(t *testing.T) {
	app := New()

	// Early flushing handler
	app.Get("/", func(c Ctx) error {
		time.Sleep(2 * time.Second)
		return c.End()
	})

	ctxTestCNoResponse(t, app, "/", os.ErrDeadlineExceeded)
}

func Test_Ctx_End_with_drop_middleware(t *testing.T) {
	app := New()

	// Middleware that will drop connections that persist after c.Next()
	app.Use(func(c Ctx) error {
		c.Next() //nolint:errcheck // unnecessary to check error
		return c.Drop()
	})

	// Early flushing handler
	app.Get("/", func(c Ctx) error {
		c.SendStatus(StatusOK) //nolint:errcheck // unnecessary to check error
		return c.End()
	})

	resp := apptest.Do(t, app, "/")
	require.NotNil(t, resp)
	require.Equal(t, StatusOK, resp.StatusCode)
}

func Test_Ctx_End_after_drop(t *testing.T) {
	app := New()

	// Middleware that ends the request after c.Next()
	app.Use(func(c Ctx) error {
		c.Next() //nolint:errcheck // unnecessary to check error
		return c.End()
	})
	app.Get("/", func(c Ctx) error { return c.Drop() })

	ctxTestCNoResponse(t, app, "/", ErrTestGotEmptyResponse)
}

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

			return c.JSON(map[string]any{"name": c.Params("name"), "id": c.Params("id"), "all": c.Route().Params})
		})

		apptest.Expect(t, app, "/user/original/123", StatusOK)
	})

	for _, tc := range []struct {
		config    Config
		overrides map[string]string // param name -> the value to override it with
		want      map[string]string // param name -> what Params must answer afterwards
		name      string
		pattern   string
		path      string
	}{
		{
			name: "plus_wildcard_params", pattern: "/files+/+", path: "/filesoriginal/original2",
			overrides: map[string]string{"+": "changed", "+2": "changed2"},
			want:      map[string]string{"+": "changed", "+2": "changed2"},
		},
		{
			name: "wildcard_params", pattern: "/files/*", path: "/files/testing",
			overrides: map[string]string{"*": "changed"},
			want:      map[string]string{"*": "changed"},
		},
		{
			name: "multi_wildcard_params", pattern: "/files/*/*", path: "/files/testing/testing",
			overrides: map[string]string{"*": "changed", "*2": "changed2"},
			want:      map[string]string{"*": "changed", "*2": "changed2"},
		},
		// Ensure OverrideParam respects the CaseSensitive configuration
		{
			name: "case_sensitive", config: Config{CaseSensitive: true}, pattern: "/user/:name", path: "/user/original",
			overrides: map[string]string{"name": "overridden"},
			want:      map[string]string{"name": "overridden", "NAME": ""},
		},
		// CaseInsensitive mode (default)
		{
			name: "case_insensitive", config: Config{CaseSensitive: false}, pattern: "/user/:name", path: "/user/original",
			overrides: map[string]string{"NAME": "overridden"},
			want:      map[string]string{"name": "overridden", "NAME": "overridden"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New(tc.config)
			app.Get(tc.pattern, func(c Ctx) error {
				for name, value := range tc.overrides {
					c.OverrideParam(name, value)
				}
				for name, value := range tc.want {
					require.Equal(t, value, c.Params(name), name)
				}
				return nil
			})

			apptest.Expect(t, app, tc.path, StatusOK)
		})
	}

	t.Run("nil_router", func(t *testing.T) {
		t.Parallel()
		// Ensure OverrideParam handles nil route context gracefully
		c := apptest.CtxAs[*DefaultCtx](t, New())
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

	t.Cleanup(func() { ctx.ForceRelease() })

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

// The b.RunParallel cases must keep the "_Parallel" in their names: CI drops
// results matching Benchmark.*_Parallel before comparing, and at a few ns/op
// these measure available cores rather than the code.
func Benchmark_Ctx_IsProxyTrusted(b *testing.B) {
	// sequential measures IsProxyTrusted on a context of an app built from config.
	sequential := func(name string, config Config) {
		b.Run(name, func(b *testing.B) {
			c := apptest.Ctx(b, New(config))
			c.Request().SetRequestURI("http://google.com/test")
			c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
			b.ReportAllocs()
			for b.Loop() {
				c.IsProxyTrusted()
			}
		})
	}
	// parallel measures IsProxyTrusted with one context per goroutine.
	parallel := func(name string, config Config) {
		b.Run(name, func(b *testing.B) {
			app := New(config)
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
	trust := func(config TrustProxyConfig) Config { return Config{TrustProxy: true, TrustProxyConfig: config} }
	proxies := func(ips ...string) Config { return trust(TrustProxyConfig{Proxies: ips}) }

	b.Run("NoProxyCheck", func(b *testing.B) {
		c := apptest.Ctx(b, New())
		c.Request().SetRequestURI("http://google.com:8080/test")
		b.ReportAllocs()
		for b.Loop() {
			c.IsProxyTrusted()
		}
	})
	b.Run("NoProxyCheck_Parallel", func(b *testing.B) {
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

	sequential("WithProxyCheckSimple", Config{TrustProxy: true})
	parallel("WithProxyCheckSimple_Parallel", Config{TrustProxy: true})
	sequential("WithProxyCheck", proxies("0.0.0.0"))
	parallel("WithProxyCheck_Parallel", proxies("0.0.0.0"))

	// Scenario with CIDR-only trust config (empty exact-IP map). The remote IP
	// reaches the range check, exercising the guard that skips ip.String().
	sequential("WithProxyCheckCIDR", proxies("0.0.0.0/8"))

	sequential("WithProxyCheckAllowPrivate", trust(TrustProxyConfig{Private: true}))
	parallel("WithProxyCheckAllowPrivate_Parallel", trust(TrustProxyConfig{Private: true}))
	privateSubnets := proxies("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7")
	sequential("WithProxyCheckAllowPrivateAsSubnets", privateSubnets)
	parallel("WithProxyCheckAllowPrivateAsSubnets_Parallel", privateSubnets)

	// private, loopback and link-local, as flags and as subnets
	allowAll := trust(TrustProxyConfig{Private: true, Loopback: true, LinkLocal: true})
	sequential("WithProxyCheckAllowAll", allowAll)
	parallel("WithProxyCheckAllowAll_Parallel", allowAll)
	allowAllSubnets := proxies(
		"169.254.0.0/16", "fe80::/10", // Link-local
		"127.0.0.0/8", "::1/128", // Loopback
		"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7", // Private
	)
	b.Run("WithProxyCheckAllowAllowAllAsSubnets", func(b *testing.B) {
		c := apptest.Ctx(b, New(allowAllSubnets))
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			c.IsProxyTrusted()
		}
	})
	parallel("WithProxyCheckAllowAllowAllAsSubnets_Parallel", allowAllSubnets)

	sequential("WithProxyCheckSubnet", proxies("0.0.0.0/8"))
	parallel("WithProxyCheck_ParallelSubnet", proxies("0.0.0.0/8"))
	sequential("WithProxyCheckMultipleSubnet", proxies("192.168.0.0/24", "10.0.0.0/16", "0.0.0.0/8"))
	parallel("WithProxyCheck_ParallelMultipleSubnet", proxies("192.168.0.0/24", "10.0.0.0/16", "0.0.0.0/8"))
	allSubnets := proxies(
		"127.0.0.0/8",     // Loopback addresses
		"169.254.0.0/16",  // Link-Local addresses
		"fe80::/10",       // Link-Local addresses
		"192.168.0.0/16",  // Private Network addresses
		"172.16.0.0/12",   // Private Network addresses
		"10.0.0.0/8",      // Private Network addresses
		"fc00::/7",        // Unique Local addresses
		"173.245.48.0/20", // My custom range
		"0.0.0.0/8",       // All IPv4 addresses
	)
	sequential("WithProxyCheckAllSubnets", allSubnets)
	parallel("WithProxyCheck_ParallelAllSubnets", allSubnets)
}

func Benchmark_Ctx_IsFromLocalhost(b *testing.B) {
	for _, bc := range []struct{ name, uri string }{
		{name: "Non_Localhost", uri: "http://google.com:8080/test"},
		{name: "Localhost", uri: "http://localhost:8080/test"},
	} {
		b.Run(bc.name, func(b *testing.B) {
			c := apptest.Ctx(b, New())
			c.Request().SetRequestURI(bc.uri)
			b.ReportAllocs()
			for b.Loop() {
				c.IsFromLocal()
			}
		})
	}
}

func Benchmark_Ctx_OverrideParam(b *testing.B) {
	c := apptest.CtxAs[*DefaultCtx](b, New())
	c.values = [maxParams]string{"original", "12345"}
	c.route = &Route{Params: []string{"name", "id"}}
	c.setMatched(true)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.OverrideParam("name", "changed")
	}
}

// Test_Ctx_Cookie_DoesNotMutateArgument verifies that Cookie treats its
// argument as read-only, so a caller can reuse the same *Cookie template
// across requests without the normalization leaking back into it.
//
// Note the deliberate absence of Partitioned: fasthttp's SetPartitioned also
// forces path=/ and secure, so a partitioned cookie would satisfy the emitted
// header assertions below even if the normalization were removed entirely.
func Test_Ctx_Cookie_DoesNotMutateArgument(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	tmpl := &Cookie{
		Name:        "session",
		Value:       "v",
		SameSite:    CookieSameSiteNoneMode,
		SessionOnly: true,
		MaxAge:      60,
		Expires:     time.Now().Add(time.Hour),
	}
	before := *tmpl

	c.Cookie(tmpl)

	require.Equal(t, before, *tmpl, "Cookie must not mutate the caller's struct")
	require.Empty(t, tmpl.Path)
	require.False(t, tmpl.Secure)
	require.Equal(t, 60, tmpl.MaxAge)

	// The emitted cookie still carries the normalized attributes. These are
	// regression guards, not proof of the normalization: fasthttp defaults an
	// empty path to "/" and forces Secure for SameSite=None on its own, so
	// they would hold even without it. The require.Equal above is the
	// assertion that actually pins this method's contract.
	header := string(c.Response().Header.Peek(HeaderSetCookie))
	require.Contains(t, header, "path=/")
	require.Contains(t, header, "secure")
	require.Contains(t, header, "SameSite=None")
	require.NotContains(t, header, "max-age=")

	// Partitioned is normalized on the copy too; assert on the struct rather
	// than the header, since fasthttp sets Secure and Path for it anyway.
	part := &Cookie{Name: "p", Value: "v", Path: "/scoped", Partitioned: true}
	c.Cookie(part)
	require.False(t, part.Secure, "Partitioned must not write Secure back to the caller")
	require.Equal(t, "/scoped", part.Path)
}

// sendFileCache returns the SendFile handlers app keeps, in the order it created them.
func sendFileCache(app *App) []*sendFileStore {
	app.sendfilesMutex.RLock()
	defer app.sendfilesMutex.RUnlock()
	return slices.Clone(app.sendfiles)
}

func Test_Ctx_SendFile_UncomparableFS(t *testing.T) {
	t.Parallel()
	app := New()
	first := fstest.MapFS{"hello.txt": &fstest.MapFile{Data: []byte("hello")}}
	second := fstest.MapFS{"hello.txt": &fstest.MapFile{Data: []byte("other")}}
	app.Get("/", func(c Ctx) error { return c.SendFile("hello.txt", SendFile{FS: first}) })
	app.Get("/other", func(c Ctx) error { return c.SendFile("hello.txt", SendFile{FS: second}) })

	for _, tc := range []struct{ path, body string }{
		{path: "/", body: "hello"},
		{path: "/", body: "hello"},
		{path: "/other", body: "other"},
		{path: "/", body: "hello"},
	} {
		apptest.ExpectBody(t, app, tc.path, StatusOK, tc.body)
	}
}

func Test_Ctx_SendFile_FunctionFSDoesNotGrowHandlerCache(t *testing.T) {
	t.Parallel()
	app := New()
	files := fstest.MapFS{"hello.txt": &fstest.MapFile{Data: []byte("hello")}}
	open := funcFS(files.Open)
	app.Get("/", func(c Ctx) error { return c.SendFile("hello.txt", SendFile{FS: open}) })

	for range 10 {
		apptest.ExpectBody(t, app, "/", StatusOK, "hello")
	}

	require.Empty(t, sendFileCache(app))
}

func Test_Ctx_SendFile_FreshConfigPerCallDoesNotGrowHandlerCache(t *testing.T) {
	t.Parallel()
	files := fstest.MapFS{"public/hello.txt": &fstest.MapFile{Data: []byte("hello")}}
	for name, handler := range map[string]Handler{
		"file system": func(c Ctx) error {
			public, err := fs.Sub(files, "public") // a new pointer on every call
			if err != nil {
				return fmt.Errorf("sub: %w", err)
			}
			return c.SendFile("hello.txt", SendFile{FS: public})
		},
		"max age": func(c Ctx) error {
			return c.SendFile("public/hello.txt", SendFile{FS: files, MaxAge: Query[int](c, "age")})
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Get("/", handler)
			app.Get("/kept", func(c Ctx) error {
				return c.SendFile("public/hello.txt", SendFile{FS: files, Download: true})
			})

			for i := range 2 * maxSendFileHandlers {
				for _, target := range []string{"/kept", "/?age=" + strconv.Itoa(i)} {
					apptest.ExpectBody(t, app, target, StatusOK, "hello")
				}
			}

			cache := sendFileCache(app)
			require.Len(t, cache, maxSendFileHandlers)
			require.True(t, cache[0].config.Download, "a handler kept before the cache filled up stays")
		})
	}
}

func Test_Ctx_SendFile_KeepsAcceptEncoding(t *testing.T) {
	t.Parallel()
	app := New()
	var seen string
	app.Use(func(c Ctx) error {
		err := c.Next()
		seen = c.Get(HeaderAcceptEncoding)
		return err
	})
	app.Get("/", func(c Ctx) error { return c.SendFile("./ctx.go") })

	resp := apptest.Expect(t, app, apptest.Req(MethodGet, "/", apptest.WithHeader(HeaderAcceptEncoding, "gzip")), StatusOK)
	require.Equal(t, "gzip", seen)
	require.Empty(t, resp.Header.Get(HeaderContentEncoding), "Compress is off, so the file is served as-is")
}

func Test_Ctx_SendFile_CompressedByMiddleware(t *testing.T) {
	t.Parallel()
	app := New()
	// compress decides on the request's Accept-Encoding after Next, so SendFile must leave it.
	compressor := fasthttp.CompressHandlerBrotliLevel(func(*fasthttp.RequestCtx) {},
		fasthttp.CompressBrotliDefaultCompression, fasthttp.CompressDefaultCompression)
	app.Use(func(c Ctx) error {
		if err := c.Next(); err != nil {
			return err
		}
		compressor(c.RequestCtx())
		return nil
	})
	app.Get("/", func(c Ctx) error { return c.SendFile("./ctx.go") })

	resp := apptest.Expect(t, app, apptest.Req(MethodGet, "/", apptest.WithHeader(HeaderAcceptEncoding, "gzip")), StatusOK)
	require.Equal(t, "gzip", resp.Header.Get(HeaderContentEncoding))
}

func Test_Ctx_AutoFormat_NoAcceptHeader(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error { return c.AutoFormat("Hello, World!") })

	testCases := []struct {
		name        string
		accept      string
		contentType string
		body        string
	}{
		{name: "no header", accept: "", contentType: "text/plain; charset=utf-8", body: "Hello, World!"},
		{name: "nothing matches", accept: "application/unsupported", contentType: "text/plain; charset=utf-8", body: "Hello, World!"},
		{name: "wildcard picks the first format", accept: "*/*", contentType: "text/html; charset=utf-8", body: "<p>Hello, World!</p>"},
		{name: "json", accept: MIMEApplicationJSON, contentType: "application/json; charset=utf-8", body: `"Hello, World!"`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := apptest.Req(MethodGet, "/", apptest.WithHeaderIf(HeaderAccept, tc.accept))
			resp := apptest.Expect(t, app, req, StatusOK)
			require.Equal(t, tc.contentType, resp.Header.Get(HeaderContentType))
			require.Equal(t, tc.body, apptest.Body(t, resp))
		})
	}
}

func Test_Ctx_IP_IPv4MappedIPv6InProxyHeader(t *testing.T) {
	t.Parallel()
	app := New(forwardedForConfig(TrustProxyConfig{Private: true}))
	fctx := &fasthttp.RequestCtx{}
	fctx.SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP("10.0.0.2"), Port: 8080})
	c := apptest.CtxFor(t, app, fctx)

	c.Request().Header.Set(HeaderXForwardedFor, "::ffff:203.0.113.5, 10.0.0.1")
	require.Equal(t, "::ffff:203.0.113.5", c.IP())
	require.Equal(t, []string{"::ffff:203.0.113.5", "10.0.0.1"}, c.IPs())

	// A mapped proxy address is still stripped as a trusted proxy.
	c.Request().Header.Set(HeaderXForwardedFor, "203.0.113.5, ::ffff:10.0.0.1")
	require.Equal(t, "203.0.113.5", c.IP())
}

func Test_Ctx_Accepts_MostSpecificRangeSetsQuality(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	c.Request().Header.Set(HeaderAccept, "text/*;q=1, text/html;q=0.5")
	require.Equal(t, "text/plain", c.Accepts("text/html", "text/plain"))
	require.Equal(t, "text/html", c.Accepts("text/html"), "a lowered quality is still acceptable")

	c.Request().Header.Set(HeaderAccept, "text/html;q=0.5, text/*;q=1")
	require.Equal(t, "text/plain", c.Accepts("text/html", "text/plain"))

	c.Request().Header.Set(HeaderAccept, "*/*;q=1, application/json;q=0.2")
	require.Equal(t, "text/plain", c.Accepts("application/json", "text/plain"))

	c.Request().Header.Set(HeaderAcceptEncoding, "gzip;q=0.5, *;q=1")
	require.Equal(t, "br", c.AcceptsEncodings("gzip", "br"))

	c.Request().Header.Set(HeaderAcceptLanguage, "en;q=0.5, *")
	require.Equal(t, "fr", c.AcceptsLanguages("en", "fr"))
	require.Equal(t, "fr", c.AcceptsLanguagesExtended("en", "fr"))

	c.Request().Header.Set(HeaderAcceptCharset, "utf-8;q=0.5, *")
	require.Equal(t, "iso-8859-1", c.AcceptsCharsets("utf-8", "iso-8859-1"))
}

func Test_Ctx_Format_CustomCtx(t *testing.T) {
	t.Parallel()
	app := ctxTestACustomApp()
	describe := func(c Ctx) error {
		if _, ok := c.(*customCtx); ok {
			return c.SendString("custom")
		}
		return c.SendString("default")
	}
	app.Get("/", func(c Ctx) error {
		return c.Format(
			ResFmt{MediaType: MIMETextPlain, Handler: describe},
			ResFmt{MediaType: "default", Handler: describe},
		)
	})

	for _, accept := range []string{"", MIMETextPlain, "application/unsupported"} {
		req := apptest.Req(MethodGet, "/", apptest.WithHeaderIf(HeaderAccept, accept))
		require.Equal(t, "custom", apptest.Body(t, apptest.Do(t, app, req)), "Accept %q", accept)
	}
}

// The identity step must not re-set the body with a slice aliasing it.
func Test_Ctx_Body_With_Compression_IdentityChain(t *testing.T) {
	t.Parallel()
	compressed := gzipLayers(t, []byte("john=doe"), 1)

	for _, reduce := range []bool{false, true} {
		app := New(Config{ReduceMemoryUsage: reduce})
		for _, encoding := range []string{"gzip, identity", "identity, gzip", "identity, gzip, identity"} {
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			c.Request().Header.Set(HeaderContentEncoding, encoding)
			c.Request().SetBody(compressed)

			require.Equal(t, []byte("john=doe"), c.Body(), "ReduceMemoryUsage=%v %q", reduce, encoding)
			require.Equal(t, compressed, c.Request().Body(), "the original body is restored")
			require.Equal(t, []byte("john=doe"), c.Body(), "decoding again still works")
			app.ReleaseCtx(c)
		}
	}
}

// gzipLayers returns data compressed with gzip, layers times over.
func gzipLayers(t *testing.T, data []byte, layers int) []byte {
	t.Helper()

	for range layers {
		data = ctxTestACompress(t, StrGzip, data)
	}
	return data
}

func Test_Ctx_Body_With_Compression_TooManyCodings(t *testing.T) {
	t.Parallel()
	codings := func(coding string, n int) string { return strings.TrimSuffix(strings.Repeat(coding+",", n), ",") }
	plain := []byte("john=doe")
	refused := []byte(ErrUnsupportedMediaType.Error())

	app := New()
	for _, tc := range []struct {
		name     string
		encoding string
		sent     []byte
		want     []byte
		wantCode int
	}{
		{
			name: "gzip up to the cap", encoding: codings(StrGzip, maxContentCodings),
			sent: gzipLayers(t, plain, maxContentCodings), want: plain, wantCode: StatusOK,
		},
		{
			name: "gzip past the cap", encoding: codings(StrGzip, maxContentCodings+1),
			sent: gzipLayers(t, plain, maxContentCodings+1), want: refused, wantCode: StatusUnsupportedMediaType,
		},
		{
			name: "identity past the cap", encoding: codings(StrIdentity, maxContentCodings+1),
			sent: plain, want: refused, wantCode: StatusUnsupportedMediaType,
		},
		// 256 members wrapped the former one-byte decode counter.
		{
			name: "256 members", encoding: codings(StrIdentity, 255) + "," + StrGzip,
			sent: gzipLayers(t, plain, 2), want: refused, wantCode: StatusUnsupportedMediaType,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)
			c.Request().Header.Set(HeaderContentEncoding, tc.encoding)
			c.Request().SetBody(tc.sent)

			require.Equal(t, tc.want, c.Body())
			require.Equal(t, tc.wantCode, c.Response().StatusCode())
			require.Equal(t, tc.sent, c.Request().Body())
		})
	}
}

// Body and BodyRaw results alias the request body buffer, so decoding must not hand it back to the pool.
func Test_Ctx_Body_With_Compression_KeepsRequestBuffer(t *testing.T) {
	t.Parallel()
	plain := []byte("john=doe")

	app := New()
	for _, tc := range []struct {
		encoding string
		sent     []byte
		want     []byte
	}{
		{encoding: "identity, identity", sent: plain, want: plain},
		{encoding: "gzip, identity", sent: gzipLayers(t, plain, 1), want: plain},
		{encoding: "identity, gzip", sent: gzipLayers(t, plain, 1), want: plain},
		{encoding: "gzip, gzip", sent: gzipLayers(t, plain, 2), want: plain},
		{encoding: "unknown, gzip", sent: gzipLayers(t, plain, 1), want: []byte(ErrUnsupportedMediaType.Error())},
	} {
		// A bare RequestCtx does not keep its body buffer, like ReduceMemoryUsage or the adaptor.
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderContentEncoding, tc.encoding)
		c.Request().SetBody(tc.sent)

		raw := c.BodyRaw()
		body := c.Body()
		require.Same(t, &raw[0], &c.Request().Body()[0], tc.encoding)

		next := fasthttp.AcquireRequest()
		next.SetBody(bytes.Repeat([]byte("!"), len(tc.sent)))
		require.Equal(t, tc.sent, raw, tc.encoding)
		require.Equal(t, tc.want, body, tc.encoding)
		fasthttp.ReleaseRequest(next)
		app.ReleaseCtx(c)
	}
}

type funcFS func(string) (fs.File, error)

func (f funcFS) Open(name string) (fs.File, error) { return f(name) }

func Test_SameFS(t *testing.T) {
	t.Parallel()
	dirA, dirB := os.DirFS("."), os.DirFS("..")
	mapA := fstest.MapFS{"a": &fstest.MapFile{}}
	openA := funcFS(func(string) (fs.File, error) { return nil, fs.ErrNotExist })
	openB := funcFS(func(string) (fs.File, error) { return nil, fs.ErrNotExist })

	require.True(t, sameFS(nil, nil))
	require.False(t, sameFS(dirA, nil))
	require.False(t, sameFS(nil, dirA))
	require.True(t, sameFS(dirA, dirA))
	require.False(t, sameFS(dirA, dirB))
	require.False(t, sameFS(dirA, mapA))
	require.True(t, sameFS(mapA, mapA))
	require.False(t, sameFS(mapA, fstest.MapFS{"a": &fstest.MapFile{}}))
	require.False(t, sameFS(openA, openB))
	require.False(t, sameFS(openA, openA))
}

// perIPConnStub mirrors fasthttp's perIPTLSConn, which wraps an accepted TLS
// connection when Server.MaxConnsPerIP is set.
type perIPConnStub struct {
	*tls.Conn
}

// ctxTestCTLSPipe returns one end of an in-memory pipe and a client TLS
// connection over it, both closed when the test ends. No handshake runs; the
// connections only carry identity.
func ctxTestCTLSPipe(t *testing.T) (raw net.Conn, client *tls.Conn) { //nolint:nonamedreturns // gocritic's unnamedResult asks for names
	t.Helper()

	raw, peer := net.Pipe()
	t.Cleanup(func() {
		require.NoError(t, raw.Close())
		require.NoError(t, peer.Close())
	})
	return raw, tls.Client(raw, &tls.Config{InsecureSkipVerify: true})
}

func Test_UnderlyingConn_UnwrapsTLSWrapper(t *testing.T) {
	t.Parallel()
	raw, tlsConn := ctxTestCTLSPipe(t)
	require.Equal(t, raw, underlyingConn(tlsConn))
	require.Equal(t, raw, underlyingConn(perIPConnStub{Conn: tlsConn}))
	require.Equal(t, raw, underlyingConn(raw))
}

func Test_TLSHandler_ForgetsRecycledWrapper(t *testing.T) {
	t.Parallel()
	handler := &TLSHandler{}
	raw1, tls1 := ctxTestCTLSPipe(t)
	wrapper := &perIPConnStub{Conn: tls1}

	handler.track(wrapper)
	_, err := handler.GetClientInfo(&tls.ClientHelloInfo{Conn: raw1, ServerName: "first"})
	require.NoError(t, err)
	require.NotNil(t, handler.clientHelloInfo(wrapper))

	// fasthttp recycles the wrapper before it reports the close of what it carried.
	raw2, tls2 := ctxTestCTLSPipe(t)
	wrapper.Conn = tls2
	handler.track(wrapper)
	_, ok := handler.clientHelloInfos.Load(raw1)
	require.False(t, ok, "the recycled wrapper stranded the record of the connection it dropped")

	_, err = handler.GetClientInfo(&tls.ClientHelloInfo{Conn: raw2, ServerName: "second"})
	require.NoError(t, err)
	require.Equal(t, "second", handler.clientHelloInfo(wrapper).ServerName)

	handler.forget(wrapper)
	require.Nil(t, handler.clientHelloInfo(tls2))
	_, ok = handler.serverConns.Load(wrapper)
	require.False(t, ok)
}

func Test_TLSHandler_ForgetsHijackedConnection(t *testing.T) {
	t.Parallel()
	raw, tlsConn := ctxTestCTLSPipe(t)

	app := New()
	app.SetTLSHandler(&TLSHandler{})
	app.mutex.Lock()
	app.hookConnState()
	app.mutex.Unlock()
	report := app.Server().ConnState

	handler := app.tlsHandler
	report(tlsConn, fasthttp.StateNew)
	_, err := handler.GetClientInfo(&tls.ClientHelloInfo{Conn: raw, ServerName: "ws"})
	require.NoError(t, err)
	require.NotNil(t, handler.clientHelloInfo(tlsConn))

	// fasthttp never reports StateClosed after a hijack, so it has to be terminal here.
	report(tlsConn, fasthttp.StateHijacked)
	require.Nil(t, handler.clientHelloInfo(tlsConn))
	_, ok := handler.serverConns.Load(tlsConn)
	require.False(t, ok)
}

// sliceFS is a file system whose identity lives in a slice header, so two
// prefixes of one backing array share an element pointer.
type sliceFS []string

func (sliceFS) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }

func Test_SameFS_SliceLength(t *testing.T) {
	t.Parallel()
	backing := sliceFS{"a", "b", "c"}
	require.False(t, sameFS(backing[:1], backing[:2]))
	require.True(t, sameFS(backing[:2], backing[:2]))
}

// Test_Res_Set_MatchesHeaderSet is the contract behind the canonical fast path
// in Set: whatever the key or value, the field line it stores is byte for byte
// the one fasthttp's own Set stores, with header normalization on and off.
func Test_Res_Set_MatchesHeaderSet(t *testing.T) {
	t.Parallel()
	keys := []string{
		"X-Request-Id", "x-request-id", "X-REQUEST-ID", "Content-Type", "content-type", "Content-Length",
		"Server", "Connection", "Date", "Set-Cookie", "Transfer-Encoding", "Content-Encoding", "Trailer",
		"Bad Key", "X-Key\r\nInjected", "Or\u00edgin", "", "etag", "X-REQUEST-ID", "x-upstream-id", strings.Repeat("Ab-", 21) + "C", // cspell:disable-line
	}
	values := []string{"v", "", "a\r\nb", "with space", "42", "text/html; charset=utf-8", "k=v; Path=/"}
	// storeNormalizes false with a normalizing app is the state a proxied
	// response leaves behind when a caller-supplied client parsed it with
	// header normalization off: the store keeps names as sent, so Set must
	// still replace a lower-case field spelled exactly the same.
	for _, normalizing := range []bool{true, false} {
		for _, storeNormalizes := range []bool{true, false} {
			app := New(Config{DisableHeaderNormalizing: !normalizing})
			for _, key := range keys {
				for _, val := range values {
					c := app.AcquireCtx(&fasthttp.RequestCtx{})
					if !storeNormalizes {
						c.Response().Header.DisableNormalizing()
					}
					c.Response().Header.Set("x-upstream-id", "stale")
					var want fasthttp.ResponseHeader
					c.Response().Header.CopyTo(&want)
					want.Set(key, val)
					c.Set(key, val)
					require.Equal(t, want.String(), c.Response().Header.String(),
						"normalizing=%v storeNormalizes=%v key=%q val=%q", normalizing, storeNormalizes, key, val)
					app.ReleaseCtx(c)
				}
			}
		}
	}
}

// Test_Res_Get_MatchesHeaderPeek is the read-side contract: Get answers what
// fasthttp's Peek answers for every spelling, whether or not the store
// normalizes.
func Test_Res_Get_MatchesHeaderPeek(t *testing.T) {
	t.Parallel()
	names := []string{"X-Request-Id", "x-request-id", "X-REQUEST-ID", "x-lower", "X-Lower", "Content-Type", "Missing", "", "Bad Key"}
	for _, storeNormalizes := range []bool{true, false} {
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		h := &c.Response().Header
		if !storeNormalizes {
			h.DisableNormalizing()
		}
		h.Set("X-Request-Id", "a")
		h.Set("x-lower", "b")
		h.Set("Content-Type", "text/plain")
		for _, name := range names {
			require.Equal(t, string(h.Peek(name)), c.Res().Get(name), "storeNormalizes=%v %q", storeNormalizes, name)
		}
		require.Equal(t, "fallback", c.Res().Get("Missing", "fallback"))
		app.ReleaseCtx(c)
	}
}

// Test_Res_Set_CopiesArguments pins that the fast path lends fasthttp the key and
// value only for the call: the stored line must not follow the caller's bytes.
func Test_Res_Set_CopiesArguments(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, New())

	keyBytes := []byte("X-Request-Id")
	valBytes := []byte("first")
	c.Set(utils.UnsafeString(keyBytes), utils.UnsafeString(valBytes))
	copy(keyBytes, "X-Rewritten!")
	copy(valBytes, "wrong")

	require.Equal(t, "first", c.Res().Get("X-Request-Id"))
	require.Empty(t, c.Res().Get("X-Rewritten!"))
}

// Test_Ctx_PathSlashCount pins the slash count routing filters on: whether the
// path's single pass counted it or pathSlashCount counts it later, it must be
// the number of '/' in the detection path, under every configuration that
// shapes that path and after trailing slashes are trimmed from it.
func Test_Ctx_PathSlashCount(t *testing.T) {
	t.Parallel()
	paths := []string{
		"/", "//", "///", "/a", "/a/", "/a//", "/A/B/c", "/a/b/c/", "/a/b/c///",
		"/ab/cd/ef/gh/ij", "/abcdefgh/ijklmnop/", "/x%2Fy/", "/x%41/", "/./a/../b/", "/a/./b//",
	}
	for _, caseSensitive := range []bool{false, true} {
		for _, strictRouting := range []bool{false, true} {
			for _, unescape := range []bool{false, true} {
				app := New(Config{CaseSensitive: caseSensitive, StrictRouting: strictRouting, UnescapePath: unescape})
				// A parametric route is what makes pathSlashCount count at all.
				app.Get("/:p", func(c Ctx) error { return c.Next() })
				app.startupProcess()

				for _, path := range paths {
					fctx := &fasthttp.RequestCtx{}
					fctx.Request.Header.SetMethod(MethodGet)
					fctx.Request.SetRequestURI(path)
					c, ok := app.AcquireCtx(fctx).(*DefaultCtx)
					require.True(t, ok)

					want := strings.Count(string(c.detectionPath), "/")
					name := fmt.Sprintf("cs=%v/sr=%v/unescape=%v %q (detection %q)",
						caseSensitive, strictRouting, unescape, path, c.detectionPath)
					require.Contains(t, []int{0, want}, c.pathSlashes, "cached count: %s", name)
					require.Equal(t, want, c.pathSlashCount(app), "count: %s", name)
					app.ReleaseCtx(c)
				}
			}
		}
	}
}

// Test_PathNeedsNormalization pins the request gate: a path that kept its
// parsed length has nothing to decode or resolve, except that a target
// without a leading slash is normalized whatever its length, since the slash
// fasthttp adds in front can be canceled by the one a doubled slash loses.
func Test_PathNeedsNormalization(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path string
		norm string
		want bool
	}{
		{path: "/a/b", norm: "/a/b", want: false},
		{path: "/a%41", norm: "/aA", want: true},
		{path: "/a//b", norm: "/a/b", want: true},
		{path: "/a/.", norm: "/a/.", want: true},
		{path: "/a%zzb", norm: "/a%zzb", want: false},
		{path: "", norm: "/", want: true},
		{path: "*", norm: "/*", want: true},
		{path: "admin//secret", norm: "/admin/secret", want: true},
		{path: "secret//", norm: "/secret/", want: true},
		{path: "example.com:443", norm: "/example.com:443", want: true},
	} {
		require.Equal(t, tc.want, pathNeedsNormalization(len(tc.norm), tc.path), "%q", tc.path)
	}
}
