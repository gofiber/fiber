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

// A store writes a session ID back only to its own cookies and headers. A store that shares the ID
// another store cached, but not the cookie it came from (a store over the second cookie of a
// chain), must not write it, or the new one of a Regenerate, to that cookie, with its own
// attributes.
func Test_Session_MultipleStores_AdoptedIDIsNotWrittenToAnotherStoresCookie(t *testing.T) {
	t.Parallel()

	storage := memory.New()
	outerHandler, outerStore := NewWithStore(Config{
		Storage:      storage,
		Extractor:    extractors.Chain(extractors.FromCookie("sid"), extractors.FromCookie("shared")),
		CookieSecure: true, CookieHTTPOnly: true,
	})
	innerHandler, innerStore := NewWithStore(Config{Storage: storage, Extractor: extractors.FromCookie("shared")})

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
	require.Equal(t, newID, cookiesByName(resp)["shared"].Value, "the new ID goes to the inner store's own cookie")
}

// A session stored with an absolute deadline keeps it on a store without an AbsoluteTimeout, as
// after the option was turned off: once it has passed, the request path starts the session over,
// once, and without a deadline, where a deadline of now would start it over on every request; and
// GetByID no longer finds it, as on a store with the option.
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

// A panic that unwinds the session middleware skips its release. The middleware and its session let
// go of the fiber.Ctx, which whatever recovers the panic answers and hands back to the pool, and
// keep the request itself until fasthttp resets it: the recovering middleware still sets cookies
// through them, and a save by a goroutine of the handler that still holds the session afterwards
// writes its record, and no cookie, into no request. A context kept past the request does not find
// the middleware. The middleware does not save the session.
func Test_Session_Middleware_UnwoundByAPanicLetsGoOfTheRequest(t *testing.T) {
	t.Parallel()

	t.Run("recovered in front of the middleware", func(t *testing.T) {
		t.Parallel()

		var (
			held  atomic.Pointer[Middleware]
			kept  atomic.Value // context.Context
			after atomic.Value // string: what the middleware in front of the recovering one could do
		)
		store := NewStore()
		app := fiber.New(fiber.Config{PassLocalsToContext: true})
		app.Use(func(c fiber.Ctx) error {
			err := c.Next()
			// The panic was recovered behind this point: the middleware is still the request's to
			// reach, and a Destroy from here deletes the record and expires the cookie, as before
			// the panic.
			after.Store("unreachable")
			if m := FromContext(c); m != nil {
				after.Store(fmt.Sprintf("reached|destroy=%v", m.Destroy()))
			}
			return err
		}, recoverer.New(), New(Config{Store: store}))
		app.Get("/", func(c fiber.Ctx) error {
			m := FromContext(c)
			m.Set("k", "v")
			held.Store(m)
			kept.Store(c.Context())
			panic("boom")
		})

		resp, _ := sendRequest(t, app, "/", nil)
		require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
		require.Equal(t, "reached|destroy=<nil>", after.Load(), "the recovering side could not reach the middleware")
		cookies := cookiesByName(resp)
		require.Len(t, cookies, 1, "the Destroy of the recovering side set no cookie, or the middleware set one")
		require.NotNil(t, cookies["session_id"])
		require.Negative(t, cookies["session_id"].MaxAge, "the Destroy of the recovering side did not expire the cookie")

		require.Nil(t, FromContext(kept.Load()), "a context kept past the request still finds the middleware the panic unwound")
		m := held.Load()
		m.mu.RLock()
		ctx, sess := m.ctx, m.Session
		m.mu.RUnlock()
		require.Nil(t, ctx, "the middleware still refers to the fiber.Ctx")
		require.NotNil(t, sess, "its session was recycled")
		require.Nil(t, sess.requestCtx(), "the session still refers to the fiber.Ctx")
		// fasthttp resets the request once the response is written, and closes the middleware then:
		// the session has nothing to write to from then on.
		require.Eventually(t, sess.holdsNoRequest, 5*time.Second, 5*time.Millisecond, "the session still writes to the request after it ended")

		require.Nil(t, storedFor(t, store, sess.ID()), "the middleware saved the session of a request that panicked")
		require.NoError(t, sess.Save(), "a save by the handler's own goroutine")
		require.NotEmpty(t, storedFor(t, store, sess.ID()), "writes the record")
	})

	t.Run("a save from the recovering side saves the session and sets its cookie", func(t *testing.T) {
		t.Parallel()

		var (
			id    atomic.Value          // string
			saved atomic.Pointer[error] // the result of the save
		)
		store := NewStore()
		app := fiber.New()
		app.Use(func(c fiber.Ctx) error {
			err := c.Next()
			// The middleware does not save the session of a request that panicked, and is not in
			// charge of it any more: a Save from here does what a Save does for a session that no
			// middleware manages, where it would do nothing while the middleware was.
			if m := FromContext(c); m != nil {
				saveErr := m.Session.Save()
				saved.Store(&saveErr)
			}
			return err
		}, recoverer.New(), New(Config{Store: store}))
		app.Get("/", func(c fiber.Ctx) error {
			m := FromContext(c)
			m.Set("k", "v")
			id.Store(m.ID())
			panic("boom")
		})

		resp, _ := sendRequest(t, app, "/", nil)
		require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
		result := saved.Load()
		require.NotNil(t, result, "the recovering side could not reach the middleware")
		require.NoError(t, *result)

		cookie := cookiesByName(resp)["session_id"]
		require.NotNil(t, cookie, "the save of the recovering side set no cookie")
		require.Equal(t, id.Load(), cookie.Value)
		require.NotEmpty(t, storedFor(t, store, cookie.Value), "the save of the recovering side wrote no record")
	})

	t.Run("two middleware unwound by one panic", func(t *testing.T) {
		t.Parallel()

		var held [2]atomic.Pointer[Middleware]
		handlerA, storeA := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
		handlerB, storeB := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})
		app := fiber.New()
		app.Use(recoverer.New(), handlerA, handlerB)
		app.Get("/", func(c fiber.Ctx) error {
			held[0].Store(storeA.FromContext(c))
			held[1].Store(storeB.FromContext(c))
			panic("boom")
		})

		resp, _ := sendRequest(t, app, "/", nil)
		require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

		// fasthttp resets the request once the response is written, and closes every middleware the
		// panic unwound then, the outer one and the inner one.
		for i := range held {
			m := held[i].Load()
			require.NotNil(t, m)
			m.mu.RLock()
			sess := m.Session
			m.mu.RUnlock()
			require.NotNil(t, sess)
			require.Eventually(t, sess.holdsNoRequest, 5*time.Second, 5*time.Millisecond, "the session of middleware %d still writes to the request after it ended", i)
		}
	})

	t.Run("timed out, then panics", func(t *testing.T) {
		t.Parallel()

		var (
			held    = make(chan *Middleware, 1)
			release = make(chan struct{})
			done    = make(chan struct{})
		)
		app := fiber.New()
		app.Use(timeout.New(func(c fiber.Ctx) error { return c.Next() }, timeout.Config{Timeout: 10 * time.Millisecond}), New())
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
		requireDetached(t, m)
	})
}

// A custom Ctx that panics while the release clears the locals leaves the middleware in the request
// for whatever recovers the panic, and the lock of the middleware free: the recovering middleware
// can still ask it for the session.
func Test_Session_Middleware_ReleasePanicLeavesTheLockFree(t *testing.T) {
	t.Parallel()

	armed := &atomic.Value{}
	app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
		return &panicAtCtx{DefaultCtx: *fiber.NewDefaultCtx(app), point: armed}
	})
	keys := make(chan []any, 1)
	app.Use(func(c fiber.Ctx) error {
		defer func() {
			if recover() == nil {
				return
			}
			if m := FromContext(c); m != nil {
				go func() { keys <- m.Keys() }()
			}
		}()
		return c.Next()
	}, New())
	app.Get("/", func(c fiber.Ctx) error {
		FromContext(c).Set("k", "v")
		armed.Store("clearing the locals")
		return nil
	})

	sendRequest(t, app, "/", nil)
	var got []any
	select {
	case got = <-keys:
	case <-time.After(2 * time.Second): // the lock of the middleware is held
	}
	require.Equal(t, []any{"k"}, got, "the recovering middleware could not read the session: the release left the lock held")
}

// Once the session middleware has finished with the request, a Store.Get reads the session ID from
// the request again, and finds the session of the cookie.
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

// A save reads the storage again only when its shard changed since the load. A shard that was
// changed before the load costs the request nothing: the load records the generation it found.
//
// Not parallel: every test bumps the generations of the shards, which are shared, and a bump
// between the load and the save would make the read legitimate.
func Test_Session_Save_ReadsTheStorageOnlyIfTheShardChangedSinceTheLoad(t *testing.T) {
	app := fiber.New()
	logged := &keyLogStorage{Storage: memory.New()}
	store := NewStore(Config{Storage: logged})
	id := seedSession(t, app, store, func(sess *Session) { sess.Set("user", "alice") })
	sibling := store.KeyGenerator()
	for lockFor(keyNamespace+sibling) != lockFor(keyNamespace+id) {
		sibling = store.KeyGenerator()
	}
	require.NoError(t, store.Delete(t.Context(), sibling))

	sess := loadSession(t, app, store, id)
	loaded := len(logged.seen())
	require.NoError(t, sess.Save())
	require.Equal(t, []string{"set:" + keyNamespace + id}, logged.seen()[loaded:], "the save only writes")
}
