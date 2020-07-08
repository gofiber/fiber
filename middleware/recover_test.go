package middleware

import (
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Middleware_Recover
func Test_Middleware_Recover(t *testing.T) {
	app := fiber.New()

	app.Use(Recover())

	app.Get("/panic", func(ctx *fiber.Ctx) {
		ctx.Set("dummy", "this should be here")
		panic("Hi, I'm an error!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/panic", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 500, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "this should be here", resp.Header.Get("dummy"))

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "Hi, I'm an error!", string(body))
}

// go test -v -run=^$ -bench=Benchmark_Middleware_Recover -benchmem -count=4
func Benchmark_Middleware_Recover(b *testing.B) {

	app := fiber.New()
	app.Use(Recover())

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
