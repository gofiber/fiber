package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Middleware_RequestID
func Test_Middleware_RequestID(t *testing.T) {
	app := fiber.New()

	app.Use(RequestID())

	app.Get("/", func(ctx *fiber.Ctx) {
		ctx.Send("Hello?")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	reqid := resp.Header.Get(fiber.HeaderXRequestID)
	utils.AssertEqual(t, 36, len(reqid))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Add(fiber.HeaderXRequestID, reqid)

	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, reqid, resp.Header.Get(fiber.HeaderXRequestID))
}

// go test -run Test_Middleware_RequestID_Header
func Test_Middleware_RequestID_Header(t *testing.T) {
	app := fiber.New()

	app.Use(RequestID("X-Test-header"))

	app.Get("/", func(ctx *fiber.Ctx) {
		ctx.Send("Hello?")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	reqid := resp.Header.Get("X-Test-header")
	utils.AssertEqual(t, 36, len(reqid))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Add("X-Test-header", reqid)

	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, reqid, resp.Header.Get("X-Test-header"))
}

// go test -run Test_Middleware_RequestID_Config
func Test_Middleware_RequestID_Config(t *testing.T) {
	app := fiber.New()

	app.Use(RequestID(RequestIDConfig{
		Header: "X-Test-Header",
		Generator: func() string {
			return "johndoe"
		},
	}))

	app.Get("/", func(ctx *fiber.Ctx) {
		ctx.Send("Hello?")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	reqid := resp.Header.Get("X-Test-Header")
	utils.AssertEqual(t, "johndoe", reqid)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Add(fiber.HeaderXRequestID, reqid)

	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, reqid, resp.Header.Get("X-Test-Header"))
}

// go test -v -run=^$ -bench=Benchmark_Middleware_RequestID -benchmem -count=4
func Benchmark_Middleware_RequestID(b *testing.B) {

	app := fiber.New()
	app.Use(RequestID())

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
