package proxy

import (
	"errors"
	"sync/atomic"

	"github.com/gofiber/fiber/v3/internal/crosshost"
	"github.com/gofiber/utils/v2"
)

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

	// ErrUpstreamHostInvalid is returned when the proxied URL is empty,
	// missing a host or cannot be parsed (the *url.Error is wrapped too).
	ErrUpstreamHostInvalid = errors.New("proxy: upstream host is empty or invalid")

	// ErrUpstreamHostBlocked is returned when the proxied URL resolves to an address inside a
	// blocked range (loopback, RFC 1918 private, link-local, multicast, unspecified, CGNAT or
	// another special-use range) and AllowPrivateIPs is false. So is an unresolvable host, as the
	// check fails closed; ErrUpstreamHostUnresolvable tells the two apart.
	ErrUpstreamHostBlocked = errors.New("proxy: upstream host resolves to a blocked address")

	// ErrUpstreamHostUnresolvable is returned with ErrUpstreamHostBlocked when a lookup fails or
	// answers with no address; errors.As finds the *net.DNSError of a failed lookup.
	ErrUpstreamHostUnresolvable = errors.New("proxy: upstream host could not be resolved")

	// ErrUpstreamNotOrigin is returned when a Balancer server carries more
	// than a scheme and a host: a path, userinfo, query or fragment. The
	// balancer dials the host only, so the rest would be dropped silently,
	// and "http://backend/api" would reach all of the upstream, not "/api".
	ErrUpstreamNotOrigin = errors.New("proxy: upstream must be a scheme and host only")

	// ErrRedirectDowngrade is returned when DoRedirects encounters a
	// redirect from an HTTPS upstream to a plaintext HTTP target and
	// AllowHTTPSDowngrade is false.
	ErrRedirectDowngrade = errors.New("proxy: HTTPS to HTTP redirect blocked")
)

// SecurityPolicy controls the runtime security restrictions of proxy.Do, proxy.Forward,
// proxy.DoRedirects, proxy.DoTimeout, proxy.DoDeadline, proxy.DomainForward and
// proxy.BalancerForward, and of Balancer instances without their own Config.SecurityPolicy.
//
// Balancer reads its policy once, when it is built; the runtime helpers read the installed
// policy on every request and dial, so keep it installed while they serve.
type SecurityPolicy struct {
	// AllowedSchemes restricts the URL schemes accepted as upstream
	// targets. Empty defaults to []string{schemeHTTP, schemeHTTPS}.
	AllowedSchemes []string

	// RedirectSensitiveHeaders lists request headers, beyond the built-in set (Authorization,
	// Proxy-Authorization, Proxy-Authenticate, WWW-Authenticate, Cookie and Cookie2), that
	// DoRedirects drops when a redirect changes the origin: scheme, host or port. Use it for
	// credentials the application defines, such as X-Api-Key, which DoRedirects cannot tell from
	// any other header and otherwise forwards to the redirect target. Names match
	// case-insensitively, whatever case the header is stored under; surrounding whitespace and
	// empty entries are ignored. A redirect that keeps the origin keeps every header; the Fiber
	// client's boundary is the hostname instead, so a change of scheme or port alone keeps them
	// there. Only DoRedirects reads it: the other helpers and Balancer never follow a redirect.
	// Default: nil.
	RedirectSensitiveHeaders []string

	// AllowPrivateIPs allows upstream hosts to resolve to loopback, private (RFC 1918), link-local,
	// multicast, unspecified, CGNAT (RFC 6598) or other special-use addresses: 0.0.0.0/8, 192.0.0.0/24
	// (which holds Oracle Cloud's metadata address 192.0.0.192), 198.18.0.0/15, 240.0.0.0/4 (the
	// limited broadcast address included) and the deprecated IPv6 site-local range fec0::/10.
	// SECURITY: enabling this exposes the proxy to SSRF attacks against internal services such as
	// cloud metadata endpoints. Default: false.
	//
	// DNS-rebinding scope: when false, the resolved IP is re-validated at dial time, so a resolver
	// cannot answer public during validation and private at connect. Balancer guards the
	// HostClients it builds and the runtime helpers the *fasthttp.Client they dispatch through; the
	// default client and one registered with WithClient are guarded before first use. A client
	// passed per call is guarded on first use, so a host it had already dialed keeps its pre-guard
	// HostClient: register it with WithClient before its first request instead. A custom Balancer
	// Config.Client (*fasthttp.LBClient) and a *fasthttp.Client with a Transport dial through the
	// caller's own dialers, which the caller guards.
	AllowPrivateIPs bool

	// AllowHTTPSDowngrade permits proxy.DoRedirects to follow redirects
	// from HTTPS upstreams to plaintext HTTP URLs. SECURITY: enabling
	// this can leak credentials or session cookies in plaintext.
	// Default: false.
	AllowHTTPSDowngrade bool

	// KeepHopByHopHeaders disables the RFC 9110 §7.6.1 hop-by-hop header
	// stripping applied to both the outbound request and the inbound
	// response. SECURITY: enabling this can enable request smuggling
	// and proxy-auth credential forwarding. Default: false.
	KeepHopByHopHeaders bool

	// AllowAmbiguousSlashes lets Balancer, DomainForward and BalancerForward
	// forward a routed path holding an escaped slash ("%2F") or an empty
	// segment ("//"), which they otherwise answer with 400 Bad Request. An
	// upstream that keeps both as sent needs it for names such as a
	// GitLab-style "group%2Fproject" id or an object key with a doubled
	// slash. SECURITY: an upstream that decodes "%2F" into a separator or
	// merges "//" before it matches routes reads such a path as another
	// one, under a prefix whose middleware the router never ran for it;
	// enable this only when every upstream keeps them, through a client
	// that does not normalize paths. A path parameter needs
	// AllowPathParameters as well, and a forged escape, a backslash and a
	// dot segment carrying parameters are refused regardless.
	// Default: false.
	AllowAmbiguousSlashes bool

	// AllowPathParameters lets Balancer, DomainForward and BalancerForward
	// forward a routed path with a path parameter, a ";" and what follows it
	// as in "/cart;jsessionid=x", which they otherwise answer with 400 Bad
	// Request. An upstream that relies on ";jsessionid" needs it. Servlet
	// containers such as Tomcat and Jetty strip the parameters from every
	// segment before they match routes, so they read "/admin;x/secret" as
	// "/admin/secret", under a prefix whose middleware the router never ran
	// for it; enable this only when every upstream behind the handler keeps
	// the ";" as part of the name or checks access itself. A dot segment
	// carrying parameters, behind a raw slash or an escaped one, a backslash
	// and a forged escape are refused regardless, and so is a segment made of
	// parameters alone, such as "/;x/admin", unless AllowAmbiguousSlashes is
	// set as well. No effect under UnescapePath, which forwards a ";" as "%3B".
	// Default: false.
	AllowPathParameters bool
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
		AllowedSchemes:           append([]string(nil), defaultAllowedSchemes...),
		RedirectSensitiveHeaders: nil,
		AllowPrivateIPs:          false,
		AllowHTTPSDowngrade:      false,
		KeepHopByHopHeaders:      false,
		AllowAmbiguousSlashes:    false,
		AllowPathParameters:      false,
	}
}

// activePolicy is the global SecurityPolicy consulted by every
// non-Balancer proxy request. Reads are lock-free via atomic.Pointer.
// The pointed-to SecurityPolicy is treated as immutable once installed:
// WithSecurityPolicy is the only writer, and it stores a value whose
// AllowedSchemes and RedirectSensitiveHeaders slices have been deep-copied to
// defend against later mutation by the caller.
var activePolicy atomic.Pointer[SecurityPolicy]

// normalizePolicy returns a copy of policy whose AllowedSchemes and
// RedirectSensitiveHeaders slices are always freshly allocated (the latter
// without blanks). This guarantees callers cannot mutate the
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
	policy.RedirectSensitiveHeaders = crosshost.CopyNames(policy.RedirectSensitiveHeaders)
	return policy
}

// WithSecurityPolicy installs policy as the global default (see SecurityPolicy) and returns
// the previous one so callers can restore it, e.g. to relax the policy for one test.
func WithSecurityPolicy(policy SecurityPolicy) SecurityPolicy {
	normalized := normalizePolicy(policy)
	// init installs the default policy, so there is always one to hand back.
	return *activePolicy.Swap(&normalized)
}

// currentSecurityPolicy returns a snapshot of the active global policy.
// The snapshot is by-value, but its slice headers alias
// the immutable backing arrays installed by WithSecurityPolicy — no copy
// is taken on the read path.
func currentSecurityPolicy() SecurityPolicy {
	return *activePolicy.Load()
}

// resolvePolicy returns override when non-nil; otherwise the current
// global policy. Both Balancer and the runtime helpers funnel through
// this so a single source of truth governs upstream validation and
// header stripping. Override paths still copy the slices so a
// caller passing Config.SecurityPolicy by reference cannot mutate the
// Balancer's view after construction.
func resolvePolicy(override *SecurityPolicy) SecurityPolicy {
	if override != nil {
		return normalizePolicy(*override)
	}
	return currentSecurityPolicy()
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

func init() {
	def := DefaultSecurityPolicy()
	activePolicy.Store(&def)
}
