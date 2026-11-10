package session

import (
	"bytes"
	"cmp"
	"context"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/loggertest"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/internal/storagetest"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// codecProfile is the struct the codec tests store: every kind of field a codec has to carry.
type codecProfile struct {
	At    time.Time         `json:"at"`
	Prefs map[string]string `json:"prefs"`
	Name  string            `json:"name"`
	Roles []string          `json:"roles"`
	ID    int               `json:"id"`
}

// newCodecProfile is a codecProfile with every field set.
func newCodecProfile() codecProfile {
	return codecProfile{
		ID:    42,
		Name:  "alice",
		Roles: []string{"admin", "dev"},
		Prefs: map[string]string{"theme": "dark"},
		At:    time.Date(2026, time.October, 29, 12, 0, 0, 0, time.UTC),
	}
}

// countingCodec counts the calls the session makes to its codec, and the questions about its name.
type countingCodec struct {
	Codec
	marshals atomic.Int64
	decodes  atomic.Int64
	names    atomic.Int64
}

func newCountingCodec() *countingCodec { return &countingCodec{Codec: JSONCodec{}} }

func (c *countingCodec) Name() string {
	c.names.Add(1)
	return c.Codec.Name()
}

func (c *countingCodec) Marshal(v any) ([]byte, error) {
	c.marshals.Add(1)
	return c.Codec.Marshal(v)
}

func (c *countingCodec) Unmarshal(data []byte, v any) error {
	c.decodes.Add(1)
	return c.Codec.Unmarshal(data, v)
}

// reusingCodec hands out the same buffer for every value it encodes, as a codec built on a pooled
// buffer may.
type reusingCodec struct {
	buf []byte
}

func (*reusingCodec) Name() string { return "reusing" }

func (c *reusingCodec) Marshal(v any) ([]byte, error) {
	encoded, err := json.Marshal(v)
	if err != nil {
		return nil, err //nolint:wrapcheck // the error is encoding/json's own
	}
	c.buf = append(c.buf[:0], encoded...)
	return c.buf, nil
}

func (*reusingCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) } //nolint:wrapcheck // the error is encoding/json's own

// sessionKey is the key under which a store without a prefix keeps the session with this ID.
func sessionKey(id string) string { return keyNamespace + id }

// storedBytes returns what is stored under id.
func storedBytes(t *testing.T, storage fiber.Storage, id string) []byte {
	t.Helper()
	raw, err := storage.Get(sessionKey(id))
	require.NoError(t, err)
	return raw
}

// storedRecord returns the record stored under id.
func storedRecord(t *testing.T, storage fiber.Storage, id string) (recordHeader, []recordEntry) {
	t.Helper()
	raw := storedBytes(t, storage, id)
	require.True(t, isRecord(raw), "stored: %q", raw)
	header, entries, err := collectRecord(raw)
	require.NoError(t, err)
	return header, entries
}

// seedAndLoad stores the session that fill makes in store, and returns it as the next request loads it.
func seedAndLoad(t *testing.T, store *Store, fill func(*Session)) *Session {
	t.Helper()
	app := fiber.New()
	return loadSession(t, app, store, seedSession(t, app, store, fill))
}

// mustGetAs returns the value of sess under key as a V, and fails the test if there is none.
func mustGetAs[V any](t *testing.T, sess *Session, key string) V {
	t.Helper()
	value, ok := GetAs[V](sess, key)
	require.True(t, ok, key)
	return value
}

// finishes runs f on a goroutine of its own, and fails the test with failure if f takes more than
// ten seconds, as a call that waits for a lock that was never given back does.
func finishes(t *testing.T, f func(), failure string) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	apptest.Recv(t, done, 10*time.Second, failure)
}

// sessionOrMiddleware is a *Session or a *Middleware, which are read in the same ways.
type sessionOrMiddleware interface {
	*Session | *Middleware
	Get(key any) any
	GetInto(key, target any) error
}

// readsOf returns the ways to read key from src: Get, GetAs and GetInto.
func readsOf[R sessionOrMiddleware](src R, key any) map[string]func() {
	return map[string]func(){
		"Get":     func() { _ = src.Get(key) },
		"GetAs":   func() { _, _ = GetAs[string](src, key) },
		"GetInto": func() { var out string; _ = src.GetInto(key, &out) }, //nolint:errcheck // only the locking matters
	}
}

// A read under a codec decodes the value into the type it is asked for, or into the generic form
// that Get returns, and what it returns does not alias the session. That holds for a codec that
// hands out the same buffer for every value it encodes as well: the session keeps a copy.
func Test_Codec_Reads(t *testing.T) {
	t.Parallel()
	for name, newCodec := range map[string]func() Codec{
		"json":                           func() Codec { return JSONCodec{} },
		"a codec that reuses its buffer": func() Codec { return &reusingCodec{} },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			profile := newCodecProfile()
			sess := seedAndLoad(t, NewStore(Config{Codec: newCodec()}), func(s *Session) {
				s.Set("profile", profile)
				s.Set("visits", 3)
				s.Set("name", "alice")
				s.Set("nothing", nil)
				s.Set("list", []string{"a", "b"})
			})
			require.NotNil(t, sess.rawData, "a record that was read is written back as it is, from the snapshot")
			require.True(t, isRecord(sess.rawData))
			require.False(t, sess.data.dirty.Load())

			var got codecProfile
			require.NoError(t, sess.GetInto("profile", &got))
			require.Equal(t, profile, got)
			require.Equal(t, 3, mustGetAs[int](t, sess, "visits"))
			require.Equal(t, profile, mustGetAs[codecProfile](t, sess, "profile"))

			_, ok := GetAs[int](sess, "name")
			require.False(t, ok, "a string does not decode into an int")
			_, ok = GetAs[string](sess, "missing")
			require.False(t, ok)

			require.ErrorIs(t, sess.GetInto("missing", &got), ErrKeyNotFound)
			var count int
			require.Error(t, sess.GetInto("name", &count))

			// Get is the generic form of the codec.
			require.InDelta(t, 3.0, sess.Get("visits"), 0)
			require.Equal(t, "alice", sess.Get("name"))
			require.Nil(t, sess.Get("nothing"))
			require.Nil(t, sess.Get("missing"))
			generic, ok := sess.Get("profile").(map[string]any)
			require.True(t, ok)
			require.Equal(t, "alice", generic["name"])

			require.ElementsMatch(t, []any{"name", "nothing", "profile", "visits", "list"}, sess.Keys())

			first := mustGetAs[[]string](t, sess, "list")
			first[0] = "changed"
			require.Equal(t, []string{"a", "b"}, mustGetAs[[]string](t, sess, "list"), "a value read does not alias the session")

			// And a read that cannot change the session leaves it unchanged, so a save writes the snapshot back.
			_ = sess.Get("list")
			var viaInto []string
			require.NoError(t, sess.GetInto("list", &viaInto))
			require.False(t, sess.data.dirty.Load(), "a read under a codec never changes the session")

			// A value that is set is encoded then: the caller may change it afterwards.
			list := []string{"a"}
			sess.Set("set", list)
			list[0] = "changed"
			require.Equal(t, []string{"a"}, mustGetAs[[]string](t, sess, "set"), "what was set is what was encoded then")
		})
	}
}

// badUTF8 is a string that JSON cannot carry: its second byte is not UTF-8.
var badUTF8 = string([]byte{'a', 0xff, 'b'})

type codecHidden struct {
	Shown  string
	Secret string `json:"-"`
}

// A session that was stored as gob is not converted: it stays gob in a store with a codec until it
// ends or is reset, and nothing is lost to a conversion that a codec does not do the way gob does.
func Test_Codec_GobSessionsStayGob(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	mem := memory.New()
	gobStore := NewStore(Config{Storage: mem, AbsoluteTimeout: time.Hour})
	gobStore.RegisterType(codecProfile{})
	gobStore.RegisterType(codecHidden{})
	codecStore := NewStore(Config{Storage: mem, Codec: JSONCodec{}, AbsoluteTimeout: time.Hour})
	values := map[any]any{
		"profile": newCodecProfile(), "visits": 3, "name": "alice", "complex": complex(1, 2),
		// What a codec does not carry over the way gob does.
		7: "a key that is not a string", "bytes": badUTF8, "hidden": codecHidden{Shown: "shown", Secret: "secret"},
	}
	// seed stores a session of the values as gob, and returns its ID.
	seed := func(t *testing.T) string {
		t.Helper()
		return seedSession(t, app, gobStore, func(s *Session) {
			for key, value := range values {
				s.Set(key, value)
			}
		})
	}

	t.Run("it reads as it did", func(t *testing.T) {
		t.Parallel()
		id := seed(t)
		wantExpiry, ok := loadSession(t, app, gobStore, id).AbsoluteExpiry()
		require.True(t, ok)

		sess := loadSession(t, app, codecStore, id)
		require.False(t, sess.Fresh())
		require.Equal(t, id, sess.ID())
		require.IsType(t, gobStream, sess.data.format, "a session that was stored as gob is gob, whatever the store now does")

		// The typed readers work in either mode, and Get returns the values themselves.
		var got codecProfile
		require.NoError(t, sess.GetInto("profile", &got))
		require.Equal(t, values["profile"], got)
		require.Equal(t, 3, mustGetAs[int](t, sess, "visits"))
		for key, value := range values {
			require.Equal(t, value, sess.Get(key), key)
		}
		expiry, ok := sess.AbsoluteExpiry()
		require.True(t, ok)
		require.True(t, wantExpiry.Equal(expiry))
	})

	t.Run("it is saved as gob, and keeps everything it holds", func(t *testing.T) {
		t.Parallel()
		id := seed(t)
		gobRaw := storedBytes(t, mem, id)
		require.False(t, isRecord(gobRaw))

		// Saved after only scalars were read, it is the same bytes.
		sess := loadSession(t, app, codecStore, id)
		require.Equal(t, 3, sess.Get("visits"))
		require.Equal(t, "a key that is not a string", sess.Get(7))
		require.NoError(t, sess.Save())
		require.Equal(t, gobRaw, storedBytes(t, mem, id))

		// Saved changed, it is gob still, and a store without a codec reads it, all of it.
		sess.Set("visits", 4)
		sess.Set("more", 1)
		require.NoError(t, sess.Save())
		require.False(t, isRecord(storedBytes(t, mem, id)))
		back := loadSession(t, app, gobStore, id)
		want := maps.Clone(values)
		want["visits"], want["more"] = 4, 1
		for key, value := range want {
			require.Equal(t, value, back.Get(key), key)
		}
	})

	t.Run("a session that starts over is a session of the store", func(t *testing.T) {
		t.Parallel()
		sess := loadSession(t, app, codecStore, seed(t))
		require.NoError(t, sess.Reset())
		require.IsType(t, &namedCodec{}, sess.data.format)
		sess.Set("k", "v")
		require.NoError(t, sess.Save())
		header, entries := storedRecord(t, mem, sess.ID())
		require.True(t, header.deadline.set, "and its deadline")
		require.Len(t, entries, 1)

		// Destroy starts a session over as Reset does, so that what is saved after it is a record.
		destroyed := loadSession(t, app, codecStore, seed(t))
		require.IsType(t, gobStream, destroyed.data.format, "a session that was stored as gob stays gob")
		require.NoError(t, destroyed.Destroy())
		destroyed.Set("b", 2)
		require.NoError(t, destroyed.Save())
		raw := storedBytes(t, mem, destroyed.ID())
		require.True(t, isRecord(raw), "after Destroy the session is one of the store: %q", raw)

		// So is one that is new.
		fresh := loadSession(t, app, codecStore, "")
		require.IsType(t, &namedCodec{}, fresh.data.format)
	})
}

// conventions:ignore parallel: the log output is process-global, so this test captures it
func Test_Codec_RecordsAStoreCannotRead(t *testing.T) {
	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	id := seedSession(t, app, store, func(s *Session) { s.Set("k", "v") })

	future := slices.Clone(storedBytes(t, mem, id))
	future[len(recordTag)] = 9
	require.NoError(t, mem.Set(sessionKey("future"), future, time.Hour))
	require.NoError(t, mem.Set(sessionKey("bare"), []byte(recordTag), time.Hour))

	for name, tc := range map[string]struct {
		store *Store
		id    string
		warn  string
	}{
		"a store without a codec":     {store: NewStore(Config{Storage: mem}), id: id, warn: `"json"`},
		"a store of another codec":    {store: NewStore(Config{Storage: mem, Codec: NewCodec("other", json.Marshal, json.Unmarshal)}), id: id, warn: `"other"`},
		"a record of a later version": {store: NewStore(Config{Storage: mem, Codec: JSONCodec{}}), id: "future", warn: "format version 9"},
		"a record with no header":     {store: NewStore(Config{Storage: mem}), id: "bare", warn: "header is damaged"},
	} {
		t.Run(name, func(t *testing.T) {
			buf := loggertest.CaptureContextLog(t, "")

			// It is not a session of the store: a new one starts, and the record is left alone.
			sess := loadSession(t, app, tc.store, tc.id)
			require.True(t, sess.Fresh())
			require.NotEqual(t, tc.id, sess.ID())
			require.NotNil(t, storedBytes(t, mem, tc.id))

			// Once is enough to say so.
			again := loadSession(t, app, tc.store, tc.id)
			require.True(t, again.Fresh())
			require.Equal(t, 1, strings.Count(buf.String(), "[Warn]"), "%q", buf.String())
			require.Contains(t, buf.String(), tc.warn)

			_, err := tc.store.GetByID(context.Background(), tc.id)
			require.ErrorIs(t, err, ErrSessionIDNotFoundInStore)
		})
	}

	// The store of the codec reads it, as before.
	sess := loadSession(t, app, store, id)
	require.False(t, sess.Fresh())
	require.Equal(t, "v", mustGetAs[string](t, sess, "k"))
}

// GetByID says so, once per store, when it meets a record its store cannot read.
// conventions:ignore parallel: the log output is process-global, so this test captures it
func Test_Codec_GetByIDWarnsAboutARecordItCannotRead(t *testing.T) {
	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	id := seedSession(t, app, store, func(s *Session) { s.Set("k", "v") })

	plain := NewStore(Config{Storage: mem})
	buf := loggertest.CaptureContextLog(t, "")
	for range 3 {
		_, err := plain.GetByID(context.Background(), id)
		require.ErrorIs(t, err, ErrSessionIDNotFoundInStore)
	}
	require.Equal(t, 1, strings.Count(buf.String(), "[Warn]"), "%q", buf.String())
	require.Contains(t, buf.String(), `"json"`)
}

// A store that cannot read the record a request names starts a new session in its place, so ending
// that session ends the record too: Destroy and Reset delete it, once, on whichever instance the
// logout lands. A request that only uses or rotates its session leaves the record alone, for the
// instance that reads it.
func Test_Codec_EndingTheSessionDeletesTheRecordItReplaced(t *testing.T) {
	t.Parallel()
	for name, codec := range map[string]Codec{"a store without a codec": nil, "a store of another codec": NewCodec("other", json.Marshal, json.Unmarshal)} {
		for _, tc := range []struct {
			do      func(m *Middleware) error
			name    string
			deletes int
		}{
			{name: "a request that ends nothing", do: func(*Middleware) error { return nil }},
			{name: "Regenerate", do: (*Middleware).Regenerate},
			{name: "Destroy", do: (*Middleware).Destroy, deletes: 1},
			{name: "Reset", do: (*Middleware).Reset, deletes: 1},
			{name: "Destroy and then Reset", do: func(m *Middleware) error { return errors.Join(m.Destroy(), m.Reset()) }, deletes: 1},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				mem := storagetest.New()
				id := seedSession(t, fiber.New(), NewStore(Config{Storage: mem, Codec: JSONCodec{}}), func(s *Session) { s.Set("k", "v") })

				app := fiber.New()
				app.Use(New(Config{Storage: mem, Codec: codec}))
				app.Get("/", func(c fiber.Ctx) error { return tc.do(FromContext(c)) })
				apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", withCookies(map[string]string{"session_id": id})), fiber.StatusOK)

				deletes := slices.DeleteFunc(mem.CallKeys(storagetest.OpDelete), func(key string) bool { return key != sessionKey(id) })
				require.Len(t, deletes, tc.deletes)
				require.Equal(t, tc.deletes == 0, storedBytes(t, mem, id) != nil)
			})
		}
	}
}

// Only the record the request itself names is the visitor's to end. A store that shares one source
// of two with another store is handed that store's ID, and a record under it that it cannot read is
// the other store's session: ending its own leaves that one alone. And a store that met no such
// record deletes its own session, once, and nothing else, as before.
func Test_Codec_EndingTheSessionLeavesOtherRecordsAlone(t *testing.T) {
	t.Parallel()
	t.Run("the session of a store that handed its ID over", func(t *testing.T) {
		t.Parallel()
		mem := memory.New()
		outer, outerStore := NewWithStore(Config{Storage: mem, Codec: JSONCodec{}, Extractor: extractors.Chain(extractors.FromCookie("a"), extractors.FromCookie("shared"))})
		inner, innerStore := NewWithStore(Config{Storage: mem, Extractor: extractors.FromCookie("shared")})
		id := seedSession(t, fiber.New(), outerStore, func(s *Session) { s.Set("k", "v") })

		app := fiber.New()
		app.Use(outer, inner)
		app.Get("/", func(c fiber.Ctx) error { return innerStore.FromContext(c).Destroy() })
		apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", withCookies(map[string]string{"a": id})), fiber.StatusOK)

		require.NotNil(t, storedBytes(t, mem, id), "the inner store deleted the session of the outer one")
	})

	t.Run("the record another store of the request could not read", func(t *testing.T) {
		t.Parallel()
		mem := memory.New()
		id := seedSession(t, fiber.New(), NewStore(Config{Storage: mem, Codec: JSONCodec{}}), func(s *Session) { s.Set("k", "v") })
		store := NewStore(Config{Storage: mem, Extractor: extractors.FromCookie("mine")})
		other := NewStore(Config{Storage: mem, Extractor: extractors.FromCookie("other")})

		app := fiber.New()
		app.Get("/", func(c fiber.Ctx) error {
			use := func(s *Store, do func(sess *Session) error) error {
				sess, err := s.Get(c)
				if err != nil {
					return err
				}
				defer sess.Release()
				return do(sess)
			}
			// The store saves a session, the other store meets the record, and the store is asked again.
			return errors.Join(
				use(store, func(sess *Session) error {
					sess.Set("k", "v")
					return sess.Save()
				}),
				use(other, func(*Session) error { return nil }),
				use(store, (*Session).Destroy),
			)
		})
		apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", withCookies(map[string]string{"other": id})), fiber.StatusOK)

		require.NotNil(t, storedBytes(t, mem, id), "a store ended the record that another store of the request could not read")
	})

	for name, stored := range map[string]bool{"a store that found the session": true, "a store that found none": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mem := storagetest.New()
			handler, store := NewWithStore(Config{Storage: mem})
			id := "unknown"
			if stored {
				id = seedSession(t, fiber.New(), store, func(s *Session) { s.Set("k", "v") })
			}

			app := fiber.New()
			app.Use(handler)
			app.Get("/", func(c fiber.Ctx) error { return FromContext(c).Destroy() })
			apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", withCookies(map[string]string{"session_id": id})), fiber.StatusOK)

			require.Len(t, mem.CallKeys(storagetest.OpDelete), 1, "ending a session that replaced no record deletes that session, once")
		})
	}
}

// A request can load the session of a store more than once: a middleware in front of the handler
// asks the store, and the handler asks it again, or the middleware of the store runs behind it, or
// another store that reads the same cookie finds the session. Once the first session is saved, a
// store finds that one and no longer looks under the ID the request names, so the later session
// learns from the request which record the first one took the place of, and ends it when it is
// ended, whatever the first one did in between.
func Test_Codec_EndingALaterSessionOfTheRequestDeletesTheRecordToo(t *testing.T) {
	t.Parallel()
	for name, first := range map[string]func(sess *Session) error{
		"the first is left unsaved": func(*Session) error { return nil },
		"the first was saved": func(sess *Session) error {
			sess.Set("k", "v")
			return sess.Save()
		},
		"the first was regenerated and saved": func(sess *Session) error { return errors.Join(sess.Regenerate(), sess.Save()) },
	} {
		for _, later := range []string{"a second store.Get", "the middleware of the store", "another store over the same cookie"} {
			t.Run(name+"/"+later, func(t *testing.T) {
				t.Parallel()
				mem := storagetest.New()
				id := seedSession(t, fiber.New(), NewStore(Config{Storage: mem, Codec: JSONCodec{}}), func(s *Session) { s.Set("k", "v") })
				handler, store := NewWithStore(Config{Storage: mem})
				asked := store
				if later == "another store over the same cookie" {
					asked = NewStore(Config{Storage: mem})
				}

				app := fiber.New()
				app.Use(func(c fiber.Ctx) error {
					sess, err := store.Get(c)
					if err != nil {
						return err
					}
					defer sess.Release()
					if err := first(sess); err != nil {
						return err
					}
					return c.Next()
				})
				if later == "the middleware of the store" {
					app.Use(handler)
					app.Get("/", func(c fiber.Ctx) error { return FromContext(c).Destroy() })
				} else {
					app.Get("/", func(c fiber.Ctx) error {
						sess, err := asked.Get(c)
						if err != nil {
							return err
						}
						defer sess.Release()
						return sess.Destroy()
					})
				}
				apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", withCookies(map[string]string{"session_id": id})), fiber.StatusOK)

				require.Nil(t, storedBytes(t, mem, id), "the later session did not end the record the request named")
			})
		}
	}
}

// A storage that fails to delete the record fails the Destroy or the Reset, which leaves the session
// as it was, and the call that follows deletes the record: it is not forgotten.
func Test_Codec_EndingTheSession_RecordThatCannotBeDeleted(t *testing.T) {
	t.Parallel()
	for name, end := range map[string]func(sess *Session) error{"Destroy": (*Session).Destroy, "Reset": (*Session).Reset} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mem := storagetest.New()
			id := seedSession(t, fiber.New(), NewStore(Config{Storage: mem, Codec: JSONCodec{}}), func(s *Session) { s.Set("k", "v") })
			sess := loadSession(t, fiber.New(), NewStore(Config{Storage: mem}), id)
			sess.Set("mine", "kept")
			newID, errBoom := sess.ID(), errors.New("boom")

			mem.SetErr(storagetest.OpDelete, sessionKey(id), errBoom)
			require.ErrorIs(t, end(sess), errBoom)
			require.Equal(t, "kept", sess.Get("mine"), "a call that failed emptied the session")
			require.Equal(t, newID, sess.ID(), "a call that failed gave the session another ID")
			require.NotNil(t, storedBytes(t, mem, id))

			mem.SetErr(storagetest.OpDelete, sessionKey(id), nil)
			require.NoError(t, end(sess))
			require.Nil(t, storedBytes(t, mem, id), "the record was forgotten when the delete of it failed")
		})
	}
}

// A logout reaches the record only through the request that named it. Once a request to the store
// that cannot read it has saved the session that took its place, the cookie of the visitor names
// that session, and ending it leaves the record where it is, until its time to live runs out.
func Test_Codec_ALaterRequestNoLongerNamesTheRecord(t *testing.T) {
	t.Parallel()
	mem := storagetest.New()
	id := seedSession(t, fiber.New(), NewStore(Config{Storage: mem, Codec: JSONCodec{}}), func(s *Session) { s.Set("k", "v") })

	app := fiber.New()
	app.Use(New(Config{Storage: mem}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/logout", func(c fiber.Ctx) error { return FromContext(c).Destroy() })

	resp := apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", withCookies(map[string]string{"session_id": id})), fiber.StatusOK)
	replaced := cookiesByName(resp)["session_id"].Value
	require.NotEqual(t, id, replaced)
	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/logout", withCookies(map[string]string{"session_id": replaced})), fiber.StatusOK)

	require.Nil(t, storedBytes(t, mem, replaced))
	require.NotNil(t, storedBytes(t, mem, id))
}

func Test_Codec_CorruptRecordsAreErrors(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	good := storedBytes(t, mem, seedSession(t, app, store, func(s *Session) { s.Set("k", "v") }))
	// loadsAs is the error of loading raw as a stored session through a request that carries its cookie.
	loadsAs := func(raw []byte) error {
		require.NoError(t, mem.Set(sessionKey("cut"), raw, time.Hour))
		ctx := apptest.Ctx(t, app)
		ctx.Request().Header.SetCookie("session_id", "cut")

		sess, err := store.Get(ctx)
		if err == nil {
			sess.Release()
		}
		return err
	}

	// (An empty value is no session at all, for every store.)
	for n := 1; n < len(good); n++ {
		require.Error(t, loadsAs(good[:n]), "record cut to %d bytes", n)
	}
	require.Error(t, loadsAs(append(bytes.Clone(good), 1)))
	require.Error(t, loadsAs([]byte("not a gob stream")))
}

func Test_Codec_SetFailuresFailTheSave(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		set     func(*Session)
		fix     func(*Session)
		want    string
		entries int
	}{
		"a key that is not a string": {
			set: func(s *Session) { s.Set(7, "x") }, fix: func(s *Session) { s.Delete(7) },
			want: errNonStringKey.Error(), entries: 2,
		},
		"a value the codec refuses": {
			set: func(s *Session) { s.Set("ch", make(chan int)) }, fix: func(s *Session) { s.Set("ch", "fine") },
			want: `"ch"`, entries: 3,
		},
		"a value the codec refuses, deleted": {
			set: func(s *Session) { s.Set("ch", make(chan int)) }, fix: func(s *Session) { s.Delete("ch") },
			want: `"ch"`, entries: 2,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			rec := storagetest.New()
			store := NewStore(Config{Storage: rec, Codec: JSONCodec{}})
			sess := loadSession(t, app, store, "")

			sess.Set("fine", 1)
			tc.set(sess)
			sess.Set("alsoFine", 2)
			err := sess.Save()
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
			require.Empty(t, rec.Calls(storagetest.OpSet), "nothing is written")

			// Other keys do not make up for it...
			sess.Set("fine", 3)
			require.Error(t, sess.Save())
			require.Empty(t, rec.Calls(storagetest.OpSet))

			// ...the key does, as it does for gob, which fails the save only while the bad value is in.
			tc.fix(sess)
			require.NoError(t, sess.Save())
			require.Len(t, rec.Calls(storagetest.OpSet), 1)
			_, entries := storedRecord(t, rec, sess.ID())
			require.Len(t, entries, tc.entries)
		})

		// A Set that fails on a session that was loaded, and not changed otherwise, still fails the
		// save: the snapshot is not written back over it.
		t.Run(name+", on a loaded session", func(t *testing.T) {
			t.Parallel()
			rec := storagetest.New()
			sess := seedAndLoad(t, NewStore(Config{Storage: rec, Codec: JSONCodec{}}), func(s *Session) { s.Set("a", 1) })
			rec.ClearCalls()

			tc.set(sess)
			require.Error(t, sess.Save())
			require.Empty(t, rec.Calls(storagetest.OpSet), "nothing is written")
		})
	}

	t.Run("every failure is reported, in a fixed order", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{Codec: JSONCodec{}})
		sess := loadSession(t, fiber.New(), store, "")
		sess.Set("ch", make(chan int))
		sess.Set(1, "x")
		err := sess.Save()
		require.ErrorIs(t, err, errNonStringKey)
		require.Contains(t, err.Error(), `"ch"`)
		require.Less(t, strings.Index(err.Error(), "a store with a Codec"), strings.Index(err.Error(), `"ch"`))
	})

	t.Run("a session that starts over forgets them", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{Codec: JSONCodec{}})
		sess := loadSession(t, fiber.New(), store, "")
		sess.Set(1, "x")
		require.Error(t, sess.Save())
		require.NoError(t, sess.Reset())
		sess.Set("ok", 1)
		require.NoError(t, sess.Save())
	})

	t.Run("an empty new session with a failed Set is not skipped", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Codec: JSONCodec{}, SkipEmptySessions: true}))
		app.Get("/", func(c fiber.Ctx) error {
			FromContext(c).Set(7, "x")
			return nil
		})
		apptest.Expect(t, app, "/", fiber.StatusInternalServerError)
	})
}

// The session calls the codec for what a request reads and sets, and asks it for its name once,
// when the store is made.
func Test_Codec_IsCalledOnlyWhenNeeded(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	mem := memory.New()
	codec := newCountingCodec()
	store := NewStore(Config{Storage: mem, Codec: codec})
	require.EqualValues(t, 1, codec.names.Load())
	id := seedSession(t, app, store, func(s *Session) {
		s.Set("profile", newCodecProfile())
		s.Set("visits", 3)
		s.Set("name", "alice")
	})
	require.EqualValues(t, 3, codec.marshals.Load())
	before := storedBytes(t, mem, id)
	_, beforeEntries := storedRecord(t, mem, id)

	sess := loadSession(t, app, store, id)
	require.Zero(t, codec.decodes.Load(), "loading a session decodes none of its values")

	var name string
	require.NoError(t, sess.GetInto("name", &name))
	require.EqualValues(t, 1, codec.decodes.Load(), "reading one value decodes that one")

	require.NoError(t, sess.Save())
	require.EqualValues(t, 3, codec.marshals.Load(), "a save of an unchanged session encodes nothing")
	require.Equal(t, before, storedBytes(t, mem, id))

	sess.Set("visits", 4)
	require.EqualValues(t, 4, codec.marshals.Load(), "setting one value encodes that one")
	require.NoError(t, sess.Save())
	require.EqualValues(t, 4, codec.marshals.Load())

	_, afterEntries := storedRecord(t, mem, id)
	require.Equal(t, beforeEntries[0], afterEntries[0])
	require.Equal(t, beforeEntries[1], afterEntries[1])
	require.Equal(t, `4`, string(afterEntries[2].value))

	for range 5 {
		again := loadSession(t, app, store, id)
		again.Set("visits", 5)
		require.NoError(t, again.Save())
		_, err := store.GetByID(context.Background(), id)
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, codec.names.Load(), "the name is kept, not asked again")
}

func Test_Codec_AbsoluteTimeout(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: JSONCodec{}, AbsoluteTimeout: time.Hour})
	id := seedSession(t, app, store, func(s *Session) { s.Set("k", "v") })

	header, entries := storedRecord(t, mem, id)
	require.True(t, header.deadline.set)
	require.Len(t, entries, 1, "the deadline is in the header, not among the values")
	require.WithinDuration(t, time.Now().Add(time.Hour), time.Unix(header.deadline.sec, int64(header.deadline.nanos)), time.Minute)

	sess := loadSession(t, app, store, id)
	require.Equal(t, []any{"k"}, sess.Keys())
	expiry, ok := sess.AbsoluteExpiry()
	require.True(t, ok)
	require.Equal(t, header.deadline.sec, expiry.Unix())

	t.Run("an expired session starts over", func(t *testing.T) {
		t.Parallel()
		past := time.Now().Add(-time.Minute)
		raw, err := encodeRecord("json", deadlineOf(past),
			[]recordEntry{{key: "k", value: []byte(`"v"`)}})
		require.NoError(t, err)
		require.NoError(t, mem.Set(sessionKey("expired"), raw, time.Hour))

		expired := loadSession(t, app, store, "expired")
		require.NotEqual(t, "expired", expired.ID())
		require.Nil(t, expired.Get("k"))
		_, ok := expired.AbsoluteExpiry()
		require.True(t, ok)
	})

	t.Run("a session without a deadline stays without", func(t *testing.T) {
		t.Parallel()
		plain := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
		other := seedSession(t, app, plain, func(s *Session) { s.Set("k", "v") })
		header, _ := storedRecord(t, mem, other)
		require.False(t, header.deadline.set)
		_, ok := loadSession(t, app, plain, other).AbsoluteExpiry()
		require.False(t, ok)
	})

	// The deadline keeps its nanoseconds through the record, when it is decoded and when it is encoded.
	t.Run("the deadline keeps its nanoseconds", func(t *testing.T) {
		t.Parallel()
		deadline := time.Now().Add(30 * time.Minute).Truncate(time.Second).Add(123_456_789 * time.Nanosecond)
		id := seedSession(t, app, store, func(s *Session) {
			s.setAbsExpiration(deadline)
			s.Set("a", 1) // the record is encoded: the deadline goes through data.record
		})

		loaded, err := store.GetByID(context.Background(), id) // data.loadRecord
		require.NoError(t, err)
		got, ok := loaded.AbsoluteExpiry()
		require.True(t, ok)
		require.True(t, got.Equal(deadline), "stored %v, read %v", deadline, got)

		loaded.Set("b", 2) // encoded again, from the decoded deadline
		require.NoError(t, loaded.Save())
		loaded.Release()

		again, err := store.GetByID(context.Background(), id)
		require.NoError(t, err)
		defer again.Release()
		got, ok = again.AbsoluteExpiry()
		require.True(t, ok)
		require.True(t, got.Equal(deadline), "stored %v, read %v", deadline, got)
	})
}

func Test_Codec_Middleware(t *testing.T) {
	t.Parallel()
	mem := memory.New()
	app := fiber.New()
	app.Use(New(Config{Storage: mem, Codec: JSONCodec{}}))
	app.Get("/login", func(c fiber.Ctx) error {
		m := FromContext(c)
		m.Set("profile", newCodecProfile())
		m.Set("visits", 1)
		return nil
	})
	app.Get("/me", func(c fiber.Ctx) error {
		m := FromContext(c)
		var profile codecProfile
		if err := m.GetInto("profile", &profile); err != nil {
			return c.Status(fiber.StatusNotFound).SendString(err.Error())
		}
		visits, _ := GetAs[int](m, "visits")
		m.Set("visits", visits+1)
		return c.SendString(profile.Name + " " + strconv.Itoa(visits))
	})

	resp, _ := sendRequest(t, app, "/login", nil)
	cookie := cookiesByName(resp)["session_id"]
	require.NotNil(t, cookie)
	cookies := map[string]string{"session_id": cookie.Value}

	_, body := sendRequest(t, app, "/me", cookies)
	require.Equal(t, "alice 1", body)
	_, body = sendRequest(t, app, "/me", cookies)
	require.Equal(t, "alice 2", body)

	_, entries := storedRecord(t, mem, cookie.Value)
	require.Len(t, entries, 2)
	require.Equal(t, "visits", entries[1].key)
	require.Equal(t, "3", string(entries[1].value))

	_, body = sendExpecting(t, app, "/me", nil, fiber.StatusNotFound)
	require.Contains(t, body, ErrKeyNotFound.Error())
}

func Test_Codec_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	sess := seedAndLoad(t, NewStore(Config{Codec: JSONCodec{}}), func(s *Session) { s.Set("shared", 0) })

	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Go(func() {
			own := "own" + strconv.Itoa(worker)
			for i := range 200 {
				sess.Set(own, i)
				sess.Set("shared", worker)
				var got int
				if err := sess.GetInto(own, &got); err != nil || got != i {
					t.Errorf("read %s: %d, %v", own, got, err)
					return
				}
				_, _ = GetAs[int](sess, "shared")
				_ = sess.Get(own)
				_ = sess.Keys()
				if i%50 == 0 {
					sess.Delete(own)
				}
			}
		})
	}
	wg.Wait()
	require.NoError(t, sess.Save())
}

func Test_Codec_ConfigErrors(t *testing.T) {
	t.Parallel()
	var typedNil *countingCodec
	require.PanicsWithValue(t, "[session] Codec must not be a typed nil", func() { NewStore(Config{Codec: typedNil}) })
	require.Panics(t, func() { NewStore(Config{Codec: nameless{}}) })
	require.Panics(t, func() { NewStore(Config{Codec: longName{}}) })
	require.NotPanics(t, func() { NewStore(Config{Codec: nil}) })
	require.NotPanics(t, func() { NewStore(Config{Codec: JSONCodec{}}) })

	require.Panics(t, func() { NewCodec("", json.Marshal, json.Unmarshal) })
	require.Panics(t, func() { NewCodec("x", nil, json.Unmarshal) })
	require.Panics(t, func() { NewCodec("x", json.Marshal, nil) })
	require.Equal(t, 255, maxCodecNameLen, "the longest name is part of the format")
	require.NotPanics(t, func() { NewCodec(string(make([]byte, maxCodecNameLen)), json.Marshal, json.Unmarshal) })
	require.Panics(t, func() { NewCodec(string(make([]byte, maxCodecNameLen+1)), json.Marshal, json.Unmarshal) })

	codec := NewCodec("custom", json.Marshal, json.Unmarshal)
	require.Equal(t, "custom", codec.Name())
	encoded, err := codec.Marshal(map[string]int{"a": 1})
	require.NoError(t, err)
	var back map[string]int
	require.NoError(t, codec.Unmarshal(encoded, &back))
	require.Equal(t, map[string]int{"a": 1}, back)
	require.Equal(t, "json", JSONCodec{}.Name())
}

type nameless struct{ JSONCodec }

func (nameless) Name() string { return "" }

type longName struct{ JSONCodec }

func (longName) Name() string { return string(make([]byte, maxCodecNameLen+1)) }

// GetInto fills the target the same way for the sessions of a store without a Codec and of one
// with: all or nothing, and only a non-nil pointer is a target. A missing key is quoted in the
// error, so that a key that came from a client cannot forge a line of a log.
func Test_Session_GetInto(t *testing.T) {
	t.Parallel()
	for name, codec := range map[string]Codec{"gob": nil, "codec": JSONCodec{}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sess := seedAndLoad(t, NewStore(Config{Codec: codec}), func(s *Session) {
				s.Set("name", "alice")
				s.Set("n", 3)
				s.Set("nothing", nil)
			})

			t.Run("a value is read into a variable of its type, or into an interface", func(t *testing.T) {
				t.Parallel()
				var name string
				require.NoError(t, sess.GetInto("name", &name))
				require.Equal(t, "alice", name)

				var n int
				require.NoError(t, sess.GetInto("n", &n))
				require.Equal(t, 3, n)

				var anything any
				require.NoError(t, sess.GetInto("name", &anything))
				require.Equal(t, "alice", anything)

				require.False(t, sess.data.dirty.Load(), "scalars read into a target are not a way to change the session")
			})

			t.Run("a stored nil zeroes the target", func(t *testing.T) {
				t.Parallel()
				set := "set"
				require.NoError(t, sess.GetInto("nothing", &set))
				require.Empty(t, set)
			})

			t.Run("a value that does not fit is an error that leaves the target as it was", func(t *testing.T) {
				t.Parallel()
				count := 5
				err := sess.GetInto("name", &count)
				require.Error(t, err)
				require.NotErrorIs(t, err, ErrKeyNotFound)
				require.Equal(t, 5, count)
			})

			t.Run("a missing key leaves the target alone", func(t *testing.T) {
				t.Parallel()
				keep := "keep"
				err := sess.GetInto("missing", &keep)
				require.ErrorIs(t, err, ErrKeyNotFound)
				require.EqualError(t, err, ErrKeyNotFound.Error()+`: "missing"`)
				require.Equal(t, "keep", keep)
			})

			t.Run("the target has to be a non-nil pointer", func(t *testing.T) {
				t.Parallel()
				var nilPointer *string
				var name string
				require.Error(t, sess.GetInto("name", name))
				require.Error(t, sess.GetInto("name", nilPointer))
				require.Error(t, sess.GetInto("name", nil))
				err := sess.GetInto("missing", nil)
				require.Error(t, err)
				require.NotErrorIs(t, err, ErrKeyNotFound, "a bad target is reported before a missing key")
			})

			t.Run("a key is quoted in the error", func(t *testing.T) {
				t.Parallel()
				var out string
				err := sess.GetInto("a\nb [Error] forged", &out)
				require.ErrorIs(t, err, ErrKeyNotFound)
				require.NotContains(t, err.Error(), "\n")
				require.Contains(t, err.Error(), `"a\nb [Error] forged"`)
			})
		})
	}
}

// Without a Codec the value that is stored is assigned to the target, which has to be able to hold
// it, and a value that can share memory with the session gives up the snapshot, as it does for Get.
func Test_Session_GetInto_WithoutCodec(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	store := NewStore()
	store.RegisterType(codecProfile{})
	profile := newCodecProfile()
	id := seedSession(t, app, store, func(s *Session) {
		s.Set("n", 3)
		s.Set("profile", profile)
		s.Set("list", []string{"a"})
	})

	t.Run("a value that does not fit is an error that leaves the target", func(t *testing.T) {
		t.Parallel()
		sess := loadSession(t, app, store, id)
		wide := int64(9)
		err := sess.GetInto("n", &wide)
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrKeyNotFound)
		require.Equal(t, int64(9), wide)

		var pointer *codecProfile
		require.Error(t, sess.GetInto("profile", &pointer))
		require.Nil(t, pointer)
	})

	for key, tc := range map[string]struct{ target, want any }{
		"profile": {target: new(codecProfile), want: &profile},
		"list":    {target: new([]string), want: &[]string{"a"}},
	} {
		t.Run("a value that can alias the session gives up the snapshot: "+key, func(t *testing.T) {
			t.Parallel()
			sess := loadSession(t, app, store, id)
			require.False(t, sess.data.dirty.Load())
			require.NoError(t, sess.GetInto(key, tc.target))
			require.Equal(t, tc.want, tc.target)
			require.True(t, sess.data.dirty.Load())
		})
	}
}

func Test_Codec_ErrorsNamePortableCauses(t *testing.T) {
	t.Parallel()
	require.NotErrorIs(t, errNonStringKey, ErrKeyNotFound)
	require.NotErrorIs(t, fmt.Errorf("x: %w", errCodecMismatch), errRecordNeedsCodec)
}

// zeroCopyCodec stores bytes as they are and decodes them without a copy, as a codec that keeps a
// slice of its input does.
type zeroCopyCodec struct{}

func (zeroCopyCodec) Name() string { return "zero-copy" }

func (zeroCopyCodec) Marshal(v any) ([]byte, error) {
	b, ok := v.([]byte)
	if !ok {
		return nil, errors.New("bytes only")
	}
	return b, nil
}

func (zeroCopyCodec) Unmarshal(data []byte, v any) error {
	p, ok := v.(*[]byte)
	if !ok {
		return errors.New("bytes only")
	}
	*p = data
	return nil
}

// A decoded value may keep the bytes it is given, and a program may change it: that must not reach
// the record the session writes back.
func Test_Codec_DecodedValuesDoNotAliasTheSnapshot(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: zeroCopyCodec{}})
	id := seedSession(t, app, store, func(s *Session) {
		s.Set("a", []byte("first"))
		s.Set("b", []byte("second"))
	})
	before := storedBytes(t, mem, id)

	sess := loadSession(t, app, store, id)

	var a []byte
	require.NoError(t, sess.GetInto("a", &a))
	a[0] = 'X'
	a = append(a, "-appended past what was there, into the entry that follows"...)
	_ = a

	viaGetAs := mustGetAs[[]byte](t, sess, "a")
	require.Equal(t, "first", string(viaGetAs))
	viaGetAs[1] = 'Y'

	generic, isBytes := sess.Get("b").([]byte)
	require.True(t, isBytes, "an undecodable value is returned as it is stored")
	generic[0] = 'Z'

	require.NoError(t, sess.Save())
	require.Equal(t, before, storedBytes(t, mem, id), "a clean save writes the record back as it was read")

	again := loadSession(t, app, store, id)
	require.Equal(t, "first", string(mustGetAs[[]byte](t, again, "a")))
	require.Equal(t, "second", string(mustGetAs[[]byte](t, again, "b")))
}

// A record that claims half a million entries and holds one makes the loader allocate for the entries it
// read, a slab of 16 at most, and not for the claim.
//
// conventions:ignore parallel: it measures what the whole process allocates
func Test_Codec_LoadingARecordAllocatesForWhatItHolds(t *testing.T) {
	claim := slices.Concat([]byte(testRecordMagic), []byte{4}, []byte("json"), []byte{0}, binary.AppendUvarint(nil, 1<<19), make([]byte, 1<<20))
	codec := &namedCodec{Codec: JSONCodec{}, name: "json"}
	require.ErrorIs(t, codec.loadRecord(claim, make(map[any]any)), errInvalidRecord)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range 10 {
		_ = codec.loadRecord(claim, make(map[any]any)) //nolint:errcheck // it fails, and what it allocated is measured
	}
	runtime.ReadMemStats(&after)
	require.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(256<<10), "the claim of the record was allocated")
}

// A record holds its values in slabs of 16: one of any number of values loads whole, each value reads
// back as it was set, and a change to one is written back with the rest. The keys the session hands
// out are copies: one kept past the request would keep the whole snapshot alive.
func Test_Codec_ManyValues(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 8, 9, 16, 17, 33, 40} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			store := NewStore(Config{Storage: memory.New(), Codec: JSONCodec{}})
			name := func(i int) string { return fmt.Sprintf("k%02d", i) }
			id := seedSession(t, app, store, func(s *Session) {
				for i := range n {
					s.Set(name(i), i)
				}
			})

			sess := loadSession(t, app, store, id)
			for i := range n {
				require.Equal(t, i, mustGetAs[int](t, sess, name(i)))
			}
			keys := sess.Keys()
			require.Len(t, keys, n)
			start := reflect.ValueOf(sess.rawData).Pointer()
			for _, key := range keys {
				at := reflect.ValueOf(key).Pointer()
				require.False(t, at >= start && at < start+uintptr(len(sess.rawData)), "the key %v points into the snapshot", key)
			}

			sess.Set(name(0), -1)
			require.NoError(t, sess.Save())
			again := loadSession(t, app, store, id)
			require.Len(t, again.Keys(), n)
			require.Equal(t, -1, mustGetAs[int](t, again, name(0)))
			for i := 1; i < n; i++ {
				require.Equal(t, i, mustGetAs[int](t, again, name(i)))
			}
		})
	}
}

// A key that cannot be hashed makes a read panic. The lock the read took must go with it, or the
// deferred Release of the store pattern waits for it for ever.
func Test_Reads_ReleaseTheirLocksWhenTheyPanic(t *testing.T) {
	t.Parallel()
	unhashable := []int{1}
	for name, codec := range map[string]Codec{"gob": nil, "codec": JSONCodec{}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			store := NewStore(Config{Codec: codec})
			for _, read := range []string{"Get", "GetAs", "GetInto"} {
				sess := loadSession(t, app, store, "")
				require.Panics(t, readsOf(sess, unhashable)[read], "Session."+read)
				finishes(t, func() {
					sess.Set("k", 1) // takes the write lock
					sess.Delete("k")
				}, read+" left the lock of the session taken")

				ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
				m := startMiddleware(t, ctx, store)
				require.Panics(t, readsOf(m, unhashable)[read], "Middleware."+read)
				finishes(t, func() { m.Set("k", 1) }, read+" left the lock of the middleware taken") // takes its write lock
				app.ReleaseCtx(ctx)
			}
		})
	}
}

// A reset puts the session in the state of its store in one step: a Set that races with it is
// encoded or gone, never kept raw in a session that is read as encoded.
func Test_Codec_ResetIsOneStep(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	mem := memory.New()
	t.Cleanup(func() { require.NoError(t, mem.Close()) })
	gobStore := NewStore(Config{Storage: mem})
	codecStore := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	for range 200 {
		id := seedSession(t, app, gobStore, func(s *Session) { s.Set("k", 1) })
		sess := loadSession(t, app, codecStore, id)
		require.IsType(t, gobStream, sess.data.format, "it is a session that was stored as gob")

		var wg sync.WaitGroup
		for i := range 4 {
			wg.Go(func() {
				for j := range 50 {
					sess.Set("racing", i*100+j)
				}
			})
		}
		require.NoError(t, sess.Reset())
		wg.Wait()

		// Whatever the Sets did, what is in the session is of the store.
		require.IsType(t, &namedCodec{}, sess.data.format)
		require.NoError(t, sess.Save())
	}
}

// With a codec GetAs and GetInto decode what was stored into the type they are asked for; they do
// not check that this type is the one it was stored as, which gob does.
func Test_Codec_GetAsDecodesItDoesNotCheckTheType(t *testing.T) {
	t.Parallel()
	type user struct{ Name string }
	type admin struct{ Name string }

	gobStore := NewStore()
	gobStore.RegisterType(user{})

	for name, tc := range map[string]struct {
		store   *Store
		decodes bool
	}{
		"gob":   {store: gobStore, decodes: false},
		"codec": {store: NewStore(Config{Codec: JSONCodec{}}), decodes: true},
	} {
		sess := seedAndLoad(t, tc.store, func(s *Session) {
			s.Set("user", user{Name: "alice"})
			s.Set("count", 5)
		})

		_, isAdmin := GetAs[admin](sess, "user")
		_, isFloat := GetAs[float64](sess, "count")
		_, isInt64 := GetAs[int64](sess, "count")
		_, isUser := GetAs[user](sess, "user")
		require.True(t, isUser, name)
		why := name + ": a codec decodes into whatever fits, gob checks the type that was stored"
		require.Equal(t, tc.decodes, isAdmin, why)
		require.Equal(t, tc.decodes, isFloat, why)
		require.Equal(t, tc.decodes, isInt64, why)
	}
}

// targetCodec is a JSONCodec that remembers what its last Unmarshal was given, and can be made to
// fail or to panic after it has decoded.
type targetCodec struct {
	JSONCodec
	fail   error
	last   any
	mu     sync.Mutex
	panics bool
}

func (c *targetCodec) Unmarshal(data []byte, v any) error {
	c.mu.Lock()
	c.last = v
	c.mu.Unlock()
	if err := c.JSONCodec.Unmarshal(data, v); err != nil {
		return err
	}
	if c.panics {
		panic("boom")
	}
	return c.fail
}

// target returns what the last Unmarshal was given.
func (c *targetCodec) target() any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last
}

func Test_Codec_GetInto_DecodesIntoTheTarget(t *testing.T) {
	t.Parallel()
	errBoom := errors.New("boom")
	// setup returns a session that holds a profile and a map, and the codec its store uses.
	setup := func(t *testing.T, fail error) (*targetCodec, *Session) {
		t.Helper()
		codec := &targetCodec{fail: fail}
		return codec, seedAndLoad(t, NewStore(Config{Codec: codec}), func(s *Session) {
			s.Set("user", codecProfile{ID: 7, Name: "alice"})
			s.Set("map", map[string]string{"new": "1"})
		})
	}

	t.Run("a target that holds its zero value is decoded into directly", func(t *testing.T) {
		t.Parallel()
		codec, sess := setup(t, nil)
		var got codecProfile
		require.NoError(t, sess.GetInto("user", &got))
		require.Same(t, &got, codec.target(), "the codec decodes into the variable of the caller, not into a copy")
		require.Equal(t, codecProfile{ID: 7, Name: "alice"}, got.withoutTime())
	})

	t.Run("a target that holds something is replaced by a new value", func(t *testing.T) {
		t.Parallel()
		codec, sess := setup(t, nil)
		got := codecProfile{ID: 99, Name: "guest", Roles: []string{"stale"}}
		require.NoError(t, sess.GetInto("user", &got))
		require.NotSame(t, &got, codec.target(), "a decoder merges into what it is given, so it is given a new value")
		require.Equal(t, codecProfile{ID: 7, Name: "alice"}, got.withoutTime())

		stale := map[string]string{"old": "stale"}
		require.NoError(t, sess.GetInto("map", &stale))
		require.Equal(t, map[string]string{"new": "1"}, stale)
	})

	t.Run("a decode that fails after it wrote leaves a zero target as it was", func(t *testing.T) {
		t.Parallel()
		codec, sess := setup(t, errBoom)
		var got codecProfile
		err := sess.GetInto("user", &got)
		require.ErrorIs(t, err, errBoom, "the error of the codec is wrapped, not replaced")
		require.Same(t, &got, codec.target(), "it was decoded into the variable, which the failure then cleared")
		require.Zero(t, got)
	})

	t.Run("a decode that panics leaves a zero target as it was", func(t *testing.T) {
		t.Parallel()
		codec, sess := setup(t, nil)
		codec.panics = true
		var got codecProfile
		require.PanicsWithValue(t, "boom", func() {
			err := sess.GetInto("user", &got)
			t.Errorf("GetInto returned %v instead of panicking", err)
		})
		require.Zero(t, got)
	})

	t.Run("a decode that fails leaves a target that holds something as it was", func(t *testing.T) {
		t.Parallel()
		_, sess := setup(t, errBoom)
		got := codecProfile{ID: 99}
		require.ErrorIs(t, sess.GetInto("user", &got), errBoom)
		require.Equal(t, codecProfile{ID: 99}, got)
	})
}

// withoutTime returns the profile without its time, which a test does not compare.
func (p *codecProfile) withoutTime() codecProfile {
	c := *p
	c.At = time.Time{}
	return c
}

// A value that does not decode is still there: Get never says that a value that exists does not,
// and nothing of a decode that failed halfway is returned or left in the target.
func Test_Codec_ValuesThatFailToDecode(t *testing.T) {
	t.Parallel()
	type pair struct {
		Name string `json:"name"`
		ID   int    `json:"id"`
	}

	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	raw, err := encodeRecord("json", recordDeadline{}, []recordEntry{
		{key: "broken", value: []byte(`{not json`)},
		{key: "fine", value: []byte(`"fine"`)},
		{key: "half", value: []byte(`{"id":7,"name":5}`)}, // the id fits, the name does not
	})
	require.NoError(t, err)
	require.NoError(t, mem.Set(sessionKey("stored"), raw, time.Hour))
	sess := loadSession(t, app, store, "stored")

	require.Equal(t, []byte(`{not json`), sess.Get("broken"))
	_, ok := GetAs[string](sess, "broken")
	require.False(t, ok)
	var out string
	require.Error(t, sess.GetInto("broken", &out))
	require.NotErrorIs(t, sess.GetInto("broken", &out), ErrKeyNotFound)
	require.ElementsMatch(t, []any{"broken", "fine", "half"}, sess.Keys())
	require.Equal(t, "fine", sess.Get("fine"))

	// A decode that fails returns the zero value, not what the codec had decoded before it failed.
	value, ok := GetAs[pair](sess, "half")
	require.False(t, ok)
	require.Equal(t, pair{}, value, "nothing of a failed decode is returned")

	target := pair{Name: "keep", ID: 1}
	require.Error(t, sess.GetInto("half", &target))
	require.Equal(t, pair{Name: "keep", ID: 1}, target, "nor stored in the target")
}

// callbackCodec calls back into the session while it decodes.
type callbackCodec struct {
	JSONCodec
	onUnmarshal func()
}

func (c *callbackCodec) Unmarshal(data []byte, v any) error {
	if c.onUnmarshal != nil {
		c.onUnmarshal()
	}
	return c.JSONCodec.Unmarshal(data, v)
}

// A codec runs without the lock of the session: one that uses the session does not deadlock.
func Test_Codec_RunsWithoutTheSessionLocked(t *testing.T) {
	t.Parallel()
	codec := &callbackCodec{}
	app := fiber.New()
	store := NewStore(Config{Codec: codec})
	id := seedSession(t, app, store, func(s *Session) { s.Set("name", "alice") })
	ctx := apptest.Ctx(t, app)
	ctx.Request().Header.SetCookie("session_id", id)
	m := startMiddleware(t, ctx, store)
	codec.onUnmarshal = func() { m.Set("touched", true) }

	var (
		name, got string
		ok        bool
		err       error
	)
	finishes(t, func() {
		err = m.GetInto("name", &name)
		got, ok = GetAs[string](m, "name")
		_ = m.Get("name")
	}, "GetInto, GetAs or Get holds the lock of the session while the codec runs")
	require.NoError(t, err)
	require.Equal(t, "alice", name)
	require.True(t, ok)
	require.Equal(t, name, got, "GetAs disagrees with GetInto")
}

// scribblingCodec overwrites the bytes it is handed once it has decoded them.
type scribblingCodec struct{ JSONCodec }

func (scribblingCodec) Name() string { return "scribbling" }

func (c scribblingCodec) Unmarshal(data []byte, v any) error {
	err := c.JSONCodec.Unmarshal(data, v)
	for i := range data {
		data[i] = 'X'
	}
	return err
}

// Get, like GetAs and GetInto, hands the codec a copy: what the codec does to it does not reach the
// session.
func Test_Codec_GetHandsTheCodecACopy(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: scribblingCodec{}})
	id := seedSession(t, app, store, func(s *Session) { s.Set("a", "hello") })
	before := storedBytes(t, mem, id)

	sess := loadSession(t, app, store, id)
	require.Equal(t, "hello", sess.Get("a"))
	require.Equal(t, "hello", sess.Get("a"), "the first read did not damage what is stored")
	var s string
	require.NoError(t, sess.GetInto("a", &s))
	require.Equal(t, "hello", s)
	require.NoError(t, sess.Save())

	require.Equal(t, before, storedBytes(t, mem, id))
}

// A Store built without NewStore asks its Codec for the name every time, and its sessions are the
// sessions of a store with a codec all the same.
func Test_Codec_StoreWithoutNewStore(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	mem := memory.New()
	store := &Store{Config: configDefault(Config{Storage: mem, Codec: JSONCodec{}})}
	require.Nil(t, store.named)
	require.IsType(t, &namedCodec{}, store.sessionFormat())

	id := seedSession(t, app, store, func(s *Session) { s.Set("name", "alice") })
	header, entries := storedRecord(t, mem, id)
	require.Equal(t, "json", string(header.codec))
	require.Len(t, entries, 1)

	require.Equal(t, "alice", mustGetAs[string](t, loadSession(t, app, store, id), "name"))
}

// Reads wait for the lock of the middleware, which a Reset, Destroy or Regenerate holds while it
// calls the storage.
func Test_Middleware_ReadsTakeTheReadLock(t *testing.T) {
	t.Parallel()
	for name, codec := range map[string]Codec{"gob": nil, "json": JSONCodec{}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := startMiddleware(t, apptest.Ctx(t, fiber.New()), NewStore(Config{Codec: codec}))
			m.Set("k", "v")

			reads := readsOf(m, "k")
			reads["fetch"] = func() { _, _ = m.fetch("k") }
			for readName, read := range reads {
				m.mu.Lock()
				done := make(chan struct{})
				go func() {
					defer close(done)
					read()
				}()
				select {
				case <-done:
					m.mu.Unlock()
					t.Fatalf("%s did not wait for the lock of the middleware", readName)
				case <-time.After(50 * time.Millisecond):
				}
				m.mu.Unlock()
				apptest.Recv(t, done, 5*time.Second, "%s did not finish after the lock was given back", readName)
			}
		})
	}
}

// A Set can be inside the codec when its session is released, or reset into another format: the
// data goes back to the pool, to be handed to another request. What the codec returns then belongs
// to no session of that request, and neither does the error.
func Test_Codec_SetInsideTheCodecWhenTheDataIsReleasedStoresNothing(t *testing.T) {
	t.Parallel()
	for name, fail := range map[string]bool{"encoded": false, "failed": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entered, leave := make(chan struct{}), make(chan struct{})
			marshal := func(any) ([]byte, error) {
				close(entered)
				<-leave
				if fail {
					return nil, errors.New("the codec failed")
				}
				return []byte("x"), nil
			}
			codec := &namedCodec{Codec: NewCodec("blocking", marshal, func([]byte, any) error { return nil }), name: "blocking"}

			d := acquireData()
			d.reset(codec)
			done := make(chan struct{})
			go func() {
				defer close(done)
				d.Set("late", "v")
			}()
			apptest.Recv(t, entered, 5*time.Second)
			d.reset(gobStream) // the session is released while the codec runs
			close(leave)
			apptest.Recv(t, done, 5*time.Second)

			require.Empty(t, d.Data, "a value for a session that is gone")
			require.NoError(t, d.pendingError(), "an error of a Set of a session that is gone")
			releaseData(d)
		})
	}
}

// Under a codec a request that changed nothing does not store its record over a change another
// request stored meanwhile, and a Regenerate by it carries that change to the new ID, with what the
// other request deleted gone; a request that called Set stores its whole session.
func Test_Codec_UnchangedRequestKeepsConcurrentChange(t *testing.T) {
	t.Parallel()
	read := func(s *Session) { _ = s.Get("role") }
	set := func(s *Session) { s.Set("role", "owner") }
	for name, tc := range map[string]struct {
		touch      func(*Session)
		want, gone any
		regenerate bool
	}{
		"read":                {touch: read, want: "user"},
		"read and regenerate": {touch: read, want: "user", regenerate: true},
		"set":                 {touch: set, want: "owner", gone: "x"},
		"set and regenerate":  {touch: set, want: "owner", gone: "x", regenerate: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			store := NewStore(Config{Codec: JSONCodec{}})
			id := seedSession(t, app, store, func(s *Session) { s.Set("role", "admin"); s.Set("gone", "x") })

			inFlight := loadSession(t, app, store, id)
			tc.touch(inFlight)
			other := loadSession(t, app, store, id)
			other.Set("role", "user")
			other.Delete("gone")
			require.NoError(t, other.Save())
			if tc.regenerate {
				require.NoError(t, inFlight.Regenerate())
			}
			require.NoError(t, inFlight.Save())

			stored := sessionTestStored(t, store, inFlight.ID())
			require.Equal(t, tc.want, stored.Get("role"))
			require.Equal(t, tc.gone, stored.Get("gone"))
		})
	}
}

// A Regenerate that cannot read what another request stored, a record of another codec for
// instance, still rotates the ID and leaves the data of the request as it was.
func Test_Codec_RegenerateKeepsSessionOnUnreadableChange(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	store := NewStore(Config{Codec: JSONCodec{}})
	id := seedSession(t, app, store, func(s *Session) { s.Set("role", "admin") })

	inFlight := loadSession(t, app, store, id)
	other := loadSession(t, app, store, id)
	other.Set("role", "user")
	require.NoError(t, other.Save())
	foreign := mustEncodeRecord(t, "other", recordDeadline{}, []recordEntry{{key: "role", value: []byte(`"x"`)}})
	require.NoError(t, store.Storage.Set(string(inFlight.storageKey()), foreign, 0))

	require.ErrorIs(t, inFlight.Regenerate(), errCodecMismatch)
	require.NotEqual(t, id, inFlight.ID())
	require.Equal(t, "admin", inFlight.Get("role"))
	require.NoError(t, inFlight.Save())
	require.Equal(t, "admin", sessionTestStored(t, store, inFlight.ID()).Get("role"))
	sessionTestRequireGone(t, store, id)
}

// A Set under a codec is a change of the request, as under gob, and not a stale snapshot: the
// request then stores its session over a change another request stored meanwhile.
func Test_Codec_SetCountsAsTheRequestsChange(t *testing.T) {
	t.Parallel()
	d := acquireData()
	defer releaseData(d)
	d.reset(&namedCodec{Codec: JSONCodec{}, name: "json"})
	d.mutated.Store(false)
	d.dirty.Store(false)

	d.Set("k", "v")
	require.True(t, d.mutated.Load())
	require.False(t, d.dirty.Load())
}

// A session behaves the same whatever it is stored with. The same random requests, each setting,
// deleting, resetting, regenerating or destroying, are played against a store that keeps gob, one
// with a JSON codec and one with a codec made by NewCodec, and every one of them is checked against
// a model that is a plain Go map: whatever was set last is what a later request reads back, typed.
//
// Each key has one type, so that the same GetAs reads it in all modes.

// modelKey is one key of the model: how to make a value for it and how to read it back typed.
type modelKey struct {
	generate func(r *rand.Rand) any
	read     func(s *Session, key string) (any, bool)
	name     string
}

// modelString makes a short string out of pieces that stress a codec: multi-byte runes, quotes,
// backslashes and a slash.
func modelString(r *rand.Rand) string {
	pieces := []string{"a", "b", "é", "\"", "\\", "/", "x y", "日本", "0"}
	parts := make([]string, 1+r.IntN(4))
	for i := range parts {
		parts[i] = pieces[r.IntN(len(pieces))]
	}
	return strings.Join(parts, "")
}

// readAs reads key as a V through GetAs, in the shape the model compares.
func readAs[V any](s *Session, key string) (any, bool) {
	v, ok := GetAs[V](s, key)
	return v, ok
}

// modelKeys are the keys the model plays with, one for each kind of value a session holds.
var modelKeys = []modelKey{
	{name: "s", generate: func(r *rand.Rand) any { return modelString(r) }, read: readAs[string]},
	{name: "n", generate: func(r *rand.Rand) any { return r.IntN(2001) - 1000 }, read: readAs[int]},
	{name: "l", generate: func(r *rand.Rand) any {
		out := make([]string, 1+r.IntN(3))
		for i := range out {
			out[i] = modelString(r)
		}
		return out
	}, read: readAs[[]string]},
	{name: "m", generate: func(r *rand.Rand) any {
		out := make(map[string]int)
		for range 1 + r.IntN(3) {
			out[modelString(r)] = r.IntN(100)
		}
		return out
	}, read: readAs[map[string]int]},
	{name: "p", generate: func(r *rand.Rand) any {
		p := newCodecProfile()
		p.ID = r.IntN(1000)
		p.Name = modelString(r)
		p.Roles = []string{modelString(r)}
		p.Prefs = map[string]string{modelString(r): modelString(r)}
		p.At = time.Date(2026, time.Month(1+r.IntN(12)), 1+r.IntN(28), r.IntN(24), r.IntN(60), r.IntN(60), 0, time.UTC)
		return p
	}, read: readAs[codecProfile]},
}

func Test_Codec_MatchesGobAndAModel(t *testing.T) {
	t.Parallel()
	modes := map[string]Codec{
		"gob":    nil,
		"json":   JSONCodec{},
		"custom": NewCodec("custom", json.Marshal, json.Unmarshal),
	}
	for mode, codec := range modes {
		for _, absolute := range []time.Duration{0, time.Hour} {
			t.Run(mode+"/absolute="+absolute.String(), func(t *testing.T) {
				t.Parallel()
				for seed := range invariantSeeds() * 25 {
					playModel(t, seed, codec, absolute)
				}
			})
		}
	}
}

func playModel(t *testing.T, seed uint64, codec Codec, absolute time.Duration) {
	t.Helper()

	r := rand.New(rand.NewPCG(seed, seed*2654435761+1))
	app := fiber.New()
	mem := memory.New()
	defer func() { require.NoError(t, mem.Close()) }()
	store := NewStore(Config{Storage: mem, Codec: codec, AbsoluteTimeout: absolute})
	store.RegisterType(codecProfile{})
	store.RegisterType(map[string]int{})

	model := make(map[string]any)
	clientID := ""

	// checkKey checks that the session reads the key as the model says.
	checkKey := func(s *Session, mk modelKey, when string) {
		t.Helper()
		got, ok := mk.read(s, mk.name)
		expected, present := model[mk.name]
		require.Equal(t, present, ok, "seed %d, %s: key %s present", seed, when, mk.name)
		if present {
			require.Equal(t, expected, got, "seed %d, %s: key %s", seed, when, mk.name)
		}
	}
	check := func(s *Session, when string) {
		t.Helper()
		var keys []string
		for _, k := range s.Keys() {
			if name, ok := k.(string); ok {
				keys = append(keys, name)
			}
		}
		slices.Sort(keys)
		require.Equal(t, slices.Sorted(maps.Keys(model)), keys, "seed %d, %s: keys", seed, when)

		for _, mk := range modelKeys {
			checkKey(s, mk, when)
		}
	}

	for step := range 40 {
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		if clientID != "" {
			ctx.Request().Header.SetCookie("session_id", clientID)
		}
		sess, err := store.Get(ctx)
		require.NoError(t, err, "seed %d, step %d", seed, step)
		require.Equal(t, clientID == "", sess.Fresh(), "seed %d, step %d: a session the client holds is found", seed, step)
		check(sess, fmt.Sprintf("step %d, loaded", step))

		destroyed := false
		for range 1 + r.IntN(4) {
			mk := modelKeys[r.IntN(len(modelKeys))]
			switch r.IntN(12) {
			case 0, 1, 2, 3, 4:
				value := mk.generate(r)
				sess.Set(mk.name, value)
				model[mk.name] = value
			case 5, 6:
				sess.Delete(mk.name)
				delete(model, mk.name)
			case 7:
				require.NoError(t, sess.Reset())
				clear(model)
			case 8:
				require.NoError(t, sess.Regenerate())
			case 9:
				if r.IntN(3) == 0 {
					require.NoError(t, sess.Destroy())
					clear(model)
					destroyed = true
				}
			default:
				checkKey(sess, mk, fmt.Sprintf("step %d, a read", step))
			}
			if destroyed {
				break
			}
		}
		check(sess, fmt.Sprintf("step %d, after the operations", step))

		if destroyed {
			clientID = ""
		} else {
			require.NoError(t, sess.Save(), "seed %d, step %d", seed, step)
			clientID = sess.ID()
		}
		sess.Release()
		app.ReleaseCtx(ctx)
	}

	// Whatever is left in the storage is what the model says.
	if clientID != "" {
		final := loadSession(t, app, store, clientID)
		check(final, "at the end")
	}
}

// FuzzSessionCodecStoredBytes loads whatever bytes a storage holds under a session ID, as a store
// with a codec and as one without. Neither may panic, a session that loads must be readable and
// savable, an unchanged one must be written back as it was, and a change must keep the other values
// as they were.
func FuzzSessionCodecStoredBytes(f *testing.F) {
	mem := memory.New()
	f.Cleanup(func() { require.NoError(f, mem.Close()) })
	app := fiber.New()
	codecStore := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	plainStore := NewStore(Config{Storage: mem})

	valid, err := encodeRecord("json", recordDeadline{sec: 1_900_000_000, nanos: 5, set: true},
		[]recordEntry{{key: "a", value: []byte(`"x"`)}, {key: "b", value: []byte(`{"n":1}`)}, {key: "c", value: []byte(`[1,2]`)}})
	require.NoError(f, err)
	f.Add(valid)
	f.Add(valid[:len(valid)-1])
	f.Add([]byte(recordTag))
	f.Add([]byte(recordTag + "\x02"))
	f.Add([]byte("\x00"))
	f.Add([]byte{0x0c, 0xff, 0x81, 0x04, 0x01, 0x02, 0xff, 0x82, 0x00, 0x01, 0x10, 0x01})
	f.Add([]byte("garbage"))
	f.Add([]byte{})

	load := func(t *testing.T, store *Store) (*Session, error) {
		t.Helper()
		ctx := apptest.Ctx(t, app)
		ctx.Request().Header.SetCookie("session_id", "stored")
		return store.Get(ctx)
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 4096 {
			return
		}
		require.NoError(t, mem.Set(sessionKey("stored"), raw, time.Hour))

		// A record is not for a store without a codec: it is no session of its.
		if isRecord(raw) {
			plain, err := load(t, plainStore)
			require.NoError(t, err)
			require.True(t, plain.Fresh())
			require.NotEqual(t, "stored", plain.ID())
			plain.Release()
		}

		sess, err := load(t, codecStore)
		if err != nil {
			require.Nil(t, sess)
			return
		}
		defer sess.Release()

		_, coded := sess.data.format.(*namedCodec)
		for _, key := range sess.Keys() {
			if coded {
				_, ok := key.(string)
				require.True(t, ok, "a session with a codec has string keys, got %T", key)
			}
			if name, ok := key.(string); ok {
				var anything any
				if err := sess.GetInto(name, &anything); err != nil {
					require.NotEmpty(t, err.Error())
				}
				_, _ = GetAs[string](sess, name)
				_ = sess.Get(name)
			}
		}

		// Saving what was only read gives the bytes back, whatever they were. (A session that is
		// new, or whose deadline has passed, has another ID, and keeps nothing.)
		id := sess.ID()
		require.NoError(t, sess.Save())
		if id == "stored" {
			stored := storedBytes(t, mem, id)
			if isRecord(raw) {
				require.Equal(t, raw, stored)
			}
		}
		if !coded {
			return
		}

		// A change keeps the rest.
		before, _ := collectRecordOf(t, mem, id)
		sess.Set("fuzz", 1)
		require.NoError(t, sess.Save())
		after, ok := collectRecordOf(t, mem, id)
		require.True(t, ok)
		for _, entry := range before {
			if entry.key == "fuzz" {
				continue
			}
			found := false
			for _, other := range after {
				if other.key == entry.key {
					require.Equal(t, entry.value, other.value, entry.key)
					found = true
				}
			}
			require.True(t, found, "%q survives a change of another key", entry.key)
		}
	})
}

// collectRecordOf parses the record stored under id, if it is one.
func collectRecordOf(t *testing.T, mem *memory.Storage, id string) ([]recordEntry, bool) {
	t.Helper()
	raw := storedBytes(t, mem, id)
	if !isRecord(raw) {
		return nil, false
	}
	_, entries, err := collectRecord(raw)
	require.NoError(t, err)
	return entries, true
}

// benchSession is the session of a signed-in user: five values, one of them a struct.
func benchSession(s *Session) {
	s.Set("user_id", 42)
	s.Set("name", "alice")
	s.Set("admin", true)
	s.Set("csrf", "0123456789abcdef0123456789abcdef")
	s.Set("profile", newCodecProfile())
}

// Benchmark_Codec_Request compares what a request pays for its session: a read of one value, a read
// of every value, and a change of one value, with a store that keeps gob and one with a codec.
func Benchmark_Codec_Request(b *testing.B) {
	app := fiber.New()
	gobStore := NewStore(Config{Storage: memory.New()})
	gobStore.RegisterType(codecProfile{})

	// requests is what a request does with its session, between loading it and saving it. A request
	// that reads reports whether it found the values.
	requests := []struct {
		do   func(sess *Session) bool
		name string
	}{
		{name: "read one", do: func(sess *Session) bool {
			_, ok := GetAs[string](sess, "name")
			return ok
		}},
		{name: "read all", do: func(sess *Session) bool {
			_, userID := GetAs[int](sess, "user_id")
			_, name := GetAs[string](sess, "name")
			_, admin := GetAs[bool](sess, "admin")
			_, csrf := GetAs[string](sess, "csrf")
			_, profile := GetAs[codecProfile](sess, "profile")
			return userID && name && admin && csrf && profile
		}},
		{name: "change one", do: func(sess *Session) bool {
			sess.Set("user_id", 43)
			return true
		}},
	}

	for _, tc := range []struct {
		store *Store
		name  string
	}{
		{store: gobStore, name: "gob"},
		{store: NewStore(Config{Storage: memory.New(), Codec: JSONCodec{}}), name: "json"},
	} {
		id := seedSession(b, app, tc.store, benchSession)

		for _, request := range requests {
			b.Run(request.name+"/"+tc.name, func(b *testing.B) {
				// Every benchmark has a request of its own: a context caches the session ID of the
				// last store that read it.
				ctx := apptest.Ctx(b, app)
				ctx.Request().Header.SetCookie("session_id", id)
				b.ReportAllocs()
				for b.Loop() {
					sess, err := tc.store.Get(ctx)
					if err != nil {
						b.Fatal(err)
					}
					if !request.do(sess) {
						b.Fatal("a value of the session is not found")
					}
					if err := sess.Save(); err != nil {
						b.Fatal(err)
					}
					sess.Release()
				}
			})
		}
	}
}

// benchBig is a value that is costly to copy.
type benchBig struct {
	Name  string
	Bytes [1024]byte
}

// The benchmarks below keep what they read, so that a copy that is not needed is not left out.
var (
	sinkInt    int
	sinkBig    benchBig
	sinkBigPtr *benchBig
	sinkOK     bool
	errSink    error
)

// Benchmark_Codec_ReadForms compares the ways to read a value of a store without a codec, where the
// value is already decoded: the type assertion, GetAs and GetInto. The assertion copies a value
// once, GetAs hands it out as a result and the caller copies it again, and GetInto copies it into
// the variable the caller provides.
func Benchmark_Codec_ReadForms(b *testing.B) {
	sess := mustGet(b, NewStore(), apptest.Ctx(b, fiber.New()))
	sess.Set("n", 7)
	sess.Set("big", benchBig{Name: "big"})
	sess.Set("bigPointer", &benchBig{Name: "big"})

	b.Run("int/assertion", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkInt, sinkOK = sess.Get("n").(int)
		}
	})
	b.Run("int/GetAs", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkInt, sinkOK = GetAs[int](sess, "n")
		}
	})
	b.Run("int/GetInto", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			errSink = sess.GetInto("n", &sinkInt)
		}
	})
	b.Run("1KB struct/assertion", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkBig, sinkOK = sess.Get("big").(benchBig)
		}
	})
	b.Run("1KB struct/GetAs", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkBig, sinkOK = GetAs[benchBig](sess, "big")
		}
	})
	b.Run("1KB struct/GetInto", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			errSink = sess.GetInto("big", &sinkBig)
		}
	})
	b.Run("1KB struct pointer/assertion", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkBigPtr, sinkOK = sess.Get("bigPointer").(*benchBig)
		}
	})
	b.Run("1KB struct pointer/GetAs", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkBigPtr, sinkOK = GetAs[*benchBig](sess, "bigPointer")
		}
	})
}

// benchLarge is a value of about 8 KB.
type benchLarge struct {
	Extra  map[string]bool `json:"extra"`
	Name   string          `json:"name"`
	Values [1024]int64     `json:"values"`
}

// Benchmark_Codec_GetIntoLarge reads a value of about 8 KB from a store with a codec into a
// variable that holds its zero value, which is decoded into directly, into one that holds
// something, which is replaced by a new value, and with GetAs, which returns the value for the
// caller to copy.
func Benchmark_Codec_GetIntoLarge(b *testing.B) {
	sess := mustGet(b, NewStore(Config{Codec: JSONCodec{}}), apptest.Ctx(b, fiber.New()))

	large := benchLarge{Name: "large", Extra: map[string]bool{"a": true}}
	for i := range large.Values {
		large.Values[i] = int64(i) * 1000
	}
	sess.Set("large", large)

	var out benchLarge
	b.Run("zero target", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			out = benchLarge{}
			errSink = sess.GetInto("large", &out)
		}
	})
	b.Run("populated target", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			errSink = sess.GetInto("large", &out)
		}
	})
	b.Run("GetAs", func(b *testing.B) {
		b.ReportAllocs()
		var got benchLarge
		for b.Loop() {
			got, sinkOK = GetAs[benchLarge](sess, "large")
		}
		out = got
	})
}

// testRecordMagic is the tag and the version of a record.
const testRecordMagic = recordTag + "\x01"

// collectRecord parses b and returns the header and the entries, with the values copied.
func collectRecord(b []byte) (recordHeader, []recordEntry, error) {
	var entries []recordEntry
	h, err := parseRecord(b, func(key, value []byte, _ int) {
		entries = append(entries, recordEntry{key: string(key), value: slices.Clone(value)})
	})
	return h, entries, err
}

// mustEncodeRecord is encodeRecord, which has to succeed.
func mustEncodeRecord(tb testing.TB, codec string, deadline recordDeadline, entries []recordEntry) []byte {
	tb.Helper()
	b, err := encodeRecord(codec, deadline, entries)
	require.NoError(tb, err)
	return b
}

// The deadline of a record is the absolute deadline of the session: the zero time is none, and any
// other time reads back as it was, down to the nanosecond.
func Test_Record_DeadlineOf(t *testing.T) {
	t.Parallel()
	require.Equal(t, recordDeadline{}, deadlineOf(time.Time{}), "the zero time is no deadline")

	at := time.Date(2026, time.October, 29, 12, 0, 0, 123456789, time.UTC)
	got := deadlineOf(at)
	require.Equal(t, recordDeadline{sec: at.Unix(), nanos: 123456789, set: true}, got)
	require.True(t, got.valid())
	require.True(t, at.Equal(got.time()), "%v != %v", at, got.time())
}

// The bounds of the deadline of a record are the years 1 and 9999. They are spelled out here and
// not written through the constants that define them.
const (
	zeroTimeSec = -62_135_596_800 // the zero time: the first second of the year 1
	lastSec     = 253_402_300_799 // the last second of the year 9999
)

// A record of every shape is written into a buffer of exactly its size, reads back as what it was
// made of, has one spelling and hands out entries that cannot be appended to in place. The shapes are the edges of the varints (sizes, counts, lengths
// of names) and of the deadline (the years 1 and 9999).
func Test_Record_OfEveryShape(t *testing.T) {
	t.Parallel()
	// Each key is a prefix of the next one, which puts them in ascending order.
	var sizes []recordEntry
	for _, size := range []int{0, 1, 127, 128, 16_383, 16_384} {
		sizes = append(sizes, recordEntry{key: strings.Repeat("k", size), value: bytes.Repeat([]byte("v"), size)})
	}
	many := make([]recordEntry, 0, 130)
	for i := range 130 {
		many = append(many, recordEntry{key: fmt.Sprintf("%03d", i), value: []byte{byte(i)}})
	}

	cases := []struct {
		name     string
		codec    string // "json" when empty
		entries  []recordEntry
		deadline recordDeadline
	}{
		{name: "empty", codec: "j"},
		{name: "one entry", entries: []recordEntry{{key: "a", value: []byte("1")}}},
		{
			name:    "three entries",
			entries: []recordEntry{{key: "a", value: []byte("one")}, {key: "b", value: []byte("two")}, {key: "c", value: []byte("three")}},
		},
		{name: "empty key and value", entries: []recordEntry{{key: "", value: []byte{}}, {key: "a", value: []byte{}}}},
		{
			name:    "binary",
			entries: []recordEntry{{key: "\x00\xff", value: []byte{0, 1, 2, 255}}, {key: "\xff", value: []byte{0}}},
		},
		{name: "sizes at the edges of the varints", entries: sizes},
		{name: "count at the edge of a varint", entries: many[:127]},
		{name: "count past the edge of a varint", entries: many},
		{name: "long value", entries: []recordEntry{{key: "k", value: bytes.Repeat([]byte("x"), 70_000)}}},
		{name: "long name", codec: strings.Repeat("n", 128)},
		{name: "longest name", codec: strings.Repeat("n", maxCodecNameLen)},
		{name: "deadline", deadline: recordDeadline{sec: 1_700_000_000, nanos: 999_999_999, set: true}},
		{name: "deadline at the epoch", deadline: recordDeadline{set: true}},
		{name: "deadline before the epoch", deadline: recordDeadline{sec: -1, nanos: 1, set: true}},
		{name: "a second after the zero time", deadline: recordDeadline{sec: zeroTimeSec + 1, set: true}},
		{
			name:     "first deadline: a nanosecond after the zero time",
			deadline: recordDeadline{sec: zeroTimeSec, nanos: 1, set: true},
		},
		{
			name:     "last deadline: the last nanosecond of the year 9999",
			deadline: recordDeadline{sec: lastSec, nanos: 999_999_999, set: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			codec := cmp.Or(tc.codec, "json")
			encoded := mustEncodeRecord(t, codec, tc.deadline, tc.entries)
			original := slices.Clone(encoded)
			require.True(t, isRecord(encoded))
			require.Equal(t, len(encoded), cap(encoded), "no growth, and no spare room that a storage which keeps the slice would hold on to")

			h, entries, err := collectRecord(encoded)
			require.NoError(t, err)
			require.Equal(t, codec, string(h.codec))
			require.Equal(t, tc.deadline, h.deadline)
			require.Equal(t, tc.entries, entries)

			// An entry has no capacity beyond its length: a codec that keeps a slice of the record
			// and appends to it cannot overwrite the entry that follows.
			seen := 0
			_, err = parseRecord(encoded, func(key, value []byte, _ int) {
				seen++
				require.Equal(t, len(key), cap(key))
				require.Equal(t, len(value), cap(value))
				_ = append(value, "overwrite"...) // an append has to copy, and leave the record alone
				_ = append(key, "overwrite"...)
			})
			require.NoError(t, err)
			require.Equal(t, len(tc.entries), seen)
			require.Equal(t, original, encoded)

			// One record has one spelling.
			again := mustEncodeRecord(t, codec, h.deadline, entries)
			require.Equal(t, encoded, again)
		})
	}
}

// uvarintLen and zigzag size the buffer of encodeRecord, so they have to agree with what
// encoding/binary writes at the edges of every length.
func Test_Record_VarintSizes(t *testing.T) {
	t.Parallel()
	for shift := range 64 {
		for _, x := range []uint64{1<<shift - 1, 1 << shift, 1<<shift + 1} {
			require.Len(t, binary.AppendUvarint(nil, x), uvarintLen(x), "%d", x)
		}
	}
	require.Len(t, binary.AppendUvarint(nil, math.MaxUint64), uvarintLen(math.MaxUint64))

	for shift := range 63 {
		for _, x := range []int64{1<<shift - 1, 1 << shift, 1<<shift + 1} {
			for _, v := range []int64{x, -x, -x - 1} {
				require.Equal(t, binary.AppendVarint(nil, v), binary.AppendUvarint(nil, zigzag(v)), "%d", v)
			}
		}
	}
	for _, v := range []int64{math.MaxInt64, math.MinInt64} {
		require.Equal(t, binary.AppendVarint(nil, v), binary.AppendUvarint(nil, zigzag(v)), "%d", v)
	}
}

func Test_Record_EncodeRefusesWhatItCannotParse(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		codec    string
		entries  []recordEntry
		deadline recordDeadline
	}{
		"empty codec":                  {codec: ""},
		"long codec":                   {codec: strings.Repeat("c", maxCodecNameLen+1)},
		"unsorted keys":                {codec: "c", entries: []recordEntry{{key: "b"}, {key: "a"}}},
		"duplicate keys":               {codec: "c", entries: []recordEntry{{key: "a"}, {key: "a"}}},
		"nanoseconds too big":          {codec: "c", deadline: recordDeadline{set: true, nanos: 1_000_000_000}},
		"deadline after the year 9999": {codec: "c", deadline: recordDeadline{set: true, sec: lastSec + 1}},
		"deadline before the year 1":   {codec: "c", deadline: recordDeadline{set: true, sec: zeroTimeSec - 1}},
		"deadline that wraps":          {codec: "c", deadline: recordDeadline{set: true, sec: math.MaxInt64}},
		"the zero time":                {codec: "c", deadline: recordDeadline{set: true, sec: zeroTimeSec}},
		"seconds without a deadline":   {codec: "c", deadline: recordDeadline{sec: 5}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := encodeRecord(tc.codec, tc.deadline, tc.entries)
			require.ErrorIs(t, err, errInvalidRecord)
			require.Nil(t, out)
		})
	}
}

func Test_Record_Malformed(t *testing.T) {
	t.Parallel()
	magic := []byte(testRecordMagic)
	build := func(parts ...[]byte) []byte { return slices.Concat(append([][]byte{magic}, parts...)...) }
	uv := func(v uint64) []byte { return binary.AppendUvarint(nil, v) }
	name := slices.Concat(uv(4), []byte("json"))
	entry := func(key, value string) []byte {
		return slices.Concat(uv(uint64(len(key))), []byte(key), uv(uint64(len(value))), []byte(value))
	}

	cases := map[string][]byte{
		"empty":                               nil,
		"wrong first byte":                    append([]byte{1}, magic[1:]...),
		"tag only":                            []byte(recordTag),
		"gob stream":                          {0x0c, 0xff, 0x81, 0x04, 0x01, 0x02, 0xff, 0x82, 0x00, 0x01, 0x10, 0x01},
		"empty codec name":                    build(uv(0), []byte{0}, uv(0)),
		"codec name too long":                 build(uv(maxCodecNameLen+1), bytes.Repeat([]byte("c"), maxCodecNameLen+1), []byte{0}, uv(0)),
		"codec name past the end":             build(uv(9), []byte("json")),
		"overlong name length":                build([]byte{0x84, 0x00}, []byte("json"), []byte{0}, uv(0)),
		"unknown deadline flag":               build(name, []byte{2}, uv(0)),
		"deadline without seconds":            build(name, []byte{1}),
		"deadline without nanos":              build(name, []byte{1}, binary.AppendVarint(nil, 5)),
		"nanoseconds out of range":            build(name, []byte{1}, binary.AppendVarint(nil, 5), uv(nanosPerSecond), uv(0)),
		"overlong seconds":                    build(name, []byte{1}, []byte{0x84, 0x00}, uv(0), uv(0)),
		"overlong count":                      build(name, []byte{0}, []byte{0x80, 0x00}),
		"count larger than record":            build(name, []byte{0}, uv(2), entry("a", "b")),
		"count of a billion":                  build(name, []byte{0}, uv(1_000_000_000)),
		"count overflows":                     build(name, []byte{0}, bytes.Repeat([]byte{0xff}, 10)),
		"key longer than record":              build(name, []byte{0}, uv(1), uv(1<<40), []byte("a")),
		"value longer than record":            build(name, []byte{0}, uv(1), uv(1), []byte("a"), uv(1<<40)),
		"deadline after the year 9999":        build(name, []byte{1}, binary.AppendVarint(nil, maxDeadlineSec+1), uv(0), uv(0)),
		"deadline before the year 1":          build(name, []byte{1}, binary.AppendVarint(nil, minDeadlineSec-1), uv(0), uv(0)),
		"the zero time, which is no deadline": build(name, []byte{1}, binary.AppendVarint(nil, minDeadlineSec), uv(0), uv(0)),
		"deadline that wraps":                 build(name, []byte{1}, binary.AppendVarint(nil, math.MaxInt64), uv(0), uv(0)),
		"deadline that wraps the other":       build(name, []byte{1}, binary.AppendVarint(nil, math.MinInt64), uv(0), uv(0)),
		"keys out of order":                   build(name, []byte{0}, uv(2), entry("b", "1"), entry("a", "2")),
		"duplicate keys":                      build(name, []byte{0}, uv(2), entry("a", "1"), entry("a", "2")),
		"entry without value":                 build(name, []byte{0}, uv(1), uv(1), []byte("a")),
		"trailing byte":                       build(name, []byte{0}, uv(1), entry("a", "1"), []byte{0}),
		"trailing byte after empty":           build(name, []byte{0}, uv(0), []byte{0}),
	}

	// Every proper prefix of a record is refused, and so is the record with a byte added: nothing
	// may be accepted that is not exactly what was written.
	whole := mustEncodeRecord(t, "json", recordDeadline{sec: 123, nanos: 456, set: true},
		[]recordEntry{{key: "a", value: []byte("one")}, {key: "b", value: []byte("two")}, {key: "c", value: nil}})
	for n := range whole {
		cases[fmt.Sprintf("prefix of %d bytes", n)] = whole[:n]
	}
	cases["a byte added"] = append(slices.Clone(whole), 0)

	for label, b := range cases {
		_, _, err := collectRecord(b)
		require.ErrorIs(t, err, errInvalidRecord, label)
	}

	// A version that is not known is refused for what it is.
	_, _, err := collectRecord(append(slices.Clone(magic[:len(magic)-1]), 2, 4, 'j', 's', 'o', 'n', 0, 0))
	require.ErrorIs(t, err, errUnsupportedRecord)
	require.ErrorIs(t, err, errInvalidRecord)

	// The same pieces, well formed, are accepted.
	_, entries, err := collectRecord(build(name, []byte{0}, uv(2), entry("a", "1"), entry("b", "2")))
	require.NoError(t, err)
	require.Len(t, entries, 2)
}

// A gob stream is never taken for a record, whatever is in the map.
func Test_Record_GobStreamsDoNotLookLikeRecords(t *testing.T) {
	t.Parallel()
	gob.Register(struct{ A int }{})
	gob.Register(map[string]any{})
	gob.Register([]any{})
	for _, v := range []map[any]any{
		{},
		{"a": 1},
		{"a": "b", 1: 2.5, true: []byte("x")},
		{"nested": map[string]any{"k": []any{1, "two", 3.0}}},
		{"big": strings.Repeat("x", 200_000)},
		{"s": struct{ A int }{A: 7}},
	} {
		var buf bytes.Buffer
		require.NoError(t, gob.NewEncoder(&buf).Encode(&v))
		require.NotEmpty(t, buf.Bytes())
		require.NotEqual(t, testRecordMagic[0], buf.Bytes()[0])
		require.False(t, isRecord(buf.Bytes()))
	}

	// The tag alone is a record, a damaged one; a byte less is not one.
	require.True(t, isRecord([]byte(recordTag)))
	require.False(t, isRecord([]byte(recordTag)[:len(recordTag)-1]))
}

// A store without a Codec stores the gob stream it always did, byte for byte, so that a release
// with the codecs reads what one without them stored, and the other way round. A session of one
// key encodes to the same bytes every time (a map of more keys is written in no fixed order), for
// the first save and for a save that encodes the data again after a read of a value that could be
// changed in place.
func Test_Session_WithoutACodecStoresTheGobStreamAsBefore(t *testing.T) {
	t.Parallel()
	type profile struct {
		Name string
		Age  int
	}
	for name, value := range map[string]any{
		"string": "alice",
		"int":    42,
		"float":  1.5,
		"bool":   true,
		"bytes":  []byte("raw"),
		"slice":  []string{"a", "b"},
		"map":    map[string]int{"a": 1},
		"struct": profile{Name: "bob", Age: 41},
		"time":   time.Date(2026, 11, 4, 12, 0, 0, 7, time.UTC),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mem := storagetest.New()
			store := NewStore(Config{Storage: mem})
			store.RegisterType(value)
			app := fiber.New()
			id := seedSession(t, app, store, func(s *Session) { s.Set("k", value) })

			var want bytes.Buffer
			require.NoError(t, gob.NewEncoder(&want).Encode(map[any]any{"k": value}))
			stored, err := mem.Get(string(store.storageKey(id)))
			require.NoError(t, err)
			require.Equal(t, want.Bytes(), stored, "the first save")

			sess := loadSession(t, app, store, id)
			_ = sess.Get("k")
			require.NoError(t, sess.Save())
			stored, err = mem.Get(string(store.storageKey(id)))
			require.NoError(t, err)
			require.Equal(t, want.Bytes(), stored, "a save after a read")
		})
	}
}

func FuzzRecordParse(f *testing.F) {
	f.Add([]byte(nil))
	f.Add([]byte(testRecordMagic))
	f.Add(mustEncodeRecord(f, "json", recordDeadline{}, nil))
	f.Add(mustEncodeRecord(f, "json", recordDeadline{sec: 1, nanos: 2, set: true},
		[]recordEntry{{key: "", value: nil}, {key: "a", value: []byte("b")}, {key: "b", value: bytes.Repeat([]byte("v"), 300)}}))
	f.Add(slices.Concat([]byte(testRecordMagic), []byte{4}, []byte("json"), []byte{0}, binary.AppendUvarint(nil, 1<<62)))

	f.Fuzz(func(t *testing.T, b []byte) {
		before := slices.Clone(b)
		h, entries, err := collectRecord(b)
		require.Equal(t, before, b, "parsing must not change its input")
		if err != nil {
			require.ErrorIs(t, err, errInvalidRecord)
			return
		}

		// What parses is exactly what encodeRecord would have written.
		again, err := encodeRecord(string(h.codec), h.deadline, entries)
		require.NoError(t, err)
		require.Equal(t, b, again)

		// No entry is larger than the record it came from.
		total := 0
		for _, e := range entries {
			total += len(e.key) + len(e.value)
		}
		require.LessOrEqual(t, total, len(b))
	})
}

// Parsing a record takes no memory, whatever the record claims to hold.
func Test_Record_ParseAllocatesNothing(t *testing.T) {
	// testing.AllocsPerRun panics in a parallel test, so this one stays sequential.
	valid := mustEncodeRecord(t, "json", recordDeadline{sec: 1, set: true}, []recordEntry{
		{key: "a", value: []byte("one")}, {key: "b", value: []byte("two")},
	})
	noop := func(_, _ []byte, _ int) {}
	require.Zero(t, testing.AllocsPerRun(100, func() {
		if _, err := parseRecord(valid, noop); err != nil {
			t.Error(err)
		}
	}))

	// A record that claims a billion entries, or entries as long as the address space.
	claims := slices.Concat([]byte(testRecordMagic), []byte{4}, []byte("json"), []byte{0}, binary.AppendUvarint(nil, 1<<30), make([]byte, 1<<20))
	require.Zero(t, testing.AllocsPerRun(10, func() {
		_, err := parseRecord(claims, noop)
		if err == nil {
			t.Error("a record that claims what it does not hold was accepted")
		}
	}))
}

func Test_Record_CodecOf(t *testing.T) {
	t.Parallel()
	encoded := mustEncodeRecord(t, "msgpack", recordDeadline{}, nil)
	version, codec, ok := recordCodecOf(encoded)
	require.True(t, ok)
	require.EqualValues(t, recordVersion, version)
	require.Equal(t, "msgpack", string(codec))

	// A version that is not known: no name is read.
	unknown := slices.Clone(encoded)
	unknown[len(recordTag)] = 9
	version, codec, ok = recordCodecOf(unknown)
	require.True(t, ok)
	require.EqualValues(t, 9, version)
	require.Nil(t, codec)

	for name, b := range map[string][]byte{
		"nothing":        nil,
		"a gob stream":   {0x0d, 0x7f, 0x04, 0x01},
		"the tag only":   []byte(recordTag),
		"no name":        []byte(testRecordMagic),
		"a long name":    slices.Concat([]byte(testRecordMagic), binary.AppendUvarint(nil, 99), []byte("short")),
		"an empty name":  slices.Concat([]byte(testRecordMagic), []byte{0}),
		"a name too big": slices.Concat([]byte(testRecordMagic), binary.AppendUvarint(nil, maxCodecNameLen+1), make([]byte, maxCodecNameLen+1)),
	} {
		_, _, ok := recordCodecOf(b)
		require.False(t, ok, name)
	}
}

// What loadRecord cannot read it refuses, whatever the store did before it: a session without a
// codec, and a record of another codec. A failed decode leaves no entries behind.
func Test_Session_DecodeRecordRefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()
	record := mustEncodeRecord(t, "json", recordDeadline{}, []recordEntry{{key: "a", value: []byte("1")}})
	damaged := append(bytes.Clone(record), 0) // a byte too many, found after the entries were added

	for _, tc := range []struct {
		format format
		err    error
		name   string
		record []byte
	}{
		{name: "a session without a codec", format: gobStream, record: record, err: errRecordNeedsCodec},
		{
			name:   "a record of another codec",
			format: &namedCodec{Codec: JSONCodec{}, name: "other"},
			record: record,
			err:    errCodecMismatch,
		},
		{
			name:   "a damaged record leaves no entries",
			format: &namedCodec{Codec: JSONCodec{}, name: "json"},
			record: damaged,
			err:    errInvalidRecord,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sess := acquireSession()
			defer sess.Release()
			sess.data.format = tc.format
			require.ErrorIs(t, sess.decodeSessionData(tc.record), tc.err)
			require.Empty(t, sess.data.Data, "nothing of the record is left")
		})
	}
}

// A record is built into one buffer of the size it needs. AllocsPerRun cannot run in a parallel
// test, so this one is sequential.
func Test_Record_AppendAllocatesOnce(t *testing.T) { // not parallel: testing.AllocsPerRun panics in a parallel test
	entries := make([]recordEntry, 0, 100)
	for i := range 100 {
		entries = append(entries, recordEntry{key: fmt.Sprintf("key-%03d", i), value: bytes.Repeat([]byte("v"), 32)})
	}
	var sink []byte
	allocs := testing.AllocsPerRun(50, func() {
		var err error
		sink, err = encodeRecord("json", recordDeadline{sec: 1_700_000_000, nanos: 5, set: true}, entries)
		if err != nil {
			panic(err)
		}
	})
	require.NotEmpty(t, sink)
	require.InDelta(t, 1, allocs, 0, "one buffer, sized in advance")
}
