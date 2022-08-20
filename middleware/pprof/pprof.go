package pprof

import (
	"net/http/pprof"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

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

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		path := c.Path()
		// We are only interested in /debug/pprof routes
		if len(path) < 12 || !strings.HasPrefix(path, "/debug/pprof") {
			return c.Next()
		}
		// Switch to original path without stripped slashes
		switch path {
		case "/debug/pprof/":
			pprofIndex(c.Context())
		case "/debug/pprof/cmdline":
			pprofCmdline(c.Context())
		case "/debug/pprof/profile":
			pprofProfile(c.Context())
		case "/debug/pprof/symbol":
			pprofSymbol(c.Context())
		case "/debug/pprof/trace":
			pprofTrace(c.Context())
		case "/debug/pprof/allocs":
			pprofAllocs(c.Context())
		case "/debug/pprof/block":
			pprofBlock(c.Context())
		case "/debug/pprof/goroutine":
			pprofGoroutine(c.Context())
		case "/debug/pprof/heap":
			pprofHeap(c.Context())
		case "/debug/pprof/mutex":
			pprofMutex(c.Context())
		case "/debug/pprof/threadcreate":
			pprofThreadcreate(c.Context())
		default:
			// pprof index only works with trailing slash
			if strings.HasSuffix(path, "/") {
				path = strings.TrimRight(path, "/")
			} else {
				path = "/debug/pprof/"
			}

			return c.Redirect(path, fiber.StatusFound)
		}
		return nil
	}
}
