package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func listUsers(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }

func passThrough(c fiber.Ctx) error { return c.Next() }

type userService struct{}

func (*userService) GetUserByID(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }

func inferredPaths(t *testing.T, app *fiber.App, cfg ...Config) map[string]any {
	t.Helper()
	app.Use(New(cfg...))
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	var spec map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	return requireMap(t, spec["paths"])
}

func inferredOperation(t *testing.T, paths map[string]any, path, method string) map[string]any {
	t.Helper()
	return requireMap(t, requireMap(t, paths[path])[method])
}

func Test_OpenAPI_HandlerSummaries(t *testing.T) {
	t.Parallel()

	t.Run("named function", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", listUsers)
		op := inferredOperation(t, inferredPaths(t, app), "/users", "get")
		require.Equal(t, "List users", op["summary"])
	})

	t.Run("method value", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users/:id", (&userService{}).GetUserByID)
		op := inferredOperation(t, inferredPaths(t, app), "/users/{id}", "get")
		require.Equal(t, "Get user by ID", op["summary"])
	})

	t.Run("last handler names the operation", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", passThrough, listUsers)
		op := inferredOperation(t, inferredPaths(t, app), "/users", "get")
		require.Equal(t, "List users", op["summary"])
	})

	t.Run("closure falls back to method and path", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		op := inferredOperation(t, inferredPaths(t, app), "/users", "get")
		require.Equal(t, "GET /users", op["summary"])
	})

	t.Run("explicit summary wins", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", listUsers).Summary("Everyone")
		op := inferredOperation(t, inferredPaths(t, app), "/users", "get")
		require.Equal(t, "Everyone", op["summary"])
	})

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", listUsers)
		op := inferredOperation(t, inferredPaths(t, app, Config{DisableHandlerSummaries: true}), "/users", "get")
		require.Equal(t, "GET /users", op["summary"])
	})
}

func Test_summaryFromFuncName(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"main.listUsers":                         "List users",
		"github.com/acme/api/handlers.ListUsers": "List users",
		"main.(*Server).GetUserByID-fm":          "Get user by ID",
		"main.HTTPServer":                        "HTTP server",
		"main.get_user_v2":                       "Get user v2",
		"main.listIDs":                           "List IDs",
		"main.getURLs":                           "Get URLs",
		"main.Handler[...]":                      "Handler",
		"main.Handler[go.shape.int]":             "Handler",
		"main.x":                                 "X",
		"main.main.func1":                        "",
		"main.main.func1.2":                      "",
		"main.glob..func1":                       "",
		"main.init.deferwrap1":                   "",
		"main.run.gowrap1":                       "",
		"":                                       "",
		"main.":                                  "",
		"main.(*Server).handle-fm":               "Handle",
		"github.com/acme/api/v2/handlers.Create": "Create",
		"main._":                                 "",
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, want, summaryFromFuncName(name))
		})
	}
}

func Test_OpenAPI_GroupTags(t *testing.T) {
	t.Parallel()

	t.Run("innermost group", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Group("/api").Group("/v1").Group("/users").Get("/", listUsers)
		op := inferredOperation(t, inferredPaths(t, app), "/api/v1/users", "get")
		require.Equal(t, []any{"users"}, op["tags"])
	})

	t.Run("version segments are skipped", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Group("/api/v1").Get("/users", listUsers)
		op := inferredOperation(t, inferredPaths(t, app), "/api/v1/users", "get")
		require.Equal(t, []any{"api"}, op["tags"])
	})

	t.Run("parameter segments are skipped", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Group("/users/:id").Get("/posts", listUsers)
		op := inferredOperation(t, inferredPaths(t, app), "/users/{id}/posts", "get")
		require.Equal(t, []any{"users"}, op["tags"])
	})

	t.Run("named group uses its name", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Group("/api").Name("api.").Group("/u").Name("users.").Get("/", listUsers)
		op := inferredOperation(t, inferredPaths(t, app), "/api/u", "get")
		require.Equal(t, []any{"users"}, op["tags"])
	})

	t.Run("explicit tags win", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Group("/users").Get("/", listUsers).Tags("people")
		op := inferredOperation(t, inferredPaths(t, app), "/users", "get")
		require.Equal(t, []any{"people"}, op["tags"])
	})

	t.Run("route on the app has no tags", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", listUsers)
		op := inferredOperation(t, inferredPaths(t, app), "/users", "get")
		require.NotContains(t, op, "tags")
	})

	t.Run("only version segments yield no tag", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Group("/v2").Get("/users", listUsers)
		op := inferredOperation(t, inferredPaths(t, app), "/v2/users", "get")
		require.NotContains(t, op, "tags")
	})

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Group("/users").Get("/", listUsers)
		op := inferredOperation(t, inferredPaths(t, app, Config{DisableGroupTags: true}), "/users", "get")
		require.NotContains(t, op, "tags")
	})
}

func Test_OpenAPI_DefaultProduces(t *testing.T) {
	t.Parallel()

	responseContent := func(t *testing.T, op map[string]any, status string) (map[string]any, bool) {
		t.Helper()
		resp := requireMap(t, requireMap(t, op["responses"])[status])
		content, ok := resp["content"]
		if !ok {
			return nil, false
		}
		return requireMap(t, content), true
	}

	t.Run("undocumented route documents JSON", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", listUsers)
		content, ok := responseContent(t, inferredOperation(t, inferredPaths(t, app), "/users", "get"), "200")
		require.True(t, ok)
		require.Contains(t, content, fiber.MIMEApplicationJSON)
		require.Len(t, content, 1)
	})

	t.Run("configured default", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", listUsers)
		paths := inferredPaths(t, app, Config{DefaultProduces: fiber.MIMETextPlain})
		content, ok := responseContent(t, inferredOperation(t, paths, "/users", "get"), "200")
		require.True(t, ok)
		require.Contains(t, content, fiber.MIMETextPlain)
		require.Len(t, content, 1)
	})

	t.Run("explicit produces wins", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users", listUsers).Produces(fiber.MIMEApplicationXML)
		content, ok := responseContent(t, inferredOperation(t, inferredPaths(t, app), "/users", "get"), "200")
		require.True(t, ok)
		require.Contains(t, content, fiber.MIMEApplicationXML)
		require.Len(t, content, 1)
	})

	t.Run("declared response without a media type uses the default", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Post("/users", listUsers).Response(fiber.StatusCreated, "Created")
		content, ok := responseContent(t, inferredOperation(t, inferredPaths(t, app), "/users", "post"), "201")
		require.True(t, ok)
		require.Contains(t, content, fiber.MIMEApplicationJSON)
	})

	t.Run("statuses without a body stay without content", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Put("/users/:id", listUsers).
			Response(fiber.StatusNoContent, "Updated").
			Response(fiber.StatusNotModified, "Unchanged").
			Response(fiber.StatusContinue, "Go on")
		op := inferredOperation(t, inferredPaths(t, app), "/users/{id}", "put")
		for _, status := range []string{"204", "304", "100"} {
			_, ok := responseContent(t, op, status)
			require.False(t, ok, status)
		}
	})

	t.Run("delete default stays without content", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Delete("/users/:id", listUsers)
		_, ok := responseContent(t, inferredOperation(t, inferredPaths(t, app), "/users/{id}", "delete"), "204")
		require.False(t, ok)
	})
}

func Test_statusHasNoBody(t *testing.T) {
	t.Parallel()

	for code, want := range map[string]bool{
		"100": true, "101": true, "204": true, "205": true, "304": true,
		"200": false, "201": false, "404": false, "default": false, "1": false,
	} {
		require.Equal(t, want, statusHasNoBody(code), code)
	}
}
