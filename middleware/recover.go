package middleware

import (
	"fmt"

	"github.com/gofiber/fiber"
)

// Recover will recover from panics and calls the ErrorHandler
func Recover() fiber.Handler {
	return func(ctx *fiber.Ctx) {
		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				if !ok {
					err = fmt.Errorf("%v", r)
				}
				ctx.Next(err)
				return
			}
		}()
		ctx.Next()
	}
}
