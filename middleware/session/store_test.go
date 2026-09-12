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

		id, _ := store.getSessionID(ctx)
		require.Equal(t, expectedID, id)
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

		id, _ := store.getSessionID(ctx)
		require.Equal(t, expectedID, id)
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

		id, _ := store.getSessionID(ctx)
		require.Equal(t, expectedID, id)
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

// Test_Store_getSessionID_SkipsChildrenWithoutExtract covers a chain carrying a
// zero-value child, which Chain skips but the store used to call straight
// through, panicking on the nil function.
func Test_Store_getSessionID_SkipsChildrenWithoutExtract(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore(Config{
		Extractor: extractors.Chain(extractors.Extractor{}, extractors.FromCookie("session_id")),
	})

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.SetCookie("session_id", "abc123")

	id, _ := store.getSessionID(ctx)
	require.Equal(t, "abc123", id)
}

// Test_Store_getSessionID_WithoutExtractor covers a Store built directly rather
// than through NewStore, which would have replaced the zero-value extractor
// with the default. There is nothing to run, so there is no session ID.
func Test_Store_getSessionID_WithoutExtractor(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.SetCookie("session_id", "abc123")

	store := &Store{}
	emptyID, _ := store.getSessionID(ctx)
	require.Empty(t, emptyID)
}

// Test_Store_getSessionID_HonorsChainLevelExtract pins the contract the
// extractors package documents: "Extract set (leaf or chain): call Extract so
// legacy overrides / decoration (validation, normalization) are honored."
//
// The store used to walk Extractor.Chain itself, which called the children
// directly and never the chain-level Extract, so a decorator wrapping a chain
// — the documented way to validate or normalize an ID — was silently skipped.
// No test could catch that: extractors tested Chain, session tested its own
// walk, and both passed.
func Test_Store_getSessionID_HonorsChainLevelExtract(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	t.Run("decoration runs and children do not run twice", func(t *testing.T) {
		t.Parallel()

		var childCalls, overrideCalls int

		base := extractors.Chain(
			extractors.FromCookie("session_id"),
			extractors.FromCustom("probe", func(fiber.Ctx) (string, error) {
				childCalls++
				return "raw-id", nil
			}),
		)

		// Decorate the chain the way the package documents: keep the chain's
		// own resolution, then normalize what it produced.
		decorated := base
		decorated.Extract = func(c fiber.Ctx) (string, error) {
			overrideCalls++
			v, err := base.Extract(c)
			if err != nil {
				return "", err
			}
			return "normalized-" + v, nil
		}

		store := NewStore(Config{Extractor: decorated})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		id, from := store.getSessionID(ctx)

		require.Equal(t, "normalized-raw-id", id, "the chain-level Extract must produce the ID")
		require.Equal(t, 1, overrideCalls, "the override must run exactly once")
		require.Equal(t, 1, childCalls, "the children must run once, through the override")
		require.Equal(t, extractors.SourceCustom, from.Source, "the winning child must be reported")
		require.Equal(t, "probe", from.Key)
	})

	t.Run("a rejecting decorator refuses the id", func(t *testing.T) {
		t.Parallel()

		base := extractors.Chain(extractors.FromHeader("X-Session"))

		// The security-relevant shape: a decorator that validates, and refuses
		// an ID it does not like. Skipping it silently accepted the raw value.
		decorated := base
		decorated.Extract = func(c fiber.Ctx) (string, error) {
			v, err := base.Extract(c)
			if err != nil {
				return "", err
			}
			if v != "trusted" {
				return "", extractors.ErrNotFound
			}
			return v, nil
		}

		store := NewStore(Config{Extractor: decorated})

		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)
		ctx.Request().Header.Set("X-Session", "attacker-supplied")
		id, _ := store.getSessionID(ctx)
		require.Empty(t, id, "the validator must be able to refuse an ID")

		ok := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ok)
		ok.Request().Header.Set("X-Session", "trusted")
		acceptedID, acceptedFrom := store.getSessionID(ok)
		require.Equal(t, "trusted", acceptedID)
		require.Equal(t, extractors.SourceHeader, acceptedFrom.Source)
		require.Equal(t, "X-Session", acceptedFrom.Key)
	})
}

// Test_Store_getSessionID_ReportsWinnerForWriteBack pins that the extractor
// reported is the one that actually supplied the ID, which is what decides
// where setSession writes it back to.
func Test_Store_getSessionID_ReportsWinnerForWriteBack(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	chain := extractors.Chain(
		extractors.FromCookie("sid_cookie"),
		extractors.FromHeader("X-Sid"),
	)

	t.Run("cookie wins", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{Extractor: chain})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)
		ctx.Request().Header.SetCookie("sid_cookie", "from-cookie")

		id, from := store.getSessionID(ctx)
		require.Equal(t, "from-cookie", id)
		require.Equal(t, extractors.SourceCookie, from.Source)
		require.Equal(t, "sid_cookie", from.Key)
	})

	t.Run("header wins when the cookie is absent", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{Extractor: chain})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)
		ctx.Request().Header.Set("X-Sid", "from-header")

		id, from := store.getSessionID(ctx)
		require.Equal(t, "from-header", id)
		require.Equal(t, extractors.SourceHeader, from.Source)
		require.Equal(t, "X-Sid", from.Key)
	})

	t.Run("nothing matches", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{Extractor: chain})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		id, from := store.getSessionID(ctx)
		require.Empty(t, id)
		require.Equal(t, extractors.Result{}, from)
	})
}
