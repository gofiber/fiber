package cache

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

type blockingManifestStorage struct {
	*failingCacheStorage
	blocked      chan struct{}
	release      chan struct{}
	manifestSets atomic.Int32
	failBodySet  atomic.Bool
}

func (s *blockingManifestStorage) SetWithContext(ctx context.Context, key string, value []byte, exp time.Duration) error {
	if strings.HasSuffix(key, "_body") && s.failBodySet.Load() {
		return errors.New("body write failed")
	}
	if strings.HasSuffix(key, "|vary") && s.manifestSets.Add(1) == 2 {
		close(s.blocked)
		<-s.release
	}
	return s.failingCacheStorage.SetWithContext(ctx, key, value, exp)
}

func (s *blockingManifestStorage) Set(key string, value []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, value, exp)
}

// Both reloads read the same cached generation before entering the origin.
// The second store must replace the first reload's heap node even though the
// heap generation it originally read is no longer current.
func Test_Cache_ConcurrentReloadsReplaceSameKeyHeapNode(t *testing.T) {
	t.Parallel()

	probe := &accountingProbe{}
	var originCalls atomic.Int32
	firstEntered := make(chan struct{})
	secondEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})

	app := fiber.New()
	app.Use(New(Config{
		Expiration: time.Hour,
		MaxBytes:   1000,
		accounting: probe.record,
	}))
	app.Get("/entry", func(c fiber.Ctx) error {
		switch originCalls.Add(1) {
		case 2:
			close(firstEntered)
			<-releaseFirst
		case 3:
			close(secondEntered)
			<-releaseSecond
		}
		return c.Send(make([]byte, 100))
	})

	request := func(revalidate bool) (*http.Response, error) {
		req := httptest.NewRequest(fiber.MethodGet, "/entry", http.NoBody)
		if revalidate {
			req.Header.Set(fiber.HeaderCacheControl, "max-age=0")
		}
		return app.Test(req)
	}
	initial, err := request(false)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, initial.Header.Get("X-Cache"))
	_, err = io.Copy(io.Discard, initial.Body)
	require.NoError(t, err)
	require.NoError(t, initial.Body.Close())

	firstResult := make(chan error, 1)
	go func() {
		resp, requestErr := request(true)
		if requestErr == nil {
			requestErr = resp.Body.Close()
		}
		firstResult <- requestErr
	}()
	<-firstEntered

	secondResult := make(chan error, 1)
	go func() {
		resp, requestErr := request(true)
		if requestErr == nil {
			requestErr = resp.Body.Close()
		}
		secondResult <- requestErr
	}()
	<-secondEntered

	close(releaseFirst)
	require.NoError(t, <-firstResult)
	close(releaseSecond)
	require.NoError(t, <-secondResult)

	require.Equal(t, 1, probe.nodes())
	require.Equal(t, uint(100), probe.counted())

	final, err := request(false)
	require.NoError(t, err)
	require.Equal(t, cacheHit, final.Header.Get("X-Cache"))
	require.NoError(t, final.Body.Close())
	require.Equal(t, int32(3), originCalls.Load())
}

// A request can reserve space, wait on storage, and then observe another
// request's completed store. The final heap insertion must replace that node.
func Test_Cache_ConcurrentReloadAfterReservationReplacesHeapNode(t *testing.T) {
	t.Parallel()

	storage := &blockingManifestStorage{
		failingCacheStorage: newFailingCacheStorage(),
		blocked:             make(chan struct{}),
		release:             make(chan struct{}),
	}
	probe := &accountingProbe{}
	var originCalls atomic.Int32
	firstEntered := make(chan struct{})
	secondEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})

	app := fiber.New()
	app.Use(New(Config{
		Expiration: time.Hour,
		MaxBytes:   1000,
		Storage:    storage,
		accounting: probe.record,
	}))
	app.Get("/entry", func(c fiber.Ctx) error {
		switch originCalls.Add(1) {
		case 2:
			close(firstEntered)
			<-releaseFirst
		case 3:
			close(secondEntered)
			<-releaseSecond
		}
		c.Set(fiber.HeaderVary, fiber.HeaderAcceptLanguage)
		return c.Send(make([]byte, 100))
	})

	request := func(revalidate bool) (*http.Response, error) {
		req := httptest.NewRequest(fiber.MethodGet, "/entry", http.NoBody)
		req.Header.Set(fiber.HeaderAcceptLanguage, "en")
		if revalidate {
			req.Header.Set(fiber.HeaderCacheControl, "max-age=0")
		}
		return app.Test(req)
	}
	initial, err := request(false)
	require.NoError(t, err)
	require.NoError(t, initial.Body.Close())

	firstResult := make(chan error, 1)
	go func() {
		resp, requestErr := request(true)
		if requestErr == nil {
			requestErr = resp.Body.Close()
		}
		firstResult <- requestErr
	}()
	<-firstEntered

	secondResult := make(chan error, 1)
	go func() {
		resp, requestErr := request(true)
		if requestErr == nil {
			requestErr = resp.Body.Close()
		}
		secondResult <- requestErr
	}()
	<-secondEntered

	// Store the second reload first, but pause it after reservation and
	// before heap insertion while the first reload finishes its store.
	close(releaseSecond)
	<-storage.blocked
	close(releaseFirst)
	require.NoError(t, <-firstResult)
	close(storage.release)
	require.NoError(t, <-secondResult)

	require.Equal(t, 1, probe.nodes())
	require.Equal(t, uint(100), probe.counted())

	final, err := request(false)
	require.NoError(t, err)
	require.Equal(t, cacheHit, final.Header.Get("X-Cache"))
	require.NoError(t, final.Body.Close())
	require.Equal(t, int32(3), originCalls.Load())
}

func Test_Cache_ConcurrentReloadStoreFailureRestoresLatestHeapNode(t *testing.T) {
	t.Parallel()

	storage := &blockingManifestStorage{
		failingCacheStorage: newFailingCacheStorage(),
		blocked:             make(chan struct{}),
		release:             make(chan struct{}),
	}
	probe := &accountingProbe{}
	var originCalls atomic.Int32
	firstEntered := make(chan struct{})
	secondEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})

	app := fiber.New()
	app.Use(New(Config{
		Expiration: time.Hour,
		MaxBytes:   1000,
		Storage:    storage,
		accounting: probe.record,
	}))
	app.Get("/entry", func(c fiber.Ctx) error {
		switch originCalls.Add(1) {
		case 2:
			close(firstEntered)
			<-releaseFirst
		case 3:
			close(secondEntered)
			<-releaseSecond
		}
		c.Set(fiber.HeaderVary, fiber.HeaderAcceptLanguage)
		return c.Send(make([]byte, 100))
	})

	request := func(revalidate bool) (*http.Response, error) {
		req := httptest.NewRequest(fiber.MethodGet, "/entry", http.NoBody)
		req.Header.Set(fiber.HeaderAcceptLanguage, "en")
		if revalidate {
			req.Header.Set(fiber.HeaderCacheControl, "max-age=0")
		}
		return app.Test(req)
	}
	initial, err := request(false)
	require.NoError(t, err)
	require.NoError(t, initial.Body.Close())

	firstResult := make(chan error, 1)
	go func() {
		resp, requestErr := request(true)
		if requestErr == nil {
			requestErr = resp.Body.Close()
		}
		firstResult <- requestErr
	}()
	<-firstEntered

	secondResult := make(chan struct {
		resp *http.Response
		err  error
	}, 1)
	go func() {
		resp, requestErr := request(true)
		secondResult <- struct {
			resp *http.Response
			err  error
		}{resp: resp, err: requestErr}
	}()
	<-secondEntered

	close(releaseSecond)
	<-storage.blocked
	close(releaseFirst)
	require.NoError(t, <-firstResult)
	storage.failBodySet.Store(true)
	close(storage.release)
	failed := <-secondResult
	require.NoError(t, failed.err)
	require.Equal(t, fiber.StatusInternalServerError, failed.resp.StatusCode)
	require.NoError(t, failed.resp.Body.Close())
	storage.failBodySet.Store(false)

	require.Equal(t, 1, probe.nodes())
	require.Equal(t, uint(100), probe.counted())

	final, err := request(false)
	require.NoError(t, err)
	require.Equal(t, cacheHit, final.Header.Get("X-Cache"))
	require.NoError(t, final.Body.Close())
	require.Equal(t, int32(3), originCalls.Load())
}
