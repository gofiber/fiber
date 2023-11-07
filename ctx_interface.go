// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime/multipart"
	"sync"

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
	// If the header is not already set, it creates the header with the specified value.
	Append(field string, values ...string)

	// Attachment sets the HTTP response Content-Disposition header field to attachment.
	Attachment(filename ...string)

	// BaseURL returns (protocol + host + base path).
	BaseURL() string

	// Body contains the raw body submitted in a POST request.
	// Returned value is only valid within the handler. Do not store any references.
	// Make copies or use the Immutable setting instead.
	Body() []byte

	// ClearCookie expires a specific cookie by key on the client side.
	// If no key is provided it expires all cookies that came with the request.
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
	// Defaults to the empty string "" if the cookie doesn't exist.
	// If a default value is given, it will return that value if the cookie doesn't exist.
	// The returned value is only valid within the handler. Do not store any references.
	// Make copies or use the Immutable setting to use the value outside the Handler.
	Cookies(key string, defaultValue ...string) string

	// Download transfers the file from path as an attachment.
	// Typically, browsers will prompt the user for download.
	// By default, the Content-Disposition header filename= parameter is the filepath (this typically appears in the browser dialog).
	// Override this default with the filename parameter.
	Download(file string, filename ...string) error

	// Request return the *fasthttp.Request object
	// This allows you to use all fasthttp request methods
	// https://godoc.org/github.com/valyala/fasthttp#Request
	Request() *fasthttp.Request

	// Response return the *fasthttp.Response object
	// This allows you to use all fasthttp response methods
	// https://godoc.org/github.com/valyala/fasthttp#Response
	Response() *fasthttp.Response

	// Format performs content-negotiation on the Accept HTTP header.
	// It uses Accepts to select a proper format.
	// If the header is not specified or there is no proper format, text/plain is used.
	Format(body any) error

	// FormFile returns the first file by key from a MultipartForm.
	FormFile(key string) (*multipart.FileHeader, error)

	// FormValue returns the first value by key from a MultipartForm.
	// Defaults to the empty string "" if the form value doesn't exist.
	// If a default value is given, it will return that value if the form value does not exist.
	// Returned value is only valid within the handler. Do not store any references.
	// Make copies or use the Immutable setting instead.
	FormValue(key string, defaultValue ...string) string

	// Fresh returns true when the response is still “fresh” in the client's cache,
	// otherwise false is returned to indicate that the client cache is now stale
	// and the full response should be sent.
	// When a client sends the Cache-Control: no-cache request header to indicate an end-to-end
	// reload request, this module will return false to make handling these requests transparent.
	// https://github.com/jshttp/fresh/blob/10e0471669dbbfbfd8de65bc6efac2ddd0bfa057/index.js#L33
	Fresh() bool

	// Get returns the HTTP request header specified by field.
	// Field names are case-insensitive
	// Returned value is only valid within the handler. Do not store any references.
	// Make copies or use the Immutable setting instead.
	Get(key string, defaultValue ...string) string

	// GetRespHeader returns the HTTP response header specified by field.
	// Field names are case-insensitive
	// Returned value is only valid within the handler. Do not store any references.
	// Make copies or use the Immutable setting instead.
	GetRespHeader(key string, defaultValue ...string) string

	// Host contains the host derived from the X-Forwarded-Host or Host HTTP header.
	// Returned value is only valid within the handler. Do not store any references.
	// Make copies or use the Immutable setting instead.
	// Please use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
	Host() string

	// Hostname contains the hostname derived from the X-Forwarded-Host or Host HTTP header using the c.Host() method.
	// Returned value is only valid within the handler. Do not store any references.
	// Make copies or use the Immutable setting instead.
	// Please use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
	Hostname() string

	// Port returns the remote port of the request.
	Port() string

	// IP returns the remote IP address of the request.
	// Please use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
	IP() string

	// IPs returns an string slice of IP addresses specified in the X-Forwarded-For request header.
	IPs() (ips []string)

	// Is returns the matching content type,
	// if the incoming request's Content-Type HTTP header field matches the MIME type specified by the type parameter
	Is(extension string) bool

	// JSON converts any interface or string to JSON.
	// Array and slice values encode as JSON arrays,
	// except that []byte encodes as a base64-encoded string,
	// and a nil slice encodes as the null JSON value.
	// This method also sets the content header to application/json.
	JSON(data any) error

	// JSONP sends a JSON response with JSONP support.
	// This method is identical to JSON, except that it opts-in to JSONP callback support.
	// By default, the callback name is simply callback.
	JSONP(data any, callback ...string) error

	// XML converts any interface or string to XML.
	// This method also sets the content header to application/xml.
	XML(data any) error

	// Links joins the links followed by the property to populate the response's Link HTTP header field.
	Links(link ...string)

	// Locals makes it possible to pass any values under string keys scoped to the request
	// and therefore available to all following routes that match the request.
	Locals(key any, value ...any) (val any)

	// Location sets the response Location HTTP header to the specified path parameter.
	Location(path string)

	// Method contains a string corresponding to the HTTP method of the request: GET, POST, PUT and so on.
	Method(override ...string) string

	// MultipartForm parse form entries from binary.
	// This returns a map[string][]string, so given a key the value will be a string slice.
	MultipartForm() (*multipart.Form, error)

	// Next executes the next method in the stack that matches the current route.
	Next() (err error)

	// RestartRouting instead of going to the next handler. This may be useful after
	// changing the request path. Note that handlers might be executed again.
	RestartRouting() error

	// OriginalURL contains the original request URL.
	// Returned value is only valid within the handler. Do not store any references.
	// Make copies or use the Immutable setting to use the value outside the Handler.
	OriginalURL() string

	// Params is used to get the route parameters.
	// Defaults to empty string "" if the param doesn't exist.
	// If a default value is given, it will return that value if the param doesn't exist.
	// Returned value is only valid within the handler. Do not store any references.
	// Make copies or use the Immutable setting to use the value outside the Handler.
	Params(key string, defaultValue ...string) string

	// ParamsInt is used to get an integer from the route parameters
	// it defaults to zero if the parameter is not found or if the
	// parameter cannot be converted to an integer
	// If a default value is given, it will return that value in case the param
	// doesn't exist or cannot be converted to an integer
	ParamsInt(key string, defaultValue ...int) (int, error)

	// Path returns the path part of the request URL.
	// Optionally, you could override the path.
	Path(override ...string) string

	// Scheme contains the request scheme string: http or https for TLS requests.
	// Use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
	Scheme() string

	// Protocol returns the HTTP protocol of request: HTTP/1.1 and HTTP/2.
	Protocol() string

	// Query returns the query string parameter in the url.
	// Defaults to empty string "" if the query doesn't exist.
	// If a default value is given, it will return that value if the query doesn't exist.
	// Returned value is only valid within the handler. Do not store any references.
	// Make copies or use the Immutable setting to use the value outside the Handler.
	Query(key string, defaultValue ...string) string

	// Queries returns a map of query parameters and their values.
	//
	// GET /?name=alex&wanna_cake=2&id=
	// Queries()["name"] == "alex"
	// Queries()["wanna_cake"] == "2"
	// Queries()["id"] == ""
	//
	// GET /?field1=value1&field1=value2&field2=value3
	// Queries()["field1"] == "value2"
	// Queries()["field2"] == "value3"
	//
	// GET /?list_a=1&list_a=2&list_a=3&list_b[]=1&list_b[]=2&list_b[]=3&list_c=1,2,3
	// Queries()["list_a"] == "3"
	// Queries()["list_b[]"] == "3"
	// Queries()["list_c"] == "1,2,3"
	//
	// GET /api/search?filters.author.name=John&filters.category.name=Technology&filters[customer][name]=Alice&filters[status]=pending
	// Queries()["filters.author.name"] == "John"
	// Queries()["filters.category.name"] == "Technology"
	// Queries()["filters[customer][name]"] == "Alice"
	// Queries()["filters[status]"] == "pending"
	Queries() map[string]string

	// QueryInt returns integer value of key string parameter in the url.
	// Default to empty or invalid key is 0.
	//
	//	GET /?name=alex&wanna_cake=2&id=
	//	QueryInt("wanna_cake", 1) == 2
	//	QueryInt("name", 1) == 1
	//	QueryInt("id", 1) == 1
	//	QueryInt("id") == 0
	QueryInt(key string, defaultValue ...int) int

	// QueryBool returns bool value of key string parameter in the url.
	// Default to empty or invalid key is true.
	//
	//	Get /?name=alex&want_pizza=false&id=
	//	QueryBool("want_pizza") == false
	//	QueryBool("want_pizza", true) == false
	//	QueryBool("name") == false
	//	QueryBool("name", true) == true
	//	QueryBool("id") == false
	//	QueryBool("id", true) == true
	QueryBool(key string, defaultValue ...bool) bool

	// QueryFloat returns float64 value of key string parameter in the url.
	// Default to empty or invalid key is 0.
	//
	//	GET /?name=alex&amount=32.23&id=
	//	QueryFloat("amount") = 32.23
	//	QueryFloat("amount", 3) = 32.23
	//	QueryFloat("name", 1) = 1
	//	QueryFloat("name") = 0
	//	QueryFloat("id", 3) = 3
	QueryFloat(key string, defaultValue ...float64) float64

	// Range returns a struct containing the type and a slice of ranges.
	Range(size int) (rangeData Range, err error)

	// Redirect returns the Redirect reference.
	// Use Redirect().Status() to set custom redirection status code.
	// If status is not specified, status defaults to 302 Found.
	// You can use Redirect().To(), Redirect().Route() and Redirect().Back() for redirection.
	Redirect() *Redirect

	// Add vars to default view var map binding to template engine.
	// Variables are read by the Render method and may be overwritten.
	BindVars(vars Map) error

	// GetRouteURL generates URLs to named routes, with parameters. URLs are relative, for example: "/user/1831"
	GetRouteURL(routeName string, params Map) (string, error)

	// Render a template with data and sends a text/html response.
	// We support the following engines: https://github.com/gofiber/template
	Render(name string, bind Map, layouts ...string) error

	// Route returns the matched Route struct.
	Route() *Route

	// SaveFile saves any multipart file to disk.
	SaveFile(fileheader *multipart.FileHeader, path string) error

	// SaveFileToStorage saves any multipart file to an external storage system.
	SaveFileToStorage(fileheader *multipart.FileHeader, path string, storage Storage) error

	// Secure returns whether a secure connection was established.
	Secure() bool

	// Send sets the HTTP response body without copying it.
	// From this point onward the body argument must not be changed.
	Send(body []byte) error

	// SendFile transfers the file from the given path.
	// The file is not compressed by default, enable this by passing a 'true' argument
	// Sets the Content-Type response HTTP header field based on the filenames extension.
	SendFile(file string, compress ...bool) error

	// SendStatus sets the HTTP status code and if the response body is empty,
	// it sets the correct status message in the body.
	SendStatus(status int) error

	// SendString sets the HTTP response body for string types.
	// This means no type assertion, recommended for faster performance
	SendString(body string) error

	// SendStream sets response body stream and optional body size.
	SendStream(stream io.Reader, size ...int) error

	// Set sets the response's HTTP header field to the specified key, value.
	Set(key, val string)

	// Subdomains returns a string slice of subdomains in the domain name of the request.
	// The subdomain offset, which defaults to 2, is used for determining the beginning of the subdomain segments.
	Subdomains(offset ...int) []string

	// Stale is not implemented yet, pull requests are welcome!
	Stale() bool

	// Status sets the HTTP status for the response.
	// This method is chainable.
	Status(status int) Ctx

	// String returns unique string representation of the ctx.
	//
	// The returned value may be useful for logging.
	String() string

	// Type sets the Content-Type HTTP header to the MIME type specified by the file extension.
	Type(extension string, charset ...string) Ctx

	// Vary adds the given header field to the Vary response header.
	// This will append the header, if not already listed, otherwise leaves it listed in the current location.
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

	// IsProxyTrusted checks trustworthiness of remote ip.
	// If EnableTrustedProxyCheck false, it returns true
	// IsProxyTrusted can check remote ip by proxy ranges and ip map.
	IsProxyTrusted() bool

	// IsFromLocal will return true if request came from local.
	IsFromLocal() bool

	// Reset is a method to reset context fields by given request when to use server handlers.
	Reset(fctx *fasthttp.RequestCtx)

	// You can bind body, cookie, headers etc. into the map, map slice, struct easily by using Binding method.
	// It gives custom binding support, detailed binding options and more.
	// Replacement of: BodyParser, ParamsParser, GetReqHeaders, GetRespHeaders, AllParams, QueryParser, ReqHeaderParser
	Bind() *Bind

	// ClientHelloInfo return CHI from context
	ClientHelloInfo() *tls.ClientHelloInfo

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
	ctx, ok := app.pool.Get().(Ctx)
	if !ok {
		panic(fmt.Errorf("failed to type-assert to Ctx"))
	}
	return ctx
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
	c.methodINT = c.app.methodInt(c.method)

	// Prettify path
	c.configDependentPaths()
}

// Release is a method to reset context fields when to use ReleaseCtx()
func (c *DefaultCtx) release() {
	c.route = nil
	c.fasthttp = nil
	c.bind = nil
	c.redirectionMessages = c.redirectionMessages[:0]
	c.viewBindMap = sync.Map{}
	if c.redirect != nil {
		ReleaseRedirect(c.redirect)
		c.redirect = nil
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
	c.methodINT = c.app.methodInt(c.method)

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
