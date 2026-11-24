// Package tlshandshake bounds the TLS handshake of a fasthttp host client by the time its request has.
package tlshandshake

import (
	"errors"
	"net"
	"time"

	"github.com/valyala/fasthttp"
)

// Bound gives a new TLS connection of hc a read deadline of the time its request has left. fasthttp handshakes
// inside the first write, where only a write deadline applies. It wraps the dialers hc has, so it goes last.
func Bound(hc *fasthttp.HostClient) {
	if !hc.IsTLS || hc.WriteTimeout > 0 { // with WriteTimeout fasthttp handshakes at dial time, under a deadline
		return
	}
	dial, dialTimeout, dualStack := hc.Dial, hc.DialTimeout, hc.DialDualStack
	hc.DialTimeout = func(addr string, timeout time.Duration) (net.Conn, error) {
		deadline := time.Now().Add(timeout)
		var conn net.Conn
		var err error
		// The dialer fasthttp's callDialFunc would pick; addr already carries the port.
		switch {
		case dialTimeout != nil:
			conn, err = dialTimeout(addr, timeout)
		case dial != nil:
			conn, err = dial(addr)
		case timeout <= 0 && dualStack:
			conn, err = fasthttp.DialDualStack(addr)
		case timeout <= 0:
			conn, err = fasthttp.Dial(addr)
		case dualStack:
			conn, err = fasthttp.DialDualStackTimeout(addr, timeout)
		default:
			conn, err = fasthttp.DialTimeout(addr, timeout)
		}
		if err != nil || timeout <= 0 {
			return conn, err
		}
		// The request sets its own read deadline after the handshake, replacing this one.
		if err := conn.SetReadDeadline(deadline); err != nil {
			return nil, errors.Join(err, conn.Close())
		}
		return conn, nil
	}
}
