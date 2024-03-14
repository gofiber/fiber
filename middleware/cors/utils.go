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

// validateDomain checks if the domain matches the pattern
func validateDomain(domain, pattern string) bool {
	// Directly compare the domain and pattern for an exact match.
	if domain == pattern {
		return true
	}

	// Normalize domain and pattern to exclude schemes and ports for matching purposes
	normalizedDomain := normalizeDomain(domain)
	normalizedPattern := normalizeDomain(pattern)

	// Handling the case where pattern is a wildcard subdomain pattern.
	if strings.HasPrefix(normalizedPattern, ".") {
		// Trim leading "." from pattern for comparison.
		trimmedPattern := normalizedPattern[1:]

		// Check if the domain ends with a dot followed by the trimmed pattern.
		if strings.HasSuffix(normalizedDomain, "."+trimmedPattern) {
			return true
		}
	}

	return false
}

// normalizeDomain removes the scheme and port from the input domain
func normalizeDomain(input string) string {
	// Remove scheme
	input = strings.TrimPrefix(strings.TrimPrefix(input, "http://"), "https://")

	// Find and remove port, if present
	if len(input) > 0 && input[0] != '[' {
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
	// Access-Control-Allow-Origin: https://*.normal-website.com
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
