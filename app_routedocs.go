package fiber

import (
	"fmt"
	"mime"
	"reflect"
	"strings"

	"github.com/gofiber/fiber/v3/internal/deepcopy"
	"github.com/gofiber/fiber/v3/internal/schemautil"
	"github.com/gofiber/utils/v2"
)

func docSetSummary(sum string) func(route *Route) {
	return func(route *Route) { route.Summary = sum }
}

func docSetDescription(desc string) func(route *Route) {
	return func(route *Route) { route.Description = desc }
}

func validateMediaType(typ string) string {
	if _, _, err := mime.ParseMediaType(typ); err != nil || !strings.Contains(typ, "/") {
		panic("invalid media type: " + typ)
	}
	return typ
}

func optionalMediaType(typ string) string {
	typ = utils.TrimSpace(typ)
	if typ == "" {
		return ""
	}
	return validateMediaType(typ)
}

func docSetConsumes(typ string) func(route *Route) {
	typ = optionalMediaType(typ)
	return func(route *Route) { route.Consumes = typ }
}

func docSetProduces(typ string) func(route *Route) {
	typ = optionalMediaType(typ)
	return func(route *Route) { route.Produces = typ }
}

func hasSchema(schema any) bool {
	if schema == nil {
		return false
	}
	if value, ok := schemautil.AsMap(schema); ok {
		return len(value) > 0
	}
	return true
}

// copySchema detaches a schema from its caller; a Go model struct is kept as it is, since only its type is read.
func copySchema(schema any) any {
	if value, ok := schemautil.AsMap(schema); ok {
		if len(value) == 0 {
			return nil
		}
		return copyAnyMap(value)
	}
	switch reflect.ValueOf(schema).Kind() {
	case reflect.Map, reflect.Slice:
		return deepcopy.Value(schema)
	default:
		return schema
	}
}

func docRequestBodyWithExample(description string, required bool, schema any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) func(route *Route) {
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

	// A schema or example documented here replaces an earlier body; a description or media types alone keep it.
	definesBody := body.Schema != nil || example != nil || len(examples) > 0

	return func(route *Route) {
		previous := route.RequestBody
		route.RequestBody = cloneRouteRequestBody(body)
		if previous != nil {
			if description == "" {
				route.RequestBody.Description = previous.Description
			}
			if !definesBody {
				carryEarlierRequestBody(route.RequestBody, previous)
			}
		}
		if len(sanitized) > 0 && route.Consumes == "" {
			route.Consumes = sanitized[0]
		}
	}
}

//nolint:gocritic // hugeParam: by-value keeps the chainable route-helper API ergonomic.
func docAddParameter(param RouteParameter) func(route *Route) {
	if utils.TrimSpace(param.Name) == "" {
		panic("AddParameter: parameter name is required")
	}

	location := param.In.Normalize()
	if !validParamLocation(location, true) {
		panic("AddParameter: invalid parameter location: " + param.In)
	}
	param.In = location

	injectType := false
	switch {
	case len(param.Content) > 0:
		if len(param.Content) > 1 {
			panic("AddParameter: parameter content must contain exactly one media type: " + param.Name)
		}
		param.Content = sanitizeContentMediaTypes(param.Content)
		param.Schema = nil
		param.SchemaRef = ""
	case param.SchemaRef != "":
		param.Schema = map[string]any{openapiRefKey: param.SchemaRef}
	case location == ParamInQuerystring:
	default:
		injectType = true
	}

	if location == ParamInPath {
		param.Required = true
	}

	return func(route *Route) {
		paramCopy := cloneRouteParameter(&param)
		if injectType {
			schema, ok := paramCopy.Schema.(map[string]any)
			if paramCopy.Schema == nil {
				schema, ok = map[string]any{}, true
			}
			if ok {
				if typ, ok := schemautil.DefaultType(schema, openapiTypeString); ok {
					schema["type"] = typ
				}
				paramCopy.Schema = schema
			}
		}
		route.Parameters = append(route.Parameters, paramCopy)
	}
}

func responseKey(status int) string {
	if status == 0 {
		return defaultResponseKey
	}
	if status < 100 || status > 599 {
		panic("invalid status code: " + utils.FormatInt(int64(status)))
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

	explicitDescription := description != ""
	if !explicitDescription {
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

	// A schema or example documented here replaces an earlier body; a description or media types alone keep it.
	definesBody := resp.Schema != nil || example != nil || len(examples) > 0

	return func(route *Route) {
		if route.Responses == nil {
			route.Responses = make(map[string]RouteResponse)
		}
		copyResp := cloneRouteResponse(&resp)
		if existing, ok := route.Responses[key]; ok {
			copyResp.Headers = existing.Headers
			copyResp.Links = existing.Links
			if !explicitDescription && existing.Description != "" {
				copyResp.Description = existing.Description
			}
			if !definesBody {
				copyResp.Schema = existing.Schema
				copyResp.SchemaRef = existing.SchemaRef
				copyResp.Example = existing.Example
				copyResp.Examples = existing.Examples
				copyResp.Content = existing.Content
				if len(copyResp.MediaTypes) == 0 {
					copyResp.MediaTypes = existing.MediaTypes
				}
			}
		}
		route.Responses[key] = copyResp
		if status == StatusOK && len(copyResp.MediaTypes) > 0 && route.Produces == "" {
			route.Produces = copyResp.MediaTypes[0]
		}
	}
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
		route.Security = deepcopy.Security(requirements)
	}
}

func docSetHidden() func(route *Route) {
	return func(route *Route) { route.hidden = true }
}

func docResponseHeader(status int, name, description string, schema any) func(route *Route) {
	if utils.TrimSpace(name) == "" {
		panic("ResponseHeader: header name is required")
	}

	header := map[string]any{}
	if description != "" {
		header["description"] = description
	}
	if hasSchema(schema) {
		header["schema"] = schema
	} else {
		header["schema"] = map[string]any{"type": openapiTypeString}
	}

	return docSetResponseEntry(status, name, header, func(resp *RouteResponse) *map[string]any { return &resp.Headers })
}

func docOperationExternalDocs(description, url string) func(route *Route) {
	if utils.TrimSpace(url) == "" {
		panic("OperationExternalDocs: url is required")
	}
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
		return nil
	}
	return func(route *Route) {
		if route.OperationExtensions == nil {
			route.OperationExtensions = make(map[string]any, len(fields))
		}
		for key, value := range fields {
			route.OperationExtensions[key] = deepcopy.Value(value)
		}
	}
}

// sanitizeContentMediaTypes returns content keyed by trimmed, validated media types.
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
		previous := route.RequestBody
		route.RequestBody = cloneRouteRequestBody(body)
		if previous != nil && description == "" {
			route.RequestBody.Description = previous.Description
		}
	}
}

// carryEarlierRequestBody gives body the content of the body before it, for a call that documented none.
func carryEarlierRequestBody(body, previous *RouteRequestBody) {
	earlier := cloneRouteRequestBody(previous)
	body.Schema, body.SchemaRef = earlier.Schema, earlier.SchemaRef
	body.Example, body.Examples = earlier.Example, earlier.Examples
	body.Content = earlier.Content
	if len(body.MediaTypes) == 0 {
		body.MediaTypes = earlier.MediaTypes
	}
}

func docResponseContent(status int, description string, content map[string]RouteMediaType) func(route *Route) {
	content = sanitizeContentMediaTypes(content)

	key := responseKey(status)
	return func(route *Route) {
		resp := getOrCreateResponse(route, key, status)
		if description != "" {
			resp.Description = description
		}
		resp.Content = cloneRouteMediaTypeMap(content)
		route.Responses[key] = resp
	}
}

func docResponseLink(status int, name string, link map[string]any) func(route *Route) {
	if utils.TrimSpace(name) == "" {
		panic("ResponseLink: link name is required")
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

func newRouteParameter(name string, in ParamLocation, required bool, schema any, schemaRef, description string, example any, examples map[string]any) RouteParameter { //nolint:revive // flag-parameter: required is a Parameter Object field, not control flow
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

// validParamLocation reports whether the normalized location is one a parameter may have; models cannot use "querystring".
func validParamLocation(location ParamLocation, querystring bool) bool {
	switch location {
	case ParamInPath, ParamInQuery, ParamInHeader, ParamInCookie:
		return true
	case ParamInQuerystring:
		return querystring
	default:
		return false
	}
}

func docAddParameterModel(in ParamLocation, model any) func(route *Route) {
	location := in.Normalize()
	if !validParamLocation(location, false) {
		panic("Params: invalid parameter location: " + in)
	}
	t := reflect.TypeOf(model)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("Params: parameter model must be a struct, got %T", model))
	}
	return func(route *Route) {
		route.ParameterModels = append(route.ParameterModels, RouteParameterModel{In: location, Model: model})
	}
}
