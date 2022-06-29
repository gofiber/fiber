// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"context"
	"io"
	"mime/multipart"

	"github.com/savsgio/dictpool"
	"github.com/valyala/fasthttp"
)

// Ctx represents the Context which hold the HTTP request and response.
// It has methods for the request query string, parameters, body, HTTP headers and so on.
type Ctx interface {
	// Accepts checks if the specified extensions or content types are acceptable.
	Accepts(offers ...string) string

	// AcceptsCharsets checks if the specified charset is acceptable.
	AcceptsCharsets(offers ...string) string

	// AcceptsEncodings checks if the specified encoding is acceptable.
	AcceptsEncodings(offers ...string) string

	// AcceptsLanguages checks if the specified language is acceptable.
	AcceptsLanguages(offers ...string) string

	// App returns the *App reference to the instance of the Fiber application
	App() *App

	// Append the specified value to the HTTP response header field.
	Append(field string, values ...string)

	// Attachment sets the HTTP response Content-Disposition header field to attachment.
	Attachment(filename ...string)

	// BaseURL returns (protocol + host + base path).
	BaseURL() string

	// Body contains the raw body submitted in a POST request.
	Body() []byte

	// BodyParser binds the request body to a struct.
	BodyParser(out any) error

	// ClearCookie expires a specific cookie by key on the client side.
	ClearCookie(key ...string)

	// Context returns *fasthttp.RequestCtx that carries a deadline
	// a cancellation signal, and other values across API boundaries.
	Context() *fasthttp.RequestCtx

	// UserContext returns a context implementation that was set by
	// user earlier or returns a non-nil, empty context,if it was not set earlier.
	UserContext() context.Context

	// SetUserContext sets a context implementation by user.
	SetUserContext(ctx context.Context)

	// Cookie sets a cookie by passing a cookie struct.
	Cookie(cookie *Cookie)

	// Cookies is used for getting a cookie value by key.
	Cookies(key string, defaultValue ...string) string

	// Download transfers the file from path as an attachment.
	Download(file string, filename ...string) error

	// Request return the *fasthttp.Request object
	Request() *fasthttp.Request

	// Response return the *fasthttp.Response object
	Response() *fasthttp.Response

	// Format performs content-negotiation on the Accept HTTP header.
	Format(body any) error

	// FormFile returns the first file by key from a MultipartForm.
	FormFile(key string) (*multipart.FileHeader, error)

	// FormValue returns the first value by key from a MultipartForm.
	FormValue(key string, defaultValue ...string) string

	// Fresh returns true when the response is still “fresh” in the client's cache,
	// otherwise false is returned to indicate that the client cache is now stale
	// and the full response should be sent.
	Fresh() bool

	// Get returns the HTTP request header specified by field.
	Get(key string, defaultValue ...string) string

	// GetRespHeader returns the HTTP response header specified by field.
	GetRespHeader(key string, defaultValue ...string) string

	// GetReqHeaders returns the HTTP request headers.
	GetReqHeaders() map[string]string

	// GetRespHeaders returns the HTTP response headers.
	GetRespHeaders() map[string]string

	// Hostname contains the hostname derived from the X-Forwarded-Host or Host HTTP header.
	Hostname() string

	// Port returns the remote port of the request.
	Port() string

	// IP returns the remote IP address of the request.
	IP() string

	// IPs returns an string slice of IP addresses specified in the X-Forwarded-For request header.
	IPs() (ips []string)

	// Is returns the matching content type.
	Is(extension string) bool

	// JSON converts any interface or string to JSON.
	JSON(data any) error

	// JSONP sends a JSON response with JSONP support.
	JSONP(data any, callback ...string) error

	// Links joins the links followed by the property to populate the response's Link HTTP header field.
	Links(link ...string)

	// Locals makes it possible to pass any values under string keys scoped to the request
	// and therefore available to all following routes that match the request.
	Locals(key string, value ...any) (val any)

	// Location sets the response Location HTTP header to the specified path parameter.
	Location(path string)

	// Method contains a string corresponding to the HTTP method of the request: GET, POST, PUT and so on.
	Method(override ...string) string

	// MultipartForm parse form entries from binary.
	MultipartForm() (*multipart.Form, error)

	// Next executes the next method in the stack that matches the current route.
	Next() (err error)

	// RestartRouting instead of going to the next handler. This may be usefull after changing the request path.
	RestartRouting() error

	// OriginalURL contains the original request URL.
	OriginalURL() string

	// Params is used to get the route parameters.
	Params(key string, defaultValue ...string) string

	// Params is used to get all route parameters.
	AllParams() map[string]string

	// ParamsInt is used to get an integer from the route parameters
	// it defaults to zero if the parameter is not found or if the
	// parameter cannot be converted to an integer.
	ParamsInt(key string, defaultValue ...int) (int, error)

	// Path returns the path part of the request URL.
	Path(override ...string) string

	// Protocol contains the request protocol string: http or https for TLS requests.
	Protocol() string

	// Query returns the query string parameter in the url.
	Query(key string, defaultValue ...string) string

	// QueryParser binds the query string to a struct.
	QueryParser(out any) error

	// ReqHeaderParser binds the request header strings to a struct.
	ReqHeaderParser(out any) error

	// Range returns a struct containing the type and a slice of ranges.
	Range(size int) (rangeData Range, err error)

	// Redirect to the URL derived from the specified path, with specified status.
	Redirect(location string, status ...int) error

	// Add vars to default view var map binding to template engine.
	Bind(vars Map) error

	// GetRouteURL generates URLs to named routes, with parameters. URLs are relative, for example: "/user/1831"
	GetRouteURL(routeName string, params Map) (string, error)

	// RedirectToRoute to the Route registered in the app with appropriate parameters
	// If status is not specified, status defaults to 302 Found.
	RedirectToRoute(routeName string, params Map, status ...int) error

	// RedirectBack to the URL to referer
	// If status is not specified, status defaults to 302 Found.
	RedirectBack(fallback string, status ...int) error

	// Render a template with data and sends a text/html response.
	Render(name string, bind Map, layouts ...string) error

	// Route returns the matched Route struct.
	Route() *Route

	// SaveFile saves any multipart file to disk.
	SaveFile(fileheader *multipart.FileHeader, path string) error

	// SaveFileToStorage saves any multipart file to an external storage system.
	SaveFileToStorage(fileheader *multipart.FileHeader, path string, storage Storage) error

	// Secure returns a boolean property, that is true, if a TLS connection is established.
	Secure() bool

	// Send sets the HTTP response body without copying it.
	Send(body []byte) error

	// SendFile transfers the file from the given path.
	SendFile(file string, compress ...bool) error

	// SendStatus sets the HTTP status code and if the response body is empty,
	SendStatus(status int) error

	// SendString sets the HTTP response body for string types.
	// This means no type assertion, recommended for faster performance
	SendString(body string) error

	// SendStream sets response body stream and optional body size.
	SendStream(stream io.Reader, size ...int) error

	// Set sets the response's HTTP header field to the specified key, value.
	Set(key string, val string)

	// Subdomains returns a string slice of subdomains in the domain name of the request.
	Subdomains(offset ...int) []string

	// Stale is not implemented yet, pull requests are welcome!
	Stale() bool

	// Status sets the HTTP status for the response.
	Status(status int) Ctx

	// String returns unique string representation of the ctx.
	String() string

	// Type sets the Content-Type HTTP header to the MIME type specified by the file extension.
	Type(extension string, charset ...string) Ctx

	// Vary adds the given header field to the Vary response header.
	Vary(fields ...string)

	// Write appends p into response body.
	Write(p []byte) (int, error)

	// Writef appends f & a into response body writer.
	Writef(f string, a ...any) (int, error)

	// WriteString appends s to response body.
	WriteString(s string) (int, error)

	// XHR returns a Boolean property, that is true, if the request's X-Requested-With header field is XMLHttpRequest,
	// indicating that the request was issued by a client library (such as jQuery).
	XHR() bool

	// IsFromLocal will return true if request came from local.
	IsFromLocal() bool

	IsProxyTrusted() bool

	// Reset is a method to reset context fields by given request when to use server handlers.
	Reset(fctx *fasthttp.RequestCtx)

	// SetReq resets fields of context that is relating to request.
	setReq(fctx *fasthttp.RequestCtx)

	// Release is a method to reset context fields when to use ReleaseCtx()
	release()
}

type CustomCtx interface {
	Ctx

	// Reset is a method to reset context fields by given request when to use server handlers.
	Reset(fctx *fasthttp.RequestCtx)

	// Methods to use with next stack.
	getMethodINT() int
	getIndexRoute() int
	getTreePath() string
	getDetectionPath() string
	getPathOriginal() string
	getValues() *[maxParams]string
	getMatched() bool
	setIndexHandler(handler int)
	setIndexRoute(route int)
	setMatched(matched bool)
	setRoute(route *Route)
}

func NewDefaultCtx(app *App) *DefaultCtx {
	// return ctx
	return &DefaultCtx{
		// Set app reference
		app: app,

		// Reset route and handler index
		indexRoute:   -1,
		indexHandler: 0,

		// Reset matched flag
		matched: false,

		// reset base uri
		baseURI: "",
	}
}

func (app *App) NewCtx(fctx *fasthttp.RequestCtx) Ctx {
	var c Ctx

	if app.newCtxFunc != nil {
		c = app.newCtxFunc(app)
	} else {
		c = NewDefaultCtx(app)
	}

	// Set request
	c.setReq(fctx)

	return c
}

// AcquireCtx retrieves a new Ctx from the pool.
func (app *App) AcquireCtx() Ctx {
	return app.pool.Get().(Ctx)
}

// ReleaseCtx releases the ctx back into the pool.
func (app *App) ReleaseCtx(c Ctx) {
	c.release()
	app.pool.Put(c)
}

// Reset is a method to reset context fields by given request when to use server handlers.
func (c *DefaultCtx) Reset(fctx *fasthttp.RequestCtx) {
	// Reset route and handler index
	c.indexRoute = -1
	c.indexHandler = 0

	// Reset matched flag
	c.matched = false

	// Set paths
	c.pathOriginal = c.app.getString(fctx.URI().PathOriginal())

	// Attach *fasthttp.RequestCtx to ctx
	c.fasthttp = fctx

	// reset base uri
	c.baseURI = ""

	// Set method
	c.method = c.app.getString(fctx.Request.Header.Method())
	c.methodINT = methodInt(c.method)

	// Prettify path
	c.configDependentPaths()
}

// Release is a method to reset context fields when to use ReleaseCtx()
func (c *DefaultCtx) release() {
	c.route = nil
	c.fasthttp = nil
	if c.viewBindMap != nil {
		dictpool.ReleaseDict(c.viewBindMap)
	}
}

// SetReq resets fields of context that is relating to request.
func (c *DefaultCtx) setReq(fctx *fasthttp.RequestCtx) {
	// Set paths
	c.pathOriginal = c.app.getString(fctx.URI().PathOriginal())

	// Attach *fasthttp.RequestCtx to ctx
	c.fasthttp = fctx

	// Set method
	c.method = c.app.getString(fctx.Request.Header.Method())
	c.methodINT = methodInt(c.method)

	// Prettify path
	c.configDependentPaths()
}

// Methods to use with next stack.
func (c *DefaultCtx) getMethodINT() int {
	return c.methodINT
}

func (c *DefaultCtx) getIndexRoute() int {
	return c.indexRoute
}

func (c *DefaultCtx) getTreePath() string {
	return c.treePath
}

func (c *DefaultCtx) getDetectionPath() string {
	return c.detectionPath
}

func (c *DefaultCtx) getPathOriginal() string {
	return c.pathOriginal
}

func (c *DefaultCtx) getValues() *[maxParams]string {
	return &c.values
}

func (c *DefaultCtx) getMatched() bool {
	return c.matched
}

func (c *DefaultCtx) setIndexHandler(handler int) {
	c.indexHandler = handler
}

func (c *DefaultCtx) setIndexRoute(route int) {
	c.indexRoute = route
}

func (c *DefaultCtx) setMatched(matched bool) {
	c.matched = matched
}

func (c *DefaultCtx) setRoute(route *Route) {
	c.route = route
}
