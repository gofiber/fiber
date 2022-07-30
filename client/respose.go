package client

import (
	"sync"

	"github.com/valyala/fasthttp"
)

type Response struct {
	rawResponse *fasthttp.Response
}

// Reset clear Response object.
func (r *Response) Reset() {
	r.rawResponse.Reset()
}

var responsePool sync.Pool

// AcquireResponse returns an empty core object from the pool.
//
// The returned core may be returned to the pool with ReleaseResponse when no longer needed.
// This allows reducing GC load.
func AcquireResponse() (resp *Response) {
	respv := responsePool.Get()
	if respv != nil {
		resp = respv.(*Response)
		return
	}
	resp = &Response{
		rawResponse: fasthttp.AcquireResponse(),
	}

	return
}

// ReleaseResponse returns the object acquired via AcquireResponse to the pool.
//
// Do not access the released core object, otherwise data races may occur.
func ReleaseResponse(resp *Response) {
	resp.Reset()
	responsePool.Put(resp)
}
