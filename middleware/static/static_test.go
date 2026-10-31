package static

import (
	"bufio"
	"embed"
	"fmt"
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
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
)

const (
	winOS      = "windows"
	testCSSDir = "../../.github/testdata/fs/css"
)

var testConfig = fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true}

// discardLogger drops what the file server of fasthttp logs for every path it
// cannot open; the server of Fiber discards it as well.
type discardLogger struct{}

func (discardLogger) Printf(string, ...any) {}

// staticTestRoot returns a temporary directory that holds files, given as slash-separated paths below it and their
// contents.
func staticTestRoot(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for name, data := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
	}
	return root
}

// staticTestRead returns the content of the file at path.
func staticTestRead(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)
	return string(data)
}

// staticTestApp returns an app whose UnescapePath is unescape and which answers 403 on each of the guards, the paths
// a middleware protects in front of the file server mounted by the test.
func staticTestApp(unescape bool, guards ...string) *fiber.App {
	app := fiber.New(fiber.Config{UnescapePath: unescape})
	for _, guard := range guards {
		app.Use(guard, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusForbidden) })
	}
	return app
}

// staticTestExpect requests target from app and requires the status, a
// Content-Length header and the Content-Type contentType.
func staticTestExpect[T apptest.Target](t *testing.T, app *fiber.App, target T, status int, contentType string) *http.Response {
	t.Helper()

	resp := apptest.Expect(t, app, target, status)
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderContentLength))
	require.Equal(t, contentType, resp.Header.Get(fiber.HeaderContentType))
	return resp
}

func Test_Static_Index_Default(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/prefix", New("../../.github/workflows"))

	app.Get("", New("../../.github/"))

	app.Get("test", New("", Config{IndexNames: []string{"index.html"}}))

	resp := staticTestExpect(t, app, "/", 200, fiber.MIMETextHTMLCharsetUTF8)
	require.Contains(t, apptest.Body(t, resp), "Hello, World!")

	resp = staticTestExpect(t, app, "/not-found", 404, fiber.MIMETextPlainCharsetUTF8)
	require.Equal(t, "Not Found", apptest.Body(t, resp))
}

func Test_Static_Direct(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/*", New("../../.github"))

	resp := staticTestExpect(t, app, "/index.html", 200, fiber.MIMETextHTMLCharsetUTF8)
	require.Contains(t, apptest.Body(t, resp), "Hello, World!")

	staticTestExpect(t, app, apptest.Req(fiber.MethodPost, "/index.html"), 405, fiber.MIMETextPlainCharsetUTF8)

	resp = staticTestExpect(t, app, "/testdata/testRoutes.json", 200, fiber.MIMEApplicationJSON)
	require.Empty(t, resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")
	require.Contains(t, apptest.Body(t, resp), "test_routes")
}

func Test_Static_MaxAge(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/*", New("../../.github", Config{MaxAge: 100}))

	resp := staticTestExpect(t, app, "/index.html", 200, "text/html; charset=utf-8")
	require.Equal(t, "public, max-age=100", resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")
}

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

	resp := apptest.Do(t, app, "/index.html")
	require.Equal(t, "no-cache, no-store, must-revalidate", resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")

	normalResp := apptest.Do(t, app, "/config.yml")
	require.Empty(t, normalResp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")
}

func Test_Static_Disable_Cache(t *testing.T) {
	// Skip on Windows. It's not possible to delete a file that is in use.
	if runtime.GOOS == winOS {
		t.SkipNow()
	}

	t.Parallel()
	app := fiber.New()

	require.NoError(t, os.WriteFile("../../.github/test.txt", []byte("Hello, World!"), 0o600))

	// Remove the file even if the test fails
	defer func() {
		_ = os.Remove("../../.github/test.txt") //nolint:errcheck // not needed
	}()

	app.Get("/*", New("../../.github/", Config{CacheDuration: -1}))

	resp := apptest.Do(t, app, "/test.txt")
	require.Empty(t, resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")
	require.Contains(t, apptest.Body(t, resp), "Hello, World!")

	require.NoError(t, os.Remove("../../.github/test.txt"))

	resp = apptest.Do(t, app, "/test.txt")
	require.Empty(t, resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")
	require.Equal(t, "Not Found", apptest.Body(t, resp))
}

func Test_Static_NotFoundHandler(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/*", New("../../.github", Config{
		NotFoundHandler: func(c fiber.Ctx) error { return c.SendString("Custom 404") },
	}))

	resp := staticTestExpect(t, app, "/not-found", 404, fiber.MIMETextPlainCharsetUTF8)
	require.Equal(t, "Custom 404", apptest.Body(t, resp))
}

func Test_Static_Download(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/fiber.png", New("../../.github/testdata/fs/img/fiber.png", Config{Download: true}))

	resp := apptest.Expect(t, app, "/fiber.png", 200)
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

	resp := apptest.Expect(t, app, "/file", 200)
	// The filename* ext-value uses RFC 8187 attr-char percent-encoding;
	// assert the literal encoding rather than deriving it from url.PathEscape,
	// which differs for bytes like '=', '@', ':'.
	expect := `attachment; filename="файл.txt"; filename*=UTF-8''%D1%84%D0%B0%D0%B9%D0%BB.txt`
	require.Equal(t, expect, resp.Header.Get(fiber.HeaderContentDisposition))
}

func Test_Static_Group(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	grp := app.Group("/v1", func(c fiber.Ctx) error {
		c.Set("Test-Header", "123")
		return c.Next()
	})

	grp.Get("/v2*", New("../../.github/index.html"))

	resp := staticTestExpect(t, app, "/v1/v2", 200, fiber.MIMETextHTMLCharsetUTF8)
	require.Equal(t, "123", resp.Header.Get("Test-Header"))

	grp = app.Group("/v2")
	grp.Get("/v3*", New("../../.github/index.html"))

	staticTestExpect(t, app, "/v2/v3/john/doe", 200, fiber.MIMETextHTMLCharsetUTF8)
}

// Test_Static_Routes serves a file, or the files below a directory, from wildcard routes of different shapes.
func Test_Static_Routes(t *testing.T) {
	t.Parallel()
	const index, testdata = "../../.github/index.html", "../../.github/testdata"

	for _, tc := range []struct {
		name        string
		route       string
		root        string
		target      string
		contentType string
		body        string // must appear in the body, when set
		config      []Config
	}{
		{"bare wildcard", "*", index, "/yesyes/john/doe", fiber.MIMETextHTMLCharsetUTF8, "Test file", nil},
		{"prefix wildcard", "/test*", index, "/test/john/doe", fiber.MIMETextHTMLCharsetUTF8, "", nil},
		{
			"nested prefix wildcard", "/my/nameisjohn*", index, "/my/nameisjohn/no/its/not",
			fiber.MIMETextHTMLCharsetUTF8, "Test file", nil,
		},
		{"directory", "/john*", "../../.github", "/john/index.html", fiber.MIMETextHTMLCharsetUTF8, "", nil},
		{"subdirectory", "/prefix*", testdata, "/prefix/index.html", fiber.MIMETextHTMLCharsetUTF8, "", nil},
		{"single file", "/single*", testdata + "/testRoutes.json", "/single", fiber.MIMEApplicationJSON, "", nil},
		{
			"file of an fs.FS", "/test*", "index.html", "/test/john/doe", fiber.MIMETextHTMLCharsetUTF8, "Test file",
			[]Config{{FS: os.DirFS("../../.github"), IndexNames: []string{"not_index.html"}}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Get(tc.route, New(tc.root, tc.config...))

			resp := staticTestExpect(t, app, tc.target, 200, tc.contentType)
			if tc.body != "" {
				require.Contains(t, apptest.Body(t, resp), tc.body)
			}
		})
	}
}

func Test_Static_Trailing_Slash(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/john*", New("../../.github"))

	staticTestExpect(t, app, "/john/", 200, fiber.MIMETextHTMLCharsetUTF8)

	app.Get("/john_without_index*", New(testCSSDir))

	staticTestExpect(t, app, "/john_without_index/", 404, fiber.MIMETextPlainCharsetUTF8)

	app.Use("/john", New("../../.github"))

	staticTestExpect(t, app, "/john/", 200, fiber.MIMETextHTMLCharsetUTF8)

	staticTestExpect(t, app, "/john", 200, fiber.MIMETextHTMLCharsetUTF8)

	app.Use("/john_without_index/", New(testCSSDir))

	staticTestExpect(t, app, "/john_without_index/", 404, fiber.MIMETextPlainCharsetUTF8)
}

func Test_Static_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/*", New("../../.github", Config{
		Next: func(c fiber.Ctx) bool { return c.Get("X-Custom-Header") == "skip" },
	}))

	app.Get("/*", func(c fiber.Ctx) error { return c.SendString("You've skipped app.Static") })

	tests := []struct {
		name        string
		header      string
		contentType string
		body        string
	}{
		{name: "app.Static is skipped: invoking Get handler", header: "skip", contentType: fiber.MIMETextPlainCharsetUTF8, body: "You've skipped app.Static"},
		{name: "app.Static is not skipped: serving index.html", header: "don't skip", contentType: fiber.MIMETextHTMLCharsetUTF8, body: "Hello, World!"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeader("X-Custom-Header", tc.header))
			resp := staticTestExpect(t, app, req, 200, tc.contentType)
			require.Contains(t, apptest.Body(t, resp), tc.body)
		})
	}
}

// A refused request replays the Use chain without its routes, a guard registered as one included.
func Test_Static_RefusedRequestServesNoFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "admin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "admin", "secret.txt"), []byte("TOP-SECRET"), 0o600))

	app := fiber.New(fiber.Config{BodyLimit: 16})
	app.All("/admin/*", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusUnauthorized) })
	app.Use(New(dir))

	ln := fasthttputil.NewInmemoryListener()
	errCh := make(chan error, 1)
	go func() { errCh <- app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
		require.NoError(t, <-errCh)
	})

	conn, err := ln.Dial()
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck // closed by the server already
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	_, err = conn.Write([]byte("GET /admin/secret.txt HTTP/1.1\r\nHost: example.com\r\nContent-Length: 99999\r\n\r\n"))
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	body := apptest.Body(t, resp)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode)
	require.Equal(t, "Request Entity Too Large", body)
	require.Empty(t, resp.Header.Get(fiber.HeaderLastModified))
}

// Test_Route_Static serves a directory from the root route or from a route with a prefix, with and
// without browsing.
func Test_Route_Static(t *testing.T) {
	t.Parallel()
	browse := []Config{{Browse: true}}
	tests := []struct {
		name      string
		route     string
		prefix    string // the path of the directory the route serves, none for the root
		config    []Config
		dirStatus int
	}{
		{name: "root browse", route: "/*", config: browse, dirStatus: 200},
		{name: "root no browse", route: "/*", dirStatus: 404},
		{name: "prefix browse on a partial wildcard", route: "/static*", prefix: "/static", config: browse, dirStatus: 200},
		{name: "prefix browse on a slash wildcard", route: "/static/*", prefix: "/static", config: browse, dirStatus: 200},
		{name: "prefix no browse", route: "/static*", prefix: "/static", dirStatus: 404},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Get(tc.route, New(testCSSDir, tc.config...))

			if tc.prefix != "" {
				apptest.Expect(t, app, tc.prefix, tc.dirStatus)
			}
			apptest.Expect(t, app, tc.prefix+"/", tc.dirStatus)

			resp := apptest.Expect(t, app, tc.prefix+"/style.css", 200)
			require.Contains(t, apptest.Body(t, resp), "color")
		})
	}
}

func Test_Static_FS(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/*", New("", Config{FS: os.DirFS("../../.github/testdata/fs"), Browse: true}))

	resp := apptest.Expect(t, app, "/", 200)
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	resp = apptest.Expect(t, app, "/css/style.css", 200)
	require.Equal(t, fiber.MIMETextCSSCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
	require.Contains(t, apptest.Body(t, resp), "color")
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

	app.Get("/embed*", New("", Config{FS: fsTestFilesystem, Browse: true}))
	app.Get("/dirfs*", New("", Config{FS: os.DirFS(testCSSDir), Browse: true}))

	for _, tc := range []struct{ target, contentType, body string }{
		{"/dirfs", fiber.MIMETextHTMLCharsetUTF8, "style.css"},
		{"/dirfs/style.css", fiber.MIMETextCSSCharsetUTF8, "color"},
		{"/dirfs/test", fiber.MIMETextHTMLCharsetUTF8, ""}, // the listing, whatever it holds
		{"/dirfs/test/style2.css", fiber.MIMETextCSSCharsetUTF8, "color"},
		{"/embed", fiber.MIMETextHTMLCharsetUTF8, "static.go"},
	} {
		resp := apptest.Expect(t, app, tc.target, 200)
		require.Equal(t, tc.contentType, resp.Header.Get(fiber.HeaderContentType), tc.target)
		require.Contains(t, apptest.Body(t, resp), tc.body, tc.target)
	}
}

func Test_Static_FS_RootDirectoryEnforced(t *testing.T) {
	t.Parallel()
	expectedIndexBody := staticTestRead(t, "../../.github/testdata/fs/index.html")
	expectedCSSBody := staticTestRead(t, "../../.github/testdata/fs/css/style.css")

	newApp := func() *fiber.App {
		app := fiber.New()
		app.Get("/static*", New("fs", Config{FS: os.DirFS("../../.github/testdata")}))

		return app
	}

	successCases := []struct {
		name       string
		target     string
		wantBody   string
		wantType   string
		wantStatus int
	}{
		{name: "index file", target: "/static/index.html", wantStatus: fiber.StatusOK, wantBody: expectedIndexBody, wantType: fiber.MIMETextHTMLCharsetUTF8},
		{name: "empty path", target: "/static", wantStatus: fiber.StatusOK, wantBody: expectedIndexBody, wantType: fiber.MIMETextHTMLCharsetUTF8},
		{name: "trailing slash", target: "/static/", wantStatus: fiber.StatusOK, wantBody: expectedIndexBody, wantType: fiber.MIMETextHTMLCharsetUTF8},
		{name: "nested file", target: "/static/css/style.css", wantStatus: fiber.StatusOK, wantBody: expectedCSSBody, wantType: fiber.MIMETextCSSCharsetUTF8},
	}

	for _, tc := range successCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := apptest.Expect(t, newApp(), tc.target, tc.wantStatus)
			require.Equal(t, tc.wantType, resp.Header.Get(fiber.HeaderContentType))

			require.Equal(t, tc.wantBody, apptest.Body(t, resp))
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
			apptest.ExpectBody(t, newApp(), tc.target, fiber.StatusNotFound, "Not Found")
		})
	}
}

func Test_Static_FS_MissingRootDoesNotFallback(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/static*", New("missing", Config{FS: os.DirFS("../../.github/testdata")}))

	apptest.Expect(t, app, "/static/index.html", fiber.StatusNotFound)
}

func Test_Static_FS_RootLeadingSlash(t *testing.T) {
	t.Parallel()
	expectedIndexBody := staticTestRead(t, "../../.github/testdata/fs/index.html")
	expectedCSSBody := staticTestRead(t, "../../.github/testdata/fs/css/style.css")

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
		{name: "slash root serves index", root: "/", target: "/index.html", wantBody: expectedIndexBody, wantType: fiber.MIMETextHTMLCharsetUTF8},
		{name: "slash root serves nested", root: "/", target: "/css/style.css", wantBody: expectedCSSBody, wantType: fiber.MIMETextCSSCharsetUTF8},
		{name: "slash subdir serves file", root: "/css", target: "/style.css", wantBody: expectedCSSBody, wantType: fiber.MIMETextCSSCharsetUTF8},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use("/", New(tc.root, Config{FS: os.DirFS("../../.github/testdata/fs")}))

			resp := apptest.Expect(t, app, tc.target, fiber.StatusOK)
			require.Equal(t, tc.wantType, resp.Header.Get(fiber.HeaderContentType))

			require.Equal(t, tc.wantBody, apptest.Body(t, resp))
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
		{name: "file", path: "index.html", filesystem: os.DirFS("../../.github"), expected: true},
		{name: "file", path: "index2.html", filesystem: os.DirFS("../../.github"), gotError: fs.ErrNotExist},
		{name: "directory", path: ".", filesystem: os.DirFS("../../.github"), expected: false},
		{name: "directory", path: "not_exists", filesystem: os.DirFS("../../.github"), gotError: fs.ErrNotExist},
		{name: "directory", path: ".", filesystem: os.DirFS(testCSSDir), expected: false},
		{name: "file", path: testCSSDir + "/style.css", filesystem: nil, expected: true},
		{name: "file", path: testCSSDir + "/style2.css", filesystem: nil, gotError: fs.ErrNotExist},
		{name: "directory", path: testCSSDir, filesystem: nil, expected: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
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
	app.Get("/*", New(dir, Config{Compress: true}))

	// Note: deflate is not supported by fasthttp.FS
	algorithms := []string{"zstd", "gzip", "br"}

	for _, algo := range algorithms {
		t.Run(algo+"_compression", func(t *testing.T) {
			t.Parallel()
			// request non-compressible file (less than 200 bytes), Content Length will remain the same
			req := httptest.NewRequest(fiber.MethodGet, "/css/style.css", http.NoBody)
			req.Header.Set("Accept-Encoding", algo)
			resp := apptest.Do(t, app, req, testConfig)
			require.Equal(t, 200, resp.StatusCode, "Status code")
			require.Empty(t, resp.Header.Get(fiber.HeaderContentEncoding))
			require.Equal(t, "46", resp.Header.Get(fiber.HeaderContentLength))

			// request compressible file, ContentLength will change
			req = httptest.NewRequest(fiber.MethodGet, "/index.html", http.NoBody)
			req.Header.Set("Accept-Encoding", algo)
			resp = apptest.Do(t, app, req, testConfig)
			require.Equal(t, 200, resp.StatusCode, "Status code")
			require.Equal(t, algo, resp.Header.Get(fiber.HeaderContentEncoding))
			require.Greater(t, "299", resp.Header.Get(fiber.HeaderContentLength))
		})
	}
}

func Test_Static_Compress_FileRoot(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "index.html")
	require.NoError(t, os.WriteFile(root, []byte(strings.Repeat("<p>hello</p>", 200)), 0o600))
	app := fiber.New()
	app.Get("/", New(root, Config{Compress: true}))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAcceptEncoding, "gzip")
	resp := apptest.Expect(t, app, req, fiber.StatusOK)
	require.Equal(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
}

// Test_Static_Compress_Suffixes requests index.html with each encoding once the file cache has expired:
// the compressed copy is served, and written next to the file under the suffix of the app, its default
// one or one it configures.
func Test_Static_Compress_Suffixes(t *testing.T) {
	t.Parallel()
	dir := "../../.github/testdata/fs"
	for _, tc := range []struct {
		suffixes   map[string]string
		name       string
		configured bool // the app configures the suffixes, which are otherwise its defaults
		plainFirst bool // the file is first requested without an encoding
	}{
		{
			name: "WithoutEncoding", plainFirst: true,
			suffixes: map[string]string{"gzip": ".fiber.gz", "br": ".fiber.br", "zstd": ".fiber.zst"},
		},
		{
			name: "WithFileSuffixes", configured: true,
			suffixes: map[string]string{"gzip": ".test.gz", "br": ".test.br", "zstd": ".test.zst"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var appCfg []fiber.Config
			if tc.configured {
				appCfg = append(appCfg, fiber.Config{CompressedFileSuffixes: tc.suffixes})
			}
			app := fiber.New(appCfg...)
			app.Get("/*", New(dir, Config{Compress: true, CacheDuration: 1 * time.Second}))

			if tc.plainFirst {
				resp := apptest.Do(t, app, "/index.html", testConfig)
				require.Equal(t, 200, resp.StatusCode, "Status code")
				require.Empty(t, resp.Header.Get(fiber.HeaderContentEncoding))
				require.Equal(t, "299", resp.Header.Get(fiber.HeaderContentLength))
			}

			for _, algo := range []string{"zstd", "gzip", "br"} {
				// Wait for cache to expire
				time.Sleep(2 * time.Second)

				req := apptest.Req(fiber.MethodGet, "/index.html", apptest.WithHeader("Accept-Encoding", algo))
				resp := apptest.Do(t, app, req, testConfig)
				require.Equal(t, 200, resp.StatusCode, "Status code")
				require.Equal(t, algo, resp.Header.Get(fiber.HeaderContentEncoding))
				require.Greater(t, "299", resp.Header.Get(fiber.HeaderContentLength))

				_, err := os.Stat(dir + "/index.html" + tc.suffixes[algo])
				require.NoError(t, err, "File should exist")
			}
		})
	}
}

func Test_Router_Mount_n_Static(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use("/static", New(testCSSDir, Config{Browse: true}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("Home") })

	subApp := fiber.New()
	app.Use("/mount", subApp)
	subApp.Get("/test", func(c fiber.Ctx) error { return c.SendString("Hello from /test") })

	app.Use(func(c fiber.Ctx) error { return c.Status(fiber.StatusNotFound).SendString("Not Found") })

	apptest.Expect(t, app, "/static/style.css", 200)
}

// staticTestTraversalBlocked requires, in a parallel subtest per path, that app blocks a request for the path.
// Blocked can mean different status codes depending on what triggered the block:
//   - 404 is the expected blocked response in most cases, with the "Not Found" message of the default handler;
//   - 400 might occur if fasthttp rejects the request before it is even processed (e.g., null bytes).
func staticTestTraversalBlocked(t *testing.T, app *fiber.App, paths []string) {
	t.Helper()

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			resp := apptest.Do(t, app, path)
			body := apptest.Body(t, resp)

			switch resp.StatusCode {
			case fiber.StatusNotFound:
				require.Contains(t, body, "Not Found", "Blocked traversal should say \"Not Found\" for %s", path)
			case fiber.StatusBadRequest:
				require.Contains(t, body, "Are you a hacker?", "Blocked traversal should say it for %s", path)
			default:
				t.Fatalf("Status code for path traversal %s should be 400 or 404, got %d", path, resp.StatusCode)
			}
		})
	}
}

func Test_Static_PathTraversal(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == winOS {
		t.Skip("Skipping Windows-specific tests")
	}

	app := fiber.New()

	// Serve only from testCSSDir
	// This directory should contain `style.css` but not `index.html` or anything above it.
	rootDir := testCSSDir
	app.Get("/*", New(rootDir))

	validResp := apptest.Expect(t, app, "/style.css", 200)
	require.Equal(t, fiber.MIMETextCSSCharsetUTF8, validResp.Header.Get(fiber.HeaderContentType))
	require.Contains(t, apptest.Body(t, validResp), "color")

	paths := []string{
		// Basic attempts to escape the directory
		"/index.html..",
		"/style.css..",
		"/../index.html",
		"/../../index.html",
		"/../../../index.html",

		// Attempts with double slashes
		"//../index.html",
		"/..//index.html",

		// Encoded attempts: `%2e` is '.' and `%2f` is '/'
		"/..%2findex.html",        // ../index.html
		"/%2e%2e/index.html",      // ../index.html
		"/%2e%2e%2f%2e%2e/secret", // ../../../secret

		// Mixed encoded and normal attempts
		"/%2e%2e/../index.html",  // ../../index.html
		"/..%2f..%2fsecret.json", // ../../../secret.json

		// Attempts with current directory references
		"/./../index.html",
		"/././../index.html",

		// Trailing slashes
		"/../",
		"/../../",

		// Attempts to load files from an absolute path outside the root
		"/" + rootDir + "/../../index.html",

		// Additional edge cases:

		// Double-encoded `..`
		"/%252e%252e/index.html", // double-encoded .. -> ../index.html after double decoding

		// Multiple levels of encoding and traversal
		"/%2e%2e%2F..%2f%2e%2e%2fWINDOWS",       // multiple ups and unusual pattern
		"/%2e%2e%2F..%2f%2e%2e%2f%2e%2e/secret", // more complex chain of ../

		// Null byte attempts
		"/index.html%00.jpg",
		"/%00index.html",
		"/somefolder%00/something",
		"/%00/index.html",

		// Attempts to access known system files
		"/etc/passwd",
		"/etc/",

		// Complex mixed attempts with encoded slashes and dots
		"/..%2F..%2F..%2F..%2Fetc%2Fpasswd",

		// Attempts inside subdirectories with encoded traversal
		"/somefolder/%2e%2e%2findex.html",
		"/somefolder/%2e%2e%2f%2e%2e%2findex.html",

		// Backslash encoded attempts
		"/%5C..%5Cindex.html",
		"/%5c..%5c..%5cetc%5cpasswd",
		"/%255c..%255c..%255cetc%255cpasswd",
		"/..%5c..%5cetc%5cpasswd",
		"/%2e%2e%5c%2e%2e%5cetc%5cpasswd",
		"/%2e%2e%2f%2e%2e%5cetc%5cpasswd",
		"/%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
		"/.%2e/.%2e/etc/passwd",
		"/..%2f..%2f..%2f..%2fetc%2fpasswd",
		"/%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
		"/%2e%2e%2f%2e%2e%2fetc%2fshadow",
		"/%2e%2e/%2e%2e/var/log/auth.log",
		"/..%2f..%2fvar%2flog%2fauth.log",
		"/%2e%2e//%2e%2e//etc/passwd",
		"/..//..//etc/passwd",
		"/%2e%2e%2f%2e%2e%2f%2e%2e%2fproc%2fself%2fenviron",
		"/%2e%2e%2f%2e%2e%2f%2e%2e%2froot%2f.ssh%2fauthorized_keys",
	}

	staticTestTraversalBlocked(t, app, paths)
}

func Test_Static_PathTraversal_WindowsOnly(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != winOS {
		t.Skip("Skipping Windows-specific tests")
	}

	app := fiber.New()
	rootDir := testCSSDir
	app.Get("/*", New(rootDir))

	// A valid request (relative path without backslash):
	validResp := apptest.Do(t, app, "/style.css")
	require.Equal(t, 200, validResp.StatusCode, "Status code for valid file on Windows")
	body := apptest.Body(t, validResp)
	require.Contains(t, body, "color")

	paths := []string{
		// Windows-specific traversal attempts
		// Backslashes are treated as directory separators on Windows.
		"/..\\index.html",
		"/..\\..\\index.html",
		"/..\\..\\..\\Windows\\win.ini",
		"/..\\..\\..\\Windows\\System32\\drivers\\etc\\hosts",
		"/%5C..%5C..%5CWindows%5Cwin.ini",
		"/%255C..%255C..%255CWindows%255Cwin.ini",
		"/%5c..%5c..%5cWindows%5cSystem32%5cdrivers%5cetc%5chosts",
		"/C:\\Windows\\System32\\cmd.exe",
		"/C:%5CWindows%5CSystem32%5Ccmd.exe",
		"/%43:%5CWindows%5CSystem32%5Ccmd.exe",
		"/%5c%5cserver%5cshare%5csecret.txt",
		"//server\\share\\secret.txt",
		"//server/share/secret.txt",
		"/%2F%2Fserver%2Fshare%2Fsecret.txt",

		// Attempt with a path that might try to reference Windows drives or absolute paths
		// Note: These are artificial tests to ensure no drive-letter escapes are allowed.
		"/C:\\Windows\\System32\\cmd.exe",
		"/C:/Windows/System32/cmd.exe",

		// Attempt with UNC-like paths (though unlikely in a web context, good to test)
		"//server\\share\\secret.txt",

		// Attempt using a mixture of forward and backward slashes
		"/..\\..\\/index.html",

		// Attempt that includes a null-byte on Windows
		"/index.html%00.txt",

		// Check behavior on an obviously nonexistent and suspicious file
		"/\\this\\path\\does\\not\\exist\\..",

		// Attempts involving relative traversal and current directory reference
		"/.\\../index.html",
		"/./..\\index.html",
	}

	staticTestTraversalBlocked(t, app, paths)
}

// Test_Static_ServesRoutedPath pins that the file server resolves the path
// the router matched, so a guard on "/static/private" covers the spellings the
// router normalizes, that what the router leaves encoded remains encoded, and
// that a spelling the router keeps apart from the guarded path names no file.
func Test_Static_ServesRoutedPath(t *testing.T) {
	t.Parallel()
	root := staticTestRoot(t, map[string]string{
		"private/secret.txt":      "SECRET",
		"private area/secret.txt": "SPACED SECRET",
		"public.txt":              "PUBLIC",
		"hello world.txt":         "HELLO",
		"100%.txt":                "PERCENT",
		"photo@2x.png":            "PHOTO",
		"a{b}.txt":                "BRACES",
	})

	// the file cache is off so the served files are closed when the temp dir is removed
	mounts := []staticTestMount{
		{name: "get wildcard", mount: func(app *fiber.App) { app.Get("/static/*", New(root, Config{CacheDuration: -1})) }},
		{name: "use prefix", mount: func(app *fiber.App) { app.Use("/static", New(root, Config{CacheDuration: -1})) }},
		{name: "fs root", mount: func(app *fiber.App) {
			app.Get("/static/*", New("", Config{FS: os.DirFS(root), CacheDuration: -1}))
		}},
		{name: "fs subdirectory root", mount: func(app *fiber.App) {
			app.Get("/static/*", New(filepath.Base(root), Config{FS: os.DirFS(filepath.Dir(root)), CacheDuration: -1}))
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
		{name: "encoded space stays a distinct name", target: "/static/hello%20world.txt", wantStatus: fiber.StatusNotFound},
		{name: "encoded percent stays a distinct name", target: "/static/100%25.txt", wantStatus: fiber.StatusNotFound},
		{name: "reserved character stays encoded", target: "/static/photo%402x.png", wantStatus: fiber.StatusNotFound},
		{name: "uppercase reserved escapes stay encoded", target: "/static/a%7Bb%7D.txt", wantStatus: fiber.StatusNotFound},
		{name: "lowercase reserved escapes stay encoded", target: "/static/a%7bb%7d.txt", wantStatus: fiber.StatusNotFound},
		{name: "encoded protected space cannot bypass guard", target: "/static/private%20area/secret.txt", wantStatus: fiber.StatusNotFound},
		{name: "double encoding is not decoded again", target: "/static/hello%2520world.txt", wantStatus: fiber.StatusNotFound},
		{name: "encoded guarded segment", target: "/static/%70rivate/secret.txt", wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "encoded slash into the guarded directory", target: "/static/private%2Fsecret.txt", wantStatus: fiber.StatusNotFound},
		{name: "parent segment into the guarded directory", target: "/static/x/../private/secret.txt", wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "current segment into the guarded directory", target: "/static/./private/secret.txt", wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "empty segment is not a directory", target: "/static//private/secret.txt", wantStatus: fiber.StatusNotFound},
		{name: "empty segment before the file", target: "/static/private//secret.txt", wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "encoded backslash traversal into the guarded directory", target: "/static/..%5Cprivate/secret.txt", wantStatus: fiber.StatusNotFound},
		{name: "double encoded guarded segment", target: "/static/%2570rivate/secret.txt", wantStatus: fiber.StatusNotFound},
		{name: "triple encoded guarded segment", target: "/static/%252570rivate/secret.txt", wantStatus: fiber.StatusNotFound},
		{name: "unescaped routing encoded guarded segment", target: "/static/%70rivate/secret.txt", unescape: true, wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "unescaped routing encoded slash into the guarded directory", target: "/static/private%2Fsecret.txt", unescape: true, wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "unescaped routing parent segment into the guarded directory", target: "/static/x/../private/secret.txt", unescape: true, wantStatus: fiber.StatusForbidden, wantBody: "Forbidden"},
		{name: "unescaped routing encoded backslash traversal", target: "/static/..%5Cprivate/secret.txt", unescape: true, wantStatus: fiber.StatusNotFound},
		{name: "unescaped routing double encoded guarded segment", target: "/static/%2570rivate/secret.txt", unescape: true, wantStatus: fiber.StatusNotFound},
		{name: "unescaped routing reserved character in the name", target: "/static/photo%402x.png", unescape: true, wantStatus: fiber.StatusOK, wantBody: "PHOTO"},
		{name: "unescaped routing lowercase hex digits", target: "/static/a%7bb%7d.txt", unescape: true, wantStatus: fiber.StatusOK, wantBody: "BRACES"},
		{name: "unescaped routing empty segment is not a directory", target: "/static//private/secret.txt", unescape: true, wantStatus: fiber.StatusNotFound},
	}

	for _, m := range mounts {
		for _, tc := range requests {
			t.Run(m.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				app := staticTestApp(tc.unescape, "/static/private", "/static/private area")
				m.mount(app)

				resp := apptest.Expect(t, app, tc.target, tc.wantStatus)

				body := apptest.Body(t, resp)
				if tc.wantBody != "" {
					require.Equal(t, tc.wantBody, body)
				}
				if tc.wantStatus != fiber.StatusOK {
					require.NotContains(t, body, "SECRET")
					require.NotContains(t, body, "HELLO")
					require.NotContains(t, body, "PERCENT")
					require.NotContains(t, body, "PHOTO")
					require.NotContains(t, body, "BRACES")
				}
			})
		}
	}
}

// headerReached is set on the response of a request that went through the
// file server of a root mount test.
const headerReached = "X-Static-Reached"

// reached is the Next of the root mount tests. It never skips the file server;
// it records that a request got there, which tells a path the file server
// refused from one that never reached it.
func reached(c fiber.Ctx) bool {
	c.Set(headerReached, "true")
	return false
}

// serveRaw sends target to the handler exactly as written and returns the
// answer. app.Test builds the request line from a url.URL, which encodes a raw
// backslash and turns an absolute-form target into an origin-form one, and
// httptest refuses a malformed escape. Take the handler from app.Handler()
// once: each call prepares the app for its start again.
func serveRaw(handler fasthttp.RequestHandler, method, target string) *fasthttp.Response {
	var req fasthttp.Request
	req.Header.SetMethod(method)
	req.SetRequestURI(target)
	req.Header.SetHost("example.com")

	fctx := new(fasthttp.RequestCtx)
	fctx.Init(&req, nil, discardLogger{})
	handler(fctx)
	return &fctx.Response
}

// Test_Static_RootMount_EmptyFirstSegment pins that a root mount serves what
// the router matched to it and nothing it merely resembles. The router keeps
// the empty first segment of "//admin/secret.txt", so it is not the path
// "/admin/secret.txt" and middleware mounted on "/admin" does not run for it;
// stripping the mount's "/" must not hide that segment from the file server,
// which would open the file the middleware was mounted to guard.
//
// The rows cover the other ways to write such a path: an encoded slash, which
// the router decodes under UnescapePath, dot segments that leave the empty
// segment in place, an absolute-form target, a query, a backslash and an escape
// that was encoded twice. Each row runs with and without UnescapePath, for GET
// and HEAD, and the targets are sent as written. A row the guard does not
// answer must also have reached the file server, so it is the file server that
// refused it and not the router that never sent it there.
func Test_Static_RootMount_EmptyFirstSegment(t *testing.T) {
	t.Parallel()
	root := staticTestRoot(t, map[string]string{
		"admin/secret.txt": "SECRET", "admin/index.html": "ADMININDEX", "public.txt": "PUBLIC", "index.html": "INDEX",
	})

	// the file cache is off so the served files are closed when the temp dir is removed
	config := func(fsys fs.FS) Config { return Config{FS: fsys, CacheDuration: -1, Next: reached} }

	// each of these mounts has the root as its prefix
	mounts := []staticTestMount{
		{name: "use without a path", mount: func(app *fiber.App) { app.Use(New(root, config(nil))) }},
		{name: "use root", mount: func(app *fiber.App) { app.Use("/", New(root, config(nil))) }},
		{name: "use wildcard", mount: func(app *fiber.App) { app.Use("/*", New(root, config(nil))) }},
		{name: "get wildcard", mount: func(app *fiber.App) { app.Get("/*", New(root, config(nil))) }},
		{name: "get bare wildcard", mount: func(app *fiber.App) { app.Get("*", New(root, config(nil))) }},
		{name: "group root", mount: func(app *fiber.App) { app.Group("/").Use(New(root, config(nil))) }},
		{name: "fs root", mount: func(app *fiber.App) { app.Use(New("", config(os.DirFS(root)))) }},
		{name: "fs subdirectory root", mount: func(app *fiber.App) {
			app.Get("/*", New(filepath.Base(root), config(os.DirFS(filepath.Dir(root)))))
		}},
	}

	const (
		ok      = fiber.StatusOK
		guarded = fiber.StatusForbidden // the answer of the middleware mounted on "/admin"
		missing = fiber.StatusNotFound
	)

	requests := []struct {
		name          string
		target        string
		wantBody      string
		wantStatus    int
		wantUnescaped int // the status under UnescapePath, when it is not wantStatus
	}{
		// the guard and the files it covers
		{name: "guarded file", target: "/admin/secret.txt", wantStatus: guarded},
		{name: "guarded directory", target: "/admin/", wantStatus: guarded},
		{name: "guarded directory name", target: "/admin", wantStatus: guarded},
		{name: "public file", target: "/public.txt", wantStatus: ok, wantBody: "PUBLIC"},
		{name: "directory index", target: "/", wantStatus: ok, wantBody: "INDEX"},

		// an empty first segment
		{name: "empty first segment before the guarded file", target: "//admin/secret.txt", wantStatus: missing},
		{name: "empty first segment before a public file", target: "//public.txt", wantStatus: missing},
		{name: "empty first segment before the guarded directory", target: "//admin/", wantStatus: missing},
		{name: "empty first segment before the guarded directory name", target: "//admin", wantStatus: missing},
		{name: "two empty segments before the guarded file", target: "///admin/secret.txt", wantStatus: missing},
		{name: "empty first segment and a query", target: "//admin/secret.txt?x=1", wantStatus: missing},
		{name: "empty first segment and a query holding a scheme separator", target: "//admin/secret.txt?next=http://example.com", wantStatus: missing},
		{name: "guarded file and a query", target: "/admin/secret.txt?x=1", wantStatus: guarded},

		// an encoded slash: refused as it is, and decoded into an empty segment under UnescapePath
		{name: "encoded slash before the guarded file", target: "/%2Fadmin/secret.txt", wantStatus: missing},
		{name: "lowercase encoded slash before the guarded file", target: "/%2fadmin/secret.txt", wantStatus: missing},
		{name: "encoded slash before the guarded directory", target: "/%2Fadmin/", wantStatus: missing},
		{name: "encoded slash before a public file", target: "/%2Fpublic.txt", wantStatus: missing},
		{name: "two encoded slashes before the guarded file", target: "/%2F%2Fadmin/secret.txt", wantStatus: missing},
		{name: "encoded slash without a leading slash", target: "%2Fadmin/secret.txt", wantStatus: missing},
		{name: "encoded slash before an encoded name", target: "/%2F%61dmin/secret.txt", wantStatus: missing},
		{name: "encoded slash after the guarded directory name", target: "/admin%2Fsecret.txt", wantStatus: missing, wantUnescaped: guarded},

		// Dot segments that leave the empty first segment in place. The router has
		// turned each into "//admin/secret.txt" before the file server sees it, so
		// there is a row for each kind of normalization (a current segment, a
		// parent segment at the root and after a name, an encoded one) and not for
		// each spelling; the router's own tests cover those.
		{name: "current segment and an empty segment", target: "/.//admin/secret.txt", wantStatus: missing},
		{name: "parent segment and an empty segment", target: "/..//admin/secret.txt", wantStatus: missing},
		{name: "segment, parent segment and an empty segment", target: "/x/..//admin/secret.txt", wantStatus: missing},
		{name: "encoded parent segment and an empty segment", target: "/%2e%2e//admin/secret.txt", wantStatus: missing},

		// dot segments that leave no empty segment, so the guard sees the path it covers
		{name: "current segment before the guarded file", target: "/./admin/secret.txt", wantStatus: guarded},
		{name: "parent segment before the guarded file", target: "/../admin/secret.txt", wantStatus: guarded},
		{name: "segment and parent segment before the guarded file", target: "/x/../admin/secret.txt", wantStatus: guarded},
		{name: "empty segment and parent segment before the guarded file", target: "//../admin/secret.txt", wantStatus: guarded},
		{name: "encoded parent segment before the guarded file", target: "/%2e%2e/admin/secret.txt", wantStatus: guarded},
		{name: "encoded parent segment and encoded slash before the guarded file", target: "/%2e%2e%2fadmin/secret.txt", wantStatus: missing, wantUnescaped: guarded},
		{name: "parent segment and encoded slash before the guarded file", target: "/..%2fadmin/secret.txt", wantStatus: missing, wantUnescaped: guarded},

		// an unreserved character written as an escape, which the router decodes before it matches
		{name: "encoded name behind an empty segment", target: "//%61dmin/secret.txt", wantStatus: missing},
		{name: "encoded file behind an empty segment", target: "//admin/se%63ret.txt", wantStatus: missing},
		{name: "encoded name", target: "/%61dmin/secret.txt", wantStatus: guarded},
		{name: "encoded file", target: "/admin/se%63ret.txt", wantStatus: guarded},

		// a request target in absolute form
		{name: "absolute form with an empty first segment", target: "http://example.com//admin/secret.txt", wantStatus: missing},
		{name: "absolute form with an encoded slash", target: "http://example.com/%2Fadmin/secret.txt", wantStatus: missing},
		{name: "absolute form", target: "http://example.com/admin/secret.txt", wantStatus: guarded},

		// a backslash, raw or escaped, is neither a separator nor an empty segment
		{name: "backslash before the guarded file", target: `/\admin/secret.txt`, wantStatus: missing},
		{name: "backslash and a slash before the guarded file", target: `/\/admin/secret.txt`, wantStatus: missing},
		{name: "slash and a backslash before the guarded file", target: `//\admin/secret.txt`, wantStatus: missing},
		{name: "backslash without a leading slash", target: `\admin/secret.txt`, wantStatus: missing},
		{name: "encoded backslash before the guarded file", target: "/%5Cadmin/secret.txt", wantStatus: missing},
		{name: "lowercase encoded backslash before the guarded file", target: "/%5cadmin/secret.txt", wantStatus: missing},

		// an escape that was encoded twice stays part of a name
		{name: "double encoded slash before the guarded file", target: "/%252Fadmin/secret.txt", wantStatus: missing},
		{name: "double encoded backslash before the guarded file", target: "/%255Cadmin/secret.txt", wantStatus: missing},

		// other ways to write a separator, or nothing
		{name: "unicode escape of a slash before the guarded file", target: "/%u002fadmin/secret.txt", wantStatus: missing},
		{name: "overlong slash before the guarded file", target: "/%C0%AFadmin/secret.txt", wantStatus: missing},
		{name: "full-width solidus before the guarded file", target: "/%EF%BC%8Fadmin/secret.txt", wantStatus: missing},
		{name: "null byte before the guarded file", target: "/%00admin/secret.txt", wantStatus: missing},
		{name: "path parameter before the guarded file", target: "/;/admin/secret.txt", wantStatus: missing},
	}

	// One app per mount and value of UnescapePath, built once: the rows share
	// its handler.
	modes := []bool{false, true}
	handlers := make([][]fasthttp.RequestHandler, len(mounts))
	for i, m := range mounts {
		for _, unescape := range modes {
			app := staticTestApp(unescape, "/admin")
			m.mount(app)
			handlers[i] = append(handlers[i], app.Handler())
		}
	}

	for i, m := range mounts {
		for _, tc := range requests {
			t.Run(m.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				for j, unescape := range modes {
					want := tc.wantStatus
					if unescape && tc.wantUnescaped != 0 {
						want = tc.wantUnescaped
					}

					// HEAD must not reveal a file GET does not serve
					for _, method := range []string{fiber.MethodGet, fiber.MethodHead} {
						resp := serveRaw(handlers[i][j], method, tc.target)
						require.Equal(t, want, resp.StatusCode(), "%s %s unescape=%v", method, tc.target, unescape)
						// the guard answers before the file server; every other row has to get there
						require.Equal(t, want != guarded, len(resp.Header.Peek(headerReached)) > 0,
							"%s %s unescape=%v: reached the file server", method, tc.target, unescape)
						if method != fiber.MethodGet {
							continue // a HEAD answer has no body to check
						}

						body := string(resp.Body())
						switch want {
						case ok:
							require.Equal(t, tc.wantBody, body, "%s %s unescape=%v", method, tc.target, unescape)
						case guarded:
							require.Equal(t, "Forbidden", body, "%s %s unescape=%v", method, tc.target, unescape)
						default:
							for _, served := range []string{"SECRET", "ADMININDEX", "PUBLIC"} {
								require.NotContains(t, body, served, "%s %s unescape=%v", method, tc.target, unescape)
							}
						}
					}
				}
			})
		}
	}
}

// Test_Static_RootMount_EmptyFirstSegment_Browse pins that directory browsing
// does not list a directory through a path with an empty first segment, however
// it is written: the router keeps "//admin/" apart from "/admin/", which the
// guard answers. "//" and "/%2F" are here and not in the table above because
// only a wildcard route sends them to the file server.
func Test_Static_RootMount_EmptyFirstSegment_Browse(t *testing.T) {
	t.Parallel()
	root := staticTestRoot(t, map[string]string{"admin/secret.txt": "SECRET"})

	const (
		ok      = fiber.StatusOK
		guarded = fiber.StatusForbidden
		missing = fiber.StatusNotFound
	)

	requests := []struct {
		target     string
		wantStatus int
	}{
		{target: "/", wantStatus: ok},
		{target: "/admin/", wantStatus: guarded},
		{target: "//admin/", wantStatus: missing},
		{target: "//", wantStatus: missing},
		{target: "/%2Fadmin/", wantStatus: missing},
		{target: "/%2F", wantStatus: missing},
		{target: "/%252Fadmin/", wantStatus: missing},
		{target: "/.//admin/", wantStatus: missing},
		{target: `/\admin/`, wantStatus: missing},
	}

	for _, unescape := range []bool{false, true} {
		app := staticTestApp(unescape, "/admin")
		app.Get("/*", New(root, Config{Browse: true, CacheDuration: -1, Next: reached}))
		handler := app.Handler()

		for _, tc := range requests {
			resp := serveRaw(handler, fiber.MethodGet, tc.target)
			body := string(resp.Body())
			require.Equal(t, tc.wantStatus, resp.StatusCode(), "unescape=%v target=%s", unescape, tc.target)
			require.Equal(t, tc.wantStatus != guarded, len(resp.Header.Peek(headerReached)) > 0,
				"unescape=%v target=%s: reached the file server", unescape, tc.target)
			require.NotContains(t, body, "secret.txt", "unescape=%v target=%s", unescape, tc.target)
			require.NotContains(t, body, "SECRET", "unescape=%v target=%s", unescape, tc.target)
		}
	}
}

// Test_Static_RootMount_EmptyFirstSegment_NotFoundHandler pins that a path with
// an empty first segment is a miss like any other for the NotFoundHandler,
// whether it is written plainly, with an encoded slash or as an absolute-form
// target, while the same name without it is served.
func Test_Static_RootMount_EmptyFirstSegment_NotFoundHandler(t *testing.T) {
	t.Parallel()
	root := staticTestRoot(t, map[string]string{"public.txt": "PUBLIC"})

	for _, unescape := range []bool{false, true} {
		app := staticTestApp(unescape)
		app.Get("/*", New(root, Config{
			CacheDuration: -1,
			NotFoundHandler: func(c fiber.Ctx) error {
				return c.Status(fiber.StatusTeapot).SendString("missing " + c.Path())
			},
		}))
		handler := app.Handler()

		resp := serveRaw(handler, fiber.MethodGet, "/public.txt")
		require.Equal(t, fiber.StatusOK, resp.StatusCode(), "unescape=%v", unescape)
		require.Equal(t, "PUBLIC", string(resp.Body()), "unescape=%v", unescape)

		for _, target := range []string{"//public.txt", "/%2Fpublic.txt", "http://example.com//public.txt"} {
			resp = serveRaw(handler, fiber.MethodGet, target)
			body := string(resp.Body())
			require.Equal(t, fiber.StatusTeapot, resp.StatusCode(), "unescape=%v target=%s", unescape, target)
			require.True(t, strings.HasPrefix(body, "missing /"), "unescape=%v target=%s body=%q", unescape, target, body)
			require.NotContains(t, body, "PUBLIC", "unescape=%v target=%s", unescape, target)
		}
	}
}

// staticTestMount mounts a file server on an app.
type staticTestMount struct {
	mount func(app *fiber.App)
	name  string
}

// staticTestFSFileMount mounts the file shell.html of the fs.FS of root on route.
// The file cache is off so the served files are closed when the temp dir is
// removed.
func staticTestFSFileMount(root, route string) func(app *fiber.App) {
	return func(app *fiber.App) {
		app.Get(route, New("shell.html", Config{FS: os.DirFS(root), CacheDuration: -1, Next: reached}))
	}
}

// staticTestFileMounts returns the mounts of a root that is the file shell.html
// in root, served by route: once by its path and once through an fs.FS.
func staticTestFileMounts(root, route string) []staticTestMount {
	return []staticTestMount{
		{name: "file", mount: func(app *fiber.App) {
			app.Get(route, New(filepath.Join(root, "shell.html"), Config{CacheDuration: -1, Next: reached}))
		}},
		{name: "fs file", mount: staticTestFSFileMount(root, route)},
	}
}

// staticTestShellRequest is a request sent as written to a mount of the file
// shell.html, and the status it is answered with.
type staticTestShellRequest struct {
	target          string
	status          int
	unescapedStatus int  // the status under UnescapePath, when it is not status
	unreached       bool // the request does not get to the file server: the router or a guard answers it
}

// staticTestShell sends every request to every mount, once under each value of
// UnescapePath. A 200 is the file, a 403 the answer of the guard mounted on
// "/admin", and any other answer must not reveal the file or the secret behind
// the guard. A request that is not unreached must get to the file server, so it
// is the file server that refused it and not the router that never sent it there.
func staticTestShell(t *testing.T, mounts []staticTestMount, requests []staticTestShellRequest) {
	t.Helper()

	for _, m := range mounts {
		for _, unescape := range []bool{false, true} {
			app := staticTestApp(unescape, "/admin")
			m.mount(app)
			handler := app.Handler()

			for _, tc := range requests {
				want := tc.status
				if unescape && tc.unescapedStatus != 0 {
					want = tc.unescapedStatus
				}

				resp := serveRaw(handler, fiber.MethodGet, tc.target)
				body := string(resp.Body())
				require.Equal(t, want, resp.StatusCode(), "%s unescape=%v target=%s", m.name, unescape, tc.target)
				require.Equal(t, !tc.unreached, len(resp.Header.Peek(headerReached)) > 0,
					"%s unescape=%v target=%s: reached the file server", m.name, unescape, tc.target)
				switch want {
				case fiber.StatusOK:
					require.Equal(t, "SHELL", body, "%s unescape=%v target=%s", m.name, unescape, tc.target)
				case fiber.StatusForbidden:
					require.Equal(t, "Forbidden", body, "%s unescape=%v target=%s", m.name, unescape, tc.target)
				default:
					require.NotContains(t, body, "SHELL", "%s unescape=%v target=%s", m.name, unescape, tc.target)
					require.NotContains(t, body, "SECRET", "%s unescape=%v target=%s", m.name, unescape, tc.target)
				}
			}
		}
	}
}

// Test_Static_RootFile_EmptyFirstSegment pins how a root that is a file treats
// a path. A root mount answers a path with an empty first segment with 404
// whatever it serves, so a file root answers "//x" as a directory root does. For
// a file root that is parity and not protection: the router keeps "//x" apart
// from "/x", so middleware mounted on "/x" does not run for it, but the same
// file is served at every other path. Every other spelling gets the file,
// because nothing is looked up by the path: a backslash, a malformed escape or a
// NUL byte is served like any other path, and no spelling of a path can select
// another file. An empty segment further in is served too, since it is part of
// an opaque name, as in "/web/https://example.com", a route a single-page app
// can take. Middleware mounted on a prefix still answers the paths it matches.
func Test_Static_RootFile_EmptyFirstSegment(t *testing.T) {
	t.Parallel()
	root := staticTestRoot(t, map[string]string{"admin/secret.txt": "SECRET", "shell.html": "SHELL"})

	const (
		ok      = fiber.StatusOK
		guarded = fiber.StatusForbidden
		missing = fiber.StatusNotFound
	)

	staticTestShell(t, staticTestFileMounts(root, "/*"), []staticTestShellRequest{
		{target: "/", status: ok},
		{target: "/anything", status: ok},
		{target: "//anything", status: missing},
		{target: "///anything", status: missing},
		{target: "//admin/secret.txt", status: missing},
		{target: "/.//anything", status: missing},
		{target: "/a//b", status: ok},
		{target: "/web/https://example.com/x", status: ok},
		{target: "/%2Fanything", status: ok, unescapedStatus: missing},
		{target: `/a\b`, status: ok},
		{target: "/%5Cb", status: ok},
		{target: "/%zz", status: ok},
		{target: "/%00", status: ok},
		{target: "/admin/secret.txt", status: guarded, unreached: true},
	})
}

// Test_Static_RootFile_OnAPrefix pins that a root that is a file mounted on a
// prefix reads nothing from the path below it. The empty first segment that a
// root mount refuses cannot occur there, since the router sends the file server
// only paths that start with the prefix, and an empty segment below the prefix
// is part of a name like any other. A prefix that starts with one itself is
// covered by Test_Static_RootFile_OnAPrefixWithEmptyFirstSegment.
func Test_Static_RootFile_OnAPrefix(t *testing.T) {
	t.Parallel()
	root := staticTestRoot(t, map[string]string{"shell.html": "SHELL"})

	staticTestShell(t, staticTestFileMounts(root, "/static/*"), []staticTestShellRequest{
		{target: "/static/x", status: fiber.StatusOK},
		{target: "/static//x", status: fiber.StatusOK},
		{target: "/static/web/https://example.com", status: fiber.StatusOK},
		// the router does not send this to the file server at all
		{target: "//static/x", status: fiber.StatusNotFound, unreached: true},
	})
}

// Test_Static_RootFile_OnAPrefixWithEmptyFirstSegment pins that refusing a path
// that starts with "//" belongs to a root mount. A route can start with an empty
// segment itself, as in "//assets/*", and then every path it matches does too,
// so a root that is a file serves its file there as a directory serves its files.
func Test_Static_RootFile_OnAPrefixWithEmptyFirstSegment(t *testing.T) {
	t.Parallel()
	root := staticTestRoot(t, map[string]string{"shell.html": "SHELL"})

	mounts := append(staticTestFileMounts(root, "//assets/*"), staticTestMount{
		name: "file with Use",
		mount: func(app *fiber.App) {
			app.Use("//assets", New(filepath.Join(root, "shell.html"), Config{CacheDuration: -1, Next: reached}))
		},
	})

	staticTestShell(t, mounts, []staticTestShellRequest{
		{target: "//assets/x", status: fiber.StatusOK},
		{target: "//assets//x", status: fiber.StatusOK},
		// the router does not send these to the file server at all
		{target: "/assets/x", status: fiber.StatusNotFound, unreached: true},
		{target: "///assets/x", status: fiber.StatusNotFound, unreached: true},
	})
}

// Test_Static_RefusedPathIsNotLookedUp pins that a path the file server refuses
// is answered as missing without a file lookup. It used to be looked up under a
// reserved name, so a file of that name in the root was served for a malformed
// path or for "/staticx" on a "/static*" route, and a root that is a single file
// on an fs.FS served that file instead of its own for a leading "//". The name
// is an ordinary one when asked for.
func Test_Static_RefusedPathIsNotLookedUp(t *testing.T) {
	t.Parallel()
	root := staticTestRoot(t, map[string]string{"shell.html": "SHELL", "a.txt": "A", "__fiber_invalid__": "RESERVED"})

	type request struct {
		target     string
		wantBody   string
		wantStatus int
	}
	directory := []request{
		{target: "/a.txt", wantStatus: fiber.StatusOK, wantBody: "A"},
		{target: "/__fiber_invalid__", wantStatus: fiber.StatusOK, wantBody: "RESERVED"},
		{target: "//a.txt", wantStatus: fiber.StatusNotFound},
		{target: "/x//y", wantStatus: fiber.StatusNotFound},
	}
	mounts := []struct {
		mount    func(app *fiber.App)
		name     string
		requests []request
	}{
		{
			name:     "directory",
			mount:    func(app *fiber.App) { app.Get("/*", New(root, Config{CacheDuration: -1, Next: reached})) },
			requests: directory,
		},
		{
			name: "fs directory",
			mount: func(app *fiber.App) {
				app.Get("/*", New(".", Config{FS: os.DirFS(root), CacheDuration: -1, Next: reached}))
			},
			requests: directory,
		},
		{
			name:  "fs file",
			mount: staticTestFSFileMount(root, "/*"),
			requests: []request{
				{target: "/x", wantStatus: fiber.StatusOK, wantBody: "SHELL"},
				{target: "//probe", wantStatus: fiber.StatusNotFound},
			},
		},
		{
			name:  "fs file on a partial wildcard",
			mount: staticTestFSFileMount(root, "/static*"),
			requests: []request{
				{target: "/static/x", wantStatus: fiber.StatusOK, wantBody: "SHELL"},
				{target: "/staticx", wantStatus: fiber.StatusNotFound},
			},
		},
	}

	for _, m := range mounts {
		for _, unescape := range []bool{false, true} {
			app := staticTestApp(unescape)
			m.mount(app)
			handler := app.Handler()

			for _, method := range []string{fiber.MethodGet, fiber.MethodHead} {
				for _, tc := range m.requests {
					resp := serveRaw(handler, method, tc.target)
					require.Equal(t, tc.wantStatus, resp.StatusCode(), "%s unescape=%v %s %s", m.name, unescape, method, tc.target)
					require.NotEmpty(t, resp.Header.Peek(headerReached),
						"%s unescape=%v %s %s: reached the file server", m.name, unescape, method, tc.target)
					if method != fiber.MethodGet {
						continue // a HEAD answer has no body to check
					}
					if tc.wantStatus == fiber.StatusOK {
						require.Equal(t, tc.wantBody, string(resp.Body()), "%s unescape=%v %s", m.name, unescape, tc.target)
					} else {
						require.NotContains(t, string(resp.Body()), "RESERVED", "%s unescape=%v %s", m.name, unescape, tc.target)
					}
				}
			}
		}
	}
}

// staticTestServeOpaque sends target to app exactly as written, which httptest
// would refuse or encode, and returns the status and the body of the answer.
// app.Test writes the opaque form of the URL verbatim.
func staticTestServeOpaque(t *testing.T, app *fiber.App, target string) (status int, body string) { //nolint:nonamedreturns // gocritic unnamedResult asks for names on these results
	t.Helper()

	req := httptest.NewRequest(fiber.MethodGet, "/static/", http.NoBody)
	req.URL.Opaque = target
	resp := apptest.Do(t, app, req)
	return resp.StatusCode, apptest.Body(t, resp)
}

// Test_Static_GuardedFileSpellings pins that a spelling of "/static/private/secret.txt" the router keeps apart from
// the guard on "/static/private" never reaches the guarded file: the file server answers 404 and leaks nothing, while
// the spelling the guard covers stays 403. The requests are sent as written, since httptest would encode or refuse
// some of them.
func Test_Static_GuardedFileSpellings(t *testing.T) {
	t.Parallel()
	root := staticTestRoot(t, map[string]string{"private/secret.txt": "SECRET"})
	mounts := map[string]func(app *fiber.App, cfg Config){
		"wildcard": func(app *fiber.App, cfg Config) { app.Get("/static/*", New(root, cfg)) },
		"prefix":   func(app *fiber.App, cfg Config) { app.Use("/static", New(root, cfg)) },
	}

	for _, tc := range []struct {
		name    string
		mounts  []string
		targets []string
		config  Config
	}{
		{
			// an ordinary character to the router, so it is never a separator for the file server either
			name: "raw backslash is not a separator", mounts: []string{"wildcard"},
			targets: []string{
				`/static/..\private/secret.txt`, `/static/x\..\private/secret.txt`,
				`/static/private\secret.txt`, `/static/..\..\etc\passwd`,
			},
		},
		{
			// A stray "%" the router keeps literally cannot line up with the following bytes into an escape the file
			// server decodes a second time, reaching a guarded file the router never matched. Both spellings normalize
			// to "/static/%70rivate/secret.txt", which the guard does not cover, so the file server must not decode
			// either to "private". Under UnescapePath the router decodes nothing past its single pass, so they stay
			// literal there too.
			name: "forged escape", mounts: []string{"wildcard", "prefix"}, config: Config{CacheDuration: -1},
			targets: []string{"/static/%%370rivate/secret.txt", "/static/%7%30rivate/secret.txt"},
		},
		{
			// Windows strips a trailing dot or space from a path component, so the file server rejects such a segment
			// there, and everywhere else no such directory exists.
			name: "trailing dot or space", mounts: []string{"wildcard"}, config: Config{CacheDuration: -1},
			targets: []string{
				"/static/private./secret.txt", "/static/private.../secret.txt",
				"/static/private%20/secret.txt", "/static/private%20./secret.txt",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, unescape := range []bool{false, true} {
				for _, mount := range tc.mounts {
					app := staticTestApp(unescape, "/static/private")
					mounts[mount](app, tc.config)

					// the spelling the router does match the guard on still returns 403
					status, _ := staticTestServeOpaque(t, app, "/static/private/secret.txt")
					require.Equal(t, fiber.StatusForbidden, status, "unescape=%v mount=%s", unescape, mount)

					for _, target := range tc.targets {
						status, body := staticTestServeOpaque(t, app, target)
						label := fmt.Sprintf("unescape=%v mount=%s target=%s", unescape, mount, target)
						require.Equal(t, fiber.StatusNotFound, status, label)
						require.NotContains(t, body, "SECRET", label)
					}
				}
			}
		})
	}
}

// Test_Static_MalformedEscapeIsNotAName pins that a file whose name holds a
// percent sign has one spelling: "%zz" is not an escape, so the router keeps
// it apart from "%25zz" and the file server does not open the file it would
// name literally. Under UnescapePath both spellings route to the same path,
// which the file server takes as it is. httptest rejects the spelling, so the
// request line is set by hand.
func Test_Static_MalformedEscapeIsNotAName(t *testing.T) {
	t.Parallel()
	root := staticTestRoot(t, map[string]string{"100%zz.txt": "PERCENT"})

	for _, unescape := range []bool{false, true} {
		app := staticTestApp(unescape)
		app.Get("/static/*", New(root, Config{CacheDuration: -1}))
		for _, target := range []string{"/static/100%25zz.txt", "/static/100%zz.txt"} {
			status, body := staticTestServeOpaque(t, app, target)
			if !unescape {
				require.Equal(t, fiber.StatusNotFound, status, "target=%s", target)
				require.NotContains(t, body, "PERCENT", "target=%s", target)
				continue
			}
			require.Equal(t, fiber.StatusOK, status, "target=%s", target)
			require.Equal(t, "PERCENT", body, "target=%s", target)
		}
	}
}

// Test_Static_SharedHandlerAcrossApps pins that one handler value mounted in
// two apps decodes as each app's UnescapePath says, whichever app served
// first: under the flag the router already decoded "%2561" to "%61", and the
// file server must not decode it again into "a.txt".
func Test_Static_SharedHandlerAcrossApps(t *testing.T) {
	t.Parallel()
	root := staticTestRoot(t, map[string]string{"a.txt": "A"})
	handler := New(root, Config{CacheDuration: -1})

	plain := fiber.New()
	plain.Get("/static/*", handler)
	unescaped := fiber.New(fiber.Config{UnescapePath: true})
	unescaped.Get("/static/*", handler)

	serve := func(app *fiber.App, target string) int {
		resp := apptest.Do(t, app, target)
		require.NoError(t, resp.Body.Close())
		return resp.StatusCode
	}

	// the app that serves first must not fix the decoding mode of the other
	require.Equal(t, fiber.StatusNotFound, serve(plain, "/static/%2561.txt"))
	require.Equal(t, fiber.StatusOK, serve(plain, "/static/%61.txt"))
	require.Equal(t, fiber.StatusNotFound, serve(unescaped, "/static/%2561.txt"))
	require.Equal(t, fiber.StatusOK, serve(unescaped, "/static/%61.txt"))
}

func Test_Static_Download_NotFoundLeavesNoAttachment(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/files*", New("../../.github/testdata/fs", Config{Download: true}))
	app.Get("/files/users", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })

	resp := apptest.Expect(t, app, "/files/users", fiber.StatusOK)
	require.Equal(t, fiber.MIMEApplicationJSONCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
	require.Empty(t, resp.Header.Get(fiber.HeaderContentDisposition))
}

func Test_Static_Download_KeepsDetectedContentType(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	// fiberpng is a PNG without an extension, so only content detection can type it.
	app.Get("/docs*", New("../../.github/testdata/fs/img", Config{Download: true}))

	resp := apptest.Expect(t, app, "/docs/fiberpng", fiber.StatusOK)
	require.Contains(t, resp.Header.Get(fiber.HeaderContentType), "image/png")
	require.Equal(t, `attachment; filename="fiberpng"`, resp.Header.Get(fiber.HeaderContentDisposition))
}

func Test_Static_Download_NoAttachmentOnRangeError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/docs*", New("../../.github/testdata/fs/img", Config{Download: true, ByteRange: true}))

	req := httptest.NewRequest(fiber.MethodGet, "/docs/fiberpng", http.NoBody)
	req.Header.Set(fiber.HeaderRange, "bytes=99999999-99999999")
	resp := apptest.Expect(t, app, req, fiber.StatusRequestedRangeNotSatisfiable)
	require.Empty(t, resp.Header.Get(fiber.HeaderContentDisposition))
}

func Test_Static_SameHandlerUnderTwoPrefixes(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	handler := New("../../.github/testdata/fs")
	app.Get("/static/*", handler)
	app.Get("/s/*", handler)

	for _, path := range []string{"/static/index.html", "/s/index.html", "/static/index.html"} {
		resp := apptest.Do(t, app, path)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "GET %s", path)
		body := apptest.Body(t, resp)
		require.Contains(t, body, "<h1>Hello, World!</h1>", "GET %s", path)
	}
}

func Test_Static_MaxAge_NotOnErrorResponses(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/static/*", New("../../.github/testdata/fs", Config{ByteRange: true, MaxAge: 3600}))

	req := httptest.NewRequest(fiber.MethodGet, "/static/index.html", http.NoBody)
	req.Header.Set(fiber.HeaderRange, "bytes=999999-9999999")
	resp := apptest.Expect(t, app, req, fiber.StatusRequestedRangeNotSatisfiable)
	require.Empty(t, resp.Header.Get(fiber.HeaderCacheControl))

	resp = apptest.Expect(t, app, "/static/index.html", fiber.StatusOK)
	require.Equal(t, "public, max-age=3600", resp.Header.Get(fiber.HeaderCacheControl))
}

// With ByteRange on, the Range and If-Range rules (RFC 9110 §13.1.5, §14.2) are
// applied around the file server's single range; internal/byterange tests them
// in full.
func Test_Static_ByteRange_RFC9110(t *testing.T) {
	t.Parallel()
	const (
		content      = "0123456789abcdefghij"
		lastModified = "Thu, 02 Jan 2020 03:04:05 GMT"
		staleDate    = "Wed, 01 Jan 2020 03:04:05 GMT"
	)
	dir := t.TempDir()
	name := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(name, []byte(content), 0o600))
	require.NoError(t, os.Chtimes(name, time.Time{}, time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)))

	app := fiber.New()
	app.Get("/ranges/*", New(dir, Config{ByteRange: true}))
	app.Get("/plain/*", New(dir))

	tests := []struct {
		fields       map[string]string
		name         string
		path         string
		wantRange    string
		wantBody     string
		wantStatus   int
		wantWhole    bool // the whole file, with its validators
		wantAdvised  bool // Accept-Ranges is advertised
		wantNoRanges bool // Accept-Ranges must be absent
	}{
		{name: "a range", path: "/ranges/file.txt", fields: map[string]string{fiber.HeaderRange: "bytes=0-4"}, wantStatus: fiber.StatusPartialContent, wantBody: "01234", wantRange: "bytes 0-4/20", wantAdvised: true},
		{name: "unit in mixed case", path: "/ranges/file.txt", fields: map[string]string{fiber.HeaderRange: "Bytes=0-4"}, wantStatus: fiber.StatusPartialContent, wantBody: "01234", wantRange: "bytes 0-4/20", wantAdvised: true},
		{name: "If-Range is stale", path: "/ranges/file.txt", fields: map[string]string{fiber.HeaderRange: "bytes=0-4", fiber.HeaderIfRange: staleDate}, wantStatus: fiber.StatusOK, wantBody: content, wantWhole: true, wantAdvised: true},
		{name: "unsatisfiable", path: "/ranges/file.txt", fields: map[string]string{fiber.HeaderRange: "bytes=100-200"}, wantStatus: fiber.StatusRequestedRangeNotSatisfiable, wantBody: "Range Not Satisfiable", wantRange: "bytes */20"},
		// ByteRange off: no ranges are offered and Range is not looked at.
		{name: "ByteRange off", path: "/plain/file.txt", fields: map[string]string{fiber.HeaderRange: "bytes=0-4"}, wantStatus: fiber.StatusOK, wantBody: content, wantWhole: true, wantNoRanges: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(fiber.MethodGet, tc.path, http.NoBody)
			for k, v := range tc.fields {
				req.Header.Set(k, v)
			}
			resp := apptest.Do(t, app, req)
			t.Cleanup(func() { require.NoError(t, resp.Body.Close()) })

			require.Equal(t, tc.wantStatus, resp.StatusCode)
			require.Equal(t, tc.wantRange, resp.Header.Get(fiber.HeaderContentRange))
			require.Equal(t, tc.wantBody, apptest.Body(t, resp))

			if tc.wantAdvised {
				require.Equal(t, "bytes", resp.Header.Get(fiber.HeaderAcceptRanges))
			}
			if tc.wantNoRanges {
				require.Empty(t, resp.Header.Get(fiber.HeaderAcceptRanges))
			}
			if tc.wantWhole {
				require.Equal(t, "20", resp.Header.Get(fiber.HeaderContentLength))
				require.Equal(t, lastModified, resp.Header.Get(fiber.HeaderLastModified))
			}
		})
	}
}

func Test_Static_NonGetMethod_PassesThrough(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use("/", New("../../.github"))
	app.Post("/index.html", func(c fiber.Ctx) error { return c.SendString("posted") })

	apptest.ExpectBody(t, app, apptest.Req(fiber.MethodPost, "/index.html"), fiber.StatusOK, "posted")

	get := apptest.Expect(t, app, "/index.html", fiber.StatusOK)
	served := apptest.Body(t, get)
	require.Contains(t, served, "Hello, World!")
}

// Test_Static_PartialWildcard_SegmentBoundary checks that a partial wildcard
// route serves only what lies below its path segment, so middleware mounted on
// /static/private cannot be skipped with /staticprivate/secret.txt.
func Test_Static_PartialWildcard_SegmentBoundary(t *testing.T) {
	t.Parallel()
	root := staticTestRoot(t, map[string]string{"private/secret.txt": "SECRET", "x": "PUBLIC"})

	app := staticTestApp(false, "/static/private")
	app.Get("/static*", New(root, Config{CacheDuration: -1}))

	for target, want := range map[string]int{
		"/static/private/secret.txt": fiber.StatusForbidden,
		"/staticprivate/secret.txt":  fiber.StatusNotFound,
		"/staticx":                   fiber.StatusNotFound,
		"/static/x":                  fiber.StatusOK,
	} {
		resp := apptest.Do(t, app, target)
		body := apptest.Body(t, resp)
		require.Equal(t, want, resp.StatusCode, "target=%s", target)
		require.NotContains(t, body, "SECRET", "target=%s", target)
	}
}

// Test_Static_NonMountRoutes_Served checks that routes with a plus, a parameter
// or a slash before the wildcard serve what they matched.
func Test_Static_NonMountRoutes_Served(t *testing.T) {
	t.Parallel()
	root := staticTestRoot(t, map[string]string{"app.js": "JS", "index.html": "INDEX"})

	app := fiber.New()
	app.Get("/static+", New(root, Config{CacheDuration: -1}))
	app.Get("/users/:id", New(filepath.Join(root, "app.js"), Config{CacheDuration: -1}))
	app.Get("/:lang/*", New(filepath.Join(root, "index.html"), Config{CacheDuration: -1}))

	for target, want := range map[string]string{"/static/app.js": "JS", "/en/dashboard": "INDEX", "/users/1234": "JS"} {
		resp := apptest.Do(t, app, target)
		body := apptest.Body(t, resp)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "target=%s", target)
		require.Equal(t, want, body, "target=%s", target)
	}
}

func Test_Static_Symlinks(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "shared.txt"), []byte("shared"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "local.txt"), []byte("local"), 0o600))
	for link, target := range map[string]string{
		"outside.txt":  filepath.Join(outside, "shared.txt"),
		"absolute.txt": filepath.Join(root, "local.txt"),
		"relative.txt": "local.txt",
	} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}

	app := fiber.New()
	app.Get("/confined/*", New(root))
	app.Get("/open/*", New(root, Config{AllowSymlinksOutsideRoot: true}))

	for path, want := range map[string]int{
		"/confined/relative.txt": fiber.StatusOK,
		"/confined/absolute.txt": fiber.StatusNotFound,
		"/confined/outside.txt":  fiber.StatusNotFound,
		"/open/absolute.txt":     fiber.StatusOK,
		"/open/outside.txt":      fiber.StatusOK,
	} {
		resp := apptest.Expect(t, app, path, want)
		require.NoError(t, resp.Body.Close())
	}
}

// Test_Static_StripsRoutedPrefix pins that the prefix cut off the routed path is
// the text the router matched, not the pattern, which a plus, an escape or a doubled slash outgrows.
func Test_Static_StripsRoutedPrefix(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "private"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "private", "secret.txt"), []byte("SECRET"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "x"), []byte("PUBLIC"), 0o600))

	get := func(app *fiber.App, pattern string, handler fiber.Handler) { app.Get(pattern, handler) }
	use := func(app *fiber.App, pattern string, handler fiber.Handler) { app.Use(pattern, handler) }

	for _, tc := range []struct {
		register func(app *fiber.App, pattern string, handler fiber.Handler)
		pattern  string
		prefix   string
		strict   bool
	}{
		{pattern: "/static+", prefix: "/static", register: get},
		{pattern: "/static+", prefix: "/static", register: use},
		{pattern: "/static/+", prefix: "/static", register: get},
		{pattern: `/v1\:static`, prefix: "/v1:static", register: use},
		{pattern: `/v1\:static*`, prefix: "/v1:static", register: get},
		{pattern: `/static\**`, prefix: "/static*", register: get},
		{pattern: "/static//", prefix: "/static", register: use},
		{pattern: "/static//", prefix: "/static/", register: use, strict: true},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			t.Parallel()
			app := fiber.New(fiber.Config{StrictRouting: tc.strict})
			guard := strings.NewReplacer(":", `\:`, "*", `\*`).Replace(tc.prefix) + "/private"
			app.Use(guard, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusForbidden) })
			tc.register(app, tc.pattern, New(root, Config{CacheDuration: -1}))
			h := app.Handler()

			for target, want := range map[string]int{
				tc.prefix + "/x":                   fiber.StatusOK,
				tc.prefix + "/private/secret.txt":  fiber.StatusForbidden,
				tc.prefix + "//private/secret.txt": fiber.StatusNotFound,
				tc.prefix + "private/secret.txt":   fiber.StatusNotFound,
				tc.prefix + "Xprivate/secret.txt":  fiber.StatusNotFound,
			} {
				resp := serveRaw(h, fiber.MethodGet, target)
				require.Equal(t, want, resp.StatusCode(), "target=%s", target)
				require.NotContains(t, string(resp.Body()), "SECRET", "target=%s", target)
			}
		})
	}
}

// Test_Static_ParameterRoutes pins that after a parameter a directory root takes the file
// name from the trailing wildcard's value, and serves nothing on a route without one. A
// route whose first parameter is a wildcard keeps the text before it as the prefix.
func Test_Static_ParameterRoutes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "private"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "private", "secret.txt"), []byte("SECRET"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "private", "secret.js"), []byte("SECRET"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "app.js"), []byte("JS"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "index.html"), []byte("DOCS"), 0o600))

	deny := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusForbidden) }

	app := fiber.New()
	app.Use("/:lang/private", deny)
	app.Get("/:lang/*", New(root, Config{CacheDuration: -1}))
	used := fiber.New()
	used.Use("/private", deny)
	used.Use(New(root, Config{CacheDuration: -1}))
	app.Use("/u/:tenant", used)
	wild := fiber.New()
	wild.Use("/private", deny)
	wild.Get("/*", New(root, Config{CacheDuration: -1}))
	app.Use("/w/:tenant", wild)
	h := app.Handler()

	for target, want := range map[string]int{
		"/en/app.js":                    fiber.StatusOK,
		"/en-US/app.js":                 fiber.StatusOK,
		"/en/docs/":                     fiber.StatusOK,
		"/en/docs":                      fiber.StatusOK,
		"/en/private/secret.txt":        fiber.StatusForbidden,
		"/abcdeprivate/secret.txt":      fiber.StatusNotFound,
		"/abcde//private/secret.txt":    fiber.StatusNotFound,
		"/u/abcdefg/app.js":             fiber.StatusNotFound,
		"/u/abcdefg/private/secret.txt": fiber.StatusForbidden,
		"/u/abcdefgprivate/secret.txt":  fiber.StatusNotFound,
		"/w/acme/app.js":                fiber.StatusOK,
		"/w/acme/private/secret.txt":    fiber.StatusForbidden,
		"/w/acmeprivate/secret.txt":     fiber.StatusNotFound,
		"/w/acme//private/secret.txt":   fiber.StatusNotFound,
	} {
		resp := serveRaw(h, fiber.MethodGet, target)
		require.Equal(t, want, resp.StatusCode(), "target=%s", target)
		require.NotContains(t, string(resp.Body()), "SECRET", "target=%s", target)
	}

	for _, tc := range []struct {
		targets map[string]int
		route   string
		strict  bool
	}{
		{route: "/assets/*.js", targets: map[string]int{
			"/assets/app.js":            fiber.StatusOK,
			"/assets/private/secret.js": fiber.StatusForbidden,
			"/assetsapp.js":             fiber.StatusNotFound,
		}},
		{route: "/assets/*/index.html", targets: map[string]int{
			"/assets/docs/index.html":    fiber.StatusOK,
			"/assets/private/index.html": fiber.StatusForbidden,
		}},
		{route: "/:lang?/*", targets: map[string]int{
			"/en/app.js":             fiber.StatusOK,
			"/en/private/secret.txt": fiber.StatusForbidden,
			"//app.js":               fiber.StatusNotFound,
			"//private/secret.txt":   fiber.StatusNotFound,
		}},
		{route: "/:lang?/*", strict: true, targets: map[string]int{
			"/en/docs": fiber.StatusFound,
			"//docs":   fiber.StatusNotFound,
		}},
		{route: "/*/*", targets: map[string]int{
			"/app.js":             fiber.StatusOK,
			"/x/app.js":           fiber.StatusNotFound,
			"/private/secret.txt": fiber.StatusForbidden,
		}},
		{route: "/*/*", strict: true, targets: map[string]int{
			"/docs":  fiber.StatusFound,
			"//docs": fiber.StatusNotFound,
		}},
	} {
		app = fiber.New(fiber.Config{StrictRouting: tc.strict})
		app.Use("/private", deny)
		app.Use("/assets/private", deny)
		app.Use("/:lang/private", deny)
		app.Get(tc.route, New(root, Config{CacheDuration: -1}))
		h = app.Handler()

		for target, want := range tc.targets {
			msg := []any{"route=%s strict=%t target=%s", tc.route, tc.strict, target}
			resp := serveRaw(h, fiber.MethodGet, target)
			require.Equal(t, want, resp.StatusCode(), msg...)
			require.NotContains(t, string(resp.Body()), "SECRET", msg...)
			location := ""
			if want == fiber.StatusFound {
				location = target + "/"
			}
			require.Equal(t, location, string(resp.Header.Peek(fiber.HeaderLocation)), msg...)
		}
	}

	file := filepath.Join(root, "app.js")
	app = fiber.New()
	app.Get("/@:user", New(file))
	app.Get("/user-:id", New(file))
	app.Get("/v:ver/docs", New(file))
	app.Get("/users/:id", New(file))
	h = app.Handler()

	for _, target := range []string{"/@a", "/@alice", "/user-1", "/v1/docs", "/users/1", "/users/1234567890"} {
		resp := serveRaw(h, fiber.MethodGet, target)
		require.Equal(t, fiber.StatusOK, resp.StatusCode(), "target=%s", target)
		require.Equal(t, "JS", string(resp.Body()), "target=%s", target)
	}
}

// Test_Static_StrictRouting_TrailingSlash pins that under StrictRouting a directory
// is served for dir/ only and a file for file only, as the router keeps them apart.
func Test_Static_StrictRouting_TrailingSlash(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, content := range map[string]string{
		"admin/index.html": "ADMIN",
		"pub/index.html":   "INDEX",
		"a b/index.html":   "INDEX",
		"secret.txt":       "SECRET",
		"public.txt":       "PUBLIC",
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o600))
	}

	deny := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusUnauthorized) }

	rows := []struct {
		target   string
		location string
		status   int
	}{
		{target: "/files/public.txt", status: fiber.StatusOK},
		{target: "/files/public.txt/", status: fiber.StatusNotFound},
		{target: "/files/pub/", status: fiber.StatusOK},
		{target: "/files/pub", status: fiber.StatusFound, location: "/files/pub/"},
		{target: "/files/pub?a=1&b=%20", status: fiber.StatusFound, location: "/files/pub/?a=1&b=%20"},
		{target: "/files/x/../pub", status: fiber.StatusFound, location: "/files/pub/"},
		{target: "/files/", status: fiber.StatusNotFound},
		{target: "/files", status: fiber.StatusFound, location: "/files/"},
		{target: "/files/missing", status: fiber.StatusNotFound},
		{target: "/files/missing/", status: fiber.StatusNotFound},
		{target: "/files/admin/", status: fiber.StatusUnauthorized},
		{target: "/files/admin", status: fiber.StatusUnauthorized},
		{target: "/files/secret.txt", status: fiber.StatusUnauthorized},
		{target: "/files/secret.txt/", status: fiber.StatusNotFound},
	}

	for name, newHandler := range map[string]func() fiber.Handler{
		"os":     func() fiber.Handler { return New(root) },
		"fs":     func() fiber.Handler { return New("", Config{FS: os.DirFS(root)}) },
		"fs sub": func() fiber.Handler { return New(filepath.Base(root), Config{FS: os.DirFS(filepath.Dir(root))}) },
	} {
		for _, unescape := range []bool{false, true} {
			app := fiber.New(fiber.Config{StrictRouting: true, UnescapePath: unescape})
			app.Use("/files/admin/", deny)
			app.All("/files/secret.txt", deny)
			app.Use("/files", newHandler())
			h := app.Handler()

			for _, method := range []string{fiber.MethodGet, fiber.MethodHead} {
				for _, row := range rows {
					msg := []any{"%s unescape=%t %s %s", name, unescape, method, row.target}
					resp := serveRaw(h, method, row.target)
					require.Equal(t, row.status, resp.StatusCode(), msg...)
					require.Equal(t, row.location, string(resp.Header.Peek(fiber.HeaderLocation)), msg...)
					require.NotContains(t, string(resp.Body()), "ADMIN", msg...)
					require.NotContains(t, string(resp.Body()), "SECRET", msg...)
				}
			}

			if unescape {
				resp := serveRaw(h, fiber.MethodGet, "/files/a%20b")
				require.Equal(t, fiber.StatusFound, resp.StatusCode(), name)
				require.Equal(t, "/files/a%20b/", string(resp.Header.Peek(fiber.HeaderLocation)), name)
				require.Equal(t, fiber.StatusOK, serveRaw(h, fiber.MethodGet, "/files/a%20b/").StatusCode(), name)
			}
		}
	}

	// the redirect names the routed path, mount prefix included
	sub := fiber.New()
	sub.Use("/admin/", deny)
	sub.Use("/", New(root))
	app := fiber.New(fiber.Config{StrictRouting: true})
	app.Use("/files", sub)
	h := app.Handler()
	resp := serveRaw(h, fiber.MethodGet, "/files/pub")
	require.Equal(t, fiber.StatusFound, resp.StatusCode())
	require.Equal(t, "/files/pub/", string(resp.Header.Peek(fiber.HeaderLocation)))
	require.Equal(t, fiber.StatusUnauthorized, serveRaw(h, fiber.MethodGet, "/files/admin").StatusCode())
	require.Equal(t, fiber.StatusUnauthorized, serveRaw(h, fiber.MethodGet, "/files/admin/").StatusCode())
	require.Equal(t, fiber.StatusNotFound, serveRaw(h, fiber.MethodGet, "/files/public.txt/").StatusCode())

	// without StrictRouting the router reads both spellings as one path, and so does the file server
	app = fiber.New()
	app.Use("/files/admin/", deny)
	app.All("/files/secret.txt", deny)
	app.Use("/files", New(root))
	h = app.Handler()
	for target, want := range map[string]int{
		"/files/public.txt":  fiber.StatusOK,
		"/files/public.txt/": fiber.StatusOK,
		"/files/pub":         fiber.StatusOK,
		"/files/pub/":        fiber.StatusOK,
		"/files/admin":       fiber.StatusUnauthorized,
		"/files/admin/":      fiber.StatusUnauthorized,
		"/files/secret.txt":  fiber.StatusUnauthorized,
		"/files/secret.txt/": fiber.StatusUnauthorized,
	} {
		require.Equal(t, want, serveRaw(h, fiber.MethodGet, target).StatusCode(), "target=%s", target)
	}
}

// Test_Static_Browse_LinksFollowRoutedPath pins that a listing, and the copy cached
// for later clients, links below the routed path, not fasthttp's reading of the target.
func Test_Static_Browse_LinksFollowRoutedPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "pub"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "pub", "a.txt"), []byte("A"), 0o600))

	targets := []string{
		"/files/pub/",
		"/files/pub",
		"/files/pub/x%2Fy/..",
		"/files/pub/..%2F..%2Fother%2Fx/..",
		"/files/pub/..%2F..%2F%5Cevil.example%2Fx/..",
		"/files/pub/..%252F..%252F%255Cevil.example%252Fx/..",
	}
	roots := map[string]func(Config) fiber.Handler{
		"os": func(cfg Config) fiber.Handler { return New(root, cfg) },
		"fs": func(cfg Config) fiber.Handler { cfg.FS = os.DirFS(root); return New("", cfg) },
	}

	for _, target := range targets {
		for _, unescape := range []bool{false, true} {
			for name, handler := range roots {
				var after string
				app := fiber.New(fiber.Config{UnescapePath: unescape})
				app.Get("/files*", handler(Config{
					Browse: true,
					ModifyResponse: func(c fiber.Ctx) error {
						after = string(c.Req().URI().PathOriginal())
						return nil
					},
				}))
				h := app.Handler()

				// the first request builds the cached page the second one receives
				for _, sent := range []string{target, "/files/pub/"} {
					msg := []any{"root=%s unescape=%t first=%s target=%s", name, unescape, target, sent}
					resp := serveRaw(h, fiber.MethodGet, sent)
					if resp.StatusCode() == fiber.StatusNotFound && sent == target {
						// UnescapePath routes the decoded target elsewhere
						continue
					}
					body := string(resp.Body())
					require.Equal(t, fiber.StatusOK, resp.StatusCode(), msg...)
					require.Contains(t, body, "<title>/files/pub", msg...)
					require.Contains(t, body, `<a href="/files/" class="dir">..</a>`, msg...)
					require.Contains(t, body, `<a href="/files/pub/a.txt" class="file">`, msg...)
					require.NotContains(t, body, "evil.example", msg...)
					require.NotContains(t, body, "other", msg...)
					require.Equal(t, sent, after, msg...)
				}
			}
		}
	}

	// a parameter makes the prefix differ per request, so each listing links below its own
	for _, unescape := range []bool{false, true} {
		for name, handler := range roots {
			sub := fiber.New()
			sub.Get("/*", handler(Config{Browse: true}))
			mounted := fiber.New(fiber.Config{UnescapePath: unescape})
			mounted.Use("/t/:tenant", sub)
			param := fiber.New(fiber.Config{UnescapePath: unescape})
			param.Get("/:lang/*", handler(Config{Browse: true}))

			for prefix, app := range map[string]*fiber.App{"": param, "/t": mounted} {
				h := app.Handler()
				for _, value := range []string{`%5Cevil.example`, `%255Cevil.example`, `\evil.example`, `..%2F..%2F%5Cevil.example%2Fx`, "en", "de"} {
					target := prefix + "/" + value + "/pub/"
					msg := []any{"root=%s unescape=%t target=%s", name, unescape, target}
					resp := serveRaw(h, fiber.MethodGet, target)
					body := string(resp.Body())
					if value != "en" && value != "de" {
						require.Equal(t, fiber.StatusNotFound, resp.StatusCode(), msg...)
						continue
					}
					require.Equal(t, fiber.StatusOK, resp.StatusCode(), msg...)
					require.Contains(t, body, "<title>"+prefix+"/"+value+"/pub/</title>", msg...)
					require.Contains(t, body, `<a href="`+prefix+"/"+value+`/pub/a.txt" class="file">`, msg...)
					require.NotContains(t, body, "evil.example", msg...)
				}
			}
		}
	}
}

// Test_Static_Browse_ParameterPrefixControlByte pins that a listing refuses a
// parameter prefix with a control byte, which a browser drops from a link.
func Test_Static_Browse_ParameterPrefixControlByte(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "pub"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "pub", "a.txt"), []byte("A"), 0o600))

	for route, rest := range map[string]string{"/:lang/*": "/pub/", "/:a/:b/*": "/evil.example/"} {
		app := fiber.New(fiber.Config{UnescapePath: true})
		app.Get(route, New(root, Config{Browse: true}))
		h := app.Handler()

		require.Equal(t, fiber.StatusOK, serveRaw(h, fiber.MethodGet, "/en"+rest).StatusCode(), route)
		for _, escape := range []string{"%09", "%0A", "%0D", "%00", "%7F"} {
			for _, target := range []string{"/" + escape + rest, "/e" + escape + "n" + rest} {
				require.Equal(t, fiber.StatusNotFound, serveRaw(h, fiber.MethodGet, target).StatusCode(), "route=%s target=%s", route, target)
			}
		}
	}
}

func Benchmark_Static_Browse(b *testing.B) {
	root := b.TempDir()
	require.NoError(b, os.Mkdir(filepath.Join(root, "pub"), 0o750))
	require.NoError(b, os.WriteFile(filepath.Join(root, "pub", "a.txt"), []byte("A"), 0o600))

	for name, unescape := range map[string]bool{"default": false, "unescape": true} {
		b.Run(name, func(b *testing.B) {
			app := fiber.New(fiber.Config{UnescapePath: unescape})
			app.Use("/files", New(root, Config{Browse: true}))
			h := app.Handler()

			var req fasthttp.Request
			req.Header.SetMethod(fiber.MethodGet)
			req.SetRequestURI("/files/pub/a.txt")
			fctx := new(fasthttp.RequestCtx)
			fctx.Init(&req, nil, discardLogger{})

			h(fctx)
			require.Equal(b, fiber.StatusOK, fctx.Response.StatusCode())
			require.Equal(b, "A", string(fctx.Response.Body()))
			fctx.Response.Reset()

			b.ReportAllocs()
			for b.Loop() {
				h(fctx)
				fctx.Response.Reset()
			}
		})
	}
}
