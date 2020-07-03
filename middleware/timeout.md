# Timeout
Wrapper function which provides a handler with a timeout.

If the handler takes longer than the given duration to return, the timeout error is set and forwarded to the error handler.

### Example
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/fiber/middleware"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
func main() {
  app := fiber.New()
    
  handler := func(ctx *fiber.Ctx) {
    ctx.Send("Hello, World 👋!")
  }

  // Wrap the handler with a timeout
  app.Get("/foo", middleware.Timeout(handler, 5 * time.Second))

  // ...
}
```

### Signatures
```go
func Timeout(handler fiber.Handler, timeout time.Duration) fiber.Handler {}
```
