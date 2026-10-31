package cache

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/valyala/fasthttp"
)

// interleavedSetStorage runs duringFirstSet once, after the cache decided to
// write and before the write reaches the backend.
type interleavedSetStorage struct {
	*storagetest.Memory
	duringFirstSet func()
	entered        atomic.Bool
	failDeletes    atomic.Bool
}

func (s *interleavedSetStorage) SetWithContext(ctx context.Context, key string, value []byte, exp time.Duration) error {
	if s.entered.CompareAndSwap(false, true) {
		s.duringFirstSet()
	}
	return s.Memory.SetWithContext(ctx, key, value, exp)
}

func (s *interleavedSetStorage) Set(key string, value []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, value, exp)
}

func (s *interleavedSetStorage) DeleteWithContext(ctx context.Context, key string) error {
	if s.failDeletes.Load() {
		return errors.New("delete failed")
	}
	return s.Memory.DeleteWithContext(ctx, key)
}

func (s *interleavedSetStorage) Delete(key string) error {
	return s.DeleteWithContext(context.Background(), key)
}

func (s *interleavedSetStorage) bodiesOfSize(size int) int {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	held := 0
	for _, value := range s.Data {
		if len(value) == size {
			held++
		}
	}
	return held
}

// The heap node is inserted before the entry is written. A store that evicts
// the node in between deletes a key that is not there yet, so the entry written
// afterwards must not stay behind untracked.
func Test_Cache_MaxBytes_EntryEvictedWhileStoredIsNotKept(t *testing.T) {
	t.Parallel()
	storage := &interleavedSetStorage{Memory: storagetest.New()}
	probe := &accountingProbe{}

	app := fiber.New()
	app.Use(New(Config{
		Expiration: time.Hour,
		MaxBytes:   charged(defaultKey("/second"), 100, textPlain), // one entry
		Storage:    storage,
		accounting: probe.record,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.Send(make([]byte, 100)) })

	// Returns its error instead of asserting: the storage hook calls it from the
	// goroutine that serves the first request.
	get := func(target, cacheControl string) (int, error) {
		req := httptest.NewRequest(fiber.MethodGet, target, http.NoBody)
		if cacheControl != "" {
			req.Header.Set(fiber.HeaderCacheControl, cacheControl)
		}
		resp, err := app.Test(req)
		if err != nil {
			return 0, err
		}
		return resp.StatusCode, resp.Body.Close()
	}

	var (
		secondStatus int
		errSecond    error
	)
	storage.duringFirstSet = func() { secondStatus, errSecond = get("/second", "") }
	status, err := get("/first", "")
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, status)
	require.NoError(t, errSecond)
	require.Equal(t, fiber.StatusOK, secondStatus)

	status, err = get("/second", "only-if-cached")
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, status)
	status, err = get("/first", "only-if-cached")
	require.NoError(t, err)
	require.Equal(t, fiber.StatusGatewayTimeout, status)
	require.Equal(t, 1, probe.nodes())
	require.Equal(t, charged(defaultKey("/second"), 100, textPlain), probe.counted())
	require.Equal(t, 1, storage.bodiesOfSize(100))
}

// When the entry cannot be taken back out, it has to be tracked again so that a
// later eviction reaches it.
func Test_Cache_MaxBytes_EntryEvictedWhileStoredIsTrackedWhenDeleteFails(t *testing.T) {
	t.Parallel()
	storage := &interleavedSetStorage{Memory: storagetest.New()}
	probe := &accountingProbe{}

	app := fiber.New()
	app.Use(New(Config{
		Expiration: time.Hour,
		MaxBytes:   charged(defaultKey("/second"), 100, textPlain), // one entry
		Storage:    storage,
		accounting: probe.record,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.Send(make([]byte, 100)) })

	get := func(target string) (int, error) {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, target, http.NoBody))
		if err != nil {
			return 0, err
		}
		return resp.StatusCode, resp.Body.Close()
	}

	var (
		secondStatus int
		errSecond    error
	)
	storage.duringFirstSet = func() {
		secondStatus, errSecond = get("/second")
		storage.failDeletes.Store(true)
	}
	status, err := get("/first")
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, status)
	require.NoError(t, errSecond)
	require.Equal(t, fiber.StatusOK, secondStatus)
	require.Equal(t, 2, probe.nodes())
	require.Equal(t, charged(defaultKey("/first"), 100, textPlain)+charged(defaultKey("/second"), 100, textPlain), probe.counted())

	storage.failDeletes.Store(false)
	status, err = get("/third")
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, status)
	require.Equal(t, 1, probe.nodes())
	require.Equal(t, charged(defaultKey("/third"), 100, textPlain), probe.counted())
	require.Equal(t, 1, storage.bodiesOfSize(100))
}

// The in-memory store offers no seam between the two steps, so this one races
// for real: distinct keys, every store evicting, then a count of what is held.
func Test_Cache_MaxBytes_ConcurrentStoresStayWithinLimit(t *testing.T) {
	t.Parallel()
	const (
		bodySize = 1024
		maxBytes = 8 * bodySize
		workers  = 16
		perWork  = 4000
	)

	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour, MaxBytes: maxBytes}))
	app.Get("/*", func(c fiber.Ctx) error { return c.Send(make([]byte, bodySize)) })
	handler := app.Handler()

	get := func(target, cacheControl string) int {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetMethod(fiber.MethodGet)
		ctx.Request.Header.SetHost("example.com")
		ctx.Request.SetRequestURI(target)
		if cacheControl != "" {
			ctx.Request.Header.Set(fiber.HeaderCacheControl, cacheControl)
		}
		handler(ctx)
		return ctx.Response.StatusCode()
	}

	var wg sync.WaitGroup
	for worker := range workers {
		wg.Go(func() {
			for i := range perWork {
				get(fmt.Sprintf("/%d/%d", worker, i), "")
			}
		})
	}
	wg.Wait()

	held := 0
	for worker := range workers {
		for i := range perWork {
			if get(fmt.Sprintf("/%d/%d", worker, i), "only-if-cached") == fiber.StatusOK {
				held += bodySize
			}
		}
	}
	require.LessOrEqual(t, held, maxBytes)
}

// What exceeds MaxBytes once every node is popped is reserved by stores still in
// flight: the popped entries stay stored and counted, and the response is served
// without being stored.
func Test_Cache_MaxBytes_NoSpaceKeepsPoppedEntriesTracked(t *testing.T) {
	t.Parallel()
	storage := &blockingDeleteStorage{
		Memory:         storagetest.New(),
		deleteStarted:  make(chan struct{}),
		continueDelete: make(chan struct{}),
		key:            cacheKeyVersion + "|GET|/first/200",
	}
	charge := func(target string, size int) uint {
		return charged(cacheKeyVersion+"|GET|"+target, size, textPlain)
	}
	// Room for /second, /third and /inflight, so /inflight evicts /first.
	maxBytes := charge("/second/200", 200) + charge("/third/200", 200) + charge("/inflight/600", 600)
	probe := &accountingProbe{}
	app := fiber.New()
	app.Use(New(Config{
		ExpirationGenerator: stableAscendingExpiration(),
		KeyGenerator:        func(c fiber.Ctx) string { return c.Path() },
		MaxBytes:            maxBytes,
		Storage:             storage,
		accounting:          probe.record,
	}))
	app.Get("/:name/:size", func(c fiber.Ctx) error { return c.Send(make([]byte, fiber.Params[int](c, "size"))) })

	for _, target := range []string{"/first/200", "/second/200", "/third/200"} {
		resp := apptest.Do(t, app, target)
		require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
		require.NoError(t, resp.Body.Close())
	}

	// Reserves its room and stops while it deletes the entry it evicted.
	inFlightDone := make(chan struct{})
	var inFlightResp *http.Response
	var inFlightErr error
	go func() {
		inFlightResp, inFlightErr = app.Test(httptest.NewRequest(fiber.MethodGet, "/inflight/600", http.NoBody))
		close(inFlightDone)
	}()
	<-storage.deleteStarted

	released := false
	defer func() {
		if !released {
			close(storage.continueDelete)
		}
	}()

	// /inflight's reservation and /nospace do not fit, whatever is evicted.
	resp := apptest.Expect(t, app, "/nospace/600", fiber.StatusOK)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	require.NoError(t, resp.Body.Close())
	require.Equal(t, maxBytes, probe.counted(), "the entries that stay stored must stay counted")
	require.Equal(t, 2, probe.nodes(), "the entries that stay stored must stay evictable")

	// A refresh still finds the node its entry recorded and takes over its share.
	refresh := httptest.NewRequest(fiber.MethodGet, "/third/200", http.NoBody)
	refresh.Header.Set(fiber.HeaderCacheControl, "no-cache")
	resp = apptest.Do(t, app, refresh)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.NoError(t, resp.Body.Close())
	require.Equal(t, maxBytes, probe.counted())
	require.Equal(t, 2, probe.nodes())

	close(storage.continueDelete)
	released = true
	<-inFlightDone
	require.NoError(t, inFlightErr)
	require.Equal(t, cacheMiss, inFlightResp.Header.Get("X-Cache"))
	require.NoError(t, inFlightResp.Body.Close())

	// Needs the room of both entries and of the one that was in flight.
	resp = apptest.Do(t, app, "/later/1000")
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.NoError(t, resp.Body.Close())
	require.Equal(t, charge("/later/1000", 1000), probe.counted())
	require.Equal(t, 1, probe.nodes())

	for _, target := range []string{"/second/200", "/third/200", "/inflight/600"} {
		req := httptest.NewRequest(fiber.MethodGet, target, http.NoBody)
		req.Header.Set(fiber.HeaderCacheControl, "only-if-cached")
		resp = apptest.Do(t, app, req)
		require.Equal(t, fiber.StatusGatewayTimeout, resp.StatusCode, "%s must have been evicted", target)
		require.NoError(t, resp.Body.Close())
	}
}
