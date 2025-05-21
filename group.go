// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"reflect"
)

// Group struct
type Group[TCtx CtxGeneric[TCtx]] struct {
	app         *App[TCtx]
	parentGroup *Group[TCtx]
	name        string

	Prefix          string
	anyRouteDefined bool
}

// Name Assign name to specific route or group itself.
//
// If this method is used before any route added to group, it'll set group name and OnGroupNameHook will be used.
// Otherwise, it'll set route name and OnName hook will be used.
func (grp *Group[TCtx]) Name(name string) Router[TCtx] {
	if grp.anyRouteDefined {
		grp.app.Name(name)

		return grp
	}

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
func (grp *Group[TCtx]) Use(args ...any) Router[TCtx] {
	var subApp *App[TCtx]
	var prefix string
	var prefixes []string
	var handlers []Handler[TCtx]

	for i := 0; i < len(args); i++ {
		switch arg := args[i].(type) {
		case string:
			prefix = arg
		case *App[TCtx]:
			subApp = arg
		case []string:
			prefixes = arg
		case Handler[TCtx]:
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

		grp.app.register([]string{methodUse}, getGroupPath(grp.Prefix, prefix), grp, handlers...)
	}

	if !grp.anyRouteDefined {
		grp.anyRouteDefined = true
	}

	return grp
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (grp *Group[TCtx]) Get(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx] {
	return grp.Add([]string{MethodGet}, path, handler, handlers...)
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (grp *Group[TCtx]) Head(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx] {
	return grp.Add([]string{MethodHead}, path, handler, handlers...)
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (grp *Group[TCtx]) Post(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx] {
	return grp.Add([]string{MethodPost}, path, handler, handlers...)
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (grp *Group[TCtx]) Put(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx] {
	return grp.Add([]string{MethodPut}, path, handler, handlers...)
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (grp *Group[TCtx]) Delete(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx] {
	return grp.Add([]string{MethodDelete}, path, handler, handlers...)
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (grp *Group[TCtx]) Connect(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx] {
	return grp.Add([]string{MethodConnect}, path, handler, handlers...)
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (grp *Group[TCtx]) Options(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx] {
	return grp.Add([]string{MethodOptions}, path, handler, handlers...)
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the path to the target resource.
func (grp *Group[TCtx]) Trace(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx] {
	return grp.Add([]string{MethodTrace}, path, handler, handlers...)
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (grp *Group[TCtx]) Patch(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx] {
	return grp.Add([]string{MethodPatch}, path, handler, handlers...)
}

// Add allows you to specify multiple HTTP methods to register a route.
func (grp *Group[TCtx]) Add(methods []string, path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx] {
	grp.app.register(methods, getGroupPath(grp.Prefix, path), grp, append([]Handler{handler}, handlers...)...)
	if !grp.anyRouteDefined {
		grp.anyRouteDefined = true
	}

	return grp
}

// All will register the handler on all HTTP methods
func (grp *Group[TCtx]) All(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx] {
	_ = grp.Add(grp.app.config.RequestMethods, path, handler, handlers...)
	return grp
}

// Group is used for Routes with common prefix to define a new sub-router with optional middleware.
//
//	api := app.Group("/api")
//	api.Get("/users", handler)
func (grp *Group[TCtx]) Group(prefix string, handlers ...Handler[TCtx]) Router[TCtx] {
	prefix = getGroupPath(grp.Prefix, prefix)
	if len(handlers) > 0 {
		grp.app.register([]string{methodUse}, prefix, grp, handlers...)
	}

	// Create new group
	newGrp := &Group[TCtx]{Prefix: prefix, app: grp.app, parentGroup: grp}
	if err := grp.app.hooks.executeOnGroupHooks(*newGrp); err != nil {
		panic(err)
	}

	return newGrp
}

// Route is used to define routes with a common prefix inside the common function.
// Uses Group method to define new sub-router.
func (grp *Group[TCtx]) Route(path string) Register[TCtx] {
	// Create new group
	register := &Registering[TCtx]{app: grp.app, path: getGroupPath(grp.Prefix, path)}

	return register
}
