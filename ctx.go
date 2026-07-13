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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/gofiber/utils/v2/swar"
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

// Err mirrors context.Err, returning nil until cancellation and then the terminal error value.
//
// Due to current limitations in how fasthttp works, Err operates as a nop.
// See: https://github.com/valyala/fasthttp/issues/965#issuecomment-777268945
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
		// The detection path/tree hash changed; invalidate the lookahead index.
		c.firstMatchIndex = -1
	}
	return c.app.toString(c.path)
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

// FullPath returns the matched route path, including any group prefixes.
func (c *DefaultCtx) FullPath() string {
	return c.Route().Path
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

// HasBody returns true if the request declares a body via Content-Length, Transfer-Encoding, or already buffered payload data.
func (c *DefaultCtx) HasBody() bool {
	hdr := &c.fasthttp.Request.Header

	switch cl := hdr.ContentLength(); {
	case cl > 0:
		return true
	case cl == -1:
		// fasthttp reports -1 for Transfer-Encoding: chunked bodies.
		return true
	case cl == 0:
		if hasTransferEncodingBody(hdr) {
			return true
		}
	}

	return len(c.fasthttp.Request.Body()) > 0
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

func hasTransferEncodingBody(hdr *fasthttp.RequestHeader) bool {
	// Repeated field lines form one combined list (RFC 9110 Section 5.2),
	// so every Transfer-Encoding line must be inspected, not just the first.
	if lines := hdr.PeekAll(HeaderTransferEncoding); len(lines) > 0 {
		for _, line := range lines {
			if transferEncodingLineHasBody(utils.UnsafeString(line)) {
				return true
			}
		}
		return false
	}

	// Fallback scan for non-normalized header keys.
	for key, value := range hdr.All() {
		if !utils.EqualFold(utils.UnsafeString(key), HeaderTransferEncoding) {
			continue
		}
		if transferEncodingLineHasBody(utils.UnsafeString(value)) {
			return true
		}
	}

	return false
}

// transferEncodingLineHasBody reports whether a single Transfer-Encoding
// field line contains a transfer coding other than "identity".
func transferEncodingLineHasBody(te string) bool {
	for raw := range strings.SplitSeq(te, ",") {
		token := utils.TrimSpace(raw)
		if token == "" {
			continue
		}
		if idx := strings.IndexByte(token, ';'); idx >= 0 {
			token = utils.TrimSpace(token[:idx])
		}
		if token == "" {
			continue
		}
		if utils.EqualFold(token, "identity") {
			continue
		}
		return true
	}
	return false
}

// IsWebSocket returns true if the request includes a WebSocket upgrade handshake.
func (c *DefaultCtx) IsWebSocket() bool {
	// Repeated field lines are equivalent to one combined comma-separated
	// list (RFC 9110 Section 5.2), so inspect every Connection and Upgrade
	// field line, not just the first.
	if !headerListContainsToken(c.fasthttp.Request.Header.PeekAll(HeaderConnection), "upgrade") {
		return false
	}
	// Upgrade is a list of protocols, each optionally carrying a "/version"
	// suffix (RFC 9110 Section 7.8), e.g. "Upgrade: websocket, h2c".
	return headerListContainsToken(c.fasthttp.Request.Header.PeekAll(HeaderUpgrade), "websocket")
}

// headerListContainsToken reports whether any comma-separated element across
// the given field lines equals token case-insensitively. An optional
// "/version" suffix (Upgrade protocol syntax, RFC 9110 Section 7.8) is
// ignored when comparing; valid Connection members never contain "/", so
// this is safe for both headers.
func headerListContainsToken(lines [][]byte, token string) bool {
	for _, line := range lines {
		for v := range strings.SplitSeq(utils.UnsafeString(line), ",") {
			element := utils.TrimSpace(v)
			if i := strings.IndexByte(element, '/'); i >= 0 {
				element = element[:i]
			}
			if utils.EqualFold(element, token) {
				return true
			}
		}
	}
	return false
}

// IsPreflight returns true if the request is a CORS preflight.
func (c *DefaultCtx) IsPreflight() bool {
	if c.Method() != MethodOptions {
		return false
	}
	hdr := &c.fasthttp.Request.Header
	if len(hdr.Peek(HeaderAccessControlRequestMethod)) == 0 {
		return false
	}
	return len(hdr.Peek(HeaderOrigin)) > 0
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

// Secure returns whether a secure connection was established.
func (c *DefaultCtx) Secure() bool {
	return c.Scheme() == schemeHTTPS
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

// xmlHTTPRequestBytes is precomputed for XHR detection
var xmlHTTPRequestBytes = []byte("xmlhttprequest")

// XHR returns a Boolean property, that is true, if the request's X-Requested-With header field is XMLHttpRequest,
// indicating that the request was issued by a client library (such as jQuery).
func (c *DefaultCtx) XHR() bool {
	return utils.EqualFold(c.fasthttp.Request.Header.Peek(HeaderXRequestedWith), xmlHTTPRequestBytes)
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

// appendLowerASCII writes the ASCII-lowercased bytes of src into dst[:0],
// growing dst as needed, in a single pass over src (instead of a copy
// followed by an in-place case fold). Bytes outside 'A'..'Z', including
// non-ASCII, are copied unchanged. src and dst must not overlap.
func appendLowerASCII(dst, src []byte) []byte {
	n := len(src)
	if cap(dst) >= n {
		dst = dst[:n]
	} else {
		dst = make([]byte, n)
	}
	i := 0
	for ; i+swar.WordLen <= n; i += swar.WordLen {
		swar.Store8(dst, i, swar.ToLowerWord(swar.Load8(src, i)))
	}
	if i < n {
		if n >= swar.WordLen {
			// Finish with one overlapping word; the overlapped bytes are
			// rewritten with the same values.
			swar.Store8(dst, n-swar.WordLen, swar.ToLowerWord(swar.Load8(src, n-swar.WordLen)))
		} else {
			for ; i < n; i++ {
				c := src[i]
				if c-'A' <= 'Z'-'A' {
					c |= 0x20
				}
				dst[i] = c
			}
		}
	}
	return dst
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
