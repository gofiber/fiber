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

// Config defines the config for middleware. It controls top-level document
// metadata only; operation metadata comes from the route helpers.
type Config struct {
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

	// Tags lists top-level tag definitions (with descriptions) used by operations. Optional. Default: nil
	Tags []Tag

	// Security lists document-level requirements, combined with OR semantics. Optional. Default: nil
	Security []map[string][]string

	// Servers lists the servers hosting the API; it takes precedence over ServerURL. Optional. Default: nil
	Servers []Server
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
	SwaggerOptions:             nil,
	OpenAPIVersion:             versionOpenAPI31,
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
	if cfg.Description == "" {
		cfg.Description = ConfigDefault.Description
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = ConfigDefault.ServerURL
	}
	if cfg.Path == "" {
		cfg.Path = ConfigDefault.Path
	}
	if cfg.UIPath == "" {
		cfg.UIPath = ConfigDefault.UIPath
	}
	if cfg.SwaggerCSSURL == "" {
		cfg.SwaggerCSSURL = ConfigDefault.SwaggerCSSURL
	}
	if cfg.SwaggerBundleURL == "" {
		cfg.SwaggerBundleURL = ConfigDefault.SwaggerBundleURL
	}
	if cfg.SwaggerStandalonePresetURL == "" {
		cfg.SwaggerStandalonePresetURL = ConfigDefault.SwaggerStandalonePresetURL
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
	switch cfg.OpenAPIVersion {
	case versionOpenAPI30, versionOpenAPI31, versionOpenAPI32:
		// supported
	default:
		cfg.OpenAPIVersion = ConfigDefault.OpenAPIVersion
	}
	return cfg
}
