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
		name  string
		value string
		want  SameSite
		valid bool
	}{
		{
			name:  "disabled",
			value: "DISABLED",
			want: SameSite{
				HTTPMode:     0,
				FastHTTPMode: fasthttp.CookieSameSiteDisabled,
			},
			valid: true,
		},
		{
			name:  "lax",
			value: "lax",
			want: SameSite{
				HTTPMode:     http.SameSiteLaxMode,
				FastHTTPMode: fasthttp.CookieSameSiteLaxMode,
			},
			valid: true,
		},
		{
			name:  "strict",
			value: "STRICT",
			want: SameSite{
				HTTPMode:     http.SameSiteStrictMode,
				FastHTTPMode: fasthttp.CookieSameSiteStrictMode,
			},
			valid: true,
		},
		{
			name:  "none",
			value: "NoNe",
			want: SameSite{
				HTTPMode:       http.SameSiteNoneMode,
				FastHTTPMode:   fasthttp.CookieSameSiteNoneMode,
				RequiresSecure: true,
			},
			valid: true,
		},
		{
			name:  "invalid falls back to lax",
			value: "invalid",
			want: SameSite{
				HTTPMode:     http.SameSiteLaxMode,
				FastHTTPMode: fasthttp.CookieSameSiteLaxMode,
			},
			valid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, valid := ParseSameSite(test.value)
			require.Equal(t, test.valid, valid)
			require.Equal(t, test.want, got)
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
