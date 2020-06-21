# Timeout
Wrapper function which provides a handler with a timeout.

If the handler takes longer than the given duration, the timeout error is set and forwarded to the next method.
### Example
```go
package main

import (
    "time"
    "github.com/gofiber/fiber"
    "github.com/gofiber/fiber/middleware"
)

func main() {
  app := fiber.New()

  // wrap the handler with a timeout
  app.Get("/foo", middleware.Timeout(
    func(ctx fiber.Ctx) {
        // do somthing
    },
    5 * time.Second,
  ))

  app.Listen(3000)
}
```

### Signatures
```go
func Timeout(handler fiber.Handler, timeout time.Duration) fiber.Handler {}
```