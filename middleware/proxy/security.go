package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// dnsLookupTimeout bounds DNS resolution performed during upstream
// validation and dialing, so a slow or unresponsive resolver cannot hang
// a request (or application startup) indefinitely.
const dnsLookupTimeout = 5 * time.Second

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
	// targets. Empty defaults to []string{"http", "https"}.
	AllowedSchemes []string

	// AllowPrivateIPs allows upstream hosts to resolve to loopback,
	// private (RFC 1918), link-local, multicast, unspecified, or CGNAT
	// (RFC 6598) addresses. SECURITY: enabling this exposes the proxy
	// to SSRF attacks against internal services such as cloud
	// metadata endpoints. Default: false.
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
func DefaultSecurityPolicy() SecurityPolicy {
	return SecurityPolicy{
		AllowedSchemes:      []string{"http", "https"},
		AllowPrivateIPs:     false,
		AllowHTTPSDowngrade: false,
		KeepHopByHopHeaders: false,
	}
}

var (
	policyLock   sync.RWMutex
	activePolicy = DefaultSecurityPolicy()
)

// normalizePolicy returns a copy of policy whose AllowedSchemes slice is
// always freshly allocated. This guarantees callers cannot mutate the
// global allowlist out from under a balancer (or another goroutine) by
// retaining a reference to the slice they passed in.
func normalizePolicy(policy SecurityPolicy) SecurityPolicy {
	if len(policy.AllowedSchemes) == 0 {
		policy.AllowedSchemes = []string{"http", "https"}
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
	policy = normalizePolicy(policy)
	policyLock.Lock()
	defer policyLock.Unlock()
	prev := activePolicy
	activePolicy = policy
	return normalizePolicy(prev)
}

// currentSecurityPolicy returns a snapshot of the active global policy
// with a private copy of AllowedSchemes.
func currentSecurityPolicy() SecurityPolicy {
	policyLock.RLock()
	defer policyLock.RUnlock()
	return normalizePolicy(activePolicy)
}

// resolvePolicy returns override when non-nil; otherwise the current
// global policy. Both Balancer and the runtime helpers funnel through
// this so a single source of truth governs upstream validation and
// header stripping.
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
		allowed = DefaultSecurityPolicy().AllowedSchemes
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
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
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
func newSSRFDialer(dialDualStack bool) fasthttp.DialFunc {
	dialer := &net.Dialer{Timeout: dnsLookupTimeout}
	return func(addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("proxy: invalid dial address %q: %w", addr, err)
		}

		var ips []net.IP
		if ip := net.ParseIP(host); ip != nil {
			ips = []net.IP{ip}
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), dnsLookupTimeout)
			defer cancel()
			resolved, lerr := net.DefaultResolver.LookupIPAddr(ctx, host)
			if lerr != nil {
				return nil, fmt.Errorf("%w: %s lookup failed: %w", ErrUpstreamHostBlocked, host, lerr)
			}
			for _, r := range resolved {
				ips = append(ips, r.IP)
			}
		}

		// Reject if ANY resolved address is blocked, mirroring
		// validateHostForSSRF so a mixed public/private answer cannot slip
		// a private target past the guard.
		for _, ip := range ips {
			if isBlockedIP(ip) {
				return nil, fmt.Errorf("%w: %s -> %s", ErrUpstreamHostBlocked, host, ip)
			}
		}

		var lastErr error
		for _, ip := range ips {
			// Mirror fasthttp's default of IPv4-only unless DialDualStack.
			if !dialDualStack && ip.To4() == nil {
				continue
			}
			conn, derr := dialer.Dial("tcp", net.JoinHostPort(ip.String(), port))
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
}

// isBlockedIP reports whether ip falls inside a range that proxy
// upstreams must not reach by default. Loopback, unspecified, RFC 1918
// private, link-local (including the 169.254.169.254 cloud-metadata
// address), multicast, and RFC 6598 CGNAT ranges are blocked.
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
	return false
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
		// new authority. Treat the remainder as an opaque path.
		out.Path = "/" + utils.TrimLeft(requestPath, '/')
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
