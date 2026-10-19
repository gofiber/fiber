package openapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
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
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
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
	body := fetchSpecBody(t, app)
	validateOpenAPIDocument(t, body)
	var spec map[string]any
	require.NoError(t, json.Unmarshal(body, &spec))
	return spec
}

func modelOperation(t *testing.T, spec map[string]any, path, method string) map[string]any {
	t.Helper()
	return requireMap(t, requireMap(t, requireMap(t, spec["paths"])[path])[method])
}

func modelSchemas(t *testing.T, spec map[string]any) map[string]any {
	t.Helper()
	return requireMap(t, requireMap(t, spec["components"])["schemas"])
}

func contentSchema(t *testing.T, holder map[string]any) map[string]any {
	t.Helper()
	return requireMap(t, requireMap(t, requireMap(t, holder["content"])[fiber.MIMEApplicationJSON])["schema"])
}

// componentRef is the Reference Object that points at a registered component schema.
func componentRef(name string) map[string]any {
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

func Test_OpenAPI_ModelsBecomeComponents(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Post("/users", listUsers).Accepts(modelUser{}).Returns(fiber.StatusCreated, modelUser{})
	spec := modelSpec(t, app)
	op := modelOperation(t, spec, "/users", "post")

	body := requireMap(t, op["requestBody"])
	require.Equal(t, true, body["required"])
	require.Equal(t, componentRef("modelUser"), contentSchema(t, body))
	created := requireMap(t, requireMap(t, op["responses"])["201"])
	require.Equal(t, "Created", created["description"])
	require.Equal(t, componentRef("modelUser"), contentSchema(t, created))

	schemas := modelSchemas(t, spec)
	user := requireMap(t, schemas["modelUser"])
	props := requireMap(t, user["properties"])
	require.Equal(t, componentRef("modelAddress"), props["home"])
	require.Equal(t, map[string]any{"anyOf": []any{componentRef("modelUser"), map[string]any{"type": "null"}}}, props["boss"], "a pointer may be null")
	require.ElementsMatch(t, []any{"email", "role", "tags", "home", "id", "age", "score"}, user["required"])

	email := requireMap(t, props["email"])
	require.Equal(t, "email", email["format"])
	age := requireMap(t, props["age"])
	require.InDelta(t, 18, age["minimum"], 0)
	require.InDelta(t, 120, age["maximum"], 0)
	require.InDelta(t, 0.5, requireMap(t, props["score"])["minimum"], 0)
	require.InDelta(t, 5, requireMap(t, props["tags"])["maxItems"], 0)
	require.Equal(t, []any{"admin", "user"}, requireMap(t, props["role"])["enum"])

	address := requireMap(t, schemas["modelAddress"])
	require.Equal(t, []any{"city"}, address["required"])
	zip := requireMap(t, requireMap(t, address["properties"])["zip"])
	require.InDelta(t, 5, zip["minLength"], 0)
	require.InDelta(t, 5, zip["maxLength"], 0)
}

func Test_OpenAPI_ReturnsSliceNilAndAnonymous(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", listUsers).Returns(fiber.StatusOK, []modelUser{})
	app.Delete("/users/:id", listUsers).Returns(fiber.StatusNoContent, nil)
	app.Get("/health", listUsers).Returns(fiber.StatusOK, struct {
		OK bool `json:"ok"`
	}{})
	spec := modelSpec(t, app)

	list := requireMap(t, requireMap(t, modelOperation(t, spec, "/users", "get")["responses"])["200"])
	require.Equal(t, map[string]any{"type": "array", "items": componentRef("modelUser")}, contentSchema(t, list))

	gone := requireMap(t, requireMap(t, modelOperation(t, spec, "/users/{id}", "delete")["responses"])["204"])
	require.Equal(t, "No Content", gone["description"])
	require.NotContains(t, gone, "content")

	health := requireMap(t, requireMap(t, modelOperation(t, spec, "/health", "get")["responses"])["200"])
	require.Equal(t, map[string]any{
		"type":       "object",
		"properties": map[string]any{"ok": map[string]any{"type": "boolean"}},
		"required":   []any{"ok"},
	}, contentSchema(t, health))
	require.NotContains(t, modelSchemas(t, spec), "")
	require.Len(t, modelSchemas(t, spec), 2)
}

func Test_OpenAPI_ComponentsKeepUserSchemas(t *testing.T) {
	t.Parallel()

	t.Run("a taken name is qualified by package", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Post("/users", listUsers).Accepts(modelAddress{})
		spec := modelSpec(t, app, Config{Components: map[string]any{
			"schemas": map[string]any{"modelAddress": map[string]any{"type": "string"}},
		}})
		schemas := modelSchemas(t, spec)
		require.Equal(t, map[string]any{"type": "string"}, schemas["modelAddress"])
		require.Equal(t, "object", requireMap(t, schemas["openapi.modelAddress"])["type"])
		body := requireMap(t, modelOperation(t, spec, "/users", "post")["requestBody"])
		require.Equal(t, componentRef("openapi.modelAddress"), contentSchema(t, body))
	})

	t.Run("a taken qualified name is numbered", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Post("/users", listUsers).Accepts(modelAddress{})
		spec := modelSpec(t, app, Config{Components: map[string]any{
			"schemas": map[string]any{
				"modelAddress":         map[string]any{"type": "string"},
				"openapi.modelAddress": map[string]any{"type": "integer"},
			},
		}})
		require.Contains(t, modelSchemas(t, spec), "modelAddress_2")
	})
}

func Test_OpenAPI_ParamsFromModel(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users/:id", listUsers).
		Params("query", modelFilter{}).
		Params("header", &modelHeaders{}).
		Params("path", modelPathIDs{})
	spec := modelSpec(t, app)
	op := modelOperation(t, spec, "/users/{id}", "get")

	params := map[string]map[string]any{}
	for _, raw := range requireSlice(t, op["parameters"]) {
		p := requireMap(t, raw)
		params[requireString(t, p["in"])+":"+requireString(t, p["name"])] = p
	}
	require.Len(t, params, 5)

	page := params["query:page"]
	require.Equal(t, "Page number", page["description"])
	require.InDelta(t, 2, page["example"], 0)
	require.Equal(t, false, page["required"])
	pageSchema := requireMap(t, page["schema"])
	require.Equal(t, "integer", pageSchema["type"])
	require.InDelta(t, 1, pageSchema["minimum"], 0)
	require.NotContains(t, pageSchema, "description")

	require.Equal(t, "integer", requireMap(t, params["query:per_page"]["schema"])["type"])
	require.Equal(t, "string", requireMap(t, params["query:Q"]["schema"])["type"])

	trace := params["header:X-Trace-Id"]
	require.Equal(t, true, trace["required"])

	id := params["path:id"]
	require.Equal(t, true, id["required"])
	require.Equal(t, "User identifier", id["description"])
	require.Equal(t, "integer", requireMap(t, id["schema"])["type"])
}

func Test_OpenAPI_ParamsExplicitOverride(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", listUsers).
		Params("query", modelPaging{}).
		Parameter("page", "query", true, map[string]any{"type": "string"}, "Override")
	op := modelOperation(t, modelSpec(t, app), "/users", "get")

	var page map[string]any
	for _, raw := range requireSlice(t, op["parameters"]) {
		if p := requireMap(t, raw); p["name"] == "page" {
			require.Nil(t, page)
			page = p
		}
	}
	require.Equal(t, "Override", page["description"])
	require.Equal(t, true, page["required"])
	require.Equal(t, map[string]any{"type": "string"}, page["schema"])
}

func Test_OpenAPI_DefaultConsumes(t *testing.T) {
	t.Parallel()

	t.Run("declared body without a media type documents JSON", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Post("/users", listUsers).RequestBody("payload", true)
		body := requireMap(t, modelOperation(t, modelSpec(t, app), "/users", "post")["requestBody"])
		require.Contains(t, requireMap(t, body["content"]), fiber.MIMEApplicationJSON)
	})

	t.Run("configured default", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Post("/users", listUsers).Accepts(modelAddress{})
		spec := modelSpec(t, app, Config{DefaultConsumes: fiber.MIMETextPlain})
		body := requireMap(t, modelOperation(t, spec, "/users", "post")["requestBody"])
		content := requireMap(t, body["content"])
		require.Contains(t, content, fiber.MIMETextPlain)
		require.Len(t, content, 1)
	})

	t.Run("explicit media type wins", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Post("/users", listUsers).Accepts(modelAddress{}, fiber.MIMEApplicationXML)
		body := requireMap(t, modelOperation(t, modelSpec(t, app), "/users", "post")["requestBody"])
		content := requireMap(t, body["content"])
		require.Contains(t, content, fiber.MIMEApplicationXML)
		require.Len(t, content, 1)
	})
}

func Test_OpenAPI_HeaderSchemaModel(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/users", listUsers).ResponseHeader(fiber.StatusOK, "X-Meta", "Meta", modelAddress{})
	spec := modelSpec(t, app)
	resp := requireMap(t, requireMap(t, modelOperation(t, spec, "/users", "get")["responses"])["200"])
	header := requireMap(t, requireMap(t, resp["headers"])["X-Meta"])
	require.Equal(t, componentRef("modelAddress"), header["schema"])
	require.Contains(t, modelSchemas(t, spec), "modelAddress")
}

func Test_SchemaOf_ValidateTags(t *testing.T) {
	t.Parallel()

	schema := SchemaOf(modelUser{})
	props := requireMap(t, schema["properties"])
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
	require.Equal(t, "uuid", requireMap(t, props["a"])["format"])
	require.Equal(t, "uri", requireMap(t, props["b"])["format"])
	require.Equal(t, "ipv6", requireMap(t, props["c"])["format"])
	require.Equal(t, "byte", requireMap(t, props["d"])["format"])
	require.Equal(t, "date-time", requireMap(t, props["e"])["format"])
	require.NotContains(t, requireMap(t, props["f"]), "format")
	require.Equal(t, "idn-email", requireMap(t, props["g"])["format"])
	require.NotContains(t, requireMap(t, props["h"]), "minLength")
	require.NotContains(t, requireMap(t, props["i"]), "minimum")
	require.ElementsMatch(t, []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}, SchemaOf(formats{})["required"])
}

func Test_componentName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "Page_User", componentName("Page[github.com/x/y.User]"))
	require.Equal(t, "Pair_A_B", componentName("Pair[main.A,main.B]"))
	require.Equal(t, "Page_Array_User", componentName("Page[[]main.User]"), "a slice argument keeps its shape")
	require.Equal(t, "Page_Pair_A_B", componentName("Page[main.Pair[main.A,main.B]]"))
	require.Equal(t, "Plain", componentName("Plain"))
	require.Equal(t, "User", componentName("User"))
	require.Equal(t, "_", componentName(""))
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

	require.Empty(t, expandParameterModels([]fiber.RouteParameterModel{
		{In: "query", Model: nil},
		{In: "query", Model: 42},
		{In: "query", Model: "text"},
	}, nil))

	params := expandParameterModels([]fiber.RouteParameterModel{{In: "query", Model: &modelPaging{}}}, nil)
	require.Len(t, params, 2)
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
	model, ok := resolved["model"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "d", model["description"])
	require.Equal(t, "object", requireMap(t, model["schema"])["type"])
	// The caller's map still holds the Go value.
	require.Equal(t, modelAddress{}, headers["model"].(map[string]any)["schema"]) //nolint:forcetypeassert,errcheck // test fixture shape
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
		app := fiber.New()
		app.Post("/a", listUsers).Accepts(first)
		app.Post("/b", listUsers).Accepts(second)
		schemas := requireMap(t, requireMap(t, modelSpec(t, app)["components"])["schemas"])
		return requireMap(t, schemas["orderNode"])
	}

	nodeFirst := build(orderNode{}, orderPage[orderNode]{})
	pageFirst := build(orderPage[orderNode]{}, orderNode{})
	require.Equal(t, nodeFirst, pageFirst)

	kids := requireMap(t, requireMap(t, pageFirst["properties"])["kids"])
	require.Equal(t, "array", kids["type"])
	require.Equal(t, componentRef("orderNode"), kids["items"])
}

type refSiblingInner struct {
	ID int `json:"id" openapi:"example:7"`
}

type refSiblingOuter struct {
	Inner refSiblingInner `json:"inner" openapi:"description:the inner,readOnly"`
}

func Test_OpenAPI_RefSiblingsAreWrappedBefore31(t *testing.T) {
	t.Parallel()

	inner := func(version string) map[string]any {
		app := fiber.New()
		app.Post("/x", listUsers).Accepts(refSiblingOuter{})
		schemas := requireMap(t, requireMap(t, modelSpec(t, app, Config{OpenAPIVersion: version})["components"])["schemas"])
		return requireMap(t, requireMap(t, schemas["refSiblingOuter"])["properties"])["inner"].(map[string]any) //nolint:errcheck,forcetypeassert // map built by the generator
	}

	for _, version := range []string{"3.0.0", "3.0.3"} {
		got := inner(version)
		require.NotContains(t, got, "$ref", version)
		require.Equal(t, []any{map[string]any{"$ref": "#/components/schemas/refSiblingInner"}}, got["allOf"], version)
		require.Equal(t, "the inner", got["description"], version)
		require.Equal(t, true, got["readOnly"], version)
	}
	for _, version := range []string{"3.1.0", "3.2.0"} {
		got := inner(version)
		require.Equal(t, "#/components/schemas/refSiblingInner", got["$ref"], version)
		require.Equal(t, "the inner", got["description"], version)
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

	app := fiber.New()
	app.Post("/json", listUsers).Accepts(uploadForm{}, fiber.MIMEApplicationJSON)
	app.Post("/form", listUsers).Accepts(uploadForm{}, fiber.MIMEMultipartForm, fiber.MIMEApplicationForm)
	spec := modelSpec(t, app)

	body := func(path, mediaType string) map[string]any {
		content := requireMap(t, requireMap(t, modelOperation(t, spec, path, "post")["requestBody"])["content"])
		return requireMap(t, requireMap(t, content[mediaType])["schema"])
	}
	schemas := requireMap(t, requireMap(t, spec["components"])["schemas"])

	require.Equal(t, componentRef("uploadForm"), body("/json", fiber.MIMEApplicationJSON))
	require.Equal(t, componentRef("uploadForm_form"), body("/form", fiber.MIMEMultipartForm))
	require.Equal(t, componentRef("uploadForm_form"), body("/form", fiber.MIMEApplicationForm))

	jsonProps := requireMap(t, requireMap(t, schemas["uploadForm"])["properties"])
	require.Contains(t, jsonProps, "name")
	require.Contains(t, jsonProps, "skip")

	formProps := requireMap(t, requireMap(t, schemas["uploadForm_form"])["properties"])
	require.Contains(t, formProps, "display_name")
	require.Contains(t, formProps, "count")
	require.Contains(t, formProps, "Plain", "a field with no form tag binds by its Go name")
	require.NotContains(t, formProps, "name")
	require.NotContains(t, formProps, "skip")
	require.Equal(t, []any{"count"}, requireMap(t, schemas["uploadForm_form"])["required"])
}

func Test_OpenAPI_BodylessStatusesCarryNoContent(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/g", listUsers).
		Returns(fiber.StatusNoContent, uploadForm{}).
		Response(fiber.StatusNotModified, "unchanged", fiber.MIMEApplicationJSON).
		Returns(fiber.StatusOK, uploadForm{})
	responses := chainResponses(t, modelOperation(t, modelSpec(t, app), "/g", "get"))

	require.NotContains(t, requireMap(t, responses["204"]), "content")
	require.NotContains(t, requireMap(t, responses["304"]), "content")
	require.Contains(t, requireMap(t, responses["200"]), "content")
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

	props := func(version string) map[string]any {
		app := fiber.New()
		app.Post("/x", listUsers).Accepts(nullableModel{})
		schemas := modelSchemas(t, modelSpec(t, app, Config{OpenAPIVersion: version}))
		return requireMap(t, requireMap(t, schemas["nullableModel"])["properties"])
	}

	for _, version := range []string{"3.1.0", "3.2.0"} {
		got := props(version)
		require.Equal(t, []any{"integer", "null"}, requireMap(t, got["count"])["type"], version)
		require.Equal(t, []any{"string", "null"}, requireMap(t, got["name"])["type"], version)
		require.Equal(t, "optional name", requireMap(t, got["name"])["description"], version)
		require.Equal(t, []any{"a", "b", nil}, requireMap(t, got["kind"])["enum"], version)
		inner := requireMap(t, got["inner"])
		require.Equal(t, []any{componentRef("nullableInner"), map[string]any{"type": "null"}}, inner["anyOf"], version)
		require.Equal(t, "the inner", inner["description"], version)
		require.Equal(t, "integer", requireMap(t, got["plain"])["type"], version, "a non-pointer is never null")
		require.Equal(t, []any{"string", "null"}, requireMap(t, got["quoted"])["type"], version, "a quoted number is written as a string, null when nil")
		require.Equal(t, "array", requireMap(t, got["items"])["type"], version, "a slice is not made nullable")
	}

	for _, version := range []string{"3.0.0", "3.0.3"} {
		got := props(version)
		require.Equal(t, "integer", requireMap(t, got["count"])["type"], version)
		require.Equal(t, true, requireMap(t, got["count"])["nullable"], version)
		require.Equal(t, "string", requireMap(t, got["quoted"])["type"], version)
		require.Equal(t, true, requireMap(t, got["quoted"])["nullable"], version)
		require.Equal(t, []any{"a", "b", nil}, requireMap(t, got["kind"])["enum"], version)
		inner := requireMap(t, got["inner"])
		require.Equal(t, []any{componentRef("nullableInner")}, inner["allOf"], version)
		require.Equal(t, true, inner["nullable"], version)
		require.NotContains(t, requireMap(t, got["plain"]), "nullable", version)
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

	app := fiber.New()
	app.Get("/q/:id", listUsers).
		Params(fiber.ParamInQuery, mixedLocations{}).
		Params(fiber.ParamInHeader, mixedLocations{}).
		Params(fiber.ParamInCookie, mixedLocations{})
	params := chainParams(t, modelOperation(t, modelSpec(t, app), "/q/{id}", "get"))

	require.Contains(t, params, "query:page")
	require.Contains(t, params, "query:Free", "an untagged field binds by name from any source")
	require.NotContains(t, params, "query:Trace")
	require.NotContains(t, params, "query:sid")
	require.Contains(t, params, "header:X-Trace")
	require.Equal(t, true, params["header:X-Trace"]["required"])
	require.NotContains(t, params, "header:page")
	require.Contains(t, params, "cookie:sid")
	require.NotContains(t, params, "cookie:X-Trace")
}

// boundedTimeout is how long runBounded waits before calling a hang.
const boundedTimeout = 10 * time.Second

// runBounded runs fn and fails the test, instead of hanging it, when fn does not return in time or panics.
func runBounded(t *testing.T, what string, fn func()) {
	t.Helper()
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		fn()
	}()
	select {
	case recovered := <-done:
		if recovered != nil {
			t.Fatalf("%s panicked: %v", what, recovered)
		}
	case <-time.After(boundedTimeout):
		t.Fatalf("%s did not return within %v", what, boundedTimeout)
	}
}

type exclusiveBounds struct {
	Name  string  `json:"name" validate:"gt=2,lt=9"`
	Items []int   `json:"items" validate:"gt=1"`
	Count int     `json:"count" validate:"gt=0,lt=10"`
	Price float64 `json:"price" validate:"gt=0,lt=10"`
}

func Test_SchemaOf_ExclusiveBounds(t *testing.T) {
	t.Parallel()
	props := requireMap(t, SchemaOf(exclusiveBounds{})["properties"])

	name := requireMap(t, props["name"])
	require.EqualValues(t, 3, name["minLength"])
	require.EqualValues(t, 8, name["maxLength"])

	count := requireMap(t, props["count"])
	require.EqualValues(t, 1, count["minimum"])
	require.EqualValues(t, 9, count["maximum"])

	price := requireMap(t, props["price"])
	require.NotContains(t, price, "minimum")
	require.NotContains(t, price, "maximum")

	require.EqualValues(t, 2, requireMap(t, props["items"])["minItems"])
}

func Test_OpenAPI_EmptyResponseDescriptionFallsBack(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error { return c.SendString("x") }).
		Response(fiber.StatusNotFound, "", fiber.MIMEApplicationJSON).
		Response(0, "", fiber.MIMEApplicationJSON)
	app.Use(New())

	spec := fetchSpec(t, app)
	responses := spec.Paths["/x"]["get"].Responses
	require.Equal(t, "Not Found", responses["404"].Description)
	for code, resp := range responses {
		require.NotEmpty(t, resp.Description, code)
	}
}

func Test_OpenAPI_ParameterSchemaKeepsItsOwnType(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/p", listUsers).
		AddParameter(fiber.RouteParameter{Name: "either", In: fiber.ParamInQuery, Schema: map[string]any{
			"oneOf": []any{map[string]any{"type": "integer"}, map[string]any{"type": "string"}},
		}}).
		AddParameter(fiber.RouteParameter{Name: "ints", In: fiber.ParamInQuery, Schema: map[string]any{"enum": []int{1, 2}}})
	spec := modelSpec(t, app)

	op := modelOperation(t, spec, "/p", "get")
	params, ok := op["parameters"].([]any)
	require.True(t, ok)
	byName := map[string]map[string]any{}
	for _, p := range params {
		param := requireMap(t, p)
		name, isString := param["name"].(string)
		require.True(t, isString)
		byName[name] = requireMap(t, param["schema"])
	}
	require.NotContains(t, byName["either"], "type")
	require.Equal(t, "integer", byName["ints"]["type"])
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
		app := fiber.New()
		app.Get("/p", listUsers).Params(fiber.ParamInQuery, refParamModel{})
		spec := modelSpec(t, app, Config{OpenAPIVersion: version})

		params, ok := modelOperation(t, spec, "/p", "get")["parameters"].([]any)
		require.True(t, ok, version)
		require.Len(t, params, 1, version)
		schema := requireMap(t, requireMap(t, params[0])["schema"])
		if wrapped {
			require.NotContains(t, schema, "$ref", version)
			require.Contains(t, schema, "allOf", version)
		} else {
			require.Contains(t, schema, "$ref", version)
		}
		require.Equal(t, "custom", schema["format"], version)
	}
}

func Test_OpenAPI_DefinedMapTypesAreRawSchemas(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/x", listUsers).
		Returns(fiber.StatusOK, fiber.Map{"type": "object", "properties": fiber.Map{"id": fiber.Map{"type": "integer"}}}).
		Parameter("q", fiber.ParamInQuery, false, fiber.Map{"type": "integer"}, "")
	spec := modelSpec(t, app, Config{ErrorSchema: fiber.Map{"type": "object", "properties": fiber.Map{"error": fiber.Map{"type": "string"}}}})

	op := modelOperation(t, spec, "/x", "get")
	ok := requireMap(t, requireMap(t, chainResponses(t, op)["200"])["content"])
	schema := requireMap(t, requireMap(t, ok[fiber.MIMEApplicationJSON])["schema"])
	require.Equal(t, "object", schema["type"])
	require.NotContains(t, schema, "additionalProperties", "the map's Go type must not be reflected")
	require.Contains(t, requireMap(t, schema["properties"]), "id")

	params, isList := op["parameters"].([]any)
	require.True(t, isList)
	require.Equal(t, "integer", requireMap(t, requireMap(t, params[0])["schema"])["type"])
}
