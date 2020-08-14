package recover

import (
	"fmt"

	"github.com/gofiber/fiber"
)

// Recover will recover from panics and passes them to the global error handler
func New() fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				var ok bool
				if err, ok = r.(error); !ok {
					err = fmt.Errorf("%v", r)
				}
			}
		}()
		return c.Next()
	}
}
