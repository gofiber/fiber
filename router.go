// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"log"
	"strings"
	"time"

	utils "github.com/gofiber/utils"
	fasthttp "github.com/valyala/fasthttp"
)

// Route is a struct that holds all metadata for each registered handler
type Route struct {
	// Data for routing
	use         bool        // USE matches path prefixes
	star        bool        // Path equals '*'
	root        bool        // Path equals '/'
	path        string      // Prettified path
	routeParser routeParser // Parameter parser
	routeParams []string    // Case sensitive param keys

	// Public fields
	Path     string    // Original registered route path
	Method   string    // HTTP method
	Handlers []Handler // Ctx handlers
}

func (r *Route) match(path, original string) (match bool, values []string) {
	// root path check
	if r.root && path == "/" {
		return true, values
		// '*' wildcard matches any path
	} else if r.star {
		values := getAllocFreeParams(1)
		values[0] = original[1:]
		return true, values
	}
	// Does this route have parameters
	if len(r.routeParams) > 0 {
		// Match params
		if paramPos, match := r.routeParser.getMatch(path, r.use); match {
			// Get params from the original path
			return match, r.routeParser.paramsForPos(original, paramPos)
		}
	}
	// Is this route a Middleware?
	if r.use {
		// Single slash will match or path prefix
		if r.root || strings.HasPrefix(path, r.path) {
			return true, values
		}
		// Check for a simple path match
	} else if len(r.path) == len(path) && r.path == path {
		return true, values
	}
	// No match
	return false, values
}

func (app *App) next(ctx *Ctx) bool {
	// TODO set unique INT within handler(), not here over and over again
	method := methodINT[ctx.method]
	// Get stack length
	lenr := len(app.stack[method]) - 1
	// Loop over the route stack starting from previous index
	for ctx.indexRoute < lenr {
		// Increment route index
		ctx.indexRoute++
		// Get *Route
		route := app.stack[method][ctx.indexRoute]
		// Check if it matches the request path
		match, values := route.match(ctx.path, ctx.pathOriginal)
		// No match, next route
		if !match {
			continue
		}
		// Pass route reference and param values
		ctx.route = route
		ctx.values = values
		// Execute first handler of route
		ctx.indexHandler = 0
		route.Handlers[0](ctx)
		// Stop scanning the stack
		return true
	}
	return false
}

func (app *App) handler(rctx *fasthttp.RequestCtx) {
	// Acquire Ctx with fasthttp request from pool
	ctx := app.AcquireCtx(rctx)
	// Prettify path
	ctx.prettifyPath()
	// Find match in stack
	match := app.next(ctx)
	// Send a 404 by default if no route matched
	if !match || len(ctx.Fasthttp.Response.Body()) == 0 {
		ctx.SendStatus(404)
		ctx.SendString(fmt.Sprintf("Cannot %s %s", ctx.method, ctx.pathOriginal))
	} else if app.Settings.ETag {
		// Generate ETag if enabled
		setETag(ctx, false)
	}
	// Release Ctx
	app.ReleaseCtx(ctx)
}

func (app *App) register(method, pathRaw string, handlers ...Handler) *Route {
	// Uppercase HTTP methods
	method = utils.ToUpper(method)
	// Check if the HTTP method is valid unless it's USE
	if method != "USE" && methodINT[method] == 0 && method != MethodGet {
		log.Fatalf("Add: Invalid HTTP method %s", method)
	}
	// A route requires atleast one ctx handler
	if len(handlers) == 0 {
		log.Fatalf("Missing func(c *fiber.Ctx) handler in route: %s", pathRaw)
	}
	// Cannot have an empty path
	if pathRaw == "" {
		pathRaw = "/"
	}
	// Path always start with a '/'
	if pathRaw[0] != '/' {
		pathRaw = "/" + pathRaw
	}
	// Create a stripped path in-case sensitive / trailing slashes
	pathPretty := pathRaw
	// Case sensitive routing, all to lowercase
	if !app.Settings.CaseSensitive {
		pathPretty = utils.ToLower(pathPretty)
	}
	// Strict routing, remove trailing slashes
	if !app.Settings.StrictRouting && len(pathPretty) > 1 {
		pathPretty = utils.TrimRight(pathPretty, '/')
	}
	// Is layer a middleware?
	var isUse = method == "USE"
	// Is path a direct wildcard?
	var isStar = pathPretty == "/*"
	// Is path a root slash?
	var isRoot = pathPretty == "/"
	// Parse path parameters
	var parsedRaw = parseRoute(pathRaw)
	var parsedPretty = parseRoute(pathPretty)

	// Create route metadata
	route := &Route{
		// Router booleans
		use:  isUse,
		star: isStar,
		root: isRoot,
		// Path data
		path:        pathPretty,
		routeParser: parsedPretty,
		routeParams: parsedRaw.params,

		// Public data
		Path:     pathRaw,
		Method:   method,
		Handlers: handlers,
	}
	// Middleware route matches all HTTP methods
	if isUse {
		// Add route to all HTTP methods stack
		for m := range methodINT {
			app.addRoute(m, route)
		}
		return route
	}

	// Add route to stack
	app.addRoute(method, route)
	// Also add GET routes to HEAD stack
	if method == MethodGet {
		app.addRoute(MethodHead, route)
	}

	return route
}

func (app *App) registerStatic(prefix, root string, config ...Static) *Route {
	// For security we want to restrict to the current work directory.
	if len(root) == 0 {
		root = "."
	}
	// Cannot have an empty prefix
	if prefix == "" {
		prefix = "/"
	}
	// Prefix always start with a '/' or '*'
	if prefix[0] != '/' {
		prefix = "/" + prefix
	}
	// in case sensitive routing, all to lowercase
	if !app.Settings.CaseSensitive {
		prefix = utils.ToLower(prefix)
	}
	// Strip trailing slashes from the root path
	if len(root) > 0 && root[len(root)-1] == '/' {
		root = root[:len(root)-1]
	}
	// Is prefix a direct wildcard?
	var isStar = prefix == "/*"
	// Is prefix a root slash?
	var isRoot = prefix == "/"
	// Is prefix a partial wildcard?
	if strings.Contains(prefix, "*") {
		// /john* -> /john
		isStar = true
		prefix = strings.Split(prefix, "*")[0]
		// Fix this later
	}
	prefixLen := len(prefix)
	// Fileserver settings
	fs := &fasthttp.FS{
		Root:                 root,
		GenerateIndexPages:   false,
		AcceptByteRange:      false,
		Compress:             false,
		CompressedFileSuffix: ".fiber.gz",
		CacheDuration:        10 * time.Second,
		IndexNames:           []string{"index.html"},
		PathRewrite: func(ctx *fasthttp.RequestCtx) []byte {
			path := ctx.Path()
			if len(path) >= prefixLen {
				if isStar && getString(path[0:prefixLen]) == prefix {
					path = path[0:0]
				} else {
					path = path[prefixLen:]
				}
			}
			return append(path, '/')
		},
		PathNotFound: func(ctx *fasthttp.RequestCtx) {
			ctx.Response.SetStatusCode(404)
		},
	}
	// Set config if provided
	if len(config) > 0 {
		fs.Compress = config[0].Compress
		fs.AcceptByteRange = config[0].ByteRange
		fs.GenerateIndexPages = config[0].Browse
		if config[0].Index != "" {
			fs.IndexNames = []string{config[0].Index}
		}
	}
	fileHandler := fs.NewRequestHandler()
	handler := func(c *Ctx) {
		// Serve file
		fileHandler(c.Fasthttp)
		// Return request if found and not forbidden
		status := c.Fasthttp.Response.StatusCode()
		if status != 404 && status != 403 {
			return
		}
		// Reset response to default
		c.Fasthttp.Response.SetStatusCode(200)
		c.Fasthttp.Response.SetBodyString("")
		// Next middleware
		match := c.app.next(c)
		// If no other route is executed return 404 Not Found
		if !match {
			c.Fasthttp.Response.SetStatusCode(404)
			c.Fasthttp.Response.SetBodyString("Not Found")
		}
	}
	route := &Route{
		use:    true,
		root:   isRoot,
		Method: MethodGet,
		Path:   prefix,
	}
	route.Handlers = append(route.Handlers, handler)
	// Add route to stack
	app.addRoute(MethodGet, route)
	app.addRoute(MethodHead, route)
	return route
}
func (app *App) addRoute(method string, route *Route) {
	// Get unique HTTP method indentifier
	m := methodINT[method]
	// Add route to the stack
	app.stack[m] = append(app.stack[m], route)
}
