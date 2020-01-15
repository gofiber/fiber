// 🚀 Fiber, Express on Steriods
// 📌 Don't use in production until version 1.0.0
// 🖥 https://github.com/fenny/fiber

// 🦸 Not all heroes wear capes, thank you +1000
// 💖 @valyala, @dgrr, @erikdubbelboer, @savsgio, @julienschmidt

package fiber

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	// This json parsing lib is awesome
	// "github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
)

const (
	// Version for debugging
	Version = `0.6.2`
	// Port and Version are printed with the banner
	banner = `%s  _____ _ _
 %s|   __|_| |_ ___ ___
 %s|   __| | . | -_|  _|
 %s|__|  |_|___|___|_|%s
 %s%s

 `
	// https://play.golang.org/p/r6GNeV1gbH
	cReset   = "\x1b[0000m"
	cBlack   = "\x1b[1;30m"
	cRed     = "\x1b[1;31m"
	cGreen   = "\x1b[1;32m"
	cYellow  = "\x1b[1;33m"
	cBlue    = "\x1b[1;34m"
	cMagenta = "\x1b[1;35m"
	cCyan    = "\x1b[1;36m"
	cWhite   = "\x1b[1;37m"
)

// Fiber structure
type Fiber struct {
	// Stores all routes
	routes []*route
	// Fasthttp server settings
	Fasthttp *Fasthttp
	// Server name header
	Server string
	// Provide certificate files to enable TLS
	CertKey  string
	CertFile string
	// Disable the fiber banner on launch
	NoBanner bool
	// Clears terminal on launch
	ClearTerminal bool
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
		NoBanner: false,
		// Terminal is not cleared by default
		ClearTerminal: false,
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

// Function to add a route correctly
func (r *Fiber) register(method string, args ...interface{}) {
	// Options
	var path string
	var static string
	var handler func(*Ctx)
	// app.Get(handler)
	if len(args) == 1 {
		switch arg := args[0].(type) {
		case string:
			static = arg
		case func(*Ctx):
			handler = arg
		}
	}
	// app.Get(path, handler)
	if len(args) == 2 {
		path = args[0].(string)
		if path[0] != '/' && path[0] != '*' {
			panic("Invalid path, must begin with slash '/' or wildcard '*'")
		}
		switch arg := args[1].(type) {
		case string:
			static = arg
		case func(*Ctx):
			handler = arg
		}
	}
	// Is this a static file handler?
	if static != "" {
		// static file route!!
		r.registerStatic(method, path, static)
	} else if handler != nil {
		// function route!!
		r.registerHandler(method, path, handler)
	} else {
		fmt.Println(reflect.TypeOf(handler))
		panic("Every route needs to contain either a dir/file path or callback function")
	}
}
func (r *Fiber) registerStatic(method, prefix, root string) {
	var wildcard bool
	if prefix == "*" || prefix == "/*" {
		wildcard = true
	}
	if prefix == "" {
		prefix = "/"
	}
	files, _, err := walkDir(root)
	if err != nil {
		panic(err)
	}
	mount := filepath.Clean(root)
	for _, file := range files {
		if strings.Contains(file, ".fasthttp.gz") {
			continue
		}
		path := filepath.Join(prefix, strings.Replace(file, mount, "", 1))
		filePath := file
		if filepath.Base(filePath) == "index.html" {
			r.routes = append(r.routes, &route{method, prefix, wildcard, nil, nil, func(c *Ctx) {
				c.SendFile(filePath)
			}})
		}
		r.routes = append(r.routes, &route{method, path, wildcard, nil, nil, func(c *Ctx) {
			c.SendFile(filePath)
		}})
	}
}
func (r *Fiber) registerHandler(method, path string, handler func(*Ctx)) {
	if path == "" || path == "*" || path == "/*" {
		r.routes = append(r.routes, &route{method, path, true, nil, nil, handler})
		return
	}
	// Get params from path
	params := getParams(path)
	// If path has no params, we dont need regex
	if len(params) == 0 {
		r.routes = append(r.routes, &route{method, path, false, nil, nil, handler})
		return
	}

	// Compile regix from path
	regex, err := getRegex(path)
	if err != nil {
		panic("Invalid url pattern: " + path)
	}
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
	portStr := strconv.Itoa(port)
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
	if r.ClearTerminal {
		if runtime.GOOS == "linux" {
			cmd := exec.Command("clear")
			cmd.Stdout = os.Stdout
			cmd.Run()
		} else if runtime.GOOS == "windows" {
			cmd := exec.Command("cmd", "/c", "cls")
			cmd.Stdout = os.Stdout
			cmd.Run()
		}
	}
	if !r.NoBanner {
		fmt.Printf(banner, cGreen, cGreen, cGreen, cGreen,
			cBlack+Version,
			cBlack+"Express on steriods",
			cGreen+":"+portStr+cReset,
		)
	}
	if r.CertKey != "" && r.CertFile != "" {
		if err := server.ListenAndServeTLS(fmt.Sprintf("%s:%s", address, portStr), r.CertFile, r.CertKey); err != nil {
			panic(err)
		}
	} else {
		if err := server.ListenAndServe(fmt.Sprintf("%s:%s", address, portStr)); err != nil {
			panic(err)
		}
	}
}
