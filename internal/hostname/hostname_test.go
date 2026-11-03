package hostname

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_SplitPort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, host, port string
	}{
		{in: "[::1]:3000", host: "[::1]", port: "3000"},
		{in: "127.0.0.1:3000", host: "127.0.0.1", port: "3000"},
		{in: "[::1]", host: "[::1]", port: ""},
		{in: "2001:db8::1", host: "2001:db8::1", port: ""},
		{in: "::1", host: "::1", port: ""},
		{in: "/path/to/unix/socket", host: "/path/to/unix/socket", port: ""},
		{in: "127.0.0.1", host: "127.0.0.1", port: ""},
		{in: "localhost:8080", host: "localhost", port: "8080"},
		{in: "example.com", host: "example.com", port: ""},
		{in: "example.com:8080", host: "example.com", port: "8080"},
		{in: "[fe80::1%lo0]:1234", host: "[fe80::1%lo0]", port: "1234"},
		{in: "[fe80::1%lo0]", host: "[fe80::1%lo0]", port: ""},
		{in: ":9090", host: "", port: "9090"},
		{in: " 127.0.0.1:8080 ", host: "127.0.0.1", port: "8080"},
		{in: "", host: "", port: ""},
		// No closing bracket: not a bracketed literal, and with two colons
		// the value is returned whole like any bare IPv6 literal.
		{in: "[::1", host: "[::1", port: ""},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			host, port := SplitPort(tc.in)
			assert.Equal(t, tc.host, host)
			assert.Equal(t, tc.port, port)
		})
	}
}

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
		// The documented security default: input idna.Lookup refuses, such as a
		// label starting with a combining mark (invalid under UTS46), is returned
		// unchanged rather than surfacing an error or a converted-but-wrong value,
		// so it simply will not match any Punycode-encoded value it is compared to.
		{"rejects by non-match", "́invalid.example.com", "́invalid.example.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, ToASCII(tc.host))
		})
	}
}

func Test_ToUnicode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		host string
		want string
	}{
		{"empty", "", ""},
		{"no Punycode label, unchanged", "example.com", "example.com"},
		{"no Punycode label, case preserved", "API.Example.com", "API.Example.com"},
		{"Punycode label decodes", "xn--mnchen-3ya.example.com", "münchen.example.com"},
		{"CJK label decodes", "xn--wgv71a119e.jp", "日本語.jp"},
		{"round trip with ToASCII", ToASCII("bücher.münchen.de"), "bücher.münchen.de"},
		// Punycode idna.Lookup rejects is returned unchanged, mirroring ToASCII.
		{"invalid Punycode label, unchanged", "xn--a.example.com", "xn--a.example.com"},
		// idna.Lookup accepts a bare "xn--" prefix and decodes it to an empty
		// label; pinned as x/net's behavior, which Subdomains has always had.
		{"bare xn-- prefix decodes to an empty label", "xn--.example.com", ".example.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, ToUnicode(tc.host))
		})
	}
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

func Benchmark_SplitPort(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _ = SplitPort("api.example.com:8080")
	}
}

func Benchmark_SplitPort_IPv6(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _ = SplitPort("[2001:db8::1]:443")
	}
}

func Benchmark_ToUnicode_ASCIIFastPath(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = ToUnicode("api.example.com")
	}
}
