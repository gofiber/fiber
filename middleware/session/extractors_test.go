package session

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// newRequest creates a new *http.Request for Fiber's app.Test
func newRequest(method, target string) *http.Request {
	req, err := http.NewRequestWithContext(context.Background(), method, target, nil)
	if err != nil {
		panic(err)
	}
	return req
}

func TestFromCookie(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	extractor := FromCookie("session_id")

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		ctx.Request().Header.SetCookie("session_id", "test-session-id")

		sessionID, err := extractor.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "test-session-id", sessionID)
	})

	t.Run("missing cookie", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		sessionID, err := extractor.Extract(ctx)
		require.Error(t, err)
		require.Equal(t, ErrMissingSessionIDInCookie, err)
		require.Empty(t, sessionID)
	})
}

func TestFromHeader(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	extractor := FromHeader("X-Session-ID")

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		ctx.Request().Header.Set("X-Session-ID", "test-session-id")

		sessionID, err := extractor.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "test-session-id", sessionID)
	})

	t.Run("missing header", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		sessionID, err := extractor.Extract(ctx)
		require.Error(t, err)
		require.Equal(t, ErrMissingSessionIDInHeader, err)
		require.Empty(t, sessionID)
	})
}

func TestFromQuery(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	extractor := FromQuery("session_id")

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		ctx.Request().SetRequestURI("/test?session_id=test-session-id")

		sessionID, err := extractor.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "test-session-id", sessionID)
	})

	t.Run("missing query param", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		ctx.Request().SetRequestURI("/test")

		sessionID, err := extractor.Extract(ctx)
		require.Error(t, err)
		require.Equal(t, ErrMissingSessionIDInQuery, err)
		require.Empty(t, sessionID)
	})
}

func TestFromForm(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	extractor := FromForm("session_id")

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		ctx.Request().Header.SetMethod("POST")
		ctx.Request().Header.SetContentType("application/x-www-form-urlencoded")
		ctx.Request().SetBodyString("session_id=test-session-id")

		sessionID, err := extractor.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "test-session-id", sessionID)
	})

	t.Run("missing form field", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		ctx.Request().Header.SetMethod("POST")
		ctx.Request().Header.SetContentType("application/x-www-form-urlencoded")
		ctx.Request().SetBodyString("other_field=value")

		sessionID, err := extractor.Extract(ctx)
		require.Error(t, err)
		require.Equal(t, ErrMissingSessionIDInForm, err)
		require.Empty(t, sessionID)
	})
}

func TestFromParam(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// FromParam
	app.Get("/test/:csrf", func(c fiber.Ctx) error {
		token, err := FromParam("csrf").Extract(c)
		require.NoError(t, err)
		require.Equal(t, "token_from_param", token)
		return nil
	})

	// Note: This test is more complex as it requires route setup
	// In a real scenario, you'd set up a route with parameters
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		_, err := app.Test(newRequest(fiber.MethodGet, "/test/token_from_param"))
		require.NoError(t, err)
	})
}

func TestChain(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	t.Run("first extractor succeeds", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		ctx.Request().Header.SetCookie("session_id", "cookie-session-id")
		ctx.Request().Header.Set("X-Session-ID", "header-session-id")

		extractor := Chain(
			FromCookie("session_id"),
			FromHeader("X-Session-ID"),
		)

		sessionID, err := extractor.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "cookie-session-id", sessionID) // First extractor wins
	})

	t.Run("second extractor succeeds", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		ctx.Request().Header.Set("X-Session-ID", "header-session-id")

		extractor := Chain(
			FromCookie("session_id"),
			FromHeader("X-Session-ID"),
		)

		sessionID, err := extractor.Extract(ctx)
		require.NoError(t, err)
		require.Equal(t, "header-session-id", sessionID)
	})

	t.Run("all extractors fail", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		extractor := Chain(
			FromCookie("session_id"),
			FromHeader("X-Session-ID"),
		)

		sessionID, err := extractor.Extract(ctx)
		require.Error(t, err)
		require.Empty(t, sessionID)
	})

	t.Run("empty chain", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		extractor := Chain()

		sessionID, err := extractor.Extract(ctx)
		require.Error(t, err)
		require.Equal(t, ErrMissingSessionID, err)
		require.Empty(t, sessionID)
	})
}
