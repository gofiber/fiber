// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

// Register defines all router handle interface generate by RouteChain().
type Register interface {
	All(handler any, handlers ...any) Register
	Get(handler any, handlers ...any) Register
	Head(handler any, handlers ...any) Register
	Post(handler any, handlers ...any) Register
	Put(handler any, handlers ...any) Register
	Delete(handler any, handlers ...any) Register
	Connect(handler any, handlers ...any) Register
	Options(handler any, handlers ...any) Register
	Trace(handler any, handlers ...any) Register
	Patch(handler any, handlers ...any) Register
	Query(handler any, handlers ...any) Register

	Add(methods []string, handler any, handlers ...any) Register

	RouteChain(path string) Register

	// Documentation helpers mirror the Router interface so metadata can be set
	// fluently on the most recently registered route.

	Name(name string) Register
	Summary(sum string) Register
	Description(desc string) Register
	Consumes(typ string) Register
	Produces(typ string) Register
	RequestBody(description string, required bool, mediaTypes ...string) Register
	RequestBodyWithExample(description string, required bool, schema map[string]any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) Register
	Parameter(name, in string, required bool, schema map[string]any, description string) Register
	ParameterWithExample(name, in string, required bool, schema map[string]any, schemaRef, description string, example any, examples map[string]any) Register
	AddParameter(param RouteParameter) Register
	Response(status int, description string, mediaTypes ...string) Register
	ResponseWithExample(status int, description string, schema map[string]any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) Register
	ResponseHeader(status int, name, description string, schema map[string]any) Register
	ResponseContent(status int, description string, content map[string]RouteMediaType) Register
	ResponseLink(status int, name string, link map[string]any) Register
	RequestBodyContent(description string, required bool, content map[string]RouteMediaType) Register
	Tags(tags ...string) Register
	Deprecated() Register
	Security(requirements ...map[string][]string) Register
	Hidden() Register
	OperationExternalDocs(description, url string) Register
	OperationExtension(fields map[string]any) Register
}

var _ Register = (*Registering)(nil)

// Registering provides route registration helpers for a specific path on the
// application instance.
type Registering struct {
	app   *App
	group *Group

	path string
}

// All registers a middleware route that will match requests
// with the provided path which is stored in register struct.
//
//	app.RouteChain("/").All(func(c fiber.Ctx) error {
//	     return c.Next()
//	})
//	app.RouteChain("/api").All(func(c fiber.Ctx) error {
//	     return c.Next()
//	})
//	app.RouteChain("/api").All(handler, func(c fiber.Ctx) error {
//	     return c.Next()
//	})
//
// This method will match all HTTP verbs: GET, POST, PUT, HEAD etc...
func (r *Registering) All(handler any, handlers ...any) Register {
	converted := collectHandlers("register", append([]any{handler}, handlers...)...)
	r.app.register([]string{methodUse}, r.path, r.group, converted...)
	return r
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (r *Registering) Get(handler any, handlers ...any) Register {
	return r.Add([]string{MethodGet}, handler, handlers...)
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (r *Registering) Head(handler any, handlers ...any) Register {
	return r.Add([]string{MethodHead}, handler, handlers...)
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (r *Registering) Post(handler any, handlers ...any) Register {
	return r.Add([]string{MethodPost}, handler, handlers...)
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (r *Registering) Put(handler any, handlers ...any) Register {
	return r.Add([]string{MethodPut}, handler, handlers...)
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (r *Registering) Delete(handler any, handlers ...any) Register {
	return r.Add([]string{MethodDelete}, handler, handlers...)
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (r *Registering) Connect(handler any, handlers ...any) Register {
	return r.Add([]string{MethodConnect}, handler, handlers...)
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (r *Registering) Options(handler any, handlers ...any) Register {
	return r.Add([]string{MethodOptions}, handler, handlers...)
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the r.Path to the target resource.
func (r *Registering) Trace(handler any, handlers ...any) Register {
	return r.Add([]string{MethodTrace}, handler, handlers...)
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (r *Registering) Patch(handler any, handlers ...any) Register {
	return r.Add([]string{MethodPatch}, handler, handlers...)
}

// Query registers a route for QUERY methods that performs a safe, idempotent
// query with a request body.
func (r *Registering) Query(handler any, handlers ...any) Register {
	return r.Add([]string{MethodQuery}, handler, handlers...)
}

// Add allows you to specify multiple HTTP methods to register a route.
// The provided handlers are executed in order, starting with `handler` and then the variadic `handlers`.
func (r *Registering) Add(methods []string, handler any, handlers ...any) Register {
	converted := collectHandlers("register", append([]any{handler}, handlers...)...)
	r.app.register(methods, r.path, r.group, converted...)
	return r
}

// RouteChain returns a new Register instance whose route path takes
// the path in the current instance as its prefix.
func (r *Registering) RouteChain(path string) Register {
	// Create new group
	route := &Registering{app: r.app, group: r.group, path: getGroupPath(r.path, path)}

	return route
}

// Name assigns a name to the most recently registered route.
func (r *Registering) Name(name string) Register {
	r.app.Name(name)
	return r
}

// Summary assigns a short summary to the most recently registered route.
func (r *Registering) Summary(sum string) Register {
	r.app.Summary(sum)
	return r
}

// Description assigns a description to the most recently registered route.
func (r *Registering) Description(desc string) Register {
	r.app.Description(desc)
	return r
}

// Consumes assigns a request media type to the most recently registered route.
func (r *Registering) Consumes(typ string) Register {
	r.app.Consumes(typ)
	return r
}

// Produces assigns a response media type to the most recently registered route.
func (r *Registering) Produces(typ string) Register {
	r.app.Produces(typ)
	return r
}

// RequestBody documents the request payload for the most recently registered route.
func (r *Registering) RequestBody(description string, required bool, mediaTypes ...string) Register {
	r.app.RequestBody(description, required, mediaTypes...)
	return r
}

// RequestBodyWithExample documents the request payload with schema references and examples.
func (r *Registering) RequestBodyWithExample(description string, required bool, schema map[string]any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) Register {
	r.app.RequestBodyWithExample(description, required, schema, schemaRef, example, examples, mediaTypes...)
	return r
}

// Parameter documents an input parameter for the most recently registered route.
func (r *Registering) Parameter(name, in string, required bool, schema map[string]any, description string) Register {
	r.app.Parameter(name, in, required, schema, description)
	return r
}

// ParameterWithExample documents an input parameter, including schema references and examples.
func (r *Registering) ParameterWithExample(name, in string, required bool, schema map[string]any, schemaRef, description string, example any, examples map[string]any) Register {
	r.app.ParameterWithExample(name, in, required, schema, schemaRef, description, example, examples)
	return r
}

// AddParameter documents an input parameter using the full RouteParameter.
//
//nolint:gocritic // hugeParam: by-value keeps the chainable route-helper API ergonomic.
func (r *Registering) AddParameter(param RouteParameter) Register {
	r.app.AddParameter(param)
	return r
}

// Response documents an HTTP response for the most recently registered route.
func (r *Registering) Response(status int, description string, mediaTypes ...string) Register {
	r.app.Response(status, description, mediaTypes...)
	return r
}

// ResponseWithExample documents an HTTP response with schema references and examples.
func (r *Registering) ResponseWithExample(status int, description string, schema map[string]any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) Register {
	r.app.ResponseWithExample(status, description, schema, schemaRef, example, examples, mediaTypes...)
	return r
}

// ResponseHeader documents a response header for the most recently registered route.
func (r *Registering) ResponseHeader(status int, name, description string, schema map[string]any) Register {
	r.app.ResponseHeader(status, name, description, schema)
	return r
}

// ResponseContent documents a per-media-type response for the most recently registered route.
func (r *Registering) ResponseContent(status int, description string, content map[string]RouteMediaType) Register {
	r.app.ResponseContent(status, description, content)
	return r
}

// ResponseLink documents a response link for the most recently registered route.
func (r *Registering) ResponseLink(status int, name string, link map[string]any) Register {
	r.app.ResponseLink(status, name, link)
	return r
}

// RequestBodyContent documents a per-media-type request body for the most recently registered route.
func (r *Registering) RequestBodyContent(description string, required bool, content map[string]RouteMediaType) Register {
	r.app.RequestBodyContent(description, required, content)
	return r
}

// Tags assigns tags to the most recently registered route.
func (r *Registering) Tags(tags ...string) Register {
	r.app.Tags(tags...)
	return r
}

// Deprecated marks the most recently registered route as deprecated.
func (r *Registering) Deprecated() Register {
	r.app.Deprecated()
	return r
}

// Security sets the OpenAPI security requirements for the most recently registered route.
func (r *Registering) Security(requirements ...map[string][]string) Register {
	r.app.Security(requirements...)
	return r
}

// Hidden excludes the most recently registered route from the generated OpenAPI specification.
func (r *Registering) Hidden() Register {
	r.app.Hidden()
	return r
}

// OperationExternalDocs sets the externalDocs of the most recently registered operation.
func (r *Registering) OperationExternalDocs(description, url string) Register {
	r.app.OperationExternalDocs(description, url)
	return r
}

// OperationExtension merges arbitrary operation-object fields into the most recently registered operation.
func (r *Registering) OperationExtension(fields map[string]any) Register {
	r.app.OperationExtension(fields)
	return r
}
