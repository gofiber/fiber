package adaptor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"reflect"
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

// pooledCtx bundles the RequestCtx with its noopConn so one pool entry
// serves both and no per-request conn allocation is needed.
type pooledCtx struct {
	fctx fasthttp.RequestCtx
	conn noopConn
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

const bufferSize = 32 * 1024

var bufferPool = sync.Pool{
	New: func() any {
		return new([bufferSize]byte)
	},
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

// HTTPMiddleware wraps net/http middleware to fiber middleware
func HTTPMiddleware(mw func(http.Handler) http.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		var next bool
		nextHandler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			next = true
			c.Request().Header.SetMethod(r.Method)
			c.Request().SetRequestURI(r.RequestURI)
			c.Request().SetHost(r.Host)
			c.Request().Header.SetHost(r.Host)

			// Remove all cookies before setting, see https://github.com/valyala/fasthttp/pull/1864
			c.Request().Header.DelAllCookies()
			for key, vals := range r.Header {
				if len(vals) == 0 {
					continue
				}
				// Set replaces whatever the key held on the fiber request,
				// then Add appends the remaining values so multi-value
				// headers survive instead of collapsing to the last value.
				c.Request().Header.Set(key, vals[0])
				for _, v := range vals[1:] {
					c.Request().Header.Add(key, v)
				}
			}
			CopyContextToFiberContext(r.Context(), c.RequestCtx())
		})

		if err := HTTPHandler(mw(nextHandler))(c); err != nil {
			return err
		}

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
	if len(s) == 0 || len(s) > 5 {
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
				return &net.TCPAddr{IP: a[:], Port: port}, nil
			}
			if ip, ok := utils.ParseIPv6(host); ok {
				a := ip.As16()
				return &net.TCPAddr{IP: a[:], Port: port, Zone: ip.Zone()}, nil
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
		maxBodySize := int64(app.Config().BodyLimit)
		if r.Body != nil {
			if r.ContentLength > maxBodySize {
				http.Error(w, utils.StatusMessage(fiber.StatusRequestEntityTooLarge), fiber.StatusRequestEntityTooLarge)
				return
			}
			limit := maxBodySize
			if limit < math.MaxInt64 {
				limit++
			}
			limitedReader := io.LimitReader(r.Body, limit)
			n, err := io.Copy(req.BodyWriter(), limitedReader)
			if err != nil {
				http.Error(w, utils.StatusMessage(fiber.StatusInternalServerError), fiber.StatusInternalServerError)
				return
			}

			if n > maxBodySize {
				http.Error(w, utils.StatusMessage(fiber.StatusRequestEntityTooLarge), fiber.StatusRequestEntityTooLarge)
				return
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
				_ = app.Config().ErrorHandler(ctx, err) //nolint:errcheck // not needed
			}
		} else {
			// Execute fasthttp Ctx though app.Handler
			app.Handler()(fctx)
		}

		// Convert fasthttp Ctx -> net/http
		for k, v := range fctx.Response.Header.All() {
			w.Header().Add(string(k), string(v))
		}
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
