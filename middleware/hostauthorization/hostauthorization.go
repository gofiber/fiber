package hostauthorization

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// parsedHosts holds the pre-parsed host matching structures.
type parsedHosts struct {
	exact            map[string]bool
	wildcardSuffixes []string
	cidrNets         []*net.IPNet
}

// parseAllowedHosts categorizes AllowedHosts into exact, wildcard, and CIDR groups.
// Panics on invalid CIDR entries.
func parseAllowedHosts(hosts []string) parsedHosts {
	parsed := parsedHosts{
		exact: make(map[string]bool, len(hosts)),
	}

	for _, h := range hosts {
		h = utils.TrimSpace(h)
		if h == "" {
			continue
		}
		h = normalizeHost(h)
		if h == "" {
			continue
		}

		if hostIP, cidr, err := net.ParseCIDR(h); err == nil {
			// CIDR range — let net.ParseCIDR be the authoritative detector.
			// Reject non-canonical CIDRs (host bits set): "10.0.0.5/8" silently
			// becomes 10.0.0.0/8 which is far broader than intended.
			if !hostIP.Equal(cidr.IP) {
				panic("hostauthorization: CIDR has host bits set, use canonical form: " + h)
			}
			parsed.cidrNets = append(parsed.cidrNets, cidr)
		} else if strings.HasPrefix(h, ".") {
			// Subdomain wildcard — store with leading dot to avoid allocation in hot path
			parsed.wildcardSuffixes = append(parsed.wildcardSuffixes, h)
		} else {
			// Exact match
			parsed.exact[h] = true
		}
	}

	return parsed
}

// normalizeHost normalizes a hostname for matching.
// Strips port (if any), trailing dot, IPv6 brackets, and lowercases.
// Safe to call on both c.Hostname() output (already port-stripped) and
// raw AllowedHosts entries (which may include a port like "example.com:8080").
func normalizeHost(host string) string {
	// Strip port if present. net.SplitHostPort handles both "host:port" and
	// "[::1]:port" forms, and strips IPv6 brackets as a side effect.
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	} else {
		// No port: strip bare IPv6 brackets (e.g. "[::1]" → "::1").
		host = strings.TrimPrefix(host, "[")
		host = strings.TrimSuffix(host, "]")
	}

	// Strip trailing dot (FQDN normalization)
	host = strings.TrimSuffix(host, ".")

	return utilsstrings.ToLower(host)
}

// matchHost checks if the given host matches any of the parsed allowed hosts.
func matchHost(host string, parsed parsedHosts, allowedHostsFunc func(string) bool) bool {
	// Dynamic validator — checked first so it can override static rules
	if allowedHostsFunc != nil && allowedHostsFunc(host) {
		return true
	}

	// Exact match
	if parsed.exact[host] {
		return true
	}

	// Subdomain wildcard: ".myapp.com" matches "api.myapp.com" but NOT "myapp.com"
	for _, suffix := range parsed.wildcardSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}

	// CIDR match: parse host as IP and check against CIDR ranges
	if len(parsed.cidrNets) > 0 {
		if ip := net.ParseIP(host); ip != nil {
			for _, cidr := range parsed.cidrNets {
				if cidr.Contains(ip) {
					return true
				}
			}
		}
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
