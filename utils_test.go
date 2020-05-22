// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
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

// go test -race -run Test_Utils_matchParams
func Test_Utils_matchParams(t *testing.T) {
	t.Parallel()
	type testparams struct {
		url          string
		params       []string
		match        bool
		partialCheck bool
	}
	testCase := func(r string, cases []testparams) {
		parser := parseRoute(r)
		for _, c := range cases {
			paramsPos, match := parser.getMatch(c.url, c.partialCheck)
			utils.AssertEqual(t, c.match, match, fmt.Sprintf("route: '%s', url: '%s'", r, c.url))
			if match && paramsPos != nil {
				utils.AssertEqual(t, c.params, parser.paramsForPos(c.url, paramsPos), fmt.Sprintf("route: '%s', url: '%s'", r, c.url))
			} else {
				utils.AssertEqual(t, true, nil == paramsPos, fmt.Sprintf("route: '%s', url: '%s'", r, c.url))
			}
		}
	}
	testCase("/api/v1/:param/*", []testparams{
		{url: "/api/v1/entity", params: []string{"entity", ""}, match: true},
		{url: "/api/v1/entity/", params: []string{"entity", ""}, match: true},
		{url: "/api/v1/entity/1", params: []string{"entity", "1"}, match: true},
		{url: "/api/v", params: nil, match: false},
		{url: "/api/v2", params: nil, match: false},
		{url: "/api/v1/", params: nil, match: false},
	})
	testCase("/api/v1/:param?", []testparams{
		{url: "/api/v1", params: []string{""}, match: true},
		{url: "/api/v1/", params: []string{""}, match: true},
		{url: "/api/v1/optional", params: []string{"optional"}, match: true},
		{url: "/api/v", params: nil, match: false},
		{url: "/api/v2", params: nil, match: false},
		{url: "/api/xyz", params: nil, match: false},
	})
	testCase("/api/v1/*", []testparams{
		{url: "/api/v1", params: []string{""}, match: true},
		{url: "/api/v1/", params: []string{""}, match: true},
		{url: "/api/v1/entity", params: []string{"entity"}, match: true},
		{url: "/api/v1/entity/1/2", params: []string{"entity/1/2"}, match: true},
		{url: "/api/v1/Entity/1/2", params: []string{"Entity/1/2"}, match: true},
		{url: "/api/v", params: nil, match: false},
		{url: "/api/v2", params: nil, match: false},
		{url: "/api/abc", params: nil, match: false},
	})
	testCase("/api/v1/:param", []testparams{
		{url: "/api/v1/entity", params: []string{"entity"}, match: true},
		{url: "/api/v1/entity/8728382", params: nil, match: false},
		{url: "/api/v1", params: nil, match: false},
		{url: "/api/v1/", params: nil, match: false},
	})
	testCase("/api/v1/const", []testparams{
		{url: "/api/v1/const", params: []string{}, match: true},
		{url: "/api/v1", params: nil, match: false},
		{url: "/api/v1/", params: nil, match: false},
		{url: "/api/v1/something", params: nil, match: false},
	})
	testCase("/api/v1/:param/abc/*", []testparams{
		{url: "/api/v1/well/abc/wildcard", params: []string{"well", "wildcard"}, match: true},
		{url: "/api/v1/well/abc/", params: []string{"well", ""}, match: true},
		{url: "/api/v1/well/abc", params: []string{"well", ""}, match: true},
		{url: "/api/v1/well/ttt", params: nil, match: false},
	})
	testCase("/api/:day/:month?/:year?", []testparams{
		{url: "/api/1", params: []string{"1", "", ""}, match: true},
		{url: "/api/1/", params: []string{"1", "", ""}, match: true},
		{url: "/api/1/2", params: []string{"1", "2", ""}, match: true},
		{url: "/api/1/2/3", params: []string{"1", "2", "3"}, match: true},
		{url: "/api/", params: nil, match: false},
	})
	testCase("/api/*", []testparams{
		{url: "/api/", params: []string{""}, match: true},
		{url: "/api/joker", params: []string{"joker"}, match: true},
		{url: "/api", params: []string{""}, match: true},
		{url: "/api/v1/entity", params: []string{"v1/entity"}, match: true},
		{url: "/api2/v1/entity", params: nil, match: false},
		{url: "/api_ignore/v1/entity", params: nil, match: false},
	})
	testCase("/api/*/:param?", []testparams{
		{url: "/api/", params: []string{"", ""}, match: true},
		{url: "/api/joker", params: []string{"joker", ""}, match: true},
		{url: "/api/joker/batman", params: []string{"joker", "batman"}, match: true},
		{url: "/api/joker/batman/robin", params: []string{"joker/batman", "robin"}, match: true},
		{url: "/api/joker/batman/robin/1", params: []string{"joker/batman/robin", "1"}, match: true},
		{url: "/api", params: []string{"", ""}, match: true},
	})
	testCase("/api/*/:param", []testparams{
		{url: "/api/test/abc", params: []string{"test", "abc"}, match: true},
		{url: "/api/joker/batman", params: []string{"joker", "batman"}, match: true},
		{url: "/api/joker/batman/robin", params: []string{"joker/batman", "robin"}, match: true},
		{url: "/api/joker/batman/robin/1", params: []string{"joker/batman/robin", "1"}, match: true},
		{url: "/api", params: nil, match: false},
	})
	testCase("/partialCheck/foo/bar/:param", []testparams{
		{url: "/partialCheck/foo/bar/test", params: []string{"test"}, match: true, partialCheck: true},
		{url: "/partialCheck/foo/bar/test/test2", params: []string{"test"}, match: true, partialCheck: true},
		{url: "/partialCheck/foo/bar", params: nil, match: false, partialCheck: true},
		{url: "/partiaFoo", params: nil, match: false, partialCheck: true},
	})
	testCase("/api/*/:param/:param2", []testparams{
		{url: "/api/test/abc", params: nil, match: false},
		{url: "/api/joker/batman", params: nil, match: false},
		{url: "/api/joker/batman/robin", params: []string{"joker", "batman", "robin"}, match: true},
		{url: "/api/joker/batman/robin/1", params: []string{"joker/batman", "robin", "1"}, match: true},
		{url: "/api/joker/batman/robin/1/2", params: []string{"joker/batman/robin", "1", "2"}, match: true},
		{url: "/api", params: nil, match: false},
	})
	testCase("/", []testparams{
		{url: "/api", params: nil, match: false},
		{url: "", params: []string{}, match: true},
		{url: "/", params: []string{}, match: true},
	})
	testCase("/config/abc.json", []testparams{
		{url: "/config/abc.json", params: []string{}, match: true},
		{url: "config/abc.json", params: nil, match: false},
		{url: "/config/efg.json", params: nil, match: false},
		{url: "/config", params: nil, match: false},
	})
	testCase("/config/*.json", []testparams{
		{url: "/config/abc.json", params: []string{"abc.json"}, match: true},
		{url: "/config/efg.json", params: []string{"efg.json"}, match: true},
		//{url: "/config/efg.csv", params: nil, match: false},// doesn`t work, current: params: "efg.csv", true
		{url: "config/abc.json", params: nil, match: false},
		{url: "/config", params: nil, match: false},
	})
	testCase("/xyz", []testparams{
		{url: "xyz", params: nil, match: false},
		{url: "xyz/", params: nil, match: false},
	})
}

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
		res = getGroupPath("/v1/long/path/john/doe", "/why/this/name/is/so/awesome")
		res = getGroupPath("/v1", "/")
		res = getGroupPath("/v1", "/api")
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

// func Benchmark_Utils_getParams(b *testing.B) {
// 	// TODO
// }

// func Benchmark_Utils_matchParams(b *testing.B) {
// 	// TODO
// }

// func Benchmark_Utils_getCharPos(b *testing.B) {
// 	// TODO
// }
