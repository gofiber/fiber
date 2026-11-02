package headerlist

import (
	"iter"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_NextPrev(t *testing.T) {
	t.Parallel()
	for _, tc := range []yieldCase{
		{name: "empty", list: "", want: []string{""}},
		{name: "single", list: "203.0.113.50", want: []string{"203.0.113.50"}},
		{name: "empty elements", list: ",a,,,b,", want: []string{"", "a", "", "", "b", ""}},
		{name: "only separators", list: ",,", want: []string{"", "", ""}},
		{name: "raw whitespace", list: " \ta\t,\vb\r\n, ", want: []string{" \ta\t", "\vb\r\n", " "}},
		{name: "quotes are literal", list: `"a,b",c`, want: []string{`"a`, `b"`, "c"}},
		{name: "raw bytes", list: "\x00\xff,é,::ffff:192.0.2.1", want: []string{"\x00\xff", "é", "::ffff:192.0.2.1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var forward, backward []string
			for start := 0; start <= len(tc.list); {
				element, next := Next(tc.list, start)
				require.Greater(t, next, start)
				forward = append(forward, element)
				start = next
			}
			for end := len(tc.list); end >= 0; {
				element, prev := Prev(tc.list, end)
				require.Less(t, prev, end)
				backward = append(backward, element)
				end = prev
			}
			require.Equal(t, tc.want, forward)
			slices.Reverse(backward)
			require.Equal(t, tc.want, backward)
		})
	}
}

func Test_Append(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name string
		list string
		want []string
	}{
		{"normal case", "gzip, deflate,br", []string{"gzip", "deflate", "br"}},
		{"no matter the value", "   gzip,deflate, br, zip", []string{"gzip", "deflate", "br", "zip"}},
		{"comma with trailing spaces around values", "gzip , br", []string{"gzip", "br"}},
		{"comma with tabbed whitespace", "gzip\t,br", []string{"gzip", "br"}},
		{"headerValue is empty", "", nil},
		// RFC 9110 §5.6.1.2: empty list elements are parsed and ignored.
		{"has a comma without element", "gzip,", []string{"gzip"}},
		{"has a space between words", "  foo bar, hello  world", []string{"foo bar", "hello  world"}},
		{"single comma", ",", []string{}},
		{"multiple comma", ",,", []string{}},
		{"comma with space", ",  ,", []string{}},
		{"empty element between values", "gzip, , br", []string{"gzip", "br"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, Append(make([]string, 10), tc.list))
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

// Test_Contains runs every case through Contains, which compares byte for
// byte, and through ContainsFold, which folds ASCII case.
func Test_Contains(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		header string
		value  string
		want   bool // from Contains
		fold   bool // from ContainsFold
	}{
		// Exact match
		{name: "exact match", header: "gzip", value: "gzip", want: true, fold: true},
		{name: "exact mismatch", header: "gzip", value: "deflate"},
		// Prefix match (value at start with comma)
		{name: "prefix", header: "gzip, deflate", value: "gzip", want: true, fold: true},
		{name: "prefix without OWS", header: "gzip,deflate", value: "gzip", want: true, fold: true},
		// Suffix match (value at end); OWS is optional per RFC 9110
		{name: "suffix", header: "deflate, gzip", value: "gzip", want: true, fold: true},
		{name: "suffix without OWS", header: "deflate,gzip", value: "gzip", want: true, fold: true},
		{name: "suffix after br", header: "br, gzip", value: "gzip", want: true, fold: true},
		// Middle match (value in middle)
		{name: "middle", header: "deflate, gzip, br", value: "gzip", want: true, fold: true},
		{name: "middle without OWS", header: "deflate,gzip,br", value: "gzip", want: true, fold: true},
		// No match - similar but not equal
		{name: "longer element", header: "gzip2", value: "gzip"},
		{name: "element with a prefix", header: "2gzip", value: "gzip"},
		{name: "longer element in a list", header: "gzip2, deflate", value: "gzip"},
		// Whitespace handling (OWS per RFC 9110)
		{name: "OWS around every element", header: "  gzip  ,  deflate  ", value: "gzip", want: true, fold: true},
		{name: "OWS around the last element", header: "deflate,  gzip  ", value: "gzip", want: true, fold: true},
		// Contains compares against what All yields, so OWS around the element is
		// not part of it. A value carrying its own OWS is not an element, and that
		// holds whether or not it happens to span the whole list.
		{name: "OWS around a lone element", header: " gzip ", value: "gzip", want: true, fold: true},
		{name: "value with OWS spanning the list", header: " gzip ", value: " gzip "},
		{name: "value with OWS", header: "br, gzip", value: " gzip "},
		// Empty cases: an empty value is never present
		{name: "empty list", header: "", value: "gzip"},
		{name: "empty value", header: "gzip", value: ""},
		{name: "both empty", header: "", value: ""},
		// The documented split from ContainsFold: Contains compares byte for byte.
		{name: "other case in a list", header: "gzip, BR", value: "br", fold: true},
		{name: "upper case element", header: "GZIP", value: "gzip", fold: true},
		{name: "upper case value", header: " close ", value: "CLOSE", fold: true},
		{name: "field name in another case", header: "Accept-Encoding", value: "accept-encoding", fold: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, Contains(tc.header, tc.value), "Contains(%q, %q)", tc.header, tc.value)
			require.Equal(t, tc.fold, ContainsFold(tc.header, tc.value), "ContainsFold(%q, %q)", tc.header, tc.value)
		})
	}
}

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
	line := []byte("gzip")
	require.Same(t, &line[0], &Join([][]byte{line})[0], "a lone line must be returned as it stands")
}

func Test_JoinNext(t *testing.T) {
	t.Parallel()
	lines := [][]byte{[]byte("a"), []byte(""), []byte("b")}
	var combined []byte
	var multiple bool
	for i, line := range lines {
		combined, multiple = JoinNext(combined, line, multiple)
		require.Equal(t, i > 0, multiple)
		if i == 0 {
			require.Same(t, &line[0], &combined[0], "one line must alias the header")
		}
	}
	require.Equal(t, Join(lines), combined)
	require.Equal(t, []byte("a,,b"), combined, "empty field lines keep their position")
}

func collect(seq func(func(string) bool)) []string {
	var got []string
	for v := range seq {
		got = append(got, v)
	}
	return got
}

// yieldCase is a list value and the elements an iterator must yield for it, or Next and Prev step over.
type yieldCase struct {
	name string
	list string
	want []string
}

// requireYields runs each case through seq in a parallel subtest of its own.
func requireYields(t *testing.T, seq func(string) iter.Seq[string], cases []yieldCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, collect(seq(tc.list)))
		})
	}
}

func Test_All(t *testing.T) {
	t.Parallel()
	requireYields(t, All, []yieldCase{
		{"empty", "", nil},
		{"single", "gzip", []string{"gzip"}},
		{"trims OWS", "  gzip ,\tbr\t", []string{"gzip", "br"}},
		{"trims OWS around a lone element", " gzip ", []string{"gzip"}},
		{"skips empty elements", "gzip, , br", []string{"gzip", "br"}},
		{"only separators", ",  ,", nil},
		{"quotes are not special", `"a,b"`, []string{`"a`, `b"`}},
	})
}

func Test_TrimOWS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"nothing to trim", "203.0.113.50", "203.0.113.50"},
		{"spaces", "  gzip  ", "gzip"},
		{"tabs", "\tgzip\t", "gzip"},
		{"mixed", " \t gzip\t \t", "gzip"},
		{"only OWS", " \t ", ""},
		{"inner OWS kept", "a \t b", "a \t b"},
		{"CR and LF are not OWS", "\r\ngzip\r\n", "\r\ngzip\r\n"},
		{"VT and FF are not OWS", "\vgzip\f", "\vgzip\f"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, TrimOWS(tc.in))
			require.Equal(t, []byte(tc.want), TrimOWS([]byte(tc.in)))
		})
	}
}

func Test_TrimOWS_AliasesInput(t *testing.T) {
	t.Parallel()
	in := []byte("\tgzip ")
	got := TrimOWS(in)
	require.Equal(t, []byte("gzip"), got)
	require.Same(t, &in[1], &got[0], "TrimOWS must return a sub-slice, not a copy")
}

func Benchmark_TrimOWS(b *testing.B) {
	inputs := []string{"203.0.113.50", " 203.0.113.50", "\t203.0.113.50\t", " \t2001:db8::1 \t"}
	var got string
	b.ReportAllocs()
	for b.Loop() {
		for _, in := range inputs {
			got = TrimOWS(in)
		}
	}
	require.Equal(b, "2001:db8::1", got)
}

func Test_AllQuotedPairs(t *testing.T) {
	t.Parallel()
	requireYields(t, AllQuotedPairs, []yieldCase{
		{"empty", "", nil},
		{"comma inside quotes is kept", `ext="a,b", no-transform`, []string{`ext="a,b"`, "no-transform"}},
		{"escaped quote does not close", `ext="a\"", no-transform`, []string{`ext="a\""`, "no-transform"}},
		{"escaped backslash closes", `ext="a\\", no-transform`, []string{`ext="a\\"`, "no-transform"}},
		{"escaped comma stays inside", `ext="a\,b", x`, []string{`ext="a\,b"`, "x"}},
		{"backslash outside quotes is data", `a\b, c`, []string{`a\b`, "c"}},
		{"trailing backslash inside quotes", `ext="a\`, []string{`ext="a\`}},
		{"unterminated quote swallows rest", `ext="a, b`, []string{`ext="a, b`}},
	})
}

func Test_AllQuoted(t *testing.T) {
	t.Parallel()
	requireYields(t, AllQuoted, []yieldCase{
		{"empty", "", nil},
		{"comma inside quotes is kept", `"v1,v2"`, []string{`"v1,v2"`}},
		{"separates outside quotes", `"a", "b"`, []string{`"a"`, `"b"`}},
		{"weak tags", `W/"a", W/"b,c"`, []string{`W/"a"`, `W/"b,c"`}},
		{"unterminated quote swallows rest", `"a, b`, []string{`"a, b`}},
		{"skips empty elements", `"a", , "b"`, []string{`"a"`, `"b"`}},
	})
}

func Test_AllLines(t *testing.T) {
	t.Parallel()
	require.Nil(t, collect(AllLines(nil)))
	require.Equal(t, []string{"gzip", "br", "zstd"}, collect(AllLines([][]byte{[]byte("gzip, br"), []byte(" zstd ")})))
	// A header split across lines reads the same as one line (RFC 9110 5.3).
	require.Equal(t, collect(All("gzip,br")), collect(AllLines([][]byte{[]byte("gzip"), []byte("br")})))
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

// Test_AppendUnique runs every case through AppendUnique, for which another
// case is another value, and through AppendUniqueFold, for which it is the
// same field and the first spelling is kept.
func Test_AppendUnique(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		list   string
		want   string // from AppendUnique
		fold   string // from AppendUniqueFold
		values []string
	}{
		{"into empty", "", "Accept", "Accept", []string{"Accept"}},
		{"appends new", "Accept", "Accept, Origin", "Accept, Origin", []string{"Origin"}},
		{"skips present", "Accept, Origin", "", "", []string{"Accept"}},
		{"skips empty values", "Accept", "", "", []string{""}},
		{"nothing to add", "Accept", "", "", nil},
		{"partial add", "Accept", "Accept, Origin", "Accept, Origin", []string{"Accept", "Origin"}},
		{"other case", "Accept", "Accept, accept", "", []string{"accept"}},
		{"keeps first spelling", "accept", "accept, Accept", "", []string{"Accept"}},
		{"first spelling in one call", "", "accept, Accept", "accept", []string{"accept", "Accept"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, AppendUnique(tc.list, tc.values))
			require.Equal(t, tc.fold, AppendUniqueFold(tc.list, tc.values))
		})
	}
}

func Test_Append_ReusesStorage(t *testing.T) {
	t.Parallel()
	dst := make([]string, 0, 4)
	got := Append(dst, "a, b")
	require.Equal(t, []string{"a", "b"}, got)
	require.Same(t, &dst[:1][0], &got[0], "Append must reuse dst's storage")
	require.Nil(t, Append(dst, ""), "an empty list yields nil")
}
