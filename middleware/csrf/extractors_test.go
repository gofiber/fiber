package csrf

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Extractors_Missing
func Test_Extractors_Missing(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// Missing param
	token, err := FromParam("csrf")(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrMissingParam, err)

	// Missing form
	token, err = FromForm("csrf")(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrMissingForm, err)

	// Missing query
	token, err = FromQuery("csrf")(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrMissingQuery, err)
}
