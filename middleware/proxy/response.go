package proxy

import (
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
// when the client buffered the body after all. upstream is released on an error.
func adoptUpstreamResponse(res, upstream *fasthttp.Response, err error, policy SecurityPolicy, except ...string) error {
	if err == nil {
		err = sanitizeUpstreamResponse(upstream, policy, except...)
	}
	if upstream == res {
		return err
	}
	if err != nil {
		fasthttp.ReleaseResponse(upstream)
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
	res.SetBodyStream(&upstreamBody{Reader: upstream.BodyStream(), upstream: upstream}, upstream.Header.ContentLength())
	return nil
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
// and released with it once it has been sent, which closes the stream and so releases its connection.
type upstreamBody struct {
	io.Reader
	upstream *fasthttp.Response
}

func (b *upstreamBody) Close() error {
	fasthttp.ReleaseResponse(b.upstream)
	return nil
}
