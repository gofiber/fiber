package proxy

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// stubConn satisfies net.Conn so dialValidatedIPs can return a non-nil
// successful dial without opening real sockets.
type stubConn struct{ addr string }

func (stubConn) Read([]byte) (int, error)         { return 0, nil }
func (stubConn) Write(b []byte) (int, error)      { return len(b), nil }
func (stubConn) Close() error                     { return nil }
func (stubConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (stubConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (stubConn) SetDeadline(time.Time) error      { return nil }
func (stubConn) SetReadDeadline(time.Time) error  { return nil }
func (stubConn) SetWriteDeadline(time.Time) error { return nil }

// Test_Coverage_ResolveAndValidateHost_IPLiteralOK exercises the IP-literal
// fast path that skips DNS entirely.
func Test_Coverage_ResolveAndValidateHost_IPLiteralOK(t *testing.T) {
	t.Parallel()
	ips, err := resolveAndValidateHost("8.8.8.8")
	require.NoError(t, err)
	require.Len(t, ips, 1)
	require.True(t, ips[0].Equal(net.ParseIP("8.8.8.8")))
}

// Test_Coverage_ResolveAndValidateHost_BlocksIPLiteral covers the
// post-resolution loop that rejects a blocked IP literal.
func Test_Coverage_ResolveAndValidateHost_BlocksIPLiteral(t *testing.T) {
	t.Parallel()
	_, err := resolveAndValidateHost("127.0.0.1")
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
}

// Test_Coverage_ResolveAndValidateHost_LookupFails covers the
// LookupIPAddr error wrapping branch using a reserved RFC 6761 TLD that
// never resolves and therefore never depends on outbound DNS.
func Test_Coverage_ResolveAndValidateHost_LookupFails(t *testing.T) {
	t.Parallel()
	_, err := resolveAndValidateHost("definitely-not-a-real-host.invalid")
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
}

// Test_Coverage_DialValidatedIPs_SuccessOnIPv4 exercises the happy path
// where dialValidatedIPs returns the first successful connection.
func Test_Coverage_DialValidatedIPs_SuccessOnIPv4(t *testing.T) {
	t.Parallel()
	called := atomic.Bool{}
	conn, err := dialValidatedIPs(
		[]net.IP{net.ParseIP("203.0.113.1")},
		"example.test", "80", false,
		func(network, addr string) (net.Conn, error) {
			called.Store(true)
			require.Equal(t, "tcp", network)
			require.Equal(t, "203.0.113.1:80", addr)
			return stubConn{addr: addr}, nil
		},
	)
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.True(t, called.Load())
}

// Test_Coverage_DialValidatedIPs_SkipsV6_WhenNoDualStack covers the
// IPv4-only branch: an IPv6 candidate is skipped before any dial is
// attempted, leaving the loop with no candidates and producing the
// "no usable address" sentinel error.
func Test_Coverage_DialValidatedIPs_SkipsV6_WhenNoDualStack(t *testing.T) {
	t.Parallel()
	called := atomic.Bool{}
	_, err := dialValidatedIPs(
		[]net.IP{net.ParseIP("2606:4700:4700::1111")},
		"example.test", "80", false,
		func(string, string) (net.Conn, error) {
			called.Store(true)
			return nil, errors.New("should not dial")
		},
	)
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
	require.Contains(t, err.Error(), "no usable address")
	require.False(t, called.Load(), "IPv6 candidate must be skipped without dialing")
}

// Test_Coverage_DialValidatedIPs_FallsBackOnError exercises the
// "remember last error, continue to next IP" branch and the final
// "return lastErr" path.
func Test_Coverage_DialValidatedIPs_FallsBackOnError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("dial-refused")
	var attempts atomic.Int32
	_, err := dialValidatedIPs(
		[]net.IP{net.ParseIP("203.0.113.1"), net.ParseIP("203.0.113.2")},
		"example.test", "80", false,
		func(string, string) (net.Conn, error) {
			attempts.Add(1)
			return nil, sentinel
		},
	)
	require.ErrorIs(t, err, sentinel)
	require.Equal(t, int32(2), attempts.Load())
}

// Test_Coverage_DialValidatedIPs_DualStackTriesV6 verifies that when
// DialDualStack is enabled the IPv6 candidate is dialed instead of
// skipped.
func Test_Coverage_DialValidatedIPs_DualStackTriesV6(t *testing.T) {
	t.Parallel()
	var addr string
	_, err := dialValidatedIPs(
		[]net.IP{net.ParseIP("2606:4700:4700::1111")},
		"example.test", "443", true,
		func(_, a string) (net.Conn, error) {
			addr = a
			return stubConn{addr: a}, nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, "[2606:4700:4700::1111]:443", addr)
}

// Test_Coverage_NewSSRFDialer_RejectsLoopbackHostname drives the
// SplitHostPort → resolveAndValidateHost → blocked-IP rejection branch.
func Test_Coverage_NewSSRFDialer_RejectsLoopbackHostname(t *testing.T) {
	t.Parallel()
	dial := newSSRFDialer(false)
	_, err := dial("localhost:65535")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
}

// Test_Coverage_NewSSRFDialer_PassesGuardThenDials covers the post-guard
// dial dispatch (the final return inside newSSRFDialer). TEST-NET-1
// (192.0.2.0/24, RFC 5737) is reserved for documentation and not in the
// blocked ranges, so it survives the SSRF check; the dial itself fails
// fast because the kernel has no route. We only assert that an error
// surfaced — the *type* of error depends on the host's networking stack.
func Test_Coverage_NewSSRFDialer_PassesGuardThenDials(t *testing.T) {
	t.Parallel()
	dial := newSSRFDialer(true) // dualStack so the dial loop is reached
	_, err := dial("192.0.2.1:1")
	require.Error(t, err)
}

// Test_Coverage_ValidateHostForSSRF_PublicHostname drives the
// DNS-resolution success path of validateHostForSSRF. one.one.one.one is
// chosen because Cloudflare keeps it pointing at 1.1.1.1/1.0.0.1, both
// public. We let validateHostForSSRF do the single resolution itself
// and only skip when the failure was a DNS lookup error — using
// LookupHost as a separate skip-gate would do a redundant lookup and
// could disagree with the second lookup in restricted CI.
func Test_Coverage_ValidateHostForSSRF_PublicHostname(t *testing.T) {
	t.Parallel()
	err := validateHostForSSRF("one.one.one.one")
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		t.Skipf("offline DNS, skipping: %v", err)
	}
	require.NoError(t, err)
}

// Test_Coverage_ResolveRedirect_RejectsHostlessTarget covers the
// empty-host check after fasthttp's URI.Update merges in the Location.
// A Location of "http://" produces a URI with scheme but no host.
func Test_Coverage_ResolveRedirect_RejectsHostlessTarget(t *testing.T) {
	t.Parallel()
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	_, err := resolveRedirect("https://example.com", []byte("http://"), policy)
	require.ErrorIs(t, err, fasthttp.ErrorInvalidURI)
}

// Test_Coverage_JoinUpstreamPath_RejectsAuthorityInjection covers the
// fallback branch that fires when a parsed request path contains its
// own scheme or host. The path is treated as opaque so the upstream
// host pinned by base cannot be replaced.
func Test_Coverage_JoinUpstreamPath_RejectsAuthorityInjection(t *testing.T) {
	t.Parallel()
	base, err := url.Parse("http://upstream.example")
	require.NoError(t, err)
	out := joinUpstreamPath(base, "/foo://hijack.example/bar")
	require.NotEmpty(t, out)
	parsed, err := url.Parse(out)
	require.NoError(t, err)
	require.Equal(t, "http", parsed.Scheme)
	require.Equal(t, "upstream.example", parsed.Host, "host must remain pinned")
}

// Test_Coverage_JoinUpstreamPath_FallbackPreservesBasePathPrefix is a
// regression guard: when the request path fails to parse cleanly the
// fallback branch must still honor an upstream base path prefix.
// Without this, a malformed request like "/%zz" could silently bypass
// a configured "/api" and reach the upstream root.
func Test_Coverage_JoinUpstreamPath_FallbackPreservesBasePathPrefix(t *testing.T) {
	t.Parallel()
	base, err := url.Parse("http://upstream.example/api")
	require.NoError(t, err)

	// "/%zz" trips url.Parse (invalid escape) — the slow path's
	// parse-error fallback fires.
	out := joinUpstreamPath(base, "/%zz")
	require.NotEmpty(t, out)
	parsed, err := url.Parse(out)
	require.NoError(t, err, "fallback must still emit a parseable URL")
	require.Equal(t, "upstream.example", parsed.Host, "host must remain pinned")
	require.True(t, strings.HasPrefix(parsed.Path, "/api/"), "base path prefix must survive fallback: %q", parsed.Path)
}

// Test_Coverage_JoinUpstreamPath_PreservesRawPath_BaseHasPercentEncoded
// exercises the RawPath join path where both base and request have a
// percent-encoded segment.
func Test_Coverage_JoinUpstreamPath_PreservesRawPath_BaseHasPercentEncoded(t *testing.T) {
	t.Parallel()
	base, err := url.Parse("http://upstream.example/space%20here")
	require.NoError(t, err)
	out := joinUpstreamPath(base, "/sub")
	require.Contains(t, out, "/space%20here/sub")
}

// Test_Coverage_ConfigDefault_PanicsOnEmptyServersAndNoClient covers
// the panic branch in configDefault when neither Servers nor a
// Client override is supplied.
func Test_Coverage_ConfigDefault_PanicsOnEmptyServersAndNoClient(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "Servers cannot be empty", func() {
		configDefault(Config{})
	})
}

// Test_Coverage_ConfigDefault_NoArgsReturnsDefault covers the
// variadic-empty branch where configDefault is called with no args.
func Test_Coverage_ConfigDefault_NoArgsReturnsDefault(t *testing.T) {
	t.Parallel()
	cfg := configDefault()
	require.Equal(t, ConfigDefault.Timeout, cfg.Timeout)
	require.Equal(t, ConfigDefault.MaxConnsPerHost, cfg.MaxConnsPerHost)
}

// Test_Coverage_DialValidatedIPs_SkipsV6ThenDialsV4 verifies the
// continue branch with a follow-up successful dial: an IPv6 candidate is
// skipped (DialDualStack=false), then an IPv4 candidate dials cleanly.
func Test_Coverage_DialValidatedIPs_SkipsV6ThenDialsV4(t *testing.T) {
	t.Parallel()
	var dialed string
	conn, err := dialValidatedIPs(
		[]net.IP{net.ParseIP("2001:db8::1"), net.ParseIP("203.0.113.5")},
		"example.test", "443", false,
		func(_, addr string) (net.Conn, error) {
			dialed = addr
			return stubConn{addr: addr}, nil
		},
	)
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Equal(t, "203.0.113.5:443", dialed)
}

// Test_Coverage_JoinUpstreamPath_RawPathFallback exercises the
// `parsedRaw == "" → parsed.Path` fallback inside the RawPath join when
// the base has a RawPath but the request path does not.
func Test_Coverage_JoinUpstreamPath_RawPathFallback(t *testing.T) {
	t.Parallel()
	base, err := url.Parse("http://upstream.example/a%2Fb")
	require.NoError(t, err)
	require.NotEmpty(t, base.RawPath, "test setup: base must have a RawPath")
	out := joinUpstreamPath(base, "/plain")
	// Plain segment had no escaped chars so its RawPath is empty; the
	// fallback must use parsed.Path verbatim while preserving the base's
	// escaped prefix.
	require.Contains(t, out, "/a%2Fb/plain")
}

// Test_Coverage_DomainForward_HostMismatch_DoesNotProxy covers the
// host-mismatch branch where DomainForward returns nil without calling
// the upstream. The assertion target is that the proxy was NOT invoked
// — if it had been, the unreachable port :1 would surface as a 5xx.
func Test_Coverage_DomainForward_HostMismatch_DoesNotProxy(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(DomainForward("only-this-host.example", "http://127.0.0.1:1"))

	req := httptest.NewRequest(fiber.MethodGet, "/ping", http.NoBody)
	req.Host = "other-host.example"
	resp, err := app.Test(req)
	require.NoError(t, err)
	// Fiber returns 200 when the middleware returns nil and there is no
	// downstream handler; the important assertion is that we did NOT get
	// a 5xx from dialing 127.0.0.1:1.
	require.NotEqual(t, fiber.StatusInternalServerError, resp.StatusCode)
}
