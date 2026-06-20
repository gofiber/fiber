package openapi

import (
	"encoding/json"
	"fmt"
	htemplate "html/template"
	"maps"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// New creates a new middleware handler that serves the generated OpenAPI specification.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	var (
		specMu      sync.Mutex
		specData    []byte
		specCount   = -1
		swaggerData []byte
		swaggerOnce sync.Once
		swaggerErr  error
	)

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			return c.Next()
		}

		targetPath := resolvedSpecPath(c, cfg.Path)
		targetUIPath := resolvedSpecPath(c, cfg.UIPath)

		switch {
		case pathMatches(c.Path(), targetPath):
			// The spec is cached but regenerated whenever the number of
			// registered routes changes, so routes added after the first
			// request are reflected without a process restart.
			specMu.Lock()
			count := routeCount(c.App())
			if specData == nil || specCount != count {
				spec := generateSpec(c.App(), &cfg)
				data, err := json.Marshal(spec)
				if err != nil {
					specMu.Unlock()
					return fmt.Errorf("openapi: marshal spec: %w", err)
				}
				specData = data
				specCount = count
			}
			data := specData
			specMu.Unlock()

			c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
			return c.Status(fiber.StatusOK).Send(data)
		case pathMatches(c.Path(), targetUIPath):
			swaggerOnce.Do(func() {
				swaggerData, swaggerErr = buildSwaggerUIPage(targetPath, &cfg)
				if swaggerErr != nil {
					swaggerErr = fmt.Errorf("openapi: build swagger ui page: %w", swaggerErr)
				}
			})
			if swaggerErr != nil {
				return swaggerErr
			}
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
			return c.Status(fiber.StatusOK).Send(swaggerData)
		default:
			return c.Next()
		}
	}
}

// routeCount returns the total number of routes registered on the app across
// all HTTP methods. It is used to invalidate the cached specification when
// routes are added or removed.
func routeCount(app *fiber.App) int {
	count := 0
	for _, routes := range app.Stack() {
		count += len(routes)
	}
	return count
}

// pathMatches reports whether the request path matches the configured target
// path, tolerating a single trailing slash on either side.
func pathMatches(requestPath, target string) bool {
	return trimTrailingSlash(requestPath) == trimTrailingSlash(target)
}

func trimTrailingSlash(p string) string {
	if len(p) > 1 && p[len(p)-1] == '/' {
		return p[:len(p)-1]
	}
	return p
}

type swaggerUITemplateData struct {
	SwaggerOptionsJSON         string
	Title                      string
	OpenAPIURL                 string
	SwaggerCSSURL              string
	SwaggerBundleURL           string
	SwaggerStandalonePresetURL string
}

var swaggerUITemplate = htemplate.Must(htemplate.New("swagger-ui").Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{{ .Title }} - Swagger UI</title>
    <link
      rel="stylesheet"
      href="{{ .SwaggerCSSURL }}"
    />
  </head>
  <body>
    <div id="swagger-ui" data-swagger-options='{{ .SwaggerOptionsJSON }}'></div>

    <script
      src="{{ .SwaggerBundleURL }}"
      crossorigin="anonymous"
    ></script>
    {{ if .SwaggerStandalonePresetURL }}<script
      src="{{ .SwaggerStandalonePresetURL }}"
      crossorigin="anonymous"
    ></script>{{ end }}
    <script>
      window.addEventListener("load", function () {
        const options = JSON.parse(document.getElementById("swagger-ui").dataset.swaggerOptions);

        const presets = [SwaggerUIBundle.presets.apis];
        const config = {
          url: "{{ .OpenAPIURL }}",
          dom_id: "#swagger-ui",
          persistAuthorization: true,
        };
        if (typeof SwaggerUIStandalonePreset !== "undefined") {
          presets.push(SwaggerUIStandalonePreset);
          config.layout = "StandaloneLayout";
        }
        config.presets = presets;

        window.ui = SwaggerUIBundle({
          ...config,
          ...options,
        });
      });
    </script>
  </body>
</html>
`))

func buildSwaggerUIPage(openAPIURL string, cfg *Config) ([]byte, error) {
	swaggerOptionsJSON, err := json.Marshal(cfg.SwaggerOptions)
	if err != nil {
		return nil, fmt.Errorf("marshal swagger options: %w", err)
	}
	if len(swaggerOptionsJSON) == 0 || string(swaggerOptionsJSON) == "null" {
		swaggerOptionsJSON = []byte("{}")
	}

	data := swaggerUITemplateData{
		Title:                      cfg.Title,
		OpenAPIURL:                 openAPIURL,
		SwaggerCSSURL:              cfg.SwaggerCSSURL,
		SwaggerBundleURL:           cfg.SwaggerBundleURL,
		SwaggerStandalonePresetURL: cfg.SwaggerStandalonePresetURL,
		SwaggerOptionsJSON:         string(swaggerOptionsJSON),
	}

	var builder strings.Builder
	if err := swaggerUITemplate.Execute(&builder, data); err != nil {
		return nil, fmt.Errorf("execute swagger ui template: %w", err)
	}

	return []byte(builder.String()), nil
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
	Paths             map[string]map[string]operation `json:"paths"`
	Components        map[string]any                  `json:"components,omitempty"`
	Webhooks          map[string]any                  `json:"webhooks,omitempty"`
	ExternalDocs      *ExternalDocs                   `json:"externalDocs,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Info              openAPIInfo                     `json:"info"`
	OpenAPI           string                          `json:"openapi"`
	JSONSchemaDialect string                          `json:"jsonSchemaDialect,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Servers           []openAPIServer                 `json:"servers,omitempty"`
	Security          []map[string][]string           `json:"security,omitempty"`
	Tags              []openAPITag                    `json:"tags,omitempty"`
}

type openAPIInfo struct {
	Contact        *Contact `json:"contact,omitempty"`
	License        *License `json:"license,omitempty"`
	Title          string   `json:"title"`
	Version        string   `json:"version"`
	Summary        string   `json:"summary,omitempty"`
	Description    string   `json:"description,omitempty"`
	TermsOfService string   `json:"termsOfService,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
}

type openAPIServer struct {
	Variables   map[string]ServerVariable `json:"variables,omitempty"`
	URL         string                    `json:"url"`
	Description string                    `json:"description,omitempty"`
}

type openAPITag struct {
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
}

type operation struct {
	Responses    map[string]response `json:"responses"`
	RequestBody  *requestBody        `json:"requestBody,omitempty"`  //nolint:tagliatelle // OpenAPI spec uses camelCase
	ExternalDocs map[string]any      `json:"externalDocs,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	// extensions holds arbitrary operation-object fields (servers, callbacks,
	// x-* extensions) merged at marshal time. Excluded from normal marshaling.
	extensions map[string]any

	OperationID string `json:"operationId,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Summary     string `json:"summary"`
	Description string `json:"description"`

	Parameters []parameter           `json:"parameters,omitempty"`
	Tags       []string              `json:"tags,omitempty"`
	Security   []map[string][]string `json:"security,omitempty"`

	Deprecated bool `json:"deprecated,omitempty"`
}

// MarshalJSON merges operation extensions into the operation object without
// clobbering generated keys.
//
// are not addressable) are marshaled through this method.
//
//nolint:gocritic // hugeParam: a value receiver is required so map values (which
func (o operation) MarshalJSON() ([]byte, error) {
	type alias operation
	base, err := json.Marshal(alias(o))
	if err != nil {
		return nil, fmt.Errorf("openapi: marshal operation: %w", err)
	}
	if len(o.extensions) == 0 {
		return base, nil
	}
	merged := map[string]any{}
	if err = json.Unmarshal(base, &merged); err != nil {
		return nil, fmt.Errorf("openapi: merge operation extensions: %w", err)
	}
	for key, value := range o.extensions {
		if _, exists := merged[key]; !exists {
			merged[key] = value
		}
	}
	out, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("openapi: marshal operation: %w", err)
	}
	return out, nil
}

type response struct {
	Content     map[string]map[string]any `json:"content,omitempty"`
	Headers     map[string]any            `json:"headers,omitempty"`
	Links       map[string]any            `json:"links,omitempty"`
	Description string                    `json:"description"`
}

type parameter struct {
	Schema          map[string]any `json:"schema,omitempty"`
	Example         any            `json:"example,omitempty"`
	Examples        map[string]any `json:"examples,omitempty"`
	Explode         *bool          `json:"explode,omitempty"`
	Description     string         `json:"description,omitempty"`
	Name            string         `json:"name"`
	In              string         `json:"in"`
	Style           string         `json:"style,omitempty"`
	Required        bool           `json:"required"`
	Deprecated      bool           `json:"deprecated,omitempty"`
	AllowEmptyValue bool           `json:"allowEmptyValue,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	AllowReserved   bool           `json:"allowReserved,omitempty"`   //nolint:tagliatelle // OpenAPI spec uses camelCase
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

const (
	paramLocationPath = "path"
	schemaKeyType     = "type"
	schemaKeyFormat   = "format"
	schemaTypeString  = "string"
	schemaTypeObject  = "object"
)

// generateOperationID derives a stable, readable operationId from an HTTP method
// and an OpenAPI path, e.g. ("GET", "/users/{id}") -> "getUsersId". It is used
// when a route has no explicit Name.
func generateOperationID(method, path string) string {
	var b strings.Builder
	_, _ = b.WriteString(utilsstrings.ToLower(method)) //nolint:errcheck // strings.Builder.WriteString never returns an error
	capNext := true
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch {
		case c >= 'a' && c <= 'z':
			if capNext {
				c -= 'a' - 'A'
				capNext = false
			}
			_ = b.WriteByte(c) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		case (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9'):
			capNext = false
			_ = b.WriteByte(c) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		default:
			capNext = true
		}
	}
	return b.String()
}

// uniqueOperationID returns id when unused, otherwise appends a numeric suffix
// until the result is unique, recording the chosen value in used so the
// generated document never repeats an operationId.
func uniqueOperationID(id string, used map[string]struct{}) string {
	if id == "" {
		id = "operation"
	}
	candidate := id
	for i := 2; ; i++ {
		if _, exists := used[candidate]; !exists {
			used[candidate] = struct{}{}
			return candidate
		}
		candidate = fmt.Sprintf("%s_%d", id, i)
	}
}

func generateSpec(app *fiber.App, cfg *Config) openAPISpec {
	paths := make(map[string]map[string]operation)
	// usedOperationIDs guarantees operationId uniqueness across the document,
	// which the OpenAPI specification requires.
	usedOperationIDs := make(map[string]struct{})
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
			// Skip routes explicitly excluded from the spec via Hidden()
			if r.IsHidden() {
				continue
			}

			variants := buildOpenAPIPathVariants(r.Path, r.Params)
			for _, variant := range variants {
				params := make([]parameter, 0, len(variant.ParamNames))
				paramIndex := make(map[string]int, len(variant.ParamNames))
				for _, p := range variant.ParamNames {
					param := parameter{
						Name:     p,
						In:       paramLocationPath,
						Required: true,
						Schema:   map[string]any{schemaKeyType: schemaTypeString},
					}
					params = append(params, param)
					paramIndex[param.In+":"+param.Name] = len(params) - 1
				}

				extras := remapRouteParameters(r.Parameters, variant.PathParamAliases, variant.ParamNames)
				params = mergeRouteParameters(params, paramIndex, extras)

				summary := r.Summary
				if summary == "" {
					summary = r.Method + " " + variant.Path
				}
				description := r.Description

				operationID := r.Name
				if operationID == "" {
					operationID = generateOperationID(r.Method, variant.Path)
				}
				operationID = uniqueOperationID(operationID, usedOperationIDs)

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
				// GET and HEAD operations never carry a request body.
				if r.Method == fiber.MethodGet || r.Method == fiber.MethodHead {
					reqBody = nil
				}

				methodLower := utilsstrings.ToLower(r.Method)
				if paths[variant.Path] == nil {
					paths[variant.Path] = make(map[string]operation)
				}

				paths[variant.Path][methodLower] = operation{
					OperationID:  operationID,
					Summary:      summary,
					Description:  description,
					Tags:         r.Tags,
					Deprecated:   r.Deprecated,
					Parameters:   params,
					RequestBody:  reqBody,
					Responses:    responses,
					Security:     r.Security,
					ExternalDocs: copyAnyMap(r.ExternalDocs),
					extensions:   copyAnyMap(r.OperationExtensions),
				}
			}
		}
	}

	spec := openAPISpec{
		OpenAPI: cfg.OpenAPIVersion,
		Info: openAPIInfo{
			Title:          cfg.Title,
			Version:        cfg.Version,
			Description:    cfg.Description,
			TermsOfService: cfg.TermsOfService,
			Contact:        cfg.Contact,
			License:        cfg.License,
		},
		Paths: paths,
	}

	spec.Servers = buildServers(cfg)

	if len(cfg.Security) > 0 {
		spec.Security = cfg.Security
	}

	if len(cfg.Tags) > 0 {
		tags := make([]openAPITag, 0, len(cfg.Tags))
		for _, tag := range cfg.Tags {
			tags = append(tags, openAPITag(tag))
		}
		spec.Tags = tags
	}

	if cfg.ExternalDocs != nil {
		spec.ExternalDocs = &ExternalDocs{
			Description: cfg.ExternalDocs.Description,
			URL:         cfg.ExternalDocs.URL,
		}
	}

	// OpenAPI 3.1-only document fields.
	if cfg.OpenAPIVersion == versionOpenAPI31 {
		spec.Info.Summary = cfg.Summary
		spec.JSONSchemaDialect = cfg.JSONSchemaDialect
		if len(cfg.Webhooks) > 0 {
			spec.Webhooks = maps.Clone(cfg.Webhooks)
		}
	}

	spec.Components = buildComponents(cfg)

	return spec
}

// buildServers resolves the server list, preferring Config.Servers and falling
// back to the single Config.ServerURL for backward compatibility.
func buildServers(cfg *Config) []openAPIServer {
	if len(cfg.Servers) > 0 {
		servers := make([]openAPIServer, 0, len(cfg.Servers))
		for _, server := range cfg.Servers {
			if server.URL == "" {
				continue
			}
			servers = append(servers, openAPIServer(server))
		}
		if len(servers) > 0 {
			return servers
		}
	}
	if cfg.ServerURL != "" {
		return []openAPIServer{{URL: cfg.ServerURL}}
	}
	return nil
}

// buildComponents merges the user-provided Components with the configured
// SecuritySchemes without mutating either input.
func buildComponents(cfg *Config) map[string]any {
	if len(cfg.Components) == 0 && len(cfg.SecuritySchemes) == 0 {
		return nil
	}

	components := make(map[string]any, len(cfg.Components)+1)
	maps.Copy(components, cfg.Components)

	if len(cfg.SecuritySchemes) > 0 {
		// Preserve any securitySchemes the user already placed in Components by
		// merging rather than overwriting.
		schemes := make(map[string]any, len(cfg.SecuritySchemes))
		if existing, ok := components["securitySchemes"].(map[string]any); ok {
			maps.Copy(schemes, existing)
		}
		maps.Copy(schemes, cfg.SecuritySchemes)
		components["securitySchemes"] = schemes
	}

	return components
}

func mergeRouteParameters(params []parameter, index map[string]int, extras []fiber.RouteParameter) []parameter {
	if len(extras) == 0 {
		return params
	}
	for i := range extras {
		extra := &extras[i]
		if utils.TrimSpace(extra.Name) == "" {
			continue
		}
		location := utilsstrings.ToLower(utils.TrimSpace(extra.In))
		if location == "" {
			location = "query"
		}
		// OpenAPI spec: "example" and "examples" are mutually exclusive.
		// Prefer "examples" when both are provided.
		var paramExample any
		var paramExamples map[string]any
		if copiedExamples := copyAnyMap(extra.Examples); len(copiedExamples) > 0 {
			paramExamples = copiedExamples
		} else {
			paramExample = extra.Example
		}
		param := parameter{
			Name:            extra.Name,
			In:              location,
			Description:     extra.Description,
			Required:        extra.Required,
			Schema:          schemaFrom(extra.Schema, extra.SchemaRef, schemaTypeString),
			Example:         paramExample,
			Examples:        paramExamples,
			Deprecated:      extra.Deprecated,
			Style:           extra.Style,
			AllowEmptyValue: extra.AllowEmptyValue,
			AllowReserved:   extra.AllowReserved,
		}
		if extra.Explode != nil {
			explode := *extra.Explode
			param.Explode = &explode
		}
		if param.In == paramLocationPath {
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
	if _, ok := copied[schemaKeyType]; !ok && defaultType != "" {
		copied[schemaKeyType] = defaultType
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
	// OpenAPI spec: "example" and "examples" are mutually exclusive.
	// Prefer "examples" when both are provided.
	if ex := copyAnyMap(examples); len(ex) > 0 {
		entry["examples"] = ex
	} else if example != nil {
		entry["example"] = example
	}
	return entry
}

// routeMediaTypeContent builds an OpenAPI content map from per-media-type
// entries, allowing a different schema/example/encoding per content type.
func routeMediaTypeContent(content map[string]fiber.RouteMediaType) map[string]map[string]any {
	if len(content) == 0 {
		return nil
	}
	out := make(map[string]map[string]any, len(content))
	for mediaType, mt := range content {
		if mediaType == "" {
			continue
		}
		entry := contentEntry(mt.Schema, mt.SchemaRef, mt.Example, mt.Examples)
		if enc := copyAnyMap(mt.Encoding); len(enc) > 0 {
			entry["encoding"] = enc
		}
		if len(entry) == 0 {
			entry = map[string]any{}
		}
		out[mediaType] = entry
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func convertRouteResponses(routeResponses map[string]fiber.RouteResponse) map[string]response {
	var merged map[string]response
	if len(routeResponses) > 0 {
		merged = make(map[string]response, len(routeResponses))
		for code, resp := range routeResponses {
			content := routeMediaTypeContent(resp.Content)
			if content == nil {
				content = mediaTypesToContent(resp.MediaTypes, resp.Schema, resp.SchemaRef, resp.Example, resp.Examples)
			}
			merged[code] = response{
				Description: resp.Description,
				Content:     content,
				Headers:     copyAnyMap(resp.Headers),
				Links:       copyAnyMap(resp.Links),
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
	for i := range extras {
		copyExtra := extras[i]
		if utils.EqualFold(utils.TrimSpace(copyExtra.In), paramLocationPath) {
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
	content := routeMediaTypeContent(routeBody.Content)
	if content == nil {
		content = mediaTypesToContent(routeBody.MediaTypes, routeBody.Schema, routeBody.SchemaRef, routeBody.Example, routeBody.Examples)
	}
	merged := &requestBody{
		Description: routeBody.Description,
		Required:    routeBody.Required,
		Content:     content,
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
