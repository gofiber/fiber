// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

// Router defines all router handle interface, including app and group router.
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

	Route(prefix string, fn func(router Router), name ...string) Router

	Mount(prefix string, fiber *App) Router

	Name(name string) Router
}

// Route is a struct that holds all metadata for each registered handler.
type Route struct {
	// Data for routing
	pos         uint32      // Position in stack -> important for the sort of the matched routes
	use         bool        // USE matches path prefixes
	star        bool        // Path equals '*'
	root        bool        // Path equals '/'
	path        string      // Prettified path
	routeParser routeParser // Parameter parser

	// Public fields
	Method   string    `json:"method"` // HTTP method
	Name     string    `json:"name"`   // Route's name
	Path     string    `json:"path"`   // Original registered route path
	Params   []string  `json:"params"` // Case sensitive param keys
	Handlers []Handler `json:"-"`      // Ctx handlers
}

func (r *Route) match(detectionPath, path string, params *[maxParams]string) (match bool) {
	// root detectionPath check
	if r.root && detectionPath == "/" {
		return true
		// '*' wildcard matches any detectionPath
	} else if r.star {
		if len(path) > 1 {
			params[0] = path[1:]
		} else {
			params[0] = ""
		}
		return true
	}
	// Does this route have parameters
	if len(r.Params) > 0 {
		// Match params
		if match := r.routeParser.getMatch(detectionPath, path, params, r.use); match {
			// Get params from the path detectionPath
			return match
		}
	}
	// Is this route a Middleware?
	if r.use {
		// Single slash will match or detectionPath prefix
		if r.root || strings.HasPrefix(detectionPath, r.path) {
			return true
		}
		// Check for a simple detectionPath match
	} else if len(r.path) == len(detectionPath) && r.path == detectionPath {
		return true
	}
	// No match
	return false
}

func (app *App) next(c *Ctx) (match bool, err error) {
	// Get stack length
	tree, ok := app.treeStack[c.methodINT][c.treePath]
	if !ok {
		tree = app.treeStack[c.methodINT][""]
	}
	lenr := len(tree) - 1

	// Loop over the route stack starting from previous index
	for c.indexRoute < lenr {
		// Increment route index
		c.indexRoute++

		// Get *Route
		route := tree[c.indexRoute]

		// Check if it matches the request path
		match = route.match(c.detectionPath, c.path, &c.values)

		// No match, next route
		if !match {
			continue
		}
		// Pass route reference and param values
		c.route = route

		// Non use handler matched
		if !c.matched && !route.use {
			c.matched = true
		}

		// Execute first handler of route
		c.indexHandler = 0
		err = route.Handlers[0](c)
		return match, err // Stop scanning the stack
	}

	// If c.Next() does not match, return 404
	err = NewError(StatusNotFound, "Cannot "+c.method+" "+c.pathOriginal)

	// If no match, scan stack again if other methods match the request
	// Moved from app.handler because middleware may break the route chain
	if !c.matched && methodExist(c) {
		err = ErrMethodNotAllowed
	}
	return
}

func (app *App) handler(rctx *fasthttp.RequestCtx) {
	// Acquire Ctx with fasthttp request from pool
	c := app.AcquireCtx(rctx)

	// handle invalid http method directly
	if c.methodINT == -1 {
		_ = c.Status(StatusBadRequest).SendString("Invalid http method")
		app.ReleaseCtx(c)
		return
	}

	// Find match in stack
	match, err := app.next(c)
	if err != nil {
		if catch := c.app.ErrorHandler(c, err); catch != nil {
			_ = c.SendStatus(StatusInternalServerError)
		}
	}
	// Generate ETag if enabled
	if match && app.config.ETag {
		setETag(c, false)
	}

	// Release Ctx
	app.ReleaseCtx(c)
}

func (app *App) addPrefixToRoute(prefix string, route *Route) *Route {
	prefixedPath := getGroupPath(prefix, route.Path)
	prettyPath := prefixedPath
	// Case sensitive routing, all to lowercase
	if !app.config.CaseSensitive {
		prettyPath = utils.ToLower(prettyPath)
	}
	// Strict routing, remove trailing slashes
	if !app.config.StrictRouting && len(prettyPath) > 1 {
		prettyPath = utils.TrimRight(prettyPath, '/')
	}

	route.Path = prefixedPath
	route.path = RemoveEscapeChar(prettyPath)
	route.routeParser = parseRoute(prettyPath)
	route.root = false
	route.star = false

	return route
}

func (app *App) copyRoute(route *Route) *Route {
	return &Route{
		// Router booleans
		use:  route.use,
		star: route.star,
		root: route.root,

		// Path data
		path:        route.path,
		routeParser: route.routeParser,
		Params:      route.Params,

		// Public data
		Path:     route.Path,
		Method:   route.Method,
		Handlers: route.Handlers,
	}
}

func (app *App) register(method, pathRaw string, handlers ...Handler) Router {
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
	if !app.config.CaseSensitive {
		pathPretty = utils.ToLower(pathPretty)
	}
	// Strict routing, remove trailing slashes
	if !app.config.StrictRouting && len(pathPretty) > 1 {
		pathPretty = utils.TrimRight(pathPretty, '/')
	}
	// Is layer a middleware?
	isUse := method == methodUse
	// Is path a direct wildcard?
	isStar := pathPretty == "/*"
	// Is path a root slash?
	isRoot := pathPretty == "/"
	// Parse path parameters
	parsedRaw := parseRoute(pathRaw)
	parsedPretty := parseRoute(pathPretty)

	// Create route metadata without pointer
	route := Route{
		// Router booleans
		use:  isUse,
		star: isStar,
		root: isRoot,

		// Path data
		path:        RemoveEscapeChar(pathPretty),
		routeParser: parsedPretty,
		Params:      parsedRaw.params,

		// Public data
		Path:     pathRaw,
		Method:   method,
		Handlers: handlers,
	}
	// Increment global handler count
	atomic.AddUint32(&app.handlersCount, uint32(len(handlers)))

	// Middleware route matches all HTTP methods
	if isUse {
		// Add route to all HTTP methods stack
		for _, m := range intMethod {
			// Create a route copy to avoid duplicates during compression
			r := route
			app.addRoute(m, &r)
		}
	} else {
		// Add route to stack
		app.addRoute(method, &route)
	}
	return app
}

func (app *App) registerStatic(prefix, root string, config ...Static) Router {
	// For security we want to restrict to the current work directory.
	if root == "" {
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
	if !app.config.CaseSensitive {
		prefix = utils.ToLower(prefix)
	}
	// Strip trailing slashes from the root path
	if len(root) > 0 && root[len(root)-1] == '/' {
		root = root[:len(root)-1]
	}
	// Is prefix a direct wildcard?
	isStar := prefix == "/*"
	// Is prefix a root slash?
	isRoot := prefix == "/"
	// Is prefix a partial wildcard?
	if strings.Contains(prefix, "*") {
		// /john* -> /john
		isStar = true
		prefix = strings.Split(prefix, "*")[0]
		// Fix this later
	}
	prefixLen := len(prefix)
	if prefixLen > 1 && prefix[prefixLen-1:] == "/" {
		// /john/ -> /john
		prefixLen--
		prefix = prefix[:prefixLen]
	}
	// Fileserver settings
	fs := &fasthttp.FS{
		Root:                 root,
		AllowEmptyRoot:       true,
		GenerateIndexPages:   false,
		AcceptByteRange:      false,
		Compress:             false,
		CompressedFileSuffix: app.config.CompressedFileSuffix,
		CacheDuration:        10 * time.Second,
		IndexNames:           []string{"index.html"},
		PathRewrite: func(fctx *fasthttp.RequestCtx) []byte {
			path := fctx.Path()
			if len(path) >= prefixLen {
				if isStar && app.getString(path[0:prefixLen]) == prefix {
					path = append(path[0:0], '/')
				} else {
					path = path[prefixLen:]
					if len(path) == 0 || path[len(path)-1] != '/' {
						path = append(path, '/')
					}
				}
			}
			if len(path) > 0 && path[0] != '/' {
				path = append([]byte("/"), path...)
			}
			return path
		},
		PathNotFound: func(fctx *fasthttp.RequestCtx) {
			fctx.Response.SetStatusCode(StatusNotFound)
		},
	}

	// Set config if provided
	var cacheControlValue string
	if len(config) > 0 {
		maxAge := config[0].MaxAge
		if maxAge > 0 {
			cacheControlValue = "public, max-age=" + strconv.Itoa(maxAge)
		}
		fs.CacheDuration = config[0].CacheDuration
		fs.Compress = config[0].Compress
		fs.AcceptByteRange = config[0].ByteRange
		fs.GenerateIndexPages = config[0].Browse
		if config[0].Index != "" {
			fs.IndexNames = []string{config[0].Index}
		}
	}
	fileHandler := fs.NewRequestHandler()
	handler := func(c *Ctx) error {
		// Don't execute middleware if Next returns true
		if len(config) != 0 && config[0].Next != nil && config[0].Next(c) {
			return c.Next()
		}
		// Serve file
		fileHandler(c.fasthttp)
		// Sets the response Content-Disposition header to attachment if the Download option is true
		if len(config) > 0 && config[0].Download {
			c.Attachment()
		}
		// Return request if found and not forbidden
		status := c.fasthttp.Response.StatusCode()
		if status != StatusNotFound && status != StatusForbidden {
			if len(cacheControlValue) > 0 {
				c.fasthttp.Response.Header.Set(HeaderCacheControl, cacheControlValue)
			}
			return nil
		}
		// Reset response to default
		c.fasthttp.SetContentType("") // Issue #420
		c.fasthttp.Response.SetStatusCode(StatusOK)
		c.fasthttp.Response.SetBodyString("")
		// Next middleware
		return c.Next()
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
	atomic.AddUint32(&app.handlersCount, 1)
	// Add route to stack
	app.addRoute(MethodGet, &route)
	// Add HEAD route
	app.addRoute(MethodHead, &route)
	return app
}

func (app *App) addRoute(method string, route *Route) {
	// Get unique HTTP method identifier
	m := methodInt(method)

	// prevent identically route registration
	l := len(app.stack[m])
	if l > 0 && app.stack[m][l-1].Path == route.Path && route.use == app.stack[m][l-1].use {
		preRoute := app.stack[m][l-1]
		preRoute.Handlers = append(preRoute.Handlers, route.Handlers...)
	} else {
		// Increment global route position
		route.pos = atomic.AddUint32(&app.routesCount, 1)
		route.Method = method
		// Add route to the stack
		app.stack[m] = append(app.stack[m], route)
		app.routesRefreshed = true
	}

	app.mutex.Lock()
	app.latestRoute = route
	if err := app.hooks.executeOnRouteHooks(*route); err != nil {
		panic(err)
	}
	app.mutex.Unlock()
}

// buildTree build the prefix tree from the previously registered routes
func (app *App) buildTree() *App {
	if !app.routesRefreshed {
		return app
	}
	// loop all the methods and stacks and create the prefix tree
	for m := range intMethod {
		tsMap := make(map[string][]*Route)
		for _, route := range app.stack[m] {
			treePath := ""
			if len(route.routeParser.segs) > 0 && len(route.routeParser.segs[0].Const) >= 3 {
				treePath = route.routeParser.segs[0].Const[:3]
			}
			// create tree stack
			tsMap[treePath] = append(tsMap[treePath], route)
		}
		app.treeStack[m] = tsMap
	}
	// loop the methods and tree stacks and add global stack and sort everything
	for m := range intMethod {
		tsMap := app.treeStack[m]
		for treePart := range tsMap {
			if treePart != "" {
				// merge global tree routes in current tree stack
				tsMap[treePart] = uniqueRouteStack(append(tsMap[treePart], tsMap[""]...))
			}
			// sort tree slices with the positions
			slc := tsMap[treePart]
			sort.Slice(slc, func(i, j int) bool { return slc[i].pos < slc[j].pos })
		}
	}
	app.routesRefreshed = false

	return app
}
