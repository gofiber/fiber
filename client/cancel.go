package client

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

// errExchangeCanceled ends an exchange whose caller's context is done, canceled or
// past its deadline: the connection is closed under fasthttp, whose read or write
// then fails.
var errExchangeCanceled = errors.New("client: exchange canceled")

// cancelKey is the user value under which a request in flight carries the watch
// that can end its exchange: fasthttp hands its transport the request, not the
// context. Request.Reset clears it before the request goes back to the pool, and
// CopyTo does not carry it, so it is set per attempt (applyDeadline).
type cancelKey struct{}

// cancelContext is the watch a call holds for its exchanges: the connection in use
// is attached while the exchange runs, and end, called once the caller's context is
// done, closes it so the exchange ends at the transport and not only at the caller.
// The mutex orders end against attach and detach, so a connection whose close has
// started is never put back in the pool.
type cancelContext struct {
	conn  net.Conn
	mu    sync.Mutex
	ended bool
}

// attach starts watching conn and reports whether the watch is still on.
func (w *cancelContext) attach(conn net.Conn) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ended {
		return false
	}
	w.conn = conn
	return true
}

// detach stops watching the connection and reports whether end closed it.
func (w *cancelContext) detach() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.conn = nil
	return w.ended
}

// end closes the connection in use, if any, and refuses the ones to come.
func (w *cancelContext) end() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ended = true
	if w.conn != nil {
		_ = w.conn.Close() //nolint:errcheck // the close is the cancellation; fasthttp reports what it does to the exchange
	}
}

// cancelTransport is the RoundTripper of the host clients of a transport made by
// client.New. A request under a context that can end has its connection closed
// when the context is done, so the exchange ends at the transport and not only at
// the caller; every other request, and one whose response is streamed, goes
// through fasthttp's own transport unchanged.
type cancelTransport struct{}

// RoundTrip implements fasthttp.RoundTripper.
func (cancelTransport) RoundTrip(hc *fasthttp.HostClient, req *fasthttp.Request, resp *fasthttp.Response) (bool, error) {
	watch, ok := req.UserValue(cancelKey{}).(*cancelContext)
	if !ok || resp.StreamBody {
		return fasthttp.DefaultTransport.RoundTrip(hc, req, resp)
	}
	return roundTripCancelable(watch, hc, req, resp)
}

// roundTripCancelable is fasthttp's round trip for a buffered response, with the
// connection closed when the caller's context is done. It reports no retry for an
// exchange the context ended: nobody waits for it.
func roundTripCancelable(watch *cancelContext, hc *fasthttp.HostClient, req *fasthttp.Request, resp *fasthttp.Response) (bool, error) {
	// The request's timeout, what is left of the caller's deadline (applyDeadline),
	// bounds the dial, the wait for a free connection and the reads and writes, as
	// under fasthttp's own transport.
	timeout := req.GetTimeOut()
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	cc, err := hc.AcquireConn(timeout, req.ConnectionClose())
	if err != nil {
		return false, err
	}
	conn := cc.Conn()
	if !watch.attach(conn) {
		hc.CloseConn(cc)
		return false, errExchangeCanceled
	}
	fail := func(err error, retry bool) (bool, error) {
		canceled := watch.detach()
		hc.CloseConn(cc)
		if canceled {
			return false, errExchangeCanceled
		}
		return retry, err
	}

	resp.ParseNetConn(conn)

	// Set also when zero: the pooled connection may carry one from an earlier request.
	// The deadline is the exchange's for the write and the read alike unless the host
	// client times either by itself, which fasthttp counts from the write's end.
	timed := hc.WriteTimeout > 0 || hc.ReadTimeout > 0
	writeDeadline := deadline
	if hc.WriteTimeout > 0 {
		if t := time.Now().Add(hc.WriteTimeout); writeDeadline.IsZero() || t.Before(writeDeadline) {
			writeDeadline = t
		}
	}
	if timed {
		err = conn.SetWriteDeadline(writeDeadline)
	} else {
		err = conn.SetDeadline(deadline)
	}
	if err != nil {
		return fail(err, true)
	}

	resetConnection := false
	if hc.MaxConnDuration > 0 && time.Since(cc.CreatedTime()) > hc.MaxConnDuration && !req.ConnectionClose() {
		req.SetConnectionClose()
		resetConnection = true
	}

	bw := hc.AcquireWriter(conn)
	err = req.Write(bw)
	if resetConnection {
		req.Header.ResetConnectionClose()
	}
	if err == nil {
		err = bw.Flush()
	}
	hc.ReleaseWriter(bw)
	if err != nil {
		if x, ok := err.(interface{ Timeout() bool }); ok && x.Timeout() {
			err = fasthttp.ErrTimeout
		}
		return fail(err, true)
	}

	if timed {
		readDeadline := deadline
		if hc.ReadTimeout > 0 {
			if t := time.Now().Add(hc.ReadTimeout); readDeadline.IsZero() || t.Before(readDeadline) {
				readDeadline = t
			}
		}
		if err = conn.SetReadDeadline(readDeadline); err != nil {
			return fail(err, true)
		}
	}

	if req.Header.IsHead() {
		resp.SkipBody = true
	}
	if hc.DisableHeaderNamesNormalizing {
		resp.Header.DisableNormalizing()
	}

	br := hc.AcquireReader(conn)
	err = resp.ReadLimitBody(br, hc.MaxResponseBodySize)
	hc.ReleaseReader(br)
	if err != nil {
		// Not retried for ErrBodyTooLarge, which the same body would answer again.
		return fail(err, !errors.Is(err, fasthttp.ErrBodyTooLarge))
	}

	// A context done once the response was read in full has closed the connection
	// as well: it is not pooled, or the next request on it would fail.
	if watch.detach() || resetConnection || req.ConnectionClose() || resp.ConnectionClose() {
		hc.CloseConn(cc)
	} else {
		// ReleaseConn records that the deadlines were touched, so fasthttp's own
		// transport resets them before the next request on this connection.
		hc.ReleaseConn(cc)
	}
	return false, nil
}
