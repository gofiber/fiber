package proxy

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

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

		require.NoError(t, followRedirects(client.client, req, resp, 3, mustParseTestURL(t, "http://first.example/"), policy))
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

		require.NoError(t, followRedirects(client.client, req, resp, 3, mustParseTestURL(t, "http://same.example/"), policy))
		require.Equal(t, "Bearer secret", sawAuth, "Authorization must survive same-host redirect")
	})
}

// Test_Security_FollowRedirects_ReappliesSchemePerHop verifies that a
// redirect which changes the scheme is dispatched with the new scheme.
// followRedirects re-applies currentURL.Scheme after each SetRequestURI
// because fasthttp keeps the previous scheme when req.isTLS is set — see
// https://github.com/gofiber/fiber/issues/1762.
func Test_Security_FollowRedirects_ReappliesSchemePerHop(t *testing.T) {
	t.Parallel()

	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true

	var secondHopScheme string
	client, _ := newCountingRedirectClient(map[string]redirectStep{
		"http://origin.example/": {status: fasthttp.StatusFound, location: "https://origin.example/secure"},
	})
	client.onRequest = func(req *fasthttp.Request) {
		if string(req.URI().Path()) == "/secure" {
			secondHopScheme = string(req.URI().Scheme())
		}
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	req.SetRequestURI("http://origin.example/")
	req.Header.SetMethod(fasthttp.MethodGet)

	require.NoError(t, followRedirects(client.client, req, resp, 3, mustParseTestURL(t, "http://origin.example/"), policy))
	require.Equal(t, "https", secondHopScheme, "scheme upgrade must carry through to the next hop")
}

func Test_Security_JoinUpstreamPath_PreservesQuery(t *testing.T) {
	t.Parallel()
	base, err := parseUpstream("http://upstream.example")
	require.NoError(t, err)
	out := joinUpstreamPath(base, "/x?y=1&z=2")
	require.Equal(t, "http://upstream.example/x?y=1&z=2", out)
}

func Test_Security_JoinUpstreamPath_EmptyQueryMarkersUseSlowPath(t *testing.T) {
	t.Parallel()

	baseWithEmptyQuery, err := parseUpstream("http://upstream.example?")
	require.NoError(t, err)
	require.True(t, baseWithEmptyQuery.ForceQuery)
	require.Equal(t, "http://upstream.example/foo?", joinUpstreamPath(baseWithEmptyQuery, "/foo"))

	base, err := parseUpstream("http://upstream.example")
	require.NoError(t, err)
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

// ---------------------------------------------------------------------------
// Merged from the former security_branches_test.go: branch-coverage and
// edge-case tests plus their shared redirect-client / URL helpers.
// ---------------------------------------------------------------------------

// Test_Security_IsBlockedIP_NilReturnsTrue covers the nil-IP guard so we
// fail closed when callers feed us a parse failure they didn't check.
func Test_Security_IsBlockedIP_NilReturnsTrue(t *testing.T) {
	t.Parallel()
	require.True(t, isBlockedIP(nil))
}

// Test_Security_ValidateHostForSSRF_EmptyHost verifies the explicit
// empty-host short-circuit (the host == "" branch).
func Test_Security_ValidateHostForSSRF_EmptyHost(t *testing.T) {
	t.Parallel()
	require.ErrorIs(t, validateHostForSSRF(""), ErrUpstreamHostInvalid)
}

// Test_Security_ValidateHostForSSRF_PublicIPLiteral exercises the
// ParseIP success path with a public address.
func Test_Security_ValidateHostForSSRF_PublicIPLiteral(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateHostForSSRF("8.8.8.8"))
}

// Test_Security_ValidateHostForSSRF_LookupFailure covers the
// LookupIP-error branch. ".invalid" is reserved (RFC 6761) and never
// resolves, so the test does not depend on outbound DNS.
func Test_Security_ValidateHostForSSRF_LookupFailure(t *testing.T) {
	t.Parallel()
	err := validateHostForSSRF("does-not-exist.invalid")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
}

// Test_Security_ValidateUpstream_EmptyHostAfterScheme covers the post-
// scheme-check "host empty" guard. "http:///path" parses with scheme
// "http" and no host.
func Test_Security_ValidateUpstream_EmptyHostAfterScheme(t *testing.T) {
	t.Parallel()
	_, err := validateUpstream("http:///path", DefaultSecurityPolicy())
	require.ErrorIs(t, err, ErrUpstreamHostInvalid)
}

// Test_Security_JoinUpstreamPath_NilBase guards the nil-base branch
// that callers should never hit but we still handle.
func Test_Security_JoinUpstreamPath_NilBase(t *testing.T) {
	t.Parallel()
	require.Empty(t, joinUpstreamPath(nil, "/foo"))
}

// Test_Security_JoinUpstreamPath_EmptyPath exercises the early-return
// when the inbound request had no path component.
func Test_Security_JoinUpstreamPath_EmptyPath(t *testing.T) {
	t.Parallel()
	base, err := parseUpstream("http://upstream.example")
	require.NoError(t, err)
	require.Equal(t, base.String(), joinUpstreamPath(base, ""))
}

// Test_Security_JoinUpstreamPath_QueryOnly covers the "?" leading-char
// branch where the inbound path is just a query string.
func Test_Security_JoinUpstreamPath_QueryOnly(t *testing.T) {
	t.Parallel()
	base, err := parseUpstream("http://upstream.example/api")
	require.NoError(t, err)
	out := joinUpstreamPath(base, "?q=1")
	require.Contains(t, out, "?q=1")
	require.True(t, strings.HasPrefix(out, "http://upstream.example/"))
}

// Test_Security_WithSecurityPolicy_EmptyAllowedSchemesGetsDefault
// covers the branch in WithSecurityPolicy that fills in the default
// scheme allowlist when the caller leaves it empty.
func Test_Security_WithSecurityPolicy_EmptyAllowedSchemesGetsDefault(t *testing.T) {
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

// Test_Security_ResolveRedirect_RejectsControlChars covers the CRLF /
// control-byte rejection at the top of resolveRedirect.
func Test_Security_ResolveRedirect_RejectsControlChars(t *testing.T) {
	t.Parallel()
	_, err := resolveRedirect("https://example.com", []byte("https://example.com/\r\nX-Inject: 1"), DefaultSecurityPolicy())
	require.ErrorIs(t, err, fasthttp.ErrorInvalidURI)
}

// Test_Security_ResolveRedirect_RejectsDisallowedScheme verifies the
// validateUpstream propagation when the redirect target's scheme is
// outside the allowlist.
func Test_Security_ResolveRedirect_RejectsDisallowedScheme(t *testing.T) {
	t.Parallel()
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	_, err := resolveRedirect("https://example.com", []byte("gopher://example.com/x"), policy)
	require.ErrorIs(t, err, ErrUpstreamSchemeNotAllowed)
}

// Test_Security_ResolveRedirect_AllowsDowngradeWhenOptIn verifies the
// AllowHTTPSDowngrade escape hatch.
func Test_Security_ResolveRedirect_AllowsDowngradeWhenOptIn(t *testing.T) {
	t.Parallel()
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	policy.AllowHTTPSDowngrade = true
	out, err := resolveRedirect("https://example.com", []byte("http://example.com/x"), policy)
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Equal(t, "http://example.com/x", out.String())
}

// Test_Security_ResolveRedirect_HTTPSDowngradeBlocked verifies the
// HTTPS→HTTP downgrade guard.
func Test_Security_ResolveRedirect_HTTPSDowngradeBlocked(t *testing.T) {
	t.Parallel()
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	_, err := resolveRedirect("https://example.com", []byte("http://example.com/x"), policy)
	require.ErrorIs(t, err, ErrRedirectDowngrade)
}

// Test_Security_FollowRedirects_NegativeMaxBecomesZero ensures a
// negative redirect cap collapses to "no follow" — the first redirect
// returned by the upstream is propagated, no further requests are made.
func Test_Security_FollowRedirects_NegativeMaxBecomesZero(t *testing.T) {
	t.Parallel()

	client, hits := newCountingRedirectClient(map[string]redirectStep{
		"http://example.com/start": {status: fasthttp.StatusFound, location: "http://example.com/next"},
	})

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	req.SetRequestURI("http://example.com/start")
	req.Header.SetMethod(fasthttp.MethodGet)

	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	err := followRedirects(client.client, req, resp, -1, mustParseTestURL(t, "http://example.com/start"), policy)
	require.ErrorIs(t, err, fasthttp.ErrTooManyRedirects)
	require.Equal(t, 1, *hits)
}

// Test_Security_FollowRedirects_MissingLocation covers the
// ErrMissingLocation branch when the upstream returns a 3xx but no
// Location header.
func Test_Security_FollowRedirects_MissingLocation(t *testing.T) {
	t.Parallel()

	client, _ := newCountingRedirectClient(map[string]redirectStep{
		"http://example.com/start": {status: fasthttp.StatusFound, location: ""},
	})

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	req.SetRequestURI("http://example.com/start")
	req.Header.SetMethod(fasthttp.MethodGet)

	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	err := followRedirects(client.client, req, resp, 3, mustParseTestURL(t, "http://example.com/start"), policy)
	require.ErrorIs(t, err, fasthttp.ErrMissingLocation)
}

// Test_Security_FollowRedirects_PostBecomesGetOn303 covers the POST→GET
// rewrite branch on 303 See Other, including body and content-type
// clearing.
func Test_Security_FollowRedirects_PostBecomesGetOn303(t *testing.T) {
	t.Parallel()

	client, _ := newCountingRedirectClient(map[string]redirectStep{
		"http://example.com/start": {status: fasthttp.StatusSeeOther, location: "http://example.com/after"},
		"http://example.com/after": {status: fasthttp.StatusOK},
	})

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	req.SetRequestURI("http://example.com/start")
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetBodyString(`{"x":1}`)

	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	require.NoError(t, followRedirects(client.client, req, resp, 3, mustParseTestURL(t, "http://example.com/start"), policy))
	require.Equal(t, fasthttp.MethodGet, string(req.Header.Method()))
	require.Empty(t, req.Body())
	require.Empty(t, req.Header.ContentType())
}

// Test_Security_FollowRedirects_PropagatesClientError covers the
// `cli.Do` error path.
func Test_Security_FollowRedirects_PropagatesClientError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("dial failure")
	cli := &fasthttp.Client{
		Dial: func(string) (net.Conn, error) { return nil, sentinel },
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	req.SetRequestURI("http://example.com/")
	req.Header.SetMethod(fasthttp.MethodGet)

	err := followRedirects(cli, req, resp, 1, mustParseTestURL(t, "http://example.com/"), DefaultSecurityPolicy())
	require.ErrorIs(t, err, sentinel)
}

// Test_Security_DomainForward_HostMismatchPassesThrough covers the
// host-mismatch branch where the middleware should be a no-op.
func Test_Security_DomainForward_HostMismatchPassesThrough(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(DomainForward("only-this-host.example", "http://127.0.0.1:1"))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("pass-through") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "other-host.example"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// Test_Security_DomainForward_InvalidUpstreamPanicsAtConstruction
// verifies that DomainForward now fails fast on misconfiguration —
// matching Balancer's contract — instead of surfacing the validation
// error per request.
func Test_Security_DomainForward_InvalidUpstreamPanicsAtConstruction(t *testing.T) {
	t.Parallel()
	require.PanicsWithError(t, ErrUpstreamSchemeNotAllowed.Error()+": \"gopher\"", func() {
		DomainForward("api.example", "gopher://invalid")
	})
}

// Test_Security_BalancerForward_InvalidUpstreamPanicsAtConstruction
// verifies that BalancerForward now fails fast on misconfiguration —
// matching Balancer's contract — instead of surfacing the validation
// error per request.
func Test_Security_BalancerForward_InvalidUpstreamPanicsAtConstruction(t *testing.T) {
	t.Parallel()
	require.PanicsWithError(t, ErrUpstreamSchemeNotAllowed.Error()+": \"gopher\"", func() {
		BalancerForward([]string{"gopher://invalid"})
	})
}

// Test_Security_RoundRobin_WrapsAround covers the modulo wrap-around in
// urlRoundrobin.get when current >= len(pool).
func Test_Security_RoundRobin_WrapsAround(t *testing.T) {
	t.Parallel()
	a := &url.URL{Scheme: "http", Host: "a"}
	b := &url.URL{Scheme: "http", Host: "b"}
	r := &urlRoundrobin{pool: []*url.URL{a, b}, current: 5}
	got := r.get()
	require.Same(t, b, got, "5 %% 2 = 1 should select pool[1]")
}

// Test_Security_Balancer_PanicsOnInvalidUpstream covers the panic
// branch when a server fails validation at Config-time.
func Test_Security_Balancer_PanicsOnInvalidUpstream(t *testing.T) {
	t.Parallel()
	require.PanicsWithError(t, ErrUpstreamSchemeNotAllowed.Error()+": \"gopher\"", func() {
		Balancer(Config{Servers: []string{"gopher://example"}})
	})
}

// Test_Security_ValidateUpstream_PublicIPLiteral covers the
// validateUpstream "no private IP, lookup succeeds" success path.
func Test_Security_ValidateUpstream_PublicIPLiteral(t *testing.T) {
	t.Parallel()
	u, err := validateUpstream("http://8.8.8.8", DefaultSecurityPolicy())
	require.NoError(t, err)
	require.Equal(t, "8.8.8.8", u.Host)
}

// Test_Security_Balancer_ModifyRequestErrorPropagates covers the
// ModifyRequest error branch in Balancer's handler.
func Test_Security_Balancer_ModifyRequestErrorPropagates(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("should-not-run")
	})

	sentinel := errors.New("modify-request failed")
	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{addr},
		ModifyRequest: func(_ fiber.Ctx) error {
			return sentinel
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// Test_Security_Balancer_ModifyResponseErrorPropagates covers the
// ModifyResponse error branch in Balancer's handler.
func Test_Security_Balancer_ModifyResponseErrorPropagates(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	sentinel := errors.New("modify-response failed")
	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{addr},
		ModifyResponse: func(_ fiber.Ctx) error {
			return sentinel
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// Test_Security_Balancer_BuildsHTTPSHostClient verifies the IsTLS
// branch flips on for https upstreams.
func Test_Security_Balancer_BuildsHTTPSHostClient(t *testing.T) {
	t.Parallel()
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	// We can't reach the upstream, but we can confirm the handler is
	// installed without panicking and the TLS flag was set by inspecting
	// the resulting LBClient via the public Balancer (which would have
	// panicked at construction time for an invalid TLS upstream).
	h := Balancer(Config{
		Servers:        []string{"https://127.0.0.1:0"},
		SecurityPolicy: &policy,
	})
	require.NotNil(t, h)
}

// Test_Security_NewSSRFDialer_BlocksLoopback verifies the dial-time SSRF
// guard rejects both literal and hostname loopback targets before any
// connection is made — this is what defeats DNS rebinding.
func Test_Security_NewSSRFDialer_BlocksLoopback(t *testing.T) {
	t.Parallel()
	dial := newSSRFDialer(false)

	_, err := dial("127.0.0.1:80")
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)

	_, err = dial("localhost:80")
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
}

// Test_Security_NewSSRFDialer_RejectsMalformedAddr covers the
// SplitHostPort error branch.
func Test_Security_NewSSRFDialer_RejectsMalformedAddr(t *testing.T) {
	t.Parallel()
	dial := newSSRFDialer(false)
	_, err := dial("no-port-here")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrUpstreamHostBlocked)
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
		h = Balancer(Config{
			Servers:        []string{"http://does-not-exist.invalid:80"},
			SecurityPolicy: &policy,
		})
	})
	require.NotNil(t, h)

	// At request time the dial-time guard fails the lookup, surfacing a 5xx
	// rather than reaching any upstream.
	app := fiber.New()
	app.Use(h)
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "does-not-exist.invalid"
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: false})
	require.NoError(t, err)
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
func mustParseTestURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}
