package openapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

//nolint:gocritic // unnamedResult: nonamedreturns forbids naming these
func specBodyOf(t *testing.T, app *fiber.App, path string) (int, string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
	require.NoError(t, err)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp.StatusCode, string(b)
}

func Test_OpenAPI_ReviewRegressions(t *testing.T) {
	t.Parallel()

	t.Run("UIMountServesSpecUnderMount", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/u", func(c fiber.Ctx) error { return c.SendStatus(200) })
		app.Use("/swagger", New())
		ui, _ := specBodyOf(t, app, "/swagger")
		spec, body := specBodyOf(t, app, "/swagger/openapi.json")
		t.Logf("ui=%d spec=%d", ui, spec)
		require.Equal(t, 200, ui)
		require.Equal(t, 200, spec)
		require.Contains(t, body, "/u")
	})

	t.Run("WildcardDoesNotLeakSpec", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/secret", func(c fiber.Ctx) error { return c.SendStatus(200) })
		app.Get("/docs/*", New())
		code, _ := specBodyOf(t, app, "/docs/anything")
		require.Equal(t, 404, code)
		code, body := specBodyOf(t, app, "/docs/openapi.json")
		require.Equal(t, 200, code)
		require.Contains(t, body, "/secret")
	})

	t.Run("MidPathOptionalNotDocumented", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/api/:ver?/users/:id", func(c fiber.Ctx) error { return c.SendStatus(200) })
		app.Use(New())
		_, body := specBodyOf(t, app, "/openapi.json")
		var spec struct {
			Paths map[string]any `json:"paths"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &spec))
		t.Logf("paths=%v", spec.Paths)
		require.NotContains(t, spec.Paths, "/api/users/{id}")
		require.Contains(t, spec.Paths, "/api/{ver}/users/{id}")
	})

	t.Run("HierarchyDedupAcrossMethods", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/users/:id", func(c fiber.Ctx) error { return c.SendStatus(200) })
		app.Post("/users/:userID", func(c fiber.Ctx) error { return c.SendStatus(200) })
		app.Use(New())
		_, body := specBodyOf(t, app, "/openapi.json")
		var spec struct {
			Paths map[string]map[string]any `json:"paths"`
		}
		require.NoError(t, json.Unmarshal([]byte(body), &spec))
		t.Logf("paths=%v", spec.Paths)
		require.Len(t, spec.Paths, 1)
		require.Contains(t, spec.Paths, "/users/{id}")
		require.Contains(t, spec.Paths["/users/{id}"], "get")
		require.Contains(t, spec.Paths["/users/{id}"], "post")
	})

	t.Run("ResponseSchemaWithoutProducesKept", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(200) }).
			ResponseWithExample(200, "OK", map[string]any{"type": "object"}, "", map[string]any{"id": 1}, nil)
		app.Use(New())
		_, body := specBodyOf(t, app, "/openapi.json")
		require.Contains(t, body, "application/json")
		require.Contains(t, body, `"type":"object"`)
	})

	t.Run("BigIntSurvivesExtensions", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/big", func(c fiber.Ctx) error { return c.SendStatus(200) }).
			ResponseWithExample(200, "ok", nil, "", map[string]any{"id": int64(1234567890123456789)}, nil, fiber.MIMEApplicationJSON).
			OperationExtension(map[string]any{"x-owner": "team"})
		app.Use(New())
		_, body := specBodyOf(t, app, "/openapi.json")
		require.Contains(t, body, "1234567890123456789")
		require.Contains(t, body, "x-owner")
	})

	t.Run("CustomMethodNotEmitted", func(t *testing.T) {
		t.Parallel()
		methods := append([]string{}, fiber.DefaultMethods...)
		methods = append(methods, "PROPFIND")
		app := fiber.New(fiber.Config{RequestMethods: methods})
		app.Add([]string{"PROPFIND"}, "/res", func(c fiber.Ctx) error { return c.SendStatus(200) })
		app.Use(New())
		_, body := specBodyOf(t, app, "/openapi.json")
		require.NotContains(t, body, "propfind")
	})

	t.Run("ResponseContentKeepsDescription", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/y", func(c fiber.Ctx) error { return c.SendStatus(200) }).
			Response(201, "My custom description", fiber.MIMEApplicationJSON).
			ResponseContent(201, "", map[string]fiber.RouteMediaType{fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}}})
		app.Use(New())
		_, body := specBodyOf(t, app, "/openapi.json")
		require.Contains(t, body, "My custom description")
	})

	t.Run("HeadDefaultMirrorsGet", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/mirror", func(c fiber.Ctx) error { return c.SendStatus(200) })
		app.Head("/mirror", func(c fiber.Ctx) error { return c.SendStatus(200) })
		app.Use(New())

		_, body := specBodyOf(t, app, "/openapi.json")
		var spec openAPISpec
		require.NoError(t, json.Unmarshal([]byte(body), &spec))

		ops := spec.Paths["/mirror"]
		require.Contains(t, ops, "head")
		require.Equal(t, ops["get"].Responses, ops["head"].Responses)
		require.Contains(t, ops["head"].Responses, "200")
		require.NotContains(t, ops["head"].Responses, "204")
	})
}
