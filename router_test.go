// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📃 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

var routesFixture routeJSON

func init() {
	dat, err := os.ReadFile("./.github/testdata/testRoutes.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(dat, &routesFixture); err != nil {
		panic(err)
	}
}

func Test_Route_Handler_Order(t *testing.T) {
	t.Parallel()

	app := New()

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

func Test_Route_Match_SameLength(t *testing.T) {
	t.Parallel()

	app := New()

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

func Test_Route_Match_Star(t *testing.T) {
	t.Parallel()

	app := New()

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

func Test_Route_Match_Root(t *testing.T) {
	t.Parallel()

	app := New()

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

func Test_Route_Match_Parser(t *testing.T) {
	t.Parallel()

	app := New()

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

func Test_Route_Match_Middleware(t *testing.T) {
	t.Parallel()

	app := New()

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

func Test_Route_Match_UnescapedPath(t *testing.T) {
	t.Parallel()

	app := New(Config{UnescapePath: true})

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

func Test_Route_Match_WithEscapeChar(t *testing.T) {
	t.Parallel()

	app := New()
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

func Test_Route_Match_Middleware_HasPrefix(t *testing.T) {
	t.Parallel()

	app := New()

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

func Test_Route_Match_Middleware_Root(t *testing.T) {
	t.Parallel()

	app := New()

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

func Test_Router_Register_Missing_Handler(t *testing.T) {
	t.Parallel()

	app := New()

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

func Test_Ensure_Router_Interface_Implementation(t *testing.T) {
	t.Parallel()

	var app any = (*App)(nil)
	_, ok := app.(Router)
	require.True(t, ok)

	var group any = (*Group)(nil)
	_, ok = group.(Router)
	require.True(t, ok)
}

func Test_Router_Handler_Catch_Error(t *testing.T) {
	t.Parallel()

	app := New()
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

func Test_Router_NotFound(t *testing.T) {
	t.Parallel()
	app := New()
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

func Test_Router_NotFound_HTML_Inject(t *testing.T) {
	t.Parallel()
	app := New()
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

func registerTreeManipulationRoutes(app *App, middleware ...func(Ctx) error) {
	app.Get("/test", func(c Ctx) error {
		app.Get("/dynamically-defined", func(c Ctx) error {
			return c.SendStatus(StatusOK)
		})

		app.RebuildTree()

		return c.SendStatus(StatusOK)
	}, middleware...)
}

func verifyRequest(tb testing.TB, app *App, path string, expectedStatus int) *http.Response {
	tb.Helper()

	resp, err := app.Test(httptest.NewRequest(MethodGet, path, nil))
	require.NoError(tb, err, "app.Test(req)")
	require.Equal(tb, expectedStatus, resp.StatusCode, "Status code")

	return resp
}

func verifyRouteHandlerCounts(tb testing.TB, app *App, expectedRoutesCount int) {
	tb.Helper()

	//  this is taken from listen.go's printRoutesMessage app method
	var routes []RouteMessage
	for _, routeStack := range app.stack {
		for _, route := range routeStack {
			routeMsg := RouteMessage{
				name:   route.Name,
				method: route.Method,
				path:   route.Path,
			}

			for _, handler := range route.Handlers {
				routeMsg.handlers += runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name() + " "
			}

			routes = append(routes, routeMsg)
		}
	}

	for _, route := range routes {
		require.Equal(tb, expectedRoutesCount, strings.Count(route.handlers, " "))
	}
}

func verifyThereAreNoRoutes(tb testing.TB, app *App) {
	tb.Helper()

	require.Equal(tb, uint32(0), app.handlersCount)
	verifyRouteHandlerCounts(tb, app, 0)
}

func Test_App_Rebuild_Tree(t *testing.T) {
	t.Parallel()
	app := New()

	registerTreeManipulationRoutes(app)

	verifyRequest(t, app, "/dynamically-defined", StatusNotFound)
	verifyRequest(t, app, "/test", StatusOK)
	verifyRequest(t, app, "/dynamically-defined", StatusOK)
}

func Test_App_Remove_Route_A_B_Feature_Testing(t *testing.T) {
	t.Parallel()
	app := New()

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

func Test_App_Remove_Route_By_Name(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/api/test", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	}).Name("test")

	app.RemoveRouteByName("test", MethodGet)
	app.RebuildTree()

	verifyRequest(t, app, "/test", StatusNotFound)
	verifyThereAreNoRoutes(t, app)
}

func Test_App_Remove_Route_By_Name_Non_Existing_Route(t *testing.T) {
	t.Parallel()
	app := New()

	app.RemoveRouteByName("test", MethodGet)
	app.RebuildTree()

	verifyThereAreNoRoutes(t, app)
}

func Test_App_Remove_Route_Nested(t *testing.T) {
	t.Parallel()
	app := New()

	api := app.Group("/api")

	v1 := api.Group("/v1")
	v1.Get("/test", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	verifyRequest(t, app, "/api/v1/test", StatusOK)
	app.RemoveRoute("/api/v1/test", MethodGet)

	verifyThereAreNoRoutes(t, app)
}

func Test_App_Remove_Route_Parameterized(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/test/:id", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})
	verifyRequest(t, app, "/test/:id", StatusOK)
	app.RemoveRoute("/test/:id", MethodGet)

	verifyThereAreNoRoutes(t, app)
}

func Test_App_Remove_Route(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/test", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	app.RemoveRoute("/test", MethodGet)
	app.RebuildTree()

	verifyRequest(t, app, "/test", StatusNotFound)
}

func Test_App_Remove_Route_Non_Existing_Route(t *testing.T) {
	t.Parallel()
	app := New()

	app.RemoveRoute("/test", MethodGet, MethodHead)
	app.RebuildTree()

	verifyThereAreNoRoutes(t, app)
}

func Test_App_Remove_Route_Concurrent(t *testing.T) {
	t.Parallel()
	app := New()

	// Add test route
	app.Get("/test", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	// Concurrently remove and add routes
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
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

func Test_Route_Registration_Prevent_Duplicate_With_Middleware(t *testing.T) {
	t.Parallel()
	app := New()

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

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		expected      string
		caseSensitive bool
		strictRouting bool
	}{
		{
			name:          "Empty path",
			path:          "",
			caseSensitive: true,
			strictRouting: true,
			expected:      "/",
		},
		{
			name:          "No leading slash",
			path:          "users",
			caseSensitive: true,
			strictRouting: true,
			expected:      "/users",
		},
		{
			name:          "With trailing slash and strict routing",
			path:          "/users/",
			caseSensitive: true,
			strictRouting: true,
			expected:      "/users/",
		},
		{
			name:          "With trailing slash and non-strict routing",
			path:          "/users/",
			caseSensitive: true,
			strictRouting: false,
			expected:      "/users",
		},
		{
			name:          "Case sensitive",
			path:          "/Users",
			caseSensitive: true,
			strictRouting: true,
			expected:      "/Users",
		},
		{
			name:          "Case insensitive",
			path:          "/Users",
			caseSensitive: false,
			strictRouting: true,
			expected:      "/users",
		},
		{
			name:          "With escape characters",
			path:          "/users\\/profile",
			caseSensitive: true,
			strictRouting: true,
			expected:      "/users/profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				config: Config{
					CaseSensitive: tt.caseSensitive,
					StrictRouting: tt.strictRouting,
				},
			}
			result := app.normalizePath(tt.path)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestRemoveRoute(t *testing.T) {
	app := New()

	var buf strings.Builder

	app.Use(func(c Ctx) error {
		buf.WriteString("1") //nolint:errcheck // not needed
		return c.Next()
	})

	app.Post("/", func(c Ctx) error {
		buf.WriteString("2") //nolint:errcheck // not needed
		return c.SendStatus(StatusOK)
	})

	app.Use("/test", func(c Ctx) error {
		buf.WriteString("3") //nolint:errcheck // not needed
		return c.Next()
	})

	app.Get("/test", func(c Ctx) error {
		buf.WriteString("4") //nolint:errcheck // not needed
		return c.SendStatus(StatusOK)
	})

	app.Post("/test", func(c Ctx) error {
		buf.WriteString("5") //nolint:errcheck // not needed
		return c.SendStatus(StatusOK)
	})

	require.Equal(t, uint32(5), app.handlersCount)

	req, err := http.NewRequestWithContext(context.Background(), MethodPost, "/", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)

	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "12", buf.String())

	buf.Reset()

	req, err = http.NewRequestWithContext(context.Background(), MethodGet, "/test", nil)
	require.NoError(t, err)

	resp, err = app.Test(req)
	require.NoError(t, err)

	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "134", buf.String())

	buf.Reset()

	require.Equal(t, uint32(5), app.handlersCount)

	app.RemoveRoute("/test", MethodGet)
	app.RebuildTree()

	app.RemoveRoute("/test", "TEST")
	app.RebuildTree()

	app.RemoveRouteFunc(func(_ *Route) bool {
		return false
	}, MethodGet)

	req, err = http.NewRequestWithContext(context.Background(), MethodGet, "/test", nil)
	require.NoError(t, err)

	resp, err = app.Test(req)
	require.NoError(t, err)

	buf.Reset()

	require.Equal(t, StatusMethodNotAllowed, resp.StatusCode)

	require.Equal(t, uint32(4), app.handlersCount)

	app.RemoveRoute("/test", MethodPost)
	app.RebuildTree()

	require.Equal(t, uint32(3), app.handlersCount)

	req, err = http.NewRequestWithContext(context.Background(), MethodPost, "/test", nil)
	require.NoError(t, err)

	resp, err = app.Test(req)
	require.NoError(t, err)

	require.Equal(t, 404, resp.StatusCode)
	require.Equal(t, "1", buf.String())

	buf.Reset()

	req, err = http.NewRequestWithContext(context.Background(), MethodGet, "/test", nil)
	require.NoError(t, err)

	resp, err = app.Test(req)
	require.NoError(t, err)

	require.Equal(t, 404, resp.StatusCode)
	require.Equal(t, "1", buf.String())

	buf.Reset()

	app.RemoveRoute("/", MethodGet, MethodPost)

	require.Equal(t, uint32(2), app.handlersCount)

	req, err = http.NewRequestWithContext(context.Background(), MethodGet, "/", nil)
	require.NoError(t, err)

	resp, err = app.Test(req)
	require.NoError(t, err)

	require.Equal(t, 404, resp.StatusCode)
	require.Equal(t, "", buf.String())

	buf.Reset()

	app.RemoveRoute("/test", MethodGet, MethodPost)

	require.Equal(t, uint32(2), app.handlersCount)

	app.RemoveRoute("/test", app.config.RequestMethods...)

	require.Equal(t, uint32(1), app.handlersCount)

	app.Patch("/test", func(c Ctx) error {
		buf.WriteString("6") //nolint:errcheck // not needed
		return c.SendStatus(StatusOK)
	})

	require.Equal(t, uint32(2), app.handlersCount)

	app.RemoveRoute("/test")
	app.RemoveRoute("/")
	app.RebuildTree()

	require.Equal(t, uint32(0), app.handlersCount)
}

//////////////////////////////////////////////
///////////////// BENCHMARKS /////////////////
//////////////////////////////////////////////

func registerDummyRoutes(app *App) {
	h := func(_ Ctx) error {
		return nil
	}
	for _, r := range routesFixture.GithubAPI {
		app.Add([]string{r.Method}, r.Path, h)
	}
}

// go test -v -run=^$ -bench=Benchmark_App_MethodNotAllowed -benchmem -count=4
func Benchmark_App_MethodNotAllowed(b *testing.B) {
	app := New()
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

// go test -v ./... -run=^$ -bench=Benchmark_Router_NotFound -benchmem -count=4
func Benchmark_Router_NotFound(b *testing.B) {
	app := New()
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

// go test -v ./... -run=^$ -bench=Benchmark_Router_Handler -benchmem -count=4
func Benchmark_Router_Handler(b *testing.B) {
	app := New()
	registerDummyRoutes(app)
	appHandler := app.Handler()

	c := &fasthttp.RequestCtx{}

	c.Request.Header.SetMethod("DELETE")
	c.URI().SetPath("/user/keys/1337")

	for b.Loop() {
		appHandler(c)
	}
}

func Benchmark_Router_Handler_Strict_Case(b *testing.B) {
	app := New(Config{
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

// go test -v ./... -run=^$ -bench=Benchmark_Router_Chain -benchmem -count=4
func Benchmark_Router_Chain(b *testing.B) {
	app := New()
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

// go test -v ./... -run=^$ -bench=Benchmark_Router_WithCompression -benchmem -count=4
func Benchmark_Router_WithCompression(b *testing.B) {
	app := New()
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

// go test -run=^$ -bench=Benchmark_Startup_Process -benchmem -count=9
func Benchmark_Startup_Process(b *testing.B) {
	for b.Loop() {
		app := New()
		registerDummyRoutes(app)
		app.startupProcess()
	}
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Next -benchmem -count=4
func Benchmark_Router_Next(b *testing.B) {
	app := New()
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
	require.Equal(b, 4, c.indexRoute)
}

// go test -v ./... -run=^$ -bench=Benchmark_Router_Next_Default -benchmem -count=4
func Benchmark_Router_Next_Default(b *testing.B) {
	app := New()
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

// go test -benchmem -run=^$ -bench ^Benchmark_Router_Next_Default_Parallel$ github.com/gofiber/fiber/v3 -count=1
func Benchmark_Router_Next_Default_Parallel(b *testing.B) {
	app := New()
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

// go test -v ./... -run=^$ -bench=Benchmark_Router_Next_Default_Immutable -benchmem -count=4
func Benchmark_Router_Next_Default_Immutable(b *testing.B) {
	app := New(Config{Immutable: true})
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

// go test -benchmem -run=^$ -bench ^Benchmark_Router_Next_Default_Parallel_Immutable$ github.com/gofiber/fiber/v3 -count=1
func Benchmark_Router_Next_Default_Parallel_Immutable(b *testing.B) {
	app := New(Config{Immutable: true})
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

// go test -v ./... -run=^$ -bench=Benchmark_Route_Match -benchmem -count=4
func Benchmark_Route_Match(b *testing.B) {
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

// go test -v ./... -run=^$ -bench=Benchmark_Route_Match_Star -benchmem -count=4
func Benchmark_Route_Match_Star(b *testing.B) {
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

// go test -v ./... -run=^$ -bench=Benchmark_Route_Match_Root -benchmem -count=4
func Benchmark_Route_Match_Root(b *testing.B) {
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

// go test -v ./... -run=^$ -bench=Benchmark_Router_Handler_CaseSensitive -benchmem -count=4
func Benchmark_Router_Handler_CaseSensitive(b *testing.B) {
	app := New()
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

// go test -v ./... -run=^$ -bench=Benchmark_Router_Handler_Unescape -benchmem -count=4
func Benchmark_Router_Handler_Unescape(b *testing.B) {
	app := New()
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

// go test -run=^$ -bench=Benchmark_Router_Handler_StrictRouting -benchmem -count=4
func Benchmark_Router_Handler_StrictRouting(b *testing.B) {
	app := New()
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

// go test -run=^$ -bench=Benchmark_Router_Github_API -benchmem -count=16
func Benchmark_Router_Github_API(b *testing.B) {
	app := New()
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

type testRoute struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type routeJSON struct {
	TestRoutes []testRoute `json:"test_routes"`
	GithubAPI  []testRoute `json:"github_api"`
}

func newCustomApp() *App {
	return NewWithCustomCtx(func(app *App) CustomCtx {
		return &customCtx{DefaultCtx: *NewDefaultCtx(app)}
	})
}

func Test_NextCustom_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	app := newCustomApp()
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

func Test_NextCustom_NotFound(t *testing.T) {
	t.Parallel()
	app := newCustomApp()
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

func Test_RequestHandler_CustomCtx_NotImplemented(t *testing.T) {
	t.Parallel()
	app := newCustomApp()

	h := app.Handler()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("UNKNOWN")
	fctx.Request.SetRequestURI("/")

	h(fctx)
	require.Equal(t, StatusNotImplemented, fctx.Response.StatusCode())
}

func Test_NextCustom_Matched404(t *testing.T) {
	t.Parallel()
	app := newCustomApp()
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

func Test_NextCustom_SkipMountAndNoHandlers(t *testing.T) {
	t.Parallel()
	app := newCustomApp()
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

func Test_AddRoute_MergeHandlers(t *testing.T) {
	t.Parallel()
	app := New()
	count := func(_ Ctx) error { return nil }
	app.Get("/merge", count)
	app.Get("/merge", count)

	require.Len(t, app.stack[app.methodInt(MethodGet)], 1)
	require.Len(t, app.stack[app.methodInt(MethodGet)][0].Handlers, 2)
}
