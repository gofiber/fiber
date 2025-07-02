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
	count := 0
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
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

	count := 0
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
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

	// Request id = 3 with Cache-Control: NO-CACHE
	noCacheReqUpper := httptest.NewRequest(fiber.MethodGet, "/?id=3", nil)
	noCacheReqUpper.Header.Set(fiber.HeaderCacheControl, "NO-CACHE")
	noCacheRespUpper, err := app.Test(noCacheReqUpper)
	require.NoError(t, err)
	noCacheBodyUpper, err := io.ReadAll(noCacheRespUpper.Body)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, noCacheRespUpper.Header.Get("X-Cache"))
	require.Equal(t, []byte("3"), noCacheBodyUpper)
	// Response cached, returns updated response, entry = 3

	// Request id = 4 with Cache-Control: my-no-cache
	invalidReq := httptest.NewRequest(fiber.MethodGet, "/?id=4", nil)
	invalidReq.Header.Set(fiber.HeaderCacheControl, "my-no-cache")
	invalidResp, err := app.Test(invalidReq)
	require.NoError(t, err)
	invalidBody, err := io.ReadAll(invalidResp.Body)
	require.NoError(t, err)
	require.Equal(t, cacheHit, invalidResp.Header.Get("X-Cache"))
	require.Equal(t, []byte("3"), invalidBody)
	// Response served from cache, existing entry = 3

	// Request id = 4 again without Cache-Control: no-cache
	cachedInvalidReq := httptest.NewRequest(fiber.MethodGet, "/?id=4", nil)
	cachedInvalidResp, err := app.Test(cachedInvalidReq)
	require.NoError(t, err)
	cachedInvalidBody, err := io.ReadAll(cachedInvalidResp.Body)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedInvalidResp.Header.Get("X-Cache"))
	require.Equal(t, []byte("3"), cachedInvalidBody)
	// Response cached, returns cached response, entry id = 3

	// Request id = 1 without Cache-Control: no-cache
	cachedReq1 := httptest.NewRequest(fiber.MethodGet, "/", nil)
	cachedResp1, err := app.Test(cachedReq1)
	require.NoError(t, err)
	cachedBody1, err := io.ReadAll(cachedResp1.Body)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedResp1.Header.Get("X-Cache"))
	require.Equal(t, []byte("3"), cachedBody1)
	// Response not cached, returns cached response, entry id = 3
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

	// Request id = 3 with ETag and Cache-Control: NO-CACHE
	noCacheReqUpper := httptest.NewRequest(fiber.MethodGet, "/?id=3", nil)
	noCacheReqUpper.Header.Set(fiber.HeaderCacheControl, "NO-CACHE")
	noCacheReqUpper.Header.Set(fiber.HeaderIfNoneMatch, etagToken)
	noCacheRespUpper, err := app.Test(noCacheReqUpper)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, noCacheRespUpper.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusOK, noCacheRespUpper.StatusCode)
	// Response cached, returns updated response, entry id = 3

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

	// Request id = 3 with Cache-Control: NO-STORE
	noStoreReqUpper := httptest.NewRequest(fiber.MethodGet, "/?id=3", nil)
	noStoreReqUpper.Header.Set(fiber.HeaderCacheControl, "NO-STORE")
	noStoreRespUpper, err := app.Test(noStoreReqUpper)
	require.NoError(t, err)
	noStoreBodyUpper, err := io.ReadAll(noStoreRespUpper.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("3"), noStoreBodyUpper)
	// Response not cached, returns updated response

	// Request id = 4 with Cache-Control: my-no-store
	invalidReq := httptest.NewRequest(fiber.MethodGet, "/?id=4", nil)
	invalidReq.Header.Set(fiber.HeaderCacheControl, "my-no-store")
	invalidResp, err := app.Test(invalidReq)
	require.NoError(t, err)
	invalidBody, err := io.ReadAll(invalidResp.Body)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, invalidResp.Header.Get("X-Cache"))
	require.Equal(t, []byte("4"), invalidBody)
	// Response cached, returns updated response, entry = 4

	// Request id = 4 again without Cache-Control
	cachedInvalidReq := httptest.NewRequest(fiber.MethodGet, "/?id=4", nil)
	cachedInvalidResp, err := app.Test(cachedInvalidReq)
	require.NoError(t, err)
	cachedInvalidBody, err := io.ReadAll(cachedInvalidResp.Body)
	require.NoError(t, err)
	require.Equal(t, cacheHit, cachedInvalidResp.Header.Get("X-Cache"))
	require.Equal(t, []byte("4"), cachedInvalidBody)
	// Response cached previously, served from cache
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

	count := 0
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
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

	count := 0
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
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

	count := 0
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
	})

	errorCount := 0
	app.Get("/error", func(c fiber.Ctx) error {
		errorCount++
		return c.Status(fiber.StatusInternalServerError).SendString(strconv.Itoa(errorCount))
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

	count := 0
	app.Get("/", func(c fiber.Ctx) error {
		count++
		c.Response().Header.Add("Cache-Time", "1")
		return c.SendString(strconv.Itoa(count))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, 1, newCacheTime)

	// Sleep until the cache is expired
	time.Sleep(1*time.Second + 100*time.Millisecond)

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

	count := 0
	app.Get("/error", func(c fiber.Ctx) error {
		count++
		c.Response().Header.Add("Cache-Time", "1")
		return c.Status(fiber.StatusInternalServerError).SendString(strconv.Itoa(count))
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

	count := 0
	handler := func(c fiber.Ctx) error {
		count++
		c.Response().Header.Add("Cache-Time", "1")
		return c.SendString(strconv.Itoa(count))
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

func Test_CacheInvalidation(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		CacheControl: true,
		CacheInvalidator: func(c fiber.Ctx) bool {
			return fiber.Query[bool](c, "invalidate")
		},
	}))

	count := 0
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(strconv.Itoa(count))
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

	respInvalidate, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?invalidate=true", nil))
	require.NoError(t, err)
	bodyInvalidate, err := io.ReadAll(respInvalidate.Body)
	require.NoError(t, err)
	require.NotEqual(t, body, bodyInvalidate)
}

func Test_CacheInvalidation_noCacheEntry(t *testing.T) {
	t.Parallel()
	t.Run("Cache Invalidator should not be called if no cache entry exist ", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		cacheInvalidatorExecuted := false
		app.Use(New(Config{
			CacheControl: true,
			CacheInvalidator: func(c fiber.Ctx) bool {
				cacheInvalidatorExecuted = true
				return fiber.Query[bool](c, "invalidate")
			},
			MaxBytes: 10 * 1024 * 1024,
		}))
		_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?invalidate=true", nil))
		require.NoError(t, err)
		require.False(t, cacheInvalidatorExecuted)
	})
}

func Test_CacheInvalidation_removeFromHeap(t *testing.T) {
	t.Parallel()
	t.Run("Invalidate and remove from the heap", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{
			CacheControl: true,
			CacheInvalidator: func(c fiber.Ctx) bool {
				return fiber.Query[bool](c, "invalidate")
			},
			MaxBytes: 10 * 1024 * 1024,
		}))

		count := 0
		app.Get("/", func(c fiber.Ctx) error {
			count++
			return c.SendString(strconv.Itoa(count))
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

		respInvalidate, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?invalidate=true", nil))
		require.NoError(t, err)
		bodyInvalidate, err := io.ReadAll(respInvalidate.Body)
		require.NoError(t, err)
		require.NotEqual(t, body, bodyInvalidate)
	})
}

func Test_CacheStorage_CustomHeaders(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		CacheControl: true,
		Storage:      memory.New(),
		MaxBytes:     10 * 1024 * 1024,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Response().Header.Set("Content-Type", "text/xml")
		c.Response().Header.Set("Content-Encoding", "utf8")
		return c.Send([]byte("<xml><value>Test</value></xml>"))
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
		require.Equal(t, tcase[1], rsp.Header.Get("X-Cache"), "Case %v", idx)
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
		path := c.RequestCtx().URI().LastPathSegment()
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
		require.Equal(t, tcase[1], rsp.Header.Get("X-Cache"), "Case %v", idx)
	}
}

func Test_Cache_UncacheableStatusCodes(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	app.Get("/:statusCode", func(c fiber.Ctx) error {
		statusCode, err := strconv.Atoi(c.Params("statusCode"))
		require.NoError(t, err)
		return c.Status(statusCode).SendString("foo")
	})

	uncacheableStatusCodes := []int{
		// Informational responses
		fiber.StatusContinue,
		fiber.StatusSwitchingProtocols,
		fiber.StatusProcessing,
		fiber.StatusEarlyHints,

		// Successful responses
		fiber.StatusCreated,
		fiber.StatusAccepted,
		fiber.StatusResetContent,
		fiber.StatusMultiStatus,
		fiber.StatusAlreadyReported,
		fiber.StatusIMUsed,

		// Redirection responses
		fiber.StatusFound,
		fiber.StatusSeeOther,
		fiber.StatusNotModified,
		fiber.StatusUseProxy,
		fiber.StatusSwitchProxy,
		fiber.StatusTemporaryRedirect,
		fiber.StatusPermanentRedirect,

		// Client error responses
		fiber.StatusBadRequest,
		fiber.StatusUnauthorized,
		fiber.StatusPaymentRequired,
		fiber.StatusForbidden,
		fiber.StatusNotAcceptable,
		fiber.StatusProxyAuthRequired,
		fiber.StatusRequestTimeout,
		fiber.StatusConflict,
		fiber.StatusLengthRequired,
		fiber.StatusPreconditionFailed,
		fiber.StatusRequestEntityTooLarge,
		fiber.StatusUnsupportedMediaType,
		fiber.StatusRequestedRangeNotSatisfiable,
		fiber.StatusExpectationFailed,
		fiber.StatusMisdirectedRequest,
		fiber.StatusUnprocessableEntity,
		fiber.StatusLocked,
		fiber.StatusFailedDependency,
		fiber.StatusTooEarly,
		fiber.StatusUpgradeRequired,
		fiber.StatusPreconditionRequired,
		fiber.StatusTooManyRequests,
		fiber.StatusRequestHeaderFieldsTooLarge,
		fiber.StatusTeapot,
		fiber.StatusUnavailableForLegalReasons,

		// Server error responses
		fiber.StatusInternalServerError,
		fiber.StatusBadGateway,
		fiber.StatusServiceUnavailable,
		fiber.StatusGatewayTimeout,
		fiber.StatusHTTPVersionNotSupported,
		fiber.StatusVariantAlsoNegotiates,
		fiber.StatusInsufficientStorage,
		fiber.StatusLoopDetected,
		fiber.StatusNotExtended,
		fiber.StatusNetworkAuthenticationRequired,
	}
	for _, v := range uncacheableStatusCodes {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, fmt.Sprintf("/%d", v), nil))
		require.NoError(t, err)
		require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
		require.Equal(t, v, resp.StatusCode)
	}
}

func TestCacheAgeHeader(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 10 * time.Second}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, "0", resp.Header.Get(fiber.HeaderAge))

	time.Sleep(4 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	age, err := strconv.Atoi(resp.Header.Get(fiber.HeaderAge))
	require.NoError(t, err)
	require.Positive(t, age)
}

func TestCacheUpstreamAge(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 3 * time.Second}))
	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderAge, "5")
		return c.SendString("hi")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, "5", resp.Header.Get(fiber.HeaderAge))

	time.Sleep(1500 * time.Millisecond)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	age, err := strconv.Atoi(resp.Header.Get(fiber.HeaderAge))
	require.NoError(t, err)
	require.GreaterOrEqual(t, age, 6)
}

func Test_CacheNoStoreDirective(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
}

func Test_CacheControlNotOverwritten(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{CacheControl: true, Expiration: 10 * time.Second, StoreResponseHeaders: true}))
	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "private")
		return c.SendString("ok")
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, "private", resp.Header.Get(fiber.HeaderCacheControl))
}

func Test_CacheMaxAgeDirective(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 10 * time.Second}))
	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "max-age=1")
		return c.SendString("1")
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)

	time.Sleep(1500 * time.Millisecond)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
}

func Test_ParseMaxAge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		header string
		expect time.Duration
		ok     bool
	}{
		{"max-age=60", 60 * time.Second, true},
		{"public, max-age=86400", 86400 * time.Second, true},
		{"no-store", 0, false},
		{"max-age=invalid", 0, false},
		{"public, s-maxage=100, max-age=50", 50 * time.Second, true},
		{"MAX-AGE=20", 20 * time.Second, true},
		{"public , max-age=0", 0, true},
		{"public , max-age", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			t.Parallel()
			d, ok := parseMaxAge(tt.header)
			if tt.ok != ok {
				t.Fatalf("expected ok=%v got %v", tt.ok, ok)
			}
			if ok && d != tt.expect {
				t.Fatalf("expected %v got %v", tt.expect, d)
			}
		})
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

	for b.Loop() {
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

	for b.Loop() {
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

	for b.Loop() {
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

			n := 0
			for b.Loop() {
				n++
				fctx.Request.SetRequestURI(fmt.Sprintf("/%v", n))
				h(fctx)
			}

			require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
		})
	}
}
