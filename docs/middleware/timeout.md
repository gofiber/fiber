---
id: timeout
---

# Timeout

The timeout middleware limits how long a handler can take to complete. When the
deadline is exceeded, Fiber responds with `408 Request Timeout`. The middleware
uses `context.WithTimeout` and makes the context available through
`c.Context()` so that your code can listen for cancellation.

:::caution
`timeout.New` wraps your final handler and can't be added with `app.Use` or
used in a middleware chain. Register it per route and avoid calling
`c.Next()` inside the wrapped handler—doing so will panic.
:::

## Signatures

```go
func New(handler fiber.Handler, config ...timeout.Config) fiber.Handler
```

## Examples

### Basic example

The following program times out any request that takes longer than two seconds.
The handler simulates work with `sleepWithContext`, which stops when the
request's context is canceled:

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

Test it with curl:

```bash
curl -i http://localhost:3000/sleep/1000   # finishes within the timeout
curl -i http://localhost:3000/sleep/3000   # returns 408 Request Timeout
```

## Config

| Property  | Type               | Description                                                          | Default |
|:----------|:-------------------|:---------------------------------------------------------------------|:-------|
| Next      | `func(fiber.Ctx) bool` | Function to skip the middleware.                                   | `nil`  |
| Timeout   | `time.Duration`    | Timeout duration for requests. `0` or a negative value disables the timeout. | `0`    |
| OnTimeout | `fiber.Handler`    | Handler executed when a timeout occurs. Defaults to returning `fiber.ErrRequestTimeout`. | `nil`  |
| Errors    | `[]error`          | Custom errors treated as timeout errors.                            | `nil`  |

### Use with custom error

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
