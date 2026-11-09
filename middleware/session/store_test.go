package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/internal/storagetest"
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
// buffer the connection's next request reuses, whatever the key it is stored under is made of:
// the ID is the end of the key, or, with HashStorageKeys, a copy.
func Test_Store_SessionIDOutlivesRequest(t *testing.T) {
	t.Parallel()
	for _, policy := range storageKeyPolicies {
		t.Run(policy.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			store := NewStore(Config{StorageKeyPrefix: policy.prefix, HashStorageKeys: policy.hash})
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

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
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

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
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

// Test_Store_GetByID_StorageErrors: a storage that fails to read is reported, and one that fails to delete a
// session past its deadline still has GetByID say that the session is gone.
func Test_Store_GetByID_StorageErrors(t *testing.T) {
	t.Parallel()
	errBoom := errors.New("boom")
	raw, err := encodeRecord("json", deadlineOf(time.Now().Add(-time.Minute)), []recordEntry{{key: "k", value: []byte(`"v"`)}})
	require.NoError(t, err)

	for _, tc := range []struct {
		want     error
		name, op string
	}{
		{name: "read fails", op: storagetest.OpGet, want: errBoom},
		{name: "delete of the expired session fails", op: storagetest.OpDelete, want: ErrSessionIDNotFoundInStore},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mem := storagetest.New()
			store := NewStore(Config{Storage: mem, Codec: JSONCodec{}})
			require.NoError(t, mem.Set(string(store.storageKey("id")), raw, time.Hour))
			mem.SetErr(tc.op, "*", errBoom)

			_, err := store.GetByID(context.Background(), "id")
			require.ErrorIs(t, err, tc.want)
		})
	}
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

// Test_Store_CachedIDsDoNotLeakAcrossRequests runs requests of random Store.Get calls over four
// stores back to back on request contexts that are reset between them, as fasthttp does, on several
// goroutines at once, and checks what each request holds at its end: one entry for each store that
// issued an ID, and nothing from another request. An entry that went back to the pool twice, or
// stayed in the list of a request after it was reclaimed, shows up as an ID this request never
// issued, or a store twice.
func Test_Store_CachedIDsDoNotLeakAcrossRequests(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	stores := make([]*Store, 4)
	for i := range stores {
		stores[i] = NewStore(Config{Extractor: extractors.FromCookie("sid" + strconv.Itoa(i))})
	}

	const workers, requests = 4, 300
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
						t.Error(err)
						return
					}
					if sess.Fresh() {
						issued[sess.ID()] = store
					}
					if rng.IntN(2) == 0 {
						if saveErr := sess.Save(); saveErr != nil {
							t.Error(saveErr)
							return
						}
					}
					sess.Release()
				}

				seen := make(map[*Store]bool)
				for entry := requestIDs(c); entry != nil; entry = entry.next {
					switch {
					case seen[entry.store]:
						t.Errorf("worker %d request %d: a store has two entries", worker, request)
					case issued[entry.id] != entry.store:
						t.Errorf("worker %d request %d: entry %q was not issued by its store in this request", worker, request, entry.id)
					}
					seen[entry.store] = true
				}
			}
		})
	}
	wg.Wait()
}

// Test_Store_CachedIDReadErrorReachesCaller pins that a storage that fails when the store looks for
// the session under an ID it cached is reported, as it is for the ID the request names, and not
// taken for a session that is not there.
func Test_Store_CachedIDReadErrorReachesCaller(t *testing.T) {
	t.Parallel()
	errDown := errors.New("storage down")
	storage := storagetest.New()
	store := NewStore(Config{Storage: storage})
	c := apptest.Ctx(t, fiber.New())

	require.NoError(t, mustGet(t, store, c).Save())

	storage.SetErr(storagetest.OpGet, "*", errDown)
	_, err := store.Get(c)
	require.ErrorIs(t, err, errDown)
}

// A store that is asked twice, for a visitor whose cookie names no session, looks that cookie up
// once. A store over the same cookie with a storage of its own still looks it up: what one storage
// does not hold, another may.
func Test_Store_Get_AskedAgainDoesNotLookUpTheNamedIDTwice(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	storage := storagetest.New()
	store := NewStore(Config{Storage: storage})
	other := memory.New()
	holder := NewStore(Config{Storage: other, KeyGenerator: func() string { return "stale" }})
	require.Equal(t, "stale", seedSession(t, app, holder, func(s *Session) { s.Set("k", "v") }))
	sharing := NewStore(Config{Storage: other})

	c := apptest.Ctx(t, app)
	c.Request().Header.SetCookie("session_id", "stale")

	first, second := mustGet(t, store, c), mustGet(t, store, c)
	require.True(t, first.Fresh() && second.Fresh())
	require.NotEqual(t, first.ID(), second.ID(), "an ID that was not saved is not a session")

	lookups := 0
	for _, key := range storage.CallKeys(storagetest.OpGet) {
		if key == keyNamespace+"stale" {
			lookups++
		}
	}
	require.Equal(t, 1, lookups, "the cookie named no session the first time, and is not looked up again")

	third := mustGet(t, sharing, c)
	require.False(t, third.Fresh(), "the storage of another store holds the session the cookie names")
	require.Equal(t, "stale", third.ID())
}

// The PassLocalsToContext setting is read once for each app a store serves, and the store goes by
// the app, not by the last answer.
func Test_Store_PassesLocalsToContextByApp(t *testing.T) {
	t.Parallel()
	onCtx := apptest.Ctx(t, fiber.New(fiber.Config{PassLocalsToContext: true}))
	offCtx := apptest.Ctx(t, fiber.New())

	store := NewStore()
	for range 3 {
		require.True(t, store.passesLocalsToContext(onCtx))
		require.False(t, store.passesLocalsToContext(offCtx))
	}
}

// The ID the request named is kept as a copy: the application can rewrite the bytes of the cookie
// in place, and a store that is asked again must not take the new value for the one it has already
// looked up.
func Test_Store_Get_AskedAgainAfterTheCookieWasRewritten(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	store := NewStore()
	live := seedSession(t, app, store, func(s *Session) { s.Set("k", "v") })
	stale := strings.Repeat("s", len(live)) // as long as the other, so that it is rewritten in place

	c := apptest.Ctx(t, app)
	c.Request().Header.SetCookie("session_id", stale)
	first := mustGet(t, store, c)
	require.True(t, first.Fresh(), "the cookie names no session")

	c.Request().Header.SetCookie("session_id", live)
	again := mustGet(t, store, c)
	require.False(t, again.Fresh(), "the store took the rewritten cookie for the ID it had looked up")
	require.Equal(t, live, again.ID())
}

// digestOf is the storage key of an ID under HashStorageKeys, spelled out here and not taken from
// the code that makes it.
func digestOf(id string) string {
	sum := sha256.Sum256([]byte(id))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// storageKeyPolicies are the ways a store can key its records.
var storageKeyPolicies = []struct {
	name   string
	prefix string
	hash   bool
}{
	{name: "default"},
	{name: "prefix", prefix: "app:session:"},
	{name: "hash", hash: true},
	{name: "prefix and hash", prefix: "app:session:", hash: true},
}

// A key costs the one allocation of the key itself: without a prefix or hashing it is the namespace
// and the ID in one string, and the digest is built on the stack.
//
// testing.AllocsPerRun panics in a parallel test, so this one stays sequential.
func Test_Store_StorageKey_Allocations(t *testing.T) {
	const id = "wuyjDTdyBzOvL4mUnQ6qiM0nsSYUdyAxnXn0X2VgbX4"

	for _, tc := range storageKeyPolicies {
		store := NewStore(Config{StorageKeyPrefix: tc.prefix, HashStorageKeys: tc.hash})
		require.LessOrEqual(t, testing.AllocsPerRun(100, func() { _ = store.storageKey(id) }), 1.0, tc.name)
	}
}

// Every way a session reaches the storage must use the same key policy: a single call site left on
// the raw ID would read, write or delete a record the others never find.
func Test_Session_StorageKeyPolicy_EveryCallSite(t *testing.T) {
	t.Parallel()
	for _, tc := range storageKeyPolicies {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mem := storagetest.New()
			handler, store := NewWithStore(Config{Storage: mem, StorageKeyPrefix: tc.prefix, HashStorageKeys: tc.hash})

			app := fiber.New()
			app.Use(handler)
			addSessionRoutes(app)
			app.Get("/reset", func(c fiber.Ctx) error {
				if err := FromContext(c).Reset(); err != nil {
					return err
				}
				FromContext(c).Set("again", true)
				return c.SendString(FromContext(c).ID())
			})

			ids := make([]string, 0, 8)
			do := func(path, id string) string {
				var cookies map[string]string
				if id != "" {
					cookies = map[string]string{"session_id": id}
				}
				_, body := sendRequest(t, app, path, cookies)
				if body != "" {
					ids = append(ids, body)
				}
				return body
			}

			id := do("/login", "")
			id = do("/regenerate", id) // Get, Delete, Set
			id = do("/reset", id)      // Get, Delete, Set
			do("/destroy", id)         // Get, Delete

			// The store pattern's own entry points.
			id = do("/login", "")
			loaded, err := store.GetByID(context.Background(), id)
			require.NoError(t, err)
			require.Equal(t, "alice", loaded.Get("user"))
			loaded.Release()
			require.NoError(t, store.Delete(context.Background(), id))

			for _, op := range []string{storagetest.OpGet, storagetest.OpSet, storagetest.OpDelete} {
				keys := mem.CallKeys(op)
				require.NotEmpty(t, keys, "the scenario never made a %q call, so it proves nothing about it", op)
				for _, key := range keys {
					label := op + ":" + key
					require.True(t, strings.HasPrefix(key, tc.prefix), "%s lacks the prefix %q", label, tc.prefix)
					rest := strings.TrimPrefix(key, tc.prefix)
					require.True(t, strings.HasPrefix(rest, keyNamespace), "%s lacks the namespace %q", label, keyNamespace)
					rest = strings.TrimPrefix(rest, keyNamespace)
					if tc.hash {
						require.Len(t, rest, hashedKeyLen, "%s is not a digest", label)
						require.NotContains(t, ids, rest, "%s used a session ID as the key", label)
						require.True(t, slices.ContainsFunc(ids, func(id string) bool { return digestOf(id) == rest }),
							"%s is the digest of no session this test created", label)
					} else {
						require.Contains(t, ids, rest, "%s is the key of no session this test created", label)
					}
				}
			}
		})
	}
}

// The point of hashing: what is in the storage is not something a client can send back as its
// cookie.
func Test_Session_HashStorageKeys_StoredKeyIsNotAnID(t *testing.T) {
	t.Parallel()
	mem := memory.New()
	handler, store := NewWithStore(Config{Storage: mem, HashStorageKeys: true})

	app := fiber.New()
	app.Use(handler)
	addSessionRoutes(app)

	_, id := sendRequest(t, app, "/login", nil)
	_, body := sendRequest(t, app, "/read", map[string]string{"session_id": id})
	require.Equal(t, "alice", body, "the client's ID still finds its session")

	keys, err := mem.Keys()
	require.NoError(t, err)
	require.Len(t, keys, 1)
	stored := string(keys[0])
	require.Equal(t, keyNamespace+digestOf(id), stored)
	require.NotEqual(t, id, stored)

	// Whoever reads the storage and sends what they found gets nothing.
	_, body = sendRequest(t, app, "/read", map[string]string{"session_id": stored})
	require.Equal(t, "<nil>", body)
	_, err = store.GetByID(context.Background(), stored)
	require.ErrorIs(t, err, ErrSessionIDNotFoundInStore)
	_, err = store.GetByID(context.Background(), digestOf(id))
	require.ErrorIs(t, err, ErrSessionIDNotFoundInStore, "nor is the digest without its namespace")

	sess, err := store.GetByID(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, "alice", sess.Get("user"))
	sess.Release()
}

// Two stores over one storage and the same session ID stay apart when their prefixes differ.
func Test_Session_StorageKeyPrefix_SeparatesStores(t *testing.T) {
	t.Parallel()
	shared := memory.New()
	same := func() string { return "same-id" }
	storeA := NewStore(Config{Storage: shared, StorageKeyPrefix: "a:", KeyGenerator: same})
	storeB := NewStore(Config{Storage: shared, StorageKeyPrefix: "b:", KeyGenerator: same})
	require.Equal(t, "same-id", seedSession(t, fiber.New(), storeA, func(s *Session) { s.Set("owner", "a") }))

	ctx := context.Background()
	_, err := storeB.GetByID(ctx, "same-id")
	require.ErrorIs(t, err, ErrSessionIDNotFoundInStore, "the other prefix does not see the record")

	sess, err := storeA.GetByID(ctx, "same-id")
	require.NoError(t, err)
	require.Equal(t, "a", sess.Get("owner"))
	sess.Release()

	require.NoError(t, storeB.Delete(ctx, "same-id"))
	sess, err = storeA.GetByID(ctx, "same-id")
	require.NoError(t, err, "deleting under the other prefix left the record alone")
	sess.Release()

	require.NoError(t, storeA.Delete(ctx, "same-id"))
	_, err = storeA.GetByID(ctx, "same-id")
	require.ErrorIs(t, err, ErrSessionIDNotFoundInStore)
}

// A prefix does not scope Reset: the storage interface cannot list keys, so Reset clears the whole
// storage, as the option documents.
func Test_Store_Reset_ClearsTheWholeStorageDespiteAPrefix(t *testing.T) {
	t.Parallel()
	shared := storagetest.New()
	store := NewStore(Config{Storage: shared, StorageKeyPrefix: "app:"})
	require.NoError(t, shared.Set("unrelated", []byte("x"), time.Minute))
	require.NoError(t, shared.Set("app:session", []byte("x"), time.Minute))

	require.NoError(t, store.Reset(context.Background()))
	require.Empty(t, shared.Keys())
}

// Benchmark_Store_StorageKey measures the key every read, write and delete of a session goes
// through. Without a prefix or hashing it costs one allocation, the key itself.
func Benchmark_Store_StorageKey(b *testing.B) {
	const id = "wuyjDTdyBzOvL4mUnQ6qiM0nsSYUdyAxnXn0X2VgbX4"

	for _, tc := range storageKeyPolicies {
		b.Run(tc.name, func(b *testing.B) {
			store := NewStore(Config{StorageKeyPrefix: tc.prefix, HashStorageKeys: tc.hash})

			b.ReportAllocs()
			for b.Loop() {
				_ = store.storageKey(id)
			}
		})
	}
}

// A store takes the ID that a store sharing a source with it cached, if it names a stored session,
// and never that of one it shares none with. G reads the cookies of A and of B: it shares with A,
// whose ID was never saved, and with B, which found its own session, but not with E, whose ID was
// never saved either. G must find the session its b_sid cookie names, whichever of A and E cached
// an ID last.
func Test_Session_Store_FindsTheSessionItsCookieNamesWhateverOtherStoresCached(t *testing.T) {
	t.Parallel()
	shared := memory.New()
	app := fiber.New()
	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid"), Storage: shared})
	storeE := NewStore(Config{Extractor: extractors.FromCookie("e_sid")})
	storeG := NewStore(Config{Extractor: extractors.Chain(extractors.FromCookie("a_sid"), extractors.FromCookie("b_sid")), Storage: shared})
	idB := seedSession(t, app, storeB, func(s *Session) { s.Set("k", "v") })

	for _, tc := range []struct {
		name  string
		asked []*Store
	}{
		{"the newest ID is that of a store it shares a cookie with", []*Store{storeA}},
		{"the newest ID is that of a store it shares none with", []*Store{storeA, storeB, storeE}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)
			c.Request().Header.SetCookie("b_sid", idB)

			var issued []string
			for _, store := range tc.asked {
				sess := mustGet(t, store, c)
				if store == storeB {
					require.False(t, sess.Fresh())
					require.Equal(t, idB, sess.ID(), "B finds its session, and caches nothing")
				} else {
					require.True(t, sess.Fresh(), "the store caches an ID that it does not save")
					issued = append(issued, sess.ID())
				}
			}

			g := mustGet(t, storeG, c)
			require.NotContains(t, issued, g.ID(), "G took an ID that another store cached")
			require.False(t, g.Fresh(), "G found the session its b_sid cookie names")
			require.Equal(t, idB, g.ID())
			require.Equal(t, "v", g.Get("k"))
		})
	}
}

// A new session whose cookie names an ID the storage does not know is cached with the others of the
// request, as the session of a cookie that names a stored one is not. W and X cache an ID first,
// unsaved; Y, whose cookie is unknown, caches one after them and saves it. R shares Y's cookie but
// not W's or X's, so it must take the ID Y cached, and not find its own session as it would if Y
// had cached nothing.
func Test_Session_Store_FreshSessionOfAnUnknownCookieIsCached(t *testing.T) {
	t.Parallel()
	shared := memory.New()
	app := fiber.New()
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid"), Storage: shared})
	storeW := NewStore(Config{Extractor: extractors.FromCookie("w_sid")})
	storeX := NewStore(Config{Extractor: extractors.FromCookie("x_sid")})
	storeY := NewStore(Config{Extractor: extractors.FromCookie("y_sid"), Storage: shared})
	storeR := NewStore(Config{Extractor: extractors.Chain(extractors.FromCookie("b_sid"), extractors.FromCookie("y_sid")), Storage: shared})
	idB := seedSession(t, app, storeB, func(s *Session) { s.Set("k", "v") })

	c := apptest.Ctx(t, app)
	c.Request().Header.SetCookie("b_sid", idB)
	c.Request().Header.SetCookie("y_sid", "names-no-session")

	for _, store := range []*Store{storeB, storeW, storeX} {
		mustGet(t, store, c)
	}
	y := mustGet(t, storeY, c)
	require.True(t, y.Fresh(), "Y's cookie names no stored session")
	require.NoError(t, y.Save())
	require.Same(t, storeY, requestIDs(c).store, "Y cached its ID after W and X")
	require.Equal(t, y.ID(), c.Locals(sessionIDContextKey), "and it is the ID the logger shows")

	r := mustGet(t, storeR, c)
	require.Equal(t, y.ID(), r.ID(), "R shares Y's cookie, and Y cached last")
	require.NotEqual(t, idB, r.ID(), "R went to its own cookie instead of the ID that Y cached")
	require.False(t, r.Fresh(), "R found the session that Y saved")
}

// Stores are told apart by every source they read, by kind and name, whether or not it can be
// written back to. Stores over the same query parameter, authentication scheme or custom extractor
// key share the ID cached on the request; a store over the query string and one over a cookie do
// not, so the ID a link carries reaches the stores that read the query string only.
func Test_Session_Store_SourcesAreToldApartByKindAndName(t *testing.T) {
	t.Parallel()
	custom := func(fiber.Ctx) (string, error) { return "", extractors.ErrNotFound }
	query, cookie := extractors.FromQuery("sid"), extractors.FromCookie("c_sid")
	bearer, legacy := extractors.FromAuthHeader("Bearer"), extractors.FromCustom("legacy", custom)
	form, param := extractors.FromForm("sid"), extractors.FromParam("sid")
	shares := func(a, b extractors.Extractor) bool {
		return NewStore(Config{Extractor: a}).sharesSource(NewStore(Config{Extractor: b}))
	}

	for _, pair := range [][2]extractors.Extractor{
		{query, extractors.FromQuery("sid")},
		{bearer, extractors.FromAuthHeader("bearer")},
		{legacy, extractors.FromCustom("legacy", custom)},
		{form, extractors.FromForm("sid")},
		{param, extractors.FromParam("sid")},
	} {
		require.True(t, shares(pair[0], pair[1]), "stores over the same source share")
	}
	for _, pair := range [][2]extractors.Extractor{
		{query, extractors.FromQuery("token")},
		{query, bearer},
		{query, cookie},
		{cookie, query},
		{bearer, cookie},
		{bearer, extractors.FromAuthHeader("Token")}, // the same header, another scheme
		{legacy, extractors.FromCustom("other", custom)},
		{legacy, query},
		{form, query}, // the same name in another kind is another source
		{param, form},
		{param, cookie},
	} {
		require.False(t, shares(pair[0], pair[1]), "stores over different sources are told apart")
	}

	// A new visitor: a store over the same source takes the ID the first store saved, a store over
	// another source starts a session of its own.
	shared := memory.New()
	app := fiber.New()
	byQuery := NewStore(Config{Extractor: query, Storage: shared})
	visitor := apptest.Ctx(t, app)
	first := mustGet(t, byQuery, visitor)
	first.Set("k", "v")
	require.NoError(t, first.Save())
	second := mustGet(t, NewStore(Config{Extractor: query, Storage: shared}), visitor)
	require.Equal(t, first.ID(), second.ID(), "a store over the same query parameter took the ID the first one saved")
	require.False(t, second.Fresh())
	third := mustGet(t, NewStore(Config{Extractor: cookie, Storage: shared}), visitor)
	require.NotEqual(t, first.ID(), third.ID(), "a cookie store next to them took the ID too")
	require.True(t, third.Fresh())

	// A session that a link names is served by the store that reads the query string; the cookie
	// store behind it keeps to its own cookie, and starts a session of its own for a visitor who
	// has none.
	id := seedSession(t, app, byQuery, func(s *Session) { s.Set("k", "v") })
	queryHandler, queryStore := NewWithStore(Config{Extractor: query, Storage: shared})
	cookieHandler, cookieStore := NewWithStore(Config{Extractor: cookie, Storage: shared})
	stacked := fiber.New()
	stacked.Use(queryHandler, cookieHandler)
	stacked.Get("/", func(c fiber.Ctx) error {
		q, m := queryStore.FromContext(c), cookieStore.FromContext(c)
		return c.SendString(fmt.Sprintf("%s|%t|%v|%t|%v", q.ID(), q.Fresh(), q.Get("k"), m.ID() == id, m.Get("k")))
	})
	resp, body := sendRequest(t, stacked, "/?sid="+id, nil)
	require.Equal(t, id+"|false|v|false|<nil>", body, "the cookie store served the session the link names")
	cookies := cookiesByName(resp)
	require.NotNil(t, cookies["c_sid"], "the cookie store set no cookie for its own session")
	require.NotEqual(t, id, cookies["c_sid"].Value, "the cookie carries the ID the link named")
}

// Config is embedded, so its fields can be assigned after NewStore. A store whose Extractor was
// replaced writes the ID to, and shares with, what it reads now.
func Test_Store_ExtractorReplacedAfterNewStore(t *testing.T) {
	t.Parallel()
	store := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	store.Extractor = extractors.FromCookie("b_sid")

	c := apptest.Ctx(t, fiber.New())
	require.NoError(t, mustGet(t, store, c).Save())

	var written []string
	for key := range c.Response().Header.Cookies() {
		written = append(written, string(key))
	}
	require.Equal(t, []string{"b_sid"}, written, "the ID goes to the cookie the store reads now")

	require.True(t, store.sharesSource(NewStore(Config{Extractor: extractors.FromCookie("b_sid")})))
	require.False(t, store.sharesSource(NewStore(Config{Extractor: extractors.FromCookie("a_sid")})))
	require.False(t, store.sharesSource(NewStore(Config{Extractor: extractors.FromCookie("B_SID")})), "cookie names are case-sensitive")

	store.Extractor = extractors.Chain(extractors.FromCookie("c_sid"), extractors.FromHeader("X-C"))
	require.Equal(t, []string{"cookie:c_sid", "header:X-C"}, sinkNames(store.places().sinks))
	require.True(t, store.sharesSource(NewStore(Config{Extractor: extractors.FromHeader("x-c")})))
	require.False(t, store.sharesSource(NewStore(Config{Extractor: extractors.FromCookie("b_sid")})))

	// A chain of the same length and with the same first extractor is another chain.
	chained := NewStore(Config{Extractor: extractors.Chain(extractors.FromCookie("c_sid"), extractors.FromHeader("X-C"))})
	require.Equal(t, []string{"cookie:c_sid", "header:X-C"}, sinkNames(chained.places().sinks))
	chained.Extractor = extractors.Chain(extractors.FromCookie("c_sid"), extractors.FromHeader("X-D"))
	require.Equal(t, []string{"cookie:c_sid", "header:X-D"}, sinkNames(chained.places().sinks))
	require.True(t, chained.sharesSource(NewStore(Config{Extractor: extractors.FromHeader("x-d")})))
	require.False(t, chained.sharesSource(NewStore(Config{Extractor: extractors.FromHeader("x-c")})))

	// A chain reports the key and the source of its first child, so a cookie replaced by a header
	// of the same name, and a leaf replaced by a chain that starts with it, differ in one thing
	// each, and each is another extractor.
	swapped := NewStore(Config{Extractor: extractors.FromCookie("sid")})
	swapped.Extractor = extractors.FromHeader("sid")
	require.Equal(t, []string{"header:sid"}, sinkNames(swapped.places().sinks), "a header, not the cookie of that name")
	require.True(t, swapped.sharesSource(NewStore(Config{Extractor: extractors.FromHeader("SID")})))
	require.False(t, swapped.sharesSource(NewStore(Config{Extractor: extractors.FromCookie("sid")})))

	// The name of an Authorization source is its scheme, which a replaced extractor changes alone.
	scheme := NewStore(Config{Extractor: extractors.FromAuthHeader("Bearer")})
	scheme.Extractor = extractors.FromAuthHeader("Token")
	require.True(t, scheme.sharesSource(NewStore(Config{Extractor: extractors.FromAuthHeader("token")})))
	require.False(t, scheme.sharesSource(NewStore(Config{Extractor: extractors.FromAuthHeader("Bearer")})))

	grown := NewStore(Config{Extractor: extractors.FromCookie("a")})
	grown.Extractor = extractors.Chain(extractors.FromCookie("a"), extractors.FromHeader("X-A"))
	require.Equal(t, []string{"cookie:a", "header:X-A"}, sinkNames(grown.places().sinks), "the chain has the header as well")
	require.True(t, grown.sharesSource(NewStore(Config{Extractor: extractors.FromHeader("x-a")})))
}

// An ID that names no session of the store is passed over to the next one, and not only to the
// request: W, the newest to cache an ID, reads the cookie S reads but keeps its sessions in a
// storage of its own, so its ID is no session of S's; A cached an ID before and saved its session
// in the storage of S, and S must find that one.
func Test_Session_Store_PassesOverAnIDThatIsNoSessionToAnOlderOneThatIs(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	storeA := NewStore(Config{Extractor: extractors.FromCookie("sid")})
	storeW := NewStore(Config{Extractor: extractors.FromCookie("sid")})
	storeS := NewStore(Config{Extractor: extractors.FromCookie("sid"), Storage: storeA.Storage})

	c := apptest.Ctx(t, app)

	a := mustGet(t, storeA, c)
	a.Set("k", "v")
	require.NoError(t, a.Save())

	w := mustGet(t, storeW, c)
	require.True(t, w.Fresh(), "A's ID is no session of the storage of W")
	require.Same(t, storeW, requestIDs(c).store, "W cached the newest ID")

	s := mustGet(t, storeS, c)
	require.False(t, s.Fresh(), "S stopped at the newest ID, which names no session, and lost the one A saved")
	require.Equal(t, a.ID(), s.ID())
	require.Equal(t, "v", s.Get("k"))
}

// Of the cached IDs that name a stored session the newest one is taken: A1 and A2 read different
// cookies, so each starts a session of its own and saves it, and S reads both.
func Test_Session_Store_TakesTheNewestOfTheSavedIDsItShares(t *testing.T) {
	t.Parallel()
	storage := memory.New()
	storeA1 := NewStore(Config{Extractor: extractors.FromCookie("a1_sid"), Storage: storage})
	storeA2 := NewStore(Config{Extractor: extractors.FromCookie("a2_sid"), Storage: storage})
	storeS := NewStore(Config{
		Extractor: extractors.Chain(extractors.FromCookie("a1_sid"), extractors.FromCookie("a2_sid")),
		Storage:   storage,
	})

	c := apptest.Ctx(t, fiber.New())
	a1, a2 := saveNew(t, c, storeA1), saveNew(t, c, storeA2)
	require.NotEqual(t, a1.ID(), a2.ID())

	s := mustGet(t, storeS, c)
	require.False(t, s.Fresh())
	require.Equal(t, a2.ID(), s.ID(), "S took the older of the two sessions that were saved")
}

// Regenerate and Reset give the session a new ID, and a store that is asked again looks for the
// session under that one, not under the ID that was deleted: for a visitor who has no cookie yet
// and for one who has.
func Test_Session_Store_Get_AskedAgainFindsTheSessionUnderItsNewID(t *testing.T) {
	t.Parallel()
	for name, change := range map[string]func(*Session) error{"Regenerate": (*Session).Regenerate, "Reset": (*Session).Reset} {
		for _, returning := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s, returning visitor %t", name, returning), func(t *testing.T) {
				t.Parallel()
				app := fiber.New()
				store := NewStore()
				c := apptest.Ctx(t, app)
				if returning {
					c.Request().Header.SetCookie("session_id", seedSession(t, app, store, func(s *Session) { s.Set("k", "v") }))
				}

				first := mustGet(t, store, c)
				was := first.ID()
				require.NoError(t, change(first))
				require.NotEqual(t, was, first.ID())
				first.Set("after", "change")
				require.NoError(t, first.Save())

				again := mustGet(t, store, c)
				require.Equal(t, first.ID(), again.ID(), "the session was lost under its new ID")
				require.False(t, again.Fresh())
				require.Equal(t, "change", again.Get("after"))
			})
		}
	}
}

// Regenerate of a session the store gave out first does not hide the session it gave out and saved
// since: the entry of the store names the newer one, and the older session is not the one that
// entry is about.
func Test_Session_Store_Get_RegenerateOfAnOlderSessionKeepsTheSavedOne(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	store := NewStore()
	c := apptest.Ctx(t, app)

	older := mustGet(t, store, c)
	newer := mustGet(t, store, c)
	require.NoError(t, newer.Save())
	require.NoError(t, older.Regenerate())

	again := mustGet(t, store, c)
	require.Equal(t, newer.ID(), again.ID(), "the saved session was hidden by the regenerated older one")
	require.False(t, again.Fresh())
}

// sinkNames names the sinks as "source:key", in order; nil for none.
func sinkNames(sinks []extractors.Extractor) []string {
	var names []string
	for _, sink := range sinks {
		switch sink.Source {
		case extractors.SourceCookie:
			names = append(names, "cookie:"+sink.Key)
		case extractors.SourceHeader:
			names = append(names, "header:"+sink.Key)
		default:
			names = append(names, "unexpected:"+sink.Key)
		}
	}
	return names
}

// A store works out once where a session ID can be written back to: the cookies and headers of its
// extractor, in order, and nothing a request only reads from. A store built without NewStore finds
// the same by walking the extractor.
func Test_Store_WritableSinks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		want      []string
		extractor extractors.Extractor
	}{
		{name: "cookie", extractor: extractors.FromCookie("sid"), want: []string{"cookie:sid"}},
		{name: "header", extractor: extractors.FromHeader("X-Sid"), want: []string{"header:X-Sid"}},
		{name: "query is read-only", extractor: extractors.FromQuery("sid")},
		{name: "authorization header is read-only", extractor: extractors.FromAuthHeader("Bearer")},
		{
			name: "chain keeps the order and drops what is read-only",
			extractor: extractors.Chain(
				extractors.FromQuery("q"),
				extractors.FromCookie("a"),
				extractors.FromAuthHeader("Bearer"),
				extractors.FromHeader("X-B"),
			),
			want: []string{"cookie:a", "header:X-B"},
		},
		{
			name: "chain inside a chain",
			extractor: extractors.Chain(
				extractors.FromCookie("a"),
				extractors.Chain(extractors.FromHeader("X-B"), extractors.FromCookie("c")),
			),
			want: []string{"cookie:a", "header:X-B", "cookie:c"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := NewStore(Config{Extractor: tc.extractor})
			require.True(t, store.profile.current(store.Extractor), "NewStore works the profile out, even when there are no sinks")
			require.Equal(t, tc.want, sinkNames(store.places().sinks))

			literal := &Store{Config: store.Config}
			require.False(t, literal.profile.current(literal.Extractor))
			require.Equal(t, tc.want, sinkNames(literal.places().sinks), "a store built without NewStore walks the extractor")
		})
	}
}

// Finding the sinks of a session costs a save nothing: with the store's own slice, or the
// session's, there is nothing to allocate.
//
// testing.AllocsPerRun panics in a parallel test, so this one stays sequential.
func Test_Session_getExtractorInfo_DoesNotAllocate(t *testing.T) {
	store := NewStore(Config{Extractor: extractors.Chain(extractors.FromCookie("a"), extractors.FromHeader("X-B"))})
	provenance := &Session{
		config:    store,
		extractor: extractors.Result{Value: "v", Key: "a", Source: extractors.SourceCookie, Resolved: true},
	}
	fresh := &Session{config: store, isFresh: true}
	refused := &Session{config: store, extractor: extractors.Result{Value: "v", Key: "q", Source: extractors.SourceQuery, Resolved: true}}

	require.Zero(t, testing.AllocsPerRun(100, func() { _ = provenance.getExtractorInfo() }), "the sink the ID came from")
	require.Zero(t, testing.AllocsPerRun(100, func() { _ = fresh.getExtractorInfo() }), "the sinks of a new session")
	require.Zero(t, testing.AllocsPerRun(100, func() { _ = refused.getExtractorInfo() }), "an ID that must not be written back")
}

// Whatever ID a client sends, the key a session is stored under follows the policy of the store and
// nothing else: the prefix and then the ID itself, or the prefix and then a fixed-length digest
// from which the ID cannot be read back and in which it cannot inject anything.
func FuzzStoreStorageKey(f *testing.F) {
	f.Add("abc", "", false)
	f.Add("abc", "app:", false)
	f.Add("abc", "app:", true)
	f.Add("", "app:", true)
	f.Add("b:abc", "a:", false)
	f.Add("a\x00b\r\n:*", "", true)
	f.Add(strings.Repeat("x", 4096), "p", true)

	f.Fuzz(func(t *testing.T, id, prefix string, hashed bool) {
		store := &Store{Config: Config{StorageKeyPrefix: prefix, HashStorageKeys: hashed}}
		key := store.storageKey(id)
		require.Equal(t, key, store.storageKey(id), "the key of an ID is stable")

		if !hashed {
			require.Equal(t, storageKey(prefix+keyNamespace+id), key)
			return
		}
		require.Equal(t, storageKey(prefix+keyNamespace+digestOf(id)), key)
		require.NotEqual(t, key, store.storageKey(id+"x"), "two IDs share a key")
	})
}

// Arbitrary bytes in the cookies and the header of a request, for stores over several cookies and a
// header, never panic the middleware and never break a property of the invariant world: a new
// session never takes what the client sent, an ID is only written to a cookie or header that a
// store which owns it reads, and the rest of what check holds.
func FuzzSessionClientInput(f *testing.F) {
	f.Add("", "", "", "")
	f.Add("a", "b", "c", "d")
	f.Add("b:abc", "a:abc", "unknown", "unknown")
	f.Add(strings.Repeat("g", 512), "\x00\x01\x02", "\"quoted\"", "x y")

	f.Fuzz(func(t *testing.T, a, b, c, header string) {
		// A world of its own for every input, closed with it: nothing carries over from one input
		// to the next, so an input that fails fails alone, and neither the storage nor the
		// goroutines of the storages grow for as long as the fuzzing runs.
		w := newInvariantWorld(t, 1)
		app := w.storeAPIApp()

		req := invariantRequest{
			cookies: map[string]string{"a_sid": a, "b_sid": b, "c_sid": c},
			header:  header,
			ops:     []string{"A", "B", "C", "A2", "B"},
		}
		resp, rec := req.sendDirect(t, app.Handler())
		if resp.StatusCode != fiber.StatusOK {
			return
		}
		w.check(t, 0, req, resp, rec)
	})
}

// withCookies makes the request carry the cookies, as a client that holds them sends them.
func withCookies(cookies map[string]string) apptest.Option {
	return func(req *http.Request) {
		for name, value := range cookies {
			req.AddCookie(&http.Cookie{Name: name, Value: value})
		}
	}
}

// doRequest issues a GET with the given cookies and returns the response and its body. It reports
// failures as an error, since apptest stops a test with FailNow, which a goroutine must not call.
func doRequest(app *fiber.App, target string, cookies map[string]string) (*http.Response, string, error) {
	resp, err := app.Test(apptest.Req(fiber.MethodGet, target, withCookies(cookies)))
	if err != nil {
		return nil, "", fmt.Errorf("send %s: %w", target, err)
	}
	// conventions:ignore body-read: it runs in goroutines, where FailNow is not allowed
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", target, err)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, "", fmt.Errorf("close %s: %w", target, err)
	}
	return resp, string(raw), nil
}

// sendRequest sends a GET for target through app with the cookies, and returns the response and
// its body.
//
//nolint:gocritic // the two results are of different types, and names would only repeat what every caller calls them
func sendRequest(t *testing.T, app *fiber.App, target string, cookies map[string]string) (*http.Response, string) {
	t.Helper()
	resp := apptest.Do(t, app, apptest.Req(fiber.MethodGet, target, withCookies(cookies)))
	return resp, apptest.Body(t, resp)
}

// sendExpecting is sendRequest that also requires the status of the response.
//
//nolint:gocritic // the two results are of different types, and names would only repeat what every caller calls them
func sendExpecting(t *testing.T, app *fiber.App, target string, cookies map[string]string, status int) (*http.Response, string) {
	t.Helper()
	resp := apptest.Expect(t, app, apptest.Req(fiber.MethodGet, target, withCookies(cookies)), status)
	return resp, apptest.Body(t, resp)
}

// cookiesByName indexes the Set-Cookie headers of a response by cookie name.
func cookiesByName(resp *http.Response) map[string]*http.Cookie {
	out := make(map[string]*http.Cookie)
	for _, c := range resp.Cookies() {
		out[c.Name] = c
	}
	return out
}

// useCookieStore puts a session middleware over the cookie name on app, with the Config given
// besides its Extractor, and returns its store.
func useCookieStore(app *fiber.App, name string, config ...Config) *Store {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	cfg.Extractor = extractors.FromCookie(name)
	handler, store := NewWithStore(cfg)
	app.Use(handler)
	return store
}

// responseCookie returns the value of the cookie that the response of c sets under name.
func responseCookie(t *testing.T, c fiber.Ctx, name string) string {
	t.Helper()
	cookie := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(cookie)
	cookie.SetKey(name)
	require.True(t, c.Response().Header.Cookie(cookie), "the response sets no cookie %q", name)
	return string(cookie.Value())
}

// saveNew asks store for its session in c, which must be a new one, and saves it with some data.
func saveNew(t *testing.T, c fiber.Ctx, store *Store) *Session {
	t.Helper()
	sess := mustGet(t, store, c)
	require.True(t, sess.Fresh())
	sess.Set("k", store.Extractor.Key)
	require.NoError(t, sess.Save())
	return sess
}

// setInSessions is a handler that sets key to value in the session of the middleware of each store.
func setInSessions(key, value string, stores ...*Store) fiber.Handler {
	return func(c fiber.Ctx) error {
		for _, store := range stores {
			store.FromContext(c).Set(key, value)
		}
		return nil
	}
}

// Two session middleware, each over its own cookie, keep two sessions apart in one request. The
// second used to look up the first one's cached ID: its session never survived a request, or, over
// a shared backend, aliased the first session's record.
func Test_Session_MultipleStores_Middleware(t *testing.T) {
	t.Parallel()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared storage=%v", shared), func(t *testing.T) {
			t.Parallel()
			var storageA, storageB fiber.Storage = memory.New(), memory.New()
			if shared {
				storageB = storageA
			}
			app := fiber.New()
			storeA := useCookieStore(app, "a_sid", Config{Storage: storageA})
			storeB := useCookieStore(app, "b_sid", Config{Storage: storageB})
			app.Get("/set", func(c fiber.Ctx) error {
				storeA.FromContext(c).Set("who", "alice")
				storeB.FromContext(c).Set("lang", "en")
				return c.SendStatus(fiber.StatusOK)
			})
			app.Get("/get", func(c fiber.Ctx) error {
				a, b := storeA.FromContext(c), storeB.FromContext(c)
				return c.SendString(fmt.Sprintf("%v|%v|%v|%v|%v|%v|%s|%s",
					a.Get("who"), a.Get("lang"), b.Get("lang"), b.Get("who"), a.Fresh(), b.Fresh(), a.ID(), b.ID()))
			})

			resp, _ := sendRequest(t, app, "/set", nil)
			cookies := cookiesByName(resp)
			require.Contains(t, cookies, "a_sid")
			require.Contains(t, cookies, "b_sid")
			idA, idB := cookies["a_sid"].Value, cookies["b_sid"].Value
			require.NotEqual(t, idA, idB, "each store issues its own ID")

			_, body := sendRequest(t, app, "/get", map[string]string{"a_sid": idA, "b_sid": idB})
			require.Equal(t, fmt.Sprintf("alice|<nil>|en|<nil>|false|false|%s|%s", idA, idB), body,
				"both sessions load again, keep their own data and see none of the other's")

			// Each record is stored once, under its own ID.
			rawA, err := storageA.Get(sessionKey(idA))
			require.NoError(t, err)
			require.NotNil(t, rawA)
			rawB, err := storageB.Get(sessionKey(idB))
			require.NoError(t, err)
			require.NotNil(t, rawB)
		})
	}
}

// The package-level FromContext keeps meaning "the innermost middleware"; the store's own
// FromContext finds a particular one however deep it is nested.
func Test_Session_MultipleStores_FromContext(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	storeA, storeB := useCookieStore(app, "a_sid"), useCookieStore(app, "b_sid")
	storeC := NewStore(Config{Extractor: extractors.FromCookie("c_sid")})
	app.Get("/", func(c fiber.Ctx) error {
		innermost := FromContext(c)
		a, b := storeA.FromContext(c), storeB.FromContext(c)
		return c.SendString(fmt.Sprint(
			innermost != nil && innermost.Store() == storeB,
			a != nil && a.Store() == storeA,
			b != nil && b == innermost,
			storeC.FromContext(c) == nil,
			storeA.FromContext(c.RequestCtx()) == a,
		))
	})

	_, body := sendRequest(t, app, "/", nil)
	require.Equal(t, "true true true true true", body)

	t.Run("outside the request", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, storeA.FromContext(nil))
		require.Nil(t, storeA.FromContext(apptest.Ctx(t, app)), "no middleware ran for this context")
	})

	t.Run("one middleware", func(t *testing.T) {
		t.Parallel()
		handler, store := NewWithStore()
		single := fiber.New()
		single.Use(handler)
		single.Get("/", func(c fiber.Ctx) error {
			return c.SendString(fmt.Sprint(store.FromContext(c) == FromContext(c), FromContext(c) != nil))
		})
		_, body := sendRequest(t, single, "/", nil)
		require.Equal(t, "true true", body)
	})
}

// A middleware takes the session that a store over its cookie saved through Store.Get before it
// ran: the CSRF middleware's store and the session middleware's own are two Store values like this,
// and a visitor's first request must end up with one session, not two. It does so whatever ran in
// between: a store over another cookie, or a middleware over another cookie.
func Test_Session_MultipleStores_MiddlewareFindsTheSessionStoreGetSaved(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		want string // what the handler sees of the session, and of the middleware over b_sid if there is one
		// between asks a store over b_sid after the save, outer runs a middleware over b_sid in front of the one under test
		between, outer bool
	}{
		{name: "the same cookie", want: "t1|false|-"},
		{name: "another store asked in between", want: "t1|false|-", between: true},
		{name: "behind a middleware over another cookie", want: "t1|false|<nil>", outer: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			storage := memory.New()
			early := NewStore(Config{Extractor: extractors.FromCookie("a_sid"), Storage: storage})
			handler, late := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid"), Storage: storage})
			require.NotSame(t, early, late)
			handlerB, storeB := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})

			var earlyID atomic.Value
			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				sess, err := early.Get(c)
				if err != nil {
					return err
				}
				defer sess.Release()
				sess.Set("token", "t1")
				if saveErr := sess.Save(); saveErr != nil {
					return saveErr
				}
				earlyID.Store(sess.ID())
				if tc.between {
					between, getErr := storeB.Get(c)
					if getErr != nil {
						return getErr
					}
					between.Release()
				}
				return c.Next()
			})
			if tc.outer {
				app.Use(handlerB)
			}
			app.Use(handler)
			app.Get("/", func(c fiber.Ctx) error {
				m, outer := late.FromContext(c), "-"
				if o := storeB.FromContext(c); o != nil {
					outer = fmt.Sprint(o.Get("token"))
				}
				return c.SendString(fmt.Sprintf("%v|%v|%s", m.Get("token"), m.Fresh(), outer))
			})

			resp, body := sendRequest(t, app, "/", nil)
			require.Equal(t, tc.want, body, "the middleware did not find the session saved before it")
			cookies := cookiesByName(resp)
			require.Contains(t, cookies, "a_sid")
			require.Equal(t, earlyID.Load(), cookies["a_sid"].Value, "the cookie carries the one ID both stores used")
			keys, err := storage.Keys()
			require.NoError(t, err)
			require.Len(t, keys, 1, "one visitor, one stored session")
		})
	}
}

// A middleware finds the session its own cookie names. It takes neither the ID of a middleware over
// another cookie in front of it, nor an ID that names no session: one that a Store.Get before it
// cached and never saved, though that store shares a cookie with it (it keeps its sessions in a
// storage of its own, which never saw the session), or the new one of the middleware it runs
// inside.
func Test_Session_MultipleStores_MiddlewarePassesOverAnIDThatNamesNoSession(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		early  string               // the cookie of a store asked through Store.Get first, if any
		front  string               // the cookie of the session middleware it runs inside
		cookie string               // the cookie of the request that names its session
		late   extractors.Extractor // what the middleware under test reads
	}{
		{
			name: "the early store reads the cookie that names the session", early: "b_sid", front: "a_sid",
			late: extractors.FromCookie("b_sid"), cookie: "b_sid",
		},
		{
			name: "the early store reads the first cookie of a chain", early: "s2a_sid", front: "s1_sid",
			late:   extractors.Chain(extractors.FromCookie("s2a_sid"), extractors.FromCookie("s2b_sid")),
			cookie: "s2b_sid",
		},
		{
			name: "the middleware around it reads the first cookie of a chain", front: "a_sid",
			late:   extractors.Chain(extractors.FromCookie("a_sid"), extractors.FromCookie("b_sid")),
			cookie: "b_sid",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			shared := memory.New()
			frontHandler, _ := NewWithStore(Config{Storage: shared, Extractor: extractors.FromCookie(tc.front)})
			lateHandler, late := NewWithStore(Config{Storage: shared, Extractor: tc.late})

			app := fiber.New()
			if tc.early != "" {
				early := NewStore(Config{Extractor: extractors.FromCookie(tc.early)})
				app.Use(func(c fiber.Ctx) error {
					sess, err := early.Get(c) // new: it caches an ID, which is never saved
					if err != nil {
						return err
					}
					sess.Release()
					return c.Next()
				})
			}
			app.Use(frontHandler, lateHandler)
			app.Get("/read", func(c fiber.Ctx) error {
				m := late.FromContext(c)
				return c.SendString(fmt.Sprintf("%v|%t|%s", m.Get("who"), m.Fresh(), m.ID()))
			})

			id := seedSession(t, app, late, func(s *Session) { s.Set("who", "late") })
			resp, body := sendRequest(t, app, "/read", map[string]string{tc.cookie: id})
			require.Equal(t, "late|false|"+id, body, "the middleware found its own session; it did not take the ID that names none")
			cookies := cookiesByName(resp)
			require.Contains(t, cookies, tc.cookie)
			require.Equal(t, id, cookies[tc.cookie].Value)
		})
	}
}

// Stores that share only some of their sources: the inner middleware reads a_sid and b_sid, the one
// it runs inside b_sid, and a store over a_sid saved a session for the request before either ran.
// The inner middleware takes the session of the middleware it runs inside when that ID names one,
// and passes it over for the session the store saved when it does not.
func Test_Session_MultipleStores_NearestMiddlewareFirst(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		want   string
		seeded bool // the visitor holds a b_sid that names a session
	}{
		{name: "the ID of the outer middleware names no session", want: "t1"},
		{name: "the ID of the outer middleware names a session", want: "tB", seeded: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			storage := memory.New()
			early := NewStore(Config{Extractor: extractors.FromCookie("a_sid"), Storage: storage})
			outerHandler, outerStore := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid"), Storage: storage})
			innerHandler, inner := NewWithStore(Config{
				Extractor: extractors.Chain(extractors.FromCookie("a_sid"), extractors.FromCookie("b_sid")),
				Storage:   storage,
			})

			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				sess, err := early.Get(c)
				if err != nil {
					return err
				}
				defer sess.Release()
				sess.Set("token", "t1")
				if saveErr := sess.Save(); saveErr != nil {
					return saveErr
				}
				return c.Next()
			})
			app.Use(outerHandler, innerHandler)
			app.Get("/", func(c fiber.Ctx) error { return c.SendString(fmt.Sprint(inner.FromContext(c).Get("token"))) })

			var cookies map[string]string
			if tc.seeded {
				id := seedSession(t, app, outerStore, func(sess *Session) { sess.Set("token", "tB") })
				cookies = map[string]string{"b_sid": id}
			}
			_, body := sendRequest(t, app, "/", cookies)
			require.Equal(t, tc.want, body)
		})
	}
}

// The IDs a request keeps are a list that fasthttp closes from its head: caching again for a store
// keeps its entry in the list, and closing the list gives every entry back to the pool, cleared, so
// that a pooled entry holds no store and no ID of a finished request.
//
// conventions:ignore parallel: a parallel test could take the entries from the shared pool between
// the close and the checks.
func Test_Session_CachedIDs_StayWholeAndGoBackToThePoolCleared(t *testing.T) {
	c := apptest.Ctx(t, fiber.New())
	a, b := NewStore(), NewStore()

	cacheID(c, a, sessionIdentity{id: "1"})
	first := requestIDs(c)
	cacheID(c, b, sessionIdentity{id: "2"})
	cacheID(c, a, sessionIdentity{id: "3"})

	require.Same(t, first, requestIDs(c), "the entry a store had is its entry again")
	second := first.next
	require.Equal(t, "3", first.id)
	require.Same(t, b, second.store)
	require.Nil(t, second.next)

	require.NoError(t, first.Close())
	require.Equal(t, cachedID{}, *first)
	require.Equal(t, cachedID{}, *second)

	// A pool decides for itself whether Get returns what Put was given, and under -race drops one
	// Put in four: closing gives an entry back if one of many tries gets it again.
	for range 64 {
		entry := &cachedID{sessionIdentity: sessionIdentity{id: "x"}}
		require.NoError(t, entry.Close())
		if cachedIDPool.Get() == any(entry) {
			return
		}
	}
	t.Fatal("closing the list did not give its entries back to the pool")
}

// The provenance cached on the request belongs to the store that resolved it. Store A saves a new
// session and so caches its ID; store B then finds its own session through another cookie, or
// starts one. A's next Get takes the ID it cached and finds the record it saved, but must not take
// B's provenance with it: A's session would be written back to B's cookie, replacing the ID of B's
// session. A session of B that was found and not saved writes no cookie at all.
func Test_Session_MultipleStores_CachedProvenanceBelongsToItsStore(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})
	idB := seedSession(t, app, storeB, func(s *Session) { s.Set("n", 1) })

	for _, tc := range []struct {
		name      string
		returning bool // B finds its own session through its cookie
		saveB     bool
	}{
		{name: "B starts a session", saveB: true},
		{name: "B finds its session", returning: true, saveB: true},
		{name: "B finds its session and does not save it", returning: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)
			if tc.returning {
				c.Request().Header.SetCookie("b_sid", idB)
			}
			first := mustGet(t, storeA, c)
			first.Set("n", 1)
			require.NoError(t, first.Save())
			b := mustGet(t, storeB, c)
			again := mustGet(t, storeA, c)
			require.False(t, again.Fresh(), "A found the record it saved")
			require.Equal(t, 1, again.Get("n"))
			again.Set("n", 2)

			// A last: were its session tied to B's cookie, the later header would win.
			if tc.saveB {
				require.NoError(t, b.Save())
			}
			require.NoError(t, again.Save())
			require.Equal(t, !tc.returning, b.Fresh())
			require.Equal(t, first.ID(), again.ID(), "A found the session it saved")
			require.Equal(t, again.ID(), responseCookie(t, c, "a_sid"), "the cookie carries the one session of A")
			if tc.saveB {
				require.Equal(t, b.ID(), responseCookie(t, c, "b_sid"), "B's cookie carries B's session and nothing else")
			} else {
				require.Empty(t, c.Response().Header.PeekCookie("b_sid"), "B's session was found and not saved, so B's cookie is not written")
			}
			if tc.returning {
				require.Equal(t, idB, b.ID(), "B loaded its own session")
			}
		})
	}
}

// A store that takes the ID another store cached takes the provenance cached with it as well, as it
// holds for the store: its session goes back to the cookie the ID came from, not to every sink of
// the chain, and never to a header it does not read, nor is the ID promoted into its cookie, since
// a visitor's session is not written to a place the visitor did not send it from. The ID the first
// store cached is the one it started, or the one it gave a session it found with Regenerate.
func Test_Session_MultipleStores_SharingStoreAdoptsCachedProvenance(t *testing.T) {
	t.Parallel()
	chain := func() extractors.Extractor {
		return extractors.Chain(extractors.FromCookie("sid"), extractors.FromHeader("X-Sid"))
	}
	for _, tc := range []struct {
		request    func(h *fasthttp.RequestHeader, id string) // names the session id in the request
		name       string
		from       string               // where the first store's session goes back to
		second     extractors.Extractor // what the store asked second reads; the first reads the chain
		keepsSink  bool                 // whether that is a sink of the second store too
		regenerate bool                 // whether the first store regenerates a session the request names
	}{
		{
			name: "the same sources", second: chain(), from: "sid", keepsSink: true,
			request: func(h *fasthttp.RequestHeader, id string) { h.SetCookie("sid", id) },
		},
		{
			name: "fewer sources", second: extractors.FromCookie("sid"), from: "X-Sid",
			request: func(h *fasthttp.RequestHeader, id string) { h.Set("X-Sid", id) },
		},
		{
			name: "the same sources, after a Regenerate", second: chain(), from: "sid", keepsSink: true, regenerate: true,
			request: func(h *fasthttp.RequestHeader, id string) { h.SetCookie("sid", id) },
		},
		{
			name: "fewer sources, after a Regenerate", second: extractors.FromCookie("sid"), from: "X-Sid", regenerate: true,
			request: func(h *fasthttp.RequestHeader, id string) { h.Set("X-Sid", id) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			shared := memory.New()
			first := NewStore(Config{Storage: shared, Extractor: chain()})
			second := NewStore(Config{Storage: shared, Extractor: tc.second})
			require.NotSame(t, first, second)

			sent := "unknown"
			if tc.regenerate {
				sent = seedSession(t, fiber.New(), first, func(s *Session) { s.Set("k", "v") })
			}
			c := apptest.Ctx(t, fiber.New())
			tc.request(&c.Request().Header, sent)

			p := mustGet(t, first, c)
			if tc.regenerate {
				require.False(t, p.Fresh(), "the request names a stored session")
				require.NoError(t, p.Regenerate())
				require.NotEqual(t, sent, p.ID())
			} else {
				require.True(t, p.Fresh(), "the request names no session, so the first store starts one and caches its ID")
			}
			require.Equal(t, tc.from, p.extractor.Key, "the new session goes back to where the request named it")
			require.NoError(t, p.Save())

			q := mustGet(t, second, c)
			require.Equal(t, p.ID(), q.ID(), "the second store takes the cached ID")
			require.False(t, q.Fresh(), "and finds the session the first one saved")
			provenance := p.extractor
			provenance.Resolved = tc.keepsSink
			require.Equal(t, provenance, q.extractor, "and the provenance cached with it, as a sink of its own or not")

			// The first store's save wrote the ID into this response as well: clear it, so that
			// what is left is what the second store writes.
			c.Response().Header.DelCookie("sid")
			c.Response().Header.Del("X-Sid")
			require.NoError(t, q.Save())
			require.Empty(t, c.Response().Header.Peek("X-Sid"), "a header the ID did not come from, or the store does not read, is not written")
			if tc.keepsSink {
				require.Equal(t, q.ID(), responseCookie(t, c, "sid"), "the ID goes back to the cookie it came from")
			} else {
				require.Empty(t, c.Response().Header.PeekCookie("sid"), "and the ID is not promoted into the cookie")
			}
		})
	}
}

// A middleware over another cookie between two that share a source does not come between them: the
// inner one takes the session the outer one found through the cookie that only the outer one reads,
// though the one between shares nothing with either, and writes it to no cookie of the outer.
func Test_Session_MultipleStores_UnrelatedMiddlewareBetweenSharingOnesIsPassedOver(t *testing.T) {
	t.Parallel()
	storage := memory.New()
	outerHandler, outerStore := NewWithStore(Config{
		Storage:   storage,
		Extractor: extractors.Chain(extractors.FromCookie("sid"), extractors.FromCookie("shared")),
	})
	betweenHandler, betweenStore := NewWithStore(Config{Extractor: extractors.FromCookie("between_sid")})
	innerHandler, innerStore := NewWithStore(Config{Storage: storage, Extractor: extractors.FromCookie("shared")})

	app := fiber.New()
	app.Use(outerHandler, betweenHandler, innerHandler)
	app.Get("/read", func(c fiber.Ctx) error {
		inner, between := innerStore.FromContext(c), betweenStore.FromContext(c)
		return c.SendString(fmt.Sprintf("%v|%t|%s|%t", inner.Get("who"), inner.Fresh(), inner.ID(), between.Fresh()))
	})

	id := seedSession(t, app, outerStore, func(s *Session) { s.Set("who", "outer") })
	resp, body := sendRequest(t, app, "/read", map[string]string{"sid": id})
	require.Equal(t, "outer|false|"+id+"|true", body, "the middleware between them hid the session of the outer one from the inner one")
	cookies := cookiesByName(resp)
	require.NotContains(t, cookies, "shared", "the inner store wrote the ID it took to a cookie the client did not send")
	require.Equal(t, id, cookies["sid"].Value)
}

// Save leaves a session to the middleware that manages it, and only that one: a session no
// middleware of the request manages is saved, even while the middleware of another store is the
// current one.
func Test_Session_MultipleStores_SaveOfASessionNoMiddlewareManages(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, fiber.New())
	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})

	early := mustGet(t, storeA, c)
	early.Set("k", "v")

	m := startMiddleware(t, c, storeB)
	defer releaseMiddleware(m)

	require.NoError(t, early.Save())
	stored, err := storeA.Storage.GetWithContext(context.Background(), string(storeA.storageKey(early.ID())))
	require.NoError(t, err)
	require.NotEmpty(t, stored, "the session was saved")
}

// The provenance of one store's ID must never reach another store: an ID read from a read-only
// source is not written to a cookie, and a store never writes its ID to a cookie it does not own.
// Both are what session fixation relies on.
func Test_Session_MultipleStores_ProvenanceStaysWithItsStore(t *testing.T) {
	t.Parallel()
	storage := memory.New()
	handlerA, storeA := NewWithStore(Config{
		Storage:   storage,
		Extractor: extractors.Chain(extractors.FromQuery("sid"), extractors.FromCookie("a_sid")),
	})
	handlerB, storeB := NewWithStore(Config{Storage: storage, Extractor: extractors.FromCookie("b_sid")})

	app := fiber.New()
	app.Use(handlerA, handlerB)
	app.Get("/seed", func(c fiber.Ctx) error {
		storeA.FromContext(c).Set("who", "a")
		storeB.FromContext(c).Set("who", "b")
		return c.SendString(storeA.FromContext(c).ID() + "|" + storeB.FromContext(c).ID())
	})
	app.Get("/read", func(c fiber.Ctx) error {
		a, b := storeA.FromContext(c), storeB.FromContext(c)
		return c.SendString(fmt.Sprintf("%v|%v|%s|%s", a.Get("who"), b.Get("who"), a.ID(), b.ID()))
	})

	_, body := sendRequest(t, app, "/seed", nil)
	idA, idB, _ := strings.Cut(body, "|")
	require.NotEqual(t, idA, idB)

	// A's ID arrives in the query, a read-only source; B's arrives in its cookie.
	resp, body := sendRequest(t, app, "/read?sid="+idA, map[string]string{"b_sid": idB})
	require.Equal(t, fmt.Sprintf("a|b|%s|%s", idA, idB), body, "each store resolved its own session")

	cookies := cookiesByName(resp)
	require.NotContains(t, cookies, "a_sid", "an ID from the query is not promoted into a cookie")
	for name, cookie := range cookies {
		require.NotEqual(t, idA, cookie.Value, "A's ID reached the cookie %q", name)
	}
	require.Contains(t, cookies, "b_sid")
	require.Equal(t, idB, cookies["b_sid"].Value)
}

// An explicit Save of the session an outer middleware manages does nothing, as it does for the
// innermost one: the middleware saves it once, at the end.
func Test_Session_MultipleStores_ExplicitSaveIsNoOpForOuterSession(t *testing.T) {
	t.Parallel()
	recA, recB := storagetest.New(), storagetest.New()
	app := fiber.New()
	storeA, storeB := useCookieStore(app, "a_sid", Config{Storage: recA}), useCookieStore(app, "b_sid", Config{Storage: recB})

	var duringA, duringB atomic.Int64
	app.Get("/", func(c fiber.Ctx) error {
		for _, m := range []*Middleware{storeA.FromContext(c), storeB.FromContext(c)} {
			m.Set("k", "v")
			if err := m.Session.Save(); err != nil {
				return err
			}
		}
		duringA.Store(int64(len(recA.Calls(storagetest.OpSet))))
		duringB.Store(int64(len(recB.Calls(storagetest.OpSet))))
		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Do(t, app, "/")
	require.Zero(t, duringA.Load(), "the outer session was saved while the handler still ran")
	require.Zero(t, duringB.Load())
	require.Len(t, recA.Calls(storagetest.OpSet), 1, "the outer middleware saves its session once, after the handler")
	require.Len(t, recB.Calls(storagetest.OpSet), 1)
}

// With PassLocalsToContext a goroutine can keep the request context. Neither middleware may resolve
// through it once the request is over, and the outer one must not stay reachable through the inner
// one after it went back to the pool.
func Test_Session_MultipleStores_RetainedContext(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{PassLocalsToContext: true})
	storeA, storeB := useCookieStore(app, "a_sid"), useCookieStore(app, "b_sid")

	var kept atomic.Value
	var duringRequest atomic.Bool
	app.Get("/", func(c fiber.Ctx) error {
		kept.Store(c.Context())
		duringRequest.Store(storeA.FromContext(c.Context()) != nil && storeB.FromContext(c.Context()) != nil)
		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Do(t, app, "/")
	require.True(t, duringRequest.Load(), "both middleware resolve through the context while the request runs")

	ctx := kept.Load()
	require.NotNil(t, ctx)
	require.Nil(t, FromContext(ctx))
	require.Nil(t, storeA.FromContext(ctx), "the outer middleware stayed reachable after its request ended")
	require.Nil(t, storeB.FromContext(ctx))
}

// Each middleware saves its own session and reports a failed save to its own ErrorHandler; the
// failure of one does not stop the other from saving.
func Test_Session_MultipleStores_SaveErrorsGoToTheirOwnHandler(t *testing.T) {
	t.Parallel()
	var handledByA, handledByB atomic.Int64
	storageA := storagetest.New()
	app := fiber.New()
	storeA := useCookieStore(app, "a_sid", Config{Storage: storageA, ErrorHandler: func(fiber.Ctx, error) { handledByA.Add(1) }})
	storeB := useCookieStore(app, "b_sid", Config{
		Storage: storagetest.New().SetErr(storagetest.OpSet, "*", errors.New("storage unavailable")),
		ErrorHandler: func(c fiber.Ctx, _ error) {
			handledByB.Add(1)
			_ = c.Status(fiber.StatusInternalServerError).SendString("b failed") //nolint:errcheck // the test inspects the response
		},
	})
	app.Get("/", setInSessions("k", "v", storeA, storeB))

	_, body := sendExpecting(t, app, "/", nil, fiber.StatusInternalServerError)
	require.Equal(t, "b failed", body, "the handler of the middleware whose save failed answered")
	require.Equal(t, int64(1), handledByB.Load())
	require.Zero(t, handledByA.Load(), "A's save did not fail, so A's handler was not called")
	require.Len(t, storageA.Calls(storagetest.OpSet), 1, "A saved after B failed")
}

// Next skips only the middleware it belongs to.
func Test_Session_MultipleStores_NextSkipsOnlyItsOwn(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	storeA := useCookieStore(app, "a_sid", Config{Next: func(c fiber.Ctx) bool { return c.Query("skip-a") != "" }})
	storeB := useCookieStore(app, "b_sid")
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(fmt.Sprint(storeA.FromContext(c) != nil, storeB.FromContext(c) != nil, FromContext(c) != nil))
	})

	resp, body := sendRequest(t, app, "/", nil)
	require.Equal(t, "true true true", body)
	require.Contains(t, cookiesByName(resp), "a_sid")

	resp, body = sendRequest(t, app, "/?skip-a=1", nil)
	require.Equal(t, "false true true", body, "A did not run; B did, and is the innermost")
	require.NotContains(t, cookiesByName(resp), "a_sid")
	require.Contains(t, cookiesByName(resp), "b_sid")
}

// Many requests at once through two nested middleware: nothing leaks from one request's sessions
// into another's. Run with -race.
func Test_Session_MultipleStores_Concurrent(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	storeA, storeB := useCookieStore(app, "a_sid"), useCookieStore(app, "b_sid")
	app.Get("/set", func(c fiber.Ctx) error {
		storeA.FromContext(c).Set("n", c.Query("n"))
		storeB.FromContext(c).Set("n", "b"+c.Query("n"))
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/get", func(c fiber.Ctx) error {
		return c.SendString(fmt.Sprintf("%v|%v", storeA.FromContext(c).Get("n"), storeB.FromContext(c).Get("n")))
	})

	// The clients only record what they saw; the test goroutine judges it.
	const clients, reads = 32, 5
	client := func(n int) ([]string, error) {
		resp, _, err := doRequest(app, fmt.Sprintf("/set?n=%d", n), nil)
		if err != nil {
			return nil, err
		}
		cookies := cookiesByName(resp)
		if cookies["a_sid"] == nil || cookies["b_sid"] == nil {
			return nil, errors.New("a session cookie is missing")
		}
		var bodies []string
		for range reads {
			_, body, getErr := doRequest(app, "/get", map[string]string{"a_sid": cookies["a_sid"].Value, "b_sid": cookies["b_sid"].Value})
			if getErr != nil {
				return nil, getErr
			}
			bodies = append(bodies, body)
		}
		return bodies, nil
	}
	bodies, errs := make([][]string, clients), make([]error, clients)
	var wg sync.WaitGroup
	for n := range clients {
		wg.Go(func() { bodies[n], errs[n] = client(n) })
	}
	wg.Wait()

	for n := range clients {
		require.NoError(t, errs[n], "client %d", n)
		require.Len(t, bodies[n], reads)
		for _, body := range bodies[n] {
			require.Equal(t, fmt.Sprintf("%d|b%d", n, n), body, "client %d saw another request's data", n)
		}
	}
}

// Once the middleware that ran inside another has finished, the outer one is the current middleware
// of the request again: code that runs between the two after c.Next() returns still finds it, and
// an explicit Save of its session is still recognized as the middleware's to make.
func Test_Session_MultipleStores_OuterCurrentAgainAfterInnerFinishes(t *testing.T) {
	t.Parallel()
	for _, pass := range []bool{false, true} {
		t.Run(fmt.Sprintf("PassLocalsToContext=%t", pass), func(t *testing.T) {
			t.Parallel()
			recA := storagetest.New()
			handlerA, storeA := NewWithStore(Config{Storage: recA, Extractor: extractors.FromCookie("a_sid")})
			handlerB, storeB := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})

			var after atomic.Value
			var writesDuringSave atomic.Int64
			app := fiber.New(fiber.Config{PassLocalsToContext: pass})
			app.Use(handlerA)
			app.Use(func(c fiber.Ctx) error {
				err := c.Next() // B runs and finishes inside this call

				a := storeA.FromContext(c)
				fromContext := "-"
				if pass {
					fromContext = strconv.FormatBool(storeA.FromContext(c.Context()) == a)
				}
				after.Store(fmt.Sprintf("%t,%t,%t,%s", a != nil, FromContext(c) == a, storeB.FromContext(c) == nil, fromContext))
				if a != nil {
					if saveErr := a.Session.Save(); saveErr != nil {
						return saveErr
					}
					writesDuringSave.Store(int64(len(recA.Calls(storagetest.OpSet))))
				}
				return err
			})
			app.Use(handlerB)
			app.Get("/", setInSessions("k", "v", storeA, storeB))

			apptest.Do(t, app, "/")

			want := "true,true,true,-"
			if pass {
				want = "true,true,true,true"
			}
			require.Equal(t, want, after.Load(), "A is current again, B is gone")
			require.Zero(t, writesDuringSave.Load(), "A's session is still the middleware's to save")
			require.Len(t, recA.Calls(storagetest.OpSet), 1, "and it is saved once, when A returns")
		})
	}
}

// The login-callback example of the documentation, step by step: the login session ends with the
// flow, the application session starts under a new ID.
func Test_Session_MultipleStores_LoginCallbackExample(t *testing.T) {
	t.Parallel()
	loginHandler, loginStore := NewWithStore(Config{
		Extractor:   extractors.FromCookie("login_sid"),
		IdleTimeout: 10 * time.Minute,
	})
	appHandler, appStore := NewWithStore(Config{
		Extractor:       extractors.FromCookie("app_sid"),
		AbsoluteTimeout: 24 * time.Hour,
	})

	app := fiber.New()
	app.Use(loginHandler, appHandler)
	app.Get("/login", func(c fiber.Ctx) error {
		loginStore.FromContext(c).Set("state", "xyz")
		return c.SendString("redirect to the provider")
	})
	app.Get("/auth/callback", func(c fiber.Ctx) error {
		login := loginStore.FromContext(c)
		user := appStore.FromContext(c)

		state, ok := GetAs[string](login, "state")
		if !ok || state != c.Query("state") {
			return fiber.ErrBadRequest
		}

		if err := login.Destroy(); err != nil {
			return err
		}
		if err := user.Regenerate(); err != nil {
			return err
		}
		user.Set("user_id", 123)

		return c.SendString("signed in")
	})
	app.Get("/me", func(c fiber.Ctx) error {
		id, ok := GetAs[int](appStore.FromContext(c), "user_id")
		_, hasState := GetAs[string](loginStore.FromContext(c), "state")
		return c.SendString(fmt.Sprintf("%d/%v/%v", id, ok, hasState))
	})

	resp, _ := sendRequest(t, app, "/login", nil)
	loginID := cookiesByName(resp)["login_sid"].Value
	require.Equal(t, 600, cookiesByName(resp)["login_sid"].MaxAge, "the login session has its own, short lifetime")

	sendExpecting(t, app, "/auth/callback?state=wrong", map[string]string{"login_sid": loginID}, fiber.StatusBadRequest) // a callback with the wrong state is refused

	resp, body := sendRequest(t, app, "/auth/callback?state=xyz", map[string]string{"login_sid": loginID})
	require.Equal(t, "signed in", body)
	cookies := cookiesByName(resp)
	require.Negative(t, cookies["login_sid"].MaxAge, "the login session is over: its cookie is expired")
	appID := cookies["app_sid"].Value
	require.NotEmpty(t, appID)
	require.Greater(t, cookies["app_sid"].MaxAge, 1700, "the application session lives by its own idle timeout")

	_, body = sendRequest(t, app, "/me", map[string]string{"app_sid": appID})
	require.Equal(t, "123/true/false", body)

	_, body = sendRequest(t, app, "/me", map[string]string{"app_sid": appID, "login_sid": loginID})
	require.Equal(t, "123/true/false", body, "the login session cannot be revived with its old cookie")
}

// Benchmark_Middleware_TwoStores is a request through two session middleware, each over its own
// cookie, that reaches both sessions through their stores.
func Benchmark_Middleware_TwoStores(b *testing.B) {
	app := fiber.New()
	storeA := useCookieStore(app, "a_sid", Config{KeyGenerator: func() string { return "session-a" }})
	storeB := useCookieStore(app, "b_sid", Config{KeyGenerator: func() string { return "session-b" }})
	app.Get("/", setInSessions("john", "doe", storeA, storeB))
	handler := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")
	// The first request creates both sessions, the rest load them, so the storages do not grow
	// while the benchmark runs.
	handler(fctx)

	b.ReportAllocs()
	for b.Loop() {
		fctx.ResetUserValues()
		fctx.Response.Reset()
		fctx.Request.Header.SetCookie("a_sid", "session-a")
		fctx.Request.Header.SetCookie("b_sid", "session-b")
		handler(fctx)
	}
	if fctx.Response.StatusCode() != fiber.StatusOK {
		b.Fatalf("status %d", fctx.Response.StatusCode())
	}
}

// A store that is asked again finds the session it saved, whatever other stores were asked in
// between, for a visitor who has no cookie yet: the ID that a store issues is cached for that
// store, and for the stores that share a cookie or header with it, and one that another store
// issues takes neither's place.
func Test_Session_Store_Get_AskedAgainFindsTheSessionItSaved(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})
	storeC := NewStore(Config{Extractor: extractors.FromCookie("c_sid")})

	// found asks store again, and expects the session saved before.
	found := func(t *testing.T, c fiber.Ctx, store *Store, want *Session) {
		t.Helper()
		sess := mustGet(t, store, c)
		require.Equal(t, want.ID(), sess.ID(), "the session saved before was lost within the request")
		require.False(t, sess.Fresh())
		require.Equal(t, want.Get("k"), sess.Get("k"))
	}

	t.Run("after other stores were asked, in any order", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, app)

		a, b, cc := saveNew(t, c, storeA), saveNew(t, c, storeB), saveNew(t, c, storeC)
		found(t, c, storeA, a)
		found(t, c, storeC, cc)
		found(t, c, storeB, b)
		found(t, c, storeA, a)
	})

	t.Run("by a store over the same cookie", func(t *testing.T) {
		t.Parallel()
		c := apptest.Ctx(t, app)

		a := saveNew(t, c, storeA)
		saveNew(t, c, storeB)
		found(t, c, NewStore(Config{Extractor: extractors.FromCookie("a_sid"), Storage: storeA.Storage}), a)
	})
}

// A request keeps the IDs that the stores asked in it issued, but one is only taken when it names a
// stored session: asked again before a save, a store issues another ID, whether another store was
// asked in between or not.
func Test_Session_Store_Get_AskedAgainIssuesANewSession(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	storeA := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	storeB := NewStore(Config{Extractor: extractors.FromCookie("b_sid")})

	for _, tc := range []struct {
		name  string
		asked []*Store
	}{
		{"after another store was asked", []*Store{storeA, storeB, storeA}},
		{"the same store asked twice", []*Store{storeA, storeA}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, app)

			issued := make(map[string]bool)
			for _, store := range tc.asked {
				sess := mustGet(t, store, c)
				require.True(t, sess.Fresh())
				require.False(t, issued[sess.ID()], "a store that is asked again issues another ID, and each store its own")
				issued[sess.ID()] = true
			}
		})
	}
}

// The nearest session middleware outside that shares a source decides, not the outermost one. The
// outer one has no cookie of its own to find a session through and holds a new, unsaved ID; the
// middle one finds its session through the cookie b; the inner one shares the cookie a with both,
// and must take the session of the middle one.
func Test_Session_MultipleStores_NearestSharingOuterMiddlewareDecides(t *testing.T) {
	t.Parallel()
	storage := memory.New()
	outerHandler, outerStore := NewWithStore(Config{Storage: storage, Extractor: extractors.FromCookie("a")})
	middleHandler, middleStore := NewWithStore(Config{Storage: storage, Extractor: extractors.Chain(extractors.FromCookie("a"), extractors.FromCookie("b"))})
	innerHandler, innerStore := NewWithStore(Config{Storage: storage, Extractor: extractors.Chain(extractors.FromCookie("a"), extractors.FromCookie("d"))})

	app := fiber.New()
	app.Use(outerHandler, middleHandler, innerHandler)
	app.Get("/read", func(c fiber.Ctx) error {
		outer, middle, inner := outerStore.FromContext(c), middleStore.FromContext(c), innerStore.FromContext(c)
		return c.SendString(fmt.Sprintf("%t|%v|%t|%s|%v|%t", outer.Fresh(), middle.Get("who"), middle.Fresh(), inner.ID(), inner.Get("who"), inner.Fresh()))
	})

	id := seedSession(t, app, middleStore, func(s *Session) { s.Set("who", "middle") })
	_, body := sendRequest(t, app, "/read", map[string]string{"b": id})
	require.Equal(t, "true|middle|false|"+id+"|middle|false", body,
		"the inner middleware took the session of the outermost one, which is new, instead of the one of the nearest")
}

// A storage that fails when a middleware looks for the session of the one it runs inside is
// reported, as it is for the ID the request names and the ones Store.Get cached. The visitor has no
// cookie, so nothing else is read that could fail too.
func Test_Session_MultipleStores_OuterIDReadErrorReachesTheErrorHandler(t *testing.T) {
	t.Parallel()
	storage := storagetest.New()
	outerHandler, _ := NewWithStore(Config{Storage: storage, Extractor: extractors.FromCookie("sid")})
	innerHandler, _ := NewWithStore(Config{Storage: storage, Extractor: extractors.FromCookie("sid")})

	app := fiber.New()
	app.Use(outerHandler, func(c fiber.Ctx) error {
		storage.SetErr(storagetest.OpGet, "*", errors.New("storage down")) // the outer middleware has loaded its session; the inner one looks for it next
		return c.Next()
	}, innerHandler)
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("reached") })

	resp, body := sendRequest(t, app, "/", nil)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode, body)
}

// The security properties of several stores serving one request, checked against random requests
// instead of hand-picked ones. Whatever cookies, headers and session IDs the client sends, valid,
// foreign, crafted or garbage:
//
//   - a session ID is only ever written to a cookie or header that a store which
//     owns that session reads;
//   - a new session never takes an ID the client supplied (no session fixation);
//   - a store only loads records that a store with the same storage, key prefix
//     and key hashing wrote (no session confusion between stores);
//   - a hashed store keeps the session ID out of its storage key;
//   - stores asked through Store.Get that read a cookie or header in common agree on
//     one session, whatever other stores were asked in between.
//
// The number of seeds is 8, and SESSION_INVARIANT_SEEDS raises it.

// invariantLedger remembers which key policy wrote each storage key, and what the checks found
// wrong where they cannot fail the test themselves, in a handler.
type invariantLedger struct {
	writers    map[string]string
	violations []string
	mu         sync.Mutex
}

func (l *invariantLedger) violatef(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.violations = append(l.violations, fmt.Sprintf(msg, args...))
}

// requireClean fails the test if anything was found wrong so far.
func (l *invariantLedger) requireClean(t *testing.T, where string) {
	t.Helper()

	l.mu.Lock()
	defer l.mu.Unlock()
	require.Empty(t, l.violations, where)
}

// invariantStorage is the storage of one store: it passes everything to a storage that stores may
// share, and tells the ledger what it writes and reads.
type invariantStorage struct {
	fiber.Storage
	ledger *invariantLedger
	group  string
	policy string
}

func (s *invariantStorage) ledgerKey(key string) string {
	return s.group + "\x00" + key
}

func (s *invariantStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.ledger.mu.Lock()
	s.ledger.writers[s.ledgerKey(key)] = s.policy
	s.ledger.mu.Unlock()
	return s.Storage.SetWithContext(ctx, key, val, exp)
}

func (s *invariantStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	val, err := s.Storage.GetWithContext(ctx, key)
	if err == nil && val != nil {
		s.ledger.mu.Lock()
		writer := s.ledger.writers[s.ledgerKey(key)]
		s.ledger.mu.Unlock()
		if writer != s.policy {
			s.ledger.violatef("a store with policy %q loaded key %q, written under policy %q", s.policy, key, writer)
		}
	}
	return val, err
}

// invariantStore is one store of the world, with what the checks need to know.
type invariantStore struct {
	*Store
	cookies []string
	headers []string
}

// shares reports whether the two stores read a cookie or a header in common.
func (a *invariantStore) shares(b *invariantStore) bool {
	return slices.ContainsFunc(append(slices.Clone(a.cookies), a.headers...), func(x string) bool {
		return slices.Contains(b.cookies, x) || slices.Contains(b.headers, x)
	})
}

// invariantWorld is the stores of one run and the IDs of the sessions saved so far, which a client
// can send back.
type invariantWorld struct {
	stores map[string]*invariantStore
	saved  map[string][]string
	ledger *invariantLedger
	rng    *rand.Rand
	// storages are the memory storages by group: the one stores may share, and the one of the
	// stores that keep to themselves.
	storages map[string]*memory.Storage
}

// The stores a request is served with: asked through Store.Get, A twice over, or stacked as session
// middleware.
var (
	storeAPINames   = []string{"A", "A2", "B", "C"}
	middlewareNames = []string{"A", "B", "C"}
)

// newWorld returns a world without stores, which stops the garbage collection of its storages when
// the test ends. A memory storage runs a goroutine of its own until it is closed, so a world that
// is not closed costs two goroutines and what they hold for as long as the process runs, which a
// fuzz target that builds a world for every input adds up to megabytes per second.
func newWorld(tb testing.TB, seed uint64) *invariantWorld {
	tb.Helper()

	w := &invariantWorld{
		stores:   make(map[string]*invariantStore),
		saved:    make(map[string][]string),
		ledger:   &invariantLedger{writers: make(map[string]string)},
		rng:      rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
		storages: map[string]*memory.Storage{"shared": memory.New(), "private": memory.New()},
	}
	tb.Cleanup(func() {
		for _, storage := range w.storages {
			require.NoError(tb, storage.Close())
		}
	})
	return w
}

// add makes the store name from cfg, over the storage of the group, and notes the cookies and
// headers it reads. The key policy it tells the ledger it writes under is the group, the prefix and
// the hashing.
func (w *invariantWorld) add(name, group string, cfg *Config, cookies, headers []string) {
	cfg.Storage = &invariantStorage{
		Storage: w.storages[group], ledger: w.ledger, group: group,
		policy: fmt.Sprintf("%s|%s|%t", group, cfg.StorageKeyPrefix, cfg.HashStorageKeys),
	}
	w.stores[name] = &invariantStore{Store: NewStore(*cfg), cookies: cookies, headers: headers}
}

// newInvariantWorld has A and A2, which read the same cookie and share everything, B, which shares
// a storage with them but neither their cookie nor their key prefix, and C, with two places to read
// and a storage of its own.
func newInvariantWorld(tb testing.TB, seed uint64) *invariantWorld {
	tb.Helper()

	w := newWorld(tb, seed)
	for _, name := range []string{"A", "A2"} {
		w.add(name, "shared", &Config{Extractor: extractors.FromCookie("a_sid"), StorageKeyPrefix: "a:"}, []string{"a_sid"}, nil)
	}
	w.add("B", "shared", &Config{Extractor: extractors.FromCookie("b_sid"), StorageKeyPrefix: "b:"}, []string{"b_sid"}, nil)
	// Store C keeps its sessions as records: everything the world checks holds for them too.
	w.add("C", "private", &Config{
		Extractor:       extractors.Chain(extractors.FromCookie("c_sid"), extractors.FromHeader("X-C")),
		HashStorageKeys: true,
		Codec:           JSONCodec{},
	}, []string{"c_sid"}, []string{"X-C"})
	return w
}

// newApp returns an app with a session middleware for each of the named stores, in that order, and
// the handler for GET /.
func (w *invariantWorld) newApp(names []string, handler fiber.Handler) *fiber.App {
	app := fiber.New()
	for _, name := range names {
		app.Use(New(Config{Store: w.stores[name].Store}))
	}
	app.Get("/", handler)
	return app
}

// storeAPIApp is an app whose handler gets the session of each store named in the ops of the query
// through Store.Get, gives it data, saves it and reports it.
func (w *invariantWorld) storeAPIApp() *fiber.App {
	var visits atomic.Int32
	return w.newApp(nil, func(c fiber.Ctx) error {
		var rec invariantRecord
		for name := range strings.SplitSeq(c.Query("ops"), ",") {
			sess, err := w.stores[name].Get(c)
			if err != nil {
				return err
			}
			sess.Set("n", int(visits.Add(1)))
			if err := sess.Save(); err != nil {
				sess.Release()
				return err
			}
			rec.Sessions = append(rec.Sessions, invariantSession{Store: name, ID: sess.ID(), Fresh: sess.Fresh()})
			sess.Release()
		}
		return c.JSON(rec)
	})
}

// middlewareApp is an app with a session middleware for each of A, B and C. Its handler reports the
// session of every one of them through Store.FromContext, and gives the ones named in the ops of
// the query some data.
func (w *invariantWorld) middlewareApp() *fiber.App {
	return w.newApp(middlewareNames, func(c fiber.Ctx) error {
		// Every middleware of the stack runs and saves its session, whichever the handler touches:
		// all of them are reported, and the ops pick the ones that get data.
		touched := strings.Split(c.Query("ops"), ",")
		var rec invariantRecord
		for _, name := range middlewareNames {
			mw := w.stores[name].FromContext(c)
			if mw == nil {
				return fmt.Errorf("no middleware for store %s", name)
			}
			if slices.Contains(touched, name) {
				mw.Set("n", 1)
			}
			rec.Sessions = append(rec.Sessions, invariantSession{Store: name, ID: mw.ID(), Fresh: mw.Fresh()})
		}
		return c.JSON(rec)
	})
}

// clientValue is what the client sends for one cookie, header or query value: nothing, the ID of a
// session that store `own` saved, a crafted variation of it, or garbage.
func (w *invariantWorld) clientValue(own string) string {
	pick := func(store string) string {
		ids := w.saved[store]
		if len(ids) == 0 {
			return "unknown"
		}
		return ids[w.rng.IntN(len(ids))]
	}
	names := []string{"A", "B", "C"}
	switch w.rng.IntN(10) {
	case 0, 1, 2:
		return ""
	case 3, 4, 5:
		return pick(own)
	case 6:
		return pick(names[w.rng.IntN(len(names))]) // a session of some store, whoever's it is
	case 7:
		return "b:" + pick("B") // as another store's key would read
	case 8:
		return "a:" + pick("A")
	default:
		return strings.Repeat("g", 1+w.rng.IntN(300)) + ":" + strconv.Itoa(w.rng.IntN(10))
	}
}

// invariantRecord is what the handler reports about the sessions it used.
type invariantRecord struct {
	Sessions []invariantSession `json:"sessions"`
}

// invariantSession is one session as the handler saw it: of which store, with which ID, and
// whether it was new.
type invariantSession struct {
	Store string `json:"store"`
	ID    string `json:"id"`
	Fresh bool   `json:"fresh"`
}

// decodeRecord reads what the handler reported. A response that is not a 200 reports nothing.
func decodeRecord(t *testing.T, status int, body []byte) invariantRecord {
	t.Helper()

	var rec invariantRecord
	if status == fiber.StatusOK {
		require.NoError(t, json.Unmarshal(body, &rec), "%s", body)
	}
	return rec
}

// invariantRequest is one request of the client.
type invariantRequest struct {
	cookies map[string]string
	header  string
	ops     []string
}

func (w *invariantWorld) newRequest(opNames []string) invariantRequest {
	req := invariantRequest{cookies: make(map[string]string)}
	for _, store := range []string{"A", "B", "C"} {
		if v := w.clientValue(store); v != "" {
			req.cookies[w.stores[store].cookies[0]] = v
		}
	}
	req.header = w.clientValue("C")
	for range 1 + w.rng.IntN(5) {
		req.ops = append(req.ops, opNames[w.rng.IntN(len(opNames))])
	}
	return req
}

// where names the request in a failure message.
func (r invariantRequest) where(step int) string {
	return fmt.Sprintf("request %d (ops %v, cookies %v, header %q)", step, r.ops, r.cookies, r.header)
}

// send plays the request against the app and decodes what the handler reported.
func (r invariantRequest) send(t *testing.T, app *fiber.App) (*http.Response, invariantRecord) {
	t.Helper()

	req := apptest.Req(fiber.MethodGet, "/?ops="+strings.Join(r.ops, ","), withCookies(r.cookies), apptest.WithHeaderIf("X-C", r.header))
	resp := apptest.Do(t, app, req)
	return resp, decodeRecord(t, resp.StatusCode, apptest.BodyBytes(t, resp))
}

// sendDirect is send without the network round trip app.Test makes: the request goes to the handler
// of the app as a fasthttp request, which is several times cheaper and what a fuzzer needs. Every
// Set-Cookie of the response has to be readable by fasthttp and by net/http, whose reading the
// checks rely on.
func (r invariantRequest) sendDirect(t *testing.T, handler fasthttp.RequestHandler) (*http.Response, invariantRecord) {
	t.Helper()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/?ops=" + strings.Join(r.ops, ","))
	for name, value := range r.cookies {
		fctx.Request.Header.SetCookie(name, value)
	}
	if r.header != "" {
		fctx.Request.Header.Set("X-C", r.header)
	}
	handler(fctx)

	resp := &http.Response{StatusCode: fctx.Response.StatusCode(), Header: make(http.Header)}
	for _, raw := range fctx.Response.Header.Cookies() {
		cookie := fasthttp.AcquireCookie()
		require.NoError(t, cookie.ParseBytes(raw), "unparsable Set-Cookie %q", raw)
		fasthttp.ReleaseCookie(cookie)
		resp.Header.Add("Set-Cookie", string(raw))
	}
	// The checks look at the cookies through net/http, which drops one it cannot read: it would escape them.
	require.Len(t, resp.Cookies(), len(resp.Header.Values("Set-Cookie")), "net/http cannot read a Set-Cookie of the response")
	resp.Header.Set("X-C", string(fctx.Response.Header.Peek("X-C")))

	return resp, decodeRecord(t, resp.StatusCode, fctx.Response.Body())
}

// requireNoFixation holds the sessions of the request against the property that a new session never
// takes an ID the client supplied.
func (r invariantRequest) requireNoFixation(t *testing.T, where string, rec invariantRecord) {
	t.Helper()

	supplied := make(map[string]bool)
	for _, v := range r.cookies {
		supplied[v] = true
	}
	if r.header != "" {
		supplied[r.header] = true
	}
	for _, s := range rec.Sessions {
		if s.Fresh {
			require.False(t, supplied[s.ID], "%s: a new session of store %s took the ID the client supplied: %q", where, s.Store, s.ID)
		}
	}
}

// requireOwned holds the cookies and the header X-C that a response sets against the sessions of
// the request: a session ID is only written to a cookie or header that a store which owns that
// session reads.
func (w *invariantWorld) requireOwned(t *testing.T, where string, rec invariantRecord, cookies []*http.Cookie, header string) {
	t.Helper()

	owns := func(name, value string, read func(*invariantStore) []string) bool {
		return slices.ContainsFunc(rec.Sessions, func(s invariantSession) bool {
			return s.ID == value && slices.Contains(read(w.stores[s.Store]), name)
		})
	}
	for _, cookie := range cookies {
		require.True(t, owns(cookie.Name, cookie.Value, func(s *invariantStore) []string { return s.cookies }),
			"%s: cookie %s=%q was written, but no store reading that cookie owns that session ID (sessions: %v)", where, cookie.Name, cookie.Value, rec.Sessions)
	}
	if header != "" {
		require.True(t, owns("X-C", header, func(s *invariantStore) []string { return s.headers }),
			"%s: header X-C=%q was written, but no store reading that header owns that session ID", where, header)
	}
}

// check holds the response and what the handler saw against the properties above.
func (w *invariantWorld) check(t *testing.T, step int, req invariantRequest, resp *http.Response, rec invariantRecord) {
	t.Helper()
	where := req.where(step)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, where)

	// A session the client names with a valid ID in the cookie its store reads is found, whatever
	// else the request carries: another store's cached ID does not take its place.
	for _, sess := range rec.Sessions {
		store := w.stores[sess.Store]
		expected := req.cookies[store.cookies[0]]
		if expected == "" && len(store.headers) > 0 {
			expected = req.header
		}
		if expected == "" || !slices.Contains(w.saved[strings.TrimSuffix(sess.Store, "2")], expected) {
			continue
		}
		require.False(t, sess.Fresh, "%s: store %s did not find the session its client named: %q", where, sess.Store, expected)
		require.Equal(t, expected, sess.ID, where)
	}

	req.requireNoFixation(t, where, rec)

	// Stores that read a cookie or a header in common are one visitor's session, whatever other
	// stores were asked in between: a store asked again, or after one that shares with it, finds
	// the session that one saved, and does not start another. Stores that read none in common keep
	// sessions of their own: they never come out with the same ID, which is what a store adopting
	// the ID another one cached on the request would do.
	for i, later := range rec.Sessions {
		for _, earlier := range rec.Sessions[:i] {
			if !w.stores[earlier.Store].shares(w.stores[later.Store]) {
				require.NotEqual(t, earlier.ID, later.ID, "%s: stores %s and %s, which share no cookie or header, ended up with the same ID %q", where, earlier.Store, later.Store, earlier.ID)
				continue
			}
			require.Equal(t, earlier.ID, later.ID, "%s: store %s, asked after store %s which shares a cookie or header with it, did not find the session that one saved", where, later.Store, earlier.Store)
			require.False(t, later.Fresh, "%s: store %s started a new session, though store %s had saved one", where, later.Store, earlier.Store)
		}
	}

	w.requireOwned(t, where, rec, resp.Cookies(), resp.Header.Get("X-C"))

	// A store only loads what a store with its key policy wrote.
	w.ledger.requireClean(t, where)

	// What was saved reaches the client: the last session saved for a cookie has its ID in that
	// cookie, or in the header of the store, since every save writes it.
	last := make(map[string]invariantSession)
	for _, sess := range rec.Sessions {
		last[w.stores[sess.Store].cookies[0]] = sess
	}
	for cookieName, sess := range last {
		written := slices.ContainsFunc(resp.Cookies(), func(c *http.Cookie) bool { return c.Name == cookieName && c.Value == sess.ID }) ||
			slices.ContainsFunc(w.stores[sess.Store].headers, func(h string) bool { return resp.Header.Get(h) == sess.ID })
		require.True(t, written, "%s: the session %q of store %s was saved, but its ID reached the client in none of the cookies and headers of the store", where, sess.ID, sess.Store)
	}

	// A hashed store keeps the ID out of its storage key.
	for _, sess := range rec.Sessions {
		if sess.Store == "C" {
			key := w.stores["C"].storageKey(sess.ID)
			require.NotContains(t, key, sess.ID, where)
			require.Len(t, key, len(keyNamespace)+hashedKeyLen, where)
		}
	}

	// What was saved can be sent back.
	for _, sess := range rec.Sessions {
		owner := strings.TrimSuffix(sess.Store, "2")
		w.saved[owner] = append(w.saved[owner], sess.ID)
	}
}

func invariantSeeds() uint64 {
	if n, err := strconv.ParseUint(os.Getenv("SESSION_INVARIANT_SEEDS"), 10, 64); err == nil && n > 0 {
		return n
	}
	return 8
}

// randomRequests sends 150 random requests for each of the seeds, to an app of its own that newApp
// makes over a world of its own, and holds every response against the properties. The ops of a
// request are drawn from the names.
func randomRequests(t *testing.T, newApp func(*invariantWorld) *fiber.App, names []string) {
	t.Helper()

	for seed := range invariantSeeds() {
		t.Run("seed="+strconv.FormatUint(seed, 10), func(t *testing.T) {
			t.Parallel()
			w := newInvariantWorld(t, seed)
			app := newApp(w)

			for step := range 150 {
				req := w.newRequest(names)
				resp, rec := req.send(t, app)
				w.check(t, step, req, resp, rec)
			}
		})
	}
}

// Several stores used through Store.Get, in random order and repeatedly, within one request.
func Test_Session_Invariants_StoreAPI(t *testing.T) {
	t.Parallel()
	randomRequests(t, (*invariantWorld).storeAPIApp, storeAPINames)
}

// The same stores as stacked session middleware, reached through Store.FromContext.
func Test_Session_Invariants_Middleware(t *testing.T) {
	t.Parallel()
	randomRequests(t, (*invariantWorld).middlewareApp, middlewareNames)
}

// A store writes a session ID back only to its own cookies and headers. A store that shares the ID
// another store cached, but not the cookie it came from (a store over the second cookie of a
// chain), must not write it, or the new one of a Regenerate, to that cookie, with its own
// attributes.
func Test_Session_MultipleStores_AdoptedIDIsNotWrittenToAnotherStoresCookie(t *testing.T) {
	t.Parallel()
	storage := memory.New()
	outerHandler, outerStore := NewWithStore(Config{
		Storage:      storage,
		Extractor:    extractors.Chain(extractors.FromCookie("sid"), extractors.FromCookie("shared")),
		CookieSecure: true, CookieHTTPOnly: true,
	})
	innerHandler, innerStore := NewWithStore(Config{Storage: storage, Extractor: extractors.FromCookie("shared")})

	app := fiber.New()
	app.Use(outerHandler, innerHandler)
	app.Get("/seed", func(c fiber.Ctx) error {
		outerStore.FromContext(c).Set("user", "alice")
		return c.SendString(outerStore.FromContext(c).ID())
	})
	app.Get("/regen", func(c fiber.Ctx) error {
		m := innerStore.FromContext(c)
		if err := m.Regenerate(); err != nil {
			return err
		}
		return c.SendString(m.ID())
	})

	_, id := sendRequest(t, app, "/seed", nil)
	resp, newID := sendRequest(t, app, "/regen", map[string]string{"sid": id})
	require.NotEqual(t, id, newID, "the inner store shares the ID, and rotated it")
	for _, cookie := range resp.Cookies() {
		if cookie.Name != "sid" {
			continue
		}
		require.NotEqual(t, newID, cookie.Value, "the inner store wrote its ID to the outer store's cookie")
		require.True(t, cookie.Secure && cookie.HttpOnly, "the outer store's cookie was written with the inner store's attributes")
	}
	require.Equal(t, newID, cookiesByName(resp)["shared"].Value, "the new ID goes to the inner store's own cookie")
}

// A session stored with an absolute deadline keeps it on a store without an AbsoluteTimeout, as
// after the option was turned off: once it has passed, the request path starts the session over,
// once, and without a deadline, where a deadline of now would start it over on every request; and
// GetByID no longer finds it, as on a store with the option.
func Test_Session_DeadlineHonouredWithoutAbsoluteTimeout(t *testing.T) {
	t.Parallel()
	past := time.Now().Add(-time.Minute)
	for name, tc := range map[string]struct {
		codec Codec
		seed  func(t *testing.T, mem *memory.Storage, key string)
	}{
		"record": {
			codec: JSONCodec{},
			seed: func(t *testing.T, mem *memory.Storage, key string) {
				t.Helper()
				raw, err := encodeRecord("json", deadlineOf(past),
					[]recordEntry{{key: "k", value: []byte(`"v"`)}})
				require.NoError(t, err)
				require.NoError(t, mem.Set(key, raw, time.Hour))
			},
		},
		"gob": {
			seed: func(t *testing.T, mem *memory.Storage, key string) {
				t.Helper()
				gob.Register(absExpirationKey)
				gob.Register(time.Time{})
				var buf bytes.Buffer
				require.NoError(t, gob.NewEncoder(&buf).Encode(map[any]any{absExpirationKey: past, "k": "v"}))
				require.NoError(t, mem.Set(key, buf.Bytes(), time.Hour))
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mem := memory.New()
			store := NewStore(Config{Storage: mem, Codec: tc.codec})
			tc.seed(t, mem, string(store.storageKey("by-id")))
			tc.seed(t, mem, string(store.storageKey("by-request")))

			_, err := store.GetByID(context.Background(), "by-id")
			require.ErrorIs(t, err, ErrSessionIDNotFoundInStore, "GetByID handed out a session past its deadline")
			stored, err := mem.Get(string(store.storageKey("by-id")))
			require.NoError(t, err)
			require.Nil(t, stored, "and left its record")

			app := fiber.New()
			app.Use(New(Config{Store: store}))
			app.Get("/", func(c fiber.Ctx) error {
				m := FromContext(c)
				_, hasDeadline := m.AbsoluteExpiry()
				m.Set("login", "done")
				return c.SendString(fmt.Sprintf("%t|%t|%v", m.Fresh(), hasDeadline, m.Get("k")))
			})
			resp, body := sendRequest(t, app, "/", map[string]string{"session_id": "by-request"})
			require.Equal(t, "true|false|<nil>", body, "the session past its deadline was not started over, or was given a deadline")
			id := cookiesByName(resp)["session_id"].Value
			require.NotEqual(t, "by-request", id)

			time.Sleep(2 * time.Millisecond)
			_, body = sendRequest(t, app, "/", map[string]string{"session_id": id})
			require.Equal(t, "false|false|<nil>", body, "the session that took its place was started over again")
		})
	}
}

// Once the session middleware has finished with the request, a Store.Get reads the session ID from
// the request again, and finds the session of the cookie.
func Test_Store_Get_AfterTheMiddlewareEndedReadsTheRequest(t *testing.T) {
	t.Parallel()
	store := NewStore()
	var after atomic.Value // string
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		err := c.Next()
		sess, getErr := store.Get(c)
		if getErr != nil {
			return getErr
		}
		defer sess.Release()
		after.Store(fmt.Sprintf("%t|%s", sess.Fresh(), sess.ID()))
		return err
	})
	app.Use(New(Config{Store: store}))
	app.Get("/", func(c fiber.Ctx) error {
		FromContext(c).Set("k", "v")
		return c.SendString(FromContext(c).ID())
	})

	_, id := sendRequest(t, app, "/", nil)
	sendRequest(t, app, "/", map[string]string{"session_id": id})
	require.Equal(t, "false|"+id, after.Load(), "Store.Get after the middleware ended started a new session instead of reading the cookie")
}
