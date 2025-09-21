package openapi

import (
	"encoding/json"
	"maps"
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
	RequestBody *requestBody        `json:"requestBody,omitempty"`
	Responses   map[string]response `json:"responses"`
}

type response struct {
	Description string                    `json:"description"`
	Content     map[string]map[string]any `json:"content,omitempty"`
}

type parameter struct {
	Name        string         `json:"name"`
	In          string         `json:"in"`
	Required    bool           `json:"required"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
}

type requestBody struct {
	Description string                    `json:"description,omitempty"`
	Required    bool                      `json:"required,omitempty"`
	Content     map[string]map[string]any `json:"content"`
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
			params := make([]parameter, 0, len(r.Params))
			paramIndex := make(map[string]int, len(r.Params))
			if len(r.Params) > 0 {
				for _, p := range r.Params {
					path = strings.Replace(path, ":"+p, "{"+p+"}", 1)
					param := parameter{
						Name:     p,
						In:       "path",
						Required: true,
						Schema:   map[string]any{"type": "string"},
					}
					params = append(params, param)
					paramIndex[param.In+":"+param.Name] = len(params) - 1
				}
			}

			methodLower := utils.ToLower(r.Method)
			if paths[path] == nil {
				paths[path] = make(map[string]operation)
			}

			key := r.Method + " " + r.Path
			meta := cfg.Operations[key]

			params = mergeRouteParameters(params, paramIndex, r.Parameters)
			params = mergeConfigParameters(params, paramIndex, meta.Parameters)

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

			respType := meta.Produces
			if respType == "" {
				respType = r.Produces
			}

			responses := mergeResponses(r.Responses, meta.Responses)
			if responses == nil {
				responses = make(map[string]response)
			}
			defaultResp, exists := responses["200"]
			if defaultResp.Description == "" {
				defaultResp.Description = "OK"
			}
			if !exists && respType != "" {
				defaultResp.Content = map[string]map[string]any{
					respType: {},
				}
			}
			responses["200"] = defaultResp

			reqBody := buildRequestBody(r.RequestBody, meta.RequestBody)
			if reqBody == nil {
				reqType := meta.Consumes
				if reqType == "" {
					reqType = r.Consumes
				}
				if shouldIncludeRequestBody(reqType, meta, r) {
					reqBody = &requestBody{Content: map[string]map[string]any{reqType: {}}}
				}
			}

			opID := meta.Id
			if opID == "" {
				opID = r.Name
			}

			tags := meta.Tags
			if len(tags) == 0 {
				tags = r.Tags
			}

			deprecated := meta.Deprecated || r.Deprecated

			paths[path][methodLower] = operation{
				OperationID: opID,
				Summary:     summary,
				Description: description,
				Tags:        tags,
				Deprecated:  deprecated,
				Parameters:  params,
				RequestBody: reqBody,
				Responses:   responses,
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

func mergeRouteParameters(params []parameter, index map[string]int, extras []fiber.RouteParameter) []parameter {
	if len(extras) == 0 {
		return params
	}
	for _, extra := range extras {
		if strings.TrimSpace(extra.Name) == "" {
			continue
		}
		location := strings.ToLower(strings.TrimSpace(extra.In))
		if location == "" {
			location = "query"
		}
		param := parameter{
			Name:        extra.Name,
			In:          location,
			Description: extra.Description,
			Required:    extra.Required,
			Schema:      copyAnyMap(extra.Schema),
		}
		if param.Schema == nil {
			param.Schema = map[string]any{"type": "string"}
		}
		if param.In == "path" {
			param.Required = true
		}
		params = appendOrReplaceParameter(params, index, param)
	}
	return params
}

func mergeConfigParameters(params []parameter, index map[string]int, extras []Parameter) []parameter {
	if len(extras) == 0 {
		return params
	}
	for _, extra := range extras {
		if strings.TrimSpace(extra.Name) == "" {
			continue
		}
		location := strings.ToLower(strings.TrimSpace(extra.In))
		if location == "" {
			location = "query"
		}
		param := parameter{
			Name:        extra.Name,
			In:          location,
			Description: extra.Description,
			Required:    extra.Required,
			Schema:      copyAnyMap(extra.Schema),
		}
		if param.Schema == nil {
			param.Schema = map[string]any{"type": "string"}
		}
		if param.In == "path" {
			param.Required = true
		}
		params = appendOrReplaceParameter(params, index, param)
	}
	return params
}

func appendOrReplaceParameter(params []parameter, index map[string]int, p parameter) []parameter {
	if p.Name == "" || p.In == "" {
		return params
	}
	key := p.In + ":" + p.Name
	if idx, ok := index[key]; ok {
		params[idx] = p
		return params
	}
	index[key] = len(params)
	return append(params, p)
}

func copyAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	maps.Copy(dst, src)
	return dst
}

func mergeResponses(routeResponses map[string]fiber.RouteResponse, cfgResponses map[string]Response) map[string]response {
	var merged map[string]response
	if len(routeResponses) > 0 {
		merged = make(map[string]response, len(routeResponses))
		for code, resp := range routeResponses {
			merged[code] = response{
				Description: resp.Description,
				Content:     mediaTypesToContent(resp.MediaTypes),
			}
		}
	}
	if len(cfgResponses) > 0 {
		if merged == nil {
			merged = make(map[string]response, len(cfgResponses))
		}
		for code, resp := range cfgResponses {
			merged[code] = response{
				Description: resp.Description,
				Content:     convertMediaContent(resp.Content),
			}
		}
	}
	return merged
}

func convertMediaContent(content map[string]Media) map[string]map[string]any {
	if len(content) == 0 {
		return nil
	}
	converted := make(map[string]map[string]any, len(content))
	for mediaType, media := range content {
		entry := map[string]any{}
		if schema := copyAnyMap(media.Schema); len(schema) > 0 {
			entry["schema"] = schema
		}
		converted[mediaType] = entry
	}
	return converted
}

func mediaTypesToContent(mediaTypes []string) map[string]map[string]any {
	if len(mediaTypes) == 0 {
		return nil
	}
	content := make(map[string]map[string]any, len(mediaTypes))
	for _, mediaType := range mediaTypes {
		if mediaType == "" {
			continue
		}
		content[mediaType] = map[string]any{}
	}
	if len(content) == 0 {
		return nil
	}
	return content
}

func buildRequestBody(routeBody *fiber.RouteRequestBody, cfgBody *RequestBody) *requestBody {
	var merged *requestBody
	if routeBody != nil {
		merged = &requestBody{
			Description: routeBody.Description,
			Required:    routeBody.Required,
			Content:     mediaTypesToContent(routeBody.MediaTypes),
		}
	}
	if cfgBody != nil {
		cfgReq := &requestBody{
			Description: cfgBody.Description,
			Required:    cfgBody.Required,
			Content:     convertMediaContent(cfgBody.Content),
		}
		if merged == nil {
			merged = cfgReq
		} else {
			if cfgReq.Description != "" {
				merged.Description = cfgReq.Description
			}
			merged.Required = cfgReq.Required
			if len(cfgReq.Content) > 0 {
				if merged.Content == nil {
					merged.Content = cfgReq.Content
				} else {
					maps.Copy(merged.Content, cfgReq.Content)
				}
			}
		}
	}
	return merged
}

func shouldIncludeRequestBody(reqType string, meta Operation, route *fiber.Route) bool {
	if reqType == "" || route == nil {
		return false
	}
	if meta.Consumes != "" {
		return true
	}
	if route.Consumes != fiber.MIMETextPlain {
		return true
	}
	switch route.Method {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace:
		return false
	default:
		return true
	}
}
