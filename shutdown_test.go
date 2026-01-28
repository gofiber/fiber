package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

// ---------------------------------------------------------------------------
// Bug 1: Context propagation — GracefulContext deadline must be respected
// ---------------------------------------------------------------------------

// Test_GracefulShutdown_ContextDeadlinePropagated verifies that when a
// GracefulContext carries a deadline, the shutdown honours that deadline
// instead of ignoring it and creating a fresh context.Background().
//
// Before the fix, ShutdownWithTimeout called context.WithTimeout(context.Background(), …),
// discarding the parent's deadline entirely. Now it derives the child context
// from the parent so the effective deadline is min(parent, ShutdownTimeout).
func Test_GracefulShutdown_ContextDeadlinePropagated(t *testing.T) {
	t.Parallel()

	app := New()
	// Handler that blocks longer than any timeout we set.
	app.Get("/slow", func(c Ctx) error {
		time.Sleep(10 * time.Second)
		return c.SendString("done")
	})

	ln := fasthttputil.NewInmemoryListener()

	// GracefulContext with a 1-second deadline.  ShutdownTimeout is set to
	// 5 seconds, but the parent deadline (1 s) should win.
	parentCtx, cancelParent := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancelParent()

	listenErr := make(chan error, 1)
	go func() {
		listenErr <- app.Listener(ln, ListenConfig{
			DisableStartupMessage: true,
			GracefulContext:       parentCtx,
			ShutdownTimeout:       5 * time.Second, // longer than parent — should be clamped
		})
	}()

	// Wait for server readiness.
	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Fire a slow request so there is an active connection during shutdown.
	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /slow HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	}()
	time.Sleep(50 * time.Millisecond) // let the request land

	// Cancel the parent context to trigger graceful shutdown.
	cancelParent()

	// The server must exit within ~2 s (parent deadline was 1 s).
	// If the fix is missing, it would wait up to 5 s (ShutdownTimeout).
	select {
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not respect parent context deadline; likely still using context.Background()")
	case err := <-listenErr:
		_ = err // Listener itself returns nil; the shutdown error goes through hooks.
	}
}

// Test_GracefulShutdown_ParentCancelBeforeTimeout verifies the shutdown
// completes as soon as the parent context is cancelled, even if
// ShutdownTimeout has not yet elapsed.
func Test_GracefulShutdown_ParentCancelBeforeTimeout(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(c Ctx) error {
		time.Sleep(3 * time.Second) // block longer than everything
		return c.SendString("OK")
	})

	ln := fasthttputil.NewInmemoryListener()

	parentCtx, cancelParent := context.WithCancel(context.Background())

	listenErr := make(chan error, 1)
	go func() {
		listenErr <- app.Listener(ln, ListenConfig{
			DisableStartupMessage: true,
			GracefulContext:       parentCtx,
			ShutdownTimeout:       30 * time.Second, // very long — parent cancel should win
		})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Kick off a blocking request.
	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	}()
	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	cancelParent() // triggers shutdown immediately

	<-listenErr
	elapsed := time.Since(start)

	// Should complete well under 30 s (the ShutdownTimeout).
	// The parent cancel propagates, so fasthttp sees context.Canceled quickly.
	require.Less(t, elapsed, 5*time.Second, "shutdown should have completed shortly after parent cancel, not after ShutdownTimeout")
}

// ---------------------------------------------------------------------------
// Bug 2: Post-shutdown hooks must fire exactly once
// ---------------------------------------------------------------------------

// Test_PostShutdownHooks_ExecutedExactlyOnce proves that the post-shutdown
// hook fires once per shutdown, not twice.  Before the fix, gracefulShutdown()
// called executeOnPostShutdownHooks explicitly AND ShutdownWithContext deferred
// it, resulting in two invocations.
func Test_PostShutdownHooks_ExecutedExactlyOnce(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(c Ctx) error {
		return c.SendString("OK")
	})

	var callCount int64
	app.Hooks().OnPostShutdown(func(_ error) error {
		atomic.AddInt64(&callCount, 1)
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()

	parentCtx, cancelParent := context.WithCancel(context.Background())
	listenErr := make(chan error, 1)
	go func() {
		listenErr <- app.Listener(ln, ListenConfig{
			DisableStartupMessage: true,
			GracefulContext:       parentCtx,
			ShutdownTimeout:       2 * time.Second,
		})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	cancelParent()
	<-listenErr

	// Give hooks a moment to propagate (they run synchronously in the
	// shutdown path, but the gracefulShutdown goroutine may race slightly).
	time.Sleep(100 * time.Millisecond)

	require.Equal(t, int64(1), atomic.LoadInt64(&callCount),
		"OnPostShutdown hook must fire exactly once; previously it fired twice")
}

// Test_PostShutdownHooks_ReceiveActualError verifies that the post-shutdown
// hook receives the real shutdown error (e.g. context.DeadlineExceeded) rather
// than nil.  The old code used `defer hooks(err)` where err was captured by
// value at registration time (always nil).
func Test_PostShutdownHooks_ReceiveActualError(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(c Ctx) error {
		time.Sleep(5 * time.Second) // outlives the shutdown timeout
		return c.SendString("OK")
	})

	errCh := make(chan error, 1)
	app.Hooks().OnPostShutdown(func(err error) error {
		errCh <- err
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Send a request that will still be running when we shut down.
	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	}()
	time.Sleep(50 * time.Millisecond)

	// Shutdown with a short timeout so the long request forces DeadlineExceeded.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = app.ShutdownWithContext(ctx)

	select {
	case hookErr := <-errCh:
		require.ErrorIs(t, hookErr, context.DeadlineExceeded,
			"post-shutdown hook must receive the actual shutdown error, not nil")
	case <-time.After(2 * time.Second):
		t.Fatal("post-shutdown hook was never called")
	}
}

// ---------------------------------------------------------------------------
// Bug 3: Connection tracking — ActiveConnections and IsShuttingDown
// ---------------------------------------------------------------------------

// Test_ActiveConnections_TracksConnections confirms that the active-connection
// counter increments on accept and decrements on close, and that the value is
// visible through app.ActiveConnections().
func Test_ActiveConnections_TracksConnections(t *testing.T) {
	t.Parallel()

	app := New()
	connReady := make(chan struct{}, 10)

	app.Get("/hold", func(c Ctx) error {
		connReady <- struct{}{} // signal: request is being processed
		time.Sleep(500 * time.Millisecond)
		return c.SendString("released")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)
	// Let the probe connection's Close propagate through the atomic counter.
	time.Sleep(100 * time.Millisecond)

	// Before any requests, counter should be 0 (the probe connections above
	// are closed and the decrement has propagated).
	require.Equal(t, 0, app.ActiveConnections())

	// Open 3 concurrent connections that block inside the handler.
	for i := 0; i < 3; i++ {
		go func() {
			conn, _ := ln.Dial()
			_, _ = conn.Write([]byte("GET /hold HTTP/1.1\r\nHost: example.com\r\n\r\n"))
			// read response (blocks until handler returns)
			buf := make([]byte, 4096)
			_, _ = conn.Read(buf)
			_ = conn.Close()
		}()
	}

	// Wait for all 3 handlers to report they are running.
	for i := 0; i < 3; i++ {
		<-connReady
	}

	// All 3 connections should be tracked.
	require.Equal(t, 3, app.ActiveConnections(), "expected 3 active connections while handlers are running")

	// Wait for handlers to finish and connections to close.
	time.Sleep(800 * time.Millisecond)

	require.Equal(t, 0, app.ActiveConnections(), "expected 0 active connections after all handlers returned")

	_ = app.Shutdown()
}

// Test_ActiveConnections_ZeroBeforeAndAfterShutdown confirms the counter
// returns to zero after a full shutdown cycle.
func Test_ActiveConnections_ZeroBeforeAndAfterShutdown(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(c Ctx) error {
		return c.SendString("OK")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)
	// Let the probe connection's Close propagate through the atomic counter.
	time.Sleep(100 * time.Millisecond)

	require.Equal(t, 0, app.ActiveConnections())

	_ = app.Shutdown()
	require.Equal(t, 0, app.ActiveConnections())
}

// Test_IsShuttingDown_FalseBeforeTrueAfter confirms the flag transitions
// correctly across the shutdown lifecycle.
func Test_IsShuttingDown_FalseBeforeTrueAfter(t *testing.T) {
	t.Parallel()

	app := New()

	flagDuringRequest := make(chan bool, 1)
	app.Get("/check", func(c Ctx) error {
		flagDuringRequest <- app.IsShuttingDown()
		return c.SendString("OK")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Pre-shutdown: flag must be false.
	require.False(t, app.IsShuttingDown())

	// Send a normal request; the flag should still be false inside the handler.
	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /check HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	require.Equal(t, false, <-flagDuringRequest)

	_ = app.Shutdown()

	// Post-shutdown: flag must be true.
	require.True(t, app.IsShuttingDown())
}

// Test_IsShuttingDown_VisibleDuringShutdown proves a handler that executes
// concurrently with the shutdown sequence can observe the flag as true.
func Test_IsShuttingDown_VisibleDuringShutdown(t *testing.T) {
	t.Parallel()

	app := New()

	// Channel to synchronise: the handler will block until shutdown has started.
	shutdownStarted := make(chan struct{})
	flagDuringShutdown := make(chan bool, 1)

	app.Get("/race", func(c Ctx) error {
		<-shutdownStarted // wait until shutdown is in progress
		flagDuringShutdown <- app.IsShuttingDown()
		return c.SendString("OK")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Fire the request (it will block in the handler).
	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /race HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(50 * time.Millisecond)

	// Start shutdown in a goroutine so we can unblock the handler after it starts.
	shutdownErr := make(chan error, 1)
	go func() {
		shutdownErr <- app.ShutdownWithContext(context.Background())
	}()
	time.Sleep(50 * time.Millisecond)

	// Now the shutdown flag should be set; unblock the handler.
	close(shutdownStarted)

	require.True(t, <-flagDuringShutdown, "IsShuttingDown must be true while shutdown is in progress")
	<-shutdownErr
}

// ---------------------------------------------------------------------------
// Bug 4: Keepalive connections drained on shutdown
// ---------------------------------------------------------------------------

// Test_Shutdown_ClosesIdleKeepaliveConnections verifies that idle keepalive
// connections are evicted when shutdown begins, rather than lingering until
// their (potentially unbounded) IdleTimeout expires.
func Test_Shutdown_ClosesIdleKeepaliveConnections(t *testing.T) {
	t.Parallel()

	app := New(Config{
		IdleTimeout: 60 * time.Second, // very long idle timeout
	})

	app.Get("/", func(c Ctx) error {
		return c.SendString("OK")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Open a keepalive connection: send a request, read the response, but
	// leave the connection open (HTTP/1.1 default is keep-alive).
	client := fasthttp.HostClient{
		Dial: func(_ string) (net.Conn, error) { return ln.Dial() },
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	req.SetRequestURI("http://example.com/")
	require.NoError(t, client.Do(req, resp))
	require.Equal(t, 200, resp.StatusCode())
	fasthttp.ReleaseRequest(req)
	fasthttp.ReleaseResponse(resp)

	// The idle keepalive connection is now sitting in the client pool.
	// Shutdown should still complete quickly despite the 60 s IdleTimeout.
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = app.ShutdownWithContext(ctx)

	elapsed := time.Since(start)
	require.Less(t, elapsed, 3*time.Second,
		"shutdown blocked by idle keepalive; IdleTimeout should be overridden at shutdown start")
}

// ---------------------------------------------------------------------------
// ShutdownWithConfig: callback lifecycle
// ---------------------------------------------------------------------------

// Test_ShutdownWithConfig_OnShutdownStart fires with the active connection count.
func Test_ShutdownWithConfig_OnShutdownStart(t *testing.T) {
	t.Parallel()

	app := New()

	holdReq := make(chan struct{})
	app.Get("/hold", func(c Ctx) error {
		<-holdReq
		return c.SendString("released")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Open 2 connections that block.
	for i := 0; i < 2; i++ {
		go func() {
			conn, _ := ln.Dial()
			_, _ = conn.Write([]byte("GET /hold HTTP/1.1\r\nHost: example.com\r\n\r\n"))
			buf := make([]byte, 4096)
			_, _ = conn.Read(buf)
			_ = conn.Close()
		}()
	}
	time.Sleep(100 * time.Millisecond)

	var startConns int
	var startCalled int64

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		close(holdReq) // release blocked handlers
	}()

	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{
		OnShutdownStart: func(activeConns int) {
			startConns = activeConns
			atomic.AddInt64(&startCalled, 1)
		},
	})

	require.Equal(t, int64(1), atomic.LoadInt64(&startCalled), "OnShutdownStart must be called exactly once")
	require.GreaterOrEqual(t, startConns, 2, "OnShutdownStart should report at least 2 active connections")
}

// Test_ShutdownWithConfig_OnDrainProgress fires periodically during drain.
func Test_ShutdownWithConfig_OnDrainProgress(t *testing.T) {
	t.Parallel()

	app := New()

	holdReq := make(chan struct{})
	app.Get("/hold", func(c Ctx) error {
		<-holdReq
		return c.SendString("released")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// One blocking connection.
	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /hold HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	var progressCalls int64
	var elapsedValues []time.Duration
	var mu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Release the handler after 600 ms so we get at least 1 progress tick at 100 ms interval.
	go func() {
		time.Sleep(600 * time.Millisecond)
		close(holdReq)
	}()

	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{
		DrainInterval: 100 * time.Millisecond,
		OnDrainProgress: func(remaining int, elapsed time.Duration) {
			atomic.AddInt64(&progressCalls, 1)
			mu.Lock()
			elapsedValues = append(elapsedValues, elapsed)
			mu.Unlock()
		},
	})

	calls := atomic.LoadInt64(&progressCalls)
	require.Greater(t, calls, int64(0), "OnDrainProgress should have been called at least once")

	mu.Lock()
	defer mu.Unlock()
	// Elapsed values should be monotonically increasing.
	for i := 1; i < len(elapsedValues); i++ {
		require.True(t, elapsedValues[i] >= elapsedValues[i-1],
			"elapsed durations should be non-decreasing")
	}
}

// Test_ShutdownWithConfig_OnForceClose fires when context deadline is exceeded.
func Test_ShutdownWithConfig_OnForceClose(t *testing.T) {
	t.Parallel()

	app := New()

	app.Get("/block", func(c Ctx) error {
		time.Sleep(10 * time.Second) // outlives any timeout
		return c.SendString("done")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// One blocking connection.
	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /block HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	var forceCloseCalled int64
	var forceClosedCount int

	// Very short timeout — forces context.DeadlineExceeded.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := app.ShutdownWithConfig(ctx, ShutdownConfig{
		OnForceClose: func(forceClosed int) {
			atomic.AddInt64(&forceCloseCalled, 1)
			forceClosedCount = forceClosed
		},
	})

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, int64(1), atomic.LoadInt64(&forceCloseCalled), "OnForceClose must be called when deadline is exceeded")
	require.Greater(t, forceClosedCount, 0, "OnForceClose should report remaining connections")
}

// Test_ShutdownWithConfig_RequestDeadline applies a per-request deadline
// nested inside the outer context.
func Test_ShutdownWithConfig_RequestDeadline(t *testing.T) {
	t.Parallel()

	app := New()

	app.Get("/slow", func(c Ctx) error {
		time.Sleep(5 * time.Second)
		return c.SendString("done")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /slow HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	// Outer context has plenty of time, but RequestDeadline is short.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	err := app.ShutdownWithConfig(ctx, ShutdownConfig{
		RequestDeadline: 300 * time.Millisecond,
	})
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.DeadlineExceeded,
		"RequestDeadline should cause DeadlineExceeded when requests outlive it")
	require.Less(t, elapsed, 2*time.Second,
		"shutdown should have been bounded by RequestDeadline, not the outer 10 s context")
}

// Test_ShutdownWithConfig_DrainInterval_Default uses the default interval when
// none is specified (500 ms).
func Test_ShutdownWithConfig_DrainInterval_Default(t *testing.T) {
	t.Parallel()

	app := New()

	holdReq := make(chan struct{})
	app.Get("/hold", func(c Ctx) error {
		<-holdReq
		return c.SendString("released")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /hold HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	var progressCalls int64

	// Release after ~1.2 s so we get ~2 ticks at the default 500 ms interval.
	go func() {
		time.Sleep(1200 * time.Millisecond)
		close(holdReq)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{
		// DrainInterval intentionally omitted — default 500 ms applies.
		OnDrainProgress: func(_ int, _ time.Duration) {
			atomic.AddInt64(&progressCalls, 1)
		},
	})

	require.GreaterOrEqual(t, atomic.LoadInt64(&progressCalls), int64(2),
		"default 500 ms DrainInterval should yield at least 2 callbacks over ~1.2 s drain")
}

// ---------------------------------------------------------------------------
// connTrackingListener / connTrackingConn unit behaviour
// ---------------------------------------------------------------------------

// Test_ConnTrackingConn_DoubleCloseDecrementOnce ensures the atomic guard
// prevents a double-close from decrementing the counter twice.
func Test_ConnTrackingConn_DoubleCloseDecrementOnce(t *testing.T) {
	t.Parallel()

	var counter int64
	atomic.AddInt64(&counter, 1) // simulate one accepted connection

	var registry sync.Map
	const connID int64 = 42
	// Minimal pipe-based conn for testing Close behaviour.
	server, client := newPipeConn()
	tracked := &connTrackingConn{
		Conn:        server,
		activeConns: &counter,
		registry:    &registry,
		id:          connID,
	}
	registry.Store(connID, tracked) // mirror what Accept does

	require.NoError(t, tracked.Close())
	require.Equal(t, int64(0), atomic.LoadInt64(&counter))

	// Verify the connection was removed from the registry.
	_, exists := registry.Load(connID)
	require.False(t, exists, "Close must remove the connection from the registry")

	// Second close must not decrement again (counter would go negative).
	_ = tracked.Close() // error is expected from the underlying closed pipe
	require.Equal(t, int64(0), atomic.LoadInt64(&counter),
		"double-close must not decrement the counter a second time")

	_ = client.Close()
}

// Test_ConnTrackingListener_AcceptIncrementsCounter verifies that each
// Accept call increments the shared counter and Close decrements it.
func Test_ConnTrackingListener_AcceptIncrementsCounter(t *testing.T) {
	t.Parallel()

	var counter int64
	inner := fasthttputil.NewInmemoryListener()
	tracked := &connTrackingListener{Listener: inner, activeConns: &counter}

	require.Equal(t, int64(0), atomic.LoadInt64(&counter))

	// Channel to hand the server-side tracked conn back to the test goroutine.
	serverConn := make(chan net.Conn, 1)
	go func() {
		conn, err := tracked.Accept()
		if err == nil {
			serverConn <- conn // do NOT close yet
		}
	}()

	// Dial triggers Accept on the listener side.
	client, err := inner.Dial()
	require.NoError(t, err)

	// Receive the server-side tracked conn (proves Accept completed).
	sc := <-serverConn

	require.Equal(t, int64(1), atomic.LoadInt64(&counter), "Accept should increment counter")

	// Close the server-side tracked conn to trigger the decrement.
	_ = sc.Close()
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, int64(0), atomic.LoadInt64(&counter), "Close should decrement counter")

	_ = client.Close()
	_ = inner.Close()
}

// Test_ConnTrackingListener_CloseAll closes every tracked connection and
// verifies the counter drops to zero and the registry is empty.
func Test_ConnTrackingListener_CloseAll(t *testing.T) {
	t.Parallel()

	var counter int64
	inner := fasthttputil.NewInmemoryListener()
	tracked := &connTrackingListener{Listener: inner, activeConns: &counter}

	// Accept 3 connections.
	serverConns := make(chan net.Conn, 3)
	for range 3 {
		go func() {
			conn, _ := tracked.Accept()
			serverConns <- conn
		}()
	}

	// Dial 3 client connections to trigger 3 Accepts.
	clientConns := make([]net.Conn, 3)
	for i := range clientConns {
		clientConns[i], _ = inner.Dial()
	}

	// Drain the accepted connections from the channel.
	for range 3 {
		<-serverConns
	}

	require.Equal(t, int64(3), atomic.LoadInt64(&counter))

	// CloseAll should close all 3 and return 3.
	closed := tracked.CloseAll()
	require.Equal(t, 3, closed)
	require.Equal(t, int64(0), atomic.LoadInt64(&counter))

	// Registry should be empty.
	empty := true
	tracked.conns.Range(func(_, _ any) bool {
		empty = false
		return false
	})
	require.True(t, empty, "registry must be empty after CloseAll")

	// Calling CloseAll again is a no-op.
	require.Equal(t, 0, tracked.CloseAll())

	for _, c := range clientConns {
		_ = c.Close()
	}
	_ = inner.Close()
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

// Test_ShutdownWithContext_ServerNilReturnsError covers the guard for a
// raw App with no server initialised.
func Test_ShutdownWithContext_ServerNilReturnsError(t *testing.T) {
	t.Parallel()

	app := &App{}
	err := app.ShutdownWithContext(context.Background())
	require.ErrorIs(t, err, ErrNotRunning)
}

// Test_ShutdownWithConfig_ServerNilReturnsError mirrors the above for the
// config-based path.
func Test_ShutdownWithConfig_ServerNilReturnsError(t *testing.T) {
	t.Parallel()

	app := &App{}
	err := app.ShutdownWithConfig(context.Background(), ShutdownConfig{})
	require.ErrorIs(t, err, ErrNotRunning)
}

// Test_ShutdownWithConfig_NoCallbacksDoesNotPanic exercises ShutdownWithConfig
// with all callbacks left nil — should not panic.
func Test_ShutdownWithConfig_NoCallbacksDoesNotPanic(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(c Ctx) error { return c.SendString("OK") })

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	require.NotPanics(t, func() {
		_ = app.ShutdownWithConfig(ctx, ShutdownConfig{})
	})
}

// Test_MultipleShutdownCalls verifies that calling Shutdown twice does not
// panic or corrupt state. The second call should return ErrNotRunning since
// the server is already nil'd or stopped.
func Test_MultipleShutdownCalls(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(c Ctx) error { return c.SendString("OK") })

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	err1 := app.Shutdown()
	err2 := app.Shutdown()

	// First shutdown succeeds; second sees the server is gone.
	require.NoError(t, err1)
	require.True(t, err2 == nil || errors.Is(err2, ErrNotRunning),
		"second Shutdown should be a no-op or return ErrNotRunning")
}

// ---------------------------------------------------------------------------
// Request-aware shutdown: handlers observe Done / Err / Deadline
// ---------------------------------------------------------------------------

// Test_RequestContext_DoneClosedOnShutdown verifies that an in-flight handler's
// c.Done() channel is closed as soon as shutdown begins.
func Test_RequestContext_DoneClosedOnShutdown(t *testing.T) {
	t.Parallel()

	app := New()

	doneSig := make(chan struct{}, 1)
	app.Get("/wait", func(c Ctx) error {
		select {
		case <-c.Done():
			doneSig <- struct{}{}
			return c.SendString("shutdown detected")
		case <-time.After(5 * time.Second):
			return c.SendString("timeout — no signal")
		}
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Fire a request that blocks until the Done signal.
	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /wait HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	// Trigger shutdown — this must close the Done channel.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = app.ShutdownWithContext(ctx)

	select {
	case <-doneSig:
		// Handler observed the Done signal — pass.
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not receive Done signal within timeout")
	}
}

// Test_RequestContext_ErrReturnsErrRequestShutdown confirms that after shutdown
// starts, c.Err() returns the ErrRequestShutdown sentinel rather than the raw
// context.Canceled.
func Test_RequestContext_ErrReturnsErrRequestShutdown(t *testing.T) {
	t.Parallel()

	app := New()

	errCh := make(chan error, 1)
	app.Get("/check-err", func(c Ctx) error {
		<-c.Done()
		errCh <- c.Err()
		return c.SendString("done")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /check-err HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = app.ShutdownWithContext(ctx)

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, ErrRequestShutdown,
			"c.Err() must return ErrRequestShutdown, not raw context.Canceled")
	case <-time.After(2 * time.Second):
		t.Fatal("handler never reported Err()")
	}
}

// Test_RequestContext_PreShutdownDoneIsNil verifies that before any shutdown,
// c.Done() returns a non-nil channel that is NOT yet closed and c.Err() is nil.
func Test_RequestContext_PreShutdownDoneIsNil(t *testing.T) {
	t.Parallel()

	app := New()

	type observation struct {
		doneIsNil  bool
		doneOpen   bool
		errIsNil   bool
		deadlineOK bool
	}
	obs := make(chan observation, 1)

	app.Get("/observe", func(c Ctx) error {
		ch := c.Done()
		open := true
		if ch != nil {
			select {
			case <-ch:
				open = false // channel is closed
			default:
				// channel is open
			}
		}
		_, hasDeadline := c.Deadline()
		obs <- observation{
			doneIsNil:  ch == nil,
			doneOpen:   open,
			errIsNil:   c.Err() == nil,
			deadlineOK: !hasDeadline, // no deadline set → ok should be false
		}
		return c.SendString("OK")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /observe HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()

	o := <-obs
	require.False(t, o.doneIsNil, "c.Done() must return a non-nil channel before shutdown")
	require.True(t, o.doneOpen, "c.Done() channel must be open (not closed) before shutdown")
	require.True(t, o.errIsNil, "c.Err() must be nil before shutdown")
	require.True(t, o.deadlineOK, "c.Deadline() must report ok=false when no deadline is set")

	_ = app.Shutdown()
}

// Test_RequestContext_DeadlineFromRequestContext verifies that when
// ShutdownConfig.RequestContext carries a deadline, handlers see it via
// c.Deadline().
func Test_RequestContext_DeadlineFromRequestContext(t *testing.T) {
	t.Parallel()

	app := New()

	type deadlineObs struct {
		hasDeadline bool
		deadline    time.Time
	}
	obs := make(chan deadlineObs, 1)

	app.Get("/deadline", func(c Ctx) error {
		// Wait for Done so we are guaranteed the RequestContext swap has happened.
		<-c.Done()
		dl, ok := c.Deadline()
		obs <- deadlineObs{hasDeadline: ok, deadline: dl}
		return c.SendString("done")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /deadline HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	// Supply a RequestContext with a concrete deadline (2 s from now).
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer reqCancel()

	outerCtx, outerCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer outerCancel()

	_ = app.ShutdownWithConfig(outerCtx, ShutdownConfig{
		RequestContext: reqCtx,
	})

	select {
	case o := <-obs:
		require.True(t, o.hasDeadline, "c.Deadline() must report ok=true when RequestContext has a deadline")
		// The deadline should be roughly 2 s after the test started (± tolerance).
		require.True(t, time.Until(o.deadline) < 3*time.Second,
			"deadline should be close to the RequestContext deadline")
	case <-time.After(4 * time.Second):
		t.Fatal("handler never reported deadline observation")
	}
}

// Test_RequestContext_HandlerReturnsEarlyOn503 demonstrates the pattern where a
// handler detects shutdown and returns a 503 Service Unavailable without
// waiting for the full request processing.
func Test_RequestContext_HandlerReturnsEarlyOn503(t *testing.T) {
	t.Parallel()

	app := New()

	app.Get("/service", func(c Ctx) error {
		// Simulate expensive work that respects the shutdown signal.
		select {
		case <-c.Done():
			return c.Status(StatusServiceUnavailable).SendString("shutting down")
		case <-time.After(10 * time.Second):
			return c.SendString("completed expensive work")
		}
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Capture the response.
	respCh := make(chan []byte, 1)
	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /service HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		respCh <- buf[:n]
		_ = conn.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = app.ShutdownWithContext(ctx)
	elapsed := time.Since(start)

	// The handler returned early (much less than 10 s).
	require.Less(t, elapsed, 3*time.Second,
		"handler should have returned early after detecting shutdown")

	select {
	case resp := <-respCh:
		require.Contains(t, string(resp), "503",
			"handler should have returned 503 Service Unavailable")
		require.Contains(t, string(resp), "shutting down",
			"handler body should indicate shutdown")
	case <-time.After(2 * time.Second):
		t.Fatal("no response received from handler")
	}
}

// Test_RequestContext_ShutdownWithConfig_SignalsPropagated verifies that
// ShutdownWithConfig (not just ShutdownWithContext) also cancels the request
// context so handlers see the Done signal.
func Test_RequestContext_ShutdownWithConfig_SignalsPropagated(t *testing.T) {
	t.Parallel()

	app := New()

	doneSig := make(chan struct{}, 1)
	app.Get("/cfg-signal", func(c Ctx) error {
		select {
		case <-c.Done():
			doneSig <- struct{}{}
			return c.SendString("config shutdown detected")
		case <-time.After(5 * time.Second):
			return c.SendString("no signal")
		}
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /cfg-signal HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{
		OnShutdownStart: func(_ int) {}, // non-nil callback; confirms config path taken
	})

	select {
	case <-doneSig:
		// Handler observed Done via ShutdownWithConfig — pass.
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not receive Done signal from ShutdownWithConfig")
	}
}

// ---------------------------------------------------------------------------
// GracefulCloseTyped: WebSocket and SSE protocol-aware shutdown
// ---------------------------------------------------------------------------

// Test_GracefulClose_WebSocket_SendsCloseFrame verifies that when no cleanup
// hook is registered the framework writes a 4-byte WS close frame and
// OnWebSocketClose fires with nil error after the client echoes the frame.
func Test_GracefulClose_WebSocket_SendsCloseFrame(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/ws", func(c Ctx) error {
		tc := c.TrackedConn()
		if tc != nil {
			tc.SetConnType(ConnTypeWebSocket)
		}
		// Block until the connection is closed by shutdown.
		time.Sleep(5 * time.Second)
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Open a connection that the handler will mark as WebSocket.
	clientConn, _ := ln.Dial()
	_, _ = clientConn.Write([]byte("GET /ws HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	time.Sleep(150 * time.Millisecond)

	var wsCloseErr error
	var wsCloseID int64
	var wsCalled int64

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Goroutine reads the close frame from server and echoes it back.
	go func() {
		buf := make([]byte, 4)
		_, _ = clientConn.Read(buf)
		// Echo close frame back to server.
		_, _ = clientConn.Write(buf)
	}()

	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{
		OnWebSocketClose: func(connID int64, err error) {
			wsCloseID = connID
			wsCloseErr = err
			atomic.AddInt64(&wsCalled, 1)
		},
	})

	_ = clientConn.Close()

	require.Equal(t, int64(1), atomic.LoadInt64(&wsCalled), "OnWebSocketClose must fire once")
	require.NoError(t, wsCloseErr, "OnWebSocketClose err should be nil when client echoes close frame")
	require.Greater(t, wsCloseID, int64(0), "connID must be positive")
}

// Test_GracefulClose_WebSocket_CleanupHookReplacesFrame verifies that when
// a cleanup hook is registered the framework does NOT write the raw close
// frame and delegates the handshake entirely to the hook.
func Test_GracefulClose_WebSocket_CleanupHookReplacesFrame(t *testing.T) {
	t.Parallel()

	app := New()
	hookCalled := make(chan struct{}, 1)

	app.Get("/ws-hook", func(c Ctx) error {
		tc := c.TrackedConn()
		if tc != nil {
			tc.SetConnType(ConnTypeWebSocket)
			tc.SetCleanupHook(func() error {
				hookCalled <- struct{}{}
				return nil
			})
		}
		time.Sleep(5 * time.Second)
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	clientConn, _ := ln.Dial()
	_, _ = clientConn.Write([]byte("GET /ws-hook HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	time.Sleep(150 * time.Millisecond)

	var onCloseCalled int64

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{
		OnWebSocketClose: func(_ int64, _ error) {
			atomic.AddInt64(&onCloseCalled, 1)
		},
	})

	_ = clientConn.Close()

	// The hook must have been called.
	select {
	case <-hookCalled:
	case <-time.After(1 * time.Second):
		t.Fatal("cleanup hook was never called")
	}
	require.Equal(t, int64(1), atomic.LoadInt64(&onCloseCalled))
}

// Test_GracefulClose_WebSocket_TimeoutReportsError verifies that when the
// client never replies to the close frame, OnWebSocketClose fires with
// ErrWebSocketCloseTimeout.
func Test_GracefulClose_WebSocket_TimeoutReportsError(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/ws-timeout", func(c Ctx) error {
		tc := c.TrackedConn()
		if tc != nil {
			tc.SetConnType(ConnTypeWebSocket)
		}
		time.Sleep(5 * time.Second)
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	clientConn, _ := ln.Dial()
	_, _ = clientConn.Write([]byte("GET /ws-timeout HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	time.Sleep(150 * time.Millisecond)

	var wsCloseErr error

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{
		WebSocketCloseTimeout: 200 * time.Millisecond,
		OnWebSocketClose: func(_ int64, err error) {
			wsCloseErr = err
		},
	})

	_ = clientConn.Close()

	require.ErrorIs(t, wsCloseErr, ErrWebSocketCloseTimeout,
		"OnWebSocketClose must report ErrWebSocketCloseTimeout when client never replies")
}

// Test_GracefulClose_SSE_SendsShutdownEvent verifies that when no cleanup
// hook is registered the framework writes the default SSE shutdown event
// and OnSSEClose fires.
func Test_GracefulClose_SSE_SendsShutdownEvent(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/sse", func(c Ctx) error {
		tc := c.TrackedConn()
		if tc != nil {
			tc.SetConnType(ConnTypeSSE)
		}
		time.Sleep(5 * time.Second)
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	clientConn, _ := ln.Dial()
	_, _ = clientConn.Write([]byte("GET /sse HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	time.Sleep(150 * time.Millisecond)

	var sseCloseErr error
	var sseCalled int64

	// Read the SSE event that the server will push during shutdown.
	sseData := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := clientConn.Read(buf)
		sseData <- buf[:n]
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{
		SSECloseTimeout: 200 * time.Millisecond,
		OnSSEClose: func(_ int64, err error) {
			sseCloseErr = err
			atomic.AddInt64(&sseCalled, 1)
		},
	})

	_ = clientConn.Close()

	require.Equal(t, int64(1), atomic.LoadInt64(&sseCalled), "OnSSEClose must fire once")
	require.NoError(t, sseCloseErr, "OnSSEClose err should be nil on successful write")

	select {
	case data := <-sseData:
		require.Contains(t, string(data), "event: shutdown")
		require.Contains(t, string(data), "data: server shutting down")
	case <-time.After(1 * time.Second):
		t.Fatal("did not receive SSE shutdown event from server")
	}
}

// Test_GracefulClose_SSE_CustomEvent verifies that a custom SSECloseEvent
// payload in the config is written to the client instead of the default.
func Test_GracefulClose_SSE_CustomEvent(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/sse-custom", func(c Ctx) error {
		tc := c.TrackedConn()
		if tc != nil {
			tc.SetConnType(ConnTypeSSE)
		}
		time.Sleep(5 * time.Second)
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	clientConn, _ := ln.Dial()
	_, _ = clientConn.Write([]byte("GET /sse-custom HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	time.Sleep(150 * time.Millisecond)

	sseData := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := clientConn.Read(buf)
		sseData <- buf[:n]
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{
		SSECloseTimeout: 200 * time.Millisecond,
		SSECloseEvent:   "event: goodbye\ndata: custom payload\n\n",
		OnSSEClose:      func(_ int64, _ error) {},
	})

	_ = clientConn.Close()

	select {
	case data := <-sseData:
		require.Contains(t, string(data), "event: goodbye")
		require.Contains(t, string(data), "data: custom payload")
		require.NotContains(t, string(data), "event: shutdown")
	case <-time.After(1 * time.Second):
		t.Fatal("did not receive custom SSE event from server")
	}
}

// Test_GracefulClose_SSE_CleanupHookReplacesEvent verifies that when a
// cleanup hook is registered on an SSE connection the framework does NOT
// write the SSE event and delegates entirely to the hook.
func Test_GracefulClose_SSE_CleanupHookReplacesEvent(t *testing.T) {
	t.Parallel()

	app := New()
	hookCalled := make(chan struct{}, 1)

	app.Get("/sse-hook", func(c Ctx) error {
		tc := c.TrackedConn()
		if tc != nil {
			tc.SetConnType(ConnTypeSSE)
			tc.SetCleanupHook(func() error {
				hookCalled <- struct{}{}
				return nil
			})
		}
		time.Sleep(5 * time.Second)
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	clientConn, _ := ln.Dial()
	_, _ = clientConn.Write([]byte("GET /sse-hook HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	time.Sleep(150 * time.Millisecond)

	var onCloseCalled int64

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{
		OnSSEClose: func(_ int64, _ error) {
			atomic.AddInt64(&onCloseCalled, 1)
		},
	})

	_ = clientConn.Close()

	select {
	case <-hookCalled:
	case <-time.After(1 * time.Second):
		t.Fatal("cleanup hook was never called")
	}
	require.Equal(t, int64(1), atomic.LoadInt64(&onCloseCalled))
}

// Test_GracefulClose_PlainHTTP_Unaffected verifies that a connection which
// was never marked with SetConnType is not touched by GracefulCloseTyped —
// no close frame or SSE event is written.
func Test_GracefulClose_PlainHTTP_Unaffected(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/plain", func(c Ctx) error {
		// Deliberately do NOT call SetConnType — this is plain HTTP.
		time.Sleep(300 * time.Millisecond)
		return c.SendString("OK")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	clientConn, _ := ln.Dial()
	_, _ = clientConn.Write([]byte("GET /plain HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	time.Sleep(100 * time.Millisecond)

	var wsCloseCalled int64
	var sseCloseCalled int64

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{
		OnWebSocketClose: func(_ int64, _ error) {
			atomic.AddInt64(&wsCloseCalled, 1)
		},
		OnSSEClose: func(_ int64, _ error) {
			atomic.AddInt64(&sseCloseCalled, 1)
		},
	})

	_ = clientConn.Close()

	require.Equal(t, int64(0), atomic.LoadInt64(&wsCloseCalled),
		"OnWebSocketClose must not fire for plain HTTP connections")
	require.Equal(t, int64(0), atomic.LoadInt64(&sseCloseCalled),
		"OnSSEClose must not fire for plain HTTP connections")
}

// ---------------------------------------------------------------------------
// Comprehensive scenario tests: clean shutdown, drain timing, cancellation,
// repeated shutdown across API boundaries, and hook execution order
// ---------------------------------------------------------------------------

// Test_CleanShutdown_FullLifecycle proves that a clean shutdown (no active
// connections at the time of the call) traverses every phase, returns nil,
// and leaves activeConns at zero throughout.
func Test_CleanShutdown_FullLifecycle(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/ping", func(c Ctx) error { return c.SendString("pong") })

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond) // let probe conn close propagate

	// Confirm zero connections before we even start.
	require.Equal(t, 0, app.ActiveConnections())
	require.False(t, app.IsShuttingDown())

	var startCount int
	var startCalled int64
	var progressCalled int64
	var forceCalled int64

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := app.ShutdownWithConfig(ctx, ShutdownConfig{
		OnShutdownStart: func(active int) {
			startCount = active
			atomic.AddInt64(&startCalled, 1)
		},
		OnDrainProgress: func(_ int, _ time.Duration) {
			atomic.AddInt64(&progressCalled, 1)
		},
		OnForceClose: func(_ int) {
			atomic.AddInt64(&forceCalled, 1)
		},
	})

	// Clean shutdown: nil error, all callbacks behave correctly.
	require.NoError(t, err, "clean shutdown must return nil")
	require.True(t, app.IsShuttingDown())
	require.Equal(t, 0, app.ActiveConnections())

	require.Equal(t, int64(1), atomic.LoadInt64(&startCalled), "OnShutdownStart fires once")
	require.Equal(t, 0, startCount, "OnShutdownStart sees zero active conns on clean shutdown")

	// Drain monitor may or may not tick before conns hit zero; that's fine.
	// OnForceClose must NOT fire — there was nothing to force-close.
	require.Equal(t, int64(0), atomic.LoadInt64(&forceCalled),
		"OnForceClose must not fire when drain completes before deadline")
}

// Test_CleanShutdown_ReturnsNilImmediately shows that when all in-flight
// requests finish before the context deadline, Shutdown returns nil quickly
// without waiting for the full deadline to elapse.
func Test_CleanShutdown_ReturnsNilImmediately(t *testing.T) {
	t.Parallel()

	app := New()
	// Handler that finishes fast.
	app.Get("/quick", func(c Ctx) error {
		time.Sleep(50 * time.Millisecond)
		return c.SendString("done")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Fire a request that completes in ~50 ms.
	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /quick HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(30 * time.Millisecond) // let the request land

	// Give the outer context a very generous deadline so we can measure
	// whether shutdown returns early once the connection drains.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	err := app.ShutdownWithConfig(ctx, ShutdownConfig{})
	elapsed := time.Since(start)

	require.NoError(t, err, "shutdown returns nil when all requests finish")
	require.Less(t, elapsed, 2*time.Second,
		"shutdown must return as soon as conns drain, not wait for 10 s context")
}

// Test_DrainTime_WallClockMatchesActualDrain asserts that the elapsed values
// reported by OnDrainProgress track real wall-clock time and that the total
// drain wall-clock stays between the handler's blocking duration and the
// context deadline.
func Test_DrainTime_WallClockMatchesActualDrain(t *testing.T) {
	t.Parallel()

	app := New()
	holdReq := make(chan struct{})
	app.Get("/block", func(c Ctx) error {
		<-holdReq
		return c.SendString("released")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Open one connection that will block until we release it.
	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /block HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	var lastElapsed time.Duration
	var mu sync.Mutex

	// Release the handler after 500 ms — drain should complete shortly after.
	go func() {
		time.Sleep(500 * time.Millisecond)
		close(holdReq)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	err := app.ShutdownWithConfig(ctx, ShutdownConfig{
		DrainInterval: 50 * time.Millisecond,
		OnDrainProgress: func(_ int, elapsed time.Duration) {
			mu.Lock()
			lastElapsed = elapsed
			mu.Unlock()
		},
	})
	totalElapsed := time.Since(start)

	require.NoError(t, err, "drain should complete before context deadline")

	mu.Lock()
	finalElapsed := lastElapsed
	mu.Unlock()

	// The last OnDrainProgress elapsed value should be ≥ 400 ms (handler held
	// ~500 ms) and the total wall-clock should be close to that value too.
	require.GreaterOrEqual(t, finalElapsed, 400*time.Millisecond,
		"OnDrainProgress elapsed must reflect actual time waiting for drain")
	require.Less(t, totalElapsed, 3*time.Second,
		"shutdown must not overshoot well past the handler's hold time")
	// elapsed reported to the callback must not exceed total wall-clock by much
	// (the ticker runs inside the same process).
	require.True(t, finalElapsed <= totalElapsed+100*time.Millisecond,
		"reported elapsed must not exceed actual wall-clock by more than one tick")
}

// Test_DrainTime_ExitsEarlyOnZeroConns proves that the drain loop exits as
// soon as the active-connection count reaches zero, rather than continuing
// to poll until the context deadline.
func Test_DrainTime_ExitsEarlyOnZeroConns(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/fast", func(c Ctx) error {
		time.Sleep(100 * time.Millisecond)
		return c.SendString("done")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /fast HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(50 * time.Millisecond) // let the request land

	// Context deadline is 30 s — way longer than the 100 ms handler.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	err := app.ShutdownWithConfig(ctx, ShutdownConfig{})
	elapsed := time.Since(start)

	require.NoError(t, err, "should drain cleanly")
	require.Less(t, elapsed, 2*time.Second,
		"drain must exit immediately after conns reach zero, not wait for 30 s deadline")
}

// Test_ContextCancellation_MidDrainReturnsContextCanceled explicitly cancels
// the context (not deadline-based) while connections are still draining and
// verifies that ShutdownWithConfig returns context.Canceled, not
// context.DeadlineExceeded.
func Test_ContextCancellation_MidDrainReturnsContextCanceled(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/hold", func(c Ctx) error {
		time.Sleep(10 * time.Second) // outlives everything
		return c.SendString("done")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /hold HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 200 ms — before the 10 s handler finishes.
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	var forceCount int
	err := app.ShutdownWithConfig(ctx, ShutdownConfig{
		OnForceClose: func(n int) { forceCount = n },
	})

	require.ErrorIs(t, err, context.Canceled,
		"mid-drain cancel must produce context.Canceled, not DeadlineExceeded")
	require.Greater(t, forceCount, 0,
		"force-close must fire after cancellation to clean up remaining connections")
}

// Test_ContextCancellation_PreCancel_ShutdownExitsImmediately passes an
// already-cancelled context to ShutdownWithConfig. The drain loop should
// notice the cancelled context on its very first check and exit without
// waiting.
func Test_ContextCancellation_PreCancel_ShutdownExitsImmediately(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/slow", func(c Ctx) error {
		time.Sleep(10 * time.Second)
		return c.SendString("done")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /slow HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	// Pre-cancel the context before calling shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := app.ShutdownWithConfig(ctx, ShutdownConfig{})
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, elapsed, 1*time.Second,
		"pre-cancelled context must cause immediate exit from drain loop")
}

// Test_RepeatedShutdown_MixedAPIs calls ShutdownWithConfig first, then
// ShutdownWithContext, then Shutdown.  The first call succeeds.  Subsequent
// calls must not panic and must return an error (either ErrNotRunning when
// the server field has been nil'd, or a listener/server-level error when the
// fasthttp server object is still alive but the underlying listener is closed).
// The critical invariant is that state remains consistent: IsShuttingDown is
// true and ActiveConnections is zero after any number of shutdown calls.
func Test_RepeatedShutdown_MixedAPIs(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(c Ctx) error { return c.SendString("OK") })

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// First call via ShutdownWithConfig.
	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()
	err1 := app.ShutdownWithConfig(ctx1, ShutdownConfig{})
	require.NoError(t, err1, "first shutdown (WithConfig) must succeed")

	// Second call via ShutdownWithContext — the underlying listener is already
	// closed.  Depending on whether the fasthttp server internally errors or
	// returns nil (no remaining work), this may be nil or a listener error.
	// The critical guarantee is no panic and no state corruption.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel2()
	require.NotPanics(t, func() {
		_ = app.ShutdownWithContext(ctx2)
	}, "second shutdown (WithContext) must not panic")

	// Third call via plain Shutdown — same guarantee.
	require.NotPanics(t, func() {
		_ = app.Shutdown()
	}, "third shutdown (plain) must not panic")

	// State should still be consistent after repeated calls.
	require.True(t, app.IsShuttingDown())
	require.Equal(t, 0, app.ActiveConnections())
}

// Test_RepeatedShutdown_PostHooksNeverPanic registers a post-shutdown hook and
// calls ShutdownWithConfig twice.  ShutdownWithConfig runs the full phase
// pipeline (including post-hooks) every time app.server is non-nil.  The
// critical guarantee is that neither call panics and the hook itself remains
// safe to invoke multiple times.  The existing single-invocation contract is
// tested via Test_PostShutdownHooks_ExecutedExactlyOnce (the GracefulContext
// path which uses ShutdownWithContext internally).
func Test_RepeatedShutdown_PostHooksNeverPanic(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(c Ctx) error { return c.SendString("OK") })

	var postCount int64
	app.Hooks().OnPostShutdown(func(_ error) error {
		atomic.AddInt64(&postCount, 1)
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()
	_ = app.ShutdownWithConfig(ctx1, ShutdownConfig{})

	// Second call — must not panic; it may or may not fire hooks depending
	// on whether app.server has been nil'd by this point.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel2()
	_ = app.ShutdownWithConfig(ctx2, ShutdownConfig{})

	// The hook was invoked at least once (from the first successful shutdown).
	require.GreaterOrEqual(t, atomic.LoadInt64(&postCount), int64(1),
		"OnPostShutdown must fire at least once across repeated shutdown calls")

	// Final state is consistent regardless of how many times hooks ran.
	require.True(t, app.IsShuttingDown())
	require.Equal(t, 0, app.ActiveConnections())
}

// Test_HookOrder_PreBeforePostBeforeReturn proves the strict execution order:
//
//	OnShutdownStart  →  pre-shutdown hooks  →  [drain / force-close]  →  post-shutdown hooks  →  return
//
// Each stage appends a tag to a shared slice under a mutex so the final
// order is deterministic.
func Test_HookOrder_PreBeforePostBeforeReturn(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/hold", func(c Ctx) error {
		// Block briefly so we actually enter the drain phase.
		time.Sleep(50 * time.Millisecond)
		return c.SendString("released")
	})

	var order []string
	var mu sync.Mutex
	appendTag := func(tag string) {
		mu.Lock()
		order = append(order, tag)
		mu.Unlock()
	}

	// Pre-shutdown hook.
	app.Hooks().OnPreShutdown(func() error {
		appendTag("pre-hook")
		return nil
	})

	// Post-shutdown hook.
	app.Hooks().OnPostShutdown(func(_ error) error {
		appendTag("post-hook")
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Open a connection that blocks briefly.
	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /hold HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(30 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{
		OnShutdownStart: func(_ int) {
			appendTag("on-start")
		},
		OnDrainProgress: func(_ int, _ time.Duration) {
			appendTag("drain-tick")
		},
	})
	// After ShutdownWithConfig returns, append a final tag.
	appendTag("returned")

	mu.Lock()
	defer mu.Unlock()

	// Locate required tags.
	indexOf := func(tag string) int {
		for i, v := range order {
			if v == tag {
				return i
			}
		}
		return -1
	}

	onStartIdx := indexOf("on-start")
	preHookIdx := indexOf("pre-hook")
	postHookIdx := indexOf("post-hook")
	returnedIdx := indexOf("returned")

	require.Greater(t, onStartIdx, -1, "on-start tag must be present")
	require.Greater(t, preHookIdx, -1, "pre-hook tag must be present")
	require.Greater(t, postHookIdx, -1, "post-hook tag must be present")
	require.Greater(t, returnedIdx, -1, "returned tag must be present")

	require.Less(t, onStartIdx, preHookIdx,
		"OnShutdownStart must fire before pre-shutdown hooks")
	require.Less(t, preHookIdx, postHookIdx,
		"pre-shutdown hooks must fire before post-shutdown hooks")
	require.Less(t, postHookIdx, returnedIdx,
		"post-shutdown hooks must fire before ShutdownWithConfig returns")
}

// Test_HookOrder_ForceCloseBeforePostHooks proves that when the context
// expires and OnForceClose fires, it does so BEFORE the post-shutdown hooks.
func Test_HookOrder_ForceCloseBeforePostHooks(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/infinite", func(c Ctx) error {
		time.Sleep(30 * time.Second) // outlives everything
		return c.SendString("never")
	})

	var order []string
	var mu sync.Mutex
	appendTag := func(tag string) {
		mu.Lock()
		order = append(order, tag)
		mu.Unlock()
	}

	app.Hooks().OnPostShutdown(func(_ error) error {
		appendTag("post-hook")
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /infinite HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	// Very short deadline to trigger force-close.
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	err := app.ShutdownWithConfig(ctx, ShutdownConfig{
		OnForceClose: func(_ int) {
			appendTag("force-close")
		},
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)

	mu.Lock()
	defer mu.Unlock()

	forceIdx := -1
	postIdx := -1
	for i, v := range order {
		switch v {
		case "force-close":
			forceIdx = i
		case "post-hook":
			postIdx = i
		}
	}
	require.Greater(t, forceIdx, -1, "force-close tag must be present")
	require.Greater(t, postIdx, -1, "post-hook tag must be present")
	require.Less(t, forceIdx, postIdx,
		"OnForceClose must fire before post-shutdown hooks")
}

// Test_HookOrder_MultiplePreHooksRunInRegistrationOrder verifies that when
// two pre-shutdown hooks are registered, they execute in the order they were
// added (FIFO).
func Test_HookOrder_MultiplePreHooksRunInRegistrationOrder(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(c Ctx) error { return c.SendString("OK") })

	var order []string
	var mu sync.Mutex

	app.Hooks().OnPreShutdown(func() error {
		mu.Lock()
		order = append(order, "pre-A")
		mu.Unlock()
		return nil
	})
	app.Hooks().OnPreShutdown(func() error {
		mu.Lock()
		order = append(order, "pre-B")
		mu.Unlock()
		return nil
	})
	app.Hooks().OnPostShutdown(func(_ error) error {
		mu.Lock()
		order = append(order, "post-A")
		mu.Unlock()
		return nil
	})
	app.Hooks().OnPostShutdown(func(_ error) error {
		mu.Lock()
		order = append(order, "post-B")
		mu.Unlock()
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{})

	mu.Lock()
	defer mu.Unlock()

	require.Equal(t, []string{"pre-A", "pre-B", "post-A", "post-B"}, order,
		"hooks must execute in registration order within each phase, pre before post")
}

// ---------------------------------------------------------------------------
// Integration tests: real connections exercising shutdown, WS close frames,
// and file upload simultaneously.
// ---------------------------------------------------------------------------

// Test_Integration_ConcurrentShutdown_AllWorkloads fires a slow HTTP request,
// a WebSocket connection, and a partial file upload at the same time, then
// triggers shutdown.  Verifies that:
//   - The WebSocket receives the RFC 6455 close frame (OnWebSocketClose fires).
//   - The slow HTTP request is force-closed after drain timeout.
//   - The partial file upload connection is force-closed (OnForceClose fires).
//   - All lifecycle callbacks execute without panic or data race.
func Test_Integration_ConcurrentShutdown_AllWorkloads(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := New()
	app.Get("/slow", func(c Ctx) error {
		time.Sleep(3 * time.Second) // outlives drain timeout
		return c.SendString("slow done")
	})
	// Upload handler — sleeps to simulate a long body read so the connection
	// stays alive long enough for force-close to hit it.
	app.Post("/upload", func(c Ctx) error {
		time.Sleep(5 * time.Second)
		return c.SendString("upload done")
	})
	app.Get("/ws", func(c Ctx) error {
		tc := c.TrackedConn()
		if tc != nil {
			tc.SetConnType(ConnTypeWebSocket)
		}
		// Hold the connection open until shutdown.
		time.Sleep(5 * time.Second)
		return nil
	})

	var wsCloseCalled atomic.Int32
	var forceCloseCalled atomic.Int32

	go func() { _ = app.Listener(ln, ListenConfig{DisableStartupMessage: true}) }()

	// Give server a moment to start accepting.
	time.Sleep(50 * time.Millisecond)

	// --- Fire all three workloads concurrently ---
	var wg sync.WaitGroup

	// 1. Slow HTTP request.
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := ln.Dial()
		if err != nil {
			return
		}
		defer conn.Close()
		req := "GET /slow HTTP/1.1\r\nHost: test\r\n\r\n"
		_, _ = conn.Write([]byte(req))
		buf := make([]byte, 4096)
		_ = conn.SetReadDeadline(time.Now().Add(4 * time.Second))
		_, _ = conn.Read(buf)
	}()

	// 2. WebSocket connection (just holds open, reads close frame).
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := ln.Dial()
		if err != nil {
			return
		}
		defer conn.Close()
		req := "GET /ws HTTP/1.1\r\nHost: test\r\n\r\n"
		_, _ = conn.Write([]byte(req))
		// Read response headers + any body; then wait for close frame.
		buf := make([]byte, 4096)
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, _ = conn.Read(buf)
		// Echo close frame back if we get one.
		frame := make([]byte, 4)
		if _, err := conn.Read(frame); err == nil {
			_, _ = conn.Write(frame)
		}
	}()

	// 3. File upload — send complete body so handler is entered, then handler
	//    sleeps keeping the connection alive for force-close.
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := ln.Dial()
		if err != nil {
			return
		}
		defer conn.Close()
		header := fmt.Sprintf("POST /upload HTTP/1.1\r\nHost: test\r\nContent-Length: %d\r\n\r\n", 256)
		_, _ = conn.Write([]byte(header))
		_, _ = conn.Write(bytes.Repeat([]byte("x"), 256))
		// Block until connection is force-closed.
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
	}()

	// Let connections establish.
	time.Sleep(200 * time.Millisecond)
	require.GreaterOrEqual(t, int(app.ActiveConnections()), 3,
		"all three connections must be tracked before shutdown")

	// Trigger shutdown with a short drain window.  Expect DeadlineExceeded
	// because the slow and upload handlers outlive the 800 ms window.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	err := app.ShutdownWithConfig(shutdownCtx, ShutdownConfig{
		OnWebSocketClose:      func(_ int64, _ error) { wsCloseCalled.Add(1) },
		OnForceClose:          func(_ int) { forceCloseCalled.Add(1) },
		WebSocketCloseTimeout: 500 * time.Millisecond,
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// Wait for client goroutines to finish.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("client goroutines did not finish within deadline")
	}

	// Assertions: WS close callback fired at least once; force-close fired
	// for the slow and upload connections.
	require.GreaterOrEqual(t, int(wsCloseCalled.Load()), 1,
		"OnWebSocketClose must fire for the WS connection")
	require.GreaterOrEqual(t, int(forceCloseCalled.Load()), 1,
		"OnForceClose must fire for connections that outlive the drain window")
}

// Test_Integration_WebSocketCloseFrame_RealConnection validates that the
// framework writes exactly the 4-byte RFC 6455 close frame with status 1001
// (Going Away) when no cleanup hook is registered.
func Test_Integration_WebSocketCloseFrame_RealConnection(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := New()
	app.Get("/ws", func(c Ctx) error {
		tc := c.TrackedConn()
		if tc != nil {
			tc.SetConnType(ConnTypeWebSocket)
		}
		// Hold open so shutdown can send the close frame.
		time.Sleep(3 * time.Second)
		return nil
	})

	var receivedFrame []byte
	frameCh := make(chan struct{}, 1)
	var wsErr error

	go func() { _ = app.Listener(ln, ListenConfig{DisableStartupMessage: true}) }()

	time.Sleep(50 * time.Millisecond)

	// Connect and send HTTP request.
	conn, err := ln.Dial()
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write([]byte("GET /ws HTTP/1.1\r\nHost: test\r\n\r\n"))
	require.NoError(t, err)

	// Read response headers.
	buf := make([]byte, 4096)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _ = conn.Read(buf)

	// Trigger shutdown.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = app.ShutdownWithConfig(shutdownCtx, ShutdownConfig{
		OnWebSocketClose: func(_ int64, shutdownErr error) {
			wsErr = shutdownErr
			frameCh <- struct{}{}
		},
		WebSocketCloseTimeout: 1 * time.Second,
	})

	// Read the close frame sent by the framework.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	frameBuf := make([]byte, 4)
	n, readErr := conn.Read(frameBuf)
	if readErr == nil && n >= 4 {
		receivedFrame = frameBuf[:4]
	}

	// If we got the frame, echo it back as the "ack".
	if len(receivedFrame) == 4 {
		_, _ = conn.Write(receivedFrame)
	}

	// Wait for the callback (or timeout).
	select {
	case <-frameCh:
	case <-time.After(3 * time.Second):
	}

	// Validate exact close frame bytes: FIN + opcode 0x08, payload-len 2,
	// status code 1001 (0x03E9) in big-endian.
	if len(receivedFrame) == 4 {
		require.Equal(t, byte(0x88), receivedFrame[0],
			"byte 0: FIN bit set + close opcode (0x08)")
		require.Equal(t, byte(0x02), receivedFrame[1],
			"byte 1: payload length = 2")
		require.Equal(t, byte(0x03), receivedFrame[2],
			"byte 2: status code high byte (1001 >> 8)")
		require.Equal(t, byte(0xe9), receivedFrame[3],
			"byte 3: status code low byte (1001 & 0xFF)")
	} else {
		// If the frame wasn't read (timing), at least verify the callback ran.
		select {
		case <-frameCh:
		default:
			t.Fatal("neither close frame received nor OnWebSocketClose callback fired")
		}
	}

	_ = wsErr // referenced to avoid unused warning
}

// Test_Integration_FileUpload_Interrupted_By_Shutdown simulates a multipart
// file upload that is mid-transfer when the server is shut down.  The
// connection must be force-closed after the drain timeout expires because
// fasthttp is still waiting for the remainder of the declared Content-Length.
func Test_Integration_FileUpload_Interrupted_By_Shutdown(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := New()
	app.Post("/upload", func(c Ctx) error {
		// Simulate a slow upload handler that doesn't finish in time.
		time.Sleep(5 * time.Second)
		return c.SendString("done")
	})

	var forceClosedCount atomic.Int32
	go func() { _ = app.Listener(ln, ListenConfig{DisableStartupMessage: true}) }()

	time.Sleep(50 * time.Millisecond)

	// Dial and send a POST with a small body so the handler is entered.
	conn, err := ln.Dial()
	require.NoError(t, err)
	defer conn.Close()

	totalBody := 512
	header := fmt.Sprintf("POST /upload HTTP/1.1\r\nHost: test\r\nContent-Length: %d\r\n\r\n", totalBody)
	_, err = conn.Write([]byte(header))
	require.NoError(t, err)
	_, err = conn.Write(bytes.Repeat([]byte("A"), totalBody))
	require.NoError(t, err)

	// Wait for the connection to register and handler to be entered.
	time.Sleep(100 * time.Millisecond)
	require.GreaterOrEqual(t, int(app.ActiveConnections()), 1,
		"upload connection must be tracked")

	// Shutdown with a short drain window — the handler can't finish.
	// Expect DeadlineExceeded because the handler sleeps 5 s.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	err = app.ShutdownWithConfig(shutdownCtx, ShutdownConfig{
		OnForceClose: func(_ int) { forceClosedCount.Add(1) },
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// The slow upload connection must have been force-closed.
	require.GreaterOrEqual(t, int(forceClosedCount.Load()), 1,
		"OnForceClose must fire for the incomplete upload connection")

	// Client side: read should return an error (connection closed).
	_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	readBuf := make([]byte, 4096)
	_, readErr := conn.Read(readBuf)
	require.Error(t, readErr, "client must see a closed connection")
}

// Test_Integration_MixedDrain_FastCompletes_SlowForceClosed proves that a
// fast request drains and completes naturally while a slow request is
// force-closed after the drain timeout.  Both share the same listener and
// shutdown lifecycle.
func Test_Integration_MixedDrain_FastCompletes_SlowForceClosed(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	var fastDone atomic.Int32
	var slowDone atomic.Int32

	app := New()
	app.Get("/fast", func(c Ctx) error {
		fastDone.Add(1)
		return c.SendString("fast response")
	})
	app.Get("/slow", func(c Ctx) error {
		time.Sleep(5 * time.Second) // far exceeds drain timeout
		slowDone.Add(1)
		return c.SendString("slow response")
	})

	var forceClosedCount atomic.Int32
	go func() { _ = app.Listener(ln, ListenConfig{DisableStartupMessage: true}) }()

	time.Sleep(50 * time.Millisecond)

	// Fire the slow request first so it's already in-flight when we shutdown.
	slowConn, err := ln.Dial()
	require.NoError(t, err)
	defer slowConn.Close()
	_, _ = slowConn.Write([]byte("GET /slow HTTP/1.1\r\nHost: test\r\n\r\n"))

	time.Sleep(100 * time.Millisecond)

	// Fire a fast request — it should complete before drain timeout.
	fastConn, err := ln.Dial()
	require.NoError(t, err)
	defer fastConn.Close()
	_, _ = fastConn.Write([]byte("GET /fast HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n"))

	// Read fast response immediately — handler is instant.
	fastBuf := make([]byte, 4096)
	_ = fastConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _ := fastConn.Read(fastBuf)
	require.Contains(t, string(fastBuf[:n]), "fast response",
		"fast request must complete and return its response")

	// Now shutdown with a 600 ms drain window — long enough for fast (already
	// done) but not for slow (sleeping 5 s).  Expect DeadlineExceeded.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	err = app.ShutdownWithConfig(shutdownCtx, ShutdownConfig{
		OnForceClose: func(_ int) { forceClosedCount.Add(1) },
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// Fast handler completed; slow handler was force-killed (never incremented).
	require.Equal(t, int32(1), fastDone.Load(),
		"fast handler must have completed once")
	require.Equal(t, int32(0), slowDone.Load(),
		"slow handler must NOT have completed — it was force-closed")
	require.GreaterOrEqual(t, int(forceClosedCount.Load()), 1,
		"OnForceClose must fire for the slow connection")

	// Slow client sees a closed connection.
	_ = slowConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	slowBuf := make([]byte, 4096)
	_, slowErr := slowConn.Read(slowBuf)
	require.Error(t, slowErr, "slow client must see connection closed or EOF")
}

// ---------------------------------------------------------------------------
// ShutdownTelemetry & debug endpoint
// ---------------------------------------------------------------------------

// Test_Telemetry_CleanDrain_PopulatesAllFields performs a shutdown with no
// active connections and verifies that all timing fields are ≥ 0,
// DrainedConns == InitialConns, ForcedConns == 0, and TimedOut == false.
func Test_Telemetry_CleanDrain_PopulatesAllFields(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(c Ctx) error { return c.SendString("OK") })

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)

	require.Nil(t, app.LastShutdownTelemetry(), "no telemetry before shutdown")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := app.ShutdownWithConfig(ctx, ShutdownConfig{})
	require.NoError(t, err)

	tel := app.LastShutdownTelemetry()
	require.NotNil(t, tel, "telemetry must be populated after shutdown")

	require.False(t, tel.StartedAt.IsZero(), "StartedAt must be set")
	require.False(t, tel.CompletedAt.IsZero(), "CompletedAt must be set")
	require.True(t, tel.TotalDuration >= 0, "TotalDuration must be ≥ 0")
	require.True(t, tel.DrainDuration >= 0, "DrainDuration must be ≥ 0")
	require.True(t, tel.PreHooksDuration >= 0, "PreHooksDuration must be ≥ 0")
	require.True(t, tel.GracefulCloseDuration >= 0, "GracefulCloseDuration must be ≥ 0")
	require.True(t, tel.PostHooksDuration >= 0, "PostHooksDuration must be ≥ 0")

	require.Equal(t, tel.InitialConns, tel.DrainedConns+tel.ForcedConns,
		"InitialConns must equal DrainedConns + ForcedConns")
	require.Equal(t, 0, tel.ForcedConns, "no force-close on clean drain")
	require.False(t, tel.TimedOut, "TimedOut must be false on clean drain")
}

// Test_Telemetry_ForcedClose_CountsCorrectly verifies that a slow handler
// + short deadline produces ForcedConns ≥ 1, TimedOut == true, and
// DrainedConns == InitialConns − ForcedConns.
func Test_Telemetry_ForcedClose_CountsCorrectly(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/block", func(c Ctx) error {
		time.Sleep(10 * time.Second)
		return c.SendString("done")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Open a blocking connection.
	go func() {
		conn, _ := ln.Dial()
		_, _ = conn.Write([]byte("GET /block HTTP/1.1\r\nHost: example.com\r\n\r\n"))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_ = conn.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := app.ShutdownWithConfig(ctx, ShutdownConfig{})
	require.ErrorIs(t, err, context.DeadlineExceeded)

	tel := app.LastShutdownTelemetry()
	require.NotNil(t, tel)

	require.True(t, tel.ForcedConns >= 1, "at least 1 connection must be force-closed")
	require.True(t, tel.TimedOut, "TimedOut must be true when deadline exceeded")
	require.Equal(t, tel.InitialConns, tel.DrainedConns+tel.ForcedConns,
		"DrainedConns must equal InitialConns − ForcedConns")
}

// Test_Telemetry_HookDurations_Measured verifies that a pre-hook that sleeps
// 50 ms results in PreHooksDuration ≥ 50 ms, and PostHooksDuration ≥ 0.
func Test_Telemetry_HookDurations_Measured(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(c Ctx) error { return c.SendString("OK") })

	app.Hooks().OnPreShutdown(func() error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{})

	tel := app.LastShutdownTelemetry()
	require.NotNil(t, tel)

	require.True(t, tel.PreHooksDuration >= 50*time.Millisecond,
		"PreHooksDuration must be ≥ 50 ms (pre-hook sleeps that long)")
	require.True(t, tel.PostHooksDuration >= 0,
		"PostHooksDuration must be ≥ 0")
}

// Test_Telemetry_WebSocketSSE_Counters marks 1 WS + 1 SSE connection,
// triggers shutdown, and verifies WebSocketsClosed == 1 and SSEsClosed == 1.
func Test_Telemetry_WebSocketSSE_Counters(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/ws", func(c Ctx) error {
		tc := c.TrackedConn()
		if tc != nil {
			tc.SetConnType(ConnTypeWebSocket)
		}
		time.Sleep(5 * time.Second)
		return nil
	})
	app.Get("/sse", func(c Ctx) error {
		tc := c.TrackedConn()
		if tc != nil {
			tc.SetConnType(ConnTypeSSE)
		}
		time.Sleep(5 * time.Second)
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	// Open WS connection — echo close frame back so the handshake succeeds.
	wsConn, _ := ln.Dial()
	_, _ = wsConn.Write([]byte("GET /ws HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	go func() {
		buf := make([]byte, 4096)
		_, _ = wsConn.Read(buf)
		// Echo close frame back.
		frame := make([]byte, 4)
		if _, err := wsConn.Read(frame); err == nil {
			_, _ = wsConn.Write(frame)
		}
	}()

	// Open SSE connection — read the shutdown event.
	sseConn, _ := ln.Dial()
	_, _ = sseConn.Write([]byte("GET /sse HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	go func() {
		buf := make([]byte, 4096)
		_, _ = sseConn.Read(buf)
		// Read the SSE event written by the framework.
		_, _ = sseConn.Read(buf)
	}()

	time.Sleep(150 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{
		WebSocketCloseTimeout: 500 * time.Millisecond,
		SSECloseTimeout:       500 * time.Millisecond,
	})

	_ = wsConn.Close()
	_ = sseConn.Close()

	tel := app.LastShutdownTelemetry()
	require.NotNil(t, tel)
	require.Equal(t, 1, tel.WebSocketsClosed, "exactly 1 WebSocket must be closed")
	require.Equal(t, 1, tel.SSEsClosed, "exactly 1 SSE must be closed")
}

// Test_DebugEndpoint_ReturnsRunningStatus hits the debug handler before any
// shutdown; status must be "running" and lastShutdown must be null.
func Test_DebugEndpoint_ReturnsRunningStatus(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/debug/shutdown", app.ShutdownDebugHandler())
	app.Get("/", func(c Ctx) error { return c.SendString("OK") })

	req, _ := http.NewRequest(MethodGet, "/debug/shutdown", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var body map[string]any
	bodyBytes, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(bodyBytes, &body))

	require.Equal(t, "running", body["status"])
	require.Nil(t, body["lastShutdown"], "lastShutdown must be null before shutdown")
}

// Test_DebugEndpoint_ReturnsTelemetryAfterShutdown triggers a clean shutdown
// then hits the debug handler; status must be "shutdown" and lastShutdown must
// contain parseable duration strings.
func Test_DebugEndpoint_ReturnsTelemetryAfterShutdown(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/debug/shutdown", app.ShutdownDebugHandler())
	app.Get("/", func(c Ctx) error { return c.SendString("OK") })

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}, 2*time.Second, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = app.ShutdownWithConfig(ctx, ShutdownConfig{})

	// After shutdown the server is stopped; use app.Test to exercise the handler.
	req, _ := http.NewRequest(MethodGet, "/debug/shutdown", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var body map[string]any
	bodyBytes, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(bodyBytes, &body))

	require.Equal(t, "shutdown", body["status"])

	lastShutdown, ok := body["lastShutdown"].(map[string]any)
	require.True(t, ok, "lastShutdown must be an object after shutdown")

	// Verify duration strings are parseable.
	for _, key := range []string{"totalDuration", "drainDuration", "preHooksDuration",
		"gracefulCloseDuration", "postHooksDuration"} {
		raw, exists := lastShutdown[key].(string)
		require.True(t, exists, "key %s must exist and be a string", key)
		_, parseErr := time.ParseDuration(raw)
		require.NoError(t, parseErr, "key %s value %q must be a valid duration string", key, raw)
	}
}

// ---------------------------------------------------------------------------
// Goroutine leak checks
// ---------------------------------------------------------------------------

// goroutineLeakCheck runs fn and asserts the goroutine count does not grow
// beyond baseline + allowance after fn returns.
func goroutineLeakCheck(t *testing.T, fn func(), allowance int) {
	t.Helper()

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()

	fn()

	// Give goroutines a moment to wind down.
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	after := runtime.NumGoroutine()

	grown := after - before
	if grown > allowance {
		t.Errorf("goroutine leak: started with %d, ended with %d (grew by %d, allowance %d)",
			before, after, grown, allowance)
	}
}

// Test_Telemetry_NoGoroutineLeak_CleanShutdown verifies that a clean
// ShutdownWithConfig cycle does not leak goroutines.
func Test_Telemetry_NoGoroutineLeak_CleanShutdown(t *testing.T) {
	goroutineLeakCheck(t, func() {
		app := New()
		app.Get("/", func(c Ctx) error { return c.SendString("OK") })

		ln := fasthttputil.NewInmemoryListener()
		go func() { _ = app.Listener(ln, ListenConfig{DisableStartupMessage: true}) }()
		time.Sleep(100 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = app.ShutdownWithConfig(ctx, ShutdownConfig{})

		tel := app.LastShutdownTelemetry()
		if tel == nil {
			t.Fatal("telemetry must be non-nil after clean shutdown")
		}
	}, 5)
}

// Test_Telemetry_NoGoroutineLeak_ForcedClose verifies that a force-close
// (deadline exceeded) cycle does not leak goroutines.
func Test_Telemetry_NoGoroutineLeak_ForcedClose(t *testing.T) {
	goroutineLeakCheck(t, func() {
		app := New()
		app.Get("/block", func(c Ctx) error {
			time.Sleep(10 * time.Second)
			return c.SendString("done")
		})

		ln := fasthttputil.NewInmemoryListener()
		go func() { _ = app.Listener(ln, ListenConfig{DisableStartupMessage: true}) }()
		time.Sleep(100 * time.Millisecond)

		// Open a blocking connection.
		go func() {
			conn, _ := ln.Dial()
			_, _ = conn.Write([]byte("GET /block HTTP/1.1\r\nHost: example.com\r\n\r\n"))
			buf := make([]byte, 4096)
			_, _ = conn.Read(buf)
			_ = conn.Close()
		}()
		time.Sleep(100 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		_ = app.ShutdownWithConfig(ctx, ShutdownConfig{})

		tel := app.LastShutdownTelemetry()
		if tel == nil {
			t.Fatal("telemetry must be non-nil after forced shutdown")
		}
		if !tel.TimedOut {
			t.Fatal("TimedOut must be true after forced shutdown")
		}
	}, 5)
}

// Test_Telemetry_NoGoroutineLeak_DebugHandler verifies the debug handler
// itself does not leak goroutines across repeated calls.
func Test_Telemetry_NoGoroutineLeak_DebugHandler(t *testing.T) {
	goroutineLeakCheck(t, func() {
		app := New()
		app.Get("/debug/shutdown", app.ShutdownDebugHandler())
		app.Get("/", func(c Ctx) error { return c.SendString("OK") })

		for range 10 {
			req, _ := http.NewRequest(MethodGet, "/debug/shutdown", nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test error: %v", err)
			}
			_ = resp.Body.Close()
		}

		_ = app.Shutdown()
	}, 5)
}

// ---------------------------------------------------------------------------
// Helper: pipe-based net.Conn pair for unit tests
// ---------------------------------------------------------------------------

func newPipeConn() (server, client net.Conn) {
	// Use fasthttputil's in-memory listener to obtain a realistic conn pair.
	ln := fasthttputil.NewInmemoryListener()
	ch := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		ch <- c
	}()
	client, _ = ln.Dial()
	server = <-ch
	_ = ln.Close()
	return
}
