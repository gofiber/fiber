package client

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
)

// connFaults are the errors a faultConn answers with: its deadline setters and its writes fail as set.
type connFaults struct {
	deadline, writeDeadline, readDeadline, write error
}

// faultConn is a connection that fails as its faults say and sends on closed once when it is closed.
type faultConn struct {
	net.Conn
	closed chan<- struct{}
	faults connFaults
	once   sync.Once
}

func (c *faultConn) SetDeadline(t time.Time) error {
	if c.faults.deadline != nil {
		return c.faults.deadline
	}
	return c.Conn.SetDeadline(t) //nolint:wrapcheck // a transparent wrapper
}

func (c *faultConn) SetWriteDeadline(t time.Time) error {
	if c.faults.writeDeadline != nil {
		return c.faults.writeDeadline
	}
	return c.Conn.SetWriteDeadline(t) //nolint:wrapcheck // a transparent wrapper
}

func (c *faultConn) SetReadDeadline(t time.Time) error {
	if c.faults.readDeadline != nil {
		return c.faults.readDeadline
	}
	return c.Conn.SetReadDeadline(t) //nolint:wrapcheck // a transparent wrapper
}

func (c *faultConn) Write(p []byte) (int, error) {
	if c.faults.write != nil {
		return 0, c.faults.write
	}
	return c.Conn.Write(p) //nolint:wrapcheck // a transparent wrapper
}

func (c *faultConn) Close() error {
	c.once.Do(func() { c.closed <- struct{}{} })
	return c.Conn.Close() //nolint:wrapcheck // a transparent wrapper
}

// timeoutError is a write error that reports a timeout, as a passed deadline does.
type timeoutError struct{}

func (timeoutError) Error() string { return "write timed out" }
func (timeoutError) Timeout() bool { return true }

// faultDial dials ts through a faultConn with faults, sending on closed when the connection is closed.
func faultDial(ts *testServer, faults connFaults, closed chan<- struct{}) fasthttp.DialFunc {
	return func(addr string) (net.Conn, error) {
		conn, err := ts.dial()(addr)
		if err != nil {
			return nil, err
		}
		return &faultConn{Conn: conn, closed: closed, faults: faults}, nil
	}
}

// okServer serves "ok" at / and stops when the test ends.
func okServer(t *testing.T) *testServer {
	t.Helper()
	ts := startTestServer(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })
	})
	t.Cleanup(ts.stop)
	return ts
}

// Test_RoundTripCancelable_Faults checks the exchange of a client made by New under a context that can
// be canceled, which the client's own transport round-trips: the connection is closed, never pooled,
// when a deadline cannot be set or the write fails, a write that times out reports fasthttp's timeout,
// the host client's read and write timeouts bound the exchange as under fasthttp's transport, and
// unnormalized header names and a streamed response go through as usual.
func Test_RoundTripCancelable_Faults(t *testing.T) {
	t.Parallel()
	errDeadline := errors.New("deadline refused")
	errWrite := errors.New("write refused")
	for _, tc := range []struct {
		faults   connFaults
		err      error
		setup    func(c *fasthttp.Client)
		name     string
		timeouts bool // the host client's read and write timeouts are set
	}{
		{name: "a failing deadline ends the attempt", faults: connFaults{deadline: errDeadline}, err: errDeadline},
		{name: "a failing write deadline ends the attempt", faults: connFaults{writeDeadline: errDeadline}, timeouts: true, err: errDeadline},
		{name: "a failing read deadline ends the attempt", faults: connFaults{readDeadline: errDeadline}, timeouts: true, err: errDeadline},
		{name: "a failing write ends the attempt", faults: connFaults{write: errWrite}, err: errWrite},
		{name: "a write that times out reports the timeout", faults: connFaults{write: timeoutError{}}, err: fasthttp.ErrTimeout},
		{name: "the host client's timeouts bound the exchange", timeouts: true},
		{name: "unnormalized header names go through", setup: func(c *fasthttp.Client) { c.DisableHeaderNamesNormalizing = true }},
		{name: "a streamed response keeps fasthttp's transport", setup: func(c *fasthttp.Client) { c.StreamResponseBody = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			closed := make(chan struct{}, 16)
			client := New().SetDial(faultDial(okServer(t), tc.faults, closed))
			hc := client.FasthttpClient()
			hc.MaxIdemponentCallAttempts = 1 // the error reported is the attempt's own
			if tc.timeouts {
				hc.ReadTimeout, hc.WriteTimeout = time.Second, time.Second
			}
			if tc.setup != nil {
				tc.setup(hc)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			resp, err := client.R().SetContext(ctx).Get("http://example.com/")
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				apptest.Recv(t, closed, 2*time.Second, "the connection of the failed attempt was not closed")
				return
			}
			require.NoError(t, err)
			require.Equal(t, "ok", resp.String())
			resp.Close()
		})
	}
}

// Test_RoundTripCancelable_CanceledBeforeAttach checks that a connection dialed for a call whose context
// was canceled during the dial is closed rather than used or pooled.
func Test_RoundTripCancelable_CanceledBeforeAttach(t *testing.T) {
	t.Parallel()
	ts := okServer(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	closed := make(chan struct{}, 1)
	proceed := make(chan struct{})
	client := New().SetDial(func(addr string) (net.Conn, error) {
		conn, err := ts.dial()(addr)
		if err != nil {
			return nil, err
		}
		// The call ends while the dial is in progress; the dial returns once it has.
		cancel()
		<-proceed
		return &faultConn{Conn: conn, closed: closed}, nil
	})

	_, err := client.R().SetContext(ctx).Get("http://example.com/")
	require.ErrorIs(t, err, ErrTimeoutOrCancel)
	close(proceed)
	apptest.Recv(t, closed, 2*time.Second, "the connection dialed for the canceled call was not closed")
}

// Test_RoundTripCancelable_MaxConnDuration checks that a connection older than the host client's
// MaxConnDuration is sent its last request with Connection: close and closed after the response.
func Test_RoundTripCancelable_MaxConnDuration(t *testing.T) {
	t.Parallel()
	var closeAsked atomic.Bool
	ts := startTestServer(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			closeAsked.Store(c.Request().Header.ConnectionClose())
			return c.SendString("ok")
		})
	})
	t.Cleanup(ts.stop)
	closed := make(chan struct{}, 16)
	client := New().SetDial(faultDial(ts, connFaults{}, closed))
	client.FasthttpClient().MaxConnDuration = 10 * time.Millisecond
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// The pooled connection ages past MaxConnDuration between two requests. A slow run may age the
	// first one before its own check, which starts the next request on a fresh connection.
	for range 4 {
		resp, err := client.R().SetContext(ctx).Get("http://example.com/")
		require.NoError(t, err)
		require.Equal(t, "ok", resp.String())
		resp.Close()
		if closeAsked.Load() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.True(t, closeAsked.Load(), "no request asked the server to close the aged connection")
	apptest.Recv(t, closed, 2*time.Second, "the aged connection was not closed")
}
