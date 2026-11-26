---
id: timeout
---

# Timeout

The timeout middleware enforces a deadline on handler execution. It wraps handlers with
`context.WithTimeout`, exposes the derived context through `c.Context()`, and
returns `503 Service Unavailable` when the deadline is exceeded.

## How It Works

When a timeout occurs, the middleware **returns immediately** without waiting for the
handler to finish. This is achieved through Fiber's **Abandon mechanism**:

1. The handler runs in a goroutine with a timeout context
2. On timeout, the middleware marks the context as "abandoned" and returns `503` immediately
3. The handler goroutine can continue safely (e.g., for cleanup) without blocking the response
4. A background cleanup goroutine waits for the handler to finish, returns the context to the pool and closes what the request left open (a response body stream, multipart temp files), and removes its `Locals`, closing those that implement `io.Closer`

Handlers can detect the timeout by listening on `c.Context().Done()` and return early.
This is the recommended pattern for cooperative cancellation.

The timeout response to a `HEAD` request carries no body, on the default path and the `OnTimeout` path alike.

The timeout response is sent with `Connection: close`, and the connection is closed after it: the timed-out handler still holds the connection and may write to it, as `c.End()` or a hijacking `net/http` handler does, which must not answer the next request on it. Behind `adaptor.FiberHandler`, `FiberHandlerFunc` or `FiberApp`, `net/http` closes an HTTP/1.x connection and shuts an HTTP/2 connection down gracefully.

If a handler panics, the middleware catches it and returns `500 Internal Server Error`. So does a panic in `OnTimeout` once the deadline has passed.

When an enclosing timeout middleware already guards the request with a deadline that comes no later than the configured one, such as a shorter timeout registered with `app.Use`, the middleware runs the handler on the calling goroutine and leaves the timeout to the enclosing one. Its `OnTimeout` then runs only for a timeout error the handler returns before the enclosing deadline passes; after that, the enclosing middleware answers the request. When nested timeouts fire at the same time, only the first to answer writes the timeout response. A shorter deadline set any other way, such as with `c.SetContext`, does not count: the middleware times the request out when it passes.

## Why 503

A handler that outlives its deadline is slow on the server's side: the request arrived complete. `408 Request Timeout` means the server did not receive a complete request in time ([RFC 9110 §15.5.9](https://www.rfc-editor.org/rfc/rfc9110#section-15.5.9)), and it tells the client that it may send the request again, while the timed-out handler can still be running and may finish its work. The default is therefore `503 Service Unavailable` ([§15.6.4](https://www.rfc-editor.org/rfc/rfc9110#section-15.6.4)). Use `504 Gateway Timeout` ([§15.6.5](https://www.rfc-editor.org/rfc/rfc9110#section-15.6.5)) when the handler stands in front of an upstream that did not answer in time, and return it from `OnTimeout`:

```go
timeout.Config{
    Timeout: 5 * time.Second,
    OnTimeout: func(c fiber.Ctx) error {
        return fiber.ErrGatewayTimeout
    },
}
```

## Known limitations

- The timed-out handler keeps running in its own goroutine while the middleware returns `fiber.ErrServiceUnavailable` (or runs `OnTimeout`). A handler must stop using the context once `c.Context()` is done: the client never receives what it writes after that, and middleware registered before the timeout that uses the request or the response after `c.Next()` races with it, since even fasthttp's reads change state the two share. `c.IsAbandoned()` reports such a request.

- When the deadline passes while the handler still runs, `OnTimeout` runs on a context of its own, so the timed-out handler cannot change the response it writes. That context carries the request, its locals, `c.Context()` and the response headers as they were when the handler started, but neither the request body nor the matched route: `c.Body()` is empty, `c.Params` returns the default value, and `c.Next()` is not available there (calling it answers `500`). The response does not go back through the middleware in front either; the [encryptcookie](./encryptcookie.md) middleware still encrypts its cookies, and should its `Encryptor` panic there, the response is a `500` without cookies.

- After the deadline, `OnTimeout` has to buffer the response it writes. A body stream it sets, as `c.SendStream`, `c.SendStreamWriter` or `c.SendFile` do, is closed without being sent: under status `200` the default `503` response takes its place, under any other status the response goes out with that status and no body.

- When `OnTimeout` writes the response and returns `nil` while the handler still runs, the middleware returns `fiber.ErrServiceUnavailable` instead, so middleware registered before it that skips its work on an error, such as `compress`, `etag` or `cache`, leaves the response to the timed-out handler. The client receives the response `OnTimeout` wrote.

- The error is returned to the outer middleware, but the app's `ErrorHandler` is not run for a timed-out request: fasthttp already holds the response it will send, so the handler could not change it, and the timed-out handler may still be writing to the context.

- A request the server refused while reading it, such as one declaring a body over `BodyLimit`, still runs the middleware registered with `Use` (see [Errors Before a Handler Runs](../guide/error-handling.md#errors-before-a-handler-runs)). When a `timeout.New` registered that way fires for it, the client receives the timeout response instead of the refusal.

- Timed-out requests abandon their `fiber.Ctx` to avoid data races with the core
  request handler. The context, a custom `Ctx` implementation included, is
  reclaimed once both the timed-out handler has finished and Fiber has released
  the context, so it does return to the pool. Calling `ForceRelease` yourself on
  such a context does nothing, so an application that released timed-out
  contexts itself does not release them a second time. `ForceRelease` is for a
  context you abandoned yourself, once you can guarantee that no goroutine
  (including Fiber internals) will touch it anymore.

- A fasthttp route handler behind the middleware that answered with a timeout
  response of its own (`fasthttp.TimeoutHandler`, `ctx.TimeoutError`) may still
  run on the request, so the middleware leaves the request and the context to
  the garbage collector, as fasthttp leaves the request. If the handler answered
  first, `OnTimeout` included, the client receives its response and the
  middleware returns a `*fiber.Error` with its status. Give such a handler a
  shorter timeout than the middleware: one that answers after the middleware
  writes to a request fasthttp has already given up on.

:::caution
`timeout.New` wraps your final handler, so register it per route rather than
with `app.Use`. The handler runs in its own goroutine: if it calls `c.Next()`,
the rest of the chain runs in that goroutine too and keeps running after the
timeout response has been sent.
:::

## Signatures

```go
func New(handler fiber.Handler, config ...timeout.Config) fiber.Handler
```

## Examples

### Basic example

The following program times out any request that takes longer than two seconds.
The handler simulates work with `sleepWithContext`, which stops when the
context is canceled:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/timeout"
)

func sleepWithContext(ctx context.Context, d time.Duration) error {
    select {
    case <-time.After(d):
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func main() {
    app := fiber.New()

    handler := func(c fiber.Ctx) error {
        delay, _ := time.ParseDuration(c.Params("delay") + "ms")
        if err := sleepWithContext(c.Context(), delay); err != nil {
            return fmt.Errorf("%w: execution error", err)
        }
        return c.SendString("finished")
    }

    app.Get("/sleep/:delay", timeout.New(handler, timeout.Config{
        Timeout: 2 * time.Second,
    }))

    log.Fatal(app.Listen(":3000"))
}
```

Use these requests to see the middleware in action:

```bash
curl -i http://localhost:3000/sleep/1000   # finishes within the timeout
curl -i http://localhost:3000/sleep/3000   # returns 503 Service Unavailable
```

## Config

| Property    | Type               | Description                                                          | Default |
|:------------|:-------------------|:---------------------------------------------------------------------|:-------|
| Next        | `func(fiber.Ctx) bool` | Function to skip this middleware when it returns `true`.            | `nil`  |
| Timeout     | `time.Duration`    | Timeout duration for requests. `0` or a negative value disables the timeout. | `0`    |
| OnTimeout   | `fiber.Handler`    | Handler executed when a timeout occurs. It may write the response itself or return a `*fiber.Error`, whose status and message then form the response; otherwise the default 503 is sent. After the deadline it runs on a context of its own, see [Known limitations](#known-limitations). Defaults to returning `fiber.ErrServiceUnavailable`. | `nil`  |
| Errors      | `[]error`          | Custom errors treated as timeout errors, in addition to `context.DeadlineExceeded`. A handler that returns one of them (checked with `errors.Is`) gets the timeout response. | `nil`  |

### Use with a custom error

```go
var ErrFooTimeOut = errors.New("foo context canceled")

func main() {
    app := fiber.New()
    h := func(c fiber.Ctx) error {
        sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")
        if err := sleepWithContextWithCustomError(c.Context(), sleepTime); err != nil {
            return fmt.Errorf("%w: execution error", err)
        }
        return nil
    }

    app.Get("/foo/:sleepTime", timeout.New(h, timeout.Config{Timeout: 2 * time.Second, Errors: []error{ErrFooTimeOut}}))
    log.Fatal(app.Listen(":3000"))
}

func sleepWithContextWithCustomError(ctx context.Context, d time.Duration) error {
    timer := time.NewTimer(d)
    select {
    case <-ctx.Done():
        if !timer.Stop() {
            <-timer.C
        }
        return ErrFooTimeOut
    case <-timer.C:
    }
    return nil
}
```

### Sample usage with a database call

```go
func main() {
    app := fiber.New()
    db, _ := gorm.Open(postgres.Open("postgres://localhost/foodb"), &gorm.Config{})

    handler := func(ctx fiber.Ctx) error {
        tran := db.WithContext(ctx.Context()).Begin()

        if tran = tran.Exec("SELECT pg_sleep(50)"); tran.Error != nil {
            return tran.Error
        }

        if tran = tran.Commit(); tran.Error != nil {
            return tran.Error
        }

        return nil
    }

    app.Get("/foo", timeout.New(handler, timeout.Config{Timeout: 10 * time.Second}))
    log.Fatal(app.Listen(":3000"))
}
```

## Default Config

With the default config, `Timeout` is `0`, so the wrapped handler runs without a deadline until you set one.

```go
var ConfigDefault = Config{
    Next:      nil,
    Timeout:   0,
    OnTimeout: nil,
    Errors:    nil,
}
```
