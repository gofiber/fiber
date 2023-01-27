// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

// Package fiber is an Express inspired web framework built on top of Fasthttp,
// the fastest HTTP engine for Go. Designed to ease things up for fast
// development with zero memory allocation and performance in mind.
package fiber

import (
	"bufio"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2/utils"

	"github.com/valyala/fasthttp"
)

// Version of current fiber package
const Version = "2.41.0"

// Handler defines a function to serve HTTP requests.
type Handler = func(*Ctx) error

// Map is a shortcut for map[string]interface{}, useful for JSON returns
type Map map[string]interface{}

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
	// Hooks
	hooks *Hooks
	// Latest route & group
	latestRoute *Route
	// TLS handler
	tlsHandler *TLSHandler
	// Mount fields
	mountFields *mountFields
	// Indicates if the value was explicitly configured
	configured Config
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
	StrictRouting bool `json:"strict_routing"`

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

	// Enable or disable ETag header generation, since both weak and strong etags are generated
	// using the same hashing method (CRC-32). Weak ETags are the default when enabled.
	//
	// Default: false
	ETag bool `json:"etag"`

	// Max body size that the server accepts.
	// -1 will decline any body size
	//
	// Default: 4 * 1024 * 1024
	BodyLimit int `json:"body_limit"`

	// Maximum number of concurrent connections.
	//
	// Default: 256 * 1024
	Concurrency int `json:"concurrency"`

	// Views is the interface that wraps the Render function.
	//
	// Default: nil
	Views Views `json:"-"`

	// Views Layout is the global layout for all template render until override on Render function.
	//
	// Default: ""
	ViewsLayout string `json:"views_layout"`

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

	// ErrorHandler is executed when an error is returned from fiber.Handler.
	//
	// Default: DefaultErrorHandler
	ErrorHandler ErrorHandler `json:"-"`

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
	// The router executes the same handler by default if StrictRouting or CaseSensitive is disabled.
	// Enabling RedirectFixedPath will change this behavior into a client redirect to the original route path.
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

	// XMLEncoder set by an external client of Fiber it will use the provided implementation of a
	// XMLMarshal
	//
	// Allowing for flexibility in using another XML library for encoding
	// Default: xml.Marshal
	XMLEncoder utils.XMLMarshal `json:"-"`

	// Known networks are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only)
	// WARNING: When prefork is set to true, only "tcp4" and "tcp6" can be chose.
	//
	// Default: NetworkTCP4
	Network string

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

	// If set to true, c.IP() and c.IPs() will validate IP addresses before returning them.
	// Also, c.IP() will return only the first valid IP rather than just the raw header
	// WARNING: this has a performance cost associated with it.
	//
	// Default: false
	EnableIPValidation bool `json:"enable_ip_validation"`

	// If set to true, will print all routes with their method, path and handler.
	// Default: false
	EnablePrintRoutes bool `json:"enable_print_routes"`

	// You can define custom color scheme. They'll be used for startup message, route list and some middlewares.
	//
	// Optional. Default: DefaultColors
	ColorScheme Colors `json:"color_scheme"`

	// RequestMethods provides customizibility for HTTP methods. You can add/remove methods as you wish.
	//
	// Optional. Default: DefaultMethods
	RequestMethods []string
}

// Static defines configuration options when defining static assets.
type Static struct {
	// When set to true, the server tries minimizing CPU usage by caching compressed files.
	// This works differently than the github.com/gofiber/compression middleware.
	// Optional. Default value false
	Compress bool `json:"compress"`

	// When set to true, enables byte range requests.
	// Optional. Default value false
	ByteRange bool `json:"byte_range"`

	// When set to true, enables directory browsing.
	// Optional. Default value false.
	Browse bool `json:"browse"`

	// When set to true, enables direct download.
	// Optional. Default value false.
	Download bool `json:"download"`

	// The name of the index file for serving a directory.
	// Optional. Default value "index.html".
	Index string `json:"index"`

	// Expiration duration for inactive file handlers.
	// Use a negative time.Duration to disable it.
	//
	// Optional. Default value 10 * time.Second.
	CacheDuration time.Duration `json:"cache_duration"`

	// The value for the Cache-Control HTTP-header
	// that is set on the file response. MaxAge is defined in seconds.
	//
	// Optional. Default value 0.
	MaxAge int `json:"max_age"`

	// ModifyResponse defines a function that allows you to alter the response.
	//
	// Optional. Default: nil
	ModifyResponse Handler

	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *Ctx) bool
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

// HTTP methods enabled by default
//
//nolint:gochecknoglobals // Using a global var is fine here
var DefaultMethods = []string{
	MethodGet,
	MethodHead,
	MethodPost,
	MethodPut,
	MethodDelete,
	MethodConnect,
	MethodOptions,
	MethodTrace,
	MethodPatch,
}

// DefaultErrorHandler that process return errors from handlers
func DefaultErrorHandler(c *Ctx, err error) error {
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
		// Create Ctx pool
		pool: sync.Pool{
			New: func() interface{} {
				return new(Ctx)
			},
		},
		// Create config
		config:      Config{},
		getBytes:    utils.UnsafeBytes,
		getString:   utils.UnsafeString,
		latestRoute: &Route{},
	}

	// Define hooks
	app.hooks = newHooks(app)

	// Define mountFields
	app.mountFields = newMountFields(app)

	// Override config if provided
	if len(config) > 0 {
		app.config = config[0]
	}

	// Initialize configured before defaults are set
	app.configured = app.config

	if app.config.ETag {
		if !IsChild() {
			log.Printf("[Warning] Config.ETag is deprecated since v2.0.6, please use 'middleware/etag'.\n")
		}
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

	if app.config.ErrorHandler == nil {
		app.config.ErrorHandler = DefaultErrorHandler
	}

	if app.config.JSONEncoder == nil {
		app.config.JSONEncoder = json.Marshal
	}
	if app.config.JSONDecoder == nil {
		app.config.JSONDecoder = json.Unmarshal
	}
	if app.config.XMLEncoder == nil {
		app.config.XMLEncoder = xml.Marshal
	}
	if app.config.Network == "" {
		app.config.Network = NetworkTCP4
	}
	if len(app.config.RequestMethods) == 0 {
		app.config.RequestMethods = DefaultMethods
	}

	app.config.trustedProxiesMap = make(map[string]struct{}, len(app.config.TrustedProxies))
	for _, ipAddress := range app.config.TrustedProxies {
		app.handleTrustedProxy(ipAddress)
	}

	// Create router stack
	app.stack = make([][]*Route, len(app.config.RequestMethods))
	app.treeStack = make([]map[string][]*Route, len(app.config.RequestMethods))

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
			log.Printf("[Warning] IP range %q could not be parsed: %v\n", ipAddress, err)
		} else {
			app.config.trustedProxyRanges = append(app.config.trustedProxyRanges, ipNet)
		}
	} else {
		app.config.trustedProxiesMap[ipAddress] = struct{}{}
	}
}

// SetTLSHandler You can use SetTLSHandler to use ClientHelloInfo when using TLS with Listener.
func (app *App) SetTLSHandler(tlsHandler *TLSHandler) {
	// Attach the tlsHandler to the config
	app.mutex.Lock()
	app.tlsHandler = tlsHandler
	app.mutex.Unlock()
}

// Name Assign name to specific route.
func (app *App) Name(name string) Router {
	app.mutex.Lock()

	latestGroup := app.latestRoute.group
	if latestGroup != nil {
		app.latestRoute.Name = latestGroup.name + name
	} else {
		app.latestRoute.Name = name
	}

	if err := app.hooks.executeOnNameHooks(*app.latestRoute); err != nil {
		panic(err)
	}
	app.mutex.Unlock()

	return app
}

// GetRoute Get route by name
func (app *App) GetRoute(name string) Route {
	for _, routes := range app.stack {
		for _, route := range routes {
			if route.Name == name {
				return *route
			}
		}
	}

	return Route{}
}

// GetRoutes Get all routes. When filterUseOption equal to true, it will filter the routes registered by the middleware.
func (app *App) GetRoutes(filterUseOption ...bool) []Route {
	var rs []Route
	var filterUse bool
	if len(filterUseOption) != 0 {
		filterUse = filterUseOption[0]
	}
	for _, routes := range app.stack {
		for _, route := range routes {
			if filterUse && route.use {
				continue
			}
			rs = append(rs, *route)
		}
	}
	return rs
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
func (app *App) Use(args ...interface{}) Router {
	var prefix string
	var prefixes []string
	var handlers []Handler

	for i := 0; i < len(args); i++ {
		switch arg := args[i].(type) {
		case string:
			prefix = arg
		case []string:
			prefixes = arg
		case Handler:
			handlers = append(handlers, arg)
		default:
			panic(fmt.Sprintf("use: invalid handler %v\n", reflect.TypeOf(arg)))
		}
	}

	if len(prefixes) == 0 {
		prefixes = append(prefixes, prefix)
	}

	for _, prefix := range prefixes {
		app.register(methodUse, prefix, nil, handlers...)
	}

	return app
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (app *App) Get(path string, handlers ...Handler) Router {
	return app.Head(path, handlers...).Add(MethodGet, path, handlers...)
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

// Add allows you to specify a HTTP method to register a route
func (app *App) Add(method, path string, handlers ...Handler) Router {
	return app.register(method, path, nil, handlers...)
}

// Static will create a file server serving static files
func (app *App) Static(prefix, root string, config ...Static) Router {
	return app.registerStatic(prefix, root, config...)
}

// All will register the handler on all HTTP methods
func (app *App) All(path string, handlers ...Handler) Router {
	for _, method := range app.config.RequestMethods {
		_ = app.Add(method, path, handlers...)
	}
	return app
}

// Group is used for Routes with common prefix to define a new sub-router with optional middleware.
//
//	api := app.Group("/api")
//	api.Get("/users", handler)
func (app *App) Group(prefix string, handlers ...Handler) Router {
	grp := &Group{Prefix: prefix, app: app}
	if len(handlers) > 0 {
		app.register(methodUse, prefix, grp, handlers...)
	}
	if err := app.hooks.executeOnGroupHooks(*grp); err != nil {
		panic(err)
	}

	return grp
}

// Route is used to define routes with a common prefix inside the common function.
// Uses Group method to define new sub-router.
func (app *App) Route(prefix string, fn func(router Router), name ...string) Router {
	// Create new group
	group := app.Group(prefix)
	if len(name) > 0 {
		group.Name(name[0])
	}

	// Define routes
	fn(group)

	return group
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
func (app *App) Handler() fasthttp.RequestHandler { //revive:disable-line:confusing-naming // Having both a Handler() (uppercase) and a handler() (lowercase) is fine. TODO: Use nolint:revive directive instead. See https://github.com/golangci/golangci-lint/issues/3476
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
// Shutdown works by first closing all open listeners and then waiting indefinitely for all connections to return to idle before shutting down.
//
// Make sure the program doesn't exit and waits instead for Shutdown to return.
//
// Shutdown does not close keepalive connections so its recommended to set ReadTimeout to something else than 0.
func (app *App) Shutdown() error {
	return app.shutdownWithContext(context.Background())
}

// ShutdownWithTimeout gracefully shuts down the server without interrupting any active connections. However, if the timeout is exceeded,
// ShutdownWithTimeout will forcefully close any active connections.
// ShutdownWithTimeout works by first closing all open listeners and then waiting for all connections to return to idle before shutting down.
//
// Make sure the program doesn't exit and waits instead for ShutdownWithTimeout to return.
//
// ShutdownWithTimeout does not close keepalive connections so its recommended to set ReadTimeout to something else than 0.
func (app *App) ShutdownWithTimeout(timeout time.Duration) error {
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	defer cancelFunc()
	return app.shutdownWithContext(ctx)
}

// shutdownWithContext shuts down the server including by force if the context's deadline is exceeded.
func (app *App) shutdownWithContext(ctx context.Context) error {
	if app.hooks != nil {
		defer app.hooks.executeOnShutdownHooks()
	}

	app.mutex.Lock()
	defer app.mutex.Unlock()
	if app.server == nil {
		return fmt.Errorf("shutdown: server is not running")
	}
	return app.server.ShutdownWithContext(ctx)
}

// Server returns the underlying fasthttp server
func (app *App) Server() *fasthttp.Server {
	return app.server
}

// Hooks returns the hook struct to register hooks.
func (app *App) Hooks() *Hooks {
	return app.hooks
}

// Test is used for internal debugging by passing a *http.Request.
// Timeout is optional and defaults to 1s, -1 will disable it completely.
func (app *App) Test(req *http.Request, msTimeout ...int) (*http.Response, error) {
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
		return nil, fmt.Errorf("failed to dump request: %w", err)
	}

	// Create test connection
	conn := new(testConn)

	// Write raw http request
	if _, err := conn.r.Write(dump); err != nil {
		return nil, fmt.Errorf("failed to write: %w", err)
	}
	// prepare the server for the start
	app.startupProcess()

	// Serve conn to server
	channel := make(chan error)
	go func() {
		var returned bool
		defer func() {
			if !returned {
				channel <- fmt.Errorf("runtime.Goexit() called in handler or server panic")
			}
		}()

		channel <- app.server.ServeConn(conn)
		returned = true
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
	if err != nil && !errors.Is(err, fasthttp.ErrGetOnly) {
		return nil, err
	}

	// Read response
	buffer := bufio.NewReader(&conn.w)

	// Convert raw http response to *http.Response
	res, err := http.ReadResponse(buffer, req)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return res, nil
}

type disableLogger struct{}

func (*disableLogger) Printf(_ string, _ ...interface{}) {
	// fmt.Println(fmt.Sprintf(format, args...))
}

func (app *App) init() *App {
	// lock application
	app.mutex.Lock()

	// Only load templates if a view engine is specified
	if app.config.Views != nil {
		if err := app.config.Views.Load(); err != nil {
			log.Printf("[Warning]: failed to load views: %v\n", err)
		}
	}

	// create fasthttp server
	app.server = &fasthttp.Server{
		Logger:       &disableLogger{},
		LogAllErrors: false,
		ErrorHandler: app.serverErrorHandler,
	}

	// fasthttp server settings
	app.server.Handler = app.handler
	app.server.Name = app.config.ServerHeader
	app.server.Concurrency = app.config.Concurrency
	app.server.NoDefaultDate = app.config.DisableDefaultDate
	app.server.NoDefaultContentType = app.config.DisableDefaultContentType
	app.server.DisableHeaderNamesNormalizing = app.config.DisableHeaderNormalizing
	app.server.DisableKeepalive = app.config.DisableKeepalive
	app.server.MaxRequestBodySize = app.config.BodyLimit
	app.server.NoDefaultServerHeader = app.config.ServerHeader == ""
	app.server.ReadTimeout = app.config.ReadTimeout
	app.server.WriteTimeout = app.config.WriteTimeout
	app.server.IdleTimeout = app.config.IdleTimeout
	app.server.ReadBufferSize = app.config.ReadBufferSize
	app.server.WriteBufferSize = app.config.WriteBufferSize
	app.server.GetOnly = app.config.GETOnly
	app.server.ReduceMemoryUsage = app.config.ReduceMemoryUsage
	app.server.StreamRequestBody = app.config.StreamRequestBody
	app.server.DisablePreParseMultipartForm = app.config.DisablePreParseMultipartForm

	// unlock application
	app.mutex.Unlock()
	return app
}

// ErrorHandler is the application's method in charge of finding the
// appropriate handler for the given request. It searches any mounted
// sub fibers by their prefixes and if it finds a match, it uses that
// error handler. Otherwise it uses the configured error handler for
// the app, which if not set is the DefaultErrorHandler.
func (app *App) ErrorHandler(ctx *Ctx, err error) error {
	var (
		mountedErrHandler  ErrorHandler
		mountedPrefixParts int
	)

	for prefix, subApp := range app.mountFields.appList {
		if prefix != "" && strings.HasPrefix(ctx.path, prefix) {
			parts := len(strings.Split(prefix, "/"))
			if mountedPrefixParts <= parts {
				if subApp.configured.ErrorHandler != nil {
					mountedErrHandler = subApp.config.ErrorHandler
				}

				mountedPrefixParts = parts
			}
		}
	}

	if mountedErrHandler != nil {
		return mountedErrHandler(ctx, err)
	}

	return app.config.ErrorHandler(ctx, err)
}

// serverErrorHandler is a wrapper around the application's error handler method
// user for the fasthttp server configuration. It maps a set of fasthttp errors to fiber
// errors before calling the application's error handler method.
func (app *App) serverErrorHandler(fctx *fasthttp.RequestCtx, err error) {
	c := app.AcquireCtx(fctx)
	defer app.ReleaseCtx(c)

	var errNetOP *net.OpError

	switch {
	case errors.As(err, new(*fasthttp.ErrSmallBuffer)):
		err = ErrRequestHeaderFieldsTooLarge
	case errors.As(err, &errNetOP) && errNetOP.Timeout():
		err = ErrRequestTimeout
	case errors.Is(err, fasthttp.ErrBodyTooLarge):
		err = ErrRequestEntityTooLarge
	case errors.Is(err, fasthttp.ErrGetOnly):
		err = ErrMethodNotAllowed
	case strings.Contains(err.Error(), "timeout"):
		err = ErrRequestTimeout
	default:
		err = NewError(StatusBadRequest, err.Error())
	}

	if catch := app.ErrorHandler(c, err); catch != nil {
		log.Printf("serverErrorHandler: failed to call ErrorHandler: %v\n", catch)
		_ = c.SendStatus(StatusInternalServerError) //nolint:errcheck // It is fine to ignore the error here
		return
	}
}

// startupProcess Is the method which executes all the necessary processes just before the start of the server.
func (app *App) startupProcess() *App {
	if err := app.hooks.executeOnListenHooks(); err != nil {
		panic(err)
	}

	app.mutex.Lock()
	defer app.mutex.Unlock()

	// add routes of sub-apps
	app.mountFields.subAppsRoutesAdded.Do(func() {
		app.appendSubAppLists(app.mountFields.appList)
		app.addSubAppsRoutes(app.mountFields.appList)
		app.generateAppListKeys()
	})

	// build route tree stack
	app.buildTree()

	return app
}
