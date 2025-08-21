package openapi

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// New creates a new middleware handler that serves the generated OpenAPI specification.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	var (
		data   []byte
		once   sync.Once
		genErr error
	)

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if !strings.HasSuffix(c.Path(), cfg.Path) {
			return c.Next()
		}

		once.Do(func() {
			spec := generateSpec(c.App(), cfg)
			data, genErr = json.Marshal(spec)
		})
		if genErr != nil {
			return genErr
		}
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		return c.Status(fiber.StatusOK).Send(data)
	}
}

type openAPISpec struct {
	OpenAPI string                          `json:"openapi"`
	Info    openAPIInfo                     `json:"info"`
	Servers []openAPIServer                 `json:"servers,omitempty"`
	Paths   map[string]map[string]operation `json:"paths"`
}

type openAPIInfo struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

type openAPIServer struct {
	URL string `json:"url"`
}

type operation struct {
	OperationID string              `json:"operationId,omitempty"`
	Summary     string              `json:"summary"`
	Description string              `json:"description"`
	Tags        []string            `json:"tags,omitempty"`
	Deprecated  bool                `json:"deprecated,omitempty"`
	Parameters  []parameter         `json:"parameters,omitempty"`
	Responses   map[string]response `json:"responses"`
}

type response struct {
	Description string                    `json:"description"`
	Content     map[string]map[string]any `json:"content,omitempty"`
}

type parameter struct {
	Name     string            `json:"name"`
	In       string            `json:"in"`
	Required bool              `json:"required"`
	Schema   map[string]string `json:"schema"`
}

func generateSpec(app *fiber.App, cfg Config) openAPISpec {
	paths := make(map[string]map[string]operation)
	stack := app.Stack()

	for _, routes := range stack {
		for _, r := range routes {
			if r.Method == fiber.MethodConnect {
				continue
			}

			path := r.Path
			var params []parameter
			if len(r.Params) > 0 {
				for _, p := range r.Params {
					path = strings.Replace(path, ":"+p, "{"+p+"}", 1)
					params = append(params, parameter{
						Name:     p,
						In:       "path",
						Required: true,
						Schema:   map[string]string{"type": "string"},
					})
				}
			}

			method := utils.ToLower(r.Method)
			if paths[path] == nil {
				paths[path] = make(map[string]operation)
			}

			key := r.Method + " " + r.Path
			meta := cfg.Operations[key]

			summary := meta.Summary
			if summary == "" {
				summary = r.Summary
			}
			if summary == "" {
				summary = r.Method + " " + r.Path
			}
			description := meta.Description
			if description == "" {
				description = r.Description
			}

			respType := meta.MediaType
			if respType == "" {
				respType = r.MediaType
			}
			resp := response{Description: "OK"}
			if respType != "" {
				resp.Content = map[string]map[string]any{
					respType: {},
				}
			}

			opID := meta.OperationID
			if opID == "" {
				opID = r.Name
			}
			paths[path][method] = operation{
				OperationID: opID,
				Summary:     summary,
				Description: description,
				Tags:        meta.Tags,
				Deprecated:  meta.Deprecated,
				Parameters:  params,
				Responses: map[string]response{
					"200": resp,
				},
			}
		}
	}

	spec := openAPISpec{
		OpenAPI: "3.0.0",
		Info: openAPIInfo{
			Title:       cfg.Title,
			Version:     cfg.Version,
			Description: cfg.Description,
		},
		Paths: paths,
	}
	if cfg.ServerURL != "" {
		spec.Servers = []openAPIServer{{URL: cfg.ServerURL}}
	}
	return spec
}
