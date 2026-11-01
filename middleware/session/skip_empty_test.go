package session

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// addSessionRoutes registers the routes that take a session through its life: /login stores a
// user and answers with the ID, /read answers with the user, /regenerate answers with the new
// ID and /destroy ends the session.
func addSessionRoutes(app *fiber.App) {
	app.Get("/login", func(c fiber.Ctx) error {
		FromContext(c).Set("user", "alice")
		return c.SendString(FromContext(c).ID())
	})
	app.Get("/read", func(c fiber.Ctx) error {
		return c.SendString(fmt.Sprint(FromContext(c).Get("user")))
	})
	app.Get("/regenerate", func(c fiber.Ctx) error {
		if err := FromContext(c).Regenerate(); err != nil {
			return err
		}
		return c.SendString(FromContext(c).ID())
	})
	app.Get("/destroy", func(c fiber.Ctx) error { return FromContext(c).Destroy() })
}

// skipEmptyApp builds an app whose routes exercise every way a request can
// leave a session behind.
func skipEmptyApp(t *testing.T, cfg *Config) (*fiber.App, *memory.Storage, *recordingStorage) {
	t.Helper()

	mem := memory.New()
	rec := &recordingStorage{Storage: mem}
	cfg.Storage = rec

	app := fiber.New()
	app.Use(New(*cfg))
	addSessionRoutes(app)
	app.Get("/health", func(c fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/touch", func(c fiber.Ctx) error {
		return c.SendString(strconv.FormatBool(FromContext(c).Fresh()))
	})
	app.Get("/logout", func(c fiber.Ctx) error { return FromContext(c).Reset() })
	app.Get("/fail-empty", func(_ fiber.Ctx) error { return fiber.ErrTeapot })
	app.Get("/fail-with-data", func(c fiber.Ctx) error {
		FromContext(c).Set("k", "v")
		return fiber.ErrTeapot
	})
	return app, mem, rec
}

func storedSessions(t *testing.T, mem *memory.Storage) int {
	t.Helper()
	keys, err := mem.Keys()
	require.NoError(t, err)
	return len(keys)
}

func Test_Session_SkipEmptySessions(t *testing.T) {
	t.Parallel()

	t.Run("off by default: every anonymous request stores a session and sets a cookie", func(t *testing.T) {
		t.Parallel()
		app, mem, _ := skipEmptyApp(t, &Config{})

		for range 5 {
			resp, _ := sendRequest(t, app, "/health", nil)
			require.Contains(t, cookiesByName(resp), "session_id")
		}
		require.Equal(t, 5, storedSessions(t, mem))
	})

	t.Run("a visitor who never uses the session leaves nothing behind", func(t *testing.T) {
		t.Parallel()
		app, mem, rec := skipEmptyApp(t, &Config{SkipEmptySessions: true})

		for range 20 {
			resp, body := sendRequest(t, app, "/health", nil)
			require.Equal(t, "ok", body)
			require.Empty(t, resp.Cookies(), "no cookie for a session that is not kept")
		}
		require.Zero(t, storedSessions(t, mem))
		require.Empty(t, rec.writes())

		// Looking at the session is not using it.
		resp, body := sendRequest(t, app, "/touch", nil)
		require.Equal(t, "true", body, "the session is still new, as always")
		require.Empty(t, resp.Cookies())
		require.Zero(t, storedSessions(t, mem))

		// Nor is a cookie for a session the storage no longer has.
		resp, _ = sendRequest(t, app, "/health", map[string]string{"session_id": "expired-or-forged"})
		require.Empty(t, resp.Cookies(), "an ID the visitor made up is not turned into a session")
		require.Zero(t, storedSessions(t, mem))
	})

	t.Run("a session that has data is kept: stored, refreshed on every request, and regenerated with its data", func(t *testing.T) {
		t.Parallel()
		app, mem, rec := skipEmptyApp(t, &Config{SkipEmptySessions: true})

		resp, id := sendRequest(t, app, "/login", nil)
		cookies := cookiesByName(resp)
		require.Contains(t, cookies, "session_id")
		require.Equal(t, id, cookies["session_id"].Value)
		require.Equal(t, 1, storedSessions(t, mem))
		require.Len(t, rec.writes(), 1)

		resp, _ = sendRequest(t, app, "/health", map[string]string{"session_id": id})
		require.Len(t, rec.writes(), 2, "the idle timeout must keep sliding for a session that is in use")
		require.Contains(t, cookiesByName(resp), "session_id")
		require.Equal(t, 1, storedSessions(t, mem))

		_, body := sendRequest(t, app, "/read", map[string]string{"session_id": id})
		require.Equal(t, "alice", body)

		resp, newID := sendRequest(t, app, "/regenerate", map[string]string{"session_id": id})
		require.NotEqual(t, id, newID)
		require.Equal(t, newID, cookiesByName(resp)["session_id"].Value)
		require.Equal(t, 1, storedSessions(t, mem))

		_, body = sendRequest(t, app, "/read", map[string]string{"session_id": newID})
		require.Equal(t, "alice", body)
		_, body = sendRequest(t, app, "/read", map[string]string{"session_id": id})
		require.Equal(t, "<nil>", body, "the old ID no longer resolves")
	})

	t.Run("a logout expires the cookie and a destroy removes the record, and neither stores a replacement", func(t *testing.T) {
		t.Parallel()
		app, mem, _ := skipEmptyApp(t, &Config{SkipEmptySessions: true})

		_, id := sendRequest(t, app, "/login", nil)
		require.Equal(t, 1, storedSessions(t, mem))

		resp, _ := sendRequest(t, app, "/logout", map[string]string{"session_id": id})
		require.Zero(t, storedSessions(t, mem), "the old record is gone and no empty one took its place")
		cookies := cookiesByName(resp)
		require.Contains(t, cookies, "session_id")
		require.Negative(t, cookies["session_id"].MaxAge, "the only cookie sent expires the old one")
		require.Empty(t, cookies["session_id"].Value)

		resp, _ = sendRequest(t, app, "/logout", map[string]string{"session_id": id})
		require.Zero(t, storedSessions(t, mem))
		require.NotEmpty(t, resp.Cookies())

		_, id = sendRequest(t, app, "/login", nil)
		_, _ = sendRequest(t, app, "/destroy", map[string]string{"session_id": id})
		require.Zero(t, storedSessions(t, mem))
	})

	t.Run("without the option a logout leaves an empty session behind", func(t *testing.T) {
		t.Parallel()
		app, mem, _ := skipEmptyApp(t, &Config{})

		_, id := sendRequest(t, app, "/login", nil)
		resp, _ := sendRequest(t, app, "/logout", map[string]string{"session_id": id})
		require.Equal(t, 1, storedSessions(t, mem))
		require.Positive(t, cookiesByName(resp)["session_id"].MaxAge)
	})

	t.Run("a failing handler decides nothing: data is saved, an empty session is not", func(t *testing.T) {
		t.Parallel()
		app, mem, _ := skipEmptyApp(t, &Config{SkipEmptySessions: true})

		resp, _ := sendRequest(t, app, "/fail-empty", nil)
		require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
		require.Empty(t, resp.Cookies())
		require.Zero(t, storedSessions(t, mem))

		resp, _ = sendRequest(t, app, "/fail-with-data", nil)
		require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
		require.Contains(t, cookiesByName(resp), "session_id")
		require.Equal(t, 1, storedSessions(t, mem))
	})

	t.Run("the absolute timeout bookkeeping is not data", func(t *testing.T) {
		t.Parallel()
		app, mem, _ := skipEmptyApp(t, &Config{SkipEmptySessions: true, AbsoluteTimeout: time.Hour})

		resp, _ := sendRequest(t, app, "/health", nil)
		require.Empty(t, resp.Cookies())
		require.Zero(t, storedSessions(t, mem))

		resp, id := sendRequest(t, app, "/login", nil)
		require.Equal(t, id, cookiesByName(resp)["session_id"].Value)
		require.Equal(t, 1, storedSessions(t, mem))
	})

	t.Run("the store pattern saves what the handler tells it to", func(t *testing.T) {
		t.Parallel()
		mem := memory.New()
		store := NewStore(Config{Storage: mem, SkipEmptySessions: true})
		seedSession(t, fiber.New(), store, func(*Session) {}) // gets the session, stores nothing in it and saves it
		require.Equal(t, 1, storedSessions(t, mem), "an explicit Save is the caller's decision")
	})
}

func Test_Session_IsEmptyFresh(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		prepare func(*Session)
		name    string
		empty   bool
	}{
		{name: "new and empty", prepare: func(*Session) {}, empty: true},
		{name: "only the absolute timeout marker", prepare: func(s *Session) { s.setAbsExpiration(time.Now().Add(time.Hour)) }, empty: true},
		{name: "holds a value", prepare: func(s *Session) { s.Set("k", "v") }},
		{name: "a value that was removed again leaves nothing", empty: true, prepare: func(s *Session) {
			s.Set("k", "v")
			s.Delete("k")
		}},
		{name: "not new", prepare: func(s *Session) {
			s.mu.Lock()
			s.isFresh = false
			s.mu.Unlock()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sess := newSessionWithStorage(t, memory.New())
			tc.prepare(sess)
			require.Equal(t, tc.empty, sess.isEmptyFresh())
		})
	}

	t.Run("a session without a data map is never skipped", func(t *testing.T) {
		t.Parallel()
		require.False(t, (&Session{isFresh: true}).isEmptyFresh())
	})
}

// Benchmark_Middleware_NewVisitor is a request from a client that sends no
// cookie and never uses the session, such as a health check. With
// SkipEmptySessions it stores nothing and sets no cookie.
func Benchmark_Middleware_NewVisitor(b *testing.B) {
	for _, skip := range []bool{false, true} {
		b.Run("SkipEmptySessions="+strconv.FormatBool(skip), func(b *testing.B) {
			app := fiber.New()
			app.Use(New(Config{SkipEmptySessions: skip}))
			app.Get("/", func(_ fiber.Ctx) error { return nil })
			handler := app.Handler()

			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(fiber.MethodGet)
			fctx.Request.SetRequestURI("/")

			b.ReportAllocs()
			for b.Loop() {
				fctx.ResetUserValues()
				fctx.Response.Reset()
				handler(fctx)
			}
		})
	}
}
