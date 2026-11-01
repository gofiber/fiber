package session

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
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

// An ID from the request must not reach an entry another middleware keeps in
// the same Storage, even one whose bytes decode as session data.
//
// go test -run Test_Store_SharedStorage_ForeignRecord
func Test_Store_SharedStorage_ForeignRecord(t *testing.T) {
	t.Parallel()

	// where an earlier cache version kept a response body
	const foreignKey = "v5|GET|scheme=http|host=example.com|/files/0|q=|h=accept:0|accept-encoding:0|accept-language:0_body"

	var forged bytes.Buffer
	require.NoError(t, gob.NewEncoder(&forged).Encode(&map[any]any{"user": "admin"}))

	store := NewStore()
	require.NoError(t, store.Storage.Set(foreignKey, forged.Bytes(), 0))

	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.SetCookie(store.Extractor.Key, foreignKey)

	sess, err := store.Get(ctx)
	require.NoError(t, err)
	defer sess.Release()
	require.True(t, sess.Fresh())
	require.NotEqual(t, foreignKey, sess.ID())
	require.Nil(t, sess.Get("user"))
	require.NoError(t, sess.Save())

	_, err = store.GetByID(context.Background(), foreignKey)
	require.ErrorIs(t, err, ErrSessionIDNotFoundInStore)

	require.NoError(t, store.Delete(context.Background(), foreignKey))
	kept, err := store.Storage.Get(foreignKey)
	require.NoError(t, err)
	require.Equal(t, forged.Bytes(), kept)
}

// A session can outlive its request, so its ID must not point into the request
// buffer the connection's next request reuses.
//
// go test -run Test_Store_SessionIDOutlivesRequest
func Test_Store_SessionIDOutlivesRequest(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	store := NewStore()
	id := seedSession(t, app, store, func(s *Session) { s.Set("user", "alice") })

	loaders := map[string]func(fiber.Ctx) (*Session, error){
		"Get": store.Get,
		"GetByID": func(c fiber.Ctx) (*Session, error) {
			return store.GetByID(c, c.Cookies("session_id"))
		},
	}
	for name, load := range loaders {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetCookie("session_id", id)
			ctx := app.AcquireCtx(fctx)
			sess, err := load(ctx)
			app.ReleaseCtx(ctx)
			require.NoError(t, err)
			defer sess.Release()

			fctx.Request.Reset()
			fctx.Request.Header.SetCookie("session_id", strings.Repeat("A", len(id)))
			require.Equal(t, id, sess.ID())
		})
	}
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

// Test_Store_CachedIDsAreRecycled pins that the pooled entries of the cached IDs go
// back to the pool on request reset, which is what keeps them off the heap.
func Test_Store_CachedIDsAreRecycled(t *testing.T) {
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

	_, ok := fctx.UserValue(cachedIDsContextKey).(*cachedID)
	require.True(t, ok, "the cached ID must be stored behind a pointer, not boxed by value")

	fctx.Request.Reset()
	require.Nil(t, fctx.UserValue(cachedIDsContextKey), "the reset must drop the entry")
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

// Test_Store_CachedIDIsReused pins that a store that caches a second ID on the same
// request reuses the entry it has in the list rather than taking a fresh one from
// the pool, and that the entry stays the head fasthttp reclaims. Overwriting the local
// with another entry would strand the first outside the pool, since fasthttp only
// reclaims the one the local holds.
//
// Nothing is saved, so that both Get calls issue an ID: a saved one would be found
// the second time, and cache nothing.
func Test_Store_CachedIDIsReused(t *testing.T) {
	t.Parallel()

	store := NewStore()
	other := NewStore(Config{Extractor: extractors.FromCookie("other_sid")})
	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	sess, err := store.Get(ctx)
	require.NoError(t, err)
	require.True(t, sess.Fresh())
	first := requestIDs(ctx)
	require.NotNil(t, first)

	_, err = store.Get(ctx)
	require.NoError(t, err)
	require.Same(t, first, requestIDs(ctx), "the second ID must refill the entry, not strand it")

	_, err = other.Get(ctx)
	require.NoError(t, err)
	second := requestIDs(ctx)
	require.NotSame(t, first, second, "another store caches in an entry of its own")
	require.Same(t, first, second.next)

	_, err = store.Get(ctx)
	require.NoError(t, err)
	require.Same(t, first, requestIDs(ctx), "and the entry of the first comes back to the head, whole")
	require.Same(t, second, first.next)
	require.Nil(t, second.next, "without losing or repeating the other")
}

// Test_Store_CachedIDKeepsProvenance pins that an ID cached on the request comes
// with the provenance of the ID the request named, which says where the session is
// written back to. Losing it would leave the write-back guard with no origin for an ID
// the store already accepted.
//
// The cookie names an ID storage does not hold, which is what puts the request
// in that state: the ID resolves, then misses in storage, so a fresh one is generated
// and cached. A store that shares the cookie finds the session the first one saved
// under it, and its provenance with it.
func Test_Store_CachedIDKeepsProvenance(t *testing.T) {
	t.Parallel()

	storage := memory.New()
	store := NewStore(Config{Storage: storage})
	sharing := NewStore(Config{Storage: storage})
	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.SetCookie("session_id", "not-in-storage")

	first, err := store.Get(ctx)
	require.NoError(t, err)
	require.True(t, first.Fresh(), "an unknown ID must generate a fresh session")
	require.Equal(t, "not-in-storage", first.extractor.Value)
	require.Equal(t, extractors.SourceCookie, first.extractor.Source)
	require.NoError(t, first.Save())

	second, err := sharing.Get(ctx)
	require.NoError(t, err)
	require.False(t, second.Fresh())
	require.Equal(t, first.ID(), second.ID())
	require.Equal(t, first.extractor, second.extractor,
		"the cached ID must bring the provenance with it")
}

// Test_Store_CachedIDsDoNotLeakAcrossRequests runs requests of random Store.Get
// calls over four stores back to back on request contexts that are reset between
// them, as fasthttp does, on several goroutines at once, and checks what each
// request holds at its end: one entry for each store that issued an ID, and nothing
// from another request. An entry that went back to the pool twice, or stayed in the
// list of a request after it was reclaimed, shows up as an ID this request never
// issued, or a store twice.
func Test_Store_CachedIDsDoNotLeakAcrossRequests(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	stores := make([]*Store, 4)
	for i := range stores {
		stores[i] = NewStore(Config{Extractor: extractors.FromCookie("sid" + strconv.Itoa(i))})
	}

	const workers, requests = 4, 300
	failures := make(chan string, workers*requests)
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Go(func() {
			fctx := &fasthttp.RequestCtx{}
			c := app.AcquireCtx(fctx)
			defer app.ReleaseCtx(c)
			rng := rand.New(rand.NewPCG(uint64(worker), 7))

			for request := range requests {
				fctx.ResetUserValues() // the end of the previous request: fasthttp closes what it holds
				issued := make(map[string]*Store)
				for range 1 + rng.IntN(8) {
					store := stores[rng.IntN(len(stores))]
					sess, err := store.Get(c)
					if err != nil {
						failures <- err.Error()
						return
					}
					if sess.Fresh() {
						issued[sess.ID()] = store
					}
					if rng.IntN(2) == 0 {
						if saveErr := sess.Save(); saveErr != nil {
							failures <- saveErr.Error()
							return
						}
					}
					sess.Release()
				}

				seen := make(map[*Store]bool)
				for entry := requestIDs(c); entry != nil; entry = entry.next {
					switch {
					case seen[entry.store]:
						failures <- fmt.Sprintf("worker %d request %d: a store has two entries", worker, request)
					case issued[entry.id] != entry.store:
						failures <- fmt.Sprintf("worker %d request %d: entry %q was not issued by its store in this request", worker, request, entry.id)
					}
					seen[entry.store] = true
				}
			}
		})
	}
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}

// armedFailureStorage fails its reads once armed.
type armedFailureStorage struct {
	fiber.Storage
	err   error
	armed atomic.Bool
}

func (s *armedFailureStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	if s.armed.Load() {
		return nil, s.err
	}
	return s.Storage.GetWithContext(ctx, key)
}

// Test_Store_CachedIDReadErrorReachesCaller pins that a storage that fails when the
// store looks for the session under an ID it cached is reported, as it is for the ID
// the request names, and not taken for a session that is not there.
func Test_Store_CachedIDReadErrorReachesCaller(t *testing.T) {
	t.Parallel()

	errDown := errors.New("storage down")
	storage := &armedFailureStorage{Storage: memory.New(), err: errDown}
	store := NewStore(Config{Storage: storage})
	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	first, err := store.Get(ctx)
	require.NoError(t, err)
	defer first.Release()
	require.NoError(t, first.Save())

	storage.armed.Store(true)
	_, err = store.Get(ctx)
	require.ErrorIs(t, err, errDown)
}

// Test_CachedID_PoolHoldsSomethingElse covers the guard on what the pool hands
// back. sync.Pool is typed as any, so an entry is rebuilt rather than trusted, and a
// pool whose New answers with something else takes that branch on every Get.
//
// Putting sentinels into the shared pool would not: sync.Pool empties itself at
// GC, so a collection between the Put and the Get leaves the assertions below
// passing with the branch never reached. Not parallel, because swapping a
// package-level var is only safe while nothing else runs.
func Test_CachedID_PoolHoldsSomethingElse(t *testing.T) {
	shared := cachedIDPool
	// Pointer-like, so handing one out does not allocate.
	cachedIDPool = &sync.Pool{New: func() any { return new(struct{}) }}
	t.Cleanup(func() { cachedIDPool = shared })

	store := NewStore()
	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.SetCookie("session_id", "an-id")

	_, err := store.Get(ctx)
	require.NoError(t, err)

	cached := requestIDs(ctx)
	require.NotNil(t, cached, "a usable entry whatever the pool held")
	require.Equal(t, "an-id", cached.res.Value)
}

// A store that is asked twice, for a visitor whose cookie names no session, looks that cookie
// up once. A store over the same cookie with a storage of its own still looks it up: what one
// storage does not hold, another may.
func Test_Store_Get_AskedAgainDoesNotLookUpTheNamedIDTwice(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	storage := &keyLogStorage{Storage: memory.New()}
	store := NewStore(Config{Storage: storage})
	other := memory.New()
	holder := NewStore(Config{Storage: other, KeyGenerator: func() string { return "stale" }})
	require.Equal(t, "stale", seedSession(t, app, holder, func(s *Session) { s.Set("k", "v") }))
	sharing := NewStore(Config{Storage: other})

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Request().Header.SetCookie("session_id", "stale")

	first, err := store.Get(c)
	require.NoError(t, err)
	defer first.Release()
	second, err := store.Get(c)
	require.NoError(t, err)
	defer second.Release()
	require.True(t, first.Fresh() && second.Fresh())
	require.NotEqual(t, first.ID(), second.ID(), "an ID that was not saved is not a session")

	lookups := 0
	for _, op := range storage.seen() {
		if op == "get:session:stale" {
			lookups++
		}
	}
	require.Equal(t, 1, lookups, "the cookie named no session the first time, and is not looked up again")

	third, err := sharing.Get(c)
	require.NoError(t, err)
	defer third.Release()
	require.False(t, third.Fresh(), "the storage of another store holds the session the cookie names")
	require.Equal(t, "stale", third.ID())
}

// The PassLocalsToContext setting is read once for each app a store serves, and the store
// goes by the app, not by the last answer.
func Test_Store_PassesLocalsToContextByApp(t *testing.T) {
	t.Parallel()

	on, off := fiber.New(fiber.Config{PassLocalsToContext: true}), fiber.New()
	onCtx, offCtx := on.AcquireCtx(&fasthttp.RequestCtx{}), off.AcquireCtx(&fasthttp.RequestCtx{})
	defer on.ReleaseCtx(onCtx)
	defer off.ReleaseCtx(offCtx)

	store := NewStore()
	for range 3 {
		require.True(t, store.passesLocalsToContext(onCtx))
		require.False(t, store.passesLocalsToContext(offCtx))
	}
}
