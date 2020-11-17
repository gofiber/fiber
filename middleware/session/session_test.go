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

	// cookie should not be set if empty data
	utils.AssertEqual(t, 0, len(ctx.Response().Header.PeekCookie(store.CookieName)))
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
