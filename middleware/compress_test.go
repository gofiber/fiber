package middleware

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Middleware_Compress
func Test_Middleware_Compress(t *testing.T) {
	app := fiber.New()

	app.Use(Compress())

	app.Get("/", func(c *fiber.Ctx) {
		c.SendFile("../ctx.go", true)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(fiber.HeaderAcceptEncoding, "gzip")

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
	utils.AssertEqual(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
	os.Remove("../ctx.go.fiber.gz")
}

// go test -run Test_Middleware_Compress_Config
func Test_Middleware_Compress_Config(t *testing.T) {
	app := fiber.New()

	app.Use(Compress(CompressConfig{
		Level: CompressLevelDefault,
	}))

	app.Get("/", func(c *fiber.Ctx) {
		c.SendFile("../ctx.go", true)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(fiber.HeaderAcceptEncoding, "gzip")

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
	utils.AssertEqual(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
	os.Remove("../ctx.go.fiber.gz")
}

// go test -run Test_Middleware_Compress_Level
func Test_Middleware_Compress_Level(t *testing.T) {
	app := fiber.New()

	app.Use(Compress(2))

	app.Get("/", func(c *fiber.Ctx) {
		c.SendFile("../ctx.go", true)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(fiber.HeaderAcceptEncoding, "br")

	resp, err := app.Test(req, 3000)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "br", resp.Header.Get(fiber.HeaderContentEncoding))
	utils.AssertEqual(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
	os.Remove("../ctx.go.fiber.gz")
}

// go test -v -run=^$ -bench=Benchmark_Middleware_Compress -benchmem -count=4
func Benchmark_Middleware_Compress(b *testing.B) {
	app := fiber.New()
	app.Use(Compress())
	app.Get("/", func(c *fiber.Ctx) {
		c.SendFile("../ctx.go", true)
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
