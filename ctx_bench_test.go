// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"mime/multipart"
	"strconv"
	"testing"

	"github.com/valyala/fasthttp"
)

// go test -v ./... -run=^$ -bench=Benchmark_Ctx_Params -benchmem -count=3

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

func Benchmark_Ctx_AcceptsEncodings(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	c.Fasthttp.Request.Header.Set("Accept-Encoding", "deflate, gzip;q=1.0, *;q=0.5")

	var res string
	for n := 0; n < b.N; n++ {
		res = c.AcceptsEncodings("gzip")
	}

	assertEqual(b, "gzip", res)
}

func Benchmark_Ctx_AcceptsLanguages(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	c.Fasthttp.Request.Header.Set("Accept-Language", "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")

	var res string
	for n := 0; n < b.N; n++ {
		res = c.AcceptsLanguages("fr")
	}

	assertEqual(b, "fr", res)
}

func Benchmark_Ctx_BaseURL(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	c.Fasthttp.Request.SetHost("google.com:1337")

	var res string
	for n := 0; n < b.N; n++ {
		res = c.BaseURL()
	}

	assertEqual(b, "http://google.com:1337", res)
}

func Benchmark_Ctx_Body(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	c.Fasthttp.Request.SetBody([]byte("The best thing about a boolean is even if you are wrong, you are only off by a bit."))

	var res string
	for n := 0; n < b.N; n++ {
		res = c.Body()
	}

	assertEqual(b, "The best thing about a boolean is even if you are wrong, you are only off by a bit.", res)
}

// TODO
// func Benchmark_Ctx_BodyParser(b *testing.B) {

// }

func Benchmark_Ctx_Cookies(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	c.Fasthttp.Request.Header.Set("Cookie", "john=doe")

	var res string
	for n := 0; n < b.N; n++ {
		res = c.Cookies("john")
	}

	assertEqual(b, "doe", res)
}

func Benchmark_Ctx_FormFile(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	ioWriter, _ := writer.CreateFormFile("file", "test")
	_, _ = ioWriter.Write([]byte("hello world"))
	writer.Close()

	c.Fasthttp.Request.Header.SetMethod("POST")
	c.Fasthttp.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Fasthttp.Request.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))
	c.Fasthttp.Request.SetBody(body.Bytes())

	var res *multipart.FileHeader
	for n := 0; n < b.N; n++ {
		res, _ = c.FormFile("file")
	}

	assertEqual(b, "test", res.Filename)
	assertEqual(b, "application/octet-stream", res.Header["Content-Type"][0])
	assertEqual(b, int64(11), res.Size)
}

// func Benchmark_Ctx_FormValue(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_Fresh(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_Get(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_Hostname(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_IP(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_IPs(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_Is(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_Locals(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_Method(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_MultipartForm(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_OriginalURL(b *testing.B) {
// 	TODO
// }

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
		_ = c.Params("param1")
		_ = c.Params("param2")
		_ = c.Params("param3")
		res = c.Params("param4")
	}

	assertEqual(b, "awesome", res)
}

// func Benchmark_Ctx_Path(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_Query(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_Range(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_Route(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_SaveFile(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_Secure(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_Stale(b *testing.B) {
// 	TODO
// }

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

// func Benchmark_Ctx_Attachment(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_ClearCookie(b *testing.B) {
// 	TODO
// }

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

// func Benchmark_Ctx_Download(b *testing.B) {
// 	TODO
// }

func Benchmark_Ctx_Format(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	c.Fasthttp.Request.Header.Set("Accept", "text/html")
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	assertEqual(b, "<p>Hello, World!</p>", string(c.Fasthttp.Response.Body()))

	c.Fasthttp.Request.Header.Set("Accept", "application/json")
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	assertEqual(b, `"Hello, World!"`, string(c.Fasthttp.Response.Body()))

	c.Fasthttp.Request.Header.Set("Accept", "text/plain")
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	assertEqual(b, `Hello, World!`, string(c.Fasthttp.Response.Body()))

	c.Fasthttp.Request.Header.Set("Accept", "application/xml")
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	assertEqual(b, `<string>Hello, World!</string>`, string(c.Fasthttp.Response.Body()))
}

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
	var err error
	for n := 0; n < b.N; n++ {
		err = c.JSONP(data, "john")
	}

	assertEqual(b, nil, err)
	assertEqual(b, `john({"Name":"Grame","Age":20});`, string(c.Fasthttp.Response.Body()))
}

func Benchmark_Ctx_Links(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	for n := 0; n < b.N; n++ {
		c.Links(
			"http://api.example.com/users?page=2", "next",
			"http://api.example.com/users?page=5", "last",
		)
	}
	assertEqual(b, `<http://api.example.com/users?page=2>; rel="next",<http://api.example.com/users?page=5>; rel="last"`, string(c.Fasthttp.Response.Header.Peek("Link")))
}

// TODO
// func Benchmark_Ctx_Next(b *testing.B) {

// }

func Benchmark_Ctx_Redirect(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	for n := 0; n < b.N; n++ {
		c.Redirect("http://example.com")
		c.Redirect("http://example.com", 301)
	}
	assertEqual(b, 301, c.Fasthttp.Response.StatusCode())
	assertEqual(b, "http://example.com", string(c.Fasthttp.Response.Header.Peek("Location")))
}

// TODO
// func Benchmark_Ctx_Render(b *testing.B) {

// }

func Benchmark_Ctx_Send(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	for n := 0; n < b.N; n++ {
		c.Send([]byte("Hello, World"), "Hello, World!", "Hello, World!")
		c.Send("Hello, World", 50, 30, 20)
		c.Send(1337)
	}
	assertEqual(b, "1337", string(c.Fasthttp.Response.Body()))
}

func Benchmark_Ctx_SendBytes(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	for n := 0; n < b.N; n++ {
		c.SendBytes([]byte("Hello, World"))
	}
	assertEqual(b, "Hello, World", string(c.Fasthttp.Response.Body()))
}

func Benchmark_Ctx_SendStatus(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	for n := 0; n < b.N; n++ {
		c.SendStatus(415)
	}
	assertEqual(b, 415, c.Fasthttp.Response.StatusCode())
	assertEqual(b, "Unsupported Media Type", string(c.Fasthttp.Response.Body()))
}

func Benchmark_Ctx_SendString(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	for n := 0; n < b.N; n++ {
		c.Send("Hello, World")
	}
	assertEqual(b, "Hello, World", string(c.Fasthttp.Response.Body()))
}

func Benchmark_Ctx_Set(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	for n := 0; n < b.N; n++ {
		c.Set("X-I'm-a-Dummy-HeAdEr", "1337")
	}

	assertEqual(b, "1337", string(c.Fasthttp.Response.Header.Peek("X-I'm-a-Dummy-HeAdEr")))
}

func Benchmark_Ctx_Type(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	for n := 0; n < b.N; n++ {
		c.Type(".json")
		c.Type("json")
	}

	assertEqual(b, "application/json", string(c.Fasthttp.Response.Header.Peek("Content-Type")))
}

func Benchmark_Ctx_Vary(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	for n := 0; n < b.N; n++ {
		c.Vary("Origin")
	}

	//assertEqual(b, "origin", string(c.Fasthttp.Response.Header.Peek("Vary")))
}

func Benchmark_Ctx_Write(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	for n := 0; n < b.N; n++ {
		c.Write("Hello, ")
		c.Write([]byte("World! "))
		c.Write(123)
	}
	c.Send("") // empty body
	c.Write("Hello, World!")
	assertEqual(b, "Hello, World!", string(c.Fasthttp.Response.Body()))
}

func Benchmark_Ctx_XHR(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	c.Fasthttp.Request.Header.Set("X-Requested-With", "xMlHtTpReQuEst")

	var res bool
	for n := 0; n < b.N; n++ {
		res = c.XHR()
	}

	assertEqual(b, true, res)
}
