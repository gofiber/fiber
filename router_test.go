// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📃 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

// go test -v ./... -run=^$ -bench=Benchmark_Router -benchmem -count=2

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

var routesFixture = routeJSON{}

func init() {
	dat, err := ioutil.ReadFile("./.github/testdata/testRoutes.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(dat, &routesFixture); err != nil {
		panic(err)
	}
}

func Test_Route_Match_SameLength(t *testing.T) {
	app := New()

	app.Get("/:param", func(c *Ctx) error {
		return c.SendString(c.Params("param"))
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

	app.Get("/*", func(c *Ctx) error {
		return c.SendString(c.Params("*"))
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

	// without parameter
	route := Route{
		star:        true,
		path:        "/*",
		routeParser: routeParser{},
	}
	params := [maxParams]string{}
	match := route.match("", "", &params)
	utils.AssertEqual(t, true, match)
	utils.AssertEqual(t, [maxParams]string{}, params)

	// with parameter
	match = route.match("/favicon.ico", "/favicon.ico", &params)
	utils.AssertEqual(t, true, match)
	utils.AssertEqual(t, [maxParams]string{"favicon.ico"}, params)

	// without parameter again
	match = route.match("", "", &params)
	utils.AssertEqual(t, true, match)
	utils.AssertEqual(t, [maxParams]string{}, params)
}

func Test_Route_Match_Root(t *testing.T) {
	app := New()

	app.Get("/", func(c *Ctx) error {
		return c.SendString("root")
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

	app.Get("/foo/:ParamName", func(c *Ctx) error {
		return c.SendString(c.Params("ParamName"))
	})
	app.Get("/Foobar/*", func(c *Ctx) error {
		return c.SendString(c.Params("*"))
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

	app.Use("/foo/*", func(c *Ctx) error {
		return c.SendString(c.Params("*"))
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
	app := New(Config{UnescapePath: true})

	app.Use("/créer", func(c *Ctx) error {
		return c.SendString("test")
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
	app.config.UnescapePath = false
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/cr%C3%A9er", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusNotFound, resp.StatusCode, "Status code")
}

func Test_Route_Match_Middleware_HasPrefix(t *testing.T) {
	app := New()

	app.Use("/foo", func(c *Ctx) error {
		return c.SendString("middleware")
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

	app.Use("/", func(c *Ctx) error {
		return c.SendString("middleware")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/everything", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "middleware", getString(body))
}

func Test_Router_Register_Missing_Handler(t *testing.T) {
	app := New()
	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "missing handler in route: /doe\n", fmt.Sprintf("%v", err))
		}
	}()
	app.register("USE", "/doe")
}

func Test_Ensure_Router_Interface_Implementation(t *testing.T) {
	var app interface{} = (*App)(nil)
	_, ok := app.(Router)
	utils.AssertEqual(t, true, ok)

	var group interface{} = (*Group)(nil)
	_, ok = group.(Router)
	utils.AssertEqual(t, true, ok)
}

func Test_Router_Handler_SetETag(t *testing.T) {
	app := New()
	app.config.ETag = true

	app.Get("/", func(c *Ctx) error {
		return c.SendString("Hello, World!")
	})

	c := &fasthttp.RequestCtx{}

	app.Handler()(c)

	utils.AssertEqual(t, `"13-1831710635"`, string(c.Response.Header.Peek(HeaderETag)))
}

func Test_Router_Handler_Catch_Error(t *testing.T) {
	app := New()
	app.config.ErrorHandler = func(ctx *Ctx, err error) error {
		return errors.New("fake error")
	}

	app.Get("/", func(c *Ctx) error {
		return ErrForbidden
	})

	c := &fasthttp.RequestCtx{}

	app.Handler()(c)

	utils.AssertEqual(t, StatusInternalServerError, c.Response.Header.StatusCode())
}

//////////////////////////////////////////////
///////////////// BENCHMARKS /////////////////
//////////////////////////////////////////////

func registerDummyRoutes(app *App) {
	h := func(c *Ctx) error {
		return nil
	}
	for _, r := range routesFixture.GithubAPI {
		app.Add(r.Method, r.Path, h)
	}
}

// go test -v -run=^$ -bench=Benchmark_App_MethodNotAllowed -benchmem -count=4
func Benchmark_App_MethodNotAllowed(b *testing.B) {
	app := New()
	h := func(c *Ctx) error {
		return c.SendString("Hello World!")
	}
	app.All("/this/is/a/", h)
	app.Get("/this/is/a/dummy/route/oke", h)
	appHandler := app.Handler()
	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/this/is/a/dummy/route/oke")

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		appHandler(c)
	}
	b.StopTimer()
	utils.AssertEqual(b, 405, c.Response.StatusCode())
	utils.AssertEqual(b, "GET, HEAD", string(c.Response.Header.Peek("Allow")))
	utils.AssertEqual(b, utils.StatusMessage(StatusMethodNotAllowed), string(c.Response.Body()))
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_NotFound -benchmem -count=4
func Benchmark_Router_NotFound(b *testing.B) {
	app := New()
	app.Use(func(c *Ctx) error {
		return c.Next()
	})
	registerDummyRoutes(app)
	appHandler := app.Handler()
	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/this/route/does/not/exist")

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		appHandler(c)
	}
	utils.AssertEqual(b, 404, c.Response.StatusCode())
	utils.AssertEqual(b, "Cannot DELETE /this/route/does/not/exist", string(c.Response.Body()))
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Handler -benchmem -count=4
func Benchmark_Router_Handler(b *testing.B) {
	app := New()
	registerDummyRoutes(app)
	appHandler := app.Handler()

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		appHandler(c)
	}
}

func Benchmark_Router_Handler_Strict_Case(b *testing.B) {
	app := New(Config{
		StrictRouting: true,
		CaseSensitive: true,
	})
	registerDummyRoutes(app)
	appHandler := app.Handler()

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		appHandler(c)
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Chain -benchmem -count=4
func Benchmark_Router_Chain(b *testing.B) {
	app := New()
	handler := func(c *Ctx) error {
		return c.Next()
	}
	app.Get("/", handler, handler, handler, handler, handler, handler)

	appHandler := app.Handler()

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("GET")
	c.URI().SetPath("/")
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		appHandler(c)
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_WithCompression -benchmem -count=4
func Benchmark_Router_WithCompression(b *testing.B) {
	app := New()
	handler := func(c *Ctx) error {
		return c.Next()
	}
	app.Get("/", handler)
	app.Get("/", handler)
	app.Get("/", handler)
	app.Get("/", handler)
	app.Get("/", handler)
	app.Get("/", handler)

	appHandler := app.Handler()
	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("GET")
	c.URI().SetPath("/")
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		appHandler(c)
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Next -benchmem -count=4
func Benchmark_Router_Next(b *testing.B) {
	app := New()
	registerDummyRoutes(app)
	app.startupProcess()

	request := &fasthttp.RequestCtx{}

	request.Request.Header.SetMethod("DELETE")
	request.URI().SetPath("/user/keys/1337")
	var res bool
	var err error

	c := app.AcquireCtx(request)
	defer app.ReleaseCtx(c)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		c.indexRoute = -1
		res, err = app.next(c)
	}
	utils.AssertEqual(b, nil, err)
	utils.AssertEqual(b, true, res)
	utils.AssertEqual(b, 4, c.indexRoute)
}

// go test -v ./... -run=^$ -bench=Benchmark_Route_Match -benchmem -count=4
func Benchmark_Route_Match(b *testing.B) {
	var match bool
	var params [maxParams]string

	parsed := parseRoute("/user/keys/:id")
	route := &Route{
		use:         false,
		root:        false,
		star:        false,
		routeParser: parsed,
		Params:      parsed.params,
		path:        "/user/keys/:id",

		Path:   "/user/keys/:id",
		Method: "DELETE",
	}
	route.Handlers = append(route.Handlers, func(c *Ctx) error {
		return nil
	})
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		match = route.match("/user/keys/1337", "/user/keys/1337", &params)
	}

	utils.AssertEqual(b, true, match)
	utils.AssertEqual(b, []string{"1337"}, params[0:len(parsed.params)])
}

// go test -v ./... -run=^$ -bench=Benchmark_Route_Match_Star -benchmem -count=4
func Benchmark_Route_Match_Star(b *testing.B) {
	var match bool
	var params [maxParams]string

	parsed := parseRoute("/*")
	route := &Route{
		use:         false,
		root:        false,
		star:        true,
		routeParser: parsed,
		Params:      parsed.params,
		path:        "/user/keys/bla",

		Path:   "/user/keys/bla",
		Method: "DELETE",
	}
	route.Handlers = append(route.Handlers, func(c *Ctx) error {
		return nil
	})
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		match = route.match("/user/keys/bla", "/user/keys/bla", &params)
	}

	utils.AssertEqual(b, true, match)
	utils.AssertEqual(b, []string{"user/keys/bla"}, params[0:len(parsed.params)])
}

// go test -v ./... -run=^$ -bench=Benchmark_Route_Match_Root -benchmem -count=4
func Benchmark_Route_Match_Root(b *testing.B) {
	var match bool
	var params [maxParams]string

	parsed := parseRoute("/")
	route := &Route{
		use:         false,
		root:        true,
		star:        false,
		path:        "/",
		routeParser: parsed,
		Params:      parsed.params,

		Path:   "/",
		Method: "DELETE",
	}
	route.Handlers = append(route.Handlers, func(c *Ctx) error {
		return nil
	})

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		match = route.match("/", "/", &params)
	}

	utils.AssertEqual(b, true, match)
	utils.AssertEqual(b, []string{}, params[0:len(parsed.params)])
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Handler_CaseSensitive -benchmem -count=4
func Benchmark_Router_Handler_CaseSensitive(b *testing.B) {
	app := New()
	app.config.CaseSensitive = true
	registerDummyRoutes(app)
	appHandler := app.Handler()

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		appHandler(c)
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Handler_Unescape -benchmem -count=4
func Benchmark_Router_Handler_Unescape(b *testing.B) {
	app := New()
	app.config.UnescapePath = true
	registerDummyRoutes(app)
	app.Delete("/créer", func(c *Ctx) error {
		return nil
	})

	appHandler := app.Handler()

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod(MethodDelete)
	c.URI().SetPath("/cr%C3%A9er")

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		c.URI().SetPath("/cr%C3%A9er")
		appHandler(c)
	}
}

// go test -run=^$ -bench=Benchmark_Router_Handler_StrictRouting -benchmem -count=4
func Benchmark_Router_Handler_StrictRouting(b *testing.B) {
	app := New()
	app.config.CaseSensitive = true
	registerDummyRoutes(app)
	appHandler := app.Handler()

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		appHandler(c)
	}
}

// go test -run=^$ -bench=Benchmark_Router_Github_API -benchmem -count=16
func Benchmark_Router_Github_API(b *testing.B) {
	app := New()
	registerDummyRoutes(app)
	app.startupProcess()

	c := &fasthttp.RequestCtx{}
	var match bool
	var err error

	b.ResetTimer()

	for i := range routesFixture.TestRoutes {
		c.Request.Header.SetMethod(routesFixture.TestRoutes[i].Method)
		for n := 0; n < b.N; n++ {
			c.URI().SetPath(routesFixture.TestRoutes[i].Path)
			ctx := app.AcquireCtx(c)
			match, err = app.next(ctx)
			app.ReleaseCtx(ctx)
		}
		utils.AssertEqual(b, nil, err)
		utils.AssertEqual(b, true, match)
	}

}

type testRoute struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}
type routeJSON struct {
	TestRoutes []testRoute `json:"testRoutes"`
	GithubAPI  []testRoute `json:"githubAPI"`
}
