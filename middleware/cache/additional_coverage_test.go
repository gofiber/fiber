package cache

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// Test_Cache_MaxBytes_AdditionalCoverage provides additional coverage for MaxBytes code paths
func Test_Cache_MaxBytes_AdditionalCoverage(t *testing.T) {
	t.Parallel()

	t.Run("defer unreserves on early expiration", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()

		app.Use(New(Config{
			MaxBytes:   100,
			Expiration: 1 * time.Hour,
		}))

		app.Get("/test", func(c fiber.Ctx) error {
			c.Response().Header.Set("Cache-Control", "max-age=0")
			c.Response().Header.Set("Age", "1")
			return c.SendString("test")
		})

		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheUnreachable, rsp.Header.Get("X-Cache"))
	})

	t.Run("evicts multiple entries successfully", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()

		app.Use(New(Config{
			MaxBytes:            10,
			ExpirationGenerator: stableAscendingExpiration(),
		}))

		app.Get("/*", func(c fiber.Ctx) error {
			path := c.Path()
			if path == "/large" {
				return c.Send(make([]byte, 8))
			}
			return c.Send(make([]byte, 2))
		})

		// Cache three small entries
		_, _ = app.Test(httptest.NewRequest(fiber.MethodGet, "/small1", http.NoBody))
		_, _ = app.Test(httptest.NewRequest(fiber.MethodGet, "/small2", http.NoBody))
		_, _ = app.Test(httptest.NewRequest(fiber.MethodGet, "/small3", http.NoBody))

		// Cache large entry - should trigger eviction
		rsp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/large", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp.Header.Get("X-Cache"))

		// Verify large is cached
		rsp2, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/large", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheHit, rsp2.Header.Get("X-Cache"))
	})

	t.Run("zero MaxBytes allows unlimited", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()

		app.Use(New(Config{
			MaxBytes:   0,
			Expiration: 1 * time.Hour,
		}))

		app.Get("/test", func(c fiber.Ctx) error {
			return c.Send(make([]byte, 1000))
		})

		rsp1, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheMiss, rsp1.Header.Get("X-Cache"))

		rsp2, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheHit, rsp2.Header.Get("X-Cache"))
	})

	t.Run("deletion failure with storage", func(t *testing.T) {
		t.Parallel()
		storage := newFailingCacheStorage()
		app := fiber.New()

		app.Use(New(Config{
			MaxBytes:            5,
			Expiration:          1 * time.Hour,
			Storage:             storage,
			ExpirationGenerator: stableAscendingExpiration(),
		}))

		app.Get("/*", func(c fiber.Ctx) error {
			return c.Send(make([]byte, 3))
		})

		// Cache first entry
		_, _ = app.Test(httptest.NewRequest(fiber.MethodGet, "/a", http.NoBody))

		// Make all deletions fail
		for k := range storage.data {
			storage.errs["del|"+k] = errors.New("deletion failed")
		}

		// Try to cache second entry
		_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/b", http.NoBody))
		if err != nil {
			require.Contains(t, err.Error(), "failed to delete key")
		}
	})
}
