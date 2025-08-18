package session

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Session
func Test_Session(t *testing.T) {
	t.Parallel()

	// session store
	store := NewStore()

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

	// set session using default cookie extractor
	ctx.Request().Header.SetCookie("session_id", token)

	// get session
	sess, err = store.Get(ctx)
	require.NoError(t, err)
	require.False(t, sess.Fresh())

	// get keys
	keys := sess.Keys()
	require.Equal(t, []any{}, keys)

	// get value
	name := sess.Get("name")
	require.Nil(t, name)

	// set value
	sess.Set("name", "john")

	// get value
	name = sess.Get("name")
	require.Equal(t, "john", name)

	keys = sess.Keys()
	require.Equal(t, []any{"name"}, keys)

	// delete key
	sess.Delete("name")

	// get value
	name = sess.Get("name")
	require.Nil(t, name)

	// get keys
	keys = sess.Keys()
	require.Equal(t, []any{}, keys)

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
	ctx.Request().Header.SetCookie("session_id", id)
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
	store := NewStore()

	// fiber instance
	app := fiber.New()

	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	// set cookie
	ctx.Request().Header.SetCookie("session_id", "123")

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

	ctx.Request().Header.SetCookie("session_id", newSessionIDString)

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
	store := NewStore()
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
	ctx.Request().Header.SetCookie("session_id", sess.ID())
	require.NoError(t, sess.Save())

	// reset store
	require.NoError(t, store.Reset(ctx))
	id := sess.ID()

	sess.Release()
	app.ReleaseCtx(ctx)
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.SetCookie("session_id", id)

	// make sure the session is recreated
	sess, err = store.Get(ctx)
	defer sess.Release()
	require.NoError(t, err)
	require.True(t, sess.Fresh())
	require.Nil(t, sess.Get("hello"))
}

func Test_Session_KeyTypes(t *testing.T) {
	t.Parallel()

	// session store
	store := NewStore()
	// fiber instance
	app := fiber.New()
	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	// get session
	sess, err := store.Get(ctx)
	require.NoError(t, err)
	require.True(t, sess.Fresh())

	type Person struct {
		Name string
	}

	type unexportedKey int

	// register non-default types
	store.RegisterType(Person{})
	store.RegisterType(unexportedKey(0))

	type unregisteredKeyType int
	type unregisteredValueType int

	// verify unregistered keys types are not allowed
	var (
		unregisteredKey   unregisteredKeyType
		unregisteredValue unregisteredValueType
	)
	sess.Set(unregisteredKey, "test")
	err = sess.Save()
	require.Error(t, err)
	sess.Delete(unregisteredKey)
	err = sess.Save()
	require.NoError(t, err)
	sess.Set("abc", unregisteredValue)
	err = sess.Save()
	require.Error(t, err)
	sess.Delete("abc")
	err = sess.Save()
	require.NoError(t, err)

	require.NoError(t, sess.Reset())

	var (
		kbool                     = true
		kstring                   = "str"
		kint                      = 13
		kint8          int8       = 13
		kint16         int16      = 13
		kint32         int32      = 13
		kint64         int64      = 13
		kuint          uint       = 13
		kuint8         uint8      = 13
		kuint16        uint16     = 13
		kuint32        uint32     = 13
		kuint64        uint64     = 13
		kuintptr       uintptr    = 13
		kbyte          byte       = 'k'
		krune                     = 'k'
		kfloat32       float32    = 13
		kfloat64       float64    = 13
		kcomplex64     complex64  = 13
		kcomplex128    complex128 = 13
		kuser                     = Person{Name: "John"}
		kunexportedKey            = unexportedKey(13)
	)

	var (
		vbool                     = true
		vstring                   = "str"
		vint                      = 13
		vint8          int8       = 13
		vint16         int16      = 13
		vint32         int32      = 13
		vint64         int64      = 13
		vuint          uint       = 13
		vuint8         uint8      = 13
		vuint16        uint16     = 13
		vuint32        uint32     = 13
		vuint64        uint64     = 13
		vuintptr       uintptr    = 13
		vbyte          byte       = 'k'
		vrune                     = 'k'
		vfloat32       float32    = 13
		vfloat64       float64    = 13
		vcomplex64     complex64  = 13
		vcomplex128    complex128 = 13
		vuser                     = Person{Name: "John"}
		vunexportedKey            = unexportedKey(13)
	)

	keys := []any{
		kbool,
		kstring,
		kint,
		kint8,
		kint16,
		kint32,
		kint64,
		kuint,
		kuint8,
		kuint16,
		kuint32,
		kuint64,
		kuintptr,
		kbyte,
		krune,
		kfloat32,
		kfloat64,
		kcomplex64,
		kcomplex128,
		kuser,
		kunexportedKey,
	}

	values := []any{
		vbool,
		vstring,
		vint,
		vint8,
		vint16,
		vint32,
		vint64,
		vuint,
		vuint8,
		vuint16,
		vuint32,
		vuint64,
		vuintptr,
		vbyte,
		vrune,
		vfloat32,
		vfloat64,
		vcomplex64,
		vcomplex128,
		vuser,
		vunexportedKey,
	}

	// loop test all key value pairs
	for i, key := range keys {
		sess.Set(key, values[i])
	}

	id := sess.ID()
	ctx.Request().Header.SetCookie("session_id", id)
	// save session
	err = sess.Save()
	require.NoError(t, err)

	sess.Release()
	app.ReleaseCtx(ctx)
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.SetCookie("session_id", id)

	// get session
	sess, err = store.Get(ctx)
	require.NoError(t, err)
	defer sess.Release()
	require.False(t, sess.Fresh())

	// loop test all key value pairs
	for i, key := range keys {
		// get value
		result := sess.Get(key)
		require.Equal(t, values[i], result)
	}
}

// go test -run Test_Session_Save
func Test_Session_Save(t *testing.T) {
	t.Parallel()

	t.Run("save to cookie", func(t *testing.T) {
		t.Parallel()
		// session store
		store := NewStore()
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
		store := NewStore(Config{
			Extractor: FromHeader("session_id"),
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
		require.Equal(t, sess.ID(), string(ctx.Response().Header.Peek("session_id")))
		sess.Release()
	})
}

// Test chained extractors to ensure both cookie and header are set when both are present
func Test_Session_ChainedExtractors(t *testing.T) {
	t.Parallel()

	t.Run("cookie and header chain", func(t *testing.T) {
		t.Parallel()
		// session store with chained extractors
		store := NewStore(Config{
			Extractor: Chain(FromCookie("session_id"), FromHeader("x-session-id")),
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

		// verify both cookie and header are set
		cookie := ctx.Response().Header.PeekCookie("session_id")
		require.NotNil(t, cookie)
		require.Contains(t, string(cookie), sess.ID())

		header := string(ctx.Response().Header.Peek("x-session-id"))
		require.Equal(t, sess.ID(), header)

		sess.Release()
	})

	t.Run("header and cookie chain", func(t *testing.T) {
		t.Parallel()
		// session store with chained extractors (different order)
		store := NewStore(Config{
			Extractor: Chain(FromHeader("x-session-id"), FromCookie("session_id")),
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

		// verify both header and cookie are set
		header := string(ctx.Response().Header.Peek("x-session-id"))
		require.Equal(t, sess.ID(), header)

		cookie := ctx.Response().Header.PeekCookie("session_id")
		require.NotNil(t, cookie)
		require.Contains(t, string(cookie), sess.ID())

		sess.Release()
	})

	t.Run("only SourceOther extractors - no response setting", func(t *testing.T) {
		t.Parallel()
		// session store with only query/form extractors
		store := NewStore(Config{
			Extractor: Chain(FromQuery("session_id"), FromForm("session_id")),
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

		// verify no cookie or header is set
		cookie := ctx.Response().Header.PeekCookie("session_id")
		require.Nil(t, cookie)

		header := string(ctx.Response().Header.Peek("session_id"))
		require.Empty(t, header)

		sess.Release()
	})

	t.Run("mixed chain with SourceOther", func(t *testing.T) {
		t.Parallel()
		// session store with mixed extractors including SourceOther
		store := NewStore(Config{
			Extractor: Chain(FromCookie("session_id"), FromQuery("session_id"), FromHeader("x-session-id")),
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

		// verify both cookie and header are set (query is ignored for response)
		cookie := ctx.Response().Header.PeekCookie("session_id")
		require.NotNil(t, cookie)
		require.Contains(t, string(cookie), sess.ID())

		header := string(ctx.Response().Header.Peek("x-session-id"))
		require.Equal(t, sess.ID(), header)

		sess.Release()
	})
}

func Test_Session_Save_IdleTimeout(t *testing.T) {
	t.Parallel()

	t.Run("save to cookie", func(t *testing.T) {
		t.Parallel()

		const sessionDuration = 5 * time.Second
		// session store
		store := NewStore()
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
		ctx.Request().Header.SetCookie("session_id", token)
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
		ctx.Request().Header.SetCookie("session_id", token)
		sess, err = store.Get(ctx)
		defer sess.Release()
		require.NoError(t, err)
		require.Nil(t, sess.Get("name"))
		require.NotEqual(t, sess.ID(), token)
	})
}

func Test_Session_Save_AbsoluteTimeout(t *testing.T) {
	t.Parallel()

	t.Run("save to cookie", func(t *testing.T) {
		t.Parallel()

		const absoluteTimeout = 1 * time.Second
		// session store
		store := NewStore(Config{
			IdleTimeout:     absoluteTimeout,
			AbsoluteTimeout: absoluteTimeout,
		})

		// force change to IdleTimeout
		store.Config.IdleTimeout = 10 * time.Second

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

		// save session
		err = sess.Save()
		require.NoError(t, err)

		sess.Release()
		app.ReleaseCtx(ctx)
		ctx = app.AcquireCtx(&fasthttp.RequestCtx{})

		// here you need to get the old session yet
		ctx.Request().Header.SetCookie("session_id", token)
		sess, err = store.Get(ctx)
		require.NoError(t, err)
		require.Equal(t, "john", sess.Get("name"))

		// just to make sure the session has been expired
		time.Sleep(absoluteTimeout + (100 * time.Millisecond))

		sess.Release()

		app.ReleaseCtx(ctx)
		ctx = app.AcquireCtx(&fasthttp.RequestCtx{})

		// here you should get a new session
		ctx.Request().Header.SetCookie("session_id", token)
		sess, err = store.Get(ctx)
		require.NoError(t, err)
		require.Nil(t, sess.Get("name"))
		require.NotEqual(t, sess.ID(), token)
		require.True(t, sess.Fresh())
		require.IsType(t, time.Time{}, sess.Get(absExpirationKey))

		token = sess.ID()

		sess.Set("name", "john")

		// save session
		err = sess.Save()
		require.NoError(t, err)

		sess.Release()
		app.ReleaseCtx(ctx)

		// just to make sure the session has been expired
		time.Sleep(absoluteTimeout + (100 * time.Millisecond))

		// try to get expired session by id
		sess, err = store.GetByID(ctx, token)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrSessionIDNotFoundInStore)
		require.Nil(t, sess)
	})
}

// go test -run Test_Session_Destroy
func Test_Session_Destroy(t *testing.T) {
	t.Parallel()

	t.Run("destroy from cookie", func(t *testing.T) {
		t.Parallel()
		// session store
		store := NewStore()
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
		store := NewStore(Config{
			Extractor: FromHeader("session_id"),
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
		ctx.Request().Header.Set("session_id", id)
		sess, err = store.Get(ctx)
		require.NoError(t, err)
		defer sess.Release()

		err = sess.Destroy()
		require.NoError(t, err)
		require.Equal(t, "", string(ctx.Response().Header.Peek("session_id")))
	})
}

// go test -run Test_Session_Custom_Config
func Test_Session_Custom_Config(t *testing.T) {
	t.Parallel()

	store := NewStore(Config{IdleTimeout: time.Hour, KeyGenerator: func() string { return "very random" }})
	require.Equal(t, time.Hour, store.IdleTimeout)
	require.Equal(t, "very random", store.KeyGenerator())

	store = NewStore(Config{IdleTimeout: 0})
	require.Equal(t, ConfigDefault.IdleTimeout, store.IdleTimeout)
}

// go test -run Test_Session_Cookie
func Test_Session_Cookie(t *testing.T) {
	t.Parallel()
	// session store
	store := NewStore()
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
	cookie := ctx.Response().Header.PeekCookie("session_id")
	require.NotNil(t, cookie)
	require.Regexp(t, `^session_id=[a-f0-9\-]{36}; max-age=\d+; path=/; SameSite=Lax$`, string(cookie))
}

// go test -run Test_Session_Cookie_SameSite
func Test_Session_Cookie_SameSite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expectedInHeader string
		name             string
		sameSite         string
		initialSecure    bool
	}{
		{
			name:             "Lax should not force secure",
			sameSite:         "Lax",
			initialSecure:    false,
			expectedInHeader: "SameSite=Lax",
		},
		{
			name:             "Lax with secure should stay secure",
			sameSite:         "Lax",
			initialSecure:    true,
			expectedInHeader: "SameSite=Lax; secure",
		},
		{
			name:             "Strict should not force secure",
			sameSite:         "Strict",
			initialSecure:    false,
			expectedInHeader: "SameSite=Strict",
		},
		{
			name:             "Strict with secure should stay secure",
			sameSite:         "Strict",
			initialSecure:    true,
			expectedInHeader: "SameSite=Strict; secure",
		},
		{
			name:             "None should force secure",
			sameSite:         "None",
			initialSecure:    false,
			expectedInHeader: "SameSite=None; secure",
		},
		{
			name:             "None with secure should stay secure",
			sameSite:         "None",
			initialSecure:    true,
			expectedInHeader: "SameSite=None; secure",
		},
		{
			name:             "Case-insensitive none should force secure",
			sameSite:         "none",
			initialSecure:    false,
			expectedInHeader: "SameSite=None; secure",
		},
		{
			name:             "Case-insensitive strict should not force secure",
			sameSite:         "strict",
			initialSecure:    false,
			expectedInHeader: "SameSite=Strict",
		},
		{
			name:             "Default should be Lax",
			sameSite:         "invalid",
			initialSecure:    false,
			expectedInHeader: "SameSite=Lax",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// session store
			store := NewStore(Config{
				CookieSameSite: tc.sameSite,
				CookieSecure:   tc.initialSecure,
			})

			// fiber instance
			app := fiber.New()

			// fiber context
			ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(ctx)

			// get session
			sess, err := store.Get(ctx)
			require.NoError(t, err)
			defer sess.Release()

			// save session to trigger cookie setting
			err = sess.Save()
			require.NoError(t, err)

			// check cookie
			cookie := string(ctx.Response().Header.PeekCookie("session_id"))
			// The order of attributes in the cookie string is not guaranteed.
			// Instead of checking for a single substring, we check for the presence of each part.
			parts := strings.Split(tc.expectedInHeader, "; ")
			for _, part := range parts {
				require.Contains(t, cookie, part)
			}

			// Also check that secure is NOT present when it shouldn't be
			if !tc.initialSecure && tc.sameSite != "None" && tc.sameSite != "none" {
				require.NotContains(t, cookie, "secure")
			}
		})
	}
}

// go test -run Test_Session_Cookie_In_Response
// Regression: https://github.com/gofiber/fiber/pull/1191
func Test_Session_Cookie_In_Middleware_Chain(t *testing.T) {
	t.Parallel()
	store := NewStore()
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
	store := NewStore()
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
	ctx.Request().Header.SetCookie("session_id", id)

	sess, err = store.Get(ctx)
	require.NoError(t, err)
	sess.Delete("id")
	require.NoError(t, sess.Save())

	sess.Release()
	app.ReleaseCtx(ctx)
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	ctx.Request().Header.SetCookie("session_id", id)

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
	store := NewStore()

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
		ctx.Request().Header.SetCookie("session_id", originalSessionUUIDString)

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
		require.Equal(t, []any{}, keys)

		// Set a new value for 'name' and check that it's updated
		acquiredSession.Set("name", "john")
		require.Equal(t, "john", acquiredSession.Get("name"))
		require.Nil(t, acquiredSession.Get("email"))

		// Save after resetting
		err = acquiredSession.Save()
		require.NoError(t, err)

		acquiredSession.Release()

		// Check that the session id is not in the header or cookie anymore
		require.Equal(t, "", string(ctx.Response().Header.Peek("session_id")))
		require.Equal(t, "", string(ctx.Request().Header.Peek("session_id")))

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
		store := NewStore()
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
		ctx.Request().Header.SetCookie("session_id", originalSessionUUIDString)

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
		app, store := fiber.New(), NewStore()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.Request().Header.SetCookie("session_id", "12356789")

		b.ReportAllocs()
		for b.Loop() {
			sess, _ := store.Get(c) //nolint:errcheck // We're inside a benchmark
			sess.Set("john", "doe")
			_ = sess.Save() //nolint:errcheck // We're inside a benchmark

			sess.Release()
		}
	})

	b.Run("storage", func(b *testing.B) {
		app := fiber.New()
		store := NewStore(Config{
			Storage: memory.New(),
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.Request().Header.SetCookie("session_id", "12356789")

		b.ReportAllocs()
		for b.Loop() {
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
		app, store := fiber.New(), NewStore()
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				c := app.AcquireCtx(&fasthttp.RequestCtx{})
				c.Request().Header.SetCookie("session_id", "12356789")

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
		store := NewStore(Config{
			Storage: memory.New(),
		})
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				c := app.AcquireCtx(&fasthttp.RequestCtx{})
				c.Request().Header.SetCookie("session_id", "12356789")

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
		app, store := fiber.New(), NewStore()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.Request().Header.SetCookie("session_id", "12356789")

		b.ReportAllocs()
		for b.Loop() {
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
		store := NewStore(Config{
			Storage: memory.New(),
		})
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.Request().Header.SetCookie("session_id", "12356789")

		b.ReportAllocs()
		for b.Loop() {
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
		app, store := fiber.New(), NewStore()
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				c := app.AcquireCtx(&fasthttp.RequestCtx{})
				c.Request().Header.SetCookie("session_id", "12356789")

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
		store := NewStore(Config{
			Storage: memory.New(),
		})
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				c := app.AcquireCtx(&fasthttp.RequestCtx{})
				c.Request().Header.SetCookie("session_id", "12356789")

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
	store := NewStore()

	var wg sync.WaitGroup
	errChan := make(chan error, 10) // Buffered channel to collect errors
	const numGoroutines = 10        // Number of concurrent goroutines to test

	// Start numGoroutines goroutines
	for range numGoroutines {
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
			if saveErr := sess.Save(); saveErr != nil {
				errChan <- saveErr
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
			localCtx.Request().Header.SetCookie("session_id", id)

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

func Test_Session_StoreGetDecodeSessionDataError(t *testing.T) {
	// Initialize a new store with default config
	store := NewStore()

	// Create a new Fiber app
	app := fiber.New()

	// Generate a fake session ID
	sessionID := uuid.New().String()

	// Store invalid session data to simulate decode error
	err := store.Storage.Set(sessionID, []byte("invalid data"), 0)
	require.NoError(t, err, "Failed to set invalid session data")

	// Create a new request context
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	// Set the session ID in cookies
	c.Request().Header.SetCookie("session_id", sessionID)

	// Attempt to get the session
	_, err = store.Get(c)
	require.Error(t, err, "Expected error due to invalid session data, but got nil")

	// Check that the error message is as expected
	require.Contains(t, err.Error(), "failed to decode session data", "Unexpected error message")

	// Check that the error is as expected
	require.ErrorContains(t, err, "failed to decode session data", "Unexpected error")

	// Attempt to get the session by ID
	_, err = store.GetByID(c, sessionID)
	require.Error(t, err, "Expected error due to invalid session data, but got nil")

	// Check that the error message is as expected
	require.ErrorContains(t, err, "failed to decode session data", "Unexpected error")
}
