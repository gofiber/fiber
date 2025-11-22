// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"fmt"
	"slices"
	"sync/atomic"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// Router defines all router handle interface, including app and group router.
type Router interface {
	Use(args ...any) Router

	Get(path string, handler any, handlers ...any) Router
	Head(path string, handler any, handlers ...any) Router
	Post(path string, handler any, handlers ...any) Router
	Put(path string, handler any, handlers ...any) Router
	Delete(path string, handler any, handlers ...any) Router
	Connect(path string, handler any, handlers ...any) Router
	Options(path string, handler any, handlers ...any) Router
	Trace(path string, handler any, handlers ...any) Router
	Patch(path string, handler any, handlers ...any) Router

	Add(methods []string, path string, handler any, handlers ...any) Router
	All(path string, handler any, handlers ...any) Router

	Group(prefix string, handlers ...any) Router

	RouteChain(path string) Register
	Route(prefix string, fn func(router Router), name ...string) Router

	Name(name string) Router
}

// Route is a struct that holds all metadata for each registered handler.
type Route struct {
	// ### important: always keep in sync with the copy method "app.copyRoute" and all creations of Route struct ###
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
	use      bool // USE matches path prefixes
	mount    bool // Indicated a mounted app on a specific route
	star     bool // Path equals '*'
	root     bool // Path equals '/'
	autoHead bool // Automatically generated HEAD route
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
			if detectionPath != "" && detectionPath[0] == '/' {
				return true
			}
		} else if len(detectionPath) >= plen && detectionPath[:plen] == r.path {
			if hasPartialMatchBoundary(detectionPath, plen) {
				return true
			}
		}
	} else if len(r.path) == len(detectionPath) && detectionPath == r.path {
		// Check exact match
		return true
	}

	// No match
	return false
}

func (app *App) next(c *DefaultCtx) (bool, error) {
	methodInt := c.methodInt
	treeHash := c.treePathHash
	// Get stack length
	tree, ok := app.treeStack[methodInt][treeHash]
	if !ok {
		tree = app.treeStack[methodInt][0]
	}
	lenr := len(tree) - 1

	indexRoute := c.indexRoute

	// Loop over the route stack starting from previous index
	for indexRoute < lenr {
		// Increment route index
		indexRoute++

		// Get *Route
		route := tree[indexRoute]

		if route.mount {
			continue
		}

		// Check if it matches the request path
		if !route.match(utils.UnsafeString(c.detectionPath), utils.UnsafeString(c.path), &c.values) {
			continue
		}

		if c.skipNonUseRoutes && !route.use {
			continue
		}

		// Pass route reference and param values
		c.route = route
		// Non use handler matched
		if !route.use {
			c.matched = true
		}
		// Execute first handler of route
		if len(route.Handlers) > 0 {
			c.indexHandler = 0
			c.indexRoute = indexRoute
			return true, route.Handlers[0](c)
		}

		return true, nil // Stop scanning the stack
	}

	// If c.Next() does not match, return 404
	// If no match, scan stack again if other methods match the request
	// Moved from app.handler because middleware may break the route chain
	if c.matched {
		return false, ErrNotFound
	}

	exists := false
	methods := app.config.RequestMethods
	for i := range methods {
		// Skip original method
		if methodInt == i {
			continue
		}
		// Reset stack index
		indexRoute := -1

		tree, ok := app.treeStack[i][treeHash]
		if !ok {
			tree = app.treeStack[i][0]
		}
		// Get stack length
		lenr := len(tree) - 1
		// Loop over the route stack starting from previous index
		for indexRoute < lenr {
			// Increment route index
			indexRoute++
			// Get *Route
			route := tree[indexRoute]
			// Skip use routes
			if route.use {
				continue
			}
			// Check if it matches the request path
			// No match, next route
			if route.match(utils.UnsafeString(c.detectionPath), utils.UnsafeString(c.path), &c.values) {
				// We matched
				exists = true
				// Add method to Allow header
				c.Append(HeaderAllow, methods[i])
				// Break stack loop
				break
			}
		}
		c.indexRoute = indexRoute
	}
	if exists {
		return false, ErrMethodNotAllowed
	}
	return false, ErrNotFound
}

func (app *App) nextCustom(c CustomCtx) (bool, error) {
	methodInt := c.getMethodInt()
	treeHash := c.getTreePathHash()
	// Get stack length
	tree, ok := app.treeStack[methodInt][treeHash]
	if !ok {
		tree = app.treeStack[methodInt][0]
	}
	lenr := len(tree) - 1

	indexRoute := c.getIndexRoute()

	// Loop over the route stack starting from previous index
	for indexRoute < lenr {
		// Increment route index
		indexRoute++

		// Get *Route
		route := tree[indexRoute]

		if route.mount {
			continue
		}

		// Check if it matches the request path
		if !route.match(c.getDetectionPath(), c.Path(), c.getValues()) {
			continue
		}
		if c.getSkipNonUseRoutes() && !route.use {
			continue
		}

		// Pass route reference and param values
		c.setRoute(route)
		// Non use handler matched
		if !route.use {
			c.setMatched(true)
		}
		// Execute first handler of route
		if len(route.Handlers) > 0 {
			c.setIndexHandler(0)
			c.setIndexRoute(indexRoute)
			return true, route.Handlers[0](c)
		}
		return true, nil // Stop scanning the stack
	}

	// If c.Next() does not match, return 404
	// If no match, scan stack again if other methods match the request
	// Moved from app.handler because middleware may break the route chain
	if c.getMatched() {
		return false, ErrNotFound
	}

	exists := false
	methods := app.config.RequestMethods
	for i := range methods {
		// Skip original method
		if methodInt == i {
			continue
		}
		// Reset stack index
		indexRoute := -1

		tree, ok := app.treeStack[i][treeHash]
		if !ok {
			tree = app.treeStack[i][0]
		}
		// Get stack length
		lenr := len(tree) - 1
		// Loop over the route stack starting from previous index
		for indexRoute < lenr {
			// Increment route index
			indexRoute++
			// Get *Route
			route := tree[indexRoute]
			// Skip use routes
			if route.use {
				continue
			}
			// Check if it matches the request path
			// No match, next route
			if route.match(c.getDetectionPath(), c.Path(), c.getValues()) {
				// We matched
				exists = true
				// Add method to Allow header
				c.Append(HeaderAllow, methods[i])
				// Break stack loop
				break
			}
		}
		c.setIndexRoute(indexRoute)
	}
	if exists {
		return false, ErrMethodNotAllowed
	}
	return false, ErrNotFound
}

func (app *App) requestHandler(rctx *fasthttp.RequestCtx) {
	// Acquire context from the pool
	ctx := app.AcquireCtx(rctx)
	defer app.ReleaseCtx(ctx)

	var err error
	// Attempt to match a route and execute the chain
	if d, isDefault := ctx.(*DefaultCtx); isDefault {
		// Check if the HTTP method is valid
		if d.methodInt == -1 {
			_ = d.SendStatus(StatusNotImplemented) //nolint:errcheck // Always return nil
			return
		}

		// Optional: Check flash messages
		rawHeaders := d.Request().Header.RawHeaders()
		if len(rawHeaders) > 0 && bytes.Contains(rawHeaders, []byte(FlashCookieName)) {
			d.Redirect().parseAndClearFlashMessages()
		}
		_, err = app.next(d)
	} else {
		// Check if the HTTP method is valid
		if ctx.getMethodInt() == -1 {
			_ = ctx.SendStatus(StatusNotImplemented) //nolint:errcheck // Always return nil
			return
		}

		// Optional: Check flash messages
		rawHeaders := ctx.Request().Header.RawHeaders()
		if len(rawHeaders) > 0 && bytes.Contains(rawHeaders, []byte(FlashCookieName)) {
			ctx.Redirect().parseAndClearFlashMessages()
		}
		_, err = app.nextCustom(ctx)
	}
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
		use:      route.use,
		mount:    route.mount,
		star:     route.star,
		root:     route.root,
		autoHead: route.autoHead,

		// Path data
		path:        route.path,
		routeParser: route.routeParser,

		// Public data
		Path:     route.Path,
		Params:   route.Params,
		Name:     route.Name,
		Method:   route.Method,
		Handlers: route.Handlers,
	}
}

func (app *App) normalizePath(path string) string {
	if path == "" {
		path = "/"
	}
	if path[0] != '/' {
		path = "/" + path
	}
	if !app.config.CaseSensitive {
		path = utils.ToLower(path)
	}
	if !app.config.StrictRouting && len(path) > 1 {
		path = utils.TrimRight(path, '/')
	}
	return RemoveEscapeChar(path)
}

// RemoveRoute is used to remove a route from the stack by path.
// If no methods are specified, it will remove the route for all methods defined in the app.
// You should call RebuildTree after using this to ensure consistency of the tree.
func (app *App) RemoveRoute(path string, methods ...string) {
	// Normalize same as register uses
	norm := app.normalizePath(path)

	pathMatchFunc := func(r *Route) bool {
		return r.path == norm // compare private normalized path
	}
	app.deleteRoute(methods, pathMatchFunc)
}

// RemoveRouteByName is used to remove a route from the stack by name.
// If no methods are specified, it will remove the route for all methods defined in the app.
// You should call RebuildTree after using this to ensure consistency of the tree.
func (app *App) RemoveRouteByName(name string, methods ...string) {
	matchFunc := func(r *Route) bool { return r.Name == name }
	app.deleteRoute(methods, matchFunc)
}

// RemoveRouteFunc is used to remove a route from the stack by a custom match function.
// If no methods are specified, it will remove the route for all methods defined in the app.
// You should call RebuildTree after using this to ensure consistency of the tree.
// Note: The route.Path is original path, not the normalized path.
func (app *App) RemoveRouteFunc(matchFunc func(r *Route) bool, methods ...string) {
	app.deleteRoute(methods, matchFunc)
}

func (app *App) deleteRoute(methods []string, matchFunc func(r *Route) bool) {
	if len(methods) == 0 {
		methods = app.config.RequestMethods
	}

	app.mutex.Lock()
	defer app.mutex.Unlock()

	removedUseRoutes := make(map[string]struct{})

	for _, method := range methods {
		// Uppercase HTTP methods
		method = utils.ToUpper(method)

		// Get unique HTTP method identifier
		m := app.methodInt(method)
		if m == -1 {
			continue // Skip invalid HTTP methods
		}

		for i := len(app.stack[m]) - 1; i >= 0; i-- {
			route := app.stack[m][i]
			if !matchFunc(route) {
				continue // Skip if route does not match
			}

			app.stack[m] = append(app.stack[m][:i], app.stack[m][i+1:]...)
			app.routesRefreshed = true

			// Decrement global handler count. In middleware routes, only decrement once
			if _, ok := removedUseRoutes[route.path]; (route.use && slices.Equal(methods, app.config.RequestMethods) && !ok) || !route.use {
				if route.use {
					removedUseRoutes[route.path] = struct{}{}
				}

				atomic.AddUint32(&app.handlersCount, ^uint32(len(route.Handlers)-1)) //nolint:gosec // Not a concern
			}

			if method == MethodGet && !route.use && !route.mount {
				app.pruneAutoHeadRouteLocked(route.path)
			}
		}
	}
}

// pruneAutoHeadRouteLocked removes an automatically generated HEAD route so a
// later explicit registration can take its place without duplicating handler
// chains. The caller must already hold app.mutex.
func (app *App) pruneAutoHeadRouteLocked(path string) {
	headIndex := app.methodInt(MethodHead)
	if headIndex == -1 {
		return
	}

	norm := app.normalizePath(path)

	headStack := app.stack[headIndex]
	for i := len(headStack) - 1; i >= 0; i-- {
		headRoute := headStack[i]
		if headRoute.path != norm || headRoute.mount || headRoute.use || !headRoute.autoHead {
			continue
		}

		app.stack[headIndex] = append(headStack[:i], headStack[i+1:]...)
		app.routesRefreshed = true
		atomic.AddUint32(&app.handlersCount, ^uint32(len(headRoute.Handlers)-1)) //nolint:gosec // Not a concern
		return
	}
}

func (app *App) register(methods []string, pathRaw string, group *Group, handlers ...Handler) {
	// A regular route requires at least one ctx handler
	if len(handlers) == 0 && group == nil {
		panic(fmt.Sprintf("missing handler/middleware in route: %s\n", pathRaw))
	}
	// No nil handlers allowed
	for _, h := range handlers {
		if h == nil {
			panic(fmt.Sprintf("nil handler in route: %s\n", pathRaw))
		}
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

	isMount := group != nil && group.app != app

	for _, method := range methods {
		method = utils.ToUpper(method)
		if method != methodUse && app.methodInt(method) == -1 {
			panic(fmt.Sprintf("add: invalid http method %s\n", method))
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
				app.addRoute(m, &r)
			}
		} else {
			// Add route to stack
			app.addRoute(method, &route)
		}
	}
}

func (app *App) addRoute(method string, route *Route) {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	// Get unique HTTP method identifier
	m := app.methodInt(method)

	if method == MethodHead && !route.mount && !route.use {
		app.pruneAutoHeadRouteLocked(route.path)
	}

	// prevent identically route registration
	l := len(app.stack[m])
	if l > 0 && app.stack[m][l-1].Path == route.Path && route.use == app.stack[m][l-1].use && !route.mount && !app.stack[m][l-1].mount {
		preRoute := app.stack[m][l-1]
		preRoute.Handlers = append(preRoute.Handlers, route.Handlers...)
	} else {
		route.Method = method
		// Add route to the stack
		app.stack[m] = append(app.stack[m], route)
		app.routesRefreshed = true
	}

	// Execute onRoute hooks & change latestRoute if not adding mounted route
	if !route.mount {
		app.latestRoute = route
		if err := app.hooks.executeOnRouteHooks(route); err != nil {
			panic(err)
		}
	}
}

func (app *App) ensureAutoHeadRoutes() {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	app.ensureAutoHeadRoutesLocked()
}

func (app *App) ensureAutoHeadRoutesLocked() {
	if app.config.DisableHeadAutoRegister {
		return
	}

	headIndex := app.methodInt(MethodHead)
	getIndex := app.methodInt(MethodGet)
	if headIndex == -1 || getIndex == -1 {
		return
	}

	headStack := app.stack[headIndex]
	existing := make(map[string]struct{}, len(headStack))
	for _, route := range headStack {
		if route.mount || route.use {
			continue
		}
		existing[route.path] = struct{}{}
	}

	if len(app.stack[getIndex]) == 0 {
		return
	}

	var added bool

	for _, route := range app.stack[getIndex] {
		if route.mount || route.use {
			continue
		}
		if _, ok := existing[route.path]; ok {
			continue
		}

		headRoute := app.copyRoute(route)
		headRoute.group = route.group
		headRoute.Method = MethodHead
		headRoute.autoHead = true
		// Fasthttp automatically omits response bodies when transmitting
		// HEAD responses, so the copied GET handler stack can execute
		// unchanged while still producing an empty body on the wire.

		headStack = append(headStack, headRoute)
		existing[route.path] = struct{}{}
		app.routesRefreshed = true
		added = true

		atomic.AddUint32(&app.handlersCount, uint32(len(headRoute.Handlers))) //nolint:gosec // Not a concern

		app.latestRoute = headRoute
		if err := app.hooks.executeOnRouteHooks(headRoute); err != nil {
			panic(err)
		}
	}

	if added {
		app.stack[headIndex] = headStack
	}
}

// RebuildTree rebuilds the prefix tree from the previously registered routes.
// This method is useful when you want to register routes dynamically after the app has started.
// It is not recommended to use this method on production environments because rebuilding
// the tree is performance-intensive and not thread-safe in runtime. Since building the tree
// is only done in the startupProcess of the app, this method does not make sure that the
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
	// If routes haven't been refreshed, nothing to do
	if !app.routesRefreshed {
		return app
	}

	// 1) First loop: determine all possible 3-char prefixes ("treePaths") for each method
	for method := range app.config.RequestMethods {
		prefixSet := map[int]struct{}{
			0: {},
		}
		for _, route := range app.stack[method] {
			if len(route.routeParser.segs) > 0 && len(route.routeParser.segs[0].Const) >= maxDetectionPaths {
				prefix := int(route.routeParser.segs[0].Const[0])<<16 |
					int(route.routeParser.segs[0].Const[1])<<8 |
					int(route.routeParser.segs[0].Const[2])
				prefixSet[prefix] = struct{}{}
			}
		}
		tsMap := make(map[int][]*Route, len(prefixSet))
		for prefix := range prefixSet {
			tsMap[prefix] = nil
		}
		app.treeStack[method] = tsMap
	}

	// 2) Second loop: for each method and each discovered treePath, assign matching routes
	for method := range app.config.RequestMethods {
		// get the map of buckets for this method
		tsMap := app.treeStack[method]

		// for every treePath key (including the empty one)
		for treePath := range tsMap {
			// iterate all routes of this method
			for _, route := range app.stack[method] {
				// compute this route's own prefix ("" or first 3 chars)
				routePath := 0
				if len(route.routeParser.segs) > 0 && len(route.routeParser.segs[0].Const) >= 3 {
					routePath = int(route.routeParser.segs[0].Const[0])<<16 |
						int(route.routeParser.segs[0].Const[1])<<8 |
						int(route.routeParser.segs[0].Const[2])
				}

				// if it's a global route, assign to every bucket
				// If the route path is 0 (global route) or matches the current tree path,
				// append this route to the current bucket
				if routePath == 0 || routePath == treePath {
					tsMap[treePath] = append(tsMap[treePath], route)
				}
			}

			// after collecting, dedupe the bucket if it's not the global one
			tsMap[treePath] = uniqueRouteStack(tsMap[treePath])
		}
	}

	// reset the flag and return
	app.routesRefreshed = false
	return app
}
