package middleware

import "github.com/gofiber/fiber"

// app.Use(middleware.Cors())

// Cors : Enable cross-origin resource sharing (CORS) with various options.
func Cors(origin ...string) func(*fiber.Ctx) {
	return func(c *fiber.Ctx) {
		o := "*"
		if len(origin) > 0 {
			o = origin[0]
		}
		c.Set("Access-Control-Allow-Origin", o)
		c.Set("Access-Control-Allow-Headers", "X-Requested-With")
		c.Next()
	}
}
