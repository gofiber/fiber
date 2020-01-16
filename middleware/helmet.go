package middleware

import (
	"fmt"

	"github.com/gofiber/fiber"
)

// Helmet : Helps secure your apps by setting various HTTP headers.
func Helmet(c *fiber.Ctx) {
	fmt.Println("Helmet is still under development, disable until v1.0.0")
	c.Next()
}
