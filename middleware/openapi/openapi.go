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

// maxCachedSwaggerPages bounds the per-target Swagger UI page cache so that a
// parameterized mount prefix (one target per parameter value) cannot grow the
// cache without limit.
const maxCachedSwaggerPages = 32

// New creates a new middleware handler that serves the generated OpenAPI specification.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// The Swagger UI page only depends on the resolved spec path, so it is
	// cached per target path. The handler may serve several prefixes (e.g.
	// app.Use([]string{"/v1", "/v2"}, New())), each needing its own page.
	var (
		swaggerMu    sync.Mutex
		swaggerPages = make(map[string][]byte)
	)

	specPath := utils.TrimRight(normalizedPath(cfg.Path), '/')
	uiPath := utils.TrimRight(normalizedPath(cfg.UIPath), '/')

	// The app's case sensitivity is immutable once the server runs, so it is
	// resolved once instead of copying the config on every request.
	var (
		initOnce      sync.Once
		caseSensitive bool
	)

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			return c.Next()
		}

		initOnce.Do(func() {
			caseSensitive = c.App().Config().CaseSensitive
		})
		equal := utils.EqualFold[string]
		if caseSensitive {
			equal = stringsEqual
		}

		request := utils.TrimRight(c.Path(), '/')
		route := c.Route()
		isMiddleware := route != nil && route.IsMiddleware()

		// Fast path for prefix-mounted middleware: most requests cannot match
		// either target, so skip target resolution without allocating.
		if isMiddleware && !hasSuffix(request, specPath, equal) && !hasSuffix(request, uiPath, equal) {
			return c.Next()
		}

		specTarget, uiTarget := resolveTargets(c, specPath, uiPath, equal)

		switch {
		case equal(request, specTarget):
			// The spec is regenerated on every request so route additions,
			// removals, and metadata changes are always reflected. Generation
			// only happens for requests to the spec path itself.
			spec := generateSpec(c.App(), &cfg)
			data, err := c.App().Config().JSONEncoder(spec)
			if err != nil {
				return fmt.Errorf("openapi: marshal spec: %w", err)
			}

			c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
			return c.Status(fiber.StatusOK).Send(data)
		case uiTarget != "" && equal(request, uiTarget):
			targetPath := specTarget
			swaggerMu.Lock()
			data, ok := swaggerPages[targetPath]
			if !ok {
				var err error
				data, err = buildSwaggerUIPage(targetPath, &cfg)
				if err != nil {
					swaggerMu.Unlock()
					return fmt.Errorf("openapi: build swagger ui page: %w", err)
				}
				if len(swaggerPages) < maxCachedSwaggerPages {
					swaggerPages[targetPath] = data
				}
			}
			swaggerMu.Unlock()

			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
			return c.Status(fiber.StatusOK).Send(data)
		default:
			return c.Next()
		}
	}
}

func stringsEqual(a, b string) bool { return a == b }

// hasSuffix reports whether s ends with suffix under the given equality
// function (exact or case-folding).
func hasSuffix(s, suffix string, equal func(a, b string) bool) bool {
	return len(s) >= len(suffix) && equal(s[len(s)-len(suffix):], suffix)
}

// resolveTargets derives the spec and UI target paths (trailing slashes
// trimmed, returned in that order) for the current request from the route the
// handler runs on.
//
// For prefix middleware the targets live directly under the mount prefix. For
// an exact method route (e.g. app.Get("/openapi.json", openapi.New()), possibly
// cloned under a mount prefix) the registered path itself is the target: the
// configured suffix decides whether it serves the spec or the UI, and a custom
// path matching neither serves the specification. An empty UI target means the
// UI is not served.
//
//nolint:gocritic // unnamedResult: nonamedreturns forbids naming these
func resolveTargets(c fiber.Ctx, specPath, uiPath string, equal func(a, b string) bool) (string, string) {
	route := c.Route()
	if route == nil {
		return specPath, uiPath
	}

	if !route.IsMiddleware() {
		// The route matched exactly, so the request path IS the concrete
		// registered path — including values of any parameters the pattern
		// carries (e.g. an exact route cloned under a parameterized mount
		// prefix).
		path := utils.TrimRight(c.Path(), '/')
		switch {
		case hasSuffix(path, specPath, equal):
			prefix := path[:len(path)-len(specPath)]
			return path, prefix + uiPath
		case hasSuffix(path, uiPath, equal):
			prefix := path[:len(path)-len(uiPath)]
			return prefix + specPath, path
		default:
			return path, ""
		}
	}

	prefix := routePrefix(route.Path, c.Path())
	return prefix + specPath, prefix + uiPath
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

// normalizedPath returns cfgPath with a leading slash. Defaults for empty
// paths are applied earlier by configDefault.
func normalizedPath(cfgPath string) string {
	if !strings.HasPrefix(cfgPath, "/") {
		return "/" + cfgPath
	}
	return cfgPath
}

// routePrefix derives the concrete mount prefix of a middleware route for the
// current request. A static prefix is the route path itself. When the prefix
// pattern contains parameters (e.g. app.Use("/:tenant", openapi.New())), the
// concrete values are taken from the request path, consuming one request
// segment per pattern segment; greedy wildcards and optional parameters make
// the prefix length ambiguous and end it.
func routePrefix(pattern, requestPath string) string {
	pattern = utils.TrimRight(pattern, '/')
	if pattern == "" {
		return ""
	}
	if !strings.ContainsAny(pattern, ":*+") {
		return pattern
	}

	segments := 0
	for seg := range strings.SplitSeq(strings.TrimPrefix(pattern, "/"), "/") {
		if seg == "" || strings.ContainsAny(seg, "*+") || strings.HasSuffix(seg, "?") {
			break
		}
		segments++
	}
	if segments == 0 {
		return ""
	}

	idx := 0
	for range segments {
		if idx+1 >= len(requestPath) {
			return requestPath
		}
		next := strings.IndexByte(requestPath[idx+1:], '/')
		if next < 0 {
			// The request path ends inside the prefix (e.g. a request for the
			// bare mount path).
			return requestPath
		}
		idx += 1 + next
	}
	return requestPath[:idx]
}

type openAPISpec struct {
	Paths             map[string]map[string]operation `json:"paths"`
	Components        map[string]any                  `json:"components,omitempty"`
	Webhooks          map[string]any                  `json:"webhooks,omitempty"`
	ExternalDocs      *ExternalDocs                   `json:"externalDocs,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Info              openAPIInfo                     `json:"info"`
	OpenAPI           string                          `json:"openapi"`
	Self              string                          `json:"$self,omitempty"`
	JSONSchemaDialect string                          `json:"jsonSchemaDialect,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Servers           []Server                        `json:"servers,omitempty"`
	Security          []map[string][]string           `json:"security,omitempty"`
	Tags              []Tag                           `json:"tags,omitempty"`
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
//nolint:gocritic // hugeParam: a value receiver is required so map values (which are not addressable) are marshaled through this method
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
	schemaTypeBoolean = "boolean"
	schemaTypeInteger = "integer"
	schemaTypeNumber  = "number"
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

// openAPIVersionRank orders the supported OpenAPI versions for comparison.
var openAPIVersionRank = map[string]int{
	versionOpenAPI30: 0,
	versionOpenAPI31: 1,
	versionOpenAPI32: 2,
}

// versionAtLeast reports whether version is greater than or equal to minimum.
func versionAtLeast(version, minimum string) bool {
	return openAPIVersionRank[version] >= openAPIVersionRank[minimum]
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
			// The OpenAPI `query` operation key exists only in 3.2+; skip QUERY
			// routes for earlier versions, where it cannot be represented.
			if r.Method == fiber.MethodQuery && !versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI32) {
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
				methodLower := utilsstrings.ToLower(r.Method)
				// The router dispatches to the first matching route, so when two
				// routes produce the same path+method (e.g. an optional-parameter
				// variant colliding with an explicitly registered path), the
				// earlier registration describes the observable behavior and the
				// later one must not overwrite its documentation.
				if _, exists := paths[variant.Path][methodLower]; exists {
					continue
				}

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
				// The "querystring" location only exists in OpenAPI 3.2+;
				// emitting it for earlier versions would make the document
				// invalid.
				if !versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI32) {
					extras = dropQuerystringParameters(extras)
				}
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

				responses := convertRouteResponses(r.Responses, respType)
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
					ExternalDocs: maps.Clone(r.ExternalDocs),
					extensions:   maps.Clone(r.OperationExtensions),
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
		spec.Tags = append([]Tag(nil), cfg.Tags...)
	}

	if cfg.ExternalDocs != nil {
		spec.ExternalDocs = &ExternalDocs{
			Description: cfg.ExternalDocs.Description,
			URL:         cfg.ExternalDocs.URL,
		}
	}

	// license.identifier (SPDX) requires OpenAPI 3.1+. Drop it for 3.0 without
	// mutating the caller's License.
	if cfg.License != nil && cfg.License.Identifier != "" && !versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI31) {
		licenseCopy := *cfg.License
		licenseCopy.Identifier = ""
		spec.Info.License = &licenseCopy
	}

	// OpenAPI 3.1+ document fields.
	if versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI31) {
		spec.Info.Summary = cfg.Summary
		spec.JSONSchemaDialect = cfg.JSONSchemaDialect
		if len(cfg.Webhooks) > 0 {
			spec.Webhooks = maps.Clone(cfg.Webhooks)
		}
	}

	// OpenAPI 3.2+ document fields.
	if versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI32) {
		spec.Self = cfg.Self
	}

	spec.Components = buildComponents(cfg)

	return spec
}

// buildServers resolves the server list, preferring Config.Servers and falling
// back to the single Config.ServerURL for backward compatibility.
func buildServers(cfg *Config) []Server {
	// Server.name is an OpenAPI 3.2+ field.
	allowName := versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI32)
	if len(cfg.Servers) > 0 {
		servers := make([]Server, 0, len(cfg.Servers))
		for _, server := range cfg.Servers {
			if server.URL == "" {
				continue
			}
			if !allowName {
				server.Name = ""
			}
			servers = append(servers, server)
		}
		if len(servers) > 0 {
			return servers
		}
	}
	if cfg.ServerURL != "" {
		return []Server{{URL: cfg.ServerURL}}
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

// dropQuerystringParameters filters out parameters using the OpenAPI 3.2-only
// "querystring" location, returning the input slice unchanged when none match.
func dropQuerystringParameters(extras []fiber.RouteParameter) []fiber.RouteParameter {
	isQuerystring := func(in string) bool {
		return utils.EqualFold(utils.TrimSpace(in), "querystring")
	}
	for i := range extras {
		if !isQuerystring(extras[i].In) {
			continue
		}
		filtered := append([]fiber.RouteParameter(nil), extras[:i]...)
		for j := i + 1; j < len(extras); j++ {
			if isQuerystring(extras[j].In) {
				continue
			}
			filtered = append(filtered, extras[j])
		}
		return filtered
	}
	return extras
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
		// OpenAPI 3.2 querystring parameters are described via content rather
		// than schema, so no default schema type is injected for them.
		defaultType := schemaTypeString
		if location == "querystring" {
			defaultType = ""
		}
		// OpenAPI spec: "example" and "examples" are mutually exclusive.
		// Prefer "examples" when both are provided.
		var paramExample any
		var paramExamples map[string]any
		if copiedExamples := maps.Clone(extra.Examples); len(copiedExamples) > 0 {
			paramExamples = copiedExamples
		} else {
			paramExample = extra.Example
		}
		param := parameter{
			Name:            extra.Name,
			In:              location,
			Description:     extra.Description,
			Required:        extra.Required,
			Schema:          schemaFrom(extra.Schema, extra.SchemaRef, defaultType),
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

func schemaFrom(schema map[string]any, schemaRef, defaultType string) map[string]any {
	if schemaRef != "" {
		return map[string]any{"$ref": schemaRef}
	}

	copied := maps.Clone(schema)
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
	} else if copied := maps.Clone(schema); len(copied) > 0 {
		entry["schema"] = copied
	}
	// OpenAPI spec: "example" and "examples" are mutually exclusive.
	// Prefer "examples" when both are provided.
	if ex := maps.Clone(examples); len(ex) > 0 {
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
		if enc := maps.Clone(mt.Encoding); len(enc) > 0 {
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

// convertRouteResponses converts route response metadata, falling back to the
// route's Produces media type when a response documents a schema or example
// without naming any media type, so that data is not silently dropped.
func convertRouteResponses(routeResponses map[string]fiber.RouteResponse, fallbackMediaType string) map[string]response {
	var merged map[string]response
	if len(routeResponses) > 0 {
		merged = make(map[string]response, len(routeResponses))
		for code, resp := range routeResponses {
			content := routeMediaTypeContent(resp.Content)
			if content == nil {
				mediaTypes := resp.MediaTypes
				if len(mediaTypes) == 0 && fallbackMediaType != "" &&
					(len(resp.Schema) > 0 || resp.SchemaRef != "" || resp.Example != nil || len(resp.Examples) > 0) {
					mediaTypes = []string{fallbackMediaType}
				}
				content = mediaTypesToContent(mediaTypes, resp.Schema, resp.SchemaRef, resp.Example, resp.Examples)
			}
			merged[code] = response{
				Description: resp.Description,
				Content:     content,
				Headers:     maps.Clone(resp.Headers),
				Links:       maps.Clone(resp.Links),
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

// pathState carries the in-progress OpenAPI path while walking a Fiber route
// pattern; optional parameters fork the walk into include/exclude branches.
type pathState struct {
	aliases  map[string]string
	path     string
	params   []string
	paramIdx int
}

func (s pathState) clone() pathState {
	return pathState{
		path:     s.path,
		params:   append([]string(nil), s.params...),
		aliases:  maps.Clone(s.aliases),
		paramIdx: s.paramIdx,
	}
}

func buildOpenAPIPathVariants(fiberPath string, params []string) []pathVariant {
	var (
		length   = len(fiberPath)
		variants []pathVariant
	)

	var walk func(i int, current pathState)
	walk = func(i int, current pathState) {
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
				includeState := current.clone()
				includeState.path += "{" + resolved.openAPI + "}"
				includeState.params = append(includeState.params, resolved.openAPI)
				includeState.aliases[resolved.raw] = resolved.openAPI
				if tokenName != "" {
					includeState.aliases[tokenName] = resolved.openAPI
				}
				includeState.paramIdx++

				if isOptional {
					excludeState := current.clone()
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

			case '\\':
				// The route grammar escapes the next character, matching it
				// literally (see path.go escapeChar).
				if i+1 < length {
					current.path += string(fiberPath[i+1])
				}
				i += 2

			default:
				// Append the whole literal run at once instead of one byte at a
				// time.
				runStart := i
				for i < length {
					c := fiberPath[i]
					if c == ':' || c == '*' || c == '+' || c == '\\' {
						break
					}
					i++
				}
				current.path += fiberPath[runStart:i]
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

	walk(0, pathState{
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
		openAPI: sanitizeOpenAPIParamName(raw, paramIdx+1),
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
