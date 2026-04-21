// Package sse provides Server-Sent Events middleware for Fiber.
//
// It is the only SSE implementation built natively for Fiber's
// fasthttp architecture — no net/http adapters, no broken disconnect
// detection.
//
// Features: event coalescing (last-writer-wins), three priority lanes
// (instant/batched/coalesced), NATS-style topic wildcards, adaptive
// per-connection throttling, connection groups (publish by metadata),
// graceful Kubernetes-style drain, pluggable Last-Event-ID replay,
// and a SubscriberBridge adapter for external pub/sub sources such as
// Redis and NATS.
//
// Quick start:
//
//	handler, hub := sse.NewWithHub(sse.Config{
//	    OnConnect: func(c fiber.Ctx, conn *sse.Connection) error {
//	        conn.Topics = []string{"notifications"}
//	        return nil
//	    },
//	})
//	app.Get("/events", handler)
//	hub.Publish(sse.Event{Type: "ping", Data: "hello", Topics: []string{"notifications"}})
//
// The middleware is terminal: the returned handler hijacks the response
// stream via Fiber's SendStreamWriter and never calls c.Next(). Do not
// chain additional handlers after it.
package sse

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"maps"

	"github.com/gofiber/fiber/v3"
)

// New creates a new SSE middleware handler. Use this when you don't need
// direct access to the Hub (e.g., simple streaming without Publish).
//
// For most use cases, prefer [NewWithHub] instead.
func New(config ...Config) fiber.Handler {
	handler, _ := NewWithHub(config...)
	return handler
}

// NewWithHub creates a new SSE middleware handler and returns it along
// with the Hub for publishing events. This is the primary entry point.
//
//	handler, hub := sse.NewWithHub(sse.Config{
//	    OnConnect: func(c fiber.Ctx, conn *sse.Connection) error {
//	        conn.Topics = []string{"notifications", "live"}
//	        conn.Metadata["tenant_id"] = c.Locals("tenant_id").(string)
//	        return nil
//	    },
//	})
//	app.Get("/events", handler)
//
//	// From any handler or worker:
//	hub.Publish(sse.Event{Type: "update", Data: "hello", Topics: []string{"live"}})
func NewWithHub(config ...Config) (fiber.Handler, *Hub) {
	cfg := configDefault(config...)
	hub := newHub(cfg)

	handler := func(c fiber.Ctx) error {
		// Reject during graceful drain.
		if hub.draining.Load() {
			c.Set("Retry-After", "5")
			return c.Status(fiber.StatusServiceUnavailable).SendString("server draining, please reconnect")
		}

		conn := newConnection(
			generateID(),
			nil,
			cfg.SendBufferSize,
			cfg.FlushInterval,
		)

		// Let the application authenticate and configure the connection.
		// The returned error is never exposed to the client — callers may
		// include user/tenant identifiers or internal policy reasons that
		// would leak information to an unauthenticated peer. The error is
		// returned to the caller for logging via the standard Fiber error
		// pipeline.
		if cfg.OnConnect != nil {
			if err := cfg.OnConnect(c, conn); err != nil {
				return fiber.NewError(fiber.StatusForbidden, "forbidden")
			}
		}

		// Freeze metadata — defensive copy to prevent concurrent mutation
		// after the connection is registered with the hub.
		frozen := make(map[string]string, len(conn.Metadata))
		maps.Copy(frozen, conn.Metadata)
		conn.Metadata = frozen

		if len(conn.Topics) == 0 {
			return c.Status(fiber.StatusBadRequest).SendString("no topics subscribed")
		}

		// SSE response headers.
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("X-Accel-Buffering", "no")

		// Capture Last-Event-ID before entering the stream writer.
		lastEventID := c.Get("Last-Event-ID")
		if lastEventID == "" {
			lastEventID = c.Query("lastEventID")
		}

		// Abandon the ctx so Fiber does not return it to the pool while
		// fasthttp is still invoking the stream writer in a background
		// goroutine.
		c.Abandon()

		return c.SendStreamWriter(func(w *bufio.Writer) {
			defer func() {
				select {
				case hub.unregister <- conn:
				case <-hub.shutdown:
				}
				conn.Close()
				if cfg.OnDisconnect != nil {
					cfg.OnDisconnect(conn)
				}
			}()

			// Register BEFORE writing the preamble / replay so events
			// published during replay buffer in conn.send instead of being
			// dropped. Event IDs are monotonic, so live events always have
			// higher IDs than any replayed event — no duplicates are
			// possible with a Last-Event-ID strictly-after replayer.
			select {
			case hub.register <- conn:
			case <-hub.shutdown:
				return
			}

			if err := hub.initStream(w, conn, lastEventID); err != nil {
				return
			}

			hub.watchLifetime(conn)
			conn.writeLoop(w)
		})
	}

	return handler, hub
}

// generateID produces a random 32-character hex string for connection IDs.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("sse: failed to generate connection ID: " + err.Error())
	}
	return hex.EncodeToString(b)
}
