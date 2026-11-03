package rulematch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_MustCompile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, from, input string
		want              []string
		syntax            Syntax
	}{
		{name: "literal dot", from: "/a.b", input: "/a.b", syntax: Literal, want: []string{"/a.b"}},
		{name: "literal dot rejects other byte", from: "/a.b", input: "/aXb", syntax: Literal},
		{name: "regexp dot", from: "/a.b", input: "/aXb", syntax: Regexp, want: []string{"/aXb"}},
		{
			name: "literal metacharacters", from: `/a+([b]){2}?^$|\`, input: `/a+([b]){2}?^$|\`,
			syntax: Literal, want: []string{`/a+([b]){2}?^$|\`},
		},
		{name: "literal parentheses reject group match", from: "/a(b)", input: "/ab", syntax: Literal},
		{name: "regexp group", from: "/a(b)", input: "/ab", syntax: Regexp, want: []string{"/ab", "b"}},
		{name: "literal alternation", from: "/a|/b", input: "/a|/b", syntax: Literal, want: []string{"/a|/b"}},
		{name: "literal alternation rejects branch", from: "/a|/b", input: "/a", syntax: Literal},
		{name: "regexp first branch", from: "/a|/b", input: "/a", syntax: Regexp, want: []string{"/a"}},
		{name: "regexp second branch", from: "/a|/b", input: "/b", syntax: Regexp, want: []string{"/b"}},
		{name: "regexp rejects branch suffix", from: "/a|/b", input: "/a/extra", syntax: Regexp},
		{name: "regexp rejects branch prefix", from: "/a|/b", input: "/extra/b", syntax: Regexp},
		{
			name: "regexp group before wildcard", from: "/(old)/*", input: "/old/a/b/",
			syntax: Regexp, want: []string{"/old/a/b/", "old", "a/b/"},
		},
		{
			name: "literal group before wildcard", from: "/(old)/*", input: "/(old)/a/b/",
			syntax: Literal, want: []string{"/(old)/a/b/", "a/b/"},
		},
		{
			name: "literal backslash before wildcard", from: `/a\*`, input: `/a\b/c`,
			syntax: Literal, want: []string{`/a\b/c`, "b/c"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, MustCompile(tc.from, tc.syntax).FindStringSubmatch(tc.input))
		})
	}
}

func Test_MustCompile_SharedMatching(t *testing.T) {
	t.Parallel()
	for _, syntax := range []struct {
		name  string
		value Syntax
	}{{"literal", Literal}, {"regexp", Regexp}} {
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
					require.Equal(t, tc.want, MustCompile(tc.from, syntax.value).FindStringSubmatch(tc.input))
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
			require.Panics(t, func() { MustCompile(from, Regexp) })
			require.NotPanics(t, func() { MustCompile(from, Literal) })
		})
	}
}
