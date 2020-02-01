package middleware

import "github.com/gofiber/fiber"

// Cors : Enable cross-origin resource sharing (CORS) with various options.
func Cors(c *fiber.Ctx, d string) {
	c.Set("Access-Control-Allow-Origin", d)
	c.Next()
}
