package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Middleware_Logger
func Test_Middleware_Logger(t *testing.T) {
	format := "${ip}-${ips}-${url}-${host}-${method}-${methodColored}-${path}-${protocol}-${route}-${referer}-${ua}-${status}-${statusColored}-${body}-${error}-${bytesSent}-${bytesReceived}-${header:header}-${query:query}-${form:form}-${cookie:cookie}"
	expect := "0.0.0.0--/test?query=query-example.com-POST-\u001b[96mPOST\u001b[0m-/test-http-/test-ref-ua-500-\u001b[91m500\u001b[0m-form=form-error-5-9-header-query-form-cookie"

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(Logger(LoggerConfig{
		Format: format,
		Output: buf,
	}))

	app.Post("/test", func(ctx *fiber.Ctx) {
		ctx.Next(errors.New("error"))
	})

	req := httptest.NewRequest("POST", "/test?query=query", strings.NewReader("form=form"))
	req.Header.Set("header", "header")
	req.Header.Set("Cookie", "cookie=cookie")
	req.Header.Set("User-Agent", "ua")
	req.Header.Set("Referer", "ref")
	req.Header.Set("Content-type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusInternalServerError, resp.StatusCode, "Status code")
	utils.AssertEqual(t, expect, buf.String())

}

func Test_Middleware_Logger_WithDefaultFormat(t *testing.T) {
	expectedOutputPattern := regexp.MustCompile(`^\d{2}:\d{2}:\d{2} GET / - 0\.0\.0\.0 - 200 - \d+(\.\d+)?.{1,3}
$`)
	// fake output
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	LoggerConfigDefault.Output = buf
	config := LoggerConfigDefault
	config.Output = nil

	app := fiber.New(&fiber.Settings{DisableStartupMessage: true})
	app.Use(Logger(config))

	app.Get("/", func(ctx *fiber.Ctx) {
		ctx.SendStatus(fiber.StatusOK)
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

// go test -run Test_Middleware_Logger_Skip
func Test_Middleware_Logger_Skip(t *testing.T) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	LoggerConfigDefault.Output = buf

	app := fiber.New()

	app.Use(Logger(func(_ *fiber.Ctx) bool {
		return true
	}))

	app.Get("/", func(_ *fiber.Ctx) {})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, 0, buf.Len(), "buf.Len()")
}

// go test -run Test_Middleware_Logger_Options_And_WithConfig
func Test_Middleware_Logger_Options_And_WithConfig(t *testing.T) {
	t.Parallel()

	expectedOutputPattern := regexp.MustCompile(`^\d{2}:\d{2}:\d{2} GET / - 0\.0\.0\.0 - 200 - \d+(\.\d+)?.{1,3}
$`)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	LoggerConfigDefault.Output = buf

	loggers := []fiber.Handler{
		Logger(buf),
		Logger("15:04:05"),
		Logger("${time} ${method} ${path} - ${ip} - ${status} - ${latency}\n"),
		Logger(LoggerConfig{Output: buf}),
	}

	for _, logger := range loggers {
		buf.Reset()

		app := fiber.New()

		app.Use(logger)

		app.Get("/", func(_ *fiber.Ctx) {})

		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
		utils.AssertEqual(
			t,
			true,
			expectedOutputPattern.MatchString(buf.String()),
			fmt.Sprintf("Has: %s, expected pattern: %s", buf.String(), expectedOutputPattern.String()),
		)
	}
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
