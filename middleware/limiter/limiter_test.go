package limiter

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Limiter_Concurrency -race -v
func Test_Limiter_Concurrency(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		Max:      50,
		Duration: 2 * time.Second,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello tester!")
	})

	var wg sync.WaitGroup
	singleRequest := func(wg *sync.WaitGroup) {
		defer wg.Done()
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Unexpected status code %v", resp.StatusCode)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil || "Hello tester!" != string(body) {
			t.Fatalf("Unexpected body %v", string(body))
		}
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

// go test -v -run=^$ -bench=Benchmark_Limiter -benchmem -count=4
func Benchmark_Limiter(b *testing.B) {
	app := fiber.New()

	app.Use(New(Config{
		Max:      100,
		Duration: 60 * time.Second,
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

	utils.AssertEqual(b, "100", string(fctx.Response.Header.Peek("X-RateLimit-Limit")))
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
