package proxy

import (
	"errors"
	"sync/atomic"

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

	// ErrUpstreamHostBlocked is returned when the proxied URL resolves to
	// an address inside a blocked range (loopback, RFC 1918 private,
	// link-local, multicast, unspecified, CGNAT or another special-use range)
	// and AllowPrivateIPs is false. An unresolvable host is refused with it too
	// (the check fails closed); ErrUpstreamHostUnresolvable tells the two apart.
	ErrUpstreamHostBlocked = errors.New("proxy: upstream host resolves to a blocked address")

	// ErrUpstreamHostUnresolvable is returned, together with ErrUpstreamHostBlocked, when a
	// lookup fails or answers with no address, so a caller can answer 502 or retry rather than
	// report a policy block. errors.As finds the *net.DNSError of a failed lookup.
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

	// AllowPrivateIPs allows upstream hosts to resolve to loopback,
	// private (RFC 1918), link-local, multicast, unspecified, CGNAT
	// (RFC 6598) or other special-use addresses: 0.0.0.0/8, 192.0.0.0/24
	// (which holds Oracle Cloud's metadata address 192.0.0.192),
	// 198.18.0.0/15, 240.0.0.0/4 (the limited broadcast address included)
	// and the deprecated IPv6 site-local range fec0::/10. SECURITY:
	// enabling this exposes the proxy to SSRF attacks against internal
	// services such as cloud metadata endpoints. Default: false.
	//
	// DNS-rebinding scope: when false, the resolved IP is re-validated at
	// dial time, not only up front, so a malicious resolver cannot swap a
	// public answer (seen during validation) for a private one at connect
	// time. Balancer installs a guarded Dial and DialTimeout on each HostClient it builds;
	// the runtime helpers — Do, DoRedirects, DoTimeout, DoDeadline,
	// Forward, DomainForward, BalancerForward — install the same guard on
	// the shared/user-supplied *fasthttp.Client they dispatch through.
	//
	// The default client and clients registered via WithClient are guarded
	// before first use and are fully protected. A client passed as a
	// per-call variadic argument is guarded on first use, so any host it had
	// already dialed keeps a cached, pre-guard HostClient; register such a
	// client via WithClient (or hand the proxy a dedicated, unused client)
	// for the full guarantee. Two paths have no guard at all, and their
	// dialers are the caller's responsibility: a custom Balancer
	// Config.Client (*fasthttp.LBClient), and a *fasthttp.Client with a
	// Transport, which dials by itself.
	//
	// The up-front lookup of a hostname is remembered for a second, which
	// the dial-time check makes safe, so it is remembered for the shared
	// client only: a request through a client passed per call, or one with a
	// Transport, is judged by a lookup of its own.
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
	// that does not normalize paths. A forged escape, a backslash and a
	// dot segment carrying parameters are refused regardless.
	// Default: false.
	AllowAmbiguousSlashes bool
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
		AllowedSchemes:        append([]string(nil), defaultAllowedSchemes...),
		AllowPrivateIPs:       false,
		AllowHTTPSDowngrade:   false,
		KeepHopByHopHeaders:   false,
		AllowAmbiguousSlashes: false,
	}
}

// activePolicy is the global SecurityPolicy consulted by every
// non-Balancer proxy request. Reads are lock-free via atomic.Pointer.
// The pointed-to SecurityPolicy is treated as immutable once installed:
// WithSecurityPolicy is the only writer, and it stores a value whose
// AllowedSchemes slice has been deep-copied to defend against later
// mutation by the caller.
var activePolicy atomic.Pointer[SecurityPolicy]

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

// WithSecurityPolicy installs policy as the global default (see SecurityPolicy) and returns
// the previous one so callers can restore it, e.g. to relax the policy for one test.
func WithSecurityPolicy(policy SecurityPolicy) SecurityPolicy {
	normalized := normalizePolicy(policy)
	// init installs the default policy, so there is always one to hand back.
	return *activePolicy.Swap(&normalized)
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
