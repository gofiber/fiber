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
// signalReleased, and ReleaseCtx in isolation.
func acquireReclaimTestCtx(t *testing.T) (*App, *DefaultCtx) {
	t.Helper()
	app := New()
	raw := app.AcquireCtx(&fasthttp.RequestCtx{})
	dc, ok := raw.(*DefaultCtx)
	require.True(t, ok, "AcquireCtx must return *DefaultCtx in tests")
	return app, dc
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
	require.Eventually(t, func() bool {
		return !c.IsAbandoned()
	}, time.Second, 5*time.Millisecond, "ctx must be reclaimed once both signals fire")
}

// TestDefaultCtx_ScheduleReclaim_ReleaseBeforeHandlerDone covers the reverse
// ordering: ReleaseCtx is called first, then handlerDone closes. The reclaim
// goroutine must still wait on handlerDone before pooling.
func TestDefaultCtx_ScheduleReclaim_ReleaseBeforeHandlerDone(t *testing.T) {
	t.Parallel()
	app, c := acquireReclaimTestCtx(t)

	handlerDone := make(chan struct{})
	c.ScheduleReclaim(handlerDone, nil)

	app.ReleaseCtx(c)
	require.True(t, c.IsAbandoned(), "ctx must stay abandoned until handlerDone closes")

	close(handlerDone)
	require.Eventually(t, func() bool {
		return !c.IsAbandoned()
	}, time.Second, 5*time.Millisecond, "ctx must be reclaimed after handlerDone closes")
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
	require.Eventually(t, func() bool {
		return !c.IsAbandoned()
	}, time.Second, 5*time.Millisecond, "ctx must be reclaimed once both goroutines have finished")
}

// TestDefaultCtx_ScheduleReclaim_CancelInvoked verifies the cancel hook fires
// exactly once when the handler goroutine finishes.
func TestDefaultCtx_ScheduleReclaim_CancelInvoked(t *testing.T) {
	t.Parallel()
	app, c := acquireReclaimTestCtx(t)

	var calls atomic.Int32
	cancel := func() { calls.Add(1) }

	handlerDone := make(chan struct{})
	c.ScheduleReclaim(handlerDone, cancel)

	close(handlerDone)
	app.ReleaseCtx(c)

	require.Eventually(t, func() bool {
		return calls.Load() == 1 && !c.IsAbandoned()
	}, time.Second, 5*time.Millisecond, "cancel must fire once and ctx must be reclaimed")
}

// TestDefaultCtx_ScheduleReclaim_NilCancel exercises the cancel==nil branch.
func TestDefaultCtx_ScheduleReclaim_NilCancel(t *testing.T) {
	t.Parallel()
	app, c := acquireReclaimTestCtx(t)

	handlerDone := make(chan struct{})
	c.ScheduleReclaim(handlerDone, nil)

	close(handlerDone)
	app.ReleaseCtx(c)

	require.Eventually(t, func() bool {
		return !c.IsAbandoned()
	}, time.Second, 5*time.Millisecond, "nil cancel must not block reclamation")
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
	app, c := acquireReclaimTestCtx(t)

	handlerDone := make(chan struct{})
	c.ScheduleReclaim(handlerDone, nil)

	require.NotPanics(t, func() {
		c.signalReleased()
		c.signalReleased()
		c.signalReleased()
	})

	close(handlerDone)
	require.Eventually(t, func() bool {
		return !c.IsAbandoned()
	}, time.Second, 5*time.Millisecond, "ctx must still be reclaimed exactly once")

	_ = app
}

// TestApp_releaseDefaultCtx_AbandonedSignalsReclaim exercises the internal
// releaseDefaultCtx path (called by defaultRequestHandler's defer) for an
// abandoned, reclaim-armed context. It mirrors what ReleaseCtx does for the
// public CustomCtx path and ensures both release entry points fire the latch.
func TestApp_releaseDefaultCtx_AbandonedSignalsReclaim(t *testing.T) {
	t.Parallel()
	app, c := acquireReclaimTestCtx(t)

	handlerDone := make(chan struct{})
	c.ScheduleReclaim(handlerDone, nil)

	app.releaseDefaultCtx(c)
	require.True(t, c.IsAbandoned(), "abandoned ctx must not be pooled by releaseDefaultCtx")

	close(handlerDone)
	require.Eventually(t, func() bool {
		return !c.IsAbandoned()
	}, time.Second, 5*time.Millisecond, "releaseDefaultCtx must wire signalReleased into the latch")
}

// TestApp_releaseDefaultCtx_NotAbandonedPools verifies the non-abandoned branch
// in releaseDefaultCtx still pools the ctx through the normal path.
func TestApp_releaseDefaultCtx_NotAbandonedPools(t *testing.T) {
	t.Parallel()
	app, c := acquireReclaimTestCtx(t)

	require.False(t, c.IsAbandoned())
	require.NotPanics(t, func() { app.releaseDefaultCtx(c) })
}
