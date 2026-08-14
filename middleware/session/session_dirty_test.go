package session

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// A save without changes must still hit storage (the idle timeout keeps
// sliding) and keep the stored data intact.
func Test_Session_CleanSave_RefreshesStorageAndKeepsData(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	store := NewStore()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.SetCookie("session_id", "clean-save-id")

	sess, err := store.Get(c)
	require.NoError(t, err)
	sess.Set("uid", "1337")
	require.NoError(t, sess.Save())
	id := sess.ID()
	sess.Release()

	stored, err := store.Storage.Get(id)
	require.NoError(t, err)
	require.NotEmpty(t, stored)

	// read-only round trip
	sess, err = store.Get(c)
	require.NoError(t, err)
	require.Equal(t, "1337", sess.Get("uid"))
	require.NoError(t, sess.Save())
	sess.Release()

	afterRaw, err := store.Storage.Get(id)
	require.NoError(t, err)
	require.Equal(t, stored, afterRaw)

	sess, err = store.Get(c)
	require.NoError(t, err)
	require.Equal(t, "1337", sess.Get("uid"))
	sess.Release()
}

// Mutating a map obtained from Get must survive the save even when Set is
// never called.
func Test_Session_GetAliasableValue_MarksDirty(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	store := NewStore()
	store.RegisterType(map[string]string{})

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.SetCookie("session_id", "alias-id")

	sess, err := store.Get(c)
	require.NoError(t, err)
	sess.Set("prefs", map[string]string{"theme": "dark"})
	require.NoError(t, sess.Save())
	sess.Release()

	sess, err = store.Get(c)
	require.NoError(t, err)
	prefs, ok := sess.Get("prefs").(map[string]string)
	require.True(t, ok)
	prefs["theme"] = "light"
	require.NoError(t, sess.Save())
	sess.Release()

	sess, err = store.Get(c)
	require.NoError(t, err)
	prefs, ok = sess.Get("prefs").(map[string]string)
	require.True(t, ok)
	require.Equal(t, "light", prefs["theme"])
	sess.Release()
}

// After a dirty save the fresh encoding becomes the clean baseline, so a
// second save without writes must still leave decodable data behind.
func Test_Session_SecondCleanSave_AfterWrite(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	store := NewStore()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.SetCookie("session_id", "second-save-id")

	sess, err := store.Get(c)
	require.NoError(t, err)
	sess.Set("n", int64(42))
	require.NoError(t, sess.Save())
	require.NoError(t, sess.Save())
	sess.Release()

	sess, err = store.Get(c)
	require.NoError(t, err)
	require.Equal(t, int64(42), sess.Get("n"))
	sess.Release()
}

func Test_ValueMayAlias(t *testing.T) {
	t.Parallel()
	type named int64
	type withSlice struct{ B []byte }

	require.False(t, valueMayAlias(nil))
	require.False(t, valueMayAlias("s"))
	require.False(t, valueMayAlias(int(1)))
	require.False(t, valueMayAlias(int64(1)))
	require.False(t, valueMayAlias(uint32(1)))
	require.False(t, valueMayAlias(3.14))
	require.False(t, valueMayAlias(true))
	require.False(t, valueMayAlias(named(7)))

	require.True(t, valueMayAlias([]byte("b")))
	require.True(t, valueMayAlias(map[string]string{}))
	require.True(t, valueMayAlias(&withSlice{}))
	require.True(t, valueMayAlias(withSlice{B: []byte("x")}))
}
