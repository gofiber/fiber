package session

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
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

	// Dropped instead of recycled, so that the test can look at it afterwards
	// without a parallel test having taken it from the pool in the meantime.
	inner.escaped = true
	releaseMiddleware(inner)
	require.Nil(t, inner.parent, "a finished middleware keeps no link")
	require.Nil(t, inner.Session)
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

// Not parallel: it reads the shared session pool, which every other test uses,
// after the sessions went back to it, and a parallel test could take one in between.
func Test_Session_ReleaseMiddleware_ReleasesTheSession(t *testing.T) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	outer := startMiddleware(t, c, NewStore(Config{Extractor: extractors.FromCookie("a_sid")}))
	inner := startMiddleware(t, c, NewStore(Config{Extractor: extractors.FromCookie("b_sid")}))
	outerSession, innerSession := outer.Session, inner.Session
	require.NotEmpty(t, outerSession.ID())
	require.NotEmpty(t, innerSession.ID())

	releaseMiddleware(inner)
	require.Nil(t, inner.Session)
	require.Empty(t, innerSession.ID(), "the inner session was reset and went back to the pool with its middleware")
	require.NotEmpty(t, outerSession.ID(), "and the outer one is still the outer middleware's")

	releaseMiddleware(outer)
	require.Nil(t, outer.Session)
	require.Empty(t, outerSession.ID())
}

// A handler that a timeout middleware abandoned can hold the middleware and the
// session of its request after the request is answered. Neither is recycled then,
// or the next request would find itself in the abandoned handler's hands.
func Test_Session_ReleaseMiddleware_AbandonedRequestKeepsItsObjects(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer c.ForceRelease()

	m := startMiddleware(t, c, NewStore())
	sess := m.Session
	sess.Set("k", "v")

	c.Abandon()
	releaseMiddleware(m)
	require.True(t, m.released.Load(), "the middleware is marked finished, so lookups stop returning it")
	require.Nil(t, m.Session)

	for range 100 {
		got := acquireMiddleware()
		require.NotSame(t, m, got, "the middleware of an abandoned request went back to the pool")
		releaseMiddleware(got)

		next := acquireSession()
		require.NotSame(t, sess, next, "the session of an abandoned request went back to the pool")
		releaseSession(next)
	}
	require.Equal(t, "v", sess.Get("k"), "and what the abandoned handler holds is left as it was")
}

// The context of an abandoned request is reused for another request once the
// handlers that outlive it are done. A goroutine that still holds the session then
// must not write to the response of that request, or run its storage calls under
// its context; it can still save what it holds.
func Test_Session_ReleaseMiddleware_AbandonedSessionLetsGoOfItsContext(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer c.ForceRelease()

	store := NewStore()
	m := startMiddleware(t, c, store)
	sess := m.Session
	sess.Set("k", "v")

	c.Abandon()
	releaseMiddleware(m)
	require.Nil(t, sess.requestCtx(), "the session still points at the context of its request")

	require.NoError(t, sess.Save())
	require.Empty(t, c.Response().Header.Peek(fiber.HeaderSetCookie), "a save by a goroutine that still holds the session wrote to the response")
	stored, err := store.Storage.GetWithContext(context.Background(), store.storageKey(sess.ID()))
	require.NoError(t, err)
	require.NotEmpty(t, stored, "but the session was saved")
}

// The interleaving behind a timeout middleware: the abandoned handler reads the
// request local, the request is answered and the middleware in front of it
// finishes, the next request starts, and then the handler goes on with its lookup.
// It must not find the next request's middleware.
func Test_Session_Middleware_AbandonedLookupDoesNotFindTheNextRequest(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c1 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer c1.ForceRelease()
	c2 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c2)

	store := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	first := startMiddleware(t, c1, store)
	first.Set("req", "first")

	held := FromContext(c1) // the abandoned handler begins its lookup
	require.Same(t, first, held)

	c1.Abandon() // the timeout middleware abandons the request and answers it
	releaseMiddleware(first)

	second := startMiddleware(t, c2, store) // the next request starts
	second.Set("req", "secret-of-the-second-request")
	require.NotSame(t, first, second)

	found := held.find(func(m *Middleware) bool { return m.config.Store == store })
	require.Nil(t, found, "the abandoned handler found a middleware after its request ended")
	require.Nil(t, store.FromContext(c1))
	require.Equal(t, "secret-of-the-second-request", second.Get("req"))

	releaseMiddleware(second)
}

// The session of an abandoned request is cut off from its context while a goroutine
// that outlives the request may be reading it. Run with -race.
func Test_Session_ReleaseMiddleware_AbandonedSessionIsRaceFree(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore()

	for range 200 {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		m := startMiddleware(t, c, store)
		sess := m.Session
		c.Abandon()

		var wg sync.WaitGroup
		wg.Go(func() { _ = sess.resolveContext() })
		wg.Go(func() { _ = m.requestContext() })
		releaseMiddleware(m)
		wg.Wait()

		c.ForceRelease()
	}
}

// A walk stops at a middleware that finished, even if nothing but its flag says so.
func Test_Session_Middleware_FindStopsAtAFinishedOuter(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})
	outer := startMiddleware(t, c, storeA)
	inner := startMiddleware(t, c, storeB)

	require.Same(t, outer, storeA.FromContext(c))

	outer.released.Store(true)
	require.Nil(t, storeA.FromContext(c), "the walk went on to a middleware that finished")
	require.Same(t, inner, storeB.FromContext(c), "the middleware in front of it is still found")
	require.Nil(t, inner.find(func(m *Middleware) bool { return m == outer }), "even one a caller asks for by name")
	outer.released.Store(false)

	releaseMiddleware(inner)
	releaseMiddleware(outer)
}

// adopt, the restore and the release of the outer middleware run on different
// goroutines when a timeout middleware abandons the handlers inside it. They must
// not race on the middleware: run with -race.
func Test_Session_Middleware_LinkProtocolIsRaceFree(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})

	// outer is a running middleware of the request on c1, which a goroutine is
	// about to link to or restore. It is dropped instead of recycled when it
	// finishes, so that the test can look at it afterwards.
	run := func(t *testing.T, inner func(outer *Middleware, c1, c2 fiber.Ctx), beforeRelease func(outer *Middleware)) {
		t.Helper()

		for range 200 {
			c1 := app.AcquireCtx(&fasthttp.RequestCtx{})
			c2 := app.AcquireCtx(&fasthttp.RequestCtx{}) // restore writes the locals of the request it is given
			outer := startMiddleware(t, c1, store)
			outer.escaped = true

			var wg sync.WaitGroup
			wg.Go(func() { inner(outer, c1, c2) })
			beforeRelease(outer)
			releaseMiddleware(outer)
			wg.Wait()

			require.Zero(t, outer.children.Load(), "every link that was made was undone")
			app.ReleaseCtx(c1)
			app.ReleaseCtx(c2)
		}
	}

	t.Run("linking while the outer middleware finishes", func(t *testing.T) {
		t.Parallel()
		run(t, func(outer *Middleware, c1, c2 fiber.Ctx) {
			if outer.adopt(c1) {
				restoreMiddlewareContext(c2, outer)
				outer.unlink()
			}
		}, func(*Middleware) {})
	})

	t.Run("restoring while the outer middleware finishes", func(t *testing.T) {
		t.Parallel()

		// The link is made first; from then on the goroutine and the release run on their own.
		linked := make(chan bool, 1)
		run(t, func(outer *Middleware, c1, c2 fiber.Ctx) {
			ok := outer.adopt(c1)
			linked <- ok
			if ok {
				restoreMiddlewareContext(c2, outer)
				outer.unlink()
			}
		}, func(*Middleware) {
			require.True(t, <-linked, "the outer middleware was still running")
		})
	})
}

// A panic that unwinds a middleware inside another skips its release, so the link
// to the outer middleware has to be ended some other way, once.
func Test_Session_Middleware_UnlinkFromOuterIsOnce(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	outer := startMiddleware(t, c, NewStore(Config{Extractor: extractors.FromCookie("a_sid")}))
	inner := startMiddleware(t, c, NewStore(Config{Extractor: extractors.FromCookie("b_sid")}))
	require.True(t, inner.linked.Load())
	require.Equal(t, int32(1), outer.children.Load())

	inner.unlinkFromOuter()
	require.False(t, inner.linked.Load())
	require.Zero(t, outer.children.Load(), "the outer middleware no longer counts the one that was unwound")
	require.Same(t, outer, inner.parent, "the way to it stays, for whatever recovers the panic")

	inner.unlinkFromOuter()
	require.Zero(t, outer.children.Load(), "and a second call finds nothing to undo")

	// A release after that does not undo the link again.
	inner.escaped = true
	releaseMiddleware(inner)
	require.Zero(t, outer.children.Load())

	releaseMiddleware(outer)
	require.False(t, outer.released.Load(), "so the outer middleware is recycled like any other")
}

// recoverBetween is a middleware that recovers the panic of whatever runs behind it and
// reports what it can still reach of the session middleware at that point.
func recoverBetween(storeA *Store, seen *atomic.Value) fiber.Handler {
	return func(c fiber.Ctx) error {
		var err error
		func() {
			defer func() {
				if recover() != nil {
					seen.Store(fmt.Sprintf("outer=%t current=%t", storeA.FromContext(c) != nil, FromContext(c) != nil))
					err = fiber.ErrInternalServerError
				}
			}()
			err = c.Next()
		}()
		return err
	}
}

// A handler panics behind an inner session middleware and a middleware that
// recovers it, in front of an outer session middleware. As it does without an outer
// one, the inner middleware stays the current one for the recovering middleware, which
// can still reach the outer one; the outer one then saves its session as usual and the
// inner one, unwound by the panic, does not save.
func Test_Session_MultipleStores_PanicInsideKeepsWhatItShould(t *testing.T) {
	t.Parallel()

	recA, recB := newRecordingStorage(), newRecordingStorage()
	handlerA, storeA := NewWithStore(Config{Storage: recA, Extractor: extractors.FromCookie("a_sid")})
	handlerB, _ := NewWithStore(Config{Storage: recB, Extractor: extractors.FromCookie("b_sid")})

	var seen atomic.Value
	app := fiber.New()
	app.Use(handlerA)
	app.Use(recoverBetween(storeA, &seen))
	app.Use(handlerB)
	app.Get("/", func(c fiber.Ctx) error {
		storeA.FromContext(c).Set("k", "v")
		panic("boom")
	})

	resp, _ := sendRequest(t, app, "/", nil)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "outer=true current=true", seen.Load())
	require.Len(t, recA.writes(), 1, "the outer middleware saved its session")
	require.Empty(t, recB.writes(), "the inner one was unwound before it could")
}

// panickingStorage is a storage that reads but panics when it is asked to save.
type panickingStorage struct {
	fiber.Storage
}

func (panickingStorage) SetWithContext(context.Context, string, []byte, time.Duration) error {
	panic("storage panic")
}

func (panickingStorage) Set(string, []byte, time.Duration) error {
	panic("storage panic")
}

// Not parallel: it replaces the New of the shared middleware pool to count what the
// pool creates, and keeps the process on one P without garbage collection so that
// the pool hands back what it was given.
//
// The inner middleware is unwound by a panic and never comes back, with or
// without an outer one, so each request creates one. The outer middleware must not
// cost another: it is recycled, because nothing runs inside it any more. The panic
// can come from the rest of the stack, from the save of the inner middleware, or
// from its error handler.
func Test_Session_MultipleStores_PanicInsideRecyclesTheOuterMiddleware(t *testing.T) {
	prevGC := debug.SetGCPercent(-1)
	prevProcs := runtime.GOMAXPROCS(1)
	defer debug.SetGCPercent(prevGC)
	defer runtime.GOMAXPROCS(prevProcs)

	var created atomic.Int64
	oldNew := middlewarePool.New
	middlewarePool.New = func() any {
		created.Add(1)
		return &Middleware{}
	}
	defer func() { middlewarePool.New = oldNew }()

	for _, tc := range []struct {
		handler fiber.Handler
		name    string
		inner   Config
	}{
		{
			name:    "in the rest of the stack",
			inner:   Config{Extractor: extractors.FromCookie("b_sid")},
			handler: func(fiber.Ctx) error { panic("boom") },
		},
		{
			name:  "in the save of the inner middleware",
			inner: Config{Extractor: extractors.FromCookie("b_sid"), Storage: panickingStorage{Storage: memory.New()}},
			handler: func(c fiber.Ctx) error {
				FromContext(c).Set("k", "v")
				return nil
			},
		},
		{
			name: "in the error handler of the inner middleware",
			inner: Config{
				Extractor:    extractors.FromCookie("b_sid"),
				Storage:      unsavableStorage{Storage: memory.New()},
				ErrorHandler: func(fiber.Ctx, error) { panic("error handler") },
			},
			handler: func(c fiber.Ctx) error {
				FromContext(c).Set("k", "v")
				return nil
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handlerA, storeA := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
			handlerB, _ := NewWithStore(tc.inner)

			var seen atomic.Value
			app := fiber.New()
			app.Use(handlerA)
			app.Use(recoverBetween(storeA, &seen))
			app.Use(handlerB)
			app.Get("/", tc.handler)
			handler := app.Handler()

			do := func() int {
				fctx := &fasthttp.RequestCtx{}
				fctx.Request.Header.SetMethod(fiber.MethodGet)
				fctx.Request.SetRequestURI("/")
				handler(fctx)
				return fctx.Response.StatusCode()
			}

			require.Equal(t, fiber.StatusInternalServerError, do(), "warm-up")
			created.Store(0)

			const requests = 100
			for range requests {
				require.Equal(t, fiber.StatusInternalServerError, do())
			}
			// One per request for the inner middleware. The race detector makes a pool
			// drop some of what it is given, so the outer one is allowed a quarter more,
			// but not one more per request.
			require.Less(t, created.Load(), int64(requests)*8/5,
				"the outer middleware was dropped instead of recycled: %d created for %d requests", created.Load(), requests)
		})
	}
}

// Once a middleware finished it was released, and may already serve another
// request: the deferred call that ends the link of a middleware that was unwound
// must leave a finished one alone.
func Test_Session_Middleware_UnlinkUnlessFinished(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	outer := startMiddleware(t, c, NewStore(Config{Extractor: extractors.FromCookie("a_sid")}))
	inner := startMiddleware(t, c, NewStore(Config{Extractor: extractors.FromCookie("b_sid")}))
	require.Equal(t, int32(1), outer.children.Load())

	finished := true
	inner.unlinkUnlessFinished(&finished)
	require.Equal(t, int32(1), outer.children.Load(), "the link of a finished middleware was undone")
	require.True(t, inner.linked.Load())

	finished = false
	inner.unlinkUnlessFinished(&finished)
	require.Zero(t, outer.children.Load(), "the link of one that was unwound was not")

	inner.escaped = true
	releaseMiddleware(inner)
	releaseMiddleware(outer)
}

// panicOnClearCtx is a custom context whose Locals panics when it is asked to clear
// the session middleware, as the release of the middleware does while it holds its lock.
type panicOnClearCtx struct {
	armed *atomic.Bool
	fiber.DefaultCtx
}

func (c *panicOnClearCtx) Locals(key any, value ...any) any {
	if c.armed.Load() && len(value) == 1 && value[0] == nil && key == middlewareContextKey {
		panic("custom context: Locals panicked while clearing")
	}
	return c.DefaultCtx.Locals(key, value...)
}

// A panic inside the release of a nested middleware, which holds its lock, goes
// on as it does for a middleware without an outer one: the call that ends the link
// to the outer middleware must not wait for that lock.
func Test_Session_MultipleStores_PanicInReleaseDoesNotHang(t *testing.T) {
	t.Parallel()

	for _, nested := range []bool{false, true} {
		t.Run("nested="+strconv.FormatBool(nested), func(t *testing.T) {
			t.Parallel()

			armed := &atomic.Bool{}
			app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
				return &panicOnClearCtx{DefaultCtx: *fiber.NewDefaultCtx(app), armed: armed}
			})
			handlerA, _ := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
			handlerB, _ := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})
			if nested {
				app.Use(handlerA)
			}
			app.Use(handlerB)
			app.Get("/", func(fiber.Ctx) error {
				armed.Store(true) // the release of the middleware that runs last panics
				return nil
			})
			handler := app.Handler()

			recovered := make(chan any, 1)
			go func() {
				defer func() { recovered <- recover() }()
				fctx := &fasthttp.RequestCtx{}
				fctx.Request.Header.SetMethod(fiber.MethodGet)
				fctx.Request.SetRequestURI("/")
				handler(fctx)
			}()

			select {
			case got := <-recovered:
				require.NotNil(t, got, "the panic did not come out of the handler")
			case <-time.After(5 * time.Second):
				t.Fatal("the request hung: the panic in the release was swallowed by a deadlock")
			}
		})
	}
}
