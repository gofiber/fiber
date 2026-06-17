package proxy

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/tlstest"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// withSecurityPolicyForTest installs policy for the duration of t and
// restores the previous policy via t.Cleanup. Tests that need to
// exercise the strict defaults call this with an explicit policy.
func withSecurityPolicyForTest(t *testing.T, policy SecurityPolicy) {
	t.Helper()
	prev := WithSecurityPolicy(policy)
	t.Cleanup(func() { WithSecurityPolicy(prev) })
}

func Test_Security_IsBlockedIP(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"127.0.0.1":       true,
		"::1":             true,
		"0.0.0.0":         true,
		"10.0.0.1":        true,
		"172.16.0.1":      true,
		"192.168.1.1":     true,
		"169.254.169.254": true, // AWS metadata
		"100.64.0.1":      true, // CGNAT
		"224.0.0.1":       true, // multicast
		"8.8.8.8":         false,
		"1.1.1.1":         false,
		"93.184.216.34":   false, // example.com
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

func Test_Security_ValidateUpstream_BlocksPrivateIPs(t *testing.T) {
	t.Parallel()
	policy := DefaultSecurityPolicy()
	bad := []string{
		"http://127.0.0.1",
		"http://localhost:8080",
		"http://10.0.0.1",
		"http://192.168.1.1/path",
		"http://169.254.169.254/latest/meta-data/",
		"https://[::1]/",
	}
	for _, raw := range bad {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			_, err := validateUpstream(raw, policy)
			require.ErrorIs(t, err, ErrUpstreamHostBlocked, "expected block for %q", raw)
		})
	}
}

func Test_Security_ValidateUpstream_AllowsPrivateOptIn(t *testing.T) {
	t.Parallel()
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	u, err := validateUpstream("http://127.0.0.1:8080", policy)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:8080", u.Host)
}

func Test_Security_ValidateUpstream_RejectsSchemes(t *testing.T) {
	t.Parallel()
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	bad := []string{"file:///etc/passwd", "gopher://example.com", "ftp://example.com"}
	for _, raw := range bad {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			_, err := validateUpstream(raw, policy)
			require.ErrorIs(t, err, ErrUpstreamSchemeNotAllowed)
		})
	}
}

func Test_Security_ValidateUpstream_RejectsEmpty(t *testing.T) {
	t.Parallel()
	_, err := validateUpstream("", DefaultSecurityPolicy())
	require.ErrorIs(t, err, ErrUpstreamHostInvalid)
}

func Test_Security_Forward_BlocksPrivateByDefault(t *testing.T) {
	withSecurityPolicyForTest(t, DefaultSecurityPolicy())

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("should not be reached")
	})

	app := fiber.New()
	app.Use(Forward("http://" + addr))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func Test_Security_Balancer_BlocksPrivateByDefault(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	// Build a policy that blocks private IPs and feed it via Config so
	// the panic surfaces at Balancer construction time.
	policy := DefaultSecurityPolicy()
	require.PanicsWithError(t, ErrUpstreamHostBlocked.Error()+": 127.0.0.1", func() {
		Balancer(Config{
			Servers:        []string{addr},
			SecurityPolicy: &policy,
		})
	})
}

func Test_Security_Balancer_AllowsPrivateOptIn(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	app := fiber.New()
	app.Use(Balancer(Config{
		Servers:        []string{addr},
		SecurityPolicy: &policy,
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = addr
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
}

// Test_Security_HopByHopRequestStripping verifies hop-by-hop headers
// (RFC 7230 §6.1) are dropped from the outbound request.
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

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = addr
	for _, h := range hops {
		req.Header.Set(h, "drop-me")
	}
	req.Header.Set(fiber.HeaderConnection, "X-Custom-Hop")
	req.Header.Set("X-Custom-Hop", "should-be-removed")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
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

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = addr
	resp, err := app.Test(req)
	require.NoError(t, err)
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
	app.Use(Balancer(Config{
		Servers:              []string{addr},
		KeepConnectionHeader: true,
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = addr
	req.Header.Set(fiber.HeaderConnection, "keep-alive")
	req.Header.Set(fiber.HeaderProxyAuthorization, "Basic dXNlcjpwYXNz")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_Security_Do_BlocksFileScheme(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		return Do(c, "file:///etc/passwd")
	})
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func Test_Security_DoRedirects_BlocksDowngrade(t *testing.T) {
	// No t.Parallel: this test mutates the global SecurityPolicy and would
	// otherwise race with other proxy tests that read it at runtime.
	// Allow private IPs for the loopback test servers while keeping the
	// downgrade protection enabled.
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	withSecurityPolicyForTest(t, policy)

	// Start an HTTPS server that 302-redirects to a plaintext HTTP URL.
	cert, err := generateLocalhostCert(t)
	require.NoError(t, err)

	httpsLn, err := tls.Listen(fiber.NetworkTCP4, "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	require.NoError(t, err)
	t.Cleanup(func() { httpsLn.Close() }) //nolint:errcheck // best effort

	plaintextLn, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { plaintextLn.Close() }) //nolint:errcheck // best effort

	plaintextAddr := plaintextLn.Addr().String()

	httpsApp := fiber.New()
	httpsApp.Get("/", func(c fiber.Ctx) error {
		c.Location("http://" + plaintextAddr + "/secret")
		return c.SendStatus(fiber.StatusFound)
	})
	startServer(httpsApp, httpsLn)

	plainApp := fiber.New()
	plainApp.Get("/secret", func(c fiber.Ctx) error {
		return c.SendString("LEAKED")
	})
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "HTTPS to HTTP redirect blocked")
}

func Test_Security_JoinUpstreamPath_BlocksNetworkPathInjection(t *testing.T) {
	t.Parallel()
	base, err := parseUpstream("http://upstream.example")
	require.NoError(t, err)

	out := joinUpstreamPath(base, "//attacker.example/path")
	require.True(t, strings.HasPrefix(out, "http://upstream.example/"), "host must not change: %q", out)
	require.NotContains(t, out, "//attacker.example/")
}

// Test_Security_JoinUpstreamPath_PreservesBasePathPrefix ensures a path
// prefix configured on the upstream base survives request joining instead
// of being overwritten by the request path.
func Test_Security_JoinUpstreamPath_PreservesBasePathPrefix(t *testing.T) {
	t.Parallel()
	base, err := parseUpstream("http://upstream.example/api")
	require.NoError(t, err)

	require.Equal(t, "http://upstream.example/api/foo", joinUpstreamPath(base, "/foo"))
	require.Equal(t, "http://upstream.example/api/foo?q=1", joinUpstreamPath(base, "/foo?q=1"))

	// Trailing slash on the base should not double up.
	baseSlash, err := parseUpstream("http://upstream.example/api/")
	require.NoError(t, err)
	require.Equal(t, "http://upstream.example/api/foo", joinUpstreamPath(baseSlash, "/foo"))
}

// Test_Security_FollowRedirects_StripsCredentialsCrossHost verifies that
// Authorization/Cookie/Proxy-Authorization are dropped when a redirect
// crosses to a different host, but preserved on same-host redirects.
func Test_Security_FollowRedirects_StripsCredentialsCrossHost(t *testing.T) {
	t.Parallel()

	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true

	t.Run("cross-host strips", func(t *testing.T) {
		t.Parallel()
		var sawAuth, sawCookie string
		client, _ := newCountingRedirectClient(map[string]redirectStep{
			"http://first.example/": {status: fasthttp.StatusFound, location: "http://second.example/next"},
		})
		client.onRequest = func(req *fasthttp.Request) {
			if string(req.URI().Host()) == "second.example" {
				sawAuth = string(req.Header.Peek(fiber.HeaderAuthorization))
				sawCookie = string(req.Header.Peek(fiber.HeaderCookie))
			}
		}

		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseRequest(req)
		defer fasthttp.ReleaseResponse(resp)
		req.SetRequestURI("http://first.example/")
		req.Header.SetMethod(fasthttp.MethodGet)
		req.Header.Set(fiber.HeaderAuthorization, "Bearer secret")
		req.Header.Set(fiber.HeaderCookie, "session=abc")

		require.NoError(t, followRedirects(client.client, req, resp, 3, policy))
		require.Empty(t, sawAuth, "Authorization must be stripped cross-host")
		require.Empty(t, sawCookie, "Cookie must be stripped cross-host")
	})

	t.Run("same-host preserves", func(t *testing.T) {
		t.Parallel()
		var sawAuth string
		client, _ := newCountingRedirectClient(map[string]redirectStep{
			"http://same.example/": {status: fasthttp.StatusFound, location: "http://same.example/next"},
		})
		client.onRequest = func(req *fasthttp.Request) {
			if string(req.URI().RequestURI()) == "/next" {
				sawAuth = string(req.Header.Peek(fiber.HeaderAuthorization))
			}
		}

		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseRequest(req)
		defer fasthttp.ReleaseResponse(resp)
		req.SetRequestURI("http://same.example/")
		req.Header.SetMethod(fasthttp.MethodGet)
		req.Header.Set(fiber.HeaderAuthorization, "Bearer secret")

		require.NoError(t, followRedirects(client.client, req, resp, 3, policy))
		require.Equal(t, "Bearer secret", sawAuth, "Authorization must survive same-host redirect")
	})
}

func Test_Security_JoinUpstreamPath_PreservesQuery(t *testing.T) {
	t.Parallel()
	base, err := parseUpstream("http://upstream.example")
	require.NoError(t, err)
	out := joinUpstreamPath(base, "/x?y=1&z=2")
	require.Equal(t, "http://upstream.example/x?y=1&z=2", out)
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
	custom := DefaultSecurityPolicy()
	custom.AllowPrivateIPs = true
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
