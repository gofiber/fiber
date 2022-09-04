package fiber

type route struct {
	app  *App
	path string
}

type IRoute interface {
	Use(handlers ...Handler) IRoute

	Get(handlers ...Handler) IRoute
	Head(handlers ...Handler) IRoute
	Post(handlers ...Handler) IRoute
	Put(handlers ...Handler) IRoute
	Delete(handlers ...Handler) IRoute
	Connect(handlers ...Handler) IRoute
	Options(handlers ...Handler) IRoute
	Trace(handlers ...Handler) IRoute
	Patch(handlers ...Handler) IRoute

	Add(method string, handlers ...Handler) IRoute
	All(handlers ...Handler) IRoute
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (rf *route) Use(handlers ...Handler) IRoute {
	rf.app.register(methodUse, rf.path, handlers...)
	return rf
}

// All will register the handler on all HTTP methods
func (rf *route) All(handlers ...Handler) IRoute {
	for _, method := range intMethod {
		_ = rf.app.register(method, rf.path, handlers...)
	}
	return rf
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (rf *route) Get(handlers ...Handler) IRoute {
	rf.app.Head(rf.path, handlers...)
	rf.app.register(MethodGet, rf.path, handlers...)
	return rf
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (rf *route) Head(handlers ...Handler) IRoute {
	rf.app.register(MethodHead, rf.path, handlers...)
	return rf
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (rf *route) Post(handlers ...Handler) IRoute {
	rf.app.register(MethodPost, rf.path, handlers...)
	return rf
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (rf *route) Put(handlers ...Handler) IRoute {
	rf.app.register(MethodPut, rf.path, handlers...)
	return rf
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (rf *route) Delete(handlers ...Handler) IRoute {
	rf.app.register(MethodDelete, rf.path, handlers...)
	return rf
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (rf *route) Connect(handlers ...Handler) IRoute {
	rf.app.register(MethodConnect, rf.path, handlers...)
	return rf
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (rf *route) Options(handlers ...Handler) IRoute {
	rf.app.register(MethodOptions, rf.path, handlers...)
	return rf
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the path to the target resource.
func (rf *route) Trace(handlers ...Handler) IRoute {
	rf.app.register(MethodTrace, rf.path, handlers...)
	return rf
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (rf *route) Patch(handlers ...Handler) IRoute {
	rf.app.register(MethodPatch, rf.path, handlers...)
	return rf
}

// Add allows you to specify a HTTP method to register a route
func (rf *route) Add(method string, handlers ...Handler) IRoute {
	rf.app.register(method, rf.path, handlers...)
	return rf
}
