package middleware

import (
	"fmt"

	"github.com/fenny/fiber"
)

// Cors : Enable cross-origin resource sharing (CORS) with various options.
func Cors(c *fiber.Ctx) {
	fmt.Println("Cors is still under development, disable until v1.0.0")
	c.Next()
}
