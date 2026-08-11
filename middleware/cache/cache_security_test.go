package cache

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
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
	for i := range 150 {
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
	for i := range 30 {
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
		for i := range 50 {
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
	for i := range 200 {
		queryParams[i] = fmt.Sprintf("p%d=v%d", i, i)
	}
	url := "/?" + strings.Join(queryParams, "&")

	// Run concurrent requests
	const numRequests = 100
	var wg sync.WaitGroup
	wg.Add(numRequests)

	// Track errors in goroutines
	var errCount atomic.Int32

	for range numRequests {
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
	for i := range 100 {
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

// Test_Cache_Security_MultiDimensionInjection tests injection with multiple headers and cookies
func Test_Cache_Security_MultiDimensionInjection(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 1 * time.Hour,
		KeyHeaders: []string{"X-Header-1", "X-Header-2"},
		KeyCookies: []string{"cookie1", "cookie2"},
	}))

	var count int
	app.Get("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(fmt.Sprintf("h1=%s,h2=%s,c1=%s,c2=%s",
			c.Get("X-Header-1"), c.Get("X-Header-2"),
			c.Cookies("cookie1"), c.Cookies("cookie2")))
	})

	// Test combinations that should create distinct cache entries.
	// expected is the echoed body; cookie values with octets outside the RFC 6265
	// cookie-octet set (e.g. backslash) are rejected by fasthttp and arrive empty,
	// while header values keep them.
	testCases := []struct {
		header1  string
		header2  string
		cookie1  string
		cookie2  string
		expected string
	}{
		// Normal values
		{"value1", "value2", "cookie1", "cookie2", "h1=value1,h2=value2,c1=cookie1,c2=cookie2"},
		// Injection attempts with delimiters
		{"value|injected", "normal", "normal", "normal", "h1=value|injected,h2=normal,c1=normal,c2=normal"},
		{"normal", "value:injected", "normal", "normal", "h1=normal,h2=value:injected,c1=normal,c2=normal"},
		{"normal", "normal", "value|injected", "normal", "h1=normal,h2=normal,c1=value|injected,c2=normal"},
		{"normal", "normal", "normal", "value:injected", "h1=normal,h2=normal,c1=normal,c2=value:injected"},
		// Multiple delimiters
		{"value|with|pipes", "value:with:colons", "normal", "normal", "h1=value|with|pipes,h2=value:with:colons,c1=normal,c2=normal"},
		// Backslashes: kept in headers, rejected in cookies
		{"value\\with\\backslash", "normal", "normal", "normal", "h1=value\\with\\backslash,h2=normal,c1=normal,c2=normal"},
		{"normal", "normal", "cookie\\value", "normal", "h1=normal,h2=normal,c1=,c2=normal"},
		// Combined escapes
		{"value\\|mixed", "normal", "normal", "normal", "h1=value\\|mixed,h2=normal,c1=normal,c2=normal"},
		{"normal", "value\\:mixed", "normal", "normal", "h1=normal,h2=value\\:mixed,c1=normal,c2=normal"},
	}

	for i, tc := range testCases {
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set("X-Header-1", tc.header1)
		req.Header.Set("X-Header-2", tc.header2)
		req.Header.Set("Cookie", fmt.Sprintf("cookie1=%s; cookie2=%s", tc.cookie1, tc.cookie2))

		resp, err := app.Test(req)
		require.NoError(t, err, "Test case %d failed", i)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "Test case %d failed", i)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Test case %d failed", i)
		require.Equal(t, tc.expected, string(body), "Test case %d failed", i)
	}

	// Each test case should create a distinct cache entry (no collisions)
	require.Equal(t, len(testCases), count, "All test cases should create distinct cache entries")
}

// Test_Cache_Security_KeyHeaderRepeatedFieldLines asserts that every field line
// of a key header reaches the cache key.
//
// A name may arrive on more than one line, and that is equivalent to the
// comma-joined form on the wire (RFC 9110 Section 5.2). The key used to hash
// only the first line, so a request sending the header twice keyed identically
// to one sending just that first value — and got served its cached response.
func Test_Cache_Security_KeyHeaderRepeatedFieldLines(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 1 * time.Hour,
		KeyHeaders: []string{"X-Tenant"},
	}))
	app.Get("/", func(c fiber.Ctx) error {
		values := c.Request().Header.PeekAll("X-Tenant")
		parts := make([]string, 0, len(values))
		for _, v := range values {
			parts = append(parts, string(v)+";")
		}
		return c.SendString("tenant:" + strings.Join(parts, ""))
	})

	do := func(lines ...string) (body, cacheStatus string) { //nolint:nonamedreturns // names document the pair
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		req.Header.SetMethod(fiber.MethodGet)
		req.SetRequestURI("/")
		req.SetHost("example.com")
		for _, l := range lines {
			req.Header.Add("X-Tenant", l)
		}
		fctx := &fasthttp.RequestCtx{}
		fctx.Init(req, nil, nil)
		app.Handler()(fctx)
		return string(fctx.Response.Body()), string(fctx.Response.Header.Peek("X-Cache"))
	}

	body, status := do("public", "acme-private")
	require.Equal(t, cacheMiss, status)
	require.Equal(t, "tenant:public;acme-private;", body)

	body, status = do("public")
	require.Equal(t, cacheMiss, status, "the single-line request must not hit the two-line entry")
	require.Equal(t, "tenant:public;", body)

	// The header being absent is also its own variant, not the same as one
	// present with an empty value.
	_, status = do()
	require.Equal(t, cacheMiss, status)
	_, status = do("")
	require.Equal(t, cacheMiss, status, "an empty value must not key like an absent header")

	// Each variant still caches on its own key.
	body, status = do("public", "acme-private")
	require.Equal(t, cacheHit, status)
	require.Equal(t, "tenant:public;acme-private;", body)

	body, status = do("public")
	require.Equal(t, cacheHit, status)
	require.Equal(t, "tenant:public;", body)
}

// Test_Cache_Security_MultiLineResponseHeaders asserts that the directives
// deciding cacheability are read from every field line, not just the first.
//
// Repeated field lines are equivalent to the comma-joined form on the wire
// (RFC 9110 Section 5.2), so a "Vary: X-Tenant", a "Vary: *" or a
// "Cache-Control: no-store" on a second line binds exactly as much as one on
// the first. Reading only the first line dropped them: the Vary case served one
// tenant's response to another, and the other two cached a response the origin
// had said not to store.
func Test_Cache_Security_MultiLineResponseHeaders(t *testing.T) {
	t.Parallel()

	newApp := func(decorate func(c fiber.Ctx)) *fiber.App {
		app := fiber.New()
		app.Use(New(Config{Expiration: time.Hour}))
		app.Get("/", func(c fiber.Ctx) error {
			decorate(c)
			return c.SendString("tenant:" + c.Get("X-Tenant"))
		})
		return app
	}

	do := func(app *fiber.App, tenant string) (body, cacheStatus string) { //nolint:nonamedreturns // names document the pair
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		req.Header.SetMethod(fiber.MethodGet)
		req.SetRequestURI("/")
		req.SetHost("example.com")
		req.Header.Set("X-Tenant", tenant)
		fctx := &fasthttp.RequestCtx{}
		fctx.Init(req, nil, nil)
		app.Handler()(fctx)
		return string(fctx.Response.Body()), string(fctx.Response.Header.Peek("X-Cache"))
	}

	t.Run("vary on a later line still partitions", func(t *testing.T) {
		t.Parallel()

		app := newApp(func(c fiber.Ctx) {
			c.Response().Header.Add(fiber.HeaderVary, fiber.HeaderAcceptEncoding)
			c.Response().Header.Add(fiber.HeaderVary, "X-Tenant")
		})

		body, status := do(app, "acme")
		require.Equal(t, cacheMiss, status)
		require.Equal(t, "tenant:acme", body)

		body, status = do(app, "globex")
		require.Equal(t, cacheMiss, status, "a different X-Tenant must not hit acme's entry")
		require.Equal(t, "tenant:globex", body)

		// Each tenant still caches on its own key.
		body, status = do(app, "acme")
		require.Equal(t, cacheHit, status)
		require.Equal(t, "tenant:acme", body)
	})

	t.Run("vary star on a later line blocks caching", func(t *testing.T) {
		t.Parallel()

		app := newApp(func(c fiber.Ctx) {
			c.Response().Header.Add(fiber.HeaderVary, fiber.HeaderAcceptEncoding)
			c.Response().Header.Add(fiber.HeaderVary, "*")
		})

		_, status := do(app, "acme")
		require.Equal(t, cacheUnreachable, status)
		body, status := do(app, "globex")
		require.Equal(t, cacheUnreachable, status)
		require.Equal(t, "tenant:globex", body)
	})

	t.Run("no-store on a later line blocks caching", func(t *testing.T) {
		t.Parallel()

		app := newApp(func(c fiber.Ctx) {
			c.Response().Header.Add(fiber.HeaderCacheControl, "max-age=600")
			c.Response().Header.Add(fiber.HeaderCacheControl, "no-store")
		})

		_, status := do(app, "acme")
		require.Equal(t, cacheUnreachable, status)
		body, status := do(app, "globex")
		require.Equal(t, cacheUnreachable, status)
		require.Equal(t, "tenant:globex", body)
	})
}

// Test_JoinedHeader covers the field-line joiner the cacheability checks read
// through, including that a value returned for one name survives the PeekAll
// calls made for the next — those reuse the header's slice-header scratch, not
// the value bytes it points at.
func Test_JoinedHeader(t *testing.T) {
	t.Parallel()

	var h fasthttp.ResponseHeader
	require.Nil(t, joinedHeader(&h, fiber.HeaderVary, true), "absent header")

	h.Add(fiber.HeaderCacheControl, "no-store")
	h.Add(fiber.HeaderVary, fiber.HeaderAcceptEncoding)
	h.Add(fiber.HeaderVary, "X-Tenant")

	cacheControl := joinedHeader(&h, fiber.HeaderCacheControl, true)
	require.Equal(t, "no-store", string(cacheControl))

	require.Equal(t, "Accept-Encoding,X-Tenant", string(joinedHeader(&h, fiber.HeaderVary, true)))
	require.Equal(t, "no-store", string(cacheControl), "an earlier result must survive a later PeekAll")

	var req fasthttp.RequestHeader
	req.Add(fiber.HeaderCacheControl, "max-age=0")
	req.Add(fiber.HeaderCacheControl, "no-store")
	require.Equal(t, "max-age=0,no-store", string(joinedHeader(&req, fiber.HeaderCacheControl, true)))
}

// Test_Cache_Security_CookieKeyedOnEveryFieldLine asserts that a Cookie
// arriving on more than one field line is keyed on in full.
//
// fasthttp answers Cookie from its own store, and what it reports depends on
// whether that store has been collected: uncollected, Peek returns only the
// first field line and PeekAll returns them unmerged; collected, both return one
// merged entry. Reading either directly let a request sending
//
//	Cookie: a=1
//	Cookie: session=<someone>
//
// key like one sending just "a=1", so the second client was served the first
// one's page.
func Test_Cache_Security_CookieKeyedOnEveryFieldLine(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour, KeyHeaders: []string{"Cookie"}}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("page-for:" + c.Cookies("session"))
	})

	do := func(lines ...string) (body, cacheStatus string) { //nolint:nonamedreturns // names document the pair
		parts := []string{"GET / HTTP/1.1\r\nHost: example.com\r\n"}
		for _, l := range lines {
			parts = append(parts, "Cookie: "+l+"\r\n")
		}
		raw := strings.Join(append(parts, "\r\n"), "")

		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

		fctx := &fasthttp.RequestCtx{}
		fctx.Init(req, nil, nil)
		app.Handler()(fctx)
		return string(fctx.Response.Body()), string(fctx.Response.Header.Peek("X-Cache"))
	}

	body, status := do("a=1", "session=attacker")
	require.Equal(t, cacheMiss, status)
	require.Equal(t, "page-for:attacker", body)

	body, status = do("a=1")
	require.Equal(t, cacheMiss, status, "a request without the session cookie must not hit that entry")
	require.Equal(t, "page-for:", body)

	body, status = do("a=1", "session=victim")
	require.Equal(t, cacheMiss, status, "a different session must not hit the attacker's entry")
	require.Equal(t, "page-for:victim", body)

	// Each variant still caches on its own key, and the key does not depend on
	// whether the handler happened to read a cookie first.
	body, status = do("a=1", "session=attacker")
	require.Equal(t, cacheHit, status)
	require.Equal(t, "page-for:attacker", body)

	body, status = do("a=1", "session=victim")
	require.Equal(t, cacheHit, status)
	require.Equal(t, "page-for:victim", body)
}

// Test_Cache_Security_BackslashEscaping tests that backslashes are properly escaped
func Test_Cache_Security_BackslashEscaping(t *testing.T) {
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

	// Test backslash escaping scenarios
	testCases := []string{
		"\\",           // Single backslash
		"\\\\",         // Double backslash
		"\\p",          // Escaped pipe character
		"\\c",          // Escaped colon character
		"value\\|test", // Backslash before pipe
		"value\\:test", // Backslash before colon
		"\\\\p",        // Double backslash then p
		"\\\\c",        // Double backslash then c
	}

	for i, tc := range testCases {
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set("X-Custom-Header", tc)

		resp, err := app.Test(req)
		require.NoError(t, err, "Test case %d (%s) failed", i, tc)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "Test case %d (%s) failed", i, tc)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Test case %d (%s) failed", i, tc)
		require.Equal(t, "response-"+tc, string(body), "Test case %d (%s) failed", i, tc)
	}

	// Each test case should create a distinct cache entry
	require.Equal(t, len(testCases), count, "All backslash test cases should create distinct cache entries")
}

// Test_Cache_Security_DelimiterCollisionPrevention verifies that escaped delimiters don't collide
func Test_Cache_Security_DelimiterCollisionPrevention(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 1 * time.Hour,
		KeyHeaders: []string{"X-Header"},
		KeyCookies: []string{"session"},
	}))

	var responses []string
	app.Get("/", func(c fiber.Ctx) error {
		response := fmt.Sprintf("h=%s,c=%s", c.Get("X-Header"), c.Cookies("session"))
		responses = append(responses, response)
		return c.SendString(response)
	})

	// These pairs should NOT collide after escaping
	testCases := []struct {
		header string
		cookie string
	}{
		{"value1|part2", "normal"}, // Pipe in header
		{"value1", "part2|normal"}, // Different structure but similar
		{"value:test", "cookie"},   // Colon in header
		{"value", "test:cookie"},   // Colon in cookie
		{"a|b:c", "d"},             // Mixed delimiters
		{"a", "b:c|d"},             // Different arrangement
		{"\\|", "test"},            // Backslash-pipe sequence
		{"\\", "|test"},            // Separated backslash and pipe
	}

	for i, tc := range testCases {
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Header.Set("X-Header", tc.header)
		req.Header.Set("Cookie", "session="+tc.cookie)

		resp, err := app.Test(req)
		require.NoError(t, err, "Test case %d failed", i)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "Test case %d failed", i)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Test case %d failed", i)
		expected := fmt.Sprintf("h=%s,c=%s", tc.header, tc.cookie)
		require.Equal(t, expected, string(body), "Test case %d failed", i)
	}

	// All test cases should create distinct cache entries (no collisions from injection)
	require.Len(t, responses, len(testCases), "All test cases should create distinct cache entries")

	// Verify all responses are unique
	seen := make(map[string]bool)
	for _, resp := range responses {
		require.False(t, seen[resp], "Response should be unique: %s", resp)
		seen[resp] = true
	}
}

// Test_Cache_Security_EscapeKeyDelimiters_Unit is a direct regression test for the
// escapeKeyDelimiters function, ensuring backslashes are escaped to prevent collisions
// between e.g. a literal "a\pb" and the escaped form of "a|b" → "a\pb".
func Test_Cache_Security_EscapeKeyDelimiters_Unit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		// Fast path: no special characters
		{"hello", "hello"},
		{"", ""},
		{"foo/bar?baz=1", "foo/bar?baz=1"},
		// Pipe escaping
		{"a|b", "a\\pb"},
		// Colon escaping
		{"a:b", "a\\cb"},
		// Backslash escaping (regression: fast path must also check for \)
		{"a\\b", "a\\\\b"},
		// Backslash-pipe sequence must not collide with escaped pipe
		{"a\\pb", "a\\\\pb"}, // literal \p → \\p (differs from escaped | → \p)
		{"a\\cb", "a\\\\cb"}, // literal \c → \\c (differs from escaped : → \c)
		// Mixed delimiters
		{"k|v:w\\x", "k\\pv\\cw\\\\x"},
		// Multiple consecutive
		{"||", "\\p\\p"},
		{"::", "\\c\\c"},
		{"\\\\", "\\\\\\\\"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("escape_%q", tt.input), func(t *testing.T) {
			t.Parallel()
			result := escapeKeyDelimiters(tt.input)
			require.Equal(t, tt.expected, result, "escapeKeyDelimiters(%q)", tt.input)
		})
	}

	// Verify no collisions between pairs that would collide without backslash escaping
	collisionPairs := [][2]string{
		{"a\\pb", "a|b"}, // literal \p vs escaped |
		{"a\\cb", "a:b"}, // literal \c vs escaped :
		{"\\\\", "\\"},   // double backslash vs single
		{"x\\py", "x|y"},
	}
	for _, pair := range collisionPairs {
		t.Run(fmt.Sprintf("no_collision_%q_vs_%q", pair[0], pair[1]), func(t *testing.T) {
			t.Parallel()
			a := escapeKeyDelimiters(pair[0])
			b := escapeKeyDelimiters(pair[1])
			require.NotEqual(t, a, b, "escapeKeyDelimiters(%q) must differ from escapeKeyDelimiters(%q)", pair[0], pair[1])
		})
	}
}

// Test_Cache_BoundKeySegment_ReservedPrefixHashed verifies that the bounding
// helpers re-hash any segment that already starts with the reserved hashPrefix,
// so a short literal "sha256:..." value cannot collide with a genuinely-hashed
// long segment. Normal short values (including prefixes of "sha256:") must pass
// through verbatim so the fast path is preserved.
func Test_Cache_BoundKeySegment_ReservedPrefixHashed(t *testing.T) {
	t.Parallel()

	hashed := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hashPrefix + hex.EncodeToString(sum[:])
	}

	t.Run("short reserved-prefix value is re-hashed", func(t *testing.T) {
		t.Parallel()

		// Short enough to skip the length bound, but starts with "sha256:".
		input := hashPrefix + strings.Repeat("a", 8)
		require.LessOrEqual(t, len(input), maxKeyDimensionSegmentLength)

		got := boundKeySegment(input)
		require.NotEqual(t, input, got, "reserved-prefix value must not pass through verbatim")
		require.True(t, strings.HasPrefix(got, hashPrefix))
		require.Equal(t, hashed(input), got)

		// appendBoundKeySegment must agree with boundKeySegment.
		require.Equal(t, got, string(appendBoundKeySegment(nil, input)))
	})

	t.Run("normal short value passes through verbatim", func(t *testing.T) {
		t.Parallel()

		const input = "plain"
		require.Equal(t, input, boundKeySegment(input))
		require.Equal(t, input, string(appendBoundKeySegment(nil, input)))
	})

	t.Run("prefix of reserved namespace is not over-hashed", func(t *testing.T) {
		t.Parallel()

		// "sha256" (no colon) is a prefix of "sha256:" but is NOT in the reserved
		// namespace, so it must pass through verbatim (regression guard against
		// reversed HasPrefix arguments).
		const input = "sha256"
		require.Equal(t, input, boundKeySegment(input))
		require.Equal(t, input, string(appendBoundKeySegment(nil, input)))
	})
}

// Test_Cache_Security_QueryBody_RawHashDomain verifies that the QUERY body is
// always hashed over its RAW bytes, never the escaped form. Mixing the two
// domains would let a small body collide with a large one: "a|" repeated escapes
// to "a\p" repeated, which must NOT hash-collide with a body that already
// contains the literal bytes "a\p" repeated.
func Test_Cache_Security_QueryBody_RawHashDomain(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 1 * time.Hour,
		Methods:    []string{fiber.MethodQuery},
	}))

	var count atomic.Int32
	app.Query("/", func(c fiber.Ctx) error {
		count.Add(1)
		return c.SendString("response")
	})

	// bodyA: 130 raw bytes (<=192, takes the verbatim branch); escapes to
	// "a\p" x65 = 195 bytes (>192), so it is hashed over the RAW "a|" x65.
	bodyA := strings.Repeat("a|", 65)
	// bodyB: 195 raw bytes of literal "a\p" (>192), hashed over the RAW bytes.
	// If the small branch hashed the escaped form, bodyA and bodyB would collide.
	bodyB := strings.Repeat("a\\p", 65)
	require.NotEqual(t, bodyA, bodyB)

	doQuery := func(body string) string {
		req := httptest.NewRequest(fiber.MethodQuery, "/", strings.NewReader(body))
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		return resp.Header.Get("X-Cache")
	}

	require.Equal(t, cacheMiss, doQuery(bodyA))
	require.Equal(t, cacheMiss, doQuery(bodyB), "distinct bodies must not collide")
	require.Equal(t, int32(2), count.Load())
	require.Equal(t, cacheHit, doQuery(bodyA), "identical body must hit cache")
	require.Equal(t, int32(2), count.Load())
}

func Test_Cache_Security_QueryBody_CannotInjectAuthorizationSuffix(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 1 * time.Hour,
		Methods:    []string{fiber.MethodQuery},
	}))

	var count atomic.Int32
	app.Query("/", func(c fiber.Ctx) error {
		count.Add(1)
		c.Set(fiber.HeaderCacheControl, "public, max-age=60")
		return c.SendString("handler auth=" + c.Get(fiber.HeaderAuthorization))
	})

	const authHeader = "Bearer victim-token"
	const baseBody = "B"
	authSum := sha256.Sum256([]byte(authHeader))
	authHash := hex.EncodeToString(authSum[:])
	craftedBody := baseBody + "|auth=" + authHash

	authReq := httptest.NewRequest(fiber.MethodQuery, "/", strings.NewReader(baseBody))
	authReq.Header.Set(fiber.HeaderAuthorization, authHeader)
	authResp, err := app.Test(authReq)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, authResp.StatusCode)
	require.Equal(t, cacheMiss, authResp.Header.Get("X-Cache"))

	unauthReq := httptest.NewRequest(fiber.MethodQuery, "/", strings.NewReader(craftedBody))
	unauthResp, err := app.Test(unauthReq)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, unauthResp.StatusCode)
	require.Equal(t, cacheMiss, unauthResp.Header.Get("X-Cache"))

	body, err := io.ReadAll(unauthResp.Body)
	require.NoError(t, err)
	require.Equal(t, "handler auth=", string(body))
	require.Equal(t, int32(2), count.Load())
}

// Test_VaryCookie_KeyIsStableAcrossCookieCollection pins the reason
// keyFieldLines forces fasthttp's lazy cookie collection before reading the
// field lines.
//
// The Vary key is built twice per miss: once before the handler runs, and once
// after, to store under. A handler that reads a cookie triggers collection in
// between, and collection re-serializes every Cookie line into a single merged
// entry. Peeking the raw lines on one side of that and the merged entry on the
// other made the two keys differ, so the entry was stored under a key no lookup
// would ever produce — every request missed, and each one stranded another
// entry.
func Test_VaryCookie_KeyIsStableAcrossCookieCollection(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour}))
	app.Get("/", func(c fiber.Ctx) error {
		c.Response().Header.Set(fiber.HeaderVary, fiber.HeaderCookie)
		// Reading a cookie is what forces collection mid-request.
		return c.SendString("page-for:" + c.Cookies("session"))
	})

	do := func(lines ...string) (string, string) {
		parts := []string{"GET / HTTP/1.1\r\nHost: example.com\r\n"}
		for _, l := range lines {
			parts = append(parts, "Cookie: "+l+"\r\n")
		}
		raw := strings.Join(append(parts, "\r\n"), "")

		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

		fctx := &fasthttp.RequestCtx{}
		fctx.Init(req, nil, nil)
		app.Handler()(fctx)
		return string(fctx.Response.Body()), string(fctx.Response.Header.Peek("X-Cache"))
	}

	body, status := do("a=1", "session=alice")
	require.Equal(t, cacheMiss, status)
	require.Equal(t, "page-for:alice", body)

	body, status = do("a=1", "session=alice")
	require.Equal(t, cacheHit, status,
		"the store key and the lookup key must agree across cookie collection")
	require.Equal(t, "page-for:alice", body)

	// Stability must not have come from keying everything alike: a different
	// session still parts company, and the merged single-line spelling of the
	// same cookies reaches the entry the two lines stored.
	body, status = do("a=1", "session=bob")
	require.Equal(t, cacheMiss, status)
	require.Equal(t, "page-for:bob", body)

	body, status = do("a=1; session=alice")
	require.Equal(t, cacheHit, status)
	require.Equal(t, "page-for:alice", body)
}

// Test_Authorization_SecondFieldLine asserts a credential is seen wherever it
// sits among the Authorization field lines. Peek returns only the first, so a
// scan of it alone read an empty first line as "no credential" — and the
// request would then have been keyed and served as anonymous, handing it
// whatever the anonymous entry held.
func Test_Authorization_SecondFieldLine(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("anonymous body") })

	do := func(lines ...string) string {
		parts := []string{"GET / HTTP/1.1\r\nHost: example.com\r\n"}
		for _, l := range lines {
			parts = append(parts, "Authorization: "+l+"\r\n")
		}
		raw := strings.Join(append(parts, "\r\n"), "")

		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

		fctx := &fasthttp.RequestCtx{}
		fctx.Init(req, nil, nil)
		app.Handler()(fctx)
		return string(fctx.Response.Header.Peek("X-Cache"))
	}

	// Prime an anonymous entry.
	require.Equal(t, cacheMiss, do())
	require.Equal(t, cacheHit, do())

	// The credential is on the second line. Since this response permits no
	// shared caching, carrying one means the entry is refused outright — which
	// only happens if the scan looked past the empty first line.
	require.Equal(t, cacheUnreachable, do("", "Bearer alice"),
		"a credential on a later field line must not be read as anonymous")
}

// Test_AuthorizationHash_CoversEveryFieldLine asserts the value hashed into the
// key is the whole comma-joined credential. Hashing only the first field line
// put two principals that share it in the same partition, so one read the
// other's response.
func Test_AuthorizationHash_CoversEveryFieldLine(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour}))
	app.Get("/", func(c fiber.Ctx) error {
		// Opt in to shared caching so an authorized response is stored at all,
		// which is what lets the key partition be observed.
		c.Set(fiber.HeaderCacheControl, "public")
		// Echo the last field line so the body shows which principal the
		// handler ran for. Indexed defensively: a single-line case added later
		// should fail the assertion, not panic here.
		lines := c.Request().Header.PeekAll(fiber.HeaderAuthorization)
		last := ""
		if len(lines) > 0 {
			last = string(lines[len(lines)-1])
		}
		return c.SendString("body for:" + last)
	})

	do := func(lines ...string) (string, string) {
		parts := []string{"GET / HTTP/1.1\r\nHost: example.com\r\n"}
		for _, l := range lines {
			parts = append(parts, "Authorization: "+l+"\r\n")
		}
		raw := strings.Join(append(parts, "\r\n"), "")

		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

		fctx := &fasthttp.RequestCtx{}
		fctx.Init(req, nil, nil)
		app.Handler()(fctx)
		return string(fctx.Response.Body()), string(fctx.Response.Header.Peek("X-Cache"))
	}

	// Both principals share the first field line and differ only on the second.
	body, status := do("Bearer shared", "Bearer alice")
	require.Equal(t, cacheMiss, status)
	require.Equal(t, "body for:Bearer alice", body)

	body, status = do("Bearer shared", "Bearer alice")
	require.Equal(t, cacheHit, status)
	require.Equal(t, "body for:Bearer alice", body)

	body, status = do("Bearer shared", "Bearer bob")
	require.Equal(t, cacheMiss, status,
		"principals differing only on a later field line must not share a partition")
	require.Equal(t, "body for:Bearer bob", body)
}

// Test_RequestPragma_SecondFieldLine asserts the RFC 9111 §5.4 no-cache
// fallback is honored on a repeated Pragma line, not only the first.
func Test_RequestPragma_SecondFieldLine(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("body") })

	do := func(lines ...string) string {
		parts := []string{"GET / HTTP/1.1\r\nHost: example.com\r\n"}
		for _, l := range lines {
			parts = append(parts, "Pragma: "+l+"\r\n")
		}
		raw := strings.Join(append(parts, "\r\n"), "")

		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

		fctx := &fasthttp.RequestCtx{}
		fctx.Init(req, nil, nil)
		app.Handler()(fctx)
		return string(fctx.Response.Header.Peek("X-Cache"))
	}

	require.Equal(t, cacheMiss, do())
	require.Equal(t, cacheHit, do())
	require.Equal(t, cacheMiss, do("token", "no-cache"),
		"no-cache on a later Pragma line must still force revalidation")
}

// Test_StoreResponseHeaders_DropsTrailer pins the hop-by-hop header's real
// name. The list said "Trailers", which is not a header, so the actual Trailer
// field was stored and replayed — announcing to every later client a trailer
// that only the connection it came from ever sent.
func Test_StoreResponseHeaders_DropsTrailer(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour, StoreResponseHeaders: true}))
	app.Get("/", func(c fiber.Ctx) error {
		c.Response().Header.Set("Trailer", "Expires")
		c.Response().Header.Set("X-Keep", "kept")
		return c.SendString("hi")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Empty(t, resp.Header.Values("Trailer"))
	// A stored header that is not hop-by-hop still comes back, so the assertion
	// above is about Trailer and not about header storage being off.
	require.Equal(t, "kept", resp.Header.Get("X-Keep"))
}

// Test_SetCookie2ResponseIsNotStored covers the obsolete RFC 2965 spelling.
// fasthttp does not route it into its cookie store, so h.Cookies() alone does
// not see it and the response would have been stored and shared.
func Test_SetCookie2ResponseIsNotStored(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour}))
	app.Get("/", func(c fiber.Ctx) error {
		n := calls.Add(1)
		c.Response().Header.Set("Set-Cookie2", fmt.Sprintf(`session="secret-%d"; Version="1"`, n))
		return c.SendString(fmt.Sprintf("page-for-client-%d", n))
	})

	for i := 1; i <= 2; i++ {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("page-for-client-%d", i), string(body))
	}
	require.Equal(t, int32(2), calls.Load())
}

// Test_joinKeyHeaderValues_Framing asserts the length prefixes keep the join
// injective. It is what stops two different field-line splits of the same bytes
// from digesting alike once a dimension is too long to store verbatim, and a
// plain concatenation would collide on the first pair below.
func Test_joinKeyHeaderValues_Framing(t *testing.T) {
	t.Parallel()

	join := func(values ...string) string {
		b := make([][]byte, len(values))
		for i, v := range values {
			b[i] = []byte(v)
		}
		return string(joinKeyHeaderValues(b))
	}

	require.NotEqual(t, join("ab"), join("a", "b"))
	require.NotEqual(t, join("a", "bc"), join("ab", "c"))
	require.NotEqual(t, join(""), join(), "absent and present-empty are different")
	require.NotEqual(t, join("", ""), join(""))
	// A value that spells its own framing cannot forge a boundary.
	require.NotEqual(t, join("\x01a\x01b"), join("a", "b"))
	// The encoding itself: a uvarint length before each value, which is what
	// makes the boundaries readable and so the join injective.
	require.Equal(t, "\x01a\x01b", join("a", "b"))
}

// Test_VaryContentType_KeyIsStableAcrossTheFormFold pins that a Vary'd
// Content-Type keys the same before and after the handler runs.
//
// The key is built twice per miss — once to look an entry up, once to store it
// — and the form accessors lowercase the request's own Content-Type in place in
// between. Reading whatever spelling arrived on one side and the folded value
// on the other stored each entry under a key no lookup would produce, so a
// route with "Vary: Content-Type" missed on every second request and stranded
// an entry each time.
func Test_VaryContentType_KeyIsStableAcrossTheFormFold(t *testing.T) {
	t.Parallel()

	for _, spelling := range []string{
		"Application/X-WWW-Form-Urlencoded",
		"application/x-www-form-urlencoded",
		"Multipart/Form-Data; BOUNDARY=AbC",
	} {
		t.Run(spelling, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32
			app := fiber.New()
			app.Use(New(Config{
				Expiration: time.Hour,
				Methods:    []string{fiber.MethodGet, fiber.MethodPost},
			}))
			app.Post("/", func(c fiber.Ctx) error {
				calls.Add(1)
				c.Response().Header.Set(fiber.HeaderVary, fiber.HeaderContentType)
				// Reading a form value is what folds the header mid-request.
				return c.SendString("v=" + c.FormValue("a"))
			})

			do := func() string {
				req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader("a=1"))
				req.Header.Set(fiber.HeaderContentType, spelling)
				resp, err := app.Test(req)
				require.NoError(t, err)
				return resp.Header.Get("X-Cache")
			}

			require.Equal(t, cacheMiss, do())
			require.Equal(t, cacheHit, do(), "the store key and the lookup key must agree across the fold")
			require.Equal(t, cacheHit, do())
			require.Equal(t, int32(1), calls.Load())
		})
	}
}

// Test_Vary_PartitionsWithoutHeaderNormalizing asserts a Vary'd dimension keys
// the same whether or not fasthttp normalizes header names.
//
// PeekAll compares stored keys byte for byte. With DisableHeaderNormalizing the
// store keeps the spelling the client sent, while the names reaching the key
// come lower-cased from parseVary — so every lookup missed and the dimension
// dropped out of the key silently. A request sending application/json was then
// served the entry stored for a form.
func Test_Vary_PartitionsWithoutHeaderNormalizing(t *testing.T) {
	t.Parallel()

	for _, disable := range []bool{false, true} {
		t.Run(fmt.Sprintf("DisableHeaderNormalizing=%v", disable), func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{DisableHeaderNormalizing: disable})
			app.Use(New(Config{
				Expiration: time.Hour,
				Methods:    []string{fiber.MethodGet, fiber.MethodPost},
			}))
			app.Post("/", func(c fiber.Ctx) error {
				c.Response().Header.Set(fiber.HeaderVary, fiber.HeaderContentType)
				return c.SendString("body-for:" + c.Get(fiber.HeaderContentType))
			})

			do := func(ct string) (string, string) {
				req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader("a=1"))
				req.Header.Set(fiber.HeaderContentType, ct)
				resp, err := app.Test(req)
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				return string(body), resp.Header.Get("X-Cache")
			}

			body, status := do("application/x-www-form-urlencoded")
			require.Equal(t, cacheMiss, status)
			require.Equal(t, "body-for:application/x-www-form-urlencoded", body)

			body, status = do("application/json")
			require.Equal(t, cacheMiss, status, "a different Content-Type must not hit the form entry")
			require.Equal(t, "body-for:application/json", body)

			body, status = do("application/json")
			require.Equal(t, cacheHit, status)
			require.Equal(t, "body-for:application/json", body)
		})
	}
}

// Test_KeyHeaders_PartitionWithoutHeaderNormalizing is the same property for a
// dimension named by KeyHeaders rather than by the response's Vary.
func Test_KeyHeaders_PartitionWithoutHeaderNormalizing(t *testing.T) {
	t.Parallel()

	for _, disable := range []bool{false, true} {
		t.Run(fmt.Sprintf("DisableHeaderNormalizing=%v", disable), func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{DisableHeaderNormalizing: disable})
			app.Use(New(Config{Expiration: time.Hour, KeyHeaders: []string{"X-Tenant"}}))
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("body-for:" + c.Get("X-Tenant"))
			})

			do := func(tenant string) (string, string) {
				req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
				req.Header.Set("X-Tenant", tenant)
				resp, err := app.Test(req)
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				return string(body), resp.Header.Get("X-Cache")
			}

			body, status := do("acme")
			require.Equal(t, cacheMiss, status)
			require.Equal(t, "body-for:acme", body)

			body, status = do("beta")
			require.Equal(t, cacheMiss, status, "a different tenant must not hit the first one's entry")
			require.Equal(t, "body-for:beta", body)

			body, status = do("acme")
			require.Equal(t, cacheHit, status)
			require.Equal(t, "body-for:acme", body)
		})
	}
}

// Test_CachedRedirectKeepsItsLocation asserts a cached 3xx is replayed with
// somewhere to go.
//
// 300 and 301 are cacheable statuses, but with the default
// StoreResponseHeaders:false only the body, status, Content-Type and
// Content-Encoding were kept — so the first client got "301 Location: /new" and
// every client after it a bare 301 it could not follow.
func Test_CachedRedirectKeepsItsLocation(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: time.Hour}))
	app.Get("/old", func(c fiber.Ctx) error {
		c.Set("X-Extra", "not-stored")
		return c.Redirect().Status(fiber.StatusMovedPermanently).To("/new")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/old", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusMovedPermanently, resp.StatusCode)
	require.Equal(t, "/new", resp.Header.Get("Location"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/old", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Equal(t, fiber.StatusMovedPermanently, resp.StatusCode)
	require.Equal(t, "/new", resp.Header.Get("Location"), "a cached redirect must keep its destination")
	// Only the destination is kept, not response headers at large — that is
	// still what StoreResponseHeaders is for.
	require.Empty(t, resp.Header.Get("X-Extra"))
}

// Test_Authorization_DetectedWithoutHeaderNormalizing asserts a credential is
// seen whatever case its field name arrived in.
//
// hasAuthorization and the auth hash read the header by its canonical name,
// which fasthttp answers only while it normalizes what it stored. With
// DisableHeaderNormalizing an "authorization:" sent in lower case matched
// nothing, so the request read as anonymous: its response was stored in the
// anonymous partition and replayed to the next client that asked, bearer token
// and all.
func Test_Authorization_DetectedWithoutHeaderNormalizing(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(New(Config{Expiration: time.Hour}))
	app.Get("/", func(c fiber.Ctx) error {
		// With normalizing off a handler reads the name as it arrived.
		if tok := c.Get("authorization"); tok != "" {
			return c.SendString("PRIVATE for " + tok)
		}
		return c.SendString("PUBLIC")
	})

	do := func(header string) (string, string) {
		raw := "GET / HTTP/1.1\r\nHost: example.com\r\n"
		if header != "" {
			raw += header + "\r\n"
		}
		raw += "\r\n"

		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		req.Header.DisableNormalizing()
		require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

		fctx := &fasthttp.RequestCtx{}
		fctx.Init(req, nil, nil)
		app.Handler()(fctx)
		return string(fctx.Response.Body()), string(fctx.Response.Header.Peek("X-Cache"))
	}

	body, status := do("authorization: Bearer alice-secret")
	require.Equal(t, cacheUnreachable, status, "an authorized response must not enter the shared store")
	require.Equal(t, "PRIVATE for Bearer alice-secret", body)

	body, status = do("")
	require.Equal(t, cacheMiss, status)
	require.Equal(t, "PUBLIC", body, "an anonymous request must not be served the authorized response")

	body, status = do("")
	require.Equal(t, cacheHit, status)
	require.Equal(t, "PUBLIC", body)
}

// Test_RequestDirectives_HonoredWithoutHeaderNormalizing covers the same
// lookup for the request directives, which are read the same way: a lower-case
// "cache-control: no-store" was ignored outright.
func Test_RequestDirectives_HonoredWithoutHeaderNormalizing(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(New(Config{Expiration: time.Hour}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("body") })

	do := func(header string) string {
		raw := "GET / HTTP/1.1\r\nHost: example.com\r\n"
		if header != "" {
			raw += header + "\r\n"
		}
		raw += "\r\n"

		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		req.Header.DisableNormalizing()
		require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

		fctx := &fasthttp.RequestCtx{}
		fctx.Init(req, nil, nil)
		app.Handler()(fctx)
		return string(fctx.Response.Header.Peek("X-Cache"))
	}

	require.Equal(t, cacheMiss, do(""))
	require.Equal(t, cacheHit, do(""))
	require.Empty(t, do("cache-control: no-store"), "no-store must be honored whatever case it arrived in")
	require.Equal(t, cacheMiss, do("cache-control: no-cache"), "no-cache must force revalidation")
}

// Test_Authorization_MixedCaseFieldLines closes the other half of the
// case-insensitive lookup.
//
// Falling back to a case-insensitive walk only when the byte-for-byte lookup
// came back empty was not enough: with DisableHeaderNormalizing a request can
// send both spellings, and an empty canonical "Authorization:" line satisfies
// the fast path, so the credential on the lowercase line beside it was never
// looked at. The request read as anonymous and its private response was stored
// in the anonymous partition.
func Test_Authorization_MixedCaseFieldLines(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(New(Config{Expiration: time.Hour}))
	app.Get("/", func(c fiber.Ctx) error {
		tok := c.Get("Authorization")
		if tok == "" {
			tok = c.Get("authorization")
		}
		if tok == "" {
			return c.SendString("PUBLIC")
		}
		return c.SendString("PRIVATE for " + tok)
	})

	do := func(lines ...string) (string, string) {
		parts := []string{"GET / HTTP/1.1\r\nHost: example.com\r\n"}
		for _, l := range lines {
			parts = append(parts, l+"\r\n")
		}
		raw := strings.Join(append(parts, "\r\n"), "")

		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		req.Header.DisableNormalizing()
		require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

		fctx := &fasthttp.RequestCtx{}
		fctx.Init(req, nil, nil)
		app.Handler()(fctx)
		return string(fctx.Response.Body()), string(fctx.Response.Header.Peek("X-Cache"))
	}

	// The canonical line is present but empty, so the byte-for-byte lookup
	// succeeds and would have stopped there.
	body, status := do("Authorization:", "authorization: Bearer alice")
	require.Equal(t, cacheUnreachable, status, "the credential on the second spelling must still be seen")
	require.Equal(t, "PRIVATE for Bearer alice", body)

	body, status = do()
	require.Equal(t, cacheMiss, status)
	require.Equal(t, "PUBLIC", body, "an anonymous request must not be served the authorized response")
}

// Test_Cache_NonCanonicalResponseNamesAreNotDuplicated checks that the fields
// this middleware writes from an entry are not emitted a second time beside the
// spelling the handler chose for them.
//
// Under DisableHeaderNormalizing the stored key is whatever the handler wrote,
// so a byte-for-byte lookup found neither the value to honor nor the line to
// replace. Every field then went out twice, and the second Cache-Control was
// the "public" this middleware synthesizes on a hit when it believes the
// response carries none — telling a downstream shared cache it may store and
// share a response whose origin never said so.
func Test_Cache_NonCanonicalResponseNamesAreNotDuplicated(t *testing.T) {
	t.Parallel()

	for _, normalize := range []bool{true, false} {
		t.Run(fmt.Sprintf("normalize=%v", normalize), func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{DisableHeaderNormalizing: !normalize})
			app.Use(New(Config{StoreResponseHeaders: true, Expiration: time.Minute}))
			app.Get("/", func(c fiber.Ctx) error {
				// Lower case throughout, which is legal on the wire and is what
				// DisableHeaderNormalizing preserves.
				c.Response().Header.Set("cache-control", "max-age=60")
				c.Response().Header.Set("age", "0")
				c.Response().Header.Set("etag", `"v1"`)
				c.Response().Header.Set("expires", "Wed, 21 Oct 2099 07:28:00 GMT")
				c.Response().Header.Set("date", "Wed, 21 Oct 2020 07:28:00 GMT")
				return c.SendString("body")
			})

			single := []string{
				fiber.HeaderCacheControl,
				fiber.HeaderAge,
				fiber.HeaderETag,
				fiber.HeaderExpires,
				fiber.HeaderDate,
			}

			for _, want := range []string{cacheMiss, cacheHit} {
				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, want, resp.Header.Get("X-Cache"))

				for _, name := range single {
					require.Len(t, resp.Header.Values(name), 1,
						"%s must be sent on exactly one field line (%s): %v",
						name, want, resp.Header.Values(name))
				}

				// The origin said max-age, never public. Synthesizing one is
				// what lifts RFC 9111 Section 3.5 for a downstream shared cache.
				require.NotContains(t, resp.Header.Get(fiber.HeaderCacheControl), "public",
					"the handler's Cache-Control must be the one that goes out (%s)", want)
			}
		})
	}
}

// Test_Cache_NonCanonicalResponseNamesAreHonored checks that the values behind
// those spellings still decide what they are supposed to decide, whichever
// spelling the handler used.
func Test_Cache_NonCanonicalResponseNamesAreHonored(t *testing.T) {
	t.Parallel()

	// Each case asserts the same outcome under both header-name settings: the
	// spelling a handler picks is not allowed to change what the cache does.
	tests := []struct {
		handler  fiber.Handler
		assert   func(t *testing.T, resp *http.Response)
		name     string
		statuses []string
	}{
		{
			name: "a cached redirect keeps its destination",
			handler: func(c fiber.Ctx) error {
				c.Status(fiber.StatusMovedPermanently)
				c.Response().Header.Set("location", "/new")
				return nil
			},
			statuses: []string{cacheMiss, cacheHit},
			assert: func(t *testing.T, resp *http.Response) {
				t.Helper()
				require.Equal(t, fiber.StatusMovedPermanently, resp.StatusCode)
				require.Equal(t, "/new", resp.Header.Get(fiber.HeaderLocation))
			},
		},
		{
			// Stale on arrival, so there is nothing worth storing. Missing the
			// header meant storing it for the configured minute instead.
			name: "an Expires in the past is not stored",
			handler: func(c fiber.Ctx) error {
				c.Response().Header.Set("expires", "Wed, 21 Oct 2020 07:28:00 GMT")
				return c.SendString("body")
			},
			statuses: []string{cacheUnreachable, cacheUnreachable},
			assert: func(t *testing.T, resp *http.Response) {
				t.Helper()
				require.Equal(t, "Wed, 21 Oct 2020 07:28:00 GMT", resp.Header.Get(fiber.HeaderExpires))
			},
		},
		{
			// no-store is a refusal to keep the response at all.
			name: "no-store is obeyed",
			handler: func(c fiber.Ctx) error {
				c.Response().Header.Set("cache-control", "no-store")
				return c.SendString("body")
			},
			statuses: []string{cacheUnreachable, cacheUnreachable},
			assert:   func(_ *testing.T, _ *http.Response) {},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, normalize := range []bool{true, false} {
				app := fiber.New(fiber.Config{DisableHeaderNormalizing: !normalize})
				app.Use(New(Config{StoreResponseHeaders: true, Expiration: time.Minute}))
				app.Get("/", tc.handler)

				for _, want := range tc.statuses {
					resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
					require.NoError(t, err)
					require.Equal(t, want, resp.Header.Get("X-Cache"), "normalize=%v", normalize)
					tc.assert(t, resp)
				}
			}
		})
	}
}

// Test_Cache_OuterMiddlewareHeaderNamesAreNotDuplicated covers the other side
// of the same lookup: a middleware that runs before this one has already
// written to the response by the time a hit is served, so the fields replayed
// from the entry are written over lines that are there in whatever spelling
// that middleware chose.
//
// Set matches the key byte for byte, so under DisableHeaderNormalizing it
// appended rather than replaced and the response went out with both — the
// stored value and a stale copy of the same field, with nothing to say which
// one a recipient should read.
func Test_Cache_OuterMiddlewareHeaderNamesAreNotDuplicated(t *testing.T) {
	t.Parallel()

	for _, normalize := range []bool{true, false} {
		t.Run(fmt.Sprintf("normalize=%v", normalize), func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{DisableHeaderNormalizing: !normalize})
			app.Use(func(c fiber.Ctx) error {
				c.Response().Header.Set("cache-control", "max-age=5")
				c.Response().Header.Set("etag", `"outer"`)
				c.Response().Header.Set("expires", "Wed, 21 Oct 2099 07:28:00 GMT")
				return c.Next()
			})
			app.Use(New(Config{StoreResponseHeaders: true, Expiration: time.Minute}))
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("body")
			})

			for _, want := range []string{cacheMiss, cacheHit} {
				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, want, resp.Header.Get("X-Cache"))

				for _, name := range []string{fiber.HeaderCacheControl, fiber.HeaderETag, fiber.HeaderExpires} {
					require.Len(t, resp.Header.Values(name), 1,
						"%s must be sent on exactly one field line (%s): %v",
						name, want, resp.Header.Values(name))
				}
			}
		})
	}
}

// Test_Cache_SetCookie2IsSeenWhateverItsCase checks the second cookie header
// against the same standard as the first.
//
// Set-Cookie has a store of its own in fasthttp, which every spelling is routed
// into, so the guard found it either way. Set-Cookie2 — the obsolete RFC 2965
// spelling, which hands the client a session just the same — is an ordinary
// field, so a byte-for-byte lookup found only the canonical one. A handler
// writing it in lower case had a personalized response stored and replayed to
// every later client whose request matched the key.
func Test_Cache_SetCookie2IsSeenWhateverItsCase(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"Set-Cookie2", "set-cookie2"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
			app.Use(New(Config{Expiration: time.Minute}))

			var n atomic.Int64
			app.Get("/", func(c fiber.Ctx) error {
				id := n.Add(1)
				c.Response().Header.Set(name, fmt.Sprintf("session=client-%d; Version=1", id))
				return c.SendString(fmt.Sprintf("private to client-%d", id))
			})

			for range 2 {
				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"),
					"a response naming a cookie is never stored")
			}
			require.Equal(t, int64(2), n.Load(), "each client must reach the handler")
		})
	}
}

// Test_Cache_QuotedDirectiveDoesNotAuthorizeSharing covers a Cache-Control
// whose extension value is a quoted-string containing commas.
//
// The directive scan split on every comma, so the tokens inside the quotes were
// read as directives of their own. A response saying nothing about sharing was
// therefore taken to have said "public" — enough to pass the gate that lets a
// cookie-setting response be stored, after which its body went to later clients
// with only the cookie removed. RFC 9111 §5.2 makes the value a quoted-string;
// a comma inside one does not end the directive.
func Test_Cache_QuotedDirectiveDoesNotAuthorizeSharing(t *testing.T) {
	t.Parallel()

	for _, cc := range []string{
		`ext="a, public, b"`,
		`ext="a, s-maxage=99, b"`,
		// The escaped quote must not end the value early either, or the scan
		// resumes inside the string and finds the token after it.
		`ext="a\", public, b"`,
	} {
		t.Run(cc, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use(New(Config{Expiration: time.Minute}))

			var n atomic.Int64
			app.Get("/", func(c fiber.Ctx) error {
				id := n.Add(1)
				c.Response().Header.Set(fiber.HeaderCacheControl, cc)
				c.Response().Header.Set(fiber.HeaderSetCookie, fmt.Sprintf("session=client-%d", id))
				return c.SendString(fmt.Sprintf("private to client-%d", id))
			})

			for range 2 {
				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"),
					"a token inside a quoted extension value does not authorize sharing")
			}
			require.Equal(t, int64(2), n.Load(), "each client must reach the handler")
		})
	}
}

// Test_Cache_QuotedDirectiveStillReadsRealOnes is the control for the test
// above: a quoted value must not hide the directives written beside it.
func Test_Cache_QuotedDirectiveStillReadsRealOnes(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: time.Minute}))

	var n atomic.Int64
	app.Get("/", func(c fiber.Ctx) error {
		id := n.Add(1)
		c.Response().Header.Set(fiber.HeaderCacheControl, `ext="a, b", public`)
		c.Response().Header.Set(fiber.HeaderSetCookie, fmt.Sprintf("session=client-%d", id))
		return c.SendString(fmt.Sprintf("shared-%d", id))
	})

	first, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, first.Header.Get("X-Cache"))

	second, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, second.Header.Get("X-Cache"),
		"public written outside the quotes still authorizes sharing")
	require.Equal(t, int64(1), n.Load())
}

// Test_Cache_EveryStaleSpellingIsReplaced covers a response that already
// carries two spellings of a field this middleware writes.
//
// Writing through the first one found and stopping there left the other in
// place, so the value written here went out beside a stale copy of the same
// field — the pair the write exists to prevent, reached by a response that had
// already been touched twice before the cache saw it.
func Test_Cache_EveryStaleSpellingIsReplaced(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	// Two middlewares ahead of the cache, each choosing its own spelling.
	// Three spellings of each field: the canonical one SetBytesV replaces on its
	// own, plus two more that only the scan can reach. One is what a loop that
	// stops at the first match leaves behind.
	app.Use(func(c fiber.Ctx) error {
		c.Response().Header.Set("cache-control", "max-age=5")
		c.Response().Header.Set("etag", `"lower"`)
		return c.Next()
	})
	app.Use(func(c fiber.Ctx) error {
		c.Response().Header.Add("CACHE-CONTROL", "max-age=7")
		c.Response().Header.Add("ETAG", `"upper"`)
		return c.Next()
	})
	app.Use(func(c fiber.Ctx) error {
		c.Response().Header.Add(fiber.HeaderCacheControl, "max-age=6")
		c.Response().Header.Add(fiber.HeaderETag, `"canonical"`)
		return c.Next()
	})
	app.Use(New(Config{StoreResponseHeaders: true, Expiration: time.Minute}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("body") })

	// The miss carries both lines because both middlewares wrote one and the
	// cache writes neither on that path — their business, not this one's.
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	// The hit is where this middleware writes them from the entry, and there it
	// owns the field: one line, whatever spellings it had to replace.
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))

	for _, name := range []string{fiber.HeaderCacheControl, fiber.HeaderETag} {
		require.Len(t, resp.Header.Values(name), 1,
			"%s must be sent on exactly one field line: %v", name, resp.Header.Values(name))
	}
}

// Test_Cache_CookieKeyedEntriesStayPerSession pins that a cache keyed on Cookie
// keeps one entry per session whatever the header-name setting is.
//
// The Cookie dimension is read after forcing fasthttp's cookie collection,
// which moves the field lines out of the general header store — so a
// case-insensitive walk of that store finds nothing there. It finds them all
// the same, because the iterator yields the collected cookies separately under
// the canonical name. Were that not so, every request would hash an empty
// Cookie dimension and distinct sessions would collapse onto one key, which is
// one client's private response served to another.
func Test_Cache_CookieKeyedEntriesStayPerSession(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"KeyHeaders", "Vary"} {
		for _, normalize := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/normalize=%v", mode, normalize), func(t *testing.T) {
				t.Parallel()

				cfg := Config{Expiration: time.Minute}
				if mode == "KeyHeaders" {
					cfg.KeyHeaders = []string{fiber.HeaderCookie}
				}

				app := fiber.New(fiber.Config{DisableHeaderNormalizing: !normalize})
				app.Use(New(cfg))
				app.Get("/", func(c fiber.Ctx) error {
					if mode == "Vary" {
						c.Set(fiber.HeaderVary, fiber.HeaderCookie)
					}
					return c.SendString("private to " + c.Cookies("sess"))
				})
				h := app.Handler()

				do := func(cookieHeader, session string) (string, string) {
					req := fasthttp.AcquireRequest()
					defer fasthttp.ReleaseRequest(req)
					if !normalize {
						req.Header.DisableNormalizing()
					}
					req.Header.SetMethod(fiber.MethodGet)
					req.SetRequestURI("/")
					req.Header.SetHost("example.com")
					req.Header.Set(cookieHeader, "sess="+session)

					ctx := &fasthttp.RequestCtx{}
					ctx.Init(req, nil, nil)
					h(ctx)
					return string(ctx.Response.Body()), string(ctx.Response.Header.Peek("X-Cache"))
				}

				// alice populates an entry and then reads her own back, so a
				// miss below means a different key rather than an uncacheable
				// response.
				_, _ = do(fiber.HeaderCookie, "alice")
				body, status := do(fiber.HeaderCookie, "alice")
				require.Equal(t, cacheHit, status)
				require.Equal(t, "private to alice", body)

				body, status = do(fiber.HeaderCookie, "bob")
				require.Equal(t, cacheMiss, status, "bob must not land on alice's entry")
				require.Equal(t, "private to bob", body)

				// And the same when the name arrives in lower case, which is
				// what HTTP/2 and HTTP/3 put on the wire.
				body, status = do("cookie", "carol")
				require.Equal(t, cacheMiss, status, "carol must not land on an earlier session's entry")
				require.Equal(t, "private to carol", body)

				body, status = do("cookie", "carol")
				require.Equal(t, cacheHit, status)
				require.Equal(t, "private to carol", body)
			})
		}
	}
}

// Test_Cache_LongHeaderNameIsNotIgnored covers the length guard on the
// ignore-list lookup. No name this middleware refuses to carry between
// requests is anywhere near that long, so a longer one is by definition not
// one of them and must be stored and replayed like any other.
func Test_Cache_LongHeaderNameIsNotIgnored(t *testing.T) {
	t.Parallel()

	long := "X-" + strings.Repeat("a", maxIgnoredHeaderLen)
	require.Greater(t, len(long), maxIgnoredHeaderLen)

	app := fiber.New()
	app.Use(New(Config{StoreResponseHeaders: true, Expiration: time.Minute}))
	app.Get("/", func(c fiber.Ctx) error {
		c.Set(long, "kept")
		return c.SendString("body")
	})

	for _, want := range []string{cacheMiss, cacheHit} {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, want, resp.Header.Get("X-Cache"))
		require.Equal(t, "kept", resp.Header.Get(long), "a name too long to be on the list is carried (%s)", want)
	}
}

// Test_Cache_ReplayClearsEveryStoredSpelling covers the clear that runs before
// the stored field lines are appended back.
//
// DelBytes matches the stored key byte for byte, so under
// DisableHeaderNormalizing a line this response already carries under another
// spelling survived it and the append landed beside it — two lines of one
// field, which is the pair the clear exists to prevent. The fields the
// middleware writes from the entry's own slots go through setFieldLine and were
// already covered; these are the ones replayed from StoreResponseHeaders.
func Test_Cache_ReplayClearsEveryStoredSpelling(t *testing.T) {
	t.Parallel()

	// The spelling this middleware sees on the way out varies per request, so
	// the entry stores one and the response being replayed onto carries
	// another.
	var n atomic.Int64
	spellings := []string{"X-Tenant", "x-tenant", "X-TENANT"}

	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(func(c fiber.Ctx) error {
		c.Response().Header.Set(spellings[n.Add(1)%int64(len(spellings))], "from-middleware")
		return c.Next()
	})
	app.Use(New(Config{StoreResponseHeaders: true, Expiration: time.Minute}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("body") })

	for _, want := range []string{cacheMiss, cacheHit, cacheHit} {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, want, resp.Header.Get("X-Cache"))
		require.Len(t, resp.Header.Values("X-Tenant"), 1,
			"one field line whatever spelling each hop chose (%s): %v", want, resp.Header.Values("X-Tenant"))
	}
}

// Test_Cache_AuthorizationFieldLinesFramed covers the framing of the
// Authorization value that goes into the key.
//
// Joining field lines with a comma renders "Bearer a,Bearer b" the same whether
// it arrived as one line or as two, and this middleware treats a second line as
// a second credential, so the two layouts shared a partition: with a response
// opting into shared caching, the single-line request was served the two-line
// request's body.
func Test_Cache_AuthorizationFieldLinesFramed(t *testing.T) {
	t.Parallel()

	var handlerRuns atomic.Int64
	app := fiber.New()
	app.Use(New(Config{Expiration: time.Minute}))
	app.Get("/", func(c fiber.Ctx) error {
		handlerRuns.Add(1)
		c.Set(fiber.HeaderCacheControl, "public, max-age=60")
		return c.SendString(c.Get(fiber.HeaderAuthorization))
	})

	twoLines := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	twoLines.Header.Add(fiber.HeaderAuthorization, "Bearer shared")
	twoLines.Header.Add(fiber.HeaderAuthorization, "Bearer alice")
	resp, err := app.Test(twoLines)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	oneLine := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	oneLine.Header.Set(fiber.HeaderAuthorization, "Bearer shared,Bearer alice")
	resp, err = app.Test(oneLine)
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"),
		"a lone comma-joined credential must not land on the two-line entry")
	require.Equal(t, int64(2), handlerRuns.Load(), "each layout is its own principal")
}

// Test_Cache_LegacyStoredSetCookieIsUnsafe covers an entry left by a version
// that stored Set-Cookie among the response headers.
//
// The body beside that cookie is personalized for whoever caused the miss, so
// omitting only the cookie on replay still hands that body to everyone matching
// the key. The entry is dropped instead.
func Test_Cache_LegacyStoredSetCookieIsUnsafe(t *testing.T) {
	t.Parallel()

	store := memory.New()
	var handlerRuns atomic.Int64
	app := fiber.New()
	app.Use(New(Config{
		Storage:              store,
		StoreResponseHeaders: true,
		Expiration:           time.Minute,
		KeyGenerator:         func(_ fiber.Ctx) string { return "k" },
	}))
	app.Get("/", func(c fiber.Ctx) error {
		handlerRuns.Add(1)
		return c.SendString("alice-only")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	// Rewrite the stored entry the way the older version left it.
	key := cacheKeyVersion + "|GET|k"
	m := newManager(store, false)
	e, err := m.get(context.Background(), key)
	require.NoError(t, err)
	require.NotNil(t, e)
	e.headers = append(e.headers, cachedHeader{
		key:   []byte(fiber.HeaderSetCookie),
		value: []byte("session=alice; Path=/"),
	})
	require.NoError(t, m.set(context.Background(), key, e, time.Minute))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"),
		"an entry carrying a cookie is not replayed")
	require.Equal(t, int64(2), handlerRuns.Load(), "the handler answers instead")
	require.Empty(t, resp.Header.Get(fiber.HeaderSetCookie), "and the stored cookie goes nowhere")
}

// Test_Cache_ProxiedResponseCasingIsRead covers a response whose field names the
// app config does not describe.
//
// A proxy hands c.Response() to an outbound fasthttp.Client, and fasthttp stamps
// that client's DisableHeaderNamesNormalizing onto the response it parses into.
// A default-normalizing app can therefore hold a lower-case "cache-control:
// private" — which, read byte-exact, cached one client's body and replayed it to
// the next under a synthesized "public, max-age".
func Test_Cache_ProxiedResponseCasingIsRead(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		directive string
	}{
		{"private", "private"},
		{"no-store", "no-store"},
		{"no-cache", "no-cache"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var handlerRuns atomic.Int64
			app := fiber.New()
			app.Use(New(Config{StoreResponseHeaders: true, Expiration: time.Minute}))
			app.Get("/", func(c fiber.Ctx) error {
				handlerRuns.Add(1)
				// What the outbound client left behind, without the round trip.
				c.Response().Header.DisableNormalizing()
				c.Response().Header.Set("cache-control", tc.directive)
				return c.SendString("secret-for-" + c.Get("X-User"))
			})

			for _, user := range []string{"alice", "bob"} {
				req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
				req.Header.Set("X-User", user)
				resp, err := app.Test(req)
				require.NoError(t, err)

				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Equal(t, "secret-for-"+user, string(body), "each client gets its own body")
				require.Equal(t, tc.directive, resp.Header.Get(fiber.HeaderCacheControl),
					"the restriction the origin sent survives")
			}
			require.Equal(t, int64(2), handlerRuns.Load(), "nothing was cached")
		})
	}
}

// Test_Cache_LegacyAnonymousEntryIsNotRead covers an entry a previous version
// stored. It detected Authorization byte-exactly, so under DisableHeaderNormalizing
// a request bearing a lower-case "authorization" was taken for anonymous and its
// response cached under the anonymous key. Only lookups that carry the header
// moved to a partition of their own, so on a store that survived the upgrade an
// anonymous request would still find that entry and be served the authenticated
// body. The key namespace is versioned so no such entry is ever read.
func Test_Cache_LegacyAnonymousEntryIsNotRead(t *testing.T) {
	t.Parallel()

	storage := memory.New()
	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(New(Config{
		Storage:    storage,
		Expiration: time.Minute,
		KeyGenerator: func(c fiber.Ctx) string {
			// A custom generator, as an application preserving the old key format
			// across the upgrade would have.
			return c.Path()
		},
	}))
	app.Get("/me", func(c fiber.Ctx) error { return c.SendString("handler ran") })

	// Seed what the previous version wrote for alice: the anonymous key, holding
	// her authenticated body.
	legacy := &item{
		body:      []byte("alice's private page"),
		ctype:     []byte(fiber.MIMETextPlainCharsetUTF8),
		status:    fiber.StatusOK,
		exp:       uint64(time.Now().Add(time.Minute).Unix()),
		ttl:       uint64(time.Minute / time.Second),
		headers:   nil,
		cencoding: nil,
	}
	raw, err := legacy.MarshalMsg(nil)
	require.NoError(t, err)
	require.NoError(t, storage.Set("GET|/me", raw, time.Minute))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/me", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "handler ran", string(body), "the legacy anonymous entry must not be served")
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
}
