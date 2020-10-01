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

func Test_Cache_Expired(t *testing.T) {
	app := fiber.New()

	expiration := 1 * time.Second
	app.Use(New(Config{Expiration: expiration}))

	app.Get("/", func(c *fiber.Ctx) error {
		now := fmt.Sprintf("%d", time.Now().UnixNano())
		return c.SendString(now)
	})

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)
	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)

	// Sleep until the cache is expired
	time.Sleep(expiration)

	cachedReq := httptest.NewRequest("GET", "/", nil)
	cached, err := app.Test(cachedReq)
	utils.AssertEqual(t, nil, err)
	cachedBody, err := ioutil.ReadAll(cached.Body)
	utils.AssertEqual(t, nil, err)

	if bytes.Equal(body, cachedBody) {
		t.Errorf("Cache should have expired: %v, %v", body, cachedBody)
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
		now := fmt.Sprintf("%d", time.Now().UnixNano())
		return c.SendString(now)
	})

	req := httptest.NewRequest("POST", "/", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)

	notCachedReq := httptest.NewRequest("POST", "/", nil)
	notCachedResp, err := app.Test(notCachedReq)
	utils.AssertEqual(t, nil, err)

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	notCachedBody, err := ioutil.ReadAll(notCachedResp.Body)
	utils.AssertEqual(t, nil, err)

	if bytes.Equal(body, notCachedBody) {
		t.Errorf(t.Name(), " should not be cached, due to POST method, want: %v, got: %v", notCachedBody, body)
	}
}
