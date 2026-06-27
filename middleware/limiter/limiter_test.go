package limiter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
)

// testClock is a manually advanced clock that makes window-expiry tests
// deterministic: instead of sleeping past a window boundary (racy under
// -race -count -shuffle), tests advance the clock explicitly. It is safe for
// the concurrent reads performed by the limiter handler.
type testClock struct {
	now time.Time
	mu  sync.Mutex
}

func newTestClock(start time.Time) *testClock {
	return &testClock{now: start}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type failingLimiterStorage struct {
	data map[string][]byte
	errs map[string]error
	mu   sync.Mutex
}

const testLimiterClientKey = "client-key"

type typedNilLimiterError struct {
	message string
}

func (e *typedNilLimiterError) Error() string {
	return e.message
}

func newFailingLimiterStorage() *failingLimiterStorage {
	return &failingLimiterStorage{
		data: make(map[string][]byte),
		errs: make(map[string]error),
	}
}

// countingFailStorage fails set operations after a specified number of successful calls
type countingFailStorage struct {
	*failingLimiterStorage
	setFailErr error
	setCount   int
	failAfterN int
}

func newCountingFailStorage(failAfterN int, err error) *countingFailStorage {
	return &countingFailStorage{
		failingLimiterStorage: newFailingLimiterStorage(),
		failAfterN:            failAfterN,
		setFailErr:            err,
	}
}

func (s *countingFailStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.setCount++
	if s.setCount > s.failAfterN {
		return s.setFailErr
	}
	return s.failingLimiterStorage.SetWithContext(ctx, key, val, exp)
}

type contextRecord struct {
	key      string
	value    string
	canceled bool
}

type contextRecorderLimiterStorage struct {
	*failingLimiterStorage
	gets []contextRecord
	sets []contextRecord
}

type blockingLimiterStorage struct {
	*failingLimiterStorage
	enter   map[string]chan struct{}
	release chan struct{}
	mu      sync.Mutex
}

func sleepForRetryAfter(t *testing.T, resp *http.Response) {
	t.Helper()

	retryAfter := resp.Header.Get(fiber.HeaderRetryAfter)
	if retryAfter == "" {
		time.Sleep(500 * time.Millisecond)
		return
	}

	seconds, err := strconv.Atoi(retryAfter)
	require.NoError(t, err)

	delay := time.Duration(seconds) * time.Second
	// Sliding window needs roughly 2x the reported delay for the previous window to expire.
	if doubled := 2 * delay; doubled > delay {
		delay = doubled
	}
	if minDelay := 4 * time.Second; delay < minDelay {
		delay = minDelay
	}

	time.Sleep(delay + 500*time.Millisecond)
}

func newContextRecorderLimiterStorage() *contextRecorderLimiterStorage {
	return &contextRecorderLimiterStorage{failingLimiterStorage: newFailingLimiterStorage()}
}

func newBlockingLimiterStorage() *blockingLimiterStorage {
	return &blockingLimiterStorage{
		failingLimiterStorage: newFailingLimiterStorage(),
		enter:                 make(map[string]chan struct{}),
		release:               make(chan struct{}),
	}
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

func (s *contextRecorderLimiterStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	s.gets = append(s.gets, contextRecordFrom(ctx, key))
	return s.failingLimiterStorage.GetWithContext(ctx, key)
}

func (s *contextRecorderLimiterStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.sets = append(s.sets, contextRecordFrom(ctx, key))
	return s.failingLimiterStorage.SetWithContext(ctx, key, val, exp)
}

func (s *contextRecorderLimiterStorage) recordedGets() []contextRecord {
	out := make([]contextRecord, len(s.gets))
	copy(out, s.gets)
	return out
}

func (s *contextRecorderLimiterStorage) recordedSets() []contextRecord {
	out := make([]contextRecord, len(s.sets))
	copy(out, s.sets)
	return out
}

func (s *blockingLimiterStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.mu.Lock()
	if _, ok := s.enter[key]; !ok {
		ch := make(chan struct{})
		s.enter[key] = ch
		close(ch)
	}
	release := s.release
	s.mu.Unlock()

	<-release
	return s.failingLimiterStorage.SetWithContext(ctx, key, val, exp)
}

func (s *blockingLimiterStorage) waitForKey(t *testing.T, key string) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	for {
		s.mu.Lock()
		ch, ok := s.enter[key]
		s.mu.Unlock()
		if ok {
			select {
			case <-ch:
				return
			case <-deadline:
				t.Fatalf("timed out waiting for storage key %q", key)
			}
		}

		select {
		case <-time.After(10 * time.Millisecond):
		case <-deadline:
			t.Fatalf("timed out waiting for storage key %q", key)
		}
	}
}

func (s *failingLimiterStorage) GetWithContext(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err, ok := s.errs["get|"+key]; ok && err != nil {
		return nil, err
	}
	if val, ok := s.data[key]; ok {
		return append([]byte(nil), val...), nil
	}
	return nil, nil
}

func (s *failingLimiterStorage) Get(key string) ([]byte, error) {
	return s.GetWithContext(context.Background(), key)
}

func (s *failingLimiterStorage) SetWithContext(_ context.Context, key string, val []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err, ok := s.errs["set|"+key]; ok && err != nil {
		return err
	}
	s.data[key] = append([]byte(nil), val...)
	return nil
}

func (s *failingLimiterStorage) Set(key string, val []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, exp)
}

func (*failingLimiterStorage) DeleteWithContext(context.Context, string) error { return nil }

func (*failingLimiterStorage) Delete(string) error { return nil }

func (*failingLimiterStorage) ResetWithContext(context.Context) error { return nil }

func (*failingLimiterStorage) Reset() error { return nil }

func (*failingLimiterStorage) Close() error { return nil }

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

func TestLimiterDefaultConfigNoPanic(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	require.NotPanics(t, func() {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})
}

func TestGetEffectiveStatusCodeTypedNilFiberError(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

	c.Response().SetStatusCode(fiber.StatusAccepted)

	var err *fiber.Error
	require.NotPanics(t, func() {
		require.Equal(t, fiber.StatusAccepted, getEffectiveStatusCode(c, err))
	})
}

func TestGetEffectiveStatusCodeTypedNilCustomError(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

	c.Response().SetStatusCode(fiber.StatusAccepted)

	var err *typedNilLimiterError
	require.NotPanics(t, func() {
		require.Equal(t, fiber.StatusAccepted, getEffectiveStatusCode(c, err))
	})
}

func TestLimiterFixedStorageGetError(t *testing.T) {
	t.Parallel()

	storage := newFailingLimiterStorage()
	storage.errs["get|"+testLimiterClientKey] = errors.New("boom")

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	})

	app.Use(New(Config{Storage: storage, Max: 1, Expiration: time.Second, KeyGenerator: func(fiber.Ctx) string { return testLimiterClientKey }}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, "limiter: failed to get key")
	require.ErrorContains(t, captured, "[redacted]")
}

func TestLimiterFixedStorageSetError(t *testing.T) {
	t.Parallel()

	storage := newFailingLimiterStorage()
	storage.errs["set|"+testLimiterClientKey] = errors.New("boom")

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	})

	app.Use(New(Config{Storage: storage, Max: 1, Expiration: time.Second, KeyGenerator: func(fiber.Ctx) string { return testLimiterClientKey }}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, "limiter: failed to persist state")
	require.ErrorContains(t, captured, "limiter: failed to store key")
	require.ErrorContains(t, captured, "[redacted]")
}

func TestLimiterFixedPropagatesRequestContextToStorage(t *testing.T) {
	t.Parallel()

	storage := newContextRecorderLimiterStorage()

	app := fiber.New()

	app.Use(func(c fiber.Ctx) error {
		path := c.Path()
		if path == "/normal" {
			c.SetContext(contextWithMarker("fixed-normal"))
		}
		if path == "/rollback" {
			c.SetContext(canceledContextWithMarker("fixed-rollback"))
		}
		return c.Next()
	})

	app.Use(New(Config{
		Storage:                storage,
		Max:                    1,
		Expiration:             time.Minute,
		SkipSuccessfulRequests: true,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.Path()
		},
		LimiterMiddleware: FixedWindow{},
	}))

	app.Get("/:mode", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	for _, path := range []string{"/normal", "/rollback"} {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	}

	gets := storage.recordedGets()
	require.Len(t, gets, 4)

	sets := storage.recordedSets()
	require.Len(t, sets, 4)

	verifyRecords := func(t *testing.T, records []contextRecord, key, wantValue string, wantCanceled bool) {
		t.Helper()
		var matched []contextRecord
		for _, rec := range records {
			if rec.key == key {
				matched = append(matched, rec)
			}
		}
		require.Len(t, matched, 2)
		for _, rec := range matched {
			require.Equal(t, wantValue, rec.value)
			require.Equal(t, wantCanceled, rec.canceled)
		}
	}

	verifyRecords(t, gets, "/normal", "fixed-normal", false)
	verifyRecords(t, gets, "/rollback", "fixed-rollback", true)
	verifyRecords(t, sets, "/normal", "fixed-normal", false)
	verifyRecords(t, sets, "/rollback", "fixed-rollback", true)
}

func testLimiterDifferentKeysDoNotBlockStorage(t *testing.T, middleware Handler) {
	t.Helper()

	storage := newBlockingLimiterStorage()
	app := fiber.New()
	app.Use(New(Config{
		Storage:           storage,
		Max:               10,
		Expiration:        time.Minute,
		LimiterMiddleware: middleware,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.Get("X-Limiter-Key")
		},
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	runRequest := func(key string) <-chan error {
		done := make(chan error, 1)
		go func() {
			req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
			req.Header.Set("X-Limiter-Key", key)
			resp, err := app.Test(req)
			if err == nil && resp.StatusCode != fiber.StatusOK {
				err = fmt.Errorf("unexpected status for %s: %d", key, resp.StatusCode)
			}
			done <- err
		}()
		return done
	}

	firstDone := runRequest("alpha")
	storage.waitForKey(t, "alpha")

	secondDone := runRequest("bravo")
	storage.waitForKey(t, "bravo")

	close(storage.release)

	require.NoError(t, <-firstDone)
	require.NoError(t, <-secondDone)
}

func TestLimiterFixedDifferentKeysDoNotBlockStorage(t *testing.T) {
	t.Parallel()
	testLimiterDifferentKeysDoNotBlockStorage(t, FixedWindow{})
}

func TestLimiterFixedStorageGetErrorDisableRedaction(t *testing.T) {
	t.Parallel()

	storage := newFailingLimiterStorage()
	storage.errs["get|"+testLimiterClientKey] = errors.New("boom")

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	})

	app.Use(New(Config{DisableValueRedaction: true, Storage: storage, Max: 1, Expiration: time.Second, KeyGenerator: func(fiber.Ctx) string { return testLimiterClientKey }}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, testLimiterClientKey)
	require.NotContains(t, captured.Error(), "[redacted]")
}

func TestLimiterFixedStorageSetErrorDisableRedaction(t *testing.T) {
	t.Parallel()

	storage := newFailingLimiterStorage()
	storage.errs["set|"+testLimiterClientKey] = errors.New("boom")

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	})

	app.Use(New(Config{DisableValueRedaction: true, Storage: storage, Max: 1, Expiration: time.Second, KeyGenerator: func(fiber.Ctx) string { return testLimiterClientKey }}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, testLimiterClientKey)
	require.NotContains(t, captured.Error(), "[redacted]")
}

func TestLimiterFixedStorageSetErrorOnSkipSuccessfulRequests(t *testing.T) {
	t.Parallel()

	storage := newCountingFailStorage(1, errors.New("second set failed"))

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	})

	app.Use(New(Config{
		Storage:                storage,
		Max:                    10,
		Expiration:             time.Second,
		SkipSuccessfulRequests: true,
		KeyGenerator:           func(fiber.Ctx) string { return testLimiterClientKey },
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, "limiter: failed to persist state")
}

func TestLimiterSlidingPropagatesRequestContextToStorage(t *testing.T) {
	t.Parallel()

	storage := newContextRecorderLimiterStorage()

	app := fiber.New()

	app.Use(func(c fiber.Ctx) error {
		path := c.Path()
		if path == "/normal" {
			c.SetContext(contextWithMarker("sliding-normal"))
		}
		if path == "/rollback" {
			c.SetContext(canceledContextWithMarker("sliding-rollback"))
		}
		return c.Next()
	})

	app.Use(New(Config{
		Storage:                storage,
		Max:                    1,
		Expiration:             time.Minute,
		SkipSuccessfulRequests: true,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.Path()
		},
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/:mode", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	for _, path := range []string{"/normal", "/rollback"} {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	}

	gets := storage.recordedGets()
	require.Len(t, gets, 4)

	sets := storage.recordedSets()
	require.Len(t, sets, 4)

	verifyRecords := func(t *testing.T, records []contextRecord, key, wantValue string, wantCanceled bool) {
		t.Helper()
		var matched []contextRecord
		for _, rec := range records {
			if rec.key == key {
				matched = append(matched, rec)
			}
		}
		require.Len(t, matched, 2)
		for _, rec := range matched {
			require.Equal(t, wantValue, rec.value)
			require.Equal(t, wantCanceled, rec.canceled)
		}
	}

	verifyRecords(t, gets, "/normal", "sliding-normal", false)
	verifyRecords(t, gets, "/rollback", "sliding-rollback", true)
	verifyRecords(t, sets, "/normal", "sliding-normal", false)
	verifyRecords(t, sets, "/rollback", "sliding-rollback", true)
}

func TestLimiterSlidingDifferentKeysDoNotBlockStorage(t *testing.T) {
	t.Parallel()
	testLimiterDifferentKeysDoNotBlockStorage(t, SlidingWindow{})
}

func TestLimiterSlidingSkipsPostUpdateWhenHeadersDisabled(t *testing.T) {
	t.Parallel()

	storage := newContextRecorderLimiterStorage()
	app := fiber.New()

	app.Use(New(Config{
		Max:               1,
		Expiration:        time.Second,
		Storage:           storage,
		DisableHeaders:    true,
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.Len(t, storage.recordedGets(), 1)
	require.Len(t, storage.recordedSets(), 1)
}

// go test -run Test_Limiter_With_Max_Func_With_Zero -race -v
func Test_Limiter_With_Max_Func_With_Zero_And_Limiter_Sliding(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		MaxFunc:                func(_ fiber.Ctx) int { return 0 },
		Expiration:             2 * time.Second,
		SkipFailedRequests:     false,
		SkipSuccessfulRequests: false,
		LimiterMiddleware:      SlidingWindow{},
	}))

	app.Get("/:status", func(c fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	time.Sleep(4*time.Second + 500*time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

func Test_Limiter_Sliding_MaxFuncOverridesStaticMax(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	staticMax := 5
	dynamicMax := 2

	app.Use(New(Config{
		Max:               staticMax,
		MaxFunc:           func(fiber.Ctx) int { return dynamicMax },
		Expiration:        2 * time.Second,
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, strconv.Itoa(dynamicMax), resp.Header.Get("X-RateLimit-Limit"))
	require.Equal(t, strconv.Itoa(dynamicMax-1), resp.Header.Get("X-RateLimit-Remaining"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, strconv.Itoa(dynamicMax), resp.Header.Get("X-RateLimit-Limit"))
	require.Equal(t, strconv.Itoa(dynamicMax-2), resp.Header.Get("X-RateLimit-Remaining"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
}

// go test -run Test_Limiter_With_Max_Func_With_Zero -race -v
func Test_Limiter_With_Max_Func_With_Zero(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		MaxFunc: func(_ fiber.Ctx) int {
			return 0
		},
		Expiration: 2 * time.Second,
		Storage:    memory.New(),
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello tester!")
	})

	var wg sync.WaitGroup

	for i := 0; i <= 4; i++ {
		wg.Go(func() {
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			assert.Equal(t, fiber.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, "Hello tester!", string(body))
		})
	}

	wg.Wait()

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

// go test -run Test_Limiter_With_Max_Func -race -v
func Test_Limiter_With_Max_Func(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	maxRequests := 10

	app.Use(New(Config{
		MaxFunc: func(_ fiber.Ctx) int {
			return maxRequests
		},
		Expiration: 2 * time.Second,
		Storage:    memory.New(),
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello tester!")
	})

	var wg sync.WaitGroup

	for i := 0; i <= maxRequests-1; i++ {
		wg.Go(func() {
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			assert.Equal(t, fiber.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, "Hello tester!", string(body))
		})
	}

	wg.Wait()

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

// go test -run Test_Limiter_Fixed_ExpirationFuncOverridesStaticExpiration -race -v
func Test_Limiter_Fixed_ExpirationFuncOverridesStaticExpiration(t *testing.T) {
	t.Parallel()
	clock := newTestClock(time.Now().Truncate(time.Second))
	app := fiber.New()

	app.Use(New(Config{
		Max:               2,
		Expiration:        10 * time.Second,
		ExpirationFunc:    func(_ fiber.Ctx) time.Duration { return 2 * time.Second },
		clock:             clock.Now,
		LimiterMiddleware: FixedWindow{},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)

	// Advance past the 2s ExpirationFunc window so the fixed window resets.
	clock.Add(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// go test -run Test_Limiter_Sliding_ExpirationFuncOverridesStaticExpiration -race -v
func Test_Limiter_Sliding_ExpirationFuncOverridesStaticExpiration(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:               2,
		Expiration:        10 * time.Second,
		ExpirationFunc:    func(_ fiber.Ctx) time.Duration { return 2 * time.Second },
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)

	// Sliding window needs ~2x expiration to fully reset (considers previous window)
	sleepForRetryAfter(t, resp)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// go test -run Test_Limiter_Fixed_ExpirationFunc_FallbackOnZeroDuration -race -v
func Test_Limiter_Fixed_ExpirationFunc_FallbackOnZeroDuration(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:               1,
		ExpirationFunc:    func(_ fiber.Ctx) time.Duration { return 0 },
		LimiterMiddleware: FixedWindow{},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
}

// go test -run Test_Limiter_Fixed_ExpirationFunc_FallbackOnNegativeDuration -race -v
func Test_Limiter_Fixed_ExpirationFunc_FallbackOnNegativeDuration(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:               1,
		ExpirationFunc:    func(_ fiber.Ctx) time.Duration { return -1 * time.Second },
		LimiterMiddleware: FixedWindow{},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
}

// go test -run Test_Limiter_Sliding_ExpirationFunc_FallbackOnZeroDuration -race -v
func Test_Limiter_Sliding_ExpirationFunc_FallbackOnZeroDuration(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:               1,
		ExpirationFunc:    func(_ fiber.Ctx) time.Duration { return 0 },
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
}

// go test -run Test_Limiter_Sliding_ExpirationFunc_FallbackOnNegativeDuration -race -v
func Test_Limiter_Sliding_ExpirationFunc_FallbackOnNegativeDuration(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:               1,
		ExpirationFunc:    func(_ fiber.Ctx) time.Duration { return -1 * time.Second },
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
}

// go test -run Test_Limiter_Concurrency_Store -race -v
func Test_Limiter_Concurrency_Store(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:        50,
		Expiration: 2 * time.Second,
		Storage:    memory.New(),
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello tester!")
	})

	var wg sync.WaitGroup

	for i := 0; i <= 49; i++ {
		wg.Go(func() {
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			assert.Equal(t, fiber.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, "Hello tester!", string(body))
		})
	}

	wg.Wait()

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

// go test -run Test_Limiter_Concurrency -race -v
func Test_Limiter_Concurrency(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:        50,
		Expiration: 2 * time.Second,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello tester!")
	})

	var wg sync.WaitGroup

	for i := 0; i <= 49; i++ {
		wg.Go(func() {
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			assert.Equal(t, fiber.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, "Hello tester!", string(body))
		})
	}

	wg.Wait()

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

// go test -run Test_Limiter_Fixed_Window_No_Skip_Choices -v
func Test_Limiter_Fixed_Window_No_Skip_Choices(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                    2,
		Expiration:             2 * time.Second,
		SkipFailedRequests:     false,
		SkipSuccessfulRequests: false,
		LimiterMiddleware:      FixedWindow{},
	}))

	app.Get("/:status", func(c fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

// go test -run Test_Limiter_Fixed_Window_Custom_Storage_No_Skip_Choices -v
func Test_Limiter_Fixed_Window_Custom_Storage_No_Skip_Choices(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                    2,
		Expiration:             2 * time.Second,
		SkipFailedRequests:     false,
		SkipSuccessfulRequests: false,
		Storage:                memory.New(),
		LimiterMiddleware:      FixedWindow{},
	}))

	app.Get("/:status", func(c fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

// go test -run Test_Limiter_Sliding_Window_No_Skip_Choices -v
func Test_Limiter_Sliding_Window_No_Skip_Choices(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                    2,
		Expiration:             2 * time.Second,
		SkipFailedRequests:     false,
		SkipSuccessfulRequests: false,
		LimiterMiddleware:      SlidingWindow{},
	}))

	app.Get("/:status", func(c fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	sleepForRetryAfter(t, resp)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

// go test -run Test_Limiter_Sliding_Window_Custom_Storage_No_Skip_Choices -v
func Test_Limiter_Sliding_Window_Custom_Storage_No_Skip_Choices(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                    2,
		Expiration:             2 * time.Second,
		SkipFailedRequests:     false,
		SkipSuccessfulRequests: false,
		Storage:                memory.New(),
		LimiterMiddleware:      SlidingWindow{},
	}))

	app.Get("/:status", func(c fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	sleepForRetryAfter(t, resp)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

func Test_Limiter_Sliding_Window_RecalculatesAfterHandlerDelay(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:               2,
		Expiration:        time.Second,
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		time.Sleep(600 * time.Millisecond)
		return c.SendStatus(fiber.StatusOK)
	})

	for range 2 {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	}

	time.Sleep(2*time.Second + 100*time.Millisecond)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "2", resp.Header.Get(xRateLimitLimit))
	require.Equal(t, "1", resp.Header.Get(xRateLimitRemaining))
	require.NotEmpty(t, resp.Header.Get(xRateLimitReset))
}

func Test_Limiter_Sliding_Window_ExpiresStalePrevHits(t *testing.T) {
	t.Parallel()
	clock := newTestClock(time.Now().Truncate(time.Second))
	app := fiber.New()

	app.Use(New(Config{
		Max:               1,
		Expiration:        time.Second,
		clock:             clock.Now,
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Advance two full windows so the previous-window hits age out completely.
	clock.Add(2500 * time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "0", resp.Header.Get(xRateLimitRemaining))
}

func Test_Limiter_Sliding_Window_SkipFailedRequests_DecrementsPreviousWindow(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                2,
		Expiration:         200 * time.Millisecond,
		SkipFailedRequests: true,
		LimiterMiddleware:  SlidingWindow{},
	}))

	app.Get("/:mode", func(c fiber.Ctx) error {
		if c.Params("mode") == "fail" {
			time.Sleep(300 * time.Millisecond)
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	type respErr struct {
		resp *http.Response
		err  error
	}
	failCh := make(chan respErr, 1)

	go func() {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
		failCh <- respErr{resp: resp, err: err}
	}()

	time.Sleep(220 * time.Millisecond)

	successResp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/ok", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, successResp.StatusCode)

	result := <-failCh
	require.NoError(t, result.err)
	require.Equal(t, fiber.StatusInternalServerError, result.resp.StatusCode)
	require.Equal(t, "2", result.resp.Header.Get(xRateLimitLimit))
	require.Equal(t, "1", result.resp.Header.Get(xRateLimitRemaining))
	assert.NotEmpty(t, result.resp.Header.Get(xRateLimitReset))
}

// go test -run Test_Limiter_Fixed_Window_SkipSuccessfulRequests_DoesNotCreditNextWindow -v
func Test_Limiter_Fixed_Window_SkipSuccessfulRequests_DoesNotCreditNextWindow(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                    2,
		Expiration:             300 * time.Millisecond,
		SkipSuccessfulRequests: true,
		LimiterMiddleware:      FixedWindow{},
	}))

	app.Get("/:mode", func(c fiber.Ctx) error {
		switch c.Params("mode") {
		case "slow":
			// Hold the successful request open until the window has rolled over.
			time.Sleep(600 * time.Millisecond)
			return c.SendStatus(fiber.StatusOK)
		case "fail":
			// A failed request is not skipped, so it seeds the new window.
			return c.SendStatus(fiber.StatusInternalServerError)
		default:
			return c.SendStatus(fiber.StatusOK)
		}
	})

	type respErr struct {
		resp *http.Response
		err  error
	}
	slowCh := make(chan respErr, 1)

	// The slow successful request increments currHits in the first window and
	// only finishes after the window has rolled over. With SkipSuccessfulRequests
	// it credits its hit back; the credit must not be applied to the new window,
	// otherwise its currHits underflows and the next window grants extra requests.
	go func() {
		resp, err := app.Test(
			httptest.NewRequest(fiber.MethodGet, "/slow", http.NoBody),
			fiber.TestConfig{Timeout: 2 * time.Second},
		)
		slowCh <- respErr{resp: resp, err: err}
	}()

	// Let the first window expire, then seed the new window with one counted hit.
	time.Sleep(400 * time.Millisecond)
	failResp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, failResp.StatusCode)

	result := <-slowCh
	require.NoError(t, result.err)
	require.Equal(t, fiber.StatusOK, result.resp.StatusCode)
	require.Equal(t, "2", result.resp.Header.Get(xRateLimitLimit))
	// The credit is skipped after the rollover, so remaining reflects the new
	// window's single counted hit (2 - 1). Before the fix the slow request
	// wrongly decremented the new window's currHits and this read "2".
	require.Equal(t, "1", result.resp.Header.Get(xRateLimitRemaining))
	assert.NotEmpty(t, result.resp.Header.Get(xRateLimitReset))
}

// go test -run Test_Limiter_Fixed_Window_Skip_Failed_Requests -v
func Test_Limiter_Fixed_Window_Skip_Failed_Requests(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                1,
		Expiration:         2 * time.Second,
		SkipFailedRequests: true,
		LimiterMiddleware:  FixedWindow{},
	}))

	app.Get("/:status", func(c fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

// go test -run Test_Limiter_Fixed_Window_Custom_Storage_Skip_Failed_Requests -v
func Test_Limiter_Fixed_Window_Custom_Storage_Skip_Failed_Requests(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                1,
		Expiration:         2 * time.Second,
		Storage:            memory.New(),
		SkipFailedRequests: true,
		LimiterMiddleware:  FixedWindow{},
	}))

	app.Get("/:status", func(c fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

// go test -run Test_Limiter_Sliding_Window_Skip_Failed_Requests -v
func Test_Limiter_Sliding_Window_Skip_Failed_Requests(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                1,
		Expiration:         2 * time.Second,
		SkipFailedRequests: true,
		LimiterMiddleware:  SlidingWindow{},
	}))

	app.Get("/:status", func(c fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	sleepForRetryAfter(t, resp)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

// go test -run Test_Limiter_Sliding_Window_Custom_Storage_Skip_Failed_Requests -v
func Test_Limiter_Sliding_Window_Custom_Storage_Skip_Failed_Requests(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                1,
		Expiration:         2 * time.Second,
		Storage:            memory.New(),
		SkipFailedRequests: true,
		LimiterMiddleware:  SlidingWindow{},
	}))

	app.Get("/:status", func(c fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	sleepForRetryAfter(t, resp)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

// go test -run Test_Limiter_Fixed_Window_Skip_Successful_Requests -v
func Test_Limiter_Fixed_Window_Skip_Successful_Requests(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                    1,
		Expiration:             2 * time.Second,
		SkipSuccessfulRequests: true,
		LimiterMiddleware:      FixedWindow{},
	}))

	app.Get("/:status", func(c fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)
}

// go test -run Test_Limiter_Fixed_Window_Custom_Storage_Skip_Successful_Requests -v
func Test_Limiter_Fixed_Window_Custom_Storage_Skip_Successful_Requests(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                    1,
		Expiration:             2 * time.Second,
		Storage:                memory.New(),
		SkipSuccessfulRequests: true,
		LimiterMiddleware:      FixedWindow{},
	}))

	app.Get("/:status", func(c fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)
}

// go test -run Test_Limiter_Sliding_Window_Skip_Successful_Requests -v
func Test_Limiter_Sliding_Window_Skip_Successful_Requests(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                    1,
		Expiration:             2 * time.Second,
		SkipSuccessfulRequests: true,
		LimiterMiddleware:      SlidingWindow{},
	}))

	app.Get("/:status", func(c fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	sleepForRetryAfter(t, resp)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)
}

// go test -run Test_Limiter_Sliding_Window_Custom_Storage_Skip_Successful_Requests -v
func Test_Limiter_Sliding_Window_Custom_Storage_Skip_Successful_Requests(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:                    1,
		Expiration:             2 * time.Second,
		Storage:                memory.New(),
		SkipSuccessfulRequests: true,
		LimiterMiddleware:      SlidingWindow{},
	}))

	app.Get("/:status", func(c fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/success", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	sleepForRetryAfter(t, resp)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)
}

// go test -v -run=^$ -bench=Benchmark_Limiter_Custom_Store -benchmem -count=4
func Benchmark_Limiter_Custom_Store(b *testing.B) {
	app := fiber.New()

	app.Use(New(Config{
		Max:        100,
		Expiration: 60 * time.Second,
		Storage:    memory.New(),
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")

	for b.Loop() {
		h(fctx)
	}
}

// Test to reproduce the bug where fiber.NewErrorf responses are not counted as failed requests
func Test_Limiter_Bug_NewErrorf_SkipSuccessfulRequests_SlidingWindow(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Max:                    1,
		Expiration:             60 * time.Second,
		LimiterMiddleware:      SlidingWindow{},
		SkipSuccessfulRequests: true,
		SkipFailedRequests:     false,
		DisableHeaders:         true,
	}))

	app.Get("/", func(_ fiber.Ctx) error {
		return fiber.NewErrorf(fiber.StatusInternalServerError, "Error")
	})

	// First request should succeed (and be counted because it's a failed request)
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

	// Second request should be rate limited because the first failed request was counted
	// But currently this is not happening due to the bug
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	// This should be 429 (rate limited) but currently returns 500 due to the bug
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode, "Second request should be rate limited")
}

// Test to reproduce the bug where fiber.NewErrorf responses are not counted as failed requests (FixedWindow)
func Test_Limiter_Bug_NewErrorf_SkipSuccessfulRequests_FixedWindow(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Max:                    1,
		Expiration:             60 * time.Second,
		LimiterMiddleware:      FixedWindow{},
		SkipSuccessfulRequests: true,
		SkipFailedRequests:     false,
		DisableHeaders:         true,
	}))

	app.Get("/", func(_ fiber.Ctx) error {
		return fiber.NewErrorf(fiber.StatusInternalServerError, "Error")
	})

	// First request should succeed (and be counted because it's a failed request)
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

	// Second request should be rate limited because the first failed request was counted
	// But currently this is not happening due to the bug
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	// This should be 429 (rate limited) but currently returns 500 due to the bug
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode, "Second request should be rate limited")
}

// go test -run Test_Limiter_Next
func Test_Limiter_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_Limiter_Headers(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:        50,
		Expiration: 2 * time.Second,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello tester!")
	})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")

	app.Handler()(fctx)

	require.Equal(t, "50", string(fctx.Response.Header.Peek("X-RateLimit-Limit")))
	if v := string(fctx.Response.Header.Peek("X-RateLimit-Remaining")); v == "" {
		t.Error("The X-RateLimit-Remaining header is not set correctly - value is an empty string.")
	}
	if v := string(fctx.Response.Header.Peek("X-RateLimit-Reset")); (v != "1") && (v != "2") {
		t.Error("The X-RateLimit-Reset header is not set correctly - value is out of bounds.")
	}
}

func Test_Limiter_Disable_Headers(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:            1,
		Expiration:     2 * time.Second,
		DisableHeaders: true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello tester!")
	})

	// first request should pass
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")

	app.Handler()(fctx)

	require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode())
	require.Equal(t, "Hello tester!", string(fctx.Response.Body()))
	require.Empty(t, string(fctx.Response.Header.Peek("X-RateLimit-Limit")))
	require.Empty(t, string(fctx.Response.Header.Peek("X-RateLimit-Remaining")))
	require.Empty(t, string(fctx.Response.Header.Peek("X-RateLimit-Reset")))

	// second request should hit the limit and return 429 without headers
	fctx2 := &fasthttp.RequestCtx{}
	fctx2.Request.Header.SetMethod(fiber.MethodGet)
	fctx2.Request.SetRequestURI("/")

	app.Handler()(fctx2)

	require.Equal(t, fiber.StatusTooManyRequests, fctx2.Response.StatusCode())
	require.Empty(t, string(fctx2.Response.Header.Peek(fiber.HeaderRetryAfter)))
	require.Empty(t, string(fctx2.Response.Header.Peek("X-RateLimit-Limit")))
	require.Empty(t, string(fctx2.Response.Header.Peek("X-RateLimit-Remaining")))
	require.Empty(t, string(fctx2.Response.Header.Peek("X-RateLimit-Reset")))
}

// go test -v -run=^$ -bench=Benchmark_Limiter -benchmem -count=4
func Benchmark_Limiter(b *testing.B) {
	app := fiber.New()

	app.Use(New(Config{
		Max:        100,
		Expiration: 60 * time.Second,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")

	for b.Loop() {
		h(fctx)
	}
}

// go test -run Test_Sliding_Window -race -v
func Test_Sliding_Window(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Max:               10,
		Expiration:        1 * time.Second,
		Storage:           memory.New(),
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello tester!")
	})

	singleRequest := func(shouldFail bool) {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		if shouldFail {
			require.NoError(t, err)
			require.Equal(t, 429, resp.StatusCode)
		} else {
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
		}
	}

	for range 5 {
		singleRequest(false)
	}

	time.Sleep(3 * time.Second)

	for range 5 {
		singleRequest(false)
	}

	time.Sleep(3 * time.Second)

	for range 5 {
		singleRequest(false)
	}

	time.Sleep(3 * time.Second)

	for range 10 {
		singleRequest(false)
	}

	// requests should fail now
	for range 5 {
		singleRequest(true)
	}
}

// --- Unit tests for internal sliding-window helpers ---

func Test_rotateWindow_FreshEntry(t *testing.T) {
	t.Parallel()
	e := &item{}
	resetInSec := rotateWindow(e, 1000, 60)
	require.Equal(t, uint64(1060), e.exp)
	require.Equal(t, uint64(60), resetInSec)
	require.Equal(t, 0, e.currHits)
	require.Equal(t, 0, e.prevHits)
}

func Test_rotateWindow_WithinCurrentWindow(t *testing.T) {
	t.Parallel()
	e := &item{exp: 1060, currHits: 3, prevHits: 5}
	resetInSec := rotateWindow(e, 1020, 60)
	require.Equal(t, uint64(1060), e.exp)
	require.Equal(t, uint64(40), resetInSec)
	require.Equal(t, 3, e.currHits)
	require.Equal(t, 5, e.prevHits)
}

func Test_rotateWindow_FullExpiration(t *testing.T) {
	t.Parallel()
	e := &item{exp: 1000, currHits: 3, prevHits: 5}
	// elapsed = 1120 - 1000 = 120, expiration = 60, elapsed >= expiration
	resetInSec := rotateWindow(e, 1120, 60)
	require.Equal(t, uint64(1180), e.exp)
	require.Equal(t, uint64(60), resetInSec)
	require.Equal(t, 0, e.currHits)
	require.Equal(t, 0, e.prevHits)
}

func Test_rotateWindow_PartialExpiration(t *testing.T) {
	t.Parallel()
	e := &item{exp: 1000, currHits: 3, prevHits: 5}
	// elapsed = 1020 - 1000 = 20, expiration = 60, elapsed < expiration
	resetInSec := rotateWindow(e, 1020, 60)
	require.Equal(t, uint64(1060), e.exp) // ts + expiration - elapsed = 1020+60-20 = 1060
	require.Equal(t, uint64(40), resetInSec)
	require.Equal(t, 0, e.currHits)
	require.Equal(t, 3, e.prevHits)
}

func Test_bucketForOriginalHit_CurrentWindow(t *testing.T) {
	t.Parallel()
	e := &item{currHits: 5, prevHits: 3}
	// ts < requestExpiration → returns &currHits
	counter := bucketForOriginalHit(e, 1060, 1050, 60)
	require.NotNil(t, counter)
	require.Equal(t, 5, *counter)
	*counter--
	require.Equal(t, 4, e.currHits)
}

func Test_bucketForOriginalHit_PreviousWindow(t *testing.T) {
	t.Parallel()
	e := &item{currHits: 5, prevHits: 3}
	// ts >= requestExpiration AND ts - requestExpiration < expiration → returns &prevHits
	counter := bucketForOriginalHit(e, 1060, 1080, 60)
	require.NotNil(t, counter)
	require.Equal(t, 3, *counter)
	*counter--
	require.Equal(t, 2, e.prevHits)
}

func Test_bucketForOriginalHit_Expired(t *testing.T) {
	t.Parallel()
	e := &item{currHits: 5, prevHits: 3}
	// ts - requestExpiration >= expiration → returns nil
	counter := bucketForOriginalHit(e, 1000, 1200, 60)
	require.Nil(t, counter)
}

func Test_ttlDuration_Normal(t *testing.T) {
	t.Parallel()
	d := ttlDuration(10, 60)
	require.Equal(t, 70*time.Second, d)
}

func Test_ttlDuration_ResetOverflow(t *testing.T) {
	t.Parallel()
	d := ttlDuration(math.MaxUint64, 60)
	require.Equal(t, time.Duration(math.MaxInt64), d)
}

func Test_ttlDuration_ExpirationOverflow(t *testing.T) {
	t.Parallel()
	d := ttlDuration(10, math.MaxUint64)
	require.Equal(t, time.Duration(math.MaxInt64), d)
}

func Test_ttlDuration_SumOverflow(t *testing.T) {
	t.Parallel()
	// Use values that individually fit but their sum overflows
	maxSec := uint64(math.MaxInt64 / int64(time.Second))
	d := ttlDuration(maxSec, maxSec)
	require.Equal(t, time.Duration(math.MaxInt64), d)
}

func Test_secondsToDuration_Normal(t *testing.T) {
	t.Parallel()
	d, ok := secondsToDuration(30)
	require.True(t, ok)
	require.Equal(t, 30*time.Second, d)
}

func Test_secondsToDuration_Overflow(t *testing.T) {
	t.Parallel()
	d, ok := secondsToDuration(math.MaxUint64)
	require.False(t, ok)
	require.Equal(t, time.Duration(math.MaxInt64), d)
}

func TestLimiterSlidingStorageGetError(t *testing.T) {
	t.Parallel()

	storage := newFailingLimiterStorage()
	storage.errs["get|"+testLimiterClientKey] = errors.New("boom")

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	})

	app.Use(New(Config{
		Storage:           storage,
		Max:               1,
		Expiration:        time.Second,
		LimiterMiddleware: SlidingWindow{},
		KeyGenerator:      func(fiber.Ctx) string { return testLimiterClientKey },
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, "limiter: failed to get key")
}

func TestLimiterSlidingStorageSetError(t *testing.T) {
	t.Parallel()

	storage := newFailingLimiterStorage()
	storage.errs["set|"+testLimiterClientKey] = errors.New("boom")

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	})

	app.Use(New(Config{
		Storage:           storage,
		Max:               1,
		Expiration:        time.Second,
		LimiterMiddleware: SlidingWindow{},
		KeyGenerator:      func(fiber.Ctx) string { return testLimiterClientKey },
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, "limiter: failed to persist state")
}

func TestLimiterSlidingStorageSetErrorOnPostUpdate(t *testing.T) {
	t.Parallel()

	storage := newFailingLimiterStorage()

	// We need to fail on the second set call (post-handler update).
	// Use a custom storage that tracks set calls.
	customStorage := &countingSetStorage{
		failingLimiterStorage: storage,
		failOnSetN:            2,
		setErr:                errors.New("boom"),
	}

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	})

	app.Use(New(Config{
		Storage:            customStorage,
		Max:                10,
		Expiration:         time.Second,
		LimiterMiddleware:  SlidingWindow{},
		SkipFailedRequests: true,
		KeyGenerator:       func(fiber.Ctx) string { return testLimiterClientKey },
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusInternalServerError)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, "limiter: failed to persist state")
}

func TestLimiterSlidingStorageGetErrorOnPostUpdate(t *testing.T) {
	t.Parallel()

	customStorage := &countingGetStorage{
		failingLimiterStorage: newFailingLimiterStorage(),
		failOnGetN:            2,
		getErr:                errors.New("boom"),
	}

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
		},
	})

	app.Use(New(Config{
		Storage:            customStorage,
		Max:                10,
		Expiration:         time.Second,
		LimiterMiddleware:  SlidingWindow{},
		SkipFailedRequests: true,
		KeyGenerator:       func(fiber.Ctx) string { return testLimiterClientKey },
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusInternalServerError)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Error(t, captured)
}

func TestLimiterSlidingNextSkipsMiddleware(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Max:               1,
		Expiration:        time.Second,
		LimiterMiddleware: SlidingWindow{},
		Next: func(c fiber.Ctx) bool {
			return c.Path() == "/skip"
		},
	}))

	app.Get("/skip", func(c fiber.Ctx) error {
		return c.SendString("skipped")
	})
	app.Get("/normal", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Skipped path should always succeed even beyond limit
	for range 5 {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/skip", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	}

	// Normal path should be limited after 1 request
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/normal", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/normal", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
}

// countingSetStorage wraps failingLimiterStorage and fails on the Nth set call.
type countingSetStorage struct {
	setErr error
	*failingLimiterStorage
	setCalls   int
	failOnSetN int
	mu         sync.Mutex
}

func (s *countingSetStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.mu.Lock()
	s.setCalls++
	n := s.setCalls
	s.mu.Unlock()
	if n == s.failOnSetN {
		return s.setErr
	}
	return s.failingLimiterStorage.SetWithContext(ctx, key, val, exp)
}

func (s *countingSetStorage) Set(key string, val []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, exp)
}

// countingGetStorage wraps failingLimiterStorage and fails on the Nth get call.
type countingGetStorage struct {
	getErr error
	*failingLimiterStorage
	getCalls   int
	failOnGetN int
	mu         sync.Mutex
}

func (s *countingGetStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	s.getCalls++
	n := s.getCalls
	s.mu.Unlock()
	if n == s.failOnGetN {
		return nil, s.getErr
	}
	return s.failingLimiterStorage.GetWithContext(ctx, key)
}

func (s *countingGetStorage) Get(key string) ([]byte, error) {
	return s.GetWithContext(context.Background(), key)
}

// Test_Config_currentSecond_NegativeClock verifies that an injected pre-epoch
// (negative Unix) timestamp is clamped to 0 instead of wrapping to a huge
// uint64, while a non-negative timestamp is converted unchanged.
func Test_Config_currentSecond_NegativeClock(t *testing.T) {
	t.Parallel()

	cfg := &Config{clock: func() time.Time { return time.Unix(-100, 0) }}
	require.Equal(t, uint64(0), cfg.currentSecond())

	cfg.clock = func() time.Time { return time.Unix(42, 0) }
	require.Equal(t, uint64(42), cfg.currentSecond())
}
