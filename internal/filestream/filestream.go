// Package filestream tells the response body stream that the file server of Ctx.SendFile or the static
// middleware set from one that a handler or a net/http handler set.
//
// fasthttp marks both alike as a body stream. Code that sets one of its own, such as the callback of
// SendStreamWriter, can run once the handlers returned with whatever they handed it; a file the server
// opened holds nothing of the request.
package filestream

import (
	"github.com/valyala/fasthttp"
)

// served is the value stored under a stream that Mark recorded. It is no io.Closer: fasthttp closes
// such user values when it resets the request, and the response closes the stream already.
type served struct{}

// Mark records the body stream of the response on ctx, if it has one, as one the file server set.
// Call it right after the file server ran.
func Mark(ctx *fasthttp.RequestCtx) {
	if stream := ctx.Response.BodyStream(); stream != nil {
		// The stream itself is the key, so a stream set in its place later is not taken for it.
		ctx.SetUserValue(stream, served{})
	}
}

// Marked reports whether the response on ctx is streamed from a file the file server opened, as Mark
// recorded it. A stream that replaced that one, or wraps it, is not.
func Marked(ctx *fasthttp.RequestCtx) bool {
	stream := ctx.Response.BodyStream()
	if stream == nil {
		return false
	}
	_, ok := ctx.UserValue(stream).(served)
	return ok
}
