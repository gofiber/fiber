---
id: sse
---

# SSE

Server-Sent Events middleware for [Fiber](https://github.com/gofiber/fiber) that provides a production-grade SSE broker built natively on Fiber's fasthttp architecture. It includes a Hub-based event broker with topic routing, event coalescing (last-writer-wins), three priority lanes (instant/batched/coalesced), NATS-style topic wildcards, adaptive per-connection throttling, connection groups, built-in JWT and ticket auth helpers, Prometheus metrics, graceful Kubernetes-style drain, auto fan-out from Redis/NATS, and pluggable Last-Event-ID replay.

## Signatures

```go
func New(config ...Config) fiber.Handler
func NewWithHub(config ...Config) (fiber.Handler, *Hub)
```

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

Use JWT authentication and metadata-based groups for multi-tenant isolation:

```go
handler, hub := sse.NewWithHub(sse.Config{
    OnConnect: sse.JWTAuth(func(token string) (map[string]string, error) {
        claims, err := validateJWT(token)
        if err != nil {
            return nil, err
        }
        return map[string]string{
            "user_id":   claims.UserID,
            "tenant_id": claims.TenantID,
        }, nil
    }),
})
app.Get("/events", handler)

// Publish only to a specific tenant
hub.DomainEvent("orders", "created", orderID, tenantID, nil)
```

Use event coalescing to reduce traffic for high-frequency updates:

```go
// Progress events use PriorityCoalesced — if progress goes 5%→8%
// in one flush window, only 8% is sent to the client.
hub.Progress("import", importID, tenantID, current, total, nil)

// Completion events use PriorityInstant — always delivered immediately.
hub.Complete("import", importID, tenantID, true, map[string]any{
    "rows_imported": 1500,
})
```

Use fan-out to bridge an external pub/sub system into the SSE hub:

```go
cancel := hub.FanOut(sse.FanOutConfig{
    Subscriber: redisSubscriber,
    Channel:    "events:orders",
    EventType:  "order-update",
    Topic:      "orders",
})
defer cancel()
```

## Config

| Property          | Type                                              | Description                                                                                                          | Default        |
| :---------------- | :------------------------------------------------ | :------------------------------------------------------------------------------------------------------------------- | :------------- |
| Next              | `func(fiber.Ctx) bool`                            | Next defines a function to skip this middleware when returned true.                                                   | `nil`          |
| OnConnect         | `func(fiber.Ctx, *Connection) error`              | Called when a new client connects. Set `conn.Topics` and `conn.Metadata` here. Return error to reject (sends 403).   | `nil`          |
| OnDisconnect      | `func(*Connection)`                               | Called after a client disconnects.                                                                                    | `nil`          |
| OnPause           | `func(*Connection)`                               | Called when a connection is paused (browser tab hidden).                                                              | `nil`          |
| OnResume          | `func(*Connection)`                               | Called when a connection is resumed (browser tab visible).                                                            | `nil`          |
| Replayer          | `Replayer`                                        | Enables Last-Event-ID replay. If nil, replay is disabled.                                                            | `nil`          |
| FlushInterval     | `time.Duration`                                   | How often batched (P1) and coalesced (P2) events are flushed to clients. Instant (P0) events bypass this.            | `2s`           |
| HeartbeatInterval | `time.Duration`                                   | How often a comment is sent to idle connections to detect disconnects and prevent proxy timeouts.                     | `30s`          |
| MaxLifetime       | `time.Duration`                                   | Maximum duration a single SSE connection can stay open. Set to -1 for unlimited.                                     | `30m`          |
| SendBufferSize    | `int`                                             | Per-connection channel buffer. If full, events are dropped.                                                          | `256`          |
| RetryMS           | `int`                                             | Reconnection interval hint sent to clients via the `retry:` directive on connect.                                    | `3000`         |

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
