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
| PanicHandler      | `func(fiber.Ctx, any) error` | Customize the error returned from a recovered panic.  | `DefaultPanicHandler`      |
| EnableStackTrace  | `bool`                       | Capture and include a stack trace in error responses. | `false`                    |
| StackTraceHandler | `func(fiber.Ctx, any)`       | Handle the captured stack trace when enabled.         | `defaultStackTraceHandler` |

## Default Config

```go
var ConfigDefault = recoverer.Config{
    Next:              nil,
    PanicHandler:      DefaultPanicHandler,
    StackTraceHandler: defaultStackTraceHandler,
    EnableStackTrace:  false,
}

// Set up a PanicHandler to hide internals.
app.Use(recoverer.New(recover.Config{PanicHandler: func(c fiber.Ctx, r any) error {
    return fiber.ErrInternalServerError
}}))

// In more elaborate scenarios you can also create a custom error which can be processed differently in the fiber.ErrorHandler.
// See the tests for an example of such an ErrorHandler.
// You could also just wrap the default handler's error, e.g. fmt.Errorf("[RECOVERED]: %w", recoverer.DefaultPanicHandler(c, r))
app.Use(recoverer.New(recover.Config{PanicHandler: func(c fiber.Ctx, r any) error {
    return &MyCustomRecoveredFromPanicError {
        Inner: recoverer.DefaultPanicHandler(c, r),
    }
}}))
```
