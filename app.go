// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

// Package fiber is an Express inspired web framework built on top of Fasthttp,
// the fastest HTTP engine for Go. Designed to ease things up for fast
// development with zero memory allocation and performance in mind.
package fiber

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"encoding/json"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofiber/fiber/v3/eventemitter"
	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
)

// Version of current fiber package
const Version = "3.0.0-alpha.1"

// Handler defines a function to serve HTTP requests.
type Handler = func(*Ctx) error

// Map is a shortcut for map[string]any, useful for JSON returns
type Map map[string]any

// Storage interface for communicating with different database/key-value
// providers
type Storage interface {
	// Get gets the value for the given key.
	// `nil, nil` is returned when the key does not exist
	Get(key string) ([]byte, error)

	// Set stores the given value for the given key along
	// with an expiration value, 0 means no expiration.
	// Empty key or value will be ignored without an error.
	Set(key string, val []byte, exp time.Duration) error

	// Delete deletes the value for the given key.
	// It returns no error if the storage does not contain the key,
	Delete(key string) error

	// Reset resets the storage and delete all keys.
	Reset() error

	// Close closes the storage and will stop any running garbage
	// collectors and open connections.
	Close() error
}

// ErrorHandler defines a function that will process all errors
// returned from any handlers in the stack
//
//	cfg := fiber.Config{}
//	cfg.ErrorHandler = func(c *Ctx, err error) error {
//	 code := StatusInternalServerError
//	 var e *fiber.Error
//	 if errors.As(err, &e) {
//	   code = e.Code
//	 }
//	 c.Set(HeaderContentType, MIMETextPlainCharsetUTF8)
//	 return c.Status(code).SendString(err.Error())
//	}
//	app := fiber.New(cfg)
type ErrorHandler = func(*Ctx, error) error

// Error represents an error that occurred while handling a request.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// App denotes the Fiber application.
type App struct {
	mutex sync.Mutex
	// Route stack divided by HTTP methods
	stack [][]*Route
	// Route stack divided by HTTP methods and route prefixes
	treeStack []map[string][]*Route
	// contains the information if the route stack has been changed to build the optimized tree
	routesRefreshed bool
	// Amount of registered routes
	routesCount uint32
	// Amount of registered handlers
	handlersCount uint32
	// Ctx pool
	pool sync.Pool
	// Fasthttp server
	server *fasthttp.Server
	// App config
	config Config
	// Converts string to a byte slice
	getBytes func(s string) (b []byte)
	// Converts byte slice to a string
	getString func(b []byte) string
	// If application is a parent, It returns nil. It can accessible only from sub app
	parent *App
	// Returns the canonical path of the app, a string.
	path string
	// Mounted sub app's path
	mountpath string
	// Mounted sub apps
	subList map[string]*App
	// Registered routers
	routerList map[string]*Router
	// The app.Locals has properties that are local variables within the application, and will be available in templates rendered with ctx.Render.
	Locals map[string]any
	// Registered engines
	engineList map[string]TemplateEngine
	// ErrorHandler is executed when an error is returned from fiber.Handler.
	//
	// Default: DefaultErrorHandler
	errorHandler ErrorHandler `json:"-"`
	// Eventemitter
	eventEmitter *eventemitter.Emitter
}

// Config is a struct holding the server settings.
type Config struct {
	// When set to true, this will spawn multiple Go processes listening on the same port.
	//
	// Default: false
	Prefork bool `json:"prefork"`

	// Enables the "Server: value" HTTP header.
	//
	// Default: ""
	ServerHeader string `json:"server_header"`

	// When set to true, the router treats "/foo" and "/foo/" as different.
	// By default this is disabled and both "/foo" and "/foo/" will execute the same handler.
	//
	// Default: false
	Strict bool `json:"strict"`

	// When set to true, enables case sensitive routing.
	// E.g. "/FoO" and "/foo" are treated as different routes.
	// By default this is disabled and both "/FoO" and "/foo" will execute the same handler.
	//
	// Default: false
	CaseSensitive bool `json:"case_sensitive"`

	// When set to true, this relinquishes the 0-allocation promise in certain
	// cases in order to access the handler values (e.g. request bodies) in an
	// immutable fashion so that these values are available even if you return
	// from handler.
	//
	// Default: false
	Immutable bool `json:"immutable"`

	// When set to true, converts all encoded characters in the route back
	// before setting the path for the context, so that the routing,
	// the returning of the current url from the context `ctx.Path()`
	// and the parameters `ctx.Params(%key%)` with decoded characters will work
	//
	// Default: false
	UnescapePath bool `json:"unescape_path"`

	// Max body size that the server accepts.
	// -1 will decline any body size
	//
	// Default: 4 * 1024 * 1024
	BodyLimit int `json:"body_limit"`

	// Maximum number of concurrent connections.
	//
	// Default: 256 * 1024
	Concurrency int `json:"concurrency"`

	// PassLocalsToViews Enables passing of the locals set on a fiber.Ctx to the template engine
	//
	// Default: false
	PassLocalsToViews bool `json:"pass_locals_to_views"`

	// The amount of time allowed to read the full request including body.
	// It is reset after the request handler has returned.
	// The connection's read deadline is reset when the connection opens.
	//
	// Default: unlimited
	ReadTimeout time.Duration `json:"read_timeout"`

	// The maximum duration before timing out writes of the response.
	// It is reset after the request handler has returned.
	//
	// Default: unlimited
	WriteTimeout time.Duration `json:"write_timeout"`

	// The maximum amount of time to wait for the next request when keep-alive is enabled.
	// If IdleTimeout is zero, the value of ReadTimeout is used.
	//
	// Default: unlimited
	IdleTimeout time.Duration `json:"idle_timeout"`

	// Per-connection buffer size for requests' reading.
	// This also limits the maximum header size.
	// Increase this buffer if your clients send multi-KB RequestURIs
	// and/or multi-KB headers (for example, BIG cookies).
	//
	// Default: 4096
	ReadBufferSize int `json:"read_buffer_size"`

	// Per-connection buffer size for responses' writing.
	//
	// Default: 4096
	WriteBufferSize int `json:"write_buffer_size"`

	// CompressedFileSuffix adds suffix to the original file name and
	// tries saving the resulting compressed file under the new file name.
	//
	// Default: ".fiber.gz"
	CompressedFileSuffix string `json:"compressed_file_suffix"`

	// ProxyHeader will enable c.IP() to return the value of the given header key
	// By default c.IP() will return the Remote IP from the TCP connection
	// This property can be useful if you are behind a load balancer: X-Forwarded-*
	// NOTE: headers are easily spoofed and the detected IP addresses are unreliable.
	//
	// Default: ""
	ProxyHeader string `json:"proxy_header"`

	// GETOnly rejects all non-GET requests if set to true.
	// This option is useful as anti-DoS protection for servers
	// accepting only GET requests. The request size is limited
	// by ReadBufferSize if GETOnly is set.
	//
	// Default: false
	GETOnly bool `json:"get_only"`

	// When set to true, disables keep-alive connections.
	// The server will close incoming connections after sending the first response to client.
	//
	// Default: false
	DisableKeepalive bool `json:"disable_keepalive"`

	// When set to true, causes the default date header to be excluded from the response.
	//
	// Default: false
	DisableDefaultDate bool `json:"disable_default_date"`

	// When set to true, causes the default Content-Type header to be excluded from the response.
	//
	// Default: false
	DisableDefaultContentType bool `json:"disable_default_content_type"`

	// When set to true, disables header normalization.
	// By default all header names are normalized: conteNT-tYPE -> Content-Type.
	//
	// Default: false
	DisableHeaderNormalizing bool `json:"disable_header_normalizing"`

	// When set to true, it will not print out the «Fiber» ASCII art and listening address.
	//
	// Default: false
	DisableStartupMessage bool `json:"disable_startup_message"`

	// This function allows to setup app name for the app
	//
	// Default: nil
	AppName string `json:"app_name"`

	// StreamRequestBody enables request body streaming,
	// and calls the handler sooner when given body is
	// larger then the current limit.
	StreamRequestBody bool

	// Will not pre parse Multipart Form data if set to true.
	//
	// This option is useful for servers that desire to treat
	// multipart form data as a binary blob, or choose when to parse the data.
	//
	// Server pre parses multipart form data by default.
	DisablePreParseMultipartForm bool

	// Aggressively reduces memory usage at the cost of higher CPU usage
	// if set to true.
	//
	// Try enabling this option only if the server consumes too much memory
	// serving mostly idle keep-alive connections. This may reduce memory
	// usage by more than 50%.
	//
	// Default: false
	ReduceMemoryUsage bool `json:"reduce_memory_usage"`

	// FEATURE: v2.3.x
	// The router executes the same handler by default if Strict or CaseSensitive is disabled.
	// Enabling RedirectFixedPath will change this behaviour into a client redirect to the original route path.
	// Using the status code 301 for GET requests and 308 for all other request methods.
	//
	// Default: false
	// RedirectFixedPath bool

	// When set by an external client of Fiber it will use the provided implementation of a
	// JSONMarshal
	//
	// Allowing for flexibility in using another json library for encoding
	// Default: json.Marshal
	JSONEncoder utils.JSONMarshal `json:"-"`

	// When set by an external client of Fiber it will use the provided implementation of a
	// JSONUnmarshal
	//
	// Allowing for flexibility in using another json library for decoding
	// Default: json.Unmarshal
	JSONDecoder utils.JSONUnmarshal `json:"-"`

	// When set by an external client of Fiber it will use the provided implementation of a
	// CBORUnmarshal
	//
	// Allowing for flexibility in using another cbor library for decoding
	// Default: cbor.Unmarshal
	CBORDecoder utils.CBORUnmarshal `json:"-"`

	// When set by an external client of Fiber it will use the provided implementation of a
	// CBORMarshal
	//
	// Allowing for flexibility in using another cbor library for encoding
	// Default: cbor.Marshal
	CBOREncoder utils.CBORMarshal `json:"-"`

	// Known networks are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only)
	// WARNING: When prefork is set to true, only "tcp4" and "tcp6" can be chose.
	//
	// Default: NetworkTCP4
	Network string

	// A directory or an array of directories for the application's views.
	// If an array, the views are looked up in the order they occur in the array.
	Views []string `json:"views"`
	// Enables view template compilation caching.
	ViewCache bool `json:"view_cache"`
	// The default engine extension to use when omitted.
	ViewEngine string `json:"view_engine"`

	// If you find yourself behind some sort of proxy, like a load balancer,
	// then certain header information may be sent to you using special X-Forwarded-* headers or the Forwarded header.
	// For example, the Host HTTP header is usually used to return the requested host.
	// But when you’re behind a proxy, the actual host may be stored in an X-Forwarded-Host header.
	//
	// If you are behind a proxy, you should enable TrustedProxyCheck to prevent header spoofing.
	// If you enable EnableTrustedProxyCheck and leave TrustedProxies empty Fiber will skip
	// all headers that could be spoofed.
	// If request ip in TrustedProxies whitelist then:
	//   1. c.Protocol() get value from X-Forwarded-Proto, X-Forwarded-Protocol, X-Forwarded-Ssl or X-Url-Scheme header
	//   2. c.IP() get value from ProxyHeader header.
	//   3. c.Hostname() get value from X-Forwarded-Host header
	// But if request ip NOT in Trusted Proxies whitelist then:
	//   1. c.Protocol() WON't get value from X-Forwarded-Proto, X-Forwarded-Protocol, X-Forwarded-Ssl or X-Url-Scheme header,
	//    will return https in case when tls connection is handled by the app, of http otherwise
	//   2. c.IP() WON'T get value from ProxyHeader header, will return RemoteIP() from fasthttp context
	//   3. c.Hostname() WON'T get value from X-Forwarded-Host header, fasthttp.Request.URI().Host()
	//    will be used to get the hostname.
	//
	// Default: false
	EnableTrustedProxyCheck bool `json:"enable_trusted_proxy_check"`

	// Read EnableTrustedProxyCheck doc.
	//
	// Default: []string
	TrustedProxies     []string `json:"trusted_proxies"`
	trustedProxiesMap  map[string]struct{}
	trustedProxyRanges []*net.IPNet

	// If set to true, will print all routes with their method, path and handler.
	// Default: false
	EnablePrintRoutes bool `json:"enable_print_routes"`

	// You can define custom color scheme. They'll be used for startup message, route list and some middlewares.
	//
	// Optional. Default: DefaultColors
	ColorScheme Colors `json:"color_scheme"`
}

// RouteMessage is some message need to be print when server starts
type RouteMessage struct {
	name     string
	method   string
	path     string
	handlers string
}

// Default Config values
const (
	DefaultBodyLimit            = 4 * 1024 * 1024
	DefaultConcurrency          = 256 * 1024
	DefaultReadBufferSize       = 4096
	DefaultWriteBufferSize      = 4096
	DefaultCompressedFileSuffix = ".fiber.gz"
)

// DefaultErrorHandler that process return errors from handlers
var DefaultErrorHandler = func(c *Ctx, err error) error {
	code := StatusInternalServerError
	var e *Error
	if errors.As(err, &e) {
		code = e.Code
	}
	c.Set(HeaderContentType, MIMETextPlainCharsetUTF8)
	return c.Status(code).SendString(err.Error())
}

// New creates a new Fiber named instance.
//
//	app := fiber.New()
//
// You can pass optional configuration options by passing a Config struct:
//
//	app := fiber.New(fiber.Config{
//	    Prefork: true,
//	    ServerHeader: "Fiber",
//	})
func New(config ...Config) *App {
	// Create a new app
	app := &App{
		// Create router stack
		stack:     make([][]*Route, len(intMethod)),
		treeStack: make([]map[string][]*Route, len(intMethod)),
		// Create Ctx pool
		pool: sync.Pool{
			New: func() any {
				return new(Ctx)
			},
		},
		// Create config
		config:       Config{},
		getBytes:     utils.UnsafeBytes,
		getString:    utils.UnsafeString,
		engineList:   make(map[string]TemplateEngine),
		subList:      make(map[string]*App),
		routerList:   make(map[string]*Router),
		parent:       nil,
		path:         "/",
		mountpath:    "",
		errorHandler: DefaultErrorHandler,
		Locals:       make(map[string]any),
		eventEmitter: eventemitter.New(),
	}

	// Override config if provided
	if len(config) > 0 {
		app.config = config[0]
	}

	// Override default values
	if app.config.BodyLimit == 0 {
		app.config.BodyLimit = DefaultBodyLimit
	}
	if app.config.Concurrency <= 0 {
		app.config.Concurrency = DefaultConcurrency
	}
	if app.config.ReadBufferSize <= 0 {
		app.config.ReadBufferSize = DefaultReadBufferSize
	}
	if app.config.WriteBufferSize <= 0 {
		app.config.WriteBufferSize = DefaultWriteBufferSize
	}
	if app.config.CompressedFileSuffix == "" {
		app.config.CompressedFileSuffix = DefaultCompressedFileSuffix
	}
	if app.config.Immutable {
		app.getBytes, app.getString = getBytesImmutable, getStringImmutable
	}

	if app.config.JSONEncoder == nil {
		app.config.JSONEncoder = json.Marshal
	}
	if app.config.JSONDecoder == nil {
		app.config.JSONDecoder = json.Unmarshal
	}
	if app.config.CBORDecoder == nil {
		app.config.CBORDecoder = cbor.Unmarshal
	}
	if app.config.CBOREncoder == nil {
		app.config.CBOREncoder = cbor.Marshal
	}
	if app.config.Network == "" {
		app.config.Network = NetworkTCP4
	}

	app.config.trustedProxiesMap = make(map[string]struct{}, len(app.config.TrustedProxies))
	for _, ipAddress := range app.config.TrustedProxies {
		app.handleTrustedProxy(ipAddress)
	}

	// Override colors
	app.config.ColorScheme = defaultColors(app.config.ColorScheme)

	// Init app
	app.init()

	// Return app
	return app
}

// Adds an ip address to trustedProxyRanges or trustedProxiesMap based on whether it is an IP range or not
func (app *App) handleTrustedProxy(ipAddress string) {
	if strings.Contains(ipAddress, "/") {
		_, ipNet, err := net.ParseCIDR(ipAddress)

		if err != nil {
			fmt.Printf("[Warning] IP range `%s` could not be parsed. \n", ipAddress)
		}

		app.config.trustedProxyRanges = append(app.config.trustedProxyRanges, ipNet)
	} else {
		app.config.trustedProxiesMap[ipAddress] = struct{}{}
	}
}

type EngineCtx struct {
	Views      []string `json:"views"`
	ViewEngine string   `json:"view_engine"`
	ViewCache  bool     `json:"view_cache"`
}

type EngineCallback = func(*EngineCtx) TemplateEngine

// Registers the given template engine callback as ext.
func (app *App) Engine(ext string, callback EngineCallback) *App {
	if ext == "" {
		panic("engine: ext name cannot be empty")
	}
	app.engineList[ext] = callback(&EngineCtx{
		Views:      app.config.Views,
		ViewEngine: "." + app.config.ViewEngine,
		ViewCache:  app.config.ViewCache,
	})

	return app
}

// Returns an instance of a single route, which you can then use to handle HTTP verbs with optional middleware.
// Use app.Route() to avoid duplicate route names (and thus typo erros).
func (app *App) Route(path string) IRoute {
	return &route{
		app:  app,
		path: path,
	}
}

// The application’s in-built instance of router. This is created lazily, on first access.
func (app *App) Router() IRouter {
	r := NewRouter()
	app.registerRouter("/", r)
	return r
}

// Use registers a middleware route that will match requests
// with the provided prefix (which is optional and defaults to "/").
//
//	app.Use(func(c *fiber.Ctx) error {
//	     return c.Next()
//	})
//	app.Use("/api", func(c *fiber.Ctx) error {
//	     return c.Next()
//	})
//	app.Use("/api", handler, func(c *fiber.Ctx) error {
//	     return c.Next()
//	})
//
// This method will match all HTTP verbs: GET, POST, PUT, HEAD etc...
func (app *App) Use(args ...any) IRouter {
	var prefix string
	var multiPrefix []string
	var subApp *App
	var router *Router
	var handlers []Handler
	var static *Static
	var errorHandler ErrorHandler

	for i := 0; i < len(args); i++ {
		switch arg := args[i].(type) {
		case string:
			prefix = arg
		case []string:
			multiPrefix = arg
		case *App:
			subApp = arg
		case *Router:
			router = arg
		case *Static:
			static = arg
		case Handler:
			handlers = append(handlers, arg)
		case ErrorHandler:
			errorHandler = arg
		default:
			panic(fmt.Sprintf("use: invalid handler %v\n", reflect.TypeOf(arg)))
		}
	}

	if subApp != nil {
		if len(multiPrefix) > 0 {
			for _, p := range multiPrefix {
				app.mount(p, subApp)
			}
		}
		app.mount(prefix, subApp)
	}

	if router != nil {
		if len(multiPrefix) > 0 {
			for _, p := range multiPrefix {
				app.registerRouter(p, router)
			}
		}
		app.registerRouter(prefix, router)
	}

	if static != nil {
		if len(multiPrefix) > 0 {
			for _, p := range multiPrefix {
				app.registerStatic(p, static.Root, static.Config)
			}
		}
		app.registerStatic(prefix, static.Root, static.Config)
	}

	if len(handlers) > 0 {
		if len(multiPrefix) > 0 {
			for _, p := range multiPrefix {
				app.register(methodUse, p, handlers...)
			}
		}
		app.register(methodUse, prefix, handlers...)
	}

	if errorHandler != nil {
		app.errorHandler = errorHandler
	}

	return app
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (app *App) Get(path string, handlers ...Handler) IRouter {
	app.register(MethodHead, path, handlers...)
	app.register(MethodGet, path, handlers...)
	return app
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (app *App) Head(path string, handlers ...Handler) IRouter {
	return app.register(MethodHead, path, handlers...)
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (app *App) Post(path string, handlers ...Handler) IRouter {
	return app.register(MethodPost, path, handlers...)
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (app *App) Put(path string, handlers ...Handler) IRouter {
	return app.register(MethodPut, path, handlers...)
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (app *App) Delete(path string, handlers ...Handler) IRouter {
	return app.register(MethodDelete, path, handlers...)
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (app *App) Connect(path string, handlers ...Handler) IRouter {
	return app.register(MethodConnect, path, handlers...)
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (app *App) Options(path string, handlers ...Handler) IRouter {
	return app.register(MethodOptions, path, handlers...)
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the path to the target resource.
func (app *App) Trace(path string, handlers ...Handler) IRouter {
	return app.register(MethodTrace, path, handlers...)
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (app *App) Patch(path string, handlers ...Handler) IRouter {
	return app.register(MethodPatch, path, handlers...)
}

// All will register the handler on all HTTP methods
func (app *App) All(path string, handlers ...Handler) IRouter {
	for _, method := range intMethod {
		_ = app.register(method, path, handlers...)
	}
	return app
}

// Returns the canonical path of the app, a string.
func (app *App) Path() string {
	return app.path
}

// The MountPath property contains one or more path patterns on which a sub-app was mounted.
func (app *App) MountPath() string {
	return app.mountpath
}

// On is an alias for .AddListener(eventName, listener).
func (app *App) On(eventName string, listener any) *App {
	if err := app.eventEmitter.On(eventName, listener); err != nil {
		panic(err)
	}

	return app
}

// Once adds a one-time listener function for the event named eventName.
// The next time eventName is triggered, this listener is removed and then invoked.
func (app *App) Once(eventName string, listener any) *App {
	if err := app.eventEmitter.Once(eventName, listener); err != nil {
		panic(err)
	}

	return app
}

// Emit synchronously calls each of the listeners registered for the event named eventName, in the order they were registered, passing the supplied arguments to each.
// Returns true if the event had listeners, false otherwise
func (app *App) Emit(eventName string, arguments ...any) *App {
	if err := app.eventEmitter.Emit(eventName, arguments...); err != nil {
		panic(err)
	}

	return app
}

// Off removes the specified listener from the listener array for the event named eventName.
func (app *App) Off(eventName string, listener any) *App {
	if _, err := app.eventEmitter.Off(eventName, listener); err != nil {
		panic(err)
	}

	return app
}

// Alias for .On(eventName, listener).
func (app *App) AddListener(eventName string, listener any) *App {
	if err := app.eventEmitter.AddListener(eventName, listener); err != nil {
		panic(err)
	}
	return app
}

// RemoveAllListeners removes all listeners, or those of the specified eventNames.
func (app *App) RemoveAllListeners(eventNames ...string) *App {
	app.eventEmitter.RemoveAllListeners(eventNames...)
	return app
}

// RemoveListener is the alias for app.Off(eventName, listener).
func (app *App) RemoveListener(eventName string, listener any) *App {
	if _, err := app.eventEmitter.RemoveListener(eventName, listener); err != nil {
		panic(err)
	}

	return app
}

// ListenerCount returns the number of listeners listening to the event named eventName.
func (app *App) ListenerCount(eventName string) int {
	count, err := app.eventEmitter.ListenerCount(eventName)
	if err != nil {
		panic(err)
	}

	return count
}

// The mount event is fired on a sub-app, when it is mounted on a parent app.
//
// The parent app is passed to the callback function.
func (app *App) OnMount(callback func(parent *App)) {
	if app.parent == nil {
		panic("not mounted sub app to parent app")
	}

	if app.mountpath == "" {
		panic("onmount cannot be used on parent app")
	}

	// returns parent app in callback
	callback(app.parent)
}

// Error makes it compatible with the `error` interface.
func (e *Error) Error() string {
	return e.Message
}

// NewError creates a new Error instance with an optional message
func NewError(code int, message ...string) *Error {
	err := &Error{
		Code:    code,
		Message: utils.StatusMessage(code),
	}
	if len(message) > 0 {
		err.Message = message[0]
	}
	return err
}

// Config returns the app config as value ( read-only ).
func (app *App) Config() Config {
	return app.config
}

// Handler returns the server handler.
func (app *App) Handler() fasthttp.RequestHandler {
	// prepare the server for the start
	app.startupProcess()
	return app.handler
}

// Stack returns the raw router stack.
func (app *App) Stack() [][]*Route {
	return app.stack
}

// HandlersCount returns the amount of registered handlers.
func (app *App) HandlersCount() uint32 {
	return app.handlersCount
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
// Shutdown works by first closing all open listeners and then waiting indefinitely for all connections to return to idle and then shut down.
//
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

// Server returns the underlying fasthttp server
func (app *App) Server() *fasthttp.Server {
	return app.server
}

// Test is used for internal debugging by passing a *http.Request.
// Timeout is optional and defaults to 1s, -1 will disable it completely.
func (app *App) Test(req *http.Request, msTimeout ...int) (resp *http.Response, err error) {
	// Set timeout
	timeout := 1000
	if len(msTimeout) > 0 {
		timeout = msTimeout[0]
	}

	// Add Content-Length if not provided with body
	if req.Body != http.NoBody && req.Header.Get(HeaderContentLength) == "" {
		req.Header.Add(HeaderContentLength, strconv.FormatInt(req.ContentLength, 10))
	}

	// Dump raw http request
	dump, err := httputil.DumpRequest(req, true)
	if err != nil {
		return nil, err
	}

	// adding back the query from URL, since dump cleans it
	dumps := bytes.Split(dump, []byte(" "))
	dumps[1] = []byte(req.URL.String())
	dump = bytes.Join(dumps, []byte(" "))

	// Create test connection
	conn := new(testConn)

	// Write raw http request
	if _, err = conn.r.Write(dump); err != nil {
		return nil, err
	}
	// prepare the server for the start
	app.startupProcess()

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
	if err != nil && err != fasthttp.ErrGetOnly {
		return nil, err
	}

	// Read response
	buffer := bufio.NewReader(&conn.w)

	// Convert raw http response to *http.Response
	return http.ReadResponse(buffer, req)
}

// ErrorHandler is the application's method in charge of finding the
// appropriate handler for the given request. It searches any mounted
// sub fibers by their prefixes and if it finds a match, it uses that
// error handler. Otherwise it uses the configured error handler for
// the app, which if not set is the DefaultErrorHandler.
func (app *App) ErrorHandler(ctx *Ctx, err error) error {
	var (
		mountedPrefixParts int
		mountedErrHandler  ErrorHandler
		routerPrefixParts  int
		routerErrHandler   ErrorHandler
	)

	for prefix, subApp := range app.subList {
		if strings.HasPrefix(ctx.path, prefix) {
			parts := len(strings.Split(prefix, "/"))
			if mountedPrefixParts <= parts {
				mountedErrHandler = subApp.errorHandler
				mountedPrefixParts = parts
			}
		}
	}

	for prefix, router := range app.routerList {
		if strings.HasPrefix(ctx.path, prefix) {
			parts := len(strings.Split(prefix, "/"))
			if routerPrefixParts <= parts {
				routerErrHandler = router.errorHandler
				routerPrefixParts = parts
			}
		}
	}

	if mountedErrHandler != nil {
		return mountedErrHandler(ctx, err)
	}

	if routerErrHandler != nil {
		return routerErrHandler(ctx, err)
	}

	return app.errorHandler(ctx, err)
}
