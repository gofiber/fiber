// Package tlshandshake bounds the TLS handshake of a fasthttp host client by the time its request has.
package tlshandshake

import (
	"errors"
	"net"
	"time"

	"github.com/valyala/fasthttp"
)

// Bound bounds what a new connection of hc may take of the time its request has left: the dial and, as fasthttp
// handshakes inside the first write where only a write deadline applies, the TLS handshake. It wraps the dialers hc
// has, so it goes last.
func Bound(hc *fasthttp.HostClient) {
	dialTimeout := hc.DialTimeout
	if dialTimeout == nil {
		dialTimeout = func(addr string, timeout time.Duration) (net.Conn, error) {
			// Read when dialing: fasthttp prefers DialTimeout, so a Dial a later hook sets would be passed over.
			if dial := hc.Dial; dial != nil {
				return dial(addr)
			}
			// fasthttp's own dialer allows 1000 connects in progress per process: hold a slot no longer than its default.
			if timeout <= 0 || timeout > fasthttp.DefaultDialTimeout {
				timeout = fasthttp.DefaultDialTimeout
			}
			if hc.DialDualStack {
				return fasthttp.DialDualStackTimeout(addr, timeout)
			}
			return fasthttp.DialTimeout(addr, timeout)
		}
		hc.DialTimeout = dialTimeout
	}
	if !hc.IsTLS || hc.WriteTimeout > 0 { // with WriteTimeout fasthttp handshakes at dial time, under a deadline
		return
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
