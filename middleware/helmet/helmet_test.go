// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://docs.gofiber.io/
// 📝 Github Repository: https://github.com/gofiber/fiber

package helmet

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func Test_Default(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, "1; mode=block", resp.Header.Get(fiber.HeaderXXSSProtection))
	require.Equal(t, "nosniff", resp.Header.Get(fiber.HeaderXContentTypeOptions))
	require.Equal(t, "SAMEORIGIN", resp.Header.Get(fiber.HeaderXFrameOptions))
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentSecurityPolicy))
	require.Equal(t, "", resp.Header.Get(fiber.HeaderReferrerPolicy))
	require.Equal(t, "", resp.Header.Get(fiber.HeaderPermissionsPolicy))
}

func Test_Filter(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		Filter: func(c fiber.Ctx) bool {
			return c.Path() == "/filter"
		},
		ReferrerPolicy: "no-referrer",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})
	app.Get("/filter", func(c fiber.Ctx) error {
		return c.SendString("Skipped!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, "no-referrer", resp.Header.Get(fiber.HeaderReferrerPolicy))

	resp, err = app.Test(httptest.NewRequest("GET", "/filter", nil))
	require.NoError(t, err)
	require.Equal(t, "", resp.Header.Get(fiber.HeaderReferrerPolicy))
}

func Test_ContentSecurityPolicy(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		ContentSecurityPolicy: "default-src 'none'",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, "default-src 'none'", resp.Header.Get(fiber.HeaderContentSecurityPolicy))
}

func Test_ContentSecurityPolicyReportOnly(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		ContentSecurityPolicy: "default-src 'none'",
		CSPReportOnly:         true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, "default-src 'none'", resp.Header.Get(fiber.HeaderContentSecurityPolicyReportOnly))
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentSecurityPolicy))
}

func Test_PermissionsPolicy(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		PermissionPolicy: "microphone=()",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, "microphone=()", resp.Header.Get(fiber.HeaderPermissionsPolicy))
}
