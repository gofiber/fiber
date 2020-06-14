// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"testing"

	utils "github.com/gofiber/utils"
	fasthttp "github.com/valyala/fasthttp"
)

// go test -v -run=Test_Utils_ -count=3

func Test_Utils_ETag(t *testing.T) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Send("Hello, World!")
	setETag(c, false)
	utils.AssertEqual(t, `"13-1831710635"`, string(c.Fasthttp.Response.Header.Peek(HeaderETag)))
}

// go test -v -run=^$ -bench=Benchmark_App_ETag -benchmem -count=4
func Benchmark_Utils_ETag(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Send("Hello, World!")
	for n := 0; n < b.N; n++ {
		setETag(c, false)
	}
	utils.AssertEqual(b, `"13-1831710635"`, string(c.Fasthttp.Response.Header.Peek(HeaderETag)))
}

func Test_Utils_ETag_Weak(t *testing.T) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Send("Hello, World!")
	setETag(c, true)
	utils.AssertEqual(t, `W/"13-1831710635"`, string(c.Fasthttp.Response.Header.Peek(HeaderETag)))
}

// go test -v -run=^$ -bench=Benchmark_App_ETag_Weak -benchmem -count=4
func Benchmark_Utils_ETag_Weak(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Send("Hello, World!")
	for n := 0; n < b.N; n++ {
		setETag(c, true)
	}
	utils.AssertEqual(b, `W/"13-1831710635"`, string(c.Fasthttp.Response.Header.Peek(HeaderETag)))
}

func Test_Utils_getGroupPath(t *testing.T) {
	t.Parallel()
	res := getGroupPath("/v1", "/")
	utils.AssertEqual(t, "/v1", res)

	res = getGroupPath("/v1/", "/")
	utils.AssertEqual(t, "/v1/", res)

	res = getGroupPath("/v1", "/")
	utils.AssertEqual(t, "/v1", res)

	res = getGroupPath("/", "/")
	utils.AssertEqual(t, "/", res)

	res = getGroupPath("/v1/api/", "/")
	utils.AssertEqual(t, "/v1/api/", res)
}

// func Test_Utils_getArgument(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_parseTokenList(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_getParams(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_getTrimmedParam(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_getCharPos(t *testing.T) {
// 	// TODO
// }

//////////////////////////////////////////////
///////////////// BENCHMARKS /////////////////
//////////////////////////////////////////////
// go test -v -run=^$ -bench=Benchmark_Utils_ -benchmem -count=3

func Benchmark_Utils_getGroupPath(b *testing.B) {
	var res string
	for n := 0; n < b.N; n++ {
		_ = getGroupPath("/v1/long/path/john/doe", "/why/this/name/is/so/awesome")
		_ = getGroupPath("/v1", "/")
		_ = getGroupPath("/v1", "/api")
		res = getGroupPath("/v1", "/api/register/:project")
	}
	utils.AssertEqual(b, "/v1/api/register/:project", res)
}

// func Benchmark_Utils_getArgument(b *testing.B) {
// 	// TODO
// }

// func Benchmark_Utils_parseTokenList(b *testing.B) {
// 	// TODO
// }
