package openapi

import (
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v3/internal/apptest"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// routeFor returns a copy of the route registered for method and path, skipping automatic HEAD twins.
func routeFor(t *testing.T, app *fiber.App, method, path string) fiber.Route {
	t.Helper()
	routes := app.GetRoutes()
	for i := range routes {
		if routes[i].Method == method && routes[i].Path == path && !routes[i].IsAutoHead() {
			return routes[i]
		}
	}
	require.Failf(t, "route not found", "%s %s", method, path)
	return fiber.Route{}
}

// opFor returns the Operation the route registered for method and path was documented with; it must have one.
func opFor(t *testing.T, app *fiber.App, method, path string) *Operation {
	t.Helper()
	route := routeFor(t, app, method, path)
	op := OperationOf(&route)
	require.NotNil(t, op, "%s %s is not documented", method, path)
	return op
}

type docModelIn struct {
	Name string `json:"name"`
}

type docModelOut struct {
	ID int `json:"id"`
}

type docModelQuery struct {
	Page int `query:"page"`
}

// routerOn registers "/users/:id" for one method on r.
func routerOn(r fiber.Router) func(method string) fiber.Router {
	return func(method string) fiber.Router { return r.Add([]string{method}, "/users/:id", omOK) }
}

// routeChainer is what App, Group, the Domain router and a route chain share to start a route chain.
type routeChainer interface {
	RouteChain(path string) fiber.Register
}

// chainOn registers "/users/:id" for one method through a route chain of r.
func chainOn(r routeChainer) func(method string) fiber.Register {
	return func(method string) fiber.Register { return r.RouteChain("/users/:id").Add([]string{method}, omOK) }
}

// documentEverything calls every documentation helper, giving the helpers that replace each other a method of
// their own.
func documentEverything[R interface{ Name(name string) R }](on func(method string) R) {
	Doc(on(fiber.MethodPost).Name("createUser")).Summary("Create a user").Description("Creates a new user").
		Consumes(fiber.MIMEApplicationJSON).Produces(fiber.MIMEApplicationXML).
		Parameter("id", "path", false, map[string]any{"type": "integer"}, "identifier").
		Parameter("filter", "query", true, nil, "Filter results").
		ParameterWithExample("lang", "query", false, nil, "#/components/schemas/Lang", "language", "en", map[string]any{"sample": "en"}).
		AddParameter(RouteParameter{Name: "limit", In: "query", Schema: map[string]any{"type": "integer"}}).
		Response(fiber.StatusCreated, "Created", fiber.MIMEApplicationJSON).
		Response(0, "Default fallback").
		ResponseWithExample(fiber.StatusAccepted, "Accepted", nil, "#/components/schemas/User", map[string]any{"id": 1}, map[string]any{"sample": map[string]any{"id": 2}}, fiber.MIMEApplicationJSON).
		ResponseHeader(fiber.StatusCreated, "Location", "resource url", nil).
		ResponseContent(fiber.StatusOK, "OK", map[string]RouteMediaType{fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "array"}}}).
		ResponseLink(fiber.StatusCreated, "self", map[string]any{"operationId": "createUser"}).
		Tags("users", "write").Deprecated().Security(map[string][]string{"bearerAuth": {}}).Hidden().
		OperationExternalDocs("docs", "https://example.com/docs").
		OperationExtension(map[string]any{"x-team": "core"}).
		OperationID("createUser")
	Doc(on(fiber.MethodPut)).RequestBody("User payload", true, fiber.MIMEApplicationJSON, fiber.MIMEApplicationXML).Response(fiber.StatusOK, "OK", fiber.MIMEApplicationJSON)
	Doc(on(fiber.MethodPatch)).RequestBodyWithExample("payload", true, map[string]any{"type": "object"}, "#/components/schemas/User",
		map[string]any{"name": "doe"}, map[string]any{"sample": map[string]any{"name": "john"}}, fiber.MIMEApplicationJSON).
		ResponseWithExample(fiber.StatusOK, "ok", nil, "", map[string]any{"id": 1}, nil, fiber.MIMEApplicationJSON)
	Doc(on(fiber.MethodDelete)).RequestBodyContent("User", true, map[string]RouteMediaType{fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}}})
	Doc(on(fiber.MethodOptions)).Accepts(docModelIn{}).Returns(fiber.StatusCreated, docModelOut{}).Params("query", docModelQuery{})
}

func Test_DocHelpers_OnEveryRouter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		document func(app *fiber.App)
		name     string
		path     string
		handlers int
	}{
		{name: "app", path: "/users/:id", handlers: 1, document: func(app *fiber.App) { documentEverything(routerOn(app)) }},
		{name: "group", path: "/api/users/:id", handlers: 1, document: func(app *fiber.App) { documentEverything(routerOn(app.Group("/api"))) }},
		{name: "domain", path: "/users/:id", handlers: 1, document: func(app *fiber.App) { documentEverything(routerOn(app.Domain("api.example.com"))) }},
		{name: "app route chain", path: "/users/:id", handlers: 1, document: func(app *fiber.App) { documentEverything(chainOn(app)) }},
		{name: "group route chain", path: "/api/users/:id", handlers: 1, document: func(app *fiber.App) { documentEverything(chainOn(app.Group("/api"))) }},
		{name: "domain route chain", path: "/users/:id", handlers: 1, document: func(app *fiber.App) { documentEverything(chainOn(app.Domain("api.example.com"))) }},
		{name: "nested route chain", path: "/api/users/:id", handlers: 1, document: func(app *fiber.App) { documentEverything(chainOn(app.RouteChain("/api"))) }},
		{name: "merged registration", path: "/users/:id", handlers: 2, document: func(app *fiber.App) {
			app.Post("/users/:id", omOK)
			documentEverything(routerOn(app))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			tc.document(app)
			stored := func(method string) fiber.Route {
				var found []fiber.Route
				for _, route := range app.GetRoutes() {
					if route.Method == method && !route.IsAutoHead() {
						found = append(found, route)
					}
				}
				require.Len(t, found, 1, method)
				require.Equal(t, tc.path, found[0].Path, method)
				return found[0]
			}
			storedOp := func(method string) *Operation {
				route := stored(method)
				op := OperationOf(&route)
				require.NotNil(t, op, method)
				return op
			}

			postRoute := stored(fiber.MethodPost)
			require.Len(t, postRoute.Handlers, tc.handlers)
			require.Equal(t, "createUser", postRoute.Name)
			post := storedOp(fiber.MethodPost)
			require.Equal(t, "Create a user", post.Summary)
			require.Equal(t, "Creates a new user", post.Description)
			require.Equal(t, fiber.MIMEApplicationJSON, post.Consumes)
			require.Equal(t, fiber.MIMEApplicationXML, post.Produces, "an explicit Produces wins over the responses")
			require.Equal(t, []RouteParameter{
				{Name: "id", In: ParamInPath, Required: true, Schema: map[string]any{"type": "integer"}, Description: "identifier"},
				{Name: "filter", In: ParamInQuery, Required: true, Schema: map[string]any{"type": "string"}, Description: "Filter results"},
				{
					Name: "lang", In: ParamInQuery, SchemaRef: "#/components/schemas/Lang", Schema: map[string]any{"$ref": "#/components/schemas/Lang"},
					Description: "language", Example: "en", Examples: map[string]any{"sample": "en"},
				},
				{Name: "limit", In: ParamInQuery, Schema: map[string]any{"type": "integer"}},
			}, post.Parameters)
			require.Equal(t, map[string]RouteResponse{
				"200": {Description: "OK", Content: map[string]RouteMediaType{fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "array"}}}},
				"201": {
					Description: "Created", MediaTypes: []string{fiber.MIMEApplicationJSON},
					Headers: map[string]any{"Location": map[string]any{"description": "resource url", "schema": map[string]any{"type": "string"}}},
					Links:   map[string]any{"self": map[string]any{"operationId": "createUser"}},
				},
				"202": {
					Description: "Accepted", MediaTypes: []string{fiber.MIMEApplicationJSON}, SchemaRef: "#/components/schemas/User",
					Schema: map[string]any{"$ref": "#/components/schemas/User"}, Example: map[string]any{"id": 1},
					Examples: map[string]any{"sample": map[string]any{"id": 2}},
				},
				"default": {Description: "Default fallback"},
			}, post.Responses)
			require.Equal(t, []string{"users", "write"}, post.Tags)
			require.True(t, post.Deprecated)
			require.Equal(t, []map[string][]string{{"bearerAuth": {}}}, post.Security)
			require.True(t, post.Hidden)
			require.Equal(t, map[string]any{"description": "docs", "url": "https://example.com/docs"}, post.ExternalDocs)
			require.Equal(t, map[string]any{"x-team": "core"}, post.OperationExtensions)
			require.Equal(t, "createUser", post.OperationID)

			put := storedOp(fiber.MethodPut)
			require.Equal(t, &RouteRequestBody{Description: "User payload", Required: true, MediaTypes: []string{fiber.MIMEApplicationJSON, fiber.MIMEApplicationXML}}, put.RequestBody)
			require.Equal(t, fiber.MIMEApplicationJSON, put.Consumes, "the first body media type")
			require.Equal(t, fiber.MIMEApplicationJSON, put.Produces, "the first response media type")
			patch := storedOp(fiber.MethodPatch)
			require.Equal(t, fiber.MIMEApplicationJSON, patch.Produces, "the first example response media type")
			require.Equal(t, &RouteRequestBody{
				Description: "payload", Required: true, MediaTypes: []string{fiber.MIMEApplicationJSON}, SchemaRef: "#/components/schemas/User",
				Schema: map[string]any{"$ref": "#/components/schemas/User"}, Example: map[string]any{"name": "doe"},
				Examples: map[string]any{"sample": map[string]any{"name": "john"}},
			}, patch.RequestBody)
			require.Equal(t, &RouteRequestBody{
				Description: "User", Required: true, Content: map[string]RouteMediaType{fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}}},
			}, storedOp(fiber.MethodDelete).RequestBody)

			models := storedOp(fiber.MethodOptions)
			require.Equal(t, &RouteRequestBody{Schema: docModelIn{}, Required: true}, models.RequestBody)
			require.Equal(t, RouteResponse{Description: "Created", Schema: docModelOut{}}, models.Responses["201"])
			require.Equal(t, []RouteParameterModel{{In: "query", Model: docModelQuery{}}}, models.ParameterModels)
		})
	}
}

func Test_Doc_PanicsForANonRouter(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		arg  any
		want string
	}{
		{arg: 42, want: "openapi: Doc needs the router a route registration returned, got int"},
		{arg: nil, want: "openapi: Doc needs the router a route registration returned, got <nil>"},
		{arg: fiber.Route{}, want: "openapi: Doc needs the router a route registration returned, got fiber.Route"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			require.PanicsWithValue(t, tc.want, func() { Doc(tc.arg) })
		})
	}
}

func Test_Doc_SnapshotKeepsItsOperation(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	route := app.Get("/users/:id", omOK)
	Doc(route).Summary("Get a user").Tags("users").
		Parameter("id", ParamInPath, true, nil, "").
		Response(fiber.StatusOK, "ok", fiber.MIMEApplicationJSON).
		ResponseHeader(fiber.StatusOK, "X-Trace", "", nil)
	snapshot := routeFor(t, app, fiber.MethodGet, "/users/:id")

	Doc(route).Tags("users", "admin").
		AddParameter(RouteParameter{Name: "q", In: ParamInQuery}).
		Response(fiber.StatusNotFound, "missing").
		ResponseHeader(fiber.StatusOK, "X-Rate-Limit", "requests left", nil)

	// A copy taken earlier keeps the Operation it had.
	kept := OperationOf(&snapshot)
	require.NotNil(t, kept)
	require.Equal(t, []string{"users"}, kept.Tags)
	require.Len(t, kept.Parameters, 1)
	require.Len(t, kept.Responses, 1)
	require.Contains(t, kept.Responses, "200")
	require.Equal(t, map[string]any{"X-Trace": map[string]any{"schema": map[string]any{"type": "string"}}}, kept.Responses["200"].Headers)
	fresh := opFor(t, app, fiber.MethodGet, "/users/:id")
	require.Equal(t, []string{"users", "admin"}, fresh.Tags)
	require.Len(t, fresh.Parameters, 2)
	require.Contains(t, fresh.Responses, "404")
	require.Contains(t, fresh.Responses["200"].Headers, "X-Rate-Limit")
	require.Contains(t, fresh.Responses["200"].Headers, "X-Trace")
}

func Test_RouteDocs_MultiMethodRegistration(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Add([]string{fiber.MethodPut, fiber.MethodPost}, "/multi", omOK)).
		Summary("multi summary").
		Security(map[string][]string{"auth": {}}).
		Response(fiber.StatusCreated, "created", fiber.MIMEApplicationJSON)

	for _, method := range []string{fiber.MethodPut, fiber.MethodPost} {
		op := opFor(t, app, method, "/multi")
		require.Equal(t, "multi summary", op.Summary, method)
		require.Len(t, op.Security, 1, method)
		require.Contains(t, op.Responses, "201", method)
	}
}

func Test_CopyAnyMap(t *testing.T) {
	t.Parallel()
	t.Run("nil interface elements", func(t *testing.T) {
		t.Parallel()
		var nilStringer fmt.Stringer
		src := map[string]any{"slice": []fmt.Stringer{nilStringer}, "map": map[string]fmt.Stringer{"k": nil}}
		require.NotPanics(t, func() { require.Len(t, copyAnyMap(src), 2) })
	})

	t.Run("deep copy", func(t *testing.T) {
		t.Parallel()
		newSrc := func() map[string]any {
			return map[string]any{
				"properties": map[string]any{"id": map[string]any{"type": "string"}},
				"required":   []any{"id"},
				"names":      []string{"id"},
				"oneOf":      []map[string]any{{"type": "string"}},
			}
		}
		src := newSrc()
		cloned := copyAnyMap(src)
		cloned["properties"].(map[string]any)["id"].(map[string]any)["type"] = "integer" //nolint:errcheck,forcetypeassert // built just above
		cloned["required"].([]any)[0] = "changed"                                        //nolint:errcheck,forcetypeassert // built just above
		cloned["names"].([]string)[0] = "changed"                                        //nolint:errcheck,forcetypeassert // built just above
		cloned["oneOf"].([]map[string]any)[0]["type"] = "integer"                        //nolint:errcheck,forcetypeassert // built just above
		require.Equal(t, newSrc(), src, "the copy shares no nested map or slice, typed or not")
	})

	t.Run("nested empty maps are kept", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		Doc(app.Post("/x", omOK)).ResponseWithExample(fiber.StatusOK, "", map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}, "", []any{map[string]any{}}, nil, fiber.MIMEApplicationJSON)

		resp := opFor(t, app, fiber.MethodPost, "/x").Responses["200"]
		require.Equal(t, map[string]any{}, requireMap(t, resp.Schema)["properties"])
		example, ok := resp.Example.([]any)
		require.True(t, ok)
		require.Equal(t, map[string]any{}, example[0])

		require.Nil(t, copyAnyMap(map[string]any{}), "a top-level empty map still reads as unset")
	})
}

func Test_CopySchema_ClonesNamedMapsAsRawSchemasAndSlices(t *testing.T) {
	t.Parallel()
	type rawSchema map[string]any
	named := rawSchema{"type": "object", "properties": map[string]any{"a": "x"}}
	copied, ok := copySchema(named).(map[string]any)
	require.True(t, ok, "a defined map type with string keys is a raw schema and is stored as a plain map")
	named["properties"].(map[string]any)["a"] = "changed"             //nolint:errcheck,forcetypeassert // built just above
	require.Equal(t, "x", copied["properties"].(map[string]any)["a"]) //nolint:errcheck,forcetypeassert // built just above

	require.Nil(t, copySchema(map[string]any{}), "an empty schema map reads as no schema")
	type model struct{ Name string }
	require.Equal(t, model{Name: "n"}, copySchema(model{Name: "n"}))
}

func Test_ParamLocation_Normalize(t *testing.T) {
	t.Parallel()
	require.Equal(t, ParamInQuery, ParamLocation(" Query ").Normalize())
	require.Equal(t, ParamInHeader, ParamLocation("HEADER").Normalize())
	require.Equal(t, ParamLocation(""), ParamLocation("  ").Normalize())
}

func Test_Security_EmptyScopesStayNonNil(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Get("/secure", omOK)).Security(map[string][]string{"bearerAuth": {}})

	op := opFor(t, app, fiber.MethodGet, "/secure")
	require.Len(t, op.Security, 1)
	scopes, ok := op.Security[0]["bearerAuth"]
	require.True(t, ok)
	require.NotNil(t, scopes)
	require.Empty(t, scopes)
}

func Test_Security_NoArgumentsDocumentsNoAuth(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	none := app.Get("/none", omOK)
	Doc(none).Security()
	Doc(app.Get("/some", omOK)).Security(map[string][]string{"a": {"s"}})
	Doc(app.Get("/silent", omOK)).Summary("no security")

	require.NotNil(t, opFor(t, app, fiber.MethodGet, "/none").Security)
	require.Empty(t, opFor(t, app, fiber.MethodGet, "/none").Security)
	require.Len(t, opFor(t, app, fiber.MethodGet, "/some").Security, 1)
	require.Nil(t, opFor(t, app, fiber.MethodGet, "/silent").Security)

	Doc(none).Summary("later")
	require.NotNil(t, opFor(t, app, fiber.MethodGet, "/none").Security, "a later helper keeps the empty requirement list")
}

func Test_ExampleFields_DetachedFromTheCaller(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	example := map[string]any{"name": "original"}
	Doc(app.Post("/x", omOK)).RequestBodyWithExample("body", true, nil, "", example, nil, fiber.MIMEApplicationJSON)

	example["name"] = "caller-mutated"
	body := opFor(t, app, fiber.MethodPost, "/x").RequestBody
	require.NotNil(t, body)
	require.Equal(t, map[string]any{"name": "original"}, body.Example)
}

func Test_DocMetadata_CyclicValueDoesNotCrash(t *testing.T) {
	t.Parallel()
	cyclic := map[string]any{"k": 1}
	cyclic["self"] = cyclic

	app := fiber.New()
	require.NotPanics(t, func() {
		Doc(app.Get("/f", omOK)).ResponseWithExample(fiber.StatusOK, "ok", nil, "", cyclic, nil, fiber.MIMEApplicationJSON)
		require.NotEmpty(t, app.GetRoutes())
	})
}

func Test_AddParameter_ExampleDetached(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	example := map[string]any{"nested": map[string]any{"k": "original"}}
	Doc(app.Get("/p", omOK)).AddParameter(RouteParameter{Name: "q", In: "query", Example: example})

	nested, ok := example["nested"].(map[string]any)
	require.True(t, ok)
	nested["k"] = "mutated"
	example["added"] = true

	stored := opFor(t, app, fiber.MethodGet, "/p").Parameters[0].Example
	require.Equal(t, map[string]any{"nested": map[string]any{"k": "original"}}, stored)
}

func Test_AddParameter_SchemaSelection(t *testing.T) {
	t.Parallel()
	explode, stillTrue := true, true
	app := fiber.New()
	Doc(app.Get("/items/:id", omOK)).
		AddParameter(RouteParameter{
			Name: "filter", In: "query", Schema: map[string]any{"type": "string"}, SchemaRef: "#/components/schemas/Filter",
			Content: map[string]RouteMediaType{fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}}},
		}).
		AddParameter(RouteParameter{Name: "ref", In: "query", SchemaRef: "#/components/schemas/ID"}).
		AddParameter(RouteParameter{Name: "qs", In: "querystring"}).
		AddParameter(RouteParameter{Name: "plain", In: "query"}).
		AddParameter(RouteParameter{Name: "id", In: "path"}).
		AddParameter(RouteParameter{Name: "ids", In: "query", Explode: &explode})
	explode = false // the stored flag is a copy, not an alias

	require.Equal(t, []RouteParameter{
		{Name: "filter", In: ParamInQuery, Content: map[string]RouteMediaType{fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}}}},
		{Name: "ref", In: ParamInQuery, SchemaRef: "#/components/schemas/ID", Schema: map[string]any{"$ref": "#/components/schemas/ID"}},
		{Name: "qs", In: ParamInQuerystring},
		{Name: "plain", In: ParamInQuery, Schema: map[string]any{"type": "string"}},
		{Name: "id", In: ParamInPath, Required: true, Schema: map[string]any{"type": "string"}},
		{Name: "ids", In: ParamInQuery, Schema: map[string]any{"type": "string"}, Explode: &stillTrue},
	}, opFor(t, app, fiber.MethodGet, "/items/:id").Parameters)
}

func Test_ResponseExample_NotSharedAcrossBatch(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Add([]string{fiber.MethodGet, fiber.MethodPost}, "/e", omOK)).
		ResponseWithExample(fiber.StatusOK, "ok", map[string]any{"type": "object"}, "", map[string]any{"k": "v"}, nil, fiber.MIMEApplicationJSON)

	var examples []map[string]any
	for _, method := range []string{fiber.MethodGet, fiber.MethodPost} {
		example, ok := opFor(t, app, method, "/e").Responses["200"].Example.(map[string]any)
		require.True(t, ok, method)
		examples = append(examples, example)
	}
	examples[0]["k"] = "changed"
	require.Equal(t, "v", examples[1]["k"], "each route of the batch holds its own copy")
}

func Test_DuplicateMethodRegistration_DocumentedOnce(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Add([]string{fiber.MethodGet, fiber.MethodGet}, "/d", omOK)).Parameter("q", "query", false, nil, "")
	require.Len(t, opFor(t, app, fiber.MethodGet, "/d").Parameters, 1)
}

func Test_ContentHelpers_KeyByTrimmedMediaType(t *testing.T) {
	t.Parallel()
	t.Run("padded keys are trimmed", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		Doc(app.Post("/a", omOK)).
			RequestBodyContent("payload", true, map[string]RouteMediaType{" " + fiber.MIMEApplicationJSON + " ": {}}).
			ResponseContent(fiber.StatusOK, "ok", map[string]RouteMediaType{"\t" + fiber.MIMEApplicationXML: {}})

		op := opFor(t, app, fiber.MethodPost, "/a")
		require.Contains(t, op.RequestBody.Content, fiber.MIMEApplicationJSON)
		require.Contains(t, op.Responses["200"].Content, fiber.MIMEApplicationXML)
	})

	t.Run("empty content is left alone", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		Doc(app.Post("/d", omOK)).RequestBodyContent("payload", true, nil)
		require.Empty(t, opFor(t, app, fiber.MethodPost, "/d").RequestBody.Content)
	})

	t.Run("parameter content is trimmed", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		Doc(app.Get("/c", omOK)).AddParameter(RouteParameter{
			Name:    "filter",
			In:      "query",
			Content: map[string]RouteMediaType{" " + fiber.MIMEApplicationJSON: {}},
		})
		require.Contains(t, opFor(t, app, fiber.MethodGet, "/c").Parameters[0].Content, fiber.MIMEApplicationJSON)
	})
}

func Test_ResponseHeader_DefaultsToStringSchema(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Get("/a", omOK)).
		ResponseHeader(fiber.StatusOK, "X-Rate-Limit", "requests left", nil).
		ResponseHeader(fiber.StatusOK, "X-Trace", "", map[string]any{"type": "integer"})

	headers := opFor(t, app, fiber.MethodGet, "/a").Responses["200"].Headers
	require.Equal(t, map[string]any{"description": "requests left", "schema": map[string]any{"type": "string"}}, headers["X-Rate-Limit"])
	require.Equal(t, map[string]any{"schema": map[string]any{"type": "integer"}}, headers["X-Trace"])
}

// selfPointer is a pointer type that points at itself, so following it never reaches a struct.
type selfPointer *selfPointer

func Test_DocHelpers_RejectInvalidInput(t *testing.T) {
	t.Parallel()
	// rejects runs document on a fresh route and expects it to panic with want, or with anything when want is nil.
	rejects := func(name string, want any, document func(d *RouteDoc)) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			doc := Doc(fiber.New().Get("/:id", omOK))
			if want == nil {
				require.Panics(t, func() { document(doc) })
				return
			}
			require.PanicsWithValue(t, want, func() { document(doc) })
		})
	}
	invalid := map[string]RouteMediaType{"invalid": {}}
	collide := map[string]RouteMediaType{fiber.MIMEApplicationJSON: {}, " " + fiber.MIMEApplicationJSON: {}}
	twoTypes := RouteParameter{Name: "f", In: "query", Content: map[string]RouteMediaType{fiber.MIMEApplicationJSON: {}, fiber.MIMEApplicationXML: {}}}
	noURL := "OperationServer: url is required"

	rejects("Produces media type", nil, func(d *RouteDoc) { d.Produces("invalid") })
	rejects("Consumes media type", nil, func(d *RouteDoc) { d.Consumes("invalid") })
	rejects("RequestBody media type", nil, func(d *RouteDoc) { d.RequestBody("payload", true, "invalid") })
	rejects("Response media type", nil, func(d *RouteDoc) { d.Response(fiber.StatusOK, "", "invalid") })
	rejects("Response status", nil, func(d *RouteDoc) { d.Response(42, "invalid") })
	rejects("ResponseHeader name", nil, func(d *RouteDoc) { d.ResponseHeader(fiber.StatusOK, "  ", "blank", nil) })
	rejects("RequestBodyContent media type", nil, func(d *RouteDoc) { d.RequestBodyContent("payload", true, invalid) })
	rejects("RequestBodyContent keys that collide trimmed", nil, func(d *RouteDoc) { d.RequestBodyContent("p", true, collide) })
	rejects("ResponseContent media type", nil, func(d *RouteDoc) { d.ResponseContent(fiber.StatusOK, "ok", invalid) })
	rejects("AddParameter blank name", nil, func(d *RouteDoc) { d.AddParameter(RouteParameter{Name: "  ", In: "query"}) })
	rejects("AddParameter location", nil, func(d *RouteDoc) { d.AddParameter(RouteParameter{Name: "x", In: "body"}) })
	rejects("AddParameter two media types", nil, func(d *RouteDoc) { d.AddParameter(twoTypes) })
	rejects("AddParameter media type", nil, func(d *RouteDoc) { d.AddParameter(RouteParameter{Name: "f", In: "query", Content: invalid}) })
	rejects("Parameter location", "AddParameter: invalid parameter location: body", func(d *RouteDoc) { d.Parameter("q", "body", false, nil, "") })
	rejects("Params location", "Params: invalid parameter location: body", func(d *RouteDoc) { d.Params("body", docModelQuery{}) })
	rejects("Params non-struct model", nil, func(d *RouteDoc) { d.Params("query", 42) })
	rejects("Params nil model", nil, func(d *RouteDoc) { d.Params("query", nil) })
	rejects("Params pointer that never reaches a struct", nil, func(d *RouteDoc) { d.Params("query", new(selfPointer)) })
	rejects("OperationExternalDocs url", "OperationExternalDocs: url is required", func(d *RouteDoc) { d.OperationExternalDocs("More info", " ") })
	rejects("OperationID", "OperationID: id is required", func(d *RouteDoc) { d.OperationID("  ") })
	rejects("Callback name", "Callback: name is required", func(d *RouteDoc) { d.Callback(" ", map[string]any{"e": map[string]any{}}) })
	rejects("Callback expressions", "Callback: done needs at least one expression", func(d *RouteDoc) { d.Callback("done", nil) })
	rejects("OperationServer without url", noURL, func(d *RouteDoc) { d.OperationServer(map[string]any{"description": "no url"}) })
	rejects("OperationServer blank url", noURL, func(d *RouteDoc) { d.OperationServer(map[string]any{"url": " "}) })
}

func Test_Params_NormalizesLocation(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Get("/d", omOK)).Params(" Header ", &docModelQuery{})
	require.Equal(t, []RouteParameterModel{{In: "header", Model: &docModelQuery{}}}, opFor(t, app, fiber.MethodGet, "/d").ParameterModels)
}

func Test_RequestBody_WithoutMediaTypesInfersNone(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Post("/", omOK)).RequestBody("payload", true)
	op := opFor(t, app, fiber.MethodPost, "/")
	require.NotNil(t, op.RequestBody)
	require.Empty(t, op.RequestBody.MediaTypes)
	require.Empty(t, op.Consumes)
}

func Test_Parameter_ModelSchemaKeepsItsType(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Get("/a", omOK)).
		Parameter("filter", "query", false, docModelQuery{}, "").
		Parameter("plain", "query", false, nil, "")
	params := opFor(t, app, fiber.MethodGet, "/a").Parameters
	require.Equal(t, docModelQuery{}, params[0].Schema)
	require.Equal(t, map[string]any{"type": "string"}, params[1].Schema)
}

func Test_Parameter_DefaultTypeFollowsSchema(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Get("/p", omOK)).
		Parameter("plain", ParamInQuery, false, map[string]any{}, "").
		Parameter("ints", ParamInQuery, false, map[string]any{"enum": []int{1, 2}}, "").
		Parameter("either", ParamInQuery, false, map[string]any{"oneOf": []any{map[string]any{"type": "integer"}, map[string]any{"type": "string"}}}, "").
		Parameter("linked", ParamInQuery, false, map[string]any{"$ref": "#/components/schemas/X"}, "")

	schemas := map[string]map[string]any{}
	for _, param := range opFor(t, app, fiber.MethodGet, "/p").Parameters {
		schemas[param.Name] = requireMap(t, param.Schema)
	}
	require.Equal(t, "string", schemas["plain"]["type"])
	require.Equal(t, "integer", schemas["ints"]["type"])
	require.NotContains(t, schemas["either"], "type")
	require.NotContains(t, schemas["linked"], "type")
}

func Test_MediaTypeSetters_TreatWhitespaceAsEmpty(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	doc := Doc(app.Get("/x", omOK)).Consumes(fiber.MIMEApplicationJSON).Produces(fiber.MIMEApplicationJSON)
	require.NotPanics(t, func() { doc.Consumes("  ").Produces("\t") })
	op := opFor(t, app, fiber.MethodGet, "/x")
	require.Empty(t, op.Consumes)
	require.Empty(t, op.Produces)
}

func Test_OperationExtension_NilIsNoOp(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/a", omOK)

	require.NotPanics(t, func() { Doc(app).OperationExtension(nil) })
	require.NotPanics(t, func() { Doc(app).OperationExtension(map[string]any{}) })
	apptest.Bounded(t, boundedTimeout, "GetRoutes after a nil mutation", func() { _ = app.GetRoutes() })
	route := routeFor(t, app, fiber.MethodGet, "/a")
	require.Nil(t, OperationOf(&route), "nothing was documented")
}

func Test_OperationExtension_NilElements(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	require.NotPanics(t, func() {
		Doc(app.Get("/", omOK)).OperationExtension(map[string]any{"x-errs": []error{nil}, "x-map": map[string]error{"e": nil}})
	})

	extensions := opFor(t, app, fiber.MethodGet, "/").OperationExtensions
	errs, ok := extensions["x-errs"].([]error)
	require.True(t, ok)
	require.Len(t, errs, 1)
	m, ok := extensions["x-map"].(map[string]error)
	require.True(t, ok)
	require.Contains(t, m, "e")
}

func Test_Response_KeepsHeadersAndLinks(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Get("/", omOK)).
		ResponseHeader(fiber.StatusOK, "X-RateLimit", "requests remaining", nil).
		ResponseLink(fiber.StatusOK, "next", map[string]any{"operationId": "getNext"}).
		Response(fiber.StatusOK, "OK", fiber.MIMEApplicationJSON)

	resp, ok := opFor(t, app, fiber.MethodGet, "/").Responses["200"]
	require.True(t, ok)
	require.Equal(t, "OK", resp.Description)
	require.Equal(t, []string{fiber.MIMEApplicationJSON}, resp.MediaTypes)
	require.Contains(t, resp.Headers, "X-RateLimit")
	require.Contains(t, resp.Links, "next")
}

func Test_ExplicitMediaTypesWinOverInferredOnes(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Get("/explicit", omOK)).Produces(fiber.MIMEApplicationJSON).Response(fiber.StatusOK, "OK", "text/csv")
	Doc(app.Get("/inferred", omOK)).Response(fiber.StatusOK, "OK", "text/csv")
	Doc(app.Post("/explicit", omOK)).Consumes(fiber.MIMEApplicationJSON).RequestBody("payload", true, fiber.MIMEApplicationXML)

	require.Equal(t, fiber.MIMEApplicationJSON, opFor(t, app, fiber.MethodGet, "/explicit").Produces)
	require.Equal(t, "text/csv", opFor(t, app, fiber.MethodGet, "/inferred").Produces)
	require.Equal(t, fiber.MIMEApplicationJSON, opFor(t, app, fiber.MethodPost, "/explicit").Consumes)
}

func Test_Returns_KeepsEarlierDescriptionAndReplacesContent(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Get("/u", omOK)).
		ResponseContent(fiber.StatusOK, "List of users", map[string]RouteMediaType{fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "string"}}}).
		Returns(fiber.StatusOK, []string{}, fiber.MIMEApplicationXML)

	resp := opFor(t, app, fiber.MethodGet, "/u").Responses["200"]
	require.Equal(t, "List of users", resp.Description)
	require.Empty(t, resp.Content, "a body documented later replaces the earlier content map")
	require.Equal(t, []string{fiber.MIMEApplicationXML}, resp.MediaTypes)

	Doc(app.Get("/v", omOK)).Response(fiber.StatusOK, "Explicit", fiber.MIMEApplicationJSON).Response(fiber.StatusOK, "", fiber.MIMEApplicationXML)
	require.Equal(t, "Explicit", opFor(t, app, fiber.MethodGet, "/v").Responses["200"].Description)
}

func Test_Accepts_KeepsEarlierBodyDescription(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Post("/u", omOK)).RequestBody("The user to create", true).Accepts(struct{ Name string }{})

	body := opFor(t, app, fiber.MethodPost, "/u").RequestBody
	require.NotNil(t, body)
	require.Equal(t, "The user to create", body.Description)
}

func Test_Response_LaterCallsKeepTheBodyUnlessTheyDocumentOne(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Get("/u", omOK)).Returns(fiber.StatusOK, struct{ A int }{}).Response(fiber.StatusOK, "ok")

	resp := opFor(t, app, fiber.MethodGet, "/u").Responses["200"]
	require.NotNil(t, resp.Schema, "a description alone keeps the model Returns documented")
	require.Equal(t, "ok", resp.Description)

	Doc(app.Get("/c", omOK)).
		ResponseContent(fiber.StatusOK, "", map[string]RouteMediaType{fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "string"}}}).
		Response(fiber.StatusOK, "", fiber.MIMEApplicationJSON)
	require.NotEmpty(t, opFor(t, app, fiber.MethodGet, "/c").Responses["200"].Content, "media types alone keep the content map")
}

func Test_RequestBody_LaterCallsKeepWhatTheyDoNotDocument(t *testing.T) {
	t.Parallel()
	model := struct{ Name string }{}
	content := map[string]RouteMediaType{fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "string"}}}

	app := fiber.New()
	Doc(app.Post("/a", omOK)).Accepts(model).RequestBody("User to create", true)
	body := opFor(t, app, fiber.MethodPost, "/a").RequestBody
	require.NotNil(t, body)
	require.NotNil(t, body.Schema, "a description alone keeps the model Accepts documented")
	require.Equal(t, "User to create", body.Description)

	Doc(app.Post("/b", omOK)).RequestBody("Widget payload", true, fiber.MIMEApplicationJSON).RequestBodyContent("", true, content)
	require.Equal(t, "Widget payload", opFor(t, app, fiber.MethodPost, "/b").RequestBody.Description)

	Doc(app.Post("/c", omOK)).RequestBodyContent("D", true, content).RequestBody("", true)
	got := opFor(t, app, fiber.MethodPost, "/c").RequestBody
	require.Equal(t, "D", got.Description)
	require.NotEmpty(t, got.Content)

	Doc(app.Post("/d", omOK)).Accepts(model).Accepts(struct{ ID int }{})
	require.NotNil(t, opFor(t, app, fiber.MethodPost, "/d").RequestBody.Schema, "a new schema replaces the earlier one")
}

func Test_AutoHeadTwin_IsNeverHiddenWhenTheGetWasHiddenFirst(t *testing.T) {
	t.Parallel()
	for name, hideBeforeTwin := range map[string]bool{"hidden before the twin exists": true, "hidden after": false} {
		app := fiber.New()
		route := app.Get("/h", omOK)
		if hideBeforeTwin {
			Doc(route).Hidden()
		}
		app.RebuildTree()
		if !hideBeforeTwin {
			Doc(route).Hidden()
		}

		twins := 0
		for _, r := range app.GetRoutes() {
			if r.IsAutoHead() {
				twins++
				require.Nil(t, OperationOf(&r), name)
			}
		}
		require.Equal(t, 1, twins, name)
		require.True(t, opFor(t, app, fiber.MethodGet, "/h").Hidden, name)
	}
}

func Test_Route_OperationIDCallbackAndServer(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	callback := map[string]any{"{$request.body#/url}": map[string]any{"post": map[string]any{"responses": map[string]any{"200": map[string]any{"description": "ok"}}}}}
	server := map[string]any{"url": "https://uploads.example.com", "description": "uploads"}
	Doc(app.Post("/files", omOK).Name("upload")).OperationID(" uploadFile ").Callback("done", callback).OperationServer(server)

	op := opFor(t, app, fiber.MethodPost, "/files")
	require.Equal(t, "uploadFile", op.OperationID)
	require.Equal(t, callback, op.Callbacks["done"])
	require.Equal(t, []map[string]any{server}, op.Servers)

	// What the route keeps is a copy: changing the arguments afterwards does not reach it.
	callback["extra"] = true
	server["url"] = "changed"
	again := opFor(t, app, fiber.MethodPost, "/files")
	require.NotContains(t, again.Callbacks["done"], "extra")
	require.Equal(t, "https://uploads.example.com", again.Servers[0]["url"])
	named := app.GetRoute("upload")
	require.Equal(t, "uploadFile", OperationOf(&named).OperationID)
}
