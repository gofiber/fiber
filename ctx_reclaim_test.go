// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// acquireReclaimTestCtx wires up a *DefaultCtx for reclaim tests without going
// through a full request lifecycle, so each test can drive ScheduleReclaim,
// signalReleased, and ReleaseCtx in isolation. The tests release the context
// themselves: when and whether it is released is what they exercise.
func acquireReclaimTestCtx(t *testing.T) (*App, *DefaultCtx) {
	t.Helper()
	app := New()
	raw := app.AcquireCtx(&fasthttp.RequestCtx{})
	dc, ok := raw.(*DefaultCtx)
	require.True(t, ok, "AcquireCtx must return *DefaultCtx in tests")
	return app, dc
}

// ctxExpectReclaimed requires that c is returned to the pool, observable as
// IsAbandoned flipping back to false, and that msg holds when it is.
func ctxExpectReclaimed(t *testing.T, c *DefaultCtx, msg string) {
	t.Helper()
	require.Eventually(t, func() bool { return !c.IsAbandoned() }, time.Second, 5*time.Millisecond, msg)
}

// TestDefaultCtx_ScheduleReclaim_HappyPath covers the dominant timed-out flow:
// after ScheduleReclaim is armed and both signals fire (handlerDone closes and
// the request handler releases the context), the context is returned to the
// pool, observable as IsAbandoned flipping back to false.
func TestDefaultCtx_ScheduleReclaim_HappyPath(t *testing.T) {
	t.Parallel()
	app, c := acquireReclaimTestCtx(t)

	handlerDone := make(chan struct{})
	c.ScheduleReclaim(handlerDone, nil)
	require.True(t, c.IsAbandoned(), "ScheduleReclaim must Abandon the ctx internally")

	close(handlerDone)
	require.True(t, c.IsAbandoned(), "ctx must stay abandoned until ReleaseCtx fires")

	app.ReleaseCtx(c)
	ctxExpectReclaimed(t, c, "ctx must be reclaimed once both signals fire")
}

// TestDefaultCtx_ScheduleReclaim_ReleaseBeforeHandlerDone covers the reverse
// ordering: the context is released first, then handlerDone closes. The reclaim
// goroutine must still wait on handlerDone before pooling. Both release entry
// points fire the latch: ReleaseCtx, the public CustomCtx path, and the internal
// releaseDefaultCtx, which defaultRequestHandler's defer calls.
func TestDefaultCtx_ScheduleReclaim_ReleaseBeforeHandlerDone(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		release func(app *App, c *DefaultCtx)
		name    string
	}{
		{name: "ReleaseCtx", release: func(app *App, c *DefaultCtx) { app.ReleaseCtx(c) }},
		{name: "releaseDefaultCtx", release: (*App).releaseDefaultCtx},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, c := acquireReclaimTestCtx(t)

			handlerDone := make(chan struct{})
			c.ScheduleReclaim(handlerDone, nil)

			tc.release(app, c)
			require.True(t, c.IsAbandoned(), "an abandoned ctx must not be pooled until handlerDone closes")

			close(handlerDone)
			ctxExpectReclaimed(t, c, "the release must wire signalReleased into the latch")
		})
	}
}

// TestDefaultCtx_ScheduleReclaim_Nested arms one ctx twice, as nested timeout
// middlewares do, in every order relative to the release.
func TestDefaultCtx_ScheduleReclaim_Nested(t *testing.T) {
	t.Parallel()
	for _, order := range [][]string{
		{"outer", "inner", "release"},
		{"inner", "outer", "release"},
		{"outer", "release", "inner"},
	} {
		t.Run(strings.Join(order, "_"), func(t *testing.T) {
			t.Parallel()
			app, c := acquireReclaimTestCtx(t)

			var calls atomic.Int32
			cancel := func() { calls.Add(1) }
			outerDone, innerDone := make(chan struct{}), make(chan struct{})

			steps := map[string]func(){
				"outer":   func() { c.ScheduleReclaim(outerDone, cancel) },
				"inner":   func() { c.ScheduleReclaim(innerDone, cancel) },
				"release": func() { app.ReleaseCtx(c) },
			}
			for _, step := range order {
				steps[step]()
			}

			close(outerDone)
			require.Never(t, func() bool {
				return !c.IsAbandoned()
			}, 50*time.Millisecond, 5*time.Millisecond, "ctx must not be reclaimed while the inner handler is still running")

			close(innerDone)
			require.Eventually(t, func() bool {
				return !c.IsAbandoned() && calls.Load() == 2
			}, time.Second, 5*time.Millisecond, "ctx must be reclaimed once both goroutines have finished")
		})
	}
}

// TestDefaultCtx_ScheduleReclaim_Concurrent arms the ctx from the request
// goroutine and from the goroutine it abandons at the same time.
func TestDefaultCtx_ScheduleReclaim_Concurrent(t *testing.T) {
	t.Parallel()
	app, c := acquireReclaimTestCtx(t)

	outerDone, innerDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(outerDone)
		c.ScheduleReclaim(innerDone, nil)
	}()
	c.ScheduleReclaim(outerDone, nil)
	app.ReleaseCtx(c)

	<-outerDone
	require.Never(t, func() bool {
		return !c.IsAbandoned()
	}, 50*time.Millisecond, 5*time.Millisecond, "ctx must not be reclaimed while the inner handler is still running")

	close(innerDone)
	ctxExpectReclaimed(t, c, "ctx must be reclaimed once both goroutines have finished")
}

// TestDefaultCtx_ScheduleReclaim_Cancel verifies the cancel hook fires exactly
// once when the handler goroutine finishes, and that a nil cancel does not block
// reclamation.
func TestDefaultCtx_ScheduleReclaim_Cancel(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		nilCancel bool
	}{
		{name: "cancel invoked"},
		{name: "nil cancel", nilCancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, c := acquireReclaimTestCtx(t)

			var calls atomic.Int32
			cancel := func() { calls.Add(1) }
			if tc.nilCancel {
				cancel = nil
			}

			handlerDone := make(chan struct{})
			c.ScheduleReclaim(handlerDone, cancel)

			close(handlerDone)
			app.ReleaseCtx(c)

			require.Eventually(t, func() bool {
				return (tc.nilCancel || calls.Load() == 1) && !c.IsAbandoned()
			}, time.Second, 5*time.Millisecond, "cancel must fire once, if there is one, and ctx must be reclaimed")
		})
	}
}

// TestDefaultCtx_signalReleased_NoReclaim guards that calling signalReleased on
// a context that was never armed for reclamation is a safe no-op. This is the
// SSE-style path (Abandon without ScheduleReclaim) hit through ReleaseCtx.
func TestDefaultCtx_signalReleased_NoReclaim(t *testing.T) {
	t.Parallel()
	_, c := acquireReclaimTestCtx(t)
	require.NotPanics(t, func() { c.signalReleased() })
	require.NotPanics(t, func() { c.signalReleased() })
}

// TestDefaultCtx_signalReleased_Idempotent guards the sync.Once semantics: even
// if the request release path fires multiple times, the latch must close once.
func TestDefaultCtx_signalReleased_Idempotent(t *testing.T) {
	t.Parallel()
	_, c := acquireReclaimTestCtx(t)

	handlerDone := make(chan struct{})
	c.ScheduleReclaim(handlerDone, nil)

	require.NotPanics(t, func() {
		c.signalReleased()
		c.signalReleased()
		c.signalReleased()
	})

	close(handlerDone)
	ctxExpectReclaimed(t, c, "ctx must still be reclaimed exactly once")
}

// TestApp_releaseDefaultCtx_NotAbandonedPools verifies the non-abandoned branch
// in releaseDefaultCtx still pools the ctx through the normal path.
func TestApp_releaseDefaultCtx_NotAbandonedPools(t *testing.T) {
	t.Parallel()
	app, c := acquireReclaimTestCtx(t)

	require.False(t, c.IsAbandoned())
	require.NotPanics(t, func() { app.releaseDefaultCtx(c) })
}
