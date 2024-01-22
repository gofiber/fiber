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

	"github.com/stretchr/testify/require"
)

// go test -run Test_App_Mount
func Test_App_Mount(t *testing.T) {
	t.Parallel()
	micro := New()
	micro.Get("/doe", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	app := New()
	app.Use("/john", micro)
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, uint32(1), app.handlersCount)
}

func Test_App_Mount_RootPath_Nested(t *testing.T) {
	t.Parallel()
	app := New()
	dynamic := New()
	apiserver := New()

	apiroutes := apiserver.Group("/v1")
	apiroutes.Get("/home", func(c Ctx) error {
		return c.SendString("home")
	})

	dynamic.Use("/api", apiserver)
	app.Use("/", dynamic)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/v1/home", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, uint32(1), app.handlersCount)
}

// go test -run Test_App_Mount_Nested
func Test_App_Mount_Nested(t *testing.T) {
	t.Parallel()
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/one/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/one/two/nested", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/one/two/three/test", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	require.Equal(t, uint32(3), app.handlersCount)
	require.Equal(t, uint32(3), app.routesCount)
}

// go test -run Test_App_Mount_Express_Behavior
func Test_App_Mount_Express_Behavior(t *testing.T) {
	t.Parallel()
	createTestHandler := func(body string) func(c Ctx) error {
		return func(c Ctx) error {
			return c.SendString(body)
		}
	}
	testEndpoint := func(app *App, route, expectedBody string, expectedStatusCode int) {
		resp, err := app.Test(httptest.NewRequest(MethodGet, route, http.NoBody))
		require.NoError(t, err, "app.Test(req)")
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, expectedStatusCode, resp.StatusCode, "Status code")
		require.Equal(t, expectedBody, string(body), "Unexpected response body")
	}

	app := New()
	subApp := New()
	// app setup
	{
		subApp.Get("/hello", createTestHandler("subapp hello!"))
		subApp.Get("/world", createTestHandler("subapp world!")) // <- wins

		app.Get("/hello", createTestHandler("app hello!")) // <- wins
		app.Use("/", subApp)                               // <- subApp registration
		app.Get("/world", createTestHandler("app world!"))

		app.Get("/bar", createTestHandler("app bar!"))
		subApp.Get("/bar", createTestHandler("subapp bar!")) // <- wins

		subApp.Get("/foo", createTestHandler("subapp foo!")) // <- wins
		app.Get("/foo", createTestHandler("app foo!"))

		// 404 Handler
		app.Use(func(c Ctx) error {
			return c.SendStatus(StatusNotFound)
		})
	}
	// expectation check
	testEndpoint(app, "/world", "subapp world!", StatusOK)
	testEndpoint(app, "/hello", "app hello!", StatusOK)
	testEndpoint(app, "/bar", "subapp bar!", StatusOK)
	testEndpoint(app, "/foo", "subapp foo!", StatusOK)
	testEndpoint(app, "/unknown", ErrNotFound.Message, StatusNotFound)

	require.Equal(t, uint32(9), app.handlersCount)
	require.Equal(t, uint32(17), app.routesCount)
}

// go test -run Test_App_Mount_RoutePositions
func Test_App_Mount_RoutePositions(t *testing.T) {
	t.Parallel()
	testEndpoint := func(app *App, route, expectedBody string) {
		resp, err := app.Test(httptest.NewRequest(MethodGet, route, http.NoBody))
		require.NoError(t, err, "app.Test(req)")
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode, "Status code")
		require.Equal(t, expectedBody, string(body), "Unexpected response body")
	}

	app := New()
	subApp1 := New()
	subApp2 := New()
	// app setup
	{
		app.Use(func(c Ctx) error {
			// set initial value
			c.Locals("world", "world")
			return c.Next()
		})
		app.Use("/subApp1", subApp1)
		app.Use(func(c Ctx) error {
			return c.Next()
		})
		app.Get("/bar", func(c Ctx) error {
			return c.SendString("ok")
		})
		app.Use(func(c Ctx) error {
			// is overwritten in case the positioning is not correct
			c.Locals("world", "hello")
			return c.Next()
		})
		methods := subApp2.Group("/subApp2")
		methods.Get("/world", func(c Ctx) error {
			v, ok := c.Locals("world").(string)
			if !ok {
				panic("unexpected data type")
			}
			return c.SendString(v)
		})
		app.Use("", subApp2)
	}

	testEndpoint(app, "/subApp2/world", "hello")

	routeStackGET := app.Stack()[0]
	require.True(t, routeStackGET[0].use)
	require.Equal(t, "/", routeStackGET[0].path)

	require.True(t, routeStackGET[1].use)
	require.Equal(t, "/", routeStackGET[1].path)
	require.Less(t, routeStackGET[0].pos, routeStackGET[1].pos, "wrong position of route 0")

	require.False(t, routeStackGET[2].use)
	require.Equal(t, "/bar", routeStackGET[2].path)
	require.Less(t, routeStackGET[1].pos, routeStackGET[2].pos, "wrong position of route 1")

	require.True(t, routeStackGET[3].use)
	require.Equal(t, "/", routeStackGET[3].path)
	require.Less(t, routeStackGET[2].pos, routeStackGET[3].pos, "wrong position of route 2")

	require.False(t, routeStackGET[4].use)
	require.Equal(t, "/subapp2/world", routeStackGET[4].path)
	require.Less(t, routeStackGET[3].pos, routeStackGET[4].pos, "wrong position of route 3")

	require.Len(t, routeStackGET, 5)
}

// go test -run Test_App_MountPath
func Test_App_MountPath(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", http.NoBody))
	testErrorResponse(t, err, resp, "1: custom error")
}

func Test_App_ErrorHandler_GroupMountRootLevel(t *testing.T) {
	t.Parallel()
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", http.NoBody))
	testErrorResponse(t, err, resp, "1: custom error")
}

// go test -run Test_App_Group_Mount
func Test_App_Group_Mount(t *testing.T) {
	t.Parallel()
	micro := New()
	micro.Get("/doe", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	app := New()
	v1 := app.Group("/v1")
	v1.Use("/john", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, uint32(1), app.handlersCount)
}

func Test_App_UseParentErrorHandler(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ErrorHandler: func(ctx Ctx, err error) error {
			return ctx.Status(500).SendString("hi, i'm a custom error")
		},
	})

	fiber := New()
	fiber.Get("/", func(c Ctx) error {
		return errors.New("something happened")
	})

	app.Use("/api", fiber)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", http.NoBody))
	testErrorResponse(t, err, resp, "hi, i'm a custom error")
}

func Test_App_UseMountedErrorHandler(t *testing.T) {
	t.Parallel()
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", http.NoBody))
	testErrorResponse(t, err, resp, "hi, i'm a custom error")
}

func Test_App_UseMountedErrorHandlerRootLevel(t *testing.T) {
	t.Parallel()
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", http.NoBody))
	testErrorResponse(t, err, resp, "hi, i'm a custom error")
}

func Test_App_UseMountedErrorHandlerForBestPrefixMatch(t *testing.T) {
	t.Parallel()
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/sub", http.NoBody))
	require.NoError(t, err, "/api/sub req")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "iotuil.ReadAll()")
	require.Equal(t, "hi, i'm a custom sub fiber error", string(b), "Response body")

	resp2, err := app.Test(httptest.NewRequest(MethodGet, "/api/sub/third", http.NoBody))
	require.NoError(t, err, "/api/sub/third req")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	b, err = io.ReadAll(resp2.Body)
	require.NoError(t, err, "iotuil.ReadAll()")
	require.Equal(t, "hi, i'm a custom sub sub fiber error", string(b), "Third fiber Response body")
}

// go test -run Test_Mount_Route_Names
func Test_Mount_Route_Names(t *testing.T) {
	// create sub-app with 2 handlers:
	subApp1 := New()
	subApp1.Get("/users", func(c Ctx) error {
		url, err := c.GetRouteURL("add-user", Map{})
		require.NoError(t, err)
		require.Equal(t, "/app1/users", url, "handler: app1.add-user") // the prefix is /app1 because of the mount
		// if subApp1 is not mounted, expected url just /users
		return nil
	}).Name("get-users")
	subApp1.Post("/users", func(c Ctx) error {
		route := c.App().GetRoute("get-users")
		require.Equal(t, MethodGet, route.Method, "handler: app1.get-users method")
		require.Equal(t, "/app1/users", route.Path, "handler: app1.get-users path")
		return nil
	}).Name("add-user")

	// create sub-app with 2 handlers inside a group:
	subApp2 := New()
	app2Grp := subApp2.Group("/users").Name("users.")
	app2Grp.Get("", emptyHandler).Name("get")
	app2Grp.Post("", emptyHandler).Name("add")

	// put both sub-apps into root app
	rootApp := New()
	_ = rootApp.Use("/app1", subApp1)
	_ = rootApp.Use("/app2", subApp2)

	rootApp.startupProcess()

	// take route directly from sub-app
	route := subApp1.GetRoute("get-users")
	require.Equal(t, MethodGet, route.Method)
	require.Equal(t, "/users", route.Path)

	route = subApp1.GetRoute("add-user")
	require.Equal(t, MethodPost, route.Method)
	require.Equal(t, "/users", route.Path)

	// take route directly from sub-app with group
	route = subApp2.GetRoute("users.get")
	require.Equal(t, MethodGet, route.Method)
	require.Equal(t, "/users", route.Path)

	route = subApp2.GetRoute("users.add")
	require.Equal(t, MethodPost, route.Method)
	require.Equal(t, "/users", route.Path)

	// take route from root app (using names of sub-apps)
	route = rootApp.GetRoute("add-user")
	require.Equal(t, MethodPost, route.Method)
	require.Equal(t, "/app1/users", route.Path)

	route = rootApp.GetRoute("users.add")
	require.Equal(t, MethodPost, route.Method)
	require.Equal(t, "/app2/users", route.Path)

	// GetRouteURL inside handler
	req := httptest.NewRequest(MethodGet, "/app1/users", nil)
	resp, err := rootApp.Test(req)

	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// ctx.App().GetRoute() inside handler
	req = httptest.NewRequest(MethodPost, "/app1/users", nil)
	resp, err = rootApp.Test(req)

	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

// go test -run Test_Ctx_Render_Mount
func Test_Ctx_Render_Mount(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{}
	err := engine.Load()
	require.NoError(t, err)

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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/hello/a", http.NoBody))
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	require.NoError(t, err, "app.Test(req)")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>Hello a!</h1>", string(body))
}

// go test -run Test_Ctx_Render_Mount_ParentOrSubHasViews
func Test_Ctx_Render_Mount_ParentOrSubHasViews(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{}
	err := engine.Load()
	require.NoError(t, err)

	engine2 := &testTemplateEngine{path: "testdata2"}
	err = engine2.Load()
	require.NoError(t, err)

	engine3 := &testTemplateEngine{path: "testdata3"}
	err = engine3.Load()
	require.NoError(t, err)

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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/hello/world/a", http.NoBody))
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	require.NoError(t, err, "app.Test(req)")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>Hello a!</h1>", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test", http.NoBody))
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	require.NoError(t, err, "app.Test(req)")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>Hello, World!</h1>", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/hello/bruh/moment", http.NoBody))
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	require.NoError(t, err, "app.Test(req)")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>I'm Bruh</h1>", string(body))
}

func Test_Ctx_Render_MountGroup(t *testing.T) {
	t.Parallel()

	engine := &testTemplateEngine{}
	err := engine.Load()
	require.NoError(t, err)

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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>Hello doe!</h1>", string(body))
}
