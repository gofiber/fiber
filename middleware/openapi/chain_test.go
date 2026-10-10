package openapi

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/basicauth"
	"github.com/gofiber/fiber/v3/middleware/cache"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/fiber/v3/middleware/keyauth"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
)

const (
	headerRateLimitLimit  = "X-RateLimit-Limit"
	headerRateLimitRemain = "X-RateLimit-Remaining"
	headerRateLimitReset  = "X-RateLimit-Reset"
	headerCSRFToken       = "X-Csrf-Token"
	headerCacheStatus     = "X-Cache"
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
		require.Contains(t, chainHeaders(t, unauthorized), fiber.HeaderWWWAuthenticate)

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
	require.Contains(t, chainHeaders(t, tooMany), fiber.HeaderRetryAfter)
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

func Test_coversRoute(t *testing.T) {
	t.Parallel()

	fold := utils.EqualFold[string]
	for _, tc := range []struct {
		prefix, path string
		want         bool
	}{
		{"/", "/anything", true},
		{"", "/anything", true},
		{"/admin", "/admin", true},
		{"/admin/", "/admin/x", true},
		{"/admin", "/admin/x/y", true},
		{"/admin", "/administrators", false},
		{"/admin", "/other/admin", false},
		{"/admin", "/", false},
		{"/Admin", "/admin/x", true},
		{"/users/:id", "/users/7/posts", true},
		{"/users/:id", "/users/:id/posts", true},
		{"/users/:id<int>", "/users/7", true},
		{"/users/:id", "/users", false},
		{"/users/:id", "/accounts/7", false},
		{"/users/admin", "/users/:id", false},
		{"/users/admin", "/users/admin/x", true},
		{"/files/*", "/files/a/b", true},
		{"/files/*", "/files", true},
		{"/files/+", "/files", false},
		{"/files/+", "/files/a", true},
		{"/:tenant?", "/x", false},
		{"/a/:b?/c", "/a/c", false},
		{"/v-:n", "/v-1", false},
		{"/v-:n", "/v-:n", true},
	} {
		require.Equal(t, tc.want, coversRoute(tc.prefix, tc.path, fold), "%q covers %q", tc.prefix, tc.path)
	}
	require.False(t, coversRoute("/Admin", "/admin", stringsEqual))
	require.True(t, coversRoute("/admin", "/admin/x", stringsEqual))
}

func Test_middlewareInFile(t *testing.T) {
	t.Parallel()

	root := fiberRoot()
	require.NotEmpty(t, root)
	for file, want := range map[string]middlewareKind{
		root + "/middleware/keyauth/keyauth.go":                            kindKeyAuth,
		root + "/middleware/basicauth/basicauth.go":                        kindBasicAuth,
		root + "/middleware/csrf/csrf.go":                                  kindCSRF,
		root + "/middleware/limiter/limiter_fixed.go":                      kindLimiter,
		root + "/middleware/requestid/requestid.go":                        kindRequestID,
		root + "/middleware/etag/etag.go":                                  kindETag,
		root + "/middleware/cache/cache.go":                                kindCache,
		"/root/go/pkg/mod/github.com/gofiber/contrib/v3/jwt@v1.2.5/jwt.go": kindJWT,
		"github.com/gofiber/contrib/v3/jwt@v1.2.5/jwt.go":                  kindJWT,
		"/app/vendor/github.com/gofiber/contrib/v3/jwt/jwt.go":             kindJWT,
	} {
		var wantSet middlewareSet
		wantSet.add(want)
		require.Equal(t, wantSet, middlewareInFile(file), file)
	}
	for _, file := range []string{
		"",
		"/home/u/app/main.go",
		"/root/go/pkg/mod/github.com/gofiber/contrib/jwt@v1.1.2/jwt.go",
		root + "/middleware/keyauthx/keyauth.go",
		root + "/middleware/openapi/chain.go",
		// Shares a directory name with a Fiber package but is not Fiber's.
		"/home/u/app/middleware/keyauth/keyauth.go",
		"/src/other/middleware/csrf/csrf.go",
		"/src/acme/contrib/v3/jwt/jwt.go",
	} {
		require.Zero(t, middlewareInFile(file), file)
	}
}

func Test_middlewareOn_Domain(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/x", listUsers)
	route := app.GetRoutes(true)[0]
	var set middlewareSet
	set.add(kindRequestID)
	fold := utils.EqualFold[string]
	require.Zero(t, middlewareOn([]coveringMiddleware{{prefix: "/", domain: "api.example", set: set}}, &route, fold))
	require.Equal(t, set, middlewareOn([]coveringMiddleware{{prefix: "/", set: set}}, &route, fold))
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

func Test_OpenAPI_ConsumesDecidesBodyMediaType(t *testing.T) {
	t.Parallel()

	bodyContent := func(t *testing.T, app *fiber.App, path string, cfg ...Config) map[string]any {
		t.Helper()
		body := requireMap(t, modelOperation(t, modelSpec(t, app, cfg...), path, "post")["requestBody"])
		return requireMap(t, body["content"])
	}

	t.Run("Consumes with Accepts", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Post("/x", listUsers).Consumes(fiber.MIMEApplicationXML).Accepts(modelAddress{})
		content := bodyContent(t, app, "/x")
		require.Len(t, content, 1)
		require.Contains(t, content, fiber.MIMEApplicationXML)
	})

	t.Run("Consumes with RequestBody", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Post("/x", listUsers).Consumes(fiber.MIMEApplicationXML).RequestBody("payload", true)
		require.Contains(t, bodyContent(t, app, "/x"), fiber.MIMEApplicationXML)
	})

	t.Run("a declared media type beats Consumes", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Post("/x", listUsers).Consumes(fiber.MIMEApplicationXML).Accepts(modelAddress{}, fiber.MIMEApplicationJSON)
		content := bodyContent(t, app, "/x")
		require.Len(t, content, 1)
		require.Contains(t, content, fiber.MIMEApplicationJSON)
	})

	t.Run("Consumes alone still documents a body", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Post("/x", listUsers).Consumes(fiber.MIMEApplicationXML)
		require.Contains(t, bodyContent(t, app, "/x"), fiber.MIMEApplicationXML)
	})

	t.Run("no Consumes uses the default", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Post("/x", listUsers).Accepts(modelAddress{})
		require.Contains(t, bodyContent(t, app, "/x"), fiber.MIMEApplicationJSON)
	})
}

func Test_OpenAPI_ExplicitSecurityWinsOverMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("Security() documents no authentication", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/public", chainKeyAuth(), listUsers).Security()
		app.Get("/private", chainKeyAuth(), listUsers)
		spec := modelSpec(t, app, Config{
			Security:        []map[string][]string{{"docLevel": {}}},
			SecuritySchemes: map[string]any{"docLevel": map[string]any{"type": "http", "scheme": "basic"}},
		})

		public := modelOperation(t, spec, "/public", "get")
		require.Equal(t, []any{}, public["security"])
		require.NotContains(t, chainResponses(t, public), "401")
		require.Contains(t, modelOperation(t, spec, "/private", "get"), "security")

		raw, err := json.Marshal(public)
		require.NoError(t, err)
		require.Contains(t, string(raw), `"security":[]`)
	})

	t.Run("a stated requirement replaces inference entirely", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Get("/x", chainKeyAuth(), listUsers).Security(map[string][]string{"oauth": {"read"}})
		spec := modelSpec(t, app, Config{SecuritySchemes: map[string]any{"oauth": map[string]any{"type": "oauth2"}}})
		op := modelOperation(t, spec, "/x", "get")
		require.Equal(t, []any{map[string]any{"oauth": []any{"read"}}}, op["security"])
		require.NotContains(t, chainResponses(t, op), "401")
		require.NotContains(t, requireMap(t, requireMap(t, spec["components"])["securitySchemes"]), securitySchemeBearer)
	})

	t.Run("other middleware is still documented", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(requestid.New())
		app.Get("/x", chainKeyAuth(), listUsers).Security()
		op := modelOperation(t, modelSpec(t, app), "/x", "get")
		require.Contains(t, chainHeaders(t, chainResponses(t, op)["200"]), fiber.HeaderXRequestID)
	})
}

func Test_OpenAPI_DisableDefaultMediaTypes(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{StructValidator: stubValidator{}})
	app.Get("/x", listUsers)
	app.Post("/y", listUsers).RequestBody("payload", true)
	app.Get("/z", chainKeyAuth(), listUsers)
	app.Get("/w", listUsers).Produces(fiber.MIMEApplicationXML)
	spec := modelSpec(t, app, Config{DisableDefaultMediaTypes: true})

	require.NotContains(t, requireMap(t, chainResponses(t, modelOperation(t, spec, "/x", "get"))["200"]), "content")
	require.NotContains(t, modelOperation(t, spec, "/y", "post"), "requestBody")
	unauthorized := requireMap(t, chainResponses(t, modelOperation(t, spec, "/z", "get"))["401"])
	require.NotContains(t, unauthorized, "content")
	bad := requireMap(t, chainResponses(t, modelOperation(t, spec, "/y", "post"))["400"])
	require.NotContains(t, bad, "content")
	require.Contains(t, requireMap(t, requireMap(t, chainResponses(t, modelOperation(t, spec, "/w", "get"))["200"])["content"]), fiber.MIMEApplicationXML)

	cfg := configDefault(Config{DisableDefaultMediaTypes: true, DefaultProduces: fiber.MIMETextPlain})
	require.Equal(t, fiber.MIMETextPlain, cfg.DefaultProduces)
	require.Empty(t, cfg.DefaultConsumes)
	require.Empty(t, cfg.ErrorProduces)
}

func Test_OpenAPI_MiddlewareCoverage(t *testing.T) {
	t.Parallel()

	t.Run("a parametric Use prefix covers the routes under it", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Group("/users/:id", chainKeyAuth()).Get("/posts", listUsers)
		app.Get("/accounts/:id/posts", listUsers)
		spec := modelSpec(t, app)
		require.Contains(t, modelOperation(t, spec, "/users/{id}/posts", "get"), "security")
		require.NotContains(t, modelOperation(t, spec, "/accounts/{id}/posts", "get"), "security")
	})

	t.Run("the app's case rule applies", func(t *testing.T) {
		t.Parallel()
		insensitive := fiber.New()
		insensitive.Use("/Admin", chainKeyAuth())
		insensitive.Get("/admin/x", listUsers)
		require.Contains(t, modelOperation(t, modelSpec(t, insensitive), "/admin/x", "get"), "security")

		sensitive := fiber.New(fiber.Config{CaseSensitive: true})
		sensitive.Use("/Admin", chainKeyAuth())
		sensitive.Get("/admin/x", listUsers)
		require.NotContains(t, modelOperation(t, modelSpec(t, sensitive), "/admin/x", "get"), "security")
	})

	t.Run("domain routes", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		api := app.Domain("api.example")
		api.Get("/own", chainKeyAuth(), listUsers)
		api.Use(chainBasicAuth())
		api.Get("/covered", listUsers)
		app.Domain("other.example").Get("/elsewhere", listUsers)
		spec := modelSpec(t, app)

		own := modelOperation(t, spec, "/own", "get")
		require.Equal(t, []any{map[string]any{securitySchemeBearer: []any{}}}, own["security"])
		require.Equal(t, "List users", own["summary"])
		require.Equal(t, []any{map[string]any{securitySchemeBasic: []any{}}}, modelOperation(t, spec, "/covered", "get")["security"])
		require.NotContains(t, modelOperation(t, spec, "/elsewhere", "get"), "security")
	})

	t.Run("a domain route chain", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Domain("api.example").RouteChain("/chain").Get(chainKeyAuth(), listUsers)
		op := modelOperation(t, modelSpec(t, app), "/chain", "get")
		require.Contains(t, op, "security")
		require.Equal(t, "List users", op["summary"])
	})
}

func Test_OpenAPI_ETagHeaderOnReadsOnly(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(etag.New())
	app.Get("/x", listUsers)
	app.Post("/x", listUsers)
	spec := modelSpec(t, app)
	require.Contains(t, chainHeaders(t, chainResponses(t, modelOperation(t, spec, "/x", "get"))["200"]), fiber.HeaderETag)
	require.NotContains(t, chainHeaders(t, chainResponses(t, modelOperation(t, spec, "/x", "post"))["200"]), fiber.HeaderETag)
	require.NotContains(t, chainResponses(t, modelOperation(t, spec, "/x", "post")), "304")
}

func Test_classifySegment(t *testing.T) {
	t.Parallel()

	for segment, want := range map[string]segmentKind{
		"users":          segmentLiteral,
		"":               segmentLiteral,
		":id":            segmentParam,
		":id<int>":       segmentParam,
		"v-:n":           segmentParam,
		":id?":           segmentOptional,
		"*":              segmentGreedy,
		"+":              segmentGreedy,
		"a\\:b":          segmentLiteral,
		":id<regex(a+)>": segmentParam,
	} {
		kind, _ := classifySegment(segment)
		require.Equal(t, want, kind, segment)
	}
}

func Test_keyName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "Page_pkg.User_", keyName("Page[pkg.User]"))
	require.Equal(t, "a-b_c.d", keyName("a-b_c.d"))
	require.Equal(t, "_", keyName("é"))
	require.Empty(t, keyName(""))
	require.Equal(t, "___", keyName("日本語"))
	require.Equal(t, "_", componentName(""))
}

func Test_OpenAPI_MiddlewareHeaderNames(t *testing.T) {
	t.Parallel()

	newApp := func() *fiber.App {
		app := fiber.New()
		app.Use(
			requestid.New(requestid.Config{Header: "X-Trace-ID"}),
			limiter.New(),
			cache.New(cache.Config{CacheHeader: "X-Edge"}),
			csrf.New(),
		)
		app.Get("/x", listUsers)
		app.Post("/x", listUsers)
		return app
	}

	t.Run("renamed", func(t *testing.T) {
		t.Parallel()
		spec := modelSpec(t, newApp(), Config{
			RequestIDHeader:  "X-Trace-ID",
			CacheHeader:      "X-Edge",
			CSRFHeader:       "X-XSRF",
			RateLimitHeaders: RateLimitHeaders{Limit: "RateLimit-Limit", Reset: "RateLimit-Reset"},
		})
		headers := chainHeaders(t, chainResponses(t, modelOperation(t, spec, "/x", "get"))["200"])
		for _, name := range []string{"X-Trace-ID", "X-Edge", "RateLimit-Limit", "RateLimit-Reset", headerRateLimitRemain} {
			require.Contains(t, headers, name)
		}
		for _, name := range []string{fiber.HeaderXRequestID, headerCacheStatus, headerRateLimitLimit, headerRateLimitReset} {
			require.NotContains(t, headers, name)
		}
		params := chainParams(t, modelOperation(t, spec, "/x", "post"))
		require.Contains(t, params, "header:X-XSRF")
		require.NotContains(t, params, "header:"+headerCSRFToken)
	})

	t.Run("rate limit headers disabled", func(t *testing.T) {
		t.Parallel()
		spec := modelSpec(t, newApp(), Config{DisableRateLimitHeaders: true})
		responses := chainResponses(t, modelOperation(t, spec, "/x", "get"))
		headers := chainHeaders(t, responses["200"])
		for _, name := range []string{headerRateLimitLimit, headerRateLimitRemain, headerRateLimitReset} {
			require.NotContains(t, headers, name)
		}
		require.Contains(t, chainHeaders(t, responses["429"]), fiber.HeaderRetryAfter)
	})
}

func Test_OpenAPI_UserPackageNamedLikeFiberMiddleware(t *testing.T) {
	t.Parallel()

	// A handler compiled outside middleware/keyauth must not read as recognized middleware.
	app := fiber.New()
	app.Get("/x", listUsers)
	spec := modelSpec(t, app)
	op := modelOperation(t, spec, "/x", "get")
	require.NotContains(t, op, "security")
	require.NotContains(t, chainResponses(t, op), "401")
}
