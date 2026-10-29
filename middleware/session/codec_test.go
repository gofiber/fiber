package session

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
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

// countingCodec counts the calls the session makes to its codec.
type countingCodec struct {
	Codec
	marshals atomic.Int64
	decodes  atomic.Int64
}

func newCountingCodec() *countingCodec { return &countingCodec{Codec: JSONCodec{}} }

func (c *countingCodec) Marshal(v any) ([]byte, error) {
	c.marshals.Add(1)
	return c.Codec.Marshal(v)
}

func (c *countingCodec) Unmarshal(data []byte, v any) error {
	c.decodes.Add(1)
	return c.Codec.Unmarshal(data, v)
}

// reusingCodec hands out the same buffer for every value it encodes, as a codec
// built on a pooled buffer may.
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
func sessionKey(id string) string { return storageKeyPrefix + id }

// storedRecord returns the record stored under id.
func storedRecord(t *testing.T, mem *memory.Storage, id string) (recordHeader, []recordEntry) {
	t.Helper()
	raw, err := mem.Get(sessionKey(id))
	require.NoError(t, err)
	require.True(t, isRecord(raw), "stored: %q", raw)
	header, entries, err := collectRecord(raw)
	require.NoError(t, err)
	return header, entries
}

// loadErr is the error of loading the session of id through a request that carries its cookie.
func loadErr(app *fiber.App, store *Store, id string) error {
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.SetCookie("session_id", id)

	sess, err := store.Get(ctx)
	if err == nil {
		sess.Release()
	}
	return err
}

func Test_Codec_StoresEachValueEncoded(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	profile := newCodecProfile()
	id := seedSession(t, app, store, func(s *Session) {
		s.Set("profile", profile)
		s.Set("visits", 3)
		s.Set("name", "alice")
	})

	header, entries := storedRecord(t, mem, id)
	require.Equal(t, "json", string(header.codec))
	require.False(t, header.deadline.set)
	require.Len(t, entries, 3)
	require.Equal(t, "name", entries[0].key)
	require.Equal(t, "profile", entries[1].key)
	require.Equal(t, "visits", entries[2].key)
	require.Equal(t, `"alice"`, string(entries[0].value))
	require.Equal(t, `3`, string(entries[2].value))
	want, err := json.Marshal(profile)
	require.NoError(t, err)
	require.Equal(t, string(want), string(entries[1].value))
}

func Test_Codec_TypedAndGenericReads(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore(Config{Codec: JSONCodec{}})
	profile := newCodecProfile()
	id := seedSession(t, app, store, func(s *Session) {
		s.Set("profile", profile)
		s.Set("visits", 3)
		s.Set("name", "alice")
		s.Set("nothing", nil)
	})
	sess := loadSession(t, app, store, id)

	var got codecProfile
	require.NoError(t, sess.GetInto("profile", &got))
	require.Equal(t, profile, got)

	visits, ok := GetAs[int](sess, "visits")
	require.True(t, ok)
	require.Equal(t, 3, visits)

	fromGet, ok := GetAs[codecProfile](sess, "profile")
	require.True(t, ok)
	require.Equal(t, profile, fromGet)

	_, ok = GetAs[int](sess, "name")
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

	require.ElementsMatch(t, []any{"name", "nothing", "profile", "visits"}, sess.Keys())
}

func Test_Codec_ReadsDecodeIntoFreshValues(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore(Config{Codec: JSONCodec{}})
	id := seedSession(t, app, store, func(s *Session) { s.Set("list", []string{"a", "b"}) })
	sess := loadSession(t, app, store, id)

	first, ok := GetAs[[]string](sess, "list")
	require.True(t, ok)
	first[0] = "changed"

	second, ok := GetAs[[]string](sess, "list")
	require.True(t, ok)
	require.Equal(t, []string{"a", "b"}, second, "a value read does not alias the session")

	// And a read that cannot change the session leaves it unchanged, so a save writes the snapshot back.
	require.False(t, sess.data.dirty.Load())
}

func Test_Codec_SetStoresASnapshot(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore(Config{Codec: JSONCodec{}})
	sess := loadSession(t, app, store, "")

	list := []string{"a"}
	sess.Set("list", list)
	list[0] = "changed"
	list = append(list, "more")

	got, ok := GetAs[[]string](sess, "list")
	require.True(t, ok)
	require.Equal(t, []string{"a"}, got, "what was set is what was encoded then")
	_ = list
}

func Test_Codec_DefaultStoreWritesPlainGob(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem})
	id := seedSession(t, app, store, func(s *Session) { s.Set("k", "v") })

	raw, err := mem.Get(sessionKey(id))
	require.NoError(t, err)
	require.False(t, isRecord(raw))

	var want bytes.Buffer
	single := map[any]any{"k": "v"}
	require.NoError(t, gob.NewEncoder(&want).Encode(&single))
	require.Equal(t, want.Bytes(), raw, "the default format is the gob stream of a map[any]any")

	// A gob stream written by an independent encoder is read as it always was.
	independent := map[any]any{"n": 7, "s": "x", true: 1.5}
	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(&independent))
	require.NoError(t, mem.Set(sessionKey("legacy"), buf.Bytes(), time.Hour))
	sess := loadSession(t, app, store, "legacy")
	require.Equal(t, 7, sess.Get("n"))
	require.Equal(t, "x", sess.Get("s"))
	require.InDelta(t, 1.5, sess.Get(true), 0)
}

func Test_Codec_GobSessionsStayGob(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	mem := memory.New()
	gobStore := NewStore(Config{Storage: mem, AbsoluteTimeout: time.Hour})
	gobStore.RegisterType(codecProfile{})
	profile := newCodecProfile()
	id := seedSession(t, app, gobStore, func(s *Session) {
		s.Set("profile", profile)
		s.Set("visits", 3)
		s.Set("name", "alice")
	})
	gobRaw, err := mem.Get(sessionKey(id))
	require.NoError(t, err)
	require.False(t, isRecord(gobRaw))
	wantExpiry, ok := loadSession(t, app, gobStore, id).AbsoluteExpiry()
	require.True(t, ok)

	codecStore := NewStore(Config{Storage: mem, Codec: JSONCodec{}, AbsoluteTimeout: time.Hour})
	sess := loadSession(t, app, codecStore, id)
	require.False(t, sess.Fresh())
	require.Equal(t, id, sess.ID())
	require.Nil(t, sess.data.codec, "a session that was stored as gob is gob, whatever the store now does")

	// It reads as it did: the typed readers work in either mode, and Get returns the values themselves.
	var got codecProfile
	require.NoError(t, sess.GetInto("profile", &got))
	require.Equal(t, profile, got)
	visits, ok := GetAs[int](sess, "visits")
	require.True(t, ok)
	require.Equal(t, 3, visits)
	require.Equal(t, 3, sess.Get("visits"))
	require.Equal(t, "alice", sess.Get("name"))
	expiry, ok := sess.AbsoluteExpiry()
	require.True(t, ok)
	require.True(t, wantExpiry.Equal(expiry))

	// Saved after only scalars were read, it is the same bytes.
	scalars := loadSession(t, app, codecStore, id)
	require.Equal(t, 3, scalars.Get("visits"))
	require.NoError(t, scalars.Save())
	stored, err := mem.Get(sessionKey(id))
	require.NoError(t, err)
	require.Equal(t, gobRaw, stored)

	// Saved changed, it is gob still, and a store without a codec reads it.
	sess.Set("visits", 4)
	require.NoError(t, sess.Save())
	stored, err = mem.Get(sessionKey(id))
	require.NoError(t, err)
	require.False(t, isRecord(stored))
	back := loadSession(t, app, gobStore, id)
	require.Equal(t, 4, back.Get("visits"))
	require.Equal(t, "alice", back.Get("name"))

	// A session that starts over is a session of the store: it has a record.
	require.NoError(t, sess.Reset())
	require.NotNil(t, sess.data.codec)
	sess.Set("k", "v")
	require.NoError(t, sess.Save())
	header, entries := storedRecord(t, mem, sess.ID())
	require.True(t, header.deadline.set, "and its deadline")
	require.Len(t, entries, 1)

	// So does one that is new.
	fresh := loadSession(t, app, codecStore, "")
	require.NotNil(t, fresh.data.codec)
}

// badUTF8 is a string that JSON cannot carry: its second byte is not UTF-8.
var badUTF8 = string([]byte{'a', 0xff, 'b'})

type codecHidden struct {
	Shown  string
	Secret string `json:"-"`
}

// A session that was stored as gob is not converted, and nothing is lost to a
// conversion that a codec does not do the way gob does.
func Test_Codec_GobSessionsKeepEverythingTheyHold(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	mem := memory.New()
	gobStore := NewStore(Config{Storage: mem})
	gobStore.RegisterType(codecHidden{})
	codecStore := NewStore(Config{Storage: mem, Codec: JSONCodec{}})

	id := seedSession(t, app, gobStore, func(s *Session) {
		s.Set(7, "a key that is not a string")
		s.Set("bytes", badUTF8)
		s.Set("hidden", codecHidden{Shown: "shown", Secret: "secret"})
		s.Set("complex", complex(1, 2))
	})

	sess := loadSession(t, app, codecStore, id)
	require.Equal(t, "a key that is not a string", sess.Get(7))
	require.NoError(t, sess.Save())
	sess.Set("more", 1)
	require.NoError(t, sess.Save())

	back := loadSession(t, app, gobStore, id)
	require.Equal(t, "a key that is not a string", back.Get(7))
	require.Equal(t, badUTF8, back.Get("bytes"))
	require.Equal(t, codecHidden{Shown: "shown", Secret: "secret"}, back.Get("hidden"))
	require.Equal(t, complex(1, 2), back.Get("complex"))
	require.Equal(t, 1, back.Get("more"))
}

// Not parallel: the log output is process-global, so this test captures it.
func Test_Codec_RecordsAStoreCannotRead(t *testing.T) {
	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	id := seedSession(t, app, store, func(s *Session) { s.Set("k", "v") })
	record, err := mem.Get(sessionKey(id))
	require.NoError(t, err)

	future := slices.Clone(record)
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
			stored, err := mem.Get(sessionKey(tc.id))
			require.NoError(t, err)
			require.NotNil(t, stored)

			// Once is enough to say so.
			again := loadSession(t, app, tc.store, tc.id)
			require.True(t, again.Fresh())
			require.Equal(t, 1, strings.Count(buf.String(), "[Warn]"), "%q", buf.String())
			require.Contains(t, buf.String(), tc.warn)

			_, err = tc.store.GetByID(context.Background(), tc.id)
			require.ErrorIs(t, err, ErrSessionIDNotFoundInStore)
		})
	}

	// The store of the codec reads it, as before.
	sess := loadSession(t, app, store, id)
	require.False(t, sess.Fresh())
	got, ok := GetAs[string](sess, "k")
	require.True(t, ok)
	require.Equal(t, "v", got)
}

func Test_Codec_CorruptRecordsAreErrors(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	id := seedSession(t, app, store, func(s *Session) { s.Set("k", "v") })
	good, err := mem.Get(sessionKey(id))
	require.NoError(t, err)

	// (An empty value is no session at all, for every store.)
	for n := 1; n < len(good); n++ {
		require.NoError(t, mem.Set(sessionKey("cut"), good[:n], time.Hour))
		require.Error(t, loadErr(app, store, "cut"), "record cut to %d bytes", n)
	}
	require.NoError(t, mem.Set(sessionKey("cut"), append(bytes.Clone(good), 1), time.Hour))
	require.Error(t, loadErr(app, store, "cut"))
	require.NoError(t, mem.Set(sessionKey("cut"), []byte("not a gob stream"), time.Hour))
	require.Error(t, loadErr(app, store, "cut"))
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
			_, entries := storedRecord(t, memoryOf(t, rec), sess.ID())
			require.Len(t, entries, tc.entries)
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

		mem := memory.New()
		app := fiber.New()
		app.Use(New(Config{Storage: mem, Codec: JSONCodec{}, SkipEmptySessions: true}))
		app.Get("/", func(c fiber.Ctx) error {
			FromContext(c).Set(7, "x")
			return nil
		})
		resp, _ := sendRequest(t, app, "/", nil)
		require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	})
}

// memoryOf returns the memory storage under a recording one.
func memoryOf(t *testing.T, rec *recordingStorage) *memory.Storage {
	t.Helper()
	mem, ok := rec.Storage.(*memory.Storage)
	require.True(t, ok)
	return mem
}

func Test_Codec_DecodesOnlyWhatIsRead(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	mem := memory.New()
	codec := newCountingCodec()
	store := NewStore(Config{Storage: mem, Codec: codec})
	id := seedSession(t, app, store, func(s *Session) {
		s.Set("profile", newCodecProfile())
		s.Set("visits", 3)
		s.Set("name", "alice")
	})
	require.EqualValues(t, 3, codec.marshals.Load())
	before, err := mem.Get(sessionKey(id))
	require.NoError(t, err)
	_, beforeEntries := storedRecord(t, mem, id)

	sess := loadSession(t, app, store, id)
	require.Zero(t, codec.decodes.Load(), "loading a session decodes none of its values")

	var name string
	require.NoError(t, sess.GetInto("name", &name))
	require.EqualValues(t, 1, codec.decodes.Load(), "reading one value decodes that one")

	require.NoError(t, sess.Save())
	require.EqualValues(t, 3, codec.marshals.Load(), "a save of an unchanged session encodes nothing")
	after, err := mem.Get(sessionKey(id))
	require.NoError(t, err)
	require.Equal(t, before, after)

	sess.Set("visits", 4)
	require.EqualValues(t, 4, codec.marshals.Load(), "setting one value encodes that one")
	require.NoError(t, sess.Save())
	require.EqualValues(t, 4, codec.marshals.Load())

	_, afterEntries := storedRecord(t, mem, id)
	require.Equal(t, beforeEntries[0], afterEntries[0])
	require.Equal(t, beforeEntries[1], afterEntries[1])
	require.Equal(t, `4`, string(afterEntries[2].value))
}

func Test_Codec_KeepsACopyOfWhatTheCodecReturns(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore(Config{Codec: &reusingCodec{}})
	id := seedSession(t, app, store, func(s *Session) {
		s.Set("a", "first")
		s.Set("b", "other")
		s.Set("c", "third")
	})

	sess := loadSession(t, app, store, id)
	for key, want := range map[string]string{"a": "first", "b": "other", "c": "third"} {
		got, ok := GetAs[string](sess, key)
		require.True(t, ok)
		require.Equal(t, want, got, key)
	}
}

func Test_Codec_DeleteResetRegenerateDestroy(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	seed := func(t *testing.T) *Session {
		t.Helper()
		id := seedSession(t, app, store, func(s *Session) {
			s.Set("a", 1)
			s.Set("b", 2)
		})
		return loadSession(t, app, store, id)
	}

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		sess := seed(t)
		sess.Delete("a")
		sess.Delete(7) // not a string: nothing to delete
		require.Nil(t, sess.Get("a"))
		require.Equal(t, []any{"b"}, sess.Keys())
	})

	t.Run("Regenerate keeps the data under a new ID", func(t *testing.T) {
		t.Parallel()
		sess := seed(t)
		old := sess.ID()
		require.NoError(t, sess.Regenerate())
		require.NotEqual(t, old, sess.ID())
		got, ok := GetAs[int](sess, "b")
		require.True(t, ok)
		require.Equal(t, 2, got)
		require.NoError(t, sess.Save())
		_, entries := storedRecord(t, mem, sess.ID())
		require.Len(t, entries, 2)
	})

	t.Run("Reset clears the data", func(t *testing.T) {
		t.Parallel()
		sess := seed(t)
		require.NoError(t, sess.Reset())
		require.Empty(t, sess.Keys())
		require.Nil(t, sess.Get("b"))
	})

	t.Run("Destroy deletes the record", func(t *testing.T) {
		t.Parallel()
		sess := seed(t)
		id := sess.ID()
		require.NoError(t, sess.Destroy())
		raw, err := mem.Get(sessionKey(id))
		require.NoError(t, err)
		require.Nil(t, raw)
		require.Empty(t, sess.Keys())
	})
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
}

func Test_Codec_GetByID(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	id := seedSession(t, app, store, func(s *Session) { s.Set("profile", newCodecProfile()) })

	sess, err := store.GetByID(context.Background(), id)
	require.NoError(t, err)
	defer sess.Release()
	var got codecProfile
	require.NoError(t, sess.GetInto("profile", &got))
	require.Equal(t, newCodecProfile(), got)

	// A pooled session of a store with a codec does not carry it to the next store.
	plain := NewStore(Config{Storage: memory.New()})
	for range 20 {
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		s, err := plain.Get(ctx)
		require.NoError(t, err)
		require.Nil(t, s.data.codec)
		s.Set(7, "any key is fine without a codec")
		s.Release()
		app.ReleaseCtx(ctx)
	}
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

	app := fiber.New()
	store := NewStore(Config{Codec: JSONCodec{}})
	id := seedSession(t, app, store, func(s *Session) { s.Set("shared", 0) })
	sess := loadSession(t, app, store, id)

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

// Not parallel: the log output is process-global, so this test captures it.
func Test_Config_CodecStoreOptionWarning(t *testing.T) {
	withCodec := NewStore(Config{Codec: JSONCodec{}})
	plain := NewStore()

	for _, tc := range []struct {
		name string
		cfg  Config
		warn bool
	}{
		{name: "the codec is the store's own", cfg: Config{Store: withCodec}},
		{name: "the same codec on both", cfg: Config{Store: withCodec, Codec: JSONCodec{}}},
		{name: "a codec asked of the middleware only", cfg: Config{Store: plain, Codec: JSONCodec{}}, warn: true},
		{name: "another codec on the middleware", cfg: Config{Store: withCodec, Codec: NewCodec("other", json.Marshal, json.Unmarshal)}, warn: true},
		{name: "no store, the codec is the middleware's", cfg: Config{Codec: JSONCodec{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buf := loggertest.CaptureContextLog(t, "")
			New(tc.cfg)
			if !tc.warn {
				require.Empty(t, buf.String())
				return
			}
			require.Equal(t, 1, bytes.Count(buf.Bytes(), []byte("[Warn]")), "one warning: %q", buf.String())
			require.Contains(t, buf.String(), "Codec is taken from Config.Store")
		})
	}
}

func Test_Session_GetInto_WithoutCodec(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore()
	store.RegisterType(codecProfile{})
	profile := newCodecProfile()
	id := seedSession(t, app, store, func(s *Session) {
		s.Set("name", "alice")
		s.Set("n", 3)
		s.Set("profile", profile)
		s.Set("nothing", nil)
		s.Set("list", []string{"a"})
	})
	sess := loadSession(t, app, store, id)

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

	var got codecProfile
	require.NoError(t, sess.GetInto("profile", &got))
	require.Equal(t, profile, got)
	require.True(t, sess.data.dirty.Load(), "a struct may share memory with the session, as it does for Get")

	t.Run("a stored nil zeroes the target", func(t *testing.T) {
		t.Parallel()
		set := "set"
		require.NoError(t, sess.GetInto("nothing", &set))
		require.Empty(t, set)
	})

	t.Run("a value that does not fit is an error that leaves the target", func(t *testing.T) {
		t.Parallel()
		wide := int64(9)
		err := sess.GetInto("n", &wide)
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrKeyNotFound)
		require.Equal(t, int64(9), wide)

		var pointer *codecProfile
		require.Error(t, sess.GetInto("profile", &pointer))
		require.Nil(t, pointer)
	})

	t.Run("a missing key", func(t *testing.T) {
		t.Parallel()
		keep := "keep"
		require.ErrorIs(t, sess.GetInto("missing", &keep), ErrKeyNotFound)
		require.Equal(t, "keep", keep)
	})

	t.Run("the target has to be a non-nil pointer", func(t *testing.T) {
		t.Parallel()
		var nilPointer *string
		require.Error(t, sess.GetInto("name", name))
		require.Error(t, sess.GetInto("name", nilPointer))
		require.Error(t, sess.GetInto("name", nil))
		require.Error(t, sess.GetInto("missing", nil), "a bad target is reported before a missing key")
	})

	t.Run("a value that can alias the session gives up the snapshot", func(t *testing.T) {
		t.Parallel()
		own := loadSession(t, app, store, id)
		require.False(t, own.data.dirty.Load())
		var list []string
		require.NoError(t, own.GetInto("list", &list))
		require.Equal(t, []string{"a"}, list)
		require.True(t, own.data.dirty.Load())
	})
}

func Test_GetAs_WithAnyReader(t *testing.T) {
	t.Parallel()

	require.Equal(t, 5, mustGetAs[int](t, readerFunc(func(any) any { return 5 })))
	_, ok := GetAs[int](readerFunc(func(any) any { return "five" }), "k")
	require.False(t, ok)
}

type readerFunc func(key any) any

func (f readerFunc) Get(key any) any { return f(key) }

func mustGetAs[V any](t *testing.T, src interface{ Get(key any) any }) V {
	t.Helper()
	v, ok := GetAs[V](src, "k")
	require.True(t, ok)
	return v
}

func Test_Codec_ErrorsNamePortableCauses(t *testing.T) {
	t.Parallel()

	require.NotErrorIs(t, errNonStringKey, ErrKeyNotFound)
	require.NotErrorIs(t, fmt.Errorf("x: %w", errCodecMismatch), errRecordNeedsCodec)
}

// zeroCopyCodec stores bytes as they are and decodes them without a copy, as a
// codec that keeps a slice of its input does.
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

// A decoded value may keep the bytes it is given, and a program may change it: that
// must not reach the record the session writes back.
func Test_Codec_DecodedValuesDoNotAliasTheSnapshot(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: zeroCopyCodec{}})
	id := seedSession(t, app, store, func(s *Session) {
		s.Set("a", []byte("first"))
		s.Set("b", []byte("second"))
	})
	before, err := mem.Get(sessionKey(id))
	require.NoError(t, err)

	sess := loadSession(t, app, store, id)

	var a []byte
	require.NoError(t, sess.GetInto("a", &a))
	a[0] = 'X'
	a = append(a, "-appended past what was there, into the entry that follows"...)
	_ = a

	viaGetAs, ok := GetAs[[]byte](sess, "a")
	require.True(t, ok)
	require.Equal(t, "first", string(viaGetAs))
	viaGetAs[1] = 'Y'

	generic, isBytes := sess.Get("b").([]byte)
	require.True(t, isBytes, "an undecodable value is returned as it is stored")
	generic[0] = 'Z'

	require.NoError(t, sess.Save())
	after, err := mem.Get(sessionKey(id))
	require.NoError(t, err)
	require.Equal(t, before, after, "a clean save writes the record back as it was read")

	again := loadSession(t, app, store, id)
	first, ok := GetAs[[]byte](again, "a")
	require.True(t, ok)
	require.Equal(t, "first", string(first))
	second, ok := GetAs[[]byte](again, "b")
	require.True(t, ok)
	require.Equal(t, "second", string(second))
}

func Test_Codec_PooledSessionsForgetTheirCodec(t *testing.T) {
	t.Parallel()

	// The data of a session that goes back to the pool lets go of its codec and of the
	// failed Sets. (It is a data of the test: a pooled one is not read after it was given back.)
	own := &data{Data: map[any]any{}}
	own.setCodec(JSONCodec{}, "json")
	own.Set("k", 1)
	own.Set(2, "a key that is not a string")
	require.NotNil(t, own.codec)
	require.NotEmpty(t, own.errs)

	// A reset in the life of a session keeps the codec, and forgets the failed Sets.
	own.Reset()
	require.NotNil(t, own.codec)
	require.Equal(t, "json", own.codecName)
	require.Nil(t, own.errs)

	own.Set(2, "again")
	own.recycle()
	require.Nil(t, own.codec)
	require.Empty(t, own.codecName)
	require.Nil(t, own.errs)
	require.Empty(t, own.Data)

	// And what a store without a codec writes after sessions of one went back to the
	// pool is gob, wherever the pool puts them.
	app := fiber.New()
	withCodec := NewStore(Config{Codec: JSONCodec{}})
	mem := memory.New()
	t.Cleanup(func() { require.NoError(t, mem.Close()) })
	plain := NewStore(Config{Storage: mem})

	for range 50 {
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		sess, err := withCodec.Get(ctx)
		require.NoError(t, err)
		sess.Set("k", 1)
		sess.Set(2, "ignored: the failed Set goes with the session")
		sess.Release()
		app.ReleaseCtx(ctx)

		ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
		other, err := plain.Get(ctx)
		require.NoError(t, err)
		require.Nil(t, other.data.codec)
		other.Set(2, "any key is fine for gob")
		require.NoError(t, other.Save())
		raw, err := mem.Get(sessionKey(other.ID()))
		require.NoError(t, err)
		require.False(t, isRecord(raw), "what a store without a codec writes is gob")
		other.Release()
		app.ReleaseCtx(ctx)
	}
}

// A key that cannot be hashed makes a read panic. The lock the read took must go with it,
// or the deferred Release of the store pattern waits for it for ever.
func Test_Session_ReadsReleaseTheirLocksWhenTheyPanic(t *testing.T) {
	t.Parallel()

	unhashable := []int{1}
	for name, codec := range map[string]Codec{"gob": nil, "codec": JSONCodec{}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			store := NewStore(Config{Codec: codec})
			reads := map[string]func(*Session){
				"Get":     func(s *Session) { _ = s.Get(unhashable) },
				"GetAs":   func(s *Session) { _, _ = GetAs[int](s, unhashable) },
				"GetInto": func(s *Session) { var out int; _ = s.GetInto(unhashable, &out) }, //nolint:errcheck // the call panics
			}
			for read, fn := range reads {
				sess := loadSession(t, app, store, "")
				require.Panics(t, func() { fn(sess) }, read)

				done := make(chan struct{})
				go func() {
					defer close(done)
					sess.Set("k", 1) // takes the write lock
					sess.Delete("k")
				}()
				select {
				case <-done:
				case <-time.After(10 * time.Second):
					t.Fatalf("%s left the lock of the session taken", read)
				}
			}
		})
	}
}

func Test_Middleware_ReadsReleaseTheirLocksWhenTheyPanic(t *testing.T) {
	t.Parallel()

	unhashable := []int{1}
	for name, codec := range map[string]Codec{"gob": nil, "codec": JSONCodec{}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			store := NewStore(Config{Codec: codec})
			reads := map[string]func(*Middleware){
				"Get":     func(m *Middleware) { _ = m.Get(unhashable) },
				"GetAs":   func(m *Middleware) { _, _ = GetAs[int](m, unhashable) },
				"GetInto": func(m *Middleware) { var out int; _ = m.GetInto(unhashable, &out) }, //nolint:errcheck // the call panics
			}
			for read, fn := range reads {
				ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
				m := startMiddleware(t, ctx, store)
				require.Panics(t, func() { fn(m) }, read)

				done := make(chan struct{})
				go func() {
					defer close(done)
					m.Set("k", 1) // takes the write lock of the middleware
				}()
				select {
				case <-done:
				case <-time.After(10 * time.Second):
					t.Fatalf("%s left the lock of the middleware taken", read)
				}
				app.ReleaseCtx(ctx)
			}
		})
	}
}

// A reset puts the session in the state of its store in one step: a Set that races
// with it is encoded or gone, never kept raw in a session that is read as encoded.
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
		require.Nil(t, sess.data.codec, "it is a session that was stored as gob")

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
		require.NotNil(t, sess.data.codec)
		require.NoError(t, sess.Save())
	}
}

// With a codec GetAs and GetInto decode what was stored into the type they are asked
// for; they do not check that this type is the one it was stored as, which gob does.
func Test_Codec_GetAsDecodesItDoesNotCheckTheType(t *testing.T) {
	t.Parallel()

	type user struct{ Name string }
	type admin struct{ Name string }

	app := fiber.New()
	gobStore := NewStore()
	gobStore.RegisterType(user{})
	codecStore := NewStore(Config{Codec: JSONCodec{}})

	for name, store := range map[string]*Store{"gob": gobStore, "codec": codecStore} {
		id := seedSession(t, app, store, func(s *Session) {
			s.Set("user", user{Name: "alice"})
			s.Set("count", 5)
		})
		sess := loadSession(t, app, store, id)

		_, isAdmin := GetAs[admin](sess, "user")
		_, isFloat := GetAs[float64](sess, "count")
		_, isInt64 := GetAs[int64](sess, "count")
		_, isUser := GetAs[user](sess, "user")
		require.True(t, isUser, name)
		if name == "gob" {
			require.False(t, isAdmin, "gob checks the type that was stored")
			require.False(t, isFloat)
			require.False(t, isInt64)
		} else {
			require.True(t, isAdmin, "a codec decodes into whatever fits")
			require.True(t, isFloat)
			require.True(t, isInt64)
		}
	}
}

// A key is quoted in an error, so that a key that came from a client cannot forge a line of a log.
func Test_Session_ErrorsQuoteTheKey(t *testing.T) {
	t.Parallel()

	for _, codec := range []Codec{nil, JSONCodec{}} {
		sess := loadSession(t, fiber.New(), NewStore(Config{Codec: codec}), "")
		var out string
		err := sess.GetInto("a\nb [Error] forged", &out)
		require.ErrorIs(t, err, ErrKeyNotFound)
		require.NotContains(t, err.Error(), "\n")
		require.Contains(t, err.Error(), `"a\nb [Error] forged"`)
	}
}

func Test_Codec_GetInto(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore(Config{Codec: JSONCodec{}})
	id := seedSession(t, app, store, func(s *Session) {
		s.Set("user", codecProfile{ID: 7})
		s.Set("map", map[string]string{"new": "1"})
		s.Set("name", "alice")
		s.Set("nothing", nil)
	})
	sess := loadSession(t, app, store, id)

	t.Run("it replaces what the target holds", func(t *testing.T) {
		t.Parallel()
		got := codecProfile{ID: 99, Name: "guest", Roles: []string{"stale"}}
		require.NoError(t, sess.GetInto("user", &got))
		require.Equal(t, codecProfile{ID: 7}, got.withoutTime())

		stale := map[string]string{"old": "stale"}
		require.NoError(t, sess.GetInto("map", &stale))
		require.Equal(t, map[string]string{"new": "1"}, stale)
	})

	t.Run("a value that does not fit leaves the target as it was", func(t *testing.T) {
		t.Parallel()
		count := 5
		require.Error(t, sess.GetInto("name", &count))
		require.Equal(t, 5, count)
	})

	t.Run("a stored nil zeroes the target", func(t *testing.T) {
		t.Parallel()
		name := "set"
		require.NoError(t, sess.GetInto("nothing", &name))
		require.Empty(t, name)
	})

	t.Run("into an interface", func(t *testing.T) {
		t.Parallel()
		var anything any
		require.NoError(t, sess.GetInto("name", &anything))
		require.Equal(t, "alice", anything)
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
	})
}

// withoutTime returns the profile without its time, which a test does not compare.
func (p *codecProfile) withoutTime() codecProfile {
	c := *p
	c.At = time.Time{}
	return c
}

func Test_Codec_UndecodableValues(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	raw, err := appendRecord(nil, "json", recordDeadline{}, []recordEntry{
		{key: "broken", value: []byte(`{not json`)},
		{key: "fine", value: []byte(`"fine"`)},
	})
	require.NoError(t, err)
	require.NoError(t, mem.Set(sessionKey("stored"), raw, time.Hour))
	sess := loadSession(t, app, store, "stored")

	// It is there: Get never says that a value that exists does not.
	require.Equal(t, []byte(`{not json`), sess.Get("broken"))
	_, ok := GetAs[string](sess, "broken")
	require.False(t, ok)
	var out string
	require.Error(t, sess.GetInto("broken", &out))
	require.NotErrorIs(t, sess.GetInto("broken", &out), ErrKeyNotFound)
	require.ElementsMatch(t, []any{"broken", "fine"}, sess.Keys())
	require.Equal(t, "fine", sess.Get("fine"))
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
	app.Use(New(Config{Codec: codec}))
	app.Get("/set", func(c fiber.Ctx) error {
		FromContext(c).Set("name", "alice")
		return nil
	})
	app.Get("/get", func(c fiber.Ctx) error {
		m := FromContext(c)
		codec.onUnmarshal = func() { m.Set("touched", true) }
		var name string
		if err := m.GetInto("name", &name); err != nil {
			return err
		}
		if got, ok := GetAs[string](m, "name"); !ok || got != name {
			return errors.New("GetAs disagrees with GetInto")
		}
		_ = m.Get("name")
		codec.onUnmarshal = nil
		return c.SendString(name)
	})

	resp, _ := sendRequest(t, app, "/set", nil)
	cookies := map[string]string{"session_id": cookiesByName(resp)["session_id"].Value}

	done := make(chan string, 1)
	go func() {
		_, body, err := doRequest(app, "/get", cookies)
		if err != nil {
			done <- err.Error()
			return
		}
		done <- body
	}()
	select {
	case body := <-done:
		require.Equal(t, "alice", body)
	case <-time.After(10 * time.Second):
		t.Fatal("GetInto, GetAs or Get holds the lock of the session while the codec runs")
	}
}
