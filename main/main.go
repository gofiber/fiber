package main

import "github.com/gofiber/fiber"

func main() {
	app := fiber.New()

	app.Static("/", "./website")

	app.Listen(80)
}
