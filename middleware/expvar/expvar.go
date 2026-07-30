package expvar

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp/expvarhandler"
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		const prefix = "/debug/vars"

		path := c.Path()
		if path == prefix {
			expvarhandler.ExpvarHandler(c.RequestCtx())
			return nil
		}

		// Only /debug/vars and paths beneath it belong to this middleware.
		// Matching on the bare prefix would also swallow sibling routes that
		// merely start with the same characters, e.g. /debug/varsdump.
		if !strings.HasPrefix(path, prefix+"/") {
			return c.Next()
		}

		return c.Redirect().To(prefix)
	}
}
