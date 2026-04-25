package session

import (
	"context"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
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
		store := NewStore()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// set cookie
		ctx.Request().Header.SetCookie(store.Extractor.Key, expectedID)

		id, writable := store.getSessionID(ctx)
		require.Equal(t, expectedID, id)
		require.True(t, writable, "cookie source should be writable")
	})

	t.Run("from header", func(t *testing.T) {
		t.Parallel()
		// session store
		store := NewStore(Config{
			Extractor: extractors.FromHeader("session_id"),
		})
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// set header
		ctx.Request().Header.Set(store.Extractor.Key, expectedID)

		id, writable := store.getSessionID(ctx)
		require.Equal(t, expectedID, id)
		require.True(t, writable, "header source should be writable")
	})

	t.Run("from url query", func(t *testing.T) {
		t.Parallel()
		// session store
		store := NewStore(Config{
			Extractor: extractors.FromQuery("session_id"),
		})
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// set url parameter
		ctx.Request().SetRequestURI(fmt.Sprintf("/path?%s=%s", store.Extractor.Key, expectedID))

		id, writable := store.getSessionID(ctx)
		require.Equal(t, expectedID, id)
		require.False(t, writable, "query source should not be writable")
	})
}

// go test -run Test_Store_Get
// Regression: https://github.com/gofiber/fiber/issues/1408
// Regression: https://github.com/gofiber/fiber/security/advisories/GHSA-98j2-3j3p-fw2v
func Test_Store_Get(t *testing.T) {
	// Regression: https://github.com/gofiber/fiber/security/advisories/GHSA-98j2-3j3p-fw2v
	t.Parallel()
	unexpectedID := "test-session-id"
	// fiber instance
	app := fiber.New()
	t.Run("session should be re-generated if it is invalid", func(t *testing.T) {
		t.Parallel()
		// session store
		store := NewStore()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// set cookie
		ctx.Request().Header.SetCookie(store.Extractor.Key, unexpectedID)

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)

		require.NotEqual(t, unexpectedID, acquiredSession.ID())
	})

	// Regression: https://github.com/gofiber/fiber/issues/3710
	// Query-based (read-only) extractors must preserve the client-provided session
	// ID even when no session data exists yet, so that subsequent requests carrying
	// the same query parameter are served the same session.
	t.Run("query extractor: session should preserve provided ID for new session", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{
			Extractor: extractors.FromQuery("session_id"),
		})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		clientID := "client-chosen-session-id"
		ctx.Request().SetRequestURI("/path?session_id=" + clientID)

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)
		defer acquiredSession.Release()

		// The session ID must equal the value provided via the query parameter.
		require.Equal(t, clientID, acquiredSession.ID())
		// The session is fresh because no prior data existed under this ID.
		require.True(t, acquiredSession.Fresh())
	})

	t.Run("query extractor: session data persists across requests with same ID", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{
			Extractor: extractors.FromQuery("session_id"),
		})
		app2 := fiber.New()

		clientID := "client-chosen-session-id"

		// First request: create and save a session under the query-provided ID.
		ctx1 := app2.AcquireCtx(&fasthttp.RequestCtx{})
		ctx1.Request().SetRequestURI("/path?session_id=" + clientID)
		sess1, err := store.Get(ctx1)
		require.NoError(t, err)
		sess1.Set("key", "value")
		require.NoError(t, sess1.Save())
		sess1.Release()
		app2.ReleaseCtx(ctx1)

		// Second request with the same query ID: must load the existing session data.
		ctx2 := app2.AcquireCtx(&fasthttp.RequestCtx{})
		defer app2.ReleaseCtx(ctx2)
		ctx2.Request().SetRequestURI("/path?session_id=" + clientID)
		sess2, err := store.Get(ctx2)
		require.NoError(t, err)
		defer sess2.Release()

		require.Equal(t, clientID, sess2.ID())
		require.False(t, sess2.Fresh())
		require.Equal(t, "value", sess2.Get("key"))
	})

	t.Run("chain extractor: cookie source discards unknown ID (security)", func(t *testing.T) {
		t.Parallel()
		// Chain with cookie first: an unknown cookie ID must still be discarded to
		// prevent session fixation attacks.
		store := NewStore(Config{
			Extractor: extractors.Chain(
				extractors.FromCookie("session_id"),
				extractors.FromQuery("session_id"),
			),
		})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		untrustedID := "untrusted-cookie-session-id"
		ctx.Request().Header.SetCookie("session_id", untrustedID)

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)
		defer acquiredSession.Release()

		// Cookie with unknown ID → new ID generated to prevent session fixation.
		require.NotEqual(t, untrustedID, acquiredSession.ID())
	})

	t.Run("chain extractor: query fallback preserves ID when cookie absent", func(t *testing.T) {
		t.Parallel()
		// Chain with cookie first, query fallback: when cookie is absent the
		// query-provided ID should be preserved.
		store := NewStore(Config{
			Extractor: extractors.Chain(
				extractors.FromCookie("session_id"),
				extractors.FromQuery("session_id"),
			),
		})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		queryID := "query-provided-session-id"
		// No cookie; only query param.
		ctx.Request().SetRequestURI("/path?session_id=" + queryID)

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)
		defer acquiredSession.Release()

		require.Equal(t, queryID, acquiredSession.ID())
		require.True(t, acquiredSession.Fresh())
	})
}

// go test -run Test_Store_DeleteSession
func Test_Store_DeleteSession(t *testing.T) {
	t.Parallel()
	// fiber instance
	app := fiber.New()
	// session store
	store := NewStore()

	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// Create a new session
	session, err := store.Get(ctx)
	require.NoError(t, err)

	// Save the session ID
	sessionID := session.ID()

	// Delete the session
	err = store.Delete(ctx, sessionID)
	require.NoError(t, err)

	// Try to get the session again
	session, err = store.Get(ctx)
	require.NoError(t, err)

	// The session ID should be different now, because the old session was deleted
	require.NotEqual(t, sessionID, session.ID())
}

func TestStore_Get_SessionAlreadyLoaded(t *testing.T) {
	// Create a new Fiber app
	app := fiber.New()

	// Create a new context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// Mock middleware and set it in the context
	middleware := &Middleware{}
	ctx.Locals(middlewareContextKey, middleware)

	// Create a new store
	store := &Store{}

	// Call the Get method
	sess, err := store.Get(ctx)

	// Assert that the error is ErrSessionAlreadyLoadedByMiddleware
	require.Nil(t, sess)
	require.Equal(t, ErrSessionAlreadyLoadedByMiddleware, err)
}

func TestStore_Delete(t *testing.T) {
	// Create a new store
	store := NewStore()

	t.Run("delete with empty session ID", func(t *testing.T) {
		err := store.Delete(context.Background(), "")
		require.Error(t, err)
		require.Equal(t, ErrEmptySessionID, err)
	})

	t.Run("delete non-existing session", func(t *testing.T) {
		err := store.Delete(context.Background(), "non-existing-session-id")
		require.NoError(t, err)
	})
}

func Test_Store_GetByID(t *testing.T) {
	t.Parallel()
	// Create a new store
	store := NewStore()

	t.Run("empty session ID", func(t *testing.T) {
		t.Parallel()
		sess, err := store.GetByID(context.Background(), "")
		require.Error(t, err)
		require.Nil(t, sess)
		require.Equal(t, ErrEmptySessionID, err)
	})

	t.Run("nonexistent session ID", func(t *testing.T) {
		t.Parallel()
		sess, err := store.GetByID(context.Background(), "nonexistent-session-id")
		require.Error(t, err)
		require.Nil(t, sess)
		require.Equal(t, ErrSessionIDNotFoundInStore, err)
	})

	t.Run("valid session ID", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		// Create a new session
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		session, err := store.Get(ctx)
		defer session.Release()
		defer app.ReleaseCtx(ctx)
		require.NoError(t, err)

		// Save the session ID
		sessionID := session.ID()

		// Save the session
		err = session.Save()
		require.NoError(t, err)

		// Retrieve the session by ID
		retrievedSession, err := store.GetByID(context.Background(), sessionID)
		require.NoError(t, err)
		require.NotNil(t, retrievedSession)
		require.Equal(t, sessionID, retrievedSession.ID())

		// Call Save on the retrieved session
		retrievedSession.Set("key", "value")
		err = retrievedSession.Save()
		require.NoError(t, err)

		// Call Other Session methods
		require.Equal(t, "value", retrievedSession.Get("key"))
		require.False(t, retrievedSession.Fresh())

		require.NoError(t, retrievedSession.Reset())
		require.NoError(t, retrievedSession.Destroy())
		require.IsType(t, []any{}, retrievedSession.Keys())
		require.NoError(t, retrievedSession.Regenerate())
		require.NotPanics(t, func() {
			retrievedSession.Release()
		})
	})
}
