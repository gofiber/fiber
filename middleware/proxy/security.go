package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// dnsLookupTimeout bounds DNS resolution performed during upstream
// validation and dialing, so a slow or unresponsive resolver cannot hang
// a request (or application startup) indefinitely.
const dnsLookupTimeout = 5 * time.Second

// Supported upstream schemes. Defined as constants so the literals don't
// trip the goconst linter when referenced from policy defaults, scheme
// comparisons, and redirect downgrade checks.
const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// defaultAllowedSchemes is the internal, read-only allowlist used as the
// fallback inside schemeAllowed when a policy carries no AllowedSchemes.
// It is never handed out by reference: DefaultSecurityPolicy() and
// normalizePolicy() copy it before it can reach the exported
// SecurityPolicy.AllowedSchemes field, so nothing outside this file can
// mutate the backing array.
var defaultAllowedSchemes = []string{schemeHTTP, schemeHTTPS}

// httpsSchemeBytes is the byte form of "https" used by redirect
// downgrade checks. Stored once so the resolveRedirect hot path doesn't
// allocate []byte("https") on every hop.
var httpsSchemeBytes = []byte(schemeHTTPS)

// Sentinel errors returned when an upstream target violates the configured
// proxy security policy.
var (
	// ErrUpstreamSchemeNotAllowed is returned when the proxied URL uses a
	// scheme outside the configured allowlist (default: http, https).
	ErrUpstreamSchemeNotAllowed = errors.New("proxy: upstream scheme is not allowed")

	// ErrUpstreamHostInvalid is returned when the proxied URL is missing a
	// host or cannot be parsed.
	ErrUpstreamHostInvalid = errors.New("proxy: upstream host is empty or invalid")

	// ErrUpstreamHostBlocked is returned when the proxied URL resolves to
	// an address inside a blocked range (loopback, RFC 1918 private,
	// link-local, multicast, unspecified, or CGNAT) and AllowPrivateIPs
	// is false.
	ErrUpstreamHostBlocked = errors.New("proxy: upstream host resolves to a blocked address")

	// ErrRedirectDowngrade is returned when DoRedirects encounters a
	// redirect from an HTTPS upstream to a plaintext HTTP target and
	// AllowHTTPSDowngrade is false.
	ErrRedirectDowngrade = errors.New("proxy: HTTPS to HTTP redirect blocked")
)

// SecurityPolicy controls runtime security restrictions applied to the
// proxy.Do, proxy.Forward, proxy.DoRedirects, proxy.DoTimeout, and
// proxy.DoDeadline runtime helpers as well as Balancer instances that
// do not supply their own policy via Config.SecurityPolicy.
type SecurityPolicy struct {
	// AllowedSchemes restricts the URL schemes accepted as upstream
	// targets. Empty defaults to []string{schemeHTTP, schemeHTTPS}.
	AllowedSchemes []string

	// AllowPrivateIPs allows upstream hosts to resolve to loopback,
	// private (RFC 1918), link-local, multicast, unspecified, or CGNAT
	// (RFC 6598) addresses. SECURITY: enabling this exposes the proxy
	// to SSRF attacks against internal services such as cloud
	// metadata endpoints. Default: false.
	//
	// DNS-rebinding scope: when false, the resolved IP is re-validated at
	// dial time, not only up front, so a malicious resolver cannot swap a
	// public answer (seen during validation) for a private one at connect
	// time. Balancer installs a guarded Dial on each HostClient it builds;
	// the runtime helpers — Do, DoRedirects, DoTimeout, DoDeadline,
	// Forward, DomainForward, BalancerForward — install the same guard on
	// the shared/user-supplied *fasthttp.Client they dispatch through.
	//
	// The default client and clients registered via WithClient are guarded
	// before first use and are fully protected. A client passed as a
	// per-call variadic argument is guarded on first use, so any host it had
	// already dialed keeps a cached, pre-guard HostClient; register such a
	// client via WithClient (or hand the proxy a dedicated, unused client)
	// for the full guarantee. The only path with no guard at all is a custom
	// Balancer Config.Client (*fasthttp.LBClient): its underlying dialers
	// are the caller's responsibility.
	AllowPrivateIPs bool

	// AllowHTTPSDowngrade permits proxy.DoRedirects to follow redirects
	// from HTTPS upstreams to plaintext HTTP URLs. SECURITY: enabling
	// this can leak credentials or session cookies in plaintext.
	// Default: false.
	AllowHTTPSDowngrade bool

	// KeepHopByHopHeaders disables the RFC 7230 §6.1 hop-by-hop header
	// stripping applied to both the outbound request and the inbound
	// response. SECURITY: enabling this can enable request smuggling
	// and proxy-auth credential forwarding. Default: false.
	KeepHopByHopHeaders bool
}

// DefaultSecurityPolicy returns the secure-by-default proxy security
// policy. Callers can clone it, mutate selected fields, and pass it back
// via Config.SecurityPolicy or WithSecurityPolicy.
//
// AllowedSchemes is a freshly allocated slice on every call: the field
// is exported, so returning the shared defaultAllowedSchemes backing
// array would let a caller doing e.g. policy.AllowedSchemes[0] = "file"
// silently weaken the package defaults and every policy that references
// them.
func DefaultSecurityPolicy() SecurityPolicy {
	return SecurityPolicy{
		AllowedSchemes:      append([]string(nil), defaultAllowedSchemes...),
		AllowPrivateIPs:     false,
		AllowHTTPSDowngrade: false,
		KeepHopByHopHeaders: false,
	}
}

// activePolicy is the global SecurityPolicy consulted by every
// non-Balancer proxy request. Reads are lock-free via atomic.Pointer.
// The pointed-to SecurityPolicy is treated as immutable once installed:
// WithSecurityPolicy is the only writer, and it stores a value whose
// AllowedSchemes slice has been deep-copied to defend against later
// mutation by the caller.
var activePolicy atomic.Pointer[SecurityPolicy]

// dnsResolver is the resolver used for SSRF validation lookups. It defaults
// to net.DefaultResolver and is only ever swapped by tests, atomically, so a
// concurrent validation lookup never races the swap. Tests use this seam
// rather than mutating the process-global net.DefaultResolver — which
// fasthttp and the net package read from background dial goroutines, making a
// direct swap a data race under -race.
var dnsResolver atomic.Pointer[net.Resolver]

func init() {
	def := DefaultSecurityPolicy()
	activePolicy.Store(&def)
	dnsResolver.Store(net.DefaultResolver)
}

// normalizePolicy returns a copy of policy whose AllowedSchemes slice is
// always freshly allocated. This guarantees callers cannot mutate the
// global allowlist out from under a balancer (or another goroutine) by
// retaining a reference to the slice they passed in — including the
// package-level defaultAllowedSchemes, which must never be handed out by
// reference. The defensive copy happens at install time only — readers
// consume the immutable result via activePolicy.Load() without further
// copying.
func normalizePolicy(policy SecurityPolicy) SecurityPolicy {
	if len(policy.AllowedSchemes) == 0 {
		policy.AllowedSchemes = append([]string(nil), defaultAllowedSchemes...)
	} else {
		policy.AllowedSchemes = append([]string(nil), policy.AllowedSchemes...)
	}
	return policy
}

// WithSecurityPolicy installs policy as the global default consulted by
// proxy.Do, proxy.Forward, proxy.DoRedirects, proxy.DoTimeout, and
// proxy.DoDeadline (and by Balancer instances that do not set
// Config.SecurityPolicy). It returns the previously installed policy so
// callers can restore it — useful in tests that need to relax
// the policy for a single scope.
func WithSecurityPolicy(policy SecurityPolicy) SecurityPolicy {
	normalized := normalizePolicy(policy)
	prev := activePolicy.Swap(&normalized)
	if prev == nil {
		return DefaultSecurityPolicy()
	}
	return *prev
}

// currentSecurityPolicy returns a snapshot of the active global policy.
// The snapshot is by-value, but its AllowedSchemes slice header aliases
// the immutable backing array installed by WithSecurityPolicy — no copy
// is taken on the read path.
func currentSecurityPolicy() SecurityPolicy {
	return *activePolicy.Load()
}

// resolvePolicy returns override when non-nil; otherwise the current
// global policy. Both Balancer and the runtime helpers funnel through
// this so a single source of truth governs upstream validation and
// header stripping. Override paths still copy AllowedSchemes so a
// caller passing Config.SecurityPolicy by reference cannot mutate the
// Balancer's view after construction.
func resolvePolicy(override *SecurityPolicy) SecurityPolicy {
	if override != nil {
		return normalizePolicy(*override)
	}
	return currentSecurityPolicy()
}

// hopByHopHeaders are the RFC 7230 §6.1 connection-level headers that
// intermediaries must not forward. Stripping these prevents request
// smuggling (Transfer-Encoding/TE), proxy-credential forwarding
// (Proxy-Authorization), and protocol-upgrade leaks.
var hopByHopHeaders = []string{
	fiber.HeaderConnection,
	fiber.HeaderKeepAlive,
	fiber.HeaderProxyAuthenticate,
	fiber.HeaderProxyAuthorization,
	fiber.HeaderTE,
	fiber.HeaderTrailer,
	fiber.HeaderTransferEncoding,
	fiber.HeaderUpgrade,
}

// stripHopByHopRequestHeaders removes RFC 7230 §6.1 hop-by-hop headers
// from req. Callers can pass header names in except to preserve specific
// headers — used by the legacy KeepConnectionHeader option to retain the
// literal Connection header while still dropping the other hop-by-hop
// headers.
func stripHopByHopRequestHeaders(req *fasthttp.Request, except ...string) {
	// Headers listed in Connection must be removed first so the
	// listing is honored before the Connection field itself is dropped.
	for _, name := range connectionListedHeaders(req.Header.PeekAll(fiber.HeaderConnection)) {
		req.Header.Del(name)
	}
	for _, h := range hopByHopHeaders {
		if containsFold(except, h) {
			continue
		}
		req.Header.Del(h)
	}
}

// stripHopByHopResponseHeaders applies the same filtering on the way
// back, so upstream cannot leak connection-scoped state to the client.
func stripHopByHopResponseHeaders(res *fasthttp.Response, except ...string) {
	for _, name := range connectionListedHeaders(res.Header.PeekAll(fiber.HeaderConnection)) {
		res.Header.Del(name)
	}
	for _, h := range hopByHopHeaders {
		if containsFold(except, h) {
			continue
		}
		res.Header.Del(h)
	}
}

func containsFold(haystack []string, needle string) bool {
	for _, h := range haystack {
		if utils.EqualFold(h, needle) {
			return true
		}
	}
	return false
}

// connectionListedHeaders returns the comma-separated header names
// listed inside one or more Connection header fields, per RFC 7230 §6.1.
func connectionListedHeaders(values [][]byte) []string {
	if len(values) == 0 {
		return nil
	}
	var out []string
	for _, v := range values {
		for name := range strings.SplitSeq(string(v), ",") {
			name = utils.TrimSpace(name)
			if name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

// parseUpstream returns the parsed url.URL for raw. Hosts without an
// explicit scheme default to http:// to match the historical Balancer
// behavior where bare "host:port" entries were accepted.
func parseUpstream(raw string) (*url.URL, error) {
	raw = utils.TrimSpace(raw)
	if raw == "" {
		return nil, ErrUpstreamHostInvalid
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("proxy: parse upstream %q: %w", raw, err)
	}
	return u, nil
}

// validateUpstream parses raw, enforces the scheme allowlist, and unless
// the policy permits private addresses, resolves the hostname and
// rejects responses that include any blocked address. Rejecting on a
// single blocked answer mitigates DNS rebinding attempts in which the
// resolver returns a mix of public and private IPs.
func validateUpstream(raw string, policy SecurityPolicy) (*url.URL, error) {
	u, err := parseUpstreamScheme(raw, policy)
	if err != nil {
		return nil, err
	}
	if policy.AllowPrivateIPs {
		return u, nil
	}
	if err := validateHostForSSRF(u.Hostname()); err != nil {
		return nil, err
	}
	return u, nil
}

// parseUpstreamScheme parses raw and enforces the scheme allowlist and
// host presence without performing any DNS resolution. url.Parse can
// leave Host set while Hostname() is empty (e.g. "http://:8080"), so the
// presence check uses Hostname.
func parseUpstreamScheme(raw string, policy SecurityPolicy) (*url.URL, error) {
	u, err := parseUpstream(raw)
	if err != nil {
		return nil, err
	}
	if !schemeAllowed(u.Scheme, policy.AllowedSchemes) {
		return nil, fmt.Errorf("%w: %q", ErrUpstreamSchemeNotAllowed, u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, ErrUpstreamHostInvalid
	}
	return u, nil
}

// validateUpstreamForBalancer validates a statically configured Balancer
// upstream. It enforces the scheme allowlist and rejects IP-literal hosts
// in blocked ranges, but defers hostname resolution to the SSRF-guarded
// dialer (see newSSRFDialer). Deferring DNS keeps a transient resolver
// failure at startup from panicking the application (e.g. crash loops in
// container orchestrators) and re-checks the resolved IP on every dial,
// which also defeats DNS-rebinding.
func validateUpstreamForBalancer(raw string, policy SecurityPolicy) (*url.URL, error) {
	u, err := parseUpstreamScheme(raw, policy)
	if err != nil {
		return nil, err
	}
	if policy.AllowPrivateIPs {
		return u, nil
	}
	if ip := net.ParseIP(trimBrackets(u.Hostname())); ip != nil && isBlockedIP(ip) {
		return nil, fmt.Errorf("%w: %s", ErrUpstreamHostBlocked, ip)
	}
	return u, nil
}

// schemeAllowed reports whether scheme is on the allowlist. An empty
// allowlist falls back to the secure defaults.
func schemeAllowed(scheme string, allowed []string) bool {
	if scheme == "" {
		return false
	}
	if len(allowed) == 0 {
		allowed = defaultAllowedSchemes
	}
	for _, s := range allowed {
		if utils.EqualFold(s, scheme) {
			return true
		}
	}
	return false
}

// trimBrackets removes the surrounding "[" / "]" from an IPv6 literal
// (defensive — url.Hostname() typically does this already). The fiber
// utils Trim helpers only accept a single byte cutset, so we apply
// TrimLeft + TrimRight to handle the bracket pair.
func trimBrackets(host string) string {
	return utils.TrimRight(utils.TrimLeft(host, '['), ']')
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

// isBlockedIP reports whether ip falls inside a range that proxy
// upstreams must not reach by default. Loopback, unspecified, RFC 1918
// private, link-local (including the 169.254.169.254 cloud-metadata
// address), multicast, and RFC 6598 CGNAT ranges are blocked. IPv4-compatible,
// NAT64, 6to4, and Teredo IPv6 wrappers are unwrapped so a blocked IPv4 cannot
// be smuggled past as an IPv6 literal.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	// Look through IPv4-compatible (::a.b.c.d), NAT64 (64:ff9b::a.b.c.d and
	// the 64:ff9b:1::/48 local-use prefix), 6to4 (2002::/16), and Teredo
	// (2001:0000::/32) IPv6 wrappers to the embedded IPv4 and apply the same
	// blocklist. Some stacks route these to the embedded address, so a private
	// target could otherwise be reached via such a literal (the IPv4-mapped
	// ::ffff:a.b.c.d form is already handled above because net.IP.To4 exposes
	// it).
	if embedded := embeddedIPv4(ip); embedded != nil && isBlockedIP(embedded) {
		return true
	}
	return false
}

// embeddedIPv4 returns the IPv4 address embedded in an IPv6 transition
// address, or nil for anything else. Recognized forms are the IPv4-compatible
// address (::a.b.c.d, RFC 4291), the NAT64 well-known prefix
// (64:ff9b::a.b.c.d, RFC 6052) and its local-use counterpart (64:ff9b:1::/48,
// RFC 8215), 6to4 (2002::/16, RFC 3056), and Teredo (2001:0000::/32,
// RFC 4380). Each embeds an IPv4 address inside a globally-routable IPv6
// literal, so unwrapping them lets isBlockedIP apply the IPv4 blocklist and
// stops a private target from being smuggled past the guard. The IPv4-mapped
// form (::ffff:a.b.c.d) is deliberately excluded here: net.IP.To4 already
// surfaces its IPv4, so isBlockedIP checks it directly.
func embeddedIPv4(ip net.IP) net.IP {
	ip16 := ip.To16()
	if ip16 == nil || ip.To4() != nil {
		return nil
	}
	// NAT64 well-known prefix 64:ff9b::/96 (RFC 6052).
	if ip16[0] == 0x00 && ip16[1] == 0x64 && ip16[2] == 0xff && ip16[3] == 0x9b &&
		allZero(ip16[4:12]) {
		return net.IPv4(ip16[12], ip16[13], ip16[14], ip16[15])
	}
	// NAT64 local-use prefix 64:ff9b:1::/48 (RFC 8215): the trailing 32 bits
	// carry the embedded IPv4 address.
	if ip16[0] == 0x00 && ip16[1] == 0x64 && ip16[2] == 0xff && ip16[3] == 0x9b &&
		ip16[4] == 0x00 && ip16[5] == 0x01 {
		return net.IPv4(ip16[12], ip16[13], ip16[14], ip16[15])
	}
	// 6to4 2002::/16 (RFC 3056): the IPv4 gateway sits in bytes [2:6].
	if ip16[0] == 0x20 && ip16[1] == 0x02 {
		return net.IPv4(ip16[2], ip16[3], ip16[4], ip16[5])
	}
	// Teredo 2001:0000::/32 (RFC 4380): the client IPv4 is stored in the
	// trailing 32 bits, obfuscated by a bitwise NOT.
	if ip16[0] == 0x20 && ip16[1] == 0x01 && ip16[2] == 0x00 && ip16[3] == 0x00 {
		return net.IPv4(ip16[12]^0xff, ip16[13]^0xff, ip16[14]^0xff, ip16[15]^0xff)
	}
	// IPv4-compatible ::a.b.c.d (first 12 bytes zero). :: and ::1 have already
	// been classified above, so treat any remaining all-zero-prefix address as
	// a wrapper around its trailing IPv4.
	if allZero(ip16[:12]) {
		return net.IPv4(ip16[12], ip16[13], ip16[14], ip16[15])
	}
	return nil
}

func allZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

// secureTLSConfig returns a clone of cfg with a TLS 1.2 floor. The
// caller's tls.Config is never mutated so per-host clients can share an
// immutable template without surprise.
func secureTLSConfig(cfg *tls.Config) *tls.Config {
	if cfg == nil {
		return &tls.Config{MinVersion: tls.VersionTLS12}
	}
	cloned := cfg.Clone()
	if cloned.MinVersion == 0 {
		cloned.MinVersion = tls.VersionTLS12
	}
	return cloned
}

// joinUpstreamPath returns a URL string formed by combining an already
// validated upstream base with a request path supplied by the client.
// The request path's authority component (if any) is discarded so
// crafted inputs like "//attacker.example/foo" or "@attacker" cannot
// change the host the proxy connects to.
func joinUpstreamPath(base *url.URL, requestPath string) string {
	if base == nil {
		return ""
	}
	// Fast path: the common production case is a base with no path
	// prefix and a request path that's already in clean origin form. In
	// that case the slow path's url.Parse + URL.String() round-trip
	// produces a string identical to base.Scheme + "://" + base.Host +
	// requestPath, so build the result directly with one allocation.
	// canFastJoinPath rejects any input that could collide with the
	// security-relevant patterns the slow path handles
	// (//-authority injection, @-authority injection, :-scheme
	// injection, control bytes). Anything it rejects falls through to
	// the slow path; the FuzzJoinUpstreamPath suite guards the
	// boundary.
	if baseIsPlainOrigin(base) && canFastJoinPath(requestPath) {
		var sb strings.Builder
		sb.Grow(len(base.Scheme) + len("://") + len(base.Host) + len(requestPath))
		// strings.Builder.WriteString always returns nil per the
		// documented contract; the errors are unused on purpose.
		sb.WriteString(base.Scheme) //nolint:errcheck // strings.Builder cannot fail
		sb.WriteString("://")       //nolint:errcheck // strings.Builder cannot fail
		sb.WriteString(base.Host)   //nolint:errcheck // strings.Builder cannot fail
		sb.WriteString(requestPath) //nolint:errcheck // strings.Builder cannot fail
		return sb.String()
	}
	out := *base
	if requestPath == "" {
		return out.String()
	}
	// A leading "//" makes Go's url.Parse treat the value as a
	// network-path reference and parse a new authority. Collapse it to
	// a single slash so the host stays pinned to the configured base.
	for strings.HasPrefix(requestPath, "//") {
		requestPath = "/" + utils.TrimLeft(requestPath, '/')
	}
	if requestPath[0] != '/' && requestPath[0] != '?' && requestPath[0] != '#' {
		requestPath = "/" + requestPath
	}
	parsed, err := url.Parse(requestPath)
	if err != nil || parsed.Host != "" || parsed.Scheme != "" {
		// Either the path failed to parse cleanly or it introduced a
		// new authority. Treat the remainder as an opaque path, but
		// preserve any path prefix configured on the upstream base so
		// a malformed request can't silently bypass it (e.g.
		// "http://upstream/api" + "/%zz" must stay rooted at "/api").
		fallback := "/" + utils.TrimLeft(requestPath, '/')
		if base.Path != "" {
			fallback = strings.TrimSuffix(base.Path, "/") + "/" + strings.TrimPrefix(fallback, "/")
		}
		out.Path = fallback
		out.RawPath = ""
		out.RawQuery = ""
		out.Fragment = ""
		return out.String()
	}
	if parsed.Path != "" {
		// Preserve any path prefix configured on the upstream base (e.g.
		// "http://upstream/api" + "/foo" -> "http://upstream/api/foo")
		// rather than overwriting it with the request path alone.
		out.Path = strings.TrimSuffix(base.Path, "/") + "/" + strings.TrimPrefix(parsed.Path, "/")
		if base.RawPath != "" || parsed.RawPath != "" {
			baseRaw := base.RawPath
			if baseRaw == "" {
				baseRaw = base.Path
			}
			parsedRaw := parsed.RawPath
			if parsedRaw == "" {
				parsedRaw = parsed.Path
			}
			out.RawPath = strings.TrimSuffix(baseRaw, "/") + "/" + strings.TrimPrefix(parsedRaw, "/")
		}
	}
	out.RawQuery = parsed.RawQuery
	out.Fragment = parsed.Fragment
	return out.String()
}

// baseIsPlainOrigin reports whether base carries only a scheme + host
// — no path prefix, userinfo, query, fragment, or opaque component.
// Any of those forces the slow path because the splice cannot
// reproduce them with a simple concatenation.
func baseIsPlainOrigin(base *url.URL) bool {
	return base.Path == "" &&
		base.RawPath == "" &&
		base.User == nil &&
		base.RawQuery == "" &&
		!base.ForceQuery &&
		base.Fragment == "" &&
		base.Opaque == ""
}

// canFastJoinPath reports whether requestPath can be spliced onto an
// upstream base without going through url.Parse + URL.String.
//
// Allowed bytes match what url.URL.String() would emit unescaped:
//   - Alphanumeric and unreserved (-._~).
//   - '/' (never doubled in the path portion).
//   - '?' '#' as single separators between path / query / fragment.
//   - '%' only when followed by two valid hex digits.
//   - In the query and fragment portion: common sub-delims
//     (&=+,;!$*()'). Authority-injection markers ('@' ':') and any
//     other byte are rejected so the slow path can apply Go's stdlib
//     URL escaping rules.
//
// The FuzzJoinUpstreamPath suite covers each rejection boundary.
func canFastJoinPath(requestPath string) bool {
	if requestPath == "" || requestPath[0] != '/' {
		return false
	}
	inPath := true
	var prev byte
	for i := 0; i < len(requestPath); i++ {
		c := requestPath[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			prev = c
			continue
		}
		switch c {
		case '-', '.', '_', '~':
			// Unreserved — always safe.
		case '/':
			if inPath && prev == '/' {
				return false
			}
		case '?', '#':
			if !inPath {
				// Repeated path/query separator — let slow path handle.
				return false
			}
			if i == len(requestPath)-1 {
				// Empty query/fragment markers are not represented in the
				// slow path's output, so fall back to preserve behavior.
				return false
			}
			inPath = false
		case '%':
			if i+2 >= len(requestPath) || !isHexDigit(requestPath[i+1]) || !isHexDigit(requestPath[i+2]) {
				return false
			}
		case '&', '=', '+', ',', ';', '!', '$', '*', '(', ')', '\'':
			// Sub-delims — only safe inside query/fragment.
			if inPath {
				return false
			}
		default:
			// Everything else falls back to the slow path: authority /
			// scheme separators ('@', ':') we always refuse, plus any
			// byte (spaces, control bytes, brackets, quotes, multi-byte
			// UTF-8, etc.) that url.URL.String() would percent-encode.
			return false
		}
		prev = c
	}
	return true
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
