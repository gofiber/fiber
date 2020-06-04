package middleware

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
	"github.com/valyala/bytebufferpool"
)

// go test -run Test_Middleware_Logger
func Test_Middleware_Logger(t *testing.T) {
	format := "${ip}-${ips}-${url}-${host}-${method}-${path}-${protocol}-${route}-${referer}-${ua}-${latency}-${status}-${body}-${error}-${bytesSent}-${bytesReceived}-${header:header}-${query:query}-${cookie:cookie}"
	expect := "0.0.0.0--/?query=query-example.com-GET-/-http-/-ref-ua-0s-500--error-5-0-header-query-cookie"

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(LoggerWithConfig(LoggerConfig{
		Format: format,
		Output: buf,
	}))

	app.Get("/", func(ctx *fiber.Ctx) {
		ctx.Next(errors.New("error"))
	})

	req := httptest.NewRequest("GET", "/?query=query", nil)
	req.Header.Set("header", "header")
	req.Header.Set("Cookie", "cookie=cookie")
	req.Header.Set("User-Agent", "ua")
	req.Header.Set("Referer", "ref")

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 500, resp.StatusCode, "Status code")
	utils.AssertEqual(t, expect, buf.String())

}
