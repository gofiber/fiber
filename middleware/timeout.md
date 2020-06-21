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

  // Default ignore favicon
  timeout := middleware.Timeout(app)

  // Pass favicon
  app.Get("/foo", timeout.WrapHandler(
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
func Timeout(app *fiber.App) *timeoutWrapper {}
func (wrapper *timeoutWrapper) WrapHandler(handler fiber.Handler, timeout time.Duration) fiber.Handler {}
```