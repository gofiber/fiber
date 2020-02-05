package middleware

import (
	"github.com/gofiber/fiber"
)

// Limiter :
func Limiter() func(*fiber.Ctx) {
	return func(c *fiber.Ctx) {
		c.Next()
	}
}
