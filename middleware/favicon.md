# Favicon

Why use this middleware?

- User agents request favicon.ico frequently and indiscriminately, so you may wish to exclude these requests from your logs by using this middleware before your logger middleware.
- This middleware caches the icon in memory to improve performance by skipping disk access.

**Note** This middleware is exclusively for serving the "default, implicit favicon", which is `GET /favicon.ico`.

### Example
```go
package main

import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/fiber/middleware"
)

func main() {
  app := fiber.New()

  // Default ignore favicon
  app.Use(middleware.Favicon())

  // Pass favicon
  app.Use(middleware.Favicon("./favicon.ico"))
  

  app.Use(middleware.Logger())

  app.Listen(3000)
}
```

### Signatures
```go
func Favicon(file ...string) fiber.Handler {}
```