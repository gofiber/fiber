// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"io"
	"mime/multipart"
	"runtime"
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

// An application that releases its timed-out contexts itself must not pool one a second time.
func TestDefaultCtx_ForceRelease_LeavesReclaimedCtx(t *testing.T) {
	t.Parallel()
	t.Run("reclaim armed", func(t *testing.T) {
		t.Parallel()
		app, c := acquireReclaimTestCtx(t)
		handlerDone := make(chan struct{})
		c.ScheduleReclaim(handlerDone, nil)
		c.ForceRelease()
		require.True(t, c.IsAbandoned())
		require.NotNil(t, c.fasthttp)

		close(handlerDone)
		app.ReleaseCtx(c)
		ctxExpectReclaimed(t, c, "the reclaim must still pool the ctx")
	})
	t.Run("not abandoned", func(t *testing.T) {
		t.Parallel()
		_, c := acquireReclaimTestCtx(t)
		c.route = &Route{}
		c.ForceRelease()
		require.NotNil(t, c.route)
		require.NotNil(t, c.fasthttp)
	})
}

// A ForceRelease that runs the moment ScheduleReclaim abandons the ctx must see the reclaim it arms.
func TestDefaultCtx_ForceRelease_DuringScheduleReclaim(t *testing.T) {
	t.Parallel()
	app := New()
	for range 2000 {
		c, ok := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)
		require.True(t, ok)
		handlerDone, armed := make(chan struct{}), make(chan struct{})
		go func() {
			defer close(armed)
			c.ScheduleReclaim(handlerDone, nil)
		}()
		for !c.IsAbandoned() {
			runtime.Gosched()
		}
		c.ForceRelease()
		<-armed
		require.NotNil(t, c.fasthttp, "ForceRelease pooled a ctx the reclaim pools as well")
		close(handlerDone)
		app.ReleaseCtx(c)
	}
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

type reclaimCloser struct {
	io.Reader
	closed *atomic.Int32
}

func (r reclaimCloser) Close() error {
	r.closed.Add(1)
	return nil
}

// fasthttp never resets a RequestCtx it answered with a timeout response, so the reclaim closes what the request
// left open, and only once the handler is done. Any other RequestCtx is fasthttp's to reset, and one a fasthttp
// handler gave up on as well is that handler's, which still runs on it.
func TestDefaultCtx_ScheduleReclaim_ClosesTimedOutRequest(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ timedOut, handlerGaveUp bool }{
		"timed out":           {timedOut: true},
		"not timed out":       {},
		"handler gave up too": {timedOut: true, handlerGaveUp: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app, c := acquireReclaimTestCtx(t)

			var body bytes.Buffer
			w := multipart.NewWriter(&body)
			part, err := w.CreateFormFile("file", "upload.bin")
			require.NoError(t, err)
			_, err = part.Write(make([]byte, 64<<10))
			require.NoError(t, err)
			require.NoError(t, w.Close())
			c.Request().Header.SetContentType(w.FormDataContentType())
			c.Request().SetBodyStream(&body, body.Len()) // a streamed upload is kept in a temp file
			form, err := c.MultipartForm()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, form.RemoveAll()) })
			upload := form.File["file"][0]
			onDisk := func() bool {
				f, err := upload.Open()
				return err == nil && f.Close() == nil
			}
			require.True(t, onDisk())

			var closed atomic.Int32
			c.Locals("closer", reclaimCloser{closed: &closed})
			require.NoError(t, c.SendStream(reclaimCloser{Reader: strings.NewReader("body"), closed: &closed}))
			if tc.timedOut {
				c.RequestCtx().TimeoutErrorWithResponse(&fasthttp.Response{})
			}
			untouched := func() bool { return closed.Load() == 0 && onDisk() }

			handlerDone := make(chan struct{})
			c.ScheduleReclaim(handlerDone, nil)
			if tc.handlerGaveUp {
				c.RequestCtx().TimeoutError("busy")
			}
			app.ReleaseCtx(c)
			require.Never(t, func() bool { return !untouched() }, 50*time.Millisecond, 5*time.Millisecond)

			close(handlerDone)
			if tc.handlerGaveUp {
				require.Never(t, func() bool { return !untouched() || !c.IsAbandoned() }, 50*time.Millisecond, 5*time.Millisecond)
				return
			}
			ctxExpectReclaimed(t, c, "ctx must be reclaimed")
			if !tc.timedOut {
				require.Never(t, func() bool { return !untouched() }, 50*time.Millisecond, 5*time.Millisecond)
				return
			}
			require.Eventually(t, func() bool { return closed.Load() == 2 && !onDisk() }, time.Second, 5*time.Millisecond)
		})
	}
}
