package openapi

import (
	"github.com/gofiber/fiber/v3"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// Operations allows providing per-route metadata keyed by
	// "METHOD /path" (e.g. "GET /users").
	//
	// Optional. Default: nil
	Operations map[string]Operation

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
}

// ConfigDefault is the default config.
var ConfigDefault = Config{
	Next:        nil,
	Operations:  nil,
	Title:       "Fiber API",
	Version:     "1.0.0",
	Description: "",
	ServerURL:   "",
	Path:        "/openapi.json",
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
	if cfg.Operations == nil {
		cfg.Operations = ConfigDefault.Operations
	}

	return cfg
}

// Operation configures metadata for a single route in the generated spec.
type Operation struct {
	RequestBody *RequestBody
	Responses   map[string]Response
	Parameters  []Parameter
	Tags        []string

	ID          string
	Summary     string
	Description string
	Consumes    string
	Produces    string
	Deprecated  bool
}

// Parameter describes a single OpenAPI parameter.
type Parameter struct {
	Schema    map[string]any
	SchemaRef string
	Examples  map[string]any
	Example   any

	Name        string
	In          string
	Description string
	Required    bool
}

// Media describes the schema payload for a request or response media type.
type Media struct {
	Schema    map[string]any
	SchemaRef string
	Examples  map[string]any
	Example   any
}

// Response describes an OpenAPI response object.
type Response struct {
	Content     map[string]Media
	Examples    map[string]any
	Example     any
	SchemaRef   string
	Description string
}

// RequestBody describes the request body configuration for an operation.
type RequestBody struct {
	Content     map[string]Media
	Examples    map[string]any
	Example     any
	SchemaRef   string
	Description string
	Required    bool
}
