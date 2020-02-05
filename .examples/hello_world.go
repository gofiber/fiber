package main

import "github.com/gofiber/fiber"

func main() {
	// Create new Fiber instance
	app := fiber.New()

	// Create new GET route on path "/"
	app.Get("/", func(c *fiber.Ctx) {
		c.Send("Hello, World!")
	})

	// Start server on http://localhost:8080
	app.Listen(8080)
}
