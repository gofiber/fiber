package proxy

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber"
	"github.com/gofiber/fiber/utils"
)

// go test -run Test_Proxy
func Test_Proxy(t *testing.T) {
	go func() {
		app2 := fiber.New(fiber.Config{
			DisableStartupMessage: true,
		})
		app2.Get("/", func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusTeapot)
		})

		utils.AssertEqual(t, nil, app2.Listen(":3001"))
	}()

	time.Sleep(1 * time.Second)

	go func() {
		app1 := fiber.New(fiber.Config{
			DisableStartupMessage: true,
		})
		app1.Use(New(Config{
			Hosts: "127.0.0.1:3001",
		}))
		resp, err := app1.Test(httptest.NewRequest("GET", "/", nil))
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusTeapot, resp.StatusCode)
	}()
}
