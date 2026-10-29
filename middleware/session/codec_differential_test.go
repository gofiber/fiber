package session

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// A session behaves the same whatever it is stored with. The same random requests,
// each setting, deleting, resetting, regenerating or destroying, are played against
// a store that keeps gob, one with a JSON codec and one with a codec made by
// NewCodec, and every one of them is checked against a model that is a plain Go
// map: whatever was set last is what a later request reads back, typed.
//
// Each key has one type, so that the same GetAs reads it in all modes.

type modelKey struct {
	generate func(r *rand.Rand) any
	read     func(s *Session, key string) (any, bool)
	name     string
}

func modelString(r *rand.Rand) string {
	pieces := []string{"a", "b", "é", "\"", "\\", "/", "x y", "日本", "0"}
	parts := make([]string, 1+r.IntN(4))
	for i := range parts {
		parts[i] = pieces[r.IntN(len(pieces))]
	}
	return strings.Join(parts, "")
}

func readAs[V any](s *Session, key string) (any, bool) {
	v, ok := GetAs[V](s, key)
	return v, ok
}

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

	modes := map[string]func() Codec{
		"gob":    func() Codec { return nil },
		"json":   func() Codec { return JSONCodec{} },
		"custom": func() Codec { return NewCodec("custom", json.Marshal, json.Unmarshal) },
	}
	for mode, codec := range modes {
		for _, absolute := range []time.Duration{0, time.Hour} {
			t.Run(mode+"/absolute="+absolute.String(), func(t *testing.T) {
				t.Parallel()
				for seed := range invariantSeeds() * 25 {
					playModel(t, seed, codec(), absolute)
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

	check := func(s *Session, when string) {
		t.Helper()
		keys := make([]string, 0)
		for _, k := range s.Keys() {
			if name, ok := k.(string); ok {
				keys = append(keys, name)
			}
		}
		slices.Sort(keys)
		want := make([]string, 0, len(model))
		for name := range model {
			want = append(want, name)
		}
		slices.Sort(want)
		require.Equal(t, want, keys, "seed %d, %s: keys", seed, when)

		for i := range modelKeys {
			mk := modelKeys[i]
			got, ok := mk.read(s, mk.name)
			expected, present := model[mk.name]
			require.Equal(t, present, ok, "seed %d, %s: key %s present", seed, when, mk.name)
			if present {
				require.Equal(t, expected, got, "seed %d, %s: key %s", seed, when, mk.name)
			}
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
				got, ok := mk.read(sess, mk.name)
				expected, present := model[mk.name]
				require.Equal(t, present, ok)
				if present {
					require.Equal(t, expected, got)
				}
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
