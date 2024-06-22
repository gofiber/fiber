package session

import (
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Store_getSessionID
func Test_Store_getSessionID(t *testing.T) {
	t.Parallel()
	expectedID := "test-session-id"

	// fiber instance
	app := fiber.New()

	t.Run("from cookie", func(t *testing.T) {
		t.Parallel()
		// session store
		store := newStore()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

		// set cookie
		ctx.Request().Header.SetCookie(store.sessionName, expectedID)

		require.Equal(t, expectedID, store.getSessionID(ctx))
	})

	t.Run("from header", func(t *testing.T) {
		t.Parallel()
		// session store
		store := newStore(Config{
			KeyLookup: "header:session_id",
		})
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

		// set header
		ctx.Request().Header.Set(store.sessionName, expectedID)

		require.Equal(t, expectedID, store.getSessionID(ctx))
	})

	t.Run("from url query", func(t *testing.T) {
		t.Parallel()
		// session store
		store := newStore(Config{
			KeyLookup: "query:session_id",
		})
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

		// set url parameter
		ctx.Request().SetRequestURI(fmt.Sprintf("/path?%s=%s", store.sessionName, expectedID))

		require.Equal(t, expectedID, store.getSessionID(ctx))
	})
}

// go test -run Test_Store_Get
// Regression: https://github.com/gofiber/fiber/issues/1408
func Test_Store_Get(t *testing.T) {
	t.Parallel()
	unexpectedID := "test-session-id"
	// fiber instance
	app := fiber.New()
	t.Run("session should persisted even session is invalid", func(t *testing.T) {
		t.Parallel()
		// session store
		store := newStore()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

		// set cookie
		ctx.Request().Header.SetCookie(store.sessionName, unexpectedID)

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)

		require.Equal(t, unexpectedID, acquiredSession.ID())
	})
}

// go test -run Test_Store_DeleteSession
func Test_Store_DeleteSession(t *testing.T) {
	t.Parallel()
	// fiber instance
	app := fiber.New()
	// session store
	store := newStore()

	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Create a new session
	session, err := store.Get(ctx)
	require.NoError(t, err)

	// Save the session ID
	sessionID := session.ID()

	// Delete the session
	err = store.Delete(sessionID)
	require.NoError(t, err)

	// Try to get the session again
	session, err = store.Get(ctx)
	require.NoError(t, err)

	// The session ID should be different now, because the old session was deleted
	require.NotEqual(t, sessionID, session.ID())
}
