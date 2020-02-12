// 🚀 Fiber is an Express.js inspired web framework written in Go with 💖
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 Links: https://github.com/gofiber/fiber, https://fiber.wiki

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @erikdubbelboer, @savsgio, @julienschmidt, @koddr

package fiber

import (
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/valyala/fasthttp"
)

// Ctx is the context that contains everything
type Ctx struct {
	route    *Route
	next     bool
	params   *[]string
	values   []string
	Fasthttp *fasthttp.RequestCtx
}

// Route struct
type Route struct {
	// HTTP method in uppercase, can be a * for Use() & All()
	Method string
	// Stores the original path
	Path string
	// Bool that defines if the route is a Use() middleware
	Midware bool
	// wildcard bool is for routes without a path, * and /*
	Wildcard bool
	// Stores compiled regex special routes :params, *wildcards, optionals?
	Regex *regexp.Regexp
	// Store params if special routes :params, *wildcards, optionals?
	Params []string
	// Callback function for specific route
	Handler func(*Ctx) error
}

// Ctx pool
var poolCtx = sync.Pool{
	New: func() interface{} {
		return new(Ctx)
	},
}

// Get new Ctx from pool
func acquireCtx(fctx *fasthttp.RequestCtx) *Ctx {
	ctx := poolCtx.Get().(*Ctx)
	ctx.Fasthttp = fctx
	return ctx
}

// Return Context to pool
func releaseCtx(ctx *Ctx) {
	ctx.route = nil
	ctx.next = false
	ctx.params = nil
	ctx.values = nil
	ctx.Fasthttp = nil
	poolCtx.Put(ctx)
}

func (grp *Group) register(method string, args ...interface{}) {
	path := grp.path
	var handler func(*Ctx)
	if len(args) == 1 {
		handler = args[0].(func(*Ctx))
	} else if len(args) > 1 {
		path = path + args[0].(string)
		handler = args[1].(func(*Ctx))
		if path[0] != '/' && path[0] != '*' {
			path = "/" + path
		}
		path = strings.Replace(path, "//", "/", -1)
		path = filepath.Clean(path)
		path = filepath.ToSlash(path)
	}
	grp.app.register(method, path, handler)
}

// Function to add a route correctly
func (app *Application) register(method string, args ...interface{}) {
	// Set if method is Use() midware
	var midware = method == "USE"

	// Match any method
	if method == "ALL" || midware {
		method = "*"
	}

	// Prepare possible variables
	var path string        // We could have a path/prefix
	var handler func(*Ctx) error // We could have a ctx handler

	// Only 1 argument, so no path/prefix
	if len(args) == 1 {
		handler = args[0].(func(*Ctx) error)
	} else if len(args) > 1 {
		path = args[0].(string)
		handler = args[1].(func(*Ctx) error)
		if path == "" || path[0] != '/' && path[0] != '*' {
			path = "/" + path
		}
	}

	if midware && strings.Contains(path, "/:") {
		log.Fatal("Router: You cannot use :params in Use()")
	}

	// If Use() path == "/", match anything aka *
	if midware && path == "/" {
		path = "*"
	}

	// If the route needs to match any path
	if path == "" || path == "*" || path == "/*" {
		app.routes = append(app.routes, &Route{method, path, midware, true, nil, nil, handler})
		return
	}

	// Get params from path
	params := getParams(path)

	// If path has no params (simple path), we don't need regex (also for use())
	if midware || len(params) == 0 {
		app.routes = append(app.routes, &Route{method, path, midware, false, nil, nil, handler})
		return
	}

	// We have parameters, so we need to compile regex from the path
	regex, err := getRegex(path)
	if err != nil {
		log.Fatal("Router: Invalid url pattern: " + path)
	}

	// Add regex + params to route
	app.routes = append(app.routes, &Route{method, path, midware, false, regex, params, handler})
}

// then try to match a route as efficient as possible.
func (app *Application) handler(fctx *fasthttp.RequestCtx) {
	found := false

	// get custom context from sync pool
	ctx := acquireCtx(fctx)

	// get path and method from main context
	path := ctx.Path()
	method := ctx.Method()

	// loop trough routes
	for _, route := range app.routes {
		// Skip route if method is not allowed
		if route.Method != "*" && route.Method != method {
			continue
		}

		// First check if we match a wildcard or static path
		if route.Wildcard || route.Path == path {
			// if route.wildcard || (route.path == path && route.params == nil) {
			// If * always set the path to the wildcard parameter
			if route.Wildcard {
				ctx.params = &[]string{"*"}
				ctx.values = make([]string, 1)
				ctx.values[0] = path
			}
			found = true
			// Set route pointer if user wants to call .Route()
			ctx.route = route
			// Execute handler with context
			if err := route.Handler(ctx); err != nil {

			}
			// if next is not set, leave loop and release ctx
			if !ctx.next {
				break
			}
			// set next to false for next iteration
			ctx.next = false
			// continue to go to the next route
			continue
		}

		// If route is Use() and path starts with route.path
		// aka strings.HasPrefix(route.path, path)
		if route.Midware && strings.HasPrefix(path, route.Path) {
			found = true
			ctx.route = route
			if err := route.Handler(ctx); err != nil {

			}
			if !ctx.next {
				break
			}
			ctx.next = false
			continue
		}

		// Skip route if regex does not exist
		if route.Regex == nil {
			continue
		}

		// Skip route if regex does not match
		if !route.Regex.MatchString(path) {
			continue
		}

		// If we have parameters, lets find the matches
		if len(route.Params) > 0 {
			matches := route.Regex.FindAllStringSubmatch(path, -1)
			// If we have matches, add params and values to context
			if len(matches) > 0 && len(matches[0]) > 1 {
				ctx.params = &route.Params
				// ctx.values = make([]string, len(*ctx.params))
				ctx.values = matches[0][1:len(matches[0])]
			}
		}

		found = true

		// Set route pointer if user wants to call .Route()
		ctx.route = route

		// Execute handler with context
		if err := route.Handler(ctx); err != nil {

		}

		// if next is not set, leave loop and release ctx
		if !ctx.next {
			break
		}

		// set next to false for next iteration
		ctx.next = false
	}

	// No routes found
	if !found {
		// Custom 404 handler?
		ctx.Status(404).Send("Not Found")
	}

	// release context back into sync pool
	releaseCtx(ctx)
}
