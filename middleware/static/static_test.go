package static

import (
	"embed"
	"io"
	"io/fs"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

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
	if runtime.GOOS == "windows" {
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
	require.Equal(t, `attachment`, resp.Header.Get(fiber.HeaderContentDisposition))
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

	app.Get("/john_without_index*", New("../../.github/testdata/fs/css"))

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

	app.Use("/john_without_index/", New("../../.github/testdata/fs/css"))

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

	dir := "../../.github/testdata/fs/css"
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

	dir := "../../.github/testdata/fs/css"
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
		FS:     os.DirFS("../../.github/testdata/fs/css"),
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
		name       string
		path       string
		filesystem fs.FS
		expected   bool
		gotError   error
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
			filesystem: os.DirFS("../../.github/testdata/fs/css"),
			expected:   false,
		},
		{
			name:       "file",
			path:       "../../.github/testdata/fs/css/style.css",
			filesystem: nil,
			expected:   true,
		},
		{
			name:       "file",
			path:       "../../.github/testdata/fs/css/style2.css",
			filesystem: nil,
			expected:   false,
			gotError:   fs.ErrNotExist,
		},
		{
			name:       "directory",
			path:       "../../.github/testdata/fs/css",
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
		algo := algo
		t.Run(algo+"_compression", func(t *testing.T) {
			t.Parallel()
			// request non-compressable file (less than 200 bytes), Content Lengh will remain the same
			req := httptest.NewRequest(fiber.MethodGet, "/css/style.css", nil)
			req.Header.Set("Accept-Encoding", algo)
			resp, err := app.Test(req, 10*time.Second)

			require.NoError(t, err, "app.Test(req)")
			require.Equal(t, 200, resp.StatusCode, "Status code")
			require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))
			require.Equal(t, "46", resp.Header.Get(fiber.HeaderContentLength))

			// request compressable file, ContentLenght will change
			req = httptest.NewRequest(fiber.MethodGet, "/index.html", nil)
			req.Header.Set("Accept-Encoding", algo)
			resp, err = app.Test(req, 10*time.Second)

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

	// request compressable file without encoding
	req := httptest.NewRequest(fiber.MethodGet, "/index.html", nil)
	resp, err := app.Test(req, 10*time.Second)

	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, "299", resp.Header.Get(fiber.HeaderContentLength))

	// request compressable file with different encodings
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
		resp, err = app.Test(req, 10*time.Second)

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

	// request compressable file with different encodings
	algorithms := []string{"zstd", "gzip", "br"}

	for _, algo := range algorithms {
		// Wait for cache to expire
		time.Sleep(2 * time.Second)
		fileName := "index.html"
		compressedFileName := dir + "/index.html" + fileSuffixes[algo]

		req := httptest.NewRequest(fiber.MethodGet, "/"+fileName, nil)
		req.Header.Set("Accept-Encoding", algo)
		resp, err := app.Test(req, 10*time.Second)

		require.NoError(t, err, "app.Test(req)")
		require.Equal(t, 200, resp.StatusCode, "Status code")
		require.Equal(t, algo, resp.Header.Get(fiber.HeaderContentEncoding))
		require.Greater(t, "299", resp.Header.Get(fiber.HeaderContentLength))

		// verify suffixed file was created
		_, err = os.Stat(compressedFileName)
		require.NoError(t, err, "File should exist")
	}
}
