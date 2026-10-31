package mediatype

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Test_NormalizeRequestContentType pins the RFC 9110 case rules this package
// implements: the media type (Section 8.3.1) and each parameter name
// (Section 5.6.6) are case-insensitive and get folded, while parameter values
// are left byte-for-byte alone. Folding a value would corrupt a multipart
// boundary, which is case-sensitive and has to keep matching the body.
func Test_NormalizeRequestContentType(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"already lowercase", "application/json", "application/json"},
		{"media type only", "Application/JSON", "application/json"},
		{"media type with parameter", "Multipart/Form-Data; BOUNDARY=AbC", "multipart/form-data; boundary=AbC"},
		{"parameter value untouched", "text/plain; charset=UTF-8", "text/plain; charset=UTF-8"},
		{"several parameters", "TEXT/Plain; Charset=UTF-8; Format=Flowed", "text/plain; charset=UTF-8; format=Flowed"},
		{"no space after semicolon", "Text/HTML;Charset=UTF-8", "text/html;charset=UTF-8"},
		{"tab after semicolon", "Text/HTML;\tCharset=UTF-8", "text/html;\tcharset=UTF-8"},
		{"tab and space after semicolon", "Text/Plain;\t CHARSET=UTF-8", "text/plain;\t charset=UTF-8"},
		{"empty parameter between semicolons", "Text/Plain;; CHARSET=UTF-8", "text/plain;; charset=UTF-8"},
		{"quoted value holding a semicolon", `Multipart/Form-Data; Boundary="a;B"; Name=X`, `multipart/form-data; boundary="a;B"; name=X`},
		{"space before quoted value", `Text/Plain; X= "A;B"; Y=Z`, `text/plain; x= "A;B"; y=Z`},
		{"tab before quoted value", "Text/Plain; X=\t\"A;B\"; Y=Z", "text/plain; x=\t\"A;B\"; y=Z"},
		{"later equals does not open quotes", `Text/Plain; X=ab="CD; Y=Z`, `text/plain; x=ab="CD; y=Z`},
		{"quote in bare value", `Text/Plain; X=ab"CD; Y=Z`, `text/plain; x=ab"CD; y=Z`},
		{"terminal quoted value", `Text/Plain; X="A;B"`, `text/plain; x="A;B"`},
		{"whitespace after quoted value", "Text/Plain; X=\"A;B\" \t; Y=Z", "text/plain; x=\"A;B\" \t; y=Z"},
		{"parameter with no value", "Text/Plain; Flag; Charset=x", "text/plain; flag; charset=x"},
		{"trailing semicolon", "Text/Plain;", "text/plain;"},
		{"empty", "", ""},

		// An unterminated quote consumes the rest of the list, and a quoted-pair
		// keeps the value going (RFC 9110 Section 5.6.6). Everything inside the
		// quotes is a value, so no parameter name in there is folded: fasthttp
		// matches "boundary" case-sensitively, and folding one hidden in a value
		// is what would promote it over the boundary the author wrote.
		{"unterminated quote", `Multipart/Form-Data; X="abc`, `multipart/form-data; x="abc`},
		{"quoted-pair keeps the value open", `Multipart/Form-Data; X="\"; BOUNDARY=abc`, `multipart/form-data; x="\"; BOUNDARY=abc`},

		// Ending the quoted-string at an escaped quote folds the rest of it as
		// though those were parameters. fasthttp matches "boundary"
		// case-sensitively and takes the first one, so lowercasing a decoy it
		// would otherwise walk past is what makes it the boundary it picks — and
		// the real one is never reached.
		{
			name: "escaped quote does not end the value",
			in:   `Multipart/Form-Data; X="\"; BOUNDARY=bogus"; BOUNDARY=Real`,
			want: `multipart/form-data; x="\"; BOUNDARY=bogus"; boundary=Real`,
		},
		{"trailing backslash cannot escape past the end", `Multipart/Form-Data; X="ab\`, `multipart/form-data; x="ab\`},
		{
			name: "space before quoted decoy",
			in:   `Multipart/Form-Data; X= "; BOUNDARY=bogus"; BOUNDARY=Real`,
			want: `multipart/form-data; x= "; BOUNDARY=bogus"; boundary=Real`,
		},
		{
			name: "whitespace before escaped quote",
			in:   "Multipart/Form-Data; X= \t\"\\\"; BOUNDARY=bogus\"; BOUNDARY=Real",
			want: "multipart/form-data; x= \t\"\\\"; BOUNDARY=bogus\"; boundary=Real",
		},
		{
			name: "whitespace before unterminated quote",
			in:   "Multipart/Form-Data; X=\t\"ab; BOUNDARY=bogus",
			want: "multipart/form-data; x=\t\"ab; BOUNDARY=bogus",
		},
		{
			name: "escaped backslash still closes the value",
			in:   `Multipart/Form-Data; X="a\\"; BOUNDARY=Real`,
			want: `multipart/form-data; x="a\\"; boundary=Real`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var h fasthttp.RequestHeader
			h.SetContentType(tc.in)

			require.Equal(t, tc.want, string(NormalizeRequestContentType(&h)))
			// The header itself has to change, not a copy: fasthttp's form and
			// multipart parsers read the request's own bytes.
			require.Equal(t, tc.want, string(h.ContentType()))

			// Idempotent, since every accessor that folds may be called
			// repeatedly within one request.
			require.Equal(t, tc.want, string(NormalizeRequestContentType(&h)))
		})
	}
}

func Test_IsForm(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"application/x-www-form-urlencoded", true},
		{"Application/X-WWW-Form-Urlencoded", true},
		{"multipart/form-data", true},
		{"Multipart/Form-Data; boundary=abc", true},
		{"multipart/form-data ; boundary=abc", true},
		{"multipart/form-data\t;boundary=abc", true},

		{"application/json", false},
		{"text/plain", false},
		{"", false},
		{"multipart/mixed", false},
		{"application/x-www-form-urlencoded-not", false},
		// A leading space is not trimmed: fasthttp does not produce one, and
		// accepting it here would diverge from the parsers this gates.
		{" multipart/form-data", false},
	} {
		require.Equal(t, tc.want, IsForm([]byte(tc.in)), "input %q", tc.in)
	}
}

func Test_ParsesPostArgs(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"application/x-www-form-urlencoded", true},
		{"application/x-www-form-urlencoded; charset=utf-8", true},
		{"application/x-www-form-urlencoded-not", true},

		{"Application/X-WWW-Form-Urlencoded", false},
		{" application/x-www-form-urlencoded", false},
		{"multipart/form-data", false},
		{"", false},
	} {
		var req fasthttp.Request
		req.Header.SetContentType(tc.in)
		req.SetBodyString("k=v")
		require.Equal(t, tc.want, req.PostArgs().Has("k"), "fasthttp, input %q", tc.in)
		require.Equal(t, tc.want, ParsesPostArgs([]byte(tc.in)), "input %q", tc.in)
	}
}

// Test_NormalizeRequestContentType_KeepsBoundaryUsable is the property the
// whole package exists for: fasthttp locates the multipart boundary with
// case-sensitive comparisons, so a legal "Multipart/Form-Data" with an
// upper-case "BOUNDARY=" must still parse, and the boundary value itself must
// survive unfolded so it keeps matching the body.
func Test_NormalizeRequestContentType_KeepsBoundaryUsable(t *testing.T) {
	t.Parallel()
	var req fasthttp.Request
	req.Header.SetContentType(`Multipart/Form-Data; BOUNDARY=AbCdEf`)
	req.SetBody([]byte("--AbCdEf\r\nContent-Disposition: form-data; name=\"who\"\r\n\r\nworld\r\n--AbCdEf--\r\n"))

	NormalizeRequestContentType(&req.Header)
	require.Equal(t, "AbCdEf", string(req.Header.MultipartFormBoundary()))

	form, err := req.MultipartForm()
	require.NoError(t, err)
	require.Equal(t, []string{"world"}, form.Value["who"])
}

// Test_NormalizeRequestContentType_DecoyBoundaryLosesToReal reads the input of
// the "escaped quote does not end the value" case the way fasthttp reads it:
// the boundary it selects has to be the one the author wrote, not the one
// hidden in an earlier parameter's value.
func Test_NormalizeRequestContentType_DecoyBoundaryLosesToReal(t *testing.T) {
	t.Parallel()
	var h fasthttp.RequestHeader
	h.SetContentType(`Multipart/Form-Data; X="\"; BOUNDARY=bogus"; BOUNDARY=Real`)
	NormalizeRequestContentType(&h)
	require.Equal(t, "Real", string(h.MultipartFormBoundary()))
}

func Test_StripParams(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"text/html", "text/html"},
		{"text/html; charset=utf-8", "text/html"},
		{"text/html ; charset=utf-8", "text/html"},
		{" \ttext/html\t;charset=utf-8", "text/html"},
		{"Text/HTML;", "Text/HTML"},
		{"; charset=utf-8", ""},
		{"   ", ""},
	} {
		require.Equal(t, []byte(tc.want), StripParams([]byte(tc.in)), "input %q", tc.in)
	}
}

func Test_Params(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{name: "no parameters", in: "text/plain"},
		{name: "trailing semicolon", in: "text/plain;"},
		{name: "one parameter", in: "text/plain; charset=utf-8", want: []string{"charset", "utf-8"}},
		{name: "spaces around equals", in: "text/plain; charset = utf-8", want: []string{"charset", "utf-8"}},
		{name: "empty value", in: "text/plain; charset=", want: []string{"charset", ""}},
		{name: "missing equals skipped", in: "text/plain; flag; ; a=1", want: []string{"a", "1"}},
		{name: "several parameters", in: "text/plain;a=1;\tb=2 ; c=3", want: []string{"a", "1", "b", "2", "c", "3"}},
		{name: "quoted semicolon", in: `text/plain; title="x;charset=bad"; charset=utf-8`, want: []string{"title", `"x;charset=bad"`, "charset", "utf-8"}},
		{name: "quoted escaped quote", in: `text/plain; title="a\";c=bad"; c=ok`, want: []string{"title", `"a\";c=bad"`, "c", "ok"}},
		{name: "even backslashes before closing quote", in: `text/plain; a="x\\"; b=2`, want: []string{"a", `"x\\"`, "b", "2"}},
		{name: "odd backslashes before quote", in: `text/plain; a="x\\\";y=z"; b=2`, want: []string{"a", `"x\\\";y=z"`, "b", "2"}},
		{name: "escaped quote before closing quote", in: `text/plain; a="x\""; b=2`, want: []string{"a", `"x\""`, "b", "2"}},
		{name: "multiple escaped quotes", in: `text/plain; a="x\";\"y"; b=2`, want: []string{"a", `"x\";\"y"`, "b", "2"}},
		{name: "escaped quotes around semicolon", in: `text/plain; a="x\";\";c=bad"; c=ok`, want: []string{"a", `"x\";\";c=bad"`, "c", "ok"}},
		{name: "quote after closing quote stays bare", in: `text/plain; a="x""; c=bad"; c=ok`, want: []string{"a", `"x""`, "c", `bad"`, "c", "ok"}},
		{name: "escaped quote at start of value", in: `text/plain; a="\";c=bad"; c=ok`, want: []string{"a", `"\";c=bad"`, "c", "ok"}},
		{name: "semicolon at start of quoted value", in: `text/plain; a="; c=bad"; c=ok`, want: []string{"a", `"; c=bad"`, "c", "ok"}},
		{name: "unterminated after escaped quote", in: `text/plain; a="x\"; b=2`, want: []string{"a", `"x\"; b=2`}},
		{name: "unterminated after escaped quote and backslash", in: `text/plain; a="x\"; end\`, want: []string{"a", `"x\"; end\`}},
		{name: "quote after whitespace opens", in: `text/plain; a= "x;y"; b=2`, want: []string{"a", `"x;y"`, "b", "2"}},
		{name: "stray quote in bare value", in: `text/plain; a=b"; c=bad"; d=1`, want: []string{"a", `b"`, "c", `bad"`, "d", "1"}},
		{name: "quote in name before quoted value", in: `text/plain; "a=";b=c"; d=1`, want: []string{`"a`, `";b=c"`, "d", "1"}},
		{name: "quote without equals does not open", in: `text/plain; "a; b=2`, want: []string{"b", "2"}},
		{name: "later equals does not open quotes", in: `text/plain; a=b="; c=bad"; c=ok`, want: []string{"a", `b="`, "c", `bad"`, "c", "ok"}},
		{name: "later equals after closed quote", in: `text/plain; a="x"="; c=bad"; c=ok`, want: []string{"a", `"x"="`, "c", `bad"`, "c", "ok"}},
		{name: "later equals after whitespace", in: `text/plain; a= = "; c=bad"; c=ok`, want: []string{"a", `= "`, "c", `bad"`, "c", "ok"}},
		{name: "consecutive empty parameters", in: "text/plain;;;;a=1;;;b=2;;", want: []string{"a", "1", "b", "2"}},
		{name: "unterminated quote", in: `text/plain; a="x; b=2`, want: []string{"a", `"x; b=2`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for name, value := range Params([]byte(tc.in)) {
				got = append(got, string(name), string(value))
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func Test_Params_Break(t *testing.T) {
	t.Parallel()
	var names []string
	for name := range Params([]byte("text/plain; a=1; b=2")) {
		names = append(names, string(name))
		break
	}
	require.Equal(t, []string{"a"}, names)
}

func Test_Unescape(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in   string
		want string
		ok   bool
	}{
		{in: "", want: "", ok: true},
		{in: "abc", want: "abc", ok: true},
		{in: `a\"b`, want: `a"b`, ok: true},
		{in: `c\\d`, want: `c\d`, ok: true},
		{in: `a\"b\\c\"d`, want: `a"b\c"d`, ok: true},
		{in: `value with \"quote\"`, want: `value with "quote"`, ok: true},
		{in: `utf\-8`, want: "utf-8", ok: true},
		{in: `\\`, want: `\`, ok: true},
		{in: `bad\`, want: `bad\`, ok: false},
		{in: `\`, want: `\`, ok: false},
		{in: `a\"b\`, want: `a"b\`, ok: false},
		{in: `a\\\`, want: `a\\`, ok: false},
	} {
		got, ok := Unescape([]byte(tc.in))
		require.Equal(t, tc.ok, ok, "input %q", tc.in)
		require.Equal(t, tc.want, string(got), "input %q", tc.in)
	}
}

func Test_Unescape_AliasesWithoutEscapes(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"abc", `ab\`} {
		v := []byte(in)
		got, _ := Unescape(v)
		require.Same(t, &v[0], &got[0], "input %q", in)
	}
}

func Benchmark_StripParams(b *testing.B) {
	json := []byte("application/json")
	html := []byte("text/html ; charset=utf-8")
	var got []byte
	b.ReportAllocs()
	for b.Loop() {
		_ = StripParams(json)
		got = StripParams(html)
	}
	require.Equal(b, []byte("text/html"), got)
}

func Benchmark_Params(b *testing.B) {
	ct := []byte(`multipart/form-data; boundary="a;b"; charset=utf-8`)
	var n int
	b.ReportAllocs()
	for b.Loop() {
		for name, value := range Params(ct) {
			n += len(name) + len(value)
		}
	}
	require.Equal(b, 25*b.N, n)
}

// go test -v -run=^$ -bench=Benchmark_ParamsInputs -benchmem -count=4
func Benchmark_ParamsInputs(b *testing.B) {
	for _, bench := range []struct {
		name string
		ct   []byte
		want int
	}{
		{name: "no_parameters", ct: []byte("application/json"), want: 0},
		{name: "charset", ct: []byte("application/json; charset=utf-8"), want: 12},
		{name: "charset_last", ct: []byte(`multipart/form-data; boundary="a;b"; charset=utf-8`), want: 25},
		{name: "many_bare", ct: []byte("text/plain; a=1; b=2; c=3; d=4; charset=utf-8"), want: 20},
		{name: "long_quoted", ct: []byte(`text/plain; title="` + strings.Repeat("a", 1024) + `"; charset=utf-8`), want: 1043},
		{name: "escaped_quotes", ct: []byte(`text/plain; title="` + strings.Repeat(`\"`, 512) + `"; charset=utf-8`), want: 1043},
		{name: "empty_parameters", ct: []byte("text/plain;" + strings.Repeat(";", 4000)), want: 0},
		{name: "empty_parameters_before_charset", ct: []byte("text/plain;;;charset=utf-8"), want: 12},
		{name: "missing_equals", ct: []byte("text/plain; flag"), want: 0},
	} {
		b.Run(bench.name, func(b *testing.B) {
			var n int
			b.ReportAllocs()
			for b.Loop() {
				for name, value := range Params(bench.ct) {
					n += len(name) + len(value)
				}
			}
			require.Equal(b, bench.want*b.N, n)
		})
	}
}

func Benchmark_NormalizeRequestContentType(b *testing.B) {
	for _, bench := range []struct {
		name string
		ct   string
	}{
		{name: "lowercase", ct: "multipart/form-data; boundary=lower"},
		{name: "uppercase_value", ct: "multipart/form-data; boundary=Real"},
		{name: "quoted_value", ct: `multipart/form-data; boundary="Real"`},
		{name: "long_quoted", ct: `multipart/form-data; boundary="` + strings.Repeat("A", 70) + `"`},
		{name: "escaped_quotes", ct: `multipart/form-data; x="A` + strings.Repeat(`\"`, 512) + `"; boundary=Real`},
	} {
		b.Run(bench.name, func(b *testing.B) {
			var h fasthttp.RequestHeader
			h.SetContentType(bench.ct)
			var got []byte
			b.ReportAllocs()
			for b.Loop() {
				got = NormalizeRequestContentType(&h)
			}
			require.Equal(b, bench.ct, string(got))
		})
	}
}

func Benchmark_Unescape(b *testing.B) {
	for _, bench := range []struct {
		name string
		in   []byte
	}{
		{name: "plain", in: []byte("value with spaces")},
		{name: "quoted_pair", in: []byte(`value with \"quotes\"`)},
	} {
		b.Run(bench.name, func(b *testing.B) {
			var got []byte
			b.ReportAllocs()
			for b.Loop() {
				got, _ = Unescape(bench.in)
			}
			require.NotEmpty(b, got)
		})
	}
}

func Test_IsToken(t *testing.T) {
	t.Parallel()
	const tchar = "!#$%&'*+-.^_`|~0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	for c := range 256 {
		require.Equal(t, strings.IndexByte(tchar, byte(c)) >= 0, IsToken([]byte{byte(c)}), "%q", byte(c))
	}

	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"utf-8", true},
		{"Shift_JIS", true},
		{"", false},
		{"utf-8,latin1", false},
		{`utf\-8`, false},
		{"utf 8", false},
		{"utf-8;", false},
		{"\xc3\xbc", false},
	} {
		require.Equal(t, tc.want, IsToken([]byte(tc.in)), "%q", tc.in)
	}
}
