// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

// go test -v ./... -run=^$ -bench=Benchmark_Router -benchmem -count=2

import (
	"encoding/json"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	utils "github.com/gofiber/utils"
	fasthttp "github.com/valyala/fasthttp"
)

var routesFixture = routeJSON{}

func init() {
	dat, err := ioutil.ReadFile("./.github/fixture/testRoutes.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(dat, &routesFixture); err != nil {
		panic(err)
	}
}

func Test_Route_Match_SameLength(t *testing.T) {
	app := New()

	app.Get("/:param", func(ctx *Ctx) {
		ctx.Send(ctx.Params("param"))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/:param", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, ":param", getString(body))

	// with param
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "test", getString(body))
}

func Test_Route_Match_Star(t *testing.T) {
	app := New()

	app.Get("/*", func(ctx *Ctx) {
		ctx.Send(ctx.Params("*"))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/*", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "*", getString(body))

	// with param
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "test", getString(body))
}

func Test_Route_Match_Root(t *testing.T) {
	app := New()

	app.Get("/", func(ctx *Ctx) {
		ctx.Send("root")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "root", getString(body))
}

func Test_Route_Match_Parser(t *testing.T) {
	app := New()

	app.Get("/foo/:Param", func(ctx *Ctx) {
		ctx.Send(ctx.Params("Param"))
	})
	app.Get("/Foobar/*", func(ctx *Ctx) {
		ctx.Send(ctx.Params("*"))
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/foo/bar", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "bar", getString(body))

	// with star
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/Foobar/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "test", getString(body))
}

func Test_Route_Match_Middleware(t *testing.T) {
	app := New()

	app.Use("/foo/*", func(ctx *Ctx) {
		ctx.Send(ctx.Params("*"))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/foo/*", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "*", getString(body))

	// with param
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo/bar/fasel", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "bar/fasel", getString(body))
}

func Test_Route_Match_UnescapedPath(t *testing.T) {
	app := New(&Settings{UnescapePath: true})

	app.Use("/créer", func(ctx *Ctx) {
		ctx.Send("test")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/cr%C3%A9er", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "test", getString(body))
	// without special chars
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/créer", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	// check deactivated behavior
	app.Settings.UnescapePath = false
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/cr%C3%A9er", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusNotFound, resp.StatusCode, "Status code")
}

func Test_Route_Match_Middleware_HasPrefix(t *testing.T) {
	app := New()

	app.Use("/foo", func(ctx *Ctx) {
		ctx.Send("middleware")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/foo/bar", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "middleware", getString(body))
}

func Test_Route_Match_Middleware_Root(t *testing.T) {
	app := New()

	app.Use("/", func(ctx *Ctx) {
		ctx.Send("middleware")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/everything", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "middleware", getString(body))
}

func Test_Ensure_Router_Interface_Implementation(t *testing.T) {
	var app interface{} = (*App)(nil)
	_, ok := app.(Router)
	utils.AssertEqual(t, true, ok)

	var group interface{} = (*Group)(nil)
	_, ok = group.(Router)
	utils.AssertEqual(t, true, ok)
}

//////////////////////////////////////////////
///////////////// BENCHMARKS /////////////////
//////////////////////////////////////////////

func registerDummyRoutes(app *App) {
	h := func(c *Ctx) {}
	for _, r := range routesFixture.GithubAPI {
		app.Add(r.Method, r.Path, h)
	}
}

// go test -v -run=^$ -bench=Benchmark_App_MethodNotAllowed -benchmem -count=4
func Benchmark_App_MethodNotAllowed(b *testing.B) {
	app := New()
	h := func(c *Ctx) {
		c.Send("Hello World!")
	}
	app.All("/this/is/a/", h)
	app.Get("/this/is/a/dummy/route/oke", h)
	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/this/is/a/dummy/route/oke")

	for n := 0; n < b.N; n++ {
		app.handler(c)
	}
	utils.AssertEqual(b, 405, c.Response.StatusCode())
	utils.AssertEqual(b, "GET, HEAD", string(c.Response.Header.Peek("Allow")))
	utils.AssertEqual(b, "Cannot DELETE /this/is/a/dummy/route/oke", string(c.Response.Body()))
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_NotFound -benchmem -count=4
func Benchmark_Router_NotFound(b *testing.B) {
	app := New()
	app.Use(func(c *Ctx) {
		c.Next()
	})
	registerDummyRoutes(app)
	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/this/route/does/not/exist")

	for n := 0; n < b.N; n++ {
		app.handler(c)
	}
	utils.AssertEqual(b, 404, c.Response.StatusCode())
	utils.AssertEqual(b, "Cannot DELETE /this/route/does/not/exist", string(c.Response.Body()))
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Handler -benchmem -count=4
func Benchmark_Router_Handler(b *testing.B) {
	app := New()
	registerDummyRoutes(app)

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	for n := 0; n < b.N; n++ {
		app.handler(c)
	}
}

func Benchmark_Router_Handler_Strict_Case(b *testing.B) {
	app := New(&Settings{
		StrictRouting: true,
		CaseSensitive: true,
	})
	registerDummyRoutes(app)

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	for n := 0; n < b.N; n++ {
		app.handler(c)
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Chain -benchmem -count=4
func Benchmark_Router_Chain(b *testing.B) {
	app := New()
	handler := func(c *Ctx) {
		c.Next()
	}
	app.Get("/", handler, handler, handler, handler, handler, handler)

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("GET")
	c.URI().SetPath("/")

	for n := 0; n < b.N; n++ {
		app.handler(c)
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Next -benchmem -count=4
func Benchmark_Router_Next(b *testing.B) {
	app := New()
	registerDummyRoutes(app)

	request := &fasthttp.RequestCtx{}

	request.Request.Header.SetMethod("DELETE")
	request.URI().SetPath("/user/keys/1337")
	var res bool

	c := app.AcquireCtx(request)
	defer app.ReleaseCtx(c)

	for n := 0; n < b.N; n++ {
		c.indexRoute = -1
		res = app.next(c)
	}
	utils.AssertEqual(b, true, res)
	utils.AssertEqual(b, 31, c.indexRoute)
}

// go test -v ./... -run=^$ -bench=Benchmark_Route_Match -benchmem -count=4
func Benchmark_Route_Match(b *testing.B) {
	var match bool
	var params []string

	parsed := parseRoute("/user/keys/:id")
	route := &Route{
		use:         false,
		root:        false,
		star:        false,
		routeParser: parsed,
		routeParams: parsed.params,
		path:        "/user/keys/:id",

		Path:   "/user/keys/:id",
		Method: "DELETE",
	}
	route.Handlers = append(route.Handlers, func(c *Ctx) {})
	for n := 0; n < b.N; n++ {
		match, params = route.match("/user/keys/1337", "/user/keys/1337")
	}

	utils.AssertEqual(b, true, match)
	utils.AssertEqual(b, []string{"1337"}, params)
}

// go test -v ./... -run=^$ -bench=Benchmark_Route_Match_Star -benchmem -count=4
func Benchmark_Route_Match_Star(b *testing.B) {
	var match bool
	var params []string

	parsed := parseRoute("/*")
	route := &Route{
		use:         false,
		root:        false,
		star:        true,
		routeParser: parsed,
		routeParams: parsed.params,
		path:        "/user/keys/bla",

		Path:   "/user/keys/bla",
		Method: "DELETE",
	}
	route.Handlers = append(route.Handlers, func(c *Ctx) {})
	for n := 0; n < b.N; n++ {
		match, params = route.match("/user/keys/bla", "/user/keys/bla")
	}

	utils.AssertEqual(b, true, match)
	utils.AssertEqual(b, []string{"user/keys/bla"}, params)
}

// go test -v ./... -run=^$ -bench=Benchmark_Route_Match_Root -benchmem -count=4
func Benchmark_Route_Match_Root(b *testing.B) {
	var match bool
	var params []string

	parsed := parseRoute("/")
	route := &Route{
		use:         false,
		root:        true,
		star:        false,
		path:        "/",
		routeParser: parsed,
		routeParams: parsed.params,

		Path:   "/",
		Method: "DELETE",
	}
	route.Handlers = append(route.Handlers, func(c *Ctx) {})
	for n := 0; n < b.N; n++ {
		match, params = route.match("/", "/")
	}

	utils.AssertEqual(b, true, match)
	utils.AssertEqual(b, []string(nil), params)
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Handler_CaseSensitive -benchmem -count=4
func Benchmark_Router_Handler_CaseSensitive(b *testing.B) {
	app := New()
	app.Settings.CaseSensitive = true
	registerDummyRoutes(app)

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	for n := 0; n < b.N; n++ {
		app.handler(c)
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Handler_Unescape -benchmem -count=4
func Benchmark_Router_Handler_Unescape(b *testing.B) {
	app := New()
	app.Settings.UnescapePath = true
	registerDummyRoutes(app)
	app.Delete("/créer", func(c *Ctx) {})

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod(MethodDelete)
	c.URI().SetPath("/cr%C3%A9er")

	for n := 0; n < b.N; n++ {
		app.handler(c)
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Handler_StrictRouting -benchmem -count=4
func Benchmark_Router_Handler_StrictRouting(b *testing.B) {
	app := New()
	app.Settings.CaseSensitive = true
	registerDummyRoutes(app)

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	for n := 0; n < b.N; n++ {
		app.handler(c)
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Github_API -benchmem -count=4
func Benchmark_Router_Github_API(b *testing.B) {
	app := New()
	registerDummyRoutes(app)

	var match bool
	var params []string

	for n := 0; n < b.N; n++ {
		for i := range routesFixture.TestRoutes {

			mINT := methodInt(routesFixture.TestRoutes[i].Method)
			path := routesFixture.TestRoutes[i].Path

			for i := range app.stack[mINT] {
				match, params = app.stack[mINT][i].match(path, path)
			}
		}
	}

	utils.AssertEqual(b, true, match)
	utils.AssertEqual(b, true, params != nil)
}

type testRoute struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}
type routeJSON struct {
	TestRoutes []testRoute `json:"testRoutes"`
	GithubAPI  []testRoute `json:"githubAPI"`
}
