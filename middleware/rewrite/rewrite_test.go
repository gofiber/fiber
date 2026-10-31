package rewrite

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_New(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		cfg  []Config
	}{
		{name: "no config"},
		{name: "config", cfg: []Config{{Rules: map[string]string{"/old": "/new"}}}},
		{name: "full config", cfg: []Config{{
			Next:  func(fiber.Ctx) bool { return true },
			Rules: map[string]string{"/old": "/new"},
		}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.NotNil(t, New(tc.cfg...), "Expected middleware to be returned, got nil")
		})
	}
}

// oldToNew and tokens are the rules of Test_Rewrite and the benchmarks.
var (
	oldToNew = map[string]string{"/old": "/new"}
	tokens   = map[string]string{"/users/*/orders/*": "/user/$1/order/$2"}
)

func Test_Rewrite(t *testing.T) {
	t.Parallel()
	// success answers "Rewrite Successful" at path; orders is the route the
	// token rule rewrites into and answers with the two captured IDs.
	success := func(path string) func(*fiber.App) {
		return func(app *fiber.App) {
			app.Get(path, func(c fiber.Ctx) error { return c.SendString("Rewrite Successful") })
		}
	}
	orders := func(app *fiber.App) {
		app.Get("/user/:userID/order/:orderID", func(c fiber.Ctx) error {
			return c.SendString(fmt.Sprintf("User ID: %s, Order ID: %s", c.Params("userID"), c.Params("orderID")))
		})
	}

	for _, tc := range []struct {
		routes func(*fiber.App)
		name   string
		target string
		body   string
		cfg    Config
	}{
		{
			name: "Next always returns true", routes: success("/old"), target: "/old", body: "Rewrite Successful",
			cfg: Config{Next: func(fiber.Ctx) bool { return true }, Rules: oldToNew},
		},
		{
			name: "Next always returns false", routes: success("/new"), target: "/old", body: "Rewrite Successful",
			cfg: Config{Next: func(fiber.Ctx) bool { return false }, Rules: oldToNew},
		},
		{
			name: "captured tokens in the rewrite rule", cfg: Config{Rules: tokens}, routes: orders,
			target: "/users/123/orders/456", body: "User ID: 123, Order ID: 456",
		},
		{
			name: "non-matching request handled by default route",
			cfg:  Config{Rules: tokens},
			routes: func(app *fiber.App) {
				orders(app)
				app.Use(func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
			},
			target: "/not-matching-any-rule", body: "OK",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(tc.cfg))
			tc.routes(app)
			apptest.ExpectBody(t, app, tc.target, fiber.StatusOK, tc.body)
		})
	}

	// Only the status is pinned here: nothing answers, so the body is Fiber's.
	t.Run("non-matching request with no default route", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Rules: tokens}))
		orders(app)
		apptest.Expect(t, app, "/not-matching-any-rule", fiber.StatusNotFound)
	})
}

// Test_Rewrite_StartAnchor verifies that a rule only matches from the start of
// the path, so a rule does not fire on an unrelated route that merely contains
// the rule path as a suffix (issue #4476).
func Test_Rewrite_StartAnchor(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Rules: map[string]string{"/users/*": "/u/$1"}}))
	app.Get("/u/:id", func(c fiber.Ctx) error { return c.SendString("rewritten:" + c.Params("id")) })
	app.Get("/api/users/:id", func(c fiber.Ctx) error { return c.SendString("api:" + c.Params("id")) })

	// The rule matches from the start of the path and is rewritten.
	apptest.ExpectBody(t, app, "/users/1", fiber.StatusOK, "rewritten:1")

	// A path that only contains the rule path as a suffix must not be rewritten.
	apptest.ExpectBody(t, app, "/api/users/1", fiber.StatusOK, "api:1")
}

// rewriteBenchCase is a scenario both benchmarks run: a config, a request
// target and whether a fallback route answers what nothing rewrites.
type rewriteBenchCase struct {
	name, target string
	cfg          Config
	fallback     bool
}

var rewriteBenchCases = []rewriteBenchCase{
	{"Next always true", "/old", Config{Next: func(fiber.Ctx) bool { return true }, Rules: oldToNew}, false},
	{"Next always false", "/old", Config{Next: func(fiber.Ctx) bool { return false }, Rules: oldToNew}, false},
	{"Rewrite with tokens", "/users/123/orders/456", Config{Rules: tokens}, false},
	{"NonMatch with default", "/not-matching-any-rule", Config{Rules: tokens}, true},
	{"NonMatch without default", "/not-matching-any-rule", Config{Rules: tokens}, false},
}

// app returns an app running the middleware with the case's config, answering
// with 200 behind it when the case has a fallback.
func (bc *rewriteBenchCase) app() *fiber.App {
	app := fiber.New()
	app.Use(New(bc.cfg))
	if bc.fallback {
		app.Use(func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	}
	return app
}

func Benchmark_Rewrite(b *testing.B) {
	for _, bc := range rewriteBenchCases {
		b.Run(bc.name, func(b *testing.B) {
			app := bc.app()

			reqCtx := &fasthttp.RequestCtx{}
			reqCtx.Request.SetRequestURI(bc.target)
			b.ReportAllocs()
			for b.Loop() {
				app.Handler()(reqCtx)
			}
		})
	}
}

func Benchmark_Rewrite_Parallel(b *testing.B) {
	for _, bc := range rewriteBenchCases {
		b.Run(bc.name, func(b *testing.B) {
			app := bc.app()
			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				reqCtx := &fasthttp.RequestCtx{}
				reqCtx.Request.SetRequestURI(bc.target)
				for pb.Next() {
					app.Handler()(reqCtx)
				}
			})
		})
	}
}

// rewritten runs one request through a rewrite config and reports the path the
// handler saw.
func rewritten(t *testing.T, cfg Config, path string) string {
	t.Helper()

	app := fiber.New()
	app.Use(New(cfg))
	app.Use(func(c fiber.Ctx) error { return c.SendString(c.Path()) })

	return apptest.Body(t, apptest.Do(t, app, path))
}

func Test_Rewrite_DeprecatedRulesOrderIsStable(t *testing.T) {
	t.Parallel()
	// A map has no order, so this pins that the ranking decides the winner
	// rather than Go's randomized iteration.
	cfg := Config{Rules: map[string]string{"/p/*": "/broad/$1", "/p/a*": "/narrow/$1"}}
	for range 25 {
		require.Equal(t, "/narrow/bc", rewritten(t, cfg, "/p/abc"))
	}
}

// Test_Rewrite_RewrittenPath runs one request through a config and checks the path the handler behind
// the rewrite saw: the order the rules are tried in, and that a rule's From is path text and not a regexp.
func Test_Rewrite_RewrittenPath(t *testing.T) {
	t.Parallel()
	// hit rewrites from to "/hit".
	hit := func(from string) Config { return Config{RuleList: []Rule{{From: from, To: "/hit"}}} }
	narrowFirst := Config{RuleList: []Rule{{From: "/p/a*", To: "/narrow/$1"}, {From: "/p/*", To: "/broad/$1"}}}
	broadFirst := Config{RuleList: []Rule{{From: "/p/*", To: "/broad/$1"}, {From: "/p/a*", To: "/narrow/$1"}}}
	// Read as a regexp, "^/a|/b$" is "(^/a)|(/b$)", so the anchors bound one
	// branch each and the rule fired on any path starting "/a" or ending "/b"
	// (issue #4476). Quoted, "|" is path text and the rule claims no path a
	// client can send, since the byte arrives percent-encoded.
	alternation := hit("/a|/b")
	// Both entry points share compilePattern, so a future split cannot quietly
	// leave the map path compiling its keys as regular expressions.
	deprecatedRules := Config{Rules: map[string]string{"/preis-1.000-euro": "/hit"}}

	for _, tc := range []struct {
		name string
		path string
		want string
		cfg  Config
	}{
		{name: "the first rule of the list wins, narrow first", cfg: narrowFirst, path: "/p/abc", want: "/narrow/bc"},
		{name: "the first rule of the list wins, broad first", cfg: broadFirst, path: "/p/abc", want: "/broad/abc"},
		{name: "dot is a dot", cfg: hit("/preis-1.000-euro"), path: "/preis-1X000-euro", want: "/preis-1X000-euro"},
		{name: "dot matches itself", cfg: hit("/preis-1.000-euro"), path: "/preis-1.000-euro", want: "/hit"},
		{name: "parentheses are text", cfg: hit("/files(old)"), path: "/filesold", want: "/filesold"},
		{name: "parentheses match themselves", cfg: hit("/files(old)"), path: "/files(old)", want: "/hit"},
		{name: "plus is text", cfg: hit("/a+"), path: "/aaa", want: "/aaa"},
		{name: "plus matches itself", cfg: hit("/a+"), path: "/a+", want: "/hit"},
		{name: "brackets are text", cfg: hit("/v[1]"), path: "/v1", want: "/v1"},
		{name: "brackets match themselves", cfg: hit("/v[1]"), path: "/v[1]", want: "/hit"},
		// A rule spelling "?" can never fire: the byte starts the query, so no
		// path carries it. Quoted it is inert, whereas as a regexp it made the
		// preceding byte optional and fired on a path the author never wrote.
		{name: "question mark no longer eats a shorter path", cfg: hit("/faq?"), path: "/fa", want: "/fa"},
		{name: "alternation does not escape the anchors, /a", cfg: alternation, path: "/a", want: "/a"},
		{name: "alternation does not escape the anchors, /b", cfg: alternation, path: "/b", want: "/b"},
		{name: "alternation does not escape the anchors, /xx/b", cfg: alternation, path: "/xx/b", want: "/xx/b"},
		{name: "alternation does not escape the anchors, /a/yy", cfg: alternation, path: "/a/yy", want: "/a/yy"},
		{name: "deprecated rules are quoted too, dot is a dot", cfg: deprecatedRules, path: "/preis-1X000-euro", want: "/preis-1X000-euro"},
		{name: "deprecated rules are quoted too, dot matches itself", cfg: deprecatedRules, path: "/preis-1.000-euro", want: "/hit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, rewritten(t, tc.cfg, tc.path))
		})
	}
}

func Test_Rewrite_BothRuleFieldsPanic(t *testing.T) {
	t.Parallel()
	require.Panics(t, func() {
		New(Config{Rules: map[string]string{"/old": "/new"}, RuleList: []Rule{{From: "/old", To: "/new"}}})
	})
}

func Test_Rewrite_DeprecatedRulesRanking(t *testing.T) {
	t.Parallel()
	type ruleMap = map[string]string
	tests := []struct {
		name  string
		rules ruleMap
		want  string // the keys in the order they are tried
	}{
		{"longer pinned prefix first", ruleMap{"/api/*": "/a", "/api/users/*": "/b"}, "/api/users/* /api/*"},
		{"same prefix, more pinned text first", ruleMap{"/cdn/*": "/a", "/cdn/*x": "/b"}, "/cdn/*x /cdn/*"},
		{"same pinned text, fewer wildcards first", ruleMap{"/p/*a*b": "/a", "/p/*ab": "/b"}, "/p/*ab /p/*a*b"},
		{"otherwise the key, so the order is total", ruleMap{"/b/*": "/x", "/a/*": "/y"}, "/a/* /b/*"},
		{"a rule without a wildcard pins its whole length", ruleMap{"/exact": "/a", "/ex*": "/b"}, "/exact /ex*"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := make([]string, 0, len(tc.rules))
			for _, rule := range orderedRules(Config{Rules: tc.rules}) {
				got = append(got, rule.From)
			}
			require.Equal(t, tc.want, strings.Join(got, " "))
		})
	}
}

// Test_Rewrite_TwoDigitCaptures pins that "$10" names the tenth capture rather than "$1" followed by "0".
func Test_Rewrite_TwoDigitCaptures(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{RuleList: []Rule{{From: "/t/*/*/*/*/*/*/*/*/*/*/*", To: "/r/$11/$10/$1"}}}))
	app.Get("/r/:a/:b/:c", func(c fiber.Ctx) error {
		return c.SendString(c.Params("a") + " " + c.Params("b") + " " + c.Params("c"))
	})

	apptest.ExpectBody(t, app, "/t/a/b/c/d/e/f/g/h/i/j/k", fiber.StatusOK, "k j a")
}

// rewrittenRaw hands app a request line exactly as written and reports the
// path the handler behind the rewrite saw.
func rewrittenRaw(t *testing.T, app *fiber.App, target string) string {
	t.Helper()

	var req fasthttp.Request
	raw := fiber.MethodGet + " " + target + " HTTP/1.1\r\nHost: example.com\r\n\r\n"
	require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

	var fctx fasthttp.RequestCtx
	fctx.Init(&req, nil, nil)
	app.Handler()(&fctx)
	require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode(), target)
	return string(fctx.Response.Body())
}

// Test_Rewrite_UnescapePathDecodesOnce pins that a capture is not decoded a
// second time when the rewritten path is set: with UnescapePath the router
// decoded "/files/%252e%252e/secret" into "/files/%2e%2e/secret", and the
// "%2e%2e" carried into "/public/$1" has to stay that name rather than
// become ".." and resolve out of "/public/".
func Test_Rewrite_UnescapePathDecodesOnce(t *testing.T) {
	t.Parallel()
	// Without UnescapePath (raw) the path keeps its escapes, and a capture is
	// carried as is; with it (unescaped) the router has decoded the path once.
	cases := []struct {
		name      string
		target    string
		unescaped string
		raw       string
	}{
		{name: "plain path", target: "/files/a/b", unescaped: "/public/a/b", raw: "/public/a/b"},
		{name: "double-encoded dot dot", target: "/files/%252e%252e/secret", unescaped: "/public/%2e%2e/secret", raw: "/public/%252e%252e/secret"},
		{name: "double-encoded slash", target: "/files/..%252Fsecret", unescaped: "/public/..%2Fsecret", raw: "/public/..%252Fsecret"},
		{name: "encoded space", target: "/files/%41%20b", unescaped: "/public/A b", raw: "/public/A%20b"},
		{name: "encoded query and fragment markers", target: "/files/a%3Fb%23c", unescaped: "/public/a?b#c", raw: "/public/a%3Fb%23c"},
		{name: "encoded multibyte rune", target: "/files/caf%C3%A9", unescaped: "/public/café", raw: "/public/caf%C3%A9"},
		{name: "single-encoded dot dot", target: "/files/%2e%2e/secret", unescaped: "/secret", raw: "/secret"},
		{name: "double-encoded dot dot, trailing slash", target: "/files/%252e%252e/secret/", unescaped: "/public/%2e%2e/secret/", raw: "/public/%252e%252e/secret/"},
	}

	for _, unescape := range []bool{false, true} {
		t.Run(fmt.Sprintf("UnescapePath=%v", unescape), func(t *testing.T) {
			t.Parallel()
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					app := fiber.New(fiber.Config{UnescapePath: unescape})
					app.Use(New(Config{RuleList: []Rule{{From: "/files/*", To: "/public/$1"}}}))
					app.Use(func(c fiber.Ctx) error { return c.SendString(c.Path()) })

					want := tc.raw
					if unescape {
						want = tc.unescaped
					}
					require.Equal(t, want, rewrittenRaw(t, app, tc.target), "UnescapePath=%v %s", unescape, tc.target)
				})
			}
		})
	}
}

// Test_Rewrite_CaptureStaysBelowTheTargetPath pins that a capture cannot spell a
// dot segment that resolves out of the prefix To names.
func Test_Rewrite_CaptureStaysBelowTheTargetPath(t *testing.T) {
	t.Parallel()

	for _, unescape := range []bool{false, true} {
		app := fiber.New(fiber.Config{UnescapePath: unescape})
		app.Use(New(Config{
			RuleList: []Rule{
				{From: "/@*", To: "/users/$1"},
				{From: "/f/*-*", To: "/users/$1$2/admin"},
				{From: "/up/*", To: "/a/../b/$1"},
			},
		}))
		app.Use(func(c fiber.Ctx) error {
			return c.SendString(c.Path())
		})

		for target, want := range map[string]string{
			"/@bob/x":    "/users/bob/x",
			"/@.../x":    "/users/.../x",
			"/f/a-b":     "/users/ab/admin",
			"/up/x":      "/b/x", // the author's own dot segment still resolves
			"/@../admin": "/@../admin",
			"/@./admin":  "/@./admin",
			"/f/.-.":     "/f/.-.",
		} {
			require.Equal(t, want, rewrittenRaw(t, app, target), "UnescapePath=%v %s", unescape, target)
		}
	}

	// Without UnescapePath a stray "%2" stays as sent, and an "e" completes it.
	app := fiber.New()
	app.Use(New(Config{RuleList: []Rule{{From: "/g/*/*", To: "/users/$1$2/admin"}}}))
	app.Use(func(c fiber.Ctx) error {
		return c.SendString(c.Path())
	})
	require.Equal(t, "/g/.%2/e", rewrittenRaw(t, app, "/g/.%2/e"))
}

// Test_Rewrite_MountedOnPrefix_RunsLaterGuards pins that routes registered after
// a rewrite mounted on its own prefix see the rewritten path.
func Test_Rewrite_MountedOnPrefix_RunsLaterGuards(t *testing.T) {
	t.Parallel()

	for _, global := range []bool{false, true} {
		app := fiber.New()
		app.Use("/v1", New(Config{RuleList: []Rule{{From: "/v1/*", To: "/$1"}}}))
		deny := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusUnauthorized) }
		if global {
			app.Use(deny)
		} else {
			app.Use("/admin", deny)
		}
		app.Get("/admin/report", func(c fiber.Ctx) error { return c.SendString("report") })

		req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/v1/admin/report", http.NoBody)
		require.NoError(t, err)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode, "global guard: %v", global)
	}
}
