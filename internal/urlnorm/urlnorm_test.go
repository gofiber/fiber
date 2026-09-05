package urlnorm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Test_RootedPath pins the one shape a URL composed from a named route may take:
// a path on the origin now being served, opening exactly one slash. Two would
// open an authority, so "//evil.com" resolves against "https://good.example" as
// "https://evil.com" — a value spliced at the front must never reach that.
func Test_RootedPath(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		location string
		want     string
	}{
		{"ordinary path", "/user/john", "/user/john"},
		{"value adds a slash", "//evil.com", "/evil.com"},
		{"value adds a backslash", `/\evil.com`, "/evil.com"},
		{"value adds several", "///evil.com", "/evil.com"},
		{"value is only slashes", "//", "/"},
		{"backslash is folded", `\internal`, "/internal"},
		// Tab, LF and CR go first, or one hides behind the leading slash.
		{"control bytes cannot hide a slash", "/\t/evil.com", "/evil.com"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, RootedPath(tc.location))
		})
	}
}

func Test_LeadingSlashes(t *testing.T) {
	t.Parallel()

	require.Equal(t, 0, LeadingSlashes("internal"))
	require.Equal(t, 1, LeadingSlashes("/internal"))
	require.Equal(t, 2, LeadingSlashes("//internal"))
	require.Equal(t, 2, LeadingSlashes(`/\internal`))
}
