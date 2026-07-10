package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/dns/dnsmessage"
)

// startRebindingDNS starts a loopback UDP DNS server that answers A queries
// for any name. The very first A answer for a given name is firstIP (a
// public-looking address that passes the SSRF blocklist); every A answer
// after that is rebindIP (the "internal" target). AAAA queries are answered
// with an empty NODATA response so the Go resolver falls back to the A
// record. This reproduces a classic DNS-rebinding resolver: safe on the
// validation lookup, hostile on the dial-time lookup. It returns the
// server's "host:port" address and a counter of A queries served, which a
// test can assert is >= 2 to prove the block happened at dial time (a second
// lookup) rather than up front (the first, public, lookup).
func startRebindingDNS(t *testing.T, firstIP, rebindIP net.IP) (string, *atomic.Int64) {
	t.Helper()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { pc.Close() }) //nolint:errcheck // best-effort close in cleanup

	aQueries := &atomic.Int64{}

	var (
		mu       sync.Mutex
		answered = map[string]bool{}
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
				aQueries.Add(1)
				// Track "first answer" per queried name so a stray lookup for
				// some other name cannot consume the target name's public
				// (validation-time) answer and mask a broken dial-time guard.
				name := q.Name.String()
				mu.Lock()
				if answered[name] {
					answer = rebindIP
				} else {
					answer = firstIP
					answered[name] = true
				}
				mu.Unlock()
			}

			resp, berr := buildDNSResponse(hdr.ID, &q, answer)
			if berr != nil {
				continue
			}
			pc.WriteTo(resp, addr) //nolint:errcheck // best-effort UDP reply
		}
	}()

	return pc.LocalAddr().String(), aQueries
}

// buildDNSResponse serializes a DNS reply echoing the question. When ip is
// a non-nil IPv4 address an A record is included; otherwise the reply is a
// NODATA answer (used for AAAA queries).
func buildDNSResponse(id uint16, q *dnsmessage.Question, ip net.IP) ([]byte, error) {
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{
		ID:            id,
		Response:      true,
		Authoritative: true,
	})
	b.EnableCompression()
	if err := b.StartQuestions(); err != nil {
		return nil, fmt.Errorf("dns build questions: %w", err)
	}
	if err := b.Question(*q); err != nil {
		return nil, fmt.Errorf("dns build question: %w", err)
	}
	if err := b.StartAnswers(); err != nil {
		return nil, fmt.Errorf("dns build answers: %w", err)
	}
	if v4 := ip.To4(); v4 != nil {
		var a [4]byte
		copy(a[:], v4)
		if err := b.AResource(dnsmessage.ResourceHeader{
			Name:  q.Name,
			Type:  dnsmessage.TypeA,
			Class: dnsmessage.ClassINET,
		}, dnsmessage.AResource{A: a}); err != nil {
			return nil, fmt.Errorf("dns build A record: %w", err)
		}
	}
	out, err := b.Finish()
	if err != nil {
		return nil, fmt.Errorf("dns build finish: %w", err)
	}
	return out, nil
}

// withRebindingResolver points the proxy's SSRF-validation resolver at the
// fake DNS server at dnsAddr for the duration of t, restoring the previous
// resolver via t.Cleanup. It swaps the package's atomic dnsResolver seam
// rather than the process-global net.DefaultResolver, so it does not race the
// DNS lookups other tests' background dial goroutines perform. Tests using it
// must not call t.Parallel: the seam is process-global.
func withRebindingResolver(t *testing.T, dnsAddr string) {
	t.Helper()
	prev := dnsResolver.Load()
	dnsResolver.Store(&net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "udp", dnsAddr)
		},
	})
	t.Cleanup(func() { dnsResolver.Store(prev) })
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
			cli := &fasthttp.Client{DialTimeout: fasthttp.DialTimeout}
			return Do(c, target, cli)
		},
		// DoRedirects dispatches through the same guarded client; the first
		// hop's dial must be blocked. Pins that the redirect path (and its
		// per-hop cli.Do) is guarded too.
		"DoRedirects": func(c fiber.Ctx, target string) error {
			return DoRedirects(c, target, 3)
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
			dnsAddr, aQueries := startRebindingDNS(t, net.IPv4(198, 51, 100, 1), net.IPv4(127, 0, 0, 1))
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
			// >= 2 A lookups proves the up-front validation lookup (public,
			// allowed) happened AND a second dial-time lookup (rebound,
			// blocked) happened — i.e. the block is genuinely at dial time,
			// not an up-front block masking a broken guard.
			require.GreaterOrEqual(t, aQueries.Load(), int64(2),
				"expected an up-front and a dial-time DNS lookup")
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

// Test_Security_NewGuardedClientDialer_AllowsValidatedHostname is the
// positive-path counterpart to the rebinding test: under the strict policy, a
// hostname that resolves to a public (non-blocked) address must resolve,
// validate, and dial through to that exact IP — proving the guard does not
// over-block legitimate upstreams. Every end-to-end suite test runs with
// AllowPrivateIPs=true (the delegate branch), so this is the only coverage of
// the resolve→validate→dial success path.
func Test_Security_NewGuardedClientDialer_AllowsValidatedHostname(t *testing.T) {
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false

	// Resolver maps the name to a public IP on every answer (no rebinding).
	dnsAddr, _ := startRebindingDNS(t, net.IPv4(198, 51, 100, 7), net.IPv4(198, 51, 100, 7))
	withRebindingResolver(t, dnsAddr)

	var got string
	orig := func(addr string) (net.Conn, error) {
		got = addr
		return stubConn{addr: addr}, nil
	}

	dial := newGuardedClientDialer(orig, false)
	conn, err := dial("public.test:80")
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Equal(t, "198.51.100.7:80", got, "must dial the validated resolved IP")
}

// Test_Security_EnsureClientGuarded_ComposesAndIsIdempotent verifies that
// installing the guard on a client that already carries its own
// ConfigureClient hook composes with it (the original still runs, exactly
// once, not nested on repeated installs) AND that the guard is the outermost
// dialer even when the user's hook installs its own hc.Dial — i.e. the
// composition order is existing-then-guard. A guard-then-existing order would
// let the user's dialer overwrite the guard and reopen the SSRF hole, so this
// test is the regression guard for that ordering.
func Test_Security_EnsureClientGuarded_ComposesAndIsIdempotent(t *testing.T) {
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false

	called := 0
	userDialed := false
	cli := &fasthttp.Client{
		ConfigureClient: func(hc *fasthttp.HostClient) error {
			called++
			// A user hook that installs its own (unguarded) dialer — the
			// canonical reason to use ConfigureClient. The guard must run
			// after this and wrap it.
			hc.Dial = func(addr string) (net.Conn, error) {
				userDialed = true
				return stubConn{addr: addr}, nil
			}
			return nil
		},
	}

	ensureClientGuarded(cli)
	ensureClientGuarded(cli) // must not wrap a second time

	hc := &fasthttp.HostClient{}
	require.NoError(t, cli.ConfigureClient(hc))
	require.Equal(t, 1, called, "original ConfigureClient must run exactly once per HostClient")
	require.NotNil(t, hc.Dial, "guard must install a dial-time Dial")

	// A loopback target must be blocked BEFORE the user's dialer runs: proves
	// the guard wraps the user's final Dial (order is existing-then-guard).
	_, err := hc.Dial("127.0.0.1:80")
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
	require.False(t, userDialed, "guard must reject a blocked IP before delegating to the user's dialer")

	// A validated (public) target must delegate through to the user's dialer.
	_, err = hc.Dial("203.0.113.9:80") // TEST-NET-3, not blocked
	require.NoError(t, err)
	require.True(t, userDialed, "guard must delegate an allowed IP to the user's dialer")
}

// Test_Security_EnsureClientGuarded_ConcurrentIsSafe pins that guarding the
// same client from many goroutines is race-free (run under -race) and
// idempotent, for both the nil-ConfigureClient and pre-existing-hook paths.
// Regression guard for the serializing mutex in ensureClientGuarded.
func Test_Security_EnsureClientGuarded_ConcurrentIsSafe(t *testing.T) {
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false

	cases := map[string]func(*fasthttp.HostClient) error{
		"nil ConfigureClient":          nil,
		"pre-existing ConfigureClient": func(_ *fasthttp.HostClient) error { return nil },
	}

	for name, hook := range cases {
		t.Run(name, func(t *testing.T) {
			cli := &fasthttp.Client{ConfigureClient: hook}

			var wg sync.WaitGroup
			for range 32 {
				wg.Go(func() {
					ensureClientGuarded(cli)
				})
			}
			wg.Wait()

			hc := &fasthttp.HostClient{}
			require.NoError(t, cli.ConfigureClient(hc))
			require.NotNil(t, hc.Dial)
			_, err := hc.Dial("127.0.0.1:80")
			require.ErrorIs(t, err, ErrUpstreamHostBlocked)
		})
	}
}

// Test_Security_InstallHostClientGuard_GuardsDialTimeout verifies both dial
// entry points are guarded: a HostClient that carries DialTimeout has it
// wrapped (fasthttp prefers DialTimeout over Dial), and the wrapped func
// blocks a loopback target under the strict policy.
func Test_Security_InstallHostClientGuard_GuardsDialTimeout(t *testing.T) {
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false

	hc := &fasthttp.HostClient{DialTimeout: fasthttp.DialTimeout}
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
	defer ln.Close() //nolint:errcheck // best-effort close

	dial := newGuardedClientDialer(nil, false)
	conn, err := dial(ln.Addr().String())
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}
