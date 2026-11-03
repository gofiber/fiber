package hostauthorization

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

func Test_ConfigDefault(t *testing.T) {
	t.Parallel()
	t.Run("allowed hosts", func(t *testing.T) {
		t.Parallel()
		cfg := configDefault(Config{AllowedHosts: []string{"example.com"}})
		require.NotNil(t, cfg.ErrorHandler)
		require.Equal(t, []string{"example.com"}, cfg.AllowedHosts)
	})
	t.Run("allowed hosts func only", func(t *testing.T) {
		t.Parallel()
		cfg := configDefault(Config{AllowedHostsFunc: func(host string) bool { return host == "example.com" }})
		require.NotNil(t, cfg.AllowedHostsFunc)
	})
	t.Run("custom error handler", func(t *testing.T) {
		t.Parallel()
		custom := func(c fiber.Ctx, _ error) error { return c.Status(fiber.StatusTeapot).SendString("nope") }
		cfg := configDefault(Config{AllowedHosts: []string{"example.com"}, ErrorHandler: custom})
		require.NotNil(t, cfg.ErrorHandler)
	})
}

// Test_ConfigPanics covers the configurations that must be rejected at startup.
func Test_ConfigPanics(t *testing.T) {
	t.Parallel()
	longHost, longLabel := strings.Repeat("a", 254), strings.Repeat("a", 64)+".example.com"
	for _, tc := range []struct {
		fn   func()
		name string
	}{
		{name: "no hosts or func", fn: func() { configDefault(Config{}) }},
		{name: "empty slice", fn: func() { configDefault(Config{AllowedHosts: []string{}}) }},
		{name: "no args", fn: func() { configDefault() }},
		{name: "host exceeds RFC 1035 total length", fn: func() { New(Config{AllowedHosts: []string{longHost}}) }},
		{name: "leading dot form", fn: func() { New(Config{AllowedHosts: []string{".myapp.com"}}) }},
		{name: "label exceeds RFC 1035 length", fn: func() { New(Config{AllowedHosts: []string{longLabel}}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Panics(t, tc.fn)
		})
	}
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
		{"multiple trailing dots only trims one", "example.com..", "example.com."},
		{"host with port", "example.com:8080", "example.com"},
		{"uppercase host with port", "EXAMPLE.COM:8080", "example.com"},
		{"ipv4", "192.168.1.1", "192.168.1.1"},
		{"ipv4 with port", "192.168.1.1:8080", "192.168.1.1"},
		{"ipv6 brackets", "[::1]", "::1"},
		{"ipv6 bare", "::1", "::1"},
		{"ipv6 with port", "[::1]:8080", "::1"},
		{"ipv6 uncompressed", "[0:0:0:0:0:0:0:1]", "::1"},
		{"ipv6 zero padded", "[2001:0db8:0000:0000:0000:0000:0000:0001]", "2001:db8::1"},
		{"ipv6 zero padded with port", "[2001:0db8:0000:0000:0000:0000:0000:0001]:8080", "2001:db8::1"},
		{"ipv6 uppercase hex", "[2001:DB8::1]", "2001:db8::1"},
		{"ipv6 ipv4-mapped hex form", "[::ffff:7f00:1]", "127.0.0.1"},
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

func Test_ParseNormalizedAuthority(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
		expectOK bool
	}{
		{name: "plain host", input: "example.com", expected: "example.com", expectOK: true},
		{name: "host with valid port", input: "example.com:8080", expected: "example.com", expectOK: true},
		{name: "multiple trailing dots are rejected as malformed", input: "api.example.com...:443", expectOK: false},
		{name: "ipv6 with port", input: "[::1]:443", expected: "::1", expectOK: true},
		{name: "ipv6 without port", input: "[::1]", expected: "::1", expectOK: true},
		{name: "ipv6 uncompressed", input: "[0:0:0:0:0:0:0:1]:443", expected: "::1", expectOK: true},
		{name: "ipv6 zero padded", input: "[2001:0db8:0000:0000:0000:0000:0000:0001]", expected: "2001:db8::1", expectOK: true},
		{name: "ipv6 ipv4-mapped hex form", input: "[::ffff:7f00:1]:8080", expected: "127.0.0.1", expectOK: true},
		{name: "empty port", input: "example.com:", expectOK: false},
		{name: "signed port", input: "example.com:+80", expectOK: false},
		{name: "port out of range", input: "example.com:65536", expectOK: false},
		{name: "zero-padded port", input: "example.com:000080", expected: "example.com", expectOK: true},
		{name: "long zero-padded max port", input: "example.com:000000000000065535", expected: "example.com", expectOK: true},
		{name: "zero-padded port out of range", input: "example.com:00065536", expectOK: false},
		{name: "ipv6 with zero-padded port", input: "[::1]:0080", expected: "::1", expectOK: true},
		{name: "ipv6 with zero-padded port out of range", input: "[::1]:065536", expectOK: false},
		{name: "port with a long non-zero digit run", input: "example.com:999999999999999999999999999999", expectOK: false},
		{name: "userinfo style authority", input: "allowed.com:443@attacker.example", expectOK: false},
		{name: "invalid port syntax", input: "allowed.com:http", expectOK: false},
		{name: "bare ipv6", input: "::1", expectOK: false},
		{name: "malformed bracket", input: "[::1", expectOK: false},
		{name: "extra data after bracket", input: "[::1]extra", expectOK: false},
		{name: "path delimiter in host", input: "attacker.test/.example.com", expectOK: false},
		{name: "query delimiter in host", input: "attacker.test?.example.com", expectOK: false},
		{name: "space in host", input: "bad host.example.com", expectOK: false},
		{name: "253 bytes", input: strings.Repeat("a.", 125) + "com", expected: strings.Repeat("a.", 125) + "com", expectOK: true},
		{name: "253 bytes with trailing dot and port", input: strings.Repeat("a.", 125) + "com.:443", expected: strings.Repeat("a.", 125) + "com", expectOK: true},
		{name: "254 bytes", input: strings.Repeat("a.", 125) + "coms", expectOK: false},
		{name: "254 bytes in brackets", input: "[" + strings.Repeat("a.", 125) + "coms]", expectOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			host, ok := parseNormalizedAuthority(tt.input)
			require.Equal(t, tt.expectOK, ok)
			require.Equal(t, tt.expected, host)
		})
	}
}

// Not parallel: AllocsPerRun counts allocations process-wide and refuses to run
// under a parallel test.
func Test_ParseNormalizedAuthority_LongUnicodeHostNotConverted(t *testing.T) {
	var label []byte
	for r := rune(0x4E00); len(label) < 8000; r++ {
		label = utf8.AppendRune(label, r)
	}
	host := string(label)

	require.Zero(t, testing.AllocsPerRun(10, func() {
		_, ok := parseNormalizedAuthority(host)
		require.False(t, ok)
	}))
}

func Test_IsValidHostSyntax(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		host  string
		valid bool
	}{
		{name: "empty", host: "", valid: false},
		{name: "single label", host: "localhost", valid: true},
		{name: "multi label", host: "sub.api.example.com", valid: true},
		{name: "ipv4 literal", host: "192.168.0.1", valid: true},
		{name: "ipv6 literal", host: "::1", valid: true},
		{name: "ipv6 full", host: "2001:db8::1", valid: true},
		{name: "malformed ipv6", host: "::g", valid: false},
		{name: "ipv4 mapped ipv6", host: "::ffff:192.0.2.1", valid: true},
		{name: "zoned ipv6 rejected", host: "fe80::1%eth0", valid: false},
		{name: "digit label", host: "1.example.com", valid: true},
		{name: "leading hyphen label", host: "-bad.example.com", valid: false},
		{name: "trailing hyphen label", host: "bad-.example.com", valid: false},
		{name: "final label trailing hyphen", host: "example.com-", valid: false},
		{name: "empty label", host: "a..b", valid: false},
		{name: "leading dot", host: ".example.com", valid: false},
		{name: "trailing dot", host: "example.com.", valid: false},
		{name: "underscore rejected", host: "a_b.example.com", valid: false},
		{name: "max length label", host: strings.Repeat("a", 63) + ".com", valid: true},
		{name: "oversize label", host: strings.Repeat("a", 64) + ".com", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.valid, isValidHostSyntax(tt.host))
		})
	}
}

// Test_MatchHost checks the order in which matchHost applies its rules (exact
// entries, subdomain wildcards, then the AllowedHostsFunc fallback) to hosts that
// are already normalized.
func Test_MatchHost(t *testing.T) {
	t.Parallel()
	var (
		blank       = []string{"", "   ", "example.com"}
		example     = []string{"example.com"}
		exact       = []string{"example.com", "api.myapp.com"}
		wildcard    = []string{"*.myapp.com"}
		idnWildcard = []string{"*.münchen.example.com"}
		mixed       = []string{"example.com", "*.myapp.com", "127.0.0.1"}
		ipv6Port    = []string{"[::1]:8080"}
		dynamic     = func(host string) bool { return host == "dynamic.com" }
	)

	for _, tc := range []struct {
		fn      func(string) bool
		name    string
		host    string
		allowed []string
		want    bool
	}{
		{name: "blank entries are skipped, listed host matches", allowed: blank, host: "example.com", want: true},
		{name: "blank entries are skipped, empty host does not match", allowed: blank, host: "", want: false},

		{name: "exact first entry", allowed: exact, host: "example.com", want: true},
		{name: "exact second entry", allowed: exact, host: "api.myapp.com", want: true},
		{name: "exact unlisted host", allowed: exact, host: "evil.com", want: false},
		{name: "exact entry is case insensitive", allowed: []string{"Example.COM"}, host: "example.com", want: true},

		{name: "wildcard matches subdomain", allowed: wildcard, host: "api.myapp.com", want: true},
		{name: "wildcard matches another subdomain", allowed: wildcard, host: "www.myapp.com", want: true},
		{name: "wildcard matches nested subdomain", allowed: wildcard, host: "deep.sub.myapp.com", want: true},
		// The bare domain must NOT match a subdomain wildcard.
		{name: "wildcard does not match bare domain", allowed: wildcard, host: "myapp.com", want: false},
		{name: "wildcard does not match unrelated host", allowed: wildcard, host: "evil.com", want: false},

		{name: "idn wildcard matches normalized unicode subdomain", allowed: idnWildcard, host: normalizeHost("api.münchen.example.com"), want: true},
		{name: "idn wildcard matches punycode subdomain", allowed: idnWildcard, host: "api.xn--mnchen-3ya.example.com", want: true},
		// The bare domain must NOT match a subdomain wildcard.
		{name: "idn wildcard does not match bare domain", allowed: idnWildcard, host: "xn--mnchen-3ya.example.com", want: false},

		{name: "func does not hide static host", allowed: example, fn: dynamic, host: "example.com", want: true},
		{name: "func accepts host it allows", allowed: example, fn: dynamic, host: "dynamic.com", want: true},
		{name: "func rejects other host", allowed: example, fn: dynamic, host: "evil.com", want: false},

		{name: "mixed rules match exact host", allowed: mixed, host: "example.com", want: true},
		{name: "mixed rules match wildcard subdomain", allowed: mixed, host: "api.myapp.com", want: true},
		{name: "mixed rules match ip", allowed: mixed, host: "127.0.0.1", want: true},
		{name: "mixed rules reject unlisted host", allowed: mixed, host: "evil.com", want: false},
		{name: "mixed rules reject unlisted ip", allowed: mixed, host: "192.168.1.1", want: false},

		{name: "empty host never matches", allowed: example, host: "", want: false},

		{name: "ipv6 entry with port matches bare address", allowed: ipv6Port, host: "::1", want: true},
		{name: "ipv6 entry with port rejects other address", allowed: ipv6Port, host: "::2", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, matchHost(tc.host, parseAllowedHosts(tc.allowed), tc.fn), "host %q", tc.host)
		})
	}
}

// hostAuthApp returns an app that answers GET / with "OK" behind the middleware
// configured by cfg. appCfg optionally configures the app itself.
func hostAuthApp(cfg Config, appCfg ...fiber.Config) *fiber.App {
	app := fiber.New(appCfg...)
	app.Use(New(cfg))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("OK") })
	return app
}

// Test_HostAuthorization_Hosts sends GET / with each row's Host header through
// the middleware configured by the row's allowed hosts and func, and expects the
// row's status.
func Test_HostAuthorization_Hosts(t *testing.T) {
	t.Parallel()
	var (
		example     = []string{"example.com"}
		examplePort = []string{"example.com:8080"}
		wildcard    = []string{"*.myapp.com"}
		dynamic     = func(host string) bool { return host == "dynamic.com" }
	)

	for _, tc := range []struct {
		fn      func(string) bool
		name    string
		host    string
		allowed []string
		status  int
	}{
		{name: "allowed host", allowed: example, host: "example.com", status: fiber.StatusOK},
		{name: "rejected host", allowed: example, host: "evil.com", status: fiber.StatusForbidden},
		// app.Test() substitutes "localhost" when req.Host is empty, which isn't in the allowlist.
		{name: "empty host", allowed: example, host: "", status: fiber.StatusForbidden},
		{name: "host with port", allowed: example, host: "example.com:8080", status: fiber.StatusOK},
		{name: "allowed host with port, request with port", allowed: examplePort, host: "example.com:8080", status: fiber.StatusOK},
		{name: "allowed host with port, request without port", allowed: examplePort, host: "example.com", status: fiber.StatusOK},
		{name: "case insensitive", allowed: []string{"Example.COM"}, host: "EXAMPLE.com", status: fiber.StatusOK},
		{name: "trailing dot", allowed: example, host: "example.com.", status: fiber.StatusOK},
		{name: "exact ip", allowed: []string{"127.0.0.1"}, host: "127.0.0.1", status: fiber.StatusOK},
		{name: "idn punycode request", allowed: []string{"münchen.example.com"}, host: "xn--mnchen-3ya.example.com", status: fiber.StatusOK},

		{name: "subdomain wildcard allowed", allowed: wildcard, host: "api.myapp.com", status: fiber.StatusOK},
		{name: "subdomain wildcard bare domain rejected", allowed: wildcard, host: "myapp.com", status: fiber.StatusForbidden},
		{name: "overlapping rules", allowed: []string{"api.myapp.com", "*.myapp.com"}, host: "api.myapp.com", status: fiber.StatusOK},

		{name: "allowed hosts func allowed", fn: dynamic, host: "dynamic.com", status: fiber.StatusOK},
		{name: "allowed hosts func rejected", fn: dynamic, host: "evil.com", status: fiber.StatusForbidden},

		// A port is *DIGIT (RFC 3986 section 3.2.3): leading zeros do not change its value.
		{name: "port 80", allowed: example, host: "example.com:80", status: fiber.StatusOK},
		{name: "port 080", allowed: example, host: "example.com:080", status: fiber.StatusOK},
		{name: "port 00080", allowed: example, host: "example.com:00080", status: fiber.StatusOK},
		{name: "port 000080", allowed: example, host: "example.com:000080", status: fiber.StatusOK},
		{name: "port 65536 is out of range", allowed: example, host: "example.com:65536", status: fiber.StatusForbidden},
		{name: "port 00065536 is out of range", allowed: example, host: "example.com:00065536", status: fiber.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := hostAuthApp(Config{AllowedHosts: tc.allowed, AllowedHostsFunc: tc.fn})
			apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost(tc.host)), tc.status)
		})
	}
}

func Test_HostAuthorization_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		AllowedHosts: []string{"example.com"},
		Next:         func(c fiber.Ctx) bool { return c.Path() == "/healthz" },
	}))
	app.Get("/healthz", func(c fiber.Ctx) error { return c.SendString("OK") })

	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/healthz", apptest.WithHost("evil.com")), fiber.StatusOK)
}

func Test_HostAuthorization_AllowedHostsFunc_EmptyHostRejectedBeforeCallback(t *testing.T) {
	t.Parallel()
	called := false
	app := hostAuthApp(Config{
		AllowedHostsFunc: func(_ string) bool {
			called = true
			return true
		},
	})

	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost(":443")), fiber.StatusForbidden)
	require.False(t, called)
}

func Test_HostAuthorization_CustomErrorHandler(t *testing.T) {
	t.Parallel()
	var handlerErr error
	app := hostAuthApp(Config{
		AllowedHosts: []string{"example.com"},
		ErrorHandler: func(c fiber.Ctx, err error) error {
			handlerErr = err
			return c.Status(fiber.StatusTeapot).SendString("custom rejection")
		},
	})

	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost("evil.com")), fiber.StatusTeapot)
	require.ErrorIs(t, handlerErr, ErrForbiddenHost)
}

// Test_HostAuthorization_XForwardedHost checks that X-Forwarded-Host decides
// the request only when the proxy that sent it is trusted.
func Test_HostAuthorization_XForwardedHost(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		allowed    string
		host       string
		forwarded  string
		status     int
		trustProxy bool
	}{
		{"trust proxy, forwarded host allowed", "example.com", "proxy.internal", "example.com", fiber.StatusOK, true},
		{"trust proxy, forwarded host rejected", "example.com", "example.com", "evil.com", fiber.StatusForbidden, true},
		{
			name: "trust proxy, malformed forwarded authority rejected", trustProxy: true,
			allowed: "allowed.com", host: "proxy.internal", forwarded: "allowed.com:443@attacker.example", status: fiber.StatusForbidden,
		},
		// X-Forwarded-Host should be ignored without TrustProxy.
		{"no trust proxy, forwarded host ignored", "example.com", "example.com", "evil.com", fiber.StatusOK, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var appCfg fiber.Config
			if tc.trustProxy {
				// app.Test() uses remote address 0.0.0.0; trust that proxy IP.
				appCfg = fiber.Config{
					TrustProxy:       true,
					TrustProxyConfig: fiber.TrustProxyConfig{Proxies: []string{"0.0.0.0"}},
				}
			}

			app := hostAuthApp(Config{AllowedHosts: []string{tc.allowed}}, appCfg)
			req := apptest.Req(fiber.MethodGet, "/", apptest.WithHost(tc.host), apptest.WithHeader("X-Forwarded-Host", tc.forwarded))
			apptest.Expect(t, app, req, tc.status)
		})
	}
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

	require.True(t, matchHost("example.com", parsed, fn))
	require.Equal(t, 0, called, "AllowedHostsFunc must not be called when a static host matches")

	require.False(t, matchHost("other.com", parsed, fn))
	require.Equal(t, 1, called, "AllowedHostsFunc must be called when no static rule matches")
}

// Every spelling of one IPv6 address names the same host (RFC 4291 Section 2.2),
// whichever one the configuration or the client wrote.
func Test_HostAuthorization_IPv6CanonicalForm(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		allowed string
		host    string
		want    int
	}{
		{name: "allowed compressed, host uncompressed", allowed: "[::1]", host: "[0:0:0:0:0:0:0:1]:3000", want: fiber.StatusOK},
		{name: "allowed compressed, host zero padded", allowed: "[::1]", host: "[0000:0000:0000:0000:0000:0000:0000:0001]", want: fiber.StatusOK},
		{name: "allowed uncompressed, host as a browser sends it", allowed: "[2001:db8:0:0:0:0:0:1]", host: "[2001:db8::1]:8080", want: fiber.StatusOK},
		{name: "allowed zero padded, host compressed", allowed: "[2001:0db8::0001]", host: "[2001:db8::1]", want: fiber.StatusOK},
		{name: "allowed ipv4-mapped dotted, host hex", allowed: "[::ffff:127.0.0.1]", host: "[::ffff:7f00:1]", want: fiber.StatusOK},
		{name: "different address still refused", allowed: "[::1]", host: "[0:0:0:0:0:0:0:2]", want: fiber.StatusForbidden},
		{name: "different address with matching prefix still refused", allowed: "[2001:db8::1]", host: "[2001:db8::1:0]", want: fiber.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := hostAuthApp(Config{AllowedHosts: []string{tc.allowed}})
			apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost(tc.host)), tc.want)
		})
	}
}

// Canonicalization must not drop the zone: a zoned host is refused even when the
// same address without a zone is allowed.
func Test_HostAuthorization_IPv6ZonedStillRefused(t *testing.T) {
	t.Parallel()
	app := hostAuthApp(Config{AllowedHosts: []string{"[fe80::1]"}})
	for _, host := range []string{"[fe80::1%25eth0]", "[FE80:0:0:0:0:0:0:1%25eth0]:80"} {
		apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost(host)), fiber.StatusForbidden)
	}
}

func Test_HostAuthorization_AllowedHostsFuncReceivesCanonicalIPv6(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		host string
		want string
	}{
		{host: "[0:0:0:0:0:0:0:1]:3000", want: "::1"},
		{host: "[2001:0DB8::0001]", want: "2001:db8::1"},
		{host: "[::ffff:7f00:1]", want: "127.0.0.1"},
		{host: "[2001:db8::1]:8080", want: "2001:db8::1"},
	} {
		t.Run(tc.host, func(t *testing.T) {
			t.Parallel()
			var got string
			app := hostAuthApp(Config{AllowedHostsFunc: func(host string) bool {
				got = host
				return true
			}})
			apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost(tc.host)), fiber.StatusOK)
			require.Equal(t, tc.want, got)
		})
	}
}

func Test_MatchHost_WildcardSkipsIPv6Literals(t *testing.T) {
	t.Parallel()
	// An embedded IPv4 spelling must not leave a dotted tail for a subdomain rule.
	parsed := parseAllowedHosts([]string{"*.2.3.4", "*.example.com"})
	require.False(t, matchHost(normalizeHost("[64:ff9b::1.2.3.4]"), parsed, nil))
	require.False(t, matchHost(normalizeHost("[::1.2.3.4]"), parsed, nil))
	require.True(t, matchHost("api.example.com", parsed, nil))
}

func Test_HostAuthorization_IPv4MappedIsTheIPv4Host(t *testing.T) {
	t.Parallel()
	app := hostAuthApp(Config{AllowedHosts: []string{"127.0.0.1"}})
	for _, host := range []string{"[::ffff:127.0.0.1]", "[::ffff:7f00:1]:8080", "127.0.0.1"} {
		apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost(host)), fiber.StatusOK)
	}
	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost("[::ffff:127.0.0.2]")), fiber.StatusForbidden)
}

func Test_ParseNormalizedAuthority_IPv6KindAndAllocs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		authority string
		want      string
		ok        bool
	}{
		{authority: "[2001:db8::1]:8080", want: "2001:db8::1", ok: true},
		{authority: "[0:0:0:0:0:0:0:1]", want: "::1", ok: true},
		{authority: "[fe80::1%25eth0]", ok: false},
		{authority: "[::1%eth0]", ok: false},
		{authority: "[2001:db8::g]", ok: false},
		{authority: "[allowed.com:443]", ok: false},
		{authority: "[allowed.com]", ok: false},
		{authority: "[1:80]", ok: false},
	} {
		got, ok := parseNormalizedAuthority(tc.authority)
		require.Equal(t, tc.ok, ok, tc.authority)
		require.Equal(t, tc.want, got, tc.authority)
	}
}

// AllocsPerRun panics under t.Parallel, so this test stays sequential.
func Test_ParseNormalizedAuthority_IPv6Allocations(t *testing.T) {
	zone := "[fe80::1%" + strings.Repeat("a", 4000) + "]:80"
	for _, tc := range []struct {
		name      string
		authority string
		maxAllocs float64
	}{
		{name: "canonical", authority: "[2001:db8::1]:8080", maxAllocs: 0},
		{name: "zoned and refused", authority: zone, maxAllocs: 0},
		{name: "non-canonical pays for its canonical string once", authority: "[0:0:0:0:0:0:0:1]:8080", maxAllocs: 1},
	} {
		got := testing.AllocsPerRun(100, func() { _, _ = parseNormalizedAuthority(tc.authority) })
		require.LessOrEqual(t, got, tc.maxAllocs, tc.name)
	}
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
	parsed := parseAllowedHosts([]string{"example.com", "*.myapp.com", "127.0.0.1"})
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
	for i := range n {
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
	app := hostAuthApp(Config{AllowedHosts: []string{"example.com"}})
	req := apptest.Req(fiber.MethodGet, "/", apptest.WithHost("example.com"))

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
	app := hostAuthApp(Config{AllowedHosts: []string{"example.com", "*.myapp.com", "127.0.0.1"}})
	req := apptest.Req(fiber.MethodGet, "/", apptest.WithHost("api.myapp.com"))

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

func Benchmark_HostAuthorization_IPv6ExactMatch(b *testing.B) {
	app := hostAuthApp(Config{AllowedHosts: []string{"[2001:db8::1]"}})
	req := apptest.Req(fiber.MethodGet, "/", apptest.WithHost("[2001:db8::1]:8080"))

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

func Benchmark_normalizeHost_IPv6(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = normalizeHost("[2001:db8::1]:8080")
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
	f.Fuzz(func(_ *testing.T, input string) { _ = normalizeHost(input) })
}
