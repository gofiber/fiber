// 🚀 Fiber, Express on Steriods
// 📌 Don't use in production until version 1.0.0
// 🖥 https://github.com/gofiber/fiber

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @dgrr, @erikdubbelboer, @savsgio, @julienschmidt

package fiber

import (
	"regexp"

	"github.com/valyala/fasthttp"
)

type route struct {
	// HTTP method in uppercase, can be a * for Use() & All()
	method string
	// Stores the orignal path
	path string
	// wildcard bool is for routes without a path, * and /*
	wildcard bool
	// Stores compiled regex special routes :params, *wildcards, optionals?
	regex *regexp.Regexp
	// Store params if special routes :params, *wildcards, optionals?
	params []string
	// Callback function for specific route
	handler func(*Ctx)
}

// Function to add a route correctly
func (r *Fiber) register(method string, args ...interface{}) {
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
	// If the route needs to match any path
	if path == "" || path == "*" || path == "/*" {
		r.routes = append(r.routes, &route{method, path, true, nil, nil, handler})
		return
	}
	// Get params from path
	params := getParams(path)
	// If path has no params (simple path), we dont need regex
	if len(params) == 0 {
		r.routes = append(r.routes, &route{method, path, false, nil, nil, handler})
		return
	}

	// We have parametes, so we need to compile regix from the path
	regex, err := getRegex(path)
	if err != nil {
		panic("Invalid url pattern: " + path)
	}
	// Add regex + params to route
	r.routes = append(r.routes, &route{method, path, false, regex, params, handler})
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
		if route.method != "*" && route.method != method {
			continue
		}
		// First check if we match a static path or wildcard
		if route.wildcard || (route.path == path && route.params == nil) {
			// If * always set the path to the wildcard parameter
			if route.wildcard {
				ctx.params = &[]string{"*"}
				ctx.values = []string{path}
			}
			found = true
			// Set route pointer if user wants to call .Route()
			ctx.route = route
			// Execute handler with context
			route.handler(ctx)
			// if next is not set, leave loop and release ctx
			if !ctx.next {
				break
			}
			// set next to false for next iteration
			ctx.next = false
			// continue to go to the next route
			continue
		}
		// Skip route if regex does not exist
		if route.regex == nil {
			continue
		}
		// Skip route if regex does not match
		if !route.regex.MatchString(path) {
			continue
		}
		// If we have parameters, lets find the matches
		if route.params != nil && len(route.params) > 0 {
			matches := route.regex.FindAllStringSubmatch(path, -1)
			// If we have matches, add params and values to context
			if len(matches) > 0 && len(matches[0]) > 1 {
				ctx.params = &route.params
				ctx.values = matches[0][1:len(matches[0])]
			}
		}
		found = true
		// Set route pointer if user wants to call .Route()
		ctx.route = route
		// Execute handler with context
		route.handler(ctx)
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
