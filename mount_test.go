// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_App_Mount_PreservesSubAppRegexHandler(t *testing.T) {
	t.Parallel()

	parent := New(Config{
		CaseSensitive: true,
		RegexHandler: func(pattern string) *matchOnlyRegexCompiler {
			return &matchOnlyRegexCompiler{re: regexp.MustCompile("(?i)" + pattern)}
		},
	})

	sub := New(Config{
		CaseSensitive: true,
		RegexHandler:  regexp.MustCompile,
	})
	sub.Get("/resource/:id<regex(ALLOW)>", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	parent.Use("/mounted", sub)

	resp, err := sub.Test(httptest.NewRequest(MethodGet, "/resource/ALLOW", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	resp, err = parent.Test(httptest.NewRequest(MethodGet, "/mounted/resource/ALLOW", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	resp, err = sub.Test(httptest.NewRequest(MethodGet, "/resource/allow", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)

	resp, err = parent.Test(httptest.NewRequest(MethodGet, "/mounted/resource/allow", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

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
	require.Equal(t, uint32(2), app.handlersCount)
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
	require.Equal(t, uint32(2), app.handlersCount)
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

	require.Equal(t, uint32(6), app.handlersCount)
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
	// expectation check
	testEndpoint(app, "/world", "subapp world!", StatusOK)
	testEndpoint(app, "/hello", "app hello!", StatusOK)
	testEndpoint(app, "/bar", "subapp bar!", StatusOK)
	testEndpoint(app, "/foo", "subapp foo!", StatusOK)
	testEndpoint(app, "/unknown", ErrNotFound.Message, StatusNotFound)

	require.Equal(t, uint32(17), app.handlersCount)
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
			// is overwritten when the positioning is not correct
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

	require.False(t, routeStackGET[2].use)
	require.Equal(t, "/bar", routeStackGET[2].path)

	require.True(t, routeStackGET[3].use)
	require.Equal(t, "/", routeStackGET[3].path)

	require.False(t, routeStackGET[4].use)
	require.Equal(t, "/subapp2/world", routeStackGET[4].path)

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
	require.Empty(t, app.MountPath())
}

func Test_App_ErrorHandler_GroupMount(t *testing.T) {
	t.Parallel()
	micro := New(Config{
		ErrorHandler: func(c Ctx, err error) error {
			require.Equal(t, "0: GET error", err.Error())
			return c.Status(500).SendString("1: custom error")
		},
	})
	micro.Get("/doe", func(_ Ctx) error {
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
	micro.Get("/john/doe", func(_ Ctx) error {
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
	require.Equal(t, uint32(2), app.handlersCount)
}

func Test_App_UseParentErrorHandler(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ErrorHandler: func(ctx Ctx, _ error) error {
			return ctx.Status(500).SendString("hi, i'm a custom error")
		},
	})

	fiber := New()
	fiber.Get("/", func(_ Ctx) error {
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
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(500).SendString("hi, i'm a custom error")
		},
	})
	fiber.Get("/", func(_ Ctx) error {
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
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(500).SendString("hi, i'm a custom error")
		},
	})
	fiber.Get("/api", func(_ Ctx) error {
		return errors.New("something happened")
	})

	app.Use("/", fiber)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", http.NoBody))
	testErrorResponse(t, err, resp, "hi, i'm a custom error")
}

func Test_App_UseMountedErrorHandlerForBestPrefixMatch(t *testing.T) {
	t.Parallel()
	app := New()

	tsf := func(c Ctx, _ error) error {
		return c.Status(200).SendString("hi, i'm a custom sub fiber error 2")
	}
	tripleSubFiber := New(Config{
		ErrorHandler: tsf,
	})
	tripleSubFiber.Get("/", func(_ Ctx) error {
		return errors.New("something happened")
	})

	sf := func(c Ctx, _ error) error {
		return c.Status(200).SendString("hi, i'm a custom sub fiber error")
	}
	subfiber := New(Config{
		ErrorHandler: sf,
	})
	subfiber.Get("/", func(_ Ctx) error {
		return errors.New("something happened")
	})
	subfiber.Use("/third", tripleSubFiber)

	f := func(c Ctx, _ error) error {
		return c.Status(200).SendString("hi, i'm a custom error")
	}
	fiber := New(Config{
		ErrorHandler: f,
	})
	fiber.Get("/", func(_ Ctx) error {
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
	require.Equal(t, 200, resp2.StatusCode, "Status code")

	b, err = io.ReadAll(resp2.Body)
	require.NoError(t, err, "iotuil.ReadAll()")
	require.Equal(t, "hi, i'm a custom sub fiber error 2", string(b), "Third fiber Response body")
}

// go test -run Test_Mount_Route_Names
func Test_Mount_Route_Names(t *testing.T) {
	t.Parallel()
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
	req := httptest.NewRequest(MethodGet, "/app1/users", http.NoBody)
	resp, err := rootApp.Test(req)

	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// ctx.App().GetRoute() inside handler
	req = httptest.NewRequest(MethodPost, "/app1/users", http.NoBody)
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

// Test_App_Mount_FewerRequestMethods verifies that a sub-app serving only some
// of the parent's request methods can be mounted: the parent must read the
// sub-app's routes by method, not by its own stack index.
func Test_App_Mount_FewerRequestMethods(t *testing.T) {
	t.Parallel()

	micro := New(Config{RequestMethods: []string{MethodGet}})
	micro.Get("/doe", func(c Ctx) error {
		return c.SendString("doe")
	})

	app := New()
	app.Use("/john", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "doe", string(body))
}

// Test_App_Mount_FewerRequestMethodsNested verifies the same for a sub-app
// reached through another mount.
func Test_App_Mount_FewerRequestMethodsNested(t *testing.T) {
	t.Parallel()

	micro := New(Config{RequestMethods: []string{MethodGet}})
	micro.Get("/doe", func(c Ctx) error {
		return c.SendString("doe")
	})

	sub := New()
	sub.Use("/v1", micro)

	app := New()
	app.Use("/john", sub)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john/v1/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "doe", string(body))
}

// Test_App_Mount_ReorderedRequestMethods verifies that a sub-app which lists
// the same request methods in a different order keeps serving each route under
// the method it was registered with.
func Test_App_Mount_ReorderedRequestMethods(t *testing.T) {
	t.Parallel()

	// Swap GET and POST, so the two apps' stack indexes disagree.
	methods := append([]string{}, DefaultMethods...)
	for i, method := range methods {
		switch method {
		case MethodGet:
			methods[i] = MethodPost
		case MethodPost:
			methods[i] = MethodGet
		}
	}

	micro := New(Config{RequestMethods: methods})
	micro.Get("/doe", func(c Ctx) error {
		return c.SendString("get doe")
	})
	micro.Post("/doe", func(c Ctx) error {
		return c.SendString("post doe")
	})

	app := New()
	app.Use("/john", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "get doe", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodPost, "/john/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "post doe", string(body))
}

// onlyFooConstraint and onlyBarConstraint are custom constraints used to check
// that a mounted app's constraints survive the mount's route expansion.
type onlyFooConstraint struct{}

func (*onlyFooConstraint) Name() string { return "onlyfoo" }

func (*onlyFooConstraint) Execute(param string, _ ...string) bool { return param == "foo" }

type onlyBarConstraint struct{}

func (*onlyBarConstraint) Name() string { return "onlybar" }

func (*onlyBarConstraint) Execute(param string, _ ...string) bool { return param == "bar" }

// sameNameFooConstraint and sameNameBarConstraint share a name and accept
// different values, as two apps mounted side by side are free to do.
type sameNameFooConstraint struct{}

func (*sameNameFooConstraint) Name() string { return "same" }

func (*sameNameFooConstraint) Execute(param string, _ ...string) bool { return param == "foo" }

type sameNameBarConstraint struct{}

func (*sameNameBarConstraint) Name() string { return "same" }

func (*sameNameBarConstraint) Execute(param string, _ ...string) bool { return param == "bar" }

// Test_App_Mount_SiblingConstraintNames verifies that a constraint binds only
// the routes of the app that registered it: two apps mounted side by side can
// name one differently, and the app above them must not hold either to the
// other's.
func Test_App_Mount_SiblingConstraintNames(t *testing.T) {
	t.Parallel()

	// A fresh tree per case: mounting one app on two parents would have the
	// first expand it before the second cloned it.
	build := func() *App {
		handler := func(c Ctx) error { return c.SendString("ok") }

		foo := New()
		foo.RegisterCustomConstraint(&sameNameFooConstraint{})
		foo.Get("/:value<same>", handler)

		bar := New()
		bar.RegisterCustomConstraint(&sameNameBarConstraint{})
		bar.Get("/:value<same>", handler)

		subApp := New()
		subApp.Use("/foo", foo)
		subApp.Use("/bar", bar)

		return subApp
	}

	plain := New()
	plain.Use("/sub", build())

	domain := New()
	domain.Domain("api.example.com").Use("/sub", build())

	want := map[string]int{
		"/sub/foo/foo": StatusOK,
		"/sub/foo/bar": StatusNotFound,
		"/sub/bar/bar": StatusOK,
		"/sub/bar/foo": StatusNotFound,
	}

	for path, status := range want {
		resp, err := plain.Test(httptest.NewRequest(MethodGet, path, http.NoBody))
		require.NoError(t, err)
		require.Equal(t, status, resp.StatusCode, "plain "+path)

		req := httptest.NewRequest(MethodGet, path, http.NoBody)
		req.Host = "api.example.com"
		resp, err = domain.Test(req)
		require.NoError(t, err)
		require.Equal(t, status, resp.StatusCode, "domain "+path)
	}
}

// Test_App_Mount_PreservesNestedCustomConstraint verifies that a custom
// constraint registered on an app reached through two mounts still validates.
// Expanding a mount re-parses the routes against the app above, so the
// constraint has to travel with them or it silently stops rejecting anything.
func Test_App_Mount_PreservesNestedCustomConstraint(t *testing.T) {
	t.Parallel()

	micro := New()
	micro.RegisterCustomConstraint(&onlyFooConstraint{})
	micro.Get("/doe/:name<onlyfoo>", func(c Ctx) error {
		return c.SendString(c.Params("name"))
	})

	sub := New()
	sub.Use("/v1", micro)

	app := New()
	app.Use("/john", sub)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john/v1/doe/foo", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/john/v1/doe/bar", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")
}

// Test_App_Mount_CarriesDomainMount verifies that mounting an app which has a
// domain mount of its own carries that mount's config along, host check
// included.
func Test_App_Mount_CarriesDomainMount(t *testing.T) {
	t.Parallel()

	micro := New(Config{
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(596).SendString("micro error")
		},
	})
	micro.Get("/doe", func(_ Ctx) error {
		return errors.New("boom")
	})

	sub := New()
	sub.Domain("api.example.com").Use("/v1", micro)

	app := New()
	app.Use("/john", sub)

	req := httptest.NewRequest(MethodGet, "/john/v1/doe", http.NoBody)
	req.Host = "api.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 596, resp.StatusCode, "Status code")

	// The route does not match on another host, and the sub-app's error
	// handler must not answer for it either.
	req = httptest.NewRequest(MethodGet, "/john/v1/doe", http.NoBody)
	req.Host = "www.example.com"
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")
}

// Test_App_Mount_AutoHeadKeepsMiddleware verifies that a mounted app which does
// not serve HEAD gets no synthesized HEAD routes once its routes are expanded
// into the parent — they would run without the middleware it registered.
func Test_App_Mount_AutoHeadKeepsMiddleware(t *testing.T) {
	t.Parallel()

	micro := New(Config{RequestMethods: []string{MethodGet, MethodPost}})
	micro.Use(func(c Ctx) error {
		return c.SendStatus(StatusUnauthorized)
	})
	micro.Get("/doe", func(c Ctx) error {
		c.Set("X-Secret", "leaked")
		return c.SendString("doe")
	})

	app := New()
	app.Use("/john", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusUnauthorized, resp.StatusCode, "Status code")

	// Twice: app.Test runs the startup process on every call, and the second
	// pass sees the mount already expanded into the parent's stack.
	for range 2 {
		resp, err = app.Test(httptest.NewRequest(MethodHead, "/john/doe", http.NoBody))
		require.NoError(t, err, "app.Test(req)")
		require.Equal(t, StatusMethodNotAllowed, resp.StatusCode, "Status code")
		require.Empty(t, resp.Header.Get("X-Secret"))
	}
}

// Test_App_Mount_RootMountKeepsParentAutoHead verifies that a mounted app which
// does not serve HEAD only withholds automatic HEAD routes from its own routes.
// A mount at "/" covers every path, and the app above keeps its own.
func Test_App_Mount_RootMountKeepsParentAutoHead(t *testing.T) {
	t.Parallel()

	micro := New(Config{RequestMethods: []string{MethodGet, MethodPost}})
	micro.Get("/doe", func(c Ctx) error {
		return c.SendString("doe")
	})

	app := New()
	app.Get("/own", func(c Ctx) error {
		return c.SendString("own")
	})
	app.Use("/", micro)

	resp, err := app.Test(httptest.NewRequest(MethodHead, "/own", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "the app's own route keeps its HEAD route")

	resp, err = app.Test(httptest.NewRequest(MethodHead, "/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusMethodNotAllowed, resp.StatusCode, "the mounted app serves no HEAD")
}

// Test_App_Mount_ParametricPrefix verifies that a mount prefix carrying a
// parameter is matched as a pattern. The prefix rewrites every route of the
// mounted app, so the parameters it introduces have to be rebuilt onto them.
func Test_App_Mount_ParametricPrefix(t *testing.T) {
	t.Parallel()

	micro := New()
	micro.Get("/doe", func(c Ctx) error {
		return c.SendString("version=" + c.Params("Version"))
	})

	app := New()
	app.Use("/v1/:Version", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/42/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "version=42", string(body))
}

// Test_App_Mount_ParametricPrefixNested verifies the same when the prefix is
// applied a second time, by an outer mount.
func Test_App_Mount_ParametricPrefixNested(t *testing.T) {
	t.Parallel()

	micro := New()
	micro.Get("/doe", func(c Ctx) error {
		return c.SendString("version=" + c.Params("version"))
	})

	sub := New()
	sub.Use("/v1/:version", micro)

	app := New()
	app.Use("/john", sub)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john/v1/42/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "version=42", string(body))
}

// Test_App_Mount_CaseInsensitivePath verifies that a mount path is matched by
// the app's own routing rules when its config is resolved: a case-insensitive
// app serves "/api/doe" from a mount registered as "/API", so the mounted app's
// error handler has to answer for it too.
func Test_App_Mount_CaseInsensitivePath(t *testing.T) {
	t.Parallel()

	micro := New(Config{
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(599).SendString("micro error")
		},
	})
	micro.Get("/doe", func(_ Ctx) error {
		return errors.New("boom")
	})

	app := New()
	app.Use("/API", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 599, resp.StatusCode, "Status code")

	// A case-sensitive app keeps the two apart.
	strict := New(Config{CaseSensitive: true})
	strict.Use("/API", micro)

	resp, err = strict.Test(httptest.NewRequest(MethodGet, "/API/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 599, resp.StatusCode, "Status code")

	resp, err = strict.Test(httptest.NewRequest(MethodGet, "/api/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")
}

// Test_App_Mount_ViewsPathOnly verifies that a mounted app's views are chosen
// by the request path, not by its mount path appearing anywhere in the URL.
func Test_App_Mount_ViewsPathOnly(t *testing.T) {
	t.Parallel()

	subEngine := &testTemplateEngine{path: "testdata2"}
	require.NoError(t, subEngine.Load())

	parentEngine := &testTemplateEngine{}
	require.NoError(t, parentEngine.Load())

	micro := New(Config{Views: subEngine})
	micro.Get("/view", func(c Ctx) error {
		return c.Render("bruh.tmpl", Map{})
	})

	app := New(Config{Views: parentEngine})
	app.Use("/john", micro)
	app.Get("/elsewhere", func(c Ctx) error {
		return c.Render("index.tmpl", Map{"Title": "parent"})
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/elsewhere?next=/john/view", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "<h1>parent</h1>", string(body))
}

// Test_App_Mount_ParametricPrefixConstraint verifies that a constraint named in
// a mount prefix is enforced. The prefix belongs to the app doing the mounting,
// so its constraints have to reach the re-parse of the mounted routes.
func Test_App_Mount_ParametricPrefixConstraint(t *testing.T) {
	t.Parallel()

	micro := New()
	micro.Get("/doe", func(c Ctx) error {
		return c.SendString("name=" + c.Params("name"))
	})

	app := New()
	app.RegisterCustomConstraint(&onlyFooConstraint{})
	app.Use("/:name<onlyfoo>", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/foo/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/bar/doe", http.NoBody))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "the prefix constraint still rejects")
}

func Test_App_Mount_StrictRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path   string
		strict int
		loose  int
	}{
		{path: "/api/sub", strict: StatusOK, loose: StatusOK},
		{path: "/api/sub/", strict: StatusNotFound, loose: StatusOK},
		{path: "/api/p/7", strict: StatusOK, loose: StatusOK},
		{path: "/api/p/7/", strict: StatusNotFound, loose: StatusOK},
	}

	for _, strict := range []bool{false, true} {
		app := New(Config{StrictRouting: strict})
		sub := New()
		sub.Get("/sub", func(c Ctx) error { return c.SendString("plain") })
		sub.Get("/p/:id", func(c Ctx) error { return c.SendString("param") })
		app.Use("/api", sub)

		for _, tc := range tests {
			want := tc.loose
			if strict {
				want = tc.strict
			}
			resp, err := app.Test(httptest.NewRequest(MethodGet, tc.path, http.NoBody))
			require.NoError(t, err)
			require.Equal(t, want, resp.StatusCode, "strict=%v path=%q", strict, tc.path)
		}
	}
}
