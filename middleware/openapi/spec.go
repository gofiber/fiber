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

const (
	// querystringMediaType wraps a querystring parameter's schema when no content map is given.
	querystringMediaType = "application/x-www-form-urlencoded"

	schemaKeyType     = "type"
	schemaTypeNull    = "null"
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
	Security          *[]map[string][]string          `json:"security,omitempty"`
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

	// A pointer so an empty list (no authentication) is written as [] while nil is omitted.
	Security *[]map[string][]string `json:"security,omitempty"`

	OperationID string `json:"operationId,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`

	Parameters []parameter `json:"parameters,omitempty"`
	Tags       []string    `json:"tags,omitempty"`
	// Servers narrows an operation registered under app.Domain to its host.
	Servers []Server `json:"servers,omitempty"`

	Deprecated bool `json:"deprecated,omitempty"`
}

// MarshalJSON merges operation extensions without clobbering generated keys.
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
	// Spliced in rather than round-tripped through map[string]any, which turns numbers into float64.
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
	// Sorted for byte-stable output.
	slices.Sort(keys)

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
	In              fiber.ParamLocation       `json:"in"`
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

// isOpenAPIOperationMethod reports whether method maps to a Path Item operation key.
func isOpenAPIOperationMethod(method string) bool {
	_, ok := openAPIOperationMethods[method]
	return ok
}

// generateOperationID derives an operationId such as "getUsersId" from a method and path.
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

func isASCIIDigit(c byte) bool { return c >= '0' && c <= '9' }

func isASCIIAlphanumeric(c byte) bool {
	return isASCIIDigit(c) || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// operationIDFromName makes a route name safe to publish as an operationId, which code
// generators turn into an identifier: letters, digits, "_", "-" and "." are kept,
// any other run of characters becomes a single "_", and a leading digit gets an "op_" prefix.
// A name with nothing usable yields "".
func operationIDFromName(name string) string {
	var b strings.Builder
	pending := false
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case isASCIIAlphanumeric(c) || c == '-' || c == '.':
			if pending && b.Len() > 0 {
				_ = b.WriteByte('_') //nolint:errcheck // strings.Builder.WriteByte never returns an error
			}
			pending = false
			_ = b.WriteByte(c) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		case c == '_':
			pending = false
			_ = b.WriteByte(c) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		default:
			pending = true
		}
	}
	id := b.String()
	// An identifier cannot start with a digit.
	if id != "" && isASCIIDigit(id[0]) {
		return "op_" + id
	}
	return id
}

// uniqueOperationID returns id with a numeric suffix until it is unique.
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

// undeclaredTags lists, by name, the tags operations use that Config.Tags does not describe, so every tag an
// operation carries is declared at the top of the document. They are sorted so the document is stable.
func undeclaredTags(paths map[string]map[string]operation, declared []Tag) []Tag {
	known := make(map[string]struct{}, len(declared))
	for i := range declared {
		known[declared[i].Name] = struct{}{}
	}
	var names []string
	for _, item := range paths {
		for method := range item {
			op := item[method]
			for _, name := range op.Tags {
				if _, ok := known[name]; !ok {
					known[name] = struct{}{}
					names = append(names, name)
				}
			}
		}
	}
	slices.Sort(names)
	tags := make([]Tag, len(names))
	for i, name := range names {
		tags[i] = Tag{Name: name}
	}
	return tags
}

// specEnv carries what generation reads from the app rather than from Config.
type specEnv struct {
	validator fiber.StructValidator
	// equal compares route text the way the app's router does.
	equal segmentEqual
	// strict keeps a trailing slash, as StrictRouting makes "/a" and "/a/" different routes.
	strict bool
}

type specBuilder struct {
	cfg     *Config
	reg     *schemaRegistry
	schemes *securitySchemes
	paths   map[string]map[string]operation
	// usedOperationIDs keeps operationIds unique, as OpenAPI requires.
	usedOperationIDs map[string]struct{}
	// hierarchyPaths maps a name-blanked template to its published path; OpenAPI forbids paths differing only in parameter names.
	hierarchyPaths map[string]canonicalPathItem
	env            specEnv
	policy         routePolicy
	// dialect is what the document's OpenAPI version permits.
	dialect dialect
}

func generateSpec(routes []fiber.Route, cfg *Config, env specEnv) openAPISpec {
	version := newDialect(cfg.OpenAPIVersion)
	b := &specBuilder{
		cfg:              cfg,
		env:              env,
		dialect:          version,
		reg:              newSchemaRegistry(cfg, version),
		schemes:          newSecuritySchemes(),
		paths:            make(map[string]map[string]operation),
		usedOperationIDs: make(map[string]struct{}),
		hierarchyPaths:   make(map[string]canonicalPathItem),
		policy:           newRoutePolicy(cfg, env),
	}
	b.addRoutes(routes)
	return b.document()
}

func (b *specBuilder) addRoutes(routes []fiber.Route) {
	// Use routes precede the routes they cover in each method's stack.
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
			if b.policy.middlewareInference {
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
		for _, variant := range buildOpenAPIPathVariants(r.Path, r.Params, b.env.strict) {
			b.addVariant(r, &facts, &variant)
		}
	}
}

func (b *specBuilder) documents(r *fiber.Route) bool {
	// A Path Item has fixed operation keys; custom methods and CONNECT cannot be represented.
	if !isOpenAPIOperationMethod(r.Method) {
		return false
	}
	// The `query` operation key exists only in 3.2+.
	if r.Method == fiber.MethodQuery && !b.dialect.hasQueryMethod() {
		return false
	}
	return !r.IsAutoHead() && !r.IsHidden()
}

// inferRoute resolves a route's facts, then registers what they imply: the security requirement
// middleware adds, and the parameter models and CSRF header the route declares.
func (b *specBuilder) inferRoute(r *fiber.Route, covering []coveringMiddleware) routeFacts {
	facts := resolveRoute(r, covering, b.policy)
	if facts.security == nil {
		if requirement := b.schemes.requirement(facts.chain); requirement != nil {
			security := []map[string][]string{requirement}
			facts.security = &security
		}
	}
	facts.declared = expandParameterModels(r.ParameterModels, b.reg)
	if facts.chain.has(kindCSRF) && !isSafeMethod(r.Method) {
		facts.declared = append([]fiber.RouteParameter{csrfParameter(b.cfg.CSRFHeader)}, facts.declared...)
	}
	return facts
}

func (b *specBuilder) addVariant(r *fiber.Route, facts *routeFacts, variant *pathVariant) {
	methodLower := utilsstrings.ToLower(r.Method)
	// The router dispatches to the first match, so the earlier registration wins.
	if _, exists := b.paths[variant.Path][methodLower]; exists {
		return
	}
	// Templates identical up to parameter names MUST NOT coexist in OpenAPI; first wins.
	hierarchy := normalizePathHierarchy(variant.Path)
	if canonical, exists := b.hierarchyPaths[hierarchy]; exists {
		// Join the already published path item, or drop the operation if its method is taken.
		if canonical.path != variant.Path {
			if _, taken := b.paths[canonical.path][methodLower]; taken {
				return
			}
			// Parameters must be renamed too, or the document is invalid.
			adoptCanonicalParamNames(variant, canonical.params)
			variant.Path = canonical.path
		}
	} else {
		b.hierarchyPaths[hierarchy] = canonicalPathItem{path: variant.Path, params: variant.ParamNames}
	}

	// Only a route that stays in the document declares the schemes it uses.
	b.schemes.use(facts.chain)
	if b.paths[variant.Path] == nil {
		b.paths[variant.Path] = make(map[string]operation)
	}
	b.paths[variant.Path][methodLower] = b.buildOperation(r, facts, variant)
}

func (b *specBuilder) buildOperation(r *fiber.Route, facts *routeFacts, variant *pathVariant) operation {
	cfg := b.cfg

	params := make([]parameter, 0, len(variant.ParamNames))
	paramIndex := make(map[parameterKey]int, len(variant.ParamNames))
	for _, p := range variant.ParamNames {
		param := parameter{
			Name:     p,
			In:       fiber.ParamInPath,
			Required: true,
			// A "<...>" constraint narrows the schema beyond string.
			Schema: pathParamSchema(variant.ParamConstraints[p]),
		}
		params = append(params, param)
		paramIndex[parameterKey{param.In, param.Name}] = len(params) - 1
	}
	// Middleware and declared models come first so an explicit AddParameter overrides them.
	extras := remapRouteParameters(append(slices.Clone(facts.declared), r.Parameters...), variant.PathParamAliases, variant.ParamNames)
	// The "querystring" location exists only in 3.2+.
	if b.dialect.hasQuerystringLocation() {
		extras = dropQueryAlongsideQuerystring(extras)
	} else {
		extras = dropQuerystringParameters(extras)
	}
	params = mergeRouteParameters(params, paramIndex, extras, b.reg)

	summary := facts.summary
	if summary == "" {
		summary = r.Method + " " + variant.Path
	}
	operationID := operationIDFromName(r.Name)
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
	if facts.badRequest {
		if _, ok := responses["400"]; !ok {
			responses["400"] = errorResponse("Bad Request", cfg, b.reg)
		}
	}

	reqBody := buildRequestBody(r.RequestBody, facts.bodyType, b.reg)
	if reqBody == nil && facts.ownConsumes != "" {
		reqBody = &requestBody{Content: map[string]map[string]any{facts.ownConsumes: {}}}
	}
	// RFC 9110: TRACE MUST NOT include content; GET and HEAD bodies are never documented.
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
		Servers:      domainServers(r.Domain()),
		extensions:   r.OperationExtensions,
	}
}

// domainServers describes the host of an app.Domain route as an operation-level
// server, or nil for any host. Each ":param" label becomes a server variable
// defaulting to its name. Routes sharing path and method across hosts still
// collapse to the first, as OpenAPI allows one operation per path and method.
func domainServers(pattern string) []Server {
	if pattern == "" {
		return nil
	}
	labels := strings.Split(pattern, ".")
	var variables map[string]ServerVariable
	for i, label := range labels {
		name, isParam := strings.CutPrefix(label, ":")
		if !isParam {
			continue
		}
		if variables == nil {
			variables = make(map[string]ServerVariable)
		}
		variables[name] = ServerVariable{Default: name}
		labels[i] = "{" + name + "}"
	}
	return []Server{{URL: "//" + strings.Join(labels, "."), Variables: variables}}
}

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
			License:        copyLicense(cfg.License),
		},
		Paths: paths,
	}

	spec.Servers = buildServers(cfg, b.dialect)

	// An empty list is kept: it states the API as a whole needs no authentication.
	if cfg.Security != nil {
		security := cfg.Security
		spec.Security = &security
	}

	spec.Tags = append([]Tag(nil), cfg.Tags...)
	spec.Tags = append(spec.Tags, undeclaredTags(paths, cfg.Tags)...)
	if len(spec.Tags) == 0 {
		spec.Tags = nil
	}

	if cfg.ExternalDocs != nil {
		spec.ExternalDocs = &ExternalDocs{
			Description: cfg.ExternalDocs.Description,
			URL:         cfg.ExternalDocs.URL,
		}
	}

	// The License Object allows identifier or url, never both, and identifier requires 3.1+.
	if spec.Info.License != nil && spec.Info.License.Identifier != "" {
		licenseCopy := b.dialect.license(*spec.Info.License)
		spec.Info.License = &licenseCopy
	}

	if b.dialect.hasJSONSchemaDialect() {
		spec.Info.Summary = cfg.Summary
		spec.JSONSchemaDialect = cfg.JSONSchemaDialect
		if len(cfg.Webhooks) > 0 {
			spec.Webhooks = maps.Clone(cfg.Webhooks)
		}
	}

	if b.dialect.hasSelf() {
		spec.Self = cfg.Self
	}

	spec.Components = buildComponents(cfg, reg, schemes)

	return spec
}

// buildServers prefers Config.Servers and falls back to Config.ServerURL.
// copyLicense returns a License the document owns, so nothing built from it can alias Config.
func copyLicense(license *License) *License {
	if license == nil {
		return nil
	}
	licenseCopy := *license
	return &licenseCopy
}

func buildServers(cfg *Config, d dialect) []Server {
	allowName := d.hasServerName()
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

// buildComponents merges Components, SecuritySchemes and inferred entries without mutating the inputs.
func buildComponents(cfg *Config, reg *schemaRegistry, schemes *securitySchemes) map[string]any {
	registered := reg.componentSchemas()
	inferred := schemes.declarations()
	if len(cfg.Components) == 0 && len(cfg.SecuritySchemes) == 0 && len(registered) == 0 && len(inferred) == 0 {
		return nil
	}

	components := make(map[string]any, len(cfg.Components)+2)
	maps.Copy(components, cfg.Components)

	if len(registered) > 0 {
		// The registry never claims a name the user's schemas hold.
		schemas := make(map[string]any, len(registered))
		maps.Copy(schemas, registered)
		maps.Copy(schemas, stringKeyedEntries(components["schemas"]))
		components["schemas"] = schemas
	}

	if len(cfg.SecuritySchemes) > 0 || len(inferred) > 0 {
		// Inferred schemes go first so user-supplied ones with the same name replace them.
		merged := make(map[string]any, len(cfg.SecuritySchemes)+len(inferred))
		maps.Copy(merged, inferred)
		maps.Copy(merged, stringKeyedEntries(components["securitySchemes"]))
		maps.Copy(merged, cfg.SecuritySchemes)
		components["securitySchemes"] = merged
	}

	return components
}

// stringKeyedEntries reads any string-keyed map as map[string]any, since a typed
// map such as map[string]MyScheme fails a plain assertion and would be dropped.
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
