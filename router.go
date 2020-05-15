// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"log"
	"strings"
	"time"

	fasthttp "github.com/valyala/fasthttp"
)

func (app *App) next(ctx *Ctx) bool {
	// TODO set unique INT within handler(), not here over and over again
	method := methodINT[ctx.method]
	// Get stack length
	lenr := len(app.stack[method]) - 1
	// Loop over the layer stack starting from previous index
	for ctx.index < lenr {
		// Increment stack index
		ctx.index++
		// Get *Route
		layer := app.stack[method][ctx.index]
		// Check if it matches the request path
		match, values := layer.match(ctx.path)
		// No match, continue
		if !match {
			continue
		}
		// Pass layer and param values to Ctx
		ctx.layer = layer
		ctx.values = values
		// Execute Ctx handler
		layer.Handler(ctx)
		// Stop looping the stack
		return true
	}
	return false
}

func (app *App) handler(rctx *fasthttp.RequestCtx) {
	// Acquire Ctx with fasthttp request from pool
	ctx := AcquireCtx(rctx)
	// Attach app poiner to access the routes
	ctx.app = app
	// Attach fasthttp RequestCtx
	ctx.Fasthttp = rctx
	// In case sensitive routing, all to lowercase
	if !app.Settings.CaseSensitive {
		ctx.path = toLower(ctx.path)
	}
	// Strict routing
	if !app.Settings.StrictRouting && len(ctx.path) > 1 && ctx.path[len(ctx.path)-1] == '/' {
		ctx.path = trimRight(ctx.path, '/')
	}
	// Find match in stack
	match := app.next(ctx)
	// Generate ETag if enabled
	if app.Settings.ETag {
		setETag(ctx, false)
	}
	// Send a 404 by default if no layer matched
	if !match {
		ctx.SendStatus(404)
	}
	// Release Ctx
	ReleaseCtx(ctx)
}

func (app *App) register(method, path string, handlers ...func(*Ctx)) *App {
	// A layer requires atleast one ctx handler
	if len(handlers) == 0 {
		log.Fatalf("Missing handler in route")
	}
	// Cannot have an empty path
	if path == "" {
		path = "/"
	}
	// Path always start with a '/'
	if path[0] != '/' {
		path = "/" + path
	}
	// Store original path to strip case sensitive params
	original := path
	// Case sensitive routing, all to lowercase
	if !app.Settings.CaseSensitive {
		path = toLower(path)
	}
	// Strict routing, remove last `/`
	if !app.Settings.StrictRouting && len(path) > 1 {
		path = trimRight(path, '/')
	}
	// Is layer a middleware?
	var isUse = method == "USE"
	// Is path a direct wildcard?
	var isStar = path == "/*"
	// Is path a root slash?
	var isRoot = path == "/"
	// Parse path parameters
	var isParsed = getParams(original)
	// Loop over handlers
	for i := range handlers {
		// Set layer metadata
		layer := &Layer{
			// Internals
			use:    isUse,
			star:   isStar,
			root:   isRoot,
			parsed: isParsed,
			// Externals
			Path:    path,
			Method:  method,
			Params:  isParsed.params,
			Handler: handlers[i],
		}
		// Middleware layer matches all HTTP methods
		if isUse {
			// Add layer to all HTTP methods stack
			for m := range methodINT {
				app.addLayer(m, layer)
			}
			// Skip to next handler
			continue
		}
		// Add layer to stack
		app.addLayer(method, layer)
		// Also add GET layer to HEAD
		if method == MethodGet {
			app.addLayer(MethodHead, layer)
		}
	}
	return app
}

func (app *App) registerStatic(prefix, root string, config ...Static) {
	// Cannot have an empty prefix
	if prefix == "" {
		prefix = "/"
	}
	// Prefix always start with a '/' or '*'
	if prefix[0] != '/' {
		prefix = "/" + prefix
	}
	// Match anything
	var wildcard = false
	if prefix == "*" || prefix == "/*" {
		wildcard = true
		prefix = "/"
	}
	// in case sensitive routing, all to lowercase
	if !app.Settings.CaseSensitive {
		prefix = toLower(prefix)
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
	layer := &Layer{
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
	// Add layer to stack
	app.addLayer(MethodGet, layer)
	app.addLayer(MethodHead, layer)
}

func (app *App) addLayer(method string, layer *Layer) {
	// Get unique HTTP method indentifier
	m := methodINT[method]
	// Add layer to the stack
	app.stack[m] = append(app.stack[m], layer)
}
