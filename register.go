// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

// Register defines all router handle interface generate by Route().
type Register interface {
	All(handler Handler, handlers ...Handler) Register
	Get(handler Handler, handlers ...Handler) Register
	Head(handler Handler, handlers ...Handler) Register
	Post(handler Handler, handlers ...Handler) Register
	Put(handler Handler, handlers ...Handler) Register
	Delete(handler Handler, handlers ...Handler) Register
	Connect(handler Handler, handlers ...Handler) Register
	Options(handler Handler, handlers ...Handler) Register
	Trace(handler Handler, handlers ...Handler) Register
	Patch(handler Handler, handlers ...Handler) Register

	Add(methods []string, handler Handler, handlers ...Handler) Register

	Route(path string) Register
}

var _ (Register) = (*Registering)(nil)

// Registering struct
type Registering struct {
	app *App

	path string
}

// All registers a middleware route that will match requests
// with the provided path which is stored in register struct.
//
//	app.Route("/").All(func(c fiber.Ctx) error {
//	     return c.Next()
//	})
//	app.Route("/api").All(func(c fiber.Ctx) error {
//	     return c.Next()
//	})
//	app.Route("/api").All(handler, func(c fiber.Ctx) error {
//	     return c.Next()
//	})
//
// This method will match all HTTP verbs: GET, POST, PUT, HEAD etc...
func (r *Registering) All(handler Handler, handlers ...Handler) Register {
	r.app.register([]string{methodUse}, r.path, nil, append([]Handler{handler}, handlers...)...)
	return r
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (r *Registering) Get(handler Handler, handlers ...Handler) Register {
	r.app.Add([]string{MethodGet}, r.path, handler, handlers...)
	return r
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (r *Registering) Head(handler Handler, handlers ...Handler) Register {
	return r.Add([]string{MethodHead}, handler, handlers...)
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (r *Registering) Post(handler Handler, handlers ...Handler) Register {
	return r.Add([]string{MethodPost}, handler, handlers...)
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (r *Registering) Put(handler Handler, handlers ...Handler) Register {
	return r.Add([]string{MethodPut}, handler, handlers...)
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (r *Registering) Delete(handler Handler, handlers ...Handler) Register {
	return r.Add([]string{MethodDelete}, handler, handlers...)
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (r *Registering) Connect(handler Handler, handlers ...Handler) Register {
	return r.Add([]string{MethodConnect}, handler, handlers...)
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (r *Registering) Options(handler Handler, handlers ...Handler) Register {
	return r.Add([]string{MethodOptions}, handler, handlers...)
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the r.Path to the target resource.
func (r *Registering) Trace(handler Handler, handlers ...Handler) Register {
	return r.Add([]string{MethodTrace}, handler, handlers...)
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (r *Registering) Patch(handler Handler, handlers ...Handler) Register {
	return r.Add([]string{MethodPatch}, handler, handlers...)
}

// Add allows you to specify multiple HTTP methods to register a route.
func (r *Registering) Add(methods []string, handler Handler, handlers ...Handler) Register {
	r.app.register(methods, r.path, nil, append([]Handler{handler}, handlers...)...)
	return r
}

// Route returns a new Register instance whose route path takes
// the path in the current instance as its prefix.
func (r *Registering) Route(path string) Register {
	// Create new group
	route := &Registering{app: r.app, path: getGroupPath(r.path, path)}

	return route
}
