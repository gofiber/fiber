package hostauthorization

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// --- Config tests ---

func Test_ConfigDefault(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{
		AllowedHosts: []string{"example.com"},
	})
	require.NotNil(t, cfg.ErrorHandler)
	require.Equal(t, []string{"example.com"}, cfg.AllowedHosts)
}

func Test_ConfigPanicNoHostsOrFunc(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		configDefault(Config{})
	})
}

func Test_ConfigPanicEmptySlice(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		configDefault(Config{
			AllowedHosts: []string{},
		})
	})
}

func Test_ConfigPanicNoArgs(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		configDefault()
	})
}

func Test_ConfigAllowedHostsFuncOnly(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{
		AllowedHostsFunc: func(host string) bool {
			return host == "example.com"
		},
	})
	require.NotNil(t, cfg.AllowedHostsFunc)
}

func Test_ConfigPanicInvalidCIDR(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		New(Config{
			AllowedHosts: []string{"not-a-cidr/99"},
		})
	})
}

func Test_ConfigCustomErrorHandler(t *testing.T) {
	t.Parallel()

	custom := func(c fiber.Ctx, _ error) error {
		return c.Status(fiber.StatusTeapot).SendString("nope")
	}

	cfg := configDefault(Config{
		AllowedHosts: []string{"example.com"},
		ErrorHandler: custom,
	})
	require.NotNil(t, cfg.ErrorHandler)
}

// --- normalizeHost tests ---

func Test_NormalizeHost(t *testing.T) {
	t.Parallel()

	// normalizeHost receives output from c.Hostname() which already strips ports.
	// It handles trailing dots, IPv6 brackets, and lowercasing.
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain host", "example.com", "example.com"},
		{"uppercase", "EXAMPLE.COM", "example.com"},
		{"trailing dot", "example.com.", "example.com"},
		{"ipv4", "192.168.1.1", "192.168.1.1"},
		{"ipv6 brackets", "[::1]", "::1"},
		{"ipv6 bare", "::1", "::1"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, normalizeHost(tt.input))
		})
	}
}

// --- Matching logic tests ---

func Test_MatchExact(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"example.com", "api.myapp.com"})

	require.True(t, matchHost("example.com", parsed, nil))
	require.True(t, matchHost("api.myapp.com", parsed, nil))
	require.False(t, matchHost("evil.com", parsed, nil))
}

func Test_MatchExactCaseInsensitive(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"Example.COM"})

	require.True(t, matchHost("example.com", parsed, nil))
}

func Test_MatchSubdomainWildcard(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{".myapp.com"})

	require.True(t, matchHost("api.myapp.com", parsed, nil))
	require.True(t, matchHost("www.myapp.com", parsed, nil))
	require.True(t, matchHost("deep.sub.myapp.com", parsed, nil))
	require.False(t, matchHost("myapp.com", parsed, nil), "bare domain must NOT match subdomain wildcard")
	require.False(t, matchHost("evil.com", parsed, nil))
}

func Test_MatchCIDRv4(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"10.0.0.0/8"})

	require.True(t, matchHost("10.0.50.3", parsed, nil))
	require.True(t, matchHost("10.255.255.255", parsed, nil))
	require.False(t, matchHost("192.168.1.1", parsed, nil))
	require.False(t, matchHost("169.254.169.254", parsed, nil))
}

func Test_MatchCIDRv6(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"fd00::/8"})

	require.True(t, matchHost("fd00::1", parsed, nil))
	require.False(t, matchHost("2001:db8::1", parsed, nil))
}

func Test_MatchAllowedHostsFunc(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"example.com"})
	fn := func(host string) bool {
		return host == "dynamic.com"
	}

	require.True(t, matchHost("example.com", parsed, fn))
	require.True(t, matchHost("dynamic.com", parsed, fn))
	require.False(t, matchHost("evil.com", parsed, fn))
}

func Test_MatchMixedCategories(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{
		"example.com",
		".myapp.com",
		"10.0.0.0/8",
		"127.0.0.1",
	})

	require.True(t, matchHost("example.com", parsed, nil))
	require.True(t, matchHost("api.myapp.com", parsed, nil))
	require.True(t, matchHost("10.0.50.3", parsed, nil))
	require.True(t, matchHost("127.0.0.1", parsed, nil))
	require.False(t, matchHost("evil.com", parsed, nil))
	require.False(t, matchHost("192.168.1.1", parsed, nil))
}

func Test_MatchEmptyHost(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"example.com"})

	require.False(t, matchHost("", parsed, nil))
}

// --- Integration tests ---

func Test_HostAuthorization_AllowedHost(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"example.com"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "example.com"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_HostAuthorization_RejectedHost(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"example.com"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "evil.com"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func Test_HostAuthorization_EmptyHost(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"example.com"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = ""

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func Test_HostAuthorization_HostWithPort(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"example.com"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "example.com:8080"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_HostAuthorization_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"example.com"},
		Next: func(c fiber.Ctx) bool {
			return c.Path() == "/healthz"
		},
	}))

	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/healthz", http.NoBody)
	req.Host = "evil.com"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_HostAuthorization_SubdomainWildcard(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{".myapp.com"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Subdomain should be allowed
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "api.myapp.com"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Bare domain should be rejected
	req2 := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req2.Host = "myapp.com"

	resp2, err := app.Test(req2)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp2.StatusCode)
}

func Test_HostAuthorization_CIDR(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"10.0.0.0/8"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "10.0.50.3"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Cloud metadata IP should be rejected
	req2 := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req2.Host = "169.254.169.254"

	resp2, err := app.Test(req2)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp2.StatusCode)
}

func Test_HostAuthorization_AllowedHostsFunc(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHostsFunc: func(host string) bool {
			return host == "dynamic.com"
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "dynamic.com"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	req2 := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req2.Host = "evil.com"

	resp2, err := app.Test(req2)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp2.StatusCode)
}

func Test_HostAuthorization_CustomErrorHandler(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"example.com"},
		ErrorHandler: func(c fiber.Ctx, err error) error {
			require.ErrorIs(t, err, ErrForbiddenHost)
			return c.Status(fiber.StatusTeapot).SendString("custom rejection")
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "evil.com"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
}

func Test_HostAuthorization_CaseInsensitive(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"Example.COM"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "EXAMPLE.com"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_HostAuthorization_TrailingDot(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"example.com"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "example.com."

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_HostAuthorization_ExactIP(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"127.0.0.1"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "127.0.0.1"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_HostAuthorization_OverlappingRules(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{
			"api.myapp.com",
			".myapp.com",
			"10.0.0.0/8",
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Matches both exact and wildcard
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "api.myapp.com"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_HostAuthorization_IPv6Brackets(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"fd00::/8"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "[fd00::1]"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_HostAuthorization_XForwardedHost_TrustProxy(t *testing.T) {
	t.Parallel()

	// With TrustProxy enabled, X-Forwarded-Host should be used
	// app.Test() uses remote address 0.0.0.0, so we trust that proxy IP
	app := fiber.New(fiber.Config{
		TrustProxy: true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies: []string{"0.0.0.0"},
		},
	})

	app.Use(New(Config{
		AllowedHosts: []string{"example.com"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Allowed via X-Forwarded-Host
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "proxy.internal"
	req.Header.Set("X-Forwarded-Host", "example.com")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Rejected: X-Forwarded-Host is unauthorized
	req2 := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req2.Host = "example.com"
	req2.Header.Set("X-Forwarded-Host", "evil.com")

	resp2, err := app.Test(req2)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp2.StatusCode)
}

func Test_HostAuthorization_XForwardedHost_NoTrustProxy(t *testing.T) {
	t.Parallel()

	// Without TrustProxy, X-Forwarded-Host should be ignored
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"example.com"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "example.com"
	req.Header.Set("X-Forwarded-Host", "evil.com")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "X-Forwarded-Host should be ignored without TrustProxy")
}

// --- Benchmarks ---

func Benchmark_HostAuthorization_ExactMatch(b *testing.B) {
	app := fiber.New()
	app.Use(New(Config{
		AllowedHosts: []string{"example.com"},
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "example.com"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := app.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close() //nolint:errcheck // benchmark cleanup
	}
}

func Benchmark_HostAuthorization_CIDR(b *testing.B) {
	app := fiber.New()
	app.Use(New(Config{
		AllowedHosts: []string{"10.0.0.0/8"},
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "10.0.50.3"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := app.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close() //nolint:errcheck // benchmark cleanup
	}
}

func Benchmark_HostAuthorization_Mixed(b *testing.B) {
	app := fiber.New()
	app.Use(New(Config{
		AllowedHosts: []string{
			"example.com",
			".myapp.com",
			"10.0.0.0/8",
			"127.0.0.1",
		},
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "api.myapp.com"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := app.Test(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close() //nolint:errcheck // benchmark cleanup
	}
}
