package session

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
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
