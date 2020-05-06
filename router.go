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

// Route metadata
type Route struct {
	// Internal fields
	get    bool         // GET allows HEAD requests
	all    bool         // ALL allows all HTTP methods
	use    bool         // USE allows all HTTP methods and path prefixes
	star   bool         // Path equals '*' or '/*'
	root   bool         // Path equals '/'
	params bool         // Path contains params: '/:p', '/:o?' or '/*'
	parsed parsedParams // parsed contains parsed params segments

	// External fields
	Path    string     // Registered route path
	Method  string     // HTTP method
	Params  []string   // Slice containing the params names
	Handler func(*Ctx) // Ctx handler
}

func (app *App) nextRoute(ctx *Ctx) {
	// Keep track of head matches
	lenr := len(app.routes) - 1
	for ctx.index < lenr {
		ctx.index++
		route := app.routes[ctx.index]
		match, values := route.matchRoute(ctx.method, ctx.path)
		if match {
			ctx.route = route
			ctx.values = values
			route.Handler(ctx)
			// Generate ETag if enabled / found
			if app.Settings.ETag {
				setETag(ctx, false)
			}
			return
		}
	}
	// Send a 404
	if len(ctx.Fasthttp.Response.Body()) == 0 {
		ctx.SendStatus(404)
	}
}

func (r *Route) matchRoute(method, path string) (match bool, values []string) {
	// Middleware routes match all HTTP methods
	if r.use {
		// Match any path if route equals '*' or '/'
		if r.star || r.root {
			return true, values
		}
		// Middleware matches path prefixes only
		if strings.HasPrefix(path, r.Path) {
			return true, values
		}
		// Middleware routes do not support params
		return false, values
	}
	// All matches any HTTP method
	// HTTP method is equal
	// GET routes allow HEAD methods
	if r.all || r.Method == method || (r.get && len(method) == 4 && method == "HEAD") {
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
	var isGet = method == "GET"
	var isAll = method == "ALL"
	var isUse = method == "USE"
	// Middleware / All allows all HTTP methods
	if isUse || isAll {
		method = "*"
	}
	var isStar = path == "*" || path == "/*"
	// Middleware containing only a `/` equals wildcard
	if isUse && path == "/" {
		isStar = true
	}
	var isRoot = path == "/"
	var isParams = false
	// Route properties
	var isParsed = parseParams(original)
	if len(isParsed.Keys) > 0 {
		isParams = true
	}
	for i := range handlers {
		app.routes = append(app.routes, &Route{
			get:    isGet,
			all:    isAll,
			use:    isUse,
			star:   isStar,
			root:   isRoot,
			params: isParams,
			parsed: isParsed,

			Path:    path,
			Method:  method,
			Params:  isParsed.Keys,
			Handler: handlers[i],
		})
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
	app.routes = append(app.routes, &Route{
		use:    true,
		root:   isRoot,
		Method: "*",
		Path:   prefix,
		Handler: func(c *Ctx) {
			// Only handle GET & HEAD methods
			if c.method == "GET" || c.method == "HEAD" {
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
			}
			c.Next()
		},
	})
}
