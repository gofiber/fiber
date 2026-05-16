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

type pathVariant struct {
	PathParamAliases map[string]string
	Path             string
	ParamNames       []string
}

type resolvedParamName struct {
	openAPI string
	raw     string
}

const wildcardParamName = "wildcard"

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

			variants := buildOpenAPIPathVariants(r.Path, r.Params)
			for _, variant := range variants {
				params := make([]parameter, 0, len(variant.ParamNames))
				paramIndex := make(map[string]int, len(variant.ParamNames))
				for _, p := range variant.ParamNames {
					param := parameter{
						Name:     p,
						In:       "path",
						Required: true,
						Schema:   map[string]any{"type": "string"},
					}
					params = append(params, param)
					paramIndex[param.In+":"+param.Name] = len(params) - 1
				}

				extras := remapRouteParameters(r.Parameters, variant.PathParamAliases, variant.ParamNames)
				params = mergeRouteParameters(params, paramIndex, extras)

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

				methodLower := utilsstrings.ToLower(r.Method)
				if paths[variant.Path] == nil {
					paths[variant.Path] = make(map[string]operation)
				}

				paths[variant.Path][methodLower] = operation{
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
	}

	spec := openAPISpec{
		OpenAPI: cfg.OpenAPIVersion,
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

func remapRouteParameters(extras []fiber.RouteParameter, aliases map[string]string, pathParams []string) []fiber.RouteParameter {
	if len(extras) == 0 {
		return nil
	}
	pathSet := make(map[string]struct{}, len(pathParams))
	for _, name := range pathParams {
		pathSet[name] = struct{}{}
	}
	out := make([]fiber.RouteParameter, 0, len(extras))
	for _, extra := range extras {
		copyExtra := extra
		location := strings.ToLower(strings.TrimSpace(copyExtra.In))
		if location == "path" {
			if mapped, ok := aliases[copyExtra.Name]; ok {
				copyExtra.Name = mapped
			}
			if _, ok := pathSet[copyExtra.Name]; !ok {
				continue
			}
		}
		out = append(out, copyExtra)
	}
	return out
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

// convertToOpenAPIPath converts a Fiber route path pattern to one OpenAPI path template.
// When the path contains optional parameters and therefore yields multiple variants,
// this helper returns the first generated variant for backward compatibility.
func convertToOpenAPIPath(fiberPath string, params []string) string {
	variants := buildOpenAPIPathVariants(fiberPath, params)
	if len(variants) == 0 {
		return fiberPath
	}
	return variants[0].Path
}

func buildOpenAPIPathVariants(fiberPath string, params []string) []pathVariant {
	type state struct {
		aliases  map[string]string
		path     string
		params   []string
		paramIdx int
	}

	var (
		length   = len(fiberPath)
		variants []pathVariant
	)

	var walk func(i int, current state)
	walk = func(i int, current state) {
		for i < length {
			switch fiberPath[i] {
			case ':':
				tokenStart := i + 1
				i = tokenStart
				for i < length {
					c := fiberPath[i]
					if c == '<' || c == '?' || c == '/' || c == '-' || c == '.' {
						break
					}
					i++
				}
				tokenName := fiberPath[tokenStart:i]

				if i < length && fiberPath[i] == '<' {
					depth := 1
					i++
					for i < length && depth > 0 {
						switch fiberPath[i] {
						case '<':
							depth++
						case '>':
							depth--
						default:
						}
						i++
					}
				}

				isOptional := i < length && fiberPath[i] == '?'
				if isOptional {
					i++
				}

				resolved := resolveOpenAPIPathParamName(current.paramIdx, tokenName, params)
				includeState := clonePathState(current)
				includeState.path += "{" + resolved.openAPI + "}"
				includeState.params = append(includeState.params, resolved.openAPI)
				includeState.aliases[resolved.raw] = resolved.openAPI
				if tokenName != "" {
					includeState.aliases[tokenName] = resolved.openAPI
				}
				includeState.paramIdx++

				if isOptional {
					excludeState := clonePathState(current)
					if strings.HasSuffix(excludeState.path, "/") && (i == length || fiberPath[i] == '/') && len(excludeState.path) > 1 {
						excludeState.path = strings.TrimSuffix(excludeState.path, "/")
					}
					excludeState.paramIdx++
					walk(i, includeState)
					walk(i, excludeState)
					return
				}
				current = includeState

			case '*', '+':
				resolved := resolveOpenAPIWildcardParamName(current.paramIdx, params)
				current.path += "{" + resolved.openAPI + "}"
				current.params = append(current.params, resolved.openAPI)
				current.aliases[resolved.raw] = resolved.openAPI
				current.paramIdx++
				i++

			default:
				current.path += string(fiberPath[i])
				i++
			}
		}

		finalPath := current.path
		if finalPath == "" {
			finalPath = "/"
		}
		variants = append(variants, pathVariant{
			Path:             finalPath,
			ParamNames:       append([]string(nil), current.params...),
			PathParamAliases: current.aliases,
		})
	}

	walk(0, state{
		path:     "",
		params:   nil,
		aliases:  map[string]string{},
		paramIdx: 0,
	})

	seen := make(map[string]struct{}, len(variants))
	unique := make([]pathVariant, 0, len(variants))
	for _, variant := range variants {
		key := variant.Path + "|" + strings.Join(variant.ParamNames, ",")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, variant)
	}

	return unique
}

func clonePathState(current struct {
	aliases  map[string]string
	path     string
	params   []string
	paramIdx int
}) struct {
	aliases  map[string]string
	path     string
	params   []string
	paramIdx int
} {
	aliases := make(map[string]string, len(current.aliases))
	maps.Copy(aliases, current.aliases)
	return struct {
		aliases  map[string]string
		path     string
		params   []string
		paramIdx int
	}{
		path:     current.path,
		params:   append([]string(nil), current.params...),
		aliases:  aliases,
		paramIdx: current.paramIdx,
	}
}

func resolveOpenAPIPathParamName(paramIdx int, extracted string, params []string) resolvedParamName {
	raw := extracted
	if paramIdx < len(params) && params[paramIdx] != "" {
		raw = params[paramIdx]
	}
	if raw == "" {
		raw = extracted
	}
	return resolvedParamName{
		raw:     raw,
		openAPI: sanitizeOpenAPIPathParamName(raw, paramIdx+1),
	}
}

func resolveOpenAPIWildcardParamName(paramIdx int, params []string) resolvedParamName {
	raw := wildcardParamName
	if paramIdx < len(params) && params[paramIdx] != "" {
		raw = params[paramIdx]
	}
	return resolvedParamName{
		raw:     raw,
		openAPI: sanitizeOpenAPIWildcardParamName(raw, paramIdx+1),
	}
}

func sanitizeOpenAPIPathParamName(name string, idx int) string {
	return sanitizeOpenAPIParamName(name, idx)
}

func sanitizeOpenAPIWildcardParamName(name string, idx int) string {
	trimmed := strings.TrimLeft(name, "*+")
	if trimmed == "" {
		trimmed = wildcardParamName
	}
	trimmed = strings.TrimLeft(trimmed, "_.-")
	if trimmed == "" {
		trimmed = wildcardParamName
	}
	if trimmed[0] >= '0' && trimmed[0] <= '9' {
		trimmed = wildcardParamName + trimmed
	}
	if !strings.HasPrefix(trimmed, wildcardParamName) {
		trimmed = wildcardParamName + trimmed
	}
	return sanitizeOpenAPIParamName(trimmed, idx)
}

func sanitizeOpenAPIParamName(name string, idx int) string {
	trimmed := strings.TrimLeft(name, "*+")
	if trimmed == "" {
		trimmed = name
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-' {
			_, _ = builder.WriteRune(r) //nolint:errcheck // strings.Builder.WriteRune never returns an error
			continue
		}
		_ = builder.WriteByte('_') //nolint:errcheck // strings.Builder.WriteByte never returns an error
	}
	sanitized := builder.String()
	if sanitized == "" {
		return fmt.Sprintf("param%d", idx)
	}
	return sanitized
}
