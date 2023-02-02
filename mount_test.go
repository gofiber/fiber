// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

//nolint:bodyclose // Much easier to just ignore memory leaks in tests
package fiber

import (
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2/internal/template/html"
	"github.com/gofiber/fiber/v2/utils"
)

// go test -run Test_App_Mount
func Test_App_Mount(t *testing.T) {
	t.Parallel()
	micro := New()
	micro.Get("/doe", func(c *Ctx) error {
		return c.SendStatus(StatusOK)
	})

	app := New()
	app.Mount("/john", micro)
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, uint32(2), app.handlersCount)
}

func Test_App_Mount_RootPath_Nested(t *testing.T) {
	t.Parallel()
	app := New()
	dynamic := New()
	apiserver := New()

	apiroutes := apiserver.Group("/v1")
	apiroutes.Get("/home", func(c *Ctx) error {
		return c.SendString("home")
	})

	dynamic.Mount("/api", apiserver)
	app.Mount("/", dynamic)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/v1/home", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, uint32(2), app.handlersCount)
}

// go test -run Test_App_Mount_Nested
func Test_App_Mount_Nested(t *testing.T) {
	t.Parallel()
	app := New()
	one := New()
	two := New()
	three := New()

	two.Mount("/three", three)
	app.Mount("/one", one)
	one.Mount("/two", two)

	one.Get("/doe", func(c *Ctx) error {
		return c.SendStatus(StatusOK)
	})

	two.Get("/nested", func(c *Ctx) error {
		return c.SendStatus(StatusOK)
	})

	three.Get("/test", func(c *Ctx) error {
		return c.SendStatus(StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/one/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/one/two/nested", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/one/two/three/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	utils.AssertEqual(t, uint32(6), app.handlersCount)
}

// go test -run Test_App_MountPath
func Test_App_MountPath(t *testing.T) {
	t.Parallel()
	app := New()
	one := New()
	two := New()
	three := New()

	two.Mount("/three", three)
	one.Mount("/two", two)
	app.Mount("/one", one)

	utils.AssertEqual(t, "/one", one.MountPath())
	utils.AssertEqual(t, "/one/two", two.MountPath())
	utils.AssertEqual(t, "/one/two/three", three.MountPath())
	utils.AssertEqual(t, "", app.MountPath())
}

func Test_App_ErrorHandler_GroupMount(t *testing.T) {
	t.Parallel()
	micro := New(Config{
		ErrorHandler: func(c *Ctx, err error) error {
			utils.AssertEqual(t, "0: GET error", err.Error())
			return c.Status(500).SendString("1: custom error")
		},
	})
	micro.Get("/doe", func(c *Ctx) error {
		return errors.New("0: GET error")
	})

	app := New()
	v1 := app.Group("/v1")
	v1.Mount("/john", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", nil))
	testErrorResponse(t, err, resp, "1: custom error")
}

func Test_App_ErrorHandler_GroupMountRootLevel(t *testing.T) {
	t.Parallel()
	micro := New(Config{
		ErrorHandler: func(c *Ctx, err error) error {
			utils.AssertEqual(t, "0: GET error", err.Error())
			return c.Status(500).SendString("1: custom error")
		},
	})
	micro.Get("/john/doe", func(c *Ctx) error {
		return errors.New("0: GET error")
	})

	app := New()
	v1 := app.Group("/v1")
	v1.Mount("/", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", nil))
	testErrorResponse(t, err, resp, "1: custom error")
}

// go test -run Test_App_Group_Mount
func Test_App_Group_Mount(t *testing.T) {
	t.Parallel()
	micro := New()
	micro.Get("/doe", func(c *Ctx) error {
		return c.SendStatus(StatusOK)
	})

	app := New()
	v1 := app.Group("/v1")
	v1.Mount("/john", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, uint32(2), app.handlersCount)
}

func Test_App_UseParentErrorHandler(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ErrorHandler: func(ctx *Ctx, err error) error {
			return ctx.Status(500).SendString("hi, i'm a custom error")
		},
	})

	fiber := New()
	fiber.Get("/", func(c *Ctx) error {
		return errors.New("something happened")
	})

	app.Mount("/api", fiber)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", nil))
	testErrorResponse(t, err, resp, "hi, i'm a custom error")
}

func Test_App_UseMountedErrorHandler(t *testing.T) {
	t.Parallel()
	app := New()

	fiber := New(Config{
		ErrorHandler: func(ctx *Ctx, err error) error {
			return ctx.Status(500).SendString("hi, i'm a custom error")
		},
	})
	fiber.Get("/", func(c *Ctx) error {
		return errors.New("something happened")
	})

	app.Mount("/api", fiber)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", nil))
	testErrorResponse(t, err, resp, "hi, i'm a custom error")
}

func Test_App_UseMountedErrorHandlerRootLevel(t *testing.T) {
	t.Parallel()
	app := New()

	fiber := New(Config{
		ErrorHandler: func(ctx *Ctx, err error) error {
			return ctx.Status(500).SendString("hi, i'm a custom error")
		},
	})
	fiber.Get("/api", func(c *Ctx) error {
		return errors.New("something happened")
	})

	app.Mount("/", fiber)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", nil))
	testErrorResponse(t, err, resp, "hi, i'm a custom error")
}

func Test_App_UseMountedErrorHandlerForBestPrefixMatch(t *testing.T) {
	t.Parallel()
	app := New()

	tsf := func(ctx *Ctx, err error) error {
		return ctx.Status(200).SendString("hi, i'm a custom sub sub fiber error")
	}
	tripleSubFiber := New(Config{
		ErrorHandler: tsf,
	})
	tripleSubFiber.Get("/", func(c *Ctx) error {
		return errors.New("something happened")
	})

	sf := func(ctx *Ctx, err error) error {
		return ctx.Status(200).SendString("hi, i'm a custom sub fiber error")
	}
	subfiber := New(Config{
		ErrorHandler: sf,
	})
	subfiber.Get("/", func(c *Ctx) error {
		return errors.New("something happened")
	})
	subfiber.Mount("/third", tripleSubFiber)

	f := func(ctx *Ctx, err error) error {
		return ctx.Status(200).SendString("hi, i'm a custom error")
	}
	fiber := New(Config{
		ErrorHandler: f,
	})
	fiber.Get("/", func(c *Ctx) error {
		return errors.New("something happened")
	})
	fiber.Mount("/sub", subfiber)

	app.Mount("/api", fiber)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/sub", nil))
	utils.AssertEqual(t, nil, err, "/api/sub req")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	b, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "iotuil.ReadAll()")
	utils.AssertEqual(t, "hi, i'm a custom sub fiber error", string(b), "Response body")

	resp2, err := app.Test(httptest.NewRequest(MethodGet, "/api/sub/third", nil))
	utils.AssertEqual(t, nil, err, "/api/sub/third req")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	b, err = io.ReadAll(resp2.Body)
	utils.AssertEqual(t, nil, err, "iotuil.ReadAll()")
	utils.AssertEqual(t, "hi, i'm a custom sub sub fiber error", string(b), "Third fiber Response body")
}

// go test -run Test_Ctx_Render_Mount
func Test_Ctx_Render_Mount(t *testing.T) {
	t.Parallel()

	sub := New(Config{
		Views: html.New("./.github/testdata/template", ".gohtml"),
	})

	sub.Get("/:name", func(ctx *Ctx) error {
		return ctx.Render("hello_world", Map{
			"Name": ctx.Params("name"),
		})
	})

	app := New()
	app.Mount("/hello", sub)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/hello/a", nil))
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, nil, err, "app.Test(req)")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello a!</h1>", string(body))
}

// go test -run Test_Ctx_Render_Mount_ParentOrSubHasViews
func Test_Ctx_Render_Mount_ParentOrSubHasViews(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{}
	err := engine.Load()
	utils.AssertEqual(t, nil, err)

	engine2 := &testTemplateEngine{path: "testdata2"}
	err = engine2.Load()
	utils.AssertEqual(t, nil, err)

	sub := New(Config{
		Views: html.New("./.github/testdata/template", ".gohtml"),
	})

	sub2 := New(Config{
		Views: engine2,
	})

	app := New(Config{
		Views: engine,
	})

	app.Get("/test", func(c *Ctx) error {
		return c.Render("index.tmpl", Map{
			"Title": "Hello, World!",
		})
	})

	sub.Get("/world/:name", func(c *Ctx) error {
		return c.Render("hello_world", Map{
			"Name": c.Params("name"),
		})
	})

	sub2.Get("/moment", func(c *Ctx) error {
		return c.Render("bruh.tmpl", Map{})
	})

	sub.Mount("/bruh", sub2)
	app.Mount("/hello", sub)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/hello/world/a", nil))
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, nil, err, "app.Test(req)")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello a!</h1>", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, nil, err, "app.Test(req)")

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello, World!</h1>", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/hello/bruh/moment", nil))
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, nil, err, "app.Test(req)")

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>I'm Bruh</h1>", string(body))
}

func Test_Ctx_Render_MountGroup(t *testing.T) {
	t.Parallel()

	micro := New(Config{
		Views: html.New("./.github/testdata/template", ".gohtml"),
	})

	micro.Get("/doe", func(c *Ctx) error {
		return c.Render("hello_world", Map{
			"Name": "doe",
		})
	})

	app := New()
	v1 := app.Group("/v1")
	v1.Mount("/john", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello doe!</h1>", string(body))
}
