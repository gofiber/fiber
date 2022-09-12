// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
)

// Router defines all router handle interface includes app and group router.
type IRouter interface {
	Use(args ...interface{}) IRouter

	All(path string, handlers ...Handler) IRouter
	Get(path string, handlers ...Handler) IRouter
	Head(path string, handlers ...Handler) IRouter
	Post(path string, handlers ...Handler) IRouter
	Put(path string, handlers ...Handler) IRouter
	Delete(path string, handlers ...Handler) IRouter
	Connect(path string, handlers ...Handler) IRouter
	Options(path string, handlers ...Handler) IRouter
	Trace(path string, handlers ...Handler) IRouter
	Patch(path string, handlers ...Handler) IRouter
}

type RouterConfig struct {
	// Enable case sensitivity.
	//
	// Disabled by default, treating “/Foo” and “/foo” as the same.
	CaseSensitive bool `json:"case_sensitive"`
	// Preserve the req.params values from the parent router.
	//
	// If the parent and the child have conflicting param names, the child’s value take precedence.
	//
	// Disabled by default
	MergeParams bool `json:"merge_params"`
	// Enable strict routing.
	//
	// Disabled by default, “/foo” and “/foo/” are treated the same by the router.
	Strict bool `json:"strict"`
}

var DefaultRouterConfig = RouterConfig{
	CaseSensitive: false,
	MergeParams:   false,
	Strict:        false,
}

type Router struct {
	// mutex sync.Mutex
	// App
	app *App
	// Route stack divided by HTTP methods
	stack [][]*Route
	// contains the information if the route stack has been changed to build the optimized tree
	routesRefreshed bool
	// Amount of registered routes
	routesCount uint32
	// Amount of registered handlers
	handlersCount uint32
	// Router config
	config RouterConfig
	// It is neccessary for merge params
	params []string
}

// Creates a new router object.
//
// You can add middleware and HTTP method routes (such as get, put, post, and so on) to router just like an application.
func NewRouter(config ...RouterConfig) *Router {
	r := &Router{
		stack:  make([][]*Route, len(intMethod)),
		config: DefaultRouterConfig,
		params: make([]string, 0),
	}

	if len(config) > 0 {
		if config[0].CaseSensitive {
			r.config.CaseSensitive = true
		}
		//TODO: MergeParams Feature
		if config[0].MergeParams {
			r.config.MergeParams = true
		}
		if config[0].Strict {
			r.config.Strict = true
		}
	}

	return r
}

// Stack returns the raw router stack.
func (r *Router) Stack() [][]*Route {
	return r.stack
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (r *Router) Use(args ...interface{}) IRouter {
	var prefix string
	var multiPrefix []string
	var handlers []Handler
	var static *Static

	for i := 0; i < len(args); i++ {
		switch arg := args[i].(type) {
		case string:
			prefix = arg
		case []string:
			multiPrefix = arg
		case *Static:
			static = arg
		case Handler:
			handlers = append(handlers, arg)
		default:
			panic(fmt.Sprintf("use: invalid handler %v\n", reflect.TypeOf(arg)))
		}
	}

	if static != nil {
		if len(multiPrefix) > 0 {
			for _, p := range multiPrefix {
				r.registerStatic(p, static.Root, static.Config)
			}
		}
		r.registerStatic(prefix, static.Root, static.Config)
	}

	if len(handlers) > 0 {
		if len(multiPrefix) > 0 {
			for _, p := range multiPrefix {
				r.register(methodUse, p, handlers...)
			}
		}
		r.register(methodUse, prefix, handlers...)
	}

	return r
}

// Get registers a route for GET methods that requests a representation
// of the specified resource. Requests using GET should only retrieve data.
func (r *Router) Get(path string, handlers ...Handler) IRouter {
	r.register(MethodHead, path, handlers...)
	r.register(MethodGet, path, handlers...)
	return r
}

// Head registers a route for HEAD methods that asks for a response identical
// to that of a GET request, but without the response body.
func (r *Router) Head(path string, handlers ...Handler) IRouter {
	r.register(MethodHead, path, handlers...)
	return r
}

// Post registers a route for POST methods that is used to submit an entity to the
// specified resource, often causing a change in state or side effects on the server.
func (r *Router) Post(path string, handlers ...Handler) IRouter {
	r.register(MethodPost, path, handlers...)
	return r
}

// Put registers a route for PUT methods that replaces all current representations
// of the target resource with the request payload.
func (r *Router) Put(path string, handlers ...Handler) IRouter {
	r.register(MethodPut, path, handlers...)
	return r
}

// Delete registers a route for DELETE methods that deletes the specified resource.
func (r *Router) Delete(path string, handlers ...Handler) IRouter {
	r.register(MethodDelete, path, handlers...)
	return r
}

// Connect registers a route for CONNECT methods that establishes a tunnel to the
// server identified by the target resource.
func (r *Router) Connect(path string, handlers ...Handler) IRouter {
	r.register(MethodConnect, path, handlers...)
	return r
}

// Options registers a route for OPTIONS methods that is used to describe the
// communication options for the target resource.
func (r *Router) Options(path string, handlers ...Handler) IRouter {
	r.register(MethodOptions, path, handlers...)
	return r
}

// Trace registers a route for TRACE methods that performs a message loop-back
// test along the path to the target resource.
func (r *Router) Trace(path string, handlers ...Handler) IRouter {
	r.register(MethodTrace, path, handlers...)
	return r
}

// Patch registers a route for PATCH methods that is used to apply partial
// modifications to a resource.
func (r *Router) Patch(path string, handlers ...Handler) IRouter {
	r.register(MethodPatch, path, handlers...)
	return r
}

// All will register the handler on all HTTP methods
func (r *Router) All(path string, handlers ...Handler) IRouter {
	for _, method := range intMethod {
		r.register(method, path, handlers...)
	}
	return r
}

// Returns an instance of a single route which you can then use to handle HTTP verbs with optional middleware.
//
// Use router.Route() to avoid duplicate route naming and thus typing errors.
func (r *Router) Route(path string) IRoute {
	return &route{
		app:  r.app,
		path: path,
	}
}

func (r *Router) register(method, pathRaw string, handlers ...Handler) {
	// Uppercase HTTP methods
	method = utils.ToUpper(method)
	// Check if the HTTP method is valid unless it's USE
	if method != methodUse && methodInt(method) == -1 {
		panic(fmt.Sprintf("add: invalid http method %s\n", method))
	}
	// A route requires atleast one ctx handler
	if len(handlers) == 0 {
		panic(fmt.Sprintf("missing handler in route: %s\n", pathRaw))
	}
	// Cannot have an empty path
	if pathRaw == "" {
		pathRaw = "/"
	}
	// Path always start with a '/'
	if pathRaw[0] != '/' {
		pathRaw = "/" + pathRaw
	}
	// Create a stripped path in-case sensitive / trailing slashes
	pathPretty := pathRaw
	// Case sensitive routing, all to lowercase
	if !r.config.CaseSensitive {
		pathPretty = utils.ToLower(pathPretty)
	}
	// Strict routing, remove trailing slashes
	if !r.config.Strict && len(pathPretty) > 1 {
		pathPretty = utils.TrimRight(pathPretty, '/')
	}
	// Is layer a middleware?
	isUse := method == methodUse
	// Is path a direct wildcard?
	isStar := pathPretty == "/*"
	// Is path a root slash?
	isRoot := pathPretty == "/"
	// Parse path parameters
	parsedRaw := parseRoute(pathRaw)
	parsedPretty := parseRoute(pathPretty)

	if len(r.params) > 0 {
		parsedRaw.params = append(r.params, parsedRaw.params...)
		parsedPretty.params = append(r.params, parsedPretty.params...)
	}

	// Create route metadata without pointer
	route := Route{
		// Router booleans
		use:  isUse,
		star: isStar,
		root: isRoot,

		// Path data
		path:        RemoveEscapeChar(pathPretty),
		routeParser: parsedPretty,
		Params:      parsedRaw.params,

		// Public data
		Path:     pathRaw,
		Method:   method,
		Handlers: handlers,
	}

	// Increment global handler count
	atomic.AddUint32(&r.handlersCount, uint32(len(handlers)))

	// Middleware route matches all HTTP methods
	if isUse {
		// Add route to all HTTP methods stack
		for _, m := range intMethod {
			// Create a route copy to avoid duplicates during compression
			rt := route
			r.addRoute(m, &rt)
		}
	} else {
		// Add route to stack
		r.addRoute(method, &route)
	}
}

func (r *Router) addRoute(method string, route *Route) {
	// Get unique HTTP method identifier
	m := methodInt(method)

	// prevent identically route registration
	l := len(r.stack[m])
	if l > 0 && r.stack[m][l-1].Path == route.Path && route.use == r.stack[m][l-1].use {
		preRoute := r.stack[m][l-1]
		preRoute.Handlers = append(preRoute.Handlers, route.Handlers...)
	} else {
		// Increment global route position
		route.pos = atomic.AddUint32(&r.routesCount, 1)
		route.Method = method
		// Add route to the stack
		r.stack[m] = append(r.stack[m], route)
		r.routesRefreshed = true
	}
}

func (r *Router) registerStatic(prefix, root string, config ...StaticConfig) IRouter {
	// For security we want to restrict to the current work directory.
	if root == "" {
		root = "."
	}
	// Cannot have an empty prefix
	if prefix == "" {
		prefix = "/"
	}
	// Prefix always start with a '/' or '*'
	if prefix[0] != '/' {
		prefix = "/" + prefix
	}
	// in case sensitive routing, all to lowercase
	if !r.config.CaseSensitive {
		prefix = utils.ToLower(prefix)
	}
	// Strip trailing slashes from the root path
	if len(root) > 0 && root[len(root)-1] == '/' {
		root = root[:len(root)-1]
	}
	// Is prefix a direct wildcard?
	isStar := prefix == "/*"
	// Is prefix a root slash?
	isRoot := prefix == "/"
	// Is prefix a partial wildcard?
	if strings.Contains(prefix, "*") {
		// /john* -> /john
		isStar = true
		prefix = strings.Split(prefix, "*")[0]
		// Fix this later
	}
	prefixLen := len(prefix)
	if prefixLen > 1 && prefix[prefixLen-1:] == "/" {
		// /john/ -> /john
		prefixLen--
		prefix = prefix[:prefixLen]
	}
	// Fileserver settings
	fs := &fasthttp.FS{
		Root:                 root,
		AllowEmptyRoot:       true,
		GenerateIndexPages:   false,
		AcceptByteRange:      false,
		Compress:             false,
		CompressedFileSuffix: r.app.config.CompressedFileSuffix,
		CacheDuration:        10 * time.Second,
		IndexNames:           []string{"index.html"},
		PathRewrite: func(fctx *fasthttp.RequestCtx) []byte {
			path := fctx.Path()
			if len(path) >= prefixLen {
				if isStar && r.app.getString(path[0:prefixLen]) == prefix {
					path = append(path[0:0], '/')
				} else {
					path = path[prefixLen:]
					if len(path) == 0 || path[len(path)-1] != '/' {
						path = append(path, '/')
					}
				}
			}
			if len(path) > 0 && path[0] != '/' {
				path = append([]byte("/"), path...)
			}
			return path
		},
		PathNotFound: func(fctx *fasthttp.RequestCtx) {
			fctx.Response.SetStatusCode(StatusNotFound)
		},
	}

	// Set config if provided
	var cacheControlValue string
	if len(config) > 0 {
		maxAge := config[0].MaxAge
		if maxAge > 0 {
			cacheControlValue = "public, max-age=" + strconv.Itoa(maxAge)
		}
		fs.CacheDuration = config[0].CacheDuration
		fs.Compress = config[0].Compress
		fs.AcceptByteRange = config[0].ByteRange
		fs.GenerateIndexPages = config[0].Browse
		if config[0].Index != "" {
			fs.IndexNames = []string{config[0].Index}
		}
	}
	fileHandler := fs.NewRequestHandler()
	handler := func(c *Ctx) error {
		// Don't execute middleware if Next returns true
		if len(config) != 0 && config[0].Next != nil && config[0].Next(c) {
			return c.Next()
		}
		// Serve file
		fileHandler(c.fasthttp)
		// Sets the response Content-Disposition header to attachment if the Download option is true
		if len(config) > 0 && config[0].Download {
			c.Attachment()
		}
		// Return request if found and not forbidden
		status := c.fasthttp.Response.StatusCode()
		if status != StatusNotFound && status != StatusForbidden {
			if len(cacheControlValue) > 0 {
				c.fasthttp.Response.Header.Set(HeaderCacheControl, cacheControlValue)
			}
			return nil
		}
		// Reset response to default
		c.fasthttp.SetContentType("") // Issue #420
		c.fasthttp.Response.SetStatusCode(StatusOK)
		c.fasthttp.Response.SetBodyString("")
		// Next middleware
		return c.Next()
	}

	// Create route metadata without pointer
	route := Route{
		// Router booleans
		use:  true,
		root: isRoot,
		path: prefix,
		// Public data
		Method:   MethodGet,
		Path:     prefix,
		Handlers: []Handler{handler},
	}
	// Increment global handler count
	atomic.AddUint32(&r.handlersCount, 1)
	// Add route to stack
	r.addRoute(MethodGet, &route)
	// Add HEAD route
	r.addRoute(MethodHead, &route)
	return r
}
