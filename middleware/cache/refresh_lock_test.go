package cache

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/storagetest"
	"github.com/stretchr/testify/require"
)

// cacheTestCEvict sends GET /new through app in the background, a request the
// test arranged to be answered unstored because the eviction it starts cannot
// delete the entry it picked. The channel yields what went wrong, or nil.
func cacheTestCEvict(app *fiber.App) <-chan error {
	done := make(chan error, 1)
	go func() {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/new", http.NoBody))
		if err == nil {
			if resp.StatusCode != fiber.StatusOK || resp.Header.Get("X-Cache") != cacheUnreachable {
				err = errors.New("the request was not answered unstored")
			}
			if closeErr := resp.Body.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}
		done <- err
	}()
	return done
}

func Test_CacheStorage_SlowRestoreDoesNotBlockUnrelatedHit(t *testing.T) {
	t.Parallel()
	const oldKey = cacheKeyVersion + "|GET|/old"
	started := make(chan struct{})
	resume := make(chan struct{})
	var armed atomic.Bool
	storage := &hookedStorage{Memory: storagetest.New(), onGet: func(key string) error {
		// The read that restores the entry eviction picked parks until the test lets it go on.
		if key == oldKey && armed.CompareAndSwap(true, false) {
			close(started)
			<-resume
		}
		return nil
	}}
	// Room for /old and /hot, so /new evicts /old.
	stored := charged(oldKey, len("stale"), textPlain, "public, max-age=30") +
		charged(cacheKeyVersion+"|GET|/hot", len("hot"), textPlain, "public, max-age=60")
	probe := &accountingProbe{}
	app := cacheTestBPathKeyedApp(&Config{MaxBytes: stored, Storage: storage, accounting: probe.record})
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
		cacheTestAExpect(t, app, path, cacheMiss)
	}
	cacheTestBRequireAccounting(t, probe, stored, 2)

	storage.SetErr(storagetest.OpDelete, oldKey, errors.New("eviction delete failed"))
	armed.Store(true)

	evictionDone := cacheTestCEvict(app)

	var resumeOnce sync.Once
	release := func() { resumeOnce.Do(func() { close(resume) }) }
	defer release()

	apptest.Recv(t, started, 2*time.Second, "eviction did not reach restore read")

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

	require.NoError(t, apptest.Recv(t, hitDone, 2*time.Second, "unrelated cache hit blocked on restore storage I/O"))
	require.Equal(t, stored, probe.counted(), "pending restore must retain its byte reservation")
	require.Equal(t, 2, probe.nodes())

	release()
	require.NoError(t, apptest.Recv(t, evictionDone, 2*time.Second, "eviction did not finish after storage resumed"))
	cacheTestBRequireAccounting(t, probe, stored, 2)
}

func Test_CacheStorage_RestoredGenerationSurvivesIndexReuse(t *testing.T) {
	t.Parallel()
	key, gate := cacheKeyVersion+"|GET|/old", newCacheTestGate(false)
	storage := cacheTestParkedDelete(key, gate, errors.New("eviction delete failed"))
	stale := charged(key, len("stale"), textPlain, "public, max-age=30")
	slot := charged(cacheKeyVersion+"|GET|/slot", len("x"), textPlain, "public, max-age=60")
	fresh := charged(cacheKeyVersion+"|GET|/new", len("012345678"), textPlain, "public, max-age=60")
	probe := &accountingProbe{}
	var uncacheable atomic.Bool
	app := cacheTestBPathKeyedApp(&Config{MaxBytes: fresh + slot, Storage: storage, accounting: probe.record})
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
			// Fits beside the reservation of /new only while /old is popped.
			c.Set(fiber.HeaderCacheControl, "public, max-age=60")
			return c.SendString("x")
		default:
			c.Set(fiber.HeaderCacheControl, "public, max-age=60")
			return c.SendString("012345678")
		}
	})

	apptest.BodyBytes(t, apptest.Do(t, app, "/old"))

	gate.armed.Store(true)
	evictionDone := cacheTestCEvict(app)

	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(gate.release) }) }
	defer release()
	apptest.Recv(t, gate.entered, 2*time.Second, "eviction did not reach blocked deletion")

	cacheTestAExpect(t, app, "/slot", cacheMiss)
	release()
	require.NoError(t, apptest.Recv(t, evictionDone, 2*time.Second, "eviction did not finish"))
	cacheTestBRequireAccounting(t, probe, stale+slot, 2)

	uncacheable.Store(true)
	apptest.BodyBytes(t, apptest.Expect(t, app, cacheTestAGetCC("/old", "max-age=0"), fiber.StatusOK))
	cacheTestBRequireAccounting(t, probe, slot, 1)
}
