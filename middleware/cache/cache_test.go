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
	"github.com/gofiber/fiber/v2/utils"
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

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)

	resp, err = app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "max-age=10", resp.Header.Get(fiber.HeaderCacheControl))
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
	time.Sleep(expiration)

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
