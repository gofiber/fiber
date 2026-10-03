package fiber

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Test_Route_URL_ParameterRepresentability checks rejection through all URL
// entry points and verifies that accepted values round-trip through routing.
func Test_Route_URL_ParameterRepresentability(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, pattern, key, value, want string
		unescape, reject                bool
	}{
		{name: "plain dot", pattern: "/user/:value", key: "value", value: ".", reject: true},
		{name: "plain parent", pattern: "/user/:value", key: "value", value: "..", reject: true},
		{name: "greedy parent", pattern: "/files/*", key: "*", value: "a/../b", reject: true},
		{name: "greedy trailing dot", pattern: "/files/*", key: "*", value: "a/.", reject: true},
		{name: "dot completed by prefix", pattern: "/p/.:value", key: "value", value: ".", reject: true},
		{name: "dot completed by suffix", pattern: "/p/:value.", key: "value", value: ".", reject: true},
		{name: "dot with extension", pattern: "/p/:value.txt", key: "value", value: ".", want: "/p/..txt"},
		{name: "dotted value after a constant dot", pattern: "/p/.:value", key: "value", value: "a.b", want: "/p/.a.b"},
		{name: "parent after a constant dot", pattern: "/p/.:value", key: "value", value: "..", want: "/p/..."},
		{name: "greedy dot after a constant dot", pattern: "/p/.*", key: "*", value: "./x", reject: true},
		{name: "hidden file", pattern: "/files/*", key: "*", value: ".config/name.txt", want: "/files/.config/name.txt"},
		{name: "three dots", pattern: "/user/:value", key: "value", value: "...", want: "/user/..."},
		{name: "encoded slash remains data", pattern: "/user/:value", key: "value", value: "a/b", want: "/user/a%2Fb"},
		{name: "decoded plain slash", pattern: "/user/:value", key: "value", value: "a/b", unescape: true, reject: true},
		{name: "decoded slash before constant", pattern: "/p/:value/end", key: "value", value: "a/b", unescape: true, reject: true},
		{name: "decoded greedy slash", pattern: "/files/*", key: "*", value: "a/b", unescape: true, want: "/files/a/b"},
		{name: "decoded plus slash", pattern: "/files/+", key: "+", value: "a/b", unescape: true, want: "/files/a/b"},
		{name: "single byte terminator consumes slash", pattern: "/p/:value-", key: "value", value: "a/b", unescape: true, want: "/p/a%2Fb-"},
		{name: "adjacent parameter consumes slash", pattern: "/p/:value:tail", key: "value", value: "/", unescape: true, want: "/p/%2Fb"},
		{name: "adjacent parameter cannot consume middle slash", pattern: "/p/:value:tail", key: "value", value: "a/b", unescape: true, reject: true},
		{name: "adjacent parameter cannot consume leading slash", pattern: "/p/:value:tail", key: "value", value: "/a", unescape: true, reject: true},
		{name: "adjacent parameter cannot consume trailing slash", pattern: "/p/:value:tail", key: "value", value: "a/", unescape: true, reject: true},
		{name: "adjacent parameter cannot consume multibyte slash value", pattern: "/p/:value:tail", key: "value", value: "é/", unescape: true, reject: true},
		{name: "encoded parent stays in one segment", pattern: "/p/:value-", key: "value", value: "a/../b", want: "/p/a%2F..%2Fb-"},
		{name: "decoded parent before terminator", pattern: "/p/:value-", key: "value", value: "a/../b", unescape: true, reject: true},
		{name: "decoded slash splits constant dot before", pattern: "/p/.:value-", key: "value", value: "/x", unescape: true, reject: true},
		{name: "decoded slash splits constant dot after", pattern: "/p/:value.", key: "value", value: "a/", unescape: true, reject: true},
		{name: "literal percent encoded dot", pattern: "/user/:value", key: "value", value: "%2e", unescape: true, want: "/user/%252e"},
		{name: "literal percent before real dot", pattern: "/files/*", key: "*", value: "%2e/name.txt", unescape: true, want: "/files/%252e/name.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New(Config{UnescapePath: tc.unescape})
			app.Get(tc.pattern, func(c Ctx) error { return c.SendString(c.Params(tc.key)) }).Name("target")
			ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(ctx)
			params := Map{tc.key: tc.value, "tail": "b"}
			location, routeErr := app.GetRoute("target").URL(params)
			ctxLocation, ctxErr := ctx.GetRouteURL("target", params)
			redirectErr := ctx.Redirect().Route("target", RedirectConfig{Params: params})
			if tc.reject {
				require.ErrorIs(t, routeErr, ErrRouteNotRepresentable)
				require.ErrorIs(t, ctxErr, ErrRouteNotRepresentable)
				require.ErrorIs(t, redirectErr, ErrRouteNotRepresentable)
				require.Empty(t, location)
				require.Empty(t, ctxLocation)
				require.Empty(t, ctx.Response().Header.Peek(HeaderLocation))
				return
			}
			require.NoError(t, routeErr)
			require.NoError(t, ctxErr)
			require.NoError(t, redirectErr)
			require.Equal(t, tc.want, location)
			require.Equal(t, location, ctxLocation)
			require.Equal(t, location, string(ctx.Response().Header.Peek(HeaderLocation)))
			response, err := app.Test(httptest.NewRequest(MethodGet, location, http.NoBody))
			require.NoError(t, err)
			defer func() { require.NoError(t, response.Body.Close()) }()
			require.Equal(t, StatusOK, response.StatusCode)
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			value := string(body)
			if !tc.unescape {
				value, err = url.PathUnescape(value)
				require.NoError(t, err)
			}
			require.Equal(t, tc.value, value)
		})
	}
}

// Test_Route_URL_MountedRepresentabilityConfig verifies that mounted routes
// and their automatic HEAD copies validate using the parent's decoding policy.
func Test_Route_urlHasDotSegment(t *testing.T) {
	t.Parallel()
	route := &Route{}
	for path, want := range map[string]bool{
		"": false, ".": true, "..": true, "...": false, "/.": true, "/..": true, "/...": false,
		"/a/./b": true, "/a/../b": true, "/a/.../b": false, "./a": true, "../a": true,
		"/.a": false, "/a.": false, "/a./b": false, "/..a/b": false, "/a/b.c": false,
		"/a/b/.": true, "/a/b/..": true, "//.": true, "/a//../b": true, "/%2e/": true, "/%2E%2e/x": true,
	} {
		require.Equal(t, want, route.urlHasDotSegment([]byte(path)), path)
	}
}

func Test_Route_URL_MountedRepresentabilityConfig(t *testing.T) {
	t.Parallel()
	for _, parentUnescape := range []bool{false, true} {
		for _, domain := range []bool{false, true} {
			t.Run(fmt.Sprintf("unescape=%t/domain=%t", parentUnescape, domain), func(t *testing.T) {
				t.Parallel()
				parent := New(Config{UnescapePath: parentUnescape})
				child := New(Config{UnescapePath: !parentUnescape})
				child.Get("/user/:value", emptyHandler).Name("target")
				if domain {
					parent.Domain("example.com").Use("/api", child)
				} else {
					parent.Use("/api", child)
				}
				parent.startupProcess()
				matched := 0
				for _, route := range parent.GetRoutes() {
					if route.Name != "target" {
						continue
					}
					matched++
					location, err := route.URL(Map{"value": "a/b"})
					if parentUnescape {
						require.ErrorIs(t, err, ErrRouteNotRepresentable)
						require.Empty(t, location)
					} else {
						require.NoError(t, err)
						require.Equal(t, "/api/user/a%2Fb", location)
					}
				}
				require.Equal(t, 2, matched, "GET and its automatic HEAD copy must retain the parent config")
			})
		}
	}
}

// Benchmark_Route_URL_Representability measures conditional validation costs
// for static, ordinary, escaped, greedy, dotted, and terminated route values.
func Benchmark_Route_URL_Representability(b *testing.B) {
	for _, tc := range []struct{ name, pattern, value string }{
		{"static", "/health", ""},
		{"plain", "/user/:value", "fiber"},
		{"escaped", "/user/:value", "a/b?c#d"},
		{"greedy", "/files/*", "docs/readme"},
		{"dotted", "/files/*", "docs/readme.md"},
		{"terminator", "/p/:value-", "fiber"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			app := New()
			app.Get(tc.pattern, emptyHandler).Name("target")
			route := app.GetRoute("target")
			params := Map{"value": tc.value, "*": tc.value}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := route.URL(params); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
