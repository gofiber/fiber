package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Middleware_Logger
func Test_Middleware_Logger(t *testing.T) {
	format := "${ip}-${ips}-${url}-${host}-${method}-${path}-${protocol}-${route}-${referer}-${ua}-${status}-${body}-${error}-${bytesSent}-${bytesReceived}-${header:header}-${query:query}-${cookie:cookie}"
	expect := "0.0.0.0--/test?query=query-example.com-GET-/test-http-/test-ref-ua-500--error-5-0-header-query-cookie"

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(Logger(LoggerConfig{
		Format: format,
		Output: buf,
	}))

	app.Get("/test", func(ctx *fiber.Ctx) {
		ctx.Next(errors.New("error"))
	})

	req := httptest.NewRequest("GET", "/test?query=query", nil)
	req.Header.Set("header", "header")
	req.Header.Set("Cookie", "cookie=cookie")
	req.Header.Set("User-Agent", "ua")
	req.Header.Set("Referer", "ref")

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 500, resp.StatusCode, "Status code")
	utils.AssertEqual(t, expect, buf.String())

}

func Test_Middleware_Logger_WithDefaultFormat(t *testing.T) {
	expectedOutputPattern := regexp.MustCompile(`^\d{2}:\d{2}:\d{2} GET / - 0\.0\.0\.0 - 200 - \d+(\.\d+)?.{1,3}
$`)
	// fake output
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	config := LoggerConfigDefault
	config.Output = buf
	app := fiber.New(&fiber.Settings{DisableStartupMessage: true})
	app.Use(Logger(config))

	app.Get("/", func(ctx *fiber.Ctx) {
		ctx.SendStatus(200)
	})

	_, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(
		t,
		true,
		expectedOutputPattern.MatchString(buf.String()),
		fmt.Sprintf("Has: %s, expected pattern: %s", buf.String(), expectedOutputPattern.String()),
	)
}

// go test -v -run=^$ -bench=Benchmark_Middleware_Logger -benchmem -count=4
func Benchmark_Middleware_Logger(b *testing.B) {

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(Logger(LoggerConfig{
		Output: buf,
	}))

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
