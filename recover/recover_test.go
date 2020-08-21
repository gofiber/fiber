package recover

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
)

// go test -run Test_Recover
func Test_Recover(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Get("/panic", func(c *fiber.Ctx) error {
		panic("Hi, I'm an error!")
	})

	app.Errors(func(c *fiber.Ctx, err error) {
		utils.AssertEqual(t, "Hi, I'm an error!", err.Error())
		c.SendStatus(fiber.StatusTeapot)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/panic", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusTeapot, resp.StatusCode)
}
