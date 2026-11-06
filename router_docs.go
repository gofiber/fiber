package fiber

import (
	"slices"

	"github.com/gofiber/fiber/v3/internal/deepcopy"
)

// copyRoute clones a route without its group, so Name() does not prefix it with the source app's group name.
// The omission is load-bearing for a mount placeholder, which domainRouter.cloneRoutesForDomain expands instead.
func (app *App) copyRoute(route *Route) *Route {
	copied := new(Route)
	app.copyRouteInto(copied, route)
	return copied
}

func (r *Route) isDocumented() bool {
	return r.RequestBody != nil || r.Parameters != nil || r.ParameterModels != nil ||
		r.Responses != nil || r.Tags != nil || r.Security != nil ||
		r.ExternalDocs != nil || r.OperationExtensions != nil
}

// copyRouteInto copies route into dst, cloning its documentation metadata; Handlers and Params are shared.
func (app *App) copyRouteInto(dst, route *Route) {
	*dst = *route
	dst.group = nil

	if !route.isDocumented() {
		return
	}

	app.cloneRouteDocInto(dst, route)
}

func (*App) cloneRouteDocInto(dst, route *Route) {
	dst.RequestBody = cloneRouteRequestBody(route.RequestBody)
	dst.Parameters = cloneRouteParameters(route.Parameters)
	dst.ParameterModels = slices.Clone(route.ParameterModels)
	dst.Responses = cloneRouteResponses(route.Responses)
	dst.Tags = append([]string(nil), route.Tags...)
	dst.Security = deepcopy.Security(route.Security)
	dst.ExternalDocs = copyAnyMap(route.ExternalDocs)
	dst.OperationExtensions = copyAnyMap(route.OperationExtensions)
}

func (app *App) copyRouteBase(route *Route) *Route {
	copied := new(Route)
	app.copyRouteBaseInto(copied, route)
	return copied
}

// copyRouteBaseInto is copyRouteBase filling the caller's slot.
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
	dst.hidden = false
	dst.docHandlers = nil
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
	clone.Examples = copyAnyMap(body.Examples)
	clone.Example = deepcopy.Value(body.Example)
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
			Example:   deepcopy.Value(mt.Example),
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
		cloned[i] = cloneRouteParameter(&params[i])
	}
	return cloned
}

func cloneRouteParameter(p *RouteParameter) RouteParameter {
	cloned := *p
	cloned.Schema = copySchema(p.Schema)
	cloned.Examples = copyAnyMap(p.Examples)
	cloned.Example = deepcopy.Value(p.Example)
	cloned.Content = cloneRouteMediaTypeMap(p.Content)
	if p.Explode != nil {
		explode := *p.Explode
		cloned.Explode = &explode
	}
	return cloned
}

func cloneRouteResponses(responses map[string]RouteResponse) map[string]RouteResponse {
	if len(responses) == 0 {
		return nil
	}
	cloned := make(map[string]RouteResponse, len(responses))
	for code := range responses {
		resp := responses[code]
		cloned[code] = cloneRouteResponse(&resp)
	}
	return cloned
}

func cloneRouteResponse(resp *RouteResponse) RouteResponse {
	cloned := *resp
	cloned.Schema = copySchema(resp.Schema)
	cloned.Examples = copyAnyMap(resp.Examples)
	cloned.Example = deepcopy.Value(resp.Example)
	cloned.Headers = copyAnyMap(resp.Headers)
	cloned.Links = copyAnyMap(resp.Links)
	cloned.Content = cloneRouteMediaTypeMap(resp.Content)
	cloned.MediaTypes = nil
	if len(resp.MediaTypes) > 0 {
		cloned.MediaTypes = append([]string(nil), resp.MediaTypes...)
	}
	return cloned
}

func copyAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	return deepcopy.Map(src)
}
