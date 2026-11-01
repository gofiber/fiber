package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/loggertest"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type codecProfile struct {
	At    time.Time         `json:"at"`
	Prefs map[string]string `json:"prefs"`
	Name  string            `json:"name"`
	Roles []string          `json:"roles"`
	ID    int               `json:"id"`
}

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
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal(failure)
	}
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
		require.IsType(t, gobFormat{}, sess.data.format, "a session that was stored as gob is gob, whatever the store now does")

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
		require.IsType(t, gobFormat{}, destroyed.data.format, "a session that was stored as gob stays gob")
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

// Not parallel: the log output is process-global, so this test captures it.
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
func Test_Codec_GetByIDWarnsAboutARecordItCannotRead(t *testing.T) { // not parallel: the log is captured for the whole process
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

func Test_Codec_CorruptRecordsAreErrors(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	good := storedBytes(t, mem, seedSession(t, app, store, func(s *Session) { s.Set("k", "v") }))
	// loadsAs is the error of loading raw as a stored session through a request that carries its cookie.
	loadsAs := func(raw []byte) error {
		require.NoError(t, mem.Set(sessionKey("cut"), raw, time.Hour))
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)
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
			rec := newRecordingStorage()
			store := NewStore(Config{Storage: rec, Codec: JSONCodec{}})
			sess := loadSession(t, app, store, "")

			sess.Set("fine", 1)
			tc.set(sess)
			sess.Set("alsoFine", 2)
			err := sess.Save()
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
			require.Empty(t, rec.writes(), "nothing is written")

			// Other keys do not make up for it...
			sess.Set("fine", 3)
			require.Error(t, sess.Save())
			require.Empty(t, rec.writes())

			// ...the key does, as it does for gob, which fails the save only while the bad value is in.
			tc.fix(sess)
			require.NoError(t, sess.Save())
			require.Len(t, rec.writes(), 1)
			_, entries := storedRecord(t, rec, sess.ID())
			require.Len(t, entries, tc.entries)
		})

		// A Set that fails on a session that was loaded, and not changed otherwise, still fails the
		// save: the snapshot is not written back over it.
		t.Run(name+", on a loaded session", func(t *testing.T) {
			t.Parallel()

			rec := newRecordingStorage()
			sess := seedAndLoad(t, NewStore(Config{Storage: rec, Codec: JSONCodec{}}), func(s *Session) { s.Set("a", 1) })
			rec.reset()

			tc.set(sess)
			require.Error(t, sess.Save())
			require.Empty(t, rec.writes(), "nothing is written")
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
		resp, _ := sendRequest(t, app, "/", nil)
		require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
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
		raw, err := appendRecord(nil, "json", recordDeadline{sec: past.Unix(), nanos: uint32(past.Nanosecond()), set: true},
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

	resp, body = sendRequest(t, app, "/me", nil)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
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
				require.ErrorIs(t, sess.GetInto("missing", &keep), ErrKeyNotFound)
				require.Equal(t, "keep", keep)
			})

			t.Run("the target has to be a non-nil pointer", func(t *testing.T) {
				t.Parallel()
				var nilPointer *string
				var name string
				require.Error(t, sess.GetInto("name", name))
				require.Error(t, sess.GetInto("name", nilPointer))
				require.Error(t, sess.GetInto("name", nil))
				require.Error(t, sess.GetInto("missing", nil), "a bad target is reported before a missing key")
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
		require.IsType(t, gobFormat{}, sess.data.format, "it is a session that was stored as gob")

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
		codec, sess := setup(t, errors.New("boom"))
		var got codecProfile
		err := sess.GetInto("user", &got)
		require.ErrorContains(t, err, "boom")
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
		_, sess := setup(t, errors.New("boom"))
		got := codecProfile{ID: 99}
		require.Error(t, sess.GetInto("user", &got))
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
	raw, err := appendRecord(nil, "json", recordDeadline{}, []recordEntry{
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
	ctx := newRequest(t, app)
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

			m := startMiddleware(t, newRequest(t, fiber.New()), NewStore(Config{Codec: codec}))
			m.Set("k", "v")

			reads := readsOf(m, "k")
			reads["lookup"] = func() { _ = m.lookup("k") }
			reads["fetch"] = func() { _, _ = m.fetch("k") } //nolint:errcheck // only the locking matters here
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
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatalf("%s did not finish after the lock was given back", readName)
				}
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
			d.resetAs(codec)
			done := make(chan struct{})
			go func() {
				defer close(done)
				d.Set("late", "v")
			}()
			<-entered
			d.resetAs(gobFormat{}) // the session is released while the codec runs
			close(leave)
			<-done

			require.Empty(t, d.Data, "a value for a session that is gone")
			require.NoError(t, d.pendingError(), "an error of a Set of a session that is gone")
			releaseData(d)
		})
	}
}
