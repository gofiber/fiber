// Package byterange adds the Range and If-Range rules (RFC 9110 §13.1.5, §14.2)
// that fasthttp's file server lacks: it refuses several ranges with 416, takes
// only a lower-case "bytes" unit, applies Range to HEAD and ignores If-Range.
// It is shared by Ctx.SendFile and the static middleware.
package byterange

import (
	"bytes"
	"strings"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

const (
	// unitBytes is the only unit the file server knows. Units are
	// case-insensitive (RFC 9110 §14.1), its parser is not.
	unitBytes = "bytes"

	weakPrefix = "W/"

	// A Last-Modified time is implicitly weak unless the server knows the file did
	// not change twice within that second (§8.8.2.2), which it cannot. Two seconds
	// of age only rules out a file still being written; one rewritten within the
	// second a client fetched it in still matches that client's date.
	strongAfter = 2 * time.Second
)

// verdict is what to do with a Range field before the file server sees it.
type verdict int

const (
	pass    verdict = iota // as is; the file server rejects what it cannot parse
	rewrite                // spelled the way the file server parses it
	ignore                 // without the field, so the whole file is sent
)

// plan is what to do with a Range field; for rewrite, canonical is the field to use.
type plan struct {
	canonical []byte
	verdict   verdict
}

// classify decides what to do with a Range field, building a rewrite in dst.
func classify(dst, field []byte) plan {
	unit, set, found := utils.CutByte(field, '=')
	unit = utils.TrimSpace(unit)
	if !found || len(unit) == 0 {
		return plan{verdict: pass}
	}
	// A Range with an unknown unit must be ignored (§14.2).
	if !utils.EqualFold(utils.UnsafeString(unit), unitBytes) {
		return plan{verdict: ignore}
	}

	// The file server answers one range. Several are ignored, which §14.2 allows,
	// rather than refused with a 416.
	var spec []byte
	for set != nil {
		var element []byte
		element, set, found = utils.CutByte(set, ',')
		if !found {
			set = nil
		}
		// Empty list elements are ignored (§5.6.1.2).
		element = utils.TrimSpace(element)
		if len(element) == 0 {
			continue
		}
		if spec != nil {
			return plan{verdict: ignore}
		}
		spec = element
	}
	if spec == nil {
		return plan{verdict: pass}
	}

	canonical := append(append(dst[:0], unitBytes...), '=')
	canonical = append(canonical, spec...)
	if bytes.Equal(canonical, field) {
		return plan{verdict: pass}
	}
	return plan{canonical: canonical, verdict: rewrite}
}

// Wrap returns handler, a fasthttp file server with byte ranges enabled, with the
// Range field handled as RFC 9110 asks. The field is ignored, and the whole file
// sent, for a non-GET request, a unit other than "bytes", several ranges, or an
// If-Range that does not match the Last-Modified the file server sent.
func Wrap(handler fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		serveAt(ctx, handler, time.Now)
	}
}

// serveAt is the handler Wrap returns, with the clock that ages a Last-Modified given.
func serveAt(ctx *fasthttp.RequestCtx, handler fasthttp.RequestHandler, now func() time.Time) {
	header := &ctx.Request.Header
	field := header.Peek(fasthttp.HeaderRange)
	if len(field) == 0 {
		handler(ctx)
		return
	}

	// Only GET has range semantics (§14.2).
	var buf [64]byte
	p := plan{verdict: ignore}
	if ctx.IsGet() {
		p = classify(buf[:0], field)
	}

	ifRange := header.Peek(fasthttp.HeaderIfRange)
	if len(ifRange) == 0 || p.verdict == ignore {
		p.serve(ctx, handler)
		return
	}
	// The value is copied because serving rearranges the request header's storage.
	serveConditional(ctx, handler, p, utils.CopyBytes(ifRange), now)
}

// serve runs handler for the Range field as the plan has it.
func (p plan) serve(ctx *fasthttp.RequestCtx, handler fasthttp.RequestHandler) {
	switch p.verdict {
	case pass:
		handler(ctx)
	case rewrite:
		serveWithField(ctx, handler, p.canonical)
	default:
		serveWithField(ctx, handler, nil)
	}
}

// serveWithField runs handler with the Range field set to value, or removed when
// value is nil, and puts the original back for the rest of the chain.
func serveWithField(ctx *fasthttp.RequestCtx, handler fasthttp.RequestHandler, value []byte) {
	header := &ctx.Request.Header
	original := utils.CopyBytes(header.Peek(fasthttp.HeaderRange))
	defer header.SetBytesV(fasthttp.HeaderRange, original)

	if value == nil {
		header.Del(fasthttp.HeaderRange)
	} else {
		header.SetBytesV(fasthttp.HeaderRange, value)
	}
	handler(ctx)
}

// serveConditional serves a Range that comes with an If-Range. Only the response
// has the validators to judge it by, so when they do not match, the whole file is
// served in its place.
func serveConditional(ctx *fasthttp.RequestCtx, handler fasthttp.RequestHandler, p plan, ifRange []byte, now func() time.Time) {
	// A refusal wipes the response, so the headers the caller had set are kept to
	// start the second attempt from.
	callerHeader := &fasthttp.ResponseHeader{}
	ctx.Response.Header.CopyTo(callerHeader)

	p.serve(ctx, handler)

	switch ctx.Response.StatusCode() {
	case fasthttp.StatusPartialContent:
		if !ifRangeMatches(ifRange, &ctx.Response, now()) {
			serveWhole(ctx, handler, callerHeader)
		}
	case fasthttp.StatusRequestedRangeNotSatisfiable:
		// The 416 has no validators. The whole file has, and is the answer when
		// If-Range does not match.
		serveWhole(ctx, handler, callerHeader)
		if ctx.Response.StatusCode() == fasthttp.StatusOK && ifRangeMatches(ifRange, &ctx.Response, now()) {
			ctx.Error("Range Not Satisfiable", fasthttp.StatusRequestedRangeNotSatisfiable)
		}
	}
}

// serveWhole discards the response and has handler send the whole file, from the
// headers the caller had set.
func serveWhole(ctx *fasthttp.RequestCtx, handler fasthttp.RequestHandler, callerHeader *fasthttp.ResponseHeader) {
	ctx.Response.ResetBody()
	callerHeader.CopyTo(&ctx.Response.Header)
	serveWithField(ctx, handler, nil)
}

// ifRangeMatches evaluates If-Range against the file server's response (§13.1.5):
// an entity-tag must equal a strong ETag, a date the exact, strong Last-Modified,
// judged at now. The file server sends no ETag of its own, and a weak tag never matches.
func ifRangeMatches(ifRange []byte, resp *fasthttp.Response, now time.Time) bool {
	ifRange = utils.TrimSpace(ifRange)
	if len(ifRange) == 0 {
		return false
	}

	if ifRange[0] == '"' || strings.HasPrefix(utils.UnsafeString(ifRange), weakPrefix) {
		etag := resp.Header.Peek(fasthttp.HeaderETag)
		return ifRange[0] == '"' && len(etag) > 0 && etag[0] == '"' && bytes.Equal(etag, ifRange)
	}

	lastModified := resp.Header.Peek(fasthttp.HeaderLastModified)
	if !bytes.Equal(lastModified, ifRange) {
		return false
	}
	modified, err := fasthttp.ParseHTTPDate(lastModified)
	return err == nil && now.Sub(modified) >= strongAfter
}
