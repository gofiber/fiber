// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	utils "github.com/gofiber/utils"
	fasthttp "github.com/valyala/fasthttp"
)

// Version of current package
const Version = "1.11.0"

// Map is a shortcut for map[string]interface{}, useful for JSON returns
type Map map[string]interface{}

// Handler defines a function to serve HTTP requests.
type Handler = func(*Ctx)

// App denotes the Fiber application.
type App struct {
	mutex sync.Mutex
	// Route stack
	stack [][]*Route
	// Amount of registered routes
	routes int
	// Ctx pool
	pool sync.Pool
	// Fasthttp server
	server *fasthttp.Server
	// App settings
	Settings *Settings
}

// Settings holds is a struct holding the server settings
type Settings struct {
	// ErrorHandler is executed when you pass an error in the Next(err) method
	// This function is also executed when middleware.Recover() catches a panic
	// Default: func(ctx *Ctx, err error) {
	// 	code := StatusInternalServerError
	// 	if e, ok := err.(*Error); ok {
	// 		code = e.Code
	// 	}
	// 	ctx.Status(code).SendString(err.Error())
	// }
	ErrorHandler Handler

	// Enables the "Server: value" HTTP header.
	// Default: ""
	ServerHeader string

	// Enable strict routing. When enabled, the router treats "/foo" and "/foo/" as different.
	// By default this is disabled and both "/foo" and "/foo/" will execute the same handler.
	StrictRouting bool

	// Enable case sensitive routing. When enabled, "/FoO" and "/foo" are different routes.
	// By default this is disabled and both "/FoO" and "/foo" will execute the same handler.
	CaseSensitive bool

	// Enables handler values to be immutable even if you return from handler
	// Default: false
	Immutable bool

	// Enable or disable ETag header generation, since both weak and strong etags are generated
	// using the same hashing method (CRC-32). Weak ETags are the default when enabled.
	// Default value false
	ETag bool

	// This will spawn multiple Go processes listening on the same port
	// Default: false
	Prefork bool

	// Max body size that the server accepts
	// Default: 4 * 1024 * 1024
	BodyLimit int

	// Maximum number of concurrent connections.
	// Default: 256 * 1024
	Concurrency int

	// Disable keep-alive connections, the server will close incoming connections after sending the first response to client
	// Default: false
	DisableKeepalive bool

	// When set to true causes the default date header to be excluded from the response.
	// Default: false
	DisableDefaultDate bool

	// When set to true, causes the default Content-Type header to be excluded from the Response.
	// Default: false
	DisableDefaultContentType bool

	// By default all header names are normalized: conteNT-tYPE -> Content-Type
	// Default: false
	DisableHeaderNormalizing bool

	// When set to true, it will not print out the «Fiber» ASCII art and listening address
	// Default: false
	DisableStartupMessage bool

	// Templates is the interface that wraps the Render function.
	// Default: nil
	Templates Templates

	// The amount of time allowed to read the full request including body.
	// Default: unlimited
	ReadTimeout time.Duration

	// The maximum duration before timing out writes of the response.
	// Default: unlimited
	WriteTimeout time.Duration

	// The maximum amount of time to wait for the next request when keep-alive is enabled.
	// Default: unlimited
	IdleTimeout time.Duration

	// TODO: v1.11
	// The router executes the same handler by default if StrictRouting or CaseSensitive is disabled.
	// Enabling RedirectFixedPath will change this behaviour into a client redirect to the original route path.
	// Using the status code 301 for GET requests and 308 for all other request methods.
	// RedirectFixedPath bool
}

// Static struct
type Static struct {
	// This works differently than the github.com/gofiber/compression middleware
	// The server tries minimizing CPU usage by caching compressed files.
	// Optional. Default value false
	Compress bool

	// Enables byte range requests if set to true.
	// Optional. Default value false
	ByteRange bool

	// Enable directory browsing.
	// Optional. Default value false.
	Browse bool

	// Index file for serving a directory.
	// Optional. Default value "index.html".
	Index string
}

// Error represents an error that occurred while handling a request.
type Error struct {
	Code    int
	Message string
}

// Error makes it compatible with `error` interface.
func (e *Error) Error() string {
	return e.Message
}

// NewError creates a new HTTPError instance.
func NewError(code int, message ...string) *Error {
	e := &Error{code, utils.StatusMessage(code)}
	if len(message) > 0 {
		e.Message = message[0]
	}
	return e
}

// Routes returns all registered routes
//
// for _, r := range app.Routes() {
// 	fmt.Printf("%s\t%s\n", r.Method, r.Path)
// }
func (app *App) Routes() []*Route {
	routes := make([]*Route, 0)
	for m := range app.stack {
		for r := range app.stack[m] {
			// Ignore HEAD routes handling GET routes
			if m == 1 && app.stack[m][r].Method == MethodGet {
				continue
			}
			routes = append(routes, app.stack[m][r])
		}
	}
	// Sort routes by stack position
	sort.Slice(routes, func(i, k int) bool {
		return routes[i].pos < routes[k].pos
	})
	return routes
}

// New creates a new Fiber named instance.
// You can pass optional settings when creating a new instance.
func New(settings ...*Settings) *App {
	// Create a new app
	app := &App{
		// Create router stack
		stack: make([][]*Route, len(methodINT)),
		// Create Ctx pool
		pool: sync.Pool{
			New: func() interface{} {
				return new(Ctx)
			},
		},
		// Set default settings
		Settings: &Settings{
			Prefork:     utils.GetArgument("-prefork"),
			BodyLimit:   4 * 1024 * 1024,
			Concurrency: 256 * 1024,
			ErrorHandler: func(ctx *Ctx) {
				code := StatusInternalServerError
				if e, ok := ctx.Error().(*Error); ok {
					code = e.Code
				}
				ctx.Status(code).SendString(ctx.Error().Error())
			},
		},
	}

	// Overwrite settings if provided
	if len(settings) > 0 {
		app.Settings = settings[0]
		if !app.Settings.Prefork { // Default to -prefork flag if false
			app.Settings.Prefork = utils.GetArgument("-prefork")
		}
		if app.Settings.BodyLimit <= 0 {
			app.Settings.BodyLimit = 4 * 1024 * 1024
		}
		if app.Settings.Concurrency <= 0 {
			app.Settings.Concurrency = 256 * 1024
		}
		// Replace unsafe conversion functions
		if app.Settings.Immutable {
			getBytes = getBytesImmutable
			getString = getStringImmutable
		}
		// Set default error handler
		if app.Settings.ErrorHandler == nil {
			app.Settings.ErrorHandler = func(ctx *Ctx) {
				code := StatusInternalServerError
				if e, ok := ctx.Error().(*Error); ok {
					code = e.Code
				}
				ctx.Status(code).SendString(ctx.Error().Error())
			}
		}
	}
	// Initialize app
	return app.init()
}

// Use registers a middleware route.
// Middleware matches requests beginning with the provided prefix.
// Providing a prefix is optional, it defaults to "/".
//
// - app.Use(handler)
// - app.Use("/api", handler)
// - app.Use("/api", handler, handler)
func (app *App) Use(args ...interface{}) *Route {
	var prefix string
	var handlers []Handler

	for i := 0; i < len(args); i++ {
		switch arg := args[i].(type) {
		case string:
			prefix = arg
		case Handler:
			handlers = append(handlers, arg)
		default:
			log.Fatalf("Use: Invalid Handler %v", reflect.TypeOf(arg))
		}
	}
	return app.register("USE", prefix, handlers...)
}

// Get ...
func (app *App) Get(path string, handlers ...Handler) *Route {
	return app.Add(MethodGet, path, handlers...)
}

// Head ...
func (app *App) Head(path string, handlers ...Handler) *Route {
	return app.Add(MethodHead, path, handlers...)
}

// Post ...
func (app *App) Post(path string, handlers ...Handler) *Route {
	return app.Add(MethodPost, path, handlers...)
}

// Put ...
func (app *App) Put(path string, handlers ...Handler) *Route {
	return app.Add(MethodPut, path, handlers...)
}

// Delete ...
func (app *App) Delete(path string, handlers ...Handler) *Route {
	return app.Add(MethodDelete, path, handlers...)
}

// Connect ...
func (app *App) Connect(path string, handlers ...Handler) *Route {
	return app.Add(MethodConnect, path, handlers...)
}

// Options ...
func (app *App) Options(path string, handlers ...Handler) *Route {
	return app.Add(MethodOptions, path, handlers...)
}

// Trace ...
func (app *App) Trace(path string, handlers ...Handler) *Route {
	return app.Add(MethodTrace, path, handlers...)
}

// Patch ...
func (app *App) Patch(path string, handlers ...Handler) *Route {
	return app.Add(MethodPatch, path, handlers...)
}

// Add ...
func (app *App) Add(method, path string, handlers ...Handler) *Route {
	return app.register(method, path, handlers...)
}

// Static ...
func (app *App) Static(prefix, root string, config ...Static) *Route {
	return app.registerStatic(prefix, root, config...)
}

// All ...
func (app *App) All(path string, handlers ...Handler) []*Route {
	routes := make([]*Route, len(methodINT))
	for method, i := range methodINT {
		routes[i] = app.Add(method, path, handlers...)
	}
	return routes
}

// Group is used for Routes with common prefix to define a new sub-router with optional middleware.
func (app *App) Group(prefix string, handlers ...Handler) *Group {
	if len(handlers) > 0 {
		app.register("USE", prefix, handlers...)
	}
	return &Group{prefix: prefix, app: app}
}

// Serve can be used to pass a custom listener
// This method does not support the Prefork feature
// Preforkin is not available using app.Serve(ln net.Listener)
// You can pass an optional *tls.Config to enable TLS.
func (app *App) Serve(ln net.Listener, tlsconfig ...*tls.Config) error {
	// Update fiber server settings
	app.init()
	// TLS config
	if len(tlsconfig) > 0 {
		ln = tls.NewListener(ln, tlsconfig[0])
	}
	// Print startup message
	if !app.Settings.DisableStartupMessage {
		fmt.Printf("        _______ __\n  ____ / ____(_) /_  ___  _____\n_____ / /_  / / __ \\/ _ \\/ ___/\n  __ / __/ / / /_/ /  __/ /\n    /_/   /_/_.___/\\___/_/ v%s\n", Version)
		fmt.Printf("Started listening on %s\n", ln.Addr().String())
	}

	return app.server.Serve(ln)
}

// Listen serves HTTP requests from the given addr or port.
// You can pass an optional *tls.Config to enable TLS.
func (app *App) Listen(address interface{}, tlsconfig ...*tls.Config) error {
	addr, ok := address.(string)
	if !ok {
		port, ok := address.(int)
		if !ok {
			return fmt.Errorf("Listen: Host must be an INT port or STRING address")
		}
		addr = strconv.Itoa(port)
	}
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	// Update fiber server settings
	app.init()
	// Setup listener
	var ln net.Listener
	var err error
	// Prefork enabled, not available on windows
	if app.Settings.Prefork && runtime.NumCPU() > 1 && runtime.GOOS != "windows" {
		if ln, err = app.prefork(addr); err != nil {
			return err
		}
	} else {
		if ln, err = net.Listen("tcp4", addr); err != nil {
			return err
		}
	}
	// TLS config
	if len(tlsconfig) > 0 {
		ln = tls.NewListener(ln, tlsconfig[0])
	}
	// Print startup message
	if !app.Settings.DisableStartupMessage && !utils.GetArgument("-child") {
		fmt.Printf("        _______ __\n  ____ / ____(_) /_  ___  _____\n_____ / /_  / / __ \\/ _ \\/ ___/\n  __ / __/ / / /_/ /  __/ /\n    /_/   /_/_.___/\\___/_/ v%s\n", Version)
		fmt.Printf("Started listening on %s\n", ln.Addr().String())
	}

	return app.server.Serve(ln)
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
// Shutdown works by first closing all open listeners and then waiting indefinitely for all connections to return to idle and then shut down.
//
// When Shutdown is called, Serve, ListenAndServe, and ListenAndServeTLS immediately return nil.
// Make sure the program doesn't exit and waits instead for Shutdown to return.
//
// Shutdown does not close keepalive connections so its recommended to set ReadTimeout to something else than 0.
func (app *App) Shutdown() error {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	if app.server == nil {
		return fmt.Errorf("Server is not running")
	}
	return app.server.Shutdown()
}

// Test is used for internal debugging by passing a *http.Request
// Timeout is optional and defaults to 1s, -1 will disable it completely.
func (app *App) Test(request *http.Request, msTimeout ...int) (*http.Response, error) {
	timeout := 1000 // 1 second default
	if len(msTimeout) > 0 {
		timeout = msTimeout[0]
	}
	// Add Content-Length if not provided with body
	if request.Body != http.NoBody && request.Header.Get("Content-Length") == "" {
		request.Header.Add("Content-Length", strconv.FormatInt(request.ContentLength, 10))
	}
	// Dump raw http request
	dump, err := httputil.DumpRequest(request, true)
	if err != nil {
		return nil, err
	}
	// Update server settings
	app.init()
	// Create test connection
	conn := new(testConn)
	// Write raw http request
	if _, err = conn.r.Write(dump); err != nil {
		return nil, err
	}
	// Serve conn to server
	channel := make(chan error)
	go func() {
		channel <- app.server.ServeConn(conn)
	}()
	// Wait for callback
	if timeout >= 0 {
		// With timeout
		select {
		case err = <-channel:
		case <-time.After(time.Duration(timeout) * time.Millisecond):
			return nil, fmt.Errorf("Timeout error %vms", timeout)
		}
	} else {
		// Without timeout
		err = <-channel
	}
	// Check for errors
	if err != nil {
		return nil, err
	}
	// Read response
	buffer := bufio.NewReader(&conn.w)
	// Convert raw http response to *http.Response
	resp, err := http.ReadResponse(buffer, request)
	if err != nil {
		return nil, err
	}
	// Return *http.Response
	return resp, nil
}

// Sharding: https://www.nginx.com/blog/socket-sharding-nginx-release-1-9-1/
func (app *App) prefork(address string) (ln net.Listener, err error) {
	// Master proc
	if !utils.GetArgument("-child") {
		addr, err := net.ResolveTCPAddr("tcp", address)
		if err != nil {
			return ln, err
		}
		tcplistener, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return ln, err
		}
		fl, err := tcplistener.File()
		if err != nil {
			return ln, err
		}
		files := []*os.File{fl}
		childs := make([]*exec.Cmd, runtime.NumCPU()/2)
		// #nosec G204
		for i := range childs {
			childs[i] = exec.Command(os.Args[0], append(os.Args[1:], "-prefork", "-child")...)
			childs[i].Stdout = os.Stdout
			childs[i].Stderr = os.Stderr
			childs[i].ExtraFiles = files
			if err := childs[i].Start(); err != nil {
				return ln, err
			}
		}

		for k := range childs {
			if err := childs[k].Wait(); err != nil {
				return ln, err
			}
		}
		os.Exit(0)
	} else {
		// 1 core per child
		runtime.GOMAXPROCS(1)
		ln, err = net.FileListener(os.NewFile(3, ""))
	}
	return ln, err
}

type disableLogger struct{}

func (dl *disableLogger) Printf(format string, args ...interface{}) {
	// fmt.Println(fmt.Sprintf(format, args...))
}

func (app *App) init() *App {
	app.mutex.Lock()
	if app.server == nil {
		app.server = &fasthttp.Server{
			Logger:       &disableLogger{},
			LogAllErrors: false,
			ErrorHandler: func(fctx *fasthttp.RequestCtx, err error) {
				ctx := app.AcquireCtx(fctx)
				if _, ok := err.(*fasthttp.ErrSmallBuffer); ok {
					ctx.err = ErrRequestHeaderFieldsTooLarge
				} else if netErr, ok := err.(*net.OpError); ok && netErr.Timeout() {
					ctx.err = ErrRequestTimeout
				} else if len(err.Error()) == 33 && err.Error() == "body size exceeds the given limit" {
					ctx.err = ErrRequestEntityTooLarge
				} else {
					ctx.err = ErrBadRequest
				}
				app.Settings.ErrorHandler(ctx) // ctx.Route() not available
				app.ReleaseCtx(ctx)
			},
		}
	}
	if app.server.Handler == nil {
		app.server.Handler = app.handler
	}
	app.server.Name = app.Settings.ServerHeader
	app.server.Concurrency = app.Settings.Concurrency
	app.server.NoDefaultDate = app.Settings.DisableDefaultDate
	app.server.NoDefaultContentType = app.Settings.DisableDefaultContentType
	app.server.DisableHeaderNamesNormalizing = app.Settings.DisableHeaderNormalizing
	app.server.DisableKeepalive = app.Settings.DisableKeepalive
	app.server.MaxRequestBodySize = app.Settings.BodyLimit
	app.server.NoDefaultServerHeader = app.Settings.ServerHeader == ""
	app.server.ReadTimeout = app.Settings.ReadTimeout
	app.server.WriteTimeout = app.Settings.WriteTimeout
	app.server.IdleTimeout = app.Settings.IdleTimeout
	app.mutex.Unlock()
	return app
}
