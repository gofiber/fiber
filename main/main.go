package main

import (
	"fmt"

	"github.com/gofiber/fiber"
)

func main() {
	app := fiber.New()

	app.Static("/", "./website")

	err := app.Listen(80)
	if err != nil {
		fmt.Printf("[Error] %s \n", err)
	}
}
