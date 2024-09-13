package session

import (
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
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
		store := newStore()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

		// set cookie
		ctx.Request().Header.SetCookie(store.sessionName, unexpectedID)

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)

		require.NotEqual(t, unexpectedID, acquiredSession.ID())
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

func TestStore_Get_SessionAlreadyLoaded(t *testing.T) {
	// Create a new Fiber app
	app := fiber.New()

	// Create a new context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	// Mock middleware and set it in the context
	middleware := &Middleware{}
	ctx.Locals(key, middleware)

	// Create a new store
	store := &Store{}

	// Call the Get method
	sess, err := store.Get(ctx)

	// Assert that the error is ErrSessionAlreadyLoadedByMiddleware
	assert.Nil(t, sess)
	assert.Equal(t, ErrSessionAlreadyLoadedByMiddleware, err)
}

func TestStore_Delete(t *testing.T) {
	// Create a new store
	store := newStore()

	t.Run("delete with empty session ID", func(t *testing.T) {
		err := store.Delete("")
		assert.Error(t, err)
		assert.Equal(t, ErrEmptySessionID, err)
	})

	t.Run("delete non-existing session", func(t *testing.T) {
		err := store.Delete("non-existing-session-id")
		assert.NoError(t, err)
	})
}

func Test_Store_GetSessionByID(t *testing.T) {
	t.Parallel()
	// Create a new store
	store := newStore()

	t.Run("empty session ID", func(t *testing.T) {
		t.Parallel()
		sess, err := store.GetSessionByID("")
		require.Error(t, err)
		assert.Nil(t, sess)
		assert.Equal(t, ErrEmptySessionID, err)
	})

	t.Run("non-existent session ID", func(t *testing.T) {
		t.Parallel()
		sess, err := store.GetSessionByID("non-existent-session-id")
		require.Error(t, err)
		assert.Nil(t, sess)
		assert.Equal(t, ErrSessionIDNotFoundInStore, err)
	})

	t.Run("valid session ID", func(t *testing.T) {
		t.Parallel()
		// Create a new session
		ctx := fiber.New().AcquireCtx(&fasthttp.RequestCtx{})
		session, err := store.Get(ctx)
		require.NoError(t, err)

		// Save the session ID
		sessionID := session.ID()

		// Save the session
		err = session.Save()
		require.NoError(t, err)

		// Retrieve the session by ID
		retrievedSession, err := store.GetSessionByID(sessionID)
		require.NoError(t, err)
		assert.NotNil(t, retrievedSession)
		assert.Equal(t, sessionID, retrievedSession.ID())
	})
}
