package idnafold

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ToASCII(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		host string
		want string
	}{
		{"empty", "", ""},
		{"already ASCII, unchanged", "example.com", "example.com"},
		{"already Punycode, unchanged", "xn--mnchen-3ya.example.com", "xn--mnchen-3ya.example.com"},
		{"mixed case ASCII, case preserved", "API.Example.com", "API.Example.com"},
		{"Unicode label folds to Punycode", "münchen.example.com", "xn--mnchen-3ya.example.com"},
		{"CJK label folds to Punycode", "日本語.jp", "xn--wgv71a119e.jp"},
		{"colon: host:port is left alone", "münchen.example.com:8080", "münchen.example.com:8080"},
		{"colon: bracketed IPv6 is left alone", "[::1]", "[::1]"},
		{"a lone colon is left alone", ":", ":"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, ToASCII(tc.host))
		})
	}
}

// Test_ToASCII_RejectsByNonMatch pins the documented security default: input
// idna.Lookup refuses is returned as-is rather than surfacing an error, so it
// simply will not match any Punycode-encoded value a caller compares it to.
func Test_ToASCII_RejectsByNonMatch(t *testing.T) {
	t.Parallel()
	// A label starting with a combining mark is invalid under idna.Lookup's
	// validation (UTS46), so ToASCII must return it unchanged rather than a
	// converted-but-wrong value.
	const invalid = "́invalid.example.com"
	assert.Equal(t, invalid, ToASCII(invalid))
}

func Benchmark_ToASCII_ASCIIFastPath(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = ToASCII("example.com")
	}
}

func Benchmark_ToASCII_UnicodeSlowPath(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = ToASCII("münchen.example.com")
	}
}
