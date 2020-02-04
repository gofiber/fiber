package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber"
)

// Morgan : Simple logger
func Morgan() func(*fiber.Ctx) {
	return func(c *fiber.Ctx) {
		currentTime := time.Now().Format("02 Jan, 15:04:05")
		fmt.Printf("%s \x1b[1;32m%s \x1b[1;37m%s\x1b[0000m, %s\n", currentTime, c.Method(), c.Path(), c.Get("User-Agent"))
		c.Next()
	}
}
