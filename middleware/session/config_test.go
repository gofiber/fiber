package session

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestConfigDefault(t *testing.T) {
	// Test default config
	cfg := configDefault()
	require.Equal(t, 30*time.Minute, cfg.IdleTimeout)
	require.NotNil(t, cfg.KeyGenerator)
	require.NotNil(t, cfg.Extractor)
	require.Equal(t, "session_id", cfg.Extractor.Key)
	require.Equal(t, "Lax", cfg.CookieSameSite)
}

func TestConfigDefaultWithCustomConfig(t *testing.T) {
	// Test custom config
	customConfig := Config{
		IdleTimeout:  48 * time.Hour,
		Extractor:    FromHeader("X-Custom-Session"),
		KeyGenerator: func() string { return "custom_key" },
	}
	cfg := configDefault(customConfig)
	require.Equal(t, 48*time.Hour, cfg.IdleTimeout)
	require.NotNil(t, cfg.KeyGenerator)
	require.NotNil(t, cfg.Extractor)
	require.Equal(t, "X-Custom-Session", cfg.Extractor.Key)
}

func TestDefaultErrorHandler(t *testing.T) {
	// Create a new Fiber app
	app := fiber.New()

	// Create a new context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Test DefaultErrorHandler
	DefaultErrorHandler(ctx, fiber.ErrInternalServerError)
	require.Equal(t, fiber.StatusInternalServerError, ctx.Response().StatusCode())
}

func TestAbsoluteTimeoutValidation(t *testing.T) {
	require.PanicsWithValue(t, "[session] AbsoluteTimeout must be greater than or equal to IdleTimeout", func() {
		configDefault(Config{
			IdleTimeout:     30 * time.Minute,
			AbsoluteTimeout: 15 * time.Minute, // Less than IdleTimeout
		})
	})
}
