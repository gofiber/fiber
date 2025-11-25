package limiter

import (
	"io"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/storage/memory"
	"github.com/gofiber/fiber/v2/utils"

	"github.com/valyala/fasthttp"
)

type recordingStorage struct {
	mu          sync.Mutex
	values      map[string][]byte
	expirations []time.Duration
}

func newRecordingStorage() *recordingStorage {
	return &recordingStorage{values: make(map[string][]byte)}
}

func (s *recordingStorage) Get(key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.values[key]
	if !ok {
		return nil, nil
	}

	return append([]byte(nil), v...), nil
}

func (s *recordingStorage) Set(key string, val []byte, exp time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.expirations = append(s.expirations, exp)
	s.values[key] = append([]byte(nil), val...)

	return nil
}

func (s *recordingStorage) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.values, key)

	return nil
}

func (s *recordingStorage) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.values = make(map[string][]byte)
	s.expirations = nil

	return nil
}

func (s *recordingStorage) Close() error {
	return nil
}

// go test -run Test_Limiter_Concurrency_Store -race -v
func Test_Limiter_Concurrency_Store(t *testing.T) {
	t.Parallel()
	// Test concurrency using a custom store

	app := fiber.New()

	app.Use(New(Config{
		Max:        50,
		Expiration: 2 * time.Second,
		Storage:    memory.New(),
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello tester!")
	})

	var wg sync.WaitGroup
	singleRequest := func(wg *sync.WaitGroup) {
		defer wg.Done()
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "Hello tester!", string(body))
	}

	for i := 0; i <= 49; i++ {
		wg.Add(1)
		go singleRequest(&wg)
	}

	wg.Wait()

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
}

// go test -run Test_Limiter_Concurrency -race -v
func Test_Limiter_Concurrency(t *testing.T) {
	t.Parallel()
	// Test concurrency using a default store

	app := fiber.New()

	app.Use(New(Config{
		Max:        50,
		Expiration: 2 * time.Second,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello tester!")
	})

	var wg sync.WaitGroup
	singleRequest := func(wg *sync.WaitGroup) {
		defer wg.Done()
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "Hello tester!", string(body))
	}

	for i := 0; i <= 49; i++ {
		wg.Add(1)
		go singleRequest(&wg)
	}

	wg.Wait()

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
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

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" { //nolint:goconst // False positive
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
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

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
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

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(4*time.Second + 500*time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
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

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(4*time.Second + 500*time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
}

func Test_Limiter_Sliding_Window_Reset_After_Expiration(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:               1,
		Expiration:        time.Second,
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusTooManyRequests, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
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

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
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

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
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

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(4*time.Second + 500*time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
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

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(4*time.Second + 500*time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
}

// go test -run Test_Limiter_Fixed_Window_Skip_Successful_Requests -v
func Test_Limiter_Fixed_Window_Skip_Successful_Requests(t *testing.T) {
	t.Parallel()
	// Test concurrency using a default store

	app := fiber.New()

	app.Use(New(Config{
		Max:                    1,
		Expiration:             2 * time.Second,
		SkipSuccessfulRequests: true,
		LimiterMiddleware:      FixedWindow{},
	}))

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)
}

// go test -run Test_Limiter_Fixed_Window_Skip_Long_Request_Expiration -v
func Test_Limiter_Fixed_Window_Skip_Long_Request_Expiration(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	hit := 0

	app.Use(New(Config{
		Max:                    1,
		Expiration:             time.Second,
		SkipSuccessfulRequests: true,
		LimiterMiddleware:      FixedWindow{},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		hit++
		if hit == 1 {
			time.Sleep(time.Second + 200*time.Millisecond)
			return c.SendStatus(fiber.StatusOK)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil), -1)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil), -1)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 500, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil), -1)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)
}

// go test -run Test_Limiter_Fixed_Window_Custom_Storage_Skip_Successful_Requests -v
func Test_Limiter_Fixed_Window_Custom_Storage_Skip_Successful_Requests(t *testing.T) {
	t.Parallel()
	// Test concurrency using a default store

	app := fiber.New()

	app.Use(New(Config{
		Max:                    1,
		Expiration:             2 * time.Second,
		Storage:                memory.New(),
		SkipSuccessfulRequests: true,
		LimiterMiddleware:      FixedWindow{},
	}))

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)
}

// go test -run Test_Limiter_Sliding_Window_Skip_Successful_Requests -v
func Test_Limiter_Sliding_Window_Skip_Successful_Requests(t *testing.T) {
	t.Parallel()
	// Test concurrency using a default store

	app := fiber.New()

	app.Use(New(Config{
		Max:                    1,
		Expiration:             2 * time.Second,
		SkipSuccessfulRequests: true,
		LimiterMiddleware:      SlidingWindow{},
	}))

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(4*time.Second + 500*time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)
}

// go test -run Test_Limiter_Sliding_Window_Skip_Long_Request_Expiration -v
func Test_Limiter_Sliding_Window_Skip_Long_Request_Expiration(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	hit := 0

	app.Use(New(Config{
		Max:                    1,
		Expiration:             time.Second,
		SkipSuccessfulRequests: true,
		LimiterMiddleware:      SlidingWindow{},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		hit++
		if hit == 1 {
			time.Sleep(time.Second + 200*time.Millisecond)
			return c.SendStatus(fiber.StatusOK)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil), -1)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil), -1)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 500, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil), -1)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)
}

// go test -run Test_Limiter_Sliding_Window_Custom_Storage_Skip_Successful_Requests -v
func Test_Limiter_Sliding_Window_Custom_Storage_Skip_Successful_Requests(t *testing.T) {
	t.Parallel()
	// Test concurrency using a default store

	app := fiber.New()

	app.Use(New(Config{
		Max:                    1,
		Expiration:             2 * time.Second,
		Storage:                memory.New(),
		SkipSuccessfulRequests: true,
		LimiterMiddleware:      SlidingWindow{},
	}))

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(4*time.Second + 500*time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)
}

// go test -run Test_Limiter_Sliding_Window_Skip_TTL_Preservation -v
func Test_Limiter_Sliding_Window_Skip_TTL_Preservation(t *testing.T) {
	t.Parallel()

	store := newRecordingStorage()

	app := fiber.New()

	app.Use(New(Config{
		Max:                    1,
		Expiration:             2 * time.Second,
		Storage:                store,
		SkipSuccessfulRequests: true,
		LimiterMiddleware:      SlidingWindow{},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	store.mu.Lock()
	exps := append([]time.Duration(nil), store.expirations...)
	store.mu.Unlock()

	if len(exps) < 2 {
		t.Fatalf("expected at least two set calls, got %d", len(exps))
	}

	firstTTL := exps[len(exps)-2]
	secondTTL := exps[len(exps)-1]

	if firstTTL <= 2*time.Second {
		t.Fatalf("expected initial TTL to exceed expiration, got %s", firstTTL)
	}

	if secondTTL < firstTTL {
		t.Fatalf("expected skip branch to preserve TTL, got first=%s second=%s", firstTTL, secondTTL)
	}
}

// go test -v -run=^$ -bench=Benchmark_Limiter_Custom_Store -benchmem -count=4
func Benchmark_Limiter_Custom_Store(b *testing.B) {
	app := fiber.New()

	app.Use(New(Config{
		Max:        100,
		Expiration: 60 * time.Second,
		Storage:    memory.New(),
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(fctx)
	}
}

// go test -run Test_Limiter_Next
func Test_Limiter_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_Limiter_Headers(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Max:        50,
		Expiration: 2 * time.Second,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello tester!")
	})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")

	app.Handler()(fctx)

	utils.AssertEqual(t, "50", string(fctx.Response.Header.Peek("X-RateLimit-Limit")))
	if v := string(fctx.Response.Header.Peek("X-RateLimit-Remaining")); v == "" {
		t.Errorf("The X-RateLimit-Remaining header is not set correctly - value is an empty string.")
	}
	if v := string(fctx.Response.Header.Peek("X-RateLimit-Reset")); !(v == "1" || v == "2") {
		t.Errorf("The X-RateLimit-Reset header is not set correctly - value is out of bounds.")
	}
}

// go test -v -run=^$ -bench=Benchmark_Limiter -benchmem -count=4
func Benchmark_Limiter(b *testing.B) {
	app := fiber.New()

	app.Use(New(Config{
		Max:        100,
		Expiration: 60 * time.Second,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(fctx)
	}
}

// go test -run Test_Sliding_Window -race -v
func Test_Sliding_Window(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Max:               10,
		Expiration:        2 * time.Second,
		Storage:           memory.New(),
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello tester!")
	})

	singleRequest := func(shouldFail bool) {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
		if shouldFail {
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, 429, resp.StatusCode)
		} else {
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
		}
	}

	for i := 0; i < 5; i++ {
		singleRequest(false)
	}

	time.Sleep(2 * time.Second)

	for i := 0; i < 5; i++ {
		singleRequest(false)
	}

	time.Sleep(3 * time.Second)

	for i := 0; i < 5; i++ {
		singleRequest(false)
	}

	time.Sleep(4 * time.Second)

	for i := 0; i < 9; i++ {
		singleRequest(false)
	}
}

func Test_Limiter_Fixed_Window_SubSecondExpiration(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	baseTs := uint32(time.Now().Unix())
	refreshTimestamp := func(ts uint32) {
		atomic.StoreUint32(&utils.Timestamp, ts)
	}
	refreshTimestamp(baseTs)

	app.Use(New(Config{
		Max:               1,
		Expiration:        500 * time.Millisecond,
		LimiterMiddleware: FixedWindow{},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	refreshTimestamp(baseTs)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, true, resp.Header.Get(xRateLimitReset) != "")

	refreshTimestamp(baseTs)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusTooManyRequests, resp.StatusCode)
}

func Test_Limiter_Fixed_Window_FractionalExpiration_TTLSynced(t *testing.T) {
	t.Parallel()

	storage := newRecordingStorage()

	app := fiber.New()

	app.Use(New(Config{
		Max:                    1,
		Expiration:             500 * time.Millisecond,
		LimiterMiddleware:      FixedWindow{},
		Storage:                storage,
		SkipSuccessfulRequests: true,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	if len(storage.expirations) != 2 {
		t.Fatalf("expected 2 expirations recorded, got %d", len(storage.expirations))
	}

	utils.AssertEqual(t, time.Second, storage.expirations[0])
	utils.AssertEqual(t, time.Second, storage.expirations[1])
}

func Test_Limiter_Fixed_Window_FractionalExpiration_PersistsWindow(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Max:               1,
		Expiration:        1500 * time.Millisecond,
		LimiterMiddleware: FixedWindow{},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

	time.Sleep(1700 * time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusTooManyRequests, resp.StatusCode)

	time.Sleep(400 * time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
}

func Test_Limiter_Sliding_Window_SubSecondExpiration(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	baseTs := uint32(time.Now().Unix())
	refreshTimestamp := func(ts uint32) {
		atomic.StoreUint32(&utils.Timestamp, ts)
	}
	refreshTimestamp(baseTs)

	app.Use(New(Config{
		Max:               1,
		Expiration:        500 * time.Millisecond,
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	refreshTimestamp(baseTs)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, true, resp.Header.Get(xRateLimitReset) != "")

	refreshTimestamp(baseTs)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusTooManyRequests, resp.StatusCode)
}
