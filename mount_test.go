// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"errors"
	"regexp"
	"testing"

	"github.com/gofiber/fiber/v3/internal/apptest"
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

	sub := New(Config{CaseSensitive: true, RegexHandler: regexp.MustCompile})
	sub.Get("/resource/:id<regex(ALLOW)>", func(c Ctx) error { return c.SendStatus(StatusOK) })

	parent.Use("/mounted", sub)
	apptest.Expect(t, sub, "/resource/ALLOW", StatusOK)
	apptest.Expect(t, parent, "/mounted/resource/ALLOW", StatusOK)
	apptest.Expect(t, sub, "/resource/allow", StatusNotFound)
	apptest.Expect(t, parent, "/mounted/resource/allow", StatusNotFound)
}

func Test_App_Mount(t *testing.T) {
	t.Parallel()
	micro := New()
	micro.Get("/doe", func(c Ctx) error { return c.SendStatus(StatusOK) })

	app := New()
	app.Use("/john", micro)
	apptest.Expect(t, app, "/john/doe", 200)
	require.Equal(t, uint32(2), app.handlersCount)
}

func Test_App_Mount_RootPath_Nested(t *testing.T) {
	t.Parallel()
	app := New()
	dynamic := New()
	apiserver := New()

	apiroutes := apiserver.Group("/v1")
	apiroutes.Get("/home", func(c Ctx) error { return c.SendString("home") })

	dynamic.Use("/api", apiserver)
	app.Use("/", dynamic)

	apptest.Expect(t, app, "/api/v1/home", 200)
	require.Equal(t, uint32(2), app.handlersCount)
}

func Test_App_Mount_Nested(t *testing.T) {
	t.Parallel()
	app := New()
	one := New()
	two := New()
	three := New()

	two.Use("/three", three)
	app.Use("/one", one)
	one.Use("/two", two)
	one.Get("/doe", func(c Ctx) error { return c.SendStatus(StatusOK) })
	two.Get("/nested", func(c Ctx) error { return c.SendStatus(StatusOK) })
	three.Get("/test", func(c Ctx) error { return c.SendStatus(StatusOK) })
	for _, path := range []string{"/one/doe", "/one/two/nested", "/one/two/three/test"} {
		apptest.Expect(t, app, path, 200)
	}
	require.Equal(t, uint32(6), app.handlersCount)
}

func Test_App_Mount_Express_Behavior(t *testing.T) {
	t.Parallel()
	createTestHandler := func(body string) func(c Ctx) error { return func(c Ctx) error { return c.SendString(body) } }

	app := New()
	subApp := New()
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
	app.Use(func(c Ctx) error { return c.SendStatus(StatusNotFound) })
	apptest.ExpectBody(t, app, "/world", StatusOK, "subapp world!")
	apptest.ExpectBody(t, app, "/hello", StatusOK, "app hello!")
	apptest.ExpectBody(t, app, "/bar", StatusOK, "subapp bar!")
	apptest.ExpectBody(t, app, "/foo", StatusOK, "subapp foo!")
	apptest.ExpectBody(t, app, "/unknown", StatusNotFound, ErrNotFound.Message)

	require.Equal(t, uint32(17), app.handlersCount)
}

func Test_App_Mount_RoutePositions(t *testing.T) {
	t.Parallel()
	app := New()
	subApp1 := New()
	subApp2 := New()
	app.Use(func(c Ctx) error {
		c.Locals("world", "world")
		return c.Next()
	})
	app.Use("/subApp1", subApp1)
	app.Use(func(c Ctx) error { return c.Next() })
	app.Get("/bar", func(c Ctx) error { return c.SendString("ok") })
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

	apptest.ExpectBody(t, app, "/subApp2/world", StatusOK, "hello")

	routeStackGET := app.Stack()[0]
	require.Len(t, routeStackGET, 5)
	for i, want := range []struct {
		path string
		use  bool
	}{{"/", true}, {"/", true}, {"/bar", false}, {"/", true}, {"/subapp2/world", false}} {
		require.Equal(t, want.use, routeStackGET[i].use, "route %d", i)
		require.Equal(t, want.path, routeStackGET[i].path, "route %d", i)
	}
}

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

func Test_App_Mount_ErrorHandler(t *testing.T) {
	t.Parallel()
	const (
		routeErr = "0: GET error"
		body     = "1: custom error"
	)

	tests := []struct {
		name   string
		group  string // the mounted app is mounted on this group of the parent when set
		prefix string // where the mounted app is mounted
		route  string // the route of the mounted app, which always fails
		target string // the request that reaches the failing route
		// handledByParent tells which app has the custom error handler: the one mounting or the mounted one.
		handledByParent bool
	}{
		{name: "group mount", group: "/v1", prefix: "/john", route: "/doe", target: "/v1/john/doe"},
		{name: "group mount at the root level", group: "/v1", prefix: "/", route: "/john/doe", target: "/v1/john/doe"},
		{name: "the mounting app's handler is used", prefix: "/api", route: "/", target: "/api", handledByParent: true},
		{name: "the mounted app's handler is used", prefix: "/api", route: "/", target: "/api"},
		{name: "the mounted app's handler is used at the root level", prefix: "/", route: "/api", target: "/api"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errorHandler := func(c Ctx, err error) error {
				require.Equal(t, routeErr, err.Error())
				return c.Status(StatusInternalServerError).SendString(body)
			}

			app := New()
			micro := New()
			if tc.handledByParent {
				app = New(Config{ErrorHandler: errorHandler})
			} else {
				micro = New(Config{ErrorHandler: errorHandler})
			}
			micro.Get(tc.route, func(_ Ctx) error { return errors.New(routeErr) })

			var router Router = app
			if tc.group != "" {
				router = app.Group(tc.group)
			}
			router.Use(tc.prefix, micro)

			apptest.ExpectBody(t, app, tc.target, StatusInternalServerError, body)
		})
	}
}

func Test_App_Group_Mount(t *testing.T) {
	t.Parallel()
	micro := New()
	micro.Get("/doe", func(c Ctx) error { return c.SendStatus(StatusOK) })

	app := New()
	v1 := app.Group("/v1")
	v1.Use("/john", micro)

	apptest.Expect(t, app, "/v1/john/doe", 200)
	require.Equal(t, uint32(2), app.handlersCount)
}

func Test_App_UseMountedErrorHandlerForBestPrefixMatch(t *testing.T) {
	t.Parallel()
	app := New()

	tripleSubFiber := domainTestFailingApp(200, "hi, i'm a custom sub fiber error 2", "/")

	subfiber := domainTestFailingApp(200, "hi, i'm a custom sub fiber error", "/")
	subfiber.Use("/third", tripleSubFiber)

	fiber := domainTestFailingApp(200, "hi, i'm a custom error", "/")
	fiber.Use("/sub", subfiber)

	app.Use("/api", fiber)

	apptest.ExpectBody(t, app, "/api/sub", 200, "hi, i'm a custom sub fiber error")
	apptest.ExpectBody(t, app, "/api/sub/third", 200, "hi, i'm a custom sub fiber error 2")
}

func Test_Mount_Route_Names(t *testing.T) {
	t.Parallel()
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

	subApp2 := New()
	app2Grp := subApp2.Group("/users").Name("users.")
	app2Grp.Get("", emptyHandler).Name("get")
	app2Grp.Post("", emptyHandler).Name("add")

	rootApp := New()
	_ = rootApp.Use("/app1", subApp1)
	_ = rootApp.Use("/app2", subApp2)

	rootApp.startupProcess()

	for _, tc := range []struct {
		app    *App
		name   string
		method string
		path   string
	}{
		// take route directly from sub-app
		{app: subApp1, name: "get-users", method: MethodGet, path: "/users"},
		{app: subApp1, name: "add-user", method: MethodPost, path: "/users"},
		// take route directly from sub-app with group
		{app: subApp2, name: "users.get", method: MethodGet, path: "/users"},
		{app: subApp2, name: "users.add", method: MethodPost, path: "/users"},
		// take route from root app (using names of sub-apps)
		{app: rootApp, name: "add-user", method: MethodPost, path: "/app1/users"},
		{app: rootApp, name: "users.add", method: MethodPost, path: "/app2/users"},
	} {
		route := tc.app.GetRoute(tc.name)
		require.Equal(t, tc.method, route.Method, tc.name)
		require.Equal(t, tc.path, route.Path, tc.name)
	}

	// GetRouteURL inside handler
	apptest.Expect(t, rootApp, "/app1/users", StatusOK)

	// ctx.App().GetRoute() inside handler
	apptest.Expect(t, rootApp, apptest.Req(MethodPost, "/app1/users"), StatusOK)
}

func Test_Ctx_Render_Mount(t *testing.T) {
	t.Parallel()
	sub := New(Config{Views: domainTestEngine(t, "")})
	sub.Get("/:name", func(c Ctx) error { return c.Render("hello_world.tmpl", Map{"Name": c.Params("name")}) })

	app := New()
	app.Use("/hello", sub)

	apptest.ExpectBody(t, app, "/hello/a", StatusOK, "<h1>Hello a!</h1>")
}

func Test_Ctx_Render_Mount_ParentOrSubHasViews(t *testing.T) {
	t.Parallel()
	sub := New(Config{Views: domainTestEngine(t, "testdata3")})
	sub2 := New(Config{Views: domainTestEngine(t, "testdata2")})
	app := New(Config{Views: domainTestEngine(t, "")})
	app.Get("/test", func(c Ctx) error { return c.Render("index.tmpl", Map{"Title": "Hello, World!"}) })
	sub.Get("/world/:name", func(c Ctx) error { return c.Render("hello_world.tmpl", Map{"Name": c.Params("name")}) })
	sub2.Get("/moment", func(c Ctx) error { return c.Render("bruh.tmpl", Map{}) })
	sub.Use("/bruh", sub2)
	app.Use("/hello", sub)

	apptest.ExpectBody(t, app, "/hello/world/a", StatusOK, "<h1>Hello a!</h1>")
	apptest.ExpectBody(t, app, "/test", StatusOK, "<h1>Hello, World!</h1>")
	apptest.ExpectBody(t, app, "/hello/bruh/moment", StatusOK, "<h1>I'm Bruh</h1>")
}

func Test_Ctx_Render_MountGroup(t *testing.T) {
	t.Parallel()
	micro := New(Config{Views: domainTestEngine(t, "")})
	micro.Get("/doe", func(c Ctx) error { return c.Render("hello_world.tmpl", Map{"Name": "doe"}) })

	app := New()
	v1 := app.Group("/v1")
	v1.Use("/john", micro)

	apptest.ExpectBody(t, app, "/v1/john/doe", 200, "<h1>Hello doe!</h1>")
}

// Test_App_Mount_FewerRequestMethods verifies that a sub-app serving only some
// of the parent's request methods can be mounted, directly and through another
// mount: the parent must read the sub-app's routes by method, not by its own
// stack index.
func Test_App_Mount_FewerRequestMethods(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path string
		nested     bool // micro is reached through another mount
	}{
		{name: "direct", path: "/john/doe"},
		{name: "nested", path: "/john/v1/doe", nested: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			micro := New(Config{RequestMethods: []string{MethodGet}})
			micro.Get("/doe", func(c Ctx) error { return c.SendString("doe") })
			mounted := micro
			if tc.nested {
				mounted = New()
				mounted.Use("/v1", micro)
			}
			app := New()
			app.Use("/john", mounted)
			apptest.ExpectBody(t, app, tc.path, StatusOK, "doe")
		})
	}
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
	micro.Get("/doe", func(c Ctx) error { return c.SendString("get doe") })
	micro.Post("/doe", func(c Ctx) error { return c.SendString("post doe") })

	app := New()
	app.Use("/john", micro)

	apptest.ExpectBody(t, app, "/john/doe", StatusOK, "get doe")

	apptest.ExpectBody(t, app, apptest.Req(MethodPost, "/john/doe"), StatusOK, "post doe")
}

// onlyFooConstraint and onlyBarConstraint are custom constraints used to check
// that a mounted app's constraints survive the mount's route expansion.
type onlyFooConstraint struct{}

func (*onlyFooConstraint) Name() string { return "onlyfoo" }

func (*onlyFooConstraint) Execute(param string, _ ...string) bool { return param == "foo" }

type onlyBarConstraint struct{}

func (*onlyBarConstraint) Name() string { return "onlybar" }

func (*onlyBarConstraint) Execute(param string, _ ...string) bool { return param == "bar" }

// mountSameNameConstraint is named "same" whatever value it accepts, as two
// apps mounted side by side are free to give one name different meanings.
type mountSameNameConstraint struct{ allowed string }

func (*mountSameNameConstraint) Name() string { return "same" }

func (c *mountSameNameConstraint) Execute(param string, _ ...string) bool { return param == c.allowed }

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
		foo.RegisterCustomConstraint(&mountSameNameConstraint{allowed: "foo"})
		foo.Get("/:value<same>", handler)

		bar := New()
		bar.RegisterCustomConstraint(&mountSameNameConstraint{allowed: "bar"})
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
		resp := apptest.Do(t, plain, path)
		require.Equal(t, status, resp.StatusCode, "plain "+path)

		resp = apptest.Do(t, domain, apptest.Req(MethodGet, path, apptest.WithHost("api.example.com")))
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
	micro.Get("/doe/:name<onlyfoo>", func(c Ctx) error { return c.SendString(c.Params("name")) })

	sub := New()
	sub.Use("/v1", micro)

	app := New()
	app.Use("/john", sub)

	apptest.Expect(t, app, "/john/v1/doe/foo", StatusOK)

	apptest.Expect(t, app, "/john/v1/doe/bar", StatusNotFound)
}

// Test_App_Mount_CarriesDomainMount verifies that mounting an app which has a
// domain mount of its own carries that mount's config along, host check
// included.
func Test_App_Mount_CarriesDomainMount(t *testing.T) {
	t.Parallel()
	micro := domainTestFailingApp(596, "micro error", "/doe")

	sub := New()
	sub.Domain("api.example.com").Use("/v1", micro)

	app := New()
	app.Use("/john", sub)

	apptest.Expect(t, app, apptest.Req(MethodGet, "/john/v1/doe", apptest.WithHost("api.example.com")), 596)

	// The route does not match on another host, and the sub-app's error
	// handler must not answer for it either.
	apptest.Expect(t, app, apptest.Req(MethodGet, "/john/v1/doe", apptest.WithHost("www.example.com")), StatusNotFound)
}

// routerMountVariants are the two ways a test mounts an app: plainly under /john, and through a domain router under
// /api on api.example.com. host is the Host that reaches the mount.
var routerMountVariants = []struct {
	mount  func(app, micro *App)
	host   apptest.Option
	name   string
	prefix string
}{
	{
		name: "plain", prefix: "/john", host: apptest.WithHost("example.com"),
		mount: func(app, micro *App) { app.Use("/john", micro) },
	},
	{
		name: "domain", prefix: "/api", host: apptest.WithHost("api.example.com"),
		mount: func(app, micro *App) { app.Domain("api.example.com").Use("/api", micro) },
	},
}

// Test_App_Mount_AutoHeadKeepsMiddleware verifies that a mounted app which does
// not serve HEAD gets no synthesized HEAD routes once its routes are expanded
// into the parent, directly or through a domain router — they would run without
// the middleware it registered.
func Test_App_Mount_AutoHeadKeepsMiddleware(t *testing.T) {
	t.Parallel()
	for _, tc := range routerMountVariants {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			micro := New(Config{RequestMethods: []string{MethodGet, MethodPost}})
			micro.Use(func(c Ctx) error { return c.SendStatus(StatusUnauthorized) })
			micro.Get("/doe", func(c Ctx) error {
				c.Set("X-Secret", "leaked")
				return c.SendString("doe")
			})

			app := New()
			tc.mount(app, micro)
			target := tc.prefix + "/doe"

			apptest.Expect(t, app, apptest.Req(MethodGet, target, tc.host), StatusUnauthorized)

			// Twice: app.Test runs the startup process on every call, and the second
			// pass sees the mount already expanded into the parent's stack.
			for range 2 {
				resp := apptest.Expect(t, app, apptest.Req(MethodHead, target, tc.host), StatusMethodNotAllowed)
				require.Empty(t, resp.Header.Get("X-Secret"))
			}
		})
	}
}

// Test_App_Mount_RootMountKeepsParentAutoHead verifies that a mounted app which
// does not serve HEAD only withholds automatic HEAD routes from its own routes.
// A mount at "/" covers every path, and the app above keeps its own.
func Test_App_Mount_RootMountKeepsParentAutoHead(t *testing.T) {
	t.Parallel()
	micro := New(Config{RequestMethods: []string{MethodGet, MethodPost}})
	micro.Get("/doe", func(c Ctx) error { return c.SendString("doe") })

	app := New()
	app.Get("/own", func(c Ctx) error { return c.SendString("own") })
	app.Use("/", micro)

	// the app's own route keeps its HEAD route
	apptest.Expect(t, app, apptest.Req(MethodHead, "/own"), StatusOK)
	// the mounted app serves no HEAD
	apptest.Expect(t, app, apptest.Req(MethodHead, "/doe"), StatusMethodNotAllowed)
}

// Test_App_Mount_ParametricPrefix verifies that a mount prefix carrying a
// parameter is matched as a pattern, also when an outer mount applies the
// prefix a second time. The prefix rewrites every route of the mounted app, so
// the parameters it introduces have to be rebuilt onto them.
func Test_App_Mount_ParametricPrefix(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, param, path string
		nested            bool // micro is mounted under the prefix by an app that is mounted itself
	}{
		{name: "direct", param: "Version", path: "/v1/42/doe"},
		{name: "nested", param: "version", path: "/john/v1/42/doe", nested: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			micro := New()
			micro.Get("/doe", func(c Ctx) error { return c.SendString("version=" + c.Params(tc.param)) })
			app := New()
			if tc.nested {
				sub := New()
				sub.Use("/v1/:"+tc.param, micro)
				app.Use("/john", sub)
			} else {
				app.Use("/v1/:"+tc.param, micro)
			}
			apptest.ExpectBody(t, app, tc.path, StatusOK, "version=42")
		})
	}
}

// Test_App_Mount_CaseInsensitivePath verifies that a mount path is matched by
// the app's own routing rules when its config is resolved: a case-insensitive
// app serves "/api/doe" from a mount registered as "/API", so the mounted app's
// error handler has to answer for it too.
func Test_App_Mount_CaseInsensitivePath(t *testing.T) {
	t.Parallel()
	micro := domainTestFailingApp(599, "micro error", "/doe")

	app := New()
	app.Use("/API", micro)

	apptest.Expect(t, app, "/api/doe", 599)

	// A case-sensitive app keeps the two apart.
	strict := New(Config{CaseSensitive: true})
	strict.Use("/API", micro)

	apptest.Expect(t, strict, "/API/doe", 599)

	apptest.Expect(t, strict, "/api/doe", StatusNotFound)
}

// Test_App_Mount_ViewsPathOnly verifies that a mounted app's views are chosen
// by the request path, not by its mount path appearing anywhere in the URL — a
// query string carrying it must not select them.
func Test_App_Mount_ViewsPathOnly(t *testing.T) {
	t.Parallel()
	for _, tc := range routerMountVariants {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			micro := New(Config{Views: domainTestEngine(t, "testdata2")})
			micro.Get("/view", func(c Ctx) error { return c.Render("bruh.tmpl", Map{}) })

			app := New(Config{Views: domainTestEngine(t, "")})
			tc.mount(app, micro)
			app.Get("/elsewhere", func(c Ctx) error { return c.Render("index.tmpl", Map{"Title": "parent"}) })

			req := apptest.Req(MethodGet, "/elsewhere?next="+tc.prefix+"/view", tc.host)
			apptest.ExpectBody(t, app, req, StatusOK, "<h1>parent</h1>")
		})
	}
}

// Test_App_Mount_ParametricPrefixConstraint verifies that a constraint named in
// a mount prefix is enforced. The prefix belongs to the app doing the mounting,
// so its constraints have to reach the re-parse of the mounted routes.
func Test_App_Mount_ParametricPrefixConstraint(t *testing.T) {
	t.Parallel()
	micro := New()
	micro.Get("/doe", func(c Ctx) error { return c.SendString("name=" + c.Params("name")) })

	app := New()
	app.RegisterCustomConstraint(&onlyFooConstraint{})
	app.Use("/:name<onlyfoo>", micro)

	apptest.Expect(t, app, "/foo/doe", StatusOK)
	// the prefix constraint still rejects
	apptest.Expect(t, app, "/bar/doe", StatusNotFound)
}

// Test_App_Mount_PrefixAsWritten verifies that mounted routes are re-parsed with
// the mount prefix as written, so they answer only where middleware on it runs.
func Test_App_Mount_PrefixAsWritten(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		prefix   string
		guarded  string
		notFound []string
	}{
		{prefix: `/v1\:internal`, guarded: "/v1:internal/doe", notFound: []string{"/v1foo/doe", "/v1/doe"}},
		{prefix: `/a\*`, guarded: "/a*/doe", notFound: []string{"/a/doe", "/a/b/doe"}},
		{prefix: "/even/:n<isEven>", guarded: "/even/4/doe", notFound: []string{"/even/3/doe", "/even/abc/doe"}},
		{prefix: "/code/:code<regex(^[A-Z]{2}$)>", guarded: "/code/US/doe", notFound: []string{"/code/us/doe"}},
		{prefix: `/num/:n<regex(^\d+$)>`, guarded: "/num/12/doe", notFound: []string{"/num/dd/doe"}},
	}

	mounts := map[string]func(app *App, prefix string, guard Handler, micro *App){
		"app": func(app *App, prefix string, guard Handler, micro *App) {
			app.Use(prefix, guard)
			app.Use(prefix, micro)
		},
		"group": func(app *App, prefix string, guard Handler, micro *App) {
			grp := app.Group("/")
			grp.Use(prefix, guard)
			grp.Use(prefix, micro)
		},
		"domain": func(app *App, prefix string, guard Handler, micro *App) {
			app.Use(prefix, guard)
			app.Domain("example.com").Use(prefix, micro)
		},
		"nested in domain": func(app *App, prefix string, guard Handler, micro *App) {
			mid := New()
			mid.RegisterCustomConstraint(&evenConstraint{})
			mid.Use(prefix, guard)
			mid.Use(prefix, micro)
			app.Domain("example.com").Use("/", mid)
		},
	}

	for name, mount := range mounts {
		for _, tc := range testCases {
			t.Run(name+" "+tc.prefix, func(t *testing.T) {
				t.Parallel()
				micro := New()
				micro.Get("/doe", func(c Ctx) error { return c.SendString("doe") })

				app := New()
				app.RegisterCustomConstraint(&evenConstraint{})
				mount(app, tc.prefix, func(c Ctx) error { return c.SendStatus(StatusUnauthorized) }, micro)

				resp := apptest.Do(t, app, tc.guarded)
				require.Equal(t, StatusUnauthorized, resp.StatusCode, "GET %s", tc.guarded)

				for _, path := range tc.notFound {
					resp = apptest.Do(t, app, path)
					require.Equal(t, StatusNotFound, resp.StatusCode, "GET %s", path)
				}
			})
		}
	}
}

// Test_App_Mount_NestedPrefixKeepsCase verifies that a nested mount keeps the
// case of its prefix when only the top-level app is case-sensitive.
func Test_App_Mount_NestedPrefixKeepsCase(t *testing.T) {
	t.Parallel()
	sub := New()
	sub.Get("/secret", func(c Ctx) error { return c.SendString("secret") })
	mid := New()
	mid.Use("/Admin", sub)

	app := New(Config{CaseSensitive: true})
	app.Use("/v1/Admin", func(c Ctx) error { return c.SendStatus(StatusUnauthorized) })
	app.Use("/v1", mid)

	apptest.Expect(t, app, "/v1/Admin/secret", StatusUnauthorized)

	apptest.Expect(t, app, "/v1/admin/secret", StatusNotFound)
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
			resp := apptest.Do(t, app, tc.path)
			require.Equal(t, want, resp.StatusCode, "strict=%v path=%q", strict, tc.path)
		}
	}
}

// Test_App_Mount_StrictRouting_RootMiddleware pins that a mounted app's root
// middleware runs for "/users", which "/users/:id?" answers under StrictRouting too.
func Test_App_Mount_StrictRouting_RootMiddleware(t *testing.T) {
	t.Parallel()
	for _, strict := range []bool{false, true} {
		// "/u/" is as long as a tree hash, which "/u" is too short to carry
		for prefix, mounted := range map[string]string{"/users": "/users", "/u": "/u", "/t/:tenant": "/t/acme"} {
			for _, pattern := range []string{"/:id?", "/*"} {
				users := New()
				users.Use(func(c Ctx) error { return c.SendStatus(StatusUnauthorized) })
				users.Get(pattern, func(c Ctx) error { return c.SendString("users") })
				app := New(Config{StrictRouting: strict})
				app.Use(prefix, users)

				for _, target := range []string{mounted, mounted + "/", mounted + "/7"} {
					resp := apptest.Do(t, app, target)
					require.Equal(t, StatusUnauthorized, resp.StatusCode, "strict=%v prefix=%q pattern=%q path=%q", strict, prefix, pattern, target)
				}

				resp := apptest.Do(t, app, "/other")
				require.Equal(t, StatusNotFound, resp.StatusCode, "strict=%v prefix=%q pattern=%q", strict, prefix, pattern)
			}
		}
	}
}
