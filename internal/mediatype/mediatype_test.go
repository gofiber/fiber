package mediatype

import (
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

// Test_NormalizeRequestContentType_QuotedPair covers a quoted-pair inside a
// parameter value.
//
// Ending the quoted-string at an escaped quote folds the rest of it as though
// those were parameters. fasthttp matches "boundary" case-sensitively and takes
// the first one, so lowercasing a decoy it would otherwise walk past is what
// makes it the boundary it picks — and the real one is never reached.
func Test_NormalizeRequestContentType_QuotedPair(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "escaped quote does not end the value",
			in:   `Multipart/Form-Data; X="\"; BOUNDARY=bogus"; BOUNDARY=Real`,
			want: `multipart/form-data; x="\"; BOUNDARY=bogus"; boundary=Real`,
		},
		{
			name: "trailing backslash cannot escape past the end",
			in:   `Multipart/Form-Data; X="ab\`,
			want: `multipart/form-data; x="ab\`,
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
		})
	}
}

// Test_NormalizeRequestContentType_DecoyBoundaryLosesToReal is the same input
// read the way fasthttp reads it: the boundary it selects has to be the one the
// author wrote, not the one hidden in an earlier parameter's value.
func Test_NormalizeRequestContentType_DecoyBoundaryLosesToReal(t *testing.T) {
	t.Parallel()

	var h fasthttp.RequestHeader
	h.SetContentType(`Multipart/Form-Data; X="\"; BOUNDARY=bogus"; BOUNDARY=Real`)
	NormalizeRequestContentType(&h)
	require.Equal(t, "Real", string(h.MultipartFormBoundary()))
}
