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
	querystringMediaType = "application/x-www-form-urlencoded"

	schemaKeyType       = "type"
	schemaKeyEnum       = "enum"
	schemaKeyAllOf      = "allOf"
	schemaKeyOneOf      = "oneOf"
	schemaKeyAnyOf      = "anyOf"
	schemaKeyAdditional = "additionalProperties"
	schemaKeyRequired   = "required"
	schemaKeyNot        = "not"
	schemaKeyProperties = "properties"
	locCookie           = "cookie"
	locHeader           = "header"
	ruleContains        = "contains" // the validate rule, and the schema keyword of that name
	keyDefault          = "default"  // the default response, and the schema keyword of that name
	keyNameField        = "name"
	schemaKeyItems      = "items"
	schemaTypeNull      = "null"
	schemaKeyRef        = "$ref"
	schemaTypeArray     = "array"
	schemaKeyFormat     = "format"
	schemaTypeString    = "string"
	schemaTypeObject    = "object"
	schemaTypeBoolean   = "boolean"
	schemaTypeInteger   = "integer"
	schemaTypeNumber    = "number"
)

// openAPIOperationMethods is the set of methods a Path Item can express; `query` is gated by the caller.
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
	pathExtras        map[string]pathExtra            // Path Item fields beside the operations, merged at marshal time
	extensions        map[string]any                  // Arbitrary document-object fields, merged at marshal time
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
	extensions     map[string]any // Arbitrary info-object fields, merged at marshal time
	Contact        *Contact       `json:"contact,omitempty"`
	License        *License       `json:"license,omitempty"`
	Title          string         `json:"title"`
	Version        string         `json:"version"`
	Summary        string         `json:"summary,omitempty"`
	Description    string         `json:"description,omitempty"`
	TermsOfService string         `json:"termsOfService,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
}

// MarshalJSON writes the info object with its extension fields.
//
//nolint:gocritic // hugeParam: a value receiver is required so the object is marshaled through this method
func (i openAPIInfo) MarshalJSON() ([]byte, error) {
	type alias openAPIInfo
	base, err := json.Marshal(alias(i))
	if err != nil {
		return nil, fmt.Errorf("openapi: marshal info: %w", err)
	}
	return withExtensions(base, i.extensions, "info")
}

type operation struct {
	Responses    map[string]response `json:"responses"`
	RequestBody  *requestBody        `json:"requestBody,omitempty"`  //nolint:tagliatelle // OpenAPI spec uses camelCase
	ExternalDocs map[string]any      `json:"externalDocs,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	extensions   map[string]any      // Arbitrary operation-object fields, merged at marshal time
	Callbacks    map[string]any      `json:"callbacks,omitempty"`

	Security *[]map[string][]string `json:"security,omitempty"`

	OperationID string `json:"operationId,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`

	Parameters []parameter `json:"parameters,omitempty"`
	Tags       []string    `json:"tags,omitempty"`
	Servers    []Server    `json:"servers,omitempty"`

	Deprecated bool `json:"deprecated,omitempty"`

	ownServers bool // the servers were set on the route itself, so a domain route sharing the path adds none
}

//nolint:gocritic // hugeParam: a value receiver is required so map values are marshaled through this method
func (o operation) MarshalJSON() ([]byte, error) {
	type alias operation
	base, err := json.Marshal(alias(o))
	if err != nil {
		return nil, fmt.Errorf("openapi: marshal operation: %w", err)
	}
	return withExtensions(base, o.extensions, "operation")
}

// withExtensions adds the extension fields to the JSON object base, leaving out any whose key base already has.
func withExtensions(base []byte, extensions map[string]any, what string) ([]byte, error) {
	if len(extensions) == 0 {
		return base, nil
	}
	var existing map[string]json.RawMessage
	if err := json.Unmarshal(base, &existing); err != nil {
		return nil, fmt.Errorf("openapi: merge %s extensions: %w", what, err)
	}

	keys := make([]string, 0, len(extensions))
	for key := range extensions {
		if _, taken := existing[key]; taken {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return base, nil
	}
	slices.Sort(keys)

	type extensionPair struct{ key, value []byte }
	pairs := make([]extensionPair, 0, len(keys))
	size := len(base)
	for _, key := range keys {
		encodedKey, kErr := json.Marshal(key)
		if kErr != nil {
			return nil, fmt.Errorf("openapi: marshal %s extension key %q: %w", what, key, kErr)
		}
		encodedValue, vErr := json.Marshal(extensions[key])
		if vErr != nil {
			return nil, fmt.Errorf("openapi: marshal %s extension %q: %w", what, key, vErr)
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

// pathExtra holds the fields of a Path Item that are not one of its operations.
type pathExtra struct {
	additional  map[string]operation // 3.2 additionalOperations: operations for methods the Path Item has no field for
	summary     string
	description string
	parameters  []parameter
}

// MarshalJSON writes the document with its path items completed from pathExtras and its own extension fields.
//
//nolint:gocritic // hugeParam: a value receiver is required so the document is marshaled through this method
func (s openAPISpec) MarshalJSON() ([]byte, error) {
	type plain openAPISpec
	if len(s.pathExtras) == 0 {
		base, err := json.Marshal(plain(s))
		if err != nil {
			return nil, fmt.Errorf("openapi: marshal document: %w", err)
		}
		return withExtensions(base, s.extensions, "document")
	}

	items := make(map[string]map[string]any, len(s.Paths))
	for path, operations := range s.Paths {
		item := make(map[string]any, len(operations)+1)
		for method, op := range operations {
			item[method] = op
		}
		items[path] = item
	}
	for path, extra := range s.pathExtras {
		item := items[path]
		if item == nil {
			item = make(map[string]any)
			items[path] = item
		}
		if extra.summary != "" {
			item["summary"] = extra.summary
		}
		if extra.description != "" {
			item["description"] = extra.description
		}
		if len(extra.parameters) > 0 {
			item["parameters"] = extra.parameters
		}
		if len(extra.additional) > 0 {
			item["additionalOperations"] = extra.additional
		}
	}
	base, err := json.Marshal(struct {
		Paths map[string]map[string]any `json:"paths"`
		plain
	}{Paths: items, plain: plain(s)})
	if err != nil {
		return nil, fmt.Errorf("openapi: marshal document: %w", err)
	}
	return withExtensions(base, s.extensions, "document")
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
	In              ParamLocation             `json:"in"`
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

func isOpenAPIOperationMethod(method string) bool {
	_, ok := openAPIOperationMethods[method]
	return ok
}

// additionalOperationPrefix marks, in a path's operation map, an operation of a method the Path Item has no
// field for; the document moves it to additionalOperations.
const additionalOperationPrefix = "additional:"

// operationKey is the key of an operation in its path's operation map.
func operationKey(method string) string {
	if isOpenAPIOperationMethod(method) {
		return utilsstrings.ToLower(method)
	}
	return additionalOperationPrefix + method
}

func generateOperationID(method, path string) string {
	var b strings.Builder
	_, _ = b.WriteString(utilsstrings.ToLower(method)) //nolint:errcheck // strings.Builder.WriteString never returns an error
	capNext := true
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch {
		case isLower(c):
			if capNext {
				c -= 'a' - 'A'
				capNext = false
			}
			_ = b.WriteByte(c) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		case isUpper(c) || isASCIIDigit(c):
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

// operationIDFromName makes a route name safe as an operationId: letters, digits, "_", "-" and "." are kept,
// other runs become "_", and a leading digit gets an "op_" prefix.
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
	if id != "" && isASCIIDigit(id[0]) {
		return "op_" + id
	}
	return id
}

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

// undeclaredTags lists, sorted, the tags operations use that Config.Tags does not describe.
func undeclaredTags(paths map[string]map[string]operation, extras map[string]pathExtra, declared []Tag) []Tag {
	known := make(map[string]struct{}, len(declared))
	for i := range declared {
		known[declared[i].Name] = struct{}{}
	}
	var names []string
	note := func(op *operation) {
		for _, name := range op.Tags {
			if _, ok := known[name]; !ok {
				known[name] = struct{}{}
				names = append(names, name)
			}
		}
	}
	for _, item := range paths {
		for method := range item {
			op := item[method]
			note(&op)
		}
	}
	for _, extra := range extras {
		for method := range extra.additional {
			op := extra.additional[method]
			note(&op)
		}
	}
	slices.Sort(names)
	tags := make([]Tag, len(names))
	for i, name := range names {
		tags[i] = Tag{Name: name}
	}
	return tags
}

type specEnv struct {
	validator fiber.StructValidator
	equal     segmentEqual
	strict    bool
}

type specBuilder struct {
	cfg              *Config
	reg              *schemaRegistry
	schemes          *securitySchemes
	paths            map[string]map[string]operation
	usedOperationIDs map[string]struct{}
	hierarchyPaths   map[string]canonicalPathItem
	env              specEnv
	policy           routePolicy
	dialect          dialect
}

func generateSpec(routes []fiber.Route, cfg *Config, env specEnv) openAPISpec {
	version := newDialect(cfg.OpenAPIVersion)
	b := &specBuilder{
		cfg:              cfg,
		env:              env,
		dialect:          version,
		reg:              newSchemaRegistry(cfg, version),
		schemes:          newSecuritySchemes(cfg),
		paths:            make(map[string]map[string]operation),
		usedOperationIDs: make(map[string]struct{}),
		hierarchyPaths:   make(map[string]canonicalPathItem),
		policy:           newRoutePolicy(cfg, env),
	}
	b.addRoutes(routes)
	return b.document()
}

func (b *specBuilder) addRoutes(routes []fiber.Route) {
	var (
		coveredMethod string
		covering      []coveringMiddleware
	)
	for i := range routes {
		r := &routes[i]
		host, reachable := effectiveHost(r.Domains())
		if !reachable {
			continue
		}
		if r.Method != coveredMethod {
			coveredMethod = r.Method
			covering = covering[:0]
		}
		if r.IsMiddleware() {
			if b.policy.middlewareInference {
				if set := handlerMiddleware(r.InnerHandlers(), b.policy.custom); set != 0 {
					covering = append(covering, coveringMiddleware{prefix: r.Path, domain: host, set: set})
				}
			}
			continue
		}
		if !b.documents(r) {
			continue
		}
		facts := b.inferRoute(r, covering)
		facts.host = host
		for _, variant := range buildOpenAPIPathVariants(r.Path, r.Params, b.env.strict) {
			b.addVariant(r, &facts, &variant)
		}
	}
}

func (b *specBuilder) documents(r *fiber.Route) bool {
	if !isOpenAPIOperationMethod(r.Method) && !b.dialect.hasAdditionalOperations() {
		return false
	}
	if r.Method == fiber.MethodQuery && !b.dialect.hasQueryMethod() {
		return false
	}
	if r.IsAutoHead() {
		return false
	}
	if op := OperationOf(r); op != nil && op.Hidden {
		return false
	}
	if b.cfg.Filter != nil {
		route := *r
		return b.cfg.Filter(&route)
	}
	return true
}

func (b *specBuilder) inferRoute(r *fiber.Route, covering []coveringMiddleware) routeFacts {
	facts := resolveRoute(r, covering, &b.policy)
	if facts.security == nil {
		if requirement := b.schemes.requirement(facts.chain); requirement != nil {
			security := []map[string][]string{requirement}
			facts.security = &security
		} else {
			facts.security = facts.defaultSecurity
		}
	}
	return facts
}

// declareParameters adds the parameters the route's models and middleware declare, once an operation of the route is kept.
func (b *specBuilder) declareParameters(r *fiber.Route, facts *routeFacts) {
	if facts.parametersDeclared {
		return
	}
	facts.parametersDeclared = true
	facts.declared = expandParameterModels(facts.doc.ParameterModels, b.reg)
	var inferred []RouteParameter
	if facts.chain.has(kindCSRF) && !isSafeMethod(r.Method) {
		inferred = append(inferred, csrfParameter(b.cfg.CSRFHeader))
	}
	if facts.chain.has(kindIdempotency) && !isSafeMethod(r.Method) {
		inferred = append(inferred, idempotencyParameter(b.cfg.IdempotencyKeyHeader))
	}
	if facts.chain.has(kindSession) {
		inferred = append(inferred, sessionParameter(b.cfg))
	}
	for i := range b.cfg.Middleware {
		if facts.chain.has(kindCustom + middlewareKind(i)) {
			inferred = append(inferred, b.cfg.Middleware[i].Parameters...)
		}
	}
	facts.declared = append(inferred, facts.declared...)
}

func (b *specBuilder) addVariant(r *fiber.Route, facts *routeFacts, variant *pathVariant) {
	methodLower := operationKey(r.Method)
	if _, exists := b.paths[variant.Path][methodLower]; exists {
		b.addHost(variant.Path, methodLower, facts.host)
		return
	}
	// Templates identical up to parameter names must not coexist in OpenAPI; first wins.
	hierarchy := normalizePathHierarchy(variant.Path)
	if canonical, exists := b.hierarchyPaths[hierarchy]; exists {
		// Join the already published path item, or drop the operation if its method is taken.
		if canonical.path != variant.Path {
			if _, taken := b.paths[canonical.path][methodLower]; taken {
				b.addHost(canonical.path, methodLower, facts.host)
				return
			}
			adoptCanonicalParamNames(variant, canonical.params)
			variant.Path = canonical.path
		}
	} else {
		b.hierarchyPaths[hierarchy] = canonicalPathItem{path: variant.Path, params: variant.ParamNames}
	}

	// Only a route that stays in the document declares the schemes and schemas it uses.
	b.schemes.use(facts.chain)
	b.declareParameters(r, facts)
	if b.paths[variant.Path] == nil {
		b.paths[variant.Path] = make(map[string]operation)
	}
	b.paths[variant.Path][methodLower] = b.buildOperation(r, facts, variant)
}

// addHost lists the host of a route that lost its path and method to an earlier registration on the
// operation that won. An operation for every host already covers it, and one for an any-host route that
// follows becomes one for every host.
func (b *specBuilder) addHost(path, method, host string) {
	op := b.paths[path][method]
	if len(op.Servers) == 0 || op.ownServers {
		return
	}
	if host == "" {
		op.Servers = nil
		b.paths[path][method] = op
		return
	}
	for _, server := range domainServers(host) {
		if !slices.ContainsFunc(op.Servers, func(known Server) bool { return known.URL == server.URL }) {
			op.Servers = append(slices.Clone(op.Servers), server)
		}
	}
	b.paths[path][method] = op
}

func (b *specBuilder) buildOperation(r *fiber.Route, facts *routeFacts, variant *pathVariant) operation {
	cfg := b.cfg
	doc := facts.doc

	params := make([]parameter, 0, len(variant.ParamNames))
	paramIndex := make(map[parameterKey]int, len(variant.ParamNames))
	for _, p := range variant.ParamNames {
		param := parameter{
			Name:     p,
			In:       ParamInPath,
			Required: true,
			Schema:   pathParamSchema(variant.ParamConstraints[p]),
		}
		params = append(params, param)
		paramIndex[parameterKey{param.In, param.Name}] = len(params) - 1
	}
	// What Config.Defaults supplies comes first, so a declared parameter of the same name and location replaces it.
	declared := append(slices.Clone(facts.defaultParams), facts.declared...)
	extras := remapRouteParameters(append(declared, doc.Parameters...), variant.PathParamAliases, variant.ParamNames)
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
	operationID := doc.OperationID
	if operationID == "" {
		operationID = operationIDFromName(r.Name)
	}
	if operationID == "" {
		operationID = generateOperationID(r.Method, variant.Path)
	}
	operationID = uniqueOperationID(operationID, b.usedOperationIDs)

	responses := convertRouteResponses(doc.Responses, facts.respType, b.reg)
	if len(responses) == 0 {
		status, defaultResp := defaultResponseForMethod(r.Method, facts.respType)
		responses = map[string]response{status: defaultResp}
	}
	if facts.chain.has(kindHealthcheck) && len(doc.Responses) == 0 {
		responses = healthResponses(cfg)
	}
	if facts.chain != 0 {
		applyMiddlewareResponses(responses, r.Method, facts.chain, cfg, b.reg)
	}
	if facts.badRequest {
		if _, ok := responses["400"]; !ok {
			responses["400"] = errorResponse("Bad Request", cfg, b.reg)
		}
	}
	for key, resp := range convertRouteResponses(facts.defaultResponses, facts.respType, b.reg) {
		if _, ok := responses[key]; !ok {
			responses[key] = resp
		}
	}
	applyErrorSchema(responses, doc.Responses, cfg, b.reg)

	servers := domainServers(facts.host)
	if len(doc.Servers) > 0 {
		servers = serversFromMaps(doc.Servers)
	}

	b.reg.request = true
	reqBody := buildRequestBody(doc.RequestBody, facts.bodyType, b.reg)
	b.reg.request = false
	if reqBody == nil && facts.ownConsumes != "" {
		reqBody = &requestBody{Content: map[string]map[string]any{facts.ownConsumes: {}}}
	}
	if r.Method == fiber.MethodTrace || (!cfg.DocumentGetBodies && (r.Method == fiber.MethodGet || r.Method == fiber.MethodHead)) {
		reqBody = nil
	}

	return operation{
		OperationID:  operationID,
		Summary:      summary,
		Description:  doc.Description,
		Tags:         facts.tags,
		Deprecated:   doc.Deprecated,
		Parameters:   params,
		RequestBody:  reqBody,
		Responses:    responses,
		Security:     facts.security,
		ExternalDocs: doc.ExternalDocs,
		Servers:      servers,
		ownServers:   len(doc.Servers) > 0,
		Callbacks:    doc.Callbacks,
		extensions:   doc.OperationExtensions,
	}
}

// serversFromMaps reads the Server Objects of Operation.Servers, skipping one that is not a Server Object.
func serversFromMaps(objects []map[string]any) []Server {
	servers := make([]Server, 0, len(objects))
	for _, object := range objects {
		data, err := json.Marshal(object)
		if err != nil {
			continue
		}
		var server Server
		if json.Unmarshal(data, &server) == nil && server.URL != "" {
			servers = append(servers, server)
		}
	}
	return servers
}

// domainServers describes the host of a domain route as an operation-level server, or nil for any host.
// A ":param" label becomes a server variable defaulting to its name.
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

// pathExtras builds the Path Item fields beside the operations for the documented paths Config.PathItems names.
func (b *specBuilder) pathExtras() map[string]pathExtra {
	var extras map[string]pathExtra
	for _, path := range sortedKeys(b.paths) {
		operations := b.paths[path]
		for _, key := range sortedKeys(operations) {
			method, additional := strings.CutPrefix(key, additionalOperationPrefix)
			if !additional {
				continue
			}
			if extras == nil {
				extras = make(map[string]pathExtra)
			}
			extra := extras[path]
			if extra.additional == nil {
				extra.additional = make(map[string]operation)
			}
			extra.additional[method] = operations[key]
			extras[path] = extra
			delete(operations, key)
		}
	}
	for _, path := range sortedKeys(b.cfg.PathItems) {
		if _, documented := b.paths[path]; !documented {
			continue
		}
		item := b.cfg.PathItems[path]
		declared := item.Parameters
		if b.dialect.hasQuerystringLocation() {
			declared = dropQueryAlongsideQuerystring(declared)
		} else {
			declared = dropQuerystringParameters(declared)
		}
		if extras == nil {
			extras = make(map[string]pathExtra)
		}
		extra := extras[path]
		extra.summary, extra.description = item.Summary, item.Description
		extra.parameters = mergeRouteParameters(nil, make(map[parameterKey]int), declared, b.reg)
		extras[path] = extra
	}
	return extras
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
		Paths:      paths,
		extensions: cfg.Extensions,
	}
	spec.Info.extensions = cfg.InfoExtensions

	spec.Servers = buildServers(cfg, b.dialect)
	spec.pathExtras = b.pathExtras()

	// An empty list is kept: the API as a whole needs no authentication.
	if cfg.Security != nil {
		security := cfg.Security
		spec.Security = &security
	}

	spec.Tags = append([]Tag(nil), cfg.Tags...)
	spec.Tags = append(spec.Tags, undeclaredTags(paths, spec.pathExtras, cfg.Tags)...)
	if len(spec.Tags) == 0 {
		spec.Tags = nil
	}
	if !b.dialect.hasTagMetadata() {
		for i := range spec.Tags {
			spec.Tags[i].Summary, spec.Tags[i].Parent, spec.Tags[i].Kind = "", "", ""
		}
	}

	if cfg.ExternalDocs != nil {
		spec.ExternalDocs = &ExternalDocs{
			Description: cfg.ExternalDocs.Description,
			URL:         cfg.ExternalDocs.URL,
		}
	}

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

// copyLicense returns a License the document owns, so nothing aliases Config.
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

func buildComponents(cfg *Config, reg *schemaRegistry, schemes *securitySchemes) map[string]any {
	registered := reg.componentSchemas()
	inferred := schemes.declarations()
	if len(cfg.Components) == 0 && len(cfg.SecuritySchemes) == 0 && len(registered) == 0 && len(inferred) == 0 {
		return nil
	}

	components := make(map[string]any, len(cfg.Components)+2)
	maps.Copy(components, cfg.Components)

	if len(registered) > 0 {
		schemas := make(map[string]any, len(registered))
		maps.Copy(schemas, registered)
		maps.Copy(schemas, stringKeyedEntries(components["schemas"]))
		components["schemas"] = schemas
	}

	if len(cfg.SecuritySchemes) > 0 || len(inferred) > 0 {
		merged := make(map[string]any, len(cfg.SecuritySchemes)+len(inferred))
		maps.Copy(merged, inferred)
		maps.Copy(merged, stringKeyedEntries(components["securitySchemes"]))
		maps.Copy(merged, cfg.SecuritySchemes)
		components["securitySchemes"] = merged
	}

	return components
}

// stringKeyedEntries reads any string-keyed map as map[string]any; a typed map fails a plain assertion.
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
