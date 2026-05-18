package hostauthorization

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

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

func Test_ConfigPanicHostExceedsRFC1035TotalLength(t *testing.T) {
	t.Parallel()

	tooLong := strings.Repeat("a", 254)
	require.Panics(t, func() {
		New(Config{
			AllowedHosts: []string{tooLong},
		})
	})
}

func Test_ConfigPanicLeadingDotForm(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		New(Config{
			AllowedHosts: []string{".myapp.com"},
		})
	})
}

func Test_ConfigPanicLabelExceedsRFC1035Length(t *testing.T) {
	t.Parallel()

	tooLong := strings.Repeat("a", 64) + ".example.com"
	require.Panics(t, func() {
		New(Config{
			AllowedHosts: []string{tooLong},
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

func Test_NormalizeHost(t *testing.T) {
	t.Parallel()

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
		{"idn unicode → punycode", "münchen.example.com", "xn--mnchen-3ya.example.com"},
		{"idn already punycode", "xn--mnchen-3ya.example.com", "xn--mnchen-3ya.example.com"},
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

	parsed := parseAllowedHosts([]string{"", "   ", "example.com"})

	require.True(t, matchHost("example.com", parsed, nil))
	require.False(t, matchHost("", parsed, nil))
}

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

	parsed := parseAllowedHosts([]string{"*.myapp.com"})

	require.True(t, matchHost("api.myapp.com", parsed, nil))
	require.True(t, matchHost("www.myapp.com", parsed, nil))
	require.True(t, matchHost("deep.sub.myapp.com", parsed, nil))
	require.False(t, matchHost("myapp.com", parsed, nil), "bare domain must NOT match subdomain wildcard")
	require.False(t, matchHost("evil.com", parsed, nil))
}

func Test_MatchSubdomainWildcard_IDN(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"*.münchen.example.com"})

	require.True(t, matchHost(normalizeHost("api.münchen.example.com"), parsed, nil))
	require.True(t, matchHost("api.xn--mnchen-3ya.example.com", parsed, nil))
	require.False(t, matchHost("xn--mnchen-3ya.example.com", parsed, nil), "bare domain must NOT match subdomain wildcard")
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
		"*.myapp.com",
		"127.0.0.1",
	})

	require.True(t, matchHost("example.com", parsed, nil))
	require.True(t, matchHost("api.myapp.com", parsed, nil))
	require.True(t, matchHost("127.0.0.1", parsed, nil))
	require.False(t, matchHost("evil.com", parsed, nil))
	require.False(t, matchHost("192.168.1.1", parsed, nil))
}

func Test_MatchEmptyHost(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"example.com"})

	require.False(t, matchHost("", parsed, nil))
}

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

	// app.Test() substitutes "localhost" when req.Host is empty, which isn't in the allowlist.
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

func Test_HostAuthorization_AllowedHostWithPort(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"example.com:8080"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "example.com:8080"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

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
		AllowedHosts: []string{"*.myapp.com"},
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
		AllowedHosts: []string{"*.myapp.com"},
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

func Test_HostAuthorization_IDN_PunycodeRequest(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		AllowedHosts: []string{"münchen.example.com"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "xn--mnchen-3ya.example.com"

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
			"*.myapp.com",
		},
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

func Test_HostAuthorization_XForwardedHost_TrustProxy_Allowed(t *testing.T) {
	t.Parallel()

	// app.Test() uses remote address 0.0.0.0; trust that proxy IP.
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

	// Locked in so callers matching on err.Error() get a failing test on change.
	require.Equal(t, "hostauthorization: forbidden host", ErrForbiddenHost.Error())
}

func Test_AllowedHostsFuncFallback(t *testing.T) {
	t.Parallel()

	called := 0
	parsed := parseAllowedHosts([]string{"example.com"})
	fn := func(_ string) bool {
		called++
		return false
	}

	result := matchHost("example.com", parsed, fn)
	require.True(t, result)
	require.Equal(t, 0, called, "AllowedHostsFunc must not be called when a static host matches")

	result = matchHost("other.com", parsed, fn)
	require.False(t, result)
	require.Equal(t, 1, called, "AllowedHostsFunc must be called when no static rule matches")
}

func Test_NormalizeHost_IPv6WithPortInConfig(t *testing.T) {
	t.Parallel()

	parsed := parseAllowedHosts([]string{"[::1]:8080"})

	require.True(t, matchHost("::1", parsed, nil))
	require.False(t, matchHost("::2", parsed, nil))
}

// --- Benchmarks ---

func Benchmark_matchHost_ExactMatch(b *testing.B) {
	parsed := parseAllowedHosts([]string{"example.com"})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchHost("example.com", parsed, nil)
	}
}

func Benchmark_matchHost_WildcardMatch(b *testing.B) {
	parsed := parseAllowedHosts([]string{"*.myapp.com"})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchHost("api.myapp.com", parsed, nil)
	}
}

func Benchmark_matchHost_Mixed(b *testing.B) {
	parsed := parseAllowedHosts([]string{
		"example.com",
		"*.myapp.com",
		"127.0.0.1",
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchHost("api.myapp.com", parsed, nil)
	}
}

// Worst-case linear HasSuffix scan: target only matches the last entry.
func Benchmark_matchHost_ManyWildcards(b *testing.B) {
	const n = 100
	hosts := make([]string, n)
	for i := 0; i < n; i++ {
		hosts[i] = fmt.Sprintf("*.tenant%d.example.com", i)
	}
	parsed := parseAllowedHosts(hosts)
	target := fmt.Sprintf("api.tenant%d.example.com", n-1)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchHost(target, parsed, nil)
	}
}

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

func Benchmark_HostAuthorization_Mixed(b *testing.B) {
	app := fiber.New()
	app.Use(New(Config{
		AllowedHosts: []string{
			"example.com",
			"*.myapp.com",
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

func FuzzNormalizeHost(f *testing.F) {
	f.Add("example.com")
	f.Add("example.com.")
	f.Add("[::1]:8080")
	f.Add("[::1]")
	f.Add("*.myapp.com")
	f.Add("192.168.1.1:443")
	f.Add("münchen.example.com")
	f.Add("")
	f.Fuzz(func(_ *testing.T, input string) {
		_ = normalizeHost(input)
	})
}
