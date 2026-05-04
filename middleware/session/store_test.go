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

// go test -run Test_Store_resolveSessionID
func Test_Store_resolveSessionID(t *testing.T) {
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

		info := store.resolveSessionID(ctx)
		require.Equal(t, expectedID, info.id)
		require.Equal(t, extractors.SourceCookie, info.source)
		require.True(t, info.source.IsWritable(), "cookie source should be writable")
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

		info := store.resolveSessionID(ctx)
		require.Equal(t, expectedID, info.id)
		require.Equal(t, extractors.SourceHeader, info.source)
		require.True(t, info.source.IsWritable(), "header source should be writable")
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

		info := store.resolveSessionID(ctx)
		require.Equal(t, expectedID, info.id)
		require.Equal(t, extractors.SourceQuery, info.source)
		require.False(t, info.source.IsWritable(), "query source should not be writable")
	})

	t.Run("chain reports source of value-providing extractor", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{
			Extractor: extractors.Chain(
				extractors.FromCookie("session_id"),
				extractors.FromQuery("session_id"),
			),
		})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// Cookie absent → query wins; source must reflect query, not the
		// chain wrapper's primary source.
		ctx.Request().SetRequestURI("/path?session_id=" + expectedID)

		info := store.resolveSessionID(ctx)
		require.Equal(t, expectedID, info.id)
		require.Equal(t, extractors.SourceQuery, info.source)
		require.False(t, info.source.IsWritable())
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

	// Regression: https://github.com/gofiber/fiber/issues/4234
	// Without TrustClientSessionID, an unknown ID from a read-only source
	// (query/form/param/custom) must be discarded — same fixation protection
	// as the cookie/header default.
	t.Run("query extractor: client ID is rejected by default", func(t *testing.T) {
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

		require.NotEqual(t, clientID, acquiredSession.ID(),
			"client-supplied query ID must not be trusted without TrustClientSessionID")
		require.True(t, acquiredSession.Fresh())
	})

	// Regression: https://github.com/gofiber/fiber/issues/4234
	// With TrustClientSessionID + a permissive validator, the query-supplied
	// ID is preserved and persists across requests.
	t.Run("query extractor: opt-in preserves provided ID", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{
			Extractor:                extractors.FromQuery("session_id"),
			TrustClientSessionID:     true,
			ClientSessionIDValidator: func(string) bool { return true },
		})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		clientID := "client-chosen-session-id"
		ctx.Request().SetRequestURI("/path?session_id=" + clientID)

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)
		defer acquiredSession.Release()

		require.Equal(t, clientID, acquiredSession.ID())
		require.True(t, acquiredSession.Fresh())
	})

	t.Run("query extractor: opt-in but validator rejects → fall back to generated ID", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{
			Extractor:                extractors.FromQuery("session_id"),
			TrustClientSessionID:     true,
			ClientSessionIDValidator: func(string) bool { return false },
		})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		clientID := "client-chosen-session-id"
		ctx.Request().SetRequestURI("/path?session_id=" + clientID)

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)
		defer acquiredSession.Release()

		require.NotEqual(t, clientID, acquiredSession.ID())
	})

	t.Run("query extractor: opt-in without validator is rejected", func(t *testing.T) {
		t.Parallel()
		// TrustClientSessionID alone (nil validator) must NOT trust the client
		// ID — fail closed.
		store := NewStore(Config{
			Extractor:            extractors.FromQuery("session_id"),
			TrustClientSessionID: true,
		})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		clientID := "client-chosen-session-id"
		ctx.Request().SetRequestURI("/path?session_id=" + clientID)

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)
		defer acquiredSession.Release()

		require.NotEqual(t, clientID, acquiredSession.ID())
	})

	// Roundtrip: opt-in + valid client ID persists data across requests.
	t.Run("query extractor: opt-in roundtrip persists data across requests", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{
			Extractor:                extractors.FromQuery("session_id"),
			TrustClientSessionID:     true,
			ClientSessionIDValidator: func(string) bool { return true },
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
		// Chain with cookie first: an unknown cookie ID must still be discarded
		// to prevent session fixation attacks, regardless of TrustClientSessionID.
		store := NewStore(Config{
			Extractor: extractors.Chain(
				extractors.FromCookie("session_id"),
				extractors.FromQuery("session_id"),
			),
			TrustClientSessionID:     true,
			ClientSessionIDValidator: func(string) bool { return true },
		})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		untrustedID := "untrusted-cookie-session-id"
		ctx.Request().Header.SetCookie("session_id", untrustedID)

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)
		defer acquiredSession.Release()

		require.NotEqual(t, untrustedID, acquiredSession.ID())
	})

	t.Run("chain extractor: opt-in query fallback preserves ID when cookie absent", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{
			Extractor: extractors.Chain(
				extractors.FromCookie("session_id"),
				extractors.FromQuery("session_id"),
			),
			TrustClientSessionID:     true,
			ClientSessionIDValidator: func(string) bool { return true },
		})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		queryID := "query-provided-session-id"
		ctx.Request().SetRequestURI("/path?session_id=" + queryID)

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)
		defer acquiredSession.Release()

		require.Equal(t, queryID, acquiredSession.ID())
		require.True(t, acquiredSession.Fresh())
	})

	// Regression: two Store.Get calls in the same request must return the
	// same ID — covers the chain case where source decision was previously
	// re-derived from the wrapper instead of being cached.
	t.Run("chain extractor: two Get calls in same request return same query ID", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{
			Extractor: extractors.Chain(
				extractors.FromCookie("session_id"),
				extractors.FromQuery("session_id"),
			),
			TrustClientSessionID:     true,
			ClientSessionIDValidator: func(string) bool { return true },
		})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		queryID := "stable-query-id"
		ctx.Request().SetRequestURI("/path?session_id=" + queryID)

		sess1, err := store.Get(ctx)
		require.NoError(t, err)
		// Note: not calling Release; releasing returns the session to the pool
		// and clears its ID.
		require.Equal(t, queryID, sess1.ID())

		// Second call within the same request must return the same ID, even
		// without an intervening Save().
		sess2, err := store.Get(ctx)
		require.NoError(t, err)
		defer sess2.Release()

		require.Equal(t, queryID, sess2.ID())
		require.Equal(t, sess1.ID(), sess2.ID())
		sess1.Release()
	})

	// Two Get calls in same request with cookie source generate the same ID
	// after regeneration — guards against the regenerate-loop bug.
	t.Run("cookie extractor: two Get calls return same regenerated ID", func(t *testing.T) {
		t.Parallel()
		store := NewStore()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		ctx.Request().Header.SetCookie(store.Extractor.Key, unexpectedID)

		sess1, err := store.Get(ctx)
		require.NoError(t, err)
		first := sess1.ID()
		require.NotEqual(t, unexpectedID, first)

		sess2, err := store.Get(ctx)
		require.NoError(t, err)
		defer sess2.Release()

		require.Equal(t, first, sess2.ID(),
			"second Get within same request must return the same regenerated ID")
		sess1.Release()
	})

	// Empty client-supplied query ID falls through to server generation
	// instead of being persisted under the empty string.
	t.Run("query extractor: empty client ID is regenerated", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{
			Extractor:                extractors.FromQuery("session_id"),
			TrustClientSessionID:     true,
			ClientSessionIDValidator: func(string) bool { return true },
		})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		ctx.Request().SetRequestURI("/path?session_id=")

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)
		defer acquiredSession.Release()

		require.NotEmpty(t, acquiredSession.ID())
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

	// First request: create and persist a session.
	ctx1 := app.AcquireCtx(&fasthttp.RequestCtx{})
	session, err := store.Get(ctx1)
	require.NoError(t, err)
	require.NoError(t, session.Save())
	sessionID := session.ID()
	session.Release()
	app.ReleaseCtx(ctx1)

	// Delete the session out of band.
	require.NoError(t, store.Delete(t.Context(), sessionID))

	// Second request presents the same cookie. Storage no longer holds the
	// data, so the cookie ID must be discarded and a fresh ID generated to
	// prevent fixation.
	ctx2 := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx2)
	ctx2.Request().Header.SetCookie(store.Extractor.Key, sessionID)

	session2, err := store.Get(ctx2)
	require.NoError(t, err)
	defer session2.Release()

	require.NotEqual(t, sessionID, session2.ID(),
		"deleted session ID must be regenerated on the next request")
	require.True(t, session2.Fresh())
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
