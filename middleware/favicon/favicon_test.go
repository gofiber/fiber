package favicon

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Middleware_Favicon
func Test_Middleware_Favicon(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return nil
	})

	// Skip Favicon middleware
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("GET", "/favicon.ico", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusNoContent, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("OPTIONS", "/favicon.ico", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("PUT", "/favicon.ico", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusMethodNotAllowed, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "GET, HEAD, OPTIONS", resp.Header.Get(fiber.HeaderAllow))
}

// go test -run Test_Middleware_Favicon_Not_Found
func Test_Middleware_Favicon_Not_Found(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Fatal("should cache panic")
		}
	}()

	fiber.New().Use(New(Config{
		File: "non-exist.ico",
	}))
}

// go test -run Test_Middleware_Favicon_Found
func Test_Middleware_Favicon_Found(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		File: "../../.github/testdata/favicon.ico",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return nil
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/favicon.ico", nil))

	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "image/x-icon", resp.Header.Get(fiber.HeaderContentType))
	utils.AssertEqual(t, "public, max-age=31536000", resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")
}

// go test -run Test_Middleware_Favicon_CacheControl
func Test_Middleware_Favicon_CacheControl(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		CacheControl: "public, max-age=100",
		File:         "../../.github/testdata/favicon.ico",
	}))
	resp, err := app.Test(httptest.NewRequest("GET", "/favicon.ico", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "image/x-icon", resp.Header.Get(fiber.HeaderContentType))
	utils.AssertEqual(t, "public, max-age=100", resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")
}

// go test -v -run=^$ -bench=Benchmark_Middleware_Favicon -benchmem -count=4
func Benchmark_Middleware_Favicon(b *testing.B) {
	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c *fiber.Ctx) error {
		return nil
	})
	handler := app.Handler()

	c := &fasthttp.RequestCtx{}
	c.Request.SetRequestURI("/")

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		handler(c)
	}
}

// go test -run Test_Favicon_Next
func Test_Favicon_Next(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}
