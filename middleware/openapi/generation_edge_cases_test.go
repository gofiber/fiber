package openapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

//nolint:gocritic // unnamedResult: nonamedreturns forbids naming these
func specBodyOf(t *testing.T, app *fiber.App, path string) (int, string) {
	t.Helper()
	resp := apptest.Do(t, app, path)
	b := apptest.BodyBytes(t, resp)
	require.NoError(t, resp.Body.Close())
	return resp.StatusCode, string(b)
}

// hostGet requests path on host and returns the status and body.
//
//nolint:gocritic // unnamedResult: named results conflict with nonamedreturns
func hostGet(t *testing.T, app *fiber.App, host, path string) (int, string) {
	t.Helper()
	resp := apptest.Do(t, app, apptest.Req(fiber.MethodGet, path, apptest.WithHost(host)))
	body := apptest.BodyBytes(t, resp)
	return resp.StatusCode, string(body)
}

// rfHostSpec requests the document at /openapi.json on host and decodes it.
func rfHostSpec(t *testing.T, app *fiber.App, host string) map[string]any {
	t.Helper()
	status, body := hostGet(t, app, host, "/openapi.json")
	require.Equal(t, fiber.StatusOK, status)
	var spec map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &spec))
	return spec
}

// rfServerURLs lists the server URLs an operation names; an operation for every host names none.
func rfServerURLs(t *testing.T, spec map[string]any, path string) []string {
	t.Helper()
	raw, ok := modelOperation(t, spec, path, "get")["servers"]
	if !ok {
		return nil
	}
	var urls []string
	for _, server := range requireSlice(t, raw) {
		urls = append(urls, requireString(t, requireMap(t, server)["url"]))
	}
	return urls
}

func Test_OpenAPI_GenerationEdgeCases(t *testing.T) {
	t.Parallel()
	t.Run("UIMountServesSpecUnderMount", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/u", listUsers)
		app.Use("/swagger", New())
		apptest.Expect(t, app, "/swagger", fiber.StatusOK)
		require.Contains(t, apptest.Body(t, apptest.Expect(t, app, "/swagger/openapi.json", fiber.StatusOK)), "/u")
	})

	t.Run("WildcardDoesNotLeakSpec", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/secret", listUsers)
		app.Get("/docs/*", New())
		apptest.Expect(t, app, "/docs/anything", fiber.StatusNotFound)
		require.Contains(t, apptest.Body(t, apptest.Expect(t, app, "/docs/openapi.json", fiber.StatusOK)), "/secret")
	})

	t.Run("MidPathOptionalNotDocumented", func(t *testing.T) {
		t.Parallel()
		paths := requireMap(t, rfSpec(t, func(app *fiber.App) { app.Get("/api/:ver?/users/:id", listUsers) })["paths"])
		require.NotContains(t, paths, "/api/users/{id}")
		require.Contains(t, paths, "/api/{ver}/users/{id}")
	})

	t.Run("HierarchyDedupAcrossMethods", func(t *testing.T) {
		t.Parallel()
		paths := requireMap(t, rfSpec(t, func(app *fiber.App) {
			app.Get("/users/:id", listUsers)
			app.Post("/users/:userID", listUsers)
		})["paths"])
		require.Len(t, paths, 1)
		require.Contains(t, requireMap(t, paths["/users/{id}"]), "get")
		require.Contains(t, requireMap(t, paths["/users/{id}"]), "post")
	})

	for _, tc := range []struct {
		register func(app *fiber.App)
		name     string
		contains []string
	}{
		{name: "ResponseSchemaWithoutProducesKept", contains: []string{"application/json", `"type":"object"`}, register: func(app *fiber.App) {
			app.Get("/x", listUsers).ResponseWithExample(200, "OK", map[string]any{"type": "object"}, "", map[string]any{"id": 1}, nil)
		}},
		{name: "BigIntSurvivesExtensions", contains: []string{"1234567890123456789", "x-owner"}, register: func(app *fiber.App) {
			app.Get("/big", listUsers).ResponseWithExample(200, "ok", nil, "", map[string]any{"id": int64(1234567890123456789)}, nil, fiber.MIMEApplicationJSON).
				OperationExtension(map[string]any{"x-owner": "team"})
		}},
		{name: "ResponseContentKeepsDescription", contains: []string{"My custom description"}, register: func(app *fiber.App) {
			app.Get("/y", listUsers).Response(201, "My custom description", fiber.MIMEApplicationJSON).
				ResponseContent(201, "", map[string]fiber.RouteMediaType{fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}}})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			tc.register(app)
			app.Use(New())
			_, body := specBodyOf(t, app, "/openapi.json")
			for _, want := range tc.contains {
				require.Contains(t, body, want)
			}
		})
	}

	t.Run("CustomMethodNotEmitted", func(t *testing.T) {
		t.Parallel()
		app := fiber.New(fiber.Config{RequestMethods: append(append([]string{}, fiber.DefaultMethods...), "PROPFIND")})
		app.Add([]string{"PROPFIND"}, "/res", listUsers)
		app.Use(New())
		require.NotContains(t, string(fetchSpecBody(t, app)), "propfind")
	})

	t.Run("HeadDefaultMirrorsGet", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/mirror", listUsers)
		app.Head("/mirror", listUsers)
		app.Use(New())
		ops := fetchSpec(t, app).Paths["/mirror"]
		require.Contains(t, ops, "head")
		require.Equal(t, ops["get"].Responses, ops["head"].Responses)
		require.Contains(t, ops["head"].Responses, "200")
		require.NotContains(t, ops["head"].Responses, "204")
	})
}

func Test_OpenAPI_DomainRoutesCarryTheirHost(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Domain("api.example.com").Get("/pets", listUsers)
		app.Domain(":tenant.example.com").Get("/orgs", listUsers)
		app.Get("/open", listUsers)
		sub := fiber.New()
		sub.Get("/s", listUsers)
		app.Domain("api.example.com").Use("/dm", sub)
	})
	servers := func(path string) []any {
		list := requireSlice(t, modelOperation(t, spec, path, "get")["servers"])
		require.Len(t, list, 1, path)
		return list
	}
	pets := requireMap(t, servers("/pets")[0])
	require.Equal(t, "//api.example.com", pets["url"])
	require.NotContains(t, pets, "variables")
	org := requireMap(t, servers("/orgs")[0])
	require.Equal(t, "//{tenant}.example.com", org["url"])
	require.Equal(t, "tenant", requireMap(t, requireMap(t, org["variables"])["tenant"])["default"])
	require.Equal(t, "//api.example.com", requireMap(t, servers("/dm/s")[0])["url"], "a domain-mounted sub app carries the host")
	require.NotContains(t, modelOperation(t, spec, "/open", "get"), "servers")
}

func Test_RouteLexers_Agree(t *testing.T) {
	t.Parallel()
	// segments.go and paths.go lex the route grammar separately; both must find the same parameter count.
	for _, pattern := range []string{
		"/users", "/users/:id", "/users/:id<int>/posts/:post?", "/files/*", "/files/+", "/a/:x-:y/b",
		`/lit\:eral/:id`, `/esc\*aped/*`, "/:lang<regex(en|de)>?/docs", "/:a?/:b?/:c", "/v:ver/api", "/p/:id<range(1,10)>",
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
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/users", listUsers).Name("list all users")
		app.Get("/users/:id", listUsers).Name("")
	})
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
	require.Contains(t, requireMap(t, rfSpec(t, func(app *fiber.App) { app.Get("/a/", listUsers) })["paths"]), "/a")

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

func Test_OpenAPI_DocumentLevelEmptySecurity(t *testing.T) {
	t.Parallel()
	register := func(app *fiber.App) { app.Get("/x", listUsers) }
	require.Equal(t, []any{}, rfSpec(t, register, Config{Security: []map[string][]string{}})["security"])
	require.NotContains(t, rfSpec(t, register), "security")
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
	for version, want := range map[string]string{"3.0.3": "3.0.3", "3.1.1": "3.1.1", "3.2.0": "3.2.0", "3.1.0": "3.1.0", "": "3.1.0"} {
		require.Equal(t, want, rfSpec(t, func(app *fiber.App) { app.Get("/x", listUsers) }, Config{OpenAPIVersion: version})["openapi"], version)
	}
	for _, version := range []string{"3.2", "3.3.0", "2.0.0", "3.1.x", "garbage", " 3.1.0"} {
		require.PanicsWithValue(t, `openapi: OpenAPIVersion must be a 3.0.x, 3.1.x or 3.2.x release, got "`+version+`"`, func() {
			New(Config{OpenAPIVersion: version})
		}, version)
	}
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

func Test_OpenAPI_ReservedHeaderParametersAreIgnored(t *testing.T) {
	t.Parallel()
	str := map[string]any{"type": "string"}
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/x", listUsers).
			Parameter("Accept", fiber.ParamInHeader, false, str, "ignored").
			Parameter("content-type", fiber.ParamInHeader, false, str, "ignored").
			Parameter("Authorization", fiber.ParamInHeader, false, str, "ignored").
			Parameter("X-Trace", fiber.ParamInHeader, false, str, "kept").
			Parameter("Accept", fiber.ParamInQuery, false, str, "a query parameter named Accept stays")
	})
	require.ElementsMatch(t, []string{"header:X-Trace", "query:Accept"}, rfKeys(chainParams(t, modelOperation(t, spec, "/x", "get"))))
}

func Test_OpenAPI_QuerystringExcludesQueryParameters(t *testing.T) {
	t.Parallel()
	params := func(version string) map[string]map[string]any {
		spec := rfSpec(t, func(app *fiber.App) {
			app.Get("/x", listUsers).
				Parameter("page", fiber.ParamInQuery, false, map[string]any{"type": "integer"}, "p").
				AddParameter(fiber.RouteParameter{Name: "qs", In: fiber.ParamInQuerystring, Content: map[string]fiber.RouteMediaType{"application/x-www-form-urlencoded": {Schema: map[string]any{"type": "object"}}}}).
				Parameter("X-Trace", fiber.ParamInHeader, false, map[string]any{"type": "string"}, "h")
		}, Config{OpenAPIVersion: version})
		return chainParams(t, modelOperation(t, spec, "/x", "get"))
	}
	require.ElementsMatch(t, []string{"querystring:qs", "header:X-Trace"}, rfKeys(params("3.2.0")))
	// Before 3.2 there is no querystring location, so the query parameter stays.
	require.ElementsMatch(t, []string{"query:page", "header:X-Trace"}, rfKeys(params("3.1.0")))
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
	spec := modelSpec(t, app, Config{SecuritySchemes: map[string]any{"oauth": scheme, "other": scheme}})
	require.Equal(t, []any{map[string]any{"oauth": []any{"read"}}}, rfSecurity(t, spec, "/x"))
}

func Test_Dialect_Capabilities(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
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
	} {
		d := newDialect(tc.version)
		require.Equal(t, tc.query, d.hasQueryMethod(), tc.version)
		require.Equal(t, tc.querystring, d.hasQuerystringLocation(), tc.version)
		require.Equal(t, tc.serverName, d.hasServerName(), tc.version)
		require.Equal(t, tc.self, d.hasSelf(), tc.version)
		require.Equal(t, tc.jsonSchemaDialect, d.hasJSONSchemaDialect(), tc.version)
	}
	both := License{Name: "MIT", URL: "https://x", Identifier: "MIT"}
	require.Equal(t, License{Name: "MIT", Identifier: "MIT"}, newDialect("3.1.0").license(both))
	require.Equal(t, License{Name: "MIT", URL: "https://x"}, newDialect("3.0.3").license(both))
}

func Test_Dialect_Nullable(t *testing.T) {
	t.Parallel()
	ref := map[string]any{"$ref": "#/components/schemas/A"}
	composed := map[string]any{"oneOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "integer"}}}
	// 3.0 defines nullable only next to a type, so a reference or composition gets a branch that holds only null.
	nullOnly := map[string]any{"type": "object", "nullable": true, "enum": []any{nil}}
	old, modern := newDialect("3.0.3"), newDialect("3.1.0")
	for _, tc := range []struct {
		in   map[string]any
		want map[string]any
		name string
		d    dialect
	}{
		{name: "3.0 typed", d: old, in: map[string]any{"type": "string", "enum": []any{"a"}}, want: map[string]any{"type": "string", "enum": []any{"a", nil}, "nullable": true}},
		{name: "3.0 ref", d: old, in: ref, want: map[string]any{"anyOf": []any{ref, nullOnly}}},
		{name: "3.0 composition", d: old, in: composed, want: map[string]any{"anyOf": []any{composed, nullOnly}}},
		{name: "3.0 typed enum", d: old, in: map[string]any{"type": "integer", "enum": []int{1, 2}}, want: map[string]any{"type": "integer", "enum": []any{1, 2, nil}, "nullable": true}},
		{name: "3.1 typed", d: modern, in: map[string]any{"type": "string", "enum": []any{"a"}}, want: map[string]any{"type": []any{"string", "null"}, "enum": []any{"a", nil}}},
		{name: "3.1 ref", d: modern, in: ref, want: map[string]any{"anyOf": []any{ref, map[string]any{"type": "null"}}}},
		{name: "3.1 composition", d: modern, in: composed, want: map[string]any{"anyOf": []any{composed, map[string]any{"type": "null"}}}},
		{name: "3.1 accepts anything already", d: modern, in: map[string]any{}, want: map[string]any{}},
		{name: "3.1 list type", d: modern, in: map[string]any{"type": []string{"string", "integer"}, "enum": []string{"a"}}, want: map[string]any{"type": []any{"string", "integer", "null"}, "enum": []any{"a", nil}}},
		{name: "3.1 null not added twice", d: modern, in: map[string]any{"type": []any{"string", "null"}}, want: map[string]any{"type": []any{"string", "null"}}},
		{name: "3.1 list type and typed enum", d: modern, in: map[string]any{"type": []any{"string"}, "enum": []int{1}}, want: map[string]any{"type": []any{"string", "null"}, "enum": []any{1, nil}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			before, err := json.Marshal(tc.in)
			require.NoError(t, err)
			typ, enum := tc.in["type"], tc.in["enum"]
			require.Equal(t, tc.want, tc.d.nullable(tc.in))
			after, err := json.Marshal(tc.in)
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after), "the input is left alone")
			require.Equal(t, typ, tc.in["type"])
			require.Equal(t, enum, tc.in["enum"])
		})
	}
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
	spec := rfSpec(t, func(app *fiber.App) {
		app.Group("/users").Get("/", listUsers)
		app.Get("/items", listUsers).Tags("zeta", "described")
		app.Get("/plain-root", listUsers)
	}, Config{Tags: []Tag{{Name: "described", Description: "kept as written"}}})
	require.Equal(t, []any{
		map[string]any{"name": "described", "description": "kept as written"},
		map[string]any{"name": "users"},
		map[string]any{"name": "zeta"},
	}, spec["tags"], "declared tags first, then the rest sorted")
	require.NotContains(t, rfSpec(t, func(app *fiber.App) { app.Get("/x", listUsers) }, Config{DisableGroupTags: true}), "tags")
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

func Test_PathLexer_AgreesWithTheRouter(t *testing.T) {
	t.Parallel()
	for _, pattern := range []string{
		"/users/:id", "/users/:id<int>/posts/:post", "/x/:date<regex([^>]+)>", "/api/v1/:param<regex(^[^>]+$)>",
		"/shop/product/color::color/size::size", "/a:b/c", "/files/*", "/files/+/x", "/a/:x-:y/b", `/lit\:eral/:id`,
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

	variants := buildOpenAPIPathVariants(`/e/a\{b\}`, nil, false)
	require.Len(t, variants, 1)
	require.Equal(t, "/e/a%7Bb%7D", variants[0].Path, "escaped braces are encoded")
}

func Test_OpenAPI_PathsMatchServedRoutes(t *testing.T) {
	t.Parallel()
	paths := requireMap(t, rfSpec(t, func(app *fiber.App) {
		app.Get("/x/:date<regex([^>]+)>", listUsers)
		app.Get("/shop/product/color::color/size::size", listUsers)
	})["paths"])
	require.Contains(t, paths, "/x/{date}")
	require.NotContains(t, paths, "/x/{date}]{wildcard})>")
	require.Contains(t, paths, "/shop/product/color:{color}/size:{size}")
}

func Test_OpenAPI_GroupTagSkipsVersionMarkersInNamesToo(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	api, ok := app.Group("/api").Name("api.").(*fiber.Group)
	require.True(t, ok)
	v1, ok := api.Group("/v1").Name("v1.").(*fiber.Group)
	require.True(t, ok)
	v1.Get("/users", listUsers)
	app.Group("/api").Group("/v1").Get("/items", listUsers)
	spec := modelSpec(t, app)
	require.Equal(t, []any{"api"}, modelOperation(t, spec, "/api/v1/users", "get")["tags"])
	require.Equal(t, []any{"api"}, modelOperation(t, spec, "/api/v1/items", "get")["tags"])
}

func Test_OpenAPI_SamePathOnSeveralDomainsListsEveryHost(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Domain("a.example.com").Get("/pets", listUsers).Summary("A pets")
		app.Domain("b.example.com").Get("/pets", listUsers).Summary("B pets")
		app.Domain("b.example.com").Get("/pets", listUsers) // a repeated host adds nothing
		app.Domain("a.example.com").Get("/pets/:id", listUsers)
		app.Domain(":tenant.example.org").Get("/pets/:name", listUsers) // same template up to the parameter name
		app.Get("/shared", listUsers)
		app.Domain("a.example.com").Get("/shared", listUsers) // the route for every host already covers it
		app.Domain("c.example.com").Get("/only", listUsers)
		app.Get("/only", listUsers)
	})
	require.Equal(t, "A pets", modelOperation(t, spec, "/pets", "get")["summary"], "the first registration is the one described")
	require.Equal(t, []string{"//a.example.com", "//b.example.com"}, rfServerURLs(t, spec, "/pets"))
	require.Equal(t, []string{"//a.example.com", "//{tenant}.example.org"}, rfServerURLs(t, spec, "/pets/{id}"))
	require.Empty(t, rfServerURLs(t, spec, "/shared"))
	require.Empty(t, rfServerURLs(t, spec, "/only"), "a route registered for every host later makes the operation one for every host")
}

func Test_OpenAPI_EffectiveHost(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
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
	} {
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
	for _, tc := range []struct {
		name, cover, route string
		want               bool
	}{
		{"any host covers every host", "", "api.example.com", true},
		{"any host covers any host", "", "", true},
		{"a host never covers every host", "api.example.com", "", false},
		{"equal hosts", "api.example.com", "api.example.com", true},
		{"a parameter covers a literal", ":tenant.example.com", "api.example.com", true},
		{"a literal does not cover a parameter", "api.example.com", ":tenant.example.com", false},
		{"different hosts", "a.example.com", "b.example.com", false},
		{"different label counts", "example.com", "api.example.com", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, hostCovers(tc.cover, tc.route))
		})
	}
}

func Test_OpenAPI_AnyHostRouteAndADomainRoute(t *testing.T) {
	t.Parallel()
	t.Run("an any-host route after a domain route widens the operation", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Domain("a.example.com").Get("/x", func(c fiber.Ctx) error { return c.SendString("domain") })
		app.Get("/x", func(c fiber.Ctx) error { return c.SendString("any") })
		_, body := hostGet(t, app, "a.example.com", "/x")
		require.Equal(t, "domain", body)
		_, body = hostGet(t, app, "b.example.com", "/x")
		require.Equal(t, "any", body, "both routes answer, each on its own hosts")
		require.NotContains(t, modelOperation(t, modelSpec(t, app), "/x", "get"), "servers", "the operation no longer claims one host")
	})

	t.Run("an any-host route before a domain route shadows it", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/x", func(c fiber.Ctx) error { return c.SendString("any") }).Summary("any host")
		app.Domain("a.example.com").Get("/x", func(c fiber.Ctx) error { return c.SendString("domain") }).Summary("a.example.com")
		_, body := hostGet(t, app, "a.example.com", "/x")
		require.Equal(t, "any", body, "the route for every host answers first")
		op := modelOperation(t, modelSpec(t, app), "/x", "get")
		require.Equal(t, "any host", op["summary"])
		require.NotContains(t, op, "servers")
	})
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
	require.NotContains(t, requireMap(t, spec["paths"]), "/v1/never", "a route no host reaches is not documented")
	servers := requireSlice(t, modelOperation(t, spec, "/v1/pets", "get")["servers"])
	require.Equal(t, []any{map[string]any{"url": "//api.example.com"}}, servers, "the literal replaced the parameter")
}

func Test_OpenAPI_MiddlewareCoversRoutesByHost(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, guard, route string
		want               bool
	}{
		{"a parameter host covers a literal one", ":tenant.example.com", "api.example.com", true},
		{"a literal host covers itself", "api.example.com", "api.example.com", true},
		{"a literal host does not cover a parameter one", "api.example.com", ":other.example.com", false},
		{"another host is not covered", "a.example.com", "b.example.com", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New())
			app.Domain(tc.guard).Use(chainKeyAuth())
			app.Domain(tc.route).Get("/x", listUsers)
			_, secured := modelOperation(t, rfHostSpec(t, app, "unrelated.example.net"), "/x", "get")["security"]
			require.Equal(t, tc.want, secured)
		})
	}
}

func Test_OpenAPI_ServedFromADomainRouter(t *testing.T) {
	t.Parallel()
	const host = "api.example.com"
	for _, tc := range []struct {
		register                      func(app *fiber.App)
		name, specPath, uiPath, reqTo string
	}{
		{name: "Use", register: func(app *fiber.App) { app.Domain(host).Use(New()) }, specPath: "/openapi.json", uiPath: "/swagger"},
		{name: "Use with a prefix", register: func(app *fiber.App) { app.Domain(host).Use("/docs", New()) }, specPath: "/docs/openapi.json", uiPath: "/docs/swagger"},
		{name: "Get", register: func(app *fiber.App) { app.Domain(host).Get("/openapi.json", New()) }, specPath: "/openapi.json"},
		{name: "RouteChain", register: func(app *fiber.App) { app.Domain(host).RouteChain("/openapi.json").Get(New()) }, specPath: "/openapi.json"},
		{name: "a group of the domain router", register: func(app *fiber.App) { app.Domain(host).Group("/api").Use(New()) }, specPath: "/api/openapi.json", uiPath: "/api/swagger"},
		{name: "the domain router of a group", register: func(app *fiber.App) { app.Group("/v1").Domain(host).Use(New()) }, specPath: "/v1/openapi.json", uiPath: "/v1/swagger"},
		{name: "a host with a parameter", register: func(app *fiber.App) { app.Domain(":tenant.example.com").Use(New()) }, specPath: "/openapi.json", uiPath: "/swagger", reqTo: "acme.example.com"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Get("/public", listUsers)
			app.Domain(host).Get("/pets", listUsers)
			tc.register(app)
			requestHost := host
			if tc.reqTo != "" {
				requestHost = tc.reqTo
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
		require.Equal(t, title, requireMap(t, rfHostSpec(t, app, host)["info"])["title"], host)
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

	spec := rfHostSpec(t, app, "api.example.com")
	require.NotNil(t, rfSecurity(t, spec, "/pets"))
	require.Nil(t, rfSecurity(t, spec, "/health"))
	require.NotContains(t, requireMap(t, spec["paths"]), "/secret", "Hidden works on a domain route")
	require.NotNil(t, rfSecurity(t, spec, "/inner"), "keyauth on the domain router covers its routes")
}

func fullPolicy() *routePolicy {
	return &routePolicy{
		equal:               func(a, b string) bool { return a == b },
		defaultProduces:     fiber.MIMEApplicationJSON,
		defaultConsumes:     fiber.MIMEApplicationJSON,
		handlerSummaries:    true,
		groupTags:           true,
		middlewareInference: true,
		validates:           true,
	}
}

func Test_resolveRoute(t *testing.T) {
	t.Parallel()
	without := func(change func(p *routePolicy)) *routePolicy {
		policy := fullPolicy()
		change(policy)
		return policy
	}

	route := &fiber.Route{Method: fiber.MethodGet, Path: "/users", Handlers: []fiber.Handler{listUsers}}
	require.Equal(t, "List users", resolveRoute(route, nil, fullPolicy()).summary)
	require.Empty(t, resolveRoute(route, nil, without(func(p *routePolicy) { p.handlerSummaries = false })).summary)
	route.Summary = "Explicit"
	require.Equal(t, "Explicit", resolveRoute(route, nil, fullPolicy()).summary)

	route = &fiber.Route{Method: fiber.MethodPost, Path: "/x"}
	facts := resolveRoute(route, nil, fullPolicy())
	require.Equal(t, fiber.MIMEApplicationJSON, facts.respType)
	require.Equal(t, fiber.MIMEApplicationJSON, facts.bodyType)
	require.Empty(t, facts.ownConsumes, "a default is not a statement by the route")
	route.Produces = fiber.MIMETextPlain
	route.Consumes = fiber.MIMEApplicationForm
	facts = resolveRoute(route, nil, fullPolicy())
	require.Equal(t, fiber.MIMETextPlain, facts.respType)
	require.Equal(t, fiber.MIMEApplicationForm, facts.bodyType)
	require.Equal(t, fiber.MIMEApplicationForm, facts.ownConsumes)

	declares := &fiber.Route{Method: fiber.MethodPost, Path: "/x", Parameters: []fiber.RouteParameter{{Name: "q", In: fiber.ParamInQuery}}}
	require.True(t, resolveRoute(declares, nil, fullPolicy()).badRequest)
	require.False(t, resolveRoute(&fiber.Route{Method: fiber.MethodGet, Path: "/x"}, nil, fullPolicy()).badRequest, "nothing to reject")
	require.False(t, resolveRoute(declares, nil, without(func(p *routePolicy) { p.validates = false })).badRequest, "no validator")

	covering := []coveringMiddleware{{prefix: "/", set: 1 << kindBasicAuth}}
	route = &fiber.Route{Method: fiber.MethodGet, Path: "/x"}
	facts = resolveRoute(route, covering, fullPolicy())
	require.True(t, facts.chain.has(kindBasicAuth))
	require.Nil(t, facts.security, "the requirement is registered later")
	route.Security = []map[string][]string{}
	facts = resolveRoute(route, covering, fullPolicy())
	require.False(t, facts.chain.has(kindBasicAuth), "documented security suppresses inference")
	require.NotNil(t, facts.security)
	require.Empty(t, *facts.security)
	route.Security = nil
	require.Zero(t, resolveRoute(route, covering, without(func(p *routePolicy) { p.middlewareInference = false })).chain)
}
