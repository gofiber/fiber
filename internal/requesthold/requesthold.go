// Package requesthold keeps the fasthttp.RequestCtx of a request out of reuse for as long as the net/http
// handler adapted for it runs.
//
// fasthttpadaptor builds the *http.Request out of views of the buffers of the RequestCtx (method, URL, host,
// every header key and value, the body) and makes the RequestCtx its context. It runs the handler in a
// goroutine, and once the handler flushes the response it returns, without waiting for the handler.
// When the client leaves, the server closes the response stream, takes the RequestCtx back and the next
// request overwrites its buffers while the handler still runs: its r.Header.Get, r.URL.Path, r.Host,
// r.Method, the strings it read earlier and r.Context().Value then answer for another request. A net/http
// server never does that to a handler. The adaptor is in fasthttp and cannot be changed from here.
//
// The handler returned by NewFastHTTPHandler registers a hold in the user values of the RequestCtx. fasthttp
// closes the user values of a request when it resets it for the next one, which happens after the
// response was written and before the buffers are reused, and the hold's Close waits for the handler.
// A handler that returns before the adaptor does, which is nearly all of them, leaves nothing to wait for.
package requesthold

import (
	"net/http"
	"sync"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

type holdKey struct{}

// hold counts the net/http handler of one request that is still running.
type hold struct {
	resp *fasthttp.Response
	wg   sync.WaitGroup
}

func acquire() *hold {
	if hd, ok := holds.Get().(*hold); ok {
		return hd
	}
	return new(hold)
}

var holds = sync.Pool{New: func() any { return new(hold) }}

// Close ends the response stream, so that a handler blocked on a write returns, and waits for the handler.
// fasthttp calls it when it resets the request, after it wrote the response.
func (h *hold) Close() error {
	if h.resp != nil {
		_ = h.resp.CloseBodyStream() //nolint:errcheck // nothing to do with it: the response is done
	}
	h.wg.Wait()
	return nil
}

// first stores hd on ctx as its first user value. fasthttp closes and clears the user values of a request in
// order when it resets it, so the values stored ahead of the hold would be cleared while the handler
// still reads them, and only those after it are kept until it returns.
func first(ctx *fasthttp.RequestCtx, hd *hold) {
	hd.resp = &ctx.Response
	ctx.SetUserValue(holdKey{}, hd)
	var key, value any
	ctx.VisitUserValuesAll(func(k, v any) {
		if key == nil {
			key, value = k, v
		}
	})
	if _, isHold := key.(holdKey); !isHold && key != nil {
		// Remove moves the last value, the hold, into the slot of the removed one.
		ctx.RemoveUserValue(key)
		ctx.SetUserValue(key, value)
	}
}

// Register puts a hold on ctx now. A value stored on ctx afterwards is kept until the handler returns, so a
// caller that stores one for the handler to read, as adaptor.HTTPHandlerWithContext does, registers first.
// NewFastHTTPHandler takes a hold registered this way.
func Register(ctx *fasthttp.RequestCtx) {
	if _, ok := ctx.UserValue(holdKey{}).(*hold); !ok {
		first(ctx, acquire())
	}
}

// NewFastHTTPHandler is fasthttpadaptor.NewFastHTTPHandler(h) that does not let the RequestCtx be reused
// while h still runs.
func NewFastHTTPHandler(h http.Handler) fasthttp.RequestHandler {
	adapted := fasthttpadaptor.NewFastHTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.Context() is the RequestCtx, and Value its user values.
		if hd, ok := r.Context().Value(holdKey{}).(*hold); ok {
			hd.wg.Add(1)
			defer hd.wg.Done()
		}
		h.ServeHTTP(w, r)
	}))
	return func(ctx *fasthttp.RequestCtx) {
		hd, registered := ctx.UserValue(holdKey{}).(*hold)
		if !registered {
			hd = acquire()
			first(ctx, hd)
		}
		defer func() {
			// A streamed or hijacked response leaves the handler running: the hold stays until fasthttp
			// closes it. Otherwise the handler returned before the adaptor did, or never started.
			if ctx.Response.IsBodyStream() || ctx.Hijacked() {
				return
			}
			ctx.RemoveUserValue(holdKey{})
			hd.resp = nil
			holds.Put(hd)
		}()
		adapted(ctx)
	}
}
