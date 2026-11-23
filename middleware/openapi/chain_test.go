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

// rfSchemes returns the security schemes a document declares.
func rfSchemes(t *testing.T, spec map[string]any) map[string]any {
	t.Helper()
	return requireMap(t, requireMap(t, spec["components"])["securitySchemes"])
}

// rfSecurity returns the security requirement the GET operation of path states, nil only when the field is absent.
func rfSecurity(t *testing.T, spec map[string]any, path string) any {
	t.Helper()
	security, ok := modelOperation(t, spec, path, "get")["security"]
	require.True(t, !ok || security != nil, "a present security field is never null")
	return security
}

// rfRequirement is the security requirement of one or more schemes with no scopes.
func rfRequirement(schemes ...string) []any {
	req := map[string]any{}
	for _, scheme := range schemes {
		req[scheme] = []any{}
	}
	return []any{req}
}

func Test_OpenAPI_MiddlewareSecurity(t *testing.T) {
	t.Parallel()
	t.Run("group middleware covers its routes", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, func(app *fiber.App) {
			app.Group("/admin", chainKeyAuth()).Get("/x", listUsers)
			app.Get("/y", listUsers)
			app.Get("/administrators/x", listUsers)
		})
		require.Equal(t, rfRequirement(securitySchemeBearer), rfSecurity(t, spec, "/admin/x"))
		unauthorized := rfResp(t, spec, "/admin/x", "get", "401")
		require.Equal(t, "Unauthorized", unauthorized["description"])
		require.Contains(t, requireMap(t, unauthorized["content"]), fiber.MIMETextPlainCharsetUTF8)
		require.Contains(t, chainHeaders(t, unauthorized), fiber.HeaderWWWAuthenticate)
		for _, path := range []string{"/y", "/administrators/x"} {
			op := modelOperation(t, spec, path, "get")
			require.NotContains(t, op, "security", path)
			require.NotContains(t, chainResponses(t, op), "401", path)
		}
		require.Equal(t, map[string]any{"type": "http", "scheme": "bearer"}, rfSchemes(t, spec)[securitySchemeBearer])
	})

	t.Run("basic auth and a chained pair", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, func(app *fiber.App) {
			app.Get("/basic", chainBasicAuth(), listUsers)
			app.Get("/both", chainBasicAuth(), chainKeyAuth(), listUsers)
		})
		require.Equal(t, rfRequirement(securitySchemeBasic), rfSecurity(t, spec, "/basic"))
		require.Equal(t, rfRequirement(securitySchemeBasic, securitySchemeBearer), rfSecurity(t, spec, "/both"))
		require.Equal(t, map[string]any{"type": "http", "scheme": "basic"}, rfSchemes(t, spec)[securitySchemeBasic])
	})

	t.Run("registration order decides coverage", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, func(app *fiber.App) {
			group := app.Group("/g")
			group.Get("/early", listUsers)
			group.Use(chainKeyAuth())
			group.Get("/late", listUsers)
		})
		require.Nil(t, rfSecurity(t, spec, "/g/early"))
		require.NotNil(t, rfSecurity(t, spec, "/g/late"))
	})

	t.Run("explicit security and user schemes win", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, func(app *fiber.App) {
			Doc(app.Get("/x", chainKeyAuth(), listUsers)).Security(map[string][]string{"apiKey": {}})
			app.Get("/y", chainKeyAuth(), listUsers)
		}, Config{SecuritySchemes: map[string]any{
			"apiKey":             map[string]any{"type": "apiKey", "in": "header", "name": "X-API-Key"},
			securitySchemeBearer: map[string]any{"type": "apiKey", "in": "header", "name": "X-Token"},
		}})
		require.Equal(t, rfRequirement("apiKey"), rfSecurity(t, spec, "/x"))
		require.Equal(t, "X-Token", requireMap(t, rfSchemes(t, spec)[securitySchemeBearer])["name"])
	})

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, func(app *fiber.App) { app.Get("/x", chainKeyAuth(), listUsers) }, Config{DisableMiddlewareInference: true})
		op := modelOperation(t, spec, "/x", "get")
		require.NotContains(t, op, "security")
		require.NotContains(t, chainResponses(t, op), "401")
		require.NotContains(t, spec, "components")
	})
}

func Test_OpenAPI_MiddlewareHeadersAndResponses(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Use(requestid.New(), limiter.New(), etag.New(), cache.New(), csrf.New())
		Doc(app.Get("/x", listUsers)).Returns(fiber.StatusOK, modelAddress{})
		app.Post("/x", listUsers)
	})
	get := modelOperation(t, spec, "/x", "get")
	responses := chainResponses(t, get)
	ok := requireMap(t, responses["200"])
	require.Equal(t, componentRef("modelAddress"), contentSchema(t, ok))
	headers := chainHeaders(t, ok)
	require.ElementsMatch(t, []string{fiber.HeaderXRequestID, headerRateLimitLimit, headerRateLimitRemain, headerRateLimitReset, fiber.HeaderETag, headerCacheStatus}, rfKeys(headers))
	require.Equal(t, "integer", requireMap(t, requireMap(t, headers[headerRateLimitLimit])["schema"])["type"])

	tooMany := requireMap(t, responses["429"])
	require.Equal(t, "Too Many Requests", tooMany["description"])
	require.Contains(t, chainHeaders(t, tooMany), fiber.HeaderRetryAfter)
	require.Contains(t, requireMap(t, tooMany["content"]), fiber.MIMETextPlainCharsetUTF8)

	notModified := requireMap(t, responses["304"])
	require.NotContains(t, notModified, "content")
	require.Contains(t, chainHeaders(t, notModified), fiber.HeaderXRequestID)
	require.Contains(t, chainHeaders(t, notModified), fiber.HeaderETag, "the 304 repeats the tag")
	require.NotContains(t, chainParams(t, get), "header:"+headerCSRFToken)
	require.NotContains(t, responses, "403")

	post := modelOperation(t, spec, "/x", "post")
	require.Equal(t, true, chainParams(t, post)["header:"+headerCSRFToken]["required"])
	postResponses := chainResponses(t, post)
	require.Contains(t, postResponses, "403")
	require.NotContains(t, postResponses, "304", "a 304 answers GET and HEAD only")
	require.Contains(t, chainHeaders(t, postResponses["200"]), fiber.HeaderETag, "a POST 200 carries a tag too")
}

func Test_OpenAPI_MiddlewareKeepsDocumentedHeaders(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Use(requestid.New())
		Doc(app.Get("/x", listUsers)).ResponseHeader(fiber.StatusOK, fiber.HeaderXRequestID, "Mine", nil)
	})
	require.Equal(t, "Mine", requireMap(t, chainHeaders(t, rfResp(t, spec, "/x", "get", "200"))[fiber.HeaderXRequestID])["description"])
}

func Test_OpenAPI_ValidationResponse(t *testing.T) {
	t.Parallel()
	validated := func(register func(app *fiber.App), cfg ...Config) map[string]any {
		app := fiber.New(fiber.Config{StructValidator: stubValidator{}})
		register(app)
		return modelSpec(t, app, cfg...)
	}
	acceptsAddress := func(app *fiber.App) { Doc(app.Post("/u", listUsers)).Accepts(modelAddress{}) }

	spec := validated(func(app *fiber.App) {
		acceptsAddress(app)
		Doc(app.Get("/q", listUsers)).Params("query", modelPaging{})
		app.Get("/plain", listUsers)
	})
	for _, tc := range []struct{ path, method string }{{"/u", "post"}, {"/q", "get"}} {
		bad := rfResp(t, spec, tc.path, tc.method, "400")
		require.Equal(t, "Bad Request", bad["description"])
		require.Contains(t, requireMap(t, bad["content"]), fiber.MIMETextPlainCharsetUTF8)
	}
	require.NotContains(t, chainResponses(t, modelOperation(t, spec, "/plain", "get")), "400")

	spec = validated(acceptsAddress, Config{ErrorProduces: fiber.MIMEApplicationJSON, ErrorSchema: modelPaging{}})
	require.Equal(t, componentRef("modelPaging"), contentSchema(t, rfResp(t, spec, "/u", "post", "400")), "error schema and media type")

	require.NotContains(t, chainResponses(t, modelOperation(t, rfSpec(t, acceptsAddress), "/u", "post")), "400", "absent without a validator")
	spec = validated(acceptsAddress, Config{DisableValidationResponses: true})
	require.NotContains(t, chainResponses(t, modelOperation(t, spec, "/u", "post")), "400", "absent when disabled")
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
		set := handlerMiddleware([]fiber.Handler{handler}, nil)
		require.True(t, set.has(kind), kind)
		require.Equal(t, middlewareSet(1<<kind), set, kind)
	}
	require.Zero(t, handlerMiddleware([]fiber.Handler{nil, listUsers, func(c fiber.Ctx) error { return c.Next() }}, nil))
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
		{"/users/:id<int>", "/users/me", false},
		{"/users/:id<int>", "/users/:id<int>", true},
		{"/users/:id<int>", "/users/:id", false},
		{"/users/:id<min(1)>", "/users/7", false},
		{"/:tenant", "/", false},
		{"/:tenant", "/:id?", false},
		{"/:tenant", "/*", false},
		{"/api/:tenant", "/api/:id?", false},
		{"/api/:v<int>", "/api/health", false},
		{"/api/:v<alpha>", "/api/health", true},
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
	policy := &routePolicy{equal: utils.EqualFold[string]}
	require.Zero(t, middlewareOn([]coveringMiddleware{{prefix: "/", domain: "api.example", set: set}}, &route, policy))
	require.Equal(t, set, middlewareOn([]coveringMiddleware{{prefix: "/", set: set}}, &route, policy))
}

func Test_securitySchemes_Declarations(t *testing.T) {
	t.Parallel()
	var nilSchemes *securitySchemes
	require.Nil(t, nilSchemes.declarations())

	schemes := newSecuritySchemes(&Config{})
	require.Nil(t, schemes.requirement(0))
	require.Nil(t, schemes.declarations())

	var jwt middlewareSet
	jwt.add(kindJWT)
	require.Equal(t, map[string][]string{securitySchemeBearer: {}}, schemes.requirement(jwt))
	require.Nil(t, schemes.declarations(), "asking for a requirement declares nothing")
	schemes.use(jwt)
	require.Equal(t, "JWT", requireMap(t, schemes.declarations()[securitySchemeBearer])["bearerFormat"])

	var key middlewareSet
	key.add(kindKeyAuth)
	schemes.use(key)
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
	schema := SchemaOf(flaggedUser{})
	props := requireMap(t, schema["properties"])
	require.Equal(t, true, requireMap(t, props["id"])["readOnly"])
	require.Equal(t, true, requireMap(t, props["secret"])["writeOnly"])
	old := requireMap(t, props["old"])
	require.Equal(t, true, old["deprecated"])
	require.Equal(t, "Use new", old["description"])
	require.Equal(t, map[string]any{"id": int64(7), "home": map[string]any{"city": "Berlin"}}, schema["example"])

	spec := rfSpec(t, func(app *fiber.App) { Doc(app.Get("/u", listUsers)).Returns(fiber.StatusOK, flaggedUser{}) })
	require.Equal(t, map[string]any{"id": float64(7), "home": map[string]any{"city": "Berlin"}}, requireMap(t, modelSchemas(t, spec)["flaggedUser"])["example"])
}

func Test_OpenAPI_RequestBodyMediaType(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		register func(app *fiber.App)
		name     string
		want     string
		cfg      Config
	}{
		{name: "declared body without a media type documents JSON", register: func(app *fiber.App) { Doc(app.Post("/x", listUsers)).RequestBody("payload", true) }, want: fiber.MIMEApplicationJSON},
		{name: "configured default", register: func(app *fiber.App) { Doc(app.Post("/x", listUsers)).Accepts(modelAddress{}) }, want: fiber.MIMETextPlain, cfg: Config{DefaultConsumes: fiber.MIMETextPlain}},
		{name: "explicit media type wins", register: func(app *fiber.App) { Doc(app.Post("/x", listUsers)).Accepts(modelAddress{}, fiber.MIMEApplicationXML) }, want: fiber.MIMEApplicationXML},
		{name: "Consumes with Accepts", register: func(app *fiber.App) {
			Doc(app.Post("/x", listUsers)).Consumes(fiber.MIMEApplicationXML).Accepts(modelAddress{})
		}, want: fiber.MIMEApplicationXML},
		{name: "Consumes with RequestBody", want: fiber.MIMEApplicationXML, register: func(app *fiber.App) {
			Doc(app.Post("/x", listUsers)).Consumes(fiber.MIMEApplicationXML).RequestBody("payload", true)
		}},
		{name: "a declared media type beats Consumes", want: fiber.MIMEApplicationJSON, register: func(app *fiber.App) {
			Doc(app.Post("/x", listUsers)).Consumes(fiber.MIMEApplicationXML).Accepts(modelAddress{}, fiber.MIMEApplicationJSON)
		}},
		{name: "Consumes alone still documents a body", register: func(app *fiber.App) { Doc(app.Post("/x", listUsers)).Consumes(fiber.MIMEApplicationXML) }, want: fiber.MIMEApplicationXML},
		{name: "no Consumes uses the default", register: func(app *fiber.App) { Doc(app.Post("/x", listUsers)).Accepts(modelAddress{}) }, want: fiber.MIMEApplicationJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := requireMap(t, modelOperation(t, rfSpec(t, tc.register, tc.cfg), "/x", "post")["requestBody"])
			content := requireMap(t, body["content"])
			require.Len(t, content, 1)
			require.Contains(t, content, tc.want)
		})
	}
}

func Test_OpenAPI_ExplicitSecurityWinsOverMiddleware(t *testing.T) {
	t.Parallel()
	t.Run("Security() documents no authentication", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, func(app *fiber.App) {
			Doc(app.Get("/public", chainKeyAuth(), listUsers)).Security()
			app.Get("/private", chainKeyAuth(), listUsers)
		}, Config{
			Security:        []map[string][]string{{"docLevel": {}}},
			SecuritySchemes: map[string]any{"docLevel": map[string]any{"type": "http", "scheme": "basic"}},
		})
		public := modelOperation(t, spec, "/public", "get")
		require.Equal(t, []any{}, public["security"])
		require.NotContains(t, chainResponses(t, public), "401")
		require.NotNil(t, rfSecurity(t, spec, "/private"))
		raw, err := json.Marshal(public)
		require.NoError(t, err)
		require.Contains(t, string(raw), `"security":[]`)
	})

	t.Run("a stated requirement replaces inference entirely", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, func(app *fiber.App) {
			Doc(app.Get("/x", chainKeyAuth(), listUsers)).Security(map[string][]string{"oauth": {"read"}})
		},
			Config{SecuritySchemes: map[string]any{"oauth": map[string]any{"type": "oauth2"}}})
		op := modelOperation(t, spec, "/x", "get")
		require.Equal(t, []any{map[string]any{"oauth": []any{"read"}}}, op["security"])
		require.NotContains(t, chainResponses(t, op), "401")
		require.NotContains(t, rfSchemes(t, spec), securitySchemeBearer)
	})

	t.Run("other middleware is still documented", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, func(app *fiber.App) {
			app.Use(requestid.New())
			Doc(app.Get("/x", chainKeyAuth(), listUsers)).Security()
		})
		require.Contains(t, chainHeaders(t, rfResp(t, spec, "/x", "get", "200")), fiber.HeaderXRequestID)
	})

	t.Run("no inferred scheme is declared", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, func(app *fiber.App) { Doc(app.Get("/public", chainBasicAuth(), listUsers)).Security() })
		if components, ok := spec["components"].(map[string]any); ok {
			require.NotContains(t, components, "securitySchemes")
		}
	})
}

func Test_OpenAPI_DisableDefaultMediaTypes(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{StructValidator: stubValidator{}})
	app.Get("/x", listUsers)
	Doc(app.Post("/y", listUsers)).RequestBody("payload", true)
	app.Get("/z", chainKeyAuth(), listUsers)
	Doc(app.Get("/w", listUsers)).Produces(fiber.MIMEApplicationXML)
	spec := modelSpec(t, app, Config{DisableDefaultMediaTypes: true})

	require.NotContains(t, rfResp(t, spec, "/x", "get", "200"), "content")
	require.NotContains(t, modelOperation(t, spec, "/y", "post"), "requestBody")
	require.NotContains(t, rfResp(t, spec, "/z", "get", "401"), "content")
	require.NotContains(t, rfResp(t, spec, "/y", "post", "400"), "content")
	require.Contains(t, requireMap(t, rfResp(t, spec, "/w", "get", "200")["content"]), fiber.MIMEApplicationXML)

	cfg := configDefault(Config{DisableDefaultMediaTypes: true, DefaultProduces: fiber.MIMETextPlain})
	require.Equal(t, fiber.MIMETextPlain, cfg.DefaultProduces)
	require.Empty(t, cfg.DefaultConsumes)
	require.Empty(t, cfg.ErrorProduces)
}

func Test_OpenAPI_MiddlewareCoverage(t *testing.T) {
	t.Parallel()
	t.Run("a parametric Use prefix covers the routes under it", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, func(app *fiber.App) {
			app.Group("/users/:id", chainKeyAuth()).Get("/posts", listUsers)
			app.Get("/accounts/:id/posts", listUsers)
		})
		require.NotNil(t, rfSecurity(t, spec, "/users/{id}/posts"))
		require.Nil(t, rfSecurity(t, spec, "/accounts/{id}/posts"))
	})

	t.Run("the app's case rule applies", func(t *testing.T) {
		t.Parallel()
		for sensitive, want := range map[bool]bool{false: true, true: false} {
			app := fiber.New(fiber.Config{CaseSensitive: sensitive})
			app.Use("/Admin", chainKeyAuth())
			app.Get("/admin/x", listUsers)
			require.Equal(t, want, rfSecurity(t, modelSpec(t, app), "/admin/x") != nil, "case sensitive: %v", sensitive)
		}
	})

	t.Run("domain routes", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, func(app *fiber.App) {
			api := app.Domain("api.example")
			api.Get("/own", chainKeyAuth(), listUsers)
			api.Use(chainBasicAuth())
			api.Get("/covered", listUsers)
			app.Domain("other.example").Get("/elsewhere", listUsers)
			app.Domain("chain.example").RouteChain("/chain").Get(chainKeyAuth(), listUsers)
		})
		own := modelOperation(t, spec, "/own", "get")
		require.Equal(t, rfRequirement(securitySchemeBearer), own["security"])
		require.Equal(t, "List users", own["summary"])
		require.Equal(t, rfRequirement(securitySchemeBasic), rfSecurity(t, spec, "/covered"))
		require.Nil(t, rfSecurity(t, spec, "/elsewhere"))
		chain := modelOperation(t, spec, "/chain", "get")
		require.Contains(t, chain, "security", "a domain route chain")
		require.Equal(t, "List users", chain["summary"])
	})
}

func Test_classifySegment(t *testing.T) {
	t.Parallel()
	for segment, want := range map[string]segmentKind{
		"users": segmentLiteral, "": segmentLiteral, "a\\:b": segmentLiteral,
		":id": segmentParam, ":id<int>": segmentParam, "v-:n": segmentParam, ":id<regex(a+)>": segmentParam,
		":id?": segmentOptional, "*": segmentGreedy, "+": segmentGreedy,
	} {
		kind, _ := classifySegment(segment)
		require.Equal(t, want, kind, segment)
	}
}

func Test_OpenAPI_MiddlewareHeaderNames(t *testing.T) {
	t.Parallel()
	register := func(app *fiber.App) {
		app.Use(requestid.New(requestid.Config{Header: "X-Trace-ID"}), limiter.New(), cache.New(cache.Config{CacheHeader: "X-Edge"}), csrf.New())
		app.Get("/x", listUsers)
		app.Post("/x", listUsers)
	}

	t.Run("renamed", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, register, Config{
			RequestIDHeader:  "X-Trace-ID",
			CacheHeader:      "X-Edge",
			CSRFHeader:       "X-XSRF",
			RateLimitHeaders: RateLimitHeaders{Limit: "RateLimit-Limit", Reset: "RateLimit-Reset"},
		})
		headers := rfKeys(chainHeaders(t, rfResp(t, spec, "/x", "get", "200")))
		require.ElementsMatch(t, []string{"X-Trace-ID", "X-Edge", "RateLimit-Limit", "RateLimit-Reset", headerRateLimitRemain}, headers)
		require.ElementsMatch(t, []string{"header:X-XSRF"}, rfKeys(chainParams(t, modelOperation(t, spec, "/x", "post"))))
	})

	t.Run("rate limit headers disabled", func(t *testing.T) {
		t.Parallel()
		spec := rfSpec(t, register, Config{DisableRateLimitHeaders: true})
		headers := chainHeaders(t, rfResp(t, spec, "/x", "get", "200"))
		for _, name := range []string{headerRateLimitLimit, headerRateLimitRemain, headerRateLimitReset} {
			require.NotContains(t, headers, name)
		}
		require.NotContains(t, chainHeaders(t, rfResp(t, spec, "/x", "get", "429")), fiber.HeaderRetryAfter, "the limiter drops Retry-After with its other headers")
	})
}

func Test_OpenAPI_ErrorSchemaImpliesJSON(t *testing.T) {
	t.Parallel()
	type errBody struct {
		Message string `json:"message"`
	}
	media := func(cfg Config) map[string]any {
		spec := rfSpec(t, func(app *fiber.App) { app.Get("/x", chainKeyAuth(), listUsers) }, cfg)
		return requireMap(t, rfResp(t, spec, "/x", "get", "401")["content"])
	}
	require.Contains(t, media(Config{ErrorSchema: errBody{}}), fiber.MIMEApplicationJSON)
	require.Contains(t, media(Config{ErrorSchema: errBody{}, ErrorProduces: fiber.MIMETextPlain}), fiber.MIMETextPlain, "an explicit choice wins")
	require.Contains(t, media(Config{}), fiber.MIMETextPlainCharsetUTF8)
}

func Test_OpenAPI_MiddlewareHeadersOnlyOnResponsesThatCarryThem(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/x", requestid.New(), chainKeyAuth(), limiter.New(), cache.New(), listUsers)
		Doc(app.Get("/y", chainKeyAuth(), cache.New(), listUsers)).Response(fiber.StatusNotFound, "Missing")
	})
	responses := chainResponses(t, modelOperation(t, spec, "/x", "get"))
	headersOf := func(status string) map[string]any { return chainHeaders(t, responses[status]) }

	require.Subset(t, rfKeys(headersOf("200")), []string{headerRateLimitLimit, headerCacheStatus})
	// A keyauth 401 never reached the limiter or the cache.
	require.ElementsMatch(t, []string{fiber.HeaderXRequestID, fiber.HeaderWWWAuthenticate}, rfKeys(headersOf("401")))
	// The limiter's own 429 carries Retry-After; the window headers are set only after the handler ran.
	require.ElementsMatch(t, []string{fiber.HeaderXRequestID, fiber.HeaderRetryAfter}, rfKeys(headersOf("429")))
	for status := range responses {
		require.Contains(t, headersOf(status), fiber.HeaderXRequestID, status)
	}

	require.Contains(t, chainHeaders(t, rfResp(t, spec, "/y", "get", "404")), headerCacheStatus, "the cache sets it on the handler's errors too")
	require.NotContains(t, chainHeaders(t, rfResp(t, spec, "/y", "get", "401")), headerCacheStatus)
}
