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

// Test_OpenAPI_Cache verifies the spec is regenerated per request, so routes
// added after the first request are reflected without a process restart.
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
	require.Equal(t, "querystring", requireMap(t, params[0])["in"])

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

// Test_OpenAPI_OptionalVariantDoesNotOverwrite verifies an optional-parameter
// variant never overwrites the documentation of an earlier registered route at
// the same path and method, matching router dispatch precedence.
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

// Test_OpenAPI_ResponseSchemaWithoutMediaType verifies a response schema or
// example documented without media types falls back to the route's Produces
// type instead of being dropped.
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

// Test_OpenAPI_DuplicateSanitizedParamNames verifies distinct Fiber parameters
// that sanitize to the same identifier get unique names in the path template,
// as required by the OpenAPI specification.
func Test_OpenAPI_DuplicateSanitizedParamNames(t *testing.T) {
	t.Parallel()

	variants := buildOpenAPIPathVariants("/x/:na_ve/:naïve", nil)
	require.Len(t, variants, 1)
	require.Equal(t, "/x/{na_ve}/{na_ve_2}", variants[0].Path)
	require.Equal(t, []string{"na_ve", "na_ve_2"}, variants[0].ParamNames)
}

// Test_OpenAPI_ConcurrentDocsAndSpecRequests guards the locking contract: spec
// generation snapshots routes under the router lock, so serving /openapi.json
// while other goroutines mutate route metadata must be race-free (run with -race).
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
