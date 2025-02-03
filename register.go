// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

// Register defines all router handle interface generate by Route().
type Register[TCtx CtxGeneric[TCtx]] interface {
	All(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx]
	Get(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx]
	Head(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx]
	Post(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx]
	Put(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx]
	Delete(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx]
	Connect(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx]
	Options(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx]
	Trace(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx]
	Patch(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx]

	Add(methods []string, handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx]

	Route(path string) Register[TCtx]
}

var _ Register[*DefaultCtx] = (*Registering[*DefaultCtx])(nil)

// Registering struct
type Registering[TCtx CtxGeneric[TCtx]] struct {
	app *App[TCtx]

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
func (r *Registering[TCtx]) All(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx] {
	r.app.register([]string{methodUse}, r.path, nil, handler, middleware...)
	return r
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (r *Registering[TCtx]) Get(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx] {
	r.app.Add([]string{MethodGet}, r.path, handler, middleware...)
	return r
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (r *Registering[TCtx]) Head(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx] {
	return r.Add([]string{MethodHead}, handler, middleware...)
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (r *Registering[TCtx]) Post(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx] {
	return r.Add([]string{MethodPost}, handler, middleware...)
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (r *Registering[TCtx]) Put(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx] {
	return r.Add([]string{MethodPut}, handler, middleware...)
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (r *Registering[TCtx]) Delete(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx] {
	return r.Add([]string{MethodDelete}, handler, middleware...)
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (r *Registering[TCtx]) Connect(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx] {
	return r.Add([]string{MethodConnect}, handler, middleware...)
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (r *Registering[TCtx]) Options(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx] {
	return r.Add([]string{MethodOptions}, handler, middleware...)
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the r.Path to the target resource.
func (r *Registering[TCtx]) Trace(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx] {
	return r.Add([]string{MethodTrace}, handler, middleware...)
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (r *Registering[TCtx]) Patch(handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx] {
	return r.Add([]string{MethodPatch}, handler, middleware...)
}

// Add allows you to specify multiple HTTP methods to register a route.
func (r *Registering[TCtx]) Add(methods []string, handler Handler[TCtx], middleware ...Handler[TCtx]) Register[TCtx] {
	r.app.register(methods, r.path, nil, handler, middleware...)
	return r
}

// Route returns a new Register instance whose route path takes
// the path in the current instance as its prefix.
func (r *Registering[TCtx]) Route(path string) Register[TCtx] {
	// Create new group
	route := &Registering[TCtx]{app: r.app, path: getGroupPath(r.path, path)}

	return route
}
