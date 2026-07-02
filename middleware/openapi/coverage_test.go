package openapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// convertToOpenAPIPath converts a Fiber route path pattern to one OpenAPI path
// template. When the path yields multiple variants (optional parameters), it
// returns the first generated variant.
func convertToOpenAPIPath(fiberPath string, params []string) string {
	variants := buildOpenAPIPathVariants(fiberPath, params)
	if len(variants) == 0 {
		return fiberPath
	}
	return variants[0].Path
}

func Test_uniqueOperationID(t *testing.T) {
	t.Parallel()

	used := map[string]struct{}{}
	// Empty id falls back to "operation".
	require.Equal(t, "operation", uniqueOperationID("", used))

	// Collisions get numeric suffixes.
	require.Equal(t, "op", uniqueOperationID("op", used))
	require.Equal(t, "op_2", uniqueOperationID("op", used))
	require.Equal(t, "op_3", uniqueOperationID("op", used))
}

func Test_appendOrReplaceParameter_Guards(t *testing.T) {
	t.Parallel()

	params := []parameter{{Name: "a", In: "query"}}
	index := map[string]int{"query:a": 0}

	// nil / empty Name / empty In are no-ops.
	require.Len(t, appendOrReplaceParameter(params, index, nil), 1)
	require.Len(t, appendOrReplaceParameter(params, index, &parameter{In: "query"}), 1)
	require.Len(t, appendOrReplaceParameter(params, index, &parameter{Name: "a"}), 1)

	// Existing key is replaced in place.
	replaced := appendOrReplaceParameter(params, index, &parameter{Name: "a", In: "query", Description: "x"})
	require.Len(t, replaced, 1)
	require.Equal(t, "x", replaced[0].Description)

	// New key is appended.
	added := appendOrReplaceParameter(params, index, &parameter{Name: "b", In: "query"})
	require.Len(t, added, 2)
}

func Test_schemaFrom(t *testing.T) {
	t.Parallel()

	require.Equal(t, map[string]any{"$ref": "#/x"}, schemaFrom(nil, "#/x", "string"))
	require.Equal(t, map[string]any{"type": "string"}, schemaFrom(nil, "", "string"))
	// No schema and no default type yields nil.
	require.Nil(t, schemaFrom(nil, "", ""))
	// Existing type is preserved.
	require.Equal(t, map[string]any{"type": "integer"}, schemaFrom(map[string]any{"type": "integer"}, "", "string"))
}

func Test_mediaTypesToContent(t *testing.T) {
	t.Parallel()

	// Empty list and only-empty entries both yield nil.
	require.Nil(t, mediaTypesToContent(nil, nil, "", nil, nil))
	require.Nil(t, mediaTypesToContent([]string{""}, nil, "", nil, nil))

	content := mediaTypesToContent([]string{"application/json"}, nil, "", nil, nil)
	require.Contains(t, content, "application/json")
	require.Empty(t, content["application/json"])
}

func Test_routeMediaTypeContent(t *testing.T) {
	t.Parallel()

	require.Nil(t, routeMediaTypeContent(nil))
	// Only an empty media-type key -> nothing emitted.
	require.Nil(t, routeMediaTypeContent(map[string]fiber.RouteMediaType{"": {Schema: map[string]any{"type": "string"}}}))

	out := routeMediaTypeContent(map[string]fiber.RouteMediaType{
		"":                 {Schema: map[string]any{"type": "string"}}, // skipped
		"application/json": {},                                         // empty entry -> {}
	})
	require.Len(t, out, 1)
	require.Empty(t, out["application/json"])
}

func Test_buildRequestBody_Guards(t *testing.T) {
	t.Parallel()

	require.Nil(t, buildRequestBody(nil))
	// Content that resolves to nothing -> no request body.
	require.Nil(t, buildRequestBody(&fiber.RouteRequestBody{Content: map[string]fiber.RouteMediaType{"": {}}}))
}

func Test_shouldIncludeRequestBody(t *testing.T) {
	t.Parallel()

	require.False(t, shouldIncludeRequestBody("", nil))
	require.False(t, shouldIncludeRequestBody("", &fiber.Route{Method: fiber.MethodPost}))
	require.False(t, shouldIncludeRequestBody(fiber.MIMEApplicationJSON, nil))
	require.True(t, shouldIncludeRequestBody(fiber.MIMEApplicationJSON, &fiber.Route{Method: fiber.MethodPost}))
	require.False(t, shouldIncludeRequestBody(fiber.MIMETextPlain, &fiber.Route{Method: fiber.MethodGet, Consumes: fiber.MIMETextPlain}))
	require.True(t, shouldIncludeRequestBody(fiber.MIMETextPlain, &fiber.Route{Method: fiber.MethodPost, Consumes: fiber.MIMETextPlain}))
}

func Test_defaultResponseForMethod(t *testing.T) {
	t.Parallel()

	status, resp := defaultResponseForMethod(fiber.MethodGet, fiber.MIMEApplicationJSON)
	require.Equal(t, "200", status)
	require.Contains(t, resp.Content, fiber.MIMEApplicationJSON)

	status, resp = defaultResponseForMethod(fiber.MethodDelete, fiber.MIMEApplicationJSON)
	require.Equal(t, "204", status)
	require.Nil(t, resp.Content)

	status, resp = defaultResponseForMethod(fiber.MethodGet, "")
	require.Equal(t, "200", status)
	require.Nil(t, resp.Content)
}

func Test_buildServers_Internal(t *testing.T) {
	t.Parallel()

	// Empty-URL servers are skipped; Name kept for 3.2.
	servers := buildServers(&Config{
		OpenAPIVersion: versionOpenAPI32,
		Servers:        []Server{{URL: ""}, {URL: "https://x", Name: "n"}},
	})
	require.Len(t, servers, 1)
	require.Equal(t, "n", servers[0].Name)

	// Name cleared below 3.2.
	servers = buildServers(&Config{
		OpenAPIVersion: versionOpenAPI31,
		Servers:        []Server{{URL: "https://x", Name: "n"}},
	})
	require.Len(t, servers, 1)
	require.Empty(t, servers[0].Name)

	// All servers invalid -> fall back to ServerURL.
	servers = buildServers(&Config{OpenAPIVersion: versionOpenAPI31, Servers: []Server{{URL: ""}}, ServerURL: "https://y"})
	require.Equal(t, "https://y", servers[0].URL)
}

func Test_buildComponents_MergeSecuritySchemes(t *testing.T) {
	t.Parallel()

	require.Nil(t, buildComponents(&Config{}))

	components := buildComponents(&Config{
		Components:      map[string]any{"securitySchemes": map[string]any{"a": map[string]any{"type": "http"}}},
		SecuritySchemes: map[string]any{"b": map[string]any{"type": "apiKey"}},
	})
	schemes := requireMap(t, components["securitySchemes"])
	require.Contains(t, schemes, "a")
	require.Contains(t, schemes, "b")
}

func Test_mergeRouteParameters_Internal(t *testing.T) {
	t.Parallel()

	index := map[string]int{}
	out := mergeRouteParameters(nil, index, []fiber.RouteParameter{
		{Name: "  "},              // blank name -> skipped
		{Name: "q"},               // empty In -> defaults to query
		{Name: "h", In: "Header"}, // normalized to lowercase
	})
	require.Len(t, out, 2)
	require.Equal(t, "query", out[0].In)
	require.Equal(t, "header", out[1].In)
}

func Test_remapRouteParameters_DropsUnknownPathParam(t *testing.T) {
	t.Parallel()

	out := remapRouteParameters(
		[]fiber.RouteParameter{
			{Name: "ghost", In: "path"}, // not a real path param -> dropped
			{Name: "q", In: "query"},    // kept
		},
		map[string]string{},
		[]string{"id"},
	)
	require.Len(t, out, 1)
	require.Equal(t, "q", out[0].Name)
}

func Test_sanitizeParamNames(t *testing.T) {
	t.Parallel()

	// "*"/"+" are trimmed (so trimmed falls back to the original), then each
	// disallowed char becomes "_".
	require.Equal(t, "___", sanitizeOpenAPIParamName("***", 1))
	// A fully empty name sanitizes to the positional fallback.
	require.Equal(t, "param1", sanitizeOpenAPIParamName("", 1))
	require.Equal(t, "id", sanitizeOpenAPIParamName("id", 1))

	require.Equal(t, wildcardParamName, sanitizeOpenAPIWildcardParamName("*", 1))
	require.Equal(t, wildcardParamName, sanitizeOpenAPIWildcardParamName("_._", 1))
	require.NotEmpty(t, sanitizeOpenAPIWildcardParamName("*5", 1))
	require.Contains(t, sanitizeOpenAPIWildcardParamName("*foo", 1), wildcardParamName)
}

func Test_resolveParamNames(t *testing.T) {
	t.Parallel()

	// Blank extracted + blank params entry keeps raw empty, sanitizes to paramN.
	resolved := resolveOpenAPIPathParamName(0, "", []string{""})
	require.Empty(t, resolved.raw)
	require.Equal(t, "param1", resolved.openAPI)

	// Provided param name overrides the extracted token.
	resolved = resolveOpenAPIPathParamName(0, "x", []string{"override"})
	require.Equal(t, "override", resolved.raw)

	// Wildcard with a provided name.
	wild := resolveOpenAPIWildcardParamName(0, []string{"rest"})
	require.Equal(t, "rest", wild.raw)
	require.Contains(t, wild.openAPI, wildcardParamName)
}

func Test_buildOpenAPIPathVariants_Edge(t *testing.T) {
	t.Parallel()

	// Empty path -> single "/" variant.
	variants := buildOpenAPIPathVariants("", nil)
	require.Len(t, variants, 1)
	require.Equal(t, "/", variants[0].Path)

	// Nested angle-bracket constraint exercises the depth counter.
	variants = buildOpenAPIPathVariants("/:id<range(1<2)>", nil)
	require.Equal(t, "/{id}", variants[0].Path)

	// convertToOpenAPIPath delegates to the first variant.
	require.Equal(t, "/{id}", convertToOpenAPIPath("/:id", nil))

	// Generated variants must always be unique.
	dup := buildOpenAPIPathVariants("/:a?/:a?", nil)
	seen := map[string]struct{}{}
	for _, v := range dup {
		key := v.Path + "|" + strings.Join(v.ParamNames, ",")
		_, exists := seen[key]
		require.False(t, exists, "variants must be unique")
		seen[key] = struct{}{}
	}
}

func Test_inferExampleValue_NoType(t *testing.T) {
	t.Parallel()

	// Missing/!string type returns the raw value unchanged.
	require.Equal(t, "x", inferExampleValue("x", map[string]any{}))
	require.Equal(t, "x", inferExampleValue("x", map[string]any{"type": 123}))
}

func Test_SchemaOf_UnsupportedSliceAndMapElements(t *testing.T) {
	t.Parallel()

	type withUnsupported struct {
		M  map[string]chan int `json:"m"`
		Ch []chan int          `json:"ch"`
	}

	schema := SchemaOf(withUnsupported{})
	props := requireMap(t, schema["properties"])

	ch := requireProp(t, props, "ch")
	require.Equal(t, "array", ch["type"])
	require.Equal(t, map[string]any{}, ch["items"])

	m := requireProp(t, props, "m")
	require.Equal(t, "object", m["type"])
	require.Equal(t, map[string]any{}, m["additionalProperties"])
}

func Test_OpenAPI_SpecMarshalError(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	// A non-marshalable operation extension makes spec marshaling fail.
	app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
		OperationExtension(map[string]any{"bad": func() {}})
	app.Use(New())

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func Test_OpenAPI_SwaggerOptionsMarshalError(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New(Config{SwaggerOptions: map[string]any{"bad": func() {}}}))

	req := httptest.NewRequest(fiber.MethodGet, "/swagger", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}
