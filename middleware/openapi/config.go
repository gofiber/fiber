package openapi

import (
	"maps"

	"github.com/gofiber/fiber/v3"
)

// Config defines the config for middleware.
type Config struct {
	// Config controls top-level OpenAPI document metadata only.
	// Operation-level metadata is derived from route helper methods.
	//
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

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

	// ServerURL is the server URL used in the generated specification.
	//
	// Optional. Default: ""
	ServerURL string

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

	// SwaggerBundleURL is the script URL used by the generated Swagger UI page.
	//
	// Optional. Default: "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-bundle.js"
	SwaggerBundleURL string

	// OpenAPIVersion specifies the OpenAPI specification version to generate.
	// Supported values: "3.0.0", "3.1.0" (default)
	//
	// Optional. Default: "3.1.0"
	OpenAPIVersion string
}

// ConfigDefault is the default config.
var ConfigDefault = Config{
	Next:             nil,
	Title:            "Fiber API",
	Version:          "1.0.0",
	Description:      "",
	ServerURL:        "",
	Path:             "/openapi.json",
	UIPath:           "/swagger",
	SwaggerCSSURL:    "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui.css",
	SwaggerBundleURL: "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-bundle.js",
	SwaggerOptions:   nil,
	OpenAPIVersion:   "3.1.0",
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
	if cfg.SwaggerOptions != nil {
		cfg.SwaggerOptions = maps.Clone(cfg.SwaggerOptions)
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
