package wiretarget

import (
	"bufio"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// answerFor hands app a request line exactly as written, which net/http's
// request builders would reject or rewrite, and returns what Routed answered.
func answerFor(t *testing.T, app *fiber.App, target string) string {
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

func echoRouted(c fiber.Ctx) error {
	return c.SendString(Routed(c))
}

func Test_Routed(t *testing.T) {
	t.Parallel()

	t.Run("default", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/*", echoRouted)
		for target, want := range map[string]string{
			"/":                       "/",
			"/a%2Fb/%41?q=%2F&x=1":    "/a%2Fb/A?q=%2F&x=1",
			"/a//b":                   "/a//b",
			"/a/./b/../c":             "/a/c",
			"/%2e%2e/a":               "/a",
			"/a%2fb":                  "/a%2Fb",
			"/%zz":                    "/%25zz",
			"/a%zzb%2Fc%":             "/a%25zzb%2Fc%25",
			"/trailing%2":             "/trailing%252",
			"/%":                      "/%25",
			"/a?":                     "/a",
			"/a?%zz=%zz":              "/a?%zz=%zz",
			"/caf%C3%A9?x=caf%C3%A9":  "/caf%C3%A9?x=caf%C3%A9",
			"/a?x=1#frag":             "/a?x=1",
			"/sp%20ace/%7Etilde?x=+y": "/sp%20ace/~tilde?x=+y",
			"/UPPER/%5C?X=1":          "/UPPER/%5C?X=1",
		} {
			require.Equal(t, want, answerFor(t, app, target), target)
		}
	})

	t.Run("unescape path", func(t *testing.T) {
		t.Parallel()
		app := fiber.New(fiber.Config{UnescapePath: true})
		app.Get("/*", echoRouted)
		for target, want := range map[string]string{
			"/a%2Fb/%41%20c?q=%2F": "/a/b/A%20c?q=%2F",
			"/a%2F..%2Fb":          "/b",
			"/a%2F%2Fb":            "/a//b",
			"/caf%C3%A9":           "/caf%C3%A9",
			"/%zz":                 "/%25zz",
			"/%252e%252e/a":        "/%252e%252e/a",
			"/a%3Fb%23c%25d":       "/a%3Fb%23c%25d",
		} {
			require.Equal(t, want, answerFor(t, app, target), target)
		}
	})

	t.Run("query arguments changed by a handler", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(func(c fiber.Ctx) error {
			c.Request().URI().QueryArgs().Set("k", "v")
			return c.Next()
		})
		app.Get("/*", echoRouted)
		require.Equal(t, "/a?x=1&k=v", answerFor(t, app, "/a?x=1"))
	})

	t.Run("path overridden by a handler", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(func(c fiber.Ctx) error {
			c.Path("/rewritten/%41")
			return c.Next()
		})
		app.Get("/*", echoRouted)
		require.Equal(t, "/rewritten/A?x=1", answerFor(t, app, "/original?x=1"))
	})

	t.Run("immutable", func(t *testing.T) {
		t.Parallel()
		app := fiber.New(fiber.Config{Immutable: true})
		app.Get("/*", echoRouted)
		require.Equal(t, "/a%2Fb/A?q=1", answerFor(t, app, "/a%2Fb/%41?q=1"))
	})
}

func Test_SegmentsAsRouted(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"/", "/a/b", "/a/b?x=%2F&y=//", "/a%252Fb", "/%2E%2E/a", "/a%5Cb", "/a/b/"} {
		require.True(t, SegmentsAsRouted(target), target)
	}
	for _, target := range []string{"//", "//admin", "/a//b", "/a/b//", "/public/..%2Fadmin", "/a%2fb", "/a%2Fb?x=1"} {
		require.False(t, SegmentsAsRouted(target), target)
	}
}
