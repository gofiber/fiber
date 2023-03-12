package pprof

import (
	"net/http/pprof"
	"strings"

	"github.com/gofiber/fiber/v2"

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

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		path := c.Path()
		// We are only interested in /debug/pprof routes
		if len(path) < 12 || !strings.HasPrefix(path, cfg.Prefix+"/debug/pprof") {
			return c.Next()
		}
		// Switch to original path without stripped slashes
		switch path {
		case cfg.Prefix + "/debug/pprof/":
			pprofIndex(c.Context())
		case cfg.Prefix + "/debug/pprof/cmdline":
			pprofCmdline(c.Context())
		case cfg.Prefix + "/debug/pprof/profile":
			pprofProfile(c.Context())
		case cfg.Prefix + "/debug/pprof/symbol":
			pprofSymbol(c.Context())
		case cfg.Prefix + "/debug/pprof/trace":
			pprofTrace(c.Context())
		case cfg.Prefix + "/debug/pprof/allocs":
			pprofAllocs(c.Context())
		case cfg.Prefix + "/debug/pprof/block":
			pprofBlock(c.Context())
		case cfg.Prefix + "/debug/pprof/goroutine":
			pprofGoroutine(c.Context())
		case cfg.Prefix + "/debug/pprof/heap":
			pprofHeap(c.Context())
		case cfg.Prefix + "/debug/pprof/mutex":
			pprofMutex(c.Context())
		case cfg.Prefix + "/debug/pprof/threadcreate":
			pprofThreadcreate(c.Context())
		default:
			// pprof index only works with trailing slash
			if strings.HasSuffix(path, "/") {
				path = strings.TrimRight(path, "/")
			} else {
				path = cfg.Prefix + "/debug/pprof/"
			}

			return c.Redirect(path, fiber.StatusFound)
		}
		return nil
	}
}
