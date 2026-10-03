package adaptor

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/textproto"
	"reflect"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/headerlist"
	"github.com/gofiber/fiber/v3/internal/wiretarget"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// disableLogger implements the fasthttp Logger interface and discards log output.
type disableLogger struct{}

// Printf implements the fasthttp Logger interface and discards log output.
func (*disableLogger) Printf(string, ...any) {
}

// noopConn is the net.Conn handed to fasthttp's RequestCtx for adaptor-served
// requests. Unlike fasthttp's internal fakeAddrer (installed by
// RequestCtx.Init), whose Write panics, it silently discards writes so that
// interim responses written directly to the connection (e.g. a 103 from
// SendEarlyHints) degrade gracefully: the final response is still delivered
// through the http.ResponseWriter copy-back.
type noopConn struct {
	remoteAddr net.Addr
}

// tlsNoopConn is the connection handed to fasthttp for a request net/http
// served over TLS; fasthttp detects TLS by a ConnectionState method.
type tlsNoopConn struct {
	noopConn
	state tls.ConnectionState
}

func (c *tlsNoopConn) ConnectionState() tls.ConnectionState {
	return c.state
}

// Handshake is a no-op; net/http already performed the real one.
func (*tlsNoopConn) Handshake() error {
	return nil
}

func (*noopConn) Read([]byte) (int, error)    { return 0, io.EOF }
func (*noopConn) Write(p []byte) (int, error) { return len(p), nil }
func (*noopConn) Close() error                { return nil }
func (*noopConn) LocalAddr() net.Addr         { return &net.TCPAddr{} }

func (c *noopConn) RemoteAddr() net.Addr {
	if c.remoteAddr == nil {
		return &net.TCPAddr{}
	}
	return c.remoteAddr
}

func (*noopConn) SetDeadline(time.Time) error      { return nil }
func (*noopConn) SetReadDeadline(time.Time) error  { return nil }
func (*noopConn) SetWriteDeadline(time.Time) error { return nil }

// tcpAddrBuf co-locates a net.TCPAddr with the storage for its IP so the
// common "ip:port" remote address form costs one allocation instead of two
// (one for the address, one for the IP slice).
//
// The buffer is deliberately not pooled: RemoteAddr()/RemoteIP() hand this
// value to user code, which may legitimately keep it past the handler — an
// async access log, a connection registry — so every request gets its own.
type tcpAddrBuf struct {
	addr net.TCPAddr
	ip   [16]byte
}

// newTCPAddr returns a TCPAddr backed by inline IP storage. The IP slice is
// capped at its length so appending to it allocates instead of writing into
// the rest of the buffer.
func newTCPAddr(ip []byte, port int, zone string) *net.TCPAddr {
	buf := &tcpAddrBuf{}
	n := copy(buf.ip[:], ip)
	buf.addr.IP = buf.ip[:n:n]
	buf.addr.Port = port
	buf.addr.Zone = zone
	return &buf.addr
}

// pooledCtx bundles the RequestCtx with the per-request scratch space it
// needs — the noopConn handed to fasthttp and the io.LimitedReader used to
// bound the request body — so a single pool entry serves them all and the
// request path stays allocation-free. The conn comes first to keep the
// struct's trailing pointer span minimal (govet fieldalignment).
type pooledCtx struct {
	conn    noopConn
	tlsConn tlsNoopConn
	lr      io.LimitedReader
	fctx    fasthttp.RequestCtx
}

var ctxPool = sync.Pool{
	New: func() any {
		return new(pooledCtx)
	},
}

// disabledLogger is shared: it is stateless, and reusing one value keeps
// the logger interface conversion off the per-request path.
var disabledLogger = &disableLogger{}

// LocalContextKey is the key used to store the user's context.Context in the fasthttp request context.
// Adapted http.Handler functions can retrieve this context using r.Context().Value(adaptor.LocalContextKey)
var localContextKey = &struct{}{}

const (
	bufferSize = 32 * 1024

	// maxHeaderScratch bounds the response-header staging buffers that are
	// returned to the pool.
	maxHeaderScratch = 16 * 1024
)

var bufferPool = sync.Pool{
	New: func() any {
		return new([bufferSize]byte)
	},
}

// headerScratchPool holds the staging buffers used to pack response header
// keys and values into a single string per response.
var headerScratchPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 512)
		return &b
	},
}

// copyBody streams src into dst using a pooled buffer. io.Copy would
// allocate a fresh 32 KiB buffer on every call because neither an
// io.LimitedReader nor fasthttp's body writer implements the
// WriterTo/ReaderFrom shortcuts it looks for.
func copyBody(dst io.Writer, src io.Reader) (int64, error) {
	bufPtr, ok := bufferPool.Get().(*[bufferSize]byte)
	if !ok {
		panic(fmt.Errorf("failed to type-assert to *[%d]byte", bufferSize))
	}
	defer bufferPool.Put(bufPtr)

	n, err := io.CopyBuffer(dst, src, bufPtr[:])
	return n, err //nolint:wrapcheck // the caller maps this onto a status code
}

// copyResponseHeaders copies the fasthttp response headers onto the net/http
// header map.
//
// A plain dst.Add(string(k), string(v)) loop allocates a string per key, a
// string per value and a one-element slice per header. Instead every key and
// value is packed into a single string that the map entries re-slice, and the
// single-valued entries share one []string backing array, so the whole
// copy-back costs two allocations no matter how many headers the response
// carries.
func copyResponseHeaders(dst http.Header, src *fasthttp.ResponseHeader) {
	bufPtr, ok := headerScratchPool.Get().(*[]byte)
	if !ok {
		panic(errors.New("failed to type-assert to *[]byte"))
	}

	buf := (*bufPtr)[:0]
	count := 0
	for k, v := range src.All() {
		buf = append(buf, k...)
		buf = append(buf, v...)
		count++
	}

	var packed string
	if len(buf) > 0 {
		packed = string(buf)
	}

	// Keep oversized scratch out of the pool so one huge response does not
	// pin a large buffer for the lifetime of the process.
	if cap(buf) <= maxHeaderScratch {
		*bufPtr = buf
		headerScratchPool.Put(bufPtr)
	}

	if count == 0 {
		return
	}

	// Backing array for the single-valued entries, allocated on first use so
	// a response whose headers all merge into existing entries pays nothing.
	// Each entry gets a full slice expression so a later Add on it grows into
	// a fresh array instead of overwriting the neighboring entry.
	var vals []string

	off := 0
	for k, v := range src.All() {
		// Defensive: both passes always see the same headers, but never
		// slice out of range if that assumption is ever broken.
		if off+len(k)+len(v) > len(packed) {
			dst.Add(string(k), string(v))
			continue
		}

		key := textproto.CanonicalMIMEHeaderKey(packed[off : off+len(k)])
		off += len(k)
		val := packed[off : off+len(v)]
		off += len(v)

		if existing, found := dst[key]; found {
			dst[key] = append(existing, val)
			continue
		}

		if vals == nil {
			vals = make([]string, 0, count)
		} else if len(vals) == cap(vals) {
			dst[key] = []string{val}
			continue
		}

		i := len(vals)
		vals = append(vals, val)
		dst[key] = vals[i : i+1 : i+1]
	}
}

var (
	ErrRemoteAddrEmpty   = errors.New("remote address cannot be empty")
	ErrRemoteAddrTooLong = errors.New("remote address too long")
)

// HTTPHandlerFunc wraps net/http handler func to fiber handler
func HTTPHandlerFunc(h http.HandlerFunc) fiber.Handler {
	return HTTPHandler(h)
}

// HTTPHandler wraps net/http handler to fiber handler
func HTTPHandler(h http.Handler) fiber.Handler {
	handler := fasthttpadaptor.NewFastHTTPHandler(h)
	return func(c fiber.Ctx) error {
		return serveRouted(c, handler)
	}
}

// HTTPHandlerWithContext is like HTTPHandler, but additionally stores Fiber’s user context in the request context
func HTTPHandlerWithContext(h http.Handler) fiber.Handler {
	handler := fasthttpadaptor.NewFastHTTPHandler(h)
	return func(c fiber.Ctx) error {
		// Store the Fiber user context (c.Context()) in the fasthttp request context
		// so adapted net/http handlers can retrieve it via adaptor.LocalContextFromHTTPRequest(r)
		c.RequestCtx().SetUserValue(localContextKey, c.Context())

		return serveRouted(c, handler)
	}
}

// serveRouted runs handler, a net/http handler adapted by fasthttpadaptor, on
// a request line that spells the path the router matched (wiretarget.Routed)
// rather than the one that arrived, and puts the original back afterwards.
// The two differ for an escaped or non-canonical request, and net/http reads
// the raw one its own way: "/public/..%2Fadmin" decodes into a URL.Path that
// http.FileServer cleans to "/admin", although no middleware mounted on
// "/admin" has run. A routed path whose segments would not survive that
// reading, one with an empty segment or an escaped slash, is not handed on.
func serveRouted(c fiber.Ctx, handler fasthttp.RequestHandler) error {
	target := wiretarget.Routed(c)
	if !wiretarget.SegmentsAsRouted(target) {
		return fiber.ErrNotFound
	}
	if target == c.OriginalURL() {
		// the request line already spells the routed path
		handler(c.RequestCtx())
		return nil
	}
	req := c.Request()
	original := utils.CopyString(c.OriginalURL())
	setRoutedTarget(c, target)
	// Buffered, the handler has returned, or panicked, in which case
	// fasthttpadaptor re-panics here and the recover middleware or error
	// handler that catches it still has to see the original line. Streaming
	// or hijacked, the handler is still running with an http.Request whose
	// URL aliases the request line's storage, so the line is left as the
	// handler read it.
	defer func() {
		if !c.Response().IsBodyStream() && !c.RequestCtx().Hijacked() {
			req.SetRequestURI(original)
		}
	}()
	handler(c.RequestCtx())
	return nil
}

// setRoutedTarget sets target, the routed request target, as c's request line.
// A target that is the line as it arrived is left alone, which spares a second
// parse of it. Otherwise the host fasthttp parsed is kept: a target that is a
// path makes fasthttp take the host from the Host header again, while an
// absolute-form request line names its own, which RFC 9112 Section 3.2.2 says
// the server uses and which c.Hostname() reported. Kept, the adapted handler
// sees in r.Host the host Fiber saw.
func setRoutedTarget(c fiber.Ctx, target string) {
	if target == c.OriginalURL() {
		return
	}
	req := c.Request()
	var buf [64]byte
	host := append(buf[:0], req.URI().Host()...)
	req.SetRequestURI(target)
	req.URI().SetHostBytes(host)
}

// LocalContextFromHTTPRequest extracts the Fiber user context previously stored into r.Context() by the adaptor.
func LocalContextFromHTTPRequest(r *http.Request) (context.Context, bool) {
	if r == nil {
		return nil, false
	}

	ctx, err := r.Context().Value(localContextKey).(context.Context)
	return ctx, err
}

// ConvertRequest converts a fiber.Ctx to a http.Request.
// forServer should be set to true when the http.Request is going to be passed to a http.Handler.
//
// The request is built from the path the router matched (see
// wiretarget.Routed), which the request line is set to; the returned request's
// URL and RequestURI alias that line's storage, as they do in fasthttpadaptor,
// so it is left in place. A routed path fasthttp would read as an authority
// rather than a path (see wiretarget.ParsesAsPath) is answered with
// fiber.ErrBadRequest instead of being set, since the request's host would
// then be read out of it; so is one holding a forged escape (see
// wiretarget.HasForgedEscape), which URL.Path would decode into a segment the
// router never matched.
func ConvertRequest(c fiber.Ctx, forServer bool) (*http.Request, error) {
	target := wiretarget.Routed(c)
	if !wiretarget.ParsesAsPath(target, c.Request().Header.Host()) || wiretarget.HasForgedEscape(target) {
		return nil, fiber.ErrBadRequest
	}
	setRoutedTarget(c, target)
	var req http.Request
	if err := fasthttpadaptor.ConvertRequest(c.RequestCtx(), &req, forServer); err != nil {
		return nil, err //nolint:wrapcheck // This must not be wrapped
	}
	return &req, nil
}

// CopyContextToFiberContext copies the values of context.Context to a fasthttp.RequestCtx.
// This function safely handles struct fields, using unsafe operations only when necessary for unexported fields.
//
// Deprecated: This function uses reflection and unsafe pointers; consider using explicit context passing.
func CopyContextToFiberContext(src any, requestContext *fasthttp.RequestCtx) {
	copyContextValues(src, requestContext, make(map[visitedContext]struct{}))
}

// visitedContext identifies a context already walked. The address alone would
// not: an embedded first field shares the address of the struct holding it, and
// context's own types nest that way.
type visitedContext struct {
	typ reflect.Type
	ptr uintptr
}

// copyContextValues walks the chain to its end, stopping only where it would
// repeat itself. A chain that points back at a context already walked is a
// cycle; depth cannot tell one from a legitimately deep middleware stack, whose
// outermost values a bound would silently drop. Every cycle runs through a
// pointer — a struct cannot contain itself by value — so remembering those is
// enough.
func copyContextValues(src any, requestContext *fasthttp.RequestCtx, seen map[visitedContext]struct{}) {
	if requestContext == nil {
		return
	}

	v := reflect.ValueOf(src)
	if !v.IsValid() {
		return
	}
	// Deref pointer chains
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		visited := visitedContext{typ: v.Type(), ptr: v.Pointer()}
		if _, walked := seen[visited]; walked {
			return
		}
		seen[visited] = struct{}{}
		v = v.Elem()
	}
	t := v.Type()
	if t.Kind() != reflect.Struct {
		return
	}
	// Ensure addressable for safe unsafe-access of unexported fields
	if !v.CanAddr() {
		tmp := reflect.New(t)
		tmp.Elem().Set(v)
		v = tmp.Elem()
	}
	contextValues := v
	contextKeys := t

	var lastKey any
	for i := 0; i < contextValues.NumField(); i++ {
		reflectValue := contextValues.Field(i)
		reflectField := contextKeys.Field(i)

		if reflectField.Name == "noCopy" {
			break
		}

		// Avoid unsafe access for unexported fields; use safe reflection where possible
		if !reflectValue.CanInterface() {
			/* #nosec */
			reflectValue = reflect.NewAt(reflectValue.Type(), unsafe.Pointer(reflectValue.UnsafeAddr())).Elem()
		}

		switch reflectField.Name {
		case "key":
			lastKey = reflectValue.Interface()
			continue
		case "val":
			if lastKey != nil {
				requestContext.SetUserValue(lastKey, reflectValue.Interface())
				lastKey = nil
			}
			continue
		}

		// Any other context-typed field carries the parent chain; parents are
		// copied first, so a child's value for the same key wins.
		if parent, ok := contextOf(reflectValue); ok {
			copyContextValues(parent, requestContext, seen)
		}
	}
}

// contextOf returns the context a struct field holds, as an interface, a pointer or an embedded struct.
func contextOf(v reflect.Value) (context.Context, bool) {
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return nil, false
		}
		if ctx, ok := v.Interface().(context.Context); ok {
			return ctx, true
		}
	case reflect.Struct:
		// A pointer's method set contains the value's, so the addressable form
		// answers for both receiver kinds; a value this walk cannot address
		// still answers for value receivers.
		val := v
		if v.CanAddr() {
			val = v.Addr()
		}
		if ctx, ok := val.Interface().(context.Context); ok {
			return ctx, true
		}
	default:
	}
	return nil, false
}

// framingHeaders delimit and address the message rather than describe what it
// carries, and are excluded from the snapshot and the clear: the wrapped
// middleware sees a copy, so clearing one corrupts the body still being read.
//
// Connection is not among them. It frames the hop rather than the body, and
// naming which fields are hop-by-hop is something middleware rewrites — dropping
// that edit left the handler reading the client's own list. fasthttp re-derives
// its close flag when the line is written back, so the round trip keeps it.
var framingHeaders = [...]string{
	fiber.HeaderHost,
	fiber.HeaderContentLength,
	fiber.HeaderTransferEncoding,
}

// headerPair is one owned copy of a header line taken from the converted
// net/http request.
type headerPair struct {
	key   string
	value string
}

// snapshotHeaders copies the non-framing headers of the converted request into
// owned strings. fasthttpadaptor builds r.Header with b2s, so its keys and values
// alias buffers the fiber header owns — writing back in place corrupted them.
func snapshotHeaders(h http.Header) []headerPair {
	// One entry per field line, not per name: len(h) regrows the slice for
	// every multi-valued header, which is what these tend to be.
	lines := 0
	for _, vals := range h {
		lines += len(vals)
	}

	pairs := make([]headerPair, 0, lines)
	for key, vals := range h {
		if isFramingHeader(key) {
			continue
		}
		ownedKey := strings.Clone(key)
		for _, v := range vals {
			pairs = append(pairs, headerPair{key: ownedKey, value: strings.Clone(v)})
		}
	}
	return pairs
}

// clearCopiedHeaders deletes every non-framing header from the fiber request so
// the copy that follows rebuilds the set as the wrapped middleware left it.
// Set-then-Add cannot: Set replaces only the first entry, and removals never did.
func clearCopiedHeaders(fhdr *fasthttp.RequestHeader) {
	// Not sized from fhdr.Len(): that counts by walking every header, so
	// pre-sizing would traverse them twice — and the walk also collects the
	// cookie store and re-serializes it, on a per-request path.
	var removed []string
	for key := range fhdr.All() {
		name := utils.UnsafeString(key)
		if isFramingHeader(name) {
			continue
		}
		// Repeated names are Del'd repeatedly; Del removes every line at once, so
		// the second finds nothing. Skipping it costs more — a slices.Contains scan
		// measured 17% slower at twenty headers and 61% at a hundred.
		removed = append(removed, string(key))
	}

	for _, name := range removed {
		fhdr.Del(name)
	}
}

func isFramingHeader(name string) bool {
	for _, h := range framingHeaders {
		if utils.EqualFold(name, h) {
			return true
		}
	}
	return false
}

// HTTPMiddleware wraps net/http middleware to fiber middleware
func HTTPMiddleware(mw func(http.Handler) http.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		var (
			mu              sync.Mutex
			next            bool
			connectionClose bool
			returned        bool
		)
		nextHandler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			if returned {
				// The middleware flushed or hijacked before calling next, so the
				// Fiber context is no longer ours.
				return
			}
			next = true

			freq := c.Request()
			fhdr := &freq.Header

			// Snapshot before mutating: fasthttpadaptor fills r.Header with b2s views
			// into fhdr's storage, so every Set, Del or SetHost below rewrites bytes the
			// remaining entries point at. The method/URI/host writes follow for that.
			pairs := snapshotHeaders(r.Header)

			fhdr.SetMethod(r.Method)
			// r.Host is a view of the URI's host, which SetRequestURI makes
			// fasthttp parse again, from the Host header for a target that is
			// a path: copy it first, or the host written back below is the
			// header's rather than the one r carried.
			var hostBuf [64]byte
			host := append(hostBuf[:0], r.Host...)
			// A rewrite of r.URL (http.StripPrefix) leaves RequestURI untouched; route the URL.
			requestURI := r.RequestURI
			newPath := ""
			if r.URL != nil {
				if rewritten := r.URL.RequestURI(); rewritten != "" && rewritten != requestURI {
					requestURI = rewritten
					// Clone now: r.URL aliases the storage SetRequestURI overwrites.
					newPath = strings.Clone(r.URL.EscapedPath())
				}
			}
			freq.SetRequestURI(requestURI)
			freq.URI().SetHostBytes(host)
			fhdr.SetHostBytes(host)
			// Only a genuine rewrite re-routes. The decoded path differs from the raw
			// one for every escaped or non-canonical request, and overriding it there
			// re-buckets the tree, resuming the chain past middleware that already ran.
			if newPath != "" && newPath != c.Path() {
				c.Path(newPath)
			}

			// Remove all cookies before setting, see https://github.com/valyala/fasthttp/pull/1864
			fhdr.DelAllCookies()
			clearCopiedHeaders(fhdr)
			// Connection lives in a slot of its own, so a second Add replaces the
			// first rather than appending: net/http represents a combined value as
			// several entries, and ["keep-alive", "X-Internal"] arrived as
			// "X-Internal" alone — dropping the token that marks a field hop-by-hop,
			// which a proxy downstream would then forward. ["close", …] lost the
			// close signal the same way. A recipient may combine field lines with
			// commas (RFC 9110 §5.3), so they are joined and set once.
			var connection []string
			for _, p := range pairs {
				if utils.EqualFold(p.key, fiber.HeaderConnection) {
					connection = append(connection, p.value)
					continue
				}
				fhdr.Add(p.key, p.value)
			}
			if len(connection) > 0 {
				joined := strings.Join(connection, ", ")
				fhdr.Set(fiber.HeaderConnection, joined)
				// RFC 9110 Section 7.6.1 matches connection options
				// case-insensitively.
				if headerlist.ContainsFold(joined, "close") {
					// The close instruction is carried separately rather than
					// written back here: fasthttp's request flag makes Peek answer
					// "close" and hides the rest of the list (RFC 9110 §7.6.1),
					// which is how a proxy downstream learns what to strip.
					connectionClose = true
				}
			}
			CopyContextToFiberContext(r.Context(), c.RequestCtx())
		})

		// The middleware reads the path the router matched (see
		// wiretarget.Routed), not the request line as it arrived: a guard on
		// r.URL.Path has to see the "/admin/x" that "/a/../admin/x" was routed
		// as. Escaped slashes are handed on, in r.URL.RawPath, so a Fiber
		// route behind the middleware still receives its "%2F". A path
		// fasthttp would read as an authority rather than a path (see
		// wiretarget.ParsesAsPath) is refused: set as the request line, it
		// would become the host of both the net/http request and the Fiber
		// request behind the middleware. So is a path holding a forged
		// escape (see wiretarget.HasForgedEscape), which URL.Path would
		// decode into a segment the router never matched.
		target := wiretarget.Routed(c)
		if !wiretarget.ParsesAsPath(target, c.Request().Header.Host()) || wiretarget.HasForgedEscape(target) {
			return fiber.ErrBadRequest
		}
		setRoutedTarget(c, target)
		// Call the fasthttp adaptor directly: HTTPHandler would wrap it in a
		// second closure that has to be built on every request, and its
		// error result is always nil.
		fasthttpadaptor.NewFastHTTPHandler(mw(nextHandler))(c.RequestCtx())

		mu.Lock()
		returned = true
		ranNext, closeConnection := next, connectionClose
		mu.Unlock()

		var err error
		if ranNext {
			err = c.Next()
		}

		if closeConnection {
			// The close instruction rides on the response so the request keeps the
			// complete field for every observer — downstream handlers, middleware
			// resuming after Next, and the app's ErrorHandler alike. It also has to
			// go on the response to have any effect: fasthttp stores the request
			// flag before calling the handler and never reads it again, whereas the
			// response flag is what the server consults once the handler returns.
			//
			// Applied on the way out, because a single flag stands for the whole
			// response and the downstream chain can clear it: a handler resetting
			// the response, or setting a Connection value of its own, left the
			// connection reused despite the rewrite that asked for it to close.
			c.Response().Header.SetConnectionClose()
		}
		return err
	}
}

// FiberHandler wraps fiber handler to net/http handler
func FiberHandler(h fiber.Handler) http.Handler {
	return FiberHandlerFunc(h)
}

// FiberHandlerFunc wraps fiber handler to net/http handler func
func FiberHandlerFunc(h fiber.Handler) http.HandlerFunc {
	return handlerFunc(fiber.New(), h)
}

// FiberApp wraps fiber app to net/http handler func
func FiberApp(app *fiber.App) http.HandlerFunc {
	return handlerFunc(app)
}

func isUnixNetwork(network string) bool {
	return network == "unix" || network == "unixgram" || network == "unixpacket"
}

// parseDecimalPort parses a purely numeric port string. Anything else —
// signs, service names like "http", overflow — reports ok=false so the
// caller falls back to net.ResolveTCPAddr, which handles the long tail.
func parseDecimalPort(s string) (int, bool) {
	if s == "" || len(s) > 5 {
		return 0, false
	}
	port := 0
	for i := 0; i < len(s); i++ {
		d := s[i] - '0'
		if d > 9 {
			return 0, false
		}
		port = port*10 + int(d)
	}
	return port, port <= 65535
}

func resolveRemoteAddr(remoteAddr string, localAddr any) (net.Addr, error) {
	if addr, ok := localAddr.(net.Addr); ok && isUnixNetwork(addr.Network()) {
		return addr, nil
	}

	// Validate input to prevent malformed addresses
	if remoteAddr == "" {
		return nil, ErrRemoteAddrEmpty
	}

	// Fast path: "ip:port" literals — the only form net/http servers set on
	// Request.RemoteAddr — build the TCPAddr directly instead of going
	// through net.ResolveTCPAddr's resolver machinery. Hostnames, service
	// port names, and anything else fall through to the resolver below.
	if host, portStr, hasPort := utils.SplitHostPort(remoteAddr); hasPort {
		if port, ok := parseDecimalPort(portStr); ok {
			if ip, ok := utils.ParseIPv4(host); ok {
				a := ip.As4()
				return newTCPAddr(a[:], port, ""), nil
			}
			if ip, ok := utils.ParseIPv6(host); ok {
				a := ip.As16()
				return newTCPAddr(a[:], port, ip.Zone()), nil
			}
		}
	}

	resolved, err := net.ResolveTCPAddr("tcp", remoteAddr)
	if err == nil {
		return resolved, nil
	}

	var addrErr *net.AddrError
	if errors.As(err, &addrErr) && addrErr != nil && addrErr.Err == "missing port in address" {
		if len(remoteAddr) > 253 { // Max hostname length
			return nil, ErrRemoteAddrTooLong
		}
		remoteAddr = net.JoinHostPort(remoteAddr, "80")
		resolved, err2 := net.ResolveTCPAddr("tcp", remoteAddr)
		if err2 != nil {
			return nil, fmt.Errorf("failed to resolve TCP address after adding port: %w", err2)
		}
		return resolved, nil
	}
	return nil, fmt.Errorf("failed to resolve TCP address: %w", err)
}

// requestTarget is the request line net/http writes for r, as Request.write
// spells it: URL.RequestURI(), except for a CONNECT request whose URL has no
// path. net/http reads and writes such a request in the authority form,
// keeping the target in URL.Host, and URL.RequestURI() reads "/" for it; the
// line is then the authority, from URL.Opaque or the host, so a Fiber CONNECT
// route sees the host the request names rather than "/".
func requestTarget(r *http.Request) string {
	if r.URL == nil {
		return ""
	}
	if r.Method != http.MethodConnect || r.URL.Path != "" {
		return r.URL.RequestURI()
	}
	if r.URL.Opaque != "" {
		return r.URL.Opaque
	}
	if r.Host != "" {
		return r.Host
	}
	return r.URL.Host
}

// routedRequestURI returns the request line Fiber routes for r. It is
// r.RequestURI as net/http received it, so Fiber routes the bytes the client
// sent, a raw "/über" or "/a|b" included, as it would served directly. Only
// when r.URL no longer says what that line says, because a handler in front
// rewrote it as http.StripPrefix does, or when there is no such line, as for a
// request built in code, is it the line net/http would write for r.URL (see
// requestTarget). The comparison decodes the received path, since r.URL holds
// it decoded and re-escapes it its own way.
func routedRequestURI(r *http.Request) string {
	if r.URL == nil {
		return r.RequestURI
	}
	if raw := r.RequestURI; raw != "" && raw[0] == '/' {
		rawPath, rawQuery, _ := strings.Cut(raw, "?")
		if r.URL.RawQuery == rawQuery && decodesTo(rawPath, r.URL.Path) {
			return raw
		}
		return requestTarget(r)
	}
	if fromURL := requestTarget(r); fromURL != "" && fromURL != r.RequestURI {
		return fromURL
	}
	return r.RequestURI
}

// decodesTo reports whether raw, a path whose escapes are well formed as
// net/http requires of a request line, percent-decodes to decoded.
func decodesTo(raw, decoded string) bool {
	j := 0
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == '%' && i+2 < len(raw) && isHex(raw[i+1]) && isHex(raw[i+2]) {
			c = fromHex(raw[i+1])<<4 | fromHex(raw[i+2])
			i += 2
		}
		if j >= len(decoded) || decoded[j] != c {
			return false
		}
		j++
	}
	return j == len(decoded)
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// fromHex returns the value of the hex digit c.
func fromHex(c byte) byte {
	switch {
	case c >= 'a':
		return c - 'a' + 10
	case c >= 'A':
		return c - 'A' + 10
	default:
		return c - '0'
	}
}

func handlerFunc(app *fiber.App, h ...fiber.Handler) http.HandlerFunc {
	// App.Config returns the config by value, so read the body limit once at
	// construction instead of copying the whole 624-byte struct on every
	// request. Fiber only writes app.config in New. The error handler is
	// deliberately not cached: App.ErrorHandler resolves a mounted sub-app's
	// handler from the request path, and that lookup belongs per request.
	maxBodySize := int64(app.Config().BodyLimit)

	return func(w http.ResponseWriter, r *http.Request) {
		// New fasthttp Ctx from pool
		pctx := ctxPool.Get().(*pooledCtx) //nolint:forcetypeassert,errcheck // not needed
		fctx := &pctx.fctx
		fctx.Response.Reset()
		fctx.Request.Reset()
		defer ctxPool.Put(pctx)

		remoteAddr, err := resolveRemoteAddr(r.RemoteAddr, r.Context().Value(http.LocalAddrContextKey))
		if err != nil {
			remoteAddr = nil // Fallback to nil
		}
		pctx.conn.remoteAddr = remoteAddr
		var conn net.Conn = &pctx.conn
		if r.TLS != nil {
			// Carry the TLS state over, so c.Scheme() and c.Secure() describe the connection.
			pctx.tlsConn.noopConn = pctx.conn
			pctx.tlsConn.state = *r.TLS
			conn = &pctx.tlsConn
		}

		// Init2 mirrors fasthttp's RequestCtx.Init, but with a no-op
		// connection instead of fasthttp's fakeAddrer, whose Write panics.
		// Interim responses (e.g. SendEarlyHints' 103) are then silently
		// discarded instead of panicking; the final response still reaches
		// the client through the ResponseWriter copy-back below. Init2 only
		// touches connection metadata and buffer-retention flags, so the
		// request is built directly into fctx.Request afterwards — the same
		// order fasthttp's Init uses, minus its full request copy.
		fctx.Init2(conn, disabledLogger, true)
		req := &fctx.Request

		// Convert net/http -> fasthttp request with size limit
		if r.Body != nil {
			if r.ContentLength > maxBodySize {
				http.Error(w, utils.StatusMessage(fiber.StatusRequestEntityTooLarge), fiber.StatusRequestEntityTooLarge)
				return
			}

			var n int64
			// http.NoBody never yields any bytes, so skip the copy machinery
			// entirely for the (very common) bodyless request.
			if r.Body != http.NoBody {
				limit := maxBodySize
				if limit < math.MaxInt64 {
					limit++
				}
				// The LimitedReader lives in the pooled ctx and the copy
				// buffer comes from the shared pool: io.Copy would otherwise
				// allocate a fresh 32 KiB buffer on every single request.
				pctx.lr.R = r.Body
				pctx.lr.N = limit

				var err error
				n, err = copyBody(req.BodyWriter(), &pctx.lr)
				pctx.lr.R = nil // don't keep the request body alive in the pool
				if err != nil {
					http.Error(w, utils.StatusMessage(fiber.StatusInternalServerError), fiber.StatusInternalServerError)
					return
				}

				if n > maxBodySize {
					http.Error(w, utils.StatusMessage(fiber.StatusRequestEntityTooLarge), fiber.StatusRequestEntityTooLarge)
					return
				}
			}

			req.Header.SetContentLength(int(n))
		}
		req.Header.SetMethod(r.Method)
		req.SetRequestURI(routedRequestURI(r))
		req.SetHost(r.Host)
		req.Header.SetHost(r.Host)
		// Propagate the real protocol version so protocol-dependent behavior
		// (e.g. skipping interim 1xx responses for non-HTTP/1.1 requests,
		// RFC 9110 Section 15.2) sees the truth instead of fasthttp's
		// default HTTP/1.1. net/http reports "HTTP/2.0"/"HTTP/3.0", while
		// Fiber's Protocol() convention is "HTTP/2"/"HTTP/3" — key on
		// ProtoMajor so variant protocol strings normalize too, and fall
		// back to HTTP/1.1 for hand-built requests with an empty Proto.
		proto := r.Proto
		switch {
		case r.ProtoMajor == 2:
			proto = "HTTP/2"
		case r.ProtoMajor == 3:
			proto = "HTTP/3"
		case proto == "":
			proto = "HTTP/1.1"
		}
		req.Header.SetProtocol(proto)

		for key, vals := range r.Header {
			if len(vals) == 0 {
				continue
			}
			// Set replaces any value fasthttp derived while building the
			// request, then Add appends the remaining values so multi-value
			// headers (e.g. repeated X-Forwarded-For lines) survive instead
			// of collapsing to the last value. fasthttp's Add keeps its own
			// singleton semantics for Cookie/Content-Type/etc., which can
			// only hold one value there by design.
			req.Header.Set(key, vals[0])
			for _, v := range vals[1:] {
				req.Header.Add(key, v)
			}
		}

		if len(h) > 0 {
			// New fiber Ctx
			ctx := app.AcquireCtx(fctx)
			defer app.ReleaseCtx(ctx)

			// Execute fiber Ctx
			err := h[0](ctx)
			if err != nil {
				_ = app.ErrorHandler(ctx, err) //nolint:errcheck // not needed
			}
		} else {
			// Execute fasthttp Ctx though app.Handler
			app.Handler()(fctx)
		}

		// Convert fasthttp Ctx -> net/http
		copyResponseHeaders(w.Header(), &fctx.Response.Header)
		w.WriteHeader(fctx.Response.StatusCode())

		// Check if streaming is not possible or unnecessary.
		bodyStream := fctx.Response.BodyStream()
		flusher, ok := w.(http.Flusher)
		if !ok || bodyStream == nil {
			_, _ = w.Write(fctx.Response.Body()) //nolint:errcheck // not needed
			return
		}

		defer func() {
			_ = fctx.Response.CloseBodyStream() //nolint:errcheck // not needed
		}()

		// Stream fctx.Response.BodyStream() -> w
		// in chunks.
		bufPtr, ok := bufferPool.Get().(*[bufferSize]byte)
		if !ok {
			panic(fmt.Errorf("failed to type-assert to *[%d]byte", bufferSize))
		}
		defer bufferPool.Put(bufPtr)

		buf := bufPtr[:]
		for {
			n, err := bodyStream.Read(buf)
			if n > 0 {
				if _, writeErr := w.Write(buf[:n]); writeErr != nil {
					break
				}
				flusher.Flush()
			}

			if err != nil {
				break
			}
		}
	}
}
