package openapi

import (
	"fmt"
)

// annotator is what fiber's App, Group, Domain router and RouteChain registration have in common: they store a
// value on the route they registered last.
type annotator interface {
	Annotate(key any, update func(current any) any)
}

// RouteDoc documents the route a registration created; [Doc] returns it.
type RouteDoc struct {
	target annotator
}

// Doc documents the latest route registered through route. Pass it what a registration returned, such as
// app.Get(...), group.Post(...), app.Domain(...).Put(...) or app.RouteChain(...).Get(...).
//
//	openapi.Doc(app.Get("/users/:id", getUser)).
//		Summary("Get a user").
//		Returns(fiber.StatusOK, User{})
//
// Doc panics when route is not the result of a fiber registration.
func Doc(route any) *RouteDoc {
	target, ok := route.(annotator)
	if !ok {
		panic(fmt.Sprintf("openapi: Doc needs the router a route registration returned, got %T", route))
	}
	return &RouteDoc{target: target}
}

// apply stores change applied to a copy of the route's Operation; the stored one is never changed in place.
func (d *RouteDoc) apply(change func(op *Operation)) *RouteDoc {
	if change == nil {
		return d
	}
	d.target.Annotate(operationAnnotation{}, func(current any) any {
		op := cloneOperation(current)
		change(op)
		return op
	})
	return d
}

// Summary assigns a short summary to the route.
func (d *RouteDoc) Summary(sum string) *RouteDoc {
	return d.apply(docSetSummary(sum))
}

// Description assigns a description to the route.
func (d *RouteDoc) Description(desc string) *RouteDoc {
	return d.apply(docSetDescription(desc))
}

// Consumes assigns a request media type to the route.
func (d *RouteDoc) Consumes(typ string) *RouteDoc {
	return d.apply(docSetConsumes(typ))
}

// Produces assigns a response media type to the route.
func (d *RouteDoc) Produces(typ string) *RouteDoc {
	return d.apply(docSetProduces(typ))
}

// RequestBody documents the request payload for the route.
func (d *RouteDoc) RequestBody(description string, required bool, mediaTypes ...string) *RouteDoc {
	return d.RequestBodyWithExample(description, required, nil, "", nil, nil, mediaTypes...)
}

// RequestBodyWithExample documents the request payload for the route with schema references and examples.
func (d *RouteDoc) RequestBodyWithExample(description string, required bool, schema any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) *RouteDoc {
	return d.apply(docRequestBodyWithExample(description, required, schema, schemaRef, example, examples, mediaTypes...))
}

// Parameter documents an input parameter for the route.
func (d *RouteDoc) Parameter(name string, in ParamLocation, required bool, schema any, description string) *RouteDoc {
	return d.ParameterWithExample(name, in, required, schema, "", description, nil, nil)
}

// ParameterWithExample documents an input parameter for the route with schema references and examples.
func (d *RouteDoc) ParameterWithExample(name string, in ParamLocation, required bool, schema any, schemaRef, description string, example any, examples map[string]any) *RouteDoc {
	return d.AddParameter(newRouteParameter(name, in, required, schema, schemaRef, description, example, examples))
}

// Response documents an HTTP response for the route.
func (d *RouteDoc) Response(status int, description string, mediaTypes ...string) *RouteDoc {
	return d.ResponseWithExample(status, description, nil, "", nil, nil, mediaTypes...)
}

// ResponseWithExample documents an HTTP response for the route with schema references and examples.
func (d *RouteDoc) ResponseWithExample(status int, description string, schema any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) *RouteDoc {
	return d.apply(docAddResponse(status, description, schema, schemaRef, example, examples, mediaTypes...))
}

// Tags assigns tags to the route.
func (d *RouteDoc) Tags(tags ...string) *RouteDoc {
	return d.apply(docSetTags(tags...))
}

// Deprecated marks the route as deprecated.
func (d *RouteDoc) Deprecated() *RouteDoc {
	return d.apply(docSetDeprecated())
}

// Security sets the OpenAPI security requirements for the route.
func (d *RouteDoc) Security(requirements ...map[string][]string) *RouteDoc {
	return d.apply(docSetSecurity(requirements...))
}

// Hidden excludes the route from the generated OpenAPI specification.
func (d *RouteDoc) Hidden() *RouteDoc {
	return d.apply(docSetHidden())
}

// ResponseHeader documents a response header for the route.
func (d *RouteDoc) ResponseHeader(status int, name, description string, schema any) *RouteDoc {
	return d.apply(docResponseHeader(status, name, description, schema))
}

// Accepts documents a required request body reflected from model, under the given media types.
func (d *RouteDoc) Accepts(model any, mediaTypes ...string) *RouteDoc {
	return d.apply(docAccepts(model, mediaTypes...))
}

// Returns documents a response as the schema of model.
func (d *RouteDoc) Returns(status int, model any, mediaTypes ...string) *RouteDoc {
	return d.apply(docReturns(status, model, mediaTypes...))
}

// Params documents the fields of model as parameters.
func (d *RouteDoc) Params(in ParamLocation, model any) *RouteDoc {
	return d.apply(docAddParameterModel(in, model))
}

// AddParameter documents an input parameter using the full RouteParameter.
//
//nolint:gocritic // hugeParam: by-value keeps the chainable route-helper API ergonomic.
func (d *RouteDoc) AddParameter(param RouteParameter) *RouteDoc {
	return d.apply(docAddParameter(param))
}

// OperationExternalDocs sets the externalDocs of the route.
func (d *RouteDoc) OperationExternalDocs(description, url string) *RouteDoc {
	return d.apply(docOperationExternalDocs(description, url))
}

// OperationID sets the operationId of the route; it overrides the one derived from the
// route's name or path. An id another operation already uses gets a numeric suffix in the document.
func (d *RouteDoc) OperationID(id string) *RouteDoc {
	return d.apply(docOperationID(id))
}

// Callback documents a callback of the route: name identifies it and callback is a
// Callback Object, mapping each runtime expression to the Path Item that describes the request sent to it.
func (d *RouteDoc) Callback(name string, callback map[string]any) *RouteDoc {
	return d.apply(docCallback(name, callback))
}

// OperationServer adds a Server Object (a map with at least a "url") to the route,
// replacing the servers the document or a Domain router would give it.
func (d *RouteDoc) OperationServer(server map[string]any) *RouteDoc {
	return d.apply(docOperationServer(server))
}

// RequestBodyContent documents a per-media-type request body.
func (d *RouteDoc) RequestBodyContent(description string, required bool, content map[string]RouteMediaType) *RouteDoc {
	return d.apply(docRequestBodyContent(description, required, content))
}

// ResponseContent documents a per-media-type response.
func (d *RouteDoc) ResponseContent(status int, description string, content map[string]RouteMediaType) *RouteDoc {
	return d.apply(docResponseContent(status, description, content))
}

// ResponseLink documents a response link.
func (d *RouteDoc) ResponseLink(status int, name string, link map[string]any) *RouteDoc {
	return d.apply(docResponseLink(status, name, link))
}

// OperationExtension merges arbitrary operation-object fields.
func (d *RouteDoc) OperationExtension(fields map[string]any) *RouteDoc {
	return d.apply(docOperationExtension(fields))
}
