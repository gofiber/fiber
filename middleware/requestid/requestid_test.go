package requestid

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/utils"
)

// go test -run Test_RequestID
func Test_RequestID(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World 👋!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	reqid := resp.Header.Get(fiber.HeaderXRequestID)
	utils.AssertEqual(t, 36, len(reqid))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Add(fiber.HeaderXRequestID, reqid)

	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, reqid, resp.Header.Get(fiber.HeaderXRequestID))
}
