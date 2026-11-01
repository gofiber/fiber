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

// hostGet requests path on host and returns the status and body.
//
//nolint:gocritic // unnamedResult: named results conflict with nonamedreturns
func hostGet(t *testing.T, app *fiber.App, host, path string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodGet, path, http.NoBody)
	req.Host = host
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

func Test_OpenAPI_ServedFromADomainRouter(t *testing.T) {
	t.Parallel()

	const host = "api.example.com"

	cases := []struct {
		name     string
		register func(app *fiber.App)
		specPath string
		uiPath   string
	}{
		{"Use", func(app *fiber.App) { app.Domain(host).Use(New()) }, "/openapi.json", "/swagger"},
		{"Use with a prefix", func(app *fiber.App) { app.Domain(host).Use("/docs", New()) }, "/docs/openapi.json", "/docs/swagger"},
		{"Get", func(app *fiber.App) { app.Domain(host).Get("/openapi.json", New()) }, "/openapi.json", ""},
		{"RouteChain", func(app *fiber.App) { app.Domain(host).RouteChain("/openapi.json").Get(New()) }, "/openapi.json", ""},
		{"a group of the domain router", func(app *fiber.App) { app.Domain(host).Group("/api").Use(New()) }, "/api/openapi.json", "/api/swagger"},
		{"the domain router of a group", func(app *fiber.App) { app.Group("/v1").Domain(host).Use(New()) }, "/v1/openapi.json", "/v1/swagger"},
		{"a host with a parameter", func(app *fiber.App) { app.Domain(":tenant.example.com").Use(New()) }, "/openapi.json", "/swagger"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Get("/public", listUsers)
			app.Domain(host).Get("/pets", listUsers)
			tc.register(app)

			requestHost := host
			if tc.name == "a host with a parameter" {
				requestHost = "acme.example.com"
			}

			status, body := hostGet(t, app, requestHost, tc.specPath)
			require.Equal(t, fiber.StatusOK, status)
			require.Contains(t, body, `"/pets"`, "the domain's routes are documented")
			require.Contains(t, body, `"//api.example.com"`, "a domain route names its host")
			require.Contains(t, body, `"/public"`, "the document describes the whole application")

			status, _ = hostGet(t, app, requestHost+":8080", tc.specPath)
			require.Equal(t, fiber.StatusOK, status, "a port in the Host header does not matter")

			if tc.uiPath != "" {
				uiStatus, page := hostGet(t, app, requestHost, tc.uiPath)
				require.Equal(t, fiber.StatusOK, uiStatus)
				require.Contains(t, page, `data-url="`+tc.specPath+`"`, "the page loads the spec from the same mount")
			}

			status, _ = hostGet(t, app, "other.example.org", tc.specPath)
			require.Equal(t, fiber.StatusNotFound, status, "another host does not get the document")
		})
	}
}

func Test_OpenAPI_DomainSubAppServesItsOwnDocument(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	sub := fiber.New()
	sub.Get("/pets", listUsers)
	sub.Use(New())
	app.Domain("api.example.com").Use("/api", sub)

	status, body := hostGet(t, app, "api.example.com", "/api/openapi.json")
	require.Equal(t, fiber.StatusOK, status)
	require.Contains(t, body, `"/api/pets"`)
	require.Contains(t, body, `"//api.example.com"`)

	status, _ = hostGet(t, app, "other.example.org", "/api/openapi.json")
	require.Equal(t, fiber.StatusNotFound, status)
}

func Test_OpenAPI_OneDocumentPerDomainRouter(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	a := app.Domain("a.example.com")
	a.Get("/x", listUsers)
	a.Use(New(Config{Title: "A"}))
	b := app.Domain("b.example.com")
	b.Get("/y", listUsers)
	b.Use(New(Config{Title: "B"}))

	for host, title := range map[string]string{"a.example.com": "A", "b.example.com": "B"} {
		status, body := hostGet(t, app, host, "/openapi.json")
		require.Equal(t, fiber.StatusOK, status, host)
		require.Contains(t, body, `"title":"`+title+`"`, host)
	}
}

func Test_OpenAPI_DomainRouterAuthIsDocumented(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	d := app.Domain("api.example.com")
	d.Get("/pets", chainKeyAuth(), listUsers)
	d.Get("/health", listUsers)
	d.Get("/secret", listUsers).Hidden()
	guarded := app.Domain("guarded.example.com")
	guarded.Use(chainKeyAuth())
	guarded.Get("/inner", listUsers)
	d.Use(New())

	status, body := hostGet(t, app, "api.example.com", "/openapi.json")
	require.Equal(t, fiber.StatusOK, status)

	var spec map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &spec))
	require.Contains(t, modelOperation(t, spec, "/pets", "get"), "security")
	require.NotContains(t, modelOperation(t, spec, "/health", "get"), "security")
	require.NotContains(t, requireMap(t, spec["paths"]), "/secret", "Hidden works on a domain route")
	require.Contains(t, modelOperation(t, spec, "/inner", "get"), "security", "keyauth on the domain router covers its routes")
}
