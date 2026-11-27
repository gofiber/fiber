package openapi

import (
	"slices"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/deepcopy"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// Parameter locations accepted by the documentation helpers.
const (
	ParamInPath        ParamLocation = "path"
	ParamInQuery       ParamLocation = "query"
	ParamInHeader      ParamLocation = "header"
	ParamInCookie      ParamLocation = "cookie"
	ParamInQuerystring ParamLocation = "querystring"
)

const defaultResponseKey = "default"

// ParamLocation is where a documented parameter is read from. Use the ParamIn constants.
type ParamLocation string

// Normalize trims and lower-cases a location, so "Query " reads as "query".
func (l ParamLocation) Normalize() ParamLocation {
	return ParamLocation(utilsstrings.ToLower(utils.TrimSpace(string(l))))
}

// Operation is what a route is documented with: the helpers of [Doc] build it and the middleware reads it
// when it generates the specification. [OperationOf] returns the one a route carries.
type Operation struct {
	// Responses, RequestBody, Parameters and ParameterModels can hold the Go models a route is documented
	// from, which need not be marshalable.
	Responses   map[string]RouteResponse
	RequestBody *RouteRequestBody
	// ExternalDocs is the operation's externalDocs object.
	ExternalDocs map[string]any
	// OperationExtensions are extra operation-object fields, such as "x-" extensions.
	OperationExtensions map[string]any
	// Callbacks are the operation's callbacks by name: each a Callback Object, which maps an expression to a Path Item.
	Callbacks map[string]any
	Summary   string
	// OperationID is the operationId to document the route with; empty derives it from the route's name or path.
	OperationID string
	Description string
	Consumes    string
	Produces    string
	Parameters  []RouteParameter
	// ParameterModels are structs expanded into parameters when the spec is built.
	ParameterModels []RouteParameterModel
	Tags            []string
	// Security holds the OpenAPI security requirements; a non-nil empty slice means none are needed.
	Security []map[string][]string
	// Servers are the operation's own Server Objects, which replace the ones a Domain router implies.
	Servers    []map[string]any
	Deprecated bool
	// Hidden excludes the route from the generated specification.
	Hidden bool
}

// operationAnnotation is the key the Operation of a route is stored under with fiber's Annotate.
type operationAnnotation struct{}

// OperationOf returns the Operation route was documented with, or nil. It is shared with every copy of the
// route and must not be modified.
func OperationOf(route *fiber.Route) *Operation {
	if route == nil {
		return nil
	}
	op, ok := route.Annotation(operationAnnotation{}).(*Operation)
	if !ok {
		return nil
	}
	return op
}

// cloneOperation returns a deep copy of current, which a documentation helper may change before it is stored;
// a stored Operation is never changed, since every copy of the route shares it.
func cloneOperation(current any) *Operation {
	src, ok := current.(*Operation)
	if !ok || src == nil {
		return &Operation{}
	}
	op := *src
	op.RequestBody = cloneRouteRequestBody(src.RequestBody)
	op.Parameters = cloneRouteParameters(src.Parameters)
	op.ParameterModels = slices.Clone(src.ParameterModels)
	op.Responses = cloneRouteResponses(src.Responses)
	op.Tags = slices.Clone(src.Tags)
	op.Security = deepcopy.Security(src.Security)
	op.ExternalDocs = copyAnyMap(src.ExternalDocs)
	op.OperationExtensions = copyAnyMap(src.OperationExtensions)
	op.Callbacks = copyAnyMap(src.Callbacks)
	op.Servers = cloneMaps(src.Servers)
	return &op
}

// RouteParameter describes an input captured by a route. Content wins over Schema/SchemaRef.
type RouteParameter struct {
	Schema          any                       `json:"schema"`
	Content         map[string]RouteMediaType `json:"content,omitempty"`
	SchemaRef       string                    `json:"schemaRef,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Example         any                       `json:"example,omitempty"`
	Examples        map[string]any            `json:"examples,omitempty"`
	Explode         *bool                     `json:"explode,omitempty"`
	Description     string                    `json:"description"`
	Name            string                    `json:"name"`
	In              ParamLocation             `json:"in"`
	Style           string                    `json:"style,omitempty"`
	Required        bool                      `json:"required"`
	Deprecated      bool                      `json:"deprecated,omitempty"`
	AllowEmptyValue bool                      `json:"allowEmptyValue,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	AllowReserved   bool                      `json:"allowReserved,omitempty"`   //nolint:tagliatelle // OpenAPI spec uses camelCase
}

// RouteParameterModel declares parameters through a Go struct, expanded by the OpenAPI middleware.
type RouteParameterModel struct {
	Model any           `json:"model"`
	In    ParamLocation `json:"in"`
}

// RouteMediaType describes one media type entry of a body or response.
type RouteMediaType struct {
	// ItemSchema describes each item of a sequential media type such as text/event-stream or
	// application/jsonl, for a stream the client reads item by item. OpenAPI 3.2 writes it as itemSchema;
	// earlier versions get the equivalent array schema when Schema is empty.
	ItemSchema any            `json:"itemSchema,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Schema     any            `json:"schema,omitempty"`
	Example    any            `json:"example,omitempty"`
	Examples   map[string]any `json:"examples,omitempty"`
	Encoding   map[string]any `json:"encoding,omitempty"`
	SchemaRef  string         `json:"schemaRef,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
}

// RouteResponse describes a response emitted by a route.
type RouteResponse struct {
	Example     any                       `json:"example,omitempty"`
	Schema      any                       `json:"schema,omitempty"`
	Examples    map[string]any            `json:"examples,omitempty"`
	Headers     map[string]any            `json:"headers,omitempty"`
	Links       map[string]any            `json:"links,omitempty"`
	Content     map[string]RouteMediaType `json:"content,omitempty"`
	SchemaRef   string                    `json:"schemaRef,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Description string                    `json:"description"`
	MediaTypes  []string                  `json:"mediaTypes"` //nolint:tagliatelle // OpenAPI spec uses camelCase
}

// RouteRequestBody describes the request payload accepted by a route.
type RouteRequestBody struct {
	Example     any                       `json:"example,omitempty"`
	Schema      any                       `json:"schema,omitempty"`
	Examples    map[string]any            `json:"examples,omitempty"`
	Content     map[string]RouteMediaType `json:"content,omitempty"`
	SchemaRef   string                    `json:"schemaRef,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Description string                    `json:"description"`
	MediaTypes  []string                  `json:"mediaTypes"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Required    bool                      `json:"required"`
}

func cloneMaps(list []map[string]any) []map[string]any {
	if len(list) == 0 {
		return nil
	}
	cloned := make([]map[string]any, len(list))
	for i := range list {
		cloned[i] = deepcopy.Map(list[i])
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
			Schema:     copySchema(mt.Schema),
			ItemSchema: copySchema(mt.ItemSchema),
			SchemaRef:  mt.SchemaRef,
			Example:    deepcopy.Value(mt.Example),
			Examples:   copyAnyMap(mt.Examples),
			Encoding:   copyAnyMap(mt.Encoding),
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
