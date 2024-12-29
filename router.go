// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"sort"
	"sync/atomic"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// Router defines all router handle interface, including app and group router.
type Router interface {
	Use(args ...any) Router

	Get(path string, handler Handler, middleware ...Handler) Router
	Head(path string, handler Handler, middleware ...Handler) Router
	Post(path string, handler Handler, middleware ...Handler) Router
	Put(path string, handler Handler, middleware ...Handler) Router
	Delete(path string, handler Handler, middleware ...Handler) Router
	Connect(path string, handler Handler, middleware ...Handler) Router
	Options(path string, handler Handler, middleware ...Handler) Router
	Trace(path string, handler Handler, middleware ...Handler) Router
	Patch(path string, handler Handler, middleware ...Handler) Router

	Add(methods []string, path string, handler Handler, middleware ...Handler) Router
	All(path string, handler Handler, middleware ...Handler) Router

	Group(prefix string, handlers ...Handler) Router

	Route(path string) Register

	Name(name string) Router
}

// Route is a struct that holds all metadata for each registered handler.
type Route struct {
	// ### important: always keep in sync with the copy method "app.copyRoute" ###
	group *Group // Group instance. used for routes in groups

	path string // Prettified path

	// Public fields
	Method string `json:"method"` // HTTP method
	Name   string `json:"name"`   // Route's name
	//nolint:revive // Having both a Path (uppercase) and a path (lowercase) is fine
	Path        string      `json:"path"`   // Original registered route path
	Params      []string    `json:"params"` // Case-sensitive param keys
	Handlers    []Handler   `json:"-"`      // Ctx handlers
	routeParser routeParser // Parameter parser
	// Data for routing
	pos   uint32 // Position in stack -> important for the sort of the matched routes
	use   bool   // USE matches path prefixes
	mount bool   // Indicated a mounted app on a specific route
	star  bool   // Path equals '*'
	root  bool   // Path equals '/'
}

func (r *Route) match(detectionPath, path string, params *[maxParams]string) bool {
	// root detectionPath check
	if r.root && len(detectionPath) == 1 && detectionPath[0] == '/' {
		return true
	}

	// '*' wildcard matches any detectionPath
	if r.star {
		if len(path) > 1 {
			params[0] = path[1:]
		} else {
			params[0] = ""
		}
		return true
	}

	// Does this route have parameters?
	if len(r.Params) > 0 {
		// Match params using precomputed routeParser
		if r.routeParser.getMatch(detectionPath, path, params, r.use) {
			return true
		}
	}

	// Middleware route?
	if r.use {
		// Single slash or prefix match
		plen := len(r.path)
		if r.root {
			// If r.root is '/', it matches everything starting at '/'
			if len(detectionPath) > 0 && detectionPath[0] == '/' {
				return true
			}
		} else if len(detectionPath) >= plen && detectionPath[:plen] == r.path {
			return true
		}
	} else if len(r.path) == len(detectionPath) && detectionPath == r.path {
		// Check exact match
		return true
	}

	// No match
	return false
}

func (app *App) nextCustom(c CustomCtx) (bool, error) { //nolint: unparam // bool param might be useful for testing
	// Get stack length
	tree, ok := app.treeStack[c.getMethodINT()][c.getTreePath()]
	if !ok {
		tree = app.treeStack[c.getMethodINT()][""]
	}
	lenr := len(tree) - 1

	// Loop over the route stack starting from previous index
	for c.getIndexRoute() < lenr {
		// Increment route index
		c.setIndexRoute(c.getIndexRoute() + 1)

		// Get *Route
		route := tree[c.getIndexRoute()]

		// Check if it matches the request path
		match := route.match(c.getDetectionPath(), c.Path(), c.getValues())

		// No match, next route
		if !match {
			continue
		}
		// Pass route reference and param values
		c.setRoute(route)

		// Non use handler matched
		if !c.getMatched() && !route.use {
			c.setMatched(true)
		}

		// Execute first handler of route
		c.setIndexHandler(0)
		err := route.Handlers[0](c)
		return match, err // Stop scanning the stack
	}

	// If c.Next() does not match, return 404
	err := NewError(StatusNotFound, "Cannot "+c.Method()+" "+c.getPathOriginal())

	// If no match, scan stack again if other methods match the request
	// Moved from app.handler because middleware may break the route chain
	if !c.getMatched() && app.methodExistCustom(c) {
		err = ErrMethodNotAllowed
	}
	return false, err
}

func (app *App) next(c *DefaultCtx) (bool, error) {
	// Get stack length
	tree, ok := app.treeStack[c.methodINT][c.treePath]
	if !ok {
		tree = app.treeStack[c.methodINT][""]
	}
	lenTree := len(tree) - 1

	// Loop over the route stack starting from previous index
	for c.indexRoute < lenTree {
		// Increment route index
		c.indexRoute++

		// Get *Route
		route := tree[c.indexRoute]

		var match bool
		var err error
		// skip for mounted apps
		if route.mount {
			continue
		}

		// Check if it matches the request path
		match = route.match(c.detectionPath, c.path, &c.values)
		if !match {
			// No match, next route
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
		if len(route.Handlers) > 0 {
			err = route.Handlers[0](c)
		}
		return match, err // Stop scanning the stack
	}

	// If c.Next() does not match, return 404
	err := NewError(StatusNotFound, "Cannot "+c.method+" "+html.EscapeString(c.pathOriginal))
	if !c.matched && app.methodExist(c) {
		// If no match, scan stack again if other methods match the request
		// Moved from app.handler because middleware may break the route chain
		err = ErrMethodNotAllowed
	}
	return false, err
}

func (app *App) defaultRequestHandler(rctx *fasthttp.RequestCtx) {
	// Acquire DefaultCtx from the pool
	ctx, ok := app.AcquireCtx(rctx).(*DefaultCtx)
	if !ok {
		panic(errors.New("requestHandler: failed to type-assert to *DefaultCtx"))
	}

	defer app.ReleaseCtx(ctx)

	// Check if the HTTP method is valid
	if ctx.methodINT == -1 {
		_ = ctx.SendStatus(StatusNotImplemented) //nolint:errcheck // Always return nil
		return
	}

	// Optional: Check flash messages
	rawHeaders := ctx.Request().Header.RawHeaders()
	if len(rawHeaders) > 0 && bytes.Contains(rawHeaders, []byte(FlashCookieName)) {
		ctx.Redirect().parseAndClearFlashMessages()
	}

	// Attempt to match a route and execute the chain
	_, err := app.next(ctx)
	if err != nil {
		if catch := ctx.App().ErrorHandler(ctx, err); catch != nil {
			_ = ctx.SendStatus(StatusInternalServerError) //nolint:errcheck // Always return nil
		}
		// TODO: Do we need to return here?
	}
}

func (app *App) customRequestHandler(rctx *fasthttp.RequestCtx) {
	// Acquire CustomCtx from the pool
	ctx, ok := app.AcquireCtx(rctx).(CustomCtx)
	if !ok {
		panic(errors.New("requestHandler: failed to type-assert to CustomCtx"))
	}

	defer app.ReleaseCtx(ctx)

	// Check if the HTTP method is valid
	if app.methodInt(ctx.Method()) == -1 {
		_ = ctx.SendStatus(StatusNotImplemented) //nolint:errcheck // Always return nil
		return
	}

	// Optional: Check flash messages
	rawHeaders := ctx.Request().Header.RawHeaders()
	if len(rawHeaders) > 0 && bytes.Contains(rawHeaders, []byte(FlashCookieName)) {
		ctx.Redirect().parseAndClearFlashMessages()
	}

	// Attempt to match a route and execute the chain
	_, err := app.nextCustom(ctx)
	if err != nil {
		if catch := ctx.App().ErrorHandler(ctx, err); catch != nil {
			_ = ctx.SendStatus(StatusInternalServerError) //nolint:errcheck // Always return nil
		}
		// TODO: Do we need to return here?
	}
}

func (app *App) addPrefixToRoute(prefix string, route *Route) *Route {
	prefixedPath := getGroupPath(prefix, route.Path)
	prettyPath := prefixedPath
	// Case-sensitive routing, all to lowercase
	if !app.config.CaseSensitive {
		prettyPath = utils.ToLower(prettyPath)
	}
	// Strict routing, remove trailing slashes
	if !app.config.StrictRouting && len(prettyPath) > 1 {
		prettyPath = utils.TrimRight(prettyPath, '/')
	}

	route.Path = prefixedPath
	route.path = RemoveEscapeChar(prettyPath)
	route.routeParser = parseRoute(prettyPath, app.customConstraints...)
	route.root = false
	route.star = false

	return route
}

func (*App) copyRoute(route *Route) *Route {
	return &Route{
		// Router booleans
		use:   route.use,
		mount: route.mount,
		star:  route.star,
		root:  route.root,

		// Path data
		path:        route.path,
		routeParser: route.routeParser,

		// misc
		pos: route.pos,

		// Public data
		Path:     route.Path,
		Params:   route.Params,
		Name:     route.Name,
		Method:   route.Method,
		Handlers: route.Handlers,
	}
}

func (app *App) register(methods []string, pathRaw string, group *Group, handler Handler, middleware ...Handler) {
	handlers := middleware
	if handler != nil {
		handlers = append(handlers, handler)
	}

	// Precompute path normalization ONCE
	if pathRaw == "" {
		pathRaw = "/"
	}
	if pathRaw[0] != '/' {
		pathRaw = "/" + pathRaw
	}
	pathPretty := pathRaw
	if !app.config.CaseSensitive {
		pathPretty = utils.ToLower(pathPretty)
	}
	if !app.config.StrictRouting && len(pathPretty) > 1 {
		pathPretty = utils.TrimRight(pathPretty, '/')
	}
	pathClean := RemoveEscapeChar(pathPretty)

	parsedRaw := parseRoute(pathRaw, app.customConstraints...)
	parsedPretty := parseRoute(pathPretty, app.customConstraints...)

	for _, method := range methods {
		method = utils.ToUpper(method)
		if method != methodUse && app.methodInt(method) == -1 {
			panic(fmt.Sprintf("add: invalid http method %s\n", method))
		}

		isMount := group != nil && group.app != app
		if len(handlers) == 0 && !isMount {
			panic(fmt.Sprintf("missing handler/middleware in route: %s\n", pathRaw))
		}

		isUse := method == methodUse
		isStar := pathClean == "/*"
		isRoot := pathClean == "/"

		route := Route{
			use:   isUse,
			mount: isMount,
			star:  isStar,
			root:  isRoot,

			path:        pathClean,
			routeParser: parsedPretty,
			Params:      parsedRaw.params,
			group:       group,

			Path:     pathRaw,
			Method:   method,
			Handlers: handlers,
		}

		// Increment global handler count
		atomic.AddUint32(&app.handlersCount, uint32(len(handlers))) //nolint:gosec // Not a concern

		// Middleware route matches all HTTP methods
		if isUse {
			// Add route to all HTTP methods stack
			for _, m := range app.config.RequestMethods {
				// Create a route copy to avoid duplicates during compression
				r := route
				app.addRoute(m, &r, isMount)
			}
		} else {
			// Add route to stack
			app.addRoute(method, &route, isMount)
		}
	}
}

func (app *App) addRoute(method string, route *Route, isMounted ...bool) {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	// Check mounted routes
	var mounted bool
	if len(isMounted) > 0 {
		mounted = isMounted[0]
	}

	// Get unique HTTP method identifier
	m := app.methodInt(method)

	// prevent identically route registration
	l := len(app.stack[m])
	if l > 0 && app.stack[m][l-1].Path == route.Path && route.use == app.stack[m][l-1].use && !route.mount && !app.stack[m][l-1].mount {
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

	// Execute onRoute hooks & change latestRoute if not adding mounted route
	if !mounted {
		app.latestRoute = route
		if err := app.hooks.executeOnRouteHooks(*route); err != nil {
			panic(err)
		}
	}
}

// BuildTree rebuilds the prefix tree from the previously registered routes.
// This method is useful when you want to register routes dynamically after the app has started.
// It is not recommended to use this method on production environments because rebuilding
// the tree is performance-intensive and not thread-safe in runtime. Since building the tree
// is only done in the startupProcess of the app, this method does not makes sure that the
// routeTree is being safely changed, as it would add a great deal of overhead in the request.
// Latest benchmark results showed a degradation from 82.79 ns/op to 94.48 ns/op and can be found in:
// https://github.com/gofiber/fiber/issues/2769#issuecomment-2227385283
func (app *App) RebuildTree() *App {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	return app.buildTree()
}

// buildTree build the prefix tree from the previously registered routes
func (app *App) buildTree() *App {
	if !app.routesRefreshed {
		return app
	}

	// loop all the methods and stacks and create the prefix tree
	for m := range app.config.RequestMethods {
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
	for m := range app.config.RequestMethods {
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
