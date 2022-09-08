package session

import (
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
	store := New()

	// fiber instance
	app := fiber.New()

	// fiber context
	ctx := app.NewCtx(&fasthttp.RequestCtx{})

	// set session
	ctx.Request().Header.SetCookie(store.sessionName, "123")

	// get session
	sess, err := store.Get(ctx)
	require.NoError(t, err)
	require.True(t, sess.Fresh())

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
	require.Equal(t, "123", id)

	// save the old session first
	err = sess.Save()
	require.NoError(t, err)

	// requesting entirely new context to prevent falsy tests
	ctx = app.NewCtx(&fasthttp.RequestCtx{})

	sess, err = store.Get(ctx)
	require.NoError(t, err)
	require.True(t, sess.Fresh())

	// this id should be randomly generated as session key was deleted
	require.Equal(t, 36, len(sess.ID()))

	// when we use the original session for the second time
	// the session be should be same if the session is not expired
	ctx = app.NewCtx(&fasthttp.RequestCtx{})

	// request the server with the old session
	ctx.Request().Header.SetCookie(store.sessionName, id)
	sess, err = store.Get(ctx)
	require.NoError(t, err)
	require.False(t, sess.Fresh())
	require.Equal(t, sess.id, id)
}

// go test -run Test_Session_Types
func Test_Session_Types(t *testing.T) {
	t.Parallel()

	// session store
	store := New()

	// fiber instance
	app := fiber.New()

	// fiber context
	ctx := app.NewCtx(&fasthttp.RequestCtx{})

	// set cookie
	ctx.Request().Header.SetCookie(store.sessionName, "123")

	// get session
	sess, err := store.Get(ctx)
	require.NoError(t, err)
	require.True(t, sess.Fresh())

	// the session string is no longer be 123
	newSessionIDString := sess.ID()
	ctx.Request().Header.SetCookie(store.sessionName, newSessionIDString)

	type User struct {
		Name string
	}
	store.RegisterType(User{})
	vuser := User{
		Name: "John",
	}
	// set value
	var vbool = true
	var vstring = "str"
	var vint = 13
	var vint8 int8 = 13
	var vint16 int16 = 13
	var vint32 int32 = 13
	var vint64 int64 = 13
	var vuint uint = 13
	var vuint8 uint8 = 13
	var vuint16 uint16 = 13
	var vuint32 uint32 = 13
	var vuint64 uint64 = 13
	var vuintptr uintptr = 13
	var vbyte byte = 'k'
	var vrune rune = 'k'
	var vfloat32 float32 = 13
	var vfloat64 float64 = 13
	var vcomplex64 complex64 = 13
	var vcomplex128 complex128 = 13
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

	// get session
	sess, err = store.Get(ctx)
	require.NoError(t, err)
	require.False(t, sess.Fresh())

	// get value
	require.Equal(t, vuser, sess.Get("vuser").(User))
	require.Equal(t, vbool, sess.Get("vbool").(bool))
	require.Equal(t, vstring, sess.Get("vstring").(string))
	require.Equal(t, vint, sess.Get("vint").(int))
	require.Equal(t, vint8, sess.Get("vint8").(int8))
	require.Equal(t, vint16, sess.Get("vint16").(int16))
	require.Equal(t, vint32, sess.Get("vint32").(int32))
	require.Equal(t, vint64, sess.Get("vint64").(int64))
	require.Equal(t, vuint, sess.Get("vuint").(uint))
	require.Equal(t, vuint8, sess.Get("vuint8").(uint8))
	require.Equal(t, vuint16, sess.Get("vuint16").(uint16))
	require.Equal(t, vuint32, sess.Get("vuint32").(uint32))
	require.Equal(t, vuint64, sess.Get("vuint64").(uint64))
	require.Equal(t, vuintptr, sess.Get("vuintptr").(uintptr))
	require.Equal(t, vbyte, sess.Get("vbyte").(byte))
	require.Equal(t, vrune, sess.Get("vrune").(rune))
	require.Equal(t, vfloat32, sess.Get("vfloat32").(float32))
	require.Equal(t, vfloat64, sess.Get("vfloat64").(float64))
	require.Equal(t, vcomplex64, sess.Get("vcomplex64").(complex64))
	require.Equal(t, vcomplex128, sess.Get("vcomplex128").(complex128))
}

// go test -run Test_Session_Store_Reset
func Test_Session_Store_Reset(t *testing.T) {
	t.Parallel()
	// session store
	store := New()
	// fiber instance
	app := fiber.New()
	// fiber context
	ctx := app.NewCtx(&fasthttp.RequestCtx{})

	// get session
	sess, _ := store.Get(ctx)
	// make sure its new
	require.True(t, sess.Fresh())
	// set value & save
	sess.Set("hello", "world")
	ctx.Request().Header.SetCookie(store.sessionName, sess.ID())
	sess.Save()

	// reset store
	store.Reset()

	// make sure the session is recreated
	sess, _ = store.Get(ctx)
	require.True(t, sess.Fresh())
	require.Nil(t, sess.Get("hello"))
}

// go test -run Test_Session_Save
func Test_Session_Save(t *testing.T) {
	t.Parallel()

	t.Run("save to cookie", func(t *testing.T) {
		// session store
		store := New()
		// fiber instance
		app := fiber.New()
		// fiber context
		ctx := app.NewCtx(&fasthttp.RequestCtx{})

		// get session
		sess, _ := store.Get(ctx)
		// set value
		sess.Set("name", "john")

		// save session
		err := sess.Save()
		require.NoError(t, err)
	})

	t.Run("save to header", func(t *testing.T) {
		// session store
		store := New(Config{
			KeyLookup: "header:session_id",
		})
		// fiber instance
		app := fiber.New()
		// fiber context
		ctx := app.NewCtx(&fasthttp.RequestCtx{})

		// get session
		sess, _ := store.Get(ctx)
		// set value
		sess.Set("name", "john")

		// save session
		err := sess.Save()
		require.NoError(t, err)
		require.Equal(t, store.getSessionID(ctx), string(ctx.Response().Header.Peek(store.sessionName)))
		require.Equal(t, store.getSessionID(ctx), string(ctx.Request().Header.Peek(store.sessionName)))
	})
}

func Test_Session_Save_Expiration(t *testing.T) {
	t.Parallel()

	t.Run("save to cookie", func(t *testing.T) {
		// session store
		store := New()
		// fiber instance
		app := fiber.New()
		// fiber context
		ctx := app.NewCtx(&fasthttp.RequestCtx{})

		// get session
		sess, _ := store.Get(ctx)
		// set value
		sess.Set("name", "john")

		// expire this session in 5 seconds
		sess.SetExpiry(time.Second * 5)

		// save session
		err := sess.Save()
		require.NoError(t, err)

		// here you need to get the old session yet
		sess, _ = store.Get(ctx)
		require.Equal(t, "john", sess.Get("name"))

		// just to make sure the session has been expired
		time.Sleep(time.Second * 5)

		// here you should get a new session
		sess, _ = store.Get(ctx)
		require.Nil(t, sess.Get("name"))
	})
}

// go test -run Test_Session_Reset
func Test_Session_Reset(t *testing.T) {
	t.Parallel()

	t.Run("reset from cookie", func(t *testing.T) {
		// session store
		store := New()
		// fiber instance
		app := fiber.New()
		// fiber context
		ctx := app.NewCtx(&fasthttp.RequestCtx{})

		// get session
		sess, _ := store.Get(ctx)

		sess.Set("name", "fenny")
		sess.Destroy()
		name := sess.Get("name")
		require.Nil(t, name)
	})

	t.Run("reset from header", func(t *testing.T) {
		// session store
		store := New(Config{
			KeyLookup: "header:session_id",
		})
		// fiber instance
		app := fiber.New()
		// fiber context
		ctx := app.NewCtx(&fasthttp.RequestCtx{})

		// get session
		sess, _ := store.Get(ctx)

		// set value & save
		sess.Set("name", "fenny")
		_ = sess.Save()
		sess, _ = store.Get(ctx)

		err := sess.Destroy()
		require.NoError(t, err)
		require.Equal(t, "", string(ctx.Response().Header.Peek(store.sessionName)))
		require.Equal(t, "", string(ctx.Request().Header.Peek(store.sessionName)))
	})
}

// go test -run Test_Session_Custom_Config
func Test_Session_Custom_Config(t *testing.T) {
	t.Parallel()

	store := New(Config{Expiration: time.Hour, KeyGenerator: func() string { return "very random" }})
	require.Equal(t, time.Hour, store.Expiration)
	require.Equal(t, "very random", store.KeyGenerator())

	store = New(Config{Expiration: 0})
	require.Equal(t, ConfigDefault.Expiration, store.Expiration)
}

// go test -run Test_Session_Cookie
func Test_Session_Cookie(t *testing.T) {
	t.Parallel()
	// session store
	store := New()
	// fiber instance
	app := fiber.New()
	// fiber context
	ctx := app.NewCtx(&fasthttp.RequestCtx{})

	// get session
	sess, _ := store.Get(ctx)
	sess.Save()

	// cookie should be set on Save ( even if empty data )
	require.Equal(t, 84, len(ctx.Response().Header.PeekCookie(store.sessionName)))
}

// go test -run Test_Session_Cookie_In_Response
func Test_Session_Cookie_In_Response(t *testing.T) {
	t.Parallel()
	store := New()
	app := fiber.New()

	// fiber context
	ctx := app.NewCtx(&fasthttp.RequestCtx{})

	// get session
	sess, _ := store.Get(ctx)
	sess.Set("id", "1")
	require.True(t, sess.Fresh())
	sess.Save()

	sess, _ = store.Get(ctx)
	sess.Set("name", "john")
	require.True(t, sess.Fresh())

	require.Equal(t, "1", sess.Get("id"))
	require.Equal(t, "john", sess.Get("name"))
}

// go test -run Test_Session_Deletes_Single_Key
// Regression: https://github.com/gofiber/fiber/issues/1365
func Test_Session_Deletes_Single_Key(t *testing.T) {
	t.Parallel()
	store := New()
	app := fiber.New()

	ctx := app.NewCtx(&fasthttp.RequestCtx{})

	sess, err := store.Get(ctx)
	require.NoError(t, err)
	ctx.Request().Header.SetCookie(store.sessionName, sess.ID())

	sess.Set("id", "1")
	require.Nil(t, sess.Save())

	sess, err = store.Get(ctx)
	require.NoError(t, err)
	sess.Delete("id")
	require.Nil(t, sess.Save())

	sess, err = store.Get(ctx)
	require.NoError(t, err)
	require.False(t, sess.Fresh())
	require.Nil(t, sess.Get("id"))
}

// go test -run Test_Session_Regenerate
// Regression: https://github.com/gofiber/fiber/issues/1395
func Test_Session_Regenerate(t *testing.T) {
	// fiber instance
	app := fiber.New()
	t.Run("set fresh to be true when regenerating a session", func(t *testing.T) {
		// session store
		store := New()
		// a random session uuid
		originalSessionUUIDString := ""
		// fiber context
		ctx := app.NewCtx(&fasthttp.RequestCtx{})

		// now the session is in the storage
		freshSession, err := store.Get(ctx)
		require.NoError(t, err)

		originalSessionUUIDString = freshSession.ID()

		err = freshSession.Save()
		require.NoError(t, err)

		// set cookie
		ctx.Request().Header.SetCookie(store.sessionName, originalSessionUUIDString)

		// as the session is in the storage, session.fresh should be false
		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)
		require.False(t, acquiredSession.Fresh())

		err = acquiredSession.Regenerate()
		require.NoError(t, err)

		if acquiredSession.ID() == originalSessionUUIDString {
			t.Fatal("regenerate should generate another different id")
		}
		// acquiredSession.fresh should be true after regenerating
		require.True(t, acquiredSession.Fresh())
	})
}

// go test -v -run=^$ -bench=Benchmark_Session -benchmem -count=4
func Benchmark_Session(b *testing.B) {
	app, store := fiber.New(), New()
	c := app.NewCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.SetCookie(store.sessionName, "12356789")

	b.Run("default", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			sess, _ := store.Get(c)
			sess.Set("john", "doe")
			_ = sess.Save()
		}
	})

	b.Run("storage", func(b *testing.B) {
		store = New(Config{
			Storage: memory.New(),
		})
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			sess, _ := store.Get(c)
			sess.Set("john", "doe")
			_ = sess.Save()
		}
	})
}
