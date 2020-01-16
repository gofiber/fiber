// 🚀 Fiber, Express on Steriods
// 📌 Don't use in production until version 1.0.0
// 🖥 https://github.com/fenny/fiber

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @dgrr, @erikdubbelboer, @savsgio, @julienschmidt

package fiber

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	// This json parsing lib is awesome
	// "github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
)

// Version for debugging
const Version = "0.7.0"

// Fiber structure
type Fiber struct {
	// Stores all routes
	routes []*route
	// Server name header
	Server string
	// Disable the fiber banner on launch
	Banner bool
	// RedirectTrailingSlash TODO*
	RedirectTrailingSlash bool
	// Provide certificate files to enable TLS
	CertKey  string
	CertFile string
	// Fasthttp server settings
	Fasthttp *Fasthttp
}

type route struct {
	// HTTP method in uppercase, can be a * for Use() & All()
	method string
	// Stores the orignal path
	path string
	// wildcard bool is for routes without a path, * and /*
	wildcard bool
	// Stores compiled regex special routes :params, *wildcards, optionals?
	regex *regexp.Regexp
	// Store params if special routes :params, *wildcards, optionals?
	params []string
	// Callback function for specific route
	handler func(*Ctx)
}

// Fasthttp settings
// https://github.com/valyala/fasthttp/blob/master/server.go#L150
type Fasthttp struct {
	Concurrency                        int
	DisableKeepAlive                   bool
	ReadBufferSize                     int
	WriteBufferSize                    int
	ReadTimeout                        time.Duration
	WriteTimeout                       time.Duration
	IdleTimeout                        time.Duration
	MaxConnsPerIP                      int
	MaxRequestsPerConn                 int
	TCPKeepalive                       bool
	TCPKeepalivePeriod                 time.Duration
	MaxRequestBodySize                 int
	ReduceMemoryUsage                  bool
	GetOnly                            bool
	DisableHeaderNamesNormalizing      bool
	SleepWhenConcurrencyLimitsExceeded time.Duration
	NoDefaultContentType               bool
	KeepHijackedConns                  bool
}

// New creates a Fiber instance
func New() *Fiber {
	return &Fiber{
		// No server header is sent when set empty ""
		Server: "",
		// TLS is disabled by default, unless files are provided
		CertKey:  "",
		CertFile: "",
		// Fiber banner is printed by default
		Banner: true,
		Fasthttp: &Fasthttp{
			// Default fasthttp settings
			// https://github.com/valyala/fasthttp/blob/master/server.go#L150
			Concurrency:                        256 * 1024,
			DisableKeepAlive:                   false,
			ReadBufferSize:                     4096,
			WriteBufferSize:                    4096,
			WriteTimeout:                       0,
			ReadTimeout:                        0,
			IdleTimeout:                        0,
			MaxConnsPerIP:                      0,
			MaxRequestsPerConn:                 0,
			TCPKeepalive:                       false,
			TCPKeepalivePeriod:                 0,
			MaxRequestBodySize:                 4 * 1024 * 1024,
			ReduceMemoryUsage:                  false,
			GetOnly:                            false,
			DisableHeaderNamesNormalizing:      false,
			SleepWhenConcurrencyLimitsExceeded: 0,
			NoDefaultContentType:               false,
			KeepHijackedConns:                  false,
		},
	}
}

// Connect establishes a tunnel to the server
// identified by the target resource.
func (r *Fiber) Connect(args ...interface{}) {
	r.register("CONNECT", args...)
}

// Put replaces all current representations
// of the target resource with the request payload.
func (r *Fiber) Put(args ...interface{}) {
	r.register("PUT", args...)
}

// Post is used to submit an entity to the specified resource,
// often causing a change in state or side effects on the server.
func (r *Fiber) Post(args ...interface{}) {
	r.register("POST", args...)
}

// Delete deletes the specified resource.
func (r *Fiber) Delete(args ...interface{}) {
	r.register("DELETE", args...)
}

// Head asks for a response identical to that of a GET request,
// but without the response body.
func (r *Fiber) Head(args ...interface{}) {
	r.register("HEAD", args...)
}

// Patch is used to apply partial modifications to a resource.
func (r *Fiber) Patch(args ...interface{}) {
	r.register("PATCH", args...)
}

// Options is used to describe the communication options
// for the target resource.
func (r *Fiber) Options(args ...interface{}) {
	r.register("OPTIONS", args...)
}

// Trace performs a message loop-back test
// along the path to the target resource.
func (r *Fiber) Trace(args ...interface{}) {
	r.register("TRACE", args...)
}

// Get requests a representation of the specified resource.
// Requests using GET should only retrieve data.
func (r *Fiber) Get(args ...interface{}) {
	r.register("GET", args...)
}

// All matches any HTTP method
func (r *Fiber) All(args ...interface{}) {
	r.register("*", args...)
}

// Use is another name for All()
// People using Expressjs are used to this
func (r *Fiber) Use(args ...interface{}) {
	r.All(args...)
}

// Static :
func (r *Fiber) Static(args ...string) {
	prefix := "/"
	root := "./"
	wildcard := false
	gzip := true
	if len(args) == 1 {
		root = args[0]
	} else if len(args) == 2 {
		prefix = args[0]
		root = args[1]
		if prefix[0] != '/' {
			prefix = "/" + prefix
		}
	}
	// Check if wildcard for single files
	if prefix == "*" || prefix == "/*" {
		wildcard = true
	}
	// Lets get all files from root
	files, _, err := walkDir(root)
	if err != nil {
		panic(err)
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
		// Store original file path to use in ctx handler
		filePath := file
		// If the file is an index.html, bind the prefix to index.html directly
		if filepath.Base(filePath) == "index.html" {
			r.routes = append(r.routes, &route{"GET", prefix, wildcard, nil, nil, func(c *Ctx) {
				c.SendFile(filePath, gzip)
			}})
		}
		// Add the route + SendFile(filepath) to routes
		r.routes = append(r.routes, &route{"GET", path, wildcard, nil, nil, func(c *Ctx) {
			c.SendFile(filePath, gzip)
		}})
	}
}

// Function to add a route correctly
func (r *Fiber) register(method string, args ...interface{}) {
	// Prepare possible variables
	var path string        // We could have a path/prefix
	var handler func(*Ctx) // We could have a ctx handler
	// Only 1 argument, so no path/prefix
	if len(args) == 1 {
		handler = args[0].(func(*Ctx))
	} else if len(args) > 1 {
		path = args[0].(string)
		handler = args[1].(func(*Ctx))
		if path[0] != '/' && path[0] != '*' {
			panic("Invalid path, must begin with slash '/' or wildcard '*'")
		}
	}
	// If the route needs to match any path
	if path == "" || path == "*" || path == "/*" {
		r.routes = append(r.routes, &route{method, path, true, nil, nil, handler})
		return
	}
	// Get params from path
	params := getParams(path)
	// If path has no params (simple path), we dont need regex
	if len(params) == 0 {
		r.routes = append(r.routes, &route{method, path, false, nil, nil, handler})
		return
	}

	// We have parametes, so we need to compile regix from the path
	regex, err := getRegex(path)
	if err != nil {
		panic("Invalid url pattern: " + path)
	}
	// Add regex + params to route
	r.routes = append(r.routes, &route{method, path, false, regex, params, handler})
}

// handler create a new context struct from the pool
// then try to match a route as efficient as possible.
// 1 > loop trough all routes
// 2 > if method != * or method != method   							SKIP
// 3 > if any == true or (path == path && params == nil): MATCH
// 4 > if regex == nil: 																	SKIP
// 5 > if regex.match(path) != true: 											SKIP
// 6 > if params != nil && len(params) > 0 								REGEXPARAMS
func (r *Fiber) handler(fctx *fasthttp.RequestCtx) {
	found := false
	// get custom context from sync pool
	ctx := acquireCtx(fctx)
	// get path and method from main context
	path := ctx.Path()
	method := ctx.Method()
	// loop trough routes
	for _, route := range r.routes {
		// Skip route if method is not allowed
		if route.method != "*" && route.method != method {
			continue
		}
		// First check if we match a static path or wildcard
		if route.wildcard || (route.path == path && route.params == nil) {
			// If * always set the path to the wildcard parameter
			if route.wildcard {
				ctx.params = &[]string{"*"}
				ctx.values = []string{path}
			}
			found = true
			// Set route pointer if user wants to call .Route()
			ctx.route = route
			// Execute handler with context
			route.handler(ctx)
			// if next is not set, leave loop and release ctx
			if !ctx.next {
				break
			}
			// set next to false for next iteration
			ctx.next = false
			// continue to go to the next route
			continue
		}
		// Skip route if regex does not exist
		if route.regex == nil {
			continue
		}
		// Skip route if regex does not match
		if !route.regex.MatchString(path) {
			continue
		}
		// If we have parameters, lets find the matches
		if route.params != nil && len(route.params) > 0 {
			matches := route.regex.FindAllStringSubmatch(path, -1)
			// If we have matches, add params and values to context
			if len(matches) > 0 && len(matches[0]) > 1 {
				ctx.params = &route.params
				ctx.values = matches[0][1:len(matches[0])]
			}
		}
		found = true
		// Set route pointer if user wants to call .Route()
		ctx.route = route
		// Execute handler with context
		route.handler(ctx)
		// if next is not set, leave loop and release ctx
		if !ctx.next {
			break
		}
		// set next to false for next iteration
		ctx.next = false
	}
	// No routes found
	if !found {
		// Custom 404 handler?
		ctx.Status(404).Send("Not Found")
	}
	// release context back into sync pool
	releaseCtx(ctx)
}

// Listen starts the server with the correct settings
func (r *Fiber) Listen(port int, addr ...string) {
	var address string
	if len(addr) > 0 {
		address = addr[0]
	}
	server := &fasthttp.Server{
		Handler:                            r.handler,
		Name:                               r.Server,
		Concurrency:                        r.Fasthttp.Concurrency,
		DisableKeepalive:                   r.Fasthttp.DisableKeepAlive,
		ReadBufferSize:                     r.Fasthttp.ReadBufferSize,
		WriteBufferSize:                    r.Fasthttp.WriteBufferSize,
		ReadTimeout:                        r.Fasthttp.ReadTimeout,
		WriteTimeout:                       r.Fasthttp.WriteTimeout,
		IdleTimeout:                        r.Fasthttp.IdleTimeout,
		MaxConnsPerIP:                      r.Fasthttp.MaxConnsPerIP,
		MaxRequestsPerConn:                 r.Fasthttp.MaxRequestsPerConn,
		TCPKeepalive:                       r.Fasthttp.TCPKeepalive,
		TCPKeepalivePeriod:                 r.Fasthttp.TCPKeepalivePeriod,
		MaxRequestBodySize:                 r.Fasthttp.MaxRequestBodySize,
		ReduceMemoryUsage:                  r.Fasthttp.ReduceMemoryUsage,
		GetOnly:                            r.Fasthttp.GetOnly,
		DisableHeaderNamesNormalizing:      r.Fasthttp.DisableHeaderNamesNormalizing,
		SleepWhenConcurrencyLimitsExceeded: r.Fasthttp.SleepWhenConcurrencyLimitsExceeded,
		NoDefaultServerHeader:              r.Server == "",
		NoDefaultContentType:               r.Fasthttp.NoDefaultContentType,
		KeepHijackedConns:                  r.Fasthttp.KeepHijackedConns,
	}
	if r.Banner {
		// https://play.golang.org/p/r6GNeV1gbH
		// http://patorjk.com/software/taag
		fmt.Printf("\x1b[1;32m  _____ _ _\n \x1b[1;32m|   __|_| |_ ___ ___\n \x1b[1;32m|   __| | . | -_|  _|\n \x1b[1;32m|__|  |_|___|___|_|\x1b[1;30m%s\n \x1b[1;30m%s\x1b[1;32m%v\x1b[0000m\n\n", Version, "Express on steriods:", port)
	}
	if r.CertKey != "" && r.CertFile != "" {
		if err := server.ListenAndServeTLS(fmt.Sprintf("%s:%v", address, port), r.CertFile, r.CertKey); err != nil {
			panic(err)
		}
	} else {
		if err := server.ListenAndServe(fmt.Sprintf("%s:%v", address, port)); err != nil {
			panic(err)
		}
	}
}
