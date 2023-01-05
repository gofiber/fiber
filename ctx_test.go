// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

//nolint:bodyclose // Much easier to just ignore memory leaks in tests
package fiber

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/gofiber/fiber/v2/internal/storage/memory"
	"github.com/gofiber/fiber/v2/utils"

	"github.com/valyala/bytebufferpool"
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
	utils.AssertEqual(b, "Hello, World", app.getString(c.Response().Header.Peek("X-Custom-Header")))
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

// go test -run Test_Ctx_Body_With_Compression
func Test_Ctx_Body_With_Compression(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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

// go test -run Test_Ctx_BodyParser
func Test_Ctx_BodyParser(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	type Demo struct {
		Name string `json:"name" xml:"name" form:"name" query:"name"`
	}

	{
		var gzipJSON bytes.Buffer
		w := gzip.NewWriter(&gzipJSON)
		_, err := w.Write([]byte(`{"name":"john"}`))
		utils.AssertEqual(t, nil, err)
		err = w.Close()
		utils.AssertEqual(t, nil, err)

		c.Request().Header.SetContentType(MIMEApplicationJSON)
		c.Request().Header.Set(HeaderContentEncoding, "gzip")
		c.Request().SetBody(gzipJSON.Bytes())
		c.Request().Header.SetContentLength(len(gzipJSON.Bytes()))
		d := new(Demo)
		utils.AssertEqual(t, nil, c.BodyParser(d))
		utils.AssertEqual(t, "john", d.Name)
		c.Request().Header.Del(HeaderContentEncoding)
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

	type CollectionQuery struct {
		Data []Demo `query:"data"`
	}

	c.Request().Reset()
	c.Request().Header.SetContentType(MIMEApplicationForm)
	c.Request().SetBody([]byte("data[0][name]=john&data[1][name]=doe"))
	c.Request().Header.SetContentLength(len(c.Body()))
	cq := new(CollectionQuery)
	utils.AssertEqual(t, nil, c.BodyParser(cq))
	utils.AssertEqual(t, 2, len(cq.Data))
	utils.AssertEqual(t, "john", cq.Data[0].Name)
	utils.AssertEqual(t, "doe", cq.Data[1].Name)

	c.Request().Reset()
	c.Request().Header.SetContentType(MIMEApplicationForm)
	c.Request().SetBody([]byte("data.0.name=john&data.1.name=doe"))
	c.Request().Header.SetContentLength(len(c.Body()))
	cq = new(CollectionQuery)
	utils.AssertEqual(t, nil, c.BodyParser(cq))
	utils.AssertEqual(t, 2, len(cq.Data))
	utils.AssertEqual(t, "john", cq.Data[0].Name)
	utils.AssertEqual(t, "doe", cq.Data[1].Name)
}

func Test_Ctx_ParamParser(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test1/:userId/role/:roleId", func(ctx *Ctx) error {
		type Demo struct {
			UserID uint `params:"userId"`
			RoleID uint `params:"roleId"`
		}
		d := new(Demo)
		if err := ctx.ParamsParser(d); err != nil {
			t.Fatal(err)
		}
		utils.AssertEqual(t, uint(111), d.UserID)
		utils.AssertEqual(t, uint(222), d.RoleID)
		return nil
	})
	_, err := app.Test(httptest.NewRequest(MethodGet, "/test1/111/role/222", nil))
	utils.AssertEqual(t, nil, err)

	_, err = app.Test(httptest.NewRequest(MethodGet, "/test2/111/role/222", nil))
	utils.AssertEqual(t, nil, err)
}

// go test -run Test_Ctx_BodyParser_WithSetParserDecoder
func Test_Ctx_BodyParser_WithSetParserDecoder(t *testing.T) {
	t.Parallel()
	type CustomTime time.Time

	timeConverter := func(value string) reflect.Value {
		if v, err := time.Parse("2006-01-02", value); err == nil {
			return reflect.ValueOf(v)
		}
		return reflect.Value{}
	}

	customTime := ParserType{
		Customtype: CustomTime{},
		Converter:  timeConverter,
	}

	SetParserDecoder(ParserConfig{
		IgnoreUnknownKeys: true,
		ParserType:        []ParserType{customTime},
		ZeroEmpty:         true,
		SetAliasTag:       "form",
	})

	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	type Demo struct {
		Date  CustomTime `form:"date"`
		Title string     `form:"title"`
		Body  string     `form:"body"`
	}

	testDecodeParser := func(contentType, body string) {
		c.Request().Header.SetContentType(contentType)
		c.Request().SetBody([]byte(body))
		c.Request().Header.SetContentLength(len(body))
		d := Demo{
			Title: "Existing title",
			Body:  "Existing Body",
		}
		utils.AssertEqual(t, nil, c.BodyParser(&d))
		date := fmt.Sprintf("%v", d.Date)
		utils.AssertEqual(t, "{0 63743587200 <nil>}", date)
		utils.AssertEqual(t, "", d.Title)
		utils.AssertEqual(t, "New Body", d.Body)
	}

	testDecodeParser(MIMEApplicationForm, "date=2020-12-15&title=&body=New Body")
	testDecodeParser(MIMEMultipartForm+`; boundary="b"`, "--b\r\nContent-Disposition: form-data; name=\"date\"\r\n\r\n2020-12-15\r\n--b\r\nContent-Disposition: form-data; name=\"title\"\r\n\r\n\r\n--b\r\nContent-Disposition: form-data; name=\"body\"\r\n\r\nNew Body\r\n--b--")
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
		_ = c.BodyParser(d) //nolint:errcheck // It is fine to ignore the error here as we check it once further below
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
		_ = c.BodyParser(d) //nolint:errcheck // It is fine to ignore the error here as we check it once further below
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
		_ = c.BodyParser(d) //nolint:errcheck // It is fine to ignore the error here as we check it once further below
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
		_ = c.BodyParser(d) //nolint:errcheck // It is fine to ignore the error here as we check it once further below
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

// go test -run Test_Ctx_UserContext
func Test_Ctx_UserContext(t *testing.T) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	t.Run("Nil_Context", func(t *testing.T) {
		ctx := c.UserContext()
		utils.AssertEqual(t, ctx, context.Background())
	})
	t.Run("ValueContext", func(t *testing.T) {
		testKey := struct{}{}
		testValue := "Test Value"
		ctx := context.WithValue(context.Background(), testKey, testValue)
		utils.AssertEqual(t, testValue, ctx.Value(testKey))
	})
}

// go test -run Test_Ctx_SetUserContext
func Test_Ctx_SetUserContext(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	testKey := struct{}{}
	testValue := "Test Value"
	ctx := context.WithValue(context.Background(), testKey, testValue)
	c.SetUserContext(ctx)
	utils.AssertEqual(t, testValue, c.UserContext().Value(testKey))
}

// go test -run Test_Ctx_UserContext_Multiple_Requests
func Test_Ctx_UserContext_Multiple_Requests(t *testing.T) {
	t.Parallel()
	testKey := struct{}{}
	testValue := "foobar-value"

	app := New()
	app.Get("/", func(c *Ctx) error {
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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
	utils.AssertEqual(b, "John=Doe; path=/; SameSite=Lax", app.getString(c.Response().Header.Peek("Set-Cookie")))
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
	err := c.Format([]byte("Hello, World!"))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "Hello, World!", string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMETextHTML)
	err = c.Format("Hello, World!")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<p>Hello, World!</p>", string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMEApplicationJSON)
	err = c.Format("Hello, World!")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, `"Hello, World!"`, string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMETextPlain)
	err = c.Format(complex(1, 1))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "(1+1i)", string(c.Response().Body()))

	c.Request().Header.Set(HeaderAccept, MIMEApplicationXML)
	err = c.Format("Hello, World!")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, `<string>Hello, World!</string>`, string(c.Response().Body()))

	err = c.Format(complex(1, 1))
	utils.AssertEqual(t, true, err != nil)

	c.Request().Header.Set(HeaderAccept, MIMETextPlain)
	err = c.Format(Map{})
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "map[]", string(c.Response().Body()))

	type broken string
	c.Request().Header.Set(HeaderAccept, "broken/accept")
	err = c.Format(broken("Hello, World!"))
	utils.AssertEqual(t, nil, err)
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

	var err error
	for n := 0; n < b.N; n++ {
		err = c.Format("Hello, World!")
	}

	utils.AssertEqual(b, nil, err)
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

	var err error
	for n := 0; n < b.N; n++ {
		err = c.Format("Hello, World!")
	}

	utils.AssertEqual(b, nil, err)
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

	var err error
	for n := 0; n < b.N; n++ {
		err = c.Format("Hello, World!")
	}

	utils.AssertEqual(b, nil, err)
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

	var err error
	for n := 0; n < b.N; n++ {
		err = c.Format("Hello, World!")
	}

	utils.AssertEqual(b, nil, err)
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
		defer func() {
			utils.AssertEqual(t, nil, f.Close())
		}()

		b := new(bytes.Buffer)
		_, err = io.Copy(b, f)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "hello world", b.String())
		return nil
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	ioWriter, err := writer.CreateFormFile("file", "test")
	utils.AssertEqual(t, nil, err)

	_, err = ioWriter.Write([]byte("hello world"))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, nil, writer.Close())

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
	utils.AssertEqual(t, nil, writer.Close())

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

// go test -run Test_Ctx_IsProxyTrusted
func Test_Ctx_IsProxyTrusted(t *testing.T) {
	t.Parallel()

	{
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, true, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			EnableTrustedProxyCheck: false,
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, true, c.IsProxyTrusted())
	}

	{
		app := New(Config{
			EnableTrustedProxyCheck: true,
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, false, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			EnableTrustedProxyCheck: true,

			TrustedProxies: []string{},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, false, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			EnableTrustedProxyCheck: true,

			TrustedProxies: []string{
				"127.0.0.1",
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, false, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			EnableTrustedProxyCheck: true,

			TrustedProxies: []string{
				"127.0.0.1/8",
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, false, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			EnableTrustedProxyCheck: true,

			TrustedProxies: []string{
				"0.0.0.0",
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, true, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			EnableTrustedProxyCheck: true,

			TrustedProxies: []string{
				"0.0.0.1/31",
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, true, c.IsProxyTrusted())
	}
	{
		app := New(Config{
			EnableTrustedProxyCheck: true,

			TrustedProxies: []string{
				"0.0.0.1/31junk",
			},
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, false, c.IsProxyTrusted())
	}
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

// go test -run Test_Ctx_Hostname_Untrusted
func Test_Ctx_Hostname_UntrustedProxy(t *testing.T) {
	t.Parallel()
	// Don't trust any proxy
	{
		app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{}})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		utils.AssertEqual(t, "google.com", c.Hostname())
		app.ReleaseCtx(c)
	}
	// Trust to specific proxy list
	{
		app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.8.0.0", "0.8.0.1"}})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
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
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
		utils.AssertEqual(t, "google1.com", c.Hostname())
		app.ReleaseCtx(c)
	}
}

// go test -run Test_Ctx_Hostname_Trusted_Multiple
func Test_Ctx_Hostname_TrustedProxy_Multiple(t *testing.T) {
	t.Parallel()
	{
		app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0", "0.8.0.1"}})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().SetRequestURI("http://google.com/test")
		c.Request().Header.Set(HeaderXForwardedHost, "google1.com, google2.com")
		utils.AssertEqual(t, "google1.com", c.Hostname())
		app.ReleaseCtx(c)
	}
}

// go test -run Test_Ctx_Hostname_UntrustedProxyRange
func Test_Ctx_Hostname_TrustedProxyRange(t *testing.T) {
	t.Parallel()

	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0/30"}})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com/test")
	c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
	utils.AssertEqual(t, "google1.com", c.Hostname())
	app.ReleaseCtx(c)
}

// go test -run Test_Ctx_Hostname_UntrustedProxyRange
func Test_Ctx_Hostname_UntrustedProxyRange(t *testing.T) {
	t.Parallel()

	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"1.0.0.0/30"}})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI("http://google.com/test")
	c.Request().Header.Set(HeaderXForwardedHost, "google1.com")
	utils.AssertEqual(t, "google.com", c.Hostname())
	app.ReleaseCtx(c)
}

// go test -run Test_Ctx_Port
func Test_Ctx_Port(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	utils.AssertEqual(t, "0", c.Port())
}

// go test -run Test_Ctx_PortInHandler
func Test_Ctx_PortInHandler(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/port", func(c *Ctx) error {
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	// default behavior will return the remote IP from the stack
	utils.AssertEqual(t, "0.0.0.0", c.IP())

	// X-Forwarded-For is set, but it is ignored because proxyHeader is not set
	c.Request().Header.Set(HeaderXForwardedFor, "0.0.0.1")
	utils.AssertEqual(t, "0.0.0.0", c.IP())
}

// go test -run Test_Ctx_IP_ProxyHeader
func Test_Ctx_IP_ProxyHeader(t *testing.T) {
	t.Parallel()

	// make sure that the same behavior exists for different proxy header names
	proxyHeaderNames := []string{"Real-Ip", HeaderXForwardedFor}

	for _, proxyHeaderName := range proxyHeaderNames {
		app := New(Config{ProxyHeader: proxyHeaderName})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})

		c.Request().Header.Set(proxyHeaderName, "0.0.0.1")
		utils.AssertEqual(t, "0.0.0.1", c.IP())

		// without IP validation we return the full string
		c.Request().Header.Set(proxyHeaderName, "0.0.0.1, 0.0.0.2")
		utils.AssertEqual(t, "0.0.0.1, 0.0.0.2", c.IP())

		// without IP validation we return invalid IPs
		c.Request().Header.Set(proxyHeaderName, "invalid, 0.0.0.2, 0.0.0.3")
		utils.AssertEqual(t, "invalid, 0.0.0.2, 0.0.0.3", c.IP())

		// when proxy header is enabled but the value is empty, without IP validation we return an empty string
		c.Request().Header.Set(proxyHeaderName, "")
		utils.AssertEqual(t, "", c.IP())

		// without IP validation we return an invalid IP
		c.Request().Header.Set(proxyHeaderName, "not-valid-ip")
		utils.AssertEqual(t, "not-valid-ip", c.IP())

		app.ReleaseCtx(c)
	}
}

// go test -run Test_Ctx_IP_ProxyHeader
func Test_Ctx_IP_ProxyHeader_With_IP_Validation(t *testing.T) {
	t.Parallel()

	// make sure that the same behavior exists for different proxy header names
	proxyHeaderNames := []string{"Real-Ip", HeaderXForwardedFor}

	for _, proxyHeaderName := range proxyHeaderNames {
		app := New(Config{EnableIPValidation: true, ProxyHeader: proxyHeaderName})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})

		// when proxy header & validation is enabled and the value is a valid IP, we return it
		c.Request().Header.Set(proxyHeaderName, "0.0.0.1")
		utils.AssertEqual(t, "0.0.0.1", c.IP())

		// when proxy header & validation is enabled and the value is a list of IPs, we return the first valid IP
		c.Request().Header.Set(proxyHeaderName, "0.0.0.1, 0.0.0.2")
		utils.AssertEqual(t, "0.0.0.1", c.IP())

		c.Request().Header.Set(proxyHeaderName, "invalid, 0.0.0.2, 0.0.0.3")
		utils.AssertEqual(t, "0.0.0.2", c.IP())

		// when proxy header & validation is enabled but the value is empty, we will ignore the header
		c.Request().Header.Set(proxyHeaderName, "")
		utils.AssertEqual(t, "0.0.0.0", c.IP())

		// when proxy header & validation is enabled but the value is not an IP, we will ignore the header
		// and return the IP of the caller
		c.Request().Header.Set(proxyHeaderName, "not-valid-ip")
		utils.AssertEqual(t, "0.0.0.0", c.IP())

		app.ReleaseCtx(c)
	}
}

// go test -run Test_Ctx_IP_UntrustedProxy
func Test_Ctx_IP_UntrustedProxy(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.8.0.1"}, ProxyHeader: HeaderXForwardedFor})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderXForwardedFor, "0.0.0.1")
	defer app.ReleaseCtx(c)
	utils.AssertEqual(t, "0.0.0.0", c.IP())
}

// go test -run Test_Ctx_IP_TrustedProxy
func Test_Ctx_IP_TrustedProxy(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0"}, ProxyHeader: HeaderXForwardedFor})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(HeaderXForwardedFor, "0.0.0.1")
	defer app.ReleaseCtx(c)
	utils.AssertEqual(t, "0.0.0.1", c.IP())
}

// go test -run Test_Ctx_IPs  -parallel
func Test_Ctx_IPs(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	// normal happy path test case
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, 127.0.0.2, 127.0.0.3")
	utils.AssertEqual(t, []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}, c.IPs())

	// inconsistent space formatting
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1,127.0.0.2  ,127.0.0.3")
	utils.AssertEqual(t, []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}, c.IPs())

	// invalid IPs are allowed to be returned
	c.Request().Header.Set(HeaderXForwardedFor, "invalid, 127.0.0.1, 127.0.0.2")
	utils.AssertEqual(t, []string{"invalid", "127.0.0.1", "127.0.0.2"}, c.IPs())
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, invalid, 127.0.0.2")
	utils.AssertEqual(t, []string{"127.0.0.1", "invalid", "127.0.0.2"}, c.IPs())

	// ensure that the ordering of IPs in the header is maintained
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.3, 127.0.0.1, 127.0.0.2")
	utils.AssertEqual(t, []string{"127.0.0.3", "127.0.0.1", "127.0.0.2"}, c.IPs())

	// ensure for IPv6
	c.Request().Header.Set(HeaderXForwardedFor, "9396:9549:b4f7:8ed0:4791:1330:8c06:e62d, invalid, 2345:0425:2CA1::0567:5673:23b5")
	utils.AssertEqual(t, []string{"9396:9549:b4f7:8ed0:4791:1330:8c06:e62d", "invalid", "2345:0425:2CA1::0567:5673:23b5"}, c.IPs())

	// empty header
	c.Request().Header.Set(HeaderXForwardedFor, "")
	utils.AssertEqual(t, 0, len(c.IPs()))

	// missing header
	c.Request()
	utils.AssertEqual(t, 0, len(c.IPs()))
}

func Test_Ctx_IPs_With_IP_Validation(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableIPValidation: true})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	// normal happy path test case
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, 127.0.0.2, 127.0.0.3")
	utils.AssertEqual(t, []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}, c.IPs())

	// inconsistent space formatting
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1,127.0.0.2  ,127.0.0.3")
	utils.AssertEqual(t, []string{"127.0.0.1", "127.0.0.2", "127.0.0.3"}, c.IPs())

	// invalid IPs are in the header
	c.Request().Header.Set(HeaderXForwardedFor, "invalid, 127.0.0.1, 127.0.0.2")
	utils.AssertEqual(t, []string{"127.0.0.1", "127.0.0.2"}, c.IPs())
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, invalid, 127.0.0.2")
	utils.AssertEqual(t, []string{"127.0.0.1", "127.0.0.2"}, c.IPs())

	// ensure that the ordering of IPs in the header is maintained
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.3, 127.0.0.1, 127.0.0.2")
	utils.AssertEqual(t, []string{"127.0.0.3", "127.0.0.1", "127.0.0.2"}, c.IPs())

	// ensure for IPv6
	c.Request().Header.Set(HeaderXForwardedFor, "f037:825e:eadb:1b7b:1667:6f0a:5356:f604, invalid, 9396:9549:b4f7:8ed0:4791:1330:8c06:e62d")
	utils.AssertEqual(t, []string{"f037:825e:eadb:1b7b:1667:6f0a:5356:f604", "9396:9549:b4f7:8ed0:4791:1330:8c06:e62d"}, c.IPs())

	// empty header
	c.Request().Header.Set(HeaderXForwardedFor, "")
	utils.AssertEqual(t, 0, len(c.IPs()))

	// missing header
	c.Request()
	utils.AssertEqual(t, 0, len(c.IPs()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_IPs -benchmem -count=4
func Benchmark_Ctx_IPs(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, invalid, 127.0.0.1")
	var res []string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IPs()
	}
	utils.AssertEqual(b, []string{"127.0.0.1", "invalid", "127.0.0.1"}, res)
}

func Benchmark_Ctx_IPs_v6(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderXForwardedFor, "f037:825e:eadb:1b7b:1667:6f0a:5356:f604, invalid, 2345:0425:2CA1::0567:5673:23b5")
	var res []string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IPs()
	}
	utils.AssertEqual(b, []string{"f037:825e:eadb:1b7b:1667:6f0a:5356:f604", "invalid", "2345:0425:2CA1::0567:5673:23b5"}, res)
}

func Benchmark_Ctx_IPs_With_IP_Validation(b *testing.B) {
	app := New(Config{EnableIPValidation: true})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1, invalid, 127.0.0.1")
	var res []string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IPs()
	}
	utils.AssertEqual(b, []string{"127.0.0.1", "127.0.0.1"}, res)
}

func Benchmark_Ctx_IPs_v6_With_IP_Validation(b *testing.B) {
	app := New(Config{EnableIPValidation: true})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderXForwardedFor, "2345:0425:2CA1:0000:0000:0567:5673:23b5, invalid, 2345:0425:2CA1::0567:5673:23b5")
	var res []string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IPs()
	}
	utils.AssertEqual(b, []string{"2345:0425:2CA1:0000:0000:0567:5673:23b5", "2345:0425:2CA1::0567:5673:23b5"}, res)
}

func Benchmark_Ctx_IP_With_ProxyHeader(b *testing.B) {
	app := New(Config{ProxyHeader: HeaderXForwardedFor})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IP()
	}
	utils.AssertEqual(b, "127.0.0.1", res)
}

func Benchmark_Ctx_IP_With_ProxyHeader_and_IP_Validation(b *testing.B) {
	app := New(Config{ProxyHeader: HeaderXForwardedFor, EnableIPValidation: true})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1")
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IP()
	}
	utils.AssertEqual(b, "127.0.0.1", res)
}

func Benchmark_Ctx_IP(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request()
	var res string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.IP()
	}
	utils.AssertEqual(b, "0.0.0.0", res)
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
	t.Parallel()
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

// go test -run Test_Ctx_ClientHelloInfo
func Test_Ctx_ClientHelloInfo(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/ServerName", func(c *Ctx) error {
		result := c.ClientHelloInfo()
		if result != nil {
			return c.SendString(result.ServerName)
		}

		return c.SendString("ClientHelloInfo is nil")
	})
	app.Get("/SignatureSchemes", func(c *Ctx) error {
		result := c.ClientHelloInfo()
		if result != nil {
			return c.JSON(result.SignatureSchemes)
		}

		return c.SendString("ClientHelloInfo is nil")
	})
	app.Get("/SupportedVersions", func(c *Ctx) error {
		result := c.ClientHelloInfo()
		if result != nil {
			return c.JSON(result.SupportedVersions)
		}

		return c.SendString("ClientHelloInfo is nil")
	})

	// Test without TLS handler
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/ServerName", nil))
	utils.AssertEqual(t, nil, err)
	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, []byte("ClientHelloInfo is nil"), body)

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
	utils.AssertEqual(t, nil, err)
	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, []byte("example.golang"), body)

	// Test SignatureSchemes
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/SignatureSchemes", nil))
	utils.AssertEqual(t, nil, err)
	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "["+strconv.Itoa(pssWithSHA256)+"]", string(body))

	// Test SupportedVersions
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/SupportedVersions", nil))
	utils.AssertEqual(t, nil, err)
	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "["+strconv.Itoa(versionTLS13)+"]", string(body))
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
	utils.AssertEqual(t, nil, writer.Close())

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
	app.Get("/test5/:id/:Id", func(c *Ctx) error {
		utils.AssertEqual(t, "first", c.Params("id"))
		utils.AssertEqual(t, "first", c.Params("Id"))
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

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test5/first/second", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

func Test_Ctx_Params_Case_Sensitive(t *testing.T) {
	t.Parallel()
	app := New(Config{CaseSensitive: true})
	app.Get("/test/:User", func(c *Ctx) error {
		utils.AssertEqual(t, "john", c.Params("User"))
		utils.AssertEqual(t, "", c.Params("user"))
		return nil
	})
	app.Get("/test2/:id/:Id", func(c *Ctx) error {
		utils.AssertEqual(t, "first", c.Params("id"))
		utils.AssertEqual(t, "second", c.Params("Id"))
		return nil
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test/john", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test2/first/second", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -race -run Test_Ctx_AllParams
func Test_Ctx_AllParams(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test/:user", func(c *Ctx) error {
		utils.AssertEqual(t, map[string]string{"user": "john"}, c.AllParams())
		return nil
	})
	app.Get("/test2/*", func(c *Ctx) error {
		utils.AssertEqual(t, map[string]string{"*1": "im/a/cookie"}, c.AllParams())
		return nil
	})
	app.Get("/test3/*/blafasel/*", func(c *Ctx) error {
		utils.AssertEqual(t, map[string]string{"*1": "1111", "*2": "2222"}, c.AllParams())
		return nil
	})
	app.Get("/test4/:optional?", func(c *Ctx) error {
		utils.AssertEqual(t, map[string]string{"optional": ""}, c.AllParams())
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

// go test -v -run=^$ -bench=Benchmark_Ctx_AllParams -benchmem -count=4
func Benchmark_Ctx_AllParams(b *testing.B) {
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
	var res map[string]string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res = c.AllParams()
	}
	utils.AssertEqual(
		b,
		map[string]string{
			"param1": "john",
			"param2": "doe",
			"param3": "is",
			"param4": "awesome",
		},
		res,
	)
}

// go test -v -run=^$ -bench=Benchmark_Ctx_ParamsParse -benchmem -count=4
func Benchmark_Ctx_ParamsParse(b *testing.B) {
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
	var res struct {
		Param1 string `params:"param1"`
		Param2 string `params:"param2"`
		Param3 string `params:"param3"`
		Param4 string `params:"param4"`
	}
	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		err = c.ParamsParser(&res)
	}

	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, "john", res.Param1)
	utils.AssertEqual(b, "doe", res.Param2)
	utils.AssertEqual(b, "is", res.Param3)
	utils.AssertEqual(b, "awesome", res.Param4)
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
	t.Parallel()
	app := New()

	freq := &fasthttp.RequestCtx{}
	freq.Request.Header.Set("X-Forwarded", "invalid")

	c := app.AcquireCtx(freq)
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderXForwardedProto, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTPS, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTPS, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProto, "https, http")
	utils.AssertEqual(t, schemeHTTPS, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, "https, http")
	utils.AssertEqual(t, schemeHTTPS, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	utils.AssertEqual(t, schemeHTTPS, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTPS, c.Protocol())
	c.Request().Header.Reset()

	utils.AssertEqual(t, schemeHTTP, c.Protocol())
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
	utils.AssertEqual(b, schemeHTTP, res)
}

// go test -run Test_Ctx_Protocol_TrustedProxy
func Test_Ctx_Protocol_TrustedProxy(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0"}})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	c.Request().Header.Set(HeaderXForwardedProto, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTPS, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTPS, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	utils.AssertEqual(t, schemeHTTPS, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTPS, c.Protocol())
	c.Request().Header.Reset()

	utils.AssertEqual(t, schemeHTTP, c.Protocol())
}

// go test -run Test_Ctx_Protocol_TrustedProxyRange
func Test_Ctx_Protocol_TrustedProxyRange(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.0.0.0/30"}})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	c.Request().Header.Set(HeaderXForwardedProto, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTPS, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTPS, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	utils.AssertEqual(t, schemeHTTPS, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTPS, c.Protocol())
	c.Request().Header.Reset()

	utils.AssertEqual(t, schemeHTTP, c.Protocol())
}

// go test -run Test_Ctx_Protocol_UntrustedProxyRange
func Test_Ctx_Protocol_UntrustedProxyRange(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"1.1.1.1/30"}})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	c.Request().Header.Set(HeaderXForwardedProto, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTP, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTP, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	utils.AssertEqual(t, schemeHTTP, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTP, c.Protocol())
	c.Request().Header.Reset()

	utils.AssertEqual(t, schemeHTTP, c.Protocol())
}

// go test -run Test_Ctx_Protocol_UnTrustedProxy
func Test_Ctx_Protocol_UnTrustedProxy(t *testing.T) {
	t.Parallel()
	app := New(Config{EnableTrustedProxyCheck: true, TrustedProxies: []string{"0.8.0.1"}})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	c.Request().Header.Set(HeaderXForwardedProto, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTP, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedProtocol, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTP, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXForwardedSsl, "on")
	utils.AssertEqual(t, schemeHTTP, c.Protocol())
	c.Request().Header.Reset()

	c.Request().Header.Set(HeaderXUrlScheme, schemeHTTPS)
	utils.AssertEqual(t, schemeHTTP, c.Protocol())
	c.Request().Header.Reset()

	utils.AssertEqual(t, schemeHTTP, c.Protocol())
}

// go test -run Test_Ctx_Query
func Test_Ctx_Query(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().URI().SetQueryString("search=john&age=20&id=")
	utils.AssertEqual(t, "john", c.Query("search"))
	utils.AssertEqual(t, "20", c.Query("age"))
	utils.AssertEqual(t, "default", c.Query("unknown", "default"))
}

func Test_Ctx_QueryInt(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().URI().SetQueryString("search=john&age=20&id=")

	utils.AssertEqual(t, 0, c.QueryInt("foo"))
	utils.AssertEqual(t, 20, c.QueryInt("age", 12))
	utils.AssertEqual(t, 0, c.QueryInt("search"))
	utils.AssertEqual(t, 1, c.QueryInt("search", 1))
	utils.AssertEqual(t, 0, c.QueryInt("id"))
	utils.AssertEqual(t, 2, c.QueryInt("id", 2))
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

		tempFile, err := os.CreateTemp(os.TempDir(), "test-")
		utils.AssertEqual(t, nil, err)

		defer func(file *os.File) {
			err := file.Close()
			utils.AssertEqual(t, nil, err)
			err = os.Remove(file.Name())
			utils.AssertEqual(t, nil, err)
		}(tempFile)
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
	utils.AssertEqual(t, nil, writer.Close())

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

	app.Post("/test", func(c *Ctx) error {
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
	utils.AssertEqual(t, nil, writer.Close())

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

	utils.AssertEqual(t, nil, c.Download("ctx.go", "Awesome File!"))

	f, err := os.Open("./ctx.go")
	utils.AssertEqual(t, nil, err)
	defer func() {
		utils.AssertEqual(t, nil, f.Close())
	}()

	expect, err := io.ReadAll(f)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expect, c.Response().Body())
	utils.AssertEqual(t, `attachment; filename="Awesome+File%21"`, string(c.Response().Header.Peek(HeaderContentDisposition)))

	utils.AssertEqual(t, nil, c.Download("ctx.go"))
	utils.AssertEqual(t, `attachment; filename="ctx.go"`, string(c.Response().Header.Peek(HeaderContentDisposition)))
}

// go test -race -run Test_Ctx_SendFile
func Test_Ctx_SendFile(t *testing.T) {
	t.Parallel()
	app := New()

	// fetch file content
	f, err := os.Open("./ctx.go")
	utils.AssertEqual(t, nil, err)
	defer func() {
		utils.AssertEqual(t, nil, f.Close())
	}()
	expectFileContent, err := io.ReadAll(f)
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
		err := c.SendFile(filepath.FromSlash("john_dow.go/"))
		utils.AssertEqual(t, false, err == nil)
		return err
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
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
		app.Get(endpoint, func(c *Ctx) error {
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
			resp, err := app.Test(httptest.NewRequest(MethodGet, endpoint, nil))
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, StatusOK, resp.StatusCode)
			// 2nd try
			resp, err = app.Test(httptest.NewRequest(MethodGet, endpoint, nil))
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, StatusOK, resp.StatusCode)
		})
	}
}

// go test -race -run Test_Ctx_SendFile_RestoreOriginalURL
func Test_Ctx_SendFile_RestoreOriginalURL(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c *Ctx) error {
		originalURL := utils.CopyString(c.OriginalURL())
		err := c.SendFile("ctx.go")
		utils.AssertEqual(t, originalURL, c.OriginalURL())
		return err
	})

	_, err1 := app.Test(httptest.NewRequest(MethodGet, "/?test=true", nil))
	// second request required to confirm with zero allocation
	_, err2 := app.Test(httptest.NewRequest(MethodGet, "/?test=true", nil))

	utils.AssertEqual(t, nil, err1)
	utils.AssertEqual(t, nil, err2)
}

// go test -run Test_Ctx_JSON
func Test_Ctx_JSON(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	utils.AssertEqual(t, true, c.JSON(complex(1, 1)) != nil)

	err := c.JSON(Map{ // map has no order
		"Name": "Grame",
		"Age":  20,
	})
	utils.AssertEqual(t, nil, err)
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

	err := c.JSONP(Map{
		"Name": "Grame",
		"Age":  20,
	})
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, `callback({"Age":20,"Name":"Grame"});`, string(c.Response().Body()))
	utils.AssertEqual(t, "text/javascript; charset=utf-8", string(c.Response().Header.Peek("content-type")))

	err = c.JSONP(Map{
		"Name": "Grame",
		"Age":  20,
	}, "john")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, `john({"Age":20,"Name":"Grame"});`, string(c.Response().Body()))
	utils.AssertEqual(t, "text/javascript; charset=utf-8", string(c.Response().Header.Peek("content-type")))
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

// go test -run Test_Ctx_XML
func Test_Ctx_XML(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	utils.AssertEqual(t, true, c.JSON(complex(1, 1)) != nil)

	type xmlResult struct {
		XMLName xml.Name `xml:"Users"`
		Names   []string `xml:"Names"`
		Ages    []int    `xml:"Ages"`
	}

	err := c.XML(xmlResult{
		Names: []string{"Grame", "John"},
		Ages:  []int{1, 12, 20},
	})
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, `<Users><Names>Grame</Names><Names>John</Names><Ages>1</Ages><Ages>12</Ages><Ages>20</Ages></Users>`, string(c.Response().Body()))
	utils.AssertEqual(t, "application/xml", string(c.Response().Header.Peek("content-type")))

	testEmpty := func(v interface{}, r string) {
		err := c.XML(v)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, r, string(c.Response().Body()))
	}

	testEmpty(nil, "")
	testEmpty("", `<string></string>`)
	testEmpty(0, "<int>0</int>")
	testEmpty([]int{}, "")
}

// go test -run=^$ -bench=Benchmark_Ctx_XML -benchmem -count=4
func Benchmark_Ctx_XML(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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

	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, `<SomeStruct><Name>Grame</Name><Age>20</Age></SomeStruct>`, string(c.Response().Body()))
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
	t.Parallel()
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
	t.Parallel()
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

	err := c.Redirect("http://default.com")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "http://default.com", string(c.Response().Header.Peek(HeaderLocation)))

	err = c.Redirect("http://example.com", 301)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 301, c.Response().StatusCode())
	utils.AssertEqual(t, "http://example.com", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_RedirectToRouteWithParams
func Test_Ctx_RedirectToRouteWithParams(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name", func(c *Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	err := c.RedirectToRoute("user", Map{
		"name": "fiber",
	})
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_RedirectToRouteWithParams
func Test_Ctx_RedirectToRouteWithQueries(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name", func(c *Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	err := c.RedirectToRoute("user", Map{
		"name":    "fiber",
		"queries": map[string]string{"data[0][name]": "john", "data[0][age]": "10", "test": "doe"},
	})
	utils.AssertEqual(t, nil, err)
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
	app.Get("/user/:name?", func(c *Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	err := c.RedirectToRoute("user", Map{
		"name": "fiber",
	})
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_RedirectToRouteWithOptionalParamsWithoutValue
func Test_Ctx_RedirectToRouteWithOptionalParamsWithoutValue(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name?", func(c *Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	err := c.RedirectToRoute("user", Map{})
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/user/", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_RedirectToRouteWithGreedyParameters
func Test_Ctx_RedirectToRouteWithGreedyParameters(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/+", func(c *Ctx) error {
		return c.JSON(c.Params("+"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	err := c.RedirectToRoute("user", Map{
		"+": "test/routes",
	})
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/user/test/routes", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_RedirectBack
func Test_Ctx_RedirectBack(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c *Ctx) error {
		return c.JSON("Home")
	}).Name("home")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	err := c.RedirectBack("/")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_RedirectBackWithReferer
func Test_Ctx_RedirectBackWithReferer(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c *Ctx) error {
		return c.JSON("Home")
	}).Name("home")
	app.Get("/back", func(c *Ctx) error {
		return c.JSON("Back")
	}).Name("back")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.Set(HeaderReferer, "/back")
	err := c.RedirectBack("/")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/back", c.Get(HeaderReferer))
	utils.AssertEqual(t, "/back", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Ctx_Render
func Test_Ctx_Render(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	err := c.Render("./.github/testdata/index.tmpl", Map{
		"Title": "Hello, World!",
	})
	utils.AssertEqual(t, nil, err)

	buf := bytebufferpool.Get()
	_, _ = buf.WriteString("overwrite") //nolint:errcheck // This will never fail
	defer bytebufferpool.Put(buf)

	utils.AssertEqual(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))

	err = c.Render("./.github/testdata/template-non-exists.html", nil)
	utils.AssertEqual(t, false, err == nil)

	err = c.Render("./.github/testdata/template-invalid.html", nil)
	utils.AssertEqual(t, false, err == nil)
}

func Test_Ctx_RenderWithoutLocals(t *testing.T) {
	t.Parallel()
	app := New(Config{
		PassLocalsToViews: false,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Locals("Title", "Hello, World!")
	defer app.ReleaseCtx(c)
	err := c.Render("./.github/testdata/index.tmpl", Map{})
	utils.AssertEqual(t, nil, err)

	buf := bytebufferpool.Get()
	_, _ = buf.WriteString("overwrite") //nolint:errcheck // This will never fail
	defer bytebufferpool.Put(buf)

	utils.AssertEqual(t, "<h1><no value></h1>", string(c.Response().Body()))
}

func Test_Ctx_RenderWithLocals(t *testing.T) {
	t.Parallel()
	app := New(Config{
		PassLocalsToViews: true,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Locals("Title", "Hello, World!")
	defer app.ReleaseCtx(c)
	err := c.Render("./.github/testdata/index.tmpl", Map{})
	utils.AssertEqual(t, nil, err)

	buf := bytebufferpool.Get()
	_, _ = buf.WriteString("overwrite") //nolint:errcheck // This will never fail
	defer bytebufferpool.Put(buf)

	utils.AssertEqual(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

func Test_Ctx_RenderWithBind(t *testing.T) {
	t.Parallel()

	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Bind(Map{
		"Title": "Hello, World!",
	})
	utils.AssertEqual(t, nil, err)
	defer app.ReleaseCtx(c)
	err = c.Render("./.github/testdata/index.tmpl", Map{})
	utils.AssertEqual(t, nil, err)

	buf := bytebufferpool.Get()
	_, _ = buf.WriteString("overwrite") //nolint:errcheck // This will never fail
	defer bytebufferpool.Put(buf)

	utils.AssertEqual(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

func Test_Ctx_RenderWithOverwrittenBind(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Bind(Map{
		"Title": "Hello, World!",
	})
	utils.AssertEqual(t, nil, err)
	defer app.ReleaseCtx(c)
	err = c.Render("./.github/testdata/index.tmpl", Map{
		"Title": "Hello from Fiber!",
	})
	utils.AssertEqual(t, nil, err)

	buf := bytebufferpool.Get()
	_, _ = buf.WriteString("overwrite") //nolint:errcheck // This will never fail
	defer bytebufferpool.Put(buf)

	utils.AssertEqual(t, "<h1>Hello from Fiber!</h1>", string(c.Response().Body()))
}

func Test_Ctx_RenderWithBindLocals(t *testing.T) {
	t.Parallel()
	app := New(Config{
		PassLocalsToViews: true,
	})

	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Bind(Map{
		"Title": "Hello, World!",
	})
	utils.AssertEqual(t, nil, err)

	c.Locals("Summary", "Test")
	defer app.ReleaseCtx(c)

	err = c.Render("./.github/testdata/template.tmpl", Map{})
	utils.AssertEqual(t, nil, err)

	utils.AssertEqual(t, "<h1>Hello, World! Test</h1>", string(c.Response().Body()))
}

func Test_Ctx_RenderWithLocalsAndBinding(t *testing.T) {
	t.Parallel()
	engine := &testTemplateEngine{}
	err := engine.Load()
	utils.AssertEqual(t, nil, err)
	app := New(Config{
		PassLocalsToViews: true,
		Views:             engine,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Locals("Title", "This is a test.")
	defer app.ReleaseCtx(c)

	err = c.Render("index.tmpl", Map{
		"Title": "Hello, World!",
	})
	utils.AssertEqual(t, nil, err)

	utils.AssertEqual(t, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

func Benchmark_Ctx_RenderWithLocalsAndBinding(b *testing.B) {
	engine := &testTemplateEngine{}
	err := engine.Load()
	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, nil, err)
	app := New(Config{
		PassLocalsToViews: true,
		Views:             engine,
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err = c.Bind(Map{
		"Title": "Hello, World!",
	})
	utils.AssertEqual(b, nil, err)
	c.Locals("Summary", "Test")

	defer app.ReleaseCtx(c)

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
	app.Get("/user/:name", func(c *Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		err = c.RedirectToRoute("user", Map{
			"name": "fiber",
		})
	}
	utils.AssertEqual(b, nil, err)

	utils.AssertEqual(b, 302, c.Response().StatusCode())
	utils.AssertEqual(b, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

func Benchmark_Ctx_RedirectToRouteWithQueries(b *testing.B) {
	app := New()
	app.Get("/user/:name", func(c *Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		err = c.RedirectToRoute("user", Map{
			"name":    "fiber",
			"queries": map[string]string{"a": "a", "b": "b"},
		})
	}
	utils.AssertEqual(b, nil, err)

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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Locals("Title", "Hello, World!")

	defer app.ReleaseCtx(c)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		err = c.Render("index.tmpl", Map{})
	}
	utils.AssertEqual(b, nil, err)

	utils.AssertEqual(b, "<h1>Hello, World!</h1>", string(c.Response().Body()))
}

func Benchmark_Ctx_RenderBind(b *testing.B) {
	engine := &testTemplateEngine{}
	err := engine.Load()
	utils.AssertEqual(b, nil, err)
	app := New()
	app.config.Views = engine
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err = c.Bind(Map{
		"Title": "Hello, World!",
	})
	utils.AssertEqual(b, nil, err)

	defer app.ReleaseCtx(c)

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
	t.Parallel()
	app := New()
	calls := 0
	app.Get("/", func(c *Ctx) error {
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
	t.Parallel()
	app := New()
	var executedOldHandler, executedNewHandler bool

	app.Get("/old", func(c *Ctx) error {
		c.Path("/new")
		return c.RestartRouting()
	})
	app.Get("/old", func(c *Ctx) error {
		executedOldHandler = true
		return nil
	})
	app.Get("/new", func(c *Ctx) error {
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
	t.Parallel()
	app := New()
	app.Get("/new", func(c *Ctx) error {
		return nil
	})
	app.Use(func(c *Ctx) error {
		c.Path("/new")
		// c.Next() would fail this test as a 404 is returned from the next handler
		return c.RestartRouting()
	})
	app.Use(func(c *Ctx) error {
		return ErrNotFound
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "http://example.com/old", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

type testTemplateEngine struct {
	templates *template.Template
	path      string
}

func (t *testTemplateEngine) Render(w io.Writer, name string, bind interface{}, layout ...string) error {
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
	utils.AssertEqual(t, nil, engine.Load())
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

// go test -run Test_Ctx_Render_Engine_With_View_Layout
func Test_Ctx_Render_Engine_With_View_Layout(t *testing.T) {
	t.Parallel()
	engine := &testTemplateEngine{}
	utils.AssertEqual(t, nil, engine.Load())
	app := New(Config{ViewsLayout: "main.tmpl"})
	app.config.Views = engine
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
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

// go test -v -run=^$ -bench=Benchmark_Ctx_Get_Location_From_Route -benchmem -count=4
func Benchmark_Ctx_Get_Location_From_Route(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	app.Get("/user/:name", func(c *Ctx) error {
		return c.SendString(c.Params("name"))
	}).Name("User")

	var err error
	var location string
	for n := 0; n < b.N; n++ {
		location, err = c.getLocationFromRoute(app.GetRoute("User"), Map{"name": "fiber"})
	}
	utils.AssertEqual(b, "/user/fiber", location)
	utils.AssertEqual(b, nil, err)
}

// go test -run Test_Ctx_Get_Location_From_Route_name
func Test_Ctx_Get_Location_From_Route_name(t *testing.T) {
	t.Parallel()

	t.Run("case insensitive", func(t *testing.T) {
		t.Parallel()
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		app.Get("/user/:name", func(c *Ctx) error {
			return c.SendString(c.Params("name"))
		}).Name("User")

		location, err := c.GetRouteURL("User", Map{"name": "fiber"})
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "/user/fiber", location)

		location, err = c.GetRouteURL("User", Map{"Name": "fiber"})
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "/user/fiber", location)
	})

	t.Run("case sensitive", func(t *testing.T) {
		t.Parallel()
		app := New(Config{CaseSensitive: true})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		app.Get("/user/:name", func(c *Ctx) error {
			return c.SendString(c.Params("name"))
		}).Name("User")

		location, err := c.GetRouteURL("User", Map{"name": "fiber"})
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "/user/fiber", location)

		location, err = c.GetRouteURL("User", Map{"Name": "fiber"})
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "/user/", location)
	})
}

// go test -run Test_Ctx_Get_Location_From_Route_name_Optional_greedy
func Test_Ctx_Get_Location_From_Route_name_Optional_greedy(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	app.Get("/:phone/*/send/*", func(c *Ctx) error {
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
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	app.Get("/:phone/*/send", func(c *Ctx) error {
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

func (errorTemplateEngine) Render(_ io.Writer, _ string, _ interface{}, _ ...string) error {
	return errors.New("errorTemplateEngine")
}

func (errorTemplateEngine) Load() error { return nil }

// go test -run Test_Ctx_Render_Engine_Error
func Test_Ctx_Render_Engine_Error(t *testing.T) {
	t.Parallel()
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
	file, err := os.CreateTemp(os.TempDir(), "fiber")
	utils.AssertEqual(t, nil, err)
	defer func() {
		err := os.Remove(file.Name())
		utils.AssertEqual(t, nil, err)
	}()

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
	utils.AssertEqual(t, nil, c.Send([]byte("Hello, World")))
	utils.AssertEqual(t, nil, c.Send([]byte("Don't crash please")))
	utils.AssertEqual(t, nil, c.Send([]byte("1337")))
	utils.AssertEqual(t, "1337", string(c.Response().Body()))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_Send -benchmem -count=4
func Benchmark_Ctx_Send(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	byt := []byte("Hello, World!")
	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		err = c.Send(byt)
	}
	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, "Hello, World!", string(c.Response().Body()))
}

// go test -run Test_Ctx_SendStatus
func Test_Ctx_SendStatus(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	err := c.SendStatus(415)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 415, c.Response().StatusCode())
	utils.AssertEqual(t, "Unsupported Media Type", string(c.Response().Body()))
}

// go test -run Test_Ctx_SendString
func Test_Ctx_SendString(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	err := c.SendString("Don't crash please")
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "Don't crash please", string(c.Response().Body()))
}

// go test -run Test_Ctx_SendStream
func Test_Ctx_SendStream(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	err := c.SendStream(bytes.NewReader([]byte("Don't crash please")))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "Don't crash please", string(c.Response().Body()))

	err = c.SendStream(bytes.NewReader([]byte("Don't crash please")), len([]byte("Don't crash please")))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "Don't crash please", string(c.Response().Body()))

	err = c.SendStream(bufio.NewReader(bytes.NewReader([]byte("Hello bufio"))))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "Hello bufio", string(c.Response().Body()))
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
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Status(400)
	utils.AssertEqual(t, 400, c.Response().StatusCode())
	err := c.Status(415).Send([]byte("Hello, World"))
	utils.AssertEqual(t, nil, err)
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
	_, err := c.Write([]byte("Hello, "))
	utils.AssertEqual(t, nil, err)
	_, err = c.Write([]byte("World!"))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "Hello, World!", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Write -benchmem -count=4
func Benchmark_Ctx_Write(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	byt := []byte("Hello, World!")
	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		_, err = c.Write(byt)
	}
	utils.AssertEqual(b, nil, err)
}

// go test -run Test_Ctx_Writef
func Test_Ctx_Writef(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	world := "World!"
	_, err := c.Writef("Hello, %s", world)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "Hello, World!", string(c.Response().Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_Writef -benchmem -count=4
func Benchmark_Ctx_Writef(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	world := "World!"
	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		_, err = c.Writef("Hello, %s", world)
	}
	utils.AssertEqual(b, nil, err)
}

// go test -run Test_Ctx_WriteString
func Test_Ctx_WriteString(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	_, err := c.WriteString("Hello, ")
	utils.AssertEqual(t, nil, err)
	_, err = c.WriteString("World!")
	utils.AssertEqual(t, nil, err)
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

	var err error
	for n := 0; n < b.N; n++ {
		err = c.SendString(body)
	}
	utils.AssertEqual(b, nil, err)
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
		Bool            bool
		ID              int
		Name            string
		Hobby           string
		FavouriteDrinks []string
		Empty           []string
		Alloc           []string
		No              []int64
	}

	c.Request().URI().SetQueryString("id=1&name=tom&hobby=basketball,football&favouriteDrinks=milo,coke,pepsi&alloc=&no=1")
	q2 := new(Query2)
	q2.Bool = true
	q2.Name = "hello world"
	utils.AssertEqual(t, nil, c.QueryParser(q2))
	utils.AssertEqual(t, "basketball,football", q2.Hobby)
	utils.AssertEqual(t, true, q2.Bool)
	utils.AssertEqual(t, "tom", q2.Name) // check value get overwritten
	utils.AssertEqual(t, []string{"milo", "coke", "pepsi"}, q2.FavouriteDrinks)
	var nilSlice []string
	utils.AssertEqual(t, nilSlice, q2.Empty)
	utils.AssertEqual(t, []string{""}, q2.Alloc)
	utils.AssertEqual(t, []int64{1}, q2.No)

	type RequiredQuery struct {
		Name string `query:"name,required"`
	}
	rq := new(RequiredQuery)
	c.Request().URI().SetQueryString("")
	utils.AssertEqual(t, "failed to decode: name is empty", c.QueryParser(rq).Error())

	type ArrayQuery struct {
		Data []string
	}
	aq := new(ArrayQuery)
	c.Request().URI().SetQueryString("data[]=john&data[]=doe")
	utils.AssertEqual(t, nil, c.QueryParser(aq))
	utils.AssertEqual(t, 2, len(aq.Data))
}

// go test -run Test_Ctx_QueryParser_WithSetParserDecoder -v
func Test_Ctx_QueryParser_WithSetParserDecoder(t *testing.T) {
	t.Parallel()
	type NonRFCTime time.Time

	nonRFCConverter := func(value string) reflect.Value {
		if v, err := time.Parse("2006-01-02", value); err == nil {
			return reflect.ValueOf(v)
		}
		return reflect.Value{}
	}

	nonRFCTime := ParserType{
		Customtype: NonRFCTime{},
		Converter:  nonRFCConverter,
	}

	SetParserDecoder(ParserConfig{
		IgnoreUnknownKeys: true,
		ParserType:        []ParserType{nonRFCTime},
		ZeroEmpty:         true,
		SetAliasTag:       "query",
	})

	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	type NonRFCTimeInput struct {
		Date  NonRFCTime `query:"date"`
		Title string     `query:"title"`
		Body  string     `query:"body"`
	}

	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	q := new(NonRFCTimeInput)

	c.Request().URI().SetQueryString("date=2021-04-10&title=CustomDateTest&Body=October")
	utils.AssertEqual(t, nil, c.QueryParser(q))
	utils.AssertEqual(t, "CustomDateTest", q.Title)
	date := fmt.Sprintf("%v", q.Date)
	utils.AssertEqual(t, "{0 63753609600 <nil>}", date)
	utils.AssertEqual(t, "October", q.Body)

	c.Request().URI().SetQueryString("date=2021-04-10&title&Body=October")
	q = &NonRFCTimeInput{
		Title: "Existing title",
		Body:  "Existing Body",
	}
	utils.AssertEqual(t, nil, c.QueryParser(q))
	utils.AssertEqual(t, "", q.Title)
}

// go test -run Test_Ctx_QueryParser_Schema -v
func Test_Ctx_QueryParser_Schema(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	type Query1 struct {
		Name   string `query:"name,required"`
		Nested struct {
			Age int `query:"age"`
		} `query:"nested,required"`
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString("name=tom&nested.age=10")
	q := new(Query1)
	utils.AssertEqual(t, nil, c.QueryParser(q))

	c.Request().URI().SetQueryString("namex=tom&nested.age=10")
	q = new(Query1)
	utils.AssertEqual(t, "failed to decode: name is empty", c.QueryParser(q).Error())

	c.Request().URI().SetQueryString("name=tom&nested.agex=10")
	q = new(Query1)
	utils.AssertEqual(t, nil, c.QueryParser(q))

	c.Request().URI().SetQueryString("name=tom&test.age=10")
	q = new(Query1)
	utils.AssertEqual(t, "failed to decode: nested is empty", c.QueryParser(q).Error())

	type Query2 struct {
		Name   string `query:"name"`
		Nested struct {
			Age int `query:"age,required"`
		} `query:"nested"`
	}
	c.Request().URI().SetQueryString("name=tom&nested.age=10")
	q2 := new(Query2)
	utils.AssertEqual(t, nil, c.QueryParser(q2))

	c.Request().URI().SetQueryString("nested.age=10")
	q2 = new(Query2)
	utils.AssertEqual(t, nil, c.QueryParser(q2))

	c.Request().URI().SetQueryString("nested.agex=10")
	q2 = new(Query2)
	utils.AssertEqual(t, "failed to decode: nested.age is empty", c.QueryParser(q2).Error())

	c.Request().URI().SetQueryString("nested.agex=10")
	q2 = new(Query2)
	utils.AssertEqual(t, "failed to decode: nested.age is empty", c.QueryParser(q2).Error())

	type Node struct {
		Value int   `query:"val,required"`
		Next  *Node `query:"next,required"`
	}
	c.Request().URI().SetQueryString("val=1&next.val=3")
	n := new(Node)
	utils.AssertEqual(t, nil, c.QueryParser(n))
	utils.AssertEqual(t, 1, n.Value)
	utils.AssertEqual(t, 3, n.Next.Value)

	c.Request().URI().SetQueryString("next.val=2")
	n = new(Node)
	utils.AssertEqual(t, "failed to decode: val is empty", c.QueryParser(n).Error())

	c.Request().URI().SetQueryString("val=3&next.value=2")
	n = new(Node)
	n.Next = new(Node)
	utils.AssertEqual(t, nil, c.QueryParser(n))
	utils.AssertEqual(t, 3, n.Value)
	utils.AssertEqual(t, 0, n.Next.Value)

	type Person struct {
		Name string `query:"name"`
		Age  int    `query:"age"`
	}

	type CollectionQuery struct {
		Data []Person `query:"data"`
	}

	c.Request().URI().SetQueryString("data[0][name]=john&data[0][age]=10&data[1][name]=doe&data[1][age]=12")
	cq := new(CollectionQuery)
	utils.AssertEqual(t, nil, c.QueryParser(cq))
	utils.AssertEqual(t, 2, len(cq.Data))
	utils.AssertEqual(t, "john", cq.Data[0].Name)
	utils.AssertEqual(t, 10, cq.Data[0].Age)
	utils.AssertEqual(t, "doe", cq.Data[1].Name)
	utils.AssertEqual(t, 12, cq.Data[1].Age)

	c.Request().URI().SetQueryString("data.0.name=john&data.0.age=10&data.1.name=doe&data.1.age=12")
	cq = new(CollectionQuery)
	utils.AssertEqual(t, nil, c.QueryParser(cq))
	utils.AssertEqual(t, 2, len(cq.Data))
	utils.AssertEqual(t, "john", cq.Data[0].Name)
	utils.AssertEqual(t, 10, cq.Data[0].Age)
	utils.AssertEqual(t, "doe", cq.Data[1].Name)
	utils.AssertEqual(t, 12, cq.Data[1].Age)
}

// go test -run Test_Ctx_ReqHeaderParser -v
func Test_Ctx_ReqHeaderParser(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	type Header struct {
		ID    int
		Name  string
		Hobby []string
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")

	c.Request().Header.Add("id", "1")
	c.Request().Header.Add("Name", "John Doe")
	c.Request().Header.Add("Hobby", "golang,fiber")
	q := new(Header)
	utils.AssertEqual(t, nil, c.ReqHeaderParser(q))
	utils.AssertEqual(t, 2, len(q.Hobby))

	c.Request().Header.Del("hobby")
	c.Request().Header.Add("Hobby", "golang,fiber,go")
	q = new(Header)
	utils.AssertEqual(t, nil, c.ReqHeaderParser(q))
	utils.AssertEqual(t, 3, len(q.Hobby))

	empty := new(Header)
	c.Request().Header.Del("hobby")
	utils.AssertEqual(t, nil, c.QueryParser(empty))
	utils.AssertEqual(t, 0, len(empty.Hobby))

	type Header2 struct {
		Bool            bool
		ID              int
		Name            string
		Hobby           string
		FavouriteDrinks []string
		Empty           []string
		Alloc           []string
		No              []int64
	}

	c.Request().Header.Add("id", "2")
	c.Request().Header.Add("Name", "Jane Doe")
	c.Request().Header.Del("hobby")
	c.Request().Header.Add("Hobby", "go,fiber")
	c.Request().Header.Add("favouriteDrinks", "milo,coke,pepsi")
	c.Request().Header.Add("alloc", "")
	c.Request().Header.Add("no", "1")

	h2 := new(Header2)
	h2.Bool = true
	h2.Name = "hello world"
	utils.AssertEqual(t, nil, c.ReqHeaderParser(h2))
	utils.AssertEqual(t, "go,fiber", h2.Hobby)
	utils.AssertEqual(t, true, h2.Bool)
	utils.AssertEqual(t, "Jane Doe", h2.Name) // check value get overwritten
	utils.AssertEqual(t, []string{"milo", "coke", "pepsi"}, h2.FavouriteDrinks)
	var nilSlice []string
	utils.AssertEqual(t, nilSlice, h2.Empty)
	utils.AssertEqual(t, []string{""}, h2.Alloc)
	utils.AssertEqual(t, []int64{1}, h2.No)

	type RequiredHeader struct {
		Name string `reqHeader:"name,required"`
	}
	rh := new(RequiredHeader)
	c.Request().Header.Del("name")
	utils.AssertEqual(t, "failed to decode: name is empty", c.ReqHeaderParser(rh).Error())
}

// go test -run Test_Ctx_ReqHeaderParser_WithSetParserDecoder -v
func Test_Ctx_ReqHeaderParser_WithSetParserDecoder(t *testing.T) {
	t.Parallel()
	type NonRFCTime time.Time

	nonRFCConverter := func(value string) reflect.Value {
		if v, err := time.Parse("2006-01-02", value); err == nil {
			return reflect.ValueOf(v)
		}
		return reflect.Value{}
	}

	nonRFCTime := ParserType{
		Customtype: NonRFCTime{},
		Converter:  nonRFCConverter,
	}

	SetParserDecoder(ParserConfig{
		IgnoreUnknownKeys: true,
		ParserType:        []ParserType{nonRFCTime},
		ZeroEmpty:         true,
		SetAliasTag:       "req",
	})

	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	type NonRFCTimeInput struct {
		Date  NonRFCTime `req:"date"`
		Title string     `req:"title"`
		Body  string     `req:"body"`
	}

	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	r := new(NonRFCTimeInput)

	c.Request().Header.Add("Date", "2021-04-10")
	c.Request().Header.Add("Title", "CustomDateTest")
	c.Request().Header.Add("Body", "October")

	utils.AssertEqual(t, nil, c.ReqHeaderParser(r))
	utils.AssertEqual(t, "CustomDateTest", r.Title)
	date := fmt.Sprintf("%v", r.Date)
	utils.AssertEqual(t, "{0 63753609600 <nil>}", date)
	utils.AssertEqual(t, "October", r.Body)

	c.Request().Header.Add("Title", "")
	r = &NonRFCTimeInput{
		Title: "Existing title",
		Body:  "Existing Body",
	}
	utils.AssertEqual(t, nil, c.ReqHeaderParser(r))
	utils.AssertEqual(t, "", r.Title)
}

// go test -run Test_Ctx_ReqHeaderParser_Schema -v
func Test_Ctx_ReqHeaderParser_Schema(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	type Header1 struct {
		Name   string `reqHeader:"Name,required"`
		Nested struct {
			Age int `reqHeader:"Age"`
		} `reqHeader:"Nested,required"`
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")

	c.Request().Header.Add("Name", "tom")
	c.Request().Header.Add("Nested.Age", "10")
	q := new(Header1)
	utils.AssertEqual(t, nil, c.ReqHeaderParser(q))

	c.Request().Header.Del("Name")
	q = new(Header1)
	utils.AssertEqual(t, "failed to decode: Name is empty", c.ReqHeaderParser(q).Error())

	c.Request().Header.Add("Name", "tom")
	c.Request().Header.Del("Nested.Age")
	c.Request().Header.Add("Nested.Agex", "10")
	q = new(Header1)
	utils.AssertEqual(t, nil, c.ReqHeaderParser(q))

	c.Request().Header.Del("Nested.Agex")
	q = new(Header1)
	utils.AssertEqual(t, "failed to decode: Nested is empty", c.ReqHeaderParser(q).Error())

	c.Request().Header.Del("Nested.Agex")
	c.Request().Header.Del("Name")

	type Header2 struct {
		Name   string `reqHeader:"Name"`
		Nested struct {
			Age int `reqHeader:"age,required"`
		} `reqHeader:"Nested"`
	}

	c.Request().Header.Add("Name", "tom")
	c.Request().Header.Add("Nested.Age", "10")

	h2 := new(Header2)
	utils.AssertEqual(t, nil, c.ReqHeaderParser(h2))

	c.Request().Header.Del("Name")
	h2 = new(Header2)
	utils.AssertEqual(t, nil, c.ReqHeaderParser(h2))

	c.Request().Header.Del("Name")
	c.Request().Header.Del("Nested.Age")
	c.Request().Header.Add("Nested.Agex", "10")
	h2 = new(Header2)
	utils.AssertEqual(t, "failed to decode: Nested.age is empty", c.ReqHeaderParser(h2).Error())

	type Node struct {
		Value int   `reqHeader:"Val,required"`
		Next  *Node `reqHeader:"Next,required"`
	}
	c.Request().Header.Add("Val", "1")
	c.Request().Header.Add("Next.Val", "3")
	n := new(Node)
	utils.AssertEqual(t, nil, c.ReqHeaderParser(n))
	utils.AssertEqual(t, 1, n.Value)
	utils.AssertEqual(t, 3, n.Next.Value)

	c.Request().Header.Del("Val")
	n = new(Node)
	utils.AssertEqual(t, "failed to decode: Val is empty", c.ReqHeaderParser(n).Error())

	c.Request().Header.Add("Val", "3")
	c.Request().Header.Del("Next.Val")
	c.Request().Header.Add("Next.Value", "2")
	n = new(Node)
	n.Next = new(Node)
	utils.AssertEqual(t, nil, c.ReqHeaderParser(n))
	utils.AssertEqual(t, 3, n.Value)
	utils.AssertEqual(t, 0, n.Next.Value)
}

func Test_Ctx_EqualFieldType(t *testing.T) {
	t.Parallel()
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

	var err error
	for n := 0; n < b.N; n++ {
		err = c.QueryParser(q)
	}
	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, nil, c.QueryParser(q))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_parseQuery -benchmem -count=4
func Benchmark_Ctx_parseQuery(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	type Person struct {
		Name string `query:"name"`
		Age  int    `query:"age"`
	}

	type CollectionQuery struct {
		Data []Person `query:"data"`
	}

	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")
	c.Request().URI().SetQueryString("data[0][name]=john&data[0][age]=10")
	cq := new(CollectionQuery)

	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		err = c.QueryParser(cq)
	}

	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, nil, c.QueryParser(cq))
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

	var err error
	for n := 0; n < b.N; n++ {
		err = c.QueryParser(q)
	}
	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, nil, c.QueryParser(q))
}

// go test -v  -run=^$ -bench=Benchmark_Ctx_ReqHeaderParser -benchmem -count=4
func Benchmark_Ctx_ReqHeaderParser(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	type ReqHeader struct {
		ID    int
		Name  string
		Hobby []string
	}
	c.Request().SetBody([]byte(``))
	c.Request().Header.SetContentType("")

	c.Request().Header.Add("id", "1")
	c.Request().Header.Add("Name", "John Doe")
	c.Request().Header.Add("Hobby", "golang,fiber")

	q := new(ReqHeader)
	b.ReportAllocs()
	b.ResetTimer()

	var err error
	for n := 0; n < b.N; n++ {
		err = c.ReqHeaderParser(q)
	}
	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, nil, c.ReqHeaderParser(q))
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
	utils.AssertEqual(b, nil, err)
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
	t.Parallel()
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

	// For the user id I will use the number 2222, so I should be able to get the number
	// 2222 from the Ctx even when the default value is specified
	app.Get("/testignoredefault/:user", func(c *Ctx) error {
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
	app.Get("/testdefault/:user", func(c *Ctx) error {
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

	_, err := app.Test(httptest.NewRequest(MethodGet, "/test/1111", nil))
	utils.AssertEqual(t, nil, err)

	_, err = app.Test(httptest.NewRequest(MethodGet, "/testnoint/xd", nil))
	utils.AssertEqual(t, nil, err)

	_, err = app.Test(httptest.NewRequest(MethodGet, "/testignoredefault/2222", nil))
	utils.AssertEqual(t, nil, err)

	_, err = app.Test(httptest.NewRequest(MethodGet, "/testdefault/xd", nil))
	utils.AssertEqual(t, nil, err)
}

// go test -run Test_Ctx_GetRespHeader
func Test_Ctx_GetRespHeader(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	c.Set("test", "Hello, World 👋!")
	c.Response().Header.Set(HeaderContentType, "application/json")
	utils.AssertEqual(t, c.GetRespHeader("test"), "Hello, World 👋!")
	utils.AssertEqual(t, c.GetRespHeader(HeaderContentType), "application/json")
}

// go test -run Test_Ctx_GetRespHeaders
func Test_Ctx_GetRespHeaders(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	c.Set("test", "Hello, World 👋!")
	c.Set("foo", "bar")
	c.Response().Header.Set(HeaderContentType, "application/json")

	utils.AssertEqual(t, c.GetRespHeaders(), map[string]string{
		"Content-Type": "application/json",
		"Foo":          "bar",
		"Test":         "Hello, World 👋!",
	})
}

// go test -run Test_Ctx_GetReqHeaders
func Test_Ctx_GetReqHeaders(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	c.Request().Header.Set("test", "Hello, World 👋!")
	c.Request().Header.Set("foo", "bar")
	c.Request().Header.Set(HeaderContentType, "application/json")

	utils.AssertEqual(t, c.GetReqHeaders(), map[string]string{
		"Content-Type": "application/json",
		"Foo":          "bar",
		"Test":         "Hello, World 👋!",
	})
}

// go test -run Test_Ctx_IsFromLocal
func Test_Ctx_IsFromLocal(t *testing.T) {
	t.Parallel()
	// Test "0.0.0.0", "127.0.0.1" and "::1".
	{
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, true, c.IsFromLocal())
	}
	// This is a test for "0.0.0.0"
	{
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "0.0.0.0")
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, true, c.IsFromLocal())
	}

	// This is a test for "127.0.0.1"
	{
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "127.0.0.1")
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, true, c.IsFromLocal())
	}

	// This is a test for "localhost"
	{
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, true, c.IsFromLocal())
	}

	// This is testing "::1", it is the compressed format IPV6 loopback address 0:0:0:0:0:0:0:1.
	// It is the equivalent of the IPV4 address 127.0.0.1.
	{
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "::1")
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, true, c.IsFromLocal())
	}

	{
		app := New()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().Header.Set(HeaderXForwardedFor, "93.46.8.90")
		defer app.ReleaseCtx(c)
		utils.AssertEqual(t, false, c.IsFromLocal())
	}
}

// go test -run Test_Ctx_RepeatParserWithSameStruct -v
func Test_Ctx_RepeatParserWithSameStruct(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	type Request struct {
		QueryParam  string `query:"query_param"`
		HeaderParam string `reqHeader:"header_param"`
		BodyParam   string `json:"body_param" xml:"body_param" form:"body_param"`
	}

	r := new(Request)

	c.Request().URI().SetQueryString("query_param=query_param")
	utils.AssertEqual(t, nil, c.QueryParser(r))
	utils.AssertEqual(t, "query_param", r.QueryParam)

	c.Request().Header.Add("header_param", "header_param")
	utils.AssertEqual(t, nil, c.ReqHeaderParser(r))
	utils.AssertEqual(t, "header_param", r.HeaderParam)

	var gzipJSON bytes.Buffer
	w := gzip.NewWriter(&gzipJSON)
	_, _ = w.Write([]byte(`{"body_param":"body_param"}`)) //nolint:errcheck // This will never fail
	err := w.Close()
	utils.AssertEqual(t, nil, err)
	c.Request().Header.SetContentType(MIMEApplicationJSON)
	c.Request().Header.Set(HeaderContentEncoding, "gzip")
	c.Request().SetBody(gzipJSON.Bytes())
	c.Request().Header.SetContentLength(len(gzipJSON.Bytes()))
	utils.AssertEqual(t, nil, c.BodyParser(r))
	utils.AssertEqual(t, "body_param", r.BodyParam)
	c.Request().Header.Del(HeaderContentEncoding)

	testDecodeParser := func(contentType, body string) {
		c.Request().Header.SetContentType(contentType)
		c.Request().SetBody([]byte(body))
		c.Request().Header.SetContentLength(len(body))
		utils.AssertEqual(t, nil, c.BodyParser(r))
		utils.AssertEqual(t, "body_param", r.BodyParam)
	}

	testDecodeParser(MIMEApplicationJSON, `{"body_param":"body_param"}`)
	testDecodeParser(MIMEApplicationXML, `<Demo><body_param>body_param</body_param></Demo>`)
	testDecodeParser(MIMEApplicationForm, "body_param=body_param")
	testDecodeParser(MIMEMultipartForm+`;boundary="b"`, "--b\r\nContent-Disposition: form-data; name=\"body_param\"\r\n\r\nbody_param\r\n--b--")
}
