package session

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// startMiddleware runs the beginning of the session handler for store on c: the
// middleware is acquired, loads its session and becomes the current middleware of
// the request. releaseMiddleware is its end.
func startMiddleware(t *testing.T, c fiber.Ctx, store *Store) *Middleware {
	t.Helper()

	cfg := store.Config
	cfg.Store = store

	m := acquireMiddleware()
	require.NoError(t, m.initialize(c, &cfg))
	return m
}

// A middleware that runs inside another links to it and lets go when it finishes,
// and the outer one is the current middleware of the request again.
func Test_Session_Middleware_NestedLinkIsBalanced(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})

	outer := startMiddleware(t, c, storeA)
	inner := startMiddleware(t, c, storeB)
	require.Same(t, outer, inner.parent)
	require.Equal(t, int32(1), outer.children.Load())
	require.Same(t, inner, c.Locals(middlewareContextKey))

	innerSession := inner.Session
	releaseMiddleware(inner)
	require.Nil(t, inner.parent, "a finished middleware keeps no link")
	require.Nil(t, inner.Session)
	require.Empty(t, innerSession.ID(), "its session went back to the pool with it")
	require.Zero(t, outer.children.Load(), "and the outer one is not counted as having it inside any more")
	require.Same(t, outer, c.Locals(middlewareContextKey), "the outer middleware is current again")
	require.False(t, outer.released.Load())

	releaseMiddleware(outer)
	require.False(t, outer.released.Load(), "nothing refers to it, so it is recycled like any other")
	require.Nil(t, c.Locals(middlewareContextKey))
}

// A middleware links only to one that serves its own request: a Middleware is
// recycled, and an abandoned handler can come back to it after another request
// has taken it over.
func Test_Session_Middleware_AdoptOnlyWithinItsRequest(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c1 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c1)
	c2 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c2)

	store := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	outer := startMiddleware(t, c1, store)

	require.False(t, outer.adopt(c2), "a middleware of another request is no parent")
	require.Zero(t, outer.children.Load(), "and nothing was counted for the refusal")

	require.True(t, outer.adopt(c1))
	require.Equal(t, int32(1), outer.children.Load())
	outer.unlink()
	require.Zero(t, outer.children.Load())

	// A finished middleware that was dropped instead of recycled, so that the test
	// can look at it after it let go.
	outer.escaped = true
	releaseMiddleware(outer)
	require.True(t, outer.released.Load())
	require.False(t, outer.adopt(c1), "a middleware that finished is no parent either")
	require.Zero(t, outer.children.Load())
}

// The shape a timeout middleware gives a request: the outer middleware finishes
// and its request is answered while the middleware inside it still runs. The
// outer one must not be recycled then, for the inner one holds it, and when the
// inner one finishes it must not hand the request an outer middleware that now
// belongs to somebody else.
func Test_Session_Middleware_OuterFinishesBeforeInner(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c1 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c1)
	c2 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c2)

	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})

	outer := startMiddleware(t, c1, storeA)
	inner := startMiddleware(t, c1, storeB)

	// The outer middleware finishes first.
	releaseMiddleware(outer)
	require.True(t, outer.released.Load(), "an outer middleware with one still inside it is not recycled")
	require.Nil(t, c1.Locals(middlewareContextKey))
	require.Nil(t, storeA.FromContext(c1))
	require.Same(t, outer, inner.parent, "the inner middleware still holds what it was linked to")

	// Another request starts. Nothing the pool hands out is the old outer middleware.
	for range 100 {
		got := acquireMiddleware()
		require.NotSame(t, outer, got, "the outer middleware went back to the pool with a middleware inside it")
		releaseMiddleware(got)
	}
	other := startMiddleware(t, c2, storeA)
	require.NotSame(t, outer, other)
	otherSession := other.Session

	// The inner middleware finishes last.
	releaseMiddleware(inner)
	require.Nil(t, c1.Locals(middlewareContextKey), "the request got a finished middleware back")
	require.Empty(t, c1.Locals(sessionIDContextKey))
	require.Zero(t, outer.children.Load())

	require.Same(t, other, c2.Locals(middlewareContextKey), "the other request kept its own")
	require.Same(t, otherSession, other.Session)
	require.NotEmpty(t, other.Session.ID())
	releaseMiddleware(other)
}

// A timeout middleware around a session middleware, behind another one. The
// handlers inside the timeout outlive the answer. The outer middleware is not
// recycled while they may still use it, so the next request is served by another
// one before they finish. What the abandoned handler finds in the request
// afterwards may be the middleware of its own request, which stays alive for it,
// but must not be the session of the request that came after.
//
// The answer waits until the inner middleware has started, so that what the
// timeout leaves running is ordered after what it left behind, and the test
// does not trip over the race detector on the shared request locals, which the
// timeout middleware leaves to the application.
func Test_Session_MultipleStores_AbandonedByTimeout(t *testing.T) {
	t.Parallel()

	type attempt struct {
		slowMayFinish chan struct{} // closed to let the abandoned handler finish
		innerStarted  chan struct{} // closed once the abandoned handler runs inside the inner middleware
		otherHolds    chan struct{} // closed once the next request has stored its secret
		abandonedDone chan struct{} // closed once the abandoned handler looked around
		seen          atomic.Value  // what the abandoned handler found after its inner middleware finished
		injected      atomic.Value  // what the next request found in its own session at the end
	}
	var current atomic.Pointer[attempt]

	handlerA, storeA := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
	handlerB, _ := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})

	app := fiber.New()
	app.Use(handlerA)
	app.Get("/slow", timeout.New(func(c fiber.Ctx) error {
		att := current.Load()
		err := c.Next() // handlerB starts and finishes in here, after handlerA did
		if m := FromContext(c); m != nil {
			att.seen.Store(fmt.Sprint(m.Get("marker")))
			m.Set("injected", "from-the-abandoned-handler")
		} else {
			att.seen.Store("nil")
		}
		close(att.abandonedDone)
		return err
	}, timeout.Config{
		Timeout: 10 * time.Millisecond,
		OnTimeout: func(fiber.Ctx) error {
			select {
			case <-current.Load().innerStarted:
			case <-time.After(5 * time.Second):
			}
			return fiber.ErrServiceUnavailable
		},
	}), handlerB, func(c fiber.Ctx) error {
		att := current.Load()
		close(att.innerStarted)
		<-att.slowMayFinish
		return c.SendString("slow")
	})
	app.Get("/next", func(c fiber.Ctx) error {
		att := current.Load()
		storeA.FromContext(c).Set("marker", "secret-of-the-next-request")
		close(att.otherHolds)
		select {
		case <-att.abandonedDone:
		case <-time.After(5 * time.Second):
		}
		att.injected.Store(fmt.Sprint(storeA.FromContext(c).Get("injected")))
		return c.SendString("next")
	})

	for range 25 {
		att := &attempt{
			slowMayFinish: make(chan struct{}),
			innerStarted:  make(chan struct{}),
			otherHolds:    make(chan struct{}),
			abandonedDone: make(chan struct{}),
		}
		current.Store(att)

		resp, _ := sendRequest(t, app, "/slow", nil)
		require.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)

		// The middleware of the answered request stays out of the pool, so the next request gets another one.
		nextDone := make(chan error, 1)
		go func() {
			_, _, err := doRequest(app, "/next", nil)
			nextDone <- err
		}()
		<-att.otherHolds
		close(att.slowMayFinish)
		<-att.abandonedDone
		require.NoError(t, <-nextDone)

		require.NotContains(t, att.seen.Load(), "secret-of-the-next-request", "the abandoned handler reached the session of the next request")
		require.Equal(t, "<nil>", att.injected.Load(), "the abandoned handler wrote into another request's session")
	}
}

// Finding a store's middleware, and telling that a session belongs to one, cost a
// request nothing: they walk the middleware without allocating.
//
// testing.AllocsPerRun panics in a parallel test, so this one stays sequential.
func Test_Session_Middleware_FindDoesNotAllocate(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})
	outer := startMiddleware(t, c, storeA)
	inner := startMiddleware(t, c, storeB)
	defer releaseMiddleware(outer)
	defer releaseMiddleware(inner)

	require.Same(t, inner, storeB.FromContext(c))
	require.Same(t, outer, storeA.FromContext(c))

	save := func(s *Session) func() {
		return func() {
			if err := s.Save(); err != nil {
				t.Error(err)
			}
		}
	}

	require.Zero(t, testing.AllocsPerRun(100, func() { _ = storeB.FromContext(c) }), "the innermost middleware")
	require.Zero(t, testing.AllocsPerRun(100, func() { _ = storeA.FromContext(c) }), "the one it runs inside")
	require.Zero(t, testing.AllocsPerRun(100, save(inner.Session)), "the session of the innermost middleware")
	require.Zero(t, testing.AllocsPerRun(100, save(outer.Session)), "the session of the one it runs inside")
}
