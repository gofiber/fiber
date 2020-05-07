// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"log"
	"strings"
	"time"

	fasthttp "github.com/valyala/fasthttp"
)

// All HTTP methods
var methods = []string{"CONNECT", "PUT", "POST", "DELETE", "HEAD", "PATCH", "OPTIONS", "TRACE", "GET"}

// Route metadata
type Route struct {
	// Internal fields
	use    bool         // USE matches path prefixes
	star   bool         // Path equals '*' or '/*'
	root   bool         // Path equals '/'
	parsed parsedParams // parsed contains parsed params segments

	// External fields for ctx.Route() method
	Path    string     // Registered route path
	Method  string     // HTTP method
	Params  []string   // Slice containing the params names
	Handler func(*Ctx) // Ctx handler
}

func (app *App) nextRoute(ctx *Ctx) {
	// Get stack length
	lenr := len(app.routes[ctx.method]) - 1
	// Loop over stack starting from previous index
	for ctx.index < lenr {
		// Increment stack index
		ctx.index++
		// Get *Route
		route := app.routes[ctx.method][ctx.index]
		// Check if it matches the request path
		match, values := route.matchRoute(ctx.path)
		// No match, continue
		if !match {
			continue
		}
		// Match! Set route and param values to Ctx
		ctx.route = route
		ctx.values = values
		// Execute handler
		route.Handler(ctx)
		// Generate ETag if enabled
		if app.Settings.ETag {
			setETag(ctx, false)
		}
		return
	}
	// Send a 404 by default if no route is matched
	if len(ctx.Fasthttp.Response.Body()) == 0 {
		ctx.SendStatus(404)
	}
}

func (r *Route) matchRoute(path string) (match bool, values []string) {
	// Middleware routes allow prefix matches
	if r.use {
		// Match any path if route equals '*' or '/'
		if r.star || r.root {
			return true, values
		}
		// Middleware matches path prefix
		if strings.HasPrefix(path, r.Path) {
			return true, values
		}
		// No prefix match, and we do not allow params in app.use
		return false, values
	}
	// '*' wildcard matches any path
	if r.star {
		return true, values
	}
	// Check if a single '/' matches
	if r.root && path == "/" {
		return true, values
	}
	// Does this route have parameters
	if len(r.Params) > 0 {
		// Do we have a match?
		params, ok := r.parsed.matchParams(path)
		// We have a match!
		if ok {
			return true, params
		}
	}
	// Check for a simple path match
	if len(r.Path) == len(path) && r.Path == path {
		return true, values
	}

	// Nothing match
	return false, values
}

func (app *App) handler(fctx *fasthttp.RequestCtx) {
	// get fiber context from sync pool
	ctx := acquireCtx(fctx)
	defer releaseCtx(ctx)
	// attach app poiner and compress settings
	ctx.app = app

	// Case sensitive routing
	if !app.Settings.CaseSensitive {
		ctx.path = strings.ToLower(ctx.path)
	}
	// Strict routing
	if !app.Settings.StrictRouting && len(ctx.path) > 1 {
		ctx.path = strings.TrimRight(ctx.path, "/")
	}
	// Find route
	app.nextRoute(ctx)
}

func (app *App) registerMethod(method, path string, handlers ...func(*Ctx)) {
	// Route requires atleast one handler
	if len(handlers) == 0 {
		log.Fatalf("Missing handler in route")
	}
	// Cannot have an empty path
	if path == "" {
		path = "/"
	}
	// Path always start with a '/' or '*'
	if path[0] != '/' && path[0] != '*' {
		path = "/" + path
	}
	// Store original path to strip case sensitive params
	original := path
	// Case sensitive routing, all to lowercase
	if !app.Settings.CaseSensitive {
		path = strings.ToLower(path)
	}
	// Strict routing, remove last `/`
	if !app.Settings.StrictRouting && len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	// Set route booleans
	var isUse = method == "USE"
	// Middleware / All allows all HTTP methods
	if isUse || method == "ALL" {
		method = "*"
	}
	var isStar = path == "*" || path == "/*"
	// Middleware containing only a `/` equals wildcard
	if isUse && path == "/" {
		isStar = true
	}
	var isRoot = path == "/"
	// Route properties
	var isParsed = parseParams(original)
	for i := range handlers {
		route := &Route{
			use:    isUse,
			star:   isStar,
			root:   isRoot,
			parsed: isParsed,

			Path:    path,
			Method:  method,
			Params:  isParsed.Keys,
			Handler: handlers[i],
		}
		if method == "*" {
			// Add handler to all HTTP methods
			for i := range methods {
				app.addRoute(methods[i], route)
			}
			continue
		}
		// Add route to stack
		app.addRoute(method, route)
		// Add route to HEAD method if GET
		if method == MethodGet {
			app.addRoute(MethodHead, route)
		}

	}
}

func (app *App) registerStatic(prefix, root string, config ...Static) {
	// Cannot have an empty prefix
	if prefix == "" {
		prefix = "/"
	}
	// Prefix always start with a '/' or '*'
	if prefix[0] != '/' && prefix[0] != '*' {
		prefix = "/" + prefix
	}
	// Match anything
	var wildcard = false
	if prefix == "*" || prefix == "/*" {
		wildcard = true
		prefix = "/"
	}
	// Case sensitive routing, all to lowercase
	if !app.Settings.CaseSensitive {
		prefix = strings.ToLower(prefix)
	}
	// For security we want to restrict to the current work directory.
	if len(root) == 0 {
		root = "."
	}
	// Strip trailing slashes from the root path
	if len(root) > 0 && root[len(root)-1] == '/' {
		root = root[:len(root)-1]
	}
	// isSlash ?
	var isRoot = prefix == "/"
	if strings.Contains(prefix, "*") {
		wildcard = true
		prefix = strings.Split(prefix, "*")[0]
	}
	var stripper = len(prefix)
	if isRoot {
		stripper = 0
	}
	// Fileserver settings
	fs := &fasthttp.FS{
		Root:                 root,
		GenerateIndexPages:   false,
		AcceptByteRange:      false,
		Compress:             false,
		CompressedFileSuffix: ".fiber.gz",
		CacheDuration:        10 * time.Second,
		IndexNames:           []string{"index.html"},
		PathRewrite:          fasthttp.NewPathPrefixStripper(stripper),
		PathNotFound: func(ctx *fasthttp.RequestCtx) {
			ctx.Response.SetStatusCode(404)
			ctx.Response.SetBodyString("Not Found")
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
	route := &Route{
		use:    true,
		root:   isRoot,
		Method: "*",
		Path:   prefix,
		Handler: func(c *Ctx) {
			// Do stuff
			if wildcard {
				c.Fasthttp.Request.SetRequestURI(prefix)
			}
			// Serve file
			fileHandler(c.Fasthttp)

			// Finish request if found and not forbidden
			status := c.Fasthttp.Response.StatusCode()
			if status != 404 && status != 403 {
				return
			}
			// Reset response
			c.Fasthttp.Response.Reset()
			// Next middleware
			c.Next()
		},
	}
	// Add route to stack
	app.addRoute(MethodGet, route)
	app.addRoute(MethodHead, route)
}

func (app *App) addRoute(method string, route *Route) {
	app.routes[method] = append(app.routes[method], route)
}
