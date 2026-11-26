package pprof

import (
	"errors"
	"net/http"
	"net/http/pprof"
	"net/url"
	runtimepprof "runtime/pprof"
	"strings"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/appconfig"
	"github.com/gofiber/fiber/v3/internal/bodystream"
)

// profileNameKey carries the name of a registered profile to pprofCustom.
type profileNameKey struct{}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Set pprof adaptors
	var (
		pprofIndex        = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Index)
		pprofCmdline      = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Cmdline)
		pprofProfile      = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Profile)
		pprofSymbol       = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Symbol)
		pprofTrace        = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Trace)
		pprofAllocs       = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("allocs").ServeHTTP)
		pprofBlock        = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("block").ServeHTTP)
		pprofGoroutine    = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("goroutine").ServeHTTP)
		pprofHeap         = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("heap").ServeHTTP)
		pprofMutex        = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("mutex").ServeHTTP)
		pprofThreadcreate = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("threadcreate").ServeHTTP)
		pprofCustom       = fasthttpadaptor.NewFastHTTPHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name, _ := r.Context().Value(profileNameKey{}).(string) //nolint:errcheck // set by the handler below
			pprof.Handler(name).ServeHTTP(w, r)
		})
	)

	// Construct actual prefix
	prefix := cfg.Prefix + "/debug/pprof"

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		path := c.Path()
		// We are only interested in /debug/pprof routes
		path, found := strings.CutPrefix(path, prefix)
		if !found {
			return c.Next()
		}
		// The remainder must be empty or start a new path segment. Without
		// this check the bare prefix also matches sibling routes that merely
		// begin with the same characters, e.g. /debug/pprofiler, which would
		// then be redirected to the pprof index instead of reaching the app.
		if path != "" && path[0] != '/' {
			return c.Next()
		}
		if err := bufferBodyStream(c); err != nil {
			return err
		}
		// Switch on trimmed path against constant strings
		switch path {
		case "/":
			pprofIndex(c.RequestCtx())
		case "/cmdline":
			pprofCmdline(c.RequestCtx())
		case "/profile":
			pprofProfile(c.RequestCtx())
		case "/symbol":
			pprofSymbol(c.RequestCtx())
		case "/trace":
			pprofTrace(c.RequestCtx())
		case "/allocs":
			pprofAllocs(c.RequestCtx())
		case "/block":
			pprofBlock(c.RequestCtx())
		case "/goroutine":
			pprofGoroutine(c.RequestCtx())
		case "/heap":
			pprofHeap(c.RequestCtx())
		case "/mutex":
			pprofMutex(c.RequestCtx())
		case "/threadcreate":
			pprofThreadcreate(c.RequestCtx())
		default:
			// The index also links profiles registered with runtime/pprof.NewProfile,
			// escaped; c.Path() keeps the escapes unless UnescapePath decoded them.
			name := strings.TrimPrefix(path, "/")
			if !appconfig.Of(c.App()).UnescapePath {
				if unescaped, err := url.PathUnescape(name); err == nil {
					name = unescaped
				}
			}
			if name != "" && runtimepprof.Lookup(name) != nil {
				c.RequestCtx().SetUserValue(profileNameKey{}, name)
				pprofCustom(c.RequestCtx())
				return nil
			}
			// pprof index only works with trailing slash
			if strings.HasSuffix(path, "/") {
				path = prefix + utils.TrimRight(path, '/')
			} else {
				path = prefix + "/"
			}

			return c.Redirect().To(path)
		}
		return nil
	}
}

// bufferBodyStream holds a streamed body to BodyLimit, since fasthttpadaptor reads it whole into the http.Request.
func bufferBodyStream(c fiber.Ctx) error {
	err := bodystream.Buffer(c.Request(), c.App().Server().MaxRequestBodySize)
	var fe *fiber.Error
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fasthttp.ErrBodyTooLarge):
		return fiber.ErrRequestEntityTooLarge
	case errors.As(err, &fe):
		return fe
	}
	return fiber.ErrBadRequest
}
