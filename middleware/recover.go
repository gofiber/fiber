package middleware

import (
	"fmt"
	"log"

	"github.com/gofiber/fiber"
)

// Recover ...
func Recover(handle ...func(*fiber.Ctx, error)) func(*fiber.Ctx) {
	log.Println("Warning: middleware.Recover() is deprecated since v1.8.2, please use github.com/gofiber/recover")
	h := func(c *fiber.Ctx, err error) {
		log.Println(err)
		c.SendStatus(500)
	}
	// Init custom error handler if exist
	if len(handle) > 0 {
		h = handle[0]
	}
	// Return middleware handle
	return func(c *fiber.Ctx) {
		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				if !ok {
					err = fmt.Errorf("%v", r)
				}
				h(c, err)
			}
		}()
		c.Next()
	}
}
