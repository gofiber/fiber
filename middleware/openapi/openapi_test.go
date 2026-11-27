package openapi

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"weak"

	"github.com/gofiber/fiber/v3/internal/apptest"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
)

// omOK is the handler most spec tests register. It is a func literal, so the generated summaries stay anonymous.
var omOK = func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }

// omSpec registers routes on a new app, mounts New(cfg...) on it and decodes the served document.
func omSpec(t *testing.T, register func(app *fiber.App), cfg ...Config) map[string]any {
	t.Helper()
	app := fiber.New()
	register(app)
	app.Use(New(cfg...))
	return fetchJSON(t, app)
}

// omT is the *testing.T a document check receives; a check is not a helper, so a failure points at its assertion.
type omT struct{ *testing.T }

// omAt walks a decoded document: a string key indexes an object and an int indexes an array.
func omAt(tb testing.TB, v any, keys ...any) any {
	tb.Helper()
	for _, key := range keys {
		switch k := key.(type) {
		case string:
			m, ok := v.(map[string]any)
			require.Truef(tb, ok, "%v is not an object", v)
			v = m[k]
		case int:
			items, ok := v.([]any)
			require.Truef(tb, ok, "%v is not an array", v)
			require.Greater(tb, len(items), k, "index %d is out of range", k)
			v = items[k]
		default:
			require.FailNow(tb, "unsupported key", "%v", key)
		}
	}
	return v
}

// omMap is omAt for a value that must be an object.
func omMap(tb testing.TB, v any, keys ...any) map[string]any {
	tb.Helper()
	m, ok := omAt(tb, v, keys...).(map[string]any)
	require.True(tb, ok, "not an object")
	return m
}

// omHas checks that every key in want is present in got with the same value.
func omHas(tb testing.TB, got, want map[string]any) {
	tb.Helper()
	for key, value := range want {
		require.Equalf(tb, value, got[key], "key %q", key)
	}
}

// omDocCase registers routes, mounts New(cfg) and checks the decoded document.
type omDocCase struct {
	register func(app *fiber.App)
	check    func(t omT, spec map[string]any)
	name     string
	cfg      Config
}

// omRunDocCases runs each case as a parallel subtest on an app of its own.
func omRunDocCases(t *testing.T, cases []omDocCase) {
	t.Helper()
	for i := range cases {
		tc := &cases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.check(omT{t}, omSpec(t, tc.register, tc.cfg))
		})
	}
}

func Test_OpenAPI_Generate(t *testing.T) {
	t.Parallel()
	spec := omSpec(t, func(app *fiber.App) {
		app.Get("/users", omOK)
		app.Post("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })
	})
	require.Contains(t, omMap(t, spec, "paths", "/users"), "post")
	require.Contains(t, omMap(t, spec, "paths", "/users", "get", "responses"), "200")
}

func Test_OpenAPI_JSONEquality(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Doc(app.Get("/users", omOK).Name("listUsers")).Produces(fiber.MIMEApplicationJSON)
	app.Use(New())
	body := fetchSpecBody(t, app)

	exp, err := json.Marshal(openAPISpec{
		OpenAPI: "3.1.0",
		Info:    openAPIInfo{Title: "Fiber API", Version: "1.0.0"},
		Paths: map[string]map[string]operation{"/users": {"get": {
			OperationID: "listUsers",
			Summary:     "GET /users",
			Description: "",
			Responses:   map[string]response{"200": {Description: "OK", Content: map[string]map[string]any{fiber.MIMEApplicationJSON: {}}}},
		}}},
	})
	require.NoError(t, err)
	require.JSONEq(t, string(exp), string(body))

	file, err := os.ReadFile("testdata/openapi.json")
	require.NoError(t, err)
	require.JSONEq(t, string(file), string(body))
}

func Test_OpenAPI_RouteDocuments(t *testing.T) {
	t.Parallel()
	created := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) }
	omRunDocCases(t, []omDocCase{
		{name: "route helper metadata", register: func(app *fiber.App) {
			Doc(app.Post("/users", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"hello": "world"}) }).
				Name("createUserCustom")).Summary("Create user").Description("Creates users").Tags("users").Deprecated().
				Consumes(fiber.MIMEApplicationJSON).Produces(fiber.MIMEApplicationJSON).
				ParameterWithExample("pageSize", "query", true,
					map[string]any{"type": "integer"}, "", "Page size", nil, nil).
				RequestBodyWithExample("Custom payload", true, map[string]any{"type": "object"}, "", nil,
					nil, fiber.MIMEApplicationJSON).
				ResponseWithExample(fiber.StatusCreated, "Created", map[string]any{"type": "object"}, "", nil,
					nil, fiber.MIMEApplicationJSON)
		}, check: func(t omT, spec map[string]any) {
			op := omMap(t, spec, "paths", "/users", "post")
			omHas(t, op, map[string]any{
				"operationId": "createUserCustom", "summary": "Create user", "description": "Creates users",
				"tags": []any{"users"}, "deprecated": true,
			})
			require.Len(t, omMap(t, op, "responses"), 1)
			require.Contains(t, omMap(t, op, "responses", "201", "content"), fiber.MIMEApplicationJSON)
			omHas(t, omMap(t, op, "requestBody"), map[string]any{"description": "Custom payload", "required": true})
			require.Contains(t, omMap(t, op, "requestBody", "content"), fiber.MIMEApplicationJSON)
			require.Len(t, omAt(t, op, "parameters"), 1)
			omHas(t, omMap(t, op, "parameters", 0),
				map[string]any{"name": "pageSize", "schema": map[string]any{"type": "integer"}})
		}},
		{name: "route metadata", register: func(app *fiber.App) {
			Doc(app.Get("/users", omOK)).Summary("List users").Description("User list").Produces(fiber.MIMEApplicationJSON).
				Parameter("trace-id", "header", true, nil, "Tracing identifier").Tags("users", "read").Deprecated()
		}, check: func(t omT, spec map[string]any) {
			op := omMap(t, spec, "paths", "/users", "get")
			omHas(t, op, map[string]any{"summary": "List users", "description": "User list", "deprecated": true})
			require.ElementsMatch(t, []any{"users", "read"}, omAt(t, op, "tags"))
			require.Contains(t, omMap(t, op, "responses", "200", "content"), fiber.MIMEApplicationJSON)
			require.Len(t, omAt(t, op, "parameters"), 1)
			omHas(t, omMap(t, op, "parameters", 0), map[string]any{
				"name": "trace-id", "in": "header", "description": "Tracing identifier",
			})
		}},
		{name: "group metadata", register: func(app *fiber.App) {
			api := app.Group("/api")
			Doc(api.Get("/users", omOK)).Summary("List users").Description("Group users").
				Produces(fiber.MIMEApplicationJSON).Tags("users").Deprecated()
			api.Post("/users", created)
		}, check: func(t omT, spec map[string]any) {
			require.Contains(t, omMap(t, spec, "paths", "/api/users"), "post")
			op := omMap(t, spec, "paths", "/api/users", "get")
			omHas(t, op, map[string]any{"summary": "List users", "description": "Group users"})
			omHas(t, op, map[string]any{"tags": []any{"users"}, "deprecated": true})
			require.Contains(t, omMap(t, op, "responses", "200", "content"), fiber.MIMEApplicationJSON)
		}},
		{name: "request body and responses", register: func(app *fiber.App) {
			Doc(app.Post("/users", created)).RequestBody("Create user", true, fiber.MIMEApplicationJSON).
				Response(fiber.StatusCreated, "Created", fiber.MIMEApplicationJSON)
		}, check: func(t omT, spec map[string]any) {
			op := omMap(t, spec, "paths", "/users", "post")
			omHas(t, omMap(t, op, "requestBody"), map[string]any{"description": "Create user", "required": true})
			require.Contains(t, omMap(t, op, "requestBody", "content"), fiber.MIMEApplicationJSON)
			require.Len(t, omMap(t, op, "responses"), 1, "an explicit 201 adds no default 200")
			require.Equal(t, "Created", omAt(t, op, "responses", "201", "description"))
			require.Contains(t, omMap(t, op, "responses", "201", "content"), fiber.MIMEApplicationJSON)
		}},
		{name: "request body with example", register: func(app *fiber.App) {
			Doc(app.Post("/users", created)).RequestBodyWithExample("User from route", true,
				map[string]any{"type": "object"}, "",
				map[string]any{"name": "Alice"}, nil, fiber.MIMEApplicationJSON)
		}, check: func(t omT, spec map[string]any) {
			body := omMap(t, spec, "paths", "/users", "post", "requestBody")
			omHas(t, body, map[string]any{"description": "User from route", "required": true})
			require.Contains(t, omMap(t, body, "content"), fiber.MIMEApplicationJSON)
		}},
		{name: "delete defaults to 204 with no content", register: func(app *fiber.App) {
			app.Delete("/users/:id", omOK)
		}, check: func(t omT, spec map[string]any) {
			responses := omMap(t, spec, "paths", "/users/{id}", "delete", "responses")
			require.Len(t, responses, 1)
			require.Equal(t, map[string]any{"description": "No Content"}, responses["204"])
		}},
		{name: "non-200 responses remain untouched", register: func(app *fiber.App) {
			Doc(app.Get("/users", omOK)).Response(fiber.StatusNotFound, "Not Found", fiber.MIMETextPlain)
		}, check: func(t omT, spec map[string]any) {
			responses := omMap(t, spec, "paths", "/users", "get", "responses")
			require.Len(t, responses, 1)
			require.Equal(t, "Not Found", omAt(t, responses, "404", "description"))
			require.Contains(t, omMap(t, responses, "404", "content"), fiber.MIMETextPlain)
		}},
		{name: "schema refs and examples", register: func(app *fiber.App) {
			Doc(app.Post("/users", created)).
				ParameterWithExample("q", "query", false, nil, "#/components/schemas/Query", "search query", "abc",
					map[string]any{"sample": "abc"}).
				RequestBodyWithExample("user body", true, nil, "#/components/schemas/User",
					map[string]any{"name": "john"},
					map[string]any{"sample": map[string]any{"name": "doe"}}, fiber.MIMEApplicationJSON).
				ResponseWithExample(fiber.StatusCreated, "Created", nil,
					"#/components/schemas/UserResponse", map[string]any{"id": 1},
					map[string]any{"sample": map[string]any{"id": 2}}, fiber.MIMEApplicationJSON)
		}, check: func(t omT, spec map[string]any) {
			op := omMap(t, spec, "paths", "/users", "post")
			require.Len(t, omAt(t, op, "parameters"), 1)
			param := omMap(t, op, "parameters", 0)
			require.Equal(t, "search query", param["description"])
			require.Nil(t, param["example"])
			require.Equal(t, map[string]any{"sample": map[string]any{"value": "abc"}}, param["examples"])
			require.Equal(t, "#/components/schemas/Query", omAt(t, param, "schema", "$ref"))

			body := omMap(t, op, "requestBody", "content", fiber.MIMEApplicationJSON)
			require.Equal(t, "#/components/schemas/User", omAt(t, body, "schema", "$ref"))
			require.Nil(t, body["example"])
			require.Equal(t, map[string]any{"sample": map[string]any{"value": map[string]any{"name": "doe"}}}, body["examples"])

			resp := omMap(t, op, "responses", "201", "content", fiber.MIMEApplicationJSON)
			require.Equal(t, "#/components/schemas/UserResponse", omAt(t, resp, "schema", "$ref"))
			require.Nil(t, resp["example"])
			require.Equal(t, map[string]any{"sample": map[string]any{"value": map[string]any{"id": float64(2)}}}, resp["examples"])
		}},
		{name: "path parameter", register: func(app *fiber.App) {
			Doc(app.Get("/users/:id", omOK)).Parameter("id", "path", true, map[string]any{"type": "integer"}, "identifier")
		}, check: func(t omT, spec map[string]any) {
			params := omAt(t, spec, "paths", "/users/{id}", "get", "parameters")
			require.Len(t, params, 1)
			omHas(t, omMap(t, params, 0), map[string]any{
				"name": "id", "in": "path", "description": "identifier", "schema": map[string]any{"type": "integer"},
			})
		}},
		{name: "optional path parameter declared not required stays required", register: func(app *fiber.App) {
			Doc(app.Get("/test/:id", omOK)).Parameter("id", "path", false, nil, "resource identifier")
		}, check: func(t omT, spec map[string]any) {
			params := omAt(t, spec, "paths", "/test/{id}", "get", "parameters")
			require.Len(t, params, 1)
			omHas(t, omMap(t, params, 0), map[string]any{"name": "id", "in": "path", "required": true})
		}},
		{name: "multiple path parameters", register: func(app *fiber.App) {
			app.Get("/users/:uid/books/:bid", omOK)
		}, check: func(t omT, spec map[string]any) {
			params := omAt(t, spec, "paths", "/users/{uid}/books/{bid}", "get", "parameters")
			require.Len(t, params, 2)
			omHas(t, omMap(t, params, 0), map[string]any{"name": "uid", "in": "path"})
			omHas(t, omMap(t, params, 1), map[string]any{"name": "bid", "in": "path"})
		}},
		{name: "consumes and produces", register: func(app *fiber.App) {
			Doc(app.Post("/users", created)).Consumes(fiber.MIMEApplicationJSON).Produces(fiber.MIMEApplicationXML)
			app.Post("/webhook", omOK)
		}, check: func(t omT, spec map[string]any) {
			op := omMap(t, spec, "paths", "/users", "post")
			require.Contains(t, omMap(t, op, "requestBody", "content"), fiber.MIMEApplicationJSON)
			require.Contains(t, omMap(t, op, "responses", "200", "content"), fiber.MIMEApplicationXML)
			require.NotContains(t, omMap(t, spec, "paths", "/webhook", "post"), "requestBody",
				"POST without consumes has no body")
		}},
		{name: "GET has no request body", register: func(app *fiber.App) {
			app.Get("/users", omOK)
			Doc(app.Get("/search", omOK)).RequestBody("query", false, fiber.MIMEApplicationJSON)
		}, check: func(t omT, spec map[string]any) {
			require.NotContains(t, omMap(t, spec, "paths", "/users", "get"), "requestBody")
			require.NotContains(t, omMap(t, spec, "paths", "/search", "get"), "requestBody",
				"an explicit body is suppressed")
		}},
		{name: "route security", register: func(app *fiber.App) {
			Doc(app.Get("/private", omOK)).Security(map[string][]string{"bearerAuth": {"read"}})
			Doc(app.Get("/public", omOK)).Security(map[string][]string{})
		}, check: func(t omT, spec map[string]any) {
			require.Equal(t, []any{map[string]any{"bearerAuth": []any{"read"}}}, omAt(t, spec, "paths",
				"/private", "get", "security"))
			require.Equal(t, []any{map[string]any{}}, omAt(t, spec, "paths", "/public", "get", "security"))
		}},
	})
}

func Test_OpenAPI_DocumentedRoutes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		register func(app *fiber.App)
		name     string
		present  []string
		absent   []string
	}{
		{func(*fiber.App) {}, "no routes", nil, nil},
		{func(app *fiber.App) { app.Get("/", omOK) }, "root only", []string{"/"}, nil},
		{func(app *fiber.App) {
			app.Get("/string", func(c fiber.Ctx) error { return c.SendString("a") })
			app.Get("/json", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"hello": "world"}) })
		}, "different handlers", []string{"/string", "/json"}, nil},
		{func(app *fiber.App) { app.Connect("/conn", omOK) }, "connect ignored", nil, []string{"/conn"}},
		{func(app *fiber.App) {
			app.Use(func(c fiber.Ctx) error { return c.Next() })
			app.Get("/health", omOK)
		}, "middleware routes excluded", []string{"/health"}, []string{"/"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			paths := omMap(t, omSpec(t, tc.register), "paths")
			if tc.present == nil {
				require.Empty(t, paths)
			}
			for _, path := range tc.present {
				require.Contains(t, omMap(t, paths, path), "get")
			}
			for _, path := range tc.absent {
				require.NotContains(t, paths, path)
			}
		})
	}
}

func Test_OpenAPI_Methods(t *testing.T) {
	t.Parallel()
	methods := []string{
		fiber.MethodGet, fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch,
		fiber.MethodDelete, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace,
	}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			spec := omSpec(t, func(app *fiber.App) { app.Add([]string{method}, "/method", omOK) })
			require.Contains(t, omMap(t, spec, "paths", "/method"), strings.ToLower(method))
		})
	}

	t.Run("auto HEAD excluded", func(t *testing.T) {
		t.Parallel()
		ops := omMap(t, omSpec(t, func(app *fiber.App) { app.Get("/items", omOK) }), "paths", "/items")
		require.Contains(t, ops, "get")
		require.NotContains(t, ops, "head", "auto-generated HEAD route should be excluded")
	})
}

// fetchSpec decodes the spec of an app that already mounts the middleware into the typed document.
func fetchSpec(t *testing.T, app *fiber.App) openAPISpec {
	t.Helper()
	var spec openAPISpec
	require.NoError(t, json.Unmarshal(fetchSpecBody(t, app), &spec))
	return spec
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

func Test_OpenAPI_Serving(t *testing.T) {
	t.Parallel()
	t.Run("group middleware", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		api := app.Group("/api/v2")
		api.Get("/users", omOK)
		api.Use(New())
		body := apptest.Body(t, apptest.Expect(t, app, "/api/v2/openapi.json", fiber.StatusOK))
		require.Contains(t, body, `"/api/v2/users"`)
	})

	t.Run("does not intercept similar paths", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New())
		app.Get("/reports/openapi.json", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusAccepted) })
		apptest.Expect(t, app, "/reports/openapi.json", fiber.StatusAccepted)
	})

	t.Run("only intercepts GET and HEAD", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New())
		app.Post("/openapi.json", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusAccepted) })
		resp := apptest.Expect(t, app, apptest.Req(fiber.MethodHead, "/openapi.json"), fiber.StatusOK)
		require.Equal(t, fiber.MIMEApplicationJSONCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
		apptest.Expect(t, app, apptest.Req(fiber.MethodPost, "/openapi.json"), fiber.StatusAccepted)
	})

	t.Run("root and group specs", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Title: "root"}))
		app.Group("/v1").Use(New(Config{Title: "group"}))
		require.Equal(t, "root", fetchSpec(t, app).Info.Title)
		body := apptest.Body(t, apptest.Expect(t, app, "/v1/openapi.json", fiber.StatusOK))
		var spec openAPISpec
		require.NoError(t, json.Unmarshal([]byte(body), &spec))
		require.Equal(t, "group", spec.Info.Title)
	})

	t.Run("next skips the middleware", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New(Config{Next: func(fiber.Ctx) bool { return true }}))
		apptest.Expect(t, app, "/openapi.json", fiber.StatusNotFound)
	})

	t.Run("spec reflects routes added after the first serve", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/first", omOK)
		app.Use(New())
		require.Contains(t, fetchSpec(t, app).Paths, "/first")
		app.Get("/second", omOK)
		spec := fetchSpec(t, app)
		require.Contains(t, spec.Paths, "/first")
		require.Contains(t, spec.Paths, "/second")
	})
}

func Test_OpenAPI_ConfigValues(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/users", omOK)
	cfg := Config{
		Title: "Custom API", Version: "2.1.0", Description: "My description",
		ServerURL: "https://example.com", Path: "/spec.json",
	}
	app.Use(New(cfg))

	resp := apptest.Expect(t, app, "/spec.json", fiber.StatusOK)
	var spec openAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	require.Equal(t, openAPIInfo{Title: cfg.Title, Version: cfg.Version, Description: cfg.Description}, spec.Info)
	require.Len(t, spec.Servers, 1)
	require.Equal(t, cfg.ServerURL, spec.Servers[0].URL)
}

// omPage serves the Swagger UI page at path from an app with one route and the middleware mounted with cfg.
func omPage(t *testing.T, path string, cfg ...Config) string {
	t.Helper()
	app := fiber.New()
	app.Get("/users", omOK)
	app.Use(New(cfg...))
	resp := apptest.Expect(t, app, path, fiber.StatusOK)
	require.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
	return apptest.Body(t, resp)
}

func Test_OpenAPI_SwaggerUI(t *testing.T) {
	t.Parallel()
	t.Run("default template", func(t *testing.T) {
		t.Parallel()
		body := omPage(t, "/swagger")
		require.Contains(t, body, "<title>Fiber API - Swagger UI</title>")
		require.Contains(t, body, `href="https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui.css"`)
		require.Contains(t, body, `src="https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-bundle.js"`)
		require.Contains(t, body, `src="https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-standalone-preset.js"`)
		require.Contains(t, body, "StandaloneLayout")
		require.Contains(t, body, `data-url="/openapi.json"`)
		require.Contains(t, body, `data-swagger-options='{&#34;validatorUrl&#34;:null}'`,
			"the validator badge is off by default")
		require.NotContains(t, body, "persistAuthorization")
	})

	t.Run("configurable assets and options", func(t *testing.T) {
		t.Parallel()
		body := omPage(t, "/docs", Config{
			Title: "Custom API", Path: "/spec.json", UIPath: "/docs",
			SwaggerCSSURL:              "https://cdn.example.com/swagger-ui.css",
			SwaggerBundleURL:           "https://cdn.example.com/swagger-ui-bundle.js",
			SwaggerStandalonePresetURL: "https://cdn.example.com/standalone.js",
			SwaggerOptions: map[string]any{
				"docExpansion": "list", "deepLinking": true, "persistAuthorization": true,
			},
		})
		for _, want := range []string{
			"<title>Custom API - Swagger UI</title>", `href="https://cdn.example.com/swagger-ui.css"`,
			`src="https://cdn.example.com/swagger-ui-bundle.js"`, `src="https://cdn.example.com/standalone.js"`,
			`data-url="/spec.json"`, `data-swagger-options='{&#34;`, `&#34;docExpansion&#34;:&#34;list&#34;`,
			`&#34;deepLinking&#34;:true`, `&#34;persistAuthorization&#34;:true`,
		} {
			require.Contains(t, body, want)
		}
	})

	t.Run("group path", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", omOK)
		app.Group("/api").Use(New())
		require.Contains(t, apptest.Body(t, apptest.Expect(t, app, "/api/swagger", fiber.StatusOK)),
			`data-url="/api/openapi.json"`)
	})

	t.Run("validator can be turned on", func(t *testing.T) {
		t.Parallel()
		body := omPage(t, "/swagger",
			Config{SwaggerOptions: map[string]any{"validatorUrl": "https://validator.example.com"}})
		require.Contains(t, body, `&#34;validatorUrl&#34;:&#34;https://validator.example.com&#34;`)
	})
}

func Test_OpenAPI_Version(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ configured, served string }{
		{"", "3.1.0"}, {"3.0.0", "3.0.0"}, {"3.1.0", "3.1.0"}, {"3.2.0", "3.2.0"},
	} {
		t.Run("configured "+tc.configured, func(t *testing.T) {
			t.Parallel()
			spec := omSpec(t, func(app *fiber.App) { app.Get("/test", omOK) }, Config{OpenAPIVersion: tc.configured})
			require.Equal(t, tc.served, spec["openapi"])
		})
	}
	for _, version := range []string{"2.0.0", "9.9.9"} {
		t.Run("rejects "+version, func(t *testing.T) {
			t.Parallel()
			require.Panics(t, func() { New(Config{OpenAPIVersion: version}) })
		})
	}
}

func Test_OpenAPI_ConfigDocument(t *testing.T) {
	t.Parallel()
	users := func(app *fiber.App) { app.Get("/users", omOK) }
	omRunDocCases(t, []omDocCase{
		{name: "security schemes", register: users, cfg: Config{
			SecuritySchemes: map[string]any{"bearerAuth": map[string]any{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"}},
			Security:        []map[string][]string{{"bearerAuth": {}}},
		}, check: func(t omT, spec map[string]any) {
			require.Equal(t, "http", omAt(t, spec, "components", "securitySchemes", "bearerAuth", "type"))
			require.Len(t, omAt(t, spec, "security"), 1)
		}},
		{name: "security schemes merge with components", register: users, cfg: Config{
			Components:      map[string]any{"schemas": map[string]any{"User": map[string]any{"type": "object"}}},
			SecuritySchemes: map[string]any{"apiKey": map[string]any{"type": "apiKey", "in": "header", "name": "X-API-Key"}},
		}, check: func(t omT, spec map[string]any) {
			components := omMap(t, spec, "components")
			require.Contains(t, components, "schemas")
			require.Contains(t, components, "securitySchemes")
		}},
		{name: "components", register: func(app *fiber.App) {
			Doc(app.Get("/users", omOK)).
				ResponseWithExample(fiber.StatusOK, "User list", nil, "#/components/schemas/User", nil,
					nil, fiber.MIMEApplicationJSON)
		}, cfg: Config{Components: map[string]any{"schemas": map[string]any{"User": map[string]any{
			"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}},
		}}}}, check: func(t omT, spec map[string]any) {
			require.Contains(t, omMap(t, spec, "components", "schemas"), "User")
		}},
		{name: "info metadata", register: users, cfg: Config{
			Contact:        &Contact{Name: "API Team", Email: "api@example.com", URL: "https://example.com"},
			License:        &License{Name: "MIT", URL: "https://opensource.org/licenses/MIT"},
			TermsOfService: "https://example.com/terms",
		}, check: func(t omT, spec map[string]any) {
			require.Equal(t, "https://example.com/terms", omAt(t, spec, "info", "termsOfService"))
			require.Equal(t, "API Team", omAt(t, spec, "info", "contact", "name"))
			require.Equal(t, "MIT", omAt(t, spec, "info", "license", "name"))
		}},
		{name: "multiple servers", register: users, cfg: Config{Servers: []Server{
			{URL: "https://prod.example.com", Description: "Production"},
			{URL: "https://staging.example.com", Description: "Staging"},
		}}, check: func(t omT, spec map[string]any) {
			require.Len(t, omAt(t, spec, "servers"), 2)
			omHas(t, omMap(t, spec, "servers", 0),
				map[string]any{"url": "https://prod.example.com", "description": "Production"})
		}},
		{name: "server variables", register: users, cfg: Config{Servers: []Server{{
			URL: "https://{region}.example.com", Description: "Regional",
			Variables: map[string]ServerVariable{"region": {Default: "us", Enum: []string{"us", "eu"}, Description: "Region"}},
		}}}, check: func(t omT, spec map[string]any) {
			require.Len(t, omAt(t, spec, "servers"), 1)
			require.Equal(t, "us", omAt(t, spec, "servers", 0, "variables", "region", "default"))
		}},
		{name: "top level tags and external docs", register: users, cfg: Config{
			Tags: []Tag{{
				Name: "users", Description: "User operations",
				ExternalDocs: &ExternalDocs{Description: "more", URL: "https://docs.example.com/users"},
			}},
			ExternalDocs: &ExternalDocs{Description: "Docs", URL: "https://docs.example.com"},
		}, check: func(t omT, spec map[string]any) {
			require.Len(t, omAt(t, spec, "tags"), 1)
			require.Equal(t, "users", omAt(t, spec, "tags", 0, "name"))
			require.Equal(t, "https://docs.example.com/users", omAt(t, spec, "tags", 0, "externalDocs", "url"))
			require.Equal(t, "https://docs.example.com", omAt(t, spec, "externalDocs", "url"))
		}},
	})
}

func Test_OpenAPI_PathTrailingSlash(t *testing.T) {
	t.Parallel()
	t.Run("default routing serves both paths with a slash", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", omOK)
		app.Use(New())
		for _, path := range []string{"/openapi.json/", "/swagger/"} {
			apptest.Expect(t, app, path, fiber.StatusOK)
		}
	})

	t.Run("strict routing does not serve them", func(t *testing.T) {
		t.Parallel()
		app := fiber.New(fiber.Config{StrictRouting: true})
		app.Use(New())
		app.Get("/swagger/", func(c fiber.Ctx) error { return c.SendString("user") })
		apptest.ExpectBody(t, app, "/swagger/", fiber.StatusOK, "user")
		apptest.Expect(t, app, "/openapi.json/", fiber.StatusNotFound)
		for _, path := range []string{"/openapi.json", "/swagger"} {
			apptest.Expect(t, app, path, fiber.StatusOK)
		}
	})
}

func Test_ConvertToOpenAPIPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, fiberPath, expectPath string
		params                      []string
	}{
		{"simple path no params", "/users", "/users", nil},
		{"parameter with constraint", "/users/:id<int>", "/users/{id}", []string{"id"}},
		{"parameter with regex constraint", "/posts/:slug<regex([a-z]+)>", "/posts/{slug}", []string{"slug"}},
		{"optional parameter", "/items/:id?", "/items/{id}", []string{"id"}},
		{"wildcard param", "/files/*", "/files/{wildcard}", []string{"*"}},
		{"plus param", "/docs/+", "/docs/{wildcard}", []string{"+"}},
		{
			"multiple params with constraints", "/api/:version<int>/:resource/:id<int>", "/api/{version}/{resource}/{id}",
			[]string{"version", "resource", "id"},
		},
		{"param with dot delimiter", "/files/:name.:ext", "/files/{name}.{ext}", []string{"name", "ext"}},
		{
			"param with dash delimiter", "/users/:firstName-:lastName", "/users/{firstName}-{lastName}",
			[]string{"firstName", "lastName"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			variants := buildOpenAPIPathVariants(tt.fiberPath, tt.params, false)
			require.NotEmpty(t, variants)
			require.Equal(t, tt.expectPath, variants[0].Path)
		})
	}
}

func Test_BuildOpenAPIPathVariants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, route string
		params      []string
		variants    []pathVariant // only Path and ParamNames are compared
	}{
		{
			"optional parameter emits both variants", "/items/:id?",
			[]string{"id"},
			[]pathVariant{{Path: "/items/{id}", ParamNames: []string{"id"}}, {Path: "/items"}},
		},
		{
			"wildcard uses openapi-compatible parameter name", "/files/*",
			[]string{"*1"},
			[]pathVariant{{Path: "/files/{wildcard1}", ParamNames: []string{"wildcard1"}}, {Path: "/files"}},
		},
		{
			"plus wildcard needs a segment", "/files/+",
			[]string{"+1"},
			[]pathVariant{{Path: "/files/{wildcard1}", ParamNames: []string{"wildcard1"}}},
		},
		{"escaped route path", `/foo\:bar`, nil, []pathVariant{{Path: "/foo:bar"}}},
		{
			"duplicate sanitized parameter names", "/x/:na_ve/:naïve", nil,
			[]pathVariant{{Path: "/x/{na_ve}/{na_ve_2}", ParamNames: []string{"na_ve", "na_ve_2"}}},
		},
		{"empty route", "", nil, []pathVariant{{Path: "/"}}},
		{"constraint argument with a delimiter", "/:id<range(1<2)>", nil, []pathVariant{{Path: "/{id}", ParamNames: []string{"id"}}}},
		{"plain parameter", "/:id", nil, []pathVariant{{Path: "/{id}", ParamNames: []string{"id"}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := make([]pathVariant, 0, len(tt.variants))
			for _, v := range buildOpenAPIPathVariants(tt.route, tt.params, false) {
				if len(v.ParamNames) == 0 {
					v.ParamNames = nil
				}
				got = append(got, pathVariant{Path: v.Path, ParamNames: v.ParamNames})
			}
			require.Equal(t, tt.variants, got)
		})
	}

	require.Equal(t, "wildcard1", buildOpenAPIPathVariants("/files/*", []string{"*1"}, false)[0].PathParamAliases["*1"])
	seen := map[string]struct{}{}
	for _, v := range buildOpenAPIPathVariants("/:a?/:a?", nil, false) {
		key := v.Path + "|" + strings.Join(v.ParamNames, ",")
		require.NotContains(t, seen, key, "variants must be unique")
		seen[key] = struct{}{}
	}
}

func Test_OpenAPI_ResolvedSpecPathEdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, groupPath, middlewarePath, expectedPath string }{
		{"root group with custom path", "/", "spec.json", "/spec.json"},
		{"group with wildcard", "/api/*", "/openapi.json", "/api/openapi.json"},
		{"empty group path", "", "/openapi.json", "/openapi.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			group := app.Group(tt.groupPath)
			group.Get("/test", omOK)
			group.Use(New(Config{Path: tt.middlewarePath}))
			resp := apptest.Expect(t, app, tt.expectedPath, fiber.StatusOK)
			require.Equal(t, fiber.MIMEApplicationJSONCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
		})
	}
}

func Test_OpenAPI_ShouldIncludeRequestBody(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, method, consumes string
		expectBody             bool
	}{
		{"GET without consumes has no body", fiber.MethodGet, "", false},
		{"GET never has a body even with explicit consumes", fiber.MethodGet, fiber.MIMEApplicationJSON, false},
		{"HEAD never has a body", fiber.MethodHead, fiber.MIMETextPlain, false},
		{"OPTIONS with explicit consumes opts into a body", fiber.MethodOptions, fiber.MIMETextPlain, true},
		{"TRACE never has a body (RFC 9110)", fiber.MethodTrace, fiber.MIMETextPlain, false},
		{"POST without consumes has no implicit body", fiber.MethodPost, "", false},
		{"POST with custom consumes", fiber.MethodPost, fiber.MIMEApplicationXML, true},
		{"PUT with consumes", fiber.MethodPut, fiber.MIMEApplicationJSON, true},
		{"PATCH with consumes", fiber.MethodPatch, fiber.MIMEApplicationJSON, true},
		{"DELETE with consumes", fiber.MethodDelete, fiber.MIMEApplicationJSON, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			route := app.Add([]string{tt.method}, "/test", omOK)
			if tt.consumes != "" {
				Doc(route).Consumes(tt.consumes)
			}
			app.Use(New())
			op := fetchSpec(t, app).Paths["/test"][strings.ToLower(tt.method)]
			if tt.expectBody {
				require.NotNil(t, op.RequestBody, "Expected request body for %s", tt.method)
			} else {
				require.Nil(t, op.RequestBody, "Expected no request body for %s", tt.method)
			}
		})
	}
}

func Test_OpenAPI_OperationDocuments(t *testing.T) {
	t.Parallel()
	omRunDocCases(t, []omDocCase{
		{name: "response content from route", register: func(app *fiber.App) {
			Doc(app.Get("/test", omOK)).ResponseWithExample(fiber.StatusOK, "Success", nil,
				"#/components/schemas/DefaultSchema",
				map[string]any{"default": "example"}, map[string]any{"example1": map[string]any{"value": "test"}}, fiber.MIMEApplicationJSON)
		}, check: func(t omT, spec map[string]any) {
			content := omMap(t, spec, "paths", "/test", "get", "responses", "200", "content", fiber.MIMEApplicationJSON)
			require.Equal(t, "#/components/schemas/DefaultSchema", omAt(t, content, "schema", "$ref"))
			require.NotContains(t, content, "example")
			require.Contains(t, content, "examples")
		}},
		{name: "example without examples", register: func(app *fiber.App) {
			Doc(app.Get("/test", omOK)).ParameterWithExample("q", "query", false, nil, "", "search", "abc", nil).
				ResponseWithExample(fiber.StatusOK, "OK", nil, "", map[string]any{"id": 1},
					nil, fiber.MIMEApplicationJSON)
		}, check: func(t omT, spec map[string]any) {
			op := omMap(t, spec, "paths", "/test", "get")
			require.Len(t, omAt(t, op, "parameters"), 1)
			param := omMap(t, op, "parameters", 0)
			require.Equal(t, "abc", param["example"])
			require.Nil(t, param["examples"])
			resp := omMap(t, op, "responses", "200", "content", fiber.MIMEApplicationJSON)
			require.Equal(t, map[string]any{"id": float64(1)}, resp["example"])
			require.Nil(t, resp["examples"])
		}},
		{name: "schema from combinations", register: func(app *fiber.App) {
			Doc(app.Get("/test", omOK)).
				ParameterWithExample("withSchemaRef", "query", false, map[string]any{"type": "string"},
					"#/components/schemas/MySchema", "", nil, nil).
				ParameterWithExample("withSchema", "query", false,
					map[string]any{"type": "integer", "minimum": 0}, "", "", nil, nil).
				Parameter("withDefaultType", "query", false, nil, "")
		}, check: func(t omT, spec map[string]any) {
			schemas := map[string]any{}
			for _, param := range requireSlice(t.T, omAt(t, spec, "paths", "/test", "get", "parameters")) {
				schemas[requireString(t.T, omAt(t, param, "name"))] = omAt(t, param, "schema")
			}
			require.Equal(t, map[string]any{
				"withSchemaRef":   map[string]any{"$ref": "#/components/schemas/MySchema"},
				"withSchema":      map[string]any{"type": "integer", "minimum": float64(0)},
				"withDefaultType": map[string]any{"type": "string"},
			}, schemas)
		}},
		{name: "empty media type is skipped", register: func(app *fiber.App) {
			Doc(app.Get("/test", omOK)).Response(200, "Success", fiber.MIMEApplicationJSON, "", fiber.MIMETextPlain)
		}, check: func(t omT, spec map[string]any) {
			content := omMap(t, spec, "paths", "/test", "get", "responses", "200", "content")
			require.Len(t, content, 2)
			require.Contains(t, content, fiber.MIMEApplicationJSON)
			require.Contains(t, content, fiber.MIMETextPlain)
		}},
		{name: "SchemaOf integration", register: func(app *fiber.App) {
			type User struct {
				Name string `json:"name"`
				ID   int    `json:"id"`
			}
			Doc(app.Get("/users/:id", omOK)).
				ResponseWithExample(fiber.StatusOK, "User found", SchemaOf(User{}), "", nil,
					nil, fiber.MIMEApplicationJSON)
			Doc(app.Post("/users", omOK)).
				RequestBodyWithExample("Create user", true, SchemaOf(User{}), "", nil, nil, fiber.MIMEApplicationJSON).
				ResponseWithExample(fiber.StatusCreated, "Created", SchemaOf(User{}), "", nil,
					nil, fiber.MIMEApplicationJSON)
		}, check: func(t omT, spec map[string]any) {
			for _, schema := range []map[string]any{
				omMap(t, spec, "paths", "/users/{id}", "get", "responses", "200", "content",
					fiber.MIMEApplicationJSON, "schema"),
				omMap(t, spec, "paths", "/users", "post", "requestBody", "content",
					fiber.MIMEApplicationJSON, "schema"),
			} {
				require.Equal(t, "object", schema["type"])
				require.Contains(t, omMap(t, schema, "properties"), "id")
				require.Contains(t, omMap(t, schema, "properties"), "name")
			}
		}},
		{name: "operation IDs are generated and deduplicated", register: func(app *fiber.App) {
			app.Get("/users", omOK)
			app.Get("/users/:id", omOK)
			app.Get("/a", omOK).Name("dup")
			app.Get("/b", omOK).Name("dup")
		}, check: func(t omT, spec map[string]any) {
			require.Equal(t, "getUsers", omAt(t, spec, "paths", "/users", "get", "operationId"))
			require.Equal(t, "getUsersId", omAt(t, spec, "paths", "/users/{id}", "get", "operationId"))
			ids := []any{omAt(t, spec, "paths", "/a", "get", "operationId"), omAt(t, spec,
				"paths", "/b", "get", "operationId")}
			require.ElementsMatch(t, []any{"dup", "dup_2"}, ids)
		}},
		{name: "auto summary uses the OpenAPI path", register: func(app *fiber.App) {
			app.Get("/users/:id<int>", omOK)
		}, check: func(t omT, spec map[string]any) {
			require.Equal(t, "GET /users/{id}", omAt(t, spec, "paths", "/users/{id}", "get", "summary"))
		}},
		{name: "hidden route excluded", register: func(app *fiber.App) {
			app.Get("/public", omOK)
			Doc(app.Get("/internal", omOK)).Hidden()
		}, check: func(t omT, spec map[string]any) {
			require.Contains(t, omMap(t, spec, "paths"), "/public")
			require.NotContains(t, omMap(t, spec, "paths"), "/internal")
		}},
		{name: "response header", register: func(app *fiber.App) {
			Doc(app.Get("/users", omOK)).Response(200, "OK", fiber.MIMEApplicationJSON).
				ResponseHeader(200, "X-Rate-Limit", "Requests left", map[string]any{"type": "integer"})
			Doc(app.Get("/trace", omOK)).ResponseHeader(200, "X-Trace-Id", "Trace identifier", nil)
		}, check: func(t omT, spec map[string]any) {
			omHas(t, omMap(t, spec, "paths", "/users", "get", "responses", "200", "headers", "X-Rate-Limit"),
				map[string]any{"description": "Requests left", "schema": map[string]any{"type": "integer"}})
			require.Contains(t, omMap(t, spec, "paths", "/trace", "get", "responses", "200", "headers"), "X-Trace-Id",
				"a header creates its response")
		}},
	})
}

func Test_OpenAPI_OperationFeatures(t *testing.T) {
	t.Parallel()
	omRunDocCases(t, []omDocCase{
		{name: "parameter serialization", register: func(app *fiber.App) {
			Doc(app.Get("/items", omOK)).AddParameter(RouteParameter{
				Name: "ids", In: "query", Description: "Item ids", Style: "form", Explode: new(false), Deprecated: true,
				AllowEmptyValue: true, AllowReserved: true, Schema: map[string]any{"type": "array"},
			})
		}, check: func(t omT, spec map[string]any) {
			params := omAt(t, spec, "paths", "/items", "get", "parameters")
			require.Len(t, params, 1)
			omHas(t, omMap(t, params, 0), map[string]any{
				"name": "ids", "style": "form", "explode": false,
				"deprecated": true, "allowEmptyValue": true, "allowReserved": true,
			})
		}},
		{name: "operation external docs", register: func(app *fiber.App) {
			Doc(app.Get("/x", omOK)).OperationExternalDocs("See more", "https://docs.example.com/x")
		}, check: func(t omT, spec map[string]any) {
			require.Equal(t, map[string]any{"url": "https://docs.example.com/x", "description": "See more"},
				omAt(t, spec, "paths", "/x", "get", "externalDocs"))
		}},
		{name: "per media type content", register: func(app *fiber.App) {
			Doc(app.Post("/x", omOK)).
				RequestBodyContent("payload", true, map[string]RouteMediaType{
					fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}},
					fiber.MIMEApplicationXML:  {SchemaRef: "#/components/schemas/X"},
				}).
				ResponseContent(fiber.StatusCreated, "Created",
					map[string]RouteMediaType{fiber.MIMEApplicationJSON: {
						Schema:   map[string]any{"type": "string"},
						Encoding: map[string]any{"field": map[string]any{"contentType": "text/plain"}},
					}})
		}, check: func(t omT, spec map[string]any) {
			op := omMap(t, spec, "paths", "/x", "post")
			require.Equal(t, "object", omAt(t, op, "requestBody", "content",
				fiber.MIMEApplicationJSON, "schema", "type"))
			require.Equal(t, "#/components/schemas/X", omAt(t, op, "requestBody", "content",
				fiber.MIMEApplicationXML, "schema", "$ref"))
			require.Contains(t, omMap(t, op, "responses", "201", "content", fiber.MIMEApplicationJSON), "encoding")
		}},
		{name: "response links", register: func(app *fiber.App) {
			Doc(app.Get("/x", omOK)).Response(fiber.StatusOK, "OK", fiber.MIMEApplicationJSON).
				ResponseLink(fiber.StatusOK, "next", map[string]any{"operationId": "getNext"})
		}, check: func(t omT, spec map[string]any) {
			require.Equal(t, "getNext", omAt(t, spec, "paths", "/x", "get", "responses", "200",
				"links", "next", "operationId"))
		}},
		{name: "operation extension", register: func(app *fiber.App) {
			Doc(app.Get("/x", omOK)).OperationExtension(map[string]any{
				"servers":   []any{map[string]any{"url": "https://op.example.com"}},
				"callbacks": map[string]any{"onData": map[string]any{}},
				"responses": map[string]any{"999": map[string]any{"description": "ignored"}},
			})
		}, check: func(t omT, spec map[string]any) {
			op := omMap(t, spec, "paths", "/x", "get")
			require.Len(t, omAt(t, op, "servers"), 1)
			require.Contains(t, op, "callbacks")
			require.NotContains(t, omMap(t, op, "responses"), "999")
		}},
		{name: "response schema without media type", register: func(app *fiber.App) {
			Doc(app.Get("/users", omOK)).Produces(fiber.MIMEApplicationJSON).
				ResponseWithExample(fiber.StatusOK, "OK", map[string]any{"type": "object"}, "", nil, nil)
		}, check: func(t omT, spec map[string]any) {
			require.Equal(t, "object",
				omAt(t, spec, "paths", "/users", "get", "responses", "200", "content",
					fiber.MIMEApplicationJSON, "schema", "type"))
		}},
		{name: "parameter content replaces schema", register: func(app *fiber.App) {
			Doc(app.Get("/filter", omOK)).AddParameter(RouteParameter{
				Name: "criteria", In: "query", Description: "JSON encoded filter", Required: true,
				Schema: map[string]any{"type": "string"},
				Content: map[string]RouteMediaType{
					fiber.MIMEApplicationJSON: {
						Schema: map[string]any{"type": "object"}, Example: map[string]any{"color": "red"},
					},
				},
			})
		}, check: func(t omT, spec map[string]any) {
			params := omAt(t, spec, "paths", "/filter", "get", "parameters")
			require.Len(t, params, 1)
			param := omMap(t, params, 0)
			omHas(t, param, map[string]any{"name": "criteria", "in": "query", "required": true})
			require.NotContains(t, param, "schema")
			omHas(t, omMap(t, param, "content", fiber.MIMEApplicationJSON),
				map[string]any{"schema": map[string]any{"type": "object"}, "example": map[string]any{"color": "red"}})
		}},
		{name: "querystring without schema stays valid", register: func(app *fiber.App) {
			Doc(app.Get("/q", omOK)).AddParameter(RouteParameter{Name: "filter", In: "querystring"})
		}, cfg: Config{OpenAPIVersion: "3.2.0"}, check: func(t omT, spec map[string]any) {
			param := omMap(t, spec, "paths", "/q", "get", "parameters", 0)
			require.NotContains(t, param, "schema")
			require.Equal(t, map[string]any{"type": "string"}, omAt(t, param, "content",
				querystringMediaType, "schema"))
		}},
		{name: "querystring honors an explicit content map", register: func(app *fiber.App) {
			Doc(app.Get("/q", omOK)).AddParameter(RouteParameter{
				Name: "filter", In: "querystring", Content: map[string]RouteMediaType{
					fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}},
				},
			})
		}, cfg: Config{OpenAPIVersion: "3.2.0"}, check: func(t omT, spec map[string]any) {
			content := omMap(t, spec, "paths", "/q", "get", "parameters", 0, "content")
			require.NotContains(t, content, querystringMediaType)
			require.Contains(t, content, fiber.MIMEApplicationJSON)
		}},
		{name: "operation description omitted when empty", register: func(app *fiber.App) {
			app.Get("/bare", omOK)
			Doc(app.Get("/documented", omOK)).Description("has one")
		}, check: func(t omT, spec map[string]any) {
			require.NotContains(t, omMap(t, spec, "paths", "/bare", "get"), "description")
			require.Equal(t, "has one", omAt(t, spec, "paths", "/documented", "get", "description"))
		}},
	})

	t.Run("parameter content map is not aliased", func(t *testing.T) {
		t.Parallel()
		schema := RouteMediaType{Schema: map[string]any{"type": "object"}}
		content := map[string]RouteMediaType{fiber.MIMEApplicationJSON: schema}
		app := fiber.New()
		Doc(app.Get("/x", omOK)).AddParameter(RouteParameter{Name: "p", In: "query", Content: content})
		requireMap(t, content[fiber.MIMEApplicationJSON].Schema)["type"] = "array"
		delete(content, fiber.MIMEApplicationJSON)
		app.Use(New())
		body := string(fetchSpecBody(t, app))
		require.Contains(t, body, `"type":"object"`)
		require.NotContains(t, body, `"type":"array"`)
	})

	t.Run("security with empty scopes emits an array", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		Doc(app.Get("/secure", omOK)).Security(map[string][]string{"bearerAuth": {}})
		schemes := map[string]any{"bearerAuth": map[string]any{"type": "http", "scheme": "bearer"}}
		app.Use(New(Config{SecuritySchemes: schemes}))
		body := string(fetchSpecBody(t, app))
		require.Contains(t, body, `"security":[{"bearerAuth":[]}]`)
		require.NotContains(t, body, `"bearerAuth":null`)
	})
}

func Test_OpenAPI_VersionGated(t *testing.T) {
	t.Parallel()
	x := func(app *fiber.App) { app.Get("/x", omOK) }
	query := func(app *fiber.App) {
		Doc(app.Query("/search", omOK)).RequestBody("query", true, fiber.MIMEApplicationJSON)
		Doc(app.Get("/qs", omOK)).
			AddParameter(RouteParameter{Name: "q", In: "querystring", Schema: map[string]any{"type": "string"}})
	}
	license := &License{Name: "MIT", Identifier: "MIT", URL: "https://example.com/license"}
	unchanged := *license
	gated := func(version string) Config {
		return Config{
			OpenAPIVersion: version, Summary: "Short summary", License: license,
			Self:              "https://example.com/openapi.json",
			Servers:           []Server{{URL: "https://api.example.com", Name: "prod"}},
			JSONSchemaDialect: "https://spec.openapis.org/oas/3.1/dialect/base",
			Webhooks: map[string]any{
				"newItem": map[string]any{"post": map[string]any{"responses": map[string]any{"200": map[string]any{"description": "ok"}}}},
			},
		}
	}
	omRunDocCases(t, []omDocCase{
		{name: "3.2 emits every field", register: x, cfg: gated("3.2.0"), check: func(t omT, spec map[string]any) {
			require.Equal(t, "https://example.com/openapi.json", spec["$self"])
			require.Equal(t, "prod", omAt(t, spec, "servers", 0, "name"))
			require.Equal(t, "MIT", omAt(t, spec, "info", "license", "identifier"))
			require.Equal(t, "Short summary", omAt(t, spec, "info", "summary"))
			require.Contains(t, spec, "jsonSchemaDialect")
			require.Contains(t, spec, "webhooks")
		}},
		{name: "3.1 emits its own fields", register: x, cfg: gated("3.1.0"), check: func(t omT, spec map[string]any) {
			require.Equal(t, "Short summary", omAt(t, spec, "info", "summary"))
			require.Contains(t, spec, "webhooks")
			require.Contains(t, spec, "jsonSchemaDialect")
			got := omMap(t, spec, "info", "license")
			require.Equal(t, "MIT", got["identifier"])
			require.NotContains(t, got, "url", "a license identifier excludes the URL")
			require.Equal(t, unchanged, *license, "the caller's license is not changed")
		}},
		{name: "3.0 omits the newer fields", register: x, cfg: gated("3.0.0"), check: func(t omT, spec map[string]any) {
			for _, key := range []string{"$self", "webhooks", "jsonSchemaDialect"} {
				require.NotContains(t, spec, key)
			}
			require.NotContains(t, omMap(t, spec, "info"), "summary")
			require.NotContains(t, omMap(t, spec, "servers", 0), "name")
			got := omMap(t, spec, "info", "license")
			require.NotContains(t, got, "identifier")
			require.Equal(t, "https://example.com/license", got["url"])
			require.Equal(t, unchanged, *license, "the caller's license is not changed")
		}},
		{
			name: "3.2 documents QUERY and querystring", register: query, cfg: Config{OpenAPIVersion: "3.2.0"},
			check: func(t omT, spec map[string]any) {
				require.Contains(t, omMap(t, spec, "paths", "/search", "query"), "requestBody")
				param := omMap(t, spec, "paths", "/qs", "get", "parameters", 0)
				require.Equal(t, "querystring", param["in"])
				require.NotContains(t, param, "schema")
				require.Equal(t, map[string]any{"type": "string"}, omAt(t, param, "content",
					"application/x-www-form-urlencoded", "schema"))
			},
		},
		{
			name: "3.1 omits QUERY and querystring", register: query, cfg: Config{OpenAPIVersion: "3.1.0"},
			check: func(t omT, spec map[string]any) {
				require.NotContains(t, omMap(t, spec, "paths"), "/search")
				require.NotContains(t, omMap(t, spec, "paths", "/qs", "get"), "parameters")
			},
		},
		{
			name: "3.0 omits QUERY", register: query, cfg: Config{OpenAPIVersion: "3.0.0"},
			check: func(t omT, spec map[string]any) {
				require.NotContains(t, omMap(t, spec, "paths"), "/search")
			},
		},
	})
}

func Test_OpenAPI_PathTemplates(t *testing.T) {
	t.Parallel()
	omRunDocCases(t, []omDocCase{
		{name: "optional parameter variants", register: func(app *fiber.App) {
			app.Get("/items/:id?", omOK)
		}, check: func(t omT, spec map[string]any) {
			require.Nil(t, omAt(t, spec, "paths", "/items", "get", "parameters"))
			params := omAt(t, spec, "paths", "/items/{id}", "get", "parameters")
			require.Len(t, params, 1)
			require.Equal(t, "id", omAt(t, params, 0, "name"))
		}},
		{name: "wildcard parameter name matches the template", register: func(app *fiber.App) {
			app.Get("/files/*", omOK)
		}, check: func(t omT, spec map[string]any) {
			paths := omMap(t, spec, "paths")
			require.Len(t, paths, 2)
			require.Nil(t, omAt(t, paths, "/files", "get", "parameters"))
			params := omAt(t, paths, "/files/{wildcard1}", "get", "parameters")
			require.Len(t, params, 1)
			require.Equal(t, "wildcard1", omAt(t, params, 0, "name"))
		}},
		{name: "optional variant does not overwrite", register: func(app *fiber.App) {
			Doc(app.Get("/users", omOK)).Summary("List users")
			Doc(app.Get("/users/:id?", omOK)).Summary("Get user")
		}, check: func(t omT, spec map[string]any) {
			require.Equal(t, "List users", omAt(t, spec, "paths", "/users", "get", "summary"))
			require.Equal(t, "Get user", omAt(t, spec, "paths", "/users/{id}", "get", "summary"))
		}},
		{name: "multiple optional parameters form one hierarchy", register: func(app *fiber.App) {
			app.Get("/files/:dir?/:name?", omOK)
		}, check: func(t omT, spec map[string]any) {
			paths := omMap(t, spec, "paths")
			for _, path := range []string{"/files", "/files/{dir}", "/files/{dir}/{name}"} {
				require.Contains(t, paths, path)
			}
			require.NotContains(t, paths, "/files/{name}")
		}},
		{name: "sanitized parameter collision", register: func(app *fiber.App) {
			app.Get("/x/:a#/:a$", omOK)
		}, check: func(t omT, spec map[string]any) {
			paths := omMap(t, spec, "paths")
			require.Len(t, paths, 1)
			for pathKey := range paths {
				params := requireSlice(t.T, omAt(t, paths, pathKey, "get", "parameters"))
				require.Len(t, params, 2)
				seen := map[string]struct{}{}
				for _, rawParam := range params {
					name := requireString(t.T, omAt(t, rawParam, "name"))
					require.NotContains(t, seen, name, "duplicate parameter name %q in %s", name, pathKey)
					seen[name] = struct{}{}
					require.Contains(t, pathKey, "{"+name+"}", "path template must reference %q", name)
				}
			}
		}},
		{name: "mounted parameter prefix", register: func(app *fiber.App) {
			sub := fiber.New()
			sub.Get("/users/:id", omOK)
			app.Use("/tenant/:tid", sub)
		}, check: func(t omT, spec map[string]any) {
			require.Contains(t, omMap(t, spec, "paths"), "/tenant/{tid}/users/{id}")
		}},
	})
}

func Test_OpenAPI_Mounts(t *testing.T) {
	t.Parallel()
	t.Run("case insensitive paths", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", omOK)
		app.Use(New())
		for _, target := range []string{"/OPENAPI.JSON", "/Swagger"} {
			apptest.Expect(t, app, target, fiber.StatusOK)
		}
		sensitive := fiber.New(fiber.Config{CaseSensitive: true})
		sensitive.Get("/users", omOK)
		sensitive.Use(New())
		apptest.Expect(t, sensitive, "/OPENAPI.JSON", fiber.StatusNotFound)
	})

	t.Run("exact route registration", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", omOK)
		handler := New()
		Doc(app.Get("/openapi.json", handler)).Hidden()
		Doc(app.Get("/swagger", handler)).Hidden()
		require.Contains(t, fetchSpec(t, app).Paths, "/users")
		body := apptest.Body(t, apptest.Expect(t, app, "/swagger", fiber.StatusOK))
		require.Contains(t, body, `data-url="/openapi.json"`)
	})

	t.Run("exact route mount", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", omOK)
		app.Get("/openapi.json", New())
		require.Contains(t, omMap(t, fetchJSON(t, app), "paths"), "/users")

		app2 := fiber.New()
		app2.Get("/users", omOK)
		app2.Use("/v1/openapi.json", New())
		status, _ := specBodyOf(t, app2, "/v1/openapi.json")
		require.Equal(t, fiber.StatusOK, status)
	})

	t.Run("spec reflects route removal", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/old", omOK)
		app.Use(New())
		require.Contains(t, fetchSpec(t, app).Paths, "/old")
		app.RemoveRoute("/old", fiber.MethodGet)
		app.Get("/new", omOK)
		app.RebuildTree()
		spec := fetchSpec(t, app)
		require.Contains(t, spec.Paths, "/new")
		require.NotContains(t, spec.Paths, "/old")
	})

	t.Run("metadata changes after the first serve", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(New())
		app.Get("/late", omOK)
		require.NotContains(t, omMap(t, fetchJSON(t, app), "paths", "/late", "get", "responses"), "201")
		Doc(app).Response(fiber.StatusCreated, "Created", fiber.MIMEApplicationJSON)
		require.Contains(t, omMap(t, fetchJSON(t, app), "paths", "/late", "get", "responses"), "201")
	})

	t.Run("multi prefix UI pages", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", omOK)
		app.Use([]string{"/v1", "/v2"}, New())
		for _, prefix := range []string{"/v1", "/v2"} {
			body := apptest.Body(t, apptest.Expect(t, app, prefix+"/swagger", fiber.StatusOK))
			require.Contains(t, body, `data-url="/`+prefix[1:]+`/openapi.json"`, prefix)
		}
	})

	t.Run("mounted sub app exact route", func(t *testing.T) {
		t.Parallel()
		sub := fiber.New()
		sub.Get("/users", omOK)
		handler := New()
		Doc(sub.Get("/openapi.json", handler)).Hidden()
		Doc(sub.Get("/swagger", handler)).Hidden()
		app := fiber.New()
		app.Use("/api", sub)
		body := apptest.Body(t, apptest.Expect(t, app, "/api/openapi.json", fiber.StatusOK))
		require.Contains(t, body, `"/api/users"`)
		require.Contains(t, apptest.Body(t, apptest.Expect(t, app, "/api/swagger", fiber.StatusOK)),
			`data-url="/api/openapi.json"`)
	})

	t.Run("parameterized mount prefix", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", omOK)
		app.Use("/:tenant", New())
		body := apptest.Body(t, apptest.Expect(t, app, "/acme/openapi.json", fiber.StatusOK))
		require.Contains(t, body, `"/users"`)
		apptest.Expect(t, app, "/acme/foo/openapi.json", fiber.StatusNotFound)
		require.Contains(t, apptest.Body(t, apptest.Expect(t, app, "/acme/swagger", fiber.StatusOK)),
			`data-url="/acme/openapi.json"`)
	})

	t.Run("exact route under parameterized mount", func(t *testing.T) {
		t.Parallel()
		sub := fiber.New()
		sub.Get("/users", omOK)
		Doc(sub.Get("/openapi.json", New())).Hidden()
		app := fiber.New()
		app.Use("/:tenant", sub)
		body := apptest.Body(t, apptest.Expect(t, app, "/acme/openapi.json", fiber.StatusOK))
		require.Contains(t, body, `"/{tenant}/users"`)
	})

	for _, tc := range []struct{ name, mount, target string }{
		{"regex constraint", "/:id<regex([0-9]+)>/admin", "/123/admin/openapi.json"},
		{"escaped plus", `/c\+\+`, "/c++/openapi.json"},
		{"int constraint control", "/:id<int>/admin", "/7/admin/openapi.json"},
	} {
		t.Run("mount prefix with "+tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(tc.mount, New())
			apptest.Expect(t, app, tc.target, fiber.StatusOK)
		})
	}

	t.Run("shared handler across apps", func(t *testing.T) {
		t.Parallel()
		handler := New()
		app1 := fiber.New()
		app1.Get("/only-in-app1", omOK)
		app1.Use(handler)
		app2 := fiber.New()
		app2.Get("/only-in-app2", omOK)
		app2.Use(handler)

		paths1 := omMap(t, fetchJSON(t, app1), "paths")
		require.Contains(t, paths1, "/only-in-app1")
		require.NotContains(t, paths1, "/only-in-app2")
		paths2 := omMap(t, fetchJSON(t, app2), "paths")
		require.Contains(t, paths2, "/only-in-app2")
		require.NotContains(t, paths2, "/only-in-app1")
	})

	t.Run("swagger page uses the app JSON encoder", func(t *testing.T) {
		t.Parallel()
		encoderUsed := false
		app := fiber.New(fiber.Config{JSONEncoder: func(v any) ([]byte, error) {
			encoderUsed = true
			return json.Marshal(v)
		}})
		app.Get("/x", omOK)
		app.Use(New(Config{SwaggerOptions: map[string]any{"deepLinking": true}}))
		apptest.Expect(t, app, "/swagger", fiber.StatusOK)
		require.True(t, encoderUsed)
	})
}

func Test_OpenAPI_ConcurrentDocsAndSpecRequests(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	// Registered last, so the documentation helpers below keep targeting it.
	app.Get("/users", omOK)

	documented := make(chan struct{})
	go func() {
		defer close(documented)
		for range 50 {
			Doc(app).Summary("iteration summary").
				Response(fiber.StatusOK, "OK", fiber.MIMEApplicationJSON).
				ResponseHeader(fiber.StatusOK, "X-Iter", "iteration", map[string]any{"type": "integer"}).
				Tags("concurrent")
		}
	}()

	for range 50 {
		body := apptest.Body(t, apptest.Expect(t, app, "/openapi.json", fiber.StatusOK))
		var spec map[string]any
		require.NoError(t, json.Unmarshal([]byte(body), &spec), "every response is a complete document")
		require.Contains(t, spec, "paths")
	}
	<-documented

	// Once the writer is done, the next document reflects all of it.
	_, body := specBodyOf(t, app, "/openapi.json")
	require.Contains(t, body, "iteration summary")
	require.Contains(t, body, "X-Iter")
	require.Contains(t, body, "concurrent")
}

// omWireProps encodes v with encoding/json and returns the property names it put on the wire.
func omWireProps(t *testing.T, v any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(v)
	require.NoError(t, err)
	var wire map[string]any
	require.NoError(t, json.Unmarshal(encoded, &wire))
	return wire
}

func Test_SchemaOf_Edges(t *testing.T) {
	t.Parallel()
	t.Run("omitzero is not required", func(t *testing.T) {
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
		require.Contains(t, omMap(t, schema, "properties"), "zero")
		require.Contains(t, omMap(t, schema, "properties"), "empty")
	})

	t.Run("invalid JSON tag name", func(t *testing.T) {
		t.Parallel()
		typ := reflect.StructOf([]reflect.StructField{
			{Name: "Reserved", Type: reflect.TypeFor[string](), Tag: `json:"a\\b"`},
			{Name: "Punctuated", Type: reflect.TypeFor[string](), Tag: `json:"a b"`},
		})
		value := reflect.New(typ).Elem()
		value.Field(0).SetString("x")
		value.Field(1).SetString("y")
		wire := omWireProps(t, value.Interface())
		props := omMap(t, SchemaOf(value.Interface()), "properties")
		for name := range wire {
			require.Containsf(t, props, name, "schema is missing wire property %q", name)
		}
		require.Len(t, props, len(wire))
		require.Contains(t, props, "a b")
	})

	t.Run("uintptr is documented", func(t *testing.T) {
		t.Parallel()
		type withUintptr struct {
			Pointer uintptr `json:"pointer"`
			Count   int     `json:"count"`
		}
		props := omMap(t, SchemaOf(withUintptr{}), "properties")
		for name := range omWireProps(t, withUintptr{Pointer: 42, Count: 7}) {
			require.Containsf(t, props, name, "schema is missing wire property %q", name)
		}
		require.Equal(t, map[string]any{"type": "integer"}, props["pointer"])
	})

	t.Run("unsupported slice and map elements", func(t *testing.T) {
		t.Parallel()
		type withUnsupported struct {
			M  map[string]chan int `json:"m"`
			OK string              `json:"ok"`
			Ch []chan int          `json:"ch"`
		}
		props := omMap(t, SchemaOf(withUnsupported{}), "properties")
		require.NotContains(t, props, "ch")
		require.NotContains(t, props, "m")
		require.Contains(t, props, "ok")
	})

	t.Run("self referential types", func(t *testing.T) {
		t.Parallel()
		type recMap map[string]recMap
		type recSlice []recSlice
		type selfPtr *selfPtr
		type holder struct {
			Field selfPtr `json:"field"`
		}
		require.NotPanics(t, func() {
			require.Equal(t, schemaTypeObject, SchemaOf(recMap{})[schemaKeyType])
			require.Equal(t, "array", SchemaOf(recSlice{})["type"])
		})
		var schema map[string]any
		apptest.Bounded(t, boundedTimeout, "SchemaOf on a self-referential pointer field", func() { schema = SchemaOf(holder{}) })
		require.Equal(t, schemaTypeObject, schema[schemaKeyType])
	})
}

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
		{map[string]any{"type": "string"}, "/o/:v<nonexistent>"},
		{map[string]any{"type": "integer", "minimum": 5.0}, "/p/:v<int;min(5)>"},
		{map[string]any{"type": "string", "minLength": 2.0}, "/q/:v<minlen(2)>"},
		{map[string]any{"type": "string", "pattern": "[0-9]{1,3}"}, "/r/:v<regex([0-9]{1,3})>"},
	}

	paths := omMap(t, omSpec(t, func(app *fiber.App) {
		for _, tc := range cases {
			app.Get(tc.pattern, omOK)
		}
	}), "paths")
	for _, tc := range cases {
		params := omAt(t, paths, tc.pattern[:strings.IndexByte(tc.pattern, ':')]+"{v}", "get", "parameters")
		require.Lenf(t, params, 1, "%s", tc.pattern)
		require.Equalf(t, tc.expected, omAt(t, params, 0, "schema"), "%s", tc.pattern)
	}

	require.Equal(t, parsedConstraint{name: "regex", args: []string{"a,b"}}, splitConstraintEntry("regex(a,b)"))
	require.Equal(t, parsedConstraint{name: "range", args: []string{"1", "9"}}, splitConstraintEntry("range(1,9)"))
	require.Equal(t, parsedConstraint{name: "range", args: []string{"1,9"}}, splitConstraintEntry(`range(1\,9)`))
}

func Test_OpenAPI_ConstraintSchemas(t *testing.T) {
	t.Parallel()
	omRunDocCases(t, []omDocCase{
		{name: "explicit parameter is kept", register: func(app *fiber.App) {
			Doc(app.Get("/items/:id<int>", omOK)).AddParameter(RouteParameter{
				Name: "id", In: "path", Description: "item identifier",
				Schema: map[string]any{"type": "integer", "format": "int64"},
			})
			Doc(app.Get("/codes/:code<int>", omOK)).
				Parameter("code", "path", true, map[string]any{"type": "string", "pattern": "^[0-9]+$"}, "explicit")
		}, check: func(t omT, spec map[string]any) {
			params := omAt(t, spec, "paths", "/items/{id}", "get", "parameters")
			require.Len(t, params, 1)
			omHas(t, omMap(t, params, 0), map[string]any{
				"description": "item identifier", "schema": map[string]any{"type": "integer", "format": "int64"},
			})
			require.Equal(t, map[string]any{"type": "string", "pattern": "^[0-9]+$"},
				omAt(t, spec, "paths", "/codes/{code}", "get", "parameters", 0, "schema"))
		}},
		{name: "a description keeps the constraint schema", register: func(app *fiber.App) {
			Doc(app.Get("/users/:id<int>", omOK)).
				AddParameter(RouteParameter{Name: "id", In: "path", Description: "User ID"})
		}, check: func(t omT, spec map[string]any) {
			omHas(t, omMap(t, spec, "paths", "/users/{id}", "get", "parameters", 0),
				map[string]any{"description": "User ID", "schema": map[string]any{"type": "integer"}})
		}},
		{name: "optional parameter", register: func(app *fiber.App) {
			app.Get("/pages/:page<int>?", omOK)
		}, check: func(t omT, spec map[string]any) {
			require.Equal(t, map[string]any{"type": "integer"}, omAt(t, spec, "paths", "/pages/{page}",
				"get", "parameters", 0, "schema"))
			require.NotContains(t, omMap(t, spec, "paths", "/pages", "get"), "parameters")
		}},
		{name: "escaped constraint delimiter", register: func(app *fiber.App) {
			app.Get(`/a/:v<regex(^a\>b$)>/tail`, omOK)
		}, check: func(t omT, spec map[string]any) {
			require.Len(t, omMap(t, spec, "paths"), 1)
			params := omAt(t, spec, "paths", "/a/{v}/tail", "get", "parameters")
			require.Len(t, params, 1)
			require.Equal(t, "v", omAt(t, params, 0, "name"))
		}},
		{name: "escaped comma stays in the argument", register: func(app *fiber.App) {
			app.Get("/b/:code<betweenLen(2\\,4)>", omOK)
		}, check: func(t omT, spec map[string]any) {
			require.Contains(t, omMap(t, spec, "paths"), "/b/{code}")
		}},
		{name: "canonical path adopts parameter names", register: func(app *fiber.App) {
			app.Get("/files/:id", omOK)
			app.Post("/files/:name", omOK)
		}, check: func(t omT, spec map[string]any) {
			require.NotContains(t, omMap(t, spec, "paths"), "/files/{name}")
			for _, method := range []string{"get", "post"} {
				params := omAt(t, spec, "paths", "/files/{id}", method, "parameters")
				require.Lenf(t, params, 1, "%s", method)
				require.Equalf(t, "id", omAt(t, params, 0, "name"), "%s declares the canonical name", method)
			}
		}},
		{name: "canonical path keeps the constraint schema", register: func(app *fiber.App) {
			app.Get("/items/:id", omOK)
			app.Post("/items/:code<int>", omOK)
		}, check: func(t omT, spec map[string]any) {
			omHas(t, omMap(t, spec, "paths", "/items/{id}", "post", "parameters", 0),
				map[string]any{"name": "id", "schema": map[string]any{"type": "integer"}})
		}},
		{name: "literal braces are encoded", register: func(app *fiber.App) {
			app.Get("/files/{raw}", omOK)
			app.Get("/users/:id", omOK)
		}, check: func(t omT, spec map[string]any) {
			paths := omMap(t, spec, "paths")
			require.Contains(t, paths, "/files/%7Braw%7D")
			require.NotContains(t, paths, "/files/{raw}")
			require.Contains(t, paths, "/users/{id}")
		}},
		{name: "wildcard documents the bare path", register: func(app *fiber.App) {
			app.Get("/files/*", omOK)
			app.Get("/strict/+", omOK)
		}, check: func(t omT, spec map[string]any) {
			paths := omMap(t, spec, "paths")
			require.Contains(t, paths, "/files")
			require.Contains(t, paths, "/files/{wildcard1}")
			require.NotContains(t, paths, "/strict")
			require.Contains(t, paths, "/strict/{wildcard1}")
		}},
	})
}

func Test_AdoptCanonicalParamNames(t *testing.T) {
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

	// A count mismatch leaves the variant alone.
	variant = pathVariant{ParamNames: []string{"a", "b"}, ParamConstraints: map[string]string{"a": "int"}}
	adoptCanonicalParamNames(&variant, []string{"x"})
	require.Equal(t, []string{"a", "b"}, variant.ParamNames)
	require.Equal(t, map[string]string{"a": "int"}, variant.ParamConstraints)
}

func Test_OpenAPI_ConfigContainersDetached(t *testing.T) {
	t.Parallel()
	type schema struct {
		Type string `json:"type"`
	}
	enum := []string{"eu", "us"}
	typed := map[string]schema{"User": {Type: "object"}}
	slice := []schema{{Type: "array"}}
	app := fiber.New()
	app.Get("/x", omOK)
	app.Use(New(Config{
		Servers: []Server{{
			URL: "https://{region}.example.com", Variables: map[string]ServerVariable{"region": {Default: "eu", Enum: enum}},
		}},
		Components: map[string]any{"schemas": typed, "list": slice},
	}))

	enum[0] = "mutated"
	typed["User"] = schema{Type: "mutated"}
	slice[0] = schema{Type: "mutated"}

	body := string(fetchSpecBody(t, app))
	require.NotContains(t, body, "mutated")
	for _, want := range []string{`"eu"`, `"object"`, `"array"`} {
		require.Contains(t, body, want)
	}
}

func Test_OpenAPI_DynamicMountPrefix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, mount, request string
		status               int
	}{
		{"optional segment present", "/:tenant?", "/acme/openapi.json", fiber.StatusOK},
		{"optional segment omitted", "/:tenant?", "/openapi.json", fiber.StatusOK},
		{"optional segment serves the ui too", "/:tenant?", "/acme/swagger", fiber.StatusOK},
		{"beyond the segment bound", "/:tenant?", "/a/b/openapi.json", fiber.StatusNotFound},
		{"required parameter", "/:tenant", "/acme/openapi.json", fiber.StatusOK},
		{"optional between literals", "/api/:tenant?/docs", "/api/acme/docs/openapi.json", fiber.StatusOK},
		{"greedy segment", "/files/*", "/files/a/b/openapi.json", fiber.StatusOK},
		{"greedy segment with no target", "/files/*", "/files/a/b", fiber.StatusNotFound},
		{"greedy segment falls back to the static prefix", "/files/+", "/files/openapi.json", fiber.StatusOK},
		{"static mount", "/v1", "/v1/openapi.json", fiber.StatusOK},
		{"static mount is not global", "/v1", "/openapi.json", fiber.StatusNotFound},
		{"global mount", "/", "/openapi.json", fiber.StatusOK},
		{"global mount ignores nested lookalikes", "/", "/x/openapi.json", fiber.StatusNotFound},
		{"optional trailing segment present", "/:tenant/:ver?", "/acme/v1/openapi.json", fiber.StatusOK},
		{"optional trailing segment with a slash", "/:tenant/:ver?", "/acme/v1/openapi.json/", fiber.StatusOK},
		{"optional trailing segment omitted with a slash", "/:tenant/:ver?", "/acme/openapi.json/", fiber.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Get("/hello", omOK)
			app.Use(tc.mount, New())
			apptest.Expect(t, app, tc.request, tc.status)
		})
	}
}

func Test_PrefixSegmentBounds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		pattern         string
		lowest, highest int
		dynamic         bool
	}{
		{"/", 0, 0, false},
		{"/v1", 1, 1, false},
		{"/v1/api", 2, 2, false},
		{"/:tenant", 1, 1, true},
		{"/:tenant?", 0, 1, true},
		{"/api/:tenant?/docs", 2, 3, true},
		{"/files/*", 1, -1, true},
		{"/files/+", 2, -1, true},
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

func Test_PathPrefixSegments(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path, prefix string
		n            int
		ok           bool
	}{
		{"/acme/openapi.json", "/acme", 1, true},
		{"/a/b/c", "/a/b", 2, true},
		{"/acme/openapi.json", "", 0, true},
		{"/acme/openapi.json", "", 2, false},
		{"", "", 1, false},
	} {
		prefix, ok := pathPrefixSegments(tc.path, tc.n)
		require.Equal(t, tc.ok, ok, "%s %d", tc.path, tc.n)
		require.Equal(t, tc.prefix, prefix, "%s %d", tc.path, tc.n)
	}

	require.Equal(t, 0, countPathSegments("/"))
	require.Equal(t, 1, countPathSegments("/a"))
	require.Equal(t, 3, countPathSegments("/a/b/c"))
}

func Test_uniqueOperationID(t *testing.T) {
	t.Parallel()
	used := map[string]struct{}{}
	require.Equal(t, "operation", uniqueOperationID("", used))
	require.Equal(t, "op", uniqueOperationID("op", used))
	require.Equal(t, "op_2", uniqueOperationID("op", used))
	require.Equal(t, "op_3", uniqueOperationID("op", used))
}

func Test_appendOrReplaceParameter_Guards(t *testing.T) {
	t.Parallel()
	params := []parameter{{Name: "a", In: "query"}}
	index := map[parameterKey]int{{ParamInQuery, "a"}: 0}

	require.Len(t, appendOrReplaceParameter(params, index, nil), 1)
	require.Len(t, appendOrReplaceParameter(params, index, &parameter{In: "query"}), 1)
	require.Len(t, appendOrReplaceParameter(params, index, &parameter{Name: "a"}), 1)

	replaced := appendOrReplaceParameter(params, index, &parameter{Name: "a", In: "query", Description: "x"})
	require.Len(t, replaced, 1)
	require.Equal(t, "x", replaced[0].Description)
	require.Len(t, appendOrReplaceParameter(params, index, &parameter{Name: "b", In: "query"}), 2)
}

func Test_ContentBuilders(t *testing.T) {
	t.Parallel()
	require.Equal(t, map[string]any{"$ref": "#/x"}, schemaFrom(nil, "#/x", "string", nil))
	require.Equal(t, map[string]any{"type": "string"}, schemaFrom(nil, "", "string", nil))
	require.Nil(t, schemaFrom(nil, "", "", nil))
	require.Equal(t, map[string]any{"type": "integer"}, schemaFrom(map[string]any{"type": "integer"}, "", "string", nil))

	require.Nil(t, mediaTypesToContent(nil, &RouteMediaType{}, nil))
	require.Nil(t, mediaTypesToContent([]string{""}, &RouteMediaType{}, nil))
	content := mediaTypesToContent([]string{"application/json"}, &RouteMediaType{}, nil)
	require.Contains(t, content, "application/json")
	require.Empty(t, content["application/json"])

	require.Nil(t, routeMediaTypeContent(nil, nil))
	require.Nil(t, routeMediaTypeContent(map[string]RouteMediaType{"": {Schema: map[string]any{"type": "string"}}}, nil))
	out := routeMediaTypeContent(map[string]RouteMediaType{
		"":                 {Schema: map[string]any{"type": "string"}},
		"application/json": {},
	}, nil)
	require.Len(t, out, 1)
	require.Empty(t, out["application/json"])

	require.Nil(t, buildRequestBody(nil, "", nil))
	require.Nil(t, buildRequestBody(&RouteRequestBody{Content: map[string]RouteMediaType{"": {}}}, "", nil))

	require.Equal(t, "x", inferExampleValue("x", map[string]any{}))
	require.Equal(t, "x", inferExampleValue("x", map[string]any{"type": 123}))
}

func Test_defaultResponseForMethod(t *testing.T) {
	t.Parallel()
	status, resp := defaultResponseForMethod(fiber.MethodGet, fiber.MIMEApplicationJSON)
	require.Equal(t, "200", status)
	require.Contains(t, resp.Content, fiber.MIMEApplicationJSON)

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
	serversFor := func(cfg *Config) []Server { return buildServers(cfg, newDialect(cfg.OpenAPIVersion)) }
	servers := serversFor(&Config{
		OpenAPIVersion: versionOpenAPI32, Servers: []Server{{URL: ""}, {URL: "https://x", Name: "n"}},
	})
	require.Len(t, servers, 1)
	require.Equal(t, "n", servers[0].Name)

	servers = serversFor(&Config{OpenAPIVersion: versionOpenAPI31, Servers: []Server{{URL: "https://x", Name: "n"}}})
	require.Len(t, servers, 1)
	require.Empty(t, servers[0].Name)

	servers = serversFor(&Config{OpenAPIVersion: versionOpenAPI31, Servers: []Server{{URL: ""}}, ServerURL: "https://y"})
	require.Equal(t, "https://y", servers[0].URL)
}

func Test_buildComponents_MergeSecuritySchemes(t *testing.T) {
	t.Parallel()
	require.Nil(t, buildComponents(&Config{}, nil, nil))

	components := buildComponents(&Config{
		Components:      map[string]any{"securitySchemes": map[string]any{"a": map[string]any{"type": "http"}}},
		SecuritySchemes: map[string]any{"b": map[string]any{"type": "apiKey"}},
	}, nil, nil)
	require.Contains(t, omMap(t, components, "securitySchemes"), "a")
	require.Contains(t, omMap(t, components, "securitySchemes"), "b")

	type scheme struct {
		Type   string `json:"type"`
		Scheme string `json:"scheme"`
	}
	cfg := configDefault(Config{
		Components:      map[string]any{"securitySchemes": map[string]scheme{"fromComponents": {Type: "http", Scheme: "basic"}}},
		SecuritySchemes: map[string]any{"fromConfig": map[string]any{"type": "http", "scheme": "bearer"}},
	})
	schemes := omMap(t, buildComponents(&cfg, nil, nil), "securitySchemes")
	require.Contains(t, schemes, "fromComponents", "the caller's typed schemes were dropped")
	require.Contains(t, schemes, "fromConfig")
}

func Test_RouteParameterInternals(t *testing.T) {
	t.Parallel()
	out := mergeRouteParameters(nil, map[parameterKey]int{},
		[]RouteParameter{{Name: "  "}, {Name: "q"}, {Name: "h", In: "Header"}}, nil)
	require.Len(t, out, 2)
	require.Equal(t, ParamInQuery, out[0].In)
	require.Equal(t, ParamInHeader, out[1].In)

	remapped := remapRouteParameters([]RouteParameter{{Name: "ghost", In: "path"}, {Name: "q", In: "query"}},
		map[string]string{}, []string{"id"})
	require.Len(t, remapped, 1, "an unknown path parameter is dropped")
	require.Equal(t, "q", remapped[0].Name)

	require.Equal(t, "___", sanitizeOpenAPIParamName("***", 1))
	require.Equal(t, "param1", sanitizeOpenAPIParamName("", 1))
	require.Equal(t, "id", sanitizeOpenAPIParamName("id", 1))
	require.Equal(t, wildcardParamName, sanitizeOpenAPIWildcardParamName("*", 1))
	require.Equal(t, wildcardParamName, sanitizeOpenAPIWildcardParamName("_._", 1))
	require.NotEmpty(t, sanitizeOpenAPIWildcardParamName("*5", 1))
	require.Contains(t, sanitizeOpenAPIWildcardParamName("*foo", 1), wildcardParamName)

	resolved := resolveOpenAPIPathParamName(0, "", []string{""})
	require.Empty(t, resolved.raw)
	require.Equal(t, "param1", resolved.openAPI)
	require.Equal(t, "override", resolveOpenAPIPathParamName(0, "x", []string{"override"}).raw)
	wild := resolveOpenAPIWildcardParamName(0, []string{"rest"})
	require.Equal(t, "rest", wild.raw)
	require.Contains(t, wild.openAPI, wildcardParamName)
}

func Test_OpenAPI_MarshalErrors(t *testing.T) {
	t.Parallel()
	t.Run("spec", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		Doc(app.Get("/x", omOK)).OperationExtension(map[string]any{"bad": func() {}})
		app.Use(New())
		apptest.Expect(t, app, "/openapi.json", fiber.StatusInternalServerError)
	})

	t.Run("swagger options", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/x", omOK)
		app.Use(New(Config{SwaggerOptions: map[string]any{"bad": func() {}}}))
		apptest.Expect(t, app, "/swagger", fiber.StatusInternalServerError)
	})
}

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
	require.Equal(t, map[string]any{"bearer": scheme{Type: "http"}},
		stringKeyedEntries(map[string]scheme{"bearer": {Type: "http"}}))
}

func Test_Config_CyclicOptionsDoNotOverflow(t *testing.T) {
	t.Parallel()
	opts := map[string]any{}
	opts["self"] = opts
	require.NotPanics(t, func() { _ = New(Config{SwaggerOptions: opts}) })

	type nested map[string]any
	components := map[string]any{}
	components["schemas"] = nested{"self": components}
	require.NotPanics(t, func() { _ = New(Config{Components: components}) })
}

func Test_OpenAPI_SharedHandler(t *testing.T) {
	t.Parallel()
	t.Run("resolves the case rule per app", func(t *testing.T) {
		t.Parallel()
		handler := New()
		folding := fiber.New()
		folding.Use(handler)
		strict := fiber.New(fiber.Config{CaseSensitive: true})
		strict.Use(handler)
		for range 2 {
			apptest.Expect(t, folding, "/OPENAPI.JSON", fiber.StatusOK)
			apptest.Expect(t, strict, "/OPENAPI.JSON", fiber.StatusNotFound)
		}
	})

	t.Run("beyond the cache bound", func(t *testing.T) {
		t.Parallel()
		handler := New()
		apps := make([]*fiber.App, 0, maxCachedEntries+8)
		for i := range maxCachedEntries + 8 {
			app := fiber.New()
			app.Get("/app"+strconv.Itoa(i), omOK)
			app.Use(handler)
			apps = append(apps, app)
		}
		for range 2 {
			for i, app := range apps {
				require.Contains(t, apptest.Body(t, apptest.Do(t, app, "/openapi.json")), `"/app`+strconv.Itoa(i)+`"`)
			}
		}
	})

	t.Run("cache does not keep apps alive", func(t *testing.T) {
		t.Parallel()
		handler := New()
		probe := func() weak.Pointer[fiber.App] {
			app := fiber.New()
			app.Get("/x", listUsers)
			app.Get("/openapi.json", handler)
			_ = fetchSpecBody(t, app)
			return weak.Make(app)
		}
		ref := probe()
		require.Eventually(t, func() bool {
			runtime.GC()
			return ref.Value() == nil
		}, 5*time.Second, 10*time.Millisecond, "the handler's cache kept the app reachable")
		runtime.KeepAlive(handler)
	})
}

func Test_OpenAPI_SwaggerUI_Assets(t *testing.T) {
	t.Parallel()
	// html/template escapes "+" in an attribute, and the browser decodes it back.
	attr := func(value string) string { return strings.ReplaceAll(value, "+", "&#43;") }
	hashes := []string{ConfigDefault.SwaggerCSSIntegrity, ConfigDefault.SwaggerBundleIntegrity, ConfigDefault.SwaggerStandalonePresetIntegrity}

	t.Run("defaults carry the pinned hashes", func(t *testing.T) {
		t.Parallel()
		body := omPage(t, "/swagger")
		for _, hash := range hashes {
			require.Contains(t, body, `integrity="`+attr(hash)+`"`)
		}
		require.Equal(t, 3, strings.Count(body, `crossorigin="anonymous"`))
	})

	t.Run("self hosted assets leave no external URL or hash", func(t *testing.T) {
		t.Parallel()
		body := omPage(t, "/swagger", Config{
			SwaggerCSSURL:              "/swagger-ui/swagger-ui.css",
			SwaggerBundleURL:           "/swagger-ui/swagger-ui-bundle.js",
			SwaggerStandalonePresetURL: "/swagger-ui/swagger-ui-standalone-preset.js",
		})
		require.NotContains(t, body, "://")
		require.Contains(t, body, `href="/swagger-ui/swagger-ui.css"`)
		require.Contains(t, body, `src="/swagger-ui/swagger-ui-bundle.js"`)
		require.Contains(t, body, `src="/swagger-ui/swagger-ui-standalone-preset.js"`)
		require.NotContains(t, body, "integrity=")
		// Without a hash there is no CORS requirement, so a CDN that sends no CORS headers still works.
		require.NotContains(t, body, "crossorigin=")
	})

	t.Run("only the overridden asset changes", func(t *testing.T) {
		t.Parallel()
		body := omPage(t, "/swagger", Config{SwaggerBundleURL: "/js/bundle.js"})
		require.Contains(t, body, ConfigDefault.SwaggerStandalonePresetURL,
			"an omitted asset URL falls back to the default")
		require.NotContains(t, body, attr(ConfigDefault.SwaggerBundleIntegrity))
		require.Contains(t, body, attr(ConfigDefault.SwaggerCSSIntegrity))
		require.Contains(t, body, attr(ConfigDefault.SwaggerStandalonePresetIntegrity))
	})

	t.Run("a custom URL takes the hash given for it", func(t *testing.T) {
		t.Parallel()
		body := omPage(t, "/swagger",
			Config{SwaggerBundleURL: "/js/bundle.js", SwaggerBundleIntegrity: "sha384-custom+hash/="})
		require.Contains(t, body, `integrity="`+attr("sha384-custom+hash/=")+`"`)
	})

	t.Run("default assets share one pinned version", func(t *testing.T) {
		t.Parallel()
		// The default hashes match swagger-ui-dist at the URLs' version; the URLs must share one release.
		versions := make(map[string]struct{})
		for _, url := range []string{ConfigDefault.SwaggerCSSURL, ConfigDefault.SwaggerBundleURL, ConfigDefault.SwaggerStandalonePresetURL} {
			_, rest, ok := strings.Cut(url, "swagger-ui-dist@")
			require.True(t, ok, url)
			version, _, ok := strings.Cut(rest, "/")
			require.True(t, ok, url)
			require.NotEmpty(t, version, url)
			versions[version] = struct{}{}
		}
		require.Len(t, versions, 1)
		for _, hash := range hashes {
			require.True(t, strings.HasPrefix(hash, "sha384-"), hash)
		}
	})

	t.Run("inline script is constant and hashed", func(t *testing.T) {
		t.Parallel()
		sum := sha256.Sum256([]byte(swaggerInitScript))
		want := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
		require.Equal(t, want, UIScriptCSPSource, "UIScriptCSPSource must be the hash of swaggerInitScript; update it with the script")
		// Whatever the settings, the one inline script is the same, so one hash covers every page.
		for _, path := range []string{"/openapi.json", "/very/different/spec.json?x=1&y=2"} {
			cfg := Config{Path: path, UIPath: "/docs", SwaggerOptions: map[string]any{"deepLinking": true}}
			body := omPage(t, "/docs", cfg)
			require.Equal(t, 1, strings.Count(body, "<script>"), path)
			require.Contains(t, body, "<script>"+swaggerInitScript+"</script>", path)
		}
	})

	t.Run("spec at the root", func(t *testing.T) {
		t.Parallel()
		require.Contains(t, omPage(t, "/swagger", Config{Path: "/", UIPath: "/swagger"}), `data-url="/"`)
	})

	t.Run("never a network path spec URL", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use("/*", New())
		body := apptest.Body(t, apptest.Expect(t, app, "//evil.com/swagger", fiber.StatusOK))
		require.NotContains(t, body, `data-url="//`)
		require.Contains(t, body, `data-url="/evil.com/openapi.json"`)
	})
}

func Test_OpenAPI_ConditionalRequests(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/users", omOK)
	app.Use(New())

	get := func(path string, headers ...string) *http.Response {
		opts := make([]apptest.Option, 0, len(headers)/2)
		for name, value := range apptest.Pairs(headers) {
			opts = append(opts, apptest.WithHeader(name, value))
		}
		return apptest.Do(t, app, apptest.Req(fiber.MethodGet, path, opts...))
	}

	for _, path := range []string{"/openapi.json", "/swagger"} {
		first := get(path)
		require.Equal(t, fiber.StatusOK, first.StatusCode, path)
		etag := first.Header.Get(fiber.HeaderETag)
		require.NotEmpty(t, etag, path)
		require.Equal(t, "no-cache", first.Header.Get(fiber.HeaderCacheControl), path)

		notModified := get(path, fiber.HeaderIfNoneMatch, etag)
		require.Equal(t, fiber.StatusNotModified, notModified.StatusCode, path)
		require.Empty(t, apptest.Body(t, notModified), path)

		other := get(path, fiber.HeaderIfNoneMatch, `"something-else"`)
		require.Equal(t, fiber.StatusOK, other.StatusCode, path)
		require.Equal(t, etag, other.Header.Get(fiber.HeaderETag), path)

		// An end-to-end reload always gets the body.
		require.Equal(t, fiber.StatusOK, get(path, fiber.HeaderIfNoneMatch, etag, fiber.HeaderCacheControl,
			"no-cache").StatusCode, path)
	}

	// The tag changes with the routes and only with them.
	tag := func() string { return apptest.Do(t, app, "/openapi.json").Header.Get(fiber.HeaderETag) }
	before := tag()
	require.Equal(t, before, tag(), "an unchanged document keeps its tag")
	app.Get("/more", omOK)
	require.NotEqual(t, before, tag())
}

// legacyResolveDynamicMountPrefix is the original split-by-split search, kept to check the linear version.
func legacyResolveDynamicMountPrefix(pattern, requestPath, specPath, uiPath string, equal segmentEqual) (string, bool) {
	minSegments, maxSegments, dynamic := prefixSegmentBounds(pattern)
	if !dynamic {
		return "", false
	}
	if maxSegments < 0 || maxSegments > countPathSegments(requestPath) {
		maxSegments = countPathSegments(requestPath)
	}
	for n := minSegments; n <= maxSegments; n++ {
		candidate, ok := pathPrefixSegments(requestPath, n)
		if !ok {
			break
		}
		if specPath != "" && equal(candidate+specPath, requestPath) {
			return candidate, true
		}
		if uiPath != "" && equal(candidate+uiPath, requestPath) {
			return candidate, true
		}
	}
	return "", false
}

func Test_resolveDynamicMountPrefix_MatchesSplitBySplitSearch(t *testing.T) {
	t.Parallel()
	patterns := []string{"/*", "/+", "/:a?", "/:a?/:b?", "/files/*", "/:a/*", "/api/:v?/*", "/:a/:b", "/x", "/*/y"}
	targets := [][2]string{{"/openapi.json", "/swagger"}, {"/openapi.json", ""}, {"", "/swagger"}, {"/a/openapi.json", "/swagger"}}
	segments := []string{"a", "files", "api", "v1", "openapi.json", "OPENAPI.JSON", "swagger", "Swagger"}
	rng := rand.New(rand.NewSource(1))

	for range 20000 {
		parts := make([]string, rng.Intn(6))
		for i := range parts {
			parts[i] = segments[rng.Intn(len(segments))]
		}
		request := ""
		if len(parts) > 0 {
			request = "/" + strings.Join(parts, "/")
		}
		pattern := patterns[rng.Intn(len(patterns))]
		pair := targets[rng.Intn(len(targets))]
		for _, equal := range []segmentEqual{stringsEqual, utils.EqualFold[string]} {
			wantPrefix, wantOK := legacyResolveDynamicMountPrefix(pattern, request, pair[0], pair[1], equal)
			gotPrefix, gotOK := resolveDynamicMountPrefix(pattern, request, pair[0], pair[1], equal)
			require.Equal(t, wantOK, gotOK, "pattern %q request %q targets %v", pattern, request, pair)
			require.Equal(t, wantPrefix, gotPrefix, "pattern %q request %q targets %v", pattern, request, pair)
		}
	}
}

func Test_resolveDynamicMountPrefix_LongPathStaysLinear(t *testing.T) {
	t.Parallel()
	// A quadratic search takes minutes on this path; a linear one takes microseconds.
	request := strings.Repeat("/a", 100_000) + "/openapi.json"
	apptest.Bounded(t, boundedTimeout, "resolving a mount prefix on a long path", func() {
		prefix, ok := resolveDynamicMountPrefix("/*", request, "/openapi.json", "/swagger", stringsEqual)
		require.True(t, ok)
		require.Len(t, prefix, len(request)-len("/openapi.json"))
	})
}

func Test_exampleObjects(t *testing.T) {
	t.Parallel()
	require.Nil(t, exampleObjects(nil))
	got := exampleObjects(map[string]any{
		"bare":     "xyz-789",
		"object":   map[string]any{"name": "bob"},
		"explicit": map[string]any{"summary": "s", "value": map[string]any{"name": "bob"}},
		"external": map[string]any{"externalValue": "https://example.com/e.json"},
		"ref":      map[string]any{"$ref": "#/components/examples/E"},
		"summary":  map[string]any{"summary": "only a summary"},
		"fiber":    fiber.Map{"description": "d", "value": 1},
	})
	require.Equal(t, map[string]any{
		"bare":     map[string]any{"value": "xyz-789"},
		"object":   map[string]any{"value": map[string]any{"name": "bob"}},
		"explicit": map[string]any{"summary": "s", "value": map[string]any{"name": "bob"}},
		"external": map[string]any{"externalValue": "https://example.com/e.json"},
		"ref":      map[string]any{"$ref": "#/components/examples/E"},
		"summary":  map[string]any{"value": map[string]any{"summary": "only a summary"}},
		"fiber":    map[string]any{"description": "d", "value": 1},
	}, got)
}
