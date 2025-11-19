// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
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
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3/binder"
	"github.com/gofiber/fiber/v3/log"
)

// Version of current fiber package
const Version = "3.0.0-rc.3"

// Handler defines a function to serve HTTP requests.
type Handler = func(Ctx) error

// Map is a shortcut for map[string]any, useful for JSON returns
type Map map[string]any

// ErrorHandler defines a function that will process all errors
// returned from any handlers in the stack
//
//	cfg := fiber.Config{}
//	cfg.ErrorHandler = func(c Ctx, err error) error {
//	 code := StatusInternalServerError
//	 var e *fiber.Error
//	 if errors.As(err, &e) {
//	   code = e.Code
//	 }
//	 c.Set(HeaderContentType, MIMETextPlainCharsetUTF8)
//	 return c.Status(code).SendString(err.Error())
//	}
//	app := fiber.New(cfg)
type ErrorHandler = func(Ctx, error) error

// Error represents an error that occurred while handling a request.
type Error struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// App denotes the Fiber application.
type App struct {
	// App config
	config Config
	// Indicates if the value was explicitly configured
	configured Config
	// Ctx pool
	pool sync.Pool
	// Fasthttp server
	server *fasthttp.Server
	// Converts string to a byte slice
	toBytes func(s string) (b []byte)
	// Converts byte slice to a string
	toString func(b []byte) string
	// Hooks
	hooks *Hooks
	// Latest route & group
	latestRoute *Route
	// newCtxFunc
	newCtxFunc func(app *App) CustomCtx
	// TLS handler
	tlsHandler *TLSHandler
	// Mount fields
	mountFields *mountFields
	// state management
	state *State
	// Route stack divided by HTTP methods
	stack [][]*Route
	// customConstraints is a list of external constraints
	customConstraints []CustomConstraint
	// sendfiles stores configurations for handling ctx.SendFile operations
	sendfiles []*sendFileStore
	// custom binders
	customBinders []CustomBinder
	// Route stack divided by HTTP methods and route prefixes
	treeStack []map[int][]*Route
	// sendfilesMutex is a mutex used for sendfile operations
	sendfilesMutex sync.RWMutex
	mutex          sync.Mutex
	// Amount of registered handlers
	handlersCount uint32
	// contains the information if the route stack has been changed to build the optimized tree
	routesRefreshed bool
	// hasCustomCtx tracks whether app uses a custom context implementation
	hasCustomCtx bool
}

// Config is a struct holding the server settings.
type Config struct { //nolint:govet // Aligning the struct fields is not necessary. betteralign:ignore
	// Enables the "Server: value" HTTP header.
	//
	// Default: ""
	ServerHeader string `json:"server_header"`

	// When set to true, the router treats "/foo" and "/foo/" as different.
	// By default this is disabled and both "/foo" and "/foo/" will execute the same handler.
	//
	// Default: false
	StrictRouting bool `json:"strict_routing"`

	// When set to true, enables case-sensitive routing.
	// E.g. "/FoO" and "/foo" are treated as different routes.
	// By default this is disabled and both "/FoO" and "/foo" will execute the same handler.
	//
	// Default: false
	CaseSensitive bool `json:"case_sensitive"`

	// When set to true, disables automatic registration of HEAD routes for
	// every GET route.
	//
	// Default: false
	DisableHeadAutoRegister bool `json:"disable_head_auto_register"`

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
	// Zero or negative values fall back to the default limit.
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

	// CompressedFileSuffixes adds suffix to the original file name and
	// tries saving the resulting compressed file under the new file name.
	//
	// Default: map[string]string{"gzip": ".fiber.gz", "br": ".fiber.br", "zstd": ".fiber.zst"}
	CompressedFileSuffixes map[string]string `json:"compressed_file_suffixes"`

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

	// This function allows to setup app name for the app
	//
	// Default: nil
	AppName string `json:"app_name"`

	// StreamRequestBody enables request body streaming,
	// and calls the handler sooner when given body is
	// larger than the current limit.
	//
	// Default: false
	StreamRequestBody bool

	// Will not pre parse Multipart Form data if set to true.
	//
	// This option is useful for servers that desire to treat
	// multipart form data as a binary blob, or choose when to parse the data.
	//
	// Server pre parses multipart form data by default.
	//
	// Default: false
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
	// MsgPackMarshal
	//
	// Allowing for flexibility in using another msgpack library for encoding
	// Default: binder.UnimplementedMsgpackMarshal
	MsgPackEncoder utils.MsgPackMarshal `json:"-"`

	// When set by an external client of Fiber it will use the provided implementation of a
	// MsgPackUnmarshal
	//
	// Allowing for flexibility in using another msgpack library for decoding
	// Default: binder.UnimplementedMsgpackUnmarshal
	MsgPackDecoder utils.MsgPackUnmarshal `json:"-"`

	// When set by an external client of Fiber it will use the provided implementation of a
	// CBORMarshal
	//
	// Allowing for flexibility in using another cbor library for encoding
	// Default: binder.UnimplementedCborMarshal
	CBOREncoder utils.CBORMarshal `json:"-"`

	// When set by an external client of Fiber it will use the provided implementation of a
	// CBORUnmarshal
	//
	// Allowing for flexibility in using another cbor library for decoding
	// Default: binder.UnimplementedCborUnmarshal
	CBORDecoder utils.CBORUnmarshal `json:"-"`

	// XMLEncoder set by an external client of Fiber it will use the provided implementation of a
	// XMLMarshal
	//
	// Allowing for flexibility in using another XML library for encoding
	// Default: xml.Marshal
	XMLEncoder utils.XMLMarshal `json:"-"`

	// XMLDecoder set by an external client of Fiber it will use the provided implementation of a
	// XMLUnmarshal
	//
	// Allowing for flexibility in using another XML library for decoding
	// Default: xml.Unmarshal
	XMLDecoder utils.XMLUnmarshal `json:"-"`

	// If you find yourself behind some sort of proxy, like a load balancer,
	// then certain header information may be sent to you using special X-Forwarded-* headers or the Forwarded header.
	// For example, the Host HTTP header is usually used to return the requested host.
	// But when you’re behind a proxy, the actual host may be stored in an X-Forwarded-Host header.
	//
	// If you are behind a proxy, you should enable TrustProxy to prevent header spoofing.
	// If you enable TrustProxy and do not provide a TrustProxyConfig, Fiber will skip
	// all headers that could be spoofed.
	// If the request IP is in the TrustProxyConfig.Proxies allowlist, then:
	//   1. c.Scheme() get value from X-Forwarded-Proto, X-Forwarded-Protocol, X-Forwarded-Ssl or X-Url-Scheme header
	//   2. c.IP() get value from ProxyHeader header.
	//   3. c.Host() and c.Hostname() get value from X-Forwarded-Host header
	// But if the request IP is NOT in the TrustProxyConfig.Proxies allowlist, then:
	//   1. c.Scheme() WON'T get value from X-Forwarded-Proto, X-Forwarded-Protocol, X-Forwarded-Ssl or X-Url-Scheme header,
	//    will return https when a TLS connection is handled by the app, or http otherwise.
	//   2. c.IP() WON'T get value from ProxyHeader header, will return RemoteIP() from fasthttp context
	//   3. c.Host() and c.Hostname() WON'T get value from X-Forwarded-Host header, fasthttp.Request.URI().Host()
	//    will be used to get the hostname.
	//
	// To automatically trust all loopback, link-local, or private IP addresses,
	// without manually adding them to the TrustProxyConfig.Proxies allowlist,
	// you can set TrustProxyConfig.Loopback, TrustProxyConfig.LinkLocal, or TrustProxyConfig.Private to true.
	//
	// Default: false
	TrustProxy bool `json:"trust_proxy"`

	// Read TrustProxy doc.
	//
	// Default: DefaultTrustProxyConfig
	TrustProxyConfig TrustProxyConfig `json:"trust_proxy_config"`

	// If set to true, c.IP() and c.IPs() will validate IP addresses before returning them.
	// Also, c.IP() will return only the first valid IP rather than just the raw header
	// WARNING: this has a performance cost associated with it.
	//
	// Default: false
	EnableIPValidation bool `json:"enable_ip_validation"`

	// You can define custom color scheme. They'll be used for startup message, route list and some middlewares.
	//
	// Optional. Default: DefaultColors
	ColorScheme Colors `json:"color_scheme"`

	// If you want to validate header/form/query... automatically when to bind, you can define struct validator.
	// Fiber doesn't have default validator, so it'll skip validator step if you don't use any validator.
	//
	// Default: nil
	StructValidator StructValidator

	// RequestMethods provides customizability for HTTP methods. You can add/remove methods as you wish.
	//
	// Optional. Default: DefaultMethods
	RequestMethods []string

	// EnableSplittingOnParsers splits the query/body/header parameters by comma when it's true.
	// For example, you can use it to parse multiple values from a query parameter like this:
	//   /api?foo=bar,baz == foo[]=bar&foo[]=baz
	//
	// Optional. Default: false
	EnableSplittingOnParsers bool `json:"enable_splitting_on_parsers"`

	// Services is a list of services that are used by the app (e.g. databases, caches, etc.)
	//
	// Optional. Default: a zero value slice
	Services []Service

	// ServicesStartupContextProvider is a context provider for the startup of the services.
	//
	// Optional. Default: a provider that returns context.Background()
	ServicesStartupContextProvider func() context.Context

	// ServicesShutdownContextProvider is a context provider for the shutdown of the services.
	//
	// Optional. Default: a provider that returns context.Background()
	ServicesShutdownContextProvider func() context.Context
}

// Default TrustProxyConfig
var DefaultTrustProxyConfig = TrustProxyConfig{}

// TrustProxyConfig is a struct for configuring trusted proxies if Config.TrustProxy is true.
type TrustProxyConfig struct {
	ips map[string]struct{}

	// Proxies is a list of trusted proxy IP addresses or CIDR ranges.
	//
	// Default: []string
	Proxies []string `json:"proxies"`

	ranges []*net.IPNet

	// LinkLocal enables trusting all link-local IP ranges (e.g., 169.254.0.0/16, fe80::/10).
	//
	// Default: false
	LinkLocal bool `json:"link_local"`

	// Loopback enables trusting all loopback IP ranges (e.g., 127.0.0.0/8, ::1/128).
	//
	// Default: false
	Loopback bool `json:"loopback"`

	// Private enables trusting all private IP ranges (e.g., 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7).
	//
	// Default: false
	Private bool `json:"private"`
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
	DefaultBodyLimit       = 4 * 1024 * 1024
	DefaultConcurrency     = 256 * 1024
	DefaultReadBufferSize  = 4096
	DefaultWriteBufferSize = 4096
)

const (
	methodGet = iota
	methodHead
	methodPost
	methodPut
	methodDelete
	methodConnect
	methodOptions
	methodTrace
	methodPatch
)

// HTTP methods enabled by default
var DefaultMethods = []string{
	methodGet:     MethodGet,
	methodHead:    MethodHead,
	methodPost:    MethodPost,
	methodPut:     MethodPut,
	methodDelete:  MethodDelete,
	methodConnect: MethodConnect,
	methodOptions: MethodOptions,
	methodTrace:   MethodTrace,
	methodPatch:   MethodPatch,
}

// httpReadResponse - Used for test mocking http.ReadResponse
var httpReadResponse = http.ReadResponse

// DefaultErrorHandler that process return errors from handlers
func DefaultErrorHandler(c Ctx, err error) error {
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
		// Create config
		config:        Config{},
		toBytes:       utils.UnsafeBytes,
		toString:      utils.UnsafeString,
		latestRoute:   &Route{},
		customBinders: []CustomBinder{},
		sendfiles:     []*sendFileStore{},
	}

	// Create Ctx pool
	app.pool = sync.Pool{
		New: func() any {
			if app.newCtxFunc != nil {
				return app.newCtxFunc(app)
			}
			return NewDefaultCtx(app)
		},
	}

	// Define hooks
	app.hooks = newHooks(app)

	// Define mountFields
	app.mountFields = newMountFields(app)

	// Define state
	app.state = newState()

	// Override config if provided
	if len(config) > 0 {
		app.config = config[0]
	}

	// Initialize configured before defaults are set
	app.configured = app.config

	// Override default values
	if app.config.BodyLimit <= 0 {
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
	if app.config.CompressedFileSuffixes == nil {
		app.config.CompressedFileSuffixes = map[string]string{
			"gzip": ".fiber.gz",
			"br":   ".fiber.br",
			"zstd": ".fiber.zst",
		}
	}

	if app.config.Immutable {
		app.toBytes, app.toString = toBytesImmutable, toStringImmutable
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
	if app.config.MsgPackEncoder == nil {
		app.config.MsgPackEncoder = binder.UnimplementedMsgpackMarshal
	}
	if app.config.MsgPackDecoder == nil {
		app.config.MsgPackDecoder = binder.UnimplementedMsgpackUnmarshal
	}
	if app.config.CBOREncoder == nil {
		app.config.CBOREncoder = binder.UnimplementedCborMarshal
	}
	if app.config.CBORDecoder == nil {
		app.config.CBORDecoder = binder.UnimplementedCborUnmarshal
	}
	if app.config.XMLEncoder == nil {
		app.config.XMLEncoder = xml.Marshal
	}
	if app.config.XMLDecoder == nil {
		app.config.XMLDecoder = xml.Unmarshal
	}
	if len(app.config.RequestMethods) == 0 {
		app.config.RequestMethods = DefaultMethods
	}

	app.config.TrustProxyConfig.ips = make(map[string]struct{}, len(app.config.TrustProxyConfig.Proxies))
	for _, ipAddress := range app.config.TrustProxyConfig.Proxies {
		app.handleTrustedProxy(ipAddress)
	}

	// Create router stack
	app.stack = make([][]*Route, len(app.config.RequestMethods))
	app.treeStack = make([]map[int][]*Route, len(app.config.RequestMethods))

	// Override colors
	app.config.ColorScheme = defaultColors(&app.config.ColorScheme)

	// Init app
	app.init()

	// Return app
	return app
}

// NewWithCustomCtx creates a new Fiber instance and applies the
// provided function to generate a custom context type. It mirrors the behavior
// of calling `New()` followed by `app.setCtxFunc(fn)`.
func NewWithCustomCtx(newCtxFunc func(app *App) CustomCtx, config ...Config) *App {
	app := New(config...)
	app.setCtxFunc(newCtxFunc)
	return app
}

// GetString returns s unchanged when Immutable is off or s is read-only (rodata).
// Otherwise, it returns a detached copy (strings.Clone).
func (app *App) GetString(s string) string {
	if !app.config.Immutable || s == "" {
		return s
	}
	if isReadOnly(unsafe.Pointer(unsafe.StringData(s))) { //nolint:gosec // pointer check avoids unnecessary copy
		return s // literal / rodata → safe to return as-is
	}
	return strings.Clone(s) // heap-backed / aliased → detach
}

// GetBytes returns b unchanged when Immutable is off or b is read-only (rodata).
// Otherwise, it returns a detached copy.
func (app *App) GetBytes(b []byte) []byte {
	if !app.config.Immutable || len(b) == 0 {
		return b
	}
	if isReadOnly(unsafe.Pointer(unsafe.SliceData(b))) { //nolint:gosec // pointer check avoids unnecessary copy
		return b // rodata → safe to return as-is
	}
	return utils.CopyBytes(b) // detach when backed by request/response memory
}

// Adds an ip address to TrustProxyConfig.ranges or TrustProxyConfig.ips based on whether it is an IP range or not
func (app *App) handleTrustedProxy(ipAddress string) {
	if strings.Contains(ipAddress, "/") {
		_, ipNet, err := net.ParseCIDR(ipAddress)
		if err != nil {
			log.Warnf("IP range %q could not be parsed: %v", ipAddress, err)
		} else {
			app.config.TrustProxyConfig.ranges = append(app.config.TrustProxyConfig.ranges, ipNet)
		}
	} else {
		ip := net.ParseIP(ipAddress)
		if ip == nil {
			log.Warnf("IP address %q could not be parsed", ipAddress)
		} else {
			app.config.TrustProxyConfig.ips[ipAddress] = struct{}{}
		}
	}
}

// setCtxFunc applies the given context factory to the app.
// It is used internally by NewWithCustomCtx. It doesn't allow adding new methods,
// only customizing existing ones.
func (app *App) setCtxFunc(function func(app *App) CustomCtx) {
	app.newCtxFunc = function
	app.hasCustomCtx = function != nil

	if app.server != nil {
		app.server.Handler = app.requestHandler
	}
}

// RegisterCustomConstraint allows to register custom constraint.
func (app *App) RegisterCustomConstraint(constraint CustomConstraint) {
	app.customConstraints = append(app.customConstraints, constraint)
}

// RegisterCustomBinder Allows to register custom binders to use as Bind().Custom("name").
// They should be compatible with CustomBinder interface.
func (app *App) RegisterCustomBinder(customBinder CustomBinder) {
	app.customBinders = append(app.customBinders, customBinder)
}

// ReloadViews reloads the configured view engine by invoking its Load method.
// It returns an error if no view engine is configured or if reloading fails.
func (app *App) ReloadViews() error {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	if app.config.Views == nil {
		return ErrNoViewEngineConfigured
	}

	if viewValue := reflect.ValueOf(app.config.Views); viewValue.Kind() == reflect.Pointer && viewValue.IsNil() {
		return ErrNoViewEngineConfigured
	}

	if err := app.config.Views.Load(); err != nil {
		return fmt.Errorf("fiber: failed to reload views: %w", err)
	}

	return nil
}

// SetTLSHandler Can be used to set ClientHelloInfo when using TLS with Listener.
func (app *App) SetTLSHandler(tlsHandler *TLSHandler) {
	// Attach the tlsHandler to the config
	app.mutex.Lock()
	app.tlsHandler = tlsHandler
	app.mutex.Unlock()
}

// Name Assign name to specific route.
func (app *App) Name(name string) Router {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	for _, routes := range app.stack {
		for _, route := range routes {
			isMethodValid := route.Method == app.latestRoute.Method || app.latestRoute.use ||
				(app.latestRoute.Method == MethodGet && route.Method == MethodHead)

			if route.Path == app.latestRoute.Path && isMethodValid {
				route.Name = name
				if route.group != nil {
					route.Name = route.group.name + route.Name
				}
			}
		}
	}

	if err := app.hooks.executeOnNameHooks(app.latestRoute); err != nil {
		panic(err)
	}

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
// Also, you can pass another app instance as a sub-router along a routing path.
// It's very useful to split up a large API as many independent routers and
// compose them as a single service using Use. The fiber's error handler and
// any of the fiber's sub apps are added to the application's error handlers
// to be invoked on errors that happen within the prefix route.
//
//		app.Use(func(c fiber.Ctx) error {
//		     return c.Next()
//		})
//		app.Use("/api", func(c fiber.Ctx) error {
//		     return c.Next()
//		})
//		app.Use("/api", handler, func(c fiber.Ctx) error {
//		     return c.Next()
//		})
//	 	subApp := fiber.New()
//		app.Use("/mounted-path", subApp)
//
// This method will match all HTTP verbs: GET, POST, PUT, HEAD etc...
func (app *App) Use(args ...any) Router {
	var prefix string
	var subApp *App
	var prefixes []string
	var handlers []Handler

	for i := range args {
		switch arg := args[i].(type) {
		case string:
			prefix = arg
		case *App:
			subApp = arg
		case []string:
			prefixes = arg
		default:
			handler, ok := toFiberHandler(arg)
			if !ok {
				panic(fmt.Sprintf("use: invalid handler %v\n", reflect.TypeOf(arg)))
			}
			handlers = append(handlers, handler)
		}
	}

	if len(prefixes) == 0 {
		prefixes = append(prefixes, prefix)
	}

	for _, prefix := range prefixes {
		if subApp != nil {
			app.mount(prefix, subApp)
			return app
		}

		app.register([]string{methodUse}, prefix, nil, handlers...)
	}

	return app
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (app *App) Get(path string, handler any, handlers ...any) Router {
	return app.Add([]string{MethodGet}, path, handler, handlers...)
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (app *App) Head(path string, handler any, handlers ...any) Router {
	return app.Add([]string{MethodHead}, path, handler, handlers...)
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (app *App) Post(path string, handler any, handlers ...any) Router {
	return app.Add([]string{MethodPost}, path, handler, handlers...)
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (app *App) Put(path string, handler any, handlers ...any) Router {
	return app.Add([]string{MethodPut}, path, handler, handlers...)
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (app *App) Delete(path string, handler any, handlers ...any) Router {
	return app.Add([]string{MethodDelete}, path, handler, handlers...)
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (app *App) Connect(path string, handler any, handlers ...any) Router {
	return app.Add([]string{MethodConnect}, path, handler, handlers...)
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (app *App) Options(path string, handler any, handlers ...any) Router {
	return app.Add([]string{MethodOptions}, path, handler, handlers...)
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the path to the target resource.
func (app *App) Trace(path string, handler any, handlers ...any) Router {
	return app.Add([]string{MethodTrace}, path, handler, handlers...)
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (app *App) Patch(path string, handler any, handlers ...any) Router {
	return app.Add([]string{MethodPatch}, path, handler, handlers...)
}

// Add allows you to specify multiple HTTP methods to register a route.
func (app *App) Add(methods []string, path string, handler any, handlers ...any) Router {
	converted := collectHandlers("add", append([]any{handler}, handlers...)...)
	app.register(methods, path, nil, converted...)

	return app
}

// All will register the handler on all HTTP methods
func (app *App) All(path string, handler any, handlers ...any) Router {
	return app.Add(app.config.RequestMethods, path, handler, handlers...)
}

// Group is used for Routes with common prefix to define a new sub-router with optional middleware.
//
//	api := app.Group("/api")
//	api.Get("/users", handler)
func (app *App) Group(prefix string, handlers ...any) Router {
	grp := &Group{Prefix: prefix, app: app}
	if len(handlers) > 0 {
		converted := collectHandlers("group", handlers...)
		app.register([]string{methodUse}, prefix, grp, converted...)
	}
	if err := app.hooks.executeOnGroupHooks(*grp); err != nil {
		panic(err)
	}

	return grp
}

// RouteChain creates a Registering instance that lets you declare a stack of
// handlers for the same route. Handlers defined via the returned Register are
// scoped to the provided path.
func (app *App) RouteChain(path string) Register {
	// Create new route
	route := &Registering{app: app, path: path}

	return route
}

// Route is used to define routes with a common prefix inside the supplied
// function. It mirrors the legacy helper and reuses the Group method to create
// a sub-router.
func (app *App) Route(prefix string, fn func(router Router), name ...string) Router {
	if fn == nil {
		panic("route handler 'fn' cannot be nil")
	}
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

// NewErrorf creates a new Error instance with an optional message.
// Additional arguments are formatted using fmt.Sprintf when provided.
// If the first argument in the message slice is not a string, the function
// falls back to using fmt.Sprint on the first element to generate the message.
func NewErrorf(code int, message ...any) *Error {
	var msg string

	switch len(message) {
	case 0:
		// nothing to override
		msg = utils.StatusMessage(code)

	case 1:
		// One argument → treat it like fmt.Sprint(arg)
		if s, ok := message[0].(string); ok {
			msg = s
		} else {
			msg = fmt.Sprint(message[0])
		}

	default:
		// Two or more → first must be a format string.
		if format, ok := message[0].(string); ok {
			msg = fmt.Sprintf(format, message[1:]...)
		} else {
			// If the first arg isn’t a string, fall back.
			msg = fmt.Sprint(message[0])
		}
	}

	return &Error{Code: code, Message: msg}
}

// Config returns the app config as value ( read-only ).
func (app *App) Config() Config {
	return app.config
}

// Handler returns the server handler.
func (app *App) Handler() fasthttp.RequestHandler { //revive:disable-line:confusing-naming // Having both a Handler() (uppercase) and a handler() (lowercase) is fine. TODO: Use nolint:revive directive instead. See https://github.com/golangci/golangci-lint/issues/3476
	// prepare the server for the start
	app.startupProcess()
	return app.requestHandler
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
// Important: app.Listen() must be called in a separate goroutine; otherwise, shutdown hooks will not work
// as Listen() is a blocking operation. Example:
//
//	go app.Listen(":3000")
//	// ...
//	app.Shutdown()
//
// Shutdown does not close keepalive connections so its recommended to set ReadTimeout to something else than 0.
func (app *App) Shutdown() error {
	return app.ShutdownWithContext(context.Background())
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
	return app.ShutdownWithContext(ctx)
}

// ShutdownWithContext shuts down the server including by force if the context's deadline is exceeded.
//
// Make sure the program doesn't exit and waits instead for ShutdownWithTimeout to return.
//
// ShutdownWithContext does not close keepalive connections so its recommended to set ReadTimeout to something else than 0.
func (app *App) ShutdownWithContext(ctx context.Context) error {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	var err error

	if app.server == nil {
		return ErrNotRunning
	}

	// Execute the Shutdown hook
	app.hooks.executeOnPreShutdownHooks()
	defer app.hooks.executeOnPostShutdownHooks(err)

	err = app.server.ShutdownWithContext(ctx)
	return err
}

// Server returns the underlying fasthttp server
func (app *App) Server() *fasthttp.Server {
	return app.server
}

// Hooks returns the hook struct to register hooks.
func (app *App) Hooks() *Hooks {
	return app.hooks
}

// State returns the state struct to store global data in order to share it between handlers.
func (app *App) State() *State {
	return app.state
}

var ErrTestGotEmptyResponse = errors.New("test: got empty response")

// TestConfig is a struct holding Test settings
type TestConfig struct {
	// Timeout defines the maximum duration a
	// test can run before timing out.
	// Default: time.Second
	Timeout time.Duration

	// FailOnTimeout specifies whether the test
	// should return a timeout error if the HTTP response
	// exceeds the Timeout duration.
	// Default: true
	FailOnTimeout bool
}

// Test is used for internal debugging by passing a *http.Request.
// Config is optional and defaults to a 1s error on timeout,
// 0 timeout will disable it completely.
func (app *App) Test(req *http.Request, config ...TestConfig) (*http.Response, error) {
	// Default config
	cfg := TestConfig{
		Timeout:       time.Second,
		FailOnTimeout: true,
	}

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]
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
	if _, err = conn.r.Write(dump); err != nil {
		return nil, fmt.Errorf("failed to write: %w", err)
	}
	// prepare the server for the start
	app.startupProcess()

	// Serve conn to server
	channel := make(chan error, 1)
	go func() {
		var returned bool
		defer func() {
			if !returned {
				channel <- ErrHandlerExited
			}
		}()

		channel <- app.server.ServeConn(conn)
		returned = true
	}()

	// Wait for callback
	if cfg.Timeout > 0 {
		// With timeout
		select {
		case err = <-channel:
		case <-time.After(cfg.Timeout):
			conn.Close() //nolint:errcheck // It is fine to ignore the error here
			if cfg.FailOnTimeout {
				return nil, os.ErrDeadlineExceeded
			}
		}
	} else {
		// Without timeout
		err = <-channel
	}

	// Check for errors
	if err != nil && !errors.Is(err, fasthttp.ErrGetOnly) && !errors.Is(err, errTestConnClosed) {
		return nil, err
	}

	// Read response(s)
	buffer := bufio.NewReader(&conn.w)

	var res *http.Response
	for {
		// Convert raw http response to *http.Response
		res, err = httpReadResponse(buffer, req)
		if err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) {
				return nil, ErrTestGotEmptyResponse
			}
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		// Break if this response is non-1xx or there are no more responses
		if res.StatusCode >= http.StatusOK || buffer.Buffered() == 0 {
			break
		}

		// Discard interim response body before reading the next one
		if res.Body != nil {
			if _, errCopy := io.Copy(io.Discard, res.Body); errCopy != nil {
				return nil, fmt.Errorf("failed to discard interim response body: %w", errCopy)
			}
			if errClose := res.Body.Close(); errClose != nil {
				return nil, fmt.Errorf("failed to close interim response body: %w", errClose)
			}
		}
	}

	return res, nil
}

type disableLogger struct{}

// Printf implements the fasthttp Logger interface and discards log output.
func (*disableLogger) Printf(string, ...any) {
}

func (app *App) init() *App {
	// lock application
	app.mutex.Lock()

	// Initialize Services when needed,
	// panics if there is an error starting them.
	app.initServices()

	// Only load templates if a view engine is specified
	if app.config.Views != nil {
		if err := app.config.Views.Load(); err != nil {
			log.Warnf("failed to load views: %v", err)
		}
	}

	// create fasthttp server
	app.server = &fasthttp.Server{
		Logger:       &disableLogger{},
		LogAllErrors: false,
		ErrorHandler: app.serverErrorHandler,
	}

	// fasthttp server settings
	app.server.Handler = app.requestHandler
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

	// Register the Services shutdown handler once the app is initialized and unlocked.
	app.Hooks().OnPostShutdown(func(_ error) error {
		if err := app.shutdownServices(app.servicesShutdownCtx()); err != nil {
			log.Errorf("failed to shutdown services: %v", err)
		}
		return nil
	})

	return app
}

// ErrorHandler is the application's method in charge of finding the
// appropriate handler for the given request. It searches any mounted
// sub fibers by their prefixes and if it finds a match, it uses that
// error handler. Otherwise, it uses the configured error handler for
// the app, which if not set is the DefaultErrorHandler.
func (app *App) ErrorHandler(ctx Ctx, err error) error {
	var (
		mountedErrHandler  ErrorHandler
		mountedPrefixParts int
	)

	for prefix, subApp := range app.mountFields.appList {
		if prefix != "" && strings.HasPrefix(ctx.Path(), prefix) {
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
	// Acquire Ctx with fasthttp request from pool
	c := app.AcquireCtx(fctx)
	defer app.ReleaseCtx(c)

	var (
		errNetOP *net.OpError
		netErr   net.Error
	)

	switch {
	case errors.As(err, new(*fasthttp.ErrSmallBuffer)):
		err = ErrRequestHeaderFieldsTooLarge
	case errors.As(err, &errNetOP) && errNetOP.Timeout():
		err = ErrRequestTimeout
	case errors.As(err, &netErr):
		err = ErrBadGateway
	case errors.Is(err, fasthttp.ErrBodyTooLarge):
		err = ErrRequestEntityTooLarge
	case errors.Is(err, fasthttp.ErrGetOnly):
		err = ErrMethodNotAllowed
	case strings.Contains(err.Error(), "unsupported http request method"):
		err = ErrNotImplemented
	case strings.Contains(err.Error(), "timeout"):
		err = ErrRequestTimeout
	default:
		err = NewError(StatusBadRequest, err.Error())
	}

	if c.getMethodInt() != -1 {
		c.setSkipNonUseRoutes(true)
		defer c.setSkipNonUseRoutes(false)

		var nextErr error
		if d, isDefault := c.(*DefaultCtx); isDefault {
			_, nextErr = app.next(d)
		} else {
			_, nextErr = app.nextCustom(c)
		}

		if nextErr != nil && !errors.Is(nextErr, ErrNotFound) && !errors.Is(nextErr, ErrMethodNotAllowed) {
			log.Errorf("serverErrorHandler: middleware traversal failed: %v", nextErr)
		}
	}

	if catch := app.ErrorHandler(c, err); catch != nil {
		log.Errorf("serverErrorHandler: failed to call ErrorHandler: %v", catch)
		_ = c.SendStatus(StatusInternalServerError) //nolint:errcheck // It is fine to ignore the error here
		return
	}
}

// startupProcess Is the method which executes all the necessary processes just before the start of the server.
func (app *App) startupProcess() {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	app.ensureAutoHeadRoutesLocked()
	for prefix, subApp := range app.mountFields.appList {
		if prefix == "" {
			continue
		}
		subApp.ensureAutoHeadRoutes()
	}
	app.mountStartupProcess()

	// build route tree stack
	app.buildTree()
}

// Run onListen hooks. If they return an error, panic.
func (app *App) runOnListenHooks(listenData *ListenData) {
	if err := app.hooks.executeOnListenHooks(listenData); err != nil {
		panic(err)
	}
}
