package fiber

import (
	"sync/atomic"
)

// docTarget is what Group, domainRouter and Registering share to document the route they registered last.
// R is the router type their helpers return, so a chain keeps its own type.
type docTarget[R any] struct { //nolint:govet // fieldalignment: lastRegID must stay first for 64-bit atomics on 32-bit platforms; betteralign:ignore
	// lastRegID is the latest registration, accessed atomically. It must stay the first field, and
	// docTarget the first field of its owner, as 64-bit atomics need an 8-byte aligned word on 32-bit platforms.
	lastRegID uint64
	app       *App
	// self is the router the helpers return.
	self R
}

// document applies a change to the most recently registered route.
func (t *docTarget[R]) document(apply func(route *Route)) R {
	t.app.applyToRegistration(atomic.LoadUint64(&t.lastRegID), apply)
	return t.self
}

// Summary assigns a short summary to the most recently registered route.
func (t *docTarget[R]) Summary(sum string) R {
	return t.document(docSetSummary(sum))
}

// Description assigns a description to the most recently registered route.
func (t *docTarget[R]) Description(desc string) R {
	return t.document(docSetDescription(desc))
}

// Consumes assigns a request media type to the most recently registered route.
func (t *docTarget[R]) Consumes(typ string) R {
	return t.document(docSetConsumes(typ))
}

// Produces assigns a response media type to the most recently registered route.
func (t *docTarget[R]) Produces(typ string) R {
	return t.document(docSetProduces(typ))
}

// RequestBody documents the request payload for the most recently registered route.
func (t *docTarget[R]) RequestBody(description string, required bool, mediaTypes ...string) R {
	return t.RequestBodyWithExample(description, required, nil, "", nil, nil, mediaTypes...)
}

// RequestBodyWithExample documents the request payload for the most recently registered route with schema references and examples.
func (t *docTarget[R]) RequestBodyWithExample(description string, required bool, schema any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) R {
	return t.document(docRequestBodyWithExample(description, required, schema, schemaRef, example, examples, mediaTypes...))
}

// Parameter documents an input parameter for the most recently registered route.
func (t *docTarget[R]) Parameter(name string, in ParamLocation, required bool, schema any, description string) R {
	return t.ParameterWithExample(name, in, required, schema, "", description, nil, nil)
}

// ParameterWithExample documents an input parameter for the most recently registered route with schema references and examples.
func (t *docTarget[R]) ParameterWithExample(name string, in ParamLocation, required bool, schema any, schemaRef, description string, example any, examples map[string]any) R {
	return t.AddParameter(newRouteParameter(name, in, required, schema, schemaRef, description, example, examples))
}

// Response documents an HTTP response for the most recently registered route.
func (t *docTarget[R]) Response(status int, description string, mediaTypes ...string) R {
	return t.ResponseWithExample(status, description, nil, "", nil, nil, mediaTypes...)
}

// ResponseWithExample documents an HTTP response for the most recently registered route with schema references and examples.
func (t *docTarget[R]) ResponseWithExample(status int, description string, schema any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) R {
	return t.document(docAddResponse(status, description, schema, schemaRef, example, examples, mediaTypes...))
}

// Tags assigns tags to the most recently registered route.
func (t *docTarget[R]) Tags(tags ...string) R {
	return t.document(docSetTags(tags...))
}

// Deprecated marks the most recently registered route as deprecated.
func (t *docTarget[R]) Deprecated() R {
	return t.document(docSetDeprecated())
}

// Security sets the OpenAPI security requirements for the most recently registered route.
func (t *docTarget[R]) Security(requirements ...map[string][]string) R {
	return t.document(docSetSecurity(requirements...))
}

// Hidden excludes the most recently registered route from the generated OpenAPI specification.
func (t *docTarget[R]) Hidden() R {
	return t.document(docSetHidden())
}

// ResponseHeader documents a response header for the most recently registered route.
func (t *docTarget[R]) ResponseHeader(status int, name, description string, schema any) R {
	return t.document(docResponseHeader(status, name, description, schema))
}

// Accepts documents the request body as the schema of model; see App.Accepts.
func (t *docTarget[R]) Accepts(model any, mediaTypes ...string) R {
	return t.document(docAccepts(model, mediaTypes...))
}

// Returns documents a response as the schema of model; see App.Returns.
func (t *docTarget[R]) Returns(status int, model any, mediaTypes ...string) R {
	return t.document(docReturns(status, model, mediaTypes...))
}

// Params documents the fields of model as parameters; see App.Params.
func (t *docTarget[R]) Params(in ParamLocation, model any) R {
	return t.document(docAddParameterModel(in, model))
}

// AddParameter documents an input parameter using the full RouteParameter.
//
//nolint:gocritic // hugeParam: by-value keeps the chainable route-helper API ergonomic.
func (t *docTarget[R]) AddParameter(param RouteParameter) R {
	return t.document(docAddParameter(param))
}

// OperationExternalDocs sets the externalDocs of the most recently registered route.
func (t *docTarget[R]) OperationExternalDocs(description, url string) R {
	return t.document(docOperationExternalDocs(description, url))
}

// RequestBodyContent documents a per-media-type request body.
func (t *docTarget[R]) RequestBodyContent(description string, required bool, content map[string]RouteMediaType) R {
	return t.document(docRequestBodyContent(description, required, content))
}

// ResponseContent documents a per-media-type response.
func (t *docTarget[R]) ResponseContent(status int, description string, content map[string]RouteMediaType) R {
	return t.document(docResponseContent(status, description, content))
}

// ResponseLink documents a response link.
func (t *docTarget[R]) ResponseLink(status int, name string, link map[string]any) R {
	return t.document(docResponseLink(status, name, link))
}

// OperationExtension merges arbitrary operation-object fields.
func (t *docTarget[R]) OperationExtension(fields map[string]any) R {
	return t.document(docOperationExtension(fields))
}

// newGroup returns a Group of app under prefix and, when it is nested, parent.
func newGroup(app *App, prefix string, parent *Group) *Group {
	grp := &Group{Prefix: prefix, parentGroup: parent}
	grp.docTarget = docTarget[Router]{app: app, self: grp}
	return grp
}

// newDomainRouter returns a domain router for matcher, created from group when it is non-nil, whose
// helpers target registration lastRegID.
//
//nolint:gocritic // hugeParam: the matcher is stored by value.
func newDomainRouter(app *App, group *Group, matcher domainMatcher, lastRegID uint64) *domainRouter {
	d := &domainRouter{group: group, matcher: matcher}
	d.docTarget = docTarget[Router]{app: app, self: d, lastRegID: lastRegID}
	return d
}

// newRegistering returns a Register for path of app, scoped to group and domain when they are non-nil.
func newRegistering(app *App, group *Group, domain *domainRouter, path string) *Registering {
	r := &Registering{group: group, domain: domain, path: path}
	r.docTarget = docTarget[Register]{app: app, self: r}
	return r
}
