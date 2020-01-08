package fiber

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/valyala/fasthttp"
)

const (
	Version = "v0.2.0"
	banner  = ` _____ _ _
|   __|_| |_ ___ ___
|   __| | . | -_|  _|
|__|  |_|___|___|_|%s
`
)

type route struct {
	method  string
	any     bool
	path    string
	regex   *regexp.Regexp
	params  []string
	handler func(*Ctx)
}

// Settings :
type Settings struct {
	Name                               string
	HideBanner                         bool
	TLSEnable                          bool
	CertKey                            string
	CertFile                           string
	Concurrency                        int
	DisableKeepAlive                   bool
	ReadBufferSize                     int
	WriteBufferSize                    int
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
	NoDefaultServerHeader              bool
	NoDefaultContentType               bool
	KeepHijackedConns                  bool
}

// Fiber :
type Fiber struct {
	routes   []*route
	methods  []string
	Settings *Settings
}

// New :
func New() *Fiber {
	return &Fiber{
		Settings: &Settings{
			Name:                               "",
			HideBanner:                         false,
			TLSEnable:                          false,
			CertKey:                            "",
			CertFile:                           "",
			Concurrency:                        256 * 1024,
			DisableKeepAlive:                   false,
			ReadBufferSize:                     4096,
			WriteBufferSize:                    4096,
			WriteTimeout:                       0,
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
			NoDefaultServerHeader:              true,
			NoDefaultContentType:               false,
			KeepHijackedConns:                  false,
		},
	}
}

// Connect :
func (r *Fiber) Connect(args ...interface{}) {
	r.register("CONNECT", args...)
}

// Put :
func (r *Fiber) Put(args ...interface{}) {
	r.register("PUT", args...)
}

// Post :
func (r *Fiber) Post(args ...interface{}) {
	r.register("POST", args...)
}

// Delete :
func (r *Fiber) Delete(args ...interface{}) {
	r.register("DELETE", args...)
}

// Head :
func (r *Fiber) Head(args ...interface{}) {
	r.register("HEAD", args...)
}

// Patch :
func (r *Fiber) Patch(args ...interface{}) {
	r.register("PATCH", args...)
}

// Options :
func (r *Fiber) Options(args ...interface{}) {
	r.register("OPTIONS", args...)
}

// Trace :
func (r *Fiber) Trace(args ...interface{}) {
	r.register("TRACE", args...)
}

// Get :
func (r *Fiber) Get(args ...interface{}) {
	r.register("GET", args...)
}

// Use :
func (r *Fiber) Use(args ...interface{}) {
	r.register("*", args...)
}

// All :
func (r *Fiber) All(args ...interface{}) {
	r.register("*", args...)
}

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
		panic("Every route needs to contain either a dir/file path or callback function")
	}
}
func (r *Fiber) registerStatic(method, prefix, root string) {
	var any bool
	if prefix == "*" || prefix == "/*" {
		any = true
	}
	if prefix == "" {
		prefix = "/"
	}
	files, _, err := walk(root)
	if err != nil {
		panic(err)
	}
	mount := filepath.Clean(root)
	for _, file := range files {
		path := filepath.Join(prefix, strings.Replace(file, mount, "", 1))
		filePath := file
		if filepath.Base(filePath) == "index.html" {
			r.routes = append(r.routes, &route{method, any, prefix, nil, nil, func(c *Ctx) {
				c.SendFile(filePath)
			}})
		}
		r.routes = append(r.routes, &route{method, any, path, nil, nil, func(c *Ctx) {
			c.SendFile(filePath)
		}})
	}
}
func (r *Fiber) registerHandler(method, path string, handler func(*Ctx)) {
	if path == "" || path == "*" || path == "/*" {
		r.routes = append(r.routes, &route{method, true, path, nil, nil, handler})
		return
	}
	// Get params from path
	params := getParams(path)
	// If path has no params, we dont need regex
	if len(params) == 0 {
		r.routes = append(r.routes, &route{method, false, path, nil, nil, handler})
		return
	}

	// Compile regix from path
	regex, err := getRegex(path)
	if err != nil {
		panic("Invalid url pattern: " + path)
	}
	r.routes = append(r.routes, &route{method, false, path, regex, params, handler})
}

// handler :
func (r *Fiber) handler(fctx *fasthttp.RequestCtx) {
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
		if route.any || (route.path == path && route.params == nil) {
			// If * always set the path to the wildcard parameter
			if route.any {
				ctx.params = &[]string{"*"}
				ctx.values = []string{path}
			}
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
		// Skip route if regex does not match
		if route.regex == nil || !route.regex.MatchString(path) {
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
		// Execute handler with context
		route.handler(ctx)
		// if next is not set, leave loop and release ctx
		if !ctx.next {
			break
		}
		// set next to false for next iteration
		ctx.next = false
	}
	// release context back into sync pool
	releaseCtx(ctx)
}

// Listen :
func (r *Fiber) Listen(args ...interface{}) {
	var port string
	var addr string
	if len(args) == 1 {
		port = strconv.Itoa(args[0].(int))
	}
	if len(args) == 2 {
		addr = args[0].(string)
		port = strconv.Itoa(args[1].(int))
	}
	// Disable server header if server name is not given
	if r.Settings.Name != "" {
		r.Settings.NoDefaultServerHeader = false
	}
	server := &fasthttp.Server{
		// Express custom handler
		Handler: r.handler,
		// Server settings
		Name:                               r.Settings.Name,
		Concurrency:                        r.Settings.Concurrency,
		DisableKeepalive:                   r.Settings.DisableKeepAlive,
		ReadBufferSize:                     r.Settings.ReadBufferSize,
		WriteBufferSize:                    r.Settings.WriteBufferSize,
		WriteTimeout:                       r.Settings.WriteTimeout,
		IdleTimeout:                        r.Settings.IdleTimeout,
		MaxConnsPerIP:                      r.Settings.MaxConnsPerIP,
		MaxRequestsPerConn:                 r.Settings.MaxRequestsPerConn,
		TCPKeepalive:                       r.Settings.TCPKeepalive,
		TCPKeepalivePeriod:                 r.Settings.TCPKeepalivePeriod,
		MaxRequestBodySize:                 r.Settings.MaxRequestBodySize,
		ReduceMemoryUsage:                  r.Settings.ReduceMemoryUsage,
		GetOnly:                            r.Settings.GetOnly,
		DisableHeaderNamesNormalizing:      r.Settings.DisableHeaderNamesNormalizing,
		SleepWhenConcurrencyLimitsExceeded: r.Settings.SleepWhenConcurrencyLimitsExceeded,
		NoDefaultServerHeader:              r.Settings.NoDefaultServerHeader,
		NoDefaultContentType:               r.Settings.NoDefaultContentType,
		KeepHijackedConns:                  r.Settings.KeepHijackedConns,
	}
	if !r.Settings.HideBanner {
		fmt.Printf(color.CyanString(banner), color.GreenString(":"+port))
	}
	// fmt.Printf(banner, Version)
	if r.Settings.TLSEnable {
		if err := server.ListenAndServeTLS(fmt.Sprintf("%s:%s", addr, port), r.Settings.CertFile, r.Settings.CertKey); err != nil {
			panic(err)
		}
	} else {
		if err := server.ListenAndServe(fmt.Sprintf("%s:%s", addr, port)); err != nil {
			panic(err)
		}
	}
}
