package session

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/storage/memory"
	"github.com/gofiber/fiber/v2/utils"

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
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// set session
	ctx.Request().Header.SetCookie(store.sessionName, "123")

	// get session
	sess, err := store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, sess.Fresh())

	// get keys
	keys := sess.Keys()
	utils.AssertEqual(t, []string{}, keys)

	// get value
	name := sess.Get("name")
	utils.AssertEqual(t, nil, name)

	// set value
	sess.Set("name", "john")

	// get value
	name = sess.Get("name")
	utils.AssertEqual(t, "john", name)

	keys = sess.Keys()
	utils.AssertEqual(t, []string{"name"}, keys)

	// delete key
	sess.Delete("name")

	// get value
	name = sess.Get("name")
	utils.AssertEqual(t, nil, name)

	// get keys
	keys = sess.Keys()
	utils.AssertEqual(t, []string{}, keys)

	// get id
	id := sess.ID()
	utils.AssertEqual(t, "123", id)

	// save the old session first
	err = sess.Save()
	utils.AssertEqual(t, nil, err)

	// requesting entirely new context to prevent falsy tests
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	sess, err = store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, sess.Fresh())

	// this id should be randomly generated as session key was deleted
	utils.AssertEqual(t, 36, len(sess.ID()))

	// when we use the original session for the second time
	// the session be should be same if the session is not expired
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// request the server with the old session
	ctx.Request().Header.SetCookie(store.sessionName, id)
	sess, err = store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, false, sess.Fresh())
	utils.AssertEqual(t, sess.id, id)
}

// go test -run Test_Session_Types
//
//nolint:forcetypeassert // TODO: Do not force-type assert
func Test_Session_Types(t *testing.T) {
	t.Parallel()

	// session store
	store := New()

	// fiber instance
	app := fiber.New()

	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// set cookie
	ctx.Request().Header.SetCookie(store.sessionName, "123")

	// get session
	sess, err := store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, sess.Fresh())

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
	utils.AssertEqual(t, nil, err)

	// get session
	sess, err = store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, false, sess.Fresh())

	// get value
	utils.AssertEqual(t, vuser, sess.Get("vuser").(User))
	utils.AssertEqual(t, vbool, sess.Get("vbool").(bool))
	utils.AssertEqual(t, vstring, sess.Get("vstring").(string))
	utils.AssertEqual(t, vint, sess.Get("vint").(int))
	utils.AssertEqual(t, vint8, sess.Get("vint8").(int8))
	utils.AssertEqual(t, vint16, sess.Get("vint16").(int16))
	utils.AssertEqual(t, vint32, sess.Get("vint32").(int32))
	utils.AssertEqual(t, vint64, sess.Get("vint64").(int64))
	utils.AssertEqual(t, vuint, sess.Get("vuint").(uint))
	utils.AssertEqual(t, vuint8, sess.Get("vuint8").(uint8))
	utils.AssertEqual(t, vuint16, sess.Get("vuint16").(uint16))
	utils.AssertEqual(t, vuint32, sess.Get("vuint32").(uint32))
	utils.AssertEqual(t, vuint64, sess.Get("vuint64").(uint64))
	utils.AssertEqual(t, vuintptr, sess.Get("vuintptr").(uintptr))
	utils.AssertEqual(t, vbyte, sess.Get("vbyte").(byte))
	utils.AssertEqual(t, vrune, sess.Get("vrune").(rune))
	utils.AssertEqual(t, vfloat32, sess.Get("vfloat32").(float32))
	utils.AssertEqual(t, vfloat64, sess.Get("vfloat64").(float64))
	utils.AssertEqual(t, vcomplex64, sess.Get("vcomplex64").(complex64))
	utils.AssertEqual(t, vcomplex128, sess.Get("vcomplex128").(complex128))
}

// go test -run Test_Session_Store_Reset
func Test_Session_Store_Reset(t *testing.T) {
	t.Parallel()
	// session store
	store := New()
	// fiber instance
	app := fiber.New()
	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// get session
	sess, err := store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	// make sure its new
	utils.AssertEqual(t, true, sess.Fresh())
	// set value & save
	sess.Set("hello", "world")
	ctx.Request().Header.SetCookie(store.sessionName, sess.ID())
	utils.AssertEqual(t, nil, sess.Save())

	// reset store
	utils.AssertEqual(t, nil, store.Reset())

	// make sure the session is recreated
	sess, err = store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, sess.Fresh())
	utils.AssertEqual(t, nil, sess.Get("hello"))
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
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)
		// get session
		sess, err := store.Get(ctx)
		utils.AssertEqual(t, nil, err)
		// set value
		sess.Set("name", "john")

		// save session
		err = sess.Save()
		utils.AssertEqual(t, nil, err)
	})

	t.Run("save to header", func(t *testing.T) {
		// session store
		store := New(Config{
			KeyLookup: "header:session_id",
		})
		// fiber instance
		app := fiber.New()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)
		// get session
		sess, err := store.Get(ctx)
		utils.AssertEqual(t, nil, err)
		// set value
		sess.Set("name", "john")

		// save session
		err = sess.Save()
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, store.getSessionID(ctx), string(ctx.Response().Header.Peek(store.sessionName)))
		utils.AssertEqual(t, store.getSessionID(ctx), string(ctx.Request().Header.Peek(store.sessionName)))
	})
}

func Test_Session_Save_Expiration(t *testing.T) {
	t.Parallel()

	t.Run("save to cookie", func(t *testing.T) {
		t.Parallel()
		// session store
		store := New()
		// fiber instance
		app := fiber.New()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)
		// get session
		sess, err := store.Get(ctx)
		utils.AssertEqual(t, nil, err)
		// set value
		sess.Set("name", "john")

		// expire this session in 5 seconds
		sess.SetExpiry(time.Second * 5)

		// save session
		err = sess.Save()
		utils.AssertEqual(t, nil, err)

		// here you need to get the old session yet
		sess, err = store.Get(ctx)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "john", sess.Get("name"))

		// just to make sure the session has been expired
		time.Sleep(time.Second * 5)

		// here you should get a new session
		sess, err = store.Get(ctx)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, nil, sess.Get("name"))
	})
}

// go test -run Test_Session_Reset
func Test_Session_Reset(t *testing.T) {
	t.Parallel()

	t.Run("reset from cookie", func(t *testing.T) {
		t.Parallel()
		// session store
		store := New()
		// fiber instance
		app := fiber.New()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)
		// get session
		sess, err := store.Get(ctx)
		utils.AssertEqual(t, nil, err)

		sess.Set("name", "fenny")
		utils.AssertEqual(t, nil, sess.Destroy())
		name := sess.Get("name")
		utils.AssertEqual(t, nil, name)
	})

	t.Run("reset from header", func(t *testing.T) {
		t.Parallel()
		// session store
		store := New(Config{
			KeyLookup: "header:session_id",
		})
		// fiber instance
		app := fiber.New()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)
		// get session
		sess, err := store.Get(ctx)
		utils.AssertEqual(t, nil, err)

		// set value & save
		sess.Set("name", "fenny")
		utils.AssertEqual(t, nil, sess.Save())
		sess, err = store.Get(ctx)
		utils.AssertEqual(t, nil, err)

		err = sess.Destroy()
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "", string(ctx.Response().Header.Peek(store.sessionName)))
		utils.AssertEqual(t, "", string(ctx.Request().Header.Peek(store.sessionName)))
	})
}

// go test -run Test_Session_Custom_Config
func Test_Session_Custom_Config(t *testing.T) {
	t.Parallel()

	store := New(Config{Expiration: time.Hour, KeyGenerator: func() string { return "very random" }})
	utils.AssertEqual(t, time.Hour, store.Expiration)
	utils.AssertEqual(t, "very random", store.KeyGenerator())

	store = New(Config{Expiration: 0})
	utils.AssertEqual(t, ConfigDefault.Expiration, store.Expiration)
}

// go test -run Test_Session_Cookie
func Test_Session_Cookie(t *testing.T) {
	t.Parallel()
	// session store
	store := New()
	// fiber instance
	app := fiber.New()
	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// get session
	sess, err := store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, nil, sess.Save())

	// cookie should be set on Save ( even if empty data )
	utils.AssertEqual(t, 84, len(ctx.Response().Header.PeekCookie(store.sessionName)))
}

// go test -run Test_Session_Cookie_In_Response
func Test_Session_Cookie_In_Response(t *testing.T) {
	t.Parallel()
	store := New()
	app := fiber.New()

	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// get session
	sess, err := store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	sess.Set("id", "1")
	utils.AssertEqual(t, true, sess.Fresh())
	utils.AssertEqual(t, nil, sess.Save())

	sess, err = store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	sess.Set("name", "john")
	utils.AssertEqual(t, true, sess.Fresh())

	utils.AssertEqual(t, "1", sess.Get("id"))
	utils.AssertEqual(t, "john", sess.Get("name"))
}

// go test -run Test_Session_Deletes_Single_Key
// Regression: https://github.com/gofiber/fiber/issues/1365
func Test_Session_Deletes_Single_Key(t *testing.T) {
	t.Parallel()
	store := New()
	app := fiber.New()

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	sess, err := store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	ctx.Request().Header.SetCookie(store.sessionName, sess.ID())

	sess.Set("id", "1")
	utils.AssertEqual(t, nil, sess.Save())

	sess, err = store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	sess.Delete("id")
	utils.AssertEqual(t, nil, sess.Save())

	sess, err = store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, false, sess.Fresh())
	utils.AssertEqual(t, nil, sess.Get("id"))
}

// go test -run Test_Session_Regenerate
// Regression: https://github.com/gofiber/fiber/issues/1395
func Test_Session_Regenerate(t *testing.T) {
	t.Parallel()
	// fiber instance
	app := fiber.New()
	t.Run("set fresh to be true when regenerating a session", func(t *testing.T) {
		// session store
		store := New()
		// a random session uuid
		originalSessionUUIDString := ""
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// now the session is in the storage
		freshSession, err := store.Get(ctx)
		utils.AssertEqual(t, nil, err)

		originalSessionUUIDString = freshSession.ID()

		err = freshSession.Save()
		utils.AssertEqual(t, nil, err)

		// set cookie
		ctx.Request().Header.SetCookie(store.sessionName, originalSessionUUIDString)

		// as the session is in the storage, session.fresh should be false
		acquiredSession, err := store.Get(ctx)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, false, acquiredSession.Fresh())

		err = acquiredSession.Regenerate()
		utils.AssertEqual(t, nil, err)

		if acquiredSession.ID() == originalSessionUUIDString {
			t.Fatal("regenerate should generate another different id")
		}
		// acquiredSession.fresh should be true after regenerating
		utils.AssertEqual(t, true, acquiredSession.Fresh())
	})
}

// go test -v -run=^$ -bench=Benchmark_Session -benchmem -count=4
func Benchmark_Session(b *testing.B) {
	app, store := fiber.New(), New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.SetCookie(store.sessionName, "12356789")

	var err error
	b.Run("default", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			sess, _ := store.Get(c) //nolint:errcheck // We're inside a benchmark
			sess.Set("john", "doe")
			err = sess.Save()
		}

		utils.AssertEqual(b, nil, err)
	})

	b.Run("storage", func(b *testing.B) {
		store = New(Config{
			Storage: memory.New(),
		})
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			sess, _ := store.Get(c) //nolint:errcheck // We're inside a benchmark
			sess.Set("john", "doe")
			err = sess.Save()
		}

		utils.AssertEqual(b, nil, err)
	})
}
