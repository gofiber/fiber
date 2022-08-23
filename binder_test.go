package fiber

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Binder(t *testing.T) {
	t.Parallel()
	app := New()

	ctx := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)
	ctx.values = [maxParams]string{"id string"}
	ctx.route = &Route{Params: []string{"id"}}
	ctx.Request().SetBody([]byte(`{"name": "john doe"}`))
	ctx.Request().Header.Set("content-type", "application/json")

	var req struct {
		ID string `param:"id"`
	}

	var body struct {
		Name string `json:"name"`
	}

	err := ctx.Bind().Req(&req).JSON(&body).Err()
	require.NoError(t, err)
	require.Equal(t, "id string", req.ID)
	require.Equal(t, "john doe", body.Name)
}
