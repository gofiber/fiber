// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
)

// Group struct
type Group struct {
	app  *App
	name string

	Prefix string
}

// Mount attaches another app instance as a sub-router along a routing path.
// It's very useful to split up a large API as many independent routers and
// compose them as a single service using Mount.
func (grp *Group) Mount(prefix string, fiber *App) Router {
	stack := fiber.Stack()
	groupPath := getGroupPath(grp.Prefix, prefix)
	groupPath = strings.TrimRight(groupPath, "/")
	if groupPath == "" {
		groupPath = "/"
	}

	for m := range stack {
		for r := range stack[m] {
			route := grp.app.copyRoute(stack[m][r])
			grp.app.addRoute(route.Method, grp.app.addPrefixToRoute(groupPath, route))
		}
	}

	// Support for configs of mounted-apps and sub-mounted-apps
	for mountedPrefixes, subApp := range fiber.appList {
		grp.app.appList[groupPath+mountedPrefixes] = subApp
		subApp.init()
	}

	atomic.AddUint32(&grp.app.handlersCount, fiber.handlersCount)

	return grp
}

// Assign name to specific route.
func (grp *Group) Name(name string) Router {
	grp.app.mutex.Lock()
	if strings.HasPrefix(grp.Prefix, grp.app.latestGroup.Prefix) {
		grp.name = grp.app.latestGroup.name + name
	} else {
		grp.name = name
	}

	grp.app.latestGroup = grp

	if err := grp.app.hooks.executeOnGroupNameHooks(*grp.app.latestGroup); err != nil {
		panic(err)
	}
	grp.app.mutex.Unlock()

	return grp
}

// Use registers a middleware route that will match requests
// with the provided prefix (which is optional and defaults to "/").
//
//  app.Use(func(c *fiber.Ctx) error {
//       return c.Next()
//  })
//  app.Use("/api", func(c *fiber.Ctx) error {
//       return c.Next()
//  })
//  app.Use("/api", handler, func(c *fiber.Ctx) error {
//       return c.Next()
//  })
//
// This method will match all HTTP verbs: GET, POST, PUT, HEAD etc...
func (grp *Group) Use(args ...interface{}) Router {
	prefix := ""
	var handlers []Handler
	for i := 0; i < len(args); i++ {
		switch arg := args[i].(type) {
		case string:
			prefix = arg
		case Handler:
			handlers = append(handlers, arg)
		default:
			panic(fmt.Sprintf("use: invalid handler %v\n", reflect.TypeOf(arg)))
		}
	}
	grp.app.register(methodUse, getGroupPath(grp.Prefix, prefix), handlers...)
	return grp
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (grp *Group) Get(path string, handlers ...Handler) Router {
	path = getGroupPath(grp.Prefix, path)
	return grp.app.Add(MethodHead, path, handlers...).Add(MethodGet, path, handlers...)
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (grp *Group) Head(path string, handlers ...Handler) Router {
	return grp.Add(MethodHead, path, handlers...)
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (grp *Group) Post(path string, handlers ...Handler) Router {
	return grp.Add(MethodPost, path, handlers...)
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (grp *Group) Put(path string, handlers ...Handler) Router {
	return grp.Add(MethodPut, path, handlers...)
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (grp *Group) Delete(path string, handlers ...Handler) Router {
	return grp.Add(MethodDelete, path, handlers...)
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (grp *Group) Connect(path string, handlers ...Handler) Router {
	return grp.Add(MethodConnect, path, handlers...)
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (grp *Group) Options(path string, handlers ...Handler) Router {
	return grp.Add(MethodOptions, path, handlers...)
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the path to the target resource.
func (grp *Group) Trace(path string, handlers ...Handler) Router {
	return grp.Add(MethodTrace, path, handlers...)
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (grp *Group) Patch(path string, handlers ...Handler) Router {
	return grp.Add(MethodPatch, path, handlers...)
}

// Add allows you to specify a HTTP method to register a route
func (grp *Group) Add(method, path string, handlers ...Handler) Router {
	return grp.app.register(method, getGroupPath(grp.Prefix, path), handlers...)
}

// Static will create a file server serving static files
func (grp *Group) Static(prefix, root string, config ...Static) Router {
	return grp.app.registerStatic(getGroupPath(grp.Prefix, prefix), root, config...)
}

// All will register the handler on all HTTP methods
func (grp *Group) All(path string, handlers ...Handler) Router {
	for _, method := range intMethod {
		_ = grp.Add(method, path, handlers...)
	}
	return grp
}

// Group is used for Routes with common prefix to define a new sub-router with optional middleware.
//  api := app.Group("/api")
//  api.Get("/users", handler)
func (grp *Group) Group(prefix string, handlers ...Handler) Router {
	prefix = getGroupPath(grp.Prefix, prefix)
	if len(handlers) > 0 {
		_ = grp.app.register(methodUse, prefix, handlers...)
	}
	return grp.app.Group(prefix)
}

// Route is used to define routes with a common prefix inside the common function.
// Uses Group method to define new sub-router.
func (grp *Group) Route(prefix string, fn func(router Router), name ...string) Router {
	// Create new group
	group := grp.Group(prefix)
	if len(name) > 0 {
		group.Name(name[0])
	}

	// Define routes
	fn(group)

	return group
}
