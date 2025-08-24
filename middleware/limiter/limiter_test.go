package limiter

import (
	"io"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

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
		if c.Params("status") == "fail" { //nolint:goconst // test
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	time.Sleep(4*time.Second + 500*time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
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
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
			assert.NoError(t, err)
			assert.Equal(t, fiber.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			assert.Equal(t, "Hello tester!", string(body))
		}(&wg)
	}

	wg.Wait()

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
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
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
			assert.NoError(t, err)
			assert.Equal(t, fiber.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			assert.Equal(t, "Hello tester!", string(body))
		}(&wg)
	}

	wg.Wait()

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
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
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
			assert.NoError(t, err)
			assert.Equal(t, fiber.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			assert.Equal(t, "Hello tester!", string(body))
		}(&wg)
	}

	wg.Wait()

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
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
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
			assert.NoError(t, err)
			assert.Equal(t, fiber.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			assert.Equal(t, "Hello tester!", string(body))
		}(&wg)
	}

	wg.Wait()

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(4*time.Second + 500*time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(4*time.Second + 500*time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(4*time.Second + 500*time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(4*time.Second + 500*time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(4*time.Second + 500*time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/success", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
	require.NoError(t, err)
	require.Equal(t, 429, resp.StatusCode)

	time.Sleep(4*time.Second + 500*time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/fail", nil))
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
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

	// Second request should be rate limited because the first failed request was counted
	// But currently this is not happening due to the bug
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
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
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

	// Second request should be rate limited because the first failed request was counted
	// But currently this is not happening due to the bug
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
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
		t.Errorf("The X-RateLimit-Remaining header is not set correctly - value is an empty string.")
	}
	if v := string(fctx.Response.Header.Peek("X-RateLimit-Reset")); !(v == "1" || v == "2") {
		t.Errorf("The X-RateLimit-Reset header is not set correctly - value is out of bounds.")
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
	require.Equal(t, "", string(fctx.Response.Header.Peek("X-RateLimit-Limit")))
	require.Equal(t, "", string(fctx.Response.Header.Peek("X-RateLimit-Remaining")))
	require.Equal(t, "", string(fctx.Response.Header.Peek("X-RateLimit-Reset")))

	// second request should hit the limit and return 429 without headers
	fctx2 := &fasthttp.RequestCtx{}
	fctx2.Request.Header.SetMethod(fiber.MethodGet)
	fctx2.Request.SetRequestURI("/")

	app.Handler()(fctx2)

	require.Equal(t, fiber.StatusTooManyRequests, fctx2.Response.StatusCode())
	require.Equal(t, "", string(fctx2.Response.Header.Peek(fiber.HeaderRetryAfter)))
	require.Equal(t, "", string(fctx2.Response.Header.Peek("X-RateLimit-Limit")))
	require.Equal(t, "", string(fctx2.Response.Header.Peek("X-RateLimit-Remaining")))
	require.Equal(t, "", string(fctx2.Response.Header.Peek("X-RateLimit-Reset")))
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
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
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
