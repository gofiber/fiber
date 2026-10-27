package proxy

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/gofiber/utils/v2"
)

// parseUpstream returns the parsed url.URL for raw. Hosts without an
// explicit scheme default to http:// to match the historical Balancer
// behavior where bare "host:port" entries were accepted. A parse failure is
// ErrUpstreamHostInvalid wrapping the *url.Error.
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
		return nil, fmt.Errorf("%w: %w", ErrUpstreamHostInvalid, err)
	}
	return u, nil
}

// validateUpstream parses raw, enforces the scheme allowlist, and unless
// the policy permits private addresses, resolves the hostname and
// rejects responses that include any blocked address. Rejecting on a
// single blocked answer mitigates DNS rebinding attempts in which the
// resolver returns a mix of public and private IPs. A non-nil lookups may answer for the
// hostname from a recent lookup; see validateHostForSSRF.
func validateUpstream(raw string, policy SecurityPolicy, lookups *lookupCache) (*url.URL, error) {
	u, err := parseAllowedUpstream(raw, policy)
	if err != nil {
		return nil, err
	}
	if policy.AllowPrivateIPs {
		return u, nil
	}
	if err := validateHostForSSRF(u.Hostname(), lookups); err != nil {
		return nil, err
	}
	return u, nil
}

// parseAllowedUpstream parses raw and enforces the scheme allowlist and
// host presence without performing any DNS resolution. url.Parse can
// leave Host set while Hostname() is empty (e.g. "http://:8080"), so the
// presence check uses Hostname.
func parseAllowedUpstream(raw string, policy SecurityPolicy) (*url.URL, error) {
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
// upstream. It enforces the scheme allowlist, requires the entry to be an
// origin (the balancer dials the host and forwards the request target as
// is, so a path, userinfo, query or fragment would be dropped silently) and
// rejects IP-literal hosts in blocked ranges, but defers hostname resolution
// to the SSRF-guarded dialer (see dialGuard.dialChecked). Deferring DNS keeps
// a transient resolver failure at startup from panicking the application
// (e.g. crash loops in container orchestrators) and re-checks the resolved
// IP on every dial, which also defeats DNS-rebinding. A lone trailing slash is
// accepted.
func validateUpstreamForBalancer(raw string, policy SecurityPolicy) (*url.URL, error) {
	u, err := parseAllowedUpstream(raw, policy)
	if err != nil {
		return nil, err
	}
	if u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return nil, fmt.Errorf("%w: %q", ErrUpstreamNotOrigin, raw)
	}
	if policy.AllowPrivateIPs {
		return u, nil
	}
	if ip := net.ParseIP(trimBrackets(u.Hostname())); ip != nil && isBlockedIP(ip) {
		return nil, fmt.Errorf("%w: %s", ErrUpstreamHostBlocked, ip)
	}
	return u, nil
}

// trimBrackets removes the surrounding "[" / "]" from an IPv6 literal
// (defensive — url.Hostname() typically does this already). The fiber
// utils Trim helpers only accept a single byte cutset, so we apply
// TrimLeft + TrimRight to handle the bracket pair.
func trimBrackets(host string) string {
	return utils.TrimRight(utils.TrimLeft(host, '['), ']')
}

// placeholderOrigin is the authority joinUpstreamPath parses a request target
// behind. Neither its scheme nor its host is what any request is sent to; it
// only keeps the parser from reading the target's own leading "//" as an
// authority.
const placeholderOrigin = "https://h"

// joinUpstreamPath returns a URL string formed by combining an already
// validated upstream base with the request target the proxy forwards (see
// wiretarget.Routed). The target can only ever be a path, a query and a
// fragment: crafted inputs like "//attacker.example/foo" or "@attacker"
// cannot change the host the proxy connects to, and a leading "//" stays the
// empty segment the router kept apart from "/foo" rather than being
// collapsed into a path the router never matched.
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
		b := make([]byte, 0, len(base.Scheme)+len("://")+len(base.Host)+len(requestPath))
		b = append(b, base.Scheme...)
		b = append(b, "://"...)
		b = append(b, base.Host...)
		b = append(b, requestPath...)
		return utils.UnsafeString(b)
	}
	out := *base
	if requestPath == "" {
		return out.String()
	}
	if requestPath[0] != '/' && requestPath[0] != '?' && requestPath[0] != '#' {
		requestPath = "/" + requestPath
	}
	// Parse behind a placeholder authority, so that the target can only be a
	// path, a query and a fragment. Parsed on its own, a leading "//" would
	// be a network-path reference that moves the proxy to another host, and
	// collapsing it to one slash would forward a path the router never
	// matched: "//admin" is not "/admin" to the router, which keeps empty
	// segments, so no middleware mounted on "/admin" has run for it.
	parsed, err := url.Parse(placeholderOrigin + requestPath)
	if err != nil {
		// The target does not parse cleanly (a malformed escape, say).
		// Treat it as an opaque path, but preserve any path prefix
		// configured on the upstream base so a malformed request can't
		// silently bypass it (e.g. "http://upstream/api" + "/%zz" must stay
		// rooted at "/api").
		fallback := requestPath
		if fallback[0] != '/' {
			fallback = "/" + fallback
		}
		joinURLPath(&out, base, &url.URL{Path: fallback})
		out.RawQuery = ""
		out.Fragment = ""
		return out.String()
	}
	if parsed.Path != "" {
		// Preserve any path prefix configured on the upstream base (e.g.
		// "http://upstream/api" + "/foo" -> "http://upstream/api/foo")
		// rather than overwriting it with the request path alone.
		joinURLPath(&out, base, parsed)
	}
	out.RawQuery = parsed.RawQuery
	out.Fragment = parsed.Fragment
	return out.String()
}

// joinURLPath sets out's Path and RawPath to base's path joined with target's, as net/http/httputil
// does: the RawPath from both EscapedPaths, since an empty RawPath stands for Path's default escaping.
func joinURLPath(out, base, target *url.URL) {
	if base.RawPath == "" && target.RawPath == "" {
		out.Path, out.RawPath = strings.TrimSuffix(base.Path, "/")+"/"+strings.TrimPrefix(target.Path, "/"), ""
		return
	}
	basePath, baseRaw := base.Path, base.EscapedPath()
	// A trailing "%2F" belongs to the last segment and stays.
	if raw, ok := strings.CutSuffix(baseRaw, "/"); ok {
		basePath, baseRaw = strings.TrimSuffix(basePath, "/"), raw
	}
	out.Path = basePath + "/" + strings.TrimPrefix(target.Path, "/")
	out.RawPath = baseRaw + "/" + strings.TrimPrefix(target.EscapedPath(), "/")
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
