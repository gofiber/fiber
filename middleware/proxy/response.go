package proxy

import (
	"io"

	"github.com/valyala/fasthttp"
)

// streamsResponses reports whether one of lbc's clients reads response bodies as streams.
func streamsResponses(lbc *fasthttp.LBClient) bool {
	for _, client := range lbc.Clients {
		switch c := client.(type) {
		case *fasthttp.HostClient:
			if c.StreamResponseBody {
				return true
			}
		case schemeClient:
			if c.StreamResponseBody {
				return true
			}
		}
	}
	return false
}

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
	upstream.StreamBody, upstream.SkipBody = res.StreamBody, res.SkipBody
	return upstream
}

// adoptUpstreamResponse readies upstream, the response the client filled with err as the outcome, as res, the
// client's: sanitized and, when upstream is the proxy's own, its header section copied over and its body read
// from it through an upstreamBody, which releases it once the body has been sent, or copied along when the
// client buffered the body after all. upstream is released on an error.
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
	if !upstream.IsBodyStream() {
		// A body of known size within the client's bound, or none, is buffered.
		streamBody := res.StreamBody
		upstream.CopyTo(res)
		res.StreamBody = streamBody
		fasthttp.ReleaseResponse(upstream)
		return nil
	}
	upstream.Header.CopyTo(&res.Header)
	res.SetBodyStream(&upstreamBody{Reader: upstream.BodyStream(), upstream: upstream}, upstream.Header.ContentLength())
	return nil
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
