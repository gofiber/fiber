package cors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// go test -run -v Test_NormalizeOrigin
func Test_NormalizeOrigin(t *testing.T) {
	testCases := []struct {
		origin         string
		expectedOrigin string
		expectedValid  bool
	}{
		{origin: "http://example.com", expectedValid: true, expectedOrigin: "http://example.com"},                       // Simple case should work.
		{origin: "http://example.com/", expectedValid: true, expectedOrigin: "http://example.com"},                      // Trailing slash should be removed.
		{origin: "http://example.com:3000", expectedValid: true, expectedOrigin: "http://example.com:3000"},             // Port should be preserved.
		{origin: "http://example.com:3000/", expectedValid: true, expectedOrigin: "http://example.com:3000"},            // Trailing slash should be removed.
		{origin: "http://", expectedValid: false, expectedOrigin: ""},                                                   // Invalid origin should not be accepted.
		{origin: "file:///etc/passwd", expectedValid: false, expectedOrigin: ""},                                        // File scheme should not be accepted.
		{origin: "https://*example.com", expectedValid: false, expectedOrigin: ""},                                      // Wildcard domain should not be accepted.
		{origin: "http://*.example.com", expectedValid: false, expectedOrigin: ""},                                      // Wildcard subdomain should not be accepted.
		{origin: "http://example.com/path", expectedValid: false, expectedOrigin: ""},                                   // Path should not be accepted.
		{origin: "http://example.com?query=123", expectedValid: false, expectedOrigin: ""},                              // Query should not be accepted.
		{origin: "http://example.com#fragment", expectedValid: false, expectedOrigin: ""},                               // Fragment should not be accepted.
		{origin: "http://localhost", expectedValid: true, expectedOrigin: "http://localhost"},                           // Localhost should be accepted.
		{origin: "http://127.0.0.1", expectedValid: true, expectedOrigin: "http://127.0.0.1"},                           // IPv4 address should be accepted.
		{origin: "http://[::1]", expectedValid: true, expectedOrigin: "http://[::1]"},                                   // IPv6 address should be accepted.
		{origin: "http://[::1]:8080", expectedValid: true, expectedOrigin: "http://[::1]:8080"},                         // IPv6 address with port should be accepted.
		{origin: "http://[::1]:8080/", expectedValid: true, expectedOrigin: "http://[::1]:8080"},                        // IPv6 address with port and trailing slash should be accepted.
		{origin: "http://[::1]:8080/path", expectedValid: false, expectedOrigin: ""},                                    // IPv6 address with port and path should not be accepted.
		{origin: "http://[::1]:8080?query=123", expectedValid: false, expectedOrigin: ""},                               // IPv6 address with port and query should not be accepted.
		{origin: "http://[::1]:8080#fragment", expectedValid: false, expectedOrigin: ""},                                // IPv6 address with port and fragment should not be accepted.
		{origin: "http://[::1]:8080/path?query=123#fragment", expectedValid: false, expectedOrigin: ""},                 // IPv6 address with port, path, query, and fragment should not be accepted.
		{origin: "http://[::1]:8080/path?query=123#fragment/", expectedValid: false, expectedOrigin: ""},                // IPv6 address with port, path, query, fragment, and trailing slash should not be accepted.
		{origin: "http://[::1]:8080/path?query=123#fragment/invalid", expectedValid: false, expectedOrigin: ""},         // IPv6 address with port, path, query, fragment, trailing slash, and invalid segment should not be accepted.
		{origin: "http://[::1]:8080/path?query=123#fragment/invalid/", expectedValid: false, expectedOrigin: ""},        // IPv6 address with port, path, query, fragment, trailing slash, and invalid segment with trailing slash should not be accepted.
		{origin: "http://[::1]:8080/path?query=123#fragment/invalid/segment", expectedValid: false, expectedOrigin: ""}, // IPv6 address with port, path, query, fragment, trailing slash, and invalid segment with additional segment should not be accepted.
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

// go test -run -v Test_MatchScheme
func Test_MatchScheme(t *testing.T) {
	testCases := []struct {
		domain   string
		pattern  string
		expected bool
	}{
		{domain: "http://example.com", pattern: "http://example.com", expected: true},           // Exact match should work.
		{domain: "https://example.com", pattern: "http://example.com", expected: false},         // Scheme mismatch should matter.
		{domain: "http://example.com", pattern: "https://example.com", expected: false},         // Scheme mismatch should matter.
		{domain: "http://example.com", pattern: "http://example.org", expected: true},           // Different domains should not matter.
		{domain: "http://example.com", pattern: "http://example.com:8080", expected: true},      // Port should not matter.
		{domain: "http://example.com:8080", pattern: "http://example.com", expected: true},      // Port should not matter.
		{domain: "http://example.com:8080", pattern: "http://example.com:8081", expected: true}, // Different ports should not matter.
		{domain: "http://localhost", pattern: "http://localhost", expected: true},               // Localhost should match.
		{domain: "http://127.0.0.1", pattern: "http://127.0.0.1", expected: true},               // IPv4 address should match.
		{domain: "http://[::1]", pattern: "http://[::1]", expected: true},                       // IPv6 address should match.
	}

	for _, tc := range testCases {
		result := matchScheme(tc.domain, tc.pattern)

		if result != tc.expected {
			t.Errorf("Expected matchScheme('%s', '%s') to be %v, but got %v", tc.domain, tc.pattern, tc.expected, result)
		}
	}
}

// go test -run -v Test_NormalizeDomain
func Test_NormalizeDomain(t *testing.T) {
	testCases := []struct {
		input          string
		expectedOutput string
	}{
		{input: "http://example.com", expectedOutput: "example.com"},                     // Simple case with http scheme.
		{input: "https://example.com", expectedOutput: "example.com"},                    // Simple case with https scheme.
		{input: "http://example.com:3000", expectedOutput: "example.com"},                // Case with port.
		{input: "https://example.com:3000", expectedOutput: "example.com"},               // Case with port and https scheme.
		{input: "http://example.com/path", expectedOutput: "example.com/path"},           // Case with path.
		{input: "http://example.com?query=123", expectedOutput: "example.com?query=123"}, // Case with query.
		{input: "http://example.com#fragment", expectedOutput: "example.com#fragment"},   // Case with fragment.
		{input: "example.com", expectedOutput: "example.com"},                            // Case without scheme.
		{input: "example.com:8080", expectedOutput: "example.com"},                       // Case without scheme but with port.
		{input: "sub.example.com", expectedOutput: "sub.example.com"},                    // Case with subdomain.
		{input: "sub.sub.example.com", expectedOutput: "sub.sub.example.com"},            // Case with nested subdomain.
		{input: "http://localhost", expectedOutput: "localhost"},                         // Case with localhost.
		{input: "http://127.0.0.1", expectedOutput: "127.0.0.1"},                         // Case with IPv4 address.
		{input: "http://[::1]", expectedOutput: "[::1]"},                                 // Case with IPv6 address.
	}

	for _, tc := range testCases {
		output := normalizeDomain(tc.input)

		if output != tc.expectedOutput {
			t.Errorf("Expected normalized domain '%s' for input '%s', but got: '%s'", tc.expectedOutput, tc.input, output)
		}
	}
}

// go test -v -run=^$ -bench=Benchmark_CORS_SubdomainMatch -benchmem -count=4
func Benchmark_CORS_SubdomainMatch(b *testing.B) {
	s := subdomain{
		prefix: "www",
		suffix: ".example.com",
	}

	o := "www.example.com"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		s.match(o)
	}
}

func Test_CORS_SubdomainMatch(t *testing.T) {
	tests := []struct {
		name     string
		sub      subdomain
		origin   string
		expected bool
	}{
		{
			name:     "match with different scheme",
			sub:      subdomain{prefix: "http://api.", suffix: ".example.com"},
			origin:   "https://api.service.example.com",
			expected: false,
		},
		{
			name:     "match with different scheme",
			sub:      subdomain{prefix: "https://", suffix: ".example.com"},
			origin:   "http://api.service.example.com",
			expected: false,
		},
		{
			name:     "match with valid subdomain",
			sub:      subdomain{prefix: "https://", suffix: ".example.com"},
			origin:   "https://api.service.example.com",
			expected: true,
		},
		{
			name:     "match with valid nested subdomain",
			sub:      subdomain{prefix: "https://", suffix: ".example.com"},
			origin:   "https://1.2.api.service.example.com",
			expected: true,
		},

		{
			name:     "no match with invalid prefix",
			sub:      subdomain{prefix: "https://abc.", suffix: ".example.com"},
			origin:   "https://service.example.com",
			expected: false,
		},
		{
			name:     "no match with invalid suffix",
			sub:      subdomain{prefix: "https://", suffix: ".example.com"},
			origin:   "https://api.example.org",
			expected: false,
		},
		{
			name:     "no match with empty origin",
			sub:      subdomain{prefix: "https://", suffix: ".example.com"},
			origin:   "",
			expected: false,
		},
		{
			name:     "partial match not considered a match",
			sub:      subdomain{prefix: "https://service.", suffix: ".example.com"},
			origin:   "https://api.example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sub.match(tt.origin)
			assert.Equal(t, tt.expected, got, "subdomain.match()")
		})
	}
}
