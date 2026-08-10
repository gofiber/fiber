// Package schemehost compares the scheme and host of two URLs for same-origin
// checks. It is shared by the CSRF middleware (Origin/Referer validation) and
// the core redirect logic (open-redirect prevention).
package schemehost

import (
	"net/url"
	"strings"

	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// schemePorts is the single source of truth for the schemes whose default
// port is normalized away during origin comparison.
var schemePorts = [...]struct {
	scheme string
	port   string
}{
	{schemeHTTP, "80"},
	{schemeHTTPS, "443"},
}

// foldSchemePort resolves scheme against schemePorts, ASCII
// case-insensitively, returning the canonical lowercase scheme and its
// default port.
func foldSchemePort(scheme string) (canonical, port string, known bool) { //nolint:nonamedreturns // names document the three results
	for _, e := range schemePorts {
		if utils.EqualFold(scheme, e.scheme) {
			return e.scheme, e.port, true
		}
	}
	return "", "", false
}

// Match reports whether (schemeA, hostA) and (schemeB, hostB) denote the same
// origin. Scheme comparison is case-insensitive and default ports (http:80,
// https:443) are normalized so "example.com" and "example.com:443" match.
func Match(schemeA, hostA, schemeB, hostB string) bool {
	if !utils.EqualFold(schemeA, schemeB) {
		return false
	}

	// Identical host strings always normalize identically, so they denote the
	// same origin once the schemes match. This is the dominant same-origin
	// input (e.g. Origin-vs-Host on non-CORS requests).
	if hostA == hostB {
		return true
	}

	scheme, defaultPort, known := foldSchemePort(schemeA)
	if !known {
		// Unknown schemes get no port normalization: the hosts must simply be
		// equal, ASCII case-insensitively.
		return utils.EqualFold(hostA, hostB)
	}

	// Fast path for two clean "host" or "host:port" values (the common case):
	// compare the host parts case-insensitively and the effective ports
	// exactly, without allocating lowered or port-normalized copies.
	if hostOnlyA, portA, cleanA := splitCleanHostPort(hostA); cleanA {
		if hostOnlyB, portB, cleanB := splitCleanHostPort(hostB); cleanB {
			if portA == "" {
				portA = defaultPort
			}
			if portB == "" {
				portB = defaultPort
			}
			return portA == portB && utils.EqualFold(hostOnlyA, hostOnlyB)
		}
	}

	// Anything unusual (userinfo, percent-encoding, bracketed IPv6, control
	// chars, invalid port, ...) takes the legacy normalize-and-compare path.
	return normalizeHostPort(scheme, hostA, defaultPort) == normalizeHostPort(scheme, hostB, defaultPort)
}

// normalizeHostPort lowercases host and appends defaultPort when no explicit
// port is present. scheme is only used by the url.Parse fallback.
func normalizeHostPort(scheme, host, defaultPort string) string {
	host = utilsstrings.ToLower(host)

	// Clean "host" or "host:port" values (e.g. the clean side of a mixed
	// clean/unclean pair; Match handles the clean/clean case itself) avoid the
	// url.Parse allocation. Anything unusual (userinfo, path, percent-encoding,
	// bracketed IPv6, control chars, empty/invalid port, ...) falls back to the
	// url.Parse path, which preserves the exact legacy behavior.
	if _, port, clean := splitCleanHostPort(host); clean {
		if port != "" {
			return host
		}
		return host + ":" + defaultPort
	}

	return normalizeHostPortViaParse(scheme, host, defaultPort)
}

// splitCleanHostPort splits a plain "<reg-name-or-IPv4>" or
// "<reg-name-or-IPv4>:<port>" value (clean) into its host and port parts. The
// accepted character set is deliberately narrow (ASCII letters, digits, '.',
// '-', and a single ':' followed by digits); anything else, including
// bracketed IPv6 literals, returns clean=false and is handled by the
// url.Parse fallback so behavior stays identical to the legacy implementation.
func splitCleanHostPort(s string) (host, port string, clean bool) { //nolint:nonamedreturns // names document the three results
	colon := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '.', c == '-':
			// safe reg-name / IPv4 character
		case c == ':':
			if colon >= 0 {
				return "", "", false // more than one colon -> not a clean host:port
			}
			colon = i
		default:
			return "", "", false // brackets, control chars, anything else
		}
	}

	if colon < 0 {
		return s, "", s != "" // no port; empty host falls back to url.Parse
	}
	if !allDigits(s[colon+1:]) {
		return "", "", false // "host:" or "host:abc" -> let url.Parse decide
	}
	return s[:colon], s[colon+1:], true
}

// allDigits reports whether s is non-empty and all ASCII digits.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// normalizeHostPortViaParse is the url.Parse-based fallback. host is already
// lowercased and scheme is known to be http or https.
func normalizeHostPortViaParse(scheme, host, defaultPort string) string {
	parsedHost, err := url.Parse(scheme + "://" + host)
	if err != nil {
		return host
	}

	if port := parsedHost.Port(); port != "" {
		return host
	}

	hostname := parsedHost.Hostname()
	if hostname == "" {
		return host
	}

	if strings.IndexByte(hostname, ':') >= 0 && !strings.HasPrefix(hostname, "[") {
		hostname = "[" + hostname + "]"
	}

	return hostname + ":" + defaultPort
}
