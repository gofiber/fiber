package cache

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/storagetest"
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
		{name: "noquery", uri: "/", want: keyStart + keyNoHeaders},
		{name: "single", uri: "/?foo=bar", want: keyStart + "foo=bar" + keyNoHeaders},
		{name: "multi_dup", uri: "/?b=2&a=1&a=3", want: keyStart + "a=1&a=3&b=2" + keyNoHeaders},
		{name: "dup_order", uri: "/?a=3&a=1", want: keyStart + "a=3&a=1" + keyNoHeaders},
		{name: "path_delims", uri: "/a|b:c\\d?x=1", want: `scheme=http|host=example.com|/a\pb\cc\\d|q=x=1` + keyNoHeaders},
		{name: "query_escape", uri: "/?k=a b&z=%2F&k=z", want: keyStart + "k=a b&k=z&z=%2F" + keyNoHeaders},
		{name: "query_spellings", uri: "/?b=%41&a=x+y&a=x%20y&b=A", want: keyStart + "a=x+y&a=x%20y&b=%41&b=A" + keyNoHeaders},
		{name: "query_delims", uri: `/?b=x|y&a=1:2&c=\`, want: keyStart + `a=1\c2&b=x\py&c=\\` + keyNoHeaders},
		// The names are sorted as the parser decodes them, so two spellings of one name stay in the order sent.
		{name: "query_name_spellings", uri: "/?b=1&%61=2&a=3&%62=4", want: keyStart + "%61=2&a=3&b=1&%62=4" + keyNoHeaders},
		{name: "query_name_plus", uri: "/?b=1&+=2&%20=3&a=4", want: keyStart + "+=2&%20=3&a=4&b=1" + keyNoHeaders},
		{name: "query_bare_parameters", uri: "/?b&&a=&=x&", want: keyStart + "&=x&&a=&b" + keyNoHeaders},
		{name: "long_query_sorted", uri: "/?b=" + strings.Repeat("x", 300) + "&a=1", want: keyStart + boundKeySegment("a=1&b="+strings.Repeat("x", 300)) + keyNoHeaders},
		// maxQueryParams parameters in maxQueryBufferSize bytes are still sorted; a query past either bound is keyed as sent.
		{name: "params_at_bound", uri: "/?" + strings.Repeat("b&", maxQueryParams-1) + "a", want: keyStart + boundKeySegment("a"+strings.Repeat("&b", maxQueryParams-1)) + keyNoHeaders},
		{name: "params_past_bound", uri: "/?" + strings.Repeat("b&", maxQueryParams) + "a", want: keyStart + boundKeySegment(strings.Repeat("b&", maxQueryParams)+"a") + keyNoHeaders},
		{name: "query_at_bound", uri: "/?b=" + strings.Repeat("x", maxQueryBufferSize-6) + "&a=1", want: keyStart + boundKeySegment("a=1&b="+strings.Repeat("x", maxQueryBufferSize-6)) + keyNoHeaders},
		{name: "query_past_bound", uri: "/?b=" + strings.Repeat("x", maxQueryBufferSize-5) + "&a=1", want: keyStart + boundKeySegment("b="+strings.Repeat("x", maxQueryBufferSize-5)+"&a=1") + keyNoHeaders},
		{name: "with_headers", uri: "/?foo=bar", headers: map[string]string{"Accept": "text/html", "Accept-Encoding": "gzip"}, want: keyStart + "foo=bar|h=accept:1|text/html|accept-encoding:1|gzip|accept-language:0"},
		{name: "with_cookie", uri: "/?foo=bar", cookie: "sid=abc123", keyCooks: []string{"sid"}, want: keyStart + "foo=bar" + keyNoHeaders + "|c=sid:sha256:ccfb62d77d32969b64f7920fe3bfb66c50cbf9e9faea6f69222c34c7e5f30627"},
		// A missing and an empty cookie leave nothing after the name, and share a key.
		{name: "cookie_empty", uri: "/", cookie: "sid=", keyCooks: []string{"sid"}, want: keyStart + keyNoHeaders + "|c=sid:"},
		{name: "cookie_absent", uri: "/", keyCooks: []string{"sid"}, want: keyStart + keyNoHeaders + "|c=sid:"},
		{name: "cookie_delims", uri: "/", cookie: "sid=a|b:c", keyCooks: []string{"sid"}, want: keyStart + keyNoHeaders + "|c=sid:sha256:166f86ddd38c71229af5fa7dfacf4c0bd335848667481d07c6cbd2aabc727955"},
		{name: "cookie_long", uri: "/", cookie: "sid=" + strings.Repeat("x", 300), keyCooks: []string{"sid"}, want: keyStart + keyNoHeaders + "|c=sid:sha256:95e98730d96605ac7a50fb8dfcd3e46bea8b14b5450f5ecb4fea105fa8564ac0"},
		{name: "cookie_several", uri: "/", cookie: "a=1; b=2", keyCooks: []string{"a", "b", "c"}, want: keyStart + keyNoHeaders + "|c=a:sha256:f16ba6fa61da3398815be2a6c0f7cb1351982dbcc6c64bbeb9b65f672a8b102a|b:sha256:7f774bb46e7e342a2d9d0514b27cee622012f7415645345054f8cd31e1dda661|c:"},
		{name: "long_query", uri: "/?q=" + strings.Repeat("x", 300), want: keyStart + "sha256:4d86f7dbfc8b3bfe229da7e27f4ac8f6cf8114e24e5cb6b5af1d09cb4cc3d982" + keyNoHeaders},
		{name: "long_path", uri: "/" + strings.Repeat("p", 300) + "?a=1", want: "scheme=http|host=example.com|" + boundKeySegment("/"+strings.Repeat("p", 300)) + "|q=a=1" + keyNoHeaders},
		{name: "disable_query", uri: "/?foo=bar", noQuery: true, want: "scheme=http|host=example.com|/" + keyNoHeaders},
		{name: "header_val_delims", uri: "/", headers: map[string]string{"Accept": "a|b:c"}, want: keyStart + "|h=accept:1|a\\pb\\cc|accept-encoding:0|accept-language:0"},
		// The raw target where it differs from the routed path, an empty "?" included, and a fragment.
		{name: "empty_query", uri: "/?", want: keyStart + "|p=/?" + keyNoHeaders},
		{name: "target_fragment", uri: "/?a=1#top", want: keyStart + "a=1|f=top" + keyNoHeaders},
		{name: "target_empty_fragment", uri: "/#", want: keyStart + "|f=" + keyNoHeaders},
		{name: "target_query_in_fragment", uri: "/#a?b=1", want: keyStart + "|f=a?b=1" + keyNoHeaders},
		{name: "target_empty_query_and_fragment", uri: "/?#top", want: keyStart + "|p=/?|f=top" + keyNoHeaders},
		{name: "target_dot_segments", uri: "/a/../?x=1", want: keyStart + "x=1|p=/a/../" + keyNoHeaders},
		{name: "target_escaped_letter", uri: "/%70age", want: "scheme=http|host=example.com|/page|q=|p=/%70age" + keyNoHeaders},
		{name: "target_absolute_form", uri: "http://example.com/?x=1", want: keyStart + `x=1|p=http\c//example.com/` + keyNoHeaders},
		{name: "target_delims_and_bound", uri: "/a|b/../#" + strings.Repeat("f:", 100), want: keyStart + `|p=/a\pb/../|f=` + boundKeySegment(strings.Repeat(`f\c`, 100)) + keyNoHeaders},
		{name: "disable_query_target", uri: "/a/../?foo=bar#top", noQuery: true, want: "scheme=http|host=example.com|/|p=/a/../|f=top" + keyNoHeaders},
		{name: "disable_query_empty_query", uri: "/?", noQuery: true, want: "scheme=http|host=example.com|/" + keyNoHeaders},
		{name: "many_params", uri: "/?" + strings.Repeat("k=v&", 200) + "z=1", want: keyStart + "sha256:f8f7166c8aec35092b4c6f66a895ec9f302746c6310aa0dbfde45cbd30aa1829" + keyNoHeaders},
		{name: "query_empty_body", uri: "/", method: fiber.MethodQuery, want: keyStart + keyNoHeaders + "|m=0|0|b="},
		{name: "query_body", uri: "/", method: fiber.MethodQuery, body: "foo=bar", want: keyStart + keyNoHeaders + "|m=0|0|b=foo=bar"},
		{name: "query_body_delims", uri: "/", method: fiber.MethodQuery, body: "a|b:c", want: keyStart + keyNoHeaders + "|m=0|0|b=a\\pb\\cc"},
		{name: "query_with_querystring", uri: "/?x=1", method: fiber.MethodQuery, body: "foo=bar", want: keyStart + "x=1" + keyNoHeaders + "|m=0|0|b=foo=bar"},
		{name: "query_body_metadata", uri: "/", method: fiber.MethodQuery, body: "foo=bar", headers: map[string]string{"Content-Type": "application/json", "Content-Encoding": "gzip"}, want: keyStart + keyNoHeaders + "|m=1|gzip|1|application/json|b=foo=bar"},
		{name: "post_body", uri: "/", method: fiber.MethodPost, body: "foo=bar", want: keyStart + keyNoHeaders + "|m=0|0|b=foo=bar"},
		{name: "post_body_metadata", uri: "/", method: fiber.MethodPost, body: "{}", headers: map[string]string{"Content-Type": "application/json"}, want: keyStart + keyNoHeaders + "|m=0|1|application/json|b={}"},
		{name: "post_no_body", uri: "/", method: fiber.MethodPost, want: keyStart + keyNoHeaders},
		{name: "get_body", uri: "/", body: "foo=bar", want: keyStart + keyNoHeaders + "|m=0|0|b=foo=bar"},
		{name: "get_encoding_no_body", uri: "/", headers: map[string]string{"Content-Encoding": "gzip"}, want: keyStart + keyNoHeaders + "|m=1|gzip|0|b="},
		{name: "get_encoding_empty_no_body", uri: "/", headers: map[string]string{"Content-Encoding": ""}, want: keyStart + keyNoHeaders + "|m=1||0|b="},
		// The authority: the port is part of it, its delimiters are escaped like
		// every other segment's, and past the bound it is hashed rather than
		// allowed to grow the key.
		{name: "host_port", uri: "/", host: "example.com:8443", want: "scheme=http|host=example.com\\c8443|/|q=" + keyNoHeaders},
		{name: "host_delims", uri: "/", host: "a|b:c\\d", want: "scheme=http|host=a\\pb\\cc\\\\d|/|q=" + keyNoHeaders},
		{name: "long_host", uri: "/", host: strings.Repeat("h", 300), want: "scheme=http|host=" + boundKeySegment(strings.Repeat("h", 300)) + "|/|q=" + keyNoHeaders},
	}
}

// buildKeygenCtx returns tc's request context, released when the test ends, and its config as New fills it.
func buildKeygenCtx(tb testing.TB, tc *keygenCase) (fiber.Ctx, *Config) {
	tb.Helper()

	app := fiber.New()
	// The request line is set before AcquireCtx, which caches the method and path.
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(cmp.Or(tc.method, fiber.MethodGet))
	fctx.Request.SetRequestURI(tc.uri)
	fctx.Request.SetHost(cmp.Or(tc.host, "example.com"))
	c := apptest.CtxFor(tb, app, fctx)
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
		{"noquery", "/"},
		{"singleparam", "/?foo=bar"},
		{"multiparam", "/?foo=bar&baz=qux&alpha=1"},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			c, cfg := buildKeygenCtx(b, &keygenCase{uri: tc.uri})
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
			c, cfg := buildKeygenCtx(t, &tc)
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
		{name: "anonymous", want: []string{"v8|manifest|v8|GET|/demo", "v8|GET|/demo"}},
		{
			name: "authenticated",
			auth: "Bearer token",
			want: []string{"v8|manifest|v8|GET|/demo|auth=" + authHash, "v8|GET|/demo|auth=" + authHash},
		},
		{name: "vary disabled", disableVary: true, want: []string{"v8|GET|/demo"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Every key the middleware reads, in order and without duplicates.
			var mu sync.Mutex
			var lookedUp []string
			store := &hookedStorage{Memory: storagetest.New(), onGet: func(key string) error {
				mu.Lock()
				defer mu.Unlock()
				if !slices.Contains(lookedUp, key) {
					lookedUp = append(lookedUp, key)
				}
				return nil
			}}
			app := fiber.New()
			app.Use(New(Config{
				Storage:            store,
				DisableVaryHeaders: tc.disableVary,
				// A generator of the caller's own, to pin that its result lands
				// verbatim between the version and method partitions and the
				// suffixes this middleware adds.
				KeyGenerator: pathKey,
			}))
			app.Get("/demo", func(c fiber.Ctx) error { return c.SendString("ok") })

			req := apptest.Req(fiber.MethodGet, "/demo", apptest.WithHeaderIf(fiber.HeaderAuthorization, tc.auth))
			apptest.Expect(t, app, req, fiber.StatusOK)

			// The body is stored under its own key, which is the entry key plus
			// the suffix the manager appends.
			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, tc.want, lookedUp)
		})
	}
}

func Test_defaultKeyGenerator_KeysTheTargetAsSent(t *testing.T) {
	t.Parallel()
	paths := []string{"/page", "/pa%67e", "/x/../page", "/./page", "http://example.com/page"}
	queries := []string{"", "?", "?a=1", "?a=1&b=2", "?b=2&a=1"}
	fragments := []string{"", "#", "#top", "#a?b=1"}

	for _, tc := range []struct {
		name    string
		keys    int
		noQuery bool
	}{
		// The last two queries are one to the key, and no other two targets are.
		{name: "query keyed", keys: len(paths) * (len(queries) - 1) * len(fragments)},
		{name: "query left out", noQuery: true, keys: len(paths) * len(fragments)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			keys := map[string]struct{}{}
			for _, path := range paths {
				for _, query := range queries {
					for _, fragment := range fragments {
						c, cfg := buildKeygenCtx(t, &keygenCase{uri: path + query + fragment, noQuery: tc.noQuery})
						require.Equal(t, "/page", c.Path(), path+query+fragment)
						keys[defaultKeyGenerator(c, cfg)] = struct{}{}
					}
				}
			}
			require.Len(t, keys, tc.keys)
		})
	}
}

func Test_appendTargetAsSent_RewrittenQuery(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, target, path, keyed, want string
		noQuery                         bool
	}{
		{name: "sent as keyed", target: "/page?a=1", path: "/page", keyed: "a=1"},
		{name: "added", target: "/page", path: "/page", keyed: "a=1", want: "|n=/page"},
		{name: "added to a rewritten path", target: "/old", path: "/page", keyed: "a=1", want: "|n=/old"},
		{name: "added before a fragment", target: "/page#top", path: "/page", keyed: "a=1", want: "|n=/page|f=top"},
		{name: "added behind a bare question mark", target: "/page?", path: "/page", keyed: "a=1", want: "|p=/page?"},
		{name: "rewritten", target: "/page?b=2", path: "/page", keyed: "a=1", want: "|p=/page?b=2"},
		{name: "removed", target: "/page?b=2", path: "/page", want: "|p=/page?b=2"},
		{name: "added, left out", target: "/page", path: "/page", keyed: "a=1", noQuery: true},
		{name: "added, moved, left out", target: "/old", path: "/page", keyed: "a=1", noQuery: true, want: "|p=/old"},
		{name: "rewritten, left out", target: "/page?b=2", path: "/page", keyed: "a=1", noQuery: true},
		{name: "removed, left out", target: "/page?b=2", path: "/page", noQuery: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{DisableQueryKeys: tc.noQuery}
			got := appendTargetAsSent(nil, []byte(tc.target), tc.path, []byte(tc.keyed), cfg)
			require.Equal(t, tc.want, string(got))
		})
	}
}

func Test_appendCanonicalQueryString_SharedKeyReadsTheSame(t *testing.T) {
	t.Parallel()
	parameters := []string{
		"a=1", "a=2", "%61=3", "A=4", "a=b=5", "b", "b=", "+=6", "%20=7", "x%+=8", "x%%2B=9",
		"%=10", "%zz=11", "", "=", "=12", "c%3D=13",
	}
	key := func(query string) string {
		var uri fasthttp.URI
		uri.SetQueryString(query)
		return string(appendCanonicalQueryString(nil, &uri))
	}
	read := func(query string) string {
		var args fasthttp.Args
		args.Parse(query)
		var names []string
		for name := range args.All() {
			if !slices.Contains(names, string(name)) {
				names = append(names, string(name))
			}
		}
		slices.Sort(names)
		var values strings.Builder
		for _, name := range names {
			fmt.Fprintf(&values, "%q=%q;", name, args.PeekMulti(name))
		}
		return values.String()
	}

	sharing := map[string][]string{}
	for i, first := range parameters {
		for j, second := range parameters {
			for k, third := range parameters {
				if i != j && i != k && j != k {
					query := first + "&" + second + "&" + third
					sharing[key(query)] = append(sharing[key(query)], query)
				}
			}
		}
	}

	shared := 0
	for _, queries := range sharing {
		for _, query := range queries[1:] {
			shared++
			require.ElementsMatch(t, strings.Split(queries[0], "&"), strings.Split(query, "&"), "%q and %q", queries[0], query)
			require.Equal(t, read(queries[0]), read(query), "%q and %q", queries[0], query)
		}
	}
	require.NotZero(t, shared, "parameters of different names share a key in either order")
}

func Test_appendCanonicalQueryString_KeepsTheOrderOfOneName(t *testing.T) {
	t.Parallel()
	var sent []string
	byName := map[string][]string{}
	for i, value := range "0123456789abcdefghijklmnopqrstuvwxyz" {
		name := string("cab"[i%3])
		sent = append(sent, name+"="+string(value))
		byName[name] = append(byName[name], name+"="+string(value))
	}

	var uri fasthttp.URI
	uri.SetQueryString(strings.Join(sent, "&"))
	want := strings.Join(slices.Concat(byName["a"], byName["b"], byName["c"]), "&")
	require.Equal(t, want, string(appendCanonicalQueryString(nil, &uri)))
}

// Sequential: testing.AllocsPerRun panics in a parallel test. The fewest of many runs counts under -race.
func Test_appendCanonicalQueryString_DoesNotAllocate(t *testing.T) {
	var uri fasthttp.URI
	uri.SetQueryString("q=hello+world&filter%5Bstatus%5D=active&page=2&sort=-created")
	dst := make([]byte, 0, 512)

	fewest := math.MaxFloat64
	for range 200 {
		fewest = min(fewest, testing.AllocsPerRun(1, func() { dst = appendCanonicalQueryString(dst[:0], &uri) }))
	}
	require.Zero(t, fewest)
	require.Equal(t, "filter%5Bstatus%5D=active&page=2&q=hello+world&sort=-created", string(dst))
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
		c, _ := buildKeygenCtx(t, &keygenCase{uri: "/", cookie: cookie})
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
	withCookie := func(value string) []string { return []string{"Cookie", "session=" + value} }

	for _, kind := range storageKinds {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			storage := storageOf(kind)
			app := fiber.New()
			app.Use(New(Config{Storage: storage, KeyCookies: []string{"session"}}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("for:" + c.Cookies("session")) })

			cacheTestCRequireReply(t, cacheTestCGet(t, app, "/", withCookie(session)...), cacheMiss, "for:"+session)
			require.Equal(t, cacheHit, cacheTestCGet(t, app, "/", withCookie(session)...).cache)

			cacheTestCRequireReply(t, cacheTestCGet(t, app, "/", withCookie(other)...), cacheMiss, "for:"+other)

			require.Equal(t, cacheMiss, cacheTestCGet(t, app, "/").cache)
			require.Equal(t, cacheHit, cacheTestCGet(t, app, "/", withCookie("")...).cache, "an empty cookie is the partition of none")

			external, ok := storage.(*storagetest.Memory)
			if !ok {
				return
			}
			external.Mu.RLock()
			defer external.Mu.RUnlock()
			hashed := 0
			for key := range external.Data {
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
	c, _ := buildKeygenCtx(t, &keygenCase{uri: "/", cookie: "sid=sess_0123456789abcdefghijklmnopqrstuvwxyz"})
	dst := make([]byte, 0, 512)

	cfg := &Config{KeyCookies: []string{"sid"}}
	require.Zero(t, testing.AllocsPerRun(100, func() { dst = appendCanonicalCookieSubset(dst[:0], c, cfg) }))
}
