package fiber

type routeFunc struct {
	app  *App
	path string
}

type RouteFunc interface {
	Use(handlers ...Handler) RouteFunc

	Get(handlers ...Handler) RouteFunc
	Head(handlers ...Handler) RouteFunc
	Post(handlers ...Handler) RouteFunc
	Put(handlers ...Handler) RouteFunc
	Delete(handlers ...Handler) RouteFunc
	Connect(handlers ...Handler) RouteFunc
	Options(handlers ...Handler) RouteFunc
	Trace(handlers ...Handler) RouteFunc
	Patch(handlers ...Handler) RouteFunc

	Add(method, path string, handlers ...Handler) RouteFunc
	All(handlers ...Handler) RouteFunc
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (rf *routeFunc) Use(handlers ...Handler) RouteFunc {
	rf.app.Add(methodUse, rf.path, handlers...)
	return rf
}

// All will register the handler on all HTTP methods
func (rf *routeFunc) All(handlers ...Handler) RouteFunc {
	for _, method := range intMethod {
		_ = rf.app.Add(method, rf.path, handlers...)
	}
	return rf
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (rf *routeFunc) Get(handlers ...Handler) RouteFunc {
	rf.app.Head(rf.path, handlers...).Add(MethodGet, rf.path, handlers...)
	return rf
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (rf *routeFunc) Head(handlers ...Handler) RouteFunc {
	rf.app.Add(MethodHead, rf.path, handlers...)
	return rf
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (rf *routeFunc) Post(handlers ...Handler) RouteFunc {
	rf.app.Add(MethodPost, rf.path, handlers...)
	return rf
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (rf *routeFunc) Put(handlers ...Handler) RouteFunc {
	rf.app.Add(MethodPut, rf.path, handlers...)
	return rf
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (rf *routeFunc) Delete(handlers ...Handler) RouteFunc {
	rf.app.Add(MethodDelete, rf.path, handlers...)
	return rf
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (rf *routeFunc) Connect(handlers ...Handler) RouteFunc {
	rf.app.Add(MethodConnect, rf.path, handlers...)
	return rf
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (rf *routeFunc) Options(handlers ...Handler) RouteFunc {
	rf.app.Add(MethodOptions, rf.path, handlers...)
	return rf
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the path to the target resource.
func (rf *routeFunc) Trace(handlers ...Handler) RouteFunc {
	rf.app.Add(MethodTrace, rf.path, handlers...)
	return rf
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (rf *routeFunc) Patch(handlers ...Handler) RouteFunc {
	rf.app.Add(MethodPatch, rf.path, handlers...)
	return rf
}

// Add allows you to specify a HTTP method to register a route
func (rf *routeFunc) Add(method, path string, handlers ...Handler) RouteFunc {
	rf.app.register(method, path, handlers...)
	return rf
}
