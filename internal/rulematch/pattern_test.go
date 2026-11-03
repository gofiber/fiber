package rulematch

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_MustCompile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, from, input string
		compile           func(string) *regexp.Regexp
		want              []string
	}{
		{name: "literal dot", from: "/a.b", input: "/a.b", compile: MustCompilePath, want: []string{"/a.b"}},
		{name: "literal dot rejects other byte", from: "/a.b", input: "/aXb", compile: MustCompilePath},
		{name: "regexp dot", from: "/a.b", input: "/aXb", compile: MustCompileRegexp, want: []string{"/aXb"}},
		{
			name: "literal metacharacters", from: `/a+([b]){2}?^$|\`, input: `/a+([b]){2}?^$|\`,
			compile: MustCompilePath, want: []string{`/a+([b]){2}?^$|\`},
		},
		{name: "literal parentheses reject group match", from: "/a(b)", input: "/ab", compile: MustCompilePath},
		{name: "regexp group", from: "/a(b)", input: "/ab", compile: MustCompileRegexp, want: []string{"/ab", "b"}},
		{name: "literal alternation", from: "/a|/b", input: "/a|/b", compile: MustCompilePath, want: []string{"/a|/b"}},
		{name: "literal alternation rejects branch", from: "/a|/b", input: "/a", compile: MustCompilePath},
		{name: "regexp first branch", from: "/a|/b", input: "/a", compile: MustCompileRegexp, want: []string{"/a"}},
		{name: "regexp second branch", from: "/a|/b", input: "/b", compile: MustCompileRegexp, want: []string{"/b"}},
		{name: "regexp rejects branch suffix", from: "/a|/b", input: "/a/extra", compile: MustCompileRegexp},
		{name: "regexp rejects branch prefix", from: "/a|/b", input: "/extra/b", compile: MustCompileRegexp},
		{
			name: "regexp group before wildcard", from: "/(old)/*", input: "/old/a/b/",
			compile: MustCompileRegexp, want: []string{"/old/a/b/", "old", "a/b/"},
		},
		{
			name: "literal group before wildcard", from: "/(old)/*", input: "/(old)/a/b/",
			compile: MustCompilePath, want: []string{"/(old)/a/b/", "a/b/"},
		},
		{
			name: "literal backslash before wildcard", from: `/a\*`, input: `/a\b/c`,
			compile: MustCompilePath, want: []string{`/a\b/c`, "b/c"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.compile(tc.from).FindStringSubmatch(tc.input))
		})
	}
}

func Test_MustCompile_SharedMatching(t *testing.T) {
	t.Parallel()
	for _, syntax := range []struct {
		compile func(string) *regexp.Regexp
		name    string
	}{{MustCompilePath, "path"}, {MustCompileRegexp, "regexp"}} {
		t.Run(syntax.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name, from, input string
				want              []string
			}{
				{name: "empty pattern", want: []string{""}},
				{name: "exact path", from: "/old", input: "/old", want: []string{"/old"}},
				{name: "reject prefix", from: "/old", input: "/prefix/old"},
				{name: "reject suffix", from: "/old", input: "/old/suffix"},
				{name: "case sensitive", from: "/old", input: "/OLD"},
				{name: "empty wildcard", from: "/old/*", input: "/old/", want: []string{"/old/", ""}},
				{name: "wildcard crosses slashes", from: "/old/*", input: "/old/a/b/", want: []string{"/old/a/b/", "a/b/"}},
				{name: "wildcard excludes newline", from: "/old/*", input: "/old/a\nb"},
				{name: "reject final newline", from: "/old", input: "/old\n"},
				{name: "multiple captures", from: "/a/*/b/*", input: "/a/7/b/9", want: []string{"/a/7/b/9", "7", "9"}},
				{name: "adjacent wildcards", from: "/a/**", input: "/a/b", want: []string{"/a/b", "b", ""}},
				{name: "unicode", from: "/café/*", input: "/café/été", want: []string{"/café/été", "été"}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					require.Equal(t, tc.want, syntax.compile(tc.from).FindStringSubmatch(tc.input))
				})
			}
		})
	}
}

func Test_MustCompile_InvalidRegexp(t *testing.T) {
	t.Parallel()
	for _, from := range []string{"/old(", "/[", `/old\`, `/a\*`} {
		t.Run(from, func(t *testing.T) {
			t.Parallel()
			require.Panics(t, func() { MustCompileRegexp(from) })
			require.NotPanics(t, func() { MustCompilePath(from) })
		})
	}
}
