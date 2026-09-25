package openapi

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/basicauth"
	"github.com/gofiber/fiber/v3/middleware/cache"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/fiber/v3/middleware/keyauth"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/stretchr/testify/require"
)

func chainKeyAuth() fiber.Handler {
	return keyauth.New(keyauth.Config{Validator: func(fiber.Ctx, string) (bool, error) { return true, nil }})
}

func chainBasicAuth() fiber.Handler {
	sum := sha256.Sum256([]byte("secret"))
	return basicauth.New(basicauth.Config{Users: map[string]string{"john": "{SHA256}" + base64.StdEncoding.EncodeToString(sum[:])}})
}

type stubValidator struct{}

func (stubValidator) Validate(any) error { return nil }

func chainResponses(t *testing.T, op map[string]any) map[string]any {
	t.Helper()
	return requireMap(t, op["responses"])
}

func chainHeaders(t *testing.T, resp any) map[string]any {
	t.Helper()
	headers, ok := requireMap(t, resp)["headers"]
	if !ok {
		return nil
	}
	return requireMap(t, headers)
}

func chainParams(t *testing.T, op map[string]any) map[string]map[string]any {
	t.Helper()
	params := map[string]map[string]any{}
	raw, ok := op["parameters"]
	if !ok {
		return params
	}
	for _, entry := range requireSlice(t, raw) {
		p := requireMap(t, entry)
		params[requireString(t, p["in"])+":"+requireString(t, p["name"])] = p
	}
	return params
}

func Test_OpenAPI_MiddlewareSecurity(t *testing.T) {
	t.Parallel()

	t.Run("group middleware covers its routes", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Group("/admin", chainKeyAuth()).Get("/x", listUsers)
		app.Get("/y", listUsers)
		app.Get("/administrators/x", listUsers)
		spec := modelSpec(t, app)

		admin := modelOperation(t, spec, "/admin/x", "get")
		require.Equal(t, []any{map[string]any{securitySchemeBearer: []any{}}}, admin["security"])
		unauthorized := requireMap(t, chainResponses(t, admin)["401"])
		require.Equal(t, "Unauthorized", unauthorized["description"])
		require.Contains(t, requireMap(t, unauthorized["content"]), fiber.MIMETextPlainCharsetUTF8)
		require.Contains(t, chainHeaders(t, unauthorized), headerWWWAuthenticate)

		for _, path := range []string{"/y", "/administrators/x"} {
			op := modelOperation(t, spec, path, "get")
			require.NotContains(t, op, "security", path)
			require.NotContains(t, chainResponses(t, op), "401", path)
		}

		schemes := requireMap(t, requireMap(t, spec["components"])["securitySchemes"])
		require.Equal(t, map[string]any{"type": "http", "scheme": "bearer"}, schemes[securitySchemeBearer])
	})

	t.Run("basic auth and a chained pair", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/basic", chainBasicAuth(), listUsers)
		app.Get("/both", chainBasicAuth(), chainKeyAuth(), listUsers)
		spec := modelSpec(t, app)

		require.Equal(t, []any{map[string]any{securitySchemeBasic: []any{}}}, modelOperation(t, spec, "/basic", "get")["security"])
		require.Equal(t, []any{map[string]any{securitySchemeBasic: []any{}, securitySchemeBearer: []any{}}}, modelOperation(t, spec, "/both", "get")["security"])
		schemes := requireMap(t, requireMap(t, spec["components"])["securitySchemes"])
		require.Equal(t, map[string]any{"type": "http", "scheme": "basic"}, schemes[securitySchemeBasic])
	})

	t.Run("registration order decides coverage", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		group := app.Group("/g")
		group.Get("/early", listUsers)
		group.Use(chainKeyAuth())
		group.Get("/late", listUsers)
		spec := modelSpec(t, app)
		require.NotContains(t, modelOperation(t, spec, "/g/early", "get"), "security")
		require.Contains(t, modelOperation(t, spec, "/g/late", "get"), "security")
	})

	t.Run("explicit security and user schemes win", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/x", chainKeyAuth(), listUsers).Security(map[string][]string{"apiKey": {}})
		app.Get("/y", chainKeyAuth(), listUsers)
		spec := modelSpec(t, app, Config{SecuritySchemes: map[string]any{
			"apiKey":             map[string]any{"type": "apiKey", "in": "header", "name": "X-API-Key"},
			securitySchemeBearer: map[string]any{"type": "apiKey", "in": "header", "name": "X-Token"},
		}})
		require.Equal(t, []any{map[string]any{"apiKey": []any{}}}, modelOperation(t, spec, "/x", "get")["security"])
		schemes := requireMap(t, requireMap(t, spec["components"])["securitySchemes"])
		require.Equal(t, "X-Token", requireMap(t, schemes[securitySchemeBearer])["name"])
	})

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/x", chainKeyAuth(), listUsers)
		spec := modelSpec(t, app, Config{DisableMiddlewareInference: true})
		op := modelOperation(t, spec, "/x", "get")
		require.NotContains(t, op, "security")
		require.NotContains(t, chainResponses(t, op), "401")
		require.NotContains(t, spec, "components")
	})
}

func Test_OpenAPI_MiddlewareHeadersAndResponses(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(requestid.New(), limiter.New(), etag.New(), cache.New(), csrf.New())
	app.Get("/x", listUsers).Returns(fiber.StatusOK, modelAddress{})
	app.Post("/x", listUsers)
	spec := modelSpec(t, app)

	get := modelOperation(t, spec, "/x", "get")
	responses := chainResponses(t, get)
	ok := requireMap(t, responses["200"])
	require.Equal(t, ref("modelAddress"), contentSchema(t, ok))
	headers := chainHeaders(t, ok)
	for _, name := range []string{fiber.HeaderXRequestID, headerRateLimitLimit, headerRateLimitRemain, headerRateLimitReset, fiber.HeaderETag, headerCacheStatus} {
		require.Contains(t, headers, name)
	}
	require.Equal(t, "integer", requireMap(t, requireMap(t, headers[headerRateLimitLimit])["schema"])["type"])

	tooMany := requireMap(t, responses["429"])
	require.Equal(t, "Too Many Requests", tooMany["description"])
	require.Contains(t, chainHeaders(t, tooMany), headerRetryAfter)
	require.Contains(t, requireMap(t, tooMany["content"]), fiber.MIMETextPlainCharsetUTF8)

	notModified := requireMap(t, responses["304"])
	require.NotContains(t, notModified, "content")
	require.Contains(t, chainHeaders(t, notModified), fiber.HeaderXRequestID)
	require.NotContains(t, chainHeaders(t, notModified), fiber.HeaderETag)

	require.NotContains(t, chainParams(t, get), "header:"+headerCSRFToken)
	require.NotContains(t, responses, "403")

	post := modelOperation(t, spec, "/x", "post")
	token := chainParams(t, post)["header:"+headerCSRFToken]
	require.Equal(t, true, token["required"])
	postResponses := chainResponses(t, post)
	require.Contains(t, postResponses, "403")
	require.NotContains(t, postResponses, "304")
}

func Test_OpenAPI_MiddlewareKeepsDocumentedHeaders(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/x", listUsers).ResponseHeader(fiber.StatusOK, fiber.HeaderXRequestID, "Mine", nil)
	op := modelOperation(t, modelSpec(t, app), "/x", "get")
	header := requireMap(t, chainHeaders(t, chainResponses(t, op)["200"])[fiber.HeaderXRequestID])
	require.Equal(t, "Mine", header["description"])
}

func Test_OpenAPI_ValidationResponse(t *testing.T) {
	t.Parallel()

	t.Run("documented with a validator", func(t *testing.T) {
		t.Parallel()
		app := fiber.New(fiber.Config{StructValidator: stubValidator{}})
		app.Post("/u", listUsers).Accepts(modelAddress{})
		app.Get("/q", listUsers).Params("query", modelPaging{})
		app.Get("/plain", listUsers)
		spec := modelSpec(t, app)
		for _, tc := range []struct{ path, method string }{{"/u", "post"}, {"/q", "get"}} {
			bad := requireMap(t, chainResponses(t, modelOperation(t, spec, tc.path, tc.method))["400"])
			require.Equal(t, "Bad Request", bad["description"])
			require.Contains(t, requireMap(t, bad["content"]), fiber.MIMETextPlainCharsetUTF8)
		}
		require.NotContains(t, chainResponses(t, modelOperation(t, spec, "/plain", "get")), "400")
	})

	t.Run("error schema and media type", func(t *testing.T) {
		t.Parallel()
		app := fiber.New(fiber.Config{StructValidator: stubValidator{}})
		app.Post("/u", listUsers).Accepts(modelAddress{})
		spec := modelSpec(t, app, Config{ErrorProduces: fiber.MIMEApplicationJSON, ErrorSchema: modelPaging{}})
		bad := requireMap(t, chainResponses(t, modelOperation(t, spec, "/u", "post"))["400"])
		require.Equal(t, ref("modelPaging"), contentSchema(t, bad))
	})

	t.Run("absent without a validator or when disabled", func(t *testing.T) {
		t.Parallel()
		plain := fiber.New()
		plain.Post("/u", listUsers).Accepts(modelAddress{})
		require.NotContains(t, chainResponses(t, modelOperation(t, modelSpec(t, plain), "/u", "post")), "400")

		disabled := fiber.New(fiber.Config{StructValidator: stubValidator{}})
		disabled.Post("/u", listUsers).Accepts(modelAddress{})
		spec := modelSpec(t, disabled, Config{DisableValidationResponses: true})
		require.NotContains(t, chainResponses(t, modelOperation(t, spec, "/u", "post")), "400")
	})
}

func Test_handlerMiddleware(t *testing.T) {
	t.Parallel()

	for kind, handler := range map[middlewareKind]fiber.Handler{
		kindKeyAuth:   chainKeyAuth(),
		kindBasicAuth: chainBasicAuth(),
		kindCSRF:      csrf.New(),
		kindRequestID: requestid.New(),
		kindLimiter:   limiter.New(),
		kindETag:      etag.New(),
		kindCache:     cache.New(),
	} {
		set := handlerMiddleware([]fiber.Handler{handler})
		require.True(t, set.has(kind), kind)
		require.Equal(t, middlewareSet(1<<kind), set, kind)
	}
	require.Zero(t, handlerMiddleware([]fiber.Handler{nil, listUsers, func(c fiber.Ctx) error { return c.Next() }}))
}

func Test_coversPath(t *testing.T) {
	t.Parallel()

	require.True(t, coversPath("/", "/anything"))
	require.True(t, coversPath("", "/anything"))
	require.True(t, coversPath("/admin", "/admin"))
	require.True(t, coversPath("/admin/", "/admin/x"))
	require.True(t, coversPath("/admin", "/admin/x/y"))
	require.False(t, coversPath("/admin", "/administrators"))
	require.False(t, coversPath("/admin", "/other/admin"))
}

func Test_middlewareOn_Domain(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/x", listUsers)
	route := app.GetRoutes(true)[0]
	var set middlewareSet
	set.add(kindRequestID)
	require.Zero(t, middlewareOn([]coveringMiddleware{{prefix: "/", domain: "api.example", set: set}}, &route))
	require.Equal(t, set, middlewareOn([]coveringMiddleware{{prefix: "/", set: set}}, &route))
}

func Test_securitySchemes_Declarations(t *testing.T) {
	t.Parallel()

	var nilSchemes *securitySchemes
	require.Nil(t, nilSchemes.declarations())

	schemes := newSecuritySchemes()
	require.Nil(t, schemes.requirement(0))
	require.Nil(t, schemes.declarations())

	var jwt middlewareSet
	jwt.add(kindJWT)
	require.Equal(t, map[string][]string{securitySchemeBearer: {}}, schemes.requirement(jwt))
	require.Equal(t, "JWT", requireMap(t, schemes.declarations()[securitySchemeBearer])["bearerFormat"])

	var key middlewareSet
	key.add(kindKeyAuth)
	schemes.requirement(key)
	require.NotContains(t, requireMap(t, schemes.declarations()[securitySchemeBearer]), "bearerFormat")
}

type flaggedAddress struct {
	City string `json:"city" openapi:"example:Berlin"`
}

type flaggedUser struct {
	Secret string         `json:"secret" openapi:"writeOnly"`
	Old    string         `json:"old" openapi:"deprecated,description:Use new"`
	Home   flaggedAddress `json:"home"`
	ID     int            `json:"id" openapi:"readonly,example:7"`
}

func Test_OpenAPI_FlagTagsAndExamples(t *testing.T) {
	t.Parallel()

	props := requireMap(t, SchemaOf(flaggedUser{})["properties"])
	require.Equal(t, true, requireMap(t, props["id"])["readOnly"])
	require.Equal(t, true, requireMap(t, props["secret"])["writeOnly"])
	old := requireMap(t, props["old"])
	require.Equal(t, true, old["deprecated"])
	require.Equal(t, "Use new", old["description"])
	require.Equal(t, map[string]any{"id": int64(7), "home": map[string]any{"city": "Berlin"}}, SchemaOf(flaggedUser{})["example"])

	app := fiber.New()
	app.Get("/u", listUsers).Returns(fiber.StatusOK, flaggedUser{})
	spec := modelSpec(t, app)
	user := requireMap(t, modelSchemas(t, spec)["flaggedUser"])
	require.Equal(t, map[string]any{"id": float64(7), "home": map[string]any{"city": "Berlin"}}, user["example"])
}
