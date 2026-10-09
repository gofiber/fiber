// Package byterange adds the Range and If-Range rules (RFC 9110 §13.1.5, §14.2)
// that fasthttp's file server lacks: it refuses several ranges with 416, takes
// only a lower-case "bytes" unit, applies Range to HEAD and ignores If-Range.
// It is shared by Ctx.SendFile and the static middleware.
package byterange

import (
	"bytes"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

const (
	fieldRange        = "Range"
	fieldIfRange      = "If-Range"
	fieldETag         = "ETag"
	fieldLastModified = "Last-Modified"

	// unitBytes is the only unit the file server knows. Units are
	// case-insensitive (RFC 9110 §14.1), its parser is not.
	unitBytes = "bytes"

	// A Last-Modified time is implicitly weak unless it can be deduced to be strong
	// (§8.8.2.2). It has whole-second resolution, so one that is two seconds old
	// belongs to a file last modified more than a second ago.
	strongAfter = 2 * time.Second
)

var (
	bytesUnit  = []byte(unitBytes)
	weakPrefix = []byte("W/")
)

// verdict is what to do with a Range field before the file server sees it.
type verdict int

const (
	pass    verdict = iota // as is; the file server rejects what it cannot parse
	rewrite                // spelled the way the file server parses it
	ignore                 // without the field, so the whole file is sent
)

// classify decides what to do with a Range field. For rewrite it returns the
// field to use, built in dst.
func classify(dst, field []byte) (canonical []byte, v verdict) { //nolint:nonamedreturns // the two results are easy to swap without names
	unit, set, found := utils.CutByte(field, '=')
	unit = utils.TrimSpace(unit)
	if !found || len(unit) == 0 {
		return nil, pass
	}
	// A Range with an unknown unit must be ignored (§14.2).
	if !utils.EqualFold(unit, bytesUnit) {
		return nil, ignore
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
			return nil, ignore
		}
		spec = element
	}
	if spec == nil {
		return nil, pass
	}

	canonical = append(append(dst[:0], unitBytes...), '=')
	canonical = append(canonical, spec...)
	if bytes.Equal(canonical, field) {
		return nil, pass
	}
	return canonical, rewrite
}

// Serve runs handler, a fasthttp file server with byte ranges enabled, for ctx.
// The Range field is ignored, and the whole file sent, for a non-GET request, a
// unit other than "bytes", several ranges, or an If-Range that does not match
// the Last-Modified the file server sent.
func Serve(ctx *fasthttp.RequestCtx, handler fasthttp.RequestHandler) {
	serveAt(ctx, handler, time.Now)
}

// serveAt is Serve with the clock that ages a Last-Modified given, for the tests.
func serveAt(ctx *fasthttp.RequestCtx, handler fasthttp.RequestHandler, now func() time.Time) {
	header := &ctx.Request.Header
	field := header.Peek(fieldRange)
	if len(field) == 0 {
		handler(ctx)
		return
	}

	if !ctx.IsGet() {
		serveWithoutRange(ctx, handler)
		return
	}

	var buf [64]byte
	canonical, v := classify(buf[:0], field)
	if v == ignore {
		serveWithoutRange(ctx, handler)
		return
	}

	// With If-Range the file may have to be served again, and a refusal wipes the
	// response, so the caller's headers are saved. The value is copied because
	// removing and restoring Range rearranges the request header's storage.
	var (
		callerHeader *fasthttp.ResponseHeader
		ifRange      []byte
	)
	if value := header.Peek(fieldIfRange); len(value) > 0 {
		ifRange = utils.CopyBytes(value)
		callerHeader = &fasthttp.ResponseHeader{}
		ctx.Response.Header.CopyTo(callerHeader)
	}

	if v == rewrite {
		original := utils.CopyBytes(field)
		header.SetBytesV(fieldRange, canonical)
		handler(ctx)
		header.SetBytesV(fieldRange, original)
	} else {
		handler(ctx)
	}

	if callerHeader == nil {
		return
	}
	switch ctx.Response.StatusCode() {
	case fasthttp.StatusPartialContent:
		if ifRangeMatches(ifRange, &ctx.Response, now()) {
			return
		}
		replaceWithWhole(ctx, handler, callerHeader)
	case fasthttp.StatusRequestedRangeNotSatisfiable:
		// The 416 has no validator. The whole file has, and is the answer when
		// If-Range does not match.
		replaceWithWhole(ctx, handler, callerHeader)
		if ctx.Response.StatusCode() == fasthttp.StatusOK && ifRangeMatches(ifRange, &ctx.Response, now()) {
			_ = ctx.Response.CloseBodyStream() //nolint:errcheck // a stream never sent
			ctx.Error("Range Not Satisfiable", fasthttp.StatusRequestedRangeNotSatisfiable)
		}
	}
}

// serveWithoutRange runs handler as if the request had no Range, then restores
// the field for the rest of the chain.
func serveWithoutRange(ctx *fasthttp.RequestCtx, handler fasthttp.RequestHandler) {
	header := &ctx.Request.Header
	original := utils.CopyBytes(header.Peek(fieldRange))
	header.Del(fieldRange)
	handler(ctx)
	header.SetBytesV(fieldRange, original)
}

// replaceWithWhole discards the response and has handler send the whole file,
// from the headers the caller had set.
func replaceWithWhole(ctx *fasthttp.RequestCtx, handler fasthttp.RequestHandler, callerHeader *fasthttp.ResponseHeader) {
	_ = ctx.Response.CloseBodyStream() //nolint:errcheck // a stream never sent
	ctx.Response.ResetBody()
	callerHeader.CopyTo(&ctx.Response.Header)
	serveWithoutRange(ctx, handler)
}

// ifRangeMatches evaluates If-Range against the file server's response (§13.1.5):
// an entity-tag must equal a strong ETag, a date the exact, strong Last-Modified,
// judged at now. The file server sends no ETag of its own, and a weak tag never matches.
func ifRangeMatches(ifRange []byte, resp *fasthttp.Response, now time.Time) bool {
	ifRange = utils.TrimSpace(ifRange)
	if len(ifRange) == 0 {
		return false
	}

	if ifRange[0] == '"' || bytes.HasPrefix(ifRange, weakPrefix) {
		etag := resp.Header.Peek(fieldETag)
		return ifRange[0] == '"' && len(etag) > 0 && etag[0] == '"' && bytes.Equal(etag, ifRange)
	}

	lastModified := resp.Header.Peek(fieldLastModified)
	if !bytes.Equal(lastModified, ifRange) {
		return false
	}
	modified, err := fasthttp.ParseHTTPDate(lastModified)
	return err == nil && now.Sub(modified) >= strongAfter
}
