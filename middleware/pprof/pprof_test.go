//nolint:bodyclose // Much easier to just ignore memory leaks in tests
package pprof

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

func Test_Non_Pprof_Path(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("escaped")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "escaped", string(b))
}

func Test_Non_Pprof_Path_WithPrefix(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Use(New(Config{Prefix: "/federated-fiber"}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("escaped")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "escaped", string(b))
}

func Test_Pprof_Index(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("escaped")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/debug/pprof/", http.NoBody))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
	utils.AssertEqual(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	b, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, bytes.Contains(b, []byte("<title>/debug/pprof/</title>")))
}

func Test_Pprof_Index_WithPrefix(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Use(New(Config{Prefix: "/federated-fiber"}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("escaped")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/federated-fiber/debug/pprof/", http.NoBody))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
	utils.AssertEqual(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	b, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, bytes.Contains(b, []byte("<title>/debug/pprof/</title>")))
}

func Test_Pprof_Subs(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("escaped")
	})

	subs := []string{
		"cmdline", "profile", "symbol", "trace", "allocs", "block",
		"goroutine", "heap", "mutex", "threadcreate",
	}

	for _, sub := range subs {
		sub := sub

		t.Run(sub, func(t *testing.T) {
			t.Parallel()
			target := "/debug/pprof/" + sub
			if sub == "profile" {
				target += "?seconds=1"
			}
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, target, http.NoBody), 5000)
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, 200, resp.StatusCode)
		})
	}
}

//nolint:paralleltest // TODO: Investigate why this makes tests fail
func Test_Pprof_Subs_WithPrefix(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Use(New(Config{Prefix: "/federated-fiber"}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("escaped")
	})

	subs := []string{
		"cmdline", "profile", "symbol", "trace", "allocs", "block",
		"goroutine", "heap", "mutex", "threadcreate",
	}

	//nolint:paralleltest // TODO: Investigate why this makes tests fail
	for _, sub := range subs {
		sub := sub
		t.Run(sub, func(t *testing.T) {
			target := "/federated-fiber/debug/pprof/" + sub
			if sub == "profile" {
				target += "?seconds=1"
			}
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, target, http.NoBody), 5000)
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, 200, resp.StatusCode)
		})
	}
}

func Test_Pprof_Other(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("escaped")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/debug/pprof/302", http.NoBody))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 302, resp.StatusCode)
}

func Test_Pprof_Other_WithPrefix(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Use(New(Config{Prefix: "/federated-fiber"}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("escaped")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/federated-fiber/debug/pprof/302", http.NoBody))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 302, resp.StatusCode)
}

// go test -run Test_Pprof_Next
func Test_Pprof_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/debug/pprof/", http.NoBody))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 404, resp.StatusCode)
}

// go test -run Test_Pprof_Next_WithPrefix
func Test_Pprof_Next_WithPrefix(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
		Prefix: "/federated-fiber",
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/federated-fiber/debug/pprof/", http.NoBody))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 404, resp.StatusCode)
}
