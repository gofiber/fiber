package session

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestConfigDefault(t *testing.T) {
	t.Parallel()

	// Test default config
	cfg := configDefault()
	require.Equal(t, 30*time.Minute, cfg.IdleTimeout)
	require.NotNil(t, cfg.KeyGenerator)
	require.NotNil(t, cfg.Extractor)
	require.Equal(t, "session_id", cfg.Extractor.Key)
	require.Equal(t, "Lax", cfg.CookieSameSite)
	require.True(t, cfg.CookieSecure)
	require.True(t, cfg.CookieHTTPOnly)
}

func TestConfigDefaultWithCustomConfig(t *testing.T) {
	t.Parallel()

	// Test custom config
	customConfig := Config{
		IdleTimeout:  48 * time.Hour,
		Extractor:    extractors.FromHeader("X-Custom-Session"),
		KeyGenerator: func() string { return "custom_key" },
	}
	cfg := configDefault(customConfig)
	require.Equal(t, 48*time.Hour, cfg.IdleTimeout)
	require.NotNil(t, cfg.KeyGenerator)
	require.NotNil(t, cfg.Extractor)
	require.Equal(t, "X-Custom-Session", cfg.Extractor.Key)
}

func TestConfigDefaultCookieSecurity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		config           Config
		expectedSecure   bool
		expectedHTTPOnly bool
	}{
		{
			name:             "empty config uses secure defaults",
			config:           Config{},
			expectedSecure:   true,
			expectedHTTPOnly: true,
		},
		{
			name: "false cookie flags use secure defaults",
			config: Config{
				CookieSecure:   false,
				CookieHTTPOnly: false,
			},
			expectedSecure:   true,
			expectedHTTPOnly: true,
		},
		{
			name: "disable flags opt out of secure defaults",
			config: Config{
				DisableCookieSecure:   true,
				DisableCookieHTTPOnly: true,
			},
			expectedSecure:   false,
			expectedHTTPOnly: false,
		},
		{
			name: "disable flags take precedence over explicit true",
			config: Config{
				CookieSecure:          true,
				CookieHTTPOnly:        true,
				DisableCookieSecure:   true,
				DisableCookieHTTPOnly: true,
			},
			expectedSecure:   false,
			expectedHTTPOnly: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := configDefault(tc.config)
			require.Equal(t, tc.expectedSecure, cfg.CookieSecure)
			require.Equal(t, tc.expectedHTTPOnly, cfg.CookieHTTPOnly)
		})
	}
}

func TestDefaultErrorHandler(t *testing.T) {
	t.Parallel()

	// Create a new Fiber app
	app := fiber.New()

	// Create a new context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Test DefaultErrorHandler
	DefaultErrorHandler(ctx, fiber.ErrInternalServerError)
	require.Equal(t, fiber.StatusInternalServerError, ctx.Response().StatusCode())
}

func TestAbsoluteTimeoutValidation(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "[session] AbsoluteTimeout must be greater than or equal to IdleTimeout", func() {
		configDefault(Config{
			IdleTimeout:     30 * time.Minute,
			AbsoluteTimeout: 15 * time.Minute, // Less than IdleTimeout
		})
	})
}
