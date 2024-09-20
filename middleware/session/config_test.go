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
	require.Equal(t, "cookie:session_id", cfg.KeyLookup)
	require.NotNil(t, cfg.KeyGenerator)
	require.Equal(t, SourceCookie, cfg.source)
	require.Equal(t, "session_id", cfg.sessionName)
}

func TestConfigDefaultWithCustomConfig(t *testing.T) {
	// Test custom config
	customConfig := Config{
		IdleTimeout:  48 * time.Hour,
		KeyLookup:    "header:custom_session_id",
		KeyGenerator: func() string { return "custom_key" },
	}
	cfg := configDefault(customConfig)
	require.Equal(t, 48*time.Hour, cfg.IdleTimeout)
	require.Equal(t, "header:custom_session_id", cfg.KeyLookup)
	require.NotNil(t, cfg.KeyGenerator)
	require.Equal(t, SourceHeader, cfg.source)
	require.Equal(t, "custom_session_id", cfg.sessionName)
}

func TestDefaultErrorHandler(t *testing.T) {
	// Create a new Fiber app
	app := fiber.New()

	// Create a new context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Test DefaultErrorHandler
	DefaultErrorHandler(&ctx, fiber.ErrInternalServerError)
	require.Equal(t, fiber.StatusInternalServerError, ctx.Response().StatusCode())
}

func TestInvalidKeyLookupFormat(t *testing.T) {
	require.PanicsWithValue(t, "[session] KeyLookup must in the form of <source>:<name>", func() {
		configDefault(Config{KeyLookup: "invalid_format"})
	})
}

func TestUnsupportedSource(t *testing.T) {
	require.PanicsWithValue(t, "[session] source is not supported", func() {
		configDefault(Config{KeyLookup: "unsupported:session_id"})
	})
}
