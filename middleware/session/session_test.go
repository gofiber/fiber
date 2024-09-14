package session

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Session
func Test_Session(t *testing.T) {
	t.Parallel()

	// session store
	store := newStore()

	// fiber instance
	app := fiber.New()

	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Get a new session
	sess, err := store.Get(ctx)
	require.NoError(t, err)
	require.True(t, sess.Fresh())
	token := sess.ID()
	require.NoError(t, sess.Save())

	sess.Release()
	app.ReleaseCtx(ctx)
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})

	// set session
	ctx.Request().Header.SetCookie(store.sessionName, token)

	// get session
	sess, err = store.Get(ctx)
	require.NoError(t, err)
	require.False(t, sess.Fresh())

	// get keys
	keys := sess.Keys()
	require.Equal(t, []string{}, keys)

	// get value
	name := sess.Get("name")
	require.Nil(t, name)

	// set value
	sess.Set("name", "john")

	// get value
	name = sess.Get("name")
	require.Equal(t, "john", name)

	keys = sess.Keys()
	require.Equal(t, []string{"name"}, keys)

	// delete key
	sess.Delete("name")

	// get value
	name = sess.Get("name")
	require.Nil(t, name)

	// get keys
	keys = sess.Keys()
	require.Equal(t, []string{}, keys)

	// get id
	id := sess.ID()
	require.Equal(t, token, id)

	// save the old session first
	err = sess.Save()
	require.NoError(t, err)

	// release the session
	sess.Release()
	// release the context
	app.ReleaseCtx(ctx)

	// requesting entirely new context to prevent falsy tests
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})

	sess, err = store.Get(ctx)
	require.NoError(t, err)
	require.True(t, sess.Fresh())

	// this id should be randomly generated as session key was deleted
	require.Len(t, sess.ID(), 36)

	sess.Release()

	// when we use the original session for the second time
	// the session be should be same if the session is not expired
	app.ReleaseCtx(ctx)
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// request the server with the old session
	ctx.Request().Header.SetCookie(store.sessionName, id)
	sess, err = store.Get(ctx)
	defer sess.Release()
	require.NoError(t, err)
	require.False(t, sess.Fresh())
	require.Equal(t, sess.id, id)
}

// go test -run Test_Session_Types
func Test_Session_Types(t *testing.T) {
	t.Parallel()

	// session store
	store := newStore()

	// fiber instance
	app := fiber.New()

	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	// set cookie
	ctx.Request().Header.SetCookie(store.sessionName, "123")

	// get session
	sess, err := store.Get(ctx)
	require.NoError(t, err)
	require.True(t, sess.Fresh())

	// the session string is no longer be 123
	newSessionIDString := sess.ID()

	type User struct {
		Name string
	}
	store.RegisterType(User{})
	vuser := User{
		Name: "John",
	}
	// set value
	var (
		vbool                  = true
		vstring                = "str"
		vint                   = 13
		vint8       int8       = 13
		vint16      int16      = 13
		vint32      int32      = 13
		vint64      int64      = 13
		vuint       uint       = 13
		vuint8      uint8      = 13
		vuint16     uint16     = 13
		vuint32     uint32     = 13
		vuint64     uint64     = 13
		vuintptr    uintptr    = 13
		vbyte       byte       = 'k'
		vrune                  = 'k'
		vfloat32    float32    = 13
		vfloat64    float64    = 13
		vcomplex64  complex64  = 13
		vcomplex128 complex128 = 13
	)
	sess.Set("vuser", vuser)
	sess.Set("vbool", vbool)
	sess.Set("vstring", vstring)
	sess.Set("vint", vint)
	sess.Set("vint8", vint8)
	sess.Set("vint16", vint16)
	sess.Set("vint32", vint32)
	sess.Set("vint64", vint64)
	sess.Set("vuint", vuint)
	sess.Set("vuint8", vuint8)
	sess.Set("vuint16", vuint16)
	sess.Set("vuint32", vuint32)
	sess.Set("vuint32", vuint32)
	sess.Set("vuint64", vuint64)
	sess.Set("vuintptr", vuintptr)
	sess.Set("vbyte", vbyte)
	sess.Set("vrune", vrune)
	sess.Set("vfloat32", vfloat32)
	sess.Set("vfloat64", vfloat64)
	sess.Set("vcomplex64", vcomplex64)
	sess.Set("vcomplex128", vcomplex128)

	// save session
	err = sess.Save()
	require.NoError(t, err)

	sess.Release()
	app.ReleaseCtx(ctx)
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})

	ctx.Request().Header.SetCookie(store.sessionName, newSessionIDString)

	// get session
	sess, err = store.Get(ctx)
	require.NoError(t, err)
	require.False(t, sess.Fresh())

	// get value
	vuserResult, ok := sess.Get("vuser").(User)
	require.True(t, ok)
	require.Equal(t, vuser, vuserResult)

	vboolResult, ok := sess.Get("vbool").(bool)
	require.True(t, ok)
	require.Equal(t, vbool, vboolResult)

	vstringResult, ok := sess.Get("vstring").(string)
	require.True(t, ok)
	require.Equal(t, vstring, vstringResult)

	vintResult, ok := sess.Get("vint").(int)
	require.True(t, ok)
	require.Equal(t, vint, vintResult)

	vint8Result, ok := sess.Get("vint8").(int8)
	require.True(t, ok)
	require.Equal(t, vint8, vint8Result)

	vint16Result, ok := sess.Get("vint16").(int16)
	require.True(t, ok)
	require.Equal(t, vint16, vint16Result)

	vint32Result, ok := sess.Get("vint32").(int32)
	require.True(t, ok)
	require.Equal(t, vint32, vint32Result)

	vint64Result, ok := sess.Get("vint64").(int64)
	require.True(t, ok)
	require.Equal(t, vint64, vint64Result)

	vuintResult, ok := sess.Get("vuint").(uint)
	require.True(t, ok)
	require.Equal(t, vuint, vuintResult)

	vuint8Result, ok := sess.Get("vuint8").(uint8)
	require.True(t, ok)
	require.Equal(t, vuint8, vuint8Result)

	vuint16Result, ok := sess.Get("vuint16").(uint16)
	require.True(t, ok)
	require.Equal(t, vuint16, vuint16Result)

	vuint32Result, ok := sess.Get("vuint32").(uint32)
	require.True(t, ok)
	require.Equal(t, vuint32, vuint32Result)

	vuint64Result, ok := sess.Get("vuint64").(uint64)
	require.True(t, ok)
	require.Equal(t, vuint64, vuint64Result)

	vuintptrResult, ok := sess.Get("vuintptr").(uintptr)
	require.True(t, ok)
	require.Equal(t, vuintptr, vuintptrResult)

	vbyteResult, ok := sess.Get("vbyte").(byte)
	require.True(t, ok)
	require.Equal(t, vbyte, vbyteResult)

	vruneResult, ok := sess.Get("vrune").(rune)
	require.True(t, ok)
	require.Equal(t, vrune, vruneResult)

	vfloat32Result, ok := sess.Get("vfloat32").(float32)
	require.True(t, ok)
	require.InEpsilon(t, vfloat32, vfloat32Result, 0.001)

	vfloat64Result, ok := sess.Get("vfloat64").(float64)
	require.True(t, ok)
	require.InEpsilon(t, vfloat64, vfloat64Result, 0.001)

	vcomplex64Result, ok := sess.Get("vcomplex64").(complex64)
	require.True(t, ok)
	require.Equal(t, vcomplex64, vcomplex64Result)

	vcomplex128Result, ok := sess.Get("vcomplex128").(complex128)
	require.True(t, ok)
	require.Equal(t, vcomplex128, vcomplex128Result)

	sess.Release()

	app.ReleaseCtx(ctx)
}

// go test -run Test_Session_Store_Reset
func Test_Session_Store_Reset(t *testing.T) {
	t.Parallel()
	// session store
	store := newStore()
	// fiber instance
	app := fiber.New()
	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	// get session
	sess, err := store.Get(ctx)
	require.NoError(t, err)
	// make sure its new
	require.True(t, sess.Fresh())
	// set value & save
	sess.Set("hello", "world")
	ctx.Request().Header.SetCookie(store.sessionName, sess.ID())
	require.NoError(t, sess.Save())

	// reset store
	require.NoError(t, store.Reset())
	id := sess.ID()

	sess.Release()
	app.ReleaseCtx(ctx)
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.SetCookie(store.sessionName, id)

	// make sure the session is recreated
	sess, err = store.Get(ctx)
	defer sess.Release()
	require.NoError(t, err)
	require.True(t, sess.Fresh())
	require.Nil(t, sess.Get("hello"))
}

// go test -run Test_Session_Save
func Test_Session_Save(t *testing.T) {
	t.Parallel()

	t.Run("save to cookie", func(t *testing.T) {
		t.Parallel()
		// session store
		store := newStore()
		// fiber instance
		app := fiber.New()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

		// get session
		sess, err := store.Get(ctx)
		require.NoError(t, err)
		// set value
		sess.Set("name", "john")

		// save session
		err = sess.Save()
		require.NoError(t, err)
		sess.Release()
	})

	t.Run("save to header", func(t *testing.T) {
		t.Parallel()
		// session store
		store := newStore(Config{
			KeyLookup: "header:session_id",
		})
		// fiber instance
		app := fiber.New()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// get session
		sess, err := store.Get(ctx)
		require.NoError(t, err)
		// set value
		sess.Set("name", "john")

		// save session
		err = sess.Save()
		require.NoError(t, err)
		require.Equal(t, store.getSessionID(ctx), string(ctx.Response().Header.Peek(store.sessionName)))
		require.Equal(t, store.getSessionID(ctx), string(ctx.Request().Header.Peek(store.sessionName)))
		sess.Release()
	})
}

func Test_Session_Save_Expiration(t *testing.T) {
	t.Parallel()

	t.Run("save to cookie", func(t *testing.T) {
		t.Parallel()

		const sessionDuration = 5 * time.Second
		// session store
		store := newStore()
		// fiber instance
		app := fiber.New()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// get session
		sess, err := store.Get(ctx)
		require.NoError(t, err)

		// set value
		sess.Set("name", "john")

		token := sess.ID()

		// expire this session in 5 seconds
		sess.SetIdleTimeout(sessionDuration)

		// save session
		err = sess.Save()
		require.NoError(t, err)

		sess.Release()
		app.ReleaseCtx(ctx)
		ctx = app.AcquireCtx(&fasthttp.RequestCtx{})

		// here you need to get the old session yet
		ctx.Request().Header.SetCookie(store.sessionName, token)
		sess, err = store.Get(ctx)
		require.NoError(t, err)
		require.Equal(t, "john", sess.Get("name"))

		// just to make sure the session has been expired
		time.Sleep(sessionDuration + (10 * time.Millisecond))

		sess.Release()

		app.ReleaseCtx(ctx)
		ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// here you should get a new session
		ctx.Request().Header.SetCookie(store.sessionName, token)
		sess, err = store.Get(ctx)
		defer sess.Release()
		require.NoError(t, err)
		require.Nil(t, sess.Get("name"))
		require.NotEqual(t, sess.ID(), token)
	})
}

// go test -run Test_Session_Destroy
func Test_Session_Destroy(t *testing.T) {
	t.Parallel()

	t.Run("destroy from cookie", func(t *testing.T) {
		t.Parallel()
		// session store
		store := newStore()
		// fiber instance
		app := fiber.New()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// get session
		sess, err := store.Get(ctx)
		defer sess.Release()
		require.NoError(t, err)

		sess.Set("name", "fenny")
		require.NoError(t, sess.Destroy())
		name := sess.Get("name")
		require.Nil(t, name)
	})

	t.Run("destroy from header", func(t *testing.T) {
		t.Parallel()
		// session store
		store := newStore(Config{
			KeyLookup: "header:session_id",
		})
		// fiber instance
		app := fiber.New()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// get session
		sess, err := store.Get(ctx)
		require.NoError(t, err)

		// set value & save
		sess.Set("name", "fenny")
		id := sess.ID()
		require.NoError(t, sess.Save())

		sess.Release()
		app.ReleaseCtx(ctx)
		ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// get session
		ctx.Request().Header.Set(store.sessionName, id)
		sess, err = store.Get(ctx)
		require.NoError(t, err)
		defer sess.Release()

		err = sess.Destroy()
		require.NoError(t, err)
		require.Equal(t, "", string(ctx.Response().Header.Peek(store.sessionName)))
		require.Equal(t, "", string(ctx.Request().Header.Peek(store.sessionName)))
	})
}

// go test -run Test_Session_Custom_Config
func Test_Session_Custom_Config(t *testing.T) {
	t.Parallel()

	store := newStore(Config{IdleTimeout: time.Hour, KeyGenerator: func() string { return "very random" }})
	require.Equal(t, time.Hour, store.IdleTimeout)
	require.Equal(t, "very random", store.KeyGenerator())

	store = newStore(Config{IdleTimeout: 0})
	require.Equal(t, ConfigDefault.IdleTimeout, store.IdleTimeout)
}

// go test -run Test_Session_Cookie
func Test_Session_Cookie(t *testing.T) {
	t.Parallel()
	// session store
	store := newStore()
	// fiber instance
	app := fiber.New()
	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// get session
	sess, err := store.Get(ctx)
	require.NoError(t, err)
	require.NoError(t, sess.Save())

	sess.Release()

	// cookie should be set on Save ( even if empty data )
	require.Len(t, ctx.Response().Header.PeekCookie(store.sessionName), 84)
}

// go test -run Test_Session_Cookie_In_Response
// Regression: https://github.com/gofiber/fiber/pull/1191
func Test_Session_Cookie_In_Middleware_Chain(t *testing.T) {
	t.Parallel()
	store := newStore()
	app := fiber.New()

	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// get session
	sess, err := store.Get(ctx)
	require.NoError(t, err)
	sess.Set("id", "1")
	require.True(t, sess.Fresh())
	id := sess.ID()
	require.NoError(t, sess.Save())

	sess.Release()

	sess, err = store.Get(ctx)
	require.NoError(t, err)
	defer sess.Release()
	sess.Set("name", "john")
	require.True(t, sess.Fresh())
	require.Equal(t, id, sess.ID()) // session id should be the same

	require.Equal(t, "1", sess.Get("id"))
	require.Equal(t, "john", sess.Get("name"))
}

// go test -run Test_Session_Deletes_Single_Key
// Regression: https://github.com/gofiber/fiber/issues/1365
func Test_Session_Deletes_Single_Key(t *testing.T) {
	t.Parallel()
	store := newStore()
	app := fiber.New()

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	sess, err := store.Get(ctx)
	require.NoError(t, err)
	id := sess.ID()
	sess.Set("id", "1")
	require.NoError(t, sess.Save())

	sess.Release()
	app.ReleaseCtx(ctx)
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	ctx.Request().Header.SetCookie(store.sessionName, id)

	sess, err = store.Get(ctx)
	require.NoError(t, err)
	sess.Delete("id")
	require.NoError(t, sess.Save())

	sess.Release()
	app.ReleaseCtx(ctx)
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	ctx.Request().Header.SetCookie(store.sessionName, id)

	sess, err = store.Get(ctx)
	defer sess.Release()
	require.NoError(t, err)
	require.False(t, sess.Fresh())
	require.Nil(t, sess.Get("id"))

	app.ReleaseCtx(ctx)
}

// go test -run Test_Session_Reset
func Test_Session_Reset(t *testing.T) {
	t.Parallel()
	// fiber instance
	app := fiber.New()

	// session store
	store := newStore()

	t.Run("reset session data and id, and set fresh to be true", func(t *testing.T) {
		t.Parallel()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		// a random session uuid
		originalSessionUUIDString := ""

		// now the session is in the storage
		freshSession, err := store.Get(ctx)
		require.NoError(t, err)

		originalSessionUUIDString = freshSession.ID()

		// set a value
		freshSession.Set("name", "fenny")
		freshSession.Set("email", "fenny@example.com")

		err = freshSession.Save()
		require.NoError(t, err)

		freshSession.Release()
		app.ReleaseCtx(ctx)
		ctx = app.AcquireCtx(&fasthttp.RequestCtx{})

		// set cookie
		ctx.Request().Header.SetCookie(store.sessionName, originalSessionUUIDString)

		// as the session is in the storage, session.fresh should be false
		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)
		require.False(t, acquiredSession.Fresh())

		err = acquiredSession.Reset()
		require.NoError(t, err)

		require.NotEqual(t, originalSessionUUIDString, acquiredSession.ID())

		// acquiredSession.fresh should be true after resetting
		require.True(t, acquiredSession.Fresh())

		// Check that the session data has been reset
		keys := acquiredSession.Keys()
		require.Equal(t, []string{}, keys)

		// Set a new value for 'name' and check that it's updated
		acquiredSession.Set("name", "john")
		require.Equal(t, "john", acquiredSession.Get("name"))
		require.Nil(t, acquiredSession.Get("email"))

		// Save after resetting
		err = acquiredSession.Save()
		require.NoError(t, err)

		acquiredSession.Release()

		// Check that the session id is not in the header or cookie anymore
		require.Equal(t, "", string(ctx.Response().Header.Peek(store.sessionName)))
		require.Equal(t, "", string(ctx.Request().Header.Peek(store.sessionName)))

		app.ReleaseCtx(ctx)
	})
}

// go test -run Test_Session_Regenerate
// Regression: https://github.com/gofiber/fiber/issues/1395
func Test_Session_Regenerate(t *testing.T) {
	t.Parallel()
	// fiber instance
	app := fiber.New()
	t.Run("set fresh to be true when regenerating a session", func(t *testing.T) {
		t.Parallel()
		// session store
		store := newStore()
		// a random session uuid
		originalSessionUUIDString := ""
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// now the session is in the storage
		freshSession, err := store.Get(ctx)
		require.NoError(t, err)

		originalSessionUUIDString = freshSession.ID()

		err = freshSession.Save()
		require.NoError(t, err)

		freshSession.Release()

		// release the context
		app.ReleaseCtx(ctx)

		// acquire a new context
		ctx = app.AcquireCtx(&fasthttp.RequestCtx{})

		// set cookie
		ctx.Request().Header.SetCookie(store.sessionName, originalSessionUUIDString)

		// as the session is in the storage, session.fresh should be false
		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)
		defer acquiredSession.Release()
		require.False(t, acquiredSession.Fresh())

		err = acquiredSession.Regenerate()
		require.NoError(t, err)

		require.NotEqual(t, originalSessionUUIDString, acquiredSession.ID())

		// acquiredSession.fresh should be true after regenerating
		require.True(t, acquiredSession.Fresh())

		// release the context
		app.ReleaseCtx(ctx)
	})
}

// go test -v -run=^$ -bench=Benchmark_Session -benchmem -count=4
func Benchmark_Session(b *testing.B) {
	b.Run("default", func(b *testing.B) {
		app, store := fiber.New(), newStore()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.Request().Header.SetCookie(store.sessionName, "12356789")

		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			sess, _ := store.Get(c) //nolint:errcheck // We're inside a benchmark
			sess.Set("john", "doe")
			_ = sess.Save() //nolint:errcheck // We're inside a benchmark

			sess.Release()
		}
	})

	b.Run("storage", func(b *testing.B) {
		app := fiber.New()
		store := newStore(Config{
			Storage: memory.New(),
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.Request().Header.SetCookie(store.sessionName, "12356789")

		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			sess, _ := store.Get(c) //nolint:errcheck // We're inside a benchmark
			sess.Set("john", "doe")
			_ = sess.Save() //nolint:errcheck // We're inside a benchmark

			sess.Release()
		}
	})
}

// go test -v -run=^$ -bench=Benchmark_Session_Parallel -benchmem -count=4
func Benchmark_Session_Parallel(b *testing.B) {
	b.Run("default", func(b *testing.B) {
		app, store := fiber.New(), newStore()
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				c := app.AcquireCtx(&fasthttp.RequestCtx{})
				c.Request().Header.SetCookie(store.sessionName, "12356789")

				sess, _ := store.Get(c) //nolint:errcheck // We're inside a benchmark
				sess.Set("john", "doe")
				_ = sess.Save() //nolint:errcheck // We're inside a benchmark

				sess.Release()

				app.ReleaseCtx(c)
			}
		})
	})

	b.Run("storage", func(b *testing.B) {
		app := fiber.New()
		store := newStore(Config{
			Storage: memory.New(),
		})
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				c := app.AcquireCtx(&fasthttp.RequestCtx{})
				c.Request().Header.SetCookie(store.sessionName, "12356789")

				sess, _ := store.Get(c) //nolint:errcheck // We're inside a benchmark
				sess.Set("john", "doe")
				_ = sess.Save() //nolint:errcheck // We're inside a benchmark

				sess.Release()

				app.ReleaseCtx(c)
			}
		})
	})
}

// go test -v -run=^$ -bench=Benchmark_Session_Asserted -benchmem -count=4
func Benchmark_Session_Asserted(b *testing.B) {
	b.Run("default", func(b *testing.B) {
		app, store := fiber.New(), newStore()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.Request().Header.SetCookie(store.sessionName, "12356789")

		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			sess, err := store.Get(c)
			require.NoError(b, err)
			sess.Set("john", "doe")
			err = sess.Save()
			require.NoError(b, err)
			sess.Release()
		}
	})

	b.Run("storage", func(b *testing.B) {
		app := fiber.New()
		store := newStore(Config{
			Storage: memory.New(),
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.Request().Header.SetCookie(store.sessionName, "12356789")

		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			sess, err := store.Get(c)
			require.NoError(b, err)
			sess.Set("john", "doe")
			err = sess.Save()
			require.NoError(b, err)
			sess.Release()
		}
	})
}

// go test -v -run=^$ -bench=Benchmark_Session_Asserted_Parallel -benchmem -count=4
func Benchmark_Session_Asserted_Parallel(b *testing.B) {
	b.Run("default", func(b *testing.B) {
		app, store := fiber.New(), newStore()
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				c := app.AcquireCtx(&fasthttp.RequestCtx{})
				c.Request().Header.SetCookie(store.sessionName, "12356789")

				sess, err := store.Get(c)
				require.NoError(b, err)
				sess.Set("john", "doe")
				require.NoError(b, sess.Save())
				sess.Release()
				app.ReleaseCtx(c)
			}
		})
	})

	b.Run("storage", func(b *testing.B) {
		app := fiber.New()
		store := newStore(Config{
			Storage: memory.New(),
		})
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				c := app.AcquireCtx(&fasthttp.RequestCtx{})
				c.Request().Header.SetCookie(store.sessionName, "12356789")

				sess, err := store.Get(c)
				require.NoError(b, err)
				sess.Set("john", "doe")
				require.NoError(b, sess.Save())
				sess.Release()
				app.ReleaseCtx(c)
			}
		})
	})
}

// go test -v -race -run Test_Session_Concurrency ./...
func Test_Session_Concurrency(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	store := newStore()

	var wg sync.WaitGroup
	errChan := make(chan error, 10) // Buffered channel to collect errors
	const numGoroutines = 10        // Number of concurrent goroutines to test

	// Start numGoroutines goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			localCtx := app.AcquireCtx(&fasthttp.RequestCtx{})

			sess, err := store.getSession(localCtx)
			if err != nil {
				errChan <- err
				return
			}

			// Set a value
			sess.Set("name", "john")

			// get the session id
			id := sess.ID()

			// Check if the session is fresh
			if !sess.Fresh() {
				errChan <- errors.New("session should be fresh")
				return
			}

			// Save the session
			if err := sess.Save(); err != nil {
				errChan <- err
				return
			}

			// release the session
			sess.Release()

			// Release the context
			app.ReleaseCtx(localCtx)

			// Acquire a new context
			localCtx = app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(localCtx)

			// Set the session id in the header
			localCtx.Request().Header.SetCookie(store.sessionName, id)

			// Get the session
			sess, err = store.Get(localCtx)
			if err != nil {
				errChan <- err
				return
			}
			defer sess.Release()

			// Get the value
			name := sess.Get("name")
			if name != "john" {
				errChan <- errors.New("name should be john")
				return
			}

			// Get ID from the session
			if sess.ID() != id {
				errChan <- errors.New("id should be the same")
				return
			}

			// Check if the session is fresh
			if sess.Fresh() {
				errChan <- errors.New("session should not be fresh")
				return
			}

			// Delete the key
			sess.Delete("name")

			// Get the value
			name = sess.Get("name")
			if name != nil {
				errChan <- errors.New("name should be nil")
				return
			}

			// Destroy the session
			if err := sess.Destroy(); err != nil {
				errChan <- err
				return
			}
		}()
	}

	wg.Wait()      // Wait for all goroutines to finish
	close(errChan) // Close the channel to signal no more errors will be sent

	// Check for errors sent to errChan
	for err := range errChan {
		require.NoError(t, err)
	}
}
