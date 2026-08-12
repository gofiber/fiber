package cache

import (
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
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
