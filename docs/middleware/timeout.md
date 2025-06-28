---
id: timeout
---

# Timeout

There exist two distinct implementations of timeout middleware [Fiber](https://github.com/gofiber/fiber).

## New

As a `fiber.Handler` wrapper, it creates a context with `context.WithTimeout` which is then used with `c.Context()`.

If the context passed executions (eg. DB ops, Http calls) takes longer than the given duration to return, the timeout error is set and forwarded to the centralized `ErrorHandler`.

It does not cancel long running executions. Underlying executions must handle timeout by using `context.Context` parameter.

## Signatures

```go
func New(handler fiber.Handler, config ...timeout.Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/timeout"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
func main() {
    app := fiber.New()
    h := func(c fiber.Ctx) error {
        sleepTime, _ := time.ParseDuration(c.Params("sleepTime") + "ms")
        if err := sleepWithContext(c.Context(), sleepTime); err != nil {
            return fmt.Errorf("%w: execution error", err)
        }
        return nil
    }

    app.Get("/foo/:sleepTime", timeout.New(h, timeout.Config{Timeout: 2 * time.Second}))
    log.Fatal(app.Listen(":3000"))
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
    timer := time.NewTimer(d)

    select {
    case <-ctx.Done():
        if !timer.Stop() {
            <-timer.C
        }
        return context.DeadlineExceeded
    case <-timer.C:
    }
    return nil
}
```

Test http 200 with curl:

```bash
curl --location -I --request GET 'http://localhost:3000/foo/1000' 
```

Test http 408 with curl:

```bash
curl --location -I --request GET 'http://localhost:3000/foo/3000' 
```

Use with custom error:

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

Sample usage with a DB call:

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

## Config

### Config

| Property   | Type                                | Description                                                               | Default |
|:-----------|:------------------------------------|:--------------------------------------------------------------------------|:-------|
| Next       | `func(fiber.Ctx) bool`              | Function to skip the middleware.                                         | `nil`  |
| Timeout    | `time.Duration`                     | Default timeout for routes.                                              | `0`    |
| OnTimeout  | `fiber.Handler`                     | Handler executed when a timeout occurs.                                  | `nil`  |
| Errors     | `[]error`                           | Custom errors treated as timeout errors.                                 | `nil`  |

