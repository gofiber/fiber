package openapi

import (
	"encoding/json"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func Test_OpenAPI_EffectiveHost(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		want     string
		patterns []string
		ok       bool
	}{
		{"any host", "", nil, true},
		{"one pattern", "api.example.com", []string{"api.example.com"}, true},
		{"equal literals", "api.example.com", []string{"api.example.com", "api.example.com"}, true},
		{"a literal beats a parameter", "api.example.com", []string{":tenant.example.com", "api.example.com"}, true},
		{"either order", "api.example.com", []string{"api.example.com", ":tenant.example.com"}, true},
		{"parameters keep the innermost name", ":a.example.com", []string{":a.example.com", ":b.example.com"}, true},
		{"different literals", "", []string{"a.example.com", "b.example.com"}, false},
		{"different label counts", "", []string{"example.com", "api.example.com"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := effectiveHost(tc.patterns)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

func Test_OpenAPI_HostCovers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		cover string
		route string
		want  bool
	}{
		{"any host covers every host", "", "api.example.com", true},
		{"any host covers any host", "", "", true},
		{"a host never covers every host", "api.example.com", "", false},
		{"equal hosts", "api.example.com", "api.example.com", true},
		{"a parameter covers a literal", ":tenant.example.com", "api.example.com", true},
		{"a literal does not cover a parameter", "api.example.com", ":tenant.example.com", false},
		{"different hosts", "a.example.com", "b.example.com", false},
		{"different label counts", "example.com", "api.example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, hostCovers(tc.cover, tc.route))
		})
	}
}

func Test_OpenAPI_AnyHostRouteAfterADomainRouteWidensTheOperation(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Domain("a.example.com").Get("/x", func(c fiber.Ctx) error { return c.SendString("domain") })
	app.Get("/x", func(c fiber.Ctx) error { return c.SendString("any") })

	_, body := hostGet(t, app, "a.example.com", "/x")
	require.Equal(t, "domain", body)
	_, body = hostGet(t, app, "b.example.com", "/x")
	require.Equal(t, "any", body, "both routes answer, each on its own hosts")

	spec := modelSpec(t, app)
	require.NotContains(t, modelOperation(t, spec, "/x", "get"), "servers", "the operation no longer claims one host")
}

func Test_OpenAPI_AnyHostRouteBeforeADomainRouteShadowsIt(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error { return c.SendString("any") }).Summary("any host")
	app.Domain("a.example.com").Get("/x", func(c fiber.Ctx) error { return c.SendString("domain") }).Summary("a.example.com")

	_, body := hostGet(t, app, "a.example.com", "/x")
	require.Equal(t, "any", body, "the route for every host answers first")

	op := modelOperation(t, modelSpec(t, app), "/x", "get")
	require.Equal(t, "any host", op["summary"])
	require.NotContains(t, op, "servers")
}

func Test_OpenAPI_NestedDomainPatternsDocumentTheirIntersection(t *testing.T) {
	t.Parallel()
	sub := fiber.New()
	sub.Domain(":tenant.example.com").Get("/pets", listUsers)
	sub.Domain("other.example.org").Get("/never", listUsers)
	app := fiber.New()
	app.Domain("api.example.com").Use("/v1", sub)

	status, _ := hostGet(t, app, "api.example.com", "/v1/pets")
	require.Equal(t, fiber.StatusOK, status)
	for _, host := range []string{"api.example.com", "other.example.org"} {
		status, _ = hostGet(t, app, host, "/v1/never")
		require.Equal(t, fiber.StatusNotFound, status, "no hostname matches both patterns: %s", host)
	}

	spec := modelSpec(t, app)
	paths := requireMap(t, spec["paths"])
	require.NotContains(t, paths, "/v1/never", "a route no host reaches is not documented")
	servers, ok := modelOperation(t, spec, "/v1/pets", "get")["servers"].([]any)
	require.True(t, ok)
	require.Len(t, servers, 1)
	require.Equal(t, "//api.example.com", requireMap(t, servers[0])["url"])
	require.NotContains(t, requireMap(t, servers[0]), "variables", "the literal replaced the parameter")
}

func Test_OpenAPI_MiddlewareCoversRoutesByHost(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		guard string
		route string
		want  bool
	}{
		{"a parameter host covers a literal one", ":tenant.example.com", "api.example.com", true},
		{"a literal host covers itself", "api.example.com", "api.example.com", true},
		{"a literal host does not cover a parameter one", "api.example.com", ":other.example.com", false},
		{"another host is not covered", "a.example.com", "b.example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New())
			app.Domain(tc.guard).Use(chainKeyAuth())
			app.Domain(tc.route).Get("/x", listUsers)

			status, body := hostGet(t, app, "unrelated.example.net", "/openapi.json")
			require.Equal(t, fiber.StatusOK, status)
			var spec map[string]any
			require.NoError(t, json.Unmarshal([]byte(body), &spec))
			_, secured := modelOperation(t, spec, "/x", "get")["security"]
			require.Equal(t, tc.want, secured)
		})
	}
}
