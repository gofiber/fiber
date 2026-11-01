package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// doRequest issues a GET with the given cookies and returns the response and its body. It reports
// failures as an error so goroutines can use it.
func doRequest(app *fiber.App, target string, cookies map[string]string) (*http.Response, string, error) {
	req := httptest.NewRequest(fiber.MethodGet, target, http.NoBody)
	for name, value := range cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	resp, err := app.Test(req)
	if err != nil {
		return nil, "", fmt.Errorf("send %s: %w", target, err)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", target, err)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, "", fmt.Errorf("close %s: %w", target, err)
	}
	return resp, string(raw), nil
}

// sendRequest is doRequest for the test goroutine: a failure ends the test.
//
//nolint:gocritic // the two results are of different types, and names would only repeat what every caller calls them
func sendRequest(t *testing.T, app *fiber.App, target string, cookies map[string]string) (*http.Response, string) {
	t.Helper()
	resp, body, err := doRequest(app, target, cookies)
	require.NoError(t, err)
	return resp, body
}

// cookiesByName indexes the Set-Cookie headers of a response by cookie name.
func cookiesByName(resp *http.Response) map[string]*http.Cookie {
	out := make(map[string]*http.Cookie)
	for _, c := range resp.Cookies() {
		out[c.Name] = c
	}
	return out
}

// useCookieStore puts a session middleware over the cookie name on app, with the Config given
// besides its Extractor, and returns its store.
func useCookieStore(app *fiber.App, name string, config ...Config) *Store {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	cfg.Extractor = extractors.FromCookie(name)
	handler, store := NewWithStore(cfg)
	app.Use(handler)
	return store
}

// responseCookie returns the value of the cookie that the response of c sets under name.
func responseCookie(t *testing.T, c fiber.Ctx, name string) string {
	t.Helper()
	cookie := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(cookie)
	cookie.SetKey(name)
	require.True(t, c.Response().Header.Cookie(cookie), "the response sets no cookie %q", name)
	return string(cookie.Value())
}

// saveNew asks store for its session in c, which must be a new one, and saves it with some data.
func saveNew(t *testing.T, c fiber.Ctx, store *Store) *Session {
	t.Helper()
	sess := mustGet(t, store, c)
	require.True(t, sess.Fresh())
	sess.Set("k", store.Extractor.Key)
	require.NoError(t, sess.Save())
	return sess
}

// setInSessions is a handler that sets key to value in the session of the middleware of each store.
func setInSessions(key, value string, stores ...*Store) fiber.Handler {
	return func(c fiber.Ctx) error {
		for _, store := range stores {
			store.FromContext(c).Set(key, value)
		}
		return nil
	}
}

// Two session middleware, each over its own cookie, keep two sessions apart in one request. The
// second used to look up the first one's cached ID: its session never survived a request, or, over
// a shared backend, aliased the first session's record.
func Test_Session_MultipleStores_Middleware(t *testing.T) {
	t.Parallel()

	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared storage=%v", shared), func(t *testing.T) {
			t.Parallel()

			var storageA, storageB fiber.Storage = memory.New(), memory.New()
			if shared {
				storageB = storageA
			}
			app := fiber.New()
			storeA := useCookieStore(app, "a_sid", Config{Storage: storageA})
			storeB := useCookieStore(app, "b_sid", Config{Storage: storageB})
			app.Get("/set", func(c fiber.Ctx) error {
				storeA.FromContext(c).Set("who", "alice")
				storeB.FromContext(c).Set("lang", "en")
				return c.SendStatus(fiber.StatusOK)
			})
			app.Get("/get", func(c fiber.Ctx) error {
				a, b := storeA.FromContext(c), storeB.FromContext(c)
				return c.SendString(fmt.Sprintf("%v|%v|%v|%v|%v|%v|%s|%s",
					a.Get("who"), a.Get("lang"), b.Get("lang"), b.Get("who"), a.Fresh(), b.Fresh(), a.ID(), b.ID()))
			})

			resp, _ := sendRequest(t, app, "/set", nil)
			cookies := cookiesByName(resp)
			require.Contains(t, cookies, "a_sid")
			require.Contains(t, cookies, "b_sid")
			idA, idB := cookies["a_sid"].Value, cookies["b_sid"].Value
			require.NotEqual(t, idA, idB, "each store issues its own ID")

			_, body := sendRequest(t, app, "/get", map[string]string{"a_sid": idA, "b_sid": idB})
			require.Equal(t, fmt.Sprintf("alice|<nil>|en|<nil>|false|false|%s|%s", idA, idB), body,
				"both sessions load again, keep their own data and see none of the other's")

			// Each record is stored once, under its own ID.
			rawA, err := storageA.Get(sessionKey(idA))
			require.NoError(t, err)
			require.NotNil(t, rawA)
			rawB, err := storageB.Get(sessionKey(idB))
			require.NoError(t, err)
			require.NotNil(t, rawB)
		})
	}
}

// The package-level FromContext keeps meaning "the innermost middleware"; the store's own
// FromContext finds a particular one however deep it is nested.
func Test_Session_MultipleStores_FromContext(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	storeA, storeB := useCookieStore(app, "a_sid"), useCookieStore(app, "b_sid")
	storeC := NewStore(Config{Extractor: extractors.FromCookie("c_sid")})
	app.Get("/", func(c fiber.Ctx) error {
		innermost := FromContext(c)
		a, b := storeA.FromContext(c), storeB.FromContext(c)
		return c.SendString(fmt.Sprint(
			innermost != nil && innermost.Store() == storeB,
			a != nil && a.Store() == storeA,
			b != nil && b == innermost,
			storeC.FromContext(c) == nil,
			storeA.FromContext(c.RequestCtx()) == a,
		))
	})

	_, body := sendRequest(t, app, "/", nil)
	require.Equal(t, "true true true true true", body)

	t.Run("outside the request", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, storeA.FromContext(nil))
		require.Nil(t, storeA.FromContext(newRequest(t, app)), "no middleware ran for this context")
	})

	t.Run("one middleware", func(t *testing.T) {
		t.Parallel()
		handler, store := NewWithStore()
		single := fiber.New()
		single.Use(handler)
		single.Get("/", func(c fiber.Ctx) error {
			return c.SendString(fmt.Sprint(store.FromContext(c) == FromContext(c), FromContext(c) != nil))
		})
		_, body := sendRequest(t, single, "/", nil)
		require.Equal(t, "true true", body)
	})
}

// A middleware takes the session that a store over its cookie saved through Store.Get before it
// ran: the CSRF middleware's store and the session middleware's own are two Store values like this,
// and a visitor's first request must end up with one session, not two. It does so whatever ran in
// between: a store over another cookie, or a middleware over another cookie.
func Test_Session_MultipleStores_MiddlewareFindsTheSessionStoreGetSaved(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		want string // what the handler sees of the session, and of the middleware over b_sid if there is one
		// between asks a store over b_sid after the save, outer runs a middleware over b_sid in front of the one under test
		between, outer bool
	}{
		{name: "the same cookie", want: "t1|false|-"},
		{name: "another store asked in between", want: "t1|false|-", between: true},
		{name: "behind a middleware over another cookie", want: "t1|false|<nil>", outer: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			storage := memory.New()
			early := NewStore(Config{Extractor: extractors.FromCookie("a_sid"), Storage: storage})
			handler, late := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid"), Storage: storage})
			require.NotSame(t, early, late)
			handlerB, storeB := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})

			var earlyID atomic.Value
			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				sess, err := early.Get(c)
				if err != nil {
					return err
				}
				defer sess.Release()
				sess.Set("token", "t1")
				if saveErr := sess.Save(); saveErr != nil {
					return saveErr
				}
				earlyID.Store(sess.ID())
				if tc.between {
					between, getErr := storeB.Get(c)
					if getErr != nil {
						return getErr
					}
					between.Release()
				}
				return c.Next()
			})
			if tc.outer {
				app.Use(handlerB)
			}
			app.Use(handler)
			app.Get("/", func(c fiber.Ctx) error {
				m, outer := late.FromContext(c), "-"
				if o := storeB.FromContext(c); o != nil {
					outer = fmt.Sprint(o.Get("token"))
				}
				return c.SendString(fmt.Sprintf("%v|%v|%s", m.Get("token"), m.Fresh(), outer))
			})

			resp, body := sendRequest(t, app, "/", nil)
			require.Equal(t, tc.want, body, "the middleware did not find the session saved before it")
			cookies := cookiesByName(resp)
			require.Contains(t, cookies, "a_sid")
			require.Equal(t, earlyID.Load(), cookies["a_sid"].Value, "the cookie carries the one ID both stores used")
			keys, err := storage.Keys()
			require.NoError(t, err)
			require.Len(t, keys, 1, "one visitor, one stored session")
		})
	}
}

// A middleware finds the session its own cookie names. It takes neither the ID of a middleware over
// another cookie in front of it, nor an ID that names no session: one that a Store.Get before it
// cached and never saved, though that store shares a cookie with it (it keeps its sessions in a
// storage of its own, which never saw the session), or the new one of the middleware it runs
// inside.
func Test_Session_MultipleStores_MiddlewarePassesOverAnIDThatNamesNoSession(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		early  string               // the cookie of a store asked through Store.Get first, if any
		front  string               // the cookie of the session middleware it runs inside
		cookie string               // the cookie of the request that names its session
		late   extractors.Extractor // what the middleware under test reads
	}{
		{
			name: "the early store reads the cookie that names the session", early: "b_sid", front: "a_sid",
			late: extractors.FromCookie("b_sid"), cookie: "b_sid",
		},
		{
			name: "the early store reads the first cookie of a chain", early: "s2a_sid", front: "s1_sid",
			late:   extractors.Chain(extractors.FromCookie("s2a_sid"), extractors.FromCookie("s2b_sid")),
			cookie: "s2b_sid",
		},
		{
			name: "the middleware around it reads the first cookie of a chain", front: "a_sid",
			late:   extractors.Chain(extractors.FromCookie("a_sid"), extractors.FromCookie("b_sid")),
			cookie: "b_sid",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			shared := memory.New()
			frontHandler, _ := NewWithStore(Config{Storage: shared, Extractor: extractors.FromCookie(tc.front)})
			lateHandler, late := NewWithStore(Config{Storage: shared, Extractor: tc.late})

			app := fiber.New()
			if tc.early != "" {
				early := NewStore(Config{Extractor: extractors.FromCookie(tc.early)})
				app.Use(func(c fiber.Ctx) error {
					sess, err := early.Get(c) // new: it caches an ID, which is never saved
					if err != nil {
						return err
					}
					sess.Release()
					return c.Next()
				})
			}
			app.Use(frontHandler, lateHandler)
			app.Get("/read", func(c fiber.Ctx) error {
				m := late.FromContext(c)
				return c.SendString(fmt.Sprintf("%v|%t|%s", m.Get("who"), m.Fresh(), m.ID()))
			})

			id := seedSession(t, app, late, func(s *Session) { s.Set("who", "late") })
			resp, body := sendRequest(t, app, "/read", map[string]string{tc.cookie: id})
			require.Equal(t, "late|false|"+id, body, "the middleware found its own session; it did not take the ID that names none")
			cookies := cookiesByName(resp)
			require.Contains(t, cookies, tc.cookie)
			require.Equal(t, id, cookies[tc.cookie].Value)
		})
	}
}

// Stores that share only some of their sources: the inner middleware reads a_sid and b_sid, the one
// it runs inside b_sid, and a store over a_sid saved a session for the request before either ran.
// The inner middleware takes the session of the middleware it runs inside when that ID names one,
// and passes it over for the session the store saved when it does not.
func Test_Session_MultipleStores_NearestMiddlewareFirst(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		want   string
		seeded bool // the visitor holds a b_sid that names a session
	}{
		{name: "the ID of the outer middleware names no session", want: "t1"},
		{name: "the ID of the outer middleware names a session", want: "tB", seeded: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			storage := memory.New()
			early := NewStore(Config{Extractor: extractors.FromCookie("a_sid"), Storage: storage})
			outerHandler, outerStore := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid"), Storage: storage})
			innerHandler, inner := NewWithStore(Config{
				Extractor: extractors.Chain(extractors.FromCookie("a_sid"), extractors.FromCookie("b_sid")),
				Storage:   storage,
			})

			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				sess, err := early.Get(c)
				if err != nil {
					return err
				}
				defer sess.Release()
				sess.Set("token", "t1")
				if saveErr := sess.Save(); saveErr != nil {
					return saveErr
				}
				return c.Next()
			})
			app.Use(outerHandler, innerHandler)
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString(fmt.Sprint(inner.FromContext(c).Get("token")))
			})

			var cookies map[string]string
			if tc.seeded {
				id := seedSession(t, app, outerStore, func(sess *Session) { sess.Set("token", "tB") })
				cookies = map[string]string{"b_sid": id}
			}
			_, body := sendRequest(t, app, "/", cookies)
			require.Equal(t, tc.want, body)
		})
	}
}

// The IDs a request keeps are a list that fasthttp closes from its head: caching again for a store
// keeps its entry in the list, and closing the list gives every entry back to the pool, cleared, so
// that a pooled entry holds no store and no ID of a finished request.
//
// Not parallel: a parallel test could take the entries from the shared pool between the close and
// the checks.
func Test_Session_CachedIDs_StayWholeAndGoBackToThePoolCleared(t *testing.T) {
	c := newRequest(t, fiber.New())
	a, b := NewStore(), NewStore()

	cacheID(c, nil, a, "1", extractors.Result{})
	first := requestIDs(c)
	cacheID(c, first, b, "2", extractors.Result{})
	cacheID(c, requestIDs(c), a, "3", extractors.Result{})

	require.Same(t, first, requestIDs(c), "the entry a store had is its entry again")
	second := first.next
	require.Equal(t, "3", first.id)
	require.Same(t, b, second.store)
	require.Nil(t, second.next)

	require.NoError(t, first.Close())
	require.Equal(t, cachedID{}, *first)
	require.Equal(t, cachedID{}, *second)

	// A pool decides for itself whether Get returns what Put was given, and under -race drops one
	// Put in four: closing gives an entry back if one of many tries gets it again.
	for range 64 {
		entry := &cachedID{id: "x"}
		require.NoError(t, entry.Close())
		if cachedIDPool.Get() == any(entry) {
			return
		}
	}
	t.Fatal("closing the list did not give its entries back to the pool")
}

// The provenance cached on the request belongs to the store that resolved it. Store A saves a new
// session and so caches its ID; store B then finds its own session through another cookie, or
// starts one. A's next Get takes the ID it cached and finds the record it saved, but must not take
// B's provenance with it: A's session would be written back to B's cookie, replacing the ID of B's
// session. A session of B that was found and not saved writes no cookie at all.
func Test_Session_MultipleStores_CachedProvenanceBelongsToItsStore(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})
	idB := seedSession(t, app, storeB, func(s *Session) { s.Set("n", 1) })

	for _, tc := range []struct {
		name      string
		returning bool // B finds its own session through its cookie
		saveB     bool
	}{
		{name: "B starts a session", saveB: true},
		{name: "B finds its session", returning: true, saveB: true},
		{name: "B finds its session and does not save it", returning: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := newRequest(t, app)
			if tc.returning {
				c.Request().Header.SetCookie("b_sid", idB)
			}
			first := mustGet(t, storeA, c)
			first.Set("n", 1)
			require.NoError(t, first.Save())
			b := mustGet(t, storeB, c)
			again := mustGet(t, storeA, c)
			require.False(t, again.Fresh(), "A found the record it saved")
			require.Equal(t, 1, again.Get("n"))
			again.Set("n", 2)

			// A last: were its session tied to B's cookie, the later header would win.
			if tc.saveB {
				require.NoError(t, b.Save())
			}
			require.NoError(t, again.Save())
			require.Equal(t, !tc.returning, b.Fresh())
			require.Equal(t, first.ID(), again.ID(), "A found the session it saved")
			require.Equal(t, again.ID(), responseCookie(t, c, "a_sid"), "the cookie carries the one session of A")
			if tc.saveB {
				require.Equal(t, b.ID(), responseCookie(t, c, "b_sid"), "B's cookie carries B's session and nothing else")
			} else {
				require.Empty(t, c.Response().Header.PeekCookie("b_sid"), "B's session was found and not saved, so B's cookie is not written")
			}
			if tc.returning {
				require.Equal(t, idB, b.ID(), "B loaded its own session")
			}
		})
	}
}

// A store that takes the ID another store cached takes the provenance cached with it as well, as it
// holds for the store: its session goes back to the cookie the ID came from, not to every sink of
// the chain, and never to a header it does not read, nor is the ID promoted into its cookie, since
// a visitor's session is not written to a place the visitor did not send it from.
func Test_Session_MultipleStores_SharingStoreAdoptsCachedProvenance(t *testing.T) {
	t.Parallel()

	chain := func() extractors.Extractor {
		return extractors.Chain(extractors.FromCookie("sid"), extractors.FromHeader("X-Sid"))
	}
	for _, tc := range []struct {
		request   func(*fasthttp.RequestHeader)
		name      string
		from      string               // where the first store's session goes back to
		second    extractors.Extractor // what the store asked second reads; the first reads the chain
		keepsSink bool                 // whether that is a sink of the second store too
	}{
		{
			name: "the same sources", second: chain(), from: "sid", keepsSink: true,
			request: func(h *fasthttp.RequestHeader) { h.SetCookie("sid", "unknown") },
		},
		{
			name: "fewer sources", second: extractors.FromCookie("sid"), from: "X-Sid",
			request: func(h *fasthttp.RequestHeader) { h.Set("X-Sid", "unknown") },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			shared := memory.New()
			first := NewStore(Config{Storage: shared, Extractor: chain()})
			second := NewStore(Config{Storage: shared, Extractor: tc.second})
			require.NotSame(t, first, second)

			c := newRequest(t, fiber.New())
			tc.request(&c.Request().Header)

			p := mustGet(t, first, c)
			require.True(t, p.Fresh(), "the request names no session, so the first store starts one and caches its ID")
			require.Equal(t, tc.from, p.extractor.Key, "the new session goes back to where the request named it")
			require.NoError(t, p.Save())

			q := mustGet(t, second, c)
			require.Equal(t, p.ID(), q.ID(), "the second store takes the cached ID")
			require.False(t, q.Fresh(), "and finds the session the first one saved")
			provenance := p.extractor
			provenance.Resolved = tc.keepsSink
			require.Equal(t, provenance, q.extractor, "and the provenance cached with it, as a sink of its own or not")

			// The first store's save wrote the ID into this response as well: clear it, so that
			// what is left is what the second store writes.
			c.Response().Header.DelCookie("sid")
			c.Response().Header.Del("X-Sid")
			require.NoError(t, q.Save())
			require.Empty(t, c.Response().Header.Peek("X-Sid"), "a header the ID did not come from, or the store does not read, is not written")
			if tc.keepsSink {
				require.Equal(t, q.ID(), responseCookie(t, c, "sid"), "the ID goes back to the cookie it came from")
			} else {
				require.Empty(t, c.Response().Header.PeekCookie("sid"), "and the ID is not promoted into the cookie")
			}
		})
	}
}

// A middleware over another cookie between two that share a source does not come between them: the
// inner one takes the session the outer one found through the cookie that only the outer one reads,
// though the one between shares nothing with either, and writes it to no cookie of the outer.
func Test_Session_MultipleStores_UnrelatedMiddlewareBetweenSharingOnesIsPassedOver(t *testing.T) {
	t.Parallel()

	storage := memory.New()
	outerHandler, outerStore := NewWithStore(Config{
		Storage:   storage,
		Extractor: extractors.Chain(extractors.FromCookie("sid"), extractors.FromCookie("shared")),
	})
	betweenHandler, betweenStore := NewWithStore(Config{Extractor: extractors.FromCookie("between_sid")})
	innerHandler, innerStore := NewWithStore(Config{Storage: storage, Extractor: extractors.FromCookie("shared")})

	app := fiber.New()
	app.Use(outerHandler, betweenHandler, innerHandler)
	app.Get("/read", func(c fiber.Ctx) error {
		inner, between := innerStore.FromContext(c), betweenStore.FromContext(c)
		return c.SendString(fmt.Sprintf("%v|%t|%s|%t", inner.Get("who"), inner.Fresh(), inner.ID(), between.Fresh()))
	})

	id := seedSession(t, app, outerStore, func(s *Session) { s.Set("who", "outer") })
	resp, body := sendRequest(t, app, "/read", map[string]string{"sid": id})
	require.Equal(t, "outer|false|"+id+"|true", body, "the middleware between them hid the session of the outer one from the inner one")
	cookies := cookiesByName(resp)
	require.NotContains(t, cookies, "shared", "the inner store wrote the ID it took to a cookie the client did not send")
	require.Equal(t, id, cookies["sid"].Value)
}

// Save leaves a session to the middleware that manages it, and only that one: a session no
// middleware of the request manages is saved, even while the middleware of another store is the
// current one.
func Test_Session_MultipleStores_SaveOfASessionNoMiddlewareManages(t *testing.T) {
	t.Parallel()

	c := newRequest(t, fiber.New())
	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})

	early := mustGet(t, storeA, c)
	early.Set("k", "v")

	m := startMiddleware(t, c, storeB)
	defer releaseMiddleware(m)

	require.NoError(t, early.Save())
	stored, err := storeA.Storage.GetWithContext(context.Background(), storeA.storageKey(early.ID()))
	require.NoError(t, err)
	require.NotEmpty(t, stored, "the session was saved")
}

// The provenance of one store's ID must never reach another store: an ID read from a read-only
// source is not written to a cookie, and a store never writes its ID to a cookie it does not own.
// Both are what session fixation relies on.
func Test_Session_MultipleStores_ProvenanceStaysWithItsStore(t *testing.T) {
	t.Parallel()

	storage := memory.New()
	handlerA, storeA := NewWithStore(Config{
		Storage:   storage,
		Extractor: extractors.Chain(extractors.FromQuery("sid"), extractors.FromCookie("a_sid")),
	})
	handlerB, storeB := NewWithStore(Config{Storage: storage, Extractor: extractors.FromCookie("b_sid")})

	app := fiber.New()
	app.Use(handlerA, handlerB)
	app.Get("/seed", func(c fiber.Ctx) error {
		storeA.FromContext(c).Set("who", "a")
		storeB.FromContext(c).Set("who", "b")
		return c.SendString(storeA.FromContext(c).ID() + "|" + storeB.FromContext(c).ID())
	})
	app.Get("/read", func(c fiber.Ctx) error {
		a, b := storeA.FromContext(c), storeB.FromContext(c)
		return c.SendString(fmt.Sprintf("%v|%v|%s|%s", a.Get("who"), b.Get("who"), a.ID(), b.ID()))
	})

	_, body := sendRequest(t, app, "/seed", nil)
	idA, idB, _ := strings.Cut(body, "|")
	require.NotEqual(t, idA, idB)

	// A's ID arrives in the query, a read-only source; B's arrives in its cookie.
	resp, body := sendRequest(t, app, "/read?sid="+idA, map[string]string{"b_sid": idB})
	require.Equal(t, fmt.Sprintf("a|b|%s|%s", idA, idB), body, "each store resolved its own session")

	cookies := cookiesByName(resp)
	require.NotContains(t, cookies, "a_sid", "an ID from the query is not promoted into a cookie")
	for name, cookie := range cookies {
		require.NotEqual(t, idA, cookie.Value, "A's ID reached the cookie %q", name)
	}
	require.Contains(t, cookies, "b_sid")
	require.Equal(t, idB, cookies["b_sid"].Value)
}

// An explicit Save of the session an outer middleware manages does nothing, as it does for the
// innermost one: the middleware saves it once, at the end.
func Test_Session_MultipleStores_ExplicitSaveIsNoOpForOuterSession(t *testing.T) {
	t.Parallel()

	recA, recB := newRecordingStorage(), newRecordingStorage()
	app := fiber.New()
	storeA, storeB := useCookieStore(app, "a_sid", Config{Storage: recA}), useCookieStore(app, "b_sid", Config{Storage: recB})

	var duringA, duringB atomic.Int64
	app.Get("/", func(c fiber.Ctx) error {
		for _, m := range []*Middleware{storeA.FromContext(c), storeB.FromContext(c)} {
			m.Set("k", "v")
			if err := m.Session.Save(); err != nil {
				return err
			}
		}
		duringA.Store(int64(len(recA.writes())))
		duringB.Store(int64(len(recB.writes())))
		return c.SendStatus(fiber.StatusOK)
	})

	_, _ = sendRequest(t, app, "/", nil)
	require.Zero(t, duringA.Load(), "the outer session was saved while the handler still ran")
	require.Zero(t, duringB.Load())
	require.Len(t, recA.writes(), 1, "the outer middleware saves its session once, after the handler")
	require.Len(t, recB.writes(), 1)
}

// With PassLocalsToContext a goroutine can keep the request context. Neither middleware may resolve
// through it once the request is over, and the outer one must not stay reachable through the inner
// one after it went back to the pool.
func Test_Session_MultipleStores_RetainedContext(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{PassLocalsToContext: true})
	storeA, storeB := useCookieStore(app, "a_sid"), useCookieStore(app, "b_sid")

	var kept atomic.Value
	var duringRequest atomic.Bool
	app.Get("/", func(c fiber.Ctx) error {
		kept.Store(c.Context())
		duringRequest.Store(storeA.FromContext(c.Context()) != nil && storeB.FromContext(c.Context()) != nil)
		return c.SendStatus(fiber.StatusOK)
	})

	_, _ = sendRequest(t, app, "/", nil)
	require.True(t, duringRequest.Load(), "both middleware resolve through the context while the request runs")

	ctx := kept.Load()
	require.NotNil(t, ctx)
	require.Nil(t, FromContext(ctx))
	require.Nil(t, storeA.FromContext(ctx), "the outer middleware stayed reachable after its request ended")
	require.Nil(t, storeB.FromContext(ctx))
}

// unsavableStorage is a storage that reads but cannot save.
type unsavableStorage struct {
	fiber.Storage
}

func (unsavableStorage) SetWithContext(context.Context, string, []byte, time.Duration) error {
	return errors.New("storage unavailable")
}

func (unsavableStorage) Set(string, []byte, time.Duration) error {
	return errors.New("storage unavailable")
}

// Each middleware saves its own session and reports a failed save to its own ErrorHandler; the
// failure of one does not stop the other from saving.
func Test_Session_MultipleStores_SaveErrorsGoToTheirOwnHandler(t *testing.T) {
	t.Parallel()

	var handledByA, handledByB atomic.Int64
	storageA := newRecordingStorage()
	app := fiber.New()
	storeA := useCookieStore(app, "a_sid", Config{Storage: storageA, ErrorHandler: func(fiber.Ctx, error) { handledByA.Add(1) }})
	storeB := useCookieStore(app, "b_sid", Config{
		Storage: unsavableStorage{Storage: memory.New()},
		ErrorHandler: func(c fiber.Ctx, _ error) {
			handledByB.Add(1)
			_ = c.Status(fiber.StatusInternalServerError).SendString("b failed") //nolint:errcheck // the test inspects the response
		},
	})
	app.Get("/", setInSessions("k", "v", storeA, storeB))

	resp, body := sendRequest(t, app, "/", nil)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "b failed", body, "the handler of the middleware whose save failed answered")
	require.Equal(t, int64(1), handledByB.Load())
	require.Zero(t, handledByA.Load(), "A's save did not fail, so A's handler was not called")
	require.Len(t, storageA.writes(), 1, "A saved after B failed")
}

// Next skips only the middleware it belongs to.
func Test_Session_MultipleStores_NextSkipsOnlyItsOwn(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	storeA := useCookieStore(app, "a_sid", Config{Next: func(c fiber.Ctx) bool { return c.Query("skip-a") != "" }})
	storeB := useCookieStore(app, "b_sid")
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(fmt.Sprint(storeA.FromContext(c) != nil, storeB.FromContext(c) != nil, FromContext(c) != nil))
	})

	resp, body := sendRequest(t, app, "/", nil)
	require.Equal(t, "true true true", body)
	require.Contains(t, cookiesByName(resp), "a_sid")

	resp, body = sendRequest(t, app, "/?skip-a=1", nil)
	require.Equal(t, "false true true", body, "A did not run; B did, and is the innermost")
	require.NotContains(t, cookiesByName(resp), "a_sid")
	require.Contains(t, cookiesByName(resp), "b_sid")
}

// Many requests at once through two nested middleware: nothing leaks from one request's sessions
// into another's. Run with -race.
func Test_Session_MultipleStores_Concurrent(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	storeA, storeB := useCookieStore(app, "a_sid"), useCookieStore(app, "b_sid")
	app.Get("/set", func(c fiber.Ctx) error {
		storeA.FromContext(c).Set("n", c.Query("n"))
		storeB.FromContext(c).Set("n", "b"+c.Query("n"))
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/get", func(c fiber.Ctx) error {
		return c.SendString(fmt.Sprintf("%v|%v", storeA.FromContext(c).Get("n"), storeB.FromContext(c).Get("n")))
	})

	// The clients only record what they saw; the test goroutine judges it.
	const clients, reads = 32, 5
	client := func(n int) ([]string, error) {
		resp, _, err := doRequest(app, fmt.Sprintf("/set?n=%d", n), nil)
		if err != nil {
			return nil, err
		}
		cookies := cookiesByName(resp)
		if cookies["a_sid"] == nil || cookies["b_sid"] == nil {
			return nil, errors.New("a session cookie is missing")
		}
		var bodies []string
		for range reads {
			_, body, getErr := doRequest(app, "/get", map[string]string{"a_sid": cookies["a_sid"].Value, "b_sid": cookies["b_sid"].Value})
			if getErr != nil {
				return nil, getErr
			}
			bodies = append(bodies, body)
		}
		return bodies, nil
	}
	bodies, errs := make([][]string, clients), make([]error, clients)
	var wg sync.WaitGroup
	for n := range clients {
		wg.Go(func() { bodies[n], errs[n] = client(n) })
	}
	wg.Wait()

	for n := range clients {
		require.NoError(t, errs[n], "client %d", n)
		require.Len(t, bodies[n], reads)
		for _, body := range bodies[n] {
			require.Equal(t, fmt.Sprintf("%d|b%d", n, n), body, "client %d saw another request's data", n)
		}
	}
}

// Once the middleware that ran inside another has finished, the outer one is the current middleware
// of the request again: code that runs between the two after c.Next() returns still finds it, and
// an explicit Save of its session is still recognized as the middleware's to make.
func Test_Session_MultipleStores_OuterCurrentAgainAfterInnerFinishes(t *testing.T) {
	t.Parallel()

	for _, pass := range []bool{false, true} {
		t.Run(fmt.Sprintf("PassLocalsToContext=%t", pass), func(t *testing.T) {
			t.Parallel()

			recA := newRecordingStorage()
			handlerA, storeA := NewWithStore(Config{Storage: recA, Extractor: extractors.FromCookie("a_sid")})
			handlerB, storeB := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})

			var after atomic.Value
			var writesDuringSave atomic.Int64
			app := fiber.New(fiber.Config{PassLocalsToContext: pass})
			app.Use(handlerA)
			app.Use(func(c fiber.Ctx) error {
				err := c.Next() // B runs and finishes inside this call

				a := storeA.FromContext(c)
				fromContext := "-"
				if pass {
					fromContext = strconv.FormatBool(storeA.FromContext(c.Context()) == a)
				}
				after.Store(fmt.Sprintf("%t,%t,%t,%s", a != nil, FromContext(c) == a, storeB.FromContext(c) == nil, fromContext))
				if a != nil {
					if saveErr := a.Session.Save(); saveErr != nil {
						return saveErr
					}
					writesDuringSave.Store(int64(len(recA.writes())))
				}
				return err
			})
			app.Use(handlerB)
			app.Get("/", setInSessions("k", "v", storeA, storeB))

			_, _ = sendRequest(t, app, "/", nil)

			want := "true,true,true,-"
			if pass {
				want = "true,true,true,true"
			}
			require.Equal(t, want, after.Load(), "A is current again, B is gone")
			require.Zero(t, writesDuringSave.Load(), "A's session is still the middleware's to save")
			require.Len(t, recA.writes(), 1, "and it is saved once, when A returns")
		})
	}
}

// The login-callback example of the documentation, step by step: the login session ends with the
// flow, the application session starts under a new ID.
func Test_Session_MultipleStores_LoginCallbackExample(t *testing.T) {
	t.Parallel()

	loginHandler, loginStore := NewWithStore(Config{
		Extractor:   extractors.FromCookie("login_sid"),
		IdleTimeout: 10 * time.Minute,
	})
	appHandler, appStore := NewWithStore(Config{
		Extractor:       extractors.FromCookie("app_sid"),
		AbsoluteTimeout: 24 * time.Hour,
	})

	app := fiber.New()
	app.Use(loginHandler, appHandler)
	app.Get("/login", func(c fiber.Ctx) error {
		loginStore.FromContext(c).Set("state", "xyz")
		return c.SendString("redirect to the provider")
	})
	app.Get("/auth/callback", func(c fiber.Ctx) error {
		login := loginStore.FromContext(c)
		user := appStore.FromContext(c)

		state, ok := GetAs[string](login, "state")
		if !ok || state != c.Query("state") {
			return fiber.ErrBadRequest
		}

		if err := login.Destroy(); err != nil {
			return err
		}
		if err := user.Regenerate(); err != nil {
			return err
		}
		user.Set("user_id", 123)

		return c.SendString("signed in")
	})
	app.Get("/me", func(c fiber.Ctx) error {
		id, ok := GetAs[int](appStore.FromContext(c), "user_id")
		_, hasState := GetAs[string](loginStore.FromContext(c), "state")
		return c.SendString(fmt.Sprintf("%d/%v/%v", id, ok, hasState))
	})

	resp, _ := sendRequest(t, app, "/login", nil)
	loginID := cookiesByName(resp)["login_sid"].Value
	require.Equal(t, 600, cookiesByName(resp)["login_sid"].MaxAge, "the login session has its own, short lifetime")

	resp, _ = sendRequest(t, app, "/auth/callback?state=wrong", map[string]string{"login_sid": loginID})
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode, "a callback with the wrong state is refused")

	resp, body := sendRequest(t, app, "/auth/callback?state=xyz", map[string]string{"login_sid": loginID})
	require.Equal(t, "signed in", body)
	cookies := cookiesByName(resp)
	require.Negative(t, cookies["login_sid"].MaxAge, "the login session is over: its cookie is expired")
	appID := cookies["app_sid"].Value
	require.NotEmpty(t, appID)
	require.Greater(t, cookies["app_sid"].MaxAge, 1700, "the application session lives by its own idle timeout")

	_, body = sendRequest(t, app, "/me", map[string]string{"app_sid": appID})
	require.Equal(t, "123/true/false", body)

	_, body = sendRequest(t, app, "/me", map[string]string{"app_sid": appID, "login_sid": loginID})
	require.Equal(t, "123/true/false", body, "the login session cannot be revived with its old cookie")
}

// Benchmark_Middleware_TwoStores is a request through two session middleware, each over its own
// cookie, that reaches both sessions through their stores.
func Benchmark_Middleware_TwoStores(b *testing.B) {
	app := fiber.New()
	storeA := useCookieStore(app, "a_sid", Config{KeyGenerator: func() string { return "session-a" }})
	storeB := useCookieStore(app, "b_sid", Config{KeyGenerator: func() string { return "session-b" }})
	app.Get("/", setInSessions("john", "doe", storeA, storeB))
	handler := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")
	// The first request creates both sessions, the rest load them, so the storages do not grow
	// while the benchmark runs.
	handler(fctx)

	b.ReportAllocs()
	for b.Loop() {
		fctx.ResetUserValues()
		fctx.Response.Reset()
		fctx.Request.Header.SetCookie("a_sid", "session-a")
		fctx.Request.Header.SetCookie("b_sid", "session-b")
		handler(fctx)
	}
	if fctx.Response.StatusCode() != fiber.StatusOK {
		b.Fatalf("status %d", fctx.Response.StatusCode())
	}
}

// A store that is asked again finds the session it saved, whatever other stores were asked in
// between, for a visitor who has no cookie yet: the ID that a store issues is cached for that
// store, and for the stores that share a cookie or header with it, and one that another store
// issues takes neither's place.
func Test_Session_Store_Get_AskedAgainFindsTheSessionItSaved(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})
	storeC := NewStore(Config{Extractor: extractors.FromCookie("c_sid")})

	// found asks store again, and expects the session saved before.
	found := func(t *testing.T, c fiber.Ctx, store *Store, want *Session) {
		t.Helper()
		sess := mustGet(t, store, c)
		require.Equal(t, want.ID(), sess.ID(), "the session saved before was lost within the request")
		require.False(t, sess.Fresh())
		require.Equal(t, want.Get("k"), sess.Get("k"))
	}

	t.Run("after other stores were asked, in any order", func(t *testing.T) {
		t.Parallel()
		c := newRequest(t, app)

		a, b, cc := saveNew(t, c, storeA), saveNew(t, c, storeB), saveNew(t, c, storeC)
		found(t, c, storeA, a)
		found(t, c, storeC, cc)
		found(t, c, storeB, b)
		found(t, c, storeA, a)
	})

	t.Run("by a store over the same cookie", func(t *testing.T) {
		t.Parallel()
		c := newRequest(t, app)

		a := saveNew(t, c, storeA)
		saveNew(t, c, storeB)
		found(t, c, NewStore(Config{Extractor: extractors.FromCookie("a_sid"), Storage: storeA.Storage}), a)
	})
}

// A request keeps the IDs that the stores asked in it issued, but one is only taken when it names a
// stored session: asked again before a save, a store issues another ID, whether another store was
// asked in between or not.
func Test_Session_Store_Get_AskedAgainIssuesANewSession(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})

	for _, tc := range []struct {
		name  string
		asked []*Store
	}{
		{"after another store was asked", []*Store{storeA, storeB, storeA}},
		{"the same store asked twice", []*Store{storeA, storeA}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newRequest(t, app)

			issued := make(map[string]bool)
			for _, store := range tc.asked {
				sess := mustGet(t, store, c)
				require.True(t, sess.Fresh())
				require.False(t, issued[sess.ID()], "a store that is asked again issues another ID, and each store its own")
				issued[sess.ID()] = true
			}
		})
	}
}

// The nearest session middleware outside that shares a source decides, not the outermost one. The
// outer one has no cookie of its own to find a session through and holds a new, unsaved ID; the
// middle one finds its session through the cookie b; the inner one shares the cookie a with both,
// and must take the session of the middle one.
func Test_Session_MultipleStores_NearestSharingOuterMiddlewareDecides(t *testing.T) {
	t.Parallel()

	storage := memory.New()
	outerHandler, outerStore := NewWithStore(Config{Storage: storage, Extractor: extractors.FromCookie("a")})
	middleHandler, middleStore := NewWithStore(Config{Storage: storage, Extractor: extractors.Chain(extractors.FromCookie("a"), extractors.FromCookie("b"))})
	innerHandler, innerStore := NewWithStore(Config{Storage: storage, Extractor: extractors.Chain(extractors.FromCookie("a"), extractors.FromCookie("d"))})

	app := fiber.New()
	app.Use(outerHandler, middleHandler, innerHandler)
	app.Get("/read", func(c fiber.Ctx) error {
		outer, middle, inner := outerStore.FromContext(c), middleStore.FromContext(c), innerStore.FromContext(c)
		return c.SendString(fmt.Sprintf("%t|%v|%t|%s|%v|%t", outer.Fresh(), middle.Get("who"), middle.Fresh(), inner.ID(), inner.Get("who"), inner.Fresh()))
	})

	id := seedSession(t, app, middleStore, func(s *Session) { s.Set("who", "middle") })
	_, body := sendRequest(t, app, "/read", map[string]string{"b": id})
	require.Equal(t, "true|middle|false|"+id+"|middle|false", body,
		"the inner middleware took the session of the outermost one, which is new, instead of the one of the nearest")
}

// A storage that fails when a middleware looks for the session of the one it runs inside is
// reported, as it is for the ID the request names and the ones Store.Get cached. The visitor has no
// cookie, so nothing else is read that could fail too.
func Test_Session_MultipleStores_OuterIDReadErrorReachesTheErrorHandler(t *testing.T) {
	t.Parallel()

	storage := &armedFailureStorage{Storage: memory.New(), err: errors.New("storage down")}
	outerHandler, _ := NewWithStore(Config{Storage: storage, Extractor: extractors.FromCookie("sid")})
	innerHandler, _ := NewWithStore(Config{Storage: storage, Extractor: extractors.FromCookie("sid")})

	app := fiber.New()
	app.Use(outerHandler, func(c fiber.Ctx) error {
		storage.armed.Store(true) // the outer middleware has loaded its session; the inner one looks for it next
		return c.Next()
	}, innerHandler)
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("reached") })

	resp, body := sendRequest(t, app, "/", nil)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode, body)
}
