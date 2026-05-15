package hostauthorization

import (
	"fmt"
	"net"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"golang.org/x/net/idna"
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
// and converts IDN labels to Punycode (matching what browsers send).
func normalizeHost(host string) string {
	// Fast path for plain hostnames — avoids net.SplitHostPort's error allocation.
	if host != "" && host[0] != '[' && strings.IndexByte(host, ':') < 0 {
		host = strings.TrimSuffix(host, ".")
		host = utilsstrings.ToLower(host)
		return toPunycode(host)
	}

	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	} else {
		host = strings.TrimPrefix(host, "[")
		host = strings.TrimSuffix(host, "]")
	}

	host = strings.TrimSuffix(host, ".")
	host = utilsstrings.ToLower(host)
	return toPunycode(host)
}

func toPunycode(host string) string {
	if host == "" || strings.IndexByte(host, ':') >= 0 || isASCII(host) {
		return host
	}
	if ascii, err := idna.Lookup.ToASCII(host); err == nil {
		return ascii
	}
	// Non-convertible input falls through; it won't match any Punycode entry,
	// which is the correct security default.
	return host
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// matchHost evaluates exact → wildcard → AllowedHostsFunc.
// The func is a fallback only — never called when a static rule matched.
func matchHost(host string, parsed parsedHosts, allowedHostsFunc func(string) bool) bool {
	if _, ok := parsed.exact[host]; ok {
		return true
	}

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

// New creates a new host authorization middleware handler.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)
	parsed := parseAllowedHosts(cfg.AllowedHosts)

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		host := normalizeHost(c.Hostname())

		if matchHost(host, parsed, cfg.AllowedHostsFunc) {
			return c.Next()
		}

		return cfg.ErrorHandler(c, ErrForbiddenHost)
	}
}
