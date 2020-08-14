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

	app.Use(func(c *fiber.Ctx, err error) error {
		utils.AssertEqual(t, "Hi, I'm an error!", err.Error())
		return c.SendStatus(500)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/panic", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 500, resp.StatusCode, "Status code")
}
