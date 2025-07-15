package main

import (
	"bufio"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

func main() {
	app := fiber.New(fiber.Config{
		AppName: "Fiber v3 SSE Demo",
	})

	// Serve the demo HTML file
	app.Get("/", static.New("./"))

	// Basic SSE endpoint - demonstrates simple event streaming
	app.Get("/events", func(c fiber.Ctx) error {
		// Set required SSE headers
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Access-Control-Allow-Origin", "*")

		return c.SendStreamWriter(func(w *bufio.Writer) {
			// Send periodic updates
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for i := 0; i < 10; i++ {
				select {
				case t := <-ticker.C:
					fmt.Fprintf(w, "data: Event %d at %s\n\n", i+1, t.Format(time.RFC3339))
					if err := w.Flush(); err != nil {
						// Client disconnected
						log.Printf("Client disconnected from /events: %v", err)
						return
					}
				}
			}

			// Send final message
			fmt.Fprintf(w, "data: All events sent! Connection will close.\n\n")
			w.Flush()
		})
	})

	// Typed SSE endpoint - demonstrates different event types
	app.Get("/typed-events", func(c fiber.Ctx) error {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Access-Control-Allow-Origin", "*")

		return c.SendStreamWriter(func(w *bufio.Writer) {
			// Welcome message
			fmt.Fprintf(w, "event: welcome\ndata: Connected to server\n\n")
			if err := w.Flush(); err != nil {
				log.Printf("Client disconnected during welcome: %v", err)
				return
			}

			// Send periodic notifications
			for i := 1; i <= 5; i++ {
				fmt.Fprintf(w, "id: %d\nevent: notification\ndata: {\"count\": %d, \"message\": \"Update %d\"}\n\n", i, i, i)
				if err := w.Flush(); err != nil {
					log.Printf("Client disconnected during notification %d: %v", i, err)
					return
				}
				time.Sleep(1 * time.Second)
			}

			// Goodbye message
			fmt.Fprintf(w, "event: goodbye\ndata: Connection closing\n\n")
			w.Flush()
		})
	})

	// Infinite SSE endpoint - demonstrates long-lived connections
	app.Get("/infinite-events", func(c fiber.Ctx) error {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Access-Control-Allow-Origin", "*")

		return c.SendStreamWriter(func(w *bufio.Writer) {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			counter := 0
			for {
				select {
				case t := <-ticker.C:
					counter++
					fmt.Fprintf(w, "data: Infinite event #%d at %s\n\n", counter, t.Format("15:04:05"))
					if err := w.Flush(); err != nil {
						log.Printf("Client disconnected from infinite stream: %v", err)
						return
					}
				}
			}
		})
	})

	// Health check endpoint
	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"time":    time.Now().Format(time.RFC3339),
			"message": "Fiber v3 SSE Demo Server",
		})
	})

	log.Println("Starting Fiber v3 SSE Demo Server...")
	log.Println("Open http://localhost:3000/sse_demo.html to see the demo")
	log.Println("Available endpoints:")
	log.Println("  GET /events           - Basic SSE events")
	log.Println("  GET /typed-events     - Typed SSE events")
	log.Println("  GET /infinite-events  - Infinite SSE stream")
	log.Println("  GET /health           - Health check")

	log.Fatal(app.Listen(":3000"))
}