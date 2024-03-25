package csrf

import (
	"crypto/subtle"
	"net/url"
	"strings"
)

func compareTokens(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

func compareStrings(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// normalizeOrigin checks if the provided origin is in a correct format
// and normalizes it by removing any path or trailing slash.
// It returns a boolean indicating whether the origin is valid
// and the normalized origin.
func normalizeOrigin(origin string) (bool, string) {
	parsedOrigin, err := url.Parse(origin)
	if err != nil {
		return false, ""
	}

	// Validate the scheme is either http or https
	if parsedOrigin.Scheme != "http" && parsedOrigin.Scheme != "https" {
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
	return true, strings.ToLower(parsedOrigin.Scheme) + "://" + strings.ToLower(parsedOrigin.Host)
}

type subdomain struct {
	prefix string
	suffix string
}

func (s subdomain) match(o string) bool {
	return len(o) >= len(s.prefix)+len(s.suffix) && strings.HasPrefix(o, s.prefix) && strings.HasSuffix(o, s.suffix)
}
