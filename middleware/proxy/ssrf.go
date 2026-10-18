package proxy

import (
	"context"
	"errors"
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
	host = trimBrackets(host)
	// Not via resolveAndValidateHost: this runs on every request and needs no slice back. The
	// check ahead of ParseIP spares a hostname the allocations of a parse that fails.
	if utils.IsIPv4(host) || utils.IsIPv6(host) {
		if ip := net.ParseIP(host); isBlockedIP(ip) {
			return fmt.Errorf("%w: %s", ErrUpstreamHostBlocked, ip)
		}
		return nil
	}
	addrs, err := lookupHost(host, upfrontLookups)
	if err != nil {
		return err
	}
	return checkResolved(host, addrs)
}

// resolveAndValidateHost looks up host (or treats it as an IP literal),
// then enforces the SSRF blocklist on every returned address. A single
// blocked answer fails the whole resolution so a mixed public/private
// reply cannot slip past the guard. The dial guard calls it, so it always
// looks up afresh.
func resolveAndValidateHost(host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return validateResolved(host, []net.IPAddr{{IP: ip}})
	}
	addrs, err := lookupHost(host, nil)
	if err != nil {
		return nil, err
	}
	return validateResolved(host, addrs)
}

// lookupHost resolves host, answering from cache when it holds a fresh answer and storing successes there.
func lookupHost(host string, cache *lookupCache) ([]net.IPAddr, error) {
	resolver := dnsResolver.Load()
	if cache != nil {
		if addrs, ok := cache.get(host, resolver); ok {
			return addrs, nil
		}
	}
	// Bound the lookup so a slow resolver cannot stall the caller.
	ctx, cancel := context.WithTimeout(context.Background(), dnsLookupTimeout)
	defer cancel()
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, &unresolvableError{host: host, cause: err}
	}
	if cache != nil && len(addrs) > 0 {
		cache.put(host, resolver, addrs)
	}
	return addrs, nil
}

// checkResolved applies the blocklist to every address host resolved to. An
// empty answer is refused: it leaves nothing to dial.
func checkResolved(host string, addrs []net.IPAddr) error {
	if len(addrs) == 0 {
		return &unresolvableError{host: host, cause: errNoAddresses}
	}
	for _, addr := range addrs {
		if isBlockedIP(addr.IP) {
			return fmt.Errorf("%w: %s -> %s", ErrUpstreamHostBlocked, host, addr.IP)
		}
	}
	return nil
}

// validateResolved is checkResolved that also returns the addresses.
func validateResolved(host string, addrs []net.IPAddr) ([]net.IP, error) {
	if err := checkResolved(host, addrs); err != nil {
		return nil, err
	}
	ips := make([]net.IP, len(addrs))
	for i, addr := range addrs {
		ips[i] = addr.IP
	}
	return ips, nil
}

var errNoAddresses = errors.New("no addresses")

// unresolvableError is the error for a host whose lookup failed or came back empty. It
// matches ErrUpstreamHostBlocked (the check fails closed) and ErrUpstreamHostUnresolvable
// (no address was judged), and wraps the lookup's own error.
type unresolvableError struct {
	cause error
	host  string
}

func (e *unresolvableError) Error() string {
	return "proxy: upstream host " + e.host + " could not be resolved: " + e.cause.Error()
}

func (e *unresolvableError) Unwrap() []error {
	return []error{ErrUpstreamHostBlocked, ErrUpstreamHostUnresolvable, e.cause}
}

// ssrfDialFunc is the (network, address) -> conn signature consumed by
// dialGuard. It accepts the standard library *net.Dialer's Dial method
// directly, and lets tests inject a deterministic stub.
type ssrfDialFunc func(network, address string) (net.Conn, error)

// dialGuard is the SSRF guard in front of a client's dialer: it resolves the address, rejects it
// if any resolved IP is blocked, and only then dials. Checking at dial time, not only up front,
// defeats DNS rebinding: a resolver that answers public during validation and private at connect.
type dialGuard struct {
	// dial connects to a validated address.
	dial ssrfDialFunc

	// dualStack mirrors fasthttp's DialDualStack: IPv6 is dialed only when set.
	dualStack bool
}

// newDialGuard returns a guard whose validated dials are bounded by dnsLookupTimeout.
func newDialGuard(dualStack bool) dialGuard {
	dialer := &net.Dialer{Timeout: dnsLookupTimeout}
	return dialGuard{dial: dialer.Dial, dualStack: dualStack}
}

// clientDialer wraps orig with the guard. Unlike dialChecked, which always applies the blocklist
// and which Balancer installs only when its policy forbids private IPs, it serves clients that
// outlive any one policy snapshot, so it re-reads the active policy on every dial. A caller's own dialer is
// used on both paths; without one, the policy-allows path hands the raw address to fasthttp's
// default dialer, and the blocking path dials the checked IP with g.dial under an explicit timeout.
func (g dialGuard) clientDialer(orig fasthttp.DialFunc) fasthttp.DialFunc {
	delegate := func(addr string, _ time.Duration) (net.Conn, error) {
		switch {
		case orig != nil:
			return orig(addr)
		case g.dualStack:
			return fasthttp.DialDualStack(addr)
		default:
			return fasthttp.Dial(addr)
		}
	}
	validated := delegate
	if orig == nil {
		validated = func(address string, _ time.Duration) (net.Conn, error) {
			return g.dial("tcp", address)
		}
	}
	dial := g.guarded(delegate, validated)
	return func(addr string) (net.Conn, error) {
		return dial(addr, 0)
	}
}

// clientDialerWithTimeout is clientDialer for a DialFuncWithTimeout. orig is non-nil here (it is
// installed only when the HostClient already has a DialTimeout) and gets the caller's timeout.
func (g dialGuard) clientDialerWithTimeout(orig fasthttp.DialFuncWithTimeout) fasthttp.DialFuncWithTimeout {
	return g.guarded(orig, orig)
}

// guarded is the core both client dialers share. When the active policy allows private IPs,
// delegate runs unchanged; otherwise the address is resolved and checked and validated dials
// the IP that passed, closing the rebinding window the up-front lookup leaves open.
func (g dialGuard) guarded(delegate, validated fasthttp.DialFuncWithTimeout) fasthttp.DialFuncWithTimeout {
	return func(addr string, timeout time.Duration) (net.Conn, error) {
		if currentSecurityPolicy().AllowPrivateIPs {
			return delegate(addr, timeout)
		}
		checked := g
		checked.dial = func(_, address string) (net.Conn, error) {
			return validated(address, timeout)
		}
		return checked.dialChecked(addr)
	}
}

// dialChecked dials addr unless an IP it resolves to is blocked.
func (g dialGuard) dialChecked(addr string) (net.Conn, error) {
	host, port, ok := utils.SplitHostPort(addr)
	if !ok {
		return nil, fmt.Errorf("proxy: invalid dial address %q", addr)
	}
	ips, err := resolveAndValidateHost(host)
	if err != nil {
		return nil, err
	}
	return g.dialValidatedIPs(ips, host, port)
}

// dialValidatedIPs walks ips and tries to dial the first non-skipped
// address. Unless the guard is dual-stack, IPv6 addresses are skipped to
// match fasthttp's IPv4-only default. If every candidate is skipped, a
// "no usable address" error is returned; otherwise the last dial error
// is propagated so the caller can see why each attempt failed.
func (g dialGuard) dialValidatedIPs(ips []net.IP, host, port string) (net.Conn, error) {
	var lastErr error
	for _, ip := range ips {
		// Mirror fasthttp's default of IPv4-only unless DialDualStack.
		if !g.dualStack && ip.To4() == nil {
			continue
		}
		conn, derr := g.dial("tcp", net.JoinHostPort(ip.String(), port))
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
