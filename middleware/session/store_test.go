package session

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// sessionTestResolve returns the session ID that store reads from ctx and where
// it came from, failing the test if the extractor errors.
func sessionTestResolve(t *testing.T, store *Store, ctx fiber.Ctx) extractors.Result {
	t.Helper()

	from, err := store.getSessionID(ctx)
	require.NoError(t, err)
	return from
}

func Test_Store_getSessionID(t *testing.T) {
	t.Parallel()
	expectedID := "test-session-id"
	app := fiber.New()
	plantCookie := func(ctx fiber.Ctx, _ string) { ctx.Request().Header.SetCookie("session_id", "abc123") }
	skipping := extractors.Chain(extractors.Extractor{}, extractors.FromCookie("session_id"))

	for _, tc := range []struct {
		store *Store
		plant func(ctx fiber.Ctx, key string) // puts the ID where the store looks for it; key is the extractor key
		name  string
		want  string
	}{
		{
			name:  "from cookie",
			store: NewStore(),
			plant: func(ctx fiber.Ctx, key string) { ctx.Request().Header.SetCookie(key, expectedID) },
			want:  expectedID,
		},
		{
			name:  "from header",
			store: NewStore(Config{Extractor: extractors.FromHeader("session_id")}),
			plant: func(ctx fiber.Ctx, key string) { ctx.Request().Header.Set(key, expectedID) },
			want:  expectedID,
		},
		{
			name:  "from url query",
			store: NewStore(Config{Extractor: extractors.FromQuery("session_id")}),
			plant: func(ctx fiber.Ctx, key string) {
				ctx.Request().SetRequestURI(fmt.Sprintf("/path?%s=%s", key, expectedID))
			},
			want: expectedID,
		},
		// A chain carrying a zero-value child, which Chain skips but the store used
		// to call straight through, panicking on the nil function.
		{
			name:  "skips children without Extract",
			store: NewStore(Config{Extractor: skipping}),
			plant: plantCookie,
			want:  "abc123",
		},
		// A Store built directly rather than through NewStore, which would have
		// replaced the zero-value extractor with the default. There is nothing to
		// run, so there is no session ID.
		{name: "without extractor", store: &Store{}, plant: plantCookie},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := apptest.Ctx(t, app)
			tc.plant(ctx, tc.store.Extractor.Key)

			from := sessionTestResolve(t, tc.store, ctx)
			require.Equal(t, tc.want, from.Value)
		})
	}
}

// Regression: https://github.com/gofiber/fiber/issues/1408
// Regression: https://github.com/gofiber/fiber/security/advisories/GHSA-98j2-3j3p-fw2v
func Test_Store_Get(t *testing.T) {
	t.Parallel()
	unexpectedID := "test-session-id"
	app := fiber.New()
	t.Run("session should be re-generated if it is invalid", func(t *testing.T) {
		t.Parallel()
		store := NewStore()
		ctx := apptest.Ctx(t, app)
		ctx.Request().Header.SetCookie(store.Extractor.Key, unexpectedID)

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)

		require.NotEqual(t, unexpectedID, acquiredSession.ID())
	})
}

// An ID from the request must not reach an entry another middleware keeps in
// the same Storage, even one whose bytes decode as session data.
func Test_Store_SharedStorage_ForeignRecord(t *testing.T) {
	t.Parallel()
	// where an earlier cache version kept a response body
	const foreignKey = "v5|GET|scheme=http|host=example.com|/files/0|q=|h=accept:0|accept-encoding:0|accept-language:0_body"

	var forged bytes.Buffer
	require.NoError(t, gob.NewEncoder(&forged).Encode(&map[any]any{"user": "admin"}))

	store := NewStore()
	require.NoError(t, store.Storage.Set(foreignKey, forged.Bytes(), 0))

	sess := loadSession(t, fiber.New(), store, foreignKey)
	require.True(t, sess.Fresh())
	require.NotEqual(t, foreignKey, sess.ID())
	require.Nil(t, sess.Get("user"))
	require.NoError(t, sess.Save())

	sessionTestRequireGone(t, store, foreignKey)

	require.NoError(t, store.Delete(context.Background(), foreignKey))
	kept, err := store.Storage.Get(foreignKey)
	require.NoError(t, err)
	require.Equal(t, forged.Bytes(), kept)
}

// A session can outlive its request, so its ID must not point into the request
// buffer the connection's next request reuses.
func Test_Store_SessionIDOutlivesRequest(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	store := NewStore()
	id := seedSession(t, app, store, func(s *Session) { s.Set("user", "alice") })

	loaders := map[string]func(fiber.Ctx) (*Session, error){
		"Get":     store.Get,
		"GetByID": func(c fiber.Ctx) (*Session, error) { return store.GetByID(c, c.Cookies("session_id")) },
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

func Test_Store_DeleteSession(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	store := NewStore()
	ctx := apptest.Ctx(t, app)

	session, err := store.Get(ctx)
	require.NoError(t, err)
	sessionID := session.ID()
	require.NoError(t, store.Delete(ctx, sessionID))

	session, err = store.Get(ctx)
	require.NoError(t, err)

	// The session ID should be different now, because the old session was deleted
	require.NotEqual(t, sessionID, session.ID())
}

func TestStore_Get_SessionAlreadyLoaded(t *testing.T) {
	app := fiber.New()
	ctx := apptest.Ctx(t, app)

	// Mock middleware and set it in the context
	middleware := &Middleware{}
	ctx.Locals(middlewareContextKey, middleware)

	store := &Store{}
	sess, err := store.Get(ctx)
	require.Nil(t, sess)
	require.Equal(t, ErrSessionAlreadyLoadedByMiddleware, err)
}

func TestStore_Delete(t *testing.T) {
	store := NewStore()

	t.Run("delete with empty session ID", func(t *testing.T) {
		err := store.Delete(context.Background(), "")
		require.Error(t, err)
		require.Equal(t, ErrEmptySessionID, err)
	})

	t.Run("delete non-existing session", func(t *testing.T) {
		require.NoError(t, store.Delete(context.Background(), "non-existing-session-id"))
	})
}

func Test_Store_GetByID(t *testing.T) {
	t.Parallel()
	store := NewStore()

	for _, tc := range []struct {
		err  error
		name string
		id   string
	}{
		{name: "empty session ID", err: ErrEmptySessionID},
		{name: "nonexistent session ID", id: "nonexistent-session-id", err: ErrSessionIDNotFoundInStore},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sess, err := store.GetByID(context.Background(), tc.id)
			require.Error(t, err)
			require.Nil(t, sess)
			require.Equal(t, tc.err, err)
		})
	}

	t.Run("valid session ID", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, fiber.New())
		session, err := store.Get(ctx)
		defer session.Release()
		require.NoError(t, err)
		sessionID := session.ID()
		require.NoError(t, session.Save())

		retrievedSession, err := store.GetByID(context.Background(), sessionID)
		require.NoError(t, err)
		require.NotNil(t, retrievedSession)
		require.Equal(t, sessionID, retrievedSession.ID())

		retrievedSession.Set("key", "value")
		require.NoError(t, retrievedSession.Save())
		require.Equal(t, "value", retrievedSession.Get("key"))
		require.False(t, retrievedSession.Fresh())

		require.NoError(t, retrievedSession.Reset())
		require.NoError(t, retrievedSession.Destroy())
		require.IsType(t, []any{}, retrievedSession.Keys())
		require.NoError(t, retrievedSession.Regenerate())
		require.NotPanics(t, func() { retrievedSession.Release() })
	})
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
		ctx := apptest.Ctx(t, app)

		from := sessionTestResolve(t, store, ctx)

		require.Equal(t, "raw-id", from.Value, "the chain-level Extract must produce the ID")
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

		ctx := apptest.Ctx(t, app)
		ctx.Request().Header.Set("X-Session", "attacker-supplied")
		from := sessionTestResolve(t, store, ctx)
		require.Empty(t, from.Value, "the validator must be able to refuse an ID")

		ok := apptest.Ctx(t, app)
		ok.Request().Header.Set("X-Session", "trusted")
		acceptedFrom := sessionTestResolve(t, store, ok)
		require.Equal(t, "trusted", acceptedFrom.Value)
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
	chain := extractors.Chain(extractors.FromCookie("sid_cookie"), extractors.FromHeader("X-Sid"))

	for _, tc := range []struct {
		plant func(ctx fiber.Ctx) // puts the request in the state under test; nil leaves it blank
		name  string
		want  extractors.Result
	}{
		{
			name:  "cookie wins",
			plant: func(ctx fiber.Ctx) { ctx.Request().Header.SetCookie("sid_cookie", "from-cookie") },
			want:  extractors.Result{Value: "from-cookie", Key: "sid_cookie", Source: extractors.SourceCookie, Resolved: true},
		},
		{
			name:  "header wins when the cookie is absent",
			plant: func(ctx fiber.Ctx) { ctx.Request().Header.Set("X-Sid", "from-header") },
			want:  extractors.Result{Value: "from-header", Key: "X-Sid", Source: extractors.SourceHeader, Resolved: true},
		},
		{name: "nothing matches", want: extractors.Result{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := NewStore(Config{Extractor: chain})
			ctx := apptest.Ctx(t, app)
			if tc.plant != nil {
				tc.plant(ctx)
			}

			require.Equal(t, tc.want, sessionTestResolve(t, store, ctx))
		})
	}
}

// Test_Store_FixationGuard pins the session-fixation guard: an ID that reaches a
// store through a read-only source loads the existing session it names, and
// saving that session must not pin the ID into a cookie, whichever way the
// extractor hides where the ID came from.
func Test_Store_FixationGuard(t *testing.T) {
	t.Parallel()
	// A chain-level Extract answering on its own records no child, so Resolve
	// reports the chain's declared metadata — its first child. Reading that as
	// provenance would classify a query-supplied ID as "came from a writable
	// sink" and pin the victim to an attacker-chosen session.
	base := extractors.Chain(
		extractors.FromCookie("sid"), // declared metadata: cookie, a writable sink
		extractors.FromQuery("sid"),
	)
	attackable := base
	attackable.Extract = func(c fiber.Ctx) (string, error) {
		return fiber.Query[string](c, "sid"), nil // reads the query, delegates to no child
	}

	// The guard keys off whether an ID arrived with the request, not off Key. A
	// keyless child — FromCustom(""), or a hand-rolled one — used to skip the
	// guard entirely and let a read-only ID reach the configured cookie sink.
	keyless := extractors.Chain(
		extractors.FromCustom("", func(c fiber.Ctx) (string, error) { return fiber.Query[string](c, "sid"), nil }),
		extractors.FromCookie("sid"),
	)

	for _, tc := range []struct {
		name      string
		msg       string
		extractor extractors.Extractor
	}{
		{
			name:      "unattributable ID",
			extractor: attackable,
			msg:       "an ID whose origin cannot be attributed must never be pinned into a cookie",
		},
		{
			name:      "keyless extractor",
			extractor: keyless,
			msg:       "a read-only ID must not be pinned into a cookie just because its extractor has no Key",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := NewStore(Config{Extractor: tc.extractor})
			app := fiber.New()

			// Seed a real session, so the ID the attacker plants names an existing one.
			plantedID := seedSession(t, app, store, nil)

			// The victim's request carries that ID in the query and no cookie.
			ctx := apptest.Ctx(t, app)
			ctx.Request().SetRequestURI("/p?sid=" + plantedID)

			sess, err := store.Get(ctx)
			require.NoError(t, err)
			require.False(t, sess.Fresh(), "the planted ID should load the existing session")
			require.NoError(t, sess.Save())

			require.NotContains(t, string(ctx.Response().Header.Peek(fiber.HeaderSetCookie)), plantedID, tc.msg)
		})
	}
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
	ctx := apptest.Ctx(t, fiber.New())
	ctx.Request().SetRequestURI("/p?sid=planted")
	ctx.Request().Header.SetCookie("tenant", "acme")

	from := sessionTestResolve(t, store, ctx)
	require.Equal(t, "planted", from.Value)
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

	ctx := apptest.Ctx(t, fiber.New())

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
		ctx := apptest.Ctx(t, app)
		ctx.Request().Header.SetCookie("session_id", "forged-value")

		_, err := store.Get(ctx)
		require.ErrorIs(t, err, errForged, "the validator's rejection must reach the caller")
	})

	t.Run("an absent id is not an error", func(t *testing.T) {
		t.Parallel()
		ctx := apptest.Ctx(t, app)

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

	savedID := seedSession(t, app, store, nil)

	ctx := apptest.Ctx(t, app)
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

// Test_Store_CachedIDKeepsProvenance pins the other side of the box: once an ID
// has been cached on the request, a second getSession skips extraction
// entirely, and the provenance has to come back from the box instead. Losing it
// would leave the write-back guard with no origin for an ID it already
// accepted.
//
// The cookie names an ID storage does not hold, which is what puts the request
// in that state — the ID resolves and is recorded, then misses in storage, so a
// fresh one is generated and cached.
func Test_Store_CachedIDKeepsProvenance(t *testing.T) {
	t.Parallel()
	store := NewStore()
	ctx := apptest.Ctx(t, fiber.New())
	ctx.Request().Header.SetCookie("session_id", "not-in-storage")

	first, err := store.Get(ctx)
	require.NoError(t, err)
	require.True(t, first.Fresh(), "an unknown ID must generate a fresh session")
	require.Equal(t, "not-in-storage", first.extractor.Value)
	require.Equal(t, extractors.SourceCookie, first.extractor.Source)

	_, cached := ctx.Locals(sessionIDContextKey).(string)
	require.True(t, cached, "the fresh ID is what sends the next resolve down the cached path")

	second, err := store.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, first.extractor, second.extractor, "the cached path must recover the provenance from the box")
}

// Test_ProvenanceBox_PoolHoldsSomethingElse covers the guard on what the pool
// hands back. sync.Pool is typed as any, so a box is rebuilt rather than
// trusted, and a pool whose New answers with something else takes that branch
// on every Get.
//
// Putting sentinels into the shared pool would not: sync.Pool empties itself at
// GC, so a collection between the Put and the Get leaves the assertions below
// passing with the branch never reached. Not parallel, because swapping a
// package-level var is only safe while nothing else runs.
func Test_ProvenanceBox_PoolHoldsSomethingElse(t *testing.T) {
	shared := provenancePool
	// Pointer-like, so handing one out does not allocate.
	provenancePool = &sync.Pool{New: func() any { return new(struct{}) }}
	t.Cleanup(func() { provenancePool = shared })

	store := NewStore()
	ctx := apptest.Ctx(t, fiber.New())
	ctx.Request().Header.SetCookie("session_id", "an-id")

	_, err := store.Get(ctx)
	require.NoError(t, err)

	box, ok := ctx.Locals(sessionExtractorContextKey).(*provenanceBox)
	require.True(t, ok, "a usable box whatever the pool held")
	require.Equal(t, "an-id", box.res.Value)
}
