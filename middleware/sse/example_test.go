package sse

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
)

func Example() {
	app := fiber.New()

	handler, hub := NewWithHub(Config{
		OnConnect: func(_ fiber.Ctx, conn *Connection) error {
			conn.Topics = []string{"notifications"}
			return nil
		},
	})

	app.Get("/events", handler)

	// Publish from any handler or worker
	hub.Publish(Event{
		Type:   "update",
		Data:   map[string]string{"message": "hello"},
		Topics: []string{"notifications"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := hub.Shutdown(ctx); err != nil {
		panic(err)
	}

	fmt.Println("Hub created and shut down successfully") //nolint:errcheck // example test output
	// Output: Hub created and shut down successfully
}

func Example_priorities() {
	_, hub := NewWithHub()

	// Instant: delivered immediately, bypasses buffering
	hub.Publish(Event{
		Type:     "alert",
		Data:     "critical",
		Topics:   []string{"alerts"},
		Priority: PriorityInstant,
	})

	// Coalesced: last-writer-wins per CoalesceKey within flush window
	for i := 1; i <= 100; i++ {
		hub.Publish(Event{
			Type:        "progress",
			Data:        fmt.Sprintf(`{"pct":%d}`, i),
			Topics:      []string{"progress"},
			Priority:    PriorityCoalesced,
			CoalesceKey: "import",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := hub.Shutdown(ctx); err != nil {
		panic(err)
	}

	fmt.Println("Events published") //nolint:errcheck // example test output
	// Output: Events published
}

func Example_topicWildcards() {
	_, hub := NewWithHub(Config{
		OnConnect: func(_ fiber.Ctx, conn *Connection) error {
			// Subscribe to all order events using NATS-style wildcard
			conn.Topics = []string{"orders.*"}
			return nil
		},
	})

	// These will all match orders.*
	hub.Publish(Event{Type: "created", Topics: []string{"orders.created"}})
	hub.Publish(Event{Type: "updated", Topics: []string{"orders.updated"}})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := hub.Shutdown(ctx); err != nil {
		panic(err)
	}

	fmt.Println("Wildcard subscription example") //nolint:errcheck // example test output
	// Output: Wildcard subscription example
}
