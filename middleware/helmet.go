package middleware

import (
	"fmt"

	"github.com/fenny/fiber"
)

// Helmet : Helps secure your apps by setting various HTTP headers.
func Helmet() func(*fiber.Midware) {
	return func(c *fiber.Midware) {
		fmt.Println("Helmet is still under development, this middleware does nothing yet.")
		c.Next()
	}
}
