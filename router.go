// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"log"
	"strings"
	"time"

	utils "github.com/gofiber/utils"
	fasthttp "github.com/valyala/fasthttp"
)

// Router defines all router handle interface includes app and group router.
type Router interface {
	Use(args ...interface{}) *Route

	Get(path string, handlers ...Handler) *Route
	Head(path string, handlers ...Handler) *Route
	Post(path string, handlers ...Handler) *Route
	Put(path string, handlers ...Handler) *Route
	Delete(path string, handlers ...Handler) *Route
	Connect(path string, handlers ...Handler) *Route
	Options(path string, handlers ...Handler) *Route
	Trace(path string, handlers ...Handler) *Route
	Patch(path string, handlers ...Handler) *Route

	Add(method, path string, handlers ...Handler) *Route
	Static(prefix, root string, config ...Static) *Route
	All(path string, handlers ...Handler) []*Route

	Group(prefix string, handlers ...Handler) *Group
}

// Route is a struct that holds all metadata for each registered handler
type Route struct {
	// Data for routing
	pos         int         // Position in stack
	use         bool        // USE matches path prefixes
	star        bool        // Path equals '*'
	root        bool        // Path equals '/'
	path        string      // Prettified path
	routeParser routeParser // Parameter parser

	// Public fields
	Method   string    `json:"method"` // HTTP method
	Path     string    `json:"path"`   // Original registered route path
	Params   []string  `json:"params"` // Case sensitive param keys
	Name     string    `json:"name"`   // Name of first handler used in route
	Handlers []Handler `json:"-"`      // Ctx handlers
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
	if len(r.Params) > 0 {
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
	// Get stack length
	lenr := len(app.stack[ctx.methodINT]) - 1
	// Loop over the route stack starting from previous index
	for ctx.indexRoute < lenr {
		// Increment route index
		ctx.indexRoute++
		// Get *Route
		route := app.stack[ctx.methodINT][ctx.indexRoute]
		// Check if it matches the request path
		match, values := route.match(ctx.path, ctx.pathOriginal)
		// No match, next route
		if !match {
			continue
		}
		// Pass route reference and param values
		ctx.route = route
		// Non use handler matched
		if !ctx.matched && !route.use {
			ctx.matched = true
		}

		ctx.values = values
		// Execute first handler of route
		ctx.indexHandler = 0
		route.Handlers[0](ctx)
		// Stop scanning the stack
		return true
	}
	// If c.Next() does not match, return 404
	ctx.SendStatus(404)
	ctx.SendString("Cannot " + ctx.method + " " + ctx.pathOriginal)

	// Scan stack for other methods
	// Moved from app.handler
	// It should be here,
	// because middleware may break the route chain
	if !ctx.matched {
		setMethodNotAllowed(ctx)
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
	// Generate ETag if enabled
	if match && app.Settings.ETag {
		setETag(ctx, false)
	}
	// Release Ctx
	app.ReleaseCtx(ctx)
}

func (app *App) register(method, pathRaw string, handlers ...Handler) *Route {
	// Uppercase HTTP methods
	method = utils.ToUpper(method)
	// Check if the HTTP method is valid unless it's USE
	if method != "USE" && methodInt(method) == 0 && method != MethodGet {
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

	// Increment global route position
	app.mutex.Lock()
	app.routes++
	app.mutex.Unlock()
	// Create route metadata
	route := &Route{
		// Router booleans
		pos:  app.routes,
		use:  isUse,
		star: isStar,
		root: isRoot,
		// Path data
		path:        pathPretty,
		routeParser: parsedPretty,
		Params:      parsedRaw.params,

		// Public data
		Path:     pathRaw,
		Method:   method,
		Handlers: handlers,
	}
	// Middleware route matches all HTTP methods
	if isUse {
		// Add route to all HTTP methods stack
		for _, m := range intMethod {
			app.addRoute(m, route)
		}
		return route
	}

	// Handle GET routes on HEAD requests
	if method == MethodGet {
		app.addRoute(MethodHead, route)
	}

	// Add route to stack
	app.addRoute(method, route)

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
		CompressedFileSuffix: app.Settings.CompressedFileSuffix,
		CacheDuration:        10 * time.Second,
		IndexNames:           []string{"index.html"},
		PathRewrite: func(ctx *fasthttp.RequestCtx) []byte {
			path := ctx.Path()
			if len(path) >= prefixLen {
				if isStar && getString(path[0:prefixLen]) == prefix {
					path = append(path[0:0], '/')
				} else {
					path = append(path[prefixLen:], '/')
				}
			}
			if len(path) > 0 && path[0] != '/' {
				path = append([]byte("/"), path...)
			}
			return path
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
		c.Fasthttp.SetContentType("") // Issue #420
		c.Fasthttp.Response.SetStatusCode(200)
		c.Fasthttp.Response.SetBodyString("")
		// Next middleware
		c.Next()
	}
	// Increment global route position
	app.mutex.Lock()
	app.routes++
	app.mutex.Unlock()
	route := &Route{
		pos:    app.routes,
		use:    true,
		root:   isRoot,
		path:   prefix,
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
	// Give name to route if not defined
	if route.Name == "" && len(route.Handlers) > 0 {
		route.Name = utils.FunctionName(route.Handlers[0])
	}
	// Get unique HTTP method indentifier
	m := methodInt(method)
	// Add route to the stack
	app.stack[m] = append(app.stack[m], route)
}
