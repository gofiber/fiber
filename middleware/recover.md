# Recover

Recover middleware recovers from panics anywhere in the stack chain and handles the control to the centralized [ErrorHandler](https://docs.gofiber.io/error-handling).

### Example
```go
package main

import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/fiber/middleware"
)

func main() {
  app := fiber.New()

  app.Use(middleware.Recover())

  app.Get("/", func(c *fiber.Ctx) {
    panic("normally this would crash your app")
  })

  app.Listen(3000)
}
```

### Signatures
```go
func Recover() fiber.Handler {}
```