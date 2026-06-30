// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// newSkipApp builds an app with SkipUnmatchedRoutes enabled plus a benign
// middleware, so requests actually exercise the lookahead fast path (which is
// gated on the presence of middleware).
func newSkipApp(cfg ...Config) *App {
	c := Config{SkipUnmatchedRoutes: true}
	if len(cfg) > 0 {
		c = cfg[0]
		c.SkipUnmatchedRoutes = true
	}
	app := New(c)
	app.Use(func(c Ctx) error { return c.Next() })
	return app
}

func Test_App_SkipUnmatchedRoutes_Static(t *testing.T) {
	t.Parallel()

	app := newSkipApp()
	app.Get("/users", func(c Ctx) error { return c.SendString("users") })
	app.Post("/users", func(c Ctx) error { return c.SendString("created") })

	t.Run("match", func(t *testing.T) {
		t.Parallel()
		resp, err := app.Test(httptest.NewRequest(MethodGet, "/users", nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "users", string(body))
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()
		resp, err := app.Test(httptest.NewRequest(MethodGet, "/does/not/exist", nil))
		require.NoError(t, err)
		require.Equal(t, StatusNotFound, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})

	t.Run("method_not_allowed", func(t *testing.T) {
		t.Parallel()
		resp, err := app.Test(httptest.NewRequest(MethodDelete, "/users", nil))
		require.NoError(t, err)
		require.Equal(t, StatusMethodNotAllowed, resp.StatusCode)
		allow := resp.Header.Get(HeaderAllow)
		require.Contains(t, allow, MethodGet)
		require.Contains(t, allow, MethodPost)
		require.NoError(t, resp.Body.Close())
	})
}

func Test_App_SkipUnmatchedRoutes_Parametric(t *testing.T) {
	t.Parallel()

	app := newSkipApp()
	app.Get("/user/keys/:id", func(c Ctx) error { return c.SendString(c.Params("id")) })
	app.Post("/user/keys/:id", func(c Ctx) error { return c.SendString("post") })

	t.Run("match", func(t *testing.T) {
		t.Parallel()
		resp, err := app.Test(httptest.NewRequest(MethodGet, "/user/keys/1337", nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "1337", string(body))
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()
		resp, err := app.Test(httptest.NewRequest(MethodGet, "/user/secrets/1337", nil))
		require.NoError(t, err)
		require.Equal(t, StatusNotFound, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})

	t.Run("method_not_allowed", func(t *testing.T) {
		t.Parallel()
		resp, err := app.Test(httptest.NewRequest(MethodDelete, "/user/keys/1337", nil))
		require.NoError(t, err)
		require.Equal(t, StatusMethodNotAllowed, resp.StatusCode)
		allow := resp.Header.Get(HeaderAllow)
		require.Contains(t, allow, MethodGet)
		require.Contains(t, allow, MethodPost)
		require.NoError(t, resp.Body.Close())
	})
}

// Test_App_SkipUnmatchedRoutes_MiddlewareSkipped verifies that middleware does
// NOT run for unmatched requests but DOES run for matched ones.
func Test_App_SkipUnmatchedRoutes_MiddlewareSkipped(t *testing.T) {
	t.Parallel()

	t.Run("skipped_on_404", func(t *testing.T) {
		t.Parallel()
		app := New(Config{SkipUnmatchedRoutes: true})
		called := false
		app.Use(func(c Ctx) error { called = true; return c.Next() })
		app.Get("/ok", func(c Ctx) error { return c.SendString("ok") })

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/nope", nil))
		require.NoError(t, err)
		require.Equal(t, StatusNotFound, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
		require.False(t, called, "middleware must NOT run for an unmatched route")
	})

	t.Run("runs_on_match", func(t *testing.T) {
		t.Parallel()
		app := New(Config{SkipUnmatchedRoutes: true})
		called := false
		app.Use(func(c Ctx) error { called = true; return c.Next() })
		app.Get("/ok", func(c Ctx) error { return c.SendString("ok") })

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/ok", nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
		require.True(t, called, "middleware must run for a matched route")
	})

	t.Run("disabled_runs_middleware", func(t *testing.T) {
		t.Parallel()
		app := New() // SkipUnmatchedRoutes is false by default
		called := false
		app.Use(func(c Ctx) error { called = true; return c.Next() })
		app.Get("/ok", func(c Ctx) error { return c.SendString("ok") })

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/nope", nil))
		require.NoError(t, err)
		require.Equal(t, StatusNotFound, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
		require.True(t, called, "middleware should still run when the feature is disabled")
	})
}

// Test_App_SkipUnmatchedRoutes_NoMiddlewareGate verifies correct results when the
// feature is enabled but no middleware is registered (the lookahead is gated off).
func Test_App_SkipUnmatchedRoutes_NoMiddlewareGate(t *testing.T) {
	t.Parallel()

	app := New(Config{SkipUnmatchedRoutes: true}) // no Use
	app.Get("/ping", func(c Ctx) error { return c.SendString("pong") })

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/ping", nil))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/missing", nil))
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	resp, err = app.Test(httptest.NewRequest(MethodPost, "/ping", nil))
	require.NoError(t, err)
	require.Equal(t, StatusMethodNotAllowed, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

// Test_App_SkipUnmatchedRoutes_CustomErrorHandler covers the emitSkip path where
// the configured error handler itself returns an error (-> 500).
func Test_App_SkipUnmatchedRoutes_CustomErrorHandler(t *testing.T) {
	t.Parallel()

	app := New(Config{
		SkipUnmatchedRoutes: true,
		ErrorHandler:        func(_ Ctx, _ error) error { return errors.New("boom") },
	})
	app.Use(func(c Ctx) error { return c.Next() })
	app.Get("/ok", func(c Ctx) error { return c.SendString("ok") })

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/nope", nil))
	require.NoError(t, err)
	require.Equal(t, StatusInternalServerError, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

// Test_App_SkipUnmatchedRoutes_CrossMethodMixed exercises the 405 path where the
// path matches a static endpoint of one method and a parametric endpoint of
// another, so both Allow entries must be reported.
func Test_App_SkipUnmatchedRoutes_CrossMethodMixed(t *testing.T) {
	t.Parallel()

	app := newSkipApp()
	app.Get("/thing/:id", func(c Ctx) error { return c.SendString("get") })  // parametric
	app.Post("/thing/fixed", func(c Ctx) error { return c.SendString("p") }) // static

	resp, err := app.Test(httptest.NewRequest(MethodDelete, "/thing/fixed", nil))
	require.NoError(t, err)
	require.Equal(t, StatusMethodNotAllowed, resp.StatusCode)
	allow := resp.Header.Get(HeaderAllow)
	require.Contains(t, allow, MethodGet)
	require.Contains(t, allow, MethodPost)
	require.NoError(t, resp.Body.Close())
}

// Test_App_SkipUnmatchedRoutes_Bucket0Fallback exercises the tier-2 fallback to
// the global (bucket 0) candidate list when the request's prefix bucket is absent.
func Test_App_SkipUnmatchedRoutes_Bucket0Fallback(t *testing.T) {
	t.Parallel()

	app := newSkipApp()
	app.Get("/:id", func(c Ctx) error { return c.SendString(c.Params("id")) })

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/abc", nil))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, "abc", string(body))
}

// Test_App_SkipUnmatchedRoutes_SkipBefore exercises next()'s skip-before guard:
// a static endpoint sits before the parametric endpoint in the same bucket, so
// firstMatchIndex > 0 and the endpoints before it are skipped.
func Test_App_SkipUnmatchedRoutes_SkipBefore(t *testing.T) {
	t.Parallel()

	app := newSkipApp()
	app.Get("/key/aaa", func(c Ctx) error { return c.SendString("static") })
	app.Get("/key/:id", func(c Ctx) error { return c.SendString(c.Params("id")) })

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/key/123", nil))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, "123", string(body))
}

func Test_App_SkipUnmatchedRoutes_OptionalParam(t *testing.T) {
	t.Parallel()

	app := newSkipApp()
	app.Get("/opt/:v?", func(c Ctx) error { return c.SendString("opt:" + c.Params("v")) })

	for _, path := range []string{"/opt", "/opt/x"} {
		resp, err := app.Test(httptest.NewRequest(MethodGet, path, nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode, path)
		require.NoError(t, resp.Body.Close())
	}

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/other", nil))
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

func Test_App_SkipUnmatchedRoutes_Constraint(t *testing.T) {
	t.Parallel()

	app := newSkipApp()
	app.Get("/num/:id<int>", func(c Ctx) error { return c.SendString(c.Params("id")) })

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/num/5", nil))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/num/abc", nil))
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

func Test_App_SkipUnmatchedRoutes_RootAndStar(t *testing.T) {
	t.Parallel()

	t.Run("root", func(t *testing.T) {
		t.Parallel()
		app := newSkipApp()
		app.Get("/", func(c Ctx) error { return c.SendString("root") })

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})

	t.Run("star", func(t *testing.T) {
		t.Parallel()
		app := newSkipApp()
		app.Get("/*", func(c Ctx) error { return c.SendString("star") })

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/anything/here", nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "star", string(body))
	})
}

func Test_App_SkipUnmatchedRoutes_AutoHead(t *testing.T) {
	t.Parallel()

	t.Run("static", func(t *testing.T) {
		t.Parallel()
		app := newSkipApp()
		app.Get("/page", func(c Ctx) error { return c.SendString("page") })

		resp, err := app.Test(httptest.NewRequest(MethodHead, "/page", nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})

	t.Run("param", func(t *testing.T) {
		t.Parallel()
		app := newSkipApp()
		app.Get("/h/:id", func(c Ctx) error { return c.SendString(c.Params("id")) })

		resp, err := app.Test(httptest.NewRequest(MethodHead, "/h/1", nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})
}

func Test_App_SkipUnmatchedRoutes_GroupMiddleware(t *testing.T) {
	t.Parallel()

	app := New(Config{SkipUnmatchedRoutes: true})
	called := false
	g := app.Group("/g")
	g.Use(func(c Ctx) error { called = true; return c.Next() })
	g.Get("/ok", func(c Ctx) error { return c.SendString("ok") })

	t.Run("unmatched_skips_group_mw", func(t *testing.T) {
		resp, err := app.Test(httptest.NewRequest(MethodGet, "/g/nope", nil))
		require.NoError(t, err)
		require.Equal(t, StatusNotFound, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
		require.False(t, called, "group middleware must not run for an unmatched route")
	})

	t.Run("matched_runs_group_mw", func(t *testing.T) {
		resp, err := app.Test(httptest.NewRequest(MethodGet, "/g/ok", nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
		require.True(t, called, "group middleware must run for a matched route")
	})
}

func Test_App_SkipUnmatchedRoutes_Mount(t *testing.T) {
	t.Parallel()

	sub := New()
	sub.Get("/profile", func(c Ctx) error { return c.SendString("profile") })

	app := newSkipApp()
	app.Use("/account", sub)

	t.Run("mounted_match", func(t *testing.T) {
		t.Parallel()
		resp, err := app.Test(httptest.NewRequest(MethodGet, "/account/profile", nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "profile", string(body))
	})

	t.Run("mounted_miss", func(t *testing.T) {
		t.Parallel()
		resp, err := app.Test(httptest.NewRequest(MethodGet, "/account/missing", nil))
		require.NoError(t, err)
		require.Equal(t, StatusNotFound, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})
}

func Test_App_SkipUnmatchedRoutes_CaseSensitiveAndStrict(t *testing.T) {
	t.Parallel()

	t.Run("case_sensitive", func(t *testing.T) {
		t.Parallel()
		app := newSkipApp(Config{CaseSensitive: true})
		app.Get("/Foo", func(c Ctx) error { return c.SendString("foo") })

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/Foo", nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())

		resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo", nil))
		require.NoError(t, err)
		require.Equal(t, StatusNotFound, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})

	t.Run("strict_routing", func(t *testing.T) {
		t.Parallel()
		app := newSkipApp(Config{StrictRouting: true})
		app.Get("/bar", func(c Ctx) error { return c.SendString("bar") })

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/bar", nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())

		resp, err = app.Test(httptest.NewRequest(MethodGet, "/bar/", nil))
		require.NoError(t, err)
		require.Equal(t, StatusNotFound, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})

	t.Run("non_strict_trailing_slash", func(t *testing.T) {
		t.Parallel()
		app := newSkipApp()
		app.Get("/baz", func(c Ctx) error { return c.SendString("baz") })

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/baz/", nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})
}

func Test_App_SkipUnmatchedRoutes_RebuildTree(t *testing.T) {
	t.Parallel()

	app := newSkipApp()
	app.Get("/first", func(c Ctx) error { return c.SendString("first") })

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/second", nil))
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	app.Get("/second", func(c Ctx) error { return c.SendString("second") })
	app.RebuildTree()

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/second", nil))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

// Test_App_SkipUnmatchedRoutes_RestartRouting ensures a handler that rewrites
// the path and restarts routing re-resolves correctly (no stale lookahead index).
func Test_App_SkipUnmatchedRoutes_RestartRouting(t *testing.T) {
	t.Parallel()

	app := newSkipApp()
	app.Get("/old", func(c Ctx) error {
		c.Path("/new")
		return c.RestartRouting()
	})
	app.Get("/new", func(c Ctx) error { return c.SendString("new") })

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/old", nil))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, "new", string(body))
}

// Test_App_SkipUnmatchedRoutes_Parity asserts that enabling SkipUnmatchedRoutes
// produces the same status codes as the default behavior across a route set.
func Test_App_SkipUnmatchedRoutes_Parity(t *testing.T) {
	t.Parallel()

	build := func(skip bool) *App {
		app := New(Config{SkipUnmatchedRoutes: skip})
		app.Use(func(c Ctx) error { return c.Next() })
		registerDummyRoutes(app)
		return app
	}
	off := build(false)
	on := build(true)

	cases := []struct {
		method string
		path   string
	}{
		{MethodGet, "/user/keys/1337"},
		{MethodGet, "/does/not/exist"},
		{MethodGet, "/repos/gofiber/fiber/stargazers"},
		{MethodPost, "/user/keys/1337"},
		{MethodDelete, "/user/keys/1337"},
		{MethodGet, "/"},
		{MethodGet, "/applications/client/tokens"},
		{MethodGet, "/user/repos"},
	}

	for _, tc := range cases {
		respOff, err := off.Test(httptest.NewRequest(tc.method, tc.path, nil))
		require.NoError(t, err)
		respOn, err := on.Test(httptest.NewRequest(tc.method, tc.path, nil))
		require.NoError(t, err)
		require.Equal(t, respOff.StatusCode, respOn.StatusCode, "%s %s", tc.method, tc.path)
		require.Equal(t, respOff.Header.Get(HeaderAllow), respOn.Header.Get(HeaderAllow), "%s %s allow", tc.method, tc.path)
		require.NoError(t, respOff.Body.Close())
		require.NoError(t, respOn.Body.Close())
	}
}

// Test_App_SkipUnmatchedRoutes_CustomCtx exercises the customRequestHandler path.
func Test_App_SkipUnmatchedRoutes_CustomCtx(t *testing.T) {
	t.Parallel()

	newCtx := func(app *App) CustomCtx { return &customCtx{DefaultCtx: *NewDefaultCtx(app)} }
	app := NewWithCustomCtx(newCtx, Config{SkipUnmatchedRoutes: true})
	app.Use(func(c Ctx) error { return c.Next() })
	app.Get("/users", func(c Ctx) error { return c.SendString("users") })
	app.Get("/user/keys/:id", func(c Ctx) error { return c.SendString("key") })

	t.Run("static_match", func(t *testing.T) {
		t.Parallel()
		resp, err := app.Test(httptest.NewRequest(MethodGet, "/users", nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})

	t.Run("param_match", func(t *testing.T) {
		t.Parallel()
		resp, err := app.Test(httptest.NewRequest(MethodGet, "/user/keys/1", nil))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()
		resp, err := app.Test(httptest.NewRequest(MethodGet, "/nope", nil))
		require.NoError(t, err)
		require.Equal(t, StatusNotFound, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})

	t.Run("method_not_allowed", func(t *testing.T) {
		t.Parallel()
		resp, err := app.Test(httptest.NewRequest(MethodDelete, "/users", nil))
		require.NoError(t, err)
		require.Equal(t, StatusMethodNotAllowed, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	})
}

// go test -v -run=^$ -bench=Benchmark_SkipUnmatchedRoutes -benchmem -count=4
func Benchmark_SkipUnmatchedRoutes_Matched(b *testing.B) {
	// Static route hit — should be at parity with the feature disabled.
	run := func(b *testing.B, skip bool) {
		b.Helper()
		app := New(Config{SkipUnmatchedRoutes: skip})
		app.Use(func(c Ctx) error { return c.Next() })
		registerDummyRoutes(app)
		appHandler := app.Handler()

		c := &fasthttp.RequestCtx{}
		c.Request.Header.SetMethod(MethodGet)
		c.URI().SetPath("/user/repos") // genuinely static route

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			appHandler(c)
		}
	}
	b.Run("without_skip", func(b *testing.B) { run(b, false) })
	b.Run("with_skip", func(b *testing.B) { run(b, true) })
}

func Benchmark_SkipUnmatchedRoutes_MatchedParam(b *testing.B) {
	run := func(b *testing.B, skip bool) {
		b.Helper()
		app := New(Config{SkipUnmatchedRoutes: skip})
		app.Use(func(c Ctx) error { return c.Next() })
		registerDummyRoutes(app)
		appHandler := app.Handler()

		c := &fasthttp.RequestCtx{}
		c.Request.Header.SetMethod(MethodGet)
		c.URI().SetPath("/user/keys/1337")

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			appHandler(c)
		}
	}
	b.Run("without_skip", func(b *testing.B) { run(b, false) })
	b.Run("with_skip", func(b *testing.B) { run(b, true) })
}

func Benchmark_SkipUnmatchedRoutes_Unmatched(b *testing.B) {
	run := func(b *testing.B, skip bool) {
		b.Helper()
		app := New(Config{SkipUnmatchedRoutes: skip})
		app.Use(func(c Ctx) error { return c.Next() })
		registerDummyRoutes(app)
		appHandler := app.Handler()

		c := &fasthttp.RequestCtx{}
		c.Request.Header.SetMethod(MethodGet)
		c.URI().SetPath("/this/route/does/not/exist")

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			appHandler(c)
		}
	}
	b.Run("without_skip", func(b *testing.B) { run(b, false) })
	b.Run("with_skip", func(b *testing.B) { run(b, true) })
}
