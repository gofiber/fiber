// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"reflect"
	"sync/atomic"
)

// Group represents a collection of routes that share middleware and a common
// path prefix.
type Group struct {
	app         *App
	parentGroup *Group
	name        string

	Prefix string

	lastRegID uint64 // Most recent registration, targeted by the doc helpers. Accessed atomically.

	hasAnyRoute bool
}

// Name Assign name to specific route or group itself.
//
// If this method is used before any route added to group, it'll set group name and OnGroupNameHook will be used.
// Otherwise, it'll set route name and OnName hook will be used.
func (grp *Group) Name(name string) Router {
	if grp.hasAnyRoute {
		grp.app.applyNameToRegistration(atomic.LoadUint64(&grp.lastRegID), name)

		return grp
	}

	grp.app.mutex.Lock()
	if grp.parentGroup != nil {
		grp.name = grp.parentGroup.name + name
	} else {
		grp.name = name
	}
	snapshot := *grp
	grp.app.mutex.Unlock()

	// Fire hooks after releasing the lock so they may safely call locking app
	// methods (GetRoutes, documentation helpers, RemoveRoute, ...).
	if err := grp.app.hooks.executeOnGroupNameHooks(snapshot); err != nil {
		panic(err)
	}

	return grp
}

// Summary assigns a short summary to the most recently added route in the group.
func (grp *Group) Summary(sum string) Router {
	return grp.document(docSetSummary(sum))
}

// Description assigns a description to the most recently added route in the group.
func (grp *Group) Description(desc string) Router {
	return grp.document(docSetDescription(desc))
}

// Consumes assigns a request media type to the most recently added route in the group.
func (grp *Group) Consumes(typ string) Router {
	return grp.document(docSetConsumes(typ))
}

// Produces assigns a response media type to the most recently added route in the group.
func (grp *Group) Produces(typ string) Router {
	return grp.document(docSetProduces(typ))
}

// RequestBody documents the request payload for the most recently added route in the group.
func (grp *Group) RequestBody(description string, required bool, mediaTypes ...string) Router {
	return grp.RequestBodyWithExample(description, required, nil, "", nil, nil, mediaTypes...)
}

// RequestBodyWithExample documents the request payload for the most recently added route in the group with schema references and examples.
func (grp *Group) RequestBodyWithExample(description string, required bool, schema any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) Router {
	return grp.document(docRequestBodyWithExample(description, required, schema, schemaRef, example, examples, mediaTypes...))
}

// Parameter documents an input parameter for the most recently added route in the group.
func (grp *Group) Parameter(name, in string, required bool, schema any, description string) Router {
	return grp.ParameterWithExample(name, in, required, schema, "", description, nil, nil)
}

// ParameterWithExample documents an input parameter for the most recently added route in the group with schema references and examples.
func (grp *Group) ParameterWithExample(name, in string, required bool, schema any, schemaRef, description string, example any, examples map[string]any) Router {
	return grp.AddParameter(newRouteParameter(name, in, required, schema, schemaRef, description, example, examples))
}

// Response documents an HTTP response for the most recently added route in the group.
func (grp *Group) Response(status int, description string, mediaTypes ...string) Router {
	return grp.ResponseWithExample(status, description, nil, "", nil, nil, mediaTypes...)
}

// ResponseWithExample documents an HTTP response for the most recently added route in the group with schema references and examples.
func (grp *Group) ResponseWithExample(status int, description string, schema any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) Router {
	return grp.document(docAddResponse(status, description, schema, schemaRef, example, examples, mediaTypes...))
}

// Tags assigns tags to the most recently added route in the group.
func (grp *Group) Tags(tags ...string) Router {
	return grp.document(docSetTags(tags...))
}

// Deprecated marks the most recently added route in the group as deprecated.
func (grp *Group) Deprecated() Router {
	return grp.document(docSetDeprecated())
}

// Security sets the OpenAPI security requirements for the most recently added
// route in the group.
func (grp *Group) Security(requirements ...map[string][]string) Router {
	return grp.document(docSetSecurity(requirements...))
}

// Hidden excludes the most recently added route in the group from the generated
// OpenAPI specification.
func (grp *Group) Hidden() Router {
	return grp.document(docSetHidden())
}

// ResponseHeader documents a response header for the most recently added route
// in the group.
func (grp *Group) ResponseHeader(status int, name, description string, schema any) Router {
	return grp.document(docResponseHeader(status, name, description, schema))
}

// Accepts documents the request body as the schema of model; see App.Accepts.
func (grp *Group) Accepts(model any, mediaTypes ...string) Router {
	return grp.document(docAccepts(model, mediaTypes...))
}

// Returns documents a response as the schema of model; see App.Returns.
func (grp *Group) Returns(status int, model any, mediaTypes ...string) Router {
	return grp.document(docReturns(status, model, mediaTypes...))
}

// Params documents the fields of model as parameters; see App.Params.
func (grp *Group) Params(in string, model any) Router {
	return grp.document(docAddParameterModel(in, model))
}

// AddParameter documents an input parameter on the most recently added route in
// the group using the full RouteParameter.
//
//nolint:gocritic // hugeParam: by-value keeps the chainable route-helper API ergonomic.
func (grp *Group) AddParameter(param RouteParameter) Router {
	return grp.document(docAddParameter(param))
}

// OperationExternalDocs sets the externalDocs of the most recently added route in
// the group.
func (grp *Group) OperationExternalDocs(description, url string) Router {
	return grp.document(docOperationExternalDocs(description, url))
}

// RequestBodyContent documents a per-media-type request body on the most recently
// added route in the group.
func (grp *Group) RequestBodyContent(description string, required bool, content map[string]RouteMediaType) Router {
	return grp.document(docRequestBodyContent(description, required, content))
}

// ResponseContent documents a per-media-type response on the most recently added
// route in the group.
func (grp *Group) ResponseContent(status int, description string, content map[string]RouteMediaType) Router {
	return grp.document(docResponseContent(status, description, content))
}

// ResponseLink documents a response link on the most recently added route in the
// group.
func (grp *Group) ResponseLink(status int, name string, link map[string]any) Router {
	return grp.document(docResponseLink(status, name, link))
}

// OperationExtension merges arbitrary operation-object fields on the most recently
// added route in the group.
func (grp *Group) OperationExtension(fields map[string]any) Router {
	return grp.document(docOperationExtension(fields))
}

// Use registers a middleware route that will match requests
// with the provided prefix (which is optional and defaults to "/").
// Also, you can pass another app instance as a sub-router along a routing path.
// It's very useful to split up a large API as many independent routers and
// compose them as a single service using Use. The fiber's error handler and
// any of the fiber's sub apps are added to the application's error handlers
// to be invoked on errors that happen within the prefix route.
//
//		app.Use(func(c fiber.Ctx) error {
//		     return c.Next()
//		})
//		app.Use("/api", func(c fiber.Ctx) error {
//		     return c.Next()
//		})
//		app.Use("/api", handler, func(c fiber.Ctx) error {
//		     return c.Next()
//		})
//	 	subApp := fiber.New()
//		app.Use("/mounted-path", subApp)
//
// This method will match all HTTP verbs: GET, POST, PUT, HEAD etc...
func (grp *Group) Use(args ...any) Router {
	var subApp *App
	var prefix string
	var prefixes []string
	var handlers []Handler

	for i := range args {
		switch arg := args[i].(type) {
		case string:
			prefix = arg
		case *App:
			subApp = arg
		case []string:
			prefixes = arg
		default:
			handler, ok := toFiberHandler(arg)
			if !ok {
				panic(fmt.Sprintf("use: invalid handler %v\n", reflect.TypeOf(arg)))
			}
			handlers = append(handlers, handler)
		}
	}

	if len(prefixes) == 0 {
		prefixes = append(prefixes, prefix)
	}

	for _, prefix := range prefixes {
		if subApp != nil {
			grp.mount(prefix, subApp)
			continue
		}

		atomic.StoreUint64(&grp.lastRegID, grp.app.register([]string{methodUse}, getGroupPath(grp.Prefix, prefix), grp, "", handlers...))
	}

	if !grp.hasAnyRoute {
		grp.hasAnyRoute = true
	}

	return grp
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (grp *Group) Get(path string, handler any, handlers ...any) Router {
	return grp.Add([]string{MethodGet}, path, handler, handlers...)
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (grp *Group) Head(path string, handler any, handlers ...any) Router {
	return grp.Add([]string{MethodHead}, path, handler, handlers...)
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (grp *Group) Post(path string, handler any, handlers ...any) Router {
	return grp.Add([]string{MethodPost}, path, handler, handlers...)
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (grp *Group) Put(path string, handler any, handlers ...any) Router {
	return grp.Add([]string{MethodPut}, path, handler, handlers...)
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (grp *Group) Delete(path string, handler any, handlers ...any) Router {
	return grp.Add([]string{MethodDelete}, path, handler, handlers...)
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (grp *Group) Connect(path string, handler any, handlers ...any) Router {
	return grp.Add([]string{MethodConnect}, path, handler, handlers...)
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (grp *Group) Options(path string, handler any, handlers ...any) Router {
	return grp.Add([]string{MethodOptions}, path, handler, handlers...)
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the path to the target resource.
func (grp *Group) Trace(path string, handler any, handlers ...any) Router {
	return grp.Add([]string{MethodTrace}, path, handler, handlers...)
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (grp *Group) Patch(path string, handler any, handlers ...any) Router {
	return grp.Add([]string{MethodPatch}, path, handler, handlers...)
}

// Query registers a route for QUERY methods that performs a safe, idempotent
// query with a request body.
func (grp *Group) Query(path string, handler any, handlers ...any) Router {
	return grp.Add([]string{MethodQuery}, path, handler, handlers...)
}

// Add allows you to specify multiple HTTP methods to register a route.
// The provided handlers are executed in order, starting with `handler` and then the variadic `handlers`.
func (grp *Group) Add(methods []string, path string, handler any, handlers ...any) Router {
	converted := collectHandlers("group", append([]any{handler}, handlers...)...)
	atomic.StoreUint64(&grp.lastRegID, grp.app.register(methods, getGroupPath(grp.Prefix, path), grp, "", converted...))
	if !grp.hasAnyRoute {
		grp.hasAnyRoute = true
	}

	return grp
}

// All will register the handler on all HTTP methods
func (grp *Group) All(path string, handler any, handlers ...any) Router {
	_ = grp.Add(grp.app.config.RequestMethods, path, handler, handlers...)
	return grp
}

// Group is used for Routes with common prefix to define a new sub-router with optional middleware.
//
//	api := app.Group("/api")
//	api.Get("/users", handler)
func (grp *Group) Group(prefix string, handlers ...any) Router {
	prefix = getGroupPath(grp.Prefix, prefix)

	// Create new group
	newGrp := &Group{Prefix: prefix, app: grp.app, parentGroup: grp}
	if len(handlers) > 0 {
		converted := collectHandlers("group", handlers...)
		// The middleware belongs to the sub-group; writing it to the parent
		// would retarget the parent's later doc helpers at this Use route.
		atomic.StoreUint64(&newGrp.lastRegID, grp.app.register([]string{methodUse}, prefix, grp, "", converted...))
	}
	if err := grp.app.hooks.executeOnGroupHooks(*newGrp); err != nil {
		panic(err)
	}

	return newGrp
}

// Domain creates a new router scoped to the given hostname pattern within
// this group. Routes registered through the returned Router inherit the
// group prefix and only match requests whose hostname (from c.Hostname())
// matches the pattern. When TrustProxy is enabled and the proxy is trusted,
// the hostname may be derived from the X-Forwarded-Host header.
//
// To prevent header spoofing, configure both Config.TrustProxy and
// Config.TrustProxyConfig (trusted proxy IPs/ranges). See:
// https://docs.gofiber.io/api/fiber#trustproxy
//
//	api := app.Group("/api")
//	api.Domain("api.example.com").Get("/users", listUsers)
func (grp *Group) Domain(host string) Router {
	return &domainRouter{
		app:     grp.app,
		group:   grp,
		matcher: parseDomainPattern(host),
	}
}

// RouteChain creates a Registering instance scoped to the group's prefix,
// allowing chained route declarations for the same path.
func (grp *Group) RouteChain(path string) Register {
	// Create new group
	register := &Registering{app: grp.app, group: grp, path: getGroupPath(grp.Prefix, path)}

	return register
}

// Route is used to define routes with a common prefix inside the supplied
// function. It mirrors the legacy helper and reuses the Group method to create
// a sub-router.
func (grp *Group) Route(prefix string, fn func(router Router), name ...string) Router {
	if fn == nil {
		panic("route handler 'fn' cannot be nil")
	}
	// Create new group
	group := grp.Group(prefix)
	if len(name) > 0 {
		group.Name(name[0])
	}

	// Define routes
	fn(group)

	return group
}

// document applies a documentation change to the route this group registered last.
func (grp *Group) document(apply func(route *Route)) Router {
	grp.app.applyToRegistration(atomic.LoadUint64(&grp.lastRegID), apply)
	return grp
}
