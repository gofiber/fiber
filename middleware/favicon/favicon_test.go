//nolint:bodyclose // Much easier to just ignore memory leaks in tests
package favicon

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"

	"github.com/valyala/fasthttp"
)

// go test -run Test_Middleware_Favicon
func Test_Middleware_Favicon(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return nil
	})

	// Skip Favicon middleware
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/favicon.ico", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusNoContent, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodOptions, "/favicon.ico", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(fiber.MethodPut, "/favicon.ico", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusMethodNotAllowed, resp.StatusCode, "Status code")
	utils.AssertEqual(t, strings.Join([]string{fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions}, ", "), resp.Header.Get(fiber.HeaderAllow))
}

// go test -run Test_Middleware_Favicon_Not_Found
func Test_Middleware_Favicon_Not_Found(t *testing.T) {
	t.Parallel()
	defer func() {
		if err := recover(); err == nil {
			t.Fatal("should cache panic")
		}
	}()

	fiber.New().Use(New(Config{
		File: "non-exist.ico",
	}))
}

// go test -run Test_Middleware_Favicon_Found
func Test_Middleware_Favicon_Found(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		File: "../../.github/testdata/favicon.ico",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/favicon.ico", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "image/x-icon", resp.Header.Get(fiber.HeaderContentType))
	utils.AssertEqual(t, "public, max-age=31536000", resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")
}

// go test -run Test_Custom_Favicon_Url
func Test_Custom_Favicon_Url(t *testing.T) {
	app := fiber.New()
	const customURL = "/favicon.svg"
	app.Use(New(Config{
		File: "../../.github/testdata/favicon.ico",
		URL:  customURL,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, customURL, nil))

	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "image/x-icon", resp.Header.Get(fiber.HeaderContentType))
}

// go test -run Test_Custom_Favicon_Data
func Test_Custom_Favicon_Data(t *testing.T) {
	data, err := os.ReadFile("../../.github/testdata/favicon.ico")
	utils.AssertEqual(t, nil, err)

	app := fiber.New()

	app.Use(New(Config{
		Data: data,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/favicon.ico", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "image/x-icon", resp.Header.Get(fiber.HeaderContentType))
	utils.AssertEqual(t, "public, max-age=31536000", resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")
}

// mockFS wraps local filesystem for the purposes of
// Test_Middleware_Favicon_FileSystem located below
// TODO use os.Dir if fiber upgrades to 1.16
type mockFS struct{}

func (mockFS) Open(name string) (http.File, error) {
	if name == "/" {
		name = "."
	} else {
		name = strings.TrimPrefix(name, "/")
	}
	file, err := os.Open(name) //nolint:gosec // We're in a test func, so this is fine
	if err != nil {
		return nil, fmt.Errorf("failed to open: %w", err)
	}
	return file, nil
}

// go test -run Test_Middleware_Favicon_FileSystem
func Test_Middleware_Favicon_FileSystem(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		File:       "../../.github/testdata/favicon.ico",
		FileSystem: mockFS{},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/favicon.ico", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "image/x-icon", resp.Header.Get(fiber.HeaderContentType))
	utils.AssertEqual(t, "public, max-age=31536000", resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")
}

// go test -run Test_Middleware_Favicon_CacheControl
func Test_Middleware_Favicon_CacheControl(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		CacheControl: "public, max-age=100",
		File:         "../../.github/testdata/favicon.ico",
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/favicon.ico", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "image/x-icon", resp.Header.Get(fiber.HeaderContentType))
	utils.AssertEqual(t, "public, max-age=100", resp.Header.Get(fiber.HeaderCacheControl), "CacheControl Control")
}

// go test -v -run=^$ -bench=Benchmark_Middleware_Favicon -benchmem -count=4
func Benchmark_Middleware_Favicon(b *testing.B) {
	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c *fiber.Ctx) error {
		return nil
	})
	handler := app.Handler()

	c := &fasthttp.RequestCtx{}
	c.Request.SetRequestURI("/")

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		handler(c)
	}
}

// go test -run Test_Favicon_Next
func Test_Favicon_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}
