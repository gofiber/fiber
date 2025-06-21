// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

const (
	// maxParams defines the maximum number of parameters per route.
	maxParams         = 30
	maxDetectionPaths = 3
)

var (
	_ io.Writer       = (*DefaultCtx)(nil) // Compile-time check
	_ context.Context = (*DefaultCtx)(nil) // Compile-time check
)

// The contextKey type is unexported to prevent collisions with context keys defined in
// other packages.
type contextKey int //nolint:unused // need for future (nolintlint)

// DefaultCtx is the default implementation of the Ctx interface
// generation tool `go install github.com/vburenin/ifacemaker@c0371201a75b1c79e69cda79a6f90b8ae5799b37`
// https://github.com/vburenin/ifacemaker/blob/975a95966976eeb2d4365a7fb236e274c54da64c/ifacemaker.go#L14-L30
//
//go:generate ifacemaker --file ctx.go --file req.go --file res.go --struct DefaultCtx --iface Ctx --pkg fiber --promoted --output ctx_interface_gen.go --not-exported true --iface-comment "Ctx represents the Context which hold the HTTP request and response.\nIt has methods for the request query string, parameters, body, HTTP headers and so on."
type DefaultCtx struct {
	app           *App                 // Reference to *App
	bind          *Bind                // Default bind reference
	fasthttp      *fasthttp.RequestCtx // Reference to *fasthttp.RequestCtx
	path          []byte               // HTTP path with the modifications by the configuration
	pathOriginal  string               // Original HTTP path
	detectionPath []byte               // Route detection path
	treePathHash  int                  // Hash of the path for the search in the tree
	indexRoute    int                  // Index of the current route
	indexHandler  int                  // Index of the current handler
	matched       bool                 // Non use route matched

	DefaultReq                   // Default request api
	baseURI    string            // HTTP base uri
	methodInt  int               // HTTP method INT equivalent
	route      *Route            // Reference to *Route
	values     [maxParams]string // Route parameter values

	DefaultRes                    // Default response api
	flashMessages redirectionMsgs // Flash messages
	redirect      *Redirect       // Default redirect reference
	viewBindMap   sync.Map        // Default view map to bind template engine
}

// SendFile defines configuration options when to transfer file with SendFile.
type SendFile struct {
	// FS is the file system to serve the static files from.
	// You can use interfaces compatible with fs.FS like embed.FS, os.DirFS etc.
	//
	// Optional. Default: nil
	FS fs.FS

	// When set to true, the server tries minimizing CPU usage by caching compressed files.
	// This works differently than the github.com/gofiber/compression middleware.
	// You have to set Content-Encoding header to compress the file.
	// Available compression methods are gzip, br, and zstd.
	//
	// Optional. Default: false
	Compress bool `json:"compress"`

	// When set to true, enables byte range requests.
	//
	// Optional. Default: false
	ByteRange bool `json:"byte_range"`

	// When set to true, enables direct download.
	//
	// Optional. Default: false
	Download bool `json:"download"`

	// Expiration duration for inactive file handlers.
	// Use a negative time.Duration to disable it.
	//
	// Optional. Default: 10 * time.Second
	CacheDuration time.Duration `json:"cache_duration"`

	// The value for the Cache-Control HTTP-header
	// that is set on the file response. MaxAge is defined in seconds.
	//
	// Optional. Default: 0
	MaxAge int `json:"max_age"`
}

// sendFileStore is used to keep the SendFile configuration and the handler.
type sendFileStore struct {
	handler           fasthttp.RequestHandler
	cacheControlValue string
	config            SendFile
}

// compareConfig compares the current SendFile config with the new one
// and returns true if they are different.
//
// Here we don't use reflect.DeepEqual because it is quite slow compared to manual comparison.
func (sf *sendFileStore) compareConfig(cfg SendFile) bool {
	if sf.config.FS != cfg.FS {
		return false
	}

	if sf.config.Compress != cfg.Compress {
		return false
	}

	if sf.config.ByteRange != cfg.ByteRange {
		return false
	}

	if sf.config.Download != cfg.Download {
		return false
	}

	if sf.config.CacheDuration != cfg.CacheDuration {
		return false
	}

	if sf.config.MaxAge != cfg.MaxAge {
		return false
	}

	return true
}

// TLSHandler object
type TLSHandler struct {
	clientHelloInfo *tls.ClientHelloInfo
}

// GetClientInfo Callback function to set ClientHelloInfo
// Must comply with the method structure of https://cs.opensource.google/go/go/+/refs/tags/go1.20:src/crypto/tls/common.go;l=554-563
// Since we overlay the method of the TLS config in the listener method
func (t *TLSHandler) GetClientInfo(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
	t.clientHelloInfo = info
	return nil, nil //nolint:nilnil // Not returning anything useful here is probably fine
}

// Range data for c.Range
type Range struct {
	Type   string
	Ranges []RangeSet
}

// RangeSet represents a single content range from a request.
type RangeSet struct {
	Start int
	End   int
}

// Cookie data for c.Cookie
type Cookie struct {
	Expires     time.Time `json:"expires"`      // The expiration date of the cookie
	Name        string    `json:"name"`         // The name of the cookie
	Value       string    `json:"value"`        // The value of the cookie
	Path        string    `json:"path"`         // Specifies a URL path which is allowed to receive the cookie
	Domain      string    `json:"domain"`       // Specifies the domain which is allowed to receive the cookie
	SameSite    string    `json:"same_site"`    // Controls whether or not a cookie is sent with cross-site requests
	MaxAge      int       `json:"max_age"`      // The maximum age (in seconds) of the cookie
	Secure      bool      `json:"secure"`       // Indicates that the cookie should only be transmitted over a secure HTTPS connection
	HTTPOnly    bool      `json:"http_only"`    // Indicates that the cookie is accessible only through the HTTP protocol
	Partitioned bool      `json:"partitioned"`  // Indicates if the cookie is stored in a partitioned cookie jar
	SessionOnly bool      `json:"session_only"` // Indicates if the cookie is a session-only cookie
}

// Views is the interface that wraps the Render function.
type Views interface {
	Load() error
	Render(out io.Writer, name string, binding any, layout ...string) error
}

// ResFmt associates a Content Type to a fiber.Handler for c.Format
type ResFmt struct {
	Handler   func(Ctx) error
	MediaType string
}

// App returns the *App reference to the instance of the Fiber application
func (c *DefaultCtx) App() *App {
	return c.app
}

// Accepts checks if the specified extensions or content types are acceptable.
func (c *DefaultCtx) Accepts(offers ...string) string {
	return c.Req().Accepts(offers...)
}

// BaseURL returns (protocol + host + base path).
func (c *DefaultCtx) BaseURL() string {
	// TODO: Could be improved: 53.8 ns/op  32 B/op  1 allocs/op
	// Should work like https://codeigniter.com/user_guide/helpers/url_helper.html
	if c.baseURI != "" {
		return c.baseURI
	}
	c.baseURI = c.Scheme() + "://" + c.Host()
	return c.baseURI
}

// Bind You can bind body, cookie, headers etc. into the map, map slice, struct easily by using Binding method.
// It gives custom binding support, detailed binding options and more.
// Replacement of: BodyParser, ParamsParser, GetReqHeaders, GetRespHeaders, AllParams, QueryParser, ReqHeaderParser
func (c *DefaultCtx) Bind() *Bind {
	if c.bind == nil {
		c.bind = &Bind{
			ctx:            c,
			dontHandleErrs: true,
		}
	}
	return c.bind
}

// Deadline returns the time when work done on behalf of this context
// should be canceled. Deadline returns ok==false when no deadline is
// set. Successive calls to Deadline return the same results.
//
// Due to current limitations in how fasthttp works, Deadline operates as a nop.
// See: https://github.com/valyala/fasthttp/issues/965#issuecomment-777268945
func (*DefaultCtx) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

// Done returns a channel that's closed when work done on behalf of this
// context should be canceled. Done may return nil if this context can
// never be canceled. Successive calls to Done return the same value.
// The close of the Done channel may happen asynchronously,
// after the cancel function returns.
//
// Due to current limitations in how fasthttp works, Done operates as a nop.
// See: https://github.com/valyala/fasthttp/issues/965#issuecomment-777268945
func (*DefaultCtx) Done() <-chan struct{} {
	return nil
}

// If Done is not yet closed, Err returns nil.
// If Done is closed, Err returns a non-nil error explaining why:
// context.DeadlineExceeded if the context's deadline passed,
// or context.Canceled if the context was canceled for some other reason.
// After Err returns a non-nil error, successive calls to Err return the same error.
//
// Due to current limitations in how fasthttp works, Err operates as a nop.
// See: https://github.com/valyala/fasthttp/issues/965#issuecomment-777268945
func (*DefaultCtx) Err() error {
	return nil
}

func (c *DefaultCtx) Get(key string, defaultValue ...string) string {
	return c.Req().Get(key, defaultValue...)
}

func (c *DefaultCtx) GetRespHeader(key string, defaultValue ...string) string {
	return c.Res().Get(key, defaultValue...)
}

// ClientHelloInfo return CHI from context
func (c *DefaultCtx) ClientHelloInfo() *tls.ClientHelloInfo {
	if c.app.tlsHandler != nil {
		return c.app.tlsHandler.clientHelloInfo
	}

	return nil
}

// Cookies are used for getting a cookie value by key.
// Defaults to the empty string "" if the cookie doesn't exist.
// If a default value is given, it will return that value if the cookie doesn't exist.
// The returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (c *DefaultCtx) Cookies(key string, defaultValue ...string) string {
	return c.Req().Cookies(key, defaultValue...)
}

// Method returns the HTTP request method for the context, optionally overridden by the provided argument.
// If no override is given or if the provided override is not a valid HTTP method, it returns the current method from the context.
// Otherwise, it updates the context's method and returns the overridden method as a string.
func (c *DefaultCtx) Method(override ...string) string {
	if len(override) == 0 {
		// Nothing to override, just return current method from context
		return c.App().method(c.methodInt)
	}

	method := utils.ToUpper(override[0])
	methodInt := c.App().methodInt(method)
	if methodInt == -1 {
		// Provided override does not valid HTTP method, no override, return current method
		return c.App().method(c.methodInt)
	}
	c.methodInt = methodInt
	return method
}

// Next executes the next method in the stack that matches the current route.
func (c *DefaultCtx) Next() error {
	// Increment handler index
	c.indexHandler++

	// Did we execute all route handlers?
	if c.indexHandler < len(c.route.Handlers) {
		// Continue route stack
		return c.route.Handlers[c.indexHandler](c)
	}

	// Continue handler stack
	if c.app.newCtxFunc != nil {
		_, err := c.app.nextCustom(c)
		return err
	}

	_, err := c.app.next(c)
	return err
}

// RestartRouting instead of going to the next handler. This may be useful after
// changing the request path. Note that handlers might be executed again.
func (c *DefaultCtx) RestartRouting() error {
	var err error

	c.indexRoute = -1
	if c.app.newCtxFunc != nil {
		_, err = c.app.nextCustom(c)
	} else {
		_, err = c.app.next(c)
	}
	return err
}

// OriginalURL contains the original request URL.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (c *DefaultCtx) OriginalURL() string {
	return c.App().getString(c.Request().Header.RequestURI())
}

// Path returns the path part of the request URL.
// Optionally, you could override the path.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (c *DefaultCtx) Path(override ...string) string {
	if len(override) != 0 && string(c.path) != override[0] {
		// Set new path to context
		c.pathOriginal = override[0]

		// Set new path to request context
		c.fasthttp.Request.URI().SetPath(c.pathOriginal)
		// Prettify path
		c.configDependentPaths()
	}
	return c.app.getString(c.path)
}

// Req returns a convenience type whose API is limited to operations
// on the incoming request.
func (c *DefaultCtx) Req() Req {
	return &c.DefaultReq
}

// Res returns a convenience type whose API is limited to operations
// on the outgoing response.
func (c *DefaultCtx) Res() Res {
	return &c.DefaultRes
}

// RequestCtx returns *fasthttp.RequestCtx that carries a deadline
// a cancellation signal, and other values across API boundaries.
func (c *DefaultCtx) RequestCtx() *fasthttp.RequestCtx {
	return c.fasthttp
}

// Request return the *fasthttp.Request object
// This allows you to use all fasthttp request methods
// https://godoc.org/github.com/valyala/fasthttp#Request
func (c *DefaultCtx) Request() *fasthttp.Request {
	return &c.fasthttp.Request
}

// Response return the *fasthttp.Response object
// This allows you to use all fasthttp response methods
// https://godoc.org/github.com/valyala/fasthttp#Response
func (c *DefaultCtx) Response() *fasthttp.Response {
	return &c.fasthttp.Response
}

// Redirect returns the Redirect reference.
// Use Redirect().Status() to set custom redirection status code.
// If status is not specified, status defaults to 303 See Other.
// You can use Redirect().To(), Redirect().Route() and Redirect().Back() for redirection.
func (c *DefaultCtx) Redirect() *Redirect {
	if c.redirect == nil {
		c.redirect = AcquireRedirect()
		c.redirect.c = c
	}

	return c.redirect
}

// Route returns the matched Route struct.
func (c *DefaultCtx) Route() *Route {
	if c.route == nil {
		// Fallback for fasthttp error handler
		return &Route{
			path:     c.pathOriginal,
			Path:     c.pathOriginal,
			Method:   c.Method(),
			Handlers: make([]Handler, 0),
			Params:   make([]string, 0),
		}
	}
	return c.route
}

// SaveFile saves any multipart file to disk.
func (*DefaultCtx) SaveFile(fileheader *multipart.FileHeader, path string) error {
	return fasthttp.SaveMultipartFile(fileheader, path)
}

// SaveFileToStorage saves any multipart file to an external storage system.
func (*DefaultCtx) SaveFileToStorage(fileheader *multipart.FileHeader, path string, storage Storage) error {
	file, err := fileheader.Open()
	if err != nil {
		return fmt.Errorf("failed to open: %w", err)
	}
	defer file.Close() //nolint:errcheck // not needed

	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read: %w", err)
	}

	if err := storage.Set(path, content, 0); err != nil {
		return fmt.Errorf("failed to store: %w", err)
	}

	return nil
}

// Secure returns whether a secure connection was established.
func (c *DefaultCtx) Secure() bool {
	return c.Protocol() == schemeHTTPS
}

// Status sets the HTTP status for the response.
// This method is chainable.
func (c *DefaultCtx) Status(status int) Ctx {
	c.fasthttp.Response.SetStatusCode(status)
	return c
}

// String returns unique string representation of the ctx.
//
// The returned value may be useful for logging.
func (c *DefaultCtx) String() string {
	// Get buffer from pool
	buf := bytebufferpool.Get()

	// Start with the ID, converting it to a hex string without fmt.Sprintf
	buf.WriteByte('#')
	// Convert ID to hexadecimal
	id := strconv.FormatUint(c.fasthttp.ID(), 16)
	// Pad with leading zeros to ensure 16 characters
	for i := 0; i < (16 - len(id)); i++ {
		buf.WriteByte('0')
	}
	buf.WriteString(id)
	buf.WriteString(" - ")

	// Add local and remote addresses directly
	buf.WriteString(c.fasthttp.LocalAddr().String())
	buf.WriteString(" <-> ")
	buf.WriteString(c.fasthttp.RemoteAddr().String())
	buf.WriteString(" - ")

	// Add method and URI
	buf.Write(c.fasthttp.Request.Header.Method())
	buf.WriteByte(' ')
	buf.Write(c.fasthttp.URI().FullURI())

	// Allocate string
	str := buf.String()

	// Reset buffer
	buf.Reset()
	bytebufferpool.Put(buf)

	return str
}

// Value makes it possible to retrieve values (Locals) under keys scoped to the request
// and therefore available to all following routes that match the request.
func (c *DefaultCtx) Value(key any) any {
	return c.fasthttp.UserValue(key)
}

// XHR returns a Boolean property, that is true, if the request's X-Requested-With header field is XMLHttpRequest,
// indicating that the request was issued by a client library (such as jQuery).
func (c *DefaultCtx) XHR() bool {
	return utils.EqualFold(c.app.getBytes(c.Get(HeaderXRequestedWith)), []byte("xmlhttprequest"))
}

// configDependentPaths set paths for route recognition and prepared paths for the user,
// here the features for caseSensitive, decoded paths, strict paths are evaluated
func (c *DefaultCtx) configDependentPaths() {
	c.path = append(c.path[:0], c.pathOriginal...)
	// If UnescapePath enabled, we decode the path and save it for the framework user
	if c.app.config.UnescapePath {
		c.path = fasthttp.AppendUnquotedArg(c.path[:0], c.path)
	}

	// another path is specified which is for routing recognition only
	// use the path that was changed by the previous configuration flags
	c.detectionPath = append(c.detectionPath[:0], c.path...)
	// If CaseSensitive is disabled, we lowercase the original path
	if !c.app.config.CaseSensitive {
		c.detectionPath = utils.ToLowerBytes(c.detectionPath)
	}
	// If StrictRouting is disabled, we strip all trailing slashes
	if !c.app.config.StrictRouting && len(c.detectionPath) > 1 && c.detectionPath[len(c.detectionPath)-1] == '/' {
		c.detectionPath = utils.TrimRight(c.detectionPath, '/')
	}

	// Define the path for dividing routes into areas for fast tree detection, so that fewer routes need to be traversed,
	// since the first three characters area select a list of routes
	c.treePathHash = 0
	if len(c.detectionPath) >= maxDetectionPaths {
		c.treePathHash = int(c.detectionPath[0])<<16 |
			int(c.detectionPath[1])<<8 |
			int(c.detectionPath[2])
	}
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
	// Set method
	c.methodInt = c.app.methodInt(utils.UnsafeString(fctx.Request.Header.Method()))
	// Attach *fasthttp.RequestCtx to ctx
	c.fasthttp = fctx
	// reset base uri
	c.baseURI = ""
	// Prettify path
	c.configDependentPaths()
	c.reset(c)
}

// Reset is a method to reset context fields by given request when to use server handlers.
func (c *DefaultCtx) reset(ctx Ctx) {
	c.DefaultReq.reset(c)
	c.DefaultRes.reset(c)
}

// ViewBind Add vars to default view var map binding to template engine.
// Variables are read by the Render method and may be overwritten.
func (c *DefaultCtx) ViewBind(vars Map) error {
	// init viewBindMap - lazy map
	for k, v := range vars {
		c.viewBindMap.Store(k, v)
	}
	return nil
}

// Release is a method to reset context fields when to use ReleaseCtx()
func (c *DefaultCtx) release() {
	c.route = nil
	c.fasthttp = nil
	c.bind = nil
	c.flashMessages = c.flashMessages[:0]
	c.viewBindMap = sync.Map{}
	if c.redirect != nil {
		ReleaseRedirect(c.redirect)
		c.redirect = nil
	}
	c.DefaultReq.release()
	c.DefaultRes.release()
}

func (c *DefaultCtx) renderExtensions(bind any) {
	if bindMap, ok := bind.(Map); ok {
		// Bind view map
		c.viewBindMap.Range(func(key, value any) bool {
			keyValue, ok := key.(string)
			if !ok {
				return true
			}
			if _, ok := bindMap[keyValue]; !ok {
				bindMap[keyValue] = value
			}
			return true
		})

		// Check if the PassLocalsToViews option is enabled (by default it is disabled)
		if c.App().config.PassLocalsToViews {
			// Loop through each local and set it in the map
			c.RequestCtx().VisitUserValues(func(key []byte, val any) {
				// check if bindMap doesn't contain the key
				if _, ok := bindMap[c.App().getString(key)]; !ok {
					// Set the key and value in the bindMap
					bindMap[c.App().getString(key)] = val
				}
			})
		}
	}

	if len(c.App().mountFields.appListKeys) == 0 {
		c.App().generateAppListKeys()
	}
}

// Methods to use with next stack.
func (c *DefaultCtx) getMethodInt() int {
	return c.methodInt
}

func (c *DefaultCtx) setMethodInt(methodInt int) {
	c.methodInt = methodInt
}

func (c *DefaultCtx) getIndexRoute() int {
	return c.indexRoute
}

func (c *DefaultCtx) getTreePathHash() int {
	return c.treePathHash
}

func (c *DefaultCtx) getDetectionPath() string {
	return c.app.getString(c.detectionPath)
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

func (c *DefaultCtx) keepOriginalPath() {
	c.pathOriginal = utils.CopyString(c.pathOriginal)
}
