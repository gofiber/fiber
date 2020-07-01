package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Middleware_Favicon
func Test_Middleware_Favicon(t *testing.T) {
	app := fiber.New()

	app.Use(Favicon())

	app.Get("/", func(c *fiber.Ctx) {})

	resp, err := app.Test(httptest.NewRequest("GET", "/favicon.ico", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 204, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("OPTIONS", "/favicon.ico", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("PUT", "/favicon.ico", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 405, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "GET, HEAD, OPTIONS", resp.Header.Get(fiber.HeaderAllow))
}

// go test -v -run=^$ -bench=Benchmark_Middleware_Favicon -benchmem -count=4
func Benchmark_Middleware_Favicon(b *testing.B) {
	app := fiber.New()
	app.Use(Favicon())
	app.Get("/", func(c *fiber.Ctx) {})
	handler := app.Handler()

	c := &fasthttp.RequestCtx{}
	c.Request.SetRequestURI("/")

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		handler(c)
	}
}
