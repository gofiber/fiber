package headerlist

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Append(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		description  string
		headerValue  string
		expectedList []string
	}{
		{
			description:  "normal case",
			headerValue:  "gzip, deflate,br",
			expectedList: []string{"gzip", "deflate", "br"},
		},
		{
			description:  "no matter the value",
			headerValue:  "   gzip,deflate, br, zip",
			expectedList: []string{"gzip", "deflate", "br", "zip"},
		},
		{
			description:  "comma with trailing spaces around values",
			headerValue:  "gzip , br",
			expectedList: []string{"gzip", "br"},
		},
		{
			description:  "comma with tabbed whitespace",
			headerValue:  "gzip\t,br",
			expectedList: []string{"gzip", "br"},
		},
		{
			description:  "headerValue is empty",
			headerValue:  "",
			expectedList: nil,
		},
		{
			// RFC 9110 §5.6.1.2: empty list elements are parsed and ignored.
			description:  "has a comma without element",
			headerValue:  "gzip,",
			expectedList: []string{"gzip"},
		},
		{
			description:  "has a space between words",
			headerValue:  "  foo bar, hello  world",
			expectedList: []string{"foo bar", "hello  world"},
		},
		{
			description:  "single comma",
			headerValue:  ",",
			expectedList: []string{},
		},
		{
			description:  "multiple comma",
			headerValue:  ",,",
			expectedList: []string{},
		},
		{
			description:  "comma with space",
			headerValue:  ",  ,",
			expectedList: []string{},
		},
		{
			description:  "empty element between values",
			headerValue:  "gzip, , br",
			expectedList: []string{"gzip", "br"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tc := tc // create a new 'tc' variable for the goroutine
			t.Parallel()
			dst := make([]string, 10)
			result := Append(dst, tc.headerValue)
			require.Equal(t, tc.expectedList, result)
		})
	}
}

func Benchmark_Append(b *testing.B) {
	destination := make([]string, 5)
	result := destination
	const input = `deflate, gzip,br,brotli,zstd`
	b.ReportAllocs()
	for b.Loop() {
		result = Append(destination, input)
	}
	require.Equal(b, []string{"deflate", "gzip", "br", "brotli", "zstd"}, result)
}

func Test_Contains(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		header   string
		value    string
		expected bool
	}{
		// Exact match
		{header: "gzip", value: "gzip", expected: true},
		{header: "gzip", value: "deflate", expected: false},
		// Prefix match (value at start with comma)
		{header: "gzip, deflate", value: "gzip", expected: true},
		{header: "gzip,deflate", value: "gzip", expected: true},
		// Suffix match (value at end)
		{header: "deflate, gzip", value: "gzip", expected: true},
		{header: "deflate,gzip", value: "gzip", expected: true}, // No space - OWS is optional per RFC 9110
		{header: "br, gzip", value: "gzip", expected: true},
		// Middle match (value in middle)
		{header: "deflate, gzip, br", value: "gzip", expected: true},
		{header: "deflate,gzip,br", value: "gzip", expected: true}, // No spaces - OWS is optional per RFC 9110
		// No match - similar but not equal
		{header: "gzip2", value: "gzip", expected: false},
		{header: "2gzip", value: "gzip", expected: false},
		{header: "gzip2, deflate", value: "gzip", expected: false},
		// Whitespace handling (OWS per RFC 9110)
		{header: "  gzip  ,  deflate  ", value: "gzip", expected: true},
		{header: "deflate,  gzip  ", value: "gzip", expected: true},
		// Empty cases
		{header: "", value: "gzip", expected: false},
		{header: "gzip", value: "", expected: false},
		{header: "", value: "", expected: false}, // Both empty - should return false
	}

	for _, tc := range testCases {
		result := Contains(tc.header, tc.value)
		require.Equal(t, tc.expected, result,
			"Contains(%q, %q) = %v, want %v",
			tc.header, tc.value, result, tc.expected)
	}
}

// go test -v -run=^$ -bench=Benchmark_Contains -benchmem -count=4
func Benchmark_Contains(b *testing.B) {
	var ok bool
	b.ReportAllocs()
	for b.Loop() {
		_ = Contains("gzip", "gzip")
		_ = Contains("gzip, deflate, br", "deflate")
		_ = Contains("deflate, gzip", "gzip")
		ok = Contains("deflate, gzip, br", "gzip")
	}
	require.True(b, ok)
}

func Test_Join(t *testing.T) {
	t.Parallel()
	require.Nil(t, Join(nil))
	require.Equal(t, []byte("a"), Join([][]byte{[]byte("a")}))
	require.Equal(t, []byte("a,b"), Join([][]byte{[]byte("a"), []byte("b")}))
}

func collect(seq func(func(string) bool)) []string {
	var got []string
	for v := range seq {
		got = append(got, v)
	}
	return got
}

func Test_All(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		list string
		want []string
	}{
		{"empty", "", nil},
		{"single", "gzip", []string{"gzip"}},
		{"trims OWS", "  gzip ,\tbr\t", []string{"gzip", "br"}},
		{"skips empty elements", "gzip, , br", []string{"gzip", "br"}},
		{"only separators", ",  ,", nil},
		{"quotes are not special", `"a,b"`, []string{`"a`, `b"`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, collect(All(tc.list)))
		})
	}
}

func Test_AllQuoted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		list string
		want []string
	}{
		{"empty", "", nil},
		{"comma inside quotes is kept", `"v1,v2"`, []string{`"v1,v2"`}},
		{"separates outside quotes", `"a", "b"`, []string{`"a"`, `"b"`}},
		{"weak tags", `W/"a", W/"b,c"`, []string{`W/"a"`, `W/"b,c"`}},
		{"unterminated quote swallows rest", `"a, b`, []string{`"a, b`}},
		{"skips empty elements", `"a", , "b"`, []string{`"a"`, `"b"`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, collect(AllQuoted(tc.list)))
		})
	}
}

func Test_AllLines(t *testing.T) {
	t.Parallel()
	require.Nil(t, collect(AllLines(nil)))
	require.Equal(t,
		[]string{"gzip", "br", "zstd"},
		collect(AllLines([][]byte{[]byte("gzip, br"), []byte(" zstd ")})),
	)
	// A header split across lines reads the same as one line (RFC 9110 5.3).
	require.Equal(t,
		collect(All("gzip,br")),
		collect(AllLines([][]byte{[]byte("gzip"), []byte("br")})),
	)
}

func Test_All_EarlyExit(t *testing.T) {
	t.Parallel()
	// Each iterator must stop when the caller breaks, not run to completion.
	for _, seq := range []func(func(string) bool){
		All("a,b,c"),
		AllQuoted(`"a","b","c"`),
		AllLines([][]byte{[]byte("a"), []byte("b,c")}),
	} {
		n := 0
		for range seq {
			n++
			break
		}
		require.Equal(t, 1, n)
	}
}

func Test_ContainsFold(t *testing.T) {
	t.Parallel()
	require.True(t, ContainsFold("gzip, BR", "br"))
	require.True(t, ContainsFold("GZIP", "gzip"))
	require.True(t, ContainsFold(" close ", "CLOSE"))
	require.False(t, ContainsFold("gzip2", "gzip"))
	require.False(t, ContainsFold("gzip", ""))
	require.False(t, ContainsFold("", "gzip"))
}

func Test_Contains_MatchesElementsNotRawList(t *testing.T) {
	t.Parallel()
	// Contains compares against what All yields, so OWS around the element is
	// not part of it. A value carrying its own OWS is not an element, and that
	// holds whether or not it happens to span the whole list.
	require.True(t, Contains(" gzip ", "gzip"))
	require.False(t, Contains(" gzip ", " gzip "))
	require.False(t, Contains("br, gzip", " gzip "))
	require.Equal(t, []string{"gzip"}, collect(All(" gzip ")))
}

func Test_Contains_IsCaseSensitive(t *testing.T) {
	t.Parallel()
	// The documented split from ContainsFold: Contains compares byte for byte.
	require.False(t, Contains("Accept-Encoding", "accept-encoding"))
	require.True(t, ContainsFold("Accept-Encoding", "accept-encoding"))
}

func Test_AppendUnique(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		list   string
		want   string
		values []string
	}{
		{"into empty", "", "Accept", []string{"Accept"}},
		{"appends new", "Accept", "Accept, Origin", []string{"Origin"}},
		{"skips present", "Accept, Origin", "", []string{"Accept"}},
		{"skips empty values", "Accept", "", []string{""}},
		{"nothing to add", "Accept", "", nil},
		{"partial add", "Accept", "Accept, Origin", []string{"Accept", "Origin"}},
		{"case counts as different", "Accept", "Accept, accept", []string{"accept"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, AppendUnique(tc.list, tc.values))
		})
	}
}

func Test_Join_AliasesSingleLine(t *testing.T) {
	t.Parallel()
	line := []byte("gzip")
	require.Equal(t, &line[0], &Join([][]byte{line})[0], "a lone line must be returned as it stands")
}

func Test_Append_ReusesStorage(t *testing.T) {
	t.Parallel()
	dst := make([]string, 0, 4)
	got := Append(dst, "a, b")
	require.Equal(t, []string{"a", "b"}, got)
	require.Equal(t, &dst[:1][0], &got[0], "Append must reuse dst's storage")
	require.Nil(t, Append(dst, ""), "an empty list yields nil")
}
