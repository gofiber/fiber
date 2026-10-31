package proxy

import (
	"bufio"
	"bytes"
	"cmp"
	"crypto/tls"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/crosshost"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/internal/tlstest"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// withSecurityPolicyForTest installs policy for the duration of tb and
// restores the previous policy via tb.Cleanup. Tests that need to
// exercise the strict defaults call this with an explicit policy.
func withSecurityPolicyForTest(tb testing.TB, policy SecurityPolicy) {
	tb.Helper()
	prev := WithSecurityPolicy(policy)
	tb.Cleanup(func() { WithSecurityPolicy(prev) })
}

func Test_Security_IsBlockedIP(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"127.0.0.1":                true,
		"::1":                      true,
		"0.0.0.0":                  true,
		"10.0.0.1":                 true,
		"172.16.0.1":               true,
		"192.168.1.1":              true,
		"169.254.169.254":          true,  // AWS metadata
		"100.64.0.1":               true,  // CGNAT
		"224.0.0.1":                true,  // multicast
		"0.1.2.3":                  true,  // 0.0.0.0/8
		"0.255.255.255":            true,  // 0.0.0.0/8
		"192.0.0.192":              true,  // Oracle Cloud legacy metadata, 192.0.0.0/24
		"192.0.0.1":                true,  // 192.0.0.0/24
		"198.18.0.1":               true,  // benchmarking, 198.18.0.0/15
		"198.19.255.254":           true,  // benchmarking
		"240.0.0.1":                true,  // reserved, 240.0.0.0/4
		"255.255.255.255":          true,  // limited broadcast
		"::ffff:192.0.0.192":       true,  // IPv4-mapped 192.0.0.192
		"64:ff9b::c000:c0":         true,  // NAT64 WKP 192.0.0.192
		"fec0::1":                  true,  // site-local, fec0::/10
		"fed0::1":                  true,  // site-local
		"192.0.1.1":                false, // just outside 192.0.0.0/24
		"192.0.2.1":                false, // TEST-NET-1 documentation range
		"198.17.255.255":           false, // just below 198.18.0.0/15
		"198.20.0.0":               false, // just above 198.18.0.0/15
		"198.51.100.7":             false, // TEST-NET-2 documentation range
		"203.0.113.9":              false, // TEST-NET-3 documentation range
		"223.255.255.254":          false, // below 240.0.0.0/4
		"239.255.255.255":          true,  // top of multicast
		"9.255.255.255":            false, // just below 10.0.0.0/8
		"11.0.0.0":                 false, // just above 10.0.0.0/8
		"100.63.255.255":           false, // just below CGNAT, 100.64.0.0/10
		"100.127.255.255":          true,  // top of CGNAT
		"100.128.0.0":              false, // just above CGNAT
		"126.255.255.255":          false, // just below 127.0.0.0/8
		"128.0.0.0":                false, // just above 127.0.0.0/8
		"169.253.255.255":          false, // just below link-local, 169.254.0.0/16
		"169.255.0.0":              false, // just above link-local
		"172.15.255.255":           false, // just below 172.16.0.0/12
		"172.31.255.255":           true,  // top of 172.16.0.0/12
		"172.32.0.0":               false, // just above 172.16.0.0/12
		"192.167.255.255":          false, // just below 192.168.0.0/16
		"192.169.0.0":              false, // just above 192.168.0.0/16
		"fb00::1":                  false, // below fc00::/7
		"::ffff:127.0.0.1":         true,  // IPv4-mapped loopback
		"::ffff:10.0.0.1":          true,  // IPv4-mapped private
		"::127.0.0.1":              true,  // IPv4-compatible loopback
		"::10.0.0.1":               true,  // IPv4-compatible private
		"64:ff9b::7f00:1":          true,  // NAT64 WKP 127.0.0.1
		"64:ff9b::a00:1":           true,  // NAT64 WKP 10.0.0.1
		"64:ff9b::a9fe:a9fe":       true,  // NAT64 WKP 169.254.169.254 (metadata)
		"64:ff9b:1::a9fe:a9fe":     true,  // NAT64 local-use prefix (blocked wholesale, RFC 8215)
		"64:ff9b:1::a00:1":         true,  // NAT64 local-use prefix (blocked wholesale)
		"64:ff9b:1::808:808":       true,  // NAT64 local-use prefix (blocked wholesale even for public embed)
		"2002:a9fe:a9fe::":         true,  // 6to4 (blocked wholesale, RFC 3056/7526)
		"2002:a00:1::":             true,  // 6to4 (blocked wholesale)
		"2002:7f00:1::":            true,  // 6to4 (blocked wholesale)
		"2002:808:808::":           true,  // 6to4 (blocked wholesale even for public embed)
		"2001::5601:5601":          true,  // Teredo (blocked wholesale, client field)
		"2001::f5ff:fffe":          true,  // Teredo (blocked wholesale)
		"2001:0:a00:1::":           true,  // Teredo (blocked wholesale, private server field bytes [4:8])
		"2001::808:808":            true,  // Teredo (blocked wholesale even for public embed)
		"2001:db8::5efe:a9fe:a9fe": true,  // ISATAP 169.254.169.254 (metadata)
		"2001:db8::200:5efe:a00:1": true,  // ISATAP 10.0.0.1 (global-scope u/g bit)
		"2001:db8::5efe:808:808":   false, // ISATAP 8.8.8.8 (public → allowed)
		"::ffff:0:7f00:1":          true,  // SIIT IPv4-translated 127.0.0.1
		"::ffff:0:a00:1":           true,  // SIIT IPv4-translated 10.0.0.1
		"::ffff:0:a9fe:a9fe":       true,  // SIIT IPv4-translated 169.254.169.254 (metadata)
		"::ffff:0:808:808":         false, // SIIT IPv4-translated 8.8.8.8 (public → allowed)
		"8.8.8.8":                  false,
		"1.1.1.1":                  false,
		"93.184.216.34":            false, // example.com
		"64:ff9b::808:808":         false, // NAT64 WKP 8.8.8.8 (public → allowed)
		"2606:4700:4700::1111":     false, // public IPv6 (Cloudflare)
	}
	for raw, blocked := range cases {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(raw)
			require.NotNil(t, ip, "ParseIP %q", raw)
			require.Equal(t, blocked, isBlockedIP(ip))
		})
	}
}

func Test_Security_SchemeAllowed(t *testing.T) {
	t.Parallel()
	require.True(t, schemeAllowed("http", nil))
	require.True(t, schemeAllowed("https", nil))
	require.True(t, schemeAllowed("HTTPS", nil))
	require.False(t, schemeAllowed("file", nil))
	require.False(t, schemeAllowed("gopher", nil))
	require.False(t, schemeAllowed("ftp", nil))
	require.False(t, schemeAllowed("", nil))
	require.True(t, schemeAllowed("ftp", []string{"ftp"}))
}

// Test_Security_ValidateUpstream_Rejects pins what validateUpstream refuses: an
// address the strict policy blocks (the private-IP filtering that
// FuzzValidateUpstream leaves to this test), a scheme outside the allowlist, and
// an upstream without a host.
func Test_Security_ValidateUpstream_Rejects(t *testing.T) {
	t.Parallel()
	strict := DefaultSecurityPolicy()
	allowPrivate := proxyTestAllowPrivatePolicy()

	for _, tc := range []struct {
		err      error
		upstream string
		policy   SecurityPolicy
	}{
		{upstream: "http://127.0.0.1", policy: strict, err: ErrUpstreamHostBlocked},
		{upstream: "http://localhost:8080", policy: strict, err: ErrUpstreamHostBlocked},
		{upstream: "http://10.0.0.1", policy: strict, err: ErrUpstreamHostBlocked},
		{upstream: "http://192.168.1.1/path", policy: strict, err: ErrUpstreamHostBlocked},
		{upstream: "http://169.254.169.254/latest/meta-data/", policy: strict, err: ErrUpstreamHostBlocked},
		{upstream: "https://[::1]/", policy: strict, err: ErrUpstreamHostBlocked},
		{upstream: "file:///etc/passwd", policy: allowPrivate, err: ErrUpstreamSchemeNotAllowed},
		{upstream: "gopher://example.com", policy: allowPrivate, err: ErrUpstreamSchemeNotAllowed},
		{upstream: "ftp://example.com", policy: allowPrivate, err: ErrUpstreamSchemeNotAllowed},
		{upstream: "", policy: strict, err: ErrUpstreamHostInvalid},
		// "http:///path" parses with scheme "http" and no host, which the post-scheme-check
		// "host empty" guard rejects.
		{upstream: "http:///path", policy: strict, err: ErrUpstreamHostInvalid},
	} {
		t.Run(cmp.Or(tc.upstream, "empty"), func(t *testing.T) {
			t.Parallel()
			_, err := validateUpstream(tc.upstream, tc.policy, nil)
			require.ErrorIs(t, err, tc.err, "expected %v for %q", tc.err, tc.upstream)
		})
	}
}

// Test_Security_ValidateUpstream_Accepts pins the upstreams validateUpstream hands back: a private
// address once the policy opts in, and a public IP literal under the strict defaults.
func Test_Security_ValidateUpstream_Accepts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, upstream, host string
		policy               SecurityPolicy
	}{
		{
			name: "private IP with the opt-in", upstream: "http://127.0.0.1:8080", host: "127.0.0.1:8080",
			policy: proxyTestAllowPrivatePolicy(),
		},
		{name: "public IP literal", upstream: "http://8.8.8.8", host: "8.8.8.8", policy: DefaultSecurityPolicy()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u, err := validateUpstream(tc.upstream, tc.policy, nil)
			require.NoError(t, err)
			require.Equal(t, tc.host, u.Host)
		})
	}
}

func Test_Security_Forward_BlocksPrivateByDefault(t *testing.T) {
	// Intentionally not parallel: it sets the strict default policy process-wide.
	withSecurityPolicyForTest(t, DefaultSecurityPolicy())

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString("should not be reached") })

	app := fiber.New()
	app.Use(Forward("http://" + addr))

	apptest.Expect(t, app, "/", fiber.StatusInternalServerError)
}

func Test_Security_Balancer_BlocksPrivateByDefault(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })

	// Build a policy that blocks private IPs and feed it via Config so
	// the panic surfaces at Balancer construction time.
	policy := DefaultSecurityPolicy()
	require.PanicsWithError(t, ErrUpstreamHostBlocked.Error()+": 127.0.0.1", func() {
		Balancer(Config{Servers: []string{addr}, SecurityPolicy: &policy})
	})
}

func Test_Security_Balancer_AllowsPrivateOptIn(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })

	policy := proxyTestAllowPrivatePolicy()
	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{addr}, SecurityPolicy: &policy}))

	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost(addr)), fiber.StatusTeapot)
}

// Test_Security_HopByHopRequestStripping verifies hop-by-hop headers
// (RFC 9110 §7.6.1) are dropped from the outbound request.
func Test_Security_HopByHopRequestStripping(t *testing.T) {
	t.Parallel()
	hops := []string{
		fiber.HeaderKeepAlive,
		fiber.HeaderProxyAuthenticate,
		fiber.HeaderProxyAuthorization,
		fiber.HeaderTE,
		fiber.HeaderTrailer,
		fiber.HeaderTransferEncoding,
		fiber.HeaderUpgrade,
	}

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		// Use t.Errorf (not require) inside the handler goroutine so a
		// failure marks the test instead of aborting the goroutine.
		for _, h := range hops {
			if v := c.Get(h); v != "" {
				t.Errorf("expected hop-by-hop %q stripped, got %q", h, v)
			}
		}
		if v := c.Get("X-Custom-Hop"); v != "" {
			t.Errorf("Connection-listed header should be stripped, got %q", v)
		}
		return c.SendString("ok")
	})

	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{addr}}))

	req := apptest.Req(fiber.MethodGet, "/", apptest.WithHost(addr))
	for _, h := range hops {
		req.Header.Set(h, "drop-me")
	}
	req.Header.Set(fiber.HeaderConnection, "X-Custom-Hop")
	req.Header.Set("X-Custom-Hop", "should-be-removed")

	apptest.Expect(t, app, req, fiber.StatusOK)
}

// Test_Security_IsForwardingHeader pins the family a client cannot have
// removed by naming it in Connection, whatever its case, and that the prefix
// rule takes X-Forwarded-* only.
func Test_Security_IsForwardingHeader(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"X-Real-IP":         true,
		"x-real-ip":         true,
		"Forwarded":         true,
		"FORWARDED":         true,
		"X-Forwarded-For":   true,
		"x-forwarded-host":  true,
		"X-FORWARDED-PROTO": true,
		"X-Forwarded-Port":  true,
		"X-Forwarded":       false,
		"X-ForwardedFor":    false,
		"Via":               false,
		"X-Custom-Hop":      false,
		"Keep-Alive":        false,
		"":                  false,
	} {
		require.Equal(t, want, isForwardingHeader(name), name)
	}
}

func Test_Security_HopByHopResponseStripping(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		c.Set(fiber.HeaderProxyAuthenticate, "Basic")
		c.Set(fiber.HeaderKeepAlive, "timeout=5")
		c.Set("X-Custom-Hop", "leak-me")
		c.Set(fiber.HeaderConnection, "X-Custom-Hop")
		return c.SendString("ok")
	})

	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{addr}}))

	resp := apptest.Do(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost(addr)))
	require.Empty(t, resp.Header.Get(fiber.HeaderProxyAuthenticate))
	require.Empty(t, resp.Header.Get(fiber.HeaderKeepAlive))
	require.Empty(t, resp.Header.Get("X-Custom-Hop"))
}

// Test_Security_KeepConnectionPreservesOtherHopByHop verifies that the
// backwards-compat KeepConnectionHeader option does NOT bypass the
// stripping of the other hop-by-hop headers.
func Test_Security_KeepConnectionPreservesOtherHopByHop(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		if v := c.Get(fiber.HeaderProxyAuthorization); v != "" {
			t.Errorf("Proxy-Authorization should still be stripped, got %q", v)
		}
		if v := c.Get(fiber.HeaderConnection); v != "keep-alive" {
			t.Errorf("expected Connection %q, got %q", "keep-alive", v)
		}
		return c.SendString("ok")
	})

	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{addr}, KeepConnectionHeader: true}))

	req := apptest.Req(fiber.MethodGet, "/", apptest.WithHost(addr),
		apptest.WithHeader(fiber.HeaderConnection, "keep-alive"),
		apptest.WithHeader(fiber.HeaderProxyAuthorization, "Basic dXNlcjpwYXNz"))

	apptest.Expect(t, app, req, fiber.StatusOK)
}

func Test_Security_Do_BlocksFileScheme(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error { return Do(c, "file:///etc/passwd") })
	apptest.Expect(t, app, "/test", fiber.StatusInternalServerError)
}

func Test_Security_DoRedirects_BlocksDowngrade(t *testing.T) {
	// No t.Parallel: this test mutates the global SecurityPolicy and would
	// otherwise race with other proxy tests that read it at runtime.
	// Allow private IPs for the loopback test servers while keeping the
	// downgrade protection enabled.
	withSecurityPolicyForTest(t, proxyTestAllowPrivatePolicy())

	// Start an HTTPS server that 302-redirects to a plaintext HTTP URL.
	cert, err := generateLocalhostCert(t)
	require.NoError(t, err)

	httpsLn, err := tls.Listen(fiber.NetworkTCP4, "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	require.NoError(t, err)
	t.Cleanup(func() { httpsLn.Close() }) //nolint:errcheck // best effort

	plaintextLn := proxyTestListen(t)
	t.Cleanup(func() { plaintextLn.Close() }) //nolint:errcheck // best effort

	plaintextAddr := plaintextLn.Addr().String()

	httpsApp := fiber.New()
	httpsApp.Get("/", func(c fiber.Ctx) error {
		c.Location("http://" + plaintextAddr + "/secret")
		return c.SendStatus(fiber.StatusFound)
	})
	startServer(httpsApp, httpsLn)

	plainApp := fiber.New()
	plainApp.Get("/secret", func(c fiber.Ctx) error { return c.SendString("LEAKED") })
	startServer(plainApp, plaintextLn)

	// Use a custom proxy client that trusts the self-signed cert; otherwise
	// the TLS dial fails before we get to evaluate the redirect.
	tlsClient := &fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,
		TLSConfig:                &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12},
	}

	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		return DoRedirects(c, "https://"+httpsLn.Addr().String(), 1, tlsClient)
	})

	resp := apptest.Expect(t, app, "/test", fiber.StatusInternalServerError)
	body := apptest.Body(t, resp)
	require.Contains(t, body, "HTTPS to HTTP redirect blocked")
}

// Test_Security_JoinUpstreamPath_KeepsEscapes ensures the escapes of the base
// and of the request target leave as written, whichever side net/url escapes
// by default: joined from a decoded path, "%3B" left as ";" and "%2F" as "/".
func Test_Security_JoinUpstreamPath_KeepsEscapes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ base, target, want string }{
		{"/my%20app", "/pub/..%3B/admin", "/my%20app/pub/..%3B/admin"},
		{"/my%20app/", "/admin%3Bx/secret", "/my%20app/admin%3Bx/secret"},
		{"/my%20app", "/pub/..%2Fadmin", "/my%20app/pub/..%2Fadmin"},
		{"/caf%C3%A9", "/a%40b%3Ac%24%26%2B%2C%3D", "/caf%C3%A9/a%40b%3Ac%24%26%2B%2C%3D"},
		{"/café", "/pub/..%3B/admin", "/caf%C3%A9/pub/..%3B/admin"},
		{"/a%2Fb", "/c%3Fd%23e%25f%20g", "/a%2Fb/c%3Fd%23e%25f%20g"},
		{"/a%2Fb", "/caf%C3%A9", "/a%2Fb/caf%C3%A9"},
		{"/api", "/pub/..%3B/admin", "/api/pub/..%3B/admin"},
		{"/a%2F", "/pub/..%3B/admin", "/a%2F/pub/..%3B/admin"},
		{"/a%2F/", "/pub/..%3B/admin", "/a%2F/pub/..%3B/admin"},
		// The parse-error fallback keeps the base's spelling as well.
		{"/a%2Fb", "/%zz", "/a%2Fb/%25zz"},
		{"/my%20app", "/%zz", "/my%20app/%25zz"},
	} {
		base := proxyTestParseUpstream(t, "http://upstream.example"+tc.base)
		require.Equal(t, "http://upstream.example"+tc.want, joinUpstreamPath(base, tc.target), "%s + %s", tc.base, tc.target)
	}
}

// proxyTestParseUpstream parses raw the way a configured upstream is, failing the
// test if it does not parse.
func proxyTestParseUpstream(t *testing.T, raw string) *url.URL {
	t.Helper()

	base, err := parseUpstream(raw)
	require.NoError(t, err)
	return base
}

// Test_Security_JoinUpstreamPath_KeepsTheUpstream pins that no request path
// changes the scheme or the host the upstream pins, however it is spelled.
func Test_Security_JoinUpstreamPath_KeepsTheUpstream(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		base       string
		target     string
		wantPath   string // the path of the joined URL, unless empty
		wantPrefix string // what the path of the joined URL starts with, unless empty
	}{
		// The router keeps empty segments, so "//attacker.example/path" is not
		// "/attacker.example/path" to it. Forwarding it collapsed would hand the
		// upstream a path no middleware mounted on "/attacker.example" has seen.
		{name: "network path", base: "http://upstream.example", target: "//attacker.example/path", wantPath: "//attacker.example/path"},
		{name: "network path with more slashes", base: "http://upstream.example", target: "////attacker.example/path"},
		{name: "backslash", base: "http://upstream.example", target: "/\\attacker.example"},
		{name: "userinfo in the query", base: "http://upstream.example", target: "/?@evil.com"},
		{name: "userinfo", base: "http://upstream.example", target: "@evil.com"},
		{name: "absolute URL", base: "http://upstream.example", target: "https://evil.com/path"},
		{name: "network path behind a base path", base: "http://upstream.example/api", target: "//attacker.example/path", wantPath: "/api//attacker.example/path"},
		// The fallback for a target that is a fragment alone and does not parse: the
		// opaque path it becomes is rooted, and stays under the base's prefix.
		{name: "malformed fragment", base: "http://upstream.example", target: "#%zz", wantPath: "/#%zz"},
		{name: "malformed fragment behind a base path", base: "http://upstream.example/api", target: "#%zz", wantPath: "/api/#%zz"},
		// A request path that spells a scheme and host of its own. Parsed behind the
		// placeholder authority it is a path and nothing else, so the upstream host
		// pinned by base cannot be replaced.
		{name: "scheme and host in the path", base: "http://upstream.example", target: "/foo://hijack.example/bar", wantPath: "/foo://hijack.example/bar"},
		// A regression guard: when the request path fails to parse cleanly the
		// fallback branch must still honor an upstream base path prefix. Without
		// this, a malformed request like "/%zz" could silently bypass a configured
		// "/api" and reach the upstream root.
		{name: "malformed path behind a base path", base: "http://upstream.example/api", target: "/%zz", wantPrefix: "/api/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := joinUpstreamPath(proxyTestParseUpstream(t, tc.base), tc.target)
			require.NotEmpty(t, out)
			parsed, err := url.Parse(out)
			require.NoError(t, err, "%q must join into a parseable URL: %q", tc.target, out)
			require.Equal(t, "http", parsed.Scheme, "scheme must not change for %q: %q", tc.target, out)
			require.Equal(t, "upstream.example", parsed.Host, "host must not change for %q: %q", tc.target, out)
			if tc.wantPath != "" {
				require.Equal(t, tc.wantPath, parsed.Path)
			}
			if tc.wantPrefix != "" {
				require.True(t, strings.HasPrefix(parsed.Path, tc.wantPrefix), "base path prefix must survive: %q", parsed.Path)
			}
		})
	}
}

func Test_Security_JoinUpstreamPath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		base   string
		target string
		want   string
	}{
		// A path prefix configured on the upstream base survives request joining
		// instead of being overwritten by the request path.
		{name: "base path prefix", base: "http://upstream.example/api", target: "/foo", want: "http://upstream.example/api/foo"},
		{name: "base path prefix and query", base: "http://upstream.example/api", target: "/foo?q=1", want: "http://upstream.example/api/foo?q=1"},
		// Trailing slash on the base should not double up.
		{name: "base path prefix with a trailing slash", base: "http://upstream.example/api/", target: "/foo", want: "http://upstream.example/api/foo"},
		{name: "query", base: "http://upstream.example", target: "/x?y=1&z=2", want: "http://upstream.example/x?y=1&z=2"},
		// The early return when the inbound request had no path component.
		{name: "empty path", base: "http://upstream.example", target: "", want: "http://upstream.example"},
		// The "?" leading-char branch, where the inbound path is just a query string.
		{
			name: "query only", base: "http://upstream.example/api", target: "?q=1",
			want: "http://upstream.example/api?q=1",
		},
		// A percent-encoded segment of the base survives the join.
		{
			name: "percent-encoded base path", base: "http://upstream.example/space%20here", target: "/sub",
			want: "http://upstream.example/space%20here/sub",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, joinUpstreamPath(proxyTestParseUpstream(t, tc.base), tc.target))
		})
	}
}

// Test_Security_FollowRedirects_ReappliesSchemePerHop verifies that a
// redirect which changes the scheme is dispatched with the new scheme.
// followRedirects re-applies currentURL.Scheme after each SetRequestURI
// because fasthttp keeps the previous scheme when req.isTLS is set — see
// https://github.com/gofiber/fiber/issues/1762. This request is not marked
// TLS, so it gets the scheme from its URL either way; the re-application is
// pinned by Test_Proxy_DoRedirects_AppliesSchemePerHopBehindTLS.
func Test_Security_FollowRedirects_ReappliesSchemePerHop(t *testing.T) {
	t.Parallel()
	var secondHopScheme string
	followRecording(t, "http://origin.example/", fasthttp.StatusFound, "https://origin.example/secure",
		nil, func(req *fasthttp.Request) {
			if string(req.URI().Path()) == "/secure" {
				secondHopScheme = string(req.URI().Scheme())
			}
		})

	require.Equal(t, "https", secondHopScheme, "scheme upgrade must carry through to the next hop")
}

func Test_Security_JoinUpstreamPath_EmptyQueryMarkersUseSlowPath(t *testing.T) {
	t.Parallel()
	baseWithEmptyQuery := proxyTestParseUpstream(t, "http://upstream.example?")
	require.True(t, baseWithEmptyQuery.ForceQuery)
	require.Equal(t, "http://upstream.example/foo?", joinUpstreamPath(baseWithEmptyQuery, "/foo"))

	base := proxyTestParseUpstream(t, "http://upstream.example")
	require.Equal(t, "http://upstream.example/foo", joinUpstreamPath(base, "/foo?"))
	require.Equal(t, "http://upstream.example/foo", joinUpstreamPath(base, "/foo#"))
}

func Test_Security_SecureTLSConfig_DefaultMinVersion(t *testing.T) {
	t.Parallel()
	out := secureTLSConfig(nil)
	require.Equal(t, uint16(tls.VersionTLS12), out.MinVersion)

	in := &tls.Config{InsecureSkipVerify: true}
	out = secureTLSConfig(in)
	require.Equal(t, uint16(tls.VersionTLS12), out.MinVersion)
	require.True(t, out.InsecureSkipVerify)
	// Caller's config must not be mutated.
	require.Equal(t, uint16(0), in.MinVersion)
}

func Test_Security_WithSecurityPolicy_RestoresDefaults(t *testing.T) {
	// No t.Parallel: mutates the global SecurityPolicy.
	custom := proxyTestAllowPrivatePolicy()
	prev := WithSecurityPolicy(custom)
	t.Cleanup(func() { WithSecurityPolicy(prev) })

	current := currentSecurityPolicy()
	require.True(t, current.AllowPrivateIPs)

	// Restoring back returns the just-installed custom policy.
	got := WithSecurityPolicy(prev)
	require.Equal(t, custom.AllowPrivateIPs, got.AllowPrivateIPs)
}

// generateLocalhostCert pulls a self-signed cert/key from fiber's
// internal tlstest helper for the downgrade-protection test.
func generateLocalhostCert(t *testing.T) (tls.Certificate, error) {
	t.Helper()
	cfg, _, err := tlstest.GetTLSConfigs()
	if err != nil {
		return tls.Certificate{}, err
	}
	if len(cfg.Certificates) == 0 {
		t.Fatal("tlstest returned no certificates")
	}
	return cfg.Certificates[0], nil
}

// Test_Security_IsBlockedIP_NilReturnsTrue covers the nil-IP guard so we
// fail closed when callers feed us a parse failure they didn't check.
func Test_Security_IsBlockedIP_NilReturnsTrue(t *testing.T) {
	t.Parallel()
	require.True(t, isBlockedIP(nil))
}

// Test_Security_IPv6Helpers_DeclineMalformedAddress pins that the IPv6 helpers decline odd-length slices.
func Test_Security_IPv6Helpers_DeclineMalformedAddress(t *testing.T) {
	t.Parallel()
	ip := net.IP{1, 2, 3}
	require.False(t, isBlockedIPv6TransitionRange(ip))
	require.Nil(t, embeddedIPv4(ip))
}

func Test_Security_ValidateHostForSSRF_EmptyHost(t *testing.T) {
	t.Parallel()
	require.ErrorIs(t, validateHostForSSRF("", nil), ErrUpstreamHostInvalid)
}

func Test_Security_ValidateHostForSSRF_PublicIPLiteral(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateHostForSSRF("8.8.8.8", nil))
}

// Test_Security_ValidateHostForSSRF_LookupFailure covers the
// LookupIP-error branch. ".invalid" is reserved (RFC 6761) and never
// resolves, so the test does not depend on outbound DNS.
func Test_Security_ValidateHostForSSRF_LookupFailure(t *testing.T) {
	t.Parallel()
	err := validateHostForSSRF("does-not-exist.invalid", nil)
	require.Error(t, err)
	// Refused like a blocked host, but told apart so a caller can answer 502 or retry.
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
	require.ErrorIs(t, err, ErrUpstreamHostUnresolvable)
	var dnsErr *net.DNSError
	require.ErrorAs(t, err, &dnsErr)
	require.NotContains(t, err.Error(), "blocked address")
}

// Test_Security_ValidateHostForSSRF_IPLiteralAllocs pins that an IP literal costs no allocation and no lookup.
func Test_Security_ValidateHostForSSRF_IPLiteralAllocs(t *testing.T) {
	// Intentionally not parallel: AllocsPerRun counts allocations process-wide and panics in a parallel test.
	for _, host := range []string{"203.0.113.5", "2606:4700:4700::1111", "::ffff:203.0.113.5"} {
		var err error
		require.Zero(t, testing.AllocsPerRun(100, func() { err = validateHostForSSRF(host, upfrontLookups) }), host)
		require.NoError(t, err, host)

		_, remembered := upfrontLookups.get(host, dnsResolver.Load())
		require.False(t, remembered, "a literal is judged where it stands: %s", host)
	}
}

// Test_Security_ValidateHostForSSRF_ZonedLiteral pins that a zoned literal is judged through the lookup.
func Test_Security_ValidateHostForSSRF_ZonedLiteral(t *testing.T) {
	t.Parallel()
	err := validateHostForSSRF("fe80::1%eth0", nil)
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
	require.ErrorContains(t, err, "fe80::1%eth0 -> fe80::1")
	require.NoError(t, validateHostForSSRF("2606:4700:4700::1111%eth0", nil))
}

// Test_Security_ValidateResolved pins what a lookup may answer: one or more addresses, none blocked.
func Test_Security_ValidateResolved(t *testing.T) {
	t.Parallel()
	public := net.IPAddr{IP: net.ParseIP("198.51.100.7")}
	private := net.IPAddr{IP: net.ParseIP("10.0.0.1")}

	_, err := validateResolved("empty.test", nil)
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
	require.ErrorIs(t, err, ErrUpstreamHostUnresolvable)
	require.ErrorContains(t, err, "empty.test could not be resolved: no addresses")

	_, err = validateResolved("mixed.test", []net.IPAddr{public, private})
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
	require.NotErrorIs(t, err, ErrUpstreamHostUnresolvable)
	require.ErrorContains(t, err, "mixed.test -> 10.0.0.1")

	ips, err := validateResolved("ok.test", []net.IPAddr{public})
	require.NoError(t, err)
	require.Len(t, ips, 1)
	require.True(t, ips[0].Equal(public.IP))
}

func Test_Security_JoinUpstreamPath_NilBase(t *testing.T) {
	t.Parallel()
	require.Empty(t, joinUpstreamPath(nil, "/foo"))
}

func Test_Security_WithSecurityPolicy_EmptyAllowedSchemesGetsDefault(t *testing.T) {
	// Intentionally not parallel: it sets a process-wide security policy and reads it back.
	prev := WithSecurityPolicy(SecurityPolicy{})
	t.Cleanup(func() { WithSecurityPolicy(prev) })
	got := currentSecurityPolicy()
	require.Equal(t, []string{"http", "https"}, got.AllowedSchemes)
}

// Test_Security_DefaultSecurityPolicy_AllowedSchemesIsCopy verifies that
// mutating the AllowedSchemes slice returned by DefaultSecurityPolicy
// cannot corrupt the package defaults — the slice must not alias the
// internal defaultAllowedSchemes backing array.
func Test_Security_DefaultSecurityPolicy_AllowedSchemesIsCopy(t *testing.T) {
	t.Parallel()
	p1 := DefaultSecurityPolicy()
	require.Equal(t, []string{"http", "https"}, p1.AllowedSchemes)

	// Adversarial mutation: try to weaken the allowlist through the
	// exported field.
	p1.AllowedSchemes[0] = "file"

	// A fresh DefaultSecurityPolicy must be unaffected.
	p2 := DefaultSecurityPolicy()
	require.Equal(t, []string{"http", "https"}, p2.AllowedSchemes,
		"DefaultSecurityPolicy must return an isolated AllowedSchemes slice")

	// The internal fallback used by schemeAllowed must be unaffected too.
	require.True(t, schemeAllowed("https", nil))
	require.False(t, schemeAllowed("file", nil))
}

// Test_Security_NormalizePolicy_EmptyAllowedSchemesIsFreshCopy verifies
// normalizePolicy honors its "always freshly allocated" contract even
// for the empty-allowlist branch: the returned slice must not alias
// defaultAllowedSchemes.
func Test_Security_NormalizePolicy_EmptyAllowedSchemesIsFreshCopy(t *testing.T) {
	t.Parallel()
	got := normalizePolicy(SecurityPolicy{})
	require.Equal(t, []string{"http", "https"}, got.AllowedSchemes)

	got.AllowedSchemes[0] = "file"

	// A second normalize must still see the clean defaults.
	again := normalizePolicy(SecurityPolicy{})
	require.Equal(t, []string{"http", "https"}, again.AllowedSchemes,
		"normalizePolicy empty branch must not alias defaultAllowedSchemes")
}

// Test_Security_ResolvePolicy_OverrideEmptySchemes covers the override
// path where the caller supplies a Config-level policy with no
// scheme allowlist set — resolvePolicy must fall back to defaults.
func Test_Security_ResolvePolicy_OverrideEmptySchemes(t *testing.T) {
	t.Parallel()
	override := &SecurityPolicy{AllowPrivateIPs: true}
	got := resolvePolicy(override)
	require.Equal(t, []string{"http", "https"}, got.AllowedSchemes)
	require.True(t, got.AllowPrivateIPs)
}

func Test_Security_DefaultSecurityPolicy_RedirectSensitiveHeadersIsEmpty(t *testing.T) {
	t.Parallel()
	require.Nil(t, DefaultSecurityPolicy().RedirectSensitiveHeaders)
}

func Test_Security_NormalizePolicy_RedirectSensitiveHeaders(t *testing.T) {
	t.Parallel()
	in := []string{" X-Api-Key", "", "X-Auth-Token\t"}
	policy := DefaultSecurityPolicy()
	policy.RedirectSensitiveHeaders = in

	got := normalizePolicy(policy).RedirectSensitiveHeaders
	require.Equal(t, []string{"X-Api-Key", "X-Auth-Token"}, got)

	in[0] = "X-Changed"
	require.Equal(t, []string{"X-Api-Key", "X-Auth-Token"}, got, "the caller's slice must not reach the policy")

	got[1] = "X-Changed"
	require.Equal(t, []string{"X-Changed", "", "X-Auth-Token\t"}, in, "the policy's slice must not reach the caller")
}

func Test_Security_WithSecurityPolicy_CopiesRedirectSensitiveHeaders(t *testing.T) {
	// Intentionally not parallel: it sets a process-wide security policy and reads it back.
	names := []string{" X-Api-Key", "X-Auth-Token"}
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	policy.RedirectSensitiveHeaders = names
	withSecurityPolicyForTest(t, policy)

	names[1] = "X-Changed"
	require.Equal(t, []string{"X-Api-Key", "X-Auth-Token"}, currentSecurityPolicy().RedirectSensitiveHeaders)
}

// Test_Security_ResolveRedirect pins what resolveRedirect makes of the Location of a
// redirect from "https://example.com".
func Test_Security_ResolveRedirect(t *testing.T) {
	t.Parallel()
	allowPrivate := proxyTestAllowPrivatePolicy()
	allowDowngrade := allowPrivate
	allowDowngrade.AllowHTTPSDowngrade = true

	for _, tc := range []struct {
		err      error
		name     string
		location string
		wantURL  string // what the Location resolves to, when it is followed
		policy   SecurityPolicy
	}{
		// The CRLF / control-byte rejection at the top of resolveRedirect.
		{name: "control characters", policy: DefaultSecurityPolicy(), location: "https://example.com/\r\nX-Inject: 1", err: fasthttp.ErrorInvalidURI},
		// The validateUpstream propagation when the redirect target's scheme is
		// outside the allowlist.
		{name: "disallowed scheme", policy: allowPrivate, location: "gopher://example.com/x", err: ErrUpstreamSchemeNotAllowed},
		// The AllowHTTPSDowngrade escape hatch.
		{name: "HTTPS to HTTP when allowed", policy: allowDowngrade, location: "http://example.com/x", wantURL: "http://example.com/x"},
		// The HTTPS to HTTP downgrade guard.
		{name: "HTTPS to HTTP", policy: allowPrivate, location: "http://example.com/x", err: ErrRedirectDowngrade},
		// The empty-host check after fasthttp's URI.Update merges in the Location. A
		// Location of "http://" produces a URI with scheme but no host.
		{name: "hostless target", policy: allowPrivate, location: "http://", err: fasthttp.ErrorInvalidURI},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, err := resolveRedirect("https://example.com", []byte(tc.location), tc.policy, nil)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, out)
			require.Equal(t, tc.wantURL, out.String())
		})
	}
}

// proxyTestFollow follows the redirects of a plain GET for start through client, at most
// maxRedirects of them, and returns the error the loop ends in.
func proxyTestFollow(t *testing.T, client *fasthttp.Client, start string, maxRedirects int, policy SecurityPolicy) error {
	t.Helper()

	req := proxyTestRequest(t)
	req.SetRequestURI(start)
	req.Header.SetMethod(fasthttp.MethodGet)

	return followRedirects(client, req, proxyTestResponse(t), maxRedirects, mustParseTestURL(t, start), policy)
}

// Test_Security_FollowRedirects_NegativeMaxBecomesZero ensures a
// negative redirect cap collapses to "no follow" — the first redirect
// returned by the upstream is propagated, no further requests are made.
func Test_Security_FollowRedirects_NegativeMaxBecomesZero(t *testing.T) {
	t.Parallel()
	client, hits := newCountingRedirectClient(map[string]redirectStep{
		"http://example.com/start": {status: fasthttp.StatusFound, location: "http://example.com/next"},
	})

	require.ErrorIs(t, proxyTestFollow(t, client.client, "http://example.com/start", -1, proxyTestAllowPrivatePolicy()), fasthttp.ErrTooManyRedirects)
	require.Equal(t, 1, *hits)
}

func Test_Security_FollowRedirects_MissingLocation(t *testing.T) {
	t.Parallel()
	client, _ := newCountingRedirectClient(map[string]redirectStep{
		"http://example.com/start": {status: fasthttp.StatusFound, location: ""},
	})

	require.ErrorIs(t, proxyTestFollow(t, client.client, "http://example.com/start", 3, proxyTestAllowPrivatePolicy()), fasthttp.ErrMissingLocation)
}

// Test_Security_FollowRedirects_LeavesCallerRequestIntact pins that a redirect changes a copy of
// the request: the handler's keeps its method, body and credentials.
func Test_Security_FollowRedirects_LeavesCallerRequestIntact(t *testing.T) {
	t.Parallel()
	type seen struct{ method, body, contentType, auth, cookie string }
	var hops []seen
	req := followRecording(t, "http://first.example/start", fasthttp.StatusSeeOther, "http://second.example/after",
		func(req *fasthttp.Request) {
			req.Header.SetMethod(fasthttp.MethodPost)
			req.Header.SetContentType(fiber.MIMEApplicationForm)
			req.SetBodyString("token=secret&b=2")
			withCredentials(req)
			require.Equal(t, "secret", string(req.PostArgs().Peek("token")), "the handler read the form")
		}, func(req *fasthttp.Request) {
			hops = append(hops, seen{
				method:      string(req.Header.Method()),
				body:        string(req.Body()),
				contentType: string(req.Header.ContentType()),
				auth:        string(req.Header.Peek(fiber.HeaderAuthorization)),
				cookie:      string(req.Header.Peek(fiber.HeaderCookie)),
			})
		})

	require.Equal(t, []seen{
		{method: fasthttp.MethodPost, body: "token=secret&b=2", contentType: fiber.MIMEApplicationForm, auth: "Bearer secret", cookie: "session=abc"},
		{method: fasthttp.MethodGet},
	}, hops)

	require.Equal(t, fasthttp.MethodPost, string(req.Header.Method()))
	require.Equal(t, "token=secret&b=2", string(req.Body()))
	require.Equal(t, fiber.MIMEApplicationForm, string(req.Header.ContentType()))
	require.Equal(t, "Bearer secret", string(req.Header.Peek(fiber.HeaderAuthorization)))
	require.Equal(t, "session=abc", string(req.Header.Peek(fiber.HeaderCookie)))
	require.Equal(t, "secret", string(req.PostArgs().Peek("token")))
}

// Test_Security_FollowRedirects_KeepsMultipartForm pins that a multipart form the handler already
// parsed survives a redirect that drops the body: removing it would delete its temp files.
func Test_Security_FollowRedirects_KeepsMultipartForm(t *testing.T) {
	t.Parallel()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	require.NoError(t, mw.WriteField("a", "1"))
	fw, err := mw.CreateFormFile("f", "f.txt")
	require.NoError(t, err)
	_, err = fw.Write([]byte("file-bytes"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := followRecording(t, "http://example.com/start", fasthttp.StatusSeeOther, "http://example.com/after",
		func(req *fasthttp.Request) {
			req.Header.SetMethod(fasthttp.MethodPost)
			req.Header.SetMultipartFormBoundary(mw.Boundary())
			req.SetBody(body.Bytes())
			_, formErr := req.MultipartForm()
			require.NoError(t, formErr)
		}, nil)

	form, err := req.MultipartForm()
	require.NoError(t, err)
	require.Equal(t, []string{"1"}, form.Value["a"])
	require.Len(t, form.File["f"], 1)
}

func Test_Security_FollowRedirects_PropagatesClientError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("dial failure")
	cli := &fasthttp.Client{Dial: func(string) (net.Conn, error) { return nil, sentinel }}

	require.ErrorIs(t, proxyTestFollow(t, cli, "http://example.com/", 1, DefaultSecurityPolicy()), sentinel)
}

// Test_Security_DomainForward_HostMismatchPassesThrough covers the host-mismatch
// branch, where the middleware is a no-op: the request goes on to what is
// behind it, and the upstream is never dialed (the unreachable port 1 would
// surface as a 500).
func Test_Security_DomainForward_HostMismatchPassesThrough(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		path   string
		status int
		route  bool // a route behind the middleware answers the path
	}{
		{name: "to the route behind", path: "/", route: true, status: fiber.StatusOK},
		{name: "with no route behind", path: "/ping", status: fiber.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(DomainForward("only-this-host.example", "http://127.0.0.1:1"))
			if tc.route {
				app.Get(tc.path, func(c fiber.Ctx) error { return c.SendString("pass-through") })
			}

			req := apptest.Req(fiber.MethodGet, tc.path, apptest.WithHost("other-host.example"))
			apptest.Expect(t, app, req, tc.status)
		})
	}
}

// Test_Security_InvalidUpstreamPanicsAtConstruction verifies that every
// constructor fails fast on a server that fails validation — DomainForward and
// BalancerForward matching Balancer's contract — instead of surfacing the
// validation error per request.
func Test_Security_InvalidUpstreamPanicsAtConstruction(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		build func()
		name  string
	}{
		{name: "DomainForward", build: func() { DomainForward("api.example", "gopher://invalid") }},
		{name: "BalancerForward", build: func() { BalancerForward([]string{"gopher://invalid"}) }},
		{name: "Balancer", build: func() { Balancer(Config{Servers: []string{"gopher://example"}}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.PanicsWithError(t, ErrUpstreamSchemeNotAllowed.Error()+": \"gopher\"", tc.build)
		})
	}
}

// Test_Security_RoundRobin_WrapsAround covers both the normal cyclic
// selection and recovery from a counter value near the uint64 limit.
func Test_Security_RoundRobin_WrapsAround(t *testing.T) {
	t.Parallel()
	a := &url.URL{Scheme: schemeHTTP, Host: "a"}
	b := &url.URL{Scheme: schemeHTTP, Host: "b"}
	c := &url.URL{Scheme: schemeHTTP, Host: "c"}

	t.Run("current index", func(t *testing.T) {
		t.Parallel()
		r := &urlRoundrobin{pool: []*url.URL{a, b}}
		r.next.Store(5)
		require.Same(t, b, r.get(), "5 %% 2 should select pool[1]")
		require.Same(t, a, r.get())
	})

	t.Run("counter limit", func(t *testing.T) {
		t.Parallel()

		// The wrap repeats one upstream, then the cycle resumes.
		r := &urlRoundrobin{pool: []*url.URL{a, b, c}}
		r.next.Store(^uint64(0) - 1)
		require.Same(t, c, r.get())
		require.Same(t, a, r.get())
		require.Same(t, a, r.get())
		require.Same(t, b, r.get())
	})
}

func Test_Security_RoundRobin_ConcurrentSelection(t *testing.T) {
	t.Parallel()
	pool := []*url.URL{
		{Scheme: schemeHTTP, Host: "a"},
		{Scheme: schemeHTTP, Host: "b"},
		{Scheme: schemeHTTP, Host: "c"},
	}

	r := &urlRoundrobin{pool: pool}

	var counts [3]atomic.Int32
	var wg sync.WaitGroup
	for range 300 {
		wg.Go(func() { counts[slices.Index(pool, r.get())].Add(1) })
	}
	wg.Wait()

	for i := range counts {
		require.Equal(t, int32(100), counts[i].Load())
	}
}

func Test_Security_Balancer_BuildsHTTPSHostClient(t *testing.T) {
	t.Parallel()
	policy := proxyTestAllowPrivatePolicy()
	// We can't reach the upstream, but we can confirm the handler is
	// installed without panicking and the TLS flag was set by inspecting
	// the resulting LBClient via the public Balancer (which would have
	// panicked at construction time for an invalid TLS upstream).
	h := Balancer(Config{Servers: []string{"https://127.0.0.1:0"}, SecurityPolicy: &policy})
	require.NotNil(t, h)
}

// Test_Security_DialChecked_BlocksLoopback verifies the dial-time SSRF
// guard rejects both literal and hostname loopback targets before any
// connection is made — this is what defeats DNS rebinding.
func Test_Security_DialChecked_BlocksLoopback(t *testing.T) {
	t.Parallel()
	for _, addr := range []string{"127.0.0.1:80", "localhost:80", "localhost:65535"} {
		t.Run(addr, func(t *testing.T) {
			t.Parallel()
			_, err := newDialGuard(false).dialChecked(addr)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrUpstreamHostBlocked)
		})
	}
}

// Test_Security_DialChecked_RejectsMalformedAddr covers the SplitHostPort error, which keeps the reason.
func Test_Security_DialChecked_RejectsMalformedAddr(t *testing.T) {
	t.Parallel()
	dial := newDialGuard(false).dialChecked
	for addr, reason := range map[string]string{
		"no-port-here": "missing port in address",
		"a:b:c":        "too many colons in address",
		"[::1":         "missing ']' in address",
	} {
		_, err := dial(addr)
		require.Error(t, err, addr)
		require.NotErrorIs(t, err, ErrUpstreamHostBlocked, addr)
		var addrErr *net.AddrError
		require.ErrorAs(t, err, &addrErr, addr)
		require.ErrorContains(t, err, reason, addr)
	}
}

// Test_Security_Balancer_HostnameDefersToDial verifies that a hostname
// upstream is NOT resolved at construction time (so transient DNS failures
// cannot crash startup) and is instead validated at dial time.
func Test_Security_Balancer_HostnameDefersToDial(t *testing.T) {
	t.Parallel()
	policy := DefaultSecurityPolicy() // AllowPrivateIPs == false

	// Must not panic even though the host cannot be resolved right now.
	var h fiber.Handler
	require.NotPanics(t, func() {
		h = Balancer(Config{Servers: []string{"http://does-not-exist.invalid:80"}, SecurityPolicy: &policy})
	})
	require.NotNil(t, h)

	// At request time the dial-time guard fails the lookup, surfacing a 5xx
	// rather than reaching any upstream.
	app := fiber.New()
	app.Use(h)
	req := apptest.Req(fiber.MethodGet, "/", apptest.WithHost("does-not-exist.invalid"))
	resp := apptest.Do(t, app, req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: false})
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// redirectStep / countingRedirectClient are minimal stubs for the
// followRedirects branches without an actual network.
type redirectStep struct {
	location string
	status   int
}

type countingRedirectClient struct {
	client *fasthttp.Client
	calls  *int
	// onRequest, if set, is invoked with each request as it reaches the
	// transport, letting tests inspect headers per hop.
	onRequest func(req *fasthttp.Request)
}

// newCountingRedirectClient wires a fasthttp.Client to a recorder
// RoundTripper so followRedirects can drive a redirect chain without
// any sockets or real DNS. The returned int pointer tracks Do calls.
//
//nolint:gocritic // pair-return is idiomatic for test fixtures
func newCountingRedirectClient(steps map[string]redirectStep) (*countingRedirectClient, *int) {
	calls := 0
	c := &countingRedirectClient{calls: &calls}
	c.client = &fasthttp.Client{
		Transport: roundTripperFunc(func(req *fasthttp.Request, resp *fasthttp.Response) error {
			calls++
			if c.onRequest != nil {
				c.onRequest(req)
			}
			step, ok := steps[req.URI().String()]
			if !ok {
				resp.Reset()
				resp.Header.SetStatusCode(fasthttp.StatusOK)
				return nil
			}
			resp.Reset()
			resp.Header.SetStatusCode(step.status)
			if step.location != "" {
				resp.Header.Set(fiber.HeaderLocation, step.location)
			}
			return nil
		}),
	}
	return c, &calls
}

// roundTripperFunc adapts a plain function to fasthttp.RoundTripper.
type roundTripperFunc func(req *fasthttp.Request, resp *fasthttp.Response) error

func (f roundTripperFunc) RoundTrip(_ *fasthttp.HostClient, req *fasthttp.Request, resp *fasthttp.Response) (bool, error) {
	return false, f(req, resp)
}

// mustParseTestURL is a shared parse-or-fail helper for tests that pass
// a *url.URL into followRedirects's initialURL argument.
func mustParseTestURL(tb testing.TB, raw string) *url.URL {
	tb.Helper()
	u, err := url.Parse(raw)
	require.NoError(tb, err)
	return u
}

// proxyTestRequest returns a blank fasthttp request that goes back to the pool when tb ends.
func proxyTestRequest(tb testing.TB) *fasthttp.Request {
	tb.Helper()

	req := fasthttp.AcquireRequest()
	tb.Cleanup(func() { fasthttp.ReleaseRequest(req) })
	return req
}

// proxyTestReadRequest parses raw into req, as the server does for a request it read off the wire.
func proxyTestReadRequest(tb testing.TB, req *fasthttp.Request, raw string) {
	tb.Helper()

	require.NoError(tb, req.Read(bufio.NewReader(strings.NewReader(raw))))
}

// proxyTestResponse returns a blank fasthttp response that goes back to the pool when tb ends.
func proxyTestResponse(tb testing.TB) *fasthttp.Response {
	tb.Helper()

	resp := fasthttp.AcquireResponse()
	tb.Cleanup(func() { fasthttp.ReleaseResponse(resp) })
	return resp
}

// followRecording follows a GET for start that setup may reshape through a redirect to location with
// status, and hands record each request as it reaches the transport. It returns the caller's request.
func followRecording(
	t *testing.T, start string, status int, location string, setup, record func(req *fasthttp.Request),
) *fasthttp.Request {
	t.Helper()

	req, err := followRecordingErr(t, start, status, location, setup, record)
	require.NoError(t, err)
	return req
}

// followRecordingErr is followRecording that returns the error the loop ends in.
func followRecordingErr(
	t *testing.T, start string, status int, location string, setup, record func(req *fasthttp.Request),
) (*fasthttp.Request, error) {
	t.Helper()

	steps := map[string]redirectStep{start: {status: status, location: location}}
	return followRecordingChain(t, start, steps, setup, record)
}

// redirectTestPolicy is the policy the redirect tests follow under: private addresses and an HTTPS downgrade
// are allowed, and listed names the application headers a hop to another origin drops.
func redirectTestPolicy(listed ...string) SecurityPolicy {
	policy := proxyTestAllowPrivatePolicy()
	policy.AllowHTTPSDowngrade = true
	policy.RedirectSensitiveHeaders = listed
	return policy
}

// followRecordingChain follows start through steps, as followRecordingErr does for one.
func followRecordingChain(
	t *testing.T, start string, steps map[string]redirectStep, setup, record func(req *fasthttp.Request),
) (*fasthttp.Request, error) {
	t.Helper()

	return followRecordingPolicy(t, redirectTestPolicy(), start, steps, setup, record)
}

// followRecordingPolicy is followRecordingChain under policy.
func followRecordingPolicy(
	t *testing.T, policy SecurityPolicy, start string, steps map[string]redirectStep, setup, record func(req *fasthttp.Request),
) (*fasthttp.Request, error) {
	t.Helper()

	client, _ := newCountingRedirectClient(steps)
	client.onRequest = record

	req := proxyTestRequest(t)
	resp := proxyTestResponse(t)
	req.SetRequestURI(start)
	req.Header.SetMethod(fasthttp.MethodGet)
	if setup != nil {
		setup(req)
	}

	return req, followRedirects(client.client, req, resp, 3, mustParseTestURL(t, start), policy)
}

// withCredentials gives req what a redirect to another origin must not carry.
func withCredentials(req *fasthttp.Request) {
	req.Header.Set(fiber.HeaderAuthorization, "Bearer secret")
	req.Header.Set(fiber.HeaderCookie, "session=abc")
}

// redirectRewriteCases is what a redirect leaves of a request, by method and status.
var redirectRewriteCases = []struct {
	name       string
	method     string
	wantMethod string
	status     int
	wantBody   bool
}{
	{name: "301 POST", method: fasthttp.MethodPost, status: fasthttp.StatusMovedPermanently, wantMethod: fasthttp.MethodGet},
	{name: "302 POST", method: fasthttp.MethodPost, status: fasthttp.StatusFound, wantMethod: fasthttp.MethodGet},
	{name: "301 PUT", method: fasthttp.MethodPut, status: fasthttp.StatusMovedPermanently, wantMethod: fasthttp.MethodPut, wantBody: true},
	{name: "302 PATCH", method: fasthttp.MethodPatch, status: fasthttp.StatusFound, wantMethod: fasthttp.MethodPatch, wantBody: true},
	{name: "302 DELETE", method: fasthttp.MethodDelete, status: fasthttp.StatusFound, wantMethod: fasthttp.MethodDelete, wantBody: true},
	{name: "303 POST", method: fasthttp.MethodPost, status: fasthttp.StatusSeeOther, wantMethod: fasthttp.MethodGet},
	{name: "303 PUT", method: fasthttp.MethodPut, status: fasthttp.StatusSeeOther, wantMethod: fasthttp.MethodGet},
	{name: "303 PATCH", method: fasthttp.MethodPatch, status: fasthttp.StatusSeeOther, wantMethod: fasthttp.MethodGet},
	{name: "303 DELETE", method: fasthttp.MethodDelete, status: fasthttp.StatusSeeOther, wantMethod: fasthttp.MethodGet},
	{name: "303 GET", method: fasthttp.MethodGet, status: fasthttp.StatusSeeOther, wantMethod: fasthttp.MethodGet, wantBody: true},
	{name: "303 HEAD", method: fasthttp.MethodHead, status: fasthttp.StatusSeeOther, wantMethod: fasthttp.MethodHead, wantBody: true},
	{name: "307 POST", method: fasthttp.MethodPost, status: fasthttp.StatusTemporaryRedirect, wantMethod: fasthttp.MethodPost, wantBody: true},
	{name: "308 PUT", method: fasthttp.MethodPut, status: fasthttp.StatusPermanentRedirect, wantMethod: fasthttp.MethodPut, wantBody: true},
	{name: "200 is no redirect", method: fasthttp.MethodPost, status: fasthttp.StatusOK, wantMethod: fasthttp.MethodPost, wantBody: true},
}

// Test_Security_RedirectRequest pins the request a redirect leaves for the next hop (RFC 9110 §15.4).
func Test_Security_RedirectRequest(t *testing.T) {
	t.Parallel()
	for _, tc := range redirectRewriteCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := proxyTestRequest(t)
			req.Header.SetMethod(tc.method)
			req.Header.SetContentType("text/plain")
			req.SetBodyString("payload")
			req.Header.SetContentLength(len("payload"))

			changes := redirectChangesMethod(req, tc.status)
			require.Equal(t, !tc.wantBody, changes)
			if changes {
				redirectRequest(req)
			}

			require.Equal(t, tc.wantMethod, string(req.Header.Method()))
			if tc.wantBody {
				require.Equal(t, "payload", string(req.Body()))
				require.Equal(t, "text/plain", string(req.Header.ContentType()))
				require.Equal(t, len("payload"), req.Header.ContentLength())
				return
			}
			require.Empty(t, req.Body())
			require.Empty(t, req.Header.ContentType())
			require.Zero(t, req.Header.ContentLength())
		})
	}
}

// Test_Security_RedirectRequest_LeavesTheHandlerBody pins that rewriting a hop copy leaves the
// handler's body alone.
func Test_Security_RedirectRequest_LeavesTheHandlerBody(t *testing.T) {
	t.Parallel()
	req := proxyTestRequest(t)
	req.Header.SetMethod(fasthttp.MethodPut)
	req.SetBodyString("payload-of-request-A")
	held := req.Body()

	hop := copyRequest(req)
	redirectRequest(hop)

	// Another request takes a pooled body buffer, as another connection would.
	other := proxyTestRequest(t)
	other.SetBodyString("payload-of-request-B")

	require.Equal(t, "payload-of-request-A", string(held))
	require.Empty(t, hop.Body())
}

// Test_Security_RedirectRequest_DropsParsedForm pins that a parsed form goes with the body.
func Test_Security_RedirectRequest_DropsParsedForm(t *testing.T) {
	t.Parallel()
	for _, method := range []string{fasthttp.MethodPost, fasthttp.MethodPut} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			req := proxyTestRequest(t)
			req.SetRequestURI("http://example.com/start")
			req.Header.SetMethod(method)
			req.Header.SetContentType(fiber.MIMEApplicationForm)
			req.SetBodyString("token=secret&b=2")
			require.Equal(t, "secret", string(req.PostArgs().Peek("token")), "the handler read the form")

			redirectRequest(req)

			wire := req.String()
			requestLine, _, _ := strings.Cut(wire, "\r\n")
			require.Equal(t, "GET /start HTTP/1.1", requestLine)
			require.NotContains(t, wire, "token=secret")
			require.NotContains(t, wire, "Content-")
		})
	}
}

// Test_Security_RedirectRequest_DropsFraming pins that a dropped chunked body takes its framing with it.
func Test_Security_RedirectRequest_DropsFraming(t *testing.T) {
	t.Parallel()
	const raw = "POST /start HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\nTrailer: X-Sum\r\n\r\n" +
		"3\r\nabc\r\n0\r\nX-Sum: 1\r\n\r\n"
	req := proxyTestRequest(t)
	proxyTestReadRequest(t, req, raw)
	require.Equal(t, -1, req.Header.ContentLength(), "the request is chunked")

	redirectRequest(req)

	wire := req.String()
	requestLine, _, _ := strings.Cut(wire, "\r\n")
	require.Equal(t, "GET /start HTTP/1.1", requestLine)
	require.NotContains(t, wire, "Transfer-Encoding")
	require.NotContains(t, wire, "Trailer")
	require.NotContains(t, wire, "abc")
}

// Test_Security_FollowRedirects_MethodPerStatus drives the same cases through the redirect loop.
func Test_Security_FollowRedirects_MethodPerStatus(t *testing.T) {
	t.Parallel()
	for _, tc := range redirectRewriteCases {
		if !fasthttp.StatusCodeIsRedirect(tc.status) {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var hops []string
			followRecording(t, "http://example.com/start", tc.status, "http://example.com/after",
				func(req *fasthttp.Request) {
					req.Header.SetMethod(tc.method)
					req.SetBodyString("payload")
				}, func(req *fasthttp.Request) {
					hops = append(hops, string(req.Header.Method())+" "+string(req.Body()))
				})

			second := tc.wantMethod + " "
			if tc.wantBody {
				second += "payload"
			}
			require.Equal(t, []string{tc.method + " payload", second}, hops)
		})
	}
}

// Test_Security_FollowRedirects_StripsCredentialsOnOriginChange pins what the next hop of a redirect carries of the
// request's headers: an origin is scheme, host and port (RFC 6454), and a change of any loses the credentials and the
// names SecurityPolicy.RedirectSensitiveHeaders lists, a spelled-out default port does not. The application's other
// headers go on.
func Test_Security_FollowRedirects_StripsCredentialsOnOriginChange(t *testing.T) {
	t.Parallel()
	const first = "http://first.example/start"
	credentials := []string{fiber.HeaderAuthorization, fiber.HeaderCookie}
	key, both := []string{"X-Api-Key"}, []string{"X-Api-Key", "X-Auth-Token"}
	withKey := append(slices.Clone(credentials), key...)
	for _, tc := range []struct {
		name, from, to string
		listed         []string // SecurityPolicy.RedirectSensitiveHeaders
		lost           []string // the headers of the request the next hop does not carry
	}{
		{name: "same scheme, host and port", from: "http://same.example:8080/", to: "http://same.example:8080/next"},
		{name: "default http port spelled", from: "http://same.example/", to: "http://same.example:80/next"},
		{name: "default https port spelled", from: "https://same.example/", to: "https://same.example:443/next"},
		{name: "other host", from: "http://first.example/", to: "http://second.example/next", lost: credentials},
		{name: "other port", from: "http://same.example:8080/", to: "http://same.example:9090/next", lost: credentials},
		{name: "http to https", from: "http://same.example/", to: "https://same.example/next", lost: credentials},
		{name: "https to http", from: "https://same.example/", to: "http://same.example/next", lost: credentials},
		{
			name: "https with the http port", from: "https://same.example/", to: "https://same.example:80/next",
			lost: credentials,
		},
		{
			name: "a listed name is dropped, an unlisted one is kept", from: first, to: "http://second.example/next",
			listed: key, lost: withKey,
		},
		{name: "another scheme counts", from: first, to: "https://first.example/next", listed: key, lost: withKey},
		{name: "another port counts", from: first, to: "http://first.example:8080/next", listed: key, lost: withKey},
		{name: "the same origin keeps every header", from: first, to: "http://first.example/next", listed: both},
		{
			name: "nothing listed forwards the application's headers and drops the built-in set",
			from: first, to: "http://second.example/next", lost: credentials,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sent := map[string]string{
				fiber.HeaderAuthorization: "Bearer secret", fiber.HeaderCookie: "session=abc",
				"X-Api-Key": "key", "X-Auth-Token": "tok",
			}
			var next map[string]string // what the hop to /next carries, nil when it was not made
			_, err := followRecordingPolicy(t, redirectTestPolicy(tc.listed...), tc.from, map[string]redirectStep{
				tc.from: {status: fasthttp.StatusFound, location: tc.to},
			}, func(req *fasthttp.Request) {
				withCredentials(req)
				withAppHeaders(req)
			}, func(req *fasthttp.Request) {
				if string(req.URI().Path()) == "/next" {
					next = map[string]string{}
					for name := range sent {
						next[name] = string(req.Header.Peek(name))
					}
				}
			})
			require.NoError(t, err)

			require.NotNil(t, next, "the redirect was not followed")
			for name, value := range sent {
				if slices.Contains(tc.lost, name) {
					value = ""
				}
				require.Equal(t, value, next[name], name)
			}
		})
	}
}

func withAppHeaders(req *fasthttp.Request) {
	req.Header.Set("X-Api-Key", "key")
	req.Header.Set("X-Auth-Token", "tok")
}

func Test_Security_FollowRedirects_RedirectSensitiveHeadersStayGone(t *testing.T) {
	t.Parallel()
	sawKey := map[string]string{}
	req, err := followRecordingPolicy(t, redirectTestPolicy("X-Api-Key"), "http://first.example/start", map[string]redirectStep{
		"http://first.example/start": {status: fasthttp.StatusFound, location: "http://second.example/mid"},
		"http://second.example/mid":  {status: fasthttp.StatusFound, location: "http://first.example/back"},
	}, withAppHeaders, func(req *fasthttp.Request) {
		sawKey[string(req.URI().Path())] = string(req.Header.Peek("X-Api-Key"))
	})
	require.NoError(t, err)

	require.Equal(t, map[string]string{"/start": "key", "/mid": "", "/back": ""}, sawKey)
	require.Equal(t, "key", string(req.Header.Peek("X-Api-Key")), "the caller's request keeps its header")
}

func Test_Security_FollowRedirects_RedirectSensitiveHeadersWhateverTheirCase(t *testing.T) {
	t.Parallel()
	var lines []int
	_, err := followRecordingPolicy(t, redirectTestPolicy("X-Api-Key"), "http://first.example/start", map[string]redirectStep{
		"http://first.example/start": {status: fasthttp.StatusFound, location: "http://second.example/next"},
	}, func(req *fasthttp.Request) {
		// Header.Del is byte-exact under DisableNormalizing.
		req.Header.DisableNormalizing()
		req.Header.Add("x-api-key", "k1")
		req.Header.Add("X-API-KEY", "k2")
	}, func(req *fasthttp.Request) {
		lines = append(lines, len(fieldname.Lines(&req.Header, "X-Api-Key", false)))
	})
	require.NoError(t, err)

	require.Equal(t, []int{2, 0}, lines)
}

// Test_Security_FollowRedirects_SpellingLimit pins that under DisableNormalizing a hop is refused, not
// sent, when the fields it drops are spelled in more ways than one per name plus maxConnectionOptions.
func Test_Security_FollowRedirects_SpellingLimit(t *testing.T) {
	t.Parallel()
	const start = "http://first.example/start"
	for name, tc := range map[string]struct {
		location string
		field    string
		names    int
		status   int
	}{
		"303 drops the content fields": {
			status: fasthttp.StatusSeeOther, location: "http://first.example/next", field: "content-language", names: len(contentFields),
		},
		"another origin drops the credentials": {
			status: fasthttp.StatusTemporaryRedirect, location: "http://second.example/next", field: "authorization", names: len(crosshost.SensitiveHeaders),
		},
	} {
		for over, label := range []string{"at the limit", "one over"} {
			t.Run(name+"/"+label, func(t *testing.T) {
				t.Parallel()
				var reached bool
				_, err := followRecordingPolicy(t, redirectTestPolicy(), start, map[string]redirectStep{
					start: {status: tc.status, location: tc.location},
				}, func(req *fasthttp.Request) {
					req.Header.DisableNormalizing()
					req.Header.SetMethod(fasthttp.MethodPost)
					for _, k := range caseSpellings(tc.field, tc.names+maxConnectionOptions+over) {
						req.Header.Add(k, "v")
					}
				}, func(req *fasthttp.Request) { reached = reached || string(req.URI().Path()) == "/next" })
				if over == 0 {
					require.NoError(t, err)
					require.True(t, reached)
					return
				}
				require.ErrorIs(t, err, fiber.ErrRequestHeaderFieldsTooLarge)
				require.False(t, reached)
			})
		}
	}
}

// Test_Security_DoRedirects_ErrorDropsUpstreamResponse pins that a redirect DoRedirects gives up on
// does not reach the client along with the error.
func Test_Security_DoRedirects_ErrorDropsUpstreamResponse(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		c.Set(fiber.HeaderSetCookie, "up=secret")
		// Another port is another origin, so this hop drops the credentials.
		c.Location("http://127.0.0.1:1/next")
		return c.SendStatus(fiber.StatusTemporaryRedirect)
	})

	for name, tc := range map[string]struct {
		redirects, spellings, status int
	}{
		"too many redirects": {status: fiber.StatusInternalServerError},
		"too many spellings": {
			redirects: 1, spellings: len(crosshost.SensitiveHeaders) + maxConnectionOptions + 1, status: fiber.StatusRequestHeaderFieldsTooLarge,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
			app.Get("/", func(c fiber.Ctx) error { return DoRedirects(c, "http://"+addr, tc.redirects) })
			req := apptest.Req(fiber.MethodGet, "/")
			for _, k := range caseSpellings("authorization", tc.spellings) {
				req.Header[k] = []string{"v"}
			}
			resp := apptest.Expect(t, app, req, tc.status)
			require.Empty(t, resp.Header.Values(fiber.HeaderSetCookie))
			require.Empty(t, resp.Header.Values(fiber.HeaderLocation))
		})
	}
}

// Test_Security_FollowRedirects_CopiedHopKeepsPreParsedMultipart pins that a copied hop keeps the body
// of a multipart form the server parsed ahead of the handler: CopyTo leaves such a form out.
func Test_Security_FollowRedirects_CopiedHopKeepsPreParsedMultipart(t *testing.T) {
	t.Parallel()
	var form bytes.Buffer
	mw := multipart.NewWriter(&form)
	require.NoError(t, mw.WriteField("field", "value"))
	fw, err := mw.CreateFormFile("file", "f.txt")
	require.NoError(t, err)
	_, err = fw.Write([]byte("file-bytes"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	raw := "POST http://first.example/start HTTP/1.1\r\nHost: first.example\r\n" +
		"Content-Type: multipart/form-data; boundary=" + mw.Boundary() + "\r\n" +
		"Content-Length: " + strconv.Itoa(form.Len()) + "\r\n\r\n" + form.String()

	var bodies []string
	req := followRecording(t, "http://first.example/start", fasthttp.StatusTemporaryRedirect,
		"http://second.example/next", func(req *fasthttp.Request) {
			proxyTestReadRequest(t, req, raw)
		}, func(req *fasthttp.Request) { bodies = append(bodies, string(req.Body())) })

	require.Len(t, bodies, 2)
	for hop, body := range bodies {
		require.Contains(t, body, "value", "hop %d", hop)
		require.Contains(t, body, "file-bytes", "hop %d", hop)
	}

	// The caller's request still holds its form.
	parsed, err := req.MultipartForm()
	require.NoError(t, err)
	require.Equal(t, []string{"value"}, parsed.Value["field"])
	require.Len(t, parsed.File["file"], 1)
}

// Test_Security_FollowRedirects_CopiedHopKeepsBody pins that a copied hop that keeps its method sends
// the request's body again.
func Test_Security_FollowRedirects_CopiedHopKeepsBody(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		method string
		body   string
		status int
	}{
		"307 POST":       {method: fasthttp.MethodPost, status: fasthttp.StatusTemporaryRedirect, body: "payload"},
		"308 PUT":        {method: fasthttp.MethodPut, status: fasthttp.StatusPermanentRedirect, body: "payload"},
		"302 PUT":        {method: fasthttp.MethodPut, status: fasthttp.StatusFound, body: "payload"},
		"307 without":    {method: fasthttp.MethodPost, status: fasthttp.StatusTemporaryRedirect},
		"307 GET (none)": {method: fasthttp.MethodGet, status: fasthttp.StatusTemporaryRedirect},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var hops []string
			followRecording(t, "http://first.example/start", tc.status, "http://second.example/next",
				func(req *fasthttp.Request) {
					req.Header.SetMethod(tc.method)
					req.SetBodyString(tc.body)
				}, func(req *fasthttp.Request) {
					hops = append(hops, string(req.Header.Method())+" "+string(req.Body()))
				})

			want := tc.method + " " + tc.body
			require.Equal(t, []string{want, want}, hops)
		})
	}
}

// closeCounter counts the Close calls fasthttp makes on user values it releases.
type closeCounter struct{ closed atomic.Int32 }

func (c *closeCounter) Close() error {
	c.closed.Add(1)
	return nil
}

// Test_Security_FollowRedirects_CopiedHopKeepsTimeoutAndUserValues pins that a copied hop carries the
// request timeout and the user values CopyTo leaves out, and does not Close the values it shares.
func Test_Security_FollowRedirects_CopiedHopKeepsTimeoutAndUserValues(t *testing.T) {
	t.Parallel()
	type hopState struct {
		trace   any
		timeout time.Duration
	}
	for name, tc := range map[string]struct {
		method   string
		location string
		status   int
	}{
		"another origin": {method: fasthttp.MethodGet, status: fasthttp.StatusTemporaryRedirect, location: "http://second.example/next"},
		"another method": {method: fasthttp.MethodPost, status: fasthttp.StatusFound, location: "http://first.example/next"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			closer := &closeCounter{}
			var hops []hopState
			followRecording(t, "http://first.example/", tc.status, tc.location, func(req *fasthttp.Request) {
				req.Header.SetMethod(tc.method)
				req.SetTimeout(750 * time.Millisecond)
				req.SetUserValue("trace", "abc")
				req.SetUserValue("closer", closer)
			}, func(req *fasthttp.Request) {
				hops = append(hops, hopState{trace: req.UserValue("trace"), timeout: req.GetTimeOut()})
			})

			require.Len(t, hops, 2)
			for hop, state := range hops {
				require.Equal(t, "abc", state.trace, "hop %d", hop)
				// The client hands the transport what is left of the timeout.
				require.Positive(t, state.timeout, "hop %d", hop)
				require.LessOrEqual(t, state.timeout, 750*time.Millisecond, "hop %d", hop)
			}
			require.Zero(t, closer.closed.Load(), "the copy must not Close the caller's values")
		})
	}
}

// Test_Security_FollowRedirects_StreamedBody pins that a redirect keeping the body of a streamed
// request fails, as fasthttp's own loop does, and that one dropping the body goes on.
func Test_Security_FollowRedirects_StreamedBody(t *testing.T) {
	t.Parallel()
	for _, tc := range redirectRewriteCases {
		if !fasthttp.StatusCodeIsRedirect(tc.status) {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var methods []string
			_, err := followRecordingErr(t, "http://example.com/start", tc.status, "http://example.com/after",
				func(req *fasthttp.Request) {
					req.Header.SetMethod(tc.method)
					req.SetBodyStream(strings.NewReader("payload"), len("payload"))
				}, func(req *fasthttp.Request) {
					methods = append(methods, string(req.Header.Method()))
					if req.IsBodyStream() { // sending the request consumes the stream
						body, readErr := io.ReadAll(req.BodyStream())
						require.NoError(t, readErr)
						require.Equal(t, "payload", string(body))
						require.NoError(t, req.CloseBodyStream())
					}
				})

			if tc.wantBody {
				require.ErrorIs(t, err, fasthttp.ErrRedirectBodyStream)
				require.Equal(t, []string{tc.method}, methods, "nothing is replayed")
				return
			}
			require.NoError(t, err)
			require.Equal(t, []string{tc.method, fasthttp.MethodGet}, methods)
		})
	}
}

// streamedRequest makes req what fasthttp's server hands a handler when StreamRequestBody is on: the
// head parsed, up to 8 KiB of a known-length body read ahead, the rest behind a stream. raw is the request.
func streamedRequest(tb testing.TB, req *fasthttp.Request, raw string) {
	tb.Helper()

	br := bufio.NewReader(strings.NewReader(raw))
	require.NoError(tb, req.Header.Read(br))
	require.NoError(tb, req.ContinueReadBodyStream(br, 1<<20))
	require.True(tb, req.IsBodyStream())
}

// Test_Security_FollowRedirects_ServerStreamedBody pins that a body-keeping redirect of a server-streamed
// request is followed when the server read the whole body ahead (up to 8 KiB), and fails otherwise.
func Test_Security_FollowRedirects_ServerStreamedBody(t *testing.T) {
	t.Parallel()
	const readAhead = 8 * 1024
	for _, tc := range []struct {
		name    string
		framing string
		onWire  string // the body as it is on the connection
		payload string // the body a hop sends
		wantErr bool
	}{
		{name: "empty", framing: "Content-Length: 0"},
		{name: "small", framing: "Content-Length: 7", onWire: "payload", payload: "payload"},
		{name: "as much as is read ahead", framing: "Content-Length: " + strconv.Itoa(readAhead), onWire: strings.Repeat("a", readAhead), payload: strings.Repeat("a", readAhead)},
		{name: "more than is read ahead", framing: "Content-Length: " + strconv.Itoa(readAhead+1), onWire: strings.Repeat("a", readAhead+1), wantErr: true},
		{name: "chunked", framing: "Transfer-Encoding: chunked", onWire: "7\r\npayload\r\n0\r\n\r\n", wantErr: true},
	} {
		for _, location := range []string{"http://example.com/after", "http://other.example/after"} {
			for _, status := range []int{fasthttp.StatusTemporaryRedirect, fasthttp.StatusPermanentRedirect} {
				t.Run(tc.name+" "+strconv.Itoa(status)+" to "+location, func(t *testing.T) {
					t.Parallel()
					var wires []string
					_, err := followRecordingErr(t, "http://example.com/start", status, location,
						func(req *fasthttp.Request) {
							streamedRequest(t, req, "POST /start HTTP/1.1\r\nHost: example.com\r\n"+tc.framing+"\r\n\r\n"+tc.onWire)
						}, func(req *fasthttp.Request) {
							wires = append(wires, req.String()) // sending the request consumes the stream
						})

					if tc.wantErr {
						require.ErrorIs(t, err, fasthttp.ErrRedirectBodyStream)
						require.Len(t, wires, 1, "nothing is sent again")
						return
					}
					require.NoError(t, err)
					require.Len(t, wires, 2)
					for hop, wire := range wires {
						head, body, found := strings.Cut(wire, "\r\n\r\n")
						require.True(t, found, "hop %d", hop)
						require.Contains(t, head, "Content-Length: "+strconv.Itoa(len(tc.payload)), "hop %d", hop)
						require.Equal(t, tc.payload, body, "hop %d", hop)
					}
				})
			}
		}
	}
}

// Test_Security_FollowRedirects_StreamedBodyEndsWithTheMethod pins that a redirect making a streamed
// request a GET ends the stream rule, so a later method-keeping redirect is followed.
func Test_Security_FollowRedirects_StreamedBodyEndsWithTheMethod(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ framing, onWire string }{
		"known length": {framing: "Content-Length: 7", onWire: "payload"},
		"chunked":      {framing: "Transfer-Encoding: chunked", onWire: "7\r\npayload\r\n0\r\n\r\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var methods []string
			_, err := followRecordingChain(t, "http://example.com/start", map[string]redirectStep{
				"http://example.com/start": {status: fasthttp.StatusSeeOther, location: "http://example.com/mid"},
				"http://example.com/mid":   {status: fasthttp.StatusTemporaryRedirect, location: "http://example.com/end"},
			}, func(req *fasthttp.Request) {
				streamedRequest(t, req, "POST /start HTTP/1.1\r\nHost: example.com\r\n"+tc.framing+"\r\n\r\n"+tc.onWire)
			}, func(req *fasthttp.Request) {
				methods = append(methods, string(req.Header.Method()))
				_ = req.String() // sending the request consumes the stream
			})

			require.NoError(t, err)
			require.Equal(t, []string{fasthttp.MethodPost, fasthttp.MethodGet, fasthttp.MethodGet}, methods)
		})
	}
}

// Test_Security_FollowRedirects_UnreadStream pins that a stream the transport left unread is not
// buffered by the redirect, which fails.
func Test_Security_FollowRedirects_UnreadStream(t *testing.T) {
	t.Parallel()
	reader := &countingReader{Reader: strings.NewReader("payload")}
	_, err := followRecordingErr(t, "http://example.com/start", fasthttp.StatusTemporaryRedirect,
		"http://example.com/after", func(req *fasthttp.Request) {
			req.Header.SetMethod(fasthttp.MethodPost)
			req.SetBodyStream(reader, len("payload"))
		}, nil)

	require.ErrorIs(t, err, fasthttp.ErrRedirectBodyStream)
	require.Zero(t, reader.read.Load(), "the stream stays unread")
}

type countingReader struct {
	io.Reader
	read atomic.Int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read.Add(int64(n))
	return n, err //nolint:wrapcheck // a pass-through
}

// contentFieldNames are the fields a request without content must not carry (RFC 9110 §8, §10.1.1).
var contentFieldNames = []string{
	fiber.HeaderContentEncoding, fiber.HeaderContentLanguage, fiber.HeaderContentLocation,
	fiber.HeaderExpect, "Digest", "Content-MD5",
}

// Test_Security_RedirectRequest_DropsContentFields pins that the fields describing or expecting
// content go with it, whatever case they are stored under.
func Test_Security_RedirectRequest_DropsContentFields(t *testing.T) {
	t.Parallel()
	for name, canonical := range map[string]bool{"canonical names": true, "names as sent": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := proxyTestRequest(t)
			if !canonical {
				req.Header.DisableNormalizing()
			}
			req.SetRequestURI("http://example.com/")
			req.Header.SetMethod(fasthttp.MethodPost)
			for _, field := range contentFieldNames {
				if !canonical {
					field = strings.ToLower(field)
				}
				req.Header.Set(field, "x")
			}
			req.Header.Set("X-Kept", "kept")
			req.SetBodyString("payload")

			redirectRequest(req)

			wire := strings.ToLower(req.String())
			for _, field := range contentFieldNames {
				require.NotContains(t, wire, strings.ToLower(field)+":", field)
			}
			require.Contains(t, wire, "x-kept: kept")
		})
	}
}

// Test_Security_FollowRedirects_ContentFieldsFollowTheBody pins that the fields describing or
// expecting content leave with a body a redirect drops, and stay with one it repeats.
func Test_Security_FollowRedirects_ContentFieldsFollowTheBody(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		status int
		kept   bool
	}{
		"302 drops the body": {status: fasthttp.StatusFound},
		"307 repeats it":     {status: fasthttp.StatusTemporaryRedirect, kept: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var hops [][]string
			followRecording(t, "http://example.com/start", tc.status, "http://example.com/after",
				func(req *fasthttp.Request) {
					req.Header.SetMethod(fasthttp.MethodPost)
					req.SetBodyString("payload")
					for _, field := range contentFieldNames {
						req.Header.Set(field, "x")
					}
				}, func(req *fasthttp.Request) {
					var present []string
					for _, field := range contentFieldNames {
						if len(req.Header.Peek(field)) > 0 {
							present = append(present, field)
						}
					}
					hops = append(hops, present)
				})

			require.Len(t, hops, 2)
			require.Equal(t, contentFieldNames, hops[0])
			if tc.kept {
				require.Equal(t, contentFieldNames, hops[1])
			} else {
				require.Empty(t, hops[1])
			}
		})
	}
}

// Test_Security_KeepHopByHopHeaders_Balancer pins the opt-out for a Balancer with its own policy.
func Test_Security_KeepHopByHopHeaders_Balancer(t *testing.T) {
	t.Parallel()
	for name, keep := range map[string]bool{"stripped by default": false, "kept with the opt-out": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			seen := make(chan [2]string, 1)
			_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
				seen <- [2]string{c.Get(fiber.HeaderProxyAuthorization), c.Get(fiber.HeaderKeepAlive)}
				c.Set(fiber.HeaderProxyAuthenticate, "Basic")
				c.Set(fiber.HeaderKeepAlive, "timeout=7")
				return c.SendString("ok")
			})

			policy := proxyTestAllowPrivatePolicy()
			policy.KeepHopByHopHeaders = keep
			app := fiber.New()
			app.Use(Balancer(Config{Servers: []string{addr}, SecurityPolicy: &policy}))

			req := apptest.Req(fiber.MethodGet, "/",
				apptest.WithHeader(fiber.HeaderProxyAuthorization, "Basic dXNlcjpwYXNz"),
				apptest.WithHeader(fiber.HeaderKeepAlive, "timeout=5"))
			resp := apptest.Expect(t, app, req, fiber.StatusOK)

			got := <-seen
			if keep {
				require.Equal(t, "Basic dXNlcjpwYXNz", got[0])
				require.Equal(t, "timeout=5", got[1])
				require.Equal(t, "Basic", resp.Header.Get(fiber.HeaderProxyAuthenticate))
				require.Equal(t, "timeout=7", resp.Header.Get(fiber.HeaderKeepAlive))
				return
			}
			require.Empty(t, got[0])
			require.Empty(t, got[1])
			require.Empty(t, resp.Header.Get(fiber.HeaderProxyAuthenticate))
			require.Empty(t, resp.Header.Get(fiber.HeaderKeepAlive))
		})
	}
}

// Test_Security_KeepHopByHopHeaders_Do is the same opt-out for the helpers, via the process-wide policy.
func Test_Security_KeepHopByHopHeaders_Do(t *testing.T) {
	// Intentionally not parallel: it sets a process-wide security policy.
	policy := proxyTestAllowPrivatePolicy()
	policy.KeepHopByHopHeaders = true
	withSecurityPolicyForTest(t, policy)

	seen := make(chan string, 1)
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		seen <- c.Get(fiber.HeaderProxyAuthorization)
		c.Set(fiber.HeaderProxyAuthenticate, "Basic")
		return c.SendString("ok")
	})

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error { return Do(c, "http://"+addr) })

	req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderProxyAuthorization, "Basic dXNlcjpwYXNz"))
	resp := apptest.Expect(t, app, req, fiber.StatusOK)
	require.Equal(t, "Basic dXNlcjpwYXNz", <-seen)
	require.Equal(t, "Basic", resp.Header.Get(fiber.HeaderProxyAuthenticate))
}

// Test_Security_ValidateUpstream_UnparsableIsHostInvalid pins that an address net/url cannot
// parse is ErrUpstreamHostInvalid, with the *url.Error still reachable.
func Test_Security_ValidateUpstream_UnparsableIsHostInvalid(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"http://[::1", "http://exam ple.com", "http://%zz.com/", "http://a:b:c/"} {
		_, err := validateUpstream(raw, DefaultSecurityPolicy(), nil)
		require.ErrorIs(t, err, ErrUpstreamHostInvalid, raw)
		_, ok := errors.AsType[*url.Error](err)
		require.True(t, ok, "the net/url error must stay reachable: %q", raw)
	}

	// "" and "http:///path" are pinned by RejectsEmpty and EmptyHostAfterScheme.
	for _, raw := range []string{"   ", "http://", "http://:8080"} {
		_, err := validateUpstream(raw, DefaultSecurityPolicy(), nil)
		require.ErrorIs(t, err, ErrUpstreamHostInvalid, raw)
	}
}

// Test_Security_ValidateUpstreamForBalancer_Schemes pins the scheme of each accepted Balancer server
// form; Test_Proxy_Balancer_RejectsNonOriginServer pins what is refused.
func Test_Security_ValidateUpstreamForBalancer_Schemes(t *testing.T) {
	t.Parallel()
	policy := proxyTestAllowPrivatePolicy()

	for raw, wantScheme := range map[string]string{
		"203.0.113.5:8080":          schemeHTTP,
		"http://203.0.113.5:8080":   schemeHTTP,
		"http://203.0.113.5:8080/":  schemeHTTP,
		"https://upstream.example":  schemeHTTPS,
		"http://[2001:db8::1]:8080": schemeHTTP,
	} {
		u, err := validateUpstreamForBalancer(raw, policy)
		require.NoError(t, err, raw)
		require.Equal(t, wantScheme, u.Scheme, raw)
	}
}
