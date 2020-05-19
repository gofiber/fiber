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

	fasthttp "github.com/valyala/fasthttp"
)

// go test -run Test_Ctx_Accepts
func Test_Ctx_Accepts(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderAccept, "text/html,application/xhtml+xml,application/xml;q=0.9")
	assertEqual(t, "", ctx.Accepts(""))
	assertEqual(t, "", ctx.Accepts())
	assertEqual(t, ".xml", ctx.Accepts(".xml"))
	assertEqual(t, "", ctx.Accepts(".john"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Accepts -benchmem -count=4
func Benchmark_Ctx_Accepts(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")
	var res string
	for n := 0; n < b.N; n++ {
		res = c.Accepts(".xml")
	}
	assertEqual(b, ".xml", res)
}

// go test -run Test_Ctx_Accepts_EmptyAccept
func Test_Ctx_Accepts_EmptyAccept(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	assertEqual(t, ".forwarded", ctx.Accepts(".forwarded"))
}

// go test -run Test_Ctx_Accepts_Wildcard
func Test_Ctx_Accepts_Wildcard(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderAccept, "*/*;q=0.9")
	assertEqual(t, "html", ctx.Accepts("html"))
	assertEqual(t, "foo", ctx.Accepts("foo"))
	assertEqual(t, ".bar", ctx.Accepts(".bar"))
	ctx.Fasthttp.Request.Header.Set(HeaderAccept, "text/html,application/*;q=0.9")
	assertEqual(t, "xml", ctx.Accepts("xml"))
}

// go test -run Test_Ctx_AcceptsCharsets
func Test_Ctx_AcceptsCharsets(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderAcceptCharset, "utf-8, iso-8859-1;q=0.5")
	assertEqual(t, "utf-8", ctx.AcceptsCharsets("utf-8"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsCharsets -benchmem -count=4
func Benchmark_Ctx_AcceptsCharsets(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set("Accept-Charset", "utf-8, iso-8859-1;q=0.5")
	var res string
	for n := 0; n < b.N; n++ {
		res = c.AcceptsCharsets("utf-8")
	}
	assertEqual(b, "utf-8", res)
}

// go test -run Test_Ctx_AcceptsEncodings
func Test_Ctx_AcceptsEncodings(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderAcceptEncoding, "deflate, gzip;q=1.0, *;q=0.5")
	assertEqual(t, "gzip", ctx.AcceptsEncodings("gzip"))
	assertEqual(t, "abc", ctx.AcceptsEncodings("abc"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsEncodings -benchmem -count=4
func Benchmark_Ctx_AcceptsEncodings(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set(HeaderAcceptEncoding, "deflate, gzip;q=1.0, *;q=0.5")
	var res string
	for n := 0; n < b.N; n++ {
		res = c.AcceptsEncodings("gzip")
	}
	assertEqual(b, "gzip", res)
}

// go test -run Test_Ctx_AcceptsLanguages
func Test_Ctx_AcceptsLanguages(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderAcceptLanguage, "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")
	assertEqual(t, "fr", ctx.AcceptsLanguages("fr"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsLanguages -benchmem -count=4
func Benchmark_Ctx_AcceptsLanguages(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set(HeaderAcceptLanguage, "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")
	var res string
	for n := 0; n < b.N; n++ {
		res = c.AcceptsLanguages("fr")
	}
	assertEqual(b, "fr", res)
}

// go test -run Test_Ctx_Append
func Test_Ctx_Append(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Append("X-Test", "Hello")
	ctx.Append("X-Test", "World")
	ctx.Append("X-Test", "Hello", "World")
	ctx.Append("X-Custom-Header")
	assertEqual(t, "Hello, World", string(ctx.Fasthttp.Response.Header.Peek("X-Test")))
	assertEqual(t, "", string(ctx.Fasthttp.Response.Header.Peek("x-custom-header")))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Append -benchmem -count=4
func Benchmark_Ctx_Append(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	for n := 0; n < b.N; n++ {
		c.Append("X-Custom-Header", "Hello")
		c.Append("X-Custom-Header", "World")
		c.Append("X-Custom-Header", "Hello")
	}
	assertEqual(b, "Hello, World", getString(c.Fasthttp.Response.Header.Peek("X-Custom-Header")))
}

// go test -run Test_Ctx_Attachment
func Test_Ctx_Attachment(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Attachment()
	ctx.Attachment("./static/img/logo.png")
	assertEqual(t, `attachment; filename="logo.png"`, string(ctx.Fasthttp.Response.Header.Peek(HeaderContentDisposition)))
	assertEqual(t, "image/png", string(ctx.Fasthttp.Response.Header.Peek(HeaderContentType)))
}

// go test -run Test_Ctx_BaseURL
func Test_Ctx_BaseURL(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.SetRequestURI("http://google.com/test")
	assertEqual(t, "http://google.com", ctx.BaseURL())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Append -benchmem -count=4
func Benchmark_Ctx_BaseURL(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	c.Fasthttp.Request.SetHost("google.com:1337")
	c.Fasthttp.Request.URI().SetPath("/haha/oke/lol")
	var res string
	for n := 0; n < b.N; n++ {
		res = c.BaseURL()
	}
	assertEqual(b, "http://google.com:1337", res)
}

// go test -run Test_Ctx_Body
func Test_Ctx_Body(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.SetBody([]byte("john=doe"))
	assertEqual(t, "john=doe", ctx.Body())
}

// go test -run Test_Ctx_BodyParser
func Test_Ctx_BodyParser(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	type Demo struct {
		Name string `json:"name" xml:"name" form:"name" query:"name"`
	}
	ctx.Fasthttp.Request.SetBody([]byte(`{"name":"john"}`))
	ctx.Fasthttp.Request.Header.SetContentType(MIMEApplicationJSON)
	ctx.Fasthttp.Request.Header.SetContentLength(len([]byte(`{"name":"john"}`)))
	d := new(Demo)
	assertEqual(t, nil, ctx.BodyParser(d))
	assertEqual(t, "john", d.Name)

	type Query struct {
		ID    int
		Name  string
		Hobby []string
	}
	ctx.Fasthttp.Request.SetBody([]byte(``))
	ctx.Fasthttp.Request.Header.SetContentType("")
	ctx.Fasthttp.Request.URI().SetQueryString("id=1&name=tom&hobby=basketball&hobby=football")
	q := new(Query)
	assertEqual(t, nil, ctx.BodyParser(q))
	assertEqual(t, 2, len(q.Hobby))
}

// TODO Benchmark_Ctx_BodyParser

// go test -v -run=^$ -bench=Benchmark_Ctx_Cookie -benchmem -count=4
func Benchmark_Ctx_Cookie(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	for n := 0; n < b.N; n++ {
		c.Cookie(&Cookie{
			Name:  "John",
			Value: "Doe",
		})
	}
	assertEqual(b, "John=Doe; path=/", getString(c.Fasthttp.Response.Header.Peek("Set-Cookie")))
}

// go test -run Test_Ctx_Cookie
func Test_Ctx_Cookie(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	expire := time.Now().Add(24 * time.Hour)
	var dst []byte
	dst = expire.In(time.UTC).AppendFormat(dst, time.RFC1123)
	httpdate := strings.Replace(string(dst), "UTC", "GMT", -1)
	ctx.Cookie(&Cookie{
		Name:    "username",
		Value:   "john",
		Expires: expire,
	})
	expect := "username=john; expires=" + string(httpdate) + "; path=/"
	assertEqual(t, expect, string(ctx.Fasthttp.Response.Header.Peek(HeaderSetCookie)))
}

// go test -run Test_Ctx_Cookies
func Test_Ctx_Cookies(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set("Cookie", "john=doe")
	assertEqual(t, "doe", ctx.Cookies("john"))
}

// go test -run Test_Ctx_Format
func Test_Ctx_Format(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderAccept, "text/html")
	ctx.Format("Hello, World!")
	assertEqual(t, "<p>Hello, World!</p>", string(ctx.Fasthttp.Response.Body()))

	ctx.Fasthttp.Request.Header.Set(HeaderAccept, "application/json")
	ctx.Format("Hello, World!")
	assertEqual(t, `"Hello, World!"`, string(ctx.Fasthttp.Response.Body()))

	ctx.Fasthttp.Request.Header.Set(HeaderAccept, "application/xml")
	ctx.Format("Hello, World!")
	assertEqual(t, `<string>Hello, World!</string>`, string(ctx.Fasthttp.Response.Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Format -benchmem -count=4
func Benchmark_Ctx_Format(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set("Accept", "text/plain")
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	assertEqual(b, `Hello, World!`, string(c.Fasthttp.Response.Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Format_HTML -benchmem -count=4
func Benchmark_Ctx_Format_HTML(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set("Accept", "text/html")
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	assertEqual(b, "<p>Hello, World!</p>", string(c.Fasthttp.Response.Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Format_JSON -benchmem -count=4
func Benchmark_Ctx_Format_JSON(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set("Accept", "application/json")
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	assertEqual(b, `"Hello, World!"`, string(c.Fasthttp.Response.Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Format_XML -benchmem -count=4
func Benchmark_Ctx_Format_XML(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set("Accept", "application/xml")
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	assertEqual(b, `<string>Hello, World!</string>`, string(c.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_FormFile
func Test_Ctx_FormFile(t *testing.T) {
	// TODO: CLEAN THIS UP
	t.Parallel()
	app := New()

	app.Post("/test", func(c *Ctx) {
		fh, err := c.FormFile("file")
		assertEqual(t, nil, err)
		assertEqual(t, "test", fh.Filename)

		f, err := fh.Open()
		assertEqual(t, nil, err)

		b := new(bytes.Buffer)
		_, err = io.Copy(b, f)
		assertEqual(t, nil, err)

		f.Close()
		assertEqual(t, "hello world", b.String())
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	ioWriter, err := writer.CreateFormFile("file", "test")
	assertEqual(t, nil, err)

	_, err = ioWriter.Write([]byte("hello world"))
	assertEqual(t, nil, err)

	writer.Close()

	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_FormValue
func Test_Ctx_FormValue(t *testing.T) {
	t.Parallel()
	app := New()

	app.Post("/test", func(c *Ctx) {
		assertEqual(t, "john", c.FormValue("name"))
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	assertEqual(t, nil, writer.WriteField("name", "john"))

	writer.Close()
	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", writer.Boundary()))
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Fresh
func Test_Ctx_Fresh(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	assertEqual(t, false, ctx.Fresh())
}

// go test -run Test_Ctx_Get
func Test_Ctx_Get(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderAcceptCharset, "utf-8, iso-8859-1;q=0.5")
	ctx.Fasthttp.Request.Header.Set(HeaderReferer, "Monster")
	assertEqual(t, "utf-8, iso-8859-1;q=0.5", ctx.Get(HeaderAcceptCharset))
	assertEqual(t, "Monster", ctx.Get(HeaderReferer))
}

// go test -run Test_Ctx_Hostname
func Test_Ctx_Hostname(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.SetRequestURI("http://google.com/test")
	assertEqual(t, "google.com", ctx.Hostname())
}

// go test -run Test_Ctx_IP
func Test_Ctx_IP(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	assertEqual(t, "0.0.0.0", ctx.IP())
}

// go test -run Test_Ctx_IPs
func Test_Ctx_IPs(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderXForwardedFor, "127.0.0.1, 127.0.0.1, 127.0.0.1")
	assertEqual(t, []string{"127.0.0.1", "127.0.0.1", "127.0.0.1"}, ctx.IPs())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_IPs -benchmem -count=4
func Benchmark_Ctx_IPs(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set(HeaderXForwardedFor, "127.0.0.1, 127.0.0.1, 127.0.0.1")
	var res []string
	for n := 0; n < b.N; n++ {
		res = c.IPs()
	}
	assertEqual(b, []string{"127.0.0.1", "127.0.0.1", "127.0.0.1"}, res)
}

// go test -run Test_Ctx_Is
func Test_Ctx_Is(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderContentType, MIMETextHTML+"; boundary=something")
	assertEqual(t, true, ctx.Is(".html"))
	assertEqual(t, true, ctx.Is("html"))
	assertEqual(t, false, ctx.Is("json"))
	assertEqual(t, false, ctx.Is(".json"))
	assertEqual(t, false, ctx.Is(""))
	assertEqual(t, false, ctx.Is(".foooo"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Is -benchmem -count=4
func Benchmark_Ctx_Is(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	c.Fasthttp.Request.Header.Set(HeaderContentType, MIMEApplicationJSON)
	var res bool
	for n := 0; n < b.N; n++ {
		res = c.Is(".json")
		res = c.Is("json")
	}
	assertEqual(b, true, res)
}

// go test -run Test_Ctx_Locals
func Test_Ctx_Locals(t *testing.T) {
	app := New()
	app.Use(func(c *Ctx) {
		c.Locals("john", "doe")
		c.Next()
	})
	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "doe", c.Locals("john"))
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Method
func Test_Ctx_Method(t *testing.T) {
	t.Parallel()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	ctx := AcquireCtx(fctx)
	defer ReleaseCtx(ctx)
	assertEqual(t, MethodGet, ctx.Method())
}

// go test -run Test_Ctx_MultipartForm
func Test_Ctx_MultipartForm(t *testing.T) {
	t.Parallel()
	app := New()

	app.Post("/test", func(c *Ctx) {
		result, err := c.MultipartForm()
		assertEqual(t, nil, err)
		assertEqual(t, "john", result.Value["name"][0])
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	assertEqual(t, nil, writer.WriteField("name", "john"))

	writer.Close()
	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set(HeaderContentType, fmt.Sprintf("multipart/form-data; boundary=%s", writer.Boundary()))
	req.Header.Set(HeaderContentLength, strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_OriginalURL
func Test_Ctx_OriginalURL(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.SetRequestURI("http://google.com/test?search=demo")
	assertEqual(t, "http://google.com/test?search=demo", ctx.OriginalURL())
}

// go test -run Test_Ctx_Params
func Test_Ctx_Params(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test/:user", func(c *Ctx) {
		assertEqual(t, "john", c.Params("user"))
	})
	app.Get("/test2/*", func(c *Ctx) {
		assertEqual(t, "im/a/cookie", c.Params("*"))
	})
	app.Get("/test3/:optional?", func(c *Ctx) {
		assertEqual(t, "", c.Params("optional"))
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test/john", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, StatusOK, resp.StatusCode, "Status code")
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test2/im/a/cookie", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, StatusOK, resp.StatusCode, "Status code")
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test3", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Params -benchmem -count=4
func Benchmark_Ctx_Params(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	c.route = &Route{
		Params: []string{
			"param1", "param2", "param3", "param4",
		},
	}
	c.values = []string{
		"john", "doe", "is", "awesome",
	}
	var res string
	for n := 0; n < b.N; n++ {
		res = c.Params("param1")
		res = c.Params("param2")
		res = c.Params("param3")
		res = c.Params("param4")
	}
	assertEqual(b, "awesome", res)
}

// go test -run Test_Ctx_Path
func Test_Ctx_Path(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test/:user", func(c *Ctx) {
		assertEqual(t, "/test/john", c.Path())
		// not strict && case insensitive
		assertEqual(t, "/abc", c.Path("/ABC/"))
		assertEqual(t, "/test/john", c.Path("/test/john/"))
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test/john", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Query
func Test_Ctx_Query(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.URI().SetQueryString("search=john&age=20")
	assertEqual(t, "john", ctx.Query("search"))
	assertEqual(t, "20", ctx.Query("age"))
}

// go test -run Test_Ctx_Range
func Test_Ctx_Range(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderRange, "bytes=500-700")
	result, err := ctx.Range(1000)
	assertEqual(t, nil, err)
	assertEqual(t, "bytes", result.Type)
	assertEqual(t, 500, result.Ranges[0].Start)
	assertEqual(t, 700, result.Ranges[0].End)
}

// go test -run Test_Ctx_Route
func Test_Ctx_Route(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "/test", c.Route().Path)
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_SaveFile
func Test_Ctx_SaveFile(t *testing.T) {
	// TODO CLEAN THIS UP
	t.Parallel()
	app := New()

	app.Post("/test", func(c *Ctx) {
		fh, err := c.FormFile("file")
		assertEqual(t, nil, err)

		tempFile, err := ioutil.TempFile(os.TempDir(), "test-")
		assertEqual(t, nil, err)

		defer os.Remove(tempFile.Name())
		err = c.SaveFile(fh, tempFile.Name())
		assertEqual(t, nil, err)

		bs, err := ioutil.ReadFile(tempFile.Name())
		assertEqual(t, nil, err)
		assertEqual(t, "hello world", string(bs))
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	ioWriter, err := writer.CreateFormFile("file", "test")
	assertEqual(t, nil, err)

	_, err = ioWriter.Write([]byte("hello world"))
	assertEqual(t, nil, err)
	writer.Close()

	req := httptest.NewRequest(MethodPost, "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Secure
func Test_Ctx_Secure(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	// TODO Add TLS conn
	assertEqual(t, false, ctx.Secure())
}

// go test -run Test_Ctx_Stale
func Test_Ctx_Stale(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	assertEqual(t, true, ctx.Stale())
}

// go test -run Test_Ctx_Subdomains
func Test_Ctx_Subdomains(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.URI().SetHost("john.doe.google.com")
	assertEqual(t, []string{"john", "doe"}, ctx.Subdomains())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Subdomains -benchmem -count=4
func Benchmark_Ctx_Subdomains(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	c.Fasthttp.Request.SetRequestURI("http://john.doe.google.com")
	var res []string
	for n := 0; n < b.N; n++ {
		res = c.Subdomains()
	}
	assertEqual(b, []string{"john", "doe"}, res)
}

// go test -run Test_Ctx_ClearCookie
func Test_Ctx_ClearCookie(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderCookie, "john=doe")
	ctx.ClearCookie("john")
	assertEqual(t, true, strings.HasPrefix(string(ctx.Fasthttp.Response.Header.Peek(HeaderSetCookie)), "john=; expires="))
}

// go test -run Test_Ctx_Download
func Test_Ctx_Download(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)

	ctx.Download("ctx.go")

	f, err := os.Open("./ctx.go")
	assertEqual(t, nil, err)
	defer f.Close()

	expect, err := ioutil.ReadAll(f)
	assertEqual(t, nil, err)
	assertEqual(t, true, bytes.Equal(expect, ctx.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_JSON
func Test_Ctx_JSON(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.JSON(Map{ // map has no order
		"Name": "Grame",
		"Age":  20,
	})
	assertEqual(t, `{"Age":20,"Name":"Grame"}`, string(ctx.Fasthttp.Response.Body()))
	assertEqual(t, "application/json", string(ctx.Fasthttp.Response.Header.Peek("content-type")))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_JSON -benchmem -count=4
func Benchmark_Ctx_JSON(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	type SomeStruct struct {
		Name string
		Age  uint8
	}
	data := SomeStruct{
		Name: "Grame",
		Age:  20,
	}
	var err error
	for n := 0; n < b.N; n++ {
		err = c.JSON(data)
	}
	assertEqual(b, nil, err)
	assertEqual(b, `{"Name":"Grame","Age":20}`, string(c.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_JSONP
func Test_Ctx_JSONP(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.JSONP(Map{ // map has no order
		"Name": "Grame",
		"Age":  20,
	}, "john")
	assertEqual(t, `john({"Age":20,"Name":"Grame"});`, string(ctx.Fasthttp.Response.Body()))
	assertEqual(t, "application/javascript", string(ctx.Fasthttp.Response.Header.Peek("content-type")))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_JSONP -benchmem -count=4
func Benchmark_Ctx_JSONP(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
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
	for n := 0; n < b.N; n++ {
		err = c.JSONP(data, callback)
	}
	assertEqual(b, nil, err)
	assertEqual(b, `emit({"Name":"Grame","Age":20});`, string(c.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_Links
func Test_Ctx_Links(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Links(
		"http://api.example.com/users?page=2", "next",
		"http://api.example.com/users?page=5", "last",
	)
	assertEqual(t, `<http://api.example.com/users?page=2>; rel="next",<http://api.example.com/users?page=5>; rel="last"`, string(ctx.Fasthttp.Response.Header.Peek(HeaderLink)))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Links -benchmem -count=4
func Benchmark_Ctx_Links(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
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
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Location("http://example.com")
	assertEqual(t, "http://example.com", string(ctx.Fasthttp.Response.Header.Peek(HeaderLocation)))
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
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, StatusOK, resp.StatusCode, "Status code")
	assertEqual(t, "Works", resp.Header.Get("X-Next-Result"))
}

// go test -run Test_Ctx_Redirect
func Test_Ctx_Redirect(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Redirect("http://example.com", 301)
	assertEqual(t, 301, ctx.Fasthttp.Response.StatusCode())
	assertEqual(t, "http://example.com", string(ctx.Fasthttp.Response.Header.Peek(HeaderLocation)))
}

// ViewEngine is coming in v1.10
// func Test_Ctx_Render(t *testing.T) {
// 	// TODO
// }

// go test -run Test_Ctx_Send
func Test_Ctx_Send(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Send([]byte("Hello, World"))
	ctx.Send("Don't crash please")
	ctx.Send(1337)
	assertEqual(t, "1337", string(ctx.Fasthttp.Response.Body()))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Send -benchmem -count=4
func Benchmark_Ctx_Send(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	var str = "Hello, World!"
	var byt = []byte("Hello, World!")
	var nmb = 123
	var bol = true
	for n := 0; n < b.N; n++ {
		c.Send(str)
		c.Send(byt)
		c.Send(nmb)
		c.Send(bol)
	}
	assertEqual(b, "true", string(c.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_SendBytes
func Test_Ctx_SendBytes(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.SendBytes([]byte("Hello, World!"))
	assertEqual(t, "Hello, World!", string(ctx.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_SendStatus
func Test_Ctx_SendStatus(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.SendStatus(415)
	assertEqual(t, 415, ctx.Fasthttp.Response.StatusCode())
	assertEqual(t, "Unsupported Media Type", string(ctx.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_SendString
func Test_Ctx_SendString(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.SendString("Don't crash please")
	assertEqual(t, "Don't crash please", string(ctx.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_Set
func Test_Ctx_Set(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Set("X-1", "1")
	ctx.Set("X-2", "2")
	ctx.Set("X-3", "3")
	ctx.Set("X-3", "1337")
	assertEqual(t, "1", string(ctx.Fasthttp.Response.Header.Peek("x-1")))
	assertEqual(t, "2", string(ctx.Fasthttp.Response.Header.Peek("x-2")))
	assertEqual(t, "1337", string(ctx.Fasthttp.Response.Header.Peek("x-3")))
}

// go test -run Test_Ctx_Status
func Test_Ctx_Status(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Status(400)
	assertEqual(t, 400, ctx.Fasthttp.Response.StatusCode())
	ctx.Status(415).Send("Hello, World")
	assertEqual(t, 415, ctx.Fasthttp.Response.StatusCode())
	assertEqual(t, "Hello, World", string(ctx.Fasthttp.Response.Body()))
}

// go test -run Test_Ctx_Type
func Test_Ctx_Type(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Type(".json")
	assertEqual(t, "application/json", string(ctx.Fasthttp.Response.Header.Peek("Content-Type")))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Type -benchmem -count=4
func Benchmark_Ctx_Type(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	for n := 0; n < b.N; n++ {
		c.Type(".json")
		c.Type("json")
	}
}

// go test -run Test_Ctx_Vary
func Test_Ctx_Vary(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Vary("Origin")
	ctx.Vary("User-Agent")
	ctx.Vary("Accept-Encoding", "Accept")
	assertEqual(t, "Origin, User-Agent, Accept-Encoding, Accept", string(ctx.Fasthttp.Response.Header.Peek("Vary")))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Vary -benchmem -count=4
func Benchmark_Ctx_Vary(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	for n := 0; n < b.N; n++ {
		c.Vary("Origin", "User-Agent")
	}
}

// go test -run Test_Ctx_Write
func Test_Ctx_Write(t *testing.T) {
	t.Parallel()
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Write("Hello, ")
	ctx.Write([]byte("World! "))
	ctx.Write(123)
	assertEqual(t, "Hello, World! 123", string(ctx.Fasthttp.Response.Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Write -benchmem -count=4
func Benchmark_Ctx_Write(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)
	var str = "Hello, World!"
	var byt = []byte("Hello, World!")
	var nmb = 123
	var bol = true
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
	ctx := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(ctx)
	ctx.Fasthttp.Request.Header.Set(HeaderXRequestedWith, "XMLHttpRequest")
	assertEqual(t, true, ctx.XHR())
}
