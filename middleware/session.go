package middleware

import (
	"github.com/gofiber/fiber"
)

// Session :
func Session() func(*fiber.Ctx) {
	return func(c *fiber.Ctx) {
		c.Next()
	}
}
