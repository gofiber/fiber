// Package schemehost compares the scheme and host of two URLs for same-origin
// checks. It is shared by the CSRF middleware (Origin/Referer validation) and
// the core redirect logic (open-redirect prevention).
package schemehost

import (
	"net/url"
	"strings"

	utilsstrings "github.com/gofiber/utils/v2/strings"
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// Match reports whether (schemeA, hostA) and (schemeB, hostB) denote the same
// origin. Scheme comparison is case-insensitive and default ports (http:80,
// https:443) are normalized so "example.com" and "example.com:443" match.
func Match(schemeA, hostA, schemeB, hostB string) bool {
	normalizedSchemeA := utilsstrings.ToLower(schemeA)
	normalizedSchemeB := utilsstrings.ToLower(schemeB)

	normalizedHostA := normalizeSchemeHost(normalizedSchemeA, hostA)
	normalizedHostB := normalizeSchemeHost(normalizedSchemeB, hostB)

	return normalizedSchemeA == normalizedSchemeB && normalizedHostA == normalizedHostB
}

func normalizeSchemeHost(scheme, host string) string {
	host = utilsstrings.ToLower(host)

	var defaultPort string
	switch scheme {
	case schemeHTTP:
		defaultPort = "80"
	case schemeHTTPS:
		defaultPort = "443"
	default:
		return host
	}

	// Fast path for a clean "host" or "host:port" value (the common case),
	// avoiding the url.Parse allocation. Anything unusual (userinfo, path,
	// percent-encoding, bracketed IPv6, control chars, empty/invalid port, ...)
	// falls back to the url.Parse path, which preserves the exact legacy behavior.
	if hasPort, clean := classifyHostPort(host); clean {
		if hasPort {
			return host
		}
		return host + ":" + defaultPort
	}

	return normalizeSchemeHostViaParse(scheme, host, defaultPort)
}

// classifyHostPort reports whether host is a plain "<reg-name-or-IPv4>" or
// "<reg-name-or-IPv4>:<port>" value (clean) and, if so, whether it carries an
// explicit numeric port. The accepted character set is deliberately narrow
// (lowercase ASCII letters, digits, '.', '-', and a single ':'); anything else,
// including bracketed IPv6 literals, returns clean=false and is handled by the
// url.Parse fallback so behavior stays identical to the legacy implementation.
func classifyHostPort(host string) (hasPort, clean bool) { //nolint:nonamedreturns // names document the two booleans
	colon := -1
	for i := 0; i < len(host); i++ {
		c := host[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '.', c == '-':
			// safe reg-name / IPv4 character
		case c == ':':
			if colon >= 0 {
				return false, false // more than one colon -> not a clean host:port
			}
			colon = i
		default:
			return false, false // brackets, control chars, anything else
		}
	}

	if colon < 0 {
		return false, host != "" // no port; empty host falls back to url.Parse
	}
	if !allDigits(host[colon+1:]) {
		return false, false // "host:" or "host:abc" -> let url.Parse decide
	}
	return true, true
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

// normalizeSchemeHostViaParse is the url.Parse-based fallback. host is already
// lowercased and scheme is known to be http or https.
func normalizeSchemeHostViaParse(scheme, host, defaultPort string) string {
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
