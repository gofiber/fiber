package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Test_CORS_Security_CredentialsWithSubdomainWildcard ensures that when
// credentials are allowed together with a wildcard-subdomain origin pattern, a
// matching origin is reflected verbatim (never "*") alongside the credentials
// header, while a non-matching origin receives neither.
func Test_CORS_Security_CredentialsWithSubdomainWildcard(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		AllowCredentials: true,
		AllowOrigins:     []string{"https://*.example.com"},
	}))

	handler := app.Handler()

	cases := []struct {
		name          string
		origin        string
		expectOrigin  string
		expectCredHdr string
	}{
		{
			name:          "matching subdomain reflected with credentials",
			origin:        "https://api.example.com",
			expectOrigin:  "https://api.example.com",
			expectCredHdr: "true",
		},
		{
			name:          "non-matching origin gets nothing",
			origin:        "https://attacker.evil.com",
			expectOrigin:  "",
			expectCredHdr: "",
		},
		{
			name:          "apex domain does not match wildcard",
			origin:        "https://example.com",
			expectOrigin:  "",
			expectCredHdr: "",
		},
		{
			name:          "scheme downgrade does not match",
			origin:        "http://api.example.com",
			expectOrigin:  "",
			expectCredHdr: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := &fasthttp.RequestCtx{}
			ctx.Request.SetRequestURI("/")
			ctx.Request.Header.SetMethod(fiber.MethodOptions)
			ctx.Request.Header.Set(fiber.HeaderAccessControlRequestMethod, fiber.MethodGet)
			ctx.Request.Header.Set(fiber.HeaderOrigin, tc.origin)

			handler(ctx)

			gotOrigin := string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowOrigin))
			gotCred := string(ctx.Response.Header.Peek(fiber.HeaderAccessControlAllowCredentials))
			require.Equal(t, tc.expectOrigin, gotOrigin)
			require.Equal(t, tc.expectCredHdr, gotCred)
			require.NotEqual(t, "*", gotOrigin, "credentialed responses must never use the wildcard")
		})
	}
}

// Test_CORS_Security_CredentialsSimpleRequest ensures the credentialed-origin
// reflection also applies to non-preflight (simple) requests, not just OPTIONS
// preflights.
func Test_CORS_Security_CredentialsSimpleRequest(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		AllowCredentials: true,
		AllowOrigins:     []string{"https://trusted.example.com"},
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	// Allowed origin is reflected with credentials on a simple GET.
	allowed := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	allowed.Header.Set(fiber.HeaderOrigin, "https://trusted.example.com")
	resp, err := app.Test(allowed)
	require.NoError(t, err)
	require.Equal(t, "https://trusted.example.com", resp.Header.Get(fiber.HeaderAccessControlAllowOrigin))
	require.Equal(t, "true", resp.Header.Get(fiber.HeaderAccessControlAllowCredentials))

	// Disallowed origin is reflected nowhere.
	denied := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	denied.Header.Set(fiber.HeaderOrigin, "https://evil.example.org")
	resp, err = app.Test(denied)
	require.NoError(t, err)
	require.Empty(t, resp.Header.Get(fiber.HeaderAccessControlAllowOrigin))
	require.Empty(t, resp.Header.Get(fiber.HeaderAccessControlAllowCredentials))
}

// Test_CORS_Security_NoOriginReflectionForDisallowed ensures a disallowed origin
// is never reflected back even when many origins are configured — the response
// must not echo an arbitrary attacker-controlled Origin.
func Test_CORS_Security_NoOriginReflectionForDisallowed(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		AllowOrigins: []string{"https://a.example.com", "https://b.example.com"},
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderOrigin, "https://attacker.example.net")
	resp, err := app.Test(req)
	require.NoError(t, err)

	got := resp.Header.Get(fiber.HeaderAccessControlAllowOrigin)
	require.Empty(t, got)
	require.NotEqual(t, "https://attacker.example.net", got)
}
