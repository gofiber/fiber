package limiter

import (
	"io/ioutil"
	"math/rand"
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
		Max:      100,
		Duration: 1 * time.Minute,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		// random delay between the requests
		time.Sleep(time.Duration(rand.Intn(10000)) * time.Microsecond)
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

	for i := 0; i <= 50; i++ {
		wg.Add(1)
		go singleRequest(&wg)
	}

	wg.Wait()
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
