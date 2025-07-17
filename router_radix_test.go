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

func Test_Router_Radix_OptionalPlusRegexEscaped(t *testing.T) {
	t.Parallel()
	app := newRadixApp()
	app.Get("/user/:name?", func(c Ctx) error {
		return c.SendString(c.Params("name"))
	})
	app.Get("/user/+", func(c Ctx) error {
		return c.SendString(c.Params("+"))
	})
	app.Get(`/:date<regex(\d{4}-\d{2}-\d{2})>`, func(c Ctx) error {
		return c.SendString(c.Params("date"))
	})
	app.Get(`/v1/some/resource/name\:customVerb`, func(c Ctx) error {
		return c.SendString("ok")
	})
	app.Get("/v1/*/shop/*", func(c Ctx) error {
		return c.SendString(c.Params("*1") + "," + c.Params("*2"))
	})

	// optional parameter
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/user", nil))
body, err := io.ReadAll(resp.Body)
require.NoError(t, err)
require.Equal(t, "", app.getString(body))

body, err = io.ReadAll(resp.Body)
require.NoError(t, err)
require.Equal(t, "john", app.getString(body))
	require.Equal(t, "john", app.getString(body))

	// plus parameter
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/user/1/2", nil))
	require.NoError(t, err)
body, err = io.ReadAll(resp.Body)
require.NoError(t, err)
require.Equal(t, "1/2", app.getString(body))
	require.Equal(t, "1/2", app.getString(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/user/", nil))
	require.NoError(t, err)
body, err = io.ReadAll(resp.Body)
require.NoError(t, err)
require.Equal(t, "", app.getString(body))
	require.Equal(t, "", app.getString(body))

	// regex constraint
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/2022-08-27", nil))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	require.Equal(t, "2022-08-27", app.getString(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/125", nil))
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)

	// escaped colon
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/v1/some/resource/name:customVerb", nil))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	require.Equal(t, "ok", app.getString(body))

	// multi wildcard
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/v1/brand/4/shop/blue/xs", nil))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	require.Equal(t, "brand/4,blue/xs", app.getString(body))
}
