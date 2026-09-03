// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net"
	"sync"
	"sync/atomic"
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
	_                  io.Writer       = (*DefaultCtx)(nil) // Compile-time check
	_                  context.Context = (*DefaultCtx)(nil) // Compile-time check
	emptyRouteHandlers [0]Handler
	emptyRouteParams   [0]string
)

// The contextKey type is unexported to prevent collisions with context keys defined in
// other packages.
type contextKey int

// userContextKey define the key name for storing context.Context in *fasthttp.RequestCtx
const (
	userContextKey contextKey = iota // __local_user_context__
)

// DefaultCtx is the default implementation of the Ctx interface
// generation tool `go install github.com/vburenin/ifacemaker@f30b6f9bdbed4b5c4804ec9ba4a04a999525c202`
// https://github.com/vburenin/ifacemaker/blob/f30b6f9bdbed4b5c4804ec9ba4a04a999525c202/ifacemaker.go#L14-L31
//
//go:generate ifacemaker --file ctx.go --file req.go --file res.go --struct DefaultCtx --iface Ctx --pkg fiber --promoted --output ctx_interface_gen.go --not-exported true --iface-comment "Ctx represents the Context which hold the HTTP request and response.\nIt has methods for the request query string, parameters, body, HTTP headers and so on."
type DefaultCtx struct {
	handlerCtx             CustomCtx            // Active custom context implementation, if any
	DefaultReq                                  // Default request api
	DefaultRes                                  // Default response api
	app                    *App                 // Reference to *App
	route                  *Route               // Reference to *Route
	fasthttp               *fasthttp.RequestCtx // Reference to *fasthttp.RequestCtx
	bind                   *Bind                // Default bind reference
	redirect               *Redirect            // Default redirect reference
	reclaim                *reclaimLatch        // Coordinates safe pool reclamation of an abandoned ctx; nil on the hot path
	viewBindMap            Map                  // Default view map to bind template engine
	values                 [maxParams]string    // Route parameter values
	baseURI                string               // HTTP base uri
	pathOriginal           string               // Original HTTP path
	flashMessages          redirectionMsgs      // Flash messages
	path                   []byte               // HTTP path with the modifications by the configuration
	detectionPath          []byte               // Route detection path
	treePathHash           int                  // Hash of the path for the search in the tree
	pathSlashes            int                  // Number of '/' in the detection path, used to quick-reject routes
	indexRoute             int                  // Index of the current route
	indexHandler           int                  // Index of the current handler
	firstMatchIndex        int                  // Pre-resolved endpoint index from the SkipUnmatchedRoutes lookahead; -1 when unused
	methodInt              int                  // HTTP method INT equivalent
	isAbandoned            atomic.Bool          // If true, ctx won't be pooled until ForceRelease is called
	isMatched              bool                 // Non use route matched
	shouldSkipNonUseRoutes bool                 // Skip non-use routes while iterating middleware
	isUserContextSet       bool                 // User context was stored in fasthttp user values
}

// TLSHandler hosts the callback hooks Fiber invokes while negotiating TLS
// connections, including optional client certificate lookups.
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

// Views is the interface that wraps the Render function.
type Views interface {
	Load() error
	Render(out io.Writer, name string, binding any, layout ...string) error
}

// App returns the *App reference to the instance of the Fiber application
func (c *DefaultCtx) App() *App {
	return c.app
}

// BaseURL returns (protocol + host + base path).
func (c *DefaultCtx) BaseURL() string {
	// TODO: Could be improved: 53.8 ns/op  32 B/op  1 allocs/op
	// Should work like https://codeigniter.com/user_guide/helpers/url_helper.html
	if c.baseURI != "" {
		return c.baseURI
	}
	scheme := c.Scheme()
	host := c.Host()
	buf := make([]byte, 0, len(scheme)+len("://")+len(host))
	buf = append(buf, scheme...)
	buf = append(buf, "://"...)
	buf = append(buf, host...)
	c.baseURI = c.app.toString(buf)
	return c.baseURI
}

// RequestCtx returns *fasthttp.RequestCtx that carries a deadline
// a cancellation signal, and other values across API boundaries.
func (c *DefaultCtx) RequestCtx() *fasthttp.RequestCtx {
	return c.fasthttp
}

// Context returns a context implementation that was set by
// user earlier or returns a non-nil, empty context, if it was not set earlier.
func (c *DefaultCtx) Context() context.Context {
	if c.fasthttp == nil {
		return context.Background()
	}
	if ctx, ok := c.fasthttp.UserValue(userContextKey).(context.Context); ok && ctx != nil {
		return ctx
	}
	ctx := context.Background()
	c.SetContext(ctx)
	return ctx
}

// SetContext sets a context implementation by user.
func (c *DefaultCtx) SetContext(ctx context.Context) {
	if c.fasthttp == nil {
		return
	}
	c.fasthttp.SetUserValue(userContextKey, ctx)
	c.isUserContextSet = true
}

// Deadline returns the time when work done on behalf of this context
// should be canceled. Ctx carries no deadline, so ok is always false.
//
// Ctx satisfies context.Context as a context that can never be canceled: it is
// pooled and reused, so it cannot honor the stability and concurrency the
// interface requires. Pass Context() to anything that is cancellation-aware or
// that outlives the handler.
func (*DefaultCtx) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

// Done returns a channel that's closed when work done on behalf of this
// context should be canceled. Ctx can never be canceled, so Done always
// returns nil, which context.Context explicitly permits. See Deadline.
func (*DefaultCtx) Done() <-chan struct{} {
	return nil
}

// Err returns nil until the Done channel is closed. Done is always nil here,
// so Err always returns nil. See Deadline.
func (*DefaultCtx) Err() error {
	return nil
}

// Request return the *fasthttp.Request object
// This allows you to use all fasthttp request methods
// https://godoc.org/github.com/valyala/fasthttp#Request
// Returns nil if the context has been released.
func (c *DefaultCtx) Request() *fasthttp.Request {
	if c.fasthttp == nil {
		return nil
	}
	return &c.fasthttp.Request
}

// Response return the *fasthttp.Response object
// This allows you to use all fasthttp response methods
// https://godoc.org/github.com/valyala/fasthttp#Response
// Returns nil if the context has been released.
func (c *DefaultCtx) Response() *fasthttp.Response {
	if c.fasthttp == nil {
		return nil
	}
	return &c.fasthttp.Response
}

// Body returns the request body, decompressing it when the request declares a
// Content-Encoding the app accepts. Req and Res both carry a Body, so on Ctx
// the request wins, as it does for Get; use Res().Body() for the response.
func (c *DefaultCtx) Body() []byte {
	return c.DefaultReq.Body()
}

// ContentLength returns the value of the Content-Length request header. Req and
// Res both carry one, so on Ctx the request wins, as it does for Get; use
// Res().ContentLength() for the response.
func (c *DefaultCtx) ContentLength() int {
	return c.DefaultReq.ContentLength()
}

// ContentType returns the Content-Type request header, parameters included. Req
// and Res both carry one, so on Ctx the request wins, as it does for Get; use
// Res().ContentType() for the response.
func (c *DefaultCtx) ContentType() string {
	return c.DefaultReq.ContentType()
}

// Cookies returns the request cookie with the given key, or defaultValue. Only
// valid within the handler unless Immutable is set. For the cookies the response
// is set to send, use Res().GetCookies().
func (c *DefaultCtx) Cookies(key string, defaultValue ...string) string {
	return c.DefaultReq.Cookies(key, defaultValue...)
}

// Get returns the HTTP request header specified by field.
// Field names are case-insensitive
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (c *DefaultCtx) Get(key string, defaultValue ...string) string {
	return c.DefaultReq.Get(key, defaultValue...)
}

// GetHeaders returns the HTTP request headers.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (c *DefaultCtx) GetHeaders() map[string][]string {
	return c.DefaultReq.GetHeaders()
}

// GetReqHeaders returns the HTTP request headers.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (c *DefaultCtx) GetReqHeaders() map[string][]string {
	return c.DefaultReq.GetHeaders()
}

// GetRespHeader returns the HTTP response header specified by field.
// Field names are case-insensitive
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (c *DefaultCtx) GetRespHeader(key string, defaultValue ...string) string {
	return c.DefaultRes.Get(key, defaultValue...)
}

// GetRespHeaders returns the HTTP response headers.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (c *DefaultCtx) GetRespHeaders() map[string][]string {
	return c.DefaultRes.GetHeaders()
}

// ClientHelloInfo return CHI from context
func (c *DefaultCtx) ClientHelloInfo() *tls.ClientHelloInfo {
	if c.app.tlsHandler != nil {
		return c.app.tlsHandler.clientHelloInfo
	}

	return nil
}

// Next executes the next method in the stack that matches the current route.
func (c *DefaultCtx) Next() error {
	// Increment handler index
	c.indexHandler++

	// Did we execute all route handlers?
	if c.indexHandler < len(c.route.Handlers) {
		if c.handlerCtx != nil {
			return c.route.Handlers[c.indexHandler](c.handlerCtx)
		}
		return c.route.Handlers[c.indexHandler](c)
	}

	if c.handlerCtx != nil {
		_, err := c.app.nextCustom(c.handlerCtx)
		return err
	}
	_, err := c.app.next(c)
	return err
}

// RestartRouting instead of going to the next handler. This may be useful after
// changing the request path. Note that handlers might be executed again.
func (c *DefaultCtx) RestartRouting() error {
	c.indexRoute = -1
	// Path may have changed; invalidate the lookahead index
	c.firstMatchIndex = -1
	if c.handlerCtx != nil {
		_, err := c.app.nextCustom(c.handlerCtx)
		return err
	}
	_, err := c.app.next(c)
	return err
}

func (c *DefaultCtx) setHandlerCtx(ctx CustomCtx) {
	if ctx == nil {
		c.handlerCtx = nil
		return
	}
	if defaultCtx, ok := ctx.(*DefaultCtx); ok && defaultCtx == c {
		c.handlerCtx = nil
		return
	}
	c.handlerCtx = ctx
}

// OriginalURL contains the original request URL.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (c *DefaultCtx) OriginalURL() string {
	return c.app.toString(c.fasthttp.Request.Header.RequestURI())
}

// RequestID returns the request identifier from the response header or request header.
func (c *DefaultCtx) RequestID() string {
	if requestID := c.GetRespHeader(HeaderXRequestID); requestID != "" {
		return requestID
	}
	return c.Get(HeaderXRequestID)
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

// ViewBind Add vars to default view var map binding to template engine.
// Variables are read by the Render method and may be overwritten.
func (c *DefaultCtx) ViewBind(vars Map) error {
	// init viewBindMap - lazy map
	if c.viewBindMap == nil {
		c.viewBindMap = make(Map, len(vars))
	}
	maps.Copy(c.viewBindMap, vars)
	return nil
}

// Route returns the matched Route struct.
func (c *DefaultCtx) Route() *Route {
	if c.route == nil {
		// Cold path kept out of line so Route stays within the inlining budget.
		return c.routeFallback()
	}
	return c.route
}

// routeFallback builds the synthetic route for the fasthttp error handler.
// Its Method field is resolved like c.Method() (including the raw-header
// fallback for unregistered methods) so Route and Method always agree.
// Never inlined: inlining it would push Route over the inlining budget.
//
//go:noinline
func (c *DefaultCtx) routeFallback() *Route {
	return &Route{
		path:     c.pathOriginal,
		Path:     c.pathOriginal,
		Method:   currentMethod(c),
		Handlers: emptyRouteHandlers[:],
		Params:   emptyRouteParams[:],
	}
}

// Endpoint returns the route that will handle this request, without advancing the
// handler chain, so global middleware can read its Path or Name before calling
// Next. Returns nil when no endpoint will run: 404, 405, and while the error
// handler replays the chain for a request rejected at the protocol level.
//
// It looks ahead, where the neighboring accessors look back: Route reports the
// route currently executing, which inside middleware is the middleware itself,
// and Matched reports whether an endpoint has been selected yet.
//
// It scans the remaining routes in the request's tree bucket, so calling it from
// global middleware costs a second router scan per request.
func (c *DefaultCtx) Endpoint() *Route {
	// Already on a non-middleware endpoint.
	if c.route != nil && !c.route.use && !c.route.mount {
		return c.route
	}
	if c.methodInt == -1 || c.app == nil {
		return nil
	}
	// serverErrorHandler replays the chain with this set, and next() then lets no
	// endpoint run, so naming one here would promise a handler that cannot run.
	if c.shouldSkipNonUseRoutes {
		return nil
	}

	tree := c.app.treeIndex[c.methodInt].lookup(c.treePathHash)
	detectionPath := utils.UnsafeString(c.detectionPath)
	path := utils.UnsafeString(c.path)
	head := pathHeadWord(detectionPath)
	pathSlashes := c.pathSlashCount(c.app)

	// SkipUnmatchedRoutes already resolved the endpoint, but the index only answers
	// while routing has not walked past it, and a route registered mid-request can
	// shift the bucket under it, so it still has to clear the prefix filter.
	if c.firstMatchIndex > c.indexRoute && c.firstMatchIndex < len(tree) {
		if route := tree[c.firstMatchIndex]; route != nil && !route.use && !route.mount &&
			!route.prefixRejects(head) {
			return route
		}
	}

	// Use a scratch params buffer so look-ahead does not clobber c.values.
	var scratch [maxParams]string

	// Starting past indexRoute follows the chain, which is why the result needs no
	// cache: every mutation that would invalidate one already moves a field read here.
	for i := c.indexRoute + 1; i < len(tree); i++ {
		route := tree[i]
		if route.mount || route.use {
			continue
		}
		if route.prefixRejects(head) {
			continue
		}
		if route.match(detectionPath, path, &scratch, pathSlashes) {
			return route
		}
	}

	return nil
}

// FullPath returns the matched route path, including any group prefixes.
func (c *DefaultCtx) FullPath() string {
	return c.Route().Path
}

// RouteName returns the name of the route currently executing, or "" when it is
// unnamed. Inside middleware that is the middleware's own route; use
// Endpoint().Name to look ahead to the one that will handle the request.
func (c *DefaultCtx) RouteName() string {
	// Route() builds a synthetic Route when none matched, and that one never
	// carries a Name — so the allocation could only ever produce "".
	if c.route == nil {
		return ""
	}
	return c.route.Name
}

// MountPath returns the prefix the sub-app owning the current route was mounted
// under, or "" for a top-level route. Path is not relative to it. One *App
// mounted twice reports its last prefix, as App.MountPath does.
func (c *DefaultCtx) MountPath() string {
	if c.app == nil {
		return ""
	}
	// A route with no recorded owner is this app's own, so it is not under a
	// mount at all — reading the app's own prefix here would answer with a
	// mount the request never went through.
	owner := c.app.routeOwner(c.route)
	if owner == nil {
		return ""
	}

	return owner.MountPath()
}

// Matched returns true if the current request path was matched by the router.
func (c *DefaultCtx) Matched() bool {
	return c.getMatched()
}

// IsMiddleware returns true if the current request handler was registered as middleware.
func (c *DefaultCtx) IsMiddleware() bool {
	if c.route == nil {
		return false
	}
	if c.route.use {
		return true
	}
	// For route-level middleware, there will be a next handler in the chain
	return c.indexHandler+1 < len(c.route.Handlers)
}

// IsFinal reports whether this is the last handler of a matched non-middleware
// route, so nothing further on that route runs. It describes the route, not the
// request: another route can still match, and a Use route is never final.
func (c *DefaultCtx) IsFinal() bool {
	return c.route != nil && !c.IsMiddleware()
}

// OverrideParam overwrites a route parameter value by name.
// If the parameter name does not exist in the route, this method does nothing.
func (c *DefaultCtx) OverrideParam(name, value string) {
	// If no route is matched, there are no parameters to update
	if !c.Matched() {
		return
	}

	// Normalize wildcard (*) and plus (+) tokens to their internal
	// representations (*1, +1) used by the router.
	if name == "*" || name == "+" {
		name += "1"
	}

	if c.app.config.CaseSensitive {
		for i, param := range c.route.Params {
			if param == name {
				c.values[i] = value
				return
			}
		}
		return
	}

	nameBytes := utils.UnsafeBytes(name)
	for i, param := range c.route.Params {
		if utils.EqualFold(utils.UnsafeBytes(param), nameBytes) {
			c.values[i] = value
			return
		}
	}
}

// SaveFile saves any multipart file to disk.
func (*DefaultCtx) SaveFile(fileheader *multipart.FileHeader, path string) error {
	if fileheader == nil {
		return ErrFileHeaderNil
	}
	return fasthttp.SaveMultipartFile(fileheader, path)
}

// SaveFileToStorage saves any multipart file to an external storage system.
func (c *DefaultCtx) SaveFileToStorage(fileheader *multipart.FileHeader, path string, storage Storage) error {
	if fileheader == nil {
		return ErrFileHeaderNil
	}

	file, err := fileheader.Open()
	if err != nil {
		return fmt.Errorf("%w: %q: %w", ErrFileOpen, fileheader.Filename, err)
	}
	defer file.Close() //nolint:errcheck // not needed

	maxUploadSize := c.app.config.BodyLimit
	if maxUploadSize <= 0 {
		maxUploadSize = DefaultBodyLimit
	}

	if fileheader.Size > 0 && fileheader.Size > int64(maxUploadSize) {
		return fmt.Errorf("%w: %q: %w", ErrFileRead, fileheader.Filename, fasthttp.ErrBodyTooLarge)
	}

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	limitedReader := io.LimitReader(file, int64(maxUploadSize)+1)
	if _, err = buf.ReadFrom(limitedReader); err != nil {
		return fmt.Errorf("%w: %q: %w", ErrFileRead, fileheader.Filename, err)
	}

	if buf.Len() > maxUploadSize {
		return fmt.Errorf("%w: %q: %w", ErrFileRead, fileheader.Filename, fasthttp.ErrBodyTooLarge)
	}

	data := append([]byte(nil), buf.Bytes()...)

	if err := storage.SetWithContext(c.Context(), path, data, 0); err != nil {
		return fmt.Errorf("%w: %q to %q: %w", ErrFileStore, fileheader.Filename, path, err)
	}

	return nil
}

// Error returns an *Error carrying the given status code, defaulting the message
// to the status text. Returning it hands it to the app's ErrorHandler, which
// writes the response; Error itself sets nothing.
func (*DefaultCtx) Error(status int, message ...string) error {
	return NewError(status, message...)
}

// Status sets the HTTP status for the response.
// This method is chainable.
func (c *DefaultCtx) Status(status int) Ctx {
	c.fasthttp.Response.SetStatusCode(status)
	return c
}

// ID returns the connection-unique identifier fasthttp assigned to this request,
// unlike RequestID, which reads a header. It is not unique across processes or
// restarts, so use it to correlate log lines within one server run.
func (c *DefaultCtx) ID() uint64 {
	return c.fasthttp.ID()
}

// StartTime returns the time the server began handling this request. It is the
// reference point Elapsed measures from.
func (c *DefaultCtx) StartTime() time.Time {
	return c.fasthttp.Time()
}

// Elapsed returns how long this request has been handled so far, measured from
// StartTime. Called after the handler chain has run, it is the request latency.
func (c *DefaultCtx) Elapsed() time.Duration {
	return time.Since(c.fasthttp.Time())
}

// LocalAddr returns the server-side address of the connection this request
// arrived on. IP returns the client address as a string; this is the full
// net.Addr, so the port, network, and unix socket path survive.
func (c *DefaultCtx) LocalAddr() net.Addr {
	return c.fasthttp.LocalAddr()
}

// RemoteAddr returns the address of the immediate peer, which is the proxy
// rather than the client when the app sits behind one. Use IP or IPs for the
// client address a trusted proxy forwarded.
func (c *DefaultCtx) RemoteAddr() net.Addr {
	return c.fasthttp.RemoteAddr()
}

// Hijack registers a handler that takes over the connection once the response is
// sent, for protocols Fiber does not speak. The connection then closes unless
// KeepHijackedConns is set, and the handler must not touch the pooled Ctx.
func (c *DefaultCtx) Hijack(handler fasthttp.HijackHandler) {
	c.fasthttp.Hijack(handler)
}

// Hijacked returns true if Hijack has been called on this request, so a later
// handler can tell that the connection is already spoken for and leave the
// response alone.
func (c *DefaultCtx) Hijacked() bool {
	return c.fasthttp.Hijacked()
}

// String returns unique string representation of the ctx.
//
// The returned value may be useful for logging.
func (c *DefaultCtx) String() string {
	// Get buffer from pool
	buf := bytebufferpool.Get()

	// Start with the ID, converting it to a hex string without fmt.Sprintf
	buf.WriteByte('#')
	const hex = "0123456789abcdef"
	var id [16]byte
	ctxID := c.fasthttp.ID()
	for i := len(id) - 1; i >= 0; i-- {
		id[i] = hex[ctxID&0xf]
		ctxID >>= 4
	}
	buf.Write(id[:])
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
// and therefore available to all following routes that match the request. If the context
// has been released and c.fasthttp is nil (for example, after ReleaseCtx), Value returns nil.
func (c *DefaultCtx) Value(key any) any {
	if c.fasthttp == nil {
		return nil
	}
	return c.fasthttp.UserValue(key)
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
	// If CaseSensitive is disabled, we lowercase the original path while
	// copying it, fusing the copy and the case fold into a single pass.
	if !c.app.config.CaseSensitive {
		c.detectionPath = appendLowerASCII(c.detectionPath[:0], c.path)
	} else {
		c.detectionPath = append(c.detectionPath[:0], c.path...)
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

	// Invalidate the cached slash count of the detection path; pathSlashCount
	// recomputes it lazily when route matching first needs it.
	c.pathSlashes = 0
}

// Reset is a method to reset context fields by given request when to use server handlers.
func (c *DefaultCtx) Reset(fctx *fasthttp.RequestCtx) {
	// Reset route and handler index
	c.indexRoute = -1
	c.indexHandler = 0
	// Reset matched flag
	c.isMatched = false
	c.shouldSkipNonUseRoutes = false
	c.firstMatchIndex = -1
	c.route = nil
	// Set paths
	c.pathOriginal = c.app.toString(fctx.URI().PathOriginal())
	// Set method
	c.methodInt = c.app.methodInt(utils.UnsafeString(fctx.Request.Header.Method()))
	// Attach *fasthttp.RequestCtx to ctx
	c.fasthttp = fctx
	// reset base uri
	c.baseURI = ""
	// Prettify path
	c.configDependentPaths()

	c.DefaultReq.c = c
	c.DefaultRes.c = c
}

// release is a method to reset context fields when to use ReleaseCtx()
func (c *DefaultCtx) release() {
	if c.isUserContextSet {
		if c.fasthttp != nil {
			c.fasthttp.SetUserValue(userContextKey, nil)
		}
		c.isUserContextSet = false
	}
	c.route = nil
	c.fasthttp = nil
	if c.bind != nil {
		ReleaseBind(c.bind)
		c.bind = nil
	}
	// Zero the whole backing array before pooling: what lives here is the previous
	// request's flash data, which for WithInput is its entire form.
	// parseAndClearFlashMessages clears too, since UnmarshalMsg re-slices this.
	clear(c.flashMessages[:cap(c.flashMessages)])
	c.flashMessages = c.flashMessages[:0]
	// Clear viewBindMap by deleting all keys (reuse underlying map if possible)
	if c.viewBindMap != nil {
		clear(c.viewBindMap)
	}
	if c.redirect != nil {
		ReleaseRedirect(c.redirect)
		c.redirect = nil
	}
	c.shouldSkipNonUseRoutes = false
	// performance: no need for using c.isAbandoned.Store(false) here, as it is always set to false when it was true in ForceRelease
	c.reclaim = nil
	c.handlerCtx = nil
}

// reclaimLatch coordinates the safe, automatic reclamation of an abandoned
// context back into the pool. It is armed only via ScheduleReclaim (currently by
// the timeout middleware) and stays nil on the common request path, so requests
// that are not abandoned pay no additional cost.
type reclaimLatch struct {
	releasedCh chan struct{} // closed once the request handler has released the ctx (event b)
	once       sync.Once     // guards the close-exactly-once of releasedCh
}

// Abandon marks this context as abandoned. An abandoned context will not be
// returned to the pool when ReleaseCtx is called.
//
// This is used by the timeout and SSE middlewares to return immediately while a
// goroutine continues using the context safely.
//
// Only call ForceRelease after Abandon if you can guarantee no other goroutine
// (including Fiber's requestHandler and ErrorHandler) will touch the context.
// Callers that cannot make that guarantee themselves can instead call
// ScheduleReclaim, which arranges a race-free ForceRelease once the handler has
// finished and the request handler has released the context.
func (c *DefaultCtx) Abandon() {
	c.isAbandoned.Store(true)
}

// IsAbandoned returns true if Abandon() was called on this context.
func (c *DefaultCtx) IsAbandoned() bool {
	return c.isAbandoned.Load()
}

// ForceRelease releases an abandoned context back to the pool.
// This MUST only be called after all goroutines (including requestHandler and
// ErrorHandler) have completely finished using this context. Calling it while
// any goroutine is still running causes races.
func (c *DefaultCtx) ForceRelease() {
	c.isAbandoned.Store(false)
	c.app.ReleaseCtx(c)
}

// ScheduleReclaim arms automatic reclamation of an abandoned context, returning
// it to the pool once it is safe to do so.
//
// handlerDone must be closed once the goroutine that still uses this context
// (for the timeout middleware, the handler goroutine) has completely finished.
// cancel, if non-nil, is the CancelFunc of the context installed for that
// goroutine and is invoked as soon as it finishes.
//
// ForceRelease is performed only after BOTH handlerDone is closed AND the request
// handler has released the context (signaled from ReleaseCtx/releaseDefaultCtx),
// which makes the reclamation race-free. If handlerDone never closes — a handler
// that never returns — the context is intentionally never reclaimed, because the
// handler still owns it.
//
// This method calls Abandon internally, so callers do not need to call Abandon
// separately. Calling Abandon before ScheduleReclaim is still safe (idempotent).
func (c *DefaultCtx) ScheduleReclaim(handlerDone <-chan struct{}, cancel context.CancelFunc) {
	c.Abandon()

	latch := &reclaimLatch{releasedCh: make(chan struct{})}
	c.reclaim = latch

	go func() {
		<-handlerDone
		if cancel != nil {
			cancel()
		}
		<-latch.releasedCh
		c.ForceRelease()
	}()
}

// signalReleased records that the request handler is done touching an abandoned,
// reclaim-armed context (event b). It is a no-op when reclamation was not armed
// and is safe to call multiple times.
func (c *DefaultCtx) signalReleased() {
	if c.reclaim != nil {
		c.reclaim.once.Do(func() {
			close(c.reclaim.releasedCh)
		})
	}
}

func (c *DefaultCtx) renderExtensions(bind any) {
	if bindMap, ok := bind.(Map); ok {
		// Bind view map
		for key, value := range c.viewBindMap {
			if _, ok := bindMap[key]; !ok {
				bindMap[key] = value
			}
		}

		// Check if the PassLocalsToViews option is enabled (by default it is disabled)
		if c.app.config.PassLocalsToViews {
			// Loop through each local and set it in the map
			c.fasthttp.VisitUserValues(func(key []byte, val any) {
				// check if bindMap doesn't contain the key
				if _, ok := bindMap[c.app.toString(key)]; !ok {
					// Set the key and value in the bindMap
					bindMap[c.app.toString(key)] = val
				}
			})
		}
	}

	c.app.mountFields.appListKeysOnce.Do(c.app.generateAppListKeys)
}

// Bind You can bind body, cookie, headers etc. into the map, map slice, struct easily by using Binding method.
// It gives custom binding support, detailed binding options and more.
// Replacement of: BodyParser, ParamsParser, GetReqHeaders, GetRespHeaders, AllParams, QueryParser, ReqHeaderParser
func (c *DefaultCtx) Bind() *Bind {
	if c.bind == nil {
		c.bind = AcquireBind()
	}
	c.bind.ctx = c
	return c.bind
}

// Methods to use with next stack.
func (c *DefaultCtx) getMethodInt() int {
	return c.methodInt
}

func (c *DefaultCtx) getIndexRoute() int {
	return c.indexRoute
}

func (c *DefaultCtx) getTreePathHash() int {
	return c.treePathHash
}

// pathSlashCount lazily counts the '/' bytes of the detection path and caches
// the result for the request; matching uses it to reject route candidates
// without walking their segments. app is the serving App, which can differ
// from c.app when an App value was copied. When it registers no route that
// consults the count, counting is skipped and 0 is returned — a real detection
// path always contains a '/', so 0 doubles as the "unknown" state that makes
// Route.match skip the quick-reject entirely.
func (c *DefaultCtx) pathSlashCount(app *App) int {
	if c.pathSlashes == 0 && app.hasParamRoutes {
		c.pathSlashes = bytes.Count(c.detectionPath, slashDelimiterBytes)
	}
	return c.pathSlashes
}

func (c *DefaultCtx) getDetectionPath() string {
	return c.app.toString(c.detectionPath)
}

func (c *DefaultCtx) getValues() *[maxParams]string {
	return &c.values
}

func (c *DefaultCtx) getMatched() bool {
	return c.isMatched
}

func (c *DefaultCtx) getSkipNonUseRoutes() bool {
	return c.shouldSkipNonUseRoutes
}

func (c *DefaultCtx) getFirstMatchIndex() int {
	return c.firstMatchIndex
}

func (c *DefaultCtx) setIndexHandler(handler int) {
	c.indexHandler = handler
}

func (c *DefaultCtx) setIndexRoute(route int) {
	c.indexRoute = route
}

func (c *DefaultCtx) setMatched(matched bool) {
	c.isMatched = matched
}

func (c *DefaultCtx) setSkipNonUseRoutes(skip bool) {
	c.shouldSkipNonUseRoutes = skip
}

func (c *DefaultCtx) setFirstMatchIndex(index int) {
	c.firstMatchIndex = index
}

func (c *DefaultCtx) setRoute(route *Route) {
	c.route = route
}

func (c *DefaultCtx) getPathOriginal() string {
	return c.pathOriginal
}
