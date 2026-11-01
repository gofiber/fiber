package session

import (
	"bytes"
	"context"
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
func sessionKey(id string) string { return keyNamespace + id }

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

func Test_Codec_ReadsDoNotAliasTheSession(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore(Config{Codec: JSONCodec{}})
	id := seedSession(t, app, store, func(s *Session) { s.Set("list", []string{"a", "b"}) })
	sess := loadSession(t, app, store, id)
	require.NotNil(t, sess.rawData, "a record that was read is written back as it is, from the snapshot")
	require.True(t, isRecord(sess.rawData))
	require.False(t, sess.data.dirty.Load())

	first, ok := GetAs[[]string](sess, "list")
	require.True(t, ok)
	first[0] = "changed"

	second, ok := GetAs[[]string](sess, "list")
	require.True(t, ok)
	require.Equal(t, []string{"a", "b"}, second, "a value read does not alias the session")

	// And a read that cannot change the session leaves it unchanged, so a save writes the snapshot back.
	_ = sess.Get("list")
	var viaInto []string
	require.NoError(t, sess.GetInto("list", &viaInto))
	require.False(t, sess.data.dirty.Load(), "a read under a codec never changes the session")
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

// targetCodec is a JSONCodec that remembers what its last Unmarshal was given, and
// can be made to fail or to panic after it has decoded.
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

	// setup returns a session that holds a profile, and the codec its store uses.
	setup := func(t *testing.T, fail error) (*targetCodec, *Session) {
		t.Helper()
		app := fiber.New()
		codec := &targetCodec{fail: fail}
		store := NewStore(Config{Codec: codec})
		id := seedSession(t, app, store, func(s *Session) { s.Set("user", codecProfile{ID: 7, Name: "alice"}) })
		return codec, loadSession(t, app, store, id)
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
		got := codecProfile{ID: 99, Roles: []string{"stale"}}
		require.NoError(t, sess.GetInto("user", &got))
		require.NotSame(t, &got, codec.target(), "a decoder merges into what it is given, so it is given a new value")
		require.Equal(t, codecProfile{ID: 7, Name: "alice"}, got.withoutTime())
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

// Get, like GetAs and GetInto, hands the codec a copy: what the codec does to it does not reach
// the session.
func Test_Codec_GetHandsTheCodecACopy(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	mem := memory.New()
	store := NewStore(Config{Storage: mem, Codec: scribblingCodec{}})
	id := seedSession(t, app, store, func(s *Session) { s.Set("a", "hello") })
	before, err := mem.Get(sessionKey(id))
	require.NoError(t, err)

	sess := loadSession(t, app, store, id)
	require.Equal(t, "hello", sess.Get("a"))
	require.Equal(t, "hello", sess.Get("a"), "the first read did not damage what is stored")
	var s string
	require.NoError(t, sess.GetInto("a", &s))
	require.Equal(t, "hello", s)
	require.NoError(t, sess.Save())

	after, err := mem.Get(sessionKey(id))
	require.NoError(t, err)
	require.Equal(t, before, after)
}

// A Set that fails on a session that was loaded, and not changed otherwise, still fails the save:
// the snapshot is not written back over it.
func Test_Codec_FailedSetOnALoadedSessionFailsTheSave(t *testing.T) {
	t.Parallel()

	for name, set := range map[string]func(*Session){
		"a value the codec refuses":  func(s *Session) { s.Set("ch", make(chan int)) },
		"a key that is not a string": func(s *Session) { s.Set(7, "x") },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			rec := newRecordingStorage()
			store := NewStore(Config{Storage: rec, Codec: JSONCodec{}})
			id := seedSession(t, app, store, func(s *Session) { s.Set("a", 1) })
			rec.reset()

			sess := loadSession(t, app, store, id)
			set(sess)
			require.Error(t, sess.Save())
			require.Empty(t, rec.writes(), "nothing is written")
		})
	}
}

// Destroy starts a session over as a session of the store, as Reset does, so that what is saved
// after it is a record.
func Test_Codec_DestroyStartsAGobSessionOverAsARecord(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	mem := memory.New()
	plain := NewStore(Config{Storage: mem})
	id := seedSession(t, app, plain, func(s *Session) { s.Set("a", 1) })

	store := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	sess := loadSession(t, app, store, id)
	require.Nil(t, sess.data.codec, "a session that was stored as gob stays gob")

	require.NoError(t, sess.Destroy())
	sess.Set("b", 2)
	require.NoError(t, sess.Save())

	raw, err := mem.Get(sessionKey(sess.ID()))
	require.NoError(t, err)
	require.True(t, isRecord(raw), "after Destroy the session is one of the store: %q", raw)
}

// The deadline keeps its nanoseconds through the record, when it is decoded and when it is encoded.
func Test_Codec_AbsoluteDeadlineKeepsItsNanoseconds(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore(Config{Storage: memory.New(), Codec: JSONCodec{}, AbsoluteTimeout: time.Hour})
	deadline := time.Now().Add(30 * time.Minute).Truncate(time.Second).Add(123_456_789 * time.Nanosecond)

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	sess, err := store.Get(ctx)
	require.NoError(t, err)
	sess.setAbsExpiration(deadline)
	sess.Set("a", 1) // the record is encoded: the deadline goes through data.record
	require.NoError(t, sess.Save())
	id := sess.ID()
	sess.Release()

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
}

// nameCountingCodec counts the questions about its name.
type nameCountingCodec struct {
	Codec
	names atomic.Int64
}

func (c *nameCountingCodec) Name() string {
	c.names.Add(1)
	return c.Codec.Name()
}

// The name of the codec is asked once, when the store is made.
func Test_Codec_NameIsAskedOnce(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	codec := &nameCountingCodec{Codec: JSONCodec{}}
	store := NewStore(Config{Storage: memory.New(), Codec: codec})
	require.EqualValues(t, 1, codec.names.Load())

	id := seedSession(t, app, store, func(s *Session) { s.Set("a", 1) })
	for range 5 {
		sess := loadSession(t, app, store, id)
		sess.Set("a", 2)
		require.NoError(t, sess.Save())
		_, err := store.GetByID(context.Background(), id)
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, codec.names.Load(), "the name is kept, not asked again")
}

// A decode that fails returns the zero value, not what the codec had decoded before it failed.
func Test_Codec_GetAsAfterAFailedDecodeIsZero(t *testing.T) {
	t.Parallel()

	type pair struct {
		Name string `json:"name"`
		ID   int    `json:"id"`
	}

	app := fiber.New()
	store := NewStore(Config{Storage: memory.New(), Codec: JSONCodec{}})
	id := seedSession(t, app, store, func(s *Session) {
		s.Set("p", map[string]any{"id": 7, "name": 5}) // the id fits, the name does not
	})
	sess := loadSession(t, app, store, id)

	value, ok := GetAs[pair](sess, "p")
	require.False(t, ok)
	require.Equal(t, pair{}, value, "nothing of a failed decode is returned")

	target := pair{Name: "keep", ID: 1}
	require.Error(t, sess.GetInto("p", &target))
	require.Equal(t, pair{Name: "keep", ID: 1}, target, "nor stored in the target")
}

// Reads wait for the lock of the middleware, which a Reset, Destroy or Regenerate holds while it
// calls the storage.
func Test_Middleware_ReadsTakeTheReadLock(t *testing.T) {
	t.Parallel()

	for name, codec := range map[string]Codec{"gob": nil, "json": JSONCodec{}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Codec: codec}))
			app.Get("/", func(c fiber.Ctx) error {
				m := FromContext(c)
				m.Set("k", "v")

				reads := map[string]func(){
					"Get":     func() { _ = m.Get("k") },
					"GetAs":   func() { _, _ = GetAs[string](m, "k") },
					"GetInto": func() { var s string; _ = m.GetInto("k", &s) }, //nolint:errcheck // only the locking matters here
					"lookup":  func() { _ = m.lookup("k") },
					"fetch":   func() { _, _ = m.fetch("k") }, //nolint:errcheck // only the locking matters here
				}
				for readName, read := range reads {
					m.mu.Lock()
					done := make(chan struct{})
					go func() {
						read()
						close(done)
					}()
					select {
					case <-done:
						m.mu.Unlock()
						t.Errorf("%s did not wait for the lock of the middleware", readName)
						return nil
					case <-time.After(50 * time.Millisecond):
					}
					m.mu.Unlock()
					select {
					case <-done:
					case <-time.After(5 * time.Second):
						t.Errorf("%s did not finish after the lock was given back", readName)
						return nil
					}
				}
				return nil
			})
			require.Equal(t, fiber.StatusOK, serveGet(app, "/"))
		})
	}
}

// GetByID says so, once per store, when it meets a record its store cannot read.
func Test_Codec_GetByIDWarnsAboutARecordItCannotRead(t *testing.T) { //nolint:paralleltest // the log is captured for the whole process
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
