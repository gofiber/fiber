// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"testing"

	"github.com/valyala/fasthttp"
)

// go test -v ./... -run=^$ -bench=Benchmark_Ctx -benchmem -count=3

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

// TODO
// func Benchmark_Ctx_BodyParser(b *testing.B) {

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

func Benchmark_Ctx_Format(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	c.Fasthttp.Request.Header.Set("Accept", "text/plain")
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	assertEqual(b, `Hello, World!`, string(c.Fasthttp.Response.Body()))
}

func Benchmark_Ctx_Format_HTML(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	c.Fasthttp.Request.Header.Set("Accept", "text/html")
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	assertEqual(b, "<p>Hello, World!</p>", string(c.Fasthttp.Response.Body()))
}

func Benchmark_Ctx_Format_JSON(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	c.Fasthttp.Request.Header.Set("Accept", "application/json")
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	assertEqual(b, `"Hello, World!"`, string(c.Fasthttp.Response.Body()))
}
func Benchmark_Ctx_Format_XML(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	c.Fasthttp.Request.Header.Set("Accept", "application/xml")
	for n := 0; n < b.N; n++ {
		c.Format("Hello, World!")
	}
	assertEqual(b, `<string>Hello, World!</string>`, string(c.Fasthttp.Response.Body()))

}

// func Benchmark_Ctx_Fresh(b *testing.B) {
// 	TODO
// }

// func Benchmark_Ctx_Is(b *testing.B) {
// 	TODO
// }
// TODO
// func Benchmark_Ctx_Next(b *testing.B) {

// }

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

func Benchmark_Ctx_Params(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	c.layer = &Layer{
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

// TODO
// func Benchmark_Ctx_Render(b *testing.B) {

// }

func Benchmark_Ctx_Send(b *testing.B) {
	c := AcquireCtx(&fasthttp.RequestCtx{})
	defer ReleaseCtx(c)

	var str = "Hello, World!"
	var byt = []byte("Hello, World!")
	var nmb = 123

	for n := 0; n < b.N; n++ {
		c.Send(byt, str, str)
		c.Send(str, nmb, nmb, nmb)
		c.Send(nmb)
	}

	assertEqual(b, "123", string(c.Fasthttp.Response.Body()))
}

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

	var str = "Hello, World!"
	var byt = []byte("Hello, World!")
	var nmb = 123

	for n := 0; n < b.N; n++ {
		c.Write(str)
		c.Write(byt)
		c.Write(nmb)
	}

	c.Send("") // empty body
	c.Write(str)

	assertEqual(b, "Hello, World!", string(c.Fasthttp.Response.Body()))
}
