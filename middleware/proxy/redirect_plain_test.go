package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Test_PlainAbsoluteURL pins which Locations resolveRedirect validates without resolving them.
func Test_PlainAbsoluteURL(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		location string
		want     bool
	}{
		{"http://example.com", true},
		{"https://example.com/", true},
		{"https://example.com:8443/a/b?x=1&y=%2F", true},
		{"https://example.com/a%2Fb/c//d/", true},
		{"https://example.com//a", true},
		{"https://example.com?x=1", true},
		{"https://example.com/x?", true},
		{"https://example.com/a%2e%2e/b", true},
		{"https://example.com/a..b/.c/d.", true},
		{"https://example.com/x?a=%zz", true},
		{"", false},
		{"http://", false},
		{"https:///x", false},
		{"/a/b", false},
		{"//example.com/", false},
		{"HTTP://example.com/", false},
		{"ftp://example.com/", false},
		{"https://u:p@example.com/", false},
		{"https://example.com:/", false},
		{"https://example.com:80a/", false},
		{"https://[::1]/", false},
		{"https://exa_mple.com/", false},
		{"https://example.com/a/./b", false},
		{"https://example.com/a/../b", false},
		{"https://example.com/a/..", false},
		{"https://example.com/.", false},
		{"https://example.com/100%", false},
		{"https://example.com/%zz", false},
		{"https://example.com/a b", false},
		{"https://example.com/a|b", false},
		{"https://example.com/é", false},
		{"https://example.com/x#frag", false},
		{"https://example.com/x?a#b", false},
		{"https://example.com/x?a b", false},
	} {
		require.Equal(t, tc.want, plainAbsoluteURL([]byte(tc.location)), tc.location)
	}
}

// Test_SchemeOf pins the scheme resolveRedirect reads off the current URL to tell an https to http downgrade.
func Test_SchemeOf(t *testing.T) {
	t.Parallel()
	for ref, want := range map[string]string{
		"https://example.com/": "https", "HTTP://example.com/": "HTTP", "a+b.c-d:rest": "a+b.c-d",
		"/a/b": "", "//example.com/": "", "": "", "1http://x": "", ":x": "",
	} {
		require.Equal(t, want, schemeOf(ref), ref)
	}
}

// Test_PlainAbsoluteURL_IsWhatResolvingWrites pins that a Location plainAbsoluteURL accepts is written back as it is
// by resolveLocation, which is what lets resolveRedirect validate it without resolving it.
func Test_PlainAbsoluteURL_IsWhatResolvingWrites(t *testing.T) {
	t.Parallel()
	policy := proxyTestAllowPrivatePolicy()
	hosts := []string{"example.com", "Example.COM", "a-b.c", "127.0.0.1", "example.com:8080", "h:1"}
	paths := []string{
		"", "/", "//", "/a", "/a/", "/a//b", "//a", "/a/b/c", "/%2F", "/a%2fb", "/%41", "/%7C", "/a%2e%2eb", "/%2e",
		"/..a", "/a..", "/.a", "/a.", "/~x", "/a;b=c,d", "/a:b@c", "/!$&'()*+,;=", "/./a", "/a/./b", "/a/../b", "/a/..",
		"/a/.", "/..", "/.", "/%zz", "/a|b", "/100%",
	}
	queries := []string{"", "?", "?x=1", "?x=%2F&y=a+b", "?a=%zz", "?;", "?x=/../y", "?x#y", "#f"}
	plain := 0
	for _, scheme := range []string{"http", "https"} {
		for _, host := range hosts {
			for _, path := range paths {
				for _, query := range queries {
					location := scheme + "://" + host + path + query
					if !plainAbsoluteURL([]byte(location)) {
						continue
					}
					plain++
					long, err := resolveLocation("https://example.com/base", []byte(location), policy, nil)
					require.NoError(t, err, location)
					require.Equal(t, location, long.String())
					short, err := validateUpstream(location, policy, nil)
					require.NoError(t, err, location)
					require.Equal(t, long.String(), short.String(), location)
				}
			}
		}
	}
	require.Greater(t, plain, 500, "the generated Locations are mostly plain ones")
}
