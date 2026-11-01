package session

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// startMiddleware runs the beginning of the session handler for store on c: the middleware is
// acquired, loads its session and becomes the current middleware of the request. releaseMiddleware
// is its end.
func startMiddleware(t *testing.T, c fiber.Ctx, store *Store) *Middleware {
	t.Helper()

	cfg := store.Config
	cfg.Store = store

	m := acquireMiddleware()
	require.NoError(t, m.initialize(c, &cfg))
	return m
}

// middlewarePair is a session middleware over b_sid started inside one over a_sid on one request.
type middlewarePair struct {
	app            *fiber.App
	c              fiber.Ctx
	storeA, storeB *Store
	outer, inner   *Middleware
}

func startPair(t *testing.T) middlewarePair {
	t.Helper()

	p := middlewarePair{
		app:    fiber.New(),
		storeA: NewStore(Config{Extractor: extractors.FromCookie("a_sid")}),
		storeB: NewStore(Config{Extractor: extractors.FromCookie("b_sid")}),
	}
	p.c = newRequest(t, p.app)
	p.outer = startMiddleware(t, p.c, p.storeA)
	p.inner = startMiddleware(t, p.c, p.storeB)
	return p
}

// requireDetached waits until m and its session let go of the fiber.Ctx of the request, which goes
// back to the pool while a handler that outlives the request (one the timeout middleware gave up
// on, or a panic unwound) still holds them.
func requireDetached(t *testing.T, m *Middleware) {
	t.Helper()

	require.Eventually(t, func() bool {
		m.mu.RLock()
		sess, ctx := m.Session, m.ctx
		m.mu.RUnlock()
		return sess != nil && ctx == nil && sess.requestCtx() == nil
	}, 5*time.Second, 5*time.Millisecond, "the middleware and its session still refer to the request")
}

// holdsNoRequest reports whether the session writes to no request any more.
func (s *Session) holdsNoRequest() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, response := s.requestResponse()
	return response == nil
}

// storedFor returns what the storage of store holds for the session id.
func storedFor(t *testing.T, store *Store, id string) []byte {
	t.Helper()

	stored, err := store.Storage.GetWithContext(context.Background(), store.storageKey(id))
	require.NoError(t, err)
	return stored
}

// A middleware that runs inside another links to it and lets go when it finishes, and the outer one
// is the current middleware of the request again.
func Test_Session_Middleware_NestedLinkIsBalanced(t *testing.T) {
	t.Parallel()

	p := startPair(t)
	c, outer, inner := p.c, p.outer, p.inner
	require.Same(t, outer, inner.outer)
	require.Equal(t, int32(1), outer.links.Load())
	require.Same(t, inner, c.Locals(middlewareContextKey))

	// Dropped instead of recycled, so that the test can look at it afterwards without a parallel
	// test having taken it from the pool in the meantime.
	inner.mark(lifeEscaped)
	releaseMiddleware(inner)
	require.True(t, inner.is(lifeReleased))
	require.Nil(t, inner.outer, "a finished middleware keeps no link, so one the request context still holds cannot lead to an object that went back to the pool")
	require.Nil(t, inner.Session)
	require.Zero(t, outer.links.Load(), "and the outer one is not counted as having it inside any more")
	require.Same(t, outer, c.Locals(middlewareContextKey), "the outer middleware is current again")
	require.False(t, outer.is(lifeReleased))

	releaseMiddleware(outer)
	require.Nil(t, c.Locals(middlewareContextKey))
}

// A middleware links only to one that serves its own request: a Middleware is recycled, and an
// abandoned handler can come back to it after another request has taken it over.
func Test_Session_Middleware_LinkOnlyWithinItsRequest(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c1 := newRequest(t, app)
	c2 := newRequest(t, app)

	outer := startMiddleware(t, c1, NewStore(Config{Extractor: extractors.FromCookie("a_sid")}))

	require.False(t, outer.link(c2), "a middleware of another request is no outer one")
	require.Zero(t, outer.links.Load(), "and nothing was counted for the refusal")

	require.True(t, outer.link(c1))
	require.Equal(t, int32(1), outer.links.Load())
	outer.unlink()
	require.Zero(t, outer.links.Load())

	// Dropped instead of recycled, so that the test can look at it once it finished.
	outer.mark(lifeEscaped)
	releaseMiddleware(outer)
	require.True(t, outer.is(lifeReleased))
	require.False(t, outer.link(c1), "a middleware that finished is no outer one either")
	require.Zero(t, outer.links.Load())
}

// The shape of a request that a middleware abandoned, as sse does: the outer middleware finishes
// and its request is answered while the middleware inside it still runs. A walk from the inner
// middleware stops at the finished one, even if nothing but its flag says so. The outer one must
// not be recycled then, for the inner one holds it, and when the inner one finishes it must not
// hand the request an outer middleware that now belongs to somebody else.
func Test_Session_Middleware_OuterFinishesBeforeInner(t *testing.T) {
	t.Parallel()

	p := startPair(t)
	c1, outer, inner := p.c, p.outer, p.inner
	c2 := newRequest(t, p.app)

	// Nothing but its flag says that the outer middleware finished, and a walk stops there.
	require.Same(t, outer, p.storeA.FromContext(c1))
	outer.mark(lifeReleased)
	require.Nil(t, p.storeA.FromContext(c1), "the walk went on to a middleware that finished")
	require.Same(t, inner, p.storeB.FromContext(c1), "the middleware in front of it is still found")
	require.Nil(t, inner.find(func(m *Middleware) bool { return m == outer }), "even one a caller asks for by name")
	outer.unmark(lifeReleased)

	// The outer middleware finishes first.
	releaseMiddleware(outer)
	require.True(t, outer.is(lifeReleased), "an outer middleware with one still inside it is not recycled")
	require.Nil(t, c1.Locals(middlewareContextKey))
	require.Nil(t, p.storeA.FromContext(c1))
	require.Same(t, outer, inner.outer, "the inner middleware still holds what it was linked to")

	// Another request starts. Nothing the pool hands out is the old outer middleware.
	for range 100 {
		got := acquireMiddleware()
		require.NotSame(t, outer, got, "the outer middleware went back to the pool with a middleware inside it")
		releaseMiddleware(got)
	}
	other := startMiddleware(t, c2, p.storeA)
	require.NotSame(t, outer, other)
	otherSession := other.Session

	// The inner middleware finishes last.
	releaseMiddleware(inner)
	require.Nil(t, c1.Locals(middlewareContextKey), "the request got a finished middleware back")
	require.Empty(t, c1.Locals(sessionIDContextKey))
	require.Zero(t, outer.links.Load())

	require.Same(t, other, c2.Locals(middlewareContextKey), "the other request kept its own")
	require.Same(t, otherSession, other.Session)
	require.NotEmpty(t, other.Session.ID())
	releaseMiddleware(other)
}

// A timeout middleware around a session middleware, behind another one. The handlers inside the
// timeout outlive the answer. The outer middleware is not recycled while they may still use it, so
// the next request is served by another one before they finish. What the abandoned handler finds in
// the request afterwards may be the middleware of its own request, but must not be the session of
// the request that came after.
//
// The answer waits until the inner middleware has started, so that what the timeout leaves running
// is ordered after what it left behind: the race detector would otherwise trip over the shared
// request locals, which the timeout middleware leaves to the application.
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

// Finding a store's middleware, and telling that a session belongs to one, cost a request nothing:
// they walk the middleware without allocating.
//
// testing.AllocsPerRun panics in a parallel test, so this one stays sequential.
func Test_Session_Middleware_FindDoesNotAllocate(t *testing.T) {
	p := startPair(t)
	defer releaseMiddleware(p.outer)
	defer releaseMiddleware(p.inner)

	require.Same(t, p.inner, p.storeB.FromContext(p.c))
	require.Same(t, p.outer, p.storeA.FromContext(p.c))

	save := func(s *Session) func() {
		return func() {
			if err := s.Save(); err != nil {
				t.Error(err)
			}
		}
	}

	require.Zero(t, testing.AllocsPerRun(100, func() { _ = p.storeB.FromContext(p.c) }), "the innermost middleware")
	require.Zero(t, testing.AllocsPerRun(100, func() { _ = p.storeA.FromContext(p.c) }), "the one it runs inside")
	require.Zero(t, testing.AllocsPerRun(100, save(p.inner.Session)), "the session of the innermost middleware")
	require.Zero(t, testing.AllocsPerRun(100, save(p.outer.Session)), "the session of the one it runs inside")
}

// A middleware that nothing refers to is recycled when it finishes, with its session reset, and the
// outer middleware is not held by the one that ran inside it any more.
//
// Not parallel: it reads the middleware and the sessions after they went back to the shared pools,
// where a parallel test could take them in the meantime.
func Test_Session_ReleaseMiddleware_RecyclesWhatNothingRefersTo(t *testing.T) {
	p := startPair(t)
	outer, inner := p.outer, p.inner
	outerSession, innerSession := outer.Session, inner.Session
	require.NotEmpty(t, outerSession.ID())
	require.NotEmpty(t, innerSession.ID())

	releaseMiddleware(inner)
	require.False(t, inner.is(lifeReleased), "nothing refers to the inner middleware, so it is recycled")
	require.Nil(t, inner.Session)
	require.Empty(t, innerSession.ID(), "the inner session was reset and went back to the pool with its middleware")
	require.NotEmpty(t, outerSession.ID(), "and the outer one is still the outer middleware's")
	require.Zero(t, outer.links.Load())

	releaseMiddleware(outer)
	require.False(t, outer.is(lifeReleased), "nothing refers to the outer one either, since the inner one let go of it")
	require.Nil(t, outer.Session)
	require.Empty(t, outerSession.ID())
}

// A handler that a middleware abandoned, as sse does, can hold the middleware and the session of
// its request after the request is answered, the middleware in front of it finished and the next
// request started. Neither is recycled then, or the next request would find itself in the handler's
// hands. The context of the request is reused once the handlers that outlive it are done, so the
// session lets go of it: a goroutine that still holds the session neither writes to the response of
// that request nor runs its storage calls under its context, but can still save what it holds. And
// a lookup the handler began before the middleware finished must not find the middleware of the
// next request.
func Test_Session_ReleaseMiddleware_AbandonedRequestKeepsItsObjects(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer c.ForceRelease()

	store := NewStore()
	first := startMiddleware(t, c, store)
	sess := first.Session
	sess.Set("k", "v")
	held := FromContext(c) // the abandoned handler begins a lookup
	require.Same(t, first, held)

	c.Abandon()
	releaseMiddleware(first)
	require.True(t, first.is(lifeReleased), "the middleware is marked finished, so lookups stop returning it")
	require.Nil(t, first.Session)

	for range 100 {
		got := acquireMiddleware()
		require.NotSame(t, first, got, "the middleware of an abandoned request went back to the pool")
		releaseMiddleware(got)

		next := acquireSession()
		require.NotSame(t, sess, next, "the session of an abandoned request went back to the pool")
		releaseSession(next)
	}
	require.Equal(t, "v", sess.Get("k"), "and what the abandoned handler holds is left as it was")

	require.Nil(t, sess.requestCtx(), "the session still points at the context of its request")
	require.NoError(t, sess.Save())
	require.Empty(t, c.Response().Header.Peek(fiber.HeaderSetCookie), "a save by a goroutine that still holds the session wrote to the response")
	require.NotEmpty(t, storedFor(t, store, sess.ID()), "but the session was saved")

	second := startMiddleware(t, newRequest(t, app), store) // the next request starts
	second.Set("req", "secret-of-the-second-request")
	require.NotSame(t, first, second)

	found := held.find(func(m *Middleware) bool { return m.store == store })
	require.Nil(t, found, "the abandoned handler found a middleware after its request ended")
	require.Nil(t, store.FromContext(c))
	require.Equal(t, "secret-of-the-second-request", second.Get("req"))

	releaseMiddleware(second)
}

// The session of an abandoned request is cut off from its context while a goroutine that outlives
// the request may be reading it. Run with -race.
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

// Save and SaveWithContext read the context of the session under its lock, while the end of a
// request forgets it. Run with -race: it fails if either reads it bare. Whether the detector sees
// the bare read depends on how the goroutines interleave: on several CPUs it always does, on a
// single one it does in most runs, not in all.
func Test_Session_Save_EntryPointsReadTheContextUnderItsLock(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := newRequest(t, app)

	store := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	sess := mustGet(t, store, c)
	sess.Set("k", "v")

	stop := make(chan struct{})
	var writer sync.WaitGroup
	writer.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			sess.detach(nil)
			sess.mu.Lock()
			sess.ctx = c
			sess.mu.Unlock()
		}
	})

	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for range 200 {
				assert.NoError(t, sess.Save())
				assert.NoError(t, sess.SaveWithContext(context.Background()))
			}
		})
	}
	readers.Wait()
	close(stop)
	writer.Wait()
}

// contextSpyStorage records the context each save runs under.
type contextSpyStorage struct {
	fiber.Storage
	sets []context.Context
	mu   sync.Mutex
}

func (s *contextSpyStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.mu.Lock()
	s.sets = append(s.sets, ctx)
	s.mu.Unlock()
	return s.Storage.SetWithContext(ctx, key, val, exp)
}

// Save runs its storage call under the context of the request, SaveWithContext under the one it is
// given, and a session that belongs to no request falls back to the background context.
func Test_Session_Save_RunsUnderTheRequestContext(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := newRequest(t, app)

	spy := &contextSpyStorage{Storage: memory.New()}
	store := NewStore(Config{Storage: spy})
	sess := mustGet(t, store, c)
	sess.Set("k", "v")

	require.NoError(t, sess.Save())
	require.Len(t, spy.sets, 1)
	require.Same(t, c, spy.sets[0], "Save runs its storage call under the context of the request")

	type ctxKey struct{}
	given := context.WithValue(context.Background(), ctxKey{}, "given")
	require.NoError(t, sess.SaveWithContext(given))
	require.Len(t, spy.sets, 2)
	require.Same(t, given, spy.sets[1], "SaveWithContext runs it under the context it is given")

	byID, err := store.GetByID(context.Background(), sess.ID())
	require.NoError(t, err)
	defer byID.Release()
	byID.Set("k", "w")
	require.NoError(t, byID.Save())
	require.Len(t, spy.sets, 3)
	require.Equal(t, context.Background(), spy.sets[2], "a session without a request saves under the background context")
}

// The link, the restore and the release of the outer middleware run on different goroutines when a
// handler outlives its request. They must not race on the middleware: run with -race.
func Test_Session_Middleware_LinkProtocolIsRaceFree(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})

	for name, linkFirst := range map[string]bool{
		"linking while the outer middleware finishes": false,
		// The link is made first; from then on the goroutine and the release run on their own.
		"restoring while the outer middleware finishes": true,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			linked := make(chan bool, 1)
			for range 200 {
				c1 := app.AcquireCtx(&fasthttp.RequestCtx{})
				c2 := app.AcquireCtx(&fasthttp.RequestCtx{}) // restore writes the locals of the request it is given
				// outer runs on c1 and the goroutine is about to link to or restore it; it is
				// dropped instead of recycled, so that the test can look at it afterwards.
				outer := startMiddleware(t, c1, store)
				outer.mark(lifeEscaped)

				var wg sync.WaitGroup
				wg.Go(func() {
					ok := outer.link(c1)
					if linkFirst {
						linked <- ok
					}
					if ok {
						restoreMiddlewareContext(c2, outer)
						outer.unlink()
					}
				})
				if linkFirst {
					require.True(t, <-linked, "the outer middleware was still running")
				}
				releaseMiddleware(outer)
				wg.Wait()

				require.Zero(t, outer.links.Load(), "every link that was made was undone")
				app.ReleaseCtx(c1)
				app.ReleaseCtx(c2)
			}
		})
	}
}

// recoverBetween is a middleware that recovers the panic of whatever runs behind it and reports
// what it can still reach of the session middleware at that point.
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

// A handler panics behind an inner session middleware and a middleware that recovers it, in front
// of an outer session middleware. As it does without an outer one, the inner middleware stays the
// current one for the recovering middleware, which can still reach the outer one; the outer one
// then saves its session as usual and the inner one, unwound by the panic, does not save.
func Test_Session_MultipleStores_PanicInsideKeepsWhatItShould(t *testing.T) {
	t.Parallel()

	// With PassLocalsToContext the request context holds the inner middleware too; it is still the
	// recovering middleware's to reach through the request.
	for _, pass := range []bool{false, true} {
		t.Run(fmt.Sprintf("PassLocalsToContext=%t", pass), func(t *testing.T) {
			t.Parallel()

			recA, recB := newRecordingStorage(), newRecordingStorage()
			handlerA, storeA := NewWithStore(Config{Storage: recA, Extractor: extractors.FromCookie("a_sid")})
			handlerB, _ := NewWithStore(Config{Storage: recB, Extractor: extractors.FromCookie("b_sid")})

			var seen atomic.Value
			var inner atomic.Pointer[Middleware]
			app := fiber.New(fiber.Config{PassLocalsToContext: pass})
			app.Use(handlerA, recoverBetween(storeA, &seen), handlerB)
			app.Get("/", func(c fiber.Ctx) error {
				storeA.FromContext(c).Set("k", "v")
				inner.Store(FromContext(c))
				panic("boom")
			})

			resp, _ := sendRequest(t, app, "/", nil)
			require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
			require.Equal(t, "outer=true current=true", seen.Load())
			require.Len(t, recA.writes(), 1, "the outer middleware saved its session")
			require.Empty(t, recB.writes(), "the inner one was unwound before it could")

			// The outer middleware, which finished after it, ended the inner one's request.
			m := inner.Load()
			m.mu.RLock()
			sess := m.Session
			m.mu.RUnlock()
			require.NotNil(t, sess)
			require.True(t, sess.holdsNoRequest(), "the session of the inner middleware still writes to the request")
		})
	}
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

// Not parallel: it replaces the New of the shared middleware pool to count what the pool creates,
// and keeps the process on one P without garbage collection so that the pool hands back what it was
// given.
//
// The inner middleware is unwound by a panic and never comes back, with or without an outer one, so
// each request creates one. The outer middleware must not cost another: it is recycled, because
// nothing runs inside it any more. The panic can come from the rest of the stack, from the save of
// the inner middleware, or from its error handler.
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
			app.Use(handlerA, recoverBetween(storeA, &seen), handlerB)
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
			// One per request for the inner middleware. The race detector makes a pool drop some of
			// what it is given, so the outer one is allowed a quarter more, but not one more per
			// request.
			require.Less(t, created.Load(), int64(requests)*8/5,
				"the outer middleware was dropped instead of recycled: %d created for %d requests", created.Load(), requests)
		})
	}
}

// A panic that unwinds a middleware inside another skips its release, so the deferred call that
// ends the middleware does what the release would. Once a middleware finished it was released, and
// may already serve another request: the call leaves a finished one alone. One that was unwound
// gives the outer middleware its count back, once, and lets go of the request, and keeps its
// session; the way to the outer middleware stays, for whatever recovers the panic.
func Test_Session_Middleware_EndIfUnwoundUnlinksOnce(t *testing.T) {
	t.Parallel()

	p := startPair(t)
	c, outer, inner := p.c, p.outer, p.inner
	require.Equal(t, int32(1), outer.links.Load())

	finished := true
	inner.endIfUnwound(&finished, c)
	require.Equal(t, int32(1), outer.links.Load(), "the count of a finished middleware was given back")
	require.Same(t, outer, inner.linked)
	require.NotNil(t, inner.ctx, "a finished middleware was cut off")

	finished = false
	inner.endIfUnwound(&finished, c)
	require.Nil(t, inner.linked)
	require.Zero(t, outer.links.Load(), "the count of one that was unwound was not")
	require.Same(t, outer, inner.outer, "the way to the outer middleware stays")
	require.Nil(t, inner.ctx, "the middleware that was unwound still refers to the fiber.Ctx")
	require.NotNil(t, inner.Session, "its session was let go of")
	require.Nil(t, inner.Session.requestCtx(), "its session still refers to the fiber.Ctx")
	require.Same(t, c.RequestCtx(), inner.Session.rctx, "its session keeps the request itself until the request ends")
	require.False(t, inner.is(lifeReleased), "it stays reachable for whatever recovers the panic")
	require.True(t, inner.is(lifeUnwound), "and is marked unwound, for a context kept past the request")
	c.RequestCtx().ResetUserValues() // what fasthttp does when it resets the request: it closes the middleware
	require.Nil(t, inner.Session.rctx, "the end of the request did not cut the session off from it")

	inner.unlinkFromOuter()
	require.Zero(t, outer.links.Load(), "a second call finds nothing to undo")

	// A release after that does not undo the link again.
	inner.mark(lifeEscaped)
	releaseMiddleware(inner)
	require.Zero(t, outer.links.Load())

	releaseMiddleware(outer)
}

// panicAtCtx is a custom context that panics at one chosen point of the life of a session
// middleware, as a Ctx of an application could: as the middleware starts (publishing it to the
// request) and as it ends (clearing the request locals, which it does holding its lock; replacing
// the Go context; asking whether the request was abandoned; making the outer middleware current
// again).
type panicAtCtx struct {
	point *atomic.Value // string; unset: nowhere
	fiber.DefaultCtx
}

func (c *panicAtCtx) Locals(key any, value ...any) any {
	if key == middlewareContextKey && len(value) == 1 {
		storing := value[0] != nil
		switch c.point.Load() {
		case "publishing the inner middleware", "restoring the outer middleware":
			if storing {
				panic("custom context: Locals panicked while storing the middleware")
			}
		case "clearing the locals":
			if !storing {
				panic("custom context: Locals panicked while clearing")
			}
		default:
		}
	}
	return c.DefaultCtx.Locals(key, value...)
}

func (c *panicAtCtx) SetContext(ctx context.Context) {
	if c.point.Load() == "replacing the Go context" {
		panic("custom context: SetContext panicked")
	}
	c.DefaultCtx.SetContext(ctx)
}

func (c *panicAtCtx) IsAbandoned() bool {
	if c.point.Load() == "asking whether the request was abandoned" {
		panic("custom context: IsAbandoned panicked")
	}
	return c.DefaultCtx.IsAbandoned()
}

// A panic in the start or in the release of a nested middleware, wherever it comes from, goes on as
// it does for a middleware without an outer one: nothing waits for the lock the release holds, and
// the outer middleware's count is given back, or was never taken, so that it is recycled when it
// finishes.
func Test_Session_MultipleStores_PanicInReleaseGoesOn(t *testing.T) {
	t.Parallel()

	for _, point := range []string{
		"publishing the inner middleware",
		"clearing the locals",
		"replacing the Go context",
		"asking whether the request was abandoned",
		"restoring the outer middleware",
	} {
		for _, nested := range []bool{false, true} {
			for _, pass := range []bool{false, true} {
				// Without an outer middleware there is none to restore or to run inside, and
				// without PassLocalsToContext the release leaves the Go context alone.
				if (!nested && (point == "restoring the outer middleware" || point == "publishing the inner middleware")) ||
					(!pass && point == "replacing the Go context") {
					continue
				}
				t.Run(fmt.Sprintf("%s nested=%t pass=%t", point, nested, pass), func(t *testing.T) {
					t.Parallel()

					armed := &atomic.Value{}
					app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
						return &panicAtCtx{DefaultCtx: *fiber.NewDefaultCtx(app), point: armed}
					}, fiber.Config{PassLocalsToContext: pass})
					handlerA, storeA := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
					handlerB, _ := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})
					if nested {
						app.Use(handlerA)
					}
					var outer atomic.Pointer[Middleware]
					app.Use(func(c fiber.Ctx) error { // between the two session middleware
						if nested {
							outer.Store(storeA.FromContext(c))
						}
						if point == "publishing the inner middleware" {
							armed.Store(point) // the start of the middleware that runs last panics
						}
						return c.Next()
					}, handlerB)
					app.Get("/", func(fiber.Ctx) error {
						if point != "publishing the inner middleware" {
							armed.Store(point) // the release of the middleware that runs last panics
						}
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
					if nested {
						require.NotNil(t, outer.Load())
						require.Zero(t, outer.Load().links.Load(), "the outer middleware still counts the one that panicked")
					}
				})
			}
		}
	}
}

// observeRestoreCtx is a custom context that tells when a session middleware makes the one it ran
// inside the current middleware of the request again.
type observeRestoreCtx struct {
	onRestore func()
	fiber.DefaultCtx
}

func (c *observeRestoreCtx) Locals(key any, value ...any) any {
	if c.onRestore != nil && key == middlewareContextKey && len(value) == 1 && value[0] != nil {
		c.onRestore()
	}
	return c.DefaultCtx.Locals(key, value...)
}

// The outer middleware stays counted until the inner one is done with it, including the moment it
// makes it current again: that is the moment a handler that a timeout middleware abandoned could
// otherwise find it finished and recycled.
func Test_Session_Middleware_OuterStaysLinkedWhileItIsRestored(t *testing.T) {
	t.Parallel()

	var (
		outer atomic.Pointer[Middleware]
		armed atomic.Bool
		seen  atomic.Int32
		after atomic.Int32
	)
	seen.Store(-1)
	after.Store(-1)

	app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
		return &observeRestoreCtx{DefaultCtx: *fiber.NewDefaultCtx(app), onRestore: func() {
			if armed.Load() {
				seen.Store(outer.Load().links.Load())
			}
		}}
	})
	handlerA, storeA := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
	handlerB, _ := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})
	app.Use(handlerA, func(c fiber.Ctx) error {
		err := c.Next() // the inner middleware starts and finishes in here
		// The outer middleware still runs, so it is still the request's own.
		after.Store(storeA.FromContext(c).links.Load())
		return err
	}, handlerB)
	app.Get("/", func(c fiber.Ctx) error {
		outer.Store(storeA.FromContext(c))
		armed.Store(true)
		return nil
	})

	resp, _ := sendRequest(t, app, "/", nil)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, int32(1), seen.Load(), "the outer middleware was given back before it was made current again")
	require.Zero(t, after.Load(), "and it was given back by the time the inner middleware was done")
}

// A handler that outlives its request calls Destroy, Reset or Regenerate while the request ends:
// the read of the middleware's context in those methods must not race with the release. Run with
// -race.
func Test_Session_Middleware_DestroyResetRegenerateRaceTheRelease(t *testing.T) {
	t.Parallel()

	for name, call := range map[string]func(m *Middleware) error{
		"Destroy":    (*Middleware).Destroy,
		"Reset":      (*Middleware).Reset,
		"Regenerate": (*Middleware).Regenerate,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			store := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
			for range 200 {
				c := app.AcquireCtx(&fasthttp.RequestCtx{})
				m := startMiddleware(t, c, store)
				m.mark(lifeEscaped) // dropped, not recycled: the goroutine below still holds it

				var wg sync.WaitGroup
				wg.Go(func() {
					// A finished middleware has no session: the call panics.
					defer func() { recover() }() //nolint:errcheck,revive // only the race matters here, not how the call ends
					if err := call(m); err != nil {
						t.Error(err)
					}
				})
				releaseMiddleware(m)
				wg.Wait()

				app.ReleaseCtx(c)
			}
		})
	}
}
