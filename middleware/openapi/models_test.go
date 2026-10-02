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

func modelSpec(t *testing.T, app *fiber.App, cfg ...Config) map[string]any {
	t.Helper()
	app.Use(New(cfg...))
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
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

func ref(name string) map[string]any {
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
	require.Equal(t, ref("modelUser"), contentSchema(t, body))
	created := requireMap(t, requireMap(t, op["responses"])["201"])
	require.Equal(t, "Created", created["description"])
	require.Equal(t, ref("modelUser"), contentSchema(t, created))

	schemas := modelSchemas(t, spec)
	user := requireMap(t, schemas["modelUser"])
	props := requireMap(t, user["properties"])
	require.Equal(t, ref("modelAddress"), props["home"])
	require.Equal(t, ref("modelUser"), props["boss"])
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
	require.Equal(t, map[string]any{"type": "array", "items": ref("modelUser")}, contentSchema(t, list))

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
		require.Equal(t, ref("openapi.modelAddress"), contentSchema(t, body))
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
	require.Equal(t, ref("modelAddress"), header["schema"])
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

	require.Equal(t, "Page_github.com_x_y.User_", componentName("Page[github.com/x/y.User]"))
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
