package openapi

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// Test_OpenAPI_ParameterContent covers describing a parameter by media type,
// which any location allows and 3.2 "querystring" requires.
func Test_OpenAPI_ParameterContent(t *testing.T) {
	t.Parallel()

	t.Run("content replaces schema", func(t *testing.T) {
		t.Parallel()

		spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
			app.Get("/filter", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
				AddParameter(fiber.RouteParameter{
					Name:        "criteria",
					In:          "query",
					Description: "JSON encoded filter",
					Required:    true,
					// A schema alongside content is discarded: the Parameter
					// Object carries exactly one of the two.
					Schema: map[string]any{"type": "string"},
					Content: map[string]fiber.RouteMediaType{
						fiber.MIMEApplicationJSON: {
							Schema:  map[string]any{"type": "object"},
							Example: map[string]any{"color": "red"},
						},
					},
				})
		})

		op := requireMap(t, requireMap(t, requireMap(t, spec["paths"])["/filter"])["get"])
		params, ok := op["parameters"].([]any)
		require.True(t, ok)
		require.Len(t, params, 1)

		param := requireMap(t, params[0])
		require.Equal(t, "criteria", param["name"])
		require.Equal(t, "query", param["in"])
		require.Equal(t, true, param["required"])
		require.NotContains(t, param, "schema")

		entry := requireMap(t, requireMap(t, param["content"])[fiber.MIMEApplicationJSON])
		require.Equal(t, map[string]any{"type": "object"}, requireMap(t, entry["schema"]))
		require.Equal(t, map[string]any{"color": "red"}, requireMap(t, entry["example"]))
	})

	t.Run("querystring without schema stays valid", func(t *testing.T) {
		t.Parallel()

		spec := fetchSpecWithConfig(t, Config{OpenAPIVersion: "3.2.0"}, func(app *fiber.App) {
			app.Get("/q", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
				AddParameter(fiber.RouteParameter{Name: "filter", In: "querystring"})
		})

		op := requireMap(t, requireMap(t, requireMap(t, spec["paths"])["/q"])["get"])
		params, ok := op["parameters"].([]any)
		require.True(t, ok)

		param := requireMap(t, params[0])
		require.NotContains(t, param, "schema")
		entry := requireMap(t, requireMap(t, param["content"])[querystringMediaType])
		// A bare querystring parameter still has to describe something.
		require.Equal(t, map[string]any{"type": "string"}, requireMap(t, entry["schema"]))
	})

	t.Run("querystring honors an explicit content map", func(t *testing.T) {
		t.Parallel()

		spec := fetchSpecWithConfig(t, Config{OpenAPIVersion: "3.2.0"}, func(app *fiber.App) {
			app.Get("/q", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
				AddParameter(fiber.RouteParameter{
					Name: "filter",
					In:   "querystring",
					Content: map[string]fiber.RouteMediaType{
						fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}},
					},
				})
		})

		op := requireMap(t, requireMap(t, requireMap(t, spec["paths"])["/q"])["get"])
		params, ok := op["parameters"].([]any)
		require.True(t, ok)

		content := requireMap(t, requireMap(t, params[0])["content"])
		require.NotContains(t, content, querystringMediaType)
		require.Contains(t, content, fiber.MIMEApplicationJSON)
	})

	t.Run("caller map is not aliased", func(t *testing.T) {
		t.Parallel()

		content := map[string]fiber.RouteMediaType{
			fiber.MIMEApplicationJSON: {Schema: map[string]any{"type": "object"}},
		}

		app := fiber.New()
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
			AddParameter(fiber.RouteParameter{Name: "p", In: "query", Content: content})

		// Mutating the caller's map after registration must not reach the route.
		content[fiber.MIMEApplicationJSON].Schema["type"] = "array"
		delete(content, fiber.MIMEApplicationJSON)

		app.Use(New())
		_, body := specBodyOf(t, app, "/openapi.json")
		require.Contains(t, body, `"type":"object"`)
		require.NotContains(t, body, `"type":"array"`)
	})
}

// Test_OpenAPI_OperationDescriptionOmitted asserts that an operation without a
// description omits the optional key instead of emitting an empty string.
func Test_OpenAPI_OperationDescriptionOmitted(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		app.Get("/bare", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Get("/documented", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
			Description("has one")
	})

	paths := requireMap(t, spec["paths"])
	bare := requireMap(t, requireMap(t, paths["/bare"])["get"])
	require.NotContains(t, bare, "description")

	documented := requireMap(t, requireMap(t, paths["/documented"])["get"])
	require.Equal(t, "has one", documented["description"])
}

// Test_SchemaOf_OmitZero asserts that a field encoding/json omits when zero is
// not reported as required.
func Test_SchemaOf_OmitZero(t *testing.T) {
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

	props := requireMap(t, schema["properties"])
	require.Contains(t, props, "zero")
	require.Contains(t, props, "empty")
}

// Test_SchemaOf_InvalidJSONTagName asserts a tag name encoding/json rejects
// falls back to the field name here too, so schema and wire agree.
func Test_SchemaOf_InvalidJSONTagName(t *testing.T) {
	t.Parallel()

	// Built reflectively: `go vet`'s structtag check rejects a literal tag
	// carrying a reserved character.
	typ := reflect.StructOf([]reflect.StructField{
		{Name: "Reserved", Type: reflect.TypeFor[string](), Tag: `json:"a\\b"`},
		{Name: "Punctuated", Type: reflect.TypeFor[string](), Tag: `json:"a b"`},
	})
	value := reflect.New(typ).Elem()
	value.Field(0).SetString("x")
	value.Field(1).SetString("y")

	encoded, err := json.Marshal(value.Interface())
	require.NoError(t, err)

	var wire map[string]any
	require.NoError(t, json.Unmarshal(encoded, &wire))

	props := requireMap(t, SchemaOf(value.Interface())["properties"])
	for name := range wire {
		require.Containsf(t, props, name, "schema is missing wire property %q", name)
	}
	require.Len(t, props, len(wire))
	// Backslash is reserved, so encoding/json ignores the rename; a space is not.
	require.Contains(t, props, "Reserved")
	require.Contains(t, props, "a b")
}

// Test_OpenAPI_PathParameterConstraintSchema asserts a "<...>" constraint types
// the path parameter schema instead of reporting every one as a string.
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
		// An unknown constraint must not invent a type.
		{map[string]any{"type": "string"}, "/o/:v<nonexistent>"},
		// The first constraint that pins a type keeps it.
		{map[string]any{"type": "integer", "minimum": 5.0}, "/p/:v<int;min(5)>"},
		// A lowercase alias resolves like the router resolves it.
		{map[string]any{"type": "string", "minLength": 2.0}, "/q/:v<minlen(2)>"},
	}

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		for _, tc := range cases {
			app.Get(tc.pattern, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		}
	})

	paths := requireMap(t, spec["paths"])
	for _, tc := range cases {
		openAPIPath := tc.pattern[:strings.IndexByte(tc.pattern, ':')] + "{v}"
		op := requireMap(t, requireMap(t, paths[openAPIPath])["get"])
		params, ok := op["parameters"].([]any)
		require.Truef(t, ok, "%s has no parameters", tc.pattern)
		require.Lenf(t, params, 1, "%s", tc.pattern)
		require.Equalf(t, tc.expected, requireMap(t, requireMap(t, params[0])["schema"]), "%s", tc.pattern)
	}
}

// Test_OpenAPI_ConstraintSchemaKeepsExplicitParameter asserts that AddParameter
// still overrides a constraint-derived schema for the same path parameter.
func Test_OpenAPI_ConstraintSchemaKeepsExplicitParameter(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		app.Get("/items/:id<int>", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
			AddParameter(fiber.RouteParameter{
				Name:        "id",
				In:          "path",
				Description: "item identifier",
				Schema:      map[string]any{"type": "integer", "format": "int64"},
			})
	})

	op := requireMap(t, requireMap(t, requireMap(t, spec["paths"])["/items/{id}"])["get"])
	params, ok := op["parameters"].([]any)
	require.True(t, ok)
	require.Len(t, params, 1)

	param := requireMap(t, params[0])
	require.Equal(t, "item identifier", param["description"])
	require.Equal(t, map[string]any{"type": "integer", "format": "int64"}, requireMap(t, param["schema"]))
}

// Test_OpenAPI_ConstraintSchemaOptionalParameter asserts the constraint survives
// the optional-parameter fork, which clones the walk state per variant.
func Test_OpenAPI_ConstraintSchemaOptionalParameter(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		app.Get("/pages/:page<int>?", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	})

	paths := requireMap(t, spec["paths"])
	op := requireMap(t, requireMap(t, paths["/pages/{page}"])["get"])
	params, ok := op["parameters"].([]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{"type": "integer"}, requireMap(t, requireMap(t, params[0])["schema"]))

	// The variant that omits the optional parameter declares none.
	bare := requireMap(t, requireMap(t, paths["/pages"])["get"])
	require.NotContains(t, bare, "parameters")
}

// Test_OpenAPI_EscapedConstraintDelimiter asserts an escaped '>' does not close
// the span early, which used to leak constraint text into the path.
func Test_OpenAPI_EscapedConstraintDelimiter(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		app.Get(`/a/:v<regex(^a\>b$)>/tail`, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	})

	paths := requireMap(t, spec["paths"])
	require.Contains(t, paths, "/a/{v}/tail")
	require.Len(t, paths, 1)

	op := requireMap(t, requireMap(t, paths["/a/{v}/tail"])["get"])
	params, ok := op["parameters"].([]any)
	require.True(t, ok)
	require.Len(t, params, 1)
	require.Equal(t, "v", requireMap(t, params[0])["name"])
}

// Test_OpenAPI_SelfHostedSwaggerAssets locks the documented offline behavior:
// all three URLs overridden leaves no outbound request; empty means default.
func Test_OpenAPI_SelfHostedSwaggerAssets(t *testing.T) {
	t.Parallel()

	t.Run("all three overridden leaves no external URL", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Use(New(Config{
			SwaggerCSSURL:              "/swagger-ui/swagger-ui.css",
			SwaggerBundleURL:           "/swagger-ui/swagger-ui-bundle.js",
			SwaggerStandalonePresetURL: "/swagger-ui/swagger-ui-standalone-preset.js",
		}))

		status, body := specBodyOf(t, app, "/swagger")
		require.Equal(t, fiber.StatusOK, status)
		require.NotContains(t, body, "://")
		require.Contains(t, body, `href="/swagger-ui/swagger-ui.css"`)
		require.Contains(t, body, `src="/swagger-ui/swagger-ui-bundle.js"`)
		require.Contains(t, body, `src="/swagger-ui/swagger-ui-standalone-preset.js"`)
	})

	t.Run("an omitted asset URL falls back to the default", func(t *testing.T) {
		t.Parallel()

		app := fiber.New()
		app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Use(New(Config{
			SwaggerCSSURL:    "/swagger-ui/swagger-ui.css",
			SwaggerBundleURL: "/swagger-ui/swagger-ui-bundle.js",
			// Left empty on purpose: this is the offline footgun.
		}))

		_, body := specBodyOf(t, app, "/swagger")
		require.Contains(t, body, ConfigDefault.SwaggerStandalonePresetURL)
	})
}

// Test_OpenAPI_CanonicalPathAdoptsParamNames asserts an operation folded onto an
// equivalent template adopts its names, or the document is invalid.
func Test_OpenAPI_CanonicalPathAdoptsParamNames(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		app.Get("/files/:id", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Post("/files/:name", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	})

	paths := requireMap(t, spec["paths"])
	require.Contains(t, paths, "/files/{id}")
	require.NotContains(t, paths, "/files/{name}")

	item := requireMap(t, paths["/files/{id}"])
	for _, method := range []string{"get", "post"} {
		op := requireMap(t, item[method])
		params, ok := op["parameters"].([]any)
		require.Truef(t, ok, "%s has no parameters", method)
		require.Lenf(t, params, 1, "%s", method)
		require.Equalf(t, "id", requireMap(t, params[0])["name"], "%s declares the canonical name", method)
	}
}

// Test_OpenAPI_ServerVariableEnumDetached asserts the config detaches server
// variable enum slices from the caller, which maps.Clone alone did not do.
func Test_OpenAPI_ServerVariableEnumDetached(t *testing.T) {
	t.Parallel()

	enum := []string{"eu", "us"}
	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(New(Config{
		Servers: []Server{{
			URL:       "https://{region}.example.com",
			Variables: map[string]ServerVariable{"region": {Default: "eu", Enum: enum}},
		}},
	}))

	// Mutating the caller's slice after New must not reach the served document.
	enum[0] = "mutated"

	_, body := specBodyOf(t, app, "/openapi.json")
	require.Contains(t, body, `"eu"`)
	require.NotContains(t, body, "mutated")
}

// Test_AdoptCanonicalParamNames covers the rename directly, including constraint
// keys and the guard a hierarchy match makes unreachable in practice.
func Test_AdoptCanonicalParamNames(t *testing.T) {
	t.Parallel()

	t.Run("renames names, constraints and aliases", func(t *testing.T) {
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
	})

	t.Run("a count mismatch leaves the variant alone", func(t *testing.T) {
		t.Parallel()

		variant := pathVariant{
			ParamNames:       []string{"a", "b"},
			ParamConstraints: map[string]string{"a": "int"},
		}
		adoptCanonicalParamNames(&variant, []string{"x"})

		require.Equal(t, []string{"a", "b"}, variant.ParamNames)
		require.Equal(t, map[string]string{"a": "int"}, variant.ParamConstraints)
	})
}

// Test_OpenAPI_CanonicalPathKeepsConstraintSchema asserts the adopted names
// keep their constraint-derived schemas through the rename.
func Test_OpenAPI_CanonicalPathKeepsConstraintSchema(t *testing.T) {
	t.Parallel()

	spec := fetchSpecWithConfig(t, Config{}, func(app *fiber.App) {
		app.Get("/items/:id", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Post("/items/:code<int>", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	})

	item := requireMap(t, requireMap(t, spec["paths"])["/items/{id}"])
	op := requireMap(t, item["post"])
	params, ok := op["parameters"].([]any)
	require.True(t, ok)

	param := requireMap(t, params[0])
	require.Equal(t, "id", param["name"])
	// The constraint traveled with the rename.
	require.Equal(t, map[string]any{"type": "integer"}, requireMap(t, param["schema"]))
}

// Test_OpenAPI_DynamicMountPrefix covers a mount whose segment count varies;
// truncating at the first variable segment answered 404 on served paths.
func Test_OpenAPI_DynamicMountPrefix(t *testing.T) {
	t.Parallel()

	serve := func(t *testing.T, mount, request string) int {
		t.Helper()
		app := fiber.New()
		app.Get("/hello", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Use(mount, New())
		status, _ := specBodyOf(t, app, request)
		return status
	}

	cases := []struct {
		name    string
		mount   string
		request string
		status  int
	}{
		{"optional segment present", "/:tenant?", "/acme/openapi.json", fiber.StatusOK},
		{"optional segment omitted", "/:tenant?", "/openapi.json", fiber.StatusOK},
		{"optional segment serves the ui too", "/:tenant?", "/acme/swagger", fiber.StatusOK},
		// One optional segment cannot consume two, so this is not a target.
		{"beyond the segment bound", "/:tenant?", "/a/b/openapi.json", fiber.StatusNotFound},
		{"required parameter", "/:tenant", "/acme/openapi.json", fiber.StatusOK},
		{"optional between literals", "/api/:tenant?/docs", "/api/acme/docs/openapi.json", fiber.StatusOK},
		{"greedy segment", "/files/*", "/files/a/b/openapi.json", fiber.StatusOK},
		// Under a greedy mount every split is tried and none is a target.
		{"greedy segment with no target", "/files/*", "/files/a/b", fiber.StatusNotFound},
		// No split of a "+" mount leaves a target here, so resolution reports
		// nothing and the static truncation ("/files") still serves it.
		{"greedy segment falls back to the static prefix", "/files/+", "/files/openapi.json", fiber.StatusOK},
		// Static mounts keep their exact-path behavior.
		{"static mount", "/v1", "/v1/openapi.json", fiber.StatusOK},
		{"static mount is not global", "/v1", "/openapi.json", fiber.StatusNotFound},
		{"global mount", "/", "/openapi.json", fiber.StatusOK},
		{"global mount ignores nested lookalikes", "/", "/x/openapi.json", fiber.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.status, serve(t, tc.mount, tc.request))
		})
	}
}

// Test_PrefixSegmentBounds pins how many request segments each mount shape can
// consume; -1 means a greedy segment leaves it unbounded.
func Test_PrefixSegmentBounds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		pattern string
		lowest  int
		highest int
		dynamic bool
	}{
		{"/", 0, 0, false},
		{"/v1", 1, 1, false},
		{"/v1/api", 2, 2, false},
		{"/:tenant", 1, 1, true},
		{"/:tenant?", 0, 1, true},
		{"/api/:tenant?/docs", 2, 3, true},
		{"/files/*", 1, -1, true},
		{"/files/+", 2, -1, true},
		// A constraint is not a parameter marker of its own.
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

// Test_PathPrefixSegments covers the split helper, including asking for more
// segments than the path has.
func Test_PathPrefixSegments(t *testing.T) {
	t.Parallel()

	prefix, ok := pathPrefixSegments("/acme/openapi.json", 1)
	require.True(t, ok)
	require.Equal(t, "/acme", prefix)

	prefix, ok = pathPrefixSegments("/a/b/c", 2)
	require.True(t, ok)
	require.Equal(t, "/a/b", prefix)

	prefix, ok = pathPrefixSegments("/acme/openapi.json", 0)
	require.True(t, ok)
	require.Empty(t, prefix)

	_, ok = pathPrefixSegments("/acme/openapi.json", 2)
	require.False(t, ok)

	// An empty path has no segment to give.
	_, ok = pathPrefixSegments("", 1)
	require.False(t, ok)

	require.Equal(t, 0, countPathSegments("/"))
	require.Equal(t, 1, countPathSegments("/a"))
	require.Equal(t, 3, countPathSegments("/a/b/c"))
}
