package session

import (
	"testing"

	"github.com/gofiber/fiber/v2"
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
	ctx.Request().Header.SetCookie("session_id", "123")

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
