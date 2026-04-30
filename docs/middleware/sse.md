---
id: sse
---

# SSE

The SSE handler provides the transport pieces for Server-Sent Events: response headers, event formatting, flushing, heartbeat comments, and disconnect detection through `Flush` errors.

It intentionally does not include a hub, topics, authentication, replay storage, metrics, or external pub/sub bridges. Those are application concerns that can be composed around the stream handler.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package:

```go
import (
    "context"
    "time"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/sse"
)
```

Once your Fiber app is initialized, mount an SSE endpoint like this:

```go
app.Get("/events", sse.New(sse.Config{
    Retry: 5 * time.Second,
    Handler: func(c fiber.Ctx, stream *sse.Stream) error {
        return stream.Event(sse.Event{
            Name: "message",
            Data: fiber.Map{"message": "hello"},
        })
    },
}))
```

For long-running streams, subscribe each client to its own event channel and stop when the client disconnects.
A single shared channel load-balances messages across clients; use a fan-out source when every client must receive every event:

```go
type Broker interface {
    Subscribe(ctx context.Context) (<-chan string, error)
}

app.Get("/events", sse.New(sse.Config{
    Handler: func(c fiber.Ctx, stream *sse.Stream) error {
        events, err := broker.Subscribe(stream.Context())
        if err != nil {
            return err
        }

        for {
            select {
            case msg, ok := <-events:
                if !ok {
                    return nil
                }
                if err := stream.Event(sse.Event{Name: "message", Data: msg}); err != nil {
                    return err
                }
            case <-stream.Done():
                return stream.Err()
            }
        }
    },
}))
```

`stream.Context()` is canceled when the stream ends or a write fails, which makes it convenient to pass into database, broker, or gRPC calls:

```go
app.Get("/events", sse.New(sse.Config{
    Handler: func(c fiber.Ctx, stream *sse.Stream) error {
        rows, err := db.QueryContext(stream.Context(), "SELECT id FROM jobs")
        if err != nil {
            return err
        }
        defer rows.Close()

        return stream.Comment("connected")
    },
}))
```

## Config

| Property          | Type                     | Description                                                                                          | Default            |
|:------------------|:-------------------------|:-----------------------------------------------------------------------------------------------------|:-------------------|
| Handler           | `sse.Handler`            | Writes events to the stream.                                                                         | `nil`              |
| OnClose           | `func(fiber.Ctx, error)` | Called when the stream ends, with `nil` when the handler returned successfully and no stream write failed. | `nil`              |
| Retry             | `time.Duration`          | Initial EventSource reconnect delay.                                                                 | `0`                |
| HeartbeatInterval | `time.Duration`          | Interval for SSE comment heartbeats.                                                                 | `15 * time.Second` |
| DisableHeartbeat  | `bool`                   | Disable automatic heartbeat comments. When disabled, disconnected clients may not be detected until the next write. | `false`            |

## Default Config

```go
var ConfigDefault = Config{
    Handler:           nil,
    OnClose:           nil,
    Retry:             0,
    HeartbeatInterval: 15 * time.Second,
    DisableHeartbeat:  false,
}
```

## Stream

```go
func (s *Stream) Event(event Event) error
func (s *Stream) Comment(comment string) error
func (s *Stream) Retry(retry time.Duration) error
func (s *Stream) Context() context.Context
func (s *Stream) Done() <-chan struct{}
func (s *Stream) Err() error
func (s *Stream) LastEventID() string
```

Every write is flushed. A failed flush closes `Done`, stores the error returned by `Err`, and lets the handler stop without relying on `fasthttp.RequestCtx.Done`, which is not a per-client disconnect signal. After a normal handler return, `Done` is closed and `Context()` is canceled while `Err()` remains `nil`; writes after that return `sse: stream closed`.

Automatic heartbeat comments keep idle streams active and make silent client disconnects observable through the next flush error. If heartbeats are disabled, a handler waiting on an external source might not notice a disconnected client until it writes again. Stopping a stream waits for an in-flight heartbeat write to finish, so a very slow client can delay shutdown until the underlying write unblocks.

`Config.Retry` sends the initial reconnect delay when the stream opens. `Event.Retry` changes the reconnect delay for a specific event, following the SSE wire format.
