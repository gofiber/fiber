// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

// go test -v -run=^$ -bench=Benchmark_Ctx_Accepts -benchmem -count=4
// go test -run Test_Ctx

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/gofiber/fiber/v2/internal/bytebufferpool"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Ctx_Accepts
func Test_Ctx_Accepts(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	utils.AssertEqual(t, ".forwarded", c.Accepts(".forwarded"))
}

// go test -run Test_Ctx_Accepts_Wildcard
func Test_Ctx_Accepts_Wildcard(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderAcceptCharset, "utf-8, iso-8859-1;q=0.5")
	utils.AssertEqual(t, "utf-8", c.AcceptsCharsets("utf-8"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsCharsets -benchmem -count=4
func Benchmark_Ctx_AcceptsCharsets(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderAcceptEncoding, "deflate, gzip;q=1.0, *;q=0.5")
	utils.AssertEqual(t, "gzip", c.AcceptsEncodings("gzip"))
	utils.AssertEqual(t, "abc", c.AcceptsEncodings("abc"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsEncodings -benchmem -count=4
func Benchmark_Ctx_AcceptsEncodings(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderAcceptLanguage, "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")
	utils.AssertEqual(t, "fr", c.AcceptsLanguages("fr"))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcceptsLanguages -benchmem -count=4
func Benchmark_Ctx_AcceptsLanguages(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	utils.AssertEqual(t, 1000, c.App().config.BodyLimit)
}

// go test -run Test_Ctx_Append
func Test_Ctx_Append(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Append("X-Custom-Header", "Hello")
		c.Append("X-Custom-Header", "World")
		c.Append("X-Custom-Header", "Hello")
	}
	utils.AssertEqual(b, "Hello, World", getString(c.Response().Header.Peek("X-Custom-Header")))
}

// go test -run Test_Ctx_Attachment
func Test_Ctx_Attachment(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().SetRequestURI("http://google.com/test")
	utils.AssertEqual(t, "http://google.com", c.BaseURL())
	// Check cache
	utils.AssertEqual(t, "http://google.com", c.BaseURL())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_BaseURL -benchmem
func Benchmark_Ctx_BaseURL(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().SetBody([]byte("john=doe"))
	utils.AssertEqual(t, []byte("john=doe"), c.Body())
}

// go test -run Test_Ctx_BodyParser
func Test_Ctx_BodyParser(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	type Demo struct {
		Name string `json:"name" xml:"name" form:"name" query:"name"`
	}

	testDecodeParser := func(contentType, body string) {
		c.Request().Header.SetContentType(contentType)
		c.Request().SetBody([]byte(body))
		c.Request().Header.SetContentLength(len(body))
		d := new(Demo)
		utils.AssertEqual(t, nil, c.BodyParser(d))
		utils.AssertEqual(t, "john", d.Name)
	}

	testDecodeParser(MIMEApplicationJSON, `{"name":"john"}`)
	testDecodeParser(MIMEApplicationXML, `<Demo><name>john</name></Demo>`)
	testDecodeParser(MIMEApplicationJSON, `{"name":"john"}`)
	testDecodeParser(MIMEApplicationForm, "name=john")
	testDecodeParser(MIMEMultipartForm+`;boundary="b"`, "--b\r\nContent-Disposition: form-data; name=\"name\"\r\n\r\njohn\r\n--b--")

	testDecodeParserError := func(contentType, body string) {
		c.Request().Header.SetContentType(contentType)
		c.Request().SetBody([]byte(body))
		c.Request().Header.SetContentLength(len(body))
		utils.AssertEqual(t, false, c.BodyParser(nil) == nil)
	}

	testDecodeParserError("invalid-content-type", "")
	testDecodeParserError(MIMEMultipartForm+`;boundary="b"`, "--b")
}

// go test -v -run=^$ -bench=Benchmark_Ctx_BodyParser_JSON -benchmem -count=4
func Benchmark_Ctx_BodyParser_JSON(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	type Demo struct {
		Name string `json:"name"`
	}
	body := []byte(`{"name":"john"}`)
	c.Request().SetBody(body)
	c.Request().Header.SetContentType(MIMEApplicationJSON)
	c.Request().Header.SetContentLength(len(body))
	d := new(Demo)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_ = c.BodyParser(d)
	}
	utils.AssertEqual(b, nil, c.BodyParser(d))
	utils.AssertEqual(b, "john", d.Name)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_BodyParser_XML -benchmem -count=4
func Benchmark_Ctx_BodyParser_XML(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	type Demo struct {
		Name string `xml:"name"`
	}
	body := []byte("<Demo><name>john</name></Demo>")
	c.Request().SetBody(body)
	c.Request().Header.SetContentType(MIMEApplicationXML)
	c.Request().Header.SetContentLength(len(body))
	d := new(Demo)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_ = c.BodyParser(d)
	}
	utils.AssertEqual(b, nil, c.BodyParser(d))
	utils.AssertEqual(b, "john", d.Name)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_BodyParser_Form -benchmem -count=4
func Benchmark_Ctx_BodyParser_Form(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	type Demo struct {
		Name string `form:"name"`
	}
	body := []byte("name=john")
	c.Request().SetBody(body)
	c.Request().Header.SetContentType(MIMEApplicationForm)
	c.Request().Header.SetContentLength(len(body))
	d := new(Demo)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_ = c.BodyParser(d)
	}
	utils.AssertEqual(b, nil, c.BodyParser(d))
	utils.AssertEqual(b, "john", d.Name)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_BodyParser_MultipartForm -benchmem -count=4
func Benchmark_Ctx_BodyParser_MultipartForm(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	type Demo struct {
		Name string `form:"name"`
	}

	body := []byte("--b\r\nContent-Disposition: form-data; name=\"name\"\r\n\r\njohn\r\n--b--")
	c.Request().SetBody(body)
	c.Request().Header.SetContentType(MIMEMultipartForm + `;boundary="b"`)
	c.Request().Header.SetContentLength(len(body))
	d := new(Demo)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		_ = c.BodyParser(d)
	}
	utils.AssertEqual(b, nil, c.BodyParser(d))
	utils.AssertEqual(b, "john", d.Name)
}

// go test -run Test_Ctx_Context
func Test_Ctx_Context(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	utils.AssertEqual(t, "*fasthttp.RequestCtx", fmt.Sprintf("%T", c.Context()))
}

// go test -run Test_Ctx_Cookie
func Test_Ctx_Cookie(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	expire := time.Now().Add(24 * time.Hour)
	var dst []byte
	dst = expire.In(time.UTC).AppendFormat(dst, time.RFC1123)
	httpdate := strings.Replace(string(dst), "UTC", "GMT", -1)
	c.Cookie(&Cookie{
		Name:    "username",
		Value:   "john",
		Expires: expire,
	})
	expect := "username=john; expires=" + httpdate + "; path=/; SameSite=Lax"
	utils.AssertEqual(t, expect, string(c.Response().Header.Peek(HeaderSetCookie)))

	c.Cookie(&Cookie{SameSite: "strict"})
	c.Cookie(&Cookie{SameSite: "none"})
}

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
	utils.AssertEqual(b, "John=Doe; path=/; SameSite=Lax", getString(c.Response().Header.Peek("Set-Cookie")))
}

// go test -run Test_Ctx_Cookies
func Test_Ctx_Cookies(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set("Cookie", "john=doe")
	utils.AssertEqual(t, "doe", c.Cookies("john"))
	utils.AssertEqual(t, "default", c.Cookies("unknown", "default"))
}

// go test -run Test_Ctx_Format
func Test_Ctx_Format(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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

	app.Post("/test", func(c *Ctx) error {
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

	app.Post("/test", func(c *Ctx) error {
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().SetRequestURI("http://google.com/test")
	utils.AssertEqual(t, "google.com", c.Hostname())
}

// go test -run Test_Ctx_IP
func Test_Ctx_IP(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	utils.AssertEqual(t, "0.0.0.0", c.IP())
}

// go test -run Test_Ctx_IP_ProxyHeader
func Test_Ctx_IP_ProxyHeader(t *testing.T) {
	t.Parallel()
	app := New(Config{ProxyHeader: "Real-Ip"})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	utils.AssertEqual(t, "", c.IP())
}

// go test -run Test_Ctx_IPs  -parallel
func Test_Ctx_IPs(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	app.Use(func(c *Ctx) error {
		c.Locals("john", "doe")
		return c.Next()
	})
	app.Get("/test", func(c *Ctx) error {
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
	c := app.AcquireCtx(fctx)
	defer app.ReleaseCtx(c)
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
	app.Get("/", func(c *Ctx) error {
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

	app.Post("/test", func(c *Ctx) error {
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

	app.Post("/", func(c *Ctx) error {
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.SetRequestURI("http://google.com/test?search=demo")
	utils.AssertEqual(t, "http://google.com/test?search=demo", c.OriginalURL())
}

// go test -race -run Test_Ctx_Params
func Test_Ctx_Params(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test/:user", func(c *Ctx) error {
		utils.AssertEqual(t, "john", c.Params("user"))
		return nil
	})
	app.Get("/test2/*", func(c *Ctx) error {
		utils.AssertEqual(t, "im/a/cookie", c.Params("*"))
		return nil
	})
	app.Get("/test3/*/blafasel/*", func(c *Ctx) error {
		utils.AssertEqual(t, "1111", c.Params("*1"))
		utils.AssertEqual(t, "2222", c.Params("*2"))
		utils.AssertEqual(t, "1111", c.Params("*"))
		return nil
	})
	app.Get("/test4/:optional?", func(c *Ctx) error {
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	app.Get("/test/:user", func(c *Ctx) error {
		utils.AssertEqual(t, "/Test/John", c.Path())
		// not strict && case insensitive
		utils.AssertEqual(t, "/ABC/", c.Path("/ABC/"))
		utils.AssertEqual(t, "/test/john/", c.Path("/test/john/"))
		return nil
	})

	// test with special chars
	app.Get("/specialChars/:name", func(c *Ctx) error {
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

	freq := &fasthttp.RequestCtx{}
	freq.Request.Header.Set("X-Forwarded", "invalid")

	c := app.AcquireCtx(freq)
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderXForwardedProto, "https")
	utils.AssertEqual(t, "https", c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, "https")
	utils.AssertEqual(t, "https", c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	utils.AssertEqual(t, "https", c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, "https")
	utils.AssertEqual(t, "https", c.Protocol())
	c.Request().Header.Reset()

	utils.AssertEqual(t, "http", c.Protocol())
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().URI().SetQueryString("search=john&age=20")
	utils.AssertEqual(t, "john", c.Query("search"))
	utils.AssertEqual(t, "20", c.Query("age"))
	utils.AssertEqual(t, "default", c.Query("unknown", "default"))
}

// go test -run Test_Ctx_Range
func Test_Ctx_Range(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	var (
		result Range
		err    error
	)

	result, err = c.Range(1000)
	utils.AssertEqual(t, true, err != nil)

	c.Request().Header.Set(HeaderRange, "bytes=500")
	result, err = c.Range(1000)
	utils.AssertEqual(t, true, err != nil)

	c.Request().Header.Set(HeaderRange, "bytes=500=")
	result, err = c.Range(1000)
	utils.AssertEqual(t, true, err != nil)

	c.Request().Header.Set(HeaderRange, "bytes=500-300")
	result, err = c.Range(1000)
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
	app.Get("/test", func(c *Ctx) error {
		utils.AssertEqual(t, "/test", c.Route().Path)
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	utils.AssertEqual(t, "/", c.Route().Path)
	utils.AssertEqual(t, MethodGet, c.Route().Method)
	utils.AssertEqual(t, 0, len(c.Route().Handlers))
}

// go test -run Test_Ctx_RouteNormalized
func Test_Ctx_RouteNormalized(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) error {
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

	app.Post("/test", func(c *Ctx) error {
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	// TODO Add TLS conn
	utils.AssertEqual(t, false, c.Secure())
}

// go test -run Test_Ctx_Stale
func Test_Ctx_Stale(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	utils.AssertEqual(t, true, c.Stale())
}

// go test -run Test_Ctx_Subdomains
func Test_Ctx_Subdomains(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().URI().SetHost("john.doe.is.awesome.google.com")
	utils.AssertEqual(t, []string{"john", "doe"}, c.Subdomains(4))

	c.Request().URI().SetHost("localhost:3000")
	utils.AssertEqual(t, []string{"localhost:3000"}, c.Subdomains())
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Subdomains -benchmem -count=4
func Benchmark_Ctx_Subdomains(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	c.Download("ctx.go", "Awesome File!")

	f, err := os.Open("./ctx.go")
	utils.AssertEqual(t, nil, err)
	defer f.Close()

	expect, err := ioutil.ReadAll(f)
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
	expectFileContent, err := ioutil.ReadAll(f)
	utils.AssertEqual(t, nil, err)
	// fetch file info for the not modified test case
	fI, err := os.Stat("./ctx.go")
	utils.AssertEqual(t, nil, err)

	// simple test case
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	err = c.SendFile("ctx.go")
	// check expectation
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectFileContent, c.Response().Body())
	utils.AssertEqual(t, StatusOK, c.Response().StatusCode())
	app.ReleaseCtx(c)

	// test with custom error code
	c = app.AcquireCtx(&fasthttp.RequestCtx{})
	err = c.Status(StatusInternalServerError).SendFile("ctx.go")
	// check expectation
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectFileContent, c.Response().Body())
	utils.AssertEqual(t, StatusInternalServerError, c.Response().StatusCode())
	app.ReleaseCtx(c)

	// test not modified
	c = app.AcquireCtx(&fasthttp.RequestCtx{})
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
	app.Get("/", func(c *Ctx) error {
		err := c.SendFile("./john_dow.go/")
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
	app.Get("/:file", func(c *Ctx) error {
		file := c.Params("file")
		if err := c.SendFile("./.github/" + file + ".html"); err != nil {
			utils.AssertEqual(t, nil, err)
		}
		utils.AssertEqual(t, "index", file)
		return c.SendString(file)
	})
	// 1st try
	resp, err := app.Test(httptest.NewRequest("GET", "/index", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, StatusOK, resp.StatusCode)
	// 2nd try
	resp, err = app.Test(httptest.NewRequest("GET", "/index", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, StatusOK, resp.StatusCode)
}

// go test -run Test_Ctx_JSON
func Test_Ctx_JSON(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	utils.AssertEqual(t, true, c.JSON(complex(1, 1)) != nil)

	c.JSON(Map{ // map has no order
		"Name": "Grame",
		"Age":  20,
	})
	utils.AssertEqual(t, `{"Age":20,"Name":"Grame"}`, string(c.Response().Body()))
	utils.AssertEqual(t, "application/json", string(c.Response().Header.Peek("content-type")))

	testEmpty := func(v interface{}, r string) {
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
	utils.AssertEqual(b, `{"Name":"Grame","Age":20}`, string(c.Response().Body()))
}

// go test -run Test_Ctx_JSONP
func Test_Ctx_JSONP(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

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
	utils.AssertEqual(b, `emit({"Name":"Grame","Age":20});`, string(c.Response().Body()))
}

// go test -run Test_Ctx_Links
func Test_Ctx_Links(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Location("http://example.com")
	utils.AssertEqual(t, "http://example.com", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_Next
func Test_Ctx_Next(t *testing.T) {
	app := New()
	app.Use("/", func(c *Ctx) error {
		return c.Next()
	})
	app.Get("/test", func(c *Ctx) error {
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
	app.Use("/", func(c *Ctx) error {
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	c.Redirect("http://default.com")
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "http://default.com", string(c.Response().Header.Peek(HeaderLocation)))

	c.Redirect("http://example.com", 301)
	utils.AssertEqual(t, 301, c.Response().StatusCode())
	utils.AssertEqual(t, "http://example.com", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_Render
func Test_Ctx_Render(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	err := c.Render("./.github/testdata/template.html", Map{
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

type testTemplateEngine struct {
	mu        sync.Mutex
	templates *template.Template
}

func (t *testTemplateEngine) Render(w io.Writer, name string, bind interface{}, layout ...string) error {
	return t.templates.ExecuteTemplate(w, name, bind)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	err := c.Render("index.tmpl", Map{
		"Title": "Hello, World!",
	})
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Render_Engine -benchmem -count=4
func Benchmark_Ctx_Render_Engine(b *testing.B) {
	engine := &testTemplateEngine{}
	err := engine.Load()
	utils.AssertEqual(b, nil, err)
	app := New()
	app.config.Views = engine
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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

type errorTemplateEngine struct{}

func (t errorTemplateEngine) Render(w io.Writer, name string, bind interface{}, layout ...string) error {
	return errors.New("errorTemplateEngine")
}

func (t errorTemplateEngine) Load() error { return nil }

// go test -run Test_Ctx_Render_Engine_Error
func Test_Ctx_Render_Engine_Error(t *testing.T) {
	app := New()
	app.config.Views = errorTemplateEngine{}
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	err := c.Render("index.tmpl", nil)
	utils.AssertEqual(t, false, err == nil)
}

// go test -run Test_Ctx_Render_Go_Template
func Test_Ctx_Render_Go_Template(t *testing.T) {
	t.Parallel()

	file, err := ioutil.TempFile(os.TempDir(), "fiber")
	utils.AssertEqual(t, nil, err)
	defer os.Remove(file.Name())

	_, err = file.Write([]byte("template"))
	utils.AssertEqual(t, nil, err)

	err = file.Close()
	utils.AssertEqual(t, nil, err)

	app := New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	err = c.Render(file.Name(), nil)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "template", string(c.Response().Body()))
}

// go test -run Test_Ctx_Send
func Test_Ctx_Send(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Send([]byte("Hello, World"))
	c.Send([]byte("Don't crash please"))
	c.Send([]byte("1337"))
	utils.AssertEqual(t, "1337", string(c.Response().Body()))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Send -benchmem -count=4
func Benchmark_Ctx_Send(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	var byt = []byte("Hello, World!")
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.SendStatus(415)
	utils.AssertEqual(t, 415, c.Response().StatusCode())
	utils.AssertEqual(t, "Unsupported Media Type", string(c.Response().Body()))
}

// go test -run Test_Ctx_SendString
func Test_Ctx_SendString(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.SendString("Don't crash please")
	utils.AssertEqual(t, "Don't crash please", string(c.Response().Body()))
}

// go test -run Test_Ctx_SendStream
func Test_Ctx_SendStream(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	var val = "1431-15132-3423"
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Vary("Origin")
	c.Vary("User-Agent")
	c.Vary("Accept-Encoding", "Accept")
	utils.AssertEqual(t, "Origin, User-Agent, Accept-Encoding, Accept", string(c.Response().Header.Peek("Vary")))
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Write([]byte("Hello, "))
	c.Write([]byte("World!"))
	utils.AssertEqual(t, "Hello, World!", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Write -benchmem -count=4
func Benchmark_Ctx_Write(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	var byt = []byte("Hello, World!")
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.Write(byt)
	}
}

// go test -run Test_Ctx_WriteString
func Test_Ctx_WriteString(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.WriteString("Hello, ")
	c.WriteString("World!")
	utils.AssertEqual(t, "Hello, World!", string(c.Response().Body()))
}

// go test -run Test_Ctx_XHR
func Test_Ctx_XHR(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderXRequestedWith, "XMLHttpRequest")
	utils.AssertEqual(t, true, c.XHR())
}

// go test -run=^$ -bench=Benchmark_Ctx_XHR -benchmem -count=4
func Benchmark_Ctx_XHR(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	body := "Hello, world!"
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.SendString(body)
	}
	utils.AssertEqual(b, []byte("Hello, world!"), c.Response().Body())
}

// go test -run Test_Ctx_QueryParser -v
func Test_Ctx_QueryParser(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	type Query struct {
		ID    int
		Name  string
		Hobby []string
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball&hobby=football")
	q := new(Query)
	utils.AssertEqual(t, nil, c.QueryParser(q))
	utils.AssertEqual(t, 2, len(q.Hobby))

	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball,football")
	q = new(Query)
	utils.AssertEqual(t, nil, c.QueryParser(q))
	utils.AssertEqual(t, 2, len(q.Hobby))

	c.Request().URI().SetQueryString("id=1&name=tom&hobby=scoccer&hobby=basketball,football")
	q = new(Query)
	utils.AssertEqual(t, nil, c.QueryParser(q))
	utils.AssertEqual(t, 3, len(q.Hobby))

	empty := new(Query)
	c.Request().URI().SetQueryString("")
	utils.AssertEqual(t, nil, c.QueryParser(empty))
	utils.AssertEqual(t, 0, len(empty.Hobby))

	type Query2 struct {
		ID              int
		Name            string
		Hobby           string
		FavouriteDrinks []string
		Empty           []string
		No              []int64
	}

	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball,football&favouriteDrinks=milo,coke,pepsi&empty=&no=1")
	q2 := new(Query2)
	utils.AssertEqual(t, nil, c.QueryParser(q2))
	utils.AssertEqual(t, "basketball,football", q2.Hobby)
	utils.AssertEqual(t, []string{"milo", "coke", "pepsi"}, q2.FavouriteDrinks)
	utils.AssertEqual(t, []string{}, q2.Empty)
	utils.AssertEqual(t, []int64{1}, q2.No)

	type RequiredQuery struct {
		Name string `query:"name,required"`
	}
	rq := new(RequiredQuery)
	c.Request().URI().SetQueryString("")
	utils.AssertEqual(t, "name is empty", c.QueryParser(rq).Error())
}

func Test_Ctx_EqualFieldType(t *testing.T) {
	var out int
	utils.AssertEqual(t, false, equalFieldType(&out, reflect.Int, "key"))

	var dummy struct{ f string }
	utils.AssertEqual(t, false, equalFieldType(&dummy, reflect.String, "key"))

	var dummy2 struct{ f string }
	utils.AssertEqual(t, false, equalFieldType(&dummy2, reflect.String, "f"))

	var user struct {
		Name    string
		Address string `query:"address"`
		Age     int    `query:"AGE"`
	}
	utils.AssertEqual(t, true, equalFieldType(&user, reflect.String, "name"))
	utils.AssertEqual(t, true, equalFieldType(&user, reflect.String, "Name"))
	utils.AssertEqual(t, true, equalFieldType(&user, reflect.String, "address"))
	utils.AssertEqual(t, true, equalFieldType(&user, reflect.String, "Address"))
	utils.AssertEqual(t, true, equalFieldType(&user, reflect.Int, "AGE"))
	utils.AssertEqual(t, true, equalFieldType(&user, reflect.Int, "age"))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_QueryParser -benchmem -count=4
func Benchmark_Ctx_QueryParser(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	type Query struct {
		ID    int
		Name  string
		Hobby []string
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball&hobby=football")
	q := new(Query)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.QueryParser(q)
	}
	utils.AssertEqual(b, nil, c.QueryParser(q))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_QueryParser_Comma -benchmem -count=4
func Benchmark_Ctx_QueryParser_Comma(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	type Query struct {
		ID    int
		Name  string
		Hobby []string
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	// c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball&hobby=football")
	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball,football")
	q := new(Query)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.QueryParser(q)
	}
	utils.AssertEqual(b, nil, c.QueryParser(q))
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	utils.AssertEqual(t, "#0000000000000000 - 0.0.0.0:0 <-> 0.0.0.0:0 - GET http:///", c.String())
}

func TestCtx_ParamsInt(t *testing.T) {
	// Create a test context and set some strings (or params)

	// create a fake app to be used within this test
	app := New()

	// Create some test endpoints

	// For the user id I will use the number 1111, so I should be able to get the number
	// 1111 from the Ctx
	app.Get("/test/:user", func(c *Ctx) error {
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
	app.Get("/testnoint/:user", func(c *Ctx) error {
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

	app.Test(httptest.NewRequest(MethodGet, "/test/1111", nil))
	app.Test(httptest.NewRequest(MethodGet, "/testnoint/xd", nil))

}
