package pprof

import (
	"net/http/pprof"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

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
			// pprof index only works with trailing slash
			if strings.HasSuffix(path, "/") {
				path = utils.TrimRight(path, '/')
			} else {
				path = prefix + "/"
			}

			return c.Redirect().To(path)
		}
		return nil
	}
}
