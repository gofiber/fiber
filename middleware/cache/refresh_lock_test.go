package cache

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

type pausedRestoreStorage struct {
	*failingCacheStorage
	started chan struct{}
	resume  chan struct{}
	key     string
	armed   atomic.Bool
}

func (s *pausedRestoreStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	if key == s.key && s.armed.CompareAndSwap(true, false) {
		close(s.started)
		<-s.resume
	}
	return s.failingCacheStorage.GetWithContext(ctx, key)
}

func Test_CacheStorage_SlowRestoreDoesNotBlockUnrelatedHit(t *testing.T) {
	t.Parallel()

	storage := &pausedRestoreStorage{
		failingCacheStorage: newFailingCacheStorage(),
		key:                 cacheKeyVersion + "|GET|/old",
		started:             make(chan struct{}),
		resume:              make(chan struct{}),
	}
	probe := &accountingProbe{}
	app := fiber.New()
	app.Use(New(Config{
		Expiration: time.Hour,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.Path()
		},
		MaxBytes:   10,
		Storage:    storage,
		accounting: probe.record,
	}))
	app.Get("/:name", func(c fiber.Ctx) error {
		switch c.Path() {
		case "/old":
			c.Set(fiber.HeaderCacheControl, "public, max-age=30")
			return c.SendString("stale")
		case "/hot":
			c.Set(fiber.HeaderCacheControl, "public, max-age=60")
			return c.SendString("hot")
		default:
			c.Set(fiber.HeaderCacheControl, "public, max-age=60")
			return c.SendString("fresh")
		}
	})

	for _, path := range []string{"/old", "/hot"} {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
		require.NoError(t, resp.Body.Close())
	}
	require.Equal(t, uint(8), probe.counted())
	require.Equal(t, 2, probe.nodes())

	storage.mu.Lock()
	storage.errs["del|"+storage.key] = errors.New("eviction delete failed")
	storage.mu.Unlock()
	storage.armed.Store(true)

	evictionDone := make(chan error, 1)
	go func() {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/new", http.NoBody))
		if err == nil {
			if resp.StatusCode != fiber.StatusInternalServerError {
				err = errors.New("eviction did not fail")
			}
			if closeErr := resp.Body.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}
		evictionDone <- err
	}()

	var resumeOnce sync.Once
	release := func() { resumeOnce.Do(func() { close(storage.resume) }) }
	defer release()

	select {
	case <-storage.started:
	case <-time.After(2 * time.Second):
		t.Fatal("eviction did not reach restore read")
	}

	hitDone := make(chan error, 1)
	go func() {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/hot", http.NoBody))
		if err == nil {
			if resp.Header.Get("X-Cache") != cacheHit {
				err = errors.New("unrelated cache entry was not a hit")
			}
			if closeErr := resp.Body.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}
		hitDone <- err
	}()

	select {
	case err := <-hitDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("unrelated cache hit blocked on restore storage I/O")
	}
	require.Equal(t, uint(8), probe.counted(), "pending restore must retain its byte reservation")
	require.Equal(t, 2, probe.nodes())

	release()
	select {
	case evictionErr := <-evictionDone:
		require.NoError(t, evictionErr)
	case <-time.After(2 * time.Second):
		t.Fatal("eviction did not finish after storage resumed")
	}
	require.Equal(t, uint(8), probe.counted())
	require.Equal(t, 2, probe.nodes())
}

func Test_CacheStorage_RestoredGenerationSurvivesIndexReuse(t *testing.T) {
	t.Parallel()

	storage := &blockingFailedEvictionStorage{
		failingCacheStorage: newFailingCacheStorage(),
		key:                 cacheKeyVersion + "|GET|/old",
		started:             make(chan struct{}),
		continueDelete:      make(chan struct{}),
	}
	probe := &accountingProbe{}
	var uncacheable atomic.Bool
	app := fiber.New()
	app.Use(New(Config{
		Expiration: time.Hour,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.Path()
		},
		MaxBytes:   10,
		Storage:    storage,
		accounting: probe.record,
	}))
	app.Get("/:name", func(c fiber.Ctx) error {
		switch c.Path() {
		case "/old":
			if uncacheable.Load() {
				c.Set(fiber.HeaderCacheControl, "no-store")
				return c.SendString("newer")
			}
			c.Set(fiber.HeaderCacheControl, "public, max-age=30")
			return c.SendString("stale")
		case "/slot":
			c.Set(fiber.HeaderCacheControl, "public, max-age=60")
			return c.SendString("")
		default:
			c.Set(fiber.HeaderCacheControl, "public, max-age=60")
			return c.SendString("0123456789")
		}
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/old", http.NoBody))
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	storage.armed.Store(true)
	evictionDone := make(chan error, 1)
	go func() {
		result, requestErr := app.Test(httptest.NewRequest(fiber.MethodGet, "/new", http.NoBody))
		if requestErr == nil {
			if result.StatusCode != fiber.StatusInternalServerError {
				requestErr = errors.New("eviction did not fail")
			}
			if closeErr := result.Body.Close(); closeErr != nil {
				requestErr = errors.Join(requestErr, closeErr)
			}
		}
		evictionDone <- requestErr
	}()

	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(storage.continueDelete) }) }
	defer release()
	select {
	case <-storage.started:
	case <-time.After(2 * time.Second):
		t.Fatal("eviction did not reach blocked deletion")
	}

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/slot", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.NoError(t, resp.Body.Close())
	release()
	select {
	case evictionErr := <-evictionDone:
		require.NoError(t, evictionErr)
	case <-time.After(2 * time.Second):
		t.Fatal("eviction did not finish")
	}
	require.Equal(t, uint(5), probe.counted())
	require.Equal(t, 2, probe.nodes())

	uncacheable.Store(true)
	request := httptest.NewRequest(fiber.MethodGet, "/old", http.NoBody)
	request.Header.Set(fiber.HeaderCacheControl, "max-age=0")
	resp, err = app.Test(request)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
	require.Zero(t, probe.counted())
	require.Equal(t, 1, probe.nodes())
}
