package extractors

import (
	"bufio"
	"context"
	"errors"
	"fmt"
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
	_, err := app.Test(newRequest("/test"))
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

// newRequest creates a new GET *http.Request for Fiber's app.Test.
func newRequest(target string) *http.Request {
	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, target, http.NoBody)
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
		_, err := app.Test(newRequest("/test/token_from_param"))
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

// Test_Extractor_FromHeader_CombinesRepeatedLines covers a field carried on
// several lines. FromHeader is given its name by the application, and a name it
// is given may be a list field a peer may legally send twice, so the lines are
// combined the way RFC 9110 §5.3 says a recipient may rather than refused as an
// ambiguous single value.
func Test_Extractor_FromHeader_CombinesRepeatedLines(t *testing.T) {
	t.Parallel()

	for _, normalize := range []bool{true, false} {
		t.Run(fmt.Sprintf("normalize=%v", normalize), func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{DisableHeaderNormalizing: !normalize})
			ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(ctx)
			if !normalize {
				ctx.Request().Header.DisableNormalizing()
			}

			ctx.Request().Header.Add("Accept", "text/html")
			// The second spelling is the one HTTP/2 and 3 put on the wire.
			ctx.Request().Header.Add("accept", "application/json")

			value, err := FromHeader("Accept").Extract(ctx)
			require.NoError(t, err)
			require.Equal(t, "text/html, application/json", value)
		})
	}
}

// Test_Extractor_FromHeader_CookieKeepsItsOwnSeparator covers FromHeader named
// with Cookie, whose crumbs are joined with "; " (RFC 6265 §5.4) rather than
// with the comma a list field takes.
//
// fasthttp keeps cookies in a store of their own that PeekAll enumerates one at
// a time, so the generic join produced "a=1, b=2" for a request that said
// "a=1; b=2" — a value no cookie parser downstream would read back.
func Test_Extractor_FromHeader_CookieKeepsItsOwnSeparator(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// Read from the wire rather than Set: a request carrying Cookie twice, which
	// is what an HTTP/1 gateway writes for the split field HTTP/2 permits. Only
	// then does fasthttp hand the crumbs back one at a time.
	raw := "GET / HTTP/1.1\r\nHost: example.com\r\nCookie: a=1\r\nCookie: b=2\r\n\r\n"
	require.NoError(t, ctx.Request().Header.Read(bufio.NewReader(strings.NewReader(raw))))

	value, err := FromHeader(fiber.HeaderCookie).Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "a=1; b=2", value)
}

// Test_Extractor_FromHeader_SingleLineIsUnchanged is the control for the test
// above: one line is returned as it arrived, with no separator introduced.
func Test_Extractor_FromHeader_SingleLineIsUnchanged(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.Set("X-API-Key", "abc123")

	value, err := FromHeader("X-API-Key").Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "abc123", value)
}

// Test_Extractor_FromAuthHeader_RepeatedLineIsNotACredential covers a message
// carrying Authorization twice, which is what a middleware clearing the field
// leaves behind when the client sent another spelling of it.
//
// Under DisableHeaderNormalizing a byte-exact Set writes the canonical name and
// leaves the client's lower-case line — the spelling HTTP/2 and 3 put on the
// wire — in place beside it. Reading either one lets whoever wrote the other
// decide: the first line is the cleared value only where fasthttp happens to
// store it first, and the first non-empty one is always the client's. Neither
// is an answer, so no credential is extracted and the caller refuses.
func Test_Extractor_FromAuthHeader_RepeatedLineIsNotACredential(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	extractor := FromAuthHeader("Bearer")

	// Both orders fasthttp stores, depending on whether the client's line was
	// already under the canonical name when the middleware cleared it.
	for _, order := range [][2][2]string{
		{{"authorization", "Bearer client"}, {"Authorization", ""}},
		{{"Authorization", ""}, {"authorization", "Bearer client"}},
	} {
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		ctx.Request().Header.DisableNormalizing()
		for _, line := range order {
			ctx.Request().Header.Add(line[0], line[1])
		}

		token, err := extractor.Extract(ctx)
		require.ErrorIs(t, err, ErrNotFound,
			"a second Authorization line makes the credential ambiguous, whichever order it is stored in")
		require.Empty(t, token)
		app.ReleaseCtx(ctx)
	}

	// A single line is still extracted, so the assertions above cannot pass by
	// refusing everything.
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.DisableNormalizing()
	ctx.Request().Header.Set("authorization", "Bearer client")

	token, err := extractor.Extract(ctx)
	require.NoError(t, err)
	require.Equal(t, "client", token)
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

func Test_Extractor_Contains(t *testing.T) {
	t.Parallel()

	t.Run("nil_predicate", func(t *testing.T) {
		t.Parallel()

		extractor := FromHeader("X-Token")
		require.False(t, extractor.Contains(nil))
	})

	t.Run("single_extractor_match", func(t *testing.T) {
		t.Parallel()

		extractor := FromCookie("CSRF")
		require.True(t, extractor.Contains(func(e Extractor) bool {
			return e.Source == SourceCookie && e.Key == "CSRF"
		}))
	})

	t.Run("nested_chain_match", func(t *testing.T) {
		t.Parallel()

		inner := Chain(FromHeader("X-Token"), FromCookie("CSRF"))
		outer := Chain(FromQuery("token"), inner)

		require.True(t, outer.Contains(func(e Extractor) bool {
			return e.Source == SourceCookie && e.Key == "CSRF"
		}))
	})

	t.Run("nested_chain_no_match", func(t *testing.T) {
		t.Parallel()

		inner := Chain(FromHeader("X-Token"), FromCookie("session"))
		outer := Chain(FromQuery("token"), inner)

		require.False(t, outer.Contains(func(e Extractor) bool {
			return e.Source == SourceCookie && e.Key == "CSRF"
		}))
	})

	t.Run("cyclic_chain_no_match", func(t *testing.T) {
		t.Parallel()

		extractor := FromHeader("X-Token")
		extractor.Chain = make([]Extractor, 1)
		extractor.Chain[0] = extractor

		require.False(t, extractor.Contains(func(e Extractor) bool {
			return e.Source == SourceCookie && e.Key == "CSRF"
		}))
	})
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
		require.Empty(t, customExtractor.AuthScheme)
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
		require.Empty(t, extractor.AuthScheme)
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

// go test -run Test_Extractor_Chain_Cycle_Prevention
func Test_Extractor_Chain_Cycle_Prevention(t *testing.T) {
	t.Parallel()

	t.Run("direct_cycle_returns_err_chain_cycle", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		var chainExtractor Extractor
		chainExtractor = Chain(FromCustom("cycle", func(c fiber.Ctx) (string, error) {
			return chainExtractor.Extract(c)
		}))

		token, err := chainExtractor.Extract(ctx)
		require.Empty(t, token)
		require.ErrorIs(t, err, ErrChainCycle)
	})

	t.Run("indirect_cycle_returns_err_chain_cycle", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		var chainA, chainB Extractor
		chainA = Chain(FromCustom("to-b", func(c fiber.Ctx) (string, error) {
			return chainB.Extract(c)
		}))
		chainB = Chain(FromCustom("to-a", func(c fiber.Ctx) (string, error) {
			return chainA.Extract(c)
		}))

		token, err := chainA.Extract(ctx)
		require.Empty(t, token)
		require.ErrorIs(t, err, ErrChainCycle)
	})

	t.Run("nested_non_cyclic_chain_still_works", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.Set("X-Token", "token_from_header")

		inner := Chain(FromHeader("X-Token"), FromQuery("token"))
		outer := Chain(inner, FromCookie("token"))

		token, err := outer.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "token_from_header", token)
	})
}

func Test_Extractor_Chain_ReusedAcrossMiddleware(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	shared := Chain(FromHeader("X-Token"), FromQuery("token"))

	app.Use(func(c fiber.Ctx) error {
		token, err := shared.Extract(c)
		require.NoError(t, err)
		require.Equal(t, "token_from_header", token)
		return c.Next()
	})

	app.Get("/test", func(c fiber.Ctx) error {
		token, err := shared.Extract(c)
		require.NoError(t, err)
		require.Equal(t, "token_from_header", token)
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := newRequest("/test")
	req.Header.Set("X-Token", "token_from_header")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNoContent, resp.StatusCode)
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
		_, err := app.Test(newRequest("/test/token%2Fwith%2Fslashes"))
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
		{token: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9P", want: true, name: "jwt-like long token"},
		{token: "abcdefgh", want: true, name: "8-byte token accepted"},
		{token: "abcdefg,", want: false, name: "trailing comma rejected"},
		{token: "abcdefghijklmno\x00", want: false, name: "NUL rejected"},
		{token: "abcdefgh\xc3\xa9", want: false, name: "non-ascii rejected"},
		{token: "abcdefghijklmnop====", want: true, name: "long token with padding run"},
		{token: "abcdefghijklmnop==x=", want: false, name: "base char inside padding run"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isValidToken68(tc.token)
			if got != tc.want {
				t.Errorf("isValidToken68(%q) = %v, want %v", tc.token, got, tc.want)
			}
		})
	}
}

// go test -v -run=^$ -bench=Benchmark_isValidToken68 -benchmem -count=4
func Benchmark_isValidToken68(b *testing.B) {
	inputs := []string{
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9P", // JWT-like
		"dXNlcjpwYXNzd29yZA==", // short base64 credential
		"token@invalid",        // early reject
	}
	var got bool
	b.ReportAllocs()
	for b.Loop() {
		for _, in := range inputs {
			got = isValidToken68(in)
		}
	}
	_ = got
}

// Test_FromHeader_IgnoresHeaderNameCase pins that a token is found under the
// name it actually arrived as.
//
// Ctx.Get compares the stored key byte for byte, so under
// DisableHeaderNormalizing a token sent under the lower-case name that HTTP/2
// and HTTP/3 put on the wire was not found, and the request refused for
// carrying no token when it carried one.
func Test_FromHeader_IgnoresHeaderNameCase(t *testing.T) {
	t.Parallel()

	for _, normalize := range []bool{true, false} {
		t.Run(fmt.Sprintf("normalize=%v", normalize), func(t *testing.T) {
			t.Parallel()

			for _, sent := range []string{"X-Csrf-Token", "x-csrf-token", "X-CSRF-TOKEN"} {
				app := fiber.New(fiber.Config{DisableHeaderNormalizing: !normalize})
				c := app.AcquireCtx(&fasthttp.RequestCtx{})
				if !normalize {
					c.Request().Header.DisableNormalizing()
				}
				c.Request().Header.Set(sent, "the-token")

				got, err := FromHeader("X-Csrf-Token").Extract(c)
				require.NoError(t, err, "sent as %q", sent)
				require.Equal(t, "the-token", got, "sent as %q", sent)
				app.ReleaseCtx(c)
			}
		})
	}
}

// Test_FromAuthHeader_IgnoresHeaderNameCase pins the same for the Authorization
// reader, which reaches every keyauth and bearer-token caller.
//
// FromHeader was fixed first and this one was missed: a lower-case
// "authorization:" read as absent, so the scheme check never ran and the
// request was refused for carrying no credential when it carried one.
func Test_FromAuthHeader_IgnoresHeaderNameCase(t *testing.T) {
	t.Parallel()

	for _, normalize := range []bool{true, false} {
		t.Run(fmt.Sprintf("normalize=%v", normalize), func(t *testing.T) {
			t.Parallel()

			for _, sent := range []string{"Authorization", "authorization", "AUTHORIZATION"} {
				app := fiber.New(fiber.Config{DisableHeaderNormalizing: !normalize})
				c := app.AcquireCtx(&fasthttp.RequestCtx{})
				if !normalize {
					c.Request().Header.DisableNormalizing()
				}
				c.Request().Header.Set(sent, "Bearer the-token")

				got, err := FromAuthHeader("Bearer").Extract(c)
				require.NoError(t, err, "sent as %q", sent)
				require.Equal(t, "the-token", got, "sent as %q", sent)
				app.ReleaseCtx(c)
			}
		})
	}
}

// go test -run Test_ExtractWithSource
func Test_ExtractWithSource(t *testing.T) {
	t.Parallel()

	t.Run("builtin_cookie_source", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.SetCookie("session", "cookie-value")

		v, src, err := ExtractWithSource(FromCookie("session"), ctx)
		require.NoError(t, err)
		require.Equal(t, "cookie-value", v)
		require.Equal(t, SourceCookie, src)
	})

	t.Run("falls_back_to_Extract_with_static_Source", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		handRolled := Extractor{
			Extract: func(_ fiber.Ctx) (string, error) {
				return "legacy-value", nil
			},
			Source: SourceHeader,
			Key:    "X-Legacy",
		}

		v, src, err := ExtractWithSource(handRolled, ctx)
		require.NoError(t, err)
		require.Equal(t, "legacy-value", v)
		require.Equal(t, SourceHeader, src)
	})

	t.Run("nil_both_returns_ErrNotFound", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		empty := Extractor{Source: SourceCustom, Key: "empty"}
		v, src, err := ExtractWithSource(empty, ctx)
		require.Empty(t, v)
		require.Equal(t, SourceCustom, src)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("builtins_report_static_source_via_ExtractWithSource", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.Set("X-Token", "h")
		ctx.Request().Header.SetCookie("token", "c")
		ctx.Request().SetRequestURI("/?token=q")

		_, src, err := ExtractWithSource(FromHeader("X-Token"), ctx)
		require.NoError(t, err)
		require.Equal(t, SourceHeader, src)
		_, src, err = ExtractWithSource(FromCookie("token"), ctx)
		require.NoError(t, err)
		require.Equal(t, SourceCookie, src)
		_, src, err = ExtractWithSource(FromQuery("token"), ctx)
		require.NoError(t, err)
		require.Equal(t, SourceQuery, src)
		require.NotNil(t, Chain(FromHeader("X-Token")).Extract)
		require.NotNil(t, Chain().Extract)
	})

	t.Run("unkeyed_literal_still_compiles_without_extra_fields", func(t *testing.T) {
		t.Parallel()
		// Positional form must match the public field set exactly. Keeping
		// source-aware extraction field-free preserves this shape.
		fn := func(_ fiber.Ctx) (string, error) { return "v", nil }
		e := Extractor{fn, "k", "", nil, SourceCustom}
		require.NotNil(t, e.Extract)
		require.Equal(t, "k", e.Key)
		require.Equal(t, SourceCustom, e.Source)
	})

	t.Run("honors_legacy_Extract_override", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.Set("X-Token", "raw-header")

		e := FromHeader("X-Token")
		// Callers may decorate Extract for validation/normalization.
		// ExtractWithSource always uses Extract for leaves, so the override wins.
		e.Extract = func(c fiber.Ctx) (string, error) {
			v, err := FromHeader("X-Token").Extract(c)
			if err != nil {
				return "", err
			}
			return "normalized:" + v, nil
		}

		v, src, err := ExtractWithSource(e, ctx)
		require.NoError(t, err)
		require.Equal(t, "normalized:raw-header", v)
		require.Equal(t, SourceHeader, src)
	})

	t.Run("honors_chain_level_Extract_override", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		// Only query is present. An undecorated chain would accept it; a
		// chain-level Extract override that rejects query-sourced values must
		// still win under ExtractWithSource.
		ctx.Request().SetRequestURI("/?token=from-query")

		chain := Chain(FromHeader("X-Token"), FromQuery("token"))
		base := chain.Extract
		rejectQuery := errors.New("query tokens not allowed")
		chain.Extract = func(c fiber.Ctx) (string, error) {
			v, err := base(c)
			if err != nil {
				return "", err
			}
			// Simulate a decorator that only permits header-sourced credentials.
			// The value is present via query; reject it.
			if c.Query("token") == v {
				return "", rejectQuery
			}
			return v, nil
		}

		v, err := chain.Extract(ctx)
		require.Empty(t, v)
		require.ErrorIs(t, err, rejectQuery)

		sv, src, serr := ExtractWithSource(chain, ctx)
		require.Empty(t, sv)
		require.Equal(t, SourceHeader, src) // static first-child metadata on error
		require.ErrorIs(t, serr, rejectQuery)
	})
}

// go test -run Test_Extractor_Chain_ExtractSource
func Test_Extractor_Chain_ExtractSource(t *testing.T) {
	t.Parallel()

	t.Run("reports_winning_child_source", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		// Header missing; query wins. Static chain Source stays SourceHeader,
		// but ExtractSource must report SourceQuery.
		ctx.Request().SetRequestURI("/?token=from-query")

		chain := Chain(FromHeader("X-Token"), FromQuery("token"))
		require.Equal(t, SourceHeader, chain.Source)

		v, src, err := ExtractWithSource(chain, ctx)
		require.NoError(t, err)
		require.Equal(t, "from-query", v)
		require.Equal(t, SourceQuery, src)
	})

	t.Run("first_child_source_when_first_wins", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.Set("X-Token", "from-header")
		ctx.Request().SetRequestURI("/?token=from-query")

		v, src, err := ExtractWithSource(Chain(FromHeader("X-Token"), FromQuery("token")), ctx)
		require.NoError(t, err)
		require.Equal(t, "from-header", v)
		require.Equal(t, SourceHeader, src)
	})

	t.Run("skips_empty_children_preserves_prior_error", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		customErr := errors.New("custom failure")
		customFailure := Extractor{
			Extract: func(_ fiber.Ctx) (string, error) {
				return "", customErr
			},
			Source: SourceCustom,
			Key:    "fail",
		}
		// Zero-value trailing child must not rewrite the chain error to ErrNotFound
		// under ExtractWithSource (Extract already skips nil Extract).
		chain := Chain(customFailure, Extractor{})

		v, err := chain.Extract(ctx)
		require.Empty(t, v)
		require.ErrorIs(t, err, customErr)

		sv, src, serr := ExtractWithSource(chain, ctx)
		require.Empty(t, sv)
		require.Equal(t, SourceCustom, src)
		require.ErrorIs(t, serr, customErr)
	})

	t.Run("hand_rolled_in_chain", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		handRolled := Extractor{
			Extract: func(_ fiber.Ctx) (string, error) {
				return "hand-rolled", nil
			},
			Source: SourceForm,
			Key:    "form-key",
		}
		chain := Chain(FromHeader("X-Token"), handRolled)

		v, src, err := ExtractWithSource(chain, ctx)
		require.NoError(t, err)
		require.Equal(t, "hand-rolled", v)
		require.Equal(t, SourceForm, src)
	})

	t.Run("shared_guard_allows_sequential_Extract_then_ExtractWithSource", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.SetCookie("token", "cookie-token")

		chain := Chain(FromHeader("X-Token"), FromCookie("token"))

		// Guard is cleared on return, so a later entry point on the same
		// request is not treated as a cycle.
		v, err := chain.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "cookie-token", v)

		sv, src, serr := ExtractWithSource(chain, ctx)
		require.NoError(t, serr)
		require.Equal(t, "cookie-token", sv)
		require.Equal(t, SourceCookie, src)
	})

	t.Run("source_path_detects_cycle", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		var chainExtractor Extractor
		chainExtractor = Chain(FromCustom("cycle", func(c fiber.Ctx) (string, error) {
			// Re-enter via ExtractWithSource while Extract is already active.
			_, _, err := ExtractWithSource(chainExtractor, c)
			return "", err
		}))

		// Shared guard must treat the cross-API re-entry as a cycle.
		v, src, err := ExtractWithSource(chainExtractor, ctx)
		require.Empty(t, v)
		require.Equal(t, SourceCustom, src)
		require.ErrorIs(t, err, ErrChainCycle)
	})

	t.Run("cross_api_reentry_via_Extract_detects_cycle", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		// Enter via Extract while a nested child hits ExtractWithSource.
		var chainB Extractor
		chainB = Chain(FromCustom("to-source", func(c fiber.Ctx) (string, error) {
			_, _, err := ExtractWithSource(chainB, c)
			return "", err
		}))
		token, err := chainB.Extract(ctx)
		require.Empty(t, token)
		require.ErrorIs(t, err, ErrChainCycle)
	})

	t.Run("empty_chain_source_path", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		v, src, err := ExtractWithSource(Chain(), ctx)
		require.Empty(t, v)
		require.Equal(t, SourceCustom, src)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("public_Chain_slice_mutation_does_not_affect_Extract", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.Set("X-Token", "from-header")
		ctx.Request().SetRequestURI("/?token=from-query")

		chain := Chain(FromHeader("X-Token"), FromQuery("token"))
		require.Len(t, chain.Chain, 2)

		// Mutating the public introspection slice must not change which
		// children Extract runs (private execution list).
		chain.Chain[0] = FromQuery("token")
		chain.Chain[1] = FromHeader("X-Missing")

		v, err := chain.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "from-header", v)

		// Value and source come from private kids via Extract capture, not
		// a second walk of the mutated public Chain.
		sv, src, serr := ExtractWithSource(chain, ctx)
		require.NoError(t, serr)
		require.Equal(t, "from-header", sv)
		require.Equal(t, SourceHeader, src)
	})

	t.Run("recursive_public_Chain_metadata_uses_captured_source", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.Set("X-Token", "from-header")

		chain := Chain(FromHeader("X-Token"), FromQuery("token"))
		// Corrupt introspection metadata after construction. Extract still
		// succeeds via the private kids list and records the winner so
		// ExtractWithSource does not need to walk the recursive public slice.
		chain.Chain[0] = chain

		v, err := chain.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "from-header", v)

		sv, src, serr := ExtractWithSource(chain, ctx)
		require.NoError(t, serr)
		require.Equal(t, "from-header", sv)
		require.Equal(t, SourceHeader, src)
	})

	t.Run("replaced_Extract_without_base_uses_static_Source", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.SetCookie("session", "cookie-value")
		ctx.Request().SetRequestURI("/?token=from-query")

		// Full Extract replacement (does not call base): must not peek Chain
		// children and mis-tag the replacement value as SourceQuery.
		chain := Chain(FromHeader("X-Missing"), FromQuery("token"))
		chain.Extract = func(c fiber.Ctx) (string, error) {
			return FromCookie("session").Extract(c)
		}

		sv, src, serr := ExtractWithSource(chain, ctx)
		require.NoError(t, serr)
		require.Equal(t, "cookie-value", sv)
		require.Equal(t, SourceHeader, src) // static first-child metadata
	})

	t.Run("shared_guard_blocks_cross_entry_during_source_path", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().SetRequestURI("/?token=from-query")

		// First child re-enters the outer chain via Extract while the chain's
		// Extract is active; second child would succeed if the cycle were ignored.
		var chain Extractor
		chain = Chain(
			FromCustom("reenter", func(c fiber.Ctx) (string, error) {
				return chain.Extract(c)
			}),
			FromQuery("token"),
		)

		sv, src, serr := ExtractWithSource(chain, ctx)
		require.NoError(t, serr)
		require.Equal(t, "from-query", sv)
		// Must not mis-attribute the query value to the custom re-entry child.
		require.Equal(t, SourceQuery, src)
	})

	t.Run("no_double_extract_of_stateful_custom_child", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })

		calls := 0
		chain := Chain(
			FromCustom("once", func(_ fiber.Ctx) (string, error) {
				calls++
				if calls == 1 {
					return "only-once", nil
				}
				return "", ErrNotFound
			}),
			FromQuery("token"),
		)

		sv, src, serr := ExtractWithSource(chain, ctx)
		require.NoError(t, serr)
		require.Equal(t, "only-once", sv)
		require.Equal(t, SourceCustom, src)
		require.Equal(t, 1, calls, "stateful child must not run again during source resolution")
	})

	t.Run("nested_chain_reports_inner_winning_source", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().SetRequestURI("/?token=from-query")

		// Outer chain has one child: an inner chain whose first child misses
		// and second (query) wins. Must report SourceQuery, not the inner
		// chain's static first-child SourceCookie.
		inner := Chain(FromCookie("token"), FromQuery("token"))
		outer := Chain(inner)

		sv, src, serr := ExtractWithSource(outer, ctx)
		require.NoError(t, serr)
		require.Equal(t, "from-query", sv)
		require.Equal(t, SourceQuery, src)
	})

	t.Run("clearing_public_Chain_still_returns_captured_source", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().SetRequestURI("/?token=from-query")

		chain := Chain(FromHeader("X-Token"), FromQuery("token"))
		// Mutating/clearing public metadata must not drop the source captured
		// from the private execution list.
		chain.Chain = nil

		sv, src, serr := ExtractWithSource(chain, ctx)
		require.NoError(t, serr)
		require.Equal(t, "from-query", sv)
		require.Equal(t, SourceQuery, src)
	})

	t.Run("bare_Extract_does_not_pollute_later_ExtractWithSource", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.SetCookie("session", "cookie-value")
		ctx.Request().SetRequestURI("/?token=from-query")

		// Legacy path: Extract alone must not leave a winner that a later
		// source-aware call on an unrelated leaf would mis-attribute.
		chain := Chain(FromHeader("X-Missing"), FromQuery("token"))
		v, err := chain.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "from-query", v)

		sv, src, serr := ExtractWithSource(FromCookie("session"), ctx)
		require.NoError(t, serr)
		require.Equal(t, "cookie-value", sv)
		require.Equal(t, SourceCookie, src)
	})

	t.Run("rejected_nested_chain_does_not_poison_fallback_source", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().Header.SetCookie("session", "cookie-value")
		ctx.Request().SetRequestURI("/?token=from-query")

		// Nested chain would win via cookie, but a decorator rejects after
		// base Extract. Outer fallback is query — source must be SourceQuery,
		// not the rejected nested cookie capture.
		inner := Chain(FromCookie("session"), FromHeader("X-Missing"))
		base := inner.Extract
		reject := errors.New("cookie rejected")
		inner.Extract = func(c fiber.Ctx) (string, error) {
			if _, err := base(c); err != nil {
				return "", err
			}
			return "", reject
		}
		outer := Chain(inner, FromQuery("token"))

		sv, src, serr := ExtractWithSource(outer, ctx)
		require.NoError(t, serr)
		require.Equal(t, "from-query", sv)
		require.Equal(t, SourceQuery, src)
	})

	t.Run("chain_override_success_still_reports_winning_source", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		t.Cleanup(func() { app.ReleaseCtx(ctx) })
		ctx.Request().SetRequestURI("/?token=from-query")

		chain := Chain(FromHeader("X-Token"), FromQuery("token"))
		base := chain.Extract
		chain.Extract = func(c fiber.Ctx) (string, error) {
			v, err := base(c)
			if err != nil {
				return "", err
			}
			return "normalized:" + v, nil
		}

		v, src, err := ExtractWithSource(chain, ctx)
		require.NoError(t, err)
		require.Equal(t, "normalized:from-query", v)
		require.Equal(t, SourceQuery, src)
	})
}

// Test_FromParam_DecodesOnce pins that a parameter is percent-decoded exactly
// once whatever UnescapePath is set to. With UnescapePath enabled the router
// already decoded the path, so a second decode here turned "%2520" into a
// space instead of the "%20" the client sent.
func Test_FromParam_DecodesOnce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		unescapePath bool
	}{
		{name: "UnescapePath disabled", unescapePath: false},
		{name: "UnescapePath enabled", unescapePath: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{UnescapePath: tc.unescapePath})
			app.Get("/test/:token", func(c fiber.Ctx) error {
				token, err := FromParam("token").Extract(c)
				require.NoError(t, err)
				// The client sent "%2520", which stands for the literal
				// four-character string "%20".
				require.Equal(t, "%20", token)
				return nil
			})

			resp, err := app.Test(newRequest("/test/%2520"))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
		})
	}
}
