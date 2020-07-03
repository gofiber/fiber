# Favicon
Favicon middleware ignores favicon requests or caches a provided icon in memory to improve performance by skipping disk access. User agents request favicon.ico frequently and indiscriminately, so you may wish to exclude these requests from your logs by using this middleware before your logger middleware.

**Note** This middleware is exclusively for serving the _default, implicit favicon_, which is `GET /favicon.ico`.

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
      
  // Default ignore favicon
  app.Use(middleware.Favicon())

  // Pass a favicon file that will be cached in memory
  app.Use(middleware.Favicon("./favicon.ico"))

  // ...
}
```

### Signatures
```go
func Favicon(file ...string) fiber.Handler {}
```