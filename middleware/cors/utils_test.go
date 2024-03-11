package cors

import (
	"testing"
)

// go test -run -v Test_normalizeOrigin
func Test_normalizeOrigin(t *testing.T) {
	testCases := []struct {
		origin         string
		expectedValid  bool
		expectedOrigin string
	}{
		{"http://example.com", true, "http://example.com"},            // Simple case should work.
		{"http://example.com/", true, "http://example.com"},           // Trailing slash should be removed.
		{"http://example.com:3000", true, "http://example.com:3000"},  // Port should be preserved.
		{"http://example.com:3000/", true, "http://example.com:3000"}, // Trailing slash should be removed.
		{"http://", false, ""},                                                   // Invalid origin should not be accepted.
		{"http://example.com/path", false, ""},                                   // Path should not be accepted.
		{"http://example.com?query=123", false, ""},                              // Query should not be accepted.
		{"http://example.com#fragment", false, ""},                               // Fragment should not be accepted.
		{"http://localhost", true, "http://localhost"},                           // Localhost should be accepted.
		{"http://127.0.0.1", true, "http://127.0.0.1"},                           // IPv4 address should be accepted.
		{"http://[::1]", true, "http://[::1]"},                                   // IPv6 address should be accepted.
		{"http://[::1]:8080", true, "http://[::1]:8080"},                         // IPv6 address with port should be accepted.
		{"http://[::1]:8080/", true, "http://[::1]:8080"},                        // IPv6 address with port and trailing slash should be accepted.
		{"http://[::1]:8080/path", false, ""},                                    // IPv6 address with port and path should not be accepted.
		{"http://[::1]:8080?query=123", false, ""},                               // IPv6 address with port and query should not be accepted.
		{"http://[::1]:8080#fragment", false, ""},                                // IPv6 address with port and fragment should not be accepted.
		{"http://[::1]:8080/path?query=123#fragment", false, ""},                 // IPv6 address with port, path, query, and fragment should not be accepted.
		{"http://[::1]:8080/path?query=123#fragment/", false, ""},                // IPv6 address with port, path, query, fragment, and trailing slash should not be accepted.
		{"http://[::1]:8080/path?query=123#fragment/invalid", false, ""},         // IPv6 address with port, path, query, fragment, trailing slash, and invalid segment should not be accepted.
		{"http://[::1]:8080/path?query=123#fragment/invalid/", false, ""},        // IPv6 address with port, path, query, fragment, trailing slash, and invalid segment with trailing slash should not be accepted.
		{"http://[::1]:8080/path?query=123#fragment/invalid/segment", false, ""}, // IPv6 address with port, path, query, fragment, trailing slash, and invalid segment with additional segment should not be accepted.
	}

	for _, tc := range testCases {
		valid, normalizedOrigin := normalizeOrigin(tc.origin)

		if valid != tc.expectedValid {
			t.Errorf("Expected origin '%s' to be valid: %v, but got: %v", tc.origin, tc.expectedValid, valid)
		}

		if normalizedOrigin != tc.expectedOrigin {
			t.Errorf("Expected normalized origin '%s' for origin '%s', but got: '%s'", tc.expectedOrigin, tc.origin, normalizedOrigin)
		}
	}
}

// go test -run -v Test_matchScheme
func Test_matchScheme(t *testing.T) {
	testCases := []struct {
		domain   string
		pattern  string
		expected bool
	}{
		{"http://example.com", "http://example.com", true},           // Exact match should work.
		{"https://example.com", "http://example.com", false},         // Scheme mismatch should matter.
		{"http://example.com", "https://example.com", false},         // Scheme mismatch should matter.
		{"http://example.com", "http://example.org", true},           // Different domains should not matter.
		{"http://example.com", "http://example.com:8080", true},      // Port should not matter.
		{"http://example.com:8080", "http://example.com", true},      // Port should not matter.
		{"http://example.com:8080", "http://example.com:8081", true}, // Different ports should not matter.
		{"http://localhost", "http://localhost", true},               // Localhost should match.
		{"http://127.0.0.1", "http://127.0.0.1", true},               // IPv4 address should match.
		{"http://[::1]", "http://[::1]", true},                       // IPv6 address should match.
	}

	for _, tc := range testCases {
		result := matchScheme(tc.domain, tc.pattern)

		if result != tc.expected {
			t.Errorf("Expected matchScheme('%s', '%s') to be %v, but got %v", tc.domain, tc.pattern, tc.expected, result)
		}
	}
}

// go test -run -v Test_validateOrigin
func Test_validateOrigin(t *testing.T) {
	testCases := []struct {
		domain   string
		pattern  string
		expected bool
	}{
		{"http://example.com", "http://example.com", true},            // Exact match should work.
		{"https://example.com", "http://example.com", false},          // Scheme mismatch should matter in CORS context.
		{"http://example.com", "https://example.com", false},          // Scheme mismatch should matter in CORS context.
		{"http://example.com", "http://example.org", false},           // Different domains should not match.
		{"http://example.com", "http://example.com:8080", false},      // Port mismatch should matter.
		{"http://example.com:8080", "http://example.com", false},      // Port mismatch should matter.
		{"http://example.com:8080", "http://example.com:8081", false}, // Different ports should not match.
		{"example.com", "example.com", true},                          // Simplified form, assuming scheme and port are not considered here, but in practice, they are part of the origin.
		{"sub.example.com", "example.com", false},                     // Subdomain should not match the base domain directly.
		{"sub.example.com", ".example.com", true},                     // Correct assumption for wildcard subdomain matching.
		{"evilexample.com", ".example.com", false},                    // Base domain should not match its wildcard subdomain pattern.
		{"example.com", ".example.com", false},                        // Base domain should not match its wildcard subdomain pattern.
		{"sub.example.com", ".com", true},                             // Technically correct for pattern matching, but broad wildcard use like this is not recommended for CORS.
		{"sub.sub.example.com", ".example.com", true},                 // Nested subdomain should match the wildcard pattern.
		{"example.com", ".org", false},                                // Different TLDs should not match.
		{"example.com", "example.org", false},                         // Different domains should not match.
		{"example.com:8080", ".example.com", false},                   // Different ports mean different origins.
		{"example.com", "sub.example.net", false},                     // Different domains should not match.
		{"http://localhost", "http://localhost", true},                // Localhost should match.
		{"http://127.0.0.1", "http://127.0.0.1", true},                // IPv4 address should match.
		{"http://[::1]", "http://[::1]", true},                        // IPv6 address should match.
	}

	for _, tc := range testCases {
		result := validateDomain(tc.domain, tc.pattern)

		if result != tc.expected {
			t.Errorf("Expected validateOrigin('%s', '%s') to be %v, but got %v", tc.domain, tc.pattern, tc.expected, result)
		}
	}
}

// go test -run -v Test_normalizeDomain
func Test_normalizeDomain(t *testing.T) {
	testCases := []struct {
		input          string
		expectedOutput string
	}{
		{"http://example.com", "example.com"},                     // Simple case with http scheme.
		{"https://example.com", "example.com"},                    // Simple case with https scheme.
		{"http://example.com:3000", "example.com"},                // Case with port.
		{"https://example.com:3000", "example.com"},               // Case with port and https scheme.
		{"http://example.com/path", "example.com/path"},           // Case with path.
		{"http://example.com?query=123", "example.com?query=123"}, // Case with query.
		{"http://example.com#fragment", "example.com#fragment"},   // Case with fragment.
		{"example.com", "example.com"},                            // Case without scheme.
		{"example.com:8080", "example.com"},                       // Case without scheme but with port.
		{"sub.example.com", "sub.example.com"},                    // Case with subdomain.
		{"sub.sub.example.com", "sub.sub.example.com"},            // Case with nested subdomain.
		{"http://localhost", "localhost"},                         // Case with localhost.
		{"http://127.0.0.1", "127.0.0.1"},                         // Case with IPv4 address.
		{"http://[::1]", "[::1]"},                                 // Case with IPv6 address.
	}

	for _, tc := range testCases {
		output := normalizeDomain(tc.input)

		if output != tc.expectedOutput {
			t.Errorf("Expected normalized domain '%s' for input '%s', but got: '%s'", tc.expectedOutput, tc.input, output)
		}
	}
}
