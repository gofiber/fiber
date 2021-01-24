// Special thanks to @codemicro for moving this to fiber core
// Original middleware: github.com/codemicro/fiber-cache
package cache

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/storage/memory"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

func Test_Cache_CacheControl(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		CacheControl: true,
		Expiration:   10 * time.Second,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	_, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "public, max-age=10", resp.Header.Get(fiber.HeaderCacheControl))
}

func Test_Cache_Expired(t *testing.T) {
	app := fiber.New()

	expiration := 1 * time.Second

	app.Use(New(Config{
		Expiration: expiration,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(fmt.Sprintf("%d", time.Now().UnixNano()))
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	// Sleep until the cache is expired
	time.Sleep(2 * time.Second)

	respCached, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	bodyCached, err := ioutil.ReadAll(respCached.Body)
	utils.AssertEqual(t, nil, err)

	if bytes.Equal(body, bodyCached) {
		t.Errorf("Cache should have expired: %s, %s", body, bodyCached)
	}
}

func Test_Cache(t *testing.T) {
	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		now := fmt.Sprintf("%d", time.Now().UnixNano())
		return c.SendString(now)
	})

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	cachedReq := httptest.NewRequest("GET", "/", nil)
	cachedResp, err := app.Test(cachedReq)
	utils.AssertEqual(t, nil, err)

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	cachedBody, err := ioutil.ReadAll(cachedResp.Body)
	utils.AssertEqual(t, nil, err)

	utils.AssertEqual(t, cachedBody, body)
}

// // go test -run Test_Cache_Concurrency_Storage -race -v
// func Test_Cache_Concurrency_Storage(t *testing.T) {
// 	// Test concurrency using a custom store

// 	app := fiber.New()

// 	app.Use(New(Config{
// 		Storage: memory.New(),
// 	}))

// 	app.Get("/", func(c *fiber.Ctx) error {
// 		return c.SendString("Hello tester!")
// 	})

// 	var wg sync.WaitGroup
// 	singleRequest := func(wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
// 		utils.AssertEqual(t, nil, err)
// 		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

// 		body, err := ioutil.ReadAll(resp.Body)
// 		utils.AssertEqual(t, nil, err)
// 		utils.AssertEqual(t, "Hello tester!", string(body))
// 	}

// 	for i := 0; i <= 49; i++ {
// 		wg.Add(1)
// 		go singleRequest(&wg)
// 	}

// 	wg.Wait()

// 	req := httptest.NewRequest("GET", "/", nil)
// 	resp, err := app.Test(req)
// 	utils.AssertEqual(t, nil, err)

// 	cachedReq := httptest.NewRequest("GET", "/", nil)
// 	cachedResp, err := app.Test(cachedReq)
// 	utils.AssertEqual(t, nil, err)

// 	body, err := ioutil.ReadAll(resp.Body)
// 	utils.AssertEqual(t, nil, err)
// 	cachedBody, err := ioutil.ReadAll(cachedResp.Body)
// 	utils.AssertEqual(t, nil, err)

// 	utils.AssertEqual(t, cachedBody, body)
// }

func Test_Cache_Invalid_Expiration(t *testing.T) {
	app := fiber.New()
	cache := New(Config{Expiration: 0 * time.Second})
	app.Use(cache)

	app.Get("/", func(c *fiber.Ctx) error {
		now := fmt.Sprintf("%d", time.Now().UnixNano())
		return c.SendString(now)
	})

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	cachedReq := httptest.NewRequest("GET", "/", nil)
	cachedResp, err := app.Test(cachedReq)
	utils.AssertEqual(t, nil, err)

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	cachedBody, err := ioutil.ReadAll(cachedResp.Body)
	utils.AssertEqual(t, nil, err)

	utils.AssertEqual(t, cachedBody, body)
}

func Test_Cache_Invalid_Method(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Post("/", func(c *fiber.Ctx) error {
		return c.SendString(c.Query("cache"))
	})

	app.Get("/get", func(c *fiber.Ctx) error {
		return c.SendString(c.Query("cache"))
	})

	resp, err := app.Test(httptest.NewRequest("POST", "/?cache=123", nil))
	utils.AssertEqual(t, nil, err)
	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "123", string(body))

	resp, err = app.Test(httptest.NewRequest("POST", "/?cache=12345", nil))
	utils.AssertEqual(t, nil, err)
	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "12345", string(body))

	resp, err = app.Test(httptest.NewRequest("GET", "/get?cache=123", nil))
	utils.AssertEqual(t, nil, err)
	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "123", string(body))

	resp, err = app.Test(httptest.NewRequest("GET", "/get?cache=12345", nil))
	utils.AssertEqual(t, nil, err)
	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "123", string(body))
}

func Test_Cache_NothingToCache(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{Expiration: -(time.Second * 1)}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(time.Now().String())
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	time.Sleep(500 * time.Millisecond)

	respCached, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	bodyCached, err := ioutil.ReadAll(respCached.Body)
	utils.AssertEqual(t, nil, err)

	if bytes.Equal(body, bodyCached) {
		t.Errorf("Cache should have expired: %s, %s", body, bodyCached)
	}
}

func Test_Cache_CustomNext(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{
		Next: func(c *fiber.Ctx) bool {
			return !(c.Response().StatusCode() == fiber.StatusOK)
		},
		CacheControl: true,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(time.Now().String())
	})

	app.Get("/error", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusInternalServerError).SendString(time.Now().String())
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	respCached, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	bodyCached, err := ioutil.ReadAll(respCached.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, bytes.Equal(body, bodyCached))
	utils.AssertEqual(t, true, respCached.Header.Get(fiber.HeaderCacheControl) != "")

	_, err = app.Test(httptest.NewRequest("GET", "/error", nil))
	utils.AssertEqual(t, nil, err)

	errRespCached, err := app.Test(httptest.NewRequest("GET", "/error", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, errRespCached.Header.Get(fiber.HeaderCacheControl) == "")
}

func Test_CustomKey(t *testing.T) {
	app := fiber.New()
	var called bool
	app.Use(New(Config{KeyGenerator: func(c *fiber.Ctx) string {
		called = true
		return c.Path()
	}}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("hi")
	})

	req := httptest.NewRequest("GET", "/", nil)
	_, err := app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, called)

}

// go test -v -run=^$ -bench=Benchmark_Cache -benchmem -count=4
func Benchmark_Cache(b *testing.B) {
	app := fiber.New()

	app.Use(New())

	app.Get("/demo", func(c *fiber.Ctx) error {
		data, _ := ioutil.ReadFile("../../.github/README.md")
		return c.Status(fiber.StatusTeapot).Send(data)
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/demo")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(fctx)
	}

	utils.AssertEqual(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
	utils.AssertEqual(b, true, len(fctx.Response.Body()) > 30000)
}

// go test -v -run=^$ -bench=Benchmark_Cache_Storage -benchmem -count=4
func Benchmark_Cache_Storage(b *testing.B) {
	app := fiber.New()

	app.Use(New(Config{
		Storage: memory.New(),
	}))

	app.Get("/demo", func(c *fiber.Ctx) error {
		data, _ := ioutil.ReadFile("../../.github/README.md")
		return c.Status(fiber.StatusTeapot).Send(data)
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/demo")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(fctx)
	}

	utils.AssertEqual(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
	utils.AssertEqual(b, true, len(fctx.Response.Body()) > 30000)
}
