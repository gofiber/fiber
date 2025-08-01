package csrf

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Extractors_Missing
func Test_Extractors_Missing(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	// Add a route to test the missing param
	app.Get("/test", func(c fiber.Ctx) error {
		token, err := FromParam("csrf").Extract(c)
		require.Empty(t, token)
		require.Equal(t, ErrMissingParam, err)
		return nil
	})
	_, err := app.Test(newRequest(fiber.MethodGet, "/test"))
	require.NoError(t, err)

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// Missing form
	token, err := FromForm("csrf").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrMissingForm, err)

	// Missing query
	token, err = FromQuery("csrf").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrMissingQuery, err)

	// Missing header
	token, err = FromHeader("X-Csrf-Token").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrMissingHeader, err)
}

// newRequest creates a new *http.Request for Fiber's app.Test
func newRequest(method, target string) *http.Request {
	req, err := http.NewRequestWithContext(context.Background(), method, target, nil)
	if err != nil {
		panic(err)
	}
	return req
}

// go test -run Test_Extractors
func Test_Extractors(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// FromParam
	app.Get("/test/:csrf", func(c fiber.Ctx) error {
		token, err := FromParam("csrf").Extract(c)
		require.NoError(t, err)
		require.Equal(t, "token_from_param", token)
		return nil
	})
	_, err := app.Test(newRequest(fiber.MethodGet, "/test/token_from_param"))
	require.NoError(t, err)

	// FromForm
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.SetContentType(fiber.MIMEApplicationForm)
	ctx.Request().SetBodyString("csrf=token_from_form")
	token, err := FromForm("csrf").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_form", token)

	// FromQuery
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().SetRequestURI("/?csrf=token_from_query")
	token, err = FromQuery("csrf").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_query", token)

	// FromHeader
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set("X-Csrf-Token", "token_from_header")
	token, err = FromHeader("X-Csrf-Token").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_header", token)
}

// go test -run Test_Extractor_Chain
func Test_Extractor_Chain(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// No extractors
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	token, err := Chain().Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrTokenNotFound, err)

	// First extractor succeeds
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set("X-Csrf-Token", "token_from_header")
	ctx.Request().SetRequestURI("/?csrf=token_from_query")
	token, err = Chain(FromHeader("X-Csrf-Token"), FromQuery("csrf")).Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_header", token)

	// Second extractor succeeds
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().SetRequestURI("/?csrf=token_from_query")
	token, err = Chain(FromHeader("X-Csrf-Token"), FromQuery("csrf")).Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_query", token)

	// All extractors fail, should return the last error
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	token, err = Chain(FromHeader("X-Csrf-Token"), FromQuery("csrf")).Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrMissingQuery, err)

	// All extractors find nothing (return empty string and nil error), should return ErrTokenNotFound
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	// This extractor will return "", nil
	dummyExtractor := Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "", nil
		},
		Source: SourceCustom,
		Key:    "_csrf",
	}
	token, err = Chain(dummyExtractor).Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrTokenNotFound, err)
}
