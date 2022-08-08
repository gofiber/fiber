// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

// go test -v -run=^$ -bench=Benchmark_Ctx_Accepts -benchmem -count=4
// go test -run Test_Ctx

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Ctx_Accepts
func Test_Ctx_Accepts(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAccept, "text/html,application/xhtml+xml,application/xml;q=0.9")
	utils.AssertEqual(t, "", c.Accepts(""))
	utils.AssertEqual(t, "", c.Accepts())
	utils.AssertEqual(t, ".xml", c.Accepts(".xml"))
	utils.AssertEqual(t, "", c.Accepts(".john"))

	c.Request().Header.Set(HeaderAccept, "text/*, application/json")
	utils.AssertEqual(t, "html", c.Accepts("html"))
	utils.AssertEqual(t, "text/html", c.Accepts("text/html"))
	utils.AssertEqual(t, "json", c.Accepts("json", "text"))
	utils.AssertEqual(t, "application/json", c.Accepts("application/json"))
	utils.AssertEqual(t, "", c.Accepts("image/png"))
	utils.AssertEqual(t, "", c.Accepts("png"))

	c.Request().Header.Set(HeaderAccept, "text/html, application/json")
	utils.AssertEqual(t, "text/*", c.Accepts("text/*"))

	c.Request().Header.Set(HeaderAccept, "*/*")
	utils.AssertEqual(t, "html", c.Accepts("html"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Accepts -benchmem -count=4
func Benchmark_Ctx_Accepts(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	c.Request().Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.Accepts(".xml")
	}
	utils.AssertEqual(b, ".xml", res)
}

type customCtx struct {
	DefaultCtx
}

func (c *customCtx) Params(key string, defaultValue ...string) string {
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
	resp, err := app.Test(httptest.NewRequest("GET", "/v3", &bytes.Buffer{}))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "io.ReadAll(resp.Body)")
	utils.AssertEqual(t, "prefix_v3", string(body))
}

// go test -run Test_Ctx_Accepts_EmptyAccept
func Test_Ctx_Accepts_EmptyAccept(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	utils.AssertEqual(t, ".forwarded", c.Accepts(".forwarded"))
}

// go test -run Test_Ctx_Accepts_Wildcard
func Test_Ctx_Accepts_Wildcard(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAccept, "*/*;q=0.9")
	utils.AssertEqual(t, "html", c.Accepts("html"))
	utils.AssertEqual(t, "foo", c.Accepts("foo"))
	utils.AssertEqual(t, ".bar", c.Accepts(".bar"))
	c.Request().Header.Set(HeaderAccept, "text/html,application/*;q=0.9")
	utils.AssertEqual(t, "xml", c.Accepts("xml"))
}

// go test -run Test_Ctx_AcceptsCharsets
func Test_Ctx_AcceptsCharsets(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAcceptCharset, "utf-8, iso-8859-1;q=0.5")
	utils.AssertEqual(t, "utf-8", c.AcceptsCharsets("utf-8"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsCharsets -benchmem -count=4
func Benchmark_Ctx_AcceptsCharsets(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	c.Request().Header.Set("Accept-Charset", "utf-8, iso-8859-1;q=0.5")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.AcceptsCharsets("utf-8")
	}
	utils.AssertEqual(b, "utf-8", res)
}

// go test -run Test_Ctx_AcceptsEncodings
func Test_Ctx_AcceptsEncodings(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAcceptEncoding, "deflate, gzip;q=1.0, *;q=0.5")
	utils.AssertEqual(t, "gzip", c.AcceptsEncodings("gzip"))
	utils.AssertEqual(t, "abc", c.AcceptsEncodings("abc"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsEncodings -benchmem -count=4
func Benchmark_Ctx_AcceptsEncodings(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	c.Request().Header.Set(HeaderAcceptEncoding, "deflate, gzip;q=1.0, *;q=0.5")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.AcceptsEncodings("gzip")
	}
	utils.AssertEqual(b, "gzip", res)
}

// go test -run Test_Ctx_AcceptsLanguages
func Test_Ctx_AcceptsLanguages(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAcceptLanguage, "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")
	utils.AssertEqual(t, "fr", c.AcceptsLanguages("fr"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsLanguages -benchmem -count=4
func Benchmark_Ctx_AcceptsLanguages(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	c.Request().Header.Set(HeaderAcceptLanguage, "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.AcceptsLanguages("fr")
	}
	utils.AssertEqual(b, "fr", res)
}

// go test -run Test_Ctx_App
func Test_Ctx_App(t *testing.T) {
	t.Parallel()
	app := New()
	app.config.BodyLimit = 1000
	c := app.NewCtx(&fasthttp.RequestCtx{})

	utils.AssertEqual(t, 1000, c.App().config.BodyLimit)
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

	utils.AssertEqual(t, "Hello, World", string(c.Response().Header.Peek("X-Test")))
	utils.AssertEqual(t, "World, XHello, Hello", string(c.Response().Header.Peek("X2-Test")))
	utils.AssertEqual(t, "XHello, World, Hello", string(c.Response().Header.Peek("X3-Test")))
	utils.AssertEqual(t, "XHello, Hello, HelloZ, YHello", string(c.Response().Header.Peek("X4-Test")))
	utils.AssertEqual(t, "", string(c.Response().Header.Peek("x-custom-header")))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Append -benchmem -count=4
func Benchmark_Ctx_Append(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Append("X-Custom-Header", "Hello")
		c.Append("X-Custom-Header", "World")
		c.Append("X-Custom-Header", "Hello")
	}
	utils.AssertEqual(b, "Hello, World", app.getString(c.Response().Header.Peek("X-Custom-Header")))
}

// go test -run Test_Ctx_Attachment
func Test_Ctx_Attachment(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	// empty
	c.Attachment()
	utils.AssertEqual(t, `attachment`, string(c.Response().Header.Peek(HeaderContentDisposition)))
	// real filename
	c.Attachment("./static/img/logo.png")
	utils.AssertEqual(t, `attachment; filename="logo.png"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
	utils.AssertEqual(t, "image/png", string(c.Response().Header.Peek(HeaderContentType)))
	// check quoting
	c.Attachment("another document.pdf\"\r\nBla: \"fasel")
	utils.AssertEqual(t, `attachment; filename="another+document.pdf%22%0D%0ABla%3A+%22fasel"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Attachment -benchmem -count=4
func Benchmark_Ctx_Attachment(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// example with quote params
		c.Attachment("another document.pdf\"\r\nBla: \"fasel")
	}
	utils.AssertEqual(b, `attachment; filename="another+document.pdf%22%0D%0ABla%3A+%22fasel"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
}

// go test -run Test_Ctx_BaseURL
func Test_Ctx_BaseURL(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().SetRequestURI("http://google.com/test")
	utils.AssertEqual(t, "http://google.com", c.BaseURL())
	// Check cache
	utils.AssertEqual(t, "http://google.com", c.BaseURL())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_BaseURL -benchmem
func Benchmark_Ctx_BaseURL(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	c.Request().SetHost("google.com:1337")
	c.Request().URI().SetPath("/haha/oke/lol")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.BaseURL()
	}
	utils.AssertEqual(b, "http://google.com:1337", res)
}

// go test -run Test_Ctx_Body
func Test_Ctx_Body(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().SetBody([]byte("john=doe"))
	utils.AssertEqual(t, []byte("john=doe"), c.Body())
}

// go test -run Test_Ctx_Body_With_Compression
func Test_Ctx_Body_With_Compression(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Content-Encoding", "gzip")
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, err := gz.Write([]byte("john=doe"))
	utils.AssertEqual(t, nil, err)
	err = gz.Flush()
	utils.AssertEqual(t, nil, err)
	err = gz.Close()
	utils.AssertEqual(t, nil, err)
	c.Request().SetBody(b.Bytes())
	utils.AssertEqual(t, []byte("john=doe"), c.Body())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Body_With_Compression -benchmem -count=4
func Benchmark_Ctx_Body_With_Compression(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Content-Encoding", "gzip")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write([]byte("john=doe"))
	utils.AssertEqual(b, nil, err)
	err = gz.Flush()
	utils.AssertEqual(b, nil, err)
	err = gz.Close()
	utils.AssertEqual(b, nil, err)

	c.Request().SetBody(buf.Bytes())

	for i := 0; i < b.N; i++ {
		_ = c.Body()
	}

	utils.AssertEqual(b, []byte("john=doe"), c.Body())
}

// go test -run Test_Ctx_Context
func Test_Ctx_Context(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	utils.AssertEqual(t, "*fasthttp.RequestCtx", fmt.Sprintf("%T", c.Context()))
}

// go test -run Test_Ctx_UserContext
func Test_Ctx_UserContext(t *testing.T) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	t.Run("Nil_Context", func(t *testing.T) {
		ctx := c.UserContext()
		utils.AssertEqual(t, ctx, context.Background())
	})
	t.Run("ValueContext", func(t *testing.T) {
		testKey := "Test Key"
		testValue := "Test Value"
		ctx := context.WithValue(context.Background(), testKey, testValue)
		utils.AssertEqual(t, testValue, ctx.Value(testKey))
	})
}

// go test -run Test_Ctx_SetUserContext
func Test_Ctx_SetUserContext(t *testing.T) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	testKey := "Test Key"
	testValue := "Test Value"
	ctx := context.WithValue(context.Background(), testKey, testValue)
	c.SetUserContext(ctx)
	utils.AssertEqual(t, testValue, c.UserContext().Value(testKey))
}

// go test -run Test_Ctx_UserContext_Multiple_Requests
func Test_Ctx_UserContext_Multiple_Requests(t *testing.T) {
	testKey := "foobar-key"
	testValue := "foobar-value"

	app := New()
	app.Get("/", func(c Ctx) error {
		ctx := c.UserContext()

		if ctx.Value(testKey) != nil {
			return c.SendStatus(StatusInternalServerError)
		}

		input := utils.CopyString(c.Query("input", "NO_VALUE"))
		ctx = context.WithValue(ctx, testKey, fmt.Sprintf("%s_%s", testValue, input))
		c.SetUserContext(ctx)

		return c.Status(StatusOK).SendString(fmt.Sprintf("resp_%s_returned", input))
	})

	// Consecutive Requests
	for i := 1; i <= 10; i++ {
		t.Run(fmt.Sprintf("request_%d", i), func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest(MethodGet, fmt.Sprintf("/?input=%d", i), nil))

			utils.AssertEqual(t, nil, err, "Unexpected error from response")
			utils.AssertEqual(t, StatusOK, resp.StatusCode, "context.Context returned from c.UserContext() is reused")

			b, err := io.ReadAll(resp.Body)
			utils.AssertEqual(t, nil, err, "Unexpected error from reading response body")
			utils.AssertEqual(t, fmt.Sprintf("resp_%d_returned", i), string(b), "response text incorrect")
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
	utils.AssertEqual(t, expect, string(c.Response().Header.Peek(HeaderSetCookie)))

	expect = "username=john; expires=" + httpdate + "; path=/"
	cookie.SameSite = CookieSameSiteDisabled
	c.Cookie(cookie)
	utils.AssertEqual(t, expect, string(c.Response().Header.Peek(HeaderSetCookie)))

	expect = "username=john; expires=" + httpdate + "; path=/; SameSite=Strict"
	cookie.SameSite = CookieSameSiteStrictMode
	c.Cookie(cookie)
	utils.AssertEqual(t, expect, string(c.Response().Header.Peek(HeaderSetCookie)))

	expect = "username=john; expires=" + httpdate + "; path=/; secure; SameSite=None"
	cookie.Secure = true
	cookie.SameSite = CookieSameSiteNoneMode
	c.Cookie(cookie)
	utils.AssertEqual(t, expect, string(c.Response().Header.Peek(HeaderSetCookie)))

	expect = "username=john; path=/; secure; SameSite=None"
	// should remove expires and max-age headers
	cookie.SessionOnly = true
	cookie.Expires = expire
	cookie.MaxAge = 10000
	c.Cookie(cookie)
	utils.AssertEqual(t, expect, string(c.Response().Header.Peek(HeaderSetCookie)))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Cookie -benchmem -count=4
func Benchmark_Ctx_Cookie(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Cookie(&Cookie{
			Name:  "John",
			Value: "Doe",
		})
	}
	utils.AssertEqual(b, "John=Doe; path=/; SameSite=Lax", app.getString(c.Response().Header.Peek("Set-Cookie")))
}

// go test -run Test_Ctx_Cookies
func Test_Ctx_Cookies(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Cookie", "john=doe")
	utils.AssertEqual(t, "doe", c.Cookies("john"))
	utils.AssertEqual(t, "default", c.Cookies("unknown", "default"))
}

// go test -run Test_Ctx_Format
func Test_Ctx_Format(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderAccept, MIMETextPlain)
	c.Format([]byte("Hello, World!"))
	utils.AssertEqual(t, "Hello, World!", string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMETextHTML)
	c.Format("Hello, World!")
	utils.AssertEqual(t, "<p>Hello, World!</p>", string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMEApplicationJSON)
	c.Format("Hello, World!")
	utils.AssertEqual(t, `"Hello, World!"`, string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMETextPlain)
	c.Format(complex(1, 1))
	utils.AssertEqual(t, "(1+1i)", string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMEApplicationXML)
	c.Format("Hello, World!")
	utils.AssertEqual(t, `<string>Hello, World!</string>`, string(c.Response().Body()))

	err := c.Format(complex(1, 1))
	utils.AssertEqual(t, true, err != nil)

	c.Request().Header.Set(HeaderAccept, MIMETextPlain)
	c.Format(Map{})
	utils.AssertEqual(t, "map[]", string(c.Response().Body()))

	type broken string
	c.Request().Header.Set(HeaderAccept, "broken/accept")
	c.Format(broken("Hello, World!"))
	utils.AssertEqual(t, `Hello, World!`, string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Format -benchmem -count=4
func Benchmark_Ctx_Format(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Accept", "text/plain")
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	utils.AssertEqual(b, `Hello, World!`, string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Format_HTML -benchmem -count=4
func Benchmark_Ctx_Format_HTML(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Accept", "text/html")
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	utils.AssertEqual(b, "<p>Hello, World!</p>", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Format_JSON -benchmem -count=4
func Benchmark_Ctx_Format_JSON(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Accept", "application/json")
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	utils.AssertEqual(b, `"Hello, World!"`, string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Format_XML -benchmem -count=4
func Benchmark_Ctx_Format_XML(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set("Accept", "application/xml")
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	utils.AssertEqual(b, `<string>Hello, World!</string>`, string(c.Response().Body()))
}

// go test -run Test_Ctx_FormFile
func Test_Ctx_FormFile(t *testing.T) {
	// TODO: We should clean this up
	t.Parallel()
	app := New()

	app.Post("/test", func(c Ctx) error {
		fh, err := c.FormFile("file")
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "test", fh.Filename)

		f, err := fh.Open()
		utils.AssertEqual(t, nil, err)

		b := new(bytes.Buffer)
		_, err = io.Copy(b, f)
		utils.AssertEqual(t, nil, err)

		f.Close()
		utils.AssertEqual(t, "hello world", b.String())
		return nil
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	ioWriter, err := writer.CreateFormFile("file", "test")
	utils.AssertEqual(t, nil, err)

	_, err = ioWriter.Write([]byte("hello world"))
	utils.AssertEqual(t, nil, err)

	writer.Close()

	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set(HeaderContentType, writer.FormDataContentType())
	req.Header.Set(HeaderContentLength, strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_FormValue
func Test_Ctx_FormValue(t *testing.T) {
	t.Parallel()
	app := New()

	app.Post("/test", func(c Ctx) error {
		utils.AssertEqual(t, "john", c.FormValue("name"))
		return nil
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	utils.AssertEqual(t, nil, writer.WriteField("name", "john"))

	writer.Close()
	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", writer.Boundary()))
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
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

	utils.AssertEqual(t, false, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, "*")
	c.Request().Header.Set(HeaderCacheControl, "no-cache")
	utils.AssertEqual(t, false, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, "*")
	c.Request().Header.Set(HeaderCacheControl, ",no-cache,")
	utils.AssertEqual(t, false, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, "*")
	c.Request().Header.Set(HeaderCacheControl, "aa,no-cache,")
	utils.AssertEqual(t, false, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, "*")
	c.Request().Header.Set(HeaderCacheControl, ",no-cache,bb")
	utils.AssertEqual(t, false, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, "675af34563dc-tr34")
	c.Request().Header.Set(HeaderCacheControl, "public")
	utils.AssertEqual(t, false, c.Fresh())

	c.Request().Header.Set(HeaderIfNoneMatch, "a, b")
	c.Response().Header.Set(HeaderETag, "c")
	utils.AssertEqual(t, false, c.Fresh())

	c.Response().Header.Set(HeaderETag, "a")
	utils.AssertEqual(t, true, c.Fresh())

	c.Request().Header.Set(HeaderIfModifiedSince, "xxWed, 21 Oct 2015 07:28:00 GMT")
	c.Response().Header.Set(HeaderLastModified, "xxWed, 21 Oct 2015 07:28:00 GMT")
	utils.AssertEqual(t, false, c.Fresh())

	c.Response().Header.Set(HeaderLastModified, "Wed, 21 Oct 2015 07:28:00 GMT")
	utils.AssertEqual(t, false, c.Fresh())

	c.Request().Header.Set(HeaderIfModifiedSince, "Wed, 21 Oct 2015 07:28:00 GMT")
	utils.AssertEqual(t, false, c.Fresh())
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
	utils.AssertEqual(t, "utf-8, iso-8859-1;q=0.5", c.Get(HeaderAcceptCharset))
	utils.AssertEqual(t, "Monster", c.Get(HeaderReferer))
	utils.AssertEqual(t, "default", c.Get("unknown", "default"))
}

// go test -run Test_Ctx_Hostname
func Test_Ctx_Hostname(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().SetRequestURI("http://google.com/test")
	utils.AssertEqual(t, "google.com", c.Hostname())
}

// go test -run Test_Ctx_Hostname_Untrusted
func Test_Ctx_Hostname_UntrustedProxy(t *testing.T) {
	t.Parallel()
	// Don't trust any proxy
	{
		app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{}})
		c := app.NewCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		utils.AssertEqual(t, "google.com", c.Hostname())
		app.ReleaseCtx(c)
	}
	// Trust to specific proxy list
	{
		app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.8.0.0", "0.8.0.1"}})
		c := app.NewCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		utils.AssertEqual(t, "google.com", c.Hostname())
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
		utils.AssertEqual(t, "google1.com", c.Hostname())
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
	utils.AssertEqual(t, "google1.com", c.Hostname())
	app.ReleaseCtx(c)
}

// go test -run Test_Ctx_Hostname_UntrustedProxyRange
func Test_Ctx_Hostname_UntrustedProxyRange(t *testing.T) {
	t.Parallel()

	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"1.0.0.0/30"}})
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com/test")
	c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
	utils.AssertEqual(t, "google.com", c.Hostname())
	app.ReleaseCtx(c)
}

// go test -run Test_Ctx_Port
func Test_Ctx_Port(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	utils.AssertEqual(t, "0", c.Port())
}

// go test -run Test_Ctx_PortInHandler
func Test_Ctx_PortInHandler(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/port", func(c Ctx) error {
		return c.SendString(c.Port())
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/port", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "0", string(body))
}

// go test -run Test_Ctx_IP
func Test_Ctx_IP(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	utils.AssertEqual(t, "0.0.0.0", c.IP())
}

// go test -run Test_Ctx_IP_ProxyHeader
func Test_Ctx_IP_ProxyHeader(t *testing.T) {
	t.Parallel()
	app := New(Config{ProxyHeader: "Real-Ip"})
	c := app.NewCtx(&fasthttp.RequestCtx{})

	utils.AssertEqual(t, "", c.IP())
}

// go test -run Test_Ctx_IP_UntrustedProxy
func Test_Ctx_IP_UntrustedProxy(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.8.0.1"}, ProxyHeader: HeaderXForwardedFor})
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderXForwardedFor, "0.0.0.1")

	utils.AssertEqual(t, "0.0.0.0", c.IP())
}

// go test -run Test_Ctx_IP_TrustedProxy
func Test_Ctx_IP_TrustedProxy(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0"}, ProxyHeader: HeaderXForwardedFor})
	c := app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderXForwardedFor, "0.0.0.1")

	utils.AssertEqual(t, "0.0.0.1", c.IP())
}

// go test -run Test_Ctx_IPs  -parallel
func Test_Ctx_IPs(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, 127.0.0.2, 127.0.0.3")
	utils.AssertEqual(t, []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}, c.IPs())

	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1,127.0.0.2  ,127.0.0.3")
	utils.AssertEqual(t, []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}, c.IPs())

	c.Request().Header.Set(HeaderXForwardedFor, "")
	utils.AssertEqual(t, 0, len(c.IPs()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_IPs -benchmem -count=4
func Benchmark_Ctx_IPs(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, 127.0.0.1, 127.0.0.1")
	var res []string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IPs()
	}
	utils.AssertEqual(b, []string{"127.0.0.1", "127.0.0.1", "127.0.0.1"}, res)
}

// go test -run Test_Ctx_Is
func Test_Ctx_Is(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderContentType, MIMETextHTML+"; boundary=something")
	utils.AssertEqual(t, true, c.Is(".html"))
	utils.AssertEqual(t, true, c.Is("html"))
	utils.AssertEqual(t, false, c.Is("json"))
	utils.AssertEqual(t, false, c.Is(".json"))
	utils.AssertEqual(t, false, c.Is(""))
	utils.AssertEqual(t, false, c.Is(".foooo"))

	c.Request().Header.Set(HeaderContentType, MIMEApplicationJSONCharsetUTF8)
	utils.AssertEqual(t, false, c.Is("html"))
	utils.AssertEqual(t, true, c.Is("json"))
	utils.AssertEqual(t, true, c.Is(".json"))

	c.Request().Header.Set(HeaderContentType, " application/json;charset=UTF-8")
	utils.AssertEqual(t, false, c.Is("html"))
	utils.AssertEqual(t, true, c.Is("json"))
	utils.AssertEqual(t, true, c.Is(".json"))

	c.Request().Header.Set(HeaderContentType, MIMEApplicationXMLCharsetUTF8)
	utils.AssertEqual(t, false, c.Is("html"))
	utils.AssertEqual(t, true, c.Is("xml"))
	utils.AssertEqual(t, true, c.Is(".xml"))

	c.Request().Header.Set(HeaderContentType, MIMETextPlain)
	utils.AssertEqual(t, false, c.Is("html"))
	utils.AssertEqual(t, true, c.Is("txt"))
	utils.AssertEqual(t, true, c.Is(".txt"))
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
	utils.AssertEqual(b, true, res)
}

// go test -run Test_Ctx_Locals
func Test_Ctx_Locals(t *testing.T) {
	app := New()
	app.Use(func(c Ctx) error {
		c.Locals("john", "doe")
		return c.Next()
	})
	app.Get("/test", func(c Ctx) error {
		utils.AssertEqual(t, "doe", c.Locals("john"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Method
func Test_Ctx_Method(t *testing.T) {
	t.Parallel()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	app := New()
	c := app.NewCtx(fctx)

	utils.AssertEqual(t, MethodGet, c.Method())
	c.Method(MethodPost)
	utils.AssertEqual(t, MethodPost, c.Method())

	c.Method("MethodInvalid")
	utils.AssertEqual(t, MethodPost, c.Method())
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

	utils.AssertEqual(t, 400, fctx.Response.StatusCode())
	utils.AssertEqual(t, []byte("Invalid http method"), fctx.Response.Body())
}

// go test -run Test_Ctx_MultipartForm
func Test_Ctx_MultipartForm(t *testing.T) {
	t.Parallel()
	app := New()

	app.Post("/test", func(c Ctx) error {
		result, err := c.MultipartForm()
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "john", result.Value["name"][0])
		return nil
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	utils.AssertEqual(t, nil, writer.WriteField("name", "john"))

	writer.Close()
	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set(HeaderContentType, fmt.Sprintf("multipart/form-data; boundary=%s", writer.Boundary()))
	req.Header.Set(HeaderContentLength, strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -v -run=^$ -bench=Benchmark_Ctx_MultipartForm -benchmem -count=4
func Benchmark_Ctx_MultipartForm(b *testing.B) {
	app := New()

	app.Post("/", func(c Ctx) error {
		_, _ = c.MultipartForm()
		return nil
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
	utils.AssertEqual(t, "http://google.com/test?search=demo", c.OriginalURL())
}

// go test -race -run Test_Ctx_Params
func Test_Ctx_Params(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test/:user", func(c Ctx) error {
		utils.AssertEqual(t, "john", c.Params("user"))
		return nil
	})
	app.Get("/test2/*", func(c Ctx) error {
		utils.AssertEqual(t, "im/a/cookie", c.Params("*"))
		return nil
	})
	app.Get("/test3/*/blafasel/*", func(c Ctx) error {
		utils.AssertEqual(t, "1111", c.Params("*1"))
		utils.AssertEqual(t, "2222", c.Params("*2"))
		utils.AssertEqual(t, "1111", c.Params("*"))
		return nil
	})
	app.Get("/test4/:optional?", func(c Ctx) error {
		utils.AssertEqual(t, "", c.Params("optional"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test/john", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test2/im/a/cookie", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test3/1111/blafasel/2222", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test4", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Params -benchmem -count=4
func Benchmark_Ctx_Params(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

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
	utils.AssertEqual(b, "awesome", res)
}

// go test -run Test_Ctx_Path
func Test_Ctx_Path(t *testing.T) {
	t.Parallel()
	app := New(Config{UnescapePath: true})
	app.Get("/test/:user", func(c Ctx) error {
		utils.AssertEqual(t, "/Test/John", c.Path())
		// not strict && case insensitive
		utils.AssertEqual(t, "/ABC/", c.Path("/ABC/"))
		utils.AssertEqual(t, "/test/john/", c.Path("/test/john/"))
		return nil
	})

	// test with special chars
	app.Get("/specialChars/:name", func(c Ctx) error {
		utils.AssertEqual(t, "/specialChars/créer", c.Path())
		// unescape is also working if you set the path afterwards
		utils.AssertEqual(t, "/اختبار/", c.Path("/%D8%A7%D8%AE%D8%AA%D8%A8%D8%A7%D8%B1/"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/specialChars/cr%C3%A9er", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Protocol
func Test_Ctx_Protocol(t *testing.T) {
	app := New()

	c := app.NewCtx(&fasthttp.RequestCtx{})

	utils.AssertEqual(t, "HTTP/1.1", c.Protocol())

	c.Request().Header.SetProtocol("HTTP/2")
	utils.AssertEqual(t, "HTTP/2", c.Protocol())
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

	utils.AssertEqual(b, "HTTP/1.1", res)
}

// go test -run Test_Ctx_Scheme
func Test_Ctx_Scheme(t *testing.T) {
	app := New()

	freq := &fasthttp.RequestCtx{}
	freq.Request.Header.Set("X-Forwarded", "invalid")

	c := app.NewCtx(freq)

	c.Request().Header.Set(HeaderXForwardedProto, "https")
	utils.AssertEqual(t, "https", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, "https")
	utils.AssertEqual(t, "https", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	utils.AssertEqual(t, "https", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, "https")
	utils.AssertEqual(t, "https", c.Scheme())
	c.Request().Header.Reset()

	utils.AssertEqual(t, "http", c.Scheme())
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
	utils.AssertEqual(b, "http", res)
}

// go test -run Test_Ctx_Scheme_TrustedProxy
func Test_Ctx_Scheme_TrustedProxy(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0"}})
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXForwardedProto, "https")
	utils.AssertEqual(t, "https", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, "https")
	utils.AssertEqual(t, "https", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	utils.AssertEqual(t, "https", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, "https")
	utils.AssertEqual(t, "https", c.Scheme())
	c.Request().Header.Reset()

	utils.AssertEqual(t, "http", c.Scheme())
}

// go test -run Test_Ctx_Scheme_TrustedProxyRange
func Test_Ctx_Scheme_TrustedProxyRange(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0/30"}})
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXForwardedProto, "https")
	utils.AssertEqual(t, "https", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, "https")
	utils.AssertEqual(t, "https", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	utils.AssertEqual(t, "https", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, "https")
	utils.AssertEqual(t, "https", c.Scheme())
	c.Request().Header.Reset()

	utils.AssertEqual(t, "http", c.Scheme())
}

// go test -run Test_Ctx_Scheme_UntrustedProxyRange
func Test_Ctx_Scheme_UntrustedProxyRange(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"1.1.1.1/30"}})
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXForwardedProto, "https")
	utils.AssertEqual(t, "http", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, "https")
	utils.AssertEqual(t, "http", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	utils.AssertEqual(t, "http", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, "https")
	utils.AssertEqual(t, "http", c.Scheme())
	c.Request().Header.Reset()

	utils.AssertEqual(t, "http", c.Scheme())
}

// go test -run Test_Ctx_Scheme_UnTrustedProxy
func Test_Ctx_Scheme_UnTrustedProxy(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.8.0.1"}})
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXForwardedProto, "https")
	utils.AssertEqual(t, "http", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, "https")
	utils.AssertEqual(t, "http", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	utils.AssertEqual(t, "http", c.Scheme())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, "https")
	utils.AssertEqual(t, "http", c.Scheme())
	c.Request().Header.Reset()

	utils.AssertEqual(t, "http", c.Scheme())
}

// go test -run Test_Ctx_Query
func Test_Ctx_Query(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().URI().SetQueryString("search=john&age=20")
	utils.AssertEqual(t, "john", c.Query("search"))
	utils.AssertEqual(t, "20", c.Query("age"))
	utils.AssertEqual(t, "default", c.Query("unknown", "default"))
}

// go test -run Test_Ctx_Range
func Test_Ctx_Range(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	var (
		result Range
		err    error
	)

	_, err = c.Range(1000)
	utils.AssertEqual(t, true, err != nil)

	c.Request().Header.Set(HeaderRange, "bytes=500")
	_, err = c.Range(1000)
	utils.AssertEqual(t, true, err != nil)

	c.Request().Header.Set(HeaderRange, "bytes=500=")
	_, err = c.Range(1000)
	utils.AssertEqual(t, true, err != nil)

	c.Request().Header.Set(HeaderRange, "bytes=500-300")
	_, err = c.Range(1000)
	utils.AssertEqual(t, true, err != nil)

	testRange := func(header string, start, end int) {
		c.Request().Header.Set(HeaderRange, header)
		result, err = c.Range(1000)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "bytes", result.Type)
		utils.AssertEqual(t, start, result.Ranges[0].Start)
		utils.AssertEqual(t, end, result.Ranges[0].End)
	}

	testRange("bytes=a-700", 300, 999)
	testRange("bytes=500-b", 500, 999)
	testRange("bytes=500-1000", 500, 999)
	testRange("bytes=500-700", 500, 700)
}

// go test -run Test_Ctx_Route
func Test_Ctx_Route(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c Ctx) error {
		utils.AssertEqual(t, "/test", c.Route().Path)
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	c := app.NewCtx(&fasthttp.RequestCtx{})

	utils.AssertEqual(t, "/", c.Route().Path)
	utils.AssertEqual(t, MethodGet, c.Route().Method)
	utils.AssertEqual(t, 0, len(c.Route().Handlers))
}

// go test -run Test_Ctx_RouteNormalized
func Test_Ctx_RouteNormalized(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c Ctx) error {
		utils.AssertEqual(t, "/test", c.Route().Path)
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "//test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusNotFound, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_SaveFile
func Test_Ctx_SaveFile(t *testing.T) {
	// TODO We should clean this up
	t.Parallel()
	app := New()

	app.Post("/test", func(c Ctx) error {
		fh, err := c.FormFile("file")
		utils.AssertEqual(t, nil, err)

		tempFile, err := os.CreateTemp(os.TempDir(), "test-")
		utils.AssertEqual(t, nil, err)

		defer os.Remove(tempFile.Name())
		err = c.SaveFile(fh, tempFile.Name())
		utils.AssertEqual(t, nil, err)

		bs, err := os.ReadFile(tempFile.Name())
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "hello world", string(bs))
		return nil
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	ioWriter, err := writer.CreateFormFile("file", "test")
	utils.AssertEqual(t, nil, err)

	_, err = ioWriter.Write([]byte("hello world"))
	utils.AssertEqual(t, nil, err)
	writer.Close()

	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_SaveFileToStorage
func Test_Ctx_SaveFileToStorage(t *testing.T) {
	t.Parallel()
	app := New()
	storage := memory.New()

	app.Post("/test", func(c Ctx) error {
		fh, err := c.FormFile("file")
		utils.AssertEqual(t, nil, err)

		err = c.SaveFileToStorage(fh, "test", storage)
		utils.AssertEqual(t, nil, err)

		file, err := storage.Get("test")
		utils.AssertEqual(t, []byte("hello world"), file)
		utils.AssertEqual(t, nil, err)

		err = storage.Delete("test")
		utils.AssertEqual(t, nil, err)

		return nil
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	ioWriter, err := writer.CreateFormFile("file", "test")
	utils.AssertEqual(t, nil, err)

	_, err = ioWriter.Write([]byte("hello world"))
	utils.AssertEqual(t, nil, err)
	writer.Close()

	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Secure
func Test_Ctx_Secure(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	// TODO Add TLS conn
	utils.AssertEqual(t, false, c.Secure())
}

// go test -run Test_Ctx_Stale
func Test_Ctx_Stale(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	utils.AssertEqual(t, true, c.Stale())
}

// go test -run Test_Ctx_Subdomains
func Test_Ctx_Subdomains(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().URI().SetHost("john.doe.is.awesome.google.com")
	utils.AssertEqual(t, []string{"john", "doe"}, c.Subdomains(4))

	c.Request().URI().SetHost("localhost:3000")
	utils.AssertEqual(t, []string{"localhost:3000"}, c.Subdomains())
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
	utils.AssertEqual(b, []string{"john", "doe"}, res)
}

// go test -run Test_Ctx_ClearCookie
func Test_Ctx_ClearCookie(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderCookie, "john=doe")
	c.ClearCookie("john")
	utils.AssertEqual(t, true, strings.HasPrefix(string(c.Response().Header.Peek(HeaderSetCookie)), "john=; expires="))

	c.Request().Header.Set(HeaderCookie, "test1=dummy")
	c.Request().Header.Set(HeaderCookie, "test2=dummy")
	c.ClearCookie()
	utils.AssertEqual(t, true, strings.Contains(string(c.Response().Header.Peek(HeaderSetCookie)), "test1=; expires="))
	utils.AssertEqual(t, true, strings.Contains(string(c.Response().Header.Peek(HeaderSetCookie)), "test2=; expires="))
}

// go test -race -run Test_Ctx_Download
func Test_Ctx_Download(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Download("ctx.go", "Awesome File!")

	f, err := os.Open("./ctx.go")
	utils.AssertEqual(t, nil, err)
	defer f.Close()

	expect, err := io.ReadAll(f)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expect, c.Response().Body())
	utils.AssertEqual(t, `attachment; filename="Awesome+File%21"`, string(c.Response().Header.Peek(HeaderContentDisposition)))

	c.Download("ctx.go")
	utils.AssertEqual(t, `attachment; filename="ctx.go"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
}

// go test -race -run Test_Ctx_SendFile
func Test_Ctx_SendFile(t *testing.T) {
	t.Parallel()
	app := New()

	// fetch file content
	f, err := os.Open("./ctx.go")
	utils.AssertEqual(t, nil, err)
	defer f.Close()
	expectFileContent, err := io.ReadAll(f)
	utils.AssertEqual(t, nil, err)
	// fetch file info for the not modified test case
	fI, err := os.Stat("./ctx.go")
	utils.AssertEqual(t, nil, err)

	// simple test case
	c := app.NewCtx(&fasthttp.RequestCtx{})
	err = c.SendFile("ctx.go")
	// check expectation
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectFileContent, c.Response().Body())
	utils.AssertEqual(t, StatusOK, c.Response().StatusCode())
	app.ReleaseCtx(c)

	// test with custom error code
	c = app.NewCtx(&fasthttp.RequestCtx{})
	err = c.Status(StatusInternalServerError).SendFile("ctx.go")
	// check expectation
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectFileContent, c.Response().Body())
	utils.AssertEqual(t, StatusInternalServerError, c.Response().StatusCode())
	app.ReleaseCtx(c)

	// test not modified
	c = app.NewCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderIfModifiedSince, fI.ModTime().Format(time.RFC1123))
	err = c.SendFile("ctx.go")
	// check expectation
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, StatusNotModified, c.Response().StatusCode())
	utils.AssertEqual(t, []byte(nil), c.Response().Body())
	app.ReleaseCtx(c)
}

// go test -race -run Test_Ctx_SendFile_404
func Test_Ctx_SendFile_404(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		err := c.SendFile(filepath.FromSlash("john_dow.go/"))
		utils.AssertEqual(t, false, err == nil)
		return err
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, StatusNotFound, resp.StatusCode)
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
				utils.AssertEqual(t, nil, err)
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
		utils.AssertEqual(t, nil, err)
	} else {
		addEndpoint(path, "/absolute")
		addEndpoint(filepath.FromSlash(path), "/absoluteOS") // os related
	}

	for _, endpoint := range endpointsForTest {
		t.Run(endpoint, func(t *testing.T) {
			// 1st try
			resp, err := app.Test(httptest.NewRequest("GET", endpoint, nil))
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, StatusOK, resp.StatusCode)
			// 2nd try
			resp, err = app.Test(httptest.NewRequest("GET", endpoint, nil))
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, StatusOK, resp.StatusCode)
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
		utils.AssertEqual(t, originalURL, c.OriginalURL())
		return err
	})

	_, err1 := app.Test(httptest.NewRequest("GET", "/?test=true", nil))
	// second request required to confirm with zero allocation
	_, err2 := app.Test(httptest.NewRequest("GET", "/?test=true", nil))

	utils.AssertEqual(t, nil, err1)
	utils.AssertEqual(t, nil, err2)
}

// go test -run Test_Ctx_JSON
func Test_Ctx_JSON(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	utils.AssertEqual(t, true, c.JSON(complex(1, 1)) != nil)

	c.JSON(Map{ // map has no order
		"Name": "Grame",
		"Age":  20,
	})
	utils.AssertEqual(t, `{"Age":20,"Name":"Grame"}`, string(c.Response().Body()))
	utils.AssertEqual(t, "application/json", string(c.Response().Header.Peek("content-type")))

	testEmpty := func(v any, r string) {
		err := c.JSON(v)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, r, string(c.Response().Body()))
	}

	testEmpty(nil, "null")
	testEmpty("", `""`)
	testEmpty(0, "0")
	testEmpty([]int{}, "[]")
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
	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, `{"Name":"Grame","Age":20}`, string(c.Response().Body()))
}

// go test -run Test_Ctx_JSONP
func Test_Ctx_JSONP(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	utils.AssertEqual(t, true, c.JSONP(complex(1, 1)) != nil)

	c.JSONP(Map{
		"Name": "Grame",
		"Age":  20,
	})
	utils.AssertEqual(t, `callback({"Age":20,"Name":"Grame"});`, string(c.Response().Body()))
	utils.AssertEqual(t, "application/javascript; charset=utf-8", string(c.Response().Header.Peek("content-type")))

	c.JSONP(Map{
		"Name": "Grame",
		"Age":  20,
	}, "john")
	utils.AssertEqual(t, `john({"Age":20,"Name":"Grame"});`, string(c.Response().Body()))
	utils.AssertEqual(t, "application/javascript; charset=utf-8", string(c.Response().Header.Peek("content-type")))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_JSONP -benchmem -count=4
func Benchmark_Ctx_JSONP(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

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
	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, `emit({"Name":"Grame","Age":20});`, string(c.Response().Body()))
}

// go test -run Test_Ctx_Links
func Test_Ctx_Links(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Links()
	utils.AssertEqual(t, "", string(c.Response().Header.Peek(HeaderLink)))

	c.Links(
		"http://api.example.com/users?page=2", "next",
		"http://api.example.com/users?page=5", "last",
	)
	utils.AssertEqual(t, `<http://api.example.com/users?page=2>; rel="next",<http://api.example.com/users?page=5>; rel="last"`, string(c.Response().Header.Peek(HeaderLink)))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Links -benchmem -count=4
func Benchmark_Ctx_Links(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

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
	utils.AssertEqual(t, "http://example.com", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_Next
func Test_Ctx_Next(t *testing.T) {
	app := New()
	app.Use("/", func(c Ctx) error {
		return c.Next()
	})
	app.Get("/test", func(c Ctx) error {
		c.Set("X-Next-Result", "Works")
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "http://example.com/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "Works", resp.Header.Get("X-Next-Result"))
}

// go test -run Test_Ctx_Next_Error
func Test_Ctx_Next_Error(t *testing.T) {
	app := New()
	app.Use("/", func(c Ctx) error {
		c.Set("X-Next-Result", "Works")
		return ErrNotFound
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "http://example.com/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusNotFound, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "Works", resp.Header.Get("X-Next-Result"))
}

// go test -run Test_Ctx_Redirect
func Test_Ctx_Redirect(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Redirect("http://default.com")
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "http://default.com", string(c.Response().Header.Peek(HeaderLocation)))

	c.Redirect("http://example.com", 301)
	utils.AssertEqual(t, 301, c.Response().StatusCode())
	utils.AssertEqual(t, "http://example.com", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_RedirectToRouteWithParams
func Test_Ctx_RedirectToRouteWithParams(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.RedirectToRoute("user", Map{
		"name": "fiber",
	})
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_RedirectToRouteWithParams
func Test_Ctx_RedirectToRouteWithQueries(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.RedirectToRoute("user", Map{
		"name":    "fiber",
		"queries": map[string]string{"data[0][name]": "john", "data[0][age]": "10", "test": "doe"},
	})
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	// analysis of query parameters with url parsing, since a map pass is always randomly ordered
	location, err := url.Parse(string(c.Response().Header.Peek(HeaderLocation)))
	utils.AssertEqual(t, nil, err, "url.Parse(location)")
	utils.AssertEqual(t, "/user/fiber", location.Path)
	utils.AssertEqual(t, url.Values{"data[0][name]": []string{"john"}, "data[0][age]": []string{"10"}, "test": []string{"doe"}}, location.Query())
}

// go test -run Test_Ctx_RedirectToRouteWithOptionalParams
func Test_Ctx_RedirectToRouteWithOptionalParams(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name?", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.RedirectToRoute("user", Map{
		"name": "fiber",
	})
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_RedirectToRouteWithOptionalParamsWithoutValue
func Test_Ctx_RedirectToRouteWithOptionalParamsWithoutValue(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name?", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.RedirectToRoute("user", Map{})
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/user/", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_RedirectToRouteWithGreedyParameters
func Test_Ctx_RedirectToRouteWithGreedyParameters(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/+", func(c Ctx) error {
		return c.JSON(c.Params("+"))
	}).Name("user")
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.RedirectToRoute("user", Map{
		"+": "test/routes",
	})
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/user/test/routes", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_RedirectBack
func Test_Ctx_RedirectBack(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		return c.JSON("Home")
	}).Name("home")
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.RedirectBack("/")
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_RedirectBackWithReferer
func Test_Ctx_RedirectBackWithReferer(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		return c.JSON("Home")
	}).Name("home")
	app.Get("/back", func(c Ctx) error {
		return c.JSON("Back")
	}).Name("back")
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderReferer, "/back")
	c.RedirectBack("/")
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/back", c.Get(HeaderReferer))
	utils.AssertEqual(t, "/back", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_Render
func Test_Ctx_Render(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	err := c.Render("./.github/testdata/index.tmpl", Map{
		"Title": "Hello, World!",
	})

	buf := bytebufferpool.Get()
	_, _ = buf.WriteString("overwrite")
	defer bytebufferpool.Put(buf)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))

	err = c.Render("./.github/testdata/template-non-exists.html", nil)
	utils.AssertEqual(t, false, err == nil)

	err = c.Render("./.github/testdata/template-invalid.html", nil)
	utils.AssertEqual(t, false, err == nil)
}

// go test -run Test_Ctx_Render_Mount
func Test_Ctx_Render_Mount(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{}
	engine.Load()

	sub := New(Config{
		Views: engine,
	})

	sub.Get("/:name", func(c Ctx) error {
		return c.Render("hello_world.tmpl", Map{
			"Name": c.Params("name"),
		})
	})

	app := New()
	app.Mount("/hello", sub)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/hello/a", nil))
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, nil, err, "app.Test(req)")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello a!</h1>", string(body))
}

func Test_Ctx_Render_MountGroup(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{}
	engine.Load()

	micro := New(Config{
		Views: engine,
	})

	micro.Get("/doe", func(c Ctx) error {
		return c.Render("hello_world.tmpl", Map{
			"Name": "doe",
		})
	})

	app := New()
	v1 := app.Group("/v1")
	v1.Mount("/john", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello doe!</h1>", string(body))
}

func Test_Ctx_RenderWithoutLocals(t *testing.T) {
	t.Parallel()
	app := New(Config{
		PassLocalsToViews: false,
	})
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Locals("Title", "Hello, World!")

	err := c.Render("./.github/testdata/index.tmpl", Map{})

	buf := bytebufferpool.Get()
	_, _ = buf.WriteString("overwrite")
	defer bytebufferpool.Put(buf)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1><no value></h1>", string(c.Response().Body()))
}

func Test_Ctx_RenderWithLocals(t *testing.T) {
	t.Parallel()
	app := New(Config{
		PassLocalsToViews: true,
	})
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Locals("Title", "Hello, World!")

	err := c.Render("./.github/testdata/index.tmpl", Map{})

	buf := bytebufferpool.Get()
	_, _ = buf.WriteString("overwrite")
	defer bytebufferpool.Put(buf)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))

}

func Test_Ctx_RenderWithBindVars(t *testing.T) {
	t.Parallel()

	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.BindVars(Map{
		"Title": "Hello, World!",
	})

	err := c.Render("./.github/testdata/index.tmpl", Map{})

	buf := bytebufferpool.Get()
	_, _ = buf.WriteString("overwrite")
	defer bytebufferpool.Put(buf)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))

}

func Test_Ctx_RenderWithBindVarsLocals(t *testing.T) {
	t.Parallel()

	app := New(Config{
		PassLocalsToViews: true,
	})

	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.BindVars(Map{
		"Title": "Hello, World!",
	})

	c.Locals("Summary", "Test")

	err := c.Render("./.github/testdata/template.tmpl", Map{})

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello, World! Test</h1>", string(c.Response().Body()))

}

func Test_Ctx_RenderWithLocalsAndBinding(t *testing.T) {
	t.Parallel()
	engine := &testTemplateEngine{}
	err := engine.Load()
	app := New(Config{
		PassLocalsToViews: true,
		Views:             engine,
	})
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Locals("Title", "This is a test.")

	err = c.Render("index.tmpl", Map{
		"Title": "Hello, World!",
	})

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

func Benchmark_Ctx_RenderWithLocalsAndBindVars(b *testing.B) {
	engine := &testTemplateEngine{}
	err := engine.Load()
	utils.AssertEqual(b, nil, err)
	app := New(Config{
		PassLocalsToViews: true,
		Views:             engine,
	})
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.BindVars(Map{
		"Title": "Hello, World!",
	})
	c.Locals("Summary", "Test")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		err = c.Render("template.tmpl", Map{})
	}

	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, "<h1>Hello, World! Test</h1>", string(c.Response().Body()))
}

func Benchmark_Ctx_RedirectToRoute(b *testing.B) {
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")

	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		c.RedirectToRoute("user", Map{
			"name": "fiber",
		})
	}

	utils.AssertEqual(b, 302, c.Response().StatusCode())
	utils.AssertEqual(b, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

func Benchmark_Ctx_RedirectToRouteWithQueries(b *testing.B) {
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")

	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		c.RedirectToRoute("user", Map{
			"name":    "fiber",
			"queries": map[string]string{"a": "a", "b": "b"},
		})
	}

	utils.AssertEqual(b, 302, c.Response().StatusCode())
	// analysis of query parameters with url parsing, since a map pass is always randomly ordered
	location, err := url.Parse(string(c.Response().Header.Peek(HeaderLocation)))
	utils.AssertEqual(b, nil, err, "url.Parse(location)")
	utils.AssertEqual(b, "/user/fiber", location.Path)
	utils.AssertEqual(b, url.Values{"a": []string{"a"}, "b": []string{"b"}}, location.Query())
}

func Benchmark_Ctx_RenderLocals(b *testing.B) {
	engine := &testTemplateEngine{}
	err := engine.Load()
	utils.AssertEqual(b, nil, err)
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

	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

func Benchmark_Ctx_RenderBindVars(b *testing.B) {
	engine := &testTemplateEngine{}
	err := engine.Load()
	utils.AssertEqual(b, nil, err)
	app := New()
	app.config.Views = engine
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.BindVars(Map{
		"Title": "Hello, World!",
	})

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		err = c.Render("index.tmpl", Map{})
	}

	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

// go test -run Test_Ctx_RestartRouting
func Test_Ctx_RestartRouting(t *testing.T) {
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
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, 3, calls, "Number of calls")
}

// go test -run Test_Ctx_RestartRoutingWithChangedPath
func Test_Ctx_RestartRoutingWithChangedPath(t *testing.T) {
	app := New()
	executedOldHandler := false
	executedNewHandler := false

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
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, executedOldHandler, "Executed old handler")
	utils.AssertEqual(t, true, executedNewHandler, "Executed new handler")
}

// go test -run Test_Ctx_RestartRoutingWithChangedPathAnd404
func Test_Ctx_RestartRoutingWithChangedPathAndCatchAll(t *testing.T) {
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
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

type testTemplateEngine struct {
	templates *template.Template
}

func (t *testTemplateEngine) Render(w io.Writer, name string, bind any, layout ...string) error {
	if len(layout) == 0 {
		return t.templates.ExecuteTemplate(w, name, bind)
	}
	_ = t.templates.ExecuteTemplate(w, name, bind)
	return t.templates.ExecuteTemplate(w, layout[0], bind)
}

func (t *testTemplateEngine) Load() error {
	t.templates = template.Must(template.ParseGlob("./.github/testdata/*.tmpl"))
	return nil
}

// go test -run Test_Ctx_Render_Engine
func Test_Ctx_Render_Engine(t *testing.T) {
	engine := &testTemplateEngine{}
	engine.Load()
	app := New()
	app.config.Views = engine
	c := app.NewCtx(&fasthttp.RequestCtx{})

	err := c.Render("index.tmpl", Map{
		"Title": "Hello, World!",
	})
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

// go test -run Test_Ctx_Render_Engine_With_View_Layout
func Test_Ctx_Render_Engine_With_View_Layout(t *testing.T) {
	engine := &testTemplateEngine{}
	engine.Load()
	app := New(Config{ViewsLayout: "main.tmpl"})
	app.config.Views = engine
	c := app.NewCtx(&fasthttp.RequestCtx{})

	err := c.Render("index.tmpl", Map{
		"Title": "Hello, World!",
	})
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello, World!</h1><h1>I'm main</h1>", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Render_Engine -benchmem -count=4
func Benchmark_Ctx_Render_Engine(b *testing.B) {
	engine := &testTemplateEngine{}
	err := engine.Load()
	utils.AssertEqual(b, nil, err)
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
	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Get_Location_From_Route -benchmem -count=4
func Benchmark_Ctx_Get_Location_From_Route(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	app.Get("/user/:name", func(c Ctx) error {
		return c.SendString(c.Params("name"))
	}).Name("User")
	for n := 0; n < b.N; n++ {
		c.getLocationFromRoute(app.GetRoute("User"), Map{"name": "fiber"})
	}
}

// go test -run Test_Ctx_Get_Location_From_Route_name
func Test_Ctx_Get_Location_From_Route_name(t *testing.T) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	app.Get("/user/:name", func(c Ctx) error {
		return c.SendString(c.Params("name"))
	}).Name("User")

	location, err := c.GetRouteURL("User", Map{"name": "fiber"})
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "/user/fiber", location)
}

// go test -run Test_Ctx_Get_Location_From_Route_name_Optional_greedy
func Test_Ctx_Get_Location_From_Route_name_Optional_greedy(t *testing.T) {
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
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "/23456789/sms/send/test-msg", location)
}

// go test -run Test_Ctx_Get_Location_From_Route_name_Optional_greedy_one_param
func Test_Ctx_Get_Location_From_Route_name_Optional_greedy_one_param(t *testing.T) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	app.Get("/:phone/*/send", func(c Ctx) error {
		return c.SendString("Phone: " + c.Params("phone") + "\nFirst Param: " + c.Params("*1"))
	}).Name("SendSms")

	location, err := c.GetRouteURL("SendSms", Map{
		"phone": "23456789",
		"*":     "sms",
	})
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "/23456789/sms/send", location)
}

type errorTemplateEngine struct{}

func (t errorTemplateEngine) Render(w io.Writer, name string, bind any, layout ...string) error {
	return errors.New("errorTemplateEngine")
}

func (t errorTemplateEngine) Load() error { return nil }

// go test -run Test_Ctx_Render_Engine_Error
func Test_Ctx_Render_Engine_Error(t *testing.T) {
	app := New()
	app.config.Views = errorTemplateEngine{}
	c := app.NewCtx(&fasthttp.RequestCtx{})

	err := c.Render("index.tmpl", nil)
	utils.AssertEqual(t, false, err == nil)
}

// go test -run Test_Ctx_Render_Go_Template
func Test_Ctx_Render_Go_Template(t *testing.T) {
	t.Parallel()

	file, err := os.CreateTemp(os.TempDir(), "fiber")
	utils.AssertEqual(t, nil, err)
	defer os.Remove(file.Name())

	_, err = file.Write([]byte("template"))
	utils.AssertEqual(t, nil, err)

	err = file.Close()
	utils.AssertEqual(t, nil, err)

	app := New()

	c := app.NewCtx(&fasthttp.RequestCtx{})

	err = c.Render(file.Name(), nil)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "template", string(c.Response().Body()))
}

// go test -run Test_Ctx_Send
func Test_Ctx_Send(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Send([]byte("Hello, World"))
	c.Send([]byte("Don't crash please"))
	c.Send([]byte("1337"))
	utils.AssertEqual(t, "1337", string(c.Response().Body()))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Send -benchmem -count=4
func Benchmark_Ctx_Send(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	byt := []byte("Hello, World!")
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Send(byt)
	}
	utils.AssertEqual(b, "Hello, World!", string(c.Response().Body()))
}

// go test -run Test_Ctx_SendStatus
func Test_Ctx_SendStatus(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.SendStatus(415)
	utils.AssertEqual(t, 415, c.Response().StatusCode())
	utils.AssertEqual(t, "Unsupported Media Type", string(c.Response().Body()))
}

// go test -run Test_Ctx_SendString
func Test_Ctx_SendString(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.SendString("Don't crash please")
	utils.AssertEqual(t, "Don't crash please", string(c.Response().Body()))
}

// go test -run Test_Ctx_SendStream
func Test_Ctx_SendStream(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.SendStream(bytes.NewReader([]byte("Don't crash please")))
	utils.AssertEqual(t, "Don't crash please", string(c.Response().Body()))

	c.SendStream(bytes.NewReader([]byte("Don't crash please")), len([]byte("Don't crash please")))
	utils.AssertEqual(t, "Don't crash please", string(c.Response().Body()))

	c.SendStream(bufio.NewReader(bytes.NewReader([]byte("Hello bufio"))))
	utils.AssertEqual(t, "Hello bufio", string(c.Response().Body()))

	file, err := os.Open("./.github/index.html")
	utils.AssertEqual(t, nil, err)
	c.SendStream(bufio.NewReader(file))
	utils.AssertEqual(t, true, (c.Response().Header.ContentLength() > 200))
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
	utils.AssertEqual(t, "1", string(c.Response().Header.Peek("x-1")))
	utils.AssertEqual(t, "2", string(c.Response().Header.Peek("x-2")))
	utils.AssertEqual(t, "1337", string(c.Response().Header.Peek("x-3")))
}

// go test -run Test_Ctx_Set_Splitter
func Test_Ctx_Set_Splitter(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Set("Location", "foo\r\nSet-Cookie:%20SESSIONID=MaliciousValue\r\n")
	h := string(c.Response().Header.Peek("Location"))
	utils.AssertEqual(t, false, strings.Contains(h, "\r\n"), h)

	c.Set("Location", "foo\nSet-Cookie:%20SESSIONID=MaliciousValue\n")
	h = string(c.Response().Header.Peek("Location"))
	utils.AssertEqual(t, false, strings.Contains(h, "\n"), h)
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
	utils.AssertEqual(t, 400, c.Response().StatusCode())
	c.Status(415).Send([]byte("Hello, World"))
	utils.AssertEqual(t, 415, c.Response().StatusCode())
	utils.AssertEqual(t, "Hello, World", string(c.Response().Body()))
}

// go test -run Test_Ctx_Type
func Test_Ctx_Type(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Type(".json")
	utils.AssertEqual(t, "application/json", string(c.Response().Header.Peek("Content-Type")))

	c.Type("json", "utf-8")
	utils.AssertEqual(t, "application/json; charset=utf-8", string(c.Response().Header.Peek("Content-Type")))

	c.Type(".html")
	utils.AssertEqual(t, "text/html", string(c.Response().Header.Peek("Content-Type")))

	c.Type("html", "utf-8")
	utils.AssertEqual(t, "text/html; charset=utf-8", string(c.Response().Header.Peek("Content-Type")))
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
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

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
	utils.AssertEqual(t, "Origin, User-Agent, Accept-Encoding, Accept", string(c.Response().Header.Peek("Vary")))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Vary -benchmem -count=4
func Benchmark_Ctx_Vary(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

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

	c.Write([]byte("Hello, "))
	c.Write([]byte("World!"))
	utils.AssertEqual(t, "Hello, World!", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Write -benchmem -count=4
func Benchmark_Ctx_Write(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	byt := []byte("Hello, World!")
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Write(byt)
	}
}

// go test -run Test_Ctx_Writef
func Test_Ctx_Writef(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	world := "World!"
	c.Writef("Hello, %s", world)
	utils.AssertEqual(t, "Hello, World!", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Writef -benchmem -count=4
func Benchmark_Ctx_Writef(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	world := "World!"
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Writef("Hello, %s", world)
	}
}

// go test -run Test_Ctx_WriteString
func Test_Ctx_WriteString(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.WriteString("Hello, ")
	c.WriteString("World!")
	utils.AssertEqual(t, "Hello, World!", string(c.Response().Body()))
}

// go test -run Test_Ctx_XHR
func Test_Ctx_XHR(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderXRequestedWith, "XMLHttpRequest")
	utils.AssertEqual(t, true, c.XHR())
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
	utils.AssertEqual(b, true, equal)
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_SendString_B -benchmem -count=4
func Benchmark_Ctx_SendString_B(b *testing.B) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	body := "Hello, world!"
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.SendString(body)
	}
	utils.AssertEqual(b, []byte("Hello, world!"), c.Response().Body())
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
	if !ctx.IsBodyStream() {
		t.Fatal("IsBodyStream must return true")
	}

	s := ctx.Response.String()
	br := bufio.NewReader(bytes.NewBufferString(s))
	var resp fasthttp.Response
	if err := resp.Read(br); err != nil {
		t.Fatalf("Error when reading response: %s", err)
	}
	body := string(resp.Body())
	expectedBody := "body writer line 1\nbody writer line 2\n"
	if body != expectedBody {
		t.Fatalf("unexpected body: %q. Expecting %q", body, expectedBody)
	}
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_BodyStreamWriter -benchmem -count=4
func Benchmark_Ctx_BodyStreamWriter(b *testing.B) {
	ctx := &fasthttp.RequestCtx{}
	user := []byte(`{"name":"john"}`)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		ctx.ResetBody()
		ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
			for i := 0; i < 10; i++ {
				w.Write(user)
				if err := w.Flush(); err != nil {
					return
				}
			}
		})
	}
}

func Test_Ctx_String(t *testing.T) {
	t.Parallel()

	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	utils.AssertEqual(t, "#0000000000000000 - 0.0.0.0:0 <-> 0.0.0.0:0 - GET http:///", c.String())
}

func TestCtx_ParamsInt(t *testing.T) {
	// Create a test context and set some strings (or params)

	// create a fake app to be used within this test
	app := New()

	// Create some test endpoints

	// For the user id I will use the number 1111, so I should be able to get the number
	// 1111 from the Ctx
	app.Get("/test/:user", func(c Ctx) error {
		// utils.AssertEqual(t, "john", c.Params("user"))

		num, err := c.ParamsInt("user")

		// Check the number matches
		if num != 1111 {
			t.Fatalf("Expected number 1111 from the path, got %d", num)
		}

		// Check no errors are returned, because we want NO errors in this one
		if err != nil {
			t.Fatalf("Expected nil error for 1111 test, got " + err.Error())
		}

		return nil
	})

	// In this test case, there will be a bad request where the expected number is NOT
	// a number in the path
	app.Get("/testnoint/:user", func(c Ctx) error {
		// utils.AssertEqual(t, "john", c.Params("user"))

		num, err := c.ParamsInt("user")

		// Check the number matches
		if num != 0 {
			t.Fatalf("Expected number 0 from the path, got %d", num)
		}

		// Check an error is returned, because we want NO errors in this one
		if err == nil {
			t.Fatal("Expected non nil error for bad req test, got nil")
		}

		return nil
	})

	// For the user id I will use the number 2222, so I should be able to get the number
	// 2222 from the Ctx even when the default value is specified
	app.Get("/testignoredefault/:user", func(c Ctx) error {
		// utils.AssertEqual(t, "john", c.Params("user"))

		num, err := c.ParamsInt("user", 1111)

		// Check the number matches
		if num != 2222 {
			t.Fatalf("Expected number 2222 from the path, got %d", num)
		}

		// Check no errors are returned, because we want NO errors in this one
		if err != nil {
			t.Fatalf("Expected nil error for 2222 test, got " + err.Error())
		}

		return nil
	})

	// In this test case, there will be a bad request where the expected number is NOT
	// a number in the path, default value of 1111 should be used instead
	app.Get("/testdefault/:user", func(c Ctx) error {
		// utils.AssertEqual(t, "john", c.Params("user"))

		num, err := c.ParamsInt("user", 1111)

		// Check the number matches
		if num != 1111 {
			t.Fatalf("Expected number 1111 from the path, got %d", num)
		}

		// Check an error is returned, because we want NO errors in this one
		if err != nil {
			t.Fatalf("Expected nil error for 1111 test, got " + err.Error())
		}

		return nil
	})

	app.Test(httptest.NewRequest(MethodGet, "/test/1111", nil))
	app.Test(httptest.NewRequest(MethodGet, "/testnoint/xd", nil))
	app.Test(httptest.NewRequest(MethodGet, "/testignoredefault/2222", nil))
	app.Test(httptest.NewRequest(MethodGet, "/testdefault/xd", nil))
}

// go test -run Test_Ctx_GetRespHeader
func Test_Ctx_GetRespHeader(t *testing.T) {
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Set("test", "Hello, World 👋!")
	c.Response().Header.Set(HeaderContentType, "application/json")
	utils.AssertEqual(t, c.GetRespHeader("test"), "Hello, World 👋!")
	utils.AssertEqual(t, c.GetRespHeader(HeaderContentType), "application/json")
}

// go test -run Test_Ctx_IsFromLocal
func Test_Ctx_IsFromLocal(t *testing.T) {
	t.Parallel()
	// Test "0.0.0.0", "127.0.0.1" and "::1".
	{
		app := New()
		c := app.NewCtx(&fasthttp.RequestCtx{})

		utils.AssertEqual(t, true, c.IsFromLocal())
	}
	// This is a test for "0.0.0.0"
	{
		app := New()
		c := app.NewCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "0.0.0.0")

		utils.AssertEqual(t, true, c.IsFromLocal())
	}

	// This is a test for "127.0.0.1"
	{
		app := New()
		c := app.NewCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1")

		utils.AssertEqual(t, true, c.IsFromLocal())
	}

	// This is a test for "localhost"
	{
		app := New()
		c := app.NewCtx(&fasthttp.RequestCtx{})

		utils.AssertEqual(t, true, c.IsFromLocal())
	}

	// This is testing "::1", it is the compressed format IPV6 loopback address 0:0:0:0:0:0:0:1.
	// It is the equivalent of the IPV4 address 127.0.0.1.
	{
		app := New()
		c := app.NewCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "::1")

		utils.AssertEqual(t, true, c.IsFromLocal())
	}

	{
		app := New()
		c := app.NewCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "93.46.8.90")

		utils.AssertEqual(t, false, c.IsFromLocal())
	}
}
