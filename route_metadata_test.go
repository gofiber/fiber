package fiber

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/internal/apptest"

	"github.com/gofiber/fiber/v3/internal/deepcopy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func testHandlerOK(c Ctx) error { return c.SendStatus(StatusOK) }

// testAnnotation is the key annotate stores its probe value under.
type testAnnotation struct{}

// otherAnnotation is a second key, for tests that need two values on one route.
type otherAnnotation struct{}

// annotator is what App, Group, the Domain router and a RouteChain registration share: Annotate on the
// routes they registered last.
type annotator interface {
	Annotate(key any, update func(current any) any)
}

// annotate stores value under testAnnotation on the routes target registered last; target is the App, Group,
// Domain router or RouteChain registration a registration returned. It reports a target without Annotate
// through assert, not require, so a goroutine may call it.
func annotate(tb testing.TB, target any, value string) {
	tb.Helper()
	a, ok := target.(annotator)
	if !assert.Truef(tb, ok, "%T has no Annotate", target) {
		return
	}
	a.Annotate(testAnnotation{}, func(any) any { return value })
}

// annotationOf returns the value annotate stored on route, or "" when it has none.
func annotationOf(route *Route) string {
	value, ok := route.Annotation(testAnnotation{}).(string)
	if !ok {
		return ""
	}
	return value
}

func routesFor(app *App, path string) map[string]Route {
	out := make(map[string]Route)
	routes := app.GetRoutes()
	for i := range routes {
		if routes[i].Path == path && !routes[i].IsMiddleware() {
			out[routes[i].Method] = routes[i]
		}
	}
	return out
}

func Test_Route_Annotations(t *testing.T) {
	t.Parallel()
	t.Run("every kind of registration", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			register func(app *App) any
			name     string
			path     string
		}{
			{name: "app", path: "/a", register: func(app *App) any { return app.Get("/a", testHandlerOK) }},
			{name: "group", path: "/g/a", register: func(app *App) any { return app.Group("/g").Get("/a", testHandlerOK) }},
			{name: "domain", path: "/a", register: func(app *App) any { return app.Domain("api.example.com").Get("/a", testHandlerOK) }},
			{name: "route chain", path: "/a", register: func(app *App) any { return app.RouteChain("/a").Get(testHandlerOK) }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				app := New()
				annotate(t, tc.register(app), tc.name)
				route := findRoute(t, app, MethodGet, tc.path)
				require.Equal(t, tc.name, annotationOf(&route))
			})
		}
	})

	t.Run("a mounted sub app keeps what it was annotated with", func(t *testing.T) {
		t.Parallel()
		sub := New()
		annotate(t, sub.Get("/leaf", testHandlerOK), "sub")
		app := New()
		app.Use("/api", sub)
		app.Handler()
		route := findRoute(t, app, MethodGet, "/api/leaf")
		require.Equal(t, "sub", annotationOf(&route))
	})

	t.Run("update receives the current value and copies keep theirs", func(t *testing.T) {
		t.Parallel()
		app := New()
		a, ok := app.Get("/a", testHandlerOK).(annotator)
		require.True(t, ok)
		var first, other any = "unset", "unset"
		a.Annotate(testAnnotation{}, func(current any) any { first = current; return "first" })
		require.Nil(t, first, "a key never stored reads as nil")
		snapshot := findRoute(t, app, MethodGet, "/a")

		rev := app.RoutesRevision()
		a.Annotate(testAnnotation{}, func(current any) any { first = current; return "second" })
		a.Annotate(otherAnnotation{}, func(current any) any { other = current; return "other" })
		require.Equal(t, "first", first)
		require.Nil(t, other)
		require.Greater(t, app.RoutesRevision(), rev)

		route := findRoute(t, app, MethodGet, "/a")
		require.Equal(t, "second", annotationOf(&route))
		require.Equal(t, "other", route.Annotation(otherAnnotation{}), "another key keeps the earlier ones")
		require.Equal(t, "first", annotationOf(&snapshot), "a copy taken earlier keeps its value")
		require.Nil(t, snapshot.Annotation(otherAnnotation{}))
	})

	t.Run("a router without a registration changes nothing", func(t *testing.T) {
		t.Parallel()
		require.NotPanics(t, func() { annotate(t, New(), "nothing registered") })
		app := New()
		app.Get("/a", testHandlerOK)
		rev := app.RoutesRevision()
		annotate(t, app.Group("/g"), "group")
		annotate(t, app.Domain("d.example.com"), "domain")
		annotate(t, app.RouteChain("/c"), "chain")
		route := findRoute(t, app, MethodGet, "/a")
		require.Empty(t, annotationOf(&route))
		require.Equal(t, rev, app.RoutesRevision())
	})

	t.Run("auto-HEAD twins carry no annotation", func(t *testing.T) {
		t.Parallel()
		for name, beforeTwin := range map[string]bool{"annotated before the twin exists": true, "annotated after": false} {
			app := New()
			route := app.Get("/h", testHandlerOK)
			if beforeTwin {
				annotate(t, route, "get")
			}
			app.RebuildTree()
			if !beforeTwin {
				annotate(t, route, "get")
			}
			get := findRoute(t, app, MethodGet, "/h")
			require.Equal(t, "get", annotationOf(&get), name)
			head := findRoute(t, app, MethodHead, "/h")
			require.True(t, head.IsAutoHead(), name)
			require.Empty(t, annotationOf(&head), name)
		}
	})
}

func Test_RouteDocs_MultiMethodRegistration(t *testing.T) {
	t.Parallel()
	app := New()
	annotate(t, app.Add([]string{MethodPut, MethodPost}, "/multi", testHandlerOK), "multi")

	routes := routesFor(app, "/multi")
	require.Len(t, routes, 2)
	for method, route := range routes {
		require.Equal(t, "multi", annotationOf(&route), "method %s missing the annotation", method)
	}
}

func Test_RouteDocs_AllMethodsRegistration(t *testing.T) {
	t.Parallel()
	app := New()
	annotate(t, app.All("/everything", testHandlerOK), "all")

	annotated := 0
	routes := app.GetRoutes()
	for i := range routes {
		if routes[i].Path == "/everything" && annotationOf(&routes[i]) == "all" {
			annotated++
		}
	}
	require.Equal(t, len(DefaultMethods), annotated)
}

func Test_RouteDocs_UseDoesNotLeakToEndpoints(t *testing.T) {
	t.Parallel()
	app := New()
	app.Post("/shared", testHandlerOK)
	app.Use("/shared", func(c Ctx) error { return c.Next() })
	annotate(t, app, "middleware")

	routes := app.GetRoutes()
	for i := range routes {
		route := &routes[i]
		if route.Path != "/shared" {
			continue
		}
		if route.IsMiddleware() {
			require.Equal(t, "middleware", annotationOf(route), "middleware route should carry the annotation")
		} else {
			require.Empty(t, annotationOf(route), "annotation leaked onto %s %s", route.Method, route.Path)
		}
	}
}

func Test_RoutesRevision_Bumps(t *testing.T) {
	t.Parallel()
	app := New()
	before := app.RoutesRevision()
	app.Get("/rev", testHandlerOK)
	afterRegister := app.RoutesRevision()
	require.Greater(t, afterRegister, before, "registration must bump the revision")

	annotate(t, app, "rev")
	afterAnnotate := app.RoutesRevision()
	require.Greater(t, afterAnnotate, afterRegister, "an annotation must bump the revision")

	app.RemoveRoute("/rev", MethodGet)
	require.Greater(t, app.RoutesRevision(), afterAnnotate, "removal must bump the revision")
}

func fullyPopulatedRoute() *Route {
	return &Route{
		Method:      MethodPost,
		Name:        "name",
		Path:        "/full/:id",
		Handlers:    []Handler{testHandlerOK},
		Params:      []string{"id"},
		group:       &Group{Prefix: "/full"},
		annotations: map[any]any{testAnnotation{}: "annotated"},
	}
}

func Test_CopyRoute_Complete(t *testing.T) {
	t.Parallel()
	app := New()
	original := fullyPopulatedRoute()

	value := reflect.ValueOf(*original)
	typ := value.Type()
	for i := range typ.NumField() {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		require.Falsef(t, value.Field(i).IsZero(),
			"Route field %q is zero in fullyPopulatedRoute(); update the fixture AND copyRoute", field.Name)
	}

	clone := app.copyRoute(original)
	require.Nil(t, clone.group, "copyRoute drops the group")
	require.Equal(t, "annotated", annotationOf(clone))
	require.Len(t, clone.Handlers, len(original.Handlers))
	origCmp, cloneCmp := *original, *clone
	origCmp.Handlers, cloneCmp.Handlers = nil, nil
	origCmp.group = nil
	require.Equal(t, origCmp, cloneCmp)

	base := app.copyRouteBase(original)
	require.Nil(t, base.Annotation(testAnnotation{}), "copyRouteBase drops the annotations")
	require.Equal(t, original.Path, base.Path)
}

// boundedTimeout is how long apptest.Bounded waits before calling a deadlock.
const boundedTimeout = 10 * time.Second

func Test_Shutdown_WithInflightGetRoutes(t *testing.T) {
	t.Parallel()
	app := New()

	started := make(chan struct{})
	proceed := make(chan struct{})
	var once sync.Once
	app.Get("/routes", func(c Ctx) error {
		once.Do(func() { close(started) })
		<-proceed
		return c.SendString(strconv.Itoa(len(c.App().GetRoutes())))
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		assert.NoError(t, app.Listener(ln, ListenConfig{DisableStartupMessage: true}))
	}()

	conn, err := ln.Dial()
	require.NoError(t, err)
	defer func() { assert.NoError(t, conn.Close()) }()
	_, err = conn.Write([]byte("GET /routes HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	require.NoError(t, err)

	apptest.Recv(t, started, boundedTimeout, "the request never reached the handler")

	shuttingDown := make(chan struct{})
	app.Hooks().OnPreShutdown(func() error { close(shuttingDown); return nil })
	done := make(chan error, 1)
	go func() { done <- app.Shutdown() }()

	// Shutdown is now running while the handler is still in flight.
	apptest.Recv(t, shuttingDown, boundedTimeout, "shutdown never started")
	close(proceed)
	require.NoError(t, apptest.Recv(t, done, boundedTimeout, "shutdown deadlocked on in-flight GetRoutes call"))
}

func Test_Hooks_MayCallLockingAppMethods(t *testing.T) {
	t.Parallel()
	app := New()

	var onRouteSaw, onNameSaw int
	app.Hooks().OnRoute(func(Route) error {
		onRouteSaw = len(app.GetRoutes())
		return nil
	})
	app.Hooks().OnName(func(Route) error {
		onNameSaw = len(app.GetRoutes())
		return nil
	})

	apptest.Bounded(t, boundedTimeout, "route registration with hooks that call GetRoutes", func() {
		app.Get("/x", testHandlerOK).Name("x")
	})
	require.Equal(t, 1, onRouteSaw, "the hook saw the one registered route")
	require.Equal(t, 1, onNameSaw)
}

func Test_Mount_StartupConcurrentSubAppDocHelpers(t *testing.T) {
	t.Parallel()
	sub := New()
	sub.Get("/x", testHandlerOK)

	app := New()
	app.Use("/api", sub)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			annotate(t, sub.Get(fmt.Sprintf("/x%d", i), testHandlerOK), "ok")
		}
	})

	apptest.Expect(t, app, "/api/x", StatusOK)
	close(stop)
	wg.Wait()
}

func Test_ScopedDocHelpers_TargetOwnRegistration(t *testing.T) {
	t.Parallel()
	// Each scope registers one route, the app registers "/other" after it, and the scope then annotates its own.
	scopes := map[string]func(t *testing.T, app *App) string{
		"RouteChain": func(t *testing.T, app *App) string {
			t.Helper()
			chain := app.RouteChain("/users")
			chain.Get(testHandlerOK)
			app.Get("/other", testHandlerOK)
			annotate(t, chain.Name("own"), "mine")
			return "/users"
		},
		"Group": func(t *testing.T, app *App) string {
			t.Helper()
			api := app.Group("/api")
			api.Get("/a", testHandlerOK)
			app.Get("/other", testHandlerOK)
			annotate(t, api.Name("own"), "mine")
			return "/api/a"
		},
		"Domain": func(t *testing.T, app *App) string {
			t.Helper()
			d := app.Domain("api.example.com")
			d.Get("/a", testHandlerOK)
			app.Get("/other", testHandlerOK)
			annotate(t, d.Name("own"), "mine")
			return "/a"
		},
	}
	for name, register := range scopes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := New()
			path := register(t, app)
			own := findRoute(t, app, MethodGet, path)
			require.Equal(t, "own", own.Name)
			require.Equal(t, "mine", annotationOf(&own))
			other := findRoute(t, app, MethodGet, "/other")
			require.Empty(t, other.Name)
			require.Empty(t, annotationOf(&other))
			require.Equal(t, path, app.GetRoute("own").Path)
		})
	}
}

func Test_Domain_SamePathKeepsSeparateRoutes(t *testing.T) {
	t.Parallel()
	app := New()
	annotate(t, app.Domain("a.example.com").Get("/x", func(c Ctx) error { return c.SendString("host a") }), "A docs")
	annotate(t, app.Domain("b.example.com").Get("/x", func(c Ctx) error { return c.SendString("host b") }), "B docs")

	var annotations []string
	for _, route := range app.GetRoutes() {
		if route.Path == "/x" && route.Method == MethodGet && !route.IsAutoHead() {
			annotations = append(annotations, annotationOf(&route))
		}
	}
	require.ElementsMatch(t, []string{"A docs", "B docs"}, annotations)

	for host, want := range map[string]string{"a.example.com": "host a", "b.example.com": "host b"} {
		apptest.ExpectBody(t, app, apptest.Req(MethodGet, "/x", apptest.WithHost(host)), StatusOK, want)
	}
}

func Test_DocCursor_InvalidatedByRemovalAndStartup(t *testing.T) {
	t.Parallel()
	t.Run("RemoveRoute", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/a", testHandlerOK)
		app.RemoveRoute("/a", MethodGet)
		rev := app.RoutesRevision()

		annotate(t, app, "dangling")
		require.Equal(t, rev, app.RoutesRevision())
	})

	t.Run("AfterStartup", func(t *testing.T) {
		t.Parallel()
		app := New()
		annotate(t, app.Get("/a", testHandlerOK), "sum-a")
		app.Post("/c", testHandlerOK)

		apptest.Expect(t, app, "/a", StatusOK)

		annotate(t, app, "late")
		a, c := routesFor(app, "/a")[MethodGet], routesFor(app, "/c")[MethodPost]
		require.Equal(t, "sum-a", annotationOf(&a))
		require.Equal(t, "late", annotationOf(&c))
	})
}

func Test_MountRegistration_DocHelpersAreNoOps(t *testing.T) {
	t.Parallel()
	sub := New()
	sub.Get("/x", testHandlerOK)

	app := New()
	rev := app.RoutesRevision()
	annotate(t, app.Use("/api", sub), "mount")
	require.Greater(t, app.RoutesRevision(), rev)

	for _, route := range app.GetRoutes() {
		require.NotEqual(t, "mount", annotationOf(&route), route.Path)
	}

	apptest.Expect(t, app, "/api/x", StatusOK)

	for _, route := range app.GetRoutes() {
		require.NotEqual(t, "mount", annotationOf(&route), route.Path)
	}
}

func Test_AutoHeadTwins_CarryNoDocumentation(t *testing.T) {
	t.Parallel()
	app := New()
	annotate(t, app.Get("/doc", testHandlerOK), "get")

	apptest.Expect(t, app, apptest.Req(MethodHead, "/doc"), StatusOK)

	twin, found := Route{}, false
	for _, route := range app.GetRoutes() {
		if route.Method == MethodHead && route.Path == "/doc" && route.IsAutoHead() {
			twin, found = route, true
		}
	}
	require.True(t, found)
	require.Nil(t, twin.annotations)
}

func Test_Hooks_OutsideRouterLock(t *testing.T) {
	t.Parallel()
	t.Run("OnGroupName", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Hooks().OnGroupName(func(Group) error { _ = app.GetRoutes(); return nil })
		apptest.Bounded(t, boundedTimeout, "OnGroupName", func() { app.Group("/x").Name("grp") })
	})

	t.Run("OnMountThroughDomain", func(t *testing.T) {
		t.Parallel()
		app := New()
		sub := New()
		sub.Get("/a", testHandlerOK)
		sub.Hooks().OnMount(func(*App) error { _ = sub.GetRoutes(); return nil })
		apptest.Bounded(t, boundedTimeout, "OnMount", func() { app.Domain("api.example.com").Use("/s", sub) })
	})

	t.Run("OnPreShutdown", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/a", testHandlerOK)
		app.Hooks().OnPreShutdown(func() error { _ = app.GetRoutes(); return nil })

		listening := make(chan struct{})
		app.Hooks().OnListen(func(ListenData) error { close(listening); return nil })
		ln := fasthttputil.NewInmemoryListener()
		go func() {
			if err := app.Listener(ln, ListenConfig{DisableStartupMessage: true}); err != nil {
				panic(err)
			}
		}()
		<-listening
		apptest.Bounded(t, boundedTimeout, "OnPreShutdown", func() { require.NoError(t, app.Shutdown()) })
	})
}

func Test_ScopedHelpers_AfterMergedRegistration(t *testing.T) {
	t.Parallel()
	app := New()
	api := app.Group("/api")
	api.Get("/users", testHandlerOK)
	api.Get("/users", testHandlerOK)
	app.Get("/health", testHandlerOK)

	rev := app.RoutesRevision()
	annotate(t, api, "list users")

	route := routesFor(app, "/api/users")[MethodGet]
	require.Equal(t, "list users", annotationOf(&route))
	require.Greater(t, app.RoutesRevision(), rev)
}

func Test_Group_SubGroupDoesNotStealParentCursor(t *testing.T) {
	t.Parallel()
	app := New()
	v1 := app.Group("/v1")
	v1.Get("/users", testHandlerOK)
	v1.Group("/sub", func(c Ctx) error { return c.Next() })
	annotate(t, v1, "list users")

	route := routesFor(app, "/v1/users")[MethodGet]
	require.Equal(t, "list users", annotationOf(&route))
}

func Test_RemoveRoute_HandlerCountPerDomain(t *testing.T) {
	t.Parallel()
	app := New()
	app.Domain("a.example").Use("/x", func(c Ctx) error { return c.Next() })
	app.Domain("b.example").Use("/x", func(c Ctx) error { return c.Next() })
	require.Equal(t, uint32(2), app.HandlersCount())

	app.RemoveRoute("/x")
	require.Equal(t, uint32(0), app.HandlersCount())
}

func Test_ScopedHelpers_SurviveCompression(t *testing.T) {
	t.Parallel()
	app := New()
	g1 := app.Group("/g")
	g2 := app.Group("/g")
	g1.Get("/same", testHandlerOK)
	g2.Get("/same", testHandlerOK)

	annotate(t, g1, "from-g1")
	a, ok := g2.(annotator)
	require.True(t, ok)
	a.Annotate(otherAnnotation{}, func(any) any { return "from-g2" })

	var found *Route
	for _, r := range app.GetRoutes() {
		if r.Path == "/g/same" && r.Method == MethodGet {
			route := r
			found = &route
		}
	}
	require.NotNil(t, found)
	require.Equal(t, "from-g1", annotationOf(found), "the merged-away scope's annotation was dropped")
	require.Equal(t, "from-g2", found.Annotation(otherAnnotation{}))
}

func Test_GroupMount_DoesNotRetargetPreviousRoute(t *testing.T) {
	t.Parallel()
	app := New()
	grp := app.Group("/v1")
	annotate(t, grp.Get("/health", testHandlerOK), "health")

	sub := New()
	sub.Get("/x", testHandlerOK)
	grp.Use("/api", sub)
	annotate(t, grp, "mounted")

	for _, r := range app.GetRoutes() {
		if r.Path == "/v1/health" {
			require.Equal(t, "health", annotationOf(&r), "the mount retargeted the previous route")
		}
	}
}

func Test_CopyAnyValue_NamedMapCycle(t *testing.T) {
	t.Parallel()
	type metadata map[string]any
	cyclic := metadata{}
	cyclic["self"] = cyclic
	require.NotPanics(t, func() { _ = deepcopy.Value(cyclic) })

	type list []any
	cyclicList := list{nil}
	cyclicList[0] = cyclicList
	require.NotPanics(t, func() { _ = deepcopy.Value(cyclicList) })
}

func findRoute(t *testing.T, app *App, method, path string) Route {
	t.Helper()
	routes := app.GetRoutes()
	for i := range routes {
		if routes[i].Method == method && routes[i].Path == path {
			return routes[i]
		}
	}
	require.Failf(t, "route not found", "%s %s", method, path)
	return Route{}
}

func Test_ScopedHelpers_MergeReachesOnlyOwnEntries(t *testing.T) {
	t.Parallel()
	t.Run("earlier scope keeps its unmerged entries", func(t *testing.T) {
		t.Parallel()
		app := New()
		api := app.Group("/api")
		api.Add([]string{MethodGet, MethodPost}, "/items", testHandlerOK)
		app.Post("/api/items", testHandlerOK)
		annotate(t, api, "Items")

		get, post := findRoute(t, app, MethodGet, "/api/items"), findRoute(t, app, MethodPost, "/api/items")
		require.Equal(t, "Items", annotationOf(&get))
		require.Equal(t, "Items", annotationOf(&post))
	})

	t.Run("earlier scope never reaches the later registration", func(t *testing.T) {
		t.Parallel()
		app := New()
		api := app.Group("/api")
		api.Get("/a", testHandlerOK)
		app.Add([]string{MethodGet, MethodPost}, "/api/a", testHandlerOK)
		annotate(t, api, "list")

		get, post := findRoute(t, app, MethodGet, "/api/a"), findRoute(t, app, MethodPost, "/api/a")
		require.Equal(t, "list", annotationOf(&get))
		require.Empty(t, annotationOf(&post), "the POST route was never registered by the group")
	})

	t.Run("removed entries are forgotten", func(t *testing.T) {
		t.Parallel()
		app := New()
		g1 := app.Group("/g")
		g2 := app.Group("/g")
		g1.Get("/same", testHandlerOK)
		g2.Get("/same", testHandlerOK)
		app.RemoveRoute("/g/same", MethodGet)
		require.NotPanics(t, func() { annotate(t, g1, "gone") })
		app.mutex.Lock()
		defer app.mutex.Unlock()
		require.Empty(t, app.regEntries)
	})
}

func Test_ConcurrentRegistrations_KeepBatchesWhole(t *testing.T) {
	t.Parallel()
	for range 50 {
		app := New()
		var wg sync.WaitGroup
		for _, prefix := range []string{"/a", "/b"} {
			wg.Go(func() {
				grp := app.Group(prefix)
				grp.Add([]string{MethodGet, MethodPost}, "/x", testHandlerOK)
				annotate(t, grp, prefix)
			})
		}
		wg.Wait()

		for _, prefix := range []string{"/a", "/b"} {
			get, post := findRoute(t, app, MethodGet, prefix+"/x"), findRoute(t, app, MethodPost, prefix+"/x")
			require.Equal(t, prefix, annotationOf(&get))
			require.Equal(t, prefix, annotationOf(&post))
		}
	}
}

func Test_AutoHeadTwin_OutsideRegistrations(t *testing.T) {
	t.Parallel()
	app := New()
	api := app.Group("/api")
	api.Get("/a", testHandlerOK)
	app.Get("/b", testHandlerOK)

	apptest.Do(t, app, "/api/a")

	annotate(t, api, "A")

	get := findRoute(t, app, MethodGet, "/api/a")
	require.Equal(t, "A", annotationOf(&get))
	head := findRoute(t, app, MethodHead, "/api/a")
	require.True(t, head.IsAutoHead())
	require.Empty(t, annotationOf(&head))
}

func Test_Name_ReachesAutoHeadTwin(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/a", testHandlerOK)
	apptest.Do(t, app, "/a")

	app.Name("users")
	require.Equal(t, "users", findRoute(t, app, MethodGet, "/a").Name)
	require.Equal(t, "users", findRoute(t, app, MethodHead, "/a").Name)

	grp := app.Group("/g")
	grp.Get("/b", testHandlerOK)
	apptest.Do(t, app, "/g/b")
	grp.Name("things")
	require.Equal(t, "things", findRoute(t, app, MethodHead, "/g/b").Name)
}

func Test_Name_NoHookForMountPlaceholder(t *testing.T) {
	t.Parallel()
	app := New()
	var fired []string
	app.Hooks().OnName(func(r Route) error {
		fired = append(fired, r.Method+" "+r.Path+" "+r.Name)
		return nil
	})
	app.Get("/a", testHandlerOK)
	app.Use("/sub", New()).Name("sub")

	require.Empty(t, fired)
	require.Empty(t, findRoute(t, app, MethodGet, "/a").Name)

	app.Get("/c", testHandlerOK).Name("c")
	require.Equal(t, []string{"GET /c c"}, fired)
}

func Test_GroupMiddleware_ReachableFromGroup(t *testing.T) {
	t.Parallel()
	app := New()
	annotate(t, app.Group("/api", testHandlerOK), "group middleware")
	mw := findRoute(t, app, MethodGet, "/api")
	require.Equal(t, "group middleware", annotationOf(&mw))

	d := app.Domain("api.example.com")
	d.Get("/a", testHandlerOK)
	sub := d.Group("/v1", testHandlerOK)
	annotate(t, d, "list")
	annotate(t, sub, "mw")

	list, subMW := findRoute(t, app, MethodGet, "/a"), findRoute(t, app, MethodPost, "/v1")
	require.Equal(t, "list", annotationOf(&list))
	require.Equal(t, "mw", annotationOf(&subMW))
}

func Test_LockingLookups_FromCallbacks(t *testing.T) {
	t.Parallel()
	t.Run("RemoveRouteFunc matcher", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/old", testHandlerOK).Name("old")
		app.Get("/keep", testHandlerOK)

		apptest.Bounded(t, boundedTimeout, "RemoveRouteFunc with a matcher that calls GetRoute", func() {
			app.RemoveRouteFunc(func(r *Route) bool { return r.Path == app.GetRoute("old").Path })
		})
		require.Len(t, app.GetRoutes(true), 1)
	})

	t.Run("sub-app OnRoute hook during parent startup", func(t *testing.T) {
		t.Parallel()
		parent := New()
		sub := New()
		sub.Get("/x", testHandlerOK)
		var seen int
		sub.Hooks().OnRoute(func(Route) error {
			seen = len(parent.GetRoutes())
			return nil
		})
		parent.Use("/api", sub)

		apptest.Bounded(t, boundedTimeout, "startup with a sub-app hook that calls the parent's GetRoutes", func() {
			resp, err := parent.Test(httptest.NewRequest(MethodHead, "/api/x", http.NoBody))
			if assert.NoError(t, err) {
				assert.Equal(t, StatusOK, resp.StatusCode)
			}
		})
		require.Positive(t, seen)
	})
}

func Test_NamedRoute_SharesSnapshotForURLs(t *testing.T) {
	t.Parallel()
	app := New()
	annotate(t, app.Get("/users/:id", testHandlerOK).Name("user"), "user")

	route := app.namedRoute("user")
	require.NotNil(t, route)
	require.Equal(t, "/users/:id", route.Path)
	require.Equal(t, "user", annotationOf(route))
	require.Nil(t, app.namedRoute("missing"))

	app.Get("/go", func(c Ctx) error { return c.Redirect().Route("user", RedirectConfig{Params: Map{"id": "7"}}) })
	resp := apptest.Do(t, app, "/go")
	require.Equal(t, "/users/7", resp.Header.Get(HeaderLocation))
}

func Test_DuplicateMethodRegistration_IndexedOnce(t *testing.T) {
	t.Parallel()
	app := New()
	a, ok := app.Add([]string{MethodGet, MethodGet}, "/d", testHandlerOK).(annotator)
	require.True(t, ok)
	a.Annotate(testAnnotation{}, func(current any) any {
		count, isCount := current.(int)
		if !isCount {
			count = 0
		}
		return count + 1
	})

	route := findRoute(t, app, MethodGet, "/d")
	require.Equal(t, 1, route.Annotation(testAnnotation{}), "the update ran once for the one route")
}

func Test_RemoveRouteFunc_MatchesAgainstSnapshot(t *testing.T) {
	t.Parallel()
	app := New()
	annotate(t, app.Get("/a", testHandlerOK).Name("keep"), "original")

	app.RemoveRouteFunc(func(r *Route) bool {
		r.annotations = nil
		r.Name = "renamed"
		return false
	})

	route := findRoute(t, app, MethodGet, "/a")
	require.Equal(t, "original", annotationOf(&route))
	require.Equal(t, "keep", route.Name)
}

func Test_AutoHeadPrune_IsDomainScoped(t *testing.T) {
	t.Parallel()
	app := New()
	app.Domain("a.example").Get("/x", testHandlerOK)
	app.Domain("b.example").Get("/x", testHandlerOK)
	app.RebuildTree()

	app.Domain("a.example").Head("/x", testHandlerOK)
	app.RebuildTree()

	twins := map[string]int{}
	explicit := map[string]int{}
	for _, route := range app.stack[app.methodInt(MethodHead)] {
		if route.path != "/x" {
			continue
		}
		if route.IsAutoHead() {
			twins[route.Domain()]++
			continue
		}
		explicit[route.Domain()]++
	}
	require.Equal(t, map[string]int{"b.example": 1}, twins)
	require.Equal(t, map[string]int{"a.example": 1}, explicit)
}

func Test_Route_GroupIdentity(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/plain", testHandlerOK)
	app.Group("/api").Name("api.").Group("/users").Name("users.").Get("/:id", testHandlerOK)
	app.Group("/anon").Get("/x", testHandlerOK)

	plain := findRoute(t, app, MethodGet, "/plain")
	require.Empty(t, plain.GroupPrefix())
	require.Empty(t, plain.GroupName())

	user := findRoute(t, app, MethodGet, "/api/users/:id")
	require.Equal(t, "/api/users", user.GroupPrefix())
	require.Equal(t, "api.users.", user.GroupName())

	anon := findRoute(t, app, MethodGet, "/anon/x")
	require.Equal(t, "/anon", anon.GroupPrefix())
	require.Empty(t, anon.GroupName())
}

func Test_Route_GroupIdentity_MergedRegistrationTakesLatestGroup(t *testing.T) {
	t.Parallel()
	app := New()
	app.Group("/x").Name("first.").Get("/y", testHandlerOK)
	app.Group("/x").Name("second.").Get("/y", testHandlerOK)

	routes := app.GetRoutes()
	var merged []Route
	for i := range routes {
		if routes[i].Method == MethodGet && routes[i].Path == "/x/y" {
			merged = append(merged, routes[i])
		}
	}
	require.Len(t, merged, 1)
	require.Equal(t, "second.", merged[0].GroupName())
	require.Equal(t, "/x", merged[0].GroupPrefix())
}

func Test_Domain_UseRejectsInvalidHandler(t *testing.T) {
	t.Parallel()
	require.Panics(t, func() { New().Domain("api.example.com").Use(12345) })
}

func Test_Route_Domain(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/plain", testHandlerOK)
	app.Domain("api.example").Get("/scoped", testHandlerOK)
	plain := findRoute(t, app, MethodGet, "/plain")
	scoped := findRoute(t, app, MethodGet, "/scoped")
	require.Empty(t, plain.Domain())
	require.Equal(t, "api.example", scoped.Domain())
}

func handlerPointer(h Handler) uintptr { return reflect.ValueOf(h).Pointer() }

func Test_Route_InnerHandlers(t *testing.T) {
	t.Parallel()
	other := func(c Ctx) error { return c.Next() }

	t.Run("a plain route has one chain", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/plain", testHandlerOK)
		route := findRoute(t, app, MethodGet, "/plain")
		require.Len(t, route.InnerHandlers(), len(route.Handlers))
		require.Equal(t, handlerPointer(testHandlerOK), handlerPointer(route.InnerHandlers()[0]))
	})

	t.Run("a domain route keeps the registered handlers", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("api.example").Get("/x", other, testHandlerOK)
		route := findRoute(t, app, MethodGet, "/x")
		require.Len(t, route.Handlers, 2)
		inner := route.InnerHandlers()
		require.Len(t, inner, 2)
		require.Equal(t, handlerPointer(other), handlerPointer(inner[0]))
		require.Equal(t, handlerPointer(testHandlerOK), handlerPointer(inner[1]))
		require.NotEqual(t, handlerPointer(testHandlerOK), handlerPointer(route.Handlers[1]))
	})

	t.Run("a route chain, middleware and a group on a domain", func(t *testing.T) {
		t.Parallel()
		app := New()
		api := app.Domain("api.example")
		api.RouteChain("/chain").Get(testHandlerOK)
		api.Use("/mw", other)
		api.Group("/g", other)

		chain := findRoute(t, app, MethodGet, "/chain")
		require.Equal(t, handlerPointer(testHandlerOK), handlerPointer(chain.InnerHandlers()[0]))
		var use, group int
		for _, route := range app.GetRoutes() {
			if route.Method != MethodGet || !route.IsMiddleware() {
				continue
			}
			switch route.Path {
			case "/mw":
				use++
				require.Equal(t, handlerPointer(other), handlerPointer(route.InnerHandlers()[0]))
			case "/g":
				group++
				require.Equal(t, handlerPointer(other), handlerPointer(route.InnerHandlers()[0]))
			}
		}
		require.Equal(t, 1, use)
		require.Equal(t, 1, group)
	})

	t.Run("merged registrations append to both chains", func(t *testing.T) {
		t.Parallel()
		app := New()
		api := app.Domain("api.example")
		api.Get("/m", other)
		api.Get("/m", testHandlerOK)
		route := findRoute(t, app, MethodGet, "/m")
		require.Len(t, route.Handlers, 2)
		require.Len(t, route.InnerHandlers(), 2)
		require.Equal(t, handlerPointer(other), handlerPointer(route.InnerHandlers()[0]))
		require.Equal(t, handlerPointer(testHandlerOK), handlerPointer(route.InnerHandlers()[1]))
	})

	t.Run("an OnRoute hook sees the registered handlers", func(t *testing.T) {
		t.Parallel()
		app := New()
		var seen [][]uintptr
		app.Hooks().OnRoute(func(r Route) error {
			pointers := make([]uintptr, 0, len(r.InnerHandlers()))
			for _, h := range r.InnerHandlers() {
				pointers = append(pointers, handlerPointer(h))
			}
			seen = append(seen, pointers)
			return nil
		})
		api := app.Domain("api.example")
		api.Get("/h", other)
		// The second registration merges into the first, so its hook sees both chains.
		api.Get("/h", testHandlerOK)
		api.RouteChain("/c").Get(testHandlerOK)
		require.Equal(t, [][]uintptr{
			{handlerPointer(other)},
			{handlerPointer(other), handlerPointer(testHandlerOK)},
			{handlerPointer(testHandlerOK)},
		}, seen)
	})

	t.Run("a mounted sub app keeps its own handlers", func(t *testing.T) {
		t.Parallel()
		sub := New()
		sub.Get("/leaf", testHandlerOK)
		app := New()
		app.Domain("api.example").Use("/api", sub)
		apptest.Do(t, app, "/api/leaf")
		route := findRoute(t, app, MethodGet, "/api/leaf")
		require.Equal(t, handlerPointer(testHandlerOK), handlerPointer(route.InnerHandlers()[0]))
		require.NotEqual(t, handlerPointer(testHandlerOK), handlerPointer(route.Handlers[0]))
	})
}

const benchRouteCount = 100

func benchRoutes(b *testing.B) *App {
	b.Helper()
	app := New()
	for i := range benchRouteCount {
		path := "/bench/" + strconv.Itoa(i) + "/:id"
		route := app.Get(path, testHandlerOK).Name("bench" + strconv.Itoa(i))
		if i%4 == 0 {
			annotate(b, route, "summary")
		}
	}
	return app
}

func Benchmark_App_GetRoute_Undocumented(b *testing.B) {
	app := benchRoutes(b)
	b.ReportAllocs()

	var route Route
	for b.Loop() {
		route = app.GetRoute("bench99")
	}
	require.Equal(b, "/bench/99/:id", route.Path)
}

func Benchmark_App_GetRoute_Documented(b *testing.B) {
	app := benchRoutes(b)
	b.ReportAllocs()

	var route Route
	for b.Loop() {
		route = app.GetRoute("bench96")
	}
	require.Equal(b, "summary", annotationOf(&route))
}

func Benchmark_App_GetRoutes(b *testing.B) {
	app := benchRoutes(b)
	b.ReportAllocs()

	var routes []Route
	for b.Loop() {
		routes = app.GetRoutes(true)
	}
	require.NotEmpty(b, routes)
}

func Benchmark_App_namedRoute(b *testing.B) {
	app := benchRoutes(b)
	b.ReportAllocs()

	var route *Route
	for b.Loop() {
		route = app.namedRoute("bench96")
	}
	require.Equal(b, "/bench/96/:id", route.Path)
}

func Benchmark_App_copyRoute_Documented(b *testing.B) {
	app := New()
	original := fullyPopulatedRoute()
	b.ReportAllocs()

	var copied Route
	for b.Loop() {
		app.copyRouteInto(&copied, original)
	}
	require.Equal(b, "annotated", annotationOf(&copied))
}

func Benchmark_Route_Domain_Dispatch(b *testing.B) {
	app := New()
	app.Domain("api.example").Get("/x", func(c Ctx) error { return c.SendStatus(StatusNoContent) })
	app.startupProcess()
	h := app.Handler()
	b.ReportAllocs()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	fctx.Request.SetRequestURI("/x")
	fctx.Request.Header.SetHost("api.example")

	for b.Loop() {
		h(fctx)
	}
	require.Equal(b, StatusNoContent, fctx.Response.StatusCode())
}

// scanNamedRoute is the linear-scan reference the name index must agree with.
func scanNamedRoute(app *App, name string) (Route, bool) {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	for _, routes := range app.stack {
		for _, route := range routes {
			if route.Name == name {
				var found Route
				app.copyRouteInto(&found, route)
				return found, true
			}
		}
	}
	return Route{}, false
}

func requireNamedRoutesMatchScan(t *testing.T, app *App, names []string) {
	t.Helper()
	for _, name := range names {
		want, ok := scanNamedRoute(app, name)
		got := app.GetRoute(name)
		require.Equal(t, want.Path, got.Path, "name %q", name)
		require.Equal(t, want.Method, got.Method, "name %q", name)
		require.Equal(t, want.Name, got.Name, "name %q", name)
		require.Equal(t, annotationOf(&want), annotationOf(&got), "name %q", name)
		if !ok {
			require.Empty(t, got.Path, "name %q", name)
		}
		if ok {
			require.Equal(t, want.Path, app.namedRoute(name).Path, "name %q", name)
		} else {
			require.Nil(t, app.namedRoute(name), "name %q", name)
		}
	}
}

func Test_NamedRouteIndex_TracksTableChanges(t *testing.T) {
	t.Parallel()
	app := New()
	names := []string{"", "a", "b", "c", "renamed", "grouped.x", "missing"}
	h := func(c Ctx) error { return c.SendStatus(StatusOK) }

	app.Get("/a", h).Name("a")
	annotate(t, app.Get("/b", h).Name("b"), "first b")
	requireNamedRoutesMatchScan(t, app, names)

	// Same name twice: the earlier route in stack order wins.
	app.Post("/b2", h).Name("b")
	requireNamedRoutesMatchScan(t, app, names)

	app.Get("/c", h).Name("c")
	require.Equal(t, "/c", app.GetRoute("c").Path)
	app.Name("renamed")
	requireNamedRoutesMatchScan(t, app, names)
	annotated := app.Get("/a2", h).Name("a2")
	got := app.GetRoute("a2")
	require.Empty(t, annotationOf(&got))
	annotate(t, annotated, "annotated after index")
	requireNamedRoutesMatchScan(t, app, append(names, "a2"))
	got = app.GetRoute("a2")
	require.Equal(t, "annotated after index", annotationOf(&got))

	g := app.Group("/g").Name("grouped.")
	g.Get("/x", h).Name("x")
	requireNamedRoutesMatchScan(t, app, names)
	require.Equal(t, "/g/x", app.GetRoute("grouped.x").Path)

	app.RemoveRouteByName("b", MethodGet)
	requireNamedRoutesMatchScan(t, app, names)
	app.RemoveRoute("/a", MethodGet)
	requireNamedRoutesMatchScan(t, app, names)
	app.RemoveRouteFunc(func(r *Route) bool { return r.Name == "c" })
	requireNamedRoutesMatchScan(t, app, names)
	require.Empty(t, app.GetRoute("c").Path)

	apptest.Do(t, app, "/g/x")
	app.Get("/late", h).Name("late")
	require.Equal(t, "/late", app.GetRoute("late").Path)
}

func Test_NamedRouteIndex_ReturnsIndependentCopies(t *testing.T) {
	t.Parallel()
	app := New()
	annotate(t, app.Get("/a", func(Ctx) error { return nil }).Name("a"), "s")

	got := app.GetRoute("a")
	got.annotations = nil
	got.Path = "/changed"
	fresh := app.GetRoute("a")
	require.Equal(t, "s", annotationOf(&fresh))
	require.Equal(t, "/a", fresh.Path)
}

func Test_NamedRouteIndex_ConcurrentWithRegistration(t *testing.T) {
	t.Parallel()
	app := New()
	h := func(Ctx) error { return nil }
	app.Get("/base", h).Name("base")

	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 200 {
				if app.GetRoute("base").Path != "/base" || app.namedRoute("base").Path != "/base" {
					t.Error("base route lost")
					return
				}
			}
		})
	}
	for i := range 50 {
		annotate(t, app.Get("/r"+strconv.Itoa(i), h).Name("r"+strconv.Itoa(i)), "x")
	}
	wg.Wait()
	for i := range 50 {
		require.Equal(t, "/r"+strconv.Itoa(i), app.GetRoute("r"+strconv.Itoa(i)).Path)
	}
}

func Benchmark_Ctx_GetRouteURL(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed
	app.Get("/user/:name", testHandlerOK).Name("User")
	b.ReportAllocs()

	var location string
	var err error
	for b.Loop() {
		location, err = c.GetRouteURL("User", Map{"name": "fiber"})
	}
	require.NoError(b, err)
	require.Equal(b, "/user/fiber", location)
}

func Test_NamedRouteIndex_SmallAndLargeTables(t *testing.T) {
	t.Parallel()
	for _, count := range []int{1, smallIndexMax, smallIndexMax + 1, 40} {
		app := New()
		for i := range count {
			app.Get("/r"+strconv.Itoa(i), testHandlerOK).Name("n" + strconv.Itoa(i))
		}
		for i := range count {
			require.Equal(t, "/r"+strconv.Itoa(i), app.namedRoute("n"+strconv.Itoa(i)).Path, "count %d", count)
		}
		require.Nil(t, app.namedRoute("absent"), "count %d", count)
	}
}

func Test_NamedRouteIndex_SmallScanKeepsRegistrationOrder(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/first", testHandlerOK).Name("dup")
	app.Get("/second", testHandlerOK).Name("dup")
	app.Get("/third", testHandlerOK).Name("other")
	require.Equal(t, "/first", app.GetRoute("dup").Path)
	index := app.namedRoutes.Load()
	require.Equal(t, []string{"dup", "other"}, []string{index.small[0].Name, index.small[1].Name})
}

func Test_RemoveRoute_ClearsBothRegistrationIndexes(t *testing.T) {
	t.Parallel()
	app := New()
	// Two registrations of one path merge into one entry that is indexed under both ids.
	app.Get("/a", testHandlerOK).Name("a")
	app.Get("/a", testHandlerOK)
	app.Get("/b", testHandlerOK).Name("b")

	app.RemoveRoute("/a", MethodGet)
	app.mutex.Lock()
	remaining := len(app.regEntries)
	reverse := len(app.entryRegs)
	app.mutex.Unlock()
	require.Equal(t, 1, remaining, "only /b stays indexed")
	require.Equal(t, 1, reverse)
	require.Empty(t, app.GetRoute("a").Path)
	require.Equal(t, "/b", app.GetRoute("b").Path)

	app.RemoveRouteByName("b", MethodGet)
	app.mutex.Lock()
	require.Empty(t, app.regEntries)
	require.Empty(t, app.entryRegs)
	app.mutex.Unlock()
}

func Test_LockIsFreeAfterStartupAndRebuild(t *testing.T) {
	t.Parallel()
	app := New()
	sub := New()
	sub.Get("/s", testHandlerOK)
	app.Use("/m", sub)
	app.Get("/x", testHandlerOK)

	apptest.Do(t, app, "/m/s")
	app.RebuildTree()

	apptest.Bounded(t, boundedTimeout, "GetRoutes after startup and rebuild", func() { _ = app.GetRoutes() })
}

func Test_RegistrationCursors_AreAtomicValues(t *testing.T) {
	t.Parallel()
	// The cursor is an atomic.Uint64 behind a pointer, which every platform aligns; a copy of a Group, as
	// hooks receive, shares it.
	app := New()
	grp, ok := app.Group("/g").(*Group)
	require.True(t, ok)
	grp.Get("/a", testHandlerOK)
	cp := *grp
	require.Equal(t, grp.cursor.Load(), cp.cursor.Load())
	require.NotZero(t, grp.cursor.Load())
}

func Test_Name_ReachesRoutesRegisteredThroughDomainRouters(t *testing.T) {
	t.Parallel()
	t.Run("group name after a domain route", func(t *testing.T) {
		t.Parallel()
		app := New()
		api := app.Group("/api")
		api.Domain("a.example.com").Get("/x", testHandlerOK)
		api.Name("foo")

		require.Equal(t, "/api/x", app.GetRoute("foo").Path)
	})

	t.Run("domain name from a group that has routes", func(t *testing.T) {
		t.Parallel()
		app := New()
		api := app.Group("/api")
		api.Get("/y", testHandlerOK)
		api.Domain("b.example.com").Name("bar")

		require.Equal(t, "/api/y", app.GetRoute("bar").Path)
	})
}

func Test_Name_ReachesRoutesRegisteredThroughRouteChain(t *testing.T) {
	t.Parallel()
	t.Run("group", func(t *testing.T) {
		t.Parallel()
		app := New()
		api := app.Group("/api")
		api.Get("/a", testHandlerOK)
		api.RouteChain("/b").Get(testHandlerOK)
		annotate(t, api.Name("x"), "S")

		route := app.GetRoute("x")
		require.Equal(t, "/api/b", route.Path)
		require.Equal(t, "S", annotationOf(&route))
	})

	t.Run("domain router", func(t *testing.T) {
		t.Parallel()
		app := New()
		d := app.Domain("x.example.com")
		d.RouteChain("/b").Get(testHandlerOK)
		d.Name("x")

		require.Equal(t, "/b", app.GetRoute("x").Path)
	})

	t.Run("group with middleware", func(t *testing.T) {
		t.Parallel()
		app := New()
		g := app.Group("/g")
		g.Use(testHandlerOK)
		g.RouteChain("/b").Get(testHandlerOK)
		g.Name("x")

		require.Equal(t, "/g/b", app.GetRoute("x").Path)
	})
}

func Test_ExplicitHead_WinsOverTwinOfAnotherDomain(t *testing.T) {
	t.Parallel()
	head := func(t *testing.T, app *App, host string) string {
		t.Helper()
		resp := apptest.Do(t, app, apptest.Req(MethodHead, "/a", apptest.WithHost(host)))
		return resp.Header.Get("X-Handler")
	}
	handler := func(name string) Handler {
		return func(c Ctx) error {
			c.Set("X-Handler", name)
			return c.SendStatus(StatusOK)
		}
	}

	t.Run("domain HEAD registered after a host-agnostic twin exists", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/a", handler("get"))
		app.RebuildTree()
		app.Domain("x.com").Head("/a", handler("explicit"))

		require.Equal(t, "explicit", head(t, app, "x.com"))
		require.Equal(t, "get", head(t, app, "other.com"), "other hosts keep the generated twin")
	})

	t.Run("host-agnostic HEAD registered after a domain twin exists", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Domain("x.com").Get("/a", handler("get"))
		app.RebuildTree()
		app.Head("/a", handler("explicit"))

		require.Equal(t, "explicit", head(t, app, "x.com"))
		require.Equal(t, "explicit", head(t, app, "other.com"))
	})
}

func Test_Mount_StartupConcurrentWithNestedSubApp(t *testing.T) {
	t.Parallel()
	inner := New()
	inner.Get("/leaf", testHandlerOK)
	sub := New()
	sub.Get("/x", testHandlerOK)
	sub.Use("/inner", inner)

	app := New()
	app.Use("/api", sub)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			annotate(t, sub.Get(fmt.Sprintf("/y%d", i), testHandlerOK), "ok")
			_ = sub.GetRoutes()
		}
	})

	apptest.Bounded(t, boundedTimeout, "startup with a nested mount", func() {
		resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/inner/leaf", http.NoBody))
		if assert.NoError(t, err) {
			assert.Equal(t, StatusOK, resp.StatusCode)
		}
	})
	close(stop)
	wg.Wait()
}

func Test_Name_AfterRepeatedUseNamesEveryRouteAtThePath(t *testing.T) {
	t.Parallel()
	app := New()
	g := app.Group("/api", testHandlerOK)
	g.Name("api.")
	g.Get("/x", testHandlerOK)
	g.Use(testHandlerOK)
	g.Name("mw")

	named := 0
	for _, route := range app.GetRoutes() {
		if route.Path == "/api" && route.Method == MethodGet {
			require.Equal(t, "api.mw", route.Name)
			named++
		}
	}
	require.Equal(t, 2, named, "both Use routes at the path carry the name")
	require.Empty(t, routesFor(app, "/api/x")[MethodGet].Name, "a route at another path is not renamed")
}

func Test_Route_Domains(t *testing.T) {
	t.Parallel()
	sub := New()
	sub.Domain(":tenant.example.com").Get("/pets", testHandlerOK)
	sub.Get("/plain", testHandlerOK)
	app := New()
	app.Domain("a.example.com").Get("/a", testHandlerOK)
	app.Domain("api.example.com").Use("/v1", sub)
	app.Get("/any", testHandlerOK)
	app.Handler()

	domains := map[string][]string{}
	for _, route := range app.GetRoutes(true) {
		if route.Method == MethodGet && !route.IsAutoHead() {
			domains[route.Path] = route.Domains()
			require.Equal(t, route.Domain(), firstOrEmpty(route.Domains()), route.Path)
		}
	}
	require.Equal(t, []string{"a.example.com"}, domains["/a"])
	require.Equal(t, []string{":tenant.example.com", "api.example.com"}, domains["/v1/pets"], "innermost first")
	require.Equal(t, []string{"api.example.com"}, domains["/v1/plain"])
	require.Nil(t, domains["/any"])
}

func firstOrEmpty(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

func Test_Route_MarshalsWithoutAnnotations(t *testing.T) {
	t.Parallel()
	app := New()
	annotate(t, app.Get("/annotated", testHandlerOK), "annotated")
	app.Get("/plain", testHandlerOK)

	data, err := json.Marshal(app.GetRoutes(true))
	require.NoError(t, err)
	var routes []map[string]any
	require.NoError(t, json.Unmarshal(data, &routes))
	byPath := map[string]map[string]any{}
	for _, route := range routes {
		if route["method"] == MethodGet {
			byPath[route["path"].(string)] = route //nolint:errcheck,forcetypeassert // the path is always a string
		}
	}
	for _, path := range []string{"/plain", "/annotated"} {
		require.Equal(t, map[string]any{"method": MethodGet, "name": "", "params": nil, "path": path}, byPath[path])
	}
}

func Test_DocHelpers_OnARouterWithoutRoutesChangeNothing(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/a", testHandlerOK)
	annotate(t, app.Group("/g"), "group")
	annotate(t, app.Domain("x.example.com"), "domain")
	annotate(t, app.Group("/h").Domain("y.example.com"), "both")

	route := app.GetRoutes(true)[0]
	require.Nil(t, route.annotations)
}

func Test_GetRoute_RacesWithDocHelpers(t *testing.T) {
	t.Parallel()
	app := New()
	route := app.Get("/x", testHandlerOK).Name("x")
	app.Get("/y", testHandlerOK)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			annotate(t, route, "t"+strconv.Itoa(i%3))
		}
	}()
	for range 200 {
		got := app.GetRoute("x")
		_ = annotationOf(&got)
		app.RemoveRouteFunc(func(r *Route) bool { return annotationOf(r) == "never" })
	}
	close(stop)
	apptest.Recv(t, done, 10*time.Second, "the annotating goroutine did not stop")
}

func Test_AutoHead_TwinsOfRepeatedPath(t *testing.T) {
	t.Parallel()
	// Two GET routes share "/a" without merging, so each has an automatic HEAD route of its own.
	newApp := func() *App {
		app := New()
		app.Get("/a", testHandlerOK).Name("first")
		app.Get("/b", testHandlerOK)
		app.Get("/a", testHandlerOK, testHandlerOK)
		app.RebuildTree()
		return app
	}
	headRoutes := func(app *App) []string {
		var routes []string
		for _, route := range app.stack[app.methodInt(MethodHead)] {
			if route.Path == "/a" {
				routes = append(routes, fmt.Sprintf("%s auto=%t handlers=%d", route.Name, route.autoHead, len(route.Handlers)))
			}
		}
		return routes
	}

	t.Run("an explicit HEAD replaces every twin", func(t *testing.T) {
		t.Parallel()
		app := newApp()
		app.Head("/a", testHandlerOK).Name("explicit")
		app.RebuildTree()
		require.Equal(t, []string{"explicit auto=false handlers=1"}, headRoutes(app))
		require.Equal(t, uint32(6), app.HandlersCount())
	})

	t.Run("removing a GET leaves no twin of it behind", func(t *testing.T) {
		t.Parallel()
		app := newApp()
		app.RemoveRouteFunc(func(r *Route) bool { return r.Path == "/a" && len(r.Handlers) == 2 }, MethodGet)
		app.RebuildTree()
		require.Equal(t, []string{"first auto=true handlers=1"}, headRoutes(app))
		require.Equal(t, uint32(4), app.HandlersCount())
	})

	t.Run("naming a GET names its own twin", func(t *testing.T) {
		t.Parallel()
		app := newApp()
		app.Name("second")
		require.Equal(t, []string{"first auto=true handlers=1", "second auto=true handlers=2"}, headRoutes(app))
	})
}

func Test_RemoveRoute_HandlerCountPerUseRegistration(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use("/x", testHandlerOK)
	app.Post("/y", testHandlerOK)
	app.Use("/x", testHandlerOK, testHandlerOK)
	require.Equal(t, uint32(4), app.HandlersCount())

	app.RemoveRoute("/x")
	require.Equal(t, uint32(1), app.HandlersCount())
}
