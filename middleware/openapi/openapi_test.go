package openapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func Test_OpenAPI_Generate(t *testing.T) {
	app := fiber.New()

	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Post("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Contains(t, spec.Paths, "/users")
	operations := spec.Paths["/users"]
	require.Contains(t, operations, "get")
	require.Contains(t, operations, "post")
	getOp := requireMap(t, operations["get"])
	require.Contains(t, getOp, "responses")
	responses := requireMap(t, getOp["responses"])
	require.Contains(t, responses, "200")
}

func Test_OpenAPI_JSONEquality(t *testing.T) {
	app := fiber.New()

	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Name("listUsers").Produces(fiber.MIMEApplicationJSON)

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	expected := openAPISpec{
		OpenAPI: "3.1.0",
		Info:    openAPIInfo{Title: "Fiber API", Version: "1.0.0"},
		Paths: map[string]map[string]operation{
			"/users": {
				"get": {
					OperationID: "listUsers",
					Summary:     "GET /users",
					Description: "",
					Responses: map[string]response{
						"200": {Description: "OK", Content: map[string]map[string]any{fiber.MIMEApplicationJSON: {}}},
					},
				},
			},
		},
	}
	exp, err := json.Marshal(expected)
	require.NoError(t, err)
	require.JSONEq(t, string(exp), string(body))
}

func Test_OpenAPI_RawJSON(t *testing.T) {
	app := fiber.New()

	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Name("listUsers").Produces(fiber.MIMEApplicationJSON)

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	expected := openAPISpec{
		OpenAPI: "3.1.0",
		Info:    openAPIInfo{Title: "Fiber API", Version: "1.0.0"},
		Paths: map[string]map[string]operation{
			"/users": {
				"get": {
					OperationID: "listUsers",
					Summary:     "GET /users",
					Description: "",
					Responses: map[string]response{
						"200": {Description: "OK", Content: map[string]map[string]any{fiber.MIMEApplicationJSON: {}}},
					},
				},
			},
		},
	}
	exp, err := json.Marshal(expected)
	require.NoError(t, err)
	require.JSONEq(t, string(exp), string(body))
}

func Test_OpenAPI_RawJSONFile(t *testing.T) {
	app := fiber.New()

	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Name("listUsers").Produces(fiber.MIMEApplicationJSON)

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	expected, err := os.ReadFile("testdata/openapi.json")
	require.NoError(t, err)

	require.JSONEq(t, string(expected), string(body))
}

func Test_OpenAPI_RouteHelperMetadata(t *testing.T) {
	app := fiber.New()
	app.Post("/users", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"hello": "world"}) }).
		Name("createUserCustom").
		Summary("Create user").
		Description("Creates users").
		Tags("users").
		Deprecated().
		Consumes(fiber.MIMEApplicationJSON).
		Produces(fiber.MIMEApplicationJSON).
		ParameterWithExample("pageSize", "query", true, map[string]any{"type": "integer"}, "", "Page size", nil, nil).
		RequestBodyWithExample("Custom payload", true, map[string]any{"type": "object"}, "", nil, nil, fiber.MIMEApplicationJSON).
		ResponseWithExample(fiber.StatusCreated, "Created", map[string]any{"type": "object"}, "", nil, nil, fiber.MIMEApplicationJSON)

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))

	op := spec.Paths["/users"]["post"]
	require.Equal(t, "createUserCustom", op.OperationID)
	require.Equal(t, "Create user", op.Summary)
	require.Equal(t, "Creates users", op.Description)
	require.ElementsMatch(t, []string{"users"}, op.Tags)
	require.True(t, op.Deprecated)
	require.Len(t, op.Responses, 1)
	require.Contains(t, op.Responses, "201")
	require.Contains(t, op.Responses["201"].Content, fiber.MIMEApplicationJSON)
	require.NotNil(t, op.RequestBody)
	require.Equal(t, "Custom payload", op.RequestBody.Description)
	require.Contains(t, op.RequestBody.Content, fiber.MIMEApplicationJSON)
	require.True(t, op.RequestBody.Required)
	require.Len(t, op.Parameters, 1)
	require.Equal(t, "pageSize", op.Parameters[0].Name)
	require.Equal(t, "integer", op.Parameters[0].Schema["type"])
}

func Test_OpenAPI_RouteMetadata(t *testing.T) {
	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Summary("List users").Description("User list").Produces(fiber.MIMEApplicationJSON).
		Parameter("trace-id", "header", true, nil, "Tracing identifier").
		Tags("users", "read").Deprecated()

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))

	op := spec.Paths["/users"]["get"]
	require.Equal(t, "List users", op.Summary)
	require.Equal(t, "User list", op.Description)
	require.Contains(t, op.Responses["200"].Content, fiber.MIMEApplicationJSON)
	require.ElementsMatch(t, []string{"users", "read"}, op.Tags)
	require.True(t, op.Deprecated)
	require.Len(t, op.Parameters, 1)
	require.Equal(t, "trace-id", op.Parameters[0].Name)
	require.Equal(t, "header", op.Parameters[0].In)
	require.Equal(t, "Tracing identifier", op.Parameters[0].Description)
}

func Test_OpenAPI_RouteRequestBodyAndResponses(t *testing.T) {
	app := fiber.New()

	app.Post("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) }).
		RequestBody("Create user", true, fiber.MIMEApplicationJSON).
		Response(fiber.StatusCreated, "Created", fiber.MIMEApplicationJSON)

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))

	op := spec.Paths["/users"]["post"]
	require.NotNil(t, op.RequestBody)
	require.Equal(t, "Create user", op.RequestBody.Description)
	require.True(t, op.RequestBody.Required)
	require.Contains(t, op.RequestBody.Content, fiber.MIMEApplicationJSON)
	require.Contains(t, op.Responses, "201")
	require.Equal(t, "Created", op.Responses["201"].Description)
	require.Contains(t, op.Responses["201"].Content, fiber.MIMEApplicationJSON)
}

func Test_OpenAPI_DefaultResponses(t *testing.T) {
	t.Run("delete defaults to 204 with no content", func(t *testing.T) {
		app := fiber.New()
		app.Delete("/users/:id", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

		paths := getPaths(t, app)
		op := requireMap(t, paths["/users/{id}"]["delete"])
		responses := requireMap(t, op["responses"])
		require.Len(t, responses, 1)
		r204 := requireMap(t, responses["204"])
		require.Equal(t, "No Content", r204["description"])
		require.NotContains(t, r204, "content")
	})

	t.Run("post with explicit 201 does not add default 200", func(t *testing.T) {
		app := fiber.New()
		app.Post("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) }).
			Response(fiber.StatusCreated, "Created", fiber.MIMEApplicationJSON)

		paths := getPaths(t, app)
		op := requireMap(t, paths["/users"]["post"])
		responses := requireMap(t, op["responses"])
		require.Len(t, responses, 1)
		r201 := requireMap(t, responses["201"])
		require.Equal(t, "Created", r201["description"])
		require.Contains(t, requireMap(t, r201["content"]), fiber.MIMEApplicationJSON)
	})

	t.Run("non-200 responses remain untouched", func(t *testing.T) {
		app := fiber.New()
		app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNotFound) }).
			Response(fiber.StatusNotFound, "Not Found", fiber.MIMETextPlain)

		paths := getPaths(t, app)
		op := requireMap(t, paths["/users"]["get"])
		responses := requireMap(t, op["responses"])
		require.Len(t, responses, 1)
		r404 := requireMap(t, responses["404"])
		require.Equal(t, "Not Found", r404["description"])
		require.Contains(t, requireMap(t, r404["content"]), fiber.MIMETextPlain)
	})
}

func Test_OpenAPI_SchemaRefsAndExamples(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Post("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) }).
		ParameterWithExample("q", "query", false, nil, "#/components/schemas/Query", "search query", "abc", map[string]any{"sample": "abc"}).
		RequestBodyWithExample("user body", true, nil, "#/components/schemas/User", map[string]any{"name": "john"}, map[string]any{"sample": map[string]any{"name": "doe"}}, fiber.MIMEApplicationJSON).
		ResponseWithExample(fiber.StatusCreated, "Created", nil, "#/components/schemas/UserResponse", map[string]any{"id": 1}, map[string]any{"sample": map[string]any{"id": 2}}, fiber.MIMEApplicationJSON)

	paths := getPaths(t, app)
	op := requireMap(t, paths["/users"]["post"])

	params := requireSlice(t, op["parameters"])
	require.Len(t, params, 1)
	param := requireMap(t, params[0])
	require.Equal(t, "search query", param["description"])
	require.Equal(t, "abc", param["example"])
	require.Equal(t, map[string]any{"sample": "abc"}, requireMap(t, param["examples"]))
	paramSchema := requireMap(t, param["schema"])
	require.Equal(t, "#/components/schemas/Query", paramSchema["$ref"])

	body := requireMap(t, op["requestBody"])
	bodyContent := requireMap(t, body["content"])
	jsonContent := requireMap(t, bodyContent[fiber.MIMEApplicationJSON])
	bodySchema := requireMap(t, jsonContent["schema"])
	require.Equal(t, "#/components/schemas/User", bodySchema["$ref"])
	require.Equal(t, map[string]any{"name": "john"}, jsonContent["example"])
	require.Equal(t, map[string]any{"sample": map[string]any{"name": "doe"}}, requireMap(t, jsonContent["examples"]))

	resp := requireMap(t, requireMap(t, op["responses"])["201"])
	respContent := requireMap(t, resp["content"])
	respJSON := requireMap(t, respContent[fiber.MIMEApplicationJSON])
	respSchema := requireMap(t, respJSON["schema"])
	require.Equal(t, "#/components/schemas/UserResponse", respSchema["$ref"])
	require.Equal(t, map[string]any{"id": float64(1)}, respJSON["example"])
	require.Equal(t, map[string]any{"sample": map[string]any{"id": float64(2)}}, requireMap(t, respJSON["examples"]))
}

// getPaths is a helper that mounts the middleware, performs the request and
// decodes the resulting OpenAPI specification paths.
func getPaths(t *testing.T, app *fiber.App) map[string]map[string]any {
	t.Helper()

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	return spec.Paths
}

func Test_OpenAPI_Methods(t *testing.T) {
	handler := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }

	tests := []struct {
		register func(*fiber.App)
		method   string
	}{
		{func(a *fiber.App) { a.Get("/method", handler) }, fiber.MethodGet},
		{func(a *fiber.App) { a.Post("/method", handler) }, fiber.MethodPost},
		{func(a *fiber.App) { a.Put("/method", handler) }, fiber.MethodPut},
		{func(a *fiber.App) { a.Patch("/method", handler) }, fiber.MethodPatch},
		{func(a *fiber.App) { a.Delete("/method", handler) }, fiber.MethodDelete},
		{func(a *fiber.App) { a.Head("/method", handler) }, fiber.MethodHead},
		{func(a *fiber.App) { a.Options("/method", handler) }, fiber.MethodOptions},
		{func(a *fiber.App) { a.Trace("/method", handler) }, fiber.MethodTrace},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			app := fiber.New()
			tt.register(app)

			paths := getPaths(t, app)
			require.Contains(t, paths, "/method")
			ops := paths["/method"]
			require.Contains(t, ops, strings.ToLower(tt.method))
		})
	}
}

func Test_OpenAPI_DifferentHandlers(t *testing.T) {
	app := fiber.New()

	app.Get("/string", func(c fiber.Ctx) error { return c.SendString("a") })
	app.Get("/json", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"hello": "world"}) })

	paths := getPaths(t, app)

	require.Contains(t, paths, "/string")
	require.Contains(t, paths["/string"], "get")
	require.Contains(t, paths, "/json")
	require.Contains(t, paths["/json"], "get")
}

func Test_OpenAPI_Params(t *testing.T) {
	app := fiber.New()

	app.Get("/users/:id", func(c fiber.Ctx) error { return c.SendString(c.Params("id")) }).
		Parameter("id", "path", true, map[string]any{"type": "integer"}, "identifier")

	paths := getPaths(t, app)
	require.Contains(t, paths, "/users/{id}")
	require.Contains(t, paths["/users/{id}"], "get")
	op := requireMap(t, paths["/users/{id}"]["get"])
	params := requireSlice(t, op["parameters"])
	require.Len(t, params, 1)
	p0 := requireMap(t, params[0])
	require.Equal(t, "id", p0["name"])
	require.Equal(t, "path", p0["in"])
	require.Equal(t, "identifier", p0["description"])
	schema := requireMap(t, p0["schema"])
	require.Equal(t, "integer", schema["type"])
}

func Test_OpenAPI_Groups(t *testing.T) {
	app := fiber.New()

	api := app.Group("/api")
	api.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	api.Post("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })

	paths := getPaths(t, app)

	require.Contains(t, paths, "/api/users")
	ops := paths["/api/users"]
	require.Contains(t, ops, "get")
	require.Contains(t, ops, "post")
}

func Test_OpenAPI_Groups_Metadata(t *testing.T) {
	app := fiber.New()

	api := app.Group("/api")
	api.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Summary("List users").Description("Group users").Produces(fiber.MIMEApplicationJSON).
		Tags("users").Deprecated()

	paths := getPaths(t, app)

	require.Contains(t, paths, "/api/users")
	op := requireMap(t, paths["/api/users"]["get"])
	require.Equal(t, "List users", op["summary"])
	require.Equal(t, "Group users", op["description"])
	require.ElementsMatch(t, []any{"users"}, requireSlice(t, op["tags"]))
	require.Equal(t, true, op["deprecated"])
	resp := requireMap(t, op["responses"])
	cont := requireMap(t, requireMap(t, resp["200"])["content"])
	require.Contains(t, cont, fiber.MIMEApplicationJSON)
}

func Test_OpenAPI_NoRoutes(t *testing.T) {
	app := fiber.New()

	paths := getPaths(t, app)

	// Middleware routes registered via Use() are excluded, so an app with
	// only the openapi middleware has no paths in the generated spec.
	require.Empty(t, paths)
}

func Test_OpenAPI_RootOnly(t *testing.T) {
	app := fiber.New()

	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)

	require.Contains(t, paths, "/")
	require.Contains(t, paths["/"], "get")
}

func Test_OpenAPI_GroupMiddleware(t *testing.T) {
	app := fiber.New()

	api := app.Group("/api/v2")
	api.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	api.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/api/v2/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Contains(t, spec.Paths, "/api/v2/users")
}

func Test_OpenAPI_DoesNotInterceptSimilarPaths(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New())
	app.Get("/reports/openapi.json", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusAccepted) })

	req := httptest.NewRequest(fiber.MethodGet, "/reports/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusAccepted, resp.StatusCode)
}

func Test_OpenAPI_OnlyInterceptsGetAndHead(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New())
	app.Post("/openapi.json", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusAccepted) })

	req := httptest.NewRequest(fiber.MethodHead, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, fiber.MIMEApplicationJSONCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	req = httptest.NewRequest(fiber.MethodPost, "/openapi.json", http.NoBody)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusAccepted, resp.StatusCode)
}

func Test_OpenAPI_RootAndGroupSpecs(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{Title: "root"}))

	v1 := app.Group("/v1")
	v1.Use(New(Config{Title: "group"}))

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Equal(t, "root", spec.Info.Title)

	req = httptest.NewRequest(fiber.MethodGet, "/v1/openapi.json", http.NoBody)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Equal(t, "group", spec.Info.Title)
}

func Test_OpenAPI_ConfigValues(t *testing.T) {
	app := fiber.New()

	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	cfg := Config{
		Title:       "Custom API",
		Version:     "2.1.0",
		Description: "My description",
		ServerURL:   "https://example.com",
		Path:        "/spec.json",
	}
	app.Use(New(cfg))

	req := httptest.NewRequest(fiber.MethodGet, "/spec.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Equal(t, cfg.Title, spec.Info.Title)
	require.Equal(t, cfg.Version, spec.Info.Version)
	require.Equal(t, cfg.Description, spec.Info.Description)
	require.Len(t, spec.Servers, 1)
	require.Equal(t, cfg.ServerURL, spec.Servers[0].URL)
}

func Test_OpenAPI_Version310(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	cfg := Config{
		OpenAPIVersion: "3.1.0",
	}
	app.Use(New(cfg))

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Equal(t, "3.1.0", spec.OpenAPI)
}

func Test_OpenAPI_Version300(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	cfg := Config{
		OpenAPIVersion: "3.0.0",
	}
	app.Use(New(cfg))

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Equal(t, "3.0.0", spec.OpenAPI)
}

func Test_OpenAPI_VersionDefault(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	// No version specified, should default to 3.1.0
	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Equal(t, "3.1.0", spec.OpenAPI)
}

func Test_OpenAPI_VersionInvalid(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	// Invalid version should fall back to default 3.1.0
	cfg := Config{
		OpenAPIVersion: "2.0.0",
	}
	app.Use(New(cfg))

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Equal(t, "3.1.0", spec.OpenAPI)
}

func Test_OpenAPI_Next(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{Next: func(fiber.Ctx) bool { return true }}))

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_OpenAPI_ConnectIgnored(t *testing.T) {
	app := fiber.New()

	app.Connect("/conn", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)
	require.NotContains(t, paths, "/conn")
}

func Test_OpenAPI_MultipleParams(t *testing.T) {
	app := fiber.New()

	app.Get("/users/:uid/books/:bid", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)
	require.Contains(t, paths, "/users/{uid}/books/{bid}")
	op := requireMap(t, paths["/users/{uid}/books/{bid}"]["get"])
	params := requireSlice(t, op["parameters"])
	require.Len(t, params, 2)
	p0 := requireMap(t, params[0])
	p1 := requireMap(t, params[1])
	require.Equal(t, "uid", p0["name"])
	require.Equal(t, "path", p0["in"])
	require.Equal(t, "bid", p1["name"])
	require.Equal(t, "path", p1["in"])
}

func Test_OpenAPI_ConsumesProduces(t *testing.T) {
	app := fiber.New()

	app.Post("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) }).
		Consumes(fiber.MIMEApplicationJSON).
		Produces(fiber.MIMEApplicationXML)

	paths := getPaths(t, app)

	op := requireMap(t, paths["/users"]["post"])
	rb := requireMap(t, op["requestBody"])
	reqContent := requireMap(t, rb["content"])
	require.Contains(t, reqContent, fiber.MIMEApplicationJSON)

	resp := requireMap(t, requireMap(t, op["responses"])["200"])
	cont := requireMap(t, resp["content"])
	require.Contains(t, cont, fiber.MIMEApplicationXML)
}

func Test_OpenAPI_NoRequestBodyForGET(t *testing.T) {
	app := fiber.New()

	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)
	op := requireMap(t, paths["/users"]["get"])
	require.NotContains(t, op, "requestBody")
}

func Test_OpenAPI_Cache(t *testing.T) {
	app := fiber.New()

	app.Get("/first", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Contains(t, spec.Paths, "/first")

	app.Get("/second", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req = httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.NotContains(t, spec.Paths, "/second")
}

func requireMap(t *testing.T, value any) map[string]any {
	t.Helper()
	m, ok := value.(map[string]any)
	require.True(t, ok)
	return m
}

func requireSlice(t *testing.T, value any) []any {
	t.Helper()
	s, ok := value.([]any)
	require.True(t, ok)
	return s
}

func requireString(t *testing.T, value any) string {
	t.Helper()
	s, ok := value.(string)
	require.True(t, ok)
	return s
}

func Test_OpenAPI_MiddlewareRoutesExcluded(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Register a middleware using Use() — should be excluded from spec
	app.Use(func(c fiber.Ctx) error { return c.Next() })
	// Register an actual route — should be included
	app.Get("/health", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)
	require.Contains(t, paths, "/health")
	// The middleware path "/" should NOT appear
	require.NotContains(t, paths, "/")
}

func Test_OpenAPI_DefaultRequestBodyForPOST(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// No route body metadata should fall back to the default request body for POST.
	app.Post("/webhook", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	op := spec.Paths["/webhook"]["post"]
	require.NotNil(t, op.RequestBody)
	require.Contains(t, op.RequestBody.Content, fiber.MIMETextPlain)
}

func Test_OpenAPI_AutoHeadExcluded(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Registering a GET route automatically creates a HEAD route.
	// The auto-generated HEAD should NOT appear in the spec.
	app.Get("/items", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)
	require.Contains(t, paths, "/items")
	ops := paths["/items"]
	require.Contains(t, ops, "get")
	require.NotContains(t, ops, "head", "auto-generated HEAD route should be excluded")
}

func Test_ConvertToOpenAPIPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		fiberPath  string
		expectPath string
		params     []string
	}{
		{
			name:       "simple path no params",
			fiberPath:  "/users",
			params:     nil,
			expectPath: "/users",
		},
		{
			name:       "parameter with constraint",
			fiberPath:  "/users/:id<int>",
			params:     []string{"id"},
			expectPath: "/users/{id}",
		},
		{
			name:       "parameter with regex constraint",
			fiberPath:  "/posts/:slug<regex([a-z]+)>",
			params:     []string{"slug"},
			expectPath: "/posts/{slug}",
		},
		{
			name:       "optional parameter",
			fiberPath:  "/items/:id?",
			params:     []string{"id"},
			expectPath: "/items/{id}",
		},
		{
			name:       "wildcard param",
			fiberPath:  "/files/*",
			params:     []string{"*"},
			expectPath: "/files/{wildcard}",
		},
		{
			name:       "plus param",
			fiberPath:  "/docs/+",
			params:     []string{"+"},
			expectPath: "/docs/{wildcard}",
		},
		{
			name:       "multiple params with constraints",
			fiberPath:  "/api/:version<int>/:resource/:id<int>",
			params:     []string{"version", "resource", "id"},
			expectPath: "/api/{version}/{resource}/{id}",
		},
		{
			name:       "param with dot delimiter",
			fiberPath:  "/files/:name.:ext",
			params:     []string{"name", "ext"},
			expectPath: "/files/{name}.{ext}",
		},
		{
			name:       "param with dash delimiter",
			fiberPath:  "/users/:firstName-:lastName",
			params:     []string{"firstName", "lastName"},
			expectPath: "/users/{firstName}-{lastName}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := convertToOpenAPIPath(tt.fiberPath, tt.params)
			require.Equal(t, tt.expectPath, result)
		})
	}
}

func Test_BuildOpenAPIPathVariants(t *testing.T) {
	t.Parallel()

	t.Run("optional parameter emits both variants", func(t *testing.T) {
		t.Parallel()
		variants := buildOpenAPIPathVariants("/items/:id?", []string{"id"})
		require.Len(t, variants, 2)
		require.Equal(t, "/items/{id}", variants[0].Path)
		require.Equal(t, []string{"id"}, variants[0].ParamNames)
		require.Equal(t, "/items", variants[1].Path)
		require.Empty(t, variants[1].ParamNames)
	})

	t.Run("wildcard uses openapi-compatible parameter name", func(t *testing.T) {
		t.Parallel()
		variants := buildOpenAPIPathVariants("/files/*", []string{"*1"})
		require.Len(t, variants, 1)
		require.Equal(t, "/files/{wildcard1}", variants[0].Path)
		require.Equal(t, []string{"wildcard1"}, variants[0].ParamNames)
		require.Equal(t, "wildcard1", variants[0].PathParamAliases["*1"])
	})
}

func Test_OpenAPI_OptionalPathParameterVariants(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/items/:id?", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)
	require.Contains(t, paths, "/items")
	require.Contains(t, paths, "/items/{id}")

	baseParams := requireMap(t, paths["/items"]["get"])["parameters"]
	require.Nil(t, baseParams)

	optionalOp := requireMap(t, paths["/items/{id}"]["get"])
	optionalParams := requireSlice(t, optionalOp["parameters"])
	require.Len(t, optionalParams, 1)
	param := requireMap(t, optionalParams[0])
	require.Equal(t, "id", param["name"])
}

func Test_OpenAPI_WildcardPathParameterNameMatchesTemplate(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/files/*", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)
	require.Len(t, paths, 1)
	for template, ops := range paths {
		op := requireMap(t, ops["get"])
		params := requireSlice(t, op["parameters"])
		require.Len(t, params, 1)
		name := requireString(t, requireMap(t, params[0])["name"])
		require.Contains(t, template, "{"+name+"}")
		require.NotContains(t, name, "*")
		require.NotContains(t, name, "+")
	}
}

func Test_OpenAPI_RequestBodyFromRoute(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Route-level requestBody should be reflected in the generated spec.
	app.Post("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) }).
		RequestBodyWithExample("User from route", true, map[string]any{"type": "object"}, "", map[string]any{"name": "Alice"}, nil, fiber.MIMEApplicationJSON)
	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	op := spec.Paths["/users"]["post"]
	require.NotNil(t, op.RequestBody)
	require.Equal(t, "User from route", op.RequestBody.Description)
	require.True(t, op.RequestBody.Required)
	require.Contains(t, op.RequestBody.Content, fiber.MIMEApplicationJSON)
}

func Test_OpenAPI_ResponseContentFromRoute(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		ResponseWithExample(
			fiber.StatusOK,
			"Success",
			nil,
			"#/components/schemas/DefaultSchema",
			map[string]any{"default": "example"},
			map[string]any{"example1": map[string]any{"value": "test"}},
			fiber.MIMEApplicationJSON,
		)
	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	op := spec.Paths["/test"]["get"]
	resp200 := op.Responses["200"]
	require.Contains(t, resp200.Content, fiber.MIMEApplicationJSON)
	jsonContent := resp200.Content[fiber.MIMEApplicationJSON]
	require.Contains(t, jsonContent, "schema")
	schema, ok := jsonContent["schema"].(map[string]any)
	require.True(t, ok, "schema should be a map")
	require.Equal(t, "#/components/schemas/DefaultSchema", schema["$ref"])
	require.Contains(t, jsonContent, "example")
	require.Contains(t, jsonContent, "examples")
}

func Test_OpenAPI_ResolvedSpecPathEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		groupPath      string
		middlewarePath string
		expectedPath   string
	}{
		{
			name:           "root group with custom path",
			groupPath:      "/",
			middlewarePath: "spec.json",
			expectedPath:   "/spec.json",
		},
		{
			name:           "group with wildcard",
			groupPath:      "/api/*",
			middlewarePath: "/openapi.json",
			expectedPath:   "/api/openapi.json",
		},
		{
			name:           "empty group path",
			groupPath:      "",
			middlewarePath: "/openapi.json",
			expectedPath:   "/openapi.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			group := app.Group(tt.groupPath)
			group.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
			group.Use(New(Config{Path: tt.middlewarePath}))

			req := httptest.NewRequest(fiber.MethodGet, tt.expectedPath, http.NoBody)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
			require.Equal(t, fiber.MIMEApplicationJSONCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
		})
	}
}

func Test_OpenAPI_ParameterEdgeCases(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Route parameter helpers should force path parameters to required=true.
	app.Get("/test/:id", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Parameter("id", "path", false, nil, "resource identifier")
	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	op := spec.Paths["/test/{id}"]["get"]
	require.Len(t, op.Parameters, 1)
	require.Equal(t, "id", op.Parameters[0].Name)
	require.Equal(t, "path", op.Parameters[0].In)
	require.True(t, op.Parameters[0].Required)
}

func Test_OpenAPI_SchemaFromCombinations(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		ParameterWithExample("withSchemaRef", "query", false, map[string]any{"type": "string"}, "#/components/schemas/MySchema", "", nil, nil).
		ParameterWithExample("withSchema", "query", false, map[string]any{"type": "integer", "minimum": 0}, "", "", nil, nil).
		Parameter("withDefaultType", "query", false, nil, "")
	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	op := spec.Paths["/test"]["get"]

	for _, param := range op.Parameters {
		require.NotNil(t, param.Schema)
		switch param.Name {
		case "withSchemaRef":
			require.Equal(t, "#/components/schemas/MySchema", param.Schema["$ref"])
		case "withSchema":
			require.Equal(t, "integer", param.Schema["type"])
			require.InDelta(t, 0.0, param.Schema["minimum"], 0.001)
		case "withDefaultType":
			require.Equal(t, "string", param.Schema["type"])
		default:
			t.Fatalf("Unexpected parameter name: %s", param.Name)
		}
	}
}

func Test_OpenAPI_ShouldIncludeRequestBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		consumes   string
		method     string
		expectBody bool
	}{
		{
			name:       "GET with text/plain no body",
			method:     fiber.MethodGet,
			consumes:   fiber.MIMETextPlain, // default
			expectBody: false,
		},
		{
			name:       "GET with custom consumes has body",
			method:     fiber.MethodGet,
			consumes:   fiber.MIMEApplicationJSON,
			expectBody: true, // non-default consumes triggers body
		},
		{
			name:       "HEAD with text/plain no body",
			method:     fiber.MethodHead,
			consumes:   fiber.MIMETextPlain,
			expectBody: false,
		},
		{
			name:       "OPTIONS with text/plain no body",
			method:     fiber.MethodOptions,
			consumes:   fiber.MIMETextPlain,
			expectBody: false,
		},
		{
			name:       "TRACE with text/plain no body",
			method:     fiber.MethodTrace,
			consumes:   fiber.MIMETextPlain,
			expectBody: false,
		},
		{
			name:       "POST with custom consumes",
			method:     fiber.MethodPost,
			consumes:   fiber.MIMEApplicationXML,
			expectBody: true,
		},
		{
			name:       "PUT with consumes",
			method:     fiber.MethodPut,
			consumes:   fiber.MIMEApplicationJSON,
			expectBody: true,
		},
		{
			name:       "PATCH with consumes",
			method:     fiber.MethodPatch,
			consumes:   fiber.MIMEApplicationJSON,
			expectBody: true,
		},
		{
			name:       "DELETE with consumes",
			method:     fiber.MethodDelete,
			consumes:   fiber.MIMEApplicationJSON,
			expectBody: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()

			var route fiber.Router
			handler := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }

			switch tt.method {
			case fiber.MethodGet:
				route = app.Get("/test", handler)
			case fiber.MethodPost:
				route = app.Post("/test", handler)
			case fiber.MethodPut:
				route = app.Put("/test", handler)
			case fiber.MethodPatch:
				route = app.Patch("/test", handler)
			case fiber.MethodDelete:
				route = app.Delete("/test", handler)
			case fiber.MethodHead:
				route = app.Head("/test", handler)
			case fiber.MethodOptions:
				route = app.Options("/test", handler)
			case fiber.MethodTrace:
				route = app.Add([]string{fiber.MethodTrace}, "/test", handler)
			default:
				t.Fatalf("Unsupported method: %s", tt.method)
			}

			if tt.consumes != "" {
				route.Consumes(tt.consumes)
			}

			app.Use(New())

			req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
			resp, err := app.Test(req)
			require.NoError(t, err)

			var spec openAPISpec
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
			methodLower := strings.ToLower(tt.method)
			op := spec.Paths["/test"][methodLower]

			if tt.expectBody {
				require.NotNil(t, op.RequestBody, "Expected request body for %s", tt.method)
			} else {
				require.Nil(t, op.RequestBody, "Expected no request body for %s", tt.method)
			}
		})
	}
}

func Test_OpenAPI_MediaTypesToContentEmptyMediaType(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Test that empty media types in the list are skipped
	app.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Response(200, "Success", fiber.MIMEApplicationJSON, "", fiber.MIMETextPlain)

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	op := spec.Paths["/test"]["get"]
	resp200 := op.Responses["200"]
	require.Contains(t, resp200.Content, fiber.MIMEApplicationJSON)
	require.Contains(t, resp200.Content, fiber.MIMETextPlain)
	require.Len(t, resp200.Content, 2)
}

func Test_OpenAPI_NilParameterInAppend(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Test the defensive nil check in appendOrReplaceParameter
	// This is primarily for code coverage of the nil check branch
	app.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Parameter("valid", "query", false, nil, "A valid parameter")

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	op := spec.Paths["/test"]["get"]
	require.Len(t, op.Parameters, 1)
	require.Equal(t, "valid", op.Parameters[0].Name)
}

func Test_OpenAPI_MarshalError(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Create a route that will cause JSON marshal to be called
	app.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New())

	// First request should generate and cache the spec
	req1 := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp1.StatusCode)

	// Second request should return cached spec (coverage for once.Do and error check)
	req2 := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp2.StatusCode)
}

func Test_OpenAPI_ResponseWithSchemaRefAndExamples(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		ResponseWithExample(
			fiber.StatusOK,
			"Success",
			nil,
			"#/components/schemas/TestSchema",
			map[string]any{"test": "value"},
			map[string]any{"ex1": map[string]any{"value": "example1"}},
			fiber.MIMEApplicationJSON,
		)
	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	op := spec.Paths["/test"]["get"]
	resp200 := op.Responses["200"]
	content := resp200.Content[fiber.MIMEApplicationJSON]

	require.Contains(t, content, "schema")
	require.Contains(t, content, "example")
	require.Contains(t, content, "examples")
}
