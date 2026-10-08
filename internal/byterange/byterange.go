// Package byterange applies the rules for the Range and If-Range fields of a
// request (RFC 9110 Section 13.1.5 and Section 14.2) that fasthttp's file server
// leaves out. That server answers one byte range and nothing else: it refuses a
// perfectly satisfiable request for several ranges with 416, takes the unit
// "bytes" only in lower case, applies Range to HEAD, and has no If-Range, so a
// resumed download is spliced onto a file that has changed since.
//
// It is shared by Ctx.SendFile and the static middleware, which cannot look at
// the file before the file server does.
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
	fieldContentRange = "Content-Range"
	fieldETag         = "ETag"
	fieldLastModified = "Last-Modified"

	// unitBytes is the one range unit the file server understands. The unit is
	// case-insensitive (RFC 9110 Section 14.1), the file server's parser is not.
	unitBytes = "bytes"

	// A Last-Modified time is a strong validator only when the file cannot have
	// changed twice within the second it names (RFC 9110 Section 8.8.2.2), which
	// the server can tell once that second is over.
	strongAfter = time.Second
)

var (
	bytesUnit  = []byte(unitBytes)
	weakPrefix = []byte("W/")
)

// verdict is what to do with the Range field of a request before the file server
// sees it.
type verdict int

const (
	// pass hands the request on as it is. A field the file server cannot parse
	// is left to refuse it: an invalid ranges-specifier may be ignored or
	// rejected (Section 14.2).
	pass verdict = iota
	// rewrite hands the request on with the field spelled the way the file
	// server parses it.
	rewrite
	// ignore hands the request on without the field, so the whole
	// representation is sent.
	ignore
)

// classify decides what to do with a Range field. For rewrite it returns the
// field to use, built in dst.
func classify(dst, field []byte) (canonical []byte, v verdict) { //nolint:nonamedreturns // the two results are easy to swap without names
	unit, set, found := utils.CutByte(field, '=')
	unit = utils.TrimSpace(unit)
	if !found || len(unit) == 0 {
		return nil, pass
	}
	// An origin server must ignore a Range field with a unit it does not
	// understand (Section 14.2).
	if !utils.EqualFold(unit, bytesUnit) {
		return nil, ignore
	}

	// The file server answers one range. A request for several is ignored, which
	// is allowed (Section 14.2) and always correct, rather than refused with a 416
	// that is for ranges that cannot be satisfied (Section 15.5.17).
	var spec []byte
	for set != nil {
		var element []byte
		element, set, found = utils.CutByte(set, ',')
		if !found {
			set = nil
		}
		// Empty list elements are ignored (Section 5.6.1.2).
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

// Serve runs handler, a fasthttp file server that accepts byte ranges, for ctx.
//
// A Range field is ignored, and the whole representation sent, when the request
// is not a GET (the only method that has range semantics), when its unit is not
// "bytes", and when it asks for several ranges. A single range is answered by the
// file server. When the request also carries If-Range and the validator it holds
// does not match what the file server sent, the Range field must be ignored too
// (Section 13.1.5), and the whole representation is sent in place of the partial
// response.
func Serve(ctx *fasthttp.RequestCtx, handler fasthttp.RequestHandler) {
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

	// With If-Range the answer may turn out to be the whole file after the file
	// server has refused the range, and a refusal wipes the response. The
	// headers the caller had set are kept, to start the second attempt from the
	// state the first started from.
	// The value is copied: it lives in the request header's storage, which
	// taking the Range field out and putting it back rearranges.
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
		if ifRangeMatches(ifRange, &ctx.Response) {
			return
		}
		replaceWithWhole(ctx, handler, callerHeader)
	case fasthttp.StatusRequestedRangeNotSatisfiable:
		// The 416 carries no validator. The whole representation does, and it is
		// the answer if the one in If-Range does not match.
		replaceWithWhole(ctx, handler, callerHeader)
		if ctx.Response.StatusCode() == fasthttp.StatusOK && ifRangeMatches(ifRange, &ctx.Response) {
			_ = ctx.Response.CloseBodyStream() //nolint:errcheck // a stream never sent
			ctx.Error("Range Not Satisfiable", fasthttp.StatusRequestedRangeNotSatisfiable)
		}
	}
}

// serveWithoutRange runs handler for a request taken to carry no Range field. The
// field is put back, since the rest of the chain may read it.
func serveWithoutRange(ctx *fasthttp.RequestCtx, handler fasthttp.RequestHandler) {
	header := &ctx.Request.Header
	original := utils.CopyBytes(header.Peek(fieldRange))
	header.Del(fieldRange)
	handler(ctx)
	header.SetBytesV(fieldRange, original)
}

// replaceWithWhole takes the first response back and has handler send the whole
// representation instead, from the headers the caller had set.
func replaceWithWhole(ctx *fasthttp.RequestCtx, handler fasthttp.RequestHandler, callerHeader *fasthttp.ResponseHeader) {
	_ = ctx.Response.CloseBodyStream() //nolint:errcheck // a stream never sent
	ctx.Response.ResetBody()
	callerHeader.CopyTo(&ctx.Response.Header)
	serveWithoutRange(ctx, handler)
}

// ifRangeMatches evaluates an If-Range field against the response the file server
// produced (Section 13.1.5). An entity-tag matches the response's ETag under the
// strong comparison; a date matches only a Last-Modified that is exactly equal and
// strong. The file server sends no ETag unless it is told to, in which case no
// entity-tag can match, and a weak tag never does.
func ifRangeMatches(ifRange []byte, resp *fasthttp.Response) bool {
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
	return err == nil && time.Since(modified) >= strongAfter
}
