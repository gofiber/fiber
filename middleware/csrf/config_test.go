package csrf

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
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
			{Extractor: extractors.FromHeader("X-Csrf-Token")},
			{Extractor: extractors.FromForm("_csrf")},
			{Extractor: extractors.FromQuery("csrf_token")},
			{Extractor: extractors.FromParam("csrf")},
			{Extractor: extractors.Chain(extractors.FromHeader("X-Csrf-Token"), extractors.FromForm("_csrf"))},
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
		insecureCookieExtractor := extractors.Extractor{
			Extract: func(c fiber.Ctx) (string, error) {
				return c.Cookies("csrf_"), nil
			},
			Source: extractors.SourceCookie,
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
		insecureCookieExtractor := extractors.Extractor{
			Extract: func(c fiber.Ctx) (string, error) {
				return c.Cookies("csrf_"), nil
			},
			Source: extractors.SourceCookie,
			Key:    "csrf_",
		}

		chainedExtractor := extractors.Chain(
			extractors.FromHeader("X-Csrf-Token"),
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
		cookieExtractor := extractors.Extractor{
			Extract: func(c fiber.Ctx) (string, error) {
				return c.Cookies("different_cookie"), nil
			},
			Source: extractors.SourceCookie,
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
		extractor      extractors.Extractor
		expectedSource extractors.Source
	}{
		{
			name:           "FromHeader",
			extractor:      extractors.FromHeader("X-Custom-Token"),
			expectedSource: extractors.SourceHeader,
			expectedKey:    "X-Custom-Token",
		},
		{
			name:           "FromForm",
			extractor:      extractors.FromForm("_token"),
			expectedSource: extractors.SourceForm,
			expectedKey:    "_token",
		},
		{
			name:           "FromQuery",
			extractor:      extractors.FromQuery("token"),
			expectedSource: extractors.SourceQuery,
			expectedKey:    "token",
		},
		{
			name:           "FromParam",
			extractor:      extractors.FromParam("id"),
			expectedSource: extractors.SourceParam,
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
		chained := extractors.Chain()
		require.Equal(t, extractors.SourceCustom, chained.Source)
		require.Empty(t, chained.Key)
		require.Empty(t, chained.Chain)
	})

	t.Run("SingleExtractor", func(t *testing.T) {
		t.Parallel()
		header := extractors.FromHeader("X-Token")
		chained := extractors.Chain(header)
		require.Equal(t, extractors.SourceHeader, chained.Source)
		require.Equal(t, "X-Token", chained.Key)
		require.Len(t, chained.Chain, 1)
	})

	t.Run("MultipleExtractors", func(t *testing.T) {
		t.Parallel()
		header := extractors.FromHeader("X-Token")
		form := extractors.FromForm("_csrf")
		chained := extractors.Chain(header, form)

		// Should use first extractor's metadata
		require.Equal(t, extractors.SourceHeader, chained.Source)
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
	customExtractor := extractors.Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			// Extract from custom header
			token := c.Get("X-Custom-CSRF")
			if token == "" {
				return "", extractors.ErrNotFound
			}
			return token, nil
		},
		Source: extractors.SourceCustom,
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
		extractor     extractors.Extractor
	}{
		{
			name:      "MissingHeader",
			extractor: extractors.FromHeader("X-Missing"),
			setupRequest: func(_ *fasthttp.RequestCtx) {
				// Don't set the header
			},
			expectedError: extractors.ErrNotFound,
		},
		{
			name:      "MissingForm",
			extractor: extractors.FromForm("_missing"),
			setupRequest: func(ctx *fasthttp.RequestCtx) {
				ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
				// Don't set form data
			},
			expectedError: extractors.ErrNotFound,
		},
		{
			name:      "MissingQuery",
			extractor: extractors.FromQuery("missing"),
			setupRequest: func(ctx *fasthttp.RequestCtx) {
				ctx.Request.SetRequestURI("/")
				// Don't set query param
			},
			expectedError: extractors.ErrNotFound,
		},
		{
			name:      "MissingParam",
			extractor: extractors.FromParam("missing"),
			setupRequest: func(_ *fasthttp.RequestCtx) {
				// This would need special route setup to test properly
				// For now, we'll test the extractor directly
			},
			expectedError: extractors.ErrNotFound,
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
		{Extractor: extractors.FromQuery("csrf_token")},
		{Extractor: extractors.FromParam("csrf")},
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
		extractor  extractors.Extractor
		expected   bool
	}{
		{
			name: "SecureHeaderExtractor",
			extractor: extractors.Extractor{
				Source: extractors.SourceHeader,
				Key:    "X-Csrf-Token",
			},
			cookieName: "csrf_",
			expected:   false,
		},
		{
			name: "InsecureCookieExtractor",
			extractor: extractors.Extractor{
				Source: extractors.SourceCookie,
				Key:    "csrf_",
			},
			cookieName: "csrf_",
			expected:   true,
		},
		{
			name: "CookieExtractorDifferentName",
			extractor: extractors.Extractor{
				Source: extractors.SourceCookie,
				Key:    "different_cookie",
			},
			cookieName: "csrf_",
			expected:   false,
		},
		{
			name: "CustomExtractorSafeName",
			extractor: extractors.Extractor{
				Source: extractors.SourceCustom,
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
	extractor := extractors.Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			return c.Cookies("CSRF_"), nil
		},
		Source: extractors.SourceCookie,
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
