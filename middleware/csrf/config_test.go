package csrf

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Test security validation functions
func Test_CSRF_ExtractorSecurity_Validation(t *testing.T) {
	t.Parallel()

	// Test secure configurations - should not panic
	t.Run("SecureConfigurations", func(t *testing.T) {
		t.Parallel()
		secureConfigs := []Config{
			{Extractor: FromHeader("X-Csrf-Token")},
			{Extractor: FromForm("_csrf")},
			{Extractor: FromQuery("csrf_token")},
			{Extractor: FromParam("csrf")},
			{Extractor: Chain(FromHeader("X-Csrf-Token"), FromForm("_csrf"))},
		}

		for i, cfg := range secureConfigs {
			t.Run(fmt.Sprintf("Config%d", i), func(t *testing.T) {
				require.NotPanics(t, func() {
					configDefault(cfg)
				})
			})
		}
	})

	// Test insecure configurations - should panic
	t.Run("InsecureCookieExtractor", func(t *testing.T) {
		t.Parallel()
		// Create a custom extractor that reads from cookie (simulating dangerous behavior)
		insecureCookieExtractor := Extractor{
			Extract: func(c fiber.Ctx) (string, error) {
				return c.Cookies("csrf_"), nil
			},
			Source: SourceCookie,
			Key:    "csrf_",
		}

		cfg := Config{
			CookieName: "csrf_",
			Extractor:  insecureCookieExtractor,
		}

		require.Panics(t, func() {
			configDefault(cfg)
		}, "Should panic when extractor reads from same cookie")
	})

	// Test insecure chained extractors
	t.Run("InsecureChainedExtractor", func(t *testing.T) {
		t.Parallel()
		insecureCookieExtractor := Extractor{
			Extract: func(c fiber.Ctx) (string, error) {
				return c.Cookies("csrf_"), nil
			},
			Source: SourceCookie,
			Key:    "csrf_",
		}

		chainedExtractor := Chain(
			FromHeader("X-Csrf-Token"),
			insecureCookieExtractor, // This should trigger panic
		)

		cfg := Config{
			CookieName: "csrf_",
			Extractor:  chainedExtractor,
		}

		require.Panics(t, func() {
			configDefault(cfg)
		}, "Should panic when chained extractor reads from same cookie")
	})

	// Test different cookie names - should be secure
	t.Run("DifferentCookieNames", func(t *testing.T) {
		t.Parallel()
		cookieExtractor := Extractor{
			Extract: func(c fiber.Ctx) (string, error) {
				return c.Cookies("different_cookie"), nil
			},
			Source: SourceCookie,
			Key:    "different_cookie",
		}

		cfg := Config{
			CookieName: "csrf_",
			Extractor:  cookieExtractor,
		}

		require.NotPanics(t, func() {
			configDefault(cfg)
		}, "Should not panic when extractor reads from different cookie")
	})
}

// Test extractor metadata
func Test_CSRF_Extractor_Metadata(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		expectedKey    string
		extractor      Extractor
		expectedSource Source
	}{
		{
			name:           "FromHeader",
			extractor:      FromHeader("X-Custom-Token"),
			expectedSource: SourceHeader,
			expectedKey:    "X-Custom-Token",
		},
		{
			name:           "FromForm",
			extractor:      FromForm("_token"),
			expectedSource: SourceForm,
			expectedKey:    "_token",
		},
		{
			name:           "FromQuery",
			extractor:      FromQuery("token"),
			expectedSource: SourceQuery,
			expectedKey:    "token",
		},
		{
			name:           "FromParam",
			extractor:      FromParam("id"),
			expectedSource: SourceParam,
			expectedKey:    "id",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expectedSource, tc.extractor.Source)
			require.Equal(t, tc.expectedKey, tc.extractor.Key)
			require.NotNil(t, tc.extractor.Extract)
		})
	}
}

// Test chain extractor metadata
func Test_CSRF_Chain_Extractor_Metadata(t *testing.T) {
	t.Parallel()

	t.Run("EmptyChain", func(t *testing.T) {
		t.Parallel()
		chained := Chain()
		require.Equal(t, SourceCustom, chained.Source)
		require.Equal(t, "", chained.Key)
		require.Empty(t, chained.Chain)
	})

	t.Run("SingleExtractor", func(t *testing.T) {
		t.Parallel()
		header := FromHeader("X-Token")
		chained := Chain(header)
		require.Equal(t, SourceHeader, chained.Source)
		require.Equal(t, "X-Token", chained.Key)
		require.Len(t, chained.Chain, 1)
	})

	t.Run("MultipleExtractors", func(t *testing.T) {
		t.Parallel()
		header := FromHeader("X-Token")
		form := FromForm("_csrf")
		chained := Chain(header, form)

		// Should use first extractor's metadata
		require.Equal(t, SourceHeader, chained.Source)
		require.Equal(t, "X-Token", chained.Key)
		require.Len(t, chained.Chain, 2)
		require.Equal(t, header.Source, chained.Chain[0].Source)
		require.Equal(t, form.Source, chained.Chain[1].Source)
	})
}

// Test custom extractor with new struct pattern
func Test_CSRF_Custom_Extractor_Struct(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Custom extractor using new struct pattern
	customExtractor := Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			// Extract from custom header
			token := c.Get("X-Custom-CSRF")
			if token == "" {
				return "", ErrMissingHeader
			}
			return token, nil
		},
		Source: SourceCustom,
		Key:    "X-Custom-CSRF",
	}

	app.Use(New(Config{Extractor: customExtractor}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// Test with custom header
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set("X-Custom-CSRF", token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test without custom header
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())
}

// Test error types for different extractors
func Test_CSRF_Extractor_Error_Types(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		expectedError error
		setupRequest  func(*fasthttp.RequestCtx)
		name          string
		extractor     Extractor
	}{
		{
			name:      "MissingHeader",
			extractor: FromHeader("X-Missing"),
			setupRequest: func(_ *fasthttp.RequestCtx) {
				// Don't set the header
			},
			expectedError: ErrMissingHeader,
		},
		{
			name:      "MissingForm",
			extractor: FromForm("_missing"),
			setupRequest: func(ctx *fasthttp.RequestCtx) {
				ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
				// Don't set form data
			},
			expectedError: ErrMissingForm,
		},
		{
			name:      "MissingQuery",
			extractor: FromQuery("missing"),
			setupRequest: func(ctx *fasthttp.RequestCtx) {
				ctx.Request.SetRequestURI("/")
				// Don't set query param
			},
			expectedError: ErrMissingQuery,
		},
		{
			name:      "MissingParam",
			extractor: FromParam("missing"),
			setupRequest: func(_ *fasthttp.RequestCtx) {
				// This would need special route setup to test properly
				// For now, we'll test the extractor directly
			},
			expectedError: ErrMissingParam,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			ctx := &fasthttp.RequestCtx{}

			tc.setupRequest(ctx)

			c := app.AcquireCtx(ctx)

			_, err := tc.extractor.Extract(c)
			require.Error(t, err)
			require.Equal(t, tc.expectedError, err)
			app.ReleaseCtx(c)
		})
	}
}

// Test security warning logs (would need to capture log output in real implementation)
func Test_CSRF_Security_Warnings(t *testing.T) {
	t.Parallel()

	// Test that insecure extractors trigger warnings
	// Note: In a real implementation, you'd want to capture log output
	// For now, we just test that the configuration doesn't panic

	insecureConfigs := []Config{
		{Extractor: FromQuery("csrf_token")},
		{Extractor: FromParam("csrf")},
	}

	for i, cfg := range insecureConfigs {
		t.Run(fmt.Sprintf("InsecureConfig%d", i), func(t *testing.T) {
			t.Parallel()
			require.NotPanics(t, func() {
				configDefault(cfg)
			})
		})
	}
}

// Test isInsecureCookieExtractor function directly
func Test_isInsecureCookieExtractor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		cookieName string
		extractor  Extractor
		expected   bool
	}{
		{
			name: "SecureHeaderExtractor",
			extractor: Extractor{
				Source: SourceHeader,
				Key:    "X-Csrf-Token",
			},
			cookieName: "csrf_",
			expected:   false,
		},
		{
			name: "InsecureCookieExtractor",
			extractor: Extractor{
				Source: SourceCookie,
				Key:    "csrf_",
			},
			cookieName: "csrf_",
			expected:   true,
		},
		{
			name: "CookieExtractorDifferentName",
			extractor: Extractor{
				Source: SourceCookie,
				Key:    "different_cookie",
			},
			cookieName: "csrf_",
			expected:   false,
		},
		{
			name: "CustomExtractorSafeName",
			extractor: Extractor{
				Source: SourceCustom,
				Key:    "safe_key",
			},
			cookieName: "csrf_",
			expected:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := isInsecureCookieExtractor(tc.extractor, tc.cookieName)
			require.Equal(t, tc.expected, result)
		})
	}
}

func Test_CSRF_CookieName_CaseInsensitive_Warning(t *testing.T) {
	t.Parallel()

	// Extractor uses "CSRF_" (uppercase), config uses "csrf_" (lowercase)
	extractor := Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			return c.Cookies("CSRF_"), nil
		},
		Source: SourceCookie,
		Key:    "CSRF_",
	}

	cfg := Config{
		CookieName: "csrf_",
		Extractor:  extractor,
	}

	// Should not panic, but should log a warning
	require.NotPanics(t, func() {
		configDefault(cfg)
	}, "Should not panic for case-insensitive cookie name match, but should warn")
}
