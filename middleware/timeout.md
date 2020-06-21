# Timeout wrapper

Why use this middleware?

...

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