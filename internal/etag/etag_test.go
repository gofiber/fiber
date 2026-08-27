package etag

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Parse(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		tag   string
		value string
		weak  bool
		ok    bool
	}{
		{name: "strong tag", tag: `"abc"`, value: "abc", ok: true},
		{name: "weak tag", tag: `W/"abc"`, value: "abc", weak: true, ok: true},
		{name: "empty opaque-tag", tag: `""`, value: "", ok: true},
		{name: "comma inside opaque-tag", tag: `"a,b"`, value: "a,b", ok: true},
		{name: "unquoted", tag: `abc`},
		{name: "missing closing quote", tag: `"abc`},
		{name: "missing opening quote", tag: `abc"`},
		{name: "single quote", tag: `"`},
		{name: "empty", tag: ``},
		{name: "weak prefix only", tag: `W/`, weak: true},
		// The weak indicator is case-sensitive per RFC 9110 Section 8.8.3.
		{name: "lowercase weak indicator", tag: `w/"abc"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			value, weak, ok := Parse(tc.tag)
			require.Equal(t, tc.value, value)
			require.Equal(t, tc.weak, weak)
			require.Equal(t, tc.ok, ok)
		})
	}
}

func Test_Match(t *testing.T) {
	t.Parallel()

	require.True(t, Match(`"a"`, `"a"`))
	require.True(t, Match(`W/"a"`, `"a"`))
	require.True(t, Match(`"a"`, `W/"a"`))
	require.True(t, Match(`W/"a"`, `W/"a"`))
	require.False(t, Match(`"a"`, `"b"`))
	require.False(t, Match(`a`, `"a"`))
	require.False(t, Match(`"a"`, `b`))
	require.False(t, Match(``, `"a"`))
	require.False(t, Match(`"a"`, ``))
	require.False(t, Match(`W/`, `"a"`))
}

func Test_MatchStrong(t *testing.T) {
	t.Parallel()

	require.True(t, MatchStrong(`"a"`, `"a"`))
	require.False(t, MatchStrong(`W/"a"`, `"a"`))
	require.False(t, MatchStrong(`"a"`, `W/"a"`))
	require.False(t, MatchStrong(`W/"a"`, `W/"a"`))
	require.False(t, MatchStrong(`"a"`, `"b"`))
	require.False(t, MatchStrong(`a`, `"a"`))
	require.False(t, MatchStrong(`"a"`, `b`))
}

func Test_AnyMatch(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		header string
		etag   string
		want   bool
	}{
		{name: "empty header", header: ``, etag: `"a"`},
		{name: "single match", header: `"a"`, etag: `"a"`, want: true},
		{name: "single mismatch", header: `"b"`, etag: `"a"`},
		{name: "wildcard", header: `*`, etag: `"a"`, want: true},
		{name: "wildcard with surrounding space", header: "  *\t", etag: `"a"`, want: true},
		// RFC 9110 Section 13.1.2 allows "*" only as the sole field value, so
		// inside a list it is an ordinary, unquoted and therefore invalid tag.
		{name: "wildcard inside a list", header: `*, "b"`, etag: `"a"`},
		{name: "list, first matches", header: `"a", "b"`, etag: `"a"`, want: true},
		{name: "list, last matches", header: `"a", "b"`, etag: `"b"`, want: true},
		{name: "list, none match", header: `"a", "b"`, etag: `"c"`},
		{name: "list without OWS", header: `"a","b"`, etag: `"b"`, want: true},
		{name: "weak tag in list", header: `"a", W/"b"`, etag: `"b"`, want: true},
		{name: "weak response tag", header: `"a", "b"`, etag: `W/"b"`, want: true},
		{name: "unquoted response tag", header: `"a"`, etag: `a`},
		// A comma is a legal etagc byte, so `"a,b"` is one entity tag. Splitting
		// on every comma would compare the fragments `"a` and `b"` instead.
		{name: "comma inside opaque-tag", header: `"a,b"`, etag: `"a,b"`, want: true},
		{name: "comma inside opaque-tag in a list", header: `"x", "a,b"`, etag: `"a,b"`, want: true},
		{name: "comma inside opaque-tag, later element matches", header: `"a,b", "c"`, etag: `"c"`, want: true},
		{name: "fragment of a split tag never matches", header: `"a,b"`, etag: `"a`},
		{name: "unbalanced quote", header: `"a, "b`, etag: `"a"`},
		{name: "unbalanced quote, no match on remainder", header: `"a, "b`, etag: `"b"`},
		{name: "only a comma", header: `,`, etag: `"a"`},
		{name: "empty list element", header: `"a", , "b"`, etag: `"b"`, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, AnyMatch(tc.header, tc.etag))
		})
	}
}

func Test_Split(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   []string
	}{
		{name: "empty", header: ""},
		{name: "whitespace only", header: "   "},
		{name: "single", header: `"abc"`, want: []string{`"abc"`}},
		{name: "list", header: `"a", "b"`, want: []string{`"a"`, `"b"`}},
		{name: "weak", header: `W/"a", "b"`, want: []string{`W/"a"`, `"b"`}},
		{name: "wildcard", header: "*", want: []string{"*"}},
		{name: "comma inside tag", header: `"v1,v2"`, want: []string{`"v1,v2"`}},
		{name: "comma inside and after", header: `"v1,v2", "v3"`, want: []string{`"v1,v2"`, `"v3"`}},
		{name: "untrimmed", header: `  "a" ,  "b"  `, want: []string{`"a"`, `"b"`}},
		{name: "unquoted", header: `abc`, want: []string{"abc"}},
		{name: "trailing comma", header: `"a",`, want: []string{`"a"`}},
		{name: "trailing comma and space", header: `"a", `, want: []string{`"a"`}},
		{name: "leading comma", header: `,"a"`, want: []string{`"a"`}},
		{name: "doubled comma", header: `"a",,"b"`, want: []string{`"a"`, `"b"`}},
		{name: "only commas", header: `,,`},
		{name: "only a comma", header: `,`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, Split(tc.header))
		})
	}
}

func Test_AnyMatch_IgnoresEmptyElements(t *testing.T) {
	t.Parallel()

	require.True(t, AnyMatch(`"a",`, `"a"`))
	require.True(t, AnyMatch(`,"a"`, `"a"`))
	require.True(t, AnyMatch(`"a",,"b"`, `"b"`))
	require.False(t, AnyMatch(`,,`, `"a"`))
	require.False(t, AnyMatch(`"a",`, ""))
	require.False(t, AnyMatch(`,`, ""))
}

func Benchmark_AnyMatch(b *testing.B) {
	benchmarks := []struct {
		name   string
		header string
		etag   string
	}{
		{name: "hit", header: `"abc123"`, etag: `"abc123"`},
		{name: "miss", header: `"abc123"`, etag: `"xyz789"`},
		{name: "list_miss", header: `"a", "b", "c", "d", "e"`, etag: `"z"`},
		{name: "weak", header: `W/"abcdefghijklmnop"`, etag: `W/"abcdefghijklmnop"`},
		{name: "empty", header: "", etag: `"a"`},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = AnyMatch(bm.header, bm.etag)
			}
		})
	}
}

func Benchmark_Split(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = Split(`"a", "b", "c"`)
	}
}
