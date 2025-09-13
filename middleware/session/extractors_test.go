package session

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Session-specific extractor tests
func TestSessionExtractors(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Simulate a session extractor that checks for a session cookie and validates its format
	sessionExtractor := func(c fiber.Ctx) (string, error) {
		val := c.Cookies("session_id")
		if val == "" {
			return "", fiber.ErrUnauthorized
		}
		if val != "valid-session" {
			return "", fiber.ErrForbidden
		}
		return val, nil
	}

	t.Run("valid session cookie", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)
		ctx.Request().Header.SetCookie("session_id", "valid-session")

		sessionID, err := sessionExtractor(ctx)
		require.NoError(t, err)
		require.Equal(t, "valid-session", sessionID)
	})

	t.Run("missing session cookie", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		sessionID, err := sessionExtractor(ctx)
		require.Error(t, err)
		require.Equal(t, fiber.ErrUnauthorized, err)
		require.Empty(t, sessionID)
	})

	t.Run("invalid session cookie", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)
		ctx.Request().Header.SetCookie("session_id", "invalid-session")

		sessionID, err := sessionExtractor(ctx)
		require.Error(t, err)
		require.Equal(t, fiber.ErrForbidden, err)
		require.Empty(t, sessionID)
	})
}
