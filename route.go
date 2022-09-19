package fiber

type route struct {
	app    *App
	router *Router
	path   string
}

type IRoute interface {
	Use(handlers ...Handler) IRoute

	All(handlers ...Handler) IRoute
	Get(handlers ...Handler) IRoute
	Head(handlers ...Handler) IRoute
	Post(handlers ...Handler) IRoute
	Put(handlers ...Handler) IRoute
	Delete(handlers ...Handler) IRoute
	Connect(handlers ...Handler) IRoute
	Options(handlers ...Handler) IRoute
	Trace(handlers ...Handler) IRoute
	Patch(handlers ...Handler) IRoute
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (r *route) Use(handlers ...Handler) IRoute {
	if r.app != nil {
		r.app.register(methodUse, r.path, handlers...)
	}
	if r.router != nil {
		r.router.register(methodUse, r.path, handlers...)
	}

	return r
}

// All will register the handler on all HTTP methods
func (r *route) All(handlers ...Handler) IRoute {
	if r.app != nil {
		for _, method := range intMethod {
			_ = r.app.register(method, r.path, handlers...)
		}
	}
	if r.router != nil {
		for _, method := range intMethod {
			r.router.register(method, r.path, handlers...)
		}
	}

	return r
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (r *route) Get(handlers ...Handler) IRoute {
	if r.app != nil {
		r.app.Head(r.path, handlers...)
		r.app.register(MethodGet, r.path, handlers...)
	}
	if r.router != nil {
		r.router.Head(r.path, handlers...)
		r.router.register(MethodGet, r.path, handlers...)
	}

	return r
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (r *route) Head(handlers ...Handler) IRoute {
	if r.app != nil {
		r.app.register(MethodHead, r.path, handlers...)
	}
	if r.router != nil {
		r.router.register(MethodHead, r.path, handlers...)
	}

	return r
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (r *route) Post(handlers ...Handler) IRoute {
	if r.app != nil {
		r.app.register(MethodPost, r.path, handlers...)
	}
	if r.router != nil {
		r.router.register(MethodPost, r.path, handlers...)
	}

	return r
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (r *route) Put(handlers ...Handler) IRoute {
	if r.app != nil {
		r.app.register(MethodPut, r.path, handlers...)
	}
	if r.router != nil {
		r.router.register(MethodPut, r.path, handlers...)
	}

	return r
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (r *route) Delete(handlers ...Handler) IRoute {
	if r.app != nil {
		r.app.register(MethodDelete, r.path, handlers...)
	}
	if r.router != nil {
		r.router.register(MethodDelete, r.path, handlers...)
	}

	return r
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (r *route) Connect(handlers ...Handler) IRoute {
	if r.app != nil {
		r.app.register(MethodConnect, r.path, handlers...)
	}
	if r.router != nil {
		r.router.register(MethodConnect, r.path, handlers...)
	}

	return r
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (r *route) Options(handlers ...Handler) IRoute {
	if r.app != nil {
		r.app.register(MethodOptions, r.path, handlers...)
	}
	if r.router != nil {
		r.router.register(MethodOptions, r.path, handlers...)
	}

	return r
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the path to the target resource.
func (r *route) Trace(handlers ...Handler) IRoute {
	if r.app != nil {
		r.app.register(MethodTrace, r.path, handlers...)
	}
	if r.router != nil {
		r.router.register(MethodOptions, r.path, handlers...)
	}

	return r
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (r *route) Patch(handlers ...Handler) IRoute {
	if r.app != nil {
		r.app.register(MethodPatch, r.path, handlers...)
	}
	if r.router != nil {
		r.router.register(MethodPatch, r.path, handlers...)
	}

	return r
}
