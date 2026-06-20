package openapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// validOperationMethods is the set of OpenAPI Path Item operation keys.
var validOperationMethods = map[string]struct{}{
	"get": {}, "put": {}, "post": {}, "delete": {},
	"options": {}, "head": {}, "patch": {}, "trace": {},
}

var validParameterLocations = map[string]struct{}{
	"path": {}, "query": {}, "header": {}, "cookie": {},
}

var pathTemplateRe = regexp.MustCompile(`\{([^}]+)\}`)

// validateOpenAPIDocument asserts that raw is a structurally valid OpenAPI 3.0/3.1
// document for the subset of the specification this middleware emits. It is a
// dependency-free regression guard, not a full meta-schema validator.
func validateOpenAPIDocument(t *testing.T, raw []byte) {
	t.Helper()

	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))

	// openapi version
	version, ok := doc["openapi"].(string)
	require.True(t, ok, "openapi version must be a string")
	require.Contains(t, []string{"3.0.0", "3.1.0"}, version)

	// info
	info, ok := doc["info"].(map[string]any)
	require.True(t, ok, "info must be an object")
	title, ok := info["title"].(string)
	require.True(t, ok && title != "", "info.title must be a non-empty string")
	infoVersion, ok := info["version"].(string)
	require.True(t, ok && infoVersion != "", "info.version must be a non-empty string")

	securitySchemes := documentSecuritySchemes(t, doc)
	validateDocumentSecurity(t, doc, securitySchemes)

	paths, ok := doc["paths"].(map[string]any)
	require.True(t, ok, "paths must be an object")

	operationIDs := make(map[string]struct{})
	for pathKey, rawItem := range paths {
		require.Truef(t, strings.HasPrefix(pathKey, "/"), "path %q must start with /", pathKey)
		item, ok := rawItem.(map[string]any)
		require.Truef(t, ok, "path item %q must be an object", pathKey)

		templateParams := pathTemplateNames(pathKey)
		for method, rawOp := range item {
			_, known := validOperationMethods[method]
			require.Truef(t, known, "path %q has invalid operation %q", pathKey, method)
			op, ok := rawOp.(map[string]any)
			require.Truef(t, ok, "operation %s %s must be an object", method, pathKey)

			validateOperation(t, method, pathKey, op, templateParams, securitySchemes, operationIDs)
		}
	}
}

func validateOperation(t *testing.T, method, pathKey string, op map[string]any, templateParams, securitySchemes, operationIDs map[string]struct{}) {
	t.Helper()

	// operationId uniqueness
	if rawID, present := op["operationId"]; present {
		id, ok := rawID.(string)
		require.Truef(t, ok && id != "", "%s %s operationId must be a non-empty string", method, pathKey)
		_, dup := operationIDs[id]
		require.Falsef(t, dup, "duplicate operationId %q", id)
		operationIDs[id] = struct{}{}
	}

	// responses: required, non-empty, each has a description
	responses, ok := op["responses"].(map[string]any)
	require.Truef(t, ok, "%s %s must have a responses object", method, pathKey)
	require.NotEmptyf(t, responses, "%s %s responses must not be empty", method, pathKey)
	for code, rawResp := range responses {
		require.Truef(t, code == "default" || isThreeDigitStatus(code), "%s %s invalid response key %q", method, pathKey, code)
		resp, ok := rawResp.(map[string]any)
		require.Truef(t, ok, "%s %s response %q must be an object", method, pathKey, code)
		desc, ok := resp["description"].(string)
		require.Truef(t, ok && desc != "", "%s %s response %q needs a description", method, pathKey, code)
	}

	// parameters
	declaredPathParams := validateParameters(t, method, pathKey, op)

	// every {template} in the path must be declared as an in:path parameter
	for name := range templateParams {
		_, declared := declaredPathParams[name]
		require.Truef(t, declared, "%s %s missing path parameter %q for template", method, pathKey, name)
	}

	// requestBody rules
	if rawBody, present := op["requestBody"]; present {
		require.Falsef(t, method == "get" || method == "head", "%s %s must not have a requestBody", method, pathKey)
		body, ok := rawBody.(map[string]any)
		require.Truef(t, ok, "%s %s requestBody must be an object", method, pathKey)
		content, ok := body["content"].(map[string]any)
		require.Truef(t, ok && len(content) > 0, "%s %s requestBody.content must be a non-empty object", method, pathKey)
	}

	// per-operation security must reference defined schemes
	validateSecurityRequirements(t, op["security"], securitySchemes, method+" "+pathKey)
}

// validateParameters checks each parameter and returns the set of declared path
// parameter names.
func validateParameters(t *testing.T, method, pathKey string, op map[string]any) map[string]struct{} {
	t.Helper()

	declaredPathParams := make(map[string]struct{})
	rawParams, present := op["parameters"]
	if !present {
		return declaredPathParams
	}
	params, ok := rawParams.([]any)
	require.Truef(t, ok, "%s %s parameters must be an array", method, pathKey)

	seen := make(map[string]struct{})
	for _, rawParam := range params {
		param, ok := rawParam.(map[string]any)
		require.Truef(t, ok, "%s %s parameter must be an object", method, pathKey)

		name, ok := param["name"].(string)
		require.Truef(t, ok && name != "", "%s %s parameter needs a name", method, pathKey)
		in, ok := param["in"].(string)
		require.Truef(t, ok, "%s %s parameter %q needs an 'in'", method, pathKey, name)
		_, validIn := validParameterLocations[in]
		require.Truef(t, validIn, "%s %s parameter %q has invalid 'in' %q", method, pathKey, name, in)

		key := in + ":" + name
		_, dup := seen[key]
		require.Falsef(t, dup, "%s %s duplicate parameter %s", method, pathKey, key)
		seen[key] = struct{}{}

		if in == "path" {
			required, ok := param["required"].(bool)
			require.Truef(t, ok && required, "%s %s path parameter %q must be required", method, pathKey, name)
			declaredPathParams[name] = struct{}{}
		}
	}
	return declaredPathParams
}

func documentSecuritySchemes(t *testing.T, doc map[string]any) map[string]struct{} {
	t.Helper()

	schemes := make(map[string]struct{})
	components, ok := doc["components"].(map[string]any)
	if !ok {
		return schemes
	}
	rawSchemes, ok := components["securitySchemes"].(map[string]any)
	if !ok {
		return schemes
	}
	for name := range rawSchemes {
		schemes[name] = struct{}{}
	}
	return schemes
}

func validateDocumentSecurity(t *testing.T, doc map[string]any, schemes map[string]struct{}) {
	t.Helper()
	validateSecurityRequirements(t, doc["security"], schemes, "document")
}

func validateSecurityRequirements(t *testing.T, rawSecurity any, schemes map[string]struct{}, where string) {
	t.Helper()
	if rawSecurity == nil {
		return
	}
	requirements, ok := rawSecurity.([]any)
	require.Truef(t, ok, "%s security must be an array", where)
	for _, rawReq := range requirements {
		req, ok := rawReq.(map[string]any)
		require.Truef(t, ok, "%s security requirement must be an object", where)
		for scheme := range req {
			_, defined := schemes[scheme]
			require.Truef(t, defined, "%s security references undefined scheme %q", where, scheme)
		}
	}
}

func pathTemplateNames(pathKey string) map[string]struct{} {
	names := make(map[string]struct{})
	for _, match := range pathTemplateRe.FindAllStringSubmatch(pathKey, -1) {
		names[match[1]] = struct{}{}
	}
	return names
}

func isThreeDigitStatus(code string) bool {
	if len(code) != 3 {
		return false
	}
	n, err := strconv.Atoi(code)
	return err == nil && n >= 100 && n <= 599
}

func Test_OpenAPI_GeneratedSpecIsValid(t *testing.T) {
	t.Parallel()

	type User struct {
		Name  string `json:"name"`
		Email string `json:"email" openapi:"format:email,description:User email"`
		ID    int    `json:"id"`
	}

	build := func(version string) []byte {
		app := fiber.New()

		app.Get("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
			Summary("List users").
			Parameter("page", "query", false, map[string]any{"type": "integer"}, "Page number").
			Parameter("X-Trace", "header", false, nil, "Trace id").
			Parameter("session", "cookie", false, nil, "Session cookie").
			Response(fiber.StatusOK, "OK", fiber.MIMEApplicationJSON).
			ResponseHeader(fiber.StatusOK, "X-Rate-Limit", "Requests left", map[string]any{"type": "integer"}).
			Tags("users")

		app.Post("/users", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) }).
			RequestBodyContent("Create user", true, map[string]fiber.RouteMediaType{
				fiber.MIMEApplicationJSON: {Schema: SchemaOf(User{})},
				fiber.MIMEApplicationXML:  {SchemaRef: "#/components/schemas/User"},
			}).
			ResponseContent(fiber.StatusCreated, "Created", map[string]fiber.RouteMediaType{
				fiber.MIMEApplicationJSON: {Schema: SchemaOf(User{}), Encoding: map[string]any{"id": map[string]any{"contentType": "text/plain"}}},
			}).
			ResponseHeader(fiber.StatusCreated, "Location", "Created URL", map[string]any{"type": "string"}).
			ResponseLink(fiber.StatusCreated, "self", map[string]any{"operationId": "getUsersId"}).
			OperationExternalDocs("docs", "https://docs.example.com/create").
			OperationExtension(map[string]any{"servers": []any{map[string]any{"url": "https://op.example.com"}}}).
			Security(map[string][]string{"bearerAuth": {}})

		app.Get("/users/:id", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).
			AddParameter(fiber.RouteParameter{
				Name: "fields", In: "query", Style: "form", Explode: openapiBoolPtr(true),
				Deprecated: true, AllowEmptyValue: true, AllowReserved: true,
				Schema: map[string]any{"type": "array"},
			})
		app.Get("/files/*", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Get("/items/:id?", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		app.Delete("/users/:id", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) }).Deprecated()
		app.Get("/internal", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }).Hidden()

		app.Use(New(Config{
			OpenAPIVersion:    version,
			Title:             "Validation API",
			Version:           "1.2.3",
			Summary:           "Validation API summary",
			JSONSchemaDialect: "https://spec.openapis.org/oas/3.1/dialect/base",
			Servers: []Server{
				{URL: "https://{region}.example.com", Description: "prod", Variables: map[string]ServerVariable{
					"region": {Default: "us", Enum: []string{"us", "eu"}},
				}},
			},
			Tags:         []Tag{{Name: "users", Description: "User ops", ExternalDocs: &ExternalDocs{URL: "https://docs.example.com/users"}}},
			ExternalDocs: &ExternalDocs{Description: "docs", URL: "https://docs.example.com"},
			Webhooks: map[string]any{
				"ping": map[string]any{"post": map[string]any{"responses": map[string]any{"200": map[string]any{"description": "ok"}}}},
			},
			Components: map[string]any{
				"schemas": map[string]any{"User": SchemaOf(User{})},
			},
			SecuritySchemes: map[string]any{
				"bearerAuth": map[string]any{"type": "http", "scheme": "bearer"},
			},
			Security: []map[string][]string{{"bearerAuth": {}}},
		}))

		req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return body
	}

	for _, version := range []string{"3.0.0", "3.1.0"} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()
			validateOpenAPIDocument(t, build(version))
		})
	}
}
