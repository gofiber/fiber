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

// doRequest issues a GET with the given cookies and returns the response and
// its body. It reports failures as an error so goroutines can use it.
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

// Two session middleware, each over its own cookie, keep two sessions apart in
// one request. Before stores stopped adopting each other's cached ID, the second
// one looked up the first one's ID: its session never survived a request, or,
// over a shared backend, aliased the first session's record.
func Test_Session_MultipleStores_Middleware(t *testing.T) {
	t.Parallel()

	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared storage=%v", shared), func(t *testing.T) {
			t.Parallel()

			var storageA, storageB fiber.Storage = memory.New(), memory.New()
			if shared {
				storageB = storageA
			}
			handlerA, storeA := NewWithStore(Config{Storage: storageA, Extractor: extractors.FromCookie("a_sid")})
			handlerB, storeB := NewWithStore(Config{Storage: storageB, Extractor: extractors.FromCookie("b_sid")})

			app := fiber.New()
			app.Use(handlerA, handlerB)
			app.Get("/set", func(c fiber.Ctx) error {
				a, b := storeA.FromContext(c), storeB.FromContext(c)
				a.Set("who", "alice")
				b.Set("lang", "en")
				return c.SendString(a.ID() + "|" + b.ID())
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

// The package-level FromContext keeps meaning "the innermost middleware"; the
// store's own FromContext finds a particular one however deep it is nested.
func Test_Session_MultipleStores_FromContext(t *testing.T) {
	t.Parallel()

	handlerA, storeA := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
	handlerB, storeB := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})
	_, storeC := NewWithStore(Config{Extractor: extractors.FromCookie("c_sid")})

	app := fiber.New()
	app.Use(handlerA, handlerB)
	app.Get("/", func(c fiber.Ctx) error {
		innermost := FromContext(c)
		a, b := storeA.FromContext(c), storeB.FromContext(c)
		return c.SendString(strings.Join([]string{
			strconv.FormatBool(innermost != nil && innermost.Store() == storeB),
			strconv.FormatBool(a != nil && a.Store() == storeA),
			strconv.FormatBool(b != nil && b == innermost),
			strconv.FormatBool(storeC.FromContext(c) == nil),
			strconv.FormatBool(storeA.FromContext(c.RequestCtx()) == a),
		}, ","))
	})

	_, body := sendRequest(t, app, "/", nil)
	require.Equal(t, "true,true,true,true,true", body)

	t.Run("outside the request", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, storeA.FromContext(nil))

		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)
		require.Nil(t, storeA.FromContext(ctx), "no middleware ran for this context")
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

// The same, through the store pattern: the result must not depend on which
// store a request asks first, nor on which of the two has a session already.
func Test_Session_MultipleStores_StoreGet(t *testing.T) {
	t.Parallel()

	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})

	app := fiber.New()
	app.Get("/touch", func(c fiber.Ctx) error {
		first, second := storeA, storeB
		if c.Query("first") == "b" {
			first, second = storeB, storeA
		}
		s1, err := first.Get(c)
		if err != nil {
			return err
		}
		defer s1.Release()
		s2, err := second.Get(c)
		if err != nil {
			return err
		}
		defer s2.Release()

		a, b := s1, s2
		if c.Query("first") == "b" {
			a, b = s2, s1
		}
		out := fmt.Sprintf("A{fresh=%v n=%v} B{fresh=%v n=%v}", a.Fresh(), a.Get("n"), b.Fresh(), b.Get("n"))
		a.Set("n", 1)
		b.Set("n", 1)
		if err := a.Save(); err != nil {
			return err
		}
		if err := b.Save(); err != nil {
			return err
		}
		return c.SendString(out)
	})

	resp, body := sendRequest(t, app, "/touch", nil)
	require.Equal(t, "A{fresh=true n=<nil>} B{fresh=true n=<nil>}", body)
	cookies := cookiesByName(resp)
	idA, idB := cookies["a_sid"].Value, cookies["b_sid"].Value
	require.NotEqual(t, idA, idB)

	for _, tc := range []struct {
		name, target string
		cookies      map[string]string
		want         string
	}{
		{"both valid, A first", "/touch", map[string]string{"a_sid": idA, "b_sid": idB}, "A{fresh=false n=1} B{fresh=false n=1}"},
		{"both valid, B first", "/touch?first=b", map[string]string{"a_sid": idA, "b_sid": idB}, "A{fresh=false n=1} B{fresh=false n=1}"},
		// The store asked first has nothing yet; the other one must still find its own.
		{"only B valid, A first", "/touch", map[string]string{"b_sid": idB}, "A{fresh=true n=<nil>} B{fresh=false n=1}"},
		{"only B valid, B first", "/touch?first=b", map[string]string{"b_sid": idB}, "A{fresh=true n=<nil>} B{fresh=false n=1}"},
		{"only A valid, A first", "/touch", map[string]string{"a_sid": idA}, "A{fresh=false n=1} B{fresh=true n=<nil>}"},
		{"only A valid, B first", "/touch?first=b", map[string]string{"a_sid": idA}, "A{fresh=false n=1} B{fresh=true n=<nil>}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, got := sendRequest(t, app, tc.target, tc.cookies)
			require.Equal(t, tc.want, got)
		})
	}
}

// Stores over the same cookie still agree on one ID for a visitor without a
// cookie yet. The CSRF middleware's store and the session middleware's own are
// two Store values like this, and a visitor's first request must end up with
// one session, not two.
func Test_Session_MultipleStores_SameCookieSharesRequestID(t *testing.T) {
	t.Parallel()

	storage := memory.New()
	early := NewStore(Config{Storage: storage})
	handler, late := NewWithStore(Config{Storage: storage})
	require.NotSame(t, early, late)

	var earlyID atomic.Value
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		sess, err := early.Get(c)
		if err != nil {
			return err
		}
		defer sess.Release()
		sess.Set("token", "t1")
		if err := sess.Save(); err != nil {
			return err
		}
		earlyID.Store(sess.ID())
		return c.Next()
	})
	app.Use(handler)
	app.Get("/", func(c fiber.Ctx) error {
		m := late.FromContext(c)
		return c.SendString(fmt.Sprintf("%v|%v", m.Get("token"), m.Fresh()))
	})

	resp, body := sendRequest(t, app, "/", nil)
	require.Equal(t, "t1|false", body, "the later store adopted the ID the earlier one saved and loaded its record")

	cookies := cookiesByName(resp)
	require.Contains(t, cookies, "session_id")
	require.Equal(t, earlyID.Load(), cookies["session_id"].Value, "the cookie carries the one ID both stores used")

	keys, err := storage.Keys()
	require.NoError(t, err)
	require.Len(t, keys, 1, "one visitor, one stored session")
}

// The store that published the cached ID last is the one that counts. A session
// from Store.Get leaves its store behind; a middleware that runs after it
// publishes a newer ID, and the next middleware must compare itself with that
// one, not with the store of the older ID. Here the early store reads the cookie
// B reads, but over a storage that has never seen it, so it is new and leaves an
// ID behind; A, over another cookie, is new as well. B has a session, and must
// find it instead of adopting A's ID.
func Test_Session_MultipleStores_NewestPublisherCounts(t *testing.T) {
	t.Parallel()

	early := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})
	handlerA, _ := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
	handlerB, storeB := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		sess, err := early.Get(c)
		if err != nil {
			return err
		}
		sess.Release()
		return c.Next()
	})
	app.Use(handlerA, handlerB)
	app.Get("/seed", func(c fiber.Ctx) error {
		storeB.FromContext(c).Set("who", "b")
		return c.SendString(storeB.FromContext(c).ID())
	})
	app.Get("/read", func(c fiber.Ctx) error {
		b := storeB.FromContext(c)
		return c.SendString(fmt.Sprintf("%v|%v|%s", b.Get("who"), b.Fresh(), b.ID()))
	})

	_, idB := sendRequest(t, app, "/seed", nil)

	resp, body := sendRequest(t, app, "/read", map[string]string{"b_sid": idB})
	require.Equal(t, "b|false|"+idB, body, "B loaded its own session; it did not adopt the ID A cached")
	require.Equal(t, idB, cookiesByName(resp)["b_sid"].Value)
}

// The provenance cached on the request belongs to the store that resolved it.
// Store A saves a new session and so caches its ID; store B then finds its own
// session through another cookie, which makes B's provenance the cached one. A's
// next Get still passes for the publisher of the cached ID, but must not take B's
// provenance with it: A's session would be written back to B's cookie, replacing
// the ID of B's session in the response.
func Test_Session_MultipleStores_CachedProvenanceBelongsToItsStore(t *testing.T) {
	t.Parallel()

	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})

	app := fiber.New()
	app.Get("/seed", func(c fiber.Ctx) error {
		sess, err := storeB.Get(c)
		if err != nil {
			return err
		}
		defer sess.Release()
		sess.Set("n", 1)
		if err := sess.Save(); err != nil {
			return err
		}
		return c.SendString(sess.ID())
	})
	app.Get("/touch", func(c fiber.Ctx) error {
		first, err := storeA.Get(c)
		if err != nil {
			return err
		}
		defer first.Release()
		first.Set("n", 1)
		if saveErr := first.Save(); saveErr != nil {
			return saveErr
		}

		b, err := storeB.Get(c)
		if err != nil {
			return err
		}
		defer b.Release()

		again, err := storeA.Get(c)
		if err != nil {
			return err
		}
		defer again.Release()

		// A last: were its session tied to B's cookie, the later header would win.
		if saveErr := b.Save(); saveErr != nil {
			return saveErr
		}
		if saveErr := again.Save(); saveErr != nil {
			return saveErr
		}
		return c.SendString(fmt.Sprintf("%v|%s", b.Fresh(), b.ID()))
	})

	_, idB := sendRequest(t, app, "/seed", nil)

	resp, body := sendRequest(t, app, "/touch", map[string]string{"b_sid": idB})
	require.Equal(t, "false|"+idB, body, "B loaded its own session")
	var written []string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "b_sid" {
			written = append(written, cookie.Value)
		}
	}
	require.Equal(t, []string{idB}, written, "B's cookie carries B's session and nothing else")
	require.Contains(t, cookiesByName(resp), "a_sid", "A's session goes to A's cookie")
}

// Stores over the same cookie agree on one ID, and the store that asks second
// takes the provenance the first one cached together with it, so its session
// goes back to the cookie the ID came from and not to every sink of the chain.
func Test_Session_MultipleStores_SharingStoreAdoptsCachedProvenance(t *testing.T) {
	t.Parallel()

	chain := func() extractors.Extractor {
		return extractors.Chain(extractors.FromCookie("sid"), extractors.FromHeader("X-Sid"))
	}
	shared := memory.New()
	first := NewStore(Config{Storage: shared, Extractor: chain()})
	second := NewStore(Config{Storage: shared, Extractor: chain()})
	require.NotSame(t, first, second)

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.SetCookie("sid", "unknown")

	p, err := first.Get(c)
	require.NoError(t, err)
	defer p.Release()
	require.True(t, p.Fresh(), "the cookie names no session, so the first store starts one and caches its ID")
	require.NoError(t, p.Save())

	q, err := second.Get(c)
	require.NoError(t, err)
	defer q.Release()
	require.Equal(t, p.ID(), q.ID(), "the second store takes the cached ID")
	require.False(t, q.Fresh(), "and finds the session the first one saved")
	require.Equal(t, p.extractor, q.extractor, "and the provenance cached with it")

	// The first store's save wrote the cookie into this response as well: clear it,
	// so that what is left is what the second store writes.
	c.Response().Header.DelCookie("sid")
	require.NoError(t, q.Save())
	require.Empty(t, c.Response().Header.Peek("X-Sid"), "the ID goes back to the cookie it came from, not to the header too")

	written := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(written)
	written.SetKey("sid")
	require.True(t, c.Response().Header.Cookie(written), "and to the cookie")
	require.Equal(t, q.ID(), string(written.Value()))
}

// Save leaves a session to the middleware that manages it, and only that one: a
// session no middleware of the request manages is saved, even while the middleware
// of another store is the current one.
func Test_Session_MultipleStores_SaveOfASessionNoMiddlewareManages(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})

	early, err := storeA.Get(c)
	require.NoError(t, err)
	defer early.Release()
	early.Set("k", "v")

	m := startMiddleware(t, c, storeB)
	defer releaseMiddleware(m)

	require.NoError(t, early.Save())
	stored, err := storeA.Storage.GetWithContext(context.Background(), storeA.storageKey(early.ID()))
	require.NoError(t, err)
	require.NotEmpty(t, stored, "the session was saved")
}

// With only one of two stacked sessions in existence, each store finds its own,
// whichever of them is new. The new one caches an ID the other must not adopt.
func Test_Session_MultipleStores_Middleware_OnlyOneSessionExists(t *testing.T) {
	t.Parallel()

	handlerA, storeA := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
	handlerB, storeB := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})

	app := fiber.New()
	app.Use(handlerA, handlerB)
	app.Get("/seed", func(c fiber.Ctx) error {
		storeA.FromContext(c).Set("n", 1)
		storeB.FromContext(c).Set("n", 1)
		return c.SendString(storeA.FromContext(c).ID() + "|" + storeB.FromContext(c).ID())
	})
	app.Get("/read", func(c fiber.Ctx) error {
		a, b := storeA.FromContext(c), storeB.FromContext(c)
		return c.SendString(fmt.Sprintf("%v/%v|%v/%v", a.Get("n"), a.Fresh(), b.Get("n"), b.Fresh()))
	})

	_, body := sendRequest(t, app, "/seed", nil)
	ids := strings.Split(body, "|")
	idA, idB := ids[0], ids[1]

	_, body = sendRequest(t, app, "/read", map[string]string{"b_sid": idB})
	require.Equal(t, "<nil>/true|1/false", body, "A is new, B found its session")

	_, body = sendRequest(t, app, "/read", map[string]string{"a_sid": idA})
	require.Equal(t, "1/false|<nil>/true", body, "B is new, A found its session")
}

// The provenance of one store's ID must never reach another store: an ID read
// from a read-only source is not written to a cookie, and a store never writes
// its ID to a cookie it does not own. Both are what session fixation relies on.
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
	ids := strings.Split(body, "|")
	idA, idB := ids[0], ids[1]
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

// An explicit Save of the session an outer middleware manages does nothing, as
// it does for the innermost one: the middleware saves it once, at the end.
func Test_Session_MultipleStores_ExplicitSaveIsNoOpForOuterSession(t *testing.T) {
	t.Parallel()

	recA, recB := newRecordingStorage(), newRecordingStorage()
	handlerA, storeA := NewWithStore(Config{Storage: recA, Extractor: extractors.FromCookie("a_sid")})
	handlerB, storeB := NewWithStore(Config{Storage: recB, Extractor: extractors.FromCookie("b_sid")})

	var duringA, duringB atomic.Int64
	app := fiber.New()
	app.Use(handlerA, handlerB)
	app.Get("/", func(c fiber.Ctx) error {
		a, b := storeA.FromContext(c), storeB.FromContext(c)
		a.Set("k", "v")
		b.Set("k", "v")
		if err := a.Session.Save(); err != nil {
			return err
		}
		if err := b.Session.Save(); err != nil {
			return err
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

// With PassLocalsToContext a goroutine can keep the request context. Neither
// middleware may resolve through it once the request is over, and the outer one
// must not stay reachable through the inner one after it went back to the pool.
func Test_Session_MultipleStores_RetainedContext(t *testing.T) {
	t.Parallel()

	handlerA, storeA := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
	handlerB, storeB := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})

	app := fiber.New(fiber.Config{PassLocalsToContext: true})
	app.Use(handlerA, handlerB)

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

// Each middleware saves its own session and reports a failed save to its own
// ErrorHandler; the failure of one does not stop the other from saving.
func Test_Session_MultipleStores_SaveErrorsGoToTheirOwnHandler(t *testing.T) {
	t.Parallel()

	var handledByA, handledByB atomic.Int64
	storageA := newRecordingStorage()
	handlerA, storeA := NewWithStore(Config{
		Storage:      storageA,
		Extractor:    extractors.FromCookie("a_sid"),
		ErrorHandler: func(fiber.Ctx, error) { handledByA.Add(1) },
	})
	handlerB, storeB := NewWithStore(Config{
		Storage:   unsavableStorage{Storage: memory.New()},
		Extractor: extractors.FromCookie("b_sid"),
		ErrorHandler: func(c fiber.Ctx, _ error) {
			handledByB.Add(1)
			_ = c.Status(fiber.StatusInternalServerError).SendString("b failed") //nolint:errcheck // the test inspects the response
		},
	})

	app := fiber.New()
	app.Use(handlerA, handlerB)
	app.Get("/", func(c fiber.Ctx) error {
		storeA.FromContext(c).Set("k", "v")
		storeB.FromContext(c).Set("k", "v")
		return c.SendString("ok")
	})

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

	handlerA, storeA := NewWithStore(Config{
		Extractor: extractors.FromCookie("a_sid"),
		Next:      func(c fiber.Ctx) bool { return c.Query("skip-a") != "" },
	})
	handlerB, storeB := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})

	app := fiber.New()
	app.Use(handlerA, handlerB)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(strings.Join([]string{
			strconv.FormatBool(storeA.FromContext(c) != nil),
			strconv.FormatBool(storeB.FromContext(c) != nil),
			strconv.FormatBool(FromContext(c) != nil),
		}, ","))
	})

	resp, body := sendRequest(t, app, "/", nil)
	require.Equal(t, "true,true,true", body)
	require.Contains(t, cookiesByName(resp), "a_sid")

	resp, body = sendRequest(t, app, "/?skip-a=1", nil)
	require.Equal(t, "false,true,true", body, "A did not run; B did, and is the innermost")
	require.NotContains(t, cookiesByName(resp), "a_sid")
	require.Contains(t, cookiesByName(resp), "b_sid")
}

// Many requests at once through two nested middleware: nothing leaks from one
// request's sessions into another's. Run with -race.
func Test_Session_MultipleStores_Concurrent(t *testing.T) {
	t.Parallel()

	handlerA, storeA := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
	handlerB, storeB := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})

	app := fiber.New()
	app.Use(handlerA, handlerB)
	app.Get("/set", func(c fiber.Ctx) error {
		storeA.FromContext(c).Set("n", c.Query("n"))
		storeB.FromContext(c).Set("n", "b"+c.Query("n"))
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/get", func(c fiber.Ctx) error {
		return c.SendString(fmt.Sprintf("%v|%v", storeA.FromContext(c).Get("n"), storeB.FromContext(c).Get("n")))
	})

	// The clients only record what they saw; the test goroutine judges it.
	type outcome struct {
		err    error
		bodies []string
	}
	const clients = 32
	results := make([]outcome, clients)
	var wg sync.WaitGroup
	for n := range clients {
		wg.Go(func() {
			resp, _, err := doRequest(app, fmt.Sprintf("/set?n=%d", n), nil)
			if err != nil {
				results[n].err = err
				return
			}
			cookies := cookiesByName(resp)
			if cookies["a_sid"] == nil || cookies["b_sid"] == nil {
				results[n].err = errors.New("a session cookie is missing")
				return
			}
			for range 5 {
				_, body, err := doRequest(app, "/get", map[string]string{
					"a_sid": cookies["a_sid"].Value,
					"b_sid": cookies["b_sid"].Value,
				})
				if err != nil {
					results[n].err = err
					return
				}
				results[n].bodies = append(results[n].bodies, body)
			}
		})
	}
	wg.Wait()

	for n, result := range results {
		require.NoError(t, result.err, "client %d", n)
		require.Len(t, result.bodies, 5)
		for _, body := range result.bodies {
			require.Equal(t, fmt.Sprintf("%d|b%d", n, n), body, "client %d saw another request's data", n)
		}
	}
}

// Once the middleware that ran inside another has finished, the outer one is the
// current middleware of the request again: code that runs between the two after
// c.Next() returns still finds it, and an explicit Save of its session is still
// recognized as the middleware's to make.
func Test_Session_MultipleStores_OuterCurrentAgainAfterInnerFinishes(t *testing.T) {
	t.Parallel()

	for _, pass := range []bool{false, true} {
		t.Run("PassLocalsToContext="+strconv.FormatBool(pass), func(t *testing.T) {
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
				after.Store(strings.Join([]string{
					strconv.FormatBool(a != nil),
					strconv.FormatBool(FromContext(c) == a),
					strconv.FormatBool(storeB.FromContext(c) == nil),
					fromContext,
				}, ","))
				if a != nil {
					if saveErr := a.Session.Save(); saveErr != nil {
						return saveErr
					}
					writesDuringSave.Store(int64(len(recA.writes())))
				}
				return err
			})
			app.Use(handlerB)
			app.Get("/", func(c fiber.Ctx) error {
				storeA.FromContext(c).Set("k", "v")
				storeB.FromContext(c).Set("k", "v")
				return nil
			})

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

// The login-callback example of the documentation, step by step: the login
// session ends with the flow, the application session starts under a new ID.
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

// Benchmark_Middleware_TwoStores is a request through two session middleware,
// each over its own cookie, that reaches both sessions through their stores.
//
// go test -v -run=^$ -bench=Benchmark_Middleware_TwoStores -benchmem -count=4
func Benchmark_Middleware_TwoStores(b *testing.B) {
	handlerA, storeA := NewWithStore(Config{
		Extractor:    extractors.FromCookie("a_sid"),
		KeyGenerator: func() string { return "session-a" },
	})
	handlerB, storeB := NewWithStore(Config{
		Extractor:    extractors.FromCookie("b_sid"),
		KeyGenerator: func() string { return "session-b" },
	})

	app := fiber.New()
	app.Use(handlerA, handlerB)
	app.Get("/", func(c fiber.Ctx) error {
		storeA.FromContext(c).Set("john", "doe")
		storeB.FromContext(c).Set("john", "doe")
		return nil
	})
	handler := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")
	// The first request creates both sessions, the rest load them, so the
	// storages do not grow while the benchmark runs.
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
