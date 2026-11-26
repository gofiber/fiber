// Package unsent marks a request whose connection was closed without a response,
// so middleware that stores or counts responses can tell.
package unsent

import (
	"github.com/valyala/fasthttp"
)

type key struct{}

// Mark flags ctx as a request that is sent no response.
func Mark(ctx *fasthttp.RequestCtx) {
	ctx.SetUserValue(key{}, true)
}

// Is reports whether ctx may be sent no response: Mark flagged it, or the connection was hijacked, as the
// adaptor also does for an aborted net/http handler. fasthttp sends the response before a hijack handler
// unless HijackSetNoResponse(true) was set, which it does not report, so every hijack counts.
func Is(ctx *fasthttp.RequestCtx) bool {
	return ctx.Hijacked() || ctx.UserValue(key{}) != nil
}
