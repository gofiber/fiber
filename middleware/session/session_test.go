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

	// set cookie
	ctx.Request().Header.SetCookie(store.CookieName, "123")

	// get session
	sess, err := store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, sess.Fresh())

	// get value
	name := sess.Get("name")
	utils.AssertEqual(t, nil, name)

	// set value
	sess.Set("name", "john")

	// get value
	name = sess.Get("name")
	utils.AssertEqual(t, "john", name)

	// delete key
	sess.Delete("name")

	// get value
	name = sess.Get("name")
	utils.AssertEqual(t, nil, name)

	// get id
	id := sess.ID()
	utils.AssertEqual(t, "123", id)

	// delete cookie
	ctx.Request().Header.Del(fiber.HeaderCookie)

	// get session
	sess, err = store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, sess.Fresh())

	// get id
	id = sess.ID()
	utils.AssertEqual(t, 36, len(id))
}

// go test -run Test_Session_Types
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
	ctx.Request().Header.SetCookie(store.CookieName, "123")

	// get session
	sess, err := store.Get(ctx)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, sess.Fresh())

	type User struct {
		Name string
	}
	store.RegisterType(User{})
	var vuser = User{
		Name: "John",
	}
	// set value
	var vbool bool = true
	var vstring string = "str"
	var vint int = 13
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
	sess, _ := store.Get(ctx)
	// make sure its new
	utils.AssertEqual(t, true, sess.Fresh())
	// set value & save
	sess.Set("hello", "world")
	ctx.Request().Header.SetCookie(store.CookieName, sess.ID())
	sess.Save()

	// reset store
	store.Reset()

	// make sure the session is recreated
	sess, _ = store.Get(ctx)
	utils.AssertEqual(t, true, sess.Fresh())
	utils.AssertEqual(t, nil, sess.Get("hello"))
}

// go test -run Test_Session_Save
func Test_Session_Save(t *testing.T) {
	t.Parallel()

	// session store
	store := New()

	// fiber instance
	app := fiber.New()

	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// get store
	sess, _ := store.Get(ctx)

	// set value
	sess.Set("name", "john")

	// save session
	err := sess.Save()
	utils.AssertEqual(t, nil, err)

}

// go test -run Test_Session_Reset
func Test_Session_Reset(t *testing.T) {
	t.Parallel()
	// session store
	store := New()
	// fiber instance
	app := fiber.New()
	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	// get session
	sess, _ := store.Get(ctx)

	sess.Set("name", "fenny")
	sess.Destroy()
	name := sess.Get("name")
	utils.AssertEqual(t, nil, name)
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
	sess, _ := store.Get(ctx)
	sess.Save()

	// cookie should be set on Save ( even if empty data )
	utils.AssertEqual(t, 84, len(ctx.Response().Header.PeekCookie(store.CookieName)))
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
	sess, _ := store.Get(ctx)
	sess.Set("id", "1")
	utils.AssertEqual(t, true, sess.Fresh())
	sess.Save()

	sess, _ = store.Get(ctx)
	sess.Set("name", "john")
	utils.AssertEqual(t, true, sess.Fresh())

	utils.AssertEqual(t, "1", sess.Get("id"))
	utils.AssertEqual(t, "john", sess.Get("name"))
}

// go test -v -run=^$ -bench=Benchmark_Session -benchmem -count=4
func Benchmark_Session(b *testing.B) {
	app, store := fiber.New(), New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.SetCookie(store.CookieName, "12356789")

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
