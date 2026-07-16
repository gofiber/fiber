package cache

import (
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
