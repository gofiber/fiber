package cache

import (
	"bufio"
	"cmp"
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
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/clocktest"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/internal/storagetest"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

// Test_Cache_Security_UnusualInputs tests that requests and responses shaped to
// blow up a cache key, or to confuse how it is parsed, are neither a crash nor a
// way past the cache: whatever is bounded or hashed to fit still caches, and a
// response whose key cannot be bounded is never stored.
func Test_Cache_Security_UnusualInputs(t *testing.T) {
	t.Parallel()
	// More than maxVaryHeaders (32) names.
	varyNames := make([]string, 50)
	for i := range varyNames {
		varyNames[i] = fmt.Sprintf("X-Custom-Header-%d", i)
	}
	// Longer than maxKeyDimensionSegmentLength (192).
	longPath := "/" + strings.Repeat("a", 300)

	for _, tc := range []struct {
		app   fiber.Config // how the app reads the request
		name  string
		route string // where the handler answers, / when empty
		url   string
		vary  string   // the Vary header of the response, if any
		cache []string // the X-Cache status of each request, sent one after the other
	}{
		{
			// More than maxQueryParams (128), so the query is hashed instead of spelled out.
			name:  "excessive query params",
			url:   cacheTestCQuery(150, "param%[1]d=value%[1]d"),
			cache: []string{cacheMiss, cacheHit},
		},
		{
			// Every "=" becomes "%3D" once escaped, three times larger, so the key outgrows its buffer and is hashed.
			name:  "excessive query buffer",
			app:   fiber.Config{ReadBufferSize: 16384}, // large enough to accept the query string for testing
			url:   cacheTestCQuery(30, "key%d="+strings.Repeat("=", 50)),
			cache: []string{cacheMiss, cacheHit},
		},
		{
			// Treated as Vary: *, so the response is never stored.
			name:  "excessive Vary headers",
			url:   "/",
			vary:  strings.Join(varyNames, ", "),
			cache: []string{cacheUnreachable, cacheUnreachable},
		},
		{name: "long path segment is hashed", route: longPath, url: longPath, cache: []string{cacheMiss, cacheHit}},
		{
			// Malformed query string with invalid encoding: not a crash, and hashed as-is.
			name:  "malformed query string",
			url:   "/?invalid=%ZZ%XX",
			cache: []string{cacheMiss, cacheHit},
		},
		{
			// 100 values for the same parameter, hashed due to the param count.
			name:  "repeated query parameter",
			url:   cacheTestCQuery(100, "key=%d"),
			cache: []string{cacheMiss, cacheHit},
		},
		{
			// Empty and whitespace vary entries should be ignored.
			name:  "empty Vary entries",
			url:   "/",
			vary:  "Accept, , , ,   ,Accept-Encoding",
			cache: []string{cacheMiss, cacheHit},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New(tc.app)
			app.Use(New(Config{Expiration: 1 * time.Hour}))

			var count int
			app.Get(cmp.Or(tc.route, "/"), func(c fiber.Ctx) error {
				count++
				if tc.vary != "" {
					c.Set(fiber.HeaderVary, tc.vary)
				}
				return c.SendString("response")
			})

			wantCalls := 0
			for _, want := range tc.cache {
				resp := apptest.Expect(t, app, tc.url, fiber.StatusOK)
				require.Equal(t, want, resp.Header.Get("X-Cache"))
				if want != cacheHit {
					wantCalls++
				}
				require.Equal(t, wantCalls, count, "the handler runs for every request but a cache hit")
			}
		})
	}
}

// Test_Cache_Security_KeyValuesStayDistinct tests that the value of a dimension of
// the key cannot inject its delimiters into the key, or collide with another
// value through the way they are escaped: every distinct value is answered by the
// handler, and so stored under an entry of its own.
func Test_Cache_Security_KeyValuesStayDistinct(t *testing.T) {
	t.Parallel()
	header := headerPartition("X-Custom-Header", func(cfg *Config) { cfg.KeyHeaders = []string{"X-Custom-Header"} })

	for _, tc := range []struct {
		dimension credentialPartition
		name      string
		values    []string
	}{
		{
			// Try to inject delimiters used in key generation (avoiding null bytes which are invalid in HTTP).
			name:      "header delimiters",
			dimension: header,
			values: []string{
				"value|q=injected",
				"value|h=injected",
				"value|c=injected",
				"value|vary|injected",
				"sha256:fakehash",
			},
		},
		{
			name:      "cookie delimiters",
			dimension: credentialPartitions["keyCookies"],
			values:    []string{"value|injected", "value:injected", "value|vary|injected", "sha256:fakehash"},
		},
		{
			// Test backslash escaping scenarios.
			name:      "header backslashes",
			dimension: header,
			values: []string{
				"\\",           // Single backslash
				"\\\\",         // Double backslash
				"\\p",          // Escaped pipe character
				"\\c",          // Escaped colon character
				"value\\|test", // Backslash before pipe
				"value\\:test", // Backslash before colon
				"\\\\p",        // Double backslash then p
				"\\\\c",        // Double backslash then c
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := Config{Expiration: 1 * time.Hour}
			tc.dimension.configure(&cfg)
			app := fiber.New()
			app.Use(New(cfg))

			var count int
			app.Get("/", func(c fiber.Ctx) error {
				count++
				return c.SendString("response-" + tc.dimension.read(c))
			})

			for _, value := range tc.values {
				req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
				tc.dimension.send(req, value)

				got := doRequest(t, app, req)
				require.Equal(t, fiber.StatusOK, got.status, "value %q", value)
				require.Equal(t, "response-"+value, got.body, "value %q", value)
			}

			// Each value should create a distinct cache entry (no collision through injection).
			require.Equal(t, len(tc.values), count, "All values should create distinct cache entries")
		})
	}
}

// Concurrent requests whose query has more parameters than the key spells out.
func Test_Cache_Security_Concurrent_QueryParamDoS(t *testing.T) {
	t.Parallel()
	app, count := cacheTestAOrigin(New(Config{Expiration: 1 * time.Hour}), "response")

	url := cacheTestCQuery(200, "p%[1]d=v%[1]d")
	const numRequests = 100
	var wg sync.WaitGroup
	var errCount atomic.Int32
	for range numRequests {
		wg.Go(func() {
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, url, http.NoBody))
			if err != nil {
				errCount.Add(1)
				return
			}
			if resp.StatusCode != fiber.StatusOK {
				errCount.Add(1)
			}
		})
	}
	wg.Wait()

	require.Equal(t, int32(0), errCount.Load(), "No errors should occur during concurrent requests")
	// First request creates cache, rest should hit it
	require.LessOrEqual(t, count.Load(), int32(numRequests))
}

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
		got := getRoot(t, app, map[string]string{
			"X-Header-1": tc.header1,
			"X-Header-2": tc.header2,
			"Cookie":     fmt.Sprintf("cookie1=%s; cookie2=%s", tc.cookie1, tc.cookie2),
		})
		require.Equal(t, fiber.StatusOK, got.status, "Test case %d failed", i)
		require.Equal(t, tc.expected, got.body, "Test case %d failed", i)
	}

	require.Equal(t, len(testCases), count, "All test cases should create distinct cache entries")
}

// Test_Cache_Security_KeyHeaderRepeatedFieldLines asserts that every field line
// of a key header, or of a header the response varies on, reaches the cache key:
// a request that sends one field line must not be served the entry stored for a
// request that sent two.
//
// A name may arrive on more than one line, and that is equivalent to the
// comma-joined form on the wire (RFC 9110 Section 5.2). The key used to hash
// only the first line, so a request sending the header twice keyed identically
// to one sending just that first value — and got served its cached response.
func Test_Cache_Security_KeyHeaderRepeatedFieldLines(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		cfg  Config
		vary bool // the response varies on X-Tenant, which the key does not name
	}{
		{name: "KeyHeaders", cfg: Config{Expiration: 1 * time.Hour, KeyHeaders: []string{"X-Tenant"}}},
		{name: "Vary", cfg: Config{Expiration: time.Minute}, vary: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wire := cacheTestCWireOf(t, &tc.cfg, func(c fiber.Ctx) error {
				if tc.vary {
					c.Set("Vary", "X-Tenant")
				}
				return cacheTestCEchoTenants(c)
			})
			twoLines := []string{"public", "acme-private"}

			cacheTestCRequireReply(t, wire.field("X-Tenant", twoLines...), cacheMiss, "tenant:public;acme-private;")
			cacheTestCRequireReply(t, wire.field("X-Tenant", "public"), cacheMiss, "tenant:public;",
				"the single-line request must not hit the two-line entry")

			// The header being absent is also its own variant, not the same as one
			// present with an empty value.
			require.Equal(t, cacheMiss, wire.field("X-Tenant").cache)
			require.Equal(t, cacheMiss, wire.field("X-Tenant", "").cache,
				"an empty value must not key like an absent header")

			// Each variant still caches on its own key.
			cacheTestCRequireReply(t, wire.field("X-Tenant", twoLines...), cacheHit, "tenant:public;acme-private;")
			cacheTestCRequireReply(t, wire.field("X-Tenant", "public"), cacheHit, "tenant:public;")
		})
	}
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
	// newApp returns an app whose response carries a field line of header for each of lines.
	newApp := func(header string, lines ...string) *fiber.App {
		app := fiber.New()
		app.Use(New(Config{Expiration: time.Hour}))
		app.Get("/", func(c fiber.Ctx) error {
			for _, line := range lines {
				c.Response().Header.Add(header, line)
			}
			return c.SendString("tenant:" + c.Get("X-Tenant"))
		})
		return app
	}

	t.Run("vary on a later line still partitions", func(t *testing.T) {
		t.Parallel()
		wire := cacheTestCWire{t: t, app: newApp(fiber.HeaderVary, fiber.HeaderAcceptEncoding, "X-Tenant")}

		cacheTestCRequireReply(t, wire.field("X-Tenant", "acme"), cacheMiss, "tenant:acme")
		cacheTestCRequireReply(t, wire.field("X-Tenant", "globex"), cacheMiss, "tenant:globex",
			"a different X-Tenant must not hit acme's entry")

		// Each tenant still caches on its own key.
		cacheTestCRequireReply(t, wire.field("X-Tenant", "acme"), cacheHit, "tenant:acme")
	})

	for _, tc := range []struct {
		name   string
		header string
		lines  []string
	}{
		{name: "vary star on a later line blocks caching", header: fiber.HeaderVary, lines: []string{fiber.HeaderAcceptEncoding, "*"}},
		{name: "no-store on a later line blocks caching", header: fiber.HeaderCacheControl, lines: []string{"max-age=600", "no-store"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wire := cacheTestCWire{t: t, app: newApp(tc.header, tc.lines...)}

			require.Equal(t, cacheUnreachable, wire.field("X-Tenant", "acme").cache)
			cacheTestCRequireReply(t, wire.field("X-Tenant", "globex"), cacheUnreachable, "tenant:globex")
		})
	}
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
	wire := cacheTestCWireOf(t, &Config{Expiration: time.Hour, KeyHeaders: []string{"Cookie"}},
		func(c fiber.Ctx) error { return c.SendString("page-for:" + c.Cookies("session")) })

	cacheTestCRequireReply(t, wire.field("Cookie", "a=1", "session=attacker"), cacheMiss, "page-for:attacker")
	cacheTestCRequireReply(t, wire.field("Cookie", "a=1"), cacheMiss, "page-for:",
		"a request without the session cookie must not hit that entry")
	cacheTestCRequireReply(t, wire.field("Cookie", "a=1", "session=victim"), cacheMiss, "page-for:victim",
		"a different session must not hit the attacker's entry")

	// Each variant still caches on its own key, and the key does not depend on
	// whether the handler happened to read a cookie first.
	cacheTestCRequireReply(t, wire.field("Cookie", "a=1", "session=attacker"), cacheHit, "page-for:attacker")
	cacheTestCRequireReply(t, wire.field("Cookie", "a=1", "session=victim"), cacheHit, "page-for:victim")
}

func Test_Cache_Security_DelimiterCollisionPrevention(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 1 * time.Hour, KeyHeaders: []string{"X-Header"}, KeyCookies: []string{"session"}}))

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
		got := getRoot(t, app, map[string]string{"X-Header": tc.header, "Cookie": "session=" + tc.cookie})
		require.Equal(t, fiber.StatusOK, got.status, "Test case %d failed", i)
		require.Equal(t, fmt.Sprintf("h=%s,c=%s", tc.header, tc.cookie), got.body, "Test case %d failed", i)
	}

	require.Len(t, responses, len(testCases), "All test cases should create distinct cache entries")
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

	for _, tc := range []struct {
		name  string
		input string
	}{
		{name: "normal short value passes through verbatim", input: "plain"},
		// "sha256" (no colon) is a prefix of "sha256:" but is NOT in the reserved
		// namespace, so it must pass through verbatim (regression guard against
		// reversed HasPrefix arguments).
		{name: "prefix of reserved namespace is not over-hashed", input: "sha256"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.input, boundKeySegment(tc.input))
			require.Equal(t, tc.input, string(appendBoundKeySegment(nil, tc.input)))
		})
	}
}

// Test_Cache_Security_QueryBody_RawHashDomain verifies that the QUERY body is
// always hashed over its RAW bytes, never the escaped form. Mixing the two
// domains would let a small body collide with a large one: "a|" repeated escapes
// to "a\p" repeated, which must NOT hash-collide with a body that already
// contains the literal bytes "a\p" repeated.
func Test_Cache_Security_QueryBody_RawHashDomain(t *testing.T) {
	t.Parallel()
	app, count := cacheTestAOrigin(New(Config{Expiration: time.Hour, Methods: []string{fiber.MethodQuery}}), "response")

	// bodyA: 130 raw bytes (<=192, takes the verbatim branch); escapes to
	// "a\p" x65 = 195 bytes (>192), so it is hashed over the RAW "a|" x65.
	bodyA := strings.Repeat("a|", 65)
	// bodyB: 195 raw bytes of literal "a\p" (>192), hashed over the RAW bytes.
	// If the small branch hashed the escaped form, bodyA and bodyB would collide.
	bodyB := strings.Repeat("a\\p", 65)
	require.NotEqual(t, bodyA, bodyB)

	doQuery := func(body string) string {
		return apptest.Expect(t, app, cacheTestQuery(body), fiber.StatusOK).Header.Get("X-Cache")
	}

	require.Equal(t, cacheMiss, doQuery(bodyA))
	require.Equal(t, cacheMiss, doQuery(bodyB), "distinct bodies must not collide")
	require.Equal(t, int32(2), count.Load())
	require.Equal(t, cacheHit, doQuery(bodyA), "identical body must hit cache")
	require.Equal(t, int32(2), count.Load())
}

// The binders pick a decoder by Content-Type and Body inflates by
// Content-Encoding, so the same bytes declared otherwise are another query.
func Test_Cache_Security_QueryBody_KeyedWithItsMetadata(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 1 * time.Hour, Methods: []string{fiber.MethodQuery}}))
	app.Query("/", func(c fiber.Ctx) error {
		var filter struct {
			Category string `json:"category" form:"category"`
		}
		if err := c.Bind().Body(&filter); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		return c.SendString("category=" + filter.Category)
	})

	doQuery := func(header, value string) reply {
		return doRequest(t, app, cacheTestQuery(`{"category":"shoes"}`,
			apptest.WithHeader(fiber.HeaderContentType, fiber.MIMEApplicationJSON), apptest.WithHeader(header, value)))
	}

	cacheTestCRequireReply(t, doQuery(fiber.HeaderContentType, fiber.MIMEApplicationForm), cacheMiss, "category=")
	cacheTestCRequireReply(t, doQuery(fiber.HeaderContentType, fiber.MIMEApplicationJSON), cacheMiss, "category=shoes",
		"the same bytes as JSON are a different query")
	require.NotEqual(t, cacheHit, doQuery(fiber.HeaderContentEncoding, "gzip").cache,
		"the same bytes declared compressed are a different query")
	cacheTestCRequireReply(t, doQuery(fiber.HeaderContentType, fiber.MIMEApplicationJSON), cacheHit, "category=shoes")
}

func Test_Cache_Security_QueryBody_CannotInjectAuthorizationSuffix(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Expiration: 1 * time.Hour, Methods: []string{fiber.MethodQuery}}))

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

	cacheTestAExpectStatus(t, app, cacheTestQuery(baseBody, apptest.WithHeader(fiber.HeaderAuthorization, authHeader)),
		cacheMiss, fiber.StatusOK)

	unauthResp := cacheTestAExpectStatus(t, app, cacheTestQuery(craftedBody), cacheMiss, fiber.StatusOK)
	require.Equal(t, "handler auth=", apptest.Body(t, unauthResp))
	require.Equal(t, int32(2), count.Load())
}

// With StreamRequestBody the key must not drain a QUERY body it cannot bound, nor key two such bodies alike.
func Test_Cache_Security_QueryBody_StreamedBodyLimit(t *testing.T) {
	t.Parallel()
	const limit = 1024
	app := fiber.New(fiber.Config{StreamRequestBody: true, BodyLimit: limit})
	app.Use(New(Config{Expiration: time.Hour, Methods: []string{fiber.MethodQuery}}))
	app.Query("/", func(c fiber.Ctx) error {
		streamed := c.Request().IsBodyStream()
		body := []byte(nil)
		if streamed {
			var err error
			if body, err = io.ReadAll(c.BodyStream()); err != nil {
				return fmt.Errorf("read body stream: %w", err)
			}
		} else {
			body = c.Body()
		}
		return c.SendString(fmt.Sprintf("%.1s%d streamed=%v", body, len(body), streamed))
	})

	doQuery := func(body string, chunked bool) reply {
		t.Helper()
		req := cacheTestQuery(body)
		if chunked {
			req.TransferEncoding = []string{"chunked"}
		}
		got := doRequest(t, app, req)
		require.Equal(t, fiber.StatusOK, got.status)
		return got
	}

	for _, chunked := range []bool{false, true} {
		for _, fill := range []string{"A", "B"} {
			cacheTestCRequireReply(t, doQuery(strings.Repeat(fill, 4*limit), chunked), cacheUnreachable,
				fill+"4096 streamed=true", "chunked=%v", chunked)
		}
	}

	cacheTestCRequireReply(t, doQuery("small", true), cacheUnreachable, "s5 streamed=true",
		"a chunked body has no length to bound")
	for _, want := range []string{cacheMiss, cacheHit} {
		cacheTestCRequireReply(t, doQuery("small", false), want, "s5 streamed=false")
	}
	require.Equal(t, cacheMiss, doQuery("other", false).cache)
}

// countingReader yields zeros and counts how many were read.
type countingReader struct {
	left int
	read int
}

func (r *countingReader) Read(p []byte) (int, error) {
	if r.left == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.left)
	clear(p[:n])
	r.left -= n
	r.read += n
	return n, nil
}

// Measuring a stream with Response.Body read all of it into memory first.
func Test_Cache_Security_StreamIsNotBufferedToMeasureIt(t *testing.T) {
	t.Parallel()
	const maxBytes = 1024

	for name, tc := range map[string]struct {
		maxBytes uint
		size     int
		declared bool
		stored   bool
	}{
		"declared, too large":     {maxBytes: maxBytes, size: 64 * maxBytes, declared: true},
		"undeclared":              {maxBytes: maxBytes, size: 64 * maxBytes},
		"undeclared, no MaxBytes": {size: 64 * maxBytes},
		"declared, fits":          {maxBytes: maxBytes, size: maxBytes / 2, declared: true, stored: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stream := &countingReader{left: tc.size}
			app := fiber.New()
			app.Use(New(Config{MaxBytes: tc.maxBytes, Expiration: time.Hour}))
			app.Get("/", func(c fiber.Ctx) error {
				if tc.declared {
					return c.SendStream(stream, tc.size)
				}
				return c.SendStream(stream)
			})

			fctx := cacheTestServe(app.Handler(), "/")

			require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode())
			if tc.stored {
				require.Equal(t, cacheMiss, cacheTestXCache(fctx))
				require.Equal(t, tc.size, stream.read)
				return
			}
			require.Equal(t, cacheUnreachable, cacheTestXCache(fctx))
			require.True(t, fctx.Response.IsBodyStream())
			require.Zero(t, stream.read)
		})
	}
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
	wire := cacheTestCWireOf(t, &Config{Expiration: time.Hour}, func(c fiber.Ctx) error {
		c.Response().Header.Set(fiber.HeaderVary, fiber.HeaderCookie)
		// Reading a cookie is what forces collection mid-request.
		return c.SendString("page-for:" + c.Cookies("session"))
	})

	cacheTestCRequireReply(t, wire.field("Cookie", "a=1", "session=alice"), cacheMiss, "page-for:alice")
	cacheTestCRequireReply(t, wire.field("Cookie", "a=1", "session=alice"), cacheHit, "page-for:alice",
		"the store key and the lookup key must agree across cookie collection")

	// Stability must not have come from keying everything alike: a different
	// session still parts company, and the merged single-line spelling of the
	// same cookies reaches the entry the two lines stored.
	cacheTestCRequireReply(t, wire.field("Cookie", "a=1", "session=bob"), cacheMiss, "page-for:bob")
	cacheTestCRequireReply(t, wire.field("Cookie", "a=1; session=alice"), cacheHit, "page-for:alice")
}

// Test_RequestHeader_SecondFieldLine asserts that a request header the cache
// acts on is seen wherever it sits among its field lines.
func Test_RequestHeader_SecondFieldLine(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		header string
		why    string
		want   string   // its X-Cache status
		lines  []string // the field lines of a request that must not be taken for a plain one
	}{
		{
			// Peek returns only the first, so a scan of it alone read an empty first
			// line as "no credential" — and the request would then have been keyed
			// and served as anonymous, handing it whatever the anonymous entry held.
			//
			// Since this response permits no shared caching, carrying a credential
			// means the entry is refused outright — which only happens if the scan
			// looked past the empty first line.
			name:   "a credential on a later Authorization line",
			header: fiber.HeaderAuthorization,
			lines:  []string{"", "Bearer alice"},
			want:   cacheUnreachable,
			why:    "a credential on a later field line must not be read as anonymous",
		},
		{
			// The RFC 9111 §5.4 no-cache fallback is honored on a repeated Pragma
			// line, not only the first.
			name:   "no-cache on a later Pragma line",
			header: "Pragma",
			lines:  []string{"token", "no-cache"},
			want:   cacheMiss,
			why:    "no-cache on a later Pragma line must still force revalidation",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wire := cacheTestCWireOf(t, &Config{Expiration: time.Hour},
				func(c fiber.Ctx) error { return c.SendString("body") })

			// Prime an anonymous entry.
			require.Equal(t, cacheMiss, wire.get().cache)
			require.Equal(t, cacheHit, wire.get().cache)

			require.Equal(t, tc.want, wire.field(tc.header, tc.lines...).cache, tc.why)
		})
	}
}

// Test_AuthorizationHash_CoversEveryFieldLine asserts the value hashed into the
// key is the whole comma-joined credential. Hashing only the first field line
// put two principals that share it in the same partition, so one read the
// other's response.
func Test_AuthorizationHash_CoversEveryFieldLine(t *testing.T) {
	t.Parallel()
	wire := cacheTestCWireOf(t, &Config{Expiration: time.Hour}, func(c fiber.Ctx) error {
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

	// Both principals share the first field line and differ only on the second.
	cacheTestCRequireReply(t, wire.field("Authorization", "Bearer shared", "Bearer alice"), cacheMiss, "body for:Bearer alice")
	cacheTestCRequireReply(t, wire.field("Authorization", "Bearer shared", "Bearer alice"), cacheHit, "body for:Bearer alice")
	cacheTestCRequireReply(t, wire.field("Authorization", "Bearer shared", "Bearer bob"), cacheMiss, "body for:Bearer bob",
		"principals differing only on a later field line must not share a partition")
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

	cacheTestAExpect(t, app, "/", cacheMiss)

	resp := cacheTestAExpect(t, app, "/", cacheHit)
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
		cacheTestAExpectBody(t, app, "/", cacheUnreachable, fmt.Sprintf("page-for-client-%d", i))
	}
	require.Equal(t, int32(2), calls.Load())
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
			app.Use(New(Config{Expiration: time.Hour, Methods: []string{fiber.MethodGet, fiber.MethodPost}}))
			app.Post("/", func(c fiber.Ctx) error {
				calls.Add(1)
				c.Response().Header.Set(fiber.HeaderVary, fiber.HeaderContentType)
				// Reading a form value is what folds the header mid-request.
				return c.SendString("v=" + c.FormValue("a"))
			})

			do := func() string {
				req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader("a=1"))
				req.Header.Set(fiber.HeaderContentType, spelling)
				resp := apptest.Do(t, app, req)
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
			app.Use(New(Config{Expiration: time.Hour, Methods: []string{fiber.MethodGet, fiber.MethodPost}}))
			app.Post("/", func(c fiber.Ctx) error {
				c.Response().Header.Set(fiber.HeaderVary, fiber.HeaderContentType)
				return c.SendString("body-for:" + c.Get(fiber.HeaderContentType))
			})

			do := func(ct string) reply {
				req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader("a=1"))
				req.Header.Set(fiber.HeaderContentType, ct)
				return doRequest(t, app, req)
			}

			cacheTestCRequireReply(t, do("application/x-www-form-urlencoded"), cacheMiss, "body-for:application/x-www-form-urlencoded")
			cacheTestCRequireReply(t, do("application/json"), cacheMiss, "body-for:application/json",
				"a different Content-Type must not hit the form entry")
			cacheTestCRequireReply(t, do("application/json"), cacheHit, "body-for:application/json")
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
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("body-for:" + c.Get("X-Tenant")) })

			do := func(tenant string) reply { return cacheTestCGet(t, app, "/", "X-Tenant", tenant) }

			cacheTestCRequireReply(t, do("acme"), cacheMiss, "body-for:acme")
			cacheTestCRequireReply(t, do("beta"), cacheMiss, "body-for:beta",
				"a different tenant must not hit the first one's entry")
			cacheTestCRequireReply(t, do("acme"), cacheHit, "body-for:acme")
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

	resp := cacheTestAExpect(t, app, "/old", cacheMiss)
	require.Equal(t, fiber.StatusMovedPermanently, resp.StatusCode)
	require.Equal(t, "/new", resp.Header.Get("Location"))

	resp = cacheTestAExpect(t, app, "/old", cacheHit)
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
	wire := cacheTestCExactWire(t, func(c fiber.Ctx) error {
		// With normalizing off a handler reads the name as it arrived.
		if tok := c.Get("authorization"); tok != "" {
			return c.SendString("PRIVATE for " + tok)
		}
		return c.SendString("PUBLIC")
	})

	cacheTestCRequireReply(t, wire.get("authorization: Bearer alice-secret"), cacheUnreachable, "PRIVATE for Bearer alice-secret",
		"an authorized response must not enter the shared store")
	cacheTestCRequireReply(t, wire.get(), cacheMiss, "PUBLIC", "an anonymous request must not be served the authorized response")
	cacheTestCRequireReply(t, wire.get(), cacheHit, "PUBLIC")
}

// Test_RequestDirectives_HonoredWithoutHeaderNormalizing covers the same
// lookup for the request directives, which are read the same way: a lower-case
// "cache-control: no-store" was ignored outright.
func Test_RequestDirectives_HonoredWithoutHeaderNormalizing(t *testing.T) {
	t.Parallel()
	wire := cacheTestCExactWire(t, func(c fiber.Ctx) error { return c.SendString("body") })

	require.Equal(t, cacheMiss, wire.get().cache)
	require.Equal(t, cacheHit, wire.get().cache)
	require.Empty(t, wire.get("cache-control: no-store").cache, "no-store must be honored whatever case it arrived in")
	require.Equal(t, cacheMiss, wire.get("cache-control: no-cache").cache, "no-cache must force revalidation")
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
	wire := cacheTestCExactWire(t, func(c fiber.Ctx) error {
		tok := c.Get("Authorization")
		if tok == "" {
			tok = c.Get("authorization")
		}
		if tok == "" {
			return c.SendString("PUBLIC")
		}
		return c.SendString("PRIVATE for " + tok)
	})

	// The canonical line is present but empty, so the byte-for-byte lookup
	// succeeds and would have stopped there.
	cacheTestCRequireReply(t, wire.get("Authorization:", "authorization: Bearer alice"), cacheUnreachable, "PRIVATE for Bearer alice",
		"the credential on the second spelling must still be seen")
	cacheTestCRequireReply(t, wire.get(), cacheMiss, "PUBLIC", "an anonymous request must not be served the authorized response")
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
	// Lower case throughout, which is legal on the wire and is what
	// DisableHeaderNormalizing preserves.
	lower := [][2]string{
		{"cache-control", "max-age=60"},
		{"age", "0"},
		{"etag", `"v1"`},
		{"expires", "Wed, 21 Oct 2099 07:28:00 GMT"},
		{"date", "Wed, 21 Oct 2020 07:28:00 GMT"},
	}
	single := []string{
		fiber.HeaderCacheControl, fiber.HeaderAge, fiber.HeaderETag, fiber.HeaderExpires, fiber.HeaderDate,
	}

	for _, normalize := range []bool{true, false} {
		t.Run(fmt.Sprintf("normalize=%v", normalize), func(t *testing.T) {
			t.Parallel()
			app := fiber.New(fiber.Config{DisableHeaderNormalizing: !normalize})
			app.Use(New(Config{StoreResponseHeaders: true, Expiration: time.Minute}))
			app.Get("/", func(c fiber.Ctx) error {
				for _, header := range lower {
					c.Response().Header.Set(header[0], header[1])
				}
				return c.SendString("body")
			})

			for _, want := range []string{cacheMiss, cacheHit} {
				resp := cacheTestAExpect(t, app, "/", want)
				cacheTestCRequireOneLine(t, resp, want, single...)

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
					resp := apptest.Do(t, app, "/")
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
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("body") })

			for _, want := range []string{cacheMiss, cacheHit} {
				resp := cacheTestAExpect(t, app, "/", want)
				cacheTestCRequireOneLine(t, resp, want, fiber.HeaderCacheControl, fiber.HeaderETag, fiber.HeaderExpires)
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
				resp := apptest.Do(t, app, "/")
				require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"),
					"a response naming a cookie is never stored")
			}
			require.Equal(t, int64(2), n.Load(), "each client must reach the handler")
		})
	}
}

// Test_Cache_QuotedDirective covers a Cache-Control whose extension value is a
// quoted-string containing commas.
//
// The directive scan split on every comma, so the tokens inside the quotes were
// read as directives of their own. A response saying nothing about sharing was
// therefore taken to have said "public" — enough to pass the gate that lets a
// cookie-setting response be stored, after which its body went to later clients
// with only the cookie removed. RFC 9111 §5.2 makes the value a quoted-string;
// a comma inside one does not end the directive.
func Test_Cache_QuotedDirective(t *testing.T) {
	t.Parallel()
	const noSharing = "a token inside a quoted extension value does not authorize sharing"
	uncacheable := []string{cacheUnreachable, cacheUnreachable}

	for _, tc := range []struct {
		name  string
		cc    string
		why   string
		cache []string // the X-Cache status of each request, sent one after the other
	}{
		{name: `ext="a, public, b"`, cc: `ext="a, public, b"`, why: noSharing, cache: uncacheable},
		{name: `ext="a, s-maxage=99, b"`, cc: `ext="a, s-maxage=99, b"`, why: noSharing, cache: uncacheable},
		// The escaped quote must not end the value early either, or the scan
		// resumes inside the string and finds the token after it.
		{name: `ext="a\", public, b"`, cc: `ext="a\", public, b"`, why: noSharing, cache: uncacheable},
		// The control: a quoted value must not hide the directives written beside it.
		{
			name:  `ext="a, b", public`,
			cc:    `ext="a, b", public`,
			why:   "public written outside the quotes still authorizes sharing",
			cache: []string{cacheMiss, cacheHit},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(Config{Expiration: time.Minute}))

			var n atomic.Int64
			app.Get("/", func(c fiber.Ctx) error {
				id := n.Add(1)
				c.Response().Header.Set(fiber.HeaderCacheControl, tc.cc)
				c.Response().Header.Set(fiber.HeaderSetCookie, fmt.Sprintf("session=client-%d", id))
				return c.SendString(fmt.Sprintf("private to client-%d", id))
			})

			wantRuns := int64(0)
			for _, want := range tc.cache {
				resp := apptest.Do(t, app, "/")
				require.Equal(t, want, resp.Header.Get("X-Cache"), tc.why)
				if want != cacheHit {
					wantRuns++
				}
			}
			require.Equal(t, wantRuns, n.Load(), "every request not answered from the cache must reach the handler")
		})
	}
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
	cacheTestAExpect(t, app, "/", cacheMiss)

	// The hit is where this middleware writes them from the entry, and there it
	// owns the field: one line, whatever spellings it had to replace.
	resp := cacheTestAExpect(t, app, "/", cacheHit)

	cacheTestCRequireOneLine(t, resp, "hit", fiber.HeaderCacheControl, fiber.HeaderETag)
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
				wire := cacheTestCWire{t: t, app: app}

				do := func(cookieHeader, session string) reply {
					req := fasthttp.AcquireRequest()
					defer fasthttp.ReleaseRequest(req)
					if !normalize {
						req.Header.DisableNormalizing()
					}
					req.Header.SetMethod(fiber.MethodGet)
					req.SetRequestURI("/")
					req.Header.SetHost("example.com")
					req.Header.Set(cookieHeader, "sess="+session)
					return wire.serve(req)
				}

				// alice populates an entry and then reads her own back, so a
				// miss below means a different key rather than an uncacheable
				// response.
				do(fiber.HeaderCookie, "alice")
				cacheTestCRequireReply(t, do(fiber.HeaderCookie, "alice"), cacheHit, "private to alice")

				cacheTestCRequireReply(t, do(fiber.HeaderCookie, "bob"), cacheMiss, "private to bob", "bob must not land on alice's entry")

				// And the same when the name arrives in lower case, which is
				// what HTTP/2 and HTTP/3 put on the wire.
				cacheTestCRequireReply(t, do("cookie", "carol"), cacheMiss, "private to carol",
					"carol must not land on an earlier session's entry")
				cacheTestCRequireReply(t, do("cookie", "carol"), cacheHit, "private to carol")
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
		resp := cacheTestAExpect(t, app, "/", want)
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
		resp := cacheTestAExpect(t, app, "/", want)
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
	cacheTestAExpect(t, app, twoLines, cacheMiss)

	oneLine := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	oneLine.Header.Set(fiber.HeaderAuthorization, "Bearer shared,Bearer alice")
	resp := apptest.Do(t, app, oneLine)
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

	cacheTestAExpect(t, app, "/", cacheMiss)

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

	resp := apptest.Do(t, app, "/")
	require.Equal(t, cacheUnreachable, resp.Header.Get("X-Cache"), "an entry carrying a cookie is not replayed")
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
				resp := apptest.Do(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHeader("X-User", user)))
				require.Equal(t, "secret-for-"+user, apptest.Body(t, resp), "each client gets its own body")
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
		// A custom generator, as an application preserving the old key format
		// across the upgrade would have.
		KeyGenerator: pathKey,
	}))
	app.Get("/me", func(c fiber.Ctx) error { return c.SendString("handler ran") })

	// Seed what the previous version wrote for alice: the anonymous key, holding
	// her authenticated body.
	raw, err := cacheTestALegacyItem("alice's private page").MarshalMsg(nil)
	require.NoError(t, err)
	require.NoError(t, storage.Set("GET|/me", raw, time.Minute))

	resp := cacheTestAExpectStatus(t, app, "/me", cacheMiss, fiber.StatusOK)
	require.Equal(t, "handler ran", apptest.Body(t, resp), "the legacy anonymous entry must not be served")
}

const (
	tenantAHost = "tenant-a.example"
	tenantBHost = "tenant-b.example"
)

// getFromHost sends a GET for target under the given Host through app.
func getFromHost(t *testing.T, app *fiber.App, host, target string) reply {
	t.Helper()

	return doRequest(t, app, apptest.Req(fiber.MethodGet, "http://"+host+target))
}

// Test_Cache_Security_KeyIncludesAuthority asserts that the default key is
// partitioned by the request scheme and host. A cache key is composed of the
// request method and the target URI at a minimum (RFC 9111 §2), and the target
// URI carries the authority: two requests that differ only in Host must never
// share an entry. Keyed on the path alone, a page routed with app.Domain
// answered the other host, and a redirect built from c.BaseURL for one host
// was served for the other.
func Test_Cache_Security_KeyIncludesAuthority(t *testing.T) {
	t.Parallel()
	t.Run("domain routed pages stay with their host", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: time.Hour}))
		app.Domain(tenantAHost).Get("/dash", func(c fiber.Ctx) error { return c.SendString("tenant-a") })
		app.Domain(tenantBHost).Get("/dash", func(c fiber.Ctx) error { return c.SendString("tenant-b") })

		cacheTestCRequireReply(t, getFromHost(t, app, tenantAHost, "/dash"), cacheMiss, "tenant-a")
		cacheTestCRequireReply(t, getFromHost(t, app, tenantBHost, "/dash"), cacheMiss, "tenant-b",
			"tenant B must not hit tenant A's entry")

		// Each host still caches on a key of its own.
		cacheTestCRequireReply(t, getFromHost(t, app, tenantAHost, "/dash"), cacheHit, "tenant-a")
		cacheTestCRequireReply(t, getFromHost(t, app, tenantBHost, "/dash"), cacheHit, "tenant-b")
	})

	t.Run("a redirect built from the authority is not served for another host", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: time.Hour}))
		app.Get("/canonical", func(c fiber.Ctx) error {
			return c.Redirect().Status(fiber.StatusMovedPermanently).To(c.BaseURL() + "/landing")
		})

		got := getFromHost(t, app, tenantAHost, "/canonical")
		require.Equal(t, fiber.StatusMovedPermanently, got.status)
		require.Equal(t, cacheMiss, got.cache)
		require.Equal(t, "http://"+tenantAHost+"/landing", got.header.Get(fiber.HeaderLocation))

		got = getFromHost(t, app, tenantBHost, "/canonical")
		require.Equal(t, fiber.StatusMovedPermanently, got.status)
		require.Equal(t, cacheMiss, got.cache, "the second host must not be served the first host's redirect")
		require.Equal(t, "http://"+tenantBHost+"/landing", got.header.Get(fiber.HeaderLocation))
	})

	t.Run("the port is part of the authority", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: time.Hour}))
		app.Get("/", func(c fiber.Ctx) error { return c.SendString(c.Host()) })

		cacheTestCRequireReply(t, getFromHost(t, app, "example.com:8080", "/"), cacheMiss, "example.com:8080")
		cacheTestCRequireReply(t, getFromHost(t, app, "example.com:9090", "/"), cacheMiss, "example.com:9090",
			"a different port is a different authority")
	})

	t.Run("vary manifests are partitioned by host as well", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: time.Hour}))
		app.Get("/", func(c fiber.Ctx) error {
			c.Set(fiber.HeaderVary, "X-Tenant")
			return c.SendString(c.Host())
		})

		// The same X-Tenant on both hosts: only the authority keeps them apart,
		// in the manifest key and in the variant key beneath it.
		do := func(host string) reply {
			return doRequest(t, app, apptest.Req(fiber.MethodGet, "http://"+host+"/", apptest.WithHeader("X-Tenant", "acme")))
		}

		cacheTestCRequireReply(t, do(tenantAHost), cacheMiss, tenantAHost)
		cacheTestCRequireReply(t, do(tenantBHost), cacheMiss, tenantBHost,
			"the variant stored for tenant A must not answer tenant B")

		cacheTestCRequireReply(t, do(tenantAHost), cacheHit, tenantAHost)
		cacheTestCRequireReply(t, do(tenantBHost), cacheHit, tenantBHost)
	})

	t.Run("one cache per domain over a shared store stays isolated", func(t *testing.T) {
		t.Parallel()
		// Separate middleware instances do not help on their own: over a shared
		// store, such as one Redis, their keys land in the same namespace, so
		// the key itself has to carry the host.
		store := memory.New()
		app := fiber.New()
		app.Domain(tenantAHost).Use(New(Config{Storage: store, Expiration: time.Hour}))
		app.Domain(tenantBHost).Use(New(Config{Storage: store, Expiration: time.Hour}))
		app.Domain(tenantAHost).Get("/dash", func(c fiber.Ctx) error { return c.SendString("tenant-a") })
		app.Domain(tenantBHost).Get("/dash", func(c fiber.Ctx) error { return c.SendString("tenant-b") })

		cacheTestCRequireReply(t, getFromHost(t, app, tenantAHost, "/dash"), cacheMiss, "tenant-a")
		cacheTestCRequireReply(t, getFromHost(t, app, tenantBHost, "/dash"), cacheMiss, "tenant-b",
			"tenant B's instance must not find tenant A's entry in the shared store")
	})
}

// Test_Cache_Security_KeyAuthorityFollowsTrustedProxy pins the field source of
// the authority segment: c.Scheme and c.Host, which read X-Forwarded-Proto and
// X-Forwarded-Host from a trusted proxy. That is the authority app.Domain routes
// on and c.BaseURL builds from, so the key has to follow it — behind such a
// proxy every tenant may arrive under the backend's own Host. From an untrusted
// peer the handlers ignore the same fields, so they do not partition the cache
// either.
func Test_Cache_Security_KeyAuthorityFollowsTrustedProxy(t *testing.T) {
	t.Parallel()
	do := func(t *testing.T, app *fiber.App, forwardedHost, forwardedProto string) reply {
		t.Helper()
		return doRequest(t, app, apptest.Req(fiber.MethodGet, "http://backend.internal/",
			apptest.WithHeader(fiber.HeaderXForwardedHost, forwardedHost),
			apptest.WithHeader(fiber.HeaderXForwardedProto, forwardedProto)))
	}
	baseURL := func(c fiber.Ctx) error { return c.SendString(c.BaseURL()) }

	t.Run("trusted proxy", func(t *testing.T) {
		t.Parallel()
		// app.Test connects from 0.0.0.0, so that is the trusted proxy.
		app := fiber.New(fiber.Config{
			TrustProxy:       true,
			TrustProxyConfig: fiber.TrustProxyConfig{Proxies: []string{"0.0.0.0"}},
		})
		app.Use(New(Config{Expiration: time.Hour}))
		app.Get("/", baseURL)

		cacheTestCRequireReply(t, do(t, app, tenantAHost, "https"), cacheMiss, "https://"+tenantAHost)
		cacheTestCRequireReply(t, do(t, app, tenantBHost, "https"), cacheMiss, "https://"+tenantBHost,
			"a different forwarded host must not hit the first entry")
		cacheTestCRequireReply(t, do(t, app, tenantAHost, "http"), cacheMiss, "http://"+tenantAHost,
			"a different forwarded scheme must not hit the https entry")
		cacheTestCRequireReply(t, do(t, app, tenantAHost, "https"), cacheHit, "https://"+tenantAHost)
	})

	t.Run("untrusted peer", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Expiration: time.Hour}))
		app.Get("/", baseURL)

		cacheTestCRequireReply(t, do(t, app, tenantAHost, "https"), cacheMiss, "http://backend.internal")

		// Not the authority here, so not a partition either: the same entry
		// answers whatever the untrusted peer claims to forward.
		cacheTestCRequireReply(t, do(t, app, tenantBHost, "http"), cacheHit, "http://backend.internal")
	})
}

// A URL whose anonymous responses vary on Accept-Encoding only, as compress
// makes them, and whose signed-in ones add Cookie.
func Test_Cache_Security_StoredUnderTheResponsesOwnVary(t *testing.T) {
	t.Parallel()
	get := func(t *testing.T, app *fiber.App, cookie, cacheControl string) reply {
		t.Helper()
		cookies := apptest.WithHeader(fiber.HeaderCookie, cookie)
		reload := apptest.WithHeaderIf(fiber.HeaderCacheControl, cacheControl)
		return doRequest(t, app, apptest.Req(fiber.MethodGet, "/", cookies, reload))
	}

	for _, kind := range storageKinds {
		t.Run(kind+"/response adds Cookie", func(t *testing.T) {
			t.Parallel()
			probe := &accountingProbe{}
			app := fiber.New()
			app.Use(New(Config{Storage: storageOf(kind), MaxBytes: 1024, accounting: probe.record}))
			app.Get("/", func(c fiber.Ctx) error {
				c.Vary(fiber.HeaderAcceptEncoding)
				user := c.Cookies("session")
				if user == "" {
					return c.SendString("sign in")
				}
				c.Vary(fiber.HeaderCookie)
				return c.SendString("private to " + user)
			})

			// alice reloads while the manifest names Accept-Encoding only.
			cacheTestCRequireReply(t, get(t, app, "session=alice", ""), cacheMiss, "private to alice")
			cacheTestCRequireReply(t, get(t, app, "x=1", ""), cacheMiss, "sign in")
			cacheTestCRequireReply(t, get(t, app, "session=alice", "max-age=0"), cacheMiss, "private to alice")
			cacheTestCRequireReply(t, get(t, app, "x=2", ""), cacheMiss, "sign in")
			for _, cookie := range []string{"x=3", "_ga=77", ""} {
				cacheTestCRequireReply(t, get(t, app, cookie, ""), cacheHit, "sign in", "Cookie: %q", cookie)
			}
			// The sign-in entry alice's reload looked up stays beside hers.
			require.Equal(t, 2, probe.nodes())
			signIn := charged(variantKey(defaultKey("/"), "accept-encoding"), len("sign in"), textPlain, "Accept-Encoding")
			alice := charged(variantKey(defaultKey("/"), "accept-encoding", "cookie"), len("private to alice"), textPlain, "Accept-Encoding, Cookie")
			require.Equal(t, signIn+alice, probe.counted())

			cacheTestCRequireReply(t, get(t, app, "session=bob", "max-age=0"), cacheMiss, "private to bob")
			cacheTestCRequireReply(t, get(t, app, "session=alice", ""), cacheHit, "private to alice")
			cacheTestCRequireReply(t, get(t, app, "x=4", ""), cacheMiss, "sign in")
		})

		t.Run(kind+"/response drops Vary", func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(Config{Storage: storageOf(kind)}))
			app.Get("/", func(c fiber.Ctx) error {
				user := c.Cookies("session")
				if user == "" {
					return c.SendString("sign in")
				}
				c.Vary(fiber.HeaderCookie)
				return c.SendString("private to " + user)
			})

			cacheTestCRequireReply(t, get(t, app, "session=alice", ""), cacheMiss, "private to alice")
			// Drops the manifest, so the entry belongs under the base key.
			cacheTestCRequireReply(t, get(t, app, "x=1", ""), cacheMiss, "sign in")
			cacheTestCRequireReply(t, get(t, app, "x=2", ""), cacheHit, "sign in")
		})
	}
}

func Test_Cache_Security_VaryManifestOwnsItsBytes(t *testing.T) {
	t.Parallel()
	storage := newMutatingStorage(nil) // keeps the slices it is given
	app := fiber.New()
	app.Use(New(Config{Storage: storage}))
	app.Get("/account", func(c fiber.Ctx) error {
		c.Vary("cookie")
		return c.SendString("user=" + c.Cookies("user"))
	})
	app.Get("/about", func(c fiber.Ctx) error { return c.SendString("about") })
	handler := app.Handler()

	// One RequestCtx for every request, as the server keeps per connection.
	fctx := &fasthttp.RequestCtx{}
	get := func(target, cookie string) reply {
		fctx.Request.Reset()
		fctx.Response.Reset()
		fctx.Request.Header.SetMethod(fiber.MethodGet)
		fctx.Request.SetRequestURI(target)
		fctx.Request.Header.SetHost("example.com")
		fctx.Request.Header.Set(fiber.HeaderCookie, cookie)
		handler(fctx)
		return reply{cache: string(fctx.Response.Header.Peek("X-Cache")), body: string(fctx.Response.Body())}
	}

	require.Equal(t, "user=alice", get("/account", "user=alice").body)
	get("/about?n=1", "")

	manifests := 0
	for key, value := range storage.Data {
		if strings.HasPrefix(key, manifestKeyPrefix) {
			manifests++
			require.Equal(t, "cookie", string(value))
		}
	}
	require.Equal(t, 1, manifests)

	require.Equal(t, "user=bob", get("/account", "user=bob").body)
	get("/about?n=2", "")

	got := get("/account", "user=mallory")
	require.Equal(t, cacheMiss, got.cache)
	require.Equal(t, "user=mallory", got.body)
}

// A short or empty body barely moved the byte count, so a flood of new URLs
// never forced an eviction and every one left a heap node behind.
func Test_Cache_Security_ShortBodiesAreBoundedByEntryCount(t *testing.T) {
	t.Parallel()
	const (
		maxBytes = minTrackedEntries * trackedEntryOverhead // caps the entries at minTrackedEntries before their bytes
		urls     = 3 * minTrackedEntries
	)

	for name, handler := range map[string]fiber.Handler{
		"204":    func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) },
		"301":    func(c fiber.Ctx) error { return c.Redirect().Status(fiber.StatusMovedPermanently).To("/new") },
		"1 byte": func(c fiber.Ctx) error { return c.SendString("x") },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			probe := &accountingProbe{}
			app := fiber.New()
			app.Use(New(Config{MaxBytes: maxBytes, Expiration: time.Hour, accounting: probe.record}))
			app.Get("/", handler)
			h := app.Handler()

			get := func(i int, cacheControl string) *fasthttp.RequestCtx {
				return cacheTestServe(h, fmt.Sprintf("/?i=%d", i), fiber.HeaderCacheControl, cacheControl)
			}

			for i := range urls {
				require.Equal(t, cacheMiss, cacheTestXCache(get(i, "")))
			}
			require.LessOrEqual(t, probe.counted(), uint(maxBytes))
			require.Equal(t, minTrackedEntries, probe.nodes())

			require.Equal(t, cacheHit, cacheTestXCache(get(urls-1, "")))
			require.Equal(t, fiber.StatusGatewayTimeout, get(0, "only-if-cached").Response.StatusCode())
		})
	}
}

func countVaryManifests(storage *storagetest.Memory) int {
	storage.Mu.RLock()
	defer storage.Mu.RUnlock()

	manifests := 0
	for key := range storage.Data {
		if strings.HasPrefix(key, manifestKeyPrefix) {
			manifests++
		}
	}
	return manifests
}

// A manifest is a record per URL that MaxBytes does not count, so one left
// behind by eviction let new URLs grow the store until Expiration.
func Test_Cache_Security_EvictionRemovesVaryManifest(t *testing.T) {
	t.Parallel()
	storage := storagetest.New()
	app := fiber.New()
	// Room for two of the last entries, whose query has two digits.
	entry := charged(variantKey(strings.Replace(defaultKey("/"), "|q=", "|q=n=10", 1), "accept-encoding"), len("12345"), textPlain, "Accept-Encoding")
	app.Use(New(Config{Storage: storage, MaxBytes: 2 * entry, Expiration: time.Hour}))
	app.Get("/", func(c fiber.Ctx) error {
		c.Vary(fiber.HeaderAcceptEncoding)
		return c.SendString("12345")
	})

	for i := range 50 {
		resp := apptest.Do(t, app, fmt.Sprintf("/?n=%d", i))
		require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	}
	require.Equal(t, 2, countVaryManifests(storage), "one per entry that fits MaxBytes")
}

// only-if-cached drops an expired entry and stores nothing, so the manifest goes
// with the next store instead.
func Test_Cache_Security_DroppedVariantRemovesVaryManifest(t *testing.T) {
	t.Parallel()
	storage := storagetest.New()
	clock := clocktest.NewClock(time.Now().Truncate(time.Second))
	app := fiber.New()
	app.Use(New(Config{Storage: storage, MaxBytes: 1000, Expiration: time.Hour, clock: clock.Now}))
	app.Get("/*", func(c fiber.Ctx) error {
		c.Vary(fiber.HeaderAcceptEncoding)
		c.Set(fiber.HeaderCacheControl, "max-age=1")
		return c.SendString("12345")
	})

	resp := apptest.Do(t, app, "/a")
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.Equal(t, 1, countVaryManifests(storage))

	clock.Add(2 * time.Second)
	req := httptest.NewRequest(fiber.MethodGet, "/a", http.NoBody)
	req.Header.Set(fiber.HeaderCacheControl, "only-if-cached")
	apptest.Expect(t, app, req, fiber.StatusGatewayTimeout)

	resp = apptest.Do(t, app, "/b")
	require.Equal(t, cacheMiss, resp.Header.Get("X-Cache"))
	require.Equal(t, 1, countVaryManifests(storage), "only the manifest of /b")
}

// The manifest is shared by the variants of a URL: evicting one leaves it to the others.
func Test_Cache_Security_EvictedVariantKeepsSiblingManifest(t *testing.T) {
	t.Parallel()
	storage := storagetest.New()
	app := fiber.New()
	entry := charged(variantKey(defaultKey("/a"), "x-variant"), len("12345"), textPlain, "X-Variant")
	app.Use(New(Config{Storage: storage, MaxBytes: 2 * entry, ExpirationGenerator: stableAscendingExpiration()}))
	app.Get("/*", func(c fiber.Ctx) error {
		c.Vary("X-Variant")
		return c.SendString("12345")
	})

	get := func(target, variant string) string {
		t.Helper()
		req := httptest.NewRequest(fiber.MethodGet, target, http.NoBody)
		req.Header.Set("X-Variant", variant)
		resp := apptest.Do(t, app, req)
		return resp.Header.Get("X-Cache")
	}

	require.Equal(t, cacheMiss, get("/a", "one"))
	require.Equal(t, cacheMiss, get("/a", "two"))
	// Evicts the first variant, the entry closest to expiring.
	require.Equal(t, cacheMiss, get("/b", "one"))
	require.Equal(t, 2, countVaryManifests(storage))
	require.Equal(t, cacheHit, get("/a", "two"))
	require.Equal(t, cacheMiss, get("/a", "one"))
}

// A request the server refuses replays the Use chain without its route, so what
// the chain leaves, a catch-all's 404 or the seeded status, is not the URL's response.
func Test_Cache_Security_RefusedRequestIsNotStored(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		refused  string
		catchAll bool
	}{
		{name: "body over BodyLimit with a catch-all", refused: "Content-Length: 99999999\r\n\r\n", catchAll: true},
		{
			name:    "malformed trailer",
			refused: "Transfer-Encoding: chunked\r\n\r\n0\r\nContent-Length: unsupported http request method\r\n\r\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New())
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("home page") })
			if tc.catchAll {
				app.Use(func(c fiber.Ctx) error { return c.Status(fiber.StatusNotFound).SendString("no such page") })
			}

			ln := fasthttputil.NewInmemoryListener()
			errCh := make(chan error, 1)
			go func() { errCh <- app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()
			t.Cleanup(func() {
				require.NoError(t, app.Shutdown())
				require.NoError(t, <-errCh)
			})

			// send writes GET target and the header lines extra, ended by their blank line, and reads the answer.
			send := func(target, extra string) reply {
				conn, err := ln.Dial()
				require.NoError(t, err)
				defer conn.Close() //nolint:errcheck // usually closed by the server already
				require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
				const lines = " HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n"
				_, err = conn.Write([]byte("GET " + target + lines + extra))
				require.NoError(t, err)
				resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
				require.NoError(t, err)
				return reply{status: resp.StatusCode, cache: resp.Header.Get("X-Cache"), body: apptest.Body(t, resp)}
			}

			// "/" holds a fresh entry the refused request asks to replace, "/?page=2" none yet.
			require.Equal(t, fiber.StatusOK, send("/", "\r\n").status)

			for _, target := range []string{"/", "/?page=2"} {
				refused := send(target, "Cache-Control: no-cache\r\n"+tc.refused)
				require.GreaterOrEqual(t, refused.status, fiber.StatusBadRequest, target)
				require.Equal(t, cacheUnreachable, refused.cache, target)

				retry := send(target, "\r\n")
				require.Equal(t, fiber.StatusOK, retry.status, target)
				require.Equal(t, "home page", retry.body, target)
			}
		})
	}
}
