package cache

import (
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func TestCacheAgeHeader(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 2 * time.Second}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, "0", resp.Header.Get(fiber.HeaderAge))

	time.Sleep(1 * time.Second)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	age, err := strconv.Atoi(resp.Header.Get(fiber.HeaderAge))
	require.NoError(t, err)
	require.Greater(t, age, 0)
}

func TestCacheNoStoreDirective(t *testing.T) {
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

func TestCacheControlNotOverwritten(t *testing.T) {
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

func TestCacheMaxAgeDirective(t *testing.T) {
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
