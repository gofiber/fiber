// Package cookie centralizes cookie policy shared by Fiber core and middleware.
package cookie

import (
	"net/http"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// SameSite mode names accepted by Fiber's cookie APIs.
const (
	SameSiteDisabled = "disabled"
	SameSiteLax      = "Lax"
	SameSiteStrict   = "Strict"
	SameSiteNone     = "None"
)

// SameSite maps one Fiber SameSite mode to the cookie implementations Fiber
// validates with and writes through.
type SameSite struct {
	HTTPMode       http.SameSite
	FastHTTPMode   fasthttp.CookieSameSite
	RequiresSecure bool
}

// ParseSameSite parses value case-insensitively. Unsupported values return the
// Lax mapping and false, allowing callers to choose fallback or rejection.
func ParseSameSite(value string) (SameSite, bool) {
	switch {
	case utils.EqualFold(value, SameSiteDisabled):
		return SameSite{
			HTTPMode:     0,
			FastHTTPMode: fasthttp.CookieSameSiteDisabled,
		}, true
	case utils.EqualFold(value, SameSiteLax):
		return SameSite{
			HTTPMode:     http.SameSiteLaxMode,
			FastHTTPMode: fasthttp.CookieSameSiteLaxMode,
		}, true
	case utils.EqualFold(value, SameSiteStrict):
		return SameSite{
			HTTPMode:     http.SameSiteStrictMode,
			FastHTTPMode: fasthttp.CookieSameSiteStrictMode,
		}, true
	case utils.EqualFold(value, SameSiteNone):
		return SameSite{
			HTTPMode:       http.SameSiteNoneMode,
			FastHTTPMode:   fasthttp.CookieSameSiteNoneMode,
			RequiresSecure: true,
		}, true
	default:
		return SameSite{
			HTTPMode:     http.SameSiteLaxMode,
			FastHTTPMode: fasthttp.CookieSameSiteLaxMode,
		}, false
	}
}

// FormatSameSite names the Fiber SameSite mode a fasthttp cookie carries,
// inverting ParseSameSite for every mode Fiber writes. fasthttp's default mode
// has no Fiber name and returns an empty string.
func FormatSameSite(mode fasthttp.CookieSameSite) string {
	switch mode {
	case fasthttp.CookieSameSiteDisabled:
		return SameSiteDisabled
	case fasthttp.CookieSameSiteLaxMode:
		return SameSiteLax
	case fasthttp.CookieSameSiteStrictMode:
		return SameSiteStrict
	case fasthttp.CookieSameSiteNoneMode:
		return SameSiteNone
	default:
		// fasthttp's default mode lands here: it emits the attribute with no
		// value, which is not one of the modes Fiber names.
		return ""
	}
}
