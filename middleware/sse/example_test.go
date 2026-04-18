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
			conn.Metadata["user_id"] = "example"
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

func Example_invalidation() {
	_, hub := NewWithHub()

	// Replace polling: instead of clients polling every 30s,
	// push an invalidation signal when data changes.
	hub.Invalidate("orders", "ord_123", "created")

	// Multi-tenant
	hub.InvalidateForTenant("t_1", "orders", "ord_456", "updated")

	// With hints (small extra data)
	hub.InvalidateWithHint("orders", "ord_789", "created", map[string]any{
		"total": 149.99,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := hub.Shutdown(ctx); err != nil {
		panic(err)
	}

	fmt.Println("Invalidation events published") //nolint:errcheck // example test output
	// Output: Invalidation events published
}

func Example_progress() {
	_, hub := NewWithHub()

	// Coalesced: if progress goes 1%→2%→3%→4% in one flush window,
	// only 4% is sent to the client.
	for i := 1; i <= 100; i++ {
		hub.Progress("import", "imp_1", "t_1", i, 100)
	}
	hub.Complete("import", "imp_1", "t_1", true, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := hub.Shutdown(ctx); err != nil {
		panic(err)
	}

	fmt.Println("Progress tracking complete") //nolint:errcheck // example test output
	// Output: Progress tracking complete
}

func Example_ticketAuth() {
	store := NewMemoryTicketStore()
	defer store.Close()

	// Issue a ticket (typically in a POST handler after JWT validation)
	ticket, err := IssueTicket(store, `{"tenant":"t_1","topics":"orders,products"}`, 30*time.Second)
	if err != nil {
		panic(err)
	}

	fmt.Println("Ticket issued, length:", len(ticket)) //nolint:errcheck // example test output
	// Output: Ticket issued, length: 48
}
