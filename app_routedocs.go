package fiber

import (
	"fmt"
	"mime"
	"reflect"
	"strings"

	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

func docSetSummary(sum string) func(route *Route) {
	return func(route *Route) { route.Summary = sum }
}

func docSetDescription(desc string) func(route *Route) {
	return func(route *Route) { route.Description = desc }
}

// Summary assigns a short summary to the most recently added route.
func (app *App) Summary(sum string) Router {
	app.applyToLatest(docSetSummary(sum))
	return app
}

// Description assigns a description to the most recently added route.
func (app *App) Description(desc string) Router {
	app.applyToLatest(docSetDescription(desc))
	return app
}

// validateMediaType panics unless typ is a "type/subtype" media type, and returns it.
func validateMediaType(typ string) string {
	if _, _, err := mime.ParseMediaType(typ); err != nil || !strings.Contains(typ, "/") {
		panic("invalid media type: " + typ)
	}
	return typ
}

// optionalMediaType trims and validates typ; empty clears the route's setting.
func optionalMediaType(typ string) string {
	typ = utils.TrimSpace(typ)
	if typ == "" {
		return ""
	}
	return validateMediaType(typ)
}

func normalizeParamLocation(in string) string {
	return utilsstrings.ToLower(utils.TrimSpace(in))
}

func docSetConsumes(typ string) func(route *Route) {
	typ = optionalMediaType(typ)
	return func(route *Route) { route.Consumes = typ }
}

func docSetProduces(typ string) func(route *Route) {
	typ = optionalMediaType(typ)
	return func(route *Route) { route.Produces = typ }
}

// Consumes assigns a request media type to the most recently added route.
func (app *App) Consumes(typ string) Router {
	app.applyToLatest(docSetConsumes(typ))
	return app
}

// Produces assigns a response media type to the most recently added route.
func (app *App) Produces(typ string) Router {
	app.applyToLatest(docSetProduces(typ))
	return app
}

// RequestBody documents the request payload; the short form of RequestBodyWithExample.
func (app *App) RequestBody(description string, required bool, mediaTypes ...string) Router {
	return app.RequestBodyWithExample(description, required, nil, "", nil, nil, mediaTypes...)
}

func hasSchema(schema any) bool {
	switch value := schema.(type) {
	case nil:
		return false
	case map[string]any:
		return len(value) > 0
	default:
		return true
	}
}

// copySchema detaches a schema map from its caller; Go model values are kept as is since only their type is read.
func copySchema(schema any) any {
	if value, ok := schema.(map[string]any); ok {
		if len(value) == 0 {
			return nil
		}
		return copyAnyMap(value)
	}
	return schema
}

func docRequestBodyWithExample(description string, required bool, schema any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) func(route *Route) {
	// No media type is fine: the middleware documents its DefaultConsumes.
	sanitized := sanitizeMediaTypes(mediaTypes)

	body := &RouteRequestBody{
		Description: description,
		Required:    required,
		MediaTypes:  sanitized,
		SchemaRef:   schemaRef,
		Example:     example,
		Examples:    examples,
	}
	if schemaRef != "" {
		body.Schema = map[string]any{openapiRefKey: schemaRef}
	} else if hasSchema(schema) {
		body.Schema = schema
	}

	return func(route *Route) {
		route.RequestBody = cloneRouteRequestBody(body)
		// Adopt the body's media type only when Consumes() set none.
		if len(sanitized) > 0 && route.Consumes == "" {
			route.Consumes = sanitized[0]
		}
	}
}

// RequestBodyWithExample documents the request payload with schema references and examples.
func (app *App) RequestBodyWithExample(description string, required bool, schema any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) Router {
	app.applyToLatest(docRequestBodyWithExample(description, required, schema, schemaRef, example, examples, mediaTypes...))
	return app
}

// Parameter documents an input parameter; the short form of AddParameter.
func (app *App) Parameter(name, in string, required bool, schema any, description string) Router {
	return app.ParameterWithExample(name, in, required, schema, "", description, nil, nil)
}

// ParameterWithExample documents an input parameter with schema references and examples.
func (app *App) ParameterWithExample(name, in string, required bool, schema any, schemaRef, description string, example any, examples map[string]any) Router {
	return app.AddParameter(newRouteParameter(name, in, required, schema, schemaRef, description, example, examples))
}

//nolint:gocritic // hugeParam: by-value keeps the chainable route-helper API ergonomic.
func docAddParameter(param RouteParameter) func(route *Route) {
	if utils.TrimSpace(param.Name) == "" {
		panic("parameter name is required")
	}

	location := normalizeParamLocation(param.In)
	switch location {
	// OpenAPI 3.2: the whole query string as one value, described by content.
	case ParamInPath, ParamInQuery, ParamInHeader, ParamInCookie, ParamInQuerystring:
	default:
		panic("invalid parameter location: " + param.In)
	}
	param.In = location

	injectType := false
	switch {
	case len(param.Content) > 0:
		// A Parameter Object has a schema or exactly one content entry, never both.
		if len(param.Content) > 1 {
			panic("parameter content must contain exactly one media type: " + param.Name)
		}
		param.Content = sanitizeContentMediaTypes(param.Content)
		param.Schema = nil
		param.SchemaRef = ""
	case param.SchemaRef != "":
		param.Schema = map[string]any{openapiRefKey: param.SchemaRef}
	case location == ParamInQuerystring:
		// 3.2 querystring uses content, so no default schema.
	default:
		injectType = true
	}

	if location == ParamInPath {
		param.Required = true
	}

	return func(route *Route) {
		paramCopy := param
		paramCopy.Schema = copySchema(param.Schema)
		if injectType {
			// Only a map or no schema gets the string default; a Go value documents its own type.
			schema, ok := paramCopy.Schema.(map[string]any)
			if paramCopy.Schema == nil {
				schema, ok = map[string]any{}, true
			}
			if ok {
				if _, has := schema["type"]; !has {
					schema["type"] = openapiTypeString
				}
				paramCopy.Schema = schema
			}
		}
		// Copy so a map or slice example does not alias the caller.
		paramCopy.Example = copyAnyValue(param.Example)
		paramCopy.Examples = copyAnyMap(param.Examples)
		paramCopy.Content = cloneRouteMediaTypeMap(param.Content)
		if param.Explode != nil {
			explode := *param.Explode
			paramCopy.Explode = &explode
		}
		route.Parameters = append(route.Parameters, paramCopy)
	}
}

// AddParameter documents a parameter using the full RouteParameter.
//
//nolint:gocritic // hugeParam: by-value keeps the chainable route-helper API ergonomic.
func (app *App) AddParameter(param RouteParameter) Router {
	app.applyToLatest(docAddParameter(param))
	return app
}

// Response documents an HTTP response; the short form of ResponseWithExample.
func (app *App) Response(status int, description string, mediaTypes ...string) Router {
	return app.addResponse(status, description, nil, "", nil, nil, mediaTypes...)
}

// ResponseWithExample documents an HTTP response with schema references and examples.
func (app *App) ResponseWithExample(status int, description string, schema any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) Router {
	return app.addResponse(status, description, schema, schemaRef, example, examples, mediaTypes...)
}

func responseKey(status int) string {
	if status == 0 {
		return defaultResponseKey
	}
	if status < 100 || status > 599 {
		panic("invalid status code")
	}
	return utils.FormatInt(int64(status))
}

func defaultResponseDescription(status int) string {
	if status == 0 {
		return "Default response"
	}
	if text := utils.StatusMessage(status); text != "" {
		return text
	}
	return "Status " + utils.FormatInt(int64(status))
}

// getOrCreateResponse returns the entry for key, creating it if absent. The caller must hold app.mutex.
func getOrCreateResponse(route *Route, key string, status int) RouteResponse {
	if route.Responses == nil {
		route.Responses = make(map[string]RouteResponse)
	}
	resp, ok := route.Responses[key]
	if !ok {
		resp = RouteResponse{Description: defaultResponseDescription(status)}
	}
	return resp
}

func docAddResponse(status int, description string, schema any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) func(route *Route) {
	sanitized := sanitizeMediaTypes(mediaTypes)

	if description == "" {
		description = defaultResponseDescription(status)
	}

	key := responseKey(status)

	resp := RouteResponse{Description: description, MediaTypes: sanitized, Example: example, Examples: examples}
	if schemaRef != "" {
		resp.SchemaRef = schemaRef
		resp.Schema = map[string]any{openapiRefKey: schemaRef}
	} else if hasSchema(schema) {
		resp.Schema = schema
	}

	return func(route *Route) {
		if route.Responses == nil {
			route.Responses = make(map[string]RouteResponse)
		}
		copyResp := resp
		copyResp.MediaTypes = append([]string(nil), resp.MediaTypes...)
		copyResp.Schema = copySchema(resp.Schema)
		copyResp.Example = copyAnyValue(resp.Example)
		copyResp.Examples = copyAnyMap(resp.Examples)
		// Keep headers, links and content documented earlier.
		if existing, ok := route.Responses[key]; ok {
			copyResp.Headers = existing.Headers
			copyResp.Links = existing.Links
			copyResp.Content = existing.Content
		}
		route.Responses[key] = copyResp
		// Adopt the response media type only when Produces() set none.
		if status == StatusOK && len(copyResp.MediaTypes) > 0 && route.Produces == "" {
			route.Produces = copyResp.MediaTypes[0]
		}
	}
}

func (app *App) addResponse(status int, description string, schema any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) Router {
	app.applyToLatest(docAddResponse(status, description, schema, schemaRef, example, examples, mediaTypes...))
	return app
}

func sanitizeMediaTypes(mediaTypes []string) []string {
	if len(mediaTypes) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(mediaTypes))
	sanitized := make([]string, 0, len(mediaTypes))
	for _, typ := range mediaTypes {
		trimmed := utils.TrimSpace(typ)
		if trimmed == "" {
			continue
		}
		validateMediaType(trimmed)
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		sanitized = append(sanitized, trimmed)
	}
	if len(sanitized) == 0 {
		return nil
	}
	return sanitized
}

func docSetTags(tags ...string) func(route *Route) {
	return func(route *Route) {
		route.Tags = append([]string(nil), tags...)
	}
}

func docSetDeprecated() func(route *Route) {
	return func(route *Route) { route.Deprecated = true }
}

func docSetSecurity(requirements ...map[string][]string) func(route *Route) {
	return func(route *Route) {
		if len(requirements) == 0 {
			// No requirement means no authentication, which differs from unset.
			route.Security = []map[string][]string{}
			return
		}
		route.Security = cloneRouteSecurity(requirements)
	}
}

func docSetHidden() func(route *Route) {
	return func(route *Route) { route.hidden = true }
}

// Tags assigns tags to the most recently added route.
func (app *App) Tags(tags ...string) Router {
	app.applyToLatest(docSetTags(tags...))
	return app
}

// Deprecated marks the most recently added route as deprecated.
func (app *App) Deprecated() Router {
	app.applyToLatest(docSetDeprecated())
	return app
}

// Security sets the requirements for the most recently added route, combined with
// OR. With no requirement the route is documented as needing no authentication,
// and a route that sets its own security gets none inferred from its middleware.
func (app *App) Security(requirements ...map[string][]string) Router {
	app.applyToLatest(docSetSecurity(requirements...))
	return app
}

// Hidden excludes the most recently added route from the generated OpenAPI spec.
func (app *App) Hidden() Router {
	app.applyToLatest(docSetHidden())
	return app
}

func docResponseHeader(status int, name, description string, schema any) func(route *Route) {
	if utils.TrimSpace(name) == "" {
		panic("response header name is required")
	}

	header := map[string]any{}
	if description != "" {
		header["description"] = description
	}
	if hasSchema(schema) {
		header["schema"] = schema
	} else {
		// A Header Object needs a schema or content, so default to a string schema.
		header["schema"] = map[string]any{"type": openapiTypeString}
	}

	return docSetResponseEntry(status, name, header, func(resp *RouteResponse) *map[string]any { return &resp.Headers })
}

func docOperationExternalDocs(description, url string) func(route *Route) {
	docs := map[string]any{"url": url}
	if description != "" {
		docs["description"] = description
	}
	return func(route *Route) {
		route.ExternalDocs = copyAnyMap(docs)
	}
}

func docOperationExtension(fields map[string]any) func(route *Route) {
	if len(fields) == 0 {
		// A nil mutation is a no-op.
		return nil
	}
	return func(route *Route) {
		if route.OperationExtensions == nil {
			route.OperationExtensions = make(map[string]any, len(fields))
		}
		for key, value := range fields {
			route.OperationExtensions[key] = copyAnyValue(value)
		}
	}
}

// sanitizeContentMediaTypes returns content keyed by trimmed, validated media types
// so a padded key never reaches the generated document.
func sanitizeContentMediaTypes(content map[string]RouteMediaType) map[string]RouteMediaType {
	rekey := false
	for mediaType := range content {
		if validateMediaType(utils.TrimSpace(mediaType)) != mediaType {
			rekey = true
		}
	}
	if !rekey {
		return content
	}

	sanitized := make(map[string]RouteMediaType, len(content))
	for mediaType, entry := range content {
		trimmed := utils.TrimSpace(mediaType)
		if _, ok := sanitized[trimmed]; ok {
			panic("duplicate media type in content: " + trimmed)
		}
		sanitized[trimmed] = entry
	}
	return sanitized
}

func docRequestBodyContent(description string, required bool, content map[string]RouteMediaType) func(route *Route) {
	content = sanitizeContentMediaTypes(content)

	body := &RouteRequestBody{
		Description: description,
		Required:    required,
		Content:     content,
	}
	return func(route *Route) {
		route.RequestBody = cloneRouteRequestBody(body)
	}
}

func docResponseContent(status int, description string, content map[string]RouteMediaType) func(route *Route) {
	content = sanitizeContentMediaTypes(content)

	key := responseKey(status)
	return func(route *Route) {
		resp := getOrCreateResponse(route, key, status)
		// An empty description keeps what an earlier call set.
		if description != "" {
			resp.Description = description
		}
		resp.Content = cloneRouteMediaTypeMap(content)
		route.Responses[key] = resp
	}
}

func docResponseLink(status int, name string, link map[string]any) func(route *Route) {
	if utils.TrimSpace(name) == "" {
		panic("response link name is required")
	}
	return docSetResponseEntry(status, name, link, func(resp *RouteResponse) *map[string]any { return &resp.Links })
}

func docSetResponseEntry(status int, name string, entry map[string]any, pick func(*RouteResponse) *map[string]any) func(route *Route) {
	key := responseKey(status)
	return func(route *Route) {
		resp := getOrCreateResponse(route, key, status)
		entries := pick(&resp)
		if *entries == nil {
			*entries = make(map[string]any)
		}
		copied := copyAnyMap(entry)
		if copied == nil {
			copied = map[string]any{}
		}
		(*entries)[name] = copied
		route.Responses[key] = resp
	}
}

// ResponseHeader documents a response header for a status code; 0 is the "default" response.
func (app *App) ResponseHeader(status int, name, description string, schema any) Router {
	app.applyToLatest(docResponseHeader(status, name, description, schema))
	return app
}

func newRouteParameter(name, in string, required bool, schema any, schemaRef, description string, example any, examples map[string]any) RouteParameter { //nolint:revive // flag-parameter: required is a Parameter Object field, not control flow
	return RouteParameter{
		Name:        name,
		In:          in,
		Required:    required,
		Schema:      schema,
		SchemaRef:   schemaRef,
		Description: description,
		Example:     example,
		Examples:    examples,
	}
}

func docAccepts(model any, mediaTypes ...string) func(route *Route) {
	return docRequestBodyWithExample("", true, model, "", nil, nil, mediaTypes...)
}

func docReturns(status int, model any, mediaTypes ...string) func(route *Route) {
	return docAddResponse(status, "", model, "", nil, nil, mediaTypes...)
}

// docSetInnerHandlers records the unwrapped handlers of a wrapped route; merges append to both chains so they stay aligned.
func docSetInnerHandlers(inner []Handler) func(route *Route) {
	return func(route *Route) { route.docHandlers = append(route.docHandlers, inner...) }
}

// Accepts documents a required request body whose schema is reflected from model,
// under the given media types or the middleware's DefaultConsumes.
func (app *App) Accepts(model any, mediaTypes ...string) Router {
	app.applyToLatest(docAccepts(model, mediaTypes...))
	return app
}

// Returns documents a response whose schema is reflected from model, under the
// given media types or the middleware's DefaultProduces. A nil model documents the status alone.
func (app *App) Returns(status int, model any, mediaTypes ...string) Router {
	app.applyToLatest(docReturns(status, model, mediaTypes...))
	return app
}

// Params documents the exported fields of a struct model as parameters in the given
// location ("query", "header", "cookie" or "path"), named by their Bind tags.
// Fields tagged validate:"required" and all path parameters are required.
func (app *App) Params(in string, model any) Router {
	app.applyToLatest(docAddParameterModel(in, model))
	return app
}

func docAddParameterModel(in string, model any) func(route *Route) {
	location := normalizeParamLocation(in)
	switch location {
	case ParamInQuery, ParamInHeader, ParamInCookie, ParamInPath:
	default:
		panic("invalid parameter location: " + in)
	}
	t := reflect.TypeOf(model)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		panic("parameter model must be a struct: " + fmt.Sprint(model))
	}
	return func(route *Route) {
		route.ParameterModels = append(route.ParameterModels, RouteParameterModel{In: location, Model: model})
	}
}

// OperationExternalDocs sets the externalDocs of the most recently added route.
func (app *App) OperationExternalDocs(description, url string) Router {
	app.applyToLatest(docOperationExternalDocs(description, url))
	return app
}

// OperationExtension shallow-merges arbitrary fields (servers, callbacks, x-*) into the operation.
func (app *App) OperationExtension(fields map[string]any) Router {
	app.applyToLatest(docOperationExtension(fields))
	return app
}

// RequestBodyContent documents a request body with a schema per media type.
func (app *App) RequestBodyContent(description string, required bool, content map[string]RouteMediaType) Router {
	app.applyToLatest(docRequestBodyContent(description, required, content))
	return app
}

// ResponseContent documents a response with a schema per media type.
func (app *App) ResponseContent(status int, description string, content map[string]RouteMediaType) Router {
	app.applyToLatest(docResponseContent(status, description, content))
	return app
}

// ResponseLink documents a response link for the given status code.
func (app *App) ResponseLink(status int, name string, link map[string]any) Router {
	app.applyToLatest(docResponseLink(status, name, link))
	return app
}

func (app *App) applyToLatest(apply func(route *Route)) {
	app.mutex.Lock()
	if app.applyToRegIDLocked(app.latestRegID, apply) {
		app.bumpRoutesRevision()
	}
	app.mutex.Unlock()
}

func (app *App) applyNameToRegistration(regID uint64, name string) {
	if regID == 0 {
		return
	}

	app.mutex.Lock()
	named := app.nameRegistrationLocked(regID, name)
	app.mutex.Unlock()

	app.fireOnNameHooks(named)
}

// nameRegistrationLocked names the entries of regID and returns a snapshot of the
// last for the OnName hooks, or nil. The caller must hold app.mutex.
func (app *App) nameRegistrationLocked(regID uint64, name string) *Route {
	named := app.nameRoutesLocked(regID, name)
	if named == nil {
		return nil
	}
	app.bumpRoutesRevision()
	if len(app.hooks.onName) == 0 {
		return nil
	}
	// Snapshot under the lock: the hook runs without it.
	return app.copyRoute(named)
}

// fireOnNameHooks runs the OnName hooks, panicking on error like registration. Callers must not hold app.mutex.
func (app *App) fireOnNameHooks(named *Route) {
	if named == nil {
		return
	}
	if err := app.hooks.executeOnNameHooks(named); err != nil {
		panic(err)
	}
}

func (app *App) applyToRegistration(regID uint64, apply func(route *Route)) {
	if regID == 0 || apply == nil {
		return
	}
	app.mutex.Lock()
	if app.applyToRegIDLocked(regID, apply) {
		app.bumpRoutesRevision()
	}
	app.mutex.Unlock()
}

// applyToRegIDLocked applies a mutation to every entry of regID except mount
// placeholders, and reports whether any was touched. The caller must hold app.mutex.
func (app *App) applyToRegIDLocked(regID uint64, apply func(route *Route)) bool {
	applied := false
	for _, route := range app.regEntries[regID] {
		if route.mount {
			continue
		}
		apply(route)
		applied = true
	}
	return applied
}

// nameRoutesLocked names the entries of regID and the automatic HEAD twin of each
// GET, returning the last named. The caller must hold app.mutex.
func (app *App) nameRoutesLocked(regID uint64, name string) *Route {
	var (
		gets  []*Route
		named *Route
	)
	app.applyToRegIDLocked(regID, func(route *Route) {
		route.Name = name
		if route.group != nil {
			route.Name = route.group.name + route.Name
		}
		if route.Method == MethodGet && !route.use {
			gets = append(gets, route)
		}
		named = route
	})
	if len(gets) == 0 {
		return named
	}
	headIndex := app.methodInt(MethodHead)
	if headIndex == -1 {
		return named
	}
	for _, get := range gets {
		if _, twin := app.autoHeadTwinLocked(headIndex, app.autoHeadKey(get)); twin != nil {
			twin.Name = get.Name
		}
	}
	return named
}

// namedRouteIndex is an immutable by-name snapshot of the routes at one revision;
// the first route in stack order wins for each name.
type namedRouteIndex struct {
	routes   map[string]*Route
	small    []*Route // the same snapshots in registration order when there are few, to scan instead of hash
	revision uint64
}

// namedRoute returns the shared snapshot of the route called name, or nil; callers
// must not modify it. The index is rebuilt under the lock after a change and read
// lock-free, as published snapshots are never mutated.
func (app *App) namedRoute(name string) *Route {
	index := app.namedRoutes.Load()
	if index == nil || index.revision != app.routesRevision.Load() {
		index = app.indexNamedRoutes()
	}
	if index.small != nil {
		for _, route := range index.small {
			if route.Name == name {
				return route
			}
		}
		return nil
	}
	return index.routes[name]
}

// indexNamedRoutes rebuilds the index unless another goroutine already has. Revision
// bumps happen under the lock, so a build under it matches its revision.
func (app *App) indexNamedRoutes() *namedRouteIndex {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	revision := app.routesRevision.Load()
	if index := app.namedRoutes.Load(); index != nil && index.revision == revision {
		return index
	}

	index := &namedRouteIndex{revision: revision, routes: make(map[string]*Route)}
	var inOrder []*Route
	for _, routes := range app.stack {
		for _, route := range routes {
			if _, taken := index.routes[route.Name]; taken {
				continue
			}
			snapshot := new(Route)
			app.copyRouteInto(snapshot, route)
			index.routes[route.Name] = snapshot
			inOrder = append(inOrder, snapshot)
		}
	}
	if len(inOrder) <= smallIndexMax {
		index.small = inOrder
		if index.small == nil {
			index.small = []*Route{}
		}
	}
	app.namedRoutes.Store(index)
	return index
}

// GetRoute returns the route with the given name as a deep copy.
func (app *App) GetRoute(name string) (found Route) { //nolint:nonamedreturns // the named result is what keeps this to a single struct move
	snapshot := app.namedRoute(name)
	if snapshot == nil {
		return found
	}
	found = *snapshot
	found.group = nil
	if snapshot.isDocumented() {
		app.cloneRouteDocInto(&found, snapshot)
	}
	return found
}
