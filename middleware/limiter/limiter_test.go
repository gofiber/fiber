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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/clocktest"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/utils/v2"
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

// fakeLimiterStorage ignores TTLs. An operation fails when errs has "get|<key>" or "set|<key>".
type fakeLimiterStorage struct {
	data map[string][]byte
	errs map[string]error
}

const testLimiterClientKey = "client-key"

var windowStrategies = []struct {
	handler Handler
	name    string
}{
	{name: "fixed", handler: FixedWindow{}},
	{name: "sliding", handler: SlidingWindow{}},
}

type typedNilLimiterError struct {
	message string
}

func (e *typedNilLimiterError) Error() string {
	return e.message
}

func newFakeLimiterStorage() *fakeLimiterStorage {
	return &fakeLimiterStorage{
		data: make(map[string][]byte),
		errs: make(map[string]error),
	}
}

// countingFailStorage fails set operations, and optionally get operations,
// after a specified number of successful calls
type countingFailStorage struct {
	*fakeLimiterStorage
	setFailErr    error
	getFailErr    error
	setCount      int
	failAfterN    int
	getCount      int
	getFailAfterN int
}

func newCountingFailStorage(failAfterN int, err error) *countingFailStorage {
	return &countingFailStorage{
		fakeLimiterStorage: newFakeLimiterStorage(),
		failAfterN:         failAfterN,
		setFailErr:         err,
	}
}

func (s *countingFailStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	s.getCount++
	if s.getFailErr != nil && s.getCount > s.getFailAfterN {
		return nil, s.getFailErr
	}
	return s.fakeLimiterStorage.GetWithContext(ctx, key)
}

func (s *countingFailStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.setCount++
	if s.setCount > s.failAfterN {
		return s.setFailErr
	}
	return s.fakeLimiterStorage.SetWithContext(ctx, key, val, exp)
}

type contextRecord struct {
	key      string
	value    string
	canceled bool
}

type contextRecorderLimiterStorage struct {
	*fakeLimiterStorage
	gets []contextRecord
	sets []contextRecord
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

	// Window accounting reads the cached second clock, so wall time alone is not
	// enough: a late timestamp updater keeps the old window current.
	clocktest.SleepPast(t, delay)
}

func newContextRecorderLimiterStorage() *contextRecorderLimiterStorage {
	return &contextRecorderLimiterStorage{fakeLimiterStorage: newFakeLimiterStorage()}
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
	return s.fakeLimiterStorage.GetWithContext(ctx, key)
}

func (s *contextRecorderLimiterStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.sets = append(s.sets, contextRecordFrom(ctx, key))
	return s.fakeLimiterStorage.SetWithContext(ctx, key, val, exp)
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

func (s *fakeLimiterStorage) GetWithContext(_ context.Context, key string) ([]byte, error) {
	if err, ok := s.errs["get|"+key]; ok && err != nil {
		return nil, err
	}
	if val, ok := s.data[key]; ok {
		return append([]byte(nil), val...), nil
	}
	return nil, nil
}

func (s *fakeLimiterStorage) Get(key string) ([]byte, error) {
	return s.GetWithContext(context.Background(), key)
}

func (s *fakeLimiterStorage) SetWithContext(_ context.Context, key string, val []byte, _ time.Duration) error {
	if err, ok := s.errs["set|"+key]; ok && err != nil {
		return err
	}
	s.data[key] = append([]byte(nil), val...)
	return nil
}

func (s *fakeLimiterStorage) Set(key string, val []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, exp)
}

func (*fakeLimiterStorage) DeleteWithContext(context.Context, string) error { return nil }

func (*fakeLimiterStorage) Delete(string) error { return nil }

func (*fakeLimiterStorage) ResetWithContext(context.Context) error { return nil }

func (*fakeLimiterStorage) Reset() error { return nil }

func (*fakeLimiterStorage) Close() error { return nil }

// expiringStorage drops entries once their TTL has passed on the test clock.
type expiringStorage struct {
	*fakeLimiterStorage
	clock   *testClock
	expires map[string]time.Time
}

func (s *expiringStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	if exp, ok := s.expires[key]; ok && !s.clock.Now().Before(exp) {
		delete(s.data, key)
		delete(s.expires, key)
	}
	return s.fakeLimiterStorage.GetWithContext(ctx, key)
}

func (s *expiringStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.expires[key] = s.clock.Now().Add(exp)
	return s.fakeLimiterStorage.SetWithContext(ctx, key, val, exp)
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

func Test_Limiter_DefaultConfigNoPanic(t *testing.T) {
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

func Test_getEffectiveStatusCode_FiberError(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

	c.Response().SetStatusCode(fiber.StatusOK)

	// *fiber.Error: its code.
	require.Equal(t, fiber.StatusTeapot, getEffectiveStatusCode(c, fiber.NewError(fiber.StatusTeapot, "teapot")))

	// Plain error: an error status already set stays.
	c.Response().SetStatusCode(fiber.StatusBadGateway)
	require.Equal(t, fiber.StatusBadGateway, getEffectiveStatusCode(c, errors.New("generic")))

	// Plain error: 500.
	for _, status := range []int{fiber.StatusOK, fiber.StatusFound} {
		c.Response().SetStatusCode(status)
		require.Equal(t, fiber.StatusInternalServerError, getEffectiveStatusCode(c, errors.New("generic")), "status %d", status)
	}
}

func Test_getEffectiveStatusCode_TypedNilFiberError(t *testing.T) {
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

func Test_getEffectiveStatusCode_TypedNilCustomError(t *testing.T) {
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

func Test_Limiter_StorageErrorMessages(t *testing.T) {
	t.Parallel()

	steps := []struct {
		name string
		want []string
	}{
		{name: "get", want: []string{"limiter: failed to get key"}},
		{name: "set", want: []string{"limiter: failed to persist state", "limiter: failed to store key"}},
	}

	for _, strategy := range windowStrategies {
		for _, step := range steps {
			for _, disableRedaction := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/DisableValueRedaction=%t", strategy.name, step.name, disableRedaction), func(t *testing.T) {
					t.Parallel()

					storage := newFakeLimiterStorage()
					storage.errs[step.name+"|"+testLimiterClientKey] = errors.New("boom")

					var captured error
					app := fiber.New(fiber.Config{
						ErrorHandler: func(c fiber.Ctx, err error) error {
							captured = err
							return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
						},
					})

					app.Use(New(Config{
						DisableValueRedaction: disableRedaction,
						LimiterMiddleware:     strategy.handler,
						Storage:               storage,
						Max:                   1,
						Expiration:            time.Second,
						KeyGenerator:          func(fiber.Ctx) string { return testLimiterClientKey },
					}))
					app.Get("/", func(c fiber.Ctx) error {
						return c.SendString("ok")
					})

					resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
					require.NoError(t, err)
					require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
					require.Error(t, captured)
					for _, want := range step.want {
						require.ErrorContains(t, captured, want)
					}
					if disableRedaction {
						require.ErrorContains(t, captured, testLimiterClientKey)
						require.NotContains(t, captured.Error(), redactedKey)
					} else {
						require.ErrorContains(t, captured, redactedKey)
						require.NotContains(t, captured.Error(), testLimiterClientKey)
					}
				})
			}
		}
	}
}

func requireContextRecords(t *testing.T, records []contextRecord, key, wantValue string, wantCanceled bool) {
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

func Test_Limiter_PropagatesRequestContextToStorage(t *testing.T) {
	t.Parallel()

	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()

			storage := newContextRecorderLimiterStorage()

			app := fiber.New()

			app.Use(func(c fiber.Ctx) error {
				path := c.Path()
				if path == "/normal" {
					c.SetContext(contextWithMarker(strategy.name + "-normal"))
				}
				if path == "/rollback" {
					c.SetContext(canceledContextWithMarker(strategy.name + "-rollback"))
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
				LimiterMiddleware: strategy.handler,
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

			requireContextRecords(t, gets, "/normal", strategy.name+"-normal", false)
			requireContextRecords(t, gets, "/rollback", strategy.name+"-rollback", true)
			requireContextRecords(t, sets, "/normal", strategy.name+"-normal", false)
			requireContextRecords(t, sets, "/rollback", strategy.name+"-rollback", true)
		})
	}
}

// Every storage error path must release the key's shard, or the next request
// for that key hangs.
func Test_Limiter_StorageErrorsReleaseShard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		newStorage     func() fiber.Storage
		middleware     Handler
		name           string
		wantErr        string
		skipSuccessful bool
	}{
		{
			name:       "sliding first get",
			middleware: SlidingWindow{},
			newStorage: func() fiber.Storage {
				st := newFakeLimiterStorage()
				st.errs["get|"+testLimiterClientKey] = errors.New("boom")
				return st
			},
			wantErr: "limiter: failed to get key",
		},
		{
			name:       "sliding first set",
			middleware: SlidingWindow{},
			newStorage: func() fiber.Storage {
				st := newFakeLimiterStorage()
				st.errs["set|"+testLimiterClientKey] = errors.New("boom")
				return st
			},
			wantErr: "limiter: failed to persist state",
		},
		{
			name:       "sliding header get",
			middleware: SlidingWindow{},
			newStorage: func() fiber.Storage {
				st := newCountingFailStorage(0, nil)
				st.getFailErr = errors.New("second get failed")
				st.getFailAfterN = 1
				return st
			},
			wantErr: "limiter: failed to get key",
		},
		{
			name:       "sliding header set",
			middleware: SlidingWindow{},
			newStorage: func() fiber.Storage {
				return newCountingFailStorage(1, errors.New("second set failed"))
			},
			wantErr: "limiter: failed to persist state",
		},
		{
			name:       "fixed first get",
			middleware: FixedWindow{},
			newStorage: func() fiber.Storage {
				st := newFakeLimiterStorage()
				st.errs["get|"+testLimiterClientKey] = errors.New("boom")
				return st
			},
			wantErr: "limiter: failed to get key",
		},
		{
			name:       "fixed first set",
			middleware: FixedWindow{},
			newStorage: func() fiber.Storage {
				st := newFakeLimiterStorage()
				st.errs["set|"+testLimiterClientKey] = errors.New("boom")
				return st
			},
			wantErr: "limiter: failed to persist state",
		},
		{
			name:       "fixed skip get",
			middleware: FixedWindow{},
			newStorage: func() fiber.Storage {
				st := newCountingFailStorage(0, nil)
				st.getFailErr = errors.New("second get failed")
				st.getFailAfterN = 1
				return st
			},
			wantErr:        "limiter: failed to get key",
			skipSuccessful: true,
		},
		{
			name:       "fixed skip set",
			middleware: FixedWindow{},
			newStorage: func() fiber.Storage {
				return newCountingFailStorage(1, errors.New("second set failed"))
			},
			wantErr:        "limiter: failed to persist state",
			skipSuccessful: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var captured error
			app := fiber.New(fiber.Config{
				ErrorHandler: func(c fiber.Ctx, err error) error {
					captured = err
					return c.Status(fiber.StatusInternalServerError).SendString("storage failure")
				},
			})

			app.Use(New(Config{
				Storage:                tc.newStorage(),
				LimiterMiddleware:      tc.middleware,
				Max:                    10,
				Expiration:             time.Second,
				SkipSuccessfulRequests: tc.skipSuccessful,
				KeyGenerator:           func(fiber.Ctx) string { return testLimiterClientKey },
			}))
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
			require.ErrorContains(t, captured, tc.wantErr)

			// A shard left locked by the error path would hang this same-key request.
			resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
		})
	}
}

type skipStep struct {
	path string
	want int
}

// go test -run Test_Limiter_Skip_Choices -v
func Test_Limiter_Skip_Choices(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		name  string
		steps []skipStep
		after skipStep // asked after the window; no path means no wait
		cfg   Config
	}{
		{
			name:  "no_skip",
			cfg:   Config{Max: 2},
			steps: []skipStep{{"/fail", fiber.StatusBadRequest}, {"/success", fiber.StatusOK}, {"/success", fiber.StatusTooManyRequests}},
			after: skipStep{"/success", fiber.StatusOK},
		},
		{
			name:  "skip_failed",
			cfg:   Config{Max: 1, SkipFailedRequests: true},
			steps: []skipStep{{"/fail", fiber.StatusBadRequest}, {"/success", fiber.StatusOK}, {"/success", fiber.StatusTooManyRequests}},
			after: skipStep{"/success", fiber.StatusOK},
		},
		{
			name:  "skip_successful",
			cfg:   Config{Max: 1, SkipSuccessfulRequests: true},
			steps: []skipStep{{"/success", fiber.StatusOK}, {"/fail", fiber.StatusBadRequest}, {"/fail", fiber.StatusTooManyRequests}},
			after: skipStep{"/fail", fiber.StatusBadRequest},
		},
		{
			name:  "max_func_zero",
			cfg:   Config{MaxFunc: func(fiber.Ctx) int { return 0 }},
			steps: []skipStep{{"/fail", fiber.StatusBadRequest}, {"/success", fiber.StatusOK}, {"/success", fiber.StatusOK}},
		},
	}

	strategies := []struct {
		handler Handler
		wait    func(t *testing.T, resp *http.Response)
		name    string
	}{
		{name: "fixed", handler: FixedWindow{}, wait: func(*testing.T, *http.Response) { time.Sleep(3 * time.Second) }},
		{name: "sliding", handler: SlidingWindow{}, wait: sleepForRetryAfter},
	}

	stores := []struct {
		storage func() fiber.Storage
		name    string
	}{
		{name: "default_store", storage: func() fiber.Storage { return nil }},
		{name: "custom_store", storage: func() fiber.Storage { return memory.New() }},
	}

	for _, scenario := range scenarios {
		for _, strategy := range strategies {
			for _, store := range stores {
				t.Run(scenario.name+"/"+strategy.name+"/"+store.name, func(t *testing.T) {
					t.Parallel()

					cfg := scenario.cfg
					cfg.Expiration = 2 * time.Second
					cfg.LimiterMiddleware = strategy.handler
					cfg.Storage = store.storage()

					app := fiber.New()
					app.Use(New(cfg))
					app.Get("/:status", func(c fiber.Ctx) error {
						if c.Params("status") == "fail" {
							return c.SendStatus(fiber.StatusBadRequest)
						}
						return c.SendStatus(fiber.StatusOK)
					})

					var resp *http.Response
					for _, step := range scenario.steps {
						var err error
						resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, step.path, http.NoBody))
						require.NoError(t, err)
						require.Equal(t, step.want, resp.StatusCode, step.path)
					}

					if scenario.after.path == "" {
						return
					}

					strategy.wait(t, resp)

					resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, scenario.after.path, http.NoBody))
					require.NoError(t, err)
					require.Equal(t, scenario.after.want, resp.StatusCode)
				})
			}
		}
	}
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
	const maxRequests = 10

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

	requireBurstLimited(t, app, maxRequests)
}

// go test -run Test_Limiter_ExpirationFuncOverridesStaticExpiration -race -v
func Test_Limiter_ExpirationFuncOverridesStaticExpiration(t *testing.T) {
	t.Parallel()

	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()

			clock := newTestClock(time.Now().Truncate(time.Second))
			app := fiber.New()

			app.Use(New(Config{
				Max:               2,
				Expiration:        10 * time.Second,
				ExpirationFunc:    func(_ fiber.Ctx) time.Duration { return 2 * time.Second },
				clock:             clock.Now,
				Storage:           newFakeLimiterStorage(),
				LimiterMiddleware: strategy.handler,
			}))

			app.Get("/", func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			for _, want := range []int{fiber.StatusOK, fiber.StatusOK, fiber.StatusTooManyRequests} {
				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, want, resp.StatusCode)
			}

			// Two 2s windows, far short of the 10s Expiration.
			clock.Add(5 * time.Second)

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
		})
	}
}

// Test_Limiter_ExpirationFunc_NonPositiveUsesConfiguredExpiration covers an
// ExpirationFunc that returns zero or less: the window is the configured
// Expiration, not the package default of one minute. Only the existing tests
// for this fallback checked that the limiter still limits, so a configured
// ten-second window was a minute long for those requests, and Retry-After and
// X-RateLimit-Reset said so.
func Test_Limiter_ExpirationFunc_NonPositiveUsesConfiguredExpiration(t *testing.T) {
	t.Parallel()

	const expiration = 10 * time.Second

	strategies := []struct {
		handler Handler
		// storage returns the store the strategy is tested with: the fixed window
		// keeps its entry in storage, whose own expiry would otherwise run on
		// real time while the test moves a fake clock.
		storage func() fiber.Storage
		name    string
	}{
		{name: "fixed", handler: FixedWindow{}, storage: func() fiber.Storage { return newFakeLimiterStorage() }},
		{name: "sliding", handler: SlidingWindow{}, storage: func() fiber.Storage { return nil }},
	}

	for _, strategy := range strategies {
		for _, returned := range []time.Duration{0, -time.Second} {
			t.Run(strategy.name+"/"+returned.String(), func(t *testing.T) {
				t.Parallel()

				clock := newTestClock(time.Now().Truncate(time.Second))
				app := fiber.New()
				app.Use(New(Config{
					Max:               1,
					Expiration:        expiration,
					ExpirationFunc:    func(fiber.Ctx) time.Duration { return returned },
					LimiterMiddleware: strategy.handler,
					clock:             clock.Now,
					Storage:           strategy.storage(),
				}))
				app.Get("/", func(c fiber.Ctx) error {
					return c.SendStatus(fiber.StatusOK)
				})

				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, fiber.StatusOK, resp.StatusCode)
				require.Equal(t, "10", resp.Header.Get(xRateLimitReset), "the window is the configured Expiration")

				resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
				require.Equal(t, "10", resp.Header.Get(fiber.HeaderRetryAfter))

				// Just over the configured window, the client is admitted again. Two
				// whole windows are needed for the sliding one to forget the last.
				clock.Add(2*expiration + time.Second)

				resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, fiber.StatusOK, resp.StatusCode)
			})
		}
	}
}

// Test_Limiter_ExpirationFunc_PositiveStillWins checks the other side of the
// fallback: a positive value from ExpirationFunc keeps overriding Expiration.
func Test_Limiter_ExpirationFunc_PositiveStillWins(t *testing.T) {
	t.Parallel()

	for name, handler := range map[string]Handler{"fixed": FixedWindow{}, "sliding": SlidingWindow{}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			clock := newTestClock(time.Now().Truncate(time.Second))
			app := fiber.New()
			app.Use(New(Config{
				Max:               5,
				Expiration:        time.Minute,
				ExpirationFunc:    func(fiber.Ctx) time.Duration { return 3 * time.Second },
				LimiterMiddleware: handler,
				clock:             clock.Now,
			}))
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, "3", resp.Header.Get(xRateLimitReset))
		})
	}
}

func Test_Limiter_SubSecondExpirationIsOneSecondWindow(t *testing.T) {
	t.Parallel()

	// A positive Expiration below one second is a one-second window.
	require.Equal(t, 500*time.Millisecond, configDefault(Config{Expiration: 500 * time.Millisecond}).Expiration)

	t.Run("fixed", func(t *testing.T) {
		t.Parallel()

		clock := newTestClock(time.Now().Truncate(time.Second))
		app := fiber.New()
		app.Use(New(Config{
			Max:               1,
			Expiration:        500 * time.Millisecond,
			LimiterMiddleware: FixedWindow{},
			clock:             clock.Now,
			Storage:           newFakeLimiterStorage(),
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
		require.Equal(t, "1", resp.Header.Get(fiber.HeaderRetryAfter))

		// Past the one-second window the client is admitted again.
		clock.Add(1300 * time.Millisecond)

		resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("sliding", func(t *testing.T) {
		t.Parallel()

		clock := newTestClock(time.Now().Truncate(time.Second))
		app := fiber.New()
		app.Use(New(Config{
			Max:               1,
			Expiration:        500 * time.Millisecond,
			LimiterMiddleware: SlidingWindow{},
			clock:             clock.Now,
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
		require.Equal(t, "1", resp.Header.Get(fiber.HeaderRetryAfter))

		// Two whole windows on, the previous window carries no weight.
		clock.Add(2300 * time.Millisecond)

		resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})
}

// requireBurstLimited sends n concurrent requests, expects a 429, then a 200 after the window.
func requireBurstLimited(t *testing.T, app *fiber.App, n int) {
	t.Helper()

	var wg sync.WaitGroup

	for range n {
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
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// go test -run Test_Limiter_Concurrency -race -v
func Test_Limiter_Concurrency(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		storage func() fiber.Storage
		name    string
	}{
		{name: "default_store", storage: func() fiber.Storage { return nil }},
		{name: "custom_store", storage: func() fiber.Storage { return memory.New() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{
				Max:        50,
				Expiration: 2 * time.Second,
				Storage:    tc.storage(),
			}))
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("Hello tester!")
			})

			requireBurstLimited(t, app, 50)
		})
	}
}

// stallingStorage parks Get for stallKey until release is closed.
type stallingStorage struct {
	fiber.Storage
	entered  chan struct{}
	release  chan struct{}
	stallKey string
	once     sync.Once
}

func (s *stallingStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	if key == s.stallKey {
		s.once.Do(func() { close(s.entered) })
		<-s.release
	}
	return s.Storage.GetWithContext(ctx, key)
}

// A key stuck in a storage round-trip must not hold up requests for other keys.
//
// go test -run Test_Limiter_Concurrency_UnrelatedKeys -race -v
func Test_Limiter_Concurrency_UnrelatedKeys(t *testing.T) {
	t.Parallel()

	fast := "fast"
	for i := 0; shardIndex(fast) == shardIndex("slow"); i++ {
		fast = "fast" + strconv.Itoa(i)
	}

	newReq := func(client string) *http.Request {
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set("X-Client", client)
		return req
	}

	for _, mw := range []Handler{FixedWindow{}, SlidingWindow{}} {
		st := &stallingStorage{Storage: memory.New(), stallKey: "slow", entered: make(chan struct{}), release: make(chan struct{})}
		app := fiber.New()
		app.Use(New(Config{
			Storage:           st,
			LimiterMiddleware: mw,
			KeyGenerator:      func(c fiber.Ctx) string { return c.Get("X-Client") },
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		})

		done := make(chan error, 1)
		go func() {
			_, err := app.Test(newReq("slow"), fiber.TestConfig{}) // no timeout, it waits for release
			done <- err
		}()
		select {
		case <-st.entered:
		case <-time.After(time.Second):
			t.Fatalf("%T: the slow request never reached the storage", mw)
		}

		resp, err := app.Test(newReq(fast))
		close(st.release)
		require.NoError(t, err, "%T: a request for an unrelated key waited on the stalled one", mw)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		require.NoError(t, <-done)
	}
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

	// Two full windows: the previous hits age out.
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

	clock := newTestClock(time.Now().Truncate(time.Second))
	app := fiber.New()

	app.Use(New(Config{
		Max:                    2,
		Expiration:             time.Second,
		SkipSuccessfulRequests: true,
		LimiterMiddleware:      FixedWindow{},
		clock:                  clock.Now,
		Storage:                newFakeLimiterStorage(),
	}))

	started := make(chan struct{})
	release := make(chan struct{})
	app.Get("/:mode", func(c fiber.Ctx) error {
		switch c.Params("mode") {
		case "slow":
			// Hold the successful request open until the window has rolled over.
			close(started)
			<-release
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
			fiber.TestConfig{Timeout: 5 * time.Second},
		)
		slowCh <- respErr{resp: resp, err: err}
	}()

	// Roll the window over only once the slow request holds a hit in the old one,
	// then seed the new window with one counted hit before releasing it.
	<-started
	clock.Add(1500 * time.Millisecond)
	failResp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, failResp.StatusCode)
	close(release)

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

// A returned *fiber.Error counts as failed, whatever status the response still carries.
func Test_Limiter_Bug_NewErrorf_SkipSuccessfulRequests(t *testing.T) {
	t.Parallel()

	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()

			app.Use(New(Config{
				Max:                    1,
				Expiration:             60 * time.Second,
				LimiterMiddleware:      strategy.handler,
				SkipSuccessfulRequests: true,
				SkipFailedRequests:     false,
				DisableHeaders:         true,
			}))

			app.Get("/", func(_ fiber.Ctx) error {
				return fiber.NewErrorf(fiber.StatusInternalServerError, "Error")
			})

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

			// The failed first request is counted, so the second is limited.
			resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode, "Second request should be rate limited")
		})
	}
}

// A Config given straight to a strategy, or none, gets the defaults New would give it.
func Test_Limiter_RawConfigUsesDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg   *Config
		name  string
		limit int
	}{
		{name: "nil", cfg: nil, limit: defaultLimiterMax},
		{name: "ConfigDefault", cfg: &ConfigDefault, limit: defaultLimiterMax},
		{name: "zero value", cfg: &Config{}, limit: defaultLimiterMax},
		{name: "only Max", cfg: &Config{Max: 2}, limit: 2},
		{name: "MaxFunc wins over Max", cfg: &Config{Max: 9, MaxFunc: func(fiber.Ctx) int { return 3 }}, limit: 3},
		{name: "only ExpirationFunc", cfg: &Config{ExpirationFunc: func(fiber.Ctx) time.Duration { return time.Hour }}, limit: defaultLimiterMax},
	}

	for _, strategy := range windowStrategies {
		for _, tc := range tests {
			t.Run(strategy.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				app := fiber.New()
				app.Use(strategy.handler.New(tc.cfg))
				app.Get("/", func(c fiber.Ctx) error {
					return c.SendStatus(fiber.StatusOK)
				})

				for i := range tc.limit {
					resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
					require.NoError(t, err)
					require.Equal(t, fiber.StatusOK, resp.StatusCode, "request %d", i+1)
					require.Equal(t, strconv.Itoa(tc.limit), resp.Header.Get(xRateLimitLimit), "request %d", i+1)
				}

				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
			})
		}
	}
}

// A strategy leaves the Config it is given alone: ConfigDefault is shared.
func Test_Limiter_RawConfigIsNotModified(t *testing.T) {
	t.Parallel()

	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()

			for _, cfg := range []*Config{{Max: 2}, &ConfigDefault} {
				before := *cfg
				strategy.handler.New(cfg)

				require.Nil(t, cfg.MaxFunc)
				require.Nil(t, cfg.ExpirationFunc)
				require.Equal(t, before.Max, cfg.Max)
				require.Equal(t, before.Expiration, cfg.Expiration)
				require.Equal(t, before.KeyGenerator == nil, cfg.KeyGenerator == nil)
				require.Equal(t, before.LimitReached == nil, cfg.LimitReached == nil)
			}
		})
	}
}

// Remaining reads 0 although rejected requests keep their hit.
func Test_Limiter_RemainingHeaderIsNeverNegative(t *testing.T) {
	t.Parallel()

	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()

			clock := newTestClock(time.Now().Truncate(time.Second))
			app := fiber.New()

			app.Use(New(Config{
				Max:                    1,
				Expiration:             time.Minute,
				SkipSuccessfulRequests: true,
				LimiterMiddleware:      strategy.handler,
				clock:                  clock.Now,
				Storage:                newFakeLimiterStorage(),
			}))

			started := make(chan struct{})
			release := make(chan struct{})
			app.Get("/:mode", func(c fiber.Ctx) error {
				if c.Params("mode") == "slow" {
					close(started)
					<-release
				}
				return c.SendStatus(fiber.StatusOK)
			})

			type respErr struct {
				resp *http.Response
				err  error
			}
			slowCh := make(chan respErr, 1)

			go func() {
				resp, err := app.Test(
					httptest.NewRequest(fiber.MethodGet, "/slow", http.NoBody),
					fiber.TestConfig{Timeout: 5 * time.Second},
				)
				slowCh <- respErr{resp: resp, err: err}
			}()

			<-started
			for range 2 {
				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fast", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
			}
			close(release)

			result := <-slowCh
			require.NoError(t, result.err)
			require.Equal(t, fiber.StatusOK, result.resp.StatusCode)
			require.Equal(t, "1", result.resp.Header.Get(xRateLimitLimit))
			require.Equal(t, "0", result.resp.Header.Get(xRateLimitRemaining))
		})
	}
}

// Skip* counts a plain error as the 500 it is answered with.
func Test_Limiter_PlainErrorCountsAsInternalError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want []int // statuses of three requests, Max being 1
		cfg  Config
	}{
		{name: "skip_failed", cfg: Config{SkipFailedRequests: true}, want: []int{500, 500, 500}},
		{name: "skip_successful", cfg: Config{SkipSuccessfulRequests: true}, want: []int{500, 429, 429}},
	}

	for _, strategy := range windowStrategies {
		for _, tc := range tests {
			t.Run(strategy.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				cfg := tc.cfg
				cfg.Max = 1
				cfg.Expiration = time.Minute
				cfg.DisableHeaders = true
				cfg.LimiterMiddleware = strategy.handler

				app := fiber.New()
				app.Use(New(cfg))
				app.Get("/", func(fiber.Ctx) error {
					return errors.New("boom")
				})

				got := make([]int, 0, len(tc.want))
				for range tc.want {
					resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
					require.NoError(t, err)
					got = append(got, resp.StatusCode)
				}
				require.Equal(t, tc.want, got)
			})
		}
	}
}

func Test_Limiter_ConfigDefaultCopyKeepsMax(t *testing.T) {
	t.Parallel()

	cfg := ConfigDefault
	cfg.Max = 2

	app := fiber.New()
	app.Use(New(cfg))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	for i, want := range []int{fiber.StatusOK, fiber.StatusOK, fiber.StatusTooManyRequests} {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, want, resp.StatusCode, "request %d", i+1)
		if want == fiber.StatusOK {
			require.Equal(t, "2", resp.Header.Get(xRateLimitLimit), "request %d", i+1)
		}
	}
}

func Test_Limiter_Next_BeforeMaxFunc(t *testing.T) {
	t.Parallel()

	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()

			var maxFuncCalls atomic.Int32
			app := fiber.New()
			app.Use(New(Config{
				Next: func(c fiber.Ctx) bool { return c.Path() == "/skipped" },
				MaxFunc: func(fiber.Ctx) int {
					maxFuncCalls.Add(1)
					return 5
				},
				LimiterMiddleware: strategy.handler,
			}))
			app.Get("/*", func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			for range 3 {
				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/skipped", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, fiber.StatusOK, resp.StatusCode)
			}
			require.Zero(t, maxFuncCalls.Load(), "MaxFunc ran for requests that Next skips")

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/limited", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
			require.EqualValues(t, 1, maxFuncCalls.Load())
		})
	}
}

func Test_Limiter_NegativeMaxFuncRejectsEveryRequest(t *testing.T) {
	t.Parallel()

	for _, strategy := range windowStrategies {
		for _, limit := range []int{-1, math.MinInt} {
			t.Run(fmt.Sprintf("%s/%d", strategy.name, limit), func(t *testing.T) {
				t.Parallel()

				app := fiber.New()
				app.Use(New(Config{
					MaxFunc:           func(fiber.Ctx) int { return limit },
					LimiterMiddleware: strategy.handler,
				}))
				app.Get("/", func(c fiber.Ctx) error {
					return c.SendStatus(fiber.StatusOK)
				})

				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
			})
		}
	}
}

// Requests sharing a key share the window of the one that opened it (the docs' /login example).
func Test_Limiter_ExpirationFunc_SharedKeyKeepsOpeningWindow(t *testing.T) {
	t.Parallel()

	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()

			clock := newTestClock(time.Now().Truncate(time.Second))
			app := fiber.New()
			app.Use(New(Config{
				Max:        1000,
				Expiration: 30 * time.Second,
				ExpirationFunc: func(c fiber.Ctx) time.Duration {
					if c.Path() == "/login" {
						return 60 * time.Second
					}
					return 30 * time.Second
				},
				LimiterMiddleware: strategy.handler,
				clock:             clock.Now,
				Storage:           newFakeLimiterStorage(),
			}))
			app.Get("/*", func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			reset := func(path string) string {
				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
				require.NoError(t, err)
				require.Equal(t, fiber.StatusOK, resp.StatusCode)
				return resp.Header.Get(xRateLimitReset)
			}

			require.Equal(t, "30", reset("/"))

			// /login joins the open window.
			clock.Add(time.Second)
			require.Equal(t, "29", reset("/login"))

			if strategy.name != "fixed" {
				return
			}

			// The next window follows its first request.
			clock.Add(30 * time.Second)
			require.Equal(t, "60", reset("/login"))
		})
	}
}

// A shorter window joining an open one must not shorten the entry's life.
func Test_Limiter_Fixed_ShorterWindowDoesNotShortenTheEntry(t *testing.T) {
	t.Parallel()

	type step struct {
		path string
		at   time.Duration
		want int
	}
	tests := []struct {
		name        string
		steps       []step
		cfg         Config
		shortStatus int // what "/" answers
	}{
		{
			name:        "counted",
			shortStatus: fiber.StatusOK,
			steps: []step{
				{"/login", 0, fiber.StatusOK},
				{"/", time.Second, fiber.StatusOK},
				{"/login", 35 * time.Second, fiber.StatusTooManyRequests},
			},
		},
		{
			name:        "credited_back",
			cfg:         Config{SkipFailedRequests: true},
			shortStatus: fiber.StatusInternalServerError,
			steps: []step{
				{"/login", 0, fiber.StatusOK},
				{"/", time.Second, fiber.StatusInternalServerError},
				{"/login", 35 * time.Second, fiber.StatusOK},
				{"/login", 36 * time.Second, fiber.StatusTooManyRequests},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			start := time.Now().Truncate(time.Second)
			clock := newTestClock(start)
			cfg := tc.cfg
			cfg.Max = 2
			cfg.ExpirationFunc = func(c fiber.Ctx) time.Duration {
				if c.Path() == "/login" {
					return time.Minute
				}
				return 30 * time.Second
			}
			cfg.LimiterMiddleware = FixedWindow{}
			cfg.clock = clock.Now
			cfg.Storage = &expiringStorage{fakeLimiterStorage: newFakeLimiterStorage(), clock: clock, expires: map[string]time.Time{}}

			app := fiber.New()
			app.Use(New(cfg))
			app.Get("/login", func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendStatus(tc.shortStatus)
			})

			for _, s := range tc.steps {
				clock.Add(start.Add(s.at).Sub(clock.Now()))
				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, s.path, http.NoBody))
				require.NoError(t, err)
				require.Equal(t, s.want, resp.StatusCode, "%s at +%s", s.path, s.at)
			}
		})
	}
}

// A shorter window joining a longer open one must not inflate the previous weight.
func Test_Limiter_Sliding_MixedWindowsDoNotInflateThePreviousWindow(t *testing.T) {
	t.Parallel()

	clock := newTestClock(time.Now().Truncate(time.Second))
	app := fiber.New()
	app.Use(New(Config{
		Max:        5,
		Expiration: 10 * time.Second,
		ExpirationFunc: func(c fiber.Ctx) time.Duration {
			if c.Path() == "/short" {
				return 2 * time.Second
			}
			return 10 * time.Second
		},
		LimiterMiddleware: SlidingWindow{},
		clock:             clock.Now,
		Storage:           newFakeLimiterStorage(),
	}))
	app.Get("/*", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	get := func(path string) *http.Response {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
		require.NoError(t, err)
		return resp
	}

	get("/")
	clock.Add(10 * time.Second) // the first window becomes the previous one

	resp := get("/")
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "3", resp.Header.Get(xRateLimitRemaining))

	// 9 of 10s are left: the previous hit weighs ceil(0.9) = 1, the rate is 1 + 2, leaving 2 of 5.
	clock.Add(time.Second)
	resp = get("/short")
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "2", resp.Header.Get(xRateLimitRemaining))
}

// limiterStores are the built-in store, which copies keys, and a Storage that keeps the string it is given.
var limiterStores = []struct {
	storage func() fiber.Storage
	name    string
}{
	{name: "builtin", storage: func() fiber.Storage { return nil }},
	{name: "keeps_key", storage: func() fiber.Storage { return newFakeLimiterStorage() }},
}

// keyOverwritingApp serves a handler that overwrites the memory behind the key (c.Get does not copy).
func keyOverwritingApp(cfg *Config) *fiber.App {
	cfg.Expiration = time.Minute
	cfg.KeyGenerator = func(c fiber.Ctx) string { return c.Get("X-Client") }

	app := fiber.New()
	app.Use(New(*cfg))
	app.Get("/", func(c fiber.Ctx) error {
		// Same length, so fasthttp overwrites the key's bytes in place.
		c.Request().Header.Set("X-Client", "BBBB")
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

// sendAsClient sends a request under the key keyOverwritingApp's handler overwrites.
func sendAsClient(t *testing.T, app *fiber.App) *http.Response {
	t.Helper()

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Client", "AAAA")
	resp, err := app.Test(req)
	require.NoError(t, err)
	return resp
}

// A handler overwriting the memory behind the key must not misdirect the credit.
func Test_Limiter_KeyOverwrittenByHandler(t *testing.T) {
	t.Parallel()

	for _, strategy := range windowStrategies {
		for _, store := range limiterStores {
			t.Run(strategy.name+"/"+store.name, func(t *testing.T) {
				t.Parallel()

				app := keyOverwritingApp(&Config{
					Max:                    1,
					SkipSuccessfulRequests: true,
					LimiterMiddleware:      strategy.handler,
					Storage:                store.storage(),
				})
				for i := range 3 {
					require.Equal(t, fiber.StatusOK, sendAsClient(t, app).StatusCode, "request %d", i+1)
				}
			})
		}
	}
}

// A handler overwriting the key must not misdirect the sliding window's header refresh.
func Test_Limiter_KeyOverwrittenByHandler_Headers(t *testing.T) {
	t.Parallel()

	for _, store := range limiterStores {
		t.Run(store.name, func(t *testing.T) {
			t.Parallel()

			app := keyOverwritingApp(&Config{Max: 5, LimiterMiddleware: SlidingWindow{}, Storage: store.storage()})
			for i, remaining := range []string{"4", "3", "2"} {
				resp := sendAsClient(t, app)
				require.Equal(t, fiber.StatusOK, resp.StatusCode, "request %d", i+1)
				require.Equal(t, remaining, resp.Header.Get(xRateLimitRemaining), "request %d", i+1)
			}
		})
	}
}

// rejectingHandler returns a handler and request context for a limiter with the one request
// Max allows already sent, so every further request is rejected.
func rejectingHandler(tb testing.TB, cfg *Config, expiration time.Duration) (fasthttp.RequestHandler, *fasthttp.RequestCtx) {
	tb.Helper()

	cfg.Max = 1
	cfg.Expiration = expiration
	cfg.KeyGenerator = func(fiber.Ctx) string { return testLimiterClientKey }

	app := fiber.New()
	app.Use(New(*cfg))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")

	h(fctx)
	return h, fctx
}

// A rejected request never reaches the handler, so it skips the key copy made for the read after it.
//
// Intentionally not parallel: testing.AllocsPerRun panics when called from a parallel test.
func Test_Limiter_RejectDoesNotCopyKey(t *testing.T) {
	rejectAllocs := func(t *testing.T, cfg Config) float64 {
		t.Helper()

		// A Retry-After under 100 formats without allocating.
		h, fctx := rejectingHandler(t, &cfg, time.Minute)
		h(fctx)
		require.Equal(t, fiber.StatusTooManyRequests, fctx.Response.StatusCode())

		// The fewest of many single runs, not an average: under -race sync.Pool drops a quarter of
		// what it is handed at random, so an average lands on either of two counts from one test run
		// to the next, while some single run always allocates only what the handler itself does.
		fewest := math.MaxFloat64
		for range 200 {
			fewest = min(fewest, testing.AllocsPerRun(1, func() { h(fctx) }))
		}
		return fewest
	}

	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			bare := rejectAllocs(t, Config{LimiterMiddleware: strategy.handler, DisableHeaders: true})
			// SkipSuccessfulRequests refunds every hit here, so nothing would be rejected.
			rereading := rejectAllocs(t, Config{LimiterMiddleware: strategy.handler, DisableHeaders: true, SkipFailedRequests: true})
			require.InDelta(t, bare, rereading, 0, "the Skip options must not add to a rejected request")

			// Sliding also re-reads the entry for the headers.
			if strategy.name == "sliding" {
				headers := rejectAllocs(t, Config{LimiterMiddleware: strategy.handler})
				require.InDelta(t, bare, headers, 0, "the header refresh must not add to a rejected request")
			}
		})
	}
}

func Test_Limiter_DisableValueRedaction_ShowsBoundedKey(t *testing.T) {
	t.Parallel()

	oversizedKey := strings.Repeat("k", maxKeyLength*2)
	storage := newFakeLimiterStorage()
	storage.errs["get|"+boundKey(oversizedKey)] = errors.New("boom")

	var captured error
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.SendStatus(fiber.StatusInternalServerError)
		},
	})
	app.Use(New(Config{
		Storage:               storage,
		DisableValueRedaction: true,
		KeyGenerator:          func(fiber.Ctx) string { return oversizedKey },
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.ErrorContains(t, captured, boundKey(oversizedKey))
	require.NotContains(t, captured.Error(), oversizedKey)
}

// Test_Limiter_StorageRoundTrips pins the storage reads and writes of a passing request, as the docs describe.
func Test_Limiter_StorageRoundTrips(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   Config
		trips int // reads, and as many writes
	}{
		{name: "fixed", cfg: Config{LimiterMiddleware: FixedWindow{}}, trips: 1},
		{name: "fixed_skip_successful", cfg: Config{LimiterMiddleware: FixedWindow{}, SkipSuccessfulRequests: true}, trips: 2},
		{name: "sliding", cfg: Config{LimiterMiddleware: SlidingWindow{}}, trips: 2},
		{name: "sliding_disable_headers", cfg: Config{LimiterMiddleware: SlidingWindow{}, DisableHeaders: true}, trips: 1},
		{
			name:  "sliding_disable_headers_skip_successful",
			cfg:   Config{LimiterMiddleware: SlidingWindow{}, DisableHeaders: true, SkipSuccessfulRequests: true},
			trips: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			storage := newContextRecorderLimiterStorage()
			cfg := tc.cfg
			cfg.Max = 1
			cfg.Expiration = time.Second
			cfg.Storage = storage

			app := fiber.New()
			app.Use(New(cfg))
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)

			require.Len(t, storage.recordedGets(), tc.trips)
			require.Len(t, storage.recordedSets(), tc.trips)
		})
	}
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

// go test -v -run=^$ -bench=Benchmark_Limiter_Parallel_Keys -benchmem -count=4
func Benchmark_Limiter_Parallel_Keys(b *testing.B) {
	var clients atomic.Uint64

	app := fiber.New()

	app.Use(New(Config{
		Max:          b.N, // no key can pass b.N hits, so no iteration takes the 429 path
		Expiration:   60 * time.Second,
		KeyGenerator: func(c fiber.Ctx) string { return c.Get("X-Client") },
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	h := app.Handler()

	// One distinct key per goroutine: unrelated clients must not serialize.
	b.RunParallel(func(pb *testing.PB) {
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.SetMethod(fiber.MethodGet)
		fctx.Request.SetRequestURI("/")
		fctx.Request.Header.Set("X-Client", utils.FormatUint(clients.Add(1)))

		for pb.Next() {
			h(fctx)
		}
	})
}

// Benchmark_Limiter_Admit measures requests that pass: Max is too high to reach the 429 path.
//
// go test -v -run=^$ -bench=Benchmark_Limiter_Admit -benchmem -count=4
func Benchmark_Limiter_Admit(b *testing.B) {
	oversizedKey := strings.Repeat("x", maxKeyLength*4)
	oversized := func(fiber.Ctx) string { return oversizedKey }

	benchmarks := []struct {
		name string
		cfg  Config
	}{
		{name: "fixed", cfg: Config{LimiterMiddleware: FixedWindow{}}},
		{name: "fixed_skip_successful", cfg: Config{LimiterMiddleware: FixedWindow{}, SkipSuccessfulRequests: true}},
		{name: "fixed_oversized_key", cfg: Config{LimiterMiddleware: FixedWindow{}, KeyGenerator: oversized}},
		{name: "sliding", cfg: Config{LimiterMiddleware: SlidingWindow{}}},
		{name: "sliding_disable_headers", cfg: Config{LimiterMiddleware: SlidingWindow{}, DisableHeaders: true}},
		{name: "sliding_skip_successful", cfg: Config{LimiterMiddleware: SlidingWindow{}, SkipSuccessfulRequests: true}},
		{name: "sliding_oversized_key", cfg: Config{LimiterMiddleware: SlidingWindow{}, KeyGenerator: oversized}},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			cfg := bm.cfg
			cfg.Max = math.MaxInt32
			cfg.Expiration = time.Hour

			app := fiber.New()
			app.Use(New(cfg))
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("Hello, World!")
			})

			h := app.Handler()

			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(fiber.MethodGet)
			fctx.Request.SetRequestURI("/")

			b.ReportAllocs()

			for b.Loop() {
				h(fctx)
			}
		})
	}
}

// Benchmark_Limiter_Reject measures requests the limit rejects: the handler never runs.
//
// go test -v -run=^$ -bench=Benchmark_Limiter_Reject -benchmem -count=4
func Benchmark_Limiter_Reject(b *testing.B) {
	benchmarks := []struct {
		name string
		cfg  Config
	}{
		{name: "fixed", cfg: Config{LimiterMiddleware: FixedWindow{}}},
		{name: "fixed_skip_failed", cfg: Config{LimiterMiddleware: FixedWindow{}, SkipFailedRequests: true}},
		{name: "sliding", cfg: Config{LimiterMiddleware: SlidingWindow{}}},
		{name: "sliding_disable_headers", cfg: Config{LimiterMiddleware: SlidingWindow{}, DisableHeaders: true}},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			h, fctx := rejectingHandler(b, &bm.cfg, time.Hour)

			b.ReportAllocs()

			for b.Loop() {
				h(fctx)
			}
		})
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

// A sub-second ExpirationFunc floors to a 1s window: a 0s one made the sliding rate NaN.
//
// go test -run Test_Limiter_SubSecondExpirationFunc -race -v
func Test_Limiter_SubSecondExpirationFunc(t *testing.T) {
	t.Parallel()

	for _, strategy := range windowStrategies {
		t.Run(strategy.name, func(t *testing.T) {
			t.Parallel()

			clock := newTestClock(time.Now().Truncate(time.Second))
			app := fiber.New()
			app.Use(New(Config{
				Max:            5,
				ExpirationFunc: func(fiber.Ctx) time.Duration { return 500 * time.Millisecond },
				clock:          clock.Now,
				// the memory store expires by the real clock, which a slow runner could outrun
				Storage:           newFakeLimiterStorage(),
				LimiterMiddleware: strategy.handler,
			}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("Hello tester!") })

			for i := 1; i <= 5; i++ {
				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, fiber.StatusOK, resp.StatusCode, "request %d", i)
				require.Equal(t, "1", resp.Header.Get(xRateLimitReset), "request %d", i)
			}

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
			require.Equal(t, "1", resp.Header.Get(fiber.HeaderRetryAfter))
		})
	}
}

// Test_Limiter_OversizedKeyGeneratorIsBounded proves the property manager's
// boundKey exists for: an oversized key — the shape a raw, unvalidated
// ProxyHeader value or an unchecked custom KeyGenerator would produce — never
// reaches the storage backend verbatim. Runs both window strategies, since
// each calls the manager independently.
func Test_Limiter_OversizedKeyGeneratorIsBounded(t *testing.T) {
	t.Parallel()

	for _, strategy := range []Handler{FixedWindow{}, SlidingWindow{}} {
		t.Run(fmt.Sprintf("%T", strategy), func(t *testing.T) {
			t.Parallel()

			oversizedKey := strings.Repeat("x", maxKeyLength*4)
			storage := newFakeLimiterStorage()
			app := fiber.New()
			app.Use(New(Config{
				Storage:           storage,
				Max:               1,
				Expiration:        time.Minute,
				LimiterMiddleware: strategy,
				KeyGenerator:      func(fiber.Ctx) string { return oversizedKey },
			}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)

			require.Len(t, storage.data, 1, "exactly one bucket should have been written")
			for storedKey := range storage.data {
				require.NotEqual(t, oversizedKey, storedKey, "the raw oversized key must never reach storage")
				require.LessOrEqual(t, len(storedKey), maxKeyLength)
				require.True(t, strings.HasPrefix(storedKey, hashPrefix))
			}

			// The same oversized key must still be recognized as the same client on
			// the next request, so rate limiting keeps working for it rather than
			// silently exempting it or colliding it with someone else's bucket.
			resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
			require.Len(t, storage.data, 1, "the second request must land in the same bucket, not a new one")
		})
	}
}
