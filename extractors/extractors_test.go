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
		require.ErrorIs(t, err, ErrNotFound)
		return nil
	})
	_, err := app.Test(newRequest(fiber.MethodGet, "/test"))
	require.NoError(t, err)

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(ctx) })

	// Missing form
	token, err := FromForm("token").Extract(ctx)
	require.Empty(t, token)
	require.ErrorIs(t, err, ErrNotFound)

	// Missing query
	token, err = FromQuery("token").Extract(ctx)
	require.Empty(t, token)
	require.ErrorIs(t, err, ErrNotFound)

	// Missing header
	token, err = FromHeader("X-Token").Extract(ctx)
	require.Empty(t, token)
	require.ErrorIs(t, err, ErrNotFound)

	// Missing Auth header
	token, err = FromAuthHeader("Bearer").Extract(ctx)
	require.Empty(t, token)
	require.ErrorIs(t, err, ErrNotFound)

	// Missing cookie
	token, err = FromCookie("token").Extract(ctx)
	require.Empty(t, token)
	require.ErrorIs(t, err, ErrNotFound)
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

	t.Run("FromParam", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Get("/test/:token", func(c fiber.Ctx) error {
			token, err := FromParam("token").Extract(c)
			require.NoError(t, err)
			require.Equal(t, "token_from_param", token)
			return nil
		})
		_, err := app.Test(newRequest(fiber.MethodGet, "/test/token_from_param"))
		require.NoError(t, err)
	})

	t.Run("FromForm", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.SetContentType(fiber.MIMEApplicationForm)
		ctx.Request().Header.SetMethod(fiber.MethodPost)
		ctx.Request().SetBodyString("token=token_from_form")
		token, err := FromForm("token").Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "token_from_form", token)
	})

	t.Run("FromQuery", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().SetRequestURI("/?token=token_from_query")
		token, err := FromQuery("token").Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "token_from_query", token)
	})

	t.Run("FromHeader", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.Set("X-Token", "token_from_header")
		token, err := FromHeader("X-Token").Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "token_from_header", token)
	})

	t.Run("FromAuthHeader", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.Set(fiber.HeaderAuthorization, "Bearer token_from_auth_header")
		token, err := FromAuthHeader("Bearer").Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "token_from_auth_header", token)
	})

	t.Run("FromCookie", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.SetCookie("token", "token_from_cookie")
		token, err := FromCookie("token").Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "token_from_cookie", token)
	})
}

// go test -run Test_Extractor_Chain
func Test_Extractor_Chain(t *testing.T) {
	t.Parallel()

	t.Run("no_extractors", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		token, err := Chain().Extract(ctx)
		require.Empty(t, token)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("first_extractor_succeeds", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.Set("X-Token", "token_from_header")
		ctx.Request().SetRequestURI("/?token=token_from_query")
		token, err := Chain(FromHeader("X-Token"), FromQuery("token")).Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "token_from_header", token)
	})

	t.Run("second_extractor_succeeds", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().SetRequestURI("/?token=token_from_query")
		token, err := Chain(FromHeader("X-Token"), FromQuery("token")).Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "token_from_query", token)
	})

	t.Run("all_extractors_fail", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		token, err := Chain(FromHeader("X-Token"), FromQuery("token")).Extract(ctx)
		require.Empty(t, token)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("empty_extractor_returns_not_found", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		// This extractor will return "", nil
		dummyExtractor := Extractor{
			Extract: func(_ fiber.Ctx) (string, error) {
				return "", nil
			},
			Source: SourceCustom,
			Key:    "token",
		}
		token, err := Chain(dummyExtractor).Extract(ctx)
		require.Empty(t, token)
		require.ErrorIs(t, err, ErrNotFound)
	})
}

// go test -run Test_Extractor_FromAuthHeader_EdgeCases
func Test_Extractor_FromAuthHeader_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("wrong_scheme", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.Set(fiber.HeaderAuthorization, "Basic dXNlcjpwYXNz") // Basic auth instead of Bearer
		token, err := FromAuthHeader("Bearer").Extract(ctx)
		require.Empty(t, token)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("missing_space_after_scheme", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.Set(fiber.HeaderAuthorization, "Bearertoken") // Missing space after Bearer
		token, err := FromAuthHeader("Bearer").Extract(ctx)
		require.Empty(t, token)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("case_insensitive_scheme_matching", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.Set(fiber.HeaderAuthorization, "bearer token") // lowercase bearer
		token, err := FromAuthHeader("Bearer").Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "token", token)
	})
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

	t.Run("successful_extraction", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.Set("X-Custom", "custom-value")

		customExtractor := FromCustom("X-Custom", func(c fiber.Ctx) (string, error) {
			value := c.Get("X-Custom")
			if value == "" {
				return "", ErrNotFound
			}
			return strings.ToUpper(value), nil
		})

		token, err := customExtractor.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "CUSTOM-VALUE", token)

		// Verify metadata
		require.Equal(t, SourceCustom, customExtractor.Source)
		require.Equal(t, "X-Custom", customExtractor.Key)
		require.Equal(t, "", customExtractor.AuthScheme)
	})

	t.Run("extraction_with_error", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		errorExtractor := FromCustom("test", func(_ fiber.Ctx) (string, error) {
			return "", fiber.NewError(fiber.StatusBadRequest, "Custom error")
		})

		token, err := errorExtractor.Extract(ctx)
		require.Empty(t, token)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Custom error")
	})

	t.Run("extraction_returning_empty_string", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		emptyExtractor := FromCustom("empty", func(_ fiber.Ctx) (string, error) {
			return "", nil
		})

		token, err := emptyExtractor.Extract(ctx)
		require.Empty(t, token)
		require.NoError(t, err) // Should return empty string with no error
	})

	t.Run("nil_function", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		nilExtractor := FromCustom("nil", nil)

		token, err := nilExtractor.Extract(ctx)
		require.Empty(t, token)
		require.ErrorIs(t, err, ErrNotFound) // Should return ErrNotFound for nil function
	})
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
	t.Cleanup(func() { app.ReleaseCtx(ctx) })

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
	t.Cleanup(func() { app.ReleaseCtx(ctx) })

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
	t.Cleanup(func() { app.ReleaseCtx(ctx) })
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

	// Test with token containing whitespace (should be rejected per RFC 7235 token68 spec)
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(ctx) })
	ctx.Request().Header.Set(fiber.HeaderAuthorization, "Bearer token with spaces and\ttabs")

	extractor := FromAuthHeader("Bearer")
	token, err := extractor.Extract(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotFound)
	require.Empty(t, token)

	// Verify metadata
	require.Equal(t, SourceAuthHeader, extractor.Source)
	require.Equal(t, fiber.HeaderAuthorization, extractor.Key)
	require.Equal(t, "Bearer", extractor.AuthScheme)
}

// go test -run Test_Extractor_FromAuthHeader_RFC_Compliance
func Test_Extractor_FromAuthHeader_RFC_Compliance(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		header        string
		expectedToken string
		description   string
		shouldFail    bool
	}{
		{
			name:        "tab_after_scheme",
			header:      "Bearer\ttoken",
			shouldFail:  true,
			description: "tab character after scheme should be rejected - RFC specifies 1*SP, not tabs",
		},
		{
			name:          "single_space_after_scheme",
			header:        "Bearer token",
			shouldFail:    false,
			expectedToken: "token",
			description:   "single space after scheme should be accepted - standard format",
		},
		{
			name:        "multiple_spaces_after_scheme",
			header:      "Bearer  token",
			shouldFail:  true,
			description: "multiple spaces after scheme rejected for simplicity - single space is standard",
		},
		{
			name:        "mixed_whitespace_after_scheme",
			header:      "Bearer \t \ttoken",
			shouldFail:  true,
			description: "mixed whitespace after scheme should be rejected - RFC specifies 1*SP, not tabs",
		},
		{
			name:        "no_whitespace_after_scheme",
			header:      "Bearertoken",
			shouldFail:  true,
			description: "no whitespace after scheme should fail",
		},
		{
			name:        "header_too_short",
			header:      "Bearer",
			shouldFail:  true,
			description: "header too short for scheme + space + token",
		},
		{
			name:        "only_whitespace_after_scheme",
			header:      "Bearer   \t  ",
			shouldFail:  true,
			description: "only whitespace after scheme should fail",
		},
		{
			name:          "case_insensitive_scheme",
			header:        "BEARER token",
			shouldFail:    false,
			expectedToken: "token",
			description:   "case-insensitive scheme matching should work",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
			t.Cleanup(func() { app.ReleaseCtx(ctx) })

			ctx.Request().Header.Set(fiber.HeaderAuthorization, tc.header)
			token, err := FromAuthHeader("Bearer").Extract(ctx)

			if tc.shouldFail {
				require.Error(t, err, "Expected error for %s", tc.description)
				require.ErrorIs(t, err, ErrNotFound)
				require.Empty(t, token)
			} else {
				require.NoError(t, err, "Expected no error for %s", tc.description)
				require.Equal(t, tc.expectedToken, token)
			}
		})
	}

	// Special case for case-insensitive scheme matching with different extractor scheme
	t.Run("case_insensitive_extractor_scheme", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		ctx.Request().Header.Set(fiber.HeaderAuthorization, "BEARER token")
		token, err := FromAuthHeader("bearer").Extract(ctx) // lowercase extractor scheme
		require.NoError(t, err)
		require.Equal(t, "token", token)
	})
}

// go test -run Test_Extractor_FromAuthHeader_Token68_Validation
func Test_Extractor_FromAuthHeader_Token68_Validation(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test valid token68 characters (should pass)
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(ctx) })
	ctx.Request().Header.Set(fiber.HeaderAuthorization, "Bearer ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~+/=")
	token, err := FromAuthHeader("Bearer").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~+/=", token)

	// Test tokens with spaces (should fail)
	testCases := []struct {
		name        string
		header      string
		description string
		shouldFail  bool
	}{
		{name: "space_in_token", header: "Bearer abc def", shouldFail: true, description: "space in token"},
		{name: "space_after_scheme", header: "Bearer  abc", shouldFail: true, description: "multiple spaces after scheme"},
		{name: "no_space_after_scheme", header: "Bearertoken", shouldFail: true, description: "no space after scheme"},
		{name: "only_scheme", header: "Bearer", shouldFail: true, description: "only scheme, no token"},
		{name: "tab_after_scheme", header: "Bearer\ttoken", shouldFail: true, description: "tab after scheme"},
		{name: "tab_in_token", header: "Bearer abc\tdef", shouldFail: true, description: "tab in token"},
		{name: "newline_in_token", header: "Bearer abc\ndef", shouldFail: true, description: "newline in token"},
		{name: "leading_space_in_token", header: "Bearer  abc", shouldFail: true, description: "leading space in token after scheme space"},
		{name: "trailing_space_in_token", header: "Bearer abc ", shouldFail: true, description: "trailing space in token"},
		{name: "comma_in_token", header: "Bearer abc,def", shouldFail: true, description: "comma in token"},
		{name: "semicolon_in_token", header: "Bearer abc;def", shouldFail: true, description: "semicolon in token"},
		{name: "quote_in_token", header: "Bearer abc\"def", shouldFail: true, description: "quote in token"},
		{name: "bracket_in_token", header: "Bearer abc[def", shouldFail: true, description: "bracket in token"},
		{name: "equals_at_start", header: "Bearer =abc", shouldFail: true, description: "equals at start of token"},
		{name: "equals_in_middle", header: "Bearer ab=cd", shouldFail: true, description: "equals in middle of token"},
		{name: "valid_equals_at_end", header: "Bearer abc=", shouldFail: false, description: "valid equals at end"},
		{name: "valid_double_equals", header: "Bearer abc==", shouldFail: false, description: "valid double equals at end"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
			t.Cleanup(func() { app.ReleaseCtx(ctx) })
			ctx.Request().Header.Set(fiber.HeaderAuthorization, tc.header)

			token, err := FromAuthHeader("Bearer").Extract(ctx)

			if tc.shouldFail {
				require.Error(t, err, "Expected error for %s", tc.description)
				require.ErrorIs(t, err, ErrNotFound)
				require.Empty(t, token)
			} else {
				require.NoError(t, err, "Expected no error for %s", tc.description)
				require.NotEmpty(t, token)
			}
		})
	}
}

// go test -run Test_Extractor_FromAuthHeader_NoScheme
func Test_Extractor_FromAuthHeader_NoScheme(t *testing.T) {
	t.Parallel()

	t.Run("returns_header_value", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.Set(fiber.HeaderAuthorization, "some-token-value")

		extractor := FromAuthHeader("") // No scheme
		token, err := extractor.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "some-token-value", token)

		// Verify metadata
		require.Equal(t, SourceAuthHeader, extractor.Source)
		require.Equal(t, fiber.HeaderAuthorization, extractor.Key)
		require.Equal(t, "", extractor.AuthScheme)
	})

	t.Run("empty_header_returns_not_found", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		// No Authorization header set

		extractor := FromAuthHeader("") // No scheme
		token, err := extractor.Extract(ctx)
		require.Empty(t, token)
		require.ErrorIs(t, err, ErrNotFound)
	})
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
	t.Cleanup(func() { app.ReleaseCtx(ctx) })

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
	t.Cleanup(func() { app.ReleaseCtx(ctx) })

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

	// Define reusable extractors
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

	t.Run("error_then_success", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		chain := Chain(errorExtractor, successExtractor)
		token, err := chain.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "success", token)
	})

	t.Run("fail_then_error_then_success", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		chain := Chain(failingExtractor, errorExtractor, successExtractor)
		token, err := chain.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "success", token)
	})

	t.Run("fail_then_error", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		chain := Chain(failingExtractor, errorExtractor)
		token, err := chain.Extract(ctx)
		require.Empty(t, token)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Bad request")
	})
}

// go test -run Test_Extractor_SourceTypes
func Test_Extractor_SourceTypes(t *testing.T) {
	t.Parallel()

	t.Run("individual_extractor_sources", func(t *testing.T) {
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
	})

	t.Run("chain_source_metadata", func(t *testing.T) {
		t.Parallel()

		// Test chain source (should use first extractor's source)
		chain := Chain(FromHeader("X-Test"), FromQuery("test"))
		require.Equal(t, SourceHeader, chain.Source)
		require.Equal(t, "X-Test", chain.Key)
	})
}

// go test -run Test_Extractor_URL_Encoded
func Test_Extractor_URL_Encoded(t *testing.T) {
	t.Parallel()

	t.Run("FromQuery_with_spaces", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().SetRequestURI("/?token=token%20with%20spaces")
		token, err := FromQuery("token").Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "token with spaces", token) // Should be URL-decoded automatically by fasthttp
	})

	t.Run("FromForm_with_plus", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.SetContentType(fiber.MIMEApplicationForm)
		ctx.Request().Header.SetMethod(fiber.MethodPost)
		ctx.Request().SetBodyString("token=token%2Bwith%2Bplus")
		token, err := FromForm("token").Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "token+with+plus", token) // URL-decoded
	})

	t.Run("FromQuery_base64_encoded", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		base64Value := "cGFzc3dvcmQ%3D" // URL-encoded base64 "cGFzc3dvcmQ="
		ctx.Request().SetRequestURI("/?token=" + base64Value)
		token, err := FromQuery("token").Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "cGFzc3dvcmQ=", token) // Should be URL-decoded
	})

	t.Run("FromParam_with_slashes", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Get("/test/:token", func(c fiber.Ctx) error {
			token, extractErr := FromParam("token").Extract(c)
			require.NoError(t, extractErr)
			require.Equal(t, "token/with/slashes", token)
			return nil
		})
		_, err := app.Test(newRequest(fiber.MethodGet, "/test/token%2Fwith%2Fslashes"))
		require.NoError(t, err)
	})
}

func Test_isValidToken68(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		token string
		want  bool
	}{
		{name: "empty string", token: "", want: false},
		{name: "single uppercase", token: "A", want: true},
		{name: "single lowercase", token: "a", want: true},
		{name: "single digit", token: "0", want: true},
		{name: "all allowed symbols except =", token: "-._~+/", want: true},
		{name: "letters and digits", token: "token68", want: true},
		{name: "equals at end", token: "token=", want: true},
		{name: "multiple equals", token: "token==", want: true},
		{name: "equals at start", token: "=token", want: false},
		{name: "equals in middle", token: "tok=en", want: false},
		{name: "equals not at end with other chars", token: "token=extra", want: false},
		{name: "space in token", token: "token space", want: false},
		{name: "tab character in token", token: "token\ttab", want: false},
		{name: "invalid symbol", token: "token@", want: false},
		{name: "valid token68", token: "token68", want: true},
		{token: "token68=", want: true, name: "valid token68 with equals at end"},
		{token: "token68==", want: true, name: "multiple equals at end"},
		{token: "token68=extra", want: false, name: "equals followed by extra chars"},
		{token: "T0ken-._~+/=", want: true, name: "all allowed chars with equals at end"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isValidToken68(tc.token)
			if got != tc.want {
				t.Errorf("isValidToken68(%q) = %v, want %v", tc.token, got, tc.want)
			}
		})
	}
}
