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

	// Operations allows providing per-route metadata keyed by
	// "METHOD /path" (e.g. "GET /users").
	//
	// Optional. Default: nil
	Operations map[string]Operation
}

// ConfigDefault is the default config.
var ConfigDefault = Config{
	Next:        nil,
	Title:       "Fiber API",
	Version:     "1.0.0",
	Description: "",
	ServerURL:   "",
	Path:        "/openapi.json",
	Operations:  nil,
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
	Id          string
	Summary     string
	Description string
	Tags        []string
	Deprecated  bool
	// Consumes defines the request media type.
	Consumes string
	// Produces defines the response media type.
	Produces string
}
