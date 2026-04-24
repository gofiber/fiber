package openapi

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	utilsstrings "github.com/gofiber/utils/v2/strings"
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

		targetPath := resolvedSpecPath(c, cfg.Path)
		if c.Path() != targetPath || (c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead) {
			return c.Next()
		}

		once.Do(func() {
			spec := generateSpec(c.App(), &cfg)
			data, genErr = json.Marshal(spec)
			if genErr != nil {
				genErr = fmt.Errorf("openapi: marshal spec: %w", genErr)
			}
		})
		if genErr != nil {
			return genErr
		}
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		return c.Status(fiber.StatusOK).Send(data)
	}
}

func resolvedSpecPath(c fiber.Ctx, cfgPath string) string {
	path := cfgPath
	if path == "" {
		path = ConfigDefault.Path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	route := c.Route()
	if route == nil {
		return path
	}

	prefix := route.Path
	if idx := strings.Index(prefix, "*"); idx >= 0 {
		prefix = prefix[:idx]
	}
	if prefix == "/" || prefix == "" {
		return path
	}
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return path
	}

	return prefix + path
}

type openAPISpec struct {
	Paths   map[string]map[string]operation `json:"paths"`
	Info    openAPIInfo                     `json:"info"`
	OpenAPI string                          `json:"openapi"`
	Servers []openAPIServer                 `json:"servers,omitempty"`
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
	Responses   map[string]response `json:"responses"`
	RequestBody *requestBody        `json:"requestBody,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase

	OperationID string      `json:"operationId,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	Parameters  []parameter `json:"parameters,omitempty"`
	Tags        []string    `json:"tags,omitempty"`

	Deprecated bool `json:"deprecated,omitempty"`
}

type response struct {
	Content     map[string]map[string]any `json:"content,omitempty"`
	Description string                    `json:"description"`
}

type parameter struct {
	Schema      map[string]any `json:"schema,omitempty"`
	Example     any            `json:"example,omitempty"`
	Examples    map[string]any `json:"examples,omitempty"`
	Description string         `json:"description,omitempty"`
	Name        string         `json:"name"`
	In          string         `json:"in"`
	Required    bool           `json:"required"`
}

type requestBody struct {
	Content     map[string]map[string]any `json:"content"`
	Description string                    `json:"description,omitempty"`
	Required    bool                      `json:"required,omitempty"`
}

func generateSpec(app *fiber.App, cfg *Config) openAPISpec {
	paths := make(map[string]map[string]operation)
	stack := app.Stack()

	for _, routes := range stack {
		for _, r := range routes {
			if r.Method == fiber.MethodConnect {
				continue
			}
			// Skip middleware routes registered via Use()
			if r.IsMiddleware() {
				continue
			}
			// Skip automatically generated HEAD routes
			if r.IsAutoHead() {
				continue
			}

			path := convertToOpenAPIPath(r.Path, r.Params)
			params := make([]parameter, 0, len(r.Params))
			paramIndex := make(map[string]int, len(r.Params))
			if len(r.Params) > 0 {
				for _, p := range r.Params {
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

			methodLower := utilsstrings.ToLower(r.Method)
			if paths[path] == nil {
				paths[path] = make(map[string]operation)
			}

			params = mergeRouteParameters(params, paramIndex, r.Parameters)

			summary := r.Summary
			if summary == "" {
				summary = r.Method + " " + r.Path
			}
			description := r.Description

			respType := r.Produces

			responses := convertRouteResponses(r.Responses)
			if len(responses) == 0 {
				status, defaultResp := defaultResponseForMethod(r.Method, respType)
				responses = map[string]response{status: defaultResp}
			}

			reqBody := buildRequestBody(r.RequestBody)
			if reqBody == nil {
				reqType := r.Consumes
				if shouldIncludeRequestBody(reqType, r) {
					reqBody = &requestBody{Content: map[string]map[string]any{reqType: {}}}
				}
			}

			paths[path][methodLower] = operation{
				OperationID: r.Name,
				Summary:     summary,
				Description: description,
				Tags:        r.Tags,
				Deprecated:  r.Deprecated,
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
			Schema:      schemaFrom(extra.Schema, extra.SchemaRef, "string"),
			Example:     extra.Example,
			Examples:    copyAnyMap(extra.Examples),
		}
		if param.In == "path" {
			param.Required = true
		}
		params = appendOrReplaceParameter(params, index, &param)
	}
	return params
}

func appendOrReplaceParameter(params []parameter, index map[string]int, p *parameter) []parameter {
	if p == nil || p.Name == "" || p.In == "" {
		return params
	}
	key := p.In + ":" + p.Name
	if idx, ok := index[key]; ok {
		params[idx] = *p
		return params
	}
	index[key] = len(params)
	return append(params, *p)
}

func copyAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	maps.Copy(dst, src)
	return dst
}

func schemaFrom(schema map[string]any, schemaRef, defaultType string) map[string]any {
	if schemaRef != "" {
		return map[string]any{"$ref": schemaRef}
	}

	copied := copyAnyMap(schema)
	if copied == nil {
		copied = map[string]any{}
	}
	if _, ok := copied["type"]; !ok && defaultType != "" {
		copied["type"] = defaultType
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

func contentEntry(schema map[string]any, schemaRef string, example any, examples map[string]any) map[string]any {
	entry := map[string]any{}
	if schemaRef != "" {
		entry["schema"] = map[string]any{"$ref": schemaRef}
	} else if copied := copyAnyMap(schema); len(copied) > 0 {
		entry["schema"] = copied
	}
	if example != nil {
		entry["example"] = example
	}
	if ex := copyAnyMap(examples); len(ex) > 0 {
		entry["examples"] = ex
	}
	return entry
}

func convertRouteResponses(routeResponses map[string]fiber.RouteResponse) map[string]response {
	var merged map[string]response
	if len(routeResponses) > 0 {
		merged = make(map[string]response, len(routeResponses))
		for code, resp := range routeResponses {
			merged[code] = response{
				Description: resp.Description,
				Content:     mediaTypesToContent(resp.MediaTypes, resp.Schema, resp.SchemaRef, resp.Example, resp.Examples),
			}
		}
	}
	return merged
}

func mediaTypesToContent(mediaTypes []string, schema map[string]any, schemaRef string, example any, examples map[string]any) map[string]map[string]any {
	if len(mediaTypes) == 0 {
		return nil
	}
	content := make(map[string]map[string]any, len(mediaTypes))
	for _, mediaType := range mediaTypes {
		if mediaType == "" {
			continue
		}
		entry := contentEntry(schema, schemaRef, example, examples)
		if len(entry) == 0 {
			entry = map[string]any{}
		}
		content[mediaType] = entry
	}
	if len(content) == 0 {
		return nil
	}
	return content
}

func buildRequestBody(routeBody *fiber.RouteRequestBody) *requestBody {
	if routeBody == nil {
		return nil
	}
	merged := &requestBody{
		Description: routeBody.Description,
		Required:    routeBody.Required,
		Content:     mediaTypesToContent(routeBody.MediaTypes, routeBody.Schema, routeBody.SchemaRef, routeBody.Example, routeBody.Examples),
	}
	// Omit requestBody entirely when content could not be built, as the
	// OpenAPI specification requires at least one media type in content.
	if len(merged.Content) == 0 {
		return nil
	}
	return merged
}

// shouldIncludeRequestBody returns true when an implicit request body should be
// added for a route without explicit request body metadata. A nil route always
// returns false.
func shouldIncludeRequestBody(reqType string, route *fiber.Route) bool {
	if reqType == "" || route == nil {
		return false
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

func defaultResponseForMethod(method, mediaType string) (string, response) {
	status := "200"
	description := "OK"

	switch method {
	case fiber.MethodDelete, fiber.MethodHead:
		status = "204"
		description = "No Content"
	default:
		// Keep default 200/OK status
	}

	resp := response{Description: description}
	if mediaType != "" && status != "204" {
		resp.Content = map[string]map[string]any{
			mediaType: {},
		}
	}
	return status, resp
}

// convertToOpenAPIPath converts Fiber route path patterns to OpenAPI path templates.
// It handles parameter constraints (:id<int>), wildcards (*), plus params (+), and optional markers (?).
// Examples:
//   - /users/:id<int> -> /users/{id}
//   - /files/* -> /files/{wildcard}
//   - /items/:id? -> /items/{id}
//   - /posts/:slug+ -> /posts/{slug}
func convertToOpenAPIPath(fiberPath string, params []string) string {
	if len(params) == 0 && !strings.ContainsAny(fiberPath, ":*+") {
		return fiberPath
	}

	// Build a map of parameter names for quick lookup
	paramSet := make(map[string]struct{}, len(params))
	for _, p := range params {
		paramSet[p] = struct{}{}
	}

	var result strings.Builder
	result.Grow(len(fiberPath))
	i := 0
	length := len(fiberPath)

	for i < length {
		ch := fiberPath[i]

		switch ch {
		case ':':
			// Named parameter - extract name until we hit a constraint, optional marker, or delimiter
			i++
			paramStart := i
			for i < length {
				c := fiberPath[i]
				if c == '<' || c == '?' || c == '/' || c == '-' || c == '.' {
					break
				}
				i++
			}
			paramName := fiberPath[paramStart:i]

			// Skip constraints like <int>, <regex(...)>, etc.
			if i < length && fiberPath[i] == '<' {
				depth := 1
				i++ // skip '<'
				for i < length && depth > 0 {
					switch fiberPath[i] {
					case '<':
						depth++
					case '>':
						depth--
					default:
						// Other characters inside constraints are ignored
					}
					i++
				}
			}

			// Skip optional marker '?'
			if i < length && fiberPath[i] == '?' {
				i++
			}

			// Write OpenAPI parameter placeholder
			if paramName != "" {
				_ = result.WriteByte('{')            //nolint:errcheck // strings.Builder.WriteByte never returns an error
				_, _ = result.WriteString(paramName) //nolint:errcheck // strings.Builder.WriteString never returns an error
				_ = result.WriteByte('}')            //nolint:errcheck // strings.Builder.WriteByte never returns an error
			}

		case '*', '+':
			// Wildcard or plus param - use a generic name
			// In Fiber, * and + are greedy params that match everything
			// We represent them as {wildcard} or the named param if it exists
			_, _ = result.WriteString("{wildcard}") //nolint:errcheck // strings.Builder.WriteString never returns an error
			i++

		default:
			_ = result.WriteByte(ch) //nolint:errcheck // strings.Builder.WriteByte never returns an error
			i++
		}
	}

	return result.String()
}
