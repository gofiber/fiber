package fiber

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// newRadixApp returns a Fiber app with radix routing enabled.
func newRadixApp() *App {
	return New(Config{UseRadix: true})
}

func Test_Router_Radix_Wildcard(t *testing.T) {
	t.Parallel()
	app := newRadixApp()
	app.Get("/*", func(c Ctx) error {
		return c.SendString(c.Params("*"))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/foo/bar", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "foo/bar", app.getString(body))
}

func Test_Router_Radix_Param(t *testing.T) {
	t.Parallel()
	app := newRadixApp()
	app.Get("/user/:id", func(c Ctx) error {
		return c.SendString(c.Params("id"))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/user/42", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "42", app.getString(body))
}

func Test_Router_Radix_Group(t *testing.T) {
	t.Parallel()
	app := newRadixApp()
	g := app.Group("/v1")
	g.Get("/test", func(c Ctx) error { return c.SendStatus(StatusOK) })

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/test", nil))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
}

func Test_Router_Radix_RebuildTree(t *testing.T) {
	t.Parallel()
	app := newRadixApp()
	app.Get("/foo", func(c Ctx) error { return c.SendStatus(StatusOK) })

	// trigger initial tree build
	_, err := app.Test(httptest.NewRequest(MethodGet, "/foo", nil))
	require.NoError(t, err)

	app.Get("/bar", func(c Ctx) error { return c.SendStatus(StatusCreated) })
	app.RebuildTree()

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/bar", nil))
	require.NoError(t, err)
	require.Equal(t, StatusCreated, resp.StatusCode)
}
