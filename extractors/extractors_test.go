package extractors

import (
	"context"
	"net/http"
	"strings"
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
	ctx1 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx1)
	ctx1.Request().Header.SetContentType(fiber.MIMEApplicationForm)
	ctx1.Request().Header.SetMethod(fiber.MethodPost)
	ctx1.Request().SetBodyString("token=token_from_form")
	token, err := FromForm("token").Extract(ctx1)
	require.NoError(t, err)
	require.Equal(t, "token_from_form", token)

	// FromQuery
	ctx2 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx2)
	ctx2.Request().SetRequestURI("/?token=token_from_query")
	token, err = FromQuery("token").Extract(ctx2)
	require.NoError(t, err)
	require.Equal(t, "token_from_query", token)

	// FromHeader
	ctx3 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx3)
	ctx3.Request().Header.Set("X-Token", "token_from_header")
	token, err = FromHeader("X-Token").Extract(ctx3)
	require.NoError(t, err)
	require.Equal(t, "token_from_header", token)

	// FromAuthHeader
	ctx4 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx4)
	ctx4.Request().Header.Set(fiber.HeaderAuthorization, "Bearer token_from_auth_header")
	token, err = FromAuthHeader("Bearer").Extract(ctx4)
	require.NoError(t, err)
	require.Equal(t, "token_from_auth_header", token)

	// FromCookie
	ctx5 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx5)
	ctx5.Request().Header.SetCookie("token", "token_from_cookie")
	token, err = FromCookie("token").Extract(ctx5)
	require.NoError(t, err)
	require.Equal(t, "token_from_cookie", token)
}

// go test -run Test_Extractor_Chain
func Test_Extractor_Chain(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// No extractors
	ctx1 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx1)
	token, err := Chain().Extract(ctx1)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// First extractor succeeds
	ctx2 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx2)
	ctx2.Request().Header.Set("X-Token", "token_from_header")
	ctx2.Request().SetRequestURI("/?token=token_from_query")
	token, err = Chain(FromHeader("X-Token"), FromQuery("token")).Extract(ctx2)
	require.NoError(t, err)
	require.Equal(t, "token_from_header", token)

	// Second extractor succeeds
	ctx3 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx3)
	ctx3.Request().SetRequestURI("/?token=token_from_query")
	token, err = Chain(FromHeader("X-Token"), FromQuery("token")).Extract(ctx3)
	require.NoError(t, err)
	require.Equal(t, "token_from_query", token)

	// All extractors fail, should return the last error
	ctx4 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx4)
	token, err = Chain(FromHeader("X-Token"), FromQuery("token")).Extract(ctx4)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// All extractors find nothing (return empty string and nil error), should return ErrNotFound
	ctx5 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx5)
	// This extractor will return "", nil
	dummyExtractor := Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "", nil
		},
		Source: SourceCustom,
		Key:    "token",
	}
	token, err = Chain(dummyExtractor).Extract(ctx5)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)
}

// go test -run Test_Extractor_FromAuthHeader_EdgeCases
func Test_Extractor_FromAuthHeader_EdgeCases(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test case: Authorization header exists but doesn't match the expected scheme
	ctx1 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx1)
	ctx1.Request().Header.Set(fiber.HeaderAuthorization, "Basic dXNlcjpwYXNz") // Basic auth instead of Bearer
	token, err := FromAuthHeader("Bearer").Extract(ctx1)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// Test case: Authorization header exists but has wrong format
	ctx2 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx2)
	ctx2.Request().Header.Set(fiber.HeaderAuthorization, "Bearertoken") // Missing space after Bearer
	token, err = FromAuthHeader("Bearer").Extract(ctx2)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// Test case: Authorization header exists but scheme doesn't match case-insensitively
	ctx3 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx3)
	ctx3.Request().Header.Set(fiber.HeaderAuthorization, "bearer token") // lowercase bearer
	token, err = FromAuthHeader("Bearer").Extract(ctx3)
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

// go test -run Test_Extractor_FromCustom
func Test_Extractor_FromCustom(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test successful extraction with FromCustom
	ctx1 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx1)
	ctx1.Request().Header.Set("X-Custom", "custom-value")

	customExtractor := FromCustom("X-Custom", func(c fiber.Ctx) (string, error) {
		value := c.Get("X-Custom")
		if value == "" {
			return "", ErrNotFound
		}
		return strings.ToUpper(value), nil
	})

	token, err := customExtractor.Extract(ctx1)
	require.NoError(t, err)
	require.Equal(t, "CUSTOM-VALUE", token)

	// Verify metadata
	require.Equal(t, SourceCustom, customExtractor.Source)
	require.Equal(t, "X-Custom", customExtractor.Key)
	require.Equal(t, "", customExtractor.AuthScheme)

	// Test FromCustom with error
	ctx2 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx2)

	errorExtractor := FromCustom("test", func(_ fiber.Ctx) (string, error) {
		return "", fiber.NewError(fiber.StatusBadRequest, "Custom error")
	})

	token, err = errorExtractor.Extract(ctx2)
	require.Empty(t, token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Custom error")

	// Test FromCustom returning empty string
	ctx3 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx3)

	emptyExtractor := FromCustom("empty", func(_ fiber.Ctx) (string, error) {
		return "", nil
	})

	token, err = emptyExtractor.Extract(ctx3)
	require.Empty(t, token)
	require.NoError(t, err) // Should return empty string with no error

	// Test FromCustom with nil function
	nilExtractor := FromCustom("nil", nil)

	token, err = nilExtractor.Extract(ctx3)
	require.Empty(t, token)
	require.ErrorIs(t, err, ErrNotFound) // Should return ErrNotFound for nil function
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

// go test -run Test_Extractor_FromAuthHeader_WhitespaceToken
func Test_Extractor_FromAuthHeader_WhitespaceToken(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test with token containing whitespace (should be preserved)
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set(fiber.HeaderAuthorization, "Bearer token with spaces and\ttabs")

	extractor := FromAuthHeader("Bearer")
	token, err := extractor.Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "token with spaces and\ttabs", token)

	// Verify metadata
	require.Equal(t, SourceAuthHeader, extractor.Source)
	require.Equal(t, fiber.HeaderAuthorization, extractor.Key)
	require.Equal(t, "Bearer", extractor.AuthScheme)
}

// go test -run Test_Extractor_FromAuthHeader_RFC_Compliance
func Test_Extractor_FromAuthHeader_RFC_Compliance(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test RFC 7235: Tab character after scheme (should be accepted)
	ctx1 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx1)
	ctx1.Request().Header.Set(fiber.HeaderAuthorization, "Bearer\ttoken") // Tab after Bearer
	token, err := FromAuthHeader("Bearer").Extract(ctx1)
	require.NoError(t, err)
	require.Equal(t, "token", token)

	// Test RFC 7235: Multiple spaces after scheme
	ctx2 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx2)
	ctx2.Request().Header.Set(fiber.HeaderAuthorization, "Bearer  token") // Multiple spaces
	token, err = FromAuthHeader("Bearer").Extract(ctx2)
	require.NoError(t, err)
	require.Equal(t, "token", token)

	// Test RFC 7235: Mixed whitespace after scheme
	ctx3 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx3)
	ctx3.Request().Header.Set(fiber.HeaderAuthorization, "Bearer \t \ttoken") // Space + tabs
	token, err = FromAuthHeader("Bearer").Extract(ctx3)
	require.NoError(t, err)
	require.Equal(t, "token", token)

	// Test RFC 7235: No whitespace after scheme (should fail)
	ctx4 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx4)
	ctx4.Request().Header.Set(fiber.HeaderAuthorization, "Bearertoken") // No space
	token, err = FromAuthHeader("Bearer").Extract(ctx4)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// Test RFC 7235: Header too short for scheme + space + token
	ctx5 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx5)
	ctx5.Request().Header.Set(fiber.HeaderAuthorization, "Bearer") // Just scheme, no space or token
	token, err = FromAuthHeader("Bearer").Extract(ctx5)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// Test RFC 7235: Only whitespace after scheme (should fail)
	ctx6 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx6)
	ctx6.Request().Header.Set(fiber.HeaderAuthorization, "Bearer   \t  ") // Only whitespace
	token, err = FromAuthHeader("Bearer").Extract(ctx6)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// Test RFC 7235: Case-insensitive scheme matching
	ctx7 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx7)
	ctx7.Request().Header.Set(fiber.HeaderAuthorization, "BEARER token") // Uppercase
	token, err = FromAuthHeader("bearer").Extract(ctx7)
	require.NoError(t, err)
	require.Equal(t, "token", token)
}

// go test -run Test_Extractor_FromAuthHeader_NoScheme
func Test_Extractor_FromAuthHeader_NoScheme(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test with no auth scheme (empty string) - should return trimmed header value
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set(fiber.HeaderAuthorization, "  some-token-value  ")

	extractor := FromAuthHeader("") // No scheme
	token, err := extractor.Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "some-token-value", token)

	// Verify metadata
	require.Equal(t, SourceAuthHeader, extractor.Source)
	require.Equal(t, fiber.HeaderAuthorization, extractor.Key)
	require.Equal(t, "", extractor.AuthScheme)
}

// go test -run Test_Extractor_EdgeCases
func Test_Extractor_EdgeCases(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test empty/whitespace-only values
	ctx1 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx1)
	ctx1.Request().Header.Set("X-Empty", "   \t   ") // Only whitespace

	token, err := FromHeader("X-Empty").Extract(ctx1)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// Test cookie with only whitespace
	ctx2 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx2)
	ctx2.Request().Header.SetCookie("empty", "   ")

	token, err = FromCookie("empty").Extract(ctx2)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// Test query param with only whitespace
	ctx3 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx3)
	ctx3.Request().SetRequestURI("/?param=%20%20%20") // URL-encoded spaces

	token, err = FromQuery("param").Extract(ctx3)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)

	// Test form field with only whitespace
	ctx4 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx4)
	ctx4.Request().Header.SetContentType(fiber.MIMEApplicationForm)
	ctx4.Request().SetBodyString("field=%20%20%20")

	token, err = FromForm("field").Extract(ctx4)
	require.Empty(t, token)
	require.Equal(t, ErrNotFound, err)
}

// go test -run Test_Extractor_Chain_NilFunctions
func Test_Extractor_Chain_NilFunctions(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test chain with nil extractor functions
	nilExtractor := Extractor{
		Extract: nil,
		Key:     "nil",
		Source:  SourceCustom,
	}

	validExtractor := Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "valid-token", nil
		},
		Key:    "valid",
		Source: SourceCustom,
	}

	chainExtractor := Chain(nilExtractor, validExtractor)

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	token, err := chainExtractor.Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "valid-token", token)
}

// go test -run Test_Extractor_Chain_AllErrors
func Test_Extractor_Chain_AllErrors(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test chain where all extractors return errors
	errorExtractor1 := Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "", fiber.NewError(fiber.StatusUnauthorized, "First auth error")
		},
		Key:    "error1",
		Source: SourceCustom,
	}

	errorExtractor2 := Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "", fiber.NewError(fiber.StatusForbidden, "Second auth error")
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
	require.Contains(t, err.Error(), "Second auth error") // Should return last error

	var fe *fiber.Error
	require.ErrorAs(t, err, &fe)
	require.Equal(t, fiber.StatusForbidden, fe.Code)
}

// go test -run Test_Extractor_Chain_MixedScenarios
func Test_Extractor_Chain_MixedScenarios(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test chain with mixed success/error scenarios
	failingExtractor := Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "", ErrNotFound
		},
		Key:    "fail",
		Source: SourceCustom,
	}

	errorExtractor := Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "", fiber.NewError(fiber.StatusBadRequest, "Bad request")
		},
		Key:    "error",
		Source: SourceCustom,
	}

	successExtractor := Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "success", nil
		},
		Key:    "success",
		Source: SourceCustom,
	}

	// Test: error then success (should return success)
	chain1 := Chain(errorExtractor, successExtractor)
	ctx1 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx1)

	token, err := chain1.Extract(ctx1)
	require.NoError(t, err)
	require.Equal(t, "success", token)

	// Test: fail then error then success (should return success)
	chain2 := Chain(failingExtractor, errorExtractor, successExtractor)
	ctx2 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx2)

	token, err = chain2.Extract(ctx2)
	require.NoError(t, err)
	require.Equal(t, "success", token)

	// Test: fail then error (should return last error)
	chain3 := Chain(failingExtractor, errorExtractor)
	ctx3 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx3)

	token, err = chain3.Extract(ctx3)
	require.Empty(t, token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Bad request")
}

// go test -run Test_Extractor_SourceTypes
func Test_Extractor_SourceTypes(t *testing.T) {
	t.Parallel()

	// Test that all source types are properly set
	require.Equal(t, SourceHeader, FromHeader("test").Source)
	require.Equal(t, SourceAuthHeader, FromAuthHeader("Bearer").Source)
	require.Equal(t, SourceAuthHeader, FromAuthHeader("").Source) // Empty scheme should still be SourceAuthHeader
	require.Equal(t, SourceForm, FromForm("test").Source)
	require.Equal(t, SourceQuery, FromQuery("test").Source)
	require.Equal(t, SourceParam, FromParam("test").Source)
	require.Equal(t, SourceCookie, FromCookie("test").Source)
	require.Equal(t, SourceCustom, FromCustom("test", func(_ fiber.Ctx) (string, error) { return "test", nil }).Source)

	// Test chain source (should use first extractor's source)
	chain := Chain(FromHeader("X-Test"), FromQuery("test"))
	require.Equal(t, SourceHeader, chain.Source)
	require.Equal(t, "X-Test", chain.Key)
}
