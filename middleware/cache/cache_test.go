// Special thanks to @codemicro for moving this to fiber core
// Original middleware: github.com/codemicro/fiber-cache
package cache

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/clocktest"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/internal/storagetest"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/encryptcookie"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// mutatingStorage rewrites each value with mutate on its way in, so a test can plant values the cache never writes
// itself; a nil mutate leaves them alone. It keeps the slice it is given, where Memory stores a copy, so a value the
// cache hands over as a view of a buffer it still owns shows when that buffer changes.
type mutatingStorage struct {
	*storagetest.Memory
	mutate func(key string, value []byte) []byte
}

var storageKinds = []string{"memory", "external"}

func storageOf(kind string) fiber.Storage {
	if kind == "external" {
		return storagetest.New()
	}
	return nil
}

type reply struct {
	header http.Header
	cache  string
	body   string
	status int
}

// doRequest runs req and returns the reply, its body read and closed.
func doRequest(t *testing.T, app *fiber.App, req *http.Request) reply {
	t.Helper()

	resp := apptest.Do(t, app, req)
	body := apptest.Body(t, resp)
	return reply{status: resp.StatusCode, header: resp.Header, cache: resp.Header.Get("X-Cache"), body: body}
}

// getRoot sends GET / with the headers.
func getRoot(t *testing.T, app *fiber.App, headers map[string]string) reply {
	t.Helper()

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	return doRequest(t, app, req)
}

func newMutatingStorage(mutate func(key string, value []byte) []byte) *mutatingStorage {
	return &mutatingStorage{Memory: storagetest.New(), mutate: mutate}
}

func (s *mutatingStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	if key == "" || len(val) == 0 {
		return nil
	}
	if s.mutate != nil {
		val = s.mutate(key, val)
	}
	if err := s.Memory.SetWithContext(ctx, key, val, exp); err != nil {
		return err
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.Data[key] = val
	return nil
}

func (s *mutatingStorage) Set(key string, val []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, exp)
}

// bodyKeys returns the keys under which storage holds a response body.
func bodyKeys(storage *storagetest.Memory) []string {
	return slices.DeleteFunc(storage.Keys(), func(key string) bool { return !strings.HasPrefix(key, bodyKeyPrefix) })
}

// blockingDeleteStorage pauses the first stale metadata deletion while another
// request replaces that entry and reuses its heap index.
type blockingDeleteStorage struct {
	*storagetest.Memory
	deleteStarted  chan struct{}
	continueDelete chan struct{}
	key            string
	blockOnce      atomic.Bool
}

func (s *blockingDeleteStorage) DeleteWithContext(ctx context.Context, key string) error {
	if key == s.key && s.blockOnce.CompareAndSwap(false, true) {
		close(s.deleteStarted)
		<-s.continueDelete
	}
	return s.Memory.DeleteWithContext(ctx, key)
}

// restoreMutationStorage models a backend change while eviction has dropped
// the old heap node and its storage deletion fails.
type restoreMutationStorage struct {
	*storagetest.Memory
	key      string
	mutation string
	armed    atomic.Bool
}

type blockingFailedEvictionStorage struct {
	*storagetest.Memory
	started        chan struct{}
	continueDelete chan struct{}
	key            string
	armed          atomic.Bool
}

func (s *blockingFailedEvictionStorage) DeleteWithContext(ctx context.Context, key string) error {
	if key == s.key && s.armed.CompareAndSwap(true, false) {
		close(s.started)
		<-s.continueDelete
		return errors.New("eviction delete failed")
	}
	return s.Memory.DeleteWithContext(ctx, key)
}

func (s *restoreMutationStorage) DeleteWithContext(ctx context.Context, key string) error {
	if key != s.key || !s.armed.CompareAndSwap(true, false) {
		return s.Memory.DeleteWithContext(ctx, key)
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.mutation == "missing" {
		delete(s.Data, key)
		delete(s.Data, bodyKey(key))
	} else {
		var entry item
		if _, err := entry.UnmarshalMsg(s.Data[key]); err != nil {
			return err
		}
		entry.heapgen += 1000
		raw, err := entry.MarshalMsg(nil)
		if err != nil {
			return err
		}
		s.Data[key] = raw
	}
	return errors.New("eviction delete failed")
}

// cacheTestAFailures keeps the last error that failed a request of the app it serves.
type cacheTestAFailures struct{ err error }

// app returns an app that answers 500 to a request that fails with an error, and keeps that error.
func (f *cacheTestAFailures) app() *fiber.App {
	return fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			f.err = err
			return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	})
}

func TestCacheStorageErrors(t *testing.T) {
	t.Parallel()
	key := defaultKey("/")

	for _, tc := range []struct {
		name    string
		op      string // the failing storage operation
		key     string // the key it fails on
		wantErr string
		expired bool // the storage already holds an expired entry for the request
	}{
		{name: "get", op: storagetest.OpGet, key: key, wantErr: "cache: failed to get key"},
		{name: "set", op: storagetest.OpSet, key: bodyKey(key), wantErr: "cache: failed to store raw key"},
		{
			name: "delete", op: storagetest.OpDelete, key: key,
			wantErr: "cache: failed to delete expired key", expired: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			storage := storagetest.New()
			storage.SetErr(tc.op, tc.key, errors.New("boom"))

			if tc.expired {
				// Use an obviously expired timestamp without relying on time-based conversions
				raw, err := (&item{exp: 1}).MarshalMsg(nil)
				require.NoError(t, err)
				storage.Data[key] = raw
			}

			var failures cacheTestAFailures
			app := failures.app()
			app.Use(New(Config{Storage: storage, Expiration: time.Second}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

			apptest.Expect(t, app, "/", fiber.StatusInternalServerError)
			require.Error(t, failures.err)
			require.ErrorContains(t, failures.err, tc.wantErr)
		})
	}
}

func TestCacheEvictionPropagatesRequestContextToDelete(t *testing.T) {
	t.Parallel()
	storage := storagetest.New()
	app := fiber.New()

	app.Use(func(c fiber.Ctx) error {
		path := c.Path()
		if path == "/first" {
			c.SetContext(storagetest.Marked("first"))
		}
		if path == "/second" {
			c.SetContext(storagetest.MarkedCanceled("evict"))
		}
		return c.Next()
	})

	// Room for /second alone, so storing it evicts /first.
	app.Use(New(Config{Storage: storage, Expiration: time.Minute, MaxBytes: charged(defaultKey("/second"), len("bbbb"), textPlain)}))

	app.Get("/first", func(c fiber.Ctx) error { return c.SendString("aaa") })

	app.Get("/second", func(c fiber.Ctx) error { return c.SendString("bbbb") })

	apptest.Expect(t, app, "/first", fiber.StatusOK)

	apptest.Expect(t, app, "/second", fiber.StatusOK)

	records := storage.Calls(storagetest.OpDelete)
	require.Len(t, records, 2)

	var keys []string
	for _, rec := range records {
		keys = append(keys, rec.Key)
		require.Equal(t, "evict", rec.Marker)
		require.True(t, rec.Canceled)
	}

	require.ElementsMatch(t, []string{defaultKey("/first"), bodyKey(defaultKey("/first"))}, keys)
}

func TestCacheCleanupPropagatesRequestContextToDelete(t *testing.T) {
	t.Parallel()
	storage := storagetest.New()
	storage.SetErr(storagetest.OpSet, defaultKey("/"), errors.New("boom"))

	var failures cacheTestAFailures
	app := failures.app()

	app.Use(func(c fiber.Ctx) error {
		c.SetContext(storagetest.MarkedCanceled("cleanup"))
		return c.Next()
	})

	app.Use(New(Config{Storage: storage, Expiration: time.Minute}))

	app.Get("/", func(c fiber.Ctx) error { return c.SendString("payload") })

	apptest.Expect(t, app, "/", fiber.StatusInternalServerError)
	require.Error(t, failures.err)
	require.ErrorContains(t, failures.err, "cache: failed to store key")

	records := storage.Calls(storagetest.OpDelete)
	require.Len(t, records, 1)
	require.Equal(t, bodyKey(defaultKey("/")), records[0].Key)
	require.Equal(t, "cleanup", records[0].Marker)
	require.True(t, records[0].Canceled)
}

func TestCacheStorageOperationsObserveRequestContext(t *testing.T) {
	t.Parallel()
	storage := storagetest.New()
	app := fiber.New()

	app.Use(func(c fiber.Ctx) error {
		ctxLabel := string(c.Request().Header.Peek("X-Context"))
		if ctxLabel == "" {
			return c.Next()
		}

		canceled := string(c.Request().Header.Peek("X-Cancel")) == "true"
		if canceled {
			c.SetContext(storagetest.MarkedCanceled(ctxLabel))
		} else {
			c.SetContext(storagetest.Marked(ctxLabel))
		}
		return c.Next()
	})

	app.Use(New(Config{Storage: storage, Expiration: time.Minute}))

	app.Get("/cache", func(c fiber.Ctx) error { return c.SendString("payload") })

	for _, label := range []string{"store", "fetch"} {
		apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/cache",
			apptest.WithHeader("X-Context", label), apptest.WithHeader("X-Cancel", "true")), fiber.StatusOK)
	}

	entryKey := defaultKey("/cache")

	setRecords := storage.Calls(storagetest.OpSet)
	require.Len(t, setRecords, 2)
	for _, rec := range setRecords {
		require.Contains(t, []string{entryKey, bodyKey(entryKey)}, rec.Key)
		require.Equal(t, "store", rec.Marker)
		require.True(t, rec.Canceled)
	}

	getRecords := storage.Calls(storagetest.OpGet)
	require.NotEmpty(t, getRecords)

	var fetchEntry, fetchBody bool
	for _, rec := range getRecords {
		if rec.Marker != "fetch" {
			continue
		}

		if rec.Key == entryKey {
			require.True(t, rec.Canceled)
			fetchEntry = true
		}
		if rec.Key == bodyKey(entryKey) {
			require.True(t, rec.Canceled)
			fetchBody = true
		}
	}

	require.True(t, fetchEntry, "expected cached entry retrieval to observe request context")
	require.True(t, fetchBody, "expected cached body retrieval to observe request context")
}

func Test_Cache_CacheControl(t *testing.T) {
	t.Parallel()
	for _, disabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("DisableCacheControl=%t", disabled), func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(Config{Expiration: 10 * time.Second, DisableCacheControl: disabled}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("Hello, World!") })

			apptest.Do(t, app, "/")
			cc := apptest.Do(t, app, "/").Header.Get(fiber.HeaderCacheControl)
			if disabled {
				require.Empty(t, cc)
				return
			}
			// max-age can be 10 or 11 depending on sub-second timing between store and read
			require.Contains(t, []string{"public, max-age=10", "public, max-age=11"}, cc, "unexpected Cache-Control")
		})
	}
}

// cacheTestAOrigin returns an app that serves / for every method through mw, the
// cache middleware under test, from an origin that counts its calls: the nth
// call answers prefix followed by n and sets the response headers given as name
// and value pairs. The counter tells how many requests reached the origin.
func cacheTestAOrigin(mw fiber.Handler, prefix string, headers ...string) (*fiber.App, *atomic.Int32) {
	app := fiber.New()
	app.Use(mw)

	calls := new(atomic.Int32)
	app.All("/", func(c fiber.Ctx) error {
		n := calls.Add(1)
		for name, value := range apptest.Pairs(headers) {
			c.Set(name, value)
		}
		return c.SendString(prefix + strconv.Itoa(int(n)))
	})
	return app, calls
}

func Test_Cache_Expired(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		expiration time.Duration
		wait       time.Duration
		recached   bool // the answer to the request after the expired one is cached again
	}{
		{name: "expired entry is replaced", expiration: 2 * time.Second, wait: 3 * time.Second, recached: true},
		{name: "nothing to cache", expiration: -(time.Second * 1), wait: 500 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, _ := cacheTestAOrigin(New(Config{Expiration: tc.expiration}), "")

			resp := apptest.Do(t, app, "/")
			body := apptest.BodyBytes(t, resp)

			time.Sleep(tc.wait)

			respCached := apptest.Do(t, app, "/")
			bodyCached := apptest.BodyBytes(t, respCached)

			if bytes.Equal(body, bodyCached) {
				t.Errorf("Cache should have expired: %s, %s", body, bodyCached)
			}
			if !tc.recached {
				return
			}

			respCachedNextRound := apptest.Do(t, app, "/")
			bodyCachedNextRound := apptest.BodyBytes(t, respCachedNextRound)

			if !bytes.Equal(bodyCachedNextRound, bodyCached) {
				t.Errorf("Cache should not have expired: %s, %s", bodyCached, bodyCachedNextRound)
			}
		})
	}
}

func Test_Cache(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		mw   fiber.Handler
		name string
	}{
		{name: "query keys disabled", mw: New(Config{DisableQueryKeys: true})},
		{name: "invalid expiration", mw: New(Config{Expiration: 0 * time.Second})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, _ := cacheTestAOrigin(tc.mw, "")

			resp := apptest.Do(t, app, "/")

			cachedResp := apptest.Do(t, app, "/")

			body := apptest.BodyBytes(t, resp)
			cachedBody := apptest.BodyBytes(t, cachedResp)

			require.Equal(t, cachedBody, body)
		})
	}
}

// cacheTestAExpect sends target through app and requires the X-Cache header of the response
// to be want. The body is read and stays readable from the returned response.
func cacheTestAExpect[T apptest.Target](t *testing.T, app *fiber.App, target T, want string) *http.Response {
	t.Helper()

	resp := apptest.Do(t, app, target)
	require.Equal(t, want, resp.Header.Get("X-Cache"), "%s %s: X-Cache", resp.Request.Method, resp.Request.URL)
	apptest.BodyBytes(t, resp)
	return resp
}

// cacheTestAExpectBody is cacheTestAExpect that also requires the response body
// to be body.
func cacheTestAExpectBody[T apptest.Target](t *testing.T, app *fiber.App, target T, want, body string) *http.Response {
	t.Helper()

	resp := cacheTestAExpect(t, app, target, want)
	require.Equal(t, body, apptest.Body(t, resp), "%s %s: body", resp.Request.Method, resp.Request.URL)
	return resp
}

// cacheTestAExpectStatus is cacheTestAExpect that also requires the response
// status to be status.
func cacheTestAExpectStatus[T apptest.Target](t *testing.T, app *fiber.App, target T, want string, status int) *http.Response {
	t.Helper()

	resp := apptest.Expect(t, app, target, status)
	require.Equal(t, want, resp.Header.Get("X-Cache"), "%s %s: X-Cache", resp.Request.Method, resp.Request.URL)
	return resp
}

// cacheTestAGetCC returns a GET request for target that carries the
// Cache-Control request header cacheControl.
func cacheTestAGetCC(target, cacheControl string) *http.Request {
	return apptest.Req(fiber.MethodGet, target, apptest.WithHeader(fiber.HeaderCacheControl, cacheControl))
}

// cacheTestAServe answers every request with body, under the Cache-Control response header cacheControl.
func cacheTestAServe(cacheControl, body string) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, cacheControl)
		return c.SendString(body)
	}
}

// cacheTestAIDApp returns an app that serves GET / through the middleware mw and
// answers it with the id query parameter (default 1), so that a body says which
// request produced it.
func cacheTestAIDApp(mw ...fiber.Handler) *fiber.App {
	app := fiber.New()
	for _, h := range mw {
		app.Use(h)
	}
	app.Get("/", func(c fiber.Ctx) error { return c.SendString(fiber.Query(c, "id", "1")) })
	return app
}

func Test_Cache_WithNoCacheRequestDirective(t *testing.T) {
	t.Parallel()
	// The query is not in the key, so every request below shares one entry.
	app := cacheTestAIDApp(New(Config{DisableQueryKeys: true}))

	cacheTestAExpectBody(t, app, "/", cacheMiss, "1")
	cacheTestAExpectBody(t, app, "/?id=2", cacheHit, "1")
	cacheTestAExpectBody(t, app, cacheTestAGetCC("/?id=2", noCache), cacheMiss, "2")
	cacheTestAExpectBody(t, app, cacheTestAGetCC("/?id=2", noCache), cacheMiss, "2")
	cacheTestAExpectBody(t, app, cacheTestAGetCC("/?id=3", "NO-CACHE"), cacheMiss, "3")
	// my-no-cache is a directive of its own, not no-cache.
	cacheTestAExpectBody(t, app, cacheTestAGetCC("/?id=4", "my-no-cache"), cacheHit, "3")
	cacheTestAExpectBody(t, app, "/?id=4", cacheHit, "3")
	cacheTestAExpectBody(t, app, "/", cacheHit, "3")
}

func Test_Cache_WithETagAndNoCacheRequestDirective(t *testing.T) {
	t.Parallel()
	app := cacheTestAIDApp(etag.New(), New(Config{DisableQueryKeys: true}))

	// withETag is a GET for target that carries the Cache-Control request header
	// and the entity tag the client holds.
	withETag := func(target, cacheControl, etagToken string) *http.Request {
		return apptest.Req(fiber.MethodGet, target,
			apptest.WithHeader(fiber.HeaderCacheControl, cacheControl),
			apptest.WithHeader(fiber.HeaderIfNoneMatch, etagToken))
	}

	etagToken := cacheTestAExpectStatus(t, app, "/", cacheMiss, fiber.StatusOK).Header.Get("Etag")
	cacheTestAExpectStatus(t, app, apptest.Req(fiber.MethodGet, "/?id=2", apptest.WithHeader(fiber.HeaderIfNoneMatch, etagToken)),
		cacheHit, fiber.StatusNotModified)
	resp := cacheTestAExpectStatus(t, app, withETag("/?id=2", noCache, etagToken), cacheMiss, fiber.StatusOK)
	etagToken = resp.Header.Get("Etag")
	cacheTestAExpectStatus(t, app, withETag("/?id=3", "NO-CACHE", etagToken), cacheMiss, fiber.StatusOK)
	cacheTestAExpectStatus(t, app, withETag("/?id=2", noCache, etagToken), cacheMiss, fiber.StatusNotModified)
	cacheTestAExpectStatus(t, app, "/", cacheHit, fiber.StatusOK)
}

func Test_Cache_WithNoStoreRequestDirective(t *testing.T) {
	t.Parallel()
	app := cacheTestAIDApp(New(Config{DisableQueryKeys: true}))

	apptest.ExpectBody(t, app, cacheTestAGetCC("/?id=2", noStore), fiber.StatusOK, "2")
	apptest.ExpectBody(t, app, cacheTestAGetCC("/?id=3", "NO-STORE"), fiber.StatusOK, "3")
	// my-no-store is a directive of its own, not no-store.
	cacheTestAExpectBody(t, app, cacheTestAGetCC("/?id=4", "my-no-store"), cacheMiss, "4")
	cacheTestAExpectBody(t, app, "/?id=4", cacheHit, "4")
}

func Test_Cache_WithSeveralRequests(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 10 * time.Second}))
	app.Get("/:id", func(c fiber.Ctx) error { return c.SendString(c.Params("id")) })

	for range 10 {
		for id := range 10 {
			apptest.ExpectBody(t, app, fmt.Sprintf("/%d", id), fiber.StatusOK, strconv.Itoa(id))
		}
	}
}

func Test_Cache_Get(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Post("/", func(c fiber.Ctx) error { return c.SendString(fiber.Query[string](c, "cache")) })

	app.Get("/get", func(c fiber.Ctx) error { return c.SendString(fiber.Query[string](c, "cache")) })

	apptest.ExpectBody(t, app, apptest.Req(fiber.MethodPost, "/?cache=123"), fiber.StatusOK, "123")
	apptest.ExpectBody(t, app, apptest.Req(fiber.MethodPost, "/?cache=12345"), fiber.StatusOK, "12345")
	apptest.ExpectBody(t, app, "/get?cache=123", fiber.StatusOK, "123")
	apptest.ExpectBody(t, app, "/get?cache=12345", fiber.StatusOK, "12345")
}

func Test_Cache_Post(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	var count atomic.Int32
	app.Post("/", func(c fiber.Ctx) error {
		current := count.Add(1)
		return c.SendString(fmt.Sprintf("%d:%s", current, fiber.Query[string](c, "cache")))
	})

	cacheTestAExpectBody(t, app, apptest.Req(fiber.MethodPost, "/?cache=123"), cacheUnreachable, "1:123")
	cacheTestAExpectBody(t, app, apptest.Req(fiber.MethodPost, "/?cache=123"), cacheUnreachable, "2:123")
	cacheTestAExpectBody(t, app, apptest.Req(fiber.MethodPost, "/?cache=12345"), cacheUnreachable, "3:12345")
}

func Test_Cache_CustomMethods(t *testing.T) {
	t.Parallel()
	stored, notStored := [2]string{cacheMiss, cacheHit}, [2]string{cacheUnreachable, cacheUnreachable}
	for _, tc := range []struct {
		wantCache [2]string // X-Cache of the first and of the second request
		name      string
		method    string
		methods   []string
		wantCalls int32 // requests that reach the handler
	}{
		{
			name: "POST cached when in Methods", method: fiber.MethodPost, wantCache: stored, wantCalls: 1,
			methods: []string{fiber.MethodGet, fiber.MethodHead, fiber.MethodPost},
		},
		{
			// PUT not in Methods: always bypasses the cache
			name: "unconfigured method bypasses cache", method: fiber.MethodPut, methods: []string{fiber.MethodGet},
			wantCache: notStored, wantCalls: 2,
		},
		{
			// Even GET bypasses the cache when Methods is explicitly empty
			name: "empty Methods slice disables caching", method: fiber.MethodGet, methods: []string{},
			wantCache: notStored, wantCalls: 2,
		},
		{
			// "get" is normalized to "GET" and matches
			name: "lowercase method names are normalized", method: fiber.MethodGet, methods: []string{"get", "post"},
			wantCache: stored, wantCalls: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, calls := cacheTestAOrigin(New(Config{Methods: tc.methods}), "")

			// The origin answers with its call count, so a cached second answer repeats the first.
			cacheTestAExpectBody(t, app, apptest.Req(tc.method, "/"), tc.wantCache[0], "1")
			cacheTestAExpectBody(t, app, apptest.Req(tc.method, "/"), tc.wantCache[1], strconv.Itoa(int(tc.wantCalls)))
			require.Equal(t, tc.wantCalls, calls.Load(), "the handler must run once per request the cache lets through")
		})
	}
}

func Test_Cache_QueryMethod(t *testing.T) {
	t.Parallel()
	queryOnly := Config{Methods: []string{fiber.MethodQuery}}
	active, archived := `{"filter":"active"}`, `{"filter":"archived"}`
	longBody := strings.Repeat("x", maxKeyDimensionSegmentLength+1)
	longBodyHash := sha256.Sum256([]byte(longBody))
	// A short body spelled like the hash of the long one.
	hashLike := "sha256:" + hex.EncodeToString(longBodyHash[:])
	// step is a QUERY request for / carrying body, the X-Cache status of its answer and the call of the
	// origin that produced the answer.
	type step struct {
		body, cache string
		call        int
	}

	for _, tc := range []struct {
		name  string
		steps []step
		cfg   Config
	}{
		{name: "same body hits cache", cfg: queryOnly, steps: []step{{active, cacheMiss, 1}, {active, cacheHit, 1}}},
		{
			name: "different body produces different cache key", cfg: queryOnly,
			steps: []step{{active, cacheMiss, 1}, {archived, cacheMiss, 2}, {active, cacheHit, 1}},
		},
		{
			name: "body hash namespace does not collide with raw body", cfg: queryOnly,
			steps: []step{{longBody, cacheMiss, 1}, {hashLike, cacheMiss, 2}, {longBody, cacheHit, 1}},
		},
		{name: "empty body is cacheable", cfg: queryOnly, steps: []step{{"", cacheMiss, 1}, {"", cacheHit, 1}}},
		// The default Methods are GET and HEAD.
		{
			name: "not cached when QUERY not in Methods", cfg: ConfigDefault,
			steps: []step{{`{}`, cacheUnreachable, 1}, {`{}`, cacheUnreachable, 2}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, calls := cacheTestAOrigin(New(tc.cfg), "")
			lastCall := 0
			for _, s := range tc.steps {
				cacheTestAExpectBody(t, app, cacheTestQuery(s.body), s.cache, strconv.Itoa(s.call))
				lastCall = max(lastCall, s.call)
			}
			require.Equal(t, int32(lastCall), calls.Load(), "the origin answers every request the cache does not")
		})
	}
}

// cacheTestAAcceptLanguage returns a GET request for / that asks for language.
func cacheTestAAcceptLanguage(language string) *http.Request {
	return apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderAcceptLanguage, language))
}

func Test_Cache_DefaultKeyDimensions(t *testing.T) {
	t.Parallel()
	// echoApp returns an app that serves method / through mw and answers with the
	// number of requests it has served and the value echo takes from the request,
	// such as "2:en-US".
	echoApp := func(mw fiber.Handler, method string, echo func(c fiber.Ctx) string) *fiber.App {
		app := fiber.New()
		app.Use(mw)

		var count atomic.Int32
		app.Add([]string{method}, "/", func(c fiber.Ctx) error {
			return c.SendString(fmt.Sprintf("%d:%s", count.Add(1), echo(c)))
		})
		return app
	}
	acceptLanguage := func(c fiber.Ctx) string { return c.Get(fiber.HeaderAcceptLanguage) }

	t.Run("distinct query values do not collide", func(t *testing.T) {
		t.Parallel()
		app := echoApp(New(), fiber.MethodGet, func(c fiber.Ctx) string { return string(c.Request().URI().QueryString()) })

		apptest.ExpectBody(t, app, "/?id=1", fiber.StatusOK, "1:id=1")
		apptest.ExpectBody(t, app, "/?id=2", fiber.StatusOK, "2:id=2")
	})

	t.Run("representation headers partition cache", func(t *testing.T) {
		t.Parallel()
		app := echoApp(New(), fiber.MethodGet, acceptLanguage)

		apptest.ExpectBody(t, app, cacheTestAAcceptLanguage("en-US"), fiber.StatusOK, "1:en-US")
		apptest.ExpectBody(t, app, cacheTestAAcceptLanguage("fr-FR"), fiber.StatusOK, "2:fr-FR")
	})

	t.Run("keyed cookies preserve case-sensitive names", func(t *testing.T) {
		t.Parallel()
		app := echoApp(New(Config{KeyCookies: []string{"SessionID"}}), fiber.MethodGet,
			func(c fiber.Ctx) string { return c.Cookies("SessionID") })

		session := func(value string) *http.Request {
			req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
			req.AddCookie(&http.Cookie{Name: "SessionID", Value: value})
			return req
		}

		cacheTestAExpectBody(t, app, session("alpha"), cacheMiss, "1:alpha")
		cacheTestAExpectBody(t, app, session("beta"), cacheMiss, "2:beta")
		cacheTestAExpectBody(t, app, session("beta"), cacheHit, "2:beta")
	})

	t.Run("long paths are bounded in storage keys", func(t *testing.T) {
		t.Parallel()
		storage := storagetest.New()
		oversizedPath := "/" + strings.Repeat("a", maxKeyDimensionSegmentLength+1)
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour, Storage: storage}))
		app.Get(oversizedPath, func(c fiber.Ctx) error { return c.SendString("ok") })

		cacheTestAExpect(t, app, oversizedPath, cacheMiss)

		hash := sha256.Sum256([]byte(oversizedPath))
		expectedBoundedPath := "sha256:" + hex.EncodeToString(hash[:])
		require.Len(t, expectedBoundedPath, len("sha256:")+sha256.Size*2)

		expectedPrefix := cacheKeyVersion + "|" + fiber.MethodGet + "|scheme=http|host=example.com|" + expectedBoundedPath
		foundBoundedKey := false
		for key := range storage.Data {
			require.NotContains(t, key, oversizedPath)
			if strings.HasPrefix(key, expectedPrefix) {
				foundBoundedKey = true
			}
		}
		require.True(t, foundBoundedKey)
	})

	t.Run("empty keyed headers disable default header partitioning", func(t *testing.T) {
		t.Parallel()
		app := echoApp(New(Config{KeyHeaders: []string{}}), fiber.MethodGet, acceptLanguage)

		apptest.ExpectBody(t, app, cacheTestAAcceptLanguage("en-US"), fiber.StatusOK, "1:en-US")
		cacheTestAExpectBody(t, app, cacheTestAAcceptLanguage("fr-FR"), cacheHit, "1:en-US")
	})

	t.Run("non-keyed headers do not fragment cache", func(t *testing.T) {
		t.Parallel()
		app := echoApp(New(), fiber.MethodGet, func(c fiber.Ctx) string { return c.Get("X-Request-ID") })

		apptest.ExpectBody(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHeader("X-Request-ID", "a")), fiber.StatusOK, "1:a")
		apptest.ExpectBody(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHeader("X-Request-ID", "b")), fiber.StatusOK, "1:a")
	})

	t.Run("body-dependent requests are bypassed by default", func(t *testing.T) {
		t.Parallel()
		app := echoApp(New(), fiber.MethodPost, func(c fiber.Ctx) string { return string(c.Body()) })

		apptest.ExpectBody(t, app, httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader("a")), fiber.StatusOK, "1:a")
		apptest.ExpectBody(t, app, httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader("b")), fiber.StatusOK, "2:b")
	})
}

// cacheTestAExpectRepeat requests target twice and requires the second response
// to repeat the first one's body and to carry a Cache-Control header, as a
// cached answer does. It returns the body.
func cacheTestAExpectRepeat(t *testing.T, app *fiber.App, target string) []byte {
	t.Helper()

	body := apptest.BodyBytes(t, apptest.Do(t, app, target))

	respCached := apptest.Do(t, app, target)
	bodyCached := apptest.BodyBytes(t, respCached)
	require.True(t, bytes.Equal(body, bodyCached))
	require.NotEmpty(t, respCached.Header.Get(fiber.HeaderCacheControl))
	return body
}

func Test_Cache_CustomNext(t *testing.T) {
	t.Parallel()
	app, _ := cacheTestAOrigin(New(Config{
		Next: func(c fiber.Ctx) bool { return c.Response().StatusCode() != fiber.StatusOK },
	}), "")

	errorCount := 0
	app.Get("/error", func(c fiber.Ctx) error {
		errorCount++
		return c.Status(fiber.StatusInternalServerError).SendString(strconv.Itoa(errorCount))
	})

	cacheTestAExpectRepeat(t, app, "/")

	apptest.Do(t, app, "/error")

	errRespCached := apptest.Do(t, app, "/error")
	require.Empty(t, errRespCached.Header.Get(fiber.HeaderCacheControl))
}

func Test_CustomKey(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	var called bool
	app.Use(New(Config{KeyGenerator: func(c fiber.Ctx) string {
		called = true
		return utils.CopyString(c.Path())
	}}))

	app.Get("/", func(c fiber.Ctx) error { return c.SendString("hi") })

	apptest.Do(t, app, "/")
	require.True(t, called)
}

func Test_CustomExpiration(t *testing.T) {
	t.Parallel()
	var called bool
	var newCacheTime int
	app, _ := cacheTestAOrigin(New(Config{ExpirationGenerator: func(c fiber.Ctx, _ *Config) time.Duration {
		called = true
		var err error
		newCacheTime, err = strconv.Atoi(c.GetRespHeader("Cache-Time", "600"))
		require.NoError(t, err)
		return time.Second * time.Duration(newCacheTime)
	}}), "", "Cache-Time", "1")

	resp := apptest.Do(t, app, "/")
	require.True(t, called)
	require.Equal(t, 1, newCacheTime)

	// Wait until the cache expires (timestamp tick can delay expiry detection slightly).
	expireDeadline := time.Now().Add(3 * time.Second)
	var cachedResp *http.Response
	for {
		cachedResp = apptest.Do(t, app, "/")
		if cachedResp.Header.Get("X-Cache") != cacheHit {
			break
		}
		require.True(t, time.Now().Before(expireDeadline), "response remained cached beyond expected expiration")
		time.Sleep(50 * time.Millisecond)
	}

	body := apptest.BodyBytes(t, resp)
	cachedBody := apptest.BodyBytes(t, cachedResp)

	if bytes.Equal(body, cachedBody) {
		t.Errorf("Cache should have expired: %s, %s", body, cachedBody)
	}

	cachedRespNextRound := apptest.Do(t, app, "/")
	cachedBodyNextRound := apptest.BodyBytes(t, cachedRespNextRound)

	if !bytes.Equal(cachedBodyNextRound, cachedBody) {
		t.Errorf("Cache should not have expired: %s, %s", cachedBodyNextRound, cachedBody)
	}
}

func Test_AdditionalE2EResponseHeaders(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{StoreResponseHeaders: true}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Response().Header.Add("X-Foobar", "foobar")
		return c.SendString("hi")
	})

	resp := apptest.Do(t, app, "/")
	require.Equal(t, "foobar", resp.Header.Get("X-Foobar"))

	resp = apptest.Do(t, app, "/")
	require.Equal(t, "foobar", resp.Header.Get("X-Foobar"))
}

// Test_SetCookieResponseIsNotStored asserts that a response handing the client
// a cookie is not stored at all.
//
// The entry would be served to every client whose request matches its key, and
// a response that sets a cookie has personalized itself for the one client that
// caused the miss. Keeping Set-Cookie out of the stored headers is not enough —
// the body is the payload, so the second client below would read the first
// one's page.
//
// Nor do must-revalidate and proxy-revalidate buy a cookie-setting response a
// place in the store. RFC 9111 §3.5 lets must-revalidate carry a response to an
// authorized request, because a cache honoring it goes back to the origin once
// the entry is stale and the origin re-checks the credential. Neither half of
// that holds here: the directive says when a stale entry may be reused, not that
// the body is impersonal, and this middleware never revalidates — it serves the
// stored body for the whole configured expiration. Reusing the authorization
// test for the cookie gate therefore handed the first client's page to the second.
func Test_SetCookieResponseIsNotStored(t *testing.T) {
	t.Parallel()
	for _, directive := range []string{"none", "must-revalidate", "proxy-revalidate"} {
		t.Run(directive, func(t *testing.T) {
			t.Parallel()
			var calls int
			app := fiber.New()
			app.Use(New(Config{StoreResponseHeaders: true}))
			app.Get("/", func(c fiber.Ctx) error {
				calls++
				if directive != "none" {
					c.Set(fiber.HeaderCacheControl, directive)
				}
				c.Cookie(&fiber.Cookie{Name: "session", Value: fmt.Sprintf("secret-%d", calls)})
				return c.SendString(fmt.Sprintf("page-for-client-%d", calls))
			})

			resp := cacheTestAExpectBody(t, app, "/", cacheUnreachable, "page-for-client-1")
			require.Contains(t, resp.Header.Get("Set-Cookie"), "session=secret-1")

			resp = cacheTestAExpect(t, app, "/", cacheUnreachable)
			require.Contains(t, resp.Header.Get("Set-Cookie"), "session=secret-2")
			require.Equal(t, "page-for-client-2", apptest.Body(t, resp), "the second client must not read the first one's page")
			require.Equal(t, 2, calls, "the handler must run for both clients")
		})
	}
}

// Test_SetCookieResponseSharedCacheOptIn asserts the escape hatch still works:
// a response that says outright a shared cache may hold it is taken at its
// word, so the gate above is about the revalidate directives specifically and
// not a blanket refusal of every cookie-setting response.
func Test_SetCookieResponseSharedCacheOptIn(t *testing.T) {
	t.Parallel()
	for _, directive := range []string{"public", "s-maxage=60"} {
		t.Run(directive, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(Config{StoreResponseHeaders: true}))
			app.Get("/", func(c fiber.Ctx) error {
				c.Set(fiber.HeaderCacheControl, directive)
				c.Cookie(&fiber.Cookie{Name: "session", Value: "shared"})
				return c.SendString("hi")
			})

			resp := cacheTestAExpect(t, app, "/", cacheMiss)
			// Without this the test passes vacuously: a response carrying no
			// cookie is stored anyway, so the assertions below would not
			// distinguish "the opt-in beat the cookie gate" from "there was no
			// cookie to gate".
			require.Contains(t, resp.Header.Get("Set-Cookie"), "session=shared")

			resp = cacheTestAExpect(t, app, "/", cacheHit)
			require.Empty(t, resp.Header.Values("Set-Cookie"), "the stored entry must not replay the cookie")
		})
	}
}

// Test_SetCookieFromInnerMiddlewareIsSeen pins the ordering the cookie gate can
// actually enforce: a middleware that writes its cookie on the way out is seen
// when it sits inside the cache, because its post-Next work has already run by
// the time the store decision is made.
//
// The reverse order is a hazard the gate cannot see. With the cookie-writing
// middleware outside, its post-Next work runs after the cache has stored, so
// the entry is kept and the next client reads the first one's body. Nothing
// here can detect that — the response the cache inspects genuinely has no
// cookie on it yet — so it is documented as an ordering requirement in
// docs/middleware/cache.md rather than asserted as behavior.
func Test_SetCookieFromInnerMiddlewareIsSeen(t *testing.T) {
	t.Parallel()
	var calls int
	app := fiber.New()
	app.Use(New())
	app.Use(func(c fiber.Ctx) error {
		err := c.Next()
		c.Cookie(&fiber.Cookie{Name: "session", Value: fmt.Sprintf("secret-%d", calls)})
		return err
	})
	app.Get("/", func(c fiber.Ctx) error {
		calls++
		return c.SendString(fmt.Sprintf("page-for-client-%d", calls))
	})

	for i := 1; i <= 2; i++ {
		cacheTestAExpectBody(t, app, "/", cacheUnreachable, fmt.Sprintf("page-for-client-%d", i))
	}
	require.Equal(t, 2, calls, "the handler must run for both clients")
}

// Test_StoreResponseHeaders_DropsSetCookie covers the opt-in path: a route that
// says a shared cache may store the response is taken at its word, and then the
// stored copy still leaves Set-Cookie out so the entry cannot hand the first
// client's session to the rest.
func Test_StoreResponseHeaders_DropsSetCookie(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{StoreResponseHeaders: true}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "public, max-age=60")
		c.Cookie(&fiber.Cookie{Name: "session", Value: "first-client-secret"})
		c.Response().Header.Add("X-Foobar", "foobar")
		return c.SendString("hi")
	})

	resp := cacheTestAExpect(t, app, "/", cacheMiss)
	require.Contains(t, resp.Header.Get("Set-Cookie"), "session=first-client-secret")

	resp = cacheTestAExpect(t, app, "/", cacheHit)
	require.Empty(t, resp.Header.Values("Set-Cookie"))
	// Other stored headers still come back, so the assertion above is about
	// Set-Cookie specifically and not about header storage being off.
	require.Equal(t, "foobar", resp.Header.Get("X-Foobar"))
}

// Test_StoreResponseHeaders_KeepsRepeatedFieldLines asserts that a header sent
// on more than one field line comes back off a cache hit intact.
//
// The restore loop used to replay every stored entry with Set, which overwrites
// the first matching line and leaves the rest — so a name sent twice collapsed
// to its last value. Two of the headers below are why that matters: a Vary that
// loses "Cookie" lets a downstream shared cache serve one user's response to
// another, and a Content-Security-Policy that loses a line drops from the
// intersection of both policies (what a browser actually enforces) to whichever
// one is weaker on its own.
func Test_StoreResponseHeaders_KeepsRepeatedFieldLines(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{StoreResponseHeaders: true}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Response().Header.Add("Vary", "Cookie")
		c.Response().Header.Add("Vary", "Accept-Encoding")
		c.Response().Header.Add("Content-Security-Policy", "default-src 'none'")
		c.Response().Header.Add("Content-Security-Policy", "script-src 'self'")
		c.Response().Header.Add("X-Single", "only")
		return c.SendString("hi")
	})

	resp := cacheTestAExpect(t, app, "/", cacheMiss)
	require.Equal(t, []string{"Cookie", "Accept-Encoding"}, resp.Header.Values("Vary"))

	resp = cacheTestAExpect(t, app, "/", cacheHit)
	// Vary is carried by the entry itself rather than by the stored header
	// list, and comes back on one line with every name (RFC 9110 §5.3).
	require.Equal(t, []string{"Cookie,Accept-Encoding"}, resp.Header.Values("Vary"))
	require.Equal(t, []string{"default-src 'none'", "script-src 'self'"}, resp.Header.Values("Content-Security-Policy"))
	// A name sent once is still restored once, so the fix did not turn Set into
	// an unconditional Add.
	require.Equal(t, []string{"only"}, resp.Header.Values("X-Single"))
}

func Test_CacheHeader(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Expiration: 10 * time.Second,
		Next:       func(c fiber.Ctx) bool { return c.Response().StatusCode() != fiber.StatusOK },
	}))

	app.Get("/", func(c fiber.Ctx) error { return c.SendString("Hello, World!") })

	app.Post("/", func(c fiber.Ctx) error { return c.SendString(fiber.Query[string](c, "cache")) })

	count := 0
	app.Get("/error", func(c fiber.Ctx) error {
		count++
		c.Response().Header.Add("Cache-Time", "1")
		return c.Status(fiber.StatusInternalServerError).SendString(strconv.Itoa(count))
	})

	cacheTestAExpect(t, app, "/", cacheMiss)
	cacheTestAExpect(t, app, "/", cacheHit)
	cacheTestAExpect(t, app, apptest.Req(fiber.MethodPost, "/?cache=12345"), cacheUnreachable)
	cacheTestAExpect(t, app, "/error", cacheUnreachable)
}

func Test_Cache_WithHead(t *testing.T) {
	t.Parallel()
	app, _ := cacheTestAOrigin(New(), "", "Cache-Time", "1")

	resp := cacheTestAExpect(t, app, apptest.Req(fiber.MethodHead, "/"), cacheMiss)

	cachedResp := cacheTestAExpect(t, app, apptest.Req(fiber.MethodHead, "/"), cacheHit)

	body := apptest.BodyBytes(t, resp)
	cachedBody := apptest.BodyBytes(t, cachedResp)

	require.Equal(t, cachedBody, body)
}

func Test_Cache_WithHeadThenGet(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	handler := func(c fiber.Ctx) error { return c.SendString(fiber.Query[string](c, "cache")) }
	app.RouteChain("/").Get(handler).Head(handler)

	cacheTestAExpectBody(t, app, apptest.Req(fiber.MethodHead, "/?cache=123"), cacheMiss, "")
	cacheTestAExpectBody(t, app, apptest.Req(fiber.MethodHead, "/?cache=123"), cacheHit, "")
	cacheTestAExpectBody(t, app, "/?cache=123", cacheMiss, "123")
	cacheTestAExpectBody(t, app, "/?cache=123", cacheHit, "123")
}

func Test_CustomCacheHeader(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{CacheHeader: "Cache-Status"}))

	app.Get("/", func(c fiber.Ctx) error { return c.SendString("Hello, World!") })

	resp := apptest.Do(t, app, "/")
	require.Equal(t, cacheMiss, resp.Header.Get("Cache-Status"))
}

func Test_CacheInvalidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		maxBytes uint
	}{
		{name: "invalidate"},
		{name: "invalidate and remove from the heap", maxBytes: 10 * 1024 * 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, _ := cacheTestAOrigin(New(Config{
				CacheInvalidator: func(c fiber.Ctx) bool { return fiber.Query[bool](c, "invalidate") },
				MaxBytes:         tc.maxBytes,
			}), "")

			body := cacheTestAExpectRepeat(t, app, "/")

			respInvalidate := apptest.Do(t, app, "/?invalidate=true")
			bodyInvalidate := apptest.BodyBytes(t, respInvalidate)
			require.NotEqual(t, body, bodyInvalidate)
		})
	}
}

// The invalidator is not asked about an entry that does not exist.
func Test_CacheInvalidation_noCacheEntry(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	cacheInvalidatorExecuted := false
	app.Use(New(Config{
		CacheInvalidator: func(c fiber.Ctx) bool {
			cacheInvalidatorExecuted = true
			return fiber.Query[bool](c, "invalidate")
		},
		MaxBytes: 10 * 1024 * 1024,
	}))
	apptest.Do(t, app, "/?invalidate=true")
	require.False(t, cacheInvalidatorExecuted)
}

func Test_CacheInvalidator_PanicReleasesLock(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(recover.New())
	var panicked atomic.Bool
	app.Use(New(Config{
		CacheInvalidator: func(fiber.Ctx) bool {
			if panicked.CompareAndSwap(false, true) {
				panic("invalidator")
			}
			return false
		},
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	cfg := fiber.TestConfig{Timeout: 2 * time.Second, FailOnTimeout: true}
	resp := apptest.Do(t, app, "/", cfg)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	resp = apptest.Do(t, app, "/", cfg)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

	// A lock held across that panic hangs this request.
	resp = apptest.Do(t, app, "/", cfg)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_CacheInvalidation_IgnoresMaxStale(t *testing.T) {
	t.Parallel()
	for _, storage := range storageKinds {
		for _, maxStale := range []string{"max-stale", "max-stale=3600"} {
			t.Run(storage+"/"+maxStale, func(t *testing.T) {
				t.Parallel()
				app, calls := cacheTestAOrigin(New(Config{
					MaxBytes:         1 << 20,
					Storage:          storageOf(storage),
					CacheInvalidator: func(c fiber.Ctx) bool { return c.Get("X-Session-Ended") == "1" },
				}), "response ")

				cacheTestCRequireReply(t, getRoot(t, app, nil), cacheMiss, "response 1")
				cacheTestCRequireReply(t, getRoot(t, app, nil), cacheHit, "response 1")

				got := getRoot(t, app, map[string]string{"X-Session-Ended": "1", fiber.HeaderCacheControl: maxStale})
				require.Equal(t, "response 2", got.body, "an invalidated entry was served to a request that accepts stale ones")
				require.NotEqual(t, cacheHit, got.cache)
				require.Equal(t, int32(2), calls.Load())

				cacheTestCRequireReply(t, getRoot(t, app, nil), cacheHit, "response 2")
			})
		}
	}
}

// CacheInvalidator must not run under the middleware's lock: a slow call would stall every request, a panic would leave it held.
func Test_CacheInvalidation_RunsWithoutTheLock(t *testing.T) {
	t.Parallel()
	for _, storage := range storageKinds {
		t.Run("a panic does not wedge the cache/"+storage, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(recover.New())
			app.Use(New(Config{
				Storage: storageOf(storage),
				CacheInvalidator: func(c fiber.Ctx) bool {
					if c.Get("X-Panic") == "1" {
						panic("session store unreachable")
					}
					return false
				},
			}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

			require.Equal(t, cacheMiss, getRoot(t, app, nil).cache)
			require.Equal(t, fiber.StatusInternalServerError, getRoot(t, app, map[string]string{"X-Panic": "1"}).status)
			// The next request must not wait on a lock left held, and the panicking one's entry is back in the pool.
			cacheTestCRequireReply(t, getRoot(t, app, nil), cacheHit, "ok")
		})
	}

	t.Run("a slow one does not stall other requests", func(t *testing.T) {
		t.Parallel()
		entered := make(chan struct{})
		release := make(chan struct{})
		var releaseOnce sync.Once
		releaseSlow := func() { releaseOnce.Do(func() { close(release) }) }
		t.Cleanup(releaseSlow)

		app := fiber.New()
		app.Use(New(Config{
			CacheInvalidator: func(c fiber.Ctx) bool {
				if c.Get("X-Slow") == "1" {
					close(entered)
					<-release
				}
				return false
			},
		}))
		app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })
		require.Equal(t, cacheMiss, getRoot(t, app, nil).cache)

		slowErr := make(chan error, 1)
		go func() {
			req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
			req.Header.Set("X-Slow", "1")
			resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
			if err == nil {
				err = resp.Body.Close()
			}
			slowErr <- err
		}()
		<-entered

		// Served while the lookup is still running.
		require.Equal(t, cacheHit, getRoot(t, app, nil).cache)

		releaseSlow()
		require.NoError(t, <-slowErr)
	})

	// Requests overlap in the invalidator while others store, expire and evict entries; nothing may race or unbalance the accounting.
	for _, storage := range storageKinds {
		t.Run("overlapping with stores, expiry and eviction/"+storage, func(t *testing.T) {
			t.Parallel()
			// The limit exceeds what every request in flight can reserve at once, and is below the number of paths.
			const (
				workers  = 8
				requests = 120
				paths    = 24
				maxBytes = 120
			)
			payload := strings.Repeat("x", 12)

			probe := &accountingProbe{}
			clock := cacheTestCClock()
			var invalidations atomic.Int64
			cfg := Config{
				Storage:    storageOf(storage),
				MaxBytes:   maxBytes,
				Expiration: 2 * time.Second,
				clock:      clock.Now,
				accounting: probe.record,
				// App code that overlaps with everything else: it yields, waits and ends entries.
				CacheInvalidator: func(fiber.Ctx) bool {
					switch invalidations.Add(1) % 5 {
					case 0:
						return true
					case 1:
						time.Sleep(200 * time.Microsecond)
					case 2:
						runtime.Gosched()
					}
					return false
				},
			}
			app := fiber.New()
			app.Use(New(cfg))
			app.Get("/:name", func(c fiber.Ctx) error { return c.SendString(payload) })

			request := func(name, cacheControl string) error {
				req := apptest.Req(fiber.MethodGet, "/"+name,
					apptest.WithHeaderIf(fiber.HeaderCacheControl, cacheControl))
				resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
				if err != nil {
					return err
				}
				// conventions:ignore body-read: the close error is handled together with the read error
				body, err := io.ReadAll(resp.Body)
				if closeErr := resp.Body.Close(); err == nil {
					err = closeErr
				}
				if err != nil {
					return err
				}
				if resp.StatusCode != fiber.StatusOK || string(body) != payload {
					return fmt.Errorf("/%s answered %d %q", name, resp.StatusCode, body)
				}
				return nil
			}

			// Entries run out of time while the requests are in flight.
			stop := make(chan struct{})
			var ticker sync.WaitGroup
			ticker.Go(func() {
				for {
					select {
					case <-stop:
						return
					default:
						clock.Add(300 * time.Millisecond)
						runtime.Gosched()
					}
				}
			})

			var wg sync.WaitGroup
			for worker := range workers {
				wg.Go(func() {
					rng := rand.New(rand.NewPCG(uint64(worker), 7))
					for range requests {
						name := "p" + strconv.Itoa(rng.IntN(paths))
						cacheControl := ""
						switch rng.IntN(6) {
						case 0:
							cacheControl = "max-age=0"
						case 1:
							cacheControl = "no-cache"
						}
						if err := request(name, cacheControl); err != nil {
							t.Errorf("worker %d: %v", worker, err)
							return
						}
					}
				})
			}
			wg.Wait()
			close(stop)
			ticker.Wait()

			require.NoError(t, request("p0", ""))
			require.LessOrEqual(t, probe.counted(), uint(maxBytes), "the accounting went out of balance")
		})
	}
}

func Test_CacheStorage_CustomHeaders(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Storage: memory.New(), MaxBytes: 10 * 1024 * 1024}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Response().Header.Set("Content-Type", "text/xml")
		c.Response().Header.Set("Content-Encoding", "utf8")
		return c.Send([]byte("<xml><value>Test</value></xml>"))
	})

	cacheTestAExpectRepeat(t, app, "/")
}

// Because time points are updated once every X milliseconds, entries in tests can often have
// equal expiration times and thus be in a random order. This closure hands out increasing
// time intervals to maintain strong ascending order of expiration
func stableAscendingExpiration() func(c1 fiber.Ctx, c2 *Config) time.Duration {
	i := 0
	return func(_ fiber.Ctx, _ *Config) time.Duration {
		i++
		return time.Hour * time.Duration(i)
	}
}

// accountingProbe records the MaxBytes bookkeeping after each request.
type accountingProbe struct {
	mu          sync.Mutex
	storedBytes uint
	heapLen     int
}

func (p *accountingProbe) record(storedBytes uint, heapLen int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.storedBytes, p.heapLen = storedBytes, heapLen
}

func (p *accountingProbe) counted() uint {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.storedBytes
}

func (p *accountingProbe) nodes() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.heapLen
}

// charged is what MaxBytes counts for an entry: its key, its body and the headers it stores.
func charged(key string, bodyLen int, stored ...string) uint {
	n := len(key) + bodyLen
	for _, s := range stored {
		n += len(s)
	}
	return uint(n)
}

// The default key of a request reads alike around the part under test: the scheme, the host, the path and the query,
// then the three default key headers when the request sends none of them.
const (
	keyStart     = "scheme=http|host=example.com|/|q="
	keyNoHeaders = "|h=accept:0|accept-encoding:0|accept-language:0"
)

// defaultKey is the key the default config stores a GET of path on example.com under.
func defaultKey(path string) string {
	return cacheKeyVersion + "|GET|scheme=http|host=example.com|" + path + "|q=" + keyNoHeaders
}

// variantKey is the key an entry stored under the Vary names gets for a request that sends none
// of them; one that sends them gets a key of the same length.
func variantKey(base string, names ...string) string {
	return varyKey(base, names, &fasthttp.RequestHeader{}, true)
}

// textPlain is the Content-Type of a SendString answer, which an entry's charge counts.
const textPlain = fiber.MIMETextPlainCharsetUTF8

func Test_Cache_NoCacheRefresh_KeepsAccounting(t *testing.T) {
	t.Parallel()
	for _, header := range []string{fiber.HeaderCacheControl, fiber.HeaderPragma} {
		t.Run(header, func(t *testing.T) {
			t.Parallel()
			probe := &accountingProbe{}
			app := fiber.New()
			app.Use(New(Config{MaxBytes: 1500, Expiration: time.Hour, accounting: probe.record}))
			app.Get("/*", func(c fiber.Ctx) error { return c.Send(make([]byte, 600)) })

			cacheTestAExpect(t, app, "/x", cacheMiss)
			cacheTestAExpect(t, app, apptest.Req(fiber.MethodGet, "/x", apptest.WithHeader(header, noCache)), cacheMiss)

			require.Equal(t, charged(defaultKey("/x"), 600, textPlain), probe.counted(), "the refreshed entry must be counted once")
			require.Equal(t, 1, probe.nodes(), "the refreshed entry must own a single heap node")

			// 600 + 600 fits within 1500, so /y must not evict anything.
			cacheTestAExpect(t, app, "/y", cacheMiss)
			cacheTestAExpect(t, app, "/x", cacheHit)
			cacheTestAExpect(t, app, "/y", cacheHit)

			require.Equal(t, charged(defaultKey("/x"), 600, textPlain)+charged(defaultKey("/y"), 600, textPlain), probe.counted())
			require.Equal(t, 2, probe.nodes())
		})
	}
}

func Test_Cache_FailedRefresh(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		failedKey string // the key whose storage write fails
		keepsOld  bool   // whether the entry the refresh superseded stays counted and tracked
	}{
		{
			// Refresh the entry, failing the replacement's body store: the entry it
			// superseded stays in the backend, so it must stay counted and tracked
			// rather than leaking its bytes and its expiry.
			name:      "failed body store keeps the old entry tracked",
			failedKey: bodyKey(defaultKey("/")),
			keepsOld:  true,
		},
		{
			// Refresh the entry, letting the replacement body land but failing its
			// metadata: the cleanup deletes that body, so the entry the refresh
			// superseded can no longer be served and must not keep its accounting.
			name:      "failed metadata store after the body write drops the old entry",
			failedKey: defaultKey("/"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			storage := storagetest.New()
			probe := &accountingProbe{}
			app := fiber.New()
			app.Use(New(Config{Storage: storage, MaxBytes: 1500, Expiration: time.Hour, accounting: probe.record}))
			app.Get("/", func(c fiber.Ctx) error { return c.Send(make([]byte, 600)) })

			apptest.Expect(t, app, "/", fiber.StatusOK)
			counted, nodes := probe.counted(), probe.nodes()
			require.NotZero(t, counted)
			require.Equal(t, 1, nodes)

			storage.SetErr(storagetest.OpSet, tc.failedKey, errors.New("boom"))

			apptest.Expect(t, app, cacheTestAGetCC("/", noCache), fiber.StatusInternalServerError)

			if !tc.keepsOld {
				counted, nodes = 0, 0
			}
			require.Equal(t, counted, probe.counted())
			require.Equal(t, nodes, probe.nodes())
		})
	}
}

func Test_Cache_UncacheableResponseDoesNotEvict(t *testing.T) {
	t.Parallel()
	probe := &accountingProbe{}
	app := fiber.New()
	// Room for /b alone, which only fits if /a is evicted.
	maxBytes := charged(defaultKey("/b"), len("bbbbbbbbbb"), textPlain, "max-age=0")
	app.Use(New(Config{MaxBytes: maxBytes, Expiration: time.Hour, accounting: probe.record}))
	app.Get("/a", func(c fiber.Ctx) error { return c.SendString("aaaaa") })
	// max-age=0 is never stored, and the body only fits if /a is evicted.
	app.Get("/b", cacheTestAServe("max-age=0", "bbbbbbbbbb"))

	cacheTestAExpect(t, app, "/a", cacheMiss)
	cacheTestAExpect(t, app, "/b", cacheUnreachable)

	require.Equal(t, charged(defaultKey("/a"), len("aaaaa"), textPlain), probe.counted(), "an uncacheable response must not displace stored entries")
	require.Equal(t, 1, probe.nodes())

	cacheTestAExpect(t, app, "/a", cacheHit)
}

// Body read a sized stream to its end and kept a read error as the body, which was then cached as a 200.
func Test_Cache_SizedStreamIsStoredOnlyWhole(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		stream  func() io.Reader
		maxRead int
	}{
		"fails": {
			maxRead: 4,
			stream: func() io.Reader {
				return io.MultiReader(strings.NewReader("part"), iotest.ErrReader(errors.New("upstream reset")))
			},
		},
		"longer than declared": {
			maxRead: 64 << 10, // short of the whole MiB
			stream:  func() io.Reader { return strings.NewReader(strings.Repeat("x", 1<<20)) },
		},
	} {
		// etag reads the stream as well, inside the cache or over the one the cache sends on.
		for _, chain := range []string{"cache", "cache, etag", "etag, cache"} {
			t.Run(name+"/"+chain, func(t *testing.T) {
				t.Parallel()
				app := fiber.New()
				for mw := range strings.SplitSeq(chain, ", ") {
					if mw == "etag" {
						app.Use(etag.New())
					} else {
						app.Use(New(Config{Expiration: time.Hour}))
					}
				}

				var calls int
				var read strings.Builder
				app.Get("/", func(c fiber.Ctx) error {
					calls++
					read.Reset()
					return c.SendStream(io.TeeReader(tc.stream(), &read), 10)
				})

				for range 2 {
					fctx := cacheTestServe(app.Handler(), "/")

					require.Equal(t, cacheUnreachable, cacheTestXCache(fctx))
					require.True(t, fctx.Response.IsBodyStream())
					require.Equal(t, 10, fctx.Response.Header.ContentLength())
					require.Empty(t, fctx.Response.Header.Peek(fiber.HeaderETag))
					require.LessOrEqual(t, read.Len(), tc.maxRead)
				}
				require.Equal(t, 2, calls)
			})
		}
	}
}

// A one-byte body under a URL-long key or Location holds kilobytes, so those count too.
func Test_Cache_MaxBytes_CountsKeyAndHeaders(t *testing.T) {
	t.Parallel()
	const (
		maxBytes = 64 * 1024
		urlLen   = 3000
		urls     = 200
	)

	for name, cfg := range map[string]Config{
		"key":      {KeyGenerator: func(c fiber.Ctx) string { return c.OriginalURL() }},
		"Location": {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			probe := &accountingProbe{}
			cfg.MaxBytes, cfg.Expiration, cfg.accounting = maxBytes, time.Hour, probe.record
			app := fiber.New()
			app.Use(New(cfg))
			app.Get("/*", func(c fiber.Ctx) error {
				if name == "Location" {
					return c.Redirect().Status(fiber.StatusMovedPermanently).To("/login?next=" + c.OriginalURL())
				}
				return c.SendString("x")
			})
			h := app.Handler()

			for i := range urls {
				fctx := cacheTestServe(h, fmt.Sprintf("/%d/%s", i, strings.Repeat("a", urlLen)))
				require.Equal(t, cacheMiss, cacheTestXCache(fctx))
			}
			require.LessOrEqual(t, probe.counted(), uint(maxBytes))
			require.LessOrEqual(t, probe.nodes(), maxBytes/urlLen)
		})
	}
}

func Test_Cache_MaxBytesEviction(t *testing.T) {
	t.Parallel()
	// Room for 7 body bytes in three entries, each also charged its key and Content-Type.
	sizesMaxBytes := 7 + 3*charged(defaultKey("/1"), 0, textPlain)
	for _, tc := range []struct {
		name     string
		steps    [][2]string // the path of each request, sent one after the other, and the X-Cache status of its answer
		maxBytes uint
		sized    bool // the answer has as many bytes as the last segment of the path says, else one
	}{
		{
			name: "order", maxBytes: 2 * charged(defaultKey("/a"), 1, textPlain),
			steps: [][2]string{
				// Insert a, b into a cache with room for two entries
				{"/a", cacheMiss},
				{"/b", cacheMiss},
				{"/a", cacheHit},
				{"/b", cacheHit},
				// Add c -> a evicted
				{"/c", cacheMiss},
				{"/b", cacheHit},
				// Add a again -> b evicted
				{"/a", cacheMiss},
				{"/c", cacheHit},
				// Add b -> c evicted
				{"/b", cacheMiss},
				{"/c", cacheMiss},
			},
		},
		{
			name: "sizes", maxBytes: sizesMaxBytes, sized: true,
			steps: [][2]string{
				{"/1", cacheMiss},
				{"/2", cacheMiss},
				{"/3", cacheMiss},
				{"/4", cacheMiss}, // 1+2+3+4 > 7 => 1,2 are evicted now
				{"/3", cacheHit},
				{"/1", cacheMiss},
				{"/2", cacheMiss},
				{"/" + strconv.Itoa(int(sizesMaxBytes)), cacheUnreachable}, // too big to cache -> unreachable
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(Config{MaxBytes: tc.maxBytes, ExpirationGenerator: stableAscendingExpiration()}))
			app.Get("/*", func(c fiber.Ctx) error {
				if !tc.sized {
					return c.SendString("1")
				}
				size, err := strconv.Atoi(string(c.RequestCtx().URI().LastPathSegment()))
				require.NoError(t, err)
				return c.Send(make([]byte, size))
			})

			for idx, step := range tc.steps {
				rsp := apptest.Do(t, app, step[0])
				require.Equal(t, step[1], rsp.Header.Get("X-Cache"), "Case %v", idx)
			}
		})
	}
}

func Test_Cache_UncacheableStatusCodes(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	app.Get("/:statusCode", func(c fiber.Ctx) error {
		statusCode, err := strconv.Atoi(c.Params("statusCode"))
		require.NoError(t, err)
		return c.Status(statusCode).SendString("foo")
	})

	uncacheableStatusCodes := []int{
		// Informational responses
		fiber.StatusContinue, fiber.StatusSwitchingProtocols, fiber.StatusProcessing, fiber.StatusEarlyHints,
		// Successful responses; 206 is never stored: this cache does not handle ranges (RFC 9111 §3.3)
		fiber.StatusPartialContent, fiber.StatusCreated, fiber.StatusAccepted, fiber.StatusResetContent,
		fiber.StatusMultiStatus, fiber.StatusAlreadyReported, fiber.StatusIMUsed,
		// Redirection responses
		fiber.StatusFound, fiber.StatusSeeOther, fiber.StatusNotModified, fiber.StatusUseProxy, fiber.StatusSwitchProxy,
		fiber.StatusTemporaryRedirect,
		// Client error responses
		fiber.StatusBadRequest, fiber.StatusUnauthorized, fiber.StatusPaymentRequired, fiber.StatusForbidden,
		fiber.StatusNotAcceptable, fiber.StatusProxyAuthRequired, fiber.StatusRequestTimeout, fiber.StatusConflict,
		fiber.StatusLengthRequired, fiber.StatusPreconditionFailed, fiber.StatusRequestEntityTooLarge,
		fiber.StatusUnsupportedMediaType, fiber.StatusRequestedRangeNotSatisfiable, fiber.StatusExpectationFailed,
		fiber.StatusMisdirectedRequest, fiber.StatusUnprocessableEntity, fiber.StatusLocked,
		fiber.StatusFailedDependency, fiber.StatusTooEarly, fiber.StatusUpgradeRequired,
		fiber.StatusPreconditionRequired, fiber.StatusTooManyRequests, fiber.StatusRequestHeaderFieldsTooLarge,
		fiber.StatusTeapot, fiber.StatusUnavailableForLegalReasons,
		// Server error responses
		fiber.StatusInternalServerError, fiber.StatusBadGateway, fiber.StatusServiceUnavailable,
		fiber.StatusGatewayTimeout, fiber.StatusHTTPVersionNotSupported, fiber.StatusVariantAlsoNegotiates,
		fiber.StatusInsufficientStorage, fiber.StatusLoopDetected, fiber.StatusNotExtended,
		fiber.StatusNetworkAuthenticationRequired,
	}
	for _, v := range uncacheableStatusCodes {
		resp := cacheTestAExpect(t, app, fmt.Sprintf("/%d", v), cacheUnreachable)
		require.Equal(t, v, resp.StatusCode)
	}
}

func TestCacheAgeHeader(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 10 * time.Second}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	resp := apptest.Do(t, app, "/")
	require.Equal(t, "0", resp.Header.Get(fiber.HeaderAge))

	time.Sleep(4 * time.Second)

	resp = cacheTestAExpect(t, app, "/", cacheHit)
	age, err := strconv.Atoi(resp.Header.Get(fiber.HeaderAge))
	require.NoError(t, err)
	require.Positive(t, age)
}

func TestCacheUpstreamAge(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 3 * time.Second}))
	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderAge, "5")
		return c.SendString("hi")
	})

	resp := apptest.Do(t, app, "/")
	require.Equal(t, "5", resp.Header.Get(fiber.HeaderAge))

	time.Sleep(1500 * time.Millisecond)

	resp = cacheTestAExpect(t, app, "/", cacheUnreachable)
	require.Equal(t, "5", resp.Header.Get(fiber.HeaderAge))
}

func Test_Cache_PartialContentNotStored(t *testing.T) {
	t.Parallel()
	const full = "0123456789"
	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		if c.Get(fiber.HeaderRange) == "bytes=0-4" {
			c.Set(fiber.HeaderContentRange, "bytes 0-4/10")
			return c.Status(fiber.StatusPartialContent).SendString(full[:5])
		}
		return c.SendString(full)
	})

	resp := cacheTestAExpectStatus(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderRange, "bytes=0-4")),
		cacheUnreachable, fiber.StatusPartialContent)
	require.Equal(t, full[:5], apptest.Body(t, resp))

	resp = cacheTestAExpectStatus(t, app, "/", cacheMiss, fiber.StatusOK)
	require.Empty(t, resp.Header.Get(fiber.HeaderContentRange))
	require.Equal(t, full, apptest.Body(t, resp))
}

// Test_CacheStoreAndServe sends the requests of a scenario for / one after the
// other to an origin that answers with the scenario's response headers and its
// call count, and checks which of them the cache answers.
func Test_CacheStoreAndServe(t *testing.T) {
	t.Parallel()
	// step is a request: a header it adds to those of the scenario, the X-Cache status of the answer, and
	// the call of the origin that produced the answer.
	type step struct {
		name, value, cache string
		call               int
	}
	stored := []step{{cache: cacheMiss, call: 1}, {cache: cacheHit, call: 1}}
	notStored := []step{{cache: cacheUnreachable, call: 1}, {cache: cacheUnreachable, call: 2}}
	bearer := []string{fiber.HeaderAuthorization, "Bearer token"}
	cc := func(directives string) []string { return []string{fiber.HeaderCacheControl, directives} }
	keyedBy := func(suffix string) func(fiber.Ctx) string {
		return func(c fiber.Ctx) string { return c.Path() + suffix }
	}
	expiresIn := func(d time.Duration, layout string) []string {
		return []string{fiber.HeaderExpires, time.Now().Add(d).UTC().Format(layout)}
	}
	auth10s := Config{Expiration: 10 * time.Second}
	varyStar := []string{fiber.HeaderVary, "*"}

	for _, tc := range []struct {
		name   string
		prefix string   // of the origin's answers
		origin []string // the origin's response headers, as name and value pairs
		req    []string // the request headers of every step, as name and value pairs
		steps  []step
		cfg    Config
	}{
		{name: "private", cfg: ConfigDefault, origin: cc("private"), steps: notStored},
		{name: "no-store", cfg: ConfigDefault, origin: cc("no-store"), steps: notStored},
		{name: "no-cache", cfg: ConfigDefault, origin: cc("no-cache"), steps: notStored},
		{name: "Vary: *", cfg: Config{KeyGenerator: keyedBy("|vary-star")}, origin: varyStar, steps: notStored},
		{
			name: "Vary: *, DisableVaryHeaders", origin: varyStar, steps: notStored,
			cfg: Config{DisableVaryHeaders: true, KeyGenerator: keyedBy("|vary-star")},
		},
		{
			name: "Expires in the past", cfg: ConfigDefault, prefix: "expires",
			origin: expiresIn(-time.Minute, http.TimeFormat), steps: notStored,
		},
		{
			name: "Expires in the future", cfg: Config{StoreResponseHeaders: true}, prefix: "expires",
			origin: expiresIn(30*time.Second, http.TimeFormat), steps: stored,
		},
		{
			// Pins the RFC 9110 §5.6.7 acceptance set: a future Expires in the obsolete
			// RFC 850 format is a valid HTTP-date and must enable caching rather than
			// hit the parse-error (force-revalidate) path, consistently with how the
			// Date header is parsed.
			name: "Expires in the future, obsolete RFC 850 format", cfg: Config{StoreResponseHeaders: true},
			prefix: "expires", origin: expiresIn(30*time.Second, "Monday, 02-Jan-06 15:04:05 GMT"), steps: stored,
		},
		{
			name: "an upstream Age exhausting max-age", cfg: Config{KeyGenerator: keyedBy("|age-exhausted")},
			origin: []string{fiber.HeaderCacheControl, "public, max-age=2", fiber.HeaderAge, "2"}, steps: notStored,
		},
		{
			name:   "request max-age=0 revalidates",
			cfg:    Config{Expiration: 30 * time.Second, KeyGenerator: keyedBy("|req-max-age-zero")},
			origin: cc("public, max-age=30"),
			steps: []step{
				{cache: cacheMiss, call: 1},
				{fiber.HeaderCacheControl, "max-age=0", cacheMiss, 2},
				{cache: cacheHit, call: 2},
			},
		},
		{
			name: "request Pragma: no-cache revalidates", cfg: Config{Expiration: time.Minute}, prefix: "body",
			origin: cc("public, max-age=60"),
			steps: []step{
				{cache: cacheMiss, call: 1},
				{cache: cacheHit, call: 1},
				{fiber.HeaderPragma, "no-cache", cacheMiss, 2},
				{cache: cacheHit, call: 2},
			},
		},
		{
			name: "Authorization bypasses the entry of an anonymous request", cfg: ConfigDefault,
			steps: []step{
				{cache: cacheMiss, call: 1},
				{fiber.HeaderAuthorization, "Bearer token", cacheUnreachable, 2},
				{cache: cacheHit, call: 1},
			},
		},
		// No answer to a request carrying Authorization is stored when the response says private, nor by
		// default, which skips such requests.
		{name: "Authorization, private", cfg: ConfigDefault, origin: cc("private"), req: bearer, steps: notStored},
		{name: "Authorization, skipped by default", cfg: ConfigDefault, req: bearer, steps: notStored},
		// The answer to a request carrying Authorization is stored only when the response carries a
		// directive that lets a shared cache keep it.
		{
			name: "Authorization, public", cfg: auth10s, prefix: "ok-",
			origin: cc("public, max-age=60"), req: bearer, steps: stored,
		},
		{
			name: "Authorization, must-revalidate", cfg: auth10s, prefix: "ok-",
			origin: cc("must-revalidate, max-age=60"), req: bearer, steps: stored,
		},
		{
			name: "Authorization, proxy-revalidate", cfg: auth10s, prefix: "ok-",
			origin: cc("proxy-revalidate, max-age=60"), req: bearer, steps: stored,
		},
		{
			name: "Authorization, Expires header", cfg: auth10s, prefix: "ok-",
			origin: append(cc(""), expiresIn(time.Minute, http.TimeFormat)...), req: bearer, steps: notStored,
		},
		{
			name:   "Authorization, must-revalidate, custom key",
			cfg:    Config{Expiration: 30 * time.Second, KeyGenerator: keyedBy("|must-revalidate-auth")},
			prefix: "auth", origin: cc("must-revalidate, max-age=60"), req: bearer, steps: stored,
		},
		{
			name:   "Authorization, proxy-revalidate, custom key",
			cfg:    Config{Expiration: 30 * time.Second, KeyGenerator: keyedBy("|proxy-revalidate-auth")},
			prefix: "proxy", origin: cc("proxy-revalidate, max-age=60"), req: bearer, steps: stored,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, calls := cacheTestAOrigin(New(tc.cfg), tc.prefix, tc.origin...)
			lastCall := 0
			for i, s := range tc.steps {
				got := cacheTestCGet(t, app, "/", append(slices.Clone(tc.req), s.name, s.value)...)
				cacheTestCRequireReply(t, got, s.cache, tc.prefix+strconv.Itoa(s.call), "request %d", i+1)
				lastCall = max(lastCall, s.call)
			}
			require.Equal(t, int32(lastCall), calls.Load(), "the origin answers every request the cache does not")
		})
	}
}

func Test_CacheInvalidExpiresStoredAsStale(t *testing.T) {
	t.Parallel()
	storage := storagetest.New()

	app, _ := cacheTestAOrigin(New(Config{
		Expiration:   30 * time.Second,
		KeyGenerator: func(c fiber.Ctx) string { return c.Path() + "|invalid-expires" },
		Storage:      storage,
	}), "body", fiber.HeaderCacheControl, "public", fiber.HeaderExpires, "invalid-date")

	expectedKey := cacheKeyVersion + `|GET|/\pinvalid-expires`

	for i := 1; i <= 3; i++ {
		cacheTestAExpectBody(t, app, "/", cacheMiss, "body"+strconv.Itoa(i))
		require.Contains(t, storage.Data, expectedKey)
		require.Contains(t, storage.Data, bodyKey(expectedKey))
	}
}

func Test_CacheStoreDoesNotAgeItsOwnResponse(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		cacheControl string
		expiresIn    time.Duration // the Expires header is set this far past the clock, unless zero
	}{
		// A response fiber just generated has age 0, so a one-second lifetime has to
		// survive the store phase no matter how much time passes inside it.
		{name: "max-age", cacheControl: "max-age=1"},
		// An Expires lifetime must be measured from the instant the Date header is
		// anchored to, or clock reads inside the store phase consume it.
		{name: "Expires anchored to Date", expiresIn: 2 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clock := clocktest.NewClock(time.Now().Truncate(time.Second))
			clock.Step = time.Second
			app := fiber.New()
			app.Use(New(Config{clock: clock.Now}))

			app.Get("/", func(c fiber.Ctx) error {
				if tc.cacheControl != "" {
					c.Set(fiber.HeaderCacheControl, tc.cacheControl)
				}
				if tc.expiresIn != 0 {
					c.Set(fiber.HeaderExpires, clock.Now().Add(tc.expiresIn).UTC().Format(http.TimeFormat))
				}
				return c.SendString("cached")
			})

			cacheTestAExpect(t, app, "/", cacheMiss)
		})
	}
}

func Test_CacheSMaxAgeOverridesMaxAge(t *testing.T) {
	t.Parallel()
	// step moves the clock on by advance, then requires the X-Cache status and the body of the answer.
	type step struct {
		cache, body string
		advance     time.Duration
	}
	for _, tc := range []struct {
		name, cacheControl string
		steps              []step
	}{
		{
			// Past the 1s s-maxage window; max-age=10 is longer, so the shorter
			// s-maxage must win and the entry must be treated as stale.
			name: "when shorter", cacheControl: "public, max-age=10, s-maxage=1",
			steps: []step{{cacheMiss, "1", 0}, {cacheHit, "1", 0}, {cacheMiss, "2", 1100 * time.Millisecond}},
		},
		{
			// Past max-age=1 but within the longer s-maxage=2 window: the longer
			// s-maxage must win, so the entry is still fresh. Past 2s it is stale.
			name: "when longer", cacheControl: "public, max-age=1, s-maxage=2",
			steps: []step{
				{cacheMiss, "1", 0},
				{cacheHit, "1", 1200 * time.Millisecond},
				{cacheMiss, "2", 1700 * time.Millisecond},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Drive freshness from a manually advanced clock so the checks never straddle
			// a whole-second boundary (the previous time.Sleep based version flaked under
			// -race -count -shuffle).
			clock := clocktest.NewClock(time.Now().Truncate(time.Second))
			app, _ := cacheTestAOrigin(New(Config{clock: clock.Now}), "", fiber.HeaderCacheControl, tc.cacheControl)
			for _, s := range tc.steps {
				clock.Add(s.advance)
				cacheTestAExpectBody(t, app, "/", s.cache, s.body)
			}
		})
	}
}

func Test_CacheOnlyIfCachedMiss(t *testing.T) {
	t.Parallel()
	app, calls := cacheTestAOrigin(New(), "")

	cacheTestAExpectStatus(t, app, cacheTestAGetCC("/", "only-if-cached"), cacheUnreachable, fiber.StatusGatewayTimeout)
	require.Zero(t, calls.Load())
}

func Test_CacheOnlyIfCachedStaleNotServed(t *testing.T) {
	t.Parallel()
	app, calls := cacheTestAOrigin(New(), "", fiber.HeaderCacheControl, "public, max-age=1")

	cacheTestAExpect(t, app, "/", cacheMiss)

	time.Sleep(1500 * time.Millisecond)

	cacheTestAExpectStatus(t, app, cacheTestAGetCC("/", "only-if-cached"), cacheUnreachable, fiber.StatusGatewayTimeout)
	require.Equal(t, int32(1), calls.Load())
}

// Test_CacheOnlyIfCachedWithNoCache covers a request that asks for a stored
// response and refuses to have it served unvalidated. RFC 9111 Section 5.2.1.7
// has a cache that cannot satisfy only-if-cached without contacting the origin
// answer 504, and validating a stored response is contacting the origin. The
// fresh entry used to fall through to the handler instead.
func Test_CacheOnlyIfCachedWithNoCache(t *testing.T) {
	t.Parallel()
	requests := []struct {
		header map[string]string
		name   string
	}{
		{name: "no-cache beside only-if-cached", header: map[string]string{fiber.HeaderCacheControl: "no-cache, only-if-cached"}},
		{name: "only-if-cached before no-cache", header: map[string]string{fiber.HeaderCacheControl: "only-if-cached, no-cache"}},
		{name: "Pragma no-cache", header: map[string]string{fiber.HeaderCacheControl: "only-if-cached", fiber.HeaderPragma: "no-cache"}},
		{name: "max-age=0 behaves the same way", header: map[string]string{fiber.HeaderCacheControl: "max-age=0, only-if-cached"}},
	}

	for _, storageName := range []string{"in-memory", "storage"} {
		for _, tc := range requests {
			t.Run(storageName+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				cfg := Config{}
				if storageName == "storage" {
					cfg.Storage = memory.New()
				}

				app, calls := cacheTestAOrigin(New(cfg), "origin-", fiber.HeaderCacheControl, "public, max-age=60")

				// A fresh entry, stored by an ordinary request.
				cacheTestAExpect(t, app, "/", cacheMiss)
				require.Equal(t, int32(1), calls.Load())

				req := apptest.Req(fiber.MethodGet, "/")
				for k, v := range tc.header {
					req.Header.Set(k, v)
				}
				cacheTestAExpectStatus(t, app, req, cacheUnreachable, fiber.StatusGatewayTimeout)
				require.Equal(t, int32(1), calls.Load(), "only-if-cached must not reach the handler")

				// The refusal leaves the stored response alone.
				resp := cacheTestAExpectStatus(t, app, "/", cacheHit, fiber.StatusOK)
				require.Equal(t, "origin-1", apptest.Body(t, resp))
				require.Equal(t, int32(1), calls.Load())
			})
		}
	}
}

func Test_CacheMaxStaleServesStaleResponse(t *testing.T) {
	t.Parallel()
	app, calls := cacheTestAOrigin(New(), "", fiber.HeaderCacheControl, "public, max-age=2")

	cacheTestAExpect(t, app, "/", cacheMiss)
	cacheTestAExpect(t, app, "/", cacheHit)

	time.Sleep(2500 * time.Millisecond)

	resp := apptest.Do(t, app, cacheTestAGetCC("/", "max-stale=5"))
	require.Equalf(t, cacheHit, resp.Header.Get("X-Cache"), "dirs=%+v Age=%s count=%d", parseRequestCacheControl([]byte("max-stale=5")), resp.Header.Get(fiber.HeaderAge), calls.Load())
	require.Equal(t, "1", apptest.Body(t, resp))
	require.Equal(t, int32(1), calls.Load())
}

func Test_CacheMaxStaleRespectsMustRevalidate(t *testing.T) {
	t.Parallel()
	app, calls := cacheTestAOrigin(New(), "", fiber.HeaderCacheControl, "public, max-age=1, must-revalidate")

	cacheTestAExpect(t, app, "/", cacheMiss)

	time.Sleep(1500 * time.Millisecond)

	cacheTestAExpectBody(t, app, cacheTestAGetCC("/", "max-stale=30"), cacheMiss, "2")
	require.Equal(t, int32(2), calls.Load())
}

func Test_CacheMaxStaleRespectsProxyRevalidateSharedAuth(t *testing.T) {
	t.Parallel()
	// s-maxage=2, not 1: the store phase reads cfg.now() twice and charges the
	// whole second in between as apparent age, so a one-second lifetime can be
	// consumed entirely and the entry reported unreachable instead of cached.
	app, calls := cacheTestAOrigin(New(), "", fiber.HeaderCacheControl, "s-maxage=2, proxy-revalidate")

	cacheTestAExpect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderAuthorization, "Bearer abc")), cacheMiss)

	time.Sleep(2500 * time.Millisecond)

	req := apptest.Req(fiber.MethodGet, "/",
		apptest.WithHeader(fiber.HeaderAuthorization, "Bearer abc"),
		apptest.WithHeader(fiber.HeaderCacheControl, "max-stale=30"))

	cacheTestAExpectBody(t, app, req, cacheMiss, "2")
	require.Equal(t, int32(2), calls.Load())
}

func Test_CachePreservesCacheControlHeaders(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	expires := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)
	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "public, max-age=5, immutable")
		c.Set(fiber.HeaderExpires, expires)
		c.Set(fiber.HeaderETag, `W/"abc"`)
		return c.SendString("ok")
	})

	for _, want := range []string{cacheMiss, cacheHit} {
		resp := cacheTestAExpect(t, app, "/", want)
		require.Equal(t, "public, max-age=5, immutable", resp.Header.Get(fiber.HeaderCacheControl))
		require.Equal(t, expires, resp.Header.Get(fiber.HeaderExpires))
		require.Equal(t, `W/"abc"`, resp.Header.Get(fiber.HeaderETag))
	}
}

func setResponseDate(date time.Time) fiber.Handler {
	return func(c fiber.Ctx) error {
		if err := c.Next(); err != nil {
			return err
		}
		c.Response().Header.Set(fiber.HeaderDate, date.UTC().Format(http.TimeFormat))
		return nil
	}
}

func Test_CacheDateAndAgeHandling(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		cacheControl     string
		cacheHeader      string
		dateOffset       time.Duration
		expiration       time.Duration
		expectAgeAtLeast int
		expectCount      int
		originAge        int
	}{
		{
			name: "age derived from past date without Age header", dateOffset: -1 * time.Minute,
			expiration: 5 * time.Minute, cacheControl: "public, max-age=120", cacheHeader: cacheHit,
			expectAgeAtLeast: 1, expectCount: 1,
		},
		{
			name: "stale due to past date despite max-age", dateOffset: -90 * time.Second, expiration: 5 * time.Minute,
			cacheControl: "public, max-age=30", cacheHeader: cacheUnreachable, expectCount: 2, originAge: 90,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(Config{Expiration: tc.expiration}))
			app.Use(setResponseDate(time.Now().Add(tc.dateOffset).UTC()))

			var count int
			app.Get("/", func(c fiber.Ctx) error {
				count++
				if tc.originAge > 0 {
					c.Response().Header.Set(fiber.HeaderAge, strconv.Itoa(tc.originAge))
				}
				c.Set(fiber.HeaderCacheControl, tc.cacheControl)
				return c.SendString(strconv.Itoa(count))
			})

			apptest.Do(t, app, "/")

			if tc.cacheHeader == cacheHit {
				time.Sleep(2 * time.Second)
			}

			resp := apptest.Do(t, app, "/")
			require.Equal(t, tc.cacheHeader, resp.Header.Get("X-Cache"))
			if tc.cacheHeader == cacheHit {
				ageVal, err := strconv.Atoi(resp.Header.Get(fiber.HeaderAge))
				require.NoError(t, err)
				require.GreaterOrEqual(t, ageVal, tc.expectAgeAtLeast)
				require.Equal(t, 1, count)
			} else {
				require.Equal(t, strconv.Itoa(tc.expectCount), apptest.Body(t, resp))
				require.Equal(t, tc.expectCount, count)
			}
		})
	}
}

// cacheTestAMutatedHit stores a response for expiration behind a storage that
// lets mutate rewrite the metadata of the entry on its way in, so a test can
// plant values the cache never writes itself, and returns the cache hit that
// serves it. The origin sets the response headers given as name and value pairs.
func cacheTestAMutatedHit(t *testing.T, expiration time.Duration, mutate func(it *item), headers ...string) *http.Response {
	t.Helper()

	storage := newMutatingStorage(func(key string, val []byte) []byte {
		if strings.HasPrefix(key, bodyKeyPrefix) {
			return val
		}

		var it item
		if _, err := it.UnmarshalMsg(val); err != nil {
			return val
		}

		mutate(&it)
		updated, err := it.MarshalMsg(nil)
		if err != nil {
			return val
		}

		return updated
	})
	app, _ := cacheTestAOrigin(New(Config{Expiration: expiration, Storage: storage}), "", headers...)

	cacheTestAExpect(t, app, "/", cacheMiss)
	return cacheTestAExpect(t, app, "/", cacheHit)
}

func Test_CacheClampsInvalidStoredDate(t *testing.T) {
	t.Parallel()
	resp := cacheTestAMutatedHit(t, time.Minute, func(it *item) { it.date = uint64(math.MaxInt64) + 1024 },
		fiber.HeaderCacheControl, "public, max-age=60")

	parsedDate, err := http.ParseTime(resp.Header.Get(fiber.HeaderDate))
	require.NoError(t, err)
	require.WithinDuration(t, time.Now(), parsedDate, time.Minute)

	ageVal, err := strconv.Atoi(resp.Header.Get(fiber.HeaderAge))
	require.NoError(t, err)
	require.Less(t, ageVal, 60)
	require.GreaterOrEqual(t, ageVal, 0)
}

func Test_CacheClampsFutureStoredDate(t *testing.T) {
	t.Parallel()
	resp := cacheTestAMutatedHit(t, time.Minute, func(it *item) {
		future := time.Now().Add(2 * time.Second).UTC()
		it.date = uint64(max(future.Unix(), 0))
	}, fiber.HeaderCacheControl, "public, max-age=60")

	parsedDate, err := http.ParseTime(resp.Header.Get(fiber.HeaderDate))
	require.NoError(t, err)
	require.False(t, parsedDate.After(time.Now()))

	ageVal, err := strconv.Atoi(resp.Header.Get(fiber.HeaderAge))
	require.NoError(t, err)
	require.GreaterOrEqual(t, ageVal, 0)
}

func Test_CacheAgeClockStepsBackwards(t *testing.T) {
	t.Parallel()
	clock := clocktest.NewClock(time.Now().Truncate(time.Second))
	app := fiber.New()
	app.Use(New(Config{clock: clock.Now}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("cached") })

	cacheTestAExpect(t, app, "/", cacheMiss)

	clock.Add(-time.Hour)

	resp := cacheTestAExpect(t, app, "/", cacheHit)
	require.Equal(t, "0", resp.Header.Get(fiber.HeaderAge))
	require.Empty(t, resp.Header.Values(fiber.HeaderWarning))
}

func Test_CacheStaleResponseAddsWarning110(t *testing.T) {
	t.Parallel()
	app, _ := cacheTestAOrigin(New(Config{
		Expiration: 2 * time.Second,
	}), "body", fiber.HeaderCacheControl, "public, max-age=1")

	cacheTestAExpect(t, app, "/", cacheMiss)

	req := cacheTestAGetCC("/", "max-stale=5")

	// Wait for the cached response to become stale (max-age=1)
	// Add extra time to ensure the entry has expired
	time.Sleep(1200 * time.Millisecond)

	var resp *http.Response
	deadline := time.Now().Add(3 * time.Second)
	for {
		resp = apptest.Do(t, app, req)
		if resp.Header.Get("X-Cache") == cacheHit {
			ageVal, err := strconv.Atoi(resp.Header.Get(fiber.HeaderAge))
			require.NoError(t, err)
			if ageVal >= 1 {
				warnings := resp.Header.Values(fiber.HeaderWarning)
				if len(warnings) > 0 {
					break
				}
			}
		}
		require.True(t, time.Now().Before(deadline), "response did not become stale before deadline")
		time.Sleep(50 * time.Millisecond)
	}

	warnings := resp.Header.Values(fiber.HeaderWarning)
	require.NotEmpty(t, warnings, "Warning header should be present when serving stale response")
	require.Contains(t, strings.Join(warnings, "\n"), "110", "warning 110 not found in %v", warnings)
}

func Test_CacheHeuristicFreshnessAddsWarning113(t *testing.T) {
	t.Parallel()
	app, _ := cacheTestAOrigin(New(Config{
		Expiration: 2 * time.Second,
	}), "body", fiber.HeaderCacheControl, "public, max-age=60")

	cacheTestAExpect(t, app, "/", cacheMiss)
	resp := cacheTestAExpect(t, app, "/", cacheHit)

	for _, w := range resp.Header.Values(fiber.HeaderWarning) {
		require.NotContains(t, w, "113", "warning 113 should not be present for explicitly fresh responses")
	}
}

func Test_CacheHeuristicFreshnessAddsWarning113AfterThreshold(t *testing.T) {
	t.Parallel()
	resp := cacheTestAMutatedHit(t, 2*time.Second, func(it *item) {
		oldDate := time.Now().Add(-25 * time.Hour).UTC()
		it.date = uint64(max(oldDate.Unix(), 0))

		future := time.Now().Add(48 * time.Hour).UTC()
		it.exp = uint64(max(future.Unix(), 0))
		it.ttl = uint64((48 * time.Hour) / time.Second)
	})

	warnings := resp.Header.Values(fiber.HeaderWarning)
	require.NotEmpty(t, warnings)
	require.Contains(t, strings.Join(warnings, "\n"), "113", "warning 113 not found in %v", warnings)
}

func Test_CacheAgeHeaderIsCappedAtMaxDeltaSeconds(t *testing.T) {
	t.Parallel()
	const veryLargeAge = uint64(math.MaxInt32) + 1000
	resp := cacheTestAMutatedHit(t, time.Minute, func(it *item) { it.age = veryLargeAge },
		fiber.HeaderCacheControl, "public, max-age=60")

	ageVal, err := strconv.Atoi(resp.Header.Get(fiber.HeaderAge))
	require.NoError(t, err)
	require.Equal(t, math.MaxInt32, ageVal)
}

func Test_CacheMinFreshForcesRevalidation(t *testing.T) {
	t.Parallel()
	app, calls := cacheTestAOrigin(New(), "", fiber.HeaderCacheControl, "public, max-age=5")

	cacheTestAExpectBody(t, app, "/", cacheMiss, "1")

	resp := apptest.Do(t, app, cacheTestAGetCC("/", "min-fresh=10"))
	require.Equalf(t, cacheMiss, resp.Header.Get("X-Cache"), "dirs=%+v Age=%s count=%d", parseRequestCacheControl([]byte("min-fresh=10")), resp.Header.Get(fiber.HeaderAge), calls.Load())
	require.Equal(t, "2", apptest.Body(t, resp))

	cacheTestAExpectBody(t, app, "/", cacheHit, "2")
}

func Test_CachePermanentRedirectCached(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Expiration:           30 * time.Second,
		StoreResponseHeaders: true,
		KeyGenerator:         func(c fiber.Ctx) string { return c.Path() + "|status-308" },
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "public, max-age=30")
		c.Set(fiber.HeaderLocation, "/dest")
		return c.Status(fiber.StatusPermanentRedirect).SendString("redir" + strconv.Itoa(count))
	})

	for _, want := range []string{cacheMiss, cacheHit} {
		resp := cacheTestAExpectStatus(t, app, "/", want, fiber.StatusPermanentRedirect)
		require.Equal(t, "redir1", apptest.Body(t, resp))
		require.Equal(t, "/dest", resp.Header.Get(fiber.HeaderLocation))
	}
}

func Test_CacheNoCacheDirectiveOverridesExistingEntry(t *testing.T) {
	t.Parallel()
	var noCacheMode atomic.Bool
	app := fiber.New()
	app.Use(New())
	app.Get("/", cacheTestBFlipping(&noCacheMode, "public, max-age=60", "no-cache"))

	cacheTestAExpectBody(t, app, "/", cacheMiss, "stale")
	noCacheMode.Store(true)
	cacheTestAExpectBody(t, app, cacheTestAGetCC("/", "no-cache"), cacheUnreachable, "newer")
	cacheTestAExpectBody(t, app, "/", cacheUnreachable, "newer")
}

func Test_CachePrivateDirectiveInvalidatesExistingEntry(t *testing.T) {
	t.Parallel()
	var privateMode atomic.Bool
	app := fiber.New()
	app.Use(New())
	app.Get("/", cacheTestBFlipping(&privateMode, "public, max-age=60", "private"))

	cacheTestAExpectBody(t, app, "/", cacheMiss, "stale")
	privateMode.Store(true)
	cacheTestAExpectBody(t, app, cacheTestAGetCC("/", "no-cache"), cacheUnreachable, "newer")
	privateMode.Store(false)
	cacheTestAExpectBody(t, app, "/", cacheMiss, "stale")
}

// An entry whose upstream Age leaves it a second of its max-age expires after that second.
func Test_CacheRespectsUpstreamAgeForFreshness(t *testing.T) {
	t.Parallel()
	// Manually advanced like the s-maxage tests: with one second of lifetime
	// left, a tick of the real clock would expire the entry before the hit.
	clock := clocktest.NewClock(time.Now().Truncate(time.Second))
	app, _ := cacheTestAOrigin(New(Config{
		clock:        clock.Now,
		KeyGenerator: func(c fiber.Ctx) string { return c.Path() + "|age-remaining" },
	}), "", fiber.HeaderCacheControl, "public, max-age=2", fiber.HeaderAge, "1")

	cacheTestAExpectBody(t, app, "/", cacheMiss, "1")
	cacheTestAExpectBody(t, app, "/", cacheHit, "1")
	clock.Add(1500 * time.Millisecond)
	cacheTestAExpectBody(t, app, "/", cacheMiss, "2")
}

func Test_CacheVarySeparatesVariants(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{KeyGenerator: func(c fiber.Ctx) string { return c.Path() + "|vary-separated" }}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderVary, fiber.HeaderAcceptLanguage)
		return c.SendString(c.Get(fiber.HeaderAcceptLanguage) + strconv.Itoa(count))
	})

	cacheTestAExpectBody(t, app, cacheTestAAcceptLanguage("en"), cacheMiss, "en1")
	cacheTestAExpectBody(t, app, cacheTestAAcceptLanguage("fr"), cacheMiss, "fr2")
	cacheTestAExpectBody(t, app, cacheTestAAcceptLanguage("en"), cacheHit, "en1")
	cacheTestAExpectBody(t, app, cacheTestAAcceptLanguage("fr"), cacheHit, "fr2")
}

// cacheTestBPathKeyedApp returns an app running the cache middleware with cfg,
// except that entries live an hour and are keyed by the request path alone.
func cacheTestBPathKeyedApp(cfg *Config) *fiber.App {
	keyed := *cfg
	keyed.Expiration = time.Hour
	keyed.KeyGenerator = pathKey

	app := fiber.New()
	app.Use(New(keyed))
	return app
}

// cacheTestBInvalidateByQuery is a CacheInvalidator that fires on ?invalidate=true.
func cacheTestBInvalidateByQuery(c fiber.Ctx) bool {
	return fiber.Query[bool](c, "invalidate")
}

func Test_CacheStorage_UncacheableRevalidationDeletesStaleEntry(t *testing.T) {
	t.Parallel()
	for _, directive := range []string{"private", "no-cache", "no-store", "vary-star"} {
		t.Run(directive, func(t *testing.T) {
			t.Parallel()
			storage := storagetest.New()
			var returnUncacheable atomic.Bool
			key := cacheKeyVersion + "|GET|/cached"
			bodySlot := bodyKey(key)

			app := cacheTestBPathKeyedApp(&Config{
				CacheInvalidator: cacheTestBInvalidateByQuery,
				// Room for one entry, so storing /next evicts whatever is still tracked.
				MaxBytes: charged(key, len("stale"), textPlain, "public, max-age=60, must-revalidate"),
				Storage:  storage,
			})
			app.Get("/cached", func(c fiber.Ctx) error {
				if !returnUncacheable.Load() {
					c.Set(fiber.HeaderCacheControl, "public, max-age=60, must-revalidate")
					return c.SendString("stale")
				}
				if directive == "vary-star" {
					c.Set(fiber.HeaderVary, "*")
				} else {
					c.Set(fiber.HeaderCacheControl, directive)
				}
				return c.SendString("newer")
			})
			app.Get("/next", cacheTestAServe("public, max-age=60", "fresh"))

			cacheTestAExpect(t, app, "/cached", cacheMiss)
			require.True(t, storage.Holds(key), key)
			require.True(t, storage.Holds(bodySlot), bodySlot)

			returnUncacheable.Store(true)
			cacheTestBExpectNewer(t, app, "/cached?invalidate=true")
			require.False(t, storage.Holds(key), key)
			require.False(t, storage.Holds(bodySlot), bodySlot)

			require.Equal(t, []string{key, bodySlot}, storage.CallKeys(storagetest.OpDelete))
			fetched := storage.CallKeys(storagetest.OpGet)
			require.NotContains(t, fetched, bodySlot, "revalidation must not fetch the stale body")

			// If heap accounting retained the deleted entry, caching /next would
			// try to evict it and encounter these sentinel errors.
			storage.SetErr(storagetest.OpDelete, key, errors.New("stale metadata remained in heap"))
			storage.SetErr(storagetest.OpDelete, bodySlot, errors.New("stale body remained in heap"))

			cacheTestAExpect(t, app, "/next", cacheMiss)
			cacheTestAExpect(t, app, "/next", cacheHit)

			resp := apptest.Do(t, app, "/cached")
			require.NotEqual(t, cacheHit, resp.Header.Get("X-Cache"))
			require.Equal(t, "newer", apptest.Body(t, resp))
		})
	}
}

// cacheTestBFlipping returns a handler that answers "stale", cacheable under
// staleCC and with the staleExtra header name and value pairs, until flipped is
// set, and "newer" under newerCC from then on.
func cacheTestBFlipping(flipped *atomic.Bool, staleCC, newerCC string, staleExtra ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if flipped.Load() {
			c.Set(fiber.HeaderCacheControl, newerCC)
			return c.SendString("newer")
		}

		c.Set(fiber.HeaderCacheControl, staleCC)
		for name, value := range apptest.Pairs(staleExtra) {
			c.Set(name, value)
		}
		return c.SendString("stale")
	}
}

// cacheTestBExpectNewer sends target through app and requires the "newer" a
// flipped origin answers with, uncached and with status 200.
func cacheTestBExpectNewer[T apptest.Target](t *testing.T, app *fiber.App, target T) {
	t.Helper()

	resp := cacheTestAExpectStatus(t, app, target, cacheUnreachable, fiber.StatusOK)
	require.Equal(t, "newer", apptest.Body(t, resp))
}

func Test_CacheStorage_UncacheableRevalidationDeletionErrors(t *testing.T) {
	t.Parallel()
	key := cacheKeyVersion + "|GET|/cached"
	bodySlot := bodyKey(key)
	for _, tt := range []struct {
		name              string
		failedKey         string // the key whose deletion fails
		wantDeletes       []string
		metadataRemaining bool
	}{
		{name: "metadata", failedKey: key, metadataRemaining: true, wantDeletes: []string{key}},
		{name: "body", failedKey: bodySlot, wantDeletes: []string{key, bodySlot}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			storage := storagetest.New()
			var returnNoCache atomic.Bool

			app := cacheTestBPathKeyedApp(&Config{
				CacheInvalidator: cacheTestBInvalidateByQuery,
				// Room for one entry, so storing /next evicts whatever is still tracked.
				MaxBytes: charged(key, len("stale"), textPlain, "public, max-age=60, must-revalidate"),
				Storage:  storage,
			})
			app.Get("/cached", cacheTestBFlipping(&returnNoCache, "public, max-age=60, must-revalidate", "no-cache"))
			app.Get("/next", cacheTestAServe("public, max-age=60", "fresh"))

			cacheTestAExpect(t, app, "/cached", cacheMiss)

			storage.SetErr(storagetest.OpDelete, tt.failedKey, errors.New("delete failed"))
			returnNoCache.Store(true)

			cacheTestBExpectNewer(t, app, "/cached?invalidate=true")
			require.Equal(t, tt.metadataRemaining, storage.Holds(key))
			require.True(t, storage.Holds(bodySlot))

			require.Equal(t, tt.wantDeletes, storage.CallKeys(storagetest.OpDelete))
			fetched := storage.CallKeys(storagetest.OpGet)
			require.NotContains(t, fetched, bodySlot, "revalidation must not fetch the stale body")

			// The old heap entry remains tracked after a deletion error so a later
			// eviction can retry cleanup and restore byte accounting.
			storage.SetErr(storagetest.OpDelete, tt.failedKey, nil)
			cacheTestAExpect(t, app, "/next", cacheMiss)
			require.False(t, storage.Holds(key), key)
			require.False(t, storage.Holds(bodySlot), bodySlot)
		})
	}
}

// cacheTestBRequireAccounting requires the MaxBytes bookkeeping the probe saw
// last: the bytes counted as stored and the nodes in the eviction heap.
func cacheTestBRequireAccounting(t *testing.T, probe *accountingProbe, counted uint, nodes int) {
	t.Helper()

	require.Equal(t, counted, probe.counted(), "bytes counted as stored")
	require.Equal(t, nodes, probe.nodes(), "nodes in the eviction heap")
}

func Test_CacheStorage_UncacheableRevalidationPrecheckFailuresReturnOrigin(t *testing.T) {
	t.Parallel()
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprintf("metadataMissing=%t", missing), func(t *testing.T) {
			t.Parallel()
			storage := storagetest.New()
			probe := &accountingProbe{}
			key := cacheKeyVersion + "|GET|/cached"
			var revalidate atomic.Bool
			stale := charged(key, len("stale"), textPlain, "public, max-age=60")
			app := cacheTestBPathKeyedApp(&Config{MaxBytes: stale, Storage: storage, accounting: probe.record})
			app.Get("/cached", func(c fiber.Ctx) error {
				if revalidate.Load() {
					if missing {
						storage.Mu.Lock()
						delete(storage.Data, key)
						delete(storage.Data, bodyKey(key))
						storage.Mu.Unlock()
					} else {
						storage.SetErr(storagetest.OpGet, key, errors.New("metadata lookup failed"))
					}
					c.Set(fiber.HeaderCacheControl, "no-cache")
					return c.SendString("newer")
				}
				c.Set(fiber.HeaderCacheControl, "public, max-age=60")
				return c.SendString("stale")
			})

			cacheTestAExpect(t, app, "/cached", cacheMiss)

			revalidate.Store(true)
			cacheTestBExpectNewer(t, app, cacheTestAGetCC("/cached", "max-age=0"))
			if missing {
				cacheTestBRequireAccounting(t, probe, 0, 0)
			} else {
				cacheTestBRequireAccounting(t, probe, stale, 1)
			}
		})
	}
}

func Test_CacheStorage_EvictionRestoreDropsAbsentOrReplacedEntry(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"missing", "replaced"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			storage := &restoreMutationStorage{
				Memory:   storagetest.New(),
				key:      cacheKeyVersion + "|GET|/old",
				mutation: mutation,
			}
			probe := &accountingProbe{}
			stale := charged(storage.key, len("stale"), textPlain)
			app := cacheTestBPathKeyedApp(&Config{MaxBytes: stale, Storage: storage, accounting: probe.record})
			app.Get("/:name", func(c fiber.Ctx) error {
				if c.Path() == "/old" {
					return c.SendString("stale")
				}
				return c.SendString("fresh")
			})

			cacheTestAExpect(t, app, "/old", cacheMiss)
			cacheTestBRequireAccounting(t, probe, stale, 1)

			storage.armed.Store(true)
			apptest.Expect(t, app, "/new", fiber.StatusInternalServerError)
			cacheTestBRequireAccounting(t, probe, 0, 0)
		})
	}
}

// cacheTestBParked is a request sent in the background that stops on a hook of
// the test, in the origin or in the storage, until the test lets it go on.
type cacheTestBParked struct {
	resp    *http.Response
	err     error
	release chan struct{}
	done    chan struct{}
	once    sync.Once
}

// cacheTestBPark sends req through app in the background and returns once
// parked is closed, which the request does as it stops on release. finish lets
// it go on, and so does the end of the test should an assertion fail before.
func cacheTestBPark(t *testing.T, app *fiber.App, req *http.Request, parked <-chan struct{}, release chan struct{}) *cacheTestBParked {
	t.Helper()

	p := &cacheTestBParked{release: release, done: make(chan struct{})}
	t.Cleanup(p.open)
	go func() {
		defer close(p.done)
		p.resp, p.err = app.Test(req)
	}()
	<-parked
	return p
}

// open lets the parked request go on.
func (p *cacheTestBParked) open() {
	p.once.Do(func() { close(p.release) })
}

// finish lets the parked request go on, waits for it and returns its response,
// the body already read and closed.
func (p *cacheTestBParked) finish(t *testing.T) *http.Response {
	t.Helper()

	p.open()
	<-p.done
	require.NoError(t, p.err)
	apptest.BodyBytes(t, p.resp)
	return p.resp
}

func Test_CacheStorage_EvictionRestoreDropsOldNodeAfterConcurrentReplacement(t *testing.T) {
	t.Parallel()
	storage := &blockingFailedEvictionStorage{
		Memory:         storagetest.New(),
		key:            cacheKeyVersion + "|GET|/old",
		started:        make(chan struct{}),
		continueDelete: make(chan struct{}),
	}
	probe := &accountingProbe{}
	fresh := charged(cacheKeyVersion+"|GET|/new", len("012345678"), textPlain, "public, max-age=60")
	replacement := charged(storage.key, len("x"), textPlain, "public, max-age=60")
	app := cacheTestBPathKeyedApp(&Config{MaxBytes: fresh + replacement, Storage: storage, accounting: probe.record})
	app.Get("/:name", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "public, max-age=60")
		// The replacement fits beside the reservation of /new only while /old is popped.
		if c.Path() == "/new" {
			return c.SendString("012345678")
		}
		if fiber.Query[string](c, "replacement") == "true" {
			return c.SendString("x")
		}
		return c.SendString("stale")
	})

	cacheTestAExpect(t, app, "/old", cacheMiss)

	storage.armed.Store(true)
	evicting := cacheTestBPark(t, app, apptest.Req(fiber.MethodGet, "/new"), storage.started, storage.continueDelete)

	apptest.Expect(t, app, cacheTestAGetCC("/old?replacement=true", "max-age=0"), fiber.StatusOK)

	oldResp := evicting.finish(t)
	require.Equal(t, fiber.StatusInternalServerError, oldResp.StatusCode)
	cacheTestBRequireAccounting(t, probe, replacement, 1)
	require.True(t, storage.Holds(storage.key), "the replacement is stored")
}

func Test_CacheStorage_MinFreshUncacheableRevalidationCleansEntry(t *testing.T) {
	t.Parallel()
	storage := storagetest.New()
	probe := &accountingProbe{}
	var uncacheable atomic.Bool
	maxBytes := charged(cacheKeyVersion+"|GET|/cached", len("stale"), textPlain, "public, max-age=5")
	app := cacheTestBPathKeyedApp(&Config{MaxBytes: maxBytes, Storage: storage, accounting: probe.record})
	app.Get("/cached", cacheTestBFlipping(&uncacheable, "public, max-age=5", "no-store"))

	cacheTestAExpect(t, app, "/cached", cacheMiss)

	uncacheable.Store(true)
	cacheTestBExpectNewer(t, app, cacheTestAGetCC("/cached", "min-fresh=10"))
	cacheTestBRequireAccounting(t, probe, 0, 0)
	require.False(t, storage.Holds(cacheKeyVersion+"|GET|/cached"), "the metadata is gone")
}

func Test_Cache_InMemoryUncacheableRevalidationRemovesStaleEntry(t *testing.T) {
	t.Parallel()
	stale := charged(cacheKeyVersion+"|GET|/cached", len("stale"), textPlain, "public, max-age=60, must-revalidate")
	probe := &accountingProbe{}
	var uncacheable atomic.Bool
	app := cacheTestBPathKeyedApp(&Config{
		CacheInvalidator: cacheTestBInvalidateByQuery,
		MaxBytes:         2 * stale,
		accounting:       probe.record,
	})
	app.Get("/cached", cacheTestBFlipping(&uncacheable, "public, max-age=60, must-revalidate", "no-cache"))

	cacheTestAExpect(t, app, "/cached", cacheMiss)
	cacheTestBRequireAccounting(t, probe, stale, 1)

	uncacheable.Store(true)
	cacheTestBExpectNewer(t, app, "/cached?invalidate=true")
	cacheTestBRequireAccounting(t, probe, 0, 0)
	cacheTestBExpectNewer(t, app, "/cached")
}

func Test_Cache_DefaultMemoryRevalidationDoesNotReplayUncacheableEntry(t *testing.T) {
	t.Parallel()
	var uncacheable atomic.Bool
	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour}))
	app.Get("/cached", cacheTestBFlipping(&uncacheable, "public, max-age=60", "no-cache"))

	cacheTestAExpect(t, app, "/cached", cacheMiss)

	uncacheable.Store(true)
	cacheTestAExpect(t, app, cacheTestAGetCC("/cached", "max-age=0"), cacheUnreachable)

	resp := apptest.Do(t, app, "/cached")
	require.NotEqual(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Equal(t, "newer", apptest.Body(t, resp))
}

func Test_Cache_InvalidExpiresMemoryRevalidationCleansEntry(t *testing.T) {
	t.Parallel()
	var noCache atomic.Bool
	app := fiber.New()
	app.Use(New(Config{
		Expiration: 30 * time.Second,
		MaxBytes:   charged(defaultKey("/cached"), len("stale"), textPlain, "public", "invalid-date"),
	}))
	app.Get("/cached", cacheTestBFlipping(&noCache, "public", "no-cache", fiber.HeaderExpires, "invalid-date"))

	cacheTestAExpect(t, app, "/cached", cacheMiss)

	noCache.Store(true)
	for range 2 {
		cacheTestBExpectNewer(t, app, "/cached")
	}
}

func Test_CacheStorage_ExpiredMetadataRemovesMatchingHeapEntry(t *testing.T) {
	t.Parallel()
	storage := storagetest.New()
	probe := &accountingProbe{}
	var originCalls atomic.Int32
	key := cacheKeyVersion + "|GET|/cached"
	stored := charged(key, len("00001"), textPlain, "public, max-age=60")
	app := cacheTestBPathKeyedApp(&Config{MaxBytes: stored, Storage: storage, accounting: probe.record})
	app.Get("/cached", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "public, max-age=60")
		return c.SendString(fmt.Sprintf("%05d", originCalls.Add(1)))
	})

	cacheTestAExpect(t, app, "/cached", cacheMiss)

	storage.Mu.Lock()
	var entry item
	_, err := entry.UnmarshalMsg(storage.Data[key])
	if err == nil {
		entry.ttl = 0
		entry.exp = 1
		storage.Data[key], err = entry.MarshalMsg(nil)
	}
	storage.Mu.Unlock()
	require.NoError(t, err)

	cacheTestAExpectBody(t, app, "/cached", cacheMiss, "00002")
	cacheTestBRequireAccounting(t, probe, stored, 1)
}

// replacementCharge is the "newer!" entry the replacement tests store for /cached.
var replacementCharge = charged(cacheKeyVersion+"|GET|/cached", len("newer!"), textPlain, "public, max-age=60")

// cacheTestBVersionedOrigin answers /cached by its version query: the
// "uncacheable" version with a no-cache "old origin", the "replacement" version
// with a cacheable "newer!", and any other with a cacheable "stale". When
// started is not nil, the uncacheable version closes it and then waits for
// release before it answers, which holds that request in the origin.
func cacheTestBVersionedOrigin(started chan<- struct{}, release <-chan struct{}) fiber.Handler {
	return func(c fiber.Ctx) error {
		switch fiber.Query[string](c, "version") {
		case "uncacheable":
			if started != nil {
				close(started)
				<-release
			}
			c.Set(fiber.HeaderCacheControl, "no-cache")
			return c.SendString("old origin")
		case "replacement":
			c.Set(fiber.HeaderCacheControl, "public, max-age=60")
			return c.SendString("newer!")
		default:
			c.Set(fiber.HeaderCacheControl, "public, max-age=60")
			return c.SendString("stale")
		}
	}
}

// cacheTestBRace is the interleaving the concurrent uncacheable-revalidation
// tests share: a revalidation of /cached whose answer turns out uncacheable is
// parked, and while it is, another revalidation replaces the entry.
type cacheTestBRace struct {
	app    *fiber.App
	parked *cacheTestBParked
}

// cacheTestBStartRace caches "stale" at /cached, then parks on release the
// revalidation of it that the origin answers uncacheable, which is parked once
// parked is closed.
func cacheTestBStartRace(t *testing.T, app *fiber.App, parked <-chan struct{}, release chan struct{}) *cacheTestBRace {
	t.Helper()

	cacheTestAExpect(t, app, "/cached", cacheMiss)
	uncacheable := apptest.Req(fiber.MethodGet, "/cached?version=uncacheable", apptest.WithHeader(fiber.HeaderCacheControl, "max-age=0"))
	return &cacheTestBRace{app: app, parked: cacheTestBPark(t, app, uncacheable, parked, release)}
}

// replace completes, with the first revalidation parked, a second one that the
// origin answers cacheable: it replaces the entry, and is a miss.
func (r *cacheTestBRace) replace(t *testing.T) {
	t.Helper()

	replacement := apptest.Req(fiber.MethodGet, "/cached?version=replacement", apptest.WithHeader(fiber.HeaderCacheControl, "max-age=0"))
	resp := apptest.Expect(t, r.app, replacement, fiber.StatusOK)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	apptest.BodyBytes(t, resp)
}

// finish lets the parked revalidation go on and requires the origin's
// uncacheable answer to reach its client.
func (r *cacheTestBRace) finish(t *testing.T) {
	t.Helper()

	resp := r.parked.finish(t)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	require.Equal(t, "old origin", apptest.Body(t, resp))
}

func Test_Cache_ConcurrentUncacheableCleanupPreservesReplacementAccounting(t *testing.T) {
	t.Parallel()
	storage := &blockingDeleteStorage{
		Memory:         storagetest.New(),
		deleteStarted:  make(chan struct{}),
		continueDelete: make(chan struct{}),
		key:            cacheKeyVersion + "|GET|/cached",
	}
	probe := &accountingProbe{}
	app := cacheTestBPathKeyedApp(&Config{MaxBytes: 2 * replacementCharge, Storage: storage, accounting: probe.record})
	app.Get("/cached", cacheTestBVersionedOrigin(nil, nil))

	race := cacheTestBStartRace(t, app, storage.deleteStarted, storage.continueDelete)
	race.replace(t)
	cacheTestBRequireAccounting(t, probe, replacementCharge, 1)

	race.finish(t)
	require.Equal(t, replacementCharge, probe.counted(), "cleanup must not subtract a replacement's bytes")
	require.Equal(t, 1, probe.nodes(), "cleanup must not remove a replacement's heap node")
}

func Test_Cache_InMemoryUncacheableRevalidationPreservesConcurrentReplacement(t *testing.T) {
	t.Parallel()
	originStarted := make(chan struct{})
	continueOrigin := make(chan struct{})
	probe := &accountingProbe{}
	app := cacheTestBPathKeyedApp(&Config{MaxBytes: 2 * replacementCharge, accounting: probe.record})
	app.Get("/cached", cacheTestBVersionedOrigin(originStarted, continueOrigin))

	race := cacheTestBStartRace(t, app, originStarted, continueOrigin)
	race.replace(t)
	cacheTestBRequireAccounting(t, probe, replacementCharge, 1)

	race.finish(t)
	cacheTestBRequireAccounting(t, probe, replacementCharge, 1)

	cacheTestAExpectBody(t, app, "/cached", cacheHit, "newer!")
}

func Test_Cache_ExternalUncacheableRevalidationPreservesCompletedReplacement(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		maxBytes uint
	}{
		{name: "with MaxBytes", maxBytes: 2 * replacementCharge},
		{name: "without MaxBytes"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			originStarted := make(chan struct{})
			continueOrigin := make(chan struct{})
			storage := storagetest.New()
			probe := &accountingProbe{}
			app := cacheTestBPathKeyedApp(&Config{MaxBytes: tt.maxBytes, Storage: storage, accounting: probe.record})
			app.Get("/cached", cacheTestBVersionedOrigin(originStarted, continueOrigin))

			race := cacheTestBStartRace(t, app, originStarted, continueOrigin)
			race.replace(t)
			if tt.maxBytes > 0 {
				cacheTestBRequireAccounting(t, probe, replacementCharge, 1)
			}

			race.finish(t)
			if tt.maxBytes > 0 {
				cacheTestBRequireAccounting(t, probe, replacementCharge, 1)
			}

			key := cacheKeyVersion + "|GET|/cached"
			storage.Mu.RLock()
			require.Contains(t, storage.Data, key)
			require.Equal(t, []byte("newer!"), storage.Data[bodyKey(key)])
			storage.Mu.RUnlock()

			cacheTestAExpectBody(t, app, "/cached", cacheHit, "newer!")
		})
	}
}

func Test_CacheStorage_NoStoreRevalidationCleansVaryManifest(t *testing.T) {
	t.Parallel()
	for _, failDelete := range []bool{false, true} {
		t.Run(fmt.Sprintf("deleteFailure=%t", failDelete), func(t *testing.T) {
			t.Parallel()
			storage := storagetest.New()
			var noStore atomic.Bool
			app := cacheTestBPathKeyedApp(&Config{Storage: storage})
			app.Get("/cached", cacheTestBFlipping(&noStore, "public, max-age=60", "no-store", fiber.HeaderVary, "Accept"))

			accepting := apptest.WithHeader(fiber.HeaderAccept, "application/json")
			cacheTestAExpect(t, app, apptest.Req(fiber.MethodGet, "/cached", accepting), cacheMiss)

			manifestKey := manifestKeyPrefix + cacheKeyVersion + "|GET|/cached"
			require.True(t, storage.Holds(manifestKey), "the manifest is stored")
			if failDelete {
				storage.SetErr(storagetest.OpDelete, manifestKey, errors.New("manifest delete failed"))
			}

			noStore.Store(true)
			reload := cacheTestAGetCC("/cached", "max-age=0")
			reload.Header.Set(fiber.HeaderAccept, "application/json")
			cacheTestBExpectNewer(t, app, reload)
			require.Equal(t, failDelete, storage.Holds(manifestKey))
		})
	}
}

func Test_CacheControlNotOverwritten(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 10 * time.Second, StoreResponseHeaders: true}))
	app.Get("/", cacheTestAServe("private", "ok"))

	apptest.Do(t, app, "/")

	resp := apptest.Do(t, app, "/")
	require.Equal(t, "private", resp.Header.Get(fiber.HeaderCacheControl))
}

func Test_CacheMaxAgeDirective(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 10 * time.Second}))
	app.Get("/", cacheTestAServe("max-age=1", "1"))

	apptest.Do(t, app, "/")

	time.Sleep(1500 * time.Millisecond)

	cacheTestAExpect(t, app, "/", cacheMiss)
}

func TestCacheOnlyIfCachedRejectsNonShareableAuthorizationEntry(t *testing.T) {
	t.Parallel()
	const cacheHeader = "X-Test-Cache"

	storage := storagetest.New()
	app := fiber.New()
	app.Use(New(Config{CacheHeader: cacheHeader, Expiration: time.Hour, Storage: storage}))

	var originCalls atomic.Int32
	app.Get("/", func(c fiber.Ctx) error {
		call := originCalls.Add(1)
		if call == 1 {
			c.Set(fiber.HeaderCacheControl, "public, max-age=3600")
		}
		return c.SendString(fmt.Sprintf("origin-%d", call))
	})

	newAuthorizationRequest := func(cacheControl, pragma string) *http.Request {
		return apptest.Req(fiber.MethodGet, "/",
			apptest.WithHeader(fiber.HeaderAuthorization, "Bearer token"),
			apptest.WithHeaderIf(fiber.HeaderCacheControl, cacheControl),
			apptest.WithHeaderIf(fiber.HeaderPragma, pragma))
	}

	resp := apptest.Expect(t, app, newAuthorizationRequest("", ""), fiber.StatusOK)
	require.Equal(t, cacheMiss, resp.Header.Get(cacheHeader))
	require.Equal(t, int32(1), originCalls.Load())

	metadataKey := ""
	for key, value := range storage.Data {
		if strings.HasPrefix(key, bodyKeyPrefix) {
			continue
		}

		var cached item
		_, err := cached.UnmarshalMsg(value)
		require.NoError(t, err)
		cached.shareable = false
		storage.Data[key], err = cached.MarshalMsg(nil)
		require.NoError(t, err)
		metadataKey = key
	}
	require.NotEmpty(t, metadataKey)

	// The entry is not shareable, so a request that only takes a stored answer
	// is refused, and the entry stays.
	expectRefused := func(cacheControl, pragma string) {
		t.Helper()

		refused := apptest.Expect(t, app, newAuthorizationRequest(cacheControl, pragma), fiber.StatusGatewayTimeout)
		require.Equal(t, cacheUnreachable, refused.Header.Get(cacheHeader))
		require.Equal(t, int32(1), originCalls.Load())
		require.Contains(t, storage.Data, metadataKey)
		require.Contains(t, storage.Data, bodyKey(metadataKey))
	}
	expectRefused("only-if-cached", "")
	expectRefused("no-cache, only-if-cached", "")
	expectRefused("only-if-cached", "no-cache")

	resp = apptest.Expect(t, app, newAuthorizationRequest("no-cache", ""), fiber.StatusOK)
	require.Equal(t, cacheUnreachable, resp.Header.Get(cacheHeader))
	require.Equal(t, "origin-2", apptest.Body(t, resp))
	require.Equal(t, int32(2), originCalls.Load())
	require.Contains(t, storage.Data, metadataKey)
	require.Contains(t, storage.Data, bodyKey(metadataKey))
}

func TestCacheSeparatesAuthorizationValues(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 10 * time.Second}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "public, max-age=60")
		return c.SendString(fmt.Sprintf("body-%d-%s", count, c.Get(fiber.HeaderAuthorization)))
	})

	newRequest := func(token string) *http.Request {
		return apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderAuthorization, "Bearer "+token))
	}

	cacheTestAExpectBody(t, app, newRequest("token-a"), cacheMiss, "body-1-Bearer token-a")
	cacheTestAExpectBody(t, app, newRequest("token-a"), cacheHit, "body-1-Bearer token-a")
	require.Equal(t, 1, count)
	cacheTestAExpectBody(t, app, newRequest("token-b"), cacheMiss, "body-2-Bearer token-b")
	require.Equal(t, 2, count)
	cacheTestAExpectBody(t, app, newRequest("token-b"), cacheHit, "body-2-Bearer token-b")
	cacheTestAExpectBody(t, app, newRequest("token-a"), cacheHit, "body-1-Bearer token-a")
	require.Equal(t, 2, count)
}

// cacheBenchReadme answers with the repository's README, a body of tens of kilobytes.
func cacheBenchReadme(c fiber.Ctx) error {
	data, _ := os.ReadFile("../../README.md") //nolint:errcheck // We're inside a benchmark
	return c.Status(fiber.StatusOK).Send(data)
}

// cacheBenchStored returns the handler of app and a context for a request of method for /demo, which
// prepare completes unless it is nil, whose answer the cache already holds. One untimed request
// populates the entry, so every measured iteration is a hit rather than the first being the miss
// that fills the cache. b.Loop resets the timer when it first returns, so this stays out of the
// numbers. Without it a benchmark also fails outright at -benchtime=1x, where the single iteration
// is that miss.
func cacheBenchStored(
	app *fiber.App, method string, prepare func(*fasthttp.Request),
) (fasthttp.RequestHandler, *fasthttp.RequestCtx) {
	h := app.Handler()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(method)
	fctx.Request.SetRequestURI("/demo")
	if prepare != nil {
		prepare(&fctx.Request)
	}
	h(fctx)
	return h, fctx
}

// cacheBenchRequireHit requires the last answer of a benchmark to be a hit with status 200.
func cacheBenchRequireHit(b *testing.B, fctx *fasthttp.RequestCtx) {
	b.Helper()

	require.Equal(b, fiber.StatusOK, fctx.Response.Header.StatusCode())
	require.Equal(b, cacheHit, string(fctx.Response.Header.Peek("X-Cache")))
}

// Benchmark_Cache measures the hit path: the first request stores the response
// and every one after it is served from the entry.
//
// The status has to be one this middleware treats as cacheable, which is the
// set RFC 9110 Section 15.1 marks heuristically cacheable — RFC 9111 defines
// caching but has no Section 15, and defers the status codes to that one. This
// was 418, which is not in the set, so the middleware answered "unreachable"
// every time and the benchmark measured the path that stores nothing while the
// handler re-read the file on every iteration.
func Benchmark_Cache(b *testing.B) {
	app := fiber.New()
	app.Use(New())
	app.Get("/demo", cacheBenchReadme)
	h, fctx := cacheBenchStored(app, fiber.MethodGet, nil)

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	cacheBenchRequireHit(b, fctx)
	require.Greater(b, len(fctx.Response.Body()), 30000)
}

// Benchmark_Cache_HitConfigs measures a hit under the settings that add work to it: a Vary, an invalidator,
// credentials as long as a real token, and a refusal recorded in the URL's group.
func Benchmark_Cache_HitConfigs(b *testing.B) {
	cases := []struct {
		name    string
		vary    string
		cfg     Config
		refused bool // another session's refusal is recorded before the hits are measured
	}{
		{name: "default"},
		{name: "vary", vary: "Accept-Encoding, Accept-Language"},
		{name: "vary Cookie", vary: "Cookie"},
		{name: "invalidator", cfg: Config{CacheInvalidator: func(fiber.Ctx) bool { return false }}},
		{name: "KeyCookies", cfg: Config{KeyCookies: []string{"session"}}},
		{name: "KeyCookies, refusal recorded", cfg: Config{KeyCookies: []string{"session"}}, refused: true},
		{name: "KeyHeaders X-API-Key", cfg: Config{KeyHeaders: []string{"X-API-Key"}}},
	}

	for i := range cases {
		bc := &cases[i]
		b.Run(bc.name, func(b *testing.B) {
			var ended atomic.Bool
			app := fiber.New()
			app.Use(New(bc.cfg))
			app.Get("/demo", func(c fiber.Ctx) error {
				if ended.Load() && c.Cookies("session") == "ended" {
					return c.SendStatus(fiber.StatusUnauthorized)
				}
				if bc.vary != "" {
					c.Set(fiber.HeaderVary, bc.vary)
				}
				return c.SendString("body")
			})
			h, fctx := cacheBenchStored(app, fiber.MethodGet, func(req *fasthttp.Request) {
				req.Header.Set("X-API-Key", "tok_0123456789abcdefghijklmnopqrstuvwxyz")
				req.Header.SetCookie("session", "abc")
			})

			if bc.refused {
				// Another session's entry, then its refusal on a revalidation, leave a record in the group.
				other := &fasthttp.RequestCtx{}
				other.Request.Header.SetMethod(fiber.MethodGet)
				other.Request.SetRequestURI("/demo")
				other.Request.Header.SetCookie("session", "ended")
				h(other)
				ended.Store(true)
				other.Request.Header.Set(fiber.HeaderCacheControl, "no-cache")
				h(other)
				require.Equal(b, fiber.StatusUnauthorized, other.Response.StatusCode())
			}

			b.ReportAllocs()

			for b.Loop() {
				h(fctx)
			}

			cacheBenchRequireHit(b, fctx)
		})
	}
}

func Benchmark_Cache_Miss(b *testing.B) {
	app := fiber.New()
	app.Use(New())
	app.Get("/*", cacheBenchReadme)

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)

	b.ReportAllocs()
	b.ResetTimer()

	var n int
	for b.Loop() {
		n++
		fctx.Request.SetRequestURI("/demo/" + strconv.Itoa(n))
		h(fctx)
	}

	require.Equal(b, fiber.StatusOK, fctx.Response.Header.StatusCode())
	require.Greater(b, len(fctx.Response.Body()), 30000)
}

func Benchmark_Cache_Storage(b *testing.B) {
	app := fiber.New()
	app.Use(New(Config{Storage: memory.New()}))
	app.Get("/demo", cacheBenchReadme)
	h, fctx := cacheBenchStored(app, fiber.MethodGet, nil)

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	cacheBenchRequireHit(b, fctx)
	require.Greater(b, len(fctx.Response.Body()), 30000)
}

func Benchmark_Cache_AdditionalHeaders(b *testing.B) {
	app := fiber.New()
	app.Use(New(Config{StoreResponseHeaders: true}))
	app.Get("/demo", func(c fiber.Ctx) error {
		c.Response().Header.Add("X-Foobar", "foobar")
		return c.SendStatus(fiber.StatusOK)
	})
	h, fctx := cacheBenchStored(app, fiber.MethodGet, nil)

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	cacheBenchRequireHit(b, fctx)
	// Exactly one line, not one per iteration. The handler adds it on the miss
	// and the hit path replaces what it stored; while the response was never
	// cached the handler ran every time and its Add piled up on the reused
	// context, so the benchmark grew its own work as it went.
	require.Equal(b, [][]byte{[]byte("foobar")}, fctx.Response.Header.PeekAll("X-Foobar"))
}

func Benchmark_Cache_QueryMethod(b *testing.B) {
	app := fiber.New()
	app.Use(New(Config{Methods: []string{fiber.MethodQuery}}))
	app.Query("/demo", func(c fiber.Ctx) error { return c.SendString("ok") })
	h, fctx := cacheBenchStored(app, fiber.MethodQuery,
		func(req *fasthttp.Request) { req.SetBody([]byte(`{"filter":"active","page":1}`)) })

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	cacheBenchRequireHit(b, fctx)
}

func Benchmark_Cache_MaxSize(b *testing.B) {
	// The benchmark is run with three different MaxSize parameters
	// 1) 0:        Tracking is disabled = no overhead
	// 2) MaxInt32: Enough to store all entries = no removals
	// 3) 16 KiB:   Small size = constant insertions and removals
	cases := []uint{0, math.MaxUint32, 16 * 1024}
	names := []string{"Disabled", "Unlim", "LowBounded"}
	for i, size := range cases {
		b.Run(names[i], func(b *testing.B) {
			app := fiber.New()
			app.Use(New(Config{MaxBytes: size}))

			app.Get("/*", func(c fiber.Ctx) error { return c.Status(fiber.StatusOK).SendString("1") })

			h := app.Handler()
			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(fiber.MethodGet)

			b.ReportAllocs()

			// strconv, not fmt.Sprintf: the formatting is the benchmark's own
			// setup and its allocation was landing in the middleware's numbers.
			n := 0
			uri := make([]byte, 0, 24)
			for b.Loop() {
				n++
				uri = strconv.AppendInt(append(uri[:0], '/'), int64(n), 10)
				fctx.Request.SetRequestURIBytes(uri)
				h(fctx)
			}

			// Stored, so the bounded case actually reaches the eviction loop this
			// benchmark exists to measure.
			require.Equal(b, fiber.StatusOK, fctx.Response.Header.StatusCode())
			require.Equal(b, cacheMiss, string(fctx.Response.Header.Peek("X-Cache")))
		})
	}
}

func Test_Cache_RevalidationWithMaxBytes(t *testing.T) {
	t.Parallel()
	// A request the cached entry cannot answer revalidates it, and the next
	// request is served the new entry.
	for _, tc := range []struct {
		name string
		// Cache-Control of the origin's response.
		cacheControl string
		// Cache-Control of the revalidating request.
		revalidate string
		// How long the entry ages before it is revalidated.
		wait time.Duration
	}{
		{name: "max-age=0 revalidation removes old entry on storage success", cacheControl: "max-age=60", revalidate: "max-age=0"},
		{name: "min-fresh revalidation with MaxBytes", cacheControl: "max-age=2", revalidate: "min-fresh=5", wait: time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(Config{MaxBytes: 1000}))
			requestCount := 0
			app.Get("/test", func(c fiber.Ctx) error {
				requestCount++
				c.Set(fiber.HeaderCacheControl, tc.cacheControl)
				return c.SendString(fmt.Sprintf("response-%d", requestCount))
			})

			cacheTestAExpect(t, app, "/test", cacheMiss)
			if tc.wait > 0 {
				time.Sleep(tc.wait)
			}
			require.Equal(t, "response-2", apptest.Body(t, apptest.Do(t, app, cacheTestAGetCC("/test", tc.revalidate))))
			resp := cacheTestAExpect(t, app, "/test", cacheHit)
			require.Equal(t, "response-2", apptest.Body(t, resp), "New entry should be cached")
		})
	}

	t.Run("revalidation respects MaxBytes eviction", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{
			MaxBytes:            2 * charged(defaultKey("/a"), len("1234567890"), textPlain, "max-age=60"), // Only room for 2 responses
			ExpirationGenerator: stableAscendingExpiration(),
		}))
		app.Get("/*", cacheTestAServe("max-age=60", "1234567890")) // 10 bytes

		cacheTestAExpect(t, app, "/a", cacheMiss)
		cacheTestAExpect(t, app, "/b", cacheMiss)
		cacheTestAExpect(t, app, "/a", cacheHit)
		cacheTestAExpect(t, app, "/b", cacheHit)
		apptest.Do(t, app, cacheTestAGetCC("/a", "max-age=0"))
		cacheTestAExpect(t, app, "/a", cacheHit)
		// /b is still cached, so the heap accounting stayed correct.
		cacheTestAExpect(t, app, "/b", cacheHit)
	})

	t.Run("no-store revalidation removes old in-memory entry", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{MaxBytes: 1000}))
		requestCount := 0
		app.Get("/test", func(c fiber.Ctx) error {
			requestCount++
			if requestCount == 1 {
				c.Set(fiber.HeaderCacheControl, "max-age=60")
				return c.SendString("cacheable")
			}
			c.Set(fiber.HeaderCacheControl, "no-store")
			return c.SendString("not-cacheable")
		})

		cacheTestAExpectBody(t, app, "/test", cacheMiss, "cacheable")
		cacheTestAExpectBody(t, app, cacheTestAGetCC("/test", "max-age=0"), cacheUnreachable, "not-cacheable")
		// A later request must reach the origin instead of serving the stale body.
		cacheTestAExpectBody(t, app, "/test", cacheUnreachable, "not-cacheable")
		require.Equal(t, 3, requestCount)
	})
}

// Test_parseCacheControlDirectives_QuotedStrings tests RFC 9111 Section 5.2 compliance
// for quoted-string values in Cache-Control directives
func Test_parseCacheControlDirectives_QuotedStrings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		want  map[string]string
		input string
	}{
		{name: "simple quoted value", input: `community="UCI"`, want: map[string]string{"community": "UCI"}},
		{
			name: "multiple directives with quoted values", input: `max-age=3600, community="UCI", custom="value"`,
			want: map[string]string{"max-age": "3600", "community": "UCI", "custom": "value"},
		},
		{
			name: "quoted value with spaces", input: `custom="value with spaces"`,
			want: map[string]string{"custom": "value with spaces"},
		},
		{
			name: "quoted value with escaped quote", input: `custom="value with \"quotes\""`,
			want: map[string]string{"custom": `value with "quotes"`},
		},
		{
			name: "quoted value with escaped backslash", input: `custom="value with \\ backslash"`,
			want: map[string]string{"custom": `value with \ backslash`},
		},
		{
			name: "mixed quoted and unquoted values", input: `max-age=3600, community="UCI", no-cache, custom="test"`,
			want: map[string]string{"max-age": "3600", "community": "UCI", "no-cache": "", "custom": "test"},
		},
		{name: "quoted empty value", input: `custom=""`, want: map[string]string{"custom": ""}},
		{
			name: "spaces around quoted value", input: `custom = "value" , another="test"`,
			want: map[string]string{"custom": "value", "another": "test"},
		},
		{
			name: "tabs around directives and values", input: "\tmax-age\t=\t3600\t,\tcommunity\t=\t\"UCI\"\t",
			want: map[string]string{"max-age": "3600", "community": "UCI"},
		},
		{
			name: "whitespace other than SP and HTAB is kept", input: "max-age=\v3600",
			want: map[string]string{"max-age": "\v3600"},
		},
		{name: "unquoted token value", input: `max-age=3600`, want: map[string]string{"max-age": "3600"}},
		{
			name:  "complex mixed case",
			input: `max-age=3600, s-maxage=7200, community="UCI", no-store, custom="value with \"escaped\" quotes"`,
			want: map[string]string{
				"max-age": "3600", "s-maxage": "7200", "community": "UCI", "no-store": "",
				"custom": `value with "escaped" quotes`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := make(map[string]string)
			parseCacheControlDirectives([]byte(tt.input), func(key, value []byte) {
				result[string(key)] = string(value)
			})
			require.Equal(t, tt.want, result)
		})
	}
}

// Test_Cache_MaxBytes_InsufficientSpace tests the "insufficient space" error path
// when an entry is larger than MaxBytes, ensuring such entries are treated as unreachable
func Test_Cache_MaxBytes_InsufficientSpace(t *testing.T) {
	t.Parallel()
	t.Run("entry larger than MaxBytes with empty cache", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{MaxBytes: 10, Expiration: 1 * time.Hour}))
		app.Get("/large", func(c fiber.Ctx) error { return c.Send(make([]byte, 20)) })

		cacheTestAExpect(t, app, "/large", cacheUnreachable)
	})

	t.Run("entry larger than MaxBytes after eviction", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{
			MaxBytes:            charged(defaultKey("/large"), 20, textPlain) - 1,
			ExpirationGenerator: stableAscendingExpiration(),
		}))
		app.Get("/*", func(c fiber.Ctx) error {
			if c.Path() == "/small" {
				return c.Send(make([]byte, 5))
			}
			return c.Send(make([]byte, 20))
		})

		cacheTestAExpect(t, app, "/small", cacheMiss)
		// The large entry does not fit even once the small one is evicted.
		cacheTestAExpect(t, app, "/large", cacheUnreachable)
	})
}

func Test_Cache_MaxBytes_DeletionFailureRestoresTracking(t *testing.T) {
	t.Parallel()
	storage := storagetest.New()
	app := fiber.New()
	app.Use(New(Config{
		MaxBytes:   charged(defaultKey("/second"), len("data"), textPlain), // one entry
		Expiration: 1 * time.Hour,
		Storage:    storage,
	}))
	app.Get("/:name", func(c fiber.Ctx) error { return c.SendString("data") })

	cacheTestAExpect(t, app, "/first", cacheMiss)

	keys := storage.Keys()
	t.Logf("stored keys after first cache: %v", keys)
	for _, key := range keys {
		if strings.Contains(key, "/first") {
			storage.SetErr(storagetest.OpDelete, key, errors.New("delete failed"))
		}
	}

	// Next request triggers eviction; deletion failure should surface an error
	rsp := apptest.Do(t, app, "/second")
	require.Equal(t, fiber.StatusInternalServerError, rsp.StatusCode)
	require.Contains(t, apptest.Body(t, rsp), "failed to delete key")
	t.Logf("stored keys after deletion failure: %v", storage.Keys())
	storage.ClearErrs()

	// Another request should succeed and be cacheable after restoring heap tracking
	cacheTestAExpect(t, app, "/third", cacheMiss)
}

// Test_Cache_MaxBytes_ConcurrencyAndRaceConditions tests that the race condition fix works correctly
// under concurrent load, verifying that storedBytes never exceeds MaxBytes even with multiple
// goroutines making simultaneous requests. The -race flag detects any regression.
func Test_Cache_MaxBytes_ConcurrencyAndRaceConditions(t *testing.T) {
	t.Parallel()
	t.Run("concurrent requests with MaxBytes limit", func(t *testing.T) {
		t.Parallel()
		const numGoroutines = 20
		const requestsPerGoroutine = 5
		app := fiber.New()
		app.Use(New(Config{MaxBytes: uint(1000), Expiration: 10 * time.Second}))
		app.Get("/*", func(c fiber.Ctx) error { return c.Send(make([]byte, 50)) })

		var wg sync.WaitGroup
		errChan := make(chan error, numGoroutines*requestsPerGoroutine)
		for id := range numGoroutines {
			wg.Go(func() {
				for j := range requestsPerGoroutine {
					req := httptest.NewRequest(fiber.MethodGet, fmt.Sprintf("/test-%d-%d", id, j), http.NoBody)
					if _, err := app.Test(req); err != nil {
						errChan <- err
					}
				}
			})
		}
		wg.Wait()
		close(errChan)
		for err := range errChan {
			require.NoError(t, err, "concurrent request failed")
		}
	})

	t.Run("concurrent requests near capacity triggers eviction", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{MaxBytes: uint(200), Expiration: 10 * time.Second}))
		// Ten responses of 50 bytes exceed the capacity.
		app.Get("/*", func(c fiber.Ctx) error { return c.Send(make([]byte, 50)) })

		var wg sync.WaitGroup
		for id := range 10 {
			wg.Go(func() {
				req := httptest.NewRequest(fiber.MethodGet, fmt.Sprintf("/item-%d", id), http.NoBody)
				if _, err := app.Test(req); err != nil {
					t.Logf("request error: %v", err)
				}
			})
		}
		wg.Wait()
	})
}

// both returns the two results of a call as one value, for a table to compare.
func both[A, B any](a A, b B) [2]any { return [2]any{a, b} }

func Test_Cache_HelperFunctions(t *testing.T) {
	t.Parallel()
	cfg := &Config{Expiration: 1 * time.Hour}
	public := &item{cacheControl: []byte("public")}
	publicExpires := &item{cacheControl: []byte("public"), expires: []byte("Wed, 21 Oct 2015 07:28:00 GMT")}
	pastHeuristicAge := uint64(25 * time.Hour / time.Second)
	contentType := []cachedHeader{{key: []byte("Content-Type"), value: []byte("text/html")}}
	authorization := []cachedHeader{{key: []byte("Authorization"), value: []byte("Bearer token")}}
	mask := func(_ string) string { return "***" }
	storageErr := errors.New("storage error")

	// Each row holds what a call returned, made as the table is built, and what it should return.
	for _, tc := range []struct {
		got, want any
		name      string
	}{
		{name: "parseHTTPDate empty", got: both(parseHTTPDate([]byte{})), want: both(uint64(0), false)},
		{name: "parseHTTPDate invalid", got: both(parseHTTPDate([]byte("invalid"))), want: both(uint64(0), false)},
		{
			name: "parseHTTPDate valid", want: both(uint64(1136214245), true),
			got: both(parseHTTPDate([]byte("Mon, 02 Jan 2006 15:04:05 GMT"))),
		},
		{name: "safeUnixSeconds negative", got: safeUnixSeconds(time.Unix(-1, 0)), want: uint64(0)},
		{name: "safeUnixSeconds positive", got: safeUnixSeconds(time.Unix(1234567890, 0)), want: uint64(1234567890)},
		{name: "remainingFreshness nil", got: remainingFreshness(nil, 100), want: uint64(0)},
		{name: "remainingFreshness zero exp", got: remainingFreshness(&item{exp: 0}, 100), want: uint64(0)},
		{name: "remainingFreshness expired", got: remainingFreshness(&item{exp: 100}, 200), want: uint64(0)},
		{name: "remainingFreshness valid", got: remainingFreshness(&item{exp: 200}, 100), want: uint64(100)},
		{
			name: "lookupCachedHeader not found", want: both([]byte(nil), false),
			got: both(lookupCachedHeader(contentType, "Authorization")),
		},
		{
			name: "lookupCachedHeader case insensitive", want: both([]byte("Bearer token"), true),
			got: both(lookupCachedHeader(authorization, "authorization")),
		},
		{name: "secondsToDuration zero", got: secondsToDuration(0), want: time.Duration(0)},
		{name: "secondsToDuration large", got: secondsToDuration(9223372036), want: 9223372036 * time.Second},
		{name: "secondsToTime zero", got: secondsToTime(0), want: time.Unix(0, 0).UTC()},
		{name: "secondsToTime value", got: secondsToTime(1234567890), want: time.Unix(1234567890, 0).UTC()},
		{name: "isHeuristicFreshness short age", got: isHeuristicFreshness(public, cfg, 3600), want: false},
		{
			name: "isHeuristicFreshness with expires", want: false,
			got: isHeuristicFreshness(publicExpires, cfg, pastHeuristicAge),
		},
		{name: "isHeuristicFreshness true", got: isHeuristicFreshness(public, cfg, pastHeuristicAge), want: true},
		{name: "cacheBodyFetchError other", got: cacheBodyFetchError(mask, "key", storageErr), want: storageErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.got)
		})
	}

	t.Run("cacheBodyFetchError miss", func(t *testing.T) {
		t.Parallel()
		require.ErrorContains(t, cacheBodyFetchError(mask, "key", errCacheMiss), "no cached body")
	})
}

// cacheTestBNewApp returns an app that caches under cfg and whose GET /test
// answers "test" with cacheControl, unless empty, as its Cache-Control and with
// respHeaders.
func cacheTestBNewApp(cfg *Config, cacheControl string, respHeaders map[string]string) *fiber.App {
	app := fiber.New()
	app.Use(New(*cfg))
	app.Get("/test", func(c fiber.Ctx) error {
		if cacheControl != "" {
			c.Response().Header.Set(fiber.HeaderCacheControl, cacheControl)
		}
		for name, value := range respHeaders {
			c.Response().Header.Set(name, value)
		}
		return c.SendString("test")
	})
	return app
}

// cacheTestBCase is a scenario for cacheTestBRun: GET /test is sent to an app
// from cacheTestBNewApp, and the X-Cache status of the responses is checked.
type cacheTestBCase struct {
	// Response headers of the origin besides cacheControl.
	respHeaders map[string]string
	// Request headers, sent with every request.
	reqHeaders map[string]string
	// Request headers added to the second request.
	secondHeaders map[string]string
	// Headers the last response must carry.
	wantHeaders map[string]string
	name        string
	// Cache-Control of the origin's response, unless empty.
	cacheControl string
	// X-Cache of the first response.
	first string
	// X-Cache of the response to a second request; none is sent when it is empty.
	second string
	// Expiration is an hour unless the case sets another.
	cfg Config
}

// cacheTestBRun runs the cases as parallel subtests of t.
func cacheTestBRun(t *testing.T, cases []cacheTestBCase) {
	t.Helper()

	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := tc.cfg
			cfg.Expiration = cmp.Or(cfg.Expiration, time.Hour)
			app := cacheTestBNewApp(&cfg, tc.cacheControl, tc.respHeaders)

			newRequest := func(headers ...map[string]string) *http.Request {
				var opts []apptest.Option
				for _, h := range headers {
					for name, value := range h {
						opts = append(opts, apptest.WithHeader(name, value))
					}
				}
				return apptest.Req(fiber.MethodGet, "/test", opts...)
			}
			resp := apptest.Do(t, app, newRequest(tc.reqHeaders))
			require.Equal(t, tc.first, resp.Header.Get("X-Cache"), "first request")
			if tc.second != "" {
				resp = apptest.Do(t, app, newRequest(tc.reqHeaders, tc.secondHeaders))
				require.Equal(t, tc.second, resp.Header.Get("X-Cache"), "second request")
			}
			for name, value := range tc.wantHeaders {
				require.Equal(t, value, resp.Header.Get(name), name)
			}
		})
	}
}

func Test_Cache_VaryAndAuth(t *testing.T) {
	t.Parallel()
	t.Run("storeVaryManifest failure", func(t *testing.T) {
		t.Parallel()
		storage := storagetest.New()
		storage.SetErr(storagetest.OpSet, "manifest", errors.New("storage fail"))
		manager := &manager{storage: storage}
		require.Error(t, storeVaryManifest(context.Background(), manager, "manifest", []string{"Accept"}, 3600*time.Second))
	})

	t.Run("loadVaryManifest not found", func(t *testing.T) {
		t.Parallel()
		manager := &manager{storage: storagetest.New()}
		varyNames, found, err := loadVaryManifest(context.Background(), manager, "nonexistent")
		require.NoError(t, err)
		require.False(t, found)
		require.Nil(t, varyNames)
	})

	cacheTestBRun(t, []cacheTestBCase{
		{
			name: "vary with multiple headers", cacheControl: "max-age=3600", first: cacheMiss, second: cacheHit,
			respHeaders: map[string]string{"Vary": "Accept, Accept-Encoding"},
			reqHeaders:  map[string]string{"Accept": "application/json", "Accept-Encoding": "gzip"},
		},
		{
			name: "auth with must-revalidate", cacheControl: "must-revalidate, max-age=3600",
			reqHeaders: map[string]string{"Authorization": "Bearer token1"}, first: cacheMiss, second: cacheHit,
		},
	})
}

func Test_Cache_DateAndCacheControl(t *testing.T) {
	t.Parallel()
	cacheTestBRun(t, []cacheTestBCase{
		{
			name: "date header parsing", cacheControl: "max-age=3600", first: cacheMiss,
			respHeaders: map[string]string{"Date": "Mon, 02 Jan 2006 15:04:05 GMT"},
		},
		{
			name: "invalid date header", cacheControl: "max-age=3600", first: cacheMiss,
			respHeaders: map[string]string{"Date": "invalid"},
		},
		{name: "cache control with quoted values", cacheControl: `max-age=3600, ext="value, with, commas"`, first: cacheMiss},
		{name: "cache control with spaces", cacheControl: "max-age=3600  ,  public  ,  must-revalidate", first: cacheMiss},
	})
}

func Test_Cache_CacheControlCombinations(t *testing.T) {
	t.Parallel()
	cacheTestBRun(t, []cacheTestBCase{
		{name: "max-age with public", cacheControl: "public, max-age=3600", first: cacheMiss, second: cacheHit},
		{name: "max-age with private", cacheControl: "private, max-age=3600", first: cacheUnreachable},
		{name: "s-maxage overrides max-age", cacheControl: "public, max-age=60, s-maxage=3600", first: cacheMiss, second: cacheHit},
		{name: "no-store prevents caching", cacheControl: "no-store", first: cacheUnreachable, second: cacheUnreachable},
		{name: "no-cache with etag", cacheControl: "no-cache", respHeaders: map[string]string{"ETag": `"123456"`}, first: cacheUnreachable},
		{name: "must-revalidate with max-age", cacheControl: "must-revalidate, max-age=3600", first: cacheMiss, second: cacheHit},
		{name: "proxy-revalidate with max-age", cacheControl: "public, proxy-revalidate, max-age=3600", first: cacheMiss, second: cacheHit},
		{name: "immutable with max-age", cacheControl: "public, max-age=31536000, immutable", first: cacheMiss, second: cacheHit},
		{name: "max-age=0 with must-revalidate", cacheControl: "max-age=0, must-revalidate", first: cacheUnreachable},
		{name: "public with no explicit max-age", cacheControl: "public", first: cacheMiss, second: cacheHit},
		{name: "multiple cache directives with extensions", cacheControl: `public, max-age=3600, custom="value"`, first: cacheMiss, second: cacheHit},
		{name: "private overrides public", cacheControl: "public, private, max-age=3600", first: cacheUnreachable},
		{name: "stale-while-revalidate with max-age", cacheControl: "max-age=60, stale-while-revalidate=120", first: cacheMiss, second: cacheHit},
		{name: "stale-if-error with max-age", cacheControl: "max-age=60, stale-if-error=3600", first: cacheMiss, second: cacheHit},
	})
}

func Test_Cache_RequestResponseDirectives(t *testing.T) {
	t.Parallel()
	cacheTestBRun(t, []cacheTestBCase{
		{
			name: "request with pragma no-cache", cacheControl: "max-age=3600", first: cacheMiss,
			reqHeaders: map[string]string{"Pragma": "no-cache"},
		},
		{
			name: "request with max-age=0 directive", cacheControl: "max-age=3600", first: cacheMiss, second: cacheMiss,
			secondHeaders: map[string]string{"Cache-Control": "max-age=0"},
		},
		{
			name: "response with expires header", first: cacheMiss, second: cacheHit,
			respHeaders: map[string]string{"Expires": time.Now().Add(1 * time.Hour).UTC().Format(http.TimeFormat)},
		},
		{
			name: "response with age header", cacheControl: "max-age=3600", first: cacheMiss, second: cacheHit,
			respHeaders: map[string]string{"Age": "30"},
		},
		{
			name: "custom key generator", first: cacheMiss, second: cacheHit,
			cfg: Config{KeyGenerator: func(c fiber.Ctx) string { return "custom-" + c.Path() }},
		},
		{
			name: "only-if-cached with cache hit", cacheControl: "max-age=3600", first: cacheMiss, second: cacheHit,
			secondHeaders: map[string]string{"Cache-Control": "only-if-cached"},
		},
		{name: "cache control with uppercase directives", cacheControl: "PUBLIC, MAX-AGE=3600", first: cacheMiss, second: cacheHit},
	})

	t.Run("negative expiration skips caching", func(t *testing.T) {
		t.Parallel()
		app := cacheTestBNewApp(&Config{Expiration: -1 * time.Second}, "", nil)
		rsp := apptest.Do(t, app, "/test")
		require.NotEqual(t, cacheMiss, rsp.Header.Get("X-Cache"))
		require.NotEqual(t, cacheHit, rsp.Header.Get("X-Cache"))
	})

	t.Run("request with no-store directive", func(t *testing.T) {
		t.Parallel()
		app := cacheTestBNewApp(&Config{Expiration: 1 * time.Hour}, "", nil)
		req := apptest.Req(fiber.MethodGet, "/test", apptest.WithHeader("Cache-Control", "no-store"))
		require.NotEqual(t, cacheMiss, apptest.Do(t, app, req).Header.Get("X-Cache"))
	})

	t.Run("non-get-head method bypasses cache", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Post("/test", func(c fiber.Ctx) error { return c.SendString("test") })
		cacheTestAExpect(t, app, apptest.Req(fiber.MethodPost, "/test"), cacheUnreachable)
	})

	t.Run("request with min-fresh directive", func(t *testing.T) {
		t.Parallel()
		app := cacheTestBNewApp(&Config{Expiration: 1 * time.Hour}, "max-age=60", nil)
		cacheTestAExpect(t, app, "/test", cacheMiss)
		req := apptest.Req(fiber.MethodGet, "/test", apptest.WithHeader("Cache-Control", "min-fresh=120"))
		cacheStatus := apptest.Do(t, app, req).Header.Get("X-Cache")
		require.Contains(t, []string{cacheMiss, cacheUnreachable}, cacheStatus, "min-fresh requirement should prevent cache hit")
	})

	t.Run("request with max-stale directive", func(t *testing.T) {
		t.Parallel()
		app := cacheTestBNewApp(&Config{Expiration: 1 * time.Second}, "max-age=1", nil)
		cacheTestAExpect(t, app, "/test", cacheMiss)

		time.Sleep(2 * time.Second)

		req := apptest.Req(fiber.MethodGet, "/test", apptest.WithHeader("Cache-Control", "max-stale=60"))
		cacheStatus := apptest.Do(t, app, req).Header.Get("X-Cache")
		require.Contains(t, []string{cacheHit, cacheMiss, "stale"}, cacheStatus, "max-stale should allow stale content or revalidate")
	})

	t.Run("response with warning header", func(t *testing.T) {
		t.Parallel()
		app := cacheTestBNewApp(&Config{Expiration: 1 * time.Second}, "max-age=1", nil)
		cacheTestAExpect(t, app, "/test", cacheMiss)

		time.Sleep(2 * time.Second)

		rsp := apptest.Do(t, app, "/test")
		cacheStatus := rsp.Header.Get("X-Cache")
		warningHeader := rsp.Header.Get("Warning")
		require.True(t, cacheStatus == cacheMiss || warningHeader != "", "stale response should either revalidate or have warning header")
	})

	t.Run("external storage with body key", func(t *testing.T) {
		t.Parallel()
		storage := storagetest.New()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour, Storage: storage}))
		app.Get("/test", func(c fiber.Ctx) error { return c.SendString("test content") })

		cacheTestAExpect(t, app, "/test", cacheMiss)
		require.NotEmpty(t, bodyKeys(storage), "the body is stored under a key of its own")
	})

	t.Run("only-if-cached with cache miss", func(t *testing.T) {
		t.Parallel()
		app := cacheTestBNewApp(&Config{Expiration: 1 * time.Hour}, "", nil)
		req := apptest.Req(fiber.MethodGet, "/test", apptest.WithHeader("Cache-Control", "only-if-cached"))
		apptest.Expect(t, app, req, fiber.StatusGatewayTimeout)
	})
}

func Test_Cache_ConfigurationAndResponseHandling(t *testing.T) {
	t.Parallel()
	cacheTestBRun(t, []cacheTestBCase{
		{
			name: "response with Vary star prevents caching", cacheControl: "max-age=3600", first: cacheUnreachable,
			respHeaders: map[string]string{"Vary": "*"},
		},
		{
			name: "authorization without shared cache directives", cacheControl: "max-age=3600",
			reqHeaders: map[string]string{"Authorization": "******"}, first: cacheUnreachable,
		},
		{
			name: "disable cache control header generation", cfg: Config{DisableCacheControl: true},
			cacheControl: "max-age=3600", first: cacheMiss, second: cacheHit,
		},
		{
			name: "disable value redaction", cfg: Config{DisableValueRedaction: true},
			cacheControl: "max-age=3600", first: cacheMiss,
		},
		{
			name: "response with ETag header", cacheControl: "max-age=3600", first: cacheMiss, second: cacheHit,
			respHeaders: map[string]string{"ETag": `"abc123"`}, wantHeaders: map[string]string{"ETag": `"abc123"`},
		},
		{
			name: "response with Content-Encoding header", cacheControl: "max-age=3600", first: cacheMiss,
			second: cacheHit, respHeaders: map[string]string{"Content-Encoding": "gzip"},
			wantHeaders: map[string]string{"Content-Encoding": "gzip"},
		},
		{
			name: "response with custom headers preserved", cfg: Config{StoreResponseHeaders: true},
			cacheControl: "max-age=3600", first: cacheMiss, second: cacheHit,
			respHeaders: map[string]string{"X-Custom-Header": "custom-value"},
			wantHeaders: map[string]string{"X-Custom-Header": "custom-value"},
		},
		{
			// A request with no-cache forces revalidation
			name: "revalidation scenario with cache miss", cacheControl: "max-age=3600", first: cacheMiss,
			reqHeaders: map[string]string{"Cache-Control": "no-cache"},
		},
	})

	t.Run("next function prevents caching", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour, Next: func(c fiber.Ctx) bool { return c.Path() == "/skip" }}))
		app.Get("/skip", func(c fiber.Ctx) error { return c.SendString("test") })
		cacheTestAExpect(t, app, "/skip", cacheUnreachable)
	})

	t.Run("non-cacheable status code", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error { return c.Status(fiber.StatusCreated).SendString("created") })
		cacheTestAExpect(t, app, "/test", cacheUnreachable)
	})

	t.Run("body larger than MaxBytes", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour, MaxBytes: 10}))
		app.Get("/test", func(c fiber.Ctx) error { return c.Send(make([]byte, 100)) })
		cacheTestAExpect(t, app, "/test", cacheUnreachable)
	})

	t.Run("delete vary manifest on no-cache response", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			// The first request creates a vary manifest, the second answers no-cache, which deletes it.
			if c.Query("first") != "" {
				c.Response().Header.Set("Vary", "Accept")
				c.Response().Header.Set("Cache-Control", "max-age=3600")
			} else {
				c.Response().Header.Set("Cache-Control", "no-cache")
			}
			return c.SendString("test")
		})

		req := apptest.Req(fiber.MethodGet, "/test?first=true", apptest.WithHeader("Accept", "application/json"))
		cacheTestAExpect(t, app, req, cacheMiss)
		cacheTestAExpect(t, app, "/test", cacheUnreachable)
	})

	t.Run("vary manifest deletion on different vary response", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		var counter atomic.Int32
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			// The second response has no Vary header, which deletes the manifest.
			if counter.Add(1) == 1 {
				c.Response().Header.Set("Vary", "Accept")
			}
			c.Response().Header.Set("Cache-Control", "max-age=3600")
			return c.SendString("test")
		})

		req := apptest.Req(fiber.MethodGet, "/test", apptest.WithHeader("Accept", "application/json"))
		cacheTestAExpect(t, app, req, cacheMiss)
		cacheTestAExpect(t, app, "/test", cacheMiss)
	})
}

// Test_hasDirective covers RFC 9111 §5.2 directive parsing including
// space, tab, and '=' as valid terminators (fixes #4143).
func Test_hasDirective(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		cc        string
		directive string
		want      bool
	}{
		// Basic matches
		{"exact match", "no-cache", "no-cache", true},
		{"comma separated", "public, no-cache, max-age=0", "no-cache", true},
		{"at start", "no-cache, max-age=0", "no-cache", true},
		{"at end", "public, no-cache", "no-cache", true},
		{"not present", "public, max-age=0", "no-cache", false},
		{"shorter token does not match", "no-catch", "no-cache", false},
		{"substring of longer token", "no-cache-extended", "no-cache", false},

		// Trailing whitespace (#4143)
		{"trailing space", "no-cache ", "no-cache", true},
		{"trailing tab", "no-cache\t", "no-cache", true},
		{"private trailing space", "private ", "private", true},

		// Directive with value (#4143)
		{"directive with equals", `no-cache="Set-Cookie"`, "no-cache", true},
		{"max-age with value", "max-age=3600", "max-age", true},
		{"s-maxage with value in list", "public, s-maxage=600, max-age=3600", "s-maxage", true},

		// Tab as separator before directive
		{"tab before directive", "public,\tno-cache", "no-cache", true},

		// Case insensitive
		{"case insensitive", "No-Cache", "no-cache", true},

		// Empty / edge cases
		{"empty header", "", "no-cache", false},
		{"empty directive never matches", "no-cache", "", false},
		{"empty directive empty header", "", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := hasDirective(tc.cc, tc.directive)
			require.Equal(t, tc.want, got)
		})
	}
}

// Test_CacheInvalidator_NoDataRace exercises the path where one request
// triggers CacheInvalidator (writing e.exp under mux.Lock) while concurrent
// requests evaluate freshness (reading e.exp). The fields must be read under
// the lock; -race will flag a regression. The test also asserts the
// invalidator path actually drives fresh handler executions, so a regression
// that silently disabled invalidation would also fail.
func Test_CacheInvalidator_NoDataRace(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		invalidator  func(fiber.Ctx) bool
		invalidating func() *http.Request
		name         string
	}{
		{
			name:         "RaceWithExactTimestamp",
			invalidator:  cacheTestBInvalidateByQuery,
			invalidating: func() *http.Request { return apptest.Req(fiber.MethodGet, "/?invalidate=true") },
		},
		{
			name: "SharedEntryNoDataRace",
			// Keyed on a header outside the cache key, so every request shares one entry.
			invalidator: func(c fiber.Ctx) bool { return c.Get("X-Invalidate") == "1" },
			invalidating: func() *http.Request {
				return apptest.Req(fiber.MethodGet, "/", apptest.WithHeader("X-Invalidate", "1"))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var handlerCalls atomic.Uint64
			app := fiber.New()
			app.Use(New(Config{
				CacheInvalidator: tc.invalidator,
				// Long expiration so the only source of fresh handler executions
				// after priming is the CacheInvalidator branch.
				Expiration: time.Hour,
			}))

			app.Get("/", func(c fiber.Ctx) error { return c.SendString(strconv.FormatUint(handlerCalls.Add(1), 10)) })

			// Prime the cache so the served-from-cache branch is reachable.
			resp := apptest.Do(t, app, "/")
			require.NoError(t, resp.Body.Close())
			primed := handlerCalls.Load()

			var (
				wg            sync.WaitGroup
				invalidations atomic.Uint64
				errCount      atomic.Uint64
			)
			const workers = 16
			const iterations = 50
			for i := range workers {
				wg.Go(func() {
					for j := range iterations {
						req := apptest.Req(fiber.MethodGet, "/")
						if (i+j)%4 == 0 {
							req = tc.invalidating()
							invalidations.Add(1)
						}
						resp, err := app.Test(req)
						if err != nil {
							t.Errorf("app.Test: %v", err)
							errCount.Add(1)
							return
						}
						if err := resp.Body.Close(); err != nil {
							t.Errorf("Body.Close: %v", err)
							errCount.Add(1)
						}
					}
				})
			}
			wg.Wait()

			require.Zero(t, errCount.Load(), "worker goroutines reported errors")
			require.Positive(t, invalidations.Load(), "test never issued an invalidating request")
			// Each invalidation must drive at least one fresh handler execution.
			// Long Expiration guarantees natural expiry cannot account for the bump.
			require.Greater(t, handlerCalls.Load(), primed, "invalidator never bypassed the cache")
		})
	}
}

func Benchmark_parseCacheControlDirectives(b *testing.B) {
	inputs := [][]byte{
		[]byte("no-cache"),
		[]byte("public, max-age=3600"),
		[]byte("private, no-store, must-revalidate"),
		[]byte(`max-age=30, s-maxage=90, community="UCI", no-cache`),
		[]byte("max-age = 60 ,\tno-transform"),
	}
	var n int
	b.ReportAllocs()
	for b.Loop() {
		for _, in := range inputs {
			parseCacheControlDirectives(in, func(key, value []byte) { n += len(key) + len(value) })
		}
	}
	_ = n
}

func Benchmark_hasDirective(b *testing.B) {
	inputs := []string{
		"no-cache",
		"public, max-age=3600",
		"private, no-store, must-revalidate",
		"max-age=30, s-maxage=90, no-cache",
	}
	var got bool
	b.ReportAllocs()
	for b.Loop() {
		for _, in := range inputs {
			got = hasDirective(in, "no-cache")
		}
	}
	_ = got
}

// The body is dropped and the metadata kept, as a backend evicting under pressure would: the entry is a
// miss, and the miss stores the fresh response again.
func Test_Cache_OrphanedMetadataWithoutBody(t *testing.T) {
	t.Parallel()
	for _, maxBytes := range []uint{0, 4096} {
		t.Run(fmt.Sprintf("maxbytes-%d", maxBytes), func(t *testing.T) {
			t.Parallel()
			storage := storagetest.New()
			cfg := Config{Storage: storage, Expiration: time.Hour, MaxBytes: maxBytes}
			app, _ := cacheTestAOrigin(New(cfg), "response-")

			cacheTestAExpect(t, app, "/", cacheMiss)
			bodies := bodyKeys(storage)
			require.Len(t, bodies, 1)
			storage.Mu.Lock()
			delete(storage.Data, bodies[0])
			storage.Mu.Unlock()

			resp := cacheTestAExpectStatus(t, app, "/", cacheMiss, fiber.StatusOK)
			require.Equal(t, "response-2", apptest.Body(t, resp))
			cacheTestAExpectBody(t, app, "/", cacheHit, "response-2")
		})
	}
}

func Test_Cache_BodyFetchFailure(t *testing.T) {
	t.Parallel()
	// prime returns an app whose entry for / is stored in storage, and the key of that entry.
	prime := func(t *testing.T, storage *storagetest.Memory) (*fiber.App, string) {
		t.Helper()

		app := fiber.New()
		app.Use(New(Config{Storage: storage, Expiration: time.Hour}))
		app.Get("/", func(c fiber.Ctx) error { return c.SendString("cached") })
		cacheTestAExpect(t, app, "/", cacheMiss)
		bodies := bodyKeys(storage)
		require.NotEmpty(t, bodies)
		return app, cacheKeyVersion + "|" + strings.TrimPrefix(bodies[0], bodyKeyPrefix)
	}

	t.Run("body read fails", func(t *testing.T) {
		t.Parallel()
		storage := storagetest.New()
		app, key := prime(t, storage)
		storage.SetErr(storagetest.OpGet, bodyKey(key), errors.New("boom"))
		apptest.Expect(t, app, "/", fiber.StatusInternalServerError)
	})

	t.Run("orphaned metadata delete fails", func(t *testing.T) {
		t.Parallel()
		storage := storagetest.New()
		app, key := prime(t, storage)
		storage.Mu.Lock()
		delete(storage.Data, bodyKey(key))
		storage.Mu.Unlock()
		storage.SetErr(storagetest.OpDelete, key, errors.New("boom"))
		apptest.Expect(t, app, "/", fiber.StatusInternalServerError)
	})
}

func Test_Cache_VaryManifestStoreFailureUnreservesSpace(t *testing.T) {
	t.Parallel()
	const body = "0123456789abcdef"
	storage := storagetest.New()
	app := fiber.New()
	variant := variantKey(defaultKey("/a"), "accept-language")
	maxBytes := 2 * charged(variant, len(body), textPlain, "Accept-Language")
	app.Use(New(Config{Storage: storage, Expiration: time.Hour, MaxBytes: maxBytes}))
	app.Get("/:name", func(c fiber.Ctx) error {
		c.Response().Header.Set(fiber.HeaderVary, "Accept-Language")
		return c.SendString(body)
	})

	manifestKey := manifestKeyPrefix + defaultKey("/a")
	storage.SetErr(storagetest.OpSet, manifestKey, errors.New("boom"))
	apptest.Expect(t, app, "/a", fiber.StatusInternalServerError)
	storage.SetErr(storagetest.OpSet, manifestKey, nil)

	// The abandoned reservation must be gone, or the second entry evicts the first.
	for _, want := range []string{cacheMiss, cacheHit} {
		for _, path := range []string{"/a", "/b"} {
			cacheTestAExpect(t, app, path, want)
		}
	}
}

// Test_Cache_VaryIsReplayedOnHit asserts a hit carries the stored response's
// Vary whether or not response headers are stored. Vary used to live only in
// the StoreResponseHeaders list, so by default a hit went out without it, and a
// cache in front of the app — keying on the URL alone, as it then must — could
// serve one variant to every client. Vary now joins the fields the entry always
// carries, on one line, and the stored header list no longer holds a copy.
func Test_Cache_VaryIsReplayedOnHit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		// A prefix of the Cache-Control the hit must carry, or empty when none
		// may go out.
		wantCacheControl string
		cfg              Config
	}{
		{name: "default", wantCacheControl: "public, max-age="},
		{name: "StoreResponseHeaders", cfg: Config{StoreResponseHeaders: true}, wantCacheControl: "public, max-age="},
		{name: "DisableVaryHeaders", cfg: Config{DisableVaryHeaders: true}, wantCacheControl: "public, max-age="},
		{name: "DisableCacheControl", cfg: Config{DisableCacheControl: true}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(tc.cfg))
			app.Get("/", func(c fiber.Ctx) error {
				c.Set(fiber.HeaderVary, fiber.HeaderAcceptLanguage)
				return c.SendString("page-in:" + c.Get(fiber.HeaderAcceptLanguage))
			})

			get := func(wantStatus string) *http.Response {
				t.Helper()
				resp := cacheTestAExpectBody(t, app, cacheTestAAcceptLanguage("en"), wantStatus, "page-in:en")
				// Once, not once per place it is kept: under StoreResponseHeaders
				// the stored header list must not carry a second copy.
				require.Equal(t, []string{fiber.HeaderAcceptLanguage}, resp.Header.Values(fiber.HeaderVary), "(%s)", wantStatus)
				return resp
			}

			get(cacheMiss)
			resp := get(cacheHit)

			// A Vary naming no credential keeps the public form.
			cacheControl := resp.Header.Get(fiber.HeaderCacheControl)
			if tc.wantCacheControl == "" {
				require.Empty(t, cacheControl)
			} else {
				require.True(t, strings.HasPrefix(cacheControl, tc.wantCacheControl), "Cache-Control %q", cacheControl)
			}
		})
	}
}

// Test_Cache_CredentialKeyedHitIsPrivate asserts a hit on an entry holding one
// user's response says so. Such an entry has no Cache-Control of its own in the
// common case, and the "public, max-age" this middleware then wrote told a
// shared cache in front of the app it could store the hit and serve it to the
// next client asking for the URL: keyed on the URL alone, that cache never sees
// the cookie the entry was selected by. An entry is one user's whenever the
// response's Vary lists a credential header, or the key is partitioned by
// KeyCookies, KeyCredentialHeaders, or a credential header in KeyHeaders.
func Test_Cache_CredentialKeyedHitIsPrivate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		appCfg fiber.Config
		name   string
		// A Cache-Control of the handler's own.
		cacheControl string
		// What the hit must carry: Vary with its field lines joined, and the
		// exact Cache-Control, which defaults to the private form.
		wantVary         string
		wantCacheControl string
		// The Vary field lines the handler sends, and the spelling of the field
		// name it writes them under, "Vary" when empty.
		vary     []string
		varyName string
		cfg      Config
		// Whether the request carries the session cookie.
		noCookie bool
	}{
		{name: "Vary: Cookie", vary: []string{"Cookie"}, wantVary: "Cookie"},
		{name: "Vary: cookie in lower case", vary: []string{"cookie"}, wantVary: "cookie"},
		{name: "Vary: Cookie on a second field line", vary: []string{"Accept-Encoding", "Cookie"}, wantVary: "Accept-Encoding,Cookie"},
		{name: "Vary: Authorization", vary: []string{"Authorization"}, wantVary: "Authorization"},
		{name: "Vary: Proxy-Authorization", vary: []string{"Proxy-Authorization"}, wantVary: "Proxy-Authorization"},
		{name: "Vary: X-API-Key", vary: []string{"X-API-Key"}, wantVary: "X-API-Key"},
		{name: "KeyCookies", cfg: Config{KeyCookies: []string{"session"}}},
		{name: "KeyCookies without the cookie on the request", cfg: Config{KeyCookies: []string{"session"}}, noCookie: true},
		{name: "KeyHeaders naming Cookie", cfg: Config{KeyHeaders: []string{"Cookie"}}},
		{name: "KeyHeaders naming Authorization", cfg: Config{KeyHeaders: []string{"Authorization"}}},
		{name: "KeyCredentialHeaders", cfg: Config{KeyCredentialHeaders: []string{"X-Company-Token"}}},
		{name: "KeyHeaders naming Proxy-Authorization", cfg: Config{KeyHeaders: []string{"Proxy-Authorization"}}},
		{name: "KeyHeaders naming X-API-Key", cfg: Config{KeyHeaders: []string{"X-API-Key"}}},
		{name: "KeyHeaders naming X-Auth-Token", cfg: Config{KeyHeaders: []string{"X-Auth-Token"}}},
		{name: "KeyHeaders naming X-Access-Token", cfg: Config{KeyHeaders: []string{"X-Access-Token"}}},
		// KeyHeaders naming a credential header makes every entry one client's, whatever the generator.
		{name: "custom KeyGenerator, KeyHeaders naming Cookie, without the cookie on the request", cfg: Config{KeyGenerator: pathKey, KeyHeaders: []string{"Cookie"}}, noCookie: true},
		{name: "custom KeyGenerator, KeyHeaders naming Authorization", cfg: Config{KeyGenerator: pathKey, KeyHeaders: []string{"Authorization"}}},
		{name: "DisableVaryHeaders", cfg: Config{DisableVaryHeaders: true}, vary: []string{"Cookie"}, wantVary: "Cookie"},
		{name: "DisableHeaderNormalizing", appCfg: fiber.Config{DisableHeaderNormalizing: true}, vary: []string{"cookie"}, wantVary: "cookie"},
		{name: "DisableHeaderNormalizing with the field name in lower case", appCfg: fiber.Config{DisableHeaderNormalizing: true}, vary: []string{"Cookie"}, varyName: "vary", wantVary: "Cookie"},
		{
			name:             "the handler's own Cache-Control is kept",
			vary:             []string{"Cookie"},
			cacheControl:     "max-age=60",
			wantVary:         "Cookie",
			wantCacheControl: "max-age=60",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// A fixed clock, so the delta-seconds written on the hit is the whole
			// lifetime rather than whatever the wall clock left of it.
			clock := cacheTestCClock()
			cfg := tc.cfg
			cfg.Expiration = time.Minute
			cfg.clock = clock.Now
			wantCacheControl := cmp.Or(tc.wantCacheControl, "private, max-age=60")

			app := fiber.New(tc.appCfg)
			app.Use(New(cfg))
			varyName := cmp.Or(tc.varyName, fiber.HeaderVary)
			app.Get("/me", func(c fiber.Ctx) error {
				for _, v := range tc.vary {
					c.Response().Header.Add(varyName, v)
				}
				if tc.cacheControl != "" {
					c.Set(fiber.HeaderCacheControl, tc.cacheControl)
				}
				return c.SendString("page-for:" + c.Cookies("session"))
			})

			wantBody := "page-for:alice"
			if tc.noCookie {
				wantBody = "page-for:"
			}
			get := func(wantStatus string) *http.Response {
				t.Helper()
				req := httptest.NewRequest(fiber.MethodGet, "/me", http.NoBody)
				if !tc.noCookie {
					req.Header.Set(fiber.HeaderCookie, "session=alice")
				}
				resp := cacheTestAExpectBody(t, app, req, wantStatus, wantBody)
				require.Equal(t, tc.wantVary, strings.Join(resp.Header.Values(fiber.HeaderVary), ","), "(%s)", wantStatus)
				return resp
			}

			get(cacheMiss)
			resp := get(cacheHit)
			require.Equal(t, wantCacheControl, resp.Header.Get(fiber.HeaderCacheControl))
		})
	}
}

// cacheTestALegacyItem returns the entry of a plain text answer with the body, fresh for a minute, as a previous
// version stored it.
func cacheTestALegacyItem(body string) *item {
	return &item{
		body:   []byte(body),
		ctype:  []byte(fiber.MIMETextPlainCharsetUTF8),
		status: fiber.StatusOK,
		exp:    uint64(time.Now().Add(time.Minute).Unix()),
		ttl:    uint64(time.Minute / time.Second),
	}
}

// Test_Cache_EntryStoredWithoutVaryIsNotRead covers an entry a previous version
// stored. It carried no Vary, so it would be replayed without one and, holding
// no Cache-Control of its own, under "public, max-age" — whatever cookie it was
// selected by. The key namespace is versioned so no such entry is ever read.
func Test_Cache_EntryStoredWithoutVaryIsNotRead(t *testing.T) {
	t.Parallel()
	storage := memory.New()
	app := fiber.New()
	app.Use(New(Config{
		Storage:    storage,
		Expiration: time.Minute,
		// A generator of the caller's own, as an application preserving its
		// key format across the upgrade would have.
		KeyGenerator: pathKey,
	}))
	app.Get("/me", func(c fiber.Ctx) error { return c.SendString("handler ran") })

	// Seed what the previous version wrote for a "Vary: Cookie" response to
	// alice: the manifest naming the cookie, and her entry — metadata and body —
	// under the variant key that manifest produces for her cookie.
	const legacyBase = "v3|GET|/me"
	var hdr fasthttp.RequestHeader
	hdr.Set(fiber.HeaderCookie, "session=alice")
	legacyKey := varyKey(legacyBase, []string{"cookie"}, &hdr, true)

	legacy := cacheTestALegacyItem("alice's page")
	raw, err := legacy.MarshalMsg(nil)
	require.NoError(t, err)
	require.NoError(t, storage.Set(legacyBase+"|vary", []byte("cookie"), time.Minute))
	require.NoError(t, storage.Set(legacyKey, raw, time.Minute))
	require.NoError(t, storage.Set(legacyKey+"_body", legacy.body, time.Minute))

	req := httptest.NewRequest(fiber.MethodGet, "/me", http.NoBody)
	req.Header.Set(fiber.HeaderCookie, "session=alice")
	resp := cacheTestAExpectStatus(t, app, req, cacheMiss, fiber.StatusOK)
	require.Equal(t, "handler ran", apptest.Body(t, resp), "an entry stored without its Vary must not be served")
}

// Test_Cache_VaryReplayedWithCompress asserts the pair compress writes comes
// back together: a hit that replays Content-Encoding without the
// Vary: Accept-Encoding beside it tells a cache in front of the app the encoded
// body suits every client.
func Test_Cache_VaryReplayedWithCompress(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Use(compress.New())
	app.Get("/", func(c fiber.Ctx) error { return c.SendString(strings.Repeat("compressible ", 256)) })

	for _, want := range []string{cacheMiss, cacheHit} {
		req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderAcceptEncoding, "gzip"))
		resp := cacheTestAExpect(t, app, req, want)
		require.Equal(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding), "(%s)", want)
		require.Equal(t, []string{fiber.HeaderAcceptEncoding}, resp.Header.Values(fiber.HeaderVary), "(%s)", want)
	}
}

// Test_Cache_VaryKeyedByTheHeadersTheLookupSaw stores a response that varies on a header an inner
// middleware rewrites under the value the lookup hashed. encryptcookie decrypts the cookie in place, and
// keyed by the decrypted value the page went to anyone sending that value as a plain cookie.
func Test_Cache_VaryKeyedByTheHeadersTheLookupSaw(t *testing.T) {
	t.Parallel()
	secret := encryptcookie.GenerateKey(32)
	sealed, err := encryptcookie.EncryptCookie("user", "alice", secret)
	require.NoError(t, err)

	app := fiber.New()
	app.Use(New(Config{MaxBytes: 1024}))
	app.Use(encryptcookie.New(encryptcookie.Config{Key: secret}))
	app.Get("/account", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "max-age=30")
		c.Vary(fiber.HeaderCookie)
		if c.Cookies("user") == "alice" {
			return c.SendString("alice's recovery codes")
		}
		return c.SendString("please sign in")
	})
	app.Get("/other", func(c fiber.Ctx) error { return c.SendString(strings.Repeat("x", 300)) })

	get := func(path, cookie string) reply { return cacheTestCGet(t, app, path, fiber.HeaderCookie, cookie) }

	// The second round starts after the URL's entries, and with them its Vary manifest, were evicted.
	for _, round := range []string{"first store", "after eviction"} {
		cacheTestCRequireReply(t, get("/account", "user="+sealed), cacheMiss, "alice's recovery codes", round)
		cacheTestCRequireReply(t, get("/account", "user=alice"), cacheMiss, "please sign in", round)
		cacheTestCRequireReply(t, get("/account", "user="+sealed), cacheHit, "alice's recovery codes", round)
		for i := range 8 {
			get("/other?q="+strconv.Itoa(i), "")
		}
	}
}

// Test_Cache_VaryLongerThanEntryLimitIsUncacheable pins the cap on a stored
// Vary to the limit the entry decodes it under. The encoder does not enforce
// that limit, so a longer value reached an external Storage and every later
// read of the entry failed on it, and the route answered with that error until
// the entry expired. Such a response is not stored at all, and a value exactly
// at the limit still round-trips.
func Test_Cache_VaryLongerThanEntryLimitIsUncacheable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		wantSecond string
		varyLen    int
	}{
		{name: "at the limit", varyLen: maxVaryLen, wantSecond: cacheHit},
		{name: "past the limit", varyLen: maxVaryLen + 1, wantSecond: cacheUnreachable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// One field name of the given length: fewer names than
			// maxVaryHeaders, so only the byte length is in play.
			vary := "X-" + strings.Repeat("a", tc.varyLen-2)

			app := fiber.New()
			app.Use(New(Config{Storage: memory.New(), Expiration: time.Minute}))
			app.Get("/", func(c fiber.Ctx) error {
				c.Set(fiber.HeaderVary, vary)
				return c.SendString("body")
			})

			for i, want := range []string{"", tc.wantSecond} {
				resp := apptest.Do(t, app, "/")
				require.Equal(t, fiber.StatusOK, resp.StatusCode, "request %d", i+1)
				if want != "" {
					require.Equal(t, want, resp.Header.Get("X-Cache"), "request %d", i+1)
				}
				body := apptest.Body(t, resp)
				require.Equal(t, "body", body, "request %d", i+1)
				require.Equal(t, []string{vary}, resp.Header.Values(fiber.HeaderVary), "request %d", i+1)
			}
		})
	}
}

// The other decode limits of an entry, as Test_Cache_VaryLongerThanEntryLimitIsUncacheable
// does for Vary: past one the response is not stored, at it the entry round-trips.
func Test_Cache_EntryPastDecodeLimitIsNotStored(t *testing.T) {
	t.Parallel()
	// long sets the response header name to n bytes.
	long := func(name string) func(c fiber.Ctx, n int) {
		return func(c fiber.Ctx, n int) { c.Set(name, strings.Repeat("a", n)) }
	}
	tests := []struct {
		set   func(c fiber.Ctx, n int)
		name  string
		limit int
	}{
		{name: "Content-Type", limit: maxStoredContentType, set: long(fiber.HeaderContentType)},
		{name: "Content-Encoding", limit: maxStoredContentEncoding, set: long(fiber.HeaderContentEncoding)},
		{name: "Cache-Control", limit: maxStoredCacheControl, set: func(c fiber.Ctx, n int) {
			const prefix = "max-age=60, x="
			c.Set(fiber.HeaderCacheControl, prefix+strings.Repeat("a", n-len(prefix)))
		}},
		{name: "Expires", limit: maxStoredExpires, set: func(c fiber.Ctx, n int) {
			// max-age wins, so the value is stored without being parsed.
			c.Set(fiber.HeaderCacheControl, "max-age=60")
			c.Set(fiber.HeaderExpires, strings.Repeat("a", n))
		}},
		{name: "ETag", limit: maxStoredETag, set: long(fiber.HeaderETag)},
		{name: "Location", limit: maxStoredHeaderValue, set: func(c fiber.Ctx, n int) {
			c.Status(fiber.StatusMovedPermanently).Set(fiber.HeaderLocation, strings.Repeat("a", n))
		}},
		{name: "header value", limit: maxStoredHeaderValue, set: long("X-Long")},
		{name: "header name", limit: maxStoredHeaderKey, set: func(c fiber.Ctx, n int) {
			c.Set(strings.Repeat("a", n), "v")
		}},
		{name: "header count", limit: maxStoredHeaders, set: func(c fiber.Ctx, n int) {
			have := 0
			for key := range c.Response().Header.All() {
				if !isIgnoredHeader(key) {
					have++
				}
			}
			for i := range n - have {
				c.Set("X-H-"+strconv.Itoa(i), "v")
			}
		}},
	}

	for _, tt := range tests {
		for _, over := range []int{0, 1} {
			t.Run(fmt.Sprintf("%s/%d", tt.name, tt.limit+over), func(t *testing.T) {
				t.Parallel()
				// Location is kept without StoreResponseHeaders, the rest of the list needs it.
				app := fiber.New()
				app.Use(New(Config{Storage: memory.New(), Expiration: time.Minute, StoreResponseHeaders: tt.name != "Location"}))
				app.Get("/", func(c fiber.Ctx) error {
					tt.set(c, tt.limit+over)
					return c.SendString("body")
				})

				want := []string{cacheMiss, cacheHit}
				if over > 0 {
					want = []string{cacheUnreachable, cacheUnreachable}
				}
				for i, status := range want {
					resp := apptest.Do(t, app, "/")
					require.Less(t, resp.StatusCode, fiber.StatusInternalServerError, "request %d", i+1)
					require.Equal(t, status, resp.Header.Get("X-Cache"), "request %d", i+1)
					require.NoError(t, resp.Body.Close())
				}
			})
		}
	}

	t.Run("echoed header keeps the earlier entry", func(t *testing.T) {
		t.Parallel()
		app := fiber.New(fiber.Config{ReadBufferSize: 2 * maxStoredHeaderValue})
		app.Use(New(Config{Storage: memory.New(), Expiration: time.Minute, StoreResponseHeaders: true}))
		app.Get("/", func(c fiber.Ctx) error {
			c.Set("X-Echo", c.Get("X-Echo"))
			return c.SendString("body")
		})

		get := func(headers ...string) string {
			t.Helper()
			resp := apptest.ExpectBody(t, app, cacheTestCReq("/", headers...), fiber.StatusOK, "body")
			return resp.Header.Get("X-Cache")
		}

		require.Equal(t, cacheMiss, get())
		oversized := strings.Repeat("x", maxStoredHeaderValue+1)
		require.Equal(t, cacheUnreachable, get(fiber.HeaderCacheControl, "no-cache", "X-Echo", oversized))
		require.Equal(t, cacheHit, get())
		require.Equal(t, cacheMiss, get(fiber.HeaderCacheControl, "no-cache"))
		require.Equal(t, cacheHit, get())
	})

	t.Run("in-memory store is not limited", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: time.Minute}))
		app.Get("/", func(c fiber.Ctx) error {
			c.Set(fiber.HeaderETag, strings.Repeat("a", maxStoredETag+1))
			return c.SendString("body")
		})

		for i, status := range []string{cacheMiss, cacheHit} {
			resp := apptest.Do(t, app, "/")
			require.Equal(t, fiber.StatusOK, resp.StatusCode, "request %d", i+1)
			require.Equal(t, status, resp.Header.Get("X-Cache"), "request %d", i+1)
			require.NoError(t, resp.Body.Close())
		}
	})
}

func Test_Cache_CredentialHeadersAreHashedInTheKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		header string
		cfg    Config
	}{
		{name: "declared in KeyCredentialHeaders", header: "X-Company-Token", cfg: Config{KeyCredentialHeaders: []string{"X-Company-Token"}}},
		{name: "recognized in KeyHeaders", header: "X-API-Key", cfg: Config{KeyHeaders: []string{"X-API-Key"}}},
	}

	const token = "first-company-token"
	const otherToken = "second-company-token"

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			storage := storagetest.New()
			cfg := tc.cfg
			cfg.Storage = storage
			app := fiber.New()
			app.Use(New(cfg))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("for " + c.Get(tc.header)) })

			get := func(value string) string {
				t.Helper()
				return getRoot(t, app, map[string]string{tc.header: value}).cache
			}

			// The credential still tells entries apart.
			require.Equal(t, cacheMiss, get(token))
			require.Equal(t, cacheHit, get(token))
			require.Equal(t, cacheMiss, get(otherToken))

			storage.Mu.RLock()
			defer storage.Mu.RUnlock()
			require.NotEmpty(t, storage.Data)
			for key := range storage.Data {
				require.NotContains(t, key, token)
				require.NotContains(t, key, otherToken)
			}
		})
	}
}

// With a custom KeyGenerator, KeyCredentialHeaders takes no part in the key, but a hit for a request
// that sends one of the headers is private and not served stale. One that sends none finds the shared entry.
func Test_Cache_KeyCredentialHeadersWithCustomKeyGenerator(t *testing.T) {
	t.Parallel()
	token := map[string]string{"X-Company-Token": "t"}
	tests := []struct {
		headers    map[string]string
		name       string
		keyHeaders []string
		declared   bool
		private    bool
	}{
		{name: "declared, request sends it", declared: true, headers: token, private: true},
		{name: "declared, anonymous request", declared: true},
		{name: "not declared, so the entry is shared", headers: token},
		// Named in KeyHeaders, the header is taken to be in the key, as before the option existed.
		{name: "named in KeyHeaders, anonymous request", keyHeaders: []string{"X-API-Key"}, private: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clock := cacheTestCClock()
			cfg := Config{Expiration: time.Minute, clock: clock.Now, KeyGenerator: pathKey}
			if tc.declared {
				cfg.KeyCredentialHeaders = []string{"X-Company-Token"}
			}
			cfg.KeyHeaders = tc.keyHeaders
			app, calls := cacheTestAOrigin(New(cfg), "")

			require.Equal(t, cacheMiss, getRoot(t, app, tc.headers).cache)

			hit := getRoot(t, app, tc.headers)
			require.Equal(t, cacheHit, hit.cache)
			wantCacheControl := "public, max-age=60"
			if tc.private {
				wantCacheControl = "private, max-age=60"
			}
			require.Equal(t, wantCacheControl, hit.header.Get(fiber.HeaderCacheControl))

			// Past the entry's lifetime, well inside what the storage keeps.
			clock.Add(2 * time.Minute)
			headers := map[string]string{fiber.HeaderCacheControl: "max-stale"}
			maps.Copy(headers, tc.headers)
			got := getRoot(t, app, headers)
			if tc.private {
				require.Equal(t, int32(2), calls.Load(), "an entry holding a credential's response was served stale")
				return
			}
			require.Equal(t, cacheHit, got.cache)
			require.Equal(t, int32(1), calls.Load())
		})
	}
}

func Test_configDefault_KeyCredentialHeaders(t *testing.T) {
	t.Parallel()
	token := []string{"x-company-token"}
	tests := []struct {
		name            string
		wantKeyHeaders  []string
		wantCredentials []string
		wantCarrying    []string // the KeyHeaders that carry a credential
		cfg             Config
	}{
		{name: "unset", wantKeyHeaders: []string{"accept", "accept-encoding", "accept-language"}},
		{
			name: "joins the default key headers", cfg: Config{KeyCredentialHeaders: []string{"X-Company-Token"}},
			wantKeyHeaders:  []string{"accept", "accept-encoding", "accept-language", "x-company-token"},
			wantCredentials: token, wantCarrying: token,
		},
		{
			name: "normalized, trimmed, deduplicated and merged with KeyHeaders",
			cfg: Config{
				KeyHeaders:           []string{"X-Company-Token", "Accept"},
				KeyCredentialHeaders: []string{" X-Company-Token ", "x-company-token", "", "X-Other"},
			},
			wantKeyHeaders:  []string{"accept", "x-company-token", "x-other"},
			wantCredentials: []string{"x-company-token", "x-other"},
			wantCarrying:    []string{"x-company-token", "x-other"},
		},
		{
			name:           "with header partitioning otherwise disabled",
			cfg:            Config{KeyHeaders: []string{}, KeyCredentialHeaders: []string{"X-Company-Token"}},
			wantKeyHeaders: token, wantCredentials: token, wantCarrying: token,
		},
		{
			name:           "recognized headers named in KeyHeaders carry a credential, others do not",
			cfg:            Config{KeyHeaders: []string{"X-Tenant", "x-api-key", "Accept", "Authorization"}},
			wantKeyHeaders: []string{"accept", "authorization", "x-api-key", "x-tenant"},
			wantCarrying:   []string{"authorization", "x-api-key"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// The caller's slices are theirs: normalizing must not rewrite them.
			keyHeaders := slices.Clone(tc.cfg.KeyHeaders)
			credentials := slices.Clone(tc.cfg.KeyCredentialHeaders)

			cfg := configDefault(tc.cfg)
			require.Equal(t, tc.wantKeyHeaders, cfg.KeyHeaders)
			require.Equal(t, tc.wantCredentials, cfg.KeyCredentialHeaders)
			require.Equal(t, tc.wantCarrying, cfg.credentialKeyHeaders)
			require.Equal(t, keyHeaders, tc.cfg.KeyHeaders)
			require.Equal(t, credentials, tc.cfg.KeyCredentialHeaders)
		})
	}
}

// Test_Cache_ResponseLinesUnderTrailerNames checks that a response's Cache-Control and Vary lines are honored
// although its Trailer field names them, as an upstream may send.
func Test_Cache_ResponseLinesUnderTrailerNames(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		raw := "HTTP/1.1 200 OK\r\nCache-Control: private, no-store\r\nVary: X-User\r\nTrailer: Cache-Control, Vary\r\n" +
			"Transfer-Encoding: chunked\r\n\r\n6\r\nsecret\r\n0\r\n\r\n"
		return c.Response().Read(bufio.NewReader(strings.NewReader(raw)))
	})

	for _, user := range []string{"u0", "u1"} {
		resp := cacheTestAExpect(t, app, cacheTestCReq("/", "X-User", user), cacheUnreachable)
		require.Equal(t, "private, no-store", resp.Header.Get(fiber.HeaderCacheControl))
	}
}

// An entry key ends in what the request chose, so it must never be the key of
// another entry's body slot, Vary manifest or Vary variant.
func Test_Cache_EntryKeyNeverNamesDerivedSlot(t *testing.T) {
	t.Parallel()
	byTenant := func(c fiber.Ctx) string { return c.Path() + "|" + c.Get("X-Tenant") }
	var fr fasthttp.RequestHeader
	fr.Set(fiber.HeaderAcceptLanguage, "fr")
	// The tenant that spells the key of acme's French variant.
	spelled := "acme" + string(appendVaryKey(nil, []string{"accept-language"}, &fr, true))

	// A step is a request for /news that sends the tenant and the language, when not empty, and the answer it gets.
	type step struct{ tenant, lang, cache, body string }
	for _, tc := range []struct {
		keyGenerator func(fiber.Ctx) string
		handler      fiber.Handler
		name         string
		storage      string // a member of storageKinds
		steps        []step
	}{
		{
			name: "body", storage: "external",
			handler: func(c fiber.Ctx) error { return c.SendString("news for " + c.Get(fiber.HeaderAcceptLanguage)) },
			steps: []step{
				{"", "en", cacheMiss, "news for en"},
				{"", "en_body", cacheMiss, "news for en_body"},
				{"", "en", cacheHit, "news for en"},
				{"", "en_body", cacheHit, "news for en_body"},
			},
		},
		{
			name: "manifest", storage: "external", keyGenerator: byTenant,
			handler: func(c fiber.Ctx) error { return c.SendString("news for " + c.Get("X-Tenant")) },
			steps: []step{
				{"acme", "", cacheMiss, "news for acme"},
				{"acme|vary", "", cacheMiss, "news for acme|vary"},
				{"acme", "", cacheHit, "news for acme"},
			},
		},
		{
			name: "variant", storage: "memory", keyGenerator: byTenant,
			handler: func(c fiber.Ctx) error {
				if c.Get("X-Tenant") != "acme" {
					return c.SendString("news for " + c.Get("X-Tenant"))
				}
				c.Vary(fiber.HeaderAcceptLanguage)
				return c.SendString("news for acme in " + c.Get(fiber.HeaderAcceptLanguage))
			},
			steps: []step{
				{"acme", "de", cacheMiss, "news for acme in de"},
				{spelled, "fr", cacheMiss, "news for " + spelled},
				{"acme", "fr", cacheMiss, "news for acme in fr"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(Config{Storage: storageOf(tc.storage), KeyGenerator: tc.keyGenerator}))
			app.Get("/news", tc.handler)

			for _, s := range tc.steps {
				got := cacheTestCGet(t, app, "/news", "X-Tenant", s.tenant, fiber.HeaderAcceptLanguage, s.lang)
				require.Equal(t, fiber.StatusOK, got.status, "%+v", s)
				cacheTestCRequireReply(t, got, s.cache, s.body, "%+v", s)
			}
		})
	}
}
