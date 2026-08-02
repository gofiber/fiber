// Package crosshost names the request headers that must not survive a redirect
// to a host other than the one they were sent to.
package crosshost

import (
	"github.com/valyala/fasthttp"
)

// SensitiveHeaders lists the headers carrying credentials scoped to a single
// origin. A redirect off that origin has to drop every one of them, or the
// client's secret reaches a host the client never addressed — and where the
// redirect goes is the responding server's choice, not the client's.
//
// The set is net/http's, which fasthttp also implements. Cookie2 is the
// obsolete RFC 2965 spelling of Cookie and carries a session exactly as it
// does; the two Authenticate headers are challenges rather than credentials,
// but a peer echoing one back is describing the origin that issued it.
//
// It lives here because both the client's redirect loop and the proxy
// middleware's need the same answer, and they had already drifted apart once —
// the proxy was missing Cookie2 and both Authenticate headers, so a session
// followed a redirect the client package would have stripped it from. A list
// two packages agree on by construction cannot drift again.
var SensitiveHeaders = []string{
	fasthttp.HeaderAuthorization,
	fasthttp.HeaderProxyAuthorization,
	fasthttp.HeaderProxyAuthenticate,
	fasthttp.HeaderWWWAuthenticate,
	fasthttp.HeaderCookie,
	// fasthttp has no constant for the obsolete spelling.
	"Cookie2",
}
