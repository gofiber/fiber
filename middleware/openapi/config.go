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
}

// ConfigDefault is the default config.
var ConfigDefault = Config{
	Next:        nil,
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
	return cfg
}
