package logger

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/bytebufferpool"
	"github.com/gofiber/fiber/v2/utils"
)

// go test -run Test_Logger
func Test_Logger(t *testing.T) {
	app := fiber.New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Use(New(Config{
		Format: "${error}",
		Output: buf,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return errors.New("some random error")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusInternalServerError, resp.StatusCode)
	utils.AssertEqual(t, "some random error", buf.String())
}

// go test -run Test_Logger_Next
func Test_Logger_Next(t *testing.T) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})
	app.Use(New(Config{
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}
