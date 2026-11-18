// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

// Test_App_Mount_ExpansionLockOrder checks that a parent expanding the mounts of a sub-app takes the sub-app's
// lock before its Once, in the order the sub-app's own startup does. Taken inside the Once, a parent and a
// sub-app starting together deadlock: the parent holds the Once and waits for the lock, the sub-app holds the
// lock and waits for the Once.
func Test_App_Mount_ExpansionLockOrder(t *testing.T) {
	t.Parallel()
	leaf := New()
	leaf.Get("/x", testEmptyHandler)
	sub := New()
	sub.Use("/leaf", leaf)
	parent := New()
	parent.Use("/sub", sub)

	// The sub-app's own startup holds its lock throughout, mount expansion included.
	sub.mutex.Lock()
	released := false
	defer func() {
		if !released {
			sub.mutex.Unlock()
		}
	}()
	sub.collectSubApps()

	expanded := make(chan struct{})
	go func() {
		defer close(expanded)
		parent.mutex.Lock()
		defer parent.mutex.Unlock()
		parent.mountStartupProcess()
	}()
	// Let the parent reach the sub-app, where taking the lock inside the Once would leave it holding the Once.
	time.Sleep(50 * time.Millisecond)

	runBounded(t, "a sub-app expanding its mounts while its parent expands them", sub.mountStartupProcess)
	released = true
	sub.mutex.Unlock()
	<-expanded

	apptest.Expect(t, parent, "/sub/leaf/x", StatusOK)
}

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

// mountAnyConstraint accepts every value under the name of another constraint.
type mountAnyConstraint struct{ name string }

func (c *mountAnyConstraint) Name() string { return c.name }

func (*mountAnyConstraint) Execute(string, ...string) bool { return true }

// Test_App_Mount_PrefixConstraintOfMountingApp verifies that a constraint in a
// mount prefix stays the one of the app that wrote the prefix, as for middleware
// on it, when the mounted app gives the same name to a constraint of its own or
// overrides a built-in one.
func Test_App_Mount_PrefixConstraintOfMountingApp(t *testing.T) {
	t.Parallel()
	const host = "api.example.com"
	guard := func(c Ctx) error { return c.SendStatus(StatusUnauthorized) }
	handler := func(c Ctx) error { return c.SendString(c.Params("own")) }

	for _, tc := range []struct{ constraint, covered, outside string }{
		{constraint: "same", covered: "foo", outside: "FOO"},
		{constraint: ConstraintInt, covered: "12", outside: "twelve"},
	} {
		prefix := "/t/:name<" + tc.constraint + ">"
		newApp := func() *App {
			app := New()
			app.RegisterCustomConstraint(&mountSameNameConstraint{allowed: "foo"})
			return app
		}
		newMicro := func() *App {
			micro := New()
			micro.RegisterCustomConstraint(&mountAnyConstraint{name: tc.constraint})
			micro.Get("/doe/:own<"+tc.constraint+">", handler)
			return micro
		}
		mounts := map[string]func(app *App, guard Handler){
			"plain":  func(app *App, guard Handler) { app.Use(prefix, guard); app.Use(prefix, newMicro()) },
			"group":  func(app *App, guard Handler) { app.Group(prefix, guard).Use("/", newMicro()) },
			"domain": func(app *App, guard Handler) { app.Use(prefix, guard); app.Domain(host).Use(prefix, newMicro()) },
			"nested": func(app *App, guard Handler) {
				mid := New()
				mid.Use("/", newMicro())
				app.Use(prefix, guard)
				app.Use(prefix, mid)
			},
			"nested prefix": func(app *App, guard Handler) {
				mid := newApp()
				mid.Use(prefix, guard)
				mid.Use(prefix, newMicro())
				app.Use("/", mid)
			},
			"nested in domain": func(app *App, guard Handler) {
				mid := newApp()
				mid.Use(prefix, guard)
				mid.Use(prefix, newMicro())
				app.Domain(host).Use("/", mid)
			},
		}
		for name, mount := range mounts {
			t.Run(tc.constraint+" "+name, func(t *testing.T) {
				t.Parallel()
				do := func(app *App, method, path string) int {
					return apptest.Do(t, app, apptest.Req(method, path, apptest.WithHost(host))).StatusCode
				}
				get := func(app *App, path string) int { return do(app, MethodGet, path) }

				app := newApp()
				mount(app, guard)
				require.Equal(t, StatusUnauthorized, get(app, "/t/"+tc.covered+"/doe/x"))
				require.Equal(t, StatusNotFound, get(app, "/t/"+tc.outside+"/doe/x"))
				require.Equal(t, StatusNotFound, do(app, MethodHead, "/t/"+tc.outside+"/doe/x"))

				// the mounted app's own parameter keeps the mounted app's constraint
				open := newApp()
				mount(open, func(c Ctx) error { return c.Next() })
				require.Equal(t, StatusOK, get(open, "/t/"+tc.covered+"/doe/"+tc.outside))
				require.Equal(t, StatusNotFound, get(open, "/t/"+tc.outside+"/doe/"+tc.outside))
			})
		}
	}
}

// mountTwoLetterAlpha overrides the built-in alpha with a stricter rule.
type mountTwoLetterAlpha struct{}

func (*mountTwoLetterAlpha) Name() string { return ConstraintAlpha }

func (*mountTwoLetterAlpha) Execute(param string, _ ...string) bool { return len(param) == 2 }

// A constraint one app registers binds none of the routes of the apps mounted beside it, for any method or mount
// order: neither the built-in a guard names nor a name nothing above the guard defines.
func Test_App_Mount_SiblingConstraintOverride(t *testing.T) {
	t.Parallel()
	const host = "api.example.com"
	for _, tc := range []struct {
		constraint CustomConstraint
		guard      string
	}{
		{guard: "/:section<alpha>", constraint: &mountTwoLetterAlpha{}},
		{guard: "/:section<same>", constraint: &mountSameNameConstraint{allowed: "foo"}},
	} {
		for _, mount := range []string{"guarded first", "override first", "nested", "domain"} {
			t.Run(tc.guard+" "+mount, func(t *testing.T) {
				t.Parallel()
				guarded := New()
				guarded.Use(tc.guard, func(c Ctx) error { return c.SendStatus(StatusUnauthorized) })
				guarded.All("/users/export", func(c Ctx) error { return c.SendString("export") })
				override := New()
				override.RegisterCustomConstraint(tc.constraint)
				override.All("/:value<"+tc.constraint.Name()+">", func(c Ctx) error { return c.SendString("ok") })

				mid := New()
				if mount == "override first" {
					mid.Use("/override", override)
					mid.Use("/guarded", guarded)
				} else {
					mid.Use("/guarded", guarded)
					mid.Use("/override", override)
				}
				app := mid
				switch mount {
				case "nested":
					app = New()
					app.Use("/", mid)
				case "domain":
					app = New()
					app.Domain(host).Use("/", mid)
				}
				for _, method := range []string{MethodGet, MethodHead, MethodPost, MethodDelete} {
					resp := apptest.Do(t, app, apptest.Req(method, "/guarded/users/export", apptest.WithHost(host)))
					require.Equal(t, StatusUnauthorized, resp.StatusCode, method)
				}
			})
		}
	}
}

// A mount prefix whose parameter is followed by text matches what middleware on that prefix matches, with or
// without a constraint the mounted app also names: a plain parameter ends where the text ends its segment, a
// constrained one at the first copy of the text.
func Test_App_Mount_PrefixParamBeforeText(t *testing.T) {
	t.Parallel()
	const host = "api.example.com"
	for _, tc := range []struct {
		prefix, path, body string
		guarded            bool
	}{
		{prefix: "/f/:name.json", path: "/f/a.json.json/doe", body: "a.json", guarded: true},
		{prefix: "/f/:name.json", path: "/f/a.json/doe", body: "a", guarded: true},
		{prefix: "/f/:name<same>.json", path: "/f/foo.json/doe", body: "foo", guarded: true},
		{prefix: "/f/:name<same>.json", path: "/f/foo.json.json/doe"},
		{prefix: "/f/:name<same>.json", path: "/f/bar.json/doe"},
	} {
		for _, domain := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s %s domain=%v", tc.prefix, tc.path, domain), func(t *testing.T) {
				t.Parallel()
				newApp := func(guard Handler) *App {
					micro := New()
					micro.RegisterCustomConstraint(&mountAnyConstraint{name: "same"})
					micro.Get("/*", func(c Ctx) error { return c.SendString(c.Params("name")) })
					app := New()
					app.RegisterCustomConstraint(&mountSameNameConstraint{allowed: "foo"})
					app.Use(tc.prefix, guard)
					if domain {
						app.Domain(host).Use(tc.prefix, micro)
					} else {
						app.Use(tc.prefix, micro)
					}
					return app
				}
				req := apptest.Req(MethodGet, tc.path, apptest.WithHost(host))

				guarded := newApp(func(c Ctx) error { return c.SendStatus(StatusUnauthorized) })
				open := newApp(func(c Ctx) error { return c.Next() })
				if !tc.guarded {
					apptest.Expect(t, guarded, req, StatusNotFound)
					apptest.Expect(t, open, req, StatusNotFound)
					return
				}
				apptest.Expect(t, guarded, req, StatusUnauthorized)
				apptest.ExpectBody(t, open, req, StatusOK, tc.body)
			})
		}
	}
}

// Test_App_Mount_PrefixRegexHandlerOfMountedApp verifies that the RegexHandler of
// a mounted app compiles the regex() constraints of the mount prefix too.
func Test_App_Mount_PrefixRegexHandlerOfMountedApp(t *testing.T) {
	t.Parallel()
	const prefix = "/:code<regex(^[a-z]+$)>"
	micro := New(Config{RegexHandler: func(pattern string) *matchOnlyRegexCompiler {
		return &matchOnlyRegexCompiler{re: regexp.MustCompile("(?i)" + pattern)}
	}})
	micro.Get("/doe", func(c Ctx) error { return c.SendString("doe") })

	app := New()
	app.Use(prefix, micro)

	apptest.Expect(t, app, "/foo/doe", StatusOK)
	apptest.Expect(t, app, "/FOO/doe", StatusOK)
	apptest.Expect(t, app, "/123/doe", StatusNotFound)
}

// Test_App_Mount_RouteConstraintOfMountingApp verifies that a name a mounted
// app's route could not resolve is still resolved among the constraints of the
// app it is mounted on, beside the constraint its parameter arrived with.
func Test_App_Mount_RouteConstraintOfMountingApp(t *testing.T) {
	t.Parallel()
	micro := New()
	micro.RegisterCustomConstraint(&mountAnyConstraint{name: ConstraintInt})
	micro.Get("/doe/:own<int;same>", func(c Ctx) error { return c.SendString(c.Params("own")) })

	app := New()
	app.RegisterCustomConstraint(&mountSameNameConstraint{allowed: "foo"})
	app.Use("/john", micro)

	apptest.Expect(t, app, "/john/doe/foo", StatusOK)
	apptest.Expect(t, app, "/john/doe/12", StatusNotFound)
}

// Test_App_Mount_BuiltinConstraintOfMountedApp verifies that a built-in
// constraint in a mounted app's route stays the built-in one when the app it is
// mounted on overrides that name for its own routes.
func Test_App_Mount_BuiltinConstraintOfMountedApp(t *testing.T) {
	t.Parallel()
	handler := func(c Ctx) error { return c.SendString(c.Params("own")) }

	micro := New()
	micro.Get("/doe/:own<int>", handler)

	app := New()
	app.RegisterCustomConstraint(&mountAnyConstraint{name: ConstraintInt})
	app.Get("/jane/:own<int>", handler)
	app.Use("/john", micro)

	apptest.Expect(t, app, "/jane/twelve", StatusOK)
	apptest.Expect(t, app, "/john/doe/12", StatusOK)
	apptest.Expect(t, app, "/john/doe/twelve", StatusNotFound)
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

// routerMountVariant is a way to mount an app: mount puts micro into app, prefix is where micro's routes are served
// and host is the Host that reaches them.
type routerMountVariant struct {
	mount  func(app, micro *App)
	host   apptest.Option
	name   string
	prefix string
}

// routerMountVariants are the two ways a test mounts an app: plainly under /john, and through a domain router under
// /api on api.example.com.
var routerMountVariants = []routerMountVariant{
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
	// The same behind another mount: once flattened, the app that left HEAD out is no longer walked, only its
	// routes' marks are.
	apiHost := apptest.WithHost("api.example.com")
	flattened := []routerMountVariant{
		{name: "inner domain mount", prefix: "/mid/sub", host: apiHost, mount: func(app, micro *App) {
			mid := New()
			mid.Domain("api.example.com").Use("/sub", micro)
			app.Domain("api.example.com").Use("/mid", mid)
		}},
		{name: "plain mount over a domain mount", prefix: "/mid/sub", host: apiHost, mount: func(app, micro *App) {
			inner := New()
			inner.Domain("api.example.com").Use("/sub", micro)
			mid := New()
			mid.Use("/", inner)
			app.Domain("api.example.com").Use("/mid", mid)
		}},
		{name: "started before mounting", prefix: "/mid/sub", host: apiHost, mount: func(app, micro *App) {
			mid := New()
			mid.Use("/sub", micro)
			mid.Handler()
			app.Domain("api.example.com").Use("/mid", mid)
		}},
	}
	for _, tc := range slices.Concat(routerMountVariants, flattened) {
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

// Test_App_Mount_ParentDisableHeadAutoRegister verifies that an app that turned the automatic HEAD
// routes off takes none from the apps mounted into it: a guard it registered with Get has no HEAD
// companion, so a HEAD route flattened from a sub-app would run the handler without it.
func Test_App_Mount_ParentDisableHeadAutoRegister(t *testing.T) {
	t.Parallel()
	// The shared variants, plus the shapes a parent's opt-out has to reach: a group, a nested
	// mount, and apps started before they were mounted.
	host := apptest.WithHost("example.com")
	extra := []routerMountVariant{
		{name: "group", prefix: "/admin", host: host, mount: func(app, sub *App) { app.Group("/admin").Use("/", sub) }},
		{name: "nested", prefix: "/admin", host: host, mount: func(app, sub *App) {
			mid := New()
			mid.Use("/", sub)
			app.Use("/admin", mid)
		}},
		{name: "started before mounting", prefix: "/admin", host: host, mount: func(app, sub *App) {
			sub.Handler()
			app.Use("/admin", sub)
		}},
		{name: "domain started before mounting", prefix: "/admin", host: apptest.WithHost("api.example.com"), mount: func(app, sub *App) {
			sub.Handler()
			app.Domain("api.example.com").Use("/admin", sub)
		}},
	}
	for _, tc := range slices.Concat(routerMountVariants, extra) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New(Config{DisableHeadAutoRegister: true})
			app.Get(tc.prefix+"/*", func(c Ctx) error {
				c.Set("X-Guard", "ran")
				return c.Next()
			})
			sub := New()
			sub.Get("/x", func(c Ctx) error {
				c.Set("X-Secret", "leaked")
				return c.SendString("admin")
			})
			tc.mount(app, sub)

			target := tc.prefix + "/x"
			resp := apptest.ExpectBody(t, app, apptest.Req(MethodGet, target, tc.host), StatusOK, "admin")
			require.Equal(t, "ran", resp.Header.Get("X-Guard"))
			// Twice: the second pass sees the mount already expanded into the parent's stack.
			for range 2 {
				resp = apptest.Expect(t, app, apptest.Req(MethodHead, target, tc.host), StatusMethodNotAllowed)
				require.Empty(t, resp.Header.Get("X-Secret"))
			}
			require.Empty(t, app.Stack()[app.methodInt(MethodHead)])
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
// Neither side of the mount lookup is read as the other: a backslash in the request path is a byte, so `/ad\min` is
// not below "/admin", and a path that spells a parametric mount pattern is not below that mount. Such a request runs
// none of the mounted app's routes, so it gets neither its error handler nor its views.
func Test_App_Mount_PathReadAsPattern(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		mount   string
		path    string
		covered bool
	}{
		{mount: "/admin", path: "/admin/doe", covered: true},
		{mount: "/admin", path: "/ADMIN/doe/", covered: true},
		{mount: "/admin", path: `/ad\min`},
		{mount: "/admin", path: `/\admin`},
		{mount: "/admin", path: `/admin\`},
		{mount: "/admin", path: `/ad\min/doe`},
		{mount: `/v1\:admin`, path: "/v1:admin/doe", covered: true},
		{mount: `/v1\:admin`, path: `/v1\:admin/doe`},
		{mount: "/:id<int>", path: "/:id<int>/doe"},
	} {
		t.Run(tc.mount+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			// no route answers the path, so its error handler is looked up by the path alone
			app := New(Config{ErrorHandler: domainTestErrorHandler(598, "parent")})
			app.Use(tc.mount, domainTestFailingApp(599, "sub", "/doe"))
			if tc.covered {
				require.Equal(t, 599, serveRawTarget(t, app, tc.path).StatusCode())
				return
			}
			require.Equal(t, 598, serveRawTarget(t, app, tc.path).StatusCode())

			// a route of the parent answers it, and renders with the parent's views
			app = New(Config{Views: domainTestEngine(t, "")})
			app.Use(tc.mount, New(Config{Views: domainTestEngine(t, "testdata2")}))
			app.Get("/*", func(c Ctx) error { return c.Render("index.tmpl", Map{"Title": "parent"}) })
			resp := serveRawTarget(t, app, tc.path)
			require.Equal(t, StatusOK, resp.StatusCode())
			require.Equal(t, "<h1>parent</h1>", string(resp.Body()))
		})
	}
}

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

// routerPrefixSpellings are mount prefixes whose syntax the router has to take as written: an escaped colon or
// asterisk, a custom constraint and regular expression constraints. covered is a path the prefix names and outside
// are paths it does not.
var routerPrefixSpellings = []struct {
	prefix, covered string
	outside         []string
}{
	{prefix: `/v1\:internal`, covered: "/v1:internal", outside: []string{"/v1foo", "/v1"}},
	{prefix: `/a\*`, covered: "/a*", outside: []string{"/a", "/a/b"}},
	{prefix: "/even/:n<isEven>", covered: "/even/4", outside: []string{"/even/3", "/even/abc"}},
	{prefix: "/code/:code<regex(^[A-Z]{2}$)>", covered: "/code/US", outside: []string{"/code/us"}},
	{prefix: `/num/:n<regex(^\d+$)>`, covered: "/num/12", outside: []string{"/num/dd"}},
}

// Test_App_Mount_PrefixAsWritten verifies that mounted routes are re-parsed with
// the mount prefix as written, so they answer only where middleware on it runs.
func Test_App_Mount_PrefixAsWritten(t *testing.T) {
	t.Parallel()
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
		for _, tc := range routerPrefixSpellings {
			t.Run(name+" "+tc.prefix, func(t *testing.T) {
				t.Parallel()
				micro := New()
				micro.Get("/doe", func(c Ctx) error { return c.SendString("doe") })

				app := New()
				app.RegisterCustomConstraint(&evenConstraint{})
				mount(app, tc.prefix, func(c Ctx) error { return c.SendStatus(StatusUnauthorized) }, micro)

				routerTestExpect(t, app, tc.covered+"/doe", StatusUnauthorized)
				for _, outside := range tc.outside {
					routerTestExpect(t, app, outside+"/doe", StatusNotFound)
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
				t.Run(fmt.Sprintf("strict=%v/%s/%s", strict, prefix, pattern), func(t *testing.T) {
					t.Parallel()
					users := New()
					users.Use(func(c Ctx) error { return c.SendStatus(StatusUnauthorized) })
					users.Get(pattern, func(c Ctx) error { return c.SendString("users") })
					app := New(Config{StrictRouting: strict})
					app.Use(prefix, users)

					routerTestExpectStatus(t, app, MethodGet, StatusUnauthorized, mounted, mounted+"/", mounted+"/7")
					routerTestExpect(t, app, "/other", StatusNotFound)
				})
			}
		}
	}
}
