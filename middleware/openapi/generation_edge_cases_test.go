package openapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func Test_OpenAPI_GenerationEdgeCases(t *testing.T) {
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

func Test_OpenAPI_DomainRoutesCarryTheirHost(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Domain("api.example.com").Get("/pets", listUsers)
	app.Domain(":tenant.example.com").Get("/orgs", listUsers)
	app.Get("/open", listUsers)
	spec := modelSpec(t, app)

	servers := func(path string) []any {
		op := modelOperation(t, spec, path, "get")
		list, _ := op["servers"].([]any) //nolint:errcheck // absent means none
		return list
	}

	pets := servers("/pets")
	require.Len(t, pets, 1)
	require.Equal(t, "//api.example.com", requireMap(t, pets[0])["url"])
	require.NotContains(t, requireMap(t, pets[0]), "variables")

	orgs := servers("/orgs")
	require.Len(t, orgs, 1)
	org := requireMap(t, orgs[0])
	require.Equal(t, "//{tenant}.example.com", org["url"])
	require.Equal(t, "tenant", requireMap(t, requireMap(t, org["variables"])["tenant"])["default"])

	require.Empty(t, servers("/open"))
}

func Test_RouteLexers_Agree(t *testing.T) {
	t.Parallel()

	// segments.go and paths.go lex the route grammar separately; both must find the same parameter count.
	for _, pattern := range []string{
		"/users",
		"/users/:id",
		"/users/:id<int>/posts/:post?",
		"/files/*",
		"/files/+",
		"/a/:x-:y/b",
		`/lit\:eral/:id`,
		`/esc\*aped/*`,
		"/:lang<regex(en|de)>?/docs",
		"/:a?/:b?/:c",
		"/v:ver/api",
		"/p/:id<range(1,10)>",
	} {
		want := 0
		for seg := range strings.SplitSeq(strings.TrimPrefix(pattern, "/"), "/") {
			_, tokens := classifySegment(seg)
			want += strings.Count(tokens, ":") + strings.Count(tokens, "*") + strings.Count(tokens, "+")
		}

		got := 0
		for _, variant := range buildOpenAPIPathVariants(pattern, nil, false) {
			got = max(got, len(variant.ParamNames))
		}
		require.Equal(t, want, got, pattern)
	}
}

func Test_OpenAPI_DomainMountedSubAppCarriesTheHost(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	sub := fiber.New()
	sub.Get("/s", listUsers)
	app.Domain("api.example.com").Use("/dm", sub)
	app.Get("/open", listUsers)
	spec := modelSpec(t, app)

	servers, _ := modelOperation(t, spec, "/dm/s", "get")["servers"].([]any) //nolint:errcheck // absent means none
	require.Len(t, servers, 1)
	require.Equal(t, "//api.example.com", requireMap(t, servers[0])["url"])
	require.NotContains(t, modelOperation(t, spec, "/open", "get"), "servers")
}

func Test_OpenAPI_OperationIDFromName(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]string{
		"listUsers":      "listUsers",
		"users.list":     "users.list",
		"users_list-v2":  "users_list-v2",
		"users list":     "users_list",
		"  list  users ": "list_users",
		"GET /users/:id": "GET_users_id",
		"üñï":            "",
		"":               "",
	} {
		require.Equal(t, want, operationIDFromName(name), name)
	}

	app := fiber.New()
	app.Get("/users", listUsers).Name("list all users")
	app.Get("/users/:id", listUsers).Name("")
	spec := modelSpec(t, app)
	require.Equal(t, "list_all_users", modelOperation(t, spec, "/users", "get")["operationId"])
}

func Test_OpenAPI_StrictRoutingKeepsTrailingSlash(t *testing.T) {
	t.Parallel()

	strict := fiber.New(fiber.Config{StrictRouting: true})
	strict.Get("/a", listUsers)
	strict.Get("/a/", listUsers)
	paths := requireMap(t, modelSpec(t, strict)["paths"])
	require.Contains(t, paths, "/a")
	require.Contains(t, paths, "/a/")

	loose := fiber.New()
	loose.Get("/a/", listUsers)
	require.Contains(t, requireMap(t, modelSpec(t, loose)["paths"]), "/a")
}

func Test_OpenAPI_DocumentLevelEmptySecurity(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/x", listUsers)

	spec := modelSpec(t, app, Config{Security: []map[string][]string{}})
	require.Equal(t, []any{}, spec["security"])

	// Each spec needs its own app: the first middleware registered answers.
	plain := fiber.New()
	plain.Get("/x", listUsers)
	require.NotContains(t, modelSpec(t, plain), "security")
}
