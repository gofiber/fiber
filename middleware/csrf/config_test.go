package csrf

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_CSRF_ExtractorSecurity_Validation(t *testing.T) {
	t.Parallel()

	// An extractor that reads from the cookie "csrf_" (simulating dangerous behavior)
	insecureCookieExtractor := extractors.Extractor{
		Extract: func(c fiber.Ctx) (string, error) { return c.Cookies("csrf_"), nil },
		Source:  extractors.SourceCookie,
		Key:     "csrf_",
	}

	// A deeply nested chain must not cause unbounded recursion during validation.
	// Build it using only secure extractors so validation terminates without
	// panicking.
	deepChain := extractors.FromHeader("X-Csrf-Token")
	for range 150 {
		deepChain = extractors.Chain(extractors.FromHeader("X-Csrf-Token"), deepChain)
	}

	tests := []struct {
		name       string
		cookieName string
		message    string
		extractor  extractors.Extractor
		wantPanic  bool
	}{
		// Secure configurations - should not panic
		{name: "SecureConfigurations/FromHeader", extractor: extractors.FromHeader("X-Csrf-Token")},
		{name: "SecureConfigurations/FromForm", extractor: extractors.FromForm("_csrf")},
		{name: "SecureConfigurations/FromQuery", extractor: extractors.FromQuery("csrf_token")},
		{name: "SecureConfigurations/FromParam", extractor: extractors.FromParam("csrf")},
		{name: "SecureConfigurations/Chain", extractor: extractors.Chain(extractors.FromHeader("X-Csrf-Token"), extractors.FromForm("_csrf"))},
		// These extractors trigger a security warning. Capturing the log output
		// would be the stronger check; for now the test requires that the
		// configuration does not panic.
		{name: "InsecureConfig/FromQuery", extractor: extractors.FromQuery("csrf_token")},
		{name: "InsecureConfig/FromParam", extractor: extractors.FromParam("csrf")},
		// Insecure configurations - should panic
		{
			name: "InsecureCookieExtractor", cookieName: "csrf_", extractor: insecureCookieExtractor, wantPanic: true,
			message: "Should panic when extractor reads from same cookie",
		},
		{
			name:       "InsecureChainedExtractor",
			cookieName: "csrf_",
			extractor: extractors.Chain(
				extractors.FromHeader("X-Csrf-Token"),
				insecureCookieExtractor, // This should trigger panic
			),
			wantPanic: true,
			message:   "Should panic when chained extractor reads from same cookie",
		},
		{
			name:       "InsecureNestedChainedExtractor",
			cookieName: "csrf_",
			extractor: extractors.Chain(
				extractors.FromHeader("X-Csrf-Token"),
				extractors.Chain(extractors.FromHeader("X-Alt-Csrf-Token"), extractors.FromCookie("csrf_")),
			),
			wantPanic: true,
			message:   "Should panic when a nested chained extractor reads from same cookie",
		},
		{
			name: "DeeplyNestedSecureChain", cookieName: "csrf_", extractor: deepChain,
			message: "Should terminate without panic for a deeply nested secure chain",
		},
		// Different cookie names - should be secure
		{
			name:       "DifferentCookieNames",
			cookieName: "csrf_",
			extractor: extractors.Extractor{
				Extract: func(c fiber.Ctx) (string, error) { return c.Cookies("different_cookie"), nil },
				Source:  extractors.SourceCookie,
				Key:     "different_cookie",
			},
			message: "Should not panic when extractor reads from different cookie",
		},
		{
			// Extractor uses "CSRF_" (uppercase), config uses "csrf_" (lowercase):
			// should not panic, but should log a warning
			name:       "CookieName_CaseInsensitive_Warning",
			cookieName: "csrf_",
			extractor: extractors.Extractor{
				Extract: func(c fiber.Ctx) (string, error) { return c.Cookies("CSRF_"), nil },
				Source:  extractors.SourceCookie,
				Key:     "CSRF_",
			},
			message: "Should not panic for case-insensitive cookie name match, but should warn",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := Config{CookieName: tc.cookieName, Extractor: tc.extractor}
			if tc.wantPanic {
				require.Panics(t, func() { configDefault(cfg) }, tc.message)
				return
			}
			require.NotPanics(t, func() { configDefault(cfg) }, tc.message)
		})
	}
}

func Test_CSRF_Extractor_Metadata(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		key       string
		extractor extractors.Extractor
		source    extractors.Source
	}{
		{
			name: "FromHeader", extractor: extractors.FromHeader("X-Custom-Token"), source: extractors.SourceHeader,
			key: "X-Custom-Token",
		},
		{name: "FromForm", extractor: extractors.FromForm("_token"), source: extractors.SourceForm, key: "_token"},
		{name: "FromQuery", extractor: extractors.FromQuery("token"), source: extractors.SourceQuery, key: "token"},
		{name: "FromParam", extractor: extractors.FromParam("id"), source: extractors.SourceParam, key: "id"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.source, tc.extractor.Source)
			require.Equal(t, tc.key, tc.extractor.Key)
			require.NotNil(t, tc.extractor.Extract)
		})
	}
}

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

func Test_CSRF_Extractor_Error_Types(t *testing.T) {
	t.Parallel()
	// atRoot sets the request URI, for query parsing.
	atRoot := func(ctx *fasthttp.RequestCtx) { ctx.Request.SetRequestURI("/") }

	testCases := []struct {
		setupRequest func(*fasthttp.RequestCtx) // none when nil: a request with nothing set
		name         string
		extractor    extractors.Extractor
	}{
		{name: "MissingHeader", extractor: extractors.FromHeader("X-Missing")},
		{
			name:      "MissingForm",
			extractor: extractors.FromForm("_missing"),
			setupRequest: func(ctx *fasthttp.RequestCtx) {
				ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm) // and no form data
			},
		},
		{name: "MissingQuery", extractor: extractors.FromQuery("missing"), setupRequest: atRoot},
		// This would need special route setup to test properly, so the
		// extractor is tested directly
		{name: "MissingParam", extractor: extractors.FromParam("missing")},
		{name: "MissingHeader_Named", extractor: extractors.FromHeader("X-Missing-Header")},
		{name: "MissingQuery_Named", extractor: extractors.FromQuery("missing_param"), setupRequest: atRoot},
		// Params are handled by the router
		{name: "MissingParam_Named", extractor: extractors.FromParam("missing_param")},
		{
			name:      "MissingForm_Post",
			extractor: extractors.FromForm("missing_field"),
			setupRequest: func(ctx *fasthttp.RequestCtx) {
				// Properly initialize request for form parsing
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.Header.SetContentType(fiber.MIMEApplicationForm)
				ctx.Request.SetBodyString("") // Empty form body
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := &fasthttp.RequestCtx{}
			if tc.setupRequest != nil {
				tc.setupRequest(ctx)
			}

			c := apptest.CtxFor(t, fiber.New(), ctx)

			token, err := tc.extractor.Extract(c)
			require.Error(t, err)
			require.Empty(t, token)
			require.Equal(t, extractors.ErrNotFound, err)
		})
	}
}

func Test_isInsecureCookieExtractor(t *testing.T) {
	t.Parallel()
	// Every case checks an extractor with source and key against the cookie name "csrf_".
	testCases := []struct {
		name     string
		key      string
		source   extractors.Source
		expected bool
	}{
		{name: "SecureHeaderExtractor", source: extractors.SourceHeader, key: "X-Csrf-Token"},
		{name: "InsecureCookieExtractor", source: extractors.SourceCookie, key: "csrf_", expected: true},
		{name: "CookieExtractorDifferentName", source: extractors.SourceCookie, key: "different_cookie"},
		{name: "CustomExtractorSafeName", source: extractors.SourceCustom, key: "safe_key"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			extractor := extractors.Extractor{Source: tc.source, Key: tc.key}
			require.Equal(t, tc.expected, isInsecureCookieExtractor(extractor, "csrf_"))
		})
	}
}

func Test_CSRF_validateExtractorSecurity_NilConfig(t *testing.T) {
	t.Parallel()
	require.NotPanics(t, func() { validateExtractorSecurity(nil) })
}
