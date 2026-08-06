// Package crosshost names the request headers that must not survive a redirect
// to a host other than the one they were sent to.
package crosshost

import (
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
