// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

// Register struct
type Register struct {
	app *App

	Path string
}

// Use registers a middleware route that will match requests
// with the provided prefix (which is optional and defaults to "/").
//
//	app.Use(func(c fiber.Ctx) error {
//	     return c.Next()
//	})
//	app.Use("/api", func(c fiber.Ctx) error {
//	     return c.Next()
//	})
//	app.Use("/api", handler, func(c fiber.Ctx) error {
//	     return c.Next()
//	})
//
// This method will match all HTTP verbs: GET, POST, PUT, HEAD etc...
func (r *Register) All(handlers ...Handler) *Register {
	r.app.register(methodUse, r.Path, handlers...)
	return r
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (r *Register) Get(handlers ...Handler) *Register {
	r.app.Add(MethodHead, r.Path, handlers...).Add(MethodGet, r.Path, handlers...)
	return r
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (r *Register) Head(handlers ...Handler) *Register {
	return r.Add(MethodHead, handlers...)
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (r *Register) Post(handlers ...Handler) *Register {
	return r.Add(MethodPost, handlers...)
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (r *Register) Put(handlers ...Handler) *Register {
	return r.Add(MethodPut, handlers...)
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (r *Register) Delete(handlers ...Handler) *Register {
	return r.Add(MethodDelete, handlers...)
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (r *Register) Connect(handlers ...Handler) *Register {
	return r.Add(MethodConnect, handlers...)
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (r *Register) Options(handlers ...Handler) *Register {
	return r.Add(MethodOptions, handlers...)
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the r.Path to the target resource.
func (r *Register) Trace(handlers ...Handler) *Register {
	return r.Add(MethodTrace, handlers...)
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (r *Register) Patch(handlers ...Handler) *Register {
	return r.Add(MethodPatch, handlers...)
}

// Add allows you to specify a HTTP method to register a route
func (r *Register) Add(method string, handlers ...Handler) *Register {
	r.app.register(method, r.Path, handlers...)
	return r
}

// Static will create a file server serving static files
func (r *Register) Static(root string, config ...Static) *Register {
	r.app.registerStatic(r.Path, root, config...)
	return r
}

// Route is used to define routes with a common prefix inside the common function.
// Uses Group method to define new sub-router.
func (r *Register) Route(path string) *Register {
	// Create new group
	route := &Register{app: r.app, Path: getGroupPath(r.Path, path)}

	return route
}
