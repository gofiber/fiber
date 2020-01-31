// 🔌 Fiber is an Expressjs inspired web framework build on 🚀 Fasthttp.
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 https://github.com/gofiber/fiber

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @dgrr, @erikdubbelboer, @savsgio, @julienschmidt

package fiber

import (
	"regexp"
	"strings"

	"github.com/valyala/fasthttp"
)

type route struct {
	// HTTP method in uppercase, can be a * for Use() & All()
	Method string
	// Stores the orignal path
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
	Handler func(*Ctx)
}

// Function to add a route correctly
func (r *Fiber) register(method string, args ...interface{}) {
	// Set if method is Use() midware
	var midware = method == "MIDWARE"
	// Match any method
	if method == "ALL" || midware {
		method = "*"
	}
	// Prepare possible variables
	var path string        // We could have a path/prefix
	var handler func(*Ctx) // We could have a ctx handler
	// Only 1 argument, so no path/prefix
	if len(args) == 1 {
		handler = args[0].(func(*Ctx))
	} else if len(args) > 1 {
		path = args[0].(string)
		handler = args[1].(func(*Ctx))
		if path[0] != '/' && path[0] != '*' {
			panic("Invalid path, must begin with slash '/' or wildcard '*'")
		}
	}
	if midware && strings.Contains(path, "/:") {
		panic("You cannot use :params in Use()")
	}
	// If Use() path == "/", match anything aka *
	if midware && path == "/" {
		path = "*"
	}
	// If the route needs to match any path
	if path == "" || path == "*" || path == "/*" {
		r.routes = append(r.routes, &route{method, path, midware, true, nil, nil, handler})
		return
	}
	// Get params from path
	params := getParams(path)
	// If path has no params (simple path), we dont need regex (also for use())
	if midware || len(params) == 0 {
		r.routes = append(r.routes, &route{method, path, midware, false, nil, nil, handler})
		return
	}

	// We have parametes, so we need to compile regix from the path
	regex, err := getRegex(path)
	if err != nil {
		panic("Invalid url pattern: " + path)
	}
	// Add regex + params to route
	r.routes = append(r.routes, &route{method, path, midware, false, regex, params, handler})
}

// then try to match a route as efficient as possible.
func (r *Fiber) handler(fctx *fasthttp.RequestCtx) {
	found := false
	// get custom context from sync pool
	ctx := acquireCtx(fctx)
	// get path and method from main context
	path := ctx.Path()
	method := ctx.Method()
	// loop trough routes
	for _, route := range r.routes {
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
				ctx.values = []string{path}
			}
			found = true
			// Set route pointer if user wants to call .Route()
			ctx.route = route
			// Execute handler with context
			route.Handler(ctx)
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
			route.Handler(ctx)
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
				ctx.values = matches[0][1:len(matches[0])]
			}
		}
		found = true
		// Set route pointer if user wants to call .Route()
		ctx.route = route
		// Execute handler with context
		route.Handler(ctx)
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
