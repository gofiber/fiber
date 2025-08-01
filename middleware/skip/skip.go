package skip

import (
	"github.com/gofiber/fiber/v3"
)

// New returns a middleware that calls the provided predicate for each request.
// If the predicate evaluates to true the wrapped handler is skipped and the next
// handler in the chain is executed.
func New(handler fiber.Handler, exclude func(c fiber.Ctx) bool) fiber.Handler {
	if exclude == nil {
		return handler
	}

	return func(c fiber.Ctx) error {
		if exclude(c) {
			return c.Next()
		}

		return handler(c)
	}
}
