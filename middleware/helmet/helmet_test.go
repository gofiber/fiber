// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://docs.gofiber.io/
// 📝 Github Repository: https://github.com/gofiber/fiber

package helmet

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/utils"
)

func Test_Default(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "1; mode=block", resp.Header.Get(fiber.HeaderXXSSProtection))
	utils.AssertEqual(t, "nosniff", resp.Header.Get(fiber.HeaderXContentTypeOptions))
	utils.AssertEqual(t, "SAMEORIGIN", resp.Header.Get(fiber.HeaderXFrameOptions))
	utils.AssertEqual(t, "", resp.Header.Get(fiber.HeaderContentSecurityPolicy))
	utils.AssertEqual(t, "", resp.Header.Get(fiber.HeaderReferrerPolicy))
	utils.AssertEqual(t, "", resp.Header.Get(fiber.HeaderPermissionsPolicy))
}

func Test_Filter(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		Filter: func(ctx *fiber.Ctx) bool {
			return ctx.Path() == "/filter"
		},
		ReferrerPolicy: "no-referrer",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})
	app.Get("/filter", func(c *fiber.Ctx) error {
		return c.SendString("Skipped!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "no-referrer", resp.Header.Get(fiber.HeaderReferrerPolicy))

	resp, err = app.Test(httptest.NewRequest("GET", "/filter", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "", resp.Header.Get(fiber.HeaderReferrerPolicy))
}

func Test_ContentSecurityPolicy(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		ContentSecurityPolicy: "default-src 'none'",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "default-src 'none'", resp.Header.Get(fiber.HeaderContentSecurityPolicy))
}

func Test_ContentSecurityPolicyReportOnly(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		ContentSecurityPolicy: "default-src 'none'",
		CSPReportOnly:         true,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "default-src 'none'", resp.Header.Get(fiber.HeaderContentSecurityPolicyReportOnly))
	utils.AssertEqual(t, "", resp.Header.Get(fiber.HeaderContentSecurityPolicy))
}

func Test_PermissionsPolicy(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		PermissionPolicy: "microphone=()",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "microphone=()", resp.Header.Get(fiber.HeaderPermissionsPolicy))
}
