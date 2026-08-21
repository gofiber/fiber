package openapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func Test_OpenAPI_Generate(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	t.Run("delete defaults to 204 with no content", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
	// OpenAPI spec: "example" and "examples" are mutually exclusive; "examples" takes precedence.
	require.Nil(t, param["example"])
	require.Equal(t, map[string]any{"sample": "abc"}, requireMap(t, param["examples"]))
	paramSchema := requireMap(t, param["schema"])
	require.Equal(t, "#/components/schemas/Query", paramSchema["$ref"])

	body := requireMap(t, op["requestBody"])
	bodyContent := requireMap(t, body["content"])
	jsonContent := requireMap(t, bodyContent[fiber.MIMEApplicationJSON])
	bodySchema := requireMap(t, jsonContent["schema"])
	require.Equal(t, "#/components/schemas/User", bodySchema["$ref"])
	// OpenAPI spec: "example" and "examples" are mutually exclusive; "examples" takes precedence.
	require.Nil(t, jsonContent["example"])
	require.Equal(t, map[string]any{"sample": map[string]any{"name": "doe"}}, requireMap(t, jsonContent["examples"]))

	resp := requireMap(t, requireMap(t, op["responses"])["201"])
	respContent := requireMap(t, resp["content"])
	respJSON := requireMap(t, respContent[fiber.MIMEApplicationJSON])
	respSchema := requireMap(t, respJSON["schema"])
	require.Equal(t, "#/components/schemas/UserResponse", respSchema["$ref"])
	// OpenAPI spec: "example" and "examples" are mutually exclusive; "examples" takes precedence.
	require.Nil(t, respJSON["example"])
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
	t.Parallel()

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
			t.Parallel()
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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	app := fiber.New()

	paths := getPaths(t, app)

	// Middleware routes registered via Use() are excluded, so an app with
	// only the openapi middleware has no paths in the generated spec.
	require.Empty(t, paths)
}

func Test_OpenAPI_RootOnly(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)

	require.Contains(t, paths, "/")
	require.Contains(t, paths["/"], "get")
}

func Test_OpenAPI_GroupMiddleware(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

func Test_OpenAPI_SwaggerUI_DefaultTemplate(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/swagger", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	bodyText := string(body)
	require.Contains(t, bodyText, "<title>Fiber API - Swagger UI</title>")
	require.Contains(t, bodyText, `href="https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui.css"`)
	require.Contains(t, bodyText, `src="https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-bundle.js"`)
	require.Contains(t, bodyText, `url: "\/openapi.json"`)
	require.Contains(t, bodyText, `id="swagger-ui" data-swagger-options='{}'`)
}

func Test_OpenAPI_SwaggerUI_ConfigurableAssetsAndOptions(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New(Config{
		Title:            "Custom API",
		Path:             "/spec.json",
		UIPath:           "/docs",
		SwaggerCSSURL:    "https://cdn.example.com/swagger-ui.css",
		SwaggerBundleURL: "https://cdn.example.com/swagger-ui-bundle.js",
		SwaggerOptions: map[string]any{
			"docExpansion": "list",
			"deepLinking":  true,
		},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/docs", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	bodyText := string(body)
	require.Contains(t, bodyText, "<title>Custom API - Swagger UI</title>")
	require.Contains(t, bodyText, `href="https://cdn.example.com/swagger-ui.css"`)
	require.Contains(t, bodyText, `src="https://cdn.example.com/swagger-ui-bundle.js"`)
	require.Contains(t, bodyText, `url: "\/spec.json"`)
	require.Contains(t, bodyText, `id="swagger-ui" data-swagger-options='{&#34;`)
	require.Contains(t, bodyText, `&#34;docExpansion&#34;:&#34;list&#34;`)
	require.Contains(t, bodyText, `&#34;deepLinking&#34;:true`)
}

func Test_OpenAPI_SwaggerUI_GroupPath(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Group("/api").Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/api/swagger", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `url: "\/api\/openapi.json"`)
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
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{Next: func(fiber.Ctx) bool { return true }}))

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_OpenAPI_ConnectIgnored(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Connect("/conn", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)
	require.NotContains(t, paths, "/conn")
}

func Test_OpenAPI_MultipleParams(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	app := fiber.New()

	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)
	op := requireMap(t, paths["/users"]["get"])
	require.NotContains(t, op, "requestBody")
}

// Test_OpenAPI_Cache verifies the spec is regenerated per request, so routes
// added after the first request are reflected without a process restart.
func Test_OpenAPI_Cache(t *testing.T) {
	t.Parallel()

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
	require.Contains(t, spec.Paths, "/first")
	require.Contains(t, spec.Paths, "/second")
}

func requireMap(t *testing.T, value any) map[string]any {
	t.Helper()
	m, ok := value.(map[string]any)
	require.True(t, ok)
	return m
}

// fetchJSON requests the generated OpenAPI spec and decodes the JSON body into a
// generic map. The middleware must already be registered on the app.
func fetchJSON(t *testing.T, app *fiber.App) map[string]any {
	t.Helper()

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var out map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	return out
}

func Test_OpenAPI_SecuritySchemes(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New(Config{
		SecuritySchemes: map[string]any{
			"bearerAuth": map[string]any{
				"type":         "http",
				"scheme":       "bearer",
				"bearerFormat": "JWT",
			},
		},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}))

	spec := fetchJSON(t, app)

	components := requireMap(t, spec["components"])
	schemes := requireMap(t, components["securitySchemes"])
	require.Contains(t, schemes, "bearerAuth")
	bearer := requireMap(t, schemes["bearerAuth"])
	require.Equal(t, "http", bearer["type"])

	security, ok := spec["security"].([]any)
	require.True(t, ok)
	require.Len(t, security, 1)
}

func Test_OpenAPI_SecuritySchemes_MergeWithComponents(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New(Config{
		Components: map[string]any{
			"schemas": map[string]any{
				"User": map[string]any{"type": "object"},
			},
		},
		SecuritySchemes: map[string]any{
			"apiKey": map[string]any{"type": "apiKey", "in": "header", "name": "X-API-Key"},
		},
	}))

	spec := fetchJSON(t, app)
	components := requireMap(t, spec["components"])
	// User-provided components are preserved alongside the injected securitySchemes.
	require.Contains(t, components, "schemas")
	require.Contains(t, components, "securitySchemes")
}

func Test_OpenAPI_RouteSecurity(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/private", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Security(map[string][]string{"bearerAuth": {"read"}})
	// An explicit empty requirement documents "no authentication".
	app.Get("/public", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Security(map[string][]string{})

	paths := getPaths(t, app)

	privateOp := requireMap(t, paths["/private"]["get"])
	sec, ok := privateOp["security"].([]any)
	require.True(t, ok)
	require.Len(t, sec, 1)
	first := requireMap(t, sec[0])
	require.Contains(t, first, "bearerAuth")

	publicOp := requireMap(t, paths["/public"]["get"])
	pub, ok := publicOp["security"].([]any)
	require.True(t, ok)
	require.Len(t, pub, 1)
	require.Empty(t, requireMap(t, pub[0]))
}

func Test_OpenAPI_InfoMetadata(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New(Config{
		Contact:        &Contact{Name: "API Team", Email: "api@example.com", URL: "https://example.com"},
		License:        &License{Name: "MIT", URL: "https://opensource.org/licenses/MIT"},
		TermsOfService: "https://example.com/terms",
	}))

	spec := fetchJSON(t, app)
	info := requireMap(t, spec["info"])
	require.Equal(t, "https://example.com/terms", info["termsOfService"])
	contact := requireMap(t, info["contact"])
	require.Equal(t, "API Team", contact["name"])
	license := requireMap(t, info["license"])
	require.Equal(t, "MIT", license["name"])
}

func Test_OpenAPI_MultipleServers(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New(Config{
		Servers: []Server{
			{URL: "https://prod.example.com", Description: "Production"},
			{URL: "https://staging.example.com", Description: "Staging"},
		},
	}))

	spec := fetchJSON(t, app)
	servers, ok := spec["servers"].([]any)
	require.True(t, ok)
	require.Len(t, servers, 2)
	first := requireMap(t, servers[0])
	require.Equal(t, "https://prod.example.com", first["url"])
	require.Equal(t, "Production", first["description"])
}

func Test_OpenAPI_ServerURLBackCompat(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New(Config{ServerURL: "https://example.com"}))

	spec := fetchJSON(t, app)
	servers, ok := spec["servers"].([]any)
	require.True(t, ok)
	require.Len(t, servers, 1)
	require.Equal(t, "https://example.com", requireMap(t, servers[0])["url"])
}

func Test_OpenAPI_TopLevelTagsAndExternalDocs(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New(Config{
		Tags: []Tag{
			{Name: "users", Description: "User operations"},
		},
		ExternalDocs: &ExternalDocs{Description: "Docs", URL: "https://docs.example.com"},
	}))

	spec := fetchJSON(t, app)
	tags, ok := spec["tags"].([]any)
	require.True(t, ok)
	require.Len(t, tags, 1)
	require.Equal(t, "users", requireMap(t, tags[0])["name"])

	ext := requireMap(t, spec["externalDocs"])
	require.Equal(t, "https://docs.example.com", ext["url"])
}

func Test_OpenAPI_SwaggerUI_StandalonePreset(t *testing.T) {
	t.Parallel()

	// Default config loads the standalone preset and uses StandaloneLayout.
	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/swagger", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `src="https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-standalone-preset.js"`)
	require.Contains(t, string(body), "StandaloneLayout")

	// A custom standalone preset URL is honored (self-hosting scenario).
	app2 := fiber.New()
	app2.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app2.Use(New(Config{SwaggerStandalonePresetURL: "https://cdn.example.com/standalone.js"}))

	req = httptest.NewRequest(fiber.MethodGet, "/swagger", http.NoBody)
	resp, err = app2.Test(req)
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `src="https://cdn.example.com/standalone.js"`)
}

func Test_OpenAPI_PathTrailingSlash(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New())

	for _, path := range []string{"/openapi.json/", "/swagger/"} {
		req := httptest.NewRequest(fiber.MethodGet, path, http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "path %q should resolve", path)
	}
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

	// Without body metadata a POST gets NO implicit request body; declaring
	// Consumes explicitly opts in to one.
	app.Post("/webhook", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Post("/typed", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Consumes(fiber.MIMEApplicationJSON)
	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Nil(t, spec.Paths["/webhook"]["post"].RequestBody)

	typed := spec.Paths["/typed"]["post"]
	require.NotNil(t, typed.RequestBody)
	require.Contains(t, typed.RequestBody.Content, fiber.MIMEApplicationJSON)
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
			variants := buildOpenAPIPathVariants(tt.fiberPath, tt.params)
			require.NotEmpty(t, variants)
			require.Equal(t, tt.expectPath, variants[0].Path)
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
	// OpenAPI spec: "example" and "examples" are mutually exclusive; "examples" takes precedence.
	require.NotContains(t, jsonContent, "example")
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
			name:       "GET without consumes has no body",
			method:     fiber.MethodGet,
			consumes:   "",
			expectBody: false,
		},
		{
			name:       "GET never has a body even with explicit consumes",
			method:     fiber.MethodGet,
			consumes:   fiber.MIMEApplicationJSON,
			expectBody: false, // GET/HEAD never carry a request body
		},
		{
			name:       "HEAD never has a body",
			method:     fiber.MethodHead,
			consumes:   fiber.MIMETextPlain,
			expectBody: false,
		},
		{
			name:       "OPTIONS with explicit consumes opts into a body",
			method:     fiber.MethodOptions,
			consumes:   fiber.MIMETextPlain,
			expectBody: true,
		},
		{
			name:       "TRACE never has a body (RFC 9110)",
			method:     fiber.MethodTrace,
			consumes:   fiber.MIMETextPlain,
			expectBody: false,
		},
		{
			name:       "POST without consumes has no implicit body",
			method:     fiber.MethodPost,
			consumes:   "",
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

	// First request generates the spec
	req1 := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp1.StatusCode)

	// Second request regenerates it successfully as well
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
	// OpenAPI spec: "example" and "examples" are mutually exclusive; "examples" takes precedence.
	require.NotContains(t, content, "example")
	require.Contains(t, content, "examples")
}

func Test_OpenAPI_Components(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		ResponseWithExample(fiber.StatusOK, "User list", nil, "#/components/schemas/User", nil, nil, fiber.MIMEApplicationJSON)

	components := map[string]any{
		"schemas": map[string]any{
			"User": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
	}
	app.Use(New(Config{Components: components}))

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Contains(t, spec, "components")
	comps, ok := spec["components"].(map[string]any)
	require.True(t, ok, "components should be a map")
	require.Contains(t, comps, "schemas")
	schemas, ok := comps["schemas"].(map[string]any)
	require.True(t, ok, "schemas should be a map")
	require.Contains(t, schemas, "User")
}

func Test_OpenAPI_ExampleWithoutExamples(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// When only "example" is provided (no "examples"), "example" should appear.
	app.Get("/test", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		ParameterWithExample("q", "query", false, nil, "", "search", "abc", nil).
		ResponseWithExample(fiber.StatusOK, "OK", nil, "", map[string]any{"id": 1}, nil, fiber.MIMEApplicationJSON)

	paths := getPaths(t, app)
	op := requireMap(t, paths["/test"]["get"])

	params := requireSlice(t, op["parameters"])
	require.Len(t, params, 1)
	param := requireMap(t, params[0])
	require.Equal(t, "abc", param["example"])
	require.Nil(t, param["examples"])

	resp := requireMap(t, requireMap(t, op["responses"])["200"])
	respContent := requireMap(t, resp["content"])
	respJSON := requireMap(t, respContent[fiber.MIMEApplicationJSON])
	require.Equal(t, map[string]any{"id": float64(1)}, respJSON["example"])
	require.Nil(t, respJSON["examples"])
}

func Test_OpenAPI_SchemaOfIntegration(t *testing.T) {
	t.Parallel()

	type User struct {
		Name string `json:"name"`
		ID   int    `json:"id"`
	}

	app := fiber.New()
	app.Get("/users/:id", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		ResponseWithExample(fiber.StatusOK, "User found", SchemaOf(User{}), "", nil, nil, fiber.MIMEApplicationJSON)
	app.Post("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) }).
		RequestBodyWithExample("Create user", true, SchemaOf(User{}), "", nil, nil, fiber.MIMEApplicationJSON).
		ResponseWithExample(fiber.StatusCreated, "Created", SchemaOf(User{}), "", nil, nil, fiber.MIMEApplicationJSON)

	paths := getPaths(t, app)

	// Verify GET /users/:id response schema
	getOp := requireMap(t, paths["/users/{id}"]["get"])
	getResp := requireMap(t, requireMap(t, getOp["responses"])["200"])
	getContent := requireMap(t, getResp["content"])
	getJSON := requireMap(t, getContent[fiber.MIMEApplicationJSON])
	schema := requireMap(t, getJSON["schema"])
	require.Equal(t, "object", schema["type"])
	props := requireMap(t, schema["properties"])
	require.Contains(t, props, "id")
	require.Contains(t, props, "name")

	// Verify POST /users request body schema
	postOp := requireMap(t, paths["/users"]["post"])
	reqBody := requireMap(t, postOp["requestBody"])
	reqContent := requireMap(t, reqBody["content"])
	reqJSON := requireMap(t, reqContent[fiber.MIMEApplicationJSON])
	reqSchema := requireMap(t, reqJSON["schema"])
	require.Equal(t, "object", reqSchema["type"])
	reqProps := requireMap(t, reqSchema["properties"])
	require.Contains(t, reqProps, "id")
	require.Contains(t, reqProps, "name")
}

func Test_OpenAPI_OperationID_GeneratedAndUnique(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	// Two unnamed routes -> generated, non-empty, unique operationIds.
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/users/:id", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)

	listOp := requireMap(t, paths["/users"]["get"])
	require.Equal(t, "getUsers", listOp["operationId"])

	getOp := requireMap(t, paths["/users/{id}"]["get"])
	require.Equal(t, "getUsersId", getOp["operationId"])
}

func Test_OpenAPI_OperationID_DuplicateNamesDeduped(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/a", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).Name("dup")
	app.Get("/b", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).Name("dup")

	paths := getPaths(t, app)

	idA, ok := requireMap(t, paths["/a"]["get"])["operationId"].(string)
	require.True(t, ok)
	idB, ok := requireMap(t, paths["/b"]["get"])["operationId"].(string)
	require.True(t, ok)

	ids := map[string]bool{idA: true, idB: true}
	// Both routes asked for "dup"; the generated document must keep them unique.
	require.Len(t, ids, 2)
	require.Contains(t, ids, "dup")
	require.Contains(t, ids, "dup_2")
}

func Test_OpenAPI_AutoSummaryUsesOpenAPIPath(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users/:id<int>", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)
	op := requireMap(t, paths["/users/{id}"]["get"])
	// Auto-generated summary uses the OpenAPI path template, not Fiber syntax.
	require.Equal(t, "GET /users/{id}", op["summary"])
}

func Test_OpenAPI_HiddenRouteExcluded(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/public", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/internal", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).Hidden()

	paths := getPaths(t, app)
	require.Contains(t, paths, "/public")
	require.NotContains(t, paths, "/internal")
}

func Test_OpenAPI_ResponseHeader(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Response(200, "OK", fiber.MIMEApplicationJSON).
		ResponseHeader(200, "X-Rate-Limit", "Requests left", map[string]any{"type": "integer"})

	paths := getPaths(t, app)
	op := requireMap(t, paths["/users"]["get"])
	resp200 := requireMap(t, requireMap(t, op["responses"])["200"])
	headers := requireMap(t, resp200["headers"])
	rateLimit := requireMap(t, headers["X-Rate-Limit"])
	require.Equal(t, "Requests left", rateLimit["description"])
	require.Equal(t, map[string]any{"type": "integer"}, rateLimit["schema"])
}

func Test_OpenAPI_ResponseHeaderCreatesResponse(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	// ResponseHeader without a preceding Response() should still create the entry.
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		ResponseHeader(200, "X-Trace-Id", "Trace identifier", nil)

	paths := getPaths(t, app)
	op := requireMap(t, paths["/users"]["get"])
	resp200 := requireMap(t, requireMap(t, op["responses"])["200"])
	headers := requireMap(t, resp200["headers"])
	require.Contains(t, headers, "X-Trace-Id")
}

func Test_OpenAPI_GETExplicitRequestBodySuppressed(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/search", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		RequestBody("query", false, fiber.MIMEApplicationJSON)

	paths := getPaths(t, app)
	op := requireMap(t, paths["/search"]["get"])
	require.NotContains(t, op, "requestBody")
}

func openapiBoolPtr(b bool) *bool { return &b }

// fetchSpecWithConfig registers routes, mounts the middleware with cfg, and
// returns the decoded /openapi.json document.
//
//nolint:gocritic // hugeParam: Config is passed by value to mirror the public New signature.
func fetchSpecWithConfig(t *testing.T, cfg Config, register func(app *fiber.App)) map[string]any {
	t.Helper()
	app := fiber.New()
	if register != nil {
		register(app)
	}
	app.Use(New(cfg))
	return fetchJSON(t, app)
}

func Test_OpenAPI_ParameterSerialization(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/items", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		AddParameter(fiber.RouteParameter{
			Name:            "ids",
			In:              "query",
			Description:     "Item ids",
			Style:           "form",
			Explode:         openapiBoolPtr(false),
			Deprecated:      true,
			AllowEmptyValue: true,
			AllowReserved:   true,
			Schema:          map[string]any{"type": "array"},
		})

	paths := getPaths(t, app)
	op := requireMap(t, paths["/items"]["get"])
	params, ok := op["parameters"].([]any)
	require.True(t, ok)
	require.Len(t, params, 1)
	p := requireMap(t, params[0])
	require.Equal(t, "ids", p["name"])
	require.Equal(t, "form", p["style"])
	require.Equal(t, false, p["explode"])
	require.Equal(t, true, p["deprecated"])
	require.Equal(t, true, p["allowEmptyValue"])
	require.Equal(t, true, p["allowReserved"])
}

func Test_OpenAPI_ServerVariables(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{
		Servers: []Server{{
			URL:         "https://{region}.example.com",
			Description: "Regional",
			Variables: map[string]ServerVariable{
				"region": {Default: "us", Enum: []string{"us", "eu"}, Description: "Region"},
			},
		}},
	}, func(app *fiber.App) {
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	})

	servers, ok := spec["servers"].([]any)
	require.True(t, ok)
	require.Len(t, servers, 1)
	vars := requireMap(t, requireMap(t, servers[0])["variables"])
	region := requireMap(t, vars["region"])
	require.Equal(t, "us", region["default"])
}

func Test_OpenAPI_TagExternalDocs(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{
		Tags: []Tag{{
			Name:         "users",
			Description:  "User ops",
			ExternalDocs: &ExternalDocs{Description: "more", URL: "https://docs.example.com/users"},
		}},
	}, func(app *fiber.App) {
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	})

	tags, ok := spec["tags"].([]any)
	require.True(t, ok)
	ext := requireMap(t, requireMap(t, tags[0])["externalDocs"])
	require.Equal(t, "https://docs.example.com/users", ext["url"])
}

func Test_OpenAPI_InfoSummaryVersionGated(t *testing.T) {
	t.Parallel()

	register := func(app *fiber.App) {
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	}

	spec31 := fetchSpecWithConfig(t, Config{OpenAPIVersion: "3.1.0", Summary: "Short summary"}, register)
	require.Equal(t, "Short summary", requireMap(t, spec31["info"])["summary"])

	spec30 := fetchSpecWithConfig(t, Config{OpenAPIVersion: "3.0.0", Summary: "Short summary"}, register)
	require.NotContains(t, requireMap(t, spec30["info"]), "summary")
}

func Test_OpenAPI_OperationExternalDocs(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		OperationExternalDocs("See more", "https://docs.example.com/x")

	paths := getPaths(t, app)
	op := requireMap(t, paths["/x"]["get"])
	ext := requireMap(t, op["externalDocs"])
	require.Equal(t, "https://docs.example.com/x", ext["url"])
	require.Equal(t, "See more", ext["description"])
}

func Test_OpenAPI_PerMediaTypeContent(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Post("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) }).
		RequestBodyContent("payload", true, map[string]fiber.RouteMediaType{
			fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}},
			fiber.MIMEApplicationXML:  {SchemaRef: "#/components/schemas/X"},
		}).
		ResponseContent(fiber.StatusCreated, "Created", map[string]fiber.RouteMediaType{
			fiber.MIMEApplicationJSON: {
				Schema:   map[string]any{"type": "string"},
				Encoding: map[string]any{"field": map[string]any{"contentType": "text/plain"}},
			},
		})

	paths := getPaths(t, app)
	op := requireMap(t, paths["/x"]["post"])

	reqContent := requireMap(t, requireMap(t, op["requestBody"])["content"])
	jsonSchema := requireMap(t, requireMap(t, reqContent[fiber.MIMEApplicationJSON])["schema"])
	require.Equal(t, "object", jsonSchema["type"])
	xmlSchema := requireMap(t, requireMap(t, reqContent[fiber.MIMEApplicationXML])["schema"])
	require.Equal(t, "#/components/schemas/X", xmlSchema["$ref"])

	respContent := requireMap(t, requireMap(t, requireMap(t, op["responses"])["201"])["content"])
	jsonResp := requireMap(t, respContent[fiber.MIMEApplicationJSON])
	require.Contains(t, jsonResp, "encoding")
}

func Test_OpenAPI_ResponseLinks(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Response(fiber.StatusOK, "OK", fiber.MIMEApplicationJSON).
		ResponseLink(fiber.StatusOK, "next", map[string]any{"operationId": "getNext"})

	paths := getPaths(t, app)
	op := requireMap(t, paths["/x"]["get"])
	links := requireMap(t, requireMap(t, requireMap(t, op["responses"])["200"])["links"])
	next := requireMap(t, links["next"])
	require.Equal(t, "getNext", next["operationId"])
}

func Test_OpenAPI_WebhooksAndDialectVersionGated(t *testing.T) {
	t.Parallel()

	register := func(app *fiber.App) {
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	}
	cfg := func(version string) Config {
		return Config{
			OpenAPIVersion:    version,
			JSONSchemaDialect: "https://spec.openapis.org/oas/3.1/dialect/base",
			Webhooks: map[string]any{
				"newItem": map[string]any{"post": map[string]any{"responses": map[string]any{"200": map[string]any{"description": "ok"}}}},
			},
		}
	}

	spec31 := fetchSpecWithConfig(t, cfg("3.1.0"), register)
	require.Contains(t, spec31, "webhooks")
	require.Contains(t, spec31, "jsonSchemaDialect")

	spec30 := fetchSpecWithConfig(t, cfg("3.0.0"), register)
	require.NotContains(t, spec30, "webhooks")
	require.NotContains(t, spec30, "jsonSchemaDialect")
}

func Test_OpenAPI_OperationExtension(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		OperationExtension(map[string]any{
			"servers":   []any{map[string]any{"url": "https://op.example.com"}},
			"callbacks": map[string]any{"onData": map[string]any{}},
			// Must not clobber a generated key:
			"responses": map[string]any{"999": map[string]any{"description": "ignored"}},
		})

	paths := getPaths(t, app)
	op := requireMap(t, paths["/x"]["get"])
	servers, ok := op["servers"].([]any)
	require.True(t, ok)
	require.Len(t, servers, 1)
	require.Contains(t, op, "callbacks")
	// Generated responses must win over the extension's "responses".
	require.NotContains(t, requireMap(t, op["responses"]), "999")
}

func Test_OpenAPI_QueryMethod_Gated(t *testing.T) {
	t.Parallel()

	register := func(app *fiber.App) {
		app.Query("/search", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
			RequestBody("query", true, fiber.MIMEApplicationJSON)
	}

	// 3.2: QUERY emits a `query` operation that carries a request body.
	spec32 := fetchSpecWithConfig(t, Config{OpenAPIVersion: "3.2.0"}, register)
	paths32 := requireMap(t, spec32["paths"])
	search := requireMap(t, paths32["/search"])
	require.Contains(t, search, "query")
	queryOp := requireMap(t, search["query"])
	require.Contains(t, queryOp, "requestBody")

	// 3.1 / 3.0: no `query` operation key exists, so the route is skipped.
	for _, version := range []string{"3.1.0", "3.0.0"} {
		spec := fetchSpecWithConfig(t, Config{OpenAPIVersion: version}, register)
		paths := requireMap(t, spec["paths"])
		require.NotContains(t, paths, "/search", "version %s should omit QUERY routes", version)
	}
}

func Test_OpenAPI_Version32Accepted(t *testing.T) {
	t.Parallel()

	register := func(app *fiber.App) {
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	}

	spec := fetchSpecWithConfig(t, Config{OpenAPIVersion: "3.2.0"}, register)
	require.Equal(t, "3.2.0", spec["openapi"])

	// Unknown versions still fall back to the default.
	fallback := fetchSpecWithConfig(t, Config{OpenAPIVersion: "9.9.9"}, register)
	require.Equal(t, "3.1.0", fallback["openapi"])
}

func Test_OpenAPI_32Fields_Gated(t *testing.T) {
	t.Parallel()

	register := func(app *fiber.App) {
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	}
	cfg := Config{
		Self:    "https://example.com/openapi.json",
		Servers: []Server{{URL: "https://api.example.com", Name: "prod"}},
		License: &License{Name: "MIT", Identifier: "MIT"},
	}
	cfg32 := cfg
	cfg32.OpenAPIVersion = "3.2.0"
	cfg30 := cfg
	cfg30.OpenAPIVersion = "3.0.0"

	spec32 := fetchSpecWithConfig(t, cfg32, register)
	require.Equal(t, "https://example.com/openapi.json", spec32["$self"])
	require.Equal(t, "prod", requireMap(t, requireSlice(t, spec32["servers"])[0])["name"])
	require.Equal(t, "MIT", requireMap(t, requireMap(t, spec32["info"])["license"])["identifier"])

	spec30 := fetchSpecWithConfig(t, cfg30, register)
	require.NotContains(t, spec30, "$self")
	require.NotContains(t, requireMap(t, requireSlice(t, spec30["servers"])[0]), "name")
	require.NotContains(t, requireMap(t, requireMap(t, spec30["info"])["license"]), "identifier")
}

func Test_OpenAPI_31Fields_EmitFor32(t *testing.T) {
	t.Parallel()

	register := func(app *fiber.App) {
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	}
	spec := fetchSpecWithConfig(t, Config{
		OpenAPIVersion:    "3.2.0",
		Summary:           "Summary",
		JSONSchemaDialect: "https://spec.openapis.org/oas/3.1/dialect/base",
		Webhooks:          map[string]any{"ping": map[string]any{}},
	}, register)
	require.Equal(t, "Summary", requireMap(t, spec["info"])["summary"])
	require.Contains(t, spec, "jsonSchemaDialect")
	require.Contains(t, spec, "webhooks")
}

func Test_OpenAPI_QueryStringParameterLocation(t *testing.T) {
	t.Parallel()

	register := func(app *fiber.App) {
		app.Get("/search", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
			AddParameter(fiber.RouteParameter{Name: "q", In: "querystring", Schema: map[string]any{"type": "string"}})
	}

	// The querystring location only exists in OpenAPI 3.2+.
	spec := fetchSpecWithConfig(t, Config{OpenAPIVersion: "3.2.0"}, register)
	op := requireMap(t, requireMap(t, requireMap(t, spec["paths"])["/search"])["get"])
	params, ok := op["parameters"].([]any)
	require.True(t, ok)
	param := requireMap(t, params[0])
	require.Equal(t, "querystring", param["in"])
	// A querystring parameter is described via content, never via schema.
	require.NotContains(t, param, "schema")
	content := requireMap(t, param["content"])
	entry := requireMap(t, content["application/x-www-form-urlencoded"])
	require.Equal(t, map[string]any{"type": "string"}, requireMap(t, entry["schema"]))

	// For earlier versions the parameter would make the document invalid and
	// must be dropped.
	spec = fetchSpecWithConfig(t, Config{OpenAPIVersion: "3.1.0"}, register)
	op = requireMap(t, requireMap(t, requireMap(t, spec["paths"])["/search"])["get"])
	require.NotContains(t, op, "parameters")
}

// Test_OpenAPI_CaseInsensitivePaths verifies the spec and UI paths match with
// the same case sensitivity as the app's routing.
func Test_OpenAPI_CaseInsensitivePaths(t *testing.T) {
	t.Parallel()

	app := fiber.New() // CaseSensitive: false by default
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New())

	for _, target := range []string{"/OPENAPI.JSON", "/Swagger"} {
		req := httptest.NewRequest(fiber.MethodGet, target, http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, target)
	}

	sensitive := fiber.New(fiber.Config{CaseSensitive: true})
	sensitive.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	sensitive.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/OPENAPI.JSON", http.NoBody)
	resp, err := sensitive.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// Test_OpenAPI_ExactRouteRegistration verifies the handler also works when it
// is registered on exact method routes instead of as prefix middleware.
func Test_OpenAPI_ExactRouteRegistration(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	handler := New()
	app.Get("/openapi.json", handler).Hidden()
	app.Get("/swagger", handler).Hidden()

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Contains(t, spec.Paths, "/users")

	req = httptest.NewRequest(fiber.MethodGet, "/swagger", http.NoBody)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `url: "\/openapi.json"`)
}

// Test_OpenAPI_SpecReflectsRouteRemoval verifies the spec is not stale after a
// route is removed and another added (same total route count).
func Test_OpenAPI_SpecReflectsRouteRemoval(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/old", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Contains(t, spec.Paths, "/old")

	app.RemoveRoute("/old", fiber.MethodGet)
	app.Get("/new", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.RebuildTree()

	req = httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err = app.Test(req)
	require.NoError(t, err)
	spec = openAPISpec{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Contains(t, spec.Paths, "/new")
	require.NotContains(t, spec.Paths, "/old")
}

// Test_OpenAPI_MultiPrefixUIPages verifies one handler instance mounted on
// several prefixes serves a UI page pointing at each prefix's own spec URL.
func Test_OpenAPI_MultiPrefixUIPages(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use([]string{"/v1", "/v2"}, New())

	for _, prefix := range []string{"/v1", "/v2"} {
		req := httptest.NewRequest(fiber.MethodGet, prefix+"/swagger", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), `url: "\/`+prefix[1:]+`\/openapi.json"`, prefix)
	}
}

// Test_OpenAPI_OptionalVariantDoesNotOverwrite verifies a variant never
// overwrites an earlier route's docs, matching router dispatch precedence.
func Test_OpenAPI_OptionalVariantDoesNotOverwrite(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Summary("List users")
	app.Get("/users/:id?", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Summary("Get user")

	paths := getPaths(t, app)
	require.Equal(t, "List users", requireMap(t, paths["/users"]["get"])["summary"])
	require.Equal(t, "Get user", requireMap(t, paths["/users/{id}"]["get"])["summary"])
}

// Test_OpenAPI_EscapedRoutePath verifies escaped special characters in route
// paths are treated as literals, matching the router's grammar.
func Test_OpenAPI_EscapedRoutePath(t *testing.T) {
	t.Parallel()

	variants := buildOpenAPIPathVariants(`/foo\:bar`, nil)
	require.Len(t, variants, 1)
	require.Equal(t, "/foo:bar", variants[0].Path)
	require.Empty(t, variants[0].ParamNames)
}

// Test_OpenAPI_MountedSubAppExactRoute verifies the handler works when it is
// registered on an exact route inside a sub-app that is mounted under a prefix.
func Test_OpenAPI_MountedSubAppExactRoute(t *testing.T) {
	t.Parallel()

	sub := fiber.New()
	sub.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	handler := New()
	sub.Get("/openapi.json", handler).Hidden()
	sub.Get("/swagger", handler).Hidden()

	app := fiber.New()
	app.Use("/api", sub)

	req := httptest.NewRequest(fiber.MethodGet, "/api/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Contains(t, spec.Paths, "/api/users")

	req = httptest.NewRequest(fiber.MethodGet, "/api/swagger", http.NoBody)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `url: "\/api\/openapi.json"`)
}

// Test_OpenAPI_ParameterizedMountPrefix verifies the middleware resolves
// concrete prefixes when mounted under a path with parameters.
func Test_OpenAPI_ParameterizedMountPrefix(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use("/:tenant", New())

	req := httptest.NewRequest(fiber.MethodGet, "/acme/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Contains(t, spec.Paths, "/users")

	// Deeper paths must not be treated as the spec endpoint.
	req = httptest.NewRequest(fiber.MethodGet, "/acme/foo/openapi.json", http.NoBody)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	// The UI page points at the tenant's own spec URL.
	req = httptest.NewRequest(fiber.MethodGet, "/acme/swagger", http.NoBody)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `url: "\/acme\/openapi.json"`)
}

// Test_OpenAPI_MountedParamPrefixPathParams verifies path parameters introduced
// by a parameterized mount prefix are named correctly in the generated paths.
func Test_OpenAPI_MountedParamPrefixPathParams(t *testing.T) {
	t.Parallel()

	sub := fiber.New()
	sub.Get("/users/:id", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	app := fiber.New()
	app.Use("/tenant/:tid", sub)

	paths := getPaths(t, app)
	require.Contains(t, paths, "/tenant/{tid}/users/{id}")
}

// Test_OpenAPI_ResponseSchemaWithoutMediaType verifies a schema or example with
// no media type falls back to the route's Produces instead of being dropped.
func Test_OpenAPI_ResponseSchemaWithoutMediaType(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Produces(fiber.MIMEApplicationJSON).
		ResponseWithExample(fiber.StatusOK, "OK", map[string]any{"type": "object"}, "", nil, nil)

	paths := getPaths(t, app)
	op := requireMap(t, paths["/users"]["get"])
	responses := requireMap(t, op["responses"])
	okResp := requireMap(t, responses["200"])
	content := requireMap(t, okResp["content"])
	entry := requireMap(t, content[fiber.MIMEApplicationJSON])
	require.Equal(t, "object", requireMap(t, entry["schema"])["type"])
}

// Test_OpenAPI_ExactRouteUnderParameterizedMount verifies an exact spec route
// inside a sub-app mounted under a parameterized prefix resolves per request.
func Test_OpenAPI_ExactRouteUnderParameterizedMount(t *testing.T) {
	t.Parallel()

	sub := fiber.New()
	sub.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	sub.Get("/openapi.json", New()).Hidden()

	app := fiber.New()
	app.Use("/:tenant", sub)

	req := httptest.NewRequest(fiber.MethodGet, "/acme/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Contains(t, spec.Paths, "/{tenant}/users")
}

// Test_OpenAPI_DuplicateSanitizedParamNames verifies parameters that sanitize
// alike get unique template names, as the specification requires.
func Test_OpenAPI_DuplicateSanitizedParamNames(t *testing.T) {
	t.Parallel()

	variants := buildOpenAPIPathVariants("/x/:na_ve/:naïve", nil)
	require.Len(t, variants, 1)
	require.Equal(t, "/x/{na_ve}/{na_ve_2}", variants[0].Path)
	require.Equal(t, []string{"na_ve", "na_ve_2"}, variants[0].ParamNames)
}

// Test_OpenAPI_ConcurrentDocsAndSpecRequests guards the locking contract:
// serving the spec while other goroutines document routes must be race-free.
func Test_OpenAPI_ConcurrentDocsAndSpecRequests(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New())

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 50 {
			app.Summary("iteration summary").
				Response(fiber.StatusOK, "OK", fiber.MIMEApplicationJSON).
				ResponseHeader(fiber.StatusOK, "X-Iter", "iteration", map[string]any{"type": "integer"}).
				Tags("concurrent")
		}
	}()

	for range 50 {
		req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	}
	<-done
}

func Test_OpenAPI_ExactRouteMount(t *testing.T) {
	t.Parallel()

	// Registering the middleware as an exact GET route must serve the spec.
	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/openapi.json", New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Contains(t, requireMap(t, spec["paths"]), "/users")

	// Same for a Use mount whose prefix already ends in the configured path.
	app2 := fiber.New()
	app2.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app2.Use("/v1/openapi.json", New())

	req = httptest.NewRequest(fiber.MethodGet, "/v1/openapi.json", http.NoBody)
	resp, err = app2.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_OpenAPI_SanitizedParamCollision(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	// Both params sanitize to "a_"; the generated names must stay unique.
	app.Get("/x/:a#/:a$", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)
	require.Len(t, paths, 1)
	for pathKey, item := range paths {
		op := requireMap(t, item["get"])
		params, ok := op["parameters"].([]any)
		require.True(t, ok)
		require.Len(t, params, 2)

		seen := map[string]struct{}{}
		for _, rawParam := range params {
			name, ok := requireMap(t, rawParam)["name"].(string)
			require.True(t, ok)
			_, dup := seen[name]
			require.False(t, dup, "duplicate parameter name %q in %s", name, pathKey)
			seen[name] = struct{}{}
			require.Contains(t, pathKey, "{"+name+"}", "path template must reference %q", name)
		}
	}
}

func Test_OpenAPI_MetadataChangesAfterFirstServe(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())
	// Registered last, so the chainable doc helper below targets this route.
	app.Get("/late", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	// Prime the cache.
	paths := requireMap(t, fetchJSON(t, app)["paths"])
	op := requireMap(t, requireMap(t, paths["/late"])["get"])
	require.NotContains(t, requireMap(t, op["responses"]), "201")

	// Documenting an existing route bumps the revision and refreshes the spec.
	app.Response(fiber.StatusCreated, "Created", fiber.MIMEApplicationJSON)

	paths = requireMap(t, fetchJSON(t, app)["paths"])
	op = requireMap(t, requireMap(t, paths["/late"])["get"])
	require.Contains(t, requireMap(t, op["responses"]), "201")
}

// Test_OpenAPI_SecurityEmptyScopesEmitArray verifies referencing a non-OAuth2
// scheme with an empty scope list emits the spec-required [] and never null.
func Test_OpenAPI_SecurityEmptyScopesEmitArray(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/secure", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Security(map[string][]string{"bearerAuth": {}})
	app.Use(New(Config{
		SecuritySchemes: map[string]any{
			"bearerAuth": map[string]any{"type": "http", "scheme": "bearer"},
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	require.Contains(t, string(body), `"security":[{"bearerAuth":[]}]`)
	require.NotContains(t, string(body), `"bearerAuth":null`)
}

// Test_OpenAPI_MultiOptionalParamsSingleHierarchy verifies several optional
// parameters emit one templated path per hierarchy level, as the spec requires.
func Test_OpenAPI_MultiOptionalParamsSingleHierarchy(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/files/:dir?/:name?", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New())

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec struct {
		Paths map[string]any `json:"paths"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.NoError(t, resp.Body.Close())

	require.Contains(t, spec.Paths, "/files")
	require.Contains(t, spec.Paths, "/files/{dir}")
	require.Contains(t, spec.Paths, "/files/{dir}/{name}")
	require.NotContains(t, spec.Paths, "/files/{name}")
}

// Test_OpenAPI_MountPrefixWithConstraintsAndEscapes verifies '*'/'+' inside a
// constraint or escape is not read as a wildcard, which hid the spec target.
func Test_OpenAPI_MountPrefixWithConstraintsAndEscapes(t *testing.T) {
	t.Parallel()

	t.Run("RegexConstraint", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use("/:id<regex([0-9]+)>/admin", New())

		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/123/admin/openapi.json", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("EscapedPlus", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(`/c\+\+`, New())

		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/c++/openapi.json", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("IntConstraintControl", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use("/:id<int>/admin", New())

		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/7/admin/openapi.json", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	})
}

// Test_OpenAPI_SharedHandlerAcrossApps verifies one handler on two apps serves
// each app's own spec, even when their route revisions coincide.
func Test_OpenAPI_SharedHandlerAcrossApps(t *testing.T) {
	t.Parallel()
	handler := New()

	app1 := fiber.New()
	app1.Get("/only-in-app1", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app1.Use(handler)

	app2 := fiber.New()
	app2.Get("/only-in-app2", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app2.Use(handler)

	specOf := func(app *fiber.App) map[string]any {
		t.Helper()
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		var spec struct {
			Paths map[string]any `json:"paths"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
		require.NoError(t, resp.Body.Close())
		return spec.Paths
	}

	paths1 := specOf(app1)
	require.Contains(t, paths1, "/only-in-app1")
	require.NotContains(t, paths1, "/only-in-app2")

	paths2 := specOf(app2)
	require.Contains(t, paths2, "/only-in-app2")
	require.NotContains(t, paths2, "/only-in-app1")
}

// Test_OpenAPI_SwaggerPageUsesAppJSONEncoder verifies the Swagger UI options
// are marshaled with the app's configured JSON encoder, like the spec itself.
func Test_OpenAPI_SwaggerPageUsesAppJSONEncoder(t *testing.T) {
	t.Parallel()
	encoderUsed := false
	app := fiber.New(fiber.Config{
		JSONEncoder: func(v any) ([]byte, error) {
			encoderUsed = true
			return json.Marshal(v)
		},
	})
	app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New(Config{SwaggerOptions: map[string]any{"deepLinking": true}}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/swagger", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.True(t, encoderUsed)
}

// Gap-audit regressions: each test below pins a defect found auditing the
// middleware for missing features and spec-conformance bugs.

// Test_OpenAPI_ParameterContent covers describing a parameter by media type,
// which any location allows and 3.2 "querystring" requires.
func Test_OpenAPI_ParameterContent(t *testing.T) {
	t.Parallel()

	t.Run("content replaces schema", func(t *testing.T) {
		t.Parallel()

		spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
			app.Get("/filter", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
				AddParameter(fiber.RouteParameter{
					Name:        "criteria",
					In:          "query",
					Description: "JSON encoded filter",
					Required:    true,
					// A schema alongside content is discarded: the Parameter
					// Object carries exactly one of the two.
					Schema: map[string]any{"type": "string"},
					Content: map[string]fiber.RouteMediaType{
						fiber.MIMEApplicationJSON: {
							Schema:  map[string]any{"type": "object"},
							Example: map[string]any{"color": "red"},
						},
					},
				})
		})

		op := requireMap(t, requireMap(t, requireMap(t, spec["paths"])["/filter"])["get"])
		params, ok := op["parameters"].([]any)
		require.True(t, ok)
		require.Len(t, params, 1)

		param := requireMap(t, params[0])
		require.Equal(t, "criteria", param["name"])
		require.Equal(t, "query", param["in"])
		require.Equal(t, true, param["required"])
		require.NotContains(t, param, "schema")

		entry := requireMap(t, requireMap(t, param["content"])[fiber.MIMEApplicationJSON])
		require.Equal(t, map[string]any{"type": "object"}, requireMap(t, entry["schema"]))
		require.Equal(t, map[string]any{"color": "red"}, requireMap(t, entry["example"]))
	})

	t.Run("querystring without schema stays valid", func(t *testing.T) {
		t.Parallel()

		spec := fetchSpecWithConfig(t, Config{OpenAPIVersion: "3.2.0"}, func(app *fiber.App) {
			app.Get("/q", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
				AddParameter(fiber.RouteParameter{Name: "filter", In: "querystring"})
		})

		op := requireMap(t, requireMap(t, requireMap(t, spec["paths"])["/q"])["get"])
		params, ok := op["parameters"].([]any)
		require.True(t, ok)

		param := requireMap(t, params[0])
		require.NotContains(t, param, "schema")
		entry := requireMap(t, requireMap(t, param["content"])[querystringMediaType])
		// A bare querystring parameter still has to describe something.
		require.Equal(t, map[string]any{"type": "string"}, requireMap(t, entry["schema"]))
	})

	t.Run("querystring honors an explicit content map", func(t *testing.T) {
		t.Parallel()

		spec := fetchSpecWithConfig(t, Config{OpenAPIVersion: "3.2.0"}, func(app *fiber.App) {
			app.Get("/q", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
				AddParameter(fiber.RouteParameter{
					Name: "filter",
					In:   "querystring",
					Content: map[string]fiber.RouteMediaType{
						fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}},
					},
				})
		})

		op := requireMap(t, requireMap(t, requireMap(t, spec["paths"])["/q"])["get"])
		params, ok := op["parameters"].([]any)
		require.True(t, ok)

		content := requireMap(t, requireMap(t, params[0])["content"])
		require.NotContains(t, content, querystringMediaType)
		require.Contains(t, content, fiber.MIMEApplicationJSON)
	})

	t.Run("caller map is not aliased", func(t *testing.T) {
		t.Parallel()

		content := map[string]fiber.RouteMediaType{
			fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}},
		}

		app := fiber.New()
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
			AddParameter(fiber.RouteParameter{Name: "p", In: "query", Content: content})

		// Mutating the caller's map after registration must not reach the route.
		content[fiber.MIMEApplicationJSON].Schema["type"] = "array"
		delete(content, fiber.MIMEApplicationJSON)

		app.Use(New())
		_, body := specBodyOf(t, app, "/openapi.json")
		require.Contains(t, body, `"type":"object"`)
		require.NotContains(t, body, `"type":"array"`)
	})
}

// Test_OpenAPI_OperationDescriptionOmitted asserts that an operation without a
// description omits the optional key instead of emitting an empty string.
func Test_OpenAPI_OperationDescriptionOmitted(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		app.Get("/bare", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Get("/documented", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
			Description("has one")
	})

	paths := requireMap(t, spec["paths"])
	bare := requireMap(t, requireMap(t, paths["/bare"])["get"])
	require.NotContains(t, bare, "description")

	documented := requireMap(t, requireMap(t, paths["/documented"])["get"])
	require.Equal(t, "has one", documented["description"])
}

// Test_SchemaOf_OmitZero asserts that a field encoding/json omits when zero is
// not reported as required.
func Test_SchemaOf_OmitZero(t *testing.T) {
	t.Parallel()

	type payload struct {
		Zero  string `json:"zero,omitzero"`
		Empty string `json:"empty,omitempty"`
		Plain string `json:"plain"`
	}

	schema := SchemaOf(payload{})
	required, ok := schema["required"].([]string)
	require.True(t, ok)
	require.Equal(t, []string{"plain"}, required)

	props := requireMap(t, schema["properties"])
	require.Contains(t, props, "zero")
	require.Contains(t, props, "empty")
}

// Test_SchemaOf_InvalidJSONTagName asserts a tag name encoding/json rejects
// falls back to the field name here too, so schema and wire agree.
func Test_SchemaOf_InvalidJSONTagName(t *testing.T) {
	t.Parallel()

	// Built reflectively: `go vet`'s structtag check rejects a literal tag
	// carrying a reserved character.
	typ := reflect.StructOf([]reflect.StructField{
		{Name: "Reserved", Type: reflect.TypeFor[string](), Tag: `json:"a\\b"`},
		{Name: "Punctuated", Type: reflect.TypeFor[string](), Tag: `json:"a b"`},
	})
	value := reflect.New(typ).Elem()
	value.Field(0).SetString("x")
	value.Field(1).SetString("y")

	encoded, err := json.Marshal(value.Interface())
	require.NoError(t, err)

	var wire map[string]any
	require.NoError(t, json.Unmarshal(encoded, &wire))

	props := requireMap(t, SchemaOf(value.Interface())["properties"])
	for name := range wire {
		require.Containsf(t, props, name, "schema is missing wire property %q", name)
	}
	require.Len(t, props, len(wire))
	// A space is accepted by every release, so that rename always lands.
	require.Contains(t, props, "a b")
	// The backslash name is deliberately not asserted literally: releases
	// disagree on whether it is accepted, truncated, or ignored in favor of the
	// field name, and the loop above already pins the schema to whichever the
	// running toolchain does.
}

// Test_SchemaOf_UintptrDocumented pins uintptr, which encoding/json writes as a
// bare number: dropping it would document fewer fields than the wire carries.
func Test_SchemaOf_UintptrDocumented(t *testing.T) {
	t.Parallel()

	type withUintptr struct {
		Pointer uintptr `json:"pointer"`
		Count   int     `json:"count"`
	}

	encoded, err := json.Marshal(withUintptr{Pointer: 42, Count: 7})
	require.NoError(t, err)
	var wire map[string]any
	require.NoError(t, json.Unmarshal(encoded, &wire))

	props := requireMap(t, SchemaOf(withUintptr{})["properties"])
	for name := range wire {
		require.Containsf(t, props, name, "schema is missing wire property %q", name)
	}
	require.Equal(t, map[string]any{"type": "integer"}, props["pointer"])
}

// Test_OpenAPI_PathParameterConstraintSchema asserts a "<...>" constraint types
// the path parameter schema instead of reporting every one as a string.
func Test_OpenAPI_PathParameterConstraintSchema(t *testing.T) {
	t.Parallel()

	cases := []struct {
		expected map[string]any
		pattern  string
	}{
		{map[string]any{"type": "integer"}, "/a/:v<int>"},
		{map[string]any{"type": "boolean"}, "/b/:v<bool>"},
		{map[string]any{"type": "number"}, "/c/:v<float>"},
		{map[string]any{"type": "string"}, "/d/:v<alpha>"},
		{map[string]any{"type": "string", "format": "uuid"}, "/e/:v<guid>"},
		{map[string]any{"type": "string", "minLength": 3.0}, "/f/:v<minLen(3)>"},
		{map[string]any{"type": "string", "maxLength": 9.0}, "/g/:v<maxLen(9)>"},
		{map[string]any{"type": "string", "minLength": 4.0, "maxLength": 4.0}, "/h/:v<len(4)>"},
		{map[string]any{"type": "string", "minLength": 2.0, "maxLength": 6.0}, "/i/:v<betweenLen(2,6)>"},
		{map[string]any{"type": "integer", "minimum": 5.0}, "/j/:v<min(5)>"},
		{map[string]any{"type": "integer", "maximum": 7.0}, "/k/:v<max(7)>"},
		{map[string]any{"type": "integer", "minimum": 1.0, "maximum": 10.0}, "/l/:v<range(1,10)>"},
		{map[string]any{"type": "string", "pattern": "^[a-z]+$"}, `/m/:v<regex(^[a-z]+$)>`},
		{map[string]any{"type": "string", "format": "date"}, "/n/:v<datetime(2006-01-02)>"},
		// An unknown constraint must not invent a type.
		{map[string]any{"type": "string"}, "/o/:v<nonexistent>"},
		// The first constraint that pins a type keeps it.
		{map[string]any{"type": "integer", "minimum": 5.0}, "/p/:v<int;min(5)>"},
		// A lowercase alias resolves like the router resolves it.
		{map[string]any{"type": "string", "minLength": 2.0}, "/q/:v<minlen(2)>"},
	}

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		for _, tc := range cases {
			app.Get(tc.pattern, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		}
	})

	paths := requireMap(t, spec["paths"])
	for _, tc := range cases {
		openAPIPath := tc.pattern[:strings.IndexByte(tc.pattern, ':')] + "{v}"
		op := requireMap(t, requireMap(t, paths[openAPIPath])["get"])
		params, ok := op["parameters"].([]any)
		require.Truef(t, ok, "%s has no parameters", tc.pattern)
		require.Lenf(t, params, 1, "%s", tc.pattern)
		require.Equalf(t, tc.expected, requireMap(t, requireMap(t, params[0])["schema"]), "%s", tc.pattern)
	}
}

// Test_OpenAPI_ConstraintSchemaKeepsExplicitParameter asserts that AddParameter
// still overrides a constraint-derived schema for the same path parameter.
func Test_OpenAPI_ConstraintSchemaKeepsExplicitParameter(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		app.Get("/items/:id<int>", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
			AddParameter(fiber.RouteParameter{
				Name:        "id",
				In:          "path",
				Description: "item identifier",
				Schema:      map[string]any{"type": "integer", "format": "int64"},
			})
	})

	op := requireMap(t, requireMap(t, requireMap(t, spec["paths"])["/items/{id}"])["get"])
	params, ok := op["parameters"].([]any)
	require.True(t, ok)
	require.Len(t, params, 1)

	param := requireMap(t, params[0])
	require.Equal(t, "item identifier", param["description"])
	require.Equal(t, map[string]any{"type": "integer", "format": "int64"}, requireMap(t, param["schema"]))
}

// Test_OpenAPI_ConstraintSchemaOptionalParameter asserts the constraint survives
// the optional-parameter fork, which clones the walk state per variant.
func Test_OpenAPI_ConstraintSchemaOptionalParameter(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		app.Get("/pages/:page<int>?", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	})

	paths := requireMap(t, spec["paths"])
	op := requireMap(t, requireMap(t, paths["/pages/{page}"])["get"])
	params, ok := op["parameters"].([]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{"type": "integer"}, requireMap(t, requireMap(t, params[0])["schema"]))

	// The variant that omits the optional parameter declares none.
	bare := requireMap(t, requireMap(t, paths["/pages"])["get"])
	require.NotContains(t, bare, "parameters")
}

// Test_OpenAPI_EscapedConstraintDelimiter asserts an escaped '>' does not close
// the span early, which used to leak constraint text into the path.
func Test_OpenAPI_EscapedConstraintDelimiter(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		app.Get(`/a/:v<regex(^a\>b$)>/tail`, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	})

	paths := requireMap(t, spec["paths"])
	require.Contains(t, paths, "/a/{v}/tail")
	require.Len(t, paths, 1)

	op := requireMap(t, requireMap(t, paths["/a/{v}/tail"])["get"])
	params, ok := op["parameters"].([]any)
	require.True(t, ok)
	require.Len(t, params, 1)
	require.Equal(t, "v", requireMap(t, params[0])["name"])
}

// Test_OpenAPI_SelfHostedSwaggerAssets locks the documented offline behavior:
// all three URLs overridden leaves no outbound request; empty means default.
func Test_OpenAPI_SelfHostedSwaggerAssets(t *testing.T) {
	t.Parallel()

	t.Run("all three overridden leaves no external URL", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Use(New(Config{
			SwaggerCSSURL:              "/swagger-ui/swagger-ui.css",
			SwaggerBundleURL:           "/swagger-ui/swagger-ui-bundle.js",
			SwaggerStandalonePresetURL: "/swagger-ui/swagger-ui-standalone-preset.js",
		}))

		status, body := specBodyOf(t, app, "/swagger")
		require.Equal(t, fiber.StatusOK, status)
		require.NotContains(t, body, "://")
		require.Contains(t, body, `href="/swagger-ui/swagger-ui.css"`)
		require.Contains(t, body, `src="/swagger-ui/swagger-ui-bundle.js"`)
		require.Contains(t, body, `src="/swagger-ui/swagger-ui-standalone-preset.js"`)
	})

	t.Run("an omitted asset URL falls back to the default", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Use(New(Config{
			SwaggerCSSURL:    "/swagger-ui/swagger-ui.css",
			SwaggerBundleURL: "/swagger-ui/swagger-ui-bundle.js",
			// Left empty on purpose: this is the offline footgun.
		}))

		_, body := specBodyOf(t, app, "/swagger")
		require.Contains(t, body, ConfigDefault.SwaggerStandalonePresetURL)
	})
}

// Test_OpenAPI_CanonicalPathAdoptsParamNames asserts an operation folded onto an
// equivalent template adopts its names, or the document is invalid.
func Test_OpenAPI_CanonicalPathAdoptsParamNames(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		app.Get("/files/:id", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Post("/files/:name", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	})

	paths := requireMap(t, spec["paths"])
	require.Contains(t, paths, "/files/{id}")
	require.NotContains(t, paths, "/files/{name}")

	item := requireMap(t, paths["/files/{id}"])
	for _, method := range []string{"get", "post"} {
		op := requireMap(t, item[method])
		params, ok := op["parameters"].([]any)
		require.Truef(t, ok, "%s has no parameters", method)
		require.Lenf(t, params, 1, "%s", method)
		require.Equalf(t, "id", requireMap(t, params[0])["name"], "%s declares the canonical name", method)
	}
}

// Test_OpenAPI_ServerVariableEnumDetached asserts the config detaches server
// variable enum slices from the caller, which maps.Clone alone did not do.
func Test_OpenAPI_ServerVariableEnumDetached(t *testing.T) {
	t.Parallel()

	enum := []string{"eu", "us"}
	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New(Config{
		Servers: []Server{{
			URL:       "https://{region}.example.com",
			Variables: map[string]ServerVariable{"region": {Default: "eu", Enum: enum}},
		}},
	}))

	// Mutating the caller's slice after New must not reach the served document.
	enum[0] = "mutated"

	_, body := specBodyOf(t, app, "/openapi.json")
	require.Contains(t, body, `"eu"`)
	require.NotContains(t, body, "mutated")
}

// Test_AdoptCanonicalParamNames covers the rename directly, including constraint
// keys and the guard a hierarchy match makes unreachable in practice.
func Test_AdoptCanonicalParamNames(t *testing.T) {
	t.Parallel()

	t.Run("renames names, constraints and aliases", func(t *testing.T) {
		t.Parallel()

		variant := pathVariant{
			Path:             "/files/{name}",
			ParamNames:       []string{"name"},
			ParamConstraints: map[string]string{"name": "int"},
			PathParamAliases: map[string]string{"name": "name"},
		}
		adoptCanonicalParamNames(&variant, []string{"id"})

		require.Equal(t, []string{"id"}, variant.ParamNames)
		require.Equal(t, map[string]string{"id": "int"}, variant.ParamConstraints)
		require.Equal(t, map[string]string{"name": "id"}, variant.PathParamAliases)
	})

	t.Run("a count mismatch leaves the variant alone", func(t *testing.T) {
		t.Parallel()

		variant := pathVariant{
			ParamNames:       []string{"a", "b"},
			ParamConstraints: map[string]string{"a": "int"},
		}
		adoptCanonicalParamNames(&variant, []string{"x"})

		require.Equal(t, []string{"a", "b"}, variant.ParamNames)
		require.Equal(t, map[string]string{"a": "int"}, variant.ParamConstraints)
	})
}

// Test_OpenAPI_CanonicalPathKeepsConstraintSchema asserts the adopted names
// keep their constraint-derived schemas through the rename.
func Test_OpenAPI_CanonicalPathKeepsConstraintSchema(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		app.Get("/items/:id", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Post("/items/:code<int>", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	})

	item := requireMap(t, requireMap(t, spec["paths"])["/items/{id}"])
	op := requireMap(t, item["post"])
	params, ok := op["parameters"].([]any)
	require.True(t, ok)

	param := requireMap(t, params[0])
	require.Equal(t, "id", param["name"])
	// The constraint traveled with the rename.
	require.Equal(t, map[string]any{"type": "integer"}, requireMap(t, param["schema"]))
}

// Test_OpenAPI_DynamicMountPrefix covers a mount whose segment count varies;
// truncating at the first variable segment answered 404 on served paths.
func Test_OpenAPI_DynamicMountPrefix(t *testing.T) {
	t.Parallel()

	serve := func(t *testing.T, mount, request string) int {
		t.Helper()
		app := fiber.New()
		app.Get("/hello", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Use(mount, New())
		status, _ := specBodyOf(t, app, request)
		return status
	}

	cases := []struct {
		name    string
		mount   string
		request string
		status  int
	}{
		{"optional segment present", "/:tenant?", "/acme/openapi.json", fiber.StatusOK},
		{"optional segment omitted", "/:tenant?", "/openapi.json", fiber.StatusOK},
		{"optional segment serves the ui too", "/:tenant?", "/acme/swagger", fiber.StatusOK},
		// One optional segment cannot consume two, so this is not a target.
		{"beyond the segment bound", "/:tenant?", "/a/b/openapi.json", fiber.StatusNotFound},
		{"required parameter", "/:tenant", "/acme/openapi.json", fiber.StatusOK},
		{"optional between literals", "/api/:tenant?/docs", "/api/acme/docs/openapi.json", fiber.StatusOK},
		{"greedy segment", "/files/*", "/files/a/b/openapi.json", fiber.StatusOK},
		// Under a greedy mount every split is tried and none is a target.
		{"greedy segment with no target", "/files/*", "/files/a/b", fiber.StatusNotFound},
		// No split of a "+" mount leaves a target here, so resolution reports
		// nothing and the static truncation ("/files") still serves it.
		{"greedy segment falls back to the static prefix", "/files/+", "/files/openapi.json", fiber.StatusOK},
		// Static mounts keep their exact-path behavior.
		{"static mount", "/v1", "/v1/openapi.json", fiber.StatusOK},
		{"static mount is not global", "/v1", "/openapi.json", fiber.StatusNotFound},
		{"global mount", "/", "/openapi.json", fiber.StatusOK},
		{"global mount ignores nested lookalikes", "/", "/x/openapi.json", fiber.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.status, serve(t, tc.mount, tc.request))
		})
	}
}

// Test_PrefixSegmentBounds pins how many request segments each mount shape can
// consume; -1 means a greedy segment leaves it unbounded.
func Test_PrefixSegmentBounds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		pattern string
		lowest  int
		highest int
		dynamic bool
	}{
		{"/", 0, 0, false},
		{"/v1", 1, 1, false},
		{"/v1/api", 2, 2, false},
		{"/:tenant", 1, 1, true},
		{"/:tenant?", 0, 1, true},
		{"/api/:tenant?/docs", 2, 3, true},
		{"/files/*", 1, -1, true},
		{"/files/+", 2, -1, true},
		// A constraint is not a parameter marker of its own.
		{"/:id<int>", 1, 1, true},
	}

	for _, tc := range cases {
		t.Run(tc.pattern, func(t *testing.T) {
			t.Parallel()
			minSegments, maxSegments, dynamic := prefixSegmentBounds(tc.pattern)
			require.Equal(t, tc.lowest, minSegments, "min")
			require.Equal(t, tc.highest, maxSegments, "max")
			require.Equal(t, tc.dynamic, dynamic, "dynamic")
		})
	}
}

// Test_PathPrefixSegments covers the split helper, including asking for more
// segments than the path has.
func Test_PathPrefixSegments(t *testing.T) {
	t.Parallel()

	prefix, ok := pathPrefixSegments("/acme/openapi.json", 1)
	require.True(t, ok)
	require.Equal(t, "/acme", prefix)

	prefix, ok = pathPrefixSegments("/a/b/c", 2)
	require.True(t, ok)
	require.Equal(t, "/a/b", prefix)

	prefix, ok = pathPrefixSegments("/acme/openapi.json", 0)
	require.True(t, ok)
	require.Empty(t, prefix)

	_, ok = pathPrefixSegments("/acme/openapi.json", 2)
	require.False(t, ok)

	// An empty path has no segment to give.
	_, ok = pathPrefixSegments("", 1)
	require.False(t, ok)

	require.Equal(t, 0, countPathSegments("/"))
	require.Equal(t, 1, countPathSegments("/a"))
	require.Equal(t, 3, countPathSegments("/a/b/c"))
}

// Test_OpenAPI_LiteralBracesEncoded asserts braces from a route's literal text
// are percent-encoded, since OpenAPI would read them as an undeclared template.
func Test_OpenAPI_LiteralBracesEncoded(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		app.Get("/files/{raw}", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Get("/users/:id", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	})

	paths := requireMap(t, spec["paths"])
	require.Contains(t, paths, "/files/%7Braw%7D")
	require.NotContains(t, paths, "/files/{raw}")
	// Braces introduced for a real parameter are untouched.
	require.Contains(t, paths, "/users/{id}")
}

// Test_OpenAPI_LicenseIdentifierExcludesURL asserts the License Object emits
// identifier or url but never both, and that 3.0 still drops the identifier.
func Test_OpenAPI_LicenseIdentifierExcludesURL(t *testing.T) {
	t.Parallel()

	register := func(app *fiber.App) {
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	}
	license := &License{Name: "MIT", Identifier: "MIT", URL: "https://example.com/license"}

	spec := fetchSpecWithConfig(t, Config{OpenAPIVersion: "3.1.0", License: license}, register)
	got := requireMap(t, requireMap(t, spec["info"])["license"])
	require.Equal(t, "MIT", got["identifier"])
	require.NotContains(t, got, "url")

	// 3.0 has no identifier, so the url survives there instead.
	spec = fetchSpecWithConfig(t, Config{OpenAPIVersion: "3.0.0", License: license}, register)
	got = requireMap(t, requireMap(t, spec["info"])["license"])
	require.NotContains(t, got, "identifier")
	require.Equal(t, "https://example.com/license", got["url"])

	// The caller's License is never mutated.
	require.Equal(t, "MIT", license.Identifier)
	require.Equal(t, "https://example.com/license", license.URL)
}

// Test_OpenAPI_TypedConfigContainersDetached asserts the config deep copy reaches
// typed maps and slices, not only the built-in any-keyed ones.
func Test_OpenAPI_TypedConfigContainersDetached(t *testing.T) {
	t.Parallel()

	type schema struct {
		Type string `json:"type"`
	}
	typed := map[string]schema{"User": {Type: "object"}}
	slice := []schema{{Type: "array"}}

	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New(Config{Components: map[string]any{"schemas": typed, "list": slice}}))

	// Mutating the caller's containers after New must not reach the document.
	typed["User"] = schema{Type: "mutated"}
	slice[0] = schema{Type: "mutated"}

	_, body := specBodyOf(t, app, "/openapi.json")
	require.NotContains(t, body, "mutated")
	require.Contains(t, body, `"object"`)
	require.Contains(t, body, `"array"`)
}

// Test_DeepCopyReflected covers the container clone directly: nil and non-container
// values pass through, and nested containers inside an `any` are cloned too.
func Test_DeepCopyReflected(t *testing.T) {
	t.Parallel()

	require.Equal(t, 42, deepCopyReflected(42))

	var nilMap map[string]int
	require.Nil(t, deepCopyReflected(nilMap))
	var nilSlice []int
	require.Nil(t, deepCopyReflected(nilSlice))

	// A typed map holding `any` values must clone the nested container.
	nested := map[string]any{"inner": map[string]any{"k": "v"}}
	src := map[string]map[string]any{"outer": nested}
	cloned, ok := deepCopyReflected(src).(map[string]map[string]any)
	require.True(t, ok)
	requireMap(t, nested["inner"])["k"] = "mutated"
	require.Equal(t, "v", requireMap(t, cloned["outer"]["inner"])["k"])

	// Slices of slices clone element by element.
	rows := [][]int{{1, 2}}
	clonedRows, ok := deepCopyReflected(rows).([][]int)
	require.True(t, ok)
	rows[0][0] = 99
	require.Equal(t, 1, clonedRows[0][0])
}

// Unit tests for the unexported helpers, covering branches the request-level
// tests above cannot reach directly.

func Test_uniqueOperationID(t *testing.T) {
	t.Parallel()

	used := map[string]struct{}{}
	// Empty id falls back to "operation".
	require.Equal(t, "operation", uniqueOperationID("", used))

	// Collisions get numeric suffixes.
	require.Equal(t, "op", uniqueOperationID("op", used))
	require.Equal(t, "op_2", uniqueOperationID("op", used))
	require.Equal(t, "op_3", uniqueOperationID("op", used))
}

func Test_appendOrReplaceParameter_Guards(t *testing.T) {
	t.Parallel()

	params := []parameter{{Name: "a", In: "query"}}
	index := map[string]int{"query:a": 0}

	// nil / empty Name / empty In are no-ops.
	require.Len(t, appendOrReplaceParameter(params, index, nil), 1)
	require.Len(t, appendOrReplaceParameter(params, index, &parameter{In: "query"}), 1)
	require.Len(t, appendOrReplaceParameter(params, index, &parameter{Name: "a"}), 1)

	// Existing key is replaced in place.
	replaced := appendOrReplaceParameter(params, index, &parameter{Name: "a", In: "query", Description: "x"})
	require.Len(t, replaced, 1)
	require.Equal(t, "x", replaced[0].Description)

	// New key is appended.
	added := appendOrReplaceParameter(params, index, &parameter{Name: "b", In: "query"})
	require.Len(t, added, 2)
}

func Test_schemaFrom(t *testing.T) {
	t.Parallel()

	require.Equal(t, map[string]any{"$ref": "#/x"}, schemaFrom(nil, "#/x", "string"))
	require.Equal(t, map[string]any{"type": "string"}, schemaFrom(nil, "", "string"))
	// No schema and no default type yields nil.
	require.Nil(t, schemaFrom(nil, "", ""))
	// Existing type is preserved.
	require.Equal(t, map[string]any{"type": "integer"}, schemaFrom(map[string]any{"type": "integer"}, "", "string"))
}

func Test_mediaTypesToContent(t *testing.T) {
	t.Parallel()

	// Empty list and only-empty entries both yield nil.
	require.Nil(t, mediaTypesToContent(nil, nil, "", nil, nil))
	require.Nil(t, mediaTypesToContent([]string{""}, nil, "", nil, nil))

	content := mediaTypesToContent([]string{"application/json"}, nil, "", nil, nil)
	require.Contains(t, content, "application/json")
	require.Empty(t, content["application/json"])
}

func Test_routeMediaTypeContent(t *testing.T) {
	t.Parallel()

	require.Nil(t, routeMediaTypeContent(nil))
	// Only an empty media-type key -> nothing emitted.
	require.Nil(t, routeMediaTypeContent(map[string]fiber.RouteMediaType{"": {Schema: map[string]any{"type": "string"}}}))

	out := routeMediaTypeContent(map[string]fiber.RouteMediaType{
		"":                 {Schema: map[string]any{"type": "string"}}, // skipped
		"application/json": {},                                         // empty entry -> {}
	})
	require.Len(t, out, 1)
	require.Empty(t, out["application/json"])
}

func Test_buildRequestBody_Guards(t *testing.T) {
	t.Parallel()

	require.Nil(t, buildRequestBody(nil))
	// Content that resolves to nothing -> no request body.
	require.Nil(t, buildRequestBody(&fiber.RouteRequestBody{Content: map[string]fiber.RouteMediaType{"": {}}}))
}

func Test_shouldIncludeRequestBody(t *testing.T) {
	t.Parallel()

	require.False(t, shouldIncludeRequestBody("", nil))
	require.False(t, shouldIncludeRequestBody("", &fiber.Route{Method: fiber.MethodPost}))
	require.False(t, shouldIncludeRequestBody(fiber.MIMEApplicationJSON, nil))
	// Any explicitly declared media type opts in, regardless of method; the
	// GET/HEAD strip in generateSpec owns the method rule.
	require.True(t, shouldIncludeRequestBody(fiber.MIMEApplicationJSON, &fiber.Route{Method: fiber.MethodPost}))
	require.True(t, shouldIncludeRequestBody(fiber.MIMETextPlain, &fiber.Route{Method: fiber.MethodPost, Consumes: fiber.MIMETextPlain}))
}

func Test_defaultResponseForMethod(t *testing.T) {
	t.Parallel()

	status, resp := defaultResponseForMethod(fiber.MethodGet, fiber.MIMEApplicationJSON)
	require.Equal(t, "200", status)
	require.Contains(t, resp.Content, fiber.MIMEApplicationJSON)

	// RFC 9110: HEAD answers as GET would, minus the content, so it mirrors the
	// GET status rather than claiming 204.
	headStatus, headResp := defaultResponseForMethod(fiber.MethodHead, fiber.MIMEApplicationJSON)
	require.Equal(t, status, headStatus)
	require.Equal(t, resp, headResp)

	status, resp = defaultResponseForMethod(fiber.MethodDelete, fiber.MIMEApplicationJSON)
	require.Equal(t, "204", status)
	require.Nil(t, resp.Content)

	status, resp = defaultResponseForMethod(fiber.MethodGet, "")
	require.Equal(t, "200", status)
	require.Nil(t, resp.Content)
}

func Test_buildServers_Internal(t *testing.T) {
	t.Parallel()

	// Empty-URL servers are skipped; Name kept for 3.2.
	servers := buildServers(&Config{
		OpenAPIVersion: versionOpenAPI32,
		Servers:        []Server{{URL: ""}, {URL: "https://x", Name: "n"}},
	})
	require.Len(t, servers, 1)
	require.Equal(t, "n", servers[0].Name)

	// Name cleared below 3.2.
	servers = buildServers(&Config{
		OpenAPIVersion: versionOpenAPI31,
		Servers:        []Server{{URL: "https://x", Name: "n"}},
	})
	require.Len(t, servers, 1)
	require.Empty(t, servers[0].Name)

	// All servers invalid -> fall back to ServerURL.
	servers = buildServers(&Config{OpenAPIVersion: versionOpenAPI31, Servers: []Server{{URL: ""}}, ServerURL: "https://y"})
	require.Equal(t, "https://y", servers[0].URL)
}

func Test_buildComponents_MergeSecuritySchemes(t *testing.T) {
	t.Parallel()

	require.Nil(t, buildComponents(&Config{}))

	components := buildComponents(&Config{
		Components:      map[string]any{"securitySchemes": map[string]any{"a": map[string]any{"type": "http"}}},
		SecuritySchemes: map[string]any{"b": map[string]any{"type": "apiKey"}},
	})
	schemes := requireMap(t, components["securitySchemes"])
	require.Contains(t, schemes, "a")
	require.Contains(t, schemes, "b")
}

func Test_mergeRouteParameters_Internal(t *testing.T) {
	t.Parallel()

	index := map[string]int{}
	out := mergeRouteParameters(nil, index, []fiber.RouteParameter{
		{Name: "  "},              // blank name -> skipped
		{Name: "q"},               // empty In -> defaults to query
		{Name: "h", In: "Header"}, // normalized to lowercase
	})
	require.Len(t, out, 2)
	require.Equal(t, "query", out[0].In)
	require.Equal(t, "header", out[1].In)
}

func Test_remapRouteParameters_DropsUnknownPathParam(t *testing.T) {
	t.Parallel()

	out := remapRouteParameters(
		[]fiber.RouteParameter{
			{Name: "ghost", In: "path"}, // not a real path param -> dropped
			{Name: "q", In: "query"},    // kept
		},
		map[string]string{},
		[]string{"id"},
	)
	require.Len(t, out, 1)
	require.Equal(t, "q", out[0].Name)
}

func Test_sanitizeParamNames(t *testing.T) {
	t.Parallel()

	// "*"/"+" are trimmed (so trimmed falls back to the original), then each
	// disallowed char becomes "_".
	require.Equal(t, "___", sanitizeOpenAPIParamName("***", 1))
	// A fully empty name sanitizes to the positional fallback.
	require.Equal(t, "param1", sanitizeOpenAPIParamName("", 1))
	require.Equal(t, "id", sanitizeOpenAPIParamName("id", 1))

	require.Equal(t, wildcardParamName, sanitizeOpenAPIWildcardParamName("*", 1))
	require.Equal(t, wildcardParamName, sanitizeOpenAPIWildcardParamName("_._", 1))
	require.NotEmpty(t, sanitizeOpenAPIWildcardParamName("*5", 1))
	require.Contains(t, sanitizeOpenAPIWildcardParamName("*foo", 1), wildcardParamName)
}

func Test_resolveParamNames(t *testing.T) {
	t.Parallel()

	// Blank extracted + blank params entry keeps raw empty, sanitizes to paramN.
	resolved := resolveOpenAPIPathParamName(0, "", []string{""})
	require.Empty(t, resolved.raw)
	require.Equal(t, "param1", resolved.openAPI)

	// Provided param name overrides the extracted token.
	resolved = resolveOpenAPIPathParamName(0, "x", []string{"override"})
	require.Equal(t, "override", resolved.raw)

	// Wildcard with a provided name.
	wild := resolveOpenAPIWildcardParamName(0, []string{"rest"})
	require.Equal(t, "rest", wild.raw)
	require.Contains(t, wild.openAPI, wildcardParamName)
}

func Test_buildOpenAPIPathVariants_Edge(t *testing.T) {
	t.Parallel()

	// Empty path -> single "/" variant.
	variants := buildOpenAPIPathVariants("", nil)
	require.Len(t, variants, 1)
	require.Equal(t, "/", variants[0].Path)

	// Nested angle-bracket constraint exercises the depth counter.
	variants = buildOpenAPIPathVariants("/:id<range(1<2)>", nil)
	require.Equal(t, "/{id}", variants[0].Path)

	// A simple parameterized path yields exactly its template as the first variant.
	require.Equal(t, "/{id}", buildOpenAPIPathVariants("/:id", nil)[0].Path)

	// Generated variants must always be unique.
	dup := buildOpenAPIPathVariants("/:a?/:a?", nil)
	seen := map[string]struct{}{}
	for _, v := range dup {
		key := v.Path + "|" + strings.Join(v.ParamNames, ",")
		_, exists := seen[key]
		require.False(t, exists, "variants must be unique")
		seen[key] = struct{}{}
	}
}

func Test_inferExampleValue_NoType(t *testing.T) {
	t.Parallel()

	// Missing/!string type returns the raw value unchanged.
	require.Equal(t, "x", inferExampleValue("x", map[string]any{}))
	require.Equal(t, "x", inferExampleValue("x", map[string]any{"type": 123}))
}

func Test_SchemaOf_UnsupportedSliceAndMapElements(t *testing.T) {
	t.Parallel()

	type withUnsupported struct {
		M  map[string]chan int `json:"m"`
		OK string              `json:"ok"`
		Ch []chan int          `json:"ch"`
	}

	// encoding/json fails outright on these values, so the fields are skipped
	// rather than documented as something the handler could never produce.
	schema := SchemaOf(withUnsupported{})
	props := requireMap(t, schema["properties"])

	require.NotContains(t, props, "ch")
	require.NotContains(t, props, "m")
	require.Contains(t, props, "ok")
}

func Test_SchemaOf_SelfReferentialTypes(t *testing.T) {
	t.Parallel()

	// Recursive slice, map and pointer types are legal Go; reflection must
	// terminate on them instead of overflowing the stack.
	type recMap map[string]recMap
	type recSlice []recSlice

	require.NotPanics(t, func() {
		require.Equal(t, schemaTypeObject, SchemaOf(recMap{})[schemaKeyType])
		require.Equal(t, "array", SchemaOf(recSlice{})["type"])
	})
}

func Test_OpenAPI_SpecMarshalError(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	// A non-marshalable operation extension makes spec marshaling fail.
	app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		OperationExtension(map[string]any{"bad": func() {}})
	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func Test_OpenAPI_SwaggerOptionsMarshalError(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New(Config{SwaggerOptions: map[string]any{"bad": func() {}}}))

	req := httptest.NewRequest(fiber.MethodGet, "/swagger", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// Test_StringKeyedEntries covers reading an existing components value that the
// caller may have typed however they like.
func Test_StringKeyedEntries(t *testing.T) {
	t.Parallel()

	type scheme struct {
		Type string `json:"type"`
	}

	require.Nil(t, stringKeyedEntries(nil))
	require.Nil(t, stringKeyedEntries("not a map"))
	require.Nil(t, stringKeyedEntries(map[int]string{1: "a"}), "non-string keys cannot be merged")
	require.Nil(t, stringKeyedEntries(map[string]scheme(nil)), "a nil typed map has nothing to merge")

	untyped := map[string]any{"a": 1}
	require.Equal(t, untyped, stringKeyedEntries(untyped))

	typed := map[string]scheme{"bearer": {Type: "http"}}
	require.Equal(t, map[string]any{"bearer": scheme{Type: "http"}}, stringKeyedEntries(typed))
}

// Test_OpenAPI_TypedSecuritySchemesMerge pins a typed Components securityScheme
// map against Config.SecuritySchemes: the two must merge, not overwrite.
func Test_OpenAPI_TypedSecuritySchemesMerge(t *testing.T) {
	t.Parallel()

	type scheme struct {
		Type   string `json:"type"`
		Scheme string `json:"scheme"`
	}

	cfg := configDefault(Config{
		Components: map[string]any{
			"securitySchemes": map[string]scheme{
				"fromComponents": {Type: "http", Scheme: "basic"},
			},
		},
		SecuritySchemes: map[string]any{
			"fromConfig": map[string]any{"type": "http", "scheme": "bearer"},
		},
	})

	schemes := requireMap(t, buildComponents(&cfg)["securitySchemes"])
	require.Contains(t, schemes, "fromComponents", "the caller's typed schemes were dropped")
	require.Contains(t, schemes, "fromConfig")
}
