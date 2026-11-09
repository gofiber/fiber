package hostauthorization

import (
	"fmt"
	"net/netip"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/utils/v2"
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
	entry := func(h string) func() { return func() { New(Config{AllowedHosts: []string{h}}) } }
	for _, tc := range []struct {
		fn   func()
		name string
		msg  string
	}{
		{name: "no hosts or func", fn: func() { configDefault(Config{}) }},
		{name: "empty slice", fn: func() { configDefault(Config{AllowedHosts: []string{}}) }},
		{name: "no args", fn: func() { configDefault() }},
		{name: "host exceeds RFC 1035 total length", fn: func() { New(Config{AllowedHosts: []string{longHost}}) }},
		{name: "leading dot form", fn: func() { New(Config{AllowedHosts: []string{".myapp.com"}}) }},
		{name: "label exceeds RFC 1035 length", fn: func() { New(Config{AllowedHosts: []string{longLabel}}) }},
		{name: "wildcard of an ipv6 address", fn: entry("*.[::1]")},
		{name: "wildcard of an ipv4-mapped address", fn: entry("*.[::ffff:1.2.3.4]")},
		{name: "invalid ipv6", fn: entry("[2001:db8::g]")},
		{
			name: "zoned ipv6", fn: entry("[fe80::1%25eth0]"),
			msg: "hostauthorization: invalid host [fe80::1%25eth0]: IPv6 zones and percent-encoding are not supported",
		},
		{name: "bracketed name with port", fn: entry("[allowed.com:443]")},
		{name: "wildcard of an ipv4 address", fn: entry("*.127.0.0.1")},
		{name: "wildcard of an ipv4 address with trailing dot", fn: entry("*.127.0.0.1.")},
		{name: "wildcard of a bracketed ipv4 address", fn: entry("*.[1.2.3.4]")},
		{name: "wildcard of a bracketed name", fn: entry("*.[allowed.com]")},
		{name: "wildcard with a non-numeric port", fn: entry("*.a:b")},
		{name: "bracketed name", fn: entry("[allowed.com]")},
		{name: "bracketed name and port", fn: entry("[allowed.com]:443")},
		{name: "bracketed ipv4", fn: entry("[127.0.0.1]")},
		{name: "double brackets", fn: entry("[[::1]]")},
		{name: "unbalanced bracket", fn: entry("[::1")},
		{name: "ipv6 with port out of range", fn: entry("[::1]:65536")},
		{name: "colons that are not ipv6", fn: entry("a:b:c")},
		{name: "empty port", fn: entry("example.com:")},
		{name: "non-numeric port", fn: entry("allowed.com:http"), msg: `hostauthorization: invalid host allowed.com:http: invalid port "http"`},
		{name: "url", fn: entry("https://myapp.com"), msg: "hostauthorization: invalid host https://myapp.com: write host[:port], not a URL"},
		{name: "url with port", fn: entry("http://localhost:3000")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.msg != "" {
				require.PanicsWithValue(t, tc.msg, tc.fn)
				return
			}
			require.Panics(t, tc.fn)
		})
	}
}

func Test_NormalizeEntry(t *testing.T) {
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
		{"idn with port", "münchen.example.com:8080", "xn--mnchen-3ya.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			host, _ := normalizeEntry(tt.input, tt.input)
			require.Equal(t, tt.expected, host)
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
		{name: "ipv6 uppercase hex", input: "[2001:DB8::1]", expected: "2001:db8::1", expectOK: true},
		{name: "ipv6 nat64 dotted", input: "[64:ff9b::1.2.3.4]:443", expected: "64:ff9b::102:304", expectOK: true},
		{name: "ipv6 ipv4-compatible dotted", input: "[::1.2.3.4]", expected: "::102:304", expectOK: true},
		{name: "invalid ipv6", input: "[2001:db8::g]", expectOK: false},
		{name: "zoned ipv6", input: "[::1%eth0]", expectOK: false},
		{name: "zoned ipv6 percent-encoded", input: "[fe80::1%25eth0]", expectOK: false},
		{name: "zoned ipv4-mapped", input: "[::ffff:127.0.0.1%25eth0]", expectOK: false},
		{name: "trailing dot in brackets", input: "[::1.]", expectOK: false},
		{name: "bracketed name", input: "[allowed.com]", expectOK: false},
		{name: "bracketed name with port inside", input: "[allowed.com:443]", expectOK: false},
		// fasthttp 1.78 refuses this before the middleware; keep the middleware's own check pinned.
		{name: "bracketed name with zone", input: "[allowed.com:443%25z]", expectOK: false},
		{name: "bracketed ipv4", input: "[127.0.0.1]", expectOK: false},
		{name: "bracketed label and port", input: "[1:80]", expectOK: false},
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
		// IPv6 literals are validated by canonicalIPv6 and never reach here.
		{name: "ipv6 literal rejected", host: "::1", valid: false},
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

		{name: "idn wildcard matches normalized unicode subdomain", allowed: idnWildcard, host: normalizeName("api.münchen.example.com"), want: true},
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
		loopback4   = []string{"127.0.0.1"}
		loopback6   = []string{"[::1]"}
		docIPv6     = []string{"[2001:db8::1]"}
		linkLocal   = []string{"[fe80::1]"}
		ipWildcard  = []string{"*.2.3.4"}
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
		{name: "idn entry with port", allowed: []string{"münchen.example.com:8080"}, host: "xn--mnchen-3ya.example.com", status: fiber.StatusOK},

		// Every spelling of one IPv6 address names the same host (RFC 4291 Section 2.2).
		{name: "ipv6 uncompressed", allowed: loopback6, host: "[0:0:0:0:0:0:0:1]:3000", status: fiber.StatusOK},
		{name: "ipv6 zero padded", allowed: loopback6, host: "[0000:0000:0000:0000:0000:0000:0000:0001]", status: fiber.StatusOK},
		{name: "ipv6 entry uncompressed", allowed: []string{"[2001:db8:0:0:0:0:0:1]"}, host: "[2001:db8::1]:8080", status: fiber.StatusOK},
		{name: "ipv6 entry zero padded", allowed: []string{"[2001:0db8::0001]"}, host: "[2001:db8::1]", status: fiber.StatusOK},
		{name: "ipv6 entry ipv4-mapped dotted", allowed: []string{"[::ffff:127.0.0.1]"}, host: "[::ffff:7f00:1]", status: fiber.StatusOK},
		{name: "ipv6 other address", allowed: loopback6, host: "[0:0:0:0:0:0:0:2]", status: fiber.StatusForbidden},
		{name: "ipv6 other address, same prefix", allowed: docIPv6, host: "[2001:db8::1:0]", status: fiber.StatusForbidden},
		{name: "ipv4-mapped dotted", allowed: loopback4, host: "[::ffff:127.0.0.1]", status: fiber.StatusOK},
		{name: "ipv4-mapped hex", allowed: loopback4, host: "[::ffff:7f00:1]:8080", status: fiber.StatusOK},
		{name: "ipv4-mapped other address", allowed: loopback4, host: "[::ffff:127.0.0.2]", status: fiber.StatusForbidden},
		// An embedded IPv4 spelling must not leave a dotted tail for a subdomain rule.
		{name: "ipv4 wildcard skips nat64 literal", allowed: ipWildcard, host: "[64:ff9b::1.2.3.4]", status: fiber.StatusForbidden},
		{name: "ipv4 wildcard skips ipv4-compatible literal", allowed: ipWildcard, host: "[::1.2.3.4]:443", status: fiber.StatusForbidden},
		{name: "ipv4 wildcard matches ipv4-mapped literal", allowed: ipWildcard, host: "[::ffff:1.2.3.4]", status: fiber.StatusOK},

		// Canonicalization must not drop a zone, and Unmap does for a mapped one.
		{name: "ipv6 zoned", allowed: linkLocal, host: "[fe80::1%25eth0]", status: fiber.StatusForbidden},
		{name: "ipv6 zoned uncompressed", allowed: linkLocal, host: "[FE80:0:0:0:0:0:0:1%25eth0]:80", status: fiber.StatusForbidden},
		{name: "ipv4-mapped zoned", allowed: loopback4, host: "[::ffff:127.0.0.1%25eth0]", status: fiber.StatusForbidden},
		{name: "ipv4-mapped hex zoned", allowed: loopback4, host: "[::ffff:7f00:1%25eth0]:8080", status: fiber.StatusForbidden},

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
		{"trust proxy, bracketed name with port rejected", "allowed.com", "proxy.internal", "[allowed.com:443]", fiber.StatusForbidden, true},
		{"trust proxy, bracketed name rejected", "allowed.com", "proxy.internal", "[allowed.com]", fiber.StatusForbidden, true},
		{"trust proxy, bracketed ipv4 rejected", "127.0.0.1", "proxy.internal", "[127.0.0.1]", fiber.StatusForbidden, true},
		{"trust proxy, ipv4-compatible literal skips ipv4 wildcard", "*.2.3.4", "proxy.internal", "[::1.2.3.4]:443", fiber.StatusForbidden, true},
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
				got = strings.Clone(host) // host is only valid during the call
				return true
			}})
			apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost(tc.host)), fiber.StatusOK)
			require.Equal(t, tc.want, got)
		})
	}
}

// AllocsPerRun panics under t.Parallel, so this test stays sequential.
func Test_ParseNormalizedAuthority_Allocations(t *testing.T) {
	const fullLength = "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"
	var longUnicode []byte
	for r := rune(0x4E00); len(longUnicode) < 8000; r++ {
		longUnicode = utf8.AppendRune(longUnicode, r)
	}
	// netip interns a zone, so each run needs a fresh one to show a copy.
	zoned := make([]string, 101)
	for i := range zoned {
		zoned[i] = fmt.Sprintf("[fe80::1%%%d%s]:80", i, strings.Repeat("a", 200))
	}
	for _, tc := range []struct {
		name        string
		want        string
		authorities []string
		maxAllocs   float64
		ok          bool
	}{
		{name: "canonical", authorities: []string{"[2001:db8::1]:8080"}, want: "2001:db8::1", ok: true},
		{name: "canonical at full length", authorities: []string{"[" + fullLength + "]:80"}, want: fullLength, ok: true},
		{name: "non-canonical", authorities: []string{"[0:0:0:0:0:0:0:1]:8080"}, want: "::1", ok: true, maxAllocs: 1},
		{name: "ipv4-mapped", authorities: []string{"[::ffff:127.0.0.1]:8080"}, want: "127.0.0.1", ok: true, maxAllocs: 1},
		{name: "zoned and refused", authorities: zoned},
		{name: "long unicode host not converted", authorities: []string{string(longUnicode)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				host string
				ok   bool
				n    int
			)
			allocs := testing.AllocsPerRun(100, func() {
				host, ok = parseNormalizedAuthority(tc.authorities[n%len(tc.authorities)])
				n++
			})
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, host)
			require.LessOrEqual(t, allocs, tc.maxAllocs)
		})
	}
}

// AllocsPerRun panics under t.Parallel, so this test stays sequential.
func Test_IsAllowed_Allocations(t *testing.T) {
	parsed := parseAllowedHosts([]string{"[2001:db8::1]", "127.0.0.1"})
	for _, tc := range []struct {
		host string
		want bool
	}{
		{host: "[2001:db8::1]:8080", want: true},
		{host: "[2001:0db8:0:0:0:0:0:1]", want: true},
		{host: "[::ffff:7f00:1]", want: true},
		{host: "[::ffff:127.0.0.1]:80", want: true},
		{host: "[::ffff:7f00:2]", want: false},
		{host: "[0:0:0:0:0:0:0:2]", want: false},
	} {
		t.Run(tc.host, func(t *testing.T) {
			var got bool
			allocs := testing.AllocsPerRun(100, func() { got = isAllowed(tc.host, parsed, nil) })
			require.Equal(t, tc.want, got)
			require.Zero(t, allocs)
		})
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

func Benchmark_parseNormalizedAuthority_IPv6(b *testing.B) {
	for _, bc := range []struct{ name, authority string }{
		{name: "canonical", authority: "[2001:db8::1]:8080"},
		{name: "non-canonical", authority: "[2001:0db8:0:0:0:0:0:1]:8080"},
		{name: "ipv4-mapped", authority: "[::ffff:127.0.0.1]:8080"},
	} {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				parseNormalizedAuthority(bc.authority)
			}
		})
	}
}

// An entry is accepted or refused with a hostauthorization panic, never a runtime one.
func Benchmark_isAllowed_IPv6(b *testing.B) {
	parsed := parseAllowedHosts([]string{"[::1]", "[2001:db8::1]", "127.0.0.1"})
	for _, bc := range []struct{ name, authority string }{
		{name: "canonical", authority: "[2001:db8::1]:8080"},
		{name: "non-canonical", authority: "[2001:0db8:0:0:0:0:0:1]:8080"},
		{name: "ipv4-mapped", authority: "[::ffff:7f00:1]:8080"},
		{name: "refused", authority: "[2001:db8::2]:8080"},
	} {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				isAllowed(bc.authority, parsed, nil)
			}
		})
	}
}

func FuzzParseAllowedHosts(f *testing.F) {
	for _, seed := range []string{
		"example.com", "example.com.", "[::1]:8080", "::1", "*.myapp.com", "192.168.1.1:443", "münchen.example.com", "",
		"*.127.0.0.1", "[allowed.com]:443", "https://myapp.com", "[fe80::1%25eth0]", "a:b:c",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, entry string) {
		t.Parallel()
		defer func() {
			if r := recover(); r != nil {
				msg, ok := r.(string)
				require.True(t, ok && strings.HasPrefix(msg, "hostauthorization: "), "unexpected panic: %v", r)
			}
		}()
		parseAllowedHosts([]string{entry})
	})
}

// A bracketed authority passes only as the canonical form of a zone-free IPv6 address.
func FuzzParseNormalizedAuthority(f *testing.F) {
	for _, seed := range []string{
		"example.com:8080", "[::1]:8080", "[0:0:0:0:0:0:0:1]", "[2001:DB8:0:0:0:0:0:1]:443", "[::ffff:127.0.0.1]",
		"[::ffff:7f00:1]:80", "[fe80::1%25eth0]", "[::ffff:127.0.0.1%25eth0]", "[allowed.com:443]", "[127.0.0.1]",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, authority string) {
		t.Parallel()
		host, ok := parseNormalizedAuthority(authority)
		authority = utils.TrimSpace(authority)
		end := strings.IndexByte(authority, ']')
		if !ok || authority[0] != '[' || end < 0 {
			return
		}
		addr, err := netip.ParseAddr(authority[1:end])
		require.NoError(t, err)
		require.True(t, addr.Is6(), "brackets must hold an IPv6 address")
		require.Empty(t, addr.Zone())
		require.Equal(t, addr.Unmap().String(), host)
	})
}
