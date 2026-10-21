package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"strings"
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

// dialFuncs holds the dialers a guard connects through: fasthttp's four, and the bounded net.Dialer.
type dialFuncs struct {
	dial                 fasthttp.DialFunc
	dialDualStack        fasthttp.DialFunc
	dialTimeout          fasthttp.DialFuncWithTimeout
	dialDualStackTimeout fasthttp.DialFuncWithTimeout
	netDialer            ssrfDialFunc
}

// fasthttpDialers is swapped only by tests, like dnsResolver.
var fasthttpDialers atomic.Pointer[dialFuncs]

func init() {
	dnsResolver.Store(net.DefaultResolver)
	fasthttpDialers.Store(&dialFuncs{
		dial:                 fasthttp.Dial,
		dialDualStack:        fasthttp.DialDualStack,
		dialTimeout:          fasthttp.DialTimeout,
		dialDualStackTimeout: fasthttp.DialDualStackTimeout,
		netDialer:            (&net.Dialer{Timeout: dnsLookupTimeout}).Dial,
	})
}

// validateHostForSSRF rejects hostnames that resolve to addresses inside
// the blocked ranges. The IP literal shortcut avoids DNS lookups when
// the host is already a numeric address. A non-nil cache may answer from a recent lookup.
func validateHostForSSRF(host string, cache *lookupCache) error {
	if host == "" {
		return ErrUpstreamHostInvalid
	}
	// strip brackets from IPv6 literals (url.Hostname already does this
	// in most cases, but keep the guard for defensive callers).
	host = trimBrackets(host)
	// Not via resolveAndValidateHost: this runs on every request and needs no slice back.
	if addr, literal := ipLiteral(host); literal {
		ip := addr.As16()
		if isBlockedIP(ip[:]) {
			// A string, not ip[:]: Errorf would move ip to the heap on every call.
			return fmt.Errorf("%w: %s", ErrUpstreamHostBlocked, net.IP(ip[:]).String())
		}
		return nil
	}
	addrs, err := lookupHost(dnsResolver.Load(), host, cache, time.Time{})
	if err != nil {
		return err
	}
	return checkResolved(host, addrs)
}

// ipLiteral returns the address host spells out, or false for a name or a zoned literal. It
// allocates nothing.
func ipLiteral(host string) (netip.Addr, bool) {
	if strings.IndexByte(host, ':') < 0 {
		return utils.ParseIPv4(host)
	}
	addr, ok := utils.ParseIPv6(host)
	return addr, ok && addr.Zone() == ""
}

// resolveAndValidateHost looks up host (or treats it as an IP literal),
// then enforces the SSRF blocklist on every returned address. A single
// blocked answer fails the whole resolution so a mixed public/private
// reply cannot slip past the guard. The dial guard calls it, so it never
// uses the cache, and the lookup ends at deadline.
func resolveAndValidateHost(host string, deadline time.Time) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return validateResolved(host, []net.IPAddr{{IP: ip}})
	}
	addrs, err := lookupHost(dnsResolver.Load(), host, nil, deadline)
	if err != nil {
		return nil, err
	}
	return validateResolved(host, addrs)
}

// lookupHost resolves host, through cache if given, within dnsLookupTimeout or an earlier deadline.
func lookupHost(resolver *net.Resolver, host string, cache *lookupCache, deadline time.Time) ([]net.IPAddr, error) {
	if cache != nil {
		if addrs, ok := cache.get(host, resolver); ok {
			return addrs, nil
		}
	}
	// Bound the lookup so a slow resolver cannot stall the caller.
	limit := time.Now().Add(dnsLookupTimeout)
	if !deadline.IsZero() && deadline.Before(limit) {
		limit = deadline
	}
	ctx, cancel := context.WithDeadline(context.Background(), limit)
	defer cancel()
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, &unresolvableError{host: host, cause: err}
	}
	if cache != nil && len(addrs) > 0 && len(addrs) <= lookupCacheMaxAddrs {
		cache.put(host, resolver, addrs)
	}
	return addrs, nil
}

// checkResolved applies the blocklist to every address host resolved to, and refuses an empty answer.
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

// unresolvableError is a lookup that failed or came back empty. It matches ErrUpstreamHostBlocked
// (the check fails closed) and ErrUpstreamHostUnresolvable, and wraps the cause.
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

// dialGuard is the SSRF guard in front of a client's dialer: it resolves the address, rejects it if
// any resolved IP is blocked, and only then dials, which defeats DNS rebinding.
type dialGuard struct {
	dial ssrfDialFunc
	// dialTimeout connects with the time the request leaves: defaultDialTimeout, built once.
	dialTimeout fasthttp.DialFuncWithTimeout

	// dualStack mirrors fasthttp's DialDualStack: IPv6 is dialed only when set.
	dualStack bool
}

// newDialGuard returns a guard that connects through the dialers fasthttpDialers holds: the
// bounded net.Dialer for an address the request gives no time, fasthttp's timed dialer otherwise.
func newDialGuard(dualStack bool) dialGuard {
	return dialGuard{dial: netDial, dialTimeout: defaultDialTimeout(dualStack), dualStack: dualStack}
}

// netDial connects through the bounded net.Dialer fasthttpDialers holds.
func netDial(network, address string) (net.Conn, error) {
	return fasthttpDialers.Load().netDialer(network, address)
}

// clientDialer wraps orig with the guard and re-reads the policy on every dial, for clients that
// outlive a policy snapshot. Private IPs allowed delegates to orig (fasthttp's when nil).
func (g dialGuard) clientDialer(orig fasthttp.DialFunc) fasthttp.DialFunc {
	checked := g
	delegate := func(addr string) (net.Conn, error) { return g.dialTimeout(addr, 0) }
	if orig != nil {
		delegate = orig
		checked.dial = func(_, address string) (net.Conn, error) { return orig(address) }
	}
	return func(addr string) (net.Conn, error) {
		if currentSecurityPolicy().AllowPrivateIPs {
			return delegate(addr)
		}
		return checked.dialChecked(addr)
	}
}

// clientDialerWithTimeout is clientDialer for a DialFuncWithTimeout, which gets the time the request
// leaves for the connect. A nil orig stands for a client with no dialer of its own.
func (g dialGuard) clientDialerWithTimeout(orig fasthttp.DialFuncWithTimeout) fasthttp.DialFuncWithTimeout {
	delegate, checked := orig, func(addr string, timeout time.Duration) (net.Conn, error) {
		return g.dialWithin(addr, timeout, orig)
	}
	if orig == nil {
		delegate, checked = g.dialTimeout, g.dialCheckedTimeout
	}
	return func(addr string, timeout time.Duration) (net.Conn, error) {
		if currentSecurityPolicy().AllowPrivateIPs {
			return delegate(addr, timeout)
		}
		return checked(addr, timeout)
	}
}

// defaultDialTimeout is fasthttp's dialer for the stack as a DialFuncWithTimeout: the timed one with
// the time given, the plain one with none, as DialTimeout would fail a zero at once.
func defaultDialTimeout(dualStack bool) fasthttp.DialFuncWithTimeout {
	return func(addr string, timeout time.Duration) (net.Conn, error) {
		d := fasthttpDialers.Load()
		switch {
		case timeout > 0 && dualStack:
			return d.dialDualStackTimeout(addr, timeout)
		case timeout > 0:
			return d.dialTimeout(addr, timeout)
		case dualStack:
			return d.dialDualStack(addr)
		default:
			return d.dial(addr)
		}
	}
}

// dialChecked dials addr unless an IP it resolves to is blocked.
func (g dialGuard) dialChecked(addr string) (net.Conn, error) {
	return g.dialBy(addr, time.Time{})
}

// dialCheckedTimeout is dialChecked with timeout bounding the lookup and the connects together.
// Zero, which fasthttp hands over when the request leaves no time, is dialChecked itself.
func (g dialGuard) dialCheckedTimeout(addr string, timeout time.Duration) (net.Conn, error) {
	if timeout <= 0 {
		return g.dialChecked(addr)
	}
	return g.dialWithin(addr, timeout, g.dialTimeout)
}

// dialWithin is dialChecked through dial, with timeout shared by the lookup and the connects, as
// fasthttp's own dialers share one deadline over a name's addresses.
func (g dialGuard) dialWithin(addr string, timeout time.Duration, dial fasthttp.DialFuncWithTimeout) (net.Conn, error) {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	checked := g
	checked.dial = func(_, address string) (net.Conn, error) {
		if deadline.IsZero() {
			return dial(address, 0)
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fasthttp.ErrDialTimeout
		}
		return dial(address, remaining)
	}
	return checked.dialBy(addr, deadline)
}

// dialBy dials addr unless an IP it resolves to is blocked, giving up the lookup by deadline.
func (g dialGuard) dialBy(addr string, deadline time.Time) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("proxy: invalid dial address %q: %w", addr, err)
	}
	ips, err := resolveAndValidateHost(host, deadline)
	if err != nil {
		return nil, err
	}
	return g.dialValidatedIPs(ips, host, port)
}

// errIPv6Only is why a host that passed the blocklist still has nothing to dial.
var errIPv6Only = errors.New("only IPv6 addresses, and DialDualStack is off")

// dialValidatedIPs dials the first non-skipped address, skipping IPv6 unless the guard is
// dual-stack, as fasthttp does. If every candidate is skipped the error says so (nothing was
// blocked); otherwise it is the last dial error.
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
		lastErr = fmt.Errorf("proxy: upstream host %s has %w", host, errIPv6Only)
	}
	return nil, lastErr
}

// installHostClientGuard fits hc with the policy-aware dial-time guard, once per HostClient through
// ConfigureClient. fasthttp dials through DialTimeout when set and Dial otherwise, so both are
// guarded. A client with no dialer of its own gets the default as a DialTimeout, the only one handed
// the time a request leaves for the connect; a Dial of its own is kept, or a DialTimeout would win.
func installHostClientGuard(hc *fasthttp.HostClient) {
	g := newDialGuard(hc.DialDualStack)
	switch {
	case hc.DialTimeout != nil:
		hc.DialTimeout = g.clientDialerWithTimeout(hc.DialTimeout)
	case hc.Dial == nil:
		hc.DialTimeout = g.clientDialerWithTimeout(nil)
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
