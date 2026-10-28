package fiber

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Ctx_Fresh_ResponseStatus(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		status int
		fresh  bool
	}{
		{StatusOK, true},
		{StatusNoContent, true},
		{StatusPartialContent, true},
		{299, true},
		{StatusMovedPermanently, false},
		{StatusFound, false},
		{StatusNotModified, true},
		{StatusTemporaryRedirect, false},
		{StatusBadRequest, false},
		{StatusNotFound, false},
		{StatusPreconditionFailed, false},
		{StatusInternalServerError, false},
	} {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			t.Parallel()

			for _, conditional := range []struct {
				name  string
				value string
			}{
				{HeaderIfNoneMatch, `"version"`},
				{HeaderIfNoneMatch, "*"},
				{HeaderIfModifiedSince, "Wed, 21 Oct 2015 07:28:00 GMT"},
			} {
				app := New()
				ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
				ctx.Method(MethodGet)
				ctx.Request().Header.Set(conditional.name, conditional.value)
				ctx.Response().Header.Set(HeaderETag, `"version"`)
				ctx.Response().Header.Set(HeaderLastModified, "Wed, 21 Oct 2015 07:28:00 GMT")
				ctx.Status(tc.status)

				require.Equal(t, tc.fresh, ctx.Fresh(), "%s: %s", conditional.name, conditional.value)
				require.Equal(t, !tc.fresh, ctx.Stale(), "%s: %s", conditional.name, conditional.value)
				app.ReleaseCtx(ctx)
			}
		})
	}
}

func Test_Ctx_Fresh_PreservesNotFoundResponse(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(c Ctx) error {
		c.Status(StatusNotFound)
		if c.Fresh() {
			return c.SendStatus(StatusNotModified)
		}
		return c.SendString("resource no longer exists")
	})

	req := httptest.NewRequest(MethodGet, "/", http.NoBody)
	req.Header.Set(HeaderIfNoneMatch, "*")
	resp, err := app.Test(req)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, resp.Body.Close()) })
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
	require.Equal(t, "resource no longer exists", string(body))
}
