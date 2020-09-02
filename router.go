// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"sort"
	"strings"
	"time"

	utils "github.com/gofiber/utils"
	fasthttp "github.com/valyala/fasthttp"
)

// Router defines all router handle interface includes app and group router.
type Router interface {
	Use(args ...interface{}) Router

	Get(path string, handlers ...Handler) Router
	Head(path string, handlers ...Handler) Router
	Post(path string, handlers ...Handler) Router
	Put(path string, handlers ...Handler) Router
	Delete(path string, handlers ...Handler) Router
	Connect(path string, handlers ...Handler) Router
	Options(path string, handlers ...Handler) Router
	Trace(path string, handlers ...Handler) Router
	Patch(path string, handlers ...Handler) Router

	Add(method, path string, handlers ...Handler) Router
	Static(prefix, root string, config ...Static) Router
	All(path string, handlers ...Handler) Router

	Group(prefix string, handlers ...Handler) Router
}

// Route is a struct that holds all metadata for each registered handler
type Route struct {
	// Data for routing
	pos         int         // Position in stack -> important for the sort of the matched routes
	use         bool        // USE matches path prefixes
	star        bool        // Path equals '*'
	root        bool        // Path equals '/'
	path        string      // Prettified path
	routeParser routeParser // Parameter parser

	// Public fields
	Method   string    `json:"method"` // HTTP method
	Path     string    `json:"path"`   // Original registered route path
	Params   []string  `json:"params"` // Case sensitive param keys
	Handlers []Handler `json:"-"`      // Ctx handlers
}

func (r *Route) match(path, original string) (match bool, values []string) {
	// root path check
	if r.root && path == "/" {
		return true, values
		// '*' wildcard matches any path
	} else if r.star {
		values := getAllocFreeParams(1)
		if len(original) > 1 {
			values[0] = original[1:]
		}
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
	tree, ok := app.treeStack[ctx.methodINT][ctx.treePath]
	if !ok {
		tree = app.treeStack[ctx.methodINT][""]
	}
	lenr := len(tree) - 1
	// Loop over the route stack starting from previous index
	for ctx.indexRoute < lenr {
		// Increment route index
		ctx.indexRoute++
		// Get *Route
		route := tree[ctx.indexRoute]
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
	ctx.SendStatus(StatusNotFound)
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

	// handle invalid http method directly
	if ctx.methodINT == -1 {
		ctx.Status(StatusBadRequest).SendString("Invalid http method")
		app.ReleaseCtx(ctx)
		return
	}
	// Find match in stack
	match := app.next(ctx)
	// Generate ETag if enabled
	if match && app.Settings.ETag {
		setETag(ctx, false)
	}
	// Release Ctx
	app.ReleaseCtx(ctx)
}

func (app *App) register(method, pathRaw string, handlers ...Handler) Route {
	// Uppercase HTTP methods
	method = utils.ToUpper(method)
	// Check if the HTTP method is valid unless it's USE
	if method != methodUse && methodInt(method) == -1 {
		panic(fmt.Sprintf("add: invalid http method %s\n", method))
	}
	// A route requires atleast one ctx handler
	if len(handlers) == 0 {
		panic(fmt.Sprintf("missing handler in route: %s\n", pathRaw))
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
	var isUse = method == methodUse
	// Is path a direct wildcard?
	var isStar = pathPretty == "/*"
	// Is path a root slash?
	var isRoot = pathPretty == "/"
	// Parse path parameters
	var parsedRaw = parseRoute(pathRaw)
	var parsedPretty = parseRoute(pathPretty)

	// Create route metadata without pointer
	route := Route{
		// Router booleans
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
	// Increment global handler count
	app.mutex.Lock()
	app.handlerCount += len(handlers)
	app.mutex.Unlock()
	// Middleware route matches all HTTP methods
	if isUse {
		// Add route to all HTTP methods stack
		for _, m := range intMethod {
			// create a route copy
			r := route
			app.addRoute(m, &r)
		}
		return route
	}

	// Add route to stack
	app.addRoute(method, &route)
	return route
}

func (app *App) registerStatic(prefix, root string, config ...Static) Route {
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
			ctx.Response.SetStatusCode(StatusNotFound)
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
		if status != StatusNotFound && status != StatusForbidden {
			return
		}
		// Reset response to default
		c.Fasthttp.SetContentType("") // Issue #420
		c.Fasthttp.Response.SetStatusCode(StatusOK)
		c.Fasthttp.Response.SetBodyString("")
		// Next middleware
		c.Next()
	}

	// Create route metadata without pointer
	route := Route{
		// Router booleans
		use:  true,
		root: isRoot,
		path: prefix,
		// Public data
		Method:   MethodGet,
		Path:     prefix,
		Handlers: []Handler{handler},
	}
	// Increment global handler count
	app.mutex.Lock()
	app.handlerCount++
	app.mutex.Unlock()
	// Add route to stack
	app.addRoute(MethodGet, &route)
	// Add HEAD route
	headRoute := route
	app.addRoute(MethodHead, &headRoute)
	return route
}

func (app *App) addRoute(method string, route *Route) {
	// Get unique HTTP method indentifier
	m := methodInt(method)

	// prevent identically route registration
	l := len(app.stack[m])
	if l > 0 && app.stack[m][l-1].Path == route.Path && route.use == app.stack[m][l-1].use {
		preRoute := app.stack[m][l-1]
		preRoute.Handlers = append(preRoute.Handlers, route.Handlers...)
	} else {
		// Increment global route position
		app.mutex.Lock()
		app.routesCount++
		app.mutex.Unlock()
		route.pos = app.routesCount
		route.Method = method
		// Add route to the stack
		app.stack[m] = append(app.stack[m], route)
	}
}

// buildTree build the prefix tree from the previously registered routes
func (app *App) buildTree() *App {
	// loop all the methods and stacks and create the prefix tree
	for m := range intMethod {
		app.treeStack[m] = make(map[string][]*Route)
		for _, route := range app.stack[m] {
			treePath := ""
			if len(route.routeParser.segs) > 0 && len(route.routeParser.segs[0].Const) >= 3 {
				treePath = route.routeParser.segs[0].Const[:3]
			}
			// create tree stack
			app.treeStack[m][treePath] = append(app.treeStack[m][treePath], route)
		}
	}
	// loop the methods and tree stacks and add global stack and sort everything
	for m := range intMethod {
		for treePart := range app.treeStack[m] {
			if treePart != "" {
				// merge global tree routes in current tree stack
				app.treeStack[m][treePart] = uniqueRouteStack(append(app.treeStack[m][treePart], app.treeStack[m][""]...))
			}
			// sort tree slices with the positions
			sort.Slice(app.treeStack[m][treePart], func(i, j int) bool {
				return app.treeStack[m][treePart][i].pos < app.treeStack[m][treePart][j].pos
			})
		}
	}

	return app
}
