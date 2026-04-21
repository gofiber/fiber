package cache

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// Test_Cache_Security_DoS_ExcessiveQueryParams tests protection against DoS via excessive query parameters
func Test_Cache_Security_DoS_ExcessiveQueryParams(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 1 * time.Hour}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString("response")
	})

	// Build a URL with more than maxQueryParams (128) parameters
	queryParams := make([]string, 150)
	for i := 0; i < 150; i++ {
		queryParams[i] = fmt.Sprintf("param%d=value%d", i, i)
	}
	url := "/?" + strings.Join(queryParams, "&")

	// First request should be cached (but with hashed key due to param limit)
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, url, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.Equal(t, 1, count)

	// Second request should hit cache
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, url, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Equal(t, 1, count, "Handler should not be called on cache hit")
}

// Test_Cache_Security_DoS_ExcessiveQueryBuffer tests protection against DoS via query buffer growth
func Test_Cache_Security_DoS_ExcessiveQueryBuffer(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{
		ReadBufferSize: 16384, // Increase buffer to accommodate large query strings for testing
	})
	app.Use(New(Config{Expiration: 1 * time.Hour}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString("response")
	})

	// Build a URL with parameters that expand significantly when URL-escaped
	// Using characters that require escaping (e.g., "=" becomes "%3D", 3x larger)
	specialChars := strings.Repeat("=", 50) // Each "=" becomes "%3D"
	queryParams := make([]string, 30)
	for i := 0; i < 30; i++ {
		queryParams[i] = fmt.Sprintf("key%d=%s", i, specialChars)
	}
	url := "/?" + strings.Join(queryParams, "&")

	// Request should not crash or exhaust memory
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, url, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	// Second request should hit cache (with hashed key due to buffer limit)
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, url, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Equal(t, 1, count, "Handler should not be called on cache hit")
}

// Test_Cache_Security_DoS_ExcessiveVaryHeaders tests protection against DoS via excessive Vary headers
func Test_Cache_Security_DoS_ExcessiveVaryHeaders(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 1 * time.Hour}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		// Generate more than maxVaryHeaders (32) headers
		varyHeaders := make([]string, 50)
		for i := 0; i < 50; i++ {
			varyHeaders[i] = fmt.Sprintf("X-Custom-Header-%d", i)
		}
		c.Set(fiber.HeaderVary, strings.Join(varyHeaders, ", "))
		return c.SendString("response")
	})

	// First request should not be cached due to excessive Vary headers (treated as Vary: *)
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	require.Equal(t, 1, count)

	// Second request should also not be cached
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	require.Equal(t, 2, count, "Handler should be called each time when uncacheable")
}

// Test_Cache_Security_LongPathSegmentHashed tests that long path segments are properly hashed
func Test_Cache_Security_LongPathSegmentHashed(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 1 * time.Hour}))

	var count int
	// Create a very long path (>192 chars which is maxKeyDimensionSegmentLength)
	longPath := "/" + strings.Repeat("a", 300)
	app.Get(longPath, func(c fiber.Ctx) error {
		count++
		return c.SendString("response")
	})

	// First request
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, longPath, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.Equal(t, 1, count)

	// Second request should hit cache with hashed key
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, longPath, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Equal(t, 1, count, "Handler should not be called on cache hit")
}

// Test_Cache_Security_MalformedQueryString tests handling of malformed query strings
func Test_Cache_Security_MalformedQueryString(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 1 * time.Hour}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString("response")
	})

	// Malformed query string with invalid encoding
	malformedURL := "/?invalid=%ZZ%XX"

	// Should not crash and should handle gracefully
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, malformedURL, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	// Second request should hit cache (malformed query is hashed as-is)
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, malformedURL, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Equal(t, 1, count)
}

// Test_Cache_Security_HeaderInjection tests that header values cannot inject into cache keys
func Test_Cache_Security_HeaderInjection(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 1 * time.Hour,
		KeyHeaders: []string{"X-Custom-Header"},
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString("response-" + c.Get("X-Custom-Header"))
	})

	// Try to inject delimiters used in key generation (avoiding null bytes which are invalid in HTTP)
	injectionAttempts := []string{
		"value|q=injected",
		"value|h=injected",
		"value|c=injected",
		"value|vary|injected",
		"sha256:fakehash",
	}

	for _, injection := range injectionAttempts {
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set("X-Custom-Header", injection)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "response-"+injection, string(body))
	}

	// Each injection attempt should create a distinct cache entry
	// (no collision through injection)
	require.Equal(t, len(injectionAttempts), count)
}

// Test_Cache_Security_CookieInjection tests that cookie values cannot inject into cache keys
func Test_Cache_Security_CookieInjection(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 1 * time.Hour,
		KeyCookies: []string{"session"},
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString("response-" + c.Cookies("session"))
	})

	// Try to inject delimiters (avoiding null bytes which are invalid in HTTP)
	injectionAttempts := []string{
		"value|injected",
		"value:injected",
		"value|vary|injected",
		"sha256:fakehash",
	}

	for _, injection := range injectionAttempts {
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set("Cookie", "session="+injection)

		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	}

	// Each injection attempt should create distinct cache entries
	require.Equal(t, len(injectionAttempts), count)
}

// Test_Cache_Security_Concurrent_QueryParamDoS tests concurrent requests with excessive params
func Test_Cache_Security_Concurrent_QueryParamDoS(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 1 * time.Hour}))

	var count atomic.Int32
	app.Get("/", func(c fiber.Ctx) error {
		count.Add(1)
		return c.SendString("response")
	})

	// Build URL with excessive parameters
	queryParams := make([]string, 200)
	for i := 0; i < 200; i++ {
		queryParams[i] = fmt.Sprintf("p%d=v%d", i, i)
	}
	url := "/?" + strings.Join(queryParams, "&")

	// Run concurrent requests
	const numRequests = 100
	var wg sync.WaitGroup
	wg.Add(numRequests)

	// Track errors in goroutines
	var errCount atomic.Int32

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, url, http.NoBody))
			if err != nil {
				errCount.Add(1)
				return
			}
			if resp.StatusCode != fiber.StatusOK {
				errCount.Add(1)
			}
		}()
	}

	wg.Wait()

	// Should have handled all requests without crashing or errors
	require.Equal(t, int32(0), errCount.Load(), "No errors should occur during concurrent requests")

	// First request creates cache, rest should hit it
	require.LessOrEqual(t, count.Load(), int32(numRequests))
}

// Test_Cache_Security_QueryParameterRepeated tests handling of repeated query parameters
func Test_Cache_Security_QueryParameterRepeated(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 1 * time.Hour}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString("response")
	})

	// Test with 100 values for the same parameter
	values := make([]string, 100)
	for i := 0; i < 100; i++ {
		values[i] = fmt.Sprintf("key=%d", i)
	}
	url := "/?" + strings.Join(values, "&")

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, url, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	// Second request should hit cache (hashed due to param count)
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, url, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Equal(t, 1, count)
}

// Test_Cache_Security_EmptyVaryHeaders tests handling of empty vary header entries
func Test_Cache_Security_EmptyVaryHeaders(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 1 * time.Hour}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		// Empty and whitespace vary entries should be ignored
		c.Set(fiber.HeaderVary, "Accept, , , ,   ,Accept-Encoding")
		return c.SendString("response")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Equal(t, 1, count)
}
