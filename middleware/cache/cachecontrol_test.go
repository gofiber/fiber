package cache

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseCacheControlDirectives_Elements(t *testing.T) {
	t.Parallel()
	type directive struct {
		key      string
		value    string
		hasValue bool
	}
	for _, tc := range []struct {
		input string
		want  []directive
	}{
		{input: ""},
		{input: " \t, ,\t,"},
		{input: strings.Repeat(",", 1024) + "public", want: []directive{{key: "public"}}},
		{input: " \t, ,\tmax-age=1 ,\t public\t,", want: []directive{{key: "max-age", value: "1", hasValue: true}, {key: "public"}}},
		{input: "Aa, aA=, Aa=\"\", =x,=", want: []directive{
			{key: "Aa"}, {key: "aA", hasValue: true}, {key: "Aa", hasValue: true}, {value: "x", hasValue: true}, {hasValue: true},
		}},
		{input: "\r\n\v\f, max-age=\v1\f", want: []directive{{key: "\r\n\v\f"}, {key: "max-age", value: "\v1\f", hasValue: true}}},
		{input: `ext="a\"b,c", public`, want: []directive{{key: "ext", value: `a"b,c`, hasValue: true}, {key: "public"}}},
		{input: `ext="a\\", public`, want: []directive{{key: "ext", value: `a\`, hasValue: true}, {key: "public"}}},
		{input: `a\, public\`, want: []directive{{key: `a\`}, {key: `public\`}}},
		{input: `ext="open, public`, want: []directive{{key: "ext", value: `"open, public`, hasValue: true}}},
		{input: `ext="a\`, want: []directive{{key: "ext", value: `"a\`, hasValue: true}}},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			var got []directive
			parseCacheControlDirectives([]byte(tc.input), func(key, value []byte) {
				got = append(got, directive{key: string(key), value: string(value), hasValue: value != nil})
			})
			require.Equal(t, tc.want, got)
		})
	}
}

// Test_parseResponseCacheControl_SmallestLifetimeCounts pins which of two lifetimes a response is
// stored for: the smallest that can be read, never longer than the first RFC 9111 Section 4.2.1
// allows, so that another one cannot lengthen it.
func Test_parseResponseCacheControl_SmallestLifetimeCounts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		input           string
		maxAge, sMaxAge uint64
		maxAgeSet       bool
		sMaxAgeSet      bool
	}{
		{input: "max-age=0, max-age=3600", maxAgeSet: true},
		{input: "max-age=3600, max-age=0", maxAgeSet: true},
		{input: "s-maxage=0, s-maxage=3600", sMaxAgeSet: true},
		{input: "max-age=5, s-maxage=7, max-age=9, s-maxage=11", maxAge: 5, maxAgeSet: true, sMaxAge: 7, sMaxAgeSet: true},
		// One that cannot be read is ignored: the one that can counts.
		{input: "max-age=soon, max-age=60", maxAge: 60, maxAgeSet: true},
		{input: "s-maxage=, s-maxage=60", sMaxAge: 60, sMaxAgeSet: true},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := parseResponseCacheControl([]byte(tc.input))
			require.Equal(t, tc.maxAgeSet, got.maxAgeSet)
			require.Equal(t, tc.maxAge, got.maxAge)
			require.Equal(t, tc.sMaxAgeSet, got.sMaxAgeSet)
			require.Equal(t, tc.sMaxAge, got.sMaxAge)
		})
	}
}
