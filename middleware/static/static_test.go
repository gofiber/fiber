package static

import (
	"embed"
	"io"
	"io/fs"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gofiber/fiber/v3"
)

const (
	winOS      = "windows"
	testCSSDir = "../../.github/testdata/fs/css"
)

var testConfig = fiber.TestConfig{
	Timeout:       10 * time.Second,
	FailOnTimeout: true,
}

// go test -run Test_Static_Index_Default
func Test_Static_Index_Default(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/prefix", New("../../.github/workflows"))

	app.Get("", New("../../.github/"))

	app.Get("test", New("", Config{
		IndexNames: []string{"index.html"},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Hello, World!")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/not-found", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Cannot GET /not-found", string(body))
}

// go test -run Test_Static_Index
func Test_Static_Direct(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/*", New("../../.github"))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/index.html", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Hello, World!")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/index.html", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 405, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/testdata/testRoutes.json", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMEApplicationJSON, resp.Header.Get("Content-Type"))
	require.Equal(t, "", resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "test_routes")
}

// go test -run Test_Static_MaxAge
func Test_Static_MaxAge(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/*", New("../../.github", Config{
		MaxAge: 100,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/index.html", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, "text/html; charset=utf-8", resp.Header.Get(fiber.HeaderContentType))
	require.Equal(t, "public, max-age=100", resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")
}

// go test -run Test_Static_Custom_CacheControl
func Test_Static_Custom_CacheControl(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/*", New("../../.github", Config{
		ModifyResponse: func(c fiber.Ctx) error {
			if strings.Contains(c.GetRespHeader("Content-Type"), "text/html") {
				c.Response().Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
			}
			return nil
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/index.html", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "no-cache, no-store, must-revalidate", resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")

	normalResp, normalErr := app.Test(httptest.NewRequest(fiber.MethodGet, "/config.yml", nil))
	require.NoError(t, normalErr, "app.Test(req)")
	require.Equal(t, "", normalResp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")
}

func Test_Static_Disable_Cache(t *testing.T) {
	// Skip on Windows. It's not possible to delete a file that is in use.
	if runtime.GOOS == winOS {
		t.SkipNow()
	}

	t.Parallel()

	app := fiber.New()

	file, err := os.Create("../../.github/test.txt")
	require.NoError(t, err)
	_, err = file.WriteString("Hello, World!")
	require.NoError(t, err)
	require.NoError(t, file.Close())

	// Remove the file even if the test fails
	defer func() {
		_ = os.Remove("../../.github/test.txt") //nolint:errcheck // not needed
	}()

	app.Get("/*", New("../../.github/", Config{
		CacheDuration: -1,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test.txt", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "", resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Hello, World!")

	require.NoError(t, os.Remove("../../.github/test.txt"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/test.txt", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "", resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Cannot GET /test.txt", string(body))
}

func Test_Static_NotFoundHandler(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/*", New("../../.github", Config{
		NotFoundHandler: func(c fiber.Ctx) error {
			return c.SendString("Custom 404")
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/not-found", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Custom 404", string(body))
}

// go test -run Test_Static_Download
func Test_Static_Download(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/fiber.png", New("../../.github/testdata/fs/img/fiber.png", Config{
		Download: true,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fiber.png", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, "image/png", resp.Header.Get(fiber.HeaderContentType))
	require.Equal(t, `attachment; filename="fiber.png"`, resp.Header.Get(fiber.HeaderContentDisposition))
}

func Test_Static_Download_NonASCII(t *testing.T) {
	// Skip on Windows. It's not possible to delete a file that is in use.
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	t.Parallel()

	dir := t.TempDir()
	fname := "файл.txt"
	path := filepath.Join(dir, fname)
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644)) //nolint:gosec // Not a concern

	app := fiber.New()
	app.Get("/file", New(path, Config{Download: true}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/file", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	expect := "attachment; filename=\"" + fname + "\"; filename*=UTF-8''" + url.PathEscape(fname)
	require.Equal(t, expect, resp.Header.Get(fiber.HeaderContentDisposition))
}

// go test -run Test_Static_Group
func Test_Static_Group(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	grp := app.Group("/v1", func(c fiber.Ctx) error {
		c.Set("Test-Header", "123")
		return c.Next()
	})

	grp.Get("/v2*", New("../../.github/index.html"))

	req := httptest.NewRequest(fiber.MethodGet, "/v1/v2", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
	require.Equal(t, "123", resp.Header.Get("Test-Header"))

	grp = app.Group("/v2")
	grp.Get("/v3*", New("../../.github/index.html"))

	req = httptest.NewRequest(fiber.MethodGet, "/v2/v3/john/doe", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
}

func Test_Static_Wildcard(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("*", New("../../.github/index.html"))

	req := httptest.NewRequest(fiber.MethodGet, "/yesyes/john/doe", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Test file")
}

func Test_Static_Prefix_Wildcard(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/test*", New("../../.github/index.html"))

	req := httptest.NewRequest(fiber.MethodGet, "/test/john/doe", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	app.Get("/my/nameisjohn*", New("../../.github/index.html"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/my/nameisjohn/no/its/not", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Test file")
}

func Test_Static_Prefix(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/john*", New("../../.github"))

	req := httptest.NewRequest(fiber.MethodGet, "/john/index.html", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	app.Get("/prefix*", New("../../.github/testdata"))

	req = httptest.NewRequest(fiber.MethodGet, "/prefix/index.html", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	app.Get("/single*", New("../../.github/testdata/testRoutes.json"))

	req = httptest.NewRequest(fiber.MethodGet, "/single", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMEApplicationJSON, resp.Header.Get(fiber.HeaderContentType))
}

func Test_Static_Trailing_Slash(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/john*", New("../../.github"))

	req := httptest.NewRequest(fiber.MethodGet, "/john/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	app.Get("/john_without_index*", New(testCSSDir))

	req = httptest.NewRequest(fiber.MethodGet, "/john_without_index/", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	app.Use("/john", New("../../.github"))

	req = httptest.NewRequest(fiber.MethodGet, "/john/", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	req = httptest.NewRequest(fiber.MethodGet, "/john", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	app.Use("/john_without_index/", New(testCSSDir))

	req = httptest.NewRequest(fiber.MethodGet, "/john_without_index/", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
}

func Test_Static_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/*", New("../../.github", Config{
		Next: func(c fiber.Ctx) bool {
			return c.Get("X-Custom-Header") == "skip"
		},
	}))

	app.Get("/*", func(c fiber.Ctx) error {
		return c.SendString("You've skipped app.Static")
	})

	t.Run("app.Static is skipped: invoking Get handler", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(fiber.MethodGet, "/", nil)
		req.Header.Set("X-Custom-Header", "skip")
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
		require.Equal(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "You've skipped app.Static")
	})

	t.Run("app.Static is not skipped: serving index.html", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(fiber.MethodGet, "/", nil)
		req.Header.Set("X-Custom-Header", "don't skip")
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
		require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "Hello, World!")
	})
}

func Test_Route_Static_Root(t *testing.T) {
	t.Parallel()

	dir := testCSSDir
	app := fiber.New()
	app.Get("/*", New(dir, Config{
		Browse: true,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/style.css", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")

	app = fiber.New()
	app.Get("/*", New(dir))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/style.css", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")
}

func Test_Route_Static_HasPrefix(t *testing.T) {
	t.Parallel()

	dir := testCSSDir
	app := fiber.New()
	app.Get("/static*", New(dir, Config{
		Browse: true,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/static", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/style.css", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")

	app = fiber.New()
	app.Get("/static/*", New(dir, Config{
		Browse: true,
	}))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/style.css", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")

	app = fiber.New()
	app.Get("/static*", New(dir))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/style.css", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")

	app = fiber.New()
	app.Get("/static*", New(dir))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/style.css", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")
}

func Test_Static_FS(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/*", New("", Config{
		FS:     os.DirFS("../../.github/testdata/fs"),
		Browse: true,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/css/style.css", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, fiber.MIMETextCSSCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")
}

/*func Test_Static_FS_DifferentRoot(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/*", New("fs", Config{
		FS:         os.DirFS("../../.github/testdata"),
		IndexNames: []string{"index2.html"},
		Browse:     true,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "<h1>Hello, World!</h1>")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/css/style.css", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, fiber.MIMETextCSSCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")
}*/

//go:embed static.go config.go
var fsTestFilesystem embed.FS

func Test_Static_FS_Browse(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/embed*", New("", Config{
		FS:     fsTestFilesystem,
		Browse: true,
	}))

	app.Get("/dirfs*", New("", Config{
		FS:     os.DirFS(testCSSDir),
		Browse: true,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/dirfs", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "style.css")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/dirfs/style.css", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, fiber.MIMETextCSSCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/embed", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "static.go")
}

func Test_Static_FS_Prefix_Wildcard(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/test*", New("index.html", Config{
		FS:         os.DirFS("../../.github"),
		IndexNames: []string{"not_index.html"},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/test/john/doe", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Test file")
}

func Test_isFile(t *testing.T) {
	t.Parallel()

	cases := []struct {
		filesystem fs.FS
		gotError   error
		name       string
		path       string
		expected   bool
	}{
		{
			name:       "file",
			path:       "index.html",
			filesystem: os.DirFS("../../.github"),
			expected:   true,
		},
		{
			name:       "file",
			path:       "index2.html",
			filesystem: os.DirFS("../../.github"),
			expected:   false,
			gotError:   fs.ErrNotExist,
		},
		{
			name:       "directory",
			path:       ".",
			filesystem: os.DirFS("../../.github"),
			expected:   false,
		},
		{
			name:       "directory",
			path:       "not_exists",
			filesystem: os.DirFS("../../.github"),
			expected:   false,
			gotError:   fs.ErrNotExist,
		},
		{
			name:       "directory",
			path:       ".",
			filesystem: os.DirFS(testCSSDir),
			expected:   false,
		},
		{
			name:       "file",
			path:       testCSSDir + "/style.css",
			filesystem: nil,
			expected:   true,
		},
		{
			name:       "file",
			path:       testCSSDir + "/style2.css",
			filesystem: nil,
			expected:   false,
			gotError:   fs.ErrNotExist,
		},
		{
			name:       "directory",
			path:       testCSSDir,
			filesystem: nil,
			expected:   false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c := c
			t.Parallel()

			actual, err := isFile(c.path, c.filesystem)
			require.ErrorIs(t, err, c.gotError)
			require.Equal(t, c.expected, actual)
		})
	}
}

func Test_Static_Compress(t *testing.T) {
	t.Parallel()
	dir := "../../.github/testdata/fs" //nolint:goconst // test
	app := fiber.New()
	app.Get("/*", New(dir, Config{
		Compress: true,
	}))

	// Note: deflate is not supported by fasthttp.FS
	algorithms := []string{"zstd", "gzip", "br"}

	for _, algo := range algorithms {
		t.Run(algo+"_compression", func(t *testing.T) {
			t.Parallel()
			// request non-compressible file (less than 200 bytes), Content Length will remain the same
			req := httptest.NewRequest(fiber.MethodGet, "/css/style.css", nil)
			req.Header.Set("Accept-Encoding", algo)
			resp, err := app.Test(req, testConfig)

			require.NoError(t, err, "app.Test(req)")
			require.Equal(t, 200, resp.StatusCode, "Status code")
			require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))
			require.Equal(t, "46", resp.Header.Get(fiber.HeaderContentLength))

			// request compressible file, ContentLength will change
			req = httptest.NewRequest(fiber.MethodGet, "/index.html", nil)
			req.Header.Set("Accept-Encoding", algo)
			resp, err = app.Test(req, testConfig)

			require.NoError(t, err, "app.Test(req)")
			require.Equal(t, 200, resp.StatusCode, "Status code")
			require.Equal(t, algo, resp.Header.Get(fiber.HeaderContentEncoding))
			require.Greater(t, "299", resp.Header.Get(fiber.HeaderContentLength))
		})
	}
}

func Test_Static_Compress_WithoutEncoding(t *testing.T) {
	t.Parallel()
	dir := "../../.github/testdata/fs"
	app := fiber.New()
	app.Get("/*", New(dir, Config{
		Compress:      true,
		CacheDuration: 1 * time.Second,
	}))

	// request compressible file without encoding
	req := httptest.NewRequest(fiber.MethodGet, "/index.html", nil)
	resp, err := app.Test(req, testConfig)

	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, "299", resp.Header.Get(fiber.HeaderContentLength))

	// request compressible file with different encodings
	algorithms := []string{"zstd", "gzip", "br"}
	fileSuffixes := map[string]string{
		"gzip": ".fiber.gz",
		"br":   ".fiber.br",
		"zstd": ".fiber.zst",
	}

	for _, algo := range algorithms {
		// Wait for cache to expire
		time.Sleep(2 * time.Second)
		fileName := "index.html"
		compressedFileName := dir + "/index.html" + fileSuffixes[algo]

		req = httptest.NewRequest(fiber.MethodGet, "/"+fileName, nil)
		req.Header.Set("Accept-Encoding", algo)
		resp, err = app.Test(req, testConfig)

		require.NoError(t, err, "app.Test(req)")
		require.Equal(t, 200, resp.StatusCode, "Status code")
		require.Equal(t, algo, resp.Header.Get(fiber.HeaderContentEncoding))
		require.Greater(t, "299", resp.Header.Get(fiber.HeaderContentLength))

		// verify suffixed file was created
		_, err := os.Stat(compressedFileName)
		require.NoError(t, err, "File should exist")
	}
}

func Test_Static_Compress_WithFileSuffixes(t *testing.T) {
	t.Parallel()
	dir := "../../.github/testdata/fs"
	fileSuffixes := map[string]string{
		"gzip": ".test.gz",
		"br":   ".test.br",
		"zstd": ".test.zst",
	}

	app := fiber.New(fiber.Config{
		CompressedFileSuffixes: fileSuffixes,
	})
	app.Get("/*", New(dir, Config{
		Compress:      true,
		CacheDuration: 1 * time.Second,
	}))

	// request compressible file with different encodings
	algorithms := []string{"zstd", "gzip", "br"}

	for _, algo := range algorithms {
		// Wait for cache to expire
		time.Sleep(2 * time.Second)
		fileName := "index.html"
		compressedFileName := dir + "/index.html" + fileSuffixes[algo]

		req := httptest.NewRequest(fiber.MethodGet, "/"+fileName, nil)
		req.Header.Set("Accept-Encoding", algo)
		resp, err := app.Test(req, testConfig)

		require.NoError(t, err, "app.Test(req)")
		require.Equal(t, 200, resp.StatusCode, "Status code")
		require.Equal(t, algo, resp.Header.Get(fiber.HeaderContentEncoding))
		require.Greater(t, "299", resp.Header.Get(fiber.HeaderContentLength))

		// verify suffixed file was created
		_, err = os.Stat(compressedFileName)
		require.NoError(t, err, "File should exist")
	}
}

func Test_Router_Mount_n_Static(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use("/static", New(testCSSDir, Config{Browse: true}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Home")
	})

	subApp := fiber.New()
	app.Use("/mount", subApp)
	subApp.Get("/test", func(c fiber.Ctx) error {
		return c.SendString("Hello from /test")
	})

	app.Use(func(c fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).SendString("Not Found")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/static/style.css", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
}

func Test_Static_PathTraversal(t *testing.T) {
	// Skip this test if running on Windows
	if runtime.GOOS == winOS {
		t.Skip("Skipping Windows-specific tests")
	}

	t.Parallel()
	app := fiber.New()

	// Serve only from testCSSDir
	// This directory should contain `style.css` but not `index.html` or anything above it.
	rootDir := testCSSDir
	app.Get("/*", New(rootDir))

	// A valid request: should succeed
	validReq := httptest.NewRequest(fiber.MethodGet, "/style.css", nil)
	validResp, err := app.Test(validReq)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, validResp.StatusCode, "Status code")
	require.Equal(t, fiber.MIMETextCSSCharsetUTF8, validResp.Header.Get(fiber.HeaderContentType))
	validBody, err := io.ReadAll(validResp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(validBody), "color")

	// Helper function to assert that a given path is blocked.
	// Blocked can mean different status codes depending on what triggered the block.
	// We'll accept 400 or 404 as "blocked" statuses:
	// - 404 is the expected blocked response in most cases.
	// - 400 might occur if fasthttp rejects the request before it's even processed (e.g., null bytes).
	assertTraversalBlocked := func(path string) {
		req := httptest.NewRequest(fiber.MethodGet, path, nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "app.Test(req)")

		status := resp.StatusCode
		require.Truef(t, status == 400 || status == 404,
			"Status code for path traversal %s should be 400 or 404, got %d", path, status)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// If we got a 404, we expect the "Cannot GET" message because that's how fiber handles NotFound by default.
		if status == 404 {
			require.Contains(t, string(body), "Cannot GET",
				"Blocked traversal should have a Cannot GET message for %s", path)
		} else {
			require.Contains(t, string(body), "Are you a hacker?",
				"Blocked traversal should have a Cannot GET message for %s", path)
		}
	}

	// Basic attempts to escape the directory
	assertTraversalBlocked("/index.html..")
	assertTraversalBlocked("/style.css..")
	assertTraversalBlocked("/../index.html")
	assertTraversalBlocked("/../../index.html")
	assertTraversalBlocked("/../../../index.html")

	// Attempts with double slashes
	assertTraversalBlocked("//../index.html")
	assertTraversalBlocked("/..//index.html")

	// Encoded attempts: `%2e` is '.' and `%2f` is '/'
	assertTraversalBlocked("/..%2findex.html")        // ../index.html
	assertTraversalBlocked("/%2e%2e/index.html")      // ../index.html
	assertTraversalBlocked("/%2e%2e%2f%2e%2e/secret") // ../../../secret

	// Mixed encoded and normal attempts
	assertTraversalBlocked("/%2e%2e/../index.html")  // ../../index.html
	assertTraversalBlocked("/..%2f..%2fsecret.json") // ../../../secret.json

	// Attempts with current directory references
	assertTraversalBlocked("/./../index.html")
	assertTraversalBlocked("/././../index.html")

	// Trailing slashes
	assertTraversalBlocked("/../")
	assertTraversalBlocked("/../../")

	// Attempts to load files from an absolute path outside the root
	assertTraversalBlocked("/" + rootDir + "/../../index.html")

	// Additional edge cases:

	// Double-encoded `..`
	assertTraversalBlocked("/%252e%252e/index.html") // double-encoded .. -> ../index.html after double decoding

	// Multiple levels of encoding and traversal
	assertTraversalBlocked("/%2e%2e%2F..%2f%2e%2e%2fWINDOWS")       // multiple ups and unusual pattern
	assertTraversalBlocked("/%2e%2e%2F..%2f%2e%2e%2f%2e%2e/secret") // more complex chain of ../

	// Null byte attempts
	assertTraversalBlocked("/index.html%00.jpg")
	assertTraversalBlocked("/%00index.html")
	assertTraversalBlocked("/somefolder%00/something")
	assertTraversalBlocked("/%00/index.html")

	// Attempts to access known system files
	assertTraversalBlocked("/etc/passwd")
	assertTraversalBlocked("/etc/")

	// Complex mixed attempts with encoded slashes and dots
	assertTraversalBlocked("/..%2F..%2F..%2F..%2Fetc%2Fpasswd")

	// Attempts inside subdirectories with encoded traversal
	assertTraversalBlocked("/somefolder/%2e%2e%2findex.html")
	assertTraversalBlocked("/somefolder/%2e%2e%2f%2e%2e%2findex.html")

	// Backslash encoded attempts
	assertTraversalBlocked("/%5C..%5Cindex.html")
}

func Test_Static_PathTraversal_WindowsOnly(t *testing.T) {
	// Skip this test if not running on Windows
	if runtime.GOOS != winOS {
		t.Skip("Skipping Windows-specific tests")
	}

	t.Parallel()
	app := fiber.New()

	// Serve only from testCSSDir
	rootDir := testCSSDir
	app.Get("/*", New(rootDir))

	// A valid request (relative path without backslash):
	validReq := httptest.NewRequest(fiber.MethodGet, "/style.css", nil)
	validResp, err := app.Test(validReq)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, validResp.StatusCode, "Status code for valid file on Windows")
	body, err := io.ReadAll(validResp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")

	// Helper to test blocked responses
	assertTraversalBlocked := func(path string) {
		req := httptest.NewRequest(fiber.MethodGet, path, nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "app.Test(req)")

		// We expect a blocked request to return either 400 or 404
		status := resp.StatusCode
		require.Containsf(t, []int{400, 404}, status,
			"Status code for path traversal %s should be 400 or 404, got %d", path, status)

		// If it's a 404, we expect a "Cannot GET" message
		if status == 404 {
			respBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Contains(t, string(respBody), "Cannot GET",
				"Blocked traversal should have a 'Cannot GET' message for %s", path)
		} else {
			require.Contains(t, string(body), "Are you a hacker?",
				"Blocked traversal should have a Cannot GET message for %s", path)
		}
	}

	// Windows-specific traversal attempts
	// Backslashes are treated as directory separators on Windows.
	assertTraversalBlocked("/..\\index.html")
	assertTraversalBlocked("/..\\..\\index.html")

	// Attempt with a path that might try to reference Windows drives or absolute paths
	// Note: These are artificial tests to ensure no drive-letter escapes are allowed.
	assertTraversalBlocked("/C:\\Windows\\System32\\cmd.exe")
	assertTraversalBlocked("/C:/Windows/System32/cmd.exe")

	// Attempt with UNC-like paths (though unlikely in a web context, good to test)
	assertTraversalBlocked("//server\\share\\secret.txt")

	// Attempt using a mixture of forward and backward slashes
	assertTraversalBlocked("/..\\..\\/index.html")

	// Attempt that includes a null-byte on Windows
	assertTraversalBlocked("/index.html%00.txt")

	// Check behavior on an obviously non-existent and suspicious file
	assertTraversalBlocked("/\\this\\path\\does\\not\\exist\\..")

	// Attempts involving relative traversal and current directory reference
	assertTraversalBlocked("/.\\../index.html")
	assertTraversalBlocked("/./..\\index.html")
}

func Benchmark_SanitizePath(b *testing.B) {
	bench := func(name string, filesystem fs.FS, path []byte) {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := sanitizePath(path, filesystem); err != nil {
					b.Fatal(err)
				}
			}
		})
	}

	bench("nilFS - urlencoded chars", nil, []byte("/foo%2Fbar/../baz%20qux/index.html"))
	bench("dirFS - urlencoded chars", os.DirFS("."), []byte("/foo%2Fbar/../baz%20qux/index.html"))
	bench("nilFS - slashes", nil, []byte("\\foo%2Fbar\\baz%20qux\\index.html"))
}

func Test_SanitizePath(t *testing.T) {
	t.Parallel()

	type testCase struct {
		filesystem fs.FS
		name       string
		expectPath string
		input      []byte
	}

	testCases := []testCase{
		{name: "simple path", input: []byte("/foo/bar.txt"), expectPath: "/foo/bar.txt"},
		{name: "traversal attempt", input: []byte("/foo/../../bar.txt"), expectPath: "/bar.txt"},
		{name: "encoded traversal", input: []byte("/foo/%2e%2e/bar.txt"), expectPath: "/bar.txt"},
		{name: "double encoded traversal", input: []byte("/%252e%252e/bar.txt"), expectPath: "/bar.txt"},
		{name: "current dir reference", input: []byte("/foo/./bar.txt"), expectPath: "/foo/bar.txt"},
		{name: "encoded slash", input: []byte("/foo%2Fbar.txt"), expectPath: "/foo/bar.txt"},
		{name: "empty path", input: []byte(""), expectPath: "/"},
		// windows-specific paths
		{name: "backslash path", input: []byte("\\foo\\bar.txt"), expectPath: "/foo/bar.txt"},
		{name: "backslash traversal", input: []byte("\\foo\\..\\..\\bar.txt"), expectPath: "/bar.txt"},
		{name: "mixed slashes", input: []byte("/foo\\bar.txt"), expectPath: "/foo/bar.txt"},
		{name: "encoded backslash traversal", input: []byte("/foo%5C..%5Cbar.txt"), expectPath: "/foo\\..\\bar.txt"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := sanitizePath(tc.input, tc.filesystem)
			require.NoError(t, err)
			require.Equal(t, tc.expectPath, string(got))
		})
	}
}

func Test_SanitizePath_Error(t *testing.T) {
	t.Parallel()

	type testCase struct {
		filesystem fs.FS
		name       string
		input      []byte
	}

	testCases := []testCase{
		{name: "null byte", input: []byte("/foo/bar.txt%00")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := sanitizePath(tc.input, tc.filesystem)
			require.Error(t, err, "Expected error for input: %s", tc.input)
		})
	}
}
