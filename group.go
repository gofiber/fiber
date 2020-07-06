// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"log"
	"reflect"
)

// Group struct
type Group struct {
	app    *App
	prefix string
}

// Use registers a middleware route.
// Middleware matches requests beginning with the provided prefix.
// Providing a prefix is optional, it defaults to "/".
//
// - group.Use(handler)
// - group.Use("/api", handler)
// - group.Use("/api", handler, handler)
func (grp *Group) Use(args ...interface{}) *Route {
	var path = ""
	var handlers []Handler
	for i := 0; i < len(args); i++ {
		switch arg := args[i].(type) {
		case string:
			path = arg
		case Handler:
			handlers = append(handlers, arg)
		default:
			log.Fatalf("Use: Invalid Handler %v", reflect.TypeOf(arg))
		}
	}
	return grp.app.register("USE", getGroupPath(grp.prefix, path), handlers...)
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (grp *Group) Get(path string, handlers ...Handler) *Route {
	return grp.Add(MethodGet, path, handlers...)
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (grp *Group) Head(path string, handlers ...Handler) *Route {
	return grp.Add(MethodHead, path, handlers...)
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (grp *Group) Post(path string, handlers ...Handler) *Route {
	return grp.Add(MethodPost, path, handlers...)
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (grp *Group) Put(path string, handlers ...Handler) *Route {
	return grp.Add(MethodPut, path, handlers...)
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (grp *Group) Delete(path string, handlers ...Handler) *Route {
	return grp.Add(MethodDelete, path, handlers...)
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (grp *Group) Connect(path string, handlers ...Handler) *Route {
	return grp.Add(MethodConnect, path, handlers...)
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (grp *Group) Options(path string, handlers ...Handler) *Route {
	return grp.Add(MethodOptions, path, handlers...)
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the path to the target resource.
func (grp *Group) Trace(path string, handlers ...Handler) *Route {
	return grp.Add(MethodTrace, path, handlers...)
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (grp *Group) Patch(path string, handlers ...Handler) *Route {
	return grp.Add(MethodPatch, path, handlers...)
}

// Add ...
func (grp *Group) Add(method, path string, handlers ...Handler) *Route {
	return grp.app.register(method, getGroupPath(grp.prefix, path), handlers...)
}

// Static ...
func (grp *Group) Static(prefix, root string, config ...Static) *Route {
	return grp.app.registerStatic(getGroupPath(grp.prefix, prefix), root, config...)
}

// All ...
func (grp *Group) All(path string, handlers ...Handler) []*Route {
	routes := make([]*Route, len(intMethod))
	for i, method := range intMethod {
		routes[i] = grp.Add(method, path, handlers...)
	}
	return routes
}

// Group is used for Routes with common prefix to define a new sub-router with optional middleware.
func (grp *Group) Group(prefix string, handlers ...Handler) *Group {
	prefix = getGroupPath(grp.prefix, prefix)
	if len(handlers) > 0 {
		grp.app.register("USE", prefix, handlers...)
	}
	return grp.app.Group(prefix)
}
