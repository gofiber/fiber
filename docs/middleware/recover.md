---
id: recover
---

# Recover

The Recover middleware for [Fiber](https://github.com/gofiber/fiber) intercepts panics and forwards them to the central [ErrorHandler](../guide/error-handling).

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    recoverer "github.com/gofiber/fiber/v3/middleware/recover"
)
```

Once your Fiber app is initialized, use the middleware like this:

```go
// Initialize default config
app.Use(recoverer.New())

// Panics in subsequent handlers are caught by the middleware
app.Get("/", func(c fiber.Ctx) error {
    panic("I'm an error")
})
```

## Config

| Property          | Type                         | Description                                           | Default                    |
|:------------------|:-----------------------------|:------------------------------------------------------|:---------------------------|
| Next              | `func(fiber.Ctx) bool`       | Skip when the function returns `true`.                | `nil`                      |
| PanicHandler      | `func(fiber.Ctx, any) error` | Customize the error returned from a recovered panic.  | `defaultPanicHandler`      |
| EnableStackTrace  | `bool`                       | Capture and include a stack trace in error responses. | `false`                    |
| StackTraceHandler | `func(fiber.Ctx, any)`       | Handle the captured stack trace when enabled.         | `defaultStackTraceHandler` |

## Default Config

```go
var ConfigDefault = Config{
    Next:              nil,
    PanicHandler:   defaultPanicHandler,
    EnableStackTrace:  false,
    StackTraceHandler: defaultStackTraceHandler,
}
```
