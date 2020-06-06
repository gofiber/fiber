package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
)

// go test -run Test_Middleware_Helmet
func Test_Middleware_Helmet(t *testing.T) {
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
