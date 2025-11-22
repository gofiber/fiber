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

	Add(methods []string, handler any, handlers ...any) Register

	RouteChain(path string) Register
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

// Add allows you to specify multiple HTTP methods to register a route.
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
