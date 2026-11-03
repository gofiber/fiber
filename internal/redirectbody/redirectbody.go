// Package redirectbody turns a request into one without a body, for the hop a
// redirect sends it on as a GET: the client and the proxy follow redirects
// each with a loop of their own, and what goes with the body is decided here
// for both.
package redirectbody

import (
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/valyala/fasthttp"
)

// Drop removes the body of req and the fields that frame or describe it:
// Content-Length, Transfer-Encoding and the trailer section signal one (RFC
// 9112), Content-Type and Content-Encoding describe one no longer there, and
// Expect asks for one (RFC 9110 §10.1.1), which a strict target answers with
// 417 instead of serving the bodyless follow-up. It reports false, leaving req
// as it is, when the trailer section announces more names than
// fieldname.MaxTrailers.
func Drop(req *fasthttp.Request) bool {
	// The trailer section goes with the body. Deleting Trailer alone would have fasthttp
	// return the fields filed under the announced names to the header section, and the
	// redirect target, possibly another host, would receive them as header lines.
	if !fieldname.DropTrailers(&req.Header) {
		return false
	}
	req.Header.Del(fasthttp.HeaderContentLength)
	req.Header.Del(fasthttp.HeaderContentType)
	req.Header.Del(fasthttp.HeaderContentEncoding)
	req.Header.Del(fasthttp.HeaderTransferEncoding)
	req.Header.Del(fasthttp.HeaderExpect)
	req.ResetBody()
	// Load-bearing, not bookkeeping: Request.Write falls back to the parsed form
	// when the body is empty, so a caller that had read PostArgs would have sent
	// the old "a=1" as the body of the redirected GET.
	//
	// fasthttp's own loop clears the parsed flag beside this, which leaves a
	// reused request re-parsing whatever body it is given next. That half is not
	// reachable from here: parsedPostArgs is unexported and only Request.Reset
	// and ReadLimitBody clear it, and Reset would take the URI, headers and TLS
	// flag the redirect loops are still using. What differs is confined to a
	// caller that reuses the request without resetting it: its PostArgs reads
	// empty rather than re-parsing. The body it sends is the new one either way,
	// and an untouched fasthttp request is worse off there: it hands back the
	// previous body's values, since SetBody does not invalidate the form either.
	req.PostArgs().Reset()
	return true
}
