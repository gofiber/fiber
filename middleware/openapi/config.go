package openapi

import (
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

// Config defines the config for middleware.
type Config struct {
	// ErrorSchema is the schema, or a Go value reflected into one, documented for error responses. Optional. Default: nil
	ErrorSchema any

	// ExternalDocs references external documentation for the API. Optional. Default: nil
	ExternalDocs *ExternalDocs

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
	return cfg
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
