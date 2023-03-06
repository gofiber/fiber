// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://fiber.wiki
// 📝 Github Repository: https://github.com/gofiber/fiber

package adaptor

import (
	"io"
	"net"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// HTTPHandlerFunc wraps net/http handler func to fiber handler
func HTTPHandlerFunc(h http.HandlerFunc) fiber.Handler {
	return HTTPHandler(h)
}

// HTTPHandler wraps net/http handler to fiber handler
func HTTPHandler(h http.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		handler := fasthttpadaptor.NewFastHTTPHandler(h)
		handler(c.Context())
		return nil
	}
}

// HTTPMiddleware wraps net/http middleware to fiber middleware
func HTTPMiddleware(mw func(http.Handler) http.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		var next bool
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next = true
			// Convert again in case request may modify by middleware
			c.Request().Header.SetMethod(r.Method)
			c.Request().SetRequestURI(r.RequestURI)
			c.Request().SetHost(r.Host)
			for key, val := range r.Header {
				for _, v := range val {
					c.Request().Header.Set(key, v)
				}
			}
		})
		_ = HTTPHandler(mw(nextHandler))(c) //nolint:errcheck // TODO
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
		// New fasthttp request
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		// Convert net/http -> fasthttp request
		if r.Body != nil {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, utils.StatusMessage(fiber.StatusInternalServerError), fiber.StatusInternalServerError)
				return
			}
			req.Header.SetContentLength(len(body))
			_, _ = req.BodyWriter().Write(body) //nolint:errcheck // TODO
		}
		req.Header.SetMethod(r.Method)
		req.SetRequestURI(r.RequestURI)
		req.SetHost(r.Host)
		for key, val := range r.Header {
			for _, v := range val {
				req.Header.Set(key, v)
			}
		}
		if _, _, err := net.SplitHostPort(r.RemoteAddr); err != nil && err.(*net.AddrError).Err == "missing port in address" {
			r.RemoteAddr = net.JoinHostPort(r.RemoteAddr, "80")
		}
		remoteAddr, err := net.ResolveTCPAddr("tcp", r.RemoteAddr)
		if err != nil {
			http.Error(w, utils.StatusMessage(fiber.StatusInternalServerError), fiber.StatusInternalServerError)
			return
		}

		// New fasthttp Ctx
		var fctx fasthttp.RequestCtx
		fctx.Init(req, remoteAddr, nil)
		if len(h) > 0 {
			// New fiber Ctx
			ctx := app.NewCtx(&fctx)
			// Execute fiber Ctx
			err := h[0](ctx)
			if err != nil {
				_ = app.Config().ErrorHandler(ctx, err)
			}
		} else {
			// Execute fasthttp Ctx though app.Handler
			app.Handler()(&fctx)
		}

		// Convert fasthttp Ctx > net/http
		fctx.Response.Header.VisitAll(func(k, v []byte) {
			w.Header().Add(string(k), string(v))
		})
		w.WriteHeader(fctx.Response.StatusCode())
		_, _ = w.Write(fctx.Response.Body()) //nolint:errcheck // not needed
	}
}
