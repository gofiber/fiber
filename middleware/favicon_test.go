package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
)

// go test -run Test_Middleware_Favicon
func Test_Middleware_Favicon(t *testing.T) {
	app := fiber.New()

	app.Use(Favicon())

	app.Get("/", func(ctx *fiber.Ctx) {
		ctx.Send("Hello?")
	})

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
