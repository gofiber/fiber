package adaptor

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"sync"
	"unsafe"

	"github.com/gofiber/fiber/v3"
	utils "github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// disableLogger implements the fasthttp Logger interface and discards log output.
type disableLogger struct{}

// Printf implements the fasthttp Logger interface and discards log output.
func (*disableLogger) Printf(string, ...any) {
}

var ctxPool = sync.Pool{
	New: func() any {
		return new(fasthttp.RequestCtx)
	},
}

const bufferSize = 32 * 1024

var bufferPool = sync.Pool{
	New: func() any {
		return new([bufferSize]byte)
	},
}

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
// Deprecated: This function uses reflection and unsafe pointers; consider using explicit context passing.
func CopyContextToFiberContext(src any, requestContext *fasthttp.RequestCtx) {
	v := reflect.ValueOf(src)
	if !v.IsValid() {
		return
	}
	// Deref pointer chains
	for v.Kind() == reflect.Ptr {
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
			for key, val := range r.Header {
				for _, v := range val {
					c.Request().Header.Set(key, v)
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

func resolveRemoteAddr(remoteAddr string, localAddr any) (net.Addr, error) {
	if addr, ok := localAddr.(net.Addr); ok && isUnixNetwork(addr.Network()) {
		return addr, nil
	}

	// Validate input to prevent malformed addresses
	if remoteAddr == "" {
		return nil, errors.New("remote address cannot be empty")
	}

	resolved, err := net.ResolveTCPAddr("tcp", remoteAddr)
	if err == nil {
		return resolved, nil
	}

	var addrErr *net.AddrError
	if errors.As(err, &addrErr) && addrErr.Err == "missing port in address" {
		if len(remoteAddr) > 253 { // Max hostname length
			return nil, errors.New("remote address too long")
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
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)

		// Convert net/http -> fasthttp request with size limit
		const maxBodySize = 10 * 1024 * 1024 // 10MB limit
		if r.Body != nil {
			if r.ContentLength > maxBodySize {
				http.Error(w, utils.StatusMessage(fiber.StatusRequestEntityTooLarge), fiber.StatusRequestEntityTooLarge)
				return
			}
			limitedReader := io.LimitReader(r.Body, maxBodySize)
			n, err := io.Copy(req.BodyWriter(), limitedReader)
			req.Header.SetContentLength(int(n))

			if err != nil {
				http.Error(w, utils.StatusMessage(fiber.StatusInternalServerError), fiber.StatusInternalServerError)
				return
			}
		}
		req.Header.SetMethod(r.Method)
		req.SetRequestURI(r.RequestURI)
		req.SetHost(r.Host)
		req.Header.SetHost(r.Host)

		for key, val := range r.Header {
			for _, v := range val {
				req.Header.Set(key, v)
			}
		}

		remoteAddr, err := resolveRemoteAddr(r.RemoteAddr, r.Context().Value(http.LocalAddrContextKey))
		if err != nil {
			remoteAddr = nil // Fallback to nil
		}

		// New fasthttp Ctx from pool
		fctx := ctxPool.Get().(*fasthttp.RequestCtx) //nolint:forcetypeassert,errcheck // overlinting
		fctx.Response.Reset()
		fctx.Request.Reset()
		defer ctxPool.Put(fctx)
		fctx.Init(req, remoteAddr, &disableLogger{})

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
