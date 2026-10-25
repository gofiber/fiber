// Package crosshost names the request headers that must not survive a redirect
// to a host other than the one they were sent to, and removes them.
package crosshost

import (
	"slices"

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

// HeaderNames is a list of header names. Only NewHeaderNames builds one, so a name with stray whitespace,
// which would never match its header, cannot get through. The zero value is the empty list.
type HeaderNames struct{ names []string }

// NewHeaderNames returns names without surrounding whitespace or empty entries, in a fresh list. A name
// split from a comma-separated setting often keeps a leading space, and one that never matches leaves its
// header in place.
func NewHeaderNames(names []string) HeaderNames {
	var out []string
	for _, name := range names {
		if name = utils.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return HeaderNames{names: out}
}

// List returns a copy of the names, nil when there are none.
func (n HeaderNames) List() []string {
	return slices.Clone(n.names)
}

// Join returns the names of n followed by those of other, in a fresh list.
func (n HeaderNames) Join(other HeaderNames) HeaderNames {
	return HeaderNames{names: slices.Concat(n.names, other.names)}
}

// Strip removes SensitiveHeaders and the extra names from h in one pass, whatever case they are
// stored under.
func Strip(h fieldname.Deleter, extra HeaderNames) {
	names := SensitiveHeaders
	if len(extra.names) > 0 {
		names = slices.Concat(SensitiveHeaders, extra.names)
	}
	fieldname.DelSet(h, names)
}
