package proxy

import (
	"errors"
	"io"

	"github.com/valyala/fasthttp"
)

// upstreamResponse returns the response the client fills with the upstream's: res, the client's own, or one
// of the proxy's when the body is read as a stream, by a client that streams or into a response whose
// StreamBody is set. Such a body ends with the upstream's trailer section, which fasthttp parses into the
// header of the response it streams into once the body has been read, after sanitizeUpstreamResponse ran:
// read from a response of the proxy's own, the body leaves the section there, see adoptUpstreamResponse.
func upstreamResponse(streams bool, res *fasthttp.Response) *fasthttp.Response { //nolint:revive // flag-parameter: streams is the client's setting
	if !streams && !res.StreamBody {
		return res
	}
	upstream := fasthttp.AcquireResponse()
	// The options the server set on res go along, as the client keeps them across its own reset:
	// whether a default Content-Type is added, which has no getter to carry alone.
	res.Header.CopyTo(&upstream.Header)
	upstream.StreamBody, upstream.SkipBody = res.StreamBody, res.SkipBody
	return upstream
}

// adoptUpstreamResponse readies upstream, the response the client filled with err as the outcome, as res, the
// client's: sanitized and, when upstream is the proxy's own, its header section copied over and its body read
// from it through an upstreamBody, which releases it once the body has been sent, or moved over without a copy
// when the client buffered the body after all. On an error res is emptied and upstream, if the proxy's own, released.
//
// A body the client streams is held to limit bytes when that is positive, which fasthttp does not do for one:
// it ignores a client's MaxResponseBodySize for a body it hands over as a stream. A length the response declares
// above it is refused before anything is sent, as a body read into memory would be, and a body that passes it
// without declaring one ends with fasthttp.ErrBodyTooLarge, which aborts the response.
func adoptUpstreamResponse(res, upstream *fasthttp.Response, err error, policy SecurityPolicy, limit int, except ...string) error { //nolint:revive // argument-limit: the bound is as much the caller's as the policy
	if err == nil {
		err = sanitizeUpstreamResponse(upstream, policy, except...)
	}
	if upstream == res {
		if err != nil {
			resetUpstreamResponse(res)
		}
		return err
	}
	if err == nil && limit > 0 && upstream.IsBodyStream() && upstream.Header.ContentLength() > limit {
		err = fasthttp.ErrBodyTooLarge
	}
	if err != nil {
		fasthttp.ReleaseResponse(upstream)
		// fasthttp empties a response it fills before sending, so res ends up empty on an error either way.
		resetUpstreamResponse(res)
		return err
	}
	upstream.Header.CopyTo(&res.Header)
	if !upstream.IsBodyStream() {
		// Buffered: a response without a body, as a HEAD's or a 204, or any by a client that does not stream.
		res.SkipBody = upstream.SkipBody
		moveBody(res, upstream)
		fasthttp.ReleaseResponse(upstream)
		return nil
	}
	res.SetBodyStream(&upstreamBody{Reader: upstream.BodyStream(), upstream: upstream, limit: int64(limit)}, upstream.Header.ContentLength())
	return nil
}

// resetUpstreamResponse empties a failed upstream response, whose fields were neither stripped nor seen by
// ModifyResponse, keeping the app's choice of no default Content-Type.
func resetUpstreamResponse(res *fasthttp.Response) {
	// The option has no getter: without a Content-Type, one is reported only where the default is added.
	res.Header.SetContentTypeBytes(nil)
	noDefaultContentType := len(res.Header.ContentType()) == 0
	res.Reset()
	res.Header.SetNoDefaultContentType(noDefaultContentType)
}

// moveBody gives res the buffered body of upstream without copying it: res takes upstream's buffer and
// upstream the one res had, to be released with it. A raw body, which a client of the caller's may have set
// in place of the buffer, res refers to as upstream did.
func moveBody(res, upstream *fasthttp.Response) {
	body := upstream.Body()
	buffer := upstream.SwapBody(nil) // drops a raw body, which Body returned above
	if len(buffer) != len(body) {
		res.SetBodyRaw(body)
		return
	}
	res.ResetBody()
	upstream.SwapBody(res.SwapBody(buffer))
}

// upstreamBody is the body of a streamed upstream response, read from the response the client streams into
// and released with it once it has been sent, which closes the stream and so releases its connection. It
// delivers at most limit bytes when that is positive, and then fails with fasthttp.ErrBodyTooLarge instead of
// the byte past it. The failure of a read, that one or the upstream's own, is kept: see failureStands.
type upstreamBody struct {
	io.Reader
	upstream *fasthttp.Response
	err      error // the failure of a read, a body past its limit included, which the reads after it return too
	limit    int64
	read     int64
}

func (b *upstreamBody) Read(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	// When p holds more than is left, one byte past it is asked for, to tell a body that ends at the
	// limit from one that goes on. Written as room < len(p), which cannot overflow, for a limit near
	// the largest int.
	if room := b.limit - b.read; b.limit > 0 && room < int64(len(p)) {
		p = p[:room+1]
	}
	n, err := b.Reader.Read(p)
	b.read += int64(n)
	if b.limit > 0 && b.read > b.limit {
		b.err = fasthttp.ErrBodyTooLarge
		return n - 1, b.err
	}
	if err != nil && !errors.Is(err, io.EOF) {
		// Kept with or without a limit: fasthttp's Body hands the caller the error's text, not the error.
		b.err = err
	}
	return n, err //nolint:wrapcheck // fasthttp expects the reader's own io.EOF
}

func (b *upstreamBody) Close() error {
	fasthttp.ReleaseResponse(b.upstream)
	return nil
}

// failureStands reports whether res still carries the failure of a read of b instead of a response the caller
// set after it: the stream itself, which would end the response early once it started, or what fasthttp's Body
// leaves in its place, the error's text as the body, which would go out as the upstream's response. A stream or a body set since is the caller's own and
// stays, as when one reads the stream and sends a fallback for the failure it was handed. A body that is
// exactly the error's text cannot be told from what Body left, so it counts as the failure: a caller that
// wants a status of its own returns an error.
func (b *upstreamBody) failureStands(res *fasthttp.Response) bool {
	if b.err == nil {
		return false
	}
	if res.IsBodyStream() {
		return res.BodyStream() == io.Reader(b)
	}
	return string(res.Body()) == b.err.Error()
}
