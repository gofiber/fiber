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
	cookie   string
	want     string
	keyCooks []string
	noQuery  bool
}

func keygenCorpus() []keygenCase {
	return []keygenCase{
		{name: "noquery", uri: "/demo", want: "/|q=|h=Accept:|Accept-Encoding:|Accept-Language:"},
		{name: "single", uri: "/demo?foo=bar", want: "/|q=foo=bar|h=Accept:|Accept-Encoding:|Accept-Language:"},
		{name: "multi_dup", uri: "/demo?b=2&a=1&a=3", want: "/|q=a=1&a=3&b=2|h=Accept:|Accept-Encoding:|Accept-Language:"},
		{name: "path_delims", uri: "/a|b:c\\d?x=1", want: "/|q=x=1|h=Accept:|Accept-Encoding:|Accept-Language:"},
		{name: "query_escape", uri: "/p?k=a b&z=%2F&k=z", want: "/|q=k=a+b&k=z&z=%2F|h=Accept:|Accept-Encoding:|Accept-Language:"},
		{name: "with_headers", uri: "/p?foo=bar", headers: map[string]string{"Accept": "text/html", "Accept-Encoding": "gzip"}, want: "/|q=foo=bar|h=Accept:text/html|Accept-Encoding:gzip|Accept-Language:"},
		{name: "with_cookie", uri: "/p?foo=bar", cookie: "sid=abc123", keyCooks: []string{"sid"}, want: "/|q=foo=bar|h=Accept:|Accept-Encoding:|Accept-Language:|c=sid:abc123"},
		{name: "long_query", uri: "/p?q=" + strings.Repeat("x", 300), want: "/|q=sha256:4d86f7dbfc8b3bfe229da7e27f4ac8f6cf8114e24e5cb6b5af1d09cb4cc3d982|h=Accept:|Accept-Encoding:|Accept-Language:"},
		{name: "long_path", uri: "/" + strings.Repeat("p", 300) + "?a=1", want: "/|q=a=1|h=Accept:|Accept-Encoding:|Accept-Language:"},
		{name: "disable_query", uri: "/p?foo=bar", noQuery: true, want: "/|h=Accept:|Accept-Encoding:|Accept-Language:"},
		{name: "header_val_delims", uri: "/p", headers: map[string]string{"Accept": "a|b:c"}, want: "/|q=|h=Accept:a\\pb\\cc|Accept-Encoding:|Accept-Language:"},
		{name: "empty_query", uri: "/p?", want: "/|q=|h=Accept:|Accept-Encoding:|Accept-Language:"},
		{name: "many_params", uri: "/p?" + strings.Repeat("k=v&", 200) + "z=1", want: "/|q=sha256:f8f7166c8aec35092b4c6f66a895ec9f302746c6310aa0dbfde45cbd30aa1829|h=Accept:|Accept-Encoding:|Accept-Language:"},
	}
}

func buildKeygenCtx(tc *keygenCase) (fiber.Ctx, *Config) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().SetRequestURI(tc.uri)
	c.Request().Header.SetMethod(fiber.MethodGet)
	for k, v := range tc.headers {
		c.Request().Header.Set(k, v)
	}
	if tc.cookie != "" {
		c.Request().Header.Set("Cookie", tc.cookie)
	}
	cfg := ConfigDefault
	cfg.DisableQueryKeys = tc.noQuery
	if tc.keyCooks != nil {
		cfg.KeyCookies = tc.keyCooks
	}
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
