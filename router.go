package fiber

import (
	"fmt"
	"github.com/gofiber/fiber/v3/routing"
	"reflect"
)

type Router interface {
	FindNextHandler(method string, path string) Handler
	GetAllRoutes() []any // TODO: specific routes ?
	// TODO: Add contrains function ? or just in expressjs router
	// TODO: add mount function for the merge of the routers
}

type Route struct {
	// Public fields
	Method string `json:"method"` // HTTP method
	Name   string `json:"name"`   // Route's name
	//nolint:revive // Having both a Path (uppercase) and a path (lowercase) is fine
	Path     string    `json:"path"`   // Original registered route path
	Params   []string  `json:"params"` // Case sensitive param keys
	Handlers []Handler `json:"-"`      // Ctx handlers
}

// TODO: add Route getters

type IGroup interface {
	GetPrefix() string
}

// Group struct
type Group struct {
	Prefix string
	IGroup
}

// TODO: move it to our Router interface `router.RegisterCustomConstraint`
// RegisterCustomConstraint allows to register custom constraint.
func (app *App[TRouter]) RegisterCustomConstraint(constraint CustomConstraint) {
	app.customConstraints = append(app.customConstraints, constraint)
}

// Name Assign name to specific route.
func (app *App[TRouter]) Name(name string) routing.ExpressjsRouterI {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	for _, routes := range app.stack {
		for _, route := range routes {
			isMethodValid := route.Method == app.latestRoute.Method || app.latestRoute.use ||
				(app.latestRoute.Method == MethodGet && route.Method == MethodHead)

			if route.Path == app.latestRoute.Path && isMethodValid {
				route.Name = name
				if route.group != nil {
					route.Name = route.group.name + route.Name
				}
			}
		}
	}

	if err := app.hooks.executeOnNameHooks(*app.latestRoute); err != nil {
		panic(err)
	}

	return app
}

// GetRoute Get route by name
func (app *App[TRouter]) GetRoute(name string) routing.Route {
	for _, routes := range app.stack {
		for _, route := range routes {
			if route.Name == name {
				return *route
			}
		}
	}

	return routing.Route{}
}

// TODO: part of the router api for the interchangeable class
// GetRoutes Get all routes. When filterUseOption equal to true, it will filter the routes registered by the middleware.
func (app *App[TRouter]) GetRoutes(filterUseOption ...bool) []routing.Route {
	var rs []routing.Route
	var filterUse bool
	if len(filterUseOption) != 0 {
		filterUse = filterUseOption[0]
	}
	for _, routes := range app.stack {
		for _, route := range routes {
			if filterUse && route.use {
				continue
			}
			rs = append(rs, *route)
		}
	}
	return rs
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
func (app *App[TRouter]) Use(args ...any) routing.ExpressjsRouterI {
	var prefix string
	var subApp *App[TRouter]
	var prefixes []string
	var handlers []Handler

	for i := 0; i < len(args); i++ {
		switch arg := args[i].(type) {
		case string:
			prefix = arg
		case *App[TRouter]:
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
			app.mount(prefix, subApp)
			return app
		}

		app.register([]string{methodUse}, prefix, nil, nil, handlers...)
	}

	return app
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (app *App[TRouter]) Get(path string, handler Handler, middleware ...Handler) routing.ExpressjsRouterI {
	return app.Add([]string{MethodGet}, path, handler, middleware...)
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (app *App[TRouter]) Head(path string, handler Handler, middleware ...Handler) routing.ExpressjsRouterI {
	return app.Add([]string{MethodHead}, path, handler, middleware...)
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (app *App[TRouter]) Post(path string, handler Handler, middleware ...Handler) routing.ExpressjsRouterI {
	return app.Add([]string{MethodPost}, path, handler, middleware...)
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (app *App[TRouter]) Put(path string, handler Handler, middleware ...Handler) routing.ExpressjsRouterI {
	return app.Add([]string{MethodPut}, path, handler, middleware...)
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (app *App[TRouter]) Delete(path string, handler Handler, middleware ...Handler) routing.ExpressjsRouterI {
	return app.Add([]string{MethodDelete}, path, handler, middleware...)
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (app *App[TRouter]) Connect(path string, handler Handler, middleware ...Handler) routing.ExpressjsRouterI {
	return app.Add([]string{MethodConnect}, path, handler, middleware...)
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (app *App[TRouter]) Options(path string, handler Handler, middleware ...Handler) routing.ExpressjsRouterI {
	return app.Add([]string{MethodOptions}, path, handler, middleware...)
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the path to the target resource.
func (app *App[TRouter]) Trace(path string, handler Handler, middleware ...Handler) routing.ExpressjsRouterI {
	return app.Add([]string{MethodTrace}, path, handler, middleware...)
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (app *App[TRouter]) Patch(path string, handler Handler, middleware ...Handler) routing.ExpressjsRouterI {
	return app.Add([]string{MethodPatch}, path, handler, middleware...)
}

// Add allows you to specify multiple HTTP methods to register a route.
func (app *App[TRouter]) Add(methods []string, path string, handler Handler, middleware ...Handler) routing.ExpressjsRouterI {
	app.register(methods, path, nil, handler, middleware...)

	return app
}

// Static will create a file server serving static files
func (app *App[TRouter]) Static(prefix, root string, config ...Static) routing.ExpressjsRouterI {
	app.registerStatic(prefix, root, config...)

	return app
}

// All will register the handler on all HTTP methods
func (app *App[TRouter]) All(path string, handler Handler, middleware ...Handler) routing.ExpressjsRouterI {
	return app.Add(app.config.RequestMethods, path, handler, middleware...)
}

// Group is used for Routes with common prefix to define a new sub-router with optional middleware.
//
//	api := app.Group("/api")
//	api.Get("/users", handler)
func (app *App[TRouter]) Group(prefix string, handlers ...Handler) routing.ExpressjsRouterI {
	grp := &routing.Group{Prefix: prefix, app: app}
	if len(handlers) > 0 {
		app.register([]string{methodUse}, prefix, grp, nil, handlers...)
	}
	if err := app.hooks.executeOnGroupHooks(*grp); err != nil {
		panic(err)
	}

	return grp
}

// Route is used to define routes with a common prefix inside the common function.
// Uses Group method to define new sub-router.
func (app *App[TRouter]) Route(path string) routing.Register {
	// Create new route
	route := &routing.Registering{app: app, path: path}

	return route
}

// TODO: move to router
// Stack returns the raw router stack.
func (app *App[TRouter]) Stack() [][]*routing.Route {
	return app.stack
}

// TODO: move to router
// HandlersCount returns the amount of registered handlers.
func (app *App[TRouter]) HandlersCount() uint32 {
	return app.handlersCount
}
