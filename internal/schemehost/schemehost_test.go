package schemehost

import (
	"net/url"
	"strings"
	"testing"

	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/stretchr/testify/assert"
)

// refNormalizeSchemeHost is the original url.Parse-based implementation, kept as
// a behavioral reference. The fast-path normalizeSchemeHost must produce
// identical output for every input.
func refNormalizeSchemeHost(scheme, host string) string {
	host = utilsstrings.ToLower(host)

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

	if strings.IndexByte(hostname, ':') >= 0 && !strings.HasPrefix(hostname, "[") {
		hostname = "[" + hostname + "]"
	}

	return hostname + ":" + defaultPort
}

var corpus = []string{
	"example.com", "example.com:8080", "example.com:443", "example.com:80",
	"[::1]", "[::1]:8080", "[::1]:443", "::1", "a:b:c",
	"EXAMPLE.COM", "Example.Com:8080",
	"example.com:", "example.com:abc", ":8080", ":", "",
	"[::1", "::1]", "[]", "[]:80", "[]:", "[::1]:", "[::1]:abc",
	"192.168.0.1", "192.168.0.1:80", "[2001:db8::1]", "[2001:db8::1]:8443",
	"exa_mple.com", "host name", "exam ple.com",
	"xn--caf-dma.com", "example.com.", "example.com..",
	"user@example.com", "user:pass@example.com", "example.com/path",
	"example.com?q=1", "example.com#frag", "example%2ecom",
	"localhost", "localhost:3000", "127.0.0.1:0",
	"[fe80::1%eth0]", "[fe80::1%25eth0]:8080",
	"example.com:99999", "example.com:0", "host:00080",
	"\\example.com", "example.com\\", "[::1]extra", "[::1]x:80", "\x01",
}

// Test_normalizeSchemeHost_matchesReference verifies the fast path produces the
// exact same result as the url.Parse reference across a broad, adversarial corpus.
func Test_normalizeSchemeHost_matchesReference(t *testing.T) {
	t.Parallel()
	for _, scheme := range []string{"http", "https", "ftp", "HTTP", "HTTPS", ""} {
		for _, host := range corpus {
			got := normalizeSchemeHost(scheme, host)
			want := refNormalizeSchemeHost(scheme, host)
			assert.Equal(t, want, got, "normalizeSchemeHost(%q, %q)", scheme, host)
		}
	}
}

func Test_normalizeSchemeHost(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name, scheme, host, expectedHost string
	}{
		{"http default port added", "http", "example.com", "example.com:80"},
		{"https default port added", "https", "example.com", "example.com:443"},
		{"http custom port preserved", "http", "example.com:8080", "example.com:8080"},
		{"https ipv6 default port added", "https", "[::1]", "[::1]:443"},
		{"unknown scheme preserved", "ftp", "example.com", "example.com"},
		{"https ipv6 custom port preserved", "https", "[::1]:8080", "[::1]:8080"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expectedHost, normalizeSchemeHost(tc.scheme, tc.host))
		})
	}
}

// refMatch is the original implementation of Match, kept as a behavioral
// reference: lowercase both schemes, then compare the normalized scheme-host
// strings. Match must return the same result for every input.
func refMatch(schemeA, hostA, schemeB, hostB string) bool {
	normalizedSchemeA := utilsstrings.ToLower(schemeA)
	normalizedSchemeB := utilsstrings.ToLower(schemeB)
	return normalizedSchemeA == normalizedSchemeB &&
		refNormalizeSchemeHost(normalizedSchemeA, hostA) == refNormalizeSchemeHost(normalizedSchemeB, hostB)
}

// Test_Match_matchesReference verifies the allocation-free fast path in Match
// produces the same verdict as the reference implementation across the full
// adversarial corpus.
func Test_Match_matchesReference(t *testing.T) {
	t.Parallel()
	schemes := []string{"http", "https", "HTTP", "HTTPS", "ftp", ""}
	for _, schemeA := range schemes {
		for _, schemeB := range schemes {
			for _, hostA := range corpus {
				for _, hostB := range corpus {
					got := Match(schemeA, hostA, schemeB, hostB)
					want := refMatch(schemeA, hostA, schemeB, hostB)
					assert.Equal(t, want, got, "Match(%q,%q,%q,%q)", schemeA, hostA, schemeB, hostB)
				}
			}
		}
	}
}

func Test_Match(t *testing.T) {
	t.Parallel()
	tests := []struct {
		schemeA, hostA, schemeB, hostB string
		want                           bool
	}{
		{"https", "example.com", "https", "example.com", true},
		{"https", "example.com", "https", "example.com:443", true},
		{"http", "example.com", "http", "example.com:80", true},
		{"https", "example.com:443", "https", "example.com:443", true},
		{"HTTPS", "Example.com", "https", "example.com", true},
		{"https", "example.com", "http", "example.com", false},
		{"https", "example.com", "https", "evil.com", false},
		{"https", "example.com:8080", "https", "example.com", false},
		{"https", "[::1]", "https", "[::1]:443", true},
		{"https", "[::1]:8080", "https", "[::1]", false},
	}
	for _, tc := range tests {
		got := Match(tc.schemeA, tc.hostA, tc.schemeB, tc.hostB)
		assert.Equal(t, tc.want, got, "Match(%q,%q,%q,%q)", tc.schemeA, tc.hostA, tc.schemeB, tc.hostB)
	}
}

func Benchmark_normalizeSchemeHost(b *testing.B) {
	cases := []struct{ name, scheme, host string }{
		{"noport", "https", "example.com"},
		{"port", "https", "example.com:8080"},
		{"ipv4", "https", "192.168.0.1"},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var s string
			for b.Loop() {
				s = normalizeSchemeHost(tc.scheme, tc.host)
			}
			_ = s
		})
	}
}

func Benchmark_Match(b *testing.B) {
	b.ReportAllocs()
	var ok bool
	for b.Loop() {
		ok = Match("https", "example.com", "https", "example.com")
	}
	_ = ok
}

// FuzzNormalizeSchemeHost asserts the fast path stays byte-for-byte equivalent
// to the url.Parse reference for arbitrary host strings.
func FuzzNormalizeSchemeHost(f *testing.F) {
	for _, host := range corpus {
		f.Add(host)
	}
	f.Fuzz(func(t *testing.T, host string) {
		for _, scheme := range []string{"http", "https", "ftp", ""} {
			got := normalizeSchemeHost(scheme, host)
			want := refNormalizeSchemeHost(scheme, host)
			if got != want {
				t.Fatalf("normalizeSchemeHost(%q, %q) = %q, want %q", scheme, host, got, want)
			}
		}
	})
}
