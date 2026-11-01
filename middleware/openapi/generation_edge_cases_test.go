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
		app.Get("/u", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Use("/swagger", New())
		ui, _ := specBodyOf(t, app, "/swagger")
		spec, body := specBodyOf(t, app, "/swagger/openapi.json")
		t.Logf("ui=%d spec=%d", ui, spec)
		require.Equal(t, fiber.StatusOK, ui)
		require.Equal(t, fiber.StatusOK, spec)
		require.Contains(t, body, "/u")
	})

	t.Run("WildcardDoesNotLeakSpec", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/secret", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Get("/docs/*", New())
		code, _ := specBodyOf(t, app, "/docs/anything")
		require.Equal(t, fiber.StatusNotFound, code)
		code, body := specBodyOf(t, app, "/docs/openapi.json")
		require.Equal(t, fiber.StatusOK, code)
		require.Contains(t, body, "/secret")
	})

	t.Run("MidPathOptionalNotDocumented", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/api/:ver?/users/:id", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
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
		app.Get("/users/:id", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Post("/users/:userID", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
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
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
			ResponseWithExample(200, "OK", map[string]any{"type": "object"}, "", map[string]any{"id": 1}, nil)
		app.Use(New())
		_, body := specBodyOf(t, app, "/openapi.json")
		require.Contains(t, body, "application/json")
		require.Contains(t, body, `"type":"object"`)
	})

	t.Run("BigIntSurvivesExtensions", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/big", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
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
		app.Add([]string{"PROPFIND"}, "/res", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Use(New())
		_, body := specBodyOf(t, app, "/openapi.json")
		require.NotContains(t, body, "propfind")
	})

	t.Run("ResponseContentKeepsDescription", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/y", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
			Response(201, "My custom description", fiber.MIMEApplicationJSON).
			ResponseContent(201, "", map[string]fiber.RouteMediaType{fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}}})
		app.Use(New())
		_, body := specBodyOf(t, app, "/openapi.json")
		require.Contains(t, body, "My custom description")
	})

	t.Run("HeadDefaultMirrorsGet", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/mirror", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Head("/mirror", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
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

func Test_OpenAPI_StrictRoutingDropsUnreachableVariants(t *testing.T) {
	t.Parallel()

	for pattern, want := range map[string][]string{
		"/a/:id?/":    {"/a/{id}/"},
		"/:x?/:y?/":   {"/{x}/{y}/"},
		"/a/*/":       {"/a/{wildcard}/"},
		"/a/:id?":     {"/a", "/a/{id}"},
		"/plain/":     {"/plain/"},
		"/plain":      {"/plain"},
		"/a/:id?/end": {"/a/{id}/end"},
	} {
		var got []string
		for _, variant := range buildOpenAPIPathVariants(pattern, nil, true) {
			got = append(got, variant.Path)
		}
		require.ElementsMatch(t, want, got, pattern)
	}
}

func Test_OpenAPI_VersionPatchReleases(t *testing.T) {
	t.Parallel()

	for version, want := range map[string]string{
		"3.0.3": "3.0.3", "3.1.1": "3.1.1", "3.2.0": "3.2.0", "3.1.0": "3.1.0",
		"": "3.1.0",
	} {
		app := fiber.New()
		app.Get("/x", listUsers)
		require.Equal(t, want, modelSpec(t, app, Config{OpenAPIVersion: version})["openapi"], version)
	}

	for _, version := range []string{"3.2", "3.3.0", "2.0.0", "3.1.x", "garbage", " 3.1.0"} {
		require.PanicsWithValue(t, `openapi: OpenAPIVersion must be a 3.0.x, 3.1.x or 3.2.x release, got "`+version+`"`, func() {
			New(Config{OpenAPIVersion: version})
		}, version)
	}
}

func Test_OpenAPI_ReservedHeaderParametersAreIgnored(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/x", listUsers).
		Parameter("Accept", fiber.ParamInHeader, false, map[string]any{"type": "string"}, "ignored").
		Parameter("content-type", fiber.ParamInHeader, false, map[string]any{"type": "string"}, "ignored").
		Parameter("Authorization", fiber.ParamInHeader, false, map[string]any{"type": "string"}, "ignored").
		Parameter("X-Trace", fiber.ParamInHeader, false, map[string]any{"type": "string"}, "kept").
		Parameter("Accept", fiber.ParamInQuery, false, map[string]any{"type": "string"}, "a query parameter named Accept stays")
	params := chainParams(t, modelOperation(t, modelSpec(t, app), "/x", "get"))
	require.Contains(t, params, "header:X-Trace")
	require.Contains(t, params, "query:Accept")
	for _, name := range []string{"header:Accept", "header:content-type", "header:Authorization"} {
		require.NotContains(t, params, name)
	}
}

func Test_OpenAPI_QuerystringExcludesQueryParameters(t *testing.T) {
	t.Parallel()

	newApp := func() *fiber.App {
		app := fiber.New()
		app.Get("/x", listUsers).
			Parameter("page", fiber.ParamInQuery, false, map[string]any{"type": "integer"}, "p").
			AddParameter(fiber.RouteParameter{Name: "qs", In: fiber.ParamInQuerystring, Content: map[string]fiber.RouteMediaType{"application/x-www-form-urlencoded": {Schema: map[string]any{"type": "object"}}}}).
			Parameter("X-Trace", fiber.ParamInHeader, false, map[string]any{"type": "string"}, "h")
		return app
	}

	params := chainParams(t, modelOperation(t, modelSpec(t, newApp(), Config{OpenAPIVersion: "3.2.0"}), "/x", "get"))
	require.Contains(t, params, "querystring:qs")
	require.Contains(t, params, "header:X-Trace")
	require.NotContains(t, params, "query:page")

	// Before 3.2 there is no querystring location, so the query parameter stays.
	older := chainParams(t, modelOperation(t, modelSpec(t, newApp(), Config{OpenAPIVersion: "3.1.0"}), "/x", "get"))
	require.Contains(t, older, "query:page")
	require.NotContains(t, older, "querystring:qs")
}

func Test_OpenAPI_SpecDoesNotAliasRouteSecurity(t *testing.T) {
	t.Parallel()

	requirements := []map[string][]string{{"oauth": {"read"}}}
	app := fiber.New()
	app.Get("/x", listUsers).Security(requirements...)
	// Changing what the caller passed afterwards must not change the route or the document.
	requirements[0]["oauth"][0] = "changed"
	requirements[0]["other"] = []string{"x"}

	scheme := map[string]any{"type": "http", "scheme": "bearer"}
	security, ok := modelOperation(t, modelSpec(t, app, Config{SecuritySchemes: map[string]any{"oauth": scheme, "other": scheme}}), "/x", "get")["security"].([]any)
	require.True(t, ok)
	require.Equal(t, []any{map[string]any{"oauth": []any{"read"}}}, security)
}

func Test_OpenAPI_MountedRoutesFollowTheParentsStrictRouting(t *testing.T) {
	t.Parallel()

	// A mounted route is served under the parent's routing settings, so the parent's
	// StrictRouting decides whether the trailing slash is part of the published path.
	for _, tt := range []struct {
		want                    string
		parentStrict, subStrict bool
	}{
		{parentStrict: false, subStrict: true, want: "/m/x"},
		{parentStrict: true, subStrict: false, want: "/m/x/"},
		{parentStrict: true, subStrict: true, want: "/m/x/"},
		{parentStrict: false, subStrict: false, want: "/m/x"},
	} {
		parent := fiber.New(fiber.Config{StrictRouting: tt.parentStrict})
		sub := fiber.New(fiber.Config{StrictRouting: tt.subStrict})
		sub.Get("/x/", listUsers)
		parent.Use("/m", sub)
		require.Contains(t, requireMap(t, modelSpec(t, parent)["paths"]), tt.want, "%+v", tt)
	}
}

func Test_OpenAPI_VersionHelpers(t *testing.T) {
	t.Parallel()

	for version, want := range map[string]int{"3.0.0": 0, "3.0.3": 0, "3.1.10": 1, "3.1.01": 1, "3.2.0": 2} {
		minor, ok := parseVersion(version)
		require.True(t, ok, version)
		require.Equal(t, want, minor, version)
	}
	for _, version := range []string{"", "3", "3.1", "3.1.", "03.1.0", " 3.1.0", "3.1.0-rc1", "3.3.0", "4.0.0", "3.10.0"} {
		_, ok := parseVersion(version)
		require.False(t, ok, version)
	}
}

func Test_Dialect_Capabilities(t *testing.T) {
	t.Parallel()

	cases := []struct {
		version                                                 string
		query, querystring, serverName, self, jsonSchemaDialect bool
	}{
		{"3.0.0", false, false, false, false, false},
		{"3.0.3", false, false, false, false, false},
		{"3.1.0", false, false, false, false, true},
		{"3.1.1", false, false, false, false, true},
		{"3.2.0", true, true, true, true, true},
		{"3.2.4", true, true, true, true, true},
		{"garbage", false, false, false, false, false},
	}
	for _, tc := range cases {
		d := newDialect(tc.version)
		require.Equal(t, tc.query, d.hasQueryMethod(), tc.version)
		require.Equal(t, tc.querystring, d.hasQuerystringLocation(), tc.version)
		require.Equal(t, tc.serverName, d.hasServerName(), tc.version)
		require.Equal(t, tc.self, d.hasSelf(), tc.version)
		require.Equal(t, tc.jsonSchemaDialect, d.hasJSONSchemaDialect(), tc.version)
	}
}

func Test_Dialect_License(t *testing.T) {
	t.Parallel()

	both := License{Name: "MIT", URL: "https://x", Identifier: "MIT"}
	require.Equal(t, License{Name: "MIT", Identifier: "MIT"}, newDialect("3.1.0").license(both))
	require.Equal(t, License{Name: "MIT", URL: "https://x"}, newDialect("3.0.3").license(both))
}

func Test_Dialect_Nullable(t *testing.T) {
	t.Parallel()

	typed := func() map[string]any { return map[string]any{"type": "string", "enum": []any{"a"}} }
	ref := func() map[string]any { return map[string]any{"$ref": "#/components/schemas/A"} }

	old := newDialect("3.0.3")
	got := old.nullable(typed())
	require.Equal(t, true, got["nullable"])
	require.Equal(t, []any{"a", nil}, got["enum"])
	nullOnly := map[string]any{"type": "object", "nullable": true, "enum": []any{nil}}
	require.Equal(t, []any{map[string]any{"$ref": "#/components/schemas/A"}, nullOnly}, old.nullable(ref())["anyOf"])

	modern := newDialect("3.1.0")
	got = modern.nullable(typed())
	require.Equal(t, []any{"string", "null"}, got["type"])
	require.NotContains(t, got, "nullable")
	require.Equal(t, []any{
		map[string]any{"$ref": "#/components/schemas/A"},
		map[string]any{"type": "null"},
	}, modern.nullable(ref())["anyOf"])
}

func Test_OpenAPI_ConfigRejectsInvalidServersAndPaths(t *testing.T) {
	t.Parallel()

	for name, cfg := range map[string]Config{
		"variable without default": {Servers: []Server{{URL: "https://{r}.x", Variables: map[string]ServerVariable{"r": {}}}}},
		"default outside enum":     {Servers: []Server{{URL: "https://{r}.x", Variables: map[string]ServerVariable{"r": {Default: "eu", Enum: []string{"us"}}}}}},
		"undeclared placeholder":   {Servers: []Server{{URL: "https://{region}.x"}}},
		"path equals ui path":      {Path: "/docs", UIPath: "/docs/"},
		"path equals ui path case": {Path: "/Docs", UIPath: "/docs"},
	} {
		require.Panics(t, func() { _ = configDefault(cfg) }, name)
	}

	require.NotPanics(t, func() {
		_ = configDefault(Config{
			Servers: []Server{
				{URL: ""},
				{URL: "https://{r}.x/{v}", Variables: map[string]ServerVariable{"r": {Default: "eu", Enum: []string{"eu", "us"}}, "v": {Default: "v1"}}},
				{URL: "https://plain.example.com"},
			},
			Path: "/a.json", UIPath: "/a",
		})
	})
}

func Test_OpenAPI_UsedTagsAreDeclaredAtTheTop(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Group("/users").Get("/", listUsers)
	app.Get("/items", listUsers).Tags("zeta", "described")
	app.Get("/plain-root", listUsers)

	spec := modelSpec(t, app, Config{Tags: []Tag{{Name: "described", Description: "kept as written"}}})
	tags, ok := spec["tags"].([]any)
	require.True(t, ok)

	var names []string
	for _, raw := range tags {
		tag := requireMap(t, raw)
		name, _ := tag["name"].(string) //nolint:errcheck // checked by the assertions below
		names = append(names, name)
		if name == "described" {
			require.Equal(t, "kept as written", tag["description"])
		}
	}
	require.Equal(t, []string{"described", "users", "zeta"}, names, "declared tags first, then the rest sorted")

	none := fiber.New()
	none.Get("/x", listUsers)
	require.NotContains(t, modelSpec(t, none, Config{DisableGroupTags: true}), "tags")
}

func Test_Dialect_Nullable_Composition(t *testing.T) {
	t.Parallel()

	composed := func() map[string]any {
		return map[string]any{"oneOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "integer"}}}
	}

	got := newDialect("3.1.0").nullable(composed())
	require.Equal(t, []any{composed(), map[string]any{"type": "null"}}, got["anyOf"])

	// 3.0 defines nullable only next to a type, so the composition gets a branch that holds only null.
	oldGot := newDialect("3.0.3").nullable(composed())
	require.Equal(t, []any{composed(), map[string]any{"type": "object", "nullable": true, "enum": []any{nil}}}, oldGot["anyOf"])

	// A schema with no type, $ref or composition already accepts anything.
	require.Empty(t, newDialect("3.1.0").nullable(map[string]any{}))
}

func Test_Dialect_Nullable_LeavesInputAlone(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"3.0.3", "3.1.0"} {
		in := map[string]any{"type": "string", "enum": []any{"a"}}
		_ = newDialect(version).nullable(in)
		require.Equal(t, map[string]any{"type": "string", "enum": []any{"a"}}, in, version)
	}
}

func Test_OpenAPI_LicenseIsNotShared(t *testing.T) {
	t.Parallel()

	license := &License{Name: "MIT", URL: "https://x"}
	app := fiber.New()
	app.Get("/x", listUsers)
	app.Use(New(Config{License: license}))
	_ = fetchSpec(t, app)
	require.Equal(t, License{Name: "MIT", URL: "https://x"}, *license)
}

func Test_buildOpenAPIPathVariants_EscapedBracesAreEncoded(t *testing.T) {
	t.Parallel()

	variants := buildOpenAPIPathVariants(`/e/a\{b\}`, nil, false)
	require.Len(t, variants, 1)
	require.Equal(t, "/e/a%7Bb%7D", variants[0].Path)
}

func Test_PathLexer_AgreesWithTheRouter(t *testing.T) {
	t.Parallel()

	for _, pattern := range []string{
		"/users/:id",
		"/users/:id<int>/posts/:post",
		"/x/:date<regex([^>]+)>",
		"/api/v1/:param<regex(^[^>]+$)>",
		"/shop/product/color::color/size::size",
		"/a:b/c",
		"/files/*",
		"/files/+/x",
		"/a/:x-:y/b",
		`/lit\:eral/:id`,
	} {
		app := fiber.New()
		app.Get(pattern, listUsers)
		var params []string
		for _, route := range app.GetRoutes(true) {
			if route.Path == pattern || route.Method == fiber.MethodGet {
				params = route.Params
				break
			}
		}

		var got []string
		for _, variant := range buildOpenAPIPathVariants(pattern, params, false) {
			if len(variant.ParamNames) > len(got) {
				got = variant.ParamNames
			}
		}
		require.Len(t, got, len(params), "%s: router params %v, lexer params %v", pattern, params, got)
	}
}

func Test_OpenAPI_PathsMatchServedRoutes(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/x/:date<regex([^>]+)>", listUsers)
	app.Get("/shop/product/color::color/size::size", listUsers)
	spec := modelSpec(t, app)

	paths := requireMap(t, spec["paths"])
	require.Contains(t, paths, "/x/{date}")
	require.NotContains(t, paths, "/x/{date}]{wildcard})>")
	require.Contains(t, paths, "/shop/product/color:{color}/size:{size}")
}

func Test_Dialect_Nullable_ListTypesAndTypedEnums(t *testing.T) {
	t.Parallel()

	modern := newDialect("3.1.0")
	got := modern.nullable(map[string]any{"type": []string{"string", "integer"}, "enum": []string{"a"}})
	require.Equal(t, []any{"string", "integer", "null"}, got["type"])
	require.Equal(t, []any{"a", nil}, got["enum"])

	got = modern.nullable(map[string]any{"type": []any{"string", "null"}})
	require.Equal(t, []any{"string", "null"}, got["type"], "null is not added twice")

	in := map[string]any{"type": []any{"string"}, "enum": []int{1}}
	_ = modern.nullable(in)
	require.Equal(t, []any{"string"}, in["type"], "the input is left alone")
	require.Equal(t, []int{1}, in["enum"])

	old := newDialect("3.0.3").nullable(map[string]any{"type": "integer", "enum": []int{1, 2}})
	require.Equal(t, []any{1, 2, nil}, old["enum"])
	require.Equal(t, true, old["nullable"])
}

func Test_OpenAPI_GroupTagSkipsVersionMarkersInNamesToo(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	api, ok := app.Group("/api").Name("api.").(*fiber.Group)
	require.True(t, ok)
	v1, ok := api.Group("/v1").Name("v1.").(*fiber.Group)
	require.True(t, ok)
	v1.Get("/users", listUsers)

	plain := app.Group("/api")
	plain.Group("/v1").Get("/items", listUsers)

	spec := modelSpec(t, app)
	require.Equal(t, []any{"api"}, modelOperation(t, spec, "/api/v1/users", "get")["tags"])
	require.Equal(t, []any{"api"}, modelOperation(t, spec, "/api/v1/items", "get")["tags"])
}

func Test_OpenAPI_SamePathOnSeveralDomainsListsEveryHost(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Domain("a.example.com").Get("/pets", listUsers).Summary("A pets")
	app.Domain("b.example.com").Get("/pets", listUsers).Summary("B pets")
	app.Domain("b.example.com").Get("/pets", listUsers) // a repeated host adds nothing
	app.Domain("a.example.com").Get("/pets/:id", listUsers)
	app.Domain(":tenant.example.org").Get("/pets/:name", listUsers) // same template up to the parameter name
	app.Get("/shared", listUsers)
	app.Domain("a.example.com").Get("/shared", listUsers) // the route for every host already covers it
	app.Domain("c.example.com").Get("/only", listUsers)
	app.Get("/only", listUsers)
	spec := modelSpec(t, app)

	urls := func(path string) []string {
		list, _ := modelOperation(t, spec, path, "get")["servers"].([]any) //nolint:errcheck // absent means none
		out := make([]string, 0, len(list))
		for _, server := range list {
			out = append(out, requireMap(t, server)["url"].(string)) //nolint:errcheck,forcetypeassert // the url is always a string
		}
		return out
	}

	pets := modelOperation(t, spec, "/pets", "get")
	require.Equal(t, "A pets", pets["summary"], "the first registration is the one described")
	require.Equal(t, []string{"//a.example.com", "//b.example.com"}, urls("/pets"))
	require.Equal(t, []string{"//a.example.com", "//{tenant}.example.org"}, urls("/pets/{id}"))
	require.Empty(t, urls("/shared"))
	require.Equal(t, []string{"//c.example.com"}, urls("/only"), "a route registered for every host later does not widen the first")
}
