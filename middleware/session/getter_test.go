package session

import (
	"encoding/gob"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type getterProfile struct{ Name string }

type getterUserID int

// requireGetAs checks that src holds a V under key, and that it is want.
func requireGetAs[V any, S *Session | *Middleware](tb testing.TB, src S, key string, want V) {
	tb.Helper()
	got, ok := GetAs[V](src, key)
	require.True(tb, ok, key)
	require.Equal(tb, want, got, key)
}

// requireNoGetAs checks that src holds no V under key, and that the zero V comes back with it.
func requireNoGetAs[V any, S *Session | *Middleware](tb testing.TB, src S, key string, msgAndArgs ...any) {
	tb.Helper()
	got, ok := GetAs[V](src, key)
	require.False(tb, ok, msgAndArgs...)
	require.Zero(tb, got, key)
}

func Test_GetAs(t *testing.T) {
	t.Parallel()

	sess := newSessionWithStorage(t, memory.New())
	sess.Set("name", "alice")
	sess.Set("count", 3)
	sess.Set("zero", 0)
	sess.Set("id", getterUserID(7))
	sess.Set("profile", getterProfile{Name: "a"})
	sess.Set("pointer", &getterProfile{Name: "p"})
	sess.Set("nil pointer", (*getterProfile)(nil))
	sess.Set("list", []string{"x", "y"})
	sess.Set("nothing", nil)

	t.Run("a value of the asked type is found, a zero value too", func(t *testing.T) {
		t.Parallel()
		requireGetAs(t, sess, "name", "alice")
		requireGetAs(t, sess, "count", 3)
		requireGetAs(t, sess, "id", getterUserID(7))
		requireGetAs(t, sess, "profile", getterProfile{Name: "a"})
		requireGetAs(t, sess, "list", []string{"x", "y"})
		requireGetAs(t, sess, "zero", 0) // 0 is a value; absent is what ok is for
		requireGetAs(t, sess, "nil pointer", (*getterProfile)(nil))
		requireGetAs[any](t, sess, "name", "alice") // an interface type finds what the key holds
	})

	t.Run("another type, a missing key and a stored nil are not found", func(t *testing.T) {
		t.Parallel()
		requireNoGetAs[int](t, sess, "name")
		requireNoGetAs[int](t, sess, "id", "a named type is not its underlying type")
		requireNoGetAs[getterProfile](t, sess, "pointer", "a pointer is not the struct it points to")
		requireNoGetAs[string](t, sess, "missing")
		requireNoGetAs[any](t, sess, "missing")
		requireNoGetAs[any](t, sess, "nothing", "a stored nil reads as absent, as a nil does for any type assertion")
	})

	t.Run("a nil source finds nothing", func(t *testing.T) {
		t.Parallel()
		requireNoGetAs[string](t, (*Session)(nil), "name", "nil session")
		requireNoGetAs[string](t, (*Middleware)(nil), "name", "nil middleware")
	})
}

func Test_GetAs_Middleware(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())
	app.Get("/set", func(c fiber.Ctx) error {
		FromContext(c).Set("n", 42)
		return nil
	})
	app.Get("/get", func(c fiber.Ctx) error {
		n, ok := GetAs[int](FromContext(c), "n")
		s, sok := GetAs[string](FromContext(c), "n")
		return c.SendString(strconv.Itoa(n) + strconv.FormatBool(ok) + "|" + s + strconv.FormatBool(sok))
	})

	resp, _ := sendRequest(t, app, "/set", nil)
	id := cookiesByName(resp)["session_id"].Value
	_, body := sendRequest(t, app, "/get", map[string]string{"session_id": id})
	require.Equal(t, "42true|false", body)

	t.Run("without the middleware, FromContext is nil and GetAs says so", func(t *testing.T) {
		t.Parallel()
		bare := fiber.New()
		bare.Get("/", func(c fiber.Ctx) error {
			n, ok := GetAs[int](FromContext(c), "n")
			return c.SendString(strconv.Itoa(n) + strconv.FormatBool(ok))
		})
		_, body := sendRequest(t, bare, "/", nil)
		require.Equal(t, "0false", body)
	})
}

// testing.AllocsPerRun panics in a parallel test, so this one stays sequential.
func Test_GetAs_Allocations(t *testing.T) {
	sess := newSessionWithStorage(t, memory.New())
	sess.Set("name", "alice")
	sess.Set("count", 3)

	require.Zero(t, testing.AllocsPerRun(100, func() { _, _ = GetAs[string](sess, "name") }))
	require.Zero(t, testing.AllocsPerRun(100, func() { _, _ = GetAs[int](sess, "count") }))
	require.Zero(t, testing.AllocsPerRun(100, func() { _, _ = GetAs[string](sess, "missing") }))
	require.Zero(t, testing.AllocsPerRun(100, func() { _, _ = GetAs[string]((*Session)(nil), "name") }))

	// A value that is costly to copy is no reason to allocate: the variable that a codec would
	// decode into is made only when there is one.
	type big struct {
		Name  string
		Bytes [1024]byte
	}
	sess.Set("big", big{Name: "big"})
	sess.Set("pointer", &big{Name: "pointer"})
	require.Zero(t, testing.AllocsPerRun(100, func() { _, _ = GetAs[big](sess, "big") }))
	require.Zero(t, testing.AllocsPerRun(100, func() { _, _ = GetAs[*big](sess, "pointer") }))
	require.Zero(t, testing.AllocsPerRun(100, func() { _, _ = GetAs[big](sess, "missing") }))
}

func Test_Session_IdleTimeout(t *testing.T) {
	t.Parallel()

	t.Run("the store's until one is set", func(t *testing.T) {
		t.Parallel()
		sess := newSessionWithStorage(t, memory.New())
		require.Equal(t, 30*time.Minute, sess.IdleTimeout())

		sess.SetIdleTimeout(5 * time.Minute)
		require.Equal(t, 5*time.Minute, sess.IdleTimeout())
	})

	t.Run("a store with another idle timeout", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		store := NewStore(Config{IdleTimeout: 2 * time.Hour})
		id := seedSession(t, app, store, func(s *Session) { s.Set("k", "v") })
		require.Equal(t, 2*time.Hour, loadSession(t, app, store, id).IdleTimeout())
	})

	t.Run("a timeout that was set is not kept for the next request", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		store := NewStore(Config{IdleTimeout: time.Hour})
		id := seedSession(t, app, store, func(s *Session) {
			s.Set("k", "v")
			s.SetIdleTimeout(24 * time.Hour)
		})
		require.Equal(t, time.Hour, loadSession(t, app, store, id).IdleTimeout())
	})
}

func Test_Session_AbsoluteExpiry(t *testing.T) {
	t.Parallel()

	t.Run("none without an absolute timeout", func(t *testing.T) {
		t.Parallel()
		sess := newSessionWithStorage(t, memory.New())
		expiry, ok := sess.AbsoluteExpiry()
		require.False(t, ok)
		require.True(t, expiry.IsZero())
	})

	t.Run("a new session ends one absolute timeout from now; storage and regeneration keep that, reset starts it over", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		store := NewStore(Config{AbsoluteTimeout: time.Hour})
		id := seedSession(t, app, store, func(s *Session) { s.Set("k", "v") })

		sess := loadSession(t, app, store, id)
		original, ok := sess.AbsoluteExpiry()
		require.True(t, ok)
		require.WithinDuration(t, time.Now().Add(time.Hour), original, 10*time.Second)

		require.NoError(t, sess.Regenerate())
		kept, ok := sess.AbsoluteExpiry()
		require.True(t, ok)
		require.True(t, original.Equal(kept), "a new ID does not buy a longer session")

		time.Sleep(10 * time.Millisecond)
		require.NoError(t, sess.Reset())
		restarted, ok := sess.AbsoluteExpiry()
		require.True(t, ok)
		require.True(t, restarted.After(original), "a reset is a new session with a new deadline")
	})
}

// The middleware answers for its session: the idle timeout, which a request can change, and the
// absolute expiry.
func Test_Middleware_IdleTimeoutAndAbsoluteExpiry(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{IdleTimeout: 10 * time.Minute, AbsoluteTimeout: time.Hour}))
	app.Get("/", func(c fiber.Ctx) error {
		m := FromContext(c)
		require.Equal(t, 10*time.Minute, m.IdleTimeout())
		m.Session.SetIdleTimeout(time.Minute)
		require.Equal(t, time.Minute, m.IdleTimeout())

		expiry, ok := m.AbsoluteExpiry()
		require.True(t, ok)
		require.Greater(t, time.Until(expiry), 59*time.Minute)
		return nil
	})
	require.Equal(t, fiber.StatusOK, serveGet(app, "/"))
}

func init() { gob.Register(map[string]int{}) }

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
	requireGetAs(t, again, "m", map[string]int{"a": 1, "b": 2})

	// The same holds for GetInto, and for a scalar it does not: nothing is saved again.
	var viaInto map[string]int
	require.NoError(t, again.GetInto("m", &viaInto))
	viaInto["c"] = 3
	require.NoError(t, again.Save())
	requireGetAs(t, loadSession(t, app, store, id), "m", map[string]int{"a": 1, "b": 2, "c": 3})
}

// A session that is a zero value, with no data map and no store, answers like an empty one.
func Test_Session_WithoutDataAnswersLikeAnEmptyOne(t *testing.T) {
	t.Parallel()

	var sess Session
	require.NotPanics(t, func() {
		expiry, ok := sess.AbsoluteExpiry()
		require.False(t, ok)
		require.True(t, expiry.IsZero())
		require.Zero(t, sess.IdleTimeout())

		requireNoGetAs[string](t, &sess, "k")

		var target string
		require.ErrorIs(t, sess.GetInto("k", &target), ErrKeyNotFound)
	})
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

// serveGet runs a GET request through the app and returns the status code.
func serveGet(app *fiber.App, uri string) int {
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI(uri)
	app.Handler()(fctx)
	return fctx.Response.StatusCode()
}
