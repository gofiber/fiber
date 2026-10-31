package pprof

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

var testConfig = fiber.TestConfig{Timeout: 5 * time.Second, FailOnTimeout: true}

// pprofTestMount is where the middleware is mounted: at its default location,
// or under a Prefix.
type pprofTestMount struct {
	name   string
	prefix string
}

var pprofTestMounts = []pprofTestMount{{name: "default"}, {name: "with prefix", prefix: "/federated-fiber"}}

// app returns an app running the middleware at m, configured by cfg, with a
// route "/" that answers "escaped": what a request the middleware passes on is
// answered with.
func (m pprofTestMount) app(cfg Config) *fiber.App {
	cfg.Prefix = m.prefix

	app := fiber.New()
	app.Use(New(cfg))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("escaped") })
	return app
}

func Test_Non_Pprof_Path(t *testing.T) {
	t.Parallel()
	for _, m := range pprofTestMounts {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			apptest.ExpectBody(t, m.app(Config{}), "/", 200, "escaped")
		})
	}
}

func Test_Pprof_Index(t *testing.T) {
	t.Parallel()
	for _, m := range pprofTestMounts {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			resp := apptest.Expect(t, m.app(Config{}), m.prefix+"/debug/pprof/", 200)
			require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

			b := apptest.Body(t, resp)
			require.Contains(t, b, "<title>/debug/pprof/</title>")
		})
	}
}

// Test_Pprof_Subs is not parallel, and neither are its subtests: the profile
// and trace handlers take the process-wide CPU profiler and execution tracer,
// which a second request cannot hold at the same time.
func Test_Pprof_Subs(t *testing.T) {
	subs := []string{
		"cmdline", "profile", "symbol", "trace", "allocs", "block",
		"goroutine", "heap", "mutex", "threadcreate",
	}

	for _, m := range pprofTestMounts {
		t.Run(m.name, func(t *testing.T) {
			app := m.app(Config{})

			for _, sub := range subs {
				t.Run(sub, func(t *testing.T) {
					target := m.prefix + "/debug/pprof/" + sub
					if sub == "profile" {
						target += "?seconds=1"
					}
					resp := apptest.Do(t, app, target, testConfig)
					require.Equal(t, 200, resp.StatusCode)
				})
			}
		})
	}
}

func Test_Pprof_Other(t *testing.T) {
	t.Parallel()
	for _, m := range pprofTestMounts {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			apptest.Expect(t, m.app(Config{}), m.prefix+"/debug/pprof/303", fiber.StatusSeeOther)
		})
	}
}

func Test_Pprof_TrailingSlash_RedirectsWithinPrefix(t *testing.T) {
	t.Parallel()
	for _, m := range pprofTestMounts {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			resp := apptest.Expect(t, m.app(Config{}), m.prefix+"/debug/pprof/heap/", fiber.StatusSeeOther)
			require.Equal(t, m.prefix+"/debug/pprof/heap", resp.Header.Get(fiber.HeaderLocation))
		})
	}
}

func Test_Pprof_Next(t *testing.T) {
	t.Parallel()
	for _, m := range pprofTestMounts {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			app := m.app(Config{Next: func(_ fiber.Ctx) bool { return true }})

			apptest.Expect(t, app, m.prefix+"/debug/pprof/", 404)
		})
	}
}

// Test_Pprof_Sibling_Prefix_Path ensures a route that merely starts with the
// same characters as /debug/pprof is not swallowed by the middleware.
func Test_Pprof_Sibling_Prefix_Path(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Get("/debug/pprofiler", func(c fiber.Ctx) error { return c.SendString("app route") })

	apptest.ExpectBody(t, app, "/debug/pprofiler", fiber.StatusOK, "app route")
}
