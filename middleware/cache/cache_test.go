// Special thanks to @codemicro for moving this to fiber core
// Original middleware: github.com/codemicro/fiber-cache
package cache

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Cache_CacheControl(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		CacheControl: true,
		Expiration:   10 * time.Second,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, "public, max-age=10", resp.Header.Get(fiber.HeaderCacheControl))
}

func Test_Cache_Expired(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 2 * time.Second}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(fmt.Sprintf("%d", time.Now().UnixNano()))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Sleep until the cache is expired
	time.Sleep(3 * time.Second)

	respCached, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	bodyCached, err := io.ReadAll(respCached.Body)
	require.NoError(t, err)

	if bytes.Equal(body, bodyCached) {
		t.Errorf("Cache should have expired: %s, %s", body, bodyCached)
	}

	// Next response should be also cached
	respCachedNextRound, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	bodyCachedNextRound, err := io.ReadAll(respCachedNextRound.Body)
	require.NoError(t, err)

	if !bytes.Equal(bodyCachedNextRound, bodyCached) {
		t.Errorf("Cache should not have expired: %s, %s", bodyCached, bodyCachedNextRound)
	}
}

func Test_Cache(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		now := fmt.Sprintf("%d", time.Now().UnixNano())
		return c.SendString(now)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	cachedReq := httptest.NewRequest(fiber.MethodGet, "/", nil)
	cachedResp, err := app.Test(cachedReq)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	cachedBody, err := io.ReadAll(cachedResp.Body)
	require.NoError(t, err)

	require.Equal(t, cachedBody, body)
}

// go test -run Test_Cache_WithNoCacheRequestDirective
func Test_Cache_WithNoCacheRequestDirective(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query(c, "id", "1"))
	})

	// Request id = 1
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.Equal(t, []byte("1"), body)
	// Response cached, entry id = 1

	// Request id = 2 without Cache-Control: no-cache
	cachedReq := httptest.NewRequest(fiber.MethodGet, "/?id=2", nil)
	cachedResp, err := app.Test(cachedReq)
	require.NoError(t, err)
	cachedBody, err := io.ReadAll(cachedResp.Body)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedResp.Header.Get("X-Cache"))
	require.Equal(t, []byte("1"), cachedBody)
	// Response not cached, returns cached response, entry id = 1

	// Request id = 2 with Cache-Control: no-cache
	noCacheReq := httptest.NewRequest(fiber.MethodGet, "/?id=2", nil)
	noCacheReq.Header.Set(fiber.HeaderCacheControl, noCache)
	noCacheResp, err := app.Test(noCacheReq)
	require.NoError(t, err)
	noCacheBody, err := io.ReadAll(noCacheResp.Body)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, noCacheResp.Header.Get("X-Cache"))
	require.Equal(t, []byte("2"), noCacheBody)
	// Response cached, returns updated response, entry = 2

	/* Check Test_Cache_WithETagAndNoCacheRequestDirective */
	// Request id = 2 with Cache-Control: no-cache again
	noCacheReq1 := httptest.NewRequest(fiber.MethodGet, "/?id=2", nil)
	noCacheReq1.Header.Set(fiber.HeaderCacheControl, noCache)
	noCacheResp1, err := app.Test(noCacheReq1)
	require.NoError(t, err)
	noCacheBody1, err := io.ReadAll(noCacheResp1.Body)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, noCacheResp1.Header.Get("X-Cache"))
	require.Equal(t, []byte("2"), noCacheBody1)
	// Response cached, returns updated response, entry = 2

	// Request id = 1 without Cache-Control: no-cache
	cachedReq1 := httptest.NewRequest(fiber.MethodGet, "/", nil)
	cachedResp1, err := app.Test(cachedReq1)
	require.NoError(t, err)
	cachedBody1, err := io.ReadAll(cachedResp1.Body)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedResp1.Header.Get("X-Cache"))
	require.Equal(t, []byte("2"), cachedBody1)
	// Response not cached, returns cached response, entry id = 2
}

// go test -run Test_Cache_WithETagAndNoCacheRequestDirective
func Test_Cache_WithETagAndNoCacheRequestDirective(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(
		etag.New(),
		New(),
	)

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query(c, "id", "1"))
	})

	// Request id = 1
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	// Response cached, entry id = 1

	// If response status 200
	etagToken := resp.Header.Get("Etag")

	// Request id = 2 with ETag but without Cache-Control: no-cache
	cachedReq := httptest.NewRequest(fiber.MethodGet, "/?id=2", nil)
	cachedReq.Header.Set(fiber.HeaderIfNoneMatch, etagToken)
	cachedResp, err := app.Test(cachedReq)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedResp.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusNotModified, cachedResp.StatusCode)
	// Response not cached, returns cached response, entry id = 1, status not modified

	// Request id = 2 with ETag and Cache-Control: no-cache
	noCacheReq := httptest.NewRequest(fiber.MethodGet, "/?id=2", nil)
	noCacheReq.Header.Set(fiber.HeaderCacheControl, noCache)
	noCacheReq.Header.Set(fiber.HeaderIfNoneMatch, etagToken)
	noCacheResp, err := app.Test(noCacheReq)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, noCacheResp.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusOK, noCacheResp.StatusCode)
	// Response cached, returns updated response, entry id = 2

	// If response status 200
	etagToken = noCacheResp.Header.Get("Etag")

	// Request id = 2 with ETag and Cache-Control: no-cache again
	noCacheReq1 := httptest.NewRequest(fiber.MethodGet, "/?id=2", nil)
	noCacheReq1.Header.Set(fiber.HeaderCacheControl, noCache)
	noCacheReq1.Header.Set(fiber.HeaderIfNoneMatch, etagToken)
	noCacheResp1, err := app.Test(noCacheReq1)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, noCacheResp1.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusNotModified, noCacheResp1.StatusCode)
	// Response cached, returns updated response, entry id = 2, status not modified

	// Request id = 1 without ETag and Cache-Control: no-cache
	cachedReq1 := httptest.NewRequest(fiber.MethodGet, "/", nil)
	cachedResp1, err := app.Test(cachedReq1)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedResp1.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusOK, cachedResp1.StatusCode)
	// Response not cached, returns cached response, entry id = 2
}

// go test -run Test_Cache_WithNoStoreRequestDirective
func Test_Cache_WithNoStoreRequestDirective(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query(c, "id", "1"))
	})

	// Request id = 2
	noStoreReq := httptest.NewRequest(fiber.MethodGet, "/?id=2", nil)
	noStoreReq.Header.Set(fiber.HeaderCacheControl, noStore)
	noStoreResp, err := app.Test(noStoreReq)
	require.NoError(t, err)
	noStoreBody, err := io.ReadAll(noStoreResp.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("2"), noStoreBody)
	// Response not cached, returns updated response
}

func Test_Cache_WithSeveralRequests(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		CacheControl: true,
		Expiration:   10 * time.Second,
	}))

	app.Get("/:id", func(c fiber.Ctx) error {
		return c.SendString(c.Params("id"))
	})

	for runs := 0; runs < 10; runs++ {
		for i := 0; i < 10; i++ {
			func(id int) {
				rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, fmt.Sprintf("/%d", id), nil))
				require.NoError(t, err)

				defer func(body io.ReadCloser) {
					err := body.Close()
					require.NoError(t, err)
				}(rsp.Body)

				idFromServ, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)

				a, err := strconv.Atoi(string(idFromServ))
				require.NoError(t, err)

				// SomeTimes,The id is not equal with a
				require.Equal(t, id, a)
			}(i)
		}
	}
}

func Test_Cache_Invalid_Expiration(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	cache := New(Config{Expiration: 0 * time.Second})
	app.Use(cache)

	app.Get("/", func(c fiber.Ctx) error {
		now := fmt.Sprintf("%d", time.Now().UnixNano())
		return c.SendString(now)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	cachedReq := httptest.NewRequest(fiber.MethodGet, "/", nil)
	cachedResp, err := app.Test(cachedReq)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	cachedBody, err := io.ReadAll(cachedResp.Body)
	require.NoError(t, err)

	require.Equal(t, cachedBody, body)
}

func Test_Cache_Get(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New())

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query[string](c, "cache"))
	})

	app.Get("/get", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query[string](c, "cache"))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/?cache=123", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/?cache=12345", nil))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "12345", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/get?cache=123", nil))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/get?cache=12345", nil))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(body))
}

func Test_Cache_Post(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Methods: []string{fiber.MethodPost},
	}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query[string](c, "cache"))
	})

	app.Get("/get", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query[string](c, "cache"))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/?cache=123", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/?cache=12345", nil))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/get?cache=123", nil))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(body))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/get?cache=12345", nil))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "12345", string(body))
}

func Test_Cache_NothingToCache(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{Expiration: -(time.Second * 1)}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(time.Now().String())
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	respCached, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	bodyCached, err := io.ReadAll(respCached.Body)
	require.NoError(t, err)

	if bytes.Equal(body, bodyCached) {
		t.Errorf("Cache should have expired: %s, %s", body, bodyCached)
	}
}

func Test_Cache_CustomNext(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Next: func(c fiber.Ctx) bool {
			return c.Response().StatusCode() != fiber.StatusOK
		},
		CacheControl: true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(time.Now().String())
	})

	app.Get("/error", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusInternalServerError).SendString(time.Now().String())
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	respCached, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	bodyCached, err := io.ReadAll(respCached.Body)
	require.NoError(t, err)
	require.True(t, bytes.Equal(body, bodyCached))
	require.NotEmpty(t, respCached.Header.Get(fiber.HeaderCacheControl))

	_, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/error", nil))
	require.NoError(t, err)

	errRespCached, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/error", nil))
	require.NoError(t, err)
	require.Empty(t, errRespCached.Header.Get(fiber.HeaderCacheControl))
}

func Test_CustomKey(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	var called bool
	app.Use(New(Config{KeyGenerator: func(c fiber.Ctx) string {
		called = true
		return utils.CopyString(c.Path())
	}}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hi")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	_, err := app.Test(req)
	require.NoError(t, err)
	require.True(t, called)
}

func Test_CustomExpiration(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	var called bool
	var newCacheTime int
	app.Use(New(Config{ExpirationGenerator: func(c fiber.Ctx, _ *Config) time.Duration {
		called = true
		var err error
		newCacheTime, err = strconv.Atoi(c.GetRespHeader("Cache-Time", "600"))
		require.NoError(t, err)
		return time.Second * time.Duration(newCacheTime)
	}}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Response().Header.Add("Cache-Time", "1")
		now := fmt.Sprintf("%d", time.Now().UnixNano())
		return c.SendString(now)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, 1, newCacheTime)

	// Sleep until the cache is expired
	time.Sleep(1 * time.Second)

	cachedResp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	cachedBody, err := io.ReadAll(cachedResp.Body)
	require.NoError(t, err)

	if bytes.Equal(body, cachedBody) {
		t.Errorf("Cache should have expired: %s, %s", body, cachedBody)
	}

	// Next response should be cached
	cachedRespNextRound, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	cachedBodyNextRound, err := io.ReadAll(cachedRespNextRound.Body)
	require.NoError(t, err)

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

	app.Get("/", func(c fiber.Ctx) error {
		c.Response().Header.Add("X-Foobar", "foobar")
		return c.SendString("hi")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, "foobar", resp.Header.Get("X-Foobar"))

	req = httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, "foobar", resp.Header.Get("X-Foobar"))
}

func Test_CacheHeader(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Expiration: 10 * time.Second,
		Next: func(c fiber.Ctx) bool {
			return c.Response().StatusCode() != fiber.StatusOK
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendString(fiber.Query[string](c, "cache"))
	})

	app.Get("/error", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusInternalServerError).SendString(time.Now().String())
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/?cache=12345", nil))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))

	errRespCached, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/error", nil))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, errRespCached.Header.Get("X-Cache"))
}

func Test_Cache_WithHead(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	handler := func(c fiber.Ctx) error {
		now := fmt.Sprintf("%d", time.Now().UnixNano())
		return c.SendString(now)
	}
	app.Route("/").Get(handler).Head(handler)

	req := httptest.NewRequest(fiber.MethodHead, "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	cachedReq := httptest.NewRequest(fiber.MethodHead, "/", nil)
	cachedResp, err := app.Test(cachedReq)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedResp.Header.Get("X-Cache"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	cachedBody, err := io.ReadAll(cachedResp.Body)
	require.NoError(t, err)

	require.Equal(t, cachedBody, body)
}

func Test_Cache_WithHeadThenGet(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	handler := func(c fiber.Ctx) error {
		return c.SendString(fiber.Query[string](c, "cache"))
	}
	app.Route("/").Get(handler).Head(handler)

	headResp, err := app.Test(httptest.NewRequest(fiber.MethodHead, "/?cache=123", nil))
	require.NoError(t, err)
	headBody, err := io.ReadAll(headResp.Body)
	require.NoError(t, err)
	require.Equal(t, "", string(headBody))
	require.Equal(t, cacheMiss, headResp.Header.Get("X-Cache"))

	headResp, err = app.Test(httptest.NewRequest(fiber.MethodHead, "/?cache=123", nil))
	require.NoError(t, err)
	headBody, err = io.ReadAll(headResp.Body)
	require.NoError(t, err)
	require.Equal(t, "", string(headBody))
	require.Equal(t, cacheHit, headResp.Header.Get("X-Cache"))

	getResp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?cache=123", nil))
	require.NoError(t, err)
	getBody, err := io.ReadAll(getResp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(getBody))
	require.Equal(t, cacheMiss, getResp.Header.Get("X-Cache"))

	getResp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/?cache=123", nil))
	require.NoError(t, err)
	getBody, err = io.ReadAll(getResp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(getBody))
	require.Equal(t, cacheHit, getResp.Header.Get("X-Cache"))
}

func Test_CustomCacheHeader(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		CacheHeader: "Cache-Status",
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("Cache-Status"))
}

// Because time points are updated once every X milliseconds, entries in tests can often have
// equal expiration times and thus be in an random order. This closure hands out increasing
// time intervals to maintain strong ascending order of expiration
func stableAscendingExpiration() func(c1 fiber.Ctx, c2 *Config) time.Duration {
	i := 0
	return func(_ fiber.Ctx, _ *Config) time.Duration {
		i++
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

	app.Get("/*", func(c fiber.Ctx) error {
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
		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, tcase[0], nil))
		require.NoError(t, err)
		require.Equal(t, tcase[1], rsp.Header.Get("X-Cache"), fmt.Sprintf("Case %v", idx))
	}
}

func Test_Cache_MaxBytesSizes(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		MaxBytes:            7,
		ExpirationGenerator: stableAscendingExpiration(),
	}))

	app.Get("/*", func(c fiber.Ctx) error {
		path := c.Context().URI().LastPathSegment()
		size, err := strconv.Atoi(string(path))
		require.NoError(t, err)
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
		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, tcase[0], nil))
		require.NoError(t, err)
		require.Equal(t, tcase[1], rsp.Header.Get("X-Cache"), fmt.Sprintf("Case %v", idx))
	}
}

// go test -v -run=^$ -bench=Benchmark_Cache -benchmem -count=4
func Benchmark_Cache(b *testing.B) {
	app := fiber.New()

	app.Use(New())

	app.Get("/demo", func(c fiber.Ctx) error {
		data, _ := os.ReadFile("../../.github/README.md") //nolint:errcheck // We're inside a benchmark
		return c.Status(fiber.StatusTeapot).Send(data)
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/demo")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(fctx)
	}

	require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
	require.Greater(b, len(fctx.Response.Body()), 30000)
}

// go test -v -run=^$ -bench=Benchmark_Cache_Storage -benchmem -count=4
func Benchmark_Cache_Storage(b *testing.B) {
	app := fiber.New()

	app.Use(New(Config{
		Storage: memory.New(),
	}))

	app.Get("/demo", func(c fiber.Ctx) error {
		data, _ := os.ReadFile("../../.github/README.md") //nolint:errcheck // We're inside a benchmark
		return c.Status(fiber.StatusTeapot).Send(data)
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/demo")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(fctx)
	}

	require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
	require.Greater(b, len(fctx.Response.Body()), 30000)
}

func Benchmark_Cache_AdditionalHeaders(b *testing.B) {
	app := fiber.New()
	app.Use(New(Config{
		StoreResponseHeaders: true,
	}))

	app.Get("/demo", func(c fiber.Ctx) error {
		c.Response().Header.Add("X-Foobar", "foobar")
		return c.SendStatus(418)
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/demo")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(fctx)
	}

	require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
	require.Equal(b, []byte("foobar"), fctx.Response.Header.Peek("X-Foobar"))
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

			app.Get("/*", func(c fiber.Ctx) error {
				return c.Status(fiber.StatusTeapot).SendString("1")
			})

			h := app.Handler()
			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(fiber.MethodGet)

			b.ReportAllocs()
			b.ResetTimer()

			for n := 0; n < b.N; n++ {
				fctx.Request.SetRequestURI(fmt.Sprintf("/%v", n))
				h(fctx)
			}

			require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
		})
	}
}
