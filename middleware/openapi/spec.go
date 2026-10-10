package openapi

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

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
	extensions   map[string]any      // Arbitrary operation-object fields, merged at marshal time

	// A pointer, so an empty list (a route that needs no authentication) is
	// written as [] while nil is left out.
	Security *[]map[string][]string `json:"security,omitempty"`

	OperationID string `json:"operationId,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`

	Parameters []parameter `json:"parameters,omitempty"`
	Tags       []string    `json:"tags,omitempty"`

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
	// Spliced into the encoded object rather than round-tripped through
	// map[string]any, which would turn every number into a float64.
	var existing map[string]json.RawMessage
	if err = json.Unmarshal(base, &existing); err != nil {
		return nil, fmt.Errorf("openapi: merge operation extensions: %w", err)
	}

	keys := make([]string, 0, len(o.extensions))
	for key := range o.extensions {
		if _, taken := existing[key]; taken {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return base, nil
	}
	// Sorted so the generated document is byte-stable across runs.
	slices.Sort(keys)

	// Encode first and size the buffer by summing the parts, so the capacity is
	// exact and the computation cannot overflow.
	type extensionPair struct{ key, value []byte }
	pairs := make([]extensionPair, 0, len(keys))
	size := len(base)
	for _, key := range keys {
		encodedKey, kErr := json.Marshal(key)
		if kErr != nil {
			return nil, fmt.Errorf("openapi: marshal operation extension key %q: %w", key, kErr)
		}
		encodedValue, vErr := json.Marshal(o.extensions[key])
		if vErr != nil {
			return nil, fmt.Errorf("openapi: marshal operation extension %q: %w", key, vErr)
		}
		pairs = append(pairs, extensionPair{key: encodedKey, value: encodedValue})
		size += len(encodedKey) + len(encodedValue) + 2 // ':' separator and ',' delimiter
	}

	out := make([]byte, 0, size)
	out = append(out, base[:len(base)-1]...) // everything but the closing brace
	for _, pair := range pairs {
		if len(out) > 1 {
			out = append(out, ',')
		}
		out = append(out, pair.key...)
		out = append(out, ':')
		out = append(out, pair.value...)
	}
	out = append(out, '}')
	return out, nil
}

type response struct {
	Content     map[string]map[string]any `json:"content,omitempty"`
	Headers     map[string]any            `json:"headers,omitempty"`
	Links       map[string]any            `json:"links,omitempty"`
	Description string                    `json:"description"`
}

type parameter struct {
	Schema          map[string]any            `json:"schema,omitempty"`
	Content         map[string]map[string]any `json:"content,omitempty"`
	Example         any                       `json:"example,omitempty"`
	Examples        map[string]any            `json:"examples,omitempty"`
	Explode         *bool                     `json:"explode,omitempty"`
	Description     string                    `json:"description,omitempty"`
	Name            string                    `json:"name"`
	In              string                    `json:"in"`
	Style           string                    `json:"style,omitempty"`
	Required        bool                      `json:"required"`
	Deprecated      bool                      `json:"deprecated,omitempty"`
	AllowEmptyValue bool                      `json:"allowEmptyValue,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	AllowReserved   bool                      `json:"allowReserved,omitempty"`   //nolint:tagliatelle // OpenAPI spec uses camelCase
}

type requestBody struct {
	Content     map[string]map[string]any `json:"content"`
	Description string                    `json:"description,omitempty"`
	Required    bool                      `json:"required,omitempty"`
}

const (
	paramLocationPath = "path"
	// paramLocationQuerystring is the OpenAPI 3.2 location that describes the
	// entire query string as one value. It must be paired with "content".
	paramLocationQuerystring = "querystring"
	// querystringMediaType is the media type used to wrap a querystring
	// parameter's schema when the author supplied no explicit content map.
	querystringMediaType = "application/x-www-form-urlencoded"

	schemaKeyType     = "type"
	schemaKeyRef      = "$ref"
	schemaTypeArray   = "array"
	schemaKeyFormat   = "format"
	schemaTypeString  = "string"
	schemaTypeObject  = "object"
	schemaTypeBoolean = "boolean"
	schemaTypeInteger = "integer"
	schemaTypeNumber  = "number"
)

// openAPIOperationMethods is the set of methods a Path Item can express. CONNECT
// has no OpenAPI operation; `query` is 3.2-only and gated by the caller.
var openAPIOperationMethods = map[string]struct{}{
	fiber.MethodGet:     {},
	fiber.MethodHead:    {},
	fiber.MethodPost:    {},
	fiber.MethodPut:     {},
	fiber.MethodPatch:   {},
	fiber.MethodDelete:  {},
	fiber.MethodOptions: {},
	fiber.MethodTrace:   {},
	fiber.MethodQuery:   {},
}

// isOpenAPIOperationMethod reports whether method maps to a Path Item
// operation key.
func isOpenAPIOperationMethod(method string) bool {
	_, ok := openAPIOperationMethods[method]
	return ok
}

// generateOperationID derives a stable operationId from a method and path, e.g.
// ("GET", "/users/{id}") -> "getUsersId", for routes with no explicit Name.
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

// uniqueOperationID returns id, or id with a numeric suffix until unique, so the
// document never repeats an operationId.
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

// generateSpec builds the OpenAPI document from a snapshot of routes (deep
// copies from App.GetRoutes, safe to read without further locking).
// specEnv carries what generation reads from the app rather than from Config.
type specEnv struct {
	validator fiber.StructValidator
	// equal compares route text the way the app's router does.
	equal func(a, b string) bool
}

// routeFacts is what the document says about a route before its path is
// expanded: its own metadata with the inferences filled in.
type routeFacts struct {
	security       *[]map[string][]string
	summary        string
	respType       string
	consumes       string
	tags           []string
	declared       []fiber.RouteParameter
	chain          middlewareSet
	documentsInput bool
}

// specBuilder accumulates the paths of one document.
type specBuilder struct {
	cfg     *Config
	env     specEnv
	reg     *schemaRegistry
	schemes *securitySchemes
	paths   map[string]map[string]operation
	// usedOperationIDs guarantees operationId uniqueness across the document,
	// which the OpenAPI specification requires.
	usedOperationIDs map[string]struct{}
	// hierarchyPaths maps a name-blanked template to the path already published
	// for it: the spec forbids two paths differing only in parameter names.
	hierarchyPaths map[string]canonicalPathItem
	// validates is set when a 400 is the route's own outcome: only a validator
	// can reject what a route binds.
	validates bool
}

func generateSpec(routes []fiber.Route, cfg *Config, env specEnv) openAPISpec {
	b := &specBuilder{
		cfg:              cfg,
		env:              env,
		reg:              newSchemaRegistry(cfg),
		schemes:          newSecuritySchemes(),
		paths:            make(map[string]map[string]operation),
		usedOperationIDs: make(map[string]struct{}),
		hierarchyPaths:   make(map[string]canonicalPathItem),
		validates:        env.validator != nil && !cfg.DisableValidationResponses,
	}
	b.addRoutes(routes)
	return b.document()
}

// addRoutes documents every route the document can represent.
func (b *specBuilder) addRoutes(routes []fiber.Route) {
	// Use routes sit in every method's stack ahead of the routes they cover,
	// so the ones seen so far in a method's stack are the ones a request to a
	// later route passes through.
	var (
		coveredMethod string
		covering      []coveringMiddleware
	)
	for i := range routes {
		r := &routes[i]
		if r.Method != coveredMethod {
			coveredMethod = r.Method
			covering = covering[:0]
		}
		if r.IsMiddleware() {
			if !b.cfg.DisableMiddlewareInference {
				if set := handlerMiddleware(r.InnerHandlers()); set != 0 {
					covering = append(covering, coveringMiddleware{prefix: r.Path, domain: r.Domain(), set: set})
				}
			}
			continue
		}
		if !b.documents(r) {
			continue
		}
		facts := b.inferRoute(r, covering)
		for _, variant := range buildOpenAPIPathVariants(r.Path, r.Params) {
			b.addVariant(r, &facts, &variant)
		}
	}
}

// documents reports whether a route has an operation in the document.
func (b *specBuilder) documents(r *fiber.Route) bool {
	// A Path Item has a fixed set of operation keys, so a custom method has no
	// valid representation and is skipped. CONNECT has none either.
	if !isOpenAPIOperationMethod(r.Method) {
		return false
	}
	// The OpenAPI `query` operation key exists only in 3.2+; skip QUERY routes
	// for earlier versions, where it cannot be represented.
	if r.Method == fiber.MethodQuery && !versionAtLeast(b.cfg.OpenAPIVersion, versionOpenAPI32) {
		return false
	}
	return !r.IsAutoHead() && !r.IsHidden()
}

// inferRoute fills what a route does not say from what the router knows: the
// handler's name, the enclosing group, the app-wide media types and the
// recognized middleware on its path. Anything the route sets itself wins.
func (b *specBuilder) inferRoute(r *fiber.Route, covering []coveringMiddleware) routeFacts {
	cfg := b.cfg
	facts := routeFacts{
		summary:  r.Summary,
		tags:     r.Tags,
		respType: r.Produces,
		consumes: r.Consumes,
		documentsInput: r.RequestBody != nil || len(r.ParameterModels) > 0 ||
			len(r.Parameters) > 0,
	}
	if facts.summary == "" && !cfg.DisableHandlerSummaries {
		facts.summary = handlerSummary(r.InnerHandlers())
	}
	if len(facts.tags) == 0 && !cfg.DisableGroupTags {
		if tag := groupTag(r); tag != "" {
			facts.tags = []string{tag}
		}
	}
	if facts.respType == "" {
		facts.respType = cfg.DefaultProduces
	}
	if !cfg.DisableMiddlewareInference {
		facts.chain = middlewareOn(covering, r, b.env.equal)
	}

	if r.Security != nil {
		// The author documented the route's authentication, so none is
		// inferred from the middleware: neither its requirement nor its 401.
		facts.chain &^= authMiddleware
		security := r.Security
		facts.security = &security
	} else if requirement := b.schemes.requirement(facts.chain); requirement != nil {
		security := []map[string][]string{requirement}
		facts.security = &security
	}

	facts.declared = expandParameterModels(r.ParameterModels, b.reg)
	if facts.chain.has(kindCSRF) && !isSafeMethod(r.Method) {
		facts.declared = append([]fiber.RouteParameter{csrfParameter(cfg.CSRFHeader)}, facts.declared...)
	}
	return facts
}

// addVariant places a route's operation under one of its path templates.
func (b *specBuilder) addVariant(r *fiber.Route, facts *routeFacts, variant *pathVariant) {
	methodLower := utilsstrings.ToLower(r.Method)
	// The router dispatches to the first match, so on a path+method collision
	// the earlier registration describes the behavior.
	if _, exists := b.paths[variant.Path][methodLower]; exists {
		return
	}
	// Templates identical up to parameter names MUST NOT coexist, and the
	// router would never dispatch to the later one: first wins.
	hierarchy := normalizePathHierarchy(variant.Path)
	if canonical, exists := b.hierarchyPaths[hierarchy]; exists {
		// Already published under a different parameter name: join that path
		// item, or drop the operation if its method is taken.
		if canonical.path != variant.Path {
			if _, taken := b.paths[canonical.path][methodLower]; taken {
				return
			}
			// Parameters move with the operation: declaring "name" under a
			// path reading "{id}" makes the document invalid.
			adoptCanonicalParamNames(variant, canonical.params)
			variant.Path = canonical.path
		}
	} else {
		b.hierarchyPaths[hierarchy] = canonicalPathItem{path: variant.Path, params: variant.ParamNames}
	}

	if b.paths[variant.Path] == nil {
		b.paths[variant.Path] = make(map[string]operation)
	}
	b.paths[variant.Path][methodLower] = b.buildOperation(r, facts, variant)
}

// buildOperation assembles the operation a route has at one path template.
func (b *specBuilder) buildOperation(r *fiber.Route, facts *routeFacts, variant *pathVariant) operation {
	cfg := b.cfg

	params := make([]parameter, 0, len(variant.ParamNames))
	paramIndex := make(map[string]int, len(variant.ParamNames))
	for _, p := range variant.ParamNames {
		param := parameter{
			Name:     p,
			In:       paramLocationPath,
			Required: true,
			// A "<...>" constraint narrows what the router accepts, so the
			// schema reflects it instead of always using string.
			Schema: pathParamSchema(variant.ParamConstraints[p]),
		}
		params = append(params, param)
		paramIndex[param.In+":"+param.Name] = len(params) - 1
	}
	// Middleware and declared models come first so an explicit AddParameter for
	// the same name still overrides what they say.
	extras := remapRouteParameters(append(slices.Clone(facts.declared), r.Parameters...), variant.PathParamAliases, variant.ParamNames)
	// The "querystring" location exists only in 3.2+; emitting it earlier would
	// make the document invalid.
	if !versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI32) {
		extras = dropQuerystringParameters(extras)
	}
	params = mergeRouteParameters(params, paramIndex, extras, b.reg)

	summary := facts.summary
	if summary == "" {
		summary = r.Method + " " + variant.Path
	}
	operationID := r.Name
	if operationID == "" {
		operationID = generateOperationID(r.Method, variant.Path)
	}
	operationID = uniqueOperationID(operationID, b.usedOperationIDs)

	responses := convertRouteResponses(r.Responses, facts.respType, b.reg)
	if len(responses) == 0 {
		status, defaultResp := defaultResponseForMethod(r.Method, facts.respType)
		responses = map[string]response{status: defaultResp}
	}
	if facts.chain != 0 {
		applyMiddlewareResponses(responses, r.Method, facts.chain, cfg, b.reg)
	}
	if b.validates && facts.documentsInput {
		if _, ok := responses["400"]; !ok {
			responses["400"] = errorResponse("Bad Request", cfg, b.reg)
		}
	}

	// A body declared without a media type takes the route's Consumes, then
	// the app-wide default.
	bodyType := facts.consumes
	if bodyType == "" {
		bodyType = cfg.DefaultConsumes
	}
	reqBody := buildRequestBody(r.RequestBody, bodyType, b.reg)
	if reqBody == nil && facts.consumes != "" {
		reqBody = &requestBody{Content: map[string]map[string]any{facts.consumes: {}}}
	}
	// GET and HEAD operations never carry a request body, and a TRACE request
	// MUST NOT include content (RFC 9110).
	if r.Method == fiber.MethodGet || r.Method == fiber.MethodHead || r.Method == fiber.MethodTrace {
		reqBody = nil
	}

	return operation{
		OperationID:  operationID,
		Summary:      summary,
		Description:  r.Description,
		Tags:         facts.tags,
		Deprecated:   r.Deprecated,
		Parameters:   params,
		RequestBody:  reqBody,
		Responses:    responses,
		Security:     facts.security,
		ExternalDocs: r.ExternalDocs,
		extensions:   r.OperationExtensions,
	}
}

// document assembles the document around the collected paths.
func (b *specBuilder) document() openAPISpec {
	cfg := b.cfg
	paths := b.paths
	reg := b.reg
	schemes := b.schemes
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

	// The License Object allows identifier or url, never both, and the SPDX
	// identifier itself requires 3.1+. Narrow a copy so the caller's is untouched.
	if cfg.License != nil && cfg.License.Identifier != "" {
		licenseCopy := *cfg.License
		if versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI31) {
			// identifier wins: it is the more precise of the two.
			licenseCopy.URL = ""
		} else {
			licenseCopy.Identifier = ""
		}
		spec.Info.License = &licenseCopy
	}

	if versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI31) {
		spec.Info.Summary = cfg.Summary
		spec.JSONSchemaDialect = cfg.JSONSchemaDialect
		if len(cfg.Webhooks) > 0 {
			spec.Webhooks = maps.Clone(cfg.Webhooks)
		}
	}

	if versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI32) {
		spec.Self = cfg.Self
	}

	spec.Components = buildComponents(cfg, reg, schemes)

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
func buildComponents(cfg *Config, reg *schemaRegistry, schemes *securitySchemes) map[string]any {
	registered := reg.componentSchemas()
	inferred := schemes.declarations()
	if len(cfg.Components) == 0 && len(cfg.SecuritySchemes) == 0 && len(registered) == 0 && len(inferred) == 0 {
		return nil
	}

	components := make(map[string]any, len(cfg.Components)+2)
	maps.Copy(components, cfg.Components)

	if len(registered) > 0 {
		// The types met while generating join the user's schemas, which keep
		// their names: the registry never claims one of them.
		schemas := make(map[string]any, len(registered))
		maps.Copy(schemas, registered)
		maps.Copy(schemas, stringKeyedEntries(components["schemas"]))
		components["schemas"] = schemas
	}

	if len(cfg.SecuritySchemes) > 0 || len(inferred) > 0 {
		// The schemes the recognized middleware asked for go in first, so a
		// scheme the user placed in Components or SecuritySchemes under the
		// same name replaces them rather than the other way round.
		merged := make(map[string]any, len(cfg.SecuritySchemes)+len(inferred))
		maps.Copy(merged, inferred)
		maps.Copy(merged, stringKeyedEntries(components["securitySchemes"]))
		maps.Copy(merged, cfg.SecuritySchemes)
		components["securitySchemes"] = merged
	}

	return components
}

// stringKeyedEntries reads any string-keyed map as map[string]any. A typed map
// such as map[string]MyScheme fails a plain type assertion, and treating that as
// "absent" would drop the caller's schemes instead of merging them.
func stringKeyedEntries(src any) map[string]any {
	if src == nil {
		return nil
	}
	if typed, ok := src.(map[string]any); ok {
		return typed
	}
	value := reflect.ValueOf(src)
	if value.Kind() != reflect.Map || value.Type().Key().Kind() != reflect.String || value.IsNil() {
		return nil
	}
	out := make(map[string]any, value.Len())
	iter := value.MapRange()
	for iter.Next() {
		out[iter.Key().String()] = iter.Value().Interface()
	}
	return out
}
