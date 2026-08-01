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
		{"quoted value holding a semicolon", `Multipart/Form-Data; Boundary="a;B"; Name=X`, `multipart/form-data; boundary="a;B"; name=X`},
		{"parameter with no value", "Text/Plain; Flag; Charset=x", "text/plain; flag; charset=x"},
		{"trailing semicolon", "Text/Plain;", "text/plain;"},
		{"empty", "", ""},

		// An unterminated quote consumes the rest of the list. fasthttp's own
		// boundary scanner has no quoting rules at all, so this scanner stays no
		// more permissive than it: the string ends at the first quote, escaped
		// or not, and never swallows a later parameter name a backslash
		// "escaped" past.
		{"unterminated quote", `Multipart/Form-Data; X="abc`, `multipart/form-data; x="abc`},
		{"backslash is not an escape", `Multipart/Form-Data; X="\"; BOUNDARY=abc`, `multipart/form-data; x="\"; boundary=abc`},
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
