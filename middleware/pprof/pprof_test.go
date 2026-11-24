package pprof

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	runtimepprof "runtime/pprof"
	"strings"
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
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
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

// The profiles below are registered once for the process: NewProfile panics
// on a name already in use.
var (
	customProfile = runtimepprof.NewProfile("fiber_pprof_test_custom")
	// escapedProfile has a name the index links escaped.
	escapedProfile = runtimepprof.NewProfile("fiber pprof test/escaped#name")
	// shadowedProfile has the name of a handler net/http/pprof routes on its own.
	shadowedProfile = runtimepprof.NewProfile("cmdline")
)

// Test_Pprof_CustomProfile follows the index's link to a profile the
// application registered with runtime/pprof.NewProfile, whether or not the app
// unescapes the path.
func Test_Pprof_CustomProfile(t *testing.T) {
	t.Parallel()
	for _, p := range []*runtimepprof.Profile{customProfile, escapedProfile} {
		link := (&url.URL{Path: p.Name(), RawQuery: "debug=1"}).String()
		for _, m := range pprofTestMounts {
			for _, unescape := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/UnescapePath=%t", p.Name(), m.name, unescape), func(t *testing.T) {
					t.Parallel()
					app := fiber.New(fiber.Config{UnescapePath: unescape})
					app.Use(New(Config{Prefix: m.prefix}))

					index := apptest.Body(t, apptest.Expect(t, app, m.prefix+"/debug/pprof/", 200))
					require.Contains(t, index, "href='"+link+"'")

					resp := apptest.Expect(t, app, m.prefix+"/debug/pprof/"+link, 200)
					require.Contains(t, apptest.Body(t, resp), p.Name()+" profile: total ")
				})
			}
		}
	}
}

// Test_Pprof_CustomProfile_TrailingSlash drops the trailing slash after a
// registered profile's name, as it does after a built-in one.
func Test_Pprof_CustomProfile_TrailingSlash(t *testing.T) {
	t.Parallel()
	for _, m := range pprofTestMounts {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			resp := apptest.Expect(t, m.app(Config{}), m.prefix+"/debug/pprof/"+customProfile.Name()+"/", fiber.StatusSeeOther)
			require.Equal(t, m.prefix+"/debug/pprof/"+customProfile.Name(), resp.Header.Get(fiber.HeaderLocation))
		})
	}
}

// Test_Pprof_CustomProfile_Shadowed keeps a built-in handler on its path when a
// registered profile takes its name, as net/http/pprof does by routing cmdline,
// profile, symbol and trace apart from the index. cmdline stands for all four:
// profile and trace would take the process-wide profiler and tracer.
func Test_Pprof_CustomProfile_Shadowed(t *testing.T) {
	t.Parallel()
	for _, m := range pprofTestMounts {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			resp := apptest.Expect(t, m.app(Config{}), m.prefix+"/debug/pprof/"+shadowedProfile.Name()+"?debug=1", 200)
			require.Equal(t, strings.Join(os.Args, "\x00"), apptest.Body(t, resp))
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

// fasthttpadaptor buffers the body for the http.Request, and with StreamRequestBody the server passes on a body of any size.
func Test_Pprof_StreamedBodyLimit(t *testing.T) {
	t.Parallel()
	const limit = 1024
	app := fiber.New(fiber.Config{StreamRequestBody: true, BodyLimit: limit})
	app.Use(New())

	for _, target := range []string{"/debug/pprof/", "/debug/pprof/cmdline", "/debug/pprof/symbol", "/debug/pprof/heap"} {
		for _, chunked := range []bool{false, true} {
			for size, status := range map[int]int{limit: fiber.StatusOK, limit + 1: fiber.StatusRequestEntityTooLarge} {
				req := httptest.NewRequest(fiber.MethodPost, target, strings.NewReader(strings.Repeat("a", size)))
				if chunked {
					req.TransferEncoding = []string{"chunked"}
				}
				resp := apptest.Do(t, app, req, testConfig)
				require.Equal(t, status, resp.StatusCode, "%s chunked=%v size=%d", target, chunked, size)
			}
		}
	}

	// A body that fails to read is refused as well, instead of being served as far as it came.
	conn := apptest.Dial(t, apptest.Serve(t, app, fiber.ListenConfig{DisableStartupMessage: true}))
	_, err := conn.Write([]byte("POST /debug/pprof/cmdline HTTP/1.1\r\nHost: app.example\r\nTransfer-Encoding: chunked\r\n" +
		"Connection: close\r\n\r\nzz\r\n"))
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
