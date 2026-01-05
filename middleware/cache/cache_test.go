// Special thanks to @codemicro for moving this to fiber core
// Original middleware: github.com/codemicro/fiber-cache
package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type failingCacheStorage struct {
	data map[string][]byte
	errs map[string]error
}

type mutatingStorage struct {
	data   map[string][]byte
	mutate func(key string, value []byte) []byte
}

func newFailingCacheStorage() *failingCacheStorage {
	return &failingCacheStorage{
		data: make(map[string][]byte),
		errs: make(map[string]error),
	}
}

func newMutatingStorage(mutate func(key string, value []byte) []byte) *mutatingStorage {
	return &mutatingStorage{
		data:   make(map[string][]byte),
		mutate: mutate,
	}
}

func (s *mutatingStorage) GetWithContext(_ context.Context, key string) ([]byte, error) {
	return s.Get(key)
}

func (s *mutatingStorage) Get(key string) ([]byte, error) {
	if value, ok := s.data[key]; ok {
		return value, nil
	}

	return nil, nil
}

func (s *mutatingStorage) SetWithContext(_ context.Context, key string, val []byte, _ time.Duration) error {
	return s.Set(key, val, 0)
}

func (s *mutatingStorage) Set(key string, val []byte, _ time.Duration) error {
	if key == "" || len(val) == 0 {
		return nil
	}

	if s.mutate != nil {
		val = s.mutate(key, val)
	}

	s.data[key] = val
	return nil
}

func (s *mutatingStorage) DeleteWithContext(_ context.Context, key string) error {
	return s.Delete(key)
}

func (s *mutatingStorage) Delete(key string) error {
	delete(s.data, key)
	return nil
}

func (s *mutatingStorage) ResetWithContext(_ context.Context) error {
	return s.Reset()
}

func (s *mutatingStorage) Reset() error {
	s.data = make(map[string][]byte)
	return nil
}

func (s *mutatingStorage) Close() error {
	s.data = nil
	return nil
}

func (s *failingCacheStorage) GetWithContext(_ context.Context, key string) ([]byte, error) {
	if err, ok := s.errs["get|"+key]; ok && err != nil {
		return nil, err
	}
	if val, ok := s.data[key]; ok {
		return append([]byte(nil), val...), nil
	}
	return nil, nil
}

func (s *failingCacheStorage) Get(key string) ([]byte, error) {
	return s.GetWithContext(context.Background(), key)
}

func (s *failingCacheStorage) SetWithContext(_ context.Context, key string, val []byte, _ time.Duration) error {
	if err, ok := s.errs["set|"+key]; ok && err != nil {
		return err
	}
	s.data[key] = append([]byte(nil), val...)
	return nil
}

func (s *failingCacheStorage) Set(key string, val []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, exp)
}

func (s *failingCacheStorage) DeleteWithContext(_ context.Context, key string) error {
	if err, ok := s.errs["del|"+key]; ok && err != nil {
		return err
	}
	delete(s.data, key)
	return nil
}

func (s *failingCacheStorage) Delete(key string) error {
	return s.DeleteWithContext(context.Background(), key)
}

func (s *failingCacheStorage) ResetWithContext(context.Context) error {
	s.data = make(map[string][]byte)
	s.errs = make(map[string]error)
	return nil
}

func (s *failingCacheStorage) Reset() error {
	return s.ResetWithContext(context.Background())
}

func (*failingCacheStorage) Close() error { return nil }

type contextRecord struct {
	key      string
	value    string
	canceled bool
}

type contextRecorderStorage struct {
	*failingCacheStorage
	deletes []contextRecord
	gets    []contextRecord
	sets    []contextRecord
}

func newContextRecorderStorage() *contextRecorderStorage {
	return &contextRecorderStorage{failingCacheStorage: newFailingCacheStorage()}
}

func contextRecordFrom(ctx context.Context, key string) contextRecord {
	record := contextRecord{
		key:      key,
		canceled: errors.Is(ctx.Err(), context.Canceled),
	}
	if value, ok := ctx.Value(markerKey).(string); ok {
		record.value = value
	}
	return record
}

func (s *contextRecorderStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	s.gets = append(s.gets, contextRecordFrom(ctx, key))
	return s.failingCacheStorage.GetWithContext(ctx, key)
}

func (s *contextRecorderStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.sets = append(s.sets, contextRecordFrom(ctx, key))
	return s.failingCacheStorage.SetWithContext(ctx, key, val, exp)
}

func (s *contextRecorderStorage) DeleteWithContext(ctx context.Context, key string) error {
	s.deletes = append(s.deletes, contextRecordFrom(ctx, key))
	return s.failingCacheStorage.DeleteWithContext(ctx, key)
}

func (s *contextRecorderStorage) recordedGets() []contextRecord {
	out := make([]contextRecord, len(s.gets))
	copy(out, s.gets)
	return out
}

func (s *contextRecorderStorage) recordedSets() []contextRecord {
	out := make([]contextRecord, len(s.sets))
	copy(out, s.sets)
	return out
}

func (s *contextRecorderStorage) recordedDeletes() []contextRecord {
	out := make([]contextRecord, len(s.deletes))
	copy(out, s.deletes)
	return out
}

func TestCacheStorageGetError(t *testing.T) {
	t.Parallel()

	storage := newFailingCacheStorage()
	storage.errs["get|/_GET"] = errors.New("boom")

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	})

	app.Use(New(Config{Storage: storage, Expiration: time.Second}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, "cache: failed to get key")
}

func TestCacheStorageSetError(t *testing.T) {
	t.Parallel()

	storage := newFailingCacheStorage()
	storage.errs["set|/_GET_body"] = errors.New("boom")

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	})

	app.Use(New(Config{Storage: storage, Expiration: time.Second}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, "cache: failed to store raw key")
}

func TestCacheStorageDeleteError(t *testing.T) {
	t.Parallel()

	storage := newFailingCacheStorage()
	storage.errs["del|/_GET"] = errors.New("boom")

	// Use an obviously expired timestamp without relying on time-based conversions
	expired := &item{exp: 1}
	raw, err := expired.MarshalMsg(nil)
	require.NoError(t, err)

	storage.data["/_GET"] = raw

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	})

	app.Use(New(Config{Storage: storage, Expiration: time.Second}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, "cache: failed to delete expired key")
}

type contextKey string

const markerKey contextKey = "marker"

func contextWithMarker(label string) context.Context {
	return context.WithValue(context.Background(), markerKey, label)
}

func canceledContextWithMarker(label string) context.Context {
	ctx, cancel := context.WithCancel(contextWithMarker(label))
	cancel()
	return ctx
}

func TestCacheEvictionPropagatesRequestContextToDelete(t *testing.T) {
	t.Parallel()

	storage := newContextRecorderStorage()
	app := fiber.New()

	app.Use(func(c fiber.Ctx) error {
		path := c.Path()
		if path == "/first" {
			c.SetContext(contextWithMarker("first"))
		}
		if path == "/second" {
			c.SetContext(canceledContextWithMarker("evict"))
		}
		return c.Next()
	})

	app.Use(New(Config{Storage: storage, Expiration: time.Minute, MaxBytes: 5}))

	app.Get("/first", func(c fiber.Ctx) error {
		return c.SendString("aaa")
	})

	app.Get("/second", func(c fiber.Ctx) error {
		return c.SendString("bbbb")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/first", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/second", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	records := storage.recordedDeletes()
	require.Len(t, records, 2)

	var keys []string
	for _, rec := range records {
		keys = append(keys, rec.key)
		require.Equal(t, "evict", rec.value)
		require.True(t, rec.canceled)
	}

	require.ElementsMatch(t, []string{"/first_GET", "/first_GET_body"}, keys)
}

func TestCacheCleanupPropagatesRequestContextToDelete(t *testing.T) {
	t.Parallel()

	storage := newContextRecorderStorage()
	storage.errs["set|/_GET"] = errors.New("boom")

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	})

	app.Use(func(c fiber.Ctx) error {
		c.SetContext(canceledContextWithMarker("cleanup"))
		return c.Next()
	})

	app.Use(New(Config{Storage: storage, Expiration: time.Minute}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("payload")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, "cache: failed to store key")

	records := storage.recordedDeletes()
	require.Len(t, records, 1)
	require.Equal(t, "/_GET_body", records[0].key)
	require.Equal(t, "cleanup", records[0].value)
	require.True(t, records[0].canceled)
}

func TestCacheStorageOperationsObserveRequestContext(t *testing.T) {
	t.Parallel()

	storage := newContextRecorderStorage()
	app := fiber.New()

	app.Use(func(c fiber.Ctx) error {
		ctxLabel := string(c.Request().Header.Peek("X-Context"))
		if ctxLabel == "" {
			return c.Next()
		}

		canceled := string(c.Request().Header.Peek("X-Cancel")) == "true"
		if canceled {
			c.SetContext(canceledContextWithMarker(ctxLabel))
		} else {
			c.SetContext(contextWithMarker(ctxLabel))
		}
		return c.Next()
	})

	app.Use(New(Config{Storage: storage, Expiration: time.Minute}))

	app.Get("/cache", func(c fiber.Ctx) error {
		return c.SendString("payload")
	})

	firstReq := httptest.NewRequest(fiber.MethodGet, "/cache", http.NoBody)
	firstReq.Header.Set("X-Context", "store")
	firstReq.Header.Set("X-Cancel", "true")

	resp, err := app.Test(firstReq)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	secondReq := httptest.NewRequest(fiber.MethodGet, "/cache", http.NoBody)
	secondReq.Header.Set("X-Context", "fetch")
	secondReq.Header.Set("X-Cancel", "true")

	resp, err = app.Test(secondReq)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	setRecords := storage.recordedSets()
	require.Len(t, setRecords, 2)
	for _, rec := range setRecords {
		require.Contains(t, []string{"/cache_GET", "/cache_GET_body"}, rec.key)
		require.Equal(t, "store", rec.value)
		require.True(t, rec.canceled)
	}

	getRecords := storage.recordedGets()
	require.NotEmpty(t, getRecords)

	var fetchEntry, fetchBody bool
	for _, rec := range getRecords {
		if rec.value != "fetch" {
			continue
		}

		if rec.key == "/cache_GET" {
			require.True(t, rec.canceled)
			fetchEntry = true
		}
		if rec.key == "/cache_GET_body" {
			require.True(t, rec.canceled)
			fetchBody = true
		}
	}

	require.True(t, fetchEntry, "expected cached entry retrieval to observe request context")
	require.True(t, fetchBody, "expected cached body retrieval to observe request context")
}

func Test_Cache_CacheControl(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{Expiration: 10 * time.Second}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, "public, max-age=10", resp.Header.Get(fiber.HeaderCacheControl))
}

func Test_Cache_CacheControl_Disabled(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Expiration:          10 * time.Second,
		DisableCacheControl: true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Empty(t, resp.Header.Get(fiber.HeaderCacheControl))
}

func Test_Cache_Expired(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 2 * time.Second}))
	count := 0
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Sleep until the cache is expired
	time.Sleep(3 * time.Second)

	respCached, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	bodyCached, err := io.ReadAll(respCached.Body)
	require.NoError(t, err)

	if bytes.Equal(body, bodyCached) {
		t.Errorf("Cache should have expired: %s, %s", body, bodyCached)
	}

	// Next response should be also cached
	respCachedNextRound, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	bodyCachedNextRound, err := io.ReadAll(respCachedNextRound.Body)
	require.NoError(t, err)

	if !bytes.Equal(bodyCachedNextRound, bodyCached) {
		t.Errorf("Cache should not have expired: %s, %s", bodyCached, bodyCachedNextRound)
	}
}

func Test_Cache(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	count := 0
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)

	cachedReq := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	cachedResp, err := app.Test(cachedReq)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	cachedBody, err := io.ReadAll(cachedResp.Body)
	require.NoError(t, err)

	require.Equal(t, cachedBody, body)
}

// go test -run Test_Cache_WithNoCacheRequestDirective
func Test_Cache_WithNoCacheRequestDirective(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query(c, "id", "1"))
	})

	// Request id = 1
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.Equal(t, []byte("1"), body)
	// Response cached, entry id = 1

	// Request id = 2 without Cache-Control: no-cache
	cachedReq := httptest.NewRequest(fiber.MethodGet, "/?id=2", http.NoBody)
	cachedResp, err := app.Test(cachedReq)
	require.NoError(t, err)
	cachedBody, err := io.ReadAll(cachedResp.Body)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedResp.Header.Get("X-Cache"))
	require.Equal(t, []byte("1"), cachedBody)
	// Response not cached, returns cached response, entry id = 1

	// Request id = 2 with Cache-Control: no-cache
	noCacheReq := httptest.NewRequest(fiber.MethodGet, "/?id=2", http.NoBody)
	noCacheReq.Header.Set(fiber.HeaderCacheControl, noCache)
	noCacheResp, err := app.Test(noCacheReq)
	require.NoError(t, err)
	noCacheBody, err := io.ReadAll(noCacheResp.Body)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, noCacheResp.Header.Get("X-Cache"))
	require.Equal(t, []byte("2"), noCacheBody)
	// Response cached, returns updated response, entry = 2

	/* Check Test_Cache_WithETagAndNoCacheRequestDirective */
	// Request id = 2 with Cache-Control: no-cache again
	noCacheReq1 := httptest.NewRequest(fiber.MethodGet, "/?id=2", http.NoBody)
	noCacheReq1.Header.Set(fiber.HeaderCacheControl, noCache)
	noCacheResp1, err := app.Test(noCacheReq1)
	require.NoError(t, err)
	noCacheBody1, err := io.ReadAll(noCacheResp1.Body)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, noCacheResp1.Header.Get("X-Cache"))
	require.Equal(t, []byte("2"), noCacheBody1)
	// Response cached, returns updated response, entry = 2

	// Request id = 3 with Cache-Control: NO-CACHE
	noCacheReqUpper := httptest.NewRequest(fiber.MethodGet, "/?id=3", http.NoBody)
	noCacheReqUpper.Header.Set(fiber.HeaderCacheControl, "NO-CACHE")
	noCacheRespUpper, err := app.Test(noCacheReqUpper)
	require.NoError(t, err)
	noCacheBodyUpper, err := io.ReadAll(noCacheRespUpper.Body)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, noCacheRespUpper.Header.Get("X-Cache"))
	require.Equal(t, []byte("3"), noCacheBodyUpper)
	// Response cached, returns updated response, entry = 3

	// Request id = 4 with Cache-Control: my-no-cache
	invalidReq := httptest.NewRequest(fiber.MethodGet, "/?id=4", http.NoBody)
	invalidReq.Header.Set(fiber.HeaderCacheControl, "my-no-cache")
	invalidResp, err := app.Test(invalidReq)
	require.NoError(t, err)
	invalidBody, err := io.ReadAll(invalidResp.Body)
	require.NoError(t, err)
	require.Equal(t, cacheHit, invalidResp.Header.Get("X-Cache"))
	require.Equal(t, []byte("3"), invalidBody)
	// Response served from cache, existing entry = 3

	// Request id = 4 again without Cache-Control: no-cache
	cachedInvalidReq := httptest.NewRequest(fiber.MethodGet, "/?id=4", http.NoBody)
	cachedInvalidResp, err := app.Test(cachedInvalidReq)
	require.NoError(t, err)
	cachedInvalidBody, err := io.ReadAll(cachedInvalidResp.Body)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedInvalidResp.Header.Get("X-Cache"))
	require.Equal(t, []byte("3"), cachedInvalidBody)
	// Response cached, returns cached response, entry id = 3

	// Request id = 1 without Cache-Control: no-cache
	cachedReq1 := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	cachedResp1, err := app.Test(cachedReq1)
	require.NoError(t, err)
	cachedBody1, err := io.ReadAll(cachedResp1.Body)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedResp1.Header.Get("X-Cache"))
	require.Equal(t, []byte("3"), cachedBody1)
	// Response not cached, returns cached response, entry id = 3
}

// go test -run Test_Cache_WithETagAndNoCacheRequestDirective
func Test_Cache_WithETagAndNoCacheRequestDirective(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(
		etag.New(),
		New(),
	)

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query(c, "id", "1"))
	})

	// Request id = 1
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	// Response cached, entry id = 1

	// If response status 200
	etagToken := resp.Header.Get("Etag")

	// Request id = 2 with ETag but without Cache-Control: no-cache
	cachedReq := httptest.NewRequest(fiber.MethodGet, "/?id=2", http.NoBody)
	cachedReq.Header.Set(fiber.HeaderIfNoneMatch, etagToken)
	cachedResp, err := app.Test(cachedReq)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedResp.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusNotModified, cachedResp.StatusCode)
	// Response not cached, returns cached response, entry id = 1, status not modified

	// Request id = 2 with ETag and Cache-Control: no-cache
	noCacheReq := httptest.NewRequest(fiber.MethodGet, "/?id=2", http.NoBody)
	noCacheReq.Header.Set(fiber.HeaderCacheControl, noCache)
	noCacheReq.Header.Set(fiber.HeaderIfNoneMatch, etagToken)
	noCacheResp, err := app.Test(noCacheReq)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, noCacheResp.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusOK, noCacheResp.StatusCode)
	// Response cached, returns updated response, entry id = 2

	// If response status 200
	etagToken = noCacheResp.Header.Get("Etag")

	// Request id = 3 with ETag and Cache-Control: NO-CACHE
	noCacheReqUpper := httptest.NewRequest(fiber.MethodGet, "/?id=3", http.NoBody)
	noCacheReqUpper.Header.Set(fiber.HeaderCacheControl, "NO-CACHE")
	noCacheReqUpper.Header.Set(fiber.HeaderIfNoneMatch, etagToken)
	noCacheRespUpper, err := app.Test(noCacheReqUpper)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, noCacheRespUpper.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusOK, noCacheRespUpper.StatusCode)
	// Response cached, returns updated response, entry id = 3

	// Request id = 2 with ETag and Cache-Control: no-cache again
	noCacheReq1 := httptest.NewRequest(fiber.MethodGet, "/?id=2", http.NoBody)
	noCacheReq1.Header.Set(fiber.HeaderCacheControl, noCache)
	noCacheReq1.Header.Set(fiber.HeaderIfNoneMatch, etagToken)
	noCacheResp1, err := app.Test(noCacheReq1)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, noCacheResp1.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusNotModified, noCacheResp1.StatusCode)
	// Response cached, returns updated response, entry id = 2, status not modified

	// Request id = 1 without ETag and Cache-Control: no-cache
	cachedReq1 := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	cachedResp1, err := app.Test(cachedReq1)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedResp1.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusOK, cachedResp1.StatusCode)
	// Response not cached, returns cached response, entry id = 2
}

// go test -run Test_Cache_WithNoStoreRequestDirective
func Test_Cache_WithNoStoreRequestDirective(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query(c, "id", "1"))
	})

	// Request id = 2
	noStoreReq := httptest.NewRequest(fiber.MethodGet, "/?id=2", http.NoBody)
	noStoreReq.Header.Set(fiber.HeaderCacheControl, noStore)
	noStoreResp, err := app.Test(noStoreReq)
	require.NoError(t, err)
	noStoreBody, err := io.ReadAll(noStoreResp.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("2"), noStoreBody)
	// Response not cached, returns updated response

	// Request id = 3 with Cache-Control: NO-STORE
	noStoreReqUpper := httptest.NewRequest(fiber.MethodGet, "/?id=3", http.NoBody)
	noStoreReqUpper.Header.Set(fiber.HeaderCacheControl, "NO-STORE")
	noStoreRespUpper, err := app.Test(noStoreReqUpper)
	require.NoError(t, err)
	noStoreBodyUpper, err := io.ReadAll(noStoreRespUpper.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("3"), noStoreBodyUpper)
	// Response not cached, returns updated response

	// Request id = 4 with Cache-Control: my-no-store
	invalidReq := httptest.NewRequest(fiber.MethodGet, "/?id=4", http.NoBody)
	invalidReq.Header.Set(fiber.HeaderCacheControl, "my-no-store")
	invalidResp, err := app.Test(invalidReq)
	require.NoError(t, err)
	invalidBody, err := io.ReadAll(invalidResp.Body)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, invalidResp.Header.Get("X-Cache"))
	require.Equal(t, []byte("4"), invalidBody)
	// Response cached, returns updated response, entry = 4

	// Request id = 4 again without Cache-Control
	cachedInvalidReq := httptest.NewRequest(fiber.MethodGet, "/?id=4", http.NoBody)
	cachedInvalidResp, err := app.Test(cachedInvalidReq)
	require.NoError(t, err)
	cachedInvalidBody, err := io.ReadAll(cachedInvalidResp.Body)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedInvalidResp.Header.Get("X-Cache"))
	require.Equal(t, []byte("4"), cachedInvalidBody)
	// Response cached previously, served from cache
}

func Test_Cache_WithSeveralRequests(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Expiration: 10 * time.Second,
	}))

	app.Get("/:id", func(c fiber.Ctx) error {
		return c.SendString(c.Params("id"))
	})

	for range 10 {
		for i := range 10 {
			func(id int) {
				rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, fmt.Sprintf("/%d", id), http.NoBody))
				require.NoError(t, err)

				defer func(body io.ReadCloser) {
					closeErr := body.Close()
					require.NoError(t, closeErr)
				}(rsp.Body)

				idFromServ, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)

				a, err := strconv.Atoi(string(idFromServ))
				require.NoError(t, err)

				// Sometimes, the id is not equal to a
				require.Equal(t, id, a)
			}(i)
		}
	}
}

func Test_Cache_Invalid_Expiration(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	cache := New(Config{Expiration: 0 * time.Second})
	app.Use(cache)

	count := 0
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)

	cachedReq := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	cachedResp, err := app.Test(cachedReq)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	cachedBody, err := io.ReadAll(cachedResp.Body)
	require.NoError(t, err)

	require.Equal(t, cachedBody, body)
}

func Test_Cache_Get(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New())

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query[string](c, "cache"))
	})

	app.Get("/get", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query[string](c, "cache"))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/?cache=123", http.NoBody))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/?cache=12345", http.NoBody))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "12345", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/get?cache=123", http.NoBody))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/get?cache=12345", http.NoBody))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(body))
}

func Test_Cache_Post(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Methods: []string{fiber.MethodPost},
	}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query[string](c, "cache"))
	})

	app.Get("/get", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query[string](c, "cache"))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/?cache=123", http.NoBody))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/?cache=12345", http.NoBody))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/get?cache=123", http.NoBody))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/get?cache=12345", http.NoBody))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "12345", string(body))
}

func Test_Cache_NothingToCache(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{Expiration: -(time.Second * 1)}))

	count := 0
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	respCached, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	bodyCached, err := io.ReadAll(respCached.Body)
	require.NoError(t, err)

	if bytes.Equal(body, bodyCached) {
		t.Errorf("Cache should have expired: %s, %s", body, bodyCached)
	}
}

func Test_Cache_CustomNext(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Next: func(c fiber.Ctx) bool {
			return c.Response().StatusCode() != fiber.StatusOK
		},
	}))

	count := 0
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
	})

	errorCount := 0
	app.Get("/error", func(c fiber.Ctx) error {
		errorCount++
		return c.Status(fiber.StatusInternalServerError).SendString(strconv.Itoa(errorCount))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	respCached, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	bodyCached, err := io.ReadAll(respCached.Body)
	require.NoError(t, err)
	require.True(t, bytes.Equal(body, bodyCached))
	require.NotEmpty(t, respCached.Header.Get(fiber.HeaderCacheControl))

	_, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/error", http.NoBody))
	require.NoError(t, err)

	errRespCached, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/error", http.NoBody))
	require.NoError(t, err)
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

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hi")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	_, err := app.Test(req)
	require.NoError(t, err)
	require.True(t, called)
}

func Test_CustomExpiration(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	var called bool
	var newCacheTime int
	app.Use(New(Config{ExpirationGenerator: func(c fiber.Ctx, _ *Config) time.Duration {
		called = true
		var err error
		newCacheTime, err = strconv.Atoi(c.GetRespHeader("Cache-Time", "600"))
		require.NoError(t, err)
		return time.Second * time.Duration(newCacheTime)
	}}))

	count := 0
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Response().Header.Add("Cache-Time", "1")
		return c.SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, 1, newCacheTime)

	// Wait until the cache expires (timestamp tick can delay expiry detection slightly).
	expireDeadline := time.Now().Add(3 * time.Second)
	var cachedResp *http.Response
	for {
		cachedResp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		if cachedResp.Header.Get("X-Cache") != cacheHit {
			break
		}
		require.True(t, time.Now().Before(expireDeadline), "response remained cached beyond expected expiration")
		time.Sleep(50 * time.Millisecond)
	}

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	cachedBody, err := io.ReadAll(cachedResp.Body)
	require.NoError(t, err)

	if bytes.Equal(body, cachedBody) {
		t.Errorf("Cache should have expired: %s, %s", body, cachedBody)
	}

	// Next response should be cached
	cachedRespNextRound, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	cachedBodyNextRound, err := io.ReadAll(cachedRespNextRound.Body)
	require.NoError(t, err)

	if !bytes.Equal(cachedBodyNextRound, cachedBody) {
		t.Errorf("Cache should not have expired: %s, %s", cachedBodyNextRound, cachedBody)
	}
}

func Test_AdditionalE2EResponseHeaders(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		StoreResponseHeaders: true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Response().Header.Add("X-Foobar", "foobar")
		return c.SendString("hi")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, "foobar", resp.Header.Get("X-Foobar"))

	req = httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, "foobar", resp.Header.Get("X-Foobar"))
}

func Test_CacheHeader(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Expiration: 10 * time.Second,
		Next: func(c fiber.Ctx) bool {
			return c.Response().StatusCode() != fiber.StatusOK
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query[string](c, "cache"))
	})

	count := 0
	app.Get("/error", func(c fiber.Ctx) error {
		count++
		c.Response().Header.Add("Cache-Time", "1")
		return c.Status(fiber.StatusInternalServerError).SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/?cache=12345", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))

	errRespCached, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/error", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, errRespCached.Header.Get("X-Cache"))
}

func Test_Cache_WithHead(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	count := 0
	handler := func(c fiber.Ctx) error {
		count++
		c.Response().Header.Add("Cache-Time", "1")
		return c.SendString(strconv.Itoa(count))
	}

	app.RouteChain("/").Get(handler).Head(handler)

	req := httptest.NewRequest(fiber.MethodHead, "/", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	cachedReq := httptest.NewRequest(fiber.MethodHead, "/", http.NoBody)
	cachedResp, err := app.Test(cachedReq)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedResp.Header.Get("X-Cache"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	cachedBody, err := io.ReadAll(cachedResp.Body)
	require.NoError(t, err)

	require.Equal(t, cachedBody, body)
}

func Test_Cache_WithHeadThenGet(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	handler := func(c fiber.Ctx) error {
		return c.SendString(fiber.Query[string](c, "cache"))
	}
	app.RouteChain("/").Get(handler).Head(handler)

	headResp, err := app.Test(httptest.NewRequest(fiber.MethodHead, "/?cache=123", http.NoBody))
	require.NoError(t, err)
	headBody, err := io.ReadAll(headResp.Body)
	require.NoError(t, err)
	require.Empty(t, string(headBody))
	require.Equal(t, cacheMiss, headResp.Header.Get("X-Cache"))

	headResp, err = app.Test(httptest.NewRequest(fiber.MethodHead, "/?cache=123", http.NoBody))
	require.NoError(t, err)
	headBody, err = io.ReadAll(headResp.Body)
	require.NoError(t, err)
	require.Empty(t, string(headBody))
	require.Equal(t, cacheHit, headResp.Header.Get("X-Cache"))

	getResp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?cache=123", http.NoBody))
	require.NoError(t, err)
	getBody, err := io.ReadAll(getResp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(getBody))
	require.Equal(t, cacheMiss, getResp.Header.Get("X-Cache"))

	getResp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/?cache=123", http.NoBody))
	require.NoError(t, err)
	getBody, err = io.ReadAll(getResp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(getBody))
	require.Equal(t, cacheHit, getResp.Header.Get("X-Cache"))
}

func Test_CustomCacheHeader(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		CacheHeader: "Cache-Status",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("Cache-Status"))
}

func Test_CacheInvalidation(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		CacheInvalidator: func(c fiber.Ctx) bool {
			return fiber.Query[bool](c, "invalidate")
		},
	}))

	count := 0
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	respCached, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	bodyCached, err := io.ReadAll(respCached.Body)
	require.NoError(t, err)
	require.True(t, bytes.Equal(body, bodyCached))
	require.NotEmpty(t, respCached.Header.Get(fiber.HeaderCacheControl))

	respInvalidate, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?invalidate=true", http.NoBody))
	require.NoError(t, err)
	bodyInvalidate, err := io.ReadAll(respInvalidate.Body)
	require.NoError(t, err)
	require.NotEqual(t, body, bodyInvalidate)
}

func Test_CacheInvalidation_noCacheEntry(t *testing.T) {
	t.Parallel()
	t.Run("Cache Invalidator should not be called if no cache entry exist ", func(t *testing.T) {
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
		_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?invalidate=true", http.NoBody))
		require.NoError(t, err)
		require.False(t, cacheInvalidatorExecuted)
	})
}

func Test_CacheInvalidation_removeFromHeap(t *testing.T) {
	t.Parallel()
	t.Run("Invalidate and remove from the heap", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{
			CacheInvalidator: func(c fiber.Ctx) bool {
				return fiber.Query[bool](c, "invalidate")
			},
			MaxBytes: 10 * 1024 * 1024,
		}))

		count := 0
		app.Get("/", func(c fiber.Ctx) error {
			count++
			return c.SendString(strconv.Itoa(count))
		})

		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		respCached, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		bodyCached, err := io.ReadAll(respCached.Body)
		require.NoError(t, err)
		require.True(t, bytes.Equal(body, bodyCached))
		require.NotEmpty(t, respCached.Header.Get(fiber.HeaderCacheControl))

		respInvalidate, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?invalidate=true", http.NoBody))
		require.NoError(t, err)
		bodyInvalidate, err := io.ReadAll(respInvalidate.Body)
		require.NoError(t, err)
		require.NotEqual(t, body, bodyInvalidate)
	})
}

func Test_CacheStorage_CustomHeaders(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Storage:  memory.New(),
		MaxBytes: 10 * 1024 * 1024,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Response().Header.Set("Content-Type", "text/xml")
		c.Response().Header.Set("Content-Encoding", "utf8")
		return c.Send([]byte("<xml><value>Test</value></xml>"))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	respCached, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	bodyCached, err := io.ReadAll(respCached.Body)
	require.NoError(t, err)
	require.True(t, bytes.Equal(body, bodyCached))
	require.NotEmpty(t, respCached.Header.Get(fiber.HeaderCacheControl))
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

func Test_Cache_MaxBytesOrder(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		MaxBytes:            2,
		ExpirationGenerator: stableAscendingExpiration(),
	}))

	app.Get("/*", func(c fiber.Ctx) error {
		return c.SendString("1")
	})

	cases := [][]string{
		// Insert a, b into cache of size 2 bytes (responses are 1 byte)
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
	}

	for idx, tcase := range cases {
		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, tcase[0], http.NoBody))
		require.NoError(t, err)
		require.Equal(t, tcase[1], rsp.Header.Get("X-Cache"), "Case %v", idx)
	}
}

func Test_Cache_MaxBytesSizes(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		MaxBytes:            7,
		ExpirationGenerator: stableAscendingExpiration(),
	}))

	app.Get("/*", func(c fiber.Ctx) error {
		path := c.RequestCtx().URI().LastPathSegment()
		size, err := strconv.Atoi(string(path))
		require.NoError(t, err)
		return c.Send(make([]byte, size))
	})

	cases := [][]string{
		{"/1", cacheMiss},
		{"/2", cacheMiss},
		{"/3", cacheMiss},
		{"/4", cacheMiss}, // 1+2+3+4 > 7 => 1,2 are evicted now
		{"/3", cacheHit},
		{"/1", cacheMiss},
		{"/2", cacheMiss},
		{"/8", cacheUnreachable}, // too big to cache -> unreachable
	}

	for idx, tcase := range cases {
		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, tcase[0], http.NoBody))
		require.NoError(t, err)
		require.Equal(t, tcase[1], rsp.Header.Get("X-Cache"), "Case %v", idx)
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
		fiber.StatusContinue,
		fiber.StatusSwitchingProtocols,
		fiber.StatusProcessing,
		fiber.StatusEarlyHints,

		// Successful responses
		fiber.StatusCreated,
		fiber.StatusAccepted,
		fiber.StatusResetContent,
		fiber.StatusMultiStatus,
		fiber.StatusAlreadyReported,
		fiber.StatusIMUsed,

		// Redirection responses
		fiber.StatusFound,
		fiber.StatusSeeOther,
		fiber.StatusNotModified,
		fiber.StatusUseProxy,
		fiber.StatusSwitchProxy,
		fiber.StatusTemporaryRedirect,

		// Client error responses
		fiber.StatusBadRequest,
		fiber.StatusUnauthorized,
		fiber.StatusPaymentRequired,
		fiber.StatusForbidden,
		fiber.StatusNotAcceptable,
		fiber.StatusProxyAuthRequired,
		fiber.StatusRequestTimeout,
		fiber.StatusConflict,
		fiber.StatusLengthRequired,
		fiber.StatusPreconditionFailed,
		fiber.StatusRequestEntityTooLarge,
		fiber.StatusUnsupportedMediaType,
		fiber.StatusRequestedRangeNotSatisfiable,
		fiber.StatusExpectationFailed,
		fiber.StatusMisdirectedRequest,
		fiber.StatusUnprocessableEntity,
		fiber.StatusLocked,
		fiber.StatusFailedDependency,
		fiber.StatusTooEarly,
		fiber.StatusUpgradeRequired,
		fiber.StatusPreconditionRequired,
		fiber.StatusTooManyRequests,
		fiber.StatusRequestHeaderFieldsTooLarge,
		fiber.StatusTeapot,
		fiber.StatusUnavailableForLegalReasons,

		// Server error responses
		fiber.StatusInternalServerError,
		fiber.StatusBadGateway,
		fiber.StatusServiceUnavailable,
		fiber.StatusGatewayTimeout,
		fiber.StatusHTTPVersionNotSupported,
		fiber.StatusVariantAlsoNegotiates,
		fiber.StatusInsufficientStorage,
		fiber.StatusLoopDetected,
		fiber.StatusNotExtended,
		fiber.StatusNetworkAuthenticationRequired,
	}
	for _, v := range uncacheableStatusCodes {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, fmt.Sprintf("/%d", v), http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
		require.Equal(t, v, resp.StatusCode)
	}
}

func TestCacheAgeHeader(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 10 * time.Second}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, "0", resp.Header.Get(fiber.HeaderAge))

	time.Sleep(4 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, "5", resp.Header.Get(fiber.HeaderAge))

	time.Sleep(1500 * time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	require.Equal(t, "5", resp.Header.Get(fiber.HeaderAge))
}

func Test_CacheRequestMaxAgeRevalidates(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 30 * time.Second,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.Path() + "|req-max-age-zero"
		},
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "public, max-age=30")
		return c.SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1", string(body))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderCacheControl, "max-age=0")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2", string(body))
}

func Test_CacheExpiresFutureAllowsCaching(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		StoreResponseHeaders: true,
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderExpires, time.Now().Add(30*time.Second).UTC().Format(time.RFC1123))
		return c.SendString("expires" + strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "expires1", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "expires1", string(body))
}

func Test_CacheExpiresPastPreventsCaching(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderExpires, time.Now().Add(-1*time.Minute).UTC().Format(time.RFC1123))
		return c.SendString("expires" + strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "expires1", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "expires2", string(body))
}

func Test_CacheAllowsSharedCacheMustRevalidateWithAuthorization(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 30 * time.Second,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.Path() + "|must-revalidate-auth"
		},
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "must-revalidate, max-age=60")
		return c.SendString("auth" + strconv.Itoa(count))
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer token")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "auth1", string(body))

	req = httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer token")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "auth1", string(body))
}

func Test_CacheAllowsSharedCacheProxyRevalidateWithAuthorization(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 30 * time.Second,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.Path() + "|proxy-revalidate-auth"
		},
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "proxy-revalidate, max-age=60")
		return c.SendString("proxy" + strconv.Itoa(count))
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer token")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "proxy1", string(body))

	req = httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer token")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "proxy1", string(body))
}

func Test_CacheInvalidExpiresStoredAsStale(t *testing.T) {
	t.Parallel()

	storage := newFailingCacheStorage()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 30 * time.Second,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.Path() + "|invalid-expires"
		},
		Storage: storage,
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "public")
		c.Set(fiber.HeaderExpires, "invalid-date")
		return c.SendString("body" + strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "body1", string(body))

	expectedKey := "/|invalid-expires_GET"
	require.Contains(t, storage.data, expectedKey)
	require.Contains(t, storage.data, expectedKey+"_body")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "body2", string(body))
	require.Contains(t, storage.data, expectedKey)
	require.Contains(t, storage.data, expectedKey+"_body")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "body3", string(body))
	require.Contains(t, storage.data, expectedKey)
	require.Contains(t, storage.data, expectedKey+"_body")
}

func Test_CacheSMaxAgeOverridesMaxAgeWhenShorter(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "public, max-age=10, s-maxage=1")
		return c.SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1", string(body))

	time.Sleep(1700 * time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2", string(body))
}

func Test_CacheSMaxAgeOverridesMaxAgeWhenLonger(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "public, max-age=1, s-maxage=2")
		return c.SendString(strconv.Itoa(count))
	})

	for time.Now().Nanosecond() >= int(100*time.Millisecond) {
		time.Sleep(10 * time.Millisecond)
	}

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	time.Sleep(1200 * time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1", string(body))

	time.Sleep(1700 * time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2", string(body))
}

func Test_CacheOnlyIfCachedMiss(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString("ok")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderCacheControl, "only-if-cached")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusGatewayTimeout, resp.StatusCode)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	require.Equal(t, 0, count)
}

func Test_CacheOnlyIfCachedStaleNotServed(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "public, max-age=1")
		return c.SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	time.Sleep(1500 * time.Millisecond)

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderCacheControl, "only-if-cached")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusGatewayTimeout, resp.StatusCode)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	require.Equal(t, 1, count)
}

func Test_CacheMaxStaleServesStaleResponse(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "public, max-age=2")
		return c.SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))

	time.Sleep(2500 * time.Millisecond)

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderCacheControl, "max-stale=5")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equalf(t, cacheHit, resp.Header.Get("X-Cache"), "dirs=%+v Age=%s count=%d", parseRequestCacheControlString("max-stale=5"), resp.Header.Get(fiber.HeaderAge), count)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1", string(body))
	require.Equal(t, 1, count)
}

func Test_CacheMaxStaleRespectsMustRevalidate(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "public, max-age=1, must-revalidate")
		return c.SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	time.Sleep(1500 * time.Millisecond)

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderCacheControl, "max-stale=30")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2", string(body))
	require.Equal(t, 2, count)
}

func Test_CacheMaxStaleRespectsProxyRevalidateSharedAuth(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "s-maxage=1, proxy-revalidate")
		return c.SendString(strconv.Itoa(count))
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer abc")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	time.Sleep(1500 * time.Millisecond)

	req = httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer abc")
	req.Header.Set(fiber.HeaderCacheControl, "max-stale=30")

	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2", string(body))
	require.Equal(t, 2, count)
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.Equal(t, "public, max-age=5, immutable", resp.Header.Get(fiber.HeaderCacheControl))
	require.Equal(t, expires, resp.Header.Get(fiber.HeaderExpires))
	require.Equal(t, `W/"abc"`, resp.Header.Get(fiber.HeaderETag))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Equal(t, "public, max-age=5, immutable", resp.Header.Get(fiber.HeaderCacheControl))
	require.Equal(t, expires, resp.Header.Get(fiber.HeaderExpires))
	require.Equal(t, `W/"abc"`, resp.Header.Get(fiber.HeaderETag))
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

	type testCase struct {
		name             string
		cacheControl     string
		cacheHeader      string
		dateOffset       time.Duration
		expiration       time.Duration
		expectAgeAtLeast int
		expectCount      int
		originAge        int
	}

	cases := []testCase{
		{
			name:             "age derived from past date without Age header",
			dateOffset:       -1 * time.Minute,
			cacheControl:     "public, max-age=120",
			cacheHeader:      cacheHit,
			expiration:       5 * time.Minute,
			expectAgeAtLeast: 1,
			expectCount:      1,
		},
		{
			name:         "stale due to past date despite max-age",
			dateOffset:   -90 * time.Second,
			cacheControl: "public, max-age=30",
			cacheHeader:  cacheUnreachable,
			expiration:   5 * time.Minute,
			expectCount:  2,
			originAge:    90,
		},
	}

	for _, tc := range cases {
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

			_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)

			if tc.cacheHeader == cacheHit {
				time.Sleep(2 * time.Second)
			}

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, tc.cacheHeader, resp.Header.Get("X-Cache"))
			if tc.cacheHeader == cacheHit {
				ageVal, err := strconv.Atoi(resp.Header.Get(fiber.HeaderAge))
				require.NoError(t, err)
				require.GreaterOrEqual(t, ageVal, tc.expectAgeAtLeast)
				require.Equal(t, 1, count)
			} else {
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Equal(t, strconv.Itoa(tc.expectCount), string(body))
				require.Equal(t, tc.expectCount, count)
			}
		})
	}
}

func Test_CacheClampsInvalidStoredDate(t *testing.T) {
	t.Parallel()

	storage := newMutatingStorage(func(key string, val []byte) []byte {
		if strings.HasSuffix(key, "_body") {
			return val
		}

		var it item
		if _, err := it.UnmarshalMsg(val); err != nil {
			return val
		}

		it.date = uint64(math.MaxInt64) + 1024
		updated, err := it.MarshalMsg(nil)
		if err != nil {
			return val
		}

		return updated
	})

	app := fiber.New()
	app.Use(New(Config{
		Expiration: time.Minute,
		Storage:    storage,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "public, max-age=60")
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))

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

	storage := newMutatingStorage(func(key string, val []byte) []byte {
		if strings.HasSuffix(key, "_body") {
			return val
		}

		var it item
		if _, err := it.UnmarshalMsg(val); err != nil {
			return val
		}

		future := time.Now().Add(2 * time.Second).UTC()
		sec := future.Unix()
		if sec < 0 {
			sec = 0
		}

		it.date = uint64(sec) //nolint:gosec // safe: sec is clamped to non-negative range
		updated, err := it.MarshalMsg(nil)
		if err != nil {
			return val
		}

		return updated
	})

	app := fiber.New()
	app.Use(New(Config{
		Expiration: time.Minute,
		Storage:    storage,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "public, max-age=60")
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))

	parsedDate, err := http.ParseTime(resp.Header.Get(fiber.HeaderDate))
	require.NoError(t, err)
	require.False(t, parsedDate.After(time.Now()))

	ageVal, err := strconv.Atoi(resp.Header.Get(fiber.HeaderAge))
	require.NoError(t, err)
	require.GreaterOrEqual(t, ageVal, 0)
}

func Test_RequestPragmaNoCacheTriggersMiss(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: time.Minute,
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "public, max-age=60")
		return c.SendString("body" + strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "body1", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "body1", string(body))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderPragma, "no-cache")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "body2", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "body2", string(body))
}

func Test_CacheStaleResponseAddsWarning110(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 2 * time.Second,
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "public, max-age=1")
		return c.SendString("body" + strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderCacheControl, "max-stale=5")
	deadline := time.Now().Add(3 * time.Second)
	for {
		resp, err = app.Test(req)
		require.NoError(t, err)
		if resp.Header.Get("X-Cache") == cacheHit {
			ageVal, err := strconv.Atoi(resp.Header.Get(fiber.HeaderAge))
			require.NoError(t, err)
			if ageVal >= 1 {
				break
			}
		}
		require.True(t, time.Now().Before(deadline), "response did not become stale before deadline")
		time.Sleep(50 * time.Millisecond)
	}

	warnings := resp.Header.Values(fiber.HeaderWarning)
	require.NotEmpty(t, warnings)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "110") {
			found = true
			break
		}
	}
	require.True(t, found, "warning 110 not found in %v", warnings)
}

func Test_CacheHeuristicFreshnessAddsWarning113(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 2 * time.Second,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "public, max-age=60")
		return c.SendString("body")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))

	for _, w := range resp.Header.Values(fiber.HeaderWarning) {
		require.NotContains(t, w, "113", "warning 113 should not be present for explicitly fresh responses")
	}
}

func Test_CacheHeuristicFreshnessAddsWarning113AfterThreshold(t *testing.T) {
	t.Parallel()

	storage := newMutatingStorage(func(key string, val []byte) []byte {
		if strings.HasSuffix(key, "_body") {
			return val
		}

		var it item
		if _, err := it.UnmarshalMsg(val); err != nil {
			return val
		}

		oldDate := time.Now().Add(-25 * time.Hour).UTC()
		sec := oldDate.Unix()
		if sec < 0 {
			sec = 0
		}
		it.date = uint64(sec) //nolint:gosec // safe: sec is clamped to non-negative range

		future := time.Now().Add(48 * time.Hour).UTC()
		expSec := future.Unix()
		if expSec < 0 {
			expSec = 0
		}
		it.exp = uint64(expSec) //nolint:gosec // safe: expSec is clamped to non-negative range
		it.ttl = uint64((48 * time.Hour) / time.Second)

		updated, err := it.MarshalMsg(nil)
		if err != nil {
			return val
		}

		return updated
	})

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 2 * time.Second,
		Storage:    storage,
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString("body" + strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))

	warnings := resp.Header.Values(fiber.HeaderWarning)
	require.NotEmpty(t, warnings)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "113") {
			found = true
			break
		}
	}
	require.True(t, found, "warning 113 not found in %v", warnings)
}

func Test_CacheAgeHeaderIsCappedAtMaxDeltaSeconds(t *testing.T) {
	t.Parallel()

	const veryLargeAge = uint64(math.MaxInt32) + 1000
	storage := newMutatingStorage(func(key string, val []byte) []byte {
		if strings.HasSuffix(key, "_body") {
			return val
		}

		var it item
		if _, err := it.UnmarshalMsg(val); err != nil {
			return val
		}

		it.age = veryLargeAge
		updated, err := it.MarshalMsg(nil)
		if err != nil {
			return val
		}

		return updated
	})

	app := fiber.New()
	app.Use(New(Config{
		Expiration: time.Minute,
		Storage:    storage,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "public, max-age=60")
		return c.SendString("body")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))

	ageVal, err := strconv.Atoi(resp.Header.Get(fiber.HeaderAge))
	require.NoError(t, err)
	require.Equal(t, math.MaxInt32, ageVal)
}

func Test_CacheMinFreshForcesRevalidation(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "public, max-age=5")
		return c.SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1", string(body))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderCacheControl, "min-fresh=10")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equalf(t, cacheMiss, resp.Header.Get("X-Cache"), "dirs=%+v Age=%s count=%d", parseRequestCacheControlString("min-fresh=10"), resp.Header.Get(fiber.HeaderAge), count)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2", string(body))
}

func Test_CachePermanentRedirectCached(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration:           30 * time.Second,
		StoreResponseHeaders: true,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.Path() + "|status-308"
		},
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "public, max-age=30")
		c.Set(fiber.HeaderLocation, "/dest")
		return c.Status(fiber.StatusPermanentRedirect).SendString("redir" + strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusPermanentRedirect, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "redir1", string(body))
	require.Equal(t, "/dest", resp.Header.Get(fiber.HeaderLocation))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusPermanentRedirect, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "redir1", string(body))
	require.Equal(t, "/dest", resp.Header.Get(fiber.HeaderLocation))
}

func Test_CacheNoStoreDirective(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
}

func Test_CacheNoCacheDirective(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "no-cache")
		return c.SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2", string(body))
}

func Test_CacheNoCacheDirectiveOverridesExistingEntry(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var noCacheMode atomic.Bool
	app.Get("/", func(c fiber.Ctx) error {
		if noCacheMode.Load() {
			c.Set(fiber.HeaderCacheControl, "no-cache")
			return c.SendString("no-cache")
		}

		c.Set(fiber.HeaderCacheControl, "public, max-age=60")
		return c.SendString("cacheable")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "cacheable", string(body))

	noCacheMode.Store(true)
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderCacheControl, "no-cache")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "no-cache", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "no-cache", string(body))
}

func Test_CacheRespectsUpstreamAgeForFreshness(t *testing.T) {
	t.Parallel()

	t.Run("skipsCachingWhenAgeExhaustsFreshness", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			KeyGenerator: func(c fiber.Ctx) string {
				return c.Path() + "|age-exhausted"
			},
		}))

		var count int
		app.Get("/", func(c fiber.Ctx) error {
			count++
			c.Set(fiber.HeaderCacheControl, "public, max-age=2")
			c.Set(fiber.HeaderAge, "2")
			return c.SendString(strconv.Itoa(count))
		})

		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "1", string(body))

		resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
		body, err = io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "2", string(body))
	})

	t.Run("expiresAfterRemainingLifetime", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Use(New(Config{
			KeyGenerator: func(c fiber.Ctx) string {
				return c.Path() + "|age-remaining"
			},
		}))

		var count int
		app.Get("/", func(c fiber.Ctx) error {
			count++
			c.Set(fiber.HeaderCacheControl, "public, max-age=2")
			c.Set(fiber.HeaderAge, "1")
			return c.SendString(strconv.Itoa(count))
		})

		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "1", string(body))

		resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
		body, err = io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "1", string(body))

		time.Sleep(1500 * time.Millisecond)

		resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
		body, err = io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "2", string(body))
	})
}

func Test_CacheVarySeparatesVariants(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		KeyGenerator: func(c fiber.Ctx) string {
			return c.Path() + "|vary-separated"
		},
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderVary, fiber.HeaderAcceptLanguage)
		return c.SendString(c.Get(fiber.HeaderAcceptLanguage) + strconv.Itoa(count))
	})

	reqEN := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	reqEN.Header.Set(fiber.HeaderAcceptLanguage, "en")
	resp, err := app.Test(reqEN)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "en1", string(body))

	reqFR := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	reqFR.Header.Set(fiber.HeaderAcceptLanguage, "fr")
	resp, err = app.Test(reqFR)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "fr2", string(body))

	reqENRepeat := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	reqENRepeat.Header.Set(fiber.HeaderAcceptLanguage, "en")
	resp, err = app.Test(reqENRepeat)
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "en1", string(body))

	reqFRRepeat := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	reqFRRepeat.Header.Set(fiber.HeaderAcceptLanguage, "fr")
	resp, err = app.Test(reqFRRepeat)
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "fr2", string(body))
}

func Test_CacheVaryStarUncacheable(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		KeyGenerator: func(c fiber.Ctx) string {
			return c.Path() + "|vary-star"
		},
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderVary, "*")
		return c.SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2", string(body))
}

func Test_CachePrivateDirective(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "private")
		return c.SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2", string(body))
}

func Test_CachePrivateDirectiveWithAuthorization(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "private")
		return c.SendString(strconv.Itoa(count))
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer token")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1", string(body))

	req = httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer token")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2", string(body))
}

func Test_CachePrivateDirectiveInvalidatesExistingEntry(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var privateMode atomic.Bool
	app.Get("/", func(c fiber.Ctx) error {
		if privateMode.Load() {
			c.Set(fiber.HeaderCacheControl, "private")
			return c.SendString("private")
		}

		c.Set(fiber.HeaderCacheControl, "public, max-age=60")
		return c.SendString("public")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "public", string(body))

	privateMode.Store(true)
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderCacheControl, "no-cache")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "private", string(body))

	privateMode.Store(false)
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "public", string(body))
}

func Test_CacheControlNotOverwritten(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 10 * time.Second, StoreResponseHeaders: true}))
	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "private")
		return c.SendString("ok")
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, "private", resp.Header.Get(fiber.HeaderCacheControl))
}

func Test_CacheMaxAgeDirective(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 10 * time.Second}))
	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "max-age=1")
		return c.SendString("1")
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	time.Sleep(1500 * time.Millisecond)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
}

func Test_ParseMaxAge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		header string
		expect time.Duration
		ok     bool
	}{
		{"max-age=60", 60 * time.Second, true},
		{"public, max-age=86400", 86400 * time.Second, true},
		{"no-store", 0, false},
		{"max-age=invalid", 0, false},
		{"public, s-maxage=100, max-age=50", 50 * time.Second, true},
		{"MAX-AGE=20", 20 * time.Second, true},
		{"public , max-age=0", 0, true},
		{"public , max-age", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			t.Parallel()
			d, ok := parseMaxAge(tt.header)
			if tt.ok != ok {
				t.Fatalf("expected ok=%v got %v", tt.ok, ok)
			}
			if ok && d != tt.expect {
				t.Fatalf("expected %v got %v", tt.expect, d)
			}
		})
	}
}

func Test_AllowsSharedCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		directives string
		expect     bool
	}{
		{"public", true},
		{"private", false},
		{"s-maxage=60", true},
		{"public, max-age=60", true},
		{"public, must-revalidate", true},
		{"max-age=60", false},
		{"no-cache", false},
		{"no-cache, s-maxage=60", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.directives, func(t *testing.T) {
			t.Parallel()

			got := allowsSharedCache(tt.directives)
			require.Equal(t, tt.expect, got, "directives: %q", tt.directives)
		})
	}

	t.Run("private overrules public", func(t *testing.T) {
		t.Parallel()

		got := allowsSharedCache(strings.ToUpper("private, public"))
		require.False(t, got)
	})
}

func TestCacheSkipsAuthorizationByDefault(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer token")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1", string(body))

	req = httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer token")

	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2", string(body))
}

func TestCacheBypassesExistingEntryForAuthorization(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
	})

	nonAuthReq := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)

	resp, err := app.Test(nonAuthReq)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1", string(body))

	authReq := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	authReq.Header.Set(fiber.HeaderAuthorization, "Bearer token")

	resp, err = app.Test(authReq)
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2", string(body))

	resp, err = app.Test(nonAuthReq)
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1", string(body))
}

func TestCacheAllowsSharedCacheWithAuthorization(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 10 * time.Second}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Set(fiber.HeaderCacheControl, "public, max-age=60")
		return c.SendString("ok")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer token")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	req = httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer token")

	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "ok", string(body))
	require.Equal(t, 1, count)
}

func TestCacheAllowsAuthorizationWithRevalidateDirectives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		cacheControl  string
		expires       string
		expectedBody  string
		expectedBody2 string
		expectFirst   string
		expectSecond  string
	}{
		{
			name:          "must-revalidate",
			cacheControl:  "must-revalidate, max-age=60",
			expectedBody:  "ok-1",
			expectedBody2: "ok-1",
			expectFirst:   cacheMiss,
			expectSecond:  cacheHit,
		},
		{
			name:          "proxy-revalidate",
			cacheControl:  "proxy-revalidate, max-age=60",
			expectedBody:  "ok-1",
			expectedBody2: "ok-1",
			expectFirst:   cacheMiss,
			expectSecond:  cacheHit,
		},
		{
			name:          "expires header",
			cacheControl:  "",
			expires:       time.Now().Add(1 * time.Minute).UTC().Format(http.TimeFormat),
			expectedBody:  "ok-1",
			expectedBody2: "ok-2",
			expectFirst:   cacheUnreachable,
			expectSecond:  cacheUnreachable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Expiration: 10 * time.Second}))

			var count int
			app.Get("/", func(c fiber.Ctx) error {
				count++
				c.Set(fiber.HeaderCacheControl, tt.cacheControl)
				if tt.expires != "" {
					c.Set(fiber.HeaderExpires, tt.expires)
				}
				return c.SendString(fmt.Sprintf("ok-%d", count))
			})

			req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
			req.Header.Set(fiber.HeaderAuthorization, "Bearer token")

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tt.expectFirst, resp.Header.Get("X-Cache"))
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, tt.expectedBody, string(body))

			resp, err = app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tt.expectSecond, resp.Header.Get("X-Cache"))
			body, err = io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, tt.expectedBody2, string(body))

			if tt.expectSecond == cacheHit {
				require.Equal(t, 1, count)
			} else {
				require.Equal(t, 2, count)
			}
		})
	}
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
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
		return req
	}

	authTokenA := "token-a"
	authTokenB := "token-b"

	resp, err := app.Test(newRequest(authTokenA))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "body-1-Bearer "+authTokenA, string(body))

	resp, err = app.Test(newRequest(authTokenA))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "body-1-Bearer "+authTokenA, string(body))
	require.Equal(t, 1, count)

	resp, err = app.Test(newRequest(authTokenB))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "body-2-Bearer "+authTokenB, string(body))
	require.Equal(t, 2, count)

	resp, err = app.Test(newRequest(authTokenB))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "body-2-Bearer "+authTokenB, string(body))

	resp, err = app.Test(newRequest(authTokenA))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "body-1-Bearer "+authTokenA, string(body))
	require.Equal(t, 2, count)
}

// go test -v -run=^$ -bench=Benchmark_Cache -benchmem -count=4
func Benchmark_Cache(b *testing.B) {
	app := fiber.New()

	app.Use(New())

	app.Get("/demo", func(c fiber.Ctx) error {
		data, _ := os.ReadFile("../../.github/README.md") //nolint:errcheck // We're inside a benchmark
		return c.Status(fiber.StatusTeapot).Send(data)
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/demo")

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
	require.Greater(b, len(fctx.Response.Body()), 30000)
}

func Benchmark_Cache_Miss(b *testing.B) {
	app := fiber.New()

	app.Use(New())

	app.Get("/*", func(c fiber.Ctx) error {
		data, _ := os.ReadFile("../../.github/README.md") //nolint:errcheck // We're inside a benchmark
		return c.Status(fiber.StatusOK).Send(data)
	})

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

// go test -v -run=^$ -bench=Benchmark_Cache_Storage -benchmem -count=4
func Benchmark_Cache_Storage(b *testing.B) {
	app := fiber.New()

	app.Use(New(Config{
		Storage: memory.New(),
	}))

	app.Get("/demo", func(c fiber.Ctx) error {
		data, _ := os.ReadFile("../../.github/README.md") //nolint:errcheck // We're inside a benchmark
		return c.Status(fiber.StatusTeapot).Send(data)
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/demo")

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
	require.Greater(b, len(fctx.Response.Body()), 30000)
}

func Benchmark_Cache_AdditionalHeaders(b *testing.B) {
	app := fiber.New()
	app.Use(New(Config{
		StoreResponseHeaders: true,
	}))

	app.Get("/demo", func(c fiber.Ctx) error {
		c.Response().Header.Add("X-Foobar", "foobar")
		return c.SendStatus(418)
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/demo")

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
	require.Equal(b, []byte("foobar"), fctx.Response.Header.Peek("X-Foobar"))
}

func Benchmark_Cache_MaxSize(b *testing.B) {
	// The benchmark is run with three different MaxSize parameters
	// 1) 0:        Tracking is disabled = no overhead
	// 2) MaxInt32: Enough to store all entries = no removals
	// 3) 100:      Small size = constant insertions and removals
	cases := []uint{0, math.MaxUint32, 100}
	names := []string{"Disabled", "Unlim", "LowBounded"}
	for i, size := range cases {
		b.Run(names[i], func(b *testing.B) {
			app := fiber.New()
			app.Use(New(Config{MaxBytes: size}))

			app.Get("/*", func(c fiber.Ctx) error {
				return c.Status(fiber.StatusTeapot).SendString("1")
			})

			h := app.Handler()
			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(fiber.MethodGet)

			b.ReportAllocs()

			n := 0
			for b.Loop() {
				n++
				fctx.Request.SetRequestURI(fmt.Sprintf("/%v", n))
				h(fctx)
			}

			require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
		})
	}
}

func Test_Cache_RevalidationWithMaxBytes(t *testing.T) {
	t.Parallel()

	t.Run("max-age=0 revalidation removes old entry on storage success", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()

		app.Use(New(Config{
			MaxBytes: 100,
		}))

		requestCount := 0
		app.Get("/test", func(c fiber.Ctx) error {
			requestCount++
			c.Set(fiber.HeaderCacheControl, "max-age=60")
			return c.SendString(fmt.Sprintf("response-%d", requestCount))
		})

		// First request - cache the response
		req1 := httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
		resp1, err := app.Test(req1)
		require.NoError(t, err)
		require.Equal(t, cacheMiss, resp1.Header.Get("X-Cache"))

		// Request with max-age=0 to force revalidation
		req2 := httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
		req2.Header.Set(fiber.HeaderCacheControl, "max-age=0")
		resp2, err := app.Test(req2)
		require.NoError(t, err)
		body2, err := io.ReadAll(resp2.Body)
		require.NoError(t, err)
		require.Equal(t, "response-2", string(body2))

		// Next request should serve the NEW cached entry
		req3 := httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
		resp3, err := app.Test(req3)
		require.NoError(t, err)
		require.Equal(t, cacheHit, resp3.Header.Get("X-Cache"))
		body3, err := io.ReadAll(resp3.Body)
		require.NoError(t, err)
		require.Equal(t, "response-2", string(body3), "New entry should be cached")
	})

	t.Run("min-fresh revalidation with MaxBytes", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()

		app.Use(New(Config{
			MaxBytes: 100,
		}))

		requestCount := 0
		app.Get("/test", func(c fiber.Ctx) error {
			requestCount++
			c.Set(fiber.HeaderCacheControl, "max-age=2")
			return c.SendString(fmt.Sprintf("response-%d", requestCount))
		})

		// First request - cache the response
		req1 := httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
		resp1, err := app.Test(req1)
		require.NoError(t, err)
		require.Equal(t, cacheMiss, resp1.Header.Get("X-Cache"))

		// Wait a bit so the entry has aged
		time.Sleep(1 * time.Second)

		// Request with min-fresh that exceeds remaining freshness
		req2 := httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
		req2.Header.Set(fiber.HeaderCacheControl, "min-fresh=5")
		resp2, err := app.Test(req2)
		require.NoError(t, err)
		body2, err := io.ReadAll(resp2.Body)
		require.NoError(t, err)
		require.Equal(t, "response-2", string(body2))

		// Next request should serve the NEW cached entry
		req3 := httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
		resp3, err := app.Test(req3)
		require.NoError(t, err)
		require.Equal(t, cacheHit, resp3.Header.Get("X-Cache"))
		body3, err := io.ReadAll(resp3.Body)
		require.NoError(t, err)
		require.Equal(t, "response-2", string(body3))
	})

	t.Run("revalidation respects MaxBytes eviction", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()

		app.Use(New(Config{
			MaxBytes:            20, // Only room for 2 responses of 10 bytes each
			ExpirationGenerator: stableAscendingExpiration(),
		}))

		app.Get("/*", func(c fiber.Ctx) error {
			c.Set(fiber.HeaderCacheControl, "max-age=60")
			return c.SendString("1234567890") // 10 bytes
		})

		// Cache /a and /b
		req1 := httptest.NewRequest(fiber.MethodGet, "/a", http.NoBody)
		resp1, err := app.Test(req1)
		require.NoError(t, err)
		require.Equal(t, cacheMiss, resp1.Header.Get("X-Cache"))

		req2 := httptest.NewRequest(fiber.MethodGet, "/b", http.NoBody)
		resp2, err := app.Test(req2)
		require.NoError(t, err)
		require.Equal(t, cacheMiss, resp2.Header.Get("X-Cache"))

		// Both should be cached
		req3 := httptest.NewRequest(fiber.MethodGet, "/a", http.NoBody)
		resp3, err := app.Test(req3)
		require.NoError(t, err)
		require.Equal(t, cacheHit, resp3.Header.Get("X-Cache"))

		req4 := httptest.NewRequest(fiber.MethodGet, "/b", http.NoBody)
		resp4, err := app.Test(req4)
		require.NoError(t, err)
		require.Equal(t, cacheHit, resp4.Header.Get("X-Cache"))

		// Revalidate /a with max-age=0
		req5 := httptest.NewRequest(fiber.MethodGet, "/a", http.NoBody)
		req5.Header.Set(fiber.HeaderCacheControl, "max-age=0")
		_, err = app.Test(req5)
		require.NoError(t, err)

		// /a should be revalidated and cached again
		req6 := httptest.NewRequest(fiber.MethodGet, "/a", http.NoBody)
		resp6, err := app.Test(req6)
		require.NoError(t, err)
		require.Equal(t, cacheHit, resp6.Header.Get("X-Cache"))

		// /b should still be cached (heap accounting should be correct)
		req7 := httptest.NewRequest(fiber.MethodGet, "/b", http.NoBody)
		resp7, err := app.Test(req7)
		require.NoError(t, err)
		require.Equal(t, cacheHit, resp7.Header.Get("X-Cache"))
	})

	t.Run("revalidation with non-cacheable response preserves old entry", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()

		app.Use(New(Config{
			MaxBytes: 100,
		}))

		requestCount := 0
		app.Get("/test", func(c fiber.Ctx) error {
			requestCount++
			if requestCount == 1 {
				c.Set(fiber.HeaderCacheControl, "max-age=60")
				return c.SendString("cacheable")
			}
			// Second request returns no-store
			c.Set(fiber.HeaderCacheControl, "no-store")
			return c.SendString("not-cacheable")
		})

		// First request - cache the response
		req1 := httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
		resp1, err := app.Test(req1)
		require.NoError(t, err)
		require.Equal(t, cacheMiss, resp1.Header.Get("X-Cache"))
		body1, err := io.ReadAll(resp1.Body)
		require.NoError(t, err)
		require.Equal(t, "cacheable", string(body1))

		// Request with max-age=0 to force revalidation
		// The new response will be no-store (not cacheable)
		req2 := httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
		req2.Header.Set(fiber.HeaderCacheControl, "max-age=0")
		resp2, err := app.Test(req2)
		require.NoError(t, err)
		body2, err := io.ReadAll(resp2.Body)
		require.NoError(t, err)
		require.Equal(t, "not-cacheable", string(body2))

		// Next request should still serve the OLD cached entry
		// because the new response was not cacheable and old entry should remain tracked
		req3 := httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
		resp3, err := app.Test(req3)
		require.NoError(t, err)
		require.Equal(t, cacheHit, resp3.Header.Get("X-Cache"))
		body3, err := io.ReadAll(resp3.Body)
		require.NoError(t, err)
		require.Equal(t, "cacheable", string(body3), "Old entry should still be cached")
	})
}

// Test_parseCacheControlDirectives_QuotedStrings tests RFC 9111 Section 5.2 compliance
// for quoted-string values in Cache-Control directives
func Test_parseCacheControlDirectives_QuotedStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected map[string]string
		input    string
	}{
		{
			name:  "simple quoted value",
			input: `community="UCI"`,
			expected: map[string]string{
				"community": "UCI",
			},
		},
		{
			name:  "multiple directives with quoted values",
			input: `max-age=3600, community="UCI", custom="value"`,
			expected: map[string]string{
				"max-age":   "3600",
				"community": "UCI",
				"custom":    "value",
			},
		},
		{
			name:  "quoted value with spaces",
			input: `custom="value with spaces"`,
			expected: map[string]string{
				"custom": "value with spaces",
			},
		},
		{
			name:  "quoted value with escaped quote",
			input: `custom="value with \"quotes\""`,
			expected: map[string]string{
				"custom": `value with "quotes"`,
			},
		},
		{
			name:  "quoted value with escaped backslash",
			input: `custom="value with \\ backslash"`,
			expected: map[string]string{
				"custom": `value with \ backslash`,
			},
		},
		{
			name:  "mixed quoted and unquoted values",
			input: `max-age=3600, community="UCI", no-cache, custom="test"`,
			expected: map[string]string{
				"max-age":   "3600",
				"community": "UCI",
				"no-cache":  "",
				"custom":    "test",
			},
		},
		{
			name:  "quoted empty value",
			input: `custom=""`,
			expected: map[string]string{
				"custom": "",
			},
		},
		{
			name:  "spaces around quoted value",
			input: `custom = "value" , another="test"`,
			expected: map[string]string{
				"custom":  "value",
				"another": "test",
			},
		},
		{
			name:  "unquoted token value",
			input: `max-age=3600`,
			expected: map[string]string{
				"max-age": "3600",
			},
		},
		{
			name:  "complex mixed case",
			input: `max-age=3600, s-maxage=7200, community="UCI", no-store, custom="value with \"escaped\" quotes"`,
			expected: map[string]string{
				"max-age":   "3600",
				"s-maxage":  "7200",
				"community": "UCI",
				"no-store":  "",
				"custom":    `value with "escaped" quotes`,
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
			require.Equal(t, tt.expected, result)
		})
	}
}

// Test_unquoteCacheDirective tests the unquoting logic for quoted-string values
func Test_unquoteCacheDirective(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "simple quoted string",
			input:    []byte(`"value"`),
			expected: []byte("value"),
		},
		{
			name:     "empty quoted string",
			input:    []byte(`""`),
			expected: []byte(""),
		},
		{
			name:     "quoted string with spaces",
			input:    []byte(`"value with spaces"`),
			expected: []byte("value with spaces"),
		},
		{
			name:     "quoted string with escaped quote",
			input:    []byte(`"value with \"quote\""`),
			expected: []byte(`value with "quote"`),
		},
		{
			name:     "quoted string with escaped backslash",
			input:    []byte(`"value with \\ backslash"`),
			expected: []byte(`value with \ backslash`),
		},
		{
			name:     "quoted string with multiple escapes",
			input:    []byte(`"a\"b\\c\"d"`),
			expected: []byte(`a"b\c"d`),
		},
		{
			name:     "too short input",
			input:    []byte(`"`),
			expected: []byte(`"`),
		},
		{
			name:     "empty input",
			input:    []byte(``),
			expected: []byte(``),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := unquoteCacheDirective(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// Test_Cache_MaxBytes_InsufficientSpace tests the "insufficient space" error path
// when an entry is larger than MaxBytes (addresses review comment 2659976215)
func Test_Cache_MaxBytes_InsufficientSpace(t *testing.T) {
	t.Parallel()

	t.Run("entry larger than MaxBytes with empty cache", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()

		app.Use(New(Config{
			MaxBytes:   10, // Very small cache
			Expiration: 1 * time.Hour,
		}))

		app.Get("/large", func(c fiber.Ctx) error {
			// Return data larger than MaxBytes
			return c.Send(make([]byte, 20))
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/large", http.NoBody))
		require.NoError(t, err)
		// Should be unreachable because entry is too large
		require.Equal(t, cacheUnreachable, rsp.Header.Get("X-Cache"))
	})

	t.Run("entry larger than MaxBytes after eviction", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()

		app.Use(New(Config{
			MaxBytes:            15,
			ExpirationGenerator: stableAscendingExpiration(),
		}))

		app.Get("/*", func(c fiber.Ctx) error {
			path := c.Path()
			if path == "/small" {
				return c.Send(make([]byte, 5))
			}
			return c.Send(make([]byte, 20))
		})

		// Cache a small entry first
		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/small", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))

		// Try to cache a large entry - should return unreachable since it won't fit even after eviction
		rsp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/large", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheUnreachable, rsp.Header.Get("X-Cache"))
	})
}

// Test_Cache_HelperFunctions tests various helper functions for better coverage
func Test_Cache_HelperFunctions(t *testing.T) {
	t.Parallel()

	t.Run("parseHTTPDate empty", func(t *testing.T) {
		t.Parallel()
		result, ok := parseHTTPDate([]byte{})
		require.False(t, ok)
		require.Equal(t, uint64(0), result)
	})

	t.Run("parseHTTPDate invalid", func(t *testing.T) {
		t.Parallel()
		result, ok := parseHTTPDate([]byte("invalid"))
		require.False(t, ok)
		require.Equal(t, uint64(0), result)
	})

	t.Run("parseHTTPDate valid", func(t *testing.T) {
		t.Parallel()
		result, ok := parseHTTPDate([]byte("Mon, 02 Jan 2006 15:04:05 GMT"))
		require.True(t, ok)
		require.Greater(t, result, uint64(0))
	})

	t.Run("safeUnixSeconds negative", func(t *testing.T) {
		t.Parallel()
		result := safeUnixSeconds(time.Unix(-1, 0))
		require.Equal(t, uint64(0), result)
	})

	t.Run("safeUnixSeconds positive", func(t *testing.T) {
		t.Parallel()
		result := safeUnixSeconds(time.Unix(1234567890, 0))
		require.Equal(t, uint64(1234567890), result)
	})

	t.Run("remainingFreshness nil", func(t *testing.T) {
		t.Parallel()
		result := remainingFreshness(nil, 100)
		require.Equal(t, uint64(0), result)
	})

	t.Run("remainingFreshness zero exp", func(t *testing.T) {
		t.Parallel()
		e := &item{exp: 0}
		result := remainingFreshness(e, 100)
		require.Equal(t, uint64(0), result)
	})

	t.Run("remainingFreshness expired", func(t *testing.T) {
		t.Parallel()
		e := &item{exp: 100}
		result := remainingFreshness(e, 200)
		require.Equal(t, uint64(0), result)
	})

	t.Run("remainingFreshness valid", func(t *testing.T) {
		t.Parallel()
		e := &item{exp: 200}
		result := remainingFreshness(e, 100)
		require.Equal(t, uint64(100), result)
	})

	t.Run("lookupCachedHeader not found", func(t *testing.T) {
		t.Parallel()
		headers := []cachedHeader{{key: []byte("Content-Type"), value: []byte("text/html")}}
		value, found := lookupCachedHeader(headers, "Authorization")
		require.False(t, found)
		require.Nil(t, value)
	})

	t.Run("lookupCachedHeader case insensitive", func(t *testing.T) {
		t.Parallel()
		headers := []cachedHeader{{key: []byte("Authorization"), value: []byte("Bearer token")}}
		value, found := lookupCachedHeader(headers, "authorization")
		require.True(t, found)
		require.Equal(t, []byte("Bearer token"), value)
	})

	t.Run("secondsToDuration zero", func(t *testing.T) {
		t.Parallel()
		result := secondsToDuration(0)
		require.Equal(t, time.Duration(0), result)
	})

	t.Run("secondsToDuration large", func(t *testing.T) {
		t.Parallel()
		result := secondsToDuration(9223372036)
		require.Greater(t, result, time.Duration(0))
	})

	t.Run("secondsToTime zero", func(t *testing.T) {
		t.Parallel()
		result := secondsToTime(0)
		require.Equal(t, time.Unix(0, 0).UTC(), result)
	})

	t.Run("secondsToTime value", func(t *testing.T) {
		t.Parallel()
		result := secondsToTime(1234567890)
		require.Equal(t, time.Unix(1234567890, 0).UTC(), result)
	})

	t.Run("isHeuristicFreshness short age", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{Expiration: 1 * time.Hour}
		e := &item{cacheControl: []byte("public")}
		result := isHeuristicFreshness(e, cfg, 3600)
		require.False(t, result)
	})

	t.Run("isHeuristicFreshness with expires", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{Expiration: 1 * time.Hour}
		e := &item{cacheControl: []byte("public"), expires: []byte("Wed, 21 Oct 2015 07:28:00 GMT")}
		result := isHeuristicFreshness(e, cfg, uint64(25*time.Hour/time.Second))
		require.False(t, result)
	})

	t.Run("isHeuristicFreshness true", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{Expiration: 1 * time.Hour}
		e := &item{cacheControl: []byte("public")}
		result := isHeuristicFreshness(e, cfg, uint64(25*time.Hour/time.Second))
		require.True(t, result)
	})

	t.Run("cacheBodyFetchError miss", func(t *testing.T) {
		t.Parallel()
		mask := func(s string) string { return "***" }
		err := cacheBodyFetchError(mask, "key", errCacheMiss)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no cached body")
	})

	t.Run("cacheBodyFetchError other", func(t *testing.T) {
		t.Parallel()
		mask := func(s string) string { return "***" }
		originalErr := errors.New("storage error")
		err := cacheBodyFetchError(mask, "key", originalErr)
		require.Equal(t, originalErr, err)
	})
}

// Test_Cache_VaryAndAuth tests vary and auth functionality
func Test_Cache_VaryAndAuth(t *testing.T) {
	t.Parallel()

	t.Run("storeVaryManifest failure", func(t *testing.T) {
		t.Parallel()
		storage := newFailingCacheStorage()
		storage.errs["set|manifest"] = errors.New("storage fail")
		manager := &manager{storage: storage}
		err := storeVaryManifest(nil, manager, "manifest", []string{"Accept"}, 3600*time.Second)
		require.Error(t, err)
	})

	t.Run("loadVaryManifest not found", func(t *testing.T) {
		t.Parallel()
		storage := newFailingCacheStorage()
		manager := &manager{storage: storage}
		varyNames, found, err := loadVaryManifest(nil, manager, "nonexistent")
		require.NoError(t, err)
		require.False(t, found)
		require.Nil(t, varyNames)
	})

	t.Run("vary with multiple headers", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Vary", "Accept, Accept-Encoding")
			c.Response().Header.Set("Cache-Control", "max-age=3600")
			return c.SendString("test")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")
		rsp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))

		req2 := httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
		req2.Header.Set("Accept", "application/json")
		req2.Header.Set("Accept-Encoding", "gzip")
		rsp2, err := app.Test(req2)
		require.NoError(t, err)
		require.Equal(t, cacheHit, rsp2.Header.Get("X-Cache"))
	})

	t.Run("auth with must-revalidate", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "must-revalidate, max-age=3600")
			return c.SendString("content")
		})

		req := httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
		req.Header.Set("Authorization", "Bearer token1")
		rsp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))

		req2 := httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
		req2.Header.Set("Authorization", "Bearer token1")
		rsp2, err := app.Test(req2)
		require.NoError(t, err)
		require.Equal(t, cacheHit, rsp2.Header.Get("X-Cache"))
	})
}

// Test_Cache_DateAndCacheControl tests date parsing and cache control
func Test_Cache_DateAndCacheControl(t *testing.T) {
	t.Parallel()

	t.Run("date header parsing", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Date", "Mon, 02 Jan 2006 15:04:05 GMT")
			c.Response().Header.Set("Cache-Control", "max-age=3600")
			return c.SendString("test")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))
	})

	t.Run("invalid date header", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Date", "invalid")
			c.Response().Header.Set("Cache-Control", "max-age=3600")
			return c.SendString("test")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))
	})

	t.Run("cache control with quoted values", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", `max-age=3600, ext="value, with, commas"`)
			return c.SendString("test")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))
	})

	t.Run("cache control with spaces", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "max-age=3600  ,  public  ,  must-revalidate")
			return c.SendString("test")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))
	})
}

// Test_Cache_CacheControlCombinations tests common cache control directive combinations
func Test_Cache_CacheControlCombinations(t *testing.T) {
	t.Parallel()

	t.Run("max-age with public", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "public, max-age=3600")
			return c.SendString("public content")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))

		rsp2, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheHit, rsp2.Header.Get("X-Cache"))
	})

	t.Run("max-age with private", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "private, max-age=3600")
			return c.SendString("private content")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheUnreachable, rsp.Header.Get("X-Cache"))
	})

	t.Run("s-maxage overrides max-age", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "public, max-age=60, s-maxage=3600")
			return c.SendString("content")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))

		rsp2, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheHit, rsp2.Header.Get("X-Cache"))
	})

	t.Run("no-store prevents caching", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "no-store")
			return c.SendString("no store content")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheUnreachable, rsp.Header.Get("X-Cache"))

		rsp2, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheUnreachable, rsp2.Header.Get("X-Cache"))
	})

	t.Run("no-cache with etag", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "no-cache")
			c.Response().Header.Set("ETag", `"123456"`)
			return c.SendString("no-cache content")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheUnreachable, rsp.Header.Get("X-Cache"))
	})

	t.Run("must-revalidate with max-age", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "must-revalidate, max-age=3600")
			return c.SendString("must revalidate content")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))

		rsp2, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheHit, rsp2.Header.Get("X-Cache"))
	})

	t.Run("proxy-revalidate with max-age", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "public, proxy-revalidate, max-age=3600")
			return c.SendString("proxy revalidate content")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))

		rsp2, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheHit, rsp2.Header.Get("X-Cache"))
	})

	t.Run("immutable with max-age", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "public, max-age=31536000, immutable")
			return c.SendString("immutable content")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))

		rsp2, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheHit, rsp2.Header.Get("X-Cache"))
	})

	t.Run("max-age=0 with must-revalidate", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "max-age=0, must-revalidate")
			return c.SendString("always revalidate")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheUnreachable, rsp.Header.Get("X-Cache"))
	})

	t.Run("public with no explicit max-age", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "public")
			return c.SendString("public no max-age")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))

		rsp2, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheHit, rsp2.Header.Get("X-Cache"))
	})

	t.Run("multiple cache directives with extensions", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", `public, max-age=3600, custom="value"`)
			return c.SendString("content")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))

		rsp2, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheHit, rsp2.Header.Get("X-Cache"))
	})

	t.Run("private overrides public", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "public, private, max-age=3600")
			return c.SendString("conflicting directives")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheUnreachable, rsp.Header.Get("X-Cache"))
	})

	t.Run("stale-while-revalidate with max-age", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "max-age=60, stale-while-revalidate=120")
			return c.SendString("stale while revalidate")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))

		rsp2, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheHit, rsp2.Header.Get("X-Cache"))
	})

	t.Run("stale-if-error with max-age", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: 1 * time.Hour}))
		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "max-age=60, stale-if-error=3600")
			return c.SendString("stale if error")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))

		rsp2, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheHit, rsp2.Header.Get("X-Cache"))
	})
}
