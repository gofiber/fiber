---
id: recover
---

# Recover

Recover middleware for [Fiber](https://github.com/gofiber/fiber) that recovers from panics anywhere in the stack chain and handles the control to the centralized [ErrorHandler](https://docs.gofiber.io/guide/error-handling).

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/recover"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
// Initialize default config
app.Use(recover.New())

// This panic will be caught by the middleware
app.Get("/", func(c *fiber.Ctx) error {
    panic("I'm an error")
})
```

## Config

| Property          | Type                            | Description                                                         | Default                  |
|:------------------|:--------------------------------|:--------------------------------------------------------------------|:-------------------------|
| Next              | `func(*fiber.Ctx) bool`         | Next defines a function to skip this middleware when returned true. | `nil`                    |
| EnableStackTrace  | `bool`                          | EnableStackTrace enables handling stack trace.                      | `false`                  |
| StackTraceHandler | `func(*fiber.Ctx, interface{})` | StackTraceHandler defines a function to handle stack trace.         | defaultStackTraceHandler |

## Default Config

```go
var ConfigDefault = Config{
    Next:              nil,
    EnableStackTrace:  false,
    StackTraceHandler: defaultStackTraceHandler,
}
```
