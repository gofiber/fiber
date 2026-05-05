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

func Test_ConfigPanicInvalidCIDRFormat(t *testing.T) {
	t.Parallel()

	// Any entry containing "/" is treated as a CIDR attempt. A broken CIDR like
	// "not-a-cidr/99" that fails net.ParseCIDR panics at startup so the
	// misconfiguration is immediately visible instead of silently becoming an
	// exact entry that can never match a real host.
	require.Panics(t, func() {
		New(Config{
			AllowedHosts: []string{"not-a-cidr/99"},
		})
	})
}

func Test_ConfigPanicNonCanonicalCIDR(t *testing.T) {
	t.Parallel()

	// "10.0.0.5/8" passes net.ParseCIDR but has host bits set — it would silently
	// expand to 10.0.0.0/8, allowing far more than the developer intended.
	require.Panics(t, func() {
		New(Config{
			AllowedHosts: []string{"10.0.0.5/8"},
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
		{"host with port", "example.com:8080", "example.com"},
		{"uppercase host with port", "EXAMPLE.COM:8080", "example.com"},
		{"ipv4", "192.168.1.1", "192.168.1.1"},
		{"ipv4 with port", "192.168.1.1:8080", "192.168.1.1"},
		{"ipv6 brackets", "[::1]", "::1"},
		{"ipv6 bare", "::1", "::1"},
		{"ipv6 with port", "[::1]:8080", "::1"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, normalizeHost(tt.input))
		})
	}
}

func Test_ParseAllowedHosts_SkipsBlankEntries(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"", "   ", ".", "example.com"})

	require.True(t, matchHost("example.com", parsed, nil))
	require.False(t, matchHost("", parsed, nil))
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

	// app.Test() injects req.Host = "localhost" when req.Host is empty (app.go:1217).
	// fasthttp then receives "localhost" as the Host header, which is not in AllowedHosts.
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

// Test_HostAuthorization_AllowedHostWithPort verifies that configuring AllowedHosts
// with an explicit port (e.g. "example.com:8080") still matches correctly.
// c.Hostname() strips ports from the request Host header, so the AllowedHosts
// entry must also have its port stripped during config parsing.
func Test_HostAuthorization_AllowedHostWithPort(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"example.com:8080"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Request with matching host (port in Host header is stripped by c.Hostname())
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "example.com:8080"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Request without port should also match (both normalize to "example.com")
	req2 := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req2.Host = "example.com"

	resp2, err := app.Test(req2)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp2.StatusCode)
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

func Test_HostAuthorization_SubdomainWildcard_Allowed(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{".myapp.com"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "api.myapp.com"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_HostAuthorization_SubdomainWildcard_BareDomainRejected(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{".myapp.com"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "myapp.com"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func Test_HostAuthorization_CIDR_Allowed(t *testing.T) {
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
}

func Test_HostAuthorization_CIDR_CloudMetadataRejected(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"10.0.0.0/8"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "169.254.169.254"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func Test_HostAuthorization_AllowedHostsFunc_Allowed(t *testing.T) {
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
}

func Test_HostAuthorization_AllowedHostsFunc_Rejected(t *testing.T) {
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
	req.Host = "evil.com"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func Test_HostAuthorization_CustomErrorHandler(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	var handlerErr error
	app.Use(New(Config{
		AllowedHosts: []string{"example.com"},
		ErrorHandler: func(c fiber.Ctx, err error) error {
			handlerErr = err
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
	require.ErrorIs(t, handlerErr, ErrForbiddenHost)
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

func Test_HostAuthorization_XForwardedHost_TrustProxy_Allowed(t *testing.T) {
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

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "proxy.internal"
	req.Header.Set("X-Forwarded-Host", "example.com")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_HostAuthorization_XForwardedHost_TrustProxy_Rejected(t *testing.T) {
	t.Parallel()

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

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "example.com"
	req.Header.Set("X-Forwarded-Host", "evil.com")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
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

func Test_ErrForbiddenHostString(t *testing.T) {
	t.Parallel()

	// Lock in the exported error message text so callers who match on
	// err.Error() are notified of any future change via a failing test.
	require.Equal(t, "hostauthorization: forbidden host", ErrForbiddenHost.Error())
}

func Test_AllowedHostsFuncFallback(t *testing.T) {
	t.Parallel()

	// AllowedHostsFunc must be called only when no static rule matches.
	called := 0
	parsed := parseAllowedHosts([]string{"example.com"})
	fn := func(_ string) bool {
		called++
		return false
	}

	// Static match — func must not be invoked.
	result := matchHost("example.com", parsed, fn)
	require.True(t, result)
	require.Equal(t, 0, called, "AllowedHostsFunc must not be called when a static host matches")

	// No static match — func should be invoked as fallback.
	result = matchHost("other.com", parsed, fn)
	require.False(t, result)
	require.Equal(t, 1, called, "AllowedHostsFunc must be called when no static rule matches")
}

func Test_NormalizeHost_IPv6WithPortInConfig(t *testing.T) {
	t.Parallel()

	// An AllowedHosts entry of "[::1]:8080" should normalize to "::1" and
	// match a request whose Host header is "[::1]:8080" (or just "[::1]").
	parsed := parseAllowedHosts([]string{"[::1]:8080"})

	require.True(t, matchHost("::1", parsed, nil))
	require.False(t, matchHost("::2", parsed, nil))
}

// --- Benchmarks ---

// --- Low-level matchHost benchmarks (isolate matching cost from HTTP pipeline) ---

func Benchmark_matchHost_ExactMatch(b *testing.B) {
	parsed := parseAllowedHosts([]string{"example.com"})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchHost("example.com", parsed, nil)
	}
}

func Benchmark_matchHost_WildcardMatch(b *testing.B) {
	parsed := parseAllowedHosts([]string{".myapp.com"})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchHost("api.myapp.com", parsed, nil)
	}
}

func Benchmark_matchHost_CIDRMatch(b *testing.B) {
	parsed := parseAllowedHosts([]string{"10.0.0.0/8"})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchHost("10.0.50.3", parsed, nil)
	}
}

func Benchmark_matchHost_Mixed(b *testing.B) {
	parsed := parseAllowedHosts([]string{
		"example.com",
		".myapp.com",
		"10.0.0.0/8",
		"127.0.0.1",
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchHost("api.myapp.com", parsed, nil)
	}
}

// --- Full HTTP pipeline benchmarks (includes app.Test() overhead) ---

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

// --- Fuzz targets ---

func FuzzNormalizeHost(f *testing.F) {
	f.Add("example.com")
	f.Add("example.com.")
	f.Add("[::1]:8080")
	f.Add("[::1]")
	f.Add(".myapp.com")
	f.Add("192.168.1.1:443")
	f.Add("")
	f.Fuzz(func(_ *testing.T, input string) {
		_ = normalizeHost(input)
	})
}
