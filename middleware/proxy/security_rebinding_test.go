package proxy

import (
	"context"
	"errors"
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

// startManyAnswersDNS starts a loopback UDP DNS server that answers every A query with n distinct
// public addresses and every AAAA query with an empty NODATA response. It returns its "host:port".
func startManyAnswersDNS(t *testing.T, n int) string {
	t.Helper()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { pc.Close() }) //nolint:errcheck // best-effort close in cleanup

	go func() {
		buf := make([]byte, 512)
		for {
			size, addr, rerr := pc.ReadFrom(buf)
			if rerr != nil {
				return // listener closed
			}
			var p dnsmessage.Parser
			hdr, perr := p.Start(buf[:size])
			if perr != nil {
				continue
			}
			q, perr := p.Question()
			if perr != nil {
				continue
			}

			b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: hdr.ID, Response: true, Authoritative: true})
			b.EnableCompression()
			if b.StartQuestions() != nil || b.Question(q) != nil || b.StartAnswers() != nil {
				continue
			}
			if q.Type == dnsmessage.TypeA {
				for i := range n {
					rerr := b.AResource(dnsmessage.ResourceHeader{
						Name: q.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET,
					}, dnsmessage.AResource{A: [4]byte{198, 51, 100, byte(i + 1)}})
					if rerr != nil {
						break
					}
				}
			}
			resp, ferr := b.Finish()
			if ferr != nil {
				continue
			}
			pc.WriteTo(resp, addr) //nolint:errcheck // best-effort UDP reply
		}
	}()

	return pc.LocalAddr().String()
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

// startSilentDNS starts a loopback UDP socket that never answers, and returns its "host:port".
func startSilentDNS(t *testing.T) string {
	t.Helper()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { pc.Close() }) //nolint:errcheck // best-effort close in cleanup
	return pc.LocalAddr().String()
}

// recordedDial is a connect asked of one of fasthttp's dialers.
type recordedDial struct {
	via     string
	addr    string
	timeout time.Duration
}

// withRecordedDialers puts stand-ins for fasthttp's four dialers where a guard finds them, for the
// duration of t, and returns the connects they are asked for. The seam is process-wide: not parallel.
func withRecordedDialers(t *testing.T) *[]recordedDial {
	t.Helper()

	var dials []recordedDial
	record := func(via, addr string, timeout time.Duration) net.Conn {
		dials = append(dials, recordedDial{via: via, addr: addr, timeout: timeout})
		return stubConn{addr: addr}
	}
	prev := fasthttpDialers.Swap(&dialFuncs{
		dial:          func(addr string) (net.Conn, error) { return record("Dial", addr, 0), nil },
		dialDualStack: func(addr string) (net.Conn, error) { return record("DialDualStack", addr, 0), nil },
		dialTimeout: func(addr string, timeout time.Duration) (net.Conn, error) {
			return record("DialTimeout", addr, timeout), nil
		},
		dialDualStackTimeout: func(addr string, timeout time.Duration) (net.Conn, error) {
			return record("DialDualStackTimeout", addr, timeout), nil
		},
	})
	t.Cleanup(func() { fasthttpDialers.Store(prev) })
	return &dials
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
			// Intentionally not parallel: each case swaps the process-wide DNS resolver.

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

// Test_Security_ClientDialer_BlocksWhenPrivateDisallowed verifies
// the composable client guard rejects loopback targets — literal and
// hostname — when the active policy forbids private IPs.
func Test_Security_ClientDialer_BlocksWhenPrivateDisallowed(t *testing.T) {
	// Intentionally not parallel: it sets the strict security policy process-wide.
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false

	dial := newDialGuard(false).clientDialer(nil)

	_, err := dial("127.0.0.1:80")
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)

	_, err = dial("localhost:80")
	require.ErrorIs(t, err, ErrUpstreamHostBlocked)
}

// Test_Security_ClientDialer_ComposesWithOrig verifies that, for
// an allowed (public) target, the guard dials the validated address through
// the caller-supplied dialer instead of establishing its own connection.
func Test_Security_ClientDialer_ComposesWithOrig(t *testing.T) {
	// Intentionally not parallel: it sets the strict security policy process-wide.
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false

	var got string
	orig := func(addr string) (net.Conn, error) {
		got = addr
		return stubConn{addr: addr}, nil
	}

	dial := newDialGuard(false).clientDialer(orig)
	conn, err := dial("203.0.113.5:80") // TEST-NET-3, not blocked
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Equal(t, "203.0.113.5:80", got)
}

// Test_Security_ClientDialer_AllowsValidatedHostname is the
// positive-path counterpart to the rebinding test: under the strict policy, a
// hostname that resolves to a public (non-blocked) address must resolve,
// validate, and dial through to that exact IP — proving the guard does not
// over-block legitimate upstreams. Every end-to-end suite test runs with
// AllowPrivateIPs=true (the delegate branch), so this is the only coverage of
// the resolve→validate→dial success path.
func Test_Security_ClientDialer_AllowsValidatedHostname(t *testing.T) {
	// Intentionally not parallel: it swaps the process-wide policy and DNS resolver.
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false

	// Resolver maps the name to a public IP on every answer (no rebinding).
	dnsAddr, _ := startRebindingDNS(t, net.IPv4(198, 51, 100, 7), net.IPv4(198, 51, 100, 7))
	withRebindingResolver(t, dnsAddr)

	var got string
	orig := func(addr string) (net.Conn, error) {
		got = addr
		return stubConn{addr: addr}, nil
	}

	dial := newDialGuard(false).clientDialer(orig)
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
	// Intentionally not parallel: it sets the strict security policy process-wide.
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

// Test_Security_EnsureClientGuarded_ConcurrentIsSafe pins that guarding the same client from many
// goroutines is race-free (run under -race) and leaves one working hook.
func Test_Security_EnsureClientGuarded_ConcurrentIsSafe(t *testing.T) {
	t.Parallel()

	for name, withHook := range map[string]bool{
		"nil ConfigureClient":          false,
		"pre-existing ConfigureClient": true,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var hookCalls atomic.Int32
			cli := &fasthttp.Client{}
			if withHook {
				cli.ConfigureClient = func(*fasthttp.HostClient) error {
					hookCalls.Add(1)
					return nil
				}
			}

			var wg sync.WaitGroup
			for range 32 {
				wg.Go(func() {
					ensureClientGuarded(cli)
				})
			}
			wg.Wait()

			hc := &fasthttp.HostClient{}
			require.NoError(t, cli.ConfigureClient(hc))
			require.NotNil(t, hc.Dial, "the guard installs a dial-time Dial")
			require.True(t, hc.DisablePathNormalizing, "the guard hook ran")
			if withHook {
				require.Equal(t, int32(1), hookCalls.Load(), "the existing hook runs once per HostClient")
			}
		})
	}
}

// Test_Security_InstallHostClientGuard_GuardsDialTimeout verifies both dial
// entry points are guarded: a HostClient that carries DialTimeout has it
// wrapped (fasthttp prefers DialTimeout over Dial), and the wrapped func
// blocks a loopback target under the strict policy.
func Test_Security_InstallHostClientGuard_GuardsDialTimeout(t *testing.T) {
	// Intentionally not parallel: it sets the strict security policy process-wide.
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

// Test_Security_ClientDialer_DelegatesWhenPrivateAllowed verifies
// that when the operator opts into private targets the guard delegates to a
// normal dial, so loopback connections still succeed.
func Test_Security_ClientDialer_DelegatesWhenPrivateAllowed(t *testing.T) {
	// Intentionally not parallel: it sets a process-wide security policy.
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	withSecurityPolicyForTest(t, policy)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close() //nolint:errcheck // best-effort close

	dial := newDialGuard(false).clientDialer(nil)
	conn, err := dial(ln.Addr().String())
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}

// Test_Security_ClientDialer_DelegatesDualStack covers the
// AllowPrivateIPs delegate branch with DialDualStack enabled (fasthttp's
// DialDualStack fallback).
func Test_Security_ClientDialer_DelegatesDualStack(t *testing.T) {
	// Intentionally not parallel: it sets a process-wide security policy.
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	withSecurityPolicyForTest(t, policy)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close() //nolint:errcheck // best-effort close

	conn, err := newDialGuard(true).clientDialer(nil)(ln.Addr().String())
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}

// Test_Security_InstallHostClientGuard_DialTimeout pins which HostClients get a guarded DialTimeout.
func Test_Security_InstallHostClientGuard_DialTimeout(t *testing.T) {
	t.Parallel()

	t.Run("no dialer of its own", func(t *testing.T) {
		t.Parallel()

		hc := &fasthttp.HostClient{}
		installHostClientGuard(hc)
		require.NotNil(t, hc.DialTimeout)
		require.NotNil(t, hc.Dial)
	})

	t.Run("a Dial of its own is kept alone", func(t *testing.T) {
		t.Parallel()

		// A DialTimeout would take precedence over it, and the dialer it stands for cannot take a timeout.
		hc := &fasthttp.HostClient{Dial: func(string) (net.Conn, error) { return nil, errors.New("unused") }}
		installHostClientGuard(hc)
		require.Nil(t, hc.DialTimeout)
		require.NotNil(t, hc.Dial)
	})
}

// Test_Security_DefaultDialTimeout_ConnectsThroughFasthttp pins that the default dialer connects with
// the request's timeout, or fasthttp's default when it left none, where DialTimeout would fail at once.
func Test_Security_DefaultDialTimeout_ConnectsThroughFasthttp(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck // best-effort close in cleanup

	for _, dualStack := range []bool{false, true} {
		for _, timeout := range []time.Duration{0, time.Second} {
			conn, dialErr := defaultDialTimeout(dualStack)(ln.Addr().String(), timeout)
			require.NoError(t, dialErr, "dual stack %v, timeout %v", dualStack, timeout)
			require.NoError(t, conn.Close())
		}
	}
}

// Test_Security_DefaultDialTimeout_ChoosesTheDialer pins which of fasthttp's four dialers connects.
func Test_Security_DefaultDialTimeout_ChoosesTheDialer(t *testing.T) {
	// Intentionally not parallel: it swaps the process-wide fasthttp dialers.
	dials := withRecordedDialers(t)

	for _, tc := range []struct {
		want      recordedDial
		timeout   time.Duration
		dualStack bool
	}{
		{want: recordedDial{via: "Dial", addr: "198.51.100.7:80"}},
		{want: recordedDial{via: "DialDualStack", addr: "198.51.100.7:80"}, dualStack: true},
		{want: recordedDial{via: "DialTimeout", addr: "198.51.100.7:80", timeout: time.Second}, timeout: time.Second},
		{want: recordedDial{via: "DialDualStackTimeout", addr: "198.51.100.7:80", timeout: time.Second}, timeout: time.Second, dualStack: true},
	} {
		*dials = nil
		_, err := defaultDialTimeout(tc.dualStack)("198.51.100.7:80", tc.timeout)
		require.NoError(t, err)
		require.Equal(t, []recordedDial{tc.want}, *dials, "dual stack %v, timeout %v", tc.dualStack, tc.timeout)
	}
}

// Test_Security_GuardedDialTimeouts_HandTheTimeoutOn pins that every guarded DialTimeout checks the
// address, then connects through fasthttp's dialer for its stack with the time the request left, if any.
func Test_Security_GuardedDialTimeouts_HandTheTimeoutOn(t *testing.T) {
	// Intentionally not parallel: it swaps the process-wide security policy and fasthttp dialers.
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false
	dials := withRecordedDialers(t)

	upstream := mustParseTestURL(t, "http://upstream.example")
	for _, dualStack := range []bool{false, true} {
		guarded := map[string]func() fasthttp.DialFuncWithTimeout{
			"a client with no dialer of its own": func() fasthttp.DialFuncWithTimeout {
				hc := &fasthttp.HostClient{DialDualStack: dualStack}
				installHostClientGuard(hc)
				return hc.DialTimeout
			},
			"a Balancer host client": func() fasthttp.DialFuncWithTimeout {
				return newHostClient(upstream, &Config{DialDualStack: dualStack}, DefaultSecurityPolicy()).DialTimeout
			},
			"the guard": func() fasthttp.DialFuncWithTimeout {
				return newDialGuard(dualStack).dialCheckedTimeout
			},
		}
		viaTimeout, via := "DialTimeout", "Dial"
		if dualStack {
			viaTimeout, via = "DialDualStackTimeout", "DialDualStack"
		}
		for name, build := range guarded {
			dial := build()
			require.NotNil(t, dial, "%s, dual stack %v", name, dualStack)

			*dials = nil
			_, err := dial("198.51.100.7:80", 2*time.Second)
			require.NoError(t, err, "%s, dual stack %v", name, dualStack)
			require.Len(t, *dials, 1, "%s, dual stack %v", name, dualStack)
			got := (*dials)[0]
			require.Equal(t, viaTimeout, got.via, "%s, dual stack %v", name, dualStack)
			require.Equal(t, "198.51.100.7:80", got.addr, "%s, dual stack %v", name, dualStack)
			require.Positive(t, got.timeout, "%s, dual stack %v: the connect gets a timeout", name, dualStack)
			require.LessOrEqual(t, got.timeout, 2*time.Second, "%s, dual stack %v: and not fasthttp's default of %v", name, dualStack, fasthttp.DefaultDialTimeout)

			*dials = nil
			_, err = dial("198.51.100.7:80", 0)
			require.NoError(t, err, "%s, dual stack %v", name, dualStack)
			require.Equal(t, []recordedDial{{via: via, addr: "198.51.100.7:80"}}, *dials, "%s, dual stack %v", name, dualStack)
		}
	}
}

// Test_Security_DialWithin_SharesTheTimeout pins that the lookup and all the connects share the time
// the request leaves: a connect gets what the earlier ones left, and none is made once it is spent.
func Test_Security_DialWithin_SharesTheTimeout(t *testing.T) {
	// Intentionally not parallel: it swaps the process-wide DNS resolver.
	withRebindingResolver(t, startManyAnswersDNS(t, 2)) // 198.51.100.1 and 198.51.100.2

	guard := newDialGuard(false)

	t.Run("a connect gets what is left", func(t *testing.T) {
		var timeouts []time.Duration
		dial := func(_ string, timeout time.Duration) (net.Conn, error) {
			timeouts = append(timeouts, timeout)
			if len(timeouts) == 1 {
				time.Sleep(300 * time.Millisecond)
				return nil, errors.New("connection refused")
			}
			return stubConn{}, nil
		}

		_, err := guard.dialWithin("many.test:80", 3*time.Second, dial)
		require.NoError(t, err)
		require.Len(t, timeouts, 2)
		require.Greater(t, timeouts[0], 2*time.Second, "the lookup left most of it")
		require.LessOrEqual(t, timeouts[0], 3*time.Second)
		require.Positive(t, timeouts[1])
		require.LessOrEqual(t, timeouts[1], 3*time.Second-250*time.Millisecond, "the first connect took 300 ms of it")
	})

	t.Run("none is made once nothing is left", func(t *testing.T) {
		var dialed int
		dial := func(_ string, timeout time.Duration) (net.Conn, error) {
			dialed++
			time.Sleep(timeout) // an address nobody answers at, which uses all it is given
			return nil, fasthttp.ErrDialTimeout
		}

		_, err := guard.dialWithin("many.test:80", 500*time.Millisecond, dial)
		require.ErrorIs(t, err, fasthttp.ErrDialTimeout)
		require.Equal(t, 1, dialed, "the second address is not tried")
	})
}

// Test_Security_DialWithin_NoTimeout pins that a dial the request gave no time hands none on.
func Test_Security_DialWithin_NoTimeout(t *testing.T) {
	t.Parallel()

	got := time.Hour
	_, err := newDialGuard(false).dialWithin("198.51.100.7:80", 0, func(_ string, timeout time.Duration) (net.Conn, error) {
		got = timeout
		return stubConn{}, nil
	})
	require.NoError(t, err)
	require.Zero(t, got)
}

// Test_Security_DialWithin_BoundsTheLookup pins that the request's time ends the lookup too: a resolver
// that never answers costs the request what it allowed, not the five seconds of the lookup limit.
func Test_Security_DialWithin_BoundsTheLookup(t *testing.T) {
	// Intentionally not parallel: it swaps the process-wide DNS resolver.
	withRebindingResolver(t, startSilentDNS(t))

	var dialed int
	start := time.Now()
	_, err := newDialGuard(false).dialWithin("silent.test:80", 200*time.Millisecond, func(string, time.Duration) (net.Conn, error) {
		dialed++
		return stubConn{}, nil
	})

	require.ErrorIs(t, err, ErrUpstreamHostUnresolvable)
	require.Zero(t, dialed)
	require.Less(t, time.Since(start), dnsLookupTimeout/2)
}

// Test_Security_RequestTimeout_ReachesTheConnect pins that the time a request leaves reaches the
// dialer that connects, through DoTimeout on the shared client and through a Balancer with a Timeout.
func Test_Security_RequestTimeout_ReachesTheConnect(t *testing.T) {
	// Intentionally not parallel: it swaps the process-wide security policy, client and fasthttp dialers.
	restoreGlobalProxyClient(t)
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false
	WithClient(&fasthttp.Client{})
	dials := withRecordedDialers(t)

	// TEST-NET-2 is documentation space, outside the blocked ranges.
	const upstream = "http://198.51.100.9"
	timeout := 2 * time.Second // below fasthttp's default of 3 s
	for name, handler := range map[string]fiber.Handler{
		"DoTimeout": func(c fiber.Ctx) error { return DoTimeout(c, upstream+"/", timeout) },
		"Balancer":  Balancer(Config{Servers: []string{upstream}, Timeout: timeout}),
	} {
		*dials = nil
		app := fiber.New()
		app.Get("/", handler)
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody), fiber.TestConfig{Timeout: 5 * time.Second, FailOnTimeout: true})
		require.NoError(t, err, name)
		require.NoError(t, resp.Body.Close(), name) // the stand-in connection answers nothing, so the request fails

		require.NotEmpty(t, *dials, name)
		first := (*dials)[0]
		require.Equal(t, "DialTimeout", first.via, name)
		require.Equal(t, "198.51.100.9:80", first.addr, name)
		require.Positive(t, first.timeout, name)
		require.LessOrEqual(t, first.timeout, timeout, name)
	}
}

// Test_Security_ClientDialerWithTimeout_Delegates covers the
// DialFuncWithTimeout guard's delegate branch: when private IPs are allowed
// it forwards the raw addr and timeout to the caller's dialer.
func Test_Security_ClientDialerWithTimeout_Delegates(t *testing.T) {
	// Intentionally not parallel: it sets a process-wide security policy.
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	withSecurityPolicyForTest(t, policy)

	var got string
	var gotTimeout time.Duration
	orig := func(addr string, timeout time.Duration) (net.Conn, error) {
		got, gotTimeout = addr, timeout
		return stubConn{addr: addr}, nil
	}
	conn, err := newDialGuard(false).clientDialerWithTimeout(orig)("10.0.0.1:80", time.Second)
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Equal(t, "10.0.0.1:80", got)
	require.Equal(t, time.Second, gotTimeout)
}

// Test_Security_ClientDialerWithTimeout_ValidatedDialsThrough covers
// the strict-policy path of the DialFuncWithTimeout guard: a hostname that
// resolves to a public IP is validated and dialed through the caller's
// timeout dialer at the exact resolved IP.
func Test_Security_ClientDialerWithTimeout_ValidatedDialsThrough(t *testing.T) {
	// Intentionally not parallel: it swaps the process-wide policy and DNS resolver.
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false

	dnsAddr, _ := startRebindingDNS(t, net.IPv4(198, 51, 100, 7), net.IPv4(198, 51, 100, 7))
	withRebindingResolver(t, dnsAddr)

	var got string
	var gotTimeout time.Duration
	orig := func(addr string, timeout time.Duration) (net.Conn, error) {
		got, gotTimeout = addr, timeout
		return stubConn{addr: addr}, nil
	}
	conn, err := newDialGuard(false).clientDialerWithTimeout(orig)("public.test:80", time.Second)
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Equal(t, "198.51.100.7:80", got, "must dial the validated resolved IP")
	// What the lookup left of the second, not the second again.
	require.Greater(t, gotTimeout, 500*time.Millisecond)
	require.LessOrEqual(t, gotTimeout, time.Second)
}

// Test_Security_EnsureClientGuarded_ComposePropagatesError verifies the
// composed hook surfaces an error from the user's existing ConfigureClient.
func Test_Security_EnsureClientGuarded_ComposePropagatesError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("configure boom")
	cli := &fasthttp.Client{
		ConfigureClient: func(_ *fasthttp.HostClient) error { return wantErr },
	}
	ensureClientGuarded(cli)

	err := cli.ConfigureClient(&fasthttp.HostClient{})
	require.ErrorIs(t, err, wantErr)
}

// Test_Security_ClientDialer_DialsValidatedIPWithGuardDialer covers the strict-policy path for a
// client with no dialer of its own: the validated IP is dialed through the guard's dialer.
func Test_Security_ClientDialer_DialsValidatedIPWithGuardDialer(t *testing.T) {
	// Intentionally not parallel: it sets the strict security policy process-wide.
	withSecurityPolicyForTest(t, DefaultSecurityPolicy()) // AllowPrivateIPs == false

	var network, address string
	guard := dialGuard{dial: func(n, a string) (net.Conn, error) {
		network, address = n, a
		return stubConn{addr: a}, nil
	}}

	// TEST-NET-2 is documentation space, outside the blocked ranges.
	conn, err := guard.clientDialer(nil)("198.51.100.7:8080")
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Equal(t, "tcp", network)
	require.Equal(t, "198.51.100.7:8080", address)
}
