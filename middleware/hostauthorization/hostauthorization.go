package hostauthorization

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/hostname"
	"github.com/gofiber/fiber/v3/internal/ipv6host"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// RFC 1035 length limits.
const (
	maxDomainLength = 253
	maxLabelLength  = 63
)

type parsedHosts struct {
	exact map[string]struct{}
	// ips holds the exact IP entries again, so a bracketed host matches without being formatted.
	ips              map[netip.Addr]struct{}
	wildcardSuffixes []string
}

// parseAllowedHosts splits AllowedHosts into exact and wildcard groups,
// normalizing entries (port strip, lowercase, IDN→Punycode) and enforcing
// RFC 1035 length limits. Panics on misconfiguration so it surfaces at startup.
func parseAllowedHosts(hosts []string) parsedHosts {
	parsed := parsedHosts{
		exact: make(map[string]struct{}, len(hosts)),
		ips:   make(map[netip.Addr]struct{}),
	}

	for _, h := range hosts {
		h = utils.TrimSpace(h)
		if h == "" {
			continue
		}

		// Reject the leading-dot form some other tools use; we want "*.example.com".
		if len(h) > 1 && h[0] == '.' {
			panic("hostauthorization: invalid host " + h + " — subdomain wildcards use the \"*.example.com\" form")
		}

		entry := h
		isWildcard := strings.HasPrefix(h, "*.")
		if isWildcard {
			h = h[2:]
		}

		var isIP bool
		h, isIP = normalizeEntry(entry, h)
		if h == "" {
			continue
		}
		if isWildcard && isIP {
			panic(invalidEntry(entry, "subdomain wildcards take a domain name, not an IP address"))
		}
		if !isIP {
			validateHostLength(h)
		}

		if isWildcard {
			// Stored with leading dot so the hot-path HasSuffix check stays alloc-free.
			parsed.wildcardSuffixes = append(parsed.wildcardSuffixes, "."+h)
		} else {
			parsed.exact[h] = struct{}{}
			if isIP {
				parsed.ips[netip.MustParseAddr(h)] = struct{}{}
			}
		}
	}

	return parsed
}

func validateHostLength(host string) {
	if len(host) > maxDomainLength {
		panic(fmt.Sprintf("hostauthorization: host %q exceeds RFC 1035 maximum of %d characters (%d chars)",
			host, maxDomainLength, len(host)))
	}
	for label := range strings.SplitSeq(host, ".") {
		if len(label) > maxLabelLength {
			panic(fmt.Sprintf("hostauthorization: host %q has label %q exceeding RFC 1035 limit of %d characters (%d chars)",
				host, label, maxLabelLength, len(label)))
		}
	}
}

// normalizeEntry normalizes an AllowedHosts entry without its "*." prefix and
// reports whether it is an IP address. An entry no request could match panics.
func normalizeEntry(entry, host string) (string, bool) {
	switch {
	case strings.Contains(host, "://"):
		panic(invalidEntry(entry, "write host[:port], not a URL"))
	case strings.IndexByte(host, '%') >= 0:
		panic(invalidEntry(entry, "IPv6 zones and percent-encoding are not supported"))
	case strings.ContainsAny(host, "[]"):
		// Same rule as for a request: brackets hold an IPv6 address only.
		if h, ok := parseNormalizedAuthority(host); ok {
			return h, true
		}
		panic(invalidEntry(entry, "brackets must hold an IPv6 address, optionally followed by a valid port"))
	}

	if h, ok := canonicalIPv6(host); ok {
		return h, true
	}
	name, port, hasPort := utils.SplitHostPort(host)
	switch {
	case !hasPort && strings.IndexByte(host, ':') >= 0:
		panic(invalidEntry(entry, "not a valid IPv6 address"))
	case !hasPort:
		name = host
	case !isValidPort(port):
		panic(invalidEntry(entry, fmt.Sprintf("invalid port %q", port)))
	}
	name = normalizeName(name)
	return name, utils.IsIPv4(name)
}

func invalidEntry(entry, reason string) string {
	return "hostauthorization: invalid host " + entry + ": " + reason
}

// normalizeName drops one trailing dot, lowercases, and converts IDN labels to
// Punycode, matching what browsers send.
func normalizeName(host string) string {
	return hostname.ToASCII(utilsstrings.ToLower(trimOneTrailingDot(host)))
}

// canonicalIPv6 writes an IPv6 address in the form browsers send (see ipv6host),
// and an IPv4-mapped one as the IPv4 host, as in isTrustedProxyIP.
func canonicalIPv6(host string) (string, bool) {
	addr, ok := ipv6host.Parse(host)
	if !ok {
		return "", false
	}
	return ipv6host.String(addr.Unmap(), host), true
}

func trimOneTrailingDot(host string) string {
	if host != "" && host[len(host)-1] == '.' {
		return host[:len(host)-1]
	}

	return host
}

func parseNormalizedAuthority(authority string) (string, bool) {
	authority = utils.TrimSpace(authority)
	if authority == "" {
		return "", false
	}

	if authority[0] == '[' {
		addr, interior, ok := parseBracketed(authority)
		if !ok {
			return "", false
		}
		return ipv6host.String(addr, interior), true
	}

	host := authority
	colonIdx := -1
	for i := 0; i < len(authority); i++ {
		switch authority[i] {
		case '@', '[', ']':
			return "", false
		case ':':
			if colonIdx != -1 {
				return "", false
			}
			colonIdx = i
		default:
		}
	}

	if colonIdx != -1 {
		host = authority[:colonIdx]
		if !isValidPort(authority[colonIdx+1:]) {
			return "", false
		}
	}

	// Punycode conversion is quadratic in the label length, so bound the raw name first.
	if len(trimOneTrailingDot(host)) > maxDomainLength {
		return "", false
	}

	host = normalizeName(host)
	if host == "" {
		return "", false
	}

	if !isValidHostSyntax(host) {
		return "", false
	}

	return host, true
}

// parseBracketed parses an authority that starts with "[" and returns its address,
// unmapped, and the text between the brackets.
func parseBracketed(authority string) (netip.Addr, string, bool) {
	idx := -1
	for i := 1; i < len(authority); i++ {
		switch authority[i] {
		case '@', '[':
			return netip.Addr{}, "", false
		case ']':
			idx = i
			i = len(authority)
		default:
		}
	}
	if idx <= 1 {
		return netip.Addr{}, "", false
	}

	rest := authority[idx+1:]
	if rest != "" && (rest[0] != ':' || !isValidPort(rest[1:])) {
		return netip.Addr{}, "", false
	}

	// Brackets hold an IPv6 address only; "[allowed.com:443]" must not pass as allowed.com.
	addr, ok := ipv6host.Parse(authority[1:idx])
	if !ok {
		return netip.Addr{}, "", false
	}
	return addr.Unmap(), authority[1:idx], true
}

func isValidHostSyntax(host string) bool {
	if host == "" {
		return false
	}

	// Validate dotted DNS labels in a single pass without allocating.
	labelLen := 0
	for i := 0; i < len(host); i++ {
		ch := host[i]

		if ch == '.' {
			// Reject empty labels (leading/trailing dot or consecutive dots)
			// and labels that end with a hyphen.
			if labelLen == 0 || host[i-1] == '-' {
				return false
			}
			labelLen = 0
			continue
		}

		// A label must not start with a hyphen.
		if labelLen == 0 && ch == '-' {
			return false
		}

		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
			return false
		}

		labelLen++
		if labelLen > maxLabelLength {
			return false
		}
	}

	// The final label must be non-empty and must not end with a hyphen.
	return labelLen != 0 && host[len(host)-1] != '-'
}

func isValidPort(raw string) bool {
	if raw == "" {
		return false
	}

	// A port is *DIGIT (RFC 3986 Section 3.2.3), so leading zeros are valid and do
	// not change the value. Strip them before bounding the digits that carry it, so
	// an oversized run of zeros is rejected without walking it one digit at a time.
	digits := utils.TrimLeft(raw, '0')
	if len(digits) > 5 {
		return false
	}

	var port int
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return false
		}
		port = port*10 + int(digits[i]-'0')
	}

	return port <= 65535
}

// matchHost evaluates exact → wildcard → AllowedHostsFunc.
// The func is a fallback only — never called when a static rule matched.
func matchHost(host string, parsed parsedHosts, allowedHostsFunc func(string) bool) bool {
	if _, ok := parsed.exact[host]; ok {
		return true
	}

	// A canonical IPv6 host holds no dot, so it never matches a suffix here.
	for _, suffix := range parsed.wildcardSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}

	if allowedHostsFunc != nil && allowedHostsFunc(host) {
		return true
	}

	return false
}

// isAllowed reports whether authority is a well-formed host that matches a rule.
func isAllowed(authority string, parsed parsedHosts, allowedHostsFunc func(string) bool) bool {
	authority = utils.TrimSpace(authority)
	if authority == "" || authority[0] != '[' {
		host, ok := parseNormalizedAuthority(authority)
		return ok && matchHost(host, parsed, allowedHostsFunc)
	}

	addr, interior, ok := parseBracketed(authority)
	if !ok {
		return false
	}
	if _, ok := parsed.ips[addr]; ok {
		return true
	}
	// Nothing else can match, so skip formatting the host.
	if len(parsed.wildcardSuffixes) == 0 && allowedHostsFunc == nil {
		return false
	}
	return matchHost(ipv6host.String(addr, interior), parsed, allowedHostsFunc)
}

// New creates a new host authorization middleware handler.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)
	parsed := parseAllowedHosts(cfg.AllowedHosts)

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if isAllowed(c.Host(), parsed, cfg.AllowedHostsFunc) {
			return c.Next()
		}

		return cfg.ErrorHandler(c, ErrForbiddenHost)
	}
}
