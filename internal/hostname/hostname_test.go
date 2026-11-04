package hostname

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_SplitPort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in, host, port string
	}{
		{"empty", "", "", ""},
		{"IPv4", "127.0.0.1", "127.0.0.1", ""},
		{"IPv4 with port", "127.0.0.1:3000", "127.0.0.1", "3000"},
		{"name", "example.com", "example.com", ""},
		{"name with port", "example.com:8080", "example.com", "8080"},
		{"localhost with port", "localhost:8080", "localhost", "8080"},
		{"port only", ":9090", "", "9090"},
		{"unix socket path", "/path/to/unix/socket", "/path/to/unix/socket", ""},
		{"no trim", " 127.0.0.1:8080 ", " 127.0.0.1", "8080 "},
		{"bare IPv6", "2001:db8::1", "2001:db8::1", ""},
		{"bare IPv6 loopback", "::1", "::1", ""},
		{"bracketed IPv6", "[::1]", "[::1]", ""},
		{"bracketed IPv6 with port", "[::1]:3000", "[::1]", "3000"},
		{"bracketed IPv6 zone", "[fe80::1%lo0]", "[fe80::1%lo0]", ""},
		{"bracketed IPv6 zone with port", "[fe80::1%lo0]:1234", "[fe80::1%lo0]", "1234"},
		{"literal ends at the first ]", "[::1]]:80", "[::1]", ""},
		{"text after ] is dropped", "[::1]x:80", "[::1]", ""},
		{"unclosed [ with two colons is whole", "[::1", "[::1", ""},
		{"unclosed [ with one colon splits", "[a:80", "[a", "80"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
		// A label starting with a combining mark is invalid under UTS46.
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
		{"invalid Punycode label, unchanged", "xn--a.example.com", "xn--a.example.com"},
		{"upper-case XN-- is not decoded", "XN--BCHER-KVA.EXAMPLE.COM", "XN--BCHER-KVA.EXAMPLE.COM"},
		// x/net quirk, not a contract: UTS #46 calls an empty decoded label an error. Update this row when x/net does.
		{"bare xn-- prefix, empty label from x/net", "xn--.example.com", ".example.com"},
		// "xn--" anywhere sends the whole host through idna.Lookup; without it nothing is mapped.
		{"xn-- substring maps the other labels", "\uff41pp.axn--b.example.com", "app.axn--b.example.com"},
		{"xn-- substring maps soft hyphens and dots", "s\u00adoft\uff0exn--bcher-kva.com", "soft.bücher.com"},
		{"no xn-- substring, nothing is mapped", "\uff41pp.example.com", "\uff41pp.example.com"},
		// One rejected label keeps the whole host, including the labels that would decode.
		{"every label accepted, decodes", "myapp.xn--bcher-kva.example.com", "myapp.bücher.example.com"},
		{"one label rejected, unchanged", "my_app.xn--bcher-kva.example.com", "my_app.xn--bcher-kva.example.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, ToUnicode(tc.host))
		})
	}
}

func Test_ToASCII_ToUnicode_RoundTrip(t *testing.T) {
	t.Parallel()
	ascii := ToASCII("bücher.münchen.de")
	assert.Equal(t, "xn--bcher-kva.xn--mnchen-3ya.de", ascii)
	assert.Equal(t, "bücher.münchen.de", ToUnicode(ascii))
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

func Benchmark_SplitPort_NoPort(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _ = SplitPort("api.example.com")
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
