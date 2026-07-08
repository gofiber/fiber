package fiber

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
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
