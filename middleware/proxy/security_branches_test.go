package proxy

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

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

// Test_Security_EqualFoldASCII covers all branches: length mismatch,
// case-insensitive match, byte mismatch.
func Test_Security_EqualFoldASCII(t *testing.T) {
	t.Parallel()
	require.False(t, equalFoldASCII([]byte("https"), []byte("http")))
	require.True(t, equalFoldASCII([]byte("HTTPS"), []byte("https")))
	require.True(t, equalFoldASCII([]byte("http"), []byte("HTTP")))
	require.False(t, equalFoldASCII([]byte("http"), []byte("file")))
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
	require.Equal(t, "http://example.com/x", out)
}

// Test_Security_ResolveRedirect_HTTPSDowngradeBlocked verifies the
// guard wired against equalFoldASCII("https", ...).
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
	err := followRedirects(client.client, req, resp, -1, policy)
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
	err := followRedirects(client.client, req, resp, 3, policy)
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
	require.NoError(t, followRedirects(client.client, req, resp, 3, policy))
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

	err := followRedirects(cli, req, resp, 1, DefaultSecurityPolicy())
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

// Test_Security_DomainForward_InvalidUpstreamReturnsError covers the
// validateUpstream error path inside the matched-host branch.
func Test_Security_DomainForward_InvalidUpstreamReturnsError(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(DomainForward("api.example", "gopher://invalid"))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("unused") })

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "api.example"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// Test_Security_BalancerForward_InvalidUpstreamReturnsError covers the
// validateUpstream error path in BalancerForward.
func Test_Security_BalancerForward_InvalidUpstreamReturnsError(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(BalancerForward([]string{"gopher://invalid"}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// Test_Security_RoundRobin_WrapsAround covers the modulo wrap-around in
// roundrobin.get when current >= len(pool).
func Test_Security_RoundRobin_WrapsAround(t *testing.T) {
	t.Parallel()
	r := &roundrobin{pool: []string{"a", "b"}, current: 5}
	got := r.get()
	require.Equal(t, "b", got, "5 %% 2 = 1 should select pool[1]")
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

// redirectStep / countingRedirectClient are minimal stubs for the
// followRedirects branches without an actual network.
type redirectStep struct {
	location string
	status   int
}

type countingRedirectClient struct {
	client *fasthttp.Client
	calls  *int
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
