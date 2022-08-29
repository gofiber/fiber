// Special thanks to @codemicro for moving this to fiber core
// Original middleware: github.com/codemicro/fiber-cache
package cache

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/storage/memory"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

func Test_Cache_CacheControl(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 2 * time.Second}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(fmt.Sprintf("%d", time.Now().UnixNano()))
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	// Sleep until the cache is expired
	time.Sleep(3 * time.Second)

	respCached, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	bodyCached, err := ioutil.ReadAll(respCached.Body)
	utils.AssertEqual(t, nil, err)

	if bytes.Equal(body, bodyCached) {
		t.Errorf("Cache should have expired: %s, %s", body, bodyCached)
	}

	// Next response should be also cached
	respCachedNextRound, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	bodyCachedNextRound, err := ioutil.ReadAll(respCachedNextRound.Body)
	utils.AssertEqual(t, nil, err)

	if !bytes.Equal(bodyCachedNextRound, bodyCached) {
		t.Errorf("Cache should not have expired: %s, %s", bodyCached, bodyCachedNextRound)
	}
}

func Test_Cache(t *testing.T) {
	t.Parallel()

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

func Test_Cache_WithSeveralRequests(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		CacheControl: true,
		Expiration:   10 * time.Second,
	}))

	app.Get("/:id", func(c *fiber.Ctx) error {
		return c.SendString(c.Params("id"))
	})

	for runs := 0; runs < 10; runs++ {
		for i := 0; i < 10; i++ {
			func(id int) {
				rsp, err := app.Test(httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%d", id), nil))
				utils.AssertEqual(t, nil, err)

				defer func(Body io.ReadCloser) {
					err := Body.Close()
					utils.AssertEqual(t, nil, err)
				}(rsp.Body)

				idFromServ, err := ioutil.ReadAll(rsp.Body)
				utils.AssertEqual(t, nil, err)

				a, err := strconv.Atoi(string(idFromServ))
				utils.AssertEqual(t, nil, err)

				// SomeTimes,The id is not equal with a
				utils.AssertEqual(t, id, a)
			}(i)
		}
	}
}

func Test_Cache_Invalid_Expiration(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Next: func(c *fiber.Ctx) bool {
			return c.Response().StatusCode() != fiber.StatusOK
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
	t.Parallel()

	app := fiber.New()
	var called bool
	app.Use(New(Config{KeyGenerator: func(c *fiber.Ctx) string {
		called = true
		return utils.CopyString(c.Path())
	}}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("hi")
	})

	req := httptest.NewRequest("GET", "/", nil)
	_, err := app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, called)
}

func Test_CustomExpiration(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	var called bool
	var newCacheTime int
	app.Use(New(Config{ExpirationGenerator: func(c *fiber.Ctx, cfg *Config) time.Duration {
		called = true
		newCacheTime, _ = strconv.Atoi(c.GetRespHeader("Cache-Time", "600"))
		return time.Second * time.Duration(newCacheTime)
	}}))

	app.Get("/", func(c *fiber.Ctx) error {
		c.Response().Header.Add("Cache-Time", "1")
		now := fmt.Sprintf("%d", time.Now().UnixNano())
		return c.SendString(now)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, called)
	utils.AssertEqual(t, 1, newCacheTime)

	// Sleep until the cache is expired
	time.Sleep(1 * time.Second)

	cachedResp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	cachedBody, err := ioutil.ReadAll(cachedResp.Body)
	utils.AssertEqual(t, nil, err)

	if bytes.Equal(body, cachedBody) {
		t.Errorf("Cache should have expired: %s, %s", body, cachedBody)
	}

	// Next response should be cached
	cachedRespNextRound, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	cachedBodyNextRound, err := ioutil.ReadAll(cachedRespNextRound.Body)
	utils.AssertEqual(t, nil, err)

	if !bytes.Equal(cachedBodyNextRound, cachedBody) {
		t.Errorf("Cache should not have expired: %s, %s", cachedBodyNextRound, cachedBody)
	}
}

func Test_AdditionalE2EResponseHeaders(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		StoreResponseHeaders: true,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		c.Response().Header.Add("X-Foobar", "foobar")
		return c.SendString("hi")
	})

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "foobar", resp.Header.Get("X-Foobar"))

	req = httptest.NewRequest("GET", "/", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "foobar", resp.Header.Get("X-Foobar"))
}

func Test_CacheHeader(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Expiration: 10 * time.Second,
		Next: func(c *fiber.Ctx) bool {
			return c.Response().StatusCode() != fiber.StatusOK
		},
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	app.Post("/", func(c *fiber.Ctx) error {
		return c.SendString(c.Query("cache"))
	})

	app.Get("/error", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusInternalServerError).SendString(time.Now().String())
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, cacheHit, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest("POST", "/?cache=12345", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, cacheUnreachable, resp.Header.Get("X-Cache"))

	errRespCached, err := app.Test(httptest.NewRequest("GET", "/error", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, cacheUnreachable, errRespCached.Header.Get("X-Cache"))
}

func Test_Cache_WithHead(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		now := fmt.Sprintf("%d", time.Now().UnixNano())
		return c.SendString(now)
	})

	req := httptest.NewRequest("HEAD", "/", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, cacheMiss, resp.Header.Get("X-Cache"))

	cachedReq := httptest.NewRequest("HEAD", "/", nil)
	cachedResp, err := app.Test(cachedReq)
	utils.AssertEqual(t, cacheHit, cachedResp.Header.Get("X-Cache"))

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	cachedBody, err := ioutil.ReadAll(cachedResp.Body)
	utils.AssertEqual(t, nil, err)

	utils.AssertEqual(t, cachedBody, body)
}

func Test_Cache_WithHeadThenGet(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString(c.Query("cache"))
	})

	headResp, err := app.Test(httptest.NewRequest("HEAD", "/?cache=123", nil))
	utils.AssertEqual(t, nil, err)
	headBody, err := ioutil.ReadAll(headResp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "", string(headBody))
	utils.AssertEqual(t, cacheMiss, headResp.Header.Get("X-Cache"))

	headResp, err = app.Test(httptest.NewRequest("HEAD", "/?cache=123", nil))
	utils.AssertEqual(t, nil, err)
	headBody, err = ioutil.ReadAll(headResp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "", string(headBody))
	utils.AssertEqual(t, cacheHit, headResp.Header.Get("X-Cache"))

	getResp, err := app.Test(httptest.NewRequest("GET", "/?cache=123", nil))
	utils.AssertEqual(t, nil, err)
	getBody, err := ioutil.ReadAll(getResp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "123", string(getBody))
	utils.AssertEqual(t, cacheMiss, getResp.Header.Get("X-Cache"))

	getResp, err = app.Test(httptest.NewRequest("GET", "/?cache=123", nil))
	utils.AssertEqual(t, nil, err)
	getBody, err = ioutil.ReadAll(getResp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "123", string(getBody))
	utils.AssertEqual(t, cacheHit, getResp.Header.Get("X-Cache"))
}

func Test_CustomCacheHeader(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		CacheHeader: "Cache-Status",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, cacheMiss, resp.Header.Get("Cache-Status"))
}

// Because time points are updated once every X milliseconds, entries in tests can often have
// equal expiration times and thus be in an random order. This closure hands out increasing
// time intervals to maintain strong ascending order of expiration
func stableAscendingExpiration() func(c1 *fiber.Ctx, c2 *Config) time.Duration {
	i := 0
	return func(c1 *fiber.Ctx, c2 *Config) time.Duration {
		i += 1
		return time.Hour * time.Duration(i)
	}
}

func Test_Cache_MaxBytesOrder(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		MaxBytes:            2,
		ExpirationGenerator: stableAscendingExpiration(),
	}))

	app.Get("/*", func(c *fiber.Ctx) error {
		return c.SendString("1")
	})

	cases := [][]string{
		// Insert a, b into cache of size 2 bytes (responses are 1 byte)
		{"/a", cacheMiss},
		{"/b", cacheMiss},
		{"/a", cacheHit},
		{"/b", cacheHit},
		// Add c -> a evicted
		{"/c", cacheMiss},
		{"/b", cacheHit},
		// Add a again -> b evicted
		{"/a", cacheMiss},
		{"/c", cacheHit},
		// Add b -> c evicted
		{"/b", cacheMiss},
		{"/c", cacheMiss},
	}

	for idx, tcase := range cases {
		rsp, err := app.Test(httptest.NewRequest("GET", tcase[0], nil))
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, tcase[1], rsp.Header.Get("X-Cache"), fmt.Sprintf("Case %v", idx))
	}
}

func Test_Cache_MaxBytesSizes(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		MaxBytes:            7,
		ExpirationGenerator: stableAscendingExpiration(),
	}))

	app.Get("/*", func(c *fiber.Ctx) error {
		path := c.Context().URI().LastPathSegment()
		size, _ := strconv.Atoi(string(path))
		return c.Send(make([]byte, size))
	})

	cases := [][]string{
		{"/1", cacheMiss},
		{"/2", cacheMiss},
		{"/3", cacheMiss},
		{"/4", cacheMiss}, // 1+2+3+4 > 7 => 1,2 are evicted now
		{"/3", cacheHit},
		{"/1", cacheMiss},
		{"/2", cacheMiss},
		{"/8", cacheUnreachable}, // too big to cache -> unreachable
	}

	for idx, tcase := range cases {
		rsp, err := app.Test(httptest.NewRequest("GET", tcase[0], nil))
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, tcase[1], rsp.Header.Get("X-Cache"), fmt.Sprintf("Case %v", idx))
	}
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

func Benchmark_Cache_AdditionalHeaders(b *testing.B) {
	app := fiber.New()
	app.Use(New(Config{
		StoreResponseHeaders: true,
	}))

	app.Get("/demo", func(c *fiber.Ctx) error {
		c.Response().Header.Add("X-Foobar", "foobar")
		return c.SendStatus(418)
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
	utils.AssertEqual(b, []byte("foobar"), fctx.Response.Header.Peek("X-Foobar"))
}

func Benchmark_Cache_MaxSize(b *testing.B) {
	// The benchmark is run with three different MaxSize parameters
	// 1) 0:        Tracking is disabled = no overhead
	// 2) MaxInt32: Enough to store all entries = no removals
	// 3) 100:      Small size = constant insertions and removals
	cases := []uint{0, math.MaxUint32, 100}
	names := []string{"Disabled", "Unlim", "LowBounded"}
	for i, size := range cases {
		b.Run(names[i], func(b *testing.B) {
			app := fiber.New()
			app.Use(New(Config{MaxBytes: size}))

			app.Get("/*", func(c *fiber.Ctx) error {
				return c.Status(fiber.StatusTeapot).SendString("1")
			})

			h := app.Handler()
			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod("GET")

			b.ReportAllocs()
			b.ResetTimer()

			for n := 0; n < b.N; n++ {
				fctx.Request.SetRequestURI(fmt.Sprintf("/%v", n))
				h(fctx)
			}

			utils.AssertEqual(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
		})
	}
}
