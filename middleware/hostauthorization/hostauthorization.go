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

		switch {
		case strings.Contains(h, "/"):
			// CIDR range
			_, cidr, err := net.ParseCIDR(h)
			if err != nil {
				panic("hostauthorization: invalid CIDR: " + h)
			}
			parsed.cidrNets = append(parsed.cidrNets, cidr)

		case strings.HasPrefix(h, "."):
			// Subdomain wildcard — store with leading dot to avoid allocation in hot path
			parsed.wildcardSuffixes = append(parsed.wildcardSuffixes, h)

		default:
			// Exact match
			parsed.exact[h] = true
		}
	}

	return parsed
}

// normalizeHost normalizes a hostname (already port-stripped by c.Hostname()).
// Strips trailing dot, IPv6 brackets, and lowercases.
func normalizeHost(host string) string {
	// Strip IPv6 brackets
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")

	// Strip trailing dot (FQDN normalization)
	host = strings.TrimSuffix(host, ".")

	return utilsstrings.ToLower(host)
}

// matchHost checks if the given host matches any of the parsed allowed hosts.
func matchHost(host string, parsed parsedHosts, allowedHostsFunc func(string) bool) bool {
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

	// Dynamic validator fallback
	if allowedHostsFunc != nil {
		return allowedHostsFunc(host)
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

		if host == "" {
			return cfg.ErrorHandler(c, ErrForbiddenHost)
		}

		if matchHost(host, parsed, cfg.AllowedHostsFunc) {
			return c.Next()
		}

		return cfg.ErrorHandler(c, ErrForbiddenHost)
	}
}
