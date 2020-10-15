package proxy

import (
	"io/ioutil"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

// go test -run Test_Proxy_Empty_Host
func Test_Proxy_Empty_Upstream_Servers(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			utils.AssertEqual(t, "Servers cannot be empty", r)
		}
	}()
	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{}}))
}

// go test -run Test_Proxy_Next
func Test_Proxy_Next(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Servers: []string{"127.0.0.1"},
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}

// go test -run Test_Proxy
func Test_Proxy(t *testing.T) {
	target := fiber.New()

	target.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	go func() {
		utils.AssertEqual(t, nil, target.Listen(":3001"))
	}()

	time.Sleep(time.Second)

	resp, err := target.Test(httptest.NewRequest("GET", "/", nil), 2000)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusTeapot, resp.StatusCode)

	app := fiber.New()

	app.Use(New(Config{Servers: []string{"127.0.0.1:3001"}}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "127.0.0.1:3001"
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusTeapot, resp.StatusCode)
}

// go test -run Test_Proxy_Do_With_Error
func Test_Proxy_Do_With_Error(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{Servers: []string{"127.0.0.1:90000"}}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusInternalServerError, resp.StatusCode)

	b, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(b), "127.0.0.1:90000"))
}

func Test_Proxy_Forward(t *testing.T) {
	app := fiber.New()

	target := fiber.New(fiber.Config{DisableStartupMessage: true})
	go func() {
		utils.AssertEqual(t, nil, target.Listen(":50001"))
	}()
	target.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("forwarded")
	})

	app.Use(Forward("http://127.0.0.1:50001"))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	b, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "forwarded", string(b))
}

func Test_Proxy_Modify_Response(t *testing.T) {
	target := fiber.New(fiber.Config{DisableStartupMessage: true})
	go func() {
		utils.AssertEqual(t, nil, target.Listen(":50002"))
	}()

	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{"127.0.0.1:50002"},
		ModifyResponse: func(c *fiber.Ctx) error {
			c.Response().SetStatusCode(fiber.StatusOK)
			return c.SendString("modified response")
		},
	}))

	target.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("not modified")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	b, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "modified response", string(b))
}

func Test_Proxy_Modify_Request(t *testing.T) {
	target := fiber.New(fiber.Config{DisableStartupMessage: true})
	go func() {
		utils.AssertEqual(t, nil, target.Listen(":50003"))
	}()

	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{"127.0.0.1:50003"},
		ModifyRequest: func(c *fiber.Ctx) error {
			c.Request().SetBody([]byte("modified request"))
			return nil
		},
	}))

	target.Get("/", func(c *fiber.Ctx) error {
		b := c.Request().Body()
		return c.SendString(string(b))
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	b, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "modified request", string(b))
}
