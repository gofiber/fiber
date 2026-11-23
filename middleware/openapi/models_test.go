package openapi

import (
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type modelAddress struct {
	City string `json:"city" validate:"required"`
	Zip  string `json:"zip,omitempty" validate:"len=5"`
}

type modelUser struct {
	Boss  *modelUser   `json:"boss,omitempty"`
	Home  modelAddress `json:"home"`
	Email string       `json:"email" validate:"required,email"`
	Role  string       `json:"role" validate:"oneof=admin user"`
	Tags  []string     `json:"tags" validate:"max=5"`
	ID    int          `json:"id"`
	Age   int          `json:"age" validate:"gte=18,lte=120"`
	Score float64      `json:"score" validate:"min=0.5"`
}

type modelPaging struct {
	Page    int `query:"page" validate:"gte=1" openapi:"description:Page number,example:2"`
	PerPage int `query:"per_page,omitempty"`
}

type modelFilter struct {
	Q      string
	Sort   string `query:"-"`
	hidden string //nolint:unused // exercises the unexported-field skip
	modelPaging
}

type modelHeaders struct {
	TraceID string `header:"X-Trace-Id" validate:"required"`
}

type modelPathIDs struct {
	ID int `uri:"id" openapi:"description:User identifier"`
}

// fetchSpecBody requests the spec from an app that mounts the middleware and returns the raw document.
func fetchSpecBody(t *testing.T, app *fiber.App) []byte {
	t.Helper()
	resp := apptest.Expect(t, app, "/openapi.json", fiber.StatusOK)
	body := apptest.BodyBytes(t, resp)
	require.NoError(t, resp.Body.Close())
	return body
}

// fetchJSON decodes the spec of an app that already mounts the middleware.
func fetchJSON(t *testing.T, app *fiber.App) map[string]any {
	t.Helper()
	var spec map[string]any
	require.NoError(t, json.Unmarshal(fetchSpecBody(t, app), &spec))
	return spec
}

// modelSpec mounts the middleware on app, fetches the spec and checks the document is a valid one.
func modelSpec(t *testing.T, app *fiber.App, cfg ...Config) map[string]any {
	t.Helper()
	app.Use(New(cfg...))
	return mountedSpec(t, app)
}

// mountedSpec fetches and validates the document of an app that already mounts the middleware.
func mountedSpec(t *testing.T, app *fiber.App) map[string]any {
	t.Helper()
	body := fetchSpecBody(t, app)
	validateOpenAPIDocument(t, body)
	var spec map[string]any
	require.NoError(t, json.Unmarshal(body, &spec))
	return spec
}

// rfSpec registers routes on a fresh app and returns its validated document.
func rfSpec(t *testing.T, register func(app *fiber.App), cfg ...Config) map[string]any {
	t.Helper()
	app := fiber.New()
	register(app)
	return modelSpec(t, app, cfg...)
}

func modelOperation(t *testing.T, spec map[string]any, path, method string) map[string]any {
	t.Helper()
	return requireMap(t, requireMap(t, requireMap(t, spec["paths"])[path])[method])
}

// rfResp returns the response an operation documents for status.
func rfResp(t *testing.T, spec map[string]any, path, method, status string) map[string]any {
	t.Helper()
	return requireMap(t, chainResponses(t, modelOperation(t, spec, path, method))[status])
}

func modelSchemas(t *testing.T, spec map[string]any) map[string]any {
	t.Helper()
	return requireMap(t, requireMap(t, spec["components"])["schemas"])
}

func contentSchema(t *testing.T, holder map[string]any) map[string]any {
	t.Helper()
	return requireMap(t, requireMap(t, requireMap(t, holder["content"])[fiber.MIMEApplicationJSON])["schema"])
}

// rfKeys returns the keys of m, for comparing the exact set a document holds.
func rfKeys[V any](m map[string]V) []string { return slices.Collect(maps.Keys(m)) }

// componentRef is the Reference Object that points at a registered component schema.
func componentRef(name string) map[string]any {
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

func Test_OpenAPI_ModelsBecomeComponents(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Post("/users", listUsers).Accepts(modelUser{}).Returns(fiber.StatusCreated, modelUser{})
	})
	body := requireMap(t, modelOperation(t, spec, "/users", "post")["requestBody"])
	require.Equal(t, true, body["required"])
	require.Equal(t, componentRef("modelUser"), contentSchema(t, body))
	created := rfResp(t, spec, "/users", "post", "201")
	require.Equal(t, "Created", created["description"])
	require.Equal(t, componentRef("modelUser"), contentSchema(t, created))

	schemas := modelSchemas(t, spec)
	user := requireMap(t, schemas["modelUser"])
	props := requireMap(t, user["properties"])
	require.Equal(t, componentRef("modelAddress"), props["home"])
	require.Equal(t, map[string]any{"anyOf": []any{componentRef("modelUser"), map[string]any{"type": "null"}}}, props["boss"], "a pointer may be null")
	require.ElementsMatch(t, []any{"email", "role", "tags", "home", "id", "age", "score"}, user["required"])
	require.Equal(t, "email", requireMap(t, props["email"])["format"])
	require.Equal(t, map[string]any{"type": "integer", "minimum": 18.0, "maximum": 120.0}, props["age"])
	require.InDelta(t, 0.5, requireMap(t, props["score"])["minimum"], 0)
	require.InDelta(t, 5, requireMap(t, props["tags"])["maxItems"], 0)
	require.Equal(t, []any{"admin", "user"}, requireMap(t, props["role"])["enum"])

	address := requireMap(t, schemas["modelAddress"])
	require.Equal(t, []any{"city"}, address["required"])
	require.Equal(t, map[string]any{"type": "string", "minLength": 5.0, "maxLength": 5.0}, requireMap(t, address["properties"])["zip"])
}

func Test_OpenAPI_ReturnsSliceNilAndAnonymous(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/users", listUsers).Returns(fiber.StatusOK, []modelUser{})
		app.Delete("/users/:id", listUsers).Returns(fiber.StatusNoContent, nil)
		app.Get("/health", listUsers).Returns(fiber.StatusOK, struct {
			OK bool `json:"ok"`
		}{})
	})
	require.Equal(t, map[string]any{"type": "array", "items": componentRef("modelUser")}, contentSchema(t, rfResp(t, spec, "/users", "get", "200")))
	gone := rfResp(t, spec, "/users/{id}", "delete", "204")
	require.Equal(t, "No Content", gone["description"])
	require.NotContains(t, gone, "content")
	require.Equal(t, map[string]any{
		"type":       "object",
		"properties": map[string]any{"ok": map[string]any{"type": "boolean"}},
		"required":   []any{"ok"},
	}, contentSchema(t, rfResp(t, spec, "/health", "get", "200")))
	require.NotContains(t, modelSchemas(t, spec), "")
	require.Len(t, modelSchemas(t, spec), 2)
}

func Test_OpenAPI_ComponentsKeepUserSchemas(t *testing.T) {
	t.Parallel()
	accepts := func(app *fiber.App) { app.Post("/users", listUsers).Accepts(modelAddress{}) }
	t.Run("a taken name is qualified by package", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, accepts, Config{Components: map[string]any{"schemas": map[string]any{"modelAddress": map[string]any{"type": "string"}}}})
		schemas := modelSchemas(t, spec)
		require.Equal(t, map[string]any{"type": "string"}, schemas["modelAddress"])
		require.Equal(t, "object", requireMap(t, schemas["openapi.modelAddress"])["type"])
		body := requireMap(t, modelOperation(t, spec, "/users", "post")["requestBody"])
		require.Equal(t, componentRef("openapi.modelAddress"), contentSchema(t, body))
	})

	t.Run("a taken qualified name is numbered", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, accepts, Config{Components: map[string]any{"schemas": map[string]any{
			"modelAddress":         map[string]any{"type": "string"},
			"openapi.modelAddress": map[string]any{"type": "integer"},
		}}})
		require.Contains(t, modelSchemas(t, spec), "modelAddress_2")
	})
}

func Test_OpenAPI_ParamsFromModel(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/users/:id", listUsers).Params("query", modelFilter{}).Params("header", &modelHeaders{}).Params("path", modelPathIDs{})
	})
	params := chainParams(t, modelOperation(t, spec, "/users/{id}", "get"))
	require.Len(t, params, 5)

	require.Equal(t, map[string]any{
		"name": "page", "in": "query", "description": "Page number", "example": 2.0, "required": false,
		"schema": map[string]any{"type": "integer", "minimum": 1.0},
	}, params["query:page"])
	require.Equal(t, "integer", requireMap(t, params["query:per_page"]["schema"])["type"])
	require.Equal(t, "string", requireMap(t, params["query:Q"]["schema"])["type"])
	require.Equal(t, true, params["header:X-Trace-Id"]["required"])
	require.Equal(t, map[string]any{"name": "id", "in": "path", "required": true, "description": "User identifier", "schema": map[string]any{"type": "integer"}}, params["path:id"])
}

func Test_OpenAPI_ParamsExplicitOverride(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/users", listUsers).Params("query", modelPaging{}).Parameter("page", "query", true, map[string]any{"type": "string"}, "Override")
	})
	var pages []map[string]any
	for _, raw := range requireSlice(t, modelOperation(t, spec, "/users", "get")["parameters"]) {
		if p := requireMap(t, raw); p["name"] == "page" {
			pages = append(pages, p)
		}
	}
	require.Equal(t, []map[string]any{{"name": "page", "in": "query", "required": true, "description": "Override", "schema": map[string]any{"type": "string"}}}, pages)
}

func Test_OpenAPI_HeaderSchemaModel(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/users", listUsers).ResponseHeader(fiber.StatusOK, "X-Meta", "Meta", modelAddress{})
	})
	header := requireMap(t, chainHeaders(t, rfResp(t, spec, "/users", "get", "200"))["X-Meta"])
	require.Equal(t, componentRef("modelAddress"), header["schema"])
	require.Contains(t, modelSchemas(t, spec), "modelAddress")
}

func Test_SchemaOf_ValidateTags(t *testing.T) {
	t.Parallel()
	props := requireMap(t, SchemaOf(modelUser{})["properties"])
	require.Equal(t, "object", requireMap(t, props["home"])["type"])
	require.Equal(t, "email", requireMap(t, props["email"])["format"])
	require.Equal(t, int64(18), requireMap(t, props["age"])["minimum"])
	require.InDelta(t, 0.5, requireMap(t, props["score"])["minimum"], 0)
	require.Equal(t, int64(5), requireMap(t, props["tags"])["maxItems"])

	type formats struct {
		J *string `json:"j" validate:"required"`
		A string  `json:"a" validate:"uuid4"`
		B string  `json:"b" validate:"url"`
		C string  `json:"c" validate:"ipv6"`
		D string  `json:"d" validate:"base64"`
		E string  `json:"e" validate:"datetime=2006-01-02T15:04:05Z07:00"`
		F string  `json:"f" validate:"datetime=2006-01-02"`
		G string  `json:"g" validate:"email" openapi:"format:idn-email"`
		H string  `json:"h" validate:"min=abc"`
		K string  `json:"k,omitempty" validate:"required"`
		I bool    `json:"i" validate:"min=1"`
	}
	props = requireMap(t, SchemaOf(formats{})["properties"])
	for name, want := range map[string]string{"a": "uuid", "b": "uri", "c": "ipv6", "d": "byte", "e": "date-time", "g": "idn-email"} {
		require.Equal(t, want, requireMap(t, props[name])["format"], name)
	}
	require.NotContains(t, requireMap(t, props["f"]), "format")
	require.NotContains(t, requireMap(t, props["h"]), "minLength")
	require.NotContains(t, requireMap(t, props["i"]), "minimum")
	require.ElementsMatch(t, []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}, SchemaOf(formats{})["required"])
}

func Test_componentName(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"Page[github.com/x/y.User]":      "Page_User",
		"Pair[main.A,main.B]":            "Pair_A_B",
		"Page[[]main.User]":              "Page_Array_User", // a slice argument keeps its shape
		"Page[main.Pair[main.A,main.B]]": "Page_Pair_A_B",
		"Plain":                          "Plain",
		"User":                           "User",
		"":                               "_",
	} {
		require.Equal(t, want, componentName(in), in)
	}
}

func Test_keyName(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"Page[pkg.User]": "Page_pkg.User_", "a-b_c.d": "a-b_c.d", "é": "_", "": "", "日本語": "___"} {
		require.Equal(t, want, keyName(in), in)
	}
}

func Test_schemaRegistry_NilResolvesInline(t *testing.T) {
	t.Parallel()
	var reg *schemaRegistry
	require.Nil(t, reg.resolve(nil))
	require.Nil(t, reg.resolve(map[string]any{}))
	require.Equal(t, map[string]any{"type": "string"}, reg.resolve(map[string]any{"type": "string"}))
	require.Equal(t, "object", reg.resolve(modelAddress{})["type"])
	require.Nil(t, reg.componentSchemas())
}

func Test_expandParameterModels_SkipsWhatIsNotAStruct(t *testing.T) {
	t.Parallel()
	require.Empty(t, expandParameterModels([]fiber.RouteParameterModel{{In: "query", Model: nil}, {In: "query", Model: 42}, {In: "query", Model: "text"}}, nil))
	require.Len(t, expandParameterModels([]fiber.RouteParameterModel{{In: "query", Model: &modelPaging{}}}, nil), 2)
}

func Test_isDefaultStringSchema(t *testing.T) {
	t.Parallel()
	require.True(t, isDefaultStringSchema(nil))
	require.True(t, isDefaultStringSchema(map[string]any{"type": "string"}))
	require.False(t, isDefaultStringSchema(map[string]any{"type": "integer"}))
	require.False(t, isDefaultStringSchema(modelAddress{}))
}

func Test_resolveHeaderSchemas(t *testing.T) {
	t.Parallel()
	require.Nil(t, resolveHeaderSchemas(nil, nil))
	headers := map[string]any{
		"plain":  "not an object",
		"mapped": map[string]any{"schema": map[string]any{"type": "string"}},
		"model":  map[string]any{"description": "d", "schema": modelAddress{}},
	}
	resolved := resolveHeaderSchemas(headers, nil)
	require.Equal(t, "not an object", resolved["plain"])
	require.Equal(t, headers["mapped"], resolved["mapped"])
	model := requireMap(t, resolved["model"])
	require.Equal(t, "d", model["description"])
	require.Equal(t, "object", requireMap(t, model["schema"])["type"])
	require.Equal(t, modelAddress{}, requireMap(t, headers["model"])["schema"], "the caller's map still holds the Go value")
}

type orderNode struct {
	Name string      `json:"name"`
	Kids []orderNode `json:"kids"`
}

type orderPage[T any] struct {
	Items []T `json:"items"`
}

func Test_OpenAPI_ComponentsDoNotDependOnRegistrationOrder(t *testing.T) {
	t.Parallel()
	build := func(first, second any) map[string]any {
		spec := rfSpec(t, func(app *fiber.App) {
			app.Post("/a", listUsers).Accepts(first)
			app.Post("/b", listUsers).Accepts(second)
		})
		return requireMap(t, modelSchemas(t, spec)["orderNode"])
	}
	pageFirst := build(orderPage[orderNode]{}, orderNode{})
	require.Equal(t, build(orderNode{}, orderPage[orderNode]{}), pageFirst)
	require.Equal(t, map[string]any{"type": "array", "items": componentRef("orderNode")}, requireMap(t, requireMap(t, pageFirst["properties"])["kids"]))
}

type refSiblingInner struct {
	ID int `json:"id" openapi:"example:7"`
}

type refSiblingOuter struct {
	Inner refSiblingInner `json:"inner" openapi:"description:the inner,readOnly"`
}

func Test_OpenAPI_RefSiblingsAreWrappedBefore31(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"3.0.0", "3.0.3", "3.1.0", "3.2.0"} {
		spec := rfSpec(t, func(app *fiber.App) { app.Post("/x", listUsers).Accepts(refSiblingOuter{}) }, Config{OpenAPIVersion: version})
		got := requireMap(t, requireMap(t, requireMap(t, modelSchemas(t, spec)["refSiblingOuter"])["properties"])["inner"])
		require.Equal(t, "the inner", got["description"], version)
		if strings.HasPrefix(version, "3.0") {
			require.NotContains(t, got, "$ref", version)
			require.Equal(t, []any{componentRef("refSiblingInner")}, got["allOf"], version)
			require.Equal(t, true, got["readOnly"], version)
		} else {
			require.Equal(t, "#/components/schemas/refSiblingInner", got["$ref"], version)
		}
	}
}

type uploadForm struct {
	Name  string `json:"name" form:"display_name"`
	Skip  string `json:"skip" form:"-"`
	Plain string
	Count int `json:"count" form:"count" validate:"required"`
}

func Test_OpenAPI_FormBodiesReadFormTags(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Post("/json", listUsers).Accepts(uploadForm{}, fiber.MIMEApplicationJSON)
		app.Post("/form", listUsers).Accepts(uploadForm{}, fiber.MIMEMultipartForm, fiber.MIMEApplicationForm)
	})
	body := func(path, mediaType string) map[string]any {
		content := requireMap(t, requireMap(t, modelOperation(t, spec, path, "post")["requestBody"])["content"])
		return requireMap(t, requireMap(t, content[mediaType])["schema"])
	}
	schemas := modelSchemas(t, spec)
	require.Equal(t, componentRef("uploadForm"), body("/json", fiber.MIMEApplicationJSON))
	require.Equal(t, componentRef("uploadForm_form"), body("/form", fiber.MIMEMultipartForm))
	require.Equal(t, componentRef("uploadForm_form"), body("/form", fiber.MIMEApplicationForm))

	require.ElementsMatch(t, []string{"name", "skip", "Plain", "count"}, rfKeys(requireMap(t, requireMap(t, schemas["uploadForm"])["properties"])))
	formProps := requireMap(t, requireMap(t, schemas["uploadForm_form"])["properties"])
	require.ElementsMatch(t, []string{"display_name", "count", "Plain"}, rfKeys(formProps), "a field with no form tag binds by its Go name")
	require.Equal(t, []any{"count"}, requireMap(t, schemas["uploadForm_form"])["required"])
}

func Test_OpenAPI_BodylessStatusesCarryNoContent(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/g", listUsers).Returns(fiber.StatusNoContent, uploadForm{}).
			Response(fiber.StatusNotModified, "unchanged", fiber.MIMEApplicationJSON).Returns(fiber.StatusOK, uploadForm{})
	})
	require.NotContains(t, rfResp(t, spec, "/g", "get", "204"), "content")
	require.NotContains(t, rfResp(t, spec, "/g", "get", "304"), "content")
	require.Contains(t, rfResp(t, spec, "/g", "get", "200"), "content")
}

type nullableModel struct {
	Count  *int            `json:"count"`
	Name   *string         `json:"name" openapi:"description:optional name"`
	Kind   *string         `json:"kind" validate:"oneof=a b"`
	Inner  *nullableInner  `json:"inner" openapi:"description:the inner"`
	Quoted *int            `json:"quoted,string"`
	Items  []nullableInner `json:"items"`
	Plain  int             `json:"plain"`
}

type nullableInner struct {
	ID int `json:"id"`
}

func Test_OpenAPI_PointerFieldsAreNullable(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"3.1.0", "3.2.0", "3.0.0", "3.0.3"} {
		spec := rfSpec(t, func(app *fiber.App) { app.Post("/x", listUsers).Accepts(nullableModel{}) }, Config{OpenAPIVersion: version})
		got := requireMap(t, requireMap(t, modelSchemas(t, spec)["nullableModel"])["properties"])
		prop := func(name string) map[string]any { return requireMap(t, got[name]) }
		inner := prop("inner")
		require.Equal(t, []any{"a", "b", nil}, prop("kind")["enum"], version)
		require.Equal(t, "the inner", inner["description"], version)
		if strings.HasPrefix(version, "3.0") {
			require.Equal(t, "integer", prop("count")["type"], version)
			require.Equal(t, true, prop("count")["nullable"], version)
			require.Equal(t, "string", prop("quoted")["type"], version)
			require.Equal(t, true, prop("quoted")["nullable"], version)
			require.Equal(t, []any{componentRef("nullableInner"), map[string]any{"type": "object", "nullable": true, "enum": []any{nil}}}, inner["anyOf"], version)
			require.NotContains(t, inner, "nullable", version, "3.0 defines nullable only next to a type")
			require.NotContains(t, prop("plain"), "nullable", version)
			continue
		}
		require.Equal(t, []any{"integer", "null"}, prop("count")["type"], version)
		require.Equal(t, []any{"string", "null"}, prop("name")["type"], version)
		require.Equal(t, "optional name", prop("name")["description"], version)
		require.Equal(t, []any{componentRef("nullableInner"), map[string]any{"type": "null"}}, inner["anyOf"], version)
		require.Equal(t, "integer", prop("plain")["type"], version, "a non-pointer is never null")
		require.Equal(t, []any{"string", "null"}, prop("quoted")["type"], version, "a quoted number is written as a string, null when nil")
		require.Equal(t, "array", prop("items")["type"], version, "a slice is not made nullable")
	}
}

type exampleInner struct {
	Name string `json:"name" openapi:"example:bob"`
}

type exampleOuter struct {
	Pointer *exampleInner `json:"pointer"`
	Plain   exampleInner  `json:"plain" openapi:"description:the plain one"`
}

// Test_OpenAPI_NullableReferenceKeepsNestedExample checks that a struct pointer, whose reference nullable wraps
// in an anyOf, still lends its model's example to the object example in every dialect.
func Test_OpenAPI_NullableReferenceKeepsNestedExample(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"3.0.3", "3.1.0", "3.2.0"} {
		spec := rfSpec(t, func(app *fiber.App) { app.Post("/x", listUsers).Accepts(exampleOuter{}) }, Config{OpenAPIVersion: version})
		want := map[string]any{"name": "bob"}
		require.Equal(t, map[string]any{"pointer": want, "plain": want}, requireMap(t, modelSchemas(t, spec)["exampleOuter"])["example"], version)
	}
}

func sameNamedOK() any {
	type Response struct {
		Data string `json:"data"`
	}
	return Response{}
}

func sameNamedError() any {
	type Response struct {
		Message string `json:"message"`
	}
	return Response{}
}

// Test_OpenAPI_SameNamedModelsKeepTheirNames checks that two models sharing a type name get the same component
// names in every generation: responses are walked in order, so map order cannot hand the bare name to either.
func Test_OpenAPI_SameNamedModelsKeepTheirNames(t *testing.T) {
	t.Parallel()
	for range 50 {
		spec := rfSpec(t, func(app *fiber.App) {
			app.Get("/x", listUsers).Returns(fiber.StatusOK, sameNamedOK()).Returns(fiber.StatusBadRequest, sameNamedError())
		})
		require.Equal(t, componentRef("Response"), contentSchema(t, rfResp(t, spec, "/x", "get", "200")))
		require.Equal(t, componentRef("openapi.Response"), contentSchema(t, rfResp(t, spec, "/x", "get", "400")))
	}
}

type mixedLocations struct {
	Trace  string `header:"X-Trace" validate:"required"`
	Cookie string `cookie:"sid"`
	Free   string
	Page   int `query:"page"`
	ID     int `uri:"id"`
}

func Test_OpenAPI_ParamsSkipFieldsTaggedForOtherLocations(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/q/:id", listUsers).Params(fiber.ParamInQuery, mixedLocations{}).Params(fiber.ParamInHeader, mixedLocations{}).Params(fiber.ParamInCookie, mixedLocations{})
	})
	params := chainParams(t, modelOperation(t, spec, "/q/{id}", "get"))
	// An untagged field binds by name from any source.
	require.ElementsMatch(t, []string{"path:id", "query:page", "query:Free", "header:X-Trace", "header:Free", "cookie:sid", "cookie:Free"}, rfKeys(params))
	require.Equal(t, true, params["header:X-Trace"]["required"])
}

// boundedTimeout is how long apptest.Bounded waits before calling a deadlock.
const boundedTimeout = 10 * time.Second

type exclusiveBounds struct {
	Name  string  `json:"name" validate:"gt=2,lt=9"`
	Items []int   `json:"items" validate:"gt=1"`
	Count int     `json:"count" validate:"gt=0,lt=10"`
	Price float64 `json:"price" validate:"gt=0,lt=10"`
}

func Test_SchemaOf_ExclusiveBounds(t *testing.T) {
	t.Parallel()
	props := requireMap(t, SchemaOf(exclusiveBounds{})["properties"])
	require.Equal(t, map[string]any{"type": "string", "minLength": int64(3), "maxLength": int64(8)}, props["name"])
	require.Equal(t, map[string]any{"type": "integer", "minimum": int64(1), "maximum": int64(9)}, props["count"])
	require.Equal(t, map[string]any{"type": "number"}, props["price"])
	require.EqualValues(t, 2, requireMap(t, props["items"])["minItems"])
}

func Test_OpenAPI_EmptyResponseDescriptionFallsBack(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error { return c.SendString("x") }).
		Response(fiber.StatusNotFound, "", fiber.MIMEApplicationJSON).
		Response(0, "", fiber.MIMEApplicationJSON)
	app.Use(New())
	responses := fetchSpec(t, app).Paths["/x"]["get"].Responses
	require.Equal(t, "Not Found", responses["404"].Description)
	for code, resp := range responses {
		require.NotEmpty(t, resp.Description, code)
	}
}

func Test_OpenAPI_ParameterSchemaKeepsItsOwnType(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/p", listUsers).
			AddParameter(fiber.RouteParameter{Name: "either", In: fiber.ParamInQuery, Schema: map[string]any{
				"oneOf": []any{map[string]any{"type": "integer"}, map[string]any{"type": "string"}},
			}}).
			AddParameter(fiber.RouteParameter{Name: "ints", In: fiber.ParamInQuery, Schema: map[string]any{"enum": []int{1, 2}}})
	})
	params := chainParams(t, modelOperation(t, spec, "/p", "get"))
	require.NotContains(t, requireMap(t, params["query:either"]["schema"]), "type")
	require.Equal(t, "integer", requireMap(t, params["query:ints"]["schema"])["type"])
}

type refParamAddress struct {
	City string `json:"city"`
}

type refParamModel struct {
	Home refParamAddress `query:"home" openapi:"format:custom"`
}

func Test_OpenAPI_ParamsModelRefSiblings(t *testing.T) {
	t.Parallel()
	for version, wrapped := range map[string]bool{"3.0.3": true, "3.1.0": false} {
		spec := rfSpec(t, func(app *fiber.App) { app.Get("/p", listUsers).Params(fiber.ParamInQuery, refParamModel{}) }, Config{OpenAPIVersion: version})
		params := requireSlice(t, modelOperation(t, spec, "/p", "get")["parameters"])
		require.Len(t, params, 1, version)
		schema := requireMap(t, requireMap(t, params[0])["schema"])
		require.Equal(t, !wrapped, schema["$ref"] != nil, version)
		require.Equal(t, wrapped, schema["allOf"] != nil, version)
		require.Equal(t, "custom", schema["format"], version)
	}
}

func Test_OpenAPI_DefinedMapTypesAreRawSchemas(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/x", listUsers).
			Returns(fiber.StatusOK, fiber.Map{"type": "object", "properties": fiber.Map{"id": fiber.Map{"type": "integer"}}}).
			Parameter("q", fiber.ParamInQuery, false, fiber.Map{"type": "integer"}, "")
	}, Config{ErrorSchema: fiber.Map{"type": "object", "properties": fiber.Map{"error": fiber.Map{"type": "string"}}}})
	schema := contentSchema(t, rfResp(t, spec, "/x", "get", "200"))
	require.Equal(t, "object", schema["type"])
	require.NotContains(t, schema, "additionalProperties", "the map's Go type must not be reflected")
	require.Contains(t, requireMap(t, schema["properties"]), "id")
	require.Equal(t, "integer", requireMap(t, chainParams(t, modelOperation(t, spec, "/x", "get"))["query:q"]["schema"])["type"])
}

type orphanInner struct {
	V string `json:"v"`
}

type orphanFilter struct {
	Inner orphanInner `query:"inner"`
}

// Test_OpenAPI_DroppedDuplicatesRegisterNothing checks that a route dropped as a duplicate (the same template up to
// parameter names, so the first registration wins) leaves no component schema or security scheme behind.
func Test_OpenAPI_DroppedDuplicatesRegisterNothing(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/a/:id", listUsers)
		app.Get("/a/:name", listUsers).Params(fiber.ParamInQuery, orphanFilter{})
	})
	if components, ok := spec["components"].(map[string]any); ok {
		require.NotContains(t, components, "schemas")
	}

	spec = rfSpec(t, func(app *fiber.App) {
		app.Get("/a/:id", chainBasicAuth(), listUsers)
		app.Get("/a/:name", chainKeyAuth(), listUsers)
	})
	schemes := requireMap(t, requireMap(t, spec["components"])["securitySchemes"])
	require.Contains(t, schemes, securitySchemeBasic)
	require.NotContains(t, schemes, securitySchemeBearer)
}

type userRequest struct {
	Q     string `query:"q"`
	Token string `header:"X-Token"`
	Name  string `json:"name" validate:"required"`
	ID    int    `uri:"id"`
}

type userResponse struct {
	Echo string `json:"echo"`
}

type searchRequest struct {
	Q string `query:"q"`
}

// rfJSONPost builds a JSON POST request to target.
func rfJSONPost(target, body string) *http.Request {
	req := httptest.NewRequest(fiber.MethodPost, target, strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return req
}

func Test_OpenAPI_TypedRegistersAndDocuments(t *testing.T) {
	t.Parallel()
	var order []string
	app := fiber.New()
	Typed(app, fiber.MethodPost, "/users/:id", func(c fiber.Ctx, req *userRequest) (*userResponse, error) {
		order = append(order, "handler")
		return &userResponse{Echo: strings.Join([]string{c.Params("id"), req.Q, req.Token, req.Name}, "|")}, nil
	}, WithStatus(fiber.StatusCreated), WithMiddleware(func(c fiber.Ctx) error {
		order = append(order, "middleware")
		return c.Next()
	})).Summary("Create a user")
	Typed(app, fiber.MethodGet, "/search", func(_ fiber.Ctx, req *searchRequest) (*Empty, error) {
		if req.Q == "boom" {
			return nil, errors.New("boom")
		}
		return &Empty{}, nil
	})
	Typed(app, fiber.MethodGet, "/nothing", func(_ fiber.Ctx, _ *searchRequest) (*userResponse, error) {
		return nil, nil //nolint:nilnil // a nil response is the case under test
	})

	req := rfJSONPost("/users/7?q=x", `{"name":"ann"}`)
	req.Header.Set("X-Token", "tok")
	require.JSONEq(t, `{"echo":"7|x|tok|ann"}`, apptest.Body(t, apptest.Expect(t, app, req, fiber.StatusCreated)))
	require.Equal(t, []string{"middleware", "handler"}, order)
	apptest.Expect(t, app, "/search?q=ok", fiber.StatusNoContent)
	apptest.Expect(t, app, "/search?q=boom", fiber.StatusInternalServerError)
	apptest.Expect(t, app, "/nothing", fiber.StatusOK)

	spec := modelSpec(t, app)
	post := modelOperation(t, spec, "/users/{id}", "post")
	require.Equal(t, "Create a user", post["summary"])
	params := chainParams(t, post)
	for _, key := range []string{"path:id", "query:q", "header:X-Token"} {
		require.Contains(t, params, key)
	}
	require.Equal(t, componentRef("userRequest_request"), contentSchema(t, requireMap(t, post["requestBody"])))
	bodySchema := requireMap(t, modelSchemas(t, spec)["userRequest_request"])
	require.Equal(t, map[string]any{"name": map[string]any{"type": "string"}}, bodySchema["properties"], "a parameter is not part of the body")
	require.Equal(t, []any{"name"}, bodySchema["required"])
	require.Equal(t, componentRef("userResponse"), contentSchema(t, rfResp(t, spec, "/users/{id}", "post", "201")))

	search := modelOperation(t, spec, "/search", "get")
	require.Contains(t, chainParams(t, search), "query:q")
	require.NotContains(t, search, "requestBody", "a request with no body fields has no body")
	require.NotContains(t, rfResp(t, spec, "/search", "get", "204"), "content")
}

type plainBody struct {
	Name string `json:"name"`
}

func Test_OpenAPI_RequestBodiesLeaveOutParameters(t *testing.T) {
	t.Parallel()
	schemas := modelSchemas(t, rfSpec(t, func(app *fiber.App) {
		app.Post("/users/:id", listUsers).Accepts(userRequest{}).Returns(fiber.StatusOK, userRequest{})
		app.Post("/plain", listUsers).Accepts(plainBody{}).Returns(fiber.StatusOK, plainBody{})
	}))
	require.Contains(t, requireMap(t, requireMap(t, schemas["userRequest"])["properties"]), "ID", "a response keeps every field")
	require.NotContains(t, requireMap(t, requireMap(t, schemas["userRequest_request"])["properties"]), "ID")
	require.Contains(t, schemas, "plainBody")
	require.NotContains(t, schemas, "plainBody_request", "a struct without parameter fields is described once")
}

func Test_OpenAPI_TypedDocumentsAnEmptyRouteWithoutABody(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Typed(app, fiber.MethodDelete, "/things/:id", func(_ fiber.Ctx, _ *struct {
		ID int `uri:"id"`
	},
	) (*Empty, error) {
		return &Empty{}, nil
	})
	spec := modelSpec(t, app)
	op := modelOperation(t, spec, "/things/{id}", "delete")
	require.NotContains(t, op, "requestBody")
	require.Contains(t, chainResponses(t, op), "204")
	require.Contains(t, chainParams(t, op), "path:id")
	apptest.Expect(t, app, httptest.NewRequest(fiber.MethodDelete, "/things/3", http.NoBody), fiber.StatusNoContent)
	_, err := json.Marshal(spec)
	require.NoError(t, err)
}

func Test_OpenAPI_TypedDeclaresParametersInAFixedOrder(t *testing.T) {
	t.Parallel()
	for range 20 {
		app := fiber.New()
		Typed(app, fiber.MethodPost, "/users/:id", func(_ fiber.Ctx, _ *userRequest) (*userResponse, error) { return &userResponse{}, nil })
		var in []fiber.ParamLocation
		for _, route := range app.GetRoutes() {
			if route.Path != "/users/:id" {
				continue
			}
			for i := range route.ParameterModels {
				in = append(in, route.ParameterModels[i].In)
			}
			break
		}
		require.Equal(t, []fiber.ParamLocation{fiber.ParamInPath, fiber.ParamInQuery, fiber.ParamInHeader}, in)
	}
}

type strictRequest struct {
	Name    string `json:"name"`
	Plain   string
	Token   string `header:"X-Token"`
	Page    int    `query:"page"`
	ID      int    `uri:"id"`
	IsAdmin bool   `json:"is_admin"`
}

type strictEcho struct {
	Name    string `json:"name"`
	Plain   string `json:"plain"`
	Token   string `json:"token"`
	Page    int    `json:"page"`
	ID      int    `json:"id"`
	IsAdmin bool   `json:"is_admin"`
}

func echoStrict(_ fiber.Ctx, req *strictRequest) (*strictEcho, error) {
	return &strictEcho{Name: req.Name, Plain: req.Plain, Token: req.Token, Page: req.Page, ID: req.ID, IsAdmin: req.IsAdmin}, nil
}

func Test_OpenAPI_TypedBindsEachFieldOnlyFromItsSource(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	Typed(app, fiber.MethodPost, "/u/:id", echoStrict)
	send := func(target, body string) *strictEcho {
		req := rfJSONPost(target, body)
		req.Header.Set("X-Token", "tok")
		req.Header.Set("IsAdmin", "true")
		req.Header.Set("Is_admin", "true")
		req.Header.Set(fiber.HeaderCookie, "IsAdmin=true; is_admin=true; Plain=cookie; Name=cookie")
		var got strictEcho
		require.NoError(t, json.Unmarshal(apptest.BodyBytes(t, apptest.Expect(t, app, req, fiber.StatusOK)), &got))
		return &got
	}
	got := send("/u/7?page=3&IsAdmin=true&is_admin=true&Plain=query&Name=query", `{"name":"ann"}`)
	require.Equal(t, &strictEcho{Name: "ann", Token: "tok", Page: 3, ID: 7}, got, "a body field is not filled from the query, a header or a cookie")
	got = send("/u/7", `{"name":"ann","is_admin":true,"Plain":"body","Token":"forged","Page":9,"ID":99}`)
	require.Equal(t, &strictEcho{Name: "ann", Plain: "body", Token: "tok", ID: 7, IsAdmin: true}, got, "a parameter is not filled from the body")
	apptest.Expect(t, app, rfJSONPost("/u/1", `{"name":`), fiber.StatusBadRequest)
}

func benchApp(routes int) *fiber.App {
	app := fiber.New(fiber.Config{StructValidator: stubValidator{}})
	users := app.Group("/users", chainKeyAuth())
	for i := range routes {
		path := "/r" + strconv.Itoa(i) + "/:id"
		route := users.Get(path, listUsers)
		if i%2 == 0 {
			route.Returns(fiber.StatusOK, modelUser{}).Params("query", modelPaging{})
		}
	}
	users.Post("/create", listUsers).Accepts(modelUser{}).Returns(fiber.StatusCreated, modelUser{})
	return app
}

func Benchmark_OpenAPI_GenerateSpec(b *testing.B) {
	app := benchApp(200)
	cfg := configDefault()
	env := specEnv{validator: stubValidator{}, equal: stringsEqual}
	routes := app.GetRoutes(false)
	b.ReportAllocs()
	var spec openAPISpec
	for b.Loop() {
		spec = generateSpec(routes, &cfg, env)
	}
	require.NotEmpty(b, spec.Paths)
}

func Benchmark_OpenAPI_SchemaOf(b *testing.B) {
	b.ReportAllocs()
	var schema map[string]any
	for b.Loop() {
		schema = SchemaOf(modelUser{})
	}
	require.Equal(b, "object", schema["type"])
}

func Benchmark_OpenAPI_summaryFromFuncName(b *testing.B) {
	b.ReportAllocs()
	var summary string
	for b.Loop() {
		summary = summaryFromFuncName("github.com/acme/api/handlers.(*Server).GetUserByID-fm")
	}
	require.Equal(b, "Get user by ID", summary)
}

func Benchmark_OpenAPI_coversRoute(b *testing.B) {
	b.ReportAllocs()
	var covered bool
	for b.Loop() {
		covered = coversRoute("/users/:id", "/users/:id/posts/:post", stringsEqual)
	}
	require.True(b, covered)
}

// Benchmark_OpenAPI_PassThrough measures a request the middleware lets through, the cost every route pays when
// it is registered with Use.
func Benchmark_OpenAPI_PassThrough(b *testing.B) {
	for name, cfg := range map[string]*Config{"prefixed": {}, "rootPath": {Path: "/", UIPath: "/swagger"}} {
		b.Run(name, func(b *testing.B) {
			app := fiber.New()
			app.Use(New(*cfg))
			app.Get("/users", listUsers)
			handler := app.Handler()
			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(fiber.MethodGet)
			fctx.Request.SetRequestURI("/users")

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				handler(fctx)
			}
		})
	}
}
