package proxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/dns/dnsmessage"
)

// startRebindingDNS starts a loopback UDP DNS server that answers A queries
// for any name. The very first A answer is firstIP (a public-looking
// address that passes the SSRF blocklist); every A answer after that is
// rebindIP (the "internal" target). AAAA queries are answered with an empty
// NODATA response so the Go resolver falls back to the A record. This
// reproduces a classic DNS-rebinding resolver: safe on the validation
// lookup, hostile on the dial-time lookup. It returns the server's
// "host:port" address.
func startRebindingDNS(t *testing.T, firstIP, rebindIP net.IP) string {
	t.Helper()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = pc.Close() })

	var (
		mu       sync.Mutex
		answered bool
	)

	go func() {
		buf := make([]byte, 512)
		for {
			n, addr, rerr := pc.ReadFrom(buf)
			if rerr != nil {
				return // listener closed
			}

			var p dnsmessage.Parser
			hdr, perr := p.Start(buf[:n])
			if perr != nil {
				continue
			}
			q, perr := p.Question()
			if perr != nil {
				continue
			}

			var answer net.IP
			if q.Type == dnsmessage.TypeA {
				mu.Lock()
				if answered {
					answer = rebindIP
				} else {
					answer = firstIP
					answered = true
				}
				mu.Unlock()
			}

			resp, berr := buildDNSResponse(hdr.ID, q, answer)
			if berr != nil {
				continue
			}
			_, _ = pc.WriteTo(resp, addr)
		}
	}()

	return pc.LocalAddr().String()
}

// buildDNSResponse serializes a DNS reply echoing the question. When ip is
// a non-nil IPv4 address an A record is included; otherwise the reply is a
// NODATA answer (used for AAAA queries).
func buildDNSResponse(id uint16, q dnsmessage.Question, ip net.IP) ([]byte, error) {
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{
		ID:            id,
		Response:      true,
		Authoritative: true,
	})
	b.EnableCompression()
	if err := b.StartQuestions(); err != nil {
		return nil, err
	}
	if err := b.Question(q); err != nil {
		return nil, err
	}
	if err := b.StartAnswers(); err != nil {
		return nil, err
	}
	if v4 := ip.To4(); v4 != nil {
		var a [4]byte
		copy(a[:], v4)
		if err := b.AResource(dnsmessage.ResourceHeader{
			Name:  q.Name,
			Type:  dnsmessage.TypeA,
			Class: dnsmessage.ClassINET,
		}, dnsmessage.AResource{A: a}); err != nil {
			return nil, err
		}
	}
	return b.Finish()
}

// withRebindingResolver points net.DefaultResolver at the fake DNS server
// at dnsAddr for the duration of t, restoring the previous resolver via
// t.Cleanup. Tests using it must not call t.Parallel: net.DefaultResolver
// is process-global.
func withRebindingResolver(t *testing.T, dnsAddr string) {
	t.Helper()
	prev := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "udp", dnsAddr)
		},
	}
	t.Cleanup(func() { net.DefaultResolver = prev })
}

// Test_Security_Do_BlocksDNSRebinding is the regression test for the
// DNS-rebinding SSRF: the up-front validation lookup sees a public IP and
// passes, but the dial-time guard re-resolves the name, sees the rebound
// loopback address, and blocks the connection before the "internal" server
// is reached. Covered for both the shared default client and a per-call
// custom client (proving auto-guarding covers user-supplied clients).
func Test_Security_Do_BlocksDNSRebinding(t *testing.T) {
	// Not parallel: mutates the global resolver and security policy.
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false

	// dispatch names the client path under test. Each subtest gets a fresh
	// fake DNS server (fresh rebinding state) so the up-front lookup always
	// sees the public answer and the block genuinely happens at dial time —
	// not because a shared resolver already flipped to the loopback answer.
	dispatch := map[string]func(c fiber.Ctx, target string) error{
		"default client": func(c fiber.Ctx, target string) error {
			return Do(c, target)
		},
		"custom client": func(c fiber.Ctx, target string) error {
			return Do(c, target, &fasthttp.Client{})
		},
		// A client that configures DialTimeout instead of Dial must still be
		// guarded: fasthttp's callDialFunc prefers DialTimeout, so a guard
		// that wrapped only Dial would leave this path unvalidated.
		"custom client with DialTimeout": func(c fiber.Ctx, target string) error {
			cli := &fasthttp.Client{
				DialTimeout: func(addr string, timeout time.Duration) (net.Conn, error) {
					return fasthttp.DialTimeout(addr, timeout)
				},
			}
			return Do(c, target, cli)
		},
	}

	for name, do := range dispatch {
		t.Run(name, func(t *testing.T) {
			// The "internal" service the attacker wants to reach.
			_, internalAddr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
				return c.SendString("SECRET_INTERNAL_DATA")
			})
			_, internalPort, err := net.SplitHostPort(internalAddr)
			require.NoError(t, err)

			// 1st A answer is public TEST-NET-2 (passes the blocklist); every
			// A answer after that is loopback (the internal port).
			dnsAddr := startRebindingDNS(t, net.IPv4(198, 51, 100, 1), net.IPv4(127, 0, 0, 1))
			withRebindingResolver(t, dnsAddr)

			target := "http://rebind.test:" + internalPort + "/"

			app := fiber.New()
			app.Get("/", func(c fiber.Ctx) error { return do(c, target) })

			resp, err := app.Test(
				httptest.NewRequest(fiber.MethodGet, "/", http.NoBody),
				fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: false},
			)
			require.NoError(t, err)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())

			// The dial-time guard must reject the rebound loopback address,
			// so the internal secret is never proxied back.
			require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
			require.NotContains(t, string(body), "SECRET_INTERNAL_DATA")
			require.Contains(t, string(body), "blocked")
		})
	}
}

// Test_Security_NewGuardedClientDialer_BlocksWhenPrivateDisallowed verifies
// the composable client guard rejects loopback targets — literal and
// hostname — when the active policy forbids private IPs.
func Test_Security_NewGuardedClientDialer_BlocksWhenPrivateDisallowed(t *testing.T) {
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false

	dial := newGuardedClientDialer(nil, false)

	_, err := dial("127.0.0.1:80")
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)

	_, err = dial("localhost:80")
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
}

// Test_Security_NewGuardedClientDialer_ComposesWithOrig verifies that, for
// an allowed (public) target, the guard dials the validated address through
// the caller-supplied dialer instead of establishing its own connection.
func Test_Security_NewGuardedClientDialer_ComposesWithOrig(t *testing.T) {
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false

	var got string
	orig := func(addr string) (net.Conn, error) {
		got = addr
		return stubConn{addr: addr}, nil
	}

	dial := newGuardedClientDialer(orig, false)
	conn, err := dial("203.0.113.5:80") // TEST-NET-3, not blocked
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Equal(t, "203.0.113.5:80", got)
}

// Test_Security_EnsureClientGuarded_ComposesAndIsIdempotent verifies that
// installing the guard on a client that already carries its own
// ConfigureClient hook composes with it (the original still runs) and that
// repeated installation does not nest the wrapper — each HostClient gets the
// guard and the original hook exactly once.
func Test_Security_EnsureClientGuarded_ComposesAndIsIdempotent(t *testing.T) {
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false

	called := 0
	cli := &fasthttp.Client{
		ConfigureClient: func(_ *fasthttp.HostClient) error {
			called++
			return nil
		},
	}

	ensureClientGuarded(cli)
	ensureClientGuarded(cli) // must not wrap a second time

	hc := &fasthttp.HostClient{}
	require.NoError(t, cli.ConfigureClient(hc))
	require.Equal(t, 1, called, "original ConfigureClient must run exactly once per HostClient")
	require.NotNil(t, hc.Dial, "guard must install a dial-time Dial")

	// The installed Dial must block a loopback target under the strict policy.
	_, err := hc.Dial("127.0.0.1:80")
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
}

// Test_Security_InstallHostClientGuard_GuardsDialTimeout verifies both dial
// entry points are guarded: a HostClient that carries DialTimeout has it
// wrapped (fasthttp prefers DialTimeout over Dial), and the wrapped func
// blocks a loopback target under the strict policy.
func Test_Security_InstallHostClientGuard_GuardsDialTimeout(t *testing.T) {
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false

	hc := &fasthttp.HostClient{
		DialTimeout: func(addr string, timeout time.Duration) (net.Conn, error) {
			return fasthttp.DialTimeout(addr, timeout)
		},
	}
	installHostClientGuard(hc)

	require.NotNil(t, hc.DialTimeout)
	_, err := hc.DialTimeout("127.0.0.1:80", time.Second)
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)

	require.NotNil(t, hc.Dial)
	_, err = hc.Dial("127.0.0.1:80")
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
}

// Test_Security_NewGuardedClientDialer_DelegatesWhenPrivateAllowed verifies
// that when the operator opts into private targets the guard delegates to a
// normal dial, so loopback connections still succeed.
func Test_Security_NewGuardedClientDialer_DelegatesWhenPrivateAllowed(t *testing.T) {
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	withSecurityPolicyForTest(t, policy)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	dial := newGuardedClientDialer(nil, false)
	conn, err := dial(ln.Addr().String())
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}
