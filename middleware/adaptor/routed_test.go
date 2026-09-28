package adaptor

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// rawCtx hands app a request line exactly as written, which net/http's request
// builders would reject or rewrite, and returns the served context.
func rawCtx(t *testing.T, app *fiber.App, target string) *fasthttp.RequestCtx {
	t.Helper()

	var req fasthttp.Request
	raw := fiber.MethodGet + " " + target + " HTTP/1.1\r\nHost: example.com\r\n\r\n"
	require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

	fctx := new(fasthttp.RequestCtx)
	fctx.Init(&req, nil, nil)
	app.Handler()(fctx)
	return fctx
}

func denyAll(c fiber.Ctx) error {
	return c.SendStatus(fiber.StatusForbidden)
}

// describeURL is a net/http handler that answers with how it read the request.
func describeURL(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, r.URL.Path, "|", r.URL.RawPath, "|", r.RequestURI)
}

func Test_HTTPHandler_RoutedPath(t *testing.T) {
	t.Parallel()

	t.Run("default", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use("/admin", denyAll)
		app.Use(HTTPHandler(http.HandlerFunc(describeURL)))

		for target, want := range map[string]string{
			"/plain/path":       "/plain/path||/plain/path",
			"/public/%41?x=%2F": "/public/A||/public/A?x=%2F",
			"/%zz":              "/%zz||/%25zz",
			"/a%3Fb%23c":        "/a?b#c||/a%3Fb%23c",
			"/sp%20ace/":        "/sp ace/||/sp%20ace/",
		} {
			fctx := rawCtx(t, app, target)
			require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode(), target)
			require.Equal(t, want, string(fctx.Response.Body()), target)
		}
		// Routed under "/admin", the guard answers before the handler.
		for _, target := range []string{"/admin/x", "/a/../admin/x", "/%2e%2e/admin/x", "/public/%2E./admin/x"} {
			require.Equal(t, fiber.StatusForbidden, rawCtx(t, app, target).Response.StatusCode(), target)
		}
		// An empty segment or an escaped slash would be cleaned or decoded
		// into a path the router never matched, so the handler never sees it.
		for _, target := range []string{"//admin/x", "/public//x", "/x//", "/public/..%2Fadmin/x", "/public%2F..%2Fadmin", "/a%2fb"} {
			require.Equal(t, fiber.StatusNotFound, rawCtx(t, app, target).Response.StatusCode(), target)
		}
	})

	t.Run("unescape path", func(t *testing.T) {
		t.Parallel()
		app := fiber.New(fiber.Config{UnescapePath: true})
		app.Use("/admin", denyAll)
		app.Use(HTTPHandler(http.HandlerFunc(describeURL)))

		fctx := rawCtx(t, app, "/public/a%2Fb/%41%20c?x=%2F")
		require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode())
		require.Equal(t, "/public/a/b/A c||/public/a/b/A%20c?x=%2F", string(fctx.Response.Body()))

		// Decoded before matching, the escaped slash completed a dot segment
		// the router resolved: the request was routed under "/admin".
		for _, target := range []string{"/public/..%2Fadmin/x", "/public%2F..%2Fadmin/x"} {
			require.Equal(t, fiber.StatusForbidden, rawCtx(t, app, target).Response.StatusCode(), target)
		}
		require.Equal(t, fiber.StatusNotFound, rawCtx(t, app, "/a%2F%2Fb").Response.StatusCode())
	})
}

// Test_HTTPHandler_FileServerStaysInPlace serves a directory tree through
// net/http's file server behind a Fiber guard on "/admin", and pins that no
// spelling of the admin file's path reaches the file server unguarded.
func Test_HTTPHandler_FileServerStaysInPlace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "public"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "admin"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "public", "ok.txt"), []byte("ok"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "admin", "secret.txt"), []byte("secret"), 0o600))

	app := fiber.New()
	app.Use("/admin", denyAll)
	app.Use(HTTPHandler(http.FileServer(http.Dir(root))))

	fctx := rawCtx(t, app, "/public/ok.txt")
	require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode())
	require.Equal(t, "ok", string(fctx.Response.Body()))

	for target, want := range map[string]int{
		"/admin/secret.txt":               fiber.StatusForbidden,
		"/public/../admin/secret.txt":     fiber.StatusForbidden,
		"/public/%2e%2e/admin/secret.txt": fiber.StatusForbidden,
		"/public/..%2Fadmin/secret.txt":   fiber.StatusNotFound,
		"/public%2F..%2Fadmin/secret.txt": fiber.StatusNotFound,
		"//admin/secret.txt":              fiber.StatusNotFound,
		// The ".." removes the empty segment before it (RFC 3986 Section
		// 5.2.4), so this is routed as "/public/admin/secret.txt", a file
		// that does not exist, and never as "/admin/secret.txt".
		"/public//../admin/secret.txt": fiber.StatusNotFound,
	} {
		fctx := rawCtx(t, app, target)
		require.Equal(t, want, fctx.Response.StatusCode(), target)
		require.NotContains(t, string(fctx.Response.Body()), "secret", target)
	}
}

func Test_HTTPHandler_RestoresRequestLine(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	var after string
	app.Use(func(c fiber.Ctx) error {
		err := c.Next()
		after = c.OriginalURL()
		return err
	})
	app.Use(HTTPHandler(http.HandlerFunc(describeURL)))

	fctx := rawCtx(t, app, "/a/../public/%41?x=1")
	require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode())
	require.Equal(t, "/public/A||/public/A?x=1", string(fctx.Response.Body()))
	require.Equal(t, "/a/../public/%41?x=1", after)
}

// Test_HTTPHandler_FlushedHandlerKeepsRequestLine pins that a handler still
// running after it flushed reads its URL undisturbed: the request line is not
// put back under it. Run with -race, the restore would be reported.
func Test_HTTPHandler_FlushedHandlerKeepsRequestLine(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	var after string
	app.Use(func(c fiber.Ctx) error {
		err := c.Next()
		after = c.OriginalURL()
		return err
	})
	app.Use(HTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !assert.True(t, ok, "w does not implement http.Flusher") {
			return
		}
		flusher.Flush()
		time.Sleep(20 * time.Millisecond)
		fmt.Fprint(w, r.URL.Path, "|", r.RequestURI)
	})))

	fctx := rawCtx(t, app, "/a/../public/%41?x=1")
	require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode())
	require.Equal(t, "/public/A|/public/A?x=1", string(fctx.Response.Body()))
	require.Equal(t, "/public/A?x=1", after)
}

func Test_HTTPHandlerWithContext_RoutedPath(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use("/admin", denyAll)
	app.Use(HTTPHandlerWithContext(http.HandlerFunc(describeURL)))

	fctx := rawCtx(t, app, "/a/../public/%41?x=1")
	require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode())
	require.Equal(t, "/public/A||/public/A?x=1", string(fctx.Response.Body()))
	require.Equal(t, fiber.StatusForbidden, rawCtx(t, app, "/%2e%2e/admin/x").Response.StatusCode())
	require.Equal(t, fiber.StatusNotFound, rawCtx(t, app, "/public/..%2Fadmin/x").Response.StatusCode())
}

// Test_HTTPMiddleware_SeesRoutedPath pins that a net/http guard on r.URL.Path
// sees the path the router matched, and that an escaped slash is handed on
// rather than decoded, so a Fiber route behind the middleware still gets it.
func Test_HTTPMiddleware_SeesRoutedPath(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(HTTPMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/admin") {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			r.Header.Set("X-Seen-Path", r.URL.Path)
			r.Header.Set("X-Seen-Escaped", r.URL.EscapedPath())
			next.ServeHTTP(w, r)
		})
	}))
	app.Use(func(c fiber.Ctx) error {
		return c.SendString(c.Get("X-Seen-Path") + "|" + c.Get("X-Seen-Escaped") + "|" + c.Path() + "|" + c.OriginalURL())
	})

	for target, want := range map[string]string{
		"/items/a%2Fb?x=1": "/items/a/b|/items/a%2Fb|/items/a%2Fb|/items/a%2Fb?x=1",
		"/public/%41":      "/public/A|/public/A|/public/A|/public/A",
		"//admin/x":        "//admin/x|//admin/x|//admin/x|//admin/x",
	} {
		fctx := rawCtx(t, app, target)
		require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode(), target)
		require.Equal(t, want, string(fctx.Response.Body()), target)
	}
	for _, target := range []string{"/admin/x", "/a/../admin/x", "/%2e%2e/admin/x"} {
		require.Equal(t, fiber.StatusForbidden, rawCtx(t, app, target).Response.StatusCode(), target)
	}
}

func Test_ConvertRequest_RoutedPath(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/*", func(c fiber.Ctx) error {
		r, err := ConvertRequest(c, true)
		if err != nil {
			return err
		}
		return c.SendString(r.URL.Path + "|" + r.URL.RawPath + "|" + r.RequestURI + "|" + c.OriginalURL())
	})

	fctx := rawCtx(t, app, "/a/../public/%41%2Fb?x=1")
	require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode())
	require.Equal(t, "/public/A/b|/public/A%2Fb|/public/A%2Fb?x=1|/public/A%2Fb?x=1", string(fctx.Response.Body()))
}

// Test_FiberHandler_RoutesTheURL pins that a Fiber handler served from
// net/http is routed on r.URL when it differs from r.RequestURI: after an
// http.StripPrefix in front of it, and for a request built in code, which
// has no RequestURI at all.
func Test_FiberHandler_RoutesTheURL(t *testing.T) {
	t.Parallel()

	h := func(c fiber.Ctx) error {
		return c.SendString(c.Path() + "|" + c.OriginalURL())
	}

	rec := httptest.NewRecorder()
	http.StripPrefix("/app", FiberHandler(h)).ServeHTTP(rec, httptest.NewRequest(fiber.MethodGet, "/app/users/%41?x=1", http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "/users/A|/users/%41?x=1", rec.Body.String())

	built, err := http.NewRequestWithContext(context.Background(), fiber.MethodGet, "http://example.com/built/%41?x=1", http.NoBody)
	require.NoError(t, err)
	require.Empty(t, built.RequestURI)
	rec = httptest.NewRecorder()
	FiberHandler(h).ServeHTTP(rec, built)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "/built/A|/built/%41?x=1", rec.Body.String())

	// As served by net/http, the two agree and the request line is used as is.
	rec = httptest.NewRecorder()
	FiberHandler(h).ServeHTTP(rec, httptest.NewRequest(fiber.MethodGet, "/served/%41?x=1", http.NoBody))
	require.Equal(t, "/served/A|/served/%41?x=1", rec.Body.String())
}

// Test_HTTPMiddleware_RejectsAuthorityTarget pins that a routed target
// fasthttp would read as an authority, one that begins with "//" and holds
// "://", is refused rather than set on the request, where the next parse
// would take the host and credentials from it.
func Test_HTTPMiddleware_RejectsAuthorityTarget(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(HTTPMiddleware(func(next http.Handler) http.Handler { return next }))
	app.Use(func(c fiber.Ctx) error {
		return c.SendString(c.Hostname() + "|" + c.Path())
	})

	require.Equal(t, fiber.StatusBadRequest, rawCtx(t, app, "//u:pw@evil.example/a:/x/..//b").Response.StatusCode())

	fctx := rawCtx(t, app, "//evil.example/a:/b")
	require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode())
	require.Equal(t, "example.com|//evil.example/a:/b", string(fctx.Response.Body()))
}

func Test_ConvertRequest_RejectsAuthorityTarget(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/*", func(c fiber.Ctx) error {
		r, err := ConvertRequest(c, true)
		if err != nil {
			return err
		}
		return c.SendString(r.URL.Path + "|" + c.Hostname())
	})

	require.Equal(t, fiber.StatusBadRequest, rawCtx(t, app, "//u:pw@evil.example/a:/x/..//b").Response.StatusCode())

	fctx := rawCtx(t, app, "//evil.example/a:/b")
	require.Equal(t, fiber.StatusOK, fctx.Response.StatusCode())
	require.Equal(t, "//evil.example/a:/b|example.com", string(fctx.Response.Body()))
}
