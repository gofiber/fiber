package fiber

import (
	"errors"
	"io"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// newRadixApp returns a Fiber app with radix routing enabled.
func newRadixApp() *App {
	return New(Config{UseRadix: true})
}

// newCustomRadixApp returns a Fiber app with a custom context and radix routing enabled.
func newCustomRadixApp() *App {
	return NewWithCustomCtx(func(app *App) CustomCtx {
		return &customCtx{DefaultCtx: *NewDefaultCtx(app)}
	}, Config{UseRadix: true})
}

func Test_Router_Radix_Wildcard(t *testing.T) {
	t.Parallel()
	app := newRadixApp()
	app.Get("/*", func(c Ctx) error {
		return c.SendString(c.Params("*"))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/foo/bar", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "foo/bar", app.getString(body))
}

func Test_Router_Radix_Param(t *testing.T) {
	t.Parallel()
	app := newRadixApp()
	app.Get("/user/:id", func(c Ctx) error {
		return c.SendString(c.Params("id"))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/user/42", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "42", app.getString(body))
}

func Test_Router_Radix_Group(t *testing.T) {
	t.Parallel()
	app := newRadixApp()
	g := app.Group("/v1")
	g.Get("/test", func(c Ctx) error { return c.SendStatus(StatusOK) })

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/test", nil))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
}

func Test_Router_Radix_RebuildTree(t *testing.T) {
	t.Parallel()
	app := newRadixApp()
	app.Get("/foo", func(c Ctx) error { return c.SendStatus(StatusOK) })

	// trigger initial tree build
	_, err := app.Test(httptest.NewRequest(MethodGet, "/foo", nil))
	require.NoError(t, err)

	app.Get("/bar", func(c Ctx) error { return c.SendStatus(StatusCreated) })
	app.RebuildTree()

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/bar", nil))
	require.NoError(t, err)
	require.Equal(t, StatusCreated, resp.StatusCode)
}

func Test_Router_Radix_OptionalPlusRegexEscaped(t *testing.T) {
	t.Parallel()
	app := newRadixApp()
	app.Get("/user/:name?", func(c Ctx) error {
		return c.SendString(c.Params("name"))
	})
	app.Get("/user/+", func(c Ctx) error {
		return c.SendString(c.Params("+"))
	})
	app.Get(`/:date<regex(\d{4}-\d{2}-\d{2})>`, func(c Ctx) error {
		return c.SendString(c.Params("date"))
	})
	app.Get(`/v1/some/resource/name\:customVerb`, func(c Ctx) error {
		return c.SendString("ok")
	})
	app.Get("/v1/*/shop/*", func(c Ctx) error {
		return c.SendString(c.Params("*1") + "," + c.Params("*2"))
	})

	// optional parameter
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/user", nil))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "", app.getString(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/user/john", nil))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "john", app.getString(body))

	// plus parameter
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/user/1/2", nil))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1/2", app.getString(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/user/", nil))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "", app.getString(body))

	// regex constraint
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/2022-08-27", nil))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "2022-08-27", app.getString(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/125", nil))
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)

	// escaped colon
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/v1/some/resource/name:customVerb", nil))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "ok", app.getString(body))

	// multi wildcard
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/v1/brand/4/shop/blue/xs", nil))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "brand/4,blue/xs", app.getString(body))
}
func Test_Route_Radix_Handler_Order(t *testing.T) {
	t.Parallel()

	app := New(Config{UseRadix: true})

	var order []int

	handler1 := func(c Ctx) error {
		order = append(order, 1)
		return c.Next()
	}
	handler2 := func(c Ctx) error {
		order = append(order, 2)
		return c.Next()
	}
	handler3 := func(c Ctx) error {
		order = append(order, 3)
		return c.Next()
	}

	app.Get("/test", handler1, handler2, handler3, func(c Ctx) error {
		order = append(order, 4)
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	expectedOrder := []int{1, 2, 3, 4}
	require.Equal(t, expectedOrder, order, "Handler order")
}

func Test_Route_Radix_Match_SameLength(t *testing.T) {
	t.Parallel()

	app := New(Config{UseRadix: true})

	app.Get("/:param", func(c Ctx) error {
		return c.SendString(c.Params("param"))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/:param", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, ":param", app.getString(body))

	// with param
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "test", app.getString(body))
}

func Test_Route_Radix_Match_Star(t *testing.T) {
	t.Parallel()

	app := New(Config{UseRadix: true})

	app.Get("/*", func(c Ctx) error {
		return c.SendString(c.Params("*"))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/*", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "*", app.getString(body))

	// with param
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "test", app.getString(body))

	// without parameter
	route := Route{
		star:        true,
		path:        "/*",
		routeParser: routeParser{},
	}
	params := [maxParams]string{}
	match := route.match("", "", &params)
	require.True(t, match)
	require.Equal(t, [maxParams]string{}, params)

	// with parameter
	match = route.match("/favicon.ico", "/favicon.ico", &params)
	require.True(t, match)
	require.Equal(t, [maxParams]string{"favicon.ico"}, params)

	// without parameter again
	match = route.match("", "", &params)
	require.True(t, match)
	require.Equal(t, [maxParams]string{}, params)
}

func Test_Route_Radix_Match_Root(t *testing.T) {
	t.Parallel()

	app := New(Config{UseRadix: true})

	app.Get("/", func(c Ctx) error {
		return c.SendString("root")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "root", app.getString(body))
}

func Test_Route_Radix_Match_Parser(t *testing.T) {
	t.Parallel()

	app := New(Config{UseRadix: true})

	app.Get("/foo/:ParamName", func(c Ctx) error {
		return c.SendString(c.Params("ParamName"))
	})
	app.Get("/Foobar/*", func(c Ctx) error {
		return c.SendString(c.Params("*"))
	})
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/foo/bar", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "bar", app.getString(body))

	// with star
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/Foobar/test", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "test", app.getString(body))
}

func Test_Route_Radix_Match_Middleware(t *testing.T) {
	t.Parallel()

	app := New(Config{UseRadix: true})

	app.Use("/foo/*", func(c Ctx) error {
		return c.SendString(c.Params("*"))
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/foo/*", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "*", app.getString(body))

	// with param
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo/bar/fasel", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "bar/fasel", app.getString(body))
}

func Test_Route_Radix_Match_UnescapedPath(t *testing.T) {
	t.Parallel()

	app := New(Config{UseRadix: true, UnescapePath: true})

	app.Use("/créer", func(c Ctx) error {
		return c.SendString("test")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/cr%C3%A9er", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "test", app.getString(body))
	// without special chars
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/créer", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// check deactivated behavior
	app.config.UnescapePath = false
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/cr%C3%A9er", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")
}

func Test_Route_Radix_Match_WithEscapeChar(t *testing.T) {
	t.Parallel()

	app := New(Config{UseRadix: true})
	// static route and escaped part
	app.Get("/v1/some/resource/name\\:customVerb", func(c Ctx) error {
		return c.SendString("static")
	})
	// group route
	group := app.Group("/v2/\\:firstVerb")
	group.Get("/\\:customVerb", func(c Ctx) error {
		return c.SendString("group")
	})
	// route with resource param and escaped part
	app.Get("/v3/:resource/name\\:customVerb", func(c Ctx) error {
		return c.SendString(c.Params("resource"))
	})

	// check static route
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/some/resource/name:customVerb", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "static", app.getString(body))

	// check group route
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/v2/:firstVerb/:customVerb", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "group", app.getString(body))

	// check param route
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/v3/awesome/name:customVerb", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "awesome", app.getString(body))
}

func Test_Route_Radix_Match_Middleware_HasPrefix(t *testing.T) {
	t.Parallel()

	app := New(Config{UseRadix: true})

	app.Use("/foo", func(c Ctx) error {
		return c.SendString("middleware")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/foo/bar", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "middleware", app.getString(body))
}

func Test_Route_Radix_Match_Middleware_Root(t *testing.T) {
	t.Parallel()

	app := New(Config{UseRadix: true})

	app.Use("/", func(c Ctx) error {
		return c.SendString("middleware")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/everything", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "middleware", app.getString(body))
}

func Test_Router_Radix_Register_Missing_Handler(t *testing.T) {
	t.Parallel()

	app := New(Config{UseRadix: true})

	t.Run("No Handler", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "missing handler/middleware in route: /doe\n", func() {
			app.register([]string{"USE"}, "/doe", nil)
		})
	})

	t.Run("Nil Handler", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "nil handler in route: /doe\n", func() {
			app.register([]string{"USE"}, "/doe", nil, nil)
		})
	})
}

func Test_Ensure_Radix_Router_Interface_Implementation(t *testing.T) {
	t.Parallel()

	var app any = (*App)(nil)
	_, ok := app.(Router)
	require.True(t, ok)

	var group any = (*Group)(nil)
	_, ok = group.(Router)
	require.True(t, ok)
}

func Test_Router_Radix_Handler_Catch_Error(t *testing.T) {
	t.Parallel()

	app := New(Config{UseRadix: true})
	app.config.ErrorHandler = func(_ Ctx, _ error) error {
		return errors.New("fake error")
	}

	app.Get("/", func(_ Ctx) error {
		return ErrForbidden
	})

	c := &fasthttp.RequestCtx{}

	app.Handler()(c)

	require.Equal(t, StatusInternalServerError, c.Response.Header.StatusCode())
}

func Test_Router_Radix_NotFound(t *testing.T) {
	t.Parallel()
	app := New(Config{UseRadix: true})
	app.Use(func(c Ctx) error {
		return c.Next()
	})
	appHandler := app.Handler()
	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/this/route/does/not/exist")

	appHandler(c)

	require.Equal(t, 404, c.Response.StatusCode())
	require.Equal(t, "Cannot DELETE /this/route/does/not/exist", string(c.Response.Body()))
}

func Test_Router_Radix_NotFound_HTML_Inject(t *testing.T) {
	t.Parallel()
	app := New(Config{UseRadix: true})
	app.Use(func(c Ctx) error {
		return c.Next()
	})
	appHandler := app.Handler()
	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/does/not/exist<script>alert('foo');</script>")

	appHandler(c)

	require.Equal(t, 404, c.Response.StatusCode())
	require.Equal(t, "Cannot DELETE /does/not/exist&lt;script&gt;alert(&#39;foo&#39;);&lt;/script&gt;", string(c.Response.Body()))
}

func Test_App_Radix_Rebuild_Tree(t *testing.T) {
	t.Parallel()
	app := New(Config{UseRadix: true})

	registerTreeManipulationRoutes(app)

	verifyRequest(t, app, "/dynamically-defined", StatusNotFound)
	verifyRequest(t, app, "/test", StatusOK)
	verifyRequest(t, app, "/dynamically-defined", StatusOK)
}

func Test_App_Radix_Remove_Route_A_B_Feature_Testing(t *testing.T) {
	t.Parallel()
	app := New(Config{UseRadix: true})

	app.Get("/api/feature-a", func(c Ctx) error {
		app.RemoveRoute("/api/feature", MethodGet)
		app.RebuildTree()
		// Redefine route
		app.Get("/api/feature", func(c Ctx) error {
			return c.SendString("Testing feature-a")
		})

		app.RebuildTree()
		return c.SendStatus(StatusOK)
	})
	app.Get("/api/feature-b", func(c Ctx) error {
		app.RemoveRoute("/api/feature", MethodGet)
		app.RebuildTree()
		// Redefine route
		app.Get("/api/feature", func(c Ctx) error {
			return c.SendString("Testing feature-b")
		})

		app.RebuildTree()
		return c.SendStatus(StatusOK)
	})

	verifyRequest(t, app, "/api/feature-a", StatusOK)

	resp := verifyRequest(t, app, "/api/feature", StatusOK)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")

	require.Equal(t, "Testing feature-a", string(body), "Response Message")

	verifyRequest(t, app, "/api/feature-b", StatusOK)

	resp = verifyRequest(t, app, "/api/feature", StatusOK)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "Testing feature-b", string(body), "Response Message")
}

func Test_App_Radix_Remove_Route_By_Name(t *testing.T) {
	t.Parallel()
	app := New(Config{UseRadix: true})

	app.Get("/api/test", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	}).Name("test")

	app.RemoveRouteByName("test", MethodGet)
	app.RebuildTree()

	verifyRequest(t, app, "/api/test", StatusNotFound)
	verifyThereAreNoRoutes(t, app)
}

func Test_App_Radix_Remove_Route_By_Name_Non_Existing_Route(t *testing.T) {
	t.Parallel()
	app := New(Config{UseRadix: true})

	app.RemoveRouteByName("test", MethodGet)
	app.RebuildTree()

	verifyThereAreNoRoutes(t, app)
}

func Test_App_Radix_Remove_Route_Nested(t *testing.T) {
	t.Parallel()
	app := New(Config{UseRadix: true})

	api := app.Group("/api")

	v1 := api.Group("/v1")
	v1.Get("/test", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	verifyRequest(t, app, "/api/v1/test", StatusOK)
	app.RemoveRoute("/api/v1/test", MethodGet)

	verifyThereAreNoRoutes(t, app)
}

func Test_App_Radix_Remove_Route_Parameterized(t *testing.T) {
	t.Parallel()
	app := New(Config{UseRadix: true})

	app.Get("/test/:id", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})
	verifyRequest(t, app, "/test/:id", StatusOK)
	app.RemoveRoute("/test/:id", MethodGet)

	verifyThereAreNoRoutes(t, app)
}

func Test_App_Radix_Remove_Route(t *testing.T) {
	t.Parallel()
	app := New(Config{UseRadix: true})

	app.Get("/test", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	app.RemoveRoute("/test", MethodGet)
	app.RebuildTree()

	verifyRequest(t, app, "/test", StatusNotFound)
}

func Test_App_Radix_Remove_Route_Non_Existing_Route(t *testing.T) {
	t.Parallel()
	app := New(Config{UseRadix: true})

	app.RemoveRoute("/test", MethodGet, MethodHead)
	app.RebuildTree()

	verifyThereAreNoRoutes(t, app)
}

func Test_App_Radix_Remove_Route_Concurrent(t *testing.T) {
	t.Parallel()
	app := New(Config{UseRadix: true})

	// Add test route
	app.Get("/test", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	// Concurrently remove and add routes
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app.RemoveRoute("/test", MethodGet)
			app.Get("/test", func(c Ctx) error {
				return c.SendStatus(StatusOK)
			})
		}()
	}
	wg.Wait()

	// Verify final state
	app.RebuildTree()
	verifyRequest(t, app, "/test", StatusOK)
}

func Test_Route_Radix_Registration_Prevent_Duplicate_With_Middleware(t *testing.T) {
	t.Parallel()
	app := New(Config{UseRadix: true})

	middleware := func(c Ctx) error {
		return c.Next()
	}

	registerTreeManipulationRoutes(app, middleware)
	registerTreeManipulationRoutes(app)

	verifyRequest(t, app, "/dynamically-defined", StatusNotFound)
	require.Equal(t, uint32(3), app.handlersCount)

	verifyRequest(t, app, "/test", StatusOK)
	require.Equal(t, uint32(4), app.handlersCount)

	verifyRequest(t, app, "/dynamically-defined", StatusOK)
	require.Equal(t, uint32(4), app.handlersCount)

	verifyRequest(t, app, "/test", StatusOK)
	require.Equal(t, uint32(5), app.handlersCount)

	verifyRequest(t, app, "/dynamically-defined", StatusOK)
	require.Equal(t, uint32(5), app.handlersCount)
}

func Benchmark_App_MethodNotAllowed_Radix(b *testing.B) {
	app := New(Config{UseRadix: true})
	h := func(c Ctx) error {
		return c.SendString("Hello World!")
	}
	app.All("/this/is/a/", h)
	app.Get("/this/is/a/dummy/route/oke", h)
	appHandler := app.Handler()
	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/this/is/a/dummy/route/oke")

	for b.Loop() {
		appHandler(c)
	}
	require.Equal(b, 405, c.Response.StatusCode())
	require.Equal(b, MethodGet, string(c.Response.Header.Peek("Allow")))
	require.Equal(b, utils.StatusMessage(StatusMethodNotAllowed), string(c.Response.Body()))
}

func Benchmark_Router_NotFound_Radix(b *testing.B) {
	app := New(Config{UseRadix: true})
	app.Use(func(c Ctx) error {
		return c.Next()
	})
	registerDummyRoutes(app)
	appHandler := app.Handler()
	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/this/route/does/not/exist")

	for b.Loop() {
		appHandler(c)
	}
	require.Equal(b, 404, c.Response.StatusCode())
	require.Equal(b, "Cannot DELETE /this/route/does/not/exist", string(c.Response.Body()))
}

func Benchmark_Router_Handler_Radix(b *testing.B) {
	app := New(Config{UseRadix: true})
	registerDummyRoutes(app)
	appHandler := app.Handler()

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	for b.Loop() {
		appHandler(c)
	}
}

func Benchmark_Router_Handler_Strict_Case_Radix(b *testing.B) {
	app := New(Config{UseRadix: true,
		StrictRouting: true,
		CaseSensitive: true,
	})
	registerDummyRoutes(app)
	appHandler := app.Handler()

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	for b.Loop() {
		appHandler(c)
	}
}

func Benchmark_Router_Chain_Radix(b *testing.B) {
	app := New(Config{UseRadix: true})
	handler := func(c Ctx) error {
		return c.Next()
	}
	app.Get("/", handler, handler, handler, handler, handler, handler)

	appHandler := app.Handler()

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod(MethodGet)
	c.URI().SetPath("/")
	for b.Loop() {
		appHandler(c)
	}
}

func Benchmark_Router_WithCompression_Radix(b *testing.B) {
	app := New(Config{UseRadix: true})
	handler := func(c Ctx) error {
		return c.Next()
	}
	app.Get("/", handler)
	app.Get("/", handler)
	app.Get("/", handler)
	app.Get("/", handler)
	app.Get("/", handler)
	app.Get("/", handler)

	appHandler := app.Handler()
	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod(MethodGet)
	c.URI().SetPath("/")
	for b.Loop() {
		appHandler(c)
	}
}

func Benchmark_Startup_Process_Radix(b *testing.B) {
	for b.Loop() {
		app := New(Config{UseRadix: true})
		registerDummyRoutes(app)
		app.startupProcess()
	}
}

func Benchmark_Router_Next_Radix(b *testing.B) {
	app := New(Config{UseRadix: true})
	registerDummyRoutes(app)
	app.startupProcess()

	request := &fasthttp.RequestCtx{}

	request.Request.Header.SetMethod("DELETE")
	request.URI().SetPath("/user/keys/1337")
	var res bool
	var err error

	c := app.AcquireCtx(request).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	for b.Loop() {
		c.indexRoute = -1
		res, err = app.next(c)
	}
	require.NoError(b, err)
	require.True(b, res)
	require.Equal(b, 0, c.indexRoute)
}

func Benchmark_Router_Next_Default_Radix(b *testing.B) {
	app := New(Config{UseRadix: true})
	app.Get("/", func(_ Ctx) error {
		return nil
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	fctx.Request.SetRequestURI("/")

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}
}

func Benchmark_Router_Next_Default_Parallel_Radix(b *testing.B) {
	app := New(Config{UseRadix: true})
	app.Get("/", func(_ Ctx) error {
		return nil
	})

	h := app.Handler()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.SetMethod(MethodGet)
		fctx.Request.SetRequestURI("/")

		for pb.Next() {
			h(fctx)
		}
	})
}

func Benchmark_Router_Next_Default_Immutable_Radix(b *testing.B) {
	app := New(Config{UseRadix: true, Immutable: true})
	app.Get("/", func(_ Ctx) error {
		return nil
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	fctx.Request.SetRequestURI("/")

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}
}

func Benchmark_Router_Next_Default_Parallel_Immutable_Radix(b *testing.B) {
	app := New(Config{UseRadix: true, Immutable: true})
	app.Get("/", func(_ Ctx) error {
		return nil
	})

	h := app.Handler()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.SetMethod(MethodGet)
		fctx.Request.SetRequestURI("/")

		for pb.Next() {
			h(fctx)
		}
	})
}

func Benchmark_Route_Match_Radix(b *testing.B) {
	var match bool
	var params [maxParams]string

	parsed := parseRoute("/user/keys/:id")
	route := &Route{
		use:         false,
		root:        false,
		star:        false,
		routeParser: parsed,
		Params:      parsed.params,
		path:        "/user/keys/:id",

		Path:   "/user/keys/:id",
		Method: "DELETE",
	}
	route.Handlers = append(route.Handlers, func(_ Ctx) error {
		return nil
	})
	for b.Loop() {
		match = route.match("/user/keys/1337", "/user/keys/1337", &params)
	}

	require.True(b, match)
	require.Equal(b, []string{"1337"}, params[0:len(parsed.params)])
}

func Benchmark_Route_Match_Star_Radix(b *testing.B) {
	var match bool
	var params [maxParams]string

	parsed := parseRoute("/*")
	route := &Route{
		use:         false,
		root:        false,
		star:        true,
		routeParser: parsed,
		Params:      parsed.params,
		path:        "/user/keys/bla",

		Path:   "/user/keys/bla",
		Method: "DELETE",
	}
	route.Handlers = append(route.Handlers, func(_ Ctx) error {
		return nil
	})

	for b.Loop() {
		match = route.match("/user/keys/bla", "/user/keys/bla", &params)
	}

	require.True(b, match)
	require.Equal(b, []string{"user/keys/bla"}, params[0:len(parsed.params)])
}

func Benchmark_Route_Match_Root_Radix(b *testing.B) {
	var match bool
	var params [maxParams]string

	parsed := parseRoute("/")
	route := &Route{
		use:         false,
		root:        true,
		star:        false,
		path:        "/",
		routeParser: parsed,
		Params:      parsed.params,

		Path:   "/",
		Method: "DELETE",
	}
	route.Handlers = append(route.Handlers, func(_ Ctx) error {
		return nil
	})

	for b.Loop() {
		match = route.match("/", "/", &params)
	}

	require.True(b, match)
	require.Equal(b, []string{}, params[0:len(parsed.params)])
}

func Benchmark_Router_Handler_CaseSensitive_Radix(b *testing.B) {
	app := New(Config{UseRadix: true})
	app.config.CaseSensitive = true
	registerDummyRoutes(app)
	appHandler := app.Handler()

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	for b.Loop() {
		appHandler(c)
	}
}

func Benchmark_Router_Handler_Unescape_Radix(b *testing.B) {
	app := New(Config{UseRadix: true})
	app.config.UnescapePath = true
	registerDummyRoutes(app)
	app.Delete("/créer", func(_ Ctx) error {
		return nil
	})

	appHandler := app.Handler()

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod(MethodDelete)
	c.URI().SetPath("/cr%C3%A9er")

	for b.Loop() {
		c.URI().SetPath("/cr%C3%A9er")
		appHandler(c)
	}
}

func Benchmark_Router_Handler_StrictRouting_Radix(b *testing.B) {
	app := New(Config{UseRadix: true})
	app.config.CaseSensitive = true
	registerDummyRoutes(app)
	appHandler := app.Handler()

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	for b.Loop() {
		appHandler(c)
	}
}

func Benchmark_Router_Github_API_Radix(b *testing.B) {
	app := New(Config{UseRadix: true})
	registerDummyRoutes(app)
	app.startupProcess()

	c := &fasthttp.RequestCtx{}
	var match bool
	var err error

	b.ResetTimer()
	for i := range routesFixture.TestRoutes {
		b.RunParallel(func(pb *testing.PB) {
			c.Request.Header.SetMethod(routesFixture.TestRoutes[i].Method)
			for pb.Next() {
				c.URI().SetPath(routesFixture.TestRoutes[i].Path)

				ctx := app.AcquireCtx(c).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

				match, err = app.next(ctx)
				app.ReleaseCtx(ctx)
			}
		})

		require.NoError(b, err)
		require.True(b, match)
	}
}

func Test_NextCustom_Radix_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	app := newCustomRadixApp()
	app.Get("/foo", func(c Ctx) error { return c.SendStatus(StatusOK) })
	useRoute := &Route{use: true, path: "/foo", Path: "/foo", routeParser: parseRoute("/foo")}
	m := app.methodInt(MethodGet)
	app.stack[m] = append([]*Route{useRoute}, app.stack[m]...)
	app.routesRefreshed = true
	app.RebuildTree()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodPost)
	fctx.Request.SetRequestURI("/foo")

	ctx := app.AcquireCtx(fctx)
	defer app.ReleaseCtx(ctx)

	matched, err := app.nextCustom(ctx)
	require.False(t, matched)
	require.ErrorIs(t, err, ErrMethodNotAllowed)
	allow := string(ctx.Response().Header.Peek(HeaderAllow))
	require.Equal(t, MethodGet, allow)
}

func Test_NextCustom_Radix_NotFound(t *testing.T) {
	t.Parallel()
	app := newCustomRadixApp()
	app.RebuildTree()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	fctx.Request.SetRequestURI("/not-exist")

	ctx := app.AcquireCtx(fctx)
	defer app.ReleaseCtx(ctx)

	matched, err := app.nextCustom(ctx)
	require.False(t, matched)
	var e *Error
	require.ErrorAs(t, err, &e)
	require.Equal(t, StatusNotFound, e.Code)
}

func Test_RequestHandler_Radix_CustomCtx_NotImplemented(t *testing.T) {
	t.Parallel()
	app := newCustomRadixApp()

	h := app.Handler()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("UNKNOWN")
	fctx.Request.SetRequestURI("/")

	h(fctx)
	require.Equal(t, StatusNotImplemented, fctx.Response.StatusCode())
}

func Test_NextCustom_Radix_Matched404(t *testing.T) {
	t.Parallel()
	app := newCustomRadixApp()
	app.RebuildTree()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	fctx.Request.SetRequestURI("/none")

	ctx := app.AcquireCtx(fctx)
	ctx.setMatched(true)
	defer app.ReleaseCtx(ctx)

	matched, err := app.nextCustom(ctx)
	require.False(t, matched)
	var e *Error
	require.ErrorAs(t, err, &e)
	require.Equal(t, StatusNotFound, e.Code)
}

func Test_NextCustom_Radix_SkipMountAndNoHandlers(t *testing.T) {
	t.Parallel()
	app := newCustomRadixApp()
	m := app.methodInt(MethodGet)
	mountR := &Route{path: "/skip", Path: "/skip", routeParser: parseRoute("/skip"), mount: true}
	empty := &Route{path: "/foo", Path: "/foo", routeParser: parseRoute("/foo")}
	app.stack[m] = []*Route{mountR, empty}
	app.routesRefreshed = true
	app.RebuildTree()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	fctx.Request.SetRequestURI("/foo")

	ctx := app.AcquireCtx(fctx)
	defer app.ReleaseCtx(ctx)

	matched, err := app.nextCustom(ctx)
	require.True(t, matched)
	require.NoError(t, err)
	require.Equal(t, "/foo", ctx.Route().Path)
}

func Test_AddRoute_Radix_MergeHandlers(t *testing.T) {
	t.Parallel()
	app := New(Config{UseRadix: true})
	count := func(_ Ctx) error { return nil }
	app.Get("/merge", count)
	app.Get("/merge", count)

	require.Len(t, app.stack[app.methodInt(MethodGet)], 1)
	require.Len(t, app.stack[app.methodInt(MethodGet)][0].Handlers, 2)
}
