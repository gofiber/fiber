package session

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Fuzz_Session_CodecStoredBytes loads whatever bytes a storage holds under a
// session ID, as a store with a codec and as one without. Neither may panic, a
// session that loads must be readable and savable, an unchanged one must be
// written back as it was, and a change must keep the other values as they were.
func Fuzz_Session_CodecStoredBytes(f *testing.F) {
	mem := memory.New()
	f.Cleanup(func() { require.NoError(f, mem.Close()) })
	app := fiber.New()
	codecStore := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
	plainStore := NewStore(Config{Storage: mem})

	valid, err := appendRecord(nil, "json", recordDeadline{sec: 1_900_000_000, nanos: 5, set: true},
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

	load := func(store *Store) (*Session, fiber.Ctx, error) {
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		ctx.Request().Header.SetCookie("session_id", "stored")
		sess, err := store.Get(ctx)
		return sess, ctx, err
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 4096 {
			return
		}
		require.NoError(t, mem.Set(sessionKey("stored"), raw, time.Hour))

		// A record is not for a store without a codec: it is no session of its.
		if isRecord(raw) {
			plain, ctx, err := load(plainStore)
			require.NoError(t, err)
			require.True(t, plain.Fresh())
			require.NotEqual(t, "stored", plain.ID())
			plain.Release()
			app.ReleaseCtx(ctx)
		}

		sess, ctx, err := load(codecStore)
		defer app.ReleaseCtx(ctx)
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

		// Saving what was only read gives the bytes back, whatever they were. (A session
		// that is new, or whose deadline has passed, has another ID, and keeps nothing.)
		id := sess.ID()
		require.NoError(t, sess.Save())
		if id == "stored" {
			stored, err := mem.Get(sessionKey(id))
			require.NoError(t, err)
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
	raw, err := mem.Get(sessionKey(id))
	require.NoError(t, err)
	if !isRecord(raw) {
		return nil, false
	}
	_, entries, err := collectRecord(raw)
	require.NoError(t, err)
	return entries, true
}
