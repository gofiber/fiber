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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, "0", resp.Header.Get(fiber.HeaderXXSSProtection))
	require.Equal(t, "nosniff", resp.Header.Get(fiber.HeaderXContentTypeOptions))
	require.Equal(t, "SAMEORIGIN", resp.Header.Get(fiber.HeaderXFrameOptions))
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentSecurityPolicy))
	require.Equal(t, "no-referrer", resp.Header.Get(fiber.HeaderReferrerPolicy))
	require.Equal(t, "", resp.Header.Get(fiber.HeaderPermissionsPolicy))
	require.Equal(t, "require-corp", resp.Header.Get("Cross-Origin-Embedder-Policy"))
	require.Equal(t, "same-origin", resp.Header.Get("Cross-Origin-Opener-Policy"))
	require.Equal(t, "same-origin", resp.Header.Get("Cross-Origin-Resource-Policy"))
	require.Equal(t, "?1", resp.Header.Get("Origin-Agent-Cluster"))
	require.Equal(t, "off", resp.Header.Get("X-DNS-Prefetch-Control"))
	require.Equal(t, "noopen", resp.Header.Get("X-Download-Options"))
	require.Equal(t, "none", resp.Header.Get("X-Permitted-Cross-Domain-Policies"))
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

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	// Assertions for custom header values
	require.Equal(t, "0", resp.Header.Get(fiber.HeaderXXSSProtection))
	require.Equal(t, "custom-nosniff", resp.Header.Get(fiber.HeaderXContentTypeOptions))
	require.Equal(t, "DENY", resp.Header.Get(fiber.HeaderXFrameOptions))
	require.Equal(t, "default-src 'none'", resp.Header.Get(fiber.HeaderContentSecurityPolicyReportOnly))
	require.Equal(t, "origin", resp.Header.Get(fiber.HeaderReferrerPolicy))
	require.Equal(t, "geolocation=(self)", resp.Header.Get(fiber.HeaderPermissionsPolicy))
	require.Equal(t, "custom-value", resp.Header.Get("Cross-Origin-Embedder-Policy"))
	require.Equal(t, "custom-value", resp.Header.Get("Cross-Origin-Opener-Policy"))
	require.Equal(t, "custom-value", resp.Header.Get("Cross-Origin-Resource-Policy"))
	require.Equal(t, "custom-value", resp.Header.Get("Origin-Agent-Cluster"))
	require.Equal(t, "custom-control", resp.Header.Get("X-DNS-Prefetch-Control"))
	require.Equal(t, "custom-options", resp.Header.Get("X-Download-Options"))
	require.Equal(t, "custom-policies", resp.Header.Get("X-Permitted-Cross-Domain-Policies"))
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

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	// Assertions for real-world header values
	require.Equal(t, "0", resp.Header.Get(fiber.HeaderXXSSProtection))
	require.Equal(t, "nosniff", resp.Header.Get(fiber.HeaderXContentTypeOptions))
	require.Equal(t, "SAMEORIGIN", resp.Header.Get(fiber.HeaderXFrameOptions))
	require.Equal(t, "default-src 'self';base-uri 'self';font-src 'self' https: data:;form-action 'self';frame-ancestors 'self';img-src 'self' data:;object-src 'none';script-src 'self';script-src-attr 'none';style-src 'self' https: 'unsafe-inline';upgrade-insecure-requests", resp.Header.Get(fiber.HeaderContentSecurityPolicy))
	require.Equal(t, "no-referrer", resp.Header.Get(fiber.HeaderReferrerPolicy))
	require.Equal(t, "geolocation=(self)", resp.Header.Get(fiber.HeaderPermissionsPolicy))
	require.Equal(t, "require-corp", resp.Header.Get("Cross-Origin-Embedder-Policy"))
	require.Equal(t, "same-origin", resp.Header.Get("Cross-Origin-Opener-Policy"))
	require.Equal(t, "same-origin", resp.Header.Get("Cross-Origin-Resource-Policy"))
	require.Equal(t, "?1", resp.Header.Get("Origin-Agent-Cluster"))
	require.Equal(t, "off", resp.Header.Get("X-DNS-Prefetch-Control"))
	require.Equal(t, "noopen", resp.Header.Get("X-Download-Options"))
	require.Equal(t, "none", resp.Header.Get("X-Permitted-Cross-Domain-Policies"))
}

func Test_Next(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		Next: func(ctx fiber.Ctx) bool {
			return ctx.Path() == "/next"
		},
		ReferrerPolicy: "no-referrer",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})
	app.Get("/next", func(c fiber.Ctx) error {
		return c.SendString("Skipped!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, "no-referrer", resp.Header.Get(fiber.HeaderReferrerPolicy))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/next", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, "microphone=()", resp.Header.Get(fiber.HeaderPermissionsPolicy))
}
