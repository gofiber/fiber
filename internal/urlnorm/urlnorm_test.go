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

// Test_Policies keeps the shared character rules separate from the decisions
// about trimming, authority preservation and forcing a local path.
func Test_Policies(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input, stripped, browser, referer, rooted string
	}{
		{"empty", "", "", "", "", "/"},
		{"plain path", "/a/b", "/a/b", "/a/b", "/a/b", "/a/b"},
		{"surrounding controls", " \t/a\r\n ", " /a ", "/a", "/a", "/ /a "},
		{"authority", "///other.test/x", "///other.test/x", "///other.test/x", "//other.test/x", "/other.test/x"},
		{"only slashes", `\\//`, `\\//`, `\\//`, "/", "/"},
		{"query and fragment", `/a\b?next=\c#part\d`, `/a\b?next=\c#part\d`, `/a\b?next=\c#part\d`, `/a/b?next=\c#part\d`, `/a\b?next=\c#part\d`},
		{"fragment without query", `/a\b#part\c`, `/a\b#part\c`, `/a\b#part\c`, `/a/b#part\c`, `/a\b#part\c`},
		{"controls in query", "/a?x=1\t2#z\r\n", "/a?x=12#z", "/a?x=12#z", "/a?x=12#z", "/a?x=12#z"},
		{"first change after query", "/a?x=1\t\\b#part\\c", `/a?x=1\b#part\c`, `/a?x=1\b#part\c`, `/a?x=1\b#part\c`, `/a?x=1\b#part\c`},
		{"first change after fragment", "/a#part\t\\b", `/a#part\b`, `/a#part\b`, `/a#part\b`, `/a#part\b`},
		{"percent escapes", "/%5C?q=%09#%0A", "/%5C?q=%09#%0A", "/%5C?q=%09#%0A", "/%5C?q=%09#%0A", "/%5C?q=%09#%0A"},
		{"interior C0", "\x00/a\x1fb\x00", "\x00/a\x1fb\x00", "/a\x1fb", "/a\x1fb", "/\x00/a\x1fb\x00"},
		{"invalid UTF-8", "/caf\xe9\\x?q=\xff\\z", "/caf\xe9\\x?q=\xff\\z", "/caf\xe9\\x?q=\xff\\z", "/caf\xe9/x?q=\xff\\z", "/caf\xe9\\x?q=\xff\\z"},
		{"non-ASCII space", "\u00a0/a\u00a0", "\u00a0/a\u00a0", "\u00a0/a\u00a0", "\u00a0/a\u00a0", "/\u00a0/a\u00a0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.stripped, StripTabCRLF(tc.input))
			require.Equal(t, tc.browser, AsBrowserReads(tc.input))
			require.Equal(t, tc.referer, Referer(tc.input))
			require.Equal(t, tc.rooted, RootedPath(tc.input))
		})
	}
}

func Benchmark_Normalization(b *testing.B) {
	for _, tc := range []struct{ name, input string }{
		{"plain", "/dashboard/settings"},
		{"absolute", "https://example.com/account/settings?from=profile#details"},
		{"query-backslashes", `/search?path=C:\Users\me#part\x`},
		{"mixed", " \t/\\example.com/a\r?next=\\keep\n"},
	} {
		for _, normalizer := range []struct {
			fn   func(string) string
			name string
		}{
			{StripTabCRLF, "StripTabCRLF"},
			{AsBrowserReads, "AsBrowserReads"},
			{Referer, "Referer"},
			{RootedPath, "RootedPath"},
		} {
			b.Run(tc.name+"/"+normalizer.name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					normalizer.fn(tc.input)
				}
			})
		}
	}
}
