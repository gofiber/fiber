package client

import (
	"context"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

type Request struct {
	url        string
	method     string
	ctx        context.Context
	rawRequest *fasthttp.Request
}

func (r *Request) SetURL(url string) *Request {
	r.url = url
	return r
}

func (r *Request) SetMethod(method string) *Request {
	r.method = method
	return r
}

// Context returns the Context if its already set in request
// otherwise it creates new one using `context.Background()`.
func (r *Request) Context() context.Context {
	if r.ctx == nil {
		return context.Background()
	}
	return r.ctx
}

// SetContext sets the context.Context for current Request. It allows
// to interrupt the request execution if ctx.Done() channel is closed.
// See https://blog.golang.org/context article and the "context" package
// documentation.
func (r *Request) SetContext(ctx context.Context) *Request {
	r.ctx = ctx
	return r
}

// Reset clear Request object, used by ReleaseRequest method.
func (r *Request) Reset() {
	r.url = ""
	r.method = fiber.MethodGet
	r.ctx = nil

	r.rawRequest.Reset()
}

var requestPool sync.Pool

// AcquireRequest returns an empty core object from the pool.
//
// The returned core may be returned to the pool with ReleaseRequest when no longer needed.
// This allows reducing GC load.
func AcquireRequest() (req *Request) {
	reqv := requestPool.Get()
	if reqv != nil {
		req = reqv.(*Request)
		return
	}

	req = &Request{
		rawRequest: fasthttp.AcquireRequest(),
	}
	return
}

// ReleaseRequest returns the object acquired via AcquireRequest to the pool.
//
// Do not access the released core object, otherwise data races may occur.
func ReleaseRequest(req *Request) {
	req.Reset()
	requestPool.Put(req)
}
