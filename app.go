// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

// Package fiber is an Express inspired web framework built on top of Fasthttp,
// the fastest HTTP engine for Go. Designed to ease things up for fast
// development with zero memory allocation and performance in mind.
package fiber

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/tabwriter"
	"time"

	"encoding/json"

	"github.com/gofiber/fiber/v2/internal/colorable"
	"github.com/gofiber/fiber/v2/internal/isatty"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

// Version of current fiber package
const Version = "2.32.0"

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
//  cfg := fiber.Config{}
//  cfg.ErrorHandler = func(c *Ctx, err error) error {
//   code := StatusInternalServerError
//   if e, ok := err.(*Error); ok {
//     code = e.Code
//   }
//   c.Set(HeaderContentType, MIMETextPlainCharsetUTF8)
//   return c.Status(code).SendString(err.Error())
//  }
//  app := fiber.New(cfg)
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
	// Mounted and main apps
	appList map[string]*App
	// Hooks
	hooks *hooks
	// Latest route & group
	latestRoute *Route
	latestGroup *Group
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

	// If set to true, will print all routes with their method, path and handler.
	// Default: false
	EnablePrintRoutes bool `json:"enable_print_routes"`
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

// DefaultErrorHandler that process return errors from handlers
var DefaultErrorHandler = func(c *Ctx, err error) error {
	code := StatusInternalServerError
	if e, ok := err.(*Error); ok {
		code = e.Code
	}
	c.Set(HeaderContentType, MIMETextPlainCharsetUTF8)
	return c.Status(code).SendString(err.Error())
}

// New creates a new Fiber named instance.
//  app := fiber.New()
// You can pass optional configuration options by passing a Config struct:
//  app := fiber.New(fiber.Config{
//      Prefork: true,
//      ServerHeader: "Fiber",
//  })
func New(config ...Config) *App {
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
		// Create config
		config:      Config{},
		getBytes:    utils.UnsafeBytes,
		getString:   utils.UnsafeString,
		appList:     make(map[string]*App),
		latestRoute: &Route{},
		latestGroup: &Group{},
	}

	// Define hooks
	app.hooks = newHooks(app)

	// Override config if provided
	if len(config) > 0 {
		app.config = config[0]
	}

	if app.config.ETag {
		if !IsChild() {
			fmt.Println("[Warning] Config.ETag is deprecated since v2.0.6, please use 'middleware/etag'.")
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
	if app.config.Network == "" {
		app.config.Network = NetworkTCP4
	}

	app.config.trustedProxiesMap = make(map[string]struct{}, len(app.config.TrustedProxies))
	for _, ipAddress := range app.config.TrustedProxies {
		app.handleTrustedProxy(ipAddress)
	}

	// Init appList
	app.appList[""] = app

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

// Mount attaches another app instance as a sub-router along a routing path.
// It's very useful to split up a large API as many independent routers and
// compose them as a single service using Mount. The fiber's error handler and
// any of the fiber's sub apps are added to the application's error handlers
// to be invoked on errors that happen within the prefix route.
func (app *App) Mount(prefix string, fiber *App) Router {
	stack := fiber.Stack()
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		prefix = "/"
	}

	for m := range stack {
		for r := range stack[m] {
			route := app.copyRoute(stack[m][r])
			app.addRoute(route.Method, app.addPrefixToRoute(prefix, route))
		}
	}

	// Support for configs of mounted-apps and sub-mounted-apps
	for mountedPrefixes, subApp := range fiber.appList {
		app.appList[prefix+mountedPrefixes] = subApp
		subApp.init()
	}

	atomic.AddUint32(&app.handlersCount, fiber.handlersCount)

	return app
}

// Assign name to specific route.
func (app *App) Name(name string) Router {
	app.mutex.Lock()
	if strings.HasPrefix(app.latestRoute.path, app.latestGroup.Prefix) {
		app.latestRoute.Name = app.latestGroup.name + name
	} else {
		app.latestRoute.Name = name
	}

	if err := app.hooks.executeOnNameHooks(*app.latestRoute); err != nil {
		panic(err)
	}
	app.mutex.Unlock()

	return app
}

// Get route by name
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

// Use registers a middleware route that will match requests
// with the provided prefix (which is optional and defaults to "/").
//
//  app.Use(func(c *fiber.Ctx) error {
//       return c.Next()
//  })
//  app.Use("/api", func(c *fiber.Ctx) error {
//       return c.Next()
//  })
//  app.Use("/api", handler, func(c *fiber.Ctx) error {
//       return c.Next()
//  })
//
// This method will match all HTTP verbs: GET, POST, PUT, HEAD etc...
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
	return app.register(method, path, handlers...)
}

// Static will create a file server serving static files
func (app *App) Static(prefix, root string, config ...Static) Router {
	return app.registerStatic(prefix, root, config...)
}

// All will register the handler on all HTTP methods
func (app *App) All(path string, handlers ...Handler) Router {
	for _, method := range intMethod {
		_ = app.Add(method, path, handlers...)
	}
	return app
}

// Group is used for Routes with common prefix to define a new sub-router with optional middleware.
//  api := app.Group("/api")
//  api.Get("/users", handler)
func (app *App) Group(prefix string, handlers ...Handler) Router {
	if len(handlers) > 0 {
		app.register(methodUse, prefix, handlers...)
	}
	grp := &Group{Prefix: prefix, app: app}
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

// Listener can be used to pass a custom listener.
func (app *App) Listener(ln net.Listener) error {
	// Prefork is supported for custom listeners
	if app.config.Prefork {
		addr, tlsConfig := lnMetadata(app.config.Network, ln)
		return app.prefork(app.config.Network, addr, tlsConfig)
	}
	// prepare the server for the start
	app.startupProcess()
	// Print startup message
	if !app.config.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), getTlsConfig(ln) != nil, "")
	}
	// Print routes
	if app.config.EnablePrintRoutes {
		app.printRoutesMessage()
	}
	// Start listening
	return app.server.Serve(ln)
}

// Listen serves HTTP requests from the given addr.
//
//  app.Listen(":8080")
//  app.Listen("127.0.0.1:8080")
func (app *App) Listen(addr string) error {
	// Start prefork
	if app.config.Prefork {
		return app.prefork(app.config.Network, addr, nil)
	}
	// Setup listener
	ln, err := net.Listen(app.config.Network, addr)
	if err != nil {
		return err
	}
	// prepare the server for the start
	app.startupProcess()
	// Print startup message
	if !app.config.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), false, "")
	}
	// Print routes
	if app.config.EnablePrintRoutes {
		app.printRoutesMessage()
	}
	// Start listening
	return app.server.Serve(ln)
}

// ListenTLS serves HTTPS requests from the given addr.
// certFile and keyFile are the paths to TLS certificate and key file:
//  app.ListenTLS(":8080", "./cert.pem", "./cert.key")
func (app *App) ListenTLS(addr, certFile, keyFile string) error {
	// Check for valid cert/key path
	if len(certFile) == 0 || len(keyFile) == 0 {
		return errors.New("tls: provide a valid cert or key path")
	}
	// Prefork is supported
	if app.config.Prefork {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("tls: cannot load TLS key pair from certFile=%q and keyFile=%q: %s", certFile, keyFile, err)
		}
		config := &tls.Config{
			MinVersion: tls.VersionTLS12,
			Certificates: []tls.Certificate{
				cert,
			},
		}
		return app.prefork(app.config.Network, addr, config)
	}
	// Setup listener
	ln, err := net.Listen(app.config.Network, addr)
	if err != nil {
		return err
	}
	// prepare the server for the start
	app.startupProcess()
	// Print startup message
	if !app.config.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), true, "")
	}
	// Print routes
	if app.config.EnablePrintRoutes {
		app.printRoutesMessage()
	}
	// Start listening
	return app.server.ServeTLS(ln, certFile, keyFile)
}

// ListenMutualTLS serves HTTPS requests from the given addr.
// certFile, keyFile and clientCertFile are the paths to TLS certificate and key file:
//  app.ListenMutualTLS(":8080", "./cert.pem", "./cert.key", "./client.pem")
func (app *App) ListenMutualTLS(addr, certFile, keyFile, clientCertFile string) error {
	// Check for valid cert/key path
	if len(certFile) == 0 || len(keyFile) == 0 {
		return errors.New("tls: provide a valid cert or key path")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("tls: cannot load TLS key pair from certFile=%q and keyFile=%q: %s", certFile, keyFile, err)
	}

	clientCACert, err := ioutil.ReadFile(filepath.Clean(clientCertFile))
	if err != nil {
		return err
	}
	clientCertPool := x509.NewCertPool()
	clientCertPool.AppendCertsFromPEM(clientCACert)

	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  clientCertPool,
		Certificates: []tls.Certificate{
			cert,
		},
	}

	// Prefork is supported
	if app.config.Prefork {
		return app.prefork(app.config.Network, addr, config)
	}

	// Setup listener
	ln, err := tls.Listen(app.config.Network, addr, config)
	if err != nil {
		return err
	}

	// prepare the server for the start
	app.startupProcess()

	// Print startup message
	if !app.config.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), true, "")
	}

	// Print routes
	if app.config.EnablePrintRoutes {
		app.printRoutesMessage()
	}

	// Start listening
	return app.server.Serve(ln)
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
	if app.hooks != nil {
		defer app.hooks.executeOnShutdownHooks()
	}

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

// Hooks returns the hook struct to register hooks.
func (app *App) Hooks() *hooks {
	return app.hooks
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

type disableLogger struct{}

func (dl *disableLogger) Printf(_ string, _ ...interface{}) {
	// fmt.Println(fmt.Sprintf(format, args...))
}

func (app *App) init() *App {
	// lock application
	app.mutex.Lock()

	// Only load templates if a view engine is specified
	if app.config.Views != nil {
		if err := app.config.Views.Load(); err != nil {
			fmt.Printf("views: %v\n", err)
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

	for prefix, subApp := range app.appList {
		if prefix != "" && strings.HasPrefix(ctx.path, prefix) {
			parts := len(strings.Split(prefix, "/"))
			if mountedPrefixParts <= parts {
				mountedErrHandler = subApp.config.ErrorHandler
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
	if _, ok := err.(*fasthttp.ErrSmallBuffer); ok {
		err = ErrRequestHeaderFieldsTooLarge
	} else if netErr, ok := err.(*net.OpError); ok && netErr.Timeout() {
		err = ErrRequestTimeout
	} else if err == fasthttp.ErrBodyTooLarge {
		err = ErrRequestEntityTooLarge
	} else if err == fasthttp.ErrGetOnly {
		err = ErrMethodNotAllowed
	} else if strings.Contains(err.Error(), "timeout") {
		err = ErrRequestTimeout
	} else {
		err = ErrBadRequest
	}

	if catch := app.ErrorHandler(c, err); catch != nil {
		_ = c.SendStatus(StatusInternalServerError)
	}

	app.ReleaseCtx(c)
}

// startupProcess Is the method which executes all the necessary processes just before the start of the server.
func (app *App) startupProcess() *App {
	if err := app.hooks.executeOnListenHooks(); err != nil {
		panic(err)
	}

	app.mutex.Lock()
	app.buildTree()
	app.mutex.Unlock()
	return app
}

// startupMessage prepares the startup message with the handler number, port, address and other information
func (app *App) startupMessage(addr string, tls bool, pids string) {
	// ignore child processes
	if IsChild() {
		return
	}

	const (
		cBlack = "\u001b[90m"
		// cRed   = "\u001b[91m"
		cCyan = "\u001b[96m"
		// cGreen = "\u001b[92m"
		// cYellow  = "\u001b[93m"
		// cBlue    = "\u001b[94m"
		// cMagenta = "\u001b[95m"
		// cWhite   = "\u001b[97m"
		cReset = "\u001b[0m"
	)

	value := func(s string, width int) string {
		pad := width - len(s)
		str := ""
		for i := 0; i < pad; i++ {
			str += "."
		}
		if s == "Disabled" {
			str += " " + s
		} else {
			str += fmt.Sprintf(" %s%s%s", cCyan, s, cBlack)
		}
		return str
	}

	center := func(s string, width int) string {
		pad := strconv.Itoa((width - len(s)) / 2)
		str := fmt.Sprintf("%"+pad+"s", " ")
		str += s
		str += fmt.Sprintf("%"+pad+"s", " ")
		if len(str) < width {
			str += " "
		}
		return str
	}

	centerValue := func(s string, width int) string {
		pad := strconv.Itoa((width - len(s)) / 2)
		str := fmt.Sprintf("%"+pad+"s", " ")
		str += fmt.Sprintf("%s%s%s", cCyan, s, cBlack)
		str += fmt.Sprintf("%"+pad+"s", " ")
		if len(str)-10 < width {
			str += " "
		}
		return str
	}

	pad := func(s string, width int) (str string) {
		toAdd := width - len(s)
		str += s
		for i := 0; i < toAdd; i++ {
			str += " "
		}
		return
	}

	host, port := parseAddr(addr)
	if host == "" {
		if app.config.Network == NetworkTCP6 {
			host = "[::1]"
		} else {
			host = "0.0.0.0"
		}
	}

	scheme := "http"
	if tls {
		scheme = "https"
	}

	isPrefork := "Disabled"
	if app.config.Prefork {
		isPrefork = "Enabled"
	}

	procs := strconv.Itoa(runtime.GOMAXPROCS(0))
	if !app.config.Prefork {
		procs = "1"
	}

	mainLogo := cBlack + " ┌───────────────────────────────────────────────────┐\n"
	if app.config.AppName != "" {
		mainLogo += " │ " + centerValue(app.config.AppName, 49) + " │\n"
	}
	mainLogo += " │ " + centerValue(" Fiber v"+Version, 49) + " │\n"

	if host == "0.0.0.0" {
		mainLogo +=
			" │ " + center(fmt.Sprintf("%s://127.0.0.1:%s", scheme, port), 49) + " │\n" +
				" │ " + center(fmt.Sprintf("(bound on host 0.0.0.0 and port %s)", port), 49) + " │\n"
	} else {
		mainLogo +=
			" │ " + center(fmt.Sprintf("%s://%s:%s", scheme, host, port), 49) + " │\n"
	}

	mainLogo += fmt.Sprintf(
		" │                                                   │\n"+
			" │ Handlers %s  Processes %s │\n"+
			" │ Prefork .%s  PID ....%s │\n"+
			" └───────────────────────────────────────────────────┘"+
			cReset,
		value(strconv.Itoa(int(app.handlersCount)), 14), value(procs, 12),
		value(isPrefork, 14), value(strconv.Itoa(os.Getpid()), 14),
	)

	var childPidsLogo string
	if app.config.Prefork {
		var childPidsTemplate string
		childPidsTemplate += "%s"
		childPidsTemplate += " ┌───────────────────────────────────────────────────┐\n%s"
		childPidsTemplate += " └───────────────────────────────────────────────────┘"
		childPidsTemplate += "%s"

		newLine := " │ %s%s%s │"

		// Turn the `pids` variable (in the form ",a,b,c,d,e,f,etc") into a slice of PIDs
		var pidSlice []string
		for _, v := range strings.Split(pids, ",") {
			if v != "" {
				pidSlice = append(pidSlice, v)
			}
		}

		var lines []string
		thisLine := "Child PIDs ... "
		var itemsOnThisLine []string

		addLine := func() {
			lines = append(lines,
				fmt.Sprintf(
					newLine,
					cBlack,
					thisLine+cCyan+pad(strings.Join(itemsOnThisLine, ", "), 49-len(thisLine)),
					cBlack,
				),
			)
		}

		for _, pid := range pidSlice {
			if len(thisLine+strings.Join(append(itemsOnThisLine, pid), ", ")) > 49 {
				addLine()
				thisLine = ""
				itemsOnThisLine = []string{pid}
			} else {
				itemsOnThisLine = append(itemsOnThisLine, pid)
			}
		}

		// Add left over items to their own line
		if len(itemsOnThisLine) != 0 {
			addLine()
		}

		// Form logo
		childPidsLogo = fmt.Sprintf(childPidsTemplate,
			cBlack,
			strings.Join(lines, "\n")+"\n",
			cReset,
		)
	}

	// Combine both the child PID logo and the main Fiber logo

	// Pad the shorter logo to the length of the longer one
	splitMainLogo := strings.Split(mainLogo, "\n")
	splitChildPidsLogo := strings.Split(childPidsLogo, "\n")

	mainLen := len(splitMainLogo)
	childLen := len(splitChildPidsLogo)

	if mainLen > childLen {
		diff := mainLen - childLen
		for i := 0; i < diff; i++ {
			splitChildPidsLogo = append(splitChildPidsLogo, "")
		}
	} else {
		diff := childLen - mainLen
		for i := 0; i < diff; i++ {
			splitMainLogo = append(splitMainLogo, "")
		}
	}

	// Combine the two logos, line by line
	output := "\n"
	for i := range splitMainLogo {
		output += cBlack + splitMainLogo[i] + " " + splitChildPidsLogo[i] + "\n"
	}

	out := colorable.NewColorableStdout()
	if os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") == "1" || (!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd())) {
		out = colorable.NewNonColorable(os.Stdout)
	}

	_, _ = fmt.Fprintln(out, output)
}

// printRoutesMessage print all routes with method, path, name and handlers
// in a format of table, like this:
// method | path | name      | handlers
// GET    | /    | routeName | github.com/gofiber/fiber/v2.emptyHandler
// HEAD   | /    |           | github.com/gofiber/fiber/v2.emptyHandler
func (app *App) printRoutesMessage() {
	// ignore child processes
	if IsChild() {
		return
	}

	const (
		// cBlack = "\u001b[90m"
		// cRed   = "\u001b[91m"
		cCyan   = "\u001b[96m"
		cGreen  = "\u001b[92m"
		cYellow = "\u001b[93m"
		cBlue   = "\u001b[94m"
		// cMagenta = "\u001b[95m"
		cWhite = "\u001b[97m"
		// cReset = "\u001b[0m"
	)
	var routes []RouteMessage
	for _, routeStack := range app.stack {
		for _, route := range routeStack {
			var newRoute = RouteMessage{}
			newRoute.name = route.Name
			newRoute.method = route.Method
			newRoute.path = route.Path
			for _, handler := range route.Handlers {
				newRoute.handlers += runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name() + " "
			}
			routes = append(routes, newRoute)
		}
	}

	out := colorable.NewColorableStdout()
	if os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") == "1" || (!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd())) {
		out = colorable.NewNonColorable(os.Stdout)
	}

	w := tabwriter.NewWriter(out, 1, 1, 1, ' ', 0)
	// Sort routes by path
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].path < routes[j].path
	})
	_, _ = fmt.Fprintf(w, "%smethod\t%s| %spath\t%s| %sname\t%s| %shandlers\n", cBlue, cWhite, cGreen, cWhite, cCyan, cWhite, cYellow)
	_, _ = fmt.Fprintf(w, "%s------\t%s| %s----\t%s| %s----\t%s| %s--------\n", cBlue, cWhite, cGreen, cWhite, cCyan, cWhite, cYellow)
	for _, route := range routes {
		_, _ = fmt.Fprintf(w, "%s%s\t%s| %s%s\t%s| %s%s\t%s| %s%s\n", cBlue, route.method, cWhite, cGreen, route.path, cWhite, cCyan, route.name, cWhite, cYellow, route.handlers)
	}

	_ = w.Flush()
}
