package fiber

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
)

// go test -run Test_App_Static_Index_Default
func Test_Static_Index_Default(t *testing.T) {
	app := New()

	app.Use("/prefix", NewStatic("./.github/workflows"))
	app.Use("", NewStatic("./.github/"))
	app.Use("test", NewStatic("", StaticConfig{Index: "index.html"}))

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Hello, World!"))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/not-found", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "Cannot GET /not-found", string(body))
}

// go test -run Test_App_Static_Index
func Test_Static_Direct(t *testing.T) {
	app := New()

	app.Use("/", NewStatic("./.github"))

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/index.html", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Hello, World!"))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/testdata/testRoutes.json", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMEApplicationJSON, resp.Header.Get("Content-Type"))
	utils.AssertEqual(t, "", resp.Header.Get(HeaderCacheControl), "CacheControl Control")

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "testRoutes"))
}

// go test -run Test_App_Static_MaxAge
func Test_Static_MaxAge(t *testing.T) {
	app := New()

	app.Use("/", NewStatic("./.github", StaticConfig{MaxAge: 100}))

	resp, err := app.Test(httptest.NewRequest("GET", "/index.html", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, "text/html; charset=utf-8", resp.Header.Get(HeaderContentType))
	utils.AssertEqual(t, "public, max-age=100", resp.Header.Get(HeaderCacheControl), "CacheControl Control")
}

// go test -run Test_App_Static_Download
func Test_Static_Download(t *testing.T) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	app.Use("/fiber.png", NewStatic("./.github/testdata/fs/img/fiber.png", StaticConfig{Download: true}))

	resp, err := app.Test(httptest.NewRequest("GET", "/fiber.png", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, "image/png", resp.Header.Get(HeaderContentType))
	utils.AssertEqual(t, `attachment`, resp.Header.Get(HeaderContentDisposition))
}

func Test_Static_Wildcard(t *testing.T) {
	app := New()

	app.Use("*", NewStatic("./.github/index.html"))

	req := httptest.NewRequest(MethodGet, "/yesyes/john/doe", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Test file"))
}

func Test_Static_Prefix_Wildcard(t *testing.T) {
	app := New()

	app.Use("/test/*", NewStatic("./.github/index.html"))

	req := httptest.NewRequest(MethodGet, "/test/john/doe", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Use("/my/nameisjohn*", NewStatic("./.github/index.html"))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/my/nameisjohn/no/its/not", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Test file"))
}

func Test_Static_Prefix(t *testing.T) {
	app := New()
	app.Use("/john", NewStatic("./.github"))

	req := httptest.NewRequest(MethodGet, "/john/index.html", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Use("/prefix", NewStatic("./.github/testdata"))

	req = httptest.NewRequest(MethodGet, "/prefix/index.html", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Use("/single", NewStatic("./.github/testdata/testRoutes.json"))

	req = httptest.NewRequest(MethodGet, "/single", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMEApplicationJSON, resp.Header.Get(HeaderContentType))
}

func Test_Static_Trailing_Slash(t *testing.T) {
	app := New()
	app.Use("/john", NewStatic("./.github"))

	req := httptest.NewRequest(MethodGet, "/john/", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Use("/john_without_index", NewStatic("./.github/testdata/fs/css"))

	req = httptest.NewRequest(MethodGet, "/john_without_index/", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Use("/john/", NewStatic("./.github"))

	req = httptest.NewRequest(MethodGet, "/john/", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	req = httptest.NewRequest(MethodGet, "/john", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Use("/john_without_index/", NewStatic("./.github/testdata/fs/css"))

	req = httptest.NewRequest(MethodGet, "/john_without_index/", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))
}

func Test_Static_Next(t *testing.T) {
	app := New()
	app.Use("/", NewStatic(".github", StaticConfig{
		Next: func(c *Ctx) bool {
			// If value of the header is any other from "skip"
			// c.Next() will be invoked
			return c.Get("X-Custom-Header") == "skip"
		},
	}))
	app.Get("/", func(c *Ctx) error {
		return c.SendString("You've skipped app.Static")
	})

	t.Run("app.Static is skipped: invoking Get handler", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Custom-Header", "skip")
		resp, err := app.Test(req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 200, resp.StatusCode)
		utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
		utils.AssertEqual(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))

		body, err := io.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, true, strings.Contains(string(body), "You've skipped app.Static"))
	})

	t.Run("app.Static is not skipped: serving index.html", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Custom-Header", "don't skip")
		resp, err := app.Test(req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 200, resp.StatusCode)
		utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
		utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

		body, err := io.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, true, strings.Contains(string(body), "Hello, World!"))
	})
}

func Test_Route_Static_Root(t *testing.T) {
	dir := "./.github/testdata/fs/css"
	app := New()
	app.Use("/", NewStatic(dir, StaticConfig{
		Browse: true,
	}))

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/style.css", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, true, strings.Contains(app.getString(body), "color"))

	app = New()
	app.Use("/", NewStatic(dir))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/style.css", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, true, strings.Contains(app.getString(body), "color"))
}

func Test_Route_Static_HasPrefix(t *testing.T) {
	dir := "./.github/testdata/fs/css"
	app := New()
	app.Use("/static", NewStatic(dir, StaticConfig{
		Browse: true,
	}))

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/static", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/static/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/static/style.css", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, true, strings.Contains(app.getString(body), "color"))

	app = New()
	app.Use("/static/", NewStatic(dir, StaticConfig{
		Browse: true,
	}))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/static", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/static/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/static/style.css", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, true, strings.Contains(app.getString(body), "color"))

	app = New()
	app.Use("/static", NewStatic(dir))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/static", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/static/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/static/style.css", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, true, strings.Contains(app.getString(body), "color"))

	app = New()
	app.Use("/static/", NewStatic(dir))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/static", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/static/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/static/style.css", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, true, strings.Contains(app.getString(body), "color"))
}
