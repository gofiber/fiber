package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type keygenCase struct {
	headers  map[string]string
	name     string
	uri      string
	method   string
	body     string
	cookie   string
	want     string
	keyCooks []string
	noQuery  bool
}

func keygenCorpus() []keygenCase {
	return []keygenCase{
		{name: "noquery", uri: "/demo", want: "/|q=|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "single", uri: "/demo?foo=bar", want: "/|q=foo=bar|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "multi_dup", uri: "/demo?b=2&a=1&a=3", want: "/|q=a=1&a=3&b=2|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "path_delims", uri: "/a|b:c\\d?x=1", want: "/|q=x=1|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "query_escape", uri: "/p?k=a b&z=%2F&k=z", want: "/|q=k=a+b&k=z&z=%2F|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "with_headers", uri: "/p?foo=bar", headers: map[string]string{"Accept": "text/html", "Accept-Encoding": "gzip"}, want: "/|q=foo=bar|h=accept:1|text/html|accept-encoding:1|gzip|accept-language:0"},
		{name: "with_cookie", uri: "/p?foo=bar", cookie: "sid=abc123", keyCooks: []string{"sid"}, want: "/|q=foo=bar|h=accept:0|accept-encoding:0|accept-language:0|c=sid:abc123"},
		{name: "long_query", uri: "/p?q=" + strings.Repeat("x", 300), want: "/|q=sha256:4d86f7dbfc8b3bfe229da7e27f4ac8f6cf8114e24e5cb6b5af1d09cb4cc3d982|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "long_path", uri: "/" + strings.Repeat("p", 300) + "?a=1", want: "/|q=a=1|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "disable_query", uri: "/p?foo=bar", noQuery: true, want: "/|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "header_val_delims", uri: "/p", headers: map[string]string{"Accept": "a|b:c"}, want: "/|q=|h=accept:1|a\\pb\\cc|accept-encoding:0|accept-language:0"},
		{name: "empty_query", uri: "/p?", want: "/|q=|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "many_params", uri: "/p?" + strings.Repeat("k=v&", 200) + "z=1", want: "/|q=sha256:f8f7166c8aec35092b4c6f66a895ec9f302746c6310aa0dbfde45cbd30aa1829|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "query_empty_body", uri: "/q", method: fiber.MethodQuery, want: "/|q=|h=accept:0|accept-encoding:0|accept-language:0|b="},
		{name: "query_body", uri: "/q", method: fiber.MethodQuery, body: "foo=bar", want: "/|q=|h=accept:0|accept-encoding:0|accept-language:0|b=foo=bar"},
		{name: "query_body_delims", uri: "/q", method: fiber.MethodQuery, body: "a|b:c", want: "/|q=|h=accept:0|accept-encoding:0|accept-language:0|b=a\\pb\\cc"},
		{name: "query_with_querystring", uri: "/q?x=1", method: fiber.MethodQuery, body: "foo=bar", want: "/|q=x=1|h=accept:0|accept-encoding:0|accept-language:0|b=foo=bar"},
	}
}

func buildKeygenCtx(tc *keygenCase) (fiber.Ctx, *Config) {
	app := fiber.New()
	// Method must be set before AcquireCtx (fiber caches it); URI/body/headers are
	// read live from the request afterwards.
	fctx := &fasthttp.RequestCtx{}
	method := tc.method
	if method == "" {
		method = fiber.MethodGet
	}
	fctx.Request.Header.SetMethod(method)
	c := app.AcquireCtx(fctx)
	c.Request().SetRequestURI(tc.uri)
	if tc.body != "" {
		c.Request().SetBody([]byte(tc.body))
	}
	for k, v := range tc.headers {
		c.Request().Header.Set(k, v)
	}
	if tc.cookie != "" {
		c.Request().Header.Set("Cookie", tc.cookie)
	}
	// Through configDefault, not ConfigDefault: New applies it to every config
	// it is given, and it lowercases the header dimensions. Keying off the raw
	// default instead pinned "h=accept:0", a key format no running application
	// produces — so a change to the normalization would not have been caught by
	// the fixtures that exist to catch exactly that.
	in := Config{DisableQueryKeys: tc.noQuery}
	if tc.keyCooks != nil {
		in.KeyCookies = tc.keyCooks
	}
	cfg := configDefault(in)
	return c, &cfg
}

func Benchmark_defaultKeyGenerator(b *testing.B) {
	cases := []struct{ name, uri string }{
		{"noquery", "/demo"},
		{"singleparam", "/demo?foo=bar"},
		{"multiparam", "/demo?foo=bar&baz=qux&alpha=1"},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			c, cfg := buildKeygenCtx(&keygenCase{uri: tc.uri})
			b.ReportAllocs()
			var s string
			for b.Loop() {
				s = defaultKeyGenerator(c, cfg)
			}
			_ = s
		})
	}
}

// Test_defaultKeyGenerator_stableKeys pins the exact cache-key output so the
// allocation refactor of the canonical* helpers stays byte-for-byte identical.
//
// The header dimension reads "name:<count>|value|value": the count is what
// keeps the framing injective now that every field line of a key header is
// included, and it also separates an absent header ("name:0") from one present
// with an empty value ("name:1|"). Changing this format is a deliberate act —
// live entries keyed by the old shape miss once — so update these strings only
// alongside a matching change in appendCanonicalHeaderSubset.
func Test_defaultKeyGenerator_stableKeys(t *testing.T) {
	t.Parallel()
	for _, tc := range keygenCorpus() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, cfg := buildKeygenCtx(&tc)
			require.Equal(t, tc.want, defaultKeyGenerator(c, cfg))
		})
	}
}

// Test_EscapeKeyDelimiters_AppendFormsMatch pins the append-based escapers
// against the Replacer they replaced.
//
// They exist to keep the escaped form out of the heap, not to change it: a
// segment that escapes differently is a different cache key, so an entry a
// previous build stored would stop being found. Every assertion here is byte
// equality with escapeKeyDelimiters.
func Test_EscapeKeyDelimiters_AppendFormsMatch(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", maxKeyDimensionSegmentLength+1)
	corpus := []string{
		"", "a", "|", ":", `\`, `\\`, "::", "||", `a|b:c\d`,
		"no-delimiters-here", "sha256:deadbeef", hashPrefix,
		`{"filter":"active","page":1}`,
		// Bounded verbatim before escaping, over the bound after it: the
		// expansion is what decides, so both forms have to agree on which.
		strings.Repeat(":", maxKeyDimensionSegmentLength/2),
		strings.Repeat(":", maxKeyDimensionSegmentLength),
		long, long + ":", strings.Repeat("|", 500),
	}
	// Every length either side of the bound, in both plain and delimiter-heavy
	// form, since the two paths part at that boundary.
	for n := maxKeyDimensionSegmentLength - 3; n <= maxKeyDimensionSegmentLength+3; n++ {
		corpus = append(corpus, strings.Repeat("x", n), strings.Repeat("x:", n/2), strings.Repeat("x", n-1)+"|")
	}

	for _, s := range corpus {
		escaped := escapeKeyDelimiters(s)

		require.Equal(t, len(escaped)-len(s), countKeyDelimiters(s), "delimiter count for %q", s)
		require.Equal(t, "seg="+escaped, string(appendEscapedKeyDelimiters([]byte("seg="), s)), "escape of %q", s)
		require.Equal(t,
			string(appendBoundKeySegment([]byte("seg="), escaped)),
			string(appendEscapedBoundKeySegment([]byte("seg="), s)),
			"bounded escape of %q", s)
	}
}

// Test_Cache_KeyFormatIsStable pins the bytes of an assembled cache key.
//
// The pieces are joined in one buffer now rather than concatenated one at a
// time. That is a change to how the key is built and must not be one to what
// the key is: a different format silently empties every cache that survived the
// deploy, and no other test in this suite would notice. The keys are read back
// off a storage that records what it was asked for, so this asserts on what the
// middleware actually used.
func Test_Cache_KeyFormatIsStable(t *testing.T) {
	t.Parallel()

	authHash := string(appendAuthHash(nil, [][]byte{[]byte("Bearer token")}))

	tests := []struct {
		name        string
		auth        string
		want        []string
		disableVary bool
	}{
		{
			name: "anonymous",
			want: []string{"v2|GET|/demo|vary", "v2|GET|/demo"},
		},
		{
			name: "authenticated",
			auth: "Bearer token",
			want: []string{"v2|GET|/demo|auth=" + authHash + "|vary", "v2|GET|/demo|auth=" + authHash},
		},
		{
			name:        "vary disabled",
			disableVary: true,
			want:        []string{"v2|GET|/demo"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := newKeyRecordingStorage()
			app := fiber.New()
			app.Use(New(Config{
				Storage:            store,
				DisableVaryHeaders: tc.disableVary,
				// A generator of the caller's own, to pin that its result lands
				// verbatim between the version and method partitions and the
				// suffixes this middleware adds.
				KeyGenerator: func(c fiber.Ctx) string { return c.Path() },
			}))
			app.Get("/demo", func(c fiber.Ctx) error { return c.SendString("ok") })

			req := httptest.NewRequest(fiber.MethodGet, "/demo", http.NoBody)
			if tc.auth != "" {
				req.Header.Set(fiber.HeaderAuthorization, tc.auth)
			}
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)

			// The body is stored under its own key, which is the entry key plus
			// the suffix the manager appends.
			require.Equal(t, tc.want, store.lookedUp())
		})
	}
}

// keyRecordingStorage reports every key it was asked to read, in order and
// without duplicates, so a test can assert on the keys the middleware built.
type keyRecordingStorage struct {
	data map[string][]byte
	seen []string
	mu   sync.Mutex
}

func newKeyRecordingStorage() *keyRecordingStorage {
	return &keyRecordingStorage{data: make(map[string][]byte)}
}

func (s *keyRecordingStorage) GetWithContext(_ context.Context, key string) ([]byte, error) {
	return s.Get(key)
}

func (s *keyRecordingStorage) Get(key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !slices.Contains(s.seen, key) {
		s.seen = append(s.seen, key)
	}
	return s.data[key], nil
}

func (s *keyRecordingStorage) SetWithContext(_ context.Context, key string, val []byte, exp time.Duration) error {
	return s.Set(key, val, exp)
}

func (s *keyRecordingStorage) Set(key string, val []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = val
	return nil
}

func (s *keyRecordingStorage) DeleteWithContext(_ context.Context, key string) error {
	return s.Delete(key)
}

func (s *keyRecordingStorage) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *keyRecordingStorage) ResetWithContext(context.Context) error { return s.Reset() }

func (s *keyRecordingStorage) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string][]byte)
	return nil
}

func (*keyRecordingStorage) Close() error { return nil }

func (s *keyRecordingStorage) lookedUp() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.seen)
}

// The escaping and bounding below are the reference implementations the key
// segments were built with before they were rewritten to append into the key
// buffer. Production no longer calls them — the append forms produce the same
// bytes without the intermediate string — so they live here, as the oracle the
// tests check those forms against.
//
// Test_EscapeKeyDelimiters_AppendFormsMatch is what ties the two together, and
// the security tests that pin what escaping and bounding must do read against
// these. Change one side and that test fails, which is the point.

// keyDelimiterEscaper escapes the delimiters in one pass: \ as \\, | as \p, : as \c.
var keyDelimiterEscaper = strings.NewReplacer(`\`, `\\`, `|`, `\p`, `:`, `\c`)

// escapeKeyDelimiters escapes pipe, colon, and backslash characters used as delimiters in cache keys
// to prevent injection attacks where crafted values could collide with different inputs
func escapeKeyDelimiters(s string) string {
	// Fast path: no characters to escape
	if utils.IndexAny3(s, '|', ':', '\\') == -1 {
		return s
	}
	return keyDelimiterEscaper.Replace(s)
}

func boundKeySegment(segment string) string {
	// Hash oversized segments, and also any segment that already starts with the
	// reserved hashPrefix, so a literal "sha256:..." value cannot collide with a
	// genuinely-hashed long segment (defense-in-depth alongside escapeKeyDelimiters).
	if len(segment) <= maxKeyDimensionSegmentLength && !strings.HasPrefix(segment, hashPrefix) {
		return segment
	}
	hash := sha256.Sum256(utils.UnsafeBytes(segment))
	return hashPrefix + hex.EncodeToString(hash[:])
}
