// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://fiber.wiki
// 📝 Github Repository: https://github.com/gofiber/fiber

package fiber

import (
	"log"
	"regexp"
	"strings"

	fasthttp "github.com/valyala/fasthttp"
)

// Route struct
type Route struct {
	isMiddleware bool // is middleware route
	isWebSocket  bool // is websocket route

	isStar  bool // path == '*'
	isSlash bool // path == '/'
	isRegex bool // needs regex parsing

	Method     string         // http method
	Path       string         // orginal path
	Params     []string       // path params
	Regexp     *regexp.Regexp // regexp matcher
	HandleCtx  func(*Ctx)     // ctx handler
	HandleConn func(*Conn)    // conn handler

}

func (app *App) next(ctx *Ctx) {
	for ; ctx.index < len(app.routes)-1; ctx.index++ {
		ctx.index++
		route := app.routes[ctx.index]
		if route.match(ctx.method, ctx.path) {
			route.HandleCtx(ctx)
			return
		}
	}
}

func (r *Route) match(method, path string) bool {
	// is route middleware? matches all http methods
	if r.isMiddleware {
		// '*' or '/' means its a valid match
		if r.isStar || r.isSlash {
			return true
		}
		// if midware path starts with req.path
		if strings.HasPrefix(r.Path, path) {
			return true
		}
		// middlewares dont support regex so bye!
		return false
	}
	// non-middleware route, http method must match!
	// the wildcard method is for .All() method
	if r.Method != method && r.Method[0] != '*' {
		return false
	}
	// '*' means we match anything
	if r.isStar {
		return true
	}
	// simple '/' match, why not r.Path == path?
	// because bool is faster and you avoid
	// unnecessary comparison between long paths
	if r.isSlash && path == "/" {
		return true
	}
	// does this route need regex matching?
	if r.isRegex {
		// req.path match regex pattern
		if r.Regexp.MatchString(path) {
			return true
			// Need to think about how to pass regex params to context
			// if len(r.Params) > 0 {
			// 	matches := route.Regex.FindAllStringSubmatch(path, -1)
			// 	if len(matches) > 0 && len(matches[0]) > 1 {
			// 		params := matches[0][1:len(matches[0])]
			// 	}
			// }
		}
		return false
	}
	// last thing to do is to check for a simple path match
	if r.Path == path {
		return true
	}
	// Nothing match
	return false
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
	// Set route booleans
	var isMiddleware = method == "USE"
	// Middleware / All allows all HTTP methods
	if isMiddleware || method == "ALL" {
		method = "*"
	}
	var isStar = path == "*" || path == "/*"
	// Middleware containing only a `/` equals wildcard
	if isMiddleware && path == "/" {
		isStar = true
	}
	var isSlash = path == "/"
	var isRegex = false
	// Route properties
	var Params = getParams(path)
	var Regexp *regexp.Regexp
	// Params requires regex pattern
	if len(Params) > 0 {
		regex, err := getRegex(path)
		if err != nil {
			log.Fatal("Router: Invalid path pattern: " + path)
		}
		isRegex = true
		Regexp = regex
	}
	for i := range handlers {
		app.routes = append(app.routes, &Route{
			isMiddleware: isMiddleware,
			isStar:       isStar,
			isSlash:      isSlash,
			isRegex:      isRegex,
			Method:       method,
			Path:         path,
			Regexp:       Regexp,
			HandleCtx:    handlers[i],
		})
	}
}
func (app *App) handler(fctx *fasthttp.RequestCtx) {
	// get custom context from sync pool
	ctx := acquireCtx(fctx)
	defer releaseCtx(ctx)

	ctx.method = ctx.Method()
	ctx.path = ctx.Path()
	ctx.app = app

	app.next(ctx)
}
func (app *App) registerStatic(grpPrefix string, args ...string) {
	// var prefix = "/"
	// var root = "./"
	// // enable / disable gzipping somewhere?
	// // todo v2.0.0
	// gzip := true

	// if len(args) == 1 {
	// 	root = args[0]
	// }
	// if len(args) == 2 {
	// 	prefix = args[0]
	// 	root = args[1]
	// }

	// // A non wildcard path must start with a '/'
	// if prefix != "*" && len(prefix) > 0 && prefix[0] != '/' {
	// 	prefix = "/" + prefix
	// }
	// // Prepend group prefix
	// if len(grpPrefix) > 0 {
	// 	// `/v1`+`/` => `/v1`+``
	// 	if prefix == "/" {
	// 		prefix = grpPrefix
	// 	} else {
	// 		prefix = grpPrefix + prefix
	// 	}
	// 	// Remove duplicate slashes `//`
	// 	prefix = strings.Replace(prefix, "//", "/", -1)
	// }
	// // Empty or '/*' path equals "match anything"
	// // TODO fix * for paths with grpprefix
	// if prefix == "/*" {
	// 	prefix = "*"
	// }
	// // Lets get all files from root
	// files, _, err := getFiles(root)
	// if err != nil {
	// 	log.Fatal("Static: ", err)
	// }
	// // ./static/compiled => static/compiled
	// mount := filepath.Clean(root)

	// if !app.Settings.CaseSensitive {
	// 	prefix = strings.ToLower(prefix)
	// }
	// if !app.Settings.StrictRouting && len(prefix) > 1 {
	// 	prefix = strings.TrimRight(prefix, "/")
	// }

	// // Loop over all files
	// for _, file := range files {
	// 	// Ignore the .gzipped files by fasthttp
	// 	if strings.Contains(file, ".fasthttp.gz") {
	// 		continue
	// 	}
	// 	// Time to create a fake path for the route match
	// 	// static/index.html => /index.html
	// 	path := filepath.Join(prefix, strings.Replace(file, mount, "", 1))
	// 	// for windows: static\index.html => /index.html
	// 	path = filepath.ToSlash(path)
	// 	// Store file path to use in ctx handler
	// 	filePath := file

	// 	if len(prefix) > 1 && strings.Contains(prefix, "*") {
	// 		app.routes = append(app.routes, &Route{
	// 			Method: "GET",
	// 			Path:   path,
	// 			Prefix: strings.Split(prefix, "*")[0],
	// 			HandlerCtx: func(c *Ctx) {
	// 				c.SendFile(filePath, gzip)
	// 			},
	// 		})
	// 		return
	// 	}
	// 	// If the file is an index.html, bind the prefix to index.html directly
	// 	if filepath.Base(filePath) == "index.html" || filepath.Base(filePath) == "index.htm" {
	// 		app.routes = append(app.routes, &Route{
	// 			Method: "GET",
	// 			Path:   prefix,
	// 			HandlerCtx: func(c *Ctx) {
	// 				c.SendFile(filePath, gzip)
	// 			},
	// 		})
	// 	}
	// 	if !app.Settings.CaseSensitive {
	// 		path = strings.ToLower(path)
	// 	}
	// 	if !app.Settings.StrictRouting && len(prefix) > 1 {
	// 		path = strings.TrimRight(path, "/")
	// 	}
	// 	// Add the route + SendFile(filepath) to routes
	// 	app.routes = append(app.routes, &Route{
	// 		Method: "GET",
	// 		Path:   path,
	// 		HandlerCtx: func(c *Ctx) {
	// 			c.SendFile(filePath, gzip)
	// 		},
	// 	})
	// }
}
func (app *App) registerWebSocket(method, group, path string, handle func(*Conn)) {
	// if len(path) > 0 && path[0] != '/' {
	// 	path = "/" + path
	// }
	// if len(group) > 0 {
	// 	// `/v1`+`/` => `/v1`+``
	// 	if path == "/" {
	// 		path = group
	// 	} else {
	// 		path = group + path
	// 	}
	// 	// Remove duplicate slashes `//`
	// 	path = strings.Replace(path, "//", "/", -1)
	// }
	// // Routes are case insensitive by default
	// if !app.Settings.CaseSensitive {
	// 	path = strings.ToLower(path)
	// }
	// if !app.Settings.StrictRouting && len(path) > 1 {
	// 	path = strings.TrimRight(path, "/")
	// }
	// // Get ':param' & ':optional?' & '*' from path
	// params := getParams(path)
	// if len(params) > 0 {
	// 	log.Fatal("WebSocket routes do not support path parameters: `:param, :optional?, *`")
	// }
	// app.routes = append(app.routes, &Route{
	// 	Method: method, Path: path, HandlerConn: handler,
	// })
}

// func (app *App) registerMethod(method, group, path string, handlers ...func(*Ctx)) {
// 	// No special paths for websockets
// 	if len(handlers) == 0 {
// 		log.Fatalf("Missing handler in route")
// 	}
// 	// Set variables
// 	var prefix string
// 	var middleware = method == "USE"
// 	// A non wildcard path must start with a '/'
// 	if path != "*" && len(path) > 0 && path[0] != '/' {
// 		path = "/" + path
// 	}
// 	// Prepend group prefix
// 	if len(group) > 0 {
// 		// `/v1`+`/` => `/v1`+``
// 		if path == "/" {
// 			path = group
// 		} else {
// 			path = group + path
// 		}
// 		// Remove duplicate slashes `//`
// 		path = strings.Replace(path, "//", "/", -1)
// 	}
// 	// Empty or '/*' path equals "match anything"
// 	// TODO fix * for paths with grpprefix
// 	if path == "/*" || (middleware && path == "") {
// 		path = "*"
// 	}
// 	if method == "ALL" || middleware {
// 		method = "*"
// 	}
// 	// Routes are case insensitive by default
// 	if !app.Settings.CaseSensitive {
// 		path = strings.ToLower(path)
// 	}
// 	if !app.Settings.StrictRouting && len(path) > 1 {
// 		path = strings.TrimRight(path, "/")
// 	}
// 	// If the route can match anything
// 	if path == "*" {
// 		for i := range handlers {
// 			app.routes = append(app.routes, &Route{
// 				Method: method, Path: path, HandlerCtx: handlers[i],
// 			})
// 		}
// 		return
// 	}
// 	// Get ':param' & ':optional?' & '*' from path
// 	params := getParams(path)
// 	// Enable prefix for midware
// 	if len(params) == 0 && middleware {
// 		prefix = path
// 	}

// 	// If path has no params (simple path)
// 	if len(params) == 0 {
// 		for i := range handlers {
// 			app.routes = append(app.routes, &Route{
// 				Method: method, Path: path, Prefix: prefix, HandlerCtx: handlers[i],
// 			})
// 		}
// 		return
// 	}

// 	// If path only contains 1 wildcard, we can create a prefix
// 	// If its a middleware, we also create a prefix
// 	if len(params) == 1 && params[0] == "*" {
// 		prefix = strings.Split(path, "*")[0]
// 		for i := range handlers {
// 			app.routes = append(app.routes, &Route{
// 				Method: method, Path: path, Prefix: prefix,
// 				Params: params, HandlerCtx: handlers[i],
// 			})
// 		}
// 		return
// 	}
// 	// We have an :param or :optional? and need to compile a regex struct
// 	regex, err := getRegex(path)
// 	if err != nil {
// 		log.Fatal("Router: Invalid path pattern: " + path)
// 	}
// 	// Add route with regex
// 	for i := range handlers {
// 		app.routes = append(app.routes, &Route{
// 			Method: method, Path: path, Regex: regex,
// 			Params: params, HandlerCtx: handlers[i],
// 		})
// 	}
// }
