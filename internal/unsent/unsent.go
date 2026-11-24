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

// Dropped reports whether Mark flagged ctx.
func Dropped(ctx *fasthttp.RequestCtx) bool {
	return ctx.UserValue(key{}) != nil
}

// Is reports whether the response on ctx is not the answer to its request: the connection was
// dropped, or hijacked for another protocol, as the adaptor also does for an aborted net/http handler.
func Is(ctx *fasthttp.RequestCtx) bool {
	return ctx.Hijacked() || Dropped(ctx)
}
