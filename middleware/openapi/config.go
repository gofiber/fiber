package openapi

import (
	"reflect"
	"slices"

	"github.com/gofiber/fiber/v3"
)

// Supported OpenAPI specification versions.
const (
	versionOpenAPI30 = "3.0.0"
	versionOpenAPI31 = "3.1.0"
	versionOpenAPI32 = "3.2.0"
)

// Contact holds contact information for the exposed API.
type Contact struct {
	// Name is the identifying name of the contact person/organization.
	Name string `json:"name,omitempty"`
	// URL is the URL pointing to the contact information.
	URL string `json:"url,omitempty"`
	// Email is the email address of the contact person/organization.
	Email string `json:"email,omitempty"`
}

// License holds license information for the exposed API.
type License struct {
	// Name is the license name used for the API.
	Name string `json:"name"`
	// Identifier is an SPDX license expression for the API (OpenAPI 3.1+).
	// It is mutually exclusive with URL.
	Identifier string `json:"identifier,omitempty"`
	// URL is a URL to the license used for the API.
	URL string `json:"url,omitempty"`
}

// Server represents a server hosting the API.
type Server struct {
	// Variables is a map of server variables used for URL template substitution.
	Variables map[string]ServerVariable `json:"variables,omitempty"`
	// URL is the server URL.
	URL string `json:"url"`
	// Description is an optional description of the server.
	Description string `json:"description,omitempty"`
	// Name is an optional unique string to refer to the host designated by the
	// URL (OpenAPI 3.2+).
	Name string `json:"name,omitempty"`
}

// ServerVariable describes a single variable for server URL template substitution.
type ServerVariable struct {
	// Default is the value to use when none is supplied. Required.
	Default string `json:"default"`
	// Description is an optional description for the variable.
	Description string `json:"description,omitempty"`
	// Enum is an optional set of allowed values.
	Enum []string `json:"enum,omitempty"`
}

// Tag adds metadata to a single tag used by operations.
type Tag struct {
	// ExternalDocs references external documentation for this tag.
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	// Name is the name of the tag.
	Name string `json:"name"`
	// Description is an optional description for the tag.
	Description string `json:"description,omitempty"`
}

// ExternalDocs references external documentation for the API.
type ExternalDocs struct {
	// Description is an optional description of the external documentation.
	Description string `json:"description,omitempty"`
	// URL is the URL for the external documentation.
	URL string `json:"url"`
}

// RateLimitHeaders names the headers the limiter middleware sets on a response.
type RateLimitHeaders struct {
	// Limit is the header carrying the requests allowed per window.
	Limit string
	// Remaining is the header carrying the requests left in the window.
	Remaining string
	// Reset is the header carrying the seconds until the window resets.
	Reset string
}

// Config defines the config for middleware. It controls top-level document
// metadata only; operation metadata comes from the route helpers.
type Config struct {
	// ErrorSchema is the schema, or a Go value reflected into one, documented for those error responses. Optional. Default: nil
	ErrorSchema any

	// ExternalDocs references external documentation for the API. Optional. Default: nil
	ExternalDocs *ExternalDocs

	// SwaggerOptions holds extra options merged into the SwaggerUIBundle call. Optional. Default: nil
	SwaggerOptions map[string]any

	// Components holds reusable definitions emitted under "components", so $ref targets resolve. Optional. Default: nil
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

	// RateLimitHeaders names the headers the limiter middleware sets on every response of a route it covers: the limit, the remainder and the reset. Set it when you rename them; leave a field empty to keep its default. Optional. Default: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset
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

	// OpenAPIVersion selects the spec version: "3.0.0", "3.1.0" or "3.2.0". Optional. Default: "3.1.0"
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

	// SwaggerCSSIntegrity is the Subresource Integrity value (for example "sha384-...") the browser checks SwaggerCSSURL against. It defaults to the hash of the default stylesheet only while SwaggerCSSURL is left at its default; a custom URL has none unless set here. Optional. Default: the hash of the default stylesheet
	SwaggerCSSIntegrity string

	// SwaggerBundleIntegrity is the Subresource Integrity value the browser checks SwaggerBundleURL against, defaulting as SwaggerCSSIntegrity does. Optional. Default: the hash of the default bundle
	SwaggerBundleIntegrity string

	// SwaggerStandalonePresetIntegrity is the Subresource Integrity value the browser checks SwaggerStandalonePresetURL against, defaulting as SwaggerCSSIntegrity does. Optional. Default: the hash of the default preset
	SwaggerStandalonePresetIntegrity string

	// DefaultProduces is the response media type documented for a response that declares none. Optional. Default: "application/json"
	DefaultProduces string

	// DefaultConsumes is the request media type documented for a request body that declares none. Optional. Default: "application/json"
	DefaultConsumes string

	// ErrorProduces is the media type documented for the responses the app's error handler writes. Optional. Default: "text/plain; charset=utf-8"
	ErrorProduces string

	// RequestIDHeader is the header the requestid middleware sets, as documented on every response of a route it covers. Set it to the requestid middleware's Header when that is not the default. Optional. Default: "X-Request-ID"
	RequestIDHeader string

	// CacheHeader is the header the cache middleware sets, as documented on every response of a route it covers. Set it to the cache middleware's CacheHeader when that is not the default. Optional. Default: "X-Cache"
	CacheHeader string

	// CSRFHeader is the header the csrf middleware reads its token from, as documented on an unsafe operation it covers. Set it to the header your csrf Extractor reads when that is not the default. Optional. Default: "X-Csrf-Token"
	CSRFHeader string

	// Tags lists top-level tag definitions (with descriptions) used by operations. Optional. Default: nil
	Tags []Tag

	// Security lists document-level requirements, combined with OR semantics. Optional. Default: nil
	Security []map[string][]string

	// Servers lists the servers hosting the API; it takes precedence over ServerURL. Optional. Default: nil
	Servers []Server

	// DisableDefaultMediaTypes stops documenting DefaultProduces, DefaultConsumes and ErrorProduces, leaving a response or body that declares no media type without content. Optional. Default: false
	DisableDefaultMediaTypes bool

	// DisableMiddlewareInference stops documenting the security, headers, parameters and responses of recognized middleware on a route's path. Optional. Default: false
	DisableMiddlewareInference bool

	// DisableValidationResponses stops documenting the 400 response a configured StructValidator makes possible on routes with a body or parameters. Optional. Default: false
	DisableValidationResponses bool

	// DisableRateLimitHeaders stops documenting the headers a limiter sets on every response, for a limiter configured with DisableHeaders. Its 429 and Retry-After header are still documented. Optional. Default: false
	DisableRateLimitHeaders bool

	// DisableGroupTags stops tagging an untagged route with its group's name or last static prefix segment. Optional. Default: false
	DisableGroupTags bool

	// DisableHandlerSummaries stops deriving a missing summary from the handler function's name. Optional. Default: false
	DisableHandlerSummaries bool
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

	// Hashes of the three files above, computed from the swagger-ui-dist@5.32.6
	// package. Update them together with the URLs.
	SwaggerCSSIntegrity:              "sha384-9Q2fpS+xeS4ffJy6CagnwoUl+4ldAYhOs9pgZuEKxypVModhmZFzeMlvVsAjf7uT",
	SwaggerBundleIntegrity:           "sha384-EYdOaiRwn44zNjrw+Tfs06qYz9BGQVo2f4/pLY5i7VorbjnZNhdplAbTBk8FXHUJ",
	SwaggerStandalonePresetIntegrity: "sha384-49fpFaVrAWI/qdgl9Vv5E/4NXxRUiJX5vGuLws1NUpTWGtEqzWEx8gHTw2UTehFK",
	SwaggerOptions:                   nil,
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
	RateLimitHeaders: RateLimitHeaders{
		Limit:     "X-RateLimit-Limit",
		Remaining: "X-RateLimit-Remaining",
		Reset:     "X-RateLimit-Reset",
	},
	DisableRateLimitHeaders: false,
}

// maxCopyDepth bounds the configuration deep copy: a cyclic value in
// SwaggerOptions or Components would otherwise overflow the stack in New.
const maxCopyDepth = 100

// deepCopyAnyMap copies a raw OpenAPI object so the caller shares no nested
// container with the handler. Non-container values are copied as-is.
func deepCopyAnyMap(src map[string]any) map[string]any {
	return deepCopyAnyMapDepth(src, 0)
}

func deepCopyAnyMapDepth(src map[string]any, depth int) map[string]any {
	if src == nil {
		return nil
	}
	if depth >= maxCopyDepth {
		// Sharing the reference is the lesser evil; encoding/json reports the
		// cycle itself when the document is served.
		return src
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = deepCopyAnyValueDepth(value, depth+1)
	}
	return dst
}

func deepCopyAnyValueDepth(src any, depth int) any {
	if depth >= maxCopyDepth {
		return src
	}
	switch value := src.(type) {
	case map[string]any:
		return deepCopyAnyMapDepth(value, depth)
	case []any:
		copied := make([]any, len(value))
		for i := range value {
			copied[i] = deepCopyAnyValueDepth(value[i], depth+1)
		}
		return copied
	case []string:
		return slices.Clone(value)
	default:
		return deepCopyReflected(src, depth)
	}
}

// deepCopyReflected clones map and slice values of any concrete type, which the
// typed switch above cannot name. Anything else is returned as-is.
func deepCopyReflected(src any, depth int) any {
	v := reflect.ValueOf(src)
	switch v.Kind() {
	case reflect.Map:
		if v.IsNil() {
			return src
		}
		cloned := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			cloned.SetMapIndex(iter.Key(), deepCopyReflectedValue(iter.Value(), depth+1))
		}
		return cloned.Interface()
	case reflect.Slice:
		if v.IsNil() {
			return src
		}
		cloned := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			cloned.Index(i).Set(deepCopyReflectedValue(v.Index(i), depth+1))
		}
		return cloned.Interface()
	default:
		return src
	}
}

// deepCopyReflectedValue copies one element, recursing through interfaces so a
// nested container inside an `any` is cloned rather than shared.
func deepCopyReflectedValue(v reflect.Value, depth int) reflect.Value {
	if v.Kind() == reflect.Interface && !v.IsNil() {
		return reflect.ValueOf(deepCopyAnyValueDepth(v.Interface(), depth))
	}
	if v.Kind() == reflect.Map || v.Kind() == reflect.Slice {
		return reflect.ValueOf(deepCopyReflected(v.Interface(), depth))
	}
	return v
}

// cloneSecurityRequirements copies the requirement maps and their scope slices
// so the served document never references the caller's live maps.
func cloneSecurityRequirements(src []map[string][]string) []map[string][]string {
	if src == nil {
		return nil
	}
	cloned := make([]map[string][]string, len(src))
	for i, requirement := range src {
		entry := make(map[string][]string, len(requirement))
		for scheme, scopes := range requirement {
			copied := make([]string, len(scopes))
			copy(copied, scopes)
			entry[scheme] = copied
		}
		cloned[i] = entry
	}
	return cloned
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
	if cfg.SwaggerCSSURL == "" {
		cfg.SwaggerCSSURL = ConfigDefault.SwaggerCSSURL
		// The default hash only describes the default file.
		if cfg.SwaggerCSSIntegrity == "" {
			cfg.SwaggerCSSIntegrity = ConfigDefault.SwaggerCSSIntegrity
		}
	}
	if cfg.SwaggerBundleURL == "" {
		cfg.SwaggerBundleURL = ConfigDefault.SwaggerBundleURL
		// The default hash only describes the default file.
		if cfg.SwaggerBundleIntegrity == "" {
			cfg.SwaggerBundleIntegrity = ConfigDefault.SwaggerBundleIntegrity
		}
	}
	if cfg.SwaggerStandalonePresetURL == "" {
		cfg.SwaggerStandalonePresetURL = ConfigDefault.SwaggerStandalonePresetURL
		// The default hash only describes the default file.
		if cfg.SwaggerStandalonePresetIntegrity == "" {
			cfg.SwaggerStandalonePresetIntegrity = ConfigDefault.SwaggerStandalonePresetIntegrity
		}
	}
	// Detach every reference-typed field: the handler reads this config while
	// serving, so anything left aliased races with a caller that mutates it.
	cfg.SwaggerOptions = deepCopyAnyMap(cfg.SwaggerOptions)
	cfg.Components = deepCopyAnyMap(cfg.Components)
	cfg.SecuritySchemes = deepCopyAnyMap(cfg.SecuritySchemes)
	cfg.Webhooks = deepCopyAnyMap(cfg.Webhooks)
	cfg.Servers = slices.Clone(cfg.Servers)
	for i := range cfg.Servers {
		// maps.Clone is shallow and every ServerVariable carries an Enum slice,
		// so the values are rebuilt to detach them too.
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
	cfg.Security = cloneSecurityRequirements(cfg.Security)
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
	// With the defaults disabled the media types stay as given, empty when
	// unset, and nothing downstream documents a type nobody declared.
	if !cfg.DisableDefaultMediaTypes {
		if cfg.DefaultProduces == "" {
			cfg.DefaultProduces = ConfigDefault.DefaultProduces
		}
		if cfg.DefaultConsumes == "" {
			cfg.DefaultConsumes = ConfigDefault.DefaultConsumes
		}
		if cfg.ErrorProduces == "" {
			cfg.ErrorProduces = ConfigDefault.ErrorProduces
		}
	}
	if schema, ok := cfg.ErrorSchema.(map[string]any); ok {
		cfg.ErrorSchema = deepCopyAnyMap(schema)
	}
	switch cfg.OpenAPIVersion {
	case versionOpenAPI30, versionOpenAPI31, versionOpenAPI32:
		// supported
	default:
		cfg.OpenAPIVersion = ConfigDefault.OpenAPIVersion
	}
	return cfg
}
