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
	host     string
	method   string
	body     string
	cookie   string
	want     string
	keyCooks []string
	noQuery  bool
}

func keygenCorpus() []keygenCase {
	return []keygenCase{
		{name: "noquery", uri: "/demo", want: "scheme=http|host=example.com|/|q=|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "single", uri: "/demo?foo=bar", want: "scheme=http|host=example.com|/|q=foo=bar|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "multi_dup", uri: "/demo?b=2&a=1&a=3", want: "scheme=http|host=example.com|/|q=a=1&a=3&b=2|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "dup_order", uri: "/demo?a=3&a=1", want: "scheme=http|host=example.com|/|q=a=3&a=1|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "path_delims", uri: "/a|b:c\\d?x=1", want: "scheme=http|host=example.com|/|q=x=1|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "query_escape", uri: "/p?k=a b&z=%2F&k=z", want: "scheme=http|host=example.com|/|q=k=a+b&k=z&z=%2F|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "with_headers", uri: "/p?foo=bar", headers: map[string]string{"Accept": "text/html", "Accept-Encoding": "gzip"}, want: "scheme=http|host=example.com|/|q=foo=bar|h=accept:1|text/html|accept-encoding:1|gzip|accept-language:0"},
		{name: "with_cookie", uri: "/p?foo=bar", cookie: "sid=abc123", keyCooks: []string{"sid"}, want: "scheme=http|host=example.com|/|q=foo=bar|h=accept:0|accept-encoding:0|accept-language:0|c=sid:sha256:ccfb62d77d32969b64f7920fe3bfb66c50cbf9e9faea6f69222c34c7e5f30627"},
		// A missing and an empty cookie leave nothing after the name, and share a key.
		{name: "cookie_empty", uri: "/p", cookie: "sid=", keyCooks: []string{"sid"}, want: "scheme=http|host=example.com|/|q=|h=accept:0|accept-encoding:0|accept-language:0|c=sid:"},
		{name: "cookie_absent", uri: "/p", keyCooks: []string{"sid"}, want: "scheme=http|host=example.com|/|q=|h=accept:0|accept-encoding:0|accept-language:0|c=sid:"},
		{name: "cookie_delims", uri: "/p", cookie: "sid=a|b:c", keyCooks: []string{"sid"}, want: "scheme=http|host=example.com|/|q=|h=accept:0|accept-encoding:0|accept-language:0|c=sid:sha256:166f86ddd38c71229af5fa7dfacf4c0bd335848667481d07c6cbd2aabc727955"},
		{name: "cookie_long", uri: "/p", cookie: "sid=" + strings.Repeat("x", 300), keyCooks: []string{"sid"}, want: "scheme=http|host=example.com|/|q=|h=accept:0|accept-encoding:0|accept-language:0|c=sid:sha256:95e98730d96605ac7a50fb8dfcd3e46bea8b14b5450f5ecb4fea105fa8564ac0"},
		{name: "cookie_several", uri: "/p", cookie: "a=1; b=2", keyCooks: []string{"a", "b", "c"}, want: "scheme=http|host=example.com|/|q=|h=accept:0|accept-encoding:0|accept-language:0|c=a:sha256:f16ba6fa61da3398815be2a6c0f7cb1351982dbcc6c64bbeb9b65f672a8b102a|b:sha256:7f774bb46e7e342a2d9d0514b27cee622012f7415645345054f8cd31e1dda661|c:"},
		{name: "long_query", uri: "/p?q=" + strings.Repeat("x", 300), want: "scheme=http|host=example.com|/|q=sha256:4d86f7dbfc8b3bfe229da7e27f4ac8f6cf8114e24e5cb6b5af1d09cb4cc3d982|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "long_path", uri: "/" + strings.Repeat("p", 300) + "?a=1", want: "scheme=http|host=example.com|/|q=a=1|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "disable_query", uri: "/p?foo=bar", noQuery: true, want: "scheme=http|host=example.com|/|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "header_val_delims", uri: "/p", headers: map[string]string{"Accept": "a|b:c"}, want: "scheme=http|host=example.com|/|q=|h=accept:1|a\\pb\\cc|accept-encoding:0|accept-language:0"},
		{name: "empty_query", uri: "/p?", want: "scheme=http|host=example.com|/|q=|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "many_params", uri: "/p?" + strings.Repeat("k=v&", 200) + "z=1", want: "scheme=http|host=example.com|/|q=sha256:f8f7166c8aec35092b4c6f66a895ec9f302746c6310aa0dbfde45cbd30aa1829|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "query_empty_body", uri: "/q", method: fiber.MethodQuery, want: "scheme=http|host=example.com|/|q=|h=accept:0|accept-encoding:0|accept-language:0|m=0|0|b="},
		{name: "query_body", uri: "/q", method: fiber.MethodQuery, body: "foo=bar", want: "scheme=http|host=example.com|/|q=|h=accept:0|accept-encoding:0|accept-language:0|m=0|0|b=foo=bar"},
		{name: "query_body_delims", uri: "/q", method: fiber.MethodQuery, body: "a|b:c", want: "scheme=http|host=example.com|/|q=|h=accept:0|accept-encoding:0|accept-language:0|m=0|0|b=a\\pb\\cc"},
		{name: "query_with_querystring", uri: "/q?x=1", method: fiber.MethodQuery, body: "foo=bar", want: "scheme=http|host=example.com|/|q=x=1|h=accept:0|accept-encoding:0|accept-language:0|m=0|0|b=foo=bar"},
		{name: "query_body_metadata", uri: "/q", method: fiber.MethodQuery, body: "foo=bar", headers: map[string]string{"Content-Type": "application/json", "Content-Encoding": "gzip"}, want: "scheme=http|host=example.com|/|q=|h=accept:0|accept-encoding:0|accept-language:0|m=1|gzip|1|application/json|b=foo=bar"},
		// The authority: the port is part of it, its delimiters are escaped like
		// every other segment's, and past the bound it is hashed rather than
		// allowed to grow the key.
		{name: "host_port", uri: "/p", host: "example.com:8443", want: "scheme=http|host=example.com\\c8443|/|q=|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "host_delims", uri: "/p", host: "a|b:c\\d", want: "scheme=http|host=a\\pb\\cc\\\\d|/|q=|h=accept:0|accept-encoding:0|accept-language:0"},
		{name: "long_host", uri: "/p", host: strings.Repeat("h", 300), want: "scheme=http|host=" + boundKeySegment(strings.Repeat("h", 300)) + "|/|q=|h=accept:0|accept-encoding:0|accept-language:0"},
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
	// The authority is read live as well; a request without one is not what
	// the key is built for, so the fixtures carry the host httptest would set.
	host := tc.host
	if host == "" {
		host = "example.com"
	}
	c.Request().SetHost(host)
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
// alongside a matching change in appendCanonicalHeaderSubset. A cookie reads
// "name:sha256:<digest>", or "name:" with no value (appendCanonicalCookieSubset).
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

	authHash := authDigest("Bearer token")

	tests := []struct {
		name        string
		auth        string
		want        []string
		disableVary bool
	}{
		{
			name: "anonymous",
			want: []string{"v5|GET|/demo|vary", "v5|GET|/demo"},
		},
		{
			name: "authenticated",
			auth: "Bearer token",
			want: []string{"v5|GET|/demo|auth=" + authHash + "|vary", "v5|GET|/demo|auth=" + authHash},
		},
		{
			name:        "vary disabled",
			disableVary: true,
			want:        []string{"v5|GET|/demo"},
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

// Hashed and kept segments cannot be mistaken for one another, so a value spelled like a digest
// cannot borrow the partition it imitates.
func Test_appendCanonicalHeaderSubset_HashedAndVerbatimSegmentsDiffer(t *testing.T) {
	t.Parallel()

	segment := func(credentials []string, lines ...string) string {
		var h fasthttp.RequestHeader
		for _, line := range lines {
			h.Add("X-Api-Key", line)
		}
		return string(appendCanonicalHeaderSubset(nil, &h, []string{"x-api-key"}, credentials, true))
	}
	credential := []string{"x-api-key"}

	t.Run("a credential is hashed and not spelled out", func(t *testing.T) {
		t.Parallel()

		got := segment(credential, "tok")
		require.Equal(t, "x-api-key:1sha256:"+authDigest("tok"), got)
		require.NotContains(t, got, "tok")
	})

	t.Run("an absent credential keeps the key of a client that sends none", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, "x-api-key:0", segment(credential))
		require.Equal(t, segment(nil), segment(credential))
	})

	t.Run("a value spelled like the digest is kept, not mistaken for the credential", func(t *testing.T) {
		t.Parallel()

		lookalike := "sha256:" + authDigest("tok")
		got := segment(nil, lookalike)
		require.NotEqual(t, segment(credential, "tok"), got)
		require.True(t, strings.HasPrefix(got, "x-api-key:1|"), "a kept value follows the separator: %q", got)
		// Hashed, the lookalike is just another credential.
		require.NotEqual(t, segment(credential, "tok"), segment(credential, lookalike))
	})

	t.Run("lines too long to keep hash like a credential of the same lines", func(t *testing.T) {
		t.Parallel()

		long := strings.Repeat("x", 300)
		want := "x-api-key:1sha256:" + authDigest(long)
		require.Equal(t, want, segment(nil, long))
		require.Equal(t, want, segment(credential, long))
	})

	t.Run("a value too long to keep and a value spelled like its digest key apart", func(t *testing.T) {
		t.Parallel()

		long := strings.Repeat("x", 300)
		hashed := segment(nil, long)
		lookalike := segment(nil, "sha256:"+authDigest(long))
		require.NotEqual(t, hashed, lookalike)
		// A kept value follows the separator, with its colon escaped, and a hashed one does not, so no
		// value can imitate a digest.
		require.True(t, strings.HasPrefix(hashed, "x-api-key:1sha256:"), "%q", hashed)
		require.True(t, strings.HasPrefix(lookalike, `x-api-key:1|sha256\c`), "%q", lookalike)
	})

	t.Run("the split into lines is part of the digest", func(t *testing.T) {
		t.Parallel()

		require.NotEqual(t, segment(credential, "ab"), segment(credential, "a", "b"))
		require.NotEqual(t, segment(credential, "a", "bc"), segment(credential, "ab", "c"))
		require.NotEqual(t, segment(credential, ""), segment(credential), "present but empty differs from absent")
	})
}

// A cookie's value is most often a session identifier, so it is hashed in the key; a missing or empty one leaves its bare name.
func Test_appendCanonicalCookieSubset(t *testing.T) {
	t.Parallel()

	segment := func(cookie string, names ...string) string {
		c, _ := buildKeygenCtx(&keygenCase{uri: "/", cookie: cookie})
		return string(appendCanonicalCookieSubset(nil, c, &Config{KeyCookies: names}))
	}

	t.Run("a value is hashed and not spelled out", func(t *testing.T) {
		t.Parallel()

		got := segment("sid=tok", "sid")
		require.Equal(t, "sid:sha256:"+authDigest("tok"), got)
		require.NotContains(t, got, "tok")
	})

	t.Run("a missing or empty cookie shares one key, a value does not", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, "sid:", segment("", "sid"))
		require.Equal(t, "sid:", segment("sid=", "sid"))
		require.Equal(t, "sid:", segment("other=1", "sid"))
		require.NotEqual(t, "sid:", segment("sid=0", "sid"))
	})

	t.Run("a value shaped like the key's structure is hashed like any other", func(t *testing.T) {
		t.Parallel()

		for _, value := range []string{"a|b:c", "x|c=other:1", "sha256:" + authDigest("tok"), strings.Repeat("x", 300)} {
			got := segment("sid="+value, "sid")
			require.Equal(t, "sid:sha256:"+authDigest(value), got)
			require.Regexp(t, `^sid:sha256:[0-9a-f]{64}$`, got)
		}
		// Imitating the digest of another value does not borrow that value's partition.
		require.NotEqual(t, segment("sid=tok", "sid"), segment("sid=sha256:"+authDigest("tok"), "sid"))
	})

	t.Run("each name keeps its own segment", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, "a:sha256:"+authDigest("1")+"|b:sha256:"+authDigest("2")+"|c:", segment("a=1; b=2", "a", "b", "c"))
		require.NotEqual(t, segment("a=1; b=2", "a", "b"), segment("a=2; b=1", "a", "b"))
	})
}

func Test_Cache_KeyCookieValuesAreHashedInStorageKeys(t *testing.T) {
	t.Parallel()

	const (
		session = "sess_0123456789abcdefghijklmnopqrstuvwxyz"
		other   = "sess_other_other_other_other_other_other"
	)
	withCookie := func(value string) map[string]string { return map[string]string{"Cookie": "session=" + value} }

	for _, kind := range storageKinds {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			storage := storageOf(kind)
			app := fiber.New()
			app.Use(New(Config{Storage: storage, KeyCookies: []string{"session"}}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("for:" + c.Cookies("session")) })

			first := getRoot(t, app, withCookie(session))
			require.Equal(t, cacheMiss, first.cache)
			require.Equal(t, "for:"+session, first.body)
			require.Equal(t, cacheHit, getRoot(t, app, withCookie(session)).cache)

			second := getRoot(t, app, withCookie(other))
			require.Equal(t, cacheMiss, second.cache)
			require.Equal(t, "for:"+other, second.body)

			require.Equal(t, cacheMiss, getRoot(t, app, nil).cache)
			require.Equal(t, cacheHit, getRoot(t, app, withCookie("")).cache, "an empty cookie is the partition of none")

			external, ok := storage.(*failingCacheStorage)
			if !ok {
				return
			}
			external.mu.RLock()
			defer external.mu.RUnlock()
			hashed := 0
			for key := range external.data {
				require.NotContains(t, key, session)
				require.NotContains(t, key, other)
				if strings.Contains(key, "session:sha256:"+authDigest(session)) {
					hashed++
				}
			}
			require.NotZero(t, hashed, "the entry of the first session is keyed by the digest of its cookie")
		})
	}
}

// Hashing a cookie value is on the hit path, so it must not allocate: the digest goes into the key
// buffer. Sequential: testing.AllocsPerRun panics during a parallel test.
func Test_appendCanonicalCookieSubset_DoesNotAllocate(t *testing.T) {
	c, _ := buildKeygenCtx(&keygenCase{uri: "/", cookie: "sid=sess_0123456789abcdefghijklmnopqrstuvwxyz"})
	dst := make([]byte, 0, 512)

	cfg := &Config{KeyCookies: []string{"sid"}}
	require.Zero(t, testing.AllocsPerRun(100, func() { dst = appendCanonicalCookieSubset(dst[:0], c, cfg) }))
}
