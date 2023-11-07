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

	// Construct actual prefix
	prefix := cfg.Prefix + "/debug/pprof"

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		path := c.Path()
		// We are only interested in /debug/pprof routes
		path, found := cutPrefix(path, prefix)
		if !found {
			return c.Next()
		}
		// Switch on trimmed path against constant strings
		switch path {
		case "/":
			pprofIndex(c.Context())
		case "/cmdline":
			pprofCmdline(c.Context())
		case "/profile":
			pprofProfile(c.Context())
		case "/symbol":
			pprofSymbol(c.Context())
		case "/trace":
			pprofTrace(c.Context())
		case "/allocs":
			pprofAllocs(c.Context())
		case "/block":
			pprofBlock(c.Context())
		case "/goroutine":
			pprofGoroutine(c.Context())
		case "/heap":
			pprofHeap(c.Context())
		case "/mutex":
			pprofMutex(c.Context())
		case "/threadcreate":
			pprofThreadcreate(c.Context())
		default:
			// pprof index only works with trailing slash
			if strings.HasSuffix(path, "/") {
				path = strings.TrimRight(path, "/")
			} else {
				path = prefix + "/"
			}

			return c.Redirect(path, fiber.StatusFound)
		}
		return nil
	}
}

// cutPrefix is a copy of [strings.CutPrefix] added in Go 1.20.
// Remove this function when we drop support for Go 1.19.
//
//nolint:nonamedreturns // Align with its original form in std.
func cutPrefix(s, prefix string) (after string, found bool) {
	if !strings.HasPrefix(s, prefix) {
		return s, false
	}
	return s[len(prefix):], true
}
