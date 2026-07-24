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

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp/fasthttputil"
)

func testHandlerOK(c Ctx) error { return c.SendStatus(StatusOK) }

// routesFor returns the non-use routes registered for path, keyed by method.
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

	// Metadata after a multi-method registration must reach every method
	// variant, not only the last-registered one.
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
	// All() registers a use-style route per request method; every copy must be tagged.
	require.Equal(t, len(DefaultMethods), tagged)
}

func Test_RouteDocs_UseDoesNotLeakToEndpoints(t *testing.T) {
	t.Parallel()
	app := New()

	app.Post("/shared", testHandlerOK)
	app.Use("/shared", func(c Ctx) error { return c.Next() })
	// Documenting the middleware route must not touch the POST endpoint.
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

	// A typed interface slice/map with nil elements must clone without panicking.
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

// fullyPopulatedRoute builds a Route with every exported field set to a
// non-zero value. Test_CopyRoute_Complete uses reflection to enforce that,
// so adding a Route field without updating this fixture — and copyRoute —
// fails the build gate.
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

	// Guard: every exported Route field must be non-zero in the fixture, so a
	// newly added field forces this test (and copyRoute) to be updated.
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

	// Handlers are funcs (not comparable); compare the rest for equality.
	require.Len(t, clone.Handlers, len(original.Handlers))
	origCmp, cloneCmp := *original, *clone
	origCmp.Handlers, cloneCmp.Handlers = nil, nil
	require.Equal(t, origCmp, cloneCmp)

	// Mutating the original's composite metadata must not affect the clone.
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

// Test_Shutdown_WithInflightGetRoutes verifies graceful shutdown completes
// while an in-flight handler calls GetRoutes (previously both waited on
// app.mutex forever).
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

	// Give Shutdown time to start waiting on the in-flight request, then let
	// the handler proceed into GetRoutes.
	time.Sleep(100 * time.Millisecond)
	close(proceed)

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown deadlocked on in-flight GetRoutes call")
	}
}

// Test_Hooks_MayCallLockingAppMethods verifies OnRoute/OnName hooks can call
// GetRoutes and documentation helpers without self-deadlocking (hooks now fire
// after the router lock is released).
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

// Test_GetRoute_DeepCopy verifies GetRoute (singular) returns a deep copy like
// GetRoutes, so mutating the returned metadata cannot corrupt the live route.
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

// Test_Mount_StartupConcurrentSubAppDocHelpers verifies parent startup clones
// sub-app routes under the sub-app's lock, so concurrent documentation helpers
// on the sub-app cannot race the clone (run with -race).
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

// Test_ScopedDocHelpers_TargetOwnRegistration verifies helpers on RouteChain,
// Group, and Domain document their own last registration even after unrelated
// routes were registered on the app (previously they hit the app-global
// latest route).
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

		// No registration happened through these scopes yet, so their helpers
		// must not touch anything (previously they hit GET /a).
		app.RouteChain("/x").Summary("nope")
		app.Group("/g").Summary("nope")
		app.Domain("d.example.com").Summary("nope")

		require.Empty(t, routesFor(app, "/a")[MethodGet].Summary)
		require.Equal(t, rev, app.RoutesRevision())
	})
}

// Test_Domain_SamePathKeepsSeparateRoutes verifies same-path registrations on
// different domains are no longer compression-merged, so each keeps its own
// handlers and documentation.
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

// Test_DocCursor_InvalidatedByRemovalAndStartup verifies stray helpers cannot
// re-document removed routes or an arbitrary auto-HEAD twin's origin.
func Test_DocCursor_InvalidatedByRemovalAndStartup(t *testing.T) {
	t.Parallel()

	t.Run("RemoveRoute", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/a", testHandlerOK)
		app.RemoveRoute("/a", MethodGet)
		rev := app.RoutesRevision()

		app.Summary("dangling") // must be a no-op, not a dead-object write
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

		// The cursor still points at the last registration (POST /c), not at
		// the auto-HEAD twin created for GET /a during startup.
		app.Summary("late")
		require.Equal(t, "sum-a", routesFor(app, "/a")[MethodGet].Summary)
		require.Equal(t, "late", routesFor(app, "/c")[MethodPost].Summary)
	})
}

// Test_MountRegistration_DocHelpersAreNoOps verifies metadata chained onto a
// sub-app mount neither survives startup nor pretends to: the helpers are
// explicit no-ops for mount placeholders.
func Test_MountRegistration_DocHelpersAreNoOps(t *testing.T) {
	t.Parallel()
	sub := New()
	sub.Get("/x", testHandlerOK)

	app := New()
	rev := app.RoutesRevision()
	app.Use("/api", sub).Summary("mount summary").Tags("mount")
	require.Greater(t, app.RoutesRevision(), rev) // registration bumped...

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

// Test_Security_EmptyScopesStayNonNil verifies an empty (non-nil) scope list
// survives cloning, so the generated document can emit [] rather than null.
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

// Test_ExampleFields_DeepCopied verifies user-held maps passed as Example are
// not aliased between the live route and GetRoutes snapshots.
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

	// Mutating the snapshot must not write through to the live route...
	snapshot["name"] = "tampered"
	fresh := routesFor(app, "/x")[MethodPost]
	freshExample, ok := fresh.RequestBody.Example.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "original", freshExample["name"])

	// ...and neither must mutating the map the caller passed in.
	example["name"] = "caller-mutated"
	fresh = routesFor(app, "/x")[MethodPost]
	freshExample, ok = fresh.RequestBody.Example.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "original", freshExample["name"])
}

// Test_AutoHeadTwins_CarryNoDocumentation verifies startup-created HEAD twins
// are uniformly undocumented instead of half-copied.
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
