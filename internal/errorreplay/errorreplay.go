// Package errorreplay marks the middleware run Fiber replays for a request the
// server refused while reading it, so middleware that stores responses can stay out.
package errorreplay

import (
	"github.com/valyala/fasthttp"
)

type key struct{}

// Mark flags ctx as replayed until ctx is reset.
func Mark(ctx *fasthttp.RequestCtx) {
	ctx.SetUserValue(key{}, true)
}

// Active reports whether ctx is a refused request being replayed.
func Active(ctx *fasthttp.RequestCtx) bool {
	return ctx.UserValue(key{}) != nil
}
