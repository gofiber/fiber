package middleware

import (
	"github.com/gofiber/fiber"
)

// CSRF :
func CSRF() func(*fiber.Ctx) {
	return func(c *fiber.Ctx) {
		c.Next()
	}
}
