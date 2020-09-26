package tpl

func MainTemplate() []byte {
	return []byte(`package main

import (
	"github.com/gofiber/fiber/v2"
	"log"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})
	
	return app.Listen(":3000")
}
`)
}
