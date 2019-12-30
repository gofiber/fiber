package fiber

import (
	"fmt"
	"regexp"
	"time"

	"github.com/valyala/fasthttp"
)

type route struct {
	method  string
	path    string
	anyPath bool
	regex   *regexp.Regexp
	params  []string
	handler func(*Context)
}

// Settings :
type Settings struct {
	TLSEnable                          bool
	CertKey                            string
	CertFile                           string
	Name                               string
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
		methods: []string{"GET", "PUT", "POST", "DELETE", "HEAD", "PATCH", "OPTIONS", "TRACE", "CONNECT"},
		Settings: &Settings{
			TLSEnable:                          false,
			CertKey:                            "",
			CertFile:                           "",
			Name:                               "",
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

// Get :
func (r *Fiber) Get(args ...interface{}) {
	r.register("GET", args...)
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

// Connect :
func (r *Fiber) Connect(args ...interface{}) {
	r.register("CONNECT", args...)
}

// All :
func (r *Fiber) All(args ...interface{}) {
	r.register("*", args...)
	// for _, method := range r.methods {
	// 	r.register(method, args...)
	// }
}

// Use :
func (r *Fiber) Use(args ...interface{}) {
	r.register("*", args...)
	// for _, method := range r.methods {
	// 	r.register(method, args...)
	// }
}

// register :
func (r *Fiber) register(method string, args ...interface{}) {
	// Pre-set variables for interface assertion
	var ok bool
	var path string
	var handler func(*Context)
	// Register only handler: app.Get(handler)
	if len(args) == 1 {
		// Convert interface to func(*Context)
		handler, ok = args[0].(func(*Context))
		if !ok {
			panic("Invalid handler must be func(*express.Context)")
		}
	}
	// Register path and handler: app.Get(path, handler)
	if len(args) == 2 {
		// Convert interface to path string
		path, ok = args[0].(string)
		if !ok {
			panic("Invalid path")
		}
		// Panic if first char does not begins with / or *
		if path[0] != '/' && path[0] != '*' {
			panic("Invalid path, must begin with slash '/' or wildcard '*'")
		}
		// Convert interface to func(*Context)
		handler, ok = args[1].(func(*Context))
		if !ok {
			panic("Invalid handler, must be func(*express.Context)")
		}
	}
	// If its a simple wildcard ( aka match anything )
	if path == "" || path == "*" || path == "/*" {
		r.routes = append(r.routes, &route{method, path, true, nil, nil, handler})
		fmt.Println(r.routes[0])
		return
	}
	// Get params from path
	params := getParams(path)
	fmt.Println(params)
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
		if route.anyPath || (route.path == path && route.params == nil) {
			// If * always set the path to the wildcard parameter
			if route.anyPath {
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
		fmt.Println("We did regex -,-")
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
func (r *Fiber) Listen(port int) {
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
	if r.Settings.TLSEnable {
		if err := server.ListenAndServeTLS(fmt.Sprintf(":%v", port), r.Settings.CertFile, r.Settings.CertKey); err != nil {
			panic(err)
		}
		return
	}
	if err := server.ListenAndServe(fmt.Sprintf(":%v", port)); err != nil {
		panic(err)
	}
}
