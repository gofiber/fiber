// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

// Package fiber
// Fiber is an Express inspired web framework built on top of Fasthttp,
// the fastest HTTP engine for Go. Designed to ease things up for fast
// development with zero memory allocation and performance in mind.

package fiber

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	utils "github.com/gofiber/utils"
	colorable "github.com/mattn/go-colorable"
	isatty "github.com/mattn/go-isatty"
	fasthttp "github.com/valyala/fasthttp"
)

// Version of current package
const Version = "1.14.6"

// Map is a shortcut for map[string]interface{}, useful for JSON returns
type Map map[string]interface{}

// Handler defines a function to serve HTTP requests.
type Handler = func(*Ctx)

// Error represents an error that occurred while handling a request.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// App denotes the Fiber application.
type App struct {
	out   io.Writer
	mutex sync.Mutex
	// Route stack divided by HTTP methods
	stack [][]*Route
	// Route stack divided by HTTP methods and route prefixes
	treeStack []map[string][]*Route
	// Amount of registered routes
	routesCount int
	// Amount of registered handlers
	handlerCount int
	// Ctx pool
	pool sync.Pool
	// Fasthttp server
	server *fasthttp.Server
	// App settings
	Settings *Settings `json:"settings"`
}

// Settings is a struct holding the server settings.
type Settings struct {
	// ErrorHandler is executed when you pass an error in the Next(err) method.
	// This function is also executed when middleware.Recover() catches a panic
	// Default: func(ctx *Ctx, err error) {
	// 	code := StatusInternalServerError
	// 	if e, ok := err.(*Error); ok {
	// 		code = e.Code
	// 	}
	// 	ctx.Set(HeaderContentType, MIMETextPlainCharsetUTF8)
	// 	ctx.Status(code).SendString(err.Error())
	// }
	ErrorHandler func(*Ctx, error) `json:"-"`

	// Enables the "Server: value" HTTP header.
	// Default: ""
	ServerHeader string `json:"server_header"`

	// When set to true, the router treats "/foo" and "/foo/" as different.
	// By default this is disabled and both "/foo" and "/foo/" will execute the same handler.
	StrictRouting bool `json:"strict_routing"`

	// When set to true, enables case sensitive routing.
	// E.g. "/FoO" and "/foo" are treated as different routes.
	// By default this is disabled and both "/FoO" and "/foo" will execute the same handler.
	CaseSensitive bool `json:"case_sensitive"`

	// When set to true, this relinquishes the 0-allocation promise in certain
	// cases in order to access the handler values (e.g. request bodies) in an
	// immutable fashion so that these values are available even if you return
	// from handler.
	// Default: false
	Immutable bool `json:"immutable"`

	// When set to true, converts all encoded characters in the route back
	// before setting the path for the context, so that the routing can also
	// work with urlencoded special characters.
	// Default: false
	UnescapePath bool `json:"unescape_path"`

	// Enable or disable ETag header generation, since both weak and strong etags are generated
	// using the same hashing method (CRC-32). Weak ETags are the default when enabled.
	// Default: false
	ETag bool `json:"etag"`

	// When set to true, this will spawn multiple Go processes listening on the same port.
	// Default: false
	Prefork bool `json:"prefork"`

	// Max body size that the server accepts.
	// Default: 4 * 1024 * 1024
	BodyLimit int `json:"body_limit"`

	// Maximum number of concurrent connections.
	// Default: 256 * 1024
	Concurrency int `json:"concurrency"`

	// When set to true, disables keep-alive connections.
	// The server will close incoming connections after sending the first response to client.
	// Default: false
	DisableKeepalive bool `json:"disable_keep_alive"`

	// When set to true, causes the default date header to be excluded from the response.
	// Default: false
	DisableDefaultDate bool `json:"disable_default_date"`

	// When set to true, causes the default Content-Type header to be excluded from the response.
	// Default: false
	DisableDefaultContentType bool `json:"disable_default_content_type"`

	// When set to true, disables header normalization.
	// By default all header names are normalized: conteNT-tYPE -> Content-Type.
	// Default: false
	DisableHeaderNormalizing bool `json:"disable_header_normalizing"`

	// When set to true, it will not print out the «Fiber» ASCII art and listening address.
	// Default: false
	DisableStartupMessage bool `json:"disable_startup_message"`

	// Views is the interface that wraps the Render function.
	// Default: nil
	Views Views `json:"-"`

	// The amount of time allowed to read the full request including body.
	// It is reset after the request handler has returned.
	// The connection's read deadline is reset when the connection opens.
	// Default: unlimited
	ReadTimeout time.Duration `json:"read_timeout"`

	// The maximum duration before timing out writes of the response.
	// It is reset after the request handler has returned.
	// Default: unlimited
	WriteTimeout time.Duration `json:"write_timeout"`

	// The maximum amount of time to wait for the next request when keep-alive is enabled.
	// If IdleTimeout is zero, the value of ReadTimeout is used.
	// Default: unlimited
	IdleTimeout time.Duration `json:"idle_timeout"`

	// Per-connection buffer size for requests' reading.
	// This also limits the maximum header size.
	// Increase this buffer if your clients send multi-KB RequestURIs
	// and/or multi-KB headers (for example, BIG cookies).
	// Default: 4096
	ReadBufferSize int `json:"read_buffer_size"`

	// Per-connection buffer size for responses' writing.
	// Default: 4096
	WriteBufferSize int `json:"write_buffer_size"`

	// CompressedFileSuffix adds suffix to the original file name and
	// tries saving the resulting compressed file under the new file name.
	// Default: ".fiber.gz"
	CompressedFileSuffix string `json:"compressed_file_suffix"`

	// FEATURE: v1.13
	// The router executes the same handler by default if StrictRouting or CaseSensitive is disabled.
	// Enabling RedirectFixedPath will change this behaviour into a client redirect to the original route path.
	// Using the status code 301 for GET requests and 308 for all other request methods.
	// RedirectFixedPath bool
}

// Static defines configuration options when defining static assets.
type Static struct {
	// When set to true, the server tries minimizing CPU usage by caching compressed files.
	// This works differently than the github.com/gofiber/compression middleware.
	// Optional. Default value false
	Compress bool

	// When set to true, enables byte range requests.
	// Optional. Default value false
	ByteRange bool

	// When set to true, enables directory browsing.
	// Optional. Default value false.
	Browse bool

	// The name of the index file for serving a directory.
	// Optional. Default value "index.html".
	Index string
}

// default settings
const (
	defaultBodyLimit            = 4 * 1024 * 1024
	defaultConcurrency          = 256 * 1024
	defaultReadBufferSize       = 4096
	defaultWriteBufferSize      = 4096
	defaultCompressedFileSuffix = ".fiber.gz"
)

var defaultErrorHandler = func(ctx *Ctx, err error) {
	code := StatusInternalServerError
	if e, ok := err.(*Error); ok {
		code = e.Code
	}
	ctx.Set(HeaderContentType, MIMETextPlainCharsetUTF8)
	ctx.Status(code).SendString(err.Error())
}

// New creates a new Fiber named instance.
//  myApp := app.New()
// You can pass an optional settings by passing a *Settings struct:
//  myApp := app.New(&fiber.Settings{
//      Prefork: true,
//      ServerHeader: "Fiber",
//  })
func New(settings ...*Settings) *App {
	// Create a new app
	app := &App{
		// Create router stack
		stack:     make([][]*Route, len(intMethod)),
		treeStack: make([]map[string][]*Route, len(intMethod)),
		// Create Ctx pool
		pool: sync.Pool{
			New: func() interface{} {
				return new(Ctx)
			},
		},
		// Set settings
		Settings: &Settings{},
	}

	// Overwrite settings if provided
	if len(settings) > 0 {
		app.Settings = settings[0]
	}

	if app.Settings.BodyLimit <= 0 {
		app.Settings.BodyLimit = defaultBodyLimit
	}
	if app.Settings.Concurrency <= 0 {
		app.Settings.Concurrency = defaultConcurrency
	}
	if app.Settings.ReadBufferSize <= 0 {
		app.Settings.ReadBufferSize = defaultReadBufferSize
	}
	if app.Settings.WriteBufferSize <= 0 {
		app.Settings.WriteBufferSize = defaultWriteBufferSize
	}
	if app.Settings.CompressedFileSuffix == "" {
		app.Settings.CompressedFileSuffix = defaultCompressedFileSuffix
	}
	if app.Settings.ErrorHandler == nil {
		app.Settings.ErrorHandler = defaultErrorHandler
	}
	if app.Settings.Immutable {
		getBytes, getString = getBytesImmutable, getStringImmutable
	}

	// Return app
	return app
}

// Use registers a middleware route.
// Middleware matches requests beginning with the provided prefix.
// Providing a prefix is optional, it defaults to "/".
//
//  app.Use(handler)
//  app.Use("/api", handler)
//  app.Use("/api", handler, handler)
func (app *App) Use(args ...interface{}) Router {
	var prefix string
	var handlers []Handler

	for i := 0; i < len(args); i++ {
		switch arg := args[i].(type) {
		case string:
			prefix = arg
		case Handler:
			handlers = append(handlers, arg)
		default:
			panic(fmt.Sprintf("use: invalid handler %v\n", reflect.TypeOf(arg)))
		}
	}
	app.register(methodUse, prefix, handlers...)
	return app
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (app *App) Get(path string, handlers ...Handler) Router {
	route := app.register(MethodGet, path, handlers...)
	// Add HEAD route
	headRoute := route
	app.addRoute(MethodHead, &headRoute)

	return app
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (app *App) Head(path string, handlers ...Handler) Router {
	return app.Add(MethodHead, path, handlers...)
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (app *App) Post(path string, handlers ...Handler) Router {
	return app.Add(MethodPost, path, handlers...)
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (app *App) Put(path string, handlers ...Handler) Router {
	return app.Add(MethodPut, path, handlers...)
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (app *App) Delete(path string, handlers ...Handler) Router {
	return app.Add(MethodDelete, path, handlers...)
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (app *App) Connect(path string, handlers ...Handler) Router {
	return app.Add(MethodConnect, path, handlers...)
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (app *App) Options(path string, handlers ...Handler) Router {
	return app.Add(MethodOptions, path, handlers...)
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the path to the target resource.
func (app *App) Trace(path string, handlers ...Handler) Router {
	return app.Add(MethodTrace, path, handlers...)
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (app *App) Patch(path string, handlers ...Handler) Router {
	return app.Add(MethodPatch, path, handlers...)
}

// Add ...
func (app *App) Add(method, path string, handlers ...Handler) Router {
	app.register(method, path, handlers...)
	return app
}

// Static ...
func (app *App) Static(prefix, root string, config ...Static) Router {
	app.registerStatic(prefix, root, config...)
	return app
}

// All ...
func (app *App) All(path string, handlers ...Handler) Router {
	for _, method := range intMethod {
		app.Add(method, path, handlers...)
	}
	return app
}

// Group is used for Routes with common prefix to define a new sub-router with optional middleware.
func (app *App) Group(prefix string, handlers ...Handler) Router {
	if len(handlers) > 0 {
		app.register(methodUse, prefix, handlers...)
	}
	return &Group{prefix: prefix, app: app}
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
//  for _, r := range app.Routes() {
//  	fmt.Printf("%s\t%s\n", r.Method, r.Path)
//  }
func (app *App) Routes() []*Route {
	fmt.Println("routes is deprecated since v1.13.2, please use `app.Stack()` to access the raw router stack")
	routes := make([]*Route, 0)
	for m := range app.stack {
	stackLoop:
		for r := range app.stack[m] {
			// Don't duplicate USE routesCount
			if app.stack[m][r].use {
				for i := range routes {
					if routes[i].use && routes[i].Path == app.stack[m][r].Path {
						continue stackLoop
					}
				}
			}
			routes = append(routes, app.stack[m][r])
		}
	}
	return routes
}

// Listener can be used to pass a custom listener.
// You can pass an optional *tls.Config to enable TLS.
// This method does not support the Prefork feature
// To use Prefork, please use app.Listen()
func (app *App) Listener(ln net.Listener, tlsconfig ...*tls.Config) error {
	// Update server settings
	app.init()
	// TLS config
	if len(tlsconfig) > 0 {
		ln = tls.NewListener(ln, tlsconfig[0])
	}
	// Print startup message
	if !app.Settings.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), len(tlsconfig) > 0, "")
	}
	return app.server.Serve(ln)
}

// Listen serves HTTP requests from the given addr or port.
// You can pass an optional *tls.Config to enable TLS.
//
//  app.Listen(8080)
//  app.Listen("8080")
//  app.Listen(":8080")
//  app.Listen("127.0.0.1:8080")
func (app *App) Listen(address interface{}, tlsconfig ...*tls.Config) error {
	// Convert address to string
	addr, ok := address.(string)
	if !ok {
		port, ok := address.(int)
		if !ok {
			return fmt.Errorf("listen: host must be an `int` port or `string` address")
		}
		addr = strconv.Itoa(port)
	}
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	// Update server settings
	app.init()
	// Start prefork
	if app.Settings.Prefork {
		return app.prefork(addr, tlsconfig...)
	}
	// Set correct network protocol
	network := "tcp4"
	if isIPv6(addr) {
		network = "tcp6"
	}
	// Setup listener
	ln, err := net.Listen(network, addr)
	if err != nil {
		return err
	}
	// Add TLS config if provided
	if len(tlsconfig) > 0 {
		ln = tls.NewListener(ln, tlsconfig[0])
	}
	// Print startup message
	if !app.Settings.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), len(tlsconfig) > 0, "")
	}
	// Start listening
	return app.server.Serve(ln)
}

// Handler returns the server handler.
func (app *App) Handler() fasthttp.RequestHandler {
	app.init()
	return app.handler
}

// Handler returns the server handler.
func (app *App) Stack() [][]*Route {
	return app.stack
}

// Shutdown gracefully
// shuts down the server without interrupting any active connections.
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
		return fmt.Errorf("shutdown: server is not running")
	}
	return app.server.Shutdown()
}

// Test is used for internal debugging by passing a *http.Request.
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
			return nil, fmt.Errorf("test: timeout error %vms", timeout)
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

type disableLogger struct{}

func (dl *disableLogger) Printf(format string, args ...interface{}) {
	// fmt.Println(fmt.Sprintf(format, args...))
}

func (app *App) init() *App {
	// Lock application
	app.mutex.Lock()
	defer app.mutex.Unlock()

	// Load view engine if provided
	if app.Settings != nil {
		// Only load templates if an view engine is specified
		if app.Settings.Views != nil {
			if err := app.Settings.Views.Load(); err != nil {
				fmt.Printf("views: %v\n", err)
			}
		}
	}
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
				app.Settings.ErrorHandler(ctx, ctx.err)
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
	app.server.ReadBufferSize = app.Settings.ReadBufferSize
	app.server.WriteBufferSize = app.Settings.WriteBufferSize
	app.buildTree()
	return app
}

const (
	cBlack = "\u001b[90m"
	cRed   = "\u001b[91m"
	// cGreen  = "\u001b[92m"
	// cYellow = "\u001b[93m"
	// cBlue    = "\u001b[94m"
	// cMagenta = "\u001b[95m"
	cCyan = "\u001b[96m"
	// cWhite   = "\u001b[97m"
	cReset = "\u001b[0m"
)

func (app *App) startupMessage(addr string, tls bool, pids string) {
	// ignore child processes
	if app.IsChild() {
		return
	}
	// ascii logo
	var logo string
	logo += `%s        _______ __                 %s` + "\n"
	logo += `%s  ____%s / ____(_) /_  ___  _____  %s` + "\n"
	logo += `%s_____%s / /_  / / __ \/ _ \/ ___/  %s` + "\n"
	logo += `%s  __%s / __/ / / /_/ /  __/ /      %s` + "\n"
	logo += `%s    /_/   /_/_.___/\___/_/%s %s` + ""
	logo += cRed + "v2 will be released on 15 September 2020!\nPlease visit https://gofiber.io/v2 for more information.\n" + cReset
	host, port := parseAddr(addr)
	padding := strconv.Itoa(len(host))
	if len(host) <= 4 {
		padding = "5"
	}
	var (
		tlsStr       = "FALSE"
		preforkStr   = "FALSE"
		handlerCount = strconv.Itoa(app.handlerCount)
		osName       = utils.ToUpper(runtime.GOOS)
		cpuThreads   = runtime.NumCPU()
		pid          = os.Getpid()
	)
	if host == "" {
		host = "0.0.0.0"
	}
	if tls {
		tlsStr = "TRUE"
	}
	if app.Settings.Prefork {
		preforkStr = "TRUE"
	}
	// tabwriter makes sure the spacing are consistent across different values
	// colorable handles the escape sequence for stdout using ascii color codes
	host = fmt.Sprintf("%-"+padding+"s", host)
	port = fmt.Sprintf("%-"+padding+"s", port)
	tlsStr = fmt.Sprintf("%-"+padding+"s", tlsStr)
	handlerCount = fmt.Sprintf("%-"+padding+"s", handlerCount)

	app.out = colorable.NewColorableStdout()
	// Check if colors are supported
	if os.Getenv("TERM") == "dumb" ||
		(!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd())) {
		app.out = colorable.NewNonColorable(os.Stdout)
	}
	// simple Sprintf function that defaults back to black
	cyan := func(v interface{}) string {
		return fmt.Sprintf("%s%v%s", cCyan, v, cBlack)
	}
	// Build startup banner
	fmt.Fprintf(app.out, logo, cBlack, cBlack,
		cCyan, cBlack, fmt.Sprintf(" HOST     %s  OS      %s", cyan(host), cyan(osName)),
		cCyan, cBlack, fmt.Sprintf(" PORT     %s  THREADS %s", cyan(port), cyan(cpuThreads)),
		cCyan, cBlack, fmt.Sprintf(" TLS      %s  PREFORK %s", cyan(tlsStr), cyan(preforkStr)),
		cBlack, cyan(Version), fmt.Sprintf(" HANDLERS %s  PID     %s%s%s\n", cyan(handlerCount), cyan(pid), pids, cReset),
	)
}
