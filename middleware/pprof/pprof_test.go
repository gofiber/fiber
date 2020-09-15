package pprof

import (
	"bytes"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"io/ioutil"
	"net/http/httptest"
	"testing"
)

func Test_Non_Pprof_Path(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("escaped")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	b, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "escaped", string(b))
}

func Test_Pprof_Index(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("escaped")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/debug/pprof/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
	utils.AssertEqual(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	b, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, bytes.Contains(b, []byte("<title>/debug/pprof/</title>")))
}

func Test_Pprof_Subs(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("escaped")
	})

	subs := []string{
		"cmdline", "profile", "symbol", "trace", "allocs", "block",
		"goroutine", "heap", "mutex", "threadcreate",
	}

	for _, sub := range subs {
		t.Run(sub, func(t *testing.T) {
			target := "/debug/pprof/" + sub
			if sub == "profile" {
				target += "?seconds=1"
			}
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, target, nil), 5000)
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, 200, resp.StatusCode)
		})
	}
}

func Test_Pprof_Other(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("escaped")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/debug/pprof/302", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 302, resp.StatusCode)
}
