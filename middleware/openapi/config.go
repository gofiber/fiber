package openapi

import (
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3/internal/schemautil"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/deepcopy"
	"github.com/gofiber/utils/v2"
)

const (
	versionOpenAPI30 = "3.0.0"
	versionOpenAPI31 = "3.1.0"
	versionOpenAPI32 = "3.2.0"
)

// Contact holds contact information for the exposed API.
type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// License holds license information for the exposed API.
type License struct {
	Name string `json:"name"`
	// Identifier is an SPDX license expression (OpenAPI 3.1+); exclusive with URL.
	Identifier string `json:"identifier,omitempty"`
	URL        string `json:"url,omitempty"`
}

// Server represents a server hosting the API.
type Server struct {
	Variables   map[string]ServerVariable `json:"variables,omitempty"`
	URL         string                    `json:"url"`
	Description string                    `json:"description,omitempty"`
	// Name uniquely refers to the host (OpenAPI 3.2+).
	Name string `json:"name,omitempty"`
}

// ServerVariable describes a single variable for server URL template substitution.
type ServerVariable struct {
	Default     string   `json:"default"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// Tag adds metadata to a single tag used by operations.
type Tag struct {
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	// Summary is a short summary of the tag (OpenAPI 3.2+).
	Summary string `json:"summary,omitempty"`
	// Parent is the name of the tag this one is nested in (OpenAPI 3.2+).
	Parent string `json:"parent,omitempty"`
	// Kind classifies the tag, for example "nav", "badge" or "audience" (OpenAPI 3.2+).
	Kind string `json:"kind,omitempty"`
}

// UI selects the documentation page the middleware serves at UIPath.
type UI string

// The documentation pages the middleware can serve.
const (
	// UISwagger serves Swagger UI, the default.
	UISwagger UI = "swagger"
	// UIRedoc serves Redoc.
	UIRedoc UI = "redoc"
	// UIScalar serves Scalar API Reference.
	UIScalar UI = "scalar"
)

// PathItem holds the fields a Path Item carries beside its operations, shared by every method of the path.
type PathItem struct {
	// Summary is a short summary of the path.
	Summary string
	// Description describes the path.
	Description string
	// Parameters apply to every operation of the path.
	Parameters []fiber.RouteParameter
}

// Defaults documents every route whose path starts with PathPrefix. A route's own documentation wins:
// Tags apply to a route without tags, and Parameters and Responses only fill in what the route does not declare.
type Defaults struct {
	// Responses are added for the statuses a route does not declare, keyed by status; 0 is the default response.
	Responses map[int]fiber.RouteResponse
	// PathPrefix selects the routes by their registered path, at a segment boundary; "" or "/" selects all.
	PathPrefix string
	// Tags tag a route that has none, replacing the tag derived from its group.
	Tags []string
	// Security is the requirement of a route that sets none and that no recognized authentication middleware
	// covers; an empty non-nil list documents it as public (`security: []`).
	Security []map[string][]string
	// Parameters are added to a route that does not declare a parameter of the same name and location.
	Parameters []fiber.RouteParameter
}

// MiddlewareDoc documents a middleware of your own. Handler is one instance of the middleware: every handler
// built by the same constructor is recognized, the way Fiber's own middleware is.
type MiddlewareDoc struct {
	// Handler is an instance of the middleware, as returned by its constructor.
	Handler fiber.Handler
	// SecuritySchemes are declared under components.securitySchemes when a route the middleware covers is documented.
	SecuritySchemes map[string]any
	// Security is the requirement the middleware adds: scheme name to scopes. It joins the requirement of any
	// authentication middleware chained with it, and an explicit Security(...) on a route replaces it.
	Security map[string][]string
	// Responses are documented on a route the middleware covers, keyed by status, unless the route declares the status.
	Responses map[int]fiber.RouteResponse
	// ResponseHeaders are added to every response of a route the middleware covers, keyed by header name.
	ResponseHeaders map[string]any
	// Name identifies the middleware to Config.InferMiddleware.
	Name string
	// Parameters are added to a route the middleware covers.
	Parameters []fiber.RouteParameter
}

// APIKeyLookup says where the keyauth middleware reads its key from, as its KeyLookup does.
type APIKeyLookup struct {
	// In is "header", "query" or "cookie".
	In string
	// Name is the header, query parameter or cookie name.
	Name string
}

// ExternalDocs references external documentation for the API.
type ExternalDocs struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// RateLimitHeaders names the headers the limiter middleware sets on a response.
type RateLimitHeaders struct {
	Limit     string
	Remaining string
	Reset     string
}

// The names of the middleware Fiber recognizes, as Config.InferMiddleware receives them.
const (
	MiddlewareKeyAuth      = "keyauth"
	MiddlewareBasicAuth    = "basicauth"
	MiddlewareJWT          = "jwt"
	MiddlewareCSRF         = "csrf"
	MiddlewareRequestID    = "requestid"
	MiddlewareLimiter      = "limiter"
	MiddlewareETag         = "etag"
	MiddlewareCache        = "cache"
	MiddlewareTimeout      = "timeout"
	MiddlewareIdempotency  = "idempotency"
	MiddlewareResponseTime = "responsetime"
	MiddlewareCORS         = "cors"
	MiddlewareCompress     = "compress"
	MiddlewareRecover      = "recover"
	MiddlewareSession      = "session"
	MiddlewareHealthcheck  = "healthcheck"
	MiddlewareEarlyData    = "earlydata"
)

// Config defines the config for middleware.
type Config struct {
	// ErrorSchema is the schema, or a Go value reflected into one, documented for error responses. Optional. Default: nil
	ErrorSchema any

	// ExternalDocs references external documentation for the API. Optional. Default: nil
	ExternalDocs *ExternalDocs

	// Filter keeps a route in the document when it returns true, so one app can serve several documents. It runs
	// on a copy of each route and must not modify it. Optional. Default: nil, which documents every route
	Filter func(route *fiber.Route) bool

	// Transform edits the finished document, given as a generic map, before it is encoded; return an error to
	// fail the request. It runs on a fresh copy for every build. Optional. Default: nil
	Transform func(document map[string]any) error

	// InferMiddleware returns false to stop documenting a recognized middleware (one of the Middleware*
	// names) on a route, for a middleware whose Next skips that route or that you chose not to document.
	// Optional. Default: nil, which documents every recognized middleware
	InferMiddleware func(route *fiber.Route, name string) bool

	// KeyAuthLookup says where the keyauth middleware reads its key from, so it is documented as an apiKey
	// scheme instead of a bearer token. Optional. Default: nil, which documents a bearer token
	KeyAuthLookup *APIKeyLookup

	// SessionLookup says where the session middleware keeps the session ID: In is "cookie" or "header" and Name its
	// name. Optional. Default: nil, which documents a "session_id" cookie
	SessionLookup *APIKeyLookup

	// HealthcheckProduces is the media type of the answer of the healthcheck middleware, which depends on its
	// ResponseFormat: "text/plain; charset=utf-8", "application/json", "application/xml", "application/msgpack" or
	// "application/cbor". Optional. Default: "text/plain; charset=utf-8"
	HealthcheckProduces string

	// Extensions are "x-" fields added to the document root. Optional. Default: nil
	Extensions map[string]any

	// InfoExtensions are "x-" fields added to the info object. Optional. Default: nil
	InfoExtensions map[string]any

	// PathItems add summary, description and parameters to a Path Item, keyed by its OpenAPI path ("/users/{id}"). Optional. Default: nil
	PathItems map[string]PathItem

	// Defaults document the routes under a path prefix; the first entry that selects a route wins for what it sets. Optional. Default: nil
	Defaults []Defaults

	// Middleware documents middleware of your own. Optional. Default: nil
	Middleware []MiddlewareDoc

	// OAuth2RedirectPath is the route where the OAuth2 redirect page of Swagger UI is served, for the
	// authorization code flow; Swagger UI is pointed at it. Optional. Default: "", which serves none
	OAuth2RedirectPath string

	// UI selects the documentation page served at UIPath. Optional. Default: UISwagger
	UI UI

	// RedocBundleURL is the Redoc script URL. Optional. Default: "https://unpkg.com/redoc@2.5.0/bundles/redoc.standalone.js"
	RedocBundleURL string

	// RedocBundleIntegrity is the SRI value checked against RedocBundleURL. Optional. Default: the hash of the default bundle
	RedocBundleIntegrity string

	// ScalarBundleURL is the Scalar API Reference script URL. Optional. Default: "https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.81.0/dist/browser/standalone.min.js"
	ScalarBundleURL string

	// ScalarBundleIntegrity is the SRI value checked against ScalarBundleURL. Optional. Default: the hash of the default bundle
	ScalarBundleIntegrity string

	// SwaggerOptions holds extra options merged into the SwaggerUIBundle call. Optional. Default: nil
	SwaggerOptions map[string]any

	// Components holds reusable definitions emitted under "components". Optional. Default: nil
	Components map[string]any

	// SecuritySchemes holds scheme definitions emitted under "components.securitySchemes". Optional. Default: nil
	SecuritySchemes map[string]any

	// Webhooks maps a name to a Path Item object. OpenAPI 3.1+ only. Optional. Default: nil
	Webhooks map[string]any

	// Next defines a function to skip this middleware when returned true. Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// Contact holds contact information for the exposed API. Optional. Default: nil
	Contact *Contact

	// License holds license information for the exposed API. Optional. Default: nil
	License *License

	// RateLimitHeaders names the headers the limiter sets; empty fields keep their default. Optional. Default: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset
	RateLimitHeaders RateLimitHeaders

	// TermsOfService is a URL to the Terms of Service for the API. Optional. Default: ""
	TermsOfService string

	// Summary is a short summary of the API (info.summary). OpenAPI 3.1+ only. Optional. Default: ""
	Summary string

	// JSONSchemaDialect sets the default JSON Schema dialect. OpenAPI 3.1+ only. Optional. Default: ""
	JSONSchemaDialect string

	// Self is the document's self-assigned URI ("$self"). OpenAPI 3.2+ only. Optional. Default: ""
	Self string

	// ServerURL is the server URL used in the generated specification. Optional. Default: ""
	ServerURL string

	// OpenAPIVersion selects any 3.0.x, 3.1.x or 3.2.x release, written as given; New panics on anything else. Optional. Default: "3.1.0"
	OpenAPIVersion string

	// SwaggerStandalonePresetURL is the preset script URL; empty selects the default, never omits it. Optional. Default: "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-standalone-preset.js"
	SwaggerStandalonePresetURL string

	// Title is the title for the generated OpenAPI specification. Optional. Default: "Fiber API"
	Title string

	// Version is the version for the generated OpenAPI specification. Optional. Default: "1.0.0"
	Version string

	// Description is the description for the generated OpenAPI specification. Optional. Default: ""
	Description string

	// SwaggerBundleURL is the script URL used by the generated Swagger UI page. Optional. Default: "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-bundle.js"
	SwaggerBundleURL string

	// Path is the route where the specification will be served. Optional. Default: "/openapi.json"
	Path string

	// UIPath is the route where the Swagger UI page will be served. Optional. Default: "/swagger"
	UIPath string

	// SwaggerCSSURL is the stylesheet URL used by the generated Swagger UI page. Optional. Default: "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui.css"
	SwaggerCSSURL string

	// SwaggerCSSIntegrity is the SRI value checked against SwaggerCSSURL. The default applies only with the default URL. Optional. Default: the hash of the default stylesheet
	SwaggerCSSIntegrity string

	// SwaggerBundleIntegrity is the SRI value checked against SwaggerBundleURL, defaulting like SwaggerCSSIntegrity. Optional. Default: the hash of the default bundle
	SwaggerBundleIntegrity string

	// SwaggerStandalonePresetIntegrity is the SRI value checked against SwaggerStandalonePresetURL, defaulting like SwaggerCSSIntegrity. Optional. Default: the hash of the default preset
	SwaggerStandalonePresetIntegrity string

	// TypeSchemas gives a Go type the schema you choose, written inline wherever the type is used: a generic
	// map, or a Go value reflected like any model. It wins over a SchemaProvider, and serves types you cannot
	// add a method to, such as time.Duration or a decimal of another package. Optional. Default: nil
	TypeSchemas map[reflect.Type]any

	// SchemaName names the component of a model type; return "" to keep the default (the type's name, then
	// qualified by its package, then numbered on a clash). The result is restricted to the characters a
	// component name may have. Optional. Default: nil
	SchemaName func(t reflect.Type) string

	// DefaultProduces is the response media type documented when a response declares none. Optional. Default: "application/json"
	DefaultProduces string

	// DefaultConsumes is the request media type documented when a body declares none. Optional. Default: "application/json"
	DefaultConsumes string

	// ErrorProduces is the media type of error handler responses. Optional. Default: "text/plain; charset=utf-8", or "application/json" when ErrorSchema is set
	ErrorProduces string

	// RequestIDHeader is the header the requestid middleware sets, documented on routes it covers. Optional. Default: "X-Request-ID"
	RequestIDHeader string

	// CacheHeader is the header the cache middleware sets, documented on routes it covers. Optional. Default: "X-Cache"
	CacheHeader string

	// CSRFHeader is the header the csrf middleware reads its token from, documented on unsafe operations it covers. Optional. Default: "X-Csrf-Token"
	CSRFHeader string

	// IdempotencyKeyHeader is the header the idempotency middleware reads its key from, documented on unsafe operations it covers. Optional. Default: "X-Idempotency-Key"
	IdempotencyKeyHeader string

	// ResponseTimeHeader is the header the responsetime middleware sets, documented on routes it covers. Optional. Default: "X-Response-Time"
	ResponseTimeHeader string

	// Tags lists top-level tag definitions (with descriptions) used by operations. Optional. Default: nil
	Tags []Tag

	// Security lists document-level requirements, combined with OR semantics; an empty non-nil list is written as `security: []`. Optional. Default: nil
	Security []map[string][]string

	// Servers lists the servers hosting the API; it takes precedence over ServerURL. Optional. Default: nil
	Servers []Server

	// DisableDefaultMediaTypes stops filling in unset DefaultProduces, DefaultConsumes and ErrorProduces. Optional. Default: false
	DisableDefaultMediaTypes bool

	// DisableMiddlewareInference stops documenting what recognized middleware on a route's path adds. Optional. Default: false
	DisableMiddlewareInference bool

	// DisableValidationResponses stops documenting the 400 a configured StructValidator makes possible. Optional. Default: false
	DisableValidationResponses bool

	// DisableRateLimitHeaders stops documenting limiter headers sent on every response (the 429 stays). Optional. Default: false
	DisableRateLimitHeaders bool

	// DisableGroupTags stops tagging untagged routes with their group's name. Optional. Default: false
	DisableGroupTags bool

	// DisableHandlerSummaries stops deriving a missing summary from the handler name. Optional. Default: false
	DisableHandlerSummaries bool

	// DisableUI serves no documentation page, only the document. Optional. Default: false
	DisableUI bool

	// DocumentGetBodies documents the request body of a GET or HEAD route, which has no defined meaning
	// (RFC 9110) and is left out by default. TRACE never has one. Optional. Default: false
	DocumentGetBodies bool
}

// ConfigDefault is the default config.
var ConfigDefault = Config{
	Next:                       nil,
	Title:                      "Fiber API",
	Version:                    "1.0.0",
	Description:                "",
	ServerURL:                  "",
	Path:                       "/openapi.json",
	UIPath:                     "/swagger",
	SwaggerCSSURL:              "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui.css",
	SwaggerBundleURL:           "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-bundle.js",
	SwaggerStandalonePresetURL: "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-standalone-preset.js",

	SwaggerCSSIntegrity:              "sha384-9Q2fpS+xeS4ffJy6CagnwoUl+4ldAYhOs9pgZuEKxypVModhmZFzeMlvVsAjf7uT",
	SwaggerBundleIntegrity:           "sha384-EYdOaiRwn44zNjrw+Tfs06qYz9BGQVo2f4/pLY5i7VorbjnZNhdplAbTBk8FXHUJ",
	SwaggerStandalonePresetIntegrity: "sha384-49fpFaVrAWI/qdgl9Vv5E/4NXxRUiJX5vGuLws1NUpTWGtEqzWEx8gHTw2UTehFK",
	SwaggerOptions:                   nil,
	UI:                               UISwagger,
	RedocBundleURL:                   "https://unpkg.com/redoc@2.5.0/bundles/redoc.standalone.js",
	RedocBundleIntegrity:             "sha384-4vOjrBu7SuDWXcAw1qFznVLA/sKL+0l4nn+J1HY8w7cpa6twQEYuh4b0Cwuo7CyX",
	ScalarBundleURL:                  "https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.81.0/dist/browser/standalone.min.js",
	ScalarBundleIntegrity:            "sha384-DXupxC0wR1MllQMEexwTowkgKaBvZGJuwz+TU+7cHC09aC+q5vPMiJS2TW3GWjPN",
	OpenAPIVersion:                   versionOpenAPI31,
	DefaultProduces:                  fiber.MIMEApplicationJSON,
	DefaultConsumes:                  fiber.MIMEApplicationJSON,
	ErrorProduces:                    fiber.MIMETextPlainCharsetUTF8,
	DisableGroupTags:                 false,
	DisableHandlerSummaries:          false,
	DisableDefaultMediaTypes:         false,
	DisableMiddlewareInference:       false,
	DisableValidationResponses:       false,
	RequestIDHeader:                  fiber.HeaderXRequestID,
	CacheHeader:                      "X-Cache",
	CSRFHeader:                       "X-Csrf-Token",
	IdempotencyKeyHeader:             "X-Idempotency-Key",
	HealthcheckProduces:              fiber.MIMETextPlainCharsetUTF8,
	ResponseTimeHeader:               fiber.HeaderXResponseTime,
	RateLimitHeaders: RateLimitHeaders{
		Limit:     "X-RateLimit-Limit",
		Remaining: "X-RateLimit-Remaining",
		Reset:     "X-RateLimit-Reset",
	},
	DisableRateLimitHeaders: false,
}

func defaultAsset(url, integrity *string, defaultURL, defaultIntegrity string) {
	if *url != "" {
		return
	}
	*url = defaultURL
	if *integrity == "" {
		*integrity = defaultIntegrity
	}
}

func configDefault(config ...Config) Config {
	if len(config) < 1 {
		return ConfigDefault
	}

	cfg := config[0]

	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}
	if cfg.Title == "" {
		cfg.Title = ConfigDefault.Title
	}
	if cfg.Version == "" {
		cfg.Version = ConfigDefault.Version
	}
	if cfg.Path == "" {
		cfg.Path = ConfigDefault.Path
	}
	if cfg.RequestIDHeader == "" {
		cfg.RequestIDHeader = ConfigDefault.RequestIDHeader
	}
	if cfg.CacheHeader == "" {
		cfg.CacheHeader = ConfigDefault.CacheHeader
	}
	if cfg.CSRFHeader == "" {
		cfg.CSRFHeader = ConfigDefault.CSRFHeader
	}
	if cfg.HealthcheckProduces == "" {
		cfg.HealthcheckProduces = ConfigDefault.HealthcheckProduces
	}
	if cfg.IdempotencyKeyHeader == "" {
		cfg.IdempotencyKeyHeader = ConfigDefault.IdempotencyKeyHeader
	}
	if cfg.ResponseTimeHeader == "" {
		cfg.ResponseTimeHeader = ConfigDefault.ResponseTimeHeader
	}
	if cfg.RateLimitHeaders.Limit == "" {
		cfg.RateLimitHeaders.Limit = ConfigDefault.RateLimitHeaders.Limit
	}
	if cfg.RateLimitHeaders.Remaining == "" {
		cfg.RateLimitHeaders.Remaining = ConfigDefault.RateLimitHeaders.Remaining
	}
	if cfg.RateLimitHeaders.Reset == "" {
		cfg.RateLimitHeaders.Reset = ConfigDefault.RateLimitHeaders.Reset
	}
	if cfg.UIPath == "" {
		cfg.UIPath = ConfigDefault.UIPath
	}
	defaultAsset(&cfg.SwaggerCSSURL, &cfg.SwaggerCSSIntegrity, ConfigDefault.SwaggerCSSURL, ConfigDefault.SwaggerCSSIntegrity)
	defaultAsset(&cfg.SwaggerBundleURL, &cfg.SwaggerBundleIntegrity, ConfigDefault.SwaggerBundleURL, ConfigDefault.SwaggerBundleIntegrity)
	defaultAsset(&cfg.SwaggerStandalonePresetURL, &cfg.SwaggerStandalonePresetIntegrity, ConfigDefault.SwaggerStandalonePresetURL, ConfigDefault.SwaggerStandalonePresetIntegrity)
	defaultAsset(&cfg.RedocBundleURL, &cfg.RedocBundleIntegrity, ConfigDefault.RedocBundleURL, ConfigDefault.RedocBundleIntegrity)
	defaultAsset(&cfg.ScalarBundleURL, &cfg.ScalarBundleIntegrity, ConfigDefault.ScalarBundleURL, ConfigDefault.ScalarBundleIntegrity)
	if cfg.UI == "" {
		cfg.UI = ConfigDefault.UI
	}
	switch cfg.UI {
	case UISwagger, UIRedoc, UIScalar:
	default:
		panic("openapi: UI must be UISwagger, UIRedoc or UIScalar, got " + strconv.Quote(string(cfg.UI)))
	}
	cloneExtensions(&cfg)
	cfg.SwaggerOptions = deepcopy.Map(cfg.SwaggerOptions)
	cfg.Components = deepcopy.Map(cfg.Components)
	cfg.SecuritySchemes = deepcopy.Map(cfg.SecuritySchemes)
	cfg.Webhooks = deepcopy.Map(cfg.Webhooks)
	cfg.Servers = slices.Clone(cfg.Servers)
	for i := range cfg.Servers {
		if variables := cfg.Servers[i].Variables; variables != nil {
			cloned := make(map[string]ServerVariable, len(variables))
			for name, variable := range variables {
				variable.Enum = slices.Clone(variable.Enum)
				cloned[name] = variable
			}
			cfg.Servers[i].Variables = cloned
		}
	}
	cfg.Tags = slices.Clone(cfg.Tags)
	for i := range cfg.Tags {
		if cfg.Tags[i].ExternalDocs != nil {
			docs := *cfg.Tags[i].ExternalDocs
			cfg.Tags[i].ExternalDocs = &docs
		}
	}
	cfg.Security = deepcopy.Security(cfg.Security)
	if cfg.Contact != nil {
		contact := *cfg.Contact
		cfg.Contact = &contact
	}
	if cfg.License != nil {
		license := *cfg.License
		cfg.License = &license
	}
	if cfg.ExternalDocs != nil {
		docs := *cfg.ExternalDocs
		cfg.ExternalDocs = &docs
	}
	if cfg.OpenAPIVersion == "" {
		cfg.OpenAPIVersion = ConfigDefault.OpenAPIVersion
	}
	if !cfg.DisableDefaultMediaTypes {
		if cfg.DefaultProduces == "" {
			cfg.DefaultProduces = ConfigDefault.DefaultProduces
		}
		if cfg.DefaultConsumes == "" {
			cfg.DefaultConsumes = ConfigDefault.DefaultConsumes
		}
		if cfg.ErrorProduces == "" {
			// The default error handler writes text; a schema only makes sense as JSON.
			cfg.ErrorProduces = ConfigDefault.ErrorProduces
			if cfg.ErrorSchema != nil {
				cfg.ErrorProduces = fiber.MIMEApplicationJSON
			}
		}
	}
	if schema, ok := schemautil.AsMap(cfg.ErrorSchema); ok {
		cfg.ErrorSchema = deepcopy.Map(schema)
	}
	if _, ok := parseVersion(cfg.OpenAPIVersion); !ok {
		panic("openapi: OpenAPIVersion must be a 3.0.x, 3.1.x or 3.2.x release, got " + strconv.Quote(cfg.OpenAPIVersion))
	}
	validateServers(cfg.Servers)
	if utils.EqualFold(utils.TrimRight(normalizedPath(cfg.Path), '/'), utils.TrimRight(normalizedPath(cfg.UIPath), '/')) {
		panic("openapi: Path and UIPath must differ: " + cfg.Path)
	}
	if cfg.OAuth2RedirectPath != "" {
		redirect := utils.TrimRight(normalizedPath(cfg.OAuth2RedirectPath), '/')
		for _, other := range []string{cfg.Path, cfg.UIPath} {
			if utils.EqualFold(redirect, utils.TrimRight(normalizedPath(other), '/')) {
				panic("openapi: OAuth2RedirectPath must differ from Path and UIPath: " + cfg.OAuth2RedirectPath)
			}
		}
	}
	return cfg
}

// cloneAny returns a deep copy of v, or v itself when the copy is not of v's type.
func cloneAny[T any](v T) T {
	if cloned, ok := deepcopy.Value(v).(T); ok {
		return cloned
	}
	return v
}

// cloneExtensions detaches the reference-typed fields added for the features beyond the document metadata
// from the caller's config, and rejects what could only produce an invalid document.
func cloneExtensions(cfg *Config) {
	cfg.Extensions = deepcopy.Map(cfg.Extensions)
	cfg.InfoExtensions = deepcopy.Map(cfg.InfoExtensions)
	for _, extensions := range []map[string]any{cfg.Extensions, cfg.InfoExtensions} {
		for key := range extensions {
			if !strings.HasPrefix(key, "x-") {
				panic("openapi: an extension key must start with \"x-\": " + strconv.Quote(key))
			}
		}
	}
	declared := make(map[string]struct{}, len(cfg.Tags))
	for i := range cfg.Tags {
		declared[cfg.Tags[i].Name] = struct{}{}
	}
	for i := range cfg.Tags {
		parent := cfg.Tags[i].Parent
		if _, known := declared[parent]; parent != "" && (!known || parent == cfg.Tags[i].Name) {
			panic("openapi: the parent " + strconv.Quote(parent) + " of tag " + strconv.Quote(cfg.Tags[i].Name) + " must be another tag of Tags")
		}
	}
	if len(cfg.TypeSchemas) > 0 {
		schemas := make(map[reflect.Type]any, len(cfg.TypeSchemas))
		for t, schema := range cfg.TypeSchemas {
			if t == nil {
				panic("openapi: TypeSchemas needs a type for every schema")
			}
			schemas[t] = deepcopy.Value(schema)
		}
		cfg.TypeSchemas = schemas
	}
	cfg.PathItems = cloneAny(cfg.PathItems)
	cfg.Defaults = cloneAny(cfg.Defaults)
	if len(cfg.Middleware) > maxCustomMiddleware {
		panic("openapi: Middleware holds at most " + strconv.Itoa(maxCustomMiddleware) + " entries")
	}
	for i := range cfg.Middleware {
		if cfg.Middleware[i].Handler == nil {
			panic("openapi: Middleware[" + strconv.Itoa(i) + "].Handler is required")
		}
	}
	original := cfg.Middleware
	cfg.Middleware = cloneAny(original)
	for i := range cfg.Middleware {
		cfg.Middleware[i].Handler = original[i].Handler // a func is shared by the copy
	}
	if cfg.SessionLookup != nil {
		lookup := *cfg.SessionLookup
		cfg.SessionLookup = &lookup
		if lookup.In != locCookie && lookup.In != locHeader {
			panic("openapi: SessionLookup.In must be \"cookie\" or \"header\", got " + strconv.Quote(lookup.In))
		}
		if lookup.Name == "" {
			panic("openapi: SessionLookup.Name is required")
		}
	}
	if cfg.KeyAuthLookup != nil {
		lookup := *cfg.KeyAuthLookup
		cfg.KeyAuthLookup = &lookup
		switch lookup.In {
		case "header", "query", "cookie":
		default:
			panic("openapi: KeyAuthLookup.In must be \"header\", \"query\" or \"cookie\", got " + strconv.Quote(lookup.In))
		}
		if lookup.Name == "" {
			panic("openapi: KeyAuthLookup.Name is required")
		}
	}
}

// validateServers panics on a server the specification would reject.
func validateServers(servers []Server) {
	for i := range servers {
		server := &servers[i]
		if server.URL == "" {
			continue
		}
		for name, variable := range server.Variables {
			if variable.Default == "" {
				panic("openapi: Servers[" + utils.FormatInt(int64(i)) + "] variable " + name + " needs a Default")
			}
			if len(variable.Enum) > 0 && !slices.Contains(variable.Enum, variable.Default) {
				panic("openapi: Servers[" + utils.FormatInt(int64(i)) + "] variable " + name + ": Default " + variable.Default + " is not in Enum")
			}
		}
		rest := server.URL
		for {
			_, after, found := strings.Cut(rest, "{")
			if !found {
				break
			}
			name, tail, closed := strings.Cut(after, "}")
			if !closed {
				break
			}
			if _, declared := server.Variables[name]; !declared {
				panic("openapi: Servers[" + utils.FormatInt(int64(i)) + "] URL uses {" + name + "} but declares no such variable")
			}
			rest = tail
		}
	}
}
