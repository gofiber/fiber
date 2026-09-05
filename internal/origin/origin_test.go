package origin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// go test -run -v Test_NormalizeOrigin
func Test_Normalize_AnyScheme(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		origin         string
		expectedOrigin string
		expectedValid  bool
	}{
		{origin: "http://example.com", expectedValid: true, expectedOrigin: "http://example.com"},                       // Simple case should work.
		{origin: "http://example.com/", expectedValid: true, expectedOrigin: "http://example.com"},                      // Trailing slash should be removed.
		{origin: "http://example.com:3000", expectedValid: true, expectedOrigin: "http://example.com:3000"},             // Port should be preserved.
		{origin: "http://example.com:3000/", expectedValid: true, expectedOrigin: "http://example.com:3000"},            // Trailing slash should be removed.
		{origin: "app://example.com/", expectedValid: true, expectedOrigin: "app://example.com"},                        // App scheme should be accepted.
		{origin: "http://", expectedValid: false, expectedOrigin: ""},                                                   // Invalid origin should not be accepted.
		{origin: "file:///etc/passwd", expectedValid: false, expectedOrigin: ""},                                        // File scheme should not be accepted.
		{origin: "https://*example.com", expectedValid: false, expectedOrigin: ""},                                      // Wildcard domain should not be accepted.
		{origin: "http://*.example.com", expectedValid: false, expectedOrigin: ""},                                      // Wildcard subdomain should not be accepted.
		{origin: "http://example.com/path", expectedValid: false, expectedOrigin: ""},                                   // Path should not be accepted.
		{origin: "http://example.com?query=123", expectedValid: false, expectedOrigin: ""},                              // Query should not be accepted.
		{origin: "http://example.com#fragment", expectedValid: false, expectedOrigin: ""},                               // Fragment should not be accepted.
		{origin: "http://user:pass@example.com", expectedValid: false, expectedOrigin: ""},                              // Userinfo should not be accepted.
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
		normalizedOrigin, valid := Normalize(tc.origin, AnyScheme)

		if valid != tc.expectedValid {
			t.Errorf("Expected origin '%s' to be valid: %v, but got: %v", tc.origin, tc.expectedValid, valid)
		}

		if normalizedOrigin != tc.expectedOrigin {
			t.Errorf("Expected normalized origin '%s' for origin '%s', but got: '%s'", tc.expectedOrigin, tc.origin, normalizedOrigin)
		}
	}
}

// benchSubdomains builds n identical wildcard patterns for the worst-case
// (no-match) scan benchmarks below.
func benchSubdomains(n int) []Subdomain {
	subs := make([]Subdomain, n)
	for i := range subs {
		subs[i] = Subdomain{Prefix: "https://", Suffix: "example.com"}
	}
	return subs
}

// Benchmark_SubdomainMatch_PerPatternNormalize reproduces the pre-PR loop,
// which ran normalizeOrigin (one url.Parse) for every pattern. allocs/op scale
// with the pattern count.
//
// go test -v -run=^$ -bench=Benchmark_SubdomainMatch_PerPatternNormalize -benchmem -count=4
func Benchmark_SubdomainMatch_PerPatternNormalize(b *testing.B) {
	subdomains := benchSubdomains(16)
	origin := "https://api.service.example.org" // matches none -> full scan

	b.ReportAllocs()

	for b.Loop() {
		for _, sub := range subdomains {
			normalized, isValid := Normalize(origin, AnyScheme)
			if isValid && normalized == origin && sub.MatchNormalized(origin) {
				break
			}
		}
	}
}

// Benchmark_MatchSubdomainOrigin normalizes once regardless of pattern
// count; allocs/op stay flat as the slice grows. Compare against
// Benchmark_SubdomainMatch_PerPatternNormalize on the same input.
//
// go test -v -run=^$ -bench=Benchmark_MatchSubdomainOrigin -benchmem -count=4
func Benchmark_MatchSubdomainOrigin(b *testing.B) {
	subdomains := benchSubdomains(16)
	origin := "https://api.service.example.org" // matches none -> full scan

	b.ReportAllocs()

	for b.Loop() {
		MatchAny(subdomains, origin, AnyScheme)
	}
}

func Test_SubdomainMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sub      Subdomain
		origin   string
		expected bool
	}{
		{
			name:     "match with different scheme",
			sub:      Subdomain{Prefix: "http://api.", Suffix: "example.com"},
			origin:   "https://api.service.example.com",
			expected: false,
		},
		{
			name:     "match with different scheme",
			sub:      Subdomain{Prefix: "https://", Suffix: "example.com"},
			origin:   "http://api.service.example.com",
			expected: false,
		},
		{
			name:     "match with valid subdomain",
			sub:      Subdomain{Prefix: "https://", Suffix: "example.com"},
			origin:   "https://api.service.example.com",
			expected: true,
		},
		{
			name:     "match with valid nested subdomain",
			sub:      Subdomain{Prefix: "https://", Suffix: "example.com"},
			origin:   "https://1.2.api.service.example.com",
			expected: true,
		},

		{
			name:     "no match with invalid prefix",
			sub:      Subdomain{Prefix: "https://abc.", Suffix: "example.com"},
			origin:   "https://service.example.com",
			expected: false,
		},
		{
			name:     "no match with invalid suffix",
			sub:      Subdomain{Prefix: "https://", Suffix: "example.com"},
			origin:   "https://api.example.org",
			expected: false,
		},
		{
			name:     "no match with empty origin",
			sub:      Subdomain{Prefix: "https://", Suffix: "example.com"},
			origin:   "",
			expected: false,
		},
		{
			name:     "no match with malformed subdomain",
			sub:      Subdomain{Prefix: "https://", Suffix: "example.com"},
			origin:   "https://evil.comexample.com",
			expected: false,
		},
		{
			name:     "partial match not considered a match",
			sub:      Subdomain{Prefix: "https://service.", Suffix: "example.com"},
			origin:   "https://api.example.com",
			expected: false,
		},
		{
			name:     "no match with empty host label",
			sub:      Subdomain{Prefix: "https://", Suffix: "example.com"},
			origin:   "https://.example.com",
			expected: false,
		},
		{
			name:     "no match with malformed host label",
			sub:      Subdomain{Prefix: "https://", Suffix: "example.com"},
			origin:   "https://..example.com",
			expected: false,
		},
		{
			name:     "no match with malformed origin port before suffix",
			sub:      Subdomain{Prefix: "https://", Suffix: "example.com"},
			origin:   "https://evil.com:any.example.com",
			expected: false,
		},
		{
			name:     "no match with empty label before suffix",
			sub:      Subdomain{Prefix: "https://", Suffix: "example.com"},
			origin:   "https://foo..example.com",
			expected: false,
		},
		{
			name:     "no match with userinfo in origin",
			sub:      Subdomain{Prefix: "https://", Suffix: "example.com"},
			origin:   "https://user@api.example.com",
			expected: false,
		},
		{
			name:     "no match with non-normalized origin",
			sub:      Subdomain{Prefix: "https://", Suffix: "example.com"},
			origin:   "https://API.example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := MatchAny([]Subdomain{tt.sub}, tt.origin, AnyScheme)
			assert.Equal(t, tt.expected, got, "MatchAny()")
		})
	}
}

func Test_MatchSubdomainOrigin(t *testing.T) {
	t.Parallel()

	defaultSubs := []Subdomain{
		{Prefix: "https://", Suffix: "example.net"},
		{Prefix: "https://", Suffix: "example.org"},
		{Prefix: "https://", Suffix: "example.com"},
	}

	tests := []struct {
		name       string
		origin     string
		subdomains []Subdomain
		expected   bool
	}{
		{
			name:       "matches first pattern",
			subdomains: defaultSubs,
			origin:     "https://api.service.example.net",
			expected:   true,
		},
		{
			name:       "matches later pattern",
			subdomains: defaultSubs,
			origin:     "https://api.service.example.com",
			expected:   true,
		},
		{
			name:       "rejects invalid origin once",
			subdomains: defaultSubs,
			origin:     "https://user@api.example.com",
			expected:   false,
		},
		{
			name:       "rejects non-normalized origin",
			subdomains: defaultSubs,
			origin:     "https://API.service.example.com",
			expected:   false,
		},
		{
			name:       "rejects unmatched origin",
			subdomains: defaultSubs,
			origin:     "https://api.service.example.dev",
			expected:   false,
		},
		{
			name:       "empty slice never matches",
			subdomains: []Subdomain{},
			origin:     "https://api.service.example.com",
			expected:   false,
		},
		{
			name:       "nil slice never matches",
			subdomains: nil,
			origin:     "https://api.service.example.com",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := MatchAny(tt.subdomains, tt.origin, AnyScheme)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// Test_Normalize_WebSchemesOnly runs the same expectations as the AnyScheme
// table for the cases both policies agree on, which is every case that turns on
// something other than the scheme.
func Test_Normalize_WebSchemesOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		origin string
		want   string
		ok     bool
	}{
		{"http://example.com", "http://example.com", true},
		{"HTTP://EXAMPLE.COM", "http://example.com", true},
		{"https://example.com/", "https://example.com", true},
		{"http://example.com:3000/", "http://example.com:3000", true},
		{"http://[::1]:8080/", "http://[::1]:8080", true},
		{"http://", "", false},
		{"file:///etc/passwd", "", false},
		{"https://*example.com", "", false},
		{"http://example.com/path", "", false},
		{"http://example.com?query=123", "", false},
		{"http://example.com#fragment", "", false},
		{"http://user:pass@example.com", "", false},
	}
	for _, tc := range tests {
		got, ok := Normalize(tc.origin, WebSchemesOnly)
		assert.Equal(t, tc.ok, ok, "Normalize(%q, WebSchemesOnly) validity", tc.origin)
		assert.Equal(t, tc.want, got, "Normalize(%q, WebSchemesOnly) value", tc.origin)
	}
}

// Test_Normalize_PolicyDelta pins the one behavior the two callers do not
// share, so that changing it has to be deliberate rather than incidental.
func Test_Normalize_PolicyDelta(t *testing.T) {
	t.Parallel()
	const appOrigin = "app://example.com"

	got, ok := Normalize(appOrigin, AnyScheme)
	assert.True(t, ok, "CORS echoes origins for schemes beyond http(s)")
	assert.Equal(t, appOrigin, got)

	got, ok = Normalize(appOrigin, WebSchemesOnly)
	assert.False(t, ok, "CSRF authorizes state change, so only http(s) qualifies")
	assert.Empty(t, got)
}

func Test_ParsePattern(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pattern string
		want    Pattern
		policy  SchemePolicy
		ok      bool
	}{
		{"plain origin", "https://example.com", Pattern{Origin: "https://example.com"}, AnyScheme, true},
		{"plain origin normalizes", "HTTPS://Example.COM/", Pattern{Origin: "https://example.com"}, AnyScheme, true},
		{"wildcard", "https://*.example.com", Pattern{Subdomain: Subdomain{Prefix: "https://", Suffix: "example.com"}, Wildcard: true}, AnyScheme, true},
		{"wildcard keeps port", "https://*.example.com:8443", Pattern{Subdomain: Subdomain{Prefix: "https://", Suffix: "example.com:8443"}, Wildcard: true}, AnyScheme, true},
		{"wildcard under a label", "https://*.api.example.com", Pattern{Subdomain: Subdomain{Prefix: "https://", Suffix: "api.example.com"}, Wildcard: true}, AnyScheme, true},
		{"non-web scheme allowed", "app://example.com", Pattern{Origin: "app://example.com"}, AnyScheme, true},
		{"non-web scheme rejected", "app://example.com", Pattern{}, WebSchemesOnly, false},
		{"non-web wildcard rejected", "app://*.example.com", Pattern{Wildcard: true}, WebSchemesOnly, false},
		{"invalid plain", "http://", Pattern{}, AnyScheme, false},
		{"wildcard with path", "https://*.example.com/p", Pattern{Wildcard: true}, AnyScheme, false},
		// "://*." never appears here, because the userinfo sits between, so this
		// is read as a plain origin and rejected for the "*" in its host.
		{"userinfo before wildcard is not a wildcard", "https://u@*.example.com", Pattern{}, AnyScheme, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ParsePattern(tc.pattern, tc.policy)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

// Test_MatchAny_PolicyIsApplied shows the policy reaches the re-normalization
// step, so a scheme CSRF will not accept cannot match a CSRF wildcard.
func Test_MatchAny_PolicyIsApplied(t *testing.T) {
	t.Parallel()
	subs := []Subdomain{{Prefix: "app://", Suffix: "example.com"}}
	assert.True(t, MatchAny(subs, "app://api.example.com", AnyScheme))
	assert.False(t, MatchAny(subs, "app://api.example.com", WebSchemesOnly))
}
