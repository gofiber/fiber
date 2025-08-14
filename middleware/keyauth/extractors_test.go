package keyauth

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
		token, err := FromParam("api_key").Extract(c)
		require.Empty(t, token)
		require.Equal(t, ErrMissingAPIKeyInParam, err)
		return nil
	})
	_, err := app.Test(newRequest(fiber.MethodGet, "/test"))
	require.NoError(t, err)

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// Missing form
	token, err := FromForm("api_key").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrMissingAPIKeyInForm, err)

	// Missing query
	token, err = FromQuery("api_key").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrMissingAPIKeyInQuery, err)

	// Missing header
	token, err = FromHeader("X-Api-Key").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrMissingAPIKeyInHeader, err)

	// Missing Auth header
	token, err = FromAuthHeader(fiber.HeaderAuthorization, "Bearer").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrMissingAPIKeyInHeader, err)

	// Missing cookie
	token, err = FromCookie("api_key").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrMissingAPIKeyInCookie, err)
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
	app.Get("/test/:api_key", func(c fiber.Ctx) error {
		token, err := FromParam("api_key").Extract(c)
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
	ctx.Request().SetBodyString("api_key=token_from_form")
	token, err := FromForm("api_key").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_form", token)

	// FromQuery
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().SetRequestURI("/?api_key=token_from_query")
	token, err = FromQuery("api_key").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_query", token)

	// FromHeader
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set("X-Api-Key", "token_from_header")
	token, err = FromHeader("X-Api-Key").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_header", token)

	// FromAuthHeader
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set(fiber.HeaderAuthorization, "Bearer token_from_auth_header")
	token, err = FromAuthHeader(fiber.HeaderAuthorization, "Bearer").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_auth_header", token)

	// FromCookie
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.SetCookie("api_key", "token_from_cookie")
	token, err = FromCookie("api_key").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_cookie", token)
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
	require.Equal(t, ErrMissingAPIKey, err)

	// First extractor succeeds
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set("X-Api-Key", "token_from_header")
	ctx.Request().SetRequestURI("/?api_key=token_from_query")
	token, err = Chain(FromHeader("X-Api-Key"), FromQuery("api_key")).Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_header", token)

	// Second extractor succeeds
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().SetRequestURI("/?api_key=token_from_query")
	token, err = Chain(FromHeader("X-Api-Key"), FromQuery("api_key")).Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_query", token)

	// All extractors fail, should return the last error
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	token, err = Chain(FromHeader("X-Api-Key"), FromQuery("api_key")).Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrMissingAPIKeyInQuery, err)

	// All extractors find nothing (return empty string and nil error), should return ErrMissingAPIKey
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	// This extractor will return "", nil
	dummyExtractor := Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "", nil
		},
		Source: SourceCustom,
		Key:    "api_key",
	}
	token, err = Chain(dummyExtractor).Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrMissingAPIKey, err)
}
