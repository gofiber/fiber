package adaptor

import (
	"io"
	"net"
	"net/http"
	"reflect"
	"sync"
	"unsafe"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

type disableLogger struct{}

func (*disableLogger) Printf(string, ...any) {
}

var ctxPool = sync.Pool{
	New: func() any {
		return new(fasthttp.RequestCtx)
	},
}

// HTTPHandlerFunc wraps net/http handler func to fiber handler
func HTTPHandlerFunc(h http.HandlerFunc) fiber.Handler {
	return HTTPHandler(h)
}

// HTTPHandler wraps net/http handler to fiber handler
func HTTPHandler(h http.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		handler := fasthttpadaptor.NewFastHTTPHandler(h)
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
func CopyContextToFiberContext(context any, requestContext *fasthttp.RequestCtx) {
	contextValues := reflect.ValueOf(context).Elem()
	contextKeys := reflect.TypeOf(context).Elem()

	if contextKeys.Kind() != reflect.Struct {
		return
	}

	var lastKey any
	for i := 0; i < contextValues.NumField(); i++ {
		reflectValue := contextValues.Field(i)
		reflectField := contextKeys.Field(i)

		if reflectField.Name == "noCopy" {
			break
		}

		// Use unsafe to access potentially unexported fields.
		if reflectValue.CanAddr() {
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
				lastKey = nil // Reset lastKey after setting the value
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
			// Convert again in case request may modify by middleware
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

func handlerFunc(app *fiber.App, h ...fiber.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)

		// Convert net/http -> fasthttp request
		if r.Body != nil {
			n, err := io.Copy(req.BodyWriter(), r.Body)
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

		if _, _, err := net.SplitHostPort(r.RemoteAddr); err != nil && err.(*net.AddrError).Err == "missing port in address" { //nolint:errorlint, forcetypeassert // overlinting
			r.RemoteAddr = net.JoinHostPort(r.RemoteAddr, "80")
		}

		remoteAddr, err := net.ResolveTCPAddr("tcp", r.RemoteAddr)
		if err != nil {
			http.Error(w, utils.StatusMessage(fiber.StatusInternalServerError), fiber.StatusInternalServerError)
			return
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
		fctx.Response.Header.VisitAll(func(k, v []byte) {
			w.Header().Add(string(k), string(v))
		})
		w.WriteHeader(fctx.Response.StatusCode())
		_, _ = w.Write(fctx.Response.Body()) //nolint:errcheck // not needed
	}
}
