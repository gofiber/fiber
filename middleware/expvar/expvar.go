package expvar

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp/expvarhandler"
)

// New creates a new middleware handler
func New() fiber.Handler {
	// Return new handler
	return func(c *fiber.Ctx) error {
		path := c.Path()
		// We are only interested in /debug/vars routes
		if len(path) < 11 || !strings.HasPrefix(path, "/debug/vars") {
			return c.Next()
		}
		if path == "/debug/vars" {
			expvarhandler.ExpvarHandler(c.Context())
			return nil
		}

		return c.Redirect("/debug/vars", 302)
	}
}
