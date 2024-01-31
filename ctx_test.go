// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

//nolint:bodyclose // Much easier to just ignore memory leaks in tests
package fiber

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

const epsilon = 0.001

// go test -run Test_Ctx_Accepts
func Test_Ctx_Accepts(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAccept, "text/html,application/xhtml+xml,application/xml;q=0.9")
	require.Equal(t, "", c.Accepts(""))
	require.Equal(t, "", c.Accepts())
	require.Equal(t, ".xml", c.Accepts(".xml"))
	require.Equal(t, "", c.Accepts(".john"))
	require.Equal(t, "application/xhtml+xml", c.Accepts("application/xml", "application/xml+rss", "application/yaml", "application/xhtml+xml"), "must use client-preferred mime type")

	c.Request().Header.Set(HeaderAccept, "application/json, text/plain, */*;q=0")
	require.Equal(t, "", c.Accepts("html"), "must treat */*;q=0 as not acceptable")

	c.Request().Header.Set(HeaderAccept, "text/*, application/json")
	require.Equal(t, "html", c.Accepts("html"))
	require.Equal(t, "text/html", c.Accepts("text/html"))
	require.Equal(t, "json", c.Accepts("json", "text"))
	require.Equal(t, "application/json", c.Accepts("application/json"))
	require.Equal(t, "", c.Accepts("image/png"))
	require.Equal(t, "", c.Accepts("png"))

	c.Request().Header.Set(HeaderAccept, "text/html, application/json")
	require.Equal(t, "text/*", c.Accepts("text/*"))

	c.Request().Header.Set(HeaderAccept, "*/*")
	require.Equal(t, "html", c.Accepts("html"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Accepts -benchmem -count=4
func Benchmark_Ctx_Accepts(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	acceptHeader := "text/html,application/xhtml+xml,application/xml;q=0.9"
	c.Request().Header.Set("Accept", acceptHeader)
	acceptValues := [][]string{
		{".xml"},
		{"json", "xml"},
		{"application/json", "application/xml"},
	}
	expectedResults := []string{".xml", "xml", "application/xml"}

	for i := 0; i < len(acceptValues); i++ {
		b.Run(fmt.Sprintf("run-%#v", acceptValues[i]), func(bb *testing.B) {
			var res string
			bb.ReportAllocs()
			bb.ResetTimer()

			for n := 0; n < bb.N; n++ {
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

	app := New()

	app.NewCtxFunc(func(app *App) CustomCtx {
		return &customCtx{
			DefaultCtx: *NewDefaultCtx(app),
		}
	})

	app.Get("/:id", func(c Ctx) error {
		return c.SendString(c.Params("id"))
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v3", &bytes.Buffer{}))
	require.NoError(t, err, "app.Test(req)")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "io.ReadAll(resp.Body)")
	require.Equal(t, "prefix_v3", string(body))
}

// go test -run Test_Ctx_Accepts_EmptyAccept
func Test_Ctx_Accepts_EmptyAccept(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	require.Equal(t, ".forwarded", c.Accepts(".forwarded"))
}

// go test -run Test_Ctx_Accepts_Wildcard
func Test_Ctx_Accepts_Wildcard(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAccept, "*/*;q=0.9")
	require.Equal(t, "html", c.Accepts("html"))
	require.Equal(t, "foo", c.Accepts("foo"))
	require.Equal(t, ".bar", c.Accepts(".bar"))
	c.Request().Header.Set(HeaderAccept, "text/html,application/*;q=0.9")
	require.Equal(t, "xml", c.Accepts("xml"))
}

// go test -run Test_Ctx_AcceptsCharsets
func Test_Ctx_AcceptsCharsets(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAcceptCharset, "utf-8, iso-8859-1;q=0.5")
	require.Equal(t, "utf-8", c.AcceptsCharsets("utf-8"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsCharsets -benchmem -count=4
func Benchmark_Ctx_AcceptsCharsets(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	c.Request().Header.Set("Accept-Charset", "utf-8, iso-8859-1;q=0.5")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.AcceptsCharsets("utf-8")
	}
	require.Equal(b, "utf-8", res)
}

// go test -run Test_Ctx_AcceptsEncodings
func Test_Ctx_AcceptsEncodings(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAcceptEncoding, "deflate, gzip;q=1.0, *;q=0.5")
	require.Equal(t, "gzip", c.AcceptsEncodings("gzip"))
	require.Equal(t, "abc", c.AcceptsEncodings("abc"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsEncodings -benchmem -count=4
func Benchmark_Ctx_AcceptsEncodings(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	c.Request().Header.Set(HeaderAcceptEncoding, "deflate, gzip;q=1.0, *;q=0.5")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.AcceptsEncodings("gzip")
	}
	require.Equal(b, "gzip", res)
}

// go test -run Test_Ctx_AcceptsLanguages
func Test_Ctx_AcceptsLanguages(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAcceptLanguage, "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")
	require.Equal(t, "fr", c.AcceptsLanguages("fr"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsLanguages -benchmem -count=4
func Benchmark_Ctx_AcceptsLanguages(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	c.Request().Header.Set(HeaderAcceptLanguage, "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.AcceptsLanguages("fr")
	}
	require.Equal(b, "fr", res)
}

// go test -run Test_Ctx_App
func Test_Ctx_App(t *testing.T) {
	t.Parallel()
	app := New()
	app.config.BodyLimit = 1000
	c := app.NewCtx(&fasthttp.RequestCtx{})

	require.Equal(t, 1000, c.App().config.BodyLimit)
}

// go test -run Test_Ctx_Append
func Test_Ctx_Append(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	require.Equal(t, "", string(c.Response().Header.Peek("x-custom-header")))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Append -benchmem -count=4
func Benchmark_Ctx_Append(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Append("X-Custom-Header", "Hello")
		c.Append("X-Custom-Header", "World")
		c.Append("X-Custom-Header", "Hello")
	}
	require.Equal(b, "Hello, World", app.getString(c.Response().Header.Peek("X-Custom-Header")))
}

// go test -run Test_Ctx_Attachment
func Test_Ctx_Attachment(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	// empty
	c.Attachment()
	require.Equal(t, `attachment`, string(c.Response().Header.Peek(HeaderContentDisposition)))
	// real filename
	c.Attachment("./static/img/logo.png")
	require.Equal(t, `attachment; filename="logo.png"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
	require.Equal(t, "image/png", string(c.Response().Header.Peek(HeaderContentType)))
	// check quoting
	c.Attachment("another document.pdf\"\r\nBla: \"fasel")
	require.Equal(t, `attachment; filename="another+document.pdf%22%0D%0ABla%3A+%22fasel"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Attachment -benchmem -count=4
func Benchmark_Ctx_Attachment(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// example with quote params
		c.Attachment("another document.pdf\"\r\nBla: \"fasel")
	}
	require.Equal(b, `attachment; filename="another+document.pdf%22%0D%0ABla%3A+%22fasel"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
}

// go test -run Test_Ctx_BaseURL
func Test_Ctx_BaseURL(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().SetRequestURI("http://google.com/test")
	require.Equal(t, "http://google.com", c.BaseURL())
	// Check cache
	require.Equal(t, "http://google.com", c.BaseURL())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_BaseURL -benchmem
func Benchmark_Ctx_BaseURL(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	c.Request().SetHost("google.com:1337")
	c.Request().URI().SetPath("/haha/oke/lol")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.BaseURL()
	}
	require.Equal(b, "http://google.com:1337", res)
}

// go test -run Test_Ctx_Body
func Test_Ctx_Body(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	c.Request().SetBody([]byte("john=doe"))
	require.Equal(t, []byte("john=doe"), c.Body())
}

func Benchmark_Ctx_Body(b *testing.B) {
	const input = "john=doe"

	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	c.Request().SetBody([]byte(input))
	for i := 0; i < b.N; i++ {
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
			name:            "unsupported_encoding",
			contentEncoding: "undefined",
			body:            []byte("keeps_ORIGINAL"),
			expectedBody:    []byte("keeps_ORIGINAL"),
		},
		{
			name:            "gzip then unsupported",
			contentEncoding: "gzip, undefined",
			body:            []byte("Go, be gzipped"),
			expectedBody:    []byte("Go, be gzipped"),
		},
		{
			name:            "invalid_deflate",
			contentEncoding: "gzip,deflate",
			body:            []byte("I'm not correctly compressed"),
			expectedBody:    []byte(zlib.ErrHeader.Error()),
		},
	}

	for _, testObject := range tests {
		tCase := testObject // Duplicate object to ensure it will be unique across all runs
		t.Run(tCase.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed
			c.Request().Header.Set("Content-Encoding", tCase.contentEncoding)

			if strings.Contains(tCase.contentEncoding, "gzip") {
				var b bytes.Buffer
				gz := gzip.NewWriter(&b)

				_, err := gz.Write(tCase.body)
				require.NoError(t, err)

				err = gz.Flush()
				require.NoError(t, err)

				err = gz.Close()
				require.NoError(t, err)
				tCase.body = b.Bytes()
			}

			c.Request().SetBody(tCase.body)
			body := c.Body()
			require.Equal(t, tCase.expectedBody, body)

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
	compressionTests := []struct {
		contentEncoding string
		compressWriter  func([]byte) ([]byte, error)
	}{
		{
			contentEncoding: "gzip",
			compressWriter:  compressGzip,
		},
		{
			contentEncoding: "gzip,invalid",
			compressWriter:  compressGzip,
		},
		{
			contentEncoding: "deflate",
			compressWriter:  compressDeflate,
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
				{
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
				}

				data = make([]byte, buf.Len())
				copy(data, buf.Bytes())
				buf.Reset()

				// gzip
				{
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
				}

				return buf.Bytes(), nil
			},
		},
	}

	for _, ct := range compressionTests {
		b.Run(ct.contentEncoding, func(b *testing.B) {
			app := New()
			const input = "john=doe"
			c := app.NewCtx(&fasthttp.RequestCtx{})

			c.Request().Header.Set("Content-Encoding", ct.contentEncoding)
			compressedBody, err := ct.compressWriter([]byte(input))
			require.NoError(b, err)

			c.Request().SetBody(compressedBody)
			for i := 0; i < b.N; i++ {
				_ = c.Body()
			}

			require.Equal(b, []byte(input), c.Body())
		})
	}
}

// go test -run Test_Ctx_Context
func Test_Ctx_Context(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	require.Equal(t, "*fasthttp.RequestCtx", fmt.Sprintf("%T", c.Context()))
}

// go test -run Test_Ctx_UserContext
func Test_Ctx_UserContext(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	t.Run("Nil_Context", func(t *testing.T) {
		t.Parallel()
		ctx := c.UserContext()
		require.Equal(t, ctx, context.Background())
	})
	t.Run("ValueContext", func(t *testing.T) {
		t.Parallel()
		testKey := struct{}{}
		testValue := "Test Value"
		ctx := context.WithValue(context.Background(), testKey, testValue)
		require.Equal(t, testValue, ctx.Value(testKey))
	})
}

// go test -run Test_Ctx_SetUserContext
func Test_Ctx_SetUserContext(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	testKey := struct{}{}
	testValue := "Test Value"
	ctx := context.WithValue(context.Background(), testKey, testValue)
	c.SetUserContext(ctx)
	require.Equal(t, testValue, c.UserContext().Value(testKey))
}

// go test -run Test_Ctx_UserContext_Multiple_Requests
func Test_Ctx_UserContext_Multiple_Requests(t *testing.T) {
	t.Parallel()
	testKey := struct{}{}
	testValue := "foobar-value"

	app := New()
	app.Get("/", func(c Ctx) error {
		ctx := c.UserContext()

		if ctx.Value(testKey) != nil {
			return c.SendStatus(StatusInternalServerError)
		}

		input := utils.CopyString(Query(c, "input", "NO_VALUE"))
		ctx = context.WithValue(ctx, testKey, fmt.Sprintf("%s_%s", testValue, input))
		c.SetUserContext(ctx)

		return c.Status(StatusOK).SendString(fmt.Sprintf("resp_%s_returned", input))
	})

	// Consecutive Requests
	for i := 1; i <= 10; i++ {
		i := i
		t.Run(fmt.Sprintf("request_%d", i), func(t *testing.T) {
			t.Parallel()
			resp, err := app.Test(httptest.NewRequest(MethodGet, fmt.Sprintf("/?input=%d", i), nil))

			require.NoError(t, err, "Unexpected error from response")
			require.Equal(t, StatusOK, resp.StatusCode, "context.Context returned from c.UserContext() is reused")

			b, err := io.ReadAll(resp.Body)
			require.NoError(t, err, "Unexpected error from reading response body")
			require.Equal(t, fmt.Sprintf("resp_%d_returned", i), string(b), "response text incorrect")
		})
	}
}

// go test -run Test_Ctx_Cookie
func Test_Ctx_Cookie(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	c.Cookie(cookie)
	expect := "username=john; expires=" + httpdate + "; path=/; SameSite=Lax"
	require.Equal(t, expect, string(c.Response().Header.Peek(HeaderSetCookie)))

	expect = "username=john; expires=" + httpdate + "; path=/"
	cookie.SameSite = CookieSameSiteDisabled
	c.Cookie(cookie)
	require.Equal(t, expect, string(c.Response().Header.Peek(HeaderSetCookie)))

	expect = "username=john; expires=" + httpdate + "; path=/; SameSite=Strict"
	cookie.SameSite = CookieSameSiteStrictMode
	c.Cookie(cookie)
	require.Equal(t, expect, string(c.Response().Header.Peek(HeaderSetCookie)))

	expect = "username=john; expires=" + httpdate + "; path=/; secure; SameSite=None"
	cookie.Secure = true
	cookie.SameSite = CookieSameSiteNoneMode
	c.Cookie(cookie)
	require.Equal(t, expect, string(c.Response().Header.Peek(HeaderSetCookie)))

	expect = "username=john; path=/; secure; SameSite=None"
	// should remove expires and max-age headers
	cookie.SessionOnly = true
	cookie.Expires = expire
	cookie.MaxAge = 10000
	c.Cookie(cookie)
	require.Equal(t, expect, string(c.Response().Header.Peek(HeaderSetCookie)))

	expect = "username=john; path=/; secure; SameSite=None"
	// should remove expires and max-age headers when no expire and no MaxAge (default time)
	cookie.SessionOnly = false
	cookie.Expires = time.Time{}
	cookie.MaxAge = 0
	c.Cookie(cookie)
	require.Equal(t, expect, string(c.Response().Header.Peek(HeaderSetCookie)))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Cookie -benchmem -count=4
func Benchmark_Ctx_Cookie(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Cookie(&Cookie{
			Name:  "John",
			Value: "Doe",
		})
	}
	require.Equal(b, "John=Doe; path=/; SameSite=Lax", app.getString(c.Response().Header.Peek("Set-Cookie")))
}

// go test -run Test_Ctx_Cookies
func Test_Ctx_Cookies(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Cookie", "john=doe")
	require.Equal(t, "doe", c.Cookies("john"))
	require.Equal(t, "default", c.Cookies("unknown", "default"))
}

// go test -run Test_Ctx_Format
func Test_Ctx_Format(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	// set `accepted` to whatever media type was chosen by Format
	var accepted string
	formatHandlers := func(types ...string) []ResFmt {
		fmts := []ResFmt{}
		for _, t := range types {
			t := utils.CopyString(t)
			fmts = append(fmts, ResFmt{t, func(c Ctx) error {
				accepted = t
				return nil
			}})
		}
		return fmts
	}

	c.Request().Header.Set(HeaderAccept, `text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7`)
	err := c.Format(formatHandlers("application/xhtml+xml", "application/xml", "foo/bar")...)
	require.Equal(t, "application/xhtml+xml", accepted)
	require.Equal(t, "application/xhtml+xml", c.GetRespHeader(HeaderContentType))
	require.NoError(t, err)
	require.NotEqual(t, StatusNotAcceptable, c.Response().StatusCode())

	err = c.Format(formatHandlers("foo/bar;a=b")...)
	require.Equal(t, "foo/bar;a=b", accepted)
	require.Equal(t, "foo/bar;a=b", c.GetRespHeader(HeaderContentType))
	require.NoError(t, err)
	require.NotEqual(t, StatusNotAcceptable, c.Response().StatusCode())

	myError := errors.New("this is an error")
	err = c.Format(ResFmt{"text/html", func(c Ctx) error { return myError }})
	require.ErrorIs(t, err, myError)

	c.Request().Header.Set(HeaderAccept, "application/json")
	err = c.Format(ResFmt{"text/html", func(c Ctx) error { return c.SendStatus(StatusOK) }})
	require.Equal(t, StatusNotAcceptable, c.Response().StatusCode())
	require.NoError(t, err)

	err = c.Format(formatHandlers("text/html", "default")...)
	require.Equal(t, "default", accepted)
	require.Equal(t, "text/html", c.GetRespHeader(HeaderContentType))
	require.NoError(t, err)

	err = c.Format()
	require.ErrorIs(t, err, ErrNoHandlers)
}

func Benchmark_Ctx_Format(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
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
		for n := 0; n < b.N; n++ {
			err = c.Format(
				ResFmt{"application/xml", fail},
				ResFmt{"text/html", fail},
				ResFmt{"text/plain;format=fixed", fail},
				ResFmt{"text/plain;format=flowed", ok},
			)
		}
		require.NoError(b, err)
	})

	b.Run("pre-allocated args", func(b *testing.B) {
		offers := []ResFmt{
			{"application/xml", fail},
			{"text/html", fail},
			{"text/plain;format=fixed", fail},
			{"text/plain;format=flowed", ok},
		}
		for n := 0; n < b.N; n++ {
			err = c.Format(offers...)
		}
		require.NoError(b, err)
	})

	c.Request().Header.Set("Accept", "text/plain")
	b.Run("text/plain", func(b *testing.B) {
		offers := []ResFmt{
			{"application/xml", fail},
			{"text/plain", ok},
		}
		for n := 0; n < b.N; n++ {
			err = c.Format(offers...)
		}
		require.NoError(b, err)
	})

	c.Request().Header.Set("Accept", "json")
	b.Run("json", func(b *testing.B) {
		offers := []ResFmt{
			{"xml", fail},
			{"html", fail},
			{"json", ok},
		}
		for n := 0; n < b.N; n++ {
			err = c.Format(offers...)
		}
		require.NoError(b, err)
	})
}

// go test -run Test_Ctx_AutoFormat
func Test_Ctx_AutoFormat(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAccept, MIMETextPlain)
	err := c.AutoFormat([]byte("Hello, World!"))
	require.NoError(t, err)
	require.Equal(t, "Hello, World!", string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMETextHTML)
	err = c.AutoFormat("Hello, World!")
	require.NoError(t, err)
	require.Equal(t, "<p>Hello, World!</p>", string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMEApplicationJSON)
	err = c.AutoFormat("Hello, World!")
	require.NoError(t, err)
	require.Equal(t, `"Hello, World!"`, string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMETextPlain)
	err = c.AutoFormat(complex(1, 1))
	require.NoError(t, err)
	require.Equal(t, "(1+1i)", string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMEApplicationXML)
	err = c.AutoFormat("Hello, World!")
	require.NoError(t, err)
	require.Equal(t, `<string>Hello, World!</string>`, string(c.Response().Body()))

	err = c.AutoFormat(complex(1, 1))
	require.Error(t, err)

	c.Request().Header.Set(HeaderAccept, MIMETextPlain)
	err = c.AutoFormat(Map{})
	require.NoError(t, err)
	require.Equal(t, "map[]", string(c.Response().Body()))

	type broken string
	c.Request().Header.Set(HeaderAccept, "broken/accept")
	require.NoError(t, err)
	err = c.AutoFormat(broken("Hello, World!"))
	require.NoError(t, err)
	require.Equal(t, `Hello, World!`, string(c.Response().Body()))
}

func Test_Ctx_AutoFormat_Struct(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	type Message struct {
		Recipients []string
		Sender     string `xml:"sender,attr"`
		Urgency    int    `xml:"urgency,attr"`
	}
	data := Message{
		Recipients: []string{"Alice", "Bob"},
		Sender:     "Carol",
		Urgency:    3,
	}

	c.Request().Header.Set(HeaderAccept, MIMEApplicationJSON)
	err := c.AutoFormat(data)
	require.NoError(t, err)
	require.Equal(t,
		`{"Recipients":["Alice","Bob"],"Sender":"Carol","Urgency":3}`,
		string(c.Response().Body()),
	)

	c.Request().Header.Set(HeaderAccept, MIMEApplicationXML)
	err = c.AutoFormat(data)
	require.NoError(t, err)
	require.Equal(t,
		`<Message sender="Carol" urgency="3"><Recipients>Alice</Recipients><Recipients>Bob</Recipients></Message>`,
		string(c.Response().Body()),
	)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AutoFormat -benchmem -count=4
func Benchmark_Ctx_AutoFormat(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Accept", "text/plain")
	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		err = c.AutoFormat("Hello, World!")
	}
	require.NoError(b, err)
	require.Equal(b, `Hello, World!`, string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AutoFormat_HTML -benchmem -count=4
func Benchmark_Ctx_AutoFormat_HTML(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Accept", "text/html")
	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		err = c.AutoFormat("Hello, World!")
	}
	require.NoError(b, err)
	require.Equal(b, "<p>Hello, World!</p>", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AutoFormat_JSON -benchmem -count=4
func Benchmark_Ctx_AutoFormat_JSON(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Accept", "application/json")
	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		err = c.AutoFormat("Hello, World!")
	}
	require.NoError(b, err)
	require.Equal(b, `"Hello, World!"`, string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AutoFormat_XML -benchmem -count=4
func Benchmark_Ctx_AutoFormat_XML(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Accept", "application/xml")
	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
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
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
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
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", writer.Boundary()))
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Fresh_StaleEtag -benchmem -count=4
func Benchmark_Ctx_Fresh_StaleEtag(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	for n := 0; n < b.N; n++ {
		c.Request().Header.Set(HeaderIfNoneMatch, "a, b, c, d")
		c.Request().Header.Set(HeaderCacheControl, "c")
		c.Fresh()

		c.Request().Header.Set(HeaderIfNoneMatch, "a, b, c, d")
		c.Request().Header.Set(HeaderCacheControl, "e")
		c.Fresh()
	}
}

// go test -run Test_Ctx_Fresh
func Test_Ctx_Fresh(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

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

	c.Request().Header.Set(HeaderIfNoneMatch, "675af34563dc-tr34")
	c.Request().Header.Set(HeaderCacheControl, "public")
	require.False(t, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, "a, b")
	c.Response().Header.Set(HeaderETag, "c")
	require.False(t, c.Fresh())

	c.Response().Header.Set(HeaderETag, "a")
	require.True(t, c.Fresh())

	c.Request().Header.Set(HeaderIfModifiedSince, "xxWed, 21 Oct 2015 07:28:00 GMT")
	c.Response().Header.Set(HeaderLastModified, "xxWed, 21 Oct 2015 07:28:00 GMT")
	require.False(t, c.Fresh())

	c.Response().Header.Set(HeaderLastModified, "Wed, 21 Oct 2015 07:28:00 GMT")
	require.False(t, c.Fresh())

	c.Request().Header.Set(HeaderIfModifiedSince, "Wed, 21 Oct 2015 07:28:00 GMT")
	require.False(t, c.Fresh())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Fresh_WithNoCache -benchmem -count=4
func Benchmark_Ctx_Fresh_WithNoCache(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderIfNoneMatch, "*")
	c.Request().Header.Set(HeaderCacheControl, "no-cache")
	for n := 0; n < b.N; n++ {
		c.Fresh()
	}
}

// go test -run Test_Ctx_Get
func Test_Ctx_Get(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAcceptCharset, "utf-8, iso-8859-1;q=0.5")
	c.Request().Header.Set(HeaderReferer, "Monster")
	require.Equal(t, "utf-8, iso-8859-1;q=0.5", c.Get(HeaderAcceptCharset))
	require.Equal(t, "Monster", c.Get(HeaderReferer))
	require.Equal(t, "default", c.Get("unknown", "default"))
}

// go test -run Test_Ctx_Host
func Test_Ctx_Host(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().SetRequestURI("http://google.com/test")
	require.Equal(t, "google.com", c.Host())
}

// go test -run Test_Ctx_Host_UntrustedProxy
func Test_Ctx_Host_UntrustedProxy(t *testing.T) {
	t.Parallel()
	// Don't trust any proxy
	{
		app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{}})
		c := app.NewCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		require.Equal(t, "google.com", c.Host())
		app.ReleaseCtx(c)
	}
	// Trust to specific proxy list
	{
		app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.8.0.0", "0.8.0.1"}})
		c := app.NewCtx(&fasthttp.RequestCtx{})
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
		app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0", "0.8.0.1"}})
		c := app.NewCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		require.Equal(t, "google1.com", c.Host())
		app.ReleaseCtx(c)
	}
}

// go test -run Test_Ctx_Host_TrustedProxyRange
func Test_Ctx_Host_TrustedProxyRange(t *testing.T) {
	t.Parallel()

	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0/30"}})
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com/test")
	c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
	require.Equal(t, "google1.com", c.Host())
	app.ReleaseCtx(c)
}

// go test -run Test_Ctx_Host_UntrustedProxyRange
func Test_Ctx_Host_UntrustedProxyRange(t *testing.T) {
	t.Parallel()

	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"1.0.0.0/30"}})
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com/test")
	c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
	require.Equal(t, "google.com", c.Host())
	app.ReleaseCtx(c)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Host -benchmem -count=4
func Benchmark_Ctx_Host(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com/test")
	var host string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		host = c.Host()
	}
	require.Equal(b, "google.com", host)
}

// go test -run Test_Ctx_IsProxyTrusted
func Test_Ctx_IsProxyTrusted(t *testing.T) {
	t.Parallel()

	{
		app := New()
		c := app.NewCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		require.True(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			EnableTrustedProxyCheck: false,
		})
		c := app.NewCtx(&fasthttp.RequestCtx{})
		require.True(t, c.IsProxyTrusted())
	}

	{
		app := New(Config{
			EnableTrustedProxyCheck: true,
		})
		c := app.NewCtx(&fasthttp.RequestCtx{})
		require.False(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			EnableTrustedProxyCheck: true,

			TrustedProxies: []string{},
		})
		c := app.NewCtx(&fasthttp.RequestCtx{})
		require.False(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			EnableTrustedProxyCheck: true,

			TrustedProxies: []string{
				"127.0.0.1",
			},
		})
		c := app.NewCtx(&fasthttp.RequestCtx{})
		require.False(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			EnableTrustedProxyCheck: true,

			TrustedProxies: []string{
				"127.0.0.1/8",
			},
		})
		c := app.NewCtx(&fasthttp.RequestCtx{})
		require.False(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			EnableTrustedProxyCheck: true,

			TrustedProxies: []string{
				"0.0.0.0",
			},
		})
		c := app.NewCtx(&fasthttp.RequestCtx{})
		require.True(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			EnableTrustedProxyCheck: true,

			TrustedProxies: []string{
				"0.0.0.1/31",
			},
		})
		c := app.NewCtx(&fasthttp.RequestCtx{})
		require.True(t, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			EnableTrustedProxyCheck: true,

			TrustedProxies: []string{
				"0.0.0.1/31junk",
			},
		})
		c := app.NewCtx(&fasthttp.RequestCtx{})
		require.False(t, c.IsProxyTrusted())
	}
}

// go test -run Test_Ctx_Hostname
func Test_Ctx_Hostname(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().SetRequestURI("http://google.com/test")
	require.Equal(t, "google.com", c.Hostname())

	c.Request().SetRequestURI("http://google.com:8080/test")
	require.Equal(t, "google.com", c.Hostname())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Hostname -benchmem -count=4
func Benchmark_Ctx_Hostname(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com:8080/test")
	var hostname string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		hostname = c.Hostname()
	}
	// Trust to specific proxy list
	{
		app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.8.0.0", "0.8.0.1"}})
		c := app.NewCtx(&fasthttp.RequestCtx{})
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
		app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0", "0.8.0.1"}})
		c := app.NewCtx(&fasthttp.RequestCtx{})
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
		app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0", "0.8.0.1"}})
		c := app.NewCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com, google2.com")
		require.Equal(t, "google1.com", c.Hostname())
		app.ReleaseCtx(c)
	}
}

// go test -run Test_Ctx_Hostname_UntrustedProxyRange
func Test_Ctx_Hostname_TrustedProxyRange(t *testing.T) {
	t.Parallel()

	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0/30"}})
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com/test")
	c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
	require.Equal(t, "google1.com", c.Hostname())
	app.ReleaseCtx(c)
}

// go test -run Test_Ctx_Hostname_UntrustedProxyRange
func Test_Ctx_Hostname_UntrustedProxyRange(t *testing.T) {
	t.Parallel()

	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"1.0.0.0/30"}})
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com/test")
	c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
	require.Equal(t, "google.com", c.Hostname())
	app.ReleaseCtx(c)
}

// go test -run Test_Ctx_Port
func Test_Ctx_Port(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	require.Equal(t, "0", c.Port())
}

// go test -run Test_Ctx_PortInHandler
func Test_Ctx_PortInHandler(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/port", func(c Ctx) error {
		return c.SendString(c.Port())
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/port", nil))
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
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
		app := New(Config{ProxyHeader: proxyHeaderName})
		c := app.NewCtx(&fasthttp.RequestCtx{})

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
		require.Equal(t, "", c.IP())

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
		app := New(Config{EnableIPValidation: true, ProxyHeader: proxyHeaderName})
		c := app.NewCtx(&fasthttp.RequestCtx{})

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
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.8.0.1"}, ProxyHeader: HeaderXForwardedFor})
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderXForwardedFor, "0.0.0.1")
	require.Equal(t, "0.0.0.0", c.IP())
}

// go test -run Test_Ctx_IP_TrustedProxy
func Test_Ctx_IP_TrustedProxy(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0"}, ProxyHeader: HeaderXForwardedFor})
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderXForwardedFor, "0.0.0.1")
	require.Equal(t, "0.0.0.1", c.IP())
}

// go test -run Test_Ctx_IPs  -parallel
func Test_Ctx_IPs(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, invalid, 127.0.0.1")
	var res []string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IPs()
	}
	require.Equal(b, []string{"127.0.0.1", "invalid", "127.0.0.1"}, res)
}

func Benchmark_Ctx_IPs_v6(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderXForwardedFor, "f037:825e:eadb:1b7b:1667:6f0a:5356:f604, invalid, 2345:0425:2CA1::0567:5673:23b5")
	var res []string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IPs()
	}
	require.Equal(b, []string{"f037:825e:eadb:1b7b:1667:6f0a:5356:f604", "invalid", "2345:0425:2CA1::0567:5673:23b5"}, res)
}

func Benchmark_Ctx_IPs_With_IP_Validation(b *testing.B) {
	app := New(Config{EnableIPValidation: true})
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, invalid, 127.0.0.1")
	var res []string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IPs()
	}
	require.Equal(b, []string{"127.0.0.1", "127.0.0.1"}, res)
}

func Benchmark_Ctx_IPs_v6_With_IP_Validation(b *testing.B) {
	app := New(Config{EnableIPValidation: true})
	c := app.NewCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderXForwardedFor, "2345:0425:2CA1:0000:0000:0567:5673:23b5, invalid, 2345:0425:2CA1::0567:5673:23b5")
	var res []string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IPs()
	}
	require.Equal(b, []string{"2345:0425:2CA1:0000:0000:0567:5673:23b5", "2345:0425:2CA1::0567:5673:23b5"}, res)
}

func Benchmark_Ctx_IP_With_ProxyHeader(b *testing.B) {
	app := New(Config{ProxyHeader: HeaderXForwardedFor})
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IP()
	}
	require.Equal(b, "127.0.0.1", res)
}

func Benchmark_Ctx_IP_With_ProxyHeader_and_IP_Validation(b *testing.B) {
	app := New(Config{ProxyHeader: HeaderXForwardedFor, EnableIPValidation: true})
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IP()
	}
	require.Equal(b, "127.0.0.1", res)
}

func Benchmark_Ctx_IP(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request()
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IP()
	}
	require.Equal(b, "0.0.0.0", res)
}

// go test -run Test_Ctx_Is
func Test_Ctx_Is(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Is -benchmem -count=4
func Benchmark_Ctx_Is(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderContentType, MIMEApplicationJSON)
	var res bool
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
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
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Locals_Generic
func Test_Ctx_Locals_Generic(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error {
		Locals[string](c, "john", "doe")
		Locals[int](c, "age", 18)
		Locals[bool](c, "isHuman", true)
		return c.Next()
	})
	app.Get("/test", func(c Ctx) error {
		require.Equal(t, "doe", Locals[string](c, "john"))
		require.Equal(t, 18, Locals[int](c, "age"))
		require.True(t, Locals[bool](c, "isHuman"))
		require.Equal(t, 0, Locals[int](c, "isHuman"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", nil))
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
		Locals[User](c, "user", User{"john", 18})
		return c.Next()
	})
	app.Use("/test", func(c Ctx) error {
		require.Equal(t, User{"john", 18}, Locals[User](c, "user"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Method
func Test_Ctx_Method(t *testing.T) {
	t.Parallel()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	app := New()
	c := app.NewCtx(fctx)

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
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/ServerName", nil))
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
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/ServerName", nil))
	require.NoError(t, err)

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("example.golang"), body)

	// Test SignatureSchemes
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/SignatureSchemes", nil))
	require.NoError(t, err)

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "["+strconv.Itoa(pssWithSHA256)+"]", string(body))

	// Test SupportedVersions
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/SupportedVersions", nil))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "["+strconv.Itoa(versionTLS13)+"]", string(body))
}

// go test -run Test_Ctx_InvalidMethod
func Test_Ctx_InvalidMethod(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
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
	req.Header.Set(HeaderContentType, fmt.Sprintf("multipart/form-data; boundary=%s", writer.Boundary()))
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
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(c)
	}
}

// go test -run Test_Ctx_OriginalURL
func Test_Ctx_OriginalURL(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
		require.Equal(t, "2222", c.Params("*2"))
		require.Equal(t, "1111", c.Params("*"))
		return nil
	})
	app.Get("/test4/:optional?", func(c Ctx) error {
		require.Equal(t, "", c.Params("optional"))
		return nil
	})
	app.Get("/test5/:id/:Id", func(c Ctx) error {
		require.Equal(t, "first", c.Params("id"))
		require.Equal(t, "first", c.Params("Id"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test/john", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test2/im/a/cookie", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test3/1111/blafasel/2222", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test4", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test5/first/second", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

func Test_Ctx_Params_Case_Sensitive(t *testing.T) {
	t.Parallel()
	app := New(Config{CaseSensitive: true})
	app.Get("/test/:User", func(c Ctx) error {
		require.Equal(t, "john", c.Params("User"))
		require.Equal(t, "", c.Params("user"))
		return nil
	})
	app.Get("/test2/:id/:Id", func(c Ctx) error {
		require.Equal(t, "first", c.Params("id"))
		require.Equal(t, "second", c.Params("Id"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test/john", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test2/first/second", nil))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Params -benchmem -count=4
func Benchmark_Ctx_Params(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

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
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
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
		// not strict && case insensitive
		require.Equal(t, "/ABC/", c.Path("/ABC/"))
		require.Equal(t, "/test/john/", c.Path("/test/john/"))
		return nil
	})

	// test with special chars
	app.Get("/specialChars/:name", func(c Ctx) error {
		require.Equal(t, "/specialChars/créer", c.Path())
		// unescape is also working if you set the path afterwards
		require.Equal(t, "/اختبار/", c.Path("/%D8%A7%D8%AE%D8%AA%D8%A8%D8%A7%D8%B1/"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/specialChars/cr%C3%A9er", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Protocol
func Test_Ctx_Protocol(t *testing.T) {
	t.Parallel()
	app := New()

	c := app.NewCtx(&fasthttp.RequestCtx{})

	require.Equal(t, "HTTP/1.1", c.Protocol())

	c.Request().Header.SetProtocol("HTTP/2")
	require.Equal(t, "HTTP/2", c.Protocol())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Protocol -benchmem -count=4
func Benchmark_Ctx_Protocol(b *testing.B) {
	app := New()

	c := app.NewCtx(&fasthttp.RequestCtx{})

	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.Protocol()
	}

	require.Equal(b, "HTTP/1.1", res)
}

// go test -run Test_Ctx_Scheme
func Test_Ctx_Scheme(t *testing.T) {
	app := New()

	freq := &fasthttp.RequestCtx{}
	freq.Request.Header.Set("X-Forwarded", "invalid")

	c := app.NewCtx(freq)

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

// go test -v -run=^$ -bench=Benchmark_Ctx_Scheme -benchmem -count=4
func Benchmark_Ctx_Scheme(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.Scheme()
	}
	require.Equal(b, "http", res)
}

// go test -run Test_Ctx_Scheme_TrustedProxy
func Test_Ctx_Scheme_TrustedProxy(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0"}})
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0/30"}})
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"1.1.1.1/30"}})
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.8.0.1"}})
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().URI().SetQueryString("search=john&age=20")
	require.Equal(t, "john", c.Query("search"))
	require.Equal(t, "20", c.Query("age"))
	require.Equal(t, "default", c.Query("unknown", "default"))

	// test with generic
	require.Equal(t, "john", Query[string](c, "search"))
	require.Equal(t, "20", Query[string](c, "age"))
	require.Equal(t, "default", Query[string](c, "unknown", "default"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Query -benchmem -count=4
func Benchmark_Ctx_Query(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().URI().SetQueryString("search=john&age=8")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = Query[string](c, "search")
	}
	require.Equal(b, "john", res)
}

// go test -run Test_Ctx_QuerySignedInt
func Test_Ctx_QuerySignedInt(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().URI().SetQueryString("search=john&age=8")
	// int
	require.Equal(t, 0, Query[int](c, "foo"))
	require.Equal(t, 8, Query[int](c, "age", 12))
	require.Equal(t, 0, Query[int](c, "search"))
	require.Equal(t, 1, Query[int](c, "search", 1))
	require.Equal(t, 0, Query[int](c, "id"))
	require.Equal(t, 2, Query[int](c, "id", 2))

	// int8
	require.Equal(t, int8(0), Query[int8](c, "foo"))
	require.Equal(t, int8(8), Query[int8](c, "age", 12))
	require.Equal(t, int8(0), Query[int8](c, "search"))
	require.Equal(t, int8(1), Query[int8](c, "search", 1))
	require.Equal(t, int8(0), Query[int8](c, "id"))
	require.Equal(t, int8(2), Query[int8](c, "id", 2))

	// int16
	require.Equal(t, int16(0), Query[int16](c, "foo"))
	require.Equal(t, int16(8), Query[int16](c, "age", 12))
	require.Equal(t, int16(0), Query[int16](c, "search"))
	require.Equal(t, int16(1), Query[int16](c, "search", 1))
	require.Equal(t, int16(0), Query[int16](c, "id"))
	require.Equal(t, int16(2), Query[int16](c, "id", 2))

	// int32
	require.Equal(t, int32(0), Query[int32](c, "foo"))
	require.Equal(t, int32(8), Query[int32](c, "age", 12))
	require.Equal(t, int32(0), Query[int32](c, "search"))
	require.Equal(t, int32(1), Query[int32](c, "search", 1))
	require.Equal(t, int32(0), Query[int32](c, "id"))
	require.Equal(t, int32(2), Query[int32](c, "id", 2))

	// int64
	require.Equal(t, int64(0), Query[int64](c, "foo"))
	require.Equal(t, int64(8), Query[int64](c, "age", 12))
	require.Equal(t, int64(0), Query[int64](c, "search"))
	require.Equal(t, int64(1), Query[int64](c, "search", 1))
	require.Equal(t, int64(0), Query[int64](c, "id"))
	require.Equal(t, int64(2), Query[int64](c, "id", 2))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_QuerySignedInt -benchmem -count=4
func Benchmark_Ctx_QuerySignedInt(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().URI().SetQueryString("search=john&age=8")
	var res int
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = Query[int](c, "age")
	}
	require.Equal(b, 8, res)
}

// go test -run Test_Ctx_QueryBoundarySignedInt
func Test_Ctx_QueryBoundarySignedInt(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	var q string

	// int
	q = fmt.Sprintf("minus=%s&plus=%s&minus_over=%s&plus_over=%s", "2147483647", "-2147483648", "-2147483649", "2147483648")
	c.Request().URI().SetQueryString(q)
	require.Equal(t, 2147483647, Query[int](c, "minus"))
	require.Equal(t, -2147483648, Query[int](c, "plus"))
	require.Equal(t, 0, Query[int](c, "minus_over"))
	require.Equal(t, 0, Query[int](c, "plus_over"))

	// int8
	q = fmt.Sprintf("minus=%s&plus=%s&minus_over=%s&plus_over=%s", "127", "-128", "-129", "128")
	c.Request().URI().SetQueryString(q)
	require.Equal(t, int8(127), Query[int8](c, "minus"))
	require.Equal(t, int8(-128), Query[int8](c, "plus"))
	require.Equal(t, int8(0), Query[int8](c, "minus_over"))
	require.Equal(t, int8(0), Query[int8](c, "plus_over"))

	// int16
	q = fmt.Sprintf("minus=%s&plus=%s&minus_over=%s&plus_over=%s", "32767", "-32768", "-32769", "32768")
	c.Request().URI().SetQueryString(q)
	require.Equal(t, int16(32767), Query[int16](c, "minus"))
	require.Equal(t, int16(-32768), Query[int16](c, "plus"))
	require.Equal(t, int16(0), Query[int16](c, "minus_over"))
	require.Equal(t, int16(0), Query[int16](c, "plus_over"))

	// int32
	q = fmt.Sprintf("minus=%s&plus=%s&minus_over=%s&plus_over=%s", "2147483647", "-2147483648", "-2147483649", "2147483648")
	c.Request().URI().SetQueryString(q)
	require.Equal(t, int32(2147483647), Query[int32](c, "minus"))
	require.Equal(t, int32(-2147483648), Query[int32](c, "plus"))
	require.Equal(t, int32(0), Query[int32](c, "minus_over"))
	require.Equal(t, int32(0), Query[int32](c, "plus_over"))

	// int64
	q = fmt.Sprintf("minus=%s&plus=%s", "-9223372036854775808", "9223372036854775807")
	c.Request().URI().SetQueryString(q)
	require.Equal(t, int64(-9223372036854775808), Query[int64](c, "minus"))
	require.Equal(t, int64(9223372036854775807), Query[int64](c, "plus"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_QueryBoundarySignedInt -benchmem -count=4
func Benchmark_Ctx_QueryBoundarySignedInt(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().URI().SetQueryString("search=john&age=8")
	var res int
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = Query[int](c, "age")
	}
	require.Equal(b, 8, res)
}

// go test -run Test_Ctx_QueryUnsignedInt
func Test_Ctx_QueryUnsignedInt(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().URI().SetQueryString("search=john&age=8")
	// uint
	require.Equal(t, uint(0), Query[uint](c, "foo"))
	require.Equal(t, uint(8), Query[uint](c, "age", 12))
	require.Equal(t, uint(0), Query[uint](c, "search"))
	require.Equal(t, uint(1), Query[uint](c, "search", 1))
	require.Equal(t, uint(0), Query[uint](c, "id"))
	require.Equal(t, uint(2), Query[uint](c, "id", 2))

	// uint8
	require.Equal(t, uint8(0), Query[uint8](c, "foo"))
	require.Equal(t, uint8(8), Query[uint8](c, "age", 12))
	require.Equal(t, uint8(0), Query[uint8](c, "search"))
	require.Equal(t, uint8(1), Query[uint8](c, "search", 1))
	require.Equal(t, uint8(0), Query[uint8](c, "id"))
	require.Equal(t, uint8(2), Query[uint8](c, "id", 2))

	// uint16
	require.Equal(t, uint16(0), Query[uint16](c, "foo"))
	require.Equal(t, uint16(8), Query[uint16](c, "age", 12))
	require.Equal(t, uint16(0), Query[uint16](c, "search"))
	require.Equal(t, uint16(1), Query[uint16](c, "search", 1))
	require.Equal(t, uint16(0), Query[uint16](c, "id"))
	require.Equal(t, uint16(2), Query[uint16](c, "id", 2))

	// uint32
	require.Equal(t, uint32(0), Query[uint32](c, "foo"))
	require.Equal(t, uint32(8), Query[uint32](c, "age", 12))
	require.Equal(t, uint32(0), Query[uint32](c, "search"))
	require.Equal(t, uint32(1), Query[uint32](c, "search", 1))
	require.Equal(t, uint32(0), Query[uint32](c, "id"))
	require.Equal(t, uint32(2), Query[uint32](c, "id", 2))

	// uint64
	require.Equal(t, uint64(0), Query[uint64](c, "foo"))
	require.Equal(t, uint64(8), Query[uint64](c, "age", 12))
	require.Equal(t, uint64(0), Query[uint64](c, "search"))
	require.Equal(t, uint64(1), Query[uint64](c, "search", 1))
	require.Equal(t, uint64(0), Query[uint64](c, "id"))
	require.Equal(t, uint64(2), Query[uint64](c, "id", 2))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_QueryUnsignedInt -benchmem -count=4
func Benchmark_Ctx_QueryUnsignedInt(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().URI().SetQueryString("search=john&age=8")
	var res uint
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = Query[uint](c, "age")
	}
	require.Equal(b, uint(8), res)
}

// go test -run Test_Ctx_QueryBoundaryUnsignedInt
func Test_Ctx_QueryBoundaryUnsignedInt(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	var q string

	// uint
	q = fmt.Sprintf("minus=%s&plus=%s&minus_over=%s&plus_over=%s", "0", "4294967295", "4294967296", "4294967297")
	c.Request().URI().SetQueryString(q)
	require.Equal(t, uint(0), Query[uint](c, "minus"))
	require.Equal(t, uint(4294967295), Query[uint](c, "plus"))
	require.Equal(t, uint(0), Query[uint](c, "minus_over"))
	require.Equal(t, uint(0), Query[uint](c, "plus_over"))

	// uint8
	q = fmt.Sprintf("minus=%s&plus=%s&minus_over=%s&plus_over=%s", "0", "255", "256", "257")
	c.Request().URI().SetQueryString(q)
	require.Equal(t, uint8(0), Query[uint8](c, "minus"))
	require.Equal(t, uint8(255), Query[uint8](c, "plus"))
	require.Equal(t, uint8(0), Query[uint8](c, "minus_over"))
	require.Equal(t, uint8(0), Query[uint8](c, "plus_over"))

	// uint16
	q = fmt.Sprintf("minus=%s&plus=%s&minus_over=%s&plus_over=%s", "0", "65535", "65536", "65537")
	c.Request().URI().SetQueryString(q)
	require.Equal(t, uint16(0), Query[uint16](c, "minus"))
	require.Equal(t, uint16(65535), Query[uint16](c, "plus"))
	require.Equal(t, uint16(0), Query[uint16](c, "minus_over"))
	require.Equal(t, uint16(0), Query[uint16](c, "plus_over"))

	// uint32
	q = fmt.Sprintf("minus=%s&plus=%s&minus_over=%s&plus_over=%s", "0", "4294967295", "4294967296", "4294967297")
	c.Request().URI().SetQueryString(q)
	require.Equal(t, uint32(0), Query[uint32](c, "minus"))
	require.Equal(t, uint32(4294967295), Query[uint32](c, "plus"))
	require.Equal(t, uint32(0), Query[uint32](c, "minus_over"))
	require.Equal(t, uint32(0), Query[uint32](c, "plus_over"))

	// uint64
	q = fmt.Sprintf("minus=%s&plus=%s", "0", "18446744073709551615")
	c.Request().URI().SetQueryString(q)
	require.Equal(t, uint64(0), Query[uint64](c, "minus"))
	require.Equal(t, uint64(18446744073709551615), Query[uint64](c, "plus"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_QueryBoundaryUnsignedInt -benchmem -count=4
func Benchmark_Ctx_QueryBoundaryUnsignedInt(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().URI().SetQueryString("search=john&age=8")
	var res uint
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = Query[uint](c, "age")
	}
	require.Equal(b, uint(8), res)
}

// go test -run Test_Ctx_QueryFloat
func Test_Ctx_QueryFloat(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().URI().SetQueryString("name=alex&amount=32.23&id=")

	// float32
	require.InEpsilon(t, float32(32.23), Query[float32](c, "amount"), epsilon)
	require.InEpsilon(t, float32(32.23), Query[float32](c, "amount", 3.123), epsilon)
	require.InEpsilon(t, float32(87.123), Query[float32](c, "name", 87.123), epsilon)
	require.InDelta(t, float32(0), Query[float32](c, "name"), 0)
	require.InEpsilon(t, float32(12.87), Query[float32](c, "id", 12.87), epsilon)
	require.InDelta(t, float32(0), Query[float32](c, "id"), 0)

	// float64
	require.InEpsilon(t, 32.23, Query[float64](c, "amount"), epsilon)
	require.InEpsilon(t, 32.23, Query[float64](c, "amount", 3.123), epsilon)
	require.InEpsilon(t, 87.123, Query[float64](c, "name", 87.123), epsilon)
	require.InDelta(t, float64(0), Query[float64](c, "name"), 0)
	require.InEpsilon(t, 12.87, Query[float64](c, "id", 12.87), epsilon)
	require.InDelta(t, float64(0), Query[float64](c, "id"), 0)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_QueryFloat -benchmem -count=4
func Benchmark_Ctx_QueryFloat(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().URI().SetQueryString("search=john&age=8")
	var res float32
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = Query[float32](c, "age")
	}
	require.InEpsilon(b, float32(8), res, epsilon)
}

// go test -run Test_Ctx_QueryBool
func Test_Ctx_QueryBool(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().URI().SetQueryString("name=alex&want_pizza=false&id=")

	require.False(t, Query[bool](c, "want_pizza"))
	require.False(t, Query[bool](c, "want_pizza", true))
	require.False(t, Query[bool](c, "name"))
	require.True(t, Query[bool](c, "name", true))
	require.False(t, Query[bool](c, "id"))
	require.True(t, Query[bool](c, "id", true))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_QueryBool -benchmem -count=4
func Benchmark_Ctx_QueryBool(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().URI().SetQueryString("search=john&age=8")
	var res bool
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = Query[bool](c, "age")
	}
	require.False(b, res)
}

// go test -run Test_Ctx_QueryString
func Test_Ctx_QueryString(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().URI().SetQueryString("name=alex&amount=32.23&id=")

	require.Equal(t, "alex", Query[string](c, "name"))
	require.Equal(t, "alex", Query[string](c, "name", "john"))
	require.Equal(t, "32.23", Query[string](c, "amount"))
	require.Equal(t, "32.23", Query[string](c, "amount", "3.123"))
	require.Equal(t, "", Query[string](c, "id"))
	require.Equal(t, "12.87", Query[string](c, "id", "12.87"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_QueryString -benchmem -count=4
func Benchmark_Ctx_QueryString(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().URI().SetQueryString("search=john&age=8")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = Query[string](c, "age")
	}
	require.Equal(b, "8", res)
}

// go test -run Test_Ctx_QueryBytes
func Test_Ctx_QueryBytes(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().URI().SetQueryString("name=alex&amount=32.23&id=")

	require.Equal(t, []byte("alex"), Query[[]byte](c, "name"))
	require.Equal(t, []byte("alex"), Query[[]byte](c, "name", []byte("john")))
	require.Equal(t, []byte("32.23"), Query[[]byte](c, "amount"))
	require.Equal(t, []byte("32.23"), Query[[]byte](c, "amount", []byte("3.123")))
	require.Equal(t, []byte(nil), Query[[]byte](c, "id"))
	require.Equal(t, []byte("12.87"), Query[[]byte](c, "id", []byte("12.87")))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_QueryBytes -benchmem -count=4
func Benchmark_Ctx_QueryBytes(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().URI().SetQueryString("search=john&age=8")
	var res []byte
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = Query[[]byte](c, "age")
	}
	require.Equal(b, []byte("8"), res)
}

// go test -run Test_Ctx_QueryWithoutGenericDataType
func Test_Ctx_QueryWithoutGenericDataType(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().URI().SetQueryString("name=alex&amount=32.23&isAgent=true&id=32")

	require.Equal(t, "alex", Query(c, "name", "john"))
	require.Equal(t, "john", Query(c, "unknown", "john"))
	require.Equal(t, 32, Query(c, "id", 3))
	require.Equal(t, 3, Query(c, "unknown", 3))
	require.Equal(t, int8(32), Query(c, "id", int8(3)))
	require.Equal(t, int8(3), Query(c, "unknown", int8(3)))
	require.Equal(t, int16(32), Query(c, "id", int16(3)))
	require.Equal(t, int16(3), Query(c, "unknown", int16(3)))
	require.Equal(t, int32(32), Query(c, "id", int32(3)))
	require.Equal(t, int32(3), Query(c, "unknown", int32(3)))
	require.Equal(t, int64(32), Query(c, "id", int64(3)))
	require.Equal(t, int64(3), Query(c, "unknown", int64(3)))
	require.Equal(t, uint(32), Query(c, "id", uint(3)))
	require.Equal(t, uint(3), Query(c, "unknown", uint(3)))
	require.Equal(t, uint8(32), Query(c, "id", uint8(3)))
	require.Equal(t, uint8(3), Query(c, "unknown", uint8(3)))
	require.Equal(t, uint16(32), Query(c, "id", uint16(3)))
	require.Equal(t, uint16(3), Query(c, "unknown", uint16(3)))
	require.Equal(t, uint32(32), Query(c, "id", uint32(3)))
	require.Equal(t, uint32(3), Query(c, "unknown", uint32(3)))
	require.Equal(t, uint64(32), Query(c, "id", uint64(3)))
	require.Equal(t, uint64(3), Query(c, "unknown", uint64(3)))
	require.InEpsilon(t, 32.23, Query(c, "amount", 3.123), epsilon)
	require.InEpsilon(t, 3.123, Query(c, "unknown", 3.123), epsilon)
	require.InEpsilon(t, float32(32.23), Query(c, "amount", float32(3.123)), epsilon)
	require.InEpsilon(t, float32(3.123), Query(c, "unknown", float32(3.123)), epsilon)
	require.True(t, Query(c, "isAgent", false))
	require.False(t, Query(c, "unknown", false))
	require.Equal(t, []byte("alex"), Query(c, "name", []byte("john")))
	require.Equal(t, []byte("john"), Query(c, "unknown", []byte("john")))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_QueryWithoutGenericDataType -benchmem -count=4
func Benchmark_Ctx_QueryWithoutGenericDataType(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().URI().SetQueryString("search=john&age=8")
	var res int
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = Query(c, "age", 3)
	}
	require.Equal(b, 8, res)
}

// go test -run Test_Ctx_Range
func Test_Ctx_Range(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	testRange := func(header string, ranges ...RangeSet) {
		c.Request().Header.Set(HeaderRange, header)
		result, err := c.Range(1000)
		if len(ranges) == 0 {
			require.Error(t, err)
		} else {
			require.Equal(t, "bytes", result.Type)
			require.NoError(t, err)
		}
		require.Equal(t, len(ranges), len(result.Ranges))
		for i := range ranges {
			require.Equal(t, ranges[i], result.Ranges[i])
		}
	}

	testRange("bytes=500")
	testRange("bytes=")
	testRange("bytes=500=")
	testRange("bytes=500-300")
	testRange("bytes=a-700", RangeSet{300, 999})
	testRange("bytes=500-b", RangeSet{500, 999})
	testRange("bytes=500-1000", RangeSet{500, 999})
	testRange("bytes=500-700", RangeSet{500, 700})
	testRange("bytes=0-0,2-1000", RangeSet{0, 0}, RangeSet{2, 999})
	testRange("bytes=0-99,450-549,-100", RangeSet{0, 99}, RangeSet{450, 549}, RangeSet{900, 999})
	testRange("bytes=500-700,601-999", RangeSet{500, 700}, RangeSet{601, 999})
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Range -benchmem -count=4
func Benchmark_Ctx_Range(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	testCases := []struct {
		str   string
		start int
		end   int
	}{
		{"bytes=-700", 300, 999},
		{"bytes=500-", 500, 999},
		{"bytes=500-1000", 500, 999},
		{"bytes=0-700,800-1000", 0, 700},
	}

	for _, tc := range testCases {
		b.Run(tc.str, func(b *testing.B) {
			c.Request().Header.Set(HeaderRange, tc.str)
			var (
				result Range
				err    error
			)
			for n := 0; n < b.N; n++ {
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
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	c := app.NewCtx(&fasthttp.RequestCtx{})

	require.Equal(t, "/", c.Route().Path)
	require.Equal(t, MethodGet, c.Route().Method)
	require.Empty(t, c.Route().Handlers)
}

// go test -run Test_Ctx_RouteNormalized
func Test_Ctx_RouteNormalized(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c Ctx) error {
		require.Equal(t, "/test", c.Route().Path)
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "//test", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_SaveFile
func Test_Ctx_SaveFile(t *testing.T) {
	// TODO We should clean this up
	t.Parallel()
	app := New()

	app.Post("/test", func(c Ctx) error {
		fh, err := c.FormFile("file")
		require.NoError(t, err)

		tempFile, err := os.CreateTemp(os.TempDir(), "test-")
		require.NoError(t, err)

		defer func(file *os.File) {
			err := file.Close()
			require.NoError(t, err)
			err = os.Remove(file.Name())
			require.NoError(t, err)
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

// go test -run Test_Ctx_Secure
func Test_Ctx_Secure(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	// TODO Add TLS conn
	require.False(t, c.Secure())
}

// go test -run Test_Ctx_Stale
func Test_Ctx_Stale(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	require.True(t, c.Stale())
}

// go test -run Test_Ctx_Subdomains
func Test_Ctx_Subdomains(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().URI().SetHost("john.doe.is.awesome.google.com")
	require.Equal(t, []string{"john", "doe"}, c.Subdomains(4))

	c.Request().URI().SetHost("localhost:3000")
	require.Equal(t, []string{"localhost:3000"}, c.Subdomains())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Subdomains -benchmem -count=4
func Benchmark_Ctx_Subdomains(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().SetRequestURI("http://john.doe.google.com")
	var res []string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.Subdomains()
	}
	require.Equal(b, []string{"john", "doe"}, res)
}

// go test -run Test_Ctx_ClearCookie
func Test_Ctx_ClearCookie(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderCookie, "john=doe")
	c.ClearCookie("john")
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
	c := app.NewCtx(&fasthttp.RequestCtx{})

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

	require.NoError(t, c.Download("ctx.go"))
	require.Equal(t, `attachment; filename="ctx.go"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
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
	c := app.NewCtx(&fasthttp.RequestCtx{})
	err = c.SendFile("ctx.go")
	// check expectation
	require.NoError(t, err)
	require.Equal(t, expectFileContent, c.Response().Body())
	require.Equal(t, StatusOK, c.Response().StatusCode())
	app.ReleaseCtx(c)

	// test with custom error code
	c = app.NewCtx(&fasthttp.RequestCtx{})
	err = c.Status(StatusInternalServerError).SendFile("ctx.go")
	// check expectation
	require.NoError(t, err)
	require.Equal(t, expectFileContent, c.Response().Body())
	require.Equal(t, StatusInternalServerError, c.Response().StatusCode())
	app.ReleaseCtx(c)

	// test not modified
	c = app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderIfModifiedSince, fI.ModTime().Format(time.RFC1123))
	err = c.SendFile("ctx.go")
	// check expectation
	require.NoError(t, err)
	require.Equal(t, StatusNotModified, c.Response().StatusCode())
	require.Equal(t, []byte(nil), c.Response().Body())
	app.ReleaseCtx(c)
}

// go test -race -run Test_Ctx_SendFile_404
func Test_Ctx_SendFile_404(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		err := c.SendFile(filepath.FromSlash("john_dow.go/"))
		require.Error(t, err)
		return err
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
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
		endpoint := endpoint
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()
			// 1st try
			resp, err := app.Test(httptest.NewRequest(MethodGet, endpoint, nil))
			require.NoError(t, err)
			require.Equal(t, StatusOK, resp.StatusCode)
			// 2nd try
			resp, err = app.Test(httptest.NewRequest(MethodGet, endpoint, nil))
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

	_, err1 := app.Test(httptest.NewRequest(MethodGet, "/?test=true", nil))
	// second request required to confirm with zero allocation
	_, err2 := app.Test(httptest.NewRequest(MethodGet, "/?test=true", nil))

	require.NoError(t, err1)
	require.NoError(t, err2)
}

// go test -run Test_Ctx_JSON
func Test_Ctx_JSON(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	require.Error(t, c.JSON(complex(1, 1)))

	// Test without ctype
	err := c.JSON(Map{ // map has no order
		"Name": "Grame",
		"Age":  20,
	})
	require.NoError(t, err)
	require.Equal(t, `{"Age":20,"Name":"Grame"}`, string(c.Response().Body()))
	require.Equal(t, "application/json", string(c.Response().Header.Peek("content-type")))

	// Test with ctype
	err = c.JSON(Map{ // map has no order
		"Name": "Grame",
		"Age":  20,
	}, "application/problem+json")
	require.NoError(t, err)
	require.Equal(t, `{"Age":20,"Name":"Grame"}`, string(c.Response().Body()))
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
			JSONEncoder: func(v any) ([]byte, error) {
				return []byte(`["custom","json"]`), nil
			},
		})
		c := app.NewCtx(&fasthttp.RequestCtx{})

		err := c.JSON(Map{ // map has no order
			"Name": "Grame",
			"Age":  20,
		})
		require.NoError(t, err)
		require.Equal(t, `["custom","json"]`, string(c.Response().Body()))
		require.Equal(t, "application/json", string(c.Response().Header.Peek("content-type")))
	})
}

// go test -run=^$ -bench=Benchmark_Ctx_JSON -benchmem -count=4
func Benchmark_Ctx_JSON(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err = c.JSON(data)
	}
	require.NoError(b, err)
	require.Equal(b, `{"Name":"Grame","Age":20}`, string(c.Response().Body()))
}

// go test -run=^$ -bench=Benchmark_Ctx_JSON_Ctype -benchmem -count=4
func Benchmark_Ctx_JSON_Ctype(b *testing.B) {
	app := New()
	// TODO: Check extra allocs because of the interface stuff
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed
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
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err = c.JSON(data, "application/problem+json")
	}
	require.NoError(b, err)
	require.Equal(b, `{"Name":"Grame","Age":20}`, string(c.Response().Body()))
	require.Equal(b, "application/problem+json", string(c.Response().Header.Peek("content-type")))
}

// go test -run Test_Ctx_JSONP
func Test_Ctx_JSONP(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	require.Error(t, c.JSONP(complex(1, 1)))

	err := c.JSONP(Map{
		"Name": "Grame",
		"Age":  20,
	})
	require.NoError(t, err)
	require.Equal(t, `callback({"Age":20,"Name":"Grame"});`, string(c.Response().Body()))
	require.Equal(t, "text/javascript; charset=utf-8", string(c.Response().Header.Peek("content-type")))

	err = c.JSONP(Map{
		"Name": "Grame",
		"Age":  20,
	}, "john")
	require.NoError(t, err)
	require.Equal(t, `john({"Age":20,"Name":"Grame"});`, string(c.Response().Body()))
	require.Equal(t, "text/javascript; charset=utf-8", string(c.Response().Header.Peek("content-type")))

	t.Run("custom json encoder", func(t *testing.T) {
		t.Parallel()

		app := New(Config{
			JSONEncoder: func(v any) ([]byte, error) {
				return []byte(`["custom","json"]`), nil
			},
		})
		c := app.NewCtx(&fasthttp.RequestCtx{})

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
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

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
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err = c.JSONP(data, callback)
	}
	require.NoError(b, err)
	require.Equal(b, `emit({"Name":"Grame","Age":20});`, string(c.Response().Body()))
}

// go test -run Test_Ctx_XML
func Test_Ctx_XML(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

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
	require.Equal(t, "application/xml", string(c.Response().Header.Peek("content-type")))

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
			XMLEncoder: func(v any) ([]byte, error) {
				return []byte(`<custom>xml</custom>`), nil
			},
		})
		c := app.NewCtx(&fasthttp.RequestCtx{})

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
		require.Equal(t, "application/xml", string(c.Response().Header.Peek("content-type")))
	})
}

// go test -run=^$ -bench=Benchmark_Ctx_XML -benchmem -count=4
func Benchmark_Ctx_XML(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed
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
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err = c.XML(data)
	}

	require.NoError(b, err)
	require.Equal(b, `<SomeStruct><Name>Grame</Name><Age>20</Age></SomeStruct>`, string(c.Response().Body()))
}

// go test -run Test_Ctx_Links
func Test_Ctx_Links(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Links()
	require.Equal(t, "", string(c.Response().Header.Peek(HeaderLink)))

	c.Links(
		"http://api.example.com/users?page=2", "next",
		"http://api.example.com/users?page=5", "last",
	)
	require.Equal(t, `<http://api.example.com/users?page=2>; rel="next",<http://api.example.com/users?page=5>; rel="last"`, string(c.Response().Header.Peek(HeaderLink)))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Links -benchmem -count=4
func Benchmark_Ctx_Links(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
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
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	resp, err := app.Test(httptest.NewRequest(MethodGet, "http://example.com/test", nil))
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "http://example.com/test", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")
	require.Equal(t, "Works", resp.Header.Get("X-Next-Result"))
}

// go test -run Test_Ctx_Render
func Test_Ctx_Render(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	err := c.Render("./.github/testdata/index.tmpl", Map{
		"Title": "Hello, World!",
	})
	require.NoError(t, err)

	require.Equal(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))

	err = c.Render("./.github/testdata/template-non-exists.html", nil)
	require.Error(t, err)

	err = c.Render("./.github/testdata/template-invalid.html", nil)
	require.Error(t, err)
}

func Test_Ctx_RenderWithoutLocals(t *testing.T) {
	t.Parallel()
	app := New(Config{
		PassLocalsToViews: false,
	})
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Locals("Title", "Hello, World!")

	err := c.Render("./.github/testdata/index.tmpl", Map{})
	require.NoError(t, err)
	require.Equal(t, "<h1><no value></h1>", string(c.Response().Body()))
}

func Test_Ctx_RenderWithLocals(t *testing.T) {
	t.Parallel()
	app := New(Config{
		PassLocalsToViews: true,
	})

	t.Run("EmptyBind", func(t *testing.T) {
		t.Parallel()
		c := app.NewCtx(&fasthttp.RequestCtx{})

		c.Locals("Title", "Hello, World!")
		err := c.Render("./.github/testdata/index.tmpl", Map{})

		require.NoError(t, err)
		require.Equal(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
	})

	t.Run("NilBind", func(t *testing.T) {
		t.Parallel()
		c := app.NewCtx(&fasthttp.RequestCtx{})

		c.Locals("Title", "Hello, World!")
		err := c.Render("./.github/testdata/index.tmpl", nil)

		require.NoError(t, err)
		require.Equal(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
	})
}

func Test_Ctx_RenderWithBindVars(t *testing.T) {
	t.Parallel()

	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	err := c.BindVars(Map{
		"Title": "Hello, World!",
	})
	require.NoError(t, err)

	err = c.Render("./.github/testdata/index.tmpl", Map{})
	require.NoError(t, err)
	buf := bytebufferpool.Get()
	_, _ = buf.WriteString("overwrite") //nolint:errcheck // This will never fail
	defer bytebufferpool.Put(buf)

	require.NoError(t, err)
	require.Equal(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

func Test_Ctx_RenderWithOverwrittenBind(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	err := c.BindVars(Map{
		"Title": "Hello, World!",
	})
	require.NoError(t, err)

	err = c.Render("./.github/testdata/index.tmpl", Map{
		"Title": "Hello from Fiber!",
	})
	require.NoError(t, err)

	buf := bytebufferpool.Get()
	_, _ = buf.WriteString("overwrite") //nolint:errcheck // This will never fail
	defer bytebufferpool.Put(buf)

	require.Equal(t, "<h1>Hello from Fiber!</h1>", string(c.Response().Body()))
}

func Test_Ctx_RenderWithBindVarsLocals(t *testing.T) {
	t.Parallel()
	app := New(Config{
		PassLocalsToViews: true,
	})

	c := app.NewCtx(&fasthttp.RequestCtx{})

	err := c.BindVars(Map{
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
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Locals("Title", "This is a test.")

	err = c.Render("index.tmpl", Map{
		"Title": "Hello, World!",
	})

	require.NoError(t, err)
	require.Equal(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

func Benchmark_Ctx_RenderWithLocalsAndBindVars(b *testing.B) {
	engine := &testTemplateEngine{}
	err := engine.Load()
	require.NoError(b, err)
	app := New(Config{
		PassLocalsToViews: true,
		Views:             engine,
	})
	c := app.NewCtx(&fasthttp.RequestCtx{})

	err = c.BindVars(Map{
		"Title": "Hello, World!",
	})
	require.NoError(b, err)
	c.Locals("Summary", "Test")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
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
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Locals("Title", "Hello, World!")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		err = c.Render("index.tmpl", Map{})
	}

	require.NoError(b, err)
	require.Equal(b, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

func Benchmark_Ctx_RenderBindVars(b *testing.B) {
	engine := &testTemplateEngine{}
	err := engine.Load()
	require.NoError(b, err)
	app := New()
	app.config.Views = engine
	c := app.NewCtx(&fasthttp.RequestCtx{})

	err = c.BindVars(Map{
		"Title": "Hello, World!",
	})
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
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
	resp, err := app.Test(httptest.NewRequest(MethodGet, "http://example.com/", nil))
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
	app.Get("/old", func(c Ctx) error {
		executedOldHandler = true
		return nil
	})
	app.Get("/new", func(c Ctx) error {
		executedNewHandler = true
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "http://example.com/old", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	require.False(t, executedOldHandler, "Executed old handler")
	require.True(t, executedNewHandler, "Executed new handler")
}

// go test -run Test_Ctx_RestartRoutingWithChangedPathAnd404
func Test_Ctx_RestartRoutingWithChangedPathAndCatchAll(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/new", func(c Ctx) error {
		return nil
	})
	app.Use(func(c Ctx) error {
		c.Path("/new")
		// c.Next() would fail this test as a 404 is returned from the next handler
		return c.RestartRouting()
	})
	app.Use(func(c Ctx) error {
		return ErrNotFound
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "http://example.com/old", nil))
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
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	c := app.NewCtx(&fasthttp.RequestCtx{})

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
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
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	app.Get("/user/:name", func(c Ctx) error {
		return c.SendString(c.Params("name"))
	}).Name("User")

	var err error
	var location string
	for n := 0; n < b.N; n++ {
		location, err = c.getLocationFromRoute(app.GetRoute("User"), Map{"name": "fiber"})
	}

	require.Equal(b, "/user/fiber", location)
	require.NoError(b, err)
}

// go test -run Test_Ctx_Get_Location_From_Route_name
func Test_Ctx_Get_Location_From_Route_name(t *testing.T) {
	t.Parallel()

	t.Run("case insensitive", func(t *testing.T) {
		t.Parallel()
		app := New()
		c := app.NewCtx(&fasthttp.RequestCtx{})
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

	t.Run("case sensitive", func(t *testing.T) {
		t.Parallel()
		app := New(Config{CaseSensitive: true})
		c := app.NewCtx(&fasthttp.RequestCtx{})
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
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	c := app.NewCtx(&fasthttp.RequestCtx{})

	err := c.Render("index.tmpl", nil)
	require.Error(t, err)
}

// go test -run Test_Ctx_Render_Go_Template
func Test_Ctx_Render_Go_Template(t *testing.T) {
	t.Parallel()
	file, err := os.CreateTemp(os.TempDir(), "fiber")
	require.NoError(t, err)
	defer func() {
		err := os.Remove(file.Name())
		require.NoError(t, err)
	}()

	_, err = file.Write([]byte("template"))
	require.NoError(t, err)

	err = file.Close()
	require.NoError(t, err)

	app := New()

	c := app.NewCtx(&fasthttp.RequestCtx{})

	err = c.Render(file.Name(), nil)
	require.NoError(t, err)
	require.Equal(t, "template", string(c.Response().Body()))
}

// go test -run Test_Ctx_Send
func Test_Ctx_Send(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	require.NoError(t, c.Send([]byte("Hello, World")))
	require.NoError(t, c.Send([]byte("Don't crash please")))
	require.NoError(t, c.Send([]byte("1337")))
	require.Equal(t, "1337", string(c.Response().Body()))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Send -benchmem -count=4
func Benchmark_Ctx_Send(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	byt := []byte("Hello, World!")
	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		err = c.Send(byt)
	}
	require.NoError(b, err)
	require.Equal(b, "Hello, World!", string(c.Response().Body()))
}

// go test -run Test_Ctx_SendStatus
func Test_Ctx_SendStatus(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	err := c.SendStatus(415)
	require.NoError(t, err)
	require.Equal(t, 415, c.Response().StatusCode())
	require.Equal(t, "Unsupported Media Type", string(c.Response().Body()))
}

// go test -run Test_Ctx_SendString
func Test_Ctx_SendString(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	err := c.SendString("Don't crash please")
	require.NoError(t, err)
	require.Equal(t, "Don't crash please", string(c.Response().Body()))
}

// go test -run Test_Ctx_SendStream
func Test_Ctx_SendStream(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

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

// go test -run Test_Ctx_Set
func Test_Ctx_Set(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	c := app.NewCtx(&fasthttp.RequestCtx{})

	val := "1431-15132-3423"
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Set(HeaderXRequestID, val)
	}
}

// go test -run Test_Ctx_Status
func Test_Ctx_Status(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Type(".json")
	require.Equal(t, "application/json", string(c.Response().Header.Peek("Content-Type")))

	c.Type("json", "utf-8")
	require.Equal(t, "application/json; charset=utf-8", string(c.Response().Header.Peek("Content-Type")))

	c.Type(".html")
	require.Equal(t, "text/html", string(c.Response().Header.Peek("Content-Type")))

	c.Type("html", "utf-8")
	require.Equal(t, "text/html; charset=utf-8", string(c.Response().Header.Peek("Content-Type")))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Type -benchmem -count=4
func Benchmark_Ctx_Type(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Type(".json")
		c.Type("json")
	}
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Type_Charset -benchmem -count=4
func Benchmark_Ctx_Type_Charset(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Type(".json", "utf-8")
		c.Type("json", "utf-8")
	}
}

// go test -run Test_Ctx_Vary
func Test_Ctx_Vary(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Vary("Origin")
	c.Vary("User-Agent")
	c.Vary("Accept-Encoding", "Accept")
	require.Equal(t, "Origin, User-Agent, Accept-Encoding, Accept", string(c.Response().Header.Peek("Vary")))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Vary -benchmem -count=4
func Benchmark_Ctx_Vary(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Vary("Origin", "User-Agent")
	}
}

// go test -run Test_Ctx_Write
func Test_Ctx_Write(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	_, err := c.Write([]byte("Hello, "))
	require.NoError(t, err)
	_, err = c.Write([]byte("World!"))
	require.NoError(t, err)
	require.Equal(t, "Hello, World!", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Write -benchmem -count=4
func Benchmark_Ctx_Write(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	byt := []byte("Hello, World!")
	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		_, err = c.Write(byt)
	}
	require.NoError(b, err)
}

// go test -run Test_Ctx_Writef
func Test_Ctx_Writef(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	world := "World!"
	_, err := c.Writef("Hello, %s", world)
	require.NoError(t, err)
	require.Equal(t, "Hello, World!", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Writef -benchmem -count=4
func Benchmark_Ctx_Writef(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	world := "World!"
	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		_, err = c.Writef("Hello, %s", world)
	}
	require.NoError(b, err)
}

// go test -run Test_Ctx_WriteString
func Test_Ctx_WriteString(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

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
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXRequestedWith, "XMLHttpRequest")
	require.True(t, c.XHR())
}

// go test -run=^$ -bench=Benchmark_Ctx_XHR -benchmem -count=4
func Benchmark_Ctx_XHR(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXRequestedWith, "XMLHttpRequest")
	var equal bool
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		equal = c.XHR()
	}
	require.True(b, equal)
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_SendString_B -benchmem -count=4
func Benchmark_Ctx_SendString_B(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	body := "Hello, world!"
	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		err = c.SendString(body)
	}
	require.NoError(b, err)
	require.Equal(b, []byte("Hello, world!"), c.Response().Body())
}

// go test -run Test_Ctx_Queries -v
func Test_Ctx_Queries(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball,football&favouriteDrinks=milo,coke,pepsi&alloc=&no=1&field1=value1&field1=value2&field2=value3&list_a=1&list_a=2&list_a=3&list_b[]=1&list_b[]=2&list_b[]=3&list_c=1,2,3")

	queries := c.Queries()
	require.Equal(t, "1", queries["id"])
	require.Equal(t, "tom", queries["name"])
	require.Equal(t, "basketball,football", queries["hobby"])
	require.Equal(t, "milo,coke,pepsi", queries["favouriteDrinks"])
	require.Equal(t, "", queries["alloc"])
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

	queries = c.Queries()
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
	c := app.NewCtx(&fasthttp.RequestCtx{})

	b.ReportAllocs()
	b.ResetTimer()
	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball,football&favouriteDrinks=milo,coke,pepsi&alloc=&no=1")

	var queries map[string]string
	for n := 0; n < b.N; n++ {
		queries = c.Queries()
	}

	require.Equal(b, "1", queries["id"])
	require.Equal(b, "tom", queries["name"])
	require.Equal(b, "basketball,football", queries["hobby"])
	require.Equal(b, "milo,coke,pepsi", queries["favouriteDrinks"])
	require.Equal(b, "", queries["alloc"])
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
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		ctx.ResetBody()
		ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
			for i := 0; i < 10; i++ {
				_, err = w.Write(user)
				if err := w.Flush(); err != nil {
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
	c := app.NewCtx(&fasthttp.RequestCtx{})

	require.Equal(t, "#0000000000000000 - 0.0.0.0:0 <-> 0.0.0.0:0 - GET http:///", c.String())
}

func TestCtx_ParamsInt(t *testing.T) {
	// Create a test context and set some strings (or params)
	// create a fake app to be used within this test
	t.Parallel()
	app := New()

	// Create some test endpoints

	// For the user id I will use the number 1111, so I should be able to get the number
	// 1111 from the Ctx
	app.Get("/test/:user", func(c Ctx) error {
		// require.Equal(t, "john", c.Params("user"))

		num, err := c.ParamsInt("user")

		// Check the number matches
		require.Equal(t, 1111, num)

		// Check no errors are returned, because we want NO errors in this one
		require.NoError(t, err)

		return nil
	})

	// In this test case, there will be a bad request where the expected number is NOT
	// a number in the path
	app.Get("/testnoint/:user", func(c Ctx) error {
		// require.Equal(t, "john", c.Params("user"))

		num, err := c.ParamsInt("user")

		// Check the number matches
		require.Equal(t, 0, num)

		// Check an error is returned, because we want NO errors in this one
		require.Error(t, err)

		return nil
	})

	// For the user id I will use the number 2222, so I should be able to get the number
	// 2222 from the Ctx even when the default value is specified
	app.Get("/testignoredefault/:user", func(c Ctx) error {
		// require.Equal(t, "john", c.Params("user"))

		num, err := c.ParamsInt("user", 1111)

		// Check the number matches
		require.Equal(t, 2222, num)

		// Check no errors are returned, because we want NO errors in this one
		require.NoError(t, err)

		return nil
	})

	// In this test case, there will be a bad request where the expected number is NOT
	// a number in the path, default value of 1111 should be used instead
	app.Get("/testdefault/:user", func(c Ctx) error {
		// require.Equal(t, "john", c.Params("user"))

		num, err := c.ParamsInt("user", 1111)

		// Check the number matches
		require.Equal(t, 1111, num)

		// Check an error is returned, because we want NO errors in this one
		require.NoError(t, err)

		return nil
	})

	_, err := app.Test(httptest.NewRequest(MethodGet, "/test/1111", nil))
	require.NoError(t, err)

	_, err = app.Test(httptest.NewRequest(MethodGet, "/testnoint/xd", nil))
	require.NoError(t, err)

	_, err = app.Test(httptest.NewRequest(MethodGet, "/testignoredefault/2222", nil))
	require.NoError(t, err)

	_, err = app.Test(httptest.NewRequest(MethodGet, "/testdefault/xd", nil))
	require.NoError(t, err)
}

// go test -run Test_Ctx_IsFromLocal_X_Forwarded
func Test_Ctx_IsFromLocal_X_Forwarded(t *testing.T) {
	t.Parallel()
	// Test unset X-Forwarded-For header.
	{
		app := New()
		c := app.NewCtx(&fasthttp.RequestCtx{})
		// fasthttp returns "0.0.0.0" as IP as there is no remote address.
		require.Equal(t, "0.0.0.0", c.IP())
		require.False(t, c.IsFromLocal())
	}
	// Test when setting X-Forwarded-For header to localhost "127.0.0.1"
	{
		app := New()
		c := app.NewCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1")
		defer app.ReleaseCtx(c)
		require.False(t, c.IsFromLocal())
	}
	// Test when setting X-Forwarded-For header to localhost "::1"
	{
		app := New()
		c := app.NewCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "::1")
		defer app.ReleaseCtx(c)
		require.False(t, c.IsFromLocal())
	}
	// Test when setting X-Forwarded-For to full localhost IPv6 address "0:0:0:0:0:0:0:1"
	{
		app := New()
		c := app.NewCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "0:0:0:0:0:0:0:1")
		defer app.ReleaseCtx(c)
		require.False(t, c.IsFromLocal())
	}
	// Test for a random IP address.
	{
		app := New()
		c := app.NewCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "93.46.8.90")

		require.False(t, c.IsFromLocal())
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
		c := app.NewCtx(fastCtx)

		require.Equal(t, "127.0.0.1", c.IP())
		require.True(t, c.IsFromLocal())
	}
	// Test for the case fasthttp remoteAddr is set to "::1".
	{
		app := New()
		fastCtx := &fasthttp.RequestCtx{}
		fastCtx.SetRemoteAddr(localIPv6)
		c := app.NewCtx(fastCtx)
		require.Equal(t, "::1", c.IP())
		require.True(t, c.IsFromLocal())
	}
	// Test for the case fasthttp remoteAddr is set to "0:0:0:0:0:0:0:1".
	{
		app := New()
		fastCtx := &fasthttp.RequestCtx{}
		fastCtx.SetRemoteAddr(localIPv6long)
		c := app.NewCtx(fastCtx)
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
		c := app.NewCtx(fastCtx)
		require.Equal(t, "0.0.0.0", c.IP())
		require.False(t, c.IsFromLocal())
	}
	// Test for the case fasthttp remoteAddr is set to "93.46.8.90".
	{
		app := New()
		fastCtx := &fasthttp.RequestCtx{}
		fastCtx.SetRemoteAddr(someIPv4)
		c := app.NewCtx(fastCtx)
		require.Equal(t, "93.46.8.90", c.IP())
		require.False(t, c.IsFromLocal())
	}
	// Test for the case fasthttp remoteAddr is set to "2001:0db8:85a3:0000:0000:8a2e:0370:7334".
	{
		app := New()
		fastCtx := &fasthttp.RequestCtx{}
		fastCtx.SetRemoteAddr(someIPv6)
		c := app.NewCtx(fastCtx)
		require.Equal(t, "2001:db8:85a3::8a2e:370:7334", c.IP())
		require.False(t, c.IsFromLocal())
	}
}

// go test -run Test_Ctx_extractIPsFromHeader -v
func Test_Ctx_extractIPsFromHeader(t *testing.T) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set("x-forwarded-for", "1.1.1.1,8.8.8.8 , /n, \n,1.1, a.c, 6.,6., , a,,42.118.81.169,10.0.137.108")
	ips := c.IPs()
	res := ips[len(ips)-2]
	require.Equal(t, "42.118.81.169", res)
}

// go test -run Test_Ctx_extractIPsFromHeader -v
func Test_Ctx_extractIPsFromHeader_EnableValidateIp(t *testing.T) {
	app := New()
	app.config.EnableIPValidation = true
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set("x-forwarded-for", "1.1.1.1,8.8.8.8 , /n, \n,1.1, a.c, 6.,6., , a,,42.118.81.169,10.0.137.108")
	ips := c.IPs()
	res := ips[len(ips)-2]
	require.Equal(t, "42.118.81.169", res)
}
