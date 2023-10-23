// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

//nolint:bodyclose // Much easier to just ignore memory leaks in tests
package fiber

import (
	"errors"
	"io"
	"net/http"
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
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john/doe", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/v1/home", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/one/doe", http.NoBody))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/one/two/nested", http.NoBody))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/one/two/three/test", http.NoBody))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	utils.AssertEqual(t, uint32(6), app.handlersCount)
	utils.AssertEqual(t, uint32(6), app.routesCount)
}

// go test -run Test_App_Mount_Express_Behavior
func Test_App_Mount_Express_Behavior(t *testing.T) {
	t.Parallel()
	createTestHandler := func(body string) func(c *Ctx) error {
		return func(c *Ctx) error {
			return c.SendString(body)
		}
	}
	testEndpoint := func(app *App, route, expectedBody string, expectedStatusCode int) {
		resp, err := app.Test(httptest.NewRequest(MethodGet, route, http.NoBody))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		body, err := io.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, expectedStatusCode, resp.StatusCode, "Status code")
		utils.AssertEqual(t, expectedBody, string(body), "Unexpected response body")
	}

	app := New()
	subApp := New()
	// app setup
	{
		subApp.Get("/hello", createTestHandler("subapp hello!"))
		subApp.Get("/world", createTestHandler("subapp world!")) // <- wins

		app.Get("/hello", createTestHandler("app hello!")) // <- wins
		app.Mount("/", subApp)                             // <- subApp registration
		app.Get("/world", createTestHandler("app world!"))

		app.Get("/bar", createTestHandler("app bar!"))
		subApp.Get("/bar", createTestHandler("subapp bar!")) // <- wins

		subApp.Get("/foo", createTestHandler("subapp foo!")) // <- wins
		app.Get("/foo", createTestHandler("app foo!"))

		// 404 Handler
		app.Use(func(c *Ctx) error {
			return c.SendStatus(StatusNotFound)
		})
	}
	// expectation check
	testEndpoint(app, "/world", "subapp world!", StatusOK)
	testEndpoint(app, "/hello", "app hello!", StatusOK)
	testEndpoint(app, "/bar", "subapp bar!", StatusOK)
	testEndpoint(app, "/foo", "subapp foo!", StatusOK)
	testEndpoint(app, "/unknown", ErrNotFound.Message, StatusNotFound)

	utils.AssertEqual(t, uint32(17), app.handlersCount)
	utils.AssertEqual(t, uint32(16+9), app.routesCount)
}

// go test -run Test_App_Mount_RoutePositions
func Test_App_Mount_RoutePositions(t *testing.T) {
	t.Parallel()
	testEndpoint := func(app *App, route, expectedBody string) {
		resp, err := app.Test(httptest.NewRequest(MethodGet, route, http.NoBody))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		body, err := io.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
		utils.AssertEqual(t, expectedBody, string(body), "Unexpected response body")
	}

	app := New()
	subApp1 := New()
	subApp2 := New()
	// app setup
	{
		app.Use(func(c *Ctx) error {
			// set initial value
			c.Locals("world", "world")
			return c.Next()
		})
		app.Mount("/subApp1", subApp1)
		app.Use(func(c *Ctx) error {
			return c.Next()
		})
		app.Get("/bar", func(c *Ctx) error {
			return c.SendString("ok")
		})
		app.Use(func(c *Ctx) error {
			// is overwritten in case the positioning is not correct
			c.Locals("world", "hello")
			return c.Next()
		})
		methods := subApp2.Group("/subApp2")
		methods.Get("/world", func(c *Ctx) error {
			v, ok := c.Locals("world").(string)
			if !ok {
				panic("unexpected data type")
			}
			return c.SendString(v)
		})
		app.Mount("", subApp2)
	}

	testEndpoint(app, "/subApp2/world", "hello")

	routeStackGET := app.Stack()[0]
	utils.AssertEqual(t, true, routeStackGET[0].use)
	utils.AssertEqual(t, "/", routeStackGET[0].path)

	utils.AssertEqual(t, true, routeStackGET[1].use)
	utils.AssertEqual(t, "/", routeStackGET[1].path)
	utils.AssertEqual(t, true, routeStackGET[0].pos < routeStackGET[1].pos, "wrong position of route 0")

	utils.AssertEqual(t, false, routeStackGET[2].use)
	utils.AssertEqual(t, "/bar", routeStackGET[2].path)
	utils.AssertEqual(t, true, routeStackGET[1].pos < routeStackGET[2].pos, "wrong position of route 1")

	utils.AssertEqual(t, true, routeStackGET[3].use)
	utils.AssertEqual(t, "/", routeStackGET[3].path)
	utils.AssertEqual(t, true, routeStackGET[2].pos < routeStackGET[3].pos, "wrong position of route 2")

	utils.AssertEqual(t, false, routeStackGET[4].use)
	utils.AssertEqual(t, "/subapp2/world", routeStackGET[4].path)
	utils.AssertEqual(t, true, routeStackGET[3].pos < routeStackGET[4].pos, "wrong position of route 3")

	utils.AssertEqual(t, 5, len(routeStackGET))
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/sub", http.NoBody))
	utils.AssertEqual(t, nil, err, "/api/sub req")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	b, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "iotuil.ReadAll()")
	utils.AssertEqual(t, "hi, i'm a custom sub fiber error", string(b), "Response body")

	resp2, err := app.Test(httptest.NewRequest(MethodGet, "/api/sub/third", http.NoBody))
	utils.AssertEqual(t, nil, err, "/api/sub/third req")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	b, err = io.ReadAll(resp2.Body)
	utils.AssertEqual(t, nil, err, "iotuil.ReadAll()")
	utils.AssertEqual(t, "hi, i'm a custom sub sub fiber error", string(b), "Third fiber Response body")
}

// go test -run Test_Mount_Route_Names
func Test_Mount_Route_Names(t *testing.T) {
	// create sub-app with 2 handlers:
	subApp1 := New()
	subApp1.Get("/users", func(c *Ctx) error {
		url, err := c.GetRouteURL("add-user", Map{})
		utils.AssertEqual(t, err, nil)
		utils.AssertEqual(t, url, "/app1/users", "handler: app1.add-user") // the prefix is /app1 because of the mount
		// if subApp1 is not mounted, expected url just /users
		return nil
	}).Name("get-users")
	subApp1.Post("/users", func(c *Ctx) error {
		route := c.App().GetRoute("get-users")
		utils.AssertEqual(t, route.Method, MethodGet, "handler: app1.get-users method")
		utils.AssertEqual(t, route.Path, "/app1/users", "handler: app1.get-users path")
		return nil
	}).Name("add-user")

	// create sub-app with 2 handlers inside a group:
	subApp2 := New()
	app2Grp := subApp2.Group("/users").Name("users.")
	app2Grp.Get("", nil).Name("get")
	app2Grp.Post("", nil).Name("add")

	// put both sub-apps into root app
	rootApp := New()
	_ = rootApp.Mount("/app1", subApp1)
	_ = rootApp.Mount("/app2", subApp2)

	rootApp.startupProcess()

	// take route directly from sub-app
	route := subApp1.GetRoute("get-users")
	utils.AssertEqual(t, route.Method, MethodGet)
	utils.AssertEqual(t, route.Path, "/users")

	route = subApp1.GetRoute("add-user")
	utils.AssertEqual(t, route.Method, MethodPost)
	utils.AssertEqual(t, route.Path, "/users")

	// take route directly from sub-app with group
	route = subApp2.GetRoute("users.get")
	utils.AssertEqual(t, route.Method, MethodGet)
	utils.AssertEqual(t, route.Path, "/users")

	route = subApp2.GetRoute("users.add")
	utils.AssertEqual(t, route.Method, MethodPost)
	utils.AssertEqual(t, route.Path, "/users")

	// take route from root app (using names of sub-apps)
	route = rootApp.GetRoute("add-user")
	utils.AssertEqual(t, route.Method, MethodPost)
	utils.AssertEqual(t, route.Path, "/app1/users")

	route = rootApp.GetRoute("users.add")
	utils.AssertEqual(t, route.Method, MethodPost)
	utils.AssertEqual(t, route.Path, "/app2/users")

	// GetRouteURL inside handler
	req := httptest.NewRequest(MethodGet, "/app1/users", nil)
	resp, err := rootApp.Test(req)

	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	// ctx.App().GetRoute() inside handler
	req = httptest.NewRequest(MethodPost, "/app1/users", nil)
	resp, err = rootApp.Test(req)

	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/hello/a", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/hello/world/a", http.NoBody))
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, nil, err, "app.Test(req)")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello a!</h1>", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, nil, err, "app.Test(req)")

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello, World!</h1>", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/hello/bruh/moment", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", http.NoBody))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "<h1>Hello doe!</h1>", string(body))
}
