package middleware

import (
	"fmt"

	"github.com/fenny/fiber"
)

// Cors :
func Cors(c *fiber.Ctx) {
	fmt.Println("LoL")
	c.Next()
}
