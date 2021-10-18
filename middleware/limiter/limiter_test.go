package limiter

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/storage/memory"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Limiter_Concurrency_Store -race -v
func Test_Limiter_Concurrency_Store(t *testing.T) {
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
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

		body, err := ioutil.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "Hello tester!", string(body))
	}

	for i := 0; i <= 49; i++ {
		wg.Add(1)
		go singleRequest(&wg)
	}

	wg.Wait()

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
}

// go test -run Test_Limiter_Concurrency -race -v
func Test_Limiter_Concurrency(t *testing.T) {

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
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

		body, err := ioutil.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "Hello tester!", string(body))
	}

	for i := 0; i <= 49; i++ {
		wg.Add(1)
		go singleRequest(&wg)
	}

	wg.Wait()

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

}

// go test -run Test_Limiter_No_Skip_Choices -v
func Test_Limiter_No_Skip_Choices(t *testing.T) {

	app := fiber.New()

	app.Use(New(Config{
		Max:                    2,
		Expiration:             2 * time.Second,
		SkipFailedRequests:     false,
		SkipSuccessfulRequests: false,
	}))

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

}

// go test -run Test_Limiter_Skip_Failed_Requests -v
func Test_Limiter_Skip_Failed_Requests(t *testing.T) {

	app := fiber.New()

	app.Use(New(Config{
		Max:                1,
		Expiration:         2 * time.Second,
		SkipFailedRequests: true,
	}))

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

}

// go test -run Test_Limiter_Skip_Successful_Requests -v
func Test_Limiter_Skip_Successful_Requests(t *testing.T) {

	// Test concurrency using a default store

	app := fiber.New()

	app.Use(New(Config{
		Max:                    1,
		Expiration:             2 * time.Second,
		SkipSuccessfulRequests: true,
	}))

	app.Get("/:status", func(c *fiber.Ctx) error {
		if c.Params("status") == "fail" {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/success", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 429, resp.StatusCode)

	time.Sleep(3 * time.Second)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/fail", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 400, resp.StatusCode)

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
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/")

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(fctx)
	}
}

// go test -run Test_Limiter_Next
func Test_Limiter_Next(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_Limiter_Headers(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		Max:        50,
		Expiration: 2 * time.Second,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello tester!")
	})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
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
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/")

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(fctx)
	}
}

// go test -run Test_Limiter_Cheat -race -v
// Attempt to cheat the rate limiter by waiting until the window ends and sending more requests
func Test_Limiter_Cheat(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Max:               10,
		Expiration:        4 * time.Second,
		Storage:           memory.New(),
		LimiterMiddleware: SlidingWindow{},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello tester!")
	})

	var wg sync.WaitGroup
	singleRequest := func(wg *sync.WaitGroup, shouldFail bool) {
		if wg != nil {
			defer wg.Done()
		}
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
		if shouldFail {
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, 429, resp.StatusCode)
		} else {
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
		}
	}

	t1 := time.Now()
	singleRequest(nil, false)           // one request to start our window
	time.Sleep(1000 * time.Millisecond) // Wait to make sure we are well into the current window

	// Send requests
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go singleRequest(&wg, false)
	}

	wg.Wait()

	// wait until the current window is finished and we are into the next window
	t2 := time.Until(t1.Add(time.Millisecond * 5250))
	time.Sleep(t2)

	// Send more requests
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go singleRequest(&wg, false)
	}
	wg.Wait()

	// these should fail
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go singleRequest(&wg, true)
	}

	wg.Wait()

	time.Sleep(8 * time.Second) // wait 2 windows to ensure our rate has
	// Verify that we are able to send requests again
	for i := 0; i < 9; i++ {
		wg.Add(1)
		go singleRequest(&wg, false)
	}
	wg.Wait()
}

// go test -run Test_Sliding_Window -race -v
func Test_Sliding_Window(t *testing.T) {
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

	//var wg sync.WaitGroup
	singleRequest := func(wg *sync.WaitGroup, shouldFail bool) {
		if wg != nil {
			defer wg.Done()
		}
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
		if shouldFail {
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, 429, resp.StatusCode)
		} else {
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
		}
	}

	for i := 0; i < 5; i++ {
		singleRequest(nil, false)
	}

	time.Sleep(2 * time.Second)

	for i := 0; i < 5; i++ {
		singleRequest(nil, false)
	}

	time.Sleep(3 * time.Second)

	for i := 0; i < 5; i++ {
		singleRequest(nil, false)
	}

	time.Sleep(4 * time.Second)

	for i := 0; i < 9; i++ {
		singleRequest(nil, false)
	}
}
