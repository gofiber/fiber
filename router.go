package fiber

import (
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	websocket "github.com/fasthttp/websocket"
	fasthttp "github.com/valyala/fasthttp"
)

// Route struct
type Route struct {
	// HTTP method in uppercase, can be a * for "Use" & "All" routes
	Method string
	// Stores the original path
	Path string
	// Prefix is for ending wildcards or middlewares
	Prefix string
	// Stores regex for :params & :optionals?
	Regex *regexp.Regexp
	// Stores params keys for :params & :optionals?
	Params []string
	// Callback function for context
	HandlerCtx func(*Ctx)
	// Callback function for websockets
	HandlerConn func(*Conn)
}

func (app *App) registerStatic(grpPrefix string, args ...string) {
	var prefix = "/"
	var root = "./"
	// enable / disable gzipping somewhere?
	// todo v2.0.0
	gzip := true

	if len(args) == 1 {
		root = args[0]
	}
	if len(args) == 2 {
		prefix = args[0]
		root = args[1]
	}

	// A non wildcard path must start with a '/'
	if prefix != "*" && len(prefix) > 0 && prefix[0] != '/' {
		prefix = "/" + prefix
	}
	// Prepend group prefix
	if len(grpPrefix) > 0 {
		// `/v1`+`/` => `/v1`+``
		if prefix == "/" {
			prefix = grpPrefix
		} else {
			prefix = grpPrefix + prefix
		}
		// Remove duplicate slashes `//`
		prefix = strings.Replace(prefix, "//", "/", -1)
	}
	// Empty or '/*' path equals "match anything"
	// TODO fix * for paths with grpprefix
	if prefix == "/*" {
		prefix = "*"
	}
	// Lets get all files from root
	files, _, err := getFiles(root)
	if err != nil {
		log.Fatal("Static: ", err)
	}
	// ./static/compiled => static/compiled
	mount := filepath.Clean(root)
	// Loop over all files
	for _, file := range files {
		// Ignore the .gzipped files by fasthttp
		if strings.Contains(file, ".fasthttp.gz") {
			continue
		}
		// Time to create a fake path for the route match
		// static/index.html => /index.html
		path := filepath.Join(prefix, strings.Replace(file, mount, "", 1))
		// for windows: static\index.html => /index.html
		path = filepath.ToSlash(path)
		// Store file path to use in ctx handler
		filePath := file

		if len(prefix) > 1 && strings.Contains(prefix, "*") {
			app.routes = append(app.routes, &Route{
				Method: "GET",
				Path:   path,
				Prefix: strings.Split(prefix, "*")[0],
				HandlerCtx: func(c *Ctx) {
					c.SendFile(filePath, gzip)
				},
			})
			return
		}
		// If the file is an index.html, bind the prefix to index.html directly
		if filepath.Base(filePath) == "index.html" || filepath.Base(filePath) == "index.htm" {
			app.routes = append(app.routes, &Route{
				Method: "GET",
				Path:   prefix,
				HandlerCtx: func(c *Ctx) {
					c.SendFile(filePath, gzip)
				},
			})
		}

		// Add the route + SendFile(filepath) to routes
		app.routes = append(app.routes, &Route{
			Method: "GET",
			Path:   path,
			HandlerCtx: func(c *Ctx) {
				c.SendFile(filePath, gzip)
			},
		})
	}
}
func (app *App) register(method, grpPrefix string, args ...interface{}) {
	// Set variables
	var path = "*"
	var prefix string
	var middleware = method == "USE"
	var handlersCtx []func(*Ctx)
	var handlersConn []func(*Conn)
	for i := 0; i < len(args); i++ {
		switch arg := args[i].(type) {
		case string:
			path = arg
		case func(*Ctx):
			handlersCtx = append(handlersCtx, arg)
		case func(*Conn):
			handlersConn = append(handlersConn, arg)
		default:
			log.Fatalf("Invalid argument type: %v", reflect.TypeOf(arg))
		}
	}
	// A non wildcard path must start with a '/'
	if path != "*" && len(path) > 0 && path[0] != '/' {
		path = "/" + path
	}
	// Prepend group prefix
	if len(grpPrefix) > 0 {
		// `/v1`+`/` => `/v1`+``
		if path == "/" {
			path = grpPrefix
		} else {
			path = grpPrefix + path
		}
		// Remove duplicate slashes `//`
		path = strings.Replace(path, "//", "/", -1)
	}
	// Empty or '/*' path equals "match anything"
	// TODO fix * for paths with grpprefix
	if path == "" || path == "/*" {
		path = "*"
	}
	if method == "ALL" || middleware {
		method = "*"
	}
	// Routes are case insensitive by default
	if !app.Settings.CaseSensitive {
		path = strings.ToLower(path)
	}
	if !app.Settings.StrictRouting && len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	// If the route can match anything
	if path == "*" {
		for i := range handlersCtx {
			app.routes = append(app.routes, &Route{
				Method: method, Path: path, HandlerCtx: handlersCtx[i],
			})
		}
		for i := range handlersConn {
			app.routes = append(app.routes, &Route{
				Method: method, Path: path, HandlerConn: handlersConn[i],
			})
		}
		return
	}
	// Get ':param' & ':optional?' & '*' from path
	params := getParams(path)
	// Enable prefix for midware
	if len(params) == 0 && middleware {
		prefix = path
	}

	// If path has no params (simple path)
	if len(params) == 0 {
		for i := range handlersCtx {
			app.routes = append(app.routes, &Route{
				Method: method, Path: path, Prefix: prefix, HandlerCtx: handlersCtx[i],
			})
		}
		for i := range handlersConn {
			app.routes = append(app.routes, &Route{
				Method: method, Path: path, Prefix: prefix, HandlerConn: handlersConn[i],
			})
		}
		return
	}

	// If path only contains 1 wildcard, we can create a prefix
	// If its a middleware, we also create a prefix
	if len(params) == 1 && params[0] == "*" {
		prefix = strings.Split(path, "*")[0]
		for i := range handlersCtx {
			app.routes = append(app.routes, &Route{
				Method: method, Path: path, Prefix: prefix,
				Params: params, HandlerCtx: handlersCtx[i],
			})
		}
		for i := range handlersConn {
			app.routes = append(app.routes, &Route{
				Method: method, Path: path, Prefix: prefix,
				Params: params, HandlerConn: handlersConn[i],
			})
		}
		return
	}
	// We have an :param or :optional? and need to compile a regex struct
	regex, err := getRegex(path)
	if err != nil {
		log.Fatal("Router: Invalid path pattern: " + path)
	}
	// Add route with regex
	for i := range handlersCtx {
		app.routes = append(app.routes, &Route{
			Method: method, Path: path, Regex: regex,
			Params: params, HandlerCtx: handlersCtx[i],
		})
	}
	for i := range handlersConn {
		app.routes = append(app.routes, &Route{
			Method: method, Path: path, Regex: regex,
			Params: params, HandlerConn: handlersConn[i],
		})
	}
}
func (app *App) handler(fctx *fasthttp.RequestCtx) {
	// Use this boolean to perform 404 not found at the end
	var match = false
	// get custom context from sync pool
	ctx := acquireCtx(fctx)
	if ctx.app == nil {
		ctx.app = app
	}
	// get path and method
	path := ctx.Path()
	if !app.Settings.CaseSensitive {
		path = strings.ToLower(path)
	}
	if !app.Settings.StrictRouting && len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	method := ctx.Method()
	// enable recovery
	if app.recover != nil {
		defer func() {
			if r := recover(); r != nil {
				ctx.error = fmt.Errorf("panic: %v", r)
				app.recover(ctx)
			}
		}()
	}
	// loop trough routes
	for _, route := range app.routes {
		// Skip route if method does not match
		if route.Method != "*" && route.Method != method {
			continue
		}
		// Set route pointer if user wants to call .Route()
		ctx.route = route
		// wilcard or exact same path
		// TODO v2: enable or disable case insensitive match
		if route.Path == "*" || route.Path == path {
			// if * always set the path to the wildcard parameter
			if route.Path == "*" {
				ctx.params = &[]string{"*"}
				ctx.values = []string{path}
			}
			// ctx.Fasthttp.Request.Header.ConnectionUpgrade()
			// Websocket request
			if route.HandlerConn != nil && websocket.FastHTTPIsWebSocketUpgrade(fctx) {
				// Try to upgrade
				err := socketUpgrade.Upgrade(ctx.Fasthttp, func(fconn *websocket.Conn) {
					conn := acquireConn(fconn)
					defer releaseConn(conn)
					conn.params = ctx.params
					conn.values = ctx.values
					releaseCtx(ctx)
					route.HandlerConn(conn)
				})
				// Upgrading failed
				if err != nil {
					panic(err)
				}
				return
			}
			// No handler for HTTP nor websocket
			if route.HandlerCtx == nil {
				continue
			}
			// Match found, 404 not needed
			match = true
			route.HandlerCtx(ctx)
			// if next is not set, leave loop and release ctx
			if !ctx.next {
				break
			} else {
				// reset match to false
				match = false
			}
			// set next to false for next iteration
			ctx.next = false
			// continue to go to the next route
			continue
		}
		if route.Prefix != "" && strings.HasPrefix(path, route.Prefix) {
			ctx.route = route
			if strings.Contains(route.Path, "*") {
				ctx.params = &[]string{"*"}
				// ctx.values = matches[0][1:len(matches[0])]
				// parse query source
				ctx.values = []string{strings.Replace(path, route.Prefix, "", 1)}
			}
			// Websocket request
			if route.HandlerConn != nil {
				// Try to upgrade
				err := socketUpgrade.Upgrade(ctx.Fasthttp, func(fconn *websocket.Conn) {
					conn := acquireConn(fconn)
					defer releaseConn(conn)
					conn.params = ctx.params
					conn.values = ctx.values
					releaseCtx(ctx)
					route.HandlerConn(conn)
				})
				// Upgrading failed
				if err != nil {
					panic(err)
				}
				return
			}
			// No handler for HTTP nor websocket
			if route.HandlerCtx == nil {
				continue
			}
			// Match found, 404 not needed
			match = true
			route.HandlerCtx(ctx)
			// if next is not set, leave loop and release ctx
			if !ctx.next {
				break
			} else {
				// reset match to false
				match = false
			}
			// set next to false for next iteration
			ctx.next = false
			// continue to go to the next route
			continue
		}

		// Skip route if regex does not exist
		if route.Regex == nil {
			continue
		}

		// Skip route if regex does not match
		if !route.Regex.MatchString(path) {
			continue
		}

		// If we have parameters, lets find the matches
		if len(route.Params) > 0 {
			matches := route.Regex.FindAllStringSubmatch(path, -1)
			// If we have matches, add params and values to context
			if len(matches) > 0 && len(matches[0]) > 1 {
				ctx.params = &route.Params
				ctx.values = matches[0][1:len(matches[0])]
			}
		}
		// Websocket route
		if route.HandlerConn != nil {
			// Try to upgrade
			err := socketUpgrade.Upgrade(ctx.Fasthttp, func(fconn *websocket.Conn) {
				conn := acquireConn(fconn)
				conn.params = ctx.params
				conn.values = ctx.values
				releaseCtx(ctx)
				defer releaseConn(conn)
				route.HandlerConn(conn)
			})
			// Upgrading failed
			if err != nil {
				panic(err)
			}
			return
		}
		// No handler for HTTP nor websocket
		if route.HandlerCtx == nil {
			continue
		}
		// Match found, 404 not needed
		match = true
		route.HandlerCtx(ctx)
		// if next is not set, leave loop and release ctx
		if !ctx.next {
			break
		} else {
			// reset match to false
			match = false
		}
		// set next to false for next iteration
		ctx.next = false
	}
	// No match, send default 404
	if !match {
		ctx.SendStatus(404)
	}
	releaseCtx(ctx)
}
