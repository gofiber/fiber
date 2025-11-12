package cors

import (
	"net/url"
	"strings"
)

// matchScheme compares the scheme of the domain and pattern
func matchScheme(domain, pattern string) bool {
	didx := strings.Index(domain, ":")
	pidx := strings.Index(pattern, ":")
	return didx != -1 && pidx != -1 && domain[:didx] == pattern[:pidx]
}

// normalizeDomain removes the scheme and port from the input domain
func normalizeDomain(input string) string {
	// Remove scheme
	input = strings.TrimPrefix(strings.TrimPrefix(input, "http://"), "https://")

	// Find and remove port, if present
	if input != "" && input[0] != '[' {
		if portIndex := strings.Index(input, ":"); portIndex != -1 {
			input = input[:portIndex]
		}
	}

	return input
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
	return true, strings.ToLower(parsedOrigin.Scheme + "://" + parsedOrigin.Host)
}

type subdomain struct {
	// The wildcard pattern
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
