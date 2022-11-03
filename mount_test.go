// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// go test -run Test_App_Mount
func Test_App_Mount(t *testing.T) {
	micro := New()
	micro.Get("/doe", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	app := New()
	app.Use("/john", micro)
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john/doe", nil))
	require.Equal(t, nil, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, uint32(1), app.handlersCount)
}

// go test -run Test_App_Mount_Nested
func Test_App_Mount_Nested(t *testing.T) {
	app := New()
	one := New()
	two := New()
	three := New()

	two.Use("/three", three)
	app.Use("/one", one)
	one.Use("/two", two)

	one.Get("/doe", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	two.Get("/nested", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	three.Get("/test", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/one/doe", nil))
	require.Equal(t, nil, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/one/two/nested", nil))
	require.Equal(t, nil, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/one/two/three/test", nil))
	require.Equal(t, nil, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	require.Equal(t, uint32(3), app.handlersCount)
}

// go test -run Test_App_MountPath
func Test_App_MountPath(t *testing.T) {
	app := New()
	one := New()
	two := New()
	three := New()

	two.Use("/three", three)
	one.Use("/two", two)
	app.Use("/one", one)

	require.Equal(t, "/one", one.MountPath())
	require.Equal(t, "/one/two", two.MountPath())
	require.Equal(t, "/one/two/three", three.MountPath())
	require.Equal(t, "", app.MountPath())
}

func Test_App_ErrorHandler_GroupMount(t *testing.T) {
	micro := New(Config{
		ErrorHandler: func(c Ctx, err error) error {
			require.Equal(t, "0: GET error", err.Error())
			return c.Status(500).SendString("1: custom error")
		},
	})
	micro.Get("/doe", func(c Ctx) error {
		return errors.New("0: GET error")
	})

	app := New()
	v1 := app.Group("/v1")
	v1.Use("/john", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", nil))
	testErrorResponse(t, err, resp, "1: custom error")
}

func Test_App_ErrorHandler_GroupMountRootLevel(t *testing.T) {
	micro := New(Config{
		ErrorHandler: func(c Ctx, err error) error {
			require.Equal(t, "0: GET error", err.Error())
			return c.Status(500).SendString("1: custom error")
		},
	})
	micro.Get("/john/doe", func(c Ctx) error {
		return errors.New("0: GET error")
	})

	app := New()
	v1 := app.Group("/v1")
	v1.Use("/", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", nil))
	testErrorResponse(t, err, resp, "1: custom error")
}

// go test -run Test_App_Group_Mount
func Test_App_Group_Mount(t *testing.T) {
	micro := New()
	micro.Get("/doe", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	app := New()
	v1 := app.Group("/v1")
	v1.Use("/john", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", nil))
	require.Equal(t, nil, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, uint32(1), app.handlersCount)
}

func Test_App_UseMountedErrorHandler(t *testing.T) {
	app := New()

	fiber := New(Config{
		ErrorHandler: func(c Ctx, err error) error {
			return c.Status(500).SendString("hi, i'm a custom error")
		},
	})
	fiber.Get("/", func(c Ctx) error {
		return errors.New("something happened")
	})

	app.Use("/api", fiber)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", nil))
	testErrorResponse(t, err, resp, "hi, i'm a custom error")
}

func Test_App_UseMountedErrorHandlerRootLevel(t *testing.T) {
	app := New()

	fiber := New(Config{
		ErrorHandler: func(c Ctx, err error) error {
			return c.Status(500).SendString("hi, i'm a custom error")
		},
	})
	fiber.Get("/api", func(c Ctx) error {
		return errors.New("something happened")
	})

	app.Use("/", fiber)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", nil))
	testErrorResponse(t, err, resp, "hi, i'm a custom error")
}

func Test_App_UseMountedErrorHandlerForBestPrefixMatch(t *testing.T) {
	app := New()

	tsf := func(c Ctx, err error) error {
		return c.Status(200).SendString("hi, i'm a custom sub sub fiber error")
	}
	tripleSubFiber := New(Config{
		ErrorHandler: tsf,
	})
	tripleSubFiber.Get("/", func(c Ctx) error {
		return errors.New("something happened")
	})

	sf := func(c Ctx, err error) error {
		return c.Status(200).SendString("hi, i'm a custom sub fiber error")
	}
	subfiber := New(Config{
		ErrorHandler: sf,
	})
	subfiber.Get("/", func(c Ctx) error {
		return errors.New("something happened")
	})
	subfiber.Use("/third", tripleSubFiber)

	f := func(c Ctx, err error) error {
		return c.Status(200).SendString("hi, i'm a custom error")
	}
	fiber := New(Config{
		ErrorHandler: f,
	})
	fiber.Get("/", func(c Ctx) error {
		return errors.New("something happened")
	})
	fiber.Use("/sub", subfiber)

	app.Use("/api", fiber)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/sub", nil))
	require.Equal(t, nil, err, "/api/sub req")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	b, err := io.ReadAll(resp.Body)
	require.Equal(t, nil, err, "iotuil.ReadAll()")
	require.Equal(t, "hi, i'm a custom sub fiber error", string(b), "Response body")

	resp2, err := app.Test(httptest.NewRequest(MethodGet, "/api/sub/third", nil))
	require.Equal(t, nil, err, "/api/sub/third req")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	b, err = io.ReadAll(resp2.Body)
	require.Equal(t, nil, err, "iotuil.ReadAll()")
	require.Equal(t, "hi, i'm a custom sub sub fiber error", string(b), "Third fiber Response body")
}

// go test -run Test_Ctx_Render_Mount
func Test_Ctx_Render_Mount(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{}
	engine.Load()

	sub := New(Config{
		Views: engine,
	})

	sub.Get("/:name", func(c Ctx) error {
		return c.Render("hello_world.tmpl", Map{
			"Name": c.Params("name"),
		})
	})

	app := New()
	app.Use("/hello", sub)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/hello/a", nil))
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	require.Equal(t, nil, err, "app.Test(req)")

	body, err := io.ReadAll(resp.Body)
	require.Equal(t, nil, err)
	require.Equal(t, "<h1>Hello a!</h1>", string(body))
}

// go test -run Test_Ctx_Render_Mount_ParentOrSubHasViews
func Test_Ctx_Render_Mount_ParentOrSubHasViews(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{}
	err := engine.Load()
	require.Equal(t, nil, err)

	engine2 := &testTemplateEngine{path: "testdata2"}
	err = engine2.Load()
	require.Equal(t, nil, err)

	engine3 := &testTemplateEngine{path: "testdata3"}
	err = engine3.Load()
	require.Equal(t, nil, err)

	sub := New(Config{
		Views: engine3,
	})

	sub2 := New(Config{
		Views: engine2,
	})

	app := New(Config{
		Views: engine,
	})

	app.Get("/test", func(c Ctx) error {
		return c.Render("index.tmpl", Map{
			"Title": "Hello, World!",
		})
	})

	sub.Get("/world/:name", func(c Ctx) error {
		return c.Render("hello_world.tmpl", Map{
			"Name": c.Params("name"),
		})
	})

	sub2.Get("/moment", func(c Ctx) error {
		return c.Render("bruh.tmpl", Map{})
	})

	sub.Use("/bruh", sub2)
	app.Use("/hello", sub)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/hello/world/a", nil))
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	require.Equal(t, nil, err, "app.Test(req)")

	body, err := io.ReadAll(resp.Body)
	require.Equal(t, nil, err)
	require.Equal(t, "<h1>Hello a!</h1>", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	require.Equal(t, nil, err, "app.Test(req)")

	body, err = io.ReadAll(resp.Body)
	require.Equal(t, nil, err)
	require.Equal(t, "<h1>Hello, World!</h1>", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/hello/bruh/moment", nil))
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	require.Equal(t, nil, err, "app.Test(req)")

	body, err = io.ReadAll(resp.Body)
	require.Equal(t, nil, err)
	require.Equal(t, "<h1>I'm Bruh</h1>", string(body))

}

func Test_Ctx_Render_MountGroup(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{}
	engine.Load()

	micro := New(Config{
		Views: engine,
	})

	micro.Get("/doe", func(c Ctx) error {
		return c.Render("hello_world.tmpl", Map{
			"Name": "doe",
		})
	})

	app := New()
	v1 := app.Group("/v1")
	v1.Use("/john", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", nil))
	require.Equal(t, nil, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.Equal(t, nil, err)
	require.Equal(t, "<h1>Hello doe!</h1>", string(body))
}
