// Package crosshost names the request headers that must not survive a redirect
// to a host other than the one they were sent to, and removes them.
package crosshost

import (
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// SensitiveHeaders lists the headers carrying credentials scoped to one origin.
// A redirect off it must drop every one, or the client's secret reaches a host
// it never addressed — and the responding server chooses where that goes.
//
// net/http's set, which fasthttp also implements. Shared because the client's
// redirect loop and the proxy's had already drifted: the proxy was missing
// Cookie2 and both Authenticate headers.
var SensitiveHeaders = []string{
	fasthttp.HeaderAuthorization,
	fasthttp.HeaderProxyAuthorization,
	fasthttp.HeaderProxyAuthenticate,
	fasthttp.HeaderWWWAuthenticate,
	fasthttp.HeaderCookie,
	// fasthttp has no constant for the obsolete spelling.
	"Cookie2",
}

// CopyNames returns a copy of names without surrounding whitespace or empty entries. A name split
// from a comma-separated setting often keeps a leading space, and one that never matches leaves its
// header in place. It returns nil when nothing is left, so an unset list costs nothing.
func CopyNames(names []string) []string {
	var out []string
	for _, name := range names {
		if name = utils.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// Strip removes SensitiveHeaders and the extra names from h, whatever case they are stored under.
// The second pass runs only when extra names some.
func Strip(h fieldname.Deleter, extra []string) {
	fieldname.DelSet(h, SensitiveHeaders)
	if len(extra) > 0 {
		fieldname.DelSet(h, extra)
	}
}
