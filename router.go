// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"fmt"
	"html"
	"sync/atomic"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// Router defines all router handle interface, including app and group router.
type Router[TCtx CtxGeneric[TCtx]] interface {
	Use(args ...any) Router[TCtx]

	Get(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx]
	Head(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx]
	Post(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx]
	Put(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx]
	Delete(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx]
	Connect(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx]
	Options(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx]
	Trace(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx]
	Patch(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx]

	Add(methods []string, path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx]
	All(path string, handler Handler[TCtx], handlers ...Handler[TCtx]) Router[TCtx]

	Group(prefix string, handlers ...Handler[TCtx]) Router[TCtx]

	Route(path string) Register[TCtx]

	Name(name string) Router[TCtx]
}

// Route is a struct that holds all metadata for each registered handler.
type Route[TCtx CtxGeneric[TCtx]] struct {
	// ### important: always keep in sync with the copy method "app.copyRoute" and all creations of Route struct ###
	group *Group[TCtx] // Group instance. used for routes in groups

	path string // Prettified path

	// Public fields
	Method string `json:"method"` // HTTP method
	Name   string `json:"name"`   // Route's name
	//nolint:revive // Having both a Path (uppercase) and a path (lowercase) is fine
	Path        string          `json:"path"`   // Original registered route path
	Params      []string        `json:"params"` // Case-sensitive param keys
	Handlers    []Handler[TCtx] `json:"-"`      // Ctx handlers
	routeParser routeParser     // Parameter parser
	// Data for routing
	use   bool // USE matches path prefixes
	mount bool // Indicated a mounted app on a specific route
	star  bool // Path equals '*'
	root  bool // Path equals '/'
}

func (r *Route[TCtx]) match(detectionPath, path string, params *[maxParams]string) bool {
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

func (app *App[TCtx]) next(c TCtx) (bool, error) { //nolint:unparam // bool param might be useful for testing
	// Get stack length
	tree, ok := app.treeStack[c.getMethodInt()][c.getTreePathHash()]
	if !ok {
		tree = app.treeStack[c.getMethodInt()][0]
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
	err := NewError(StatusNotFound, "Cannot "+c.Method()+" "+html.EscapeString(c.getPathOriginal()))

	// If no match, scan stack again if other methods match the request
	// Moved from app.handler because middleware may break the route chain
	if !c.getMatched() && app.methodExistCustom(c) {
		err = ErrMethodNotAllowed
	}
	return false, err
}

func (app *App[TCtx]) requestHandler(rctx *fasthttp.RequestCtx) {
	// Acquire DefaultCtx from the pool
	ctx := app.AcquireCtx(rctx)

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
	_, err := app.next(ctx)
	if err != nil {
		if catch := app.ErrorHandler(ctx, err); catch != nil {
			_ = ctx.SendStatus(StatusInternalServerError) //nolint:errcheck // Always return nil
		}
		// TODO: Do we need to return here?
	}
}

func (app *App[TCtx]) addPrefixToRoute(prefix string, route *Route[TCtx]) *Route[TCtx] {
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

func (*App[TCtx]) copyRoute(route *Route[TCtx]) *Route[TCtx] {
	return &Route[TCtx]{
		// Router booleans
		use:   route.use,
		mount: route.mount,
		star:  route.star,
		root:  route.root,

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

func (app *App[TCtx]) register(methods []string, pathRaw string, group *Group[TCtx], handlers ...Handler[TCtx]) {
	// A regular route requires at least one ctx handler
	if len(handlers) == 0 && group == nil {
		panic(fmt.Sprintf("missing handler/middleware in route: %s\n", pathRaw))
	}
	// No nil handlers allowed
	for _, h := range handlers {
		if nil == h {
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

		route := Route[TCtx]{
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

func (app *App[TCtx]) addRoute(method string, route *Route[TCtx]) {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	// Get unique HTTP method identifier
	m := app.methodInt(method)

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
		if err := app.hooks.executeOnRouteHooks(*route); err != nil {
			panic(err)
		}
	}
}

// RebuildTree BuildTree rebuilds the prefix tree from the previously registered routes.
// This method is useful when you want to register routes dynamically after the app has started.
// It is not recommended to use this method on production environments because rebuilding
// the tree is performance-intensive and not thread-safe in runtime. Since building the tree
// is only done in the startupProcess of the app, this method does not makes sure that the
// routeTree is being safely changed, as it would add a great deal of overhead in the request.
// Latest benchmark results showed a degradation from 82.79 ns/op to 94.48 ns/op and can be found in:
// https://github.com/gofiber/fiber/issues/2769#issuecomment-2227385283
func (app *App[TCtx]) RebuildTree() *App[TCtx] {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	return app.buildTree()
}

// buildTree build the prefix tree from the previously registered routes
func (app *App[TCtx]) buildTree() *App[TCtx] {
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
		tsMap := make(map[int][]*Route[TCtx], len(prefixSet))
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
				if routePath == 0 {
					tsMap[treePath] = append(tsMap[treePath], route)
					// otherwise only assign if this route's prefix matches the current bucket's key
				} else if routePath == treePath {
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
