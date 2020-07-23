package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
	"github.com/valyala/fasthttp"
)

var (
	UUIDLen = 36
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
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	reqid := resp.Header.Get(fiber.HeaderXRequestID)
	utils.AssertEqual(t, UUIDLen, len(reqid))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Add(fiber.HeaderXRequestID, reqid)

	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
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
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	reqid := resp.Header.Get("X-Test-header")
	utils.AssertEqual(t, UUIDLen, len(reqid))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Add("X-Test-header", reqid)

	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, reqid, resp.Header.Get("X-Test-header"))
}

// go test -run Test_Middleware_RequestID_Options_And_WithConfig
func Test_Middleware_RequestID_Options_And_WithConfig(t *testing.T) {

	testCases := []struct {
		idLen   int
		header  string
		handler fiber.Handler
	}{
		{
			idLen:   UUIDLen,
			header:  "X-Test-header",
			handler: RequestID("X-Test-header"),
		},
		{
			idLen:   7,
			header:  RequestIDConfigDefault.Header,
			handler: RequestID(func() string { return "fake-id" }),
		},
		{
			idLen:   UUIDLen,
			header:  RequestIDConfigDefault.Header,
			handler: RequestID(RequestIDConfig{}),
		},
	}

	for _, testCase := range testCases {
		app := fiber.New()

		app.Use(testCase.handler)

		app.Get("/", func(ctx *fiber.Ctx) {
			ctx.Send("Hello?")
		})

		resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
		reqid := resp.Header.Get(testCase.header)
		utils.AssertEqual(t, testCase.idLen, len(reqid))
	}
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
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	reqid := resp.Header.Get("X-Test-Header")
	utils.AssertEqual(t, "johndoe", reqid)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Add(fiber.HeaderXRequestID, reqid)

	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, reqid, resp.Header.Get("X-Test-Header"))
}

// go test -run Test_Middleware_RequestID_Skip
func Test_Middleware_RequestID_Skip(t *testing.T) {
	app := fiber.New()

	app.Use(RequestID(func(_ *fiber.Ctx) bool {
		return true
	}))

	app.Get("/", func(ctx *fiber.Ctx) {})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "", resp.Header.Get(RequestIDConfigDefault.Header), RequestIDConfigDefault.Header)
}

// go test -run Test_Middleware_RequestID_Panic
func Test_Middleware_RequestID_Panic(t *testing.T) {
	defer func() {
		utils.AssertEqual(t,
			"RequestID: the following option types are allowed: `string`, `func() string`, `func(*fiber.Ctx) bool`, `RequestIDConfig`",
			fmt.Sprintf("%s", recover()))
	}()

	RequestID(0)
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
