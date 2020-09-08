package proxy

import (
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber"
	"github.com/gofiber/fiber/utils"
)

// go test -run Test_Proxy
func Test_Proxy(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		fmt.Println("1")
		app := fiber.New(fiber.Config{
			DisableStartupMessage: true,
		})

		app.Get("/", func(c *fiber.Ctx) error {
			fmt.Println(c)
			fmt.Println(c.Request())
			fmt.Println(c.Request().String())
			c.SendStatus(fiber.StatusTeapot)
			fmt.Println(c.Request().Response.StatusCode())
			return nil
		})

		utils.AssertEqual(t, nil, app.Listen(":3001"))
	}()

	time.Sleep(2 * time.Second)

	go func() {
		fmt.Println("2")
		defer wg.Done()

		app := fiber.New(fiber.Config{
			DisableStartupMessage: true,
		})
		app.Use(New(Config{
			Hosts: "localhost:3001",
		}))

		resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusTeapot, resp.StatusCode)
	}()

	wg.Wait()
}
