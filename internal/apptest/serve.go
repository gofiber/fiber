package apptest

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp/fasthttputil"
)

// Server is the part of an app that serves a listener and shuts down, as *fiber.App does.
type Server[C any] interface {
	Listener(ln net.Listener, config ...C) error
	Shutdown() error
}

// Serve serves app on an in-memory listener until tb ends, when the app is shut down and the
// error Listener returned is checked, and returns the listener, ready to dial: a dial that the
// server never accepts fails the test instead of hanging.
func Serve[C any](tb testing.TB, app Server[C], config ...C) *fasthttputil.InmemoryListener {
	tb.Helper()

	ln := fasthttputil.NewInmemoryListener()
	errCh := make(chan error, 1)
	go func() { errCh <- app.Listener(ln, config...) }()
	tb.Cleanup(func() {
		require.NoError(tb, app.Shutdown())
		if err := <-errCh; err != nil && !errors.Is(err, net.ErrClosed) {
			require.NoError(tb, err)
		}
	})
	require.Eventually(tb, func() bool {
		conn, err := ln.Dial()
		return err == nil && conn.Close() == nil
	}, time.Second, 5*time.Millisecond)
	return ln
}

// Dial opens a connection to ln with a five-second deadline, closed when tb ends, for a test
// that writes raw requests and reads what comes back.
func Dial(tb testing.TB, ln *fasthttputil.InmemoryListener) net.Conn {
	tb.Helper()

	conn, err := ln.Dial()
	require.NoError(tb, err)
	tb.Cleanup(func() { _ = conn.Close() }) //nolint:errcheck // usually closed by the server already
	require.NoError(tb, conn.SetDeadline(time.Now().Add(5*time.Second)))
	return conn
}

// FieldLines returns n header lines named prefix followed by the line's index, each holding
// value, for a raw request or response with more lines than a limit allows.
func FieldLines(prefix, value string, n int) string {
	var lines strings.Builder
	for i := range n {
		fmt.Fprintf(&lines, "%s%d: %s\r\n", prefix, i, value)
	}
	return lines.String()
}
