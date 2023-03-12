package skip_test

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/skip"
	"github.com/gofiber/fiber/v2/utils"
)

// go test -run Test_Skip
func Test_Skip(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(skip.New(errTeapotHandler, func(*fiber.Ctx) bool { return true }))
	app.Get("/", helloWorldHandler)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
}

// go test -run Test_SkipFalse
func Test_SkipFalse(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(skip.New(errTeapotHandler, func(*fiber.Ctx) bool { return false }))
	app.Get("/", helloWorldHandler)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusTeapot, resp.StatusCode)
}

// go test -run Test_SkipNilFunc
func Test_SkipNilFunc(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(skip.New(errTeapotHandler, nil))
	app.Get("/", helloWorldHandler)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusTeapot, resp.StatusCode)
}

func helloWorldHandler(c *fiber.Ctx) error {
	return c.SendString("Hello, World 👋!")
}

func errTeapotHandler(*fiber.Ctx) error {
	return fiber.ErrTeapot
}
