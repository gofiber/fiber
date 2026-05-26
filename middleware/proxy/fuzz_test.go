package proxy

import (
	"net/url"
	"strings"
	"testing"
)

// FuzzValidateUpstream stresses the upstream URL parser with adversarial
// inputs. The contract: validateUpstream must never panic, must never
// return a URL whose scheme is outside the configured allowlist, and
// must never return a URL with an empty host. Private-IP filtering is
// covered by Test_Security_ValidateUpstream_BlocksPrivateIPs.
func FuzzValidateUpstream(f *testing.F) {
	seeds := []string{
		"http://example.com",
		"https://example.com/path?q=1",
		"//attacker.example",
		"http:///etc/passwd",
		"http://user:pass@example.com",
		"http://example.com:65536",
		"http:// /space",
		"http://[::1]",
		"http://127.0.0.1\r\nX-Injected: 1",
		"\x00http://example.com",
		"http://example.com#frag",
		"gopher://example.com",
		"file:///etc/shadow",
		"http://example.com/\xff\xfe",
		"javascript:alert(1)",
		string([]byte{0x7f, 0x00, 0x01}),
		"",
		strings.Repeat("a", 8192),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true // focus on scheme/host parsing, not SSRF here

	f.Fuzz(func(t *testing.T, raw string) {
		u, err := validateUpstream(raw, policy)
		if err != nil {
			return
		}
		// validateUpstream's contract is (non-nil, nil) on success — assert
		// it directly so the dereferences below are unambiguous.
		require := func(cond bool, format string, args ...any) {
			t.Helper()
			if !cond {
				t.Fatalf(format, args...)
			}
		}
		require(u != nil, "nil URL with no error for input %q", raw)
		require(u.Host != "", "accepted URL with empty host: input=%q url=%q", raw, u.String())
		require(schemeAllowed(u.Scheme, policy.AllowedSchemes), "accepted disallowed scheme %q for input %q", u.Scheme, raw)
	})
}

// FuzzJoinUpstreamPath verifies that the path-concatenation helper used
// by DomainForward and BalancerForward never lets a caller-controlled
// request path change the upstream host or scheme.
func FuzzJoinUpstreamPath(f *testing.F) {
	seeds := []string{
		"/",
		"/foo",
		"/foo?q=1",
		"//attacker.example/foo",
		"////attacker.example/foo",
		"/\\attacker.example",
		"/?@evil.com",
		"@evil.com",
		"https://evil.com/path",
		"",
		"#fragment",
		"%2F%2Fevil.com/path",
		"/\x00null",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	base, err := url.Parse("http://upstream.invalid")
	if err != nil {
		f.Fatal(err)
	}

	f.Fuzz(func(t *testing.T, requestPath string) {
		out := joinUpstreamPath(base, requestPath)
		if out == "" {
			return
		}
		parsed, err := url.Parse(out)
		if err != nil {
			// joinUpstreamPath should produce a parsable URL; this is a
			// regression.
			t.Fatalf("unparsable output %q for input %q: %v", out, requestPath, err)
		}
		if parsed.Scheme != "http" {
			t.Fatalf("scheme drifted from http to %q (input=%q out=%q)", parsed.Scheme, requestPath, out)
		}
		if parsed.Host != "upstream.invalid" {
			t.Fatalf("host changed from upstream.invalid to %q (input=%q out=%q)", parsed.Host, requestPath, out)
		}
	})
}

// FuzzConnectionListedHeaders ensures the RFC 7230 Connection-header
// parser tolerates pathological inputs (excess whitespace, embedded
// commas, control bytes) without panicking.
func FuzzConnectionListedHeaders(f *testing.F) {
	seeds := []string{
		"keep-alive",
		"close",
		"upgrade, keep-alive",
		"  X-Foo  ,  X-Bar  ",
		",,,,",
		"x-custom\x00",
		strings.Repeat("X-A,", 200),
		"",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(_ *testing.T, v string) {
		_ = connectionListedHeaders([][]byte{[]byte(v)})
	})
}
