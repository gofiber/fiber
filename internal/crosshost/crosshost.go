// Package crosshost names the request headers that must not survive a redirect
// to a host other than the one they were sent to.
package crosshost

import (
	"math"
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

// HeaderNames holds header names without stray whitespace; only NewHeaderNames builds one.
type HeaderNames struct{ names []string }

// NewHeaderNames trims names and drops empty ones, in a fresh list.
func NewHeaderNames(names []string) HeaderNames {
	var out []string
	for _, name := range names {
		if name = utils.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return HeaderNames{names: out}
}

// List returns a copy of the names.
func (n HeaderNames) List() []string {
	return slices.Clone(n.names)
}

// Join returns n's names followed by other's, in a fresh list.
func (n HeaderNames) Join(other HeaderNames) HeaderNames {
	return HeaderNames{names: slices.Concat(n.names, other.names)}
}

// Strip removes SensitiveHeaders and extra from h, whatever case they are stored under.
func Strip(h fieldname.Deleter, extra HeaderNames) {
	StripWithin(h, extra, math.MaxInt)
}

// StripWithin is Strip bounded as fieldname.DelSetWithin is: it removes none and
// reports false when h spells the names in more than spare further ways.
func StripWithin(h fieldname.Deleter, extra HeaderNames, spare int) bool {
	names := SensitiveHeaders
	if len(extra.names) > 0 {
		names = slices.Concat(SensitiveHeaders, extra.names)
	}
	return fieldname.DelSetWithin(h, names, spare)
}
