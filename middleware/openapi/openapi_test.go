package openapi

import (
	"encoding/json"
	"io"
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

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", nil)
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
	getOp := operations["get"].(map[string]any)
	require.Contains(t, getOp, "responses")
	responses := getOp["responses"].(map[string]any)
	require.Contains(t, responses, "200")
}

func Test_OpenAPI_JSONEquality(t *testing.T) {
	app := fiber.New()

	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Name("listUsers").Produces(fiber.MIMEApplicationJSON)

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	rootOps := map[string]operation{}
	for _, m := range app.Config().RequestMethods {
		if m == fiber.MethodConnect {
			continue
		}
		lower := strings.ToLower(m)
		upper := strings.ToUpper(m)
		op := operation{
			Summary:     upper + " /",
			Description: "",
			Responses: map[string]response{
				"200": {Description: "OK", Content: map[string]map[string]any{fiber.MIMETextPlain: {}}},
			},
		}
		switch m {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace:
		default:
			op.RequestBody = &requestBody{Content: map[string]map[string]any{fiber.MIMETextPlain: {}}}
		}
		rootOps[lower] = op
	}
	expected := openAPISpec{
		OpenAPI: "3.0.0",
		Info:    openAPIInfo{Title: "Fiber API", Version: "1.0.0"},
		Paths: map[string]map[string]operation{
			"/": rootOps,
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

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	rootOps := map[string]operation{}
	for _, m := range app.Config().RequestMethods {
		if m == fiber.MethodConnect {
			continue
		}
		lower := strings.ToLower(m)
		upper := strings.ToUpper(m)
		op := operation{
			Summary:     upper + " /",
			Description: "",
			Responses: map[string]response{
				"200": {Description: "OK", Content: map[string]map[string]any{fiber.MIMETextPlain: {}}},
			},
		}
		switch m {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace:
		default:
			op.RequestBody = &requestBody{Content: map[string]map[string]any{fiber.MIMETextPlain: {}}}
		}
		rootOps[lower] = op
	}
	expected := openAPISpec{
		OpenAPI: "3.0.0",
		Info:    openAPIInfo{Title: "Fiber API", Version: "1.0.0"},
		Paths: map[string]map[string]operation{
			"/": rootOps,
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
	require.Equal(t, string(exp), string(body))
}

func Test_OpenAPI_RawJSONFile(t *testing.T) {
	app := fiber.New()

	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Name("listUsers").Produces(fiber.MIMEApplicationJSON)

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	expected, err := os.ReadFile("testdata/openapi.json")
	require.NoError(t, err)

	require.Equal(t, string(expected), string(body))
}

func Test_OpenAPI_OperationConfig(t *testing.T) {
	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"hello": "world"}) })

	app.Use(New(Config{
		Operations: map[string]Operation{
			"GET /users": {
				Id:          "listUsersCustom",
				Summary:     "List users",
				Description: "Returns all users",
				Tags:        []string{"users"},
				Deprecated:  true,
				Consumes:    fiber.MIMEApplicationJSON,
				Produces:    fiber.MIMEApplicationJSON,
			},
		},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))

	op := spec.Paths["/users"]["get"]
	require.Equal(t, "listUsersCustom", op.OperationID)
	require.Equal(t, "List users", op.Summary)
	require.Equal(t, "Returns all users", op.Description)
	require.ElementsMatch(t, []string{"users"}, op.Tags)
	require.True(t, op.Deprecated)
	require.Contains(t, op.Responses["200"].Content, fiber.MIMEApplicationJSON)
	require.NotNil(t, op.RequestBody)
	require.Contains(t, op.RequestBody.Content, fiber.MIMEApplicationJSON)
}

func Test_OpenAPI_RouteMetadata(t *testing.T) {
	app := fiber.New()
	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		Summary("List users").Description("User list").Produces(fiber.MIMEApplicationJSON).
		Tags("users", "read").Deprecated()

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", nil)
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
}

// getPaths is a helper that mounts the middleware, performs the request and
// decodes the resulting OpenAPI specification paths.
func getPaths(t *testing.T, app *fiber.App) map[string]map[string]any {
	t.Helper()

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", nil)
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
		method   string
		register func(*fiber.App)
	}{
		{fiber.MethodGet, func(a *fiber.App) { a.Get("/method", handler) }},
		{fiber.MethodPost, func(a *fiber.App) { a.Post("/method", handler) }},
		{fiber.MethodPut, func(a *fiber.App) { a.Put("/method", handler) }},
		{fiber.MethodPatch, func(a *fiber.App) { a.Patch("/method", handler) }},
		{fiber.MethodDelete, func(a *fiber.App) { a.Delete("/method", handler) }},
		{fiber.MethodHead, func(a *fiber.App) { a.Head("/method", handler) }},
		{fiber.MethodOptions, func(a *fiber.App) { a.Options("/method", handler) }},
		{fiber.MethodTrace, func(a *fiber.App) { a.Trace("/method", handler) }},
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

	app.Get("/users/:id", func(c fiber.Ctx) error { return c.SendString(c.Params("id")) })

	paths := getPaths(t, app)
	require.Contains(t, paths, "/users/{id}")
	require.Contains(t, paths["/users/{id}"], "get")
	op := paths["/users/{id}"]["get"].(map[string]any)
	params := op["parameters"].([]any)
	require.Len(t, params, 1)
	p0 := params[0].(map[string]any)
	require.Equal(t, "id", p0["name"])
	require.Equal(t, "path", p0["in"])
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
	op := paths["/api/users"]["get"].(map[string]any)
	require.Equal(t, "List users", op["summary"])
	require.Equal(t, "Group users", op["description"])
	require.ElementsMatch(t, []any{"users"}, op["tags"].([]any))
	require.Equal(t, true, op["deprecated"])
	resp := op["responses"].(map[string]any)
	cont := resp["200"].(map[string]any)["content"].(map[string]any)
	require.Contains(t, cont, fiber.MIMEApplicationJSON)
}

func Test_OpenAPI_NoRoutes(t *testing.T) {
	app := fiber.New()

	paths := getPaths(t, app)

	require.Len(t, paths, 1)
	require.Contains(t, paths, "/")
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

	req := httptest.NewRequest(fiber.MethodGet, "/api/v2/openapi.json", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Contains(t, spec.Paths, "/api/v2/users")
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

	req := httptest.NewRequest(fiber.MethodGet, "/spec.json", nil)
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

func Test_OpenAPI_Next(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{Next: func(fiber.Ctx) bool { return true }}))

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", nil)
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
	op := paths["/users/{uid}/books/{bid}"]["get"].(map[string]any)
	params := op["parameters"].([]any)
	require.Len(t, params, 2)
	p0 := params[0].(map[string]any)
	p1 := params[1].(map[string]any)
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

	op := paths["/users"]["post"].(map[string]any)
	rb := op["requestBody"].(map[string]any)
	reqContent := rb["content"].(map[string]any)
	require.Contains(t, reqContent, fiber.MIMEApplicationJSON)

	resp := op["responses"].(map[string]any)["200"].(map[string]any)
	cont := resp["content"].(map[string]any)
	require.Contains(t, cont, fiber.MIMEApplicationXML)
}

func Test_OpenAPI_NoRequestBodyForGET(t *testing.T) {
	app := fiber.New()

	app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	paths := getPaths(t, app)
	op := paths["/users"]["get"].(map[string]any)
	require.NotContains(t, op, "requestBody")
}

func Test_OpenAPI_Cache(t *testing.T) {
	app := fiber.New()

	app.Get("/first", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Contains(t, spec.Paths, "/first")

	app.Get("/second", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req = httptest.NewRequest(fiber.MethodGet, "/openapi.json", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.NotContains(t, spec.Paths, "/second")
}
