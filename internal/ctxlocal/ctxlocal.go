// Package ctxlocal stores request locals without the allocation a variadic
// Locals call through the Ctx interface costs.
package ctxlocal

import (
	"github.com/gofiber/fiber/v3"
)

// Set stores value under key on c and returns what Locals returned.
//
// Locals takes its value variadically. Reached through the Ctx interface the
// compiler cannot see that it only reads that argument, so the one-element
// "..." slice is heap-allocated on every call; the concrete method inlines and
// keeps it in the frame, which measured 50ns and a 16-byte allocation against
// 11ns and none. A custom Ctx fails the assertion and keeps the interface call,
// so an overridden Locals is still the one that runs. fiber's own
// StoreInContext does the same in-package; this is the form middleware and the
// extractors can reach.
func Set(c fiber.Ctx, key, value any) any {
	if dc, ok := c.(*fiber.DefaultCtx); ok {
		return dc.Locals(key, value)
	}
	return c.Locals(key, value)
}
