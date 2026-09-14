package session

import (
	"context"
	"errors"
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

		from, resolveErr := store.getSessionID(ctx)
		require.NoError(t, resolveErr)
		id := from.Value
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

		from, resolveErr := store.getSessionID(ctx)
		require.NoError(t, resolveErr)
		id := from.Value
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

		from, resolveErr := store.getSessionID(ctx)
		require.NoError(t, resolveErr)
		id := from.Value
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

	from, resolveErr := store.getSessionID(ctx)
	require.NoError(t, resolveErr)
	id := from.Value
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
	from, resolveErr := store.getSessionID(ctx)
	require.NoError(t, resolveErr)
	emptyID := from.Value
	require.Empty(t, emptyID)
}

// Test_Store_getSessionID_HonorsChainLevelExtract pins that a chain-level
// Extract runs. The store used to walk Extractor.Chain itself, calling the
// children directly, so a decorator wrapping a chain was silently skipped.
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

		// Value-preserving, as the store requires: the ID is written back
		// untransformed, so a decorator that rewrote it on read would never
		// find its own session again.
		decorated := base
		decorated.Extract = func(c fiber.Ctx) (string, error) {
			overrideCalls++
			v, err := base.Extract(c)
			if err != nil {
				return "", err
			}
			return v, nil
		}

		store := NewStore(Config{Extractor: decorated})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		from, resolveErr := store.getSessionID(ctx)
		require.NoError(t, resolveErr)
		id := from.Value

		require.Equal(t, "raw-id", id, "the chain-level Extract must produce the ID")
		require.Equal(t, 1, overrideCalls, "the override must run exactly once")
		require.Equal(t, 1, childCalls, "the children must run once, through the override")
		require.Equal(t, extractors.SourceCustom, from.Source, "the winning child must be reported")
		require.Equal(t, "probe", from.Key)
	})

	t.Run("a rejecting decorator refuses the id", func(t *testing.T) {
		t.Parallel()

		base := extractors.Chain(extractors.FromHeader("X-Session"))

		// A validator refusing an ID it does not like. Skipping it silently
		// accepted the raw value.
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
		from, resolveErr := store.getSessionID(ctx)
		require.NoError(t, resolveErr)
		id := from.Value
		require.Empty(t, id, "the validator must be able to refuse an ID")

		ok := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ok)
		ok.Request().Header.Set("X-Session", "trusted")
		acceptedFrom, resolveErr := store.getSessionID(ok)
		require.NoError(t, resolveErr)
		acceptedID := acceptedFrom.Value
		require.Equal(t, "trusted", acceptedID)
		require.Equal(t, extractors.SourceHeader, acceptedFrom.Source)
		require.Equal(t, "X-Session", acceptedFrom.Key)
	})
}

// Test_Store_getSessionID_ReportsWinnerForWriteBack pins that the reported
// extractor is the one that supplied the ID, which decides where it is written
// back.
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

		from, resolveErr := store.getSessionID(ctx)
		require.NoError(t, resolveErr)
		id := from.Value
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

		from, resolveErr := store.getSessionID(ctx)
		require.NoError(t, resolveErr)
		id := from.Value
		require.Equal(t, "from-header", id)
		require.Equal(t, extractors.SourceHeader, from.Source)
		require.Equal(t, "X-Sid", from.Key)
	})

	t.Run("nothing matches", func(t *testing.T) {
		t.Parallel()
		store := NewStore(Config{Extractor: chain})
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		from, resolveErr := store.getSessionID(ctx)
		require.NoError(t, resolveErr)
		id := from.Value
		require.Empty(t, id)
		require.Equal(t, extractors.Result{}, from)
	})
}

// Test_Store_UnattributableID_IsNotWrittenBack pins the session-fixation guard
// against provenance that was never observed. A chain-level Extract answering
// on its own records no child, so Resolve reports the chain's declared metadata
// — its first child. Reading that as provenance would classify a query-supplied
// ID as "came from a writable sink" and pin the victim to an attacker-chosen
// session.
func Test_Store_UnattributableID_IsNotWrittenBack(t *testing.T) {
	t.Parallel()

	base := extractors.Chain(
		extractors.FromCookie("sid"), // declared metadata: cookie, a writable sink
		extractors.FromQuery("sid"),
	)
	attackable := base
	attackable.Extract = func(c fiber.Ctx) (string, error) {
		return fiber.Query[string](c, "sid"), nil // reads the query, delegates to no child
	}

	store := NewStore(Config{Extractor: attackable})
	app := fiber.New()

	// Seed a real session, so the ID the attacker plants names an existing one.
	seed := app.AcquireCtx(&fasthttp.RequestCtx{})
	seeded, err := store.Get(seed)
	require.NoError(t, err)
	plantedID := seeded.ID()
	require.NoError(t, seeded.Save())
	app.ReleaseCtx(seed)

	// The victim's request carries that ID in the query and no cookie.
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().SetRequestURI("/p?sid=" + plantedID)

	sess, err := store.Get(ctx)
	require.NoError(t, err)
	require.False(t, sess.Fresh(), "the planted ID should load the existing session")
	require.NoError(t, sess.Save())

	require.NotContains(t, string(ctx.Response().Header.Peek(fiber.HeaderSetCookie)), plantedID,
		"an ID whose origin cannot be attributed must never be pinned into a cookie")
}

// Test_Store_ForeignChainDoesNotSupplyProvenance pins that a leaf consulting
// some other chain is not credited with that chain's winner, which would let
// the session ID be written into an unrelated application cookie.
func Test_Store_ForeignChainDoesNotSupplyProvenance(t *testing.T) {
	t.Parallel()

	tenant := extractors.Chain(extractors.FromHeader("X-Tenant"), extractors.FromCookie("tenant"))
	leaf := extractors.FromCustom("sid", func(c fiber.Ctx) (string, error) {
		//nolint:errcheck // the lookup's outcome is irrelevant; it runs only so a foreign chain records a winner
		tenant.Extract(c) // an unrelated lookup, in the same Resolve frame
		return fiber.Query[string](c, "sid"), nil
	})

	store := NewStore(Config{Extractor: leaf})
	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().SetRequestURI("/p?sid=planted")
	ctx.Request().Header.SetCookie("tenant", "acme")

	from, resolveErr := store.getSessionID(ctx)
	require.NoError(t, resolveErr)
	id := from.Value
	require.Equal(t, "planted", id)
	require.Equal(t, "sid", from.Key, "the leaf must not inherit the tenant chain's winner")
	require.Equal(t, extractors.SourceCustom, from.Source)
}

// Test_Store_NestedChainCookieSinkIsWritten pins that a cookie extractor nested
// inside an inner chain is still written back. getExtractorInfo used to range
// over direct children and judge each by its static Source, so an inner
// Chain(query, cookie) looked like a query extractor: no Set-Cookie was emitted
// and every request began a session that could never be resumed.
func Test_Store_NestedChainCookieSinkIsWritten(t *testing.T) {
	t.Parallel()

	store := NewStore(Config{
		Extractor: extractors.Chain(
			extractors.Chain(
				extractors.FromQuery("session_id"),  // first child: a read-only source
				extractors.FromCookie("session_id"), // the sink that must still be found
			),
		),
	})

	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	sess, err := store.Get(ctx)
	require.NoError(t, err)
	require.True(t, sess.Fresh())
	require.NoError(t, sess.Save())

	require.Contains(t, string(ctx.Response().Header.Peek(fiber.HeaderSetCookie)), sess.ID(),
		"a cookie sink nested one level down must still be written")
}

// Test_Store_ExtractorErrorReachesCaller pins that a validator's refusal is
// reported rather than collapsed into "no session ID present" — a forged ID and
// a first-time visitor must not look alike.
func Test_Store_ExtractorErrorReachesCaller(t *testing.T) {
	t.Parallel()

	errForged := errors.New("forged session id")

	base := extractors.Chain(extractors.FromCookie("session_id"))
	validating := base
	validating.Extract = func(c fiber.Ctx) (string, error) {
		v, err := base.Extract(c)
		if err != nil {
			return "", err // absent: still an ordinary ErrNotFound
		}
		if v != "trusted" {
			return "", errForged
		}
		return v, nil
	}

	store := NewStore(Config{Extractor: validating})
	app := fiber.New()

	t.Run("a refusal is reported", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)
		ctx.Request().Header.SetCookie("session_id", "forged-value")

		_, err := store.Get(ctx)
		require.ErrorIs(t, err, errForged, "the validator's rejection must reach the caller")
	})

	t.Run("an absent id is not an error", func(t *testing.T) {
		t.Parallel()
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		sess, err := store.Get(ctx)
		require.NoError(t, err, "a first-time visitor must not look like a failure")
		require.True(t, sess.Fresh())
	})
}

// Test_Store_ProvenanceBoxIsRecycled pins that the pooled provenance box goes
// back to the pool on request reset, which is what keeps it off the heap.
func Test_Store_ProvenanceBoxIsRecycled(t *testing.T) {
	t.Parallel()

	store := NewStore()
	app := fiber.New()
	app.Get("/t", func(c fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}
		return c.SendString(sess.ID())
	})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/t")
	fctx.Request.Header.SetCookie("session_id", "some-id")
	app.Handler()(fctx)

	_, ok := fctx.UserValue(sessionExtractorContextKey).(*provenanceBox)
	require.True(t, ok, "the provenance must be stored behind a pointer, not boxed by value")

	fctx.Request.Reset()
	require.Nil(t, fctx.UserValue(sessionExtractorContextKey), "the reset must drop the box")
}

// Test_Store_KeylessExtractorStillGuarded pins that the fixation guard keys off
// whether an ID arrived with the request, not off Key. A keyless child —
// FromCustom(""), or a hand-rolled one — used to skip the guard entirely and let
// a read-only ID reach the configured cookie sink.
func Test_Store_KeylessExtractorStillGuarded(t *testing.T) {
	t.Parallel()

	store := NewStore(Config{Extractor: extractors.Chain(
		extractors.FromCustom("", func(c fiber.Ctx) (string, error) {
			return fiber.Query[string](c, "sid"), nil
		}),
		extractors.FromCookie("sid"),
	)})

	app := fiber.New()
	seed := app.AcquireCtx(&fasthttp.RequestCtx{})
	seeded, err := store.Get(seed)
	require.NoError(t, err)
	plantedID := seeded.ID()
	require.NoError(t, seeded.Save())
	app.ReleaseCtx(seed)

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().SetRequestURI("/p?sid=" + plantedID)

	sess, err := store.Get(ctx)
	require.NoError(t, err)
	require.False(t, sess.Fresh(), "the planted ID should load the existing session")
	require.NoError(t, sess.Save())

	require.NotContains(t, string(ctx.Response().Header.Peek(fiber.HeaderSetCookie)), plantedID,
		"a read-only ID must not be pinned into a cookie just because its extractor has no Key")
}

// Test_Store_ProvenanceBoxIsReused pins that a second getSession on the same
// request updates the box already in the request rather than taking a fresh one
// from the pool. Overwriting the local would strand the first box outside the
// pool, since fasthttp only reclaims the one the local holds — and a repeat
// getSession is the case the box exists for.
//
// The cookie carries a saved session's ID so that both Get calls resolve it
// from storage. An unknown ID would make the first Get generate a fresh one and
// cache it, and the second would then read the cached ID and never reach
// storeProvenance at all.
func Test_Store_ProvenanceBoxIsReused(t *testing.T) {
	t.Parallel()

	store := NewStore()
	app := fiber.New()

	seed := app.AcquireCtx(&fasthttp.RequestCtx{})
	seeded, err := store.Get(seed)
	require.NoError(t, err)
	require.NoError(t, seeded.Save())
	savedID := seeded.ID()
	app.ReleaseCtx(seed)

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.SetCookie("session_id", savedID)

	sess, err := store.Get(ctx)
	require.NoError(t, err)
	require.False(t, sess.Fresh(), "the saved ID must resolve from storage, not generate a new one")
	first, ok := ctx.Locals(sessionExtractorContextKey).(*provenanceBox)
	require.True(t, ok)

	_, err = store.Get(ctx)
	require.NoError(t, err)
	second, ok := ctx.Locals(sessionExtractorContextKey).(*provenanceBox)
	require.True(t, ok)

	require.Same(t, first, second, "the second resolve must refill the box, not strand it")
}

// Test_ProvenanceBox_PoolHoldsSomethingElse covers the guard on what the pool
// hands back. sync.Pool is typed as any, so a box is rebuilt rather than
// trusted; this drives that branch by putting sentinels in front of it.
//
// Not parallel: it borrows the shared pool, and it takes back out everything
// it puts in.
func Test_ProvenanceBox_PoolHoldsSomethingElse(t *testing.T) {
	store := NewStore()
	app := fiber.New()

	const sentinels = 8
	for range sentinels {
		// Pointer-like, so parking it in the pool does not allocate.
		provenancePool.Put(new(struct{}))
	}

	for range sentinels {
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		ctx.Request().Header.SetCookie("session_id", "an-id")

		_, err := store.Get(ctx)
		require.NoError(t, err)

		box, ok := ctx.Locals(sessionExtractorContextKey).(*provenanceBox)
		require.True(t, ok, "a usable box whatever the pool held")
		require.Equal(t, "an-id", box.res.Value)

		app.ReleaseCtx(ctx)
	}
}
