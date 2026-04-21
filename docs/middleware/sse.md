---
id: sse
---

# SSE

Server-Sent Events middleware for [Fiber](https://github.com/gofiber/fiber) built natively on Fiber's fasthttp architecture. It provides a Hub-based event broker with topic routing, three priority lanes (instant/batched/coalesced), NATS-style topic wildcards, adaptive per-connection throttling, connection groups, graceful drain, and pluggable Last-Event-ID replay.

The middleware is fully compatible with the standard SSE wire format — any client that speaks Server-Sent Events (browser `EventSource`, `curl -N`, or any HTTP client that reads `text/event-stream`) works with it.

## Signatures

```go
func New(config ...Config) fiber.Handler
func NewWithHub(config ...Config) (fiber.Handler, *Hub)
```

`New` returns just the handler; use `NewWithHub` when you need access to the hub for publishing events.

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/sse"
)
```

Once your Fiber app is initialized, create an SSE handler and hub:

```go
// Basic usage — subscribe all clients to "notifications"
handler, hub := sse.NewWithHub(sse.Config{
    OnConnect: func(c fiber.Ctx, conn *sse.Connection) error {
        conn.Topics = []string{"notifications"}
        return nil
    },
})
app.Get("/events", handler)

// Publish an event from any goroutine
hub.Publish(sse.Event{
    Type:   "update",
    Data:   "hello",
    Topics: []string{"notifications"},
})
```

Use NATS-style wildcards to subscribe to multiple related topics:

```go
handler, hub := sse.NewWithHub(sse.Config{
    OnConnect: func(c fiber.Ctx, conn *sse.Connection) error {
        // Match orders.created, orders.updated, orders.deleted
        conn.Topics = []string{"orders.*"}
        return nil
    },
})
```

Use connection groups (metadata-based filtering) for multi-tenant isolation:

```go
handler, hub := sse.NewWithHub(sse.Config{
    OnConnect: func(c fiber.Ctx, conn *sse.Connection) error {
        tenantID := c.Locals("tenant_id").(string)
        conn.Metadata["tenant_id"] = tenantID
        conn.Topics = []string{"orders"}
        return nil
    },
})

// Publish only to connections in tenant "t_123"
hub.Publish(sse.Event{
    Type:   "order-created",
    Data:   orderJSON,
    Topics: []string{"orders"},
    Group:  map[string]string{"tenant_id": "t_123"},
})
```

Use event coalescing to reduce traffic for high-frequency updates:

```go
// Coalesced: if progress goes 5%→8% in one flush window,
// only the latest value is sent.
for i := 1; i <= 100; i++ {
    hub.Publish(sse.Event{
        Type:        "progress",
        Data:        fmt.Sprintf(`{"pct":%d}`, i),
        Topics:      []string{"import"},
        Priority:    sse.PriorityCoalesced,
        CoalesceKey: "import-progress",
    })
}
```

Fan out from an external pub/sub system (Redis, NATS, etc.) into the hub. Implement the `SubscriberBridge` interface and declare it on `Config.Bridges` — the middleware auto-starts each bridge and cancels/awaits them on `hub.Shutdown`, so there are no `CancelFunc`s for the caller to track.

```go
type redisSubscriber struct{ client *redis.Client }

func (r *redisSubscriber) Subscribe(ctx context.Context, channel string, onMessage func(string)) error {
    sub := r.client.Subscribe(ctx, channel)
    defer sub.Close()
    for msg := range sub.Channel() {
        onMessage(msg.Payload)
    }
    return ctx.Err()
}

handler, hub := sse.NewWithHub(sse.Config{
    Bridges: []sse.BridgeConfig{{
        Subscriber: &redisSubscriber{client: rdb},
        Channel:    "notifications",
        Topic:      "notifications",
        EventType:  "notification",
    }},
})
app.Get("/events", handler)
```

Graceful shutdown with deadline:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := hub.Shutdown(ctx); err != nil {
    log.Errorf("sse drain failed: %v", err)
}
```

Authentication is left to the user via `OnConnect`. Note that browser `EventSource` cannot send custom headers, so if you need token authentication, consider passing the token via a query parameter or a short-lived ticket exchanged on a separate endpoint.

## Config

| Property          | Type                                              | Description                                                                                                          | Default        |
| :---------------- | :------------------------------------------------ | :------------------------------------------------------------------------------------------------------------------- | :------------- |
| OnConnect         | `func(fiber.Ctx, *Connection) error`              | Called when a new client connects. Set `conn.Topics` and `conn.Metadata` here. Return error to reject (sends 403).   | `nil`          |
| OnDisconnect      | `func(*Connection)`                               | Called after a client disconnects.                                                                                    | `nil`          |
| OnPause           | `func(*Connection)`                               | Called when a connection is paused (browser tab hidden).                                                              | `nil`          |
| OnResume          | `func(*Connection)`                               | Called when a connection is resumed (browser tab visible).                                                            | `nil`          |
| Replayer          | `Replayer`                                        | Pluggable Last-Event-ID replay backend. If nil, replay is disabled.                                                   | `nil`          |
| Bridges           | `[]BridgeConfig`                                  | Auto-started bridges from external pub/sub systems. Each implements `SubscriberBridge`. Canceled on `hub.Shutdown`.   | `nil`          |
| FlushInterval     | `time.Duration`                                   | How often batched (P1) and coalesced (P2) events are flushed to clients. Instant (P0) events bypass this.            | `2s`           |
| HeartbeatInterval | `time.Duration`                                   | How often a comment is sent to idle connections to detect disconnects and prevent proxy timeouts.                     | `30s`          |
| MaxLifetime       | `time.Duration`                                   | Maximum duration a single SSE connection can stay open. Set to -1 for unlimited.                                     | `30m`          |
| SendBufferSize    | `int`                                             | Per-connection channel buffer. If full, events are dropped.                                                          | `256`          |
| RetryMS           | `int`                                             | Reconnection interval hint sent to clients via the `retry:` directive on connect.                                    | `3000`         |

The SSE middleware is **terminal** — the returned handler hijacks the response stream and never calls `c.Next()`. For the same reason `Config` does not include a `Next` field: placing handlers after the SSE middleware has no defined effect.

## Default Config

```go
var ConfigDefault = Config{
    FlushInterval:     2 * time.Second,
    SendBufferSize:    256,
    HeartbeatInterval: 30 * time.Second,
    MaxLifetime:       30 * time.Minute,
    RetryMS:           3000,
}
```
