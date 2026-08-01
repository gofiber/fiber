package adaptor

import (
	"context"
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
	conn noopConn
	lr   io.LimitedReader
	fctx fasthttp.RequestCtx
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
		handler(c.RequestCtx())
		return nil
	}
}

// HTTPHandlerWithContext is like HTTPHandler, but additionally stores Fiber’s user context in the request context
func HTTPHandlerWithContext(h http.Handler) fiber.Handler {
	handler := fasthttpadaptor.NewFastHTTPHandler(h)
	return func(c fiber.Ctx) error {
		// Store the Fiber user context (c.Context()) in the fasthttp request context
		// so adapted net/http handlers can retrieve it via adaptor.LocalContextFromHTTPRequest(r)
		c.RequestCtx().SetUserValue(localContextKey, c.Context())

		handler(c.RequestCtx())
		return nil
	}
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
func ConvertRequest(c fiber.Ctx, forServer bool) (*http.Request, error) {
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
		case "Context":
			CopyContextToFiberContext(reflectValue.Interface(), requestContext)
		case "key":
			lastKey = reflectValue.Interface()
		case "val":
			if lastKey != nil {
				requestContext.SetUserValue(lastKey, reflectValue.Interface())
				lastKey = nil
			}
		default:
			continue
		}
	}
}

// framingHeaders describe how the message itself is delimited and addressed
// rather than what it carries. They are excluded from both the snapshot and the
// clear: the wrapped middleware works on a converted copy of the request, so a
// framing header missing from that copy says nothing about the original, and
// clearing one would corrupt the body the fiber handler goes on to read. Host
// is also restored explicitly by the caller from r.Host.
//
// The cost is that a wrapped middleware cannot change them either — a
// Header.Set("Connection", ...) is dropped along with a Header.Del. That is the
// deliberate trade: these headers frame the fiber request that is still being
// read, not the converted copy, and Transfer-Encoding could not round-trip in
// any case because fasthttpadaptor routes it to http.Request.TransferEncoding
// rather than to Header. Middleware that needs to influence framing should do
// so on the fiber side.
var framingHeaders = [...]string{
	fiber.HeaderHost,
	fiber.HeaderContentLength,
	fiber.HeaderTransferEncoding,
	fiber.HeaderConnection,
}

// headerPair is one owned copy of a header line taken from the converted
// net/http request.
type headerPair struct {
	key   string
	value string
}

// snapshotHeaders copies the non-framing headers of the converted request into
// owned strings.
//
// fasthttpadaptor builds http.Request.Header with b2s (fasthttpadaptor's
// request.go), so its keys and values are unsafe views into the very buffers
// the fiber request header owns. Writing back in place therefore corrupts the
// data still being read: a plain Set-then-Add copy duplicated the last value of
// every multi-valued header, and SetHost spliced whatever r.Header["Host"]
// pointed at. Materializing first removes the aliasing entirely.
//
// Framing and identity headers are dropped here rather than copied back: they
// describe the original message, not the converted copy, and Host is restored
// separately from r.Host.
func snapshotHeaders(h http.Header) []headerPair {
	// One entry per field line, not per name: sizing by len(h) regrows the
	// slice for every multi-valued header, which is exactly what the headers
	// worth snapshotting (X-Forwarded-For, Accept, Cookie) tend to be.
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
// the copy that follows rebuilds the set exactly as the wrapped net/http
// middleware left it.
//
// Copying with Set-then-Add cannot do that. fasthttp's Set replaces only the
// first entry with a given name, so a middleware that collapses a multi-valued
// header — `r.Header.Set("X-Forwarded-For", sanitized)` — left the remaining
// original values in place, and a middleware that changed nothing at all
// duplicated the last value of every multi-valued header. Removals were not
// propagated either, in any of net/http's spellings: Del, assigning nil, or
// filtering the slice to empty.
//
// Keys are collected before anything is deleted, since fasthttp's iterator
// walks the header storage the deletes rewrite.
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
		// Repeated names are appended repeatedly and Del'd repeatedly. Del
		// removes every field line at once, so the second call finds nothing —
		// but skipping it costs more than making it. A slices.Contains scan of
		// what has been collected so far was measurably slower on the common
		// header set, where no name repeats and every scan therefore fails:
		// 17% at twenty headers, 61% at a hundred. It bought back a few
		// allocations only when a name actually did repeat.
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
		var next bool
		nextHandler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			next = true

			freq := c.Request()
			fhdr := &freq.Header

			// Snapshot before mutating: fasthttpadaptor fills r.Header with
			// b2s views into fhdr's own storage, so every Set, Del or SetHost
			// below rewrites the bytes the remaining entries point at. The
			// method/URI/host writes are ordered after this for the same
			// reason. They happen to be safe today — each one copies a value
			// that aliases the very field it writes back to, and none of them
			// touch the key/value store the general headers live in — but that
			// is a property of fasthttp's current internals, not something this
			// package can rely on. Taking the snapshot first makes the code
			// hold the invariant its own comment states.
			pairs := snapshotHeaders(r.Header)

			fhdr.SetMethod(r.Method)
			freq.SetRequestURI(r.RequestURI)
			freq.SetHost(r.Host)
			fhdr.SetHost(r.Host)

			// Remove all cookies before setting, see https://github.com/valyala/fasthttp/pull/1864
			fhdr.DelAllCookies()
			clearCopiedHeaders(fhdr)
			for _, p := range pairs {
				fhdr.Add(p.key, p.value)
			}
			CopyContextToFiberContext(r.Context(), c.RequestCtx())
		})

		// Call the fasthttp adaptor directly: HTTPHandler would wrap it in a
		// second closure that has to be built on every request, and its
		// error result is always nil.
		fasthttpadaptor.NewFastHTTPHandler(mw(nextHandler))(c.RequestCtx())

		if next {
			return c.Next()
		}
		return nil
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
	if host, portStr, err := net.SplitHostPort(remoteAddr); err == nil {
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

		// Init2 mirrors fasthttp's RequestCtx.Init, but with a no-op
		// connection instead of fasthttp's fakeAddrer, whose Write panics.
		// Interim responses (e.g. SendEarlyHints' 103) are then silently
		// discarded instead of panicking; the final response still reaches
		// the client through the ResponseWriter copy-back below. Init2 only
		// touches connection metadata and buffer-retention flags, so the
		// request is built directly into fctx.Request afterwards — the same
		// order fasthttp's Init uses, minus its full request copy.
		fctx.Init2(&pctx.conn, disabledLogger, true)
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
		req.SetRequestURI(r.RequestURI)
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
