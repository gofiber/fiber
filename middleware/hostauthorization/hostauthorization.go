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
	exact            map[string]struct{}
	wildcardSuffixes []string
	cidrNets         []*net.IPNet
}

// parseAllowedHosts categorizes AllowedHosts into exact, wildcard, and CIDR groups.
// Any entry containing "/" is treated as a CIDR attempt; a parse failure panics so
// misconfigured ranges surface at startup rather than silently becoming an exact entry
// that can never match a real host. Non-canonical CIDRs (host bits set) also panic.
func parseAllowedHosts(hosts []string) parsedHosts {
	parsed := parsedHosts{
		exact: make(map[string]struct{}, len(hosts)),
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

		if strings.Contains(h, "/") {
			hostIP, cidr, err := net.ParseCIDR(h)
			if err != nil {
				panic("hostauthorization: invalid CIDR entry: " + h)
			}
			// Reject non-canonical CIDRs (host bits set): "10.0.0.5/8" would
			// silently expand to 10.0.0.0/8, allowing far more than intended.
			if !hostIP.Equal(cidr.IP) {
				panic("hostauthorization: CIDR has host bits set, use canonical form: " + h)
			}
			parsed.cidrNets = append(parsed.cidrNets, cidr)
		} else if strings.HasPrefix(h, ".") {
			// Subdomain wildcard — store with leading dot to avoid allocation in hot path
			parsed.wildcardSuffixes = append(parsed.wildcardSuffixes, h)
		} else {
			parsed.exact[h] = struct{}{}
		}
	}

	return parsed
}

// normalizeHost normalizes a hostname for matching: port stripped, trailing dot
// removed, IPv6 brackets removed, lowercased.
// Safe to call on both c.Hostname() output (already port-stripped) and raw
// AllowedHosts entries (which may include a port like "example.com:8080").
func normalizeHost(host string) string {
	// Fast path for plain hostnames (no IPv6 brackets, no port).
	// net.SplitHostPort allocates a *AddrError on every error path; skipping
	// it for the common case avoids one allocation per request.
	if len(host) > 0 && host[0] != '[' && strings.IndexByte(host, ':') < 0 {
		host = strings.TrimSuffix(host, ".")
		return utilsstrings.ToLower(host)
	}

	// Handle "[::1]:port", "[::1]", and "host:port" forms.
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	} else {
		// No port: strip bare IPv6 brackets (e.g. "[::1]" → "::1").
		host = strings.TrimPrefix(host, "[")
		host = strings.TrimSuffix(host, "]")
	}

	host = strings.TrimSuffix(host, ".")
	return utilsstrings.ToLower(host)
}

// matchHost checks if the given host matches any of the parsed allowed hosts.
// Evaluation order: exact → wildcard → CIDR → AllowedHostsFunc.
// AllowedHostsFunc is a fallback called only when no static rule matches,
// matching the CORS AllowOriginsFunc convention and avoiding unnecessary calls
// to potentially expensive dynamic validators (e.g. database lookups).
func matchHost(host string, parsed parsedHosts, allowedHostsFunc func(string) bool) bool {
	// Exact match
	if _, ok := parsed.exact[host]; ok {
		return true
	}

	// Subdomain wildcard: ".myapp.com" matches "api.myapp.com" but NOT "myapp.com"
	for _, suffix := range parsed.wildcardSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}

	// CIDR match: parse host as IP and check against CIDR ranges.
	// Pre-check to skip net.ParseIP for obvious non-IP hostnames (e.g. "api.myapp.com"):
	//   - IPv4 addresses start with a digit
	//   - IPv6 addresses always contain ":" (port is already stripped by normalizeHost)
	// Regular hostnames won't match either condition.
	if len(parsed.cidrNets) > 0 && len(host) > 0 {
		firstByte := host[0]
		if (firstByte >= '0' && firstByte <= '9') || strings.IndexByte(host, ':') >= 0 {
			if ip := net.ParseIP(host); ip != nil {
				for _, cidr := range parsed.cidrNets {
					if cidr.Contains(ip) {
						return true
					}
				}
			}
		}
	}

	// Dynamic validator — fallback only; not called when a static rule matched.
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
