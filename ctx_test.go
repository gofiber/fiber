// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

// go test -v -run=^$ -bench=Benchmark_Ctx_Accepts -benchmem -count=4
// go test -run Test_Ctx

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	utils "github.com/gofiber/utils"
	fasthttp "github.com/valyala/fasthttp"
)

// go test -run Test_Ctx_Accepts
func Test_Ctx_Accepts(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderAccept, "text/html,application/xhtml+xml,application/xml;q=0.9")
	utils.AssertEqual(t, "", ctx.Accepts(""))
	utils.AssertEqual(t, "", ctx.Accepts())
	utils.AssertEqual(t, ".xml", ctx.Accepts(".xml"))
	utils.AssertEqual(t, "", ctx.Accepts(".john"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Accepts -benchmem -count=4
func Benchmark_Ctx_Accepts(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.Accepts(".xml")
	}
	utils.AssertEqual(b, ".xml", res)
}

// go test -run Test_Ctx_Accepts_EmptyAccept
func Test_Ctx_Accepts_EmptyAccept(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	utils.AssertEqual(t, ".forwarded", ctx.Accepts(".forwarded"))
}

// go test -run Test_Ctx_Accepts_Wildcard
func Test_Ctx_Accepts_Wildcard(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderAccept, "*/*;q=0.9")
	utils.AssertEqual(t, "html", ctx.Accepts("html"))
	utils.AssertEqual(t, "foo", ctx.Accepts("foo"))
	utils.AssertEqual(t, ".bar", ctx.Accepts(".bar"))
	ctx.Fasthttp.Request.Header.Set(HeaderAccept, "text/html,application/*;q=0.9")
	utils.AssertEqual(t, "xml", ctx.Accepts("xml"))
}

// go test -run Test_Ctx_AcceptsCharsets
func Test_Ctx_AcceptsCharsets(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderAcceptCharset, "utf-8, iso-8859-1;q=0.5")
	utils.AssertEqual(t, "utf-8", ctx.AcceptsCharsets("utf-8"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsCharsets -benchmem -count=4
func Benchmark_Ctx_AcceptsCharsets(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set("Accept-Charset", "utf-8, iso-8859-1;q=0.5")
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
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderAcceptEncoding, "deflate, gzip;q=1.0, *;q=0.5")
	utils.AssertEqual(t, "gzip", ctx.AcceptsEncodings("gzip"))
	utils.AssertEqual(t, "abc", ctx.AcceptsEncodings("abc"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsEncodings -benchmem -count=4
func Benchmark_Ctx_AcceptsEncodings(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set(HeaderAcceptEncoding, "deflate, gzip;q=1.0, *;q=0.5")
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
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderAcceptLanguage, "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")
	utils.AssertEqual(t, "fr", ctx.AcceptsLanguages("fr"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsLanguages -benchmem -count=4
func Benchmark_Ctx_AcceptsLanguages(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set(HeaderAcceptLanguage, "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.AcceptsLanguages("fr")
	}
	utils.AssertEqual(b, "fr", res)
}

// go test -run Test_Ctx_Append
func Test_Ctx_Append(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Append("X-Test", "Hello")
	ctx.Append("X-Test", "World")
	ctx.Append("X-Test", "Hello", "World")
	ctx.Append("X-Custom-Header")
	utils.AssertEqual(t, "Hello, World", string(ctx.Fasthttp.Response.Header.Peek("X-Test")))
	utils.AssertEqual(t, "", string(ctx.Fasthttp.Response.Header.Peek("x-custom-header")))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Append -benchmem -count=4
func Benchmark_Ctx_Append(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Append("X-Custom-Header", "Hello")
		c.Append("X-Custom-Header", "World")
		c.Append("X-Custom-Header", "Hello")
	}
	utils.AssertEqual(b, "Hello, World", getString(c.Fasthttp.Response.Header.Peek("X-Custom-Header")))
}

// go test -run Test_Ctx_Attachment
func Test_Ctx_Attachment(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Attachment()
	ctx.Attachment("./static/img/logo.png")
	utils.AssertEqual(t, `attachment; filename="logo.png"`, string(ctx.Fasthttp.Response.Header.Peek(HeaderContentDisposition)))
	utils.AssertEqual(t, "image/png", string(ctx.Fasthttp.Response.Header.Peek(HeaderContentType)))
}

// go test -run Test_Ctx_BaseURL
func Test_Ctx_BaseURL(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.SetRequestURI("http://google.com/test")
	utils.AssertEqual(t, "http://google.com", ctx.BaseURL())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Append -benchmem -count=4
func Benchmark_Ctx_BaseURL(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Fasthttp.Request.SetHost("google.com:1337")
	c.Fasthttp.Request.URI().SetPath("/haha/oke/lol")
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
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.SetBody([]byte("john=doe"))
	utils.AssertEqual(t, "john=doe", ctx.Body())
}

// go test -run Test_Ctx_BodyParser
func Test_Ctx_BodyParser(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	type Demo struct {
		Name string `json:"name" xml:"name" form:"name" query:"name"`
	}
	ctx.Fasthttp.Request.SetBody([]byte(`{"name":"john"}`))
	ctx.Fasthttp.Request.Header.SetContentType(MIMEApplicationJSON)
	ctx.Fasthttp.Request.Header.SetContentLength(len([]byte(`{"name":"john"}`)))
	d := new(Demo)
	utils.AssertEqual(t, nil, ctx.BodyParser(d))
	utils.AssertEqual(t, "john", d.Name)

	type Query struct {
		ID    int
		Name  string
		Hobby []string
	}
	ctx.Fasthttp.Request.SetBody([]byte(``))
	ctx.Fasthttp.Request.Header.SetContentType("")
	ctx.Fasthttp.Request.URI().SetQueryString("id=1&name=tom&hobby=basketball&hobby=football")
	q := new(Query)
	utils.AssertEqual(t, nil, ctx.BodyParser(q))
	utils.AssertEqual(t, 2, len(q.Hobby))
}

// TODO Benchmark_Ctx_BodyParser

// go test -v -run=^$ -bench=Benchmark_Ctx_Cookie -benchmem -count=4
func Benchmark_Ctx_Cookie(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Cookie(&Cookie{
			Name:  "John",
			Value: "Doe",
		})
	}
	utils.AssertEqual(b, "John=Doe; path=/; SameSite=Lax", getString(c.Fasthttp.Response.Header.Peek("Set-Cookie")))
}

// go test -run Test_Ctx_Cookie
func Test_Ctx_Cookie(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	expire := time.Now().Add(24 * time.Hour)
	var dst []byte
	dst = expire.In(time.UTC).AppendFormat(dst, time.RFC1123)
	httpdate := strings.Replace(string(dst), "UTC", "GMT", -1)
	ctx.Cookie(&Cookie{
		Name:    "username",
		Value:   "john",
		Expires: expire,
	})
	expect := "username=john; expires=" + httpdate + "; path=/; SameSite=Lax"
	utils.AssertEqual(t, expect, string(ctx.Fasthttp.Response.Header.Peek(HeaderSetCookie)))
}

// go test -run Test_Ctx_Cookies
func Test_Ctx_Cookies(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set("Cookie", "john=doe")
	utils.AssertEqual(t, "doe", ctx.Cookies("john"))
}

// go test -run Test_Ctx_Format
func Test_Ctx_Format(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderAccept, "text/html")
	ctx.Format("Hello, World!")
	utils.AssertEqual(t, "<p>Hello, World!</p>", string(ctx.Fasthttp.Response.Body()))

	ctx.Fasthttp.Request.Header.Set(HeaderAccept, "application/json")
	ctx.Format("Hello, World!")
	utils.AssertEqual(t, `"Hello, World!"`, string(ctx.Fasthttp.Response.Body()))

	ctx.Fasthttp.Request.Header.Set(HeaderAccept, "application/xml")
	ctx.Format("Hello, World!")
	utils.AssertEqual(t, `<string>Hello, World!</string>`, string(ctx.Fasthttp.Response.Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Format -benchmem -count=4
func Benchmark_Ctx_Format(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set("Accept", "text/plain")
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	utils.AssertEqual(b, `Hello, World!`, string(c.Fasthttp.Response.Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Format_HTML -benchmem -count=4
func Benchmark_Ctx_Format_HTML(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set("Accept", "text/html")
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	utils.AssertEqual(b, "<p>Hello, World!</p>", string(c.Fasthttp.Response.Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Format_JSON -benchmem -count=4
func Benchmark_Ctx_Format_JSON(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set("Accept", "application/json")
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	utils.AssertEqual(b, `"Hello, World!"`, string(c.Fasthttp.Response.Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Format_XML -benchmem -count=4
func Benchmark_Ctx_Format_XML(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set("Accept", "application/xml")
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	utils.AssertEqual(b, `<string>Hello, World!</string>`, string(c.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_FormFile
func Test_Ctx_FormFile(t *testing.T) {
	// TODO: CLEAN THIS UP
	t.Parallel()
	app := New()

	app.Post("/test", func(c *Ctx) {
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

// go test -run Test_Ctx_FormValue
func Test_Ctx_FormValue(t *testing.T) {
	t.Parallel()
	app := New()

	app.Post("/test", func(c *Ctx) {
		utils.AssertEqual(t, "john", c.FormValue("name"))
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

// go test -run Test_Ctx_Fresh
func Test_Ctx_Fresh(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	utils.AssertEqual(t, false, ctx.Fresh())
}

// go test -run Test_Ctx_Get
func Test_Ctx_Get(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderAcceptCharset, "utf-8, iso-8859-1;q=0.5")
	ctx.Fasthttp.Request.Header.Set(HeaderReferer, "Monster")
	utils.AssertEqual(t, "utf-8, iso-8859-1;q=0.5", ctx.Get(HeaderAcceptCharset))
	utils.AssertEqual(t, "Monster", ctx.Get(HeaderReferer))
}

// go test -run Test_Ctx_Hostname
func Test_Ctx_Hostname(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.SetRequestURI("http://google.com/test")
	utils.AssertEqual(t, "google.com", ctx.Hostname())
}

// go test -run Test_Ctx_IP
func Test_Ctx_IP(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	utils.AssertEqual(t, "0.0.0.0", ctx.IP())
}

// go test -run Test_Ctx_IPs
func Test_Ctx_IPs(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderXForwardedFor, "127.0.0.1, 127.0.0.1, 127.0.0.1")
	utils.AssertEqual(t, []string{"127.0.0.1", "127.0.0.1", "127.0.0.1"}, ctx.IPs())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_IPs -benchmem -count=4
func Benchmark_Ctx_IPs(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set(HeaderXForwardedFor, "127.0.0.1, 127.0.0.1, 127.0.0.1")
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
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderContentType, MIMETextHTML+"; boundary=something")
	utils.AssertEqual(t, true, ctx.Is(".html"))
	utils.AssertEqual(t, true, ctx.Is("html"))
	utils.AssertEqual(t, false, ctx.Is("json"))
	utils.AssertEqual(t, false, ctx.Is(".json"))
	utils.AssertEqual(t, false, ctx.Is(""))
	utils.AssertEqual(t, false, ctx.Is(".foooo"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Is -benchmem -count=4
func Benchmark_Ctx_Is(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set(HeaderContentType, MIMEApplicationJSON)
	var res bool
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.Is(".json")
		res = c.Is("json")
	}
	utils.AssertEqual(b, true, res)
}

// go test -run Test_Ctx_Locals
func Test_Ctx_Locals(t *testing.T) {
	app := New()
	app.Use(func(c *Ctx) {
		c.Locals("john", "doe")
		c.Next()
	})
	app.Get("/test", func(c *Ctx) {
		utils.AssertEqual(t, "doe", c.Locals("john"))
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
	ctx := app.AcquireCtx(fctx)
	defer app.ReleaseCtx(ctx)
	utils.AssertEqual(t, MethodGet, ctx.Method())
}

// go test -run Test_Ctx_MultipartForm
func Test_Ctx_MultipartForm(t *testing.T) {
	t.Parallel()
	app := New()

	app.Post("/test", func(c *Ctx) {
		result, err := c.MultipartForm()
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "john", result.Value["name"][0])
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

// go test -run Test_Ctx_OriginalURL
func Test_Ctx_OriginalURL(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.SetRequestURI("http://google.com/test?search=demo")
	utils.AssertEqual(t, "http://google.com/test?search=demo", ctx.OriginalURL())
}

// go test -race -run Test_Ctx_Params
func Test_Ctx_Params(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test/:user", func(c *Ctx) {
		utils.AssertEqual(t, "john", c.Params("user"))
	})
	app.Get("/test2/*", func(c *Ctx) {
		utils.AssertEqual(t, "im/a/cookie", c.Params("*"))
	})
	app.Get("/test3/:optional?", func(c *Ctx) {
		utils.AssertEqual(t, "", c.Params("optional"))
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test/john", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test2/im/a/cookie", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test3", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Params -benchmem -count=4
func Benchmark_Ctx_Params(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.route = &Route{
		routeParams: []string{
			"param1", "param2", "param3", "param4",
		},
	}
	c.values = []string{
		"john", "doe", "is", "awesome",
	}
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.Params("param1")
		res = c.Params("param2")
		res = c.Params("param3")
		res = c.Params("param4")
	}
	utils.AssertEqual(b, "awesome", res)
}

// go test -run Test_Ctx_Path
func Test_Ctx_Path(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test/:user", func(c *Ctx) {
		utils.AssertEqual(t, "/test/john", c.Path())
		// not strict && case insensitive
		utils.AssertEqual(t, "/ABC/", c.Path("/ABC/"))
		utils.AssertEqual(t, "/test/john/", c.Path("/test/john/"))
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test/john", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Protocol
func Test_Ctx_Protocol(t *testing.T) {
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderXForwardedProto, "https")
	utils.AssertEqual(t, "https", ctx.Protocol())
	ctx.Fasthttp.Request.Header.Reset()

	ctx.Fasthttp.Request.Header.Set(HeaderXForwardedProtocol, "https")
	utils.AssertEqual(t, "https", ctx.Protocol())
	ctx.Fasthttp.Request.Header.Reset()

	ctx.Fasthttp.Request.Header.Set(HeaderXForwardedSsl, "on")
	utils.AssertEqual(t, "https", ctx.Protocol())
	ctx.Fasthttp.Request.Header.Reset()

	ctx.Fasthttp.Request.Header.Set(HeaderXUrlScheme, "https")
	utils.AssertEqual(t, "https", ctx.Protocol())
	ctx.Fasthttp.Request.Header.Reset()

	utils.AssertEqual(t, "http", ctx.Protocol())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Protocol -benchmem -count=4
func Benchmark_Ctx_Protocol(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.Protocol()
	}
	utils.AssertEqual(b, "http", res)
}

// go test -run Test_Ctx_Query
func Test_Ctx_Query(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.URI().SetQueryString("search=john&age=20")
	utils.AssertEqual(t, "john", ctx.Query("search"))
	utils.AssertEqual(t, "20", ctx.Query("age"))
}

// go test -run Test_Ctx_Range
func Test_Ctx_Range(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderRange, "bytes=500-700")
	result, err := ctx.Range(1000)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "bytes", result.Type)
	utils.AssertEqual(t, 500, result.Ranges[0].Start)
	utils.AssertEqual(t, 700, result.Ranges[0].End)
}

// go test -run Test_Ctx_Route
func Test_Ctx_Route(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		utils.AssertEqual(t, "/test", c.Route().Path)
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_SaveFile
func Test_Ctx_SaveFile(t *testing.T) {
	// TODO CLEAN THIS UP
	t.Parallel()
	app := New()

	app.Post("/test", func(c *Ctx) {
		fh, err := c.FormFile("file")
		utils.AssertEqual(t, nil, err)

		tempFile, err := ioutil.TempFile(os.TempDir(), "test-")
		utils.AssertEqual(t, nil, err)

		defer os.Remove(tempFile.Name())
		err = c.SaveFile(fh, tempFile.Name())
		utils.AssertEqual(t, nil, err)

		bs, err := ioutil.ReadFile(tempFile.Name())
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "hello world", string(bs))
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
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	// TODO Add TLS conn
	utils.AssertEqual(t, false, ctx.Secure())
}

// go test -run Test_Ctx_Stale
func Test_Ctx_Stale(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	utils.AssertEqual(t, true, ctx.Stale())
}

// go test -run Test_Ctx_Subdomains
func Test_Ctx_Subdomains(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.URI().SetHost("john.doe.google.com")
	utils.AssertEqual(t, []string{"john", "doe"}, ctx.Subdomains())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Subdomains -benchmem -count=4
func Benchmark_Ctx_Subdomains(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Fasthttp.Request.SetRequestURI("http://john.doe.google.com")
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
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderCookie, "john=doe")
	ctx.ClearCookie("john")
	utils.AssertEqual(t, true, strings.HasPrefix(string(ctx.Fasthttp.Response.Header.Peek(HeaderSetCookie)), "john=; expires="))
}

// go test -race -run Test_Ctx_Download
func Test_Ctx_Download(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	ctx.Download("ctx.go")

	f, err := os.Open("./ctx.go")
	utils.AssertEqual(t, nil, err)
	defer f.Close()

	expect, err := ioutil.ReadAll(f)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, bytes.Equal(expect, ctx.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_JSON
func Test_Ctx_JSON(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.JSON(Map{ // map has no order
		"Name": "Grame",
		"Age":  20,
	})
	utils.AssertEqual(t, `{"Age":20,"Name":"Grame"}`, string(ctx.Fasthttp.Response.Body()))
	utils.AssertEqual(t, "application/json", string(ctx.Fasthttp.Response.Header.Peek("content-type")))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_JSON -benchmem -count=4
func Benchmark_Ctx_JSON(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	utils.AssertEqual(b, `{"Name":"Grame","Age":20}`, string(c.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_JSONP
func Test_Ctx_JSONP(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.JSONP(Map{ // map has no order
		"Name": "Grame",
		"Age":  20,
	}, "john")
	utils.AssertEqual(t, `john({"Age":20,"Name":"Grame"});`, string(ctx.Fasthttp.Response.Body()))
	utils.AssertEqual(t, "application/javascript", string(ctx.Fasthttp.Response.Header.Peek("content-type")))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_JSONP -benchmem -count=4
func Benchmark_Ctx_JSONP(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	type SomeStruct struct {
		Name string
		Age  uint8
	}
	data := SomeStruct{
		Name: "Grame",
		Age:  20,
	}
	var callback = "emit"
	var err error
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err = c.JSONP(data, callback)
	}
	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, `emit({"Name":"Grame","Age":20});`, string(c.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_Links
func Test_Ctx_Links(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Links(
		"http://api.example.com/users?page=2", "next",
		"http://api.example.com/users?page=5", "last",
	)
	utils.AssertEqual(t, `<http://api.example.com/users?page=2>; rel="next",<http://api.example.com/users?page=5>; rel="last"`, string(ctx.Fasthttp.Response.Header.Peek(HeaderLink)))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Links -benchmem -count=4
func Benchmark_Ctx_Links(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Location("http://example.com")
	utils.AssertEqual(t, "http://example.com", string(ctx.Fasthttp.Response.Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_Next
func Test_Ctx_Next(t *testing.T) {
	app := New()
	app.Use("/", func(c *Ctx) {
		c.Next()
	})
	app.Get("/test", func(c *Ctx) {
		c.Set("X-Next-Result", "Works")
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "http://example.com/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "Works", resp.Header.Get("X-Next-Result"))
}

// go test -run Test_Ctx_Redirect
func Test_Ctx_Redirect(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Redirect("http://example.com", 301)
	utils.AssertEqual(t, 301, ctx.Fasthttp.Response.StatusCode())
	utils.AssertEqual(t, "http://example.com", string(ctx.Fasthttp.Response.Header.Peek(HeaderLocation)))
}

// ViewEngine is coming in v1.10
// func Test_Ctx_Render(t *testing.T) {
// 	// TODO
// }

// go test -run Test_Ctx_Send
func Test_Ctx_Send(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Send([]byte("Hello, World"))
	ctx.Send("Don't crash please")
	ctx.Send(1337)
	utils.AssertEqual(t, "1337", string(ctx.Fasthttp.Response.Body()))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Send -benchmem -count=4
func Benchmark_Ctx_Send(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	var str = "Hello, World!"
	var byt = []byte("Hello, World!")
	var nmb = 123
	var bol = true
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Send(str)
		c.Send(byt)
		c.Send(nmb)
		c.Send(bol)
	}
	utils.AssertEqual(b, "true", string(c.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_SendBytes
func Test_Ctx_SendBytes(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.SendBytes([]byte("Hello, World!"))
	utils.AssertEqual(t, "Hello, World!", string(ctx.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_SendStatus
func Test_Ctx_SendStatus(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.SendStatus(415)
	utils.AssertEqual(t, 415, ctx.Fasthttp.Response.StatusCode())
	utils.AssertEqual(t, "Unsupported Media Type", string(ctx.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_SendString
func Test_Ctx_SendString(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.SendString("Don't crash please")
	utils.AssertEqual(t, "Don't crash please", string(ctx.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_Set
func Test_Ctx_Set(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Set("X-1", "1")
	ctx.Set("X-2", "2")
	ctx.Set("X-3", "3")
	ctx.Set("X-3", "1337")
	utils.AssertEqual(t, "1", string(ctx.Fasthttp.Response.Header.Peek("x-1")))
	utils.AssertEqual(t, "2", string(ctx.Fasthttp.Response.Header.Peek("x-2")))
	utils.AssertEqual(t, "1337", string(ctx.Fasthttp.Response.Header.Peek("x-3")))
}

// go test -run Test_Ctx_Status
func Test_Ctx_Status(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Status(400)
	utils.AssertEqual(t, 400, ctx.Fasthttp.Response.StatusCode())
	ctx.Status(415).Send("Hello, World")
	utils.AssertEqual(t, 415, ctx.Fasthttp.Response.StatusCode())
	utils.AssertEqual(t, "Hello, World", string(ctx.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_Type
func Test_Ctx_Type(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Type(".json")
	utils.AssertEqual(t, "application/json", string(ctx.Fasthttp.Response.Header.Peek("Content-Type")))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Type -benchmem -count=4
func Benchmark_Ctx_Type(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Type(".json")
		c.Type("json")
	}
}

// go test -run Test_Ctx_Vary
func Test_Ctx_Vary(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Vary("Origin")
	ctx.Vary("User-Agent")
	ctx.Vary("Accept-Encoding", "Accept")
	utils.AssertEqual(t, "Origin, User-Agent, Accept-Encoding, Accept", string(ctx.Fasthttp.Response.Header.Peek("Vary")))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Vary -benchmem -count=4
func Benchmark_Ctx_Vary(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Write("Hello, ")
	ctx.Write([]byte("World! "))
	ctx.Write(123)
	utils.AssertEqual(t, "Hello, World! 123", string(ctx.Fasthttp.Response.Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Write -benchmem -count=4
func Benchmark_Ctx_Write(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	var str = "Hello, World!"
	var byt = []byte("Hello, World!")
	var nmb = 123
	var bol = true
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Write(str)
		c.Write(byt)
		c.Write(nmb)
		c.Write(bol)
	}
}

// go test -run Test_Ctx_XHR
func Test_Ctx_XHR(t *testing.T) {
	t.Parallel()
	app := New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderXRequestedWith, "XMLHttpRequest")
	utils.AssertEqual(t, true, ctx.XHR())
}
