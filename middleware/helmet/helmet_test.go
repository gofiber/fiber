package helmet

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

func Test_Default(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "0", resp.Header.Get(fiber.HeaderXXSSProtection))
	utils.AssertEqual(t, "nosniff", resp.Header.Get(fiber.HeaderXContentTypeOptions))
	utils.AssertEqual(t, "SAMEORIGIN", resp.Header.Get(fiber.HeaderXFrameOptions))
	utils.AssertEqual(t, "", resp.Header.Get(fiber.HeaderContentSecurityPolicy))
	utils.AssertEqual(t, "no-referrer", resp.Header.Get(fiber.HeaderReferrerPolicy))
	utils.AssertEqual(t, "", resp.Header.Get(fiber.HeaderPermissionsPolicy))
	utils.AssertEqual(t, "require-corp", resp.Header.Get("Cross-Origin-Embedder-Policy"))
	utils.AssertEqual(t, "same-origin", resp.Header.Get("Cross-Origin-Opener-Policy"))
	utils.AssertEqual(t, "same-origin", resp.Header.Get("Cross-Origin-Resource-Policy"))
	utils.AssertEqual(t, "?1", resp.Header.Get("Origin-Agent-Cluster"))
	utils.AssertEqual(t, "off", resp.Header.Get("X-DNS-Prefetch-Control"))
	utils.AssertEqual(t, "noopen", resp.Header.Get("X-Download-Options"))
	utils.AssertEqual(t, "none", resp.Header.Get("X-Permitted-Cross-Domain-Policies"))
}

func Test_CustomValues_AllHeaders(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		// Custom values for all headers
		XSSProtection:             "0",
		ContentTypeNosniff:        "custom-nosniff",
		XFrameOptions:             "DENY",
		HSTSExcludeSubdomains:     true,
		ContentSecurityPolicy:     "default-src 'none'",
		CSPReportOnly:             true,
		HSTSPreloadEnabled:        true,
		ReferrerPolicy:            "origin",
		PermissionPolicy:          "geolocation=(self)",
		CrossOriginEmbedderPolicy: "custom-value",
		CrossOriginOpenerPolicy:   "custom-value",
		CrossOriginResourcePolicy: "custom-value",
		OriginAgentCluster:        "custom-value",
		XDNSPrefetchControl:       "custom-control",
		XDownloadOptions:          "custom-options",
		XPermittedCrossDomain:     "custom-policies",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	// Assertions for custom header values
	utils.AssertEqual(t, "0", resp.Header.Get(fiber.HeaderXXSSProtection))
	utils.AssertEqual(t, "custom-nosniff", resp.Header.Get(fiber.HeaderXContentTypeOptions))
	utils.AssertEqual(t, "DENY", resp.Header.Get(fiber.HeaderXFrameOptions))
	utils.AssertEqual(t, "default-src 'none'", resp.Header.Get(fiber.HeaderContentSecurityPolicyReportOnly))
	utils.AssertEqual(t, "origin", resp.Header.Get(fiber.HeaderReferrerPolicy))
	utils.AssertEqual(t, "geolocation=(self)", resp.Header.Get(fiber.HeaderPermissionsPolicy))
	utils.AssertEqual(t, "custom-value", resp.Header.Get("Cross-Origin-Embedder-Policy"))
	utils.AssertEqual(t, "custom-value", resp.Header.Get("Cross-Origin-Opener-Policy"))
	utils.AssertEqual(t, "custom-value", resp.Header.Get("Cross-Origin-Resource-Policy"))
	utils.AssertEqual(t, "custom-value", resp.Header.Get("Origin-Agent-Cluster"))
	utils.AssertEqual(t, "custom-control", resp.Header.Get("X-DNS-Prefetch-Control"))
	utils.AssertEqual(t, "custom-options", resp.Header.Get("X-Download-Options"))
	utils.AssertEqual(t, "custom-policies", resp.Header.Get("X-Permitted-Cross-Domain-Policies"))
}

func Test_RealWorldValues_AllHeaders(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		// Real-world values for all headers
		XSSProtection:             "0",
		ContentTypeNosniff:        "nosniff",
		XFrameOptions:             "SAMEORIGIN",
		HSTSExcludeSubdomains:     false,
		ContentSecurityPolicy:     "default-src 'self';base-uri 'self';font-src 'self' https: data:;form-action 'self';frame-ancestors 'self';img-src 'self' data:;object-src 'none';script-src 'self';script-src-attr 'none';style-src 'self' https: 'unsafe-inline';upgrade-insecure-requests",
		CSPReportOnly:             false,
		HSTSPreloadEnabled:        true,
		ReferrerPolicy:            "no-referrer",
		PermissionPolicy:          "geolocation=(self)",
		CrossOriginEmbedderPolicy: "require-corp",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
		OriginAgentCluster:        "?1",
		XDNSPrefetchControl:       "off",
		XDownloadOptions:          "noopen",
		XPermittedCrossDomain:     "none",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	// Assertions for real-world header values
	utils.AssertEqual(t, "0", resp.Header.Get(fiber.HeaderXXSSProtection))
	utils.AssertEqual(t, "nosniff", resp.Header.Get(fiber.HeaderXContentTypeOptions))
	utils.AssertEqual(t, "SAMEORIGIN", resp.Header.Get(fiber.HeaderXFrameOptions))
	utils.AssertEqual(t, "default-src 'self';base-uri 'self';font-src 'self' https: data:;form-action 'self';frame-ancestors 'self';img-src 'self' data:;object-src 'none';script-src 'self';script-src-attr 'none';style-src 'self' https: 'unsafe-inline';upgrade-insecure-requests", resp.Header.Get(fiber.HeaderContentSecurityPolicy))
	utils.AssertEqual(t, "no-referrer", resp.Header.Get(fiber.HeaderReferrerPolicy))
	utils.AssertEqual(t, "geolocation=(self)", resp.Header.Get(fiber.HeaderPermissionsPolicy))
	utils.AssertEqual(t, "require-corp", resp.Header.Get("Cross-Origin-Embedder-Policy"))
	utils.AssertEqual(t, "same-origin", resp.Header.Get("Cross-Origin-Opener-Policy"))
	utils.AssertEqual(t, "same-origin", resp.Header.Get("Cross-Origin-Resource-Policy"))
	utils.AssertEqual(t, "?1", resp.Header.Get("Origin-Agent-Cluster"))
	utils.AssertEqual(t, "off", resp.Header.Get("X-DNS-Prefetch-Control"))
	utils.AssertEqual(t, "noopen", resp.Header.Get("X-Download-Options"))
	utils.AssertEqual(t, "none", resp.Header.Get("X-Permitted-Cross-Domain-Policies"))
}

func Test_Next(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		Next: func(ctx *fiber.Ctx) bool {
			return ctx.Path() == "/next"
		},
		ReferrerPolicy: "no-referrer",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})
	app.Get("/next", func(c *fiber.Ctx) error {
		return c.SendString("Skipped!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "no-referrer", resp.Header.Get(fiber.HeaderReferrerPolicy))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/next", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "microphone=()", resp.Header.Get(fiber.HeaderPermissionsPolicy))
}
