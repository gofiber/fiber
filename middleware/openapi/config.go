package openapi

import (
	"maps"

	"github.com/gofiber/fiber/v3"
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
	// URL is a URL to the license used for the API.
	URL string `json:"url,omitempty"`
}

// Server represents a server hosting the API.
type Server struct {
	// URL is the server URL.
	URL string `json:"url"`
	// Description is an optional description of the server.
	Description string `json:"description,omitempty"`
}

// Tag adds metadata to a single tag used by operations.
type Tag struct {
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

// Config defines the config for middleware.
//
// Config controls top-level OpenAPI document metadata only.
// Operation-level metadata is derived from route helper methods.
type Config struct {
	// ExternalDocs references external documentation for the API.
	//
	// Optional. Default: nil
	ExternalDocs *ExternalDocs

	// SwaggerOptions contains additional Swagger UI options merged into the
	// generated SwaggerUIBundle call.
	//
	// Optional. Default: nil
	SwaggerOptions map[string]any

	// Components holds reusable OpenAPI component definitions such as schemas,
	// responses, and parameters. These are emitted under the top-level
	// "components" key of the generated specification, allowing $ref references
	// (e.g. "#/components/schemas/User") to resolve correctly.
	//
	// Optional. Default: nil
	Components map[string]any

	// SecuritySchemes holds reusable security scheme definitions (e.g. bearer,
	// apiKey, oauth2). They are emitted under "components.securitySchemes" and
	// can be referenced by the Security field or the route-level Security helper.
	//
	// Optional. Default: nil
	SecuritySchemes map[string]any

	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// Contact holds contact information for the exposed API.
	//
	// Optional. Default: nil
	Contact *Contact

	// License holds license information for the exposed API.
	//
	// Optional. Default: nil
	License *License

	// TermsOfService is a URL to the Terms of Service for the API.
	//
	// Optional. Default: ""
	TermsOfService string

	// ServerURL is the server URL used in the generated specification.
	//
	// Optional. Default: ""
	ServerURL string

	// OpenAPIVersion specifies the OpenAPI specification version to generate.
	// Supported values: "3.0.0", "3.1.0" (default)
	//
	// Optional. Default: "3.1.0"
	OpenAPIVersion string

	// SwaggerStandalonePresetURL is the standalone preset script URL used by the
	// generated Swagger UI page. When non-empty, the page loads it and renders
	// with the "StandaloneLayout" (top bar with the Authorize button). Like the
	// other Swagger asset URLs it can be overridden to self-host the assets.
	//
	// Optional. Default: "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-standalone-preset.js"
	SwaggerStandalonePresetURL string

	// Title is the title for the generated OpenAPI specification.
	//
	// Optional. Default: "Fiber API"
	Title string

	// Version is the version for the generated OpenAPI specification.
	//
	// Optional. Default: "1.0.0"
	Version string

	// Description is the description for the generated OpenAPI specification.
	//
	// Optional. Default: ""
	Description string

	// SwaggerBundleURL is the script URL used by the generated Swagger UI page.
	//
	// Optional. Default: "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-bundle.js"
	SwaggerBundleURL string

	// Path is the route where the specification will be served.
	//
	// Optional. Default: "/openapi.json"
	Path string

	// UIPath is the route where the Swagger UI page will be served.
	//
	// Optional. Default: "/swagger"
	UIPath string

	// SwaggerCSSURL is the stylesheet URL used by the generated Swagger UI page.
	//
	// Optional. Default: "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui.css"
	SwaggerCSSURL string

	// Tags lists top-level tag definitions (with descriptions) used by operations.
	//
	// Optional. Default: nil
	Tags []Tag

	// Security defines the document-level (default) security requirements.
	// Each requirement maps a scheme name (declared in SecuritySchemes) to its
	// required scopes; multiple requirements are combined with OR semantics.
	//
	// Optional. Default: nil
	Security []map[string][]string

	// Servers lists the servers hosting the API. When set, it takes precedence
	// over ServerURL.
	//
	// Optional. Default: nil
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
	OpenAPIVersion:             "3.1.0",
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
	if cfg.SwaggerOptions != nil {
		cfg.SwaggerOptions = maps.Clone(cfg.SwaggerOptions)
	}
	if cfg.Components != nil {
		cfg.Components = maps.Clone(cfg.Components)
	}
	if cfg.SecuritySchemes != nil {
		cfg.SecuritySchemes = maps.Clone(cfg.SecuritySchemes)
	}
	if cfg.OpenAPIVersion == "" {
		cfg.OpenAPIVersion = ConfigDefault.OpenAPIVersion
	}
	// Normalize OpenAPI version to supported values
	if cfg.OpenAPIVersion != "3.0.0" && cfg.OpenAPIVersion != "3.1.0" {
		cfg.OpenAPIVersion = ConfigDefault.OpenAPIVersion
	}
	return cfg
}
