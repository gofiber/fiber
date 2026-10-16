package proxy

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// dnsLookupTimeout bounds DNS resolution performed during upstream
// validation and dialing, so a slow or unresponsive resolver cannot hang
// a request (or application startup) indefinitely.
const dnsLookupTimeout = 5 * time.Second

// dnsResolver is the resolver used for SSRF validation lookups. It defaults
// to net.DefaultResolver and is only ever swapped by tests, atomically, so a
// concurrent validation lookup never races the swap. Tests use this seam
// rather than mutating the process-global net.DefaultResolver — which
// fasthttp and the net package read from background dial goroutines, making a
// direct swap a data race under -race.
var dnsResolver atomic.Pointer[net.Resolver]

func init() {
	dnsResolver.Store(net.DefaultResolver)
}

// validateHostForSSRF rejects hostnames that resolve to addresses inside
// the blocked ranges. The IP literal shortcut avoids DNS lookups when
// the host is already a numeric address.
func validateHostForSSRF(host string) error {
	if host == "" {
		return ErrUpstreamHostInvalid
	}
	// strip brackets from IPv6 literals (url.Hostname already does this
	// in most cases, but keep the guard for defensive callers).
	_, err := resolveAndValidateHost(trimBrackets(host))
	return err
}

// resolveAndValidateHost looks up host (or treats it as an IP literal),
// then enforces the SSRF blocklist on every returned address. A single
// blocked answer fails the whole resolution so a mixed public/private
// reply cannot slip past the guard.
func resolveAndValidateHost(host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return nil, fmt.Errorf("%w: %s", ErrUpstreamHostBlocked, ip)
		}
		return []net.IP{ip}, nil
	}
	// Bound the lookup so a slow resolver cannot stall the caller.
	ctx, cancel := context.WithTimeout(context.Background(), dnsLookupTimeout)
	defer cancel()
	addrs, err := dnsResolver.Load().LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("%w: %s lookup failed: %w", ErrUpstreamHostBlocked, host, err)
	}
	ips := make([]net.IP, len(addrs))
	for i, addr := range addrs {
		if isBlockedIP(addr.IP) {
			return nil, fmt.Errorf("%w: %s -> %s", ErrUpstreamHostBlocked, host, addr.IP)
		}
		ips[i] = addr.IP
	}
	return ips, nil
}

// ssrfDialFunc is the (network, address) -> conn signature consumed by
// dialGuard. It accepts the standard library *net.Dialer's Dial method
// directly, and lets tests inject a deterministic stub.
type ssrfDialFunc func(network, address string) (net.Conn, error)

// dialGuard is the SSRF guard in front of a client's dialer. It resolves the
// address about to be dialed, rejects it if any resolved address falls in a
// blocked range, and only then dials an address that passed. Checking at dial
// time — rather than only up front — defeats DNS-rebinding attacks (the
// check/use gap) where a resolver returns a public address during validation
// and a private one at connect time.
type dialGuard struct {
	// dial connects to an address that passed the blocklist, for a guarded
	// client with no dialer of its own.
	dial ssrfDialFunc

	// dualStack mirrors fasthttp's DialDualStack on the guarded client: IPv6
	// addresses are dialed only when it is set.
	dualStack bool
}

// newDialGuard returns the guard for a client with the given DialDualStack.
// It dials validated addresses with a net.Dialer bounded by dnsLookupTimeout.
func newDialGuard(dualStack bool) dialGuard {
	dialer := &net.Dialer{Timeout: dnsLookupTimeout}
	return dialGuard{dial: dialer.Dial, dualStack: dualStack}
}

// strictDialer returns a fasthttp DialFunc that applies the blocklist to every
// dial. Balancer installs it only when its policy disallows private IPs, so it
// never consults the policy itself.
func (g dialGuard) strictDialer() fasthttp.DialFunc {
	return func(addr string) (net.Conn, error) {
		return g.dialChecked(addr, g.dial)
	}
}

// clientDialer wraps orig with the guard. Unlike strictDialer — whose blocking
// is unconditional because Balancer only installs it when the policy forbids
// private IPs — this dialer is consulted by clients that outlive a single
// policy snapshot, so it re-reads the active global policy on every dial. When
// the caller supplied their own dialer it is run in both paths so custom
// transport is preserved while a blocked-policy connection still targets the
// validated address.
//
// When orig is nil the two paths differ, matching strictDialer: the delegate
// (private-IPs-allowed) path hands the raw address to fasthttp's default dialer
// (Dial/DialDualStack), while the validated (blocked-policy) path connects to
// the already-checked IP with g.dial — fasthttp.Dial is not reused there
// because the target is a literal IP that needs no re-resolution and the
// explicit timeout bounds the connect.
func (g dialGuard) clientDialer(orig fasthttp.DialFunc) fasthttp.DialFunc {
	delegate := func(addr string) (net.Conn, error) {
		if orig != nil {
			return orig(addr)
		}
		if g.dualStack {
			return fasthttp.DialDualStack(addr)
		}
		return fasthttp.Dial(addr)
	}
	dialValidated := func(network, address string) (net.Conn, error) {
		if orig != nil {
			return orig(address)
		}
		return g.dial(network, address)
	}
	return func(addr string) (net.Conn, error) {
		return g.guardedDial(addr, delegate, dialValidated)
	}
}

// clientDialerWithTimeout is the DialFuncWithTimeout counterpart of
// clientDialer. orig is always non-nil here (installed only when the HostClient
// already carries a DialTimeout), and the per-request timeout is threaded
// through both the delegate and the validated dial so the caller's timeout
// semantics survive the guard.
func (g dialGuard) clientDialerWithTimeout(orig fasthttp.DialFuncWithTimeout) fasthttp.DialFuncWithTimeout {
	return func(addr string, timeout time.Duration) (net.Conn, error) {
		delegate := func(a string) (net.Conn, error) {
			return orig(a, timeout)
		}
		dialValidated := func(_, address string) (net.Conn, error) {
			return orig(address, timeout)
		}
		return g.guardedDial(addr, delegate, dialValidated)
	}
}

// guardedDial is the shared core of both client dialers. When the active
// policy allows private IPs the caller has opted into internal targets, so it
// delegates unchanged. Otherwise it dials through dialChecked, closing the
// DNS-rebinding check/use window the up-front validateUpstream lookup leaves
// open. The policy is read fresh on every dial because a shared client
// outlives any single policy snapshot.
func (g dialGuard) guardedDial(
	addr string,
	delegate func(addr string) (net.Conn, error),
	dialValidated ssrfDialFunc,
) (net.Conn, error) {
	if currentSecurityPolicy().AllowPrivateIPs {
		return delegate(addr)
	}
	return g.dialChecked(addr, dialValidated)
}

// dialChecked resolves addr, rejects it if any address it names falls in a
// blocked range, and otherwise dials one that passed with dial.
func (g dialGuard) dialChecked(addr string, dial ssrfDialFunc) (net.Conn, error) {
	host, port, ok := utils.SplitHostPort(addr)
	if !ok {
		return nil, fmt.Errorf("proxy: invalid dial address %q", addr)
	}
	ips, err := resolveAndValidateHost(host)
	if err != nil {
		return nil, err
	}
	return g.dialValidatedIPs(ips, host, port, dial)
}

// dialValidatedIPs walks ips and tries to dial the first non-skipped
// address. Unless the guard is dual-stack, IPv6 addresses are skipped to
// match fasthttp's IPv4-only default. If every candidate is skipped, a
// "no usable address" error is returned; otherwise the last dial error
// is propagated so the caller can see why each attempt failed.
func (g dialGuard) dialValidatedIPs(ips []net.IP, host, port string, dial ssrfDialFunc) (net.Conn, error) {
	var lastErr error
	for _, ip := range ips {
		// Mirror fasthttp's default of IPv4-only unless DialDualStack.
		if !g.dualStack && ip.To4() == nil {
			continue
		}
		conn, derr := dial("tcp", net.JoinHostPort(ip.String(), port))
		if derr == nil {
			return conn, nil
		}
		lastErr = derr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%w: %s has no usable address", ErrUpstreamHostBlocked, host)
	}
	return nil, lastErr
}

// installHostClientGuard fits hc with the policy-aware, dial-time SSRF
// guard. It is installed through fasthttp.Client.ConfigureClient, which
// runs once per HostClient at creation — so the guard is present before the
// first dial to that host and, crucially, covers BOTH dial code paths:
// fasthttp's callDialFunc prefers DialTimeout over Dial, so guarding only
// Dial would let a client that sets DialTimeout dial unvalidated. We wrap
// DialTimeout when present (preserving its per-request timeout) and always
// wrap Dial so the nil-DialTimeout and default-dialer paths are guarded too.
func installHostClientGuard(hc *fasthttp.HostClient) {
	g := newDialGuard(hc.DialDualStack)
	if hc.DialTimeout != nil {
		hc.DialTimeout = g.clientDialerWithTimeout(hc.DialTimeout)
	}
	hc.Dial = g.clientDialer(hc.Dial)
}

// guardedConfigureClient composes a client's optional pre-existing
// ConfigureClient hook with the dial-time SSRF guard. It is installed on a
// *fasthttp.Client as the bound method value (&guardedConfigureClient{…}).run,
// which fasthttp calls once per HostClient it creates — so the guard is
// present before the first dial to each host and covers both the Dial and
// DialTimeout code paths.
//
// The bound method's code pointer is stable across receivers (unlike a
// closure's), so ensureClientGuarded recognizes an already-guarded client by
// identity — no package-level map keyed by the client, which would pin the
// client and its connection pool for the process lifetime. The struct is
// referenced only from the client's own ConfigureClient field, so it is
// collected together with the client.
type guardedConfigureClient struct {
	// orig is the caller's ConfigureClient hook, or nil. It runs before the
	// guard so the guard wraps whatever Dial/DialTimeout it finally sets —
	// running it after would let a hook that assigns hc.Dial overwrite the
	// guard and reopen the SSRF hole.
	orig func(hc *fasthttp.HostClient) error
}

func (g *guardedConfigureClient) run(hc *fasthttp.HostClient) error {
	if g.orig != nil {
		if err := g.orig(hc); err != nil {
			return err
		}
	}
	installHostClientGuard(hc)
	// The target the proxy hands the client is the one it matched and
	// validated; a client left normalizing would decode "%2F" into a
	// separator and merge "//" on the way out, undoing that.
	hc.DisablePathNormalizing = true
	return nil
}

// guardHookPtr is the code pointer shared by every guardedConfigureClient.run
// method value, used to detect whether a client is already guarded.
var guardHookPtr = reflect.ValueOf((&guardedConfigureClient{}).run).Pointer()

// guardMu serializes the read-modify-write of a client's ConfigureClient in
// ensureClientGuarded, so a concurrent guarder cannot write the field mid
// update or observe it half-installed.
var guardMu sync.Mutex

// ensureClientGuarded installs the SSRF guard on cli via its ConfigureClient
// hook, exactly once (idempotent: a client already carrying the guard method
// is left untouched). It composes with any hook the client already has,
// without retaining a reference to the client. It is invoked at registration
// for the default client (init) and WithClient clients — before either is
// used — and on the dispatch path only for a per-call variadic client (the
// global client is already guarded), so the common no-variadic request path
// never reaches this lock. A handler built with a fixed variadic client (e.g.
// Forward(addr, cli)) does take guardMu once per request; that is a deliberate
// correctness-over-throughput choice (an unlocked fast path would race a
// first-time writer).
//
// Guarantee scope: the default client and WithClient clients are fully
// protected. A client supplied as a per-call variadic argument is guarded on
// first use — but a host it had already dialed keeps its cached, pre-hook
// HostClient, and mutating a client shared with concurrent non-proxy use is
// inherently racy. For a full guarantee with a custom client, register it
// via WithClient before first use, or hand the proxy a dedicated client.
func ensureClientGuarded(cli *fasthttp.Client) {
	guardMu.Lock()
	defer guardMu.Unlock()

	existing := cli.ConfigureClient
	if existing != nil && reflect.ValueOf(existing).Pointer() == guardHookPtr {
		return // already guarded (directly or composed with a user hook)
	}
	cli.ConfigureClient = (&guardedConfigureClient{orig: existing}).run
}
