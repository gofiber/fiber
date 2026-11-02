package proxy

import (
	"errors"
	"net"
	"net/url"
	"testing"
	"time"

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

// Test_Coverage_ResolveAndValidateHost covers the answers for an IP literal and, through a reserved
// RFC 6761 TLD that never resolves and so never depends on outbound DNS, the LookupIPAddr error wrapping.
func Test_Coverage_ResolveAndValidateHost(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		name    string
		host    string
	}{
		{name: "public IP literal", host: "8.8.8.8"},
		{name: "blocked IP literal", host: "127.0.0.1", wantErr: ErrUpstreamHostBlocked},
		{name: "lookup fails", host: "definitely-not-a-real-host.invalid", wantErr: ErrUpstreamHostBlocked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ips, err := resolveAndValidateHost(tc.host, time.Time{})
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, ips, 1)
			require.True(t, ips[0].Equal(net.ParseIP(tc.host)))
		})
	}
}

// Test_Coverage_DialValidatedIPs covers the candidate loop: an IPv6 candidate is skipped before any
// dial unless DialDualStack is on, in which case the error does not match ErrUpstreamHostBlocked, since
// nothing was blocked; a dial that fails goes on to the next candidate and the last error is returned.
func Test_Coverage_DialValidatedIPs(t *testing.T) {
	t.Parallel()
	refused := errors.New("dial-refused")
	for _, tc := range []struct {
		dialErr     error // what every dial answers, when it fails
		wantErr     error
		name        string
		port        string
		wantErrText string
		ips         []string
		wantDialed  []string // the addresses dialed, in order
		dualStack   bool
	}{
		{name: "IPv4", ips: []string{"203.0.113.1"}, port: "80", wantDialed: []string{"203.0.113.1:80"}},
		{
			name: "IPv6 is skipped without dual stack", ips: []string{"2606:4700:4700::1111"}, port: "80",
			wantErr: errIPv6Only, wantErrText: "example.test has only IPv6 addresses, and DialDualStack is off",
		},
		{
			name: "falls back on error", ips: []string{"203.0.113.1", "203.0.113.2"}, port: "80", dialErr: refused,
			wantErr: refused, wantDialed: []string{"203.0.113.1:80", "203.0.113.2:80"},
		},
		{
			name: "dual stack tries IPv6", dualStack: true, ips: []string{"2606:4700:4700::1111"}, port: "443",
			wantDialed: []string{"[2606:4700:4700::1111]:443"},
		},
		{
			name: "IPv6 is skipped, then IPv4 dials", ips: []string{"2001:db8::1", "203.0.113.5"}, port: "443",
			wantDialed: []string{"203.0.113.5:443"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var dialed []string
			guard := dialGuard{dualStack: tc.dualStack, dial: func(network, addr string) (net.Conn, error) {
				require.Equal(t, "tcp", network)
				dialed = append(dialed, addr)
				if tc.dialErr != nil {
					return nil, tc.dialErr
				}
				return stubConn{addr: addr}, nil
			}}
			ips := make([]net.IP, len(tc.ips))
			for i, raw := range tc.ips {
				ips[i] = net.ParseIP(raw)
			}

			conn, err := guard.dialValidatedIPs(ips, "example.test", tc.port)
			require.Equal(t, tc.wantDialed, dialed, "the addresses dialed, an IPv6 one being skipped")
			if tc.wantErr == nil {
				require.NoError(t, err)
				require.NotNil(t, conn)
				return
			}
			require.ErrorIs(t, err, tc.wantErr)
			require.NotErrorIs(t, err, ErrUpstreamHostBlocked)
			require.NotErrorIs(t, err, ErrUpstreamHostUnresolvable)
			if tc.wantErrText != "" {
				require.ErrorContains(t, err, tc.wantErrText)
			}
		})
	}
}

// Test_Coverage_DialChecked_PassesGuardThenDials covers the post-guard
// dial dispatch (the final return inside dialGuard.dialChecked). TEST-NET-1
// (192.0.2.0/24, RFC 5737) is reserved for documentation and not in the
// blocked ranges, so it survives the SSRF check; the dial itself fails
// fast because the kernel has no route. We only assert that an error
// surfaced — the *type* of error depends on the host's networking stack.
func Test_Coverage_DialChecked_PassesGuardThenDials(t *testing.T) {
	t.Parallel()
	dial := newDialGuard(true).dialChecked // dualStack so the dial loop is reached
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
	err := validateHostForSSRF("one.one.one.one", nil)
	if _, ok := errors.AsType[*net.DNSError](err); ok {
		t.Skipf("offline DNS, skipping: %v", err)
	}
	require.NoError(t, err)
}

// Test_Coverage_ConfigDefault_FillsDefaults checks the defaults for a Config with Servers or a Client.
func Test_Coverage_ConfigDefault_FillsDefaults(t *testing.T) {
	t.Parallel()
	for name, in := range map[string]Config{
		"servers": {Servers: []string{"http://upstream.example"}},
		"client":  {Client: &fasthttp.LBClient{}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg := configDefault(in)
			require.Equal(t, ConfigDefault.Timeout, cfg.Timeout)
			require.Equal(t, ConfigDefault.MaxConnsPerHost, cfg.MaxConnsPerHost)
		})
	}
}

// Test_Coverage_JoinUpstreamPath_RawPathFallback exercises the RawPath join
// when the base has a RawPath but the request path does not.
func Test_Coverage_JoinUpstreamPath_RawPathFallback(t *testing.T) {
	t.Parallel()
	base, err := url.Parse("http://upstream.example/a%2Fb")
	require.NoError(t, err)
	require.NotEmpty(t, base.RawPath, "test setup: base must have a RawPath")
	out := joinUpstreamPath(base, "/plain")
	require.Contains(t, out, "/a%2Fb/plain")
}

// Test_Coverage_EscapedPath pins that escapedPath returns what URL.EscapedPath does, without its allocation,
// and that parseUpstream drops a RawPath net/url would not write.
func Test_Coverage_EscapedPath(t *testing.T) {
	// Intentionally not parallel: AllocsPerRun counts allocations process-wide and panics in a parallel test.
	for _, path := range []string{"", "/a", "/my%20app", "/a%3Bb", "/a%3Bb!x:y@z;k=v", "/a%3B[b]", "/a%2F/b", "/a%3B//b", "/café", "/a%3Bb c", "/a%3B\"b"} {
		u := mustParseTestURL(t, "http://upstream.example"+path)
		require.Equal(t, u.EscapedPath(), escapedPath(u), path)
	}
	for _, path := range []string{"/a%3Bb", "/a%3Bb!x:y@z;k=v", "/api/a%3Bb/widgets"} {
		u := mustParseTestURL(t, "http://upstream.example"+path)
		require.NotEmpty(t, u.RawPath, path)
		require.Zero(t, testing.AllocsPerRun(100, func() { _ = escapedPath(u) }), path)
	}

	u, err := parseUpstream("http://upstream.example/café")
	require.NoError(t, err)
	require.Empty(t, u.RawPath)
	require.Equal(t, mustParseTestURL(t, "http://upstream.example/café").String(), u.String())
}

// Test_Coverage_CanFastJoinPath_RepeatedSeparator covers a second '?' or '#' left to the url.Parse path.
func Test_Coverage_CanFastJoinPath_RepeatedSeparator(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"/a?b?c", "/a?b#c", "/a#b?c", "/a#b#c"} {
		require.False(t, canFastJoinPath(target), target)
	}
	require.True(t, canFastJoinPath("/a?b"))

	base := mustParseTestURL(t, "http://upstream.example")
	require.Equal(t, "http://upstream.example/a?b?c", joinUpstreamPath(base, "/a?b?c"))
	require.Equal(t, "http://upstream.example/a?b#c", joinUpstreamPath(base, "/a?b#c"))
}
