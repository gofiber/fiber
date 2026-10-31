package session

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/loggertest"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func init() {
	gob.Register(map[string]int{})
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

// Without a codec, GetAs gives up the saved snapshot for a value that could share memory with the
// session, as Get does: a change made through it is saved.
func Test_GetAs_GivesUpTheSnapshotLikeGet(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore(Config{Storage: memory.New()})
	id := seedSession(t, app, store, func(s *Session) { s.Set("m", map[string]int{"a": 1}) })

	sess := loadSession(t, app, store, id)
	m, ok := GetAs[map[string]int](sess, "m")
	require.True(t, ok)
	m["b"] = 2 // a change in place, without Set
	require.NoError(t, sess.Save())

	again := loadSession(t, app, store, id)
	got, ok := GetAs[map[string]int](again, "m")
	require.True(t, ok)
	require.Equal(t, map[string]int{"a": 1, "b": 2}, got)

	// The same holds for GetInto, and for a scalar it does not: nothing is saved again.
	var viaInto map[string]int
	require.NoError(t, again.GetInto("m", &viaInto))
	viaInto["c"] = 3
	require.NoError(t, again.Save())
	third := loadSession(t, app, store, id)
	got, ok = GetAs[map[string]int](third, "m")
	require.True(t, ok)
	require.Equal(t, map[string]int{"a": 1, "b": 2, "c": 3}, got)
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

// A session without its data map answers like an empty one.
func Test_Session_WithoutDataAnswersLikeAnEmptyOne(t *testing.T) {
	t.Parallel()

	var sess Session
	require.NotPanics(t, func() {
		expiry, ok := sess.AbsoluteExpiry()
		require.False(t, ok)
		require.True(t, expiry.IsZero())

		value, found := GetAs[string](&sess, "k")
		require.False(t, found)
		require.Empty(t, value)

		var target string
		require.ErrorIs(t, sess.GetInto("k", &target), ErrKeyNotFound)
	})
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

// The target of Middleware.GetInto is checked as that of Session.GetInto is.
func Test_Middleware_GetInto_TargetHasToBeANonNilPointer(t *testing.T) {
	t.Parallel()

	for name, codec := range map[string]Codec{"gob": nil, "json": JSONCodec{}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Codec: codec}))
			app.Get("/", func(c fiber.Ctx) error {
				m := FromContext(c)
				m.Set("k", 1)

				var nilPointer *int
				require.NotPanics(t, func() {
					require.Error(t, m.GetInto("k", nil))
					require.Error(t, m.GetInto("k", 5))
					require.Error(t, m.GetInto("k", nilPointer))
					require.Error(t, m.GetInto("missing", nil), "the target is checked before the key is looked up")
				})
				var n int
				require.NoError(t, m.GetInto("k", &n))
				require.Equal(t, 1, n)
				return nil
			})
			require.Equal(t, fiber.StatusOK, serveGet(app, "/"))
		})
	}
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
					"lookup":  func() { _, _ = m.lookup("k") },
					"fetch":   func() { _, _, _ = m.fetch("k") }, //nolint:errcheck // only the locking matters here
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

// What loadRecord cannot read it refuses, whatever the store did before it: a session without a
// codec, and a record of another codec. A failed decode leaves no entries behind.
func Test_Session_DecodeRecordRefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()

	record := mustAppendRecord(t, "json", recordDeadline{}, []recordEntry{{key: "a", value: []byte("1")}})

	t.Run("a session without a codec", func(t *testing.T) {
		t.Parallel()
		sess := acquireSession()
		defer sess.Release()
		sess.data.Lock()
		defer sess.data.Unlock()
		require.ErrorIs(t, sess.decodeSessionData(record), errRecordNeedsCodec)
	})

	t.Run("a record of another codec", func(t *testing.T) {
		t.Parallel()
		sess := acquireSession()
		defer sess.Release()
		sess.data.codec = &namedCodec{Codec: JSONCodec{}, name: "other"}
		sess.data.Lock()
		defer sess.data.Unlock()
		err := sess.decodeSessionData(record)
		require.ErrorIs(t, err, errCodecMismatch)
		require.Empty(t, sess.data.Data, "nothing of the record is left")
	})

	t.Run("a damaged record leaves no entries", func(t *testing.T) {
		t.Parallel()
		sess := acquireSession()
		defer sess.Release()
		sess.data.codec = &namedCodec{Codec: JSONCodec{}, name: "json"}
		sess.data.Lock()
		defer sess.data.Unlock()
		damaged := append(bytes.Clone(record), 0) // a byte too many, found after the entries were added
		require.ErrorIs(t, sess.decodeSessionData(damaged), errInvalidRecord)
		require.Empty(t, sess.data.Data)
	})
}

// A data object that goes back to the pool forgets its codec and the errors of its Sets.
func Test_Data_PoolForgetsTheCodec(t *testing.T) {
	t.Parallel()

	d := acquireData()
	d.codec = &namedCodec{Codec: JSONCodec{}, name: "json"}
	d.Set(7, "not a string key")
	require.NotEmpty(t, d.errs)
	releaseData(d)

	// Whatever the pool hands out is clean; it is usually the object that was just put back.
	fresh := acquireData()
	defer releaseData(fresh)
	require.Nil(t, fresh.codec)
	require.Nil(t, fresh.errs)
}

// A record is built into one buffer of the size it needs. AllocsPerRun cannot run in a parallel
// test, so this one is sequential.
func Test_Record_AppendAllocatesOnce(t *testing.T) { //nolint:paralleltest // testing.AllocsPerRun panics in a parallel test
	entries := make([]recordEntry, 0, 100)
	for i := range 100 {
		entries = append(entries, recordEntry{key: fmt.Sprintf("key-%03d", i), value: bytes.Repeat([]byte("v"), 32)})
	}
	var sink []byte
	allocs := testing.AllocsPerRun(50, func() {
		var err error
		sink, err = appendRecord(nil, "json", recordDeadline{sec: 1_700_000_000, nanos: 5, set: true}, entries)
		if err != nil {
			panic(err)
		}
	})
	require.NotEmpty(t, sink)
	require.InDelta(t, 1, allocs, 0, "one buffer, sized in advance")
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

// serveGet runs a GET request through the app and returns the status code.
func serveGet(app *fiber.App, uri string) int {
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI(uri)
	app.Handler()(fctx)
	return fctx.Response.StatusCode()
}
