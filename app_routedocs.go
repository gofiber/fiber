package fiber

import (
	"fmt"
	"mime"
	"reflect"
	"strings"

	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// The doc* factories below build each helper's mutation, validating and copying
// once so all five routers share one behavior instead of five copies.

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

// validateMediaType panics unless typ is a parseable "type/subtype" media type.
// It returns typ unchanged so callers can validate inline.
func validateMediaType(typ string) string {
	if _, _, err := mime.ParseMediaType(typ); err != nil || !strings.Contains(typ, "/") {
		panic("invalid media type: " + typ)
	}
	return typ
}

// optionalMediaType trims and validates a media type that may be empty, which
// clears the route's setting. A value of only whitespace counts as empty.
func optionalMediaType(typ string) string {
	typ = utils.TrimSpace(typ)
	if typ == "" {
		return ""
	}
	return validateMediaType(typ)
}

// normalizeParamLocation lower-cases and trims a parameter location so "Query "
// and "query" name the same one.
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

// RequestBody documents the request payload for the most recently added route.
// It is the short form of RequestBodyWithExample; use RequestBodyContent when the
// schema or example differs per media type.
func (app *App) RequestBody(description string, required bool, mediaTypes ...string) Router {
	return app.RequestBodyWithExample(description, required, nil, "", nil, nil, mediaTypes...)
}

// hasSchema reports whether a schema argument carries anything: a map with
// entries, or a Go value the OpenAPI middleware reflects into a schema.
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

// copySchema detaches a schema map from its caller. A Go value used as a model
// is kept as is: only its type is read, so nothing can alias into the route.
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

	// Holds the caller's values; cloneRouteRequestBody makes each route its
	// own deep copy.
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

// RequestBodyWithExample documents the request payload with schema references
// and examples, one schema for every media type given.
func (app *App) RequestBodyWithExample(description string, required bool, schema any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) Router {
	app.applyToLatest(docRequestBodyWithExample(description, required, schema, schemaRef, example, examples, mediaTypes...))
	return app
}

// Parameter documents an input parameter for the most recently added route. It
// is the short form of AddParameter, and in is one of the ParamIn constants.
func (app *App) Parameter(name, in string, required bool, schema any, description string) Router {
	return app.ParameterWithExample(name, in, required, schema, "", description, nil, nil)
}

// ParameterWithExample documents an input parameter, including schema
// references and examples. It is the short form of AddParameter.
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
	// "querystring" is an OpenAPI 3.2 location that treats the whole query
	// string as a single value (paired with content rather than schema).
	case ParamInPath, ParamInQuery, ParamInHeader, ParamInCookie, ParamInQuerystring:
	default:
		panic("invalid parameter location: " + param.In)
	}
	param.In = location

	// The per-route copy below is the only one; the caller's map is never
	// written, and a route's default type is injected into its own copy.
	injectType := false
	switch {
	case len(param.Content) > 0:
		// A Parameter Object carries a schema or a content map, never both, and
		// the map holds exactly one entry.
		if len(param.Content) > 1 {
			panic("parameter content must contain exactly one media type: " + param.Name)
		}
		param.Content = sanitizeContentMediaTypes(param.Content)
		param.Schema = nil
		param.SchemaRef = ""
	case param.SchemaRef != "":
		param.Schema = map[string]any{openapiRefKey: param.SchemaRef}
	case location == ParamInQuerystring:
		// 3.2 querystring parameters use content, so no default schema is
		// injected; the middleware wraps whatever was supplied.
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
			// A Go value used as the schema documents its own type; only a
			// map, or no schema at all, takes the string default.
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
		// Example is an `any`: a map or slice would otherwise stay aliased to
		// the caller, as the response helpers already guard against.
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

// AddParameter documents a parameter using the full RouteParameter. Content
// describes it by media type, the only valid form for 3.2 "querystring".
//
//nolint:gocritic // hugeParam: by-value keeps the chainable route-helper API ergonomic.
func (app *App) AddParameter(param RouteParameter) Router {
	app.applyToLatest(docAddParameter(param))
	return app
}

// Response documents an HTTP response for the most recently added route. It is
// the short form of ResponseWithExample; use ResponseContent when the schema or
// example differs per media type.
func (app *App) Response(status int, description string, mediaTypes ...string) Router {
	return app.addResponse(status, description, nil, "", nil, nil, mediaTypes...)
}

// ResponseWithExample documents an HTTP response with schema references and
// examples, one schema for every media type given.
func (app *App) ResponseWithExample(status int, description string, schema any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) Router {
	return app.addResponse(status, description, schema, schemaRef, example, examples, mediaTypes...)
}

// responseKey validates the status code and returns the key of its response
// entry: the numeric code, or "default" for a status of 0.
func responseKey(status int) string {
	if status == 0 {
		return defaultResponseKey
	}
	if status < 100 || status > 599 {
		panic("invalid status code")
	}
	return utils.FormatInt(int64(status))
}

// defaultResponseDescription returns a human-readable description for a response
// status code (0 represents the "default" response).
func defaultResponseDescription(status int) string {
	if status == 0 {
		return "Default response"
	}
	if text := utils.StatusMessage(status); text != "" {
		return text
	}
	return "Status " + utils.FormatInt(int64(status))
}

// getOrCreateResponse returns the response entry for key, creating it with a
// default description when absent. The caller must hold app.mutex.
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

	// Holds the caller's values; the closure copies them for each route.
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
		// Headers, links and content documented earlier belong to the same entry
		// and must survive a later Response call.
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
			// Security() with no requirement documents a route that needs no
			// authentication, which is not the same as one that says nothing.
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

// Security sets the requirements for the most recently added route, combined
// with OR semantics. With no requirement it documents a route that needs no
// authentication, and an empty requirement makes authentication optional. A
// route that sets its own security is not given any inferred from its
// middleware.
func (app *App) Security(requirements ...map[string][]string) Router {
	app.applyToLatest(docSetSecurity(requirements...))
	return app
}

// Hidden excludes the most recently added route from the generated OpenAPI
// specification.
func (app *App) Hidden() Router {
	app.applyToLatest(docSetHidden())
	return app
}

func docResponseHeader(status int, name, description string, schema any) func(route *Route) {
	if utils.TrimSpace(name) == "" {
		panic("response header name is required")
	}

	// The per-route copy in docSetResponseEntry deep-copies the header (the
	// caller's schema map included), so no defensive copy is needed here.
	header := map[string]any{}
	if description != "" {
		header["description"] = description
	}
	if hasSchema(schema) {
		header["schema"] = schema
	} else {
		// A Header Object follows the Parameter Object and needs a schema or a
		// content map, so an omitted schema becomes a string one rather than
		// an invalid header. A supplied schema is stored as given.
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
		// applyToLatest / applyToRegistration treat a nil mutation as a no-op.
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

// sanitizeContentMediaTypes returns content keyed by its trimmed media types,
// panicking on a key the map cannot legally use, matching the validation the
// simpler RequestBody/Response helpers already do. Keying by the validated
// value keeps a padded key from reaching the generated document.
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

	// cloneRouteRequestBody performs the per-route deep copy, so the caller's
	// content map is referenced but never stored.
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
		// An empty description means "unspecified": keep what an earlier call set
		// rather than overwriting it with the canned status message.
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

// docSetResponseEntry stores a copy of entry under name in the map of a
// status's response that pick selects, creating the response when absent.
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

// ResponseHeader documents a response header for a status code, creating the
// response entry if needed. A status of 0 documents the "default" response.
func (app *App) ResponseHeader(status int, name, description string, schema any) Router {
	app.applyToLatest(docResponseHeader(status, name, description, schema))
	return app
}

// newRouteParameter builds the parameter the Parameter and ParameterWithExample
// helpers describe, so each router type states the field list once.
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

// docAccepts documents a required request body whose schema is model's type.
func docAccepts(model any, mediaTypes ...string) func(route *Route) {
	return docRequestBodyWithExample("", true, model, "", nil, nil, mediaTypes...)
}

// docReturns documents a response whose schema is model's type, described by
// the status text.
func docReturns(status int, model any, mediaTypes ...string) func(route *Route) {
	return docAddResponse(status, "", model, "", nil, nil, mediaTypes...)
}

// docSetInnerHandlers records the handlers as registered on a route whose
// chain the router wrapped. A merge appends to both chains, so they stay
// aligned.
func docSetInnerHandlers(inner []Handler) func(route *Route) {
	return func(route *Route) { route.docHandlers = append(route.docHandlers, inner...) }
}

// Accepts documents the request body of the most recently added route as the
// schema of model, a Go value the OpenAPI middleware reflects, under the given
// media types or the middleware's DefaultConsumes when none is given. The body
// is documented as required.
func (app *App) Accepts(model any, mediaTypes ...string) Router {
	app.applyToLatest(docAccepts(model, mediaTypes...))
	return app
}

// Returns documents a response of the most recently added route as the schema
// of model, a Go value the OpenAPI middleware reflects, under the given media
// types or the middleware's DefaultProduces when none is given. The description
// is the status text, and a nil model documents the status alone.
func (app *App) Returns(status int, model any, mediaTypes ...string) Router {
	app.applyToLatest(docReturns(status, model, mediaTypes...))
	return app
}

// Params documents the exported fields of model, a struct or pointer to one, as
// parameters of the most recently added route in the given location: "query",
// "header", "cookie" or "path". Each field is named by its tag for that
// location, the tag Bind reads, or by its name, typed by reflection, and marked
// required by a validate:"required" tag. Path parameters are always required.
func (app *App) Params(in string, model any) Router {
	app.applyToLatest(docAddParameterModel(in, model))
	return app
}

// docAddParameterModel records a struct whose fields the OpenAPI middleware
// documents as parameters of the given location.
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

// OperationExternalDocs sets the externalDocs of the most recently added operation.
func (app *App) OperationExternalDocs(description, url string) Router {
	app.applyToLatest(docOperationExternalDocs(description, url))
	return app
}

// OperationExtension shallow-merges arbitrary fields (e.g. servers, callbacks,
// x-* extensions) into the most recently added operation object.
func (app *App) OperationExtension(fields map[string]any) Router {
	app.applyToLatest(docOperationExtension(fields))
	return app
}

// RequestBodyContent documents a request body with a different schema, example
// and encoding per media type.
func (app *App) RequestBodyContent(description string, required bool, content map[string]RouteMediaType) Router {
	app.applyToLatest(docRequestBodyContent(description, required, content))
	return app
}

// ResponseContent documents a response with a different schema, example and
// encoding per media type for the given status code.
func (app *App) ResponseContent(status int, description string, content map[string]RouteMediaType) Router {
	app.applyToLatest(docResponseContent(status, description, content))
	return app
}

// ResponseLink documents a response link for the given status code, creating the
// response entry if needed.
func (app *App) ResponseLink(status int, name string, link map[string]any) Router {
	app.applyToLatest(docResponseLink(status, name, link))
	return app
}

// applyToLatest locks the router and applies a documentation mutation to the
// most recent registration.
func (app *App) applyToLatest(apply func(route *Route)) {
	app.mutex.Lock()
	if app.applyToRegIDLocked(app.latestRegID, apply) {
		app.bumpRoutesRevision()
	}
	app.mutex.Unlock()
}

// applyNameToRegistration names every route of regID and fires the OnName hooks,
// mirroring App.Name but scoped to one registration. A regID of 0 is a no-op.
func (app *App) applyNameToRegistration(regID uint64, name string) {
	if regID == 0 {
		return
	}

	app.mutex.Lock()
	named := app.nameRegistrationLocked(regID, name)
	app.mutex.Unlock()

	app.fireOnNameHooks(named)
}

// nameRegistrationLocked names every entry of regID and returns a snapshot of
// the last one for the OnName hooks, or nil when nothing was named (a removed
// registration, or a mount placeholder). The caller must hold app.mutex.
func (app *App) nameRegistrationLocked(regID uint64, name string) *Route {
	named := app.nameRoutesLocked(regID, name)
	if named == nil {
		return nil
	}
	app.bumpRoutesRevision()
	if len(app.hooks.onName) == 0 {
		return nil
	}
	// Snapshot under the lock; the hook runs without it and must not read the
	// live route.
	return app.copyRoute(named)
}

// fireOnNameHooks runs the OnName hooks for a named route, panicking on error
// exactly like route registration does. Callers must not hold app.mutex.
func (app *App) fireOnNameHooks(named *Route) {
	if named == nil {
		return
	}
	if err := app.hooks.executeOnNameHooks(named); err != nil {
		panic(err)
	}
}

// applyToRegistration applies a documentation mutation to every route of regID,
// so a scoped router documents its own last registration. A regID of 0 is a no-op.
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

// applyToRegIDLocked applies a mutation to every entry of regID. Mount
// placeholders are indexed so a helper chained onto one is a no-op, but never
// mutated. Reports whether anything was touched; holds app.mutex.
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

// nameRoutesLocked names every entry of regID plus the automatic HEAD twin of
// each GET entry, and returns the last entry named, or nil when there was
// none. The caller must hold app.mutex.
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

// namedRouteIndex is an immutable view of the app's routes by name, taken at one
// revision of the route table. It holds the first route with each name in stack
// order, which is the one a scan would find, and the first unnamed route under
// the empty name.
type namedRouteIndex struct {
	routes   map[string]*Route
	small    []*Route // the same snapshots in registration order when there are few, to scan instead of hash
	revision uint64
}

// namedRoute returns the snapshot of the route called name, or nil. The index is
// rebuilt under the router lock the first time it is asked for after the table
// changed, then read without any lock: nothing reaches a snapshot once it is
// published, so a lookup never races registration or the documentation helpers,
// and it costs one map read however many routes the app has. The result is
// shared and must not be modified.
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

// indexNamedRoutes rebuilds the index unless another goroutine already has for
// the current revision. Every change to the route table bumps the revision
// while holding the lock, so a build under it pairs the table with the right
// revision.
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

// GetRoute Get route by name. The returned route is a deep copy, so it stays
// safe while other goroutines register or document, and changing it never
// changes the app.
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
