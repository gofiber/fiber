package pprof

import (
	"net/http/pprof"
	"strings"

	"github.com/gofiber/fiber"
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
func New() fiber.Handler {
	// Return new handler
	return func(c *fiber.Ctx) error {
		path := c.Path()
		// We are only interested in /debug/pprof routes
		if len(path) < 12 || !strings.HasPrefix(path, "/debug/pprof") {
			return c.Next()
		}
		// Switch to original path without stripped slashes
		switch path {
		case "/debug/pprof/":
			c.Request().SetContentType(fiber.MIMETextHTML)
			pprofIndex(c.Request())
		case "/debug/pprof/cmdline":
			pprofCmdline(c.Request())
		case "/debug/pprof/profile":
			pprofProfile(c.Request())
		case "/debug/pprof/symbol":
			pprofSymbol(c.Request())
		case "/debug/pprof/trace":
			pprofTrace(c.Request())
		case "/debug/pprof/allocs":
			pprofAllocs(c.Request())
		case "/debug/pprof/block":
			pprofBlock(c.Request())
		case "/debug/pprof/goroutine":
			pprofGoroutine(c.Request())
		case "/debug/pprof/heap":
			pprofHeap(c.Request())
		case "/debug/pprof/mutex":
			pprofMutex(c.Request())
		case "/debug/pprof/threadcreate":
			pprofThreadcreate(c.Request())
		default:
			// pprof index only works with trailing slash
			return c.Redirect("/debug/pprof/", 302)
		}
		return nil
	}
}
