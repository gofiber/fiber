package fiber

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_OptionalParam_EmptyMiddleDoesNotStarveRequired(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/:param1:param2?:param3", func(c Ctx) error {
		return c.SendString(c.Params("param1") + "|" + c.Params("param2") + "|" + c.Params("param3"))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/ac", nil))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "a||c", string(body))

	app2 := New()
	app2.Get("/test:optional?:mandatory", func(c Ctx) error {
		return c.SendString(c.Params("optional") + "|" + c.Params("mandatory"))
	})
	resp2, err := app2.Test(httptest.NewRequest(MethodGet, "/testo", nil))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp2.StatusCode)
	body2, err := io.ReadAll(resp2.Body)
	require.NoError(t, err)
	require.Equal(t, "|o", string(body2))
}
