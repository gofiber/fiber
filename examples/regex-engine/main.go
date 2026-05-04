// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📄 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

//go:build ignore

// Example demonstrating how to use coregex (high-performance regex engine) with Fiber.
//
// To run this example:
//   go run -tags=ignore ./examples/regex-engine/main.go
//
// The coregex package provides a drop-in replacement for Go's standard regexp package
// with significant performance improvements (3-3000x faster in many cases).
//
// See: https://github.com/coregx/coregex
package main

import (
	"log"

	"github.com/gofiber/fiber/v3"
)

// CoregexEngine wraps coregex to implement fiber.RegexEngine interface.
// This adapter allows using coregex as the regex engine for Fiber routing.
//
// Note: To actually use this, you need to:
// 1. Install coregex: go get github.com/coregx/coregex
// 2. Import it: import "github.com/coregx/coregex"
// 3. Uncomment the implementation below
//
// type CoregexEngine struct{}
//
// func (CoregexEngine) MustCompile(pattern string) fiber.RegexCompiler {
//     return &CoregexCompiler{
//         Regex: coregex.MustCompile(pattern),
//     }
// }
//
// type CoregexCompiler struct {
//     *coregex.Regex
// }
//
// func (c *CoregexCompiler) FindAllStringSubmatch(s string, n int) [][]string {
//     return c.Regex.FindAllStringSubmatch(s, n)
// }

func main() {
	// Example using the default regex engine (stdlib regexp)
	app := fiber.New()

	// Routes with regex constraints work with the default engine
	app.Get("/api/v1/:id<regex(\\d+)>", func(c fiber.Ctx) error {
		return c.SendString("ID: " + c.Params("id"))
	})

	// Email validation pattern
	app.Get("/user/:email<regex([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,})>", func(c fiber.Ctx) error {
		return c.SendString("Email: " + c.Params("email"))
	})

	// Date pattern (YYYY-MM-DD)
	app.Get("/date/:date<regex(\\d{4}-\\d{2}-\\d{2})>", func(c fiber.Ctx) error {
		return c.SendString("Date: " + c.Params("date"))
	})

	// UUID pattern
	app.Get("/resource/:uuid<regex([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})>", func(c fiber.Ctx) error {
		return c.SendString("UUID: " + c.Params("uuid"))
	})

	// Example: To use coregex instead, create the app with:
	// app := fiber.New(fiber.Config{
	//     RegexEngine: CoregexEngine{},
	// })
	//
	// This would make all regex constraints use the coregex engine for improved performance.

	log.Printf("Server starting on http://localhost:3000")
	log.Printf("Try these URLs:")
	log.Printf("  http://localhost:3000/api/v1/123")
	log.Printf("  http://localhost:3000/user/test@example.com")
	log.Printf("  http://localhost:3000/date/2024-01-15")
	log.Printf("  http://localhost:3000/resource/550e8400-e29b-41d4-a716-446655440000")

	if err := app.Listen(":3000"); err != nil {
		log.Fatal(err)
	}
}
