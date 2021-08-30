# Skip
Skip middleware for [Fiber](https://github.com/gofiber/fiber) that skips a wrapped handler is a predicate is true.

### Table of Contents
- [Signatures](#signatures)
- [Examples](#examples)


### Signatures
```go
func New(handler fiber.Handler, exclude func(c *fiber.Ctx) bool) fiber.Handler
```

### Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/skip"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
app.Use(skip.New(handler, func(ctx *fiber.Ctx) bool { return ctx.Method() == fiber.MethodOptions }))
```
