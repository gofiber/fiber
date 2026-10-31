package session

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/stretchr/testify/require"
)

// A store asked twice through Store.Get hands out one session: the second Get takes
// the fresh ID the first one cached and finds the record the first one saved. The
// provenance box of the request may still name a store over another cookie, which
// found its session before: its box describes that ID, not the fresh one, and has no
// say over it.
func Test_Store_Get_TakesItsOwnFreshIDWhateverTheBoxSays(t *testing.T) {
	t.Parallel()

	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})
	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})

	app := fiber.New()
	app.Get("/seed", func(c fiber.Ctx) error {
		b, err := storeB.Get(c)
		if err != nil {
			return err
		}
		defer b.Release()
		if err := b.Save(); err != nil {
			return err
		}
		return c.SendString(b.ID())
	})
	app.Get("/", func(c fiber.Ctx) error {
		b, err := storeB.Get(c) // found: the box of the request names B
		if err != nil {
			return err
		}
		defer b.Release()
		first, err := storeA.Get(c)
		if err != nil {
			return err
		}
		defer first.Release()
		first.Set("flash", "hello")
		if saveErr := first.Save(); saveErr != nil {
			return saveErr
		}
		again, err := storeA.Get(c)
		if err != nil {
			return err
		}
		defer again.Release()
		return c.SendString(fmt.Sprintf("%t|%t|%v|%s", b.Fresh(), again.ID() == first.ID(), again.Get("flash"), again.ID()))
	})

	_, idB := sendRequest(t, app, "/seed", nil)
	resp, body := sendRequest(t, app, "/", map[string]string{"b_sid": idB})
	require.Greater(t, len(body), len("false|true|hello|"), body)
	idA := body[len("false|true|hello|"):]
	require.Equal(t, "false|true|hello|"+idA, body, "the second Get of A did not find the session the first one saved")
	cookies := cookiesByName(resp)
	require.NotNil(t, cookies["a_sid"], "A's session goes to A's cookie")
	require.Equal(t, idA, cookies["a_sid"].Value, "A's cookie carries the one session of A")
	require.Nil(t, cookies["b_sid"], "B's session was found and not saved, so B's cookie is not written")
}

// A store writes a session ID back only to its own cookies and headers. A store that
// shares the ID another store cached, and reads no cookie or header itself, or not
// the one the ID came from, must not write it, or the new one of a Regenerate, to
// that store's cookie, with its own attributes.
func Test_Session_MultipleStores_AdoptedIDIsNotWrittenToAnotherStoresCookie(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		innerCookie  string // the cookie the inner store writes its new ID to, if any
		outer, inner extractors.Extractor
	}{
		"a store without cookies or headers": {
			outer: extractors.FromCookie("sid"),
			inner: extractors.FromQuery("qsid"),
		},
		"a store over the other cookie of a chain": {
			outer:       extractors.Chain(extractors.FromCookie("sid"), extractors.FromCookie("shared")),
			inner:       extractors.FromCookie("shared"),
			innerCookie: "shared",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			storage := memory.New()
			outerHandler, outerStore := NewWithStore(Config{Storage: storage, Extractor: tc.outer, CookieSecure: true, CookieHTTPOnly: true})
			innerHandler, innerStore := NewWithStore(Config{Storage: storage, Extractor: tc.inner})

			app := fiber.New()
			app.Use(outerHandler, innerHandler)
			app.Get("/seed", func(c fiber.Ctx) error {
				outerStore.FromContext(c).Set("user", "alice")
				return c.SendString(outerStore.FromContext(c).ID())
			})
			app.Get("/regen", func(c fiber.Ctx) error {
				m := innerStore.FromContext(c)
				if err := m.Regenerate(); err != nil {
					return err
				}
				return c.SendString(m.ID())
			})

			_, id := sendRequest(t, app, "/seed", nil)
			resp, newID := sendRequest(t, app, "/regen", map[string]string{"sid": id})
			require.NotEqual(t, id, newID, "the inner store shares the ID, and rotated it")
			for _, cookie := range resp.Cookies() {
				if cookie.Name != "sid" {
					continue
				}
				require.NotEqual(t, newID, cookie.Value, "the inner store wrote its ID to the outer store's cookie")
				require.True(t, cookie.Secure && cookie.HttpOnly, "the outer store's cookie was written with the inner store's attributes")
			}
			if tc.innerCookie != "" {
				require.Equal(t, newID, cookiesByName(resp)[tc.innerCookie].Value, "the new ID goes to the inner store's own cookie")
			}
		})
	}
}

// A session stored with an absolute deadline keeps it on a store without an
// AbsoluteTimeout, as after the option was turned off: once it has passed, the
// request path starts the session over, once, and without a deadline, where a
// deadline of now would start it over on every request; and GetByID no longer finds
// it, as on a store with the option.
func Test_Session_DeadlineHonouredWithoutAbsoluteTimeout(t *testing.T) {
	t.Parallel()

	past := time.Now().Add(-time.Minute)
	for name, tc := range map[string]struct {
		codec Codec
		seed  func(t *testing.T, mem *memory.Storage, key string)
	}{
		"record": {
			codec: JSONCodec{},
			seed: func(t *testing.T, mem *memory.Storage, key string) {
				t.Helper()
				raw, err := appendRecord(nil, "json", recordDeadline{sec: past.Unix(), nanos: uint32(past.Nanosecond()), set: true},
					[]recordEntry{{key: "k", value: []byte(`"v"`)}})
				require.NoError(t, err)
				require.NoError(t, mem.Set(key, raw, time.Hour))
			},
		},
		"gob": {
			seed: func(t *testing.T, mem *memory.Storage, key string) {
				t.Helper()
				gob.Register(absExpirationKey)
				gob.Register(time.Time{})
				var buf bytes.Buffer
				require.NoError(t, gob.NewEncoder(&buf).Encode(map[any]any{absExpirationKey: past, "k": "v"}))
				require.NoError(t, mem.Set(key, buf.Bytes(), time.Hour))
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mem := memory.New()
			store := NewStore(Config{Storage: mem, Codec: tc.codec})
			tc.seed(t, mem, store.storageKey("by-id"))
			tc.seed(t, mem, store.storageKey("by-request"))

			_, err := store.GetByID(context.Background(), "by-id")
			require.ErrorIs(t, err, ErrSessionIDNotFoundInStore, "GetByID handed out a session past its deadline")
			stored, err := mem.Get(store.storageKey("by-id"))
			require.NoError(t, err)
			require.Nil(t, stored, "and left its record")

			app := fiber.New()
			app.Use(New(Config{Store: store}))
			app.Get("/", func(c fiber.Ctx) error {
				m := FromContext(c)
				_, hasDeadline := m.AbsoluteExpiry()
				m.Set("login", "done")
				return c.SendString(fmt.Sprintf("%t|%t|%v", m.Fresh(), hasDeadline, m.Get("k")))
			})
			resp, body := sendRequest(t, app, "/", map[string]string{"session_id": "by-request"})
			require.Equal(t, "true|false|<nil>", body, "the session past its deadline was not started over, or was given a deadline")
			id := cookiesByName(resp)["session_id"].Value
			require.NotEqual(t, "by-request", id)

			time.Sleep(2 * time.Millisecond)
			_, body = sendRequest(t, app, "/", map[string]string{"session_id": id})
			require.Equal(t, "false|false|<nil>", body, "the session that took its place was started over again")
		})
	}
}

// A panic that unwinds the session middleware skips its release. The middleware and
// its session are cut off from the request, which whatever recovers the panic
// answers and hands back to the pool: a save by a goroutine of the handler that
// still holds the session writes its record, and no cookie, into no request. A
// context kept past the request does not find the middleware. The middleware does
// not save the session.
func Test_Session_Middleware_UnwoundByAPanicLetsGoOfTheRequest(t *testing.T) {
	t.Parallel()

	t.Run("recovered in front of the middleware", func(t *testing.T) {
		t.Parallel()

		var (
			held atomic.Pointer[Middleware]
			kept atomic.Value // context.Context
		)
		store := NewStore()
		app := fiber.New(fiber.Config{PassLocalsToContext: true})
		app.Use(recoverer.New())
		app.Use(New(Config{Store: store}))
		app.Get("/", func(c fiber.Ctx) error {
			m := FromContext(c)
			m.Set("k", "v")
			held.Store(m)
			kept.Store(c.Context())
			panic("boom")
		})

		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
		require.Empty(t, resp.Cookies(), "the middleware set a cookie for a request that panicked")

		require.Nil(t, FromContext(kept.Load()), "a context kept past the request still finds the middleware the panic unwound")
		m := held.Load()
		m.mu.RLock()
		ctx, sess := m.ctx, m.Session
		m.mu.RUnlock()
		require.Nil(t, ctx, "the middleware still refers to the request")
		require.NotNil(t, sess, "its session was recycled")
		require.Nil(t, sess.requestCtx(), "the session still refers to the request")

		key := store.storageKey(sess.ID())
		stored, err := store.Storage.GetWithContext(context.Background(), key)
		require.NoError(t, err)
		require.Nil(t, stored, "the middleware saved the session of a request that panicked")
		require.NoError(t, sess.Save(), "a save by the handler's own goroutine")
		stored, err = store.Storage.GetWithContext(context.Background(), key)
		require.NoError(t, err)
		require.NotEmpty(t, stored, "writes the record")
	})

	t.Run("timed out, then panics", func(t *testing.T) {
		t.Parallel()

		var (
			held    = make(chan *Middleware, 1)
			release = make(chan struct{})
			done    = make(chan struct{})
		)
		store := NewStore()
		app := fiber.New()
		app.Use(timeout.New(func(c fiber.Ctx) error { return c.Next() }, timeout.Config{Timeout: 10 * time.Millisecond}))
		app.Use(New(Config{Store: store}))
		app.Get("/slow", func(c fiber.Ctx) error {
			defer close(done)
			held <- FromContext(c)
			<-release
			panic("boom")
		})

		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/slow", http.NoBody), fiber.TestConfig{Timeout: 5 * time.Second})
		require.NoError(t, err)
		require.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)

		m := <-held
		close(release)
		<-done
		require.Eventually(t, func() bool {
			m.mu.RLock()
			sess, ctx := m.Session, m.ctx
			m.mu.RUnlock()
			return sess != nil && ctx == nil && sess.requestCtx() == nil
		}, 5*time.Second, 5*time.Millisecond, "the middleware and its session still refer to the request")
	})
}

// A custom Ctx that panics while the release clears the locals leaves the
// middleware in the request for whatever recovers the panic, and the lock of the
// middleware free: the recovering middleware can still ask it for the session.
func Test_Session_Middleware_ReleasePanicLeavesTheLockFree(t *testing.T) {
	t.Parallel()

	armed := &atomic.Value{}
	armed.Store("")
	app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
		return &panicAtCtx{DefaultCtx: *fiber.NewDefaultCtx(app), point: armed}
	})
	keys := make(chan []any, 1)
	app.Use(func(c fiber.Ctx) error {
		defer func() {
			if recover() == nil {
				return
			}
			m := FromContext(c)
			if m == nil {
				keys <- nil
				return
			}
			got := make(chan []any, 1)
			go func() { got <- m.Keys() }()
			select {
			case k := <-got:
				keys <- k
			case <-time.After(2 * time.Second):
				keys <- []any{"hung"}
			}
		}()
		return c.Next()
	})
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		FromContext(c).Set("k", "v")
		armed.Store("clearing the locals")
		return nil
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, []any{"k"}, <-keys, "the recovering middleware could not read the session: the release left the lock held")
}

// Once the session middleware has finished with the request, a Store.Get reads the
// session ID from the request again, and finds the session of the cookie.
func Test_Store_Get_AfterTheMiddlewareEndedReadsTheRequest(t *testing.T) {
	t.Parallel()

	store := NewStore()
	var after atomic.Value // string
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		err := c.Next()
		sess, getErr := store.Get(c)
		if getErr != nil {
			return getErr
		}
		defer sess.Release()
		after.Store(fmt.Sprintf("%t|%s", sess.Fresh(), sess.ID()))
		return err
	})
	app.Use(New(Config{Store: store}))
	app.Get("/", func(c fiber.Ctx) error {
		FromContext(c).Set("k", "v")
		return c.SendString(FromContext(c).ID())
	})

	_, id := sendRequest(t, app, "/", nil)
	sendRequest(t, app, "/", map[string]string{"session_id": id})
	require.Equal(t, "false|"+id, after.Load(), "Store.Get after the middleware ended started a new session instead of reading the cookie")
}
