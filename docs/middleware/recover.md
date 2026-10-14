---
id: recover
---

# Recover

The Recover middleware for [Fiber](https://github.com/gofiber/fiber) intercepts panics and forwards them to the central [ErrorHandler](../guide/error-handling).

## Signatures

```go
func New(config ...Config) fiber.Handler
func DefaultPanicHandler(c fiber.Ctx, r any) error
```

## Examples

Import the middleware package:

```go
import (
    "errors"
    "fmt"

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

Use `PanicHandler` to decide which error the central `ErrorHandler` receives:

```go
// Set up a PanicHandler to hide internals.
app.Use(recoverer.New(recoverer.Config{PanicHandler: func(c fiber.Ctx, r any) error {
    return fiber.ErrInternalServerError
}}))

// You could also just wrap the default handler's error.
app.Use(recoverer.New(recoverer.Config{PanicHandler: func(c fiber.Ctx, r any) error {
    return fmt.Errorf("[RECOVERED]: %w", recoverer.DefaultPanicHandler(c, r))
}}))
```

In more elaborate scenarios you can create a custom error type that your `ErrorHandler` handles differently:

```go
// MyCustomRecoveredFromPanicError marks an error that came from a recovered panic.
type MyCustomRecoveredFromPanicError struct {
    Inner error
}

func (e *MyCustomRecoveredFromPanicError) Error() string { return e.Inner.Error() }
func (e *MyCustomRecoveredFromPanicError) Unwrap() error { return e.Inner }

app := fiber.New(fiber.Config{
    ErrorHandler: func(c fiber.Ctx, err error) error {
        var recovered *MyCustomRecoveredFromPanicError
        if errors.As(err, &recovered) {
            // Report the panic here, then answer without its details.
            return c.SendStatus(fiber.StatusInternalServerError)
        }
        return fiber.DefaultErrorHandler(c, err)
    },
})

app.Use(recoverer.New(recoverer.Config{PanicHandler: func(c fiber.Ctx, r any) error {
    return &MyCustomRecoveredFromPanicError{
        Inner: recoverer.DefaultPanicHandler(c, r),
    }
}}))
```

## Config

| Property          | Type                         | Description                                           | Default                    |
|:------------------|:-----------------------------|:------------------------------------------------------|:---------------------------|
| Next              | `func(fiber.Ctx) bool`       | Skip when the function returns `true`.                | `nil`                      |
| PanicHandler      | `func(fiber.Ctx, any) error` | Customize the error returned from a recovered panic. `DefaultPanicHandler` returns the panic value when it is an `error` and `fmt.Errorf("%v", r)` otherwise. | `DefaultPanicHandler`      |
| EnableStackTrace  | `bool`                       | Call `StackTraceHandler` with the recovered value when a panic is caught. The stack trace is not added to the returned error or the response. | `false`                    |
| StackTraceHandler | `func(fiber.Ctx, any)`       | Handle the captured panic when `EnableStackTrace` is `true`. The default writes `recovered panic: <value>` and the stack trace to `os.Stderr`. | `defaultStackTraceHandler` |

## Default Config

```go
var ConfigDefault = recoverer.Config{
    Next:              nil,
    PanicHandler:      DefaultPanicHandler,
    StackTraceHandler: defaultStackTraceHandler,
    EnableStackTrace:  false,
}
```
