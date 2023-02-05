// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"reflect"
)

// Group struct
type Group struct {
	app         *App
	parentGroup *Group
	name        string

	Prefix string
}

// Name Assign name to specific route.
func (grp *Group) Name(name string) Router {
	grp.app.mutex.Lock()

	if grp.parentGroup != nil {
		grp.name = grp.parentGroup.name + name
	} else {
		grp.name = name
	}

	if err := grp.app.hooks.executeOnGroupNameHooks(*grp); err != nil {
		panic(err)
	}
	grp.app.mutex.Unlock()

	return grp
}

// Use registers a middleware route that will match requests
// with the provided prefix (which is optional and defaults to "/").
// Also, you can pass another app instance as a sub-router along a routing path.
// It's very useful to split up a large API as many independent routers and
// compose them as a single service using Use. The fiber's error handler and
// any of the fiber's sub apps are added to the application's error handlers
// to be invoked on errors that happen within the prefix route.
//
//		app.Use(func(c fiber.Ctx) error {
//		     return c.Next()
//		})
//		app.Use("/api", func(c fiber.Ctx) error {
//		     return c.Next()
//		})
//		app.Use("/api", handler, func(c fiber.Ctx) error {
//		     return c.Next()
//		})
//	 	subApp := fiber.New()
//		app.Use("/mounted-path", subApp)
//
// This method will match all HTTP verbs: GET, POST, PUT, HEAD etc...
func (grp *Group) Use(args ...any) Router {
	var subApp *App
	var prefix string
	var prefixes []string
	var handlers []Handler

	for i := 0; i < len(args); i++ {
		switch arg := args[i].(type) {
		case string:
			prefix = arg
		case *App:
			subApp = arg
		case []string:
			prefixes = arg
		case Handler:
			handlers = append(handlers, arg)
		default:
			panic(fmt.Sprintf("use: invalid handler %v\n", reflect.TypeOf(arg)))
		}
	}

	if len(prefixes) == 0 {
		prefixes = append(prefixes, prefix)
	}

	for _, prefix := range prefixes {
		if subApp != nil {
			grp.mount(prefix, subApp)
			return grp
		}

		grp.app.register([]string{methodUse}, getGroupPath(grp.Prefix, prefix), grp, nil, handlers...)
	}

	return grp
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (grp *Group) Get(path string, handler Handler, middleware ...Handler) Router {
	return grp.Add([]string{MethodGet}, path, handler, middleware...)
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (grp *Group) Head(path string, handler Handler, middleware ...Handler) Router {
	return grp.Add([]string{MethodHead}, path, handler, middleware...)
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (grp *Group) Post(path string, handler Handler, middleware ...Handler) Router {
	return grp.Add([]string{MethodPost}, path, handler, middleware...)
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (grp *Group) Put(path string, handler Handler, middleware ...Handler) Router {
	return grp.Add([]string{MethodPut}, path, handler, middleware...)
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (grp *Group) Delete(path string, handler Handler, middleware ...Handler) Router {
	return grp.Add([]string{MethodDelete}, path, handler, middleware...)
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (grp *Group) Connect(path string, handler Handler, middleware ...Handler) Router {
	return grp.Add([]string{MethodConnect}, path, handler, middleware...)
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (grp *Group) Options(path string, handler Handler, middleware ...Handler) Router {
	return grp.Add([]string{MethodOptions}, path, handler, middleware...)
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the path to the target resource.
func (grp *Group) Trace(path string, handler Handler, middleware ...Handler) Router {
	return grp.Add([]string{MethodTrace}, path, handler, middleware...)
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (grp *Group) Patch(path string, handler Handler, middleware ...Handler) Router {
	return grp.Add([]string{MethodPatch}, path, handler, middleware...)
}

// Add allows you to specify multiple HTTP methods to register a route.
func (grp *Group) Add(methods []string, path string, handler Handler, middleware ...Handler) Router {
	return grp.app.register(methods, getGroupPath(grp.Prefix, path), grp, handler, middleware...)
}

// Static will create a file server serving static files
func (grp *Group) Static(prefix, root string, config ...Static) Router {
	return grp.app.registerStatic(getGroupPath(grp.Prefix, prefix), root, config...)
}

// All will register the handler on all HTTP methods
func (grp *Group) All(path string, handler Handler, middleware ...Handler) Router {
	_ = grp.Add(grp.app.config.RequestMethods, path, handler, middleware...)
	return grp
}

// Group is used for Routes with common prefix to define a new sub-router with optional middleware.
//
//	api := app.Group("/api")
//	api.Get("/users", handler)
func (grp *Group) Group(prefix string, handlers ...Handler) Router {
	prefix = getGroupPath(grp.Prefix, prefix)
	if len(handlers) > 0 {
		_ = grp.app.register([]string{methodUse}, prefix, grp, nil, handlers...)
	}

	// Create new group
	newGrp := &Group{Prefix: prefix, app: grp.app, parentGroup: grp}
	if err := grp.app.hooks.executeOnGroupHooks(*newGrp); err != nil {
		panic(err)
	}

	return newGrp
}

// Route is used to define routes with a common prefix inside the common function.
// Uses Group method to define new sub-router.
func (grp *Group) Route(path string) Register {
	// Create new group
	register := &Registering{app: grp.app, path: getGroupPath(grp.Prefix, path)}

	return register
}
