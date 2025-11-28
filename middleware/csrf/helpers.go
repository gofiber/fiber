package csrf

import (
	"crypto/subtle"
	"net/url"
	"strings"

	"github.com/gofiber/utils/v2"
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

func compareTokens(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

func compareStrings(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func schemeAndHostMatch(schemeA, hostA, schemeB, hostB string) bool {
	normalizedSchemeA := utils.ToLower(schemeA)
	normalizedSchemeB := utils.ToLower(schemeB)

	normalizedHostA := normalizeSchemeHost(normalizedSchemeA, hostA)
	normalizedHostB := normalizeSchemeHost(normalizedSchemeB, hostB)

	return normalizedSchemeA == normalizedSchemeB && normalizedHostA == normalizedHostB
}

func normalizeSchemeHost(scheme, host string) string {
	host = utils.ToLower(host)

	defaultPort := ""
	switch scheme {
	case schemeHTTP:
		defaultPort = "80"
	case schemeHTTPS:
		defaultPort = "443"
	default:
		return host
	}

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

	if strings.Contains(hostname, ":") && !strings.HasPrefix(hostname, "[") {
		hostname = "[" + hostname + "]"
	}

	return hostname + ":" + defaultPort
}

// normalizeOrigin checks if the provided origin is in a correct format
// and normalizes it by removing any path or trailing slash.
// It returns a boolean indicating whether the origin is valid
// and the normalized origin.
func normalizeOrigin(origin string) (valid bool, normalized string) { //nolint:nonamedreturns // gocritic unnamedResult prefers naming validity and normalized origin results
	parsedOrigin, err := url.Parse(origin)
	if err != nil {
		return false, ""
	}

	// Validate the scheme is either http or https
	if parsedOrigin.Scheme != schemeHTTP && parsedOrigin.Scheme != schemeHTTPS {
		return false, ""
	}

	// Don't allow a wildcard with a protocol
	// wildcards cannot be used within any other value. For example, the following header is not valid:
	// Access-Control-Allow-Origin: https://*
	if strings.Contains(parsedOrigin.Host, "*") {
		return false, ""
	}

	// Validate there is a host present. The presence of a path, query, or fragment components
	// is checked, but a trailing "/" (indicative of the root) is allowed for the path and will be normalized
	if parsedOrigin.Host == "" || (parsedOrigin.Path != "" && parsedOrigin.Path != "/") || parsedOrigin.RawQuery != "" || parsedOrigin.Fragment != "" {
		return false, ""
	}

	// Normalize the origin by constructing it from the scheme and host.
	// The path or trailing slash is not included in the normalized origin.
	return true, utils.ToLower(parsedOrigin.Scheme) + "://" + utils.ToLower(parsedOrigin.Host)
}

type subdomain struct {
	prefix string
	suffix string
}

func (s subdomain) match(o string) bool {
	// Not a subdomain if not long enough for a dot separator.
	if len(o) < len(s.prefix)+len(s.suffix)+1 {
		return false
	}

	if !strings.HasPrefix(o, s.prefix) || !strings.HasSuffix(o, s.suffix) {
		return false
	}

	// Check for the dot separator and validate that there is at least one
	// non-empty label between prefix and suffix. Empty labels like
	// "https://.example.com" or "https://..example.com" should not match.
	suffixStartIndex := len(o) - len(s.suffix)
	if suffixStartIndex <= len(s.prefix) {
		return false
	}
	if o[suffixStartIndex-1] != '.' {
		return false
	}

	// Extract the subdomain part (without the trailing dot) and ensure it
	// doesn't contain empty labels.
	sub := o[len(s.prefix) : suffixStartIndex-1]
	if sub == "" || strings.HasPrefix(sub, ".") || strings.Contains(sub, "..") {
		return false
	}

	return true
}
