package cookie

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Test_ParseSameSite covers every accepted mode, case-insensitive matching,
// Secure requirements, and the invalid-to-Lax fallback contract.
func Test_ParseSameSite(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		value    string
		httpMode http.SameSite
		fastMode fasthttp.CookieSameSite
		secure   bool // RequiresSecure
		valid    bool
	}{
		{"disabled", "DISABLED", 0, fasthttp.CookieSameSiteDisabled, false, true},
		{"lax", "lax", http.SameSiteLaxMode, fasthttp.CookieSameSiteLaxMode, false, true},
		{"strict", "STRICT", http.SameSiteStrictMode, fasthttp.CookieSameSiteStrictMode, false, true},
		{"none", "NoNe", http.SameSiteNoneMode, fasthttp.CookieSameSiteNoneMode, true, true},
		{"invalid falls back to lax", "invalid", http.SameSiteLaxMode, fasthttp.CookieSameSiteLaxMode, false, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, valid := ParseSameSite(test.value)
			require.Equal(t, test.valid, valid)
			want := SameSite{HTTPMode: test.httpMode, FastHTTPMode: test.fastMode, RequiresSecure: test.secure}
			require.Equal(t, want, got)
		})
	}
}

func Test_FormatSameSite(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want string
		mode fasthttp.CookieSameSite
	}{
		{name: "disabled", mode: fasthttp.CookieSameSiteDisabled, want: SameSiteDisabled},
		{name: "lax", mode: fasthttp.CookieSameSiteLaxMode, want: SameSiteLax},
		{name: "strict", mode: fasthttp.CookieSameSiteStrictMode, want: SameSiteStrict},
		{name: "none", mode: fasthttp.CookieSameSiteNoneMode, want: SameSiteNone},
		{name: "fasthttp default has no Fiber name", mode: fasthttp.CookieSameSiteDefaultMode, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FormatSameSite(tc.mode)
			require.Equal(t, tc.want, got)

			if got == "" {
				return
			}
			parsed, ok := ParseSameSite(got)
			require.True(t, ok)
			require.Equal(t, tc.mode, parsed.FastHTTPMode)
		})
	}
}
