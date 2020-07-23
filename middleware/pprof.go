package middleware

import (
	"net/http/pprof"
	"strings"

	fiber "github.com/gofiber/fiber"
	fasthttpadaptor "github.com/valyala/fasthttp/fasthttpadaptor"
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

// Pprof will enabling profiling
func Pprof() fiber.Handler {
	// Return handler
	return func(c *fiber.Ctx) {
		path := c.Path()
		// We are only interested in /debug/pprof routes
		if len(path) < 12 || !strings.HasPrefix(path, "/debug/pprof") {
			c.Next()
			return
		}
		// Switch to original path without stripped slashes
		switch path {
		case "/debug/pprof/":
			c.Fasthttp.SetContentType(fiber.MIMETextHTML)
			pprofIndex(c.Fasthttp)
		case "/debug/pprof/cmdline":
			pprofCmdline(c.Fasthttp)
		case "/debug/pprof/profile":
			pprofProfile(c.Fasthttp)
		case "/debug/pprof/symbol":
			pprofSymbol(c.Fasthttp)
		case "/debug/pprof/trace":
			pprofTrace(c.Fasthttp)
		case "/debug/pprof/allocs":
			pprofAllocs(c.Fasthttp)
		case "/debug/pprof/block":
			pprofBlock(c.Fasthttp)
		case "/debug/pprof/goroutine":
			pprofGoroutine(c.Fasthttp)
		case "/debug/pprof/heap":
			pprofHeap(c.Fasthttp)
		case "/debug/pprof/mutex":
			pprofMutex(c.Fasthttp)
		case "/debug/pprof/threadcreate":
			pprofThreadcreate(c.Fasthttp)
		default:
			// pprof index only works with trailing slash
			c.Redirect("/debug/pprof/", 302)
		}
	}
}
