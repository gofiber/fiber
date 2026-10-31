package skip_test

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/middleware/skip"
)

func Test_Skip(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		skip   func(fiber.Ctx) bool
		name   string
		status int
	}{
		{name: "skip", skip: func(fiber.Ctx) bool { return true }, status: fiber.StatusOK},
		{name: "skip false", skip: func(fiber.Ctx) bool { return false }, status: fiber.StatusTeapot},
		{name: "nil func", skip: nil, status: fiber.StatusTeapot},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(skip.New(errTeapotHandler, tc.skip))
			app.Get("/", helloWorldHandler)

			apptest.Expect(t, app, "/", tc.status)
		})
	}
}

func helloWorldHandler(c fiber.Ctx) error {
	return c.SendString("Hello, World 👋!")
}

func errTeapotHandler(fiber.Ctx) error {
	return fiber.ErrTeapot
}
