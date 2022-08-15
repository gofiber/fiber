// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Redirect_To
func Test_Redirect_To(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Redirect().To("http://default.com")
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "http://default.com", string(c.Response().Header.Peek(HeaderLocation)))

	c.Redirect().Status(301).To("http://example.com")
	utils.AssertEqual(t, 301, c.Response().StatusCode())
	utils.AssertEqual(t, "http://example.com", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Route_WithParams
func Test_Redirect_Route_WithParams(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Redirect().Route("user", RedirectConfig{
		Params: Map{
			"name": "fiber",
		},
	})
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Route_WithParams_WithQueries
func Test_Redirect_Route_WithParams_WithQueries(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Redirect().Route("user", RedirectConfig{
		Params: Map{
			"name": "fiber",
		},
		Queries: map[string]string{"data[0][name]": "john", "data[0][age]": "10", "test": "doe"},
	})
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	// analysis of query parameters with url parsing, since a map pass is always randomly ordered
	location, err := url.Parse(string(c.Response().Header.Peek(HeaderLocation)))
	utils.AssertEqual(t, nil, err, "url.Parse(location)")
	utils.AssertEqual(t, "/user/fiber", location.Path)
	utils.AssertEqual(t, url.Values{"data[0][name]": []string{"john"}, "data[0][age]": []string{"10"}, "test": []string{"doe"}}, location.Query())
}

// go test -run Test_Redirect_Route_WithOptionalParams
func Test_Redirect_Route_WithOptionalParams(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name?", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Redirect().Route("user", RedirectConfig{
		Params: Map{
			"name": "fiber",
		},
	})
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Route_WithOptionalParamsWithoutValue
func Test_Redirect_Route_WithOptionalParamsWithoutValue(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name?", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Redirect().Route("user")
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/user/", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Route_WithGreedyParameters
func Test_Redirect_Route_WithGreedyParameters(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/+", func(c Ctx) error {
		return c.JSON(c.Params("+"))
	}).Name("user")
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Redirect().Route("user", RedirectConfig{
		Params: Map{
			"+": "test/routes",
		},
	})
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/user/test/routes", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Back
func Test_Redirect_Back(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		return c.JSON("Home")
	}).Name("home")
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Redirect().Back("/")
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Back_WithReferer
func Test_Redirect_Back_WithReferer(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		return c.JSON("Home")
	}).Name("home")
	app.Get("/back", func(c Ctx) error {
		return c.JSON("Back")
	}).Name("back")
	c := app.NewCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderReferer, "/back")
	c.Redirect().Back("/")
	utils.AssertEqual(t, 302, c.Response().StatusCode())
	utils.AssertEqual(t, "/back", c.Get(HeaderReferer))
	utils.AssertEqual(t, "/back", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Route -benchmem -count=4
func Benchmark_Redirect_Route(b *testing.B) {
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")

	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		c.Redirect().Route("user", RedirectConfig{
			Params: Map{
				"name": "fiber",
			},
		})
	}

	utils.AssertEqual(b, 302, c.Response().StatusCode())
	utils.AssertEqual(b, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Route_WithQueries -benchmem -count=4
func Benchmark_Redirect_Route_WithQueries(b *testing.B) {
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")

	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		c.Redirect().Route("user", RedirectConfig{
			Params: Map{
				"name": "fiber",
			},
			Queries: map[string]string{"a": "a", "b": "b"},
		})
	}

	utils.AssertEqual(b, 302, c.Response().StatusCode())
	// analysis of query parameters with url parsing, since a map pass is always randomly ordered
	location, err := url.Parse(string(c.Response().Header.Peek(HeaderLocation)))
	utils.AssertEqual(b, nil, err, "url.Parse(location)")
	utils.AssertEqual(b, "/user/fiber", location.Path)
	utils.AssertEqual(b, url.Values{"a": []string{"a"}, "b": []string{"b"}}, location.Query())
}
