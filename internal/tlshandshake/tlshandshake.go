// Package tlshandshake bounds the TLS handshake of a fasthttp host client by the time its request has.
package tlshandshake

import (
	"errors"
	"net"
	"time"

	"github.com/valyala/fasthttp"
)

// Bound gives a new TLS connection of hc a read deadline of the time its request has left: fasthttp handshakes inside
// the first write, where only a write deadline applies. It wraps the dialers hc has, so it goes last.
func Bound(hc *fasthttp.HostClient) {
	if !hc.IsTLS || hc.WriteTimeout > 0 { // with WriteTimeout fasthttp handshakes at dial time, under a deadline
		return
	}
	dialTimeout := hc.DialTimeout
	if dialTimeout == nil {
		dialTimeout = func(addr string, timeout time.Duration) (net.Conn, error) {
			// The dialer fasthttp's callDialFunc would pick; addr already carries the port. Dial is read when
			// dialing, so one a later hook sets is not passed over.
			switch {
			case hc.Dial != nil:
				return hc.Dial(addr)
			case timeout <= 0 && hc.DialDualStack:
				return fasthttp.DialDualStack(addr)
			case timeout <= 0:
				return fasthttp.Dial(addr)
			case hc.DialDualStack:
				return fasthttp.DialDualStackTimeout(addr, timeout)
			default:
				return fasthttp.DialTimeout(addr, timeout)
			}
		}
	}
	hc.DialTimeout = func(addr string, timeout time.Duration) (net.Conn, error) {
		deadline := time.Now().Add(timeout)
		conn, err := dialTimeout(addr, timeout)
		if err != nil || conn == nil {
			return conn, err
		}
		if timeout <= 0 {
			// fasthttp also dials without a timeout for a request that waited for a free connection,
			// and pools that connection unused if the request gave up: the first write bounds it.
			if _, isTLS := conn.(interface{ Handshake() error }); isTLS {
				return conn, nil
			}
			return handshakeConn{conn}, nil
		}
		// The request sets its own read deadline after the handshake, replacing this one.
		if err := conn.SetReadDeadline(deadline); err != nil {
			return nil, errors.Join(err, conn.Close())
		}
		return conn, nil
	}
}

// handshakeConn lets a request's write deadline bound reads too, so the TLS handshake inside the
// first write ends with the request that makes it; fasthttp sets the read deadline after the write.
type handshakeConn struct {
	net.Conn
}

func (c handshakeConn) SetWriteDeadline(t time.Time) error {
	return c.SetDeadline(t) //nolint:wrapcheck // the connection's own error, passed through
}
