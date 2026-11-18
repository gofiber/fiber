package openapi

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/idempotency"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/responsetime"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/stretchr/testify/require"
)

type secretModel struct {
	Token string `json:"token"`
}

type publicModel struct {
	Name string `json:"name"`
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

func Test_OpenAPI_FilterNarrowsTheDocument(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Filter: func(route *fiber.Route) bool { return !strings.HasPrefix(route.Path, "/internal") }}))
	app.Use(chainKeyAuth())
	app.Get("/public/pets", listUsers).Returns(fiber.StatusOK, publicModel{})
	app.Get("/internal/keys", listUsers).Returns(fiber.StatusOK, secretModel{}).Security()
	app.Delete("/internal/keys", listUsers)

	spec := mountedSpec(t, app)
	paths := requireMap(t, spec["paths"])
	require.Contains(t, paths, "/public/pets")
	require.NotContains(t, paths, "/internal/keys")
	require.Contains(t, modelOperation(t, spec, "/public/pets", "get"), "security", "a kept route keeps what its middleware infers")
	schemas := modelSchemas(t, spec)
	require.Contains(t, schemas, "publicModel")
	require.NotContains(t, schemas, "secretModel", "a filtered-out route leaves no schema behind")
}

func Test_OpenAPI_FilterReceivesACopy(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/a", listUsers).Summary("kept")
	spec := modelSpec(t, app, Config{Filter: func(route *fiber.Route) bool {
		route.Summary = "changed"
		return true
	}})
	require.Equal(t, "kept", modelOperation(t, spec, "/a", "get")["summary"])
}

func Test_OpenAPI_GenerateMatchesTheServedDocument(t *testing.T) {
	t.Parallel()
	build := func() *fiber.App {
		sub := fiber.New()
		sub.Get("/items", listUsers).Returns(fiber.StatusOK, publicModel{}).Summary("Items")
		app := fiber.New()
		app.Use("/api", sub)
		app.Get("/health", listUsers)
		return app
	}
	cfg := Config{Title: "Mounted", Extensions: map[string]any{"x-team": "api"}}

	generated, err := Generate(build(), cfg)
	require.NoError(t, err)

	served := build()
	served.Use(New(cfg))
	body := apptest.BodyBytes(t, apptest.Expect(t, served, "/openapi.json", fiber.StatusOK))
	require.JSONEq(t, string(body), string(generated))

	var doc map[string]any
	require.NoError(t, json.Unmarshal(generated, &doc))
	require.Contains(t, requireMap(t, doc["paths"]), "/api/items", "a mounted app is documented without being served")

	_, err = Generate(nil)
	require.Error(t, err)
}

func Test_OpenAPI_TransformEditsAFreshCopy(t *testing.T) {
	t.Parallel()
	builds := 0
	app := fiber.New()
	app.Get("/a", listUsers)
	app.Use(New(Config{Transform: func(document map[string]any) error {
		builds++
		info, ok := document["info"].(map[string]any)
		if !ok {
			return errors.New("no info")
		}
		info["x-build"] = builds
		document["x-extra"] = true
		delete(document, "servers")
		return nil
	}}))

	doc := fetchJSON(t, app)
	require.Equal(t, true, doc["x-extra"])
	require.InDelta(t, 1, requireMap(t, doc["info"])["x-build"], 0)

	app.Get("/b", listUsers)
	doc = fetchJSON(t, app)
	require.InDelta(t, 2, requireMap(t, doc["info"])["x-build"], 0, "each build starts from a new document")
}

func Test_OpenAPI_TransformErrorFailsTheRequest(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Transform: func(map[string]any) error { return errors.New("refused") }}))
	apptest.Expect(t, app, "/openapi.json", fiber.StatusInternalServerError)

	app = fiber.New()
	app.Use(New(Config{Transform: func(document map[string]any) error {
		document["bad"] = make(chan int)
		return nil
	}}))
	apptest.Expect(t, app, "/openapi.json", fiber.StatusInternalServerError)
}

func Test_OpenAPI_ExtensionsAreAddedToTheRootAndInfo(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/a", listUsers)
	spec := modelSpec(t, app, Config{
		Extensions:     map[string]any{"x-logo": map[string]any{"url": "https://example.com/logo.png"}},
		InfoExtensions: map[string]any{"x-audience": "internal"},
	})
	require.Equal(t, map[string]any{"url": "https://example.com/logo.png"}, spec["x-logo"])
	require.Equal(t, "internal", requireMap(t, spec["info"])["x-audience"])

	require.Panics(t, func() { New(Config{Extensions: map[string]any{"logo": 1}}) })
	require.Panics(t, func() { New(Config{InfoExtensions: map[string]any{"audience": 1}}) })
}

func Test_OpenAPI_DefaultsDocumentRoutesUnderAPrefix(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/api/v1/pets", listUsers)
	app.Get("/api/v1/owners", listUsers).Tags("own").
		AddParameter(fiber.RouteParameter{Name: "X-Tenant", In: fiber.ParamInHeader, Description: "route says"}).
		Response(fiber.StatusBadRequest, "route 400")
	app.Get("/api/v1x/other", listUsers)
	app.Get("/open", listUsers)

	headers := fiber.RouteParameter{Name: "X-Tenant", In: fiber.ParamInHeader, Description: "default says", Required: true}
	spec := modelSpec(t, app, Config{Defaults: []Defaults{
		{PathPrefix: "/api/v1", Tags: []string{"v1"}, Parameters: []fiber.RouteParameter{headers}, Responses: map[int]fiber.RouteResponse{
			fiber.StatusBadRequest: {Description: "default 400", Schema: publicModel{}},
			fiber.StatusForbidden:  {Description: "forbidden"},
			0:                      {Description: "any other error"},
		}},
		{PathPrefix: "/api", Tags: []string{"api"}, Responses: map[int]fiber.RouteResponse{fiber.StatusForbidden: {Description: "second entry loses"}}},
	}})

	pets := modelOperation(t, spec, "/api/v1/pets", "get")
	require.Equal(t, []any{"v1"}, pets["tags"])
	require.Equal(t, "default says", chainParams(t, pets)["header:X-Tenant"]["description"])
	responses := chainResponses(t, pets)
	require.Equal(t, "default 400", requireMap(t, responses["400"])["description"])
	require.Equal(t, "forbidden", requireMap(t, responses["403"])["description"], "the first matching entry wins")
	require.Contains(t, responses, "default")
	require.Contains(t, responses, "200", "default responses do not replace the automatic success response")

	owners := modelOperation(t, spec, "/api/v1/owners", "get")
	require.Equal(t, []any{"own"}, owners["tags"], "a route's own tags win")
	require.Equal(t, "route says", chainParams(t, owners)["header:X-Tenant"]["description"])
	require.Equal(t, "route 400", requireMap(t, chainResponses(t, owners)["400"])["description"])

	other := modelOperation(t, spec, "/api/v1x/other", "get")
	require.Equal(t, []any{"api"}, other["tags"], "a prefix matches at a segment boundary")
	require.NotContains(t, modelOperation(t, spec, "/open", "get"), "tags")
}

func Test_OpenAPI_ErrorSchemaFillsDeclaredErrorResponses(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/a", listUsers).
		Response(fiber.StatusNotFound, "missing").
		Response(fiber.StatusConflict, "conflict", MIMEForTest).
		ResponseWithExample(fiber.StatusBadRequest, "bad", publicModel{}, "", nil, nil, fiber.MIMEApplicationJSON).
		Response(fiber.StatusAccepted, "queued")
	spec := modelSpec(t, app, Config{ErrorSchema: secretModel{}})
	responses := chainResponses(t, modelOperation(t, spec, "/a", "get"))

	notFound := requireMap(t, requireMap(t, requireMap(t, responses["404"])["content"])[fiber.MIMEApplicationJSON])
	require.Equal(t, map[string]any{"$ref": "#/components/schemas/secretModel"}, notFound["schema"])
	require.Contains(t, modelSchemas(t, spec), "secretModel")

	bad := requireMap(t, requireMap(t, requireMap(t, responses["400"])["content"])[fiber.MIMEApplicationJSON])
	require.Equal(t, map[string]any{"$ref": "#/components/schemas/publicModel"}, bad["schema"], "a declared body is kept")
	require.NotContains(t, requireMap(t, requireMap(t, responses["409"])["content"]), fiber.MIMEApplicationJSON, "a declared media type is kept")
	require.NotContains(t, requireMap(t, requireMap(t, responses["202"])["content"])[fiber.MIMEApplicationJSON], "schema", "only error statuses are filled")
}

// MIMEForTest is a media type the error handler does not write.
const MIMEForTest = "application/problem+xml"

func Test_OpenAPI_DisableUIServesOnlyTheDocument(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{DisableUI: true}))
	app.Get("/swagger", func(c fiber.Ctx) error { return c.SendString("mine") })
	apptest.Expect(t, app, "/openapi.json", fiber.StatusOK)
	apptest.ExpectBody(t, app, "/swagger", fiber.StatusOK, "mine")
}

func Test_OpenAPI_AlternativeUIPages(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		ui       UI
		contains []string
	}{
		{"redoc", UIRedoc, []string{`<redoc spec-url="/openapi.json">`, "redoc@2.5.0/bundles/redoc.standalone.js", `integrity="sha384-`}},
		{"scalar", UIScalar, []string{`id="api-reference" data-url="/openapi.json"`, "@scalar/api-reference@1.81.0", `integrity="sha384-`}},
		{"swagger", UISwagger, []string{`id="swagger-ui"`}},
		{"default", "", []string{`id="swagger-ui"`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(Config{UI: tc.ui, Title: "Pets"}))
			page := apptest.Body(t, apptest.Expect(t, app, "/swagger", fiber.StatusOK))
			for _, want := range tc.contains {
				require.Contains(t, page, want)
			}
			require.Contains(t, page, "Pets")
		})
	}

	app := fiber.New()
	app.Use(New(Config{UI: UIRedoc, RedocBundleURL: "https://cdn.example.com/redoc.js"}))
	page := apptest.Body(t, apptest.Expect(t, app, "/swagger", fiber.StatusOK))
	require.Contains(t, page, "https://cdn.example.com/redoc.js")
	require.NotContains(t, page, "integrity=", "a custom URL carries no default hash")

	require.Panics(t, func() { New(Config{UI: "elements"}) })
}

func Test_OpenAPI_OAuth2RedirectPage(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256([]byte(oauth2RedirectScript))
	require.Equal(t, OAuth2RedirectScriptCSPSource, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")

	app := fiber.New()
	app.Use(New(Config{OAuth2RedirectPath: "/oauth2-redirect.html"}))
	page := apptest.Body(t, apptest.Expect(t, app, "/oauth2-redirect.html", fiber.StatusOK))
	require.Contains(t, page, "swaggerUIRedirectOauth2")
	require.Equal(t, 1, strings.Count(page, "<script"), "the page has the one inline script the hash covers")
	ui := apptest.Body(t, apptest.Expect(t, app, "/swagger", fiber.StatusOK))
	require.Contains(t, ui, `data-oauth2-redirect="/oauth2-redirect.html"`)

	prefixed := fiber.New()
	prefixed.Group("/docs").Use(New(Config{OAuth2RedirectPath: "/oauth2-redirect.html"}))
	apptest.Expect(t, prefixed, "/docs/oauth2-redirect.html", fiber.StatusOK)
	require.Contains(t, apptest.Body(t, apptest.Expect(t, prefixed, "/docs/swagger", fiber.StatusOK)), `data-oauth2-redirect="/docs/oauth2-redirect.html"`)

	off := fiber.New()
	off.Use(New())
	apptest.Expect(t, off, "/oauth2-redirect.html", fiber.StatusNotFound)
	require.NotContains(t, apptest.Body(t, apptest.Expect(t, off, "/swagger", fiber.StatusOK)), "data-oauth2-redirect")

	require.Panics(t, func() { New(Config{OAuth2RedirectPath: "/swagger"}) })
	require.Panics(t, func() { New(Config{OAuth2RedirectPath: "/openapi.json"}) })
}

func Test_OpenAPI_PathItemsAddSharedFields(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/pets/:id", listUsers)
	app.Delete("/pets/:id", listUsers)
	spec := modelSpec(t, app, Config{PathItems: map[string]PathItem{
		"/pets/{id}": {
			Summary:     "A pet",
			Description: "Every method of a pet",
			Parameters:  []fiber.RouteParameter{{Name: "X-Region", In: fiber.ParamInHeader, Description: "region"}},
		},
		"/missing": {Summary: "not documented, so ignored"},
	}})
	item := requireMap(t, requireMap(t, spec["paths"])["/pets/{id}"])
	require.Equal(t, "A pet", item["summary"])
	require.Equal(t, "Every method of a pet", item["description"])
	params, ok := item["parameters"].([]any)
	require.True(t, ok)
	require.Equal(t, "X-Region", requireMap(t, params[0])["name"])
	require.Contains(t, item, "get")
	require.Contains(t, item, "delete")
	require.NotContains(t, requireMap(t, spec["paths"]), "/missing")
}

func Test_OpenAPI_DocumentGetBodies(t *testing.T) {
	t.Parallel()
	build := func() *fiber.App {
		app := fiber.New()
		app.Get("/search", listUsers).Accepts(publicModel{})
		app.Head("/search", listUsers).Accepts(publicModel{})
		app.Trace("/search", listUsers).Accepts(publicModel{})
		app.Post("/search", listUsers).Accepts(publicModel{})
		return app
	}
	spec := modelSpec(t, build())
	require.NotContains(t, modelOperation(t, spec, "/search", "get"), "requestBody")
	require.Contains(t, modelOperation(t, spec, "/search", "post"), "requestBody")

	spec = modelSpec(t, build(), Config{DocumentGetBodies: true})
	require.Contains(t, modelOperation(t, spec, "/search", "get"), "requestBody")
	require.Contains(t, modelOperation(t, spec, "/search", "head"), "requestBody")
	require.NotContains(t, modelOperation(t, spec, "/search", "trace"), "requestBody", "TRACE never has one")
}

func Test_OpenAPI_CustomMethodsAreAdditionalOperationsIn32(t *testing.T) {
	t.Parallel()
	build := func() *fiber.App {
		app := fiber.New(fiber.Config{RequestMethods: append(slices.Clip(fiber.DefaultMethods), "PURGE")})
		app.Add([]string{"PURGE"}, "/cache/:key", listUsers).Summary("Purge").Tags("ops")
		app.Get("/cache/:key", listUsers)
		app.Add([]string{fiber.MethodConnect}, "/tunnel", listUsers)
		return app
	}

	spec := modelSpec(t, build(), Config{OpenAPIVersion: "3.2.0"})
	item := requireMap(t, requireMap(t, spec["paths"])["/cache/{key}"])
	additional := requireMap(t, item["additionalOperations"])
	require.Equal(t, "Purge", requireMap(t, additional["PURGE"])["summary"])
	require.NotContains(t, item, "purge")
	require.Contains(t, item, "get")
	require.Contains(t, requireMap(t, requireMap(t, requireMap(t, spec["paths"])["/tunnel"])["additionalOperations"]), fiber.MethodConnect)
	tags, ok := spec["tags"].([]any)
	require.True(t, ok)
	require.Equal(t, "ops", requireMap(t, tags[0])["name"], "a tag only an additional operation uses is declared")

	for _, version := range []string{"3.0.3", "3.1.0"} {
		spec = modelSpec(t, build(), Config{OpenAPIVersion: version})
		require.NotContains(t, requireMap(t, spec["paths"]), "/tunnel", version)
		require.NotContains(t, requireMap(t, requireMap(t, spec["paths"])["/cache/{key}"]), "additionalOperations", version)
	}
}

func Test_OpenAPI_TagMetadataIs32Only(t *testing.T) {
	t.Parallel()
	tags := []Tag{
		{Name: "pets", Summary: "Pets", Kind: "nav"},
		{Name: "cats", Parent: "pets", Description: "Cats"},
	}
	build := func() *fiber.App {
		app := fiber.New()
		app.Get("/cats", listUsers).Tags("cats")
		return app
	}
	spec := modelSpec(t, build(), Config{OpenAPIVersion: "3.2.0", Tags: tags})
	got, ok := spec["tags"].([]any)
	require.True(t, ok)
	require.Equal(t, "Pets", requireMap(t, got[0])["summary"])
	require.Equal(t, "nav", requireMap(t, got[0])["kind"])
	require.Equal(t, "pets", requireMap(t, got[1])["parent"])

	spec = modelSpec(t, build(), Config{OpenAPIVersion: "3.1.0", Tags: tags})
	got, ok = spec["tags"].([]any)
	require.True(t, ok)
	for _, tag := range got {
		for _, key := range []string{"summary", "parent", "kind"} {
			require.NotContains(t, requireMap(t, tag), key)
		}
	}
	require.Equal(t, "Cats", requireMap(t, got[1])["description"])

	require.Panics(t, func() { New(Config{Tags: []Tag{{Name: "a", Parent: "missing"}}}) })
	require.Panics(t, func() { New(Config{Tags: []Tag{{Name: "a", Parent: "a"}}}) })
}

func Test_OpenAPI_ItemSchemaDescribesStreamItems(t *testing.T) {
	t.Parallel()
	build := func() *fiber.App {
		app := fiber.New()
		app.Get("/events", listUsers).ResponseContent(fiber.StatusOK, "events", map[string]fiber.RouteMediaType{
			"text/event-stream": {ItemSchema: publicModel{}},
		})
		return app
	}
	content := func(spec map[string]any) map[string]any {
		resp := requireMap(t, chainResponses(t, modelOperation(t, spec, "/events", "get"))["200"])
		return requireMap(t, requireMap(t, resp["content"])["text/event-stream"])
	}

	entry := content(modelSpec(t, build(), Config{OpenAPIVersion: "3.2.0"}))
	require.Equal(t, map[string]any{"$ref": "#/components/schemas/publicModel"}, entry["itemSchema"])
	require.NotContains(t, entry, "schema")

	entry = content(modelSpec(t, build(), Config{OpenAPIVersion: "3.1.0"}))
	require.NotContains(t, entry, "itemSchema")
	require.Equal(t, map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/publicModel"}}, entry["schema"])
}

func Test_OpenAPI_RecognizesMoreMiddleware(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/slow", timeout.New(listUsers, timeout.Config{Timeout: time.Second}))
	app.Post("/idem", idempotency.New(), listUsers)
	app.Get("/idem", idempotency.New(), listUsers)
	app.Get("/timed", responsetime.New(), listUsers)
	app.Get("/cors", cors.New(), listUsers)
	app.Get("/zip", compress.New(), listUsers)
	app.Get("/safe", recoverer.New(), listUsers)
	spec := modelSpec(t, app)

	require.Contains(t, chainResponses(t, modelOperation(t, spec, "/slow", "get")), "503")

	require.Contains(t, chainParams(t, modelOperation(t, spec, "/idem", "post")), "header:X-Idempotency-Key")
	require.NotEqual(t, true, chainParams(t, modelOperation(t, spec, "/idem", "post"))["header:X-Idempotency-Key"]["required"])
	require.NotContains(t, chainParams(t, modelOperation(t, spec, "/idem", "get")), "header:X-Idempotency-Key", "a safe method is skipped by the middleware")

	ok := func(path string) any { return chainResponses(t, modelOperation(t, spec, path, "get"))["200"] }
	require.Contains(t, chainHeaders(t, ok("/timed")), "X-Response-Time")
	require.Contains(t, chainHeaders(t, ok("/cors")), "Access-Control-Allow-Origin")
	require.Contains(t, chainHeaders(t, ok("/zip")), "Content-Encoding")
	require.Contains(t, chainResponses(t, modelOperation(t, spec, "/safe", "get")), "500")
}

func Test_OpenAPI_MiddlewareHeaderNamesAreConfigurable(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Post("/a", idempotency.New(idempotency.Config{KeyHeader: "Idempotency-Key"}), listUsers)
	app.Get("/b", responsetime.New(responsetime.Config{Header: "Server-Timing"}), listUsers)
	spec := modelSpec(t, app, Config{IdempotencyKeyHeader: "Idempotency-Key", ResponseTimeHeader: "Server-Timing"})
	require.Contains(t, chainParams(t, modelOperation(t, spec, "/a", "post")), "header:Idempotency-Key")
	require.Contains(t, chainHeaders(t, chainResponses(t, modelOperation(t, spec, "/b", "get"))["200"]), "Server-Timing")
}

func Test_OpenAPI_InferMiddlewareCanSkipARoute(t *testing.T) {
	t.Parallel()
	var seen []string
	app := fiber.New()
	app.Use(New(Config{InferMiddleware: func(route *fiber.Route, name string) bool {
		seen = append(seen, name)
		return route.Path != "/health"
	}}))
	app.Use(chainKeyAuth())
	app.Get("/guarded", listUsers)
	app.Get("/health", listUsers)
	spec := mountedSpec(t, app)
	require.Contains(t, modelOperation(t, spec, "/guarded", "get"), "security")
	require.NotContains(t, modelOperation(t, spec, "/health", "get"), "security", "the skipped middleware documents nothing")
	require.NotContains(t, chainResponses(t, modelOperation(t, spec, "/health", "get")), "401")
	require.Contains(t, seen, MiddlewareKeyAuth)
}

func Test_OpenAPI_KeyAuthLookupDocumentsAnAPIKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		lookup APIKeyLookup
	}{
		{APIKeyLookup{In: "header", Name: "X-API-Key"}},
		{APIKeyLookup{In: "query", Name: "api_key"}},
		{APIKeyLookup{In: "cookie", Name: "session"}},
	}
	for _, tc := range cases {
		t.Run(tc.lookup.In, func(t *testing.T) {
			t.Parallel()
			lookup := tc.lookup
			app := fiber.New()
			app.Use(New(Config{KeyAuthLookup: &lookup}))
			app.Use(chainKeyAuth())
			app.Get("/a", listUsers)
			spec := mountedSpec(t, app)
			require.Equal(t, []any{map[string]any{"apiKeyAuth": []any{}}}, modelOperation(t, spec, "/a", "get")["security"])
			schemes := requireMap(t, requireMap(t, spec["components"])["securitySchemes"])
			require.Equal(t, map[string]any{"apiKeyAuth": map[string]any{"type": "apiKey", "in": lookup.In, "name": lookup.Name}}, schemes)
		})
	}
	require.Panics(t, func() { New(Config{KeyAuthLookup: &APIKeyLookup{In: "body", Name: "k"}}) })
	require.Panics(t, func() { New(Config{KeyAuthLookup: &APIKeyLookup{In: "header"}}) })
}

func customAuth() fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Get("X-Custom") == "" {
			return fiber.ErrUnauthorized
		}
		return c.Next()
	}
}

func Test_OpenAPI_CustomMiddlewareDocumentsItself(t *testing.T) {
	t.Parallel()
	build := func() *fiber.App {
		app := fiber.New()
		app.Use("/secure", customAuth())
		app.Get("/secure/a", listUsers)
		app.Get("/open/a", listUsers)
		return app
	}
	cfg := Config{Middleware: []MiddlewareDoc{{
		Handler:         customAuth(),
		Name:            "customauth",
		SecuritySchemes: map[string]any{"customAuth": map[string]any{"type": "apiKey", "in": "header", "name": "X-Custom"}},
		Security:        map[string][]string{"customAuth": {}},
		Parameters:      []fiber.RouteParameter{{Name: "X-Tenant", In: fiber.ParamInHeader}},
		Responses:       map[int]fiber.RouteResponse{fiber.StatusUnauthorized: {Description: "no custom header"}},
		ResponseHeaders: map[string]any{"X-Served-By": "the node that answered"},
	}}}

	spec := modelSpec(t, build(), cfg)
	secured := modelOperation(t, spec, "/secure/a", "get")
	require.Equal(t, []any{map[string]any{"customAuth": []any{}}}, secured["security"])
	require.Contains(t, chainParams(t, secured), "header:X-Tenant")
	responses := chainResponses(t, secured)
	require.Equal(t, "no custom header", requireMap(t, responses["401"])["description"])
	require.Contains(t, chainHeaders(t, responses["200"]), "X-Served-By")
	require.Contains(t, requireMap(t, requireMap(t, spec["components"])["securitySchemes"]), "customAuth")

	open := modelOperation(t, spec, "/open/a", "get")
	require.NotContains(t, open, "security")
	require.NotContains(t, chainHeaders(t, chainResponses(t, open)["200"]), "X-Served-By")

	cfg.InferMiddleware = func(_ *fiber.Route, name string) bool { return name != "customauth" }
	spec = modelSpec(t, build(), cfg)
	require.NotContains(t, modelOperation(t, spec, "/secure/a", "get"), "security", "InferMiddleware reaches custom middleware by name")

	require.Panics(t, func() { New(Config{Middleware: []MiddlewareDoc{{}}}) })
	require.Panics(t, func() { New(Config{Middleware: make([]MiddlewareDoc, maxCustomMiddleware+1)}) })
}

func Test_OpenAPI_ServedDocumentCarriesTheNewConfigThroughACachedBuild(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/a", listUsers)
	app.Use(New(Config{Extensions: map[string]any{"x-a": 1}, PathItems: map[string]PathItem{"/a": {Summary: "A"}}}))

	req := httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	resp := apptest.Do(t, app, req)
	etag := resp.Header.Get(fiber.HeaderETag)
	require.NotEmpty(t, etag)
	require.NoError(t, resp.Body.Close())

	req = httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody)
	req.Header.Set(fiber.HeaderIfNoneMatch, etag)
	apptest.Expect(t, app, req, fiber.StatusNotModified)
}
