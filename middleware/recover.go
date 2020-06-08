package middleware

import (
	"fmt"

	"github.com/gofiber/fiber"
)

// Middleware types
type (
	// RecoverConfig defines the config for Logger middleware.
	RecoverConfig struct {
		// Next defines a function to skip this middleware.
		Next func(ctx *fiber.Ctx) bool
	}
)

// RecoverConfigDefault is the default config
var RecoverConfigDefault = RecoverConfig{
	Next: nil,
}

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
