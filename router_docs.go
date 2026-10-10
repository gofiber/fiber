package fiber

import (
	"reflect"
	"slices"
)

// copyRoute clones a route without its group, so the clone does not inherit the
// source app's group name. A mount placeholder keeps its target app in group.app;
// cloning it would serve the mounted handlers on every host under a domain mount.
func (app *App) copyRoute(route *Route) *Route {
	copied := app.copyRouteValue(route)
	return &copied
}

// copyRouteValue is copyRoute without the heap allocation.
func (app *App) copyRouteValue(route *Route) (copied Route) { //nolint:nonamedreturns // the named result is what keeps this to a single struct copy
	app.copyRouteInto(&copied, route)
	return copied
}

// isDocumented reports whether the route carries metadata a copy must clone.
func (r *Route) isDocumented() bool {
	return r.RequestBody != nil || r.Parameters != nil || r.ParameterModels != nil ||
		r.Responses != nil || r.Tags != nil || r.Security != nil ||
		r.ExternalDocs != nil || r.OperationExtensions != nil
}

// copyRouteInto deep-copies route into dst, writing through a pointer to avoid moving the large Route.
func (app *App) copyRouteInto(dst, route *Route) {
	*dst = *route
	dst.group = nil

	if !route.isDocumented() {
		return
	}

	app.cloneRouteDocInto(dst, route)
}

// cloneRouteDocInto deep-clones the documentation containers; out of line to keep the fast path small.
func (*App) cloneRouteDocInto(dst, route *Route) {
	dst.RequestBody = cloneRouteRequestBody(route.RequestBody)
	dst.Parameters = cloneRouteParameters(route.Parameters)
	dst.ParameterModels = slices.Clone(route.ParameterModels)
	dst.Responses = cloneRouteResponses(route.Responses)
	dst.Tags = append([]string(nil), route.Tags...)
	dst.Security = cloneRouteSecurity(route.Security)
	dst.ExternalDocs = copyAnyMap(route.ExternalDocs)
	dst.OperationExtensions = copyAnyMap(route.OperationExtensions)
}

// copyRouteBase copies routing data without the documentation clone (auto-HEAD twins never read it).
func (app *App) copyRouteBase(route *Route) *Route {
	copied := new(Route)
	app.copyRouteBaseInto(copied, route)
	return copied
}

// copyRouteBaseInto is copyRouteBase filling the caller's slot; a wholesale copy
// then clear beats many field writes on a struct this large.
func (*App) copyRouteBaseInto(dst, route *Route) {
	*dst = *route

	dst.group = nil
	dst.Summary = ""
	dst.Description = ""
	dst.Consumes = ""
	dst.Produces = ""
	dst.Deprecated = false
	dst.RequestBody = nil
	dst.Parameters = nil
	dst.ParameterModels = nil
	dst.Responses = nil
	dst.Tags = nil
	dst.Security = nil
	dst.ExternalDocs = nil
	dst.OperationExtensions = nil
}

func cloneRouteSecurity(requirements []map[string][]string) []map[string][]string {
	// An empty non-nil list is kept: it means "no authentication", unlike nil.
	if requirements == nil {
		return nil
	}
	cloned := make([]map[string][]string, len(requirements))
	for i, requirement := range requirements {
		entry := make(map[string][]string, len(requirement))
		for scheme, scopes := range requirement {
			// Keep an empty scope list non-nil so it marshals as [] rather than null.
			cloned := make([]string, len(scopes))
			copy(cloned, scopes)
			entry[scheme] = cloned
		}
		cloned[i] = entry
	}
	return cloned
}

func cloneRouteRequestBody(body *RouteRequestBody) *RouteRequestBody {
	if body == nil {
		return nil
	}
	clone := &RouteRequestBody{
		Description: body.Description,
		Required:    body.Required,
	}
	clone.Schema = copySchema(body.Schema)
	clone.SchemaRef = body.SchemaRef
	if len(body.Examples) > 0 {
		clone.Examples = copyAnyMap(body.Examples)
	}
	clone.Example = copyAnyValue(body.Example)
	if len(body.MediaTypes) > 0 {
		clone.MediaTypes = append([]string(nil), body.MediaTypes...)
	}
	clone.Content = cloneRouteMediaTypeMap(body.Content)
	return clone
}

func cloneRouteMediaTypeMap(content map[string]RouteMediaType) map[string]RouteMediaType {
	if len(content) == 0 {
		return nil
	}
	cloned := make(map[string]RouteMediaType, len(content))
	for mediaType, mt := range content {
		cloned[mediaType] = RouteMediaType{
			Schema:    copySchema(mt.Schema),
			SchemaRef: mt.SchemaRef,
			Example:   copyAnyValue(mt.Example),
			Examples:  copyAnyMap(mt.Examples),
			Encoding:  copyAnyMap(mt.Encoding),
		}
	}
	return cloned
}

func cloneRouteParameters(params []RouteParameter) []RouteParameter {
	if len(params) == 0 {
		return nil
	}
	cloned := make([]RouteParameter, len(params))
	for i := range params {
		p := &params[i]
		cloned[i] = RouteParameter{
			Name:            p.Name,
			In:              p.In,
			Required:        p.Required,
			Description:     p.Description,
			Deprecated:      p.Deprecated,
			Style:           p.Style,
			AllowEmptyValue: p.AllowEmptyValue,
			AllowReserved:   p.AllowReserved,
			Schema:          copySchema(p.Schema),
			SchemaRef:       p.SchemaRef,
			Examples:        copyAnyMap(p.Examples),
			Example:         copyAnyValue(p.Example),
			Content:         cloneRouteMediaTypeMap(p.Content),
		}
		if p.Explode != nil {
			explode := *p.Explode
			cloned[i].Explode = &explode
		}
	}
	return cloned
}

func cloneRouteResponses(responses map[string]RouteResponse) map[string]RouteResponse {
	if len(responses) == 0 {
		return nil
	}
	cloned := make(map[string]RouteResponse, len(responses))
	for code, resp := range responses {
		copyResp := RouteResponse{
			Description: resp.Description,
			Schema:      copySchema(resp.Schema),
			SchemaRef:   resp.SchemaRef,
			Examples:    copyAnyMap(resp.Examples),
			Example:     copyAnyValue(resp.Example),
			Headers:     copyAnyMap(resp.Headers),
			Links:       copyAnyMap(resp.Links),
			Content:     cloneRouteMediaTypeMap(resp.Content),
		}
		if len(resp.MediaTypes) > 0 {
			copyResp.MediaTypes = append([]string(nil), resp.MediaTypes...)
		}
		cloned[code] = copyResp
	}
	return cloned
}

func copyAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	return copyAnyMapDepth(src, 0)
}

func copyAnyMapDepth(src map[string]any, depth int) map[string]any {
	// An empty nested map is kept so "properties": {} does not become null.
	if src == nil {
		return nil
	}
	if depth >= maxCopyDepth {
		// Cyclic or too deep: share the reference; encoding/json reports the cycle.
		return src
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = copyAnyValueDepth(value, depth+1)
	}
	return dst
}

func copyAnyValue(src any) any {
	return copyAnyValueDepth(src, 0)
}

func copyAnyValueDepth(src any, depth int) any {
	if src == nil {
		return nil
	}
	if depth >= maxCopyDepth {
		return src
	}

	switch value := src.(type) {
	case map[string]any:
		return copyAnyMapDepth(value, depth)
	case []any:
		copied := make([]any, len(value))
		for i := range value {
			copied[i] = copyAnyValueDepth(value[i], depth+1)
		}
		return copied
	case []map[string]any:
		copied := make([]map[string]any, len(value))
		for i := range value {
			copied[i] = copyAnyMapDepth(value[i], depth+1)
		}
		return copied
	default:
		return copyCompositeValue(src, depth)
	}
}

// copyCompositeValue clones map and slice values of named types; depth carries
// over so cycles still hit maxCopyDepth.
func copyCompositeValue(src any, depth int) any {
	value := reflect.ValueOf(src)

	switch value.Kind() {
	case reflect.Slice:
		if value.IsNil() {
			return src
		}
		copied := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := range value.Len() {
			// A nil element is an invalid reflect.Value; keep the zero value.
			if elem := copyAnyValueDepth(value.Index(i).Interface(), depth+1); elem != nil {
				copied.Index(i).Set(reflect.ValueOf(elem))
			}
		}
		return copied.Interface()
	case reflect.Map:
		if value.IsNil() {
			return src
		}
		copied := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			// SetMapIndex with an invalid value deletes the key; use the zero value instead.
			val := reflect.Zero(value.Type().Elem())
			if elem := copyAnyValueDepth(iter.Value().Interface(), depth+1); elem != nil {
				val = reflect.ValueOf(elem)
			}
			copied.SetMapIndex(iter.Key(), val)
		}
		return copied.Interface()
	default:
		return src
	}
}
