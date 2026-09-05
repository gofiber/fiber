package fiber

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp/fasthttputil"
)

func testHandlerOK(c Ctx) error { return c.SendStatus(StatusOK) }

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

func Test_RouteDocs_MultiMethodRegistration(t *testing.T) {
	t.Parallel()
	app := New()

	app.Add([]string{MethodGet, MethodPost}, "/multi", testHandlerOK).
		Summary("multi summary").
		Security(map[string][]string{"auth": {}})

	routes := routesFor(app, "/multi")
	require.Len(t, routes, 2)
	for method, route := range routes {
		require.Equal(t, "multi summary", route.Summary, "method %s missing summary", method)
		require.Len(t, route.Security, 1, "method %s missing security", method)
	}
}

func Test_RouteDocs_AllMethodsRegistration(t *testing.T) {
	t.Parallel()
	app := New()

	app.All("/everything", testHandlerOK).Tags("all")

	tagged := 0
	routes := app.GetRoutes()
	for i := range routes {
		if routes[i].Path == "/everything" && len(routes[i].Tags) == 1 && routes[i].Tags[0] == "all" {
			tagged++
		}
	}
	require.Equal(t, len(DefaultMethods), tagged)
}

func Test_RouteDocs_UseDoesNotLeakToEndpoints(t *testing.T) {
	t.Parallel()
	app := New()

	app.Post("/shared", testHandlerOK)
	app.Use("/shared", func(c Ctx) error { return c.Next() })
	app.Produces(MIMEApplicationJSON)

	routes := app.GetRoutes()
	for i := range routes {
		route := &routes[i]
		if route.Path != "/shared" {
			continue
		}
		if route.IsMiddleware() {
			require.True(t, route.Produces == MIMEApplicationJSON, "middleware route should carry the metadata") //nolint:testifylint // media type string, not JSON content
		} else {
			require.Empty(t, route.Produces, "metadata leaked onto %s %s", route.Method, route.Path)
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

	app.Summary("rev summary")
	afterDocs := app.RoutesRevision()
	require.Greater(t, afterDocs, afterRegister, "metadata mutation must bump the revision")

	app.RemoveRoute("/rev", MethodGet)
	require.Greater(t, app.RoutesRevision(), afterDocs, "removal must bump the revision")
}

func Test_CopyAnyMap_NilInterfaceElement(t *testing.T) {
	t.Parallel()

	var nilStringer fmt.Stringer
	src := map[string]any{
		"slice": []fmt.Stringer{nilStringer},
		"map":   map[string]fmt.Stringer{"k": nil},
	}
	require.NotPanics(t, func() {
		cloned := copyAnyMap(src)
		require.Len(t, cloned, 2)
	})
}

func fullyPopulatedRoute() *Route {
	return &Route{
		Method:      MethodPost,
		Name:        "name",
		Path:        "/full/:id",
		Summary:     "summary",
		Description: "description",
		Consumes:    MIMEApplicationJSON,
		Produces:    MIMEApplicationJSON,
		Handlers:    []Handler{testHandlerOK},
		Params:      []string{"id"},
		Tags:        []string{"tag"},
		Deprecated:  true,
		Security:    []map[string][]string{{"auth": {"read"}}},
		Parameters: []RouteParameter{{
			Name: "id", In: "path", Required: true,
			Schema: map[string]any{"type": "string"},
		}},
		RequestBody: &RouteRequestBody{
			Description: "body",
			MediaTypes:  []string{MIMEApplicationJSON},
			Schema:      map[string]any{"type": "object"},
		},
		Responses: map[string]RouteResponse{
			"200": {
				Description: "ok",
				Headers:     map[string]any{"X-H": map[string]any{"description": "h"}},
				Links:       map[string]any{"next": map[string]any{"operationId": "x"}},
				Content:     map[string]RouteMediaType{MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}}},
			},
		},
		ExternalDocs:        map[string]any{"url": "https://example.com"},
		OperationExtensions: map[string]any{"x-custom": "v"},
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
			"Route field %q is zero in fullyPopulatedRoute(); update the fixture AND copyRoute/clone helpers", field.Name)
	}

	clone := app.copyRoute(original)

	require.Len(t, clone.Handlers, len(original.Handlers))
	origCmp, cloneCmp := *original, *clone
	origCmp.Handlers, cloneCmp.Handlers = nil, nil
	require.Equal(t, origCmp, cloneCmp)

	original.Tags[0] = "mutated"
	original.Security[0]["auth"] = append(original.Security[0]["auth"], "write")
	original.Parameters[0].Schema["type"] = "integer"
	original.RequestBody.Schema["type"] = "mutated"
	resp := original.Responses["200"]
	resp.Headers["X-H"] = "mutated"
	resp.Links["next"] = "mutated"
	original.ExternalDocs["url"] = "mutated"
	original.OperationExtensions["x-custom"] = "mutated"

	require.Equal(t, "tag", clone.Tags[0])
	require.Equal(t, []string{"read"}, clone.Security[0]["auth"])
	require.Equal(t, "string", clone.Parameters[0].Schema["type"])
	require.Equal(t, "object", clone.RequestBody.Schema["type"])
	cloneResp := clone.Responses["200"]
	require.IsType(t, map[string]any{}, cloneResp.Headers["X-H"])
	require.IsType(t, map[string]any{}, cloneResp.Links["next"])
	require.Equal(t, "https://example.com", clone.ExternalDocs["url"])
	require.Equal(t, "v", clone.OperationExtensions["x-custom"])
}

func Test_RouteChain_DocumentationHelpers(t *testing.T) {
	t.Parallel()
	app := New()

	app.RouteChain("/chained").
		Get(testHandlerOK).
		Summary("chained summary").
		Tags("chained")

	routes := routesFor(app, "/chained")
	require.Contains(t, routes, MethodGet)
	require.Equal(t, "chained summary", routes[MethodGet].Summary)
	require.Equal(t, []string{"chained"}, routes[MethodGet].Tags)
}

func Test_Shutdown_WithInflightGetRoutes(t *testing.T) {
	t.Parallel()
	app := New()

	started := make(chan struct{})
	proceed := make(chan struct{})
	app.Get("/routes", func(c Ctx) error {
		close(started)
		<-proceed
		return c.SendString(strconv.Itoa(len(c.App().GetRoutes())))
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		if err := app.Listener(ln, ListenConfig{DisableStartupMessage: true}); err != nil {
			panic(err)
		}
	}()

	go func() {
		conn, err := ln.Dial()
		if err != nil {
			panic(err)
		}
		if _, err := conn.Write([]byte("GET /routes HTTP/1.1\r\nHost: example.com\r\n\r\n")); err != nil {
			panic(err)
		}
	}()

	<-started
	done := make(chan error, 1)
	go func() { done <- app.Shutdown() }()

	time.Sleep(100 * time.Millisecond)
	close(proceed)

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown deadlocked on in-flight GetRoutes call")
	}
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

	done := make(chan struct{})
	go func() {
		app.Get("/x", testHandlerOK).Name("x")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("route registration deadlocked inside a hook")
	}
	require.Positive(t, onRouteSaw)
	require.Positive(t, onNameSaw)
}

func Test_GetRoute_DeepCopy(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/users", testHandlerOK).Name("users").
		Response(StatusOK, "ok", MIMEApplicationJSON)

	r := app.GetRoute("users")
	require.Contains(t, r.Responses, "200")
	r.Responses["200"] = RouteResponse{Description: "tampered"}
	r.Responses["999"] = RouteResponse{}

	fresh := app.GetRoute("users")
	require.Equal(t, "ok", fresh.Responses["200"].Description)
	require.NotContains(t, fresh.Responses, "999")
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
			sub.Get(fmt.Sprintf("/x%d", i), testHandlerOK).
				Response(StatusOK, "ok", MIMEApplicationJSON)
		}
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/x", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	close(stop)
	wg.Wait()
}

func Test_ScopedDocHelpers_TargetOwnRegistration(t *testing.T) {
	t.Parallel()

	t.Run("RouteChain", func(t *testing.T) {
		t.Parallel()
		app := New()
		users := app.RouteChain("/users")
		users.Get(testHandlerOK)
		app.Get("/health", testHandlerOK)
		users.Summary("List users")

		routes := routesFor(app, "/users")
		require.Equal(t, "List users", routes[MethodGet].Summary)
		require.Empty(t, routesFor(app, "/health")[MethodGet].Summary)
	})

	t.Run("Group", func(t *testing.T) {
		t.Parallel()
		app := New()
		api := app.Group("/api")
		api.Get("/a", testHandlerOK)
		app.Get("/b", testHandlerOK)
		api.Summary("group route")

		require.Equal(t, "group route", routesFor(app, "/api/a")[MethodGet].Summary)
		require.Empty(t, routesFor(app, "/b")[MethodGet].Summary)
	})

	t.Run("Domain", func(t *testing.T) {
		t.Parallel()
		app := New()
		d := app.Domain("api.example.com")
		d.Get("/a", testHandlerOK)
		app.Get("/b", testHandlerOK)
		d.Summary("domain route")

		require.Equal(t, "domain route", routesFor(app, "/a")[MethodGet].Summary)
		require.Empty(t, routesFor(app, "/b")[MethodGet].Summary)
	})

	t.Run("UnregisteredScopeIsNoOp", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/a", testHandlerOK)
		rev := app.RoutesRevision()

		app.RouteChain("/x").Summary("nope")
		app.Group("/g").Summary("nope")
		app.Domain("d.example.com").Summary("nope")

		require.Empty(t, routesFor(app, "/a")[MethodGet].Summary)
		require.Equal(t, rev, app.RoutesRevision())
	})
}

func Test_Domain_SamePathKeepsSeparateRoutes(t *testing.T) {
	t.Parallel()
	app := New()
	app.Domain("a.example.com").Get("/x", func(c Ctx) error {
		return c.SendString("host a")
	}).Summary("A docs")
	app.Domain("b.example.com").Get("/x", func(c Ctx) error {
		return c.SendString("host b")
	}).Summary("B docs")

	var summaries []string
	for _, route := range app.GetRoutes() {
		if route.Path == "/x" && route.Method == MethodGet && !route.IsAutoHead() {
			summaries = append(summaries, route.Summary)
		}
	}
	require.ElementsMatch(t, []string{"A docs", "B docs"}, summaries)

	for host, want := range map[string]string{"a.example.com": "host a", "b.example.com": "host b"} {
		req := httptest.NewRequest(MethodGet, "/x", http.NoBody)
		req.Host = host
		resp, err := app.Test(req)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, want, string(body), host)
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

		app.Summary("dangling")
		require.Equal(t, rev, app.RoutesRevision())
	})

	t.Run("AfterStartup", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/a", testHandlerOK).Summary("sum-a")
		app.Post("/c", testHandlerOK)

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/a", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)

		app.Summary("late")
		require.Equal(t, "sum-a", routesFor(app, "/a")[MethodGet].Summary)
		require.Equal(t, "late", routesFor(app, "/c")[MethodPost].Summary)
	})
}

func Test_MountRegistration_DocHelpersAreNoOps(t *testing.T) {
	t.Parallel()
	sub := New()
	sub.Get("/x", testHandlerOK)

	app := New()
	rev := app.RoutesRevision()
	app.Use("/api", sub).Summary("mount summary").Tags("mount")
	require.Greater(t, app.RoutesRevision(), rev)

	for _, route := range app.GetRoutes() {
		require.NotEqual(t, "mount summary", route.Summary, route.Path)
		require.NotContains(t, route.Tags, "mount", route.Path)
	}

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/x", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	for _, route := range app.GetRoutes() {
		require.NotEqual(t, "mount summary", route.Summary, route.Path)
	}
}

func Test_Security_EmptyScopesStayNonNil(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/secure", testHandlerOK).Security(map[string][]string{"bearerAuth": {}})

	route := routesFor(app, "/secure")[MethodGet]
	require.Len(t, route.Security, 1)
	scopes, ok := route.Security[0]["bearerAuth"]
	require.True(t, ok)
	require.NotNil(t, scopes)
	require.Empty(t, scopes)
}

func Test_ExampleFields_DeepCopied(t *testing.T) {
	t.Parallel()
	app := New()
	example := map[string]any{"name": "original"}
	app.Post("/x", testHandlerOK).
		RequestBodyWithExample("body", true, nil, "", example, nil, MIMEApplicationJSON)

	route := routesFor(app, "/x")[MethodPost]
	require.NotNil(t, route.RequestBody)
	snapshot, ok := route.RequestBody.Example.(map[string]any)
	require.True(t, ok)

	snapshot["name"] = "tampered"
	fresh := routesFor(app, "/x")[MethodPost]
	freshExample, ok := fresh.RequestBody.Example.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "original", freshExample["name"])

	example["name"] = "caller-mutated"
	fresh = routesFor(app, "/x")[MethodPost]
	freshExample, ok = fresh.RequestBody.Example.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "original", freshExample["name"])
}

func Test_AutoHeadTwins_CarryNoDocumentation(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/doc", testHandlerOK).
		Summary("sum").Description("desc").Deprecated().
		Response(StatusOK, "ok", MIMEApplicationJSON)

	resp, err := app.Test(httptest.NewRequest(MethodHead, "/doc", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	twin, found := Route{}, false
	for _, route := range app.GetRoutes() {
		if route.Method == MethodHead && route.Path == "/doc" && route.IsAutoHead() {
			twin, found = route, true
		}
	}
	require.True(t, found)
	require.Empty(t, twin.Summary)
	require.Empty(t, twin.Description)
	require.Empty(t, twin.Consumes)
	require.Empty(t, twin.Produces)
	require.False(t, twin.Deprecated)
	require.Empty(t, twin.Responses)
}

func Test_Hooks_OutsideRouterLock(t *testing.T) {
	t.Parallel()

	runBounded := func(t *testing.T, name string, fn func()) {
		t.Helper()
		done := make(chan struct{})
		go func() { defer close(done); fn() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s deadlocked on the router lock", name)
		}
	}

	t.Run("OnGroupName", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Hooks().OnGroupName(func(Group) error { _ = app.GetRoutes(); return nil })
		runBounded(t, "OnGroupName", func() { app.Group("/x").Name("grp") })
	})

	t.Run("OnMountThroughDomain", func(t *testing.T) {
		t.Parallel()
		app := New()
		sub := New()
		sub.Get("/a", testHandlerOK)
		sub.Hooks().OnMount(func(*App) error { _ = sub.GetRoutes(); return nil })
		runBounded(t, "OnMount", func() { app.Domain("api.example.com").Use("/s", sub) })
	})

	t.Run("OnPreShutdown", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/a", testHandlerOK)
		app.Hooks().OnPreShutdown(func() error { _ = app.GetRoutes(); return nil })

		ln := fasthttputil.NewInmemoryListener()
		go func() {
			if err := app.Listener(ln, ListenConfig{DisableStartupMessage: true}); err != nil {
				panic(err)
			}
		}()
		time.Sleep(100 * time.Millisecond)
		runBounded(t, "OnPreShutdown", func() { require.NoError(t, app.Shutdown()) })
	})
}

func Test_DocMetadata_CyclicValueDoesNotCrash(t *testing.T) {
	t.Parallel()
	cyclic := map[string]any{"k": 1}
	cyclic["self"] = cyclic

	app := New()
	require.NotPanics(t, func() {
		app.Get("/f", testHandlerOK).
			ResponseWithExample(StatusOK, "ok", nil, "", cyclic, nil, MIMEApplicationJSON)
		require.NotEmpty(t, app.GetRoutes())
	})
}

func Test_ScopedName_TargetsOwnRegistration(t *testing.T) {
	t.Parallel()

	t.Run("RouteChain", func(t *testing.T) {
		t.Parallel()
		app := New()
		chain := app.RouteChain("/users")
		chain.Get(testHandlerOK)
		app.Get("/other", testHandlerOK)
		chain.Name("listUsers").Summary("List users")

		require.Equal(t, "listUsers", routesFor(app, "/users")[MethodGet].Name)
		require.Equal(t, "List users", routesFor(app, "/users")[MethodGet].Summary)
		require.Empty(t, routesFor(app, "/other")[MethodGet].Name)
		require.Equal(t, "/users", app.GetRoute("listUsers").Path)
	})

	t.Run("Group", func(t *testing.T) {
		t.Parallel()
		app := New()
		api := app.Group("/api")
		api.Get("/a", testHandlerOK)
		app.Get("/b", testHandlerOK)
		api.Name("groupRoute")

		require.Equal(t, "groupRoute", routesFor(app, "/api/a")[MethodGet].Name)
		require.Empty(t, routesFor(app, "/b")[MethodGet].Name)
	})

	t.Run("Domain", func(t *testing.T) {
		t.Parallel()
		app := New()
		d := app.Domain("api.example.com")
		d.Get("/a", testHandlerOK)
		app.Get("/b", testHandlerOK)
		d.Name("domainRoute")

		require.Equal(t, "domainRoute", routesFor(app, "/a")[MethodGet].Name)
		require.Empty(t, routesFor(app, "/b")[MethodGet].Name)
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
	api.Summary("list users").Tags("users")

	route := routesFor(app, "/api/users")[MethodGet]
	require.Equal(t, "list users", route.Summary)
	require.Equal(t, []string{"users"}, route.Tags)
	require.Greater(t, app.RoutesRevision(), rev)
}

func Test_Group_SubGroupDoesNotStealParentCursor(t *testing.T) {
	t.Parallel()
	app := New()
	v1 := app.Group("/v1")
	v1.Get("/users", testHandlerOK)
	v1.Group("/sub", func(c Ctx) error { return c.Next() })
	v1.Summary("list users").Tags("users")

	route := routesFor(app, "/v1/users")[MethodGet]
	require.Equal(t, "list users", route.Summary)
	require.Equal(t, []string{"users"}, route.Tags)
}

func Test_AddParameter_ExampleDetached(t *testing.T) {
	t.Parallel()

	app := New()
	example := map[string]any{"nested": map[string]any{"k": "original"}}
	app.Get("/p", testHandlerOK).
		AddParameter(RouteParameter{Name: "q", In: "query", Example: example})

	nested, ok := example["nested"].(map[string]any)
	require.True(t, ok)
	nested["k"] = "mutated"
	example["added"] = true

	stored := app.GetRoutes()[0].Parameters[0].Example
	require.Equal(t, map[string]any{"nested": map[string]any{"k": "original"}}, stored)
}

func Test_AddParameter_SchemaSelection(t *testing.T) {
	t.Parallel()

	t.Run("content wins over schema", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/content", testHandlerOK).AddParameter(RouteParameter{
			Name:      "filter",
			In:        "query",
			Schema:    map[string]any{"type": "string"},
			SchemaRef: "#/components/schemas/Filter",
			Content: map[string]RouteMediaType{
				MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}},
			},
		})

		param := routesFor(app, "/content")[MethodGet].Parameters[0]
		require.Nil(t, param.Schema)
		require.Empty(t, param.SchemaRef)
		require.Equal(t, map[string]any{"type": "object"}, param.Content[MIMEApplicationJSON].Schema)
	})

	t.Run("schema ref becomes a ref schema", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/ref", testHandlerOK).AddParameter(RouteParameter{
			Name:      "id",
			In:        "query",
			SchemaRef: "#/components/schemas/ID",
		})

		param := routesFor(app, "/ref")[MethodGet].Parameters[0]
		require.Equal(t, map[string]any{"$ref": "#/components/schemas/ID"}, param.Schema)
	})

	t.Run("querystring gets no default schema", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/qs", testHandlerOK).
			AddParameter(RouteParameter{Name: "q", In: "querystring"})

		param := routesFor(app, "/qs")[MethodGet].Parameters[0]
		require.Equal(t, "querystring", param.In)
		require.Nil(t, param.Schema)
	})

	t.Run("other locations get a default string schema", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/plain", testHandlerOK).
			AddParameter(RouteParameter{Name: "q", In: "query"})

		param := routesFor(app, "/plain")[MethodGet].Parameters[0]
		require.Equal(t, map[string]any{"type": "string"}, param.Schema)
	})

	t.Run("path location is forced required", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/items/:id", testHandlerOK).
			AddParameter(RouteParameter{Name: "id", In: "path"})

		param := routesFor(app, "/items/:id")[MethodGet].Parameters[0]
		require.True(t, param.Required)
	})

	t.Run("explode is copied rather than aliased", func(t *testing.T) {
		t.Parallel()
		explode := true
		app := New()
		app.Get("/explode", testHandlerOK).
			AddParameter(RouteParameter{Name: "ids", In: "query", Explode: &explode})

		param := routesFor(app, "/explode")[MethodGet].Parameters[0]
		require.NotNil(t, param.Explode)
		require.True(t, *param.Explode)
		explode = false
		param = routesFor(app, "/explode")[MethodGet].Parameters[0]
		require.True(t, *param.Explode)
	})

	t.Run("empty name panics", func(t *testing.T) {
		t.Parallel()
		app := New()
		require.Panics(t, func() {
			app.Get("/panic", testHandlerOK).AddParameter(RouteParameter{Name: "  ", In: "query"})
		})
	})

	t.Run("invalid location panics", func(t *testing.T) {
		t.Parallel()
		app := New()
		require.Panics(t, func() {
			app.Get("/panic2", testHandlerOK).AddParameter(RouteParameter{Name: "x", In: "body"})
		})
	})
}

func Test_AddParameter_ContentSingleMediaType(t *testing.T) {
	t.Parallel()

	t.Run("two media types panic", func(t *testing.T) {
		t.Parallel()
		app := New()
		require.Panics(t, func() {
			app.Get("/multi", testHandlerOK).AddParameter(RouteParameter{
				Name: "filter",
				In:   "query",
				Content: map[string]RouteMediaType{
					MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}},
					MIMEApplicationXML:  {Schema: map[string]any{"type": "object"}},
				},
			})
		})
	})

	t.Run("invalid media type panics", func(t *testing.T) {
		t.Parallel()
		app := New()
		require.Panics(t, func() {
			app.Get("/bad", testHandlerOK).AddParameter(RouteParameter{
				Name:    "filter",
				In:      "query",
				Content: map[string]RouteMediaType{"nonsense": {}},
			})
		})
	})

	t.Run("one media type is accepted", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/one", testHandlerOK).AddParameter(RouteParameter{
			Name:    "filter",
			In:      "query",
			Content: map[string]RouteMediaType{MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}}},
		})
		param := routesFor(app, "/one")[MethodGet].Parameters[0]
		require.Len(t, param.Content, 1)
	})
}

func Test_ContentHelpers_ValidateMediaTypes(t *testing.T) {
	t.Parallel()

	t.Run("request body content", func(t *testing.T) {
		t.Parallel()
		app := New()
		require.Panics(t, func() {
			app.Post("/a", testHandlerOK).
				RequestBodyContent("payload", true, map[string]RouteMediaType{"invalid": {}})
		})
	})

	t.Run("response content", func(t *testing.T) {
		t.Parallel()
		app := New()
		require.Panics(t, func() {
			app.Get("/b", testHandlerOK).
				ResponseContent(StatusOK, "ok", map[string]RouteMediaType{"invalid": {}})
		})
	})

	t.Run("valid media types are accepted", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Post("/c", testHandlerOK).
			RequestBodyContent("payload", true, map[string]RouteMediaType{MIMEApplicationJSON: {}}).
			ResponseContent(StatusOK, "ok", map[string]RouteMediaType{MIMEApplicationXML: {}})
		route := routesFor(app, "/c")[MethodPost]
		require.Len(t, route.RequestBody.Content, 1)
	})
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

	g1.Summary("from-g1")
	g2.Description("from-g2")

	var found *Route
	for _, r := range app.GetRoutes() {
		if r.Path == "/g/same" && r.Method == MethodGet {
			route := r
			found = &route
		}
	}
	require.NotNil(t, found)
	require.Equal(t, "from-g1", found.Summary, "the merged-away scope's helper was dropped")
	require.Equal(t, "from-g2", found.Description)
}

func Test_GroupMount_DoesNotRetargetPreviousRoute(t *testing.T) {
	t.Parallel()

	app := New()
	grp := app.Group("/v1")
	grp.Get("/health", testHandlerOK).Summary("health")

	sub := New()
	sub.Get("/x", testHandlerOK)
	grp.Use("/api", sub)
	grp.Tags("mounted")

	for _, r := range app.GetRoutes() {
		if r.Path == "/v1/health" {
			require.Equal(t, "health", r.Summary)
			require.Empty(t, r.Tags, "the mount retargeted the previous route")
		}
	}
}

func Test_CopyAnyValue_NamedMapCycle(t *testing.T) {
	t.Parallel()

	type metadata map[string]any
	cyclic := metadata{}
	cyclic["self"] = cyclic
	require.NotPanics(t, func() { _ = copyAnyValue(cyclic) })

	type list []any
	cyclicList := list{nil}
	cyclicList[0] = cyclicList
	require.NotPanics(t, func() { _ = copyAnyValue(cyclicList) })
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
		api.Summary("Items")

		require.Equal(t, "Items", findRoute(t, app, MethodGet, "/api/items").Summary)
		require.Equal(t, "Items", findRoute(t, app, MethodPost, "/api/items").Summary)
	})

	t.Run("earlier scope never reaches the later registration", func(t *testing.T) {
		t.Parallel()
		app := New()
		api := app.Group("/api")
		api.Get("/a", testHandlerOK)
		app.Add([]string{MethodGet, MethodPost}, "/api/a", testHandlerOK)
		api.Summary("list")

		require.Equal(t, "list", findRoute(t, app, MethodGet, "/api/a").Summary)
		require.Empty(t, findRoute(t, app, MethodPost, "/api/a").Summary, "the POST route was never registered by the group")
	})

	t.Run("removed entries are forgotten", func(t *testing.T) {
		t.Parallel()
		app := New()
		g1 := app.Group("/g")
		g2 := app.Group("/g")
		g1.Get("/same", testHandlerOK)
		g2.Get("/same", testHandlerOK)
		app.RemoveRoute("/g/same", MethodGet)
		require.NotPanics(t, func() { g1.Summary("gone") })
		app.mutex.Lock()
		defer app.mutex.Unlock()
		require.Empty(t, app.mergedEntries)
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
				grp.Summary(prefix)
			})
		}
		wg.Wait()

		for _, prefix := range []string{"/a", "/b"} {
			require.Equal(t, prefix, findRoute(t, app, MethodGet, prefix+"/x").Summary)
			require.Equal(t, prefix, findRoute(t, app, MethodPost, prefix+"/x").Summary)
		}
	}
}

func Test_AutoHeadTwin_OutsideRegistrations(t *testing.T) {
	t.Parallel()

	app := New()
	api := app.Group("/api")
	api.Get("/a", testHandlerOK)
	app.Get("/b", testHandlerOK)

	_, err := app.Test(httptest.NewRequest(MethodGet, "/api/a", http.NoBody))
	require.NoError(t, err)

	api.Summary("A").Deprecated()

	require.Equal(t, "A", findRoute(t, app, MethodGet, "/api/a").Summary)
	head := findRoute(t, app, MethodHead, "/api/a")
	require.True(t, head.IsAutoHead())
	require.Empty(t, head.Summary)
	require.False(t, head.Deprecated)
}

func Test_Name_ReachesAutoHeadTwin(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/a", testHandlerOK)
	_, err := app.Test(httptest.NewRequest(MethodGet, "/a", http.NoBody))
	require.NoError(t, err)

	app.Name("users")
	require.Equal(t, "users", findRoute(t, app, MethodGet, "/a").Name)
	require.Equal(t, "users", findRoute(t, app, MethodHead, "/a").Name)

	grp := app.Group("/g")
	grp.Get("/b", testHandlerOK)
	_, err = app.Test(httptest.NewRequest(MethodGet, "/g/b", http.NoBody))
	require.NoError(t, err)
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
	app.Group("/api", testHandlerOK).Hidden()
	mw := findRoute(t, app, MethodGet, "/api")
	require.True(t, mw.IsHidden())

	d := app.Domain("api.example.com")
	d.Get("/a", testHandlerOK)
	sub := d.Group("/v1", testHandlerOK)
	d.Summary("list")
	sub.Summary("mw")

	require.Equal(t, "list", findRoute(t, app, MethodGet, "/a").Summary)
	require.Equal(t, "mw", findRoute(t, app, MethodPost, "/v1").Summary)
}

func Test_LockingLookups_FromCallbacks(t *testing.T) {
	t.Parallel()

	t.Run("RemoveRouteFunc matcher", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/old", testHandlerOK).Name("old")
		app.Get("/keep", testHandlerOK)

		done := make(chan struct{})
		go func() {
			defer close(done)
			app.RemoveRouteFunc(func(r *Route) bool { return r.Path == app.GetRoute("old").Path })
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			require.Fail(t, "RemoveRouteFunc deadlocked on GetRoute")
		}
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

		done := make(chan struct{})
		go func() {
			defer close(done)
			resp, err := parent.Test(httptest.NewRequest(MethodHead, "/api/x", http.NoBody))
			assert.NoError(t, err)
			assert.Equal(t, StatusOK, resp.StatusCode)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			require.Fail(t, "startup deadlocked on the parent lock")
		}
		require.Positive(t, seen)
	})
}

func Test_CopyAnyMap_KeepsNestedEmptyMaps(t *testing.T) {
	t.Parallel()

	app := New()
	app.Post("/x", testHandlerOK).ResponseWithExample(StatusOK, "", map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}, "", []any{map[string]any{}}, nil, MIMEApplicationJSON)

	resp := findRoute(t, app, MethodPost, "/x").Responses["200"]
	require.NotNil(t, resp.Schema["properties"])
	require.Equal(t, map[string]any{}, resp.Schema["properties"])
	example, ok := resp.Example.([]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{}, example[0])

	require.Nil(t, copyAnyMap(map[string]any{}), "a top-level empty map still reads as unset")
}

func Test_ResponseExample_NotSharedAcrossBatch(t *testing.T) {
	t.Parallel()

	app := New()
	app.Add([]string{MethodGet, MethodPost}, "/e", testHandlerOK).
		ResponseWithExample(StatusOK, "ok", map[string]any{"type": "object"}, "", map[string]any{"k": "v"}, nil, MIMEApplicationJSON)

	app.mutex.Lock()
	defer app.mutex.Unlock()
	var examples []map[string]any
	for _, routes := range app.stack {
		for _, r := range routes {
			if r.Path == "/e" {
				example, ok := r.Responses["200"].Example.(map[string]any)
				require.True(t, ok)
				examples = append(examples, example)
			}
		}
	}
	require.Len(t, examples, 2)
	examples[0]["k"] = "changed"
	require.Equal(t, "v", examples[1]["k"])
}

func Test_RouteForURL_SkipsDocumentation(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/users/:id", testHandlerOK).Name("user").
		ResponseWithExample(StatusOK, "ok", map[string]any{"type": "object"}, "", map[string]any{"k": "v"}, nil, MIMEApplicationJSON)

	route := app.routeForURL("user")
	require.Equal(t, "/users/:id", route.Path)
	require.Nil(t, route.Responses)
	require.Empty(t, app.routeForURL("missing").Path)

	app.Get("/go", func(c Ctx) error { return c.Redirect().Route("user", RedirectConfig{Params: Map{"id": "7"}}) })
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/go", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, "/users/7", resp.Header.Get(HeaderLocation))
}
