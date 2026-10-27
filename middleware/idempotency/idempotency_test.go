package idempotency

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
	"github.com/valyala/fasthttp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validKey = "00000000-0000-0000-0000-000000000000"

// go test -run Test_Idempotency
func Test_Idempotency(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(func(c fiber.Ctx) error {
		if err := c.Next(); err != nil {
			return err
		}

		isMethodSafe := fiber.IsMethodSafe(c.Method())
		isIdempotent := IsFromCache(c) || WasPutToCache(c)
		hasReqHeader := c.Get("X-Idempotency-Key") != ""

		if isMethodSafe {
			if isIdempotent {
				return errors.New("request with safe HTTP method should not be idempotent")
			}
		} else {
			// Unsafe
			if hasReqHeader {
				if !isIdempotent {
					return errors.New("request with unsafe HTTP method should be idempotent if X-Idempotency-Key request header is set")
				}
			} else if isIdempotent {
				return errors.New("request with unsafe HTTP method should not be idempotent if X-Idempotency-Key request header is not set")
			}
		}

		return nil
	})

	// The memory storage expires entries on a coarse 1-second clock
	// (exp = uint32(ttl.Seconds()) + Timestamp()), so a freshly stored entry has
	// up to a full second less real headroom than its nominal lifetime. Keep the
	// lifetime comfortably above that 1-second granularity so the back-to-back
	// "store then replay" assertions below don't flake when a slow CI runner
	// pauses between two consecutive requests. It must also stay small enough
	// that the /slow handler's 3*lifetime sleep finishes within the 15s request
	// timeout used in doReq.
	const lifetime = 3 * time.Second

	app.Use(New(Config{
		Lifetime: lifetime,
	}))

	nextCount := func() func() int {
		var count atomic.Int32
		return func() int {
			return int(count.Add(1))
		}
	}()

	app.Add([]string{
		fiber.MethodGet,
		fiber.MethodPost,
		fiber.MethodQuery,
	}, "/", func(c fiber.Ctx) error {
		return c.SendString(strconv.Itoa(nextCount()))
	})

	app.Post("/slow", func(c fiber.Ctx) error {
		time.Sleep(3 * lifetime)

		return c.SendString(strconv.Itoa(nextCount()))
	})

	doReq := func(method, route, idempotencyKey string) string {
		req := httptest.NewRequest(method, route, http.NoBody)
		if idempotencyKey != "" {
			req.Header.Set("X-Idempotency-Key", idempotencyKey)
		}
		resp, err := app.Test(req, fiber.TestConfig{
			Timeout:       15 * time.Second,
			FailOnTimeout: true,
		})
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, string(body))
		return string(body)
	}

	require.Equal(t, "1", doReq(fiber.MethodGet, "/", ""))
	require.Equal(t, "2", doReq(fiber.MethodGet, "/", ""))

	require.Equal(t, "3", doReq(fiber.MethodPost, "/", ""))
	require.Equal(t, "4", doReq(fiber.MethodPost, "/", ""))

	require.Equal(t, "5", doReq(fiber.MethodGet, "/", "00000000-0000-0000-0000-000000000000"))
	require.Equal(t, "6", doReq(fiber.MethodGet, "/", "00000000-0000-0000-0000-000000000000"))

	require.Equal(t, "7", doReq(fiber.MethodPost, "/", "00000000-0000-0000-0000-000000000000"))
	require.Equal(t, "7", doReq(fiber.MethodPost, "/", "00000000-0000-0000-0000-000000000000"))
	require.Equal(t, "8", doReq(fiber.MethodPost, "/", ""))
	require.Equal(t, "9", doReq(fiber.MethodPost, "/", "11111111-1111-1111-1111-111111111111"))

	require.Equal(t, "7", doReq(fiber.MethodPost, "/", "00000000-0000-0000-0000-000000000000"))
	time.Sleep(4 * lifetime)
	require.Equal(t, "10", doReq(fiber.MethodPost, "/", "00000000-0000-0000-0000-000000000000"))
	require.Equal(t, "10", doReq(fiber.MethodPost, "/", "00000000-0000-0000-0000-000000000000"))

	// Test raciness
	{
		var wg sync.WaitGroup
		for range 100 {
			wg.Go(func() {
				assert.Equal(t, "11", doReq(fiber.MethodPost, "/slow", "22222222-2222-2222-2222-222222222222"))
			})
		}
		wg.Wait()
		require.Equal(t, "11", doReq(fiber.MethodPost, "/slow", "22222222-2222-2222-2222-222222222222"))
	}
	time.Sleep(3 * lifetime)
	require.Equal(t, "12", doReq(fiber.MethodPost, "/slow", "22222222-2222-2222-2222-222222222222"))

	// QUERY is safe (RFC 10008), so idempotency is skipped: each request runs the
	// handler and increments the counter even with an idempotency key set.
	require.Equal(t, "13", doReq(fiber.MethodQuery, "/", "33333333-3333-3333-3333-333333333333"))
	require.Equal(t, "14", doReq(fiber.MethodQuery, "/", "33333333-3333-3333-3333-333333333333"))
}

// go test -v -run=^$ -bench=Benchmark_Idempotency -benchmem -count=4
func Benchmark_Idempotency(b *testing.B) {
	app := fiber.New()

	// Needs to be at least a second as the memory storage doesn't support shorter durations.
	const lifetime = 1 * time.Second

	app.Use(New(Config{
		Lifetime: lifetime,
	}))

	app.Post("/", func(_ fiber.Ctx) error {
		return nil
	})

	h := app.Handler()

	b.Run("hit", func(b *testing.B) {
		c := &fasthttp.RequestCtx{}
		c.Request.Header.SetMethod(fiber.MethodPost)
		c.Request.SetRequestURI("/")
		c.Request.Header.Set("X-Idempotency-Key", "00000000-0000-0000-0000-000000000000")

		b.ReportAllocs()
		for b.Loop() {
			h(c)
		}
	})

	b.Run("skip", func(b *testing.B) {
		c := &fasthttp.RequestCtx{}
		c.Request.Header.SetMethod(fiber.MethodPost)
		c.Request.SetRequestURI("/")

		b.ReportAllocs()
		for b.Loop() {
			h(c)
		}
	})

	scoped := fiber.New()
	scoped.Use(New(Config{
		Lifetime: lifetime,
		KeyScope: func(c fiber.Ctx) string { return c.Get("X-User") },
	}))
	scoped.Post("/", func(_ fiber.Ctx) error {
		return nil
	})
	hs := scoped.Handler()

	b.Run("scoped hit", func(b *testing.B) {
		c := &fasthttp.RequestCtx{}
		c.Request.Header.SetMethod(fiber.MethodPost)
		c.Request.SetRequestURI("/")
		c.Request.Header.Set("X-Idempotency-Key", "00000000-0000-0000-0000-000000000000")
		c.Request.Header.Set("X-User", "alice")

		b.ReportAllocs()
		for b.Loop() {
			hs(c)
		}
	})
}

// go test -v -run=^$ -bench=Benchmark_scopedKey -benchmem -count=4
func Benchmark_scopedKey(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		scopedKey("alice", validKey)
	}
}

func Test_configDefault_defaults(t *testing.T) {
	t.Parallel()

	cfg := configDefault()
	require.NotNil(t, cfg.Lock)
	require.NotNil(t, cfg.Storage)
	require.Equal(t, ConfigDefault.Lifetime, cfg.Lifetime)
	require.Equal(t, ConfigDefault.KeyHeader, cfg.KeyHeader)
	require.Nil(t, cfg.KeepResponseHeaders)
	require.Nil(t, cfg.KeyScope)

	app := fiber.New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx := app.AcquireCtx(fctx)
	require.True(t, cfg.Next(ctx))
	app.ReleaseCtx(ctx)

	fctx = &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx = app.AcquireCtx(fctx)
	require.False(t, cfg.Next(ctx))
	app.ReleaseCtx(ctx)

	require.NoError(t, cfg.KeyHeaderValidate(validKey))
	require.Error(t, cfg.KeyHeaderValidate("short"))
}

func Test_configDefault_override(t *testing.T) {
	t.Parallel()

	l := &stubLock{}
	s := &stubStorage{}

	cfg := configDefault(Config{
		Lifetime:            42 * time.Second,
		KeyHeader:           "Foo",
		KeepResponseHeaders: []string{},
		Lock:                l,
		Storage:             s,
		KeyScope:            func(fiber.Ctx) string { return "scope" },
	})

	require.Equal(t, 42*time.Second, cfg.Lifetime)
	require.Equal(t, "Foo", cfg.KeyHeader)
	require.NotNil(t, cfg.KeyScope)
	require.Nil(t, cfg.KeepResponseHeaders)
	require.Equal(t, l, cfg.Lock)
	require.Equal(t, s, cfg.Storage)
	require.NotNil(t, cfg.Next)
	require.NotNil(t, cfg.KeyHeaderValidate)
}

// helper to perform request
func do(app *fiber.App, req *http.Request) (resp *http.Response, body string) { //nolint:nonamedreturns // gocritic unnamedResult prefers naming returned response and body payload for clarity
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		panic(err)
	}
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	return resp, string(payload)
}

func Test_New_NextSkip(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	var count int

	app.Use(New(Config{Next: func(_ fiber.Ctx) bool { return true }}))

	app.Post("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
	})

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	_, body1 := do(app, req)

	req2 := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req2.Header.Set(ConfigDefault.KeyHeader, validKey)
	_, body2 := do(app, req2)

	require.Equal(t, "1", body1)
	require.Equal(t, "2", body2)
}

func Test_New_InvalidKey(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Post("/", func(_ fiber.Ctx) error { return nil })

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, "bad")
	resp, body := do(app, req)

	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Contains(t, body, "invalid length")
}

func Test_New_StorageGetError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{getErr: errors.New("boom")}
	app.Use(New(Config{Storage: s, Lock: &stubLock{}}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)

	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Contains(t, body, "failed to write cached response at fastpath")
}

func Test_New_UnmarshalError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{data: map[string][]byte{validKey: []byte("bad")}}
	app.Use(New(Config{Storage: s, Lock: &stubLock{}}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)

	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Contains(t, body, "failed to write cached response at fastpath")
}

func Test_New_StoreRetrieve_FilterHeaders(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{}
	app.Use(New(Config{
		Storage:             s,
		Lock:                &stubLock{},
		KeepResponseHeaders: []string{"Foo"},
	}))

	var count int
	app.Post("/", func(c fiber.Ctx) error {
		count++
		c.Set("Foo", "foo")
		c.Set("Bar", "bar")
		return c.SendString(fmt.Sprintf("resp%d", count))
	})

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)
	require.Equal(t, "resp1", body)
	require.Equal(t, "foo", resp.Header.Get("Foo"))
	require.Equal(t, "bar", resp.Header.Get("Bar"))

	req2 := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req2.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp2, body2 := do(app, req2)
	require.Equal(t, "resp1", body2)
	require.Equal(t, "foo", resp2.Header.Get("Foo"))
	require.Empty(t, resp2.Header.Get("Bar"))
	require.Equal(t, 1, count)
	require.Equal(t, 1, s.setCount)
}

// Test_New_StoreRetrieve_FilterHeadersCaseInsensitiveSnapshot pins two
// properties of the keep-header filter: configured names match response
// header names ASCII case-insensitively (fasthttp reports canonical case like
// "Foo"), and the name list is snapshotted at New() time, so mutating the
// caller's slice afterwards cannot change an already-built handler.
func Test_New_StoreRetrieve_FilterHeadersCaseInsensitiveSnapshot(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{}
	keep := []string{"fOO"}
	app.Use(New(Config{
		Storage:             s,
		Lock:                &stubLock{},
		KeepResponseHeaders: keep,
	}))
	// Post-construction mutation must not affect the handler.
	keep[0] = "Bar"

	var count int
	app.Post("/", func(c fiber.Ctx) error {
		count++
		c.Set("Foo", "foo")
		c.Set("Bar", "bar")
		return c.SendString(fmt.Sprintf("resp%d", count))
	})

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	_, body := do(app, req)
	require.Equal(t, "resp1", body)

	req2 := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req2.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp2, body2 := do(app, req2)
	require.Equal(t, "resp1", body2)
	require.Equal(t, "foo", resp2.Header.Get("Foo"), "differently-cased configured name must still match")
	require.Empty(t, resp2.Header.Get("Bar"), "mutating the config slice after New must not change filtering")
	require.Equal(t, 1, count)
}

// Test_New_StoreRetrieve_HeadersWithCommas pins that a replayed response
// carries each header value exactly as the handler set it, even when the app
// enables EnableSplittingOnParsers, which only concerns request parsing. The
// cookie is kept because the config names it.
func Test_New_StoreRetrieve_HeadersWithCommas(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{EnableSplittingOnParsers: true})
	app.Use(New(Config{
		Storage:             &stubStorage{},
		Lock:                &stubLock{},
		KeepResponseHeaders: []string{fiber.HeaderContentDisposition, fiber.HeaderSetCookie},
	}))

	var count int
	app.Post("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderContentDisposition, `attachment; filename="a,b.txt"`)
		c.Cookie(&fiber.Cookie{
			Name:    "session",
			Value:   "abc",
			Expires: time.Date(2030, time.October, 21, 7, 28, 0, 0, time.UTC),
		})
		return c.SendString(fmt.Sprintf("resp%d", count))
	})

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)
	require.Equal(t, "resp1", body)

	req2 := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req2.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp2, body2 := do(app, req2)
	require.Equal(t, "resp1", body2)
	require.Equal(t, 1, count)
	require.NotEmpty(t, resp2.Header.Values(fiber.HeaderSetCookie))
	require.Equal(t, resp.Header.Values(fiber.HeaderContentDisposition), resp2.Header.Values(fiber.HeaderContentDisposition))
	require.Equal(t, resp.Header.Values(fiber.HeaderSetCookie), resp2.Header.Values(fiber.HeaderSetCookie))
}

// Test_New_DoesNotReplayCookies pins that a cookie is neither stored nor
// replayed by default: the key alone identifies a request, so a replayed
// Set-Cookie hands the session of whoever sent the key first to whoever sends
// it next, on any route.
func Test_New_DoesNotReplayCookies(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{}
	app.Use(New(Config{Storage: s, Lock: &stubLock{}}))

	var count int
	app.Post("/login", func(c fiber.Ctx) error {
		count++
		c.Cookie(&fiber.Cookie{Name: "sid", Value: "ALICE-SESSION"})
		c.Set(fiber.HeaderSetCookie2, "sid=ALICE-SESSION; Version=1")
		c.Set("X-Request", "alice")
		return c.SendString("welcome alice")
	})
	app.Post("/other", func(c fiber.Ctx) error {
		count++
		return c.SendString("other")
	})

	post := func(route string) (*http.Response, string) {
		req := httptest.NewRequest(http.MethodPost, route, http.NoBody)
		req.Header.Set(ConfigDefault.KeyHeader, validKey)
		return do(app, req)
	}

	// the client that caused the miss gets its cookies
	resp, body := post("/login")
	require.Equal(t, "welcome alice", body)
	require.Equal(t, []string{"sid=ALICE-SESSION; path=/; SameSite=Lax"}, resp.Header.Values(fiber.HeaderSetCookie))
	require.Equal(t, []string{"sid=ALICE-SESSION; Version=1"}, resp.Header.Values(fiber.HeaderSetCookie2))

	// whoever sends the key next gets the rest of the response, and the
	// handler is not run for them, on the route that stored it or another
	for _, route := range []string{"/login", "/other"} {
		resp, body = post(route)
		require.Equal(t, "welcome alice", body, route)
		require.Equal(t, "alice", resp.Header.Get("X-Request"), route)
		require.Empty(t, resp.Header.Values(fiber.HeaderSetCookie), route)
		require.Empty(t, resp.Header.Values(fiber.HeaderSetCookie2), route)
	}
	require.Equal(t, 1, count)

	// and nothing in the storage holds them
	for _, val := range s.data {
		require.NotContains(t, string(val), "ALICE-SESSION")
	}
}

// Test_New_KeepResponseHeaders_NamedCookies pins that naming a cookie header in
// KeepResponseHeaders stores and replays it, matched case-insensitively, and
// that naming one of the two leaves the other out.
func Test_New_KeepResponseHeaders_NamedCookies(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Storage:             &stubStorage{},
		Lock:                &stubLock{},
		KeepResponseHeaders: []string{"set-cookie", "X-Request"},
	}))

	app.Post("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{Name: "sid", Value: "abc"})
		c.Set(fiber.HeaderSetCookie2, "sid=abc; Version=1")
		c.Set("X-Request", "first")
		c.Set("X-Other", "dropped")
		return c.SendString("ok")
	})

	post := func() *http.Response {
		req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
		req.Header.Set(ConfigDefault.KeyHeader, validKey)
		resp, _ := do(app, req)
		return resp
	}
	first, replayed := post(), post()
	require.Equal(t, []string{"sid=abc; path=/; SameSite=Lax"}, first.Header.Values(fiber.HeaderSetCookie))
	require.Equal(t, first.Header.Values(fiber.HeaderSetCookie), replayed.Header.Values(fiber.HeaderSetCookie))
	require.Equal(t, "first", replayed.Header.Get("X-Request"))
	require.Empty(t, replayed.Header.Values(fiber.HeaderSetCookie2))
	require.Empty(t, replayed.Header.Values("X-Other"))
}

// Test_New_ReplayDropsStoredCookies pins that a cookie in an entry the storage
// already holds, written by a version that kept every header or under a
// configuration that named the cookie, is not replayed unless the
// configuration names it now.
func Test_New_ReplayDropsStoredCookies(t *testing.T) {
	t.Parallel()

	stored := func() *stubStorage {
		res := response{
			Headers: map[string][]string{
				fiber.HeaderSetCookie:  {"sid=ALICE-SESSION; path=/"},
				fiber.HeaderSetCookie2: {"sid=ALICE-SESSION; Version=1"},
				"X-Request":            {"alice"},
			},
			Body:       []byte("welcome alice"),
			StatusCode: fiber.StatusOK,
		}
		bs, err := res.MarshalMsg(nil)
		require.NoError(t, err)
		return &stubStorage{data: map[string][]byte{validKey: bs}}
	}
	replay := func(keep []string) *http.Response {
		app := fiber.New()
		app.Use(New(Config{Storage: stored(), Lock: &stubLock{}, KeepResponseHeaders: keep}))
		app.Post("/", func(_ fiber.Ctx) error { return errors.New("the handler must not run") })

		req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
		req.Header.Set(ConfigDefault.KeyHeader, validKey)
		resp, body := do(app, req)
		require.Equal(t, "welcome alice", body)
		require.Equal(t, "alice", resp.Header.Get("X-Request"))
		return resp
	}

	resp := replay(nil)
	require.Empty(t, resp.Header.Values(fiber.HeaderSetCookie))
	require.Empty(t, resp.Header.Values(fiber.HeaderSetCookie2))

	resp = replay([]string{"X-Request"})
	require.Empty(t, resp.Header.Values(fiber.HeaderSetCookie))
	require.Empty(t, resp.Header.Values(fiber.HeaderSetCookie2))

	resp = replay([]string{"X-Request", fiber.HeaderSetCookie})
	require.Equal(t, []string{"sid=ALICE-SESSION; path=/"}, resp.Header.Values(fiber.HeaderSetCookie))
	require.Empty(t, resp.Header.Values(fiber.HeaderSetCookie2))
}

// Test_New_KeyScope pins that a key is stored, replayed and locked per scope: a
// user who sends the key another user sent runs the handler for their own
// request and gets their own response, cookie included, and the storage holds
// neither the key nor the scope.
func Test_New_KeyScope(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{}
	l := &stubLock{}
	app.Use(New(Config{
		Storage:             s,
		Lock:                l,
		KeepResponseHeaders: []string{fiber.HeaderSetCookie},
		KeyScope:            func(c fiber.Ctx) string { return c.Get("X-User") },
	}))

	var count int
	app.Post("/", func(c fiber.Ctx) error {
		count++
		c.Cookie(&fiber.Cookie{Name: "sid", Value: c.Get("X-User") + "-session"})
		return c.SendString(fmt.Sprintf("%s:%d", c.Get("X-User"), count))
	})

	post := func(user string) (*http.Response, string) {
		req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
		req.Header.Set(ConfigDefault.KeyHeader, validKey)
		if user != "" {
			req.Header.Set("X-User", user)
		}
		return do(app, req)
	}
	requireReply := func(user, wantBody string, wantCalls int) {
		t.Helper()
		resp, body := post(user)
		require.Equal(t, wantBody, body)
		require.Equal(t, "sid="+user+"-session; path=/; SameSite=Lax", resp.Header.Get(fiber.HeaderSetCookie))
		require.Equal(t, wantCalls, count)
	}

	requireReply("alice", "alice:1", 1)
	requireReply("bob", "bob:2", 2) // the key is Alice's too, the response is not
	requireReply("alice", "alice:1", 2)
	requireReply("bob", "bob:2", 2)
	requireReply("", ":3", 3) // a request without a user is a scope of its own
	requireReply("", ":3", 3)

	require.Equal(t, 3, s.setCount)
	require.Len(t, s.data, 3)
	// Each entry holds its own user's cookie and nobody else's, under the key
	// the scope and the idempotency key digest to.
	for _, user := range []string{"alice", "bob", ""} {
		val := string(s.data[scopedKey(user, validKey)])
		require.Contains(t, val, "sid="+user+"-session")
		for _, other := range []string{"alice", "bob"} {
			if other != user {
				require.NotContains(t, val, other+"-session")
			}
		}
	}
	// A replay is answered before the lock, so only the three misses took it,
	// under the same keys, and a key two users share makes neither wait for
	// the other.
	require.Len(t, l.locked, 3)
	for _, key := range l.locked {
		require.Contains(t, s.data, key)
	}
}

// Test_New_KeyScope_NotConsulted pins that the scope is asked for only when a
// request carries a valid key.
func Test_New_KeyScope_NotConsulted(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	var asked int
	app.Use(New(Config{
		Storage:  &stubStorage{},
		Lock:     &stubLock{},
		KeyScope: func(fiber.Ctx) string { asked++; return "user" },
	}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	do(app, httptest.NewRequest(http.MethodPost, "/", http.NoBody))
	require.Zero(t, asked, "no key")

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, "bad")
	resp, _ := do(app, req)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Zero(t, asked, "invalid key")

	req = httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	do(app, req)
	require.Equal(t, 1, asked)
}

// Test_scopedKey pins that a scope and a key give one storage key, of one
// length, that no other pair gives, however their bytes run together.
func Test_scopedKey(t *testing.T) {
	t.Parallel()

	first := scopedKey("alice", validKey)
	require.Equal(t, first, scopedKey("alice", validKey))
	require.Len(t, scopedKey("", ""), 64)
	require.Len(t, scopedKey(strings.Repeat("s", 1000), strings.Repeat("k", 1000)), 64)
	require.NotEqual(t, validKey, scopedKey("", validKey))

	seen := make(map[string][2]string)
	for _, pair := range [][2]string{
		{"", ""},
		{"", "abc"},
		{"a", "bc"},
		{"ab", "c"},
		{"abc", ""},
		{"1", ":ab"},
		{"1:a", "b"},
		{"3", ":abc"},
		{"3:abc", ""},
		{"0", ":abc"},
		{"10", ":"},
		{"1", "0:"},
		{"alice", validKey},
		{"bob", validKey},
		{"alice" + validKey[:1], validKey[1:]},
	} {
		key := scopedKey(pair[0], pair[1])
		prev, dup := seen[key]
		require.False(t, dup, "%q and %q share a key", pair, prev)
		seen[key] = pair
	}
}

func Test_New_Cache_WhenBodyTooLarge(t *testing.T) {
	t.Parallel()
	bodyLimit := 8
	app := fiber.New(fiber.Config{BodyLimit: bodyLimit})
	s := &stubStorage{}
	var wasPut []bool

	app.Use(func(c fiber.Ctx) error {
		if err := c.Next(); err != nil {
			return err
		}
		wasPut = append(wasPut, WasPutToCache(c))
		return nil
	})
	app.Use(New(Config{Storage: s, Lock: &stubLock{}}))

	var count int
	oversized := strings.Repeat("a", bodyLimit+1)
	app.Post("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(oversized)
	})

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp1, body1 := do(app, req)
	require.Equal(t, fiber.StatusOK, resp1.StatusCode)
	require.Equal(t, oversized, body1)

	req2 := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req2.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp2, body2 := do(app, req2)
	require.Equal(t, fiber.StatusOK, resp2.StatusCode)
	require.Equal(t, oversized, body2)

	require.Equal(t, 1, count)
	require.Equal(t, 1, s.setCount)
	require.Len(t, wasPut, 2)
	require.True(t, wasPut[0])
	require.False(t, wasPut[1])
}

func Test_New_HandlerError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{}
	app.Use(New(Config{Storage: s, Lock: &stubLock{}}))
	app.Post("/", func(_ fiber.Ctx) error { return errors.New("boom") })

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "boom", body)
	require.Equal(t, 0, s.setCount)

	resp2, body2 := do(app, req)
	require.Equal(t, fiber.StatusInternalServerError, resp2.StatusCode)
	require.Equal(t, "boom", body2)
	require.Equal(t, 0, s.setCount)
}

func Test_New_LockError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	l := &stubLock{lockErr: errors.New("fail")}
	app.Use(New(Config{Lock: l, Storage: &stubStorage{}}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Contains(t, body, "failed to lock")
}

func Test_New_StorageSetError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{setErr: errors.New("nope")}
	app.Use(New(Config{Storage: s, Lock: &stubLock{}}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Contains(t, body, "failed to save response")
}

func Test_New_UnlockError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	l := &stubLock{unlockErr: errors.New("u")}
	app.Use(New(Config{Lock: l, Storage: &stubStorage{}}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "ok", body)
}

func Test_New_SecondPassReadError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{}
	l := &stubLock{afterLock: func() { s.getErr = errors.New("g") }}
	app.Use(New(Config{Lock: l, Storage: s}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Contains(t, body, "failed to write cached response while locked")
}

// The response is stored after the handler returns, so a handler's own deferred
// cancel must not reach storage through Ctx.Err().
func Test_Idempotency_Store_After_Handler_Cancel(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	var calls int
	app.Post("/", func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), time.Hour)
		defer cancel()
		c.SetContext(ctx)

		calls++
		return c.SendString("response")
	})

	newReq := func() *http.Request {
		req := httptest.NewRequest(fiber.MethodPost, "/", http.NoBody)
		req.Header.Set(ConfigDefault.KeyHeader, validKey)
		return req
	}

	_, body := do(app, newReq())
	require.Equal(t, "response", body)
	_, body = do(app, newReq())
	require.Equal(t, "response", body)
	require.Equal(t, 1, calls, "second request was not served from the idempotency cache")
}

// countingStream counts the reads of a response body stream.
type countingStream struct{ reads int }

func (s *countingStream) Read(_ []byte) (int, error) {
	s.reads++
	return 0, io.EOF
}

// Recording a stream read it to its end before the response was sent.
func Test_Idempotency_StreamIsPassedOnUnrecorded(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	var calls int
	var stream *countingStream
	app.Post("/", func(c fiber.Ctx) error {
		calls++
		stream = &countingStream{}
		return c.SendStream(stream)
	})

	for range 2 {
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.SetMethod(fiber.MethodPost)
		fctx.Request.SetRequestURI("/")
		fctx.Request.Header.Set(ConfigDefault.KeyHeader, validKey)
		app.Handler()(fctx)

		require.True(t, fctx.Response.IsBodyStream())
		require.Zero(t, stream.reads)
	}
	require.Equal(t, 2, calls)
}

// A refused request replays the Use chain without its handler; storing what it
// left answered the retry with an empty 413 for the whole Lifetime.
func Test_Idempotency_RefusedRequestIsNotStored(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{BodyLimit: 8})
	s := &stubStorage{}
	app.Use(New(Config{Storage: s, Lock: &stubLock{}}))

	var calls int
	app.Post("/", func(c fiber.Ctx) error {
		calls++
		return c.SendString("ok")
	})

	refused := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader(strings.Repeat("x", 64)))
	refused.Header.Set(ConfigDefault.KeyHeader, validKey)
	_, err := app.Test(refused)
	require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
	require.Zero(t, s.setCount)

	retry := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader("ok"))
	retry.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, retry)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "ok", body)
	require.Equal(t, 1, calls)
}
