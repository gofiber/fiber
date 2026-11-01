package session

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// benchSession is the session of a signed-in user: five values, one of them a struct.
func benchSession(s *Session) {
	s.Set("user_id", 42)
	s.Set("name", "alice")
	s.Set("admin", true)
	s.Set("csrf", "0123456789abcdef0123456789abcdef")
	s.Set("profile", newCodecProfile())
}

// Benchmark_Codec_Request compares what a request pays for its session: a read of
// one value, a read of every value, and a change of one value, with a store that
// keeps gob and one with a codec.
//
// go test -run '^$' -bench Benchmark_Codec_Request -benchmem
func Benchmark_Codec_Request(b *testing.B) {
	app := fiber.New()
	gob := NewStore(Config{Storage: memory.New()})
	gob.RegisterType(codecProfile{})

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
		{store: gob, name: "gob"},
		{store: NewStore(Config{Storage: memory.New(), Codec: JSONCodec{}}), name: "json"},
	} {
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		seeded, err := tc.store.Get(ctx)
		require.NoError(b, err)
		benchSession(seeded)
		require.NoError(b, seeded.Save(), tc.name)
		id := seeded.ID()
		seeded.Release()
		app.ReleaseCtx(ctx)

		for _, request := range requests {
			b.Run(request.name+"/"+tc.name, func(b *testing.B) {
				// Every benchmark has a request of its own: a context caches the session ID of
				// the last store that read it.
				ctx := newRequest(b, app)
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

// The benchmarks below keep what they read, so that a copy that is not needed
// is not left out.
var (
	sinkInt    int
	sinkBig    benchBig
	sinkBigPtr *benchBig
	sinkOK     bool
	errSink    error
)

// Benchmark_Codec_ReadForms compares the ways to read a value of a store without a
// codec, where the value is already decoded: the type assertion, GetAs and GetInto.
// The assertion copies a value once, GetAs hands it out as a result and the caller
// copies it again, and GetInto copies it into the variable the caller provides.
//
// go test -run '^$' -bench Benchmark_Codec_ReadForms -benchmem
func Benchmark_Codec_ReadForms(b *testing.B) {
	sess := mustGet(b, NewStore(), newRequest(b, fiber.New()))
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

// Benchmark_Codec_GetIntoLarge reads a value of about 8 KB from a store with a codec
// into a variable that holds its zero value, which is decoded into directly, into
// one that holds something, which is replaced by a new value, and with GetAs, which
// returns the value for the caller to copy.
//
// go test -run '^$' -bench Benchmark_Codec_GetIntoLarge -benchmem
func Benchmark_Codec_GetIntoLarge(b *testing.B) {
	sess := mustGet(b, NewStore(Config{Codec: JSONCodec{}}), newRequest(b, fiber.New()))

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
