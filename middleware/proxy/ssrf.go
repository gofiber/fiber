package proxy

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

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
	host = trimBrackets(host)
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("%w: %s", ErrUpstreamHostBlocked, ip)
		}
		return nil
	}
	// Bound the lookup so a slow resolver cannot stall the caller.
	ctx, cancel := context.WithTimeout(context.Background(), dnsLookupTimeout)
	defer cancel()
	addrs, err := dnsResolver.Load().LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("%w: %s lookup failed: %w", ErrUpstreamHostBlocked, host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("%w: %s has no addresses", ErrUpstreamHostBlocked, host)
	}
	for _, addr := range addrs {
		if isBlockedIP(addr.IP) {
			return fmt.Errorf("%w: %s -> %s", ErrUpstreamHostBlocked, host, addr.IP)
		}
	}
	return nil
}

// newSSRFDialer returns a fasthttp DialFunc that resolves the target host
// (with a bounded timeout), rejects the connection if any resolved
// address falls in a blocked range, and then dials a validated address.
// Performing the check at dial time — rather than only up front — defeats
// DNS-rebinding attacks (the check/use gap) where a resolver returns a
// public address during validation and a private one at connect time. It
// is only installed when the active policy disallows private IPs.
//
//nolint:revive // dialDualStack mirrors fasthttp.HostClient.DialDualStack
func newSSRFDialer(dialDualStack bool) fasthttp.DialFunc {
	dialer := &net.Dialer{Timeout: dnsLookupTimeout}
	return func(addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("proxy: invalid dial address %q: %w", addr, err)
		}
		ips, err := resolveAndValidateHost(host)
		if err != nil {
			return nil, err
		}
		return dialValidatedIPs(ips, host, port, dialDualStack, dialer.Dial)
	}
}

// resolveAndValidateHost looks up host (or treats it as an IP literal),
// then enforces the SSRF blocklist on every returned address. A single
// blocked answer fails the whole resolution so a mixed public/private
// reply cannot slip past the guard.
func resolveAndValidateHost(host string) ([]net.IP, error) {
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), dnsLookupTimeout)
		defer cancel()
		resolved, lerr := dnsResolver.Load().LookupIPAddr(ctx, host)
		if lerr != nil {
			return nil, fmt.Errorf("%w: %s lookup failed: %w", ErrUpstreamHostBlocked, host, lerr)
		}
		for _, r := range resolved {
			ips = append(ips, r.IP)
		}
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return nil, fmt.Errorf("%w: %s -> %s", ErrUpstreamHostBlocked, host, ip)
		}
	}
	return ips, nil
}

// ssrfDialFunc is the (network, address) -> conn signature consumed by
// dialValidatedIPs. It accepts the standard library *net.Dialer's Dial
// method directly, and lets tests inject a deterministic stub.
type ssrfDialFunc func(network, address string) (net.Conn, error)

// dialValidatedIPs walks ips and tries to dial the first non-skipped
// address. When dialDualStack is false, IPv6 addresses are skipped to
// match fasthttp's IPv4-only default. If every candidate is skipped, a
// "no usable address" error is returned; otherwise the last dial error
// is propagated so the caller can see why each attempt failed.
//
//nolint:revive // dialDualStack mirrors fasthttp.HostClient.DialDualStack
func dialValidatedIPs(ips []net.IP, host, port string, dialDualStack bool, dial ssrfDialFunc) (net.Conn, error) {
	var lastErr error
	for _, ip := range ips {
		// Mirror fasthttp's default of IPv4-only unless DialDualStack.
		if !dialDualStack && ip.To4() == nil {
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
	if hc.DialTimeout != nil {
		hc.DialTimeout = newGuardedClientDialerWithTimeout(hc.DialTimeout, hc.DialDualStack)
	}
	hc.Dial = newGuardedClientDialer(hc.Dial, hc.DialDualStack)
}

// guardedDial is the shared core of both dialer guards. When the active
// policy allows private IPs the caller has opted into internal targets, so
// it delegates unchanged. Otherwise it resolves and validates the host,
// then dials the exact validated IP via dialValidated — closing the
// DNS-rebinding check/use window the up-front validateUpstream lookup
// leaves open. The policy is read fresh on every dial because a shared
// client outlives any single policy snapshot.
//
//nolint:revive // dialDualStack mirrors fasthttp.Client.DialDualStack
func guardedDial(
	addr string,
	dialDualStack bool,
	delegate func(addr string) (net.Conn, error),
	dialValidated ssrfDialFunc,
) (net.Conn, error) {
	if currentSecurityPolicy().AllowPrivateIPs {
		return delegate(addr)
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("proxy: invalid dial address %q: %w", addr, err)
	}
	ips, err := resolveAndValidateHost(host)
	if err != nil {
		return nil, err
	}
	return dialValidatedIPs(ips, host, port, dialDualStack, dialValidated)
}

// newGuardedClientDialer wraps orig with the SSRF guard. Unlike
// newSSRFDialer — whose blocking is unconditional because Balancer only
// installs it when the policy forbids private IPs — this dialer is
// consulted by clients that outlive a single policy snapshot, so it
// re-reads the active global policy on every dial. When the caller supplied
// their own dialer it is run in both paths so custom transport is preserved
// while a blocked-policy connection still targets the validated address.
//
// When orig is nil the two paths differ, matching newSSRFDialer: the
// delegate (private-IPs-allowed) path hands the raw address to fasthttp's
// default dialer (Dial/DialDualStack), while the validated (blocked-policy)
// path connects to the already-checked IP with a net.Dialer bounded by
// dnsLookupTimeout — fasthttp.Dial is not reused there because the target is
// a literal IP that needs no re-resolution and the explicit timeout bounds
// the connect.
//
//nolint:revive // dialDualStack mirrors fasthttp.Client.DialDualStack
func newGuardedClientDialer(orig fasthttp.DialFunc, dialDualStack bool) fasthttp.DialFunc {
	dialer := &net.Dialer{Timeout: dnsLookupTimeout}
	delegate := func(addr string) (net.Conn, error) {
		if orig != nil {
			return orig(addr)
		}
		if dialDualStack {
			return fasthttp.DialDualStack(addr)
		}
		return fasthttp.Dial(addr)
	}
	dialValidated := func(_, address string) (net.Conn, error) {
		if orig != nil {
			return orig(address)
		}
		return dialer.Dial("tcp", address)
	}
	return func(addr string) (net.Conn, error) {
		return guardedDial(addr, dialDualStack, delegate, dialValidated)
	}
}

// newGuardedClientDialerWithTimeout is the DialFuncWithTimeout counterpart
// of newGuardedClientDialer. orig is always non-nil here (installed only
// when the HostClient already carries a DialTimeout), and the per-request
// timeout is threaded through both the delegate and the validated dial so
// the caller's timeout semantics survive the guard.
//
//nolint:revive // dialDualStack mirrors fasthttp.Client.DialDualStack
func newGuardedClientDialerWithTimeout(orig fasthttp.DialFuncWithTimeout, dialDualStack bool) fasthttp.DialFuncWithTimeout {
	return func(addr string, timeout time.Duration) (net.Conn, error) {
		delegate := func(a string) (net.Conn, error) {
			return orig(a, timeout)
		}
		dialValidated := func(_, address string) (net.Conn, error) {
			return orig(address, timeout)
		}
		return guardedDial(addr, dialDualStack, delegate, dialValidated)
	}
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
