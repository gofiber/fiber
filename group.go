// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"log"
	"reflect"
)

// Ensure Group implement Router interface
var _ Router = (*Group)(nil)

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

// Get ...
func (grp *Group) Get(path string, handlers ...Handler) *Route {
	return grp.Add(MethodGet, path, handlers...)
}

// Head ...
func (grp *Group) Head(path string, handlers ...Handler) *Route {
	return grp.Add(MethodHead, path, handlers...)
}

// Post ...
func (grp *Group) Post(path string, handlers ...Handler) *Route {
	return grp.Add(MethodPost, path, handlers...)
}

// Put ...
func (grp *Group) Put(path string, handlers ...Handler) *Route {
	return grp.Add(MethodPut, path, handlers...)
}

// Delete ...
func (grp *Group) Delete(path string, handlers ...Handler) *Route {
	return grp.Add(MethodDelete, path, handlers...)
}

// Connect ...
func (grp *Group) Connect(path string, handlers ...Handler) *Route {
	return grp.Add(MethodConnect, path, handlers...)
}

// Options ...
func (grp *Group) Options(path string, handlers ...Handler) *Route {
	return grp.Add(MethodOptions, path, handlers...)
}

// Trace ...
func (grp *Group) Trace(path string, handlers ...Handler) *Route {
	return grp.Add(MethodTrace, path, handlers...)
}

// Patch ...
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
	routes := make([]*Route, len(methodINT))
	for method, i := range methodINT {
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
	return grp.app.Group(prefix, handlers...)
}
