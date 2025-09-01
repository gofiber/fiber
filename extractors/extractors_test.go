package extractors

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
		token, err := FromParam("token").Extract(c)
		require.Empty(t, token)
		require.Equal(t, ErrNotFound, err)
		return nil
	})
	_, err := app.Test(newRequest(fiber.MethodGet, "/test"))
	require.NoError(t, err)

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// Missing form
	token, err := FromForm("token").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// Missing query
	token, err = FromQuery("token").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// Missing header
	token, err = FromHeader("X-Token").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// Missing Auth header
	token, err = FromAuthHeader("Bearer").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// Missing cookie
	token, err = FromCookie("token").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)
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
	app.Get("/test/:token", func(c fiber.Ctx) error {
		token, err := FromParam("token").Extract(c)
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
	ctx.Request().Header.SetMethod(fiber.MethodPost)
	ctx.Request().SetBodyString("token=token_from_form")
	token, err := FromForm("token").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_form", token)

	// FromQuery
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().SetRequestURI("/?token=token_from_query")
	token, err = FromQuery("token").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_query", token)

	// FromHeader
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set("X-Token", "token_from_header")
	token, err = FromHeader("X-Token").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_header", token)

	// FromAuthHeader
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set(fiber.HeaderAuthorization, "Bearer token_from_auth_header")
	token, err = FromAuthHeader("Bearer").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_auth_header", token)

	// FromCookie
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.SetCookie("token", "token_from_cookie")
	token, err = FromCookie("token").Extract(ctx)
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
	require.Equal(t, ErrNotFound, err)

	// First extractor succeeds
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set("X-Token", "token_from_header")
	ctx.Request().SetRequestURI("/?token=token_from_query")
	token, err = Chain(FromHeader("X-Token"), FromQuery("token")).Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_header", token)

	// Second extractor succeeds
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().SetRequestURI("/?token=token_from_query")
	token, err = Chain(FromHeader("X-Token"), FromQuery("token")).Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token_from_query", token)

	// All extractors fail, should return the last error
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	token, err = Chain(FromHeader("X-Token"), FromQuery("token")).Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// All extractors find nothing (return empty string and nil error), should return ErrNotFound
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	// This extractor will return "", nil
	dummyExtractor := Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "", nil
		},
		Source: SourceCustom,
		Key:    "token",
	}
	token, err = Chain(dummyExtractor).Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)
}

// go test -run Test_Extractor_FromAuthHeader_EdgeCases
func Test_Extractor_FromAuthHeader_EdgeCases(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test case: Authorization header exists but doesn't match the expected scheme
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set(fiber.HeaderAuthorization, "Basic dXNlcjpwYXNz") // Basic auth instead of Bearer
	token, err := FromAuthHeader("Bearer").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// Test case: Authorization header exists but has wrong format
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set(fiber.HeaderAuthorization, "Bearertoken") // Missing space after Bearer
	token, err = FromAuthHeader("Bearer").Extract(ctx)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// Test case: Authorization header exists but scheme doesn't match case-insensitively
	ctx = app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set(fiber.HeaderAuthorization, "bearer token") // lowercase bearer
	token, err = FromAuthHeader("Bearer").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token", token)
}

// go test -run Test_Extractor_Chain_Introspection
func Test_Extractor_Chain_Introspection(t *testing.T) {
	t.Parallel()

	// Test chain introspection
	extractor1 := FromHeader("X-Token")
	extractor2 := FromQuery("token")
	extractor3 := FromCookie("auth")

	chainExtractor := Chain(extractor1, extractor2, extractor3)

	// Verify chain metadata
	require.Equal(t, SourceHeader, chainExtractor.Source)
	require.Equal(t, "X-Token", chainExtractor.Key)
	require.Len(t, chainExtractor.Chain, 3)

	// Verify individual extractors in chain
	require.Equal(t, SourceHeader, chainExtractor.Chain[0].Source)
	require.Equal(t, "X-Token", chainExtractor.Chain[0].Key)
	require.Equal(t, SourceQuery, chainExtractor.Chain[1].Source)
	require.Equal(t, "token", chainExtractor.Chain[1].Key)
	require.Equal(t, SourceCookie, chainExtractor.Chain[2].Source)
	require.Equal(t, "auth", chainExtractor.Chain[2].Key)
}

// go test -run Test_Extractor_Custom
func Test_Extractor_Custom(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Custom extractor that extracts from a custom header
	customExtractor := Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			token := c.Get("X-Custom-Auth")
			if token == "" {
				return "", ErrNotFound
			}
			return token, nil
		},
		Key:    "X-Custom-Auth",
		Source: SourceCustom,
	}

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set("X-Custom-Auth", "custom-token")

	token, err := customExtractor.Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "custom-token", token)
}

// go test -run Test_Extractor_Chain_Error_Propagation
func Test_Extractor_Chain_Error_Propagation(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Create extractors that return different errors
	errorExtractor1 := Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "", fiber.NewError(fiber.StatusBadRequest, "First error")
		},
		Key:    "error1",
		Source: SourceCustom,
	}

	errorExtractor2 := Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "", fiber.NewError(fiber.StatusUnauthorized, "Second error")
		},
		Key:    "error2",
		Source: SourceCustom,
	}

	chainExtractor := Chain(errorExtractor1, errorExtractor2)

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	token, err := chainExtractor.Extract(ctx)
	require.Empty(t, token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Second error") // Should return the last error
	var fe *fiber.Error
	require.ErrorAs(t, err, &fe)
	require.Equal(t, fiber.StatusUnauthorized, fe.Code)
}

// go test -run Test_Extractor_Chain_With_Success
func Test_Extractor_Chain_With_Success(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// First extractor fails, second succeeds
	failingExtractor := Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "", ErrNotFound
		},
		Key:    "fail",
		Source: SourceCustom,
	}

	successExtractor := Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "success-token", nil
		},
		Key:    "success",
		Source: SourceCustom,
	}

	chainExtractor := Chain(failingExtractor, successExtractor)

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	token, err := chainExtractor.Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "success-token", token)
}

// go test -run Test_Extractor_FromAuthHeader_CustomScheme
func Test_Extractor_FromAuthHeader_CustomScheme(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test with custom auth scheme
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set(fiber.HeaderAuthorization, "CustomScheme my-token")

	extractor := FromAuthHeader("CustomScheme")
	token, err := extractor.Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "my-token", token)

	// Verify metadata
	require.Equal(t, SourceAuthHeader, extractor.Source)
	require.Equal(t, fiber.HeaderAuthorization, extractor.Key)
	require.Equal(t, "CustomScheme", extractor.AuthScheme)
}

// go test -run Test_Extractor_FromAuthHeader_NoScheme
func Test_Extractor_FromAuthHeader_NoScheme(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test with no auth scheme (empty string)
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set(fiber.HeaderAuthorization, "just-the-token")

	extractor := FromAuthHeader("") // No scheme
	token, err := extractor.Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "just-the-token", token)

	// Verify metadata
	require.Equal(t, SourceAuthHeader, extractor.Source)
	require.Equal(t, fiber.HeaderAuthorization, extractor.Key)
	require.Equal(t, "", extractor.AuthScheme)
}
