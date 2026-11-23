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
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/earlydata"
	"github.com/gofiber/fiber/v3/middleware/healthcheck"
	"github.com/gofiber/fiber/v3/middleware/idempotency"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/responsetime"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/stretchr/testify/require"
)

type secretModel struct {
	Token string `json:"token"`
}

type publicModel struct {
	Name string `json:"name"`
}

// MIMEForTest is a media type the error handler does not write.
const MIMEForTest = "application/problem+xml"

func Test_OpenAPI_FilterNarrowsTheDocument(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Filter: func(route *fiber.Route) bool { return !strings.HasPrefix(route.Path, "/internal") }}))
	app.Use(chainKeyAuth())
	Doc(app.Get("/public/pets", listUsers)).Returns(fiber.StatusOK, publicModel{})
	Doc(app.Get("/internal/keys", listUsers)).Returns(fiber.StatusOK, secretModel{}).Security()
	app.Delete("/internal/keys", listUsers)

	spec := mountedSpec(t, app)
	paths := requireMap(t, spec["paths"])
	require.Contains(t, paths, "/public/pets")
	require.NotContains(t, paths, "/internal/keys")
	require.NotNil(t, rfSecurity(t, spec, "/public/pets"), "a kept route keeps what its middleware infers")
	schemas := modelSchemas(t, spec)
	require.Contains(t, schemas, "publicModel")
	require.NotContains(t, schemas, "secretModel", "a filtered-out route leaves no schema behind")

	spec = rfSpec(t, func(app *fiber.App) { Doc(app.Get("/a", listUsers)).Summary("kept") }, Config{Filter: func(route *fiber.Route) bool {
		route.Path = "/changed"
		return true
	}})
	require.Equal(t, "kept", modelOperation(t, spec, "/a", "get")["summary"], "the filter receives a copy")
}

func Test_OpenAPI_GenerateMatchesTheServedDocument(t *testing.T) {
	t.Parallel()
	build := func() *fiber.App {
		sub := fiber.New()
		Doc(sub.Get("/items", listUsers)).Returns(fiber.StatusOK, publicModel{}).Summary("Items")
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
	require.JSONEq(t, string(fetchSpecBody(t, served)), string(generated))
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
	require.InDelta(t, 2, requireMap(t, fetchJSON(t, app)["info"])["x-build"], 0, "each build starts from a new document")
}

func Test_OpenAPI_TransformErrorFailsTheRequest(t *testing.T) {
	t.Parallel()
	for _, transform := range []func(map[string]any) error{
		func(map[string]any) error { return errors.New("refused") },
		func(document map[string]any) error {
			document["bad"] = make(chan int)
			return nil
		},
	} {
		app := fiber.New()
		app.Use(New(Config{Transform: transform}))
		apptest.Expect(t, app, "/openapi.json", fiber.StatusInternalServerError)
	}

	builds := 0
	app := fiber.New()
	app.Use(New(Config{Transform: func(map[string]any) error {
		builds++
		return errors.New("refused")
	}}))
	apptest.Expect(t, app, "/openapi.json", fiber.StatusInternalServerError)
	apptest.Expect(t, app, "/openapi.json", fiber.StatusInternalServerError)
	require.Equal(t, 1, builds, "the build error is cached")
	app.Get("/later", listUsers)
	apptest.Expect(t, app, "/openapi.json", fiber.StatusInternalServerError)
	require.Equal(t, 2, builds, "until the routes change")
}

func Test_OpenAPI_ExtensionsAreAddedToTheRootAndInfo(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) { app.Get("/a", listUsers) }, Config{
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
	headers := RouteParameter{Name: "X-Tenant", In: ParamInHeader, Description: "default says", Required: true}
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/api/v1/pets", listUsers)
		Doc(app.Get("/api/v1/owners", listUsers)).Tags("own").
			AddParameter(RouteParameter{Name: "X-Tenant", In: ParamInHeader, Description: "route says"}).
			Response(fiber.StatusBadRequest, "route 400")
		app.Get("/api/v1x/other", listUsers)
		app.Get("/open", listUsers)
	}, Config{Defaults: []Defaults{
		{PathPrefix: "/api/v1", Tags: []string{"v1"}, Parameters: []RouteParameter{headers}, Responses: map[int]RouteResponse{
			fiber.StatusBadRequest: {Description: "default 400", Schema: publicModel{}},
			fiber.StatusForbidden:  {Description: "forbidden"},
			0:                      {Description: "any other error"},
		}},
		{PathPrefix: "/api", Tags: []string{"api"}, Responses: map[int]RouteResponse{fiber.StatusForbidden: {Description: "second entry loses"}}},
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
	require.Equal(t, "route 400", rfResp(t, spec, "/api/v1/owners", "get", "400")["description"])
	require.Equal(t, []any{"api"}, modelOperation(t, spec, "/api/v1x/other", "get")["tags"], "a prefix matches at a segment boundary")
	require.NotContains(t, modelOperation(t, spec, "/open", "get"), "tags")
}

func Test_OpenAPI_ErrorSchemaFillsDeclaredErrorResponses(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		Doc(app.Get("/a", listUsers)).
			Response(fiber.StatusNotFound, "missing").
			Response(fiber.StatusConflict, "conflict", MIMEForTest).
			ResponseWithExample(fiber.StatusBadRequest, "bad", publicModel{}, "", nil, nil, fiber.MIMEApplicationJSON).
			Response(fiber.StatusAccepted, "queued")
	}, Config{ErrorSchema: secretModel{}})
	content := func(status string) map[string]any {
		return requireMap(t, rfResp(t, spec, "/a", "get", status)["content"])
	}

	require.Equal(t, componentRef("secretModel"), requireMap(t, content("404")[fiber.MIMEApplicationJSON])["schema"])
	require.Contains(t, modelSchemas(t, spec), "secretModel")
	require.Equal(t, componentRef("publicModel"), requireMap(t, content("400")[fiber.MIMEApplicationJSON])["schema"], "a declared body is kept")
	require.NotContains(t, content("409"), fiber.MIMEApplicationJSON, "a declared media type is kept")
	require.NotContains(t, content("202")[fiber.MIMEApplicationJSON], "schema", "only error statuses are filled")
}

func Test_OpenAPI_DisableUIServesOnlyTheDocument(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{DisableUI: true}))
	app.Get("/swagger", func(c fiber.Ctx) error { return c.SendString("mine") })
	apptest.Expect(t, app, "/openapi.json", fiber.StatusOK)
	apptest.ExpectBody(t, app, "/swagger", fiber.StatusOK, "mine")
}

// rfPage mounts the middleware with cfg on a fresh app and returns the body path answers with 200.
func rfPage(t *testing.T, cfg *Config, path string) string {
	t.Helper()
	app := fiber.New()
	app.Use(New(*cfg))
	return apptest.Body(t, apptest.Expect(t, app, path, fiber.StatusOK))
}

func Test_OpenAPI_AlternativeUIPages(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		ui       UI
		contains []string
	}{
		{"redoc", UIRedoc, []string{`<redoc spec-url="/openapi.json">`, "redoc@2.5.0/bundles/redoc.standalone.js", `integrity="sha384-`}},
		{"scalar", UIScalar, []string{`id="api-reference" data-url="/openapi.json"`, "@scalar/api-reference@1.81.0", `integrity="sha384-`}},
		{"swagger", UISwagger, []string{`id="swagger-ui"`}},
		{"default", "", []string{`id="swagger-ui"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			page := rfPage(t, &Config{UI: tc.ui, Title: "Pets"}, "/swagger")
			for _, want := range append(tc.contains, "Pets") {
				require.Contains(t, page, want)
			}
		})
	}

	page := rfPage(t, &Config{UI: UIRedoc, RedocBundleURL: "https://cdn.example.com/redoc.js"}, "/swagger")
	require.Contains(t, page, "https://cdn.example.com/redoc.js")
	require.NotContains(t, page, "integrity=", "a custom URL carries no default hash")
	require.Panics(t, func() { New(Config{UI: "elements"}) })
}

func Test_OpenAPI_OAuth2RedirectPage(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256([]byte(oauth2RedirectScript))
	require.Equal(t, OAuth2RedirectScriptCSPSource, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")

	cfg := Config{OAuth2RedirectPath: "/oauth2-redirect.html"}
	page := rfPage(t, &cfg, "/oauth2-redirect.html")
	require.Contains(t, page, "swaggerUIRedirectOauth2")
	require.Equal(t, 1, strings.Count(page, "<script"), "the page has the one inline script the hash covers")
	require.Contains(t, rfPage(t, &cfg, "/swagger"), `data-oauth2-redirect="/oauth2-redirect.html"`)

	prefixed := fiber.New()
	prefixed.Group("/docs").Use(New(cfg))
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
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/pets/:id", listUsers)
		app.Delete("/pets/:id", listUsers)
	}, Config{PathItems: map[string]PathItem{
		"/pets/{id}": {
			Summary:     "A pet",
			Description: "Every method of a pet",
			Parameters:  []RouteParameter{{Name: "X-Region", In: ParamInHeader, Description: "region"}},
		},
		"/missing": {Summary: "not documented, so ignored"},
	}})
	item := requireMap(t, requireMap(t, spec["paths"])["/pets/{id}"])
	require.Equal(t, "A pet", item["summary"])
	require.Equal(t, "Every method of a pet", item["description"])
	require.Equal(t, "X-Region", requireMap(t, requireSlice(t, item["parameters"])[0])["name"])
	require.Contains(t, item, "get")
	require.Contains(t, item, "delete")
	require.NotContains(t, requireMap(t, spec["paths"]), "/missing")
}

func Test_OpenAPISpec_MarshalsPathExtrasAndExtensions(t *testing.T) {
	t.Parallel()
	spec := openAPISpec{
		OpenAPI: "3.2.0",
		Info:    openAPIInfo{Title: "t", Version: "1"},
		Paths:   map[string]map[string]operation{"/a": {"get": {Summary: "get a"}}},
		pathExtras: map[string]pathExtra{
			"/a": {summary: "A", description: "all of A", parameters: []parameter{{Name: "id", In: "query"}}, additional: map[string]operation{"PURGE": {Summary: "purge"}}},
			"/b": {summary: "only extras"},
		},
		extensions: map[string]any{"x-team": "api", "openapi": "ignored: a generated key wins"},
	}
	data, err := json.Marshal(spec)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, "api", got["x-team"])
	require.Equal(t, "3.2.0", got["openapi"])
	paths := requireMap(t, got["paths"])
	a := requireMap(t, paths["/a"])
	require.Equal(t, "A", a["summary"])
	require.Equal(t, "all of A", a["description"])
	require.Contains(t, a, "get")
	require.Contains(t, a, "parameters")
	require.Contains(t, a["additionalOperations"], "PURGE")
	require.Equal(t, map[string]any{"summary": "only extras"}, paths["/b"])

	data, err = json.Marshal(openAPISpec{OpenAPI: "3.1.0", Info: openAPIInfo{Title: "t", Version: "1"}, Paths: map[string]map[string]operation{}})
	require.NoError(t, err)
	require.JSONEq(t, `{"openapi":"3.1.0","info":{"title":"t","version":"1"},"paths":{}}`, string(data), "a spec without extras")
}

func Test_OpenAPI_DocumentGetBodies(t *testing.T) {
	t.Parallel()
	register := func(app *fiber.App) {
		Doc(app.Get("/search", listUsers)).Accepts(publicModel{})
		Doc(app.Head("/search", listUsers)).Accepts(publicModel{})
		Doc(app.Trace("/search", listUsers)).Accepts(publicModel{})
		Doc(app.Post("/search", listUsers)).Accepts(publicModel{})
	}
	spec := rfSpec(t, register)
	require.NotContains(t, modelOperation(t, spec, "/search", "get"), "requestBody")
	require.Contains(t, modelOperation(t, spec, "/search", "post"), "requestBody")

	spec = rfSpec(t, register, Config{DocumentGetBodies: true})
	require.Contains(t, modelOperation(t, spec, "/search", "get"), "requestBody")
	require.Contains(t, modelOperation(t, spec, "/search", "head"), "requestBody")
	require.NotContains(t, modelOperation(t, spec, "/search", "trace"), "requestBody", "TRACE never has one")
}

func Test_OpenAPI_CustomMethodsAreAdditionalOperationsIn32(t *testing.T) {
	t.Parallel()
	build := func(version string) map[string]any {
		app := fiber.New(fiber.Config{RequestMethods: append(slices.Clip(fiber.DefaultMethods), "PURGE")})
		Doc(app.Add([]string{"PURGE"}, "/cache/:key", listUsers)).Summary("Purge").Tags("ops")
		app.Get("/cache/:key", listUsers)
		app.Add([]string{fiber.MethodConnect}, "/tunnel", listUsers)
		return modelSpec(t, app, Config{OpenAPIVersion: version})
	}

	spec := build("3.2.0")
	paths := requireMap(t, spec["paths"])
	item := requireMap(t, paths["/cache/{key}"])
	require.Equal(t, "Purge", requireMap(t, requireMap(t, item["additionalOperations"])["PURGE"])["summary"])
	require.NotContains(t, item, "purge")
	require.Contains(t, item, "get")
	require.Contains(t, requireMap(t, requireMap(t, paths["/tunnel"])["additionalOperations"]), fiber.MethodConnect)
	require.Equal(t, "ops", requireMap(t, requireSlice(t, spec["tags"])[0])["name"], "a tag only an additional operation uses is declared")

	for _, version := range []string{"3.0.3", "3.1.0"} {
		paths = requireMap(t, build(version)["paths"])
		require.NotContains(t, paths, "/tunnel", version)
		require.NotContains(t, requireMap(t, paths["/cache/{key}"]), "additionalOperations", version)
	}
}

func Test_OpenAPI_TagMetadataIs32Only(t *testing.T) {
	t.Parallel()
	tags := []Tag{{Name: "pets", Summary: "Pets", Kind: "nav"}, {Name: "cats", Parent: "pets", Description: "Cats"}}
	register := func(app *fiber.App) { Doc(app.Get("/cats", listUsers)).Tags("cats") }

	got := requireSlice(t, rfSpec(t, register, Config{OpenAPIVersion: "3.2.0", Tags: tags})["tags"])
	require.Equal(t, "Pets", requireMap(t, got[0])["summary"])
	require.Equal(t, "nav", requireMap(t, got[0])["kind"])
	require.Equal(t, "pets", requireMap(t, got[1])["parent"])

	got = requireSlice(t, rfSpec(t, register, Config{OpenAPIVersion: "3.1.0", Tags: tags})["tags"])
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
	content := func(version string) map[string]any {
		spec := rfSpec(t, func(app *fiber.App) {
			Doc(app.Get("/events", listUsers)).ResponseContent(fiber.StatusOK, "events", map[string]RouteMediaType{"text/event-stream": {ItemSchema: publicModel{}}})
		}, Config{OpenAPIVersion: version})
		return requireMap(t, requireMap(t, rfResp(t, spec, "/events", "get", "200")["content"])["text/event-stream"])
	}
	require.Equal(t, map[string]any{"itemSchema": componentRef("publicModel")}, content("3.2.0"))
	require.Equal(t, map[string]any{"schema": map[string]any{"type": "array", "items": componentRef("publicModel")}}, content("3.1.0"))
}

func Test_OpenAPI_RecognizesMoreMiddleware(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/slow", timeout.New(listUsers, timeout.Config{Timeout: time.Second}))
		app.Post("/idem", idempotency.New(), listUsers)
		app.Get("/idem", idempotency.New(), listUsers)
		app.Get("/timed", responsetime.New(), listUsers)
		app.Get("/cors", cors.New(), listUsers)
		app.Get("/zip", compress.New(), listUsers)
		app.Get("/safe", recoverer.New(), listUsers)
		app.Get("/s", session.New(), listUsers)
		app.Get("/live", healthcheck.New())
		app.Post("/early", earlydata.New(), listUsers)
		app.Get("/early", earlydata.New(), listUsers)
	})
	responses := func(path, method string) map[string]any {
		return chainResponses(t, modelOperation(t, spec, path, method))
	}
	okHeaders := func(path string) map[string]any { return chainHeaders(t, responses(path, "get")["200"]) }

	require.Contains(t, responses("/slow", "get"), "503")
	idem := chainParams(t, modelOperation(t, spec, "/idem", "post"))
	require.Contains(t, idem, "header:X-Idempotency-Key")
	require.NotEqual(t, true, idem["header:X-Idempotency-Key"]["required"])
	require.NotContains(t, chainParams(t, modelOperation(t, spec, "/idem", "get")), "header:X-Idempotency-Key", "a safe method is skipped by the middleware")
	require.Contains(t, okHeaders("/timed"), "X-Response-Time")
	require.Contains(t, okHeaders("/cors"), "Access-Control-Allow-Origin")
	require.Contains(t, okHeaders("/zip"), "Content-Encoding")
	require.Contains(t, responses("/safe", "get"), "500")

	require.Contains(t, chainParams(t, modelOperation(t, spec, "/s", "get")), "cookie:session_id")
	require.Contains(t, okHeaders("/s"), "Set-Cookie")
	require.Subset(t, rfKeys(responses("/live", "get")), []string{"200", "503"})
	require.Contains(t, responses("/early", "post"), "425")
	require.NotContains(t, responses("/early", "get"), "425")
}

func Test_OpenAPI_MiddlewareHeaderNamesAreConfigurable(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Post("/a", idempotency.New(idempotency.Config{KeyHeader: "Idempotency-Key"}), listUsers)
		app.Get("/b", responsetime.New(responsetime.Config{Header: "Server-Timing"}), listUsers)
		app.Get("/s", session.New(), listUsers)
		app.Get("/live", healthcheck.New())
	}, Config{
		IdempotencyKeyHeader: "Idempotency-Key",
		ResponseTimeHeader:   "Server-Timing",
		SessionLookup:        &APIKeyLookup{In: "header", Name: "X-Session"},
		HealthcheckProduces:  fiber.MIMEApplicationJSON,
	})
	require.Contains(t, chainParams(t, modelOperation(t, spec, "/a", "post")), "header:Idempotency-Key")
	require.Contains(t, chainHeaders(t, rfResp(t, spec, "/b", "get", "200")), "Server-Timing")
	require.Contains(t, chainParams(t, modelOperation(t, spec, "/s", "get")), "header:X-Session")
	require.Contains(t, chainHeaders(t, rfResp(t, spec, "/s", "get", "200")), "X-Session")
	require.Contains(t, requireMap(t, rfResp(t, spec, "/live", "get", "200")["content"]), fiber.MIMEApplicationJSON)

	require.Panics(t, func() { New(Config{SessionLookup: &APIKeyLookup{In: "query", Name: "s"}}) })
	require.Panics(t, func() { New(Config{SessionLookup: &APIKeyLookup{In: "cookie"}}) })
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
	require.NotNil(t, rfSecurity(t, spec, "/guarded"))
	require.Nil(t, rfSecurity(t, spec, "/health"), "the skipped middleware documents nothing")
	require.NotContains(t, chainResponses(t, modelOperation(t, spec, "/health", "get")), "401")
	require.Contains(t, seen, MiddlewareKeyAuth)
}

func Test_OpenAPI_KeyAuthLookupDocumentsAnAPIKey(t *testing.T) {
	t.Parallel()
	for _, lookup := range []APIKeyLookup{{In: "header", Name: "X-API-Key"}, {In: "query", Name: "api_key"}, {In: "cookie", Name: "session"}} {
		t.Run(lookup.In, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(Config{KeyAuthLookup: &lookup}))
			app.Use(chainKeyAuth())
			app.Get("/a", listUsers)
			spec := mountedSpec(t, app)
			require.Equal(t, rfRequirement("apiKeyAuth"), rfSecurity(t, spec, "/a"))
			require.Equal(t, map[string]any{"apiKeyAuth": map[string]any{"type": "apiKey", "in": lookup.In, "name": lookup.Name}}, rfSchemes(t, spec))
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
	register := func(app *fiber.App) {
		app.Use("/secure", customAuth())
		app.Get("/secure/a", listUsers)
		app.Get("/open/a", listUsers)
	}
	cfg := Config{Middleware: []MiddlewareDoc{{
		Handler:         customAuth(),
		Name:            "customauth",
		SecuritySchemes: map[string]any{"customAuth": map[string]any{"type": "apiKey", "in": "header", "name": "X-Custom"}},
		Security:        map[string][]string{"customAuth": {}},
		Parameters:      []RouteParameter{{Name: "X-Tenant", In: ParamInHeader}},
		Responses:       map[int]RouteResponse{fiber.StatusUnauthorized: {Description: "no custom header"}},
		ResponseHeaders: map[string]any{"X-Served-By": "the node that answered"},
	}}}

	spec := rfSpec(t, register, cfg)
	require.Equal(t, rfRequirement("customAuth"), rfSecurity(t, spec, "/secure/a"))
	require.Contains(t, chainParams(t, modelOperation(t, spec, "/secure/a", "get")), "header:X-Tenant")
	require.Equal(t, "no custom header", rfResp(t, spec, "/secure/a", "get", "401")["description"])
	require.Contains(t, chainHeaders(t, rfResp(t, spec, "/secure/a", "get", "200")), "X-Served-By")
	require.Contains(t, rfSchemes(t, spec), "customAuth")
	require.Nil(t, rfSecurity(t, spec, "/open/a"))
	require.NotContains(t, chainHeaders(t, rfResp(t, spec, "/open/a", "get", "200")), "X-Served-By")

	cfg.InferMiddleware = func(_ *fiber.Route, name string) bool { return name != "customauth" }
	require.Nil(t, rfSecurity(t, rfSpec(t, register, cfg), "/secure/a"), "InferMiddleware reaches custom middleware by name")

	require.Panics(t, func() { New(Config{Middleware: []MiddlewareDoc{{}}}) })
	require.Panics(t, func() { New(Config{Middleware: make([]MiddlewareDoc, maxCustomMiddleware+1)}) })
}

func Test_OpenAPI_ServedDocumentCarriesTheNewConfigThroughACachedBuild(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/a", listUsers)
	app.Use(New(Config{Extensions: map[string]any{"x-a": 1}, PathItems: map[string]PathItem{"/a": {Summary: "A"}}}))
	etag := apptest.Expect(t, app, "/openapi.json", fiber.StatusOK).Header.Get(fiber.HeaderETag)
	require.NotEmpty(t, etag)
	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/openapi.json", apptest.WithHeader(fiber.HeaderIfNoneMatch, etag)), fiber.StatusNotModified)
}

func Test_OpenAPI_OperationIDCallbackAndServers(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		Doc(app.Post("/files", listUsers).Name("upload files")).OperationID("uploadFile").
			Callback("done", map[string]any{"{$request.body#/url}": map[string]any{"post": map[string]any{"responses": map[string]any{"200": map[string]any{"description": "ok"}}}}}).
			OperationServer(map[string]any{"url": "https://{region}.example.com", "variables": map[string]any{"region": map[string]any{"default": "eu"}}})
		app.Get("/files", listUsers).Name("upload files") // the same name, so the generated id is numbered
		Doc(app.Domain("a.example.com").Get("/hosted", listUsers)).OperationServer(map[string]any{"url": "https://own.example.com"})
		app.Domain("b.example.com").Get("/hosted", listUsers)
		Doc(app.Post("/twin", listUsers)).OperationID("uploadFile") // an id another operation uses is numbered
	})
	post := modelOperation(t, spec, "/files", "post")
	require.Equal(t, "uploadFile", post["operationId"])
	require.Contains(t, requireMap(t, post["callbacks"]), "done")
	require.Equal(t, "https://{region}.example.com", requireMap(t, requireSlice(t, post["servers"])[0])["url"])
	require.Equal(t, "upload_files", modelOperation(t, spec, "/files", "get")["operationId"])
	require.Equal(t, "uploadFile_2", modelOperation(t, spec, "/twin", "post")["operationId"])
	hosted := requireSlice(t, modelOperation(t, spec, "/hosted", "get")["servers"])
	require.Len(t, hosted, 1, "a route's own servers are not widened by another host")
	require.Equal(t, "https://own.example.com", requireMap(t, hosted[0])["url"])
}

func Test_OpenAPI_DefaultsSecurityFillsRoutesWithoutAuthentication(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Use("/guarded", chainKeyAuth())
		app.Get("/guarded/a", listUsers)
		app.Get("/api/b", listUsers)
		Doc(app.Get("/api/public", listUsers)).Security()
		Doc(app.Get("/api/own", listUsers)).Security(map[string][]string{"own": {}})
		app.Get("/other", listUsers)
	}, Config{
		SecuritySchemes: map[string]any{"company": map[string]any{"type": "http", "scheme": "bearer"}, "own": map[string]any{"type": "http", "scheme": "basic"}},
		Defaults: []Defaults{
			{PathPrefix: "/api", Security: []map[string][]string{{"company": {}}}},
			{PathPrefix: "/guarded", Security: []map[string][]string{{"company": {}}}},
		},
	})
	require.Equal(t, rfRequirement("company"), rfSecurity(t, spec, "/api/b"))
	require.Equal(t, []any{}, rfSecurity(t, spec, "/api/public"), "an explicit Security() makes it public")
	require.Equal(t, rfRequirement("own"), rfSecurity(t, spec, "/api/own"))
	require.Equal(t, rfRequirement("bearerAuth"), rfSecurity(t, spec, "/guarded/a"), "inferred authentication wins")
	require.Nil(t, rfSecurity(t, spec, "/other"))
}

func Test_OpenAPI_SlowSpecBuildDoesNotBlockOtherRequests(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once
	slow := func(v any) ([]byte, error) {
		if _, isSpec := v.(openAPISpec); isSpec {
			once.Do(func() { close(started) })
			<-release
		}
		return json.Marshal(v)
	}
	handler := New()
	slowApp := fiber.New(fiber.Config{JSONEncoder: slow})
	slowApp.Use(handler)
	otherApp := fiber.New()
	otherApp.Use(handler)

	built := make(chan error, 1)
	go func() {
		resp, err := slowApp.Test(httptest.NewRequest(fiber.MethodGet, "/openapi.json", http.NoBody), fiber.TestConfig{Timeout: 10 * time.Second})
		if err == nil {
			err = resp.Body.Close()
		}
		built <- err
	}()
	apptest.Recv(t, started, 10*time.Second, "the slow build did not start")
	apptest.Expect(t, otherApp, "/openapi.json", fiber.StatusOK)
	apptest.Expect(t, slowApp, "/swagger", fiber.StatusOK)
	close(release)
	require.NoError(t, apptest.Recv(t, built, 10*time.Second, "the slow build did not finish"))
}
