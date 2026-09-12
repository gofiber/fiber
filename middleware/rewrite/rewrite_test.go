package rewrite

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_New(t *testing.T) {
	// Test with no config
	m := New()

	if m == nil {
		t.Error("Expected middleware to be returned, got nil")
	}

	// Test with config
	m = New(Config{
		Rules: map[string]string{
			"/old": "/new",
		},
	})

	if m == nil {
		t.Error("Expected middleware to be returned, got nil")
	}

	// Test with full config
	m = New(Config{
		Next: func(fiber.Ctx) bool {
			return true
		},
		Rules: map[string]string{
			"/old": "/new",
		},
	})

	if m == nil {
		t.Error("Expected middleware to be returned, got nil")
	}
}

func Test_Rewrite(t *testing.T) {
	// Case 1: Next function always returns true
	app := fiber.New()
	app.Use(New(Config{
		Next: func(fiber.Ctx) bool {
			return true
		},
		Rules: map[string]string{
			"/old": "/new",
		},
	}))

	app.Get("/old", func(c fiber.Ctx) error {
		return c.SendString("Rewrite Successful")
	})

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/old", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyString := string(body)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "Rewrite Successful", bodyString)

	// Case 2: Next function always returns false
	app = fiber.New()
	app.Use(New(Config{
		Next: func(fiber.Ctx) bool {
			return false
		},
		Rules: map[string]string{
			"/old": "/new",
		},
	}))

	app.Get("/new", func(c fiber.Ctx) error {
		return c.SendString("Rewrite Successful")
	})

	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/old", http.NoBody)
	require.NoError(t, err)
	resp, err = app.Test(req)
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyString = string(body)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "Rewrite Successful", bodyString)

	// Case 3: check for captured tokens in rewrite rule
	app = fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/users/*/orders/*": "/user/$1/order/$2",
		},
	}))

	app.Get("/user/:userID/order/:orderID", func(c fiber.Ctx) error {
		return c.SendString(fmt.Sprintf("User ID: %s, Order ID: %s", c.Params("userID"), c.Params("orderID")))
	})

	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/users/123/orders/456", http.NoBody)
	require.NoError(t, err)
	resp, err = app.Test(req)
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyString = string(body)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "User ID: 123, Order ID: 456", bodyString)

	// Case 4: Send non-matching request, handled by default route
	app = fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/users/*/orders/*": "/user/$1/order/$2",
		},
	}))

	app.Get("/user/:userID/order/:orderID", func(c fiber.Ctx) error {
		return c.SendString(fmt.Sprintf("User ID: %s, Order ID: %s", c.Params("userID"), c.Params("orderID")))
	})

	app.Use(func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/not-matching-any-rule", http.NoBody)
	require.NoError(t, err)
	resp, err = app.Test(req)
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyString = string(body)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "OK", bodyString)

	// Case 4: Send non-matching request, with no default route
	app = fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/users/*/orders/*": "/user/$1/order/$2",
		},
	}))

	app.Get("/user/:userID/order/:orderID", func(c fiber.Ctx) error {
		return c.SendString(fmt.Sprintf("User ID: %s, Order ID: %s", c.Params("userID"), c.Params("orderID")))
	})

	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/not-matching-any-rule", http.NoBody)
	require.NoError(t, err)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// Test_Rewrite_StartAnchor verifies that a rule only matches from the start of
// the path, so a rule does not fire on an unrelated route that merely contains
// the rule path as a suffix (issue #4476).
func Test_Rewrite_StartAnchor(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Rules: map[string]string{
			"/users/*": "/u/$1",
		},
	}))
	app.Get("/u/:id", func(c fiber.Ctx) error {
		return c.SendString("rewritten:" + c.Params("id"))
	})
	app.Get("/api/users/:id", func(c fiber.Ctx) error {
		return c.SendString("api:" + c.Params("id"))
	})

	// The rule matches from the start of the path and is rewritten.
	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/users/1", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "rewritten:1", string(body))

	// A path that only contains the rule path as a suffix must not be rewritten.
	req, err = http.NewRequestWithContext(context.Background(), fiber.MethodGet, "/api/users/1", http.NoBody)
	require.NoError(t, err)
	resp, err = app.Test(req)
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "api:1", string(body))
}

func Benchmark_Rewrite(b *testing.B) {
	// Helper function to create a new Fiber app with rewrite middleware
	createApp := func(config Config) *fiber.App {
		app := fiber.New()
		app.Use(New(config))
		return app
	}

	// Benchmark: Rewrite with Next function always returns true
	b.Run("Next always true", func(b *testing.B) {
		app := createApp(Config{
			Next: func(fiber.Ctx) bool {
				return true
			},
			Rules: map[string]string{
				"/old": "/new",
			},
		})

		reqCtx := &fasthttp.RequestCtx{}
		reqCtx.Request.SetRequestURI("/old")
		b.ReportAllocs()
		for b.Loop() {
			app.Handler()(reqCtx)
		}
	})

	// Benchmark: Rewrite with Next function always returns false
	b.Run("Next always false", func(b *testing.B) {
		app := createApp(Config{
			Next: func(fiber.Ctx) bool {
				return false
			},
			Rules: map[string]string{
				"/old": "/new",
			},
		})

		reqCtx := &fasthttp.RequestCtx{}
		reqCtx.Request.SetRequestURI("/old")
		b.ReportAllocs()
		for b.Loop() {
			app.Handler()(reqCtx)
		}
	})

	// Benchmark: Rewrite with tokens
	b.Run("Rewrite with tokens", func(b *testing.B) {
		app := createApp(Config{
			Rules: map[string]string{
				"/users/*/orders/*": "/user/$1/order/$2",
			},
		})

		reqCtx := &fasthttp.RequestCtx{}
		reqCtx.Request.SetRequestURI("/users/123/orders/456")
		b.ReportAllocs()
		for b.Loop() {
			app.Handler()(reqCtx)
		}
	})

	// Benchmark: Non-matching request, handled by default route
	b.Run("NonMatch with default", func(b *testing.B) {
		app := createApp(Config{
			Rules: map[string]string{
				"/users/*/orders/*": "/user/$1/order/$2",
			},
		})
		app.Use(func(c fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		})

		reqCtx := &fasthttp.RequestCtx{}
		reqCtx.Request.SetRequestURI("/not-matching-any-rule")
		b.ReportAllocs()
		for b.Loop() {
			app.Handler()(reqCtx)
		}
	})

	// Benchmark: Non-matching request, with no default route
	b.Run("NonMatch without default", func(b *testing.B) {
		app := createApp(Config{
			Rules: map[string]string{
				"/users/*/orders/*": "/user/$1/order/$2",
			},
		})

		reqCtx := &fasthttp.RequestCtx{}
		reqCtx.Request.SetRequestURI("/not-matching-any-rule")
		b.ReportAllocs()
		for b.Loop() {
			app.Handler()(reqCtx)
		}
	})
}

func Benchmark_Rewrite_Parallel(b *testing.B) {
	// Helper function to create a new Fiber app with rewrite middleware
	createApp := func(config Config) *fiber.App {
		app := fiber.New()
		app.Use(New(config))
		return app
	}

	// Parallel Benchmark: Rewrite with Next function always returns true
	b.Run("Next always true", func(b *testing.B) {
		app := createApp(Config{
			Next: func(fiber.Ctx) bool {
				return true
			},
			Rules: map[string]string{
				"/old": "/new",
			},
		})
		b.ReportAllocs()
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			reqCtx := &fasthttp.RequestCtx{}
			reqCtx.Request.SetRequestURI("/old")
			for pb.Next() {
				app.Handler()(reqCtx)
			}
		})
	})

	// Parallel Benchmark: Rewrite with Next function always returns false
	b.Run("Next always false", func(b *testing.B) {
		app := createApp(Config{
			Next: func(fiber.Ctx) bool {
				return false
			},
			Rules: map[string]string{
				"/old": "/new",
			},
		})
		b.ReportAllocs()
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			reqCtx := &fasthttp.RequestCtx{}
			reqCtx.Request.SetRequestURI("/old")
			for pb.Next() {
				app.Handler()(reqCtx)
			}
		})
	})

	// Parallel Benchmark: Rewrite with tokens
	b.Run("Rewrite with tokens", func(b *testing.B) {
		app := createApp(Config{
			Rules: map[string]string{
				"/users/*/orders/*": "/user/$1/order/$2",
			},
		})
		b.ReportAllocs()
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			reqCtx := &fasthttp.RequestCtx{}
			reqCtx.Request.SetRequestURI("/users/123/orders/456")
			for pb.Next() {
				app.Handler()(reqCtx)
			}
		})
	})

	// Parallel Benchmark: Non-matching request, handled by default route
	b.Run("NonMatch with default", func(b *testing.B) {
		app := createApp(Config{
			Rules: map[string]string{
				"/users/*/orders/*": "/user/$1/order/$2",
			},
		})
		app.Use(func(c fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		})
		b.ReportAllocs()
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			reqCtx := &fasthttp.RequestCtx{}
			reqCtx.Request.SetRequestURI("/not-matching-any-rule")
			for pb.Next() {
				app.Handler()(reqCtx)
			}
		})
	})

	// Parallel Benchmark: Non-matching request, with no default route
	b.Run("NonMatch without default", func(b *testing.B) {
		app := createApp(Config{
			Rules: map[string]string{
				"/users/*/orders/*": "/user/$1/order/$2",
			},
		})
		b.ReportAllocs()
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			reqCtx := &fasthttp.RequestCtx{}
			reqCtx.Request.SetRequestURI("/not-matching-any-rule")
			for pb.Next() {
				app.Handler()(reqCtx)
			}
		})
	})
}

// rewritten runs one request through a rewrite config and reports the path the
// handler saw.
func rewritten(t *testing.T, cfg Config, path string) string {
	t.Helper()

	app := fiber.New()
	app.Use(New(cfg))
	app.Use(func(c fiber.Ctx) error {
		return c.SendString(c.Path())
	})

	req, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, path, http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

func Test_Rewrite_RuleListOrderWins(t *testing.T) {
	t.Parallel()

	narrowFirst := Config{RuleList: []Rule{
		{From: "/p/a*", To: "/narrow/$1"},
		{From: "/p/*", To: "/broad/$1"},
	}}
	broadFirst := Config{RuleList: []Rule{
		{From: "/p/*", To: "/broad/$1"},
		{From: "/p/a*", To: "/narrow/$1"},
	}}

	require.Equal(t, "/narrow/bc", rewritten(t, narrowFirst, "/p/abc"))
	require.Equal(t, "/broad/abc", rewritten(t, broadFirst, "/p/abc"))
}

func Test_Rewrite_DeprecatedRulesOrderIsStable(t *testing.T) {
	t.Parallel()

	// A map has no order, so this pins that the ranking decides the winner
	// rather than Go's randomized iteration.
	cfg := Config{Rules: map[string]string{
		"/p/*":  "/broad/$1",
		"/p/a*": "/narrow/$1",
	}}
	for range 25 {
		require.Equal(t, "/narrow/bc", rewritten(t, cfg, "/p/abc"))
	}
}

func Test_Rewrite_PatternIsPathTextNotRegexp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		from string
		path string
		want string
	}{
		{name: "dot is a dot", from: "/preis-1.000-euro", path: "/preis-1X000-euro", want: "/preis-1X000-euro"},
		{name: "dot matches itself", from: "/preis-1.000-euro", path: "/preis-1.000-euro", want: "/hit"},
		{name: "parentheses are text", from: "/files(old)", path: "/filesold", want: "/filesold"},
		{name: "parentheses match themselves", from: "/files(old)", path: "/files(old)", want: "/hit"},
		{name: "plus is text", from: "/a+", path: "/aaa", want: "/aaa"},
		{name: "plus matches itself", from: "/a+", path: "/a+", want: "/hit"},
		{name: "brackets are text", from: "/v[1]", path: "/v1", want: "/v1"},
		{name: "brackets match themselves", from: "/v[1]", path: "/v[1]", want: "/hit"},
		// A rule spelling "?" can never fire: the byte starts the query, so no
		// path carries it. Quoted it is inert, whereas as a regexp it made the
		// preceding byte optional and fired on a path the author never wrote.
		{name: "question mark no longer eats a shorter path", from: "/faq?", path: "/fa", want: "/fa"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := Config{RuleList: []Rule{{From: tc.from, To: "/hit"}}}
			require.Equal(t, tc.want, rewritten(t, cfg, tc.path))
		})
	}
}

func Test_Rewrite_AlternationNoLongerEscapesTheAnchors(t *testing.T) {
	t.Parallel()

	// Read as a regexp, "^/a|/b$" is "(^/a)|(/b$)", so the anchors bound one
	// branch each and the rule fired on any path starting "/a" or ending "/b"
	// (issue #4476). Quoted, "|" is path text and the rule claims no path a
	// client can send, since the byte arrives percent-encoded.
	cfg := Config{RuleList: []Rule{{From: "/a|/b", To: "/hit"}}}

	for _, path := range []string{"/a", "/b", "/xx/b", "/a/yy"} {
		require.Equal(t, path, rewritten(t, cfg, path))
	}
}

func Test_Rewrite_BothRuleFieldsPanic(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		New(Config{
			Rules:    map[string]string{"/old": "/new"},
			RuleList: []Rule{{From: "/old", To: "/new"}},
		})
	})
}

func Test_Rewrite_DeprecatedRulesRanking(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		rules map[string]string
		want  []string
	}{
		{
			name:  "longer pinned prefix first",
			rules: map[string]string{"/api/*": "/a", "/api/users/*": "/b"},
			want:  []string{"/api/users/*", "/api/*"},
		},
		{
			name:  "same prefix, more pinned text first",
			rules: map[string]string{"/cdn/*": "/a", "/cdn/*x": "/b"},
			want:  []string{"/cdn/*x", "/cdn/*"},
		},
		{
			name:  "same pinned text, fewer wildcards first",
			rules: map[string]string{"/p/*a*b": "/a", "/p/*ab": "/b"},
			want:  []string{"/p/*ab", "/p/*a*b"},
		},
		{
			name:  "otherwise the key, so the order is total",
			rules: map[string]string{"/b/*": "/x", "/a/*": "/y"},
			want:  []string{"/a/*", "/b/*"},
		},
		{
			name:  "a rule without a wildcard pins its whole length",
			rules: map[string]string{"/exact": "/a", "/ex*": "/b"},
			want:  []string{"/exact", "/ex*"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := make([]string, 0, len(tc.rules))
			for _, rule := range orderedRules(Config{Rules: tc.rules}) {
				got = append(got, rule.From)
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func Test_Rewrite_DeprecatedRulesAreQuotedToo(t *testing.T) {
	t.Parallel()

	// Both entry points share compilePattern, so a future split cannot quietly
	// leave the map path compiling its keys as regular expressions.
	cfg := Config{Rules: map[string]string{"/preis-1.000-euro": "/hit"}}

	require.Equal(t, "/preis-1X000-euro", rewritten(t, cfg, "/preis-1X000-euro"))
	require.Equal(t, "/hit", rewritten(t, cfg, "/preis-1.000-euro"))
}
