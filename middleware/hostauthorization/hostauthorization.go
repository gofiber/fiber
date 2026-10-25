package hostauthorization

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/idnafold"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// RFC 1035 length limits.
const (
	maxDomainLength = 253
	maxLabelLength  = 63
)

type parsedHosts struct {
	exact            map[string]struct{}
	wildcardSuffixes []string
}

// parseAllowedHosts splits AllowedHosts into exact and wildcard groups,
// normalizing entries (port strip, lowercase, IDN→Punycode) and enforcing
// RFC 1035 length limits. Panics on misconfiguration so it surfaces at startup.
func parseAllowedHosts(hosts []string) parsedHosts {
	parsed := parsedHosts{
		exact: make(map[string]struct{}, len(hosts)),
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

		isWildcard := strings.HasPrefix(h, "*.")
		if isWildcard {
			h = h[2:]
		}

		h = normalizeHost(h)
		if h == "" {
			continue
		}

		validateHostLength(h)

		if isWildcard {
			// Stored with leading dot so the hot-path HasSuffix check stays alloc-free.
			parsed.wildcardSuffixes = append(parsed.wildcardSuffixes, "."+h)
		} else {
			parsed.exact[h] = struct{}{}
		}
	}

	return parsed
}

func validateHostLength(host string) {
	if len(host) > maxDomainLength {
		panic(fmt.Sprintf("hostauthorization: host %q exceeds RFC 1035 maximum of %d characters (%d chars)",
			host, maxDomainLength, len(host)))
	}
	// IPv6 hosts contain colons and aren't dotted labels.
	if strings.IndexByte(host, ':') >= 0 {
		return
	}
	for label := range strings.SplitSeq(host, ".") {
		if len(label) > maxLabelLength {
			panic(fmt.Sprintf("hostauthorization: host %q has label %q exceeding RFC 1035 limit of %d characters (%d chars)",
				host, label, maxLabelLength, len(label)))
		}
	}
}

// normalizeHost strips port, trailing dot, and IPv6 brackets, lowercases,
// converts IDN labels to Punycode (matching what browsers send), and writes
// IPv6 addresses in their RFC 5952 canonical form.
func normalizeHost(host string) string {
	host, _ = normalizeHostKind(host)
	return host
}

// normalizeHostKind is normalizeHost that also reports whether the result is a
// valid IPv6 address, so callers do not have to parse it a second time.
func normalizeHostKind(host string) (string, bool) {
	// Fast path for plain hostnames — skips the split and the bracket trims.
	if host != "" && host[0] != '[' && strings.IndexByte(host, ':') < 0 {
		host = trimOneTrailingDot(host)
		host = utilsstrings.ToLower(host)
		return idnafold.ToASCII(host), false
	}

	if h, _, ok := utils.SplitHostPort(host); ok {
		host = h
	} else {
		host = utils.TrimLeft(host, '[')
		host = utils.TrimRight(host, ']')
	}

	host = trimOneTrailingDot(host)
	host = utilsstrings.ToLower(host)
	return canonicalIPv6(idnafold.ToASCII(host))
}

// canonicalIPv6 returns host in the RFC 5952 canonical text form and true when
// it is an IPv6 address. An IPv6 address has many textual spellings
// (RFC 4291 Section 2.2), so comparing the text as written let an allowed
// "[::1]" refuse a Host of "[0:0:0:0:0:0:0:1]", and an allowed
// "[2001:db8:0:0:0:0:0:1]" refuse the "[2001:db8::1]" every browser sends.
// Both the configured hosts and the request host pass through here, so each
// spelling of one address compares equal.
//
// Anything else, including a zoned address, is returned unchanged with false
// so that isValidHostSyntax can refuse it. A zone is checked before parsing
// because utils.ParseIPv6 copies it into a new string, which a client could
// otherwise force on every refused request.
//
// The canonical form is written into a stack buffer and the input is returned
// when it already matches, which is what browsers send, so that case does not
// allocate. Only a non-canonical spelling pays for its new string.
func canonicalIPv6(host string) (string, bool) {
	if strings.IndexByte(host, ':') < 0 || strings.IndexByte(host, '%') >= 0 {
		return host, false
	}
	addr, ok := utils.ParseIPv6(host)
	if !ok {
		return host, false
	}
	var buf [len("ffff:ffff:ffff:ffff:ffff:ffff:255.255.255.255")]byte
	canonical := addr.AppendTo(buf[:0])
	if string(canonical) == host {
		return host, true
	}
	return string(canonical), true
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

	host := authority
	if authority[0] == '[' {
		idx := -1
		for i := 1; i < len(authority); i++ {
			switch authority[i] {
			case '@', '[':
				return "", false
			case ']':
				idx = i
				i = len(authority)
			default:
			}
		}
		if idx <= 1 {
			return "", false
		}

		host = authority[1:idx]
		rest := authority[idx+1:]
		if rest != "" {
			if rest[0] != ':' {
				return "", false
			}
			if !isValidPort(rest[1:]) {
				return "", false
			}
		}
	} else {
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
	}

	host, isIPv6 := normalizeHostKind(host)
	if host == "" {
		return "", false
	}

	// An IPv6 address was already validated while it was canonicalized.
	if !isIPv6 && !isValidHostSyntax(host) {
		return "", false
	}

	return host, true
}

func isValidHostSyntax(host string) bool {
	if host == "" {
		return false
	}

	// A colon only appears in IPv6 literals; validate those with
	// utils.ParseIPv6. Screening out zoned addresses first makes the
	// acceptance identical to net.ParseIP (netip.ParseAddr minus zones)
	// without its net.IP slice allocation on this per-request path.
	if strings.IndexByte(host, ':') >= 0 {
		if strings.IndexByte(host, '%') >= 0 {
			return false
		}
		_, ok := utils.ParseIPv6(host)
		return ok
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
	if raw == "" || len(raw) > 5 {
		return false
	}

	var port int
	for i := 0; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '9' {
			return false
		}
		port = port*10 + int(raw[i]-'0')
	}

	return port <= 65535
}

// matchHost evaluates exact → wildcard → AllowedHostsFunc.
// The func is a fallback only — never called when a static rule matched.
func matchHost(host string, parsed parsedHosts, allowedHostsFunc func(string) bool) bool {
	if _, ok := parsed.exact[host]; ok {
		return true
	}

	// Subdomain wildcards apply to DNS names only. An IP literal, which is the
	// only kind of host that contains a colon, never matches one: the dotted
	// tail of an IPv4-mapped address such as ::ffff:127.0.0.1 is not a label.
	if strings.IndexByte(host, ':') < 0 {
		for _, suffix := range parsed.wildcardSuffixes {
			if strings.HasSuffix(host, suffix) {
				return true
			}
		}
	}

	if allowedHostsFunc != nil && allowedHostsFunc(host) {
		return true
	}

	return false
}

// New creates a new host authorization middleware handler.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)
	parsed := parseAllowedHosts(cfg.AllowedHosts)

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		host, ok := parseNormalizedAuthority(c.Host())
		if !ok {
			return cfg.ErrorHandler(c, ErrForbiddenHost)
		}

		if matchHost(host, parsed, cfg.AllowedHostsFunc) {
			return c.Next()
		}

		return cfg.ErrorHandler(c, ErrForbiddenHost)
	}
}
