package static

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Hello, World!")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/not-found", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Not Found", string(body))
}

// go test -run Test_Static_Index
func Test_Static_Direct(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/*", New("../../.github"))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/index.html", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Hello, World!")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/index.html", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 405, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/testdata/testRoutes.json", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMEApplicationJSON, resp.Header.Get("Content-Type"))
	require.Empty(t, resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")

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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/index.html", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/index.html", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "no-cache, no-store, must-revalidate", resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")

	normalResp, normalErr := app.Test(httptest.NewRequest(fiber.MethodGet, "/config.yml", http.NoBody))
	require.NoError(t, normalErr, "app.Test(req)")
	require.Empty(t, normalResp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test.txt", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Empty(t, resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Hello, World!")

	require.NoError(t, os.Remove("../../.github/test.txt"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/test.txt", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Empty(t, resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Not Found", string(body))
}

func Test_Static_NotFoundHandler(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/*", New("../../.github", Config{
		NotFoundHandler: func(c fiber.Ctx) error {
			return c.SendString("Custom 404")
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/not-found", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/fiber.png", http.NoBody))
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
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))

	app := fiber.New()
	app.Get("/file", New(path, Config{Download: true}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/file", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	// The filename* ext-value uses RFC 8187 attr-char percent-encoding;
	// assert the literal encoding rather than deriving it from url.PathEscape,
	// which differs for bytes like '=', '@', ':'.
	expect := `attachment; filename="файл.txt"; filename*=UTF-8''%D1%84%D0%B0%D0%B9%D0%BB.txt`
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

	req := httptest.NewRequest(fiber.MethodGet, "/v1/v2", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
	require.Equal(t, "123", resp.Header.Get("Test-Header"))

	grp = app.Group("/v2")
	grp.Get("/v3*", New("../../.github/index.html"))

	req = httptest.NewRequest(fiber.MethodGet, "/v2/v3/john/doe", http.NoBody)
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

	req := httptest.NewRequest(fiber.MethodGet, "/yesyes/john/doe", http.NoBody)
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

	req := httptest.NewRequest(fiber.MethodGet, "/test/john/doe", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	app.Get("/my/nameisjohn*", New("../../.github/index.html"))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/my/nameisjohn/no/its/not", http.NoBody))
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

	req := httptest.NewRequest(fiber.MethodGet, "/john/index.html", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	app.Get("/prefix*", New("../../.github/testdata"))

	req = httptest.NewRequest(fiber.MethodGet, "/prefix/index.html", http.NoBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	app.Get("/single*", New("../../.github/testdata/testRoutes.json"))

	req = httptest.NewRequest(fiber.MethodGet, "/single", http.NoBody)
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

	req := httptest.NewRequest(fiber.MethodGet, "/john/", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	app.Get("/john_without_index*", New(testCSSDir))

	req = httptest.NewRequest(fiber.MethodGet, "/john_without_index/", http.NoBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	app.Use("/john", New("../../.github"))

	req = httptest.NewRequest(fiber.MethodGet, "/john/", http.NoBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	req = httptest.NewRequest(fiber.MethodGet, "/john", http.NoBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	app.Use("/john_without_index/", New(testCSSDir))

	req = httptest.NewRequest(fiber.MethodGet, "/john_without_index/", http.NoBody)
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
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
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
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/style.css", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")

	app = fiber.New()
	app.Get("/*", New(dir))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/style.css", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/static", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/style.css", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")

	app = fiber.New()
	app.Get("/static/*", New(dir, Config{
		Browse: true,
	}))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/style.css", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")

	app = fiber.New()
	app.Get("/static*", New(dir))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/style.css", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")

	app = fiber.New()
	app.Get("/static*", New(dir))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/style.css", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/css/style.css", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/dirfs", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "style.css")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/dirfs/style.css", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, fiber.MIMETextCSSCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/dirfs/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/dirfs/test/style2.css", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, fiber.MIMETextCSSCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/embed", http.NoBody))
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

	req := httptest.NewRequest(fiber.MethodGet, "/test/john/doe", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Test file")
}

func Test_Static_FS_RootDirectoryEnforced(t *testing.T) {
	t.Parallel()

	expectedIndexBody, err := os.ReadFile(filepath.Clean("../../.github/testdata/fs/index.html"))
	require.NoError(t, err)

	expectedCSSBody, err := os.ReadFile(filepath.Clean("../../.github/testdata/fs/css/style.css"))
	require.NoError(t, err)

	newApp := func() *fiber.App {
		app := fiber.New()
		app.Get("/static*", New("fs", Config{
			FS: os.DirFS("../../.github/testdata"),
		}))

		return app
	}

	successCases := []struct {
		name       string
		target     string
		wantBody   string
		wantType   string
		wantStatus int
	}{
		{name: "index file", target: "/static/index.html", wantStatus: fiber.StatusOK, wantBody: string(expectedIndexBody), wantType: fiber.MIMETextHTMLCharsetUTF8},
		{name: "empty path", target: "/static", wantStatus: fiber.StatusOK, wantBody: string(expectedIndexBody), wantType: fiber.MIMETextHTMLCharsetUTF8},
		{name: "trailing slash", target: "/static/", wantStatus: fiber.StatusOK, wantBody: string(expectedIndexBody), wantType: fiber.MIMETextHTMLCharsetUTF8},
		{name: "nested file", target: "/static/css/style.css", wantStatus: fiber.StatusOK, wantBody: string(expectedCSSBody), wantType: fiber.MIMETextCSSCharsetUTF8},
	}

	for _, tc := range successCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp, err := newApp().Test(httptest.NewRequest(fiber.MethodGet, tc.target, http.NoBody))
			require.NoError(t, err, "app.Test(req)")
			require.Equal(t, tc.wantStatus, resp.StatusCode, "Status code")
			require.Equal(t, tc.wantType, resp.Header.Get(fiber.HeaderContentType))

			responseBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, tc.wantBody, string(responseBody))
		})
	}

	traversalCases := []struct {
		name   string
		target string
	}{
		{name: "path prefix manipulation", target: "/static/fs/index.html"},
		{name: "raw traversal to root index", target: "/static/../index.html"},
		{name: "raw traversal to external file", target: "/static/../testRoutes.json"},
		{name: "nested raw traversal", target: "/static/css/../../testRoutes.json"},
		{name: "encoded parent traversal", target: "/static/%2E%2E/index.html"},
		{name: "encoded parent to external file", target: "/static/%2E%2E/testRoutes.json"},
		{name: "lowercase encoded parent", target: "/static/%2e%2e/testRoutes.json"},
		{name: "double encoded parent", target: "/static/%252E%252E/testRoutes.json"},
		{name: "mixed dot encoding", target: "/static/%2E./testRoutes.json"},
		{name: "encoded slash", target: "/static/..%2Findex.html"},
		{name: "encoded slash to external file", target: "/static/..%2FtestRoutes.json"},
		{name: "nested encoded slash traversal", target: "/static/css/%2E%2E/%2E%2E/testRoutes.json"},
		{name: "encoded backslash", target: "/static/..%5Cindex.html"},
		{name: "encoded backslash to external file", target: "/static/..%5CtestRoutes.json"},
		{name: "nested encoded backslash traversal", target: "/static/css%5C..%5C..%5CtestRoutes.json"},
		{name: "mixed separator traversal", target: "/static/css/..%5C..%5CtestRoutes.json"},
		{name: "repeated parent segments", target: "/static/%2E%2E/%2E%2E/testRoutes.json"},
		{name: "null byte", target: "/static/%00"},
	}

	for _, tc := range traversalCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp, err := newApp().Test(httptest.NewRequest(fiber.MethodGet, tc.target, http.NoBody))
			require.NoError(t, err, "app.Test(req)")
			require.Equal(t, fiber.StatusNotFound, resp.StatusCode, "Status code")

			responseBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, "Not Found", string(responseBody))
		})
	}
}

func Test_Static_FS_MissingRootDoesNotFallback(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/static*", New("missing", Config{
		FS: os.DirFS("../../.github/testdata"),
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/static/index.html", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode, "Status code")
}

func Test_Static_FS_RootLeadingSlash(t *testing.T) {
	t.Parallel()

	expectedIndexBody, err := os.ReadFile(filepath.Clean("../../.github/testdata/fs/index.html"))
	require.NoError(t, err)

	expectedCSSBody, err := os.ReadFile(filepath.Clean("../../.github/testdata/fs/css/style.css"))
	require.NoError(t, err)

	// A leading slash is not a valid io/fs path, so "/" and "/css" must be
	// treated as the fs root and the "css" subdirectory respectively instead
	// of sending every request to PathNotFound.
	cases := []struct {
		name     string
		root     string
		target   string
		wantBody string
		wantType string
	}{
		{name: "slash root serves index", root: "/", target: "/index.html", wantBody: string(expectedIndexBody), wantType: fiber.MIMETextHTMLCharsetUTF8},
		{name: "slash root serves nested", root: "/", target: "/css/style.css", wantBody: string(expectedCSSBody), wantType: fiber.MIMETextCSSCharsetUTF8},
		{name: "slash subdir serves file", root: "/css", target: "/style.css", wantBody: string(expectedCSSBody), wantType: fiber.MIMETextCSSCharsetUTF8},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Use("/", New(tc.root, Config{
				FS: os.DirFS("../../.github/testdata/fs"),
			}))

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, tc.target, http.NoBody))
			require.NoError(t, err, "app.Test(req)")
			require.Equal(t, fiber.StatusOK, resp.StatusCode, "Status code")
			require.Equal(t, tc.wantType, resp.Header.Get(fiber.HeaderContentType))

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, tc.wantBody, string(body))
		})
	}
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
	dir := "../../.github/testdata/fs"
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
			req := httptest.NewRequest(fiber.MethodGet, "/css/style.css", http.NoBody)
			req.Header.Set("Accept-Encoding", algo)
			resp, err := app.Test(req, testConfig)

			require.NoError(t, err, "app.Test(req)")
			require.Equal(t, 200, resp.StatusCode, "Status code")
			require.Empty(t, resp.Header.Get(fiber.HeaderContentEncoding))
			require.Equal(t, "46", resp.Header.Get(fiber.HeaderContentLength))

			// request compressible file, ContentLength will change
			req = httptest.NewRequest(fiber.MethodGet, "/index.html", http.NoBody)
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
	req := httptest.NewRequest(fiber.MethodGet, "/index.html", http.NoBody)
	resp, err := app.Test(req, testConfig)

	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Empty(t, resp.Header.Get(fiber.HeaderContentEncoding))
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

		req = httptest.NewRequest(fiber.MethodGet, "/"+fileName, http.NoBody)
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

		req := httptest.NewRequest(fiber.MethodGet, "/"+fileName, http.NoBody)
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/static/style.css", http.NoBody))
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
	validReq := httptest.NewRequest(fiber.MethodGet, "/style.css", http.NoBody)
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
		req := httptest.NewRequest(fiber.MethodGet, path, http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err, "app.Test(req)")

		status := resp.StatusCode
		require.Truef(t, status == 400 || status == 404,
			"Status code for path traversal %s should be 400 or 404, got %d", path, status)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// If we got a 404, we expect the "Not Found" message because that's how fiber handles NotFound by default.
		if status == 404 {
			require.Contains(t, string(body), "Not Found",
				"Blocked traversal should have a \"Not Found\" message for %s", path)
		} else {
			require.Contains(t, string(body), "Are you a hacker?",
				"Blocked traversal should have a \"Not Found\" message for %s", path)
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
	assertTraversalBlocked("/%5c..%5c..%5cetc%5cpasswd")
	assertTraversalBlocked("/%255c..%255c..%255cetc%255cpasswd")
	assertTraversalBlocked("/..%5c..%5cetc%5cpasswd")
	assertTraversalBlocked("/%2e%2e%5c%2e%2e%5cetc%5cpasswd")
	assertTraversalBlocked("/%2e%2e%2f%2e%2e%5cetc%5cpasswd")
	assertTraversalBlocked("/%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd")
	assertTraversalBlocked("/.%2e/.%2e/etc/passwd")
	assertTraversalBlocked("/..%2f..%2f..%2f..%2fetc%2fpasswd")
	assertTraversalBlocked("/%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd")
	assertTraversalBlocked("/%2e%2e%2f%2e%2e%2fetc%2fshadow")
	assertTraversalBlocked("/%2e%2e/%2e%2e/var/log/auth.log")
	assertTraversalBlocked("/..%2f..%2fvar%2flog%2fauth.log")
	assertTraversalBlocked("/%2e%2e//%2e%2e//etc/passwd")
	assertTraversalBlocked("/..//..//etc/passwd")
	assertTraversalBlocked("/%2e%2e%2f%2e%2e%2f%2e%2e%2fproc%2fself%2fenviron")
	assertTraversalBlocked("/%2e%2e%2f%2e%2e%2f%2e%2e%2froot%2f.ssh%2fauthorized_keys")
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
	validReq := httptest.NewRequest(fiber.MethodGet, "/style.css", http.NoBody)
	validResp, err := app.Test(validReq)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, validResp.StatusCode, "Status code for valid file on Windows")
	body, err := io.ReadAll(validResp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Contains(t, string(body), "color")

	// Helper to test blocked responses
	assertTraversalBlocked := func(path string) {
		req := httptest.NewRequest(fiber.MethodGet, path, http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err, "app.Test(req)")

		// We expect a blocked request to return either 400 or 404
		status := resp.StatusCode
		require.Containsf(t, []int{400, 404}, status,
			"Status code for path traversal %s should be 400 or 404, got %d", path, status)

		// If it's a 404, we expect a "Not Found" message
		if status == 404 {
			respBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Contains(t, string(respBody), "Not Found",
				"Blocked traversal should have a \"Not Found\" message for %s", path)
		} else {
			require.Contains(t, string(body), "Are you a hacker?",
				"Blocked traversal should have a \"Not Found\" message for %s", path)
		}
	}

	// Windows-specific traversal attempts
	// Backslashes are treated as directory separators on Windows.
	assertTraversalBlocked("/..\\index.html")
	assertTraversalBlocked("/..\\..\\index.html")
	assertTraversalBlocked("/..\\..\\..\\Windows\\win.ini")
	assertTraversalBlocked("/..\\..\\..\\Windows\\System32\\drivers\\etc\\hosts")
	assertTraversalBlocked("/%5C..%5C..%5CWindows%5Cwin.ini")
	assertTraversalBlocked("/%255C..%255C..%255CWindows%255Cwin.ini")
	assertTraversalBlocked("/%5c..%5c..%5cWindows%5cSystem32%5cdrivers%5cetc%5chosts")
	assertTraversalBlocked("/C:\\Windows\\System32\\cmd.exe")
	assertTraversalBlocked("/C:%5CWindows%5CSystem32%5Ccmd.exe")
	assertTraversalBlocked("/%43:%5CWindows%5CSystem32%5Ccmd.exe")
	assertTraversalBlocked("/%5c%5cserver%5cshare%5csecret.txt")
	assertTraversalBlocked("//server\\share\\secret.txt")
	assertTraversalBlocked("//server/share/secret.txt")
	assertTraversalBlocked("/%2F%2Fserver%2Fshare%2Fsecret.txt")

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

	// Check behavior on an obviously nonexistent and suspicious file
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
				if _, err := sanitizePath(path, filesystem, true); err != nil {
					b.Fatal(err)
				}
			}
		})
	}

	bench("nilFS - decoded chars", nil, []byte("/foo/bar/../baz qux/index.html"))
	bench("dirFS - decoded chars", os.DirFS("."), []byte("/foo/bar/../baz qux/index.html"))
	bench("nilFS - escapes", nil, []byte("/foo/bar/../baz%20qux/photo%402x.png"))
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
		{name: "current dir reference", input: []byte("/foo/./bar.txt"), expectPath: "/foo/bar.txt"},
		{name: "empty path", input: []byte(""), expectPath: "/"},
		{name: "dot segments", input: []byte("/foo/./bar/../baz.txt"), expectPath: "/foo/baz.txt"},
		{name: "leading dot segment", input: []byte("/./foo/bar.txt"), expectPath: "/foo/bar.txt"},
		{name: "decoded space", input: []byte("/foo bar/baz.txt"), expectPath: "/foo bar/baz.txt"},
		{name: "plus literal", input: []byte("/foo+bar/baz.txt"), expectPath: "/foo+bar/baz.txt"},
		// The router leaves reserved characters, the percent sign and any
		// escape a Ctx.Path override carries encoded; they are decoded here
		// exactly once to obtain the file name.
		{name: "encoded space", input: []byte("/foo%20bar/baz.txt"), expectPath: "/foo bar/baz.txt"},
		{name: "encoded reserved character", input: []byte("/photo%402x.png"), expectPath: "/photo@2x.png"},
		{name: "encoded percent sign", input: []byte("/100%25.txt"), expectPath: "/100%.txt"},
		{name: "percent sign decoded once", input: []byte("/%2570rivate/secret.txt"), expectPath: "/%70rivate/secret.txt"},
		{name: "double encoded traversal stays a name", input: []byte("/%252e%252e/bar.txt"), expectPath: "/%2e%2e/bar.txt"},
		{name: "encoded traversal is cleaned", input: []byte("/foo/%2e%2e/bar.txt"), expectPath: "/bar.txt"},
		{name: "literal percent", input: []byte("/100%.txt"), expectPath: "/100%.txt"},
		{name: "trailing percent", input: []byte("/foo%"), expectPath: "/foo%"},
		{name: "malformed escape kept", input: []byte("/a%zzb.txt"), expectPath: "/a%zzb.txt"},
		{name: "trailing slash preserved", input: []byte("/foo/bar/"), expectPath: "/foo/bar/"},
		{filesystem: os.DirFS("."), name: "filesystem empty path", input: []byte(""), expectPath: "/"},
		{filesystem: os.DirFS("."), name: "filesystem literal percent", input: []byte("/100%.txt"), expectPath: "/100%.txt"},
		{filesystem: os.DirFS("."), name: "filesystem trailing slash", input: []byte("/foo/"), expectPath: "/foo/"},
		{filesystem: os.DirFS("."), name: "filesystem traversal clean", input: []byte("/foo/../bar.txt"), expectPath: "/bar.txt"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := sanitizePath(tc.input, tc.filesystem, true)
			require.NoError(t, err)
			require.Equal(t, tc.expectPath, string(got))
		})
	}
}

// Test_SanitizePath_NoDecode covers the UnescapePath configuration, where the
// router decoded every escape and a "%" in the routed path is a literal
// character that must not be decoded a second time.
func Test_SanitizePath_NoDecode(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		"/%70rivate/secret.txt": "/%70rivate/secret.txt",
		"/100%.txt":             "/100%.txt",
		"/foo%2Fbar.txt":        "/foo%2Fbar.txt",
		"/foo bar/baz.txt":      "/foo bar/baz.txt",
	} {
		got, err := sanitizePath([]byte(input), nil, false)
		require.NoError(t, err, "input=%q", input)
		require.Equal(t, want, string(got), "input=%q", input)
	}

	for _, input := range []string{"/foo\\bar.txt", "/foo/bar.txt\x00", "/foo\x1fbar.txt"} {
		_, err := sanitizePath([]byte(input), nil, false)
		require.ErrorIs(t, err, ErrInvalidPath, "input=%q", input)
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
		// A file name must not contain a separator produced by decoding, a
		// backslash or a control character, whether decoded or sent raw. The
		// router treats none of these as a separator, so the file server must
		// not either: "/..\\private/x" is one name to the router and stays one
		// here rather than becoming "/../private/x".
		{name: "null byte", input: []byte("/foo/bar.txt\x00")},
		{name: "encoded null byte", input: []byte("/foo/bar.txt%00")},
		{name: "control character", input: []byte("/foo\x1fbar.txt")},
		{name: "encoded control character", input: []byte("/foo%0Abar.txt")},
		{name: "encoded slash", input: []byte("/foo%2Fbar.txt")},
		{name: "encoded trailing slash", input: []byte("/foo/bar%2F")},
		{name: "encoded backslash", input: []byte("/foo%5C..%5Cbar.txt")},
		{name: "backslash", input: []byte("/foo\\bar.txt")},
		{name: "backslash traversal", input: []byte("/..\\private/secret.txt")},
		{name: "backslash path", input: []byte("\\foo\\bar.txt")},
		{name: "backslash absolute", input: []byte("/\\Windows\\System32\\drivers\\etc\\hosts")},
		{name: "backslash unc path", input: []byte("\\\\server\\share\\secret.txt")},
		{name: "backslash drive letter", input: []byte("/C:\\Windows\\System32\\cmd.exe")},
		{name: "double slash path", input: []byte("//foo//bar.txt")},
		{name: "triple slash path", input: []byte("///server/share/secret.txt")},
		{name: "drive letter", input: []byte("C:/Windows/System32/cmd.exe")},
		{name: "drive letter with leading slash", input: []byte("/C:/Windows/System32/cmd.exe")},
		{name: "unc path", input: []byte("//server/share/secret.txt")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := sanitizePath(tc.input, tc.filesystem, true)
			require.ErrorIs(t, err, ErrInvalidPath, "Expected ErrInvalidPath for input: %s", tc.input)
		})
	}
}

func Test_HasParentDirSegment(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"/../secret.txt", "/foo/../../secret.txt", "../secret.txt"} {
		require.True(t, hasParentDirSegment(input), "Expected a parent segment in: %s", input)
	}

	// Only a "/"-separated ".." is a parent segment: a backslash is an ordinary
	// character to the router, and an escape left by it is a literal name.
	for _, input := range []string{"/foo/secret.txt", "/..foo/secret.txt", "/foo\\..\\secret.txt", "/%2e%2e/secret.txt", "/%252e%252e/secret.txt"} {
		require.False(t, hasParentDirSegment(input), "Expected no parent segment in: %s", input)
	}
}

// Test_Static_ServesRoutedPath pins that the file server resolves the path the
// router matched, so middleware mounted on "/static/private" guards every
// spelling of a path under it. The router normalizes the request per RFC 3986
// before matching, which folds "%70rivate", "x/../private", "./private" and
// "//private" into "private", and the file server decodes what the router left
// encoded exactly once: "%2570rivate" is a literal name, "%2F" never becomes a
// separator, and "100%25.txt" and "photo%402x.png" reach their files.
func Test_Static_ServesRoutedPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "private"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "private", "secret.txt"), []byte("SECRET"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "public.txt"), []byte("PUBLIC"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "hello world.txt"), []byte("HELLO"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "100%.txt"), []byte("PERCENT"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "photo@2x.png"), []byte("PHOTO"), 0o600))

	mounts := []struct {
		mount func(app *fiber.App)
		name  string
	}{
		{name: "get wildcard", mount: func(app *fiber.App) { app.Get("/static/*", New(root)) }},
		{name: "use prefix", mount: func(app *fiber.App) { app.Use("/static", New(root)) }},
		{name: "fs root", mount: func(app *fiber.App) { app.Get("/static/*", New("", Config{FS: os.DirFS(root)})) }},
		// A subdirectory root goes through hasParentDirSegment as well.
		{name: "fs subdirectory root", mount: func(app *fiber.App) {
			app.Get("/static/*", New(filepath.Base(root), Config{FS: os.DirFS(filepath.Dir(root))}))
		}},
	}

	requests := []struct {
		name       string
		target     string
		wantBody   string
		wantStatus int
		unescape   bool
	}{
		{name: "public file", target: "/static/public.txt", wantStatus: fiber.StatusOK, wantBody: "PUBLIC"},
		{name: "guarded file", target: "/static/private/secret.txt", wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "single encoding serves the decoded name", target: "/static/hello%20world.txt", wantStatus: fiber.StatusOK, wantBody: "HELLO"},
		{name: "literal percent in the name", target: "/static/100%25.txt", wantStatus: fiber.StatusOK, wantBody: "PERCENT"},
		{name: "reserved character in the name", target: "/static/photo%402x.png", wantStatus: fiber.StatusOK, wantBody: "PHOTO"},
		{name: "double encoding is not decoded again", target: "/static/hello%2520world.txt", wantStatus: fiber.StatusNotFound},
		{name: "encoded guarded segment", target: "/static/%70rivate/secret.txt", wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "encoded slash into the guarded directory", target: "/static/private%2Fsecret.txt", wantStatus: fiber.StatusNotFound},
		{name: "parent segment into the guarded directory", target: "/static/x/../private/secret.txt", wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "current segment into the guarded directory", target: "/static/./private/secret.txt", wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "empty segment into the guarded directory", target: "/static//private/secret.txt", wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "encoded backslash traversal into the guarded directory", target: "/static/..%5Cprivate/secret.txt", wantStatus: fiber.StatusNotFound},
		{name: "double encoded guarded segment", target: "/static/%2570rivate/secret.txt", wantStatus: fiber.StatusNotFound},
		{name: "triple encoded guarded segment", target: "/static/%252570rivate/secret.txt", wantStatus: fiber.StatusNotFound},
		{name: "unescaped routing encoded guarded segment", target: "/static/%70rivate/secret.txt", unescape: true, wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "unescaped routing encoded slash into the guarded directory", target: "/static/private%2Fsecret.txt", unescape: true, wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "unescaped routing parent segment into the guarded directory", target: "/static/x/../private/secret.txt", unescape: true, wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "unescaped routing encoded backslash traversal", target: "/static/..%5Cprivate/secret.txt", unescape: true, wantStatus: fiber.StatusNotFound},
		{name: "unescaped routing double encoded guarded segment", target: "/static/%2570rivate/secret.txt", unescape: true, wantStatus: fiber.StatusNotFound},
		{name: "unescaped routing reserved character in the name", target: "/static/photo%402x.png", unescape: true, wantStatus: fiber.StatusOK, wantBody: "PHOTO"},
	}

	for _, m := range mounts {
		for _, tc := range requests {
			t.Run(m.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				app := fiber.New(fiber.Config{UnescapePath: tc.unescape})
				app.Use("/static/private", func(c fiber.Ctx) error {
					return c.SendStatus(fiber.StatusForbidden)
				})
				m.mount(app)

				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, tc.target, http.NoBody))
				require.NoError(t, err, "app.Test(req)")
				require.Equal(t, tc.wantStatus, resp.StatusCode, "Status code")

				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				if tc.wantBody != "" {
					require.Equal(t, tc.wantBody, string(body))
				}
				if tc.wantStatus != fiber.StatusOK {
					require.NotContains(t, string(body), "SECRET")
					require.NotContains(t, string(body), "HELLO")
					require.NotContains(t, string(body), "PERCENT")
					require.NotContains(t, string(body), "PHOTO")
				}
			})
		}
	}
}

// Test_Static_RawBackslashIsNotASeparator sends a request line httptest would
// not produce on its own, since net/url encodes a backslash. The router reads
// a backslash as an ordinary character, so "/static/..\private/secret.txt" is
// one segment that no guard on "/static/private" matches; the file server must
// therefore not turn it into "/../private/secret.txt" and serve the guarded
// file, and it must not escape the root either.
func Test_Static_RawBackslashIsNotASeparator(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "private"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "private", "secret.txt"), []byte("SECRET"), 0o600))

	// rawRequest keeps the request line verbatim: app.Test writes the URL's
	// opaque form as given rather than the escaped form of the parsed path.
	rawRequest := func(target string) *http.Request {
		req := httptest.NewRequest(fiber.MethodGet, "/static/", http.NoBody)
		req.URL.Opaque = target
		return req
	}

	for _, unescape := range []bool{false, true} {
		app := fiber.New(fiber.Config{UnescapePath: unescape})
		app.Use("/static/private", func(c fiber.Ctx) error {
			return c.SendStatus(fiber.StatusForbidden)
		})
		app.Get("/static/*", New(root))

		for _, target := range []string{
			`/static/..\private/secret.txt`,
			`/static/x\..\private/secret.txt`,
			`/static/private\secret.txt`,
			`/static/..\..\etc\passwd`,
		} {
			resp, err := app.Test(rawRequest(target))
			require.NoError(t, err, "app.Test(req)")
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusNotFound, resp.StatusCode, "unescape=%v target=%s", unescape, target)
			require.NotContains(t, string(body), "SECRET", "unescape=%v target=%s", unescape, target)
		}

		resp, err := app.Test(rawRequest("/static/private/secret.txt"))
		require.NoError(t, err, "app.Test(req)")
		require.Equal(t, fiber.StatusForbidden, resp.StatusCode, "unescape=%v", unescape)
	}
}

func Test_Static_Download_NotFoundLeavesNoAttachment(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/files*", New("../../.github/testdata/fs", Config{Download: true}))
	app.Get("/files/users", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/files/users", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, fiber.MIMEApplicationJSONCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
	require.Empty(t, resp.Header.Get(fiber.HeaderContentDisposition))
}

func Test_Static_Download_KeepsDetectedContentType(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	// fiberpng is a PNG without an extension, so only content detection can type it.
	app.Get("/docs*", New("../../.github/testdata/fs/img", Config{Download: true}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/docs/fiberpng", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get(fiber.HeaderContentType), "image/png")
	require.Equal(t, `attachment; filename="fiberpng"`, resp.Header.Get(fiber.HeaderContentDisposition))
}

func Test_Static_Download_NoAttachmentOnRangeError(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/docs*", New("../../.github/testdata/fs/img", Config{Download: true, ByteRange: true}))

	req := httptest.NewRequest(fiber.MethodGet, "/docs/fiberpng", http.NoBody)
	req.Header.Set(fiber.HeaderRange, "bytes=99999999-99999999")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusRequestedRangeNotSatisfiable, resp.StatusCode)
	require.Empty(t, resp.Header.Get(fiber.HeaderContentDisposition))
}

func Test_Static_SameHandlerUnderTwoPrefixes(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	handler := New("../../.github/testdata/fs")
	app.Get("/static/*", handler)
	app.Get("/s/*", handler)

	for _, path := range []string{"/static/index.html", "/s/index.html", "/static/index.html"} {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "GET %s", path)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "<h1>Hello, World!</h1>", "GET %s", path)
	}
}

func Test_Static_MaxAge_NotOnErrorResponses(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/static/*", New("../../.github/testdata/fs", Config{ByteRange: true, MaxAge: 3600}))

	req := httptest.NewRequest(fiber.MethodGet, "/static/index.html", http.NoBody)
	req.Header.Set(fiber.HeaderRange, "bytes=999999-9999999")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusRequestedRangeNotSatisfiable, resp.StatusCode)
	require.Empty(t, resp.Header.Get(fiber.HeaderCacheControl))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/static/index.html", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "public, max-age=3600", resp.Header.Get(fiber.HeaderCacheControl))
}

func Test_Static_NonGetMethod_PassesThrough(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use("/", New("../../.github"))
	app.Post("/index.html", func(c fiber.Ctx) error { return c.SendString("posted") })

	resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/index.html", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "posted", string(body))

	get, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/index.html", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, get.StatusCode)
	served, err := io.ReadAll(get.Body)
	require.NoError(t, err)
	require.Contains(t, string(served), "Hello, World!")
}
