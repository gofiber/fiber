package limiter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type failingLimiterStorage struct {
	data map[string][]byte
	errs map[string]error
}

const testLimiterClientKey = "client-key"

func newFailingLimiterStorage() *failingLimiterStorage {
	return &failingLimiterStorage{
		data: make(map[string][]byte),
		errs: make(map[string]error),
	}
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

func newContextRecorderLimiterStorage() *contextRecorderLimiterStorage {
	return &contextRecorderLimiterStorage{failingLimiterStorage: newFailingLimiterStorage()}
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

func (s *failingLimiterStorage) GetWithContext(_ context.Context, key string) ([]byte, error) {
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

	time.Sleep(4*time.Second + 500*time.Millisecond)

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

	time.Sleep(4*time.Second + 500*time.Millisecond)

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

	for i := 0; i < 2; i++ {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	}

	time.Sleep(time.Second + 100*time.Millisecond)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "2", resp.Header.Get(xRateLimitLimit))
	require.Equal(t, "1", resp.Header.Get(xRateLimitRemaining))
	require.NotEmpty(t, resp.Header.Get(xRateLimitReset))
}

func Test_Limiter_Sliding_Window_ExpiresStalePrevHits(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:               1,
		Expiration:        time.Second,
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	time.Sleep(2500 * time.Millisecond)

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

	time.Sleep(4*time.Second + 500*time.Millisecond)

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

	time.Sleep(4*time.Second + 500*time.Millisecond)

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

	time.Sleep(4*time.Second + 500*time.Millisecond)

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

	time.Sleep(4*time.Second + 500*time.Millisecond)

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
