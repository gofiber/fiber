package cache

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// tagIndex unit tests
// ---------------------------------------------------------------------------

func Test_tagIndex_AddAndHas(t *testing.T) {
	t.Parallel()

	ti := newTagIndex()

	require.False(t, ti.has("k1"))
	ti.add("k1", []string{"a", "b"})
	require.True(t, ti.has("k1"))

	// Adding empty tags is a no-op
	ti.add("k2", nil)
	require.False(t, ti.has("k2"))
}

func Test_tagIndex_Remove(t *testing.T) {
	t.Parallel()

	ti := newTagIndex()
	ti.add("k1", []string{"a", "b"})
	ti.add("k2", []string{"b", "c"})

	ti.remove("k1")
	require.False(t, ti.has("k1"))
	// "b" still mapped to k2
	require.True(t, ti.has("k2"))

	// Removing unknown key is safe
	ti.remove("nonexistent")
}

func Test_tagIndex_Invalidate(t *testing.T) {
	t.Parallel()

	ti := newTagIndex()
	ti.add("k1", []string{"a", "b"})
	ti.add("k2", []string{"b", "c"})
	ti.add("k3", []string{"c", "d"})

	// Invalidate tag "b" → should return k1 and k2
	keys := ti.invalidate([]string{"b"})
	require.ElementsMatch(t, []string{"k1", "k2"}, keys)

	// k1 still has tag "a" (only "b" was invalidated), so it remains in reverse index
	require.True(t, ti.has("k1"))
	// k2 still has tag "c"
	require.True(t, ti.has("k2"))

	// Invalidate "a" → removes k1's last remaining tag
	keys = ti.invalidate([]string{"a"})
	require.ElementsMatch(t, []string{"k1"}, keys)
	require.False(t, ti.has("k1"))

	// Invalidate tags "c" and "d" → should return k2 and k3
	keys = ti.invalidate([]string{"c", "d"})
	require.ElementsMatch(t, []string{"k2", "k3"}, keys)
	require.False(t, ti.has("k2"))
	require.False(t, ti.has("k3"))

	// Invalidating already-gone tags returns nothing
	keys = ti.invalidate([]string{"a", "b", "c"})
	require.Empty(t, keys)
}

func Test_tagIndex_InvalidateMultipleTags(t *testing.T) {
	t.Parallel()

	ti := newTagIndex()
	// k1 shares both tags being invalidated – should appear only once in result
	ti.add("k1", []string{"x", "y"})
	ti.add("k2", []string{"x"})
	ti.add("k3", []string{"y"})

	keys := ti.invalidate([]string{"x", "y"})
	require.ElementsMatch(t, []string{"k1", "k2", "k3"}, keys)
}

// ---------------------------------------------------------------------------
// rejectMatcher unit tests
// ---------------------------------------------------------------------------

func Test_rejectMatcher_Exact(t *testing.T) {
	t.Parallel()
	m := newRejectMatcher([]string{"internal", "secret"})

	require.True(t, m.matches("internal"))
	require.True(t, m.matches("secret"))
	require.False(t, m.matches("public"))
	require.False(t, m.matches("internal2")) // not a prefix match
}

func Test_rejectMatcher_Prefix(t *testing.T) {
	t.Parallel()
	m := newRejectMatcher([]string{"user:*"})

	require.True(t, m.matches("user:"))
	require.True(t, m.matches("user:123"))
	require.True(t, m.matches("user:abc:def"))
	require.False(t, m.matches("User:1")) // case-sensitive
	require.False(t, m.matches("admin:1"))
}

func Test_rejectMatcher_Suffix(t *testing.T) {
	t.Parallel()
	m := newRejectMatcher([]string{"*:secret"})

	require.True(t, m.matches("data:secret"))
	require.True(t, m.matches(":secret"))
	require.False(t, m.matches("data:public"))
}

func Test_rejectMatcher_General(t *testing.T) {
	t.Parallel()
	m := newRejectMatcher([]string{"a*b*c"})

	require.True(t, m.matches("abc"))
	require.True(t, m.matches("aXbYc"))
	require.True(t, m.matches("a123b456c"))
	require.False(t, m.matches("abX"))
}

func Test_rejectMatcher_MatchesAny(t *testing.T) {
	t.Parallel()
	m := newRejectMatcher([]string{"bad", "evil:*"})

	require.True(t, m.matchesAny([]string{"good", "bad"}))
	require.True(t, m.matchesAny([]string{"evil:tag"}))
	require.False(t, m.matchesAny([]string{"good", "fine"}))
	require.False(t, m.matchesAny(nil))
}

func Test_globMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pattern string
		s       string
		want    bool
	}{
		{"*", "", true},
		{"*", "anything", true},
		{"abc", "abc", true},
		{"abc", "ab", false},
		{"a*c", "ac", true},
		{"a*c", "abc", true},
		{"a*c", "aXYZc", true},
		{"a*c", "aXYZ", false},
		{"**", "hello", true},  // consecutive stars collapse
		{"a**b", "aXb", true},  // consecutive stars collapse
		{"", "", true},
		{"", "x", false},
		{"*a*", "bab", true},
		{"*a*", "bbb", false},
	}

	for _, tc := range tests {
		got := globMatch(tc.pattern, tc.s)
		require.Equal(t, tc.want, got, "globMatch(%q, %q)", tc.pattern, tc.s)
	}
}

// ---------------------------------------------------------------------------
// Integration: tag association, invalidation, and rejection via middleware
// ---------------------------------------------------------------------------

func Test_Cache_TagAssociationAndInvalidation(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 10 * time.Second,
		Storage:    memory.New(),
		Tags: func(c fiber.Ctx) []string {
			// Use the last path segment as the tag
			p := c.Path()
			if len(p) > 1 {
				return []string{p[1:]} // strip leading /
			}
			return []string{"root"}
		},
	}))

	// Two distinct paths → distinct cache keys and tags
	app.Get("/alpha", func(c fiber.Ctx) error {
		return c.SendString("alpha")
	})
	app.Get("/beta", func(c fiber.Ctx) error {
		return c.SendString("beta")
	})
	// POST routes so they are not cached themselves (only GET/HEAD are cached by default)
	app.Post("/invalidate-alpha", func(c fiber.Ctx) error {
		return InvalidateTags(c, "alpha")
	})

	// Seed two tagged entries
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/alpha", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/beta", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	// Both should be cached hits now
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/alpha", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/beta", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))

	// Invalidate only "alpha" via POST
	resp, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/invalidate-alpha", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// alpha is gone → miss; beta untouched → hit
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/alpha", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/beta", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
}

func Test_Cache_ResponseTagsMergedWithRequestTags(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 10 * time.Second,
		Tags: func(c fiber.Ctx) []string {
			return []string{"req"}
		},
		ResponseTags: func(_ fiber.Ctx, body []byte) []string {
			return []string{"resp:" + string(body)}
		},
	}))

	app.Get("/data", func(c fiber.Ctx) error {
		return c.SendString("hello")
	})
	// POST so it is not cached itself
	app.Post("/invalidate-resp-hello", func(c fiber.Ctx) error {
		return InvalidateTags(c, "resp:hello")
	})

	// Seed
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/data", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	// Cached
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/data", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))

	// Invalidating the response-derived tag evicts the entry
	resp, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/invalidate-resp-hello", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/data", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
}

func Test_Cache_RejectTagsPreventsStorage(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 10 * time.Second,
		Tags: func(c fiber.Ctx) []string {
			return []string{fiber.Query(c, "tag", "ok")}
		},
		RejectTags: []string{"internal", "secret:*"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("body")
	})

	// Exact match rejection
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?tag=internal", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))

	// Prefix wildcard rejection
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/?tag=secret:key1", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))

	// Non-rejected tag → normal caching
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/?tag=public", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/?tag=public", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
}

func Test_Cache_InvalidateTagsWithoutMiddlewareReturnsError(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	// No cache middleware registered
	app.Get("/", func(c fiber.Ctx) error {
		err := InvalidateTags(c, "anything")
		if err != nil {
			return c.SendString(err.Error())
		}
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	// The error message should indicate middleware is required
	body := make([]byte, resp.ContentLength)
	_, _ = resp.Body.Read(body)
	require.Contains(t, string(body), "requires the cache middleware")
}

// ---------------------------------------------------------------------------
// Conditional requests: ETag (If-None-Match) and Last-Modified (If-Modified-Since)
// ---------------------------------------------------------------------------

func Test_Cache_ETagAutoGeneration(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 10 * time.Second,
		EnableETag: true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("etag-body")
	})

	// First request → miss, ETag should be generated
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	etag := resp.Header.Get(fiber.HeaderETag)
	require.NotEmpty(t, etag, "ETag must be auto-generated on miss")
	// Strong ETag: quoted hex
	require.True(t, len(etag) > 2 && etag[0] == '"' && etag[len(etag)-1] == '"')

	// Second request → hit, same ETag
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Equal(t, etag, resp.Header.Get(fiber.HeaderETag))
}

func Test_Cache_ETagConditional304(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 10 * time.Second,
		EnableETag: true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("conditional-body")
	})

	// Seed the cache
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	etag := resp.Header.Get(fiber.HeaderETag)
	require.NotEmpty(t, etag)

	// Send If-None-Match with matching ETag → 304
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderIfNoneMatch, etag)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotModified, resp.StatusCode)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))

	// Non-matching ETag → 200 hit with body
	req = httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderIfNoneMatch, `"0000000000000000000000000000000000000000000000000000000000000000"`)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
}

func Test_Cache_ETagStar304(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 10 * time.Second,
		EnableETag: true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("star-body")
	})

	// Seed
	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	// If-None-Match: * matches any stored ETag
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderIfNoneMatch, "*")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotModified, resp.StatusCode)
}

func Test_Cache_LastModifiedAutoGeneration(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration:         10 * time.Second,
		EnableETag:         false,
		EnableLastModified: true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("lm-body")
	})

	// Miss → Last-Modified should be set
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	lm := resp.Header.Get(fiber.HeaderLastModified)
	require.NotEmpty(t, lm, "Last-Modified must be auto-generated on miss")

	// Hit → same Last-Modified
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Equal(t, lm, resp.Header.Get(fiber.HeaderLastModified))
}

func Test_Cache_LastModifiedConditional304(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration:         10 * time.Second,
		EnableETag:         false,
		EnableLastModified: true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("lm-conditional")
	})

	// Seed
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	lm := resp.Header.Get(fiber.HeaderLastModified)
	require.NotEmpty(t, lm)

	// If-Modified-Since equal to Last-Modified → 304
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderIfModifiedSince, lm)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotModified, resp.StatusCode)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))

	// If-Modified-Since in the future → 304 (resource hasn't changed since then)
	req = httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderIfModifiedSince, time.Now().Add(1*time.Hour).UTC().Format(http.TimeFormat))
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotModified, resp.StatusCode)

	// If-Modified-Since far in the past → 200 (resource modified after that)
	req = httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderIfModifiedSince, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat))
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
}

func Test_Cache_RFC9110_ETagTakesPrecedenceOverLastModified(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration:         10 * time.Second,
		EnableETag:         true,
		EnableLastModified: true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("precedence-body")
	})

	// Seed
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	etag := resp.Header.Get(fiber.HeaderETag)
	require.NotEmpty(t, etag)

	// Send BOTH If-None-Match (non-matching) and If-Modified-Since (far past).
	// Per RFC 9110 §8.3, If-None-Match takes precedence: non-match → 200.
	// If If-Modified-Since were evaluated, the past date would also yield 200,
	// so we use a matching ETag with a past If-Modified-Since to confirm
	// that ETag match triggers 304 regardless of the date.
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderIfNoneMatch, etag)
	req.Header.Set(fiber.HeaderIfModifiedSince, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat))
	resp, err = app.Test(req)
	require.NoError(t, err)
	// ETag matches → 304; If-Modified-Since is ignored per §8.3
	require.Equal(t, fiber.StatusNotModified, resp.StatusCode)

	// Non-matching ETag + future If-Modified-Since: ETag non-match → 200
	// (If-Modified-Since is never consulted)
	req = httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderIfNoneMatch, `"0000000000000000000000000000000000000000000000000000000000000000"`)
	req.Header.Set(fiber.HeaderIfModifiedSince, time.Now().Add(1*time.Hour).UTC().Format(http.TimeFormat))
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_Cache_CustomETagGenerator(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 10 * time.Second,
		ETagGenerator: func(_ fiber.Ctx, body []byte) string {
			return `"custom-` + string(body) + `"`
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("myval")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, `"custom-myval"`, resp.Header.Get(fiber.HeaderETag))

	// Conditional match with custom ETag → 304
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderIfNoneMatch, `"custom-myval"`)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotModified, resp.StatusCode)
}

func Test_Cache_CustomLastModifiedGenerator(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	app := fiber.New()
	app.Use(New(Config{
		Expiration:            10 * time.Second,
		EnableETag:            false,
		EnableLastModified:    true,
		LastModifiedGenerator: func(_ fiber.Ctx) time.Time { return fixedTime },
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("fixed-lm")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fixedTime.Format(http.TimeFormat), resp.Header.Get(fiber.HeaderLastModified))

	// If-Modified-Since equal to fixed time → 304
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderIfModifiedSince, fixedTime.Format(http.TimeFormat))
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotModified, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// Cache-Control response header generation
// ---------------------------------------------------------------------------

func Test_Cache_CacheControlOnMiss(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 30 * time.Second}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("miss-cc")
	})

	// First request is a miss → Cache-Control should be generated
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	cc := resp.Header.Get(fiber.HeaderCacheControl)
	require.NotEmpty(t, cc, "Cache-Control must be generated on miss")
	require.Contains(t, cc, "public")
	require.Contains(t, cc, "max-age=")
}

func Test_Cache_CacheControlOnHit(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 60 * time.Second}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hit-cc")
	})

	// Seed
	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	// Hit → Cache-Control with decreasing max-age
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	cc := resp.Header.Get(fiber.HeaderCacheControl)
	require.Contains(t, cc, "public, max-age=")
}

func Test_Cache_CacheControlMustRevalidateOnMiss(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 30 * time.Second}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "public, max-age=30, must-revalidate")
		return c.SendString("must-reval")
	})

	// Miss: handler sets must-revalidate; the middleware should not overwrite it
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.Equal(t, "public, max-age=30, must-revalidate", resp.Header.Get(fiber.HeaderCacheControl))
}

func Test_Cache_CacheControlMustRevalidateOnHit(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 60 * time.Second}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "public, max-age=60, must-revalidate")
		return c.SendString("reval-hit")
	})

	// Seed (miss stores must-revalidate flag in item)
	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	// Hit → generated Cache-Control must include must-revalidate
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	cc := resp.Header.Get(fiber.HeaderCacheControl)
	require.Contains(t, cc, "must-revalidate")
	require.Contains(t, cc, "public, max-age=")
}

func Test_Cache_CacheControlMustRevalidateOn304(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 60 * time.Second,
		EnableETag: true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "public, max-age=60, must-revalidate")
		return c.SendString("reval-304")
	})

	// Seed
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	etag := resp.Header.Get(fiber.HeaderETag)
	require.NotEmpty(t, etag)

	// Conditional hit → 304 with must-revalidate in Cache-Control
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderIfNoneMatch, etag)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotModified, resp.StatusCode)
	cc := resp.Header.Get(fiber.HeaderCacheControl)
	require.Contains(t, cc, "must-revalidate")
}

func Test_Cache_CacheControlDisabledNoHeaderGenerated(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration:          10 * time.Second,
		DisableCacheControl: true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("no-cc")
	})

	// Miss
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Empty(t, resp.Header.Get(fiber.HeaderCacheControl))

	// Hit
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Empty(t, resp.Header.Get(fiber.HeaderCacheControl))
}

func Test_Cache_HandlerSetCacheControlPreservedOnHit(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration:           10 * time.Second,
		StoreResponseHeaders: true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.SendString("handler-cc")
	})

	// Handler sets no-store → entry is not cached
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"))
	require.Equal(t, "no-store", resp.Header.Get(fiber.HeaderCacheControl))
}

func Test_Cache_ProxyRevalidatePreservedOnHit(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{Expiration: 60 * time.Second}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "public, max-age=60, proxy-revalidate")
		return c.SendString("proxy-reval")
	})

	// Seed
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	// Handler's Cache-Control preserved on miss (middleware skips generation when handler sets one)
	require.Equal(t, "public, max-age=60, proxy-revalidate", resp.Header.Get(fiber.HeaderCacheControl))

	// Hit → stored Cache-Control (including proxy-revalidate) restored verbatim
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, cacheHit, resp.Header.Get("X-Cache"))
	require.Equal(t, "public, max-age=60, proxy-revalidate", resp.Header.Get(fiber.HeaderCacheControl))
}

// ---------------------------------------------------------------------------
// Concurrent access
// ---------------------------------------------------------------------------

func Test_Cache_ConcurrentTagOperations(t *testing.T) {
	t.Parallel()

	ti := newTagIndex()
	const n = 100

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			tag := fmt.Sprintf("tag-%d", i%10)
			ti.add(key, []string{tag})
		}()
	}
	wg.Wait()

	// All keys should be present
	for i := 0; i < n; i++ {
		require.True(t, ti.has(fmt.Sprintf("key-%d", i)))
	}

	// Concurrent invalidations of all 10 tag buckets
	wg.Add(10)
	for i := 0; i < 10; i++ {
		i := i
		go func() {
			defer wg.Done()
			ti.invalidate([]string{fmt.Sprintf("tag-%d", i)})
		}()
	}
	wg.Wait()

	// Each key had exactly one tag (tag-N); after invalidating all buckets
	// every key's tag set is empty and the key is removed from the reverse index
	for i := 0; i < n; i++ {
		require.False(t, ti.has(fmt.Sprintf("key-%d", i)))
	}
}

func Test_Cache_ConcurrentRequestsWithTags(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 10 * time.Second,
		Tags: func(c fiber.Ctx) []string {
			return []string{fiber.Query(c, "group", "default")}
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("group=" + fiber.Query(c, "group", "default"))
	})
	app.Get("/invalidate", func(c fiber.Ctx) error {
		return InvalidateTags(c, fiber.Query[string](c, "group"))
	})

	const groups = 5
	const perGroup = 10
	var wg sync.WaitGroup

	// Seed all groups concurrently
	wg.Add(groups * perGroup)
	for g := 0; g < groups; g++ {
		for i := 0; i < perGroup; i++ {
			g, i := g, i
			go func() {
				defer wg.Done()
				path := fmt.Sprintf("/?group=g%d&i=%d", g, i)
				_, _ = app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
			}()
		}
	}
	wg.Wait()

	// Concurrent invalidations of different groups
	wg.Add(groups)
	for g := 0; g < groups; g++ {
		g := g
		go func() {
			defer wg.Done()
			path := fmt.Sprintf("/invalidate?group=g%d", g)
			_, _ = app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
		}()
	}
	wg.Wait()
	// Test passes if no race conditions or panics detected by -race flag
}

func Test_Cache_ConcurrentConditionalRequests(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration:         10 * time.Second,
		EnableETag:         true,
		EnableLastModified: true,
	}))

	app.Get("/*", func(c fiber.Ctx) error {
		return c.SendString("path=" + c.Path())
	})

	const paths = 10
	const rounds = 5

	// Seed all paths
	etags := make([]string, paths)
	for i := 0; i < paths; i++ {
		path := fmt.Sprintf("/item-%d", i)
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
		require.NoError(t, err)
		etags[i] = resp.Header.Get(fiber.HeaderETag)
		require.NotEmpty(t, etags[i])
	}

	// Fire concurrent conditional requests across all paths
	var wg sync.WaitGroup
	errCh := make(chan error, paths*rounds)

	wg.Add(paths * rounds)
	for i := 0; i < paths; i++ {
		for r := 0; r < rounds; r++ {
			i, r := i, r
			go func() {
				defer wg.Done()
				path := fmt.Sprintf("/item-%d", i)
				req := httptest.NewRequest(fiber.MethodGet, path, http.NoBody)
				if r%2 == 0 {
					// Even rounds: conditional with matching ETag → expect 304
					req.Header.Set(fiber.HeaderIfNoneMatch, etags[i])
				}
				resp, err := app.Test(req)
				if err != nil {
					errCh <- err
					return
				}
				if r%2 == 0 && resp.StatusCode != fiber.StatusNotModified {
					errCh <- fmt.Errorf("path %s round %d: expected 304, got %d", path, r, resp.StatusCode)
				}
				if r%2 != 0 && resp.StatusCode != fiber.StatusOK {
					errCh <- fmt.Errorf("path %s round %d: expected 200, got %d", path, r, resp.StatusCode)
				}
			}()
		}
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}
}

func Test_Cache_ConcurrentMixedWriteAndInvalidate(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Expiration: 10 * time.Second,
		Tags: func(c fiber.Ctx) []string {
			return []string{fiber.Query(c, "tag", "default")}
		},
	}))

	app.Get("/write", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	app.Get("/invalidate", func(c fiber.Ctx) error {
		return InvalidateTags(c, fiber.Query[string](c, "tag"))
	})

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			tag := fmt.Sprintf("t%d", i%5)
			if i%3 == 0 {
				// Invalidate
				path := fmt.Sprintf("/invalidate?tag=%s", tag)
				_, _ = app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
			} else {
				// Write
				path := fmt.Sprintf("/write?tag=%s&id=%d", tag, i)
				_, _ = app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
			}
		}()
	}
	wg.Wait()
	// Passes if no race conditions or panics; -race flag validates
}
