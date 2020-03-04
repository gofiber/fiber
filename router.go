// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://fiber.wiki
// 📝 Github Repository: https://github.com/gofiber/fiber

package fiber

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	websocket "github.com/fasthttp/websocket"
	fasthttp "github.com/valyala/fasthttp"
)

// Route struct
type Route struct {
	isMiddleware bool // is middleware route
	isWebSocket  bool // is websocket route

	isStar  bool // path == '*'
	isSlash bool // path == '/'
	isRegex bool // needs regex parsing

	Method string         // http method
	Path   string         // orginal path
	Params []string       // path params
	Regexp *regexp.Regexp // regexp matcher

	HandleCtx  func(*Ctx)  // ctx handler
	HandleConn func(*Conn) // conn handler

}

func (app *App) nextRoute(ctx *Ctx) {
	lenr := len(app.routes) - 1
	for ctx.index < lenr {
		ctx.index++
		route := app.routes[ctx.index]
		match, values := route.matchRoute(ctx.method, ctx.path)
		if match {
			ctx.route = route
			if !ctx.matched {
				ctx.matched = true
			}
			if len(values) > 0 {
				ctx.values = values
			}
			if route.isWebSocket {
				if err := websocketUpgrader.Upgrade(ctx.Fasthttp, func(fconn *websocket.Conn) {
					conn := acquireConn(fconn)
					defer releaseConn(conn)
					route.HandleConn(conn)
				}); err != nil { // Upgrading failed
					ctx.SendStatus(400)
				}
			} else {
				route.HandleCtx(ctx)
			}
			return
		}
	}
	if !ctx.matched {
		ctx.SendStatus(404)
	}
}

func (r *Route) matchRoute(method, path string) (match bool, values []string) {
	// is route middleware? matches all http methods
	if r.isMiddleware {
		// '*' or '/' means its a valid match
		if r.isStar || r.isSlash {
			return true, nil
		}
		// if midware path starts with req.path
		if strings.HasPrefix(path, r.Path) {
			return true, nil
		}
		// middlewares dont support regex so bye!
		return false, nil
	}
	// non-middleware route, http method must match!
	// the wildcard method is for .All() method
	if r.Method != method && r.Method[0] != '*' {
		return false, nil
	}
	// '*' means we match anything
	if r.isStar {
		return true, nil
	}
	// simple '/' bool, so you avoid unnecessary comparison for long paths
	if r.isSlash && path == "/" {
		return true, nil
	}
	// does this route need regex matching?
	if r.isRegex {
		// req.path match regex pattern
		if r.Regexp.MatchString(path) {
			// do we have parameters
			if len(r.Params) > 0 {
				// get values for parameters
				matches := r.Regexp.FindAllStringSubmatch(path, -1)
				// did we get the values?
				if len(matches) > 0 && len(matches[0]) > 1 {
					values = matches[0][1:len(matches[0])]
					return true, values
				}
				return false, nil
			}
			return true, nil
		}
		return false, nil
	}
	// last thing to do is to check for a simple path match
	if r.Path == path {
		return true, nil
	}
	// Nothing match
	return false, nil
}

func (app *App) handler(fctx *fasthttp.RequestCtx) {
	// get fiber context from sync pool
	ctx := acquireCtx(fctx)
	defer releaseCtx(ctx)

	// attach app poiner and compress settings
	ctx.app = app
	ctx.compress = app.Settings.Compression

	// Case sensitive routing
	if !app.Settings.CaseSensitive {
		ctx.path = strings.ToLower(ctx.path)
	}
	// Strict routing
	if !app.Settings.StrictRouting && len(ctx.path) > 1 {
		ctx.path = strings.TrimRight(ctx.path, "/")
	}

	app.nextRoute(ctx)

	if ctx.compress {
		compressResponse(fctx)
	}
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

	// Case sensitive routing, all to lowercase
	if !app.Settings.CaseSensitive {
		path = strings.ToLower(path)
	}
	// Strict routing, remove last `/`
	if !app.Settings.StrictRouting && len(path) > 1 {
		path = strings.TrimRight(path, "/")
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
			Params:       Params,
			Regexp:       Regexp,
			HandleCtx:    handlers[i],
		})
	}
}

func (app *App) registerWebSocket(method, path string, handle func(*Conn)) {
	// Cannot have an empty path
	if path == "" {
		path = "/"
	}
	// Path always start with a '/' or '*'
	if path[0] != '/' && path[0] != '*' {
		path = "/" + path
	}

	// Case sensitive routing, all to lowercase
	if !app.Settings.CaseSensitive {
		path = strings.ToLower(path)
	}
	// Strict routing, remove last `/`
	if !app.Settings.StrictRouting && len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}

	var isWebSocket = true

	var isStar = path == "*" || path == "/*"
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
	app.routes = append(app.routes, &Route{
		isWebSocket: isWebSocket,
		isStar:      isStar,
		isSlash:     isSlash,
		isRegex:     isRegex,

		Method:     method,
		Path:       path,
		Params:     Params,
		Regexp:     Regexp,
		HandleConn: handle,
	})
}

func (app *App) registerStatic(prefix, root string) {
	// Cannot have an empty prefix
	if prefix == "" {
		prefix = "/"
	}
	// prefix always start with a '/' or '*'
	if prefix[0] != '/' && prefix[0] != '*' {
		prefix = "/" + prefix
	}
	// Case sensitive routing, all to lowercase
	if !app.Settings.CaseSensitive {
		prefix = strings.ToLower(prefix)
	}

	var isStar = prefix == "*" || prefix == "/*"

	files := map[string]string{}
	// Clean root path
	root = filepath.Clean(root)
	// Check if root exist and is accessible
	if _, err := os.Stat(root); err != nil {
		log.Fatalf("%s", err)
	}
	// Store path url and file paths in map
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			url := "*"
			if !isStar {
				// /css/style.css: static/css/style.css
				url = filepath.Join(prefix, strings.Replace(path, root, "", 1))
			}
			// \static\css: /static/css
			url = filepath.ToSlash(url)
			files[url] = path
			if filepath.Base(path) == "index.html" {
				files[url] = path
			}
		}
		return err
	}); err != nil {
		log.Fatalf("%s", err)
	}
	compress := app.Settings.Compression
	app.routes = append(app.routes, &Route{
		isMiddleware: true,
		isStar:       isStar,
		Method:       "*",
		Path:         prefix,
		HandleCtx: func(c *Ctx) {
			// Only allow GET & HEAD methods
			if c.method == "GET" || c.method == "HEAD" {
				path := "*"
				if !isStar {
					path = c.path
				}
				file := files[path]
				if file != "" {
					c.SendFile(file, compress)
					return
				}
			}
			c.matched = false
			c.Next()
		},
	})

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
